/*
 * staffline.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The line from one staff to the other, showing a line of music going on
 * there. See model.StaffLine.
 */

package app

import (
	"net/http"

	"github.com/stricaud/gscore/internal/model"
)

// handleStaffLine draws a line between two elements on different staves,
// or takes it away.
//
//	POST /api/staffline?sel=0:27:4,1:27:2/5        join the two
//	POST /api/staffline?sel=0:27:4,1:27:2/5&remove=1
//
// Exactly two, on two staves. Whichever comes first in the music is where
// the line starts, whichever order they were picked up in: the line goes
// the way the music does.
func (a *App) handleStaffLine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	targets, err := parseTargets(q.Get("sel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(targets) != 2 || targets[0].part == targets[1].part {
		http.Error(w, "pick up two things on two different staves, "+
			"the one the line leaves and the one it goes to", http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()
	type end struct {
		marks *model.Marks
		bar   int
		at    model.Duration
	}
	var ends []end
	for _, t := range targets {
		if t.isGrace() {
			http.Error(w, "a line to the other staff joins notes and rests, "+
				"not grace notes", http.StatusBadRequest)
			return
		}
		_, m, err := a.locate(t.part, t.measure)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		v := lineOf(m, t.voice)
		if t.index < 0 || t.index >= len(v.Elements) {
			http.Error(w, "nothing there to draw a line from", http.StatusBadRequest)
			return
		}
		mk := marksOfElement(v.Elements[t.index])
		if mk == nil {
			http.Error(w, "a line joins notes and rests", http.StatusBadRequest)
			return
		}
		at := model.Duration{Num: 0, Den: 1}
		for _, el := range v.Elements[:t.index] {
			at = at.Add(el.Duration())
		}
		ends = append(ends, end{mk, t.measure, at})
	}
	first, last := ends[0], ends[1]
	if last.bar < first.bar || (last.bar == first.bar && last.at.Cmp(first.at) < 0) {
		first, last = last, first
	}

	if q.Get("remove") == "1" {
		for _, e := range []end{first, last} {
			e.marks.StaffLine = nil
		}
		writeJSON(w, map[string]string{"status": "removed"})
		return
	}
	on := func(mk *model.Marks) *model.StaffLine {
		if mk.StaffLine == nil {
			mk.StaffLine = &model.StaffLine{}
		}
		return mk.StaffLine
	}
	on(first.marks).Start = true
	on(last.marks).Stop = true
	writeJSON(w, map[string]string{"status": "joined"})
}
