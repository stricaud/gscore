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
	_ "github.com/stricaud/gscore/internal/format/guido"
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

		addTags tagList
	)
	flag.Var(&addTags, "tag",
		"add this tag to the score if it is not on it already, and save; "+
			"give it more than once, or several tags separated by commas")
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
			// Which file. One file opened by hand makes that obvious and a
			// run over a shelf of fifteen hundred does not: a reason with
			// no name attached leaves somebody grepping the corpus for the
			// one score that would not open.
			if path != "" {
				fmt.Fprintf(os.Stderr, "gscore: %s: %v\n", path, err)
			} else {
				fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
			}
			os.Exit(1)
		}
	}

	// Tags asked for on the command line go on before anything else is done
	// with the score, so that -o and -convert carry them as well: tagging
	// and converting in one run is one thing somebody wants to do, not two
	// that cannot be combined.
	var asked, added []string
	if len(addTags) > 0 {
		if score == nil {
			fmt.Fprintln(os.Stderr, "gscore: -tag needs a file to put the tags on")
			os.Exit(1)
		}
		if len(scores) > 1 {
			// A file holding several scores would need all of them written
			// back, and only the first is loaded here. Better to refuse
			// than to save one and drop the rest.
			fmt.Fprintf(os.Stderr, "gscore: %s holds %d scores; -tag can only "+
				"tag a file that holds one\n", path, len(scores))
			os.Exit(1)
		}
		asked = model.ParseTags(strings.Join(addTags, ","))
		added = score.AddTags(asked)
	}

	// Saving them is its own errand, and it is done before anything is
	// printed: -tag with -describe means "tag it, then tell me what it
	// says", not "tell me what it would say if it had been saved". Where
	// the score is on its way somewhere else — -convert, or -o — the tags
	// travel with it and the file it came from is left alone.
	if len(addTags) > 0 && *convert == "" && *out == "" {
		if err := saveTags(score, path, added, asked, !*noPDF); err != nil {
			fmt.Fprintf(os.Stderr, "gscore: %v\n", err)
			os.Exit(1)
		}
		if !*describe {
			return
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

// tagList collects -tag, which may be given more than once.
//
// Both ways of saying it, because both are natural at a prompt and in a
// script: -tag advent -tag "to learn", and -tag "advent, to learn". They
// end up in one line for the model to divide, so the comma means the same
// thing on the command line as it does in the editor's tag window.
type tagList []string

func (t *tagList) String() string { return strings.Join(*t, ", ") }

func (t *tagList) Set(v string) error {
	*t = append(*t, v)
	return nil
}

// saveTags writes a score that has just been tagged back where it came
// from.
//
// In place, because that is what tagging a shelf of scores means: the file
// keeps its name, and the next run of the same command over the same shelf
// does nothing at all. Through a temporary file beside it and a rename, so
// that a run interrupted halfway leaves the score as it was rather than
// half of one.
//
// Only a .gscore. Tags are gscore's own field and no other format has
// anywhere to put them, so writing a tagged score back over a MusicXML
// file would quietly throw the tags away — and rewrite the music through
// an exporter while it was at it, which is not what was asked for.
func saveTags(s *model.Score, path string, added, asked []string, withPDF bool) error {
	if path == "" {
		return fmt.Errorf("-tag needs a file to put the tags on")
	}
	if !strings.EqualFold(filepath.Ext(path), ".gscore") {
		return fmt.Errorf("%s is not a .gscore, and no other format carries "+
			"tags: convert it first with -convert OUT.gscore", path)
	}
	if len(added) == 0 {
		// Nothing to write. Saying so matters more than it looks: the whole
		// point of asking for tags that are not there already is that
		// running the same command twice is safe, and silence would leave
		// somebody wondering whether it worked the first time.
		fmt.Printf("%s: already tagged %s\n", path, strings.Join(asked, ", "))
		return nil
	}

	// What the file was before we touched it. Adding a tag is the smallest
	// edit there is and it should leave everything else as it found it: the
	// file keeps the permissions it had, which a fresh temporary file does
	// not, and it keeps the drawing it had in front of it — the one that is
	// there, not a fresh one. A score saved without a PDF stays that way
	// too: put one in and a two-kilobyte file comes back as four hundred.
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	was, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pdf := gscorefmt.PDFPrefix(was)
	if !withPDF {
		pdf = nil
	}
	// And the marks whoever wrote this drew for themselves, which live in
	// the container beside the music and are not part of it. The importer
	// that loaded the score for tagging hands back the music alone, so a
	// save that did not go looking for these would write the score back
	// without the artwork its own notes point at.
	_, symbols, err := gscorefmt.Read(bytes.NewReader(was))
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".gscore-tag-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := gscorefmt.WriteWithPDF(tmp, s, symbols, pdf); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	fmt.Printf("%s: added %s\n", path, strings.Join(added, ", "))
	return nil
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
  gscore -tag advent -tag "to learn" FILE.gscore
                         add tags FILE.gscore does not have already and save
                         it; -tag "advent, to learn" says the same thing

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

	// Say which staff is which, wherever there is more than one.
	//
	// A PDF has always done this and a page never did, so the ninth symphony
	// came out as fifty-three unlabelled staves: the music is all there and
	// there is no telling which line is the oboes. It is the same question on
	// screen as on paper, and it was only ever answered on paper because the
	// name is set in the PDF branch below rather than here.
	eopt.ShowPartNames = len(s.Parts) > 1

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
	} else {
		lay = refill(s, eopt, lay)
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

// refill engraves a second time, to the width the first time actually came
// out at.
//
// A system is laid out to Options.LineWidth, and for anything but paper that
// is a flat hundred and twenty staff spaces — a sheet's worth, and the only
// width there is when there is no sheet to measure. The PDF branch above
// does not use it: gpdf.Fit works the line width out from the paper it is
// going to be printed on. A page has no paper, so nothing ever worked it out,
// and the default stood.
//
// The canvas is not that width, though. It grows to hold the widest ink
// drawn — see engrave.Layout.Width — so a single bar too wide to break drags
// it out past every other system in the score. Rhapsody in Blue is the
// extreme case: one system in a hundred and forty-three is half as wide
// again as the rest, and every other line of music stops a third of the way
// short of the right-hand edge because of that one bar.
//
// So ask the drawing how wide it came out, and engrave again to that. The
// bars repack into the room that was there all along — on the Gershwin that
// is four bars a system becoming six.
//
// Twice at most, and it keeps the better of what it has. A wider line
// repacks the bars, and repacking can hand the overfull place to a different
// bar: there is no guarantee of a fixed point to iterate to, and a build of
// a thousand scores is not where one should be looked for. A score with
// nothing overfull — most of them — measures as already full on the first
// pass and is never engraved twice at all.
func refill(s *model.Score, eopt engrave.Options, lay *engrave.Layout) *engrave.Layout {
	// Strip mode has no line to fill: the music runs as far as it runs and
	// the canvas is trimmed to it, so a width read off the drawing and fed
	// back in would be chasing its own tail.
	if lay == nil || eopt.Mode != engrave.Paginated {
		return lay
	}

	// How much of the canvas is not the line the music was set to — the
	// white strip down the right-hand side, as a fraction of the whole.
	waste := func(l *engrave.Layout, lineWidth float64) float64 {
		if l == nil || l.Width <= 0 {
			return 0
		}
		return (l.Width - (lineWidth + 2*eopt.Spacings.Margin)) / l.Width
	}

	best, worst := lay, waste(lay, eopt.LineWidth)
	for pass := 0; pass < 2; pass++ {
		// Near enough full. A hair of rounding is not worth engraving the
		// ninth symphony again for.
		if worst <= 0.001 {
			break
		}
		opt := eopt
		opt.LineWidth = best.Width - 2*eopt.Spacings.Margin
		cand := engrave.Engrave(s, opt)
		w := waste(cand, opt.LineWidth)
		if w >= worst {
			// No better, and possibly worse: a bar that fitted before now
			// does not. Keep what we had.
			break
		}
		best, worst = cand, w
	}
	return best
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
	// Engraved the way a page engraves it, names and all, or the number
	// would be of a drawing nothing is going to make.
	dopt := engrave.DefaultOptions()
	dopt.ShowPartNames = len(s.Parts) > 1
	fmt.Printf("shapes    %d\n", shapeCount(engrave.Engrave(s, dopt)))

	// Which instruments are on it, each named once, in the order they first
	// appear.
	//
	// The instrument and not the part's name, because a list to pick from
	// wants one entry for a flute: "Flöte", "Flauto 1" and "Flute" are three
	// names for the thing somebody is looking for, and a shelf that offered
	// all three would be a shelf nobody could use.
	var played []string
	named := map[string]bool{}
	for _, p := range s.Parts {
		n := p.Instrument.String()
		if n == "" || n == "unknown" || named[n] {
			continue
		}
		named[n] = true
		played = append(played, n)
	}
	if len(played) > 0 {
		fmt.Printf("instruments %s\n", strings.Join(played, ", "))
	}

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
