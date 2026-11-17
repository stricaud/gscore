/*
 * bars.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package lilypond

/*
 * Laying a staff's events into bars.
 *
 * The events come out of expand.go as one stream with a place and a length
 * each. A score is bars, so the stream is cut at the barlines: a note that
 * reaches over one is split and the pieces tied, which is what the note
 * means and what the model wants written down.
 *
 * The grid of bars comes from the music itself: the meters written in it, a
 * \partial for an upbeat, a bar of another length set by hand, and how long
 * the whole thing lasts. Timing is one thing for the whole score in
 * LilyPond, so the meters of every staff count, whichever staff they were
 * written on.
 */

import (
	"sort"

	"github.com/stricaud/gscore/internal/model"
)

type bar struct {
	at, length model.Duration
	time       *[2]int // where the meter is written at this bar
}

// A placed event in a bar: its own event, cut to the bar, and the pieces of
// marks that go with it.
type placed struct {
	e        *event
	tiedBack bool
	tiedOn   bool
}

type measure struct {
	events      []*placed
	time        *[2]int
	barline     string
	repeatStart bool
	endings     [][3]int // number, start(1)/stop(0), how many endings in all
	attrs       []*mark  // what is written at a place in the bar other than a note
}

type timeAt struct {
	at   model.Duration
	time [2]int
}

type lengthAt struct {
	at     model.Duration
	length model.Duration
}

// places walks a stream counting its length, noting where each meter and
// the upbeat are met. By counting rather than by the places the events
// carry, so that a stream of skips made by hand says the same as one that
// was timed.
func places(stream []any, times *[]timeAt, lengths *[]lengthAt, partial *model.Duration, at model.Duration) model.Duration {
	for _, x := range stream {
		switch e := x.(type) {
		case *event:
			at = at.Add(e.dur)
		case *voicesItem:
			end := at
			for _, sub := range e.subs {
				if subEnd := places(sub, times, lengths, partial, at); subEnd.Cmp(end) > 0 {
					end = subEnd
				}
			}
			at = end
		case *mark:
			switch e.kind {
			case "time":
				if len(e.args) == 2 {
					*times = append(*times, timeAt{at: at, time: [2]int{e.args[0].(int), e.args[1].(int)}})
				}
			case "measureLength":
				if d, ok := e.args[0].(model.Duration); ok {
					*lengths = append(*lengths, lengthAt{at: at, length: d})
				}
			case "partial":
				if d, ok := e.args[0].(*model.Duration); ok && d != nil && at.IsZero() {
					*partial = *d
				}
			}
		}
	}
	return at
}

// grid is the bars of the piece, from what is written in every staff.
//
// The meter in force decides each bar's length, a \partial makes the first
// bar the upbeat's length, and the bars go on until the longest staff runs
// out. A meter written partway through takes effect at the bar it is
// written at.
func grid(streams [][]any) []bar {
	var times []timeAt
	var lengths []lengthAt
	// Zero as a fraction: a Duration's zero value has no denominator, and
	// compares equal to everything.
	partial := model.Duration{Num: 0, Den: 1}
	end := model.Duration{Num: 0, Den: 1}
	for _, s := range streams {
		if e := places(s, &times, &lengths, &partial, model.Duration{Num: 0, Den: 1}); e.Cmp(end) > 0 {
			end = e
		}
	}
	timeAtPlace := func(at model.Duration) (*[2]int, bool) {
		for i := range times {
			if times[i].at.Cmp(at) == 0 {
				t := times[i].time
				return &t, true
			}
		}
		return nil, false
	}
	lengthAtPlace := func(at model.Duration) (model.Duration, bool) {
		for i := range lengths {
			if lengths[i].at.Cmp(at) == 0 {
				return lengths[i].length, true
			}
		}
		return model.Duration{}, false
	}

	var bars []bar
	at := model.Duration{Num: 0, Den: 1}
	beats, value := 4, 4
	if t, ok := timeAtPlace(at); ok {
		beats, value = t[0], t[1]
	}
	var fixed *model.Duration
	for at.Cmp(end) < 0 || len(bars) == 0 {
		var written *[2]int
		if t, ok := timeAtPlace(at); ok {
			beats, value = t[0], t[1]
			fixed = nil
			written = t
		}
		if l, ok := lengthAtPlace(at); ok {
			fixed = &l
		}
		length := model.Duration{Num: beats, Den: value}.Reduced()
		if fixed != nil {
			length = *fixed
		}
		if len(bars) == 0 {
			t := [2]int{beats, value}
			written = &t
			if !partial.IsZero() {
				length = partial
			}
		}
		if length.Num <= 0 {
			length = model.Duration{Num: 1, Den: 1}
		}
		bars = append(bars, bar{at: at, length: length, time: written})
		at = at.Add(length)
	}
	return bars
}

