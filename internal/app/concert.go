/*
 * concert.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"sync"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Showing the score as it sounds.
 *
 * gscore stores what each player reads, which is what a transposing part
 * is: a clarinet in B flat reads a C and a B flat comes out. That is the
 * right thing to store — clicking a staff puts a note where it was clicked
 * — but it is not what a conductor wants to look at. Three staves in three
 * keys playing the same line look like three different lines, and the one
 * question a score has to answer at a glance is who is playing what.
 *
 * So this is a view. The score is not changed: a copy of it is moved to
 * concert pitch and that copy is drawn. Everything written in the editor
 * still goes into the real score as what the player reads — a position
 * clicked on a staff showing concert pitch is a sounding pitch, and it is
 * turned back into the written one on the way in.
 */

// concert holds the sounding copy of the score, made when it is asked for
// and kept until the music changes under it.
//
// A copy rather than a flag the engraver reads: moving a part to concert
// pitch is moving its notes and its key signature, which is work the
// engraver has no business knowing about, and the transposition is already
// written down as the interval to do it with.
type concertCache struct {
	mu    sync.Mutex
	of    *model.Score // what it was made from
	gen   uint64       // and when
	score *model.Score
}

// sounding is the score as it sounds, where that is what is being shown,
// and the score itself where it is not.
//
// Keyed by the score it was made from and the edit count, so it is made
// once and then handed out until something changes. The layout cache
// behind it keys on the score it is given, so a stable copy keeps that one
// working too — a fresh clone on every draw would engrave the whole piece
// again at every scroll.
func (a *App) sounding(s *model.Score) *model.Score {
	if !a.concert || s == nil {
		return s
	}
	gen := a.gen.Load()
	a.concert0.mu.Lock()
	defer a.concert0.mu.Unlock()
	if a.concert0.score != nil && a.concert0.of == s && a.concert0.gen == gen {
		return a.concert0.score
	}
	out := concertOf(s)
	a.concert0.of, a.concert0.gen, a.concert0.score = s, gen, out
	return out
}

// concertOf is a copy of a score with every part moved to concert pitch.
//
// The score itself is never touched: what is stored stays what each player
// reads, and this is a picture of how it sounds.
func concertOf(s *model.Score) *model.Score {
	// Nothing to move is nothing to copy, which is most scores: a piano,
	// a quartet, a choir and every lead sheet sound as they are written.
	// Copying them anyway would be a clone of the whole piece on every
	// draw, and a layout cache that never hits.
	moves := false
	for _, p := range s.Parts {
		if !p.Transpose.IsZero() {
			moves = true
			break
		}
	}
	if !moves {
		return s
	}
	out := s.Clone()
	for _, p := range out.Parts {
		if p.Transpose.IsZero() {
			continue
		}
		// The same move a change of instrument makes, to an instrument
		// that sounds as it is written: the notes go by the interval and
		// the signature goes with them.
		retranspose(p, p.Transpose, model.Transposition{})
		p.Transpose = model.Transposition{}
	}
	return out
}

// asWritten turns a pitch read off a staff showing concert pitch into the
// one the player reads, which is what is stored.
//
// Nothing to do on a staff that sounds as it is written, which is most of
// them, or when the score is being shown as written.
func (a *App) asWritten(p *model.Part, pitch model.Pitch) model.Pitch {
	if !a.concert || p == nil || p.Transpose.IsZero() {
		return pitch
	}
	t := p.Transpose
	return pitch.Transpose(-t.Steps(), -t.Semitones())
}
