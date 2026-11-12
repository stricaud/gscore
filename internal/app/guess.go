/*
 * guess.go
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

	"github.com/stricaud/gscore/internal/model"
)

// What each staff probably is, read off what it is called.
//
// A score arrives without its instruments more often than not: a PDF says
// how many staves there are and nothing about who is reading them, and a
// file written by a program that did not ask gives every staff the piano it
// opened with. Boléro comes in as forty-four pianos.
//
// The names are still there, though — "Hautbois", "Violoncelle", "Grosse
// caisse" — and they are what a human reader goes by. So this offers what
// they come to. It changes nothing: the answer goes to the dialog, which
// shows it as the choice it would have made, and somebody looks over
// forty-four guesses and corrects the two that are wrong instead of setting
// forty-four by hand.

// handleGuessInstruments says what each staff looks like, by its name.
func (a *App) handleGuessInstruments(w http.ResponseWriter, r *http.Request) {
	a.lock()
	defer a.mu.Unlock()

	names := make([]string, 0, len(a.score.Parts))
	for _, p := range a.score.Parts {
		names = append(names, p.Name)
	}
	sounds := model.GuessSounds(names)

	type guess struct {
		Part int    `json:"part"`
		Name string `json:"name"`
		// Sound is what the name comes to, as a catalogue id. Empty where
		// the name says nothing gscore knows — which is left alone rather
		// than filled with the nearest thing, because a wrong instrument
		// quietly chosen is one nobody goes looking for.
		Sound string `json:"sound,omitempty"`
		// Called is that instrument's name, for showing the guess back.
		Called string `json:"called,omitempty"`
		// Said is true where the score itself already says what this staff
		// is, so the dialog can leave those alone.
		Said bool `json:"said,omitempty"`
	}
	out := make([]guess, 0, len(names))
	for i, p := range a.score.Parts {
		g := guess{Part: i, Name: p.Name, Sound: sounds[i], Said: p.Sound != ""}
		if n, ok := model.SoundName(g.Sound); ok {
			g.Called = n
		}
		out = append(out, g)
	}
	writeJSON(w, out)
}
