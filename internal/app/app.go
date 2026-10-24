/*
 * app.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package app is the gscore application: a window, a menu, a toolbox and a
// score.
//
// The interface is served to a web view rather than drawn with a desktop
// toolkit. That is what makes the same code run on macOS, Windows and Linux
// without a toolkit that has to be ported, and it lets the score be drawn as
// SVG, which zooms and prints without being redrawn.
//
// The 0.0.9 README called the interface "a la gimp" and said the program was
// "designed to be easy to use and tune". That is the brief: a toolbox you
// click, a score you click on, and no dialog in the way of writing a note.
package app

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/stricaud/gscore/internal/brand"
	"github.com/stricaud/gscore/internal/desktop"
	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/render/svg"
	"github.com/stricaud/gscore/internal/symbol"
	"github.com/stricaud/gscore/internal/version"
)

//go:embed ui
var uiFiles embed.FS

// App holds the running editor's state.
type App struct {
	mu    sync.RWMutex
	score *model.Score
	// book holds every score in the file, since one file may carry many: an
	// ABC tune book, for instance. score is whichever of them is being
	// worked on.
	book     []*model.Score
	current  int
	filename string

	// clip is what was last copied, kept here because the music is here.
	clip clipboard
	// undoStack and redoStack hold the score as it was before and after the
	// edits that have been taken back. See undo.go for why they are whole
	// copies rather than a list of changes.
	undoStack []snapshot
	redoStack []snapshot
	// warning holds what an importer could not fully read, so the interface
	// can say so instead of quietly showing half a score.
	warning string
	// importLog holds every line the importer could not read, one entry each.
	// A tune book is written by hand over years and has mistakes in it; the
	// person who wrote it is the one who can fix them, so they get the whole
	// list rather than a count.
	importLog []string

	// symbols holds the user's own drawings, which the renderer needs in
	// order to draw anything the music font does not carry.
	symbols *symbol.Registry

	engraveOpt engrave.Options
	renderOpt  svg.Options

	// local records that the server is only reachable from this machine, which
	// is what makes it safe to show a file dialog or open a path.
	local bool

	// native records that the interface sits in a real application window.
	// On macOS the window title already carries the application name, so the
	// interface must not repeat it the way a web page would.
	native bool
}

// New returns an app showing the given score.
func New(s *model.Score, filename string) *App {
	if s == nil {
		s = model.NewScore()
	}
	return NewBook([]*model.Score{s}, filename)
}

// Opened records what the importer could not read in the file the app was
// started on.
//
// That file is opened before the window exists, so its partial read has
// nowhere to go until now. Without this the log window was empty for exactly
// the case it was written for: a tune book opened from the command line.
func (a *App) Opened(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.warning, a.importLog = readProblems(err)
}

// NewBook returns an app showing a file that may hold several scores.
func NewBook(scores []*model.Score, filename string) *App {
	if len(scores) == 0 {
		scores = []*model.Score{model.NewScore()}
	}
	s := scores[0]
	ro := svg.DefaultOptions()
	// The font is served once as its own file rather than embedded in every
	// redraw of the score.
	ro.EmbedFont = false
	ro.Interactive = true

	eo := engrave.DefaultOptions()
	// The continuous strip is gscore's own view and the one it was built
	// around; pagination is for printing. The editor opens in the strip.
	eo.Mode = engrave.Strip

	reg := symbol.NewRegistry()
	ro.Symbols = reg

	return &App{
		score:      s,
		book:       scores,
		filename:   filename,
		symbols:    reg,
		engraveOpt: eo,
		renderOpt:  ro,
	}
}

// UIMode selects how the interface is shown.
type UIMode int

const (
	// Native opens a real application window around the interface. This is
	// the default: gscore is an application, not a browser tab.
	Native UIMode = iota
	// Browser serves the interface and opens the default browser at it.
	Browser
	// Headless serves it and opens nothing, for a remote or embedded use.
	Headless
)

// Run starts the server and shows the interface.
//
// The server runs whichever mode is chosen, because the same pages are what
// make a score embeddable in a web page. Only the window around them changes.
//
// addr is a listen address; an empty one means a free port on the loopback
// interface, which is what a desktop window wants.
// Serve starts the server and comes straight back, saying where it is.
//
// It is Run without the window, for a shell that brings its own: the macOS
// application in ui/macosx owns its menu bar, its panels and its event loop,
// and wants the pages served into a web view it created itself rather than a
// window opened for it. Everything behind the pages is the same server —
// there is one editor, not a desktop one and a web one.
//
// native says the interface is inside an application window rather than a
// browser tab, which is what lets it offer a save panel instead of a
// download.
func (a *App) Serve(addr string, native bool) (string, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("gscore: cannot listen on %s: %w", addr, err)
	}

	url := "http://" + ln.Addr().String() + "/"
	if host, port, err := net.SplitHostPort(ln.Addr().String()); err == nil {
		if host == "::" || host == "0.0.0.0" {
			// Printing the wildcard back is useless; name a host that can
			// actually be typed, and say the server is reachable off-machine.
			url = "http://localhost:" + port + "/"
		}
	}

	mux := http.NewServeMux()
	a.routes(mux)

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
		}
	}()

	host, _, _ := net.SplitHostPort(ln.Addr().String())
	a.mu.Lock()
	a.native = native
	a.local = host == "127.0.0.1" || host == "::1"
	a.mu.Unlock()
	return url, nil
}

func (a *App) Run(addr string, mode UIMode) error {
	url, err := a.Serve(addr, mode == Native && desktop.Available)
	if err != nil {
		return err
	}

	fmt.Printf("gscore %s — %s\n", version.Version, url)
	if h, _, err := net.SplitHostPort(strings.TrimSuffix(strings.TrimPrefix(url, "http://"), "/")); err == nil &&
		h != "127.0.0.1" && h != "::1" && h != "localhost" {
		// The editor can change the score, so saying who can reach it matters.
		fmt.Println("listening on all interfaces: anyone who can reach this port can edit the score")
	}

	title := brand.Name
	if a.filename != "" {
		title = filepath.Base(a.filename) + " — " + brand.Name
	}

	switch mode {
	case Native:
		if !desktop.Available {
			fmt.Println("no native window in this build; opening a browser instead")
			launch(url)
			select {} // keep serving
		}
		// The window owns the main goroutine until it is closed, and closing
		// it ends the program, the way shutting an application window should.
		return desktop.Open(url, title, 1280, 820)
	case Browser:
		launch(url)
		select {}
	default:
		fmt.Println("open that address to use the editor")
		select {}
	}
}

func launch(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("could not open a browser (%v); open %s yourself\n", err, url)
	}
}

func (a *App) routes(mux *http.ServeMux) {
	ui, err := fsSub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(ui)))
	// The mascot is served from the one place he is kept, rather than from a
	// second copy sitting in the interface's own files: the PDF writer
	// stamps the same drawing on paper and the two must not drift apart.
	mux.HandleFunc("/gilbert.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(brand.GilbertPNG)
	})
	mux.HandleFunc("/gilbert.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(brand.GilbertSVG)
	})
	a.APIRoutes(mux)
}

// APIRoutes registers everything but the interface's own files.
//
// The browser build serves the HTML, CSS and JavaScript as static files and
// runs this same set of handlers inside WebAssembly, so a score behaves
// identically whether gscore is an application or a page.
func (a *App) APIRoutes(mux *http.ServeMux) {
	// Reading: drawings, descriptions, and the pickers. Nothing here changes
	// the music, so nothing here needs to be undoable.
	mux.HandleFunc("/score.svg", a.handleSVG)
	mux.HandleFunc("/font.otf", a.handleFont)
	mux.HandleFunc("/symbol.png", a.handleSymbolPNG)
	mux.HandleFunc("/keysig.svg", a.handleKeySigSVG)
	mux.HandleFunc("/timesig.svg", a.handleTimeSigSVG)
	mux.HandleFunc("/print", a.handlePrint)
	mux.HandleFunc("/api/pdf", a.handlePDF)
	mux.HandleFunc("/api/page", a.handlePage)
	mux.HandleFunc("/api/midi", a.handleMIDI)
	mux.HandleFunc("/api/record", a.handleRecord)
	mux.HandleFunc("/api/soundfonts", a.handleSoundfonts)
	mux.HandleFunc("/api/soundfont/fetch", a.handleSoundfontFetch)
	mux.HandleFunc("/api/soundfont/pick", a.handleSoundfontPick)
	mux.HandleFunc("/api/soundfont/presets", a.handleSoundfontPresets)
	mux.HandleFunc("/api/info", a.handleInfo)
	mux.HandleFunc("/api/layout", a.handleLayout)
	mux.HandleFunc("/api/hits", a.handleHits)
	mux.HandleFunc("/api/playback", a.handlePlayback)
	mux.HandleFunc("/api/keysignatures", a.handleKeySignatures)
	mux.HandleFunc("/api/chordpreview", a.handleChordPreview)
	mux.HandleFunc("/api/chordfingering", a.handleChordFingering)
	mux.HandleFunc("/api/instruments", a.handleInstruments)
	mux.HandleFunc("/api/percussion", a.handlePercussion)
	mux.HandleFunc("/api/getlyrics", a.handleGetLyrics)
	mux.HandleFunc("/api/symbols", a.handleSymbols)
	mux.HandleFunc("/api/copy", a.handleCopy)
	mux.HandleFunc("/api/pickfile", a.handlePickFile)
	mux.HandleFunc("/api/save", a.handleSave)
	mux.HandleFunc("/api/exportsymbols", a.handleExportSymbols)
	mux.HandleFunc("/api/openbrowser", a.handleOpenBrowser)

	// How the score is drawn rather than what it says, and which of a book's
	// tunes is in front of you. Taking these back with undo would be
	// surprising: nobody thinks of turning staff names on as an edit.
	// Where the cursor is. Not an edit — it changes nothing about the music
	// — so it is not in the undo block below.
	mux.HandleFunc("/api/cursor", a.handleCursor)

	mux.HandleFunc("/api/view", a.handleView)
	mux.HandleFunc("/api/tune", a.handleTune)

	// Opening something else. The edits before it belong to a score that is
	// no longer here, so the history goes with it.
	mux.HandleFunc("/api/new", a.handleNew)
	mux.HandleFunc("/api/open", a.handleOpen)
	mux.HandleFunc("/api/openpath", a.handleOpenPath)
	mux.HandleFunc("/api/importsymbols", a.handleImportSymbols)
	mux.HandleFunc("/api/symbolimport", a.handleSymbolImport)

	mux.HandleFunc("/api/undo", a.handleUndo)
	mux.HandleFunc("/api/redo", a.handleRedo)

	// Everything that changes the music. Each is wrapped so that it can be
	// taken back, which is why they are listed together: a route that is not
	// in this block is a route whose edits cannot be undone, and that is
	// visible here rather than hidden in the handler.
	for path, h := range map[string]http.HandlerFunc{
		"/api/insert":          a.handleInsert,
		"/api/addpitch":        a.handleAddPitch,
		"/api/delete":          a.handleDelete,
		"/api/deleteselection": a.handleDeleteSelection,
		"/api/paste":           a.handlePaste,
		"/api/nudge":           a.handleNudge,
		"/api/addmeasure":      a.handleAddMeasure,
		"/api/measurerepeat":   a.handleMeasureRepeat,
		"/api/barline":         a.handleBarline,
		"/api/beam":            a.handleBeam,
		"/api/slur":            a.handleSlur,
		"/api/tie":             a.handleTie,
		"/api/stem":            a.handleStem,
		"/api/dot":             a.handleDot,
		"/api/tuplet":          a.handleTuplet,
		"/api/tremolo":         a.handleTremolo,
		"/api/accidental":      a.handleAccidental,
		"/api/articulation":    a.handleArticulation,
		"/api/dynamic":         a.handleDynamic,
		"/api/hairpin":         a.handleHairpin,
		"/api/octave":          a.handleOctave,
		"/api/ending":          a.handleEnding,
		"/api/jump":            a.handleJump,
		"/api/marker":          a.handleMarker,
		"/api/text":            a.handleText,
		"/api/lyrics":          a.handleLyrics,
		"/api/chordsymbol":     a.handleChordSymbol,
		"/api/setkey":          a.handleSetKey,
		"/api/tempo":           a.handleTempo,
		"/api/header":          a.handleHeader,
		"/api/staff":           a.handleStaff,
		"/api/addstaff":        a.handleAddStaff,
		"/api/removestaff":     a.handleRemoveStaff,
		"/api/movestaff":       a.handleMoveStaff,
		"/api/group":           a.handleGroup,
		"/api/insertsymbol":    a.handleInsertSymbol,
		"/api/movesymbol":      a.handleMoveSymbol,
	} {
		mux.HandleFunc(path, a.undoable(h))
	}
}

func (a *App) handleSVG(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	score := a.score
	if v := r.URL.Query().Get("tune"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i >= 0 && i < len(a.book) {
			score = a.book[i]
		}
	}
	eopt := a.engraveOpt
	ropt := a.renderOpt
	a.mu.RUnlock()

	if v := r.URL.Query().Get("space"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 3 && f <= 40 {
			ropt.StaffSpace = f
		}
	}
	switch r.URL.Query().Get("mode") {
	case "strip":
		eopt.Mode = engrave.Strip
	case "page":
		eopt.Mode = engrave.Paginated
	}
	if v := r.URL.Query().Get("width"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 40 && f <= 600 {
			eopt.LineWidth = f
		}
	}

	a.mu.RLock()
	ropt.Symbols = a.symbols
	a.mu.RUnlock()

	lay := engrave.Engrave(score, eopt)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	if err := svg.Render(w, lay, ropt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) handleFont(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "font/otf")
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(fontData())
}

// tuneInfo names one score in a file that holds several.
type tuneInfo struct {
	Index int    `json:"index"`
	Title string `json:"title"`
}

type info struct {
	Warning string `json:"warning,omitempty"`
	// Log lists the lines the importer could not read, for the log window.
	Log   []string   `json:"log,omitempty"`
	Tunes []tuneInfo `json:"tunes,omitempty"`
	// CanUndo and CanRedo grey the Edit menu entries, so the menu says what
	// it will do rather than looking the same whether it will do it or not.
	CanUndo  bool       `json:"canUndo"`
	CanRedo  bool       `json:"canRedo"`
	Current  int        `json:"current"`
	Native   bool       `json:"native"`
	Local    bool       `json:"local"`
	Version  string     `json:"version"`
	Filename string     `json:"filename"`
	Title    string     `json:"title"`
	Subtitle string     `json:"subtitle"`
	Composer string     `json:"composer"`
	Tempo    int        `json:"tempo"`
	Parts    []partInfo `json:"parts"`
	Formats  []string   `json:"formats"`

	// The beat the tempo counts and the words beside it. A number on its own
	// does not say anything: 120 in 6/8 is a different speed depending on
	// whether it is counting quarters or dotted quarters.
	TempoUnit   string `json:"tempoUnit"`
	TempoDotted bool   `json:"tempoDotted,omitempty"`
	TempoText   string `json:"tempoText,omitempty"`

	// TempoChanges are the speeds written partway through, keyed by the bar
	// they start at. Bar zero is the score's own, which is why it is not
	// here: the mark in the title block is that marking written over bar one.
	TempoChanges map[string]tempoInfo `json:"tempoChanges,omitempty"`

	// Cursor is where the writing cursor belongs when the score opens: where
	// it was left if the file recorded one, and otherwise at the end of the
	// first staff, which is where somebody opening a piece to add to it
	// wants to be.
	Cursor cursorInfo `json:"cursor"`
}

// cursorInfo is where the writing cursor goes when the score opens.
type cursorInfo struct {
	Part    int `json:"part"`
	Measure int `json:"measure"`
	At      int `json:"at"`
	Step    int `json:"step"`
}

// tempoInfo is one metronome mark as the interface needs it.
type tempoInfo struct {
	BPM    int    `json:"bpm"`
	Unit   string `json:"unit"`
	Dotted bool   `json:"dotted,omitempty"`
	Text   string `json:"text,omitempty"`
}

type partInfo struct {
	Name       string `json:"name"`
	Instrument string `json:"instrument"`
	Clef       string `json:"clef"`
	Key        string `json:"key"`
	Time       string `json:"time"`
	Measures   int    `json:"measures"`
	Notes      int    `json:"notes"`
	// TransposeSemitones is how far below what is written this staff sounds,
	// so the editor can get back from a sounding note to the printed one.
	// A fingering chart needs the printed one: fingers follow the page.
	TransposeSemitones int `json:"transposeSemitones"`
	// Hidden says the staff is out of the drawing but still in the score.
	Hidden bool `json:"hidden,omitempty"`
}

func (a *App) handleInfo(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := info{
		Warning:  a.warning,
		Log:      a.importLog,
		Current:  a.current,
		Native:   a.native,
		Local:    a.local,
		Version:  version.Version,
		Filename: a.filename,
		CanUndo:  len(a.undoStack) > 0,
		CanRedo:  len(a.redoStack) > 0,
		Title:    a.score.Identity.Title,
		Subtitle: a.score.Identity.Subtitle,
		Composer: a.score.Identity.Composer,
		Tempo:    a.score.Tempo.BPM,
	}
	out.Cursor = a.cursorInfo()
	out.TempoUnit, out.TempoDotted = beatName(a.score.Tempo.Unit)
	out.TempoText = a.score.Tempo.Text
	for i := 0; i < a.score.MeasureCount(); i++ {
		t := a.score.TempoChangeAt(i)
		if t == nil || i == 0 {
			continue
		}
		unit, dotted := beatName(t.Unit)
		if out.TempoChanges == nil {
			out.TempoChanges = map[string]tempoInfo{}
		}
		out.TempoChanges[strconv.Itoa(i)] = tempoInfo{
			BPM: t.BPM, Unit: unit, Dotted: dotted, Text: t.Text,
		}
	}
	for _, i := range format.Importers() {
		out.Formats = append(out.Formats, i.Name())
	}
	// Only worth listing when there is a choice to make.
	if len(a.book) > 1 {
		for i, sc := range a.book {
			title := sc.Identity.Title
			if title == "" {
				title = fmt.Sprintf("Tune %d", i+1)
			}
			out.Tunes = append(out.Tunes, tuneInfo{Index: i, Title: title})
		}
	}
	for _, p := range a.score.Parts {
		name := p.Name
		if name == "" && p.ID != "" {
			name = "Voice " + p.ID
		}
		if name == "" {
			name = "Staff"
		}
		notes := 0
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				notes += len(v.Elements)
			}
		}
		// A drum staff has no General MIDI program; saying "piano" because
		// that is what the unused field holds would be a lie the inspector
		// tells every time it is opened.
		instrument := p.Instrument.String()
		if p.Clef == model.Percussion {
			instrument = drumKitName
		}
		out.Parts = append(out.Parts, partInfo{
			Name: name, Instrument: instrument,
			Clef: p.Clef.String(), Key: keyText(p.Key),
			Time: p.Time.String(), Measures: len(p.Measures), Notes: notes,
			TransposeSemitones: p.Transpose.Semitones(),
			Hidden:             p.Hidden,
		})
	}
	writeJSON(w, out)
}

func keyText(k model.KeySignature) string {
	switch {
	case k.Fifths > 0:
		return fmt.Sprintf("%d♯ %s", k.Fifths, k.Mode)
	case k.Fifths < 0:
		return fmt.Sprintf("%d♭ %s", -k.Fifths, k.Mode)
	}
	return "♮ " + k.Mode.String()
}

// setBook replaces what is open. The caller holds the lock.
//
// The undo history goes with it. The edits before this point belong to a
// score that is no longer here, and undoing one of them would put somebody
// else's music back over the file just opened. Clearing it here, where the
// document is replaced, is why opening a file by any of the three routes
// cannot forget to.
func (a *App) setBook(scores []*model.Score, filename string, err error) {
	a.book = scores
	a.current = 0
	a.score = scores[0]
	a.filename = filename
	a.warning, a.importLog = readProblems(err)
	a.forgetHistory()
}

// readProblems splits an importer's error into the one line the status bar
// shows and the whole list the log window shows.
//
// An importer that read most of a file reports a partial read, which carries
// the lines it had to pass over. Those lines used to be reduced to the first
// one before anybody saw them, and the other thirty-nine were lost — which
// left the person who could have corrected the file with nothing to go on.
func readProblems(err error) (string, []string) {
	if err == nil {
		return "", nil
	}
	var partial format.Partial
	if errors.As(err, &partial) {
		return partial.Error(), partial.Details()
	}
	return err.Error(), []string{err.Error()}
}

// handleTune switches to another score in the same file.
func (a *App) handleTune(w http.ResponseWriter, r *http.Request) {
	i, err := strconv.Atoi(r.URL.Query().Get("i"))
	if err != nil {
		http.Error(w, "which tune?", http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if i < 0 || i >= len(a.book) {
		http.Error(w, "no such tune", http.StatusBadRequest)
		return
	}
	a.current = i
	a.score = a.book[i]
	writeJSON(w, map[string]any{"status": "ok", "current": i})
}

func (a *App) handleNew(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.setBook([]*model.Score{model.NewScore()}, "", nil)
	a.mu.Unlock()
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleOpen takes the file's bytes rather than a path, because a browser
// never hands over a path.
func (a *App) handleOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "untitled.abc"
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	imp, err := format.ImporterFor(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	scores, err := format.ImportAll(imp, bytesReader(body))
	if len(scores) == 0 {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	a.setBook(scores, name, err)
	a.mu.Unlock()
	writeJSON(w, map[string]any{"status": "ok", "filename": name, "tunes": len(scores)})
}

// handleLayout reports the engraved size, which the canvas uses to size itself.
func (a *App) handleLayout(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lay := engrave.Engrave(a.score, a.engraveOpt)
	a.mu.RUnlock()
	writeJSON(w, map[string]float64{"width": lay.Width, "height": lay.Height})
}

// handlePrint serves the score as a page ready to print.
//
// The native window cannot print: a web view has no print machinery behind
// it, and window.open returns nothing without a UI delegate. So printing
// hands the job to the real browser, which has both.
// handlePrint builds a page that prints itself.
//
// What goes on it is a choice, because one score is several documents. A tune
// book is a book, and printing the exercise you happen to be looking at is
// almost never what was wanted. A piece for several instruments is a conductor
// score plus one part for each player, and a player reading off the full score
// is reading nine staves to follow one.
//
//	what=tune   the tune on screen            (the default)
//	what=book   every tune in the file, one after another
//	what=parts  the full score, then each staff alone on its own page
//
// There is no PDF writer here and there does not need to be one: every system
// this runs on prints to PDF from the print dialog, which is also where the
// paper size, the margins and the page range already live. Writing a second,
// worse version of that dialog would be the wrong half of the job.
func (a *App) handlePrint(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	score := a.score
	book := a.book
	symbols := a.symbols
	a.mu.RUnlock()

	what := r.URL.Query().Get("what")

	type section struct {
		heading string
		score   *model.Score
	}
	var sections []section

	switch what {
	case "book":
		for _, sc := range book {
			sections = append(sections, section{score: sc})
		}
	case "parts":
		sections = append(sections, section{score: score})
		if len(score.Parts) > 1 {
			for _, p := range score.Parts {
				sections = append(sections, section{heading: partLabel(p), score: onePart(score, p)})
			}
		}
	default:
		sections = append(sections, section{score: score})
	}
	if len(sections) == 0 {
		sections = append(sections, section{score: score})
	}

	title := score.Identity.Title
	if what == "book" && len(book) > 1 {
		a.mu.RLock()
		title = a.filename
		a.mu.RUnlock()
	}
	if title == "" {
		title = "Score"
	}

	// Printing always paginates, whatever the editor is showing: a
	// continuous strip has no page breaks to print at.
	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.LineWidth = 150

	ropt := svg.DefaultOptions()
	ropt.Symbols = symbols
	// The font goes in once, on the first drawing. Embedding it in each of
	// forty exercises would make a page of some megabytes that no browser
	// will print quickly.
	ropt.EmbedFont = true
	ropt.StaffSpace = 6

	var body strings.Builder
	for i, sec := range sections {
		if i == 1 {
			ropt.EmbedFont = false
		}
		var drawing strings.Builder
		if err := svg.Render(&drawing, engrave.Engrave(sec.score, eopt), ropt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body.WriteString(`<section>`)
		if sec.heading != "" {
			fmt.Fprintf(&body, `<h2>%s</h2>`, html.EscapeString(sec.heading))
		}
		body.WriteString(drawing.String())
		body.WriteString(`</section>`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html>
<html><head><meta charset="utf-8"><title>%s</title>
<style>
  @page { size: auto; margin: 12mm; }
  html, body { margin: 0; padding: 0; background: #fff; }
  svg { display: block; width: 100%%; height: auto; }
  /* Each tune, and each extracted part, starts its own page. */
  section { break-before: page; page-break-before: always; }
  section:first-of-type { break-before: auto; page-break-before: auto; }
  h2 { font: 600 14pt Georgia, "Times New Roman", serif; margin: 0 0 6mm; }
  @media screen { body { padding: 16px; } section { margin-bottom: 24px; } }
