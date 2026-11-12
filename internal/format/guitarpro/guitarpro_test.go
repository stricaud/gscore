/*
 * guitarpro_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Guitar Pro files, read in all three dialects.
 *
 * Each test builds a song, writes it with the test writer in each of the
 * five versions the reader knows, and checks what came back. The byte
 * layout is tested against real files by the corpus test; these are about
 * what the bytes mean once they are read.
 */

package guitarpro

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func TestTheThreeDialectsReadTheSameSong(t *testing.T) {
	g := guitar("Lead")
	g.measures = []measure{
		bar(q(5, 3), q(5, 5), q(4, 2), q(4, 3)),
		bar(e(1, 0), e(2, 0), rest(4), beat{dur: 2, tempo: -1, notes: []note{{str: 1, fret: 3, finger: -1}, {str: 2, fret: 0, finger: -1}}}),
	}
	b := track{name: "Bass", strings: []int{43, 38, 33, 28}, channel: 2, frets: 20}
	b.measures = []measure{bar(q(4, 3), q(4, 3), q(3, 5), q(3, 5)), bar(beat{dur: 1, tempo: -1, notes: []note{{str: 4, fret: 0, finger: -1}}})}
	sg := newSong(2, g, b)
	sg.artist = "The Testers"
	sg.music = "A. Composer"
	sg.copyright = "© 2026"
	sg.tempo = 96

	every(t, func(t *testing.T, v, code int) {
		s, err := readBack(t, sg, v, code)
		if err != nil {
			t.Errorf("read with a complaint: %v", err)
		}
		if s.Identity.Title != "Test" || s.Identity.Subtitle != "The Testers" ||
			s.Identity.Composer != "A. Composer" || s.Identity.Copyright != "© 2026" {
			t.Errorf("the header came back as %+v", s.Identity)
		}
		if s.Tempo.BPM != 96 {
			t.Errorf("tempo %d, want 96", s.Tempo.BPM)
		}
		if len(s.Parts) != 2 {
			t.Fatalf("%d parts, want two", len(s.Parts))
		}
		lead, bass := s.Parts[0], s.Parts[1]
		if lead.Name != "Lead" || bass.Name != "Bass" {
			t.Errorf("the parts are called %q and %q", lead.Name, bass.Name)
		}
		if lead.Clef != model.Treble8vb || bass.Clef != model.Bass {
			t.Errorf("clefs %v and %v, want treble8vb and bass", lead.Clef, bass.Clef)
		}
		if lead.Sound != "pluck.guitar.steel-string" || !strings.HasPrefix(bass.Sound, "pluck.bass") {
			t.Errorf("sounds %q and %q", lead.Sound, bass.Sound)
		}
		if lead.Tab == nil || lead.Tab.Strings[0].String() != "E4" || lead.Tab.Strings[5].String() != "E2" {
			t.Errorf("the guitar's tab is %v", lead.Tab)
		}
		if bass.Tab == nil || len(bass.Tab.Strings) != 4 || bass.Tab.Strings[3].String() != "E1" {
			t.Errorf("the bass's tab is %v", bass.Tab)
		}

		// Bar one of the guitar: C3 D3 E3 F3, by fret on the A and D strings.
		var pitches []string
		for _, n := range notesOf(elements(s, 0, 0)) {
			pitches = append(pitches, n.Pitch.String())
		}
		if got := strings.Join(pitches, " "); got != "C3 D3 E3 F3" {
			t.Errorf("bar one is %s, want C3 D3 E3 F3", got)
		}
		// Bar two: two eighths, a quarter rest, a half-note chord.
		els := elements(s, 0, 1)
		if len(els) != 4 {
			t.Fatalf("bar two holds %d things, want four", len(els))
		}
		if _, ok := els[2].(*model.Rest); !ok {
			t.Errorf("the third thing is a %T, want a rest", els[2])
		}
		ch, ok := els[3].(*model.Chord)
		if !ok || len(ch.Notes) != 2 || ch.Dur != (model.Duration{Num: 1, Den: 2}) {
			t.Errorf("the last thing is %v, want a half-note chord of two", els[3])
		}
		if d := els[0].Duration(); d != (model.Duration{Num: 1, Den: 8}) {
			t.Errorf("the first eighth is %v", d)
		}
		if s.Parts[0].Measures[1].RightBarline != model.BarFinal {
			t.Errorf("the piece does not end with a final barline")
		}
		// The bass's bottom string open is E1.
		if n := notesOf(elements(s, 1, 1))[0]; n.Pitch.String() != "E1" || n.Dur != (model.Duration{Num: 1, Den: 1}) {
			t.Errorf("the bass's whole note is %s %v", n.Pitch, n.Dur)
		}
	})
}

