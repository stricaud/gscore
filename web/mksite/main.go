/*
 * main.go - assembles the gscore web site.
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

/*
 * What this does, and what it deliberately does not.
 *
 * The music on the site is engraved by gscore itself: web/build.sh runs the
 * real binary over samples/ and writes a self-contained page for each piece,
 * the same page anybody gets from "Export as a web page". This program takes
 * those pages apart for the drawing and the performance inside them, drops
 * both into the front page, and renders the documentation.
 *
 * It does not engrave anything. Resolving a score into notes and times is
 * the program's job and is tested as such; a site builder that did it again,
 * slightly differently, would be a second answer to the same question and
 * the one nobody checks.
 */

package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	ghtml "github.com/yuin/goldmark/renderer/html"
)

func main() {
	var (
		out     = flag.String("out", "web/dist", "the directory to build into")
		site    = flag.String("site", "web/site", "the page shells and their assets")
		docsDir = flag.String("docs", "docs", "the markdown to publish")
		samples = flag.String("samples", "samples", "the scores to show")
		version = flag.String("version", "", "what to print as the version")
	)
	flag.Parse()

	if err := run(*out, *site, *docsDir, *samples, *version); err != nil {
		fmt.Fprintln(os.Stderr, "mksite:", err)
		os.Exit(1)
	}
}

func run(out, site, docsDir, samples, version string) error {
	pieces, err := readPieces(samples, filepath.Join(out, "scores"))
	if err != nil {
		return err
	}
	if len(pieces) == 0 {
		return fmt.Errorf("no scores were built into %s/scores", out)
	}
	if err := writeLanding(out, site, version, pieces); err != nil {
		return err
	}
	if err := writeGallery(out, site, pieces); err != nil {
		return err
	}
	return writeDocs(out, site, docsDir)
}

/*
 * The scores
 */

// piece is one sample, and everything the site says about it.
type piece struct {
	ID       string // the file's base name, which is also its slug
	Title    string
	Composer string
	Note     string // the first comment line of the ABC, if it reads as prose
	ABC      string // only for a score written as ABC, which fits in a link
	Source   string // the file it was written in, as published
	Kind     string // what that file is: abc, mxl, musicxml, gscore
	SVG      string // lifted out of the page gscore wrote
	Sound    string // the performance, as the JSON gscore wrote
	Ways     []way  // every format it was exported to
}

// way is one file a piece can be had as.
type way struct {
	Label string
	File  string
}

var (
	svgTag   = regexp.MustCompile(`(?s)<svg[^>]*class="gscore-score".*?</svg>`)
	soundTag = regexp.MustCompile(`(?s)<script id="gscore-sound"[^>]*>(.*?)</script>`)
)

// scoreFile says whether a file in samples/ is a score gscore can read.
func scoreFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".abc", ".mxl", ".musicxml", ".xml", ".mscz", ".mscx", ".nwctxt", ".gscore":
		return true
	}
	return false
}

func readPieces(samples, built string) ([]piece, error) {
	all, err := filepath.Glob(filepath.Join(samples, "*"))
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range all {
		if scoreFile(f) {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	files = inOrder(files, filepath.Join(samples, "ORDER"))

	var out []piece
	for _, f := range files {
		base := filepath.Base(f)
		id := strings.TrimSuffix(base, filepath.Ext(base))
		// Only a score written as ABC can be carried in a link: it is the
		// one format small enough to fit in one. Anything else is shared as
		// the page built for it, which is a few tens of kilobytes and still
		// needs nothing at the other end.
		var abc []byte
		if strings.EqualFold(filepath.Ext(f), ".abc") {
			if abc, err = os.ReadFile(f); err != nil {
				return nil, err
			}
		}
		page, err := os.ReadFile(filepath.Join(built, id+".html"))
		if err != nil {
			// The score did not get a page, which means gscore could not
			// read it. Saying so beats a gallery with a hole in it.
			return nil, fmt.Errorf("%s has no built page: %w", id, err)
		}
		svg := svgTag.Find(page)
		sound := soundTag.FindSubmatch(page)
		if svg == nil || sound == nil {
			return nil, fmt.Errorf("%s: the built page has no score in it", id)
		}

		p := piece{
			ID:    id,
			ABC:   string(abc),
			SVG:   string(svg),
			Sound: strings.TrimSpace(string(sound[1])),
		}
		p.Title, p.Composer, p.Note = readHeader(string(abc))
		// What gscore itself said the piece is, which is the only thing
		// that can read every format in samples/.
		if said, err := os.ReadFile(filepath.Join(built, id+".txt")); err == nil {
			t, c := described(string(said))
			if t != "" {
				p.Title = t
			}
			if c != "" {
				p.Composer = c
			}
		}
		if p.Title == "" {
			p.Title = id
		}
		p.Kind = strings.TrimPrefix(strings.ToLower(filepath.Ext(f)), ".")
		p.Source = id + "." + p.Kind
		p.Ways = waysFor(built, id)
		out = append(out, p)
	}
	return out, nil
}

// readHeader pulls the title and the credit out of an ABC file, and the
// first line of prose above them if the file opens with one.
func readHeader(abc string) (title, composer, note string) {
	for _, line := range strings.Split(abc, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "T:") && title == "":
			title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "C:") && composer == "":
			composer = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "%") && note == "":
			s := strings.TrimSpace(strings.TrimLeft(line, "% "))
			// A sentence, not a machine-written banner.
			if len(s) > 20 && !strings.Contains(s, "://") &&
				!strings.HasPrefix(s, "File generated") {
				note = s
			}
		case strings.HasPrefix(line, "K:"):
			return // the header ends at the key
		}
	}
	return
}

