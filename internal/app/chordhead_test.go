package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// chordAt returns the chord written at the start of a bar.
func chordAt(t *testing.T, h *harness, part, measure int) *model.Chord {
	t.Helper()
	el := h.app.score.Parts[part].Measures[measure].Voices[0].Elements[0]
	ch, ok := el.(*model.Chord)
	if !ok {
		t.Fatalf("got a %T, want a chord", el)
	}
	return ch
}

// A chord of three noteheads, bottom first: E4, G4, B4 in the treble.
func triad(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.note(0, 0, 0, "quarter")
	h.post("/api/addpitch?part=0&measure=0&index=0&step=2")
	h.post("/api/addpitch?part=0&measure=0&index=0&step=4")
	return h
}

// An accidental on one notehead of a chord.
//
// Reported as: "I want to add the sharp in front of a specific note but I
// cannot" — the whole chord was sharpened, and there was no way to say which
// note had been clicked.
func TestAnAccidentalGoesOnTheNoteheadThatWasClicked(t *testing.T) {
	h := triad(t)
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp&head=2")

	ch := chordAt(t, h, 0, 0)
	want := []model.Accidental{model.NoAccidental, model.Sharp, model.NoAccidental}
	for i, n := range ch.Notes {
		if n.Accidental != want[i] {
			t.Errorf("notehead %d is %v, want %v", i, n.Accidental, want[i])
		}
	}
	if ch.Notes[1].Pitch.Alter != 1 {
		t.Errorf("the sharp did not change how the note sounds: alter=%d",
			ch.Notes[1].Pitch.Alter)
	}
}

// Asked for again, it comes off the same notehead and no other.
func TestTheSameNoteheadsAccidentalIsItsOwnUndo(t *testing.T) {
	h := triad(t)
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp&head=2")
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp&head=2")

	for i, n := range chordAt(t, h, 0, 0).Notes {
		if n.Accidental != model.NoAccidental {
			t.Errorf("notehead %d is %v, want it back as it was", i, n.Accidental)
		}
	}
}

// Sharpening by steps reaches one notehead too, which is what the + key does.
func TestSharpeningByStepsReachesOneNotehead(t *testing.T) {
	h := triad(t)
	h.post("/api/accidental?part=0&measure=0&at=1&cycle=up&head=4")
	h.post("/api/accidental?part=0&measure=0&at=1&cycle=up&head=4")

	ch := chordAt(t, h, 0, 0)
	if ch.Notes[2].Accidental != model.DoubleSharp {
		t.Errorf("the top notehead is %v, want a double sharp", ch.Notes[2].Accidental)
	}
	for _, i := range []int{0, 1} {
		if ch.Notes[i].Accidental != model.NoAccidental {
			t.Errorf("notehead %d is %v, want it left alone", i, ch.Notes[i].Accidental)
		}
	}
}

// Without a notehead named, the accidental still belongs to the whole chord.
//
// This is what a dragged selection asks for, and what the palette does with
// several chords picked up: one sharp across all of them.
func TestAnAccidentalWithNoNoteheadNamedTakesTheWholeChord(t *testing.T) {
	h := triad(t)
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp")

	for i, n := range chordAt(t, h, 0, 0).Notes {
		if n.Accidental != model.Sharp {
			t.Errorf("notehead %d is %v, want a sharp", i, n.Accidental)
		}
	}
}

// A range of chords is whole chords, whatever notehead the pointer was over
// when the drag ended: the step that names a note in one chord names a
// different note in the next.
func TestADraggedSelectionIgnoresTheNotehead(t *testing.T) {
	h := triad(t)
	h.note(0, 0, 1, "quarter")
	h.post("/api/addpitch?part=0&measure=0&index=1&step=3")
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp&sel=0:0:0,0:0:1&head=2")

	for i, el := range h.app.score.Parts[0].Measures[0].Voices[0].Elements {
		ch, ok := el.(*model.Chord)
		if !ok {
			t.Fatalf("element %d is a %T, want a chord", i, el)
		}
		for j, n := range ch.Notes {
			if n.Accidental != model.Sharp {
				t.Errorf("chord %d notehead %d is %v, want a sharp", i, j, n.Accidental)
			}
		}
	}
}

// A notehead that is not there is said so, rather than quietly taking the
// whole chord: a stale selection should not sharpen three notes when one was
// asked for.
func TestNamingANoteheadThatIsNotThereIsAnError(t *testing.T) {
	h := triad(t)
	if msg := h.postFails("/api/accidental?part=0&measure=0&at=1&accidental=sharp&head=9"); msg == "" {
		t.Error("a notehead that is not in the chord was accepted")
	}
	for i, n := range chordAt(t, h, 0, 0).Notes {
		if n.Accidental != model.NoAccidental {
			t.Errorf("notehead %d is %v, want nothing written", i, n.Accidental)
		}
	}
}

// The dot belongs to the chord, not to a notehead of it.
//
// A dot is part of how long the note is, and the notes of a chord are all
// the same length — there is no such thing as dotting one of them.
func TestDottingAChordLengthensEveryNoteInIt(t *testing.T) {
	h := triad(t)
	h.post("/api/dot?part=0&measure=0&at=1")

	ch := chordAt(t, h, 0, 0)
	want := model.Duration{Num: 3, Den: 8}
	if ch.Dur != want {
		t.Errorf("the chord is %s, want %s", ch.Dur, want)
	}
	for i, n := range ch.Notes {
		if n.Dur != want {
			t.Errorf("notehead %d is %s, want %s", i, n.Dur, want)
		}
	}
}

// A tablature staff names no noteheads.
//
// It writes fret numbers where noteheads would go, and their height is which
// string rather than which pitch — so the step read off the page is not the
// step the note sits at on a staff of pitches, and the chord goes whole.
func TestATablatureChordIsTakenWhole(t *testing.T) {
	h := triad(t)
	h.app.score.Parts[0].Tab = &model.Tablature{
		Strings: []model.Pitch{
			{Step: model.StepE, Octave: 4}, {Step: model.StepB, Octave: 3},
			{Step: model.StepG, Octave: 3}, {Step: model.StepD, Octave: 3},
			{Step: model.StepA, Octave: 2}, {Step: model.StepE, Octave: 2},
		},
		Frets: 20,
	}
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp&head=2")

	for i, n := range chordAt(t, h, 0, 0).Notes {
		if n.Accidental != model.Sharp {
			t.Errorf("notehead %d is %v, want a sharp", i, n.Accidental)
		}
	}
}