func TestACapoRaisesTheOpenStrings(t *testing.T) {
	g := guitar("Capo")
	g.capo = 2
	g.measures = []measure{bar(q(6, 0), q(6, 3), rest(2))}
	every(t, func(t *testing.T, v, code int) {
		s, _ := readBack(t, newSong(1, g), v, code)
		p := s.Parts[0]
		if p.Tab.Strings[5].String() != "F#2" {
			t.Errorf("the bottom string with a capo at 2 is %s, want F#2", p.Tab.Strings[5])
		}
		ns := notesOf(elements(s, 0, 0))
		if ns[0].Pitch.String() != "F#2" || ns[1].Pitch.String() != "A2" {
			t.Errorf("open and third fret sound %s and %s, want F#2 and A2", ns[0].Pitch, ns[1].Pitch)
		}
		if ns[0].Fret != 0 || ns[1].Fret != 3 {
			t.Errorf("the frets are written %d and %d, want 0 and 3 from the capo", ns[0].Fret, ns[1].Fret)
		}
		found := false
		for _, tm := range ns[0].Texts {
			if tm.Text == "Capo 2" {
				found = true
			}
		}
		if !found {
			t.Errorf("nothing says where the capo is: %v", ns[0].Texts)
		}
	})
}

func TestTheHandsMarksAreRead(t *testing.T) {
	g := guitar("Marks")
	g.measures = []measure{
		bar(
			with(e(1, 5), func(n *note) { n.hammer = true }),
			e(1, 7),
			with(e(1, 7), func(n *note) { n.slide = 1 }), // up to the 9
			e(1, 9),
			with(e(1, 9), func(n *note) { n.slide = 1 }), // down to the 5
			e(1, 5),
			with(e(1, 5), func(n *note) { n.bend = &bend{kind: 1, value: 100} }),
			with(e(1, 5), func(n *note) { n.bend = &bend{kind: 1, value: 50} }),
		),
		bar(
			with(e(2, 5), func(n *note) { n.bend = &bend{kind: 2, value: 100} }),
			with(e(2, 5), func(n *note) { n.bend = &bend{kind: 4, value: 100} }),
			with(e(2, 5), func(n *note) { n.letRing = true; n.palmMute = true }),
			with(e(2, 5), func(n *note) { n.vib = true; n.staccato = true }),
			with(e(2, 5), func(n *note) { n.harmonic = 1 }),
			with(e(2, 5), func(n *note) { n.harmonic = 2; n.accent = true }),
			with(e(2, 5), func(n *note) { n.harmonic = 4; n.heavy = true }),
			with(e(2, 5), func(n *note) { n.dead = true; n.ghost = true }),
		),
		bar(
			with(q(3, 7), func(n *note) { n.tremolo = 2 }),
			with(q(3, 7), func(n *note) { n.trill = &trill{fret: 9, period: 1} }),
			with(q(3, 7), func(n *note) { n.grace = &grace{fret: 5, transition: 3, duration: 1} }),
			with(q(3, 7), func(n *note) { n.velocity = 3 }),
		),
	}
	g.measures[2].voices[0][0].tap = true
	every(t, func(t *testing.T, v, code int) {
		s, _ := readBack(t, newSong(3, g), v, code)
		one := notesOf(elements(s, 0, 0))
		two := notesOf(elements(s, 0, 1))
		three := notesOf(elements(s, 0, 2))
		if len(one) != 8 || len(two) != 8 || len(three) != 4 {
			t.Fatalf("%d, %d and %d notes", len(one), len(two), len(three))
		}
		want := func(n *model.Note, tc model.Technique) {
			t.Helper()
			if !hasTechnique(n, tc) {
				t.Errorf("fret %d on string %d carries %q, want %v", n.Fret, n.String, techniques(n), tc)
			}
		}
		want(one[0], model.HammerOn)
		want(one[2], model.SlideUp)
		want(one[4], model.SlideDown)
		want(one[6], model.BendFull)
		want(one[7], model.BendHalf)
		want(two[0], model.BendRelease)
		want(two[1], model.PreBend)
		want(two[2], model.LetRing)
		want(three[0], model.Tap)
		if v == 3 {
			// GP3 writes vibrato and harmonics on the beat and has no
			// palm mute, no staccato and no second byte of effects, so
			// the rest is GP4 and later.
			return
		}
		want(two[2], model.PalmMute)
		want(two[3], model.Vibrato)
		want(two[4], model.NaturalHarmonic)
		want(two[5], model.ArtificialHarmonic)
		want(two[6], model.PinchHarmonic)
		if !hasArticulation(two[3], model.Staccato) {
			t.Errorf("no staccato: %v", two[3].Articulations)
		}
		if !hasArticulation(two[5], model.Accent) {
			t.Errorf("no accent: %v", two[5].Articulations)
		}
		if v == 5 && !hasArticulation(two[6], model.Marcato) {
			t.Errorf("no heavy accent: %v", two[6].Articulations)
		}
		if two[7].Head != model.HeadX || !two[7].Small {
			t.Errorf("the dead ghost note has head %v and small %v", two[7].Head, two[7].Small)
		}
		if three[0].Tremolo == nil || three[0].Tremolo.Strokes != 2 {
			t.Errorf("tremolo picking came back as %v", three[0].Tremolo)
		}
		if three[1].Trill == nil {
			t.Errorf("the trill was lost")
		}
		if three[3].Dynamic != model.Piano {
			t.Errorf("the dynamic is %v, want p", three[3].Dynamic)
		}
	})
	// The grace note, in every dialect that has one.
	every(t, func(t *testing.T, v, code int) {
		s, _ := readBack(t, newSong(3, g), v, code)
		n := notesOf(elements(s, 0, 2))[2]
		if n.Grace == nil || len(n.Grace.Notes) != 1 {
			t.Fatalf("the grace note was lost: %v", n.Grace)
		}
		gn := n.Grace.Notes[0]
		if gn.Fret != 5 || gn.String != 3 || gn.Pitch.String() != "C4" || !hasTechnique(gn, model.HammerOn) {
			t.Errorf("the grace note is fret %d string %d %s %q", gn.Fret, gn.String, gn.Pitch, techniques(gn))
		}
		if !n.Grace.Slashed {
			t.Errorf("a grace before the beat is not slashed")
		}
	})
}

