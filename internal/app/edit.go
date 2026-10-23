/*
 * edit.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// Editing happens on the server, against the model, not in the browser against
// the drawing. The engraver is the only thing that knows where a note went, so
// it is also the thing that says where a click landed; everything after that is
// a change to the music, which is then drawn again from scratch.
//
// That is the whole point of keeping geometry out of the model: an edit never
// has to patch coordinates, because there are none to patch.

// handleHits reports the clickable regions for the current view.
func (a *App) handleHits(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	opt := a.engraveOpt
	a.mu.RUnlock()

	switch r.URL.Query().Get("mode") {
	case "strip":
		opt.Mode = engrave.Strip
	case "page":
		opt.Mode = engrave.Paginated
	}

	a.mu.RLock()
	lay := engrave.Engrave(a.score, opt)
	a.mu.RUnlock()

	writeJSON(w, map[string]any{
		"hits":        lay.Hits,
		"barlines":    lay.Barlines,
		"staffHeight": 4.0,
	})
}

var durationNames = map[string]model.Duration{
	"breve":     {Num: 2, Den: 1},
	"whole":     {Num: 1, Den: 1},
	"half":      {Num: 1, Den: 2},
	"quarter":   {Num: 1, Den: 4},
	"eighth":    {Num: 1, Den: 8},
	"sixteenth": {Num: 1, Den: 16},
	"32nd":      {Num: 1, Den: 32},
	"64th":      {Num: 1, Den: 64},
}

var accidentalByName = map[string]model.Accidental{
	"":             model.NoAccidental,
	"none":         model.NoAccidental,
	"natural":      model.Natural,
	"sharp":        model.Sharp,
	"flat":         model.Flat,
	"double sharp": model.DoubleSharp,
	"double flat":  model.DoubleFlat,
}

var clefByStringName = map[string]model.Clef{
	"treble": model.Treble, "bass": model.Bass, "alto": model.Alto,
	"tenor": model.Tenor, "percussion": model.Percussion,
}

// handleInsert puts a note or a rest into a measure.
func (a *App) handleInsert(w http.ResponseWriter, r *http.Request) {
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
	step, _ := strconv.Atoi(q.Get("step"))

	dur, ok := durationNames[q.Get("dur")]
	if !ok {
		dur = model.Duration{Num: 1, Den: 4}
	}
	acc := accidentalByName[q.Get("accidental")]

	a.mu.Lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]

	var el model.Element
	switch {
	case q.Get("kind") == "rest":
		el = &model.Rest{Dur: dur}
	case p.Clef == model.Percussion:
		// A drum staff's lines carry no pitches: a position names an
		// instrument. The note still has a pitch, because everything else —
		// stems, beams, spacing — works from staff positions, but what
		// sounds comes from the kit and not from the pitch.
		drum, ok := model.PercussionFor(step)
		if !ok {
			http.Error(w, "nothing is written at that position", http.StatusBadRequest)
			return
		}
		el = &model.Note{
			Pitch:      pitchForStep(p.Clef, drum.Step),
			Dur:        dur,
			Head:       drum.Head,
			Percussion: drum.Key,
		}
	default:
		// The staff step is what was clicked; the clef turns it into a pitch,
		// and the key signature decides what it sounds like when no accidental
		// is written on it.
		pitch := pitchForStep(p.Clef, step)
		if acc != model.NoAccidental {
			pitch.Alter = acc.Semitones()
		} else {
			pitch.Alter = p.Key.AlterFor(pitch.Step)
		}
		el = &model.Note{Pitch: pitch, Dur: dur, Accidental: acc}
	}

	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}

	// Writing at the end of a bar that is already full means the next bar.
	// Without this the music piles up behind the barline and there is no way
	// to reach the measure after it, which is the one you are looking at.
	if at == len(v.Elements) && v.Duration().Cmp(p.Time.BarDuration()) >= 0 {
		measIdx++
		m = measureAt(p, measIdx)
		v = m.Voices[0]
		at = 0
	}

	v.Elements = append(v.Elements, nil)
	copy(v.Elements[at+1:], v.Elements[at:])
	v.Elements[at] = el

	// Inserting in the middle can push the bar past its time signature; what
	// no longer fits moves on rather than being squeezed in.
	spill(p, measIdx)

	// The element may now be in a later measure than the one written to, so
	// the answer says where it actually is and the cursor follows it there.
	part, idx := findElement(p, measIdx, el)
	writeJSON(w, map[string]any{
		"status": "ok", "measure": part, "at": idx, "index": idx,
	})
}

// measureAt returns a part's nth measure, making it and any before it if the
// part does not reach that far yet.
func measureAt(p *model.Part, n int) *model.Measure {
	for len(p.Measures) <= n {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{}}, RightBarline: model.BarSingle,
		})
	}
	m := p.Measures[n]
	if len(m.Voices) == 0 {
		m.Voices = []*model.Voice{{}}
	}
	return m
}

// spill moves whatever overflows a measure into the one after it, making new
// measures as it goes.
//
// Notes are moved whole and never divided: splitting a note across a barline
// means tying the halves together, and a tie the user did not ask for is a
// change to the music rather than to its layout.
func spill(p *model.Part, from int) {
	bar := p.Time.BarDuration()
	if bar.IsZero() {
		return
	}
	for i := from; i < len(p.Measures); i++ {
		v := p.Measures[i].Voices[0]
		for len(v.Elements) > 1 && v.Duration().Cmp(bar) > 0 {
			last := v.Elements[len(v.Elements)-1]
			v.Elements = v.Elements[:len(v.Elements)-1]
			next := measureAt(p, i+1).Voices[0]
			next.Elements = append([]model.Element{last}, next.Elements...)
		}
	}
}

// findElement says where an element ended up after a spill: which measure
// holds it, and the cursor position just after it.
func findElement(p *model.Part, from int, el model.Element) (measure, at int) {
	for i := from; i < len(p.Measures); i++ {
		for j, e := range p.Measures[i].Voices[0].Elements {
			if e == el {
				return i, j + 1
			}
		}
	}
	return from, 0
}

// handleAddPitch adds a notehead to a note or a chord already written,
// which is how a chord is built.
//
// gscore made chords with Ctrl held, and chords.c refused two noteheads of
// the same pitch; both are kept, the modifier having become a click in the
// same column at a different height.
func (a *App) handleAddPitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	index, _ := strconv.Atoi(q.Get("index"))
	step, _ := strconv.Atoi(q.Get("step"))
	acc := accidentalByName[q.Get("accidental")]

	a.mu.Lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if index < 0 || index >= len(v.Elements) {
		http.Error(w, "there is no note there", http.StatusBadRequest)
		return
	}

	pitch := pitchForStep(p.Clef, step)
	if acc != model.NoAccidental {
		pitch.Alter = acc.Semitones()
	} else {
		pitch.Alter = p.Key.AlterFor(pitch.Step)
	}
	add := &model.Note{Pitch: pitch, Dur: v.Elements[index].Duration(), Accidental: acc}

	switch e := v.Elements[index].(type) {
	case *model.Note:
		if e.Pitch.Step == pitch.Step && e.Pitch.Octave == pitch.Octave {
			http.Error(w, "a chord cannot hold the same pitch twice", http.StatusBadRequest)
			return
		}
		// The marks belong to the sound, not to one of its noteheads, so
		// they move to the chord: an accent on a note stays an accent when a
		// second pitch joins it.
		ch := &model.Chord{Notes: []*model.Note{{
			Pitch: e.Pitch, Dur: e.Dur, Accidental: e.Accidental,
		}, add}, Dur: e.Dur, Marks: e.Marks}
		sortChord(ch)
		v.Elements[index] = ch
	case *model.Chord:
		for _, n := range e.Notes {
			if n.Pitch.Step == pitch.Step && n.Pitch.Octave == pitch.Octave {
				http.Error(w, "a chord cannot hold the same pitch twice", http.StatusBadRequest)
				return
			}
		}
		e.Notes = append(e.Notes, add)
		sortChord(e)
	default:
		http.Error(w, "only a note can be made into a chord", http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"status": "ok"})
}

// sortChord puts the noteheads in order from the bottom up, which is the
// order the engraver's second-interval rule assumes.
func sortChord(c *model.Chord) {
	sort.SliceStable(c.Notes, func(i, j int) bool {
		return c.Notes[i].Pitch.DiatonicIndex() < c.Notes[j].Pitch.DiatonicIndex()
	})
}

// pitchForStep inverts the staff position back into a pitch.
func pitchForStep(c model.Clef, step int) model.Pitch {
	return model.PitchFromDiatonicIndex(step - clefOffset(c))
}

// clefOffset mirrors the engraver's table. It is small enough to restate and
// keeping it here avoids the editor depending on the engraver's internals.
func clefOffset(c model.Clef) int {
	switch c {
	case model.Bass:
		return 10
	case model.Alto:
		return 4
	case model.Tenor:
		return 6
	case model.Percussion:
		return 4
	}
	return -2
}

var barlineByName = map[string]model.BarlineStyle{
	"single": model.BarSingle, "double": model.BarDouble, "final": model.BarFinal,
	"repeat-start": model.BarRepeatStart, "repeat-end": model.BarRepeatEnd,
	"repeat-both": model.BarRepeatBoth, "none": model.BarNone,
}

// handleBarline changes how a measure ends.
func (a *App) handleBarline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	style, ok := barlineByName[q.Get("style")]
	if !ok {
		http.Error(w, "unknown barline style", http.StatusBadRequest)
		return
	}
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, _, err := a.locate(partIdx, measIdx); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// A barline runs down the whole system. Changing it on one staff and not
	// the others would draw a repeat that only one player is asked to take.
	for _, p := range a.score.Parts {
		if measIdx < len(p.Measures) {
			p.Measures[measIdx].RightBarline = style
		}
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleDelete removes one element.
func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, _ := strconv.Atoi(q.Get("at"))

	a.mu.Lock()
	defer a.mu.Unlock()

	_, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if at < 0 || at >= len(v.Elements) {
		http.Error(w, "nothing there to delete", http.StatusBadRequest)
		return
	}
	v.Elements = append(v.Elements[:at], v.Elements[at+1:]...)
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleDeleteSelection removes everything in a dragged selection.
func (a *App) handleDeleteSelection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	targets, err := parseTargets(r.URL.Query().Get("sel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Group by measure and remove from the back, so that deleting one element
	// does not shift the index of the next one still to go.
	byMeasure := map[[2]int][]int{}
	for _, t := range targets {
		k := [2]int{t.part, t.measure}
		byMeasure[k] = append(byMeasure[k], t.index)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	removed := 0
	for k, idx := range byMeasure {
		_, m, err := a.locate(k[0], k[1])
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.IntSlice(idx)))
		v := m.Voices[0]
		for _, i := range idx {
			if i < 0 || i >= len(v.Elements) {
				continue
			}
			v.Elements = append(v.Elements[:i], v.Elements[i+1:]...)
			removed++
		}
	}
	writeJSON(w, map[string]int{"removed": removed})
}

// handleAddMeasure appends an empty measure to every part, so the staves stay
// the same length and their barlines keep lining up.
func (a *App) handleAddMeasure(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, p := range a.score.Parts {
		p.AppendMeasure()
	}
	writeJSON(w, map[string]int{"measures": len(a.score.Parts[0].Measures)})
}

// locate finds a measure, making sure it has a voice to put things in.
func (a *App) locate(partIdx, measIdx int) (*model.Part, *model.Measure, error) {
	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		return nil, nil, errBadPart
	}
	p := a.score.Parts[partIdx]
	if measIdx < 0 || measIdx >= len(p.Measures) {
		return nil, nil, errBadMeasure
	}
	m := p.Measures[measIdx]
	if len(m.Voices) == 0 {
		m.Voices = []*model.Voice{{}}
	}
	return p, m, nil
}

type editError string

func (e editError) Error() string { return string(e) }

const (
	errBadPart    = editError("no such staff")
	errBadMeasure = editError("no such measure")
)

var stemByName = map[string]model.Stem{
	"auto": model.StemAuto, "up": model.StemUp, "down": model.StemDown,
}

// handleStem sets, or clears, the stem direction of the element at the cursor.
// handleAccidental writes an accidental on what is selected.
//
// Choosing a sharp from the palette says what the *next* note will be; a note
// already on the staff has to be told separately, and clicking the accidental
// while it is selected is how that is said.
func (a *App) handleAccidental(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("accidental")
	acc, ok := accidentalByName[name]
	if !ok {
		http.Error(w, "unknown accidental", http.StatusBadRequest)
		return
	}

	notes, err := a.notesOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, n := range notes {
		n.Accidental = acc
		// The written accidental decides how the note sounds. Taking one away
		// hands that decision back to the key signature.
		if acc == model.NoAccidental {
			n.Pitch.Alter = n.key.AlterFor(n.Pitch.Step)
		} else {
			n.Pitch.Alter = acc.Semitones()
		}
	}
	writeJSON(w, map[string]int{"changed": len(notes)})
}

// keyedNote is a note together with the key signature in force where it sits,
// which is what an accidental has to be removed back to.
type keyedNote struct {
	*model.Note
	key model.KeySignature
}

// notesOf resolves a selection to the notes in it, a chord counting as all of
// its noteheads: an accidental put on a chord belongs to the whole chord.
func (a *App) notesOf(q url.Values) ([]keyedNote, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var out []keyedNote
	collect := func(p *model.Part, el model.Element) {
		switch e := el.(type) {
		case *model.Note:
			out = append(out, keyedNote{e, p.Key})
		case *model.Chord:
			for _, n := range e.Notes {
				out = append(out, keyedNote{n, p.Key})
			}
		}
	}

	if sel := q.Get("sel"); sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			return nil, err
		}
		for _, t := range targets {
			p, m, err := a.locate(t.part, t.measure)
			if err != nil {
				continue // a stale selection is not worth failing over
			}
			v := m.Voices[0]
			if t.index < 0 || t.index >= len(v.Elements) {
				continue
			}
			collect(p, v.Elements[t.index])
		}
		if len(out) == 0 {
			return nil, editError("there is no note in the selection")
		}
		return out, nil
	}

	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}
	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		return nil, err
	}
	v := m.Voices[0]
	if len(v.Elements) == 0 {
		return nil, editError("the measure is empty")
	}
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	collect(p, v.Elements[at-1])
	if len(out) == 0 {
		return nil, editError("that is not a note")
	}
	return out, nil
}

func (a *App) handleStem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	stem, ok := stemByName[q.Get("stem")]
	if !ok {
		http.Error(w, "stem must be up, down or auto", http.StatusBadRequest)
		return
	}
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, m := range marks {
		m.Stem = stem
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleDynamic writes, or removes, a dynamic at the cursor.
func (a *App) handleDynamic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("dynamic")

	var d model.Dynamic
	if name != "" && name != "none" {
		v, ok := model.DynamicFromName(name)
		if !ok {
			http.Error(w, "unknown dynamic "+name, http.StatusBadRequest)
			return
		}
		d = v
	}
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, m := range marks {
		m.Dynamic = d
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// target names one element: a staff, a measure and a position within it.
type target struct {
	part, measure, index int
}

// parseTargets reads the sel parameter, which is a list of
// staff:measure:index triples. It is how a command reaches everything inside
// a dragged rectangle rather than only the note under the cursor.
func parseTargets(sel string) ([]target, error) {
	var out []target
	for _, piece := range strings.Split(sel, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		f := strings.Split(piece, ":")
		if len(f) != 3 {
			return nil, editError("a selection looks like staff:measure:index")
		}
		var t target
		var err error
		if t.part, err = strconv.Atoi(f[0]); err != nil {
			return nil, editError("bad staff in the selection")
		}
		if t.measure, err = strconv.Atoi(f[1]); err != nil {
			return nil, editError("bad measure in the selection")
		}
		if t.index, err = strconv.Atoi(f[2]); err != nil {
			return nil, editError("bad position in the selection")
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, editError("nothing selected")
	}
	return out, nil
}

// marksOf returns the marks of every element a command should act on: the
// whole selection when one was dragged, otherwise the element at the cursor.
func (a *App) marksOf(q url.Values) ([]*model.Marks, error) {
	if sel := q.Get("sel"); sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			return nil, err
		}
		a.mu.Lock()
		defer a.mu.Unlock()

		var out []*model.Marks
		for _, t := range targets {
			_, m, err := a.locate(t.part, t.measure)
			if err != nil {
				continue // a stale selection is not worth failing over
			}
			v := m.Voices[0]
			if t.index < 0 || t.index >= len(v.Elements) {
				continue
			}
			if mk := marksOfElement(v.Elements[t.index]); mk != nil {
				out = append(out, mk)
			}
		}
		if len(out) == 0 {
			return nil, editError("nothing in the selection can be marked")
		}
		return out, nil
	}

	one, err := a.markAt(q)
	if err != nil {
		return nil, err
	}
	return []*model.Marks{one}, nil
}

func marksOfElement(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	}
	return nil
}

// markAt finds the marks of the element the cursor sits after, which is the
// one just placed or just clicked.
func (a *App) markAt(q url.Values) (*model.Marks, error) {
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	_, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		return nil, err
	}
	v := m.Voices[0]
	if len(v.Elements) == 0 {
		return nil, editError("the measure is empty")
	}
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	switch el := v.Elements[at-1].(type) {
	case *model.Note:
		return &el.Marks, nil
	case *model.Chord:
		return &el.Marks, nil
	case *model.Rest:
		return &el.Marks, nil
	}
	return nil, editError("nothing there to mark")
}

var articulationByName = map[string]model.Articulation{
	"staccato": model.Staccato, "staccatissimo": model.Staccatissimo,
	"accent": model.Accent, "tenuto": model.Tenuto,
	"marcato": model.Marcato, "marcato down": model.MarcatoDown,
	"fermata": model.Fermata, "up-bow": model.UpBow, "down-bow": model.DownBow,
}

// handleArticulation adds a mark to the element at the cursor, or clears them
// all when none is named.
func (a *App) handleArticulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("articulation")

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if name == "" {
		for _, m := range marks {
			m.Articulations = nil
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	}
	art, ok := articulationByName[name]
	if !ok {
		http.Error(w, "unknown articulation "+name, http.StatusBadRequest)
		return
	}
	for _, m := range marks {
		m.AddArticulation(art)
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleStaff changes a property of a staff itself: its clef, its meter, its
// instrument. These belong to the staff rather than to any note on it, which
// is why the palette applies them without needing a note selected.
func (a *App) handleStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))

	a.mu.Lock()
	defer a.mu.Unlock()
	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[partIdx]

	changed := []string{}
	if v := q.Get("clef"); v != "" {
		c, ok := clefByStringName[v]
		if !ok {
			http.Error(w, "unknown clef "+v, http.StatusBadRequest)
			return
		}
		p.Clef = c
		changed = append(changed, "clef")
	}
	if v := q.Get("time"); v != "" {
		t, err := parseMeter(v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// The meter belongs to the music, not to one staff of it: everybody
		// is counting the same bars. A score with 4/4 on one staff and 2/4 on
		// the next does not mean anything, so this goes on every part.
		at, hasAt := strconv.Atoi(q.Get("at"))
		for _, part := range a.score.Parts {
			if hasAt == nil && at > 0 {
				measureAt(part, at).Time = &t
			} else {
				part.Time = t
				if len(part.Measures) > 0 {
					part.Measures[0].Time = nil
				}
			}
		}
		if hasAt == nil && at > 0 {
			changed = append(changed, "time change")
		} else {
			changed = append(changed, "time")
		}
	}
	if v := q.Get("instrument"); v != "" {
		inst, ok := model.InstrumentFromName(v)
		if !ok {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 127 {
				http.Error(w, "unknown instrument "+v, http.StatusBadRequest)
				return
			}
			inst = model.Instrument(n)
		}
		p.Instrument = inst
		changed = append(changed, "instrument")

		// An instrument brings its transposition with it, and changing it has
		// to leave the music sounding as it did: what is printed moves, so
		// that what is heard does not. Choosing "alto sax" on a part in C
		// rewrites it in A, three sharps, a major sixth higher — which is
		// what the player needs to read to produce the same notes.
		if q.Get("transpose") != "0" {
			want := model.TranspositionFor(inst)
			if want != p.Transpose {
				retranspose(p, p.Transpose, want)
				p.Transpose = want
				if !want.IsZero() {
					changed = append(changed, "transposition")
				}
			}
		}
	}
	if len(changed) == 0 {
		http.Error(w, "nothing to change", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "changed": changed})
}

// parseMeter reads "4/4", or "4/4c" and "2/2c" for the two that have a symbol
// of their own.
func parseMeter(v string) (model.TimeSignature, error) {
	v = strings.TrimSpace(v)

	// The two meters with glyphs of their own are usually written as those
	// glyphs: somebody asked for common time by typing "C", which is what it
	// is called and what is printed, and got told it did not look like 4/4.
	switch strings.ToLower(v) {
	case "c", "common":
		return model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeCommon}, nil
	case "¢", "c|", "cut", "alla breve":
		return model.TimeSignature{Beats: 2, BeatValue: 2, Symbol: model.TimeCut}, nil
	}

	sym := model.TimeNumeric
	if strings.HasSuffix(strings.ToLower(v), "c") {
		v = v[:len(v)-1]
		sym = model.TimeCommon
		if v == "2/2" {
			sym = model.TimeCut
		}
	}
	n, d, ok := strings.Cut(v, "/")
	if !ok {
		return model.TimeSignature{}, editError(
			`a meter looks like 4/4, or C for common time and C| for cut`)
	}
	beats, err1 := strconv.Atoi(n)
	value, err2 := strconv.Atoi(d)
	if err1 != nil || err2 != nil || beats <= 0 || value <= 0 {
		return model.TimeSignature{}, editError(
			`a meter looks like 4/4, or C for common time and C| for cut`)
	}
	return model.TimeSignature{Beats: beats, BeatValue: value, Symbol: sym}, nil
}

// handleTempo sets the metronome mark.
func (a *App) handleTempo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	bpm, err := strconv.Atoi(q.Get("bpm"))
	if err != nil || bpm < 10 || bpm > 400 {
		http.Error(w, "a tempo is between 10 and 400 beats a minute",
			http.StatusBadRequest)
		return
	}

	// The beat the number counts. A tempo of 120 means nothing without it.
	unit := model.Duration{Num: 1, Den: 4}
	if v := q.Get("unit"); v != "" {
		d, ok := durationNames[v]
		if !ok {
			http.Error(w, "unknown beat unit "+v, http.StatusBadRequest)
			return
		}
		unit = d
	}
	if q.Get("dotted") == "1" {
		unit = unit.Scale(3, 2)
	}

	a.mu.Lock()
	a.score.Tempo = model.Tempo{BPM: bpm, Unit: unit, Text: q.Get("text")}
	a.mu.Unlock()
	writeJSON(w, map[string]any{"status": "ok", "bpm": bpm})
}

// handleHeader sets the title block.
func (a *App) handleHeader(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.mu.Lock()
	defer a.mu.Unlock()
	// Only what was sent is changed, so a dialog can set one field alone.
	if q.Has("title") {
		a.score.Identity.Title = q.Get("title")
	}
	if q.Has("subtitle") {
		a.score.Identity.Subtitle = q.Get("subtitle")
	}
	if q.Has("composer") {
		a.score.Identity.Composer = q.Get("composer")
	}
	if q.Has("copyright") {
		a.score.Identity.Copyright = q.Get("copyright")
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleInstruments lists what a staff can be set to sound as.
func (a *App) handleInstruments(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Program int    `json:"program"`
		Name    string `json:"name"`
		Family  string `json:"family"`
	}
	out := []entry{}
	for p := 0; p < 128; p++ {
		i := model.Instrument(p)
		n := i.String()
		// Only the ones with a name of their own; the rest are "program 57"
		// and nobody is choosing those from a list.
		if strings.HasPrefix(n, "program ") {
			continue
		}
		out = append(out, entry{Program: p, Name: n, Family: i.Family().String()})
	}
	writeJSON(w, out)
}

// retranspose rewrites a part from one transposition to another, keeping what
// it sounds like.
//
// The written pitches move by the difference between the two intervals, and so
// does every key signature on the staff — including the changes written in the
// middle of it, which are just as much part of what the player reads.
func retranspose(p *model.Part, from, to model.Transposition) {
	steps := from.Steps() - to.Steps()
	semis := from.Semitones() - to.Semitones()
	if steps == 0 && semis == 0 {
		return
	}

	shiftKey := func(k model.KeySignature) model.KeySignature {
		out := k
		out.Fifths = k.Fifths + to.Fifths() - from.Fifths()
		for out.Fifths > 7 {
			out.Fifths -= 12
		}
		for out.Fifths < -7 {
			out.Fifths += 12
		}
		return out
	}
	p.Key = shiftKey(p.Key)

	for _, m := range p.Measures {
		if m.Key != nil {
			k := shiftKey(*m.Key)
			m.Key = &k
		}
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				switch n := el.(type) {
				case *model.Note:
					movePitch(n, steps, semis)
				case *model.Chord:
					for _, cn := range n.Notes {
						movePitch(cn, steps, semis)
					}
				}
			}
		}
	}
}

// movePitch transposes one note, keeping an accidental it was written with.
func movePitch(n *model.Note, steps, semis int) {
	before := n.Accidental
	n.Pitch = n.Pitch.Transpose(steps, semis)
	if before == model.NoAccidental {
		return
	}
	// The note was spelled with an accidental, so the transposed one is too,
	// whatever it turns out to be.
	switch n.Pitch.Alter {
	case 2:
		n.Accidental = model.DoubleSharp
	case 1:
		n.Accidental = model.Sharp
	case 0:
		n.Accidental = model.Natural
	case -1:
		n.Accidental = model.Flat
	case -2:
		n.Accidental = model.DoubleFlat
	}
}

// handleHairpin writes a crescendo or diminuendo over the selection.
//
// A hairpin is not a property of a note, it is a stretch of music: it needs a
// beginning and an end, which is exactly what a range selection is. One note
// selected gets a hairpin that opens and closes on it, which is how it is
// taken off again.
func (a *App) handleHairpin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	kind := model.Crescendo
	switch q.Get("hairpin") {
	case "crescendo", "cresc", "<":
	case "diminuendo", "dim", "decrescendo", ">":
		kind = model.Diminuendo
	case "none", "":
		marks, err := a.marksOf(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, m := range marks {
			m.Hairpin = nil
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	default:
		http.Error(w, "a hairpin is a crescendo or a diminuendo", http.StatusBadRequest)
		return
	}

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for i, m := range marks {
		m.Hairpin = &model.Hairpin{
			Kind:  kind,
			Start: i == 0,
			Stop:  i == len(marks)-1,
		}
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleOctave writes an 8va or 8vb line over the selection.
func (a *App) handleOctave(w http.ResponseWriter, r *http.Request) {
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

	amount := 0
	switch q.Get("octave") {
	case "8va", "up", "1":
		amount = 1
	case "8vb", "down", "-1":
		amount = -1
	case "15ma", "2":
		amount = 2
	case "15mb", "-2":
		amount = -2
	case "none", "":
		for _, m := range marks {
			m.Octave = nil
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	default:
		http.Error(w, "an octave line is 8va, 8vb, 15ma or 15mb", http.StatusBadRequest)
		return
	}

	for i, m := range marks {
		m.Octave = &model.OctaveShift{
			Amount: amount,
			Start:  i == 0,
			Stop:   i == len(marks)-1,
		}
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleEnding puts a volta bracket over a measure, on every staff.
//
// A repeat belongs to the piece: all the players go back at the same bar and
// all of them take the second ending the second time. So this is set on every
// part at once, whichever staff the selection happened to be on.
func (a *App) handleEnding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	first, _ := strconv.Atoi(q.Get("measure"))
	last := first
	if v, err := strconv.Atoi(q.Get("to")); err == nil && v > first {
		last = v
	}

	var numbers []int
	for _, f := range strings.Split(q.Get("numbers"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			numbers = append(numbers, n)
		}
	}
	remove := q.Get("ending") == "none"
	if !remove && len(numbers) == 0 {
		numbers = []int{1}
	}
	open := q.Get("open") == "1"

	a.mu.Lock()
	defer a.mu.Unlock()

	changed := 0
	for _, p := range a.score.Parts {
		for mi := first; mi <= last; mi++ {
			if mi < 0 || mi >= len(p.Measures) {
				continue
			}
			if remove {
				p.Measures[mi].Ending = nil
			} else {
				p.Measures[mi].Ending = &model.Ending{
					Numbers: numbers,
					Start:   mi == first,
					Stop:    mi == last,
					Open:    open,
				}
			}
			changed++
		}
	}
	if changed == 0 {
		http.Error(w, "there is no such measure", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": changed})
}

/*
 * Moving, copying and pasting what is selected
 */

