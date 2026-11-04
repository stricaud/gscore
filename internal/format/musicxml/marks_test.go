/*
 * marks_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Everything in a file that is not a notehead.
 *
 * TestEveryNoteheadArrives counts the notes, and for a long time that was
 * the only measure of the importer — so a file could arrive with every note
 * in the right place and nothing written over any of them, and pass. It is
 * what happened: hairpins, trills, words, lyrics, chord symbols, octave
 * shifts, pedalling, arpeggios, glissandi and tremolos were all read as
 * nothing at all, across 182 files written to exercise exactly those, and
 * every test in this package was green.
 *
 * So: count what the file asks for, count what came back, and say where
 * they differ. The list below is the honest state of it and only shrinks.
 */

package musicxml

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// a feature is one thing a file can say and the importer can read: how to
// count it in the text of the file, and how to count it in the music that
// came back.
type feature struct {
	name string
	want func([]byte) int       // in the file
	got  func(*model.Score) int // in the score
}

// perNote counts the <note> elements that match, rather than the matches.
//
// A chord is several <note> elements and one event, and a mark that
// belongs to the whole chord may be written on any of them or on all: the
// spread of an arpeggio is written on each notehead in turn. Counting the
// marks would ask the importer for three arpeggios where a reader sees
// one. So the question asked is how many events the file marks, which is
// the number that has to come back.
func perNote(pat string) func([]byte) int {
	re := regexp.MustCompile(pat)
	return func(b []byte) int {
		n := 0
		marked := false
		for _, chunk := range noteSplit.Split(string(b), -1)[1:] {
			// A note that sounds with the one before it is part of the
			// same event.
			if chordTag.MatchString(chunk) {
				marked = marked || re.MatchString(chunk)
				continue
			}
			if marked {
				n++
			}
			marked = re.MatchString(chunk)
		}
		if marked {
			n++
		}
		return n
	}
}

var (
	noteSplit = regexp.MustCompile(`<note[ >]`)
	chordTag  = regexp.MustCompile(`<chord\s*/>`)
)

// without counts matches outside a part of the file that is not music.
func without(drop, pat string) func([]byte) int {
	cut := regexp.MustCompile(`(?s)` + drop)
	re := regexp.MustCompile(pat)
	return func(b []byte) int {
		return len(re.FindAll(cut.ReplaceAll(b, nil), -1))
	}
}

// marksIn is every mark of every element, in order.
func marksIn(s *model.Score) []*model.Marks {
	var out []*model.Marks
	for _, p := range s.Parts {
		for _, me := range p.Measures {
			for _, v := range me.Voices {
				for _, el := range v.Elements {
					if mk := marksOfElement(el); mk != nil {
						out = append(out, mk)
						// A grace note hangs off the note it leans on and
						// carries marks of its own.
						if mk.Grace != nil {
							for _, g := range mk.Grace.Notes {
								out = append(out, &g.Marks)
							}
						}
					}
				}
			}
		}
	}
	return out
}

// tally counts the elements whose marks satisfy a test.
func tally(test func(*model.Marks) bool) func(*model.Score) int {
	return func(s *model.Score) int {
		n := 0
		for _, mk := range marksIn(s) {
			if test(mk) {
				n++
			}
		}
		return n
	}
}

// rx counts every match in the file.
func rx(pat string) func([]byte) int {
	re := regexp.MustCompile(pat)
	return func(b []byte) int { return len(re.FindAll(b, -1)) }
}

