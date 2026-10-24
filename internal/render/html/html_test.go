/*
 * html_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package html

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// A score shared as a web page has to be three things at once: a page any
// browser opens, a drawing that needs nothing fetched or installed, and the
// score itself, so that what was sent can be edited rather than only looked
// at.

func tune() *model.Score {
	p := &model.Part{
		Name: "Flute", Clef: model.Treble,
		Key:  model.KeySignature{Fifths: -2},
		Time: model.TimeSignature{Beats: 3, BeatValue: 4},
	}
	for i := 0; i < 6; i++ {
		var els []model.Element
		for k := 0; k < 3; k++ {
			els = append(els, &model.Note{
				Pitch: model.Pitch{Step: model.Step((i + k) % 7), Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 4},
			})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarSingle,
		})
	}
	s := &model.Score{
		Identity: model.Identity{Title: "A Shared Tune", Composer: "Somebody"},
		Tempo:    model.Tempo{BPM: 96, Unit: model.Duration{Num: 1, Den: 4}},
		Parts:    []*model.Part{p},
	}
	s.Normalise()
	return s
}

func page(t *testing.T, s *model.Score, opt Options) []byte {
	t.Helper()
	lay := engrave.Engrave(s, engrave.DefaultOptions())
	var b bytes.Buffer
	if err := Render(&b, lay, s, nil, opt); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestTheSharedPageStandsOnItsOwn(t *testing.T) {
	out := page(t, tune(), DefaultOptions())
	text := string(out)

	if !strings.HasPrefix(text, "<!doctype html>") {
		t.Error("it does not begin as a web page, so a browser will not treat it as one")
	}
	if !strings.Contains(text, "<svg") {
		t.Error("the music was not drawn into it")
	}
	// The font travels with it, or the page is empty staves on any machine
	// that does not happen to have Bravura installed — which is nearly all
	// of them.
	if !strings.Contains(text, "@font-face") || !strings.Contains(text, "base64,") {
		t.Error("the music font is not embedded")
	}
	// Nothing is fetched: no stylesheet, script or image from anywhere.
	// That is what the embedded font buys, and why it is the default.
	for _, fetch := range []string{"src=\"http", "href=\"http", "url(http", "src=\"/"} {
		if strings.Contains(text, fetch) {
			t.Errorf("the page asks the network for something (%s), so it does "+
				"not work from a memory stick or with the network off", fetch)
		}
	}
	if !strings.Contains(text, "<title>A Shared Tune") {
		t.Error("the tab does not say what the score is")
	}
}

// The font can be linked instead of carried, for a page that will live on a
// site that serves it. The page is then a few tens of kilobytes instead of
// more than a megabyte — and it needs the address to answer, which is why
// it is not the default.
func TestTheFontCanBeLinkedInsteadOfCarried(t *testing.T) {
	opt := DefaultOptions()
	opt.FontURL = "https://example.com/Bravura.woff2"
	linked := page(t, tune(), opt)
	carried := page(t, tune(), DefaultOptions())

	if !bytes.Contains(linked, []byte("https://example.com/Bravura.woff2")) {
		t.Error("the address was not used")
	}
	if bytes.Contains(linked, []byte("data:font")) {
		t.Error("the font was written in as well as linked")
	}
	if len(linked) >= len(carried)/4 {
		t.Errorf("linking saved nothing: %d bytes against %d",
			len(linked), len(carried))
	}
}

// With a player, the page carries what it needs to play itself and the
// noteheads carry the numbers that let it light them up.
func TestAPageCanPlayItself(t *testing.T) {
	s := tune()
	lay := engrave.Engrave(s, engrave.DefaultOptions())
	opt := DefaultOptions()
	opt.Sound = &Sound{
		Duration: 4, Tempo: 96,
		Notes: []Note{{Start: 0, Dur: 1, MIDI: 60, Velocity: 80, Family: "flute", Ref: 1}},
		Bars:  []Bar{{Index: 0, Start: 0, Dur: 3}},
		Boxes: Boxes(lay, opt.SVG.StaffSpace),
	}
	var b bytes.Buffer
	if err := Render(&b, lay, s, nil, opt); err != nil {
		t.Fatal(err)
	}
	text := b.String()

	if !strings.Contains(text, `id="gscore-sound"`) {
		t.Error("the page carries nothing to play")
	}
	if !strings.Contains(text, "AudioContext") {
		t.Error("there is no player in it")
	}
	if !strings.Contains(text, "data-ref=") {
		t.Error("the noteheads carry no numbers, so nothing can be lit up")
	}
	// A transport to press, and it is not printed.
	if !strings.Contains(text, `id="transport"`) {
		t.Error("there is no transport on the page")
	}
	if !strings.Contains(text, "@media print") {
		t.Error("the transport would be printed with the music")
	}
}

// Every bar gets a box, so the cursor has somewhere to be in each of them.
func TestEveryBarHasAPlaceOnThePage(t *testing.T) {
	s := tune()
	lay := engrave.Engrave(s, engrave.DefaultOptions())
	boxes := Boxes(lay, 8)

	if len(boxes) != len(s.Parts[0].Measures) {
		t.Fatalf("%d boxes for %d bars", len(boxes), len(s.Parts[0].Measures))
	}
	for i, b := range boxes {
		if b.Index != i {
			t.Errorf("box %d is for bar %d", i, b.Index)
		}
		if b.B <= b.A || b.D <= b.C {
			t.Errorf("bar %d has an empty box: %.1f..%.1f by %.1f..%.1f",
				i, b.A, b.B, b.C, b.D)
		}
		if i > 0 && boxes[i-1].B > b.B {
			t.Errorf("bar %d ends before bar %d does", i, i-1)
		}
	}
}

// The font and the player are two separate choices: either can be written
// into the page or fetched from an address, and one does not decide the
// other.
func TestTheFontAndThePlayerAreSeparateChoices(t *testing.T) {
	s := tune()
	lay := engrave.Engrave(s, engrave.DefaultOptions())
	sound := &Sound{
		Duration: 4, Tempo: 96,
		Notes: []Note{{Start: 0, Dur: 1, MIDI: 60, Family: "flute", Ref: 1}},
		Bars:  []Bar{{Index: 0, Start: 0, Dur: 3}},
		Boxes: Boxes(lay, 8),
	}
	const font = "https://example.com/Bravura.woff2"
	const script = "https://example.com/gscore-player.js"

	for _, c := range []struct {
		what               string
		fontURL, scriptURL string
		carriesFont        bool
		carriesPlayer      bool
	}{
		{"both carried", "", "", true, true},
		{"the font fetched", font, "", false, true},
		{"the player fetched", "", script, true, false},
		{"both fetched", font, script, false, false},
	} {
		opt := DefaultOptions()
		opt.Sound = sound
		opt.FontURL = c.fontURL
		opt.ScriptURL = c.scriptURL

		var b bytes.Buffer
		if err := Render(&b, lay, s, nil, opt); err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		text := b.String()

		if got := strings.Contains(text, "data:font"); got != c.carriesFont {
			t.Errorf("%s: the font is written in: %v", c.what, got)
		}
		if got := !strings.Contains(text, "src=\""+script+"\""); got != c.carriesPlayer {
			t.Errorf("%s: the player is written in: %v", c.what, got)
		}
		if c.fontURL != "" && !strings.Contains(text, font) {
			t.Errorf("%s: the font address was not used", c.what)
		}
		// Either way the page still has something to play and somewhere to
		// press: what changes is where the code comes from.
		if !strings.Contains(text, `id="gscore-sound"`) {
			t.Errorf("%s: the page carries nothing to play", c.what)
		}
		if !strings.Contains(text, `id="transport"`) {
			t.Errorf("%s: there is no transport on the page", c.what)
		}
	}
}

// The player can be had on its own, to serve beside a page that fetches it.
// An option to link to a file nobody can obtain is no option at all.
func TestThePlayerCanBeHadOnItsOwn(t *testing.T) {
	js := PlayerJS()
	if !strings.Contains(js, "AudioContext") || !strings.Contains(js, "gscore-sound") {
		t.Error("that is not the player")
	}
}