// described reads what `gscore -describe` said about a file.
func described(s string) (title, composer string) {
	for _, line := range strings.Split(s, "\n") {
		f := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(f) != 2 {
			continue
		}
		switch f[0] {
		case "title":
			if title == "" {
				title = strings.TrimSpace(f[1])
			}
		case "composer":
			if composer == "" {
				composer = strings.TrimSpace(f[1])
			}
		}
	}
	return
}

// waysFor lists the formats a piece was actually exported to, in the order
// somebody is likely to want them.
func waysFor(built, id string) []way {
	want := []way{
		{"web page", id + ".html"},
		{"PDF", id + ".pdf"},
		{"MusicXML", id + ".musicxml"},
		{"LilyPond", id + ".ly"},
		{"MIDI", id + ".mid"},
		{"ABC", id + ".abc"},
		{"MusicXML (original)", id + ".mxl"},
	}
	var out []way
	for _, w := range want {
		if st, err := os.Stat(filepath.Join(built, w.File)); err == nil && st.Size() > 0 {
			out = append(out, w)
		}
	}
	return out
}

/*
 * The front page
 */

func writeLanding(out, site, version string, pieces []piece) error {
	shell, err := os.ReadFile(filepath.Join(site, "index.html.in"))
	if err != nil {
		return err
	}

	var tabs, blocks bytes.Buffer
	for _, p := range pieces {
		fmt.Fprintf(&tabs, "<button type=\"button\" data-id=%q aria-pressed=\"false\">%s</button>\n",
			p.ID, html.EscapeString(p.Title))

		fmt.Fprintf(&blocks, `<div class="piece" data-id=%q data-title=%q data-composer=%q data-note=%q data-file=%q data-src=%q data-kind=%q hidden>
<script type="application/json" class="sound">%s</script>
<script type="text/plain" class="abc">%s</script>
<div class="paper">%s</div>
</div>
`,
			p.ID, p.Title, p.Composer, p.Note, p.ID+".html",
			p.Source, p.Kind,
			p.Sound, escapeForScript(p.ABC), p.SVG)
	}

	page := string(shell)
	page = strings.ReplaceAll(page, "@TABS@", tabs.String())
	page = strings.ReplaceAll(page, "@PIECES@", blocks.String())
	page = strings.ReplaceAll(page, "@VERSION@", html.EscapeString(version))
	return os.WriteFile(filepath.Join(out, "index.html"), []byte(page), 0o644)
}

// escapeForScript keeps text safe inside a <script> block, where the parser
// is looking for a closing tag and nothing else.
func escapeForScript(s string) string {
	return strings.ReplaceAll(s, "</", "<\\/")
}

/*
 * The gallery
 */

