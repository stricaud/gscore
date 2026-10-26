/*
 * musx.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package musx reads Finale's .musx files.
//
// Finale was discontinued in 2024 and its files did not go anywhere: people
// have decades of them, and the program that could open them is no longer
// sold. The format is nobody's secret any more — the container is a zip, the
// obfuscation over the music is a fixed stream cipher, and what is inside is
// Finale's own document model as XML, documented unofficially by Project
// Attacca. container.go gets at it; enigma.go reads it.
//
// What is read is the music: the staves of the score in score order, their
// bars, meters, clefs and barlines, and in each bar the notes, chords and
// rests with their pitches and lengths. What is not read is everything
// Finale writes around the music, and every one of those is named in the log
// the editor shows rather than dropped in silence — a score that opens
// missing its slurs should say so.
package musx

import (
	"io"
	"strings"

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
		return "one thing in this Finale file was not read"
	}
	return strings.Join([]string{
		"some of what Finale wrote around the music was not read",
	}, "")
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
