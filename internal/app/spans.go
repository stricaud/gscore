/*
 * spans.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The marks a score has and the editor could not write.
 *
 * gscore reads all of these — from MusicXML, from Dorico, from NoteWorthy —
 * draws them, plays them and writes them back out, and there was no way to
 * put one into a score by hand: a trill, the pedal, the stroke up a chord
 * and the slide from one note to another. A score opened, edited and saved
 * kept whatever it arrived with and could gain none of it, which is a
 * reader rather than an editor.
 *
 * They are four routes rather than one because they are four marks, and
 * because that is how the rest of them are written: /api/hairpin, /api/
 * octave, /api/breath. A caller driving gscore from a script reads the list
 * of routes as the list of things it can write.
 */

package app

import (
	"net/http"

	"github.com/stricaud/gscore/internal/model"
)

// handleTrill writes "tr" over the selection, with the wavy line where more
// than one note is picked up.
//
// A trill on a single note is both ends at once, which is how "tr" over one
// notehead is written; over a passage it opens on the first and closes on
// the last, and the line is drawn between them.
func (a *App) handleTrill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch q.Get("trill") {
	case "none":
		for _, m := range marks {
			m.Trill = nil
		}
	case "", "on", "trill":
		if len(marks) == 1 {
			marks[0].Trill = &model.Trill{Start: true, Stop: true}
			break
		}
		for i, m := range marks {
			m.Trill = &model.Trill{Start: i == 0, Stop: i == len(marks)-1}
		}
	default:
		http.Error(w, "a trill is on or none", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handlePedal puts the sustaining pedal down, lifts it, or flicks it.
//
// Down on the first note of the selection and up on the last, which is what
// picking up a passage and asking for the pedal means. "change" is the
// notch in the middle of a line — released and pressed again on one note,
// without lifting the phrase — so it goes on every note picked up, since
// that is the only way to ask for several of them.
func (a *App) handlePedal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// The old form, "Ped." under the first note and a star under the last,
	// which a great deal of printed music has.
	text := q.Get("style") == "text"
	switch q.Get("pedal") {
	case "none":
		for _, m := range marks {
			m.Pedal = nil
		}
	case "change":
		for _, m := range marks {
			m.Pedal = &model.PedalMark{Change: true, Text: text}
		}
	case "", "on", "down":
		if len(marks) == 1 {
			marks[0].Pedal = &model.PedalMark{Start: true, Stop: true, Text: text}
			break
		}
		for i, m := range marks {
			switch {
			case i == 0:
				m.Pedal = &model.PedalMark{Start: true, Text: text}
			case i == len(marks)-1:
				m.Pedal = &model.PedalMark{Stop: true, Text: text}
			default:
				m.Pedal = nil
			}
		}
	default:
		http.Error(w, "the pedal goes down, changes, or comes up",
			http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleArpeggio rolls a chord: the wavy stroke in front of it that says to
// spread the notes rather than strike them together.
//
// On each chord picked up, because a run of rolled chords is a run of
// strokes and not one long one.
func (a *App) handleArpeggio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch q.Get("arpeggio") {
	case "none":
		for _, m := range marks {
			m.Arpeggio = nil
		}
	case "down":
		for _, m := range marks {
			m.Arpeggio = &model.Arpeggio{Down: true}
		}
	case "", "on", "up":
		for _, m := range marks {
			m.Arpeggio = &model.Arpeggio{}
		}
	default:
		http.Error(w, "a chord is rolled up or down", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleGlissando draws the slide from one note to the next.
//
// It belongs to a pair: the note it leaves and the note it arrives at. One
// note picked up is the first of the pair and the line runs to whatever
// follows, which is what a player reads either way; a passage picked up
// opens it on the first and closes it on the last.
func (a *App) handleGlissando(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// A glissando is drawn as a straight line with "gliss." along it and a
	// portamento as a wavy one; which of the two is the word, and the shape
	// follows it.
	wavy := q.Get("line") == "wavy"
	word := q.Get("text")
	switch q.Get("glissando") {
	case "none":
		for _, m := range marks {
			m.Glissando = nil
		}
	case "", "on", "gliss":
		if len(marks) == 1 {
			marks[0].Glissando = &model.Glissando{
				Start: true, Wavy: wavy, Text: word,
			}
			break
		}
		for i, m := range marks {
			if i > 0 && i < len(marks)-1 {
				m.Glissando = nil
				continue
			}
			m.Glissando = &model.Glissando{
				Start: i == 0, Stop: i == len(marks)-1,
				Wavy: wavy, Text: word,
			}
		}
	default:
		http.Error(w, "a glissando is on or none", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}
