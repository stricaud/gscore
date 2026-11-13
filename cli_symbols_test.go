/*
 * cli_symbols_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package main

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/symbol"
)

/*
 * A score carries its own drawings, and every way out of the program has to
 * read them.
 *
 * A symbol is a mark the music font does not have — a fingering chart, a
 * publisher's device, a sign somebody drew — and the score names it rather
 * than containing it. The name and the drawing travel together in the file;
 * a renderer handed the music alone has the name and no way to draw it, and
 * draws nothing at all.
 *
 * The editor reads them when it opens a file. `gscore -o out.pdf score` did
 * not, so a score with a chart over bar one came out of the command line
 * with a hole where the chart had been — and looked perfectly right on
 * screen, which is the worst way for an export to be wrong.
 */

// scoreWithASymbol writes a .gscore holding one drawing and a mark that uses
// it, and returns the path.
func scoreWithASymbol(t *testing.T) string {
	t.Helper()
	reg := symbol.NewRegistry()
	bm := symbol.NewBitmap(8, 8)
	bm.PixelSize = 0.25
	for i := 0; i < 8; i++ {
		bm.Set(i, i, color.NRGBA{A: 255})
	}
	if err := reg.Add(&symbol.Symbol{
		Name: "device", Kind: symbol.FromPixels, Pixels: bm,
	}); err != nil {
		t.Fatal(err)
	}

	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		&model.Rest{Dur: model.Duration{Num: 1, Den: 1}},
		&model.SymbolElement{Symbol: model.SymbolMark{Name: "device"}},
	}

	path := filepath.Join(t.TempDir(), "with-a-device.gscore")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gscorefmt.Write(f, s, reg); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func TestTheCommandLineReadsTheDrawingsAFileCarries(t *testing.T) {
	path := scoreWithASymbol(t)
	reg := symbolsIn(path)
	if reg == nil {
		t.Fatal("the file's drawings were not read, so -o cannot draw them")
	}
	if _, ok := reg.Lookup("device"); !ok {
		t.Errorf("the registry holds %v, not the drawing in the file",
			reg.Names())
	}
}

func TestAFileWithNoDrawingsAsksForNone(t *testing.T) {
	// Most scores. Nil is what the renderers expect, and anything else
	// here would be a registry to carry about for nothing.
	for _, path := range []string{"", filepath.Join(t.TempDir(), "nothing.abc")} {
		if got := symbolsIn(path); got != nil && len(got.Names()) != 0 {
			t.Errorf("%q gave %v", path, got.Names())
		}
	}
}

func TestExportingToPDFDrawsTheSymbol(t *testing.T) {
	// End to end, by the only test that cannot be satisfied by wiring that
	// looks right: the bytes of the file.
	path := scoreWithASymbol(t)
	scores, err := load(path)
	if err != nil || len(scores) == 0 {
		t.Fatalf("reading it back: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.pdf")
	if err := renderFile(scores[0], symbolsIn(path), out,
		false, 0, false, "", "", "", "", 0, false); err != nil {
		t.Fatal(err)
	}
	withIt, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}

	// The same score rendered with no registry, which is what -o did
	// before. A drawing that reached the paper makes the file bigger; one
	// that was silently skipped does not.
	bare := filepath.Join(t.TempDir(), "bare.pdf")
	if err := renderFile(scores[0], nil, bare,
		false, 0, false, "", "", "", "", 0, false); err != nil {
		t.Fatal(err)
	}
	withoutIt, err := os.Stat(bare)
	if err != nil {
		t.Fatal(err)
	}
	if withIt.Size() <= withoutIt.Size() {
		t.Errorf("the PDF is %d bytes with the drawing and %d without it: "+
			"nothing was drawn", withIt.Size(), withoutIt.Size())
	}
}
