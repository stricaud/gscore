/*
 * playback.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"io/fs"
	"net/http"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/midi"
	"github.com/stricaud/gscore/internal/model"
	ghtml "github.com/stricaud/gscore/internal/render/html"
	"github.com/stricaud/gscore/internal/smufl"
)

// Event is one sounding note, with its timing already resolved to seconds.
//
// The score is flattened here rather than in the browser so that tempo, ties
// and voices are interpreted once, by the same code that engraves them. The
// browser only has to make a noise at the right moment.
type Event struct {
	MIDI  int     `json:"midi"`
	Start float64 `json:"start"`
	Dur   float64 `json:"dur"`
	// Velocity is how loud, 0 to 127, from the dynamic in force.
	Velocity int `json:"velocity"`
	Part     int `json:"part"`
	// Program is the staff's General MIDI instrument, and Family is how it is
	// sounded. The browser needs the second to pick a voice; the first is
	// there so it can be more specific later.
	Program int    `json:"program"`
	Family  string `json:"family"`
	// Drum is the General MIDI percussion key when this event is a drum
	// rather than a pitch, and 0 otherwise. A drum staff's positions name
	// instruments, so what sounds cannot be worked out from the pitch.
	Drum int `json:"drum,omitempty"`
	// DrumName is what that instrument is called, for anything that wants to
	// say what is playing.
	DrumName string `json:"drumName,omitempty"`
	// Ref is the element this note came from, numbered exactly as the
	// engraver numbers its glyphs. That shared numbering is what lets the
	// editor light up the notehead that is sounding.
	Ref int `json:"ref"`

	// Gate is how much of Dur the note actually sounds, as a fraction.
	//
	// Dur is how long the note *occupies*: the written length, which the
	// cursor and the highlight follow. How long it is *heard* is a
	// different question, and the answer is what tells a staccato quaver
	// from an ordinary one — a staccato note still takes its whole beat, it
	// is simply short, and the silence after it is the point.
	//
	// An ordinary note stops a little before the next begins, the way a
	// player tongues or bows one. A slurred note does not stop at all: it
	// runs a hair into the one after it, which is what a synthesiser and a
	// reader both take for a phrase rather than a row of separate notes.
	Gate float64 `json:"gate,omitempty"`
}

// How long a note is held, as a fraction of its written length.
//
// These are the figures a sequencer uses and a player would recognise.
// Ordinary notes are detached a little: run end to end they sound like one
// held note with the pitch changing, which is the one thing nobody plays.
const (
	gatePlain         = 0.88
	gateStaccato      = 0.50
	gateStaccatissimo = 0.35
	gateTenuto        = 1.00
	// Slurred notes overlap, so that what follows is heard as the same
	// breath or bow rather than as the next note.
	gateSlurred = 1.02
)

// maxDetach is the longest silence left before the next note, in seconds.
//
// The gap between ordinary notes is a breath or the turn of a bow, and
// those take about as long whatever is being played. Taken as a fraction
// the gap grows with the note, so a tied semibreve at a slow tempo would be
// let go a quarter of a second early and the music would start to limp. A
// staccato mark is the other thing and stays a fraction: a staccato minim
// really is short.
const maxDetach = 0.06

// How much louder an accent is, as a factor on the velocity in force.
const (
	accentLouder  = 1.2
	marcatoLouder = 1.3
)

// gateOf is how much of its length a note is held for, from what is written
// on it and whether it is inside a slur.
//
// A slur wins over the plain gate and loses to a staccato dot: a dot under a
// slur is a portato, which is played detached inside the phrase, and that is
// what the two marks together mean.
func gateOf(m *model.Marks, slurred bool) float64 {
	if m != nil {
		for _, a := range m.Articulations {
			switch a {
			case model.Staccato:
				return gateStaccato
			case model.Staccatissimo:
				return gateStaccatissimo
			case model.Tenuto:
				return gateTenuto
			}
		}
	}
	if slurred {
		return gateSlurred
	}
	return gatePlain
}

// breath caps the gap an ordinary note leaves before the next one.
//
// Only the ordinary gap: a staccato note is meant to be a fraction of its
// length and a slurred one is meant to overlap, and neither is a breath.
func breath(gate, dur float64) float64 {
	if gate != gatePlain || dur <= 0 {
		return gate
	}
	if gap := dur * (1 - gate); gap > maxDetach {
		return 1 - maxDetach/dur
	}
	return gate
}

// velocityOf is how hard the note is struck: the dynamic in force, and the
// accents written on this note alone.
func velocityOf(m *model.Marks, loud model.Dynamic) int {
	v := float64(loud.Velocity())
	if m != nil {
		for _, a := range m.Articulations {
			switch a {
			case model.Accent:
				v *= accentLouder
			case model.Marcato, model.MarcatoDown:
				v *= marcatoLouder
			}
		}
	}
	if v > 127 {
		v = 127
	}
	if v < 1 {
		v = 1
	}
	return int(v + 0.5)
}

// MeasureTime is when a measure begins and how long it lasts.
//
// The playback cursor follows this rather than the notes. A score has one
// timeline but many staves, and each has its notes in different places, so a
// cursor built from note positions lurches between parts. Time is the thing
// every staff agrees on.
type MeasureTime struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	Dur   float64 `json:"dur"`
}

// Playback is a whole score's worth of events.
type Playback struct {
	Events   []Event       `json:"events"`
	Measures []MeasureTime `json:"measures"`
	Duration float64       `json:"duration"`
	Tempo    int           `json:"tempo"`
}

// buildPlayback walks the score and resolves every note to a time in seconds.
//
// gscore 0.0.9 declared score_midi_play() in src/include/score.h and a
// midi_instrument field on every staff, but never implemented either; this is
// that feature, twenty years later and without a MIDI device in the way.
func buildPlayback(s *model.Score) Playback {
	bpm := s.Tempo.BPM
	if bpm <= 0 {
		bpm = 120
	}

	out := Playback{Tempo: bpm}
	// The order the bars are actually played in: a repeat sends you back, a
	// first-time bar is skipped the second time through. Both the notes and
	// the cursor walk this, so the line down the page goes round when the
	// music does.
	order := s.PlayOrder()

	// Which number the engraver gave the first element of each bar of each
	// staff. A bar played twice sounds the same notes both times, so the
	// numbering is worked out once from the written score and reused —
	// counting along as we play would give a bar's notes new numbers on the
	// second time through and the highlight would stop finding them.
	refBase := baseRefs(s)

	// When each bar begins, worked out before any note is placed.
	//
	// There is one timeline in a score and every part is on it. Letting each
	// part keep its own clock — advancing it by whatever that part happens to
	// have in the bar — means a part with a short bar falls behind the
	// barline and stays behind: in Mozart's Offertorium the soloists' clock
	// ran forty-nine seconds behind the measure timeline by the last bar, so
	// the cursor and the note it was supposed to be pointing at were most of
	// a page apart. Deciding the bar lines first and hanging every part off
	// them makes that unrepresentable.
	starts, total := measureTimeline(s, order)
	for k, start := range starts {
		d := total - start
		if k+1 < len(starts) {
			d = starts[k+1] - start
		}
		out.Measures = append(out.Measures, MeasureTime{
			Index: order[k], Start: start, Dur: d,
		})
	}
	out.Duration = total

	for pi, p := range s.Parts {
		// A tie means the two notes sound as one, so a tied-to note extends
		// the event already running rather than starting another.
		open := map[int]int{} // midi -> index in out.Events
		// A dynamic holds until another replaces it.
		loud := model.MF
		// An octave line shifts everything under it; it opens at one note and
		// closes at another, possibly bars later.
		shift := 0
		// A trill is held across the notes it covers, like an octave line.
		trilling := false
		// So is a slur. Inside one the notes are not re-articulated: each
		// runs into the next, which is what a player does with a bow or a
		// breath and what the mark is asking for.
		slurring := false
		// A hairpin is a change of volume over a stretch of music, so what it
		// does cannot be known at the note it starts on. The notes it covers
		// are collected and scaled once it closes.
		var swell *swelling
		for step, mi := range order {
			if mi >= len(p.Measures) {
				continue // this staff is shorter than the score
			}
			m := p.Measures[mi]
			// Voices within a measure sound together, so each starts at the
			// measure's time and the measure lasts as long as the longest.
			if len(m.Voices) == 0 {
				continue
			}
			// Only the first voice, matching the engraver, so that refs line
			// up. ABC import puts everything in one voice per measure.
			v := m.Voices[0]

			// A bar marked as a repeat sounds as the bar it stands for. The
			// sign is an instruction to the player, so it has to be one to
			// the playback too — a chart with eight repeat signs in it is
			// otherwise eight bars of silence.
			if m.Repeat > 0 {
				if src := p.RepeatSource(mi, m.Repeat); src != nil {
					v = src
				}
			}
			// Every part starts this bar where the bar starts, not where its
			// own last bar happened to end.
			vt := 0.0
			if step < len(starts) {
				vt = starts[step]
			}
			// How long a whole note lasts in this bar. A piece is not one
			// speed, so this is asked bar by bar rather than once for the
			// score: a tempo change partway through has to reach the notes
			// as well as the barlines, or the two drift apart from the bar
			// it is written at onwards.
			secondsPerWhole := wholeNoteSeconds(s, mi)

			ref := refBase[pi][mi]
			if m.Repeat > 0 {
				// A bar showing a repeat sign has no noteheads of its own —
				// the sign is all that is drawn. Its notes are borrowed from
				// the bar it stands for, so numbering them after that bar's
				// noteheads would light up a bar somewhere else on the page.
				// Zero means "nothing to highlight", which is the truth.
				ref = 0
			}
			// A two-note tremolo is two written notes and one sound, so the
			// first of the pair plays both and the second plays nothing.
			// The second still takes its own time: the bar is the same
			// length either way.
			// Which number the engraver gave each of this voice's elements.
			// It numbers a bar by when things start, across all its voices,
			// so a voice's third element is not necessarily the third
			// number — and a highlight on the wrong number lights up
			// somebody else's notehead.
			var voiceRefs []int
			if ref != 0 {
				voiceRefs = make([]int, len(v.Elements))
				for k, vd := range m.Ordered() {
					if vd.Voice == 0 && vd.Index < len(voiceRefs) {
						voiceRefs[vd.Index] = ref + k
					}
				}
			}

			paired := map[int]bool{}
			for ei, el := range v.Elements {
				if ei < len(voiceRefs) {
					ref = voiceRefs[ei]
				}
				d := el.Duration().Float() * secondsPerWhole
				mk := marksOf(el)
				if mk != nil && mk.Octave != nil && mk.Octave.Start {
					shift = mk.Octave.Amount
				}
				if mk != nil && mk.Trill != nil && mk.Trill.Start {
					trilling = true
				}
				// The note a slur begins on is inside it, so this is read
				// before the note is sounded and closed after.
				if mk != nil && mk.SlurStart {
					slurring = true
				}

				first := len(out.Events)
				switch n := el.(type) {
				case *model.Note:
					if n.Dynamic != model.DynamicNone {
						loud = n.Dynamic
						swell = nil // a written dynamic settles the question
					}
					gd := addGrace(&out, n.Grace, p, shift, vt, d, secondsPerWhole, pi, loud)
					if paired[ei] {
						break // sounded with the note it is paired to
					}
					if n.Tremolo.Paired() {
						if alt, at, ok := tremoloPartner(v.Elements, ei); ok {
							paired[at] = true
							addTwoNoteTremolo(&out, n.Tremolo,
								[]model.Pitch{sounding(p, n.Pitch, shift)},
								pitchesOf(alt, p, shift),
								vt+gd, d-gd+partnerSeconds(alt, secondsPerWhole),
								secondsPerWhole, pi, ref, p.Instrument, loud)
							break
						}
						// A mark with no partner is half a mark. It sounds
						// as the note it is written on rather than as
						// nothing at all.
						addNote(&out, open, &n.Marks, sounding(p, n.Pitch, shift),
							vt+gd, d-gd, pi, ref, p.Instrument, loud, n.Percussion,
							slurring && !n.SlurStop)
					} else if n.Tremolo != nil {
						addTremolo(&out, n.Tremolo,
							[]model.Pitch{sounding(p, n.Pitch, shift)},
							[]int{n.Percussion}, vt+gd, d-gd, secondsPerWhole,
							pi, ref, p.Instrument, loud)
					} else if trilling && n.Percussion == 0 {
						addTrill(&out, sounding(p, n.Pitch, shift), p.KeyAt(mi),
							vt+gd, d-gd, pi, ref, p.Instrument, loud)
					} else if orn, ok := ornamentOf(&n.Marks); ok && n.Percussion == 0 {
						addOrnament(&out, orn, sounding(p, n.Pitch, shift),
							p.KeyAt(mi), vt+gd, d-gd, pi, ref, p.Instrument, loud)
					} else {
						addNote(&out, open, &n.Marks, sounding(p, n.Pitch, shift),
							vt+gd, d-gd, pi, ref, p.Instrument, loud, n.Percussion,
							slurring && !n.SlurStop)
					}
				case *model.Chord:
					if n.Dynamic != model.DynamicNone {
						loud = n.Dynamic
						swell = nil
					}
					gd := addGrace(&out, n.Grace, p, shift, vt, d, secondsPerWhole, pi, loud)
					if paired[ei] {
						break // sounded with the chord it is paired to
					}
					if n.Tremolo.Paired() {
						if alt, at, ok := tremoloPartner(v.Elements, ei); ok {
							paired[at] = true
							ps := make([]model.Pitch, 0, len(n.Notes))
							for _, cn := range n.Notes {
								ps = append(ps, sounding(p, cn.Pitch, shift))
							}
							addTwoNoteTremolo(&out, n.Tremolo, ps,
								pitchesOf(alt, p, shift),
								vt+gd, d-gd+partnerSeconds(alt, secondsPerWhole),
								secondsPerWhole, pi, ref, p.Instrument, loud)
							break
						}
					}
					if n.Tremolo != nil && !n.Tremolo.Paired() {
						// The whole chord is struck again and again, not
						// each notehead in turn: that would be an arpeggio.
						ps := make([]model.Pitch, 0, len(n.Notes))
						ds := make([]int, 0, len(n.Notes))
						for _, cn := range n.Notes {
							ps = append(ps, sounding(p, cn.Pitch, shift))
							ds = append(ds, cn.Percussion)
						}
						addTremolo(&out, n.Tremolo, ps, ds, vt+gd, d-gd,
							secondsPerWhole, pi, ref, p.Instrument, loud)
						break
					}
					for _, cn := range n.Notes {
						addNote(&out, open, &n.Marks, sounding(p, cn.Pitch, shift),
							vt+gd, d-gd, pi, ref, p.Instrument, loud, cn.Percussion,
							slurring && !n.SlurStop)
					}
				}
				if mk != nil {
					if h := mk.Hairpin; h != nil {
						if h.Start && swell == nil {
							swell = &swelling{kind: h.Kind, from: first}
						}
						if h.Stop && swell != nil {
							swell.apply(&out, len(out.Events), loud)
							swell = nil
						}
					}

					if mk.Octave != nil && mk.Octave.Stop {
						shift = 0
					}
					if mk.Trill != nil && mk.Trill.Stop {
						trilling = false
					}
					if mk.SlurStop {
						slurring = false
					}
				}
				vt += d
			}
			// A note may ring past the last barline; the piece is not over
			// until it has stopped.
			if vt > out.Duration {
				out.Duration = vt
			}
		}
	}
	return out
}

// measureTimeline decides when each bar begins.
//
// How long a bar lasts is model.Score.MeasureDuration, and it is asked there
// rather than worked out again here: the engraver asks the same question to
// place a note within its bar, and two answers to it is exactly how the
// cursor and the note it pointed at came to be a bar apart.
//
// Returns the start of every bar and the time the last one ends.
func measureTimeline(s *model.Score, order []int) ([]float64, float64) {
	starts := make([]float64, len(order))
	at := 0.0
	for k, i := range order {
		starts[k] = at
		at += s.MeasureDuration(i).Float() * wholeNoteSeconds(s, i)
	}
	return starts, at
}

// wholeNoteSeconds is how long a whole note lasts in the bar at this index.
//
// One beat lasts 60/bpm seconds, and a whole note is however many beats fit
// in it — which is why the beat the number counts has to be carried beside
// the number. A tempo of 120 means nothing on its own.
func wholeNoteSeconds(s *model.Score, bar int) float64 {
	t := s.TempoAt(bar)
	bpm := t.BPM
	if bpm <= 0 {
		bpm = 120
	}
	unit := t.Unit
	if unit.Den == 0 || unit.Num == 0 {
		unit = model.Duration{Num: 1, Den: 4}
	}
	return (60.0 / float64(bpm)) / unit.Float()
}

// baseRefs gives the number the engraver assigned to the first element of
// each bar of each staff.
//
// The engraver numbers elements walking staves, then bars, then the bar's
// own order — by when each starts, across every voice, which is what
// Measure.Ordered gives. Playback has to agree exactly, because that number
// is the only thing tying a sounding note to the notehead drawn for it — and
// once a bar can be played more than once, counting along as we go no longer
// agrees.
func baseRefs(s *model.Score) [][]int {
	out := make([][]int, len(s.Parts))
	ref := 1
	for pi, p := range s.Parts {
		out[pi] = make([]int, len(p.Measures))
		for mi, m := range p.Measures {
			out[pi][mi] = ref
			// Every element of the bar, not the first voice's alone: the
			// engraver gives a number to each notehead it draws, and it
			// draws them all.
			ref += len(m.Ordered())
		}
	}
	return out
}

// addGrace sounds the small notes in front of a note and returns how much of
// that note's time they took.
//
// They take it from the note they lean on rather than from the note before,
// so that the bar still lasts exactly as long as it says it does. Stealing
// from the note before would be truer to how an acciaccatura is actually
// played and would put every bar after it out of step with the cursor, which
// follows the written durations — a trade the other way round is not worth
// making for an ornament a few milliseconds long.
func addGrace(out *Playback, g *model.GraceGroup, part *model.Part, shift int,
	start, dur, secondsPerWhole float64, pi int, loud model.Dynamic) float64 {

	if g == nil || len(g.Notes) == 0 {
		return 0
	}
	n := float64(len(g.Notes))
	// A sixteenth each, or a third of the note they precede shared between
	// them, whichever is less: grace notes in front of a short note have to
	// be shorter still, or they swallow it.
	each := secondsPerWhole / 16
	if each*n > dur/3 {
		each = dur / 3 / n
	}
	at := start
	for _, gn := range g.Notes {
		// Ref 0: a grace note is drawn but not numbered, so there is no
		// notehead for the editor to light up. Zero says so.
		// Slurred to what follows: a grace note is played into the note it
		// leans on, never detached from it.
		addNote(out, map[int]int{}, &model.Marks{}, sounding(part, gn.Pitch, shift),
			at, each, pi, 0, part.Instrument, loud, gn.Percussion, true)
		at += each
	}
	return each * n
}

func addNote(out *Playback, open map[int]int, m *model.Marks, p model.Pitch,
	start, dur float64, part, ref int, inst model.Instrument, loud model.Dynamic,
	drum int, slurred bool) {
	midi := p.MIDI()

	// A drum is a hit, not a note: it does not sustain, it is not tied, and
	// what sounds is named by the staff position rather than by the pitch.
	if drum > 0 {
		name := ""
		if pn, ok := model.PercussionByKey(drum); ok {
			name = pn.Name
		}
		out.Events = append(out.Events, Event{
			MIDI: drum, Start: start, Dur: dur, Part: part, Ref: ref,
			Family: "percussion", Drum: drum, DrumName: name,
			Velocity: velocityOf(m, loud),
			// A drum is struck and then rings as long as it rings: holding
			// the stick down does not lengthen it.
			Gate: gateStaccato,
		})
		return
	}

	if m.TieStop {
		if idx, ok := open[midi]; ok {
			// A tie is one sound, not two, so extend what is already running.
			out.Events[idx].Dur += dur
			if !m.TieStart {
				delete(open, midi)
			}
			return
		}
	}
	out.Events = append(out.Events, Event{
		MIDI: midi, Start: start, Dur: dur, Part: part, Ref: ref,
		Program: int(inst), Family: inst.Family().String(),
		Velocity: velocityOf(m, loud), Gate: breath(gateOf(m, slurred), dur),
	})
	if m.TieStart {
		open[midi] = len(out.Events) - 1
	}
}

// buzzRate is how fast an unmeasured roll is taken, in strokes a second.
//
// A press roll has no division to count: the player is asked for as fast as
// the instrument will answer, and the written note only says how long to
// keep going. Twenty a second is a snare drum and is fast enough to read as
// a buzz rather than as thirty-second notes.
const buzzRate = 20.0

// addTremolo sounds a tremolo as the repeated note it stands for.
//
// A tremolo is not an ornament on a note, it is a shorthand for a bar of
// them: three slashes through a half note mean sixteen thirty-second notes
// and should be heard as sixteen. Played as one long note — which is what
// happened before, the mark being drawn and then ignored — the shorthand
// silently means something different from what it is written to mean.
func addTremolo(out *Playback, tr *model.Tremolo, pitches []model.Pitch,
	drums []int, start, dur, secondsPerWhole float64,
	part, ref int, inst model.Instrument, loud model.Dynamic) {

	if dur <= 0 || len(pitches) == 0 {
		return
	}
	step := tr.Repeat().Float() * secondsPerWhole
	if tr.Buzz {
		step = 1 / buzzRate
	}
	n := 1
	if step > 0 {
		n = int(dur/step + 0.5)
	}
	if n < 2 {
		// Too short to repeat is too short to be a tremolo: it sounds as
		// the note it is written on.
		n = 1
	}
	each := dur / float64(n)
	for i := 0; i < n; i++ {
		at := start + float64(i)*each
		for j, pt := range pitches {
			drum := 0
			if j < len(drums) {
				drum = drums[j]
			}
			if drum > 0 {
				name := ""
				if pn, ok := model.PercussionByKey(drum); ok {
					name = pn.Name
				}
				out.Events = append(out.Events, Event{
					MIDI: drum, Start: at, Dur: each, Part: part, Ref: ref,
					Family: "percussion", Drum: drum, DrumName: name,
					Velocity: loud.Velocity(),
				})
				continue
			}
			out.Events = append(out.Events, Event{
				MIDI: pt.MIDI(), Start: at, Dur: each, Part: part, Ref: ref,
				Program: int(inst), Family: inst.Family().String(),
				Velocity: loud.Velocity(),
			})
		}
	}
}

// tremoloPartner finds the note a two-note tremolo is paired with: the very
// next one, which is where it is written and the only place it can be.
func tremoloPartner(els []model.Element, at int) (model.Element, int, bool) {
	if at+1 >= len(els) {
		return nil, 0, false
	}
	next := els[at+1]
	mk := marksOf(next)
	if mk == nil || !mk.Tremolo.Paired() || !mk.Tremolo.Stop {
		return nil, 0, false
	}
	return next, at + 1, true
}

// pitchesOf is what an element sounds as, one pitch or a chord's worth.
func pitchesOf(el model.Element, p *model.Part, shift int) []model.Pitch {
	switch n := el.(type) {
	case *model.Note:
		return []model.Pitch{sounding(p, n.Pitch, shift)}
	case *model.Chord:
		out := make([]model.Pitch, 0, len(n.Notes))
		for _, cn := range n.Notes {
			out = append(out, sounding(p, cn.Pitch, shift))
		}
		return out
	}
	return nil
}

func partnerSeconds(el model.Element, secondsPerWhole float64) float64 {
	return el.Duration().Float() * secondsPerWhole
}

// addTwoNoteTremolo sounds a pair as the two notes alternating.
//
// Not as two notes: two half notes with three beams between them are not a
// pair of half notes, they are sixteen thirty-second notes going back and
// forth between the two pitches, and the bar lasts exactly as long either
// way. Played as written the mark would be drawn and then ignored, which is
// the one thing a shorthand must not be.
func addTwoNoteTremolo(out *Playback, tr *model.Tremolo, first, second []model.Pitch,
	start, dur, secondsPerWhole float64,
	part, ref int, inst model.Instrument, loud model.Dynamic) {

	if dur <= 0 || len(first) == 0 {
		return
	}
	if len(second) == 0 {
		second = first
	}
	step := tr.Repeat().Float() * secondsPerWhole
	if tr.Buzz {
		step = 1 / buzzRate
	}
	n := 2
	if step > 0 {
		n = int(dur/step + 0.5)
	}
	if n < 2 {
		n = 2 // a pair alternates at least once, or it is not a pair
	}
	if n%2 == 1 {
		n++ // and ends on the second note, having been to it as often
	}
	each := dur / float64(n)
	for i := 0; i < n; i++ {
		pitches := first
		if i%2 == 1 {
			pitches = second
		}
		at := start + float64(i)*each
		for _, pt := range pitches {
			out.Events = append(out.Events, Event{
				MIDI: pt.MIDI(), Start: at, Dur: each, Part: part, Ref: ref,
				Program: int(inst), Family: inst.Family().String(),
				Velocity: loud.Velocity(),
			})
		}
	}
}

// PageSound is the score resolved into sounds, in the shape a shared web
// page's player reads.
//
// The page is handed the answer rather than the score: working out when
// every note sounds is this program's job, and a page that had to do it
// would have to carry the program.
func PageSound(s *model.Score, lay *engrave.Layout, scale float64) *ghtml.Sound {
	pb := buildPlayback(s)
	out := &ghtml.Sound{
		Duration: pb.Duration, Tempo: pb.Tempo,
		Notes: make([]ghtml.Note, 0, len(pb.Events)),
		Bars:  make([]ghtml.Bar, 0, len(pb.Measures)),
	}
	for _, e := range pb.Events {
		out.Notes = append(out.Notes, ghtml.Note{
			Start: e.Start, Dur: e.Dur, MIDI: e.MIDI, Gate: e.Gate,
			Velocity: e.Velocity, Family: e.Family, Ref: e.Ref,
		})
	}
	for _, m := range pb.Measures {
		out.Bars = append(out.Bars, ghtml.Bar{Index: m.Index, Start: m.Start, Dur: m.Dur})
	}
	out.Boxes = ghtml.Boxes(lay, scale)
	return out
}

func (a *App) handlePlayback(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	pb := buildPlayback(a.score)
	a.mu.RUnlock()
	writeJSON(w, pb)
}

/*
 * Small helpers the server needs.
 */

