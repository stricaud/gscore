/*
 * guess.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strings"

// Working out what a staff is from what it is called.
//
// A score arrives without its instruments more often than not. A PDF says
// how many staves there are and nothing about who is reading them; a
// MusicXML file written by a program that did not ask gives every staff the
// piano it opened with. What the file does carry is the name down the left
// of the first system — "Hautbois", "Violoncelle", "Grosse caisse" — which
// is the one thing a human reader uses to know what they are looking at.
//
// So that is what this reads. It is a guess and says so: a name is not a
// declaration, "Alto" is a viola in a French score and a singer in an
// English one, and the answer is offered for somebody to look over rather
// than written into the score behind their back. Forty-four staves guessed
// and four corrected is a different afternoon from forty-four set by hand.

// guessAliases are the names a staff goes by, in the languages orchestral
// scores are printed in, against what gscore calls the instrument.
//
// Longest first where one name contains another: "petite flute" is a
// piccolo and not a flute, "clarinette basse" is a bass clarinet and not a
// clarinet, and a table read in order gets those right only if the longer
// name is met first.
var guessAliases = []struct {
	match string
	sound string
}{
	// Woodwind. The small ones before the ones they are named after.
	{"petite flute", "wind.flutes.flute.piccolo"},
	{"petites flutes", "wind.flutes.flute.piccolo"},
	{"flauto piccolo", "wind.flutes.flute.piccolo"},
	{"ottavino", "wind.flutes.flute.piccolo"},
	{"kleine flote", "wind.flutes.flute.piccolo"},
	{"piccolo", "wind.flutes.flute.piccolo"},
	{"flute alto", "wind.flutes.flute.alto"},
	{"grande flute", "wind.flutes.flute"},
	{"flauto", "wind.flutes.flute"},
	{"flote", "wind.flutes.flute"},
	{"flute", "wind.flutes.flute"},

	{"hautbois d'amour", "wind.reed.oboe-damore"},
	{"hautbois damour", "wind.reed.oboe-damore"},
	{"oboe d'amore", "wind.reed.oboe-damore"},
	{"cor anglais", "wind.reed.english-horn"},
	{"corno inglese", "wind.reed.english-horn"},
	{"englisch horn", "wind.reed.english-horn"},
	{"english horn", "wind.reed.english-horn"},
	{"hautbois", "wind.reed.oboe"},
	{"oboi", "wind.reed.oboe"},
	{"oboe", "wind.reed.oboe"},

	{"clarinette basse", "wind.reed.clarinet.bass"},
	{"clarinetto basso", "wind.reed.clarinet.bass"},
	{"bassklarinette", "wind.reed.clarinet.bass"},
	{"bass clarinet", "wind.reed.clarinet.bass"},
	{"petite clarinette", "wind.reed.clarinet.eflat"},
	{"clarinette", "wind.reed.clarinet"},
	{"klarinette", "wind.reed.clarinet"},
	{"clarinetto", "wind.reed.clarinet"},
	{"clarinet", "wind.reed.clarinet"},

	{"contrebasson", "wind.reed.contrabassoon"},
	{"controfagotto", "wind.reed.contrabassoon"},
	{"contrafagotto", "wind.reed.contrabassoon"},
	{"kontrafagott", "wind.reed.contrabassoon"},
	{"contrabassoon", "wind.reed.contrabassoon"},
	{"basson", "wind.reed.bassoon"},
	{"fagott", "wind.reed.bassoon"},
	{"bassoon", "wind.reed.bassoon"},

	// The saxophones, which are read by the word after the family name as
	// often as before it.
	{"sax sopranino", "wind.reed.saxophone.sopranino"},
	{"sopranino sax", "wind.reed.saxophone.sopranino"},
	{"sax soprano", "wind.reed.saxophone.soprano"},
	{"soprano sax", "wind.reed.saxophone.soprano"},
	{"sax alto", "wind.reed.saxophone.alto"},
	{"alto sax", "wind.reed.saxophone.alto"},
	{"sax tenor", "wind.reed.saxophone.tenor"},
	{"tenor sax", "wind.reed.saxophone.tenor"},
	{"sax baryton", "wind.reed.saxophone.baritone"},
	{"baritone sax", "wind.reed.saxophone.baritone"},
	{"saxophone", "wind.reed.saxophone.alto"},

	// Brass.
	{"petite trompette", "brass.trumpet.piccolo"},
	{"piccolo trumpet", "brass.trumpet.piccolo"},
	{"trompette", "brass.trumpet"},
	{"trompete", "brass.trumpet"},
	{"tromba", "brass.trumpet"},
	{"trumpet", "brass.trumpet"},
	{"cor anglais", "wind.reed.english-horn"},
	{"corno", "brass.french-horn"},
	{"cors", "brass.french-horn"},
	{"waldhorn", "brass.french-horn"},
	{"french horn", "brass.french-horn"},
	{"trombone", "brass.trombone"},
	{"posaune", "brass.trombone"},
	{"tuba", "brass.tuba"},

	// Percussion. These are what a score is most likely to be missing,
	// because a drum has no General MIDI program of its own to fall back
	// on: without a name there is nothing at all to go by.
	{"grosse caisse", "drum.bass-drum"},
	{"gran cassa", "drum.bass-drum"},
	{"bass drum", "drum.bass-drum"},
	{"caisse claire", "drum.snare-drum"},
	{"tambour militaire", "drum.snare-drum"},
	{"tamburo", "drum.snare-drum"},
	{"snare drum", "drum.snare-drum"},
	{"side drum", "drum.snare-drum"},
	{"tambour", "drum.snare-drum"},
	{"timbales", "drum.timpani"},
	{"timpani", "drum.timpani"},
	{"pauken", "drum.timpani"},
	{"tam-tam", "metal.tamtam"},
	{"tamtam", "metal.tamtam"},
	{"cymbales", "metal.cymbal.crash"},
	{"piatti", "metal.cymbal.crash"},
	{"becken", "metal.cymbal.crash"},
	{"cymbal", "metal.cymbal.crash"},
	{"triangle", "metal.triangle"},
	{"tambourin", "drum.tambourine"},
	{"tambourine", "drum.tambourine"},
	{"castagnettes", "wood.castanets"},
	{"castanets", "wood.castanets"},
	{"xylophone", "pitched-percussion.xylophone"},
	{"glockenspiel", "pitched-percussion.glockenspiel"},
	{"vibraphone", "pitched-percussion.vibraphone"},

	// Keyboards and the harp.
	{"celesta", "keyboard.celesta"},
	{"clavecin", "keyboard.harpsichord"},
	{"cembalo", "keyboard.harpsichord"},
	{"harpsichord", "keyboard.harpsichord"},
	{"orgue", "keyboard.organ"},
	{"organo", "keyboard.organ"},
	{"orgel", "keyboard.organ"},
	{"organ", "keyboard.organ"},
	{"harpe", "pluck.harp"},
	{"arpa", "pluck.harp"},
	{"harfe", "pluck.harp"},
	{"harp", "pluck.harp"},
	{"piano", "keyboard.piano"},

	// Strings. "Alto" is the viola in a French score, and the staff it
	// names sits among the strings — which is why it is read here and not
	// as a singer.
	{"violoncelle", "strings.cello"},
	{"violoncello", "strings.cello"},
	{"violoncelli", "strings.cello"},
	{"cello", "strings.cello"},
	{"contrebasse", "strings.contrabass"},
	{"contrabbasso", "strings.contrabass"},
	{"kontrabass", "strings.contrabass"},
	{"double bass", "strings.contrabass"},
	{"contrabass", "strings.contrabass"},
	{"violon", "strings.violin"},
	{"violino", "strings.violin"},
	{"violine", "strings.violin"},
	{"violin", "strings.violin"},
	{"bratsche", "strings.viola"},
	{"viola", "strings.viola"},
	{"alto", "strings.viola"},

	// Voices, which a choral score names and nothing else says.
	{"soprano", "voice.soprano"},
	{"mezzo", "voice.mezzo-soprano"},
	{"contralto", "voice.alto"},
	{"tenore", "voice.tenor"},
	{"tenor", "voice.tenor"},
	{"baryton", "voice.baritone"},
	{"baritone", "voice.baritone"},
	{"basse", "voice.bass"},
	{"bass", "voice.bass"},
	{"choeur", "voice.vocals"},
	{"choir", "voice.vocals"},
}

// shortGuesses are the abbreviations a score writes down the left of every
// system after the first, which is what a reader often ends up with as the
// staff's name.
//
// Matched whole rather than inside a longer word: "cb" is a double bass and
// "cbn" is not, and neither of them is hidden in the middle of some other
// name.
var shortGuesses = map[string]string{
	"fl": "wind.flutes.flute", "picc": "wind.flutes.flute.piccolo",
	"ob": "wind.reed.oboe", "ca": "wind.reed.english-horn",
	"cl": "wind.reed.clarinet", "bcl": "wind.reed.clarinet.bass",
	"bn": "wind.reed.bassoon", "fag": "wind.reed.bassoon",
	"cbn": "wind.reed.contrabassoon", "cfg": "wind.reed.contrabassoon",
	"hn": "brass.french-horn", "cor": "brass.french-horn",
	"tpt": "brass.trumpet", "tbn": "brass.trombone", "tba": "brass.tuba",
	"timp": "drum.timpani", "perc": "drum.snare-drum",
	"hp": "pluck.harp", "pno": "keyboard.piano", "cel": "keyboard.celesta",
	"vn": "strings.violin", "vln": "strings.violin", "vni": "strings.violin",
	"va": "strings.viola", "vla": "strings.viola",
	"vc": "strings.cello", "vlc": "strings.cello",
	"cb": "strings.contrabass", "db": "strings.contrabass",
}

// ambiguous are the names that mean one instrument in one language and
// another in the next, where only the score can say which.
//
// "Alto" is the viola in a French score and a singer in an English one.
// "Timbales" is the timpani in French and a pair of Cuban drums in English.
// Read on their own they take the orchestral reading, which is the commoner
// score; read among their neighbours — see GuessSounds — the company they
// keep decides it.
var ambiguous = map[string]struct{ orchestral, choral string }{
	"alto":     {"strings.viola", "voice.alto"},
	"altos":    {"strings.viola", "voice.alto"},
	"timbales": {"drum.timpani", "drum.timbale"},
}

// GuessSounds works out a whole score's staves at once, which is more than
// the sum of working out each of them.
//
// A name that could be two instruments is settled by what the other staves
// are: an "Alto" among violins and cellos is a viola, and an "Alto" among
// sopranos and tenors is a singer. On its own it would have to be guessed
// by which kind of score is commoner, and that is the sort of guess that is
// wrong in front of the one person it matters to.
func GuessSounds(names []string) []string {
	out := make([]string, len(names))
	// What the staves that are not in doubt are, which is what decides
	// the ones that are. A name that is itself the question cannot answer
	// it: counting "Alto" as a string is how "Soprano, Alto, Tenor, Bass"
	// came out with a viola in the middle of it.
	voices, bowed := 0, 0
	for i, n := range names {
		out[i] = guessOne(n, true)
		if _, doubtful := ambiguous[plainName(n)]; doubtful {
			continue
		}
		switch {
		case strings.HasPrefix(out[i], "voice."):
			voices++
		case strings.HasPrefix(out[i], "strings."):
			bowed++
		}
	}
	if voices == 0 || bowed > 0 {
		return out
	}
	// Singers and no strings: this is a choir, and the doubtful names are
	// read as the voices they are.
	for i, n := range names {
		if a, ok := ambiguous[plainName(n)]; ok {
			out[i] = a.choral
		}
	}
	return out
}

// GuessSound works out which instrument a staff's name is naming.
//
// The sound id it returns is one the catalogue holds, so what comes back can
// be offered in the same list somebody would have picked from by hand. A
// name that says nothing gscore recognises comes back empty rather than as
// the nearest thing: a wrong instrument quietly chosen is worse than none,
// because nobody goes looking for it.
func GuessSound(name string) (string, bool) {
	got := guessOne(name, true)
	return got, got != ""
}

func guessOne(name string, orchestral bool) string {
	plain := plainName(name)
	if plain == "" {
		return ""
	}
	// The doubtful ones before anything else, or the catalogue's own
	// English reading of them wins and the score is read in the wrong
	// language.
	if a, ok := ambiguous[plain]; ok {
		if orchestral {
			return a.orchestral
		}
		return a.choral
	}
	// What the catalogue itself calls things, first and exactly: a staff
	// called "bass clarinet" wants the bass clarinet and not the clarinet
	// that the alias table would find inside the name.
	for _, c := range Catalogue() {
		if plainName(c.Name) == plain {
			return c.Sound
		}
	}
	for _, a := range guessAliases {
		if strings.Contains(plain, a.match) {
			return a.sound
		}
	}
	for _, word := range strings.Fields(plain) {
		if sound, ok := shortGuesses[word]; ok {
			return sound
		}
	}
	// Nothing by name. The General MIDI reading is the last resort — it
	// knows names this table does not — and what it gives back is a
	// program, so the catalogue entry that plays it is the answer.
	if inst, ok := InstrumentNamed(name); ok {
		best := ""
		for _, c := range Catalogue() {
			if c.Percussion || c.Program != inst {
				continue
			}
			if best == "" || c.ScoreRank < soundRank(best) {
				best = c.Sound
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

func soundRank(sound string) int {
	for _, c := range Catalogue() {
		if c.Sound == sound {
			return c.ScoreRank
		}
	}
	return 1 << 30
}

// plainName is a staff name with everything that is not its instrument
// taken off: the case, the accents, the numbers that tell one player from
// another, and the brackets a transposition is written in.
//
// "Petite flûte 3", "Cor (Mi♭) 1 & Cor (Ut) 1" and "VLN. II" all name an
// instrument and none of them name it the way a catalogue does.
func plainName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	// An abbreviation's full stop belongs to the writing and not to the
	// name: "Fl.2" is a flute and a number, written without a space.
	s = strings.ReplaceAll(s, ".", " ")
	for from, to := range map[string]string{
		"é": "e", "è": "e", "ê": "e", "ë": "e", "û": "u", "ù": "u",
		"ô": "o", "î": "i", "ï": "i", "à": "a", "â": "a", "ç": "c",
		"œ": "oe", "ü": "u", "ä": "a", "ö": "o", "ß": "ss", "♭": "b",
		"♯": "#",
	} {
		s = strings.ReplaceAll(s, from, to)
	}
	// Anything in brackets is which one of them it is, not which
	// instrument: "Clarinetto in Si♭/B (Si Bemolle)".
	for {
		open := strings.IndexAny(s, "([")
		if open < 0 {
			break
		}
		close := strings.IndexAny(s[open:], ")]")
		if close < 0 {
			s = s[:open]
			break
		}
		s = s[:open] + " " + s[open+close+1:]
	}
	// The numbering, in every shape a score writes it: "Flute 1", "Fl.2",
	// "Violon1 2", "Cor34", "Trumpet I", "Hn. III-IV".
	var out []string
	for _, word := range strings.Fields(s) {
		word = strings.Trim(word, ".,:;-–—&/")
		if word == "" || isNumbering(word) {
			continue
		}
		word = strings.TrimRight(word, "0123456789")
		if word != "" {
			out = append(out, word)
		}
	}
	return strings.Join(out, " ")
}

// isNumbering says a word is which player this is rather than what they
// play: a figure, a roman numeral, or a run of them.
func isNumbering(word string) bool {
	has := false
	for _, r := range word {
		switch {
		case r >= '0' && r <= '9':
			has = true
		case r == 'i' || r == 'v' || r == 'x':
			has = true
		case r == '-' || r == '+' || r == '/' || r == '.' || r == '&':
		default:
			return false
		}
	}
	return has
}