// theFeatures is what is looked for, and where. The patterns match the
// text of the file rather than its parsed form on purpose: what is being
// asked is "the file says this — did any of it arrive", and a count taken
// from the same structs the importer reads would agree with itself even
// when both were wrong.
var theFeatures = []feature{{
	name: "hairpins",
	want: rx(`<wedge[^>]*type="(crescendo|diminuendo)"`),
	got:  tally(func(m *model.Marks) bool { return m.Hairpin != nil && m.Hairpin.Start }),
}, {
	name: "trills",
	want: perNote(`<trill-mark|<wavy-line[^>]*type="start"`),
	got:  tally(func(m *model.Marks) bool { return m.Trill != nil && m.Trill.Start }),
}, {
	name: "words",
	// An empty <words/> is a mark with nothing in it and is drawn as
	// nothing, so only the ones with something to say are counted.
	want: rx(`<words[^>]*>[^<\s][^<]*</words>`),
	got: func(s *model.Score) int {
		n := 0
		for _, mk := range marksIn(s) {
			n += len(mk.Texts)
		}
		// A word that set the speed is a tempo and is kept as one.
		for _, p := range s.Parts {
			for _, me := range p.Measures {
				if me.Tempo != nil && strings.TrimSpace(me.Tempo.Text) != "" {
					n++
				}
			}
		}
		if strings.TrimSpace(s.Tempo.Text) != "" {
			n++
		}
		return n
	},
}, {
	name: "lyrics",
	want: rx(`<lyric[ >]`),
	got: func(s *model.Score) int {
		n := 0
		for _, mk := range marksIn(s) {
			n += len(mk.Lyrics)
		}
		return n
	},
}, {
	name: "chord symbols",
	want: rx(`<harmony`),
	got:  tally(func(m *model.Marks) bool { return m.Chord != nil }),
}, {
	name: "octave shifts",
	want: rx(`<octave-shift[^>]*type="(up|down)"`),
	got:  tally(func(m *model.Marks) bool { return m.Octave != nil && m.Octave.Start }),
}, {
	name: "pedalling",
	want: rx(`<pedal[^>]*type="start"`),
	got:  tally(func(m *model.Marks) bool { return m.Pedal != nil && m.Pedal.Start }),
}, {
	name: "arpeggios",
	want: perNote(`<arpeggiate|<non-arpeggiate`),
	got:  tally(func(m *model.Marks) bool { return m.Arpeggio != nil }),
}, {
	name: "glissandi",
	want: perNote(`<(glissando|slide)[^>]*type="start"`),
	got:  tally(func(m *model.Marks) bool { return m.Glissando != nil && m.Glissando.Start }),
}, {
	name: "tremolos",
	want: perNote(`<tremolo`),
	got:  tally(func(m *model.Marks) bool { return m.Tremolo != nil }),
}, {
	name: "fingering",
	// Not the numbers in a fret diagram: those are part of the picture of
	// a chord shape, drawn above the staff, and say which finger goes on
	// which string rather than anything about a note.
	want: without(`<frame>.*?</frame>`, `<fingering`),
	got: func(s *model.Score) int {
		n := 0
		for _, mk := range marksIn(s) {
			n += len(mk.Fingering)
		}
		return n
	},
}, {
	name: "breath marks",
	want: perNote(`<breath-mark|<caesura`),
	got:  tally(func(m *model.Marks) bool { return m.Break != model.NoBreath }),
}, {
	name: "turns and mordents",
	want: perNote(`<turn|<inverted-turn|<mordent|<inverted-mordent`),
	got: tally(func(m *model.Marks) bool {
		for _, a := range m.Articulations {
			switch a {
			case model.Turn, model.TurnInverted, model.Mordent, model.ShortTrill:
				return true
			}
		}
		return false
	}),
}, {
	name: "key changes",
	want: rx(`<key[ >]`),
	got: func(s *model.Score) int {
		// One per staff for the signature the part opens in, and one for
		// every change after it.
		n := 0
		for _, p := range s.Parts {
			n++
			for _, me := range p.Measures {
				if me.Key != nil {
					n++
				}
			}
		}
		return n
	},
}, {
	name: "meter changes",
	want: rx(`<time[ >]`),
	got: func(s *model.Score) int {
		n := 0
		for _, p := range s.Parts {
			n++
			for _, me := range p.Measures {
				if me.Time != nil {
					n++
				}
			}
		}
		return n
	},
}, {
	name: "clef changes",
	want: rx(`<clef[ >]`),
	got: func(s *model.Score) int {
		n := 0
		for _, p := range s.Parts {
			n++
			for _, me := range p.Measures {
				if me.Clef != nil {
					n++
				}
			}
		}
		return n
	},
}}

