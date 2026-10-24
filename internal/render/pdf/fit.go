/*
 * fit.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package pdf

import (
	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// Choosing the type size from the music.
//
// A staff space of 6.2 points is what a printed part uses, and for a song on
// one staff it is right. For a conductor's score it is not: seventeen staves
// at that size make a system twice the height of A4, and a system that will
// not fit the page is printed overflowing it — the first page of La Maritza
// carried four staves of the seventeen and dropped the other thirteen, every
// page, for fifty pages of a sixty-three bar song.
//
// So the size is a decision to be made from the music rather than a constant.
// This is what every engraver does and what every program that prints music
// offers: LilyPond calls it the global staff size, MuseScore and Finale call
// it scaling, and a publisher calls it the rastral. The rule is the same in
// all of them — pick the largest type the page will hold a system of.
//
// It only ever goes smaller. A size that was asked for is a size not to
// exceed, and a page with room to spare is not a reason to blow a song up.

// MinStaffSpace is the smallest type this will choose, in points.
//
// About one and a quarter millimetres, which is a pocket score: small, and
// the bottom of what is honestly readable. Below it there is no point
// shrinking further — the music does not become printable, it becomes
// unreadable at a different size — so a score that will not fit even here is
// printed too big and left to say so.
const MinStaffSpace = 3.5

// fitSlack is how much of the page a system is allowed to need.
//
// Not all of it. A system fitted to the very edge of the sheet sits on the
// limit, where the next thing to be added to it — a bar number, a line of
// words under the last staff — pushes it over and the page breaks in two.
// Two per cent is a couple of millimetres of air and costs nothing: the type
// size it buys back is in the second decimal place.
const fitSlack = 0.98

// Fit chooses the type size so that a system fits the page, and tells the
// engraver how long a line that leaves it.
//
// It returns both halves because they are one decision: how big the music is
// set and how much of it goes on a line are the same number seen twice, and
// the two drifting apart is how a system comes to be engraved wider than the
// sheet it is printed on.
//
// The caller engraves with the options that come back. The measuring is done
// by engraving, because the height of a system is not a thing to be guessed
// at from the number of staves — the words under one, the chord symbols over
// another and the room its marks need are all in it.
func Fit(s *model.Score, o Options, eo engrave.Options) (Options, engrave.Options) {
	if o.StaffSpace <= 0 {
		o.StaffSpace = DefaultOptions().StaffSpace
	}
	// Three goes at it. Shrinking the type lengthens the line, which puts
	// more bars in a system and can change what has to be written above it,
	// so the height is not quite settled until it has been engraved at the
	// size it will be printed at. It converges in one or two.
	for i := 0; i < 3; i++ {
		eo.LineWidth = o.LineWidth(eo.Spacings)
		need := tallestSystem(engrave.Engrave(s, eo))
		room := (o.PageHeight - o.musicTop() - o.MarginBottom) * fitSlack
		if need <= 0 || need*o.StaffSpace <= room {
			break // it fits
		}
		want := room / need
		if want < MinStaffSpace {
			want = MinStaffSpace
		}
		if want >= o.StaffSpace {
			break // no smaller than it already is: nothing left to try
		}
		o.StaffSpace = want
	}
	eo.LineWidth = o.LineWidth(eo.Spacings)
	return o, eo
}

// tallestSystem is the height of the system that needs the most room, in
// staff spaces.
//
// The first system is measured from the top of the drawing rather than from
// its own top, because the title block sits above it and is printed on the
// same page.
func tallestSystem(lay *engrave.Layout) float64 {
	if lay == nil || len(lay.Systems) == 0 {
		return 0
	}
	tallest := lay.Systems[0].Bottom // from zero: the title block is in this
	for _, sys := range lay.Systems[1:] {
		if h := sys.Bottom - sys.Top; h > tallest {
			tallest = h
		}
	}
	return tallest
}
