/*
 * guess_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Working out what a staff is from what it is called.
 *
 * The names in these tests are not invented: they are the forty-four staff
 * names of Ravel's Boléro, as that score writes them, which arrived in
 * gscore as forty-four pianos because nothing in the file said otherwise.
 * A guesser tested against names somebody made up is a guesser tested
 * against the wrong document.
 */

package model

import "testing"

// bolero is every staff of that score, in order, with what each one is.
var bolero = []struct{ name, sound string }{
	{"Petite flute1", "wind.flutes.flute.piccolo"},
	{"Petite flute2", "wind.flutes.flute.piccolo"},
	{"Petite flute3", "wind.flutes.flute.piccolo"},
	{"Flute", "wind.flutes.flute"},
	{"Hautbois", "wind.reed.oboe"},
	{"Hautbois2", "wind.reed.oboe"},
	{"Cor anglais", "wind.reed.english-horn"},
	{"Petite clarinette", "wind.reed.clarinet.eflat"},
	{"Clarinette", "wind.reed.clarinet"},
	{"Clarinette basse", "wind.reed.clarinet.bass"},
	{"Basson", "wind.reed.bassoon"},
	{"Contrebasson", "wind.reed.contrabassoon"},
	{"Sax sopranino", "wind.reed.saxophone.sopranino"},
	{"Sax soprano", "wind.reed.saxophone.soprano"},
	{"Sax tenor", "wind.reed.saxophone.tenor"},
	{"Cor", "brass.french-horn"},
	{"Cor34", "brass.french-horn"},
	{"Petite trompette", "brass.trumpet.piccolo"},
	{"Trompette", "brass.trumpet"},
	{"Trombone23", "brass.trombone"},
	{"Tuba", "brass.tuba"},
	{"Timbales", "drum.timpani"},
	{"Tambour", "drum.snare-drum"},
	{"Tamtam", "metal.tamtam"},
	{"Cymbales", "metal.cymbal.crash"},
	{"Grosse caisse", "drum.bass-drum"},
	{"Celesta1", "keyboard.celesta"},
	{"Harpe1", "pluck.harp"},
	{"Violon1", "strings.violin"},
	{"Violon2 2", "strings.violin"},
	{"Alto", "strings.viola"},
	{"Violoncelle", "strings.cello"},
	{"Cb", "strings.contrabass"},
}

func TestEveryStaffOfTheBoleroIsNamed(t *testing.T) {
	wrong := 0
	for _, c := range bolero {
		got, ok := GuessSound(c.name)
		if !ok {
			t.Errorf("%q was read as nothing at all", c.name)
			wrong++
			continue
		}
		if got != c.sound {
			t.Errorf("%q was read as %s, want %s", c.name, got, c.sound)
			wrong++
		}
	}
	if wrong == 0 {
		t.Logf("all %d staves read off their names", len(bolero))
	}
}

func TestTheSmallOnesAreNotTheOnesTheyAreNamedAfter(t *testing.T) {
	// The whole difficulty of reading these by name: nearly every one of
	// them has a bigger instrument's name inside it.
	for _, c := range []struct{ name, sound string }{
		{"Petite flûte", "wind.flutes.flute.piccolo"},
		{"Clarinette basse", "wind.reed.clarinet.bass"},
		{"Contrebasson", "wind.reed.contrabassoon"},
		{"Contrebasse", "strings.contrabass"},
		{"Petite trompette", "brass.trumpet.piccolo"},
		{"Grosse caisse", "drum.bass-drum"},
		{"Cor anglais", "wind.reed.english-horn"},
	} {
		if got, _ := GuessSound(c.name); got != c.sound {
			t.Errorf("%q was read as %s, want %s", c.name, got, c.sound)
		}
	}
}

func TestTheNumberingIsNotPartOfTheName(t *testing.T) {
	// Which player this is, written every way a score writes it.
	for _, name := range []string{
		"Flute 1", "Flute I", "Fl.2", "Flûte 1.2", "Flutes I-II",
		"Flute1", "FLUTE", "  flute  ", "Flauto III",
	} {
		if got, _ := GuessSound(name); got != "wind.flutes.flute" {
			t.Errorf("%q was read as %q", name, got)
		}
	}
}

func TestWhatIsInBracketsIsNotTheInstrument(t *testing.T) {
	// A transposition, which is written beside the name and is not it.
	for _, name := range []string{
		"Clarinetto in Si♭/B (Si Bemolle) II",
		"Horn (E Flat) 1",
		"Cor (Mi♭) 1",
	} {
		got, ok := GuessSound(name)
		if !ok {
			t.Fatalf("%q was read as nothing", name)
		}
		if got != "wind.reed.clarinet" && got != "brass.french-horn" {
			t.Errorf("%q was read as %s", name, got)
		}
	}
}

