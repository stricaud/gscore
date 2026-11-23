/*
 * review.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Marking bars while a score is being checked.
 *
 * A score typed in or read from a scan is checked against its source bar by
 * bar, and that takes longer than one sitting. Without a note of where it
 * has got to, every bar is checked again the next day, because nothing on
 * the page says which ones were already found right. So a bar can be
 * marked good, to check again, or to fix, and the editor tints it in the
 * colour of its mark. See model.Measure.Review.
 */

package app

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// handleReview marks bars.
//
//	POST /api/review?bars=0:3,0:4,1:3&review=good   mark these bars good
//	POST /api/review?bars=0:3&review=none           take the mark off
//	POST /api/review?all=1&review=none              take every mark off
//
// bars is staff:bar, counting both from zero, as many as were picked up.
// all is for when the checking is done: the marks are work in progress,
// and a finished score has no use for them.
func (a *App) handleReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	mark := q.Get("review")
	if mark == "none" {
		mark = ""
	}
	if !model.ValidReview(mark) {
		http.Error(w, "a bar is marked good, check or fix, or none",
			http.StatusBadRequest)
		return
	}
	if q.Get("all") == "1" {
		if mark != "" {
			http.Error(w, "every bar can be cleared at once, not marked",
				http.StatusBadRequest)
			return
		}
		a.lock()
		defer a.mu.Unlock()
		n := 0
		for _, p := range a.score.Parts {
			for _, m := range p.Measures {
				if m != nil && m.Review != "" {
					m.Review = ""
					n++
				}
			}
		}
		writeJSON(w, map[string]int{"changed": n})
		return
	}
	type bar struct{ part, measure int }
	var bars []bar
	for _, it := range strings.Split(q.Get("bars"), ",") {
		f := strings.Split(it, ":")
		if len(f) != 2 {
			continue
		}
		p, err1 := strconv.Atoi(f[0])
		m, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			continue
		}
		bars = append(bars, bar{p, m})
	}
	if len(bars) == 0 {
		http.Error(w, "no bar to mark", http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()
	n := 0
	for _, b := range bars {
		_, m, err := a.locate(b.part, b.measure)
		if err != nil {
			continue // a stale selection is not worth failing over
		}
		if m.Review != mark {
			m.Review = mark
			n++
		}
	}
	writeJSON(w, map[string]int{"changed": n})
}

// reviewMark is one marked bar, as the editor draws it.
type reviewMark struct {
	Part    int    `json:"part"`
	Measure int    `json:"measure"`
	Review  string `json:"review"`
}

// reviewMarks is every marked bar of a score, by the staff it is drawn on.
// A staff written a second way — the tablature under a guitar's notes —
// shows the mark of the staff whose music it is.
func reviewMarks(s *model.Score) []reviewMark {
	out := []reviewMark{}
	for pi := range s.Parts {
		p := s.MusicOf(pi)
		for mi, m := range p.Measures {
			if m != nil && m.Review != "" {
				out = append(out, reviewMark{Part: pi, Measure: mi, Review: m.Review})
			}
		}
	}
	return out
}
