/*
 * numbering.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/json"
	"net/http"
	"strconv"
)

/*
 * How the bars are numbered.
 *
 * Numbering was on or off, and where the numbers fell was the engraver's
 * decision: the first bar of each system on a page, every bar in the
 * editor's strip. That is right for a song and not for a long score, where
 * rehearsing is most of what the numbers are for — a conductor saying
 * "from a hundred and twelve" wants them often enough to find and boxed
 * enough to see.
 *
 * Three things a score can say, and all three are a decision about what
 * the page says rather than a measurement to tune, which is why they are
 * not in the engraving settings beside them:
 *
 *	how often  every nth bar, instead of wherever the engraver puts them
 *	boxed      a rectangle round each number, as a rehearsal mark has
 *	restart    begin counting again at a bar, at a number you choose
 *
 * The restart belongs to a bar and the other two to the score, which is
 * why one of them takes an "at" and the others do not.
 */

// numberingState is what the editor is told.
type numberingState struct {
	Every int  `json:"every"`
	Boxed bool `json:"boxed"`
	// Restarts are the bars where the count begins again, by bar index,
	// and the number each begins at.
	Restarts map[string]int `json:"restarts"`
}

// handleNumbering reads or changes how this score's bars are numbered.
func (a *App) handleNumbering(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.numberingSet(w, r)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.writeNumbering(w)
}

func (a *App) numberingSet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	if v, ok := q["every"]; ok && len(v) > 0 {
		n, err := strconv.Atoi(v[0])
		if err != nil || n < 0 {
			http.Error(w, v[0]+" is not how often", http.StatusBadRequest)
			return
		}
		// Not a limit anybody will reach; it is there so that a typo of
		// nine digits does not silently mean "never number anything".
		if n > 999 {
			http.Error(w, "every 999 bars is the most that can be asked for",
				http.StatusBadRequest)
			return
		}
		a.score.Numbering.Every = n
	}
	if v, ok := q["boxed"]; ok && len(v) > 0 {
		a.score.Numbering.Boxed = v[0] == "1" || v[0] == "true"
	}

	// A restart, which belongs to a bar rather than to the score.
	if v, ok := q["at"]; ok && len(v) > 0 {
		at, err := strconv.Atoi(v[0])
		if err != nil || at < 0 || at >= a.score.MeasureCount() {
			http.Error(w, "there is no bar "+v[0], http.StatusBadRequest)
			return
		}
		raw := q.Get("number")
		var number *int
		// No number, or "none", takes the restart away again.
		if raw != "" && raw != "none" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				http.Error(w, raw+" is not a bar number", http.StatusBadRequest)
				return
			}
			if n > 99999 {
				http.Error(w, "that is not a bar number anybody would print",
					http.StatusBadRequest)
				return
			}
			number = &n
		}
		// On every staff, because "bar forty" has to mean the same bar to
		// everybody in the room. The engraver reads it from whichever
		// staff carries it; writing it on one and not the rest would make
		// the parts disagree with the score the moment one was edited.
		for _, p := range a.score.Parts {
			if at < len(p.Measures) {
				p.Measures[at].NumberFrom = number
			}
		}
	}

	a.writeNumbering(w)
}

// writeNumbering sends the state. The caller holds the lock.
func (a *App) writeNumbering(w http.ResponseWriter) {
	out := numberingState{
		Every:    a.score.Numbering.Every,
		Boxed:    a.score.Numbering.Boxed,
		Restarts: map[string]int{},
	}
	for i := 0; i < a.score.MeasureCount(); i++ {
		for _, p := range a.score.Parts {
			if i < len(p.Measures) && p.Measures[i].NumberFrom != nil {
				out.Restarts[strconv.Itoa(i)] = *p.Measures[i].NumberFrom
				break
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
