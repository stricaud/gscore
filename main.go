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
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/app"
	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
	gpdf "github.com/stricaud/gscore/internal/render/pdf"
	"github.com/stricaud/gscore/internal/render/svg"
	"github.com/stricaud/gscore/internal/version"

	// Importers register themselves, which is what replaces the dlopen
	// plugin loader of the C program.
	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
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
		if err := convertTo(score, *convert); err != nil {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", *convert)
	case *out != "":
		if err := renderFile(score, *out, *strip, *space); err != nil {
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
func convertTo(s *model.Score, path string) error {
	exp, err := format.ExporterFor(path)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return exp.Export(f, s)
}

// renderFile engraves a score to a drawing, in whichever format the name asks
// for.
//
// PDF is paginated whatever -strip says: a continuous strip has no page breaks
// to print at, and a PDF is pages or it is nothing.
func renderFile(s *model.Score, path string, strip bool, space float64) error {
	wantPDF := strings.EqualFold(filepath.Ext(path), ".pdf")

	eopt := engrave.DefaultOptions()
	if strip && !wantPDF {
		eopt.Mode = engrave.Strip
	}
	if wantPDF {
		eopt.Mode = engrave.Paginated
		eopt.ShowPartNames = len(s.Parts) > 1
		eopt.ShowMeasureNumbers = true
		// A line of music that fits A4 with the margins a part is printed
		// with, measured in staff spaces.
		eopt.LineWidth = 76
	}
	lay := engrave.Engrave(s, eopt)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if wantPDF {
		popt := gpdf.DefaultOptions()
		popt.Title = s.Identity.Title
		popt.Author = s.Identity.Composer
		if space > 0 && space != 8 {
			popt.StaffSpace = space
		}
		return gpdf.Render(f, lay, popt)
	}

	ropt := svg.DefaultOptions()
	ropt.StaffSpace = space
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
