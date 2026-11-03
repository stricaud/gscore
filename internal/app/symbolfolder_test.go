/*
 * symbolfolder_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The folder of symbols that belongs to the person rather than to a score.
 */

package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stricaud/gscore/internal/symbol"
)

// aFolder writes the files given into a new symbols folder and reads it.
func aFolder(t *testing.T, files map[string]string) []folderSymbol {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return readSymbolFolder(dir)
}

const aSquareSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
  <path d="M1 1 L9 1 L9 9 L1 9 Z"/>
</svg>`

func TestAnSVGBecomesAnOutline(t *testing.T) {
	got := aFolder(t, map[string]string{"wedge.svg": aSquareSVG})
	if len(got) != 1 {
		t.Fatalf("%d files read", len(got))
	}
	if got[0].Err != "" {
		t.Fatalf("it would not read: %s", got[0].Err)
	}
	s := got[0].Sym
	if s.Name != "wedge" {
		t.Errorf("it is called %q — the file's name without the extension", s.Name)
	}
	if s.Kind != symbol.FromVector || s.Path == "" {
		t.Errorf("it came out as %v with path %q", s.Kind, s.Path)
	}
}

// Scaled to a size that means something on a staff, and registered so
// that the staff position is its middle.
func TestAnSVGIsSizedForAStaff(t *testing.T) {
	got := aFolder(t, map[string]string{"wedge.svg": aSquareSVG})
	p, err := symbol.ParsePath(got[0].Sym.Path)
	if err != nil {
		t.Fatal(err)
	}
	x0, y0, _, y1 := p.Bounds()
	if h := y1 - y0; h < 1.5 || h > 2.5 {
		t.Errorf("it is %.2f staff spaces tall", h)
	}
	// Centred on the staff position, and starting at it across.
	if y0 > -0.4 || y1 < 0.4 {
		t.Errorf("it runs %.2f..%.2f; the middle should be on the line", y0, y1)
	}
	if x0 < -0.01 {
		t.Errorf("it starts at %.2f, left of the note", x0)
	}
}

func TestAPictureBecomesAGrid(t *testing.T) {
	// A tiny PNG: four black pixels on white.
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			if x < 2 && y < 2 {
				c = color.NRGBA{A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dot.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readSymbolFolder(dir)
	if len(got) != 1 || got[0].Err != "" {
		t.Fatalf("read %+v", got)
	}
	s := got[0].Sym
	if s.Kind != symbol.FromPixels || s.Pixels == nil {
		t.Fatalf("it came out as %v", s.Kind)
	}
	// The black corner is ink and the white is paper.
	if n := s.Pixels.Opaque(); n != 4 {
		t.Errorf("%d pixels are ink, want 4", n)
	}
}

// One file it cannot read is not a reason to have none of them.
func TestABadFileDoesNotStopTheRest(t *testing.T) {
	got := aFolder(t, map[string]string{
		"good.svg": aSquareSVG,
		"bad.svg":  "<svg><path d=\"Q R S\"/></svg>",
	})
	if len(got) != 2 {
		t.Fatalf("%d files read", len(got))
	}
	var good, bad int
	for _, f := range got {
		if f.Err != "" {
			bad++
			continue
		}
		good++
	}
	if good != 1 || bad != 1 {
		t.Errorf("%d read and %d refused", good, bad)
	}
	// And the one that failed says which file it was, since that is what
	// the person has to go and look at.
	for _, f := range got {
		if f.Err != "" && f.File != "bad.svg" {
			t.Errorf("the complaint names %q", f.File)
		}
	}
}

func TestOnlyDrawingsAreRead(t *testing.T) {
	got := aFolder(t, map[string]string{
		"keep.svg":    aSquareSVG,
		"notes.txt":   "nothing to see",
		".hidden.svg": aSquareSVG,
	})
	if len(got) != 1 || got[0].Sym == nil || got[0].Sym.Name != "keep" {
		t.Errorf("read %+v", got)
	}
}

func TestAnEmptyOrMissingFolderIsFine(t *testing.T) {
	if got := readSymbolFolder(t.TempDir()); len(got) != 0 {
		t.Errorf("an empty folder gave %+v", got)
	}
	if got := readSymbolFolder(filepath.Join(t.TempDir(), "nope")); got != nil {
		t.Errorf("a folder that is not there gave %+v", got)
	}
	if got := readSymbolFolder(""); got != nil {
		t.Errorf("no folder at all gave %+v", got)
	}
}

// The rule that matters: opening somebody else's piece must not redraw
// their marks with yours.
func TestAScoresOwnSymbolBeatsTheFolders(t *testing.T) {
	h := newHarness(t)

	// A folder symbol, as startup would have added it.
	folder := &symbol.Symbol{
		Name: "mark", Kind: symbol.FromPixels,
		Pixels: symbol.NewBitmap(4, 4), Source: symbol.FromFolder,
	}
	if err := h.app.symbols.Add(folder); err != nil {
		t.Fatal(err)
	}

	// Then the score's own, read from the file afterwards.
	own := &symbol.Symbol{
		Name: "mark", Kind: symbol.FromPixels,
		Pixels: symbol.NewBitmap(8, 8), Source: symbol.FromScore,
	}
	if err := h.app.symbols.Add(own); err != nil {
		t.Fatal(err)
	}

	got, ok := h.app.symbols.Lookup("mark")
	if !ok {
		t.Fatal("the symbol has gone")
	}
	if got.Source != symbol.FromScore || got.Pixels.W != 8 {
		t.Errorf("the folder's copy won: %+v", got)
	}
}

// The folder is made where the scores are kept, beside the plugins.
func TestTheFolderSitsBesideThePlugins(t *testing.T) {
	home := t.TempDir()
	dir := symbolsDirIn(home)
	if dir != filepath.Join(home, SymbolsFolder) {
		t.Errorf("the folder is %q", dir)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Errorf("it was not made: %v", err)
	}
	// Asked for again, it is the same folder and nothing is disturbed.
	if again := symbolsDirIn(home); again != dir {
		t.Errorf("asked twice it gave %q then %q", dir, again)
	}
	// Something that is not a folder already called that is not gscore's
	// to move.
	taken := filepath.Join(t.TempDir())
	if err := os.WriteFile(filepath.Join(taken, SymbolsFolder), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := symbolsDirIn(taken); got != "" {
		t.Errorf("a file in the way gave %q", got)
	}
	if got := symbolsDirIn(""); got != "" {
		t.Errorf("no home gave %q", got)
	}
}
