/*
 * tab_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing tablature.
 *
 * gscore could draw a tab before it could write one. What is typed is a
 * fret and what is stored is a pitch, because the pitch is what sounds and
 * what every other part of the program reads — so the thing to check is
 * that the two never disagree.
 */

package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

func firstNote(t *testing.T, h *harness, part int) *model.Note {
	t.Helper()
	for _, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					return n
				}
			}
		}
	}
	t.Fatal("nothing was written")
	return nil
}

// noteAt is where the first note of a staff sits: which bar and which
// place in it. A new score opens with a bar of rest already in it, so
// nothing written is at index zero.
func noteAt(t *testing.T, h *harness, part int) (measure, index int) {
	t.Helper()
	for mi, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for ei, el := range v.Elements {
				if _, ok := el.(*model.Note); ok {
					return mi, ei
				}
			}
		}
	}
	t.Fatal("nothing was written")
	return 0, 0
}

// notesOn counts what has actually been written on a staff, which is not
// the same as counting its elements: the bar of rest was there already.
func notesWritten(h *harness, part int) int {
	n := 0
	for _, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if _, ok := el.(*model.Note); ok {
					n++
				}
			}
		}
	}
	return n
}

func TestTheTuningsAreListed(t *testing.T) {
	h := newHarness(t)
	var out []struct {
		Name    string   `json:"name"`
		Strings []string `json:"strings"`
		Frets   int      `json:"frets"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/tunings")), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) < 8 {
		t.Errorf("only %d tunings", len(out))
	}
	if out[0].Name != "guitar" || len(out[0].Strings) != 6 {
		t.Errorf("the first tuning is %+v, want a six-string guitar", out[0])
	}
}

func TestAStaffCanBeWrittenAsTablature(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")

	p := h.app.score.Parts[0]
	if p.Tab == nil {
		t.Fatal("the staff is not a tab")
	}
	if p.Tab.Lines() != 6 {
		t.Errorf("it has %d strings", p.Tab.Lines())
	}
	// And back again. The notes keep their pitches either way: a tab is
	// how the music is shown and not what it is.
	h.post("/api/staff?part=0&tab=off")
	if h.app.score.Parts[0].Tab != nil {
		t.Error("the staff is still a tab")
	}
}

func TestAnUnknownTuningIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/staff?part=0&tab=sitar"); !strings.Contains(got, "tuning") {
		t.Errorf("it said %q", got)
	}
	if h.app.score.Parts[0].Tab != nil {
		t.Error("it made a tab anyway")
	}
}

// Turning a staff into a tab puts the notes already on it somewhere on the
// neck, at the first place that reaches them.
func TestExistingNotesGetAPlaceOnTheNeck(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter") // some pitch
	h.post("/api/staff?part=0&tab=guitar")

	n := firstNote(t, h, 0)
	if n.String < 1 || n.String > 6 {
		t.Errorf("the note is on string %d", n.String)
	}
	// And the fret it was given really is that pitch on that string.
	want, ok := h.app.score.Parts[0].Tab.Fretted(n.Pitch, n.String)
	if !ok || want != n.Fret {
		t.Errorf("%v on string %d is fret %d, but it says fret %d",
			n.Pitch, n.String, want, n.Fret)
	}
}

/*
 * Typing a fret.
 */

func TestAFretWritesTheNoteItStops(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=6&fret=3&dur=quarter")

	n := firstNote(t, h, 0)
	if n.String != 6 || n.Fret != 3 {
		t.Errorf("it went to string %d fret %d", n.String, n.Fret)
	}
	// Third fret of the bottom E is G2.
	if got, want := n.Pitch.MIDI(), 43; got != want {
		t.Errorf("it sounds %v (%d), want G2 (%d)", n.Pitch, got, want)
	}
}

func TestAnOpenStringIsFretZero(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=1&fret=0&dur=quarter")

	if got, want := firstNote(t, h, 0).Pitch.MIDI(), 64; got != want {
		t.Errorf("the open top string sounds %d, want E4 (%d)", got, want)
	}
}

func TestAFretThatIsNotThereIsRefused(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")

	for _, q := range []string{
		"part=0&measure=0&string=9&fret=0&dur=quarter",
		"part=0&measure=0&string=1&fret=99&dur=quarter",
		"part=0&measure=0&string=1&fret=-1&dur=quarter",
		"part=0&measure=0&fret=3&dur=quarter",
		"part=0&measure=0&string=1&dur=quarter",
	} {
		if got := h.postFails("/api/fret?" + q); strings.Contains(got, "meant to be refused") {
			t.Errorf("%q was allowed", q)
		}
	}
	if n := notesWritten(h, 0); n != 0 {
		t.Errorf("a refused fret wrote %d notes", n)
	}
}

// A fret means nothing on a staff of pitches, and saying so is better than
// guessing which string was meant.
func TestAFretOnAnOrdinaryStaffIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/fret?part=0&measure=0&string=1&fret=3&dur=quarter"); !strings.Contains(got, "tablature") {
		t.Errorf("it said %q", got)
	}
}

/*
 * Moving a note along the neck.
 *
 * The same pitch is in several places and they do not finger alike: the
 * open B string and the fourth fret of the G are the same note and a
 * different thing to play.
 */

func TestANoteCanBeMovedToAnotherString(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=2&fret=0&dur=quarter") // open B
	was := firstNote(t, h, 0).Pitch
	mi, at := noteAt(t, h, 0)

	h.post(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&string=3", mi, at))
	n := firstNote(t, h, 0)
	if n.String != 3 || n.Fret != 4 {
		t.Errorf("it went to string %d fret %d, want string 3 fret 4", n.String, n.Fret)
	}
	// And it is the same note. That is the whole point: the hand moves
	// and the music does not.
	if n.Pitch != was {
		t.Errorf("it now sounds %v, was %v", n.Pitch, was)
	}
}

func TestANoteCanBeSteppedAlongTheNeck(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=2&fret=0&dur=quarter")
	mi, at := noteAt(t, h, 0)

	// Down the page is towards the lower strings.
	h.post(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&move=down", mi, at))
	if got := firstNote(t, h, 0).String; got != 3 {
		t.Errorf("moving down went to string %d, want 3", got)
	}
	h.post(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&move=up", mi, at))
	if got := firstNote(t, h, 0).String; got != 2 {
		t.Errorf("moving back up went to string %d, want 2", got)
	}
}

// A string that cannot reach the note says so, and nothing moves.
func TestANoteWillNotMoveToAStringThatCannotReachIt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=6&fret=0&dur=quarter") // low E
	mi, at := noteAt(t, h, 0)

	got := h.postFails(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&string=1", mi, at))
	if !strings.Contains(got, "cannot be played") {
		t.Errorf("it said %q", got)
	}
	if n := firstNote(t, h, 0); n.String != 6 || n.Fret != 0 {
		t.Errorf("it moved anyway, to string %d fret %d", n.String, n.Fret)
	}
}

func TestOnlyANoteIsPlayedOnAString(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/insert?part=0&measure=0&kind=rest&dur=quarter")

	if got := h.postFails("/api/restring?part=0&measure=0&at=0&string=2"); got == "" {
		t.Error("a rest was moved to a string")
	}
}

// Retuning a staff moves the frets under the notes, because the notes are
// the pitches and the frets are where those pitches now are.
func TestRetuningMovesTheFretsAndNotTheNotes(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=6&fret=3&dur=quarter") // G2
	was := firstNote(t, h, 0).Pitch

	h.post("/api/staff?part=0&tab=guitar%20%E2%80%94%20drop%20D")
	n := firstNote(t, h, 0)
	if n.Pitch != was {
		t.Errorf("retuning changed the note to %v, was %v", n.Pitch, was)
	}
	// The sixth string is now D2, so G2 is at the fifth fret.
	if n.String != 6 || n.Fret != 5 {
		t.Errorf("after dropping to D it reads string %d fret %d, want 6 and 5",
			n.String, n.Fret)
	}
}

// What a tab says about itself, for the dialog that sets it.
func TestAStaffSaysWhatItIsTunedTo(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=ukulele")

	got := tabOf(h.app.score.Parts[0])
	if got == nil {
		t.Fatal("it says nothing")
	}
	if got.Tuning != "ukulele" || len(got.Strings) != 4 || got.Frets != 15 {
		t.Errorf("it says %+v", got)
	}
	if tabOf(&model.Part{}) != nil {
		t.Error("a staff of pitches claims a tuning")
	}
}

/*
 * A fretted instrument as a way of writing, not as a different
 * instrument.
 *
 * A guitar part is usually printed twice over: the notation above and the
 * tablature below, joined down the left with the barlines run through,
 * because it is one player reading one line.
 */

func TestAGuitarCanBeAddedAsTab(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=tab")

	p := h.app.score.Parts[len(h.app.score.Parts)-1]
	if p.Tab == nil {
		t.Fatal("it is not a tab")
	}
	if p.Tab.Lines() != 6 {
		t.Errorf("%d strings", p.Tab.Lines())
	}
	// Still a guitar: the sound written into the file is the same
	// whichever way it is shown.
	if p.Sound != "pluck.guitar" {
		t.Errorf("its sound is %q", p.Sound)
	}
}

func TestAGuitarCanBeAddedAsNotationAndTab(t *testing.T) {
	h := newHarness(t)
	before := len(h.app.score.Parts)
	h.post("/api/addstaff?sound=pluck.guitar&written=both")

	if got := len(h.app.score.Parts) - before; got != 2 {
		t.Fatalf("it made %d staves, want 2", got)
	}
	top := h.app.score.Parts[before]
	bottom := h.app.score.Parts[before+1]
	if top.Tab != nil {
		t.Error("the upper staff is a tab; it should be the notation")
	}
	if bottom.Tab == nil {
		t.Error("the lower staff is not a tab")
	}
	// Named once, over the pair.
	if bottom.Name != "" {
		t.Errorf("the lower staff is called %q", bottom.Name)
	}
	// Joined, with the barlines run through: one player, one line.
	var joined *model.StaffGroup
	for i := range h.app.score.Groups {
		g := &h.app.score.Groups[i]
		if g.First == before && g.Last == before+1 {
			joined = g
		}
	}
	if joined == nil {
		t.Fatal("the two staves are not joined")
	}
	if !joined.Barlines {
		t.Error("the barlines do not run through")
	}
}

// Both halves are the same music, so they transpose and sound alike.
func TestTheTabHalfIsTheSameInstrument(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.bass&written=both")

	top := h.app.score.Parts[len(h.app.score.Parts)-2]
	bottom := h.app.score.Parts[len(h.app.score.Parts)-1]
	if top.Sound != bottom.Sound {
		t.Errorf("%q above and %q below", top.Sound, bottom.Sound)
	}
	if top.Transpose != bottom.Transpose {
		t.Errorf("%+v above and %+v below", top.Transpose, bottom.Transpose)
	}
	// A bass is four strings, so four lines — not five.
	if bottom.Tab.Lines() != 4 {
		t.Errorf("the bass tab has %d lines", bottom.Tab.Lines())
	}
}

// Something with no frets cannot be written as tab, and says so.
func TestAnInstrumentWithNoFretsIsRefused(t *testing.T) {
	h := newHarness(t)
	got := h.postFails("/api/addstaff?sound=wind.flutes.flute&written=tab")
	if !strings.Contains(got, "frets") {
		t.Errorf("it said %q", got)
	}
}

func TestANonsenseWayOfWritingIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/addstaff?sound=pluck.guitar&written=sideways"); got == "" {
		t.Error("it was allowed")
	}
}

// How many lines a tab is ruled with is not a setting of its own: it is
// however many strings the instrument has. A bass is four and often five.
func TestABassTabCanHaveFiveStrings(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.bass&written=tab")
	four := h.app.score.Parts[len(h.app.score.Parts)-1]
	if four.Tab.Lines() != 4 {
		t.Errorf("the default bass has %d lines", four.Tab.Lines())
	}

	h.post("/api/addstaff?sound=pluck.bass&written=tab&tuning=" +
		url.QueryEscape("bass — five string"))
	five := h.app.score.Parts[len(h.app.score.Parts)-1]
	if five.Tab.Lines() != 5 {
		t.Errorf("the five-string bass has %d lines", five.Tab.Lines())
	}
	// And the extra string is the low B below the E.
	if got := five.Tab.Strings[4].MIDI(); got != 23 {
		t.Errorf("its bottom string is %d, want B0 (23)", got)
	}
}

func TestAChosenTuningAppliesToThePairToo(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=both&tuning=" +
		url.QueryEscape("guitar — drop D"))
	tab := h.app.score.Parts[len(h.app.score.Parts)-1]
	if tab.Tab == nil {
		t.Fatal("the lower staff is not a tab")
	}
	// Dropped to D: the sixth string is two semitones below E2.
	if got := tab.Tab.Strings[5].MIDI(); got != 38 {
		t.Errorf("the sixth string is %d, want D2 (38)", got)
	}
}

func TestATuningNobodyHasIsRefused(t *testing.T) {
	h := newHarness(t)
	got := h.postFails("/api/addstaff?sound=pluck.bass&written=tab&tuning=sitar")
	if !strings.Contains(got, "tuning") {
		t.Errorf("it said %q", got)
	}
}

/*
 * The pair is one line written twice.
 *
 * Two staves each holding their own copy would be two copies to keep in
 * step, and they would not stay in step: a note fixed in one and not the
 * other is the mistake nobody finds until somebody plays from the wrong
 * half. So the tablature holds nothing and is drawn from the staff above.
 */

func TestTheTabHalfHoldsNoMusicOfItsOwn(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=both")

	n := len(h.app.score.Parts)
	above, below := h.app.score.Parts[n-2], h.app.score.Parts[n-1]
	if !below.Mirrors() {
		t.Fatal("the tab does not name the staff it shows")
	}
	if len(below.Measures) != 0 {
		t.Errorf("the tab holds %d bars of its own", len(below.Measures))
	}
	// And it names the staff above it.
	if got := h.app.score.MusicOf(n - 1); got != above {
		t.Error("the tab is drawn from the wrong staff")
	}
}

// Typing a fret on the tablature writes the note onto the staff above,
// which is where it is already being read from.
func TestAFretTypedOnTheTabLandsOnTheNotation(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=both")
	n := len(h.app.score.Parts)
	above, below := n-2, n-1

	h.post("/api/clearmeasure?part=" + itoa(above) + "&measure=0")
	h.post("/api/fret?part=" + itoa(below) + "&measure=0&string=6&fret=3&dur=quarter")

	// On the staff that holds the music.
	got := firstNote(t, h, above)
	if got.String != 6 || got.Fret != 3 {
		t.Errorf("it went to string %d fret %d", got.String, got.Fret)
	}
	// The model keeps what is printed, and a guitar reads an octave
	// above what it sounds: the third fret of the bottom string sounds
	// G2 and is written G3.
	if got.Pitch.MIDI() != 55 {
		t.Errorf("it is written %v, want G3", got.Pitch)
	}
	if sounds := h.app.score.Parts[above].Transpose.Apply(got.Pitch); sounds.MIDI() != 43 {
		t.Errorf("it sounds %v, want G2", sounds)
	}
	// And the tab still holds nothing: it shows that note, it does not
	// have one.
	if len(h.app.score.Parts[below].Measures) != 0 {
		t.Error("the tab grew bars of its own")
	}
}

// Both staves draw the same note, and it is the same note: one element,
// so clicking either picks up the same thing and playback lights both.
func TestBothHalvesDrawTheOneNote(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=both")
	n := len(h.app.score.Parts)
	h.post("/api/clearmeasure?part=" + itoa(n-2) + "&measure=0")
	h.post("/api/fret?part=" + itoa(n-1) + "&measure=0&string=2&fret=1&dur=quarter")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	// The fret number on the tab and the notehead above it.
	frets, heads := 0, 0
	for _, tx := range lay.Texts {
		if tx.S == "1" {
			frets++
		}
	}
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") {
			heads++
		}
	}
	if frets == 0 {
		t.Error("the tab shows no fret")
	}
	if heads == 0 {
		t.Error("the notation shows no notehead")
	}
}

// Bars added to the score are not added to a staff that holds none.
func TestAMirrorIsNotPaddedWithBars(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=both")
	n := len(h.app.score.Parts)

	h.post("/api/addmeasure")
	h.post("/api/insertmeasure?measure=0&count=2")

	if got := len(h.app.score.Parts[n-1].Measures); got != 0 {
		t.Errorf("the tab was given %d bars", got)
	}
}

/*
 * Running out of room.
 *
 * A fret typed into a bar that is already full belongs to the next bar.
 * Without that the numbers pile up behind the barline, where they are
 * drawn on top of each other and cannot be reached, and the only way to
 * keep typing is to click into the next bar by hand — which is exactly
 * the interruption the keyboard is there to avoid.
 */

// notesByBar says which bar each note of a staff is in, in order.
func notesByBar(h *harness, part int) []int {
	var out []int
	for mi, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if _, ok := el.(*model.Note); ok {
					out = append(out, mi)
				}
			}
		}
	}
	return out
}

func TestAFretTypedIntoAFullBarGoesIntoTheNext(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")

	// Four quarters fill a bar of four four, and the fifth cannot.
	for i := 0; i < 5; i++ {
		h.post(fmt.Sprintf("/api/fret?part=0&measure=0&string=1&fret=%d&dur=quarter", i))
	}

	bars := notesByBar(h, 0)
	if len(bars) != 5 {
		t.Fatalf("%d notes were written, want 5", len(bars))
	}
	for i, want := range []int{0, 0, 0, 0, 1} {
		if bars[i] != want {
			t.Errorf("note %d is in bar %d, want bar %d (all of them: %v)",
				i+1, bars[i], want, bars)
		}
	}
}

// And the answer says where it went, because the caret follows it there.
// A caret left in the bar that was typed at would put the next fret back
// behind the barline, which is the thing this is for.
func TestTheAnswerSaysWhichBarTheFretLandedIn(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	for i := 0; i < 4; i++ {
		h.post(fmt.Sprintf("/api/fret?part=0&measure=0&string=1&fret=%d&dur=quarter", i))
	}

	out := h.post("/api/fret?part=0&measure=0&string=1&fret=7&dur=quarter")
	if got, ok := out["measure"].(float64); !ok || int(got) != 1 {
		t.Errorf("it says the fret landed in bar %v, want bar 1", out["measure"])
	}
	if got, ok := out["at"].(float64); !ok || int(got) != 0 {
		t.Errorf("it says the fret is at %v of that bar, want 0", out["at"])
	}
}

// Putting one into the middle of a full bar pushes the end of the bar
// over the barline rather than squeezing an extra beat into it.
func TestAFretPutInPartwayPushesTheRestAlong(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	for i := 0; i < 4; i++ {
		h.post(fmt.Sprintf("/api/fret?part=0&measure=0&string=1&fret=%d&dur=quarter", i))
	}

	h.post("/api/fret?part=0&measure=0&at=1&string=1&fret=9&dur=quarter")

	bars := notesByBar(h, 0)
	if len(bars) != 5 {
		t.Fatalf("%d notes, want 5", len(bars))
	}
	if bars[4] != 1 {
		t.Errorf("the bar still holds all five notes: %v", bars)
	}
	// The one typed is where it was asked for, not pushed on itself.
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[1].(*model.Note)
	if n.Fret != 9 {
		t.Errorf("the second note of the bar is fret %d, want the 9 just typed", n.Fret)
	}
}
