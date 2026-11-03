/*
 * voicing_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Taking one player's line off a staff that carries two.
 *
 * The case this exists for: "Flute 1, 2" on one staff, a chord where the
 * players differ and a single note where they agree. The first flute's
 * part reads the upper note, the second the lower, and the note they share
 * belongs to both of them — which is the one everybody gets wrong, because
 * a rule that only ever picks out of chords leaves the second player
 * resting through every passage the two play together.
 */

package model

import "testing"

func pitchOf(t *testing.T, s string) Pitch {
	t.Helper()
	p, _, ok := ParsePitch(s)
	if !ok {
		t.Fatalf("%q is not a pitch", s)
	}
	return p
}

func half() Duration { return Duration{Num: 1, Den: 2} }

// chordBar is a bar of one chord, written highest note first.
func chordBar(t *testing.T, names ...string) *Measure {
	t.Helper()
	c := &Chord{Dur: half()}
	for _, n := range names {
		c.Notes = append(c.Notes, &Note{Pitch: pitchOf(t, n), Dur: half()})
	}
	return &Measure{Voices: []*Voice{{Elements: []Element{c}}}}
}

func noteBar(t *testing.T, name string) *Measure {
	t.Helper()
	return &Measure{Voices: []*Voice{{Elements: []Element{
		&Note{Pitch: pitchOf(t, name), Dur: half()},
	}}}}
}

// only is the single element a voiced bar came to, and fails otherwise.
func only(t *testing.T, m *Measure) Element {
	t.Helper()
	if len(m.Voices) != 1 {
		t.Fatalf("the bar has %d voices", len(m.Voices))
	}
	if n := len(m.Voices[0].Elements); n != 1 {
		t.Fatalf("the bar has %d elements, want one", n)
	}
	return m.Voices[0].Elements[0]
}

func noteIn(t *testing.T, m *Measure) *Note {
	t.Helper()
	n, ok := only(t, m).(*Note)
	if !ok {
		t.Fatalf("the bar holds a %T, want a note", only(t, m))
	}
	return n
}

func TestTheFirstPlayerTakesTheTopNote(t *testing.T) {
	v := Voicing{Pick: VoicingTop}
	got := noteIn(t, v.Voiced(chordBar(t, "C5", "E4")))
	if want := pitchOf(t, "C5"); got.Pitch != want {
		t.Errorf("the first flute reads %v, want %v", got.Pitch, want)
	}
}

func TestTheSecondPlayerTakesTheBottomNote(t *testing.T) {
	v := Voicing{Pick: VoicingBottom}
	got := noteIn(t, v.Voiced(chordBar(t, "C5", "E4")))
	if want := pitchOf(t, "E4"); got.Pitch != want {
		t.Errorf("the second flute reads %v, want %v", got.Pitch, want)
	}
}

// Whatever order the notes were typed in. A chord written from the bottom
// up is the same chord.
func TestHighAndLowAreByPitchAndNotByOrderWritten(t *testing.T) {
	for _, order := range [][]string{{"C5", "E4"}, {"E4", "C5"}} {
		top := Voicing{Pick: VoicingTop}.Voiced(chordBar(t, order...))
		if got, want := noteIn(t, top).Pitch, pitchOf(t, "C5"); got != want {
			t.Errorf("written %v, the top note came out %v", order, got)
		}
	}
}

// The a2 bar: both players on one note. It belongs to both parts, and a
// part that dropped it would rest through the passage.
func TestANoteBothPlayersShareIsInBothParts(t *testing.T) {
	for _, pick := range []VoicingPick{VoicingTop, VoicingBottom} {
		v := Voicing{Pick: pick}
		got := noteIn(t, v.Voiced(noteBar(t, "G4")))
		if want := pitchOf(t, "G4"); got.Pitch != want {
			t.Errorf("%v took %v from an a2 bar, want %v", pick, got.Pitch, want)
		}
	}
}

