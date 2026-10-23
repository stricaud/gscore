package abc

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func stringReader(s string) io.Reader { return strings.NewReader(s) }

func formatImporterFor(name string) (format.Importer, error) { return format.ImporterFor(name) }

// The fixtures are gscore's own sample files, copied from legacy/samples/abc.
// boite-musique.abc was written out by gscore 0.0.8 itself, so parsing it is a
// direct test that the rewrite can still read what the C program produced.

func load(t *testing.T, name string) *model.Score {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()
	s, err := Import(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

func TestAllSamplesParse(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("testdata", "*.abc"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	for _, path := range entries {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			s := load(t, name)
			if len(s.Parts) == 0 {
				t.Fatal("no parts")
			}
			notes := 0
			for _, p := range s.Parts {
				for _, m := range p.Measures {
					for _, v := range m.Voices {
						notes += len(v.Elements)
					}
				}
			}
			if notes == 0 {
				t.Error("parsed no elements")
			}
		})
	}
}

// firstVoice returns the elements of a part's first measure.
func firstVoice(t *testing.T, p *model.Part) []model.Element {
	t.Helper()
	if len(p.Measures) == 0 || len(p.Measures[0].Voices) == 0 {
		t.Fatal("part has no first measure")
	}
	return p.Measures[0].Voices[0].Elements
}

func notesOf(s *model.Score) []*model.Note {
	var out []*model.Note
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, e := range v.Elements {
					switch el := e.(type) {
					case *model.Note:
						out = append(out, el)
					case *model.Chord:
						out = append(out, el.Notes...)
					}
				}
			}
		}
	}
	return out
}

func TestSunshine(t *testing.T) {
	s := load(t, "sunshine.abc")

	if want := "You Are My Sunshine"; s.Identity.Title != want {
		t.Errorf("title = %q, want %q", s.Identity.Title, want)
	}
	if len(s.Parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(s.Parts))
	}
	part := s.Parts[0]
	if part.Key.Fifths != 1 {
		t.Errorf("K:G gave fifths %d, want 1", part.Key.Fifths)
	}
	if part.Time != (model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric}) {
		t.Errorf("M:4/4 gave %v", part.Time)
	}

	// " D G A|" with L:1/4 is three quarter notes.
	first := firstVoice(t, part)
	if len(first) != 3 {
		t.Fatalf("first measure has %d elements, want 3", len(first))
	}
	wantFirst := []string{"D4", "G4", "A4"}
	for i, w := range wantFirst {
		n, ok := first[i].(*model.Note)
		if !ok {
			t.Fatalf("element %d is %T, want a note", i, first[i])
		}
		if got := n.Pitch.String(); got != w {
			t.Errorf("note %d = %s, want %s", i, got, w)
		}
		if n.Dur != (model.Duration{Num: 1, Den: 4}) {
			t.Errorf("note %d duration = %v, want 1/4", i, n.Dur)
		}
	}

	// "^A" is a written sharp, so it both prints and sounds.
	var sharps int
	for _, n := range notesOf(s) {
		if n.Accidental == model.Sharp {
			sharps++
			if n.Pitch.Step != model.StepA || n.Pitch.Alter != 1 {
				t.Errorf("the sharp landed on %s", n.Pitch)
			}
		}
	}
	if sharps != 1 {
		t.Errorf("found %d written sharps, want 1", sharps)
	}

	// Every F in this tune is unwritten but sounds sharp, from K:G.
	for _, n := range notesOf(s) {
		if n.Pitch.Step == model.StepF {
			if n.Pitch.Alter != 1 {
				t.Errorf("F in G major sounds with alter %d, want 1", n.Pitch.Alter)
			}
			if n.Accidental != model.NoAccidental {
				t.Errorf("F in G major should print no accidental, got %v", n.Accidental)
			}
		}
	}

	// "B2-|" ties across the barline.
	ties := 0
	for _, n := range notesOf(s) {
		if n.TieStart {
			ties++
		}
	}
	if ties == 0 {
		t.Error("no tie found, but the tune is full of them")
	}

	// There is one "z" rest.
	rests := 0
	for _, m := range part.Measures {
		for _, v := range m.Voices {
			for _, e := range v.Elements {
				if _, ok := e.(*model.Rest); ok {
					rests++
				}
			}
		}
	}
	if rests != 1 {
		t.Errorf("found %d rests, want 1", rests)
	}

	// The tune ends "G||".
	last := part.Measures[len(part.Measures)-1]
	if last.RightBarline != model.BarDouble {
		t.Errorf("final barline = %v, want double", last.RightBarline)
	}
}

