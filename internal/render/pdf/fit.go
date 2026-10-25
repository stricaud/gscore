/*
 * fit.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"fmt"
	"math"

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
		room := (o.PageHeight - o.MarginTop - o.MarginBottom) * fitSlack
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
	// Then: the largest of those sizes that does not waste a sheet.
	o = tighten(s, o, eo)
	eo.LineWidth = o.LineWidth(eo.Spacings)
	return o, eo
}

// PartStaffSpace is the smallest type a score is shrunk to merely to save
// paper, in points.
//
// Four and a half points is about one and a half millimetres, which is a
// printed orchestral part — smaller than the size a single song is set at
// and entirely ordinary to read from a stand. Below this is a pocket score,
// which is a thing somebody asks for and not a thing to be given a page at
// a time without being asked.
const PartStaffSpace = 4.6

// tightenStep is how finely the size is tried. Fine enough to find the
// largest type that saves the sheet, coarse enough not to engrave the score
// twenty times to find it.
const tightenStep = 0.3

// tighten shrinks the type where doing so saves whole pages.
//
// Choosing the size by "will one system fit the sheet" answers the question
// a conductor's score asks and no question a song asks: a song's system
// always fits, so the size never moves, and a hundred bars of it comes out
// across five sheets with half of each one white. A publisher setting the
// same song picks the type that gets it onto two, and that is what this
// does — the largest type that needs the fewest pages, rather than the
// largest type that fits at all.
//
// Only downwards, only as far as a printed part, and only for whole pages
// saved: the point is the sheet count, so a size that merely leaves less
// white at the bottom of the last page buys nothing and costs legibility.
func tighten(sc *model.Score, o Options, eo engrave.Options) Options {
	best := o
	fewest := pagesAt(sc, o, eo)
	if fewest <= 1 {
		return o // one sheet is as few as there are
	}
	for size := o.StaffSpace - tightenStep; size >= PartStaffSpace; size -= tightenStep {
		try := o
		try.StaffSpace = size
		n := pagesAt(sc, try, eo)
		if n < fewest {
			fewest, best = n, try
		}
		if fewest <= 1 {
			break
		}
	}
	return best
}

// pagesAt is how many sheets the score comes to at a type size.
func pagesAt(sc *model.Score, o Options, eo engrave.Options) int {
	eo.LineWidth = o.LineWidth(eo.Spacings)
	lay := engrave.Engrave(sc, eo)
	if lay == nil {
		return 0
	}
	// The same cut the renderer makes, so that this counts the pages that
	// will actually be printed and not an idea of them.
	return len((&renderer{lay: lay, opt: o}).pages())
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

// Divided says whether a system had to be cut across pages, and what to do
// about it.
//
// A score of many staves on a small sheet reaches a point where no readable
// type size will fit one system on one page — seventeen staves on A4 on its
// side is past it. Rather than print the band with its bottom staves sliced
// off by the paper's edge, the page is divided between staves and the system
// continues overleaf. That is the right answer to an impossible request, and
// it is also the sort of thing somebody wants to be told about, because the
// usual cause is the paper rather than the music.
//
// The message names the way out where there is one. Turning the sheet is the
// usual way out and is counter-intuitive: a conductor's score is wider than
// it is tall, so landscape feels right, and landscape is the orientation
// with less room for staves.
func Divided(s *model.Score, lay *engrave.Layout, o Options) string {
	usable := (o.PageHeight - o.MarginTop - o.MarginBottom) / o.StaffSpace
	tallest, staves := 0.0, 0
	for _, sys := range lay.Systems {
		if h := sys.Bottom - sys.Top; h > tallest {
			tallest = h
			staves = len(s.Parts)
		}
	}
	if tallest <= usable || usable <= 0 {
		return ""
	}
	pages := int(math.Ceil(tallest / usable))

	msg := fmt.Sprintf(
		"%d staves need %.0fpt and the page gives %.0fpt, so each system is "+
			"divided across %d pages",
		staves, tallest*o.StaffSpace, usable*o.StaffSpace, pages)

	// Would turning the paper hold it whole? Only worth saying if it would.
	other := o
	other.PageWidth, other.PageHeight = o.PageHeight, o.PageWidth
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	eo.ShowPartNames = len(s.Parts) > 1
	eo.ShowMeasureNumbers = true
	fitted, feo := Fit(s, other, eo)
	alt := engrave.Engrave(s, feo)
	room := (fitted.PageHeight - fitted.MarginTop - fitted.MarginBottom) / fitted.StaffSpace
	whole := true
	for _, sys := range alt.Systems {
		if sys.Bottom-sys.Top > room {
			whole = false
		}
	}
	if whole {
		way := "upright"
		if fitted.PageWidth > fitted.PageHeight {
			way = "on its side"
		}
		msg += fmt.Sprintf("; the same paper %s holds a system whole", way)
	}
	return msg
}
