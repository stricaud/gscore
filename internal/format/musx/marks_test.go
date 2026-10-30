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
// across the bar, a tempo word over the bar, a rehearsal letter naming it
// and a direction written on the last note.
const enigmaMarks = `<?xml version="1.0" encoding="UTF-8"?>
<finale version="27.4" xmlns="http://www.makemusic.com/2012/finale">
  <others>
    <instUsed cmper="0" inci="0"><inst>1</inst></instUsed>
    <staffSpec cmper="1"><staffLines>5</staffLines><fullName>8</fullName></staffSpec>
    <textBlock cmper="8"><textID>22</textID></textBlock>
    <textBlock cmper="21"><textID>66</textID><textTag>expression</textTag></textBlock>
    <textBlock cmper="23"><textID>67</textID><textTag>expression</textTag></textBlock>
    <textBlock cmper="24"><textID>68</textID><textTag>expression</textTag></textBlock>
    <measSpec cmper="1"><beats>2</beats><divbeat>1536</divbeat><barline>normal</barline></measSpec>
    <measSpec cmper="2"><beats>2</beats><divbeat>1536</divbeat><barline>final</barline></measSpec>
    <frameSpec cmper="1" inci="0"><startEntry>1</startEntry><endEntry>3</endEntry></frameSpec>
    <markingsCategory cmper="1"><categoryType>dynamics</categoryType></markingsCategory>
    <markingsCategory cmper="2"><categoryType>tempoMarks</categoryType></markingsCategory>
    <markingsCategory cmper="5"><categoryType>techniqueText</categoryType></markingsCategory>
    <markingsCategory cmper="6"><categoryType>rehearsalMarks</categoryType></markingsCategory>
    <textExprDef cmper="3">
      <textIDKey>20</textIDKey><categoryID>1</categoryID>
      <value>88</value><playType>amplitude</playType>
      <descStr>forte (velocity = 88)</descStr>
    </textExprDef>
    <textExprDef cmper="66">
      <textIDKey>21</textIDKey><categoryID>2</categoryID>
      <value>72</value><auxdata1>1024</auxdata1><playType>time</playType>
      <descStr>Andante (playback Tempo = 72)</descStr>
    </textExprDef>
    <textExprDef cmper="67">
      <textIDKey>23</textIDKey><categoryID>5</categoryID>
      <vertMeasExprAlign>aboveStaffOrEntry</vertMeasExprAlign>
    </textExprDef>
    <textExprDef cmper="68">
      <textIDKey>24</textIDKey><categoryID>6</categoryID>
    </textExprDef>
    <measExprAssign cmper="1" inci="0">
      <textExprID>3</textExprID><staffAssign>1</staffAssign><horzEduOff>1024</horzEduOff>
    </measExprAssign>
    <measExprAssign cmper="1" inci="1">
      <textExprID>66</textExprID><staffAssign>-1</staffAssign><staffList>1</staffList>
    </measExprAssign>
    <measExprAssign cmper="1" inci="2">
      <textExprID>67</textExprID><staffAssign>1</staffAssign><horzEduOff>2048</horzEduOff>
    </measExprAssign>
    <measExprAssign cmper="2" inci="0">
      <textExprID>68</textExprID><staffAssign>-1</staffAssign><staffList>1</staffList>
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
    <expression number="66">^fontTxt(Times New Roman,8192)^size(14)^nfx(1)Andante </expression>
    <expression number="67">^fontTxt(Times New Roman,8192)^size(12)^nfx(1)pizz.</expression>
    <expression number="68">^fontid(2)^size(14)^nfx(1)B</expression>
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

// A tempo word is read, and so is the speed behind it.
//
// Finale keeps the words of a mark in a pool of its own, apart from the text
// laid on the page, and a reader that knows only about the page's pool finds
// every tempo word and every direction empty. That is how "Andante" and
// "Andantino mosso" went missing from a Rossini that printed both.
func TestATempoWordIsRead(t *testing.T) {
	score := read1(t, fixture(t, enigmaMarks))
	m := score.Parts[0].Measures[0]
	if m.Tempo == nil {
		t.Fatal("the bar carries no tempo mark")
	}
	if m.Tempo.Text != "Andante" {
		t.Errorf("the tempo mark says %q", m.Tempo.Text)
	}
	// It drives playback too, in the note it is counted in.
	if m.Tempo.BPM != 72 {
		t.Errorf("the speed came out %d, want 72", m.Tempo.BPM)
	}
	if want := (model.Duration{Num: 1, Den: 4}); m.Tempo.Unit.Reduced() != want {
		t.Errorf("the beat came out %v, want %v", m.Tempo.Unit, want)
	}
}

// A mark written for the whole band belongs to the top staff, which is
// where a score prints it.
func TestAMarkForEveryStaffGoesOnTheTopOne(t *testing.T) {
	score := read1(t, fixture(t, enigmaMarks))
	// The tempo is assigned to a list of staves rather than to one, which
	// is how Finale says "this is for everybody".
	if score.Parts[0].Measures[0].Tempo == nil {
		t.Error("a mark for the whole band was dropped for naming no staff")
	}
	if got := score.Parts[0].Measures[1].Marker; got != "B" {
		t.Errorf("the second bar is lettered %q, want B", got)
	}
}

// Words said to the player hang on the note they stand over.
func TestADirectionIsWrittenOnItsNote(t *testing.T) {
	score := read1(t, fixture(t, enigmaMarks))
	// Two quarter notes into the bar, which is the third note.
	mk := marksAt(t, score, 2)
	if len(mk.Texts) != 1 {
		t.Fatalf("the note carries %d words, want one", len(mk.Texts))
	}
	if mk.Texts[0].Text != "pizz." {
		t.Errorf("it says %q", mk.Texts[0].Text)
	}
	if mk.Texts[0].Placement != model.PlacementAbove {
		t.Errorf("it is written %v, and the file says above", mk.Texts[0].Placement)
	}
	// And not on the notes it was not written on.
	if got := marksAt(t, score, 0).Texts; len(got) != 0 {
		t.Errorf("the first note was given %v", got)
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

// The report says what was left behind, in the file's own words.
//
// "Directions: not read (2 found)" sends somebody back to Finale to work
// out which two. "pizz. at bar 1" tells them what was lost and lets them
// judge at a glance whether it mattered.
func TestTheReportNamesWhatItPassedOver(t *testing.T) {
	// A direction written across a list of staves rather than on one of
	// them: read it onto a staff and it would be a guess about which.
	const spread = `<?xml version="1.0" encoding="UTF-8"?>
