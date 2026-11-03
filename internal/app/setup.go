/*
 * setup.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Setting a score up: choosing who is playing, all at once.
//
// gscore has always opened with a staff ready, which is right — a tune
// goes in before anybody has decided what it is for, and being asked to
// fill in a form first is how a good idea gets lost. This is the other
// case: somebody who already knows it is for a wind quintet, and who
// should not have to add five staves one at a time, put them in the right
// order by hand and then bracket them.
//
// What it does that adding staves one by one does not: it puts them in
// score order, brackets each family, and gives a piano its two staves and
// its brace. All three are conventions about the page that nobody should
// have to remember — see model.InScoreOrder, which is where the order
// itself is written down.

// handleSetup builds a score's staves from the instruments chosen.
func (a *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	var want []string
	for _, raw := range strings.Split(q.Get("instruments"), ",") {
		if id := strings.TrimSpace(raw); id != "" {
			want = append(want, id)
		}
	}
	if len(want) == 0 {
		http.Error(w, "choose an instrument or two", http.StatusBadRequest)
		return
	}
	if len(want) > 64 {
		http.Error(w, "that is more staves than a page can hold",
			http.StatusBadRequest)
		return
	}
	for _, id := range want {
		if !model.SoundKnown(id) {
			http.Error(w, "unknown instrument "+id, http.StatusBadRequest)
			return
		}
	}
	// The groups somebody arranged themselves, as runs over the
	// instruments in the order they were sent: "0-1,3-5".
	//
	// A bracket is a vertical line down the left of a run of staves, so
	// it covers everything between its ends — there is no drawing a
	// bracket round the first and third staff and not the second. A
	// chooser that lets somebody pick staves that are apart has to bring
	// them together first, and then the run is what it sends here.
	brackets, err := bracketRuns(q.Get("brackets"), len(want))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Score order unless the caller says they have their own. A chooser
	// that lets somebody drag the staves about has already answered the
	// question and must not be second-guessed — and groups are given by
	// place, so arranging them is saying the order too.
	if q.Get("order") != "keep" && len(brackets) == 0 {
		want = model.InScoreOrder(want)
	}

	a.lock()
	defer a.mu.Unlock()

	// Only over a score with nothing in it, unless told plainly. Making
	// the staves is a few seconds of work and losing a morning's is not,
	// so the one that cannot be undone by hand is the one that asks.
	if !scoreIsEmpty(a.score) && q.Get("replace") != "1" {
		http.Error(w, "this score already has music in it — "+
			"save it first, or pass replace=1", http.StatusBadRequest)
		return
	}

	bars := a.score.MeasureCount()
	if bars < 1 {
		bars = 1
	}
	parts, groups := buildStaves(want, a.score, bars, brackets)
	a.score.Parts = parts
	a.score.Groups = groups
	a.score.Sheets = nil // the old parts named staves that have gone
	a.score.Normalise()

	writeJSON(w, map[string]any{
		"status": "ok", "staves": len(parts), "groups": len(groups),
	})
}

// bracketRuns reads "0-1,3-5" as the runs of instruments it names.
//
// Counted from zero, over the instruments in the order they were given,
// and inclusive at both ends. A run of one is allowed through and drawn as
// nothing, because a chooser that lets somebody group a single staff
// should not have to know that is not a group.
func bracketRuns(raw string, have int) ([][2]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out [][2]int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		first, last := part, part
		if i := strings.IndexByte(part, '-'); i >= 0 {
			first, last = part[:i], part[i+1:]
		}
		a, err1 := strconv.Atoi(strings.TrimSpace(first))
		b, err2 := strconv.Atoi(strings.TrimSpace(last))
		if err1 != nil || err2 != nil {
			return nil, errors.New(part + " is not a run of staves")
		}
		if a > b {
			a, b = b, a
		}
		if a < 0 || b >= have {
			return nil, errors.New(part + " names a staff that is not there")
		}
		out = append(out, [2]int{a, b})
	}
	return out, nil
}

// scoreIsEmpty reports a score nobody has written anything into yet.
//
// Notes are what counts. A score that has had its title typed and its
// meter set is still one somebody is setting up, and refusing to lay out
// its staves because of that would be refusing the very thing they are
// doing.
func scoreIsEmpty(s *model.Score) bool {
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					switch el.(type) {
					case *model.Note, *model.Chord:
						return false
					}
				}
			}
		}
	}
	return true
}

// buildStaves makes the staves and the brackets that gather them.
//
// brackets, where it is given, is the groups somebody arranged themselves,
// by their place in want. It replaces the families rather than adding to
// them: asked for a bracket round particular staves and then drawing the
// automatic ones as well would be two answers to one question.
func buildStaves(want []string, s *model.Score, bars int, brackets [][2]int) ([]*model.Part, []model.StaffGroup) {
	var parts []*model.Part
	var groups []model.StaffGroup

	// Which staves each instrument turned into, so a group named by the
	// instruments somebody picked can be turned into a run of staves. A
	// piano is one instrument and two staves, and a bracket round it has
	// to reach round both.
	staffOf := make([][2]int, 0, len(want))

	// Where each staff's family begins and ends, so the families can be
	// bracketed once every staff is made.
	// n counts instruments and not staves: a piano on its own is a
	// family of one however many staves it reads from.
	type span struct{ first, last, n int }
	families := map[string]*span{}
	var order []string

	// The key and meter the score is already in, so a staff added to a
	// piece in five flats is not in C.
	key, time := model.KeySignature{}, model.TimeSignature{Beats: 4, BeatValue: 4}
	if len(s.Parts) > 0 {
		first := s.Parts[0]
		key = first.Transpose.Sounding(first.Key)
		time = first.Time
	}

	for _, id := range want {
		name, _ := model.SoundName(id)
		trans, _ := model.SoundTransposition(id)
		prog, _ := model.SoundProgram(id)
		percussion := model.SoundIsPercussion(id)

		// A piano is one instrument played with two hands: two staves,
		// braced, with the barlines run through the gap between them.
		// That gap is what says one player reads both.
		staves := 1
		if model.GrandStaff(id) {
			staves = 2
		}
		at := len(parts)
		for i := 0; i < staves; i++ {
			clef := clefForSound(id)
			if staves == 2 && i == 1 {
				clef = model.Bass
			}
			if percussion {
				clef = model.Percussion
			}
			p := &model.Part{
				Instrument: prog,
				Sound:      id,
				Clef:       clef,
				Key:        trans.Written(key),
				Time:       time,
				Transpose:  trans,
			}
			if percussion {
				p.Key = model.KeySignature{}
				p.Transpose = model.Transposition{}
			}
			// Named once for a grand staff: "Piano" beside both staves
			// says there are two pianos.
			if i == 0 {
				p.Name = capitalised(name)
			}
			for b := 0; b < bars; b++ {
				p.Measures = append(p.Measures, model.NewMeasure())
			}
			parts = append(parts, p)
		}
		if staves == 2 {
			groups = append(groups, model.StaffGroup{
				First: at, Last: at + 1, Symbol: model.GroupBrace,
				Barlines: true, Name: capitalised(name),
			})
		}

		staffOf = append(staffOf, [2]int{at, len(parts) - 1})

		fam := model.ScoreFamily(id)
		if fam == "" {
			continue
		}
		if sp, ok := families[fam]; ok {
			sp.last, sp.n = len(parts)-1, sp.n+1
		} else {
			families[fam] = &span{first: at, last: len(parts) - 1, n: 1}
			order = append(order, fam)
		}
	}

	// The groups somebody arranged themselves, where there are any.
	if len(brackets) > 0 {
		for _, b := range brackets {
			if b[0] < 0 || b[1] >= len(staffOf) || b[1] < b[0] {
				continue
			}
			first, last := staffOf[b[0]][0], staffOf[b[1]][1]
			if last <= first {
				// One staff. Not a group, and a bracket round it would
				// say there is a section of one.
				continue
			}
			groups = append(groups, model.StaffGroup{
				First: first, Last: last, Symbol: model.GroupBracket,
			})
		}
		return parts, groups
	}

	// A bracket round each family that holds more than one instrument.
	//
	// Instruments and not staves: a piano on its own is a family of one
	// however many staves it reads from, and bracketing its two would
	// have drawn a bracket outside the brace that already joins them —
	// two symbols saying the same thing, one of them wrongly.
	//
	// Only where the family is in one piece. Somebody who asked to keep
	// their own order may have put the harp between the horns, and a
	// bracket reaching round the lot of them would be a lie about the
	// page rather than a convenience.
	for _, fam := range order {
		sp := families[fam]
		if sp.n < 2 || sp.last <= sp.first {
			continue
		}
		if !familyIsWhole(parts, fam, sp.first, sp.last) {
			continue
		}
		groups = append(groups, model.StaffGroup{
			First: sp.first, Last: sp.last, Symbol: model.GroupBracket,
		})
	}
	return parts, groups
}

// familyIsWhole reports that every staff between first and last belongs to
// the same family.
func familyIsWhole(parts []*model.Part, fam string, first, last int) bool {
	for i := first; i <= last && i < len(parts); i++ {
		if model.ScoreFamily(parts[i].Sound) != fam {
			return false
		}
	}
	return true
}
