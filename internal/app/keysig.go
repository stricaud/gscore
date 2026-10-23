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
	"strings"

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
	// The key belongs to the piece, so it is set on every staff — but not as
	// the same number of sharps on each. A part for an instrument in E-flat
	// is written a major sixth up, and its signature with it, so each staff
	// gets the signature its own player reads for the key that was asked for.
	// That is the whole point of knowing a part's transposition.
	asked := model.KeySignature{Fifths: fifths}

	// A key given for a measure past the first changes key there and leaves
	// what came before alone, which is how a piece modulates. Given for the
	// first measure, it is simply what the piece is in.
	at, atErr := strconv.Atoi(q.Get("at"))

	// Taking a change away again. A key written in the middle of a piece is
	// as easy to change your mind about as to write, and without this the
	// only way back was to set it to whatever was in force before it and
	// hope the reader noticed they cancelled out.
	if q.Get("remove") == "1" {
		if atErr != nil || at <= 0 {
			http.Error(w, "there is no key change at the start to remove",
				http.StatusBadRequest)
			return
		}
		removed := 0
		for _, p := range a.score.Parts {
			if at < len(p.Measures) && p.Measures[at].Key != nil {
				p.Measures[at].Key = nil
				removed++
			}
		}
		if removed == 0 {
			http.Error(w, "there is no key change at that measure",
				http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"status": "ok", "removed": removed})
		return
	}

	if atErr != nil || at <= 0 {
		for _, p := range a.score.Parts {
			p.Key = model.KeySignature{Fifths: p.Transpose.Written(asked).Fifths, Mode: p.Key.Mode}
			// A change written at the very start would be drawn twice.
			if len(p.Measures) > 0 {
				p.Measures[0].Key = nil
			}
		}
		writeJSON(w, map[string]string{"status": "ok", "where": "staff"})
		return
	}
	if at >= len(a.score.Parts[part].Measures) {
		http.Error(w, "there is no such measure", http.StatusBadRequest)
		return
	}
	for _, p := range a.score.Parts {
		if at >= len(p.Measures) {
			continue
		}
		want := p.Transpose.Written(asked).Fifths
		// Changing to what is already in force is how a change is taken away.
		if p.KeyAt(at-1).Fifths == want {
			p.Measures[at].Key = nil
			continue
		}
		k := model.KeySignature{Fifths: want, Mode: p.KeyAt(at).Mode}
		p.Measures[at].Key = &k
	}
	writeJSON(w, map[string]any{"status": "ok", "where": "measure", "measure": at})
}

// handleTimeSigSVG draws one time signature on a fragment of staff.
//
// A picker that shows the meters is better than a box to type one in, for the
// same reason the key signature picker is: you recognise the shape of 6/8
// faster than you decide what to type, and the two with glyphs of their own —
// common time and cut time — have no obvious spelling at all. Somebody typing
// "C" for common time and being told it does not look like 4/4 is the box
// being wrong, not them.
func (a *App) handleTimeSigSVG(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	t, err := parseMeter(q.Get("meter"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	space := 9.0
	if v, err := strconv.ParseFloat(q.Get("space"), 64); err == nil && v >= 3 && v <= 30 {
		space = v
	}

	s := &model.Score{Parts: []*model.Part{{
		Clef: model.Treble, Time: t, Measures: nil,
	}}}

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	opt.ShowTitle = false
	// No clef: the picture is about the meter, and a treble clef beside every
	// one of them would be the loudest thing in the picker.
	opt.Spacings.Margin = 0.5
	opt.Spacings.MarginY = 1.2
	opt.Spacings.ClefBefore = 0
	opt.Spacings.ClefAfter = 0
	opt.LineWidth = 0
	lay := engrave.Engrave(s, opt)
	// The clef is drawn by the layout, so it is lifted back out: there is no
	// option for a staff without one and inventing one would be a flag for a
	// picture nothing else asks for.
	lay.Glyphs = dropClefs(lay.Glyphs)

	ropt := svg.DefaultOptions()
	ropt.StaffSpace = space
	ropt.EmbedFont = false

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "max-age=3600")
	if err := svg.Render(w, lay, ropt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// dropClefs removes the clef glyphs from a drawing, and shifts what is left
// back to where the clef would have been.
func dropClefs(in []engrave.Glyph) []engrave.Glyph {
	shift := 0.0
	out := in[:0]
	for _, g := range in {
		if strings.HasPrefix(g.Name, "gClef") || strings.HasPrefix(g.Name, "fClef") ||
			strings.HasPrefix(g.Name, "cClef") || strings.HasPrefix(g.Name, "unpitchedPercussionClef") {
			if shift == 0 {
				shift = g.X
			}
			continue
		}
		g.X -= shift
		out = append(out, g)
	}
	return out
}
