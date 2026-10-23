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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

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
	// warning holds what an importer could not fully read, so the interface
	// can say so instead of quietly showing half a score.
	warning string

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
func (a *App) Run(addr string, mode UIMode) error {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("gscore: cannot listen on %s: %w", addr, err)
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

	fmt.Printf("gscore %s — %s\n", version.Version, url)
	if h, _, err := net.SplitHostPort(ln.Addr().String()); err == nil && h != "127.0.0.1" && h != "::1" {
		// The editor can change the score, so saying who can reach it matters.
		fmt.Println("listening on all interfaces: anyone who can reach this port can edit the score")
	}

	title := "gscore"
	if a.filename != "" {
		title = filepath.Base(a.filename) + " — gscore"
	}

	host, _, _ := net.SplitHostPort(ln.Addr().String())
	a.mu.Lock()
	a.native = mode == Native && desktop.Available
	a.local = host == "127.0.0.1" || host == "::1"
	a.mu.Unlock()

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
	a.APIRoutes(mux)
}

// APIRoutes registers everything but the interface's own files.
//
// The browser build serves the HTML, CSS and JavaScript as static files and
// runs this same set of handlers inside WebAssembly, so a score behaves
// identically whether gscore is an application or a page.
func (a *App) APIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/score.svg", a.handleSVG)
	mux.HandleFunc("/font.otf", a.handleFont)
	mux.HandleFunc("/api/info", a.handleInfo)
	mux.HandleFunc("/api/new", a.handleNew)
	mux.HandleFunc("/api/open", a.handleOpen)
	mux.HandleFunc("/api/layout", a.handleLayout)
	mux.HandleFunc("/api/playback", a.handlePlayback)
	mux.HandleFunc("/api/keysignatures", a.handleKeySignatures)
	mux.HandleFunc("/api/setkey", a.handleSetKey)
	mux.HandleFunc("/keysig.svg", a.handleKeySigSVG)
	mux.HandleFunc("/api/hits", a.handleHits)
	mux.HandleFunc("/api/insert", a.handleInsert)
	mux.HandleFunc("/api/barline", a.handleBarline)
	mux.HandleFunc("/api/delete", a.handleDelete)
	mux.HandleFunc("/api/addmeasure", a.handleAddMeasure)
	mux.HandleFunc("/api/pickfile", a.handlePickFile)
	mux.HandleFunc("/api/openpath", a.handleOpenPath)
	mux.HandleFunc("/api/tune", a.handleTune)
	mux.HandleFunc("/api/symbols", a.handleSymbols)
	mux.HandleFunc("/api/insertsymbol", a.handleInsertSymbol)
	mux.HandleFunc("/symbol.png", a.handleSymbolPNG)
	mux.HandleFunc("/api/symbolimport", a.handleSymbolImport)
	mux.HandleFunc("/api/stem", a.handleStem)
	mux.HandleFunc("/api/dynamic", a.handleDynamic)
	mux.HandleFunc("/api/articulation", a.handleArticulation)
	mux.HandleFunc("/api/deleteselection", a.handleDeleteSelection)
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
	Warning  string     `json:"warning,omitempty"`
	Tunes    []tuneInfo `json:"tunes,omitempty"`
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
}

type partInfo struct {
	Name     string `json:"name"`
	Clef     string `json:"clef"`
	Key      string `json:"key"`
	Time     string `json:"time"`
	Measures int    `json:"measures"`
	Notes    int    `json:"notes"`
}

func (a *App) handleInfo(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := info{
		Warning:  a.warning,
		Current:  a.current,
		Native:   a.native,
		Local:    a.local,
		Version:  version.Version,
		Filename: a.filename,
		Title:    a.score.Identity.Title,
		Subtitle: a.score.Identity.Subtitle,
		Composer: a.score.Identity.Composer,
		Tempo:    a.score.Tempo.BPM,
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
		out.Parts = append(out.Parts, partInfo{
			Name: name, Clef: p.Clef.String(), Key: keyText(p.Key),
			Time: p.Time.String(), Measures: len(p.Measures), Notes: notes,
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
func (a *App) setBook(scores []*model.Score, filename, warning string) {
	a.book = scores
	a.current = 0
	a.score = scores[0]
	a.filename = filename
	a.warning = warning
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
	a.setBook([]*model.Score{model.NewScore()}, "", "")
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
	warning := ""
	if err != nil {
		warning = err.Error()
	}
	a.mu.Lock()
	a.setBook(scores, name, warning)
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
