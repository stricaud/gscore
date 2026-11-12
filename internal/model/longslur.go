/*
 * longslur.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Slurs that reach further than a slur reaches.
 *
 * A phrase mark is over a phrase: a bar or two, three for a long line. One
 * that runs for six bars, or for the rest of the piece, can be what the
 * composer wrote — but nearly always it is a reading mistake: a mark whose
 * far end was dropped, or dealt into the other voice of a divided staff, so
 * the engraver drew the curve on to the next end it could find. Three such
 * came out of one NoteWorthy file in an afternoon, each from a different
 * fault, and nothing in gscore said so; the page did.
 *
 * So this is the check: every slur, measured in bars from the element that
 * opens it to the one that closes it, and the ones past the limit listed
 * with where they are. Importers report them, so that a file that reads
 * wrong says so in words, and a reader of the list can go and look.
 */

package model

import "fmt"

// LongSlur is a slur that reaches past the limit: where it opens, and how
// many bars it covers, or that it never closes at all.
type LongSlur struct {
	Part    int
	Voice   int
	Measure int // where it opens, counting from zero
	Bars    int // bars from the opening to the closing element, inclusive
	Open    bool
}

// String says where the slur is, in the numbers a reader sees.
func (l LongSlur) String() string {
	if l.Open {
		return fmt.Sprintf("staff %d bar %d, never closed", l.Part+1, l.Measure+1)
	}
	return fmt.Sprintf("staff %d bar %d, %d bars", l.Part+1, l.Measure+1, l.Bars)
}

// LongSlurs finds every slur longer than limit bars, voice by voice.
//
// Counted the way the engraver pairs them: an opening and the next closing
// in the same voice of the same part. A slur whose two ends were put in
// different voices shows up here twice over — once as an opening that never
// closes and once, if anything closes in the other voice, as whatever that
// pairs with — which is the right answer, since that is what is drawn.
func LongSlurs(s *Score, limit int) []LongSlur {
	if s == nil {
		return nil
	}
	var out []LongSlur
	for pi, p := range s.Parts {
		// One stack per voice, carried across the bars: a voice is a line
		// through the piece, not through the bar.
		type open struct{ measure int }
		var opened [][]open
		for mi, m := range p.Measures {
			for vi, v := range m.Voices {
				for len(opened) <= vi {
					opened = append(opened, nil)
				}
				for _, el := range v.Elements {
					mk := marksOfElement(el)
					if mk == nil {
						continue
					}
					for i := 0; i < mk.SlurStop; i++ {
						n := len(opened[vi])
						if n == 0 {
							break
						}
						at := opened[vi][n-1]
						opened[vi] = opened[vi][:n-1]
						if bars := mi - at.measure + 1; bars > limit {
							out = append(out, LongSlur{
								Part: pi, Voice: vi, Measure: at.measure, Bars: bars,
							})
						}
					}
					for i := 0; i < mk.SlurStart; i++ {
						opened[vi] = append(opened[vi], open{measure: mi})
					}
				}
			}
		}
		for vi := range opened {
			for _, at := range opened[vi] {
				out = append(out, LongSlur{
					Part: pi, Voice: vi, Measure: at.measure, Open: true,
					Bars: len(p.Measures) - at.measure,
				})
			}
		}
	}
	return out
}

// marksOfElement is where an element keeps its marks, or nil for one that
// carries none.
func marksOfElement(el Element) *Marks {
	switch e := el.(type) {
	case *Note:
		return &e.Marks
	case *Chord:
		return &e.Marks
	case *Rest:
		return &e.Marks
	}
	return nil
}
