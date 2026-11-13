/*
 * filedialog.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
	ghtml "github.com/stricaud/gscore/internal/render/html"
	gpdf "github.com/stricaud/gscore/internal/render/pdf"
	"github.com/stricaud/gscore/internal/render/svg"
)

// Opening a file needs a real dialog.
//
// A web page would use <input type="file">, but the native window has no file
// panel behind it — the element simply does nothing, which is why File > Open
// appeared dead. So the dialog is asked for from the system instead, and the
// browser keeps its own upload path for when gscore is served rather than run.

// errNoDialog says the platform has no usable file chooser.
var errNoDialog = errors.New("no native file dialog available here")

// closed reports that a dialog was shut by the person rather than failing
// to appear, which is the difference between doing nothing and saying so.
//
// Both zenity and kdialog answer 1 for a panel the person closed. Anything
// else is the panel not working — no display to open it on, a missing
// library, a tool that will not start — and that has to be reported, because
// the caller's answer to a dialog it cannot have is to offer the browser's
// own file picker instead. Answered as a cancellation it offers nothing.
func closed(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

// pickFile shows the system's open dialog and returns the chosen path.
func pickFile() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// AppleScript gives a real Finder panel with no extra dependency.
		//
		// No "of type" filter: that clause wants uniform type identifiers, and
		// handing it bare extensions makes the dialog match nothing and fail,
		// which looks exactly like the user cancelling.
		script := `POSIX path of (choose file with prompt "Open score"` +
			appleScriptLocation(ScoresDir()) + `)`
		cmd := exec.Command("osascript", "-e", script)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			// Cancelling is error -128 and is not a failure; anything else is.
			if strings.Contains(stderr.String(), "-128") ||
				strings.Contains(stderr.String(), "User canceled") {
				return "", nil
			}
			return "", fmt.Errorf("file dialog failed: %s", strings.TrimSpace(stderr.String()))
		}
		return strings.TrimSpace(string(out)), nil

	case "linux":
		for _, tool := range []string{"zenity", "kdialog"} {
			path, err := exec.LookPath(tool)
			if err != nil {
				continue
			}
			var cmd *exec.Cmd
			// A trailing separator is what tells both of them the path is
			// the folder to start in rather than a file to select.
			start := ScoresDir() + string(os.PathSeparator)
			if tool == "zenity" {
				cmd = exec.Command(path, "--file-selection",
					"--title=Open score", "--filename="+start)
			} else {
				cmd = exec.Command(path, "--getopenfilename", start)
			}
			out, err := cmd.Output()
			if err == nil {
				return strings.TrimSpace(string(out)), nil
			}
			// Closing the dialog and failing to open one look the same from
			// here: both come back with nothing and a status that is not
			// zero. Reading every one of them as a cancellation is what
			// made File > Open do nothing at all on a machine where the
			// dialog could not start — no score, no message, no sign that
			// anything had been asked for. "It does not show or do
			// anything, not even an error."
			//
			// The exit code tells them apart. Both tools answer 1 when the
			// person closes the panel and something else when they could
			// not show one — zenity says 255 — and a tool that cannot be
			// started at all never gets a code.
			if closed(err) {
				return "", nil
			}
			return "", fmt.Errorf("%s could not open a file dialog: %w", tool, err)
		}
		return "", errNoDialog

	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms;` +
			`$d = New-Object System.Windows.Forms.OpenFileDialog;` +
			`$d.InitialDirectory = '` + powershellQuote(ScoresDir()) + `';` +
			`$d.Filter = 'Scores|*.abc;*.mscz;*.mscx;*.musicxml;*.xml;*.mxl;*.gscore;*.nwctxt;*.nwc;*.mus;*.musx;*.gmn;*.ly;*.dorico;*.tab;*.btab;*.txt;*.gp3;*.gp4;*.gp5|All files|*.*';` +
			`if ($d.ShowDialog() -eq 'OK') { $d.FileName }`
		out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	}
	return "", errNoDialog
}

// handlePickFile shows the dialog and opens whatever is chosen, in one step,
// so the interface does not have to handle a path it was never given.
func (a *App) handlePickFile(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	native := a.native
	a.mu.RUnlock()

	// Served over a network, a dialog would open on the server's screen, which
	// is not what anyone wants. Only the local window gets one.
	if !native && !a.local {
		http.Error(w, "no dialog when gscore is served; use the file picker",
			http.StatusNotImplemented)
		return
	}

	path, err := pickFile()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	if path == "" {
		writeJSON(w, map[string]any{"cancelled": true})
		return
	}

	// The same dialog serves opening a score and taking symbols out of one.
	if r.URL.Query().Get("symbols") == "1" {
		a.importSymbolsFrom(w, path)
		return
	}
	// And picking the cover or the stamp, which are files read straight
	// into the score.
	//
	// Here for the reason the picture below is here: a hidden
	// <input type="file"> does nothing at all in the native window, so
	// the button sat there and nothing happened, with nothing on the
	// screen to say why. Reported exactly that way.
	if which := r.URL.Query().Get("artwork"); which == "cover" || which == "stamp" {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.setArtworkFrom(w, which, data, r.URL.Query().Get("align"))
		return
	}
	// And picking a picture to trace a symbol from.
	//
	// The drawing window has a file box of its own for this, which does
	// nothing at all in the native window: a hidden <input type="file">
	// needs the shell to put a dialog up, and this shell does not. So the
	// same button asks here instead, and the picture comes back as the
	// grid rather than as a file.
	if r.URL.Query().Get("picture") == "1" {
		fit, _ := strconv.Atoi(r.URL.Query().Get("fit"))
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		// With the bytes, so the window can ask for the picture again at
		// another size without the dialog opening a second time.
		writePictureGrid(w, data, name, fit, true)
		return
	}
	if err := a.openPath(path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "filename": filepath.Base(path)})
}

// handleOpenPath opens a file the interface already knows the path of.
func (a *App) handleOpenPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "no path given", http.StatusBadRequest)
		return
	}
	if !a.local {
		http.Error(w, "opening by path is only allowed on this machine",
			http.StatusForbidden)
		return
	}
	if err := a.openPath(path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "filename": filepath.Base(path)})
}

// OpenPath reads a file into the editor, for a shell that opens documents
// itself: the macOS application is handed a path by the Finder and has
// nowhere to send it but here.
//
// A file the importer only half understands is still opened, with the
// trouble recorded for the log window, because a score with one bad bar in
// it is still a score.
func (a *App) OpenPath(path string) error { return a.openPath(path) }

func (a *App) openPath(path string) error {
	// A PDF goes a different way: its bytes are kept so the reading can be
	// reviewed and done again with the grouping said out loud. See
	// pdfimport.go.
	if isPDFName(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scores, err := a.readPDF(path, data, 0)
		if len(scores) == 0 {
			return err
		}
		a.lock()
		a.setBook(scores, path, nil)
		a.mu.Unlock()
		a.remember(path)
		return nil
	}
	imp, err := format.ImporterFor(path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scores, err := format.ImportAll(imp, f)
	// A format may understand part of a file and hand back what it got. That
	// is worth showing, so only a read that produced nothing is fatal.
	if len(scores) == 0 {
		return err
	}
	a.lock()
	a.forgetPDF()
	a.setBook(scores, path, err)
	a.mu.Unlock()
	a.remember(path)
	return nil
}

// handleSave writes the score to a file, with its symbols.
//
// Saving has to carry the drawings: a .gscore holding a score that refers to
// symbols it does not contain is a score that opens wrong somewhere else, and
// the whole reason for the format is that the artwork travels with the music.
func (a *App) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		if !a.local && !a.native {
			// Nowhere to write to, so the document is the answer and the
			// browser's own panel asks where it goes.
			a.sendDocument(w, r)
			return
		}
		var err error
		suggested := a.suggestedName()
		if ext := r.URL.Query().Get("as"); ext != "" {
			suggested = strings.TrimSuffix(suggested, filepath.Ext(suggested)) + "." + ext
		}
		var shown bool
		path, shown, err = saveDialog(suggested)
		if err != nil || (!shown && path == "") {
			// No panel: gscore asks for the name itself. Saving is not
			// something to be defeated by a missing system dialog.
			writeJSON(w, map[string]any{
				"needPath":  true,
				"suggested": filepath.Join(a.saveDirectory(), suggested),
			})
			return
		}
		if path == "" {
			writeJSON(w, map[string]any{"cancelled": true})
			return
		}
	}
	if !a.local {
		http.Error(w, "saving to a path is only allowed on this machine",
			http.StatusForbidden)
		return
	}

	if err := a.writeFile(path, r); err != nil {
		http.Error(w, err.Error(), statusFor(err))
		return
	}
	if !savesTheScore(filepath.Ext(path)) {
		// A drawing or a recording of the score is not the score, and
		// writing one leaves the edits still unsaved. It is not a file to
		// reopen either, so it is not remembered.
		writeJSON(w, map[string]string{"status": "ok", "path": path})
		return
	}

	a.lock()
	a.filename = path
	// What is on disk is what is in hand, so there is nothing to warn about
	// on the way out.
	a.unsaved = false
	a.mu.Unlock()
	// A score saved is a score to be reopened.
	a.remember(path)
	writeJSON(w, map[string]string{"status": "ok", "path": path})
}

// writeFile writes the document beside where it is going and moves it
// into place when it is whole.
//
// Not straight onto the path. os.Create truncates what is there before
// anything has been written, and two things follow from that, both of
// them bad.
//
// A write that fails leaves nothing. Saving a score that came from a
// MuseScore file writes back to the file it came from, gscore has no
// exporter for that format, and the refusal arrived after the original
// had already been emptied: six megabytes of somebody's symphony gone,
// and the error message said only that the format could not be written.
//
// And a write that is slow leaves nothing for as long as it takes. A
// .gscore carries a PDF of the score inside it, and rendering one of a
// fifty-three stave symphony takes half a minute — during which the file
// on disk is there and empty. Anybody who looks sees an empty file,
// which is what was reported.
//
// Beside it rather than in a temporary directory, because the move has to
// be a rename and not a copy: a rename within one directory is atomic, so
// the file is either the old one or the new one and never half of either.
func (a *App) writeFile(path string, r *http.Request) error {
	dir, name := filepath.Dir(path), filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		// Nowhere to write beside it — a read-only directory, most
		// likely. Said plainly, and nothing of what is there is touched.
		return writeRefused{err}
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name()) // a no-op once the rename has happened
	}()

	if err := a.writeDocument(tmp, filepath.Ext(path), r); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// The permissions a new file would have had. CreateTemp makes it
	// readable by its owner alone, which is not what somebody expects of
	// a score they are about to send to a choir.
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// writeRefused is a file that could not be opened to write to, which is
// the caller's fault rather than the score's.
type writeRefused struct{ error }

// renderTo draws the score as a standalone SVG, with the font inside it so
// the file opens correctly anywhere rather than only where gscore is running.
func (a *App) renderTo(w io.Writer, r *http.Request) error {
	a.mu.RLock()
	score := a.score
	eopt := a.engraving()
	symbols := a.symbols
	a.mu.RUnlock()

	q := r.URL.Query()
	switch q.Get("mode") {
	case "strip":
		eopt.Mode = engrave.Strip
	case "page":
		eopt.Mode = engrave.Paginated
	}

	ropt := svg.DefaultOptions()
	ropt.Symbols = symbols
	ropt.EmbedFont = true
	if v := q.Get("space"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 3 && f <= 40 {
			ropt.StaffSpace = f
		}
	}
	return svg.Render(w, engrave.Engrave(score, eopt), ropt)
}

// renderPage writes the score as a web page: the music drawn and a player
// to hear it with.
//
// It is for sending to somebody who has no music program — they open it and
// read it. Whether the font and the player travel with it or are fetched
// from an address is asked for, one each.
func (a *App) renderPage(w io.Writer, r *http.Request) error {
	a.mu.RLock()
	score := a.score
	eopt := a.engraving()
	symbols := a.symbols
	a.mu.RUnlock()

	q := r.URL.Query()
	switch q.Get("mode") {
	case "strip":
		eopt.Mode = engrave.Strip
	case "page":
		eopt.Mode = engrave.Paginated
	}

	hopt := ghtml.DefaultOptions()
	hopt.SVG.Symbols = symbols
	if v := q.Get("space"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 3 && f <= 40 {
			hopt.SVG.StaffSpace = f
		}
	}
	// Addresses to fetch the font and the player from, for a page that will
	// live on a site that serves them. Left out, each is carried, which is
	// what a file somebody was sent needs.
	hopt.FontURL = q.Get("font")
	hopt.ScriptURL = q.Get("script")

	lay := engrave.Engrave(score, eopt)
	if q.Get("play") != "0" {
		hopt.Sound = PageSound(score, lay, hopt.SVG.StaffSpace)
	}
	return ghtml.Render(w, lay, score, symbols, hopt)
}

// handlePDF hands a PDF straight back, for a browser to download.
//
// Running as a page there is no filesystem to write to and no save panel to
// ask with, so the file comes back in the response and the browser does what
// it does with a file. It is the same PDF either way.
func (a *App) handlePDF(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := a.renderPDF(&buf, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.mu.RLock()
	name := a.suggestedBase()
	a.mu.RUnlock()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.pdf"`)
	_, _ = w.Write(buf.Bytes())
}

// suggestedBase is the file's name without its extension, for naming what
// comes out of it. The caller holds the lock.
func (a *App) suggestedBase() string {
	if a.filename != "" {
		base := filepath.Base(a.filename)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	if t := a.score.Identity.Title; t != "" {
		var b strings.Builder
		for _, r := range t {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
				b.WriteRune(r)
			default:
				b.WriteByte('-')
			}
		}
		return b.String()
	}
	return "score"
}

// renderPDF writes the score as a PDF, in whichever shape was asked for.
//
//	what=tune   the tune on screen            (the default)
//	what=book   every tune in the file, one after another
//	what=parts  the full score, then each staff alone
//
// The same three choices the print view offers, because they are the same
// question: a tune book is a book and a piece for several players is a score
// plus the parts, and which of those you want is not something the program
// can work out for you.
func (a *App) renderPDF(w io.Writer, r *http.Request) error {
	a.mu.RLock()
	score, book, symbols := a.score, a.book, a.symbols
	filename := a.filename
	// Whether the beats are marked, as the editor is showing them. Most of
	// the view's options are deliberately not inherited here — a printed
	// part is set up in the print dialog and not by whatever the editor
	// happened to be showing — but this one is off unless somebody has
	// turned it on, so carrying it can only ever give them what they asked
	// for. Somebody who marks the beats to practise a passage and then
	// prints it is printing it to practise from.
	beatMarks := a.engraveOpt.ShowBeatMarks
	a.mu.RUnlock()

	q := r.URL.Query()

	// The paper the score says it is printed on, which is a decision
	// somebody made about the document and is kept with it. The query
	// still overrides, for the one-off: exporting a copy on Letter to send
	// to somebody is not a change to the piece.
	popt := paperFor(score.Page)
	if q.Get("paper") == "letter" {
		letter := gpdf.Letter()
		popt.PageWidth, popt.PageHeight = letter.PageWidth, letter.PageHeight
		if score.Page.Landscape {
			popt = popt.Landscape()
		}
	}
	popt.Symbols = symbols
	popt.Title = score.Identity.Title
	popt.Author = score.Identity.Composer
	if v := q.Get("space"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 3 && f <= 20 {
			popt.StaffSpace = f
		}
	}
	// A full score is wider than it is tall — twelve staves to a system — so
	// the paper can be turned over to suit it.
	if q.Get("orientation") == "landscape" {
		popt = popt.Landscape()
	}
	popt.PageNumbers = q.Get("pagenumbers") != "0"

	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.ShowMeasureNumbers = q.Get("numbers") != "0"
	eopt.MultiRests = q.Get("multirest") == "1"
	// The coloured noteheads go to the paper too. Somebody reading by the
	// colours is reading by them away from the screen as much as at it —
	// that is most of what printing a part is for — and a PDF that came out
	// black would be the one copy they cannot use.
	eopt.ColourNotes = ColourNotes()
	eopt.NoteColours = NoteColours()
	eopt.ShowBeatMarks = beatMarks
	// As it sounds, or as it is read. A copy is moved to concert pitch and
	// that copy is drawn; what is saved in the file is untouched.
	concert := q.Get("concert") == "1"
	// The staves are named, however many there are. A printed score says
	// what its lines are, and a piece on one staff is not the exception:
	// it is read away from the program that knows, often by somebody who
	// did not write it, and "which instrument is this" is not a question a
	// sheet of music should leave to the person holding it. One staff is
	// named once, at the head of the first system — see engrave.staffLabel
	// — so the word is a heading and not a label down every page.
	eopt.ShowPartNames = true
	// Not fitted here: what the paper says about type size and line length
	// is settled per piece below, because the pieces are different sizes of
	// music.

	// Each piece is fitted to its own paper. A document of several pieces
	// is not one drawing in several parts: a book's exercises differ from
	// one another, and a part differs from the score it came out of by an
	// order of magnitude — seventeen staves against one.
	fit := func(sc *model.Score, o engrave.Options, paper gpdf.Options) gpdf.Section {
		if concert {
			sc = concertOf(sc)
		}
		paper, o = gpdf.Fit(sc, paper, o)
		return gpdf.Section{Layout: engrave.Engrave(sc, o), Opt: paper}
	}

	var secs []gpdf.Section
	switch q.Get("what") {
	case "book":
		if popt.Title == "" {
			popt.Title = filepath.Base(filename)
		}
		for _, sc := range book {
			secs = append(secs, fit(sc, eopt, popt))
		}
	case "parts":
		secs = append(secs, fit(score, eopt, popt))
		if len(score.Parts) > 1 {
			// The parts go upright whichever way the score went. A score of
			// many staves is turned on its side because that is what holds
			// it; one staff on a sheet in that shape is a line of music a
			// foot long with a hand's breadth of paper under it, and a
			// player on a stand turns twice as many pages for the trouble.
			part := popt
			if part.PageWidth > part.PageHeight {
				part.PageWidth, part.PageHeight = part.PageHeight, part.PageWidth
			}
			for _, p := range score.Parts {
				o := eopt
				// One staff on its own needs no name beside it: the subtitle
				// says whose part it is, and a label on every system of a
				// single-staff part is only in the way.
				o.ShowPartNames = false
				paper := part
				// Whose part it is, in the margin of its first page. A part
				// leaves the document on its own and has to say what it is
				// without being read.
				paper.Label = partLabel(p)
				secs = append(secs, fit(onePart(score, p), o, paper))
			}
		}
	default:
		secs = append(secs, fit(score, eopt, popt))
	}
	return gpdf.RenderSections(w, secs)
}

func (a *App) suggestedName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.filename != "" {
		base := filepath.Base(a.filename)
		return strings.TrimSuffix(base, filepath.Ext(base)) + ".gscore"
	}
	_ = a.score
	name := a.score.Identity.Title
	if name == "" {
		name = "untitled"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String() + ".gscore"
}

// saveDialog shows the system's save panel.
// saveDialog shows the system's save panel.
//
// It reports whether the panel was actually shown, which matters because every
// way of failing looks the same from here: AppleScript answers "User canceled"
// (-128) both when somebody presses Cancel and when the panel could not be put
// on screen at all. Treating the second as the first is what made File ▸ Save
// do nothing at all, with no error and no dialog — the worst possible answer.
//
// A person takes some time to find a folder and press Cancel. A panel that was
// never shown comes back at once. That is the difference used here, and when
// the panel does not appear gscore asks for the name itself rather than
// quietly giving up.
func saveDialog(suggested string) (path string, shown bool, err error) {
	switch runtime.GOOS {
	case "darwin":
		// Bringing the process forward first, or the panel opens behind the
		// window and looks like nothing happened.
		// The folder and the name are given separately: AppleScript takes a
		// name in "default name" and a folder in "default location", and a
		// path handed to the first of them is read as a very long file name.
		// So the panel opened wherever the Finder was last, whatever
		// saveDirectory had decided.
		dir, name := filepath.Split(suggested)
		if dir == "" {
			dir = ScoresDir()
		}
		script := `tell application "System Events" to set frontmost of ` +
			`(first process whose unix id is ` + strconv.Itoa(os.Getpid()) + `) to true
POSIX path of (choose file name with prompt "Save score" default name "` +
			strings.ReplaceAll(name, `"`, "") + `"` +
			appleScriptLocation(strings.TrimSuffix(dir, "/")) + `)`

		started := time.Now()
		cmd := exec.Command("osascript", "-e", script)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			if strings.Contains(stderr.String(), "-128") ||
				strings.Contains(stderr.String(), "User canceled") {
				return "", time.Since(started) > panelShownAfter, nil
			}
			return "", false, fmt.Errorf("save dialog failed: %s", strings.TrimSpace(stderr.String()))
		}
		return strings.TrimSpace(string(out)), true, nil

	case "linux":
		for _, tool := range []string{"zenity", "kdialog"} {
			toolPath, err := exec.LookPath(tool)
			if err != nil {
				continue
			}
			var cmd *exec.Cmd
			if tool == "zenity" {
				cmd = exec.Command(toolPath, "--file-selection", "--save",
					"--confirm-overwrite", "--filename="+suggested)
			} else {
				cmd = exec.Command(toolPath, "--getsavefilename", suggested)
			}
			started := time.Now()
			out, err := cmd.Output()
			if err != nil {
				return "", time.Since(started) > panelShownAfter, nil
			}
			return strings.TrimSpace(string(out)), true, nil
		}
		return "", false, errNoDialog

	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms;` +
			`$d = New-Object System.Windows.Forms.SaveFileDialog;` +
			`$d.FileName = '` + powershellQuote(suggested) + `';` +
			`$d.Filter = 'gscore|*.gscore|MusicXML|*.musicxml|All files|*.*';` +
			`if ($d.ShowDialog() -eq 'OK') { $d.FileName }`
		started := time.Now()
		out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
		if err != nil {
			return "", time.Since(started) > panelShownAfter, nil
		}
		return strings.TrimSpace(string(out)), true, nil
	}
	return "", false, errNoDialog
}

// appleScriptLocation is the clause that tells a Finder panel which folder to
// open in, or nothing at all when there is no sensible one to name.
func appleScriptLocation(dir string) string {
	if dir == "" || dir == "." {
		return ""
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	// A path with a quote in it would end the string early. Folder names may
	// hold one, and a broken script reads to the caller exactly like the
	// person cancelling the panel.
	return ` default location (POSIX file "` +
		strings.ReplaceAll(dir, `"`, `\"`) + `")`
}

// powershellQuote escapes a path for a single-quoted PowerShell string, where
// the one character that matters is the quote itself, doubled.
func powershellQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// panelShownAfter is how long a system panel has to have been up for somebody
// to have had a chance to cancel it. Anything faster never appeared.
const panelShownAfter = 400 * time.Millisecond

// saveDirectory is where a file typed by name rather than chosen in a panel
// should land: beside the score it came from, or in gscore's own folder.
func (a *App) saveDirectory() string {
	a.mu.RLock()
	name := a.filename
	a.mu.RUnlock()
	if name != "" {
		if dir := filepath.Dir(name); dir != "" && dir != "." {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
			return dir
		}
	}
	return ScoresDir()
}

// handlePage hands the web page straight back, for a browser to download.
//
// Running as a page there is no filesystem to write to and no save panel to
// ask with, so the file comes back in the response. It is the same page
// either way.
// handleConvert hands back the score in another format, as a download.
//
// For the formats that are text or markup — LilyPond, MusicXML — where
// there is no machine to save to: served to a browser, or running as a
// page. The same exporters the save panel uses; only the way out differs.
func (a *App) handleConvert(w http.ResponseWriter, r *http.Request) {
	ext := strings.TrimPrefix(r.URL.Query().Get("as"), ".")
	if ext == "" {
		http.Error(w, "say which format", http.StatusBadRequest)
		return
	}
	exp, err := format.ExporterFor("score." + ext)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	score := a.score
	name := a.suggestedBase()
	a.mu.RUnlock()

	var buf bytes.Buffer
	if err := exp.Export(&buf, score); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Sniffed rather than declared. Two of the formats gscore writes are
	// zips — a .gscore and a compressed MusicXML — and announced as text
	// they arrive broken: anything that reads the answer as text replaces
	// every byte that is not valid UTF-8, which is most of a zip.
	w.Header().Set("Content-Type", http.DetectContentType(buf.Bytes()))
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", name+"."+ext))
	w.Write(buf.Bytes())
}

// handleMIDI hands back the performance as a standard MIDI file.
//
// The counterpart of handlePage for sound: in the browser there is nothing
// to save to, so the file comes back as a download.
func (a *App) handleMIDI(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	a.mu.RLock()
	err := MIDIFile(&buf, a.score)
	name := a.suggestedBase()
	a.mu.RUnlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/midi")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", name+".mid"))
	w.Write(buf.Bytes())
}

func (a *App) handlePage(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := a.renderPage(&buf, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.mu.RLock()
	name := a.suggestedBase()
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", name+".html"))
	w.Write(buf.Bytes())
}

/*
 * What a saved document holds
 *
 * There are two ways of asking for one — a file on a disk and a download in
 * a browser — and they must not differ, because they are the same document.
 * So both come from writeDocument, and neither knows about the other.
 *
 * The difference is not academic. A .gscore carries the drawings used in the
 * score and a PDF of the page, and the one the format registry writes
 * carries neither: a page saving through the registry would hand somebody a
 * file with their symbols missing from it and nothing to say so.
 */

