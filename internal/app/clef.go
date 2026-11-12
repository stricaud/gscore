/*
 * clef.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A clef written into the music.
 *
 * Not the staff's own clef, which says how the whole part is read and is set
 * in Score setup: this is the sign a player meets partway through — a cello
 * climbing into the tenor clef for a phrase and dropping back, a pianist's
 * left hand going up into the treble for a page — and it is written where
 * the change happens, which is as often in the middle of a bar as at the
 * start of one.
 *
 * The model has carried both for a long time: Measure.Clef for a change at a
 * barline and Marks.Clef for one inside a bar, and the engraver draws each
 * where it belongs. Nothing could write either, so a part that needed a
 * second clef could only be read into gscore, never typed.
 */

package app

import (
	"net/http"
	"strconv"

	"github.com/stricaud/gscore/internal/model"
)

// handleClef writes a change of clef at the cursor, or takes one away.
func (a *App) handleClef(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	want := q.Get("clef")
	clef, ok := clefByStringName[want]
	if !ok && want != "none" {
		http.Error(w, "unknown clef "+want, http.StatusBadRequest)
		return
	}

	// Where the cursor is. A selection names the element it is on; without
	// one the bar is enough, and the change goes at its head.
	part, measure, index := 0, 0, 0
	if sel := q.Get("sel"); sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		t := targets[0]
		part, measure, index = t.part, t.measure, t.index
	} else {
		part, _ = strconv.Atoi(q.Get("part"))
		measure, _ = strconv.Atoi(q.Get("measure"))
		// "at" counts the elements written so far, so the element the
		// cursor is on is the one before it.
		if at, err := strconv.Atoi(q.Get("at")); err == nil && at > 0 {
			index = at - 1
		}
	}

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(part, measure)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if index < 0 {
		index = 0
	}
	if len(m.Voices) == 0 || len(m.Voices[0].Elements) == 0 {
		index = 0
	} else if index >= len(m.Voices[0].Elements) {
		index = len(m.Voices[0].Elements) - 1
	}

	if want == "none" {
		gone := false
		if m.Clef != nil {
			m.Clef, gone = nil, true
		}
		if len(m.Voices) > 0 {
			for _, el := range m.Voices[0].Elements {
				if mk := marksOfElement(el); mk != nil && mk.Clef != nil {
					mk.Clef, gone = nil, true
				}
			}
		}
		if !gone {
			http.Error(w, "there is no clef change in that measure",
				http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"changed": 1, "where": "removed"})
		return
	}

	// A clef that changes nothing is a sign nobody can act on, so it is not
	// written: asking for the clef the staff is already read in at that
	// point takes away whatever change was there instead.
	if index == 0 && measure == 0 {
		// The head of the piece is the staff's own clef. Written here as
		// well so that the one window can do both, rather than sending
		// somebody to another one for the first bar.
		p.Clef = clef
		writeJSON(w, map[string]any{"changed": 1, "where": "staff"})
		return
	}

	where := "measure"
	if index == 0 {
		// At the head of a bar the sign is drawn at the barline, which is
		// where a reader looks for it.
		if a.clefInForce(p, measure, 0) == clef {
			m.Clef = nil
		} else {
			c := clef
			m.Clef = &c
		}
	} else {
		el := m.Voices[0].Elements[index]
		mk := marksOfElement(el)
		if mk == nil {
			http.Error(w, "a clef cannot be written on that",
				http.StatusBadRequest)
			return
		}
		if a.clefInForce(p, measure, index) == clef {
			mk.Clef = nil
		} else {
			c := clef
			mk.Clef = &c
		}
		where = "note"
	}
	writeJSON(w, map[string]any{"changed": 1, "where": where})
}

// clefInForce is what the staff is read in just before this spot: its own
// clef, and then every change written before it.
func (a *App) clefInForce(p *model.Part, measure, index int) model.Clef {
	c := p.Clef
	for i := 0; i <= measure && i < len(p.Measures); i++ {
		m := p.Measures[i]
		if m.Clef != nil {
			c = *m.Clef
		}
		if len(m.Voices) == 0 {
			continue
		}
		for j, el := range m.Voices[0].Elements {
			if i == measure && j >= index {
				break
			}
			if mk := marksOfElement(el); mk != nil && mk.Clef != nil {
				c = *mk.Clef
			}
		}
	}
	return c
}