</style></head>
<body>%s<script>
  // Wait for the embedded font, or the first page prints blank.
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(function () { setTimeout(function () { window.print(); }, 150); });
  } else {
    setTimeout(function () { window.print(); }, 600);
  }
</script></body></html>`, html.EscapeString(title), body.String())
}

// partLabel names a staff on its own page, so a player can tell which sheet
// is theirs without reading the music.
func partLabel(p *model.Part) string {
	switch {
	case p.Name != "":
		return p.Name
	case p.Instrument != 0:
		return p.Instrument.String()
	}
	return "Part"
}

// onePart makes a score holding a single staff, which is what a player reads.
//
// Nothing is copied deeply: the measures are shared with the score they came
// from, and the result is engraved and thrown away within the request.
func onePart(s *model.Score, p *model.Part) *model.Score {
	out := *s
	out.Parts = []*model.Part{p}
	if p.Name != "" {
		out.Identity.Subtitle = p.Name
	}
	return &out
}

// handleOpenBrowser opens a page of gscore's own in the system browser.
func (a *App) handleOpenBrowser(w http.ResponseWriter, r *http.Request) {
	if !a.local {
		http.Error(w, "only on this machine", http.StatusForbidden)
		return
	}
	path := r.URL.Query().Get("path")
	if !strings.HasPrefix(path, "/") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	launch("http://" + r.Host + path)
	writeJSON(w, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// cursorInfo is where the writing cursor should be when the score opens.
//
// Where it was left, if the file said — a .gscore records it. A file from
// anywhere else does not, and then the answer is the end of the first staff:
// a new score has nothing in it and the cursor belongs at the start, and a
// piece somebody opens to add to is a piece they want to carry on writing.
// Either way there is a cursor on the page from the moment it opens, which
// is the difference between an editor and a viewer.
func (a *App) cursorInfo() cursorInfo {
	if c := a.score.Cursor; c != nil && c.Part < len(a.score.Parts) {
		return cursorInfo{Part: c.Part, Measure: c.Measure, At: c.At, Step: c.Step}
	}
	if len(a.score.Parts) == 0 {
		return cursorInfo{}
	}
	p := a.score.Parts[0]
	last := len(p.Measures) - 1
	if last < 0 {
		return cursorInfo{}
	}
	at := 0
	if len(p.Measures[last].Voices) > 0 {
		at = len(p.Measures[last].Voices[0].Elements)
	}
	return cursorInfo{Measure: last, At: at, Step: middleStaffStep}
}

// middleStaffStep is the middle line, which is where a cursor with no reason
// to be anywhere else sits.
const middleStaffStep = 4