// handleNudge moves the selected notes up or down.
//
// This is the arrow keys: a note is picked and pushed to the right pitch
// rather than deleted and written again. Moving by staff steps rather than by
// semitones is what makes it follow the key — a step up from F in G major is
// G, and a step up from F sharp is G as well.
func (a *App) handleNudge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	steps, err := strconv.Atoi(q.Get("steps"))
	if err != nil || steps == 0 {
		http.Error(w, "steps says how far to move", http.StatusBadRequest)
		return
	}

	notes, err := a.notesOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, n := range notes {
		n.Pitch = model.PitchFromDiatonicIndex(n.Pitch.DiatonicIndex() + steps)
		if n.Accidental == model.NoAccidental {
			// No accidental written, so the key decides how it sounds.
			n.Pitch.Alter = n.key.AlterFor(n.Pitch.Step)
		} else {
			n.Pitch.Alter = n.Accidental.Semitones()
		}
	}
	writeJSON(w, map[string]int{"changed": len(notes)})
}

// clipboard is what was last copied: whole elements, lifted out of the score.
//
// It is kept on the server because that is where the music is. Copying in the
// browser would mean inventing a wire format for a Chord and keeping it in
// step with the model, for no gain — nothing outside gscore reads it.
type clipboard struct {
	elements []model.Element
}