func hasArticulation(n *model.Note, want model.Articulation) bool {
	for _, a := range n.Articulations {
		if a == want {
			return true
		}
	}
	return false
}

func TestATieReachesBackToTheNoteBefore(t *testing.T) {
	g := guitar("Ties")
	g.measures = []measure{
		bar(q(2, 3), q(2, 3), beat{dur: 2, tempo: -1, notes: []note{{str: 2, fret: 3, finger: -1}, {str: 3, fret: 0, finger: -1}}}),
		bar(with(q(2, 0), func(n *note) { n.tied = true }), q(1, 0), with(q(1, 0), func(n *note) { n.tied = true }), rest(4)),
	}
	every(t, func(t *testing.T, v, code int) {
		s, err := readBack(t, newSong(2, g), v, code)
		if err != nil {
			t.Errorf("complaint: %v", err)
		}
		one := elements(s, 0, 0)
		ch := one[2].(*model.Chord)
		var held *model.Note
		for _, n := range ch.Notes {
			if n.String == 2 {
				held = n
			}
		}
		if held == nil || !held.TieStart {
			t.Errorf("the chord's B-string note does not start the tie across the bar")
		}
		two := notesOf(elements(s, 0, 1))
		if !two[0].TieStop || two[0].Fret != 3 || two[0].Pitch.String() != "D4" {
			t.Errorf("the tied note is fret %d %s stop %v; want the third fret D4 carried over", two[0].Fret, two[0].Pitch, two[0].TieStop)
		}
		if !two[1].TieStart || !two[2].TieStop {
			t.Errorf("the tie within the bar is %v → %v", two[1].TieStart, two[2].TieStop)
		}
	})
}

