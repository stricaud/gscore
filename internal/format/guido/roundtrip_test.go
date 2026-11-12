/*
 * roundtrip_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A format that can write a file it cannot read back is not a format.
 *
 * The comparison is made on what the music is — the pitches, the lengths,
 * the bars and the marks — and not on the text, because two spellings of
 * the same piece are the same piece. The text is compared too, but only
 * with itself: writing a score twice must give the same file both times,
 * or something in the writer depends on what it was asked first.
 */

package guido

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// write renders a score, failing the test if it will not.
func write(t *testing.T, s *model.Score) string {
	t.Helper()
	var b strings.Builder
	if err := Write(&b, s); err != nil {
		t.Fatalf("the score could not be written: %v", err)
	}
	return b.String()
}

// describe spells out everything about a score that a round trip has to
// keep, as one line per thing. Comparing the lines says what differs and
// not merely that something does.
func describe(s *model.Score) []string {
	var out []string
	add := func(f string, a ...any) {
		out = append(out, strings.TrimSpace(sprintf(f, a...)))
	}
	// On one line, because a GUIDO tag's string argument is: a composer
	// field carrying a note about the transcription under the name comes
	// back as one line, and that is the format's doing rather than a
	// thing the writer could keep.
	add("title %q composer %q",
		oneLine(s.Identity.Title), oneLine(s.Identity.Composer))
	add("tempo %d per %s %q", s.Tempo.BPM, s.Tempo.Unit, s.Tempo.Text)
	for pi, p := range s.Parts {
		add("staff %d %q clef %v key %d/%v meter %s",
			pi, p.Name, p.Clef, p.Key.Fifths, p.Key.Mode, p.Time)
		// Whether each voice's last note opened a tie, since a tie joins
		// one note to the next and a stop with nothing before it joins
		// nothing — see describeElement.
		tied := map[int]bool{}
		for mi, m := range p.Measures {
			add("staff %d bar %d closes %v", pi, mi, m.RightBarline)
			if m.Key != nil {
				add("staff %d bar %d key %d", pi, mi, m.Key.Fifths)
			}
			if m.Time != nil {
				add("staff %d bar %d meter %s", pi, mi, m.Time)
			}
			if m.Clef != nil {
				add("staff %d bar %d clef %v", pi, mi, *m.Clef)
			}
			if m.Ending != nil {
				add("staff %d bar %d volta %v start %v stop %v",
					pi, mi, m.Ending.Numbers, m.Ending.Start, m.Ending.Stop)
			}
			for vi, v := range m.Voices {
				for ei, el := range v.Elements {
					add("staff %d bar %d voice %d note %d: %s",
						pi, mi, vi, ei, describeElement(el, tied[vi]))
					mk := marksOf(el)
					tied[vi] = mk != nil && mk.TieStart
				}
			}
		}
	}
	return out
}

// describeElement spells out one element — what it is, and the marks on
// it that the program reads.
//
// Two of them it does not read, and those are left out rather than
// compared, because a round trip that kept them would be keeping
// something no page and no player ever sees.
//
// A hairpin's kind is taken from the note it opens on: see
// engrave.drawHairpin, which stores the kind when the span opens and never
// looks at it again. gscore's own ABC reader marks every hairpin's end a
// crescendo whatever opened it, so the field on a stop is not even
// self-consistent in a score read from a file.
//
// A tie's stop joins it to the note before, and a stop with no start on
// that note joins nothing: the engraver draws a tie from a note that opens
// one to the note after it, and an unanswered stop draws nothing at all.
func describeElement(el model.Element, afterTie bool) string {
	var b strings.Builder
	switch n := el.(type) {
	case *model.Note:
		b.WriteString(sprintf("%v%+d/%d for %s",
			n.Pitch.Step, n.Pitch.Alter, n.Pitch.Octave, n.Dur))
	case *model.Rest:
		b.WriteString(sprintf("rest for %s hidden=%v", n.Dur, n.Hidden))
	case *model.Chord:
		b.WriteString("chord")
		for _, m := range n.Notes {
			b.WriteString(sprintf(" %v%+d/%d", m.Pitch.Step, m.Pitch.Alter,
				m.Pitch.Octave))
		}
		b.WriteString(sprintf(" for %s", n.Dur))
	}
	mk := marksOf(el)
	if mk == nil {
		return b.String()
	}
	if mk.TieStart {
		b.WriteString(" tie-start")
	}
	if mk.TieStop && afterTie {
		b.WriteString(" tie-stop")
	}
	if mk.SlurStart > 0 {
		b.WriteString(" slur-start")
	}
	if mk.SlurStop > 0 {
		b.WriteString(" slur-stop")
	}
	if mk.Dynamic != model.DynamicNone {
		b.WriteString(" " + mk.Dynamic.String())
	}
	for _, a := range mk.Articulations {
		b.WriteString(" " + a.String())
	}
	if t := mk.Tuplet; t != nil {
		b.WriteString(sprintf(" tuplet %d:%d start=%v stop=%v",
			t.Actual, t.Normal, t.Start, t.Stop))
	}
	if h := mk.Hairpin; h != nil {
		if h.Start {
			b.WriteString(sprintf(" %v opens", h.Kind))
		}
		if h.Stop {
			b.WriteString(" hairpin closes")
		}
	}
	if g := mk.Grace; g != nil {
		b.WriteString(sprintf(" grace of %d", len(g.Notes)))
		for _, n := range g.Notes {
			b.WriteString(sprintf(" %v%+d/%d", n.Pitch.Step, n.Pitch.Alter,
				n.Pitch.Octave))
		}
	}
	return b.String()
}

