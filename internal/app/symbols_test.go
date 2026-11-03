/*
 * symbols_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Drawing a symbol, putting it where you want it, and moving it there.
 *
 * Three things went wrong and all of them were quiet. A name already
 * taken was written over without a word. A symbol put anywhere but onto a
 * note was in the file and never drawn, so it could not be seen, moved or
 * taken away. And a symbol standing on its own could not be moved even
 * once it was visible, because moving one looked only at the marks a note
 * carries.
 */

package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// aSymbol draws a small filled square and saves it under a name.
func aSymbol(t *testing.T, h *harness, name string) map[string]any {
	t.Helper()
	const n = 16
	px := make([]int, n*n)
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			px[y*n+x] = 1
		}
	}
	body, err := json.Marshal(map[string]any{
		"name": name, "w": n, "h": n, "pixels": px,
		"pixelSize": 0.125, "originX": 0, "originY": n / 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h.postBody("/api/symbols", string(body))
}

// aSymbolAgain saves over one that is already there, which has to be
// asked for.
func aSymbolAgain(t *testing.T, h *harness, name string) map[string]any {
	t.Helper()
	const n = 16
	px := make([]int, n*n)
	px[0] = 1
	body, _ := json.Marshal(map[string]any{
		"name": name, "w": n, "h": n, "pixels": px,
		"pixelSize": 0.125, "originX": 0, "originY": n / 2,
	})
	return h.postBody("/api/symbols?replace=1", string(body))
}

func aSymbolFails(t *testing.T, h *harness, name, query string) string {
	t.Helper()
	const n = 16
	px := make([]int, n*n)
	px[0] = 1
	body, _ := json.Marshal(map[string]any{
		"name": name, "w": n, "h": n, "pixels": px,
		"pixelSize": 0.125, "originX": 0, "originY": n / 2,
	})
	return h.postBodyFails("/api/symbols"+query, string(body))
}

func TestASymbolIsSaved(t *testing.T) {
	h := newHarness(t)
	if got := aSymbol(t, h, "blob")["name"]; got != "blob" {
		t.Errorf("it saved as %v", got)
	}
}

// A name already taken is refused rather than written over: the symbol
// that would go is one somebody drew earlier and may already have used.
func TestANameAlreadyTakenIsRefused(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")

	got := aSymbolFails(t, h, "blob", "")
	if !strings.Contains(got, "already a symbol called blob") {
		t.Errorf("saving over it said %q", got)
	}
	// And goes through when it is said plainly.
	if res := aSymbolAgain(t, h, "blob"); res["name"] != "blob" {
		t.Errorf("replace=1 gave %v", res)
	}
}

func TestASymbolWithNoNameIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := aSymbolFails(t, h, "", ""); !strings.Contains(got, "name") {
		t.Errorf("it said %q", got)
	}
}

/*
 * Putting one in.
 */

// Onto the note before the cursor, so the mark travels with its note.
func TestASymbolCanGoOntoANote(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")
	// A new score opens with a bar of rest in it, and a quarter note put
	// into a bar that is already full lands in the next one.
	h.post("/api/clearmeasure?part=0&measure=0")
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter")
	got := h.post("/api/insertsymbol?name=blob&part=0&measure=0&at=1&attach=1")
	if got["attached"] != "note" {
		t.Errorf("it attached to %v", got["attached"])
	}
}

// And on its own where there is no note to carry it, which is the case
// that was in the file and never drawn.
func TestASymbolCanStandOnItsOwn(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")
	h.post("/api/insertsymbol?name=blob&part=0&measure=0&at=0&attach=1")

	v := h.app.score.Parts[0].Measures[0].Voices[0]
	found := false
	for _, el := range v.Elements {
		if s, ok := el.(*model.SymbolElement); ok && s.Symbol.Name == "blob" {
			found = true
		}
	}
	if !found {
		t.Fatal("nothing was put into the bar")
	}
}

/*
 * Moving one.
 */

func TestASymbolOnANoteCanBeMoved(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")
	h.post("/api/clearmeasure?part=0&measure=0")
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter")
	h.post("/api/insertsymbol?name=blob&part=0&measure=0&at=1&attach=1")
	h.post("/api/movesymbol?part=0&measure=0&at=1&name=blob&dx=1.5&dy=-2")

	v := h.app.score.Parts[0].Measures[0].Voices[0]
	n, ok := v.Elements[0].(*model.Note)
	if !ok || len(n.Symbols) == 0 {
		t.Fatal("the note carries no symbol")
	}
	if n.Symbols[0].OffsetX != 1.5 || n.Symbols[0].OffsetY != -2 {
		t.Errorf("it moved to %+v", n.Symbols[0])
	}
}

