/*
 * reach.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// How far a part reaches, and whether the instrument it is written for can
// go there.
//
// This is the check a composer cannot do for themselves. Everything else
// about a part can be heard by reading it: whether the tune is any good,
// whether the rhythm sits, whether the harmony moves. Whether a horn can
// play a written passage cannot — it depends on the instrument and not on
// the music, and somebody who does not play the horn has no way of knowing
// and no way of finding out except by asking a horn player.
//
// Two different things come out of it, and they are not the same kind of
// answer. A note outside the instrument altogether is a fact: it will not be
// played, and the part has to change. A part sitting above the comfortable
// stretch is a judgement: the top of a trumpet part is where the piece
// arrives, and forty bars up there is a player with no lip left by the slow
// movement. The first is reported as impossible, the second as something to
// look at, and they are never run together.

package analysis

import (
	"sort"
	"strings"
)

// Sounded is one note as it sounds, which is the only way a range can be
// asked about: a clarinet's bottom note is the same note however the part
// is spelled.
type Sounded struct {
	// Part is the staff it is on, MIDI how high it sounds.
	Part, MIDI int
	// Bar is which bar it is in, Ref the notehead it was drawn as, so the
	// answer can point at the place rather than only name it.
	Bar, Ref int
	// Held is how long it sounds, in whole notes. The extremes of a part
	// are about the notes; where the part *lives* is about the time, and a
	// top C touched once in four hundred bars is not where the part lives.
	Held float64
}

// Instrument is what one staff is written for, as the reach analysis needs
// it: a name to say, and what it can play if that is known.
type Instrument struct {
	// Staff is what the staff is called on the page — "Violini I", "voice
	// 2" — and Name what it is played on. They are different questions and
	// the answer has to give both: "Violini I" is who, "violin" is what
	// the range belongs to, and a finding that says one when it means the
	// other sends somebody to the wrong staff.
	Staff string
	Name  string
	Known bool
	// Percussion says the staff carries drums. Its lines are places on a
	// kit rather than pitches, so the lowest and the highest of them mean
	// nothing and the question is not asked: a bass drum written on the
	// bottom line is not a bass drum playing low.
	Percussion bool
	// Low and High are the ends of the instrument; EasyLow and EasyHigh
	// the stretch inside them where it plays without working for it.
	Low, High         int
	EasyLow, EasyHigh int
}

// Span is a stretch of pitch, in sounding MIDI numbers.
type Span struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

// Placed is one note and where to find it.
type Placed struct {
	MIDI int    `json:"midi"`
	Name string `json:"name"`
	Bar  int    `json:"bar"`
	Ref  int    `json:"ref"`
}

// Finding is one thing worth saying about a part's range.
type Finding struct {
	// Severity is "impossible" for a note the instrument does not have,
	// and "strain" for a stretch it has but would rather not hold.
	Severity string `json:"severity"`
	// What it is, in a sentence somebody can act on.
	Text string `json:"text"`
	// Where: the bars it covers, and the worst note in them.
	FromBar int    `json:"fromBar"`
	ToBar   int    `json:"toBar"`
	Worst   Placed `json:"worst"`
	// Notes is how many notes the finding is about.
	Notes int `json:"notes"`
}

// PartReach is everything the panel says about one staff.
type PartReach struct {
	Part       int    `json:"part"`
	Name       string `json:"name"`
	Instrument string `json:"instrument"`

	// Notes is how many sounding notes the staff has at all. Zero means
	// there is nothing to say and nothing is said.
	Notes int `json:"notes"`
	// Percussion says the staff carries drums, and that nothing below it
	// is filled in.
	Percussion bool `json:"percussion,omitempty"`
	// Lowest and Highest are the two ends of what is written.
	Lowest  Placed `json:"lowest"`
	Highest Placed `json:"highest"`
	// Tessitura is where the part spends its time rather than where it
	// reaches: the stretch holding the middle four fifths of the sounding
	// time. It is what a player means by "this part sits high".
	Tessitura Span `json:"tessitura"`

	// Playable and Easy are the instrument's own, and are left out where
	// gscore does not know the instrument. A guess would be worse than a
	// blank: a composer who cannot tell which is which cannot use either.
	Playable *Span `json:"playable,omitempty"`
	Easy     *Span `json:"easy,omitempty"`

	Findings []Finding `json:"findings"`
}

// StrainBars is how many bars in a row a part has to sit outside the
// comfortable stretch before it is worth saying so.
//
// One bar at the top is a climax. Four is a tessitura, and a tessitura is
// the thing that tires a player out — which is the whole reason the easy
// range is written down separately from the range.
const StrainBars = 4

// Reaches works out what each staff covers.
//
// notes may be in any order and is not modified. parts is indexed by staff
// number; a staff with no entry is still reported, named by its number.
func Reaches(notes []Sounded, parts []Instrument, name func(midi int) string) []PartReach {
	if name == nil {
		name = func(int) string { return "" }
	}
	byPart := map[int][]Sounded{}
	var order []int
	for _, n := range notes {
		if _, seen := byPart[n.Part]; !seen {
			order = append(order, n.Part)
		}
		byPart[n.Part] = append(byPart[n.Part], n)
	}
	// Every staff that has an instrument, in staff order, whether or not
	// anything is written on it: a staff that is empty is a fact about the
	// score and saying nothing about it looks like a staff that was missed.
	for i := range parts {
		if _, seen := byPart[i]; !seen {
			order = append(order, i)
			byPart[i] = nil
		}
	}
	sort.Ints(order)

	out := make([]PartReach, 0, len(order))
	for _, pi := range order {
		out = append(out, reachOf(pi, byPart[pi], instrumentAt(parts, pi), name))
	}
	return out
}

func instrumentAt(parts []Instrument, i int) Instrument {
	if i >= 0 && i < len(parts) {
		return parts[i]
	}
	return Instrument{}
}

func reachOf(part int, notes []Sounded, inst Instrument, name func(int) string) PartReach {
	staff := inst.Staff
	if staff == "" {
		staff = inst.Name
	}
	r := PartReach{
		Part: part, Name: staff, Instrument: inst.Name,
		Notes: len(notes), Percussion: inst.Percussion, Findings: []Finding{},
	}
	if len(notes) == 0 || inst.Percussion {
		return r
	}

	lo, hi := notes[0], notes[0]
	for _, n := range notes {
		if n.MIDI < lo.MIDI {
			lo = n
		}
		if n.MIDI > hi.MIDI {
			hi = n
		}
	}
	r.Lowest = Placed{MIDI: lo.MIDI, Name: name(lo.MIDI), Bar: lo.Bar, Ref: lo.Ref}
	r.Highest = Placed{MIDI: hi.MIDI, Name: name(hi.MIDI), Bar: hi.Bar, Ref: hi.Ref}
	r.Tessitura = tessitura(notes)

	if inst.Known {
		r.Playable = &Span{Low: inst.Low, High: inst.High}
		r.Easy = &Span{Low: inst.EasyLow, High: inst.EasyHigh}
		r.Findings = append(r.Findings, beyond(notes, inst, name)...)
		r.Findings = append(r.Findings, strain(notes, inst, name)...)
	}
	return r
}

// tessitura is the stretch holding the middle four fifths of the sounding
// time: the top and the bottom tenth are thrown away, so one touched high
// note does not make a part a high part.
func tessitura(notes []Sounded) Span {
	byPitch := append([]Sounded(nil), notes...)
	sort.Slice(byPitch, func(i, j int) bool { return byPitch[i].MIDI < byPitch[j].MIDI })

	total := 0.0
	for _, n := range byPitch {
		total += weight(n)
	}
	if total <= 0 {
		return Span{Low: byPitch[0].MIDI, High: byPitch[len(byPitch)-1].MIDI}
	}
	at := func(share float64) int {
		want, seen := total*share, 0.0
		for _, n := range byPitch {
			seen += weight(n)
			if seen >= want {
				return n.MIDI
			}
		}
		return byPitch[len(byPitch)-1].MIDI
	}
	return Span{Low: at(0.1), High: at(0.9)}
}

// weight is how much of the part's time a note accounts for. A note with no
// length recorded still counts for something, or a part written entirely in
// grace notes would have no tessitura at all.
func weight(n Sounded) float64 {
	if n.Held <= 0 {
		return 1.0 / 64
	}
	return n.Held
}

// beyond reports the notes the instrument does not have. Those are not a
// matter of opinion: they will not be played.
func beyond(notes []Sounded, inst Instrument, name func(int) string) []Finding {
	var above, below []Sounded
	for _, n := range notes {
		switch {
		case n.MIDI > inst.High:
			above = append(above, n)
		case n.MIDI < inst.Low:
			below = append(below, n)
		}
	}
	var out []Finding
	if f, ok := impossible(above, inst, name, true); ok {
		out = append(out, f)
	}
	if f, ok := impossible(below, inst, name, false); ok {
		out = append(out, f)
	}
	return out
}

func impossible(notes []Sounded, inst Instrument, name func(int) string, high bool) (Finding, bool) {
	if len(notes) == 0 {
		return Finding{}, false
	}
	worst, from, to := notes[0], notes[0].Bar, notes[0].Bar
	for _, n := range notes {
		if (high && n.MIDI > worst.MIDI) || (!high && n.MIDI < worst.MIDI) {
			worst = n
		}
		if n.Bar < from {
			from = n.Bar
		}
		if n.Bar > to {
			to = n.Bar
		}
	}
	end, where, edge := inst.High, "above", "top"
	if !high {
		end, where, edge = inst.Low, "below", "bottom"
	}
	text := plural(len(notes), "note") + " " + where + " " + name(end) +
		", the " + edge + " of " + possessive(inst.Name) + " range"
	return Finding{
		Severity: "impossible", Text: text,
		FromBar: from, ToBar: to, Notes: len(notes),
		Worst: Placed{MIDI: worst.MIDI, Name: name(worst.MIDI), Bar: worst.Bar, Ref: worst.Ref},
	}, true
}

// barReach is what one bar of a part covers, and whether the whole of it
// sits outside the comfortable stretch.
type barReach struct {
	lo, hi  Sounded
	allHigh bool
	allLow  bool
}

// strain reports stretches sitting outside the comfortable range.
//
// By the bar, and only where every note in the bar is outside it: a bar that
// touches the top and comes back down is a phrase, not a tessitura. A run of
// such bars is what tires a player out, so one bar is never reported and
// four in a row are.
func strain(notes []Sounded, inst Instrument, name func(int) string) []Finding {
	bars := map[int]*barReach{}
	var seen []int
	for _, n := range notes {
		b, ok := bars[n.Bar]
		if !ok {
			b = &barReach{lo: n, hi: n, allHigh: true, allLow: true}
			bars[n.Bar] = b
			seen = append(seen, n.Bar)
		}
		if n.MIDI < b.lo.MIDI {
			b.lo = n
		}
		if n.MIDI > b.hi.MIDI {
			b.hi = n
		}
		if n.MIDI <= inst.EasyHigh {
			b.allHigh = false
		}
		if n.MIDI >= inst.EasyLow {
			b.allLow = false
		}
	}
	sort.Ints(seen)

	out := append(runs(seen, bars, inst, name, true), runs(seen, bars, inst, name, false)...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].FromBar < out[j].FromBar })
	return out
}

// runs walks the bars in order and reports each unbroken stretch of them
// that sits outside the comfortable range.
//
// Unbroken in bars of this part, not in bars of the piece: a part that rests
// for a bar in the middle of a high passage has not come down, and counting
// the rest as a break would split the one thing worth reporting into two
// stretches too short to report.
func runs(seen []int, bars map[int]*barReach, inst Instrument,
	name func(int) string, high bool) []Finding {

	var out []Finding
	i := 0
	for i < len(seen) {
		outside := func(at int) bool {
			b := bars[seen[at]]
			if high {
				return b.allHigh && b.hi.MIDI <= inst.High
			}
			return b.allLow && b.lo.MIDI >= inst.Low
		}
		if !outside(i) {
			i++
			continue
		}
		j := i
		for j+1 < len(seen) && outside(j+1) {
			j++
		}
		if j-i+1 >= StrainBars {
			out = append(out, strainFinding(seen[i:j+1], bars, inst, name, high))
		}
		i = j + 1
	}
	return out
}

func strainFinding(run []int, bars map[int]*barReach, inst Instrument,
	name func(int) string, high bool) Finding {

	worst := bars[run[0]].hi
	if !high {
		worst = bars[run[0]].lo
	}
	notes := 0
	for _, at := range run {
		b := bars[at]
		if high && b.hi.MIDI > worst.MIDI {
			worst = b.hi
		}
		if !high && b.lo.MIDI < worst.MIDI {
			worst = b.lo
		}
		notes++
	}
	edge, where, end := inst.EasyHigh, "above", "top"
	if !high {
		edge, where, end = inst.EasyLow, "below", "bottom"
	}
	// The count is of bars with notes in them, which is what tires a
	// player out. It is not a claim that they run consecutively: a part
	// that rests for a bar in the middle of a high passage has not come
	// down, so "four bars in a row" once covered nineteen bars of score
	// and the sentence was not true of anything.
	text := plural(len(run), "bar") + " " + where + " " + name(edge) +
		", the " + end + " of " + possessive(inst.Name) + " comfortable range"
	return Finding{
		Severity: "strain", Text: text,
		FromBar: run[0], ToBar: run[len(run)-1], Notes: notes,
		Worst: Placed{MIDI: worst.MIDI, Name: name(worst.MIDI), Bar: worst.Bar, Ref: worst.Ref},
	}
}

// possessive names an instrument as the owner of its range. A name already
// ending in s takes the apostrophe alone, or the strings end up with a
// range belonging to "the strings's".
func possessive(name string) string {
	if strings.HasSuffix(name, "s") {
		return "the " + name + "'"
	}
	return "the " + name + "'s"
}

// plural counts a thing in words, because "1 notes" reads as a bug in the
// program rather than as one note.
func plural(n int, what string) string {
	if n == 1 {
		return "One " + what
	}
	return itoa(n) + " " + what + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