// One note is a note, not a chord holding one. Everything downstream — the
// stem, the ties, the player — reads it as though it had been written that
// way, because in the part it was.
func TestAChordReducedToOneNoteIsANote(t *testing.T) {
	v := Voicing{Pick: VoicingBottom}
	if _, ok := only(t, v.Voiced(chordBar(t, "C5", "E4"))).(*Note); !ok {
		t.Error("the part holds a chord of one note")
	}
}

// Three to a staff, which is how horns are written.
func TestNthTakesANamedNoteOfTheChord(t *testing.T) {
	bar := chordBar(t, "G5", "C5", "E4")
	for _, tc := range []struct {
		nth        int
		fromBottom bool
		want       string
	}{
		{1, false, "G5"}, {2, false, "C5"}, {3, false, "E4"},
		{1, true, "E4"}, {2, true, "C5"}, {3, true, "G5"},
	} {
		v := Voicing{Pick: VoicingNth, Nth: []int{tc.nth}, FromBottom: tc.fromBottom}
		got := noteIn(t, v.Voiced(bar))
		if want := pitchOf(t, tc.want); got.Pitch != want {
			t.Errorf("note %d (fromBottom=%v) is %v, want %v",
				tc.nth, tc.fromBottom, got.Pitch, want)
		}
	}
}

func TestNthCanTakeSeveralNotes(t *testing.T) {
	v := Voicing{Pick: VoicingNth, Nth: []int{1, 3}}
	c, ok := only(t, v.Voiced(chordBar(t, "G5", "C5", "E4"))).(*Chord)
	if !ok {
		t.Fatal("two notes did not come back as a chord")
	}
	if len(c.Notes) != 2 {
		t.Fatalf("the chord has %d notes", len(c.Notes))
	}
	if c.Notes[0].Pitch != pitchOf(t, "G5") || c.Notes[1].Pitch != pitchOf(t, "E4") {
		t.Errorf("took %v and %v", c.Notes[0].Pitch, c.Notes[1].Pitch)
	}
}

// A player who is not in the chord sits out, and sits out counting: a rest
// of the right length, not a bar with nothing in it.
func TestAPlayerNotInTheChordRests(t *testing.T) {
	v := Voicing{Pick: VoicingNth, Nth: []int{3}}
	r, ok := only(t, v.Voiced(chordBar(t, "C5", "E4"))).(*Rest)
	if !ok {
		t.Fatalf("the third player got a %T", only(t, v.Voiced(chordBar(t, "C5", "E4"))))
	}
	if r.Dur != half() {
		t.Errorf("the rest is %v long, want %v", r.Dur, half())
	}
}

// And with the a2 rule set, a lone note is everybody's however they count.
func TestSingleNotePassagesCanBeKeptForEveryone(t *testing.T) {
	bar := noteBar(t, "G4")
	out := Voicing{Pick: VoicingNth, Nth: []int{3}, Single: true}.Voiced(bar)
	if got := noteIn(t, out).Pitch; got != pitchOf(t, "G4") {
		t.Errorf("the third player reads %v", got)
	}
	// Without it, the third player is not in a one-note bar.
	out = Voicing{Pick: VoicingNth, Nth: []int{3}}.Voiced(bar)
	if _, ok := only(t, out).(*Rest); !ok {
		t.Error("without the a2 rule the third player should be resting")
	}
}

/*
 * Layers: two players who are rhythmically apart.
 */

func layerBar(t *testing.T, a, b string) *Measure {
	t.Helper()
	return &Measure{Voices: []*Voice{
		{Elements: []Element{&Note{Pitch: pitchOf(t, a), Dur: half()}}},
		{Elements: []Element{&Note{Pitch: pitchOf(t, b), Dur: half()}}},
	}}
}

func TestALayerRuleTakesOneVoice(t *testing.T) {
	bar := layerBar(t, "C5", "E4")
	for layer, want := range map[int]string{1: "C5", 2: "E4"} {
		got := noteIn(t, Voicing{Layer: layer}.Voiced(bar))
		if got.Pitch != pitchOf(t, want) {
			t.Errorf("layer %d reads %v, want %v", layer, got.Pitch, want)
		}
	}
}

