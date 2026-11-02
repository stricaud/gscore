/*
 * main.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Command gscore is the score editor.
//
// Run with no arguments it opens a new, empty score, which is what a notation
// program should do. Given a file it opens that instead. The -o flag renders
// to SVG without opening anything, for scripting.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/app"
	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
	ghtml "github.com/stricaud/gscore/internal/render/html"
	gpdf "github.com/stricaud/gscore/internal/render/pdf"
	"github.com/stricaud/gscore/internal/render/svg"
	"github.com/stricaud/gscore/internal/smufl"
	"github.com/stricaud/gscore/internal/version"

	// Importers register themselves, which is what replaces the dlopen
	// plugin loader of the C program.
	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
	_ "github.com/stricaud/gscore/internal/format/lilypond"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
	_ "github.com/stricaud/gscore/internal/format/musx"
	_ "github.com/stricaud/gscore/internal/format/nwc"
)

func init() {
	// The native window's event loop has to own the main thread: both AppKit
	// and GTK require it.
	runtime.LockOSThread()
}

func main() {
	var (
		showVersion = flag.Bool("version", false, "print the version and exit")
		showFormats = flag.Bool("formats", false, "list the formats that can be read")
		out         = flag.String("o", "", "render to this file and exit; .pdf or .svg, by the extension")
		convert     = flag.String("convert", "", "convert the input to this file and exit; the format follows the extension")
		describe    = flag.Bool("describe", false, "print what the score contains and exit")
		port        = flag.Int("port", 0, "serve the editor on this port (0 picks a free one)")
		serve       = flag.String("serve", "", "run as a web server on this address, such as :8080 or 0.0.0.0:8080")
		browser     = flag.Bool("browser", false, "show the editor in a web browser instead of a native window")
		headless    = flag.Bool("headless", false, "serve the editor but open nothing")
		strip       = flag.Bool("strip", false, "engrave as one continuous strip, as gscore 0.0.9 did")
		space       = flag.Float64("space", 8, "staff space size in pixels, for -o")
		landscape   = flag.Bool("landscape", false, "turn the paper on its side, for -o to a PDF")
		fontURL     = flag.String("font", "",
			"for -o to .html: fetch the music font from this address instead of "+
				"writing it into the page")
		scriptURL = flag.String("script", "",
			"for -o to .html: fetch the player from this address instead of "+
				"writing it into the page")
		noPlay = flag.Bool("noplay", false,
			"for -o to .html: leave the player out, so the page is a drawing")
		audioURL = flag.String("audio", "",
			"for -o to .html: play a recording fetched from this address — made "+
				"with -o to a .flac — instead of synthesising the music in the page")
		bands = flag.Int("bands", 0,
			"for -o to .html: write the drawing as files of this many systems "+
				"each, beside the page, for the page to fetch as it needs them "+
				"— for a score too large to put in one")
		soundFont = flag.String("soundfont", "",
			"the soundfont to record through, for -o to a .flac")

		noPDF = flag.Bool("nopdf", false,
			"for -convert to .gscore: write the music alone, without a PDF of "+
				"it in front")
	)
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("gscore %s\n", version.Version)
		return
	}
	if *showFormats {
		for _, i := range format.Importers() {
			fmt.Printf("%-8s %s\n", i.Name(), strings.Join(i.Extensions(), " "))
		}
		return
	}
	if flag.NArg() > 1 {
		usage()
		os.Exit(2)
	}

	path := flag.Arg(0)
	scores, err := load(path)
	var score *model.Score
	if len(scores) > 0 {
		score = scores[0]
	}
	if err != nil {
		// A format may understand part of a file and say so; that is worth
		// showing rather than refusing to open anything.
		//
		// And showing means showing what: a warning that says some of the
		// file was not read, and then nothing about which part, leaves
		// whoever is reading it to open the score and hunt. The importer
		// has already worked out what it passed over and in many cases
		// what the words were, so the list is printed under the warning.
		var partial format.Partial
		if errors.As(err, &partial) && score != nil {
			fmt.Fprintf(os.Stderr, "gscore: warning: %v\n", err)
			for _, d := range partial.Details() {
				fmt.Fprintf(os.Stderr, "gscore:   %s\n", d)
			}
		} else {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
			os.Exit(1)
		}
	}

	switch {
	case *convert != "":
		if err := convertTo(score, *convert, !*noPDF); err != nil {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", *convert)
	case *out != "":
		if err := renderFile(score, *out, *strip, *space, *landscape,
			*fontURL, *scriptURL, *soundFont, *audioURL, *bands, !*noPlay); err != nil {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", *out)
	case *describe:
		describeScore(score)
	default:
		mode := app.Native
		if *browser {
			mode = app.Browser
		}
		if *headless {
			mode = app.Headless
		}
		addr := ""
		if *port != 0 {
			addr = "127.0.0.1:" + strconv.Itoa(*port)
		}
		if *serve != "" {
			// Serving is a web server, not a window: it keeps running and
			// opens nothing.
			addr = *serve
			mode = app.Headless
		}
		ap := app.NewBook(scores, path)
		// Whatever the importer could not read goes with the score, so the
		// log window has it for a file opened from the command line too.
		ap.Opened(err)
		if err := ap.Run(addr, mode); err != nil {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
			os.Exit(1)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gscore %s - a musical score editor

usage:
  gscore                 open the editor with a new score
  gscore FILE            open the editor with FILE
  gscore -o OUT.pdf FILE engrave FILE to PDF and exit
  gscore -o OUT.svg FILE engrave FILE to SVG and exit
  gscore -o OUT.html FILE  a web page anybody can open: the music, a player,
                           and the font — all in one file unless -font or
                           -script are given an address to fetch them from
  gscore -o OUT.js         write the player on its own, to serve beside pages
  gscore -o OUT.mid FILE   the performance as a MIDI file, for rendering with
                           a soundfont or opening in a sequencer
  gscore -o OUT.flac FILE  the performance recorded through -soundfont, which
                           a page made with -audio plays instead of making
                           the sound itself
  gscore -serve :8080    run as a web server
  gscore -describe FILE  print what FILE contains
  gscore -convert OUT.musicxml FILE   convert between formats

options:
`, version.Version)
	flag.PrintDefaults()
}

// load reads everything in a file, or returns a new empty score when no file
// is named. A notation program with nothing to show should still open.
//
// A file may hold more than one score — an ABC tune book holds as many as it
// likes — so all of them are read and the editor is given the lot.
func load(path string) ([]*model.Score, error) {
	if path == "" {
		return []*model.Score{model.NewScore()}, nil
	}
	imp, err := format.ImporterFor(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return format.ImportAll(imp, f)
}

// convertTo writes the score in whatever format the filename asks for.
func convertTo(s *model.Score, path string, withPDF bool) error {
	exp, err := format.ExporterFor(path)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// A .gscore carries a PDF of itself in front of the music, as a saved
	// score does: renamed .pdf, the same file opens in anything that reads
	// one. -nopdf writes the music alone, which is three decimal orders
	// smaller.
	if e := strings.ToLower(filepath.Ext(path)); e == ".mid" || e == ".midi" {
		return app.MIDIFile(f, s)
	}
	if withPDF && strings.EqualFold(filepath.Ext(path), ".gscore") {
		var pdf bytes.Buffer
		if err := renderPDFTo(&pdf, s); err != nil {
			return err
		}
		return gscorefmt.WriteWithPDF(f, s, nil, pdf.Bytes())
	}
	return exp.Export(f, s)
}

// renderPDFTo engraves the score onto paper, for carrying inside a .gscore.
func renderPDFTo(w io.Writer, s *model.Score) error {
	popt := gpdf.DefaultOptions()
	popt.Title = s.Identity.Title
	popt.Author = s.Identity.Composer
	popt.PageNumbers = true
	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.ShowPartNames = len(s.Parts) > 1
	eopt.ShowMeasureNumbers = true
	popt, eopt = gpdf.Fit(s, popt, eopt)
	lay := engrave.Engrave(s, eopt)
	sayIfDivided(s, lay, popt)
	return gpdf.Render(w, lay, popt)
}

// sayIfDivided reports a system that had to be cut across pages.
//
// Printed rather than returned: the file is still written and is still the
// best that can be done on that sheet. It is a thing to know, not a failure.
func sayIfDivided(s *model.Score, lay *engrave.Layout, popt gpdf.Options) {
	if msg := gpdf.Divided(s, lay, popt); msg != "" {
		fmt.Fprintf(os.Stderr, "gscore: %s\n", msg)
	}
}

// renderFile engraves a score to a drawing, in whichever format the name asks
// for.
//
// PDF is paginated whatever -strip says: a continuous strip has no page breaks
// to print at, and a PDF is pages or it is nothing.
func renderFile(s *model.Score, path string, strip bool, space float64,
	landscape bool, fontURL, scriptURL, soundFont, audioURL string,
	bands int, play bool) error {
	ext := strings.ToLower(filepath.Ext(path))
	wantPDF := ext == ".pdf"
	wantPage := ext == ".html" || ext == ".htm"

	// The player on its own, for serving beside pages that fetch it rather
	// than carry it. An option to link to a file nobody can obtain is no
	// option at all.
	if ext == ".js" {
		return os.WriteFile(path, []byte(ghtml.PlayerJS()), 0o644)
	}
	// And the music font, for the same reason. -font tells a page to
	// fetch the font from an address instead of carrying it, which saves
	// most of a megabyte on every page of a shelf of them — and was an
	// address nobody could put a font at, because there was no way to get
	// one out of gscore.
	if ext == ".otf" {
		return os.WriteFile(path, smufl.FontData(), 0o644)
	}
	// A MIDI file is not a drawing, but it is what -o means to anybody who
	// wants the music handed to something that makes a better noise.
	if ext == ".mid" || ext == ".midi" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return app.MIDIFile(f, s)
	}
	// And a recording is the music itself, played: the one output that
	// needs instruments gscore does not carry.
	if ext == ".flac" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return app.RecordFile(f, s, soundFont)
	}

	eopt := engrave.DefaultOptions()
	if strip && !wantPDF {
		eopt.Mode = engrave.Strip
	}

	popt := gpdf.DefaultOptions()
	if wantPDF {
		popt.Title = s.Identity.Title
		popt.Author = s.Identity.Composer
		popt.PageNumbers = true
		if space > 0 && space != 8 {
			popt.StaffSpace = space
		}
		if landscape {
			popt = popt.Landscape()
		}

		eopt.Mode = engrave.Paginated
		eopt.ShowPartNames = len(s.Parts) > 1
		eopt.ShowMeasureNumbers = true
		// How big the music is set and how long a line of music the paper
		// holds: one decision, made from the music and the paper together.
		// A size asked for is a size not to be exceeded, so a score with
		// more staves than the sheet will hold is set smaller rather than
		// printed with its lower staves falling off the page.
		popt, eopt = gpdf.Fit(s, popt, eopt)
	}
	lay := engrave.Engrave(s, eopt)
	if wantPDF {
		sayIfDivided(s, lay, popt)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if wantPDF {
		return gpdf.Render(f, lay, popt)
	}

	ropt := svg.DefaultOptions()
	ropt.StaffSpace = space

	if wantPage {
		// A page any browser opens: the music drawn, and a player to hear
		// it with. Whether the font and the player travel with it or are
		// fetched is the caller's choice, one each.
		hopt := ghtml.DefaultOptions()
		hopt.SVG = ropt
		hopt.FontURL = fontURL
		hopt.ScriptURL = scriptURL
		hopt.AudioURL = audioURL
		if play {
			hopt.Sound = app.PageSound(s, lay, ropt.StaffSpace)
		}
		if bands > 0 {
			// The drawing in pieces beside the page rather than inside it.
			// Written from this one layout: engraving the ninth symphony is
			// six seconds and there are a hundred of them, so doing it once
			// is the whole point.
			written, err := writeBands(path, lay, ropt, bands)
			if err != nil {
				return err
			}
			hopt.Bands = written
		}
		return ghtml.Render(f, lay, s, nil, hopt)
	}
	return svg.Render(f, lay, ropt)
}

// writeBands writes the drawing as pieces beside the page, and says where
// they went.
//
// One layout, many windows. The engraving is the expensive part and it has
// already been done by the time we are here, so each piece is a walk down the
// shapes that are already in hand, keeping the ones that reach into its band.
//
// The files are named after the page: play.html gets play.000.svg and so on
// beside it, and the page asks for them by those names, relative to itself.
// So the whole set can be moved or renamed as a directory and still work,
// which is what being a static site means.
func writeBands(page string, lay *engrave.Layout, ropt svg.Options,
	perBand int) ([]ghtml.Band, error) {

	dir := filepath.Dir(page)
	stem := strings.TrimSuffix(filepath.Base(page), filepath.Ext(page))
	name := func(i int) string { return fmt.Sprintf("%s.%03d.svg", stem, i) }

	list := ghtml.Bands(lay, ropt.StaffSpace, perBand, name)
	for _, band := range list {
		bopt := ropt
		// The shapes alone: the page already has the drawing to put them in.
		bopt.Fragment = true
		// A fragment carries no stylesheet and so no font either; the shell
		// in the page brought both.
		bopt.EmbedFont = false
		bopt.Interactive = true
		x0, y0, x1, y1 := band.Window(lay.Width, ropt.StaffSpace)
		bopt.Window = &svg.Box{X0: x0, Y0: y0, X1: x1, Y1: y1}

		out, err := os.Create(filepath.Join(dir, name(band.Index)))
		if err != nil {
			return nil, err
		}
		if err := svg.Render(out, lay, bopt); err != nil {
			out.Close()
			return nil, err
		}
		if err := out.Close(); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// shapeCount is how many separate things the engraver drew.
//
// Every one of them becomes an element in a page's drawing and a thing the
// browser has to lay out and repaint, so this is the number that says whether
// a page can hold the score or has to be given it a piece at a time.
func shapeCount(lay *engrave.Layout) int {
	if lay == nil {
		return 0
	}
	return len(lay.Glyphs) + len(lay.Lines) + len(lay.Texts) +
		len(lay.Curves) + len(lay.Circles) + len(lay.Quads)
}

func describeScore(s *model.Score) {
	if s.Identity.Title != "" {
		fmt.Printf("title     %s\n", s.Identity.Title)
	}
	if s.Identity.Subtitle != "" {
		fmt.Printf("subtitle  %s\n", s.Identity.Subtitle)
	}
	if s.Identity.Composer != "" {
		fmt.Printf("composer  %s\n", s.Identity.Composer)
	}
	if s.Description != "" {
		// On one line, because this is a summary and a shelf built from
		// it wants a blurb. A description written as paragraphs is shown
		// here as one; the file keeps it as it was written.
		fmt.Printf("description %s\n",
			strings.Join(strings.Fields(s.Description), " "))
	}
	if len(s.Tags) > 0 {
		// One line, comma separated, the way they were typed — so that a
		// shelf built from a directory of scores can read them straight
		// out of this without opening the files itself.
		fmt.Printf("tags      %s\n", strings.Join(s.Tags, ", "))
	}
	fmt.Printf("tempo     %d x %v per minute\n", s.Tempo.BPM, s.Tempo.Unit)

	// How much drawing the score comes to.
	//
	// It is the thing that decides whether a page can carry the music or has
	// to fetch it in pieces, and it is not something to guess at. Staves
	// times bars is the obvious guess and on the ninth symphony it is wrong
	// by a factor of seven: fifty-three staves, and most of them resting
	// most of the time. A bar's rest is one shape; a beamed chord with
	// accidentals and a slur over it is a dozen.
	//
	// This is the one line here that is not read straight out of the file.
	// Counting what the engraver drew means engraving it, which on a
	// symphony is a few seconds where everything else about it is instant.
	// It is worth it to whoever is deciding what to do with the file, and
	// nothing else in the line is any use for deciding that.
	fmt.Printf("shapes    %d\n", shapeCount(engrave.Engrave(s, engrave.DefaultOptions())))

	for _, p := range s.Parts {
		name := p.Name
		if name == "" && p.ID != "" {
			name = "voice " + p.ID
		}
		if name == "" {
			name = "(single part)"
		}
		notes := 0
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				notes += len(v.Elements)
			}
		}
		fmt.Printf("%s: %s, %d measures, %d elements\n", name, p.Clef, len(p.Measures), notes)
	}
}
