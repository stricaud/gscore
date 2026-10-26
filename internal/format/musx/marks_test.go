/*
 * marks_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musx

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The marks Finale keeps away from the notes.
 *
 * A dynamic, a slur and a hairpin are not written on the note in a Finale
 * file. They are rows in lists of their own, each saying where it lands, and
 * nothing in the music points back at them. Read as the music is built they
 * would all be forward references, so they are put back afterwards.
 */

// One staff of two bars in six eight, three quarter notes in the first: an
// "f" on the middle one, a slur from the first note to the last, a hairpin
// across the bar, and a tempo mark that is not a dynamic at all.
const enigmaMarks = `<?xml version="1.0" encoding="UTF-8"?>
<finale version="27.4" xmlns="http://www.makemusic.com/2012/finale">
  <others>
    <instUsed cmper="0" inci="0"><inst>1</inst></instUsed>
    <staffSpec cmper="1"><staffLines>5</staffLines><fullName>8</fullName></staffSpec>
    <textBlock cmper="8"><textID>22</textID></textBlock>
    <measSpec cmper="1"><beats>2</beats><divbeat>1536</divbeat><barline>normal</barline></measSpec>
    <measSpec cmper="2"><beats>2</beats><divbeat>1536</divbeat><barline>final</barline></measSpec>
    <frameSpec cmper="1" inci="0"><startEntry>1</startEntry><endEntry>3</endEntry></frameSpec>
    <markingsCategory cmper="1"><categoryType>dynamics</categoryType></markingsCategory>
    <markingsCategory cmper="2"><categoryType>tempoMarks</categoryType></markingsCategory>
    <textExprDef cmper="3">
      <textIDKey>20</textIDKey><categoryID>1</categoryID>
      <value>88</value><playType>amplitude</playType>
      <descStr>forte (velocity = 88)</descStr>
    </textExprDef>
    <textExprDef cmper="66">
      <textIDKey>21</textIDKey><categoryID>2</categoryID>
      <descStr>Andante</descStr>
    </textExprDef>
    <measExprAssign cmper="1" inci="0">
      <textExprID>3</textExprID><staffAssign>1</staffAssign><horzEduOff>1024</horzEduOff>
    </measExprAssign>
    <measExprAssign cmper="1" inci="1">
      <textExprID>66</textExprID><staffAssign>1</staffAssign>
    </measExprAssign>
    <smartShape cmper="1">
      <shapeType>slurAuto</shapeType><entryBased/>
      <startTermSeg><endPt><inst>1</inst><meas>1</meas><entryNum>1</entryNum></endPt></startTermSeg>
      <endTermSeg><endPt><inst>1</inst><meas>1</meas><entryNum>3</entryNum></endPt></endTermSeg>
    </smartShape>
    <smartShape cmper="2">
      <shapeType>cresc</shapeType>
      <startTermSeg><endPt><inst>1</inst><meas>1</meas></endPt></startTermSeg>
      <endTermSeg><endPt><inst>1</inst><meas>1</meas><edu>2048</edu></endPt></endTermSeg>
    </smartShape>
  </others>
  <details>
    <gfhold cmper1="1" cmper2="1"><clefID>0</clefID><frame1>1</frame1></gfhold>
  </details>
  <entries>
    <entry entnum="1" prev="0" next="2">
      <dura>1024</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>0</harmLev><harmAlt>0</harmAlt></note>
    </entry>
    <entry entnum="2" prev="1" next="3">
      <dura>1024</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>2</harmLev><harmAlt>0</harmAlt></note>
    </entry>
    <entry entnum="3" prev="2" next="0">
      <dura>1024</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>4</harmLev><harmAlt>0</harmAlt></note>
    </entry>
  </entries>
  <texts>
    <blockText number="22">^fontid(2)^size(14)^nfx(0)Flûte</blockText>
  </texts>
</finale>`

// marksAt is what is written on the nth element of the first bar.
func marksAt(t *testing.T, score *model.Score, n int) *model.Marks {
	t.Helper()
	els := score.Parts[0].Measures[0].Voices[0].Elements
	if n >= len(els) {
		t.Fatalf("the bar has %d things in it, not %d", len(els), n+1)
	}
	mk := marksOf(els[n])
	if mk == nil {
		t.Fatalf("a %T carries no marks", els[n])
	}
	return mk
}