func TestTupletsAreGrouped(t *testing.T) {
	g := guitar("Triplets")
	trip := func(b beat) beat { b.tuplet = 3; return b }
	g.measures = []measure{
		bar(trip(e(1, 0)), trip(e(1, 1)), trip(e(1, 2)), trip(e(1, 3)), trip(e(1, 4)), trip(e(1, 5)), rest(2)),
		bar(trip(q(1, 0)), trip(e(1, 1)), rest(2), rest(4)),
	}
	every(t, func(t *testing.T, v, code int) {
		s, _ := readBack(t, newSong(2, g), v, code)
		one := notesOf(elements(s, 0, 0))
		for i, n := range one {
			if n.Dur != (model.Duration{Num: 1, Den: 12}) {
				t.Errorf("note %d lasts %v, want a twelfth", i, n.Dur)
			}
			if n.Tuplet == nil || n.Tuplet.Actual != 3 || n.Tuplet.Normal != 2 {
				t.Fatalf("note %d has tuplet %v", i, n.Tuplet)
			}
		}
		// Six triplet eighths are two brackets of three.
		var starts, stops []int
		for i, n := range one {
			if n.Tuplet.Start {
				starts = append(starts, i)
			}
			if n.Tuplet.Stop {
				stops = append(stops, i)
			}
		}
		if len(starts) != 2 || starts[0] != 0 || starts[1] != 3 || len(stops) != 2 || stops[0] != 2 || stops[1] != 5 {
			t.Errorf("brackets open at %v and close at %v, want 0,3 and 2,5", starts, stops)
		}
		// A quarter and an eighth under one 3.
		two := notesOf(elements(s, 0, 1))
		if !two[0].Tuplet.Start || two[0].Tuplet.Stop || !two[1].Tuplet.Stop {
			t.Errorf("quarter+eighth triplet: %+v %+v", *two[0].Tuplet, *two[1].Tuplet)
		}
		if total := s.Parts[0].Measures[1].Voices[0].Duration(); total != (model.Duration{Num: 1, Den: 1}) {
			t.Errorf("the bar adds up to %v", total)
		}
	})
}

