/*
 * scope_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Whose stylesheet is it.
 *
 * An inline <svg> in an HTML page hands its <style> to the page: the rules
 * go into the document's own stylesheet and apply to everything in it, not
 * just to the drawing they were written for. The key signature picker draws
 * fifteen little staves at eleven units to the space and puts them straight
 * into the page — and every glyph of the score behind them was set to the
 * picker's size, clefs and accidentals and dynamics swollen across the
 * music, and stayed that way after the picker was closed because the
 * picture is still in the page.
 */

package svg

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

func aLittleStaff() *engrave.Layout {
	s := model.NewScore()
	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	return engrave.Engrave(s, opt)
}

func renderWith(t *testing.T, opt Options) string {
	t.Helper()
	var b strings.Builder
	if err := Render(&b, aLittleStaff(), opt); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return b.String()
}

// The rules of a scoped drawing name the drawing, so nothing outside it can
// be reached by them.
func TestAScopedDrawingKeepsItsRulesToItself(t *testing.T) {
	opt := DefaultOptions()
	opt.EmbedFont = false
	opt.Scope = "keysig"
	out := renderWith(t, opt)

	style := out[strings.Index(out, "<style>"):strings.Index(out, "</style>")]
	for _, line := range strings.Split(style, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "<style>" || strings.HasPrefix(line, "@font-face") {
			continue
		}
		if !strings.HasPrefix(line, ".gscore-keysig") {
			t.Errorf("a rule reaches outside the drawing: %s", line)
		}
	}
	if !strings.Contains(out, `class="gscore-score gscore-keysig"`) {
		t.Error("the drawing does not carry the class its rules hang from")
	}
}

// And it wins inside itself over the rules of the page it was put in, which
// are written without a scope.
func TestAScopedRuleIsMoreParticularThanAnUnscopedOne(t *testing.T) {
	opt := DefaultOptions()
	opt.EmbedFont = false
	opt.Scope = "keysig"
	scoped := renderWith(t, opt)

	opt.Scope = ""
	page := renderWith(t, opt)

	if !strings.Contains(page, "\n.g{font-family:") {
		t.Fatal("the page's own drawing no longer styles its glyphs plainly")
	}
	if !strings.Contains(scoped, ".gscore-keysig .g{font-family:") {
		t.Fatal("the scoped drawing does not style its glyphs under its own class")
	}
}

// A drawing that is a file on its own is still a file on its own.
func TestAnUnscopedDrawingIsUnchanged(t *testing.T) {
	opt := DefaultOptions()
	opt.EmbedFont = false
	out := renderWith(t, opt)
	for _, want := range []string{
		`class="gscore-score"`, ".gscore-score{background:#fff;}",
		"\n.g{", "\n.t{", "\nline{", "\npath.curve{",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is not in the drawing any more", want)
		}
	}
}