// A dynamic is read from what it does, not from the character it is drawn
// with.
//
// The mark itself is a letter of Finale's own music font — "ë" is
// fortississimo if you have Maestro and know its layout, and nothing at all
// if you do not. The definition also carries a velocity, and that reads the
// same in every document: 88 is forte on Finale's ladder.
func TestADynamicIsReadFromItsVelocity(t *testing.T) {
	score := read1(t, fixture(t, enigmaMarks))

	// It lands where it is written: a thousand and twenty-four EDUs into
	// the bar is the second of three quarter notes.
	if got := marksAt(t, score, 1).Dynamic; got != model.Forte {
		t.Errorf("the second note carries %v, want f", got)
	}
	if got := marksAt(t, score, 0).Dynamic; got != model.DynamicNone {
		t.Errorf("the first note carries %v, and nothing is written on it", got)
	}
}

// Every rung of Finale's ladder, and an edited file that lands between two.
func TestTheVelocityLadder(t *testing.T) {
	for _, tc := range []struct {
		velocity int
		want     model.Dynamic
	}{
		{23, model.PPP}, {36, model.PP}, {49, model.Piano}, {62, model.MP},
		{75, model.MF}, {88, model.Forte}, {101, model.FF}, {127, model.FFFF},
		// Not a rung: nearest wins, which is what an edited velocity means.
		{90, model.Forte}, {0, model.PPPP},
	} {
		if got := dynamicFor(tc.velocity); got != tc.want {
			t.Errorf("velocity %d read as %v, want %v", tc.velocity, got, tc.want)
		}
	}
}

// A slur hangs on its two entries, which is the one kind of shape that says
// exactly which notes it joins.
func TestASlurJoinsTheEntriesItNames(t *testing.T) {
	score := read1(t, fixture(t, enigmaMarks))

	if !marksAt(t, score, 0).SlurStart {
		t.Error("the slur does not begin on the first note")
	}
	if !marksAt(t, score, 2).SlurStop {
		t.Error("the slur does not end on the third note")
	}
	// And not on the note in between, which is under it rather than at
	// either end of it.
	if mk := marksAt(t, score, 1); mk.SlurStart || mk.SlurStop {
		t.Error("the note under the slur was given an end of it")
	}
}

// A hairpin is attached to a beat rather than to a note, so it is placed by
// how far into the bar it falls.
func TestAHairpinOpensAndClosesWhereItIsPlaced(t *testing.T) {
	score := read1(t, fixture(t, enigmaMarks))

	start := marksAt(t, score, 0).Hairpin
	if start == nil || !start.Start || start.Kind != model.Crescendo {
		t.Fatalf("the first note carries %+v, want a crescendo opening", start)
	}
	end := marksAt(t, score, 2).Hairpin
	if end == nil || !end.Stop {
		t.Fatalf("the third note carries %+v, want the hairpin closing", end)
	}
}

// An expression that is not a dynamic is left alone and said out loud.
//
// A tempo word, a rehearsal direction and a technique all live in the same
// list as the dynamics. Read as dynamics they would be nonsense, and read as
// text they would be guesses about where they belong — so they are counted
// and reported instead.
func TestAnExpressionThatIsNotADynamicIsReported(t *testing.T) {
	_, err := Import(bytes.NewReader(fixture(t, enigmaMarks)))
	var partial interface {
		Partial()
		Details() []string
	}
	if !asPartial(err, &partial) {
		t.Fatalf("a file with a tempo mark in it said nothing about it: %v", err)
	}
	found := false
	for _, d := range partial.Details() {
		if strings.Contains(d, "expressions other than dynamics") {
			found = true
		}
	}
	if !found {
		t.Errorf("the log says %v, with nothing about the tempo mark", partial.Details())
	}
}

/*
 * Where a note is, and which note it is.
 *
 * Two things about a Finale note are not what they look like, and both are
 * invisible in a piece in C major played by instruments that do not
 * transpose — which is the only kind of file this reader had been tried
 * against.
 */