// handleCopy takes a copy of the selection.
func (a *App) handleCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	targets, err := parseTargets(r.URL.Query().Get("sel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	var out []model.Element
	for _, t := range targets {
		_, m, err := a.locate(t.part, t.measure)
		if err != nil {
			continue
		}
		v := m.Voices[0]
		if t.index < 0 || t.index >= len(v.Elements) {
			continue
		}
		out = append(out, cloneElement(v.Elements[t.index]))
	}
	if len(out) == 0 {
		http.Error(w, "there is nothing there to copy", http.StatusBadRequest)
		return
	}
	a.clip = clipboard{elements: out}
	writeJSON(w, map[string]int{"copied": len(out)})
}

// handlePaste puts the copy back at the cursor.
func (a *App) handlePaste(w http.ResponseWriter, r *http.Request) {
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

	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.clip.elements) == 0 {
		http.Error(w, "nothing has been copied", http.StatusBadRequest)
		return
	}
	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}

	// A fresh copy each time, so pasting twice does not give two references
	// to the same notes — changing one would silently change the other.
	add := make([]model.Element, len(a.clip.elements))
	for i, el := range a.clip.elements {
		add[i] = cloneElement(el)
	}

	v.Elements = append(v.Elements, make([]model.Element, len(add))...)
	copy(v.Elements[at+len(add):], v.Elements[at:])
	copy(v.Elements[at:], add)

	// What no longer fits the bar moves into the next one, as when writing.
	spill(p, measIdx)
	measure, idx := findElement(p, measIdx, add[len(add)-1])
	writeJSON(w, map[string]any{
		"pasted": len(add), "measure": measure, "at": idx,
	})
}

