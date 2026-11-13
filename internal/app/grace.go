/*
 * grace.go
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
	"net/url"
	"sort"
	"strconv"

	"github.com/stricaud/gscore/internal/engrave"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Grace notes, made in the editor.
 *
 * Everything else about them was already here: the model holds them on the
 * note they lean on, the engraver sets them at cue size with their own beams
 * and slur, playback steals the time for them, and the readers bring them in
 * from ABC and from Guitar Pro. The one thing missing was any way to make
 * one. They could be imported and they could be played; they could not be
 * written.
 *
 * The verb is "turn these notes into grace notes", not "add a grace note of
 * such a pitch". That is not a smaller way of doing it, it is the way that
 * fits what is already here. Pitches are entered by the note-entry that
 * exists — click or type a note, and every tool in the editor for choosing
 * accidentals, octaves and durations applies to it — and then the notes are
 * told to lean on the one after them. The alternative, a command carrying a
 * pitch of its own, would be a second way of saying a pitch, with a second
 * set of bugs about clefs and transposing instruments.
 *
 * A grace note takes no time out of the bar — that is what it is — so the
 * bar is one note shorter afterwards. That is not damage: a bar written with
 * an ornament in it was always a bar whose ornament is not counted, and the
 * editor shows the room left over as room to write in.
 */

// handleGrace makes the chosen notes into the grace notes of the note that
// follows them, or takes a group off again.
//
//	POST /api/grace?sel=0:3:1,0:3:2            make these two into grace notes
//	POST /api/grace?sel=0:3:3&grace=none       take the group off that note
//	POST /api/grace?sel=0:3:3&slashed=0        an appoggiatura, not a crush
func (a *App) handleGrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	switch {
	case q.Get("add") == "1":
		n, err := a.addGrace(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"changed": 1, "ord": n})
	case q.Get("grace") == "none":
		n, err := a.clearGrace(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"changed": n})
	case q.Has("slashed") && !q.Has("make"):
		n, err := a.slashGrace(q, q.Get("slashed") != "0")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"changed": n})
	default:
		n, err := a.makeGrace(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"changed": n})
	}
}

// graceTargets reads the selection and checks it is one run of notes in one
// bar of one staff.
//
// One bar, because a grace note leans on the note after it and "the note
// after it" stops meaning anything once the selection is scattered over the
// score. Saying so is better than taking the first few and leaving the rest
// where they are, which is the shape of edit nobody notices going wrong.
func (a *App) graceTargets(q url.Values) ([]target, error) {
	var targets []target
	if sel := q.Get("sel"); sel != "" {
		var err error
		if targets, err = parseTargets(sel); err != nil {
			return nil, err
		}
	} else {
		// One note, named the way markAt names one: "at" counts from one,
		// because it is a place in the bar as a person counts them.
		part, _ := strconv.Atoi(q.Get("part"))
		meas, _ := strconv.Atoi(q.Get("measure"))
		at, err := strconv.Atoi(q.Get("at"))
		if err != nil || at <= 0 {
			return nil, editError("say which notes, in sel or in at")
		}
		targets = []target{{part: part, measure: meas, index: at - 1}}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].index < targets[j].index })
	for _, t := range targets[1:] {
		if t.part != targets[0].part || t.measure != targets[0].measure {
			return nil, editError("grace notes and the note they lean on " +
				"belong to one bar of one staff")
		}
	}
	return targets, nil
}

// makeGrace takes the chosen notes out of the voice and hangs them on the
// note written after them.
func (a *App) makeGrace(q url.Values) (int, error) {
	targets, err := a.graceTargets(q)
	if err != nil {
		return 0, err
	}
	a.lock()
	defer a.mu.Unlock()

	if name, ok := a.voicedHere(targets[0].part); ok {
		return 0, errVoicedElsewhere(name)
	}
	_, m, err := a.locate(targets[0].part, targets[0].measure)
	if err != nil {
		return 0, err
	}
	v := m.Voices[0]

	// The note they lean on: the one after the last of them. A grace note
	// is written before what it ornaments, so there has to be something
	// after it, and it cannot be a silence.
	last := targets[len(targets)-1].index
	if last+1 >= len(v.Elements) {
		return 0, editError("a grace note leans on the note after it, " +
			"and there is none after this one in the bar")
	}
	carrier := marksOfElement(v.Elements[last+1])
	if carrier == nil {
		return 0, editError("there is nothing after this to lean on")
	}
	if _, isRest := v.Elements[last+1].(*model.Rest); isRest {
		return 0, editError("a grace note cannot lean on a rest")
	}

	var taken []*model.Note
	for _, t := range targets {
		if t.index < 0 || t.index >= len(v.Elements) {
			return 0, editError("there is no note there")
		}
		n, ok := v.Elements[t.index].(*model.Note)
		if !ok {
			// A chord would be several small notes on one stem, which the
			// group has no way to hold: its notes are a sequence, not a
			// stack. Better to say so than to keep one note of the chord
			// and drop the rest without a word.
			return 0, editError("only single notes can be made into grace " +
				"notes, not chords")
		}
		taken = append(taken, n)
	}

	// Out of the voice, back to front, so the indexes of the ones still to
	// go are the indexes they were read at.
	for i := len(targets) - 1; i >= 0; i-- {
		at := targets[i].index
		v.Elements = append(v.Elements[:at], v.Elements[at+1:]...)
	}

	if carrier.Grace == nil {
		// Slashed unless told otherwise: the single small note crushed in
		// before the beat is what "grace note" means to most people asking
		// for one, and the unslashed appoggiatura is the deliberate choice.
		carrier.Grace = &model.GraceGroup{Slashed: q.Get("slashed") != "0"}
	}
	carrier.Grace.Notes = append(carrier.Grace.Notes, taken...)
	return len(taken), nil
}

