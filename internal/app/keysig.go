/*
 * keysig.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package app

import (
	"net/http"
	"strconv"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/render/svg"
)

// Key signatures are chosen by looking at them.
//
// gscore 0.0.9 did this already: legacy/pixmaps holds treble_A_flat.xpm,
// treble_G_sharp.xpm and the rest — a hand-drawn staff per key, so the user
// picked the signature they could see rather than a name they had to decode.
// It was the right idea, taken from how Sibelius does it.
//
// The pictures no longer have to be drawn by hand. The engraver already knows
// where a key signature's accidentals belong on any clef, so each one is
// rendered on demand, at any size, for clefs the original never had pictures
// for.

// keyNames gives the major and minor key for each position on the circle of
// fifths, for the label beneath each picture.
var keyNames = map[int][2]string{
	-7: {"C♭", "A♭m"}, -6: {"G♭", "E♭m"}, -5: {"D♭", "B♭m"}, -4: {"A♭", "Fm"},
	-3: {"E♭", "Cm"}, -2: {"B♭", "Gm"}, -1: {"F", "Dm"}, 0: {"C", "Am"},
	1: {"G", "Em"}, 2: {"D", "Bm"}, 3: {"A", "F♯m"}, 4: {"E", "C♯m"},
	5: {"B", "G♯m"}, 6: {"F♯", "D♯m"}, 7: {"C♯", "A♯m"},
}

type keyOption struct {
	Fifths int    `json:"fifths"`
	Major  string `json:"major"`
	Minor  string `json:"minor"`
}

// handleKeySignatures lists the fifteen key signatures for the picker.
func (a *App) handleKeySignatures(w http.ResponseWriter, r *http.Request) {
	out := make([]keyOption, 0, 15)
	for f := -7; f <= 7; f++ {
		n := keyNames[f]
		out = append(out, keyOption{Fifths: f, Major: n[0], Minor: n[1]})
	}
	writeJSON(w, out)
}

var clefByName = map[string]model.Clef{
	"treble": model.Treble, "bass": model.Bass,
	"alto": model.Alto, "tenor": model.Tenor,
}

// handleKeySigSVG draws one key signature on a short staff: the modern
// equivalent of treble_G_sharp.xpm, generated rather than drawn.
func (a *App) handleKeySigSVG(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	fifths, _ := strconv.Atoi(q.Get("fifths"))
	if fifths < -7 {
		fifths = -7
	}
	if fifths > 7 {
		fifths = 7
	}
	clef := model.Treble
	if c, ok := clefByName[q.Get("clef")]; ok {
		clef = c
	}
	space := 7.0
	if v, err := strconv.ParseFloat(q.Get("space"), 64); err == nil && v >= 3 && v <= 30 {
		space = v
	}

	// A one-measure score with nothing in it draws the clef and the signature
	// and stops, which is exactly the picture wanted.
	s := &model.Score{Parts: []*model.Part{{
		Clef:     clef,
		Key:      model.KeySignature{Fifths: fifths, Mode: model.Major},
		Time:     model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric},
		Measures: nil,
	}}}

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	opt.ShowTitle = false
	opt.HideTimeSignature = true
	// A clef is not confined to the staff: a treble clef curls about two and a
	// half spaces above the top line and the same below the bottom one. A
	// margin tight to the staff lines cuts both ends off, which is what the
	// picture is mostly made of. Sideways there is nothing to clear, so the
	// border stays narrow there and the picture does not swim in white.
	opt.Spacings.Margin = 0.6
	opt.Spacings.MarginY = 2.9
	opt.LineWidth = 0
	lay := engrave.Engrave(s, opt)

	ropt := svg.DefaultOptions()
	ropt.StaffSpace = space
	ropt.EmbedFont = false

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "max-age=3600")
	if err := svg.Render(w, lay, ropt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleSetKey changes a part's key signature from the picker.
func (a *App) handleSetKey(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	part, _ := strconv.Atoi(q.Get("part"))
	fifths, _ := strconv.Atoi(q.Get("fifths"))
	if fifths < -7 || fifths > 7 {
		http.Error(w, "fifths out of range", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if part < 0 || part >= len(a.score.Parts) {
		http.Error(w, "no such part", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[part]
	p.Key = model.KeySignature{Fifths: fifths, Mode: p.Key.Mode}
	writeJSON(w, map[string]string{"status": "ok"})
}