// cloneElement copies an element deeply enough that editing the copy cannot
// reach the original: the marks, and a chord's noteheads.
func cloneElement(el model.Element) model.Element {
	switch e := el.(type) {
	case *model.Note:
		n := *e
		n.Marks = cloneMarks(e.Marks)
		return &n
	case *model.Rest:
		rst := *e
		rst.Marks = cloneMarks(e.Marks)
		return &rst
	case *model.Chord:
		c := *e
		c.Marks = cloneMarks(e.Marks)
		c.Notes = make([]*model.Note, len(e.Notes))
		for i, n := range e.Notes {
			cn := *n
			cn.Marks = cloneMarks(n.Marks)
			c.Notes[i] = &cn
		}
		return &c
	case *model.SymbolElement:
		sy := *e
		return &sy
	}
	return el
}

func cloneMarks(m model.Marks) model.Marks {
	out := m
	if m.Articulations != nil {
		out.Articulations = append([]model.Articulation(nil), m.Articulations...)
	}
	if m.Symbols != nil {
		out.Symbols = append([]model.SymbolMark(nil), m.Symbols...)
	}
	if m.Tuplet != nil {
		t := *m.Tuplet
		out.Tuplet = &t
	}
	if m.Hairpin != nil {
		h := *m.Hairpin
		out.Hairpin = &h
	}
	if m.Octave != nil {
		o := *m.Octave
		out.Octave = &o
	}
	return out
}

