/*
 * textdashes_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The broken line after "accel.", which says it carries on over the bars
 * that follow.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestDashesFollowTheWords(t *testing.T) {
	for _, style := range []model.TextStyle{model.TextExpression, model.TextTempo} {
		s := movements(4)
		r := s.Parts[0].Measures[1].Voices[0].Elements[0].(*model.Rest)
		r.Marks.Texts = []model.TextMark{{Text: "Poco accel.", Style: style, Dashes: 6}}
		lay := Engrave(s, DefaultOptions())
		if len(lay.TextMarks) != 1 {
			t.Fatalf("%v: %d texts recorded, want 1", style, len(lay.TextMarks))
		}
		words := lay.TextMarks[0]
		dashed := 0
		for _, l := range lay.Lines {
			if l.Dash == hairpinDash && l.Y1 == l.Y2 &&
				l.X1 > words.X+words.Width && l.X1 < words.X+words.Width+1 &&
				l.Y1 > words.Y && l.Y1 < words.Y+words.Height {
				dashed++
				if want := hairpinDash * (1.8*6 - 0.8); l.X2-l.X1 < want-1e-9 || l.X2-l.X1 > want+1e-9 {
					t.Errorf("%v: the line is %.2f long, want six dashes, %.2f", style, l.X2-l.X1, want)
				}
			}
		}
		if dashed != 1 {
			t.Errorf("%v: %d broken lines after the words, want 1", style, dashed)
		}
		if words.Dashes != 6 {
			t.Errorf("%v: the editor is told %d dashes, want 6", style, words.Dashes)
		}
	}
}
