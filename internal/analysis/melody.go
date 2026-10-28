/*
 * melody.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// What a line is made of, and whether anybody can sing it.
//
// A melody is a shape, and the shape is made of the intervals between one
// note and the next. A line that is nearly all steps is a tune somebody can
// hum after one hearing; a line of leaps is an accompaniment figure, a
// fanfare or an arpeggio, and knowing which you have written is worth a
// glance. Neither is better. What is worth being told is which one it is,
// because a composer writing a vocal line and getting the second is usually
// not doing it on purpose.
//
// Two things are worth more than a count. A leap of a sixth or more that
// keeps going in the same direction leaves the ear a long way from where it
// started and with nothing to hold on to — the usual advice is to turn back
// after a leap, and a line that does not is harder to sing and harder to
// remember. And an augmented second is an interval singers have complained
// about for four hundred years: it is the sound of the harmonic minor
// scale's sixth and seventh degrees, and it is why the melodic minor exists.
//
// All of it is a description, not a mark. Stravinsky's lines leap and are
// not improvable.

package analysis

import "sort"

// Step is one move from a note to the next in the same line.
type Step struct {
	Line int
	Bar  int
	// From and To are the two notes, as they sound.
	From, To int
	Ref      int
	// FromName and ToName are the two notes as they are written, which is
	// what a sentence about their spelling has to say.
	FromName, ToName string
	// Letters is how many letter names apart the two are written: 1 for a
	// second, 2 for a third. It is the one thing sounding pitch cannot
	// say. A to C and A to B sharp are the same three semitones and the
	// same sound, and one of them is a third a singer takes in their
	// stride while the other is the step they have complained about since
	// the harmonic minor was invented.
	Letters int
}

// Shape is what one line is made of.
type Shape struct {
	Line int    `json:"line"`
	Name string `json:"name"`
	// Notes is how many notes the line has and Moves how many moves
	// between them, which is one fewer unless the line is in pieces.
	Notes int `json:"notes"`
	Moves int `json:"moves"`
	// Same, Steps and Leaps are how the moves break down: a repeated
	// note, a move of a tone or less, and anything wider.
	Same  int `json:"same"`
	Steps int `json:"steps"`
	Leaps int `json:"leaps"`
	// Widest is the largest move in the line, in semitones, and WidestBar
	// where it is.
	Widest    int `json:"widest"`
	WidestBar int `json:"widestBar"`
	// Commonest is the interval the line uses most, in semitones.
	Commonest int `json:"commonest"`
	// Up and Down are how many moves go each way. A line that only climbs
	// has to get back down somehow, and usually does it all at once.
	Up   int `json:"up"`
	Down int `json:"down"`
}

// Singable is the share of moves that have to be steps or repeats before a
// line is one somebody can hum after a hearing.
//
// Two thirds. It is a rule of thumb and is offered as one: plainchant is
// nearly all steps, a horn call is nearly all leaps, and both are music.
const Singable = 0.66

// Stepwise says the line is mostly steps, which is what makes a tune
// singable rather than merely playable.
func (s Shape) Stepwise() bool {
	if s.Moves == 0 {
		return false
	}
	return float64(s.Same+s.Steps)/float64(s.Moves) >= Singable
}

// Awkward is one move worth saying something about.
type Awkward struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Line int    `json:"line"`
	Name string `json:"name"`
	Bar  int    `json:"bar"`
	// ToBar and Times bracket a run of the same move in the same line: a
	// figure repeated through a bar makes the same leap four times, and
	// four findings saying one sentence is four ways of not saying it.
	ToBar int `json:"toBar"`
	Times int `json:"times"`
	Ref   int `json:"ref"`
	// Semitones is how wide the move is, downward moves negative.
	Semitones int `json:"semitones"`
}

// BigLeap is how wide a move has to be before the ear wants it turned back.
const BigLeap = 9 // a major sixth

// Shapes reads what each line is made of.
func Shapes(steps []Step, names []string) []Shape {
	out := make([]Shape, len(names))
	counts := make([]map[int]int, len(names))
	for i := range names {
		out[i] = Shape{Line: i, Name: names[i]}
		counts[i] = map[int]int{}
	}
	notes := make([]map[int]bool, len(names))
	for i := range notes {
		notes[i] = map[int]bool{}
	}
	for _, st := range steps {
		if st.Line < 0 || st.Line >= len(out) {
			continue
		}
		s := &out[st.Line]
		s.Moves++
		notes[st.Line][st.Ref] = true
		gap := st.To - st.From
		wide := gap
		if wide < 0 {
			wide = -wide
		}
		switch {
		case gap == 0:
			s.Same++
		case wide <= 2:
			s.Steps++
		default:
			s.Leaps++
		}
		if gap > 0 {
			s.Up++
		} else if gap < 0 {
			s.Down++
		}
		if wide > s.Widest {
			s.Widest, s.WidestBar = wide, st.Bar
		}
		counts[st.Line][wide]++
	}
	for i := range out {
		out[i].Notes = out[i].Moves
		if out[i].Moves > 0 {
			out[i].Notes++
		}
		best, most := 0, 0
		var widths []int
		for w := range counts[i] {
			widths = append(widths, w)
		}
		sort.Ints(widths)
		for _, w := range widths {
			if counts[i][w] > most {
				best, most = w, counts[i][w]
			}
		}
		out[i].Commonest = best
	}
	return out
}

// Awkwardness finds the moves worth saying something about: a wide leap the
// line does not turn back from, and an augmented second.
//
// name says what a pitch is called, for the sentence.
func Awkwardness(steps []Step, names []string, name func(midi int) string) []Awkward {
	if name == nil {
		name = func(int) string { return "" }
	}
	// A step with no spelling on it falls back to naming by sound, which
	// is right for everything except telling an augmented second from the
	// minor third it sounds exactly like — and that one is decided by
	// Letters rather than by the name.
	for i := range steps {
		if steps[i].FromName == "" {
			steps[i].FromName = name(steps[i].From)
		}
		if steps[i].ToName == "" {
			steps[i].ToName = name(steps[i].To)
		}
	}
	byLine := map[int][]Step{}
	var order []int
	for _, st := range steps {
		if _, seen := byLine[st.Line]; !seen {
			order = append(order, st.Line)
		}
		byLine[st.Line] = append(byLine[st.Line], st)
	}
	sort.Ints(order)

	var out []Awkward
	for _, li := range order {
		run := byLine[li]
		who := ""
		if li < len(names) {
			who = names[li]
		}
		for i, st := range run {
			gap := st.To - st.From
			wide := gap
			if wide < 0 {
				wide = -wide
			}
			// An augmented second: three semitones written as a second.
			// Heard as a minor third and spelled as a second, which is
			// what makes it hard — the ear is asked for a step and the
			// voice has to find a third.
			if wide == 3 && st.Letters == 1 {
				out = append(out, Awkward{
					Kind: "augmented second", Line: li, Name: who,
					Bar: st.Bar, Ref: st.Ref, Semitones: gap,
					Text: "an augmented second, " + st.FromName + " to " +
						st.ToName + " — the step singers have complained " +
						"about since the harmonic minor was invented",
				})
				continue
			}
			if wide < BigLeap {
				continue
			}
			// Turned back? The next move has to go the other way. A leap
			// that is the last thing in the line has nowhere to turn and
			// is left alone.
			if i+1 >= len(run) {
				continue
			}
			after := run[i+1].To - run[i+1].From
			if after == 0 || (after > 0) != (gap > 0) {
				continue
			}
			which := "up"
			if gap < 0 {
				which = "down"
			}
			out = append(out, Awkward{
				Kind: "leap not turned", Line: li, Name: who,
				Bar: st.Bar, Ref: st.Ref, Semitones: gap,
				Text: "a leap of " + widthName(wide) + " " + which + ", " +
					st.FromName + " to " + st.ToName +
					", and the line keeps going the same way",
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Bar < out[j].Bar })
	return awkwardRuns(out)
}

// awkwardRuns folds a run of the same move in the same line into one
// finding, the way every other run of findings is folded.
func awkwardRuns(found []Awkward) []Awkward {
	var out []Awkward
	for _, a := range found {
		n := len(out)
		if n > 0 && out[n-1].Line == a.Line && out[n-1].Kind == a.Kind &&
			out[n-1].Semitones == a.Semitones && a.Bar-out[n-1].ToBar <= 1 {
			out[n-1].ToBar = a.Bar
			out[n-1].Times++
			continue
		}
		a.ToBar, a.Times = a.Bar, 1
		out = append(out, a)
	}
	return out
}

func widthName(semitones int) string {
	names := map[int]string{
		9: "a major sixth", 10: "a minor seventh", 11: "a major seventh",
		12: "an octave",
	}
	if n, ok := names[semitones]; ok {
		return n
	}
	if semitones > 12 {
		return "more than an octave"
	}
	return "a sixth"
}
