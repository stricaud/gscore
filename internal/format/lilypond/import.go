/*
 * import.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package lilypond

/*
 * Reading LilyPond source into a score.
 *
 * A LilyPond file is a program for an engraver, and most of what it says is
 * about the page: where to break a line, how far to nudge a slur, which
 * font. None of that is the music. What is the music — the notes and their
 * lengths, what is written on them, the staves and how they are arranged —
 * is read as text, resolved without running anything, and written into the
 * model. What cannot be read that way is said in the log rather than
 * refused: a score with one thing missing is still the score.
 *
 * A file that holds several \score blocks is a book of movements, one score
 * each, which is how the editor shows a tune book.
 */

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string         { return "LilyPond" }
func (importer) Extensions() []string { return []string{".ly", ".ily"} }

func (importer) Import(r io.Reader) (*model.Score, error) {
	scores, err := ImportAll(r)
	if len(scores) == 0 {
		return nil, err
	}
	return scores[0], err
}

func (importer) ImportAll(r io.Reader) ([]*model.Score, error) { return ImportAll(r) }

// log is what could not be read exactly, said once each.
type log struct {
	seen    map[string]int
	skipped []string
}

func (l *log) say(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if l.seen == nil {
		l.seen = map[string]int{}
	}
	l.seen[msg]++
	if l.seen[msg] == 1 {
		l.skipped = append(l.skipped, msg)
	}
}

// PartialError reports a file that was read with something left out.
type PartialError struct {
	Skipped []string
}

func (e *PartialError) Error() string {
	if len(e.Skipped) == 1 {
		return "lilypond: " + e.Skipped[0]
	}
	return fmt.Sprintf("lilypond: %d things could not be read exactly, "+
		"the first being: %s", len(e.Skipped), e.Skipped[0])
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details is what was left out, one line each.
func (e *PartialError) Details() []string { return e.Skipped }

// ImportAll reads every movement of a LilyPond file.
//
// An \include is found beside the file when the reader is the file itself,
// which is how the editor and the command line open one; read from memory
// there is no beside, and the include is reported as missing.
func ImportAll(r io.Reader) ([]*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("lilypond: %w", err)
	}
	base, path := "", "score.ly"
	if f, ok := r.(*os.File); ok && f.Name() != "" {
		if abs, err := filepath.Abs(f.Name()); err == nil {
			base, path = filepath.Dir(abs), abs
		}
	}
	return ImportText(string(data), path, base)
}

// ImportText reads LilyPond source, with the file it came from and the
// directory its includes are found in.
func ImportText(text, path, base string) ([]*model.Score, error) {
	lg := &log{}
	ctx := newContext(base, lg)
	ctx.mainPath = path
	if path != "" {
		ctx.loaded[path] = true
	}
	top := ctx.loadText(text, path)
	here := base
	if here == "" {
		here = "."
	}
	movements := ctx.movementsOf(top, here)
	if len(movements) == 0 {
		return nil, fmt.Errorf("lilypond: no music was found in %s", filepath.Base(path))
	}
	var scores []*model.Score
	for _, mv := range movements {
		s := ctx.build(mv)
		if len(s.Parts) == 0 {
			continue
		}
		scores = append(scores, s)
	}
	if len(scores) == 0 {
		return nil, fmt.Errorf("lilypond: no staves were found in %s", filepath.Base(path))
	}
	if len(lg.skipped) > 0 {
		sort.SliceStable(lg.skipped, func(a, b int) bool {
			return lg.seen[lg.skipped[a]] > lg.seen[lg.skipped[b]]
		})
		for i, msg := range lg.skipped {
			if n := lg.seen[msg]; n > 1 {
				lg.skipped[i] = fmt.Sprintf("%s (%d times)", msg, n)
			}
		}
		return scores, &PartialError{Skipped: lg.skipped}
	}
	return scores, nil
}