// The one that could not be moved at all: a symbol standing on its own
// carries no Marks, so looking only at a note's marks never found it.
func TestASymbolOnItsOwnCanBeMoved(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")
	h.post("/api/insertsymbol?name=blob&part=0&measure=0&at=0&attach=1")

	v := h.app.score.Parts[0].Measures[0].Voices[0]
	at := -1
	for i, el := range v.Elements {
		if _, ok := el.(*model.SymbolElement); ok {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("the symbol is not in the bar")
	}

	h.post("/api/movesymbol?part=0&measure=0&at=" + itoa(at+1) + "&name=blob&dx=0.5&dy=-1.25")
	el := v.Elements[at].(*model.SymbolElement)
	if el.Symbol.OffsetX != 0.5 || el.Symbol.OffsetY != -1.25 {
		t.Errorf("it moved to %+v", el.Symbol)
	}

	// And again, so that a run of keyboard nudges adds up rather than
	// each one starting from where the engraver put it.
	h.post("/api/movesymbol?part=0&measure=0&at=" + itoa(at+1) + "&name=blob&dx=0.25&dy=0")
	if got := el.Symbol.OffsetX; got != 0.75 {
		t.Errorf("after a second nudge it is at %.2f, want 0.75", got)
	}
}

/*
 * Taking one off again.
 *
 * The one in the screenshot: a symbol visibly picked up, and delete
 * saying there was nothing selected. Nothing removed a symbol at all.
 */

func TestASymbolOnItsOwnCanBeRemoved(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")
	h.post("/api/insertsymbol?name=blob&part=0&measure=0&at=0&attach=1")

	v := h.app.score.Parts[0].Measures[0].Voices[0]
	at := -1
	for i, el := range v.Elements {
		if _, ok := el.(*model.SymbolElement); ok {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("the symbol is not in the bar")
	}
	before := len(v.Elements)

	h.post("/api/removesymbol?part=0&measure=0&at=" + itoa(at+1) + "&name=blob")

	v = h.app.score.Parts[0].Measures[0].Voices[0]
	if len(v.Elements) != before-1 {
		t.Errorf("%d elements left, want %d", len(v.Elements), before-1)
	}
	for _, el := range v.Elements {
		if _, ok := el.(*model.SymbolElement); ok {
			t.Error("the symbol is still there")
		}
	}
}

func TestASymbolOnANoteCanBeRemoved(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")
	h.post("/api/clearmeasure?part=0&measure=0")
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter")
	h.post("/api/insertsymbol?name=blob&part=0&measure=0&at=1&attach=1")
	h.post("/api/removesymbol?part=0&measure=0&at=1&name=blob")

	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if len(n.Symbols) != 0 {
		t.Errorf("the note still wears %d symbols", len(n.Symbols))
	}
	// And the note itself stays: removing a mark is not removing what it
	// was on.
	if n.Pitch.MIDI() == 0 {
		t.Error("the note went with it")
	}
}

func TestRemovingASymbolThatIsNotThereSaysSo(t *testing.T) {
	h := newHarness(t)
	aSymbol(t, h, "blob")
	h.post("/api/clearmeasure?part=0&measure=0")
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter")

	if got := h.postFails("/api/removesymbol?part=0&measure=0&at=1&name=blob"); got == "" {
		t.Error("removing nothing was allowed")
	}
}

// A picture handed up from the page comes back as the grid the drawing
// window works in. The native dialog answers with the same shape, read
// off the disk instead — see handlePickFile's picture mode.
func TestAPictureComesBackAsAGrid(t *testing.T) {
	h := newHarness(t)

	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for x := 0; x < 3; x++ {
		for y := 0; y < 2; y++ {
			img.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	img.Set(0, 0, color.NRGBA{A: 255}) // one black pixel
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	got := h.postBody("/api/symbolimport", buf.String())
	if got["w"].(float64) != 3 || got["h"].(float64) != 2 {
		t.Errorf("the grid is %vx%v", got["w"], got["h"])
	}
	px, ok := got["pixels"].([]any)
	if !ok || len(px) != 6 {
		t.Fatalf("the pixels came back as %T %v", got["pixels"], got["pixels"])
	}
	ink := 0
	for _, v := range px {
		if v.(float64) != 0 {
			ink++
		}
	}
	if ink != 1 {
		t.Errorf("%d pixels are ink, want the one black one", ink)
	}
}

func TestNonsenseIsNotAPicture(t *testing.T) {
	h := newHarness(t)
	if got := h.postBodyFails("/api/symbolimport", "this is not a picture"); got == "" {
		t.Error("it was read as a picture")
	}
}
