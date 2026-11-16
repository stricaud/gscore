/*
 * dyntext_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A dynamic written by hand travels in the file with how loud it plays.
 */

package gscore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAWrittenDynamicSurvivesTheFile(t *testing.T) {
	s := viewScore()
	n := s.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	n.Dynamic, n.DynamicText = model.Forte, "mp-f"

	got := roundTrip(t, s).Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if got.Dynamic != model.Forte || got.DynamicText != "mp-f" {
		t.Errorf("read back as %q playing %s, wanted mp-f playing f",
			got.DynamicText, got.Dynamic)
	}
}