func fontData() []byte { return smufl.FontData() }

// pngEncode writes an image, used for the symbol palette.
func pngEncode(w io.Writer, img image.Image) error { return png.Encode(w, img) }

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// fs strips the embedded directory prefix so the UI is served from the root.
func fsSub(f fs.FS, dir string) (fs.FS, error) { return fs.Sub(f, dir) }

// sounding turns a written pitch into the one that is heard: the staff's
// transposition, and any octave line over it.
//
// An alto saxophone's part is written a major sixth above what comes out of
// the instrument, and an 8va line means "play this an octave higher than it is
// written" precisely so that the notes can stay on the staff. Both are
// differences between the page and the sound, and both belong here rather than
// in the model, which holds what is printed.
func sounding(p *model.Part, pitch model.Pitch, octaves int) model.Pitch {
	out := p.Transpose.Apply(pitch)
	out.Octave += octaves
	return out
}

// marksOf gets an element's marks, or nil for one that carries none.
func marksOf(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	}
	return nil
}

// swelling is a hairpin being collected: the events it covers, and which way
// it goes.
type swelling struct {
	kind model.HairpinKind
	from int
}

// apply ramps the velocity of everything under the hairpin.
//
// A crescendo is not a volume, it is a journey between two: it starts where
// the music already is and arrives one step louder, unless a written dynamic
// says otherwise, in which case the dynamic has already replaced this.
func (s *swelling) apply(out *Playback, end int, loud model.Dynamic) {
	// Everything from the note it opens on to the note it closes on, that
	// last one included: a crescendo arrives somewhere, and where it
	// arrives is the note the sign stops under. Ramping up to it and
	// leaving it at the volume the music started from is the one note that
	// must not be left behind.
	n := end - s.from
	if n <= 0 {
		return
	}
	to := end
	base := out.Events[s.from].Velocity
	span := 28
	if s.kind == model.Diminuendo {
		span = -28
	}
	// Clamp so a crescendo from fff does not run off the top.
	if base+span > 124 {
		span = 124 - base
	}
	if base+span < 16 {
		span = 16 - base
	}
	for i := s.from; i < to; i++ {
		f := float64(i-s.from) / float64(n-1)
		if n == 1 {
			f = 1
		}
		out.Events[i].Velocity = base + int(float64(span)*f)
	}
	_ = loud
}

