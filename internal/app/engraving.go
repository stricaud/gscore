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
	Name  string `json:"name"`
	Label string `json:"label"`
	Group string `json:"group"`
	Unit  string `json:"unit,omitempty"`
	// Min and Max are the range this measurement is usually in, which is
	// what the window's arrows step through and what it says in the
	// tooltip. Advice: a house style is somebody's taste.
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	// Floor and Ceiling are what it may actually be — what can be drawn at
	// all — which is what the field itself is held to. Far outside Min and
	// Max on purpose: the window must not stop somebody typing a number
	// they want.
	Floor   float64 `json:"floor"`
	Ceiling float64 `json:"ceiling"`
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

	// Whichever document is being looked at. A part is engraved to be read
	// at a stand and the score to be read at a desk, so they want different
	// answers — and the window that sets them can only sensibly set them
	// for the thing on the screen.
	into := &a.score.Engraving
	if sh := a.showingSheet(); sh != nil {
		into = &sh.Engraving
	}

	if _, ok := q["reset"]; ok {
		*into = nil
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
		// What can be drawn, not what is usual. A number outside the usual
		// range is somebody's house style and none of this endpoint's
		// business; a negative space or one of some thousands of staff
		// spaces is a drawing nothing can show, and that is refused.
		lo, hi := k.Limits()
		if v < lo || v > hi {
			http.Error(w, k.Label+" has to be between "+
				trimFloat(lo)+" and "+trimFloat(hi), http.StatusBadRequest)
			return
		}
		changed[name] = v
	}

	if len(changed) > 0 && *into == nil {
		*into = map[string]float64{}
	}
	for name, v := range changed {
		if math.IsNaN(v) {
			delete(*into, name)
			continue
		}
		(*into)[name] = v
	}
	if len(*into) == 0 {
		*into = nil
	}
	a.writeEngraving(w)
}

// writeEngraving sends the list. The caller holds the lock.
func (a *App) writeEngraving(w http.ResponseWriter) {
	opt := engrave.DefaultOptions()
	// What is being looked at, so that the window shows the numbers it
	// would change rather than the score's while a part is on the screen.
	have := a.score.Engraving
	of := ""
	if sh := a.showingSheet(); sh != nil {
		have, of = sh.Engraving, sh.Name
	}
	out := make([]engravingKnob, 0, len(engrave.Knobs))
	for _, k := range engrave.Knobs {
		v, set := have[k.Name]
		if !set {
			v = k.Get(opt)
		} else {
			v = k.Clamp(v)
		}
		lo, hi := k.Limits()
		out = append(out, engravingKnob{
			Name: k.Name, Label: k.Label, Group: k.Group, Unit: k.Unit,
			Min: k.Min, Max: k.Max, Floor: lo, Ceiling: hi,
			Value: v, Default: k.Get(opt), Set: set,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	// of names the part these belong to, where a part is what is being
	// looked at, so the window can say whose measurements it is showing.
	json.NewEncoder(w).Encode(map[string]any{"knobs": out, "of": of})
}

// trimFloat writes a number the way a person would: 0.4 and not 0.400000.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
