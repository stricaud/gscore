/*
 * filedialog.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
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

// pickFile shows the system's open dialog and returns the chosen path.
func pickFile() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// AppleScript gives a real Finder panel with no extra dependency.
		//
		// No "of type" filter: that clause wants uniform type identifiers, and
		// handing it bare extensions makes the dialog match nothing and fail,
		// which looks exactly like the user cancelling.
		script := `POSIX path of (choose file with prompt "Open score")`
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
			if tool == "zenity" {
				cmd = exec.Command(path, "--file-selection", "--title=Open score")
			} else {
				cmd = exec.Command(path, "--getopenfilename")
			}
			out, err := cmd.Output()
			if err != nil {
				return "", nil // cancelled
			}
			return strings.TrimSpace(string(out)), nil
		}
		return "", errNoDialog

	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms;` +
			`$d = New-Object System.Windows.Forms.OpenFileDialog;` +
			`$d.Filter = 'Scores|*.abc;*.mscz;*.mscx;*.musicxml;*.xml;*.mxl;*.gscore;*.nwctxt;*.nwc;*.ly|All files|*.*';` +
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
	a.mu.Lock()
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
			http.Error(w, "say where to save it", http.StatusBadRequest)
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

	a.mu.RLock()
	score, symbols := a.score, a.symbols
	a.mu.RUnlock()

	f, err := os.Create(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()

	// A drawing is not a score format, so it is not in the registry; it is
	// handled here because "save this as a PDF" is an obvious thing to ask,
	// and the obvious thing to do with a finished score.
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		if err := a.renderTo(f, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "path": path})
		return
	case ".html", ".htm":
		if err := a.renderPage(f, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "path": path})
		return
	case ".pdf":
		if err := a.renderPDF(f, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "path": path})
		return
	case ".flac":
		// A recording, which is the one export that needs something gscore
		// does not carry: the instruments themselves.
		a.mu.RLock()
		score := a.score
		a.mu.RUnlock()
		if err := recordFLAC(f, score, readRecording(r.URL.Query())); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "path": path})
		return
	case ".mid", ".midi":
		// Nor is a performance. This is the music as it is played rather
		// than as it is written, which is what another program wants when
		// it is going to make a sound out of it.
		a.mu.RLock()
		err := MIDIFile(f, a.score)
		a.mu.RUnlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "path": path})
		return
	}

	if strings.EqualFold(filepath.Ext(path), ".gscore") {
		// The one format that can carry the drawings gets handed them, and
		// a PDF of the score in front of them unless that was turned off.
		// Renamed .pdf, the same file opens in anything that reads one.
		var preview []byte
		if r.URL.Query().Get("pdf") != "0" {
			var buf bytes.Buffer
			if err := a.renderPDF(&buf, r); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			preview = buf.Bytes()
		}
		if err := gscorefmt.WriteWithPDF(f, score, symbols, preview); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		exp, err := format.ExporterFor(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := exp.Export(f, score); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	a.mu.Lock()
	a.filename = path
	a.mu.Unlock()
	// A score saved is a score to be reopened. The drawings and recordings
	// that came out of gscore are not, and they return above without
	// reaching this.
	a.remember(path)
	writeJSON(w, map[string]string{"status": "ok", "path": path})
}

// renderTo draws the score as a standalone SVG, with the font inside it so
// the file opens correctly anywhere rather than only where gscore is running.
func (a *App) renderTo(w io.Writer, r *http.Request) error {
	a.mu.RLock()
	score := a.score
	eopt := a.engraveOpt
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
	eopt := a.engraveOpt
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
	a.mu.RUnlock()

	q := r.URL.Query()

	popt := gpdf.DefaultOptions()
	if q.Get("paper") == "letter" {
		popt = gpdf.Letter()
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
	eopt.ShowPartNames = len(score.Parts) > 1
	// Not fitted here: what the paper says about type size and line length
	// is settled per piece below, because the pieces are different sizes of
	// music.

	// Each piece is fitted to its own paper. A document of several pieces
	// is not one drawing in several parts: a book's exercises differ from
	// one another, and a part differs from the score it came out of by an
	// order of magnitude — seventeen staves against one.
	fit := func(sc *model.Score, o engrave.Options, paper gpdf.Options) gpdf.Section {
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
			o := eopt
			o.ShowPartNames = len(sc.Parts) > 1
			secs = append(secs, fit(sc, o, popt))
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
		script := `tell application "System Events" to set frontmost of ` +
			`(first process whose unix id is ` + strconv.Itoa(os.Getpid()) + `) to true
POSIX path of (choose file name with prompt "Save score" default name "` +
			strings.ReplaceAll(suggested, `"`, "") + `")`

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
			`$d.FileName = '` + suggested + `';` +
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

// panelShownAfter is how long a system panel has to have been up for somebody
// to have had a chance to cancel it. Anything faster never appeared.
const panelShownAfter = 400 * time.Millisecond

// saveDirectory is where a file typed by name rather than chosen in a panel
// should land: beside the score it came from, or in the home directory.
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
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
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
