/*
 * artwork_test.go
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
	"strings"
	"testing"
)

/*
 * The cover and the stamp.
 *
 * Two pictures printed around the music and neither of them music: the
 * cover is the sheet a publisher wraps a score in, the stamp the block a
 * printer puts at the foot of the last page. They go into the file so that
 * a score copied or emailed takes them with it, and they are printed only
 * in a PDF — never in the editor, where a cover between you and bar one
 * would be in the way.
 */

func fakePNG(w, h int) string {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	im.Set(0, 0, color.RGBA{1, 2, 3, 255})
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.String()
}

func TestACoverIsKeptInTheScore(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/cover", fakePNG(60, 80))
	if h.app.score.Cover == nil {
		t.Fatal("the score has no cover")
	}
	if got := h.app.score.Cover.Format; got != "png" {
		t.Errorf("the cover is a %q", got)
	}
	// The bytes themselves, not a path to them: a score that is emailed
	// takes its cover with it and a path does not.
	if len(h.app.score.Cover.Data) == 0 {
		t.Error("the cover holds no bytes")
	}
}

func TestAStampRemembersWhichEndItSitsAt(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/stamp?align=right", fakePNG(200, 40))
	if got := h.app.score.Stamp.Align; got != "right" {
		t.Errorf("the stamp sits at %q", got)
	}
	// And can be moved without being sent again, which is the whole point
	// of asking separately.
	h.post("/api/stamp?align=left")
	if got := h.app.score.Stamp.Align; got != "left" {
		t.Errorf("after moving it, the stamp sits at %q", got)
	}
	if len(h.app.score.Stamp.Data) == 0 {
		t.Error("moving the stamp lost the picture")
	}
}

func TestWhatIsNotAPictureIsRefused(t *testing.T) {
	// By what is in the file and not by what it was called: a browser
	// sends whatever name the file had, and the PDF writer should never
	// have to wonder whether what it was handed is a picture.
	h := newHarness(t)
	msg := h.postBodyFails("/api/cover", "this is a letter, not a picture")
	if !strings.Contains(msg, "PNG") {
		t.Errorf("refused with %q", msg)
	}
	if h.app.score.Cover != nil {
		t.Error("it was taken in anyway")
	}
}

func TestADrawingIsRecognisedWhateverItWasCalled(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/stamp", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	if got := h.app.score.Stamp.Format; got != "svg" {
		t.Errorf("the drawing came in as %q", got)
	}
}

func TestAPictureCanBeTakenAway(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/cover", fakePNG(10, 10))
	h.post("/api/cover?clear=1")
	if h.app.score.Cover != nil {
		t.Error("the cover is still there")
	}
}

func TestMovingAStampThatIsNotThereSaysSo(t *testing.T) {
	h := newHarness(t)
	if msg := h.fails("/api/stamp?align=left"); !strings.Contains(msg, "no stamp") {
		t.Errorf("refused with %q", msg)
	}
}

func TestTheDialogIsToldWhatIsThereWithoutTheBytes(t *testing.T) {
	// The dialog has to say "there is a cover already"; it does not need
	// eight megabytes to say it.
	h := newHarness(t)
	h.postBody("/api/cover", fakePNG(40, 40))
	info := h.post("/api/info")
	cover, ok := info["cover"].(map[string]any)
	if !ok {
		t.Fatalf("the info does not mention the cover: %v", info["cover"])
	}
	if cover["format"] != "png" {
		t.Errorf("the cover is reported as %v", cover["format"])
	}
	if n, _ := cover["bytes"].(float64); n <= 0 {
		t.Errorf("the cover is reported as %v bytes", cover["bytes"])
	}
	if _, leaked := cover["data"]; leaked {
		t.Error("the picture itself was sent to the dialog")
	}
}