func TestBoiteMusiqueTwoVoices(t *testing.T) {
	s := load(t, "boite-musique.abc")

	// The file is Latin-1, written by gscore 0.0.8 in 2004.
	if want := "La Boîte à musique"; s.Identity.Title != want {
		t.Errorf("title = %q, want %q", s.Identity.Title, want)
	}
	if want := "H. Berens"; s.Identity.Composer != want {
		t.Errorf("composer = %q, want %q", s.Identity.Composer, want)
	}
	if s.Identity.Subtitle == "" {
		t.Error("the second and third T: lines should become a subtitle")
	}

	// The sample is a three-voice piece: V:0, V:1 and V:2. The 0.0.9 model had
	// no concept of a voice at all, so its own sample could not round-trip.
	if len(s.Parts) != 3 {
		t.Fatalf("got %d parts, want 3 for V:0, V:1 and V:2", len(s.Parts))
	}
	for i, want := range []string{"0", "1", "2"} {
		if s.Parts[i].ID != want {
			t.Errorf("part %d has id %q, want %q", i, s.Parts[i].ID, want)
		}
		if len(s.Parts[i].Measures) == 0 {
			t.Errorf("part %q has no measures", want)
		}
	}

	// Q:116 with no L: yet in force means 116 unit note lengths per minute.
	if s.Tempo.BPM != 116 {
		t.Errorf("tempo = %d, want 116", s.Tempo.BPM)
	}

	// Both parts close with "|]".
	for _, p := range s.Parts {
		last := p.Measures[len(p.Measures)-1]
		if last.RightBarline != model.BarFinal {
			t.Errorf("part %q ends with %v, want final", p.ID, last.RightBarline)
		}
	}

	// "(e g)" opens and closes a slur, and "." marks staccato.
	var slurStarts, slurStops, staccatos int
	for _, n := range notesOf(s) {
		if n.SlurStart {
			slurStarts++
		}
		if n.SlurStop {
			slurStops++
		}
		if n.HasArticulation(model.Staccato) {
			staccatos++
		}
	}
	if slurStarts == 0 || slurStops == 0 {
		t.Errorf("slurs: %d starts, %d stops, want both non-zero", slurStarts, slurStops)
	}
	if slurStarts != slurStops {
		t.Errorf("unbalanced slurs: %d starts, %d stops", slurStarts, slurStops)
	}
	if staccatos == 0 {
		t.Error("no staccato marks found")
	}
}

func TestPitchSampleCoversTheRange(t *testing.T) {
	s := load(t, "pitch.abc")
	notes := notesOf(s)
	// The sample walks C2 up to B7 in 38 notes.
	if len(notes) != 38 {
		t.Fatalf("parsed %d notes, want 38", len(notes))
	}

	// "C,," is two octaves below middle C.
	if got, want := notes[0].Pitch.String(), "C2"; got != want {
		t.Errorf("first note = %s, want %s", got, want)
	}
	// "b''" is the top of the sample.
	if got, want := notes[len(notes)-1].Pitch.String(), "B7"; got != want {
		t.Errorf("last note = %s, want %s", got, want)
	}
	// The sample is written as a rising scale, so it tests the octave marks.
	for i := 1; i < len(notes); i++ {
		if notes[i].Pitch.MIDI() <= notes[i-1].Pitch.MIDI() {
			t.Fatalf("note %d (%s) does not rise above note %d (%s)",
				i, notes[i].Pitch, i-1, notes[i-1].Pitch)
		}
	}
}

