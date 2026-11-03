/*
 * transpose.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"net/http"
	"strconv"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Moving music to another key.
 *
 * An interval is two numbers and not one. The semitones say what is heard
 * and the steps say how it is spelled, and neither on its own describes an
 * interval: up four semitones and two letters is a major third, C to E; up
 * four semitones and three letters is a diminished fourth, C to F flat. The
 * same sound, a different note, and a reader who has to play it can tell
 * the difference. So the interval arrives here as both, the way
 * model.Pitch.Transpose and the transposing instruments already want it.
 *
 * Two kinds of move, as every notation program offers:
 *
 *   chromatic   the interval exactly, whatever it does to the spelling.
 *               A major third up turns C into E and C sharp into E sharp.
 *   diatonic    that many letters, spelled by the key. A third up in C
 *               minor turns C into E flat, because that is the third
 *               degree of the key.
 *
 * The key signature moves with the music when the whole piece moves, and
 * stays where it is when part of it does. Both are right: a song put up a
 * tone for a singer is in another key, and a phrase lifted a tone inside a
 * piece is a phrase with accidentals on it.
 */

// transposer is one transposition being applied.
type transposer struct {
	steps, semis int
	// diatonic moves by letters alone and lets the key spell the result.
	diatonic bool
	// keyMoves says the signature is going with the notes, so a note that
	// needed no accidental before needs none now.
	keyMoves bool
}

// note moves one note, and writes whatever accidental it now has to carry.
//
// at is where the note stands in its bar, because what a note has to say
// depends on what has been said before it: a sharp earlier in the bar holds
// until the barline, and a note that lands on that pitch is written plain.
func (t transposer) note(n *model.Note, m *model.Measure, at int, key model.KeySignature) {
	switch {
	case t.diatonic:
		n.Pitch = model.PitchFromDiatonicIndex(n.Pitch.DiatonicIndex() + t.steps)
		n.Pitch.Alter = key.AlterFor(n.Pitch.Step)
	case t.keyMoves:
		// The signature goes the same way, so a note in the key stays in
		// the key and one written with an accidental keeps one. This is
		// what changing a part's instrument does, and for the same reason.
		movePitch(n, t.steps, t.semis)
		return
	default:
		n.Pitch = n.Pitch.Transpose(t.steps, t.semis)
	}
	n.Accidental = signFor(n.Pitch, alterInForce(m, at, n.Pitch, key))
}

// element moves whatever is at one place in a bar, and says how many notes
// that turned out to be.
func (t transposer) element(el model.Element, m *model.Measure, at int,
	key model.KeySignature) int {
	switch e := el.(type) {
	case *model.Note:
		t.note(e, m, at, key)
		return 1
	case *model.Chord:
		// The noteheads keep their order: an interval moves them all the
		// same way, so the chord is still written from the bottom up.
		for _, n := range e.Notes {
			t.note(n, m, at, key)
		}
		return len(e.Notes)
	}
	return 0
}

// lettersFor is how many letters an interval of so many semitones moves by,
// read the ordinary way.
//
// Seven of the twelve have only one reading — a fifth is a fifth — and five
// of them have two. Those are spelled as the smaller of the pair, which is
// how they are named and how they are nearly always meant: three semitones
// is a minor third rather than an augmented second, and ten is a minor
// seventh rather than an augmented sixth.
//
// The tritone is the one with no smaller reading, and it is taken as an
// augmented fourth. Either is defensible — up six semitones from C is F
// sharp or G flat and both are six accidentals — so it is settled here
// rather than left to come out of the arithmetic.
//
// Octaves are carried whole: fourteen semitones is an octave and a tone,
// which is seven letters and one.
func lettersFor(semitones int) int {
	within := [12]int{0, 1, 1, 2, 2, 3, 3, 4, 5, 5, 6, 6}
	octaves := semitones / 12
	rest := semitones % 12
	// Go rounds towards zero, so a negative remainder is brought back to
	// the positive side and the octave count made up for it. Without this,
	// down a tone asked for the -1st entry of the table.
	if rest < 0 {
		rest += 12
		octaves--
	}
	return octaves*7 + within[rest]
}

// shiftKey moves a signature by places around the circle of fifths.
func shiftKey(k model.KeySignature, by int) model.KeySignature {
	out := k
	out.Fifths = k.Fifths + by
	for out.Fifths > 7 {
		out.Fifths -= 12
	}
	for out.Fifths < -7 {
		out.Fifths += 12
	}
	return out
}

// transposable reports whether a staff's pitches mean pitches.
//
// A drum staff's lines name instruments: moving a snare up a tone is moving
// it to a tom, which is not what anybody asked for, and moving a drum part
// is its own job with its own rules.
//
// A tablature used to be refused here for a reason that has since been
// dealt with rather than for a reason of its own: moving the pitch without
// moving the finger makes the number on the page disagree with the note it
// stands for. So the fingers are moved too — see refretTabs, which runs
// after every move — and a guitar part transposes like any other.
//
// Refusing it was worse than it looked. A guitar written as a staff of
// pitches with a tablature under it has its music on the staff, which was
// transposed, while the tablature was skipped: the notes moved and the
// fret numbers under them did not, so the two halves of one part said
// different things and nothing reported it.
func transposable(p *model.Part) bool {
	return p.Clef != model.Percussion
}