// A bar with layers this part is not in is a bar of rest, which can be
// counted into a multirest. A bar with nothing in it cannot.
func TestABarWithoutThisPartsLayerIsABarOfRest(t *testing.T) {
	out := Voicing{Layer: 3}.Voiced(layerBar(t, "C5", "E4"))
	if _, ok := only(t, out).(*Rest); !ok {
		t.Errorf("the missing layer gave a %T", only(t, out))
	}
}

// The two rules answer different bars, which is the whole point: a real
// part crosses between chords and layers bar by bar, and nobody should
// have to say which is which.
func TestChordsAndLayersAreAnsweredByTheirOwnRule(t *testing.T) {
	v := Voicing{Pick: VoicingBottom, Layer: 1}

	// A bar of chords is answered by the chord rule.
	if got := noteIn(t, v.Voiced(chordBar(t, "C5", "E4"))).Pitch; got != pitchOf(t, "E4") {
		t.Errorf("the chord bar gave %v, want the bottom note E4", got)
	}
	// A bar of layers is answered by the layer rule, and layer 1 is the
	// upper line — not the bottom note.
	if got := noteIn(t, v.Voiced(layerBar(t, "C5", "E4"))).Pitch; got != pitchOf(t, "C5") {
		t.Errorf("the layered bar gave %v, want layer one C5", got)
	}
}

/*
 * What it must not do.
 */

// The score is never touched. A part is a rule read against the score's own
// notes, and a reduction that edited them would be the bug the whole design
// exists to prevent.
func TestVoicingLeavesTheScoreAlone(t *testing.T) {
	bar := chordBar(t, "C5", "E4")
	before := len(bar.Voices[0].Elements[0].(*Chord).Notes)

	Voicing{Pick: VoicingBottom}.Voiced(bar)
	Voicing{Pick: VoicingTop}.Voiced(bar)
	Voicing{Layer: 1}.Voiced(bar)

	after := len(bar.Voices[0].Elements[0].(*Chord).Notes)
	if before != after {
		t.Errorf("the score's chord went from %d notes to %d", before, after)
	}
}

// A rule that chooses everything is no rule, and must hand back the bar it
// was given rather than a copy of it: the layout cache behind the part
// keys on what it is handed.
func TestAPlainRuleChangesNothing(t *testing.T) {
	bar := chordBar(t, "C5", "E4")
	if got := (Voicing{}).Voiced(bar); got != bar {
		t.Error("a plain rule rebuilt the bar")
	}
	if !(Voicing{}).IsPlain() {
		t.Error("a rule with nothing set is not plain")
	}
	if (Voicing{Pick: VoicingBottom}).IsPlain() {
		t.Error("taking the bottom note is a plain rule")
	}
}

func TestRestsAndEverythingElseComeThrough(t *testing.T) {
	bar := &Measure{Voices: []*Voice{{Elements: []Element{
		&Rest{Dur: half()},
		&Chord{Dur: half(), Notes: []*Note{
			{Pitch: pitchOf(t, "C5"), Dur: half()},
			{Pitch: pitchOf(t, "E4"), Dur: half()},
		}},
	}}}}
	out := Voicing{Pick: VoicingBottom}.Voiced(bar)
	if n := len(out.Voices[0].Elements); n != 2 {
		t.Fatalf("%d elements came through, want 2", n)
	}
	if _, ok := out.Voices[0].Elements[0].(*Rest); !ok {
		t.Error("the rest was swallowed")
	}
}

// A sheet answers for a staff it has no rule about, so a part printing two
// staves can voice one and print the other whole.
func TestASheetAnswersForEachStaff(t *testing.T) {
	sh := Sheet{Voicing: []Voicing{{Staff: 2, Pick: VoicingBottom}}}

	if v, ok := sh.VoicingFor(2); !ok || v.Pick != VoicingBottom {
		t.Errorf("staff 2 got %+v, %v", v, ok)
	}
	if _, ok := sh.VoicingFor(0); ok {
		t.Error("staff 0 has a rule it was never given")
	}
}