/*
 * Lyrics
 */

// handleLyrics puts a verse of words under a staff.
//
// The words arrive as one block of text rather than syllable by syllable,
// which is how NoteWorthy Composer asks for them and is far and away the
// fastest way to enter a song: you type the verse as you would write it, and
// it is handed out to the notes in order. Hyphens break a word across notes
// and an underscore holds the last syllable over the next one — the
// conventions every notation program shares, because they are the ones
// singers' copies already use.
//
//	Glo-ri-a in ex-cel-sis De-o
//	A-men_ _
//
// Rests take no words. A syllable landing on a tied note goes on the note the
// syllable is actually sung on, which is the first of the tie.
func (a *App) handleLyrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	verse, err := strconv.Atoi(q.Get("verse"))
	if err != nil || verse < 1 {
		verse = 1
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[partIdx]

	syllables := splitLyrics(q.Get("text"))
	sung := singableNotes(p)

	// Every note of this verse starts clear, so retyping the words replaces
	// them rather than leaving the tail of a longer version behind.
	for _, mk := range sung {
		out := mk.Lyrics[:0]
		for _, ly := range mk.Lyrics {
			if ly.Verse != verse {
				out = append(out, ly)
			}
		}
		mk.Lyrics = out
	}

	n := len(syllables)
	if n > len(sung) {
		n = len(sung)
	}
	for i := 0; i < n; i++ {
		s := syllables[i]
		s.Verse = verse
		sung[i].Lyrics = append(sung[i].Lyrics, s)
	}

	resp := map[string]any{"set": n, "notes": len(sung), "syllables": len(syllables)}
	if len(syllables) > len(sung) {
		resp["warning"] = fmt.Sprintf(
			"there are %d syllables and only %d notes to sing them on",
			len(syllables), len(sung))
	}
	writeJSON(w, resp)
}

