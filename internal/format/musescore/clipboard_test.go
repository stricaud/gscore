/*
 * clipboard_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Music copied in MuseScore is read off its clipboard: the staves of the
 * range, each voice a run of elements with where each begins.
 */

package musescore

import (
	"errors"
	"os"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// pastedFixture is three bars of a piano and voice score copied in
// MuseScore 4.7 — three staves, the first holding a tied note under a
// fermata and then rests, some of them hidden, and a double barline.
func pastedFixture(t *testing.T) *Pasted {
	t.Helper()
	data, err := os.ReadFile("testdata/clipboard-stafflist.xml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ReadPasted(data)
	var partial *PartialError
	if err != nil && !errors.As(err, &partial) {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("no music read")
	}
	return p
}

func whole(num, den int) model.Duration { return model.Duration{Num: num, Den: den}.Reduced() }

func TestACopiedRangeIsReadStaffByStaff(t *testing.T) {
	p := pastedFixture(t)
	if len(p.Staves) != 3 {
		t.Fatalf("%d staves, want 3", len(p.Staves))
	}
	if p.Length != whole(6, 1) {
		t.Errorf("the range is %v long, want six whole notes", p.Length)
	}
	top := p.Staves[0].Voices[0]
	if len(top) != 5 {
		t.Fatalf("the top staff holds %d elements, want 5", len(top))
	}
	// Where each begins, counted from the start of the range: the file
	// says where the range was cut from and the elements follow on.
	want := []model.Duration{whole(0, 1), whole(3, 2), whole(3, 1), whole(9, 2), whole(11, 2)}
	for i, el := range top {
		if el.At != want[i] {
			t.Errorf("element %d begins at %v, want %v", i, el.At, want[i])
		}
	}
	first, ok := top[0].Element.(*model.Note)
	if !ok || first.Pitch.String() != "F#4" || first.Dur != whole(3, 2) {
		t.Fatalf("the first element is %#v, want a dotted whole F sharp", top[0].Element)
	}
	second, ok := top[1].Element.(*model.Note)
	if !ok {
		t.Fatalf("the second element is %T", top[1].Element)
	}
	// Tied to the next, which is found; tied from before the range, which
	// is not there to be tied from.
	if !first.TieStart || !second.TieStop || first.TieStop {
		t.Errorf("ties: first starts %v stops %v, second stops %v", first.TieStart, first.TieStop, second.TieStop)
	}
	if !second.Marks.HasArticulation(model.Fermata) {
		t.Error("the held note lost its fermata")
	}
	if r, ok := top[2].Element.(*model.Rest); !ok || !r.Hidden || r.Dur != whole(3, 2) {
		t.Errorf("the third element is %#v, want a hidden dotted whole rest", top[2].Element)
	}
	if r, ok := top[3].Element.(*model.Rest); !ok || r.Hidden || r.Dur != whole(1, 1) {
		t.Errorf("the fourth element is %#v, want a whole rest", top[3].Element)
	}
	for v := 1; v < museVoices; v++ {
		if p.Staves[0].Voices[v] != nil {
			t.Errorf("voice %d of the top staff holds music nobody copied", v+1)
		}
	}
}

func TestTheMarksAmongTheChordsAreKept(t *testing.T) {
	p := pastedFixture(t)
	rh := p.Staves[1].Voices[0]
	if len(rh) < 3 {
		t.Fatalf("the right hand holds %d elements", len(rh))
	}
	if r, ok := rh[0].Element.(*model.Rest); !ok || r.Dur != whole(3, 8) {
		t.Errorf("the right hand opens with %#v, want a dotted quarter rest", rh[0].Element)
	}
	c, ok := rh[1].Element.(*model.Chord)
	if !ok || len(c.Notes) != 4 {
		t.Fatalf("the first chord is %#v, want four notes", rh[1].Element)
	}
	if c.Marks.Dynamic != model.FF {
		t.Errorf("the chord's dynamic is %v, want ff", c.Marks.Dynamic)
	}
	if !c.Marks.HasArticulation(model.Accent) {
		t.Error("the chord lost its accent")
	}
	if rh[1].At != whole(3, 8) {
		t.Errorf("the chord begins at %v, want after the rest", rh[1].At)
	}
	// The left hand runs under it, from the same start.
	lh := p.Staves[2].Voices[0]
	if len(lh) == 0 || lh[0].At != whole(0, 1) {
		t.Fatalf("the left hand begins at %v", lh)
	}
}

// A second voice is a jump back: MuseScore writes it after the first with a
// location that moves the cursor to the next voice and back in time.
func TestASecondVoiceLandsInItsOwnLine(t *testing.T) {
	const frag = `<?xml version="1.0" encoding="UTF-8"?>
<StaffList version="4.70" tick="4/1" len="1/1" staff="0" staves="1">
  <Staff id="0">
    <voiceOffset><voice id="0">0</voice><voice id="1">0</voice></voiceOffset>
    <location><fractions>4/1</fractions></location>
    <Chord><durationType>half</durationType><Note><pitch>72</pitch><tpc>14</tpc></Note></Chord>
    <Chord><durationType>half</durationType><Note><pitch>74</pitch><tpc>16</tpc></Note></Chord>
    <location><voices>1</voices><fractions>-1/1</fractions></location>
    <Chord><durationType>quarter</durationType><Note><pitch>64</pitch><tpc>18</tpc></Note></Chord>
    <Rest><durationType>quarter</durationType></Rest>
    <Chord><durationType>half</durationType><Note><pitch>65</pitch><tpc>13</tpc></Note></Chord>
  </Staff>
</StaffList>`
	p, err := ReadPasted([]byte(frag))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Staves) != 1 {
		t.Fatalf("%d staves", len(p.Staves))
	}
	one, two := p.Staves[0].Voices[0], p.Staves[0].Voices[1]
	if len(one) != 2 || len(two) != 3 {
		t.Fatalf("%d and %d elements in the two voices, want 2 and 3", len(one), len(two))
	}
	if two[0].At != whole(0, 1) || two[2].At != whole(1, 2) {
		t.Errorf("the second voice begins at %v and its half at %v", two[0].At, two[2].At)
	}
	if n, ok := two[0].Element.(*model.Note); !ok || n.Pitch.String() != "E4" {
		t.Errorf("the second voice opens with %#v", two[0].Element)
	}
}

func TestWhatIsNotMuseScoresIsRefused(t *testing.T) {
	if _, err := ReadPasted([]byte("<html>not music</html>")); err == nil {
		t.Error("a web page was read as music")
	}
	if _, err := ReadPasted([]byte(`<StaffList version="4.70" tick="0/1" len="0/1"></StaffList>`)); err == nil {
		t.Error("an empty range was read as music")
	}
}