// clearGrace takes the small notes off the note they lean on.
//
// They go altogether rather than back into the bar as ordinary notes. Put
// back, they would need a duration the bar has no room for, and the bar
// would have to grow or something else would have to give; undo is the way
// back from an edit, and it is already here.
func (a *App) clearGrace(q url.Values) (int, error) {
	marks, err := a.marksOf(q)
	if err != nil {
		return 0, err
	}
	a.lock()
	defer a.mu.Unlock()
	n := 0
	for _, mk := range marks {
		if mk.Grace == nil {
			continue
		}
		mk.Grace = nil
		n++
	}
	if n == 0 {
		return 0, editError("there are no grace notes on that note")
	}
	return n, nil
}

// slashGrace draws the stroke through the stems, or takes it away: the
// difference between a crush played before the beat and an appoggiatura that
// takes its time out of the note it leans on. A player does two different
// things with the two, so it is not a decision about how it looks.
func (a *App) slashGrace(q url.Values, slashed bool) (int, error) {
	marks, err := a.marksOf(q)
	if err != nil {
		return 0, err
	}
	a.lock()
	defer a.mu.Unlock()
	n := 0
	for _, mk := range marks {
		if mk.Grace == nil {
			continue
		}
		mk.Grace.Slashed = slashed
		n++
	}
	if n == 0 {
		return 0, editError("there are no grace notes on that note")
	}
	return n, nil
}

// addGrace hangs one more small note on the note that is picked up.
//
// The other way round from makeGrace, and both are needed. Turning notes
// that are already written into ornaments is the natural gesture when there
// is room in the bar to write them first — but a bar that is already full
// has no room, and a bar that is already full is the usual case when
// somebody is ornamenting music they have just finished. Writing the notes
// there pushed them into the next bar, so the gesture could not even be
// started.
//
// This adds nothing to the voice and so cannot overflow anything. The pitch
// is a staff step, resolved by the part's own clef through the same
// arithmetic a written note goes through: it is not a second way of saying
// a pitch, it is the way the editor already says one. Left unsaid, the new
// note sits a step above the note it leans on, which is the commonest
// ornament there is and is a pitch somebody can then move with the arrows —
// a grace note can be picked up now, which is what makes that an answer.
func (a *App) addGrace(q url.Values) (int, error) {
	targets, err := a.graceTargets(q)
	if err != nil {
		return 0, err
	}
	t := targets[len(targets)-1]
	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(t.part, t.measure)
	if err != nil {
		return 0, err
	}
	v := m.Voices[0]
	if t.index < 0 || t.index >= len(v.Elements) {
		return 0, editError("there is no note there")
	}
	el := v.Elements[t.index]
	mk := marksOfElement(el)
	if mk == nil {
		return 0, editError("nothing there to lean on")
	}
	if _, isRest := el.(*model.Rest); isRest {
		return 0, editError("a grace note cannot lean on a rest")
	}

	clef := p.ClefAt(t.measure)
	step, hasStep := 0, false
	if val := q.Get("step"); val != "" {
		if step, err = strconv.Atoi(val); err != nil {
			return 0, editError("step is a staff step")
		}
		hasStep = true
	}
	var pitch model.Pitch
	switch {
	case q.Get("pitch") != "":
		got, _, ok := model.ParsePitch(q.Get("pitch"))
		if !ok {
			return 0, editError(q.Get("pitch") + " is not a pitch")
		}
		pitch = a.asWritten(p, got)
	case hasStep:
		pitch = a.asWritten(p, pitchForStep(clef, step))
	default:
		// A step above the note it leans on.
		pitch = a.asWritten(p, pitchForStep(clef, topStepOf(clef, el)+1))
	}

	if mk.Grace == nil {
		mk.Grace = &model.GraceGroup{Slashed: q.Get("slashed") != "0"}
	}
	// An eighth, which is how a grace note is written and what the readers
	// bring in. Two or more of them are beamed by the engraver, so a group
	// built one note at a time comes out as a group and not as a row of
	// flagged notes.
	mk.Grace.Notes = append(mk.Grace.Notes, &model.Note{
		Pitch: pitch, Dur: model.Duration{Num: 1, Den: 8},
	})
	return len(mk.Grace.Notes) - 1, nil
}

// topStepOf is the staff step of the highest notehead of an element, which
// is what a grace note is placed above when nobody says where to put it.
func topStepOf(clef model.Clef, el model.Element) int {
	switch e := el.(type) {
	case *model.Note:
		return engrave.StaffStep(clef, e.Pitch)
	case *model.Chord:
		top := 0
		for i, n := range e.Notes {
			if s := engrave.StaffStep(clef, n.Pitch); i == 0 || s > top {
				top = s
			}
		}
		return top
	}
	return 0
}

// handlePairNotation puts a staff of pitches above every staff written as
// tablature alone.
//
// The same thing -notation does when a file is opened, offered from inside
// the editor as well: a Guitar Pro file is tablature and nothing else, and
// finding that out after opening it should not mean quitting and starting
// again with a flag. The work itself is the model's — see
// model.PairWithNotation, which is careful about the numbering that mirrors
// and staff groups are kept in.
func (a *App) handlePairNotation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	a.lock()
	defer a.mu.Unlock()

	n := a.score.PairWithNotation()
	if n == 0 {
		http.Error(w, "there is no tab staff here that is not already "+
			"paired with a staff of pitches", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"added": n})
}