func barAt(bars []bar, at model.Duration) int {
	lo, hi := 0, len(bars)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		b := bars[mid]
		switch {
		case at.Cmp(b.at) < 0:
			hi = mid - 1
		case at.Cmp(b.at.Add(b.length)) >= 0:
			lo = mid + 1
		default:
			return mid
		}
	}
	return -1
}

func barEndingAt(bars []bar, at model.Duration) int {
	for i, b := range bars {
		if b.at.Add(b.length).Cmp(at) == 0 {
			return i
		}
	}
	return barAt(bars, at)
}

// layOut cuts one staff's events into the bars they fall in.
//
// Each event knows where it is, so the voices of a bar — timed from the same
// place — land in the same bar, each with its voice number. The marks go to
// the bar they fall in, at their place in it, and a barline to the bar it
// closes.
func layOut(stream []any, bars []bar) []*measure {
	return layOutEvents(eventsIn(stream), marksIn(stream), bars)
}

// layOutEvents is layOut over events and marks already picked out, which
// is how a staff takes in a voice that moved onto it from the staff beside.
func layOutEvents(events []*event, marks []*mark, bars []bar) []*measure {
	out := make([]*measure, len(bars))
	for i, b := range bars {
		out[i] = &measure{time: b.time}
	}
	if len(bars) == 0 {
		return out
	}
	last := len(bars) - 1
	for _, mk := range marks {
		switch mk.kind {
		case "bar":
			style, _ := mk.args[0].(string)
			if style == "|:" {
				if i := barAt(bars, mk.at); i >= 0 {
					out[i].repeatStart = true
				}
				continue
			}
			if i := barEndingAt(bars, mk.at); i >= 0 {
				out[i].barline = style
			}
		case "ending":
			number, _ := mk.args[0].(int)
			what, _ := mk.args[1].(string)
			count, _ := mk.args[2].(int)
			i := barAt(bars, mk.at)
			flag := 1
			if what == "stop" {
				i = barEndingAt(bars, mk.at)
				flag = 0
			}
			if i >= 0 {
				out[i].endings = append(out[i].endings, [3]int{number, flag, count})
			}
		case "clef", "key", "tempo", "mark", "transposition", "numericTime",
			"instrumentName", "shortInstrumentName", "midiInstrument":
			i := barAt(bars, mk.at)
			if i < 0 {
				i = last
			}
			out[i].attrs = append(out[i].attrs, mk)
		}
	}
	for _, e := range events {
		if e.dur.IsZero() {
			i := barAt(bars, e.at)
			if i < 0 {
				i = last
			}
			out[i].events = append(out[i].events, &placed{e: e})
			continue
		}
		start, left := e.at, e.dur
		first := true
		for left.Num > 0 {
			i := barAt(bars, start)
			if i < 0 {
				break
			}
			end := bars[i].at.Add(bars[i].length)
			take := end.Sub(start)
			if left.Cmp(take) < 0 {
				take = left
			}
			piece := &placed{e: pieceOf(e, start, take), tiedBack: !first, tiedOn: take.Cmp(left) < 0}
			out[i].events = append(out[i].events, piece)
			start = start.Add(take)
			left = left.Sub(take)
			first = false
		}
	}
	for _, m := range out {
		sort.SliceStable(m.events, func(a, b int) bool {
			ea, eb := m.events[a].e, m.events[b].e
			if ea.voice != eb.voice {
				return ea.voice < eb.voice
			}
			return ea.at.Cmp(eb.at) < 0
		})
	}
	return out
}

// pieceOf is one bar's worth of an event.
func pieceOf(e *event, at, dur model.Duration) *event {
	if at.Cmp(e.at) == 0 && dur.Cmp(e.dur) == 0 {
		return e
	}
	c := *e
	c.at, c.dur = at, dur
	return &c
}
