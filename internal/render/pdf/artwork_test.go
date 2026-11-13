/*
 * artwork_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * The cover goes in front and the stamp at the back, once per document.
 *
 * Once, and not once per piece: a score and its parts are one PDF of many
 * sections, and a cover on each of them is a cover between every part.
 */

func picture(w, h int) []byte {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	im.Set(0, 0, color.RGBA{0, 0, 0, 255})
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}

func oneBar() *engrave.Layout {
	s := model.NewScore()
	q := model.Duration{Num: 1, Den: 4}
	v := s.Parts[0].Measures[0].Voices[0]
	for i := 0; i < 4; i++ {
		v.Elements = append(v.Elements, &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4}, Dur: q,
		})
	}
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	return engrave.Engrave(s, eo)
}

// pages counts the pages of a PDF by its page objects.
func pages(b []byte) int {
	return bytes.Count(b, []byte("/Type /Page\n")) +
		bytes.Count(b, []byte("/Type/Page\n")) +
		bytes.Count(b, []byte("/Type /Page/")) +
		bytes.Count(b, []byte("/Type /Page "))
}

func TestACoverAddsAPageInFront(t *testing.T) {
	lay := oneBar()
	opt := DefaultOptions()

	var plain bytes.Buffer
	if err := Render(&plain, lay, opt); err != nil {
		t.Fatal(err)
	}
	was := pages(plain.Bytes())
	if was == 0 {
		t.Fatal("the plain score has no pages to count")
	}

	opt.Cover = &model.Artwork{Data: picture(60, 80), Format: "png"}
	var withCover bytes.Buffer
	if err := Render(&withCover, lay, opt); err != nil {
		t.Fatal(err)
	}
	if got := pages(withCover.Bytes()); got != was+1 {
		t.Errorf("%d pages with a cover, %d without; want one more", got, was)
	}
}

func TestAStampAddsNoPage(t *testing.T) {
	// It goes in the bottom margin of a page that is there already. A
	// stamp that lengthened the document would be a stamp nobody wants.
	lay := oneBar()
	opt := DefaultOptions()

	var plain bytes.Buffer
	if err := Render(&plain, lay, opt); err != nil {
		t.Fatal(err)
	}
	was := pages(plain.Bytes())

	opt.Stamp = &model.Artwork{Data: picture(200, 40), Format: "png", Align: "right"}
	var stamped bytes.Buffer
	if err := Render(&stamped, lay, opt); err != nil {
		t.Fatal(err)
	}
	if got := pages(stamped.Bytes()); got != was {
		t.Errorf("%d pages with a stamp, %d without", got, was)
	}
	// And it is in there: the file grows by the picture.
	if stamped.Len() <= plain.Len() {
		t.Error("the stamp was not drawn at all")
	}
}

func TestAnUnreadablePictureLeavesTheScoreAlone(t *testing.T) {
	// A score is worth printing even where its cover is a broken file: the
	// music is what the document is for.
	lay := oneBar()
	opt := DefaultOptions()
	opt.Cover = &model.Artwork{Data: []byte("not a picture"), Format: "png"}
	var buf bytes.Buffer
	if err := Render(&buf, lay, opt); err != nil {
		t.Fatalf("a broken cover failed the whole score: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("nothing was written")
	}
}

func TestOneCoverForAWholeDocument(t *testing.T) {
	lay := oneBar()
	opt := DefaultOptions()
	opt.Cover = &model.Artwork{Data: picture(60, 80), Format: "png"}

	var one bytes.Buffer
	if err := RenderSections(&one, []Section{{Layout: lay, Opt: opt}}); err != nil {
		t.Fatal(err)
	}
	var three bytes.Buffer
	secs := []Section{{lay, opt}, {lay, opt}, {lay, opt}}
	if err := RenderSections(&three, secs); err != nil {
		t.Fatal(err)
	}
	// Three sections, and still one cover: the pages grow by the music,
	// not by the covers.
	body := pages(one.Bytes()) - 1
	if got, want := pages(three.Bytes()), body*3+1; got != want {
		t.Errorf("%d pages for three sections, want %d — one cover, not three",
			got, want)
	}
}