// writeDocument writes the score out in the format an extension names.
func (a *App) writeDocument(out io.Writer, ext string, r *http.Request) error {
	// A drawing is not a score format, so it is not in the registry; it is
	// handled here because "save this as a PDF" is an obvious thing to ask,
	// and the obvious thing to do with a finished score.
	switch strings.ToLower(ext) {
	case ".svg":
		return a.renderTo(out, r)
	case ".html", ".htm":
		return a.renderPage(out, r)
	case ".pdf":
		return a.renderPDF(out, r)
	case ".flac":
		// A recording, which is the one export that needs something gscore
		// does not carry: the instruments themselves.
		a.mu.RLock()
		score := a.score
		a.mu.RUnlock()
		return recordFLAC(out, score, readRecording(r.URL.Query()))
	case ".mid", ".midi":
		// Nor is a performance. This is the music as it is played rather
		// than as it is written, which is what another program wants when
		// it is going to make a sound out of it.
		a.mu.RLock()
		score := a.score
		a.mu.RUnlock()
		return MIDIFile(out, score)
	}

	a.mu.RLock()
	score, symbols := a.score, a.symbols
	a.mu.RUnlock()

	if strings.EqualFold(ext, ".gscore") {
		// The one format that can carry the drawings gets handed them, and
		// a PDF of the score in front of them unless that was turned off.
		// Renamed .pdf, the same file opens in anything that reads one.
		var preview []byte
		// The setting decides unless this one save says otherwise.
		want := SaveScorePDF()
		if v := r.URL.Query().Get("pdf"); v != "" {
			want = v != "0"
		}
		if want {
			var buf bytes.Buffer
			if err := a.renderPDF(&buf, r); err != nil {
				return err
			}
			preview = buf.Bytes()
		}
		return gscorefmt.WriteWithPDF(out, score, symbols, preview)
	}

	exp, err := format.ExporterFor("score" + ext)
	if err != nil {
		return unwritable{err}
	}
	return exp.Export(out, score)
}

