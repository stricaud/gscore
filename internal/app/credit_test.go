/*
 * credit_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the window that names a score may say, and what it may not.
 */

package app

import (
	"net/url"
	"testing"
)

func TestTheArrangerIsKept(t *testing.T) {
	h := newHarness(t)
	h.post("/api/header?composer=J.+S.+Bach&arranger=A.+Smith")
	if got := h.app.score.Identity.Arranger; got != "A. Smith" {
		t.Errorf("the arranger is %q", got)
	}
	if got := h.app.score.Identity.Composer; got != "J. S. Bach" {
		t.Errorf("the composer is %q", got)
	}
}

// The formatting arrives with the words it formats.
func TestHowACreditIsSetIsKept(t *testing.T) {
	h := newHarness(t)
	runs := `[{"text":"Music by "},{"text":"J. S. Bach","bold":true,"colour":"#c01020"}]`
	h.post("/api/header?composer=" + url.QueryEscape("Music by J. S. Bach") +
		"&composerRuns=" + url.QueryEscape(runs))

	got := h.app.score.Identity.ComposerRuns
	if len(got) != 2 {
		t.Fatalf("the credit is set in %d pieces, want 2", len(got))
	}
	if got[1].Text != "J. S. Bach" || !got[1].Bold || got[1].Colour != "#c01020" {
		t.Errorf("the second piece is %+v", got[1])
	}
}

// And formatting that spells something else is refused rather than kept,
// because what it would do downstream is put words on the page that the
// score does not say.
func TestFormattingThatSpellsSomethingElseIsRefused(t *testing.T) {
	h := newHarness(t)
	runs := `[{"text":"Someone else","bold":true}]`
	msg := h.postFails("/api/header?composer=" + url.QueryEscape("J. S. Bach") +
		"&composerRuns=" + url.QueryEscape(runs))
	if msg == "" {
		t.Fatal("it was taken")
	}
	if h.app.score.Identity.ComposerRuns != nil {
		t.Errorf("it was kept anyway: %+v", h.app.score.Identity.ComposerRuns)
	}
}

// A field sent without formatting is a plain field, so taking the
// formatting off in the window takes it off the score.
func TestAFieldSentPlainLosesItsFormatting(t *testing.T) {
	h := newHarness(t)
	runs := `[{"text":"Bach","bold":true}]`
	h.post("/api/header?composer=Bach&composerRuns=" + url.QueryEscape(runs))
	if len(h.app.score.Identity.ComposerRuns) != 1 {
		t.Fatal("it was not set in the first place")
	}
	h.post("/api/header?composer=Bach")
	if got := h.app.score.Identity.ComposerRuns; got != nil {
		t.Errorf("the formatting is still there: %+v", got)
	}
}