// theSamples are pieces written to cover one thing each, in the format's
// own idiom rather than in ours.
var theSamples = []struct {
	name string
	src  string
}{
	{"a scale", `[ \meter<"4/4"> c1/4 d e f | g a h c2 ]`},
	{"an upbeat", `[ \meter<"3/4"> a0/8 h | c1/2 c/4 | c/2. ]`},
	{"flats and sharps", `[ \key<-3> \meter<"4/4"> c1/4 e& g c2 | h&1 a& g f ]`},
	{"a key change", `[ \key<0> \meter<"2/4"> c1/4 d | \key<2> f# g ]`},
	{"a meter change", `[ \meter<"2/4"> c1/4 d | \meter<"3/4"> e f g ]`},
	{"a clef change", `[ \clef<"g2"> \meter<"2/4"> c1/4 d | \clef<"f4"> c0 d ]`},
	{"chords", `[ \meter<"4/4"> {c1/4, e, g} {d, f, a} {e, g, h} {f, a, c2} ]`},
	{"a slur and a tie", `[ \meter<"2/4"> \sl(c1/4 d) | \tie(e/4 e) ]`},
	{"a slur over a barline",
		`[ \meter<"2/4"> c1/4 \slurBegin d | e \slurEnd f ]`},
	{"dynamics", `[ \meter<"2/4"> \intens<"p"> c1/4 d | \intens<"ff"> e f ]`},
	{"a crescendo", `[ \meter<"2/4"> \crescBegin c1/4 d | e \crescEnd f ]`},
	{"articulations",
		`[ \meter<"4/4"> \stacc(c1/4 d) \accent(e) \marcato(f) ]`},
	{"grace notes", `[ \meter<"2/4"> \grace(c1/16 d&) e/4 f ]`},
	{"a triplet", `[ \meter<"4/4"> c1*1/12 d e f/4 g a ]`},
	{"a written tuplet", `[ \meter<"4/4"> \tuplet<"5:4">(c1*1/20 d e f g) _/4 _/2 ]`},
	{"a repeat", `[ \meter<"2/4"> \repeatBegin c1/4 d e f \repeatEnd g a ]`},
	{"voltas", `[ \meter<"3/4"> \repeatBegin c1/2 g/4
	  \volta<"1.">(e/4 g/2) \repeatEnd \volta<"2.">(e&/4 f e&) ]`},
	{"a double bar", `[ \meter<"4/4"> c1/2 e&/4 e& | _/4 f a&/2 \doubleBar c/4 d e f ]`},
	{"two voices on one staff", `{
	  [ \staff<1> \meter<"2/4"> e1/8. \slur(f/16 e/8) d ],
	  [ \staff<1> \meter<"2/4"> c1/4 d ],
	  [ \staff<2> \clef<"f4"> \meter<"2/4"> a-1/4 h ] }`},
	{"a header", `[ \title<"Aubade"> \composer<"H. Hoos">
	  \tempo<"Moderato","1/4=60"> \instr<"Flauto 1"> \meter<"2/4"> e&2/8 _ c1/4 ]`},
	{"rests and a hidden one", `[ \meter<"4/4"> c1/4 _ empty/4 d ]`},
	{"dotted notes", `[ \meter<"4/4"> c1/4. d/8 e/2 | f/2.. g/8 ]`},
}