// sendDocument hands the document back for the browser to keep.
//
// A page has no disk, so every save there is a save-as and the panel that
// asks where it goes belongs to the browser. What it gets is byte for byte
// what the desktop would have written.
func (a *App) sendDocument(w http.ResponseWriter, r *http.Request) {
	ext := "." + strings.TrimPrefix(r.URL.Query().Get("as"), ".")
	if ext == "." {
		ext = ".gscore"
	}
	name := a.suggestedBase() + ext

	var buf bytes.Buffer
	if err := a.writeDocument(&buf, ext, r); err != nil {
		http.Error(w, err.Error(), statusFor(err))
		return
	}
	// Sniffed rather than declared: half of what comes through here is a
	// zip and half is text, and a zip announced as text arrives broken.
	w.Header().Set("Content-Type", http.DetectContentType(buf.Bytes()))
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", name))
	w.Write(buf.Bytes())

	if savesTheScore(ext) {
		a.lock()
		a.unsaved = false
		a.mu.Unlock()
	}
}

// savesTheScore says whether writing this is saving the music or taking a
// picture of it. A drawing, a recording and a performance are not the
// document, and writing one leaves the edits still unsaved.
func savesTheScore(ext string) bool {
	switch strings.ToLower(ext) {
	case ".svg", ".html", ".htm", ".pdf", ".flac", ".mid", ".midi":
		return false
	}
	return true
}

