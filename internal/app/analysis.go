/*
 * analysis.go
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
	"sort"
	"strconv"

	"github.com/stricaud/gscore/internal/analysis"
	"github.com/stricaud/gscore/internal/model"
)

// handleHarmonics says what the harmony is made of at one moment.
//
// The moment is given in seconds along the performance, which is what the
// playback cursor already works in — so the panel can follow the music as it
// plays, or sit on the bar the cursor is in while it is being written.
//
// Everything is worked out from the written notes. A spectrum taken off the
// audio would show the soundfont: its samples, its filters, its reverb, and
// a different answer for the same chord on another instrument. What is asked
// here is a question about the notes.
func (a *App) handleHarmonics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	at, err := strconv.ParseFloat(q.Get("at"), 64)
	if err != nil {
		at = -1
	}

	opt := analysis.DefaultOptions()
	if v, err := strconv.Atoi(q.Get("partials")); err == nil && v > 0 && v <= 32 {
		opt.Partials = v
	}
	if v, err := strconv.ParseFloat(q.Get("rolloff"), 64); err == nil && v > 0 {
		opt.Rolloff = v
	}

	a.mu.RLock()
	pb := buildPlayback(a.score)
	names := make([]string, len(a.score.Parts))
	for i, p := range a.score.Parts {
		names[i] = p.Name
		if names[i] == "" {
			names[i] = p.Instrument.String()
		}
	}
	a.mu.RUnlock()

	// Where to look, when the question was asked about a note rather than
	// about a moment.
	//
	// The editor knows which notehead is picked up and nothing about the
	// clock; the timeline is here. So a note can be named instead of a
	// time, and "what is sounding when this note sounds" — which is the
	// question somebody writing a chord is actually asking — needs nothing
	// in the browser to work it out.
	if v, err := strconv.Atoi(q.Get("ref")); err == nil && v > 0 {
		for _, e := range pb.Events {
			if e.Ref == v {
				at = e.Start
				break
			}
		}
	} else if v, err := strconv.Atoi(q.Get("measure")); err == nil && v >= 0 {
		for _, m := range pb.Measures {
			if m.Index == v {
				at = m.Start
				break
			}
		}
	}
	if at < 0 {
		at = 0
	}

	// Everything sounding at that moment. A note counts from the instant it
	// starts until it stops: a chord is what is held together, which is what
	// the question is about, and a note that has just been released is no
	// longer part of it.
	var sounding []analysis.Note
	for _, e := range pb.Events {
		if e.Drum > 0 {
			continue // a drum has no pitch to put a series on
		}
		if at < e.Start || at >= e.Start+e.Dur {
			continue
		}
		sounding = append(sounding, analysis.Note{
			MIDI:     e.MIDI,
			Name:     pitchName(e.MIDI),
			Part:     e.Part,
			Velocity: e.Velocity,
			Ref:      e.Ref,
		})
	}

	out := analysis.Of(sounding, opt)
	out.At = at

	// What the chord is called, and what it is doing in the key the music
	// is in where it falls.
	//
	// The name is a fact about the notes; the numeral is a fact about the
	// notes and the key together, and it is the one that says what the
	// chord is *for*. Without it the panel could tell somebody that C♯ E
	// G♯ is rough to two decimal places and not that it is the tonic.
	midis := make([]int, 0, len(sounding))
	for _, n := range sounding {
		midis = append(midis, n.MIDI)
	}
	chord := analysis.NameChord(midis)
	// And whether it will speak down there. The roughness number below
	// says how rough the chord is; this says the orchestrator's version of
	// the same thing about the bottom of it, which is where a chord turns
	// to mud first.
	var thick map[string]any
	if len(midis) > 1 {
		sorted := append([]int(nil), midis...)
		sort.Ints(sorted)
		gap := sorted[1] - sorted[0]
		if limit, ok := analysis.LowLimit(gap); ok &&
			limit-sorted[0] >= analysis.LeastBelow {
			thick = map[string]any{
				"low": sorted[0], "high": sorted[1], "semitones": gap,
				"limit": limit, "below": limit - sorted[0],
				"text": "the bottom of this chord is spaced below where it " +
					"usually speaks clearly — " + pitchName(sorted[0]) +
					" under " + pitchName(limit) + " for that interval",
			}
		}
	}
	a.mu.RLock()
	bar := barAt(pb, at)
	key, fit := keyAround(a.score, bar)
	// Spelled the way the music around it is spelled: the dominant of a
	// piece in four sharps is G♯ major, and calling it A♭ major is the
	// same twelve notes written in a language the page is not in.
	fifths := 0
	for _, p := range a.score.Parts {
		if !p.Hidden && !p.Unpitched() {
			fifths = p.KeyAt(bar).Fifths
			break
		}
	}
	a.mu.RUnlock()

	writeJSON(w, map[string]any{
		"chord":      chord,
		"thick":      thick,
		"key":        key,
		"keyName":    key.Name(fifths),
		"keyFit":     fit,
		"numeral":    analysis.Numeral(chord, key),
		"function":   analysis.Function(chord, key),
		"at":         out.At,
		"duration":   pb.Duration,
		"notes":      out.Notes,
		"partials":   out.Partials,
		"coincident": out.Coincident,
		"intervals":  out.Intervals,
		"roughness":  out.Roughness,
		"verdict":    out.Verdict,
		"staves":     names,
	})
}

// pitchName writes a MIDI note the way a musician says it.
//
// Sharps throughout, because a sounding pitch has no key signature to say
// whether it is an F sharp or a G flat: this is what is heard, not what is
// written, and the two part company on every transposing instrument.
func pitchName(midi int) string {
	names := [...]string{"C", "C♯", "D", "D♯", "E", "F", "F♯", "G", "G♯", "A", "A♯", "B"}
	if midi < 0 || midi > 127 {
		return ""
	}
	return names[midi%12] + strconv.Itoa(midi/12-1)
}

// handleReach says how far each staff reaches, and whether the instrument it
// is written for can go there.
//
// The one check a composer cannot do for themselves. Everything else about a
// part can be heard by reading it; whether a horn can play a passage depends
// on the horn and not on the music, and somebody who does not play one has
// no way of finding out except by asking a player.
func (a *App) handleReach(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	notes := soundedNotes(a.score)
	insts := make([]analysis.Instrument, len(a.score.Parts))
	for i, p := range a.score.Parts {
		in := analysis.Instrument{
			Staff: p.Name, Name: p.SoundName(),
			// A staff of drums is read by position on a kit, not by
			// pitch, so there is no range to ask about. Asked anyway,
			// the bass drum of Beethoven's ninth was taken for a string
			// instrument written six hundred bars below its range.
			Percussion: p.Unpitched(),
		}
		if rr, ok := p.Reach(); ok && !in.Percussion {
			in.Known = true
			in.Low, in.High = rr.Low, rr.High
			in.EasyLow, in.EasyHigh = rr.EasyLow, rr.EasyHigh
		}
		insts[i] = in
	}
	a.mu.RUnlock()

	writeJSON(w, map[string]any{
		"parts": analysis.Reaches(notes, insts, pitchName),
	})
}

// soundedNotes is every notehead in the score, as it sounds.
//
// Sounding rather than written, because a range is a physical fact about an
// instrument and does not move because the part is spelled in another key: a
// clarinet's bottom note is the same note either way. So the part's
// transposition is applied, and so is any octave line over it.
//
// The numbering is the engraver's, by way of baseRefs, so a finding can
// point at the notehead it is about and the editor can light it up. Taken
// from the performance instead it would have counted a repeated bar twice
// and a tremolo thirty times.
func soundedNotes(s *model.Score) []analysis.Sounded {
	base := baseRefs(s)
	var out []analysis.Sounded
	for pi, p := range s.Parts {
		// An octave line shifts everything under it until it closes, the
		// same way playback reads it. A bracket over notes that are at
		// pitch already says where the music is, not where to move it.
		shift := 0
		for mi := range p.Measures {
			for k, v := range p.Measures[mi].Ordered() {
				if mk := marksOf(v.El); mk != nil && mk.Octave != nil {
					if mk.Octave.Start {
						shift = mk.Octave.Amount
						if mk.Octave.SoundsAsWritten {
							shift = 0
						}
					}
				}
				ref := base[pi][mi] + k
				d := v.El.Duration()
				held := 0.0
				if d.Den != 0 {
					held = float64(d.Num) / float64(d.Den)
				}
				add := func(pitch model.Pitch) {
					out = append(out, analysis.Sounded{
						Part: pi, MIDI: sounding(p, pitch, shift).MIDI(),
						Bar: mi, Ref: ref, Held: held,
					})
				}
				switch el := v.El.(type) {
				case *model.Note:
					add(el.Pitch)
				case *model.Chord:
					for _, n := range el.Notes {
						add(n.Pitch)
					}
				}
				if mk := marksOf(v.El); mk != nil && mk.Octave != nil && mk.Octave.Stop {
					shift = 0
				}
			}
		}
	}
	return out
}

// barAt is which bar of the score a moment falls in.
func barAt(pb Playback, at float64) int {
	for _, m := range pb.Measures {
		if at >= m.Start && at < m.Start+m.Dur {
			return m.Index
		}
	}
	if n := len(pb.Measures); n > 0 {
		return pb.Measures[n-1].Index
	}
	return 0
}

// keyAround is the key the music is in at a bar, read from the phrase around
// it rather than from the key signature.
//
// A signature is written once and stands until somebody writes another one;
// the music moves through keys inside it all the time, and a numeral worked
// out from the signature would call the whole development of a sonata by the
// numerals of its exposition.
func keyAround(s *model.Score, bar int) (analysis.Key, analysis.Fit) {
	lo := bar - analysis.KeyWindow/2
	if lo < 0 {
		lo = 0
	}
	hi := lo + analysis.KeyWindow
	var w analysis.Weights
	for _, p := range s.Parts {
		if p.Hidden || p.Unpitched() {
			continue
		}
		for mi := lo; mi < hi && mi < len(p.Measures); mi++ {
			for _, v := range p.Measures[mi].Voices {
				for _, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					for _, pitch := range pitchesIn(el) {
						pc := sounding(p, pitch, 0).MIDI() % 12
						w[((pc%12)+12)%12] += held
					}
				}
			}
		}
	}
	return analysis.KeyOf(w)
}

// pitchesIn is the written pitches an element carries, and none for a rest.
func pitchesIn(el model.Element) []model.Pitch {
	switch e := el.(type) {
	case *model.Note:
		return []model.Pitch{e.Pitch}
	case *model.Chord:
		out := make([]model.Pitch, 0, len(e.Notes))
		for _, n := range e.Notes {
			out = append(out, n.Pitch)
		}
		return out
	}
	return nil
}

// handleKeys says what key the music is in, bar by bar, gathered into the
// stretches where it stays the same.
//
// Not what the key signature says. A signature is written once and stands
// until somebody writes another one; the music moves through keys inside it
// all the time, and where it moves is most of what there is to understand
// about a piece. The development of a sonata is the signature of the
// exposition and almost none of its keys.
func (a *App) handleKeys(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	bars := barWeights(a.score)
	fifths := make([]int, len(bars))
	for i := range bars {
		for _, p := range a.score.Parts {
			if !p.Hidden && !p.Unpitched() {
				fifths[i] = p.KeyAt(i).Fifths
				break
			}
		}
	}
	a.mu.RUnlock()

	spans := analysis.Keys(bars)
	out := make([]map[string]any, 0, len(spans))
	for _, s := range spans {
		at := 0
		if s.FromBar < len(fifths) {
			at = fifths[s.FromBar]
		}
		out = append(out, map[string]any{
			"fromBar": s.FromBar, "toBar": s.ToBar,
			"key": s.Key, "name": s.Key.Name(at), "fit": s.Fit,
			"clear": s.Fit.Clear(),
		})
	}
	// The movements are not sent: the editor already has them from
	// /api/info, and that is where the bar numbers printed on the page are
	// worked out from.
	writeJSON(w, map[string]any{"bars": len(bars), "keys": out})
}

// barWeights is how long each of the twelve pitch classes sounds in each
// bar, which is what a key is read from.
//
// How long and not how many: a semibreve on the tonic says more about the
// key than six passing sixteenths, and counting notes says the opposite.
// Every pitched staff counts, because the key is the piece's and not any one
// staff's — a bass line alone is in a dozen keys at once.
func barWeights(s *model.Score) []analysis.Weights {
	bars := 0
	for _, p := range s.Parts {
		if len(p.Measures) > bars {
			bars = len(p.Measures)
		}
	}
	out := make([]analysis.Weights, bars)
	for _, p := range s.Parts {
		if p.Hidden || p.Unpitched() {
			continue
		}
		for mi, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					for _, pitch := range pitchesIn(el) {
						pc := ((sounding(p, pitch, 0).MIDI() % 12) + 12) % 12
						out[mi][pc] += held
					}
				}
			}
		}
	}
	return out
}

// handleMap draws the shape of a piece: who is playing when, and what
// repeats what.
//
// Both are questions about the whole of it, which gscore could answer
// nothing of. A symphony is fifty-three staves and two and a half thousand
// bars and there was no way to see any of it at once — not which
// instruments carry a passage, not where the material comes back, not where
// the piece is dense and where it thins out.
func (a *App) handleMap(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	busy, shapes := scoreShape(a.score)
	a.mu.RUnlock()

	least := analysis.LeastEcho
	if v, err := strconv.Atoi(r.URL.Query().Get("least")); err == nil && v >= 1 && v <= 64 {
		least = v
	}
	echoes := analysis.Echoes(shapes, least)
	if echoes == nil {
		echoes = []analysis.Echo{}
	}
	writeJSON(w, map[string]any{
		"bars":    len(shapes),
		"staves":  busy,
		"busiest": analysis.Busiest(busy),
		"echoes":  echoes,
	})
}

// scoreShape reads the whole score once: how much each staff plays in each
// bar, and what each bar holds in a form two bars can be compared in.
//
// Once, because a symphony is a large walk and there is no reason to take it
// twice. The two answers come off the same pass.
func scoreShape(s *model.Score) ([]analysis.Busy, []analysis.BarShape) {
	bars := 0
	for _, p := range s.Parts {
		if len(p.Measures) > bars {
			bars = len(p.Measures)
		}
	}
	busy := make([]analysis.Busy, 0, len(s.Parts))
	inBar := make([][]analysis.BarNote, bars)
	for pi, p := range s.Parts {
		b := analysis.Busy{Part: pi, Name: p.Name, Bars: make([]int, bars)}
		if b.Name == "" {
			b.Name = p.SoundName()
		}
		for mi, m := range p.Measures {
			at := 0.0
			for _, v := range m.Voices {
				at = 0
				for _, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					for _, pitch := range pitchesIn(el) {
						b.Bars[mi]++
						b.Notes++
						// A staff of drums is counted as playing — it is
						// — but its pitches are places on a kit and would
						// make two bars of the same rhythm look like two
						// different bars.
						if !p.Unpitched() {
							inBar[mi] = append(inBar[mi], analysis.BarNote{
								Part: pi, MIDI: sounding(p, pitch, 0).MIDI(),
								At: at, Held: held,
							})
						}
					}
					at += held
				}
			}
			if b.Bars[mi] > 0 {
				b.Playing++
			}
		}
		busy = append(busy, b)
	}
	shapes := make([]analysis.BarShape, bars)
	for i := range inBar {
		shapes[i] = analysis.Shaped(inBar[i])
	}
	return busy, shapes
}

// handleVoices says how the lines of the score move against each other.
//
// The one part of the analysis that judges rather than measures. Parallel
// fifths are not wrong; they are wrong in a tradition, and they are the
// whole sound of organum, of Debussy and of every power chord ever played.
// So nothing here is sent as an error: each finding says what was seen,
// where, and in which pair of lines, and the composer decides whether it is
// what they meant.
func (a *App) handleVoices(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lines, sounds := scoreLines(a.score)
	a.mu.RUnlock()

	// A score of many staves is many pairs of lines, and most of the pairs
	// are instruments that never hear each other. Reported anyway it would
	// be thousands of findings about a symphony, which is a way of saying
	// nothing at great length.
	if len(lines) > maxVoiceLines {
		writeJSON(w, map[string]any{
			"lines": len(lines), "tooMany": maxVoiceLines, "motions": []any{},
		})
		return
	}
	motions := analysis.Moving(lines, sounds)
	if motions == nil {
		motions = []analysis.Motion{}
	}
	names := make([]string, len(lines))
	for i, l := range lines {
		names[i] = l.Name
	}
	writeJSON(w, map[string]any{
		"lines": len(lines), "names": names, "motions": motions,
	})
}

// maxVoiceLines is how many lines a score may have before this is not worth
// asking.
//
// Sixteen is a chorale, a quartet, a piano piece in several voices — the
// music the rules it checks were written for and about. A symphony has
// sixty, and the fifths between the second clarinet and a cello eight bars
// apart in register are not a fact anybody can use.
const maxVoiceLines = 16

// lineShare is how much of its voice's music a line has to be present for
// before it is a line at all.
//
// A chorale's alto is the lower note of the treble staff's chords in every
// bar; the fifth note of one thick piano chord is not a line, it is a
// thickening. Counting it as one gave the Moonlight thirteen lines, most of
// them a handful of bars long, and put the whole piece over the limit.
const lineShare = 0.5

// scoreLines reads the score as lines of music: a voice of a staff, and
// where several notes are written in one voice, the nth down from the top
// of them.
//
// A chorale written on two staves has four lines in two voices — the soprano
// and alto are the top and bottom notes of the treble staff's chords. That
// is how such music is written and so that is how it is read.
func scoreLines(s *model.Score) ([]analysis.Line, []analysis.Sound) {
	type key struct{ part, voice, rank int }
	at := map[key]int{}
	var lines []analysis.Line
	var sounds []analysis.Sound

	base := baseRefs(s)
	// Where every bar begins, worked out once. Asked bar by bar it walked
	// the whole score from the start each time, and reading the Moonlight
	// took a quarter of a second for nothing.
	starts := barStarts(s)
	for pi, p := range s.Parts {
		if p.Hidden || p.Unpitched() {
			continue
		}
		// Named the way the editor names staves everywhere else, so a
		// finding about one can be found on the page. The instrument is
		// not the name: a sonata's two staves are both "piano".
		staff := p.Name
		if staff == "" {
			staff = "Staff " + strconv.Itoa(pi+1)
		}
		for mi, m := range p.Measures {
			for vi, v := range m.Voices {
				start := 0.0
				for ei, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					pitches := pitchesIn(el)
					// Highest first, so rank 0 is the top line of the
					// chord and stays the top line as the chord changes
					// size.
					sort.Slice(pitches, func(x, y int) bool {
						return sounding(p, pitches[x], 0).MIDI() >
							sounding(p, pitches[y], 0).MIDI()
					})
					for rank, pitch := range pitches {
						k := key{pi, vi, rank}
						idx, seen := at[k]
						if !seen {
							idx = len(lines)
							at[k] = idx
							lines = append(lines, analysis.Line{
								Part: pi, Voice: vi, Rank: rank,
								Name: lineName(staff, vi, rank),
							})
						}
						heard := sounding(p, pitch, 0)
						sounds = append(sounds, analysis.Sound{
							Line: idx, Bar: mi,
							At: starts[mi] + start, Until: starts[mi] + start + held,
							MIDI:   heard.MIDI(),
							Name:   heard.String(),
							Letter: heard.DiatonicIndex(),
							Ref:    base[pi][mi] + ei,
						})
					}
					start += held
				}
			}
		}
	}
	return thin(lines, sounds)
}

// thin drops the ranks that are not lines: those sounding in too little of
// what their voice plays to be a line of music rather than a thickening of
// one.
func thin(lines []analysis.Line, sounds []analysis.Sound) ([]analysis.Line, []analysis.Sound) {
	// How many bars each line sounds in, and how many its voice does.
	bars := make([]map[int]bool, len(lines))
	for i := range bars {
		bars[i] = map[int]bool{}
	}
	for _, s := range sounds {
		bars[s.Line][s.Bar] = true
	}
	top := map[[2]int]int{} // part and voice to its first rank's bars
	for i, l := range lines {
		if l.Rank == 0 {
			top[[2]int{l.Part, l.Voice}] = len(bars[i])
		}
	}

	keep := make([]int, len(lines))
	var out []analysis.Line
	for i, l := range lines {
		whole := top[[2]int{l.Part, l.Voice}]
		if l.Rank > 0 && whole > 0 &&
			float64(len(bars[i]))/float64(whole) < lineShare {
			keep[i] = -1
			continue
		}
		keep[i] = len(out)
		out = append(out, l)
	}
	var kept []analysis.Sound
	for _, s := range sounds {
		if keep[s.Line] < 0 {
			continue
		}
		s.Line = keep[s.Line]
		kept = append(kept, s)
	}
	return out, kept
}

func lineName(staff string, voice, rank int) string {
	name := staff
	if voice > 0 {
		name += ", voice " + strconv.Itoa(voice+1)
	}
	if rank > 0 {
		name += ", note " + strconv.Itoa(rank+1) + " down"
	}
	return name
}

// barStarts is where each bar begins along the piece, in whole notes.
//
// Counted from the written lengths rather than from the performance: a
// repeated bar sounds twice and is written once, and a finding about it
// belongs to the bar on the page.
func barStarts(s *model.Score) []float64 {
	n := s.MeasureCount()
	out := make([]float64, n+1)
	at := 0.0
	for i := 0; i < n; i++ {
		out[i] = at
		if d := s.MeasureDuration(i); d.Den != 0 {
			at += float64(d.Num) / float64(d.Den)
		}
	}
	out[n] = at
	return out
}

// handleSpacing says where the bottom of the harmony is written too low for
// the interval it is spaced at.
//
// The roughness of the harmonics panel pointed outward. That panel answers
// "is this chord rough" with a number; an orchestrator is asking the same
// question the other way round — "will this spacing speak down there" — and
// wants it on a bar rather than in a dialog.
func (a *App) handleSpacing(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	_, sounds := scoreLines(a.score)
	a.mu.RUnlock()

	thick := analysis.Spacing(sounds, pitchName)
	if thick == nil {
		thick = []analysis.Thick{}
	}
	writeJSON(w, map[string]any{"thick": thick})
}

// articulationLimits is the shortest note each family can start and stop
// cleanly, in seconds.
//
// Conventional and generous: a string player's bow and a pianist's finger
// can go faster than a tongue can, and a tuba's column of air is the slowest
// thing in the room. These are the point past which a passage stops being
// fast and starts being a smear, not the point past which it is difficult.
var articulationLimits = map[model.Family]float64{
	model.FamilyPiano:     0.045,
	model.FamilyPlucked:   0.050,
	model.FamilyTuned:     0.050,
	model.FamilyBowed:     0.055,
	model.FamilyFlute:     0.065,
	model.FamilyReed:      0.075,
	model.FamilyBrass:     0.095,
	model.FamilySustained: 0.110,
}

// handleRhythm says where the notes fall against the beat, and where the
// bars or the tempo do not add up.
//
// A metre is a pattern of strong and weak, and almost everything a rhythm
// does it does by agreeing with that pattern or pulling against it. Which is
// happening is a description and not a verdict: syncopation is not a fault,
// it is most of what makes music move.
func (a *App) handleRhythm(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	beats, names, odd := scoreBeats(a.score)
	bars := a.score.MeasureCount()
	tempos := a.score.Tempos()
	limits := make([]float64, len(a.score.Parts))
	for i, p := range a.score.Parts {
		if p.Unpitched() {
			limits[i] = 0
			continue
		}
		limits[i] = articulationLimits[p.Instrument.Family()]
	}
	a.mu.RUnlock()

	seconds := func(b analysis.Beat) float64 {
		return b.Held * wholeNoteSeconds(tempos, b.Bar)
	}
	limit := func(part int) (float64, bool) {
		if part < 0 || part >= len(limits) || limits[part] <= 0 {
			return 0, false
		}
		return limits[part], true
	}
	rushed := analysis.Rushed(beats, names, seconds, limit, pitchName)
	if rushed == nil {
		rushed = []analysis.Hurried{}
	}
	if odd == nil {
		odd = []analysis.Odd{}
	}
	writeJSON(w, map[string]any{
		"bars":   bars,
		"pulses": analysis.Pulses(beats, names, bars),
		"odd":    odd,
		"rushed": rushed,
	})
}

// scoreBeats reads where every note starts in its bar, and which bars hold
// more or less than their metre says.
func scoreBeats(s *model.Score) ([]analysis.Beat, []string, []analysis.Odd) {
	base := baseRefs(s)
	names := make([]string, len(s.Parts))
	var beats []analysis.Beat
	var odd []analysis.Odd
	seen := map[int]bool{}

	for pi, p := range s.Parts {
		names[pi] = p.Name
		if names[pi] == "" {
			names[pi] = "Staff " + strconv.Itoa(pi+1)
		}
		if p.Hidden {
			continue
		}
		for mi, m := range p.Measures {
			time := p.TimeAt(mi)
			unit, says := 0.0, 0.0
			if time.BeatValue > 0 {
				unit = 1 / float64(time.BeatValue)
				says = float64(time.Beats) * unit
			}
			longest, tremolo := 0.0, false
			for _, v := range m.Voices {
				at := 0.0
				for ei, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					if mk := marksOfElement(el); mk != nil && mk.Tremolo.Paired() {
						tremolo = true
					}
					if len(pitchesIn(el)) > 0 {
						beats = append(beats, analysis.Beat{
							Bar: mi, At: at, Held: held,
							Beats: time.Beats, Unit: unit, Part: pi,
							Ref: base[pi][mi] + ei,
						})
					}
					at += held
				}
				if at > longest {
					longest = at
				}
			}
			// A bar that does not add up is usually an import that went
			// wrong or an edit that did. Reported once for the score
			// rather than once per staff, because it is one bar.
			//
			// A bar holding a two-note tremolo is left out: gscore counts
			// the pair as the sum of its two written notes and MuseScore
			// counts it as one of them, and until that is settled a bar
			// of them is not evidence of anything.
			if says > 0 && !tremolo && !seen[mi] && !near(longest, says) &&
				!(mi == 0 && longest < says) {
				seen[mi] = true
				odd = append(odd, analysis.Odd{
					Bar: mi, Held: longest, Says: says,
					Text: barHolds(longest, says, time),
				})
			}
		}
	}
	return beats, names, odd
}

func near(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

func barHolds(held, says float64, time model.TimeSignature) string {
	what := "more"
	if held < says {
		what = "less"
	}
	return "this bar holds " + what + " than " +
		strconv.Itoa(time.Beats) + "/" + strconv.Itoa(time.BeatValue) + " says"
}

// handleMelody says what each line is made of, and which of its moves are
// worth a second look.
func (a *App) handleMelody(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lines, steps := scoreSteps(a.score)
	a.mu.RUnlock()

	names := make([]string, len(lines))
	for i, l := range lines {
		names[i] = l.Name
	}
	awkward := analysis.Awkwardness(steps, names, pitchName)
	if awkward == nil {
		awkward = []analysis.Awkward{}
	}
	writeJSON(w, map[string]any{
		"shapes":  analysis.Shapes(steps, names),
		"awkward": awkward,
	})
}

// scoreSteps reads every move from one note to the next in each line.
func scoreSteps(s *model.Score) ([]analysis.Line, []analysis.Step) {
	lines, sounds := scoreLines(s)
	byLine := map[int][]analysis.Sound{}
	for _, snd := range sounds {
		byLine[snd.Line] = append(byLine[snd.Line], snd)
	}
	var steps []analysis.Step
	for li := range lines {
		run := byLine[li]
		sort.Slice(run, func(i, j int) bool { return run[i].At < run[j].At })
		for i := 0; i+1 < len(run); i++ {
			from, to := run[i], run[i+1]
			steps = append(steps, analysis.Step{
				Line: li, Bar: to.Bar, From: from.MIDI, To: to.MIDI,
				Ref: to.Ref, Letters: lettersApart(from, to),
				FromName: from.Name, ToName: to.Name,
			})
		}
	}
	return lines, steps
}

// lettersApart is how many letter names two notes are written apart, which
// is the one thing their sounding pitches cannot say.
//
// Read off the spelling gscore keeps on every note rather than guessed at
// from the semitones: A to C and A to B sharp are the same sound, and only
// the letters tell a minor third from an augmented second.
func lettersApart(from, to analysis.Sound) int {
	n := to.Letter - from.Letter
	if n < 0 {
		n = -n
	}
	return n
}
