/*
 * accidentals_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import (
	"strings"
	"testing"
)

/*
 * One rule, asked every way round.
 *
 * Each case below is a bar or two of one staff, written as the notes that
 * sound, and what is checked is the signs that come out. The point of every
 * case is the same question: does the reader already know this?
 */

// signsFor builds a staff from a list of pitches and returns the signs.
//
// A pitch is a letter, an optional run of # or b, and an octave: "F#4". A
// bar is "|". The answer is one word per note — "sharp", "-" for no sign —
// which reads as the page does.
func signsFor(t *testing.T, key int, written string) string {
	t.Helper()
	s := staffOf(t, key, written)
	s.SpellAccidentals()
	return signsOf(s.Parts[0])
}

func staffOf(t *testing.T, key int, written string) *Score {
	t.Helper()
	s := NewScore()
	p := s.Parts[0]
	p.Key = KeySignature{Fifths: key}
	p.Measures = nil

	m := NewMeasure()
	m.Voices = []*Voice{{}}
	for _, tok := range strings.Fields(written) {
		if tok == "|" {
			p.Measures = append(p.Measures, m)
			m = NewMeasure()
			m.Voices = []*Voice{{}}
			continue
		}
		n := &Note{Pitch: writtenPitch(t, strings.TrimSuffix(tok, "~")), Dur: Duration{1, 4}}
		if strings.HasSuffix(tok, "~") {
			n.TieStart = true
		}
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	p.Measures = append(p.Measures, m)
	return s
}

func writtenPitch(t *testing.T, tok string) Pitch {
	t.Helper()
	if tok == "" {
		t.Fatal("empty pitch")
	}
	var p Pitch
	switch tok[0] {
	case 'C':
		p.Step = StepC
	case 'D':
		p.Step = StepD
	case 'E':
		p.Step = StepE
	case 'F':
		p.Step = StepF
	case 'G':
		p.Step = StepG
	case 'A':
		p.Step = StepA
	case 'B':
		p.Step = StepB
	default:
		t.Fatalf("%q does not start with a letter", tok)
	}
	rest := tok[1:]
	for len(rest) > 0 && (rest[0] == '#' || rest[0] == 'b') {
		if rest[0] == '#' {
			p.Alter++
		} else {
			p.Alter--
		}
		rest = rest[1:]
	}
	p.Octave = 4
	if len(rest) == 1 && rest[0] >= '0' && rest[0] <= '9' {
		p.Octave = int(rest[0] - '0')
	}
	return p
}

func signsOf(p *Part) string {
	var out []string
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				switch e := el.(type) {
				case *Note:
					out = append(out, sign(e))
				case *Chord:
					for _, n := range e.Notes {
						out = append(out, sign(n))
					}
				}
			}
		}
		out = append(out, "|")
	}
	return strings.Join(out[:len(out)-1], " ")
}

func sign(n *Note) string {
	if n.Accidental == NoAccidental {
		return "-"
	}
	return n.Accidental.String()
}

func TestTheRuleForAccidentals(t *testing.T) {
	for _, c := range []struct {
		what  string
		key   int
		notes string
		want  string
	}{
		{
			"a sign is printed once and then remembered",
			0, "F#4 F#4 F#4 F#4",
			"sharp - - -",
		},
		{
			"and again in the next bar, because a bar forgets",
			0, "F#4 F#4 | F#4 F#4",
			"sharp - | sharp -",
		},
		{
			"a natural takes back a sharp written earlier in the bar",
			0, "F#4 F4",
			"sharp natural",
		},
		{
			"and is not printed twice either",
			0, "F#4 F4 F4",
			"sharp natural -",
		},
		{
			"the key signature is something the reader already knows",
			1, "F#4 F#4",
			"- -",
		},
		{
			"so in that key it is the natural that has to be said",
			1, "F4 F4",
			"natural -",
		},
		{
			"and the sharp has to be said again after it",
			1, "F4 F#4",
			"natural sharp",
		},
		{
			"an accidental belongs to its own octave",
			0, "F#4 F5",
			"sharp -",
		},
		{
			"so the octave above needs telling in its turn",
			0, "F#4 F#5 F#4 F#5",
			"sharp sharp - -",
		},
		{
			"a flat where the key has a sharp is two signs, not one",
			1, "F4 Fb4",
			"natural flat",
		},
		{
			"double sharps and double flats",
			0, "F##4 F##4 Fbb4",
			"double sharp - double flat",
		},
		{
			"a different letter is a different memory",
			0, "F#4 C#4 F#4 C#4",
			"sharp sharp - -",
		},
		{
			"four sharps in a bar, which is the Finale case",
			0, "F#4 F#4 F#4 F#4 | F#4 F#4",
			"sharp - - - | sharp -",
		},
	} {
		t.Run(c.what, func(t *testing.T) {
			if got := signsFor(t, c.key, c.notes); got != c.want {
				t.Errorf("%s in %d sharps\n got  %s\n want %s",
					c.notes, c.key, got, c.want)
			}
		})
	}
}

