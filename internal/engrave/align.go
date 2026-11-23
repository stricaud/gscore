/*
 * align.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"
	"sort"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * One moment, one x, on every staff.
 *
 * A score is read down as well as across: the eye goes down a column to
 * find what both hands play together. The bars were already lined up — every
 * staff's bar begins and ends at the same x — but inside a bar each staff
 * spread its own music over the width at its own rate. Two staves needing
 * much the same room came out nearly together, which is why it went
 * unnoticed; four quarters over eight eighths were already a third of a
 * space apart by the fourth beat. And a staff whose bar is half written
 * spread its half over the whole bar: a left hand's run of eighths written
 * up on the treble staff landed among the right hand's notes at moments
 * they do not sound at, a notehead printed on a notehead. Bar 28 of Black
 * and Tan Fantasy.
 *
 * So every moment that begins on any staff of the bar gets one x, as far
 * along as the staff that needs the most room before it asks, and every
 * staff's columns are set at those xs. A staff whose own next note is later
 * than the others' simply has a wider gap: the room it would have taken is
 * given over to the moments in between.
 */

// alignMoments sets the columns of bar mi on every staff at shared xs.
func (e *engraver) alignMoments(boxes [][]measureBox, mi int) {
	var parts []int
	for pi := range boxes {
		if mi >= len(boxes[pi]) {
			continue
		}
		b := boxes[pi][mi]
		if b.ahead || len(b.slots) == 0 || b.m == nil || b.m.Repeat > 0 {
			continue
		}
		parts = append(parts, pi)
	}
	if len(parts) < 2 {
		return
	}

	// Every moment, in order.
	key := func(s slot) float64 { return math.Round(s.at.Float()*1e9) / 1e9 }
	seen := map[float64]bool{}
	var moments []float64
	for _, pi := range parts {
		for _, s := range boxes[pi][mi].slots {
			if k := key(s); !seen[k] {
				seen[k] = true
				moments = append(moments, k)
			}
		}
	}
	sort.Float64s(moments)

	// Where each moment goes: as far along as any staff with a note there
	// needs it to be, after its own note before.
	x := map[float64]float64{}
	least := map[float64]float64{}
	type mark struct{ part, slot int }
	at := map[float64][]mark{}
	for _, pi := range parts {
		for k, s := range boxes[pi][mi].slots {
			at[key(s)] = append(at[key(s)], mark{pi, k})
		}
	}
	for i, t := range moments {
		want, floor := 0.0, 0.0
		// And past whatever began at the moment before, on any staff. A
		// staff's own next note may be later than this moment, but what
		// it has just begun still needs room before anything else comes:
		// a right hand resting through the last eighth of a left hand's
		// run left that eighth under a space before its own next note,
		// and the two were printed on top of each other.
		if i > 0 {
			prev := moments[i-1]
			span := durationOf(t - prev)
			for _, m := range at[prev] {
				s := boxes[m.part][mi].slots[m.slot]
				w := math.Max(e.spanWidth(span, s.elems), e.columnInk(s)+s.flag)
				want = math.Max(want, x[prev]+w)
				floor = math.Max(floor, least[prev]+math.Min(w, s.least))
			}
		}
		for _, m := range at[t] {
			b := boxes[m.part][mi]
			s := b.slots[m.slot]
			var w, f float64
			if m.slot == 0 {
				w = b.prefix + s.lead
				f = w
			} else {
				prev := b.slots[m.slot-1]
				w = x[key(prev)] + prev.width + s.lead
				f = least[key(prev)] + prev.least + s.lead
			}
			want = math.Max(want, w)
			floor = math.Max(floor, f)
		}
		x[t], least[t] = want, floor
	}

	// Where the bar ends: past the last note of every staff, and whatever
	// each staff keeps after its music — the barline's room, the last
	// syllable's half, the room left to write in.
	end := 0.0
	extra := make(map[int]float64, len(parts))
	for _, pi := range parts {
		b := boxes[pi][mi]
		used := b.prefix
		for _, s := range b.slots {
			used += s.lead + s.width
		}
		extra[pi] = b.ideal - used
		last := b.slots[len(b.slots)-1]
		end = math.Max(end, x[key(last)]+last.width+extra[pi])
	}

	for _, pi := range parts {
		b := &boxes[pi][mi]
		slots := append([]slot(nil), b.slots...)
		for k := range slots {
			t := key(slots[k])
			if k == 0 {
				slots[k].lead = x[t] - b.prefix
			} else {
				slots[k].lead = 0
			}
			if k+1 < len(slots) {
				next := key(slots[k+1])
				slots[k].width = x[next] - x[t]
				slots[k].least = math.Min(slots[k].width, least[next]-least[t])
			} else {
				slots[k].width = end - extra[pi] - x[t]
			}
		}
		b.slots = slots
		b.ideal = end
		b.aligned = true
	}
}

// durationOf turns a part of a bar, as a fraction, back into a duration,
// for the spacing to be asked about. Moments in a bar are a few hundred
// parts of a whole note at the very finest, so this is exact for them.
func durationOf(f float64) model.Duration {
	const den = 3 * 5 * 7 * 1024
	return model.Duration{Num: int(math.Round(f * den)), Den: den}
}

// startPrefix is the room every staff's bar mi leaves at its head for a
// change of key, meter or clef: what can come out of all of them at the
// start of a system, where the change is written in the system's head.
func startPrefix(boxes [][]measureBox, mi int) float64 {
	drop := math.Inf(1)
	for pi := range boxes {
		if mi >= len(boxes[pi]) || !boxes[pi][mi].aligned {
			continue
		}
		drop = math.Min(drop, boxes[pi][mi].prefix)
	}
	if math.IsInf(drop, 1) {
		return 0
	}
	return drop
}