// The music written out and read back is the music.
func TestEverySampleSurvivesBeingWrittenOut(t *testing.T) {
	for _, tc := range theSamples {
		t.Run(tc.name, func(t *testing.T) {
			first := read(t, tc.src)
			text := write(t, first)
			again, err := Import(strings.NewReader(text))
			if again == nil {
				t.Fatalf("what was written could not be read: %v\n%s", err, text)
			}
			if err != nil {
				t.Errorf("what was written was not read cleanly: %v\n%s", err, text)
			}
			diff(t, describe(first), describe(again), text)
		})
	}
}

// Writing the same score twice gives the same file. Anything else means the
// writer is carrying state it should not.
func TestWritingIsSettled(t *testing.T) {
	for _, tc := range theSamples {
		t.Run(tc.name, func(t *testing.T) {
			s := read(t, tc.src)
			first := write(t, s)
			second := write(t, s)
			if first != second {
				t.Errorf("the same score came out differently the second "+
					"time:\n%s\n----\n%s", first, second)
			}
			// And the trip after that settles: read back and written again
			// it is the same text, which is what says the two directions
			// agree rather than merely each being self-consistent.
			back, _ := Import(strings.NewReader(first))
			if back == nil {
				t.Fatal("what was written could not be read")
			}
			if third := write(t, back); third != first {
				t.Errorf("the file changed on the second trip:\n%s\n----\n%s",
					first, third)
			}
		})
	}
}

func diff(t *testing.T, want, got []string, text string) {
	t.Helper()
	n := len(want)
	if len(got) < n {
		n = len(got)
	}
	bad := 0
	for i := 0; i < n; i++ {
		if want[i] != got[i] {
			t.Errorf("went out as %q and came back as %q", want[i], got[i])
			if bad++; bad > 6 {
				break
			}
		}
	}
	if len(want) != len(got) {
		t.Errorf("%d things went out and %d came back", len(want), len(got))
		for _, extra := range want[n:] {
			t.Errorf("  lost: %s", extra)
		}
		for _, extra := range got[n:] {
			t.Errorf("  gained: %s", extra)
		}
	}
	if t.Failed() {
		t.Logf("what was written:\n%s", text)
	}
}

// "It looks like GMN" and "the Guido engine reads it" are different claims.
//
// Checked with the real thing where it is installed, as the LilyPond writer
// is checked by compiling its output. guidoparse is the parser on its own
// and is what guidolib builds as a command-line tool; guido2svg engraves,
// which also exercises the tags rather than only the grammar.
func TestTheRealGuidoEngineReadsWhatIsWritten(t *testing.T) {
	tool, args := guidoTool()
	if tool == "" {
		t.Skip("no Guido tool is installed, so this cannot be checked")
	}
	dir := t.TempDir()
	for _, tc := range theSamples {
		t.Run(tc.name, func(t *testing.T) {
			s := read(t, tc.src)
			text := write(t, s)
			path := filepath.Join(dir, "x.gmn")
			if err := writeFile(path, text); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(tool, append(args, path)...).CombinedOutput()
			if err != nil {
				t.Errorf("%s refused it: %v\n%s\n----\n%s",
					filepath.Base(tool), err, out, text)
			}
		})
	}
}

// guidoTool finds whichever of the engine's command-line tools is about.
func guidoTool() (string, []string) {
	for _, name := range []string{"guidoparse", "guido2svg", "guido2midi"} {
		if path, err := exec.LookPath(name); err == nil {
			if name == "guido2svg" {
				return path, []string{"-p", "1", "-o", "/dev/null"}
			}
			return path, nil
		}
	}
	return "", nil
}

func sprintf(f string, a ...any) string { return fmt.Sprintf(f, a...) }

func writeFile(path, text string) error {
	return os.WriteFile(path, []byte(text), 0o600)
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