// trillRate is how many notes a second a trill is played at.
//
// A trill is not measured: it is as fast as the player can make it sound
// even, and a written-out rhythm would be a different ornament. Twelve a
// second is about what a wind player does and is fast enough to read as a
// trill rather than as a run.
const trillRate = 12.0

// addTrill sounds a trilled note as the alternation it is.
//
// Between the note written and the one above it in the key — a trill is a
// scale step, not a fixed interval, so in a flat key it is a semitone where
// the scale has one. It starts on the written note, which is what the
// ornament means today; in the eighteenth century it began on the note above
// and a score that wants that writes it out.
func addTrill(out *Playback, p model.Pitch, key model.KeySignature,
	start, dur float64, part, ref int, inst model.Instrument, loud model.Dynamic) {

	if dur <= 0 {
		return
	}
	upper := p
	upper.Step = (p.Step + 1) % 7
	if upper.Step == model.StepC {
		upper.Octave++
	}
	upper.Alter = key.AlterFor(upper.Step)

	n := int(dur*trillRate + 0.5)
	if n < 2 {
		// Too short to alternate is too short to be a trill; it sounds as
		// the note it is written on.
		n = 1
	}
	step := dur / float64(n)
	for i := 0; i < n; i++ {
		at := p
		if i%2 == 1 {
			at = upper
		}
		out.Events = append(out.Events, Event{
			MIDI: at.MIDI(), Start: start + float64(i)*step, Dur: step,
			Part: part, Ref: ref,
			Program: int(inst), Family: inst.Family().String(),
			Velocity: loud.Velocity(),
		})
	}
}