func TestDecorationsAndAccidentalMemory(t *testing.T) {
	s := load(t, "decoration.abc")
	notes := notesOf(s)
	if len(notes) != 6 {
		t.Fatalf("parsed %d notes, want 6 for \"=G ^G _G .G uG vG\"", len(notes))
	}

	// The written accidentals come first.
	wantAcc := []model.Accidental{
		model.Natural, model.Sharp, model.Flat,
		model.NoAccidental, model.NoAccidental, model.NoAccidental,
	}
	for i, want := range wantAcc {
		if notes[i].Accidental != want {
			t.Errorf("note %d prints %v, want %v", i, notes[i].Accidental, want)
		}
	}

	// An accidental holds for the rest of the measure, so the last flat still
	// governs the three undecorated Gs that follow it even though nothing is
	// printed on them.
	wantAlter := []int{0, 1, -1, -1, -1, -1}
	for i, want := range wantAlter {
		if notes[i].Pitch.Alter != want {
			t.Errorf("note %d sounds with alter %d, want %d", i, notes[i].Pitch.Alter, want)
		}
	}

	if !notes[3].HasArticulation(model.Staccato) {
		t.Error(`".G" should be staccato`)
	}
	if !notes[4].HasArticulation(model.UpBow) {
		t.Error(`"uG" should be up-bow`)
	}
	if !notes[5].HasArticulation(model.DownBow) {
		t.Error(`"vG" should be down-bow`)
	}

	// L:1/8 makes each one an eighth.
	for i, n := range notes {
		if n.Dur != (model.Duration{Num: 1, Den: 8}) {
			t.Errorf("note %d duration = %v, want 1/8", i, n.Dur)
		}
	}
}

func TestLengthSuffixes(t *testing.T) {
	cases := []struct {
		src  string
		want model.Duration
	}{
		{"K:C\nL:1/4\nC", model.Duration{Num: 1, Den: 4}},
		{"K:C\nL:1/4\nC2", model.Duration{Num: 1, Den: 2}},
		{"K:C\nL:1/4\nC4", model.Duration{Num: 1, Den: 1}},
		{"K:C\nL:1/4\nC/", model.Duration{Num: 1, Den: 8}},
		{"K:C\nL:1/4\nC//", model.Duration{Num: 1, Den: 16}},
		{"K:C\nL:1/4\nC/2", model.Duration{Num: 1, Den: 8}},
		{"K:C\nL:1/4\nC/3", model.Duration{Num: 1, Den: 12}},
		{"K:C\nL:1/4\nC3/2", model.Duration{Num: 3, Den: 8}},
		{"K:C\nL:1/8\nC3", model.Duration{Num: 3, Den: 8}},
	}
	for _, c := range cases {
		s, err := Import(stringReader(c.src))
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		notes := notesOf(s)
		if len(notes) != 1 {
			t.Errorf("%q: parsed %d notes, want 1", c.src, len(notes))
			continue
		}
		if got := notes[0].Dur; got.Cmp(c.want) != 0 {
			t.Errorf("%q: duration = %v, want %v", c.src, got, c.want)
		}
	}
}

func TestChord(t *testing.T) {
	s, err := Import(stringReader("K:C\nL:1/4\n[CEG]2|\n"))
	if err != nil {
		t.Fatal(err)
	}
	first := firstVoice(t, s.Parts[0])
	if len(first) != 1 {
		t.Fatalf("got %d elements, want one chord", len(first))
	}
	ch, ok := first[0].(*model.Chord)
	if !ok {
		t.Fatalf("got %T, want a chord", first[0])
	}
	if len(ch.Notes) != 3 {
		t.Fatalf("chord has %d notes, want 3", len(ch.Notes))
	}
	want := []string{"C4", "E4", "G4"}
	for i, w := range want {
		if got := ch.Notes[i].Pitch.String(); got != w {
			t.Errorf("chord note %d = %s, want %s", i, got, w)
		}
	}
	// The outer length governs the whole chord.
	if ch.Dur != (model.Duration{Num: 1, Den: 2}) {
		t.Errorf("chord duration = %v, want 1/2", ch.Dur)
	}
	for i, n := range ch.Notes {
		if n.Dur != ch.Dur {
			t.Errorf("chord note %d duration = %v, want %v", i, n.Dur, ch.Dur)
		}
	}
}

func TestChordInnerTie(t *testing.T) {
	// "[C-E-G-]" ties each notehead individually - note the tie after G too,
	// since each note carries its own. The grammar allows it:
	// stem ::= note / ( "[" 2*note "]" ) and note ::= pitch [note-length] [tie].
	s, err := Import(stringReader("K:C\nL:1/4\n[C-E-G-]2[CEG]2|\n"))
	if err != nil {
		t.Fatal(err)
	}
	first := firstVoice(t, s.Parts[0])
	if len(first) != 2 {
		t.Fatalf("got %d elements, want two chords", len(first))
	}
	ch, ok := first[0].(*model.Chord)
	if !ok {
		t.Fatalf("got %T, want a chord", first[0])
	}
	if len(ch.Notes) != 3 {
		t.Fatalf("chord has %d notes, want 3", len(ch.Notes))
	}
	for i, n := range ch.Notes {
		if !n.TieStart {
			t.Errorf("chord note %d does not start a tie", i)
		}
	}
	if !ch.TieStart {
		t.Error("every note is tied, so the chord as a whole should be too")
	}
	next, ok := first[1].(*model.Chord)
	if !ok {
		t.Fatalf("got %T, want a chord", first[1])
	}
	if !next.TieStop {
		t.Error("the chord after the tie should stop it")
	}
}

