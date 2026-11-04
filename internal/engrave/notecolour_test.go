/*
 * notecolour_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Colouring the noteheads by what note they are. What matters is that the
// colour says the letter and nothing else: the same for every octave, the
// same through an accidental, and nowhere near anything that is not a note.

// scaleScore is one bar per note of the scale, so every colour is used once.
func scaleScore(octave int) *model.Score {
	p := &model.Part{Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	steps := []model.Step{model.StepC, model.StepD, model.StepE, model.StepF,
		model.StepG, model.StepA, model.StepB}
	var els []model.Element
	for _, s := range steps {
		els = append(els, &model.Note{
			Pitch: model.Pitch{Step: s, Octave: octave},
			Dur:   model.Duration{Num: 1, Den: 4},
		})
	}
	p.Measures = append(p.Measures,
		&model.Measure{Voices: []*model.Voice{{Elements: els[:4]}},
			RightBarline: model.BarSingle},
		&model.Measure{Voices: []*model.Voice{{Elements: els[4:]}},
			RightBarline: model.BarSingle})
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func headColours(lay *Layout) []string {
	var out []string
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			out = append(out, g.Colour)
		}
	}
	return out
}

func colourOptions() Options {
	opt := DefaultOptions()
	opt.ColourNotes = true
	opt.WriteAhead = false
	return opt
}

func TestNotesAreNotColouredUnlessAskedFor(t *testing.T) {
	opt := DefaultOptions()
	opt.WriteAhead = false
	for _, c := range headColours(Engrave(scaleScore(4), opt)) {
		if c != "" {
			t.Fatalf("a notehead came out %q with the option off", c)
		}
	}
}

func TestEachNoteGetsItsOwnColour(t *testing.T) {
	got := headColours(Engrave(scaleScore(4), colourOptions()))
	if len(got) != 7 {
		t.Fatalf("%d noteheads, want 7", len(got))
	}
	seen := map[string]bool{}
	for i, c := range got {
		if c == "" {
			t.Fatalf("note %d was not coloured", i)
		}
		if seen[c] {
			t.Errorf("two notes of the scale share the colour %s", c)
		}
		seen[c] = true
	}
}

// Every C is a C. A colour that changed with the octave would be saying
// something about the octave, which is not what the reader is being told.
func TestTheSameNoteIsTheSameColourInEveryOctave(t *testing.T) {
	low := headColours(Engrave(scaleScore(3), colourOptions()))
	high := headColours(Engrave(scaleScore(5), colourOptions()))
	if len(low) != len(high) {
		t.Fatalf("%d against %d noteheads", len(low), len(high))
	}
	for i := range low {
		if low[i] != high[i] {
			t.Errorf("note %d is %s in one octave and %s in another",
				i, low[i], high[i])
		}
	}
}

// C sharp is a C: it sits on the C line and it is called C. Colouring it as
// something else would break the one rule the colours are there to teach.
func TestAnAccidentalKeepsTheLettersColour(t *testing.T) {
	plain := &model.Note{
		Pitch: model.Pitch{Step: model.StepF, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	sharp := &model.Note{
		Pitch: model.Pitch{Step: model.StepF, Octave: 4, Alter: 1},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	p := &model.Part{Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: []model.Element{plain, sharp}}},
		RightBarline: model.BarSingle,
	})
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}

	got := headColours(Engrave(s, colourOptions()))
	if len(got) != 2 {
		t.Fatalf("%d noteheads, want 2", len(got))
	}
	if got[0] != got[1] {
		t.Errorf("F is %s and F sharp is %s; they are the same letter",
			got[0], got[1])
	}
}

// Only the noteheads. A clef, a rest, a stem and an accidental are not notes
// with letters, and colouring them would be saying something untrue.
func TestNothingButTheNoteheadsIsColoured(t *testing.T) {
	lay := Engrave(scaleScore(4), colourOptions())
	for _, g := range lay.Glyphs {
		if g.Colour != "" && g.Name != "noteheadBlack" {
			t.Errorf("%s was coloured %s", g.Name, g.Colour)
		}
	}
}

// A kit is read by where a head sits, not by what it is called. There is no
// letter there to colour by.
func TestADrumStaffIsNotColoured(t *testing.T) {
	s := scaleScore(4)
	s.Parts[0].Clef = model.Percussion
	for _, c := range headColours(Engrave(s, colourOptions())) {
		if c != "" {
			t.Fatalf("a drum notehead came out %q", c)
		}
	}
}

// A palette given in the options is the one that is used; giving none falls
// back to the built-in colours, so turning the option on is enough on its own.
func TestAGivenPaletteWins(t *testing.T) {
	opt := colourOptions()
	opt.NoteColours = NoteColours{"#111111", "#222222", "#333333",
		"#444444", "#555555", "#666666", "#777777"}
	got := headColours(Engrave(scaleScore(4), opt))
	if len(got) == 0 || got[0] != "#111111" {
		t.Fatalf("C came out %q, want the palette's own colour", got[0])
	}
}

func TestEveryDefaultColourIsOneARendererCanDraw(t *testing.T) {
	for i, c := range DefaultNoteColours {
		if _, _, _, ok := RGB(c); !ok {
			t.Errorf("the default colour for %s, %q, cannot be read",
				model.Step(i), c)
		}
	}
}

func TestRGBReadsTheFormsAColourPickerProduces(t *testing.T) {
	for _, tc := range []struct {
		in         string
		r, g, b    float64
		wantParsed bool
	}{
		{"#ffffff", 1, 1, 1, true},
		{"#000000", 0, 0, 0, true},
		{"ff0000", 1, 0, 0, true},
		{"#f00", 1, 0, 0, true},
		{"  #00ff00  ", 0, 1, 0, true},
		{"", 0, 0, 0, false},
		{"#12345", 0, 0, 0, false},
		{"#gggggg", 0, 0, 0, false},
		{"rebeccapurple", 0, 0, 0, false},
	} {
		r, g, b, ok := RGB(tc.in)
		if ok != tc.wantParsed {
			t.Errorf("RGB(%q) read = %v, want %v", tc.in, ok, tc.wantParsed)
			continue
		}
		if ok && (r != tc.r || g != tc.g || b != tc.b) {
			t.Errorf("RGB(%q) = %v %v %v, want %v %v %v",
				tc.in, r, g, b, tc.r, tc.g, tc.b)
		}
	}
}