// ornamentOf is the ornament written on a note, or false where there is
// none. Only one is played: two ornaments on one note is not a thing anybody
// writes, and the first is the one that was meant.
func ornamentOf(m *model.Marks) (model.Articulation, bool) {
	if m == nil {
		return 0, false
	}
	for _, a := range m.Articulations {
		if a.Ornament() {
			return a, true
		}
	}
	return 0, false
}

// ornamentRate is how fast the little notes of an ornament go, in notes a
// second.
//
// Faster than a trill, because an ornament is not a texture but a flick:
// three notes at the front of the written one, over and done with before the
// ear has decided they were separate.
const ornamentRate = 14

// addOrnament sounds a mordent, a short trill or a turn.
//
// The ornament is played at the front of the note and the rest of the note
// is what is left. A mordent drops to the note below and comes back, a short
// trill goes up and comes back, a turn goes round the note above, the note
// itself, the note below and back — which is what the signs say, and what a
// player does with them.
//
// The neighbours come from the key, like a trill's: a mordent in E flat drops
// a semitone where the key says so and a whole tone where it does not.
func addOrnament(out *Playback, a model.Articulation, p model.Pitch,
	key model.KeySignature, start, dur float64,
	part, ref int, inst model.Instrument, loud model.Dynamic) {

	if dur <= 0 {
		return
	}
	step := func(by int) model.Pitch {
		n := p
		n.Step = model.Step((int(p.Step) + by + 7) % 7)
		if by > 0 && n.Step < p.Step {
			n.Octave++
		}
		if by < 0 && n.Step > p.Step {
			n.Octave--
		}
		n.Alter = key.AlterFor(n.Step)
		return n
	}

	var notes []model.Pitch
	switch a {
	case model.Mordent:
		notes = []model.Pitch{p, step(-1), p}
	case model.ShortTrill:
		notes = []model.Pitch{p, step(1), p}
	case model.Turn:
		notes = []model.Pitch{step(1), p, step(-1), p}
	case model.TurnInverted:
		notes = []model.Pitch{step(-1), p, step(1), p}
	default:
		return
	}

	// The flick, then the note. A note too short to hold both is all flick:
	// squeezing an ornament into a demisemiquaver is what the player would
	// do too.
	quick := 1 / float64(ornamentRate)
	if n := float64(len(notes) - 1); quick*n > dur/2 {
		quick = dur / 2 / n
	}
	at := start
	for i, pt := range notes {
		d := quick
		if i == len(notes)-1 {
			d = start + dur - at // the written note, for what is left of it
		}
		if d <= 0 {
			break
		}
		out.Events = append(out.Events, Event{
			MIDI: pt.MIDI(), Start: at, Dur: d, Part: part, Ref: ref,
			Program: int(inst), Family: inst.Family().String(),
			Velocity: loud.Velocity(), Gate: breath(gatePlain, d),
		})
		at += d
	}
}

