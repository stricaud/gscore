/*
 * musx.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package musx reads Finale's files: the .musx it wrote from 2014, and the
// older binary .mus it wrote until 2012.
//
// Finale was discontinued in 2024 and its files did not go anywhere: people
// have decades of them, and the program that could open them is no longer
// sold. The .musx is nobody's secret any more — the container is a zip, the
// obfuscation over the music is a fixed stream cipher, and what is inside is
// Finale's own document model as XML, documented unofficially by Project
// Attacca. container.go gets at it; enigma.go reads it.
//
// The .mus is a different container entirely, and really two of them, since
// Finale changed how it compressed the payload in 2011. Those are in mus.go,
// muspool.go, musentries.go and musdoc.go, and they are kept apart from the
// .musx on purpose: separate importers, separate errors, separate reports.
// The two formats meet in exactly one place, build() in enigma.go, because
// the document inside them is the same document — which is the only reason
// reading the older one is a few hundred lines rather than a second program.
//
// What is read is the music: the staves of the score in score order, their
// bars, meters, clefs and barlines, and in each bar the notes, chords and
// rests with their pitches and lengths. What is not read is everything
// Finale writes around the music, and every one of those is named in the log
// the editor shows rather than dropped in silence — a score that opens
// missing its slurs should say so. A .mus loses more than a .musx does, and
// says so in its own words.
package musx

import (
	"fmt"
	"io"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string         { return "Finale" }
func (importer) Extensions() []string { return []string{".musx"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// PartialError is a score that was read with things left out.
type PartialError struct {
	Skipped []string
}

func (e *PartialError) Error() string {
	if len(e.Skipped) == 1 {
		return "one kind of thing in this Finale file was not read"
	}
	return fmt.Sprintf("%d kinds of thing in this Finale file were not read",
		len(e.Skipped))
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details implements format.Partial.
func (e *PartialError) Details() []string { return e.Skipped }

// Import reads a Finale score.
//
// A file gscore cannot read at all is an error; a file it can read most of
// comes back as the music plus a list of what was left behind, which is the
// rule every importer here follows. Somebody with a Finale file and no
// Finale would rather have the notes and a list of what is missing than a
// refusal.
func Import(r io.Reader) (*model.Score, error) {
	data, meta, err := open(r)
	if err != nil {
		return nil, err
	}
	score, skipped, err := read(data, meta)
	if err != nil {
		return nil, err
	}
	if len(skipped) > 0 {
		return score, &PartialError{Skipped: skipped}
	}
	return score, nil
}
