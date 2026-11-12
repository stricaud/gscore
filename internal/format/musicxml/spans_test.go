/*
 * spans_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The marks that were read and never written back.
 *
 * marks_test.go is the same question asked of the importer, and it was
 * asked first: a file could arrive with every note in the right place and
 * nothing written over any of them. The writer had the same hole in the
 * other direction and nothing was counting it. A score opened in gscore and
 * saved as MusicXML came out with no pedalling, no crescendo, no 8va, no
 * trill, no turn and no spread chord, and the file said nothing about
 * losing them — the notes were all there, which is what makes it the kind
 * of loss nobody notices until they open the file somewhere else.
 */

package musicxml

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// marked is one staff of four quarter notes, with the marks a test puts on
// them.
func markedNotes(put func(notes []*model.Note)) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	p := &model.Part{
		Name: "Staff", Clef: model.Treble,
		Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	var notes []*model.Note
	var els []model.Element
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepC, Octave: 5},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		notes = append(notes, n)
		els = append(els, n)
	}
	put(notes)
	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: els}},
		RightBarline: model.BarSingle,
	})
	s.Parts = append(s.Parts, p)
	s.Normalise()
	return s
}

func TestThePedalIsWritten(t *testing.T) {
	s := markedNotes(func(n []*model.Note) {
		n[0].Pedal = &model.PedalMark{Start: true}
		n[2].Pedal = &model.PedalMark{Change: true}
		n[3].Pedal = &model.PedalMark{Stop: true}
	})
	out := exported(t, s)
	for _, want := range []string{
		`<pedal type="start"`, `<pedal type="change"`, `<pedal type="stop"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s is not in the file", want)
		}
	}
	back := reimported(t, out).Parts[0].Measures[0].Voices[0].Elements
	if p := back[0].(*model.Note).Pedal; p == nil || !p.Start {
		t.Errorf("the pedal does not go down on the first note: %+v", p)
	}
	if p := back[3].(*model.Note).Pedal; p == nil || !p.Stop {
		t.Errorf("the pedal does not come up on the last: %+v", p)
	}
}

func TestAHairpinIsWritten(t *testing.T) {
	s := markedNotes(func(n []*model.Note) {
		n[0].Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}
		n[2].Hairpin = &model.Hairpin{Stop: true}
	})
	out := exported(t, s)
	if !strings.Contains(out, `<wedge type="crescendo"`) ||
		!strings.Contains(out, `<wedge type="stop"`) {
		t.Errorf("the wedge is not in the file:\n%s", out)
	}
	back := reimported(t, out).Parts[0].Measures[0].Voices[0].Elements
	if h := back[0].(*model.Note).Hairpin; h == nil || !h.Start ||
		h.Kind != model.Crescendo {
		t.Errorf("the crescendo came back as %+v", h)
	}
	if h := back[2].(*model.Note).Hairpin; h == nil || !h.Stop {
		t.Errorf("the wedge does not close: %+v", h)
	}
}

func TestAnOctaveLineIsWritten(t *testing.T) {
	s := markedNotes(func(n []*model.Note) {
		n[0].Octave = &model.OctaveShift{Amount: 1, Start: true}
		n[3].Octave = &model.OctaveShift{Amount: 1, Stop: true}
	})
	out := exported(t, s)
	// 8va over the staff is type="down": the notes are written an octave
	// lower than they sound.
	if !strings.Contains(out, `<octave-shift type="down" size="8"`) {
		t.Errorf("the octave line is not in the file:\n%s", out)
	}
	back := reimported(t, out).Parts[0].Measures[0].Voices[0].Elements
	if o := back[0].(*model.Note).Octave; o == nil || !o.Start || o.Amount != 1 {
		t.Errorf("the 8va came back as %+v", o)
	}
}

func TestATrillAndATurnAreWritten(t *testing.T) {
	s := markedNotes(func(n []*model.Note) {
		n[0].Trill = &model.Trill{Start: true}
		n[1].Trill = &model.Trill{Stop: true}
		n[2].AddArticulation(model.Turn)
		n[3].AddArticulation(model.Mordent)
	})
	out := exported(t, s)
	for _, want := range []string{
		"<trill-mark>", `<wavy-line type="start"`, `<wavy-line type="stop"`,
		"<turn>", "<mordent>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s is not in the file", want)
		}
	}
	back := reimported(t, out).Parts[0].Measures[0].Voices[0].Elements
	if tr := back[0].(*model.Note).Trill; tr == nil || !tr.Start {
		t.Errorf("the trill came back as %+v", tr)
	}
	if arts := back[2].(*model.Note).Articulations; len(arts) != 1 ||
		arts[0] != model.Turn {
		t.Errorf("the turn came back as %v", arts)
	}
}

func TestASpreadChordIsWritten(t *testing.T) {
	s := markedNotes(func(n []*model.Note) {
		n[0].Arpeggio = &model.Arpeggio{}
	})
	out := exported(t, s)
	if !strings.Contains(out, "<arpeggiate") {
		t.Errorf("the spread is not in the file:\n%s", out)
	}
	back := reimported(t, out).Parts[0].Measures[0].Voices[0].Elements
	if back[0].(*model.Note).Arpeggio == nil {
		t.Error("the spread did not come back")
	}
}

func TestTwoPhrasesCanCloseOnOneNote(t *testing.T) {
	// A long slur over the line and a short one over the figure that ends
	// it. Written as a single curve, the shorter of the two is gone.
	s := markedNotes(func(n []*model.Note) {
		n[0].SlurStart = 1
		n[2].SlurStart = 1
		n[3].SlurStop = 2
	})
	out := exported(t, s)
	if n := strings.Count(out, `<slur type="stop"`); n != 2 {
		t.Errorf("%d curves close on the last note, want two:\n%s", n, out)
	}
	back := reimported(t, out).Parts[0].Measures[0].Voices[0].Elements
	if got := back[3].(*model.Note).SlurStop; got != 2 {
		t.Errorf("%d curves closed on the last note after a round trip", got)
	}
}

func TestADrumStaffKeepsItsLines(t *testing.T) {
	s := markedNotes(func(n []*model.Note) {})
	s.Parts[0].StaffLines = 1
	out := exported(t, s)
	if !strings.Contains(out, "<staff-lines>1</staff-lines>") {
		t.Errorf("the line count is not in the file:\n%s", out)
	}
	if got := reimported(t, out).Parts[0].StaffLines; got != 1 {
		t.Errorf("the staff came back ruled with %d lines", got)
	}
}

func TestAMeterThatIsNotDrawnStaysThatWay(t *testing.T) {
	// In force, and nothing about it on the page: the bars are counted in
	// it. See model.TimeHidden.
	s := markedNotes(func(n []*model.Note) {})
	s.Parts[0].Time = model.TimeSignature{
		Beats: 4, BeatValue: 4, Symbol: model.TimeHidden,
	}
	out := exported(t, s)
	if !strings.Contains(out, `print-object="no"`) {
		t.Errorf("the meter does not say it is not printed:\n%s", out)
	}
	if got := reimported(t, out).Parts[0].Time.Symbol; got != model.TimeHidden {
		t.Errorf("the meter came back as symbol %v", got)
	}
}

func TestAClefInTheMiddleOfABarIsWritten(t *testing.T) {
	// The sign goes in front of the note it changes at. A part that
	// changes clef partway through a bar came out of gscore in one clef
	// from end to end, with nothing saying so.
	s := markedNotes(func(n []*model.Note) {
		c := model.Bass
		n[2].Clef = &c
	})
	out := exported(t, s)
	before := strings.Index(out, "<clef")
	second := strings.Index(out[before+5:], "<clef")
	if second < 0 {
		t.Fatalf("only one clef is in the file:\n%s", out)
	}
	// Two notes before it: the clef stands in front of the third.
	head := out[:before+5+second]
	if n := strings.Count(head, "<note"); n != 2 {
		t.Errorf("the second clef comes after %d notes, want 2", n)
	}
	if !strings.Contains(out[before+5+second:], "<sign>F</sign>") {
		t.Errorf("the second clef is not the bass clef:\n%s", out)
	}
}
