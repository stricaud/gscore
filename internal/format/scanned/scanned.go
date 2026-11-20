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
		return nil, errors.New("scanned: this is neither a PNG nor a JPEG")
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

// Kind is the extension a picture goes by, from its first bytes: ".png",
// ".jpg", or nothing for anything else. The reader picks its decoder by
// the name, so the name has to tell the truth whatever the file was called.
func Kind(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	}
	return ""
}

// ImportFiles reads the pages named, in that order, as one score.
//
// The reader is run once over all of them: it joins the pages itself,
// carrying the time signature from one to the next, which is more than
// reading them one at a time and gluing the results could do. It writes
// its MusicXML beside the first picture it is given, so the pictures are
// copied into a folder of their own and named in order first.
func ImportFiles(ctx context.Context, eng Engine, paths []string) (*model.Score, error) {
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
			return nil, fmt.Errorf("scanned: %s is neither a PNG nor a JPEG", filepath.Base(p))
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
	cmd.Stdout, cmd.Stderr = &said, &said
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
