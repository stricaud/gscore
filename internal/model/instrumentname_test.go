/*
 * instrumentname_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "testing"

// Reading the instrument off the name written over the staff.
//
// The names are matched as substrings and the first match wins, so the order
// of the list is most of its behaviour and every case below is really a
// question about order. Two of them were wrong for as long as the list
// existed, and both are in Beethoven's ninth.
func TestTheInstrumentWrittenOverTheStaff(t *testing.T) {
	for _, c := range []struct {
		name string
		want Instrument
	}{
		// The one this was all for: a part called Voice, whose MIDI program
		// says oboe because that is what a program is for.
		{"Voice", VoiceOohs},
		{"Piano", AcousticGrandPiano},

		// Every cello and bass has "violon" inside its name, so these have
		// to be tried before the violins. Matched the other way a cello came
		// out a violin.
		{"Violoncelli", Cello},
		{"Violoncelle", Cello},
		{"Violoncello", Cello},
		{"Contrabbassi", Contrabass},
		{"Contrebasse", Contrabass},
		{"Violini I", Violin},
		{"Violon", Violin},
		{"Viole", Viola},

		// And "contrabassoon" has "contrabass" inside it, so the bassoons go
		// before the basses.
		{"Contrabassoon", Bassoon},
		{"Controfagotto", Bassoon},
		{"Fagotto", Bassoon},

		// A soprano saxophone is not a soprano, and a chorus is not a horn
		// although "cor" is inside "coro".
		{"Soprano sax", SopranoSax},
		{"Soprano solo", VoiceOohs},
		{"Coro", ChoirAahs},
		{"Corno 3 in B basso", FrenchHorn},

		// The word that names the instrument need not be the first one.
		{"Trombone alto", Trombone},
		{"Flauto piccolo", Piccolo},
		{"Clarinetto 1 in C", Clarinet},
	} {
		got, ok := InstrumentNamed(c.name)
		if !ok {
			t.Errorf("%q named nothing; expected %v", c.name, c.want)
			continue
		}
		if got != c.want {
			t.Errorf("%q named %v, expected %v", c.name, got, c.want)
		}
	}
}

// What the name does not settle is left to whatever else the file says.
//
// "Alto" is the viola in an Italian score and a singer in a vocal one, and a
// confident wrong answer is worse than none: a reader that knows which kind
// of score it has can decide, and the MusicXML reader falls back to the MIDI
// program. The Finale reader does answer it, because its scores are
// instrumental — see enigma.instrumentNamed.
func TestAnAmbiguousNameIsLeftAlone(t *testing.T) {
	for _, name := range []string{"Alto", "Tenore", "Basso", "", "Stimme 1"} {
		if got, ok := InstrumentNamed(name); ok {
			t.Errorf("%q was read as %v; it is not a name this can settle",
				name, got)
		}
	}
}

// The voices land in the family a voice belongs to, which is the whole point
// of reading the name: it is the family that decides how a note is sounded.
func TestAVoiceIsSoundedAsAVoice(t *testing.T) {
	for _, name := range []string{"Voice", "Soprano solo", "Coro", "Gesang"} {
		inst, ok := InstrumentNamed(name)
		if !ok {
			t.Errorf("%q named nothing", name)
			continue
		}
		if f := inst.Family(); f != FamilySustained {
			t.Errorf("%q is %v, in family %v; a voice is %v",
				name, inst, f, FamilySustained)
		}
	}
}

// A desk number is not part of the instrument's name.
//
// Scores number their players, and the number says which stand it is and
// never what is played at it. Matched whole, "Guitar 1" is in no list of
// instruments there is: an octet of guitars came through as eight pianos,
// which is the sound of nothing having been recognised at all. Most names
// escaped it only because they are matched as substrings — "oboe" is inside
// "Oboe 1 & 2" — so the ones it bit were those that have to match whole.
func TestAPlayerNumberIsNotPartOfTheName(t *testing.T) {
	for _, c := range []struct {
		name string
		want Instrument
	}{
		{"Guitar 1", AcousticGuitar},
		{"guitar 8", AcousticGuitar},
		{"Violin II", Violin},
		{"Violin I", Violin},
		{"Oboe 1 & 2", Oboe},
		{"Trumpet 1 & 2", Trumpet},
	} {
		got, ok := InstrumentNamed(c.name)
		if !ok || got != c.want {
			t.Errorf("%q is %v (known %v), want %v", c.name, got, ok, c.want)
		}
	}
}

// And only a trailing word that is wholly a number is dropped, so the
// stripping stops at the first word that could be part of a name.
func TestAWordThatIsNotANumberIsKept(t *testing.T) {
	// "TC" is a treble-clef piano staff, not a piano numbered TC.
	if got, ok := InstrumentNamed("Piano TC"); !ok || got != AcousticGrandPiano {
		t.Errorf("Piano TC is %v (known %v), want the piano", got, ok)
	}
	// Nothing is left to match on once the numbers are gone.
	for _, name := range []string{"1", "II", "1 & 2"} {
		if got, ok := InstrumentNamed(name); ok {
			t.Errorf("%q was read as %v; a number names no instrument", name, got)
		}
	}
}
