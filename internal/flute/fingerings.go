/*
 * fingerings.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package flute carries the fingering table for the Boehm flute.
//
// Every note of the instrument, with the standard fingering everybody
// learns and every alternative the source lists — some twenty-nine
// thousand of them. The alternatives are the point of having this rather
// than the chart in the back of a method book: the trill fingerings, the
// ones that take the gizmo, and for any note the whole family of
// combinations that sound it, with how far out of tune each one comes out.
//
// Nothing here says which alternative a player should reach for, because
// the source does not say either. What it gives is the two things that can
// be measured — how far the fingering is from the standard one, and how
// far out of tune it sits — and those are enough to put the plausible ones
// first and leave the acoustic curiosities at the end.
//
// The table is generated from a scrape: see mkdata.py, which turns three
// and a half megabytes of text into three hundred kilobytes of numbers. A
// fingering is one integer, a bit per key, and the key order is the format.
package flute

import (
	_ "embed"
	"encoding/json"
	"math/bits"
	"sort"
	"strings"
	"sync"
)

//go:embed fingerings.json
var table []byte

// Fingering is one way of playing a note: which keys are down, and how far
// out of tune it comes out in cents.
type Fingering struct {
	Keys  uint32 `json:"keys"`
	Cents int    `json:"cents"`
}

// Note is everything known about one note of the instrument.
type Note struct {
	Name     string      `json:"name"`
	Standard Fingering   `json:"standard"`
	Others   []Fingering `json:"-"`
}

type loaded struct {
	Keys  []string `json:"keys"`
	Left  int      `json:"left"`
	Notes map[string]struct {
		Std [2]int   `json:"std"`
		Alt [][2]int `json:"alt"`
	} `json:"notes"`
}

var (
	once  sync.Once
	keys  []string
	left  int
	notes map[string]*Note
	order []string
)

func load() {
	once.Do(func() {
		var in loaded
		if err := json.Unmarshal(table, &in); err != nil {
			// The table is built into the program, so this cannot happen
			// in a program that was built. An empty one still answers.
			notes = map[string]*Note{}
			return
		}
		keys, left = in.Keys, in.Left
		notes = make(map[string]*Note, len(in.Notes))
		for name, n := range in.Notes {
			note := &Note{
				Name:     name,
				Standard: Fingering{Keys: uint32(n.Std[0]), Cents: n.Std[1]},
				Others:   make([]Fingering, 0, len(n.Alt)),
			}
			for _, a := range n.Alt {
				note.Others = append(note.Others,
					Fingering{Keys: uint32(a[0]), Cents: a[1]})
			}
			notes[name] = note
			order = append(order, name)
		}
		sort.Slice(order, func(i, j int) bool {
			return pitchOf(order[i]) < pitchOf(order[j])
		})
	})
}

// pitchOf puts the note names in the order the instrument plays them,
// which is not the order they sort in as text: A4 comes after G4 and
// before A sharp 4, and C7 comes last of all.
func pitchOf(name string) int {
	step := map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}
	if name == "" {
		return 0
	}
	semi, ok := step[name[0]]
	if !ok {
		return 0
	}
	rest := name[1:]
	if strings.HasPrefix(rest, "#") {
		semi++
		rest = rest[1:]
	} else if strings.HasPrefix(rest, "b") {
		semi--
		rest = rest[1:]
	}
	octave := 0
	for i := 0; i < len(rest); i++ {
		if rest[i] >= '0' && rest[i] <= '9' {
			octave = octave*10 + int(rest[i]-'0')
		}
	}
	return octave*12 + semi
}

// Keys are the keys of the instrument, in the order a chart is written:
// the left hand, then the right. The order is the format of the table —
// a fingering is a bit per key, in this order.
func Keys() []string {
	load()
	return keys
}

// LeftHand is how many of Keys belong to the left hand.
func LeftHand() int {
	load()
	return left
}

// Names is every note the flute plays, lowest first.
func Names() []string {
	load()
	return append([]string(nil), order...)
}

// Of is everything known about one note.
func Of(name string) (*Note, bool) {
	load()
	n, ok := notes[name]
	return n, ok
}

// Spell writes a fingering the way a chart does: the left hand, a bar, the
// right hand.
func Spell(f uint32) string {
	load()
	var l, r []string
	for i, k := range keys {
		if f&(1<<uint(i)) == 0 {
			continue
		}
		if i < left {
			l = append(l, k)
		} else {
			r = append(r, k)
		}
	}
	return strings.TrimSpace(strings.Join(l, " ") + " | " + strings.Join(r, " "))
}

// Has reports whether a fingering uses a named key.
func Has(f uint32, key string) bool {
	load()
	for i, k := range keys {
		if k == key {
			return f&(1<<uint(i)) != 0
		}
	}
	return false
}

// Away is how many keys differ between two fingerings.
//
// The measure that sorts the alternatives: one key away from the chart is
// the alternate a player would reach for, and eleven keys away is the
// acoustic model saying a note can be got out of a shape nobody would use.
func Away(a, b uint32) int { return bits.OnesCount32(a ^ b) }

// Trill is one way of trilling between two notes: hold the fingering, and
// move the one key.
type Trill struct {
	// Hold is the fingering of the lower note, Key the one key that moves,
	// and Down says the key goes down to sound the upper note rather than
	// coming up.
	Hold Fingering `json:"hold"`
	Key  string    `json:"key"`
	Down bool      `json:"down"`
	// Upper is what the other note comes out at, in cents, and Away how
	// far Hold is from the standard chart for the lower note.
	Upper int `json:"upper"`
	Away  int `json:"away"`
}

// Trills finds the ways of trilling from one note to another.
//
// A trill fingering is not something the source labels, and it does not
// have to be: it is a fingering of the one note and a fingering of the
// other that differ by a single key. That is what a trill is on a flute —
// everything stays where it is and one finger shakes — and it is why the
// trill charts in the back of a method book exist at all, because for
// several pairs the single key that moves is not the one the two standard
// fingerings differ by.
//
// Ordered by how near the held shape is to the standard chart for the
// lower note, then by how far the two notes are out of tune. A trill on
// the ordinary fingering comes first, which is what a player wants, and
// the curiosities sort to the end.
func Trills(from, to string) []Trill {
	load()
	a, ok := notes[from]
	if !ok {
		return nil
	}
	b, ok := notes[to]
	if !ok {
		return nil
	}
	// Every fingering of the upper note, to look a neighbour up in.
	upper := make(map[uint32]int, len(b.Others)+1)
	upper[b.Standard.Keys] = b.Standard.Cents
	for _, f := range b.Others {
		upper[f.Keys] = f.Cents
	}

	out := []Trill{}
	seen := map[[2]uint32]bool{}
	hold := append([]Fingering{a.Standard}, a.Others...)
	for _, f := range hold {
		for i, k := range keys {
			bit := uint32(1) << uint(i)
			other := f.Keys ^ bit
			cents, ok := upper[other]
			if !ok {
				continue
			}
			pair := [2]uint32{f.Keys, other}
			if seen[pair] {
				continue
			}
			seen[pair] = true
			out = append(out, Trill{
				Hold: f, Key: k, Down: f.Keys&bit == 0,
				Upper: cents, Away: Away(f.Keys, a.Standard.Keys),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Away != out[j].Away {
			return out[i].Away < out[j].Away
		}
		oi := abs(out[i].Hold.Cents) + abs(out[i].Upper)
		oj := abs(out[j].Hold.Cents) + abs(out[j].Upper)
		return oi < oj
	})
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