// shortOf is every file some of whose marks still do not arrive, and how
// many of each do. A file that drops below its number has regressed; one
// that rises above it should have its entry raised or taken away.
//
// What is left here is of three kinds, and none of it is a mark that was
// read as nothing:
//
//   - the same thing said twice. A file states the key, meter or clef again
//     at the head of a system or the start of a movement, and gscore writes
//     a change only where something changed. 13f-KeySignatures-Visible
//     states eight signatures and four of them are the key it was already
//     in.
//   - a sign gscore draws differently. A tuplet whose figure is hidden, a
//     wavy line continued across a barline, a second verse of lyrics on a
//     note that has none.
//   - a part gscore has nowhere for. Figured bass, fret diagrams.
var shortOf = map[string]map[string]int{
	// A signature gscore has nowhere to put a second of.
	//
	// The model holds one key, one meter and one clef per bar, because
	// that is where they are drawn: at the barline, in front of the music
	// they govern. A file may change any of them partway through a bar —
	// the suite has files that change key four times inside one — and
	// gscore keeps the last, which is the one the rest of the bar is
	// read against. The notes are all there and in the right place; what
	// is missing is a sign drawn between two of them.
	"11h-TimeSignatures-SenzaMisura.musicxml":      {"meter changes": 1},
	"13b-KeySignatures-ChurchModes.musicxml":       {"key changes": 3},
	"13c-KeySignatures-NonTraditional.musicxml":    {"key changes": 1},
	"13e-KeySignatures-MidMeasure-Change.musicxml": {"key changes": 1},
	"34b-Colors.musicxml":                          {"key changes": 2, "clef changes": 3},
	"34c-Font-Size.musicxml":                       {"key changes": 2},
	"42b-MultiVoice-MidMeasureClefChange.musicxml": {"clef changes": 3},
	"46j-EmptyMeasure.musicxml":                    {"meter changes": 2},

	// Seventeen clefs, of which gscore draws five. MusicXML writes a clef
	// as a letter and a line, so a C clef alone is five signs; gscore has
	// alto and tenor and no others, and the rest are read as the nearest
	// one and logged. Nothing is lost but the sign — the pitches are
	// written as they sound either way.
	"12a-Clefs.musicxml": {"clef changes": 12},

	// Two chord symbols over one note: the harmony changes halfway
	// through it. gscore writes one symbol over a note; the second is
	// logged rather than dropped in silence.
	"71g-MultipleChordnames.musicxml": {"chord symbols": 2},
}

func suitePaths(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata/suite", "*.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 150 {
		t.Fatalf("only %d files in the suite", len(files))
	}
	sort.Strings(files)
	return files
}

// The one that notices a whole feature being read as nothing.
func TestEveryMarkArrives(t *testing.T) {
	improved := map[string][]string{}
	for _, path := range suitePaths(t) {
		name := filepath.Base(path)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s, _ := Import(strings.NewReader(string(b)))
		if s == nil {
			continue
		}
		for _, f := range theFeatures {
			want := f.want(b)
			if want == 0 {
				continue
			}
			got := f.got(s)
			least, known := shortOf[name][f.name]
			switch {
			case !known && got < want:
				t.Errorf("%s: the file asks for %d %s and %d arrived",
					name, want, f.name, got)
			case known && got < least:
				t.Errorf("%s: %d of %d %s arrived, and %d used to: "+
					"this has gone backwards", name, got, want, f.name, least)
			case known && got > least:
				improved[name] = append(improved[name],
					f.name+": "+itoa(got)+" now, "+itoa(least)+" recorded")
			}
		}
	}
	for name, lines := range improved {
		for _, l := range lines {
			t.Errorf("%s now reads more than shortOf records — %s; "+
				"raise or remove its entry", name, l)
		}
	}
}
