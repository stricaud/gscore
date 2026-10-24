/*
 * main.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
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
	"github.com/stricaud/gscore/internal/version"

	// Importers register themselves, which is what replaces the dlopen
	// plugin loader of the C program.
	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
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
		var partial format.Partial
		if errors.As(err, &partial) && score != nil {
			fmt.Fprintf(os.Stderr, "gscore: warning: %v\n", err)
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
			*fontURL, *scriptURL, !*noPlay); err != nil {
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
	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.ShowPartNames = len(s.Parts) > 1
	eopt.ShowMeasureNumbers = true
	popt, eopt = gpdf.Fit(s, popt, eopt)
	return gpdf.Render(w, engrave.Engrave(s, eopt), popt)
}

// renderFile engraves a score to a drawing, in whichever format the name asks
// for.
//
// PDF is paginated whatever -strip says: a continuous strip has no page breaks
// to print at, and a PDF is pages or it is nothing.
func renderFile(s *model.Score, path string, strip bool, space float64,
	landscape bool, fontURL, scriptURL string, play bool) error {
	ext := strings.ToLower(filepath.Ext(path))
	wantPDF := ext == ".pdf"
	wantPage := ext == ".html" || ext == ".htm"

	// The player on its own, for serving beside pages that fetch it rather
	// than carry it. An option to link to a file nobody can obtain is no
	// option at all.
	if ext == ".js" {
		return os.WriteFile(path, []byte(ghtml.PlayerJS()), 0o644)
	}

	eopt := engrave.DefaultOptions()
	if strip && !wantPDF {
		eopt.Mode = engrave.Strip
	}

	popt := gpdf.DefaultOptions()
	if wantPDF {
		popt.Title = s.Identity.Title
		popt.Author = s.Identity.Composer
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
		if play {
			hopt.Sound = app.PageSound(s, lay, ropt.StaffSpace)
		}
		return ghtml.Render(f, lay, s, nil, hopt)
	}
	return svg.Render(f, lay, ropt)
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
	fmt.Printf("tempo     %d x %v per minute\n", s.Tempo.BPM, s.Tempo.Unit)
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
