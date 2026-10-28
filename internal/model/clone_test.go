package model

import (
	"reflect"
	"testing"
)

// populated builds a score using every field that can hold a pointer or a
// slice, because those are the only ones a copy can get wrong.
func populated() *Score {
	q := Duration{Num: 1, Den: 4}
	key := KeySignature{Fifths: -3, Mode: Minor}
	meter := TimeSignature{Beats: 7, BeatValue: 8}

	n := &Note{
		Pitch: Pitch{Step: StepF, Octave: 4, Alter: 1}, Dur: q,
		Accidental: Sharp, Head: HeadX, Percussion: 42,
	}
	n.Articulations = []Articulation{Staccato, Accent}
	n.Symbols = []SymbolMark{{Name: "mine", OffsetX: 1.5, OffsetY: -2}}
	n.Lyrics = []Lyric{{Text: "Glo", Verse: 1, Syllabic: SyllableBegin}}
	n.Texts = []TextMark{{Text: "rit.", Italic: true}}
	n.Tuplet = &Tuplet{Actual: 3, Normal: 2, Start: true}
	n.Hairpin = &Hairpin{Kind: Crescendo, Start: true}
	n.Octave = &OctaveShift{Amount: 1, Start: true}
	n.Beam = BeamStart
	n.Stem = StemUp
	n.Dynamic = MF
	n.TieStart = true
	n.Chord = &ChordSymbol{
		Root: "B", Quality: "7sus4", Bass: "F",
		Frame: &ChordFrame{
			Frets: []int{-1, 1, 3, 1, 3, 1}, Fingers: []int{0, 1, 3, 1, 4, 1},
			FirstFret: 1, Barre: 1, BarreFrom: 1, BarreTo: 5,
		},
	}

	ch := &Chord{Dur: q, Notes: []*Note{
		{Pitch: Pitch{Step: StepC, Octave: 4}, Dur: q},
		{Pitch: Pitch{Step: StepE, Octave: 4}, Dur: q, Accidental: Flat},
	}}
	ch.Articulations = []Articulation{Tenuto}

	rest := &Rest{Dur: q, Hidden: true}
	rest.Lyrics = []Lyric{{Text: "x", Verse: 2}}

	sym := &SymbolElement{Symbol: SymbolMark{Name: "drawn", OffsetY: 3}}

	m := &Measure{
		Voices:       []*Voice{{Elements: []Element{n, ch, rest, sym}}, {}},
		RightBarline: BarRepeatEnd,
		Key:          &key,
		Time:         &meter,
		Ending:       &Ending{Numbers: []int{1, 2}, Start: true, Stop: true},
		Marker:       "A",
		Repeat:       2,
	}

	p := &Part{
		ID: "P1", Name: "Alto Sax", Instrument: AltoSax,
		Clef: Treble, Key: key, Time: meter,
		Transpose: TranspositionFor(AltoSax),
		Hidden:    true,
		Measures:  []*Measure{m, {Voices: []*Voice{{}}}},
	}

	return &Score{
		Identity: Identity{Title: "T", Subtitle: "S", Composer: "C", Copyright: "©"},
		Tempo:    Tempo{BPM: 132, Unit: q, Text: "Moderately"},
		Parts:    []*Part{p, {Clef: Bass, Measures: []*Measure{{Voices: []*Voice{{}}}}}},
		Groups: []StaffGroup{
			{First: 0, Last: 1, Symbol: GroupBrace, Barlines: true, Name: "Piano"},
		},
	}
}

// A copy has to be equal to what it came from.
func TestCloneIsEqual(t *testing.T) {
	s := populated()
	c := s.Clone()
	if !reflect.DeepEqual(s, c) {
		t.Fatal("the copy differs from the original")
	}
}

// And it has to share nothing with it: the point of the copy is that editing
// one cannot reach the other. This walks both and fails on any pointer they
// have in common.
func TestCloneSharesNothing(t *testing.T) {
	s := populated()
	c := s.Clone()

	seen := map[uintptr]string{}
	collect(t, reflect.ValueOf(s), "score", seen, nil)

	shared := map[uintptr]string{}
	collect(t, reflect.ValueOf(c), "copy", nil, func(p uintptr, path string) {
		if where, ok := seen[p]; ok {
			shared[p] = where + " is also " + path
		}
	})
	for _, msg := range shared {
		t.Errorf("the copy shares memory with the original: %s", msg)
	}
}

// collect walks a value, recording the address of everything it points at.
func collect(t *testing.T, v reflect.Value, path string,
	into map[uintptr]string, check func(uintptr, string)) {
	t.Helper()
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return
		}
		if v.Kind() == reflect.Ptr {
			p := v.Pointer()
			if into != nil {
				if _, seen := into[p]; seen {
					return
				}
				into[p] = path
			}
			if check != nil {
				check(p, path)
			}
		}
		collect(t, v.Elem(), path, into, check)
	case reflect.Slice:
		if v.IsNil() || v.Len() == 0 {
			return
		}
		// The backing array: two slices sharing one is sharing memory.
		p := v.Pointer()
		if into != nil {
			into[p] = path + "[]"
		}
		if check != nil {
			check(p, path+"[]")
		}
		for i := 0; i < v.Len(); i++ {
			collect(t, v.Index(i), path, into, check)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			collect(t, v.Field(i), path+"."+f.Name, into, check)
		}
	}
}

// Editing the original must leave the copy alone — the thing undo relies on.
func TestEditingTheOriginalLeavesTheCopy(t *testing.T) {
	s := populated()
	before := s.Clone()

	p := s.Parts[0]
	p.Name = "changed"
	p.Hidden = false
	p.Measures[0].Marker = "Z"
	p.Measures[0].Repeat = 0
	p.Measures[0].Key.Fifths = 5
	p.Measures[0].Ending.Numbers[0] = 9
	n := p.Measures[0].Voices[0].Elements[0].(*Note)
	n.Pitch.Octave = 9
	n.Articulations[0] = Marcato
	n.Hairpin.Kind = Diminuendo
	n.Chord.Frame.Frets[0] = 7
	n.Lyrics[0].Text = "no"
	s.Groups[0].Name = "gone"
	s.Parts = s.Parts[:1]

	if !reflect.DeepEqual(before, populated()) {
		t.Error("editing the original changed the copy")
	}
}

// Undo restores a clone, and a score that forgot its scripts every time
// somebody pressed Ctrl-Z would lose the thought that made it on the first
// mistake after writing it down.
func TestACloneKeepsTheScripts(t *testing.T) {
	s := NewScore()
	s.Scripts = []Script{{Name: "row", Body: "note(\"c4\")"}}
	back := s.Clone()
	if len(back.Scripts) != 1 || back.Scripts[0].Name != "row" {
		t.Fatalf("the clone came back with %+v", back.Scripts)
	}
	// And a copy, not the same slice: editing one must not edit the other.
	back.Scripts[0].Body = "changed"
	if s.Scripts[0].Body == "changed" {
		t.Error("the clone shares its scripts with the original")
	}
}