// A tie makes the note it lands on the same note still sounding, so it is not
// announced twice — and that is the one thing in notation that reaches across
// a barline.
func TestATiedNoteIsNotAnnouncedAgain(t *testing.T) {
	s := staffOf(t, 0, "F#4~ | F#4 F#4")
	s.Parts[0].Measures[1].Voices[0].Elements[0].(*Note).TieStop = true
	s.SpellAccidentals()

	if got, want := signsOf(s.Parts[0]), "sharp | - sharp"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Unless it is written down differently on arrival. An F sharp tied to a G
// flat is one sound and two spellings, and the second has to be printed or
// the page says a G natural.
func TestATieThatRespellsIsAnnounced(t *testing.T) {
	s := staffOf(t, 0, "F#4~ | Gb4")
	s.Parts[0].Measures[1].Voices[0].Elements[0].(*Note).TieStop = true
	s.SpellAccidentals()

	if got, want := signsOf(s.Parts[0]), "sharp | flat"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// What is remembered belongs to the player, not the staff. A pianist reading
// a sharp with the left hand does not need telling again when the right hand
// plays the same note: one reader, one line of music, written on two staves.
func TestAPianistsTwoStavesShareOneMemory(t *testing.T) {
	s := NewScore()
	s.Parts = nil
	for range 2 {
		p := &Part{Key: KeySignature{}, Time: TimeSignature{Beats: 4, BeatValue: 4}}
		m := NewMeasure()
		m.Voices = []*Voice{{Elements: []Element{
			&Note{Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}, Dur: Duration{1, 4}},
		}}}
		p.Measures = []*Measure{m}
		s.Parts = append(s.Parts, p)
	}
	s.Groups = []StaffGroup{{First: 0, Last: 1, Symbol: GroupBrace, Barlines: true}}

	s.SpellAccidentals()

	if got, want := signsOf(s.Parts[0]), "sharp"; got != want {
		t.Errorf("the upper staff got %q, want %q", got, want)
	}
	if got, want := signsOf(s.Parts[1]), "-"; got != want {
		t.Errorf("the lower staff got %q, want %q — one player is told once",
			got, want)
	}
}

// A bracket says something else entirely. Two bracketed flutes are two
// people, and a sharp in one tells the other nothing.
func TestTwoBracketedStavesAreTwoReaders(t *testing.T) {
	s := NewScore()
	s.Parts = nil
	for range 2 {
		p := &Part{Key: KeySignature{}, Time: TimeSignature{Beats: 4, BeatValue: 4}}
		m := NewMeasure()
		m.Voices = []*Voice{{Elements: []Element{
			&Note{Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}, Dur: Duration{1, 4}},
		}}}
		p.Measures = []*Measure{m}
		s.Parts = append(s.Parts, p)
	}
	s.Groups = []StaffGroup{{First: 0, Last: 1, Symbol: GroupBracket}}

	s.SpellAccidentals()

	for i, p := range s.Parts {
		if got, want := signsOf(p), "sharp"; got != want {
			t.Errorf("staff %d got %q, want %q", i, got, want)
		}
	}
}

// Two voices on one staff are one reader, which falls out of their being one
// part — but it is the case somebody would get wrong first, so it is asked.
func TestTwoVoicesOnOneStaffShareOneMemory(t *testing.T) {
	s := NewScore()
	p := s.Parts[0]
	m := NewMeasure()
	m.Voices = []*Voice{
		{Elements: []Element{&Note{Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}, Dur: Duration{1, 2}}}},
		{Elements: []Element{&Note{Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}, Dur: Duration{1, 2}}}},
	}
	p.Measures = []*Measure{m}

	s.SpellAccidentals()

	if got, want := signsOf(p), "sharp -"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A sign somebody insisted on stands, and still alters the bar.
