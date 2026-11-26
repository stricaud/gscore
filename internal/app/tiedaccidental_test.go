package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// tiedBars writes an F sharp at the end of the first bar and an F at the
// start of the second, the way it is typed: the second bar starts afresh,
// so the F is typed as an F.
func tiedBars(h *harness) (from, to *model.Note) {
	h.t.Helper()
	h.post("/api/insert?part=0&measure=0&at=-1&dur=half&kind=note&pitch=G4")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=half&kind=note&pitch=F%234")
	h.post("/api/insert?part=0&measure=1&at=-1&dur=quarter&kind=note&pitch=F4")
	ms := h.app.score.Parts[0].Measures
	return ms[0].Voices[0].Elements[1].(*model.Note), ms[1].Voices[0].Elements[0].(*model.Note)
}

// A tie holds the note it leaves, sharp and all.
//
// Reported as: an F sharp tied over the barline came out an F on the other
// side, drawn with no sign and so read as the F sharp still sounding.
func TestATieHoldsItsSharpAcrossTheBarline(t *testing.T) {
	h := newHarness(t)
	_, to := tiedBars(h)
	h.post("/api/tie?tie=on&sel=0:0:1")
	if to.Pitch.Alter != 1 {
		t.Errorf("the tied-to note is %s, want the F sharp it holds", to.Pitch)
	}
	if to.Accidental != model.NoAccidental {
		t.Errorf("a held note is not announced again, got %v", to.Accidental)
	}
}

// Sharpening a tied note carries on along the tie.
func TestSharpeningATiedNoteSharpensWhatItHolds(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&dur=half&kind=note&pitch=G4")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=half&kind=note&pitch=F4")
	h.post("/api/insert?part=0&measure=1&at=-1&dur=quarter&kind=note&pitch=F4")
	h.post("/api/tie?tie=on&sel=0:0:1")
	h.post("/api/accidental?accidental=sharp&sel=0:0:1")
	to := h.app.score.Parts[0].Measures[1].Voices[0].Elements[0].(*model.Note)
	if to.Pitch.Alter != 1 {
		t.Errorf("the tied-to note is %s, want it sharpened with the note it holds", to.Pitch)
	}
}

// A sign on a held note can be asked for, by the key or by the palette.
//
// Reported as: pressing + on the F the tie lands on never drew a sharp, and
// there was no way to make it.
func TestASignOnAHeldNoteCanBeAskedFor(t *testing.T) {
	h := newHarness(t)
	_, to := tiedBars(h)
	h.post("/api/tie?tie=on&sel=0:0:1")
	h.post("/api/accidental?accidental=sharp&sel=0:1:0")
	if to.Accidental != model.Sharp || to.Pitch.Alter != 1 {
		t.Errorf("the held note is %s with %v, want a printed sharp",
			to.Pitch, to.Accidental)
	}
	// Asked again, it comes off, as any sign asked for does.
	h.post("/api/accidental?accidental=sharp&sel=0:1:0")
	if to.Accidental != model.NoAccidental || to.Pitch.Alter != 1 {
		t.Errorf("asked twice, the held note is %s with %v, want a bare F sharp",
			to.Pitch, to.Accidental)
	}
	// + moves the sound: a held F sharp goes to F double sharp, printed.
	h.post("/api/accidental?cycle=up&sel=0:1:0")
	if to.Accidental != model.DoubleSharp || to.Pitch.Alter != 2 {
		t.Errorf("+ gave %s with %v, want a printed double sharp", to.Pitch, to.Accidental)
	}
}

// + keeps moving on a note whose sign the rules have hidden.
func TestCyclingAHiddenSignMovesOn(t *testing.T) {
	h := newHarness(t)
	typed(h, "f#", "f")
	// The second F is sharp by the first, with no sign of its own.
	h.post("/api/accidental?cycle=up&sel=0:0:1")
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[1].(*model.Note)
	if n.Pitch.Alter != 2 {
		t.Errorf("+ on an F sharp gave %s, want F double sharp", n.Pitch)
	}
}

// An F sharp tied to an F natural says so on the page.
//
// The curve says the sharp is still sounding, so a note that is not sharp
// under it needs its natural, though the new bar has not had a sharp yet.
func TestATieToAnotherPitchIsSpelled(t *testing.T) {
	h := newHarness(t)
	from, to := tiedBars(h)
	from.TieStart, to.TieStop = true, true // as a file can have it
	h.app.score.SpellAccidentals()
	if to.Accidental != model.Natural {
		t.Errorf("an F under a tie from an F sharp is drawn with %v, want a natural",
			to.Accidental)
	}
}

// A sharp clicked on an F that a tie from an F sharp lands on is printed.
func TestASignClickedOnAMistiedNoteIsPrinted(t *testing.T) {
	h := newHarness(t)
	from, to := tiedBars(h)
	from.TieStart, to.TieStop = true, true // an F sharp tied to an F
	h.post("/api/accidental?accidental=sharp&sel=0:1:0")
	if to.Accidental != model.Sharp || to.Pitch.Alter != 1 {
		t.Errorf("the held note is %s with %v, want a printed sharp",
			to.Pitch, to.Accidental)
	}
}