// singableNotes lists the marks of everything in a part that a syllable can be
// written under, in reading order.
//
// Rests are left out, and so is the far side of a tie: a tied pair is one
// sound and gets one syllable.
func singableNotes(p *model.Part) []*model.Marks {
	var out []*model.Marks
	for _, m := range p.Measures {
		if len(m.Voices) == 0 {
			continue
		}
		for _, el := range m.Voices[0].Elements {
			mk := marksOfElement(el)
			if mk == nil {
				continue
			}
			switch el.(type) {
			case *model.Note, *model.Chord:
			default:
				continue
			}
			if mk.TieStop && !mk.TieStart {
				continue
			}
			out = append(out, mk)
		}
	}
	return out
}

// splitLyrics turns a typed verse into syllables.
//
// A hyphen inside a word splits it and marks the halves as joined, so the
// engraver can draw the hyphen between them. An underscore on its own holds
// the previous syllable over another note. Line breaks are nothing more than
// spaces: a verse laid out over several lines is still one verse.
func splitLyrics(text string) []model.Lyric {
	var out []model.Lyric
	for _, word := range strings.Fields(text) {
		if word == "_" {
			// Held over: the syllable before it keeps sounding.
			if len(out) > 0 {
				out[len(out)-1].Extend = true
			}
			out = append(out, model.Lyric{Syllabic: model.SyllableSingle, Extend: true})
			continue
		}
		parts := strings.Split(word, "-")
		if len(parts) == 1 {
			out = append(out, model.Lyric{Text: word, Syllabic: model.SyllableSingle})
			continue
		}
		for i, piece := range parts {
			if piece == "" {
				continue // "Glo--ri" is a typo, not an empty syllable
			}
			kind := model.SyllableMiddle
			switch i {
			case 0:
				kind = model.SyllableBegin
			case len(parts) - 1:
				kind = model.SyllableEnd
			}
			out = append(out, model.Lyric{Text: piece, Syllabic: kind})
		}
	}
	return out
}

