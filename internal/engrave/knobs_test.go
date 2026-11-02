/*
 * knobs_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The table of measurements a score may set for itself.
 *
 * Thirty-odd entries, each carrying a pair of closures that reach into
 * Options. The compiler checks the field names in them, and nothing checks
 * that a pair agrees with itself: a reader that reads one field and a
 * writer that writes another compiles perfectly and gives a dialog where
 * turning one number moves a different part of the page.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestEveryKnobIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Knobs {
		if k.Name == "" || k.Label == "" || k.Group == "" {
			t.Errorf("%+v has nothing to show in a dialog", k)
			continue
		}
		if seen[k.Name] {
			t.Errorf("two measurements are both called %q", k.Name)
		}
		seen[k.Name] = true

		if k.Min >= k.Max {
			t.Errorf("%s allows nothing: %v..%v", k.Name, k.Min, k.Max)
		}
		// The house style has to be a legal answer. A default outside its
		// own bounds is a dialog that refuses the number already in it.
		if d := k.Default(); d < k.Min || d > k.Max {
			t.Errorf("%s defaults to %v, outside the %v..%v it allows",
				k.Name, d, k.Min, k.Max)
		}
	}
}

// Each knob reads and writes the same measurement, and no other.
func TestEachKnobReadsWhatItWrites(t *testing.T) {
	for i, k := range Knobs {
		o := DefaultOptions()
		// Somewhere inside the bounds that is not where it already is, so
		// a setter that does nothing at all is caught too.
		want := (k.Min + k.Max) / 2
		if math.Abs(want-k.Get(o)) < 1e-9 {
			want = (want + k.Max) / 2
		}
		k.set(&o, want)

		if got := k.Get(o); math.Abs(got-want) > 1e-9 {
			t.Errorf("%s was set to %v and reads back %v: it writes one "+
				"measurement and reads another", k.Name, want, got)
			continue
		}
		// And nothing else moved with it.
		for j, other := range Knobs {
			if i == j {
				continue
			}
			if math.Abs(other.Get(o)-other.Default()) > 1e-9 {
				t.Errorf("setting %s also moved %s, so the two share a field",
					k.Name, other.Name)
			}
		}
	}
}

// A score's settings reach the engraver.
//
// All of them, including the ones outside what is usual. A house style is
// somebody's taste and the program has no business overruling it: a beam
// thicker than anybody would print is a thing they will see on the page the
// moment they set it, and emptying the field puts it back. What used to
// happen was that the number was quietly held to a bound and the page did
// not change, which reads as a window that does not work.
func TestAScoresSettingsAreApplied(t *testing.T) {
	s := model.NewScore()
	s.Engraving = map[string]float64{
		"noteBase":      4.5,
		"beamThickness": 99,
		"notAThing":     3,
	}
	o := withScoreSettings(s, DefaultOptions())

	if o.Spacings.NoteBase != 4.5 {
		t.Errorf("the note width came out as %v, want 4.5", o.Spacings.NoteBase)
	}
	if o.Defaults.BeamThickness != 99 {
		t.Errorf("a beam of 99 came out as %v: somebody's own measurement was "+
			"overruled", o.Defaults.BeamThickness)
	}
	// A name this version does not know is skipped, not refused: a file
	// from a later gscore has to open here.
	if o.Spacings.MinNote != DefaultSpacings().MinNote {
		t.Error("an unknown measurement changed something")
	}
}

// What is held is what cannot be drawn at all.
//
// A negative space is not a small space, it is shapes drawn through each
// other backwards; and a bar some thousands of staff spaces wide is a
// drawing nothing can show, which on a long score is megabytes of shapes
// nobody asked for. A file can be edited by hand, so neither can be allowed
// to arrive from one.
func TestWhatCannotBeDrawnIsHeld(t *testing.T) {
	for _, k := range Knobs {
		lo, hi := k.Limits()
		if lo < 0 {
			t.Errorf("%s may be negative, which is not a small measurement "+
				"but a backwards one", k.Name)
		}
		if got := k.Clamp(-5); got != lo {
			t.Errorf("%s took -5 and came out as %v", k.Name, got)
		}
		if got := k.Clamp(1e9); got != hi {
			t.Errorf("%s took a billion and came out as %v", k.Name, got)
		}
		// And what is usual is inside what is possible, or the window's
		// arrows would step somewhere the number cannot go.
		if k.Min < lo || k.Max > hi {
			t.Errorf("%s says %v to %v is usual, which is outside the %v to %v "+
				"it can be", k.Name, k.Min, k.Max, lo, hi)
		}
	}

	s := model.NewScore()
	s.Engraving = map[string]float64{"margin": -40, "noteBase": 1e9}
	o := withScoreSettings(s, DefaultOptions())
	if o.Spacings.Margin < 0 {
		t.Errorf("the margin came out as %v", o.Spacings.Margin)
	}
	base, _ := KnobNamed("noteBase")
	_, hi := base.Limits()
	if o.Spacings.NoteBase != hi {
		t.Errorf("a note slot of a billion came out as %v", o.Spacings.NoteBase)
	}
}

// Settings reports what a score chose, and leaves out what it did not.
func TestSettingsReportsOnlyWhatWasChosen(t *testing.T) {
	s := model.NewScore()
	if got := Settings(s); len(got) != 0 {
		t.Errorf("a score that chose nothing reports %v", got)
	}

	base, _ := KnobNamed("noteBase")
	s.Engraving = map[string]float64{
		"noteBase": 4.5,
		// The same as the house style, which is not a choice: writing it
		// down would freeze today's default into the file.
		"minNote": DefaultSpacings().MinNote,
	}
	got := Settings(s)
	if len(got) != 1 || got["noteBase"] != 4.5 {
		t.Errorf("reported %v, want the note width alone", got)
	}
	if base.Default() != DefaultSpacings().NoteBase {
		t.Errorf("the knob's default is %v and the table's is %v",
			base.Default(), DefaultSpacings().NoteBase)
	}
}

// Nothing applied means nothing changed, which is what every score that
// has never opened the dialog relies on.
func TestAScoreThatSetsNothingEngravesTheHouseStyle(t *testing.T) {
	s := model.NewScore()
	if got := withScoreSettings(s, DefaultOptions()); got != DefaultOptions() {
		t.Error("a score that set nothing was engraved differently")
	}
	if got := withScoreSettings(nil, DefaultOptions()); got != DefaultOptions() {
		t.Error("no score at all was engraved differently")
	}
}
