/*
 * engraving.go
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
	"math"
	"net/http"
	"strconv"

	"github.com/stricaud/gscore/internal/engrave"
)

/*
 * How this score is engraved.
 *
 * The engraver works from two tables of measurements — how wide a quarter
 * note's slot is, how heavy a beam is, how far apart two staves sit — and
 * they were settled in Go with no way to change them. They are exactly what
 * an engraver argues about, and the right place to settle them is per
 * score: a song and an orchestral part want different answers, and the
 * answer belongs in the file rather than in a preference that follows the
 * person to the next piece.
 *
 * Nothing here knows what the measurements are. engrave.Knobs is the one
 * list of them, carrying each one's name, its bounds and how to read and
 * write it, and this endpoint hands that list to the editor so the dialog
 * builds itself. A measurement added to the engraver appears in the window
 * without a line being written here.
 */

// engravingKnob is one measurement as the editor sees it.
type engravingKnob struct {
	Name  string  `json:"name"`
	Label string  `json:"label"`
	Group string  `json:"group"`
	Unit  string  `json:"unit,omitempty"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	// Value is what this score engraves at, and Default what it would
	// engrave at if it said nothing. The dialog shows both: a field with
	// no way of telling whether 2.8 was chosen or merely inherited is a
	// field nobody dares change.
	Value   float64 `json:"value"`
	Default float64 `json:"default"`
	// Set says this score has an answer of its own for this one.
	Set bool `json:"set"`
}

// handleEngraving reads or changes what this score is engraved by.
//
// GET hands over the whole list with the score's values in it. POST takes
// any number of them as query parameters named after the knobs, and
// "reset" with no value puts the lot back to the house style.
func (a *App) handleEngraving(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.engravingSet(w, r)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.writeEngraving(w)
}

func (a *App) engravingSet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	if _, ok := q["reset"]; ok {
		a.score.Engraving = nil
		a.writeEngraving(w)
		return
	}

	// Read every one asked for before writing any of them, so a request
	// with one bad number in it changes nothing rather than half of what
	// it named. Half-applied is the worst outcome here: the person sees
	// the page move, sees the refusal, and has no way to know which of
	// the two states they are now in.
	changed := map[string]float64{}
	for name := range q {
		k, ok := engrave.KnobNamed(name)
		if !ok {
			http.Error(w, name+" is not something that can be set",
				http.StatusBadRequest)
			return
		}
		raw := q.Get(name)
		// An empty value means "stop having an answer of my own for this
		// one", which is how a single field is put back without resetting
		// the rest.
		if raw == "" {
			changed[name] = math.NaN()
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			http.Error(w, raw+" is not a measurement", http.StatusBadRequest)
			return
		}
		if v < k.Min || v > k.Max {
			http.Error(w, k.Label+" has to be between "+
				trimFloat(k.Min)+" and "+trimFloat(k.Max), http.StatusBadRequest)
			return
		}
		changed[name] = v
	}

	if len(changed) > 0 && a.score.Engraving == nil {
		a.score.Engraving = map[string]float64{}
	}
	for name, v := range changed {
		if math.IsNaN(v) {
			delete(a.score.Engraving, name)
			continue
		}
		a.score.Engraving[name] = v
	}
	if len(a.score.Engraving) == 0 {
		a.score.Engraving = nil
	}
	a.writeEngraving(w)
}

// writeEngraving sends the list. The caller holds the lock.
func (a *App) writeEngraving(w http.ResponseWriter) {
	opt := engrave.DefaultOptions()
	out := make([]engravingKnob, 0, len(engrave.Knobs))
	for _, k := range engrave.Knobs {
		v, set := a.score.Engraving[k.Name]
		if !set {
			v = k.Get(opt)
		} else {
			v = k.Clamp(v)
		}
		out = append(out, engravingKnob{
			Name: k.Name, Label: k.Label, Group: k.Group, Unit: k.Unit,
			Min: k.Min, Max: k.Max,
			Value: v, Default: k.Get(opt), Set: set,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"knobs": out})
}

// trimFloat writes a number the way a person would: 0.4 and not 0.400000.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
