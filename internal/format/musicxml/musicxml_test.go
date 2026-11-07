/*
 * musicxml_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// The arranger is a person of their own, out and back.
func TestTheArrangerSurvivesMusicXML(t *testing.T) {
	s := model.NewScore()
	s.Identity.Composer = "J. S. Bach"
	s.Identity.Arranger = "A. Smith"

	var buf bytes.Buffer
	if err := Export(&buf, s); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `type="arranger"`) {
		t.Error("the file does not say who arranged it")
	}
	got, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Identity.Arranger != "A. Smith" {
		t.Errorf("the arranger came back as %q", got.Identity.Arranger)
	}
	if got.Identity.Composer != "J. S. Bach" {
		t.Errorf("the composer came back as %q — the arranger should not be in it",
			got.Identity.Composer)
	}
}