func TestTheBarsOwnSigns(t *testing.T) {
	g := guitar("Signs")
	g.measures = make([]measure, 6)
	for i := range g.measures {
		g.measures[i] = bar(q(1, 0), q(1, 0), q(1, 0), q(1, 0))
	}
	g.measures[2] = bar(q(1, 0), q(1, 0), q(1, 0))
	g.measures[3] = bar(q(1, 0), q(1, 0), q(1, 0))
	g.measures[4].voices[0][0].tempo = 80
	two := 2
	sg := newSong(6, g)
	sg.headers = []header{
		{num: 4, den: 4, marker: "Intro"},
		{num: 4, den: 4, repeatOpen: true},
		{num: 3, den: 4, key: &two},
		{num: 3, den: 4, repeatClose: 1, alt: 0x01},
		{num: 4, den: 4, alt: 0x02},
		{num: 4, den: 4},
	}
	every(t, func(t *testing.T, v, code int) {
		s, _ := readBack(t, sg, v, code)
		ms := s.Parts[0].Measures
		if ms[0].Marker != "Intro" {
			t.Errorf("the marker is %q", ms[0].Marker)
		}
		if ms[0].RightBarline != model.BarRepeatStart {
			t.Errorf("the barline before the repeat is %v", ms[0].RightBarline)
		}
		if ms[3].RightBarline != model.BarRepeatEnd {
			t.Errorf("the repeat closes with %v", ms[3].RightBarline)
		}
		if ms[2].Time == nil || ms[2].Time.Beats != 3 || ms[4].Time == nil || ms[4].Time.Beats != 4 {
			t.Errorf("the meter changes are %v and %v", ms[2].Time, ms[4].Time)
		}
		if ms[1].Time != nil || ms[3].Time != nil {
			t.Errorf("a meter change was written where the meter did not change")
		}
		if ms[2].Key == nil || ms[2].Key.Fifths != 2 {
			t.Errorf("the key change is %v, want two sharps", ms[2].Key)
		}
		if e := ms[3].Ending; e == nil || len(e.Numbers) != 1 || e.Numbers[0] != 1 || !e.Start || !e.Stop {
			t.Errorf("the first ending is %+v", e)
		}
		if e := ms[4].Ending; e == nil || len(e.Numbers) != 1 || e.Numbers[0] != 2 || !e.Open {
			t.Errorf("the second ending is %+v", e)
		}
		if ms[4].Tempo == nil || ms[4].Tempo.BPM != 80 {
			t.Errorf("the tempo change is %v", ms[4].Tempo)
		}
		if ms[5].RightBarline != model.BarFinal {
			t.Errorf("the last bar ends with %v", ms[5].RightBarline)
		}
		if total := ms[2].Voices[0].Duration(); total != (model.Duration{Num: 3, Den: 4}) {
			t.Errorf("the 3/4 bar holds %v", total)
		}
	})
}

func TestADrumTrackIsAPercussionStaff(t *testing.T) {
	d := track{name: "Drums", strings: []int{0, 0, 0, 0, 0, 0}, channel: 10, frets: 24}
	d.measures = []measure{bar(
		beat{dur: 8, tempo: -1, notes: []note{{str: 1, fret: 42, finger: -1}, {str: 2, fret: 36, finger: -1}}},
		e(1, 42),
		beat{dur: 8, tempo: -1, notes: []note{{str: 1, fret: 42, finger: -1}, {str: 2, fret: 38, finger: -1}}},
		e(1, 42),
		e(1, 39), // a hand clap, which the kit has no line for
		rest(8), rest(4),
	)}
	every(t, func(t *testing.T, v, code int) {
		s, err := readBack(t, newSong(1, d), v, code)
		p := s.Parts[0]
		if p.Clef != model.Percussion || p.Tab != nil {
			t.Errorf("the drums are on a %v staff with tab %v", p.Clef, p.Tab)
		}
		ns := notesOf(elements(s, 0, 0))
		hat, kick := ns[0], ns[1]
		if hat.Percussion != 42 || hat.Head != model.HeadX {
			t.Errorf("the hi-hat is key %d head %v", hat.Percussion, hat.Head)
		}
		if kick.Percussion != 36 || kick.Head != model.HeadNormal {
			t.Errorf("the bass drum is key %d head %v", kick.Percussion, kick.Head)
		}
		if kick.Pitch.Step == hat.Pitch.Step && kick.Pitch.Octave == hat.Pitch.Octave {
			t.Errorf("the kick and the hi-hat sit on the same line")
		}
		if ns[6].Percussion != 39 {
			t.Errorf("the hand clap is key %d", ns[6].Percussion)
		}
		if err == nil || !strings.Contains(err.Error(), "guitarpro") {
			t.Errorf("the clap went by in silence: %v", err)
		}
		p2, ok := err.(format.Partial)
		if !ok || !strings.Contains(strings.Join(p2.Details(), " "), "39") {
			t.Errorf("the report does not name the clap's key: %v", err)
		}
	})
}