func TestChordPartialInnerTie(t *testing.T) {
	// Only one notehead is tied, so the chord as a whole is not.
	s, err := Import(stringReader("K:C\nL:1/4\n[C-EG]2[CEG]2|\n"))
	if err != nil {
		t.Fatal(err)
	}
	ch, ok := firstVoice(t, s.Parts[0])[0].(*model.Chord)
	if !ok {
		t.Fatal("want a chord")
	}
	want := []bool{true, false, false}
	for i, w := range want {
		if ch.Notes[i].TieStart != w {
			t.Errorf("chord note %d TieStart = %v, want %v", i, ch.Notes[i].TieStart, w)
		}
	}
	if ch.TieStart {
		t.Error("only one note is tied, so the chord should not be")
	}
}

func TestModesAndKeys(t *testing.T) {
	cases := []struct {
		spec   string
		fifths int
		mode   model.Mode
	}{
		{"C", 0, model.Major},
		{"G", 1, model.Major},
		{"F", -1, model.Major},
		{"Am", -3 + 3, model.Minor}, // A minor shares C major's signature
		{"Em", 1, model.Minor},
		{"Bb", -2, model.Major},
		{"F#m", 6 - 3, model.Minor},
		{"Ddor", 2 - 2, model.Dorian},
		{"Gmix", 1 - 1, model.Mixolydian},
		{"Clyd", 0 + 1, model.Lydian},
	}
	for _, c := range cases {
		k, err := parseKeySpec(c.spec)
		if err != nil {
			t.Errorf("K:%s: %v", c.spec, err)
			continue
		}
		if k.Fifths != c.fifths {
			t.Errorf("K:%s fifths = %d, want %d", c.spec, k.Fifths, c.fifths)
		}
		if k.Mode != c.mode {
			t.Errorf("K:%s mode = %v, want %v", c.spec, k.Mode, c.mode)
		}
	}
}

func TestUnsupportedConstructsAreReportedNotIgnored(t *testing.T) {
	cases := []string{
		"K:C\n{ab}c\n",         // grace notes
		"K:C\nabc&def\n",       // voice overlay
		"K:C\n!unterminated\n", // decoration with no closing !
	}
	for _, src := range cases {
		if _, err := Import(stringReader(src)); err == nil {
			t.Errorf("%q parsed without error, but is not supported yet", src)
		}
	}
}

