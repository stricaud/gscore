/*
 * systems.go
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
	"sort"
	"strconv"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Where the lines of music end.
 *
 * The engraver fits as many bars to a line as will go, and that is right
 * until somebody wants otherwise: four bars to a line through a study so
 * every one is read the same way, a page turn moved off a passage nobody
 * can spare a hand for, a last line of one bar evened out because it looks
 * like a mistake. None of it can be worked out from the notes — it is a
 * decision, and until now there was nowhere to write it down.
 *
 * One mechanism does both halves of what that needs. A break says "a new
 * line starts here", which locks the line before it: the engraver can no
 * longer move those bars about, because where they end has been said. So
 * "fit four bars to a line" is breaks every four bars, and "stop moving
 * them" is the same breaks left where they are.
 *
 * Whose lines, though. A part turns pages in different places from the
 * score — that is most of what makes a part playable, since a turn has to
 * fall where there are bars of rest to turn in — so these are written on
 * whichever document is being looked at. The window edits what is on the
 * screen, which is the only rule anybody could guess: somebody who has
 * turned to the oboe part and moves a line break has moved the oboe's.
 */

// systemsState is what the editor is told.
type systemsState struct {
	// Breaks are the bars that begin a line, by bar index.
	Breaks []int `json:"breaks"`
	// Of names the part these belong to, where a part is what is being
	// looked at. Empty for the score itself. The window says whose lines it
	// is about, so that moving one is not a surprise.
	Of string `json:"of,omitempty"`
}

// handleSystems reads or changes where the lines of music end.
func (a *App) handleSystems(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.systemsSet(w, r)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.writeSystems(w)
}

func (a *App) systemsSet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	bars := a.score.MeasureCount()
	if bars == 0 {
		http.Error(w, "there is nothing written yet", http.StatusBadRequest)
		return
	}

	// Clearing comes first, so that "fit" and "clear" together mean
	// "forget what was there and lay it out this way" rather than leaving
	// the old breaks among the new ones.
	if _, ok := q["clear"]; ok {
		from, to, err := barRange(q, bars)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.setBreaks(from, to, 0)
	}

	if v, ok := q["fit"]; ok && len(v) > 0 {
		every, err := strconv.Atoi(v[0])
		if err != nil || every < 1 {
			http.Error(w, v[0]+" is not a number of bars", http.StatusBadRequest)
			return
		}
		if every > bars {
			http.Error(w, "there are not that many bars", http.StatusBadRequest)
			return
		}
		from, to, err := barRange(q, bars)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.setBreaks(from, to, every)
	}

	// One break, set or taken away.
	if v, ok := q["at"]; ok && len(v) > 0 {
		at, err := strconv.Atoi(v[0])
		if err != nil || at < 0 || at >= bars {
			http.Error(w, "there is no bar "+v[0], http.StatusBadRequest)
			return
		}
		// The first bar of the piece already begins a line, so a break
		// there says nothing and storing one would be a break that can
		// never be seen or removed.
		if at == 0 {
			http.Error(w, "the first bar already begins a line",
				http.StatusBadRequest)
			return
		}
		a.setBreakAt(at, q.Get("on") != "0")
	}

	a.writeSystems(w)
}

// setBreaks lays out a stretch of bars: a break every `every` bars from
// the first, or none at all where every is zero.
//
// Never on the first bar of the piece, which begins a line already.
func (a *App) setBreaks(from, to, every int) {
	for mi := from; mi <= to; mi++ {
		a.setBreakAt(mi, every > 0 && mi > from && (mi-from)%every == 0)
	}
}

// showingSheet is the part being looked at, or nil where it is the score.
// The caller holds the lock.
func (a *App) showingSheet() *model.Sheet {
	if a.sheet < 0 || a.sheet >= len(a.score.Sheets) {
		return nil
	}
	return &a.score.Sheets[a.sheet]
}

// setBreakAt says whether a line begins at a bar, on whichever document is
// being looked at.
//
// On the score it is written on every staff, because a line of music is a
// line on every staff at once. On a part it is written on the part, which
// has one list of its own for exactly this reason.
func (a *App) setBreakAt(at int, on bool) {
	if at <= 0 {
		return
	}
	if sh := a.showingSheet(); sh != nil {
		was := sh.Breaks
		sh.Breaks = sh.Breaks[:0]
		for _, b := range was {
			if b != at {
				sh.Breaks = append(sh.Breaks, b)
			}
		}
		if on {
			sh.Breaks = append(sh.Breaks, at)
		}
		sort.Ints(sh.Breaks)
		return
	}
	for _, p := range a.score.Parts {
		if at < len(p.Measures) {
			p.Measures[at].SystemBreak = on
		}
	}
}

// barRange reads the stretch a request is about, which is the whole piece
// where it does not say.
func barRange(q map[string][]string, bars int) (int, int, error) {
	from, to := 0, bars-1
	read := func(name string, into *int) error {
		v, ok := q[name]
		if !ok || len(v) == 0 || v[0] == "" {
			return nil
		}
		n, err := strconv.Atoi(v[0])
		if err != nil || n < 0 || n >= bars {
			return &barError{v[0]}
		}
		*into = n
		return nil
	}
	if err := read("from", &from); err != nil {
		return 0, 0, err
	}
	if err := read("to", &to); err != nil {
		return 0, 0, err
	}
	if to < from {
		from, to = to, from
	}
	return from, to, nil
}

type barError struct{ what string }

func (e *barError) Error() string { return "there is no bar " + e.what }

// writeSystems sends where the lines begin. The caller holds the lock.
func (a *App) writeSystems(w http.ResponseWriter) {
	out := systemsState{Breaks: []int{}}
	if sh := a.showingSheet(); sh != nil {
		out.Breaks = append(out.Breaks, sh.Breaks...)
		out.Of = sh.Name
	} else {
		for i := 0; i < a.score.MeasureCount(); i++ {
			for _, p := range a.score.Parts {
				if i < len(p.Measures) && p.Measures[i].SystemBreak {
					out.Breaks = append(out.Breaks, i)
					break
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
