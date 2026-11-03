/*
 * retune.go
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

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Moving a tablature part.
 *
 * There are two things a guitarist means by putting a part up a tone, and
 * they are not the same edit:
 *
 *   the music moves    The notes change. The hand goes somewhere else to
 *                      play them, so the fret numbers are worked out
 *                      again on the instrument as it is tuned. This is
 *                      ordinary transposition and /api/transpose does it
 *                      — for a tablature too, now that refretTabs runs
 *                      after it.
 *
 *   the instrument     The notes change and the hand does not. Every
 *   moves              string is tuned somewhere else, the shapes stay
 *                      exactly as they are, and what sounds follows the
 *                      tuning. Drop D is this; so is a capo, which is
 *                      this upwards.
 *
 * The second has nowhere else to live. Writing a staff as a tablature
 * retunes it the other way about — it keeps the notes and moves the frets,
 * because a part that was written on a staff of pitches has notes that
 * mean something and fingerings that do not exist yet. That is right for
 * what it does and is the opposite of this.
 *
 * Which one somebody wants is not guessable from the request, so it is
 * asked rather than inferred.
 */

// handleRetune moves a tablature staff, keeping either the music or the
// fingering.
func (a *App) handleRetune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	semis, err := strconv.Atoi(q.Get("semitones"))
	if err != nil || semis == 0 {
		http.Error(w, "how many semitones, and which way?", http.StatusBadRequest)
		return
	}
	// Two octaves either way is past the end of any neck and past any
	// tuning anybody uses, so a number beyond it is a mistake.
	if semis < -24 || semis > 24 {
		http.Error(w, "that is further than two octaves", http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()

	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	// Either half of a pair will do. A guitar written as a staff of
	// pitches with a tablature under it is one instrument on two staves,
	// and retuning it is a thing you ask of the instrument — so asking it
	// of the staff the cursor happens to be on, which is usually the one
	// with the notes on it, has to work.
	tabIdx := a.tabHalf(partIdx)
	if tabIdx < 0 {
		http.Error(w, "that staff is not written as tablature",
			http.StatusBadRequest)
		return
	}
	tab := a.score.Parts[tabIdx].Tab
	// The music may be on the staff above: a tablature paired with a staff
	// of pitches holds none of its own.
	music := a.score.MusicOf(tabIdx)
	if music == nil {
		http.Error(w, "that staff has no music on it", http.StatusBadRequest)
		return
	}

	// Every string moves by the same interval, which is what retuning is.
	// Worked out in full before anything is changed, so a move that
	// cannot be made leaves the part as it was rather than half done.
	tuned, ok := model.Retuned(tab, semis)
	if !ok {
		http.Error(w, "that would tune a string past the end of what can be written",
			http.StatusBadRequest)
		return
	}

	notes := tabNotes(music)
	// The fingering stays and the pitch follows it. A note that was not on
	// the neck to begin with — one that arrived from a staff of pitches
	// and was out of reach — has no fingering to keep, so it is left as
	// the note it is.
	changed := 0
	for _, n := range notes {
		if n.String < 1 || n.String > len(tuned.Strings) {
			continue
		}
		sounding, ok := tuned.PitchAt(n.String, n.Fret)
		if !ok {
			continue
		}
		n.Pitch = music.Transpose.Unapply(sounding)
		changed++
	}

	// The staff keeps whatever the tab half is: on a pair both halves are
	// one instrument and the tuning belongs to the one that draws it.
	a.score.Parts[tabIdx].Tab = tuned

	writeJSON(w, map[string]any{
		"status": "ok", "changed": changed,
		"tuning":  model.TuningName(tuned),
		"strings": stringNames(tuned),
	})
}

// tabHalf is the staff that carries the tuning for the staff named, or -1.
//
// Itself, where it is a tablature. Otherwise the staff that mirrors it and
// is written as one: that is the shape of a notation-and-tablature pair,
// where the notes live on the upper staff and the tuning on the lower.
func (a *App) tabHalf(i int) int {
	if i < 0 || i >= len(a.score.Parts) {
		return -1
	}
	if a.score.Parts[i].Tab != nil {
		return i
	}
	for j, p := range a.score.Parts {
		if p.Tab != nil && p.MirrorOf != nil && *p.MirrorOf == i {
			return j
		}
	}
	return -1
}

// tabNotes is every note of a part, chords opened out.
func tabNotes(p *model.Part) []*model.Note {
	var out []*model.Note
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			if v == nil {
				continue
			}
			for _, el := range v.Elements {
				switch e := el.(type) {
				case *model.Note:
					out = append(out, e)
				case *model.Chord:
					out = append(out, e.Notes...)
				}
			}
		}
	}
	return out
}

// stringNames is the open strings as they are spoken, for the answer.
func stringNames(t *model.Tablature) []string {
	out := make([]string, 0, len(t.Strings))
	for _, s := range t.Strings {
		out = append(out, s.String())
	}
	return out
}
