/*
 * fretchord.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/analysis"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * Writing a chord by its shape.
 *
 * A guitarist does not think of a chord as six pitches. They think of it as
 * a shape — where the fingers go — and the pitches are what comes out. Typed
 * into a tablature one string at a time, a six-note chord is six frets on
 * six different lines, and the caret has to be moved between every one of
 * them; the shape, which is the thing actually being written down, never
 * appears anywhere.
 *
 * So a shape is what this takes: one fret per string, or x for a string
 * that is not played. It is the same spelling the chord diagrams already
 * use — "x02210" is an A major — so a fingering can be copied from one to
 * the other, and the same spelling guitarists have written chords in for
 * as long as there has been plain text to write them in.
 *
 * What it writes is an ordinary chord. There is nothing tablature-shaped
 * about the result: the notes know which string and fret they came from,
 * the way every note typed onto a tab does, and the staff of pitches beside
 * it draws the same chord without knowing any of this happened.
 */

// shapeFret is one string of a shape: which fret, or not played.
type shapeFret struct {
	str, fret int
	played    bool
}

// parseShape reads a fingering into one entry per string.
//
// Written lowest-sounding string first, which is how a guitarist says a
// chord and how every chord diagram in the program is already written: the
// low E of a guitar is the first character. The tablature numbers its
// strings the other way about — string one is the top line — so the two
// are turned round here, once, rather than everywhere after.
//
// Frets past nine have to be written somehow, and a chord high on the neck
// is not unusual, so the characters may be separated: "x,0,2,2,10,0" reads
// as well as "x02210" and says something the tighter spelling cannot.
func parseShape(s string, lines int) ([]shapeFret, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, editError("no shape: say a fret for each string, or x")
	}

	var fields []string
	if strings.ContainsAny(s, " ,-") {
		fields = strings.FieldsFunc(s, func(r rune) bool {
			return r == ' ' || r == ',' || r == '-'
		})
	} else {
		for _, r := range s {
			fields = append(fields, string(r))
		}
	}
	if len(fields) != lines {
		return nil, editError("that shape is " + strconv.Itoa(len(fields)) +
			" strings and the instrument has " + strconv.Itoa(lines))
	}

	out := make([]shapeFret, 0, len(fields))
	for i, f := range fields {
		// Lowest first in, string one at the top out.
		str := lines - i
		switch strings.ToLower(f) {
		case "x", ".", "-":
			out = append(out, shapeFret{str: str})
			continue
		}
		fret, err := strconv.Atoi(f)
		if err != nil || fret < 0 {
			return nil, editError(fmt.Sprintf(
				"%q is not a fret, and not x for a string that is not played", f))
		}
		out = append(out, shapeFret{str: str, fret: fret, played: true})
	}
	return out, nil
}

// chordFromShape turns a shape into notes, lowest first.
//
// Lowest first because that is how a chord is stored and drawn: the
// engraver reads the noteheads from the bottom up, and a chord handed over
// the other way round comes out with its stem on the wrong side of the
// wrong note.
func chordFromShape(tab *model.Tablature, shape []shapeFret,
	trans model.Transposition, dur model.Duration) (*model.Chord, error) {
	var notes []*model.Note
	for _, f := range shape {
		if !f.played {
			continue
		}
		sounding, ok := tab.PitchAt(f.str, f.fret)
		if !ok {
			return nil, editError(fretComplaint(tab, f.str, f.fret))
		}
		notes = append(notes, &model.Note{
			// The model keeps what is printed and the tuning says what
			// sounds; on a guitar the two are an octave apart.
			Pitch: trans.Unapply(sounding),
			Dur:   dur, String: f.str, Fret: f.fret,
		})
	}
	if len(notes) == 0 {
		return nil, editError("every string is muted, so there is no chord")
	}

	// Sorted by what they sound and not by which string they are on.
	//
	// The two are the same on a guitar and are not on a ukulele, whose
	// fourth string is a G above the third string's C — a reentrant
	// tuning, which gscore offers and which is the whole character of the
	// instrument. A chord handed over in string order would then be out of
	// order by pitch, and a chord is drawn and read from the bottom up:
	// the engraver would put the stem on the wrong side of the wrong note
	// and the noteheads of a second would fall on the wrong sides.
	sort.SliceStable(notes, func(i, j int) bool {
		return notes[i].Pitch.MIDI() < notes[j].Pitch.MIDI()
	})
	return &model.Chord{Notes: notes, Dur: dur}, nil
}

// handleFretChord writes a chord onto a tablature from its shape, or says
// what the shape is without writing it.
func (a *App) handleFretChord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}
	dur, ok := durationNames[q.Get("dur")]
	if !ok {
		dur = model.Duration{Num: 1, Den: 4}
	}
	dur = dotted(dur, q.Get("dots"))

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locateForWriting(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tab := a.tabOfStaff(partIdx, p)
	if tab == nil {
		http.Error(w, "that staff is not written as tablature",
			http.StatusBadRequest)
		return
	}

	shape, err := parseShape(q.Get("frets"), tab.Lines())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chord, err := chordFromShape(tab, shape, p.Transpose, dur)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// What it is called, which is what the builder shows while somebody is
	// still moving fingers about. Named from what sounds and not from what
	// is printed: a guitar reads an octave above where it sounds, and the
	// chord is the chord either way, but the bass note decides the
	// inversion and that is a question about pitches.
	var midis []int
	for _, n := range chord.Notes {
		midis = append(midis, p.Transpose.Apply(n.Pitch).MIDI())
	}
	named := analysis.NameChord(midis)

	answer := map[string]any{
		"status": "ok",
		"name":   named.Name,
		"notes":  noteNames(chord),
	}

	// A preview asks what the shape is and writes nothing. The builder
	// calls it on every click, and a dialog that wrote a chord each time
	// somebody tried a finger somewhere would be unusable.
	if q.Get("preview") == "1" {
		writeJSON(w, answer)
		return
	}

	v := m.Voices[0]
	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	// Into the bar rather than after its rest, and on into the next bar
	// when this one is full — the same two rules a single fret follows.
	at = openEmptyBar(p, m, measIdx, at)
	m, measIdx, at = carryOn(p, m, measIdx, at)
	v = m.Voices[0]
	v.Elements = append(v.Elements, nil)
	copy(v.Elements[at+1:], v.Elements[at:])
	v.Elements[at] = chord
	spill(p, measIdx)

	landed, idx := findElement(p, measIdx, chord)
	answer["measure"] = landed
	answer["at"] = idx - 1
	writeJSON(w, answer)
}

// noteNames is what the chord came out as, for the builder to show.
func noteNames(c *model.Chord) []string {
	out := make([]string, 0, len(c.Notes))
	for _, n := range c.Notes {
		out = append(out, n.Pitch.String())
	}
	return out
}
