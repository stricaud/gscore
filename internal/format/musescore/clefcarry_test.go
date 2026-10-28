/*
 * clefcarry_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A change of clef written at the end of a bar.
 *
 * MuseScore writes a clef at the tick where it takes effect, and a change
 * that takes effect at a barline is written after the last note of the bar
 * before — which is also where it is printed, as a warning to the player.
 * There is no note left in that bar to hang it on, so it was dropped.
 *
 * The Moonlight sonata's left hand went up to the treble in bar 189 of the
 * Presto and came back at the end of the bar after. The coming back was
 * lost, and ninety bars were drawn two clefs low: the player was reading
 * five and six ledger lines under the staff for music that sits inside it.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// clefsIn lists the clef in force at the first note of every bar.
func clefsIn(t *testing.T, s *model.Score, part int) []model.Clef {
	t.Helper()
	p := s.Parts[part]
	cur := p.Clef
	var out []model.Clef
	for _, m := range p.Measures {
		first := true
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if mk := marksOf(el); mk != nil && mk.Clef != nil {
					cur = *mk.Clef
				}
				if first {
					out = append(out, cur)
					first = false
				}
			}
		}
	}
	return out
}

const gClef = `<Clef><concertClefType>G</concertClefType>` +
	`<transposingClefType>G</transposingClefType></Clef>`

func TestAClefAtTheEndOfABarTakesEffectInTheNext(t *testing.T) {
	// The clef is in the second bar, because the first one's is the clef
	// the staff begins in rather than a change of it.
	// The staff begins in the bass, and goes up to the treble at the end
	// of the second bar. The clef in the first bar of a staff is the clef
	// it begins in rather than a change of it, which is why the change is
	// written later than that.
	s := read(t, oldScore(
		`<Measure>`+aQuarter(48)+`</Measure>`+
			`<Measure>`+aQuarter(50)+gClef+`</Measure>`+
			`<Measure>`+aQuarter(72)+aQuarter(74)+`</Measure>`))

	got := clefsIn(t, s, 0)
	if len(got) != 3 {
		t.Fatalf("%d bars, want 3", len(got))
	}
	if got[1] != model.Bass {
		t.Errorf("the second bar is in %v, want the clef the staff starts in", got[1])
	}
	if got[2] != model.Treble {
		t.Errorf("the third bar is in %v, want the treble it was given", got[2])
	}
}

// Only the bar straight after it, and only once: a clef picked up again and
// again would be printed in front of every bar to the end of the piece.
func TestACarriedClefIsOnlyCarriedOnce(t *testing.T) {
	s := read(t, oldScore(
		`<Measure>`+aQuarter(72)+`</Measure>`+
			`<Measure>`+aQuarter(74)+gClef+`</Measure>`+
			`<Measure>`+aQuarter(48)+`</Measure>`+
			`<Measure>`+aQuarter(50)+`</Measure>`))

	marks := 0
	for _, m := range s.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if mk := marksOf(el); mk != nil && mk.Clef != nil {
					marks++
				}
			}
		}
	}
	if marks != 1 {
		t.Errorf("the change is written %d times, want once", marks)
	}
}

// A clef with a note still to come in its own bar belongs to that note, and
// nothing is carried anywhere.
func TestAClefInTheMiddleOfABarStaysThere(t *testing.T) {
	s := read(t, oldScore(
		`<Measure>`+aQuarter(72)+`</Measure>`+
			`<Measure>`+aQuarter(73)+gClef+aQuarter(74)+`</Measure>`+
			`<Measure>`+aQuarter(76)+`</Measure>`))

	m := s.Parts[0].Measures[1]
	if mk := marksOf(m.Voices[0].Elements[1]); mk == nil || mk.Clef == nil {
		t.Fatal("the note after the clef does not carry it")
	}
	for _, v := range s.Parts[0].Measures[2].Voices {
		for i, el := range v.Elements {
			if mk := marksOf(el); mk != nil && mk.Clef != nil {
				t.Errorf("element %d of the next bar carries a clef as well", i)
			}
		}
	}
}
