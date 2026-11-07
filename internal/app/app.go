/*
 * app.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/stricaud/gscore/docs"
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

	// What was last engraved, and how many times anything has been able to
	// change since.
	//
	// Laying out a big score is seconds of work — Beethoven's ninth is
	// fifty-three staves of two and a half thousand bars and takes four and
	// a half — and the editor asks for it twice on every change: once for
	// the drawing and once for the regions that can be clicked. Opening that
	// file engraved it twice over for one picture.
	//
	// gen counts writes rather than changes. Every path that takes the write
	// lock bumps it, whether or not it touched anything, so a cached layout
	// is handed back only while nothing has had the chance to make it wrong.
	// Counting too generously costs a re-engrave; counting where the score
	// is touched instead would mean finding every place that touches it, and
	// one missed would draw a score that is no longer there.
	gen      atomic.Uint64
	layMu    sync.Mutex
	layGen   uint64
	layScore *model.Score
	layOpt   engrave.Options
	lay      *engrave.Layout
	// book holds every score in the file, since one file may carry many: an
	// ABC tune book, for instance. score is whichever of them is being
	// worked on.
	book     []*model.Score
	current  int
	filename string

	// source is the bytes the open score was read from, kept only for a PDF.
	//
	// A PDF is the one format whose reading is a judgement rather than a
	// parse: how many staves make a system is written nowhere on the page,
	// and getting it wrong turns a band score into one endless part. So the
	// reading has to be reviewable — and a review that cannot be acted on is
	// a complaint. The bytes stay here so the same file can be read again a
	// different way without being asked for a second time.
	//
	// Only for a PDF, and only the one that is open: every other format says
	// what it holds, and keeping a copy of every file ever opened would be a
	// memory leak dressed as a feature.
	source     []byte
	sourceName string
	// perSystem is the staves-per-system the open PDF was read with, so the
	// review can show what was used and offer something else.
	perSystem int

	// clip is what was last copied, kept here because the music is here.
	clip clipboard
	// undoStack and redoStack hold the score as it was before and after the
	// edits that have been taken back. See undo.go for why they are whole
	// copies rather than a list of changes.
	undoStack []snapshot
	redoStack []snapshot

	// batch is the one snapshot a running script will undo to, and
	// batchDepth how many begins are outstanding. While a batch is open
	// no edit pushes an undo of its own: a script that writes two hundred
	// notes and comes back as two hundred separate undos is a script
	// nobody will dare run.
	batch      *snapshot
	batchDepth int
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

	// folderSymbols is what was found in the symbols folder when the
	// editor started, including the files it could not read — which the
	// person wants told about, since a drawing they put in the folder
	// and cannot find is otherwise a mystery.
	folderSymbols []folderSymbol

	engraveOpt engrave.Options
	// concert says the score is being shown as it sounds rather than as
	// each player reads it, and concert0 holds that copy. See concert.go.
	concert  bool
	concert0 concertCache
	// sheet is which part is being shown, by its index in Score.Sheets, or
	// -1 for the score itself; sheet0 holds that copy. Which part is on the
	// screen belongs to the window rather than to the file — a score
	// reopened tomorrow shows the score. See sheets.go.
	sheet     int
	sheet0    sheetCache
	renderOpt svg.Options

	// inBrowser records that this core is the page: compiled to WebAssembly
	// and answering its own requests, with no server and no filesystem.
	inBrowser bool

	// local records that the server is only reachable from this machine, which
	// is what makes it safe to show a file dialog or open a path.
	local bool

	// native records that the interface sits in a real application window.
	// On macOS the window title already carries the application name, so the
	// interface must not repeat it the way a web page would.
	native bool

	// unsaved says the score has been edited since it was last written to
	// disk. Set by the wrapper every edit goes through and cleared where the
	// score is written or replaced, so no handler has to remember to.
	unsaved bool

	// mux is this document's own router, built once when the App is made.
	//
	// One router per session rather than one for the whole server, because
	// every route is a closure over this App — and that is exactly what
	// lets the hundred and twenty-eight handlers know nothing about
	// sessions at all. A router of a hundred-odd entries is a few kilobytes
	// beside a score and the layout engraved from it, which is the only
	// reason it is affordable to have one each. See session.go.
	mux *http.ServeMux

	// server is the Server this document belongs to, for the few things
	// shared between documents rather than owned by any one of them. Nil in
	// a test that builds an App on its own.
	server *Server
}

// clipboard is what this document copies into and pastes from.
//
// The server's where there is one, so that copying in one window and pasting
// in another does what anybody would expect; this document's own where there
// is not, which is every test that builds an App by itself.
func (a *App) clipboard() *clipboard {
	if a.server != nil {
		return &a.server.clip
	}
	return &a.clip
}

// New returns an app showing the given score.
func New(s *model.Score, filename string) *App {
	if s == nil {
		s = model.NewScore()
	}
	return NewBook([]*model.Score{s}, filename)
}

// InBrowser says this core is running as the page itself, compiled to
// WebAssembly. The browser build calls it on the way up.
//
// It unlocks the things that need a soundfont — playing through one and
// recording — which are otherwise kept to the machine gscore is running on.
// There is no machine here, but there is no network either: the core, the
// page and the person are all the same computer.
func (a *App) InBrowser() {
	a.lock()
	a.inBrowser = true
	a.mu.Unlock()
}

// Opened records what the importer could not read in the file the app was
// started on.
//
// That file is opened before the window exists, so its partial read has
// nowhere to go until now. Without this the log window was empty for exactly
// the case it was written for: a tune book opened from the command line.
func (a *App) Opened(err error) {
	a.lock()
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
	eo.ShowPartNames = nameTheStaves(s)
	// This is the editor, so a bar that is half written keeps the room for
	// the rest of it: that room is where the caret goes for the next note.
	// What is printed is engraved from options of its own and keeps the
	// tight spacing.
	eo.RoomToWrite = true

	reg := symbol.NewRegistry()
	ro.Symbols = reg

	// A bar with nothing in it is a bar of silence, here as much as when a
	// file is opened from the menu later.
	for _, sc := range scores {
		restEmptyMeasures(sc)
	}
	app := &App{
		score:    s,
		book:     scores,
		filename: filename,
		symbols:  reg,
		// The score, not one of its parts. A score opens as itself however
		// many parts it carries: the parts are there to be printed and to
		// be turned to, and neither is what somebody opening a file wants
		// to be looking at first.
		sheet:      -1,
		engraveOpt: eo,
		renderOpt:  ro,
	}

	// And whatever the file itself said about how it was last looked at.
	//
	// Here as well as in setBook, because the first document does not go
	// through setBook: a score named on the command line is read before
	// the App exists and is handed to it already loaded. Opening the same
	// file from the menu applied its view and opening it from the command
	// line did not, which is the kind of difference nobody thinks to test
	// and everybody notices.
	app.applyView()

	// The person's own symbols, before anything a score carries: a score
	// that has one by the same name writes over the folder's copy, which
	// is what keeps somebody else's piece looking the way they drew it.
	app.folderSymbols = app.loadFolderSymbols()

	// This document's own router, built once. Every route below closes over
	// this App, which is what lets a server hold several documents without
	// a single handler having to know. See session.go.
	app.mux = http.NewServeMux()
	app.routes(app.mux)
	return app
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

	// One server, many documents. The App gscore was started on becomes the
	// session that requests naming none are talking to, so everything
	// written before sessions existed goes on working unchanged. See
	// session.go.
	a.server = NewServer(a, a.newScoreApp())

	srv := &http.Server{Handler: a.server}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
		}
	}()

	host, _, _ := net.SplitHostPort(ln.Addr().String())
	a.lock()
	a.native = native
	a.local = host == "127.0.0.1" || host == "::1"
	opened := a.filename
	a.mu.Unlock()
	// The file named on the command line counts as opened, and until now
	// there was no "this machine" to count it on: a.local is what makes the
	// recent list mean anything, and it is settled here.
	a.remember(opened)
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
		if err := desktop.Open(url, title, 1280, 820); err != nil {
			return err
		}
		// The window has gone but the program has not, which is the only
		// moment left to ask about work that was never written down. A page
		// cannot refuse to be left here the way it can in a browser: the
		// close button belongs to the system and the view offers no hook
		// into it. See OfferToSaveOnExit.
		return a.OfferToSaveOnExit()
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
	// The editor's own files, told to ask before reusing what is cached.
	//
	// The editor and the core are one program in two halves, and they
	// have to be from the same build: a page that serves itself the
	// app.js it kept and fetches a core it has not got asks for things
	// the other half does not do, and one thing quietly does not work.
	// Nothing about that reads as a failure, which is what makes it
	// expensive — the site build stamps its scripts with the version for
	// exactly this reason, and this server, which hands out a UI built
	// into the binary beside it, had nothing.
	//
	// no-cache and not no-store: the file is still kept and still
	// answered from the cache, the browser simply asks first, which over
	// a socket to the same machine costs nothing worth counting.
	mux.Handle("/", noCache(http.FileServer(http.FS(ui))))
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

// noCache makes a handler's answers revalidate rather than be reused
// unasked.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

// APIRoutes registers everything but the interface's own files.
//
// The browser build serves the HTML, CSS and JavaScript as static files and
// runs this same set of handlers inside WebAssembly, so a score behaves
// identically whether gscore is an application or a page.
func (a *App) APIRoutes(mux *http.ServeMux) {
	// Reading: drawings, descriptions, and the pickers. Nothing here changes
	// the music, so nothing here needs to be undoable.
	// What all of this is, for whoever is driving it: the description of
	// every route below, served by the server it describes.
	mux.HandleFunc("/openapi.yaml", a.handleOpenAPI)
	mux.HandleFunc("/score.svg", a.handleSVG)
	mux.HandleFunc("/font.otf", a.handleFont)
	mux.HandleFunc("/symbol.png", a.handleSymbolPNG)
	mux.HandleFunc("/keysig.svg", a.handleKeySigSVG)
	mux.HandleFunc("/timesig.svg", a.handleTimeSigSVG)
	mux.HandleFunc("/print", a.handlePrint)
	mux.HandleFunc("/api/pdf", a.handlePDF)
	mux.HandleFunc("/api/page", a.handlePage)
	mux.HandleFunc("/api/midi", a.handleMIDI)
	mux.HandleFunc("/api/convert", a.handleConvert)
	mux.HandleFunc("/api/record", a.handleRecord)
	mux.HandleFunc("/api/soundfonts", a.handleSoundfonts)
	mux.HandleFunc("/api/soundfont/fetch", a.handleSoundfontFetch)
	mux.HandleFunc("/api/soundfont/pick", a.handleSoundfontPick)
	mux.HandleFunc("/api/soundfont/load", a.handleSoundfontLoad)
	mux.HandleFunc("/api/soundfont/presets", a.handleSoundfontPresets)
	mux.HandleFunc("/api/soundfont/collection", a.handleZanderjaz)
	mux.HandleFunc("/api/settings", a.handleSettings)
	mux.HandleFunc("/api/recent", a.handleRecent)
	mux.HandleFunc("/api/play/start", a.handleLiveStart)
	mux.HandleFunc("/api/play/chunk", a.handleLiveChunk)
	mux.HandleFunc("/api/play/stop", a.handleLiveStop)
	mux.HandleFunc("/api/info", a.handleInfo)
	mux.HandleFunc("/api/layout", a.handleLayout)
	mux.HandleFunc("/api/hits", a.handleHits)
	mux.HandleFunc("/api/playback", a.handlePlayback)
	mux.HandleFunc("/api/harmonics", a.handleHarmonics)
	mux.HandleFunc("/api/reach", a.handleReach)
	mux.HandleFunc("/api/keys", a.handleKeys)
	mux.HandleFunc("/api/map", a.handleMap)
	mux.HandleFunc("/api/voices", a.handleVoices)
	mux.HandleFunc("/api/spacing", a.handleSpacing)
	mux.HandleFunc("/api/rhythm", a.handleRhythm)
	mux.HandleFunc("/api/melody", a.handleMelody)
	mux.HandleFunc("/api/contents", a.handleContents)
	mux.HandleFunc("/api/script/begin", a.handleScriptBegin)
	mux.HandleFunc("/api/script/end", a.handleScriptEnd)
	mux.HandleFunc("/api/keysignatures", a.handleKeySignatures)
	mux.HandleFunc("/api/textstyles", a.handleTextStyles)
	mux.HandleFunc("/api/chordpreview", a.handleChordPreview)
	mux.HandleFunc("/api/chordfingering", a.handleChordFingering)
	mux.HandleFunc("/api/chordqualities", a.handleChordQualities)
	mux.HandleFunc("/api/plugins", a.handlePlugins)
	mux.HandleFunc("/api/sheets", a.handleSheets)
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
	mux.HandleFunc("/api/pdfsurvey", a.handlePDFSurvey)
	mux.HandleFunc("/api/pdfreread", a.handlePDFReread)
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
		"/api/abc":             a.handleABC,
		"/api/nudge":           a.handleNudge,
		"/api/transpose":       a.handleTranspose,
		"/api/pickup":          a.handlePickup,
		"/api/engraving":       a.handleEngraving,
		"/api/numbering":       a.handleNumbering,
		"/api/systems":         a.handleSystems,
		"/api/find":            a.handleFind,
		"/api/shift":           a.handleShift,
		"/api/addmeasure":      a.handleAddMeasure,
		"/api/insertmeasure":   a.handleInsertMeasure,
		"/api/removemeasure":   a.handleRemoveMeasure,
		"/api/flute":           a.handleFlute,
		"/api/splitmeasure":    a.handleSplitMeasure,
		"/api/joinmeasure":     a.handleJoinMeasure,
		"/api/clearmeasure":    a.handleClearMeasure,
		"/api/measurerepeat":   a.handleMeasureRepeat,
		"/api/barline":         a.handleBarline,
		"/api/beam":            a.handleBeam,
		"/api/slur":            a.handleSlur,
		"/api/tie":             a.handleTie,
		"/api/stem":            a.handleStem,
		"/api/dot":             a.handleDot,
		"/api/duration":        a.handleDuration,
		"/api/tuplet":          a.handleTuplet,
		"/api/tremolo":         a.handleTremolo,
		"/api/accidental":      a.handleAccidental,
		"/api/articulation":    a.handleArticulation,
		"/api/technique":       a.handleTechnique,
		"/api/notehead":        a.handleNotehead,
		"/api/retune":          a.handleRetune,
		"/api/pagesetup":       a.handlePageSetup,
		"/api/crossstaff":      a.handleCrossStaff,
		"/api/fretchord":       a.handleFretChord,
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
		"/api/scripts":         a.handleScripts,
		"/api/tempo":           a.handleTempo,
		"/api/header":          a.handleHeader,
		"/api/staff":           a.handleStaff,
		"/api/addstaff":        a.handleAddStaff,
		"/api/setup":           a.handleSetup,
		"/api/tunings":         a.handleTunings,
		"/api/fret":            a.handleFret,
		"/api/restring":        a.handleRestring,
		"/api/removestaff":     a.handleRemoveStaff,
		"/api/movestaff":       a.handleMoveStaff,
		"/api/group":           a.handleGroup,
		"/api/insertsymbol":    a.handleInsertSymbol,
		"/api/removesymbol":    a.handleRemoveSymbol,
		"/api/movesymbol":      a.handleMoveSymbol,
	} {
		mux.HandleFunc(path, a.undoable(h))
	}
}

// lock takes the write lock, and counts the write. See App.gen.
func (a *App) lock() {
	a.mu.Lock()
	a.gen.Add(1)
}

// layout engraves a score, or hands back the last layout when nothing has
// been able to change since it was made.
//
// It keeps one, not a collection: the editor asks for the same view twice in
// a row and then asks again for a different one, so one is what pays. It
// takes no lock of its own on the score, so the caller's discipline about
// that is whatever it already was.
func (a *App) layout(score *model.Score, opt engrave.Options) *engrave.Layout {
	// Read before engraving, not after. A write that lands while this is
	// working leaves the layout tagged with the older count, so the next
	// reader finds the tag stale and engraves again — which is what should
	// happen, since what was engraved may be half of each.
	gen := a.gen.Load()
	a.layMu.Lock()
	defer a.layMu.Unlock()
	if a.lay != nil && a.layGen == gen && a.layScore == score && a.layOpt == opt {
		return a.lay
	}
	lay := engrave.Engrave(score, opt)
	a.layGen, a.layScore, a.layOpt, a.lay = gen, score, opt, lay
	return lay
}

// viewOptions reads the view a request is asking about: which score of a
// book, how big a staff space is, whether it is a strip or pages, and how
// wide a system may be.
//
// One reading of it, because two routes answer questions about the same
// picture — what it looks like and how big it is — and an editor told a size
// that belongs to a different picture scrolls to the wrong place.
func (a *App) viewOptions(r *http.Request) (*model.Score, engrave.Options, svg.Options) {
	a.mu.RLock()
	score := a.score
	if v := r.URL.Query().Get("tune"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i >= 0 && i < len(a.book) {
			score = a.book[i]
		}
	}
	eopt := a.engraving()
	ropt := a.renderOpt
	ropt.Symbols = a.symbols
	// The views: the score as it sounds, and the score as one of its parts.
	// Both are copies of it rather than flags the engraver reads, and both
	// are made here rather than at each route, because two routes answer
	// questions about the same picture — what it looks like and how big it
	// is — and a size taken from a different picture sends the editor to
	// the wrong place in the music.
	score = a.sounding(score)
	var sheet *model.Sheet
	if a.sheet >= 0 && a.sheet < len(score.Sheets) {
		sheet = &score.Sheets[a.sheet]
	}
	score = a.asSheet(score)
	a.mu.RUnlock()

	if sheet != nil {
		// What a part asks for that is not a measurement: its rests
		// counted, and no staff names down the left — a part has one staff
		// and its name is on the title page, where a name beside every
		// system would only be in the way of the music.
		eopt.MultiRests = sheet.MultiRests
		eopt.ShowPartNames = false
		if sheet.StaffSpace > 0 {
			ropt.StaffSpace = sheet.StaffSpace
		}
	}

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
	// Wrapped to fit a given width on the screen, in pixels.
	//
	// The caller knows how much room it has and not how much of that the
	// margins take, which is an engraving setting; the arithmetic belongs
	// here, where both are known. A score embedded in a page asks for
	// this: a frame is a few hundred pixels wide and an unbroken strip is
	// as wide as the tune is long.
	if v := r.URL.Query().Get("fit"); v != "" {
		if px, err := strconv.ParseFloat(v, 64); err == nil && px > 0 {
			w := px/ropt.StaffSpace - 2*eopt.Spacings.Margin
			// A floor, not a refusal. A frame narrower than a bar of music
			// is still the room there is, and ignoring what it asked for
			// left the score at its full width — the one layout that
			// certainly does not fit. Twenty spaces is about two bars, and
			// below that the music overflows and the frame scrolls, which
			// is at least what a reader can see the edge of.
			if w < 20 {
				w = 20
			}
			eopt.LineWidth = w
		}
	}

	// The music without its title block.
	//
	// For a score embedded in a page: the page already says what the
	// piece is, in its own type, and the title block engraved inside the
	// frame takes the top of it — so what a reader sees first is a name
	// they have just read and no music. The title is still the score's;
	// it is handed to the page instead, which can set it as it likes.
	if r.URL.Query().Get("title") == "0" {
		eopt.ShowTitle = false
	}
	if v := r.URL.Query().Get("width"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 40 && f <= 600 {
			eopt.LineWidth = f
		}
	}
	return score, eopt, ropt
}

func (a *App) handleSVG(w http.ResponseWriter, r *http.Request) {
	score, eopt, ropt := a.viewOptions(r)

	// What part of it to draw.
	//
	// The editor asks for the stretch it is showing plus a screenful either
	// way, rather than for the whole score, because a browser handed nine
	// hundred thousand shapes stutters at every scroll. Everything else —
	// printing, a PDF, a page somebody is given — asks for all of it and
	// gets all of it.
	ropt.Window = windowOf(r)
	// And whether to write the shapes alone, to be put inside a drawing the
	// editor already has. See svg.Options.Fragment.
	ropt.Fragment = r.URL.Query().Get("frag") == "1"

	// Already the view that was asked for: viewOptions made it, so that
	// this and the size route cannot disagree about which picture it is.
	lay := a.layout(score, eopt)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	if err := svg.Render(w, lay, ropt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// markerCount is how many bars carry a rehearsal mark.
//
// One per bar, not one per staff: a mark is written on every staff of the
// bar it belongs to, because "from letter B" has to mean the same bar to
// everybody.
func markerCount(s *model.Score) int {
	n := 0
	for i := 0; i < s.MeasureCount(); i++ {
		for _, p := range s.Parts {
			if i < len(p.Measures) && p.Measures[i].HasMarker() {
				n++
				break
			}
		}
	}
	return n
}

// windowOf reads the part of the score a request is asking to be drawn, in
// staff spaces, or nil where it asks for the whole of it.
//
// All four corners or none: three of them is a request this cannot make
// sense of, and guessing at the fourth would quietly draw the wrong stretch
// of music.
func windowOf(r *http.Request) *svg.Box {
	q := r.URL.Query()
	n := func(k string) (float64, bool) {
		v := q.Get(k)
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	}
	x0, ok0 := n("x0")
	y0, ok1 := n("y0")
	x1, ok2 := n("x1")
	y1, ok3 := n("y1")
	if !ok0 || !ok1 || !ok2 || !ok3 || x1 <= x0 || y1 <= y0 {
		return nil
	}
	return &svg.Box{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

// handleOpenAPI serves the description of this API.
//
// The same file as docs/openapi.yaml — embedded, not copied, so the document
// a reader opens in the repository and the one a running gscore hands out
// cannot say different things. api_test.go holds it to the route table, so
// what is served here describes what is actually here.
func (a *App) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	// The media type OpenAPI asks for, registered as application/yaml.
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	// Read from a machine somewhere else as often as from this one, and a
	// description nobody can fetch is no description.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(docs.OpenAPI)
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
	CanUndo bool `json:"canUndo"`
	CanRedo bool `json:"canRedo"`
	// Movements is where each movement of the score begins and ends, and
	// what it is called.
	//
	// The editor needs it because bars are numbered from one in every
	// movement: "go to bar four" means the fourth bar of the movement you
	// are standing in, and without this the editor could only count from
	// the first bar of the whole piece — so typing the number printed over
	// a bar of the Presto took you to the Adagio.
	//
	// One entry for a score with no movements in it, which is every
	// ordinary score.
	Movements []movementInfo `json:"movements,omitempty"`

	// Markers is how many rehearsal marks the score carries, so the editor
	// can offer the next letter when one is written.
	//
	// Counted here rather than by looking at the drawing, which is how it
	// was done: a long score is drawn a screenful at a time, so the letters
	// already written are mostly not on the page, and counting what was
	// drawn offered A over and over.
	Markers int `json:"markers"`
	// Marks is what is printed over each marked bar, by its index, with
	// the sequence worked out. MarkerStyle is what the ones in the
	// sequence are called, and Sequenced lists the bars that are in it —
	// which the editor needs to tell them from the marks written out as
	// themselves, since those do not advance the count.
	Marks       map[int]string `json:"marks,omitempty"`
	MarkerStyle string         `json:"markerStyle,omitempty"`
	Sequenced   []int          `json:"sequenced,omitempty"`
	Current     int            `json:"current"`
	Native      bool           `json:"native"`
	Local       bool           `json:"local"`
	// InBrowser says the core is running inside the page as WebAssembly,
	// with no server and no disk. It is not "local" — there is no machine
	// to save to — but it can still make a sound, because the synthesizer
	// is compiled into the page along with everything else.
	InBrowser bool   `json:"inBrowser,omitempty"`
	Version   string `json:"version"`
	Filename  string `json:"filename"`
	// Unsaved says the score has been edited since it was last written out.
	// The interface asks before letting the window close on it.
	Unsaved  bool   `json:"unsaved"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Composer string `json:"composer"`
	// Arranger is its own field and not a second line of the composer:
	// two people, credited in two places and on two lines of the page.
	Arranger string `json:"arranger"`
	// How those four are set, where somebody has set them in anything.
	// The words are the fields above either way, so a window that knows
	// nothing about formatting still shows the piece's name.
	TitleRuns    []model.Span `json:"titleRuns,omitempty"`
	SubtitleRuns []model.Span `json:"subtitleRuns,omitempty"`
	ComposerRuns []model.Span `json:"composerRuns,omitempty"`
	ArrangerRuns []model.Span `json:"arrangerRuns,omitempty"`
	// Sent so the window that edits it can open on what is there. Left
	// out, the box came up empty and saving wrote that emptiness back.
	Copyright string `json:"copyright"`
	// Description is a few words about the piece, for a shelf of them,
	// and Tags are what it is in the words it would be looked for by.
	// The window shows both and a shelf built from the files sorts by
	// them.
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	Tempo       int        `json:"tempo"`
	Parts       []partInfo `json:"parts"`
	// Groups are the runs of staves drawn as belonging together, which is
	// how the editor can tell a grand staff from two staves that happen to
	// be next to each other. A piano is one player on two staves joined by
	// a brace, and anything that lists who is playing has to say "piano"
	// once rather than twice; two cellos side by side are two players and
	// are listed twice. Nothing in the staves themselves distinguishes the
	// two cases — the brace does.
	Groups  []groupInfo `json:"groups,omitempty"`
	Formats []string    `json:"formats"`

	// The beat the tempo counts and the words beside it. A number on its own
	// does not say anything: 120 in 6/8 is a different speed depending on
	// whether it is counting quarters or dotted quarters.
	TempoUnit   string `json:"tempoUnit"`
	TempoDotted bool   `json:"tempoDotted,omitempty"`
	TempoText   string `json:"tempoText,omitempty"`
	TempoParens bool   `json:"tempoParens,omitempty"`

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
	Parens bool   `json:"parens,omitempty"`
}

// movementInfo is one movement of a score, as the editor sees it.
type movementInfo struct {
	// First and Last are bars of the whole score, counted from zero, so
	// that everything else the editor holds — the clickable regions, the
	// performance, the cursor — goes on meaning what it meant.
	First int    `json:"first"`
	Last  int    `json:"last"`
	Title string `json:"title,omitempty"`
}

// groupInfo is one run of staves drawn as belonging together.
type groupInfo struct {
	// First and Last are staff indices, inclusive, as the editor counts
	// them everywhere else.
	First int `json:"first"`
	Last  int `json:"last"`
	// Symbol is "brace", "bracket" or "line". A brace is the one that says
	// one instrument: it is what makes a pair of staves a grand staff.
	Symbol   string `json:"symbol"`
	Barlines bool   `json:"barlines,omitempty"`
	Name     string `json:"name,omitempty"`
}

type partInfo struct {
	Name       string `json:"name"`
	Instrument string `json:"instrument"`
	// Sound is which instrument this staff is, by its MusicXML standard
	// sound id, where the file or the catalogue said. The instrument above
	// is only the General MIDI program that plays it, and cannot tell a
	// bass clarinet from a clarinet. Empty where nothing said.
	Sound    string `json:"sound,omitempty"`
	Clef     string `json:"clef"`
	Key      string `json:"key"`
	Time     string `json:"time"`
	Measures int    `json:"measures"`
	Notes    int    `json:"notes"`
	// TransposeSemitones is how far below what is written this staff sounds,
	// so the editor can get back from a sounding note to the printed one.
	// A fingering chart needs the printed one: fingers follow the page.
	TransposeSemitones int `json:"transposeSemitones"`
	// Hidden says the staff is out of the drawing but still in the score.
	Hidden bool `json:"hidden,omitempty"`
	// Tab is the tuning this staff is written in, where it is written as
	// tablature rather than on a staff of pitches. Absent otherwise, which
	// is what the editor reads to know whether a typed digit is a fret.
	Tab *tabInfo `json:"tab,omitempty"`
	// MirrorOf is the staff this one draws the music of, where it holds
	// none of its own: the tablature half of a notation-and-tablature
	// pair. Absent where the staff has its own music.
	//
	// The editor needs it to get from one half of a pair to the other. A
	// guitar written both ways is one instrument on two staves, and the
	// cursor is usually on the half with the notes — so anything asked of
	// the instrument, such as retuning it, has to find the half that
	// carries the tuning.
	MirrorOf *int `json:"mirrorOf,omitempty"`
}

func (a *App) handleInfo(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := info{
		Warning:      a.warning,
		Log:          a.importLog,
		Current:      a.current,
		Native:       a.native,
		Local:        a.local,
		InBrowser:    a.inBrowser,
		Version:      version.Version,
		Filename:     a.filename,
		Unsaved:      a.unsaved,
		CanUndo:      len(a.undoStack) > 0,
		CanRedo:      len(a.redoStack) > 0,
		Title:        a.score.Identity.Title,
		Subtitle:     a.score.Identity.Subtitle,
		Composer:     a.score.Identity.Composer,
		Arranger:     a.score.Identity.Arranger,
		TitleRuns:    a.score.Identity.TitleRuns,
		SubtitleRuns: a.score.Identity.SubtitleRuns,
		ComposerRuns: a.score.Identity.ComposerRuns,
		ArrangerRuns: a.score.Identity.ArrangerRuns,
		Copyright:    a.score.Identity.Copyright,
		Tags:         append([]string{}, a.score.Tags...),
		Description:  a.score.Description,
		Tempo:        a.score.Tempo.BPM,
	}
	for _, sec := range a.score.Sections() {
		out.Movements = append(out.Movements, movementInfo{
			First: sec.First, Last: sec.Last, Title: sec.Title,
		})
	}
	for _, g := range a.score.Groups {
		out.Groups = append(out.Groups, groupInfo{
			First: g.First, Last: g.Last, Symbol: g.Symbol.String(),
			Barlines: g.Barlines, Name: g.Name,
		})
	}
	out.Markers = markerCount(a.score)
	out.Marks = a.score.RehearsalMarks(engrave.BarNumbers(a.score))
	out.MarkerStyle = a.score.Markers.String()
	for i := 0; i < a.score.MeasureCount(); i++ {
		for _, p := range a.score.Parts {
			if i < len(p.Measures) && p.Measures[i].MarkerAuto {
				out.Sequenced = append(out.Sequenced, i)
				break
			}
		}
	}
	out.Cursor = a.cursorInfo()
	out.TempoUnit, out.TempoDotted = beatName(a.score.Tempo.Unit)
	out.TempoText = a.score.Tempo.Text
	out.TempoParens = a.score.Tempo.Parens
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
			Parens: t.Parens,
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
			Name: name, Instrument: instrument, Sound: p.Sound,
			Clef: p.Clef.String(), Key: keyText(p.Key),
			Time: p.Time.String(), Measures: len(p.Measures), Notes: notes,
			TransposeSemitones: p.Transpose.Semitones(),
			Hidden:             p.Hidden,
			Tab:                tabOf(p),
			MirrorOf:           p.MirrorOf,
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
// nameTheStaves says whether a score's staves should be labelled on the page.
//
// More than one staff and the names go on. On a score of fifty-three,
// "which line is the first violins" is not a question anybody should answer
// by counting down from the top — and the editor is where that question is
// asked most, since it is where the music is being worked on.
//
// A single-staff piece is the one case where the name says nothing: there is
// no other staff to tell it from, and the word only pushes the music right.
func nameTheStaves(s *model.Score) bool {
	return s != nil && len(s.Parts) > 1
}

func (a *App) setBook(scores []*model.Score, filename string, err error) {
	a.book = scores
	a.current = 0
	a.score = scores[0]
	// A newly opened score decides this for itself, the way it decides its
	// own clefs and its own key: a quartet opened after a solo gets its
	// staves named.
	a.engraveOpt.ShowPartNames = nameTheStaves(a.score)
	// And then whatever the file itself said, which wins: a score saved
	// with its beats marked opens with them marked. Done after the line
	// above rather than before, so that a file which has an opinion about
	// the staff names is obeyed and one which has none still gets the
	// sensible answer for its size.
	a.applyView()
	a.filename = filename
	a.warning, a.importLog = readProblems(err)
	a.forgetHistory()
	// A document just opened has nothing in it that is not on disk, and a
	// new one has nothing in it at all.
	a.unsaved = false
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
	a.lock()
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
	a.lock()
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
	var scores []*model.Score
	if isPDFName(name) {
		// A PDF keeps its bytes and its grouping, so the reading can be
		// looked at and done again. See pdfimport.go.
		per, _ := stavesParam(r)
		scores, err = a.readPDF(name, body, per)
		if len(scores) == 0 {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = nil
	} else {
		imp, ierr := format.ImporterFor(name)
		if ierr != nil {
			http.Error(w, ierr.Error(), http.StatusUnsupportedMediaType)
			return
		}
		scores, err = format.ImportAll(imp, bytesReader(body))
		if len(scores) == 0 {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	a.lock()
	if !isPDFName(name) {
		a.forgetPDF()
	}
	a.setBook(scores, name, err)
	a.mu.Unlock()
	writeJSON(w, map[string]any{"status": "ok", "filename": name, "tunes": len(scores)})
}

// handleLayout reports the engraved size, which the canvas uses to size
// itself and to work out which part of a long score it is looking at.
//
// For the view the request asks about, not for whatever the application was
// last set to: a strip and a page of the same music are nothing like the
// same shape, and a size from the wrong one sends the editor to the wrong
// place in the music.
func (a *App) handleLayout(w http.ResponseWriter, r *http.Request) {
	score, eopt, _ := a.viewOptions(r)
	lay := a.layout(score, eopt)
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
	page := a.score.Page
	// Carried from the editor for the same reason the PDF carries it: the
	// marks are off unless asked for, so printing them can only be what
	// was asked for. See renderPDF.
	beatMarks := a.engraveOpt.ShowBeatMarks
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
		// The parts the score carries, where somebody has made them: those
		// are the documents that were named, sized and laid out for the
		// stands, and printing a staff apiece instead would be printing
		// something nobody asked for.
		if len(score.Sheets) > 0 {
			for i, sh := range score.Sheets {
				sections = append(sections, section{
					heading: sh.Name, score: sheetOf(score, i),
				})
			}
			break
		}
		// A score with none: a staff apiece, which is the best guess there
		// is and what this did before parts existed.
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
	// And prints the noteheads in whatever colours they are being read in.
	eopt.ColourNotes = ColourNotes()
	eopt.NoteColours = NoteColours()
	eopt.ShowBeatMarks = beatMarks
	// As wide as the paper the score says it is printed on allows, rather
	// than a fixed number that was right for one size of paper. A score
	// set up for A3 landscape and printed at a hundred and fifty spaces
	// came out a third of the way across the sheet.
	eopt.LineWidth = paperFor(page).LineWidth(eopt.Spacings)
	// Asked for in the print dialog rather than taken from the editor: the
	// page a player reads from and the page being written are not the same
	// page, and the whole point of counting bars of rest is the first one.
	eopt.MultiRests = r.URL.Query().Get("multirest") == "1"
	// And whether the parts are printed as they sound or as they are read.
	// A conductor's score is often wanted both ways.
	if r.URL.Query().Get("concert") == "1" {
		for i, sec := range sections {
			sections[i].score = concertOf(sec.score)
		}
	}

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
  /* The paper the score was set up for, so the browser's own print
     dialog starts on the right size and the margins are the ones that
     were engraved to. */
  @page { size: %s; margin: %s; }
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
</script></body></html>`, html.EscapeString(title),
		pageCSSSize(page), pageCSSMargin(page), body.String())
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
// anywhere else does not, and then the answer is the first bar.
//
// It used to be the last bar, on the reasoning that a piece somebody opens
// to add to is a piece they want to carry on writing. That is true of a
// twelve-bar tune and wrong about everything else: open a symphony and the
// page is at bar two thousand six hundred, which is nowhere anybody asked
// to be and a long way back from where they wanted to start reading. A
// score opens at its beginning, like any other document. Carrying on from
// the end is a thing somebody asks for, and the staff sprints in the editor
// are how they ask — one per staff, because on a score of fifty-three the
// end of which staff is a real question.
//
// Either way there is a cursor on the page from the moment it opens, which
// is the difference between an editor and a viewer.
func (a *App) cursorInfo() cursorInfo {
	if c := a.score.Cursor; c != nil && c.Part < len(a.score.Parts) {
		return cursorInfo{Part: c.Part, Measure: c.Measure, At: c.At, Step: c.Step}
	}
	if len(a.score.Parts) == 0 || len(a.score.Parts[0].Measures) == 0 {
		return cursorInfo{}
	}
	return cursorInfo{Measure: 0, At: 0, Step: middleStaffStep}
}

// middleStaffStep is the middle line, which is where a cursor with no reason
// to be anywhere else sits.
const middleStaffStep = 4

// pageCSSSize and pageCSSMargin are the score's paper as a browser's print
// machinery wants to hear it.
//
// Printing goes through the browser: the pages are drawn as SVG and the
// browser's own print dialog puts them on paper. So the paper has to be
// said again in CSS, or the dialog starts on whatever the last thing
// printed used and the margins the music was engraved to are not the
// margins it is printed with.
//
// Millimetres, which is what the model keeps and what @page takes.
func pageCSSSize(p model.Page) string {
	if p.IsZero() {
		return "auto"
	}
	w, h := p.Size()
	return fmt.Sprintf("%.1fmm %.1fmm", w, h)
}

func pageCSSMargin(p model.Page) string {
	if p.IsZero() {
		return "12mm"
	}
	// CSS orders them clockwise from the top, which is not the order they
	// are asked for.
	left, right, top, bottom := p.Margins()
	return fmt.Sprintf("%.1fmm %.1fmm %.1fmm %.1fmm", top, right, bottom, left)
}
