/*
 * gscore.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package gscore reads and writes .gscore files, the program's own format.
//
// A .gscore file is a ZIP container, the way .mscz and .docx are:
//
//	META-INF/format.json   what this is and which version
//	score.json             the music
//	symbols/index.json     the custom symbols' metrics
//	symbols/*.xpm          the custom symbols themselves
//
// The container exists for the last two lines. MusicXML is the right format
// for exchanging music with other programs, and gscore writes it, but it
// cannot carry a symbol the user drew: MuseScore can embed a picture, but only
// as decoration floating on the page, with no musical meaning. A score that
// uses an invented articulation would arrive somewhere else as a smudge.
//
// Here the artwork travels with the music and stays attached to the note it
// belongs to, so a score is still a score when it comes back.
package gscore

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/symbol"
	"github.com/stricaud/gscore/internal/version"
)

func init() {
	format.RegisterImporter(importer{})
	format.RegisterExporter(exporter{})
}

type importer struct{}

func (importer) Name() string         { return "gscore" }
func (importer) Extensions() []string { return []string{".gscore"} }

func (importer) Import(r io.Reader) (*model.Score, error) {
	s, _, err := Read(r)
	return s, err
}

type exporter struct{}

func (exporter) Name() string         { return "gscore" }
func (exporter) Extensions() []string { return []string{".gscore"} }

func (exporter) Export(w io.Writer, s *model.Score) error { return Write(w, s, nil) }

// FormatVersion is the container's version. It is bumped only when an older
// reader could no longer make sense of a newer file.
const FormatVersion = 1

type formatInfo struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	Generator string `json:"generator"`
}

/*
 * Write
 */

// Write saves a score, together with whatever symbols it uses.
//
// Only the symbols the score actually refers to are written, so a file does
// not carry a whole library around for the sake of one mark.
func Write(w io.Writer, s *model.Score, symbols *symbol.Registry) error {
	zw := zip.NewWriter(w)

	put := func(name string, data []byte) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}
	putJSON := func(name string, v any) error {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		return put(name, append(b, '\n'))
	}

	if err := putJSON("META-INF/format.json", formatInfo{
		Format: "gscore", Version: FormatVersion,
		Generator: "gscore " + version.Version,
	}); err != nil {
		return err
	}
	if err := putJSON("score.json", encodeScore(s)); err != nil {
		return err
	}

	used := usedSymbols(s)
	if symbols != nil && len(used) > 0 {
		index := make([]symbolEntry, 0, len(used))
		for _, name := range used {
			sym, ok := symbols.Lookup(name)
			if !ok || sym.Kind != symbol.FromPixels || sym.Pixels == nil {
				continue
			}
			file := "symbols/" + safeName(name) + ".xpm"
			if err := put(file, symbol.EncodeXPM(sym.Pixels, name)); err != nil {
				return err
			}
			index = append(index, symbolEntry{
				Name: name, File: file,
				PixelSize: sym.Pixels.PixelSize,
				OriginX:   sym.Pixels.OriginX,
				OriginY:   sym.Pixels.OriginY,
				Advance:   sym.Advance,
			})
		}
		if len(index) > 0 {
			if err := putJSON("symbols/index.json", index); err != nil {
				return err
			}
		}
	}
	return zw.Close()
}

type symbolEntry struct {
	Name      string  `json:"name"`
	File      string  `json:"file"`
	PixelSize float64 `json:"pixelSize"`
	OriginX   float64 `json:"originX"`
	OriginY   float64 `json:"originY"`
	Advance   float64 `json:"advance,omitempty"`
}

// usedSymbols lists, once each and in a stable order, every symbol the score
// names.
func usedSymbols(s *model.Score) []string {
	seen := map[string]bool{}
	var out []string
	add := func(m *model.Marks) {
		for _, sm := range m.Symbols {
			if !seen[sm.Name] {
				seen[sm.Name] = true
				out = append(out, sm.Name)
			}
		}
	}
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					switch e := el.(type) {
					case *model.Note:
						add(&e.Marks)
					case *model.Chord:
						add(&e.Marks)
					case *model.Rest:
						add(&e.Marks)
					case *model.SymbolElement:
						if !seen[e.Symbol.Name] {
							seen[e.Symbol.Name] = true
							out = append(out, e.Symbol.Name)
						}
					}
				}
			}
		}
	}
	return out
}

func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "symbol"
	}
	return b.String()
}

/*
 * Read
 */

// Read loads a score and any symbols stored with it.
func Read(r io.Reader) (*model.Score, *symbol.Registry, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("gscore: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, fmt.Errorf("gscore: not a .gscore container: %w", err)
	}

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}

	if f, ok := files["META-INF/format.json"]; ok {
		var info formatInfo
		if b, err := readAll(f); err == nil {
			_ = json.Unmarshal(b, &info)
		}
		if info.Format != "" && info.Format != "gscore" {
			return nil, nil, fmt.Errorf("gscore: this is a %q file", info.Format)
		}
		if info.Version > FormatVersion {
			return nil, nil, fmt.Errorf(
				"gscore: the file is version %d and this is gscore %s, which reads up to version %d",
				info.Version, version.Version, FormatVersion)
		}
	}

	sf, ok := files["score.json"]
	if !ok {
		return nil, nil, errors.New("gscore: the container holds no score.json")
	}
	b, err := readAll(sf)
	if err != nil {
		return nil, nil, fmt.Errorf("gscore: %w", err)
	}
	var doc scoreDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, nil, fmt.Errorf("gscore: bad score.json: %w", err)
	}
	s, err := decodeScore(&doc)
	if err != nil {
		return nil, nil, err
	}

	reg := symbol.NewRegistry()
	if f, ok := files["symbols/index.json"]; ok {
		if b, err := readAll(f); err == nil {
			var index []symbolEntry
			if json.Unmarshal(b, &index) == nil {
				for _, e := range index {
					sf, ok := files[e.File]
					if !ok {
						continue
					}
					raw, err := readAll(sf)
					if err != nil {
						continue
					}
					bm, err := symbol.DecodeXPM(bytes.NewReader(raw))
					if err != nil {
						continue
					}
					if e.PixelSize > 0 {
						bm.PixelSize = e.PixelSize
					}
					bm.OriginX, bm.OriginY = e.OriginX, e.OriginY
					_ = reg.Add(&symbol.Symbol{
						Name: e.Name, Kind: symbol.FromPixels,
						Pixels: bm, Advance: e.Advance,
					})
				}
			}
		}
	}
	return s, reg, nil
}

func readAll(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