func TestChordNamesAndDiagrams(t *testing.T) {
	g := guitar("Chords")
	c := q(5, 3)
	c.chord = &chord{name: "C/E", firstFret: 1, frets: []int{0, 1, 0, 2, 3, -1}}
	c2 := q(5, 3)
	c2.chord = &chord{name: "C11/9-", firstFret: 1, frets: []int{-1, -1, -1, -1, -1, -1}}
	c3 := q(5, 3)
	c3.chord = &chord{name: "N.C.", firstFret: 0}
	g.measures = []measure{bar(c, c2, c3, rest(4))}
	every(t, func(t *testing.T, v, code int) {
		s, _ := readBack(t, newSong(1, g), v, code)
		ns := notesOf(elements(s, 0, 0))
		sym := ns[0].Chord
		if sym == nil || sym.Root != "C" || sym.Bass != "E" || sym.Quality != "" {
			t.Fatalf("C/E came back as %+v", sym)
		}
		if sym.Frame == nil || sym.Frame.Spell() != "x32010" || sym.Frame.FirstFret != 1 {
			t.Errorf("the diagram is %v", sym.Frame)
		}
		if sym2 := ns[1].Chord; sym2 == nil || sym2.Root != "C" || sym2.Quality != "11/9-" || sym2.Bass != "" {
			t.Errorf("C11/9- came back as %+v", sym2)
		}
		if sym3 := ns[2].Chord; sym3 == nil || sym3.Text != "N.C." {
			t.Errorf("N.C. came back as %+v", sym3)
		}
	})
}

func TestLyricsFallUnderTheNotes(t *testing.T) {
	g := guitar("Voice")
	g.measures = []measure{
		bar(q(1, 0), q(1, 0), q(1, 0), q(1, 0)),
		bar(q(1, 0), with(q(1, 0), func(n *note) { n.tied = true }), q(1, 0), q(1, 0)),
	}
	sg := newSong(2, g)
	sg.lyricTrack = 1
	sg.lyrics = []lyricLine{{from: 1, text: "Twin-kle twin-kle lit-tle\nstar"}}
	every(t, func(t *testing.T, v, code int) {
		if v < 4 {
			return // GP3 has no lyrics
		}
		s, _ := readBack(t, sg, v, code)
		var words []string
		var hyphens []bool
		for _, b := range []int{0, 1} {
			for _, n := range notesOf(elements(s, 0, b)) {
				for _, l := range n.Lyrics {
					words = append(words, l.Text)
					hyphens = append(hyphens, l.NeedsHyphen())
				}
			}
		}
		if got := strings.Join(words, " "); got != "Twin kle twin kle lit tle star" {
			t.Errorf("the words are %q", got)
		}
		if len(hyphens) != 7 || !hyphens[0] || hyphens[1] || !hyphens[4] || hyphens[6] {
			t.Errorf("the hyphens are %v", hyphens)
		}
		// The tied note sings nothing: "star" went to the note after it.
		two := notesOf(elements(s, 0, 1))
		if len(two[1].Lyrics) != 0 || len(two[2].Lyrics) != 1 {
			t.Errorf("the tied note sings %v and the next %v", two[1].Lyrics, two[2].Lyrics)
		}
	})
}

func TestASecondVoiceIsKeptWhereItHasNotes(t *testing.T) {
	g := guitar("Two voices")
	g.measures = []measure{
		{voices: [][]beat{{q(1, 0), q(1, 0), q(1, 0), q(1, 0)}, {beat{dur: 1, tempo: -1, notes: []note{{str: 6, fret: 0, finger: -1}}}}}},
		{voices: [][]beat{{q(1, 0), q(1, 0), q(1, 0), q(1, 0)}, {{dur: 4, empty: true, rest: true, tempo: -1}}}},
	}
	s, _ := readBack(t, newSong(2, g), 5, 1)
	ms := s.Parts[0].Measures
	if len(ms[0].Voices) != 2 {
		t.Fatalf("bar one has %d voices, want two", len(ms[0].Voices))
	}
	if n, ok := ms[0].Voices[1].Elements[0].(*model.Note); !ok || n.Pitch.String() != "E2" {
		t.Errorf("the second voice holds %v", ms[0].Voices[1].Elements)
	}
	if len(ms[1].Voices) != 1 {
		t.Errorf("bar two has %d voices, want one: the second had nothing in it", len(ms[1].Voices))
	}
}