// harmLev counts from the tonic of the key, not from middle C.
//
// Read from middle C whatever the key, every note of every piece that is not
// in C came out wrong by the distance from C to its tonic — a fourth in G, a
// fifth in F — with the music itself intact, because every note moved
// together. Jingle Bells in G arrived with its guitar playing an F chord
// under a chord symbol saying C.
func TestHarmLevCountsFromTheTonic(t *testing.T) {
	for _, tc := range []struct {
		fifths int
		mode   model.Mode
		lev    int
		want   string
	}{
		// C major: the tonic is middle C, which is the old reading.
		{0, model.Major, 0, "C4"},
		{0, model.Major, 7, "C5"},
		{0, model.Major, -1, "B3"},
		// G major: harmLev 0 is the G above middle C.
		{1, model.Major, 0, "G4"},
		// Three steps up from G is the C above it.
		{1, model.Major, 3, "C5"},
		{1, model.Major, -1, "F4"},
		// F major: the tonic is the F above middle C.
		{-1, model.Major, 0, "F4"},
		// D major.
		{2, model.Major, 0, "D4"},
		// A minor shares its signature with C major and counts from A.
		{0, model.Minor, 0, "A4"},
	} {
		key := model.KeySignature{Fifths: tc.fifths, Mode: tc.mode}
		idx := tc.lev + tonicStep(key)
		got := fmt.Sprintf("%v%d", model.Step(mod(idx, 7)), 4+floorDiv(idx, 7))
		if got != tc.want {
			t.Errorf("%d fifths %v, harmLev %d: %s, want %s",
				tc.fifths, tc.mode, tc.lev, got, tc.want)
		}
	}
}

// What Finale stores is the note that sounds, and a transposing staff shows
// another one.
//
// A clarinet's staff holds the note the player reads, and everything after
// the importer — the page, the key signature over it, the transposition, the
// playback — is built on that. Stored as it sounds, the part printed concert
// pitch against the wrong signature and a player reading it would be a tone
// out.
func TestATransposingStaffIsWrittenAsItIsRead(t *testing.T) {
	// A B flat clarinet: it reads a tone above what it sounds.
	clarinet := model.Transposition{Diatonic: -1, Chromatic: -2}
	for _, tc := range []struct {
		sounds model.Pitch
		want   string
	}{
		// The note the instrument is named after: read C, sound B flat.
		{model.Pitch{Step: model.StepB, Octave: 3, Alter: -1}, "C4"},
		{model.Pitch{Step: model.StepC, Octave: 4}, "D4"},
		{model.Pitch{Step: model.StepC, Octave: 5, Alter: 1}, "D5#"},
	} {
		got := asWritten(tc.sounds, clarinet)
		if s := spell(got); s != tc.want {
			t.Errorf("sounding %s is written %s, want %s", spell(tc.sounds), s, tc.want)
		}
		// And it really does sound what Finale said: written plus the
		// transposition is the note back again.
		if sounds := got.MIDI() + clarinet.Chromatic; sounds != tc.sounds.MIDI() {
			t.Errorf("%s sounds MIDI %d, want %d", spell(got), sounds, tc.sounds.MIDI())
		}
	}

	// A soprano recorder: written an octave below what it sounds.
	recorder := model.Transposition{OctaveChange: 1}
	if got := spell(asWritten(model.Pitch{Step: model.StepC, Octave: 6}, recorder)); got != "C5" {
		t.Errorf("the recorder's sounding C6 is written %s, want C5", got)
	}

	// A staff that does not transpose is left exactly as it is.
	plain := model.Pitch{Step: model.StepF, Octave: 4, Alter: 1}
	if got := asWritten(plain, model.Transposition{}); got != plain {
		t.Errorf("a plain staff turned %s into %s", spell(plain), spell(got))
	}
}

// And the staff reads in the key its transposition puts it in.
func TestATransposingStaffReadsInItsOwnKey(t *testing.T) {
	var spec staffSpec
	spec.Transposition.Adjust = 2 // a B flat clarinet
	if got := writtenKey(model.KeySignature{Fifths: 0}, spec).Fifths; got != 2 {
		t.Errorf("a clarinet in a piece in C reads in %d sharps, want 2", got)
	}
	if got := writtenKey(model.KeySignature{Fifths: -1}, spec).Fifths; got != 1 {
		t.Errorf("a clarinet in a piece in F reads in %d sharps, want 1", got)
	}
}

// spell writes a pitch the way the tests above say it.
func spell(p model.Pitch) string {
	out := fmt.Sprintf("%v%d", p.Step, p.Octave)
	switch {
	case p.Alter > 0:
		out += "#"
	case p.Alter < 0:
		out += "b"
	}
	return out
}
