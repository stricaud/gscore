/*
 * scanned.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package scanned reads a photographed or scanned page of music.
//
// A picture of a page holds no notes, only ink, and turning the one into the
// other is optical music recognition: a trained model, a gigabyte of Python
// and a minute of a processor's time. None of that belongs inside gscore.
// What is here runs the recogniser as a program of its own — homr, which
// reads PNG and JPEG and writes MusicXML — and reads what it wrote with the
// MusicXML importer gscore already has.
//
// How the program is found is the application's business, not this
// package's: an environment variable, a setting, a copy shipped beside the
// binary, or uv fetching it on first use. The application says so through
// Find, and this package only runs what it is handed.
package scanned

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/format/musicxml"
	"github.com/stricaud/gscore/internal/model"
)

// Engine is the page reader as it is run: the command and its arguments,
// which the pages are added to, and where the command was found, for
// telling the person what is being used.
type Engine struct {
	Command []string
	Source  string
}

// String is the command as it would be typed.
func (e Engine) String() string { return strings.Join(e.Command, " ") }

// ErrNoEngine says no page reader could be found. The application's Find
// wraps it with what to do about that.
var ErrNoEngine = errors.New("no page reader is installed")

// Find is how the application says which page reader to run. Until it is
// set there is none.
var Find = func() (Engine, error) { return Engine{}, ErrNoEngine }

func init() { format.RegisterImporter(importer{}) }

type importer struct{}

func (importer) Name() string                             { return "Scanned page" }
func (importer) Extensions() []string                     { return []string{".png", ".jpg", ".jpeg"} }
func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// Import reads one page handed over as its bytes.
func Import(r io.Reader) (*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("scanned: %w", err)
	}
	ext := Kind(data)
	if ext == "" {
		return nil, errors.New("scanned: this is neither a PNG, a JPEG nor a PDF")
	}
	eng, err := Find()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "gscore-scan-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "page"+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	return ImportFiles(context.Background(), eng, []string{path})
}

// Kind is the extension a page goes by, from its first bytes: ".png",
// ".jpg", ".pdf" — a PDF of scanned pages, which the reader renders itself
// — or nothing for anything else. The reader picks its decoder by the
// name, so the name has to tell the truth whatever the file was called.
func Kind(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	case bytes.HasPrefix(data, []byte("%PDF")):
		return ".pdf"
	}
	return ""
}

// Progress is how far the reader has got: which page of how many, and on
// it which staff of how many, as it says itself while it works. Percent is
// the two together, kept under a hundred until the music is written.
type Progress struct {
	Page, Pages   int
	Staff, Staves int
	Percent       int
}

// String says where the reader is, for a status line.
func (p Progress) String() string {
	switch {
	case p.Pages == 0:
		return ""
	case p.Staves == 0:
		return fmt.Sprintf("page %d of %d", p.Page, p.Pages)
	}
	return fmt.Sprintf("page %d of %d, staff %d of %d", p.Page, p.Pages, p.Staff, p.Staves)
}

// progressWriter reads the reader's own account of its work as it is
// written, a line at a time, and reports where it has got to.
//
// What homr says, in the order it says it: "Merging N files" once the
// PDFs are rendered and the pages counted; "Processing <page>" at each
// page; "Found N staffs" when the page is segmented; "Running TrOmr
// inference on staff image i" for each staff; "Finished <page>" at the
// end of each. The segmentation is counted as a little of the page, since
// it takes a few seconds and would otherwise read as nothing happening.
type progressWriter struct {
	line   []byte
	now    Progress
	report func(Progress)
}

func (w *progressWriter) Write(b []byte) (int, error) {
	for _, c := range b {
		if c != '\n' {
			w.line = append(w.line, c)
			continue
		}
		w.saw(string(w.line))
		w.line = w.line[:0]
	}
	return len(b), nil
}

func (w *progressWriter) saw(line string) {
	line = strings.TrimSpace(line)
	var n int
	switch {
	case strings.HasPrefix(line, "Merging ") && strings.Contains(line, " files"):
		fmt.Sscanf(line, "Merging %d files", &n)
		w.now.Pages = n
	case strings.HasPrefix(line, "Processing "):
		w.now.Page++
		w.now.Staff, w.now.Staves = 0, 0
	case strings.HasPrefix(line, "Found ") && strings.HasSuffix(line, " staffs"):
		fmt.Sscanf(line, "Found %d staffs", &n)
		w.now.Staves, w.now.Staff = n, 0
	case strings.HasPrefix(line, "Running TrOmr inference on staff image "):
		fmt.Sscanf(line, "Running TrOmr inference on staff image %d", &n)
		w.now.Staff = n + 1
	case strings.HasPrefix(line, "Finished parsing "):
		w.now.Staff = w.now.Staves
	default:
		return
	}
	w.now.Percent = percentOf(w.now)
	if w.report != nil {
		w.report(w.now)
	}
}

// percentOf is how much of the whole a position is, under a hundred.
func percentOf(p Progress) int {
	if p.Pages == 0 || p.Page == 0 {
		return 0
	}
	done := float64(p.Page - 1)
	switch {
	case p.Staves > 0:
		done += 0.1 + 0.9*float64(p.Staff)/float64(p.Staves)
	default:
		done += 0.05
	}
	pct := int(100 * done / float64(p.Pages))
	if pct > 99 {
		pct = 99
	}
	return pct
}

// ImportFiles reads the pages named, in that order, as one score.
func ImportFiles(ctx context.Context, eng Engine, paths []string) (*model.Score, error) {
	return ImportFilesWith(ctx, eng, paths, nil)
}

// ImportFilesWith is ImportFiles telling report how far it has got, where
// report is not nil. It is called from the goroutine that reads the
// reader's output, as the lines arrive.
//
// The reader is run once over all of them: it joins the pages itself,
// carrying the time signature from one to the next, which is more than
// reading them one at a time and gluing the results could do. It writes
// its MusicXML beside the first picture it is given, so the pictures are
// copied into a folder of their own and named in order first.
func ImportFilesWith(ctx context.Context, eng Engine, paths []string, report func(Progress)) (*model.Score, error) {
	if len(eng.Command) == 0 {
		return nil, ErrNoEngine
	}
	if len(paths) == 0 {
		return nil, errors.New("scanned: no pages to read")
	}
	dir, err := os.MkdirTemp("", "gscore-scan-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	var pages []string
	for i, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		ext := Kind(data)
		if ext == "" {
			return nil, fmt.Errorf("scanned: %s is neither a PNG, a JPEG nor a PDF", filepath.Base(p))
		}
		name := fmt.Sprintf("page-%02d%s", i+1, ext)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return nil, err
		}
		pages = append(pages, name)
	}

	// The title is quicker to type than to correct, and reading it costs
	// a second model's download.
	args := append(append([]string{}, eng.Command[1:]...), "--no-title")
	args = append(args, pages...)
	cmd := exec.CommandContext(ctx, eng.Command[0], args...)
	cmd.Dir = dir
	var said bytes.Buffer
	var out io.Writer = &said
	if report != nil {
		out = io.MultiWriter(&said, &progressWriter{report: report})
	}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("scanned: the page reader was stopped before it finished")
		}
		return nil, fmt.Errorf("scanned: the page reader failed: %w%s", err, tail(said.String()))
	}

	// One page writes page-01.musicxml; several are joined into a file of
	// the first page's name.
	f, err := os.Open(filepath.Join(dir, "page-01.musicxml"))
	if err != nil {
		return nil, fmt.Errorf("scanned: the page reader wrote no music%s", tail(said.String()))
	}
	defer f.Close()
	return musicxml.Import(f)
}

// tail is the last few lines of what the reader said, for an error message:
// the reason is at the end, after a screenful of progress.
func tail(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 6 {
		lines = lines[len(lines)-6:]
	}
	var out []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return "\n" + strings.Join(out, "\n")
}