func TestANameThatSaysNothingIsLeftAlone(t *testing.T) {
	// Rather than the nearest thing in the list. A wrong instrument
	// quietly chosen is worse than none, because nobody goes looking for
	// it: a staff still called "piano" is a staff somebody will see.
	for _, name := range []string{"", "   ", "Staff 3", "II", "???"} {
		if got, ok := GuessSound(name); ok {
			t.Errorf("%q was read as %s", name, got)
		}
	}
}

func TestTheDoubtfulNamesAreSettledByTheirNeighbours(t *testing.T) {
	// "Alto" is the viola in a French orchestral score and a singer in an
	// English choral one, and no reading of the name alone can know which.
	// The staves around it can.
	orchestra := GuessSounds([]string{
		"Violon1", "Violon2", "Alto", "Violoncelle", "Contrebasse",
	})
	if orchestra[2] != "strings.viola" {
		t.Errorf("among strings, Alto was read as %s", orchestra[2])
	}
	choir := GuessSounds([]string{"Soprano", "Alto", "Tenor", "Bass"})
	if choir[1] != "voice.alto" {
		t.Errorf("among voices, Alto was read as %s", choir[1])
	}
	// And the same question for the timpani, which in English is a pair of
	// Cuban drums.
	band := GuessSounds([]string{"Trompette", "Timbales", "Grosse caisse"})
	if band[1] != "drum.timpani" {
		t.Errorf("in an orchestra, Timbales was read as %s", band[1])
	}
}

func TestEveryGuessIsSomethingTheCatalogueHas(t *testing.T) {
	// The answer is offered in the same list somebody would have picked
	// from by hand, so it has to be a thing that is in that list.
	have := map[string]bool{}
	for _, c := range Catalogue() {
		have[c.Sound] = true
	}
	for _, c := range bolero {
		got, ok := GuessSound(c.name)
		if ok && !have[got] {
			t.Errorf("%q was read as %q, which is not in the catalogue",
				c.name, got)
		}
	}
	for _, a := range guessAliases {
		if !have[a.sound] {
			t.Errorf("the alias %q points at %q, which is not in the "+
				"catalogue", a.match, a.sound)
		}
	}
	for short, sound := range shortGuesses {
		if !have[sound] {
			t.Errorf("the abbreviation %q points at %q, which is not in "+
				"the catalogue", short, sound)
		}
	}
}

func TestTheNamesASpanishScoreWrites(t *testing.T) {
	// A concerto for orchestra written in NoteWorthy, whose thirty staves
	// arrived as pianos because its names are in Spanish and its sections
	// are abbreviated: "Vns. I", "Vlas.", "C.B.", "Fagots", "Triángulo".
	for _, c := range []struct{ name, sound string }{
		{"Flautas", "wind.flutes.flute"},
		{"Flautín", "wind.flutes.flute.piccolo"},
		{"Oboes", "wind.reed.oboe"},
		{"Clarinetes", "wind.reed.clarinet"},
		{"Fagots", "wind.reed.bassoon"},
		{"Trompas", "brass.french-horn"},
		{"Trompetas", "brass.trumpet"},
		{"Trombones", "brass.trombone"},
		{"Triángulo", "metal.triangle"},
		{"Platillos", "metal.cymbal.crash"},
		{"Caja Militar", "drum.snare-drum"},
		{"Gran Caja", "drum.bass-drum"},
		{"Violines I", "strings.violin"},
		{"Violas", "strings.viola"},
		{"Violonchelos", "strings.cello"},
		{"Contrabajo", "strings.contrabass"},
	} {
		if got, _ := GuessSound(c.name); got != c.sound {
			t.Errorf("%q was read as %q, want %s", c.name, got, c.sound)
		}
	}
}

func TestASectionIsNamedByItsAbbreviation(t *testing.T) {
	// A full score labels its staves in two or three letters and in the
	// plural, because the staff is a section and not a player: "Vns. I",
	// "Tbns.1y2", "Hns.F I". And an abbreviation written with stops
	// between its letters — "C.B." for the double basses — is one word,
	// not two of one letter.
	for _, c := range []struct{ name, sound string }{
		{"Vns. I", "strings.violin"},
		{"Vlns. II", "strings.violin"},
		{"Vlas.", "strings.viola"},
		{"Celli", "strings.cello"},
		{"C.B.", "strings.contrabass"},
		{"Tpts. en C", "brass.trumpet"},
		{"Tbns.1y2", "brass.trombone"},
		{"Hns.F  I", "brass.french-horn"},
		{"Cls. en Bb", "wind.reed.clarinet"},
		{"Fls. 1.2", "wind.flutes.flute"},
	} {
		if got, _ := GuessSound(c.name); got != c.sound {
			t.Errorf("%q was read as %q, want %s", c.name, got, c.sound)
		}
	}
}
