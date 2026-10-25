/*
 * html.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package html writes a score as a web page.
//
// A score is no use to anybody who cannot open it. Sending somebody a
// .gscore means sending them a program first, and a PDF cannot be played —
// so this is the third thing: one file that any browser opens, with the
// music drawn in it and, if it is asked for, a player.
//
// What it leans on is a choice. The font and the player can each be written
// into the page or fetched from an address: written in, the file stands on
// its own — a memory stick, an email attachment, a file:// path with the
// network off — and costs about 1.2 MB, nearly all of it the font. Fetched,
// it is a few tens of kilobytes and needs the addresses to answer, which is
// right for a page on a site that serves them beside it.
//
// It has to be called .html. A browser decides what to do with a file by its
// extension, not by what is in it, so a file named .gscore is downloaded
// whatever it holds. That is a rule of the web, not something a format can
// get around.
package html

import (
	"bytes"
	"fmt"
	"html"
	"io"

	"github.com/stricaud/gscore/internal/brand"
	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/render/svg"
	"github.com/stricaud/gscore/internal/smufl"
	"github.com/stricaud/gscore/internal/symbol"
)

// Options control one page.
type Options struct {
	// Title is what the browser tab says. Empty takes the score's own.
	Title string

	// SVG is how the music itself is drawn.
	SVG svg.Options

	// FontURL links the music font instead of carrying it.
	//
	// Embedded, the page is about 1.2 MB and works from a memory stick with
	// the network off. Linked, it is a few tens of kilobytes and needs the
	// address to answer — which is right for a page on a site that serves
	// the font beside it, and wrong for a file sent to somebody. Empty
	// means embed it, because standing alone is what the page is for.
	FontURL string

	// Sound is what the page plays and follows: when every note sounds, how
	// long the bars are, and where each was drawn. nil leaves the page a
	// drawing with no transport on it.
	//
	// It is handed over rather than worked out here, because resolving a
	// score into times is the program's job and not a web page's.
	Sound *Sound

	// ScriptURL fetches the player from an address instead of writing it
	// into the page. Empty writes it in, which is a few kilobytes; it is
	// separate from the font because the font is the large one and the two
	// are not always wanted in the same place.
	//
	// It is only used when there is a player to fetch.
	ScriptURL string
}

// DefaultOptions draws the music at a readable size, standing on its own.
func DefaultOptions() Options {
	o := svg.DefaultOptions()
	o.EmbedFont = true
	return Options{SVG: o}
}

// PlayerJS is the player itself, for serving beside a page that links to it
// rather than carrying it.
func PlayerJS() string { return playerJS }

// Render writes the page.
func Render(w io.Writer, lay *engrave.Layout, score *model.Score,
	symbols *symbol.Registry, opt Options) error {

	var art bytes.Buffer
	sv := opt.SVG
	// The font is written in unless an address was given to fetch it from.
	sv.EmbedFont = opt.FontURL == ""
	// The noteheads carry their numbers when there is a player to light
	// them up, and not otherwise: it is a few bytes a note.
	sv.Interactive = sv.Interactive || opt.Sound != nil
	if err := svg.Render(&art, lay, sv); err != nil {
		return err
	}

	title := opt.Title
	if title == "" && score != nil {
		title = score.Identity.Title
	}
	if title == "" {
		title = "Score"
	}
	by := ""
	if score != nil {
		by = score.Identity.Composer
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "<!doctype html>\n<html lang=\"en\">\n<meta charset=\"utf-8\">\n")
	fmt.Fprintf(&b, "<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s — %s</title>\n", html.EscapeString(title), brand.Name)
	if by != "" {
		fmt.Fprintf(&b, "<meta name=\"author\" content=\"%s\">\n", html.EscapeString(by))
	}
	fmt.Fprintf(&b, "<meta name=\"generator\" content=\"%s\">\n", brand.Name)
	b.WriteString(style)
	if opt.FontURL != "" {
		// Linked rather than carried. The family is the one the drawing
		// asks for, so the two have to agree.
		fmt.Fprintf(&b, "<style>@font-face{font-family:%q;src:url(%q);}</style>\n",
			smufl.FontFamily, opt.FontURL)
	}
	if opt.Sound != nil {
		fmt.Fprintf(&b, "<style>%s</style>\n", playerCSS)
	}
	b.WriteString("<body>\n")
	if opt.Sound != nil {
		b.WriteString(playerHTML)
	}
	b.WriteString("<main>\n")
	b.Write(art.Bytes())
	b.WriteString("\n</main>\n")

	// A line saying where it came from. It is not printed: a score on paper
	// is the music and nothing else.
	fmt.Fprintf(&b, "<footer>Engraved with %s</footer>\n", brand.Name)

	if opt.Sound != nil {
		data, err := soundJSON(opt.Sound)
		if err != nil {
			return err
		}
		// As data, not as a script to run: a type the browser does not know
		// is a type it does not execute, and the player reads it from here.
		b.WriteString("<script id=\"gscore-sound\" type=\"application/json\">\n")
		b.Write(data)
		b.WriteString("\n</script>\n")
		if opt.ScriptURL != "" {
			fmt.Fprintf(&b, "<script src=%q></script>\n", opt.ScriptURL)
		} else {
			fmt.Fprintf(&b, "<script>%s</script>\n", playerJS)
		}
	}
	b.WriteString("</html>\n")

	_, err := w.Write(b.Bytes())
	return err
}

// The page's own look: white paper, the music centred, and out of the way
// when it is printed.
const style = `<style>
  :root { color-scheme: light; }
  body {
    margin: 0; padding: 24px 16px 64px;
    background: #f4f4f2; color: #111;
    font: 14px/1.5 Georgia, "Times New Roman", serif;
  }
  main {
    max-width: 1100px; margin: 0 auto; padding: 24px;
    background: #fff; border-radius: 4px;
    box-shadow: 0 1px 4px rgba(0,0,0,.18);
    overflow-x: auto;
  }
  svg.gscore-score { display: block; width: 100%; height: auto; }
  footer {
    max-width: 1100px; margin: 16px auto 0;
    font-size: 12px; color: #666; text-align: center;
  }
  @media print {
    body { background: #fff; padding: 0; }
    main { max-width: none; margin: 0; padding: 0; box-shadow: none; }
    footer { display: none; }
  }
</style>
`