func TestAnEmptyBarIsABarOfRest(t *testing.T) {
	g := guitar("Empty")
	g.measures = []measure{bar(q(1, 0), q(1, 0), q(1, 0), q(1, 0)), {}}
	every(t, func(t *testing.T, v, code int) {
		s, _ := readBack(t, newSong(2, g), v, code)
		els := elements(s, 0, 1)
		if len(els) != 1 {
			t.Fatalf("the empty bar holds %d things", len(els))
		}
		r, ok := els[0].(*model.Rest)
		if !ok || r.Dur != (model.Duration{Num: 1, Den: 1}) {
			t.Errorf("the empty bar holds %v, want a whole rest", els[0])
		}
	})
}

func TestWhatTheModelCannotHoldIsReported(t *testing.T) {
	g := guitar("Whammy")
	b := q(1, 0)
	b.whammy = true
	g.measures = []measure{bar(b, b, rest(2))}
	every(t, func(t *testing.T, v, code int) {
		s, err := readBack(t, newSong(1, g), v, code)
		if len(elements(s, 0, 0)) != 3 {
			t.Errorf("the bar lost its notes")
		}
		p, ok := err.(format.Partial)
		if !ok {
			t.Fatalf("the whammy bar went by in silence: %v", err)
		}
		if d := strings.Join(p.Details(), " "); !strings.Contains(d, "2 whammy bar dives") {
			t.Errorf("the report says %q", d)
		}
	})
}

func TestAFileCutShortIsReadAsFarAsItGoes(t *testing.T) {
	g := guitar("Cut")
	g.measures = []measure{
		bar(q(1, 0), q(1, 0), q(1, 0), q(1, 0)),
		bar(q(1, 1), q(1, 1), q(1, 1), q(1, 1)),
		bar(q(1, 2), q(1, 2), q(1, 2), q(1, 2)),
	}
	every(t, func(t *testing.T, v, code int) {
		data := write(newSong(3, g), v, code)
		s, err := Import(bytes.NewReader(data[:len(data)-12]))
		if s == nil {
			t.Fatalf("nothing came back: %v", err)
		}
		if n := len(notesOf(elements(s, 0, 0))); n != 4 {
			t.Errorf("the first bar holds %d notes", n)
		}
		p, ok := err.(format.Partial)
		if !ok {
			t.Fatalf("the cut was not reported: %v", err)
		}
		if d := strings.Join(p.Details(), " "); !strings.Contains(d, "ends early") || !strings.Contains(d, "bar 3") {
			t.Errorf("the report says %q", d)
		}
	})
}

func TestOtherFilesAreRefused(t *testing.T) {
	for _, tc := range []struct{ name, data, want string }{
		{"words", "This is not a tab\nat all, and goes on for a while longer than thirty bytes", "not a Guitar Pro file"},
		{"gp6", "BCFZ" + strings.Repeat("\x00", 40), "Guitar Pro 6 or 7"},
		{"gp2", "\x18FICHIER GUITAR PRO v2.21\x00\x00\x00\x00\x00\x00", "versions 3, 4 and 5"},
		{"short", "\x18FICHIER", "shorter than"},
	} {
		_, err := Import(strings.NewReader(tc.data))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestTheImporterIsRegistered(t *testing.T) {
	for _, name := range []string{"a.gp3", "b.GP4", "c.gp5"} {
		imp, err := format.ImporterFor(name)
		if err != nil || imp.Name() != "Guitar Pro" {
			t.Errorf("%s: %v %v", name, imp, err)
		}
	}
}