// refretTabs works out again where every tablature's notes are played.
//
// Called after the music has moved. A fret number is where a finger goes
// and is only true of one pitch on one tuning, so anything that changes the
// pitch leaves it stale.
//
// The music a tablature draws may not be its own: a staff of pitches with a
// tab under it keeps the notes once, on the staff, and the tab is that same
// music written a second way. So the notes are fetched from wherever they
// live and the tuning from the tab that draws them.
func (a *App) refretTabs() {
	for i, p := range a.score.Parts {
		if p.Tab == nil {
			continue
		}
		music := a.score.MusicOf(i)
		if music == nil {
			continue
		}
		// The tuning says what the open strings sound; the model keeps
		// what is printed. They are an octave apart on a guitar, and the
		// transposition of the staff that holds the music is what bridges
		// them.
		for _, m := range music.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					switch e := el.(type) {
					case *model.Note:
						restringNote(p.Tab, e, music.Transpose)
					case *model.Chord:
						for _, n := range e.Notes {
							restringNote(p.Tab, n, music.Transpose)
						}
					}
				}
			}
		}
	}
}

// handleTranspose moves the selection, or the whole piece, to another key.
func (a *App) handleTranspose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	// Told apart from a steps of zero, which is a real interval and not an
	// omission: zero letters and one semitone is an augmented unison, C to
	// C sharp, which is a different move from the minor second C to D
	// flat. Only where nothing was said at all is the interval guessed.
	steps, haveSteps := 0, false
	if v := q.Get("steps"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "steps is a number of letters", http.StatusBadRequest)
			return
		}
		steps, haveSteps = n, true
	}
	semis, _ := strconv.Atoi(q.Get("semitones"))
	diatonic := q.Get("mode") == "diatonic"
	if diatonic {
		semis = 0
	}
	// An interval is two numbers and not one, and a caller that gave only
	// the semitones has not described one. Taken as zero, the letters never
	// moved and the whole distance went into the accidental: a tone above
	// A came out as A double sharp, and again on the next move, so a part
	// transposed twice was unreadable. What is filled in is the ordinary
	// reading of that distance — four semitones is a major third and not a
	// diminished fourth — which is what somebody who gave one number meant.
	if !diatonic && !haveSteps {
		steps = lettersFor(semis)
	}
	if steps == 0 && semis == 0 {
		http.Error(w, "that is no distance at all", http.StatusBadRequest)
		return
	}
	// Four octaves either way is past the ends of every instrument there
	// is, so a number beyond it is a mistake rather than a request.
	if steps < -28 || steps > 28 || semis < -48 || semis > 48 {
		http.Error(w, "that is further than four octaves", http.StatusBadRequest)
		return
	}

	// The signature moves with the music only when all of the music is
	// moving. Part of a piece lifted a tone is still in the key it was in,
	// written with accidentals; and a move by letters alone stays in the
	// key by definition.
	sel := q.Get("sel")
	moveKey := sel == "" && !diatonic && q.Get("key") != "0"
	t := transposer{steps: steps, semis: semis, diatonic: diatonic, keyMoves: moveKey}

	a.lock()
	defer a.mu.Unlock()

	changed, skipped := 0, 0
	if sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, tg := range targets {
			p, m, err := a.locate(tg.part, tg.measure)
			if err != nil || len(m.Voices) == 0 || m.Voices[0] == nil {
				continue // a stale selection is not worth failing over
			}
			if !transposable(p) {
				skipped++
				continue
			}
			v := m.Voices[0]
			if tg.index < 0 || tg.index >= len(v.Elements) {
				continue
			}
			changed += t.element(v.Elements[tg.index], m, tg.index, p.KeyAt(tg.measure))
		}
		if changed == 0 {
			http.Error(w, "there is no note in the selection to move",
				http.StatusBadRequest)
			return
		}
		a.refretTabs()
		writeJSON(w, map[string]any{"changed": changed, "left": skipped})
		return
	}

	// The whole piece. Every staff moves by the same interval, transposing
	// instruments included: what is stored is what each player reads, and
	// a part written a tone up from the piece is still a tone up from it
	// after the piece has moved.
	fifths := model.Transposition{Chromatic: semis}.Fifths()
	for _, p := range a.score.Parts {
		if !transposable(p) {
			skipped++
			continue
		}
		if moveKey {
			p.Key = shiftKey(p.Key, fifths)
		}
		// Walked rather than asked for bar by bar: KeyAt counts from the
		// first measure every time, and a long score asked once a bar is
		// the same walk over again for every bar in it.
		key := p.Key
		for _, m := range p.Measures {
			if m.Key != nil {
				if moveKey {
					k := shiftKey(*m.Key, fifths)
					m.Key = &k
				}
				key = *m.Key
			}
			for _, v := range m.Voices {
				if v == nil {
					continue
				}
				// The accidental a note has to carry is read off the first
				// voice, as it is everywhere else here: a second voice
				// sharing the bar is spelled against what the first one
				// has already said, which is how it is read.
				for i, el := range v.Elements {
					changed += t.element(el, m, i, key)
				}
			}
		}
	}
	if changed == 0 {
		http.Error(w, "there is nothing here to move", http.StatusBadRequest)
		return
	}
	// The fingers follow the notes. Without this the numbers on a
	// tablature are where the hand went for the music as it was.
	a.refretTabs()
	out := map[string]any{"changed": changed, "left": skipped}
	if moveKey && len(a.score.Parts) > 0 {
		out["fifths"] = a.score.Parts[0].Key.Fifths
	}
	writeJSON(w, out)
}
