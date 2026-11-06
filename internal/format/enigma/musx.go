/*
 * musx.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// This file is the .musx importer: the newest of the three containers this
// package reads, and the one that gave the package its old name.
//
// The container itself is in container.go — a zip, with a fixed stream cipher
// over the one member that holds the music. What comes out is EnigmaXML, and
// enigma.go reads that. So there is very little here: a registration, an
// error type, and the handful of lines that join the two.
//
// The package doc, in enigma.go, says how this stands beside the two .mus
// containers and why all three share a document model.
package enigma

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