// MIDIFile writes the score as a standard MIDI file.
//
// It is the same performance the editor plays — the one place that works out
// what the music sounds like — handed to something that writes it down for
// other programs. Ties, repeats, trills, tremolos, grace notes, the dynamics
// and the swells between them, every transposition: all of it is already
// resolved by the time this sees it.
func MIDIFile(w io.Writer, s *model.Score) error {
	return midi.Write(w, Song(s))
}

// Song is the performance in the terms anything outside the editor reads it
// in: notes in seconds, and the bars they fall in.
//
// Both the MIDI file and the recording are made from this, so the two cannot
// drift apart — a recording that phrased differently from the MIDI of the
// same score would mean one of them was wrong about the music.
func Song(s *model.Score) midi.Song {
	pb := buildPlayback(s)

	song := midi.Song{Name: s.Identity.Title}
	for _, e := range pb.Events {
		song.Notes = append(song.Notes, midi.Note{
			Start: e.Start, Dur: e.Dur, Gate: e.Gate, MIDI: e.MIDI,
			Velocity: e.Velocity, Part: e.Part, Program: e.Program,
			Drum: e.Drum,
		})
	}
	// Each bar as the performance played it, in seconds and in quarter
	// notes: the first says where the notes are and the second says what
	// the tempo was, and a MIDI file needs both.
	for _, m := range pb.Measures {
		song.Bars = append(song.Bars, midi.Bar{
			Start: m.Start, Dur: m.Dur,
			Quarters: s.MeasureDuration(m.Index).Float() * 4,
		})
	}
	return song
}