func TestAForcedAccidentalStands(t *testing.T) {
	s := staffOf(t, 1, "F#4 F#4")
	notes := s.Parts[0].Measures[0].Voices[0].Elements
	// In one sharp neither F needs a sign. Somebody wants one on the first.
	notes[0].(*Note).Accidental = Sharp
	notes[0].(*Note).Forced = true

	s.SpellAccidentals()

	if got, want := signsOf(s.Parts[0]), "sharp -"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// And a bracketed one, which is the same promise for the courtesy signs an
// editor supplies.
func TestABracketedAccidentalStands(t *testing.T) {
	s := staffOf(t, 0, "F4 F4")
	notes := s.Parts[0].Measures[0].Voices[0].Elements
	notes[0].(*Note).Accidental = Natural
	notes[0].(*Note).Parens = true

	s.SpellAccidentals()

	if got, want := signsOf(s.Parts[0]), "natural -"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The grace notes leaning on a note are read before it, and alter the bar as
// any other note does.
func TestGraceNotesAreSpelled(t *testing.T) {
	s := NewScore()
	p := s.Parts[0]
	m := NewMeasure()
	head := &Note{Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}, Dur: Duration{1, 4}}
	head.Grace = &GraceGroup{Notes: []*Note{
		{Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}, Dur: Duration{1, 16}},
	}}
	m.Voices = []*Voice{{Elements: []Element{head}}}
	p.Measures = []*Measure{m}

	s.SpellAccidentals()

	if got := sign(head.Grace.Notes[0]); got != "sharp" {
		t.Errorf("the grace note got %q, want a sharp", got)
	}
	if got := sign(head); got != "-" {
		t.Errorf("the note it leans on got %q — the grace note already said it", got)
	}
}

// A chord's notes are spelled one by one, and tell each other nothing: two
// octaves of the same letter in one chord each need their own sign.
func TestAChordIsSpelledNoteByNote(t *testing.T) {
	s := NewScore()
	p := s.Parts[0]
	m := NewMeasure()
	m.Voices = []*Voice{{Elements: []Element{&Chord{
		Dur: Duration{1, 4},
		Notes: []*Note{
			{Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}},
			{Pitch: Pitch{Step: StepF, Octave: 5, Alter: 1}},
		},
	}}}}
	p.Measures = []*Measure{m}

	s.SpellAccidentals()

	if got, want := signsOf(p), "sharp sharp"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A key change takes effect from its bar, and what the old key made familiar
// is not familiar any more.
func TestAKeyChangeResetsWhatIsKnown(t *testing.T) {
	s := staffOf(t, 1, "F#4 | F#4")
	key := KeySignature{Fifths: 0}
	s.Parts[0].Measures[1].Key = &key

	s.SpellAccidentals()

	if got, want := signsOf(s.Parts[0]), "- | sharp"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Running it twice says the same thing, which is what makes it safe to run
// after every edit without thinking about it.
func TestSpellingTwiceChangesNothing(t *testing.T) {
	s := staffOf(t, 1, "F4 F#4 F4 | F#4 Gb4 F4")
	s.SpellAccidentals()
	once := signsOf(s.Parts[0])
	s.SpellAccidentals()
	if twice := signsOf(s.Parts[0]); twice != once {
		t.Errorf("the second run says %q where the first said %q", twice, once)
	}
}

// Two kinds of staff carry notes and take no accidentals. A sharp beside a
// fret number tells a guitarist nothing they can act on, and a drum staff's
// positions name instruments, so a sign there would be a sharpened cymbal.
func TestStavesThatTakeNoAccidentals(t *testing.T) {
	for _, c := range []struct {
		what string
		set  func(*Part)
	}{
		{"tablature", func(p *Part) { p.Tab = &Tablature{} }},
		{"a drum staff", func(p *Part) { p.Clef = Percussion }},
	} {
		t.Run(c.what, func(t *testing.T) {
			s := staffOf(t, 0, "F#4 C#4")
			c.set(s.Parts[0])

			s.SpellAccidentals()

			if got, want := signsOf(s.Parts[0]), "- -"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestSpellingNothingIsSafe(t *testing.T) {
	var s *Score
	s.SpellAccidentals() // must not panic

	empty := NewScore()
	empty.Parts = nil
	empty.SpellAccidentals()
}

// An alteration no accidental can draw keeps whatever sign it had, rather
// than being rubbed out and leaving nothing to say the note is not what the
// key signature makes it.
func TestAnAlterationTooFarToDrawKeepsItsSign(t *testing.T) {
	s := staffOf(t, 0, "F4")
	n := s.Parts[0].Measures[0].Voices[0].Elements[0].(*Note)
	n.Pitch.Alter = 3
	n.Accidental = Sharp

	s.SpellAccidentals()

	if n.Accidental != Sharp {
		t.Errorf("the sign became %v, and there is no sign for three semitones",
			n.Accidental)
	}
}