<finale version="27.4" xmlns="http://www.makemusic.com/2012/finale">
  <others>
    <instUsed cmper="0" inci="0"><inst>1</inst></instUsed>
    <staffSpec cmper="1"><staffLines>5</staffLines></staffSpec>
    <textBlock cmper="23"><textID>67</textID><textTag>expression</textTag></textBlock>
    <measSpec cmper="1"><beats>2</beats><divbeat>1536</divbeat></measSpec>
    <markingsCategory cmper="5"><categoryType>techniqueText</categoryType></markingsCategory>
    <textExprDef cmper="67"><textIDKey>23</textIDKey><categoryID>5</categoryID></textExprDef>
    <measExprAssign cmper="1" inci="0">
      <textExprID>67</textExprID><staffAssign>-1</staffAssign><staffList>1</staffList>
    </measExprAssign>
  </others>
  <texts>
    <expression number="67">^fontid(2)^size(12)^nfx(1)con sordino</expression>
  </texts>
</finale>`

	_, err := Import(bytes.NewReader(fixture(t, spread)))
	var partial interface {
		Partial()
		Details() []string
	}
	if !asPartial(err, &partial) {
		t.Fatalf("the error is %T, which is not a partial read", err)
	}
	found := ""
	for _, line := range partial.Details() {
		if strings.Contains(line, "directions") {
			found = line
		}
	}
	if found == "" {
		t.Fatalf("nothing about the direction: %v", partial.Details())
	}
	if !strings.Contains(found, `"con sordino"`) {
		t.Errorf("the report does not say what it said: %s", found)
	}
	if !strings.Contains(found, "at bar 1") {
		t.Errorf("the report does not say where it was: %s", found)
	}
}

// A page made of Finale's own inserts has lost nothing, and says nothing.
//
// A title block in a Finale file is the single token ^title(); a footer is
// ^page(0). gscore fills its own from the same place Finale does, so
// reporting seven of them as unread sends somebody looking for seven things
// that are already on the page. Words somebody actually typed are another
// matter, and those are named.
func TestPageInsertsAreNotReportedAsLost(t *testing.T) {
	const pages = `<?xml version="1.0" encoding="UTF-8"?>
<finale version="27.4" xmlns="http://www.makemusic.com/2012/finale">
  <others>
    <instUsed cmper="0" inci="0"><inst>1</inst></instUsed>
    <staffSpec cmper="1"><staffLines>5</staffLines></staffSpec>
    <measSpec cmper="1"><beats>2</beats><divbeat>1536</divbeat></measSpec>
    <textBlock cmper="14"><textID>14</textID><textTag>block</textTag></textBlock>
    <textBlock cmper="90"><textID>90</textID><textTag>block</textTag></textBlock>
    <pageTextAssign cmper="1" inci="0"><block>14</block><startPage>1</startPage></pageTextAssign>
    <pageTextAssign cmper="1" inci="1"><block>90</block><startPage>1</startPage></pageTextAssign>
  </others>
  <texts>
    <blockText number="14">^fontid(2)^size(24)^nfx(0)^title()</blockText>
    <blockText number="90">^fontid(2)^size(10)^nfx(0)À la mémoire de mon père</blockText>
  </texts>
</finale>`

	_, err := Import(bytes.NewReader(fixture(t, pages)))
	var partial interface {
		Partial()
		Details() []string
	}
	if !asPartial(err, &partial) {
		t.Fatalf("the error is %T, which is not a partial read", err)
	}
	page := ""
	for _, line := range partial.Details() {
		if strings.Contains(line, "laid on the page") {
			page = line
		}
	}
	if page == "" {
		t.Fatalf("the dedication was not reported: %v", partial.Details())
	}
	if !strings.Contains(page, "À la mémoire de mon père") {
		t.Errorf("the report does not say what it said: %s", page)
	}
	if strings.Contains(page, "title") || strings.Contains(page, "(2 found)") {
		t.Errorf("the title block was reported as lost: %s", page)
	}
}