// handleGetLyrics gives back a verse as typed, so it can be edited rather than
// written again from nothing.
func (a *App) handleGetLyrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	verse, err := strconv.Atoi(q.Get("verse"))
	if err != nil || verse < 1 {
		verse = 1
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[partIdx]

	var b strings.Builder
	verses := 0
	for _, mk := range singableNotes(p) {
		for _, ly := range mk.Lyrics {
			if ly.Verse > verses {
				verses = ly.Verse
			}
			if ly.Verse != verse {
				continue
			}
			if ly.Text == "" {
				b.WriteString(" _")
				continue
			}
			if b.Len() > 0 && !endsHyphenated(b.String()) {
				b.WriteByte(' ')
			}
			b.WriteString(ly.Text)
			if ly.NeedsHyphen() {
				b.WriteByte('-')
			}
		}
	}
	writeJSON(w, map[string]any{
		"text": b.String(), "verse": verse, "verses": verses,
		"notes": len(singableNotes(p)),
	})
}

func endsHyphenated(s string) bool { return strings.HasSuffix(s, "-") }

// handleText writes words beside a note: "rit.", "solo", a reminder.
func (a *App) handleText(w http.ResponseWriter, r *http.Request) {
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
	text := strings.TrimSpace(q.Get("text"))

	place := model.PlacementAbove
	if q.Get("placement") == "below" {
		place = model.PlacementBelow
	}
	for _, m := range marks {
		if text == "" {
			m.Texts = nil
			continue
		}
		m.Texts = append(m.Texts, model.TextMark{
			Text: text, Placement: place,
			Italic: q.Get("italic") == "1", Bold: q.Get("bold") == "1",
		})
		// Words go on one note, not on every note of a passage.
		break
	}
	writeJSON(w, map[string]any{"status": "ok", "text": text})
}

// handleMarker puts a rehearsal mark on a bar, across the whole score.
func (a *App) handleMarker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	text := strings.TrimSpace(q.Get("text"))

	a.mu.Lock()
	defer a.mu.Unlock()

	found := false
	for _, p := range a.score.Parts {
		if measIdx < 0 || measIdx >= len(p.Measures) {
			continue
		}
		// Every staff, because "from letter B" has to mean the same bar to
		// everybody. A mark on one staff is a mark nobody else can find.
		p.Measures[measIdx].Marker = text
		found = true
	}
	if !found {
		http.Error(w, "there is no such measure", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "measure": measIdx, "text": text})
}

// handleView changes how the score is drawn rather than what it says.
func (a *App) handleView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.mu.Lock()
	defer a.mu.Unlock()

	changed := []string{}
	if v := q.Get("names"); v != "" {
		a.engraveOpt.ShowPartNames = v == "1"
		changed = append(changed, "names")
	}
	if v := q.Get("numbers"); v != "" {
		a.engraveOpt.ShowMeasureNumbers = v == "1"
		changed = append(changed, "numbers")
	}
	writeJSON(w, map[string]any{
		"status":  "ok",
		"changed": changed,
		"names":   a.engraveOpt.ShowPartNames,
		"numbers": a.engraveOpt.ShowMeasureNumbers,
	})
}