func TestBrokenRhythm(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	cases := []struct {
		src           string
		first, second model.Duration
	}{
		// a>b dots the first and halves the second.
		{"K:C\nL:1/4\na>b\n", q.Scale(3, 2), q.Scale(1, 2)},
		// a<b is the same the other way round.
		{"K:C\nL:1/4\na<b\n", q.Scale(1, 2), q.Scale(3, 2)},
		// Each extra sign halves again: seven eighths against one.
		{"K:C\nL:1/4\na>>b\n", q.Scale(7, 4), q.Scale(1, 4)},
		// The sign applies through whitespace and to written lengths.
		{"K:C\nL:1/4\na2> b2\n", q.Scale(2, 1).Scale(3, 2), q.Scale(2, 1).Scale(1, 2)},
	}
	for _, c := range cases {
		s, err := Import(stringReader(c.src))
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		notes := notesOf(s)
		if len(notes) != 2 {
			t.Errorf("%q: got %d notes, want 2", c.src, len(notes))
			continue
		}
		if notes[0].Dur.Cmp(c.first) != 0 || notes[1].Dur.Cmp(c.second) != 0 {
			t.Errorf("%q: durations %v and %v, want %v and %v",
				c.src, notes[0].Dur, notes[1].Dur, c.first, c.second)
		}
	}

	// The total is conserved, which is what makes the measure still add up.
	s, err := Import(stringReader("K:C\nL:1/4\na>b\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes := notesOf(s)
	if got := notes[0].Dur.Add(notes[1].Dur); got.Cmp(model.Duration{Num: 1, Den: 2}) != 0 {
		t.Errorf("a>b totals %v, want 1/2", got)
	}
}

func TestDecorations(t *testing.T) {
	// !>! is by far the most common decoration in real scores.
	s, err := Import(stringReader("K:C\nL:1/4\n!>!a !^!b !fermata!c !ff!d !nosuchmark!e\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes := notesOf(s)
	if len(notes) != 5 {
		t.Fatalf("got %d notes, want 5", len(notes))
	}
	if !notes[0].HasArticulation(model.Accent) {
		t.Error("!>! should be an accent")
	}
	if !notes[1].HasArticulation(model.Marcato) {
		t.Error("!^! should be marcato")
	}
	if !notes[2].HasArticulation(model.Fermata) {
		t.Error("!fermata! should be a fermata")
	}
	// A dynamic is part of the music now, not a loose symbol.
	if notes[3].Dynamic != model.FF {
		t.Errorf("!ff! became %v, want ff", notes[3].Dynamic)
	}
	if len(notes[3].Symbols) != 0 {
		t.Errorf("!ff! also left a symbol behind: %v", notes[3].Symbols)
	}
	// An unknown decoration is kept under its own name rather than dropped,
	// so the user can draw it.
	if len(notes[4].Symbols) != 1 || notes[4].Symbols[0].Name != "nosuchmark" {
		t.Errorf("!nosuchmark! became %v, want a symbol of that name", notes[4].Symbols)
	}
}

func TestTuneBook(t *testing.T) {
	src := "%%begintext\nprose that is not music\n%%endtext\n" +
		"X:1\nT:First\nK:C\nL:1/4\nCDEF|\n\n" +
		"X:2\nT:Second\nK:G\nL:1/4\nGABc|\n"
	scores, err := ImportAll(stringReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 2 {
		t.Fatalf("got %d tunes, want 2", len(scores))
	}
	if scores[0].Identity.Title != "First" || scores[1].Identity.Title != "Second" {
		t.Errorf("titles = %q, %q", scores[0].Identity.Title, scores[1].Identity.Title)
	}
	if scores[1].Parts[0].Key.Fifths != 1 {
		t.Errorf("second tune key = %d fifths, want 1", scores[1].Parts[0].Key.Fifths)
	}
	// Import alone returns the first.
	one, err := Import(stringReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if one.Identity.Title != "First" {
		t.Errorf("Import returned %q, want the first tune", one.Identity.Title)
	}
}

func TestKeysAcrossTheCircle(t *testing.T) {
	// Every key the flute corpus uses, including G sharp minor, which needs
	// the circle to run past the seven-sharp table.
	cases := map[string]int{
		"C": 0, "G": 1, "D": 2, "A": 3, "E": 4, "B": 5, "F#": 6,
		"F": -1, "Bb": -2, "Eb": -3, "Ab": -4, "Db": -5, "Gb": -6, "Cb": -7,
		"Am": 0, "Em": 1, "Bm": 2, "F#m": 3, "C#m": 4, "G#m": 5,
		"Dm": -1, "Gm": -2, "Cm": -3, "Fm": -4, "Bbm": -5, "Ebm": -6,
	}
	for spec, want := range cases {
		k, err := parseKeySpec(spec)
		if err != nil {
			t.Errorf("K:%s: %v", spec, err)
			continue
		}
		if k.Fifths != want {
			t.Errorf("K:%s = %d fifths, want %d", spec, k.Fifths, want)
		}
	}
}

func TestTuplets(t *testing.T) {
	// A triplet eighth sounds for a twelfth of a whole note but is written as
	// an eighth; both have to survive the import.
	s, err := Import(stringReader("K:C\nL:1/8\n(3cde f2|\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes := notesOf(s)
	if len(notes) != 4 {
		t.Fatalf("got %d notes, want 4", len(notes))
	}

	twelfth := model.Duration{Num: 1, Den: 12}
	for i := 0; i < 3; i++ {
		if notes[i].Dur.Cmp(twelfth) != 0 {
			t.Errorf("triplet note %d sounds for %v, want 1/12", i, notes[i].Dur)
		}
		if notes[i].Tuplet == nil {
			t.Fatalf("triplet note %d carries no tuplet", i)
		}
		if notes[i].Tuplet.Actual != 3 || notes[i].Tuplet.Normal != 2 {
			t.Errorf("note %d ratio = %d:%d, want 3:2",
				i, notes[i].Tuplet.Actual, notes[i].Tuplet.Normal)
		}
		// The written value is what the notehead shows.
		w := notes[i].WrittenDuration(notes[i].Dur)
		if w.Cmp(model.Duration{Num: 1, Den: 8}) != 0 {
			t.Errorf("note %d is written as %v, want 1/8", i, w)
		}
	}
	if !notes[0].Tuplet.Start {
		t.Error("the first note should open the group")
	}
	if !notes[2].Tuplet.Stop {
		t.Error("the third note should close the group")
	}
	// The note after the group is ordinary again.
	if notes[3].Tuplet != nil {
		t.Error("the note after the tuplet is still marked")
	}
	// Three in the time of two: the group lasts exactly a quarter.
	total := notes[0].Dur.Add(notes[1].Dur).Add(notes[2].Dur)
	if total.Cmp(model.Duration{Num: 1, Den: 4}) != 0 {
		t.Errorf("the triplet totals %v, want 1/4", total)
	}
}

func TestTupletRatios(t *testing.T) {
	cases := []struct {
		src            string
		actual, normal int
	}{
		{"K:C\nL:1/8\n(3cde\n", 3, 2},
		{"K:C\nL:1/8\n(5cdefg\n", 5, 2},
		{"K:C\nL:1/8\n(2cd\n", 2, 3},
		{"K:C\nL:1/8\n(7:2:7cdefgab\n", 7, 2},
	}
	for _, c := range cases {
		s, err := Import(stringReader(c.src))
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		n := notesOf(s)[0]
		if n.Tuplet == nil || n.Tuplet.Actual != c.actual || n.Tuplet.Normal != c.normal {
			t.Errorf("%q: ratio = %v, want %d:%d", c.src, n.Tuplet, c.actual, c.normal)
		}
	}
}

func TestImporterIsRegistered(t *testing.T) {
	// The format registry replaces the dlopen plugin loader; registration
	// happens in this package's init.
	i, err := formatImporterFor("tune.abc")
	if err != nil {
		t.Fatalf("no importer for .abc: %v", err)
	}
	if i.Name() != "ABC" {
		t.Errorf("importer name = %q, want ABC", i.Name())
	}
}

func TestDynamics(t *testing.T) {
	s, err := Import(stringReader(
		"K:C\nL:1/4\n!pppp!C !ppp!D !pp!E !p!F|!mp!G !mf!A !f!B !ff!c|!fff!d !ffff!e !sfz!f !fp!g|\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes := notesOf(s)
	want := []model.Dynamic{
		model.PPPP, model.PPP, model.PP, model.Piano,
		model.MP, model.MF, model.Forte, model.FF,
		model.FFF, model.FFFF, model.SForzando, model.FortePiano,
	}
	if len(notes) != len(want) {
		t.Fatalf("got %d notes, want %d", len(notes), len(want))
	}
	for i, w := range want {
		if notes[i].Dynamic != w {
			t.Errorf("note %d is %v, want %v", i, notes[i].Dynamic, w)
		}
	}

	// Each marking belongs to its own note; none bleeds onto the next.
	s, err = Import(stringReader("K:C\nL:1/4\n!p!C D E F|\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes = notesOf(s)
	if notes[0].Dynamic != model.Piano {
		t.Errorf("the first note is %v, want p", notes[0].Dynamic)
	}
	for i := 1; i < len(notes); i++ {
		if notes[i].Dynamic != model.DynamicNone {
			t.Errorf("note %d picked up %v from the note before it", i, notes[i].Dynamic)
		}
	}
}

func TestStemDirection(t *testing.T) {
	// A decoration sets one note; an I: instruction sets everything after it.
	s, err := Import(stringReader("K:C\nL:1/4\n!stemdown!C D [I:stemdir up]E F|\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes := notesOf(s)
	want := []model.Stem{model.StemDown, model.StemAuto, model.StemUp, model.StemUp}
	for i, w := range want {
		if notes[i].Stem != w {
			t.Errorf("note %d stem = %v, want %v", i, notes[i].Stem, w)
		}
	}

	// Turning it back off leaves the engraver to decide again.
	s, err = Import(stringReader("K:C\nL:1/4\n[I:stemdir up]C [I:stemdir auto]D|\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes = notesOf(s)
	if notes[0].Stem != model.StemUp {
		t.Errorf("the first note is %v, want up", notes[0].Stem)
	}
	if notes[1].Stem != model.StemAuto {
		t.Errorf("the second note is %v, want auto", notes[1].Stem)
	}
}
