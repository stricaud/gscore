/*
 * musx_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musx

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// The obfuscation over score.dat, against a frozen answer.
//
// Four constants stand between a Finale file and the music in it, and a
// typo in any of them gives a file that unwraps into rubbish rather than
// one that fails — so they are pinned here. The bytes are the first of the
// keystream, which is what ciphering a run of zeroes leaves behind.
func TestTheCipherIsTheOneFinaleUses(t *testing.T) {
	const want = "095c5ba9a98df55fa7cfd1d04c51ddd3"

	b := make([]byte, 16)
	decipher(b)
	if got := hex.EncodeToString(b); got != want {
		t.Errorf("the keystream starts %s, want %s", got, want)
	}
}

// It is its own inverse, which is what makes a round trip possible and what
// the fixture below relies on.
func TestCipheringTwiceGivesBackTheFile(t *testing.T) {
	want := []byte("the quick brown fox jumps over the lazy dog, twice over")
	b := append([]byte(nil), want...)
	decipher(b)
	if bytes.Equal(b, want) {
		t.Fatal("ciphering changed nothing")
	}
	decipher(b)
	if !bytes.Equal(b, want) {
		t.Error("ciphering twice did not give back what went in")
	}
}

// The generator restarts every 128 KiB.
//
// Without that a file comes out as music for the first eighth of a megabyte
// and as rubbish after it, which is a much harder thing to find than a file
// that fails on its first byte — so the restart is checked rather than
// assumed.
func TestTheCipherRestartsEveryHundredAndTwentyEightKiB(t *testing.T) {
	b := make([]byte, cipherReset+16)
	decipher(b)
	if !bytes.Equal(b[:16], b[cipherReset:cipherReset+16]) {
		t.Error("the keystream does not start again at the reset point")
	}
}

// A Finale file, built here.
//
// Not a real one: a .musx from Finale is somebody's music and does not
// belong in a repository. This is the same container with the same
// obfuscation over a small score written in the same shape as the real
// thing, which is what the reader has to cope with.
func fixture(t *testing.T, enigma string) []byte {
	t.Helper()

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write([]byte(enigma)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	score := gz.Bytes()
	decipher(score) // its own inverse: this is how Finale writes it

	var out bytes.Buffer
	zp := zip.NewWriter(&out)
	add := func(name string, body []byte) {
		w, err := zp.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	add("mimetype", []byte("application/vnd.makemusic.notation"))
	add("META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="18" xmlns="http://www.makemusic.com/2012/container">
  <rootfiles>
    <rootfile full-path="score.dat" media-type="application/vnd.makemusic.notation.dat.1"/>
  </rootfiles>
</container>`))
	add("NotationMetadata.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<metadata version="18.0" xmlns="http://www.makemusic.com/2012/NotationMetadata">
  <fileInfo>
    <title>Messe Solennelle</title>
    <composer>G. Rossini</composer>
  </fileInfo>
</metadata>`))
	add("score.dat", score)
	if err := zp.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// Two staves of two bars in six eight: a dotted quarter and two eighths on
// top, a chord and a rest underneath, with the staves in score order and a
// slur somewhere that this reader does not read.
const enigmaSample = `<?xml version="1.0" encoding="UTF-8"?>
<finale version="27.4" xmlns="http://www.makemusic.com/2012/finale">
  <others>
    <instUsed cmper="0" inci="0"><inst>2</inst></instUsed>
    <instUsed cmper="0" inci="1"><inst>1</inst></instUsed>
    <staffSpec cmper="1"><staffLines>5</staffLines><fullName>10</fullName></staffSpec>
    <staffSpec cmper="2">
      <staffLines>5</staffLines>
      <fullName>8</fullName>
      <transposition><keysig><interval>1</interval><adjust>2</adjust></keysig></transposition>
    </staffSpec>
    <textBlock cmper="8"><textID>22</textID></textBlock>
    <textBlock cmper="10"><textID>24</textID></textBlock>
    <articDef cmper="5"><charMain>94</charMain><charAlt>118</charAlt></articDef>
    <measSpec cmper="1"><beats>2</beats><divbeat>1536</divbeat><barline>normal</barline></measSpec>
    <measSpec cmper="2"><beats>2</beats><divbeat>1536</divbeat><barline>final</barline></measSpec>
    <measSpec cmper="1" part="1" shared="true"><beats>9</beats><divbeat>99</divbeat></measSpec>
    <frameSpec cmper="1" inci="0"><startEntry>1</startEntry><endEntry>3</endEntry></frameSpec>
    <frameSpec cmper="2" inci="0"><startEntry>4</startEntry><endEntry>4</endEntry></frameSpec>
    <frameSpec cmper="3" inci="0"><startEntry>5</startEntry><endEntry>5</endEntry></frameSpec>
    <smartShape cmper="1"><shapeType>slur</shapeType></smartShape>
  </others>
  <details>
    <articAssign entnum="2" inci="0"><articDef>5</articDef></articAssign>
    <gfhold cmper1="2" cmper2="1"><clefID>0</clefID><frame1>1</frame1></gfhold>
    <gfhold cmper1="2" cmper2="2"><clefID>0</clefID><frame1>2</frame1></gfhold>
    <gfhold cmper1="1" cmper2="1"><clefID>3</clefID><frame1>3</frame1></gfhold>
  </details>
  <entries>
    <entry entnum="1" prev="0" next="2">
      <dura>1536</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>0</harmLev><harmAlt>0</harmAlt><tieStart/></note>
    </entry>
    <entry entnum="2" prev="1" next="3">
      <dura>512</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>7</harmLev><harmAlt>1</harmAlt></note>
    </entry>
    <entry entnum="3" prev="2" next="0">
      <dura>512</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>-1</harmLev><harmAlt>-1</harmAlt><tieStart/></note>
    </entry>
    <entry entnum="4" prev="0" next="0">
      <dura>3072</dura><numNotes>0</numNotes><isValid/><floatRest/>
    </entry>
    <entry entnum="5" prev="0" next="0">
      <dura>3072</dura><numNotes>2</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>-7</harmLev><harmAlt>0</harmAlt></note>
      <note id="2"><harmLev>-3</harmLev><harmAlt>0</harmAlt></note>
    </entry>
  </entries>
  <texts>
    <blockText number="22">^fontid(2)^size(14)^nfx(0)Clarinette en Sib</blockText>
    <blockText number="24">^fontid(2)^size(14)^nfx(0)Baritone Sax</blockText>
  </texts>
</finale>`

func read1(t *testing.T, data []byte) *model.Score {
	t.Helper()
	score, err := Import(bytes.NewReader(data))
	if score == nil {
		t.Fatalf("nothing was read: %v", err)
	}
	return score
}

func TestReadingAFinaleFile(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))

	// The title and composer are in the container, not in the music, so a
	// reader that only opened score.dat would show a piece with no name.
	if score.Identity.Title != "Messe Solennelle" {
		t.Errorf("title is %q", score.Identity.Title)
	}
	if score.Identity.Composer != "G. Rossini" {
		t.Errorf("composer is %q", score.Identity.Composer)
	}

	if len(score.Parts) != 2 {
		t.Fatalf("%d staves, want two", len(score.Parts))
	}

	// Score order is instUsed's, not the order the staves were defined in:
	// staff 2 is listed first, so it is the top staff and it is the treble.
	if score.Parts[0].Clef != model.Treble {
		t.Errorf("the top staff has the %v clef", score.Parts[0].Clef)
	}
	if score.Parts[1].Clef != model.Bass {
		t.Errorf("the lower staff has the %v clef", score.Parts[1].Clef)
	}

	// Two beats of a dotted quarter is six eight, not two of anything.
	if got := score.Parts[0].Time; got.Beats != 6 || got.BeatValue != 8 {
		t.Errorf("the meter came out %d/%d, want 6/8", got.Beats, got.BeatValue)
	}

	for i, p := range score.Parts {
		if len(p.Measures) != 2 {
			t.Fatalf("staff %d has %d bars, want two", i+1, len(p.Measures))
		}
	}
	if got := score.Parts[0].Measures[1].RightBarline; got != model.BarFinal {
		t.Errorf("the last bar closes with %v, want a final barline", got)
	}
}

func TestTheNotesAndTheirPitches(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	top := score.Parts[0].Measures[0].Voices[0].Elements

	if len(top) != 3 {
		t.Fatalf("the first bar holds %d things, want three", len(top))
	}

	// A dotted quarter: 1536 of Finale's units, of which a quarter is 1024.
	first, ok := top[0].(*model.Note)
	if !ok {
		t.Fatalf("the first thing is a %T", top[0])
	}
	if v, dots, ok := first.Dur.Written(); !ok || v != model.Quarter || dots != 1 {
		t.Errorf("the first note is %v with %d dots", v, dots)
	}
	// harmLev counts scale steps from middle C, so 0 is middle C itself.
	if first.Pitch.Step != model.StepC || first.Pitch.Octave != 4 {
		t.Errorf("the first note is %v%d, want middle C", first.Pitch.Step, first.Pitch.Octave)
	}

	// Seven steps up is the C above, and harmAlt sharpens it.
	second := top[1].(*model.Note)
	if second.Pitch.Step != model.StepC || second.Pitch.Octave != 5 || second.Pitch.Alter != 1 {
		t.Errorf("the second note is %v%d alter %d, want C sharp above middle C",
			second.Pitch.Step, second.Pitch.Octave, second.Pitch.Alter)
	}
	if second.Accidental != model.Sharp {
		t.Errorf("the sharp is not written on it: %v", second.Accidental)
	}

	// One step below middle C is the B under it, flattened — and the octave
	// has to go down with it, which is the arithmetic a modulo gets wrong.
	third := top[2].(*model.Note)
	if third.Pitch.Step != model.StepB || third.Pitch.Octave != 3 || third.Pitch.Alter != -1 {
		t.Errorf("the third note is %v%d alter %d, want B flat below middle C",
			third.Pitch.Step, third.Pitch.Octave, third.Pitch.Alter)
	}

	// An entry with no notes in it is a rest, and a bar of six eight is
	// three quarters long.
	rest, ok := score.Parts[0].Measures[1].Voices[0].Elements[0].(*model.Rest)
	if !ok {
		t.Fatalf("the second bar holds a %T, want a rest",
			score.Parts[0].Measures[1].Voices[0].Elements[0])
	}
	if v, dots, ok := rest.Dur.Written(); !ok || v != model.Half || dots != 1 {
		t.Errorf("the rest is %v with %d dots, want a dotted half", v, dots)
	}

	// Two noteheads in one entry are a chord, lowest first.
	chord, ok := score.Parts[1].Measures[0].Voices[0].Elements[0].(*model.Chord)
	if !ok {
		t.Fatalf("the lower staff holds a %T, want a chord",
			score.Parts[1].Measures[0].Voices[0].Elements[0])
	}
	if len(chord.Notes) != 2 {
		t.Fatalf("the chord has %d notes", len(chord.Notes))
	}
	if chord.Notes[0].Pitch.MIDI() >= chord.Notes[1].Pitch.MIDI() {
		t.Error("the chord's notes are not lowest first")
	}
	if chord.Notes[0].Pitch.Step != model.StepC || chord.Notes[0].Pitch.Octave != 3 {
		t.Errorf("the chord's lower note is %v%d, want the C below middle C",
			chord.Notes[0].Pitch.Step, chord.Notes[0].Pitch.Octave)
	}
}

// The copies Finale keeps for each extracted part are not more music.
//
// Every bar is in the file once for the score and once for each part, and
// taking the lot gives a piece several times longer than it is, in a meter
// nobody wrote.
func TestThePartCopiesAreNotReadAsMusic(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	if n := len(score.Parts[0].Measures); n != 2 {
		t.Errorf("%d bars, want the score's two and not the parts' copies", n)
	}
	if got := score.Parts[0].Time; got.Beats == 9 {
		t.Error("the meter came from a part copy rather than from the score")
	}
}

// What was left behind is said, rather than quietly dropped.
func TestItSaysWhatItDidNotRead(t *testing.T) {
	_, err := Import(bytes.NewReader(fixture(t, enigmaSample)))
	if err == nil {
		t.Fatal("a file with a slur in it was read with nothing to report")
	}
	var partial interface {
		Partial()
		Details() []string
	}
	if !asPartial(err, &partial) {
		t.Fatalf("the error is %T, which is not a partial read", err)
	}
	found := false
	for _, line := range partial.Details() {
		if strings.Contains(line, "slurs") {
			found = true
		}
	}
	if !found {
		t.Errorf("the slur is not mentioned: %v", partial.Details())
	}
}

func asPartial(err error, out *interface {
	Partial()
	Details() []string
}) bool {
	p, ok := err.(interface {
		Partial()
		Details() []string
	})
	if ok {
		*out = p
	}
	return ok
}

// A file that is not a Finale file says so rather than panicking.
func TestWhatIsNotAFinaleFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"not a zip", []byte("this is not a zip archive at all")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Import(bytes.NewReader(tc.in)); err == nil {
				t.Error("it was read as a score")
			}
		})
	}

	// A zip with nothing in it that a Finale file has.
	var out bytes.Buffer
	zp := zip.NewWriter(&out)
	w, _ := zp.Create("hello.txt")
	_, _ = w.Write([]byte("nothing to see"))
	_ = zp.Close()
	if _, err := Import(bytes.NewReader(out.Bytes())); err == nil {
		t.Error("a zip with no score in it was read as a score")
	} else if !strings.Contains(err.Error(), "score.dat") {
		t.Errorf("it said %q, which does not say what is missing", err)
	}
}

// The word over the staff, out of Finale's markup.
func TestTheStaffNames(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	if got := score.Parts[0].Name; got != "Clarinette en Sib" {
		t.Errorf("the top staff is called %q", got)
	}
	if got := score.Parts[1].Name; got != "Baritone Sax" {
		t.Errorf("the lower staff is called %q", got)
	}
}

// And what the name says the instrument is.
//
// Finale identifies its instruments by a code of its own, which is not in
// the file, so the name is what there is — and a saxophone that plays back
// as a piano is a worse answer than a guess. French names count: the scores
// this was written against are French.
func TestTheInstrumentIsGuessedFromTheName(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	if got := score.Parts[0].Instrument; got != model.Clarinet {
		t.Errorf("Clarinette en Sib came out as %v", got)
	}
	if got := score.Parts[1].Instrument; got != model.BaritoneSax {
		t.Errorf("Baritone Sax came out as %v", got)
	}
}

// How far a staff is written from where it sounds.
//
// Finale says it as a player's part is described — so many scale steps up,
// so many sharps added to the key — and the sharps are what give the
// semitones: each one is a fifth, and the fifths wrap round the octave a
// semitone at a time. One step and two sharps is a B flat clarinet.
func TestTheTransposition(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	got := score.Parts[0].Transpose
	want := model.TranspositionFor(model.Clarinet)
	if got != want {
		t.Errorf("the clarinet transposes %+v, want %+v", got, want)
	}
	if score.Parts[1].Transpose != (model.Transposition{}) {
		t.Errorf("the untransposed staff came out transposed: %+v", score.Parts[1].Transpose)
	}
}

// A tie is kept when the note is actually held into the next one, and taken
// away when it is not.
//
// Finale leaves a tie mark on a note whose neighbour has since changed
// pitch, and simply does not draw it. gscore draws a tie to the next note of
// that pitch wherever it is, so a stale mark came out as a curve sweeping
// across whole systems — which is what the flute part looked like.
func TestTiesThatGoNowhereAreNotDrawn(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	bar := score.Parts[0].Measures[0].Voices[0].Elements

	// Middle C tied to a C sharp an octave up: not the same note, so not a
	// tie.
	if bar[0].(*model.Note).TieStart {
		t.Error("a tie was kept to a note of a different pitch")
	}
	// And the one at the end of the bar, whose next bar holds a rest.
	if bar[2].(*model.Note).TieStart {
		t.Error("a tie was kept into a bar with no note to hold it")
	}
}

func TestATieThatLandsIsKept(t *testing.T) {
	// Two of the same note in a row, the first tied into the second.
	const tied = `<?xml version="1.0" encoding="UTF-8"?>
<finale version="27.4" xmlns="http://www.makemusic.com/2012/finale">
  <others>
    <instUsed cmper="0" inci="0"><inst>1</inst></instUsed>
    <staffSpec cmper="1"><staffLines>5</staffLines></staffSpec>
    <measSpec cmper="1"><beats>4</beats><divbeat>1024</divbeat></measSpec>
    <frameSpec cmper="1" inci="0"><startEntry>1</startEntry><endEntry>2</endEntry></frameSpec>
  </others>
  <details><gfhold cmper1="1" cmper2="1"><clefID>0</clefID><frame1>1</frame1></gfhold></details>
  <entries>
    <entry entnum="1" prev="0" next="2">
      <dura>2048</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>4</harmLev><harmAlt>0</harmAlt><tieStart/></note>
    </entry>
    <entry entnum="2" prev="1" next="0">
      <dura>2048</dura><numNotes>1</numNotes><isValid/><isNote/>
      <note id="1"><harmLev>4</harmLev><harmAlt>0</harmAlt></note>
    </entry>
  </entries>
</finale>`

	score := read1(t, fixture(t, tied))
	els := score.Parts[0].Measures[0].Voices[0].Elements
	if len(els) != 2 {
		t.Fatalf("%d notes, want two", len(els))
	}
	if !els[0].(*model.Note).TieStart {
		t.Error("the first note does not start the tie")
	}
	// The note it is held into has to say so, or nothing closes the curve.
	if !els[1].(*model.Note).TieStop {
		t.Error("the second note does not end it")
	}
}

// Finale names each articulation by the character it draws it with.
func TestArticulationsComeThrough(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	// The caret, char 94, on the second note.
	second := score.Parts[0].Measures[0].Voices[0].Elements[1].(*model.Note)
	if !second.HasArticulation(model.Marcato) {
		t.Errorf("the marcato is not on the note: %v", second.Articulations)
	}
	first := score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if len(first.Articulations) != 0 {
		t.Errorf("a note with no articulation has %v", first.Articulations)
	}
}

// A bar Finale leaves empty is a bar of rest, not a blank bar.
//
// Finale writes no entries for one and draws the rest itself; the
// documentation says completely empty measures may omit their hold
// altogether. Left empty they came out as gaps in the middle of the music.
func TestAnEmptyBarIsABarOfRest(t *testing.T) {
	score := read1(t, fixture(t, enigmaSample))
	// The lower staff has music in its first bar and nothing in its second.
	els := score.Parts[1].Measures[1].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("the empty bar holds %d things, want one rest", len(els))
	}
	rest, ok := els[0].(*model.Rest)
	if !ok {
		t.Fatalf("it holds a %T", els[0])
	}
	// Six eight: three quarters of a whole note.
	if want := (model.Duration{Num: 3, Den: 4}); rest.Dur.Cmp(want) != 0 {
		t.Errorf("the rest lasts %v, want a bar of six eight", rest.Dur)
	}
}
