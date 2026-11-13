/*
 * symbolpicture_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/symbol"
)

/*
 * A picture used as the symbol, rather than traced into one.
 *
 * The drawing window reduces whatever it is given to ink and nothing,
 * eight pixels to the staff space. That is right for sketching a sign and
 * wrong for a photograph or a publisher's device: reduced to one bit, the
 * thing has to be repaired square by square, which is not editing.
 */

func colourPNG(w, h int) string {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			im.Set(x, y, color.RGBA{uint8(x * 7 % 255), 90, 200, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.String()
}

func TestAPictureKeepsItsColours(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/symbolpicture?name=device", colourPNG(40, 20))
	sym, ok := h.app.symbols.Lookup("device")
	if !ok {
		t.Fatal("the symbol was not registered")
	}
	if sym.Kind != symbol.FromPixels || sym.Pixels == nil {
		t.Fatalf("it came in as %v", sym.Kind)
	}
	seen := map[color.NRGBA]bool{}
	for _, c := range sym.Pixels.Pix {
		seen[c] = true
	}
	if len(seen) < 3 {
		t.Errorf("%d colours survived, so it was reduced to ink and nothing",
			len(seen))
	}
}

func TestAPictureComesInAtTheHeightAsked(t *testing.T) {
	// Two staff spaces by default: large enough to recognise, small enough
	// not to be in the way while it is put where it goes.
	h := newHarness(t)
	h.postBody("/api/symbolpicture?name=device", colourPNG(40, 20))
	sym, _ := h.app.symbols.Lookup("device")
	px := sym.Pixels
	if got := float64(px.H) * px.PixelSize; got < 1.9 || got > 2.1 {
		t.Errorf("the symbol is %v staff spaces tall, want about 2", got)
	}
}

func TestAPictureIsPinnedByItsMiddle(t *testing.T) {
	// So that making it larger grows it about the place it was put rather
	// than away from it.
	h := newHarness(t)
	h.postBody("/api/symbolpicture?name=device", colourPNG(40, 20))
	px, _ := h.app.symbols.Lookup("device")
	if px.Pixels.OriginX != float64(px.Pixels.W)/2 {
		t.Errorf("pinned across at %v of %d", px.Pixels.OriginX, px.Pixels.W)
	}
	if px.Pixels.OriginY != float64(px.Pixels.H)/2 {
		t.Errorf("pinned down at %v of %d", px.Pixels.OriginY, px.Pixels.H)
	}
}

func TestADrawingStaysADrawing(t *testing.T) {
	// An SVG becomes outlines, which print at the printer's own
	// resolution: for a device of fine rules that is the difference
	// between a mark and a smudge.
	h := newHarness(t)
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` +
		`<path d="M0 0 L10 0 L10 10 Z"/></svg>`
	h.postBody("/api/symbolpicture?name=mark", svg)
	sym, ok := h.app.symbols.Lookup("mark")
	if !ok {
		t.Fatal("the drawing was not registered")
	}
	if sym.Kind != symbol.FromVector || sym.Path == "" {
		t.Errorf("it came in as %v with path %q", sym.Kind, sym.Path)
	}
}

func TestAPictureNeedsAName(t *testing.T) {
	h := newHarness(t)
	msg := h.postBodyFails("/api/symbolpicture", colourPNG(10, 10))
	if !strings.Contains(msg, "name") {
		t.Errorf("refused with %q", msg)
	}
}

func TestWhatIsNotAPictureIsRefusedHereToo(t *testing.T) {
	h := newHarness(t)
	msg := h.postBodyFails("/api/symbolpicture?name=x", "a letter, not a picture")
	if !strings.Contains(msg, "PNG") {
		t.Errorf("refused with %q", msg)
	}
}

func TestABigPictureIsKeptSmallEnoughToSave(t *testing.T) {
	// A symbol is saved into the score as XPM, a palette format: a
	// photograph at its own size would write tens of thousands of colours
	// into the file for a mark a centimetre tall.
	h := newHarness(t)
	h.postBody("/api/symbolpicture?name=big", colourPNG(2000, 1000))
	sym, _ := h.app.symbols.Lookup("big")
	if sym.Pixels.W > 256 || sym.Pixels.H > 256 {
		t.Errorf("the grid came in at %dx%d", sym.Pixels.W, sym.Pixels.H)
	}
	// And the proportions are kept: 2000x1000 is two to one.
	if got := float64(sym.Pixels.W) / float64(sym.Pixels.H); got < 1.9 || got > 2.1 {
		t.Errorf("the grid is %v wide for its height", got)
	}
}

/*
 * A score takes its drawings with it, both ways.
 *
 * Saving has always carried them — "a .gscore holding a score that refers
 * to symbols it does not contain is a score that opens wrong somewhere
 * else" — and opening threw them away. The score kept the names, the
 * registry had nothing under them, and every hand-drawn mark was simply
 * absent: placed, sized, saved, and gone the moment the file was opened
 * again, with nothing on the page or in the log to say why.
 */

func TestASymbolSurvivesSavingAndOpening(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.postBody("/api/symbolpicture?name=device", colourPNG(40, 20))
	h.post("/api/insertsymbol?part=0&measure=0&at=1&name=device&attach=1")
	h.post("/api/symbolscale?part=0&measure=0&at=1&name=device&scale=2.5")

	dir := t.TempDir()
	path := filepath.Join(dir, "with-a-symbol.gscore")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gscorefmt.Write(f, h.app.score, h.app.symbols); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// A fresh editor, as though the program had been closed and started
	// again: the registry knows nothing until the file tells it.
	open := newHarness(t)
	if _, ok := open.app.symbols.Lookup("device"); ok {
		t.Fatal("the fresh registry already had it, so this proves nothing")
	}
	if err := open.app.openPath(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := open.app.symbols.Lookup("device"); !ok {
		t.Fatal("the drawing was left in the file: the mark cannot be drawn")
	}

	// And it reaches the page, at the size it was given.
	lay := engrave.Engrave(open.app.score, engrave.DefaultOptions())
	found := false
	for _, g := range lay.Glyphs {
		if g.Name == "device" {
			found = true
			if g.ScaleOr1() != 2.5 {
				t.Errorf("it came back at %v, want the 2.5 it was set to",
					g.ScaleOr1())
			}
		}
	}
	if !found {
		t.Error("the symbol is not in the drawing")
	}
}

func TestASymbolIsThereWhenTheFileIsNamedOnTheCommandLine(t *testing.T) {
	// The third way in, and the one that was still losing them. A score
	// named on the command line is read before the App exists and handed
	// to it already loaded, so nothing unpacked its drawings: the same
	// file opened from the menu showed its marks and opened from the
	// command line did not.
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.postBody("/api/symbolpicture?name=device", colourPNG(40, 20))
	h.post("/api/insertsymbol?part=0&measure=0&at=1&name=device&attach=1")

	path := filepath.Join(t.TempDir(), "named.gscore")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gscorefmt.Write(f, h.app.score, h.app.symbols); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// What main.go does: read the score, then build the editor around it.
	imp, err := format.ImporterFor(path)
	if err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scores, _ := format.ImportAll(imp, in)
	in.Close()
	ap := NewBook(scores, path)

	if _, ok := ap.symbols.Lookup("device"); !ok {
		t.Fatal("the drawing was left in the file")
	}
	lay := engrave.Engrave(ap.score, engrave.DefaultOptions())
	for _, g := range lay.Glyphs {
		if g.Name == "device" {
			return
		}
	}
	t.Error("the symbol is not in the drawing")
}