// handleGroup joins staves with a brace or a bracket.
func (a *App) handleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	first, _ := strconv.Atoi(q.Get("first"))
	last, _ := strconv.Atoi(q.Get("last"))

	a.mu.Lock()
	defer a.mu.Unlock()

	if q.Get("symbol") == "none" {
		// Take away whatever group covers these staves.
		var kept []model.StaffGroup
		for _, g := range a.score.Groups {
			if g.First == first && g.Last == last {
				continue
			}
			kept = append(kept, g)
		}
		a.score.Groups = kept
		writeJSON(w, map[string]any{"status": "ok", "groups": len(a.score.Groups)})
		return
	}

	if first < 0 || last >= len(a.score.Parts) || last <= first {
		http.Error(w, "a group joins two staves or more", http.StatusBadRequest)
		return
	}
	symbol := model.GroupBracket
	switch q.Get("symbol") {
	case "brace":
		symbol = model.GroupBrace
	case "line":
		symbol = model.GroupLine
	}
	a.score.Groups = append(a.score.Groups, model.StaffGroup{
		First: first, Last: last, Symbol: symbol,
		Barlines: q.Get("barlines") == "1" || symbol == model.GroupBrace,
		Name:     q.Get("name"),
	})
	writeJSON(w, map[string]any{"status": "ok", "groups": len(a.score.Groups)})
}

// handleBeam beams the selection as one group, or breaks it apart.
//
// The engraver groups by beat, which is right nearly always. "Nearly" is the
// problem: music phrased against the beat needs the beams drawn against it
// too, and that is a decision only the person writing it can make.
func (a *App) handleBeam(w http.ResponseWriter, r *http.Request) {
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

	switch q.Get("beam") {
	case "join", "":
		if len(marks) < 2 {
			http.Error(w, "select two notes or more to beam them together",
				http.StatusBadRequest)
			return
		}
		for i, m := range marks {
			switch {
			case i == 0:
				m.Beam = model.BeamStart
			case i == len(marks)-1:
				m.Beam = model.BeamStop
			default:
				m.Beam = model.BeamContinue
			}
		}
	case "break", "none":
		for _, m := range marks {
			m.Beam = model.BeamNone
		}
	case "auto":
		for _, m := range marks {
			m.Beam = model.BeamAuto
		}
	default:
		http.Error(w, "beam is join, break or auto", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleChordSymbol writes a chord name over a note.
//
// The name is given the way a musician writes it — "Em6", "Bb7", "C/G",
// "N.C." — and taken apart here rather than in the browser, so that every
// reader of a .gscore gets the same symbol and the engraver can set the root,
// the quality and the bass note each at its own size.
func (a *App) handleChordSymbol(w http.ResponseWriter, r *http.Request) {
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

	text := strings.TrimSpace(q.Get("chord"))
	if text == "" || text == "none" {
		for _, m := range marks {
			m.Chord = nil
		}
		writeJSON(w, map[string]any{"status": "ok", "chord": ""})
		return
	}

	sym := parseChordSymbol(text)
	if f := q.Get("frame"); f != "" {
		sym.Frame = parseChordFrame(f, q.Get("firstfret"), q.Get("barre"))
	}
	// One note carries it, not every note of a selection: a chord symbol
	// marks where the harmony changes, and repeating it on each note of the
	// bar would say it changes on every one.
	marks[0].Chord = sym
	writeJSON(w, map[string]any{"status": "ok", "chord": sym.String()})
}

// parseChordSymbol takes a chord name apart.
//
// "N.C." and anything else that is not a chord is kept whole: a symbol that
// is not a chord has no root to set large and no quality to set small.
func parseChordSymbol(s string) *model.ChordSymbol {
	root, rest := "", s
	if len(s) > 0 && s[0] >= 'A' && s[0] <= 'G' {
		root, rest = s[:1], s[1:]
		// The accidental belongs to the root, however it was typed.
		for len(rest) > 0 && (rest[0] == '#' || rest[0] == 'b') {
			root += rest[:1]
			rest = rest[1:]
		}
		for _, sign := range []string{"♯", "♭"} {
			for strings.HasPrefix(rest, sign) {
				root += rest[:len(sign)]
				rest = rest[len(sign):]
			}
		}
	}
	if root == "" {
		return &model.ChordSymbol{Text: s}
	}

	quality, bass := rest, ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		quality, bass = rest[:i], rest[i+1:]
	}
	return &model.ChordSymbol{Root: root, Quality: quality, Bass: bass}
}

// parseChordFrame reads a fingering as it is usually written down: one number
// per string, low to high, with x for a string that is not played.
//
//	"x02220"   an open shape
//	"133211"   a barre
func parseChordFrame(spec, firstFret, barre string) *model.ChordFrame {
	f := &model.ChordFrame{FirstFret: 1}
	if v, err := strconv.Atoi(firstFret); err == nil && v > 1 {
		f.FirstFret = v
	}
	if v, err := strconv.Atoi(barre); err == nil && v > 0 {
		f.Barre = v
	}

	fields := strings.Fields(spec)
	if len(fields) <= 1 {
		// Written as one run of characters, which only works up to fret 9 —
		// past that the spaced form is the one to use.
		for _, c := range spec {
			switch {
			case c == 'x' || c == 'X' || c == '-':
				f.Frets = append(f.Frets, -1)
			case c >= '0' && c <= '9':
				f.Frets = append(f.Frets, int(c-'0'))
			}
		}
		return f
	}
	for _, field := range fields {
		if field == "x" || field == "X" || field == "-" {
			f.Frets = append(f.Frets, -1)
			continue
		}
		if v, err := strconv.Atoi(field); err == nil {
			f.Frets = append(f.Frets, v)
		}
	}
	return f
}
