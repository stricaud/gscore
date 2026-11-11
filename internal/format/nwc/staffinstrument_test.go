/*
 * staffinstrument_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a staff sounds as, where the file says so.
 *
 * It was guessed from the staff's own name, which works for anybody who
 * names their staves in English and for nobody else. From 2.0 the header
 * carries the instrument as a name of its own, always in English, whatever
 * the staff is called: "Flute" behind "Flauto", "Bassoon" behind "Fagotti",
 * "French Horn" behind "Corni in G".
 *
 * Reading the staff name instead left the William Tell overture as two
 * dozen pianos with a piccolo and a clarinet among them — and every window
 * built on the catalogue showed it that way, because a part with no
 * catalogue name is read everywhere as a piano.
 */

package nwc

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// staffNamed is a 2.5 staff header: the five names and enough behind them
// to pass for a staff.
func staffNamed(names ...string) []byte {
	var b []byte
	for _, n := range names {
		b = append(b, []byte(n)...)
		b = append(b, 0)
	}
	return append(b, make([]byte, 256)...)
}

func readStaff(t *testing.T, names ...string) *model.Part {
	t.Helper()
	w := &walker{b: staffNamed(names...), version: 205}
	p, err := w.staff(205)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return p
}

func TestTheStaffSoundsAsTheFileSaysAndNotAsItIsCalled(t *testing.T) {
	for _, c := range []struct {
		staff, label, abbr, instrument, group string
		want                                  model.Instrument
		sound                                 string
	}{
		{"Flauto", "Flauto", "Fl.", "Flute", "Legni",
			model.Flute, "wind.flutes.flute"},
		{"Fagotti", "Fagotti", "Fag.", "Bassoon", "Legni",
			model.Bassoon, "wind.reed.bassoon"},
		{"Corni in G", "Corni in G", "Cor.", "French Horn", "Ottoni",
			model.FrenchHorn, "brass.french-horn"},
		{"Violoncello", "Violoncello", "Vc.", "Cello", "Archi",
			model.Cello, "strings.cello"},
	} {
		t.Run(c.staff, func(t *testing.T) {
			p := readStaff(t, c.staff, c.label, c.abbr, c.instrument, c.group)
			if p.Instrument != c.want {
				t.Errorf("%q sounds as %v, want %v — the file says %q",
					c.staff, p.Instrument, c.want, c.instrument)
			}
			// And the catalogue's own name for it, which is what the score
			// setup dialog and the per-instrument soundfonts look for.
			if p.Sound != c.sound {
				t.Errorf("%q came out as %q, want %q", c.staff, p.Sound, c.sound)
			}
		})
	}
}

// A word that means nothing here leaves the staff as it was, rather than
// guessing at it: the three percussion staves of the Rossini carry no
// instrument name at all.
func TestAStaffWithNoInstrumentNamedIsNotGuessedAt(t *testing.T) {
	p := readStaff(t, "Triangolo", "Triangolo", "Tri.", "", "Percussioni")
	if p.Instrument != model.AcousticGrandPiano {
		t.Errorf("a staff the file says nothing about came out as %v",
			p.Instrument)
	}
}

// The staff's own name is still the answer where there is no other: a 1.75
// header has two names in it and neither of them is an instrument.
func TestAnEarlyStaffStillGoesByItsName(t *testing.T) {
	w := &walker{b: staffNamed("Trumpet", "Brass"), version: 175}
	p, err := w.staff(175)
	if err != nil {
		t.Fatal(err)
	}
	s := &model.Score{Parts: []*model.Part{p}}
	nameInstruments(s)
	if p.Instrument != model.Trumpet {
		t.Errorf("a 1.75 staff called Trumpet came out as %v", p.Instrument)
	}
}

// Staves nothing could name are listed, so they can be set by hand instead
// of found by ear.
//
// A score of pianos looks exactly like a score somebody wrote for pianos,
// and nothing on the page says the reader gave up. Asked for by name:
// "those import warnings for the ones we could not translate".
func TestTheStavesNothingCouldNameAreListed(t *testing.T) {
	s := &model.Score{Parts: []*model.Part{
		{Name: "Flauto"},                        // named, in Italian
		{Name: "Piatti"},                        // a cymbal player, and no GM program
		{Name: "Gran Cassa"},                    // nor this
		{Name: ""},                              // nothing to translate
		{Name: "Drums", Clef: model.Percussion}, // not a program in the first place
	}}
	unknown := nameInstruments(s)
	want := []string{"Piatti", "Gran Cassa"}
	if len(unknown) != len(want) {
		t.Fatalf("%d staves were listed: %v", len(unknown), unknown)
	}
	for i, w := range want {
		if unknown[i] != w {
			t.Errorf("listed %q, want %q", unknown[i], w)
		}
	}
	// And the one that was named is named, with its catalogue id set so
	// that the score setup dialog shows it as a flute.
	if s.Parts[0].Instrument != model.Flute {
		t.Errorf("Flauto came out %v", s.Parts[0].Instrument)
	}
	if s.Parts[0].Sound != "wind.flutes.flute" {
		t.Errorf("Flauto came out as %q", s.Parts[0].Sound)
	}
}

// The reader's own guesses are still made for what the shared table will
// not answer: a NoteWorthy file is as likely to be a hymn as an overture.
func TestABareVoicePartIsStillAVoice(t *testing.T) {
	for _, name := range []string{"Alto", "Tenor", "Soprano 1"} {
		if _, ok := instrumentFromName(name); !ok {
			t.Errorf("%q was left as a piano", name)
		}
	}
}
