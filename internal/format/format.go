/*
 * format.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package format is the registry of score import and export formats.
//
// It replaces the dlopen plugin loader in legacy/src/plugin.c, which opened
// shared objects with g_module_open and looked up a gscore_plugin_init symbol.
// Go's plugin package cannot do that portably — it does not work on Windows at
// all, and demands a byte-identical toolchain where it does — so formats are
// compiled in and register themselves instead.
//
// The 0.0.9 interface was:
//
//	typedef gboolean (* GscoreImportFunction) (Score_t **score, const gchar *filename, void *user_data);
//	typedef gboolean (* GscoreExportFunction) (Score_t *score, const gchar *filename, void *user_data);
//
// The shape is kept, with a reader or writer in place of a filename so that
// conversions and tests need no files, and an error in place of the gboolean.
package format

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/stricaud/gscore/internal/model"
)

// Importer reads a score in some format.
type Importer interface {
	// Name is the format's name, as shown in a file dialog.
	Name() string
	// Extensions are the lowercase filename extensions, leading dot included.
	Extensions() []string
	// Import parses one score.
	Import(r io.Reader) (*model.Score, error)
}

// Exporter writes a score in some format.
type Exporter interface {
	Name() string
	Extensions() []string
	Export(w io.Writer, s *model.Score) error
}

var (
	mu        sync.RWMutex
	importers []Importer
	exporters []Exporter
)

// RegisterImporter adds an importer, normally from a format package's init.
func RegisterImporter(i Importer) {
	mu.Lock()
	defer mu.Unlock()
	importers = append(importers, i)
}

// RegisterExporter adds an exporter, normally from a format package's init.
func RegisterExporter(e Exporter) {
	mu.Lock()
	defer mu.Unlock()
	exporters = append(exporters, e)
}

// Importers lists the registered importers, ordered by name.
func Importers() []Importer {
	mu.RLock()
	defer mu.RUnlock()
	out := append([]Importer(nil), importers...)
	sort.Slice(out, func(a, b int) bool { return out[a].Name() < out[b].Name() })
	return out
}

// Exporters lists the registered exporters, ordered by name.
func Exporters() []Exporter {
	mu.RLock()
	defer mu.RUnlock()
	out := append([]Exporter(nil), exporters...)
	sort.Slice(out, func(a, b int) bool { return out[a].Name() < out[b].Name() })
	return out
}

func matches(exts []string, ext string) bool {
	for _, e := range exts {
		if strings.EqualFold(e, ext) {
			return true
		}
	}
	return false
}

// ImporterFor returns the importer claiming a filename's extension.
func ImporterFor(filename string) (Importer, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return nil, fmt.Errorf("format: %q has no extension to identify it by", filename)
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, i := range importers {
		if matches(i.Extensions(), ext) {
			return i, nil
		}
	}
	return nil, fmt.Errorf("format: no importer registered for %q", ext)
}

// ExporterFor returns the exporter claiming a filename's extension.
func ExporterFor(filename string) (Exporter, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return nil, fmt.Errorf("format: %q has no extension to identify it by", filename)
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, e := range exporters {
		if matches(e.Extensions(), ext) {
			return e, nil
		}
	}
	return nil, fmt.Errorf("format: no exporter registered for %q", ext)
}

// MultiImporter is implemented by formats whose files can hold more than one
// score.
//
// An ABC file is a tune book: each X: field starts another tune, and one file
// in gscore's own corpus holds forty-eight of them. Import returns the first,
// which is right for a format that holds one score and wrong for a book, so
// anything that can show more than one asks for this.
type MultiImporter interface {
	Importer
	ImportAll(r io.Reader) ([]*model.Score, error)
}

// ImportAll reads every score in a file, using the importer's own multi-score
// reader when it has one and falling back to a single score when it does not.
func ImportAll(imp Importer, r io.Reader) ([]*model.Score, error) {
	if multi, ok := imp.(MultiImporter); ok {
		scores, err := multi.ImportAll(r)
		normaliseAll(scores)
		return scores, err
	}
	s, err := imp.Import(r)
	if s == nil {
		return nil, err
	}
	scores := []*model.Score{s}
	normaliseAll(scores)
	return scores, err
}

// normaliseAll tidies what came out of a reader.
//
// Every format gets this, and it is done here rather than in each reader
// because it is not about any format: a part that stops short of the others,
// or that a failed read left with no measures at all, is a score that will
// draw wrong whichever file it came out of. A reader's job is to say what is
// in the file; this is what makes it a score.
func normaliseAll(scores []*model.Score) {
	for _, s := range scores {
		s.Normalise()
	}
}
