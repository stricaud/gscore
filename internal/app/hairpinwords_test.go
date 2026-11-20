/*
 * hairpinwords_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Asking for a hairpin as the word.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAHairpinCanBeAskedForAsTheWord(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.bars(1)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 2, "quarter")
	}
	h.post("/api/hairpin?hairpin=cresc.&sel=0:0:0,0:0:3")
	v := h.app.score.Parts[0].Measures[0].Voices[0]
	first, last := marksOfElement(v.Elements[0]), marksOfElement(v.Elements[3])
	if first.Hairpin == nil || !first.Hairpin.Start || !first.Hairpin.Words || first.Hairpin.Kind != model.Crescendo {
		t.Errorf("the first note carries %+v, want a crescendo in words starting there", first.Hairpin)
	}
	if last.Hairpin == nil || !last.Hairpin.Stop || !last.Hairpin.Words {
		t.Errorf("the last note carries %+v, want the words' end", last.Hairpin)
	}

	// The other spelling of the same request, and the diminuendo.
	h.post("/api/hairpin?hairpin=diminuendo&words=1&sel=0:0:0,0:0:3")
	if first.Hairpin == nil || first.Hairpin.Kind != model.Diminuendo || !first.Hairpin.Words {
		t.Errorf("asked for a diminuendo in words, the first note carries %+v", first.Hairpin)
	}
	h.post("/api/hairpin?hairpin=dim.&sel=0:0:0,0:0:3")
	if first.Hairpin == nil || first.Hairpin.Kind != model.Diminuendo || !first.Hairpin.Words {
		t.Errorf("asked for dim., the first note carries %+v", first.Hairpin)
	}

	// And the wedge is still the wedge.
	h.post("/api/hairpin?hairpin=crescendo&sel=0:0:0,0:0:3")
	if first.Hairpin == nil || first.Hairpin.Words {
		t.Errorf("asked for a wedge, the first note carries %+v", first.Hairpin)
	}
}
