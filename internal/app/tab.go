/*
 * tab.go
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
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Writing tablature.
//
// gscore could draw a tab before it could write one: a tab that arrived
// from another program came out right on the page, and there was no way to
// make one or to put a number on it. This is the other half.
//
// What is typed is a fret, and what is stored is a pitch. That is the whole
// shape of it. A fret number is where a finger goes and means nothing
// without the tuning; the pitch is what sounds, what transposes, what plays
// back and what every other part of the program already understands. So the
// number is turned into a pitch as it is typed, and the string and fret are
// kept beside it — because which of the several places a note can be played
// was a decision somebody made, and working it out again later would throw
// that away.

// restringPart gives every note on a staff a place on the neck, for the
// ones that have not got one.
//
// Called when a staff becomes a tab. A note that already says where it is
// played keeps that; the rest are put at the first place that reaches them,
// which is where a hand goes unless the music says otherwise. A note the
// instrument cannot reach at all is left alone and is drawn as the pitch it
// is — the engraver knows to do that, and silently moving it to a fret that
// does not exist would be worse than saying nothing.
func restringPart(p *model.Part) {
	if p.Tab == nil {
		return
	}
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				switch e := el.(type) {
				case *model.Note:
					restringNote(p.Tab, e)
				case *model.Chord:
					for _, n := range e.Notes {
						restringNote(p.Tab, n)
					}
				}
			}
		}
	}
}

func restringNote(t *model.Tablature, n *model.Note) {
	if n.String > 0 && n.String <= t.Lines() {
		// It already says where it is played. Honour it if it can be
		// honoured, and work it out again if the tuning has moved under
		// it — a part retuned to drop D has notes on the sixth string
		// that are now at a different fret.
		if fret, ok := t.Fretted(n.Pitch, n.String); ok {
			n.Fret = fret
			return
		}
	}
	if str, fret, ok := t.At(n.Pitch); ok {
		n.String, n.Fret = str, fret
	}
}

// handleTunings lists the tunings a tab staff can be written for.
func (a *App) handleTunings(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Name string `json:"name"`
		// Strings are the open strings as they are spoken, from the top
		// line of the tab down: a guitar is E4 B3 G3 D3 A2 E2.
		Strings []string `json:"strings"`
		Frets   int      `json:"frets"`
	}
	out := []entry{}
	for _, t := range model.Tunings() {
		e := entry{Name: t.Name, Frets: t.Frets}
		for _, s := range t.Strings {
			e.Strings = append(e.Strings, s.String())
		}
		out = append(out, e)
	}
	writeJSON(w, out)
}

// handleFret writes a note on a tab staff by the fret it is stopped at.
//
// The one thing a tab needs that a staff of pitches does not. On a staff
// the note is where it was clicked, and the pitch follows from the line; on
// a tab the line says which string and the number says how far up it, and
// neither alone is a note.
func (a *App) handleFret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1 // append
	}

	str, err := strconv.Atoi(q.Get("string"))
	if err != nil {
		http.Error(w, "which string?", http.StatusBadRequest)
		return
	}
	fret, err := strconv.Atoi(q.Get("fret"))
	if err != nil {
		http.Error(w, "which fret?", http.StatusBadRequest)
		return
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
	if p.Tab == nil {
		http.Error(w, "that staff is not written as tablature",
			http.StatusBadRequest)
		return
	}
	pitch, ok := p.Tab.PitchAt(str, fret)
	if !ok {
		http.Error(w, fretComplaint(p.Tab, str, fret), http.StatusBadRequest)
		return
	}

	v := m.Voices[0]

	// A second digit correcting the first. Frets run past nine and are
	// typed a digit at a time, so "1" then "2" is the twelfth fret and
	// not a first fret followed by a second — and the only way to know
	// which was meant is that the player is still typing. The editor
	// decides that and says so here; what this does is change the note
	// already written rather than add another.
	if q.Get("replace") == "1" {
		if at < 0 || at >= len(v.Elements) {
			http.Error(w, "there is nothing there to change", http.StatusBadRequest)
			return
		}
		note, ok := v.Elements[at].(*model.Note)
		if !ok {
			http.Error(w, "only a note is played on a string", http.StatusBadRequest)
			return
		}
		note.Pitch, note.String, note.Fret = pitch, str, fret
		writeJSON(w, map[string]any{
			"status": "ok", "at": at, "pitch": pitch.String(),
			"string": str, "fret": fret,
		})
		return
	}

	note := &model.Note{Pitch: pitch, Dur: dur, String: str, Fret: fret}
	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	v.Elements = append(v.Elements, nil)
	copy(v.Elements[at+1:], v.Elements[at:])
	v.Elements[at] = note

	writeJSON(w, map[string]any{
		"status": "ok", "at": at, "pitch": pitch.String(),
		"string": str, "fret": fret,
	})
}

// fretComplaint says what is wrong with a place on the neck, in the words
// of the instrument rather than of the program.
func fretComplaint(t *model.Tablature, str, fret int) string {
	switch {
	case str < 1 || str > t.Lines():
		return "this instrument has " + strconv.Itoa(t.Lines()) +
			" strings, so there is no string " + strconv.Itoa(str)
	case fret < 0:
		return "there is no fret " + strconv.Itoa(fret)
	default:
		return "this instrument has " + strconv.Itoa(t.Frets) +
			" frets, so there is no fret " + strconv.Itoa(fret)
	}
}

// handleRestring moves a note to another string without changing what it
// sounds.
//
// The same note is in several places on a neck and they do not sound or
// finger alike: the open B string and the fourth fret of the G are the same
// pitch and a different thing to play. Which one the music wants is a
// decision, and this is how it is made.
func (a *App) handleRestring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, _ := strconv.Atoi(q.Get("at"))

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locateForWriting(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if p.Tab == nil {
		http.Error(w, "that staff is not written as tablature",
			http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if at < 0 || at >= len(v.Elements) {
		http.Error(w, "there is nothing there to move", http.StatusBadRequest)
		return
	}
	note, ok := v.Elements[at].(*model.Note)
	if !ok {
		http.Error(w, "only a note is played on a string", http.StatusBadRequest)
		return
	}

	// Which string to move to: the one named, or a step along the neck
	// from where it is.
	want := note.String
	switch {
	case q.Get("string") != "":
		n, err := strconv.Atoi(q.Get("string"))
		if err != nil {
			http.Error(w, "which string?", http.StatusBadRequest)
			return
		}
		want = n
	case q.Get("move") == "up":
		// Up the page is towards string one, which is the high string.
		want = note.String - 1
	case q.Get("move") == "down":
		want = note.String + 1
	default:
		http.Error(w, "which string, or move=up or move=down",
			http.StatusBadRequest)
		return
	}

	fret, ok := p.Tab.Fretted(note.Pitch, want)
	if !ok {
		// Said in terms of the note, because the player is looking at a
		// note and not at a table: a string that cannot reach it is the
		// end of the neck or the wrong side of the pitch.
		http.Error(w, "the "+note.Pitch.String()+" cannot be played on string "+
			strconv.Itoa(want), http.StatusBadRequest)
		return
	}
	note.String, note.Fret = want, fret

	writeJSON(w, map[string]any{
		"status": "ok", "string": want, "fret": fret,
		"pitch": note.Pitch.String(),
	})
}

// tabInfo is what the editor is told about a staff written as tablature.
type tabInfo struct {
	Tuning  string   `json:"tuning"`
	Strings []string `json:"strings"`
	Frets   int      `json:"frets"`
}

// tabOf is the tablature of a staff, or nothing where it is a staff of
// pitches.
func tabOf(p *model.Part) *tabInfo {
	if p.Tab == nil {
		return nil
	}
	out := &tabInfo{Tuning: model.TuningName(p.Tab), Frets: p.Tab.Frets}
	for _, s := range p.Tab.Strings {
		out.Strings = append(out.Strings, s.String())
	}
	return out
}

// tuningLooksLike guesses the tuning a staff wants from what it is for, so
// that turning a bass staff into a tab does not offer it six strings.
func tuningLooksLike(p *model.Part) string {
	name := strings.ToLower(p.Sound + " " + p.Name)
	switch {
	case strings.Contains(name, "ukulele"):
		return "ukulele"
	case strings.Contains(name, "mandolin"):
		return "mandolin"
	case strings.Contains(name, "banjo"):
		return "banjo — five string"
	case strings.Contains(name, "bass"):
		return "bass"
	}
	return "guitar"
}
