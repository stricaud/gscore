package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// typed writes a run of named pitches, the way the typing bar does.
func typed(h *harness, names ...string) {
	h.t.Helper()
	for _, n := range names {
		h.post("/api/insert?part=0&measure=0&at=-1&dur=quarter&kind=note&pitch=" +
			strings.ReplaceAll(n, "#", "%23"))
	}
}

// signs is every accidental drawn on the page, left to right, as the letters
// anybody would write them with. The key signature's own are left out: they
// stand at the head of the staff, not on a note.
func signs(t *testing.T, h *harness) string {
	t.Helper()
	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	opt.WriteAhead = false
	name := map[string]string{
		"accidentalSharp": "#", "accidentalFlat": "b", "accidentalNatural": "n",
		"accidentalDoubleSharp": "x", "accidentalDoubleFlat": "bb",
	}
	lay := engrave.Engrave(h.app.score, opt)
	heads := 0.0
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") && (heads == 0 || g.X < heads) {
			heads = g.X
		}
	}
	var out []string
	for _, g := range lay.Glyphs {
		if s, ok := name[g.Name]; ok && g.X > heads-3 {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

func pitches(h *harness) string {
	var out []string
	for _, el := range h.app.score.Parts[0].Measures[0].Voices[0].Elements {
		for _, n := range notesIn(el) {
			out = append(out, n.Pitch.String())
		}
	}
	return strings.Join(out, " ")
}

// A sharp typed in is a sharp on the page.
//
// Reported as: typing "16 b#aba" into the typing bar put a B sharp in the
// file and drew a B. The note sounded right and the page said something
// else, which is the one thing an editor must not do.
func TestATypedSharpIsDrawn(t *testing.T) {
	h := newHarness(t)
	typed(h, "b#", "a", "b", "a")

	if got, want := pitches(h), "B#3 A3 B3 A3"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	// And the natural that cancels it: an accidental holds to the end of
	// its bar, so the third note needs a sign or it is read as B sharp too.
	if got, want := signs(t, h), "# n"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// A sign the signature already gives is not written again.
//
// Otherwise a tune typed out note by note — accidentals spelled on all of
// them, because the key would alter them otherwise — comes back peppered
// with sharps that say only what the signature said.
func TestASignTheKeyAlreadyGivesIsNotWrittenAgain(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setkey?part=0&fifths=1") // G major: every F is sharp
	typed(h, "f#", "g")

	if got, want := pitches(h), "F#4 G4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got := signs(t, h); got != "" {
		t.Errorf("the page shows %q, want nothing: the signature said it", got)
	}
}

// And one that contradicts the signature is.
func TestASignAgainstTheKeyIsWritten(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setkey?part=0&fifths=1")
	typed(h, "fn", "g")

	if got, want := pitches(h), "F4 G4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got, want := signs(t, h), "n"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// An accidental holds for its own octave and not for the others.
func TestAnAccidentalIsCancelledOnlyInItsOwnOctave(t *testing.T) {
	h := newHarness(t)
	typed(h, "f#4", "f5", "f4")

	if got, want := pitches(h), "F#4 F5 F4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	// The F in the octave above was never sharpened, so it needs no sign;
	// the one that comes back to the sharpened octave does.
	if got, want := signs(t, h), "# n"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// A plain letter after an altered note of the same name needs the sign that
// says so, however the note was put there.
func TestAPlainNoteAfterAnAlteredOneIsCancelled(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 1, "quarter") // the F in the bottom space, by hand
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp")
	h.note(0, 0, 1, "quarter")

	if got, want := pitches(h), "F#4 F4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got, want := signs(t, h), "# n"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// Choosing a sign from the palette writes it, whether or not anything else
// already said it: asking for a sharp is asking to see one.
func TestAChosenAccidentalIsAlwaysWritten(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setkey?part=0&fifths=1")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=quarter&kind=note&step=1&accidental=sharp")

	if got, want := signs(t, h), "#"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// A notehead added to a chord is read against the notes already in it.
func TestANoteheadAddedToAChordCarriesItsOwnSign(t *testing.T) {
	h := newHarness(t)
	typed(h, "c4")
	h.post("/api/addpitch?part=0&measure=0&index=0&pitch=eb4")
	h.post("/api/addpitch?part=0&measure=0&index=0&pitch=g4")

	ch, ok := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Chord)
	if !ok {
		t.Fatalf("got a %T, want a chord", h.app.score.Parts[0].Measures[0].Voices[0].Elements[0])
	}
	if len(ch.Notes) != 3 {
		t.Fatalf("%d noteheads, want 3", len(ch.Notes))
	}
	if ch.Notes[1].Accidental != model.Flat {
		t.Errorf("the E flat is written as %v, want a flat", ch.Notes[1].Accidental)
	}
	for _, i := range []int{0, 2} {
		if ch.Notes[i].Accidental != model.NoAccidental {
			t.Errorf("notehead %d carries %v, want no sign",
				i, ch.Notes[i].Accidental)
		}
	}
}

// What a typed name spells, read on its own.
func TestWhatATypedNameSpells(t *testing.T) {
	cases := []struct {
		name string
		want model.Accidental
	}{
		{"b", model.NoAccidental},
		{"b4", model.NoAccidental},
		{"b#", model.Sharp},
		{"bs", model.Sharp},
		{"bb", model.Flat},
		{"bbb", model.DoubleFlat},
		{"bn", model.Natural},
		{"bx", model.DoubleSharp},
		{"b##", model.DoubleSharp},
		{"f#3", model.Sharp},
		{"f###", model.NoAccidental}, // further than notation goes
	}
	for _, c := range cases {
		if got := writtenAccidental(c.name); got != c.want {
			t.Errorf("%q spells %v, want %v", c.name, got, c.want)
		}
	}
}