func writeGallery(out, site string, pieces []piece) error {
	var body bytes.Buffer
	body.WriteString("<h1>Scores</h1>\n")
	body.WriteString(`<p>Every one of these was engraved by gscore. Each page stands on its own — ` +
		`open it, press play, send it to somebody. There is nothing behind it.</p>` + "\n")
	body.WriteString("<ul class=\"index\">\n")
	for _, p := range pieces {
		fmt.Fprintf(&body, "<li>\n<h3><a href=%q>%s</a></h3>\n",
			p.ID+".html", html.EscapeString(p.Title))
		if p.Composer != "" {
			fmt.Fprintf(&body, "<p>%s</p>\n", html.EscapeString(p.Composer))
		}
		if p.Note != "" {
			fmt.Fprintf(&body, "<p>%s</p>\n", html.EscapeString(p.Note))
		}
		body.WriteString("<p class=\"ways\">")
		for _, w := range p.Ways {
			fmt.Fprintf(&body, "<a href=%q download>%s</a>", w.File, html.EscapeString(w.Label))
		}
		body.WriteString("</p>\n</li>\n")
	}
	body.WriteString("</ul>\n")

	// The gallery is written on the dark page rather than on paper: it is a
	// list of things, not something to read.
	page, err := shellPage(site, "Scores", "../", body.String())
	if err != nil {
		return err
	}
	page = strings.Replace(page, `<article class="sheet">`, `<article>`, 1)
	return os.WriteFile(filepath.Join(out, "scores", "index.html"), []byte(page), 0o644)
}

/*
 * The documentation
 */

var mdLink = regexp.MustCompile(`href="([^":]*)\.md(#[^"]*)?"`)

func writeDocs(out, site, docsDir string) error {
	files, err := filepath.Glob(filepath.Join(docsDir, "*.md"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no markdown in %s", docsDir)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(ghtml.WithUnsafe()),
	)

	type entry struct{ name, title, blurb string }
	var index []entry

	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := md.Convert(src, &buf); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		// A link from one document to another was written as a path to the
		// markdown, which is right in the repository and a dead end here.
		body := mdLink.ReplaceAllString(buf.String(), `href="$1.html$2"`)

		name := strings.TrimSuffix(filepath.Base(f), ".md")
		title, blurb := summarise(string(src))
		page, err := shellPage(site, title, "../", body)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, "docs", name+".html"),
			[]byte(page), 0o644); err != nil {
			return err
		}
		index = append(index, entry{name, title, blurb})
	}

	var body bytes.Buffer
	body.WriteString("<h1>Documentation</h1>\n<ul class=\"index\">\n")
	for _, e := range index {
		fmt.Fprintf(&body, "<li>\n<h3><a href=%q>%s</a></h3>\n<p>%s</p>\n</li>\n",
			e.name+".html", html.EscapeString(e.title), html.EscapeString(e.blurb))
	}
	body.WriteString("</ul>\n")

	page, err := shellPage(site, "Documentation", "../", body.String())
	if err != nil {
		return err
	}
	page = strings.Replace(page, `<article class="sheet">`, `<article>`, 1)
	return os.WriteFile(filepath.Join(out, "docs", "index.html"), []byte(page), 0o644)
}

// summarise takes a document's own heading and its first sentence, so the
// index says what each one is without anybody maintaining a second copy of
// the description.
func summarise(src string) (title, blurb string) {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case title == "" && strings.HasPrefix(line, "# "):
			title = strings.TrimSpace(line[2:])
		case title != "" && blurb == "" && line != "" &&
			!strings.HasPrefix(line, "#") && !strings.HasPrefix(line, ">"):
			blurb = line
			if i := strings.Index(blurb, ". "); i > 40 {
				blurb = blurb[:i+1]
			}
			return
		}
	}
	return
}

/*
 * The shell both of them sit in
 */

func shellPage(site, title, root, body string) (string, error) {
	shell, err := os.ReadFile(filepath.Join(site, "doc.html.in"))
	if err != nil {
		return "", err
	}
	page := string(shell)
	page = strings.ReplaceAll(page, "@TITLE@", html.EscapeString(title))
	page = strings.ReplaceAll(page, "@ROOT@", root)
	page = strings.ReplaceAll(page, "@BODY@", body)
	return page, nil
}

// inOrder puts the scores in the order samples/ORDER asks for.
//
// Alphabetical is an accident, and the first piece on the front page is the
// one a visitor judges the program by. So the order is written down, and
// anything the file does not mention follows after it.
func inOrder(files []string, orderFile string) []string {
	raw, err := os.ReadFile(orderFile)
	if err != nil {
		return files
	}
	rank := map[string]int{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, seen := rank[line]; !seen {
			rank[line] = len(rank)
		}
	}
	idOf := func(p string) string {
		b := filepath.Base(p)
		return strings.TrimSuffix(b, filepath.Ext(b))
	}
	sort.SliceStable(files, func(i, j int) bool {
		ri, oki := rank[idOf(files[i])]
		rj, okj := rank[idOf(files[j])]
		switch {
		case oki && okj:
			return ri < rj
		case oki:
			return true
		case okj:
			return false
		}
		return files[i] < files[j]
	})
	return files
}