// unwritable marks a format gscore does not write, which is something the
// caller asked for rather than something that went wrong inside.
type unwritable struct{ error }

func statusFor(err error) int {
	var u unwritable
	if errors.As(err, &u) {
		return http.StatusBadRequest
	}
	// Somewhere that cannot be written to is the request's fault, not the
	// program's: a read-only folder, or one that is not there.
	var w writeRefused
	if errors.As(err, &w) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// paperFor turns a score's page setup into the renderer's options.
//
// The model keeps millimetres, because that is what paper is specified in
// and what the dialog shows; the renderer works in points. The conversion
// happens here, once, at the edge between them — which is the only place
// it can happen without one of the two having to know about the other's
// units.
//
// A score that has chosen nothing comes back with the renderer's own
// defaults untouched, so nothing moves on a file written before any of
// this existed.
func paperFor(page model.Page) gpdf.Options {
	o := gpdf.DefaultOptions()
	if page.IsZero() {
		return o
	}
	w, h := page.Size()
	o.PageWidth, o.PageHeight = model.MMToPoints(w), model.MMToPoints(h)

	left, right, top, bottom := page.Margins()
	o.MarginLeft = model.MMToPoints(left)
	o.MarginRight = model.MMToPoints(right)
	o.MarginTop = model.MMToPoints(top)
	o.MarginBottom = model.MMToPoints(bottom)

	// Zero means "choose one and shrink it to fit", which is what the
	// renderer already does when it is not told.
	if page.StaffSpace > 0 {
		o.StaffSpace = page.StaffSpace
	}
	return o
}
