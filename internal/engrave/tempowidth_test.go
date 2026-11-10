/*
 * tempowidth_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The air between the words of a tempo marking and the note after them.
 *
 * The note is put where the words end, and where the words end is a
 * measurement. Measured in the wrong face it is the wrong number: the words
 * are set bold and were measured roman, so "Molto moderato" was reckoned
 * nearly a space narrower than it is drawn and the crotchet of its
 * metronome mark was written over the last letter of it.
 */

package engrave

import (
	"math"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// tempoRoom engraves a score whose tempo is these words at this speed, and
// says how much clear air there is between the end of the words as they are
// drawn and the start of the metronome note.
func tempoRoom(t *testing.T, words string, bpm int) float64 {
	t.Helper()
	score := model.NewScore()
	score.Tempo = model.Tempo{
		Text: words, BPM: bpm, Unit: model.Duration{Num: 1, Den: 4},
	}
	v := score.Parts[0].Measures[0].Voices[0]
	v.Elements = append(v.Elements, &model.Note{
		Pitch: model.Pitch{Step: model.StepG, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	})
	lay := Engrave(score, DefaultOptions())

	end := math.Inf(-1)
	for _, tx := range lay.Texts {
		if tx.S != words {
			continue
		}
		if !tx.Bold {
			t.Fatalf("%q is no longer set bold; this measures the face it is "+
				"set in and wants to know", words)
		}
		end = tx.X + boldWidth(tx.S, tx.Size)
	}
	if math.IsInf(end, -1) {
		t.Fatalf("%q was not written at all", words)
	}
	note := math.Inf(1)
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "metNote") {
			note = math.Min(note, g.X)
		}
	}
	if math.IsInf(note, 1) {
		t.Fatal("the metronome mark was not written at all")
	}
	return note - end
}

func TestTheMetronomeNoteStandsClearOfTheWords(t *testing.T) {
	// The reported case, and a couple either side of it: the fault grew
	// with the length of the words, so a short marking hid it.
	for _, words := range []string{
		"Molto moderato", "Allegro", "Andante con moto", "Grave",
	} {
		got := tempoRoom(t, words, 80)
		if got < tempoTextGap-0.001 {
			t.Errorf("the note after %q stands %.2f spaces off the words, "+
				"want %.2f", words, got, tempoTextGap)
		}
		// And not loose either: the two are one marking and are read as
		// one, so the room between them is a gap and not a paragraph.
		if got > tempoTextGap+0.5 {
			t.Errorf("the note after %q stands %.2f spaces off the words, "+
				"which reads as two markings", words, got)
		}
	}
}

// The words of a marking are measured in the face they are set in. Roman
// widths for bold type is how the note came to be written over them, and it
// is a mistake that reads as right until somebody uses a long marking.
func TestTheWordsAreMeasuredInTheFaceTheyAreSetIn(t *testing.T) {
	const words = "Molto moderato"
	roman := textWidth(words, 2.0)
	bold := boldWidth(words, 2.0)
	if bold <= roman {
		t.Fatalf("the tables say bold %q is %.2f and roman %.2f, so this "+
			"test is measuring nothing", words, bold, roman)
	}
	// Were the words still measured roman, the note would sit inside them
	// by the difference less the gap.
	if got := tempoRoom(t, words, 80); got < bold-roman {
		t.Errorf("the note stands %.2f spaces off, which is less than the "+
			"%.2f the two faces differ by: it is placed by the roman width",
			got, bold-roman)
	}
}

// The glyph is the real metronome note, at the size it is drawn, so the
// measurement above is of the thing the reader sees.
func TestTheMetronomeMarkIsAMusicGlyph(t *testing.T) {
	if _, ok := smufl.Glyph("metNoteQuarterUp"); !ok {
		t.Fatal("the music font has no metronome crotchet")
	}
}
