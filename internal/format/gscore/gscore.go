/*
 * gscore.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	"encoding/binary"
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
	return write(w, s, symbols, usedSymbols(s))
}

// WriteWithPDF saves a score with a PDF of it in front of the container, so
// that the one file is both.
//
// The two formats are read from opposite ends. A PDF reader looks for
// %PDF- at the start and finds the cross-reference pointer near the end; a
// zip reader scans backwards from the end for the central directory and
// tolerates whatever is in front of it — which is how self-extracting
// archives have always worked. So neither trips over the other, and a
// .gscore renamed .pdf opens in anything that reads a PDF.
//
// What it costs: the file carries the engraving as well as the music, which
// for a short piece is three decimal orders more bytes. And anything that
// rewrites the PDF — a PDF editor, some mail gateways — keeps the drawing
// and silently drops the music, because what it drops is everything it did
// not understand. Editing here is safe: a save writes the whole file again.
//
// The preview is rendered by the caller. This package knows about the
// music, not about how it is drawn.
func WriteWithPDF(w io.Writer, s *model.Score, symbols *symbol.Registry, pdf []byte) error {
	if len(pdf) > 0 {
		if _, err := w.Write(pdf); err != nil {
			return err
		}
	}
	return Write(w, s, symbols)
}

// WritePack saves a .gscore holding symbols and no music.
//
// A file like this is a symbol library: somebody draws a set of marks their
// music needs and nobody else's font has, saves them, and anyone can load
// them into their own scores. The container is the same one scores use, so a
// pack opens as an empty score and a score opens as a pack — nothing new to
// learn and nothing new to implement.
func WritePack(w io.Writer, symbols *symbol.Registry) error {
	if symbols == nil || symbols.Len() == 0 {
		return errors.New("gscore: there are no symbols to save")
	}
	empty := model.NewScore()
	empty.Identity.Title = "Symbol pack"
	return write(w, empty, symbols, symbols.Names())
}

func write(w io.Writer, s *model.Score, symbols *symbol.Registry, names []string) error {
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

	if symbols != nil && len(names) > 0 {
		index := make([]symbolEntry, 0, len(names))
		for _, name := range names {
			sym, ok := symbols.Lookup(name)
			if !ok {
				continue
			}
			// An outline goes in as the outline it is.
			//
			// It used to be passed over here — only a grid of pixels was
			// written — so a symbol drawn from an SVG was on the page
			// until the file was saved and gone when it was opened again.
			// Nothing said so: the score kept the name and the registry
			// had nothing under it.
			if sym.Kind == symbol.FromVector && sym.Path != "" {
				file := "symbols/" + safeName(name) + ".path"
				if err := put(file, []byte(sym.Path)); err != nil {
					return err
				}
				index = append(index, symbolEntry{
					Name: name, File: file, Kind: "vector",
					Advance: sym.Advance,
				})
				continue
			}
			if sym.Kind != symbol.FromPixels || sym.Pixels == nil {
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
	Name string `json:"name"`
	File string `json:"file"`
	// Kind is "vector" for an outline and empty for a grid of pixels,
	// which is what every entry written before outlines were kept is.
	Kind      string  `json:"kind,omitempty"`
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

// PDFPrefix returns the PDF a .gscore carries in front of its container, or
// nothing where it carries none.
//
// For saving a file back with the drawing it already had. Engraving it again
// is the obvious way to put a PDF on a score being rewritten, and it is the
// wrong one for an edit that does not touch the music: the engraver changes
// between versions — this week it stopped drawing a slur that crossed a page
// as a line down the middle of one — so adding a tag with one gscore and
// then another would quietly redraw the music twice. Handing the old bytes
// back writes exactly the file that was there, bar the field that changed.
//
// Found the way a zip reader finds its own directory: from the end. The
// record that closes a zip says how big its central directory is and where
// it starts, and anything in front of where those two meet is the prefix.
// Looking instead for the first PK signature would find one inside the PDF
// the first time a compressed stream happened to contain those two bytes.
func PDFPrefix(data []byte) []byte {
	// The closing record is 22 bytes, plus up to 64k of comment after it.
	const minEnd, maxComment = 22, 1 << 16
	from := len(data) - minEnd - maxComment
	if from < 0 {
		from = 0
	}
	for i := len(data) - minEnd; i >= from; i-- {
		if data[i] != 'P' || data[i+1] != 'K' || data[i+2] != 5 || data[i+3] != 6 {
			continue
		}
		size := binary.LittleEndian.Uint32(data[i+12:])
		offset := binary.LittleEndian.Uint32(data[i+16:])
		start := int64(i) - int64(size) - int64(offset)
		if start <= 0 || start > int64(len(data)) {
			return nil // no prefix, or a zip64 whose fields do not say
		}
		return data[:start]
	}
	return nil
}

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
	loadSymbols(files, reg)
	return s, reg, nil
}

// ReadSymbols loads only the symbols from a .gscore, ignoring its music.
//
// This is how one person's drawings reach another's score: point it at any
// .gscore and take the marks out of it, whether the file was made as a pack
// or is simply a piece that happens to use them.
func ReadSymbols(r io.Reader) (*symbol.Registry, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gscore: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("gscore: not a .gscore container: %w", err)
	}

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	reg := symbol.NewRegistry()
	loadSymbols(files, reg)
	if reg.Len() == 0 {
		return nil, errors.New("gscore: that file carries no symbols")
	}
	return reg, nil
}

// loadSymbols reads the symbol section of a container into a registry.
func loadSymbols(files map[string]*zip.File, reg *symbol.Registry) {
	f, ok := files["symbols/index.json"]
	if !ok {
		return
	}
	b, err := readAll(f)
	if err != nil {
		return
	}
	var index []symbolEntry
	if json.Unmarshal(b, &index) != nil {
		return
	}
	for _, e := range index {
		sf, ok := files[e.File]
		if !ok {
			continue
		}
		raw, err := readAll(sf)
		if err != nil {
			continue
		}
		if e.Kind == "vector" {
			_ = reg.Add(&symbol.Symbol{
				Name: e.Name, Kind: symbol.FromVector,
				Path: string(raw), Advance: e.Advance,
			})
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

func readAll(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
