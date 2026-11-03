/*
 * pagesetup.go
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
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
	gpdf "github.com/stricaud/gscore/internal/render/pdf"
)

// The paper a score is printed on.
//
// Everything downstream of this reads it: the PDF, the browser's print
// machinery, and the engraver, which has to be told how wide a system may
// be before it lays anything out. A score set up for A3 landscape and
// engraved to a line length that was right for A4 comes out a third of the
// way across the sheet, so this is not a question that can be left to the
// export.
//
// The whole setup goes over in one request rather than a field at a time.
// Page size, orientation and margins are decided together — widening the
// margins of a page that is about to become landscape is one decision —
// and sending them separately would re-engrave the score between each.

// handlePageSetup reads or changes the paper.
func (a *App) handlePageSetup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.mu.RLock()
		page := a.score.Page
		a.mu.RUnlock()
		writeJSON(w, pageAnswer(page))
		return
	case http.MethodPost:
	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()
	page := a.score.Page

	// Back to whatever the program would do, which is not the same as
	// setting every field to its default: a score that has chosen nothing
	// follows the program as it changes, and one that has written the
	// day's defaults into itself does not.
	if q.Get("reset") == "1" {
		a.score.Page = model.Page{}
		writeJSON(w, pageAnswer(a.score.Page))
		return
	}

	if v := q.Get("paper"); v != "" {
		name := strings.ToLower(strings.TrimSpace(v))
		if name != "custom" {
			if _, ok := model.PaperNamed(name); !ok {
				http.Error(w, "no paper called "+v+
					" — /api/pagesetup lists them", http.StatusBadRequest)
				return
			}
		}
		page.Paper = name
	}
	if v := q.Get("landscape"); v != "" {
		page.Landscape = v == "1"
	}

	// Millimetres throughout, which is what paper is specified in. The
	// limits are a sheet nobody can print and a margin wider than the
	// paper, both of which are a slip of the finger rather than a request.
	mm := func(name string, into *float64, lo, hi float64) bool {
		v := q.Get(name)
		if v == "" {
			return true
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < lo || f > hi {
			http.Error(w, name+" is a measurement in millimetres, "+
				"from "+trimFloat(lo)+" to "+trimFloat(hi),
				http.StatusBadRequest)
			return false
		}
		*into = f
		return true
	}
	ok := mm("width", &page.CustomWidth, 50, 2000) &&
		mm("height", &page.CustomHeight, 50, 2000) &&
		mm("marginLeft", &page.MarginLeft, 0, 150) &&
		mm("marginRight", &page.MarginRight, 0, 150) &&
		mm("marginTop", &page.MarginTop, 0, 150) &&
		mm("marginBottom", &page.MarginBottom, 0, 150)
	if !ok {
		return
	}

	// The type size, in points, because that is what an engraving
	// measurement is quoted in everywhere else here. Zero is "choose one
	// and shrink it to fit", which is what the renderer does untold.
	if v := q.Get("space"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || (f != 0 && (f < 3 || f > 20)) {
			http.Error(w, "the staff size is in points, from 3 to 20, "+
				"or 0 to fit the music to the page", http.StatusBadRequest)
			return
		}
		page.StaffSpace = f
	}

	// A custom size has to say what size it is, or the page comes out A4
	// and nothing says why.
	if page.Paper == "custom" && (page.CustomWidth <= 0 || page.CustomHeight <= 0) {
		http.Error(w, "a custom page needs a width and a height",
			http.StatusBadRequest)
		return
	}
	// And the margins have to leave something to print on.
	wmm, hmm := page.Size()
	left, right, top, bottom := page.Margins()
	if left+right >= wmm-20 || top+bottom >= hmm-20 {
		http.Error(w, "those margins leave no room for any music",
			http.StatusBadRequest)
		return
	}

	a.score.Page = page
	writeJSON(w, pageAnswer(page))
}

// pageAnswer is the setup as it stands, with the sizes on offer.
//
// The list goes with the answer rather than living at a route of its own:
// the dialog needs both and asking twice is two round trips for one
// window. It is a short list and it does not change.
func pageAnswer(page model.Page) map[string]any {
	papers := make([]map[string]any, 0, len(model.Papers)+1)
	for _, p := range model.Papers {
		papers = append(papers, map[string]any{
			"name": p.Name, "label": p.Label,
			"width": p.Width, "height": p.Height,
		})
	}
	papers = append(papers, map[string]any{
		"name": "custom", "label": "Custom…",
	})

	w, h := page.Size()
	left, right, top, bottom := page.Margins()
	paper := page.Paper
	if paper == "" {
		paper = model.DefaultPaper
	}
	return map[string]any{
		"papers":    papers,
		"paper":     paper,
		"landscape": page.Landscape,
		// What it actually comes to, the way round it is printed, so the
		// dialog can show the sheet without working it out again.
		"width": w, "height": h,
		"marginLeft": left, "marginRight": right,
		"marginTop": top, "marginBottom": bottom,
		"space": page.StaffSpace,
		// Whether anything has been chosen at all, which is what decides
		// if "back to the default" is worth offering.
		"chosen": !page.IsZero(),
	}
}

// defaultPaperOptions is the renderer's own defaults, for the test that
// checks a score which chose nothing is printed exactly as it always was.
func defaultPaperOptions() gpdf.Options { return gpdf.DefaultOptions() }
