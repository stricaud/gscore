package app

import (
	"strconv"
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
		"accidentalParensLeft": "(", "accidentalParensRight": ")",
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

	// And the B after it is a B sharp too, with no sign of its own: an
	// accidental holds to the end of its bar, and somebody writing is
	// reading as they go. Typing a bare letter is not asking for the
	// letter to be put back to what the key signature makes it.
	if got, want := pitches(h), "B#3 A3 B#3 A3"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got, want := signs(t, h), "#"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// Which is said by saying it: a natural is a thing you write, not a thing
// that happens to a letter you did not spell.
func TestANaturalIsWrittenWhenItIsAskedFor(t *testing.T) {
	h := newHarness(t)
	typed(h, "b#", "a", "bn", "a")

	if got, want := pitches(h), "B#3 A3 B3 A3"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
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
func TestAnAccidentalHoldsInItsOwnOctaveOnly(t *testing.T) {
	h := newHarness(t)
	typed(h, "f#4", "f5", "f4")

	// The F in the octave above was never sharpened, so it is an F and
	// needs no sign. The one that comes back to the sharpened octave is
	// still sharp, and needs none either.
	if got, want := pitches(h), "F#4 F5 F#4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got, want := signs(t, h), "#"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// A note written on a line an accidental has already altered takes the
// alteration, and says nothing.
//
// Reported twice over. Clicking the same line again after flattening a B
// gave a B natural and printed a natural in front of it: a sign nobody
// asked for, on a note nobody altered, with nothing to take it off but
// flattening a note that was flat already. What the person writing meant is
// what the person reading will read, which is the whole of the rule.
func TestANoteWrittenAfterAnAlteredOneTakesTheAlteration(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 1, "quarter") // the F in the bottom space, by hand
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp")
	h.note(0, 0, 1, "quarter")

	if got, want := pitches(h), "F#4 F#4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got, want := signs(t, h), "#"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// And a sign asked for twice over is printed once.
//
// Reported as three F naturals in a bar of a piece in G, each with a natural
// in front of it, because each was asked for one by one. The first says
// something — the key signature says sharp — and the two after it say what
// the bar has already said.
func TestASignAskedForAgainIsNotPrintedAgain(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setkey?part=0&fifths=1") // G major: every F is sharp
	for at := 1; at <= 3; at++ {
		h.note(0, 0, 1, "quarter")
		h.post("/api/accidental?part=0&measure=0&at=" + strconv.Itoa(at) +
			"&accidental=natural")
	}

	if got, want := pitches(h), "F4 F4 F4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got, want := signs(t, h), "n"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}
}

// Choosing a sign from the palette says how the note sounds. Whether it is
// printed is a question about the reader, and the answer is no when the
// reader already knows.
//
// A sharp chosen in a key that sharpens that letter anyway is the note the
// key already gives, so the page says nothing: the sign would be a second
// opinion about a note nobody is in doubt over. A reminder that is wanted
// regardless is what the brackets are for.
func TestASignIsPrintedOnlyWhereItSaysSomething(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setkey?part=0&fifths=1")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=quarter&kind=note&step=1&accidental=sharp")

	if got, want := pitches(h), "F#4"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}
	if got := signs(t, h); got != "" {
		t.Errorf("the page shows %q, want nothing: the signature said it", got)
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

// An accidental in brackets.
//
// A bracketed accidental is a reminder rather than an instruction: the note
// sounds the same with or without it, and the brackets say the editor is
// pointing something out. gscore 0.0.9 had the button; this did not.
func TestAnAccidentalCanBePutInBrackets(t *testing.T) {
	h := newHarness(t)
	typed(h, "f#", "g")

	h.post("/api/accidental?parens=toggle&sel=0:0:0")
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if !n.Parens {
		t.Fatal("the sign was not bracketed")
	}
	if n.Accidental != model.Sharp || n.Pitch.Alter != 1 {
		t.Errorf("the brackets changed the note: %v, alter %d",
			n.Accidental, n.Pitch.Alter)
	}
	// Three glyphs where there was one, in reading order.
	if got, want := signs(t, h), "( # )"; got != want {
		t.Errorf("the page shows %q, want %q", got, want)
	}

	// Asked again, they come off.
	h.post("/api/accidental?parens=toggle&sel=0:0:0")
	if n.Parens {
		t.Error("the brackets did not come off again")
	}
}

// There is nothing to bracket on a note with no sign, and saying so is
// better than a button that appears to do nothing.
func TestBracketsNeedASignToGoRound(t *testing.T) {
	h := newHarness(t)
	typed(h, "g")
	if msg := h.postFails("/api/accidental?parens=toggle&sel=0:0:0"); msg == "" {
		t.Error("a note with no accidental was bracketed")
	}
}

// Brackets belong to a sign. Taking the sign away takes them with it, or the
// next sign written there arrives already bracketed.
func TestTakingTheSignAwayTakesItsBrackets(t *testing.T) {
	h := newHarness(t)
	typed(h, "f#")
	h.post("/api/accidental?parens=toggle&sel=0:0:0")
	// Asking for the sign a note already has takes it off.
	h.post("/api/accidental?accidental=sharp&sel=0:0:0")

	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if n.Accidental != model.NoAccidental {
		t.Fatalf("the sharp is still there: %v", n.Accidental)
	}
	if n.Parens {
		t.Error("the brackets outlived the sign they were round")
	}
}

// And they can be asked for outright, which is what writing one in a file
// and reading it back has to do.
func TestBracketsCanBeTurnedOnAndOffOutright(t *testing.T) {
	h := newHarness(t)
	typed(h, "f#")
	h.post("/api/accidental?parens=on&sel=0:0:0")
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if !n.Parens {
		t.Error("parens=on did not put them on")
	}
	h.post("/api/accidental?parens=off&sel=0:0:0")
	if n.Parens {
		t.Error("parens=off did not take them off")
	}
}

// notesIn is the noteheads of an element: one for a note, all of them for a
// chord, none for anything else.
//
// It lives here because the tests are the only thing that wants it. It used
// to sit in edit.go, where alterInForce read it to work out what a reader had
// already been told in this bar — and when model.SpellAccidentals took that
// job over, nothing in the program needed it any more. A helper kept in
// production code for the sake of its tests reads as though something still
// calls it.
func notesIn(el model.Element) []*model.Note {
	switch e := el.(type) {
	case *model.Note:
		return []*model.Note{e}
	case *model.Chord:
		return e.Notes
	}
	return nil
}
