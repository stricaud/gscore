/*
 * edit.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/format/abc"
	"github.com/stricaud/gscore/internal/model"
)

// Editing happens on the server, against the model, not in the browser against
// the drawing. The engraver is the only thing that knows where a note went, so
// it is also the thing that says where a click landed; everything after that is
// a change to the music, which is then drawn again from scratch.
//
// That is the whole point of keeping geometry out of the model: an edit never
// has to patch coordinates, because there are none to patch.

// handleHits reports the clickable regions for the current view.
func (a *App) handleHits(w http.ResponseWriter, r *http.Request) {
	// Through viewOptions, as the drawing and its size are: where a note
	// went depends on every one of the settings an address can carry, not
	// on the mode alone. A width taken from a different picture reports
	// notes at coordinates nothing on the screen is drawn at — a click
	// lands in the wrong bar, and a score wrapped to the width of a frame
	// is measured as though it were not wrapped at all.
	score, opt, _ := a.viewOptions(r)
	lay := a.layout(score, opt)

	writeJSON(w, map[string]any{
		"hits":          lay.Hits,
		"dynamics":      lay.Dynamics,
		"textMarks":     lay.TextMarks,
		"markers":       lay.Markers,
		"articulations": lay.Articulations,
		"chords":        lay.Chords,
		"tempos":        lay.Tempos,
		"hairpins":      lay.Hairpins,
		"arcs":          lay.Arcs,
		"tremolos":      lay.Tremolos,
		"tuplets":       lay.Tuplets,
		"graces":        lay.Graces,
		"barlines":      lay.Barlines,
		"strips":        lay.Strips,
		"headers":       lay.Headers,
		"staffHeight":   4.0,
	})
}

var durationNames = map[string]model.Duration{
	"breve":     {Num: 2, Den: 1},
	"whole":     {Num: 1, Den: 1},
	"half":      {Num: 1, Den: 2},
	"quarter":   {Num: 1, Den: 4},
	"eighth":    {Num: 1, Den: 8},
	"sixteenth": {Num: 1, Den: 16},
	"32nd":      {Num: 1, Den: 32},
	"64th":      {Num: 1, Den: 64},
}

// dotted lengthens a note by its dots: half as long again for the first,
// half of that again for the second.
//
// A dot is part of how long a note is, not a mark on it — a dotted half is
// three quarters of a whole note and nothing in the model needs to know it
// was written with a dot. The engraver works the other way round, finding
// the notehead and the dots that spell a duration, so this has only to get
// the length right.
// maxWrittenDots is as many dots as a note is given. A third turns up in
// Baroque music and a fourth is not written at all; the model reads up to
// three, so this is the same number and the two cannot drift apart.
const maxWrittenDots = 3

func dotted(d model.Duration, dots string) model.Duration {
	n, err := strconv.Atoi(dots)
	if err != nil || n <= 0 {
		return d
	}
	if n > maxWrittenDots {
		n = maxWrittenDots
	}
	// Each dot adds half of the note again, then a quarter, then an eighth:
	// the whole thing comes to twice the note less what is left, which is
	// (2^(n+1) - 1) over 2^n. A double-dotted quarter is seven sixteenths.
	return d.Scale(1<<(n+1)-1, 1<<n).Reduced()
}

var accidentalByName = map[string]model.Accidental{
	"":             model.NoAccidental,
	"none":         model.NoAccidental,
	"natural":      model.Natural,
	"sharp":        model.Sharp,
	"flat":         model.Flat,
	"double sharp": model.DoubleSharp,
	"double flat":  model.DoubleFlat,
}

var clefByStringName = map[string]model.Clef{
	"treble": model.Treble, "bass": model.Bass, "alto": model.Alto,
	"tenor": model.Tenor, "percussion": model.Percussion,
	"treble8vb": model.Treble8vb,
}

// handleInsert puts a note or a rest into a measure.
func (a *App) handleInsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1 // append
	}
	step, _ := strconv.Atoi(q.Get("step"))

	dur, ok := durationNames[q.Get("dur")]
	if !ok {
		dur = model.Duration{Num: 1, Den: 4}
	}
	dur = dotted(dur, q.Get("dots"))
	acc := accidentalByName[q.Get("accidental")]

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locateForWriting(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]

	var el model.Element
	switch {
	case q.Get("kind") == "rest":
		el = &model.Rest{Dur: dur}
	case p.Clef == model.Percussion:
		// A drum staff's lines carry no pitches: a position names an
		// instrument. The note still has a pitch, because everything else —
		// stems, beams, spacing — works from staff positions, but what
		// sounds comes from the kit and not from the pitch.
		//
		// Naming the drum beats guessing it from where the click landed. A
		// drummer picks the snare and then says where in the bar; asking
		// them to remember which line the snare is on, and to hit it, is the
		// wrong way round.
		var drum model.PercussionNote
		var ok bool
		if v := q.Get("drum"); v != "" {
			key, err := strconv.Atoi(v)
			if err != nil {
				http.Error(w, "a drum is named by its MIDI key", http.StatusBadRequest)
				return
			}
			drum, ok = model.PercussionByKey(key)
			if !ok {
				http.Error(w, "no drum has that key", http.StatusBadRequest)
				return
			}
		} else {
			drum, ok = model.PercussionFor(step)
		}
		if !ok {
			http.Error(w, "nothing is written at that position", http.StatusBadRequest)
			return
		}
		el = &model.Note{
			Pitch:      pitchForStep(p.Clef, drum.Step),
			Dur:        dur,
			Head:       drum.Head,
			Percussion: drum.Key,
		}
	default:
		// The staff step is what was clicked; the clef turns it into a pitch,
		// and the key signature decides what it sounds like when no accidental
		// is written on it.
		// Read off the staff as it is drawn. Where that drawing is in
		// concert pitch, what was clicked is a sounding pitch and what is
		// stored is what the player reads, so it goes back through the
		// part's own interval on the way in.
		pitch := a.asWritten(p, pitchForStep(p.Clef, step))
		named := false
		// spelled says the alteration on the pitch is already the one to
		// write, so the key must not overwrite it. A staff position says
		// nothing about sharps and flats and the key has the last word;
		// a note that was played, or typed with a sign on it, has already
		// been decided.
		spelled := false

		// Or the pitch is named outright. Somebody typing a tune in writes
		// "c4 d e f g" and means a scale: they are not thinking in staff
		// positions, and asking them to is asking them to do the clef's
		// arithmetic in their head. The clef is still what draws it.
		if v := q.Get("pitch"); v != "" {
			got, hasOctave, ok := model.ParsePitch(v)
			if !ok {
				http.Error(w, v+" is not a pitch", http.StatusBadRequest)
				return
			}
			if !hasOctave {
				// No octave given: whichever one is nearest the note
				// before, which is what a run of bare letters means.
				got = got.NearestTo(lastPitch(p, m, at, pitch))
			}
			pitch, named = a.asWritten(p, got), true
		}

		// Or the note was played rather than typed. A key pressed on a
		// MIDI keyboard gives a number and nothing else — the same number
		// for D sharp and E flat — so the key signature spells it, which
		// is the only thing in the question with an opinion.
		//
		// The number is what sounds, because that is what a keyboard
		// sends and what an instrument makes. What is stored is what the
		// player reads, so it comes back through the part's own interval
		// first: middle C played on a B flat trumpet's part is written a
		// tone above where it sounds, and a part that stored the number
		// as it arrived would sound a tone flat for ever after.
		if v := q.Get("midi"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 127 {
				http.Error(w, "a MIDI note is a number from 0 to 127",
					http.StatusBadRequest)
				return
			}
			sounding := model.PitchInKey(n, model.KeySignature{})
			written := p.Transpose.Unapply(sounding)
			pitch = model.PitchInKey(written.MIDI(), p.KeyAt(measIdx))
			named, spelled = true, true
		}

		// Or by how far it is from the note before, which is how a melody
		// is thought about and said: up a third, down a step. The number is
		// in diatonic steps, so 1 is the next letter up and -1 the one
		// below, and the key signature spells it.
		if v := q.Get("interval"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				http.Error(w, "an interval is a number of steps", http.StatusBadRequest)
				return
			}
			from := lastPitch(p, m, at, pitch)
			pitch = model.PitchFromDiatonicIndex(from.DiatonicIndex() + n)
			named = false
		}

		// Whether the sign was asked for, which has to be noted before the
		// switch below works one out: a sign somebody chose stands, and a
		// sign this worked out is SpellAccidentals' to correct. Read after
		// the switch the two are indistinguishable, and freezing the
		// computed one would put a note beyond the reach of every later
		// edit to the bar in front of it.
		chosen := acc != model.NoAccidental
		switch {
		case chosen:
			// Chosen from the palette: the sign decides how the note
			// sounds. Whether it is printed is another question, and not
			// this one's — see below.
			pitch.Alter = acc.Semitones()
		case spelled || (named && hasAccidental(q.Get("pitch"))):
			// Spelled already — as it was typed, or as the key spells the
			// note that was played — so the pitch stands as it is.
		default:
			// A bare letter: the bar says how it sounds. That is the key
			// signature where nothing has happened yet, and whatever an
			// accidental earlier in the bar made of that letter where
			// something has — a B written after a B flat is a B flat,
			// because that is what the player will read and what the
			// writer, reading as they go, meant.
			//
			// The key alone put a B natural there, and then the page
			// printed a natural in front of it to say so: a sign nobody
			// asked for, on a note nobody altered, with nothing to take it
			// off but flattening a note that was flat already.
			before := at
			if before < 0 || before > len(v.Elements) {
				before = len(v.Elements)
			}
			pitch.Alter = p.AlterInForce(measIdx, 0, before, pitch)
		}
		// A sign is printed when it tells the reader something they do not
		// already know, and that is all. Asking for a natural on a note
		// that is natural already is asking for the note to be natural,
		// not for a sign saying so — and a bar of three F naturals, each
		// with its own sign because each was asked for, is not a bar
		// anybody wrote.
		//
		// So nothing here is forced and SpellAccidentals decides: the first
		// of those three prints its natural, because the key signature says
		// sharp, and the two after it print nothing, because by then the
		// bar has said it. A sign that is wanted anyway — a reminder in a
		// later bar — is what the brackets are for.
		el = &model.Note{Pitch: pitch, Dur: dur, Accidental: acc}
	}

	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}

	at = openEmptyBar(p, m, measIdx, at)

	// Writing over what is picked up rather than in front of it.
	//
	// Clicking a note and typing is correcting it, not adding to it: the bar
	// already has the right number of beats in it and pushing the rest along
	// — into the next bar, and the one after that, making bars as it goes —
	// is the opposite of what was asked for. Only a note somebody pointed at
	// is replaced; writing at the cursor still writes.
	if q.Get("replace") == "1" && at < len(v.Elements) {
		old := v.Elements[at]
		v.Elements[at] = el
		// A longer note than the one it replaced still has to go somewhere,
		// so the bar is settled the same way as after an insert. A shorter
		// one simply leaves the bar short, which is the writer's business.
		if el.Duration().Cmp(old.Duration()) > 0 {
			spill(p, measIdx)
		}
		part, idx := findElement(p, measIdx, el)
		out := map[string]any{
			"status": "ok", "measure": part, "at": idx, "index": idx,
			"replaced": true,
		}
		addSounding(out, p, el)
		writeJSON(w, out)
		return
	}

	// Writing at the end of a bar that is already full means the next bar,
	// unless the barlines are being left alone. See carryOn.
	m, measIdx, at = carryOn(p, m, measIdx, at)
	v = m.Voices[0]

	v.Elements = append(v.Elements, nil)
	copy(v.Elements[at+1:], v.Elements[at:])
	v.Elements[at] = el

	// Inserting in the middle can push the bar past its time signature; what
	// no longer fits moves on rather than being squeezed in.
	spill(p, measIdx)

	// The element may now be in a later measure than the one written to, so
	// the answer says where it actually is and the cursor follows it there.
	part, idx := findElement(p, measIdx, el)
	out := map[string]any{
		"status": "ok", "measure": part, "at": idx, "index": idx,
	}
	addSounding(out, p, el)
	writeJSON(w, out)
}

// addSounding says what the element just written sounds like, so that the
// editor can play it as it is typed.
//
// Here and not worked out in the page, because the page is in no position
// to: what a staff position sounds like is the clef, the key signature, any
// accidental on the note, and the part's own transposition, and a drum
// staff's positions are not pitches at all. All of that has just been
// settled a few lines above. The alternative is a second implementation of
// it in JavaScript, which would be wrong about a clarinet on the day it was
// written.
//
// A rest adds nothing, and silence is the right sound for one.
func addSounding(out map[string]any, p *model.Part, el model.Element) {
	n, ok := el.(*model.Note)
	if !ok {
		return
	}
	if n.Percussion > 0 {
		// What sounds is the drum the position names, not the pitch the
		// notehead happens to sit on.
		out["drum"] = n.Percussion
		out["family"] = "percussion"
		return
	}
	out["midi"] = p.Transpose.Apply(n.Pitch).MIDI()
	out["family"] = p.Instrument.Family().String()
}

// locateForWriting finds a measure to write into, making it if the click
// landed in the empty bar the engraver draws past the end of the music.
//
// That bar is drawn and not stored — there is somewhere to click before there
// is anything to click on — so writing into it is what turns it into a bar.
// Only the one directly after the last: anywhere further is a stale click
// from before something else changed the score.
// errVoicedElsewhere is a staff that cannot be written to from here.
//
// A part showing a voicing is showing a choice of the staff's notes — the
// lower note of each chord, or the second layer — and the notes drawn are
// not the notes written. There is no honest way to aim an edit at them: the
// third thing on the page is not the third thing in the bar, and a note
// typed over it would land somewhere else. So the part is read here and
// written in the score, which is the same answer every program that has
// voicings has arrived at.
func errVoicedElsewhere(name string) error {
	if name == "" {
		name = "this part"
	}
	return editError("the notes " + name + " shows are chosen from a staff " +
		"that two players share — turn to the full score to change them")
}

// voicedHere reports that the staff is being shown through a voicing rule,
// so what is drawn is not what is written.
func (a *App) voicedHere(staff int) (string, bool) {
	if a.sheet < 0 || a.sheet >= len(a.score.Sheets) {
		return "", false
	}
	sh := a.score.Sheets[a.sheet]
	if _, ok := sh.VoicingFor(staff); !ok {
		return "", false
	}
	return sh.Name, true
}

func (a *App) locateForWriting(partIdx, measIdx int) (*model.Part, *model.Measure, error) {
	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		return nil, nil, editError("no such staff")
	}
	partIdx = a.score.MusicIndex(partIdx)
	if name, ok := a.voicedHere(partIdx); ok {
		return nil, nil, errVoicedElsewhere(name)
	}
	p := a.score.Parts[partIdx]
	if measIdx == len(p.Measures) && measIdx > 0 {
		// Every staff gains the bar, so the barlines stay in a column.
		for _, other := range a.score.Parts {
			measureAt(other, measIdx)
		}
		return p, p.Measures[measIdx], nil
	}
	return a.locate(partIdx, measIdx)
}

// measureAt returns a part's nth measure, making it and any before it if the
// part does not reach that far yet.
func measureAt(p *model.Part, n int) *model.Measure {
	for len(p.Measures) <= n {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{}}, RightBarline: model.BarSingle,
		})
	}
	m := p.Measures[n]
	if len(m.Voices) == 0 {
		m.Voices = []*model.Voice{{}}
	}
	return m
}

// spill moves whatever overflows a measure into the one after it, making new
// measures as it goes.
//
// Notes are moved whole and never divided: splitting a note across a barline
// means tying the halves together, and a tie the user did not ask for is a
// change to the music rather than to its layout.
//
// Nothing happens at all when the barlines are not being kept — see
// TrackBarlines. A great deal of old music is written with bars that do not
// add up: a figured bass with the harmony sketched in, a chant barred by
// phrase rather than by beat, an exercise whose last bar is however long the
// line was. A program that insists on the arithmetic cannot be used to type
// any of it in, because every bar it fixes is a bar it has rewritten.
func spill(p *model.Part, from int) {
	if !TrackBarlines() {
		return
	}
	bar := p.Time.BarDuration()
	if bar.IsZero() {
		return
	}
	for i := from; i < len(p.Measures); i++ {
		v := p.Measures[i].Voices[0]
		moved := false
		for len(v.Elements) > 1 && v.Duration().Cmp(bar) > 0 {
			last := v.Elements[len(v.Elements)-1]
			v.Elements = v.Elements[:len(v.Elements)-1]
			next := measureAt(p, i+1).Voices[0]
			next.Elements = append([]model.Element{last}, next.Elements...)
			moved = true
		}
		// Stop where the pushing stops. Carrying on to the end of the part
		// tidied bars nobody had touched — which is the whole of what
		// somebody turning the tracking back on asked not to happen, and
		// was wrong even with it on: an overfull bar five systems away is
		// not a consequence of the note just written.
		if !moved {
			return
		}
	}
}

// carryOn says where to write when the bar being written to is already as
// long as the meter allows, and hands back the bar it chose.
//
// At the end of a full bar, the next one. Without that the music piles up
// behind the barline and there is no way to reach the measure after it,
// which is the one on the screen.
//
// Unless the barlines are not being kept. Somebody who turned that off said
// their bars are their own business, and this was overruling them: the
// switch stopped spill carrying anything over afterwards, but the writing
// point moved over the barline before spill was ever reached, so a figure
// longer than the bar still broke in two at the line and could not be typed
// into one bar however it was asked for. The twelve thirty-seconds in the
// time of eight that the Altes writes in one bar is the ordinary case, and
// it is what the switch is for.
//
// The caller's voice is stale afterwards, since the bar may have changed.
func carryOn(p *model.Part, m *model.Measure, index, at int) (*model.Measure, int, int) {
	v := m.Voices[0]
	if at != len(v.Elements) || !TrackBarlines() {
		return m, index, at
	}
	if v.Duration().Cmp(p.Time.BarDuration()) < 0 {
		return m, index, at
	}
	index++
	return measureAt(p, index), index, 0
}

// openEmptyBar takes a bar's rest out when something is being written into
// it, and says where the writing now starts.
//
// A bar of rest is what an empty bar is. The rest is put there so that the
// bar says "do not play" rather than "not written yet", and so that it can
// be picked up and dotted and taken away like anything else. The moment
// something is written in the bar it is no longer empty, and leaving the
// rest in front of what arrives would make every bar start with a bar's
// worth of silence it never asked for.
//
// Worse than untidy, for anything longer than a note: the bar is full as
// it stands, so what is written goes in after the rest, does not fit, and
// is carried into the bar after. Copying a bar of the right hand and
// pasting it into the empty left hand put it one bar late — which is what
// this is for as much as the tidiness.
func openEmptyBar(p *model.Part, m *model.Measure, index, at int) int {
	v := m.Voices[0]
	if len(v.Elements) != 1 {
		return at
	}
	r, ok := v.Elements[0].(*model.Rest)
	if !ok || r.Dur.Cmp(p.TimeAt(index).BarDuration()) != 0 {
		return at
	}
	v.Elements = v.Elements[:0]
	return 0
}

// findElement says where an element ended up after a spill: which measure
// holds it, and the cursor position just after it.
func findElement(p *model.Part, from int, el model.Element) (measure, at int) {
	for i := from; i < len(p.Measures); i++ {
		for j, e := range p.Measures[i].Voices[0].Elements {
			if e == el {
				return i, j + 1
			}
		}
	}
	return from, 0
}

// handleAddPitch adds a notehead to a note or a chord already written,
// which is how a chord is built.
//
// gscore made chords with Ctrl held, and chords.c refused two noteheads of
// the same pitch; both are kept, the modifier having become a click in the
// same column at a different height.
func (a *App) handleAddPitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	index, _ := strconv.Atoi(q.Get("index"))
	step, _ := strconv.Atoi(q.Get("step"))
	acc := accidentalByName[q.Get("accidental")]

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if index < 0 || index >= len(v.Elements) {
		http.Error(w, "there is no note there", http.StatusBadRequest)
		return
	}

	pitch := a.asWritten(p, pitchForStep(p.Clef, step))
	named := false
	// Or named outright, the same way a note is written: a chord typed or
	// scripted is a list of names, and turning each of them into a staff
	// position is the clef's arithmetic done by hand.
	if name := q.Get("pitch"); name != "" {
		got, hasOctave, ok := model.ParsePitch(name)
		if !ok {
			http.Error(w, name+" is not a pitch", http.StatusBadRequest)
			return
		}
		got = a.asWritten(p, got)
		if !hasOctave {
			// above says the notehead is being stacked on the ones already
			// there, which is what a chord typed as a list of letters means:
			// it is written from the bottom up, so "[ca]" is a sixth. A
			// click says where it goes by step and needs none of this; what
			// it asks for instead is the pitch nearest that position.
			if top, ok := topNotehead(v.Elements[index]); ok && q.Get("above") != "" {
				got = got.JustAbove(top)
			} else {
				got = got.NearestTo(pitch)
			}
		}
		pitch, named = got, true
	}
	// Or played: the other keys held down while the first was struck. The
	// same rule as a single note — the number is what sounds, the part's
	// interval takes it back to what is read, and the key spells it.
	spelled := false
	if v := q.Get("midi"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 127 {
			http.Error(w, "a MIDI note is a number from 0 to 127",
				http.StatusBadRequest)
			return
		}
		written := p.Transpose.Unapply(model.PitchInKey(n, model.KeySignature{}))
		pitch = model.PitchInKey(written.MIDI(), p.KeyAt(measIdx))
		named, spelled = true, true
	}
	key := p.KeyAt(measIdx)
	// Noted before the switch works a sign out: see handleInsert.
	chosen := acc != model.NoAccidental
	switch {
	case chosen:
		pitch.Alter = acc.Semitones()
	case spelled || (named && hasAccidental(q.Get("pitch"))):
		// Spelled already, so the pitch stands as it is.
	default:
		pitch.Alter = key.AlterFor(pitch.Step)
	}
	add := &model.Note{Pitch: pitch, Dur: v.Elements[index].Duration(),
		Accidental: acc, Forced: chosen}

	switch e := v.Elements[index].(type) {
	case *model.Note:
		if e.Pitch.Step == pitch.Step && e.Pitch.Octave == pitch.Octave {
			http.Error(w, "a chord cannot hold the same pitch twice", http.StatusBadRequest)
			return
		}
		// The marks belong to the sound, not to one of its noteheads, so
		// they move to the chord: an accent on a note stays an accent when a
		// second pitch joins it.
		ch := &model.Chord{Notes: []*model.Note{{
			Pitch: e.Pitch, Dur: e.Dur, Accidental: e.Accidental,
		}, add}, Dur: e.Dur, Marks: e.Marks}
		sortChord(ch)
		v.Elements[index] = ch
	case *model.Chord:
		for _, n := range e.Notes {
			if n.Pitch.Step == pitch.Step && n.Pitch.Octave == pitch.Octave {
				http.Error(w, "a chord cannot hold the same pitch twice", http.StatusBadRequest)
				return
			}
		}
		e.Notes = append(e.Notes, add)
		sortChord(e)
	default:
		http.Error(w, "only a note can be made into a chord", http.StatusBadRequest)
		return
	}

	out := map[string]any{"status": "ok"}
	// The notehead that was just added, so that it can be heard as it is
	// written — the one that was added, and not the chord it joined: what
	// somebody typing a chord has just done is add this note.
	addSounding(out, p, add)
	writeJSON(w, out)
}

// topNotehead is the highest notehead of a note or a chord, and says so only
// for something that has one: a rest is not a chord to build on.
//
// Highest on the staff, because it is what the next notehead of a chord
// being written is stacked over.
func topNotehead(el model.Element) (model.Pitch, bool) {
	switch e := el.(type) {
	case *model.Note:
		return e.Pitch, true
	case *model.Chord:
		if len(e.Notes) == 0 {
			return model.Pitch{}, false
		}
		top := e.Notes[0].Pitch
		for _, n := range e.Notes[1:] {
			if n.Pitch.DiatonicIndex() > top.DiatonicIndex() {
				top = n.Pitch
			}
		}
		return top, true
	}
	return model.Pitch{}, false
}

// sortChord puts the noteheads in order from the bottom up, which is the
// order the engraver's second-interval rule assumes.
func sortChord(c *model.Chord) {
	sort.SliceStable(c.Notes, func(i, j int) bool {
		return c.Notes[i].Pitch.DiatonicIndex() < c.Notes[j].Pitch.DiatonicIndex()
	})
}

// pitchForStep inverts the staff position back into a pitch.
//
// The engraver's own table, not a copy of it. A copy was kept here for a
// while on the grounds that it was small; small is not the problem with a
// second copy of a table that has to agree exactly with the first.
func pitchForStep(c model.Clef, step int) model.Pitch {
	return engrave.PitchForStep(c, step)
}

var barlineByName = map[string]model.BarlineStyle{
	"single": model.BarSingle, "double": model.BarDouble, "final": model.BarFinal,
	"heavy-light": model.BarHeavyLight, "opening": model.BarHeavyLight,
	"repeat-start": model.BarRepeatStart, "repeat-end": model.BarRepeatEnd,
	"repeat-both": model.BarRepeatBoth, "none": model.BarNone,
}

// handleBarline changes how a measure ends.
func (a *App) handleBarline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	style, ok := barlineByName[q.Get("style")]
	if !ok {
		http.Error(w, "unknown barline style", http.StatusBadRequest)
		return
	}
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))

	a.lock()
	defer a.mu.Unlock()

	if _, _, err := a.locate(partIdx, measIdx); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// A barline runs down the whole system. Changing it on one staff and not
	// the others would draw a repeat that only one player is asked to take.
	for _, p := range a.score.Parts {
		if measIdx < len(p.Measures) {
			p.Measures[measIdx].RightBarline = style
		}
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleDelete removes one element.
func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, _ := strconv.Atoi(q.Get("at"))

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if at < 0 || at >= len(v.Elements) {
		http.Error(w, "nothing there to delete", http.StatusBadRequest)
		return
	}

	// One of the small notes leaning on it, where that is what was picked
	// up. The group goes with its last note: an empty group is a thing the
	// page cannot show and nothing could then be clicked to be rid of.
	if g := q.Get("grace"); g != "" {
		ord, err := strconv.Atoi(g)
		mk := marksOfElement(v.Elements[at])
		if err != nil || ord < 0 || mk == nil || mk.Grace == nil ||
			ord >= len(mk.Grace.Notes) {
			http.Error(w, "there is no grace note there", http.StatusBadRequest)
			return
		}
		mk.Grace.Notes = append(mk.Grace.Notes[:ord], mk.Grace.Notes[ord+1:]...)
		if len(mk.Grace.Notes) == 0 {
			mk.Grace = nil
		}
		writeJSON(w, map[string]any{"status": "ok", "deleted": 1})
		return
	}

	// One notehead of a chord, where that is what was picked up.
	//
	// A notehead is named by the step it sits on, the way an accidental on
	// one notehead is and the way moving one is. Clicking a note of a
	// chord and pressing Backspace means that note: taking the whole chord
	// away and leaving the two that were right to be written again is not
	// what anybody is asking for.
	if v, ok := q["head"]; ok && len(v) > 0 && v[0] != "" {
		head, err := strconv.Atoi(v[0])
		if err != nil {
			http.Error(w, "a notehead is named by its staff step",
				http.StatusBadRequest)
			return
		}
		if gone, left := removeNotehead(p, m, measIdx, at, head); gone {
			writeJSON(w, map[string]any{"status": "ok", "left": left})
			return
		}
	}

	// A tuplet losing a note stops being a tuplet. See freeTuplet.
	freeTupletAt(v, at)

	v.Elements = append(v.Elements[:at], v.Elements[at+1:]...)
	writeJSON(w, map[string]string{"status": "ok"})
}

// removeNotehead takes one notehead out of a chord, and says whether it
// found one and how many are left.
//
// A chord of two that loses one is not a chord of one: it is a note, and
// is turned back into one so that everything downstream — the stem, the
// playback, the next notehead added to it — sees what it is. The marks the
// chord carried belong to the sound rather than to a notehead, so they go
// with it; the tie belongs to the notehead and stays with the note left.
//
// Anything that is not a chord is left alone and the caller deletes it
// whole: a note has one notehead, and taking that away is taking the note
// away.
func removeNotehead(p *model.Part, m *model.Measure, index, at, head int) (bool, int) {
	v := m.Voices[0]
	ch, ok := v.Elements[at].(*model.Chord)
	if !ok {
		return false, 0
	}
	clef := p.ClefAt(index)
	// The first notehead on that line, and only it. Two can share a line —
	// a C sharp and a C flat are both written on the C — and taking both
	// away would be answering a click on one of them with two deletions.
	cut := -1
	for i, n := range ch.Notes {
		if engrave.StaffStep(clef, n.Pitch) == head {
			cut = i
			break
		}
	}
	if cut < 0 {
		return false, 0
	}
	ch.Notes = append(ch.Notes[:cut], ch.Notes[cut+1:]...)
	switch len(ch.Notes) {
	case 0:
		v.Elements = append(v.Elements[:at], v.Elements[at+1:]...)
		return true, 0
	case 1:
		n := ch.Notes[0]
		n.Dur = ch.Dur
		marks := ch.Marks
		marks.TieStart, marks.TieStop = n.TieStart, n.TieStop
		n.Marks = marks
		v.Elements[at] = n
		return true, 1
	}
	return true, len(ch.Notes)
}

// handleDeleteSelection removes everything in a dragged selection.
func (a *App) handleDeleteSelection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	targets, err := parseTargets(r.URL.Query().Get("sel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Group by measure and remove from the back, so that deleting one element
	// does not shift the index of the next one still to go.
	byMeasure := map[[2]int][]int{}
	for _, t := range targets {
		k := [2]int{t.part, t.measure}
		byMeasure[k] = append(byMeasure[k], t.index)
	}

	a.lock()
	defer a.mu.Unlock()

	removed := 0
	for k, idx := range byMeasure {
		_, m, err := a.locate(k[0], k[1])
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.IntSlice(idx)))
		v := m.Voices[0]
		for _, i := range idx {
			if i < 0 || i >= len(v.Elements) {
				continue
			}
			// A tuplet losing a note stops being a tuplet, and the whole
			// group goes at the first of its notes to be taken: freeing it
			// moves nothing, so the indices still to be deleted are still
			// where they were. See freeTuplet.
			freeTupletAt(v, i)
			v.Elements = append(v.Elements[:i], v.Elements[i+1:]...)
			removed++
		}
	}
	writeJSON(w, map[string]int{"removed": removed})
}

// handleAddMeasure appends an empty measure to every part, so the staves stay
// the same length and their barlines keep lining up.
func (a *App) handleAddMeasure(w http.ResponseWriter, r *http.Request) {
	a.lock()
	defer a.mu.Unlock()
	for _, p := range a.score.Parts {
		// A staff that mirrors another holds no bars of its own.
		if p.Mirrors() {
			continue
		}
		p.AppendMeasure()
	}
	// A bar added is a bar of rest until something is written in it.
	restEmptyMeasures(a.score)
	writeJSON(w, map[string]int{"measures": len(a.score.Parts[0].Measures)})
}

// handleInsertMeasure puts empty bars into the middle of the piece.
//
// Music is typed in, a bar or two is missed, and the mistake is found four
// pages later. Adding bars at the end and moving everything along by hand
// is not a repair anybody should have to make.
func (a *App) handleInsertMeasure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	bars := a.score.MeasureCount()
	// Which bar the new ones follow. The bar somebody picked, since what
	// they asked for is bars to the right of it.
	after := bars - 1
	if v := q.Get("measure"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= bars {
			http.Error(w, "there is no bar "+v, http.StatusBadRequest)
			return
		}
		after = n
	}
	count := 1
	if v := q.Get("count"); v != "" {
		n, err := strconv.Atoi(v)
		// A hundred at a time is past what anybody means to type and
		// well within what a slip of the finger can ask for.
		if err != nil || n < 1 || n > 100 {
			http.Error(w, v+" is not a number of bars — 1 to 100",
				http.StatusBadRequest)
			return
		}
		count = n
	}

	// Every staff, or the one the cursor is on. Every staff is nearly
	// always right: a bar is a bar across the score and the barlines
	// have to stay in a column.
	onlyPart := -1
	if q.Get("where") == "staff" {
		n, err := strconv.Atoi(q.Get("part"))
		if err != nil || n < 0 || n >= len(a.score.Parts) {
			http.Error(w, "which staff?", http.StatusBadRequest)
			return
		}
		onlyPart = n
	}

	if a.score.InsertMeasures(after, count, onlyPart) == 0 {
		http.Error(w, "nothing could be put in there", http.StatusBadRequest)
		return
	}
	// A bar put in is a bar of rest until something is written in it.
	restEmptyMeasures(a.score)

	writeJSON(w, map[string]any{
		"status": "ok", "measures": a.score.MeasureCount(),
		"at": after + 1, "count": count,
	})
}

// locate finds a measure, making sure it has a voice to put things in.
func (a *App) locate(partIdx, measIdx int) (*model.Part, *model.Measure, error) {
	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		return nil, nil, errBadPart
	}
	// A staff that mirrors another is the music above it written a
	// second way, so reading or writing at it means the staff that holds
	// the music. Typing a fret on a guitar's tablature puts the note on
	// the notation staff, where it belongs and where it is already being
	// read from.
	p := a.score.MusicOf(partIdx)
	if measIdx < 0 || measIdx >= len(p.Measures) {
		return nil, nil, errBadMeasure
	}
	m := p.Measures[measIdx]
	if len(m.Voices) == 0 {
		m.Voices = []*model.Voice{{}}
	}
	return p, m, nil
}

type editError string

func (e editError) Error() string { return string(e) }

const (
	errBadPart    = editError("no such staff")
	errBadMeasure = editError("no such measure")
)

var stemByName = map[string]model.Stem{
	"auto": model.StemAuto, "up": model.StemUp, "down": model.StemDown,
}

// handleStem sets, or clears, the stem direction of the element at the cursor.
// sharpenBy moves an accidental one step towards the sharps or the flats,
// coming back to nothing at either end.
//
// Pressing + on a plain note sharpens it, pressing it again double-sharpens
// it, and pressing it once more takes it off: three presses and you are back
// where you started, which is what makes a key safe to lean on.
func sharpenBy(a model.Accidental, dir int) model.Accidental {
	order := []model.Accidental{
		model.DoubleFlat, model.Flat, model.NoAccidental,
		model.Sharp, model.DoubleSharp,
	}
	// Natural is written rather than implied, so it counts as nothing here:
	// a natural sharpened is a sharp.
	at := 2
	if a != model.Natural {
		for i, x := range order {
			if x == a {
				at = i
			}
		}
	}
	at += dir
	if at < 0 || at >= len(order) {
		return model.NoAccidental
	}
	return order[at]
}

// handleAccidental writes an accidental on what is selected.
//
// Choosing a sharp from the palette says what the *next* note will be; a note
// already on the staff has to be told separately, and clicking the accidental
// while it is selected is how that is said.
func (a *App) handleAccidental(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	cycle := q.Get("cycle")
	parens := q.Get("parens")

	// Brackets on their own. A bracketed accidental is a reminder rather
	// than an instruction — the note sounds the same either way — so asking
	// for them says nothing about which sign is there, and a note with no
	// sign has nothing to bracket.
	if parens != "" && cycle == "" && q.Get("accidental") == "" {
		notes, err := a.notesOf(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		changed := 0
		for _, n := range notes {
			if n.Accidental == model.NoAccidental {
				continue
			}
			switch parens {
			case "on", "1":
				n.Parens = true
			case "off", "0":
				n.Parens = false
			default:
				n.Parens = !n.Parens
			}
			changed++
		}
		if changed == 0 {
			http.Error(w, "there is no accidental here to put in brackets",
				http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"changed": changed})
		return
	}

	var acc model.Accidental
	if cycle == "" {
		var ok bool
		acc, ok = accidentalByName[q.Get("accidental")]
		if !ok {
			http.Error(w, "unknown accidental", http.StatusBadRequest)
			return
		}
	}

	notes, err := a.notesOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// How many of them came out with a sign nothing in the music required,
	// and how many had such a sign taken off again. The editor says so in
	// the status bar: the sign is drawn because it was asked for, and
	// somebody should be told that is why.
	forced, unforced := 0, 0
	for _, n := range notes {
		inForce := n.inForce()

		// A sign that says what the reader already knows.
		//
		// Asking for a sharp on an F in a key that sharpens every F changes
		// nothing about the music, so the only thing it can mean is print
		// it: a beginner reading the part has not learnt the signature yet,
		// and the teacher writing it out wants the sign there. Asking again
		// takes it off, which is the way back that this did not have.
		//
		// It is the only case where a sign is drawn against the rules of
		// notation, and it is deliberate on somebody's part every time.
		if cycle == "" && acc != model.NoAccidental &&
			acc.Semitones() == n.Pitch.Alter && n.Pitch.Alter == inForce {
			n.Forced = !n.Forced
			if n.Forced {
				n.Accidental = acc
				forced++
				if parens != "" {
					n.Parens = parens == "on" || parens == "1"
				}
			} else {
				n.Accidental = model.NoAccidental
				n.Parens = false
				unforced++
			}
			continue
		}

		switch {
		case cycle == "up":
			n.Accidental = sharpenBy(n.Accidental, 1)
		case cycle == "down":
			n.Accidental = sharpenBy(n.Accidental, -1)
		case n.Accidental == acc:
			// Asking for the accidental a note already has is asking for it
			// to go. Anything else means the sharp button has to be read as
			// "make sure this is sharp", and then nothing undoes it but a
			// second button that does nothing else.
			n.Accidental = model.NoAccidental
		default:
			n.Accidental = acc
		}
		// What was chosen here is how the note sounds. Whether a sign is
		// printed in front of it is for the rules of notation, which is
		// where SpellAccidentals leaves it: a sign is drawn when it tells
		// the reader something the key signature and the bar so far have
		// not already told them.
		//
		// Those rules may still want a sign where none was asked for:
		// clearing the second of two F sharps in a bar leaves an F
		// natural, which has to say so.
		n.Forced = false

		// Brackets belong to a sign. Taking the sign away takes them with
		// it, or the next sign written there arrives already bracketed.
		if n.Accidental == model.NoAccidental {
			n.Parens = false
		} else if parens != "" {
			n.Parens = parens == "on" || parens == "1"
		}
		// The written accidental decides how the note sounds. Taking one
		// away hands that decision back to the bar: the key signature, and
		// whatever an accidental earlier in it has already made of this
		// letter.
		if n.Accidental == model.NoAccidental {
			n.Pitch.Alter = inForce
		} else {
			n.Pitch.Alter = n.Accidental.Semitones()
		}
	}
	writeJSON(w, map[string]int{
		"changed": len(notes), "forced": forced, "unforced": unforced,
	})
}

// keyedNote is a note together with where it sits, which is what says what
// an accidental on it would mean and what taking one off would leave.
type keyedNote struct {
	*model.Note
	key model.KeySignature
	// Where it is, for asking the bar what this letter has already been
	// made into — see model.Part.AlterInForce. A note with no place is a
	// note read out of a selection that has since moved, and falls back
	// to the key signature.
	part    *model.Part
	measure int
	index   int
}

// inForce is what this note's letter is altered by where it stands, when
// nobody has said otherwise: the key signature, and whatever an accidental
// earlier in the bar has made of it.
func (n keyedNote) inForce() int {
	if n.part == nil {
		return n.key.AlterFor(n.Pitch.Step)
	}
	return n.part.AlterInForce(n.measure, 0, n.index, n.Pitch)
}

// notesOf resolves a selection to the notes in it, a chord counting as all of
// its noteheads: an accidental put on a chord belongs to the whole chord.
//
// Unless one of its noteheads was named. The head parameter is the staff step
// a notehead sits at, which is what the editor knows about the one that was
// clicked, and it narrows the chord to that note — the only way to sharpen
// the third of a chord and leave the rest of it alone. It is read only when
// one element is being asked about: a dragged selection picks up whole
// chords, and a step shared by two of them would mean different notes in
// each.
func (a *App) notesOf(q url.Values) ([]keyedNote, error) {
	a.lock()
	defer a.mu.Unlock()

	var head *int
	if v := q.Get("head"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, editError("head is a staff step")
		}
		head = &n
	}

	var out []keyedNote
	collect := func(p *model.Part, measure, index int, el model.Element, head *int) {
		switch e := el.(type) {
		case *model.Note:
			out = append(out, keyedNote{Note: e, key: p.Key,
				part: p, measure: measure, index: index})
		case *model.Chord:
			// A tablature staff writes fret numbers where noteheads would
			// go, and their height is which string rather than which pitch.
			// The step the editor read off the page is then not the step
			// the note sits at on a staff of pitches, and there is no
			// telling the chord's notes apart by it — so the chord goes
			// whole, as it did before any of them could be named.
			one := head
			if p.Tab != nil {
				one = nil
			}
			clef := p.ClefAt(measure)
			for _, n := range e.Notes {
				if one != nil && engrave.StaffStep(clef, n.Pitch) != *one {
					continue
				}
				out = append(out, keyedNote{Note: n, key: p.Key,
					part: p, measure: measure, index: index})
			}
		}
	}

	if sel := q.Get("sel"); sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			return nil, err
		}
		if len(targets) != 1 {
			head = nil
		}
		for _, t := range targets {
			p, m, err := a.locate(t.part, t.measure)
			if err != nil {
				continue // a stale selection is not worth failing over
			}
			v := m.Voices[0]
			if t.index < 0 || t.index >= len(v.Elements) {
				continue
			}
			// One of the small notes leaning on it. Here rather than in
			// each command that might want one: everything that works on
			// pitches comes through notesOf — the accidentals, the
			// brackets round them, moving a notehead, transposing — so a
			// grace note understood here is a grace note those all reach,
			// and none of them has to learn what one is.
			if t.isGrace() {
				mk := marksOfElement(v.Elements[t.index])
				if mk == nil || mk.Grace == nil ||
					t.graceOrd() >= len(mk.Grace.Notes) {
					continue
				}
				out = append(out, keyedNote{
					Note: mk.Grace.Notes[t.graceOrd()], key: p.Key,
					part: p, measure: t.measure, index: t.index,
				})
				continue
			}
			collect(p, t.measure, t.index, v.Elements[t.index], head)
		}
		if len(out) == 0 {
			if head != nil {
				return nil, editError("there is no notehead there")
			}
			return nil, editError("there is no note in the selection")
		}
		return out, nil
	}

	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}
	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		return nil, err
	}
	v := m.Voices[0]
	if len(v.Elements) == 0 {
		return nil, editError("the measure is empty")
	}
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	collect(p, measIdx, at-1, v.Elements[at-1], head)
	if len(out) == 0 {
		if head != nil {
			return nil, editError("there is no notehead there")
		}
		return nil, editError("that is not a note")
	}
	return out, nil
}

func (a *App) handleStem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	stem, ok := stemByName[q.Get("stem")]
	if !ok {
		http.Error(w, "stem must be up, down or auto", http.StatusBadRequest)
		return
	}
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, m := range marks {
		m.Stem = stem
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleDynamic writes, or removes, a dynamic at the cursor.
func (a *App) handleDynamic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("dynamic")

	var d model.Dynamic
	if name != "" && name != "none" {
		v, ok := model.DynamicFromName(name)
		if !ok {
			http.Error(w, "unknown dynamic "+name, http.StatusBadRequest)
			return
		}
		d = v
	}
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, m := range marks {
		m.Dynamic = d
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// target names one element: a staff, a measure and a position within it.
type target struct {
	part, measure, index int
	// graceAt names one of the small notes leaning on that element: zero
	// for the element itself, and otherwise one more than the grace note's
	// place in the group.
	//
	// A fourth number, because three cannot say it. A grace note is not an
	// element of a voice — it hangs off the note it leans on, since it takes
	// no time out of the bar — so there is no index that reaches it, and
	// until there was one a grace note could be written and then never
	// touched: not clicked, not stepped on to, not deleted by itself.
	//
	// Counted from one so that the zero value is the element and not its
	// first grace note. Targets are made in a dozen places and every one of
	// them would otherwise have to remember to say "not a grace note"; the
	// one that forgot would quietly aim a command at an ornament. A field
	// whose zero value is the common case cannot be forgotten.
	graceAt int
}

// isGrace says the target names a small note rather than the element it
// leans on, and graceOrd is which one, counting from zero.
func (t target) isGrace() bool { return t.graceAt > 0 }
func (t target) graceOrd() int { return t.graceAt - 1 }

// parseTargets reads the sel parameter, which is a list of
// staff:measure:index triples. It is how a command reaches everything inside
// a dragged rectangle rather than only the note under the cursor.
//
// A fourth number names a grace note on that element: "0:3:2:1" is the
// second small note leaning on the third element of bar 3. Three numbers go
// on meaning what they always meant, so nothing written before this knows
// the difference.
func parseTargets(sel string) ([]target, error) {
	var out []target
	for _, piece := range strings.Split(sel, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		f := strings.Split(piece, ":")
		if len(f) != 3 && len(f) != 4 {
			return nil, editError("a selection looks like staff:measure:index")
		}
		var t target
		var err error
		if t.part, err = strconv.Atoi(f[0]); err != nil {
			return nil, editError("bad staff in the selection")
		}
		if t.measure, err = strconv.Atoi(f[1]); err != nil {
			return nil, editError("bad measure in the selection")
		}
		if t.index, err = strconv.Atoi(f[2]); err != nil {
			return nil, editError("bad position in the selection")
		}
		if len(f) == 4 {
			ord, err := strconv.Atoi(f[3])
			if err != nil || ord < 0 {
				return nil, editError("bad grace note in the selection")
			}
			t.graceAt = ord + 1
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, editError("nothing selected")
	}
	return out, nil
}

// graceNoteAt is the small note a target names, with the group it belongs to.
func (a *App) graceNoteAt(t target) (*model.GraceGroup, *model.Note, error) {
	_, m, err := a.locate(t.part, t.measure)
	if err != nil {
		return nil, nil, err
	}
	v := m.Voices[0]
	if t.index < 0 || t.index >= len(v.Elements) {
		return nil, nil, editError("there is no note there")
	}
	mk := marksOfElement(v.Elements[t.index])
	if mk == nil || mk.Grace == nil || t.graceOrd() >= len(mk.Grace.Notes) {
		return nil, nil, editError("there is no grace note there")
	}
	return mk.Grace, mk.Grace.Notes[t.graceOrd()], nil
}

// marksOf returns the marks of every element a command should act on: the
// whole selection when one was dragged, otherwise the element at the cursor.
func (a *App) marksOf(q url.Values) ([]*model.Marks, error) {
	if sel := q.Get("sel"); sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			return nil, err
		}
		a.lock()
		defer a.mu.Unlock()

		var out []*model.Marks
		for _, t := range targets {
			if name, ok := a.voicedHere(t.part); ok {
				return nil, errVoicedElsewhere(name)
			}
			_, m, err := a.locate(t.part, t.measure)
			if err != nil {
				continue // a stale selection is not worth failing over
			}
			v := m.Voices[0]
			if t.index < 0 || t.index >= len(v.Elements) {
				continue
			}
			if t.isGrace() {
				// A mark aimed at a grace note is not a mark for the note
				// it leans on. Handing back the carrier's marks would put
				// the staccato, or the dynamic, or the tie on the wrong
				// note and say it had worked — so the command refuses, and
				// the ones that do mean something on a small note reach it
				// through graceNoteAt instead.
				return nil, editError("that mark cannot be written on a grace note")
			}
			if mk := marksOfElement(v.Elements[t.index]); mk != nil {
				out = append(out, mk)
			}
		}
		if len(out) == 0 {
			return nil, editError("nothing in the selection can be marked")
		}
		return out, nil
	}

	one, err := a.markAt(q)
	if err != nil {
		return nil, err
	}
	return []*model.Marks{one}, nil
}

func marksOfElement(el model.Element) *model.Marks {
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

// markAt finds the marks of the element the cursor sits after, which is the
// one just placed or just clicked.
func (a *App) markAt(q url.Values) (*model.Marks, error) {
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}

	a.lock()
	defer a.mu.Unlock()

	_, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		return nil, err
	}
	v := m.Voices[0]
	if len(v.Elements) == 0 {
		return nil, editError("the measure is empty")
	}
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	switch el := v.Elements[at-1].(type) {
	case *model.Note:
		return &el.Marks, nil
	case *model.Chord:
		return &el.Marks, nil
	case *model.Rest:
		return &el.Marks, nil
	}
	return nil, editError("nothing there to mark")
}

var articulationByName = map[string]model.Articulation{
	"staccato": model.Staccato, "staccatissimo": model.Staccatissimo,
	"accent": model.Accent, "tenuto": model.Tenuto,
	"marcato": model.Marcato, "marcato down": model.MarcatoDown,
	"fermata": model.Fermata, "up-bow": model.UpBow, "down-bow": model.DownBow,
}

// handleArticulation adds a mark to the element at the cursor, or clears them
// all when none is named.
func (a *App) handleArticulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("articulation")

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if name == "" {
		for _, m := range marks {
			m.Articulations = nil
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	}
	art, ok := articulationByName[name]
	if !ok {
		http.Error(w, "unknown articulation "+name, http.StatusBadRequest)
		return
	}

	// Which side of the note it is written on, where somebody has said.
	// Left alone the engraver decides: away from the stem for most of
	// them, and over the staff for the ones that always go there.
	place, named := model.PlacementAuto, false
	switch q.Get("placement") {
	case "above":
		place, named = model.PlacementAbove, true
	case "below":
		place, named = model.PlacementBelow, true
	case "auto":
		named = true
	}

	// Taking one off rather than putting it on: a mark picked up by
	// clicking it is removed by name, and the others on the note stay.
	if q.Get("remove") == "1" {
		gone := 0
		for _, m := range marks {
			if m.RemoveArticulation(art) {
				gone++
			}
		}
		if gone == 0 {
			http.Error(w, "there is no "+name+" there", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"changed": gone})
		return
	}

	for _, m := range marks {
		m.AddArticulation(art)
		if named {
			m.PlaceArticulation(art, place)
		}
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// clefBefore is what this staff reads in at the bar before this one, which
// is what a clef written here would be a change from.
func (a *App) clefBefore(p *model.Part, at int) model.Clef {
	c := p.Clef
	for i := 0; i < at && i < len(p.Measures); i++ {
		if m := p.Measures[i].Clef; m != nil {
			c = *m
		}
	}
	return c
}

// handleStaff changes a property of a staff itself: its clef, its meter, its
// instrument. These belong to the staff rather than to any note on it, which
// is why the palette applies them without needing a note selected.
func (a *App) handleStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))

	a.lock()
	defer a.mu.Unlock()
	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[partIdx]

	changed := []string{}
	// How many lines the staff is ruled with. One is the ordinary way to
	// write a drum that has no pitch; five is every other staff, and is
	// what "5" or "0" asks for.
	if v := q.Get("lines"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 5 {
			http.Error(w, "a staff has one to five lines, not "+v,
				http.StatusBadRequest)
			return
		}
		if n == 5 {
			n = 0
		}
		p.StaffLines = n
		changed = append(changed, "lines")
	}
	if v := q.Get("clef"); v != "" {
		// A clef at a bar past the first changes clef there and leaves
		// what came before alone, the way a key or a meter does: a
		// cellist climbing into the tenor clef for a passage and dropping
		// back, a pianist's left hand going up into the treble for a
		// page. The model has carried these since the importers started
		// reading them, and there was no way to write one by hand — a
		// staff was one clef from end to end.
		at, hasAt := strconv.Atoi(q.Get("at"))
		switch {
		case v == "none":
			if hasAt != nil || at <= 0 {
				http.Error(w,
					"there is no clef change at the start to remove",
					http.StatusBadRequest)
				return
			}
			if at >= len(p.Measures) || p.Measures[at].Clef == nil {
				http.Error(w, "there is no clef change at that measure",
					http.StatusBadRequest)
				return
			}
			p.Measures[at].Clef = nil
		default:
			c, ok := clefByStringName[v]
			if !ok {
				http.Error(w, "unknown clef "+v, http.StatusBadRequest)
				return
			}
			if hasAt == nil && at > 0 {
				if at >= len(p.Measures) {
					http.Error(w, "no such measure", http.StatusBadRequest)
					return
				}
				// Only where it is a change. A clef written again where
				// the staff already reads in it is a sign that says
				// nothing, and the page has one drawn on it all the same.
				if a.clefBefore(p, at) == c {
					p.Measures[at].Clef = nil
				} else {
					clef := c
					p.Measures[at].Clef = &clef
				}
			} else {
				p.Clef = c
			}
		}
		changed = append(changed, "clef")
	}
	if v := q.Get("time"); v == "none" {
		// Taking a meter change away, the same way a key change can be.
		at, err := strconv.Atoi(q.Get("at"))
		if err != nil || at <= 0 {
			http.Error(w, "there is no meter change at the start to remove",
				http.StatusBadRequest)
			return
		}
		removed := 0
		for _, part := range a.score.Parts {
			if at < len(part.Measures) && part.Measures[at].Time != nil {
				part.Measures[at].Time = nil
				removed++
			}
		}
		if removed == 0 {
			http.Error(w, "there is no meter change at that measure",
				http.StatusBadRequest)
			return
		}
		changed = append(changed, "time change removed")
	} else if v := q.Get("time"); v != "" {
		t, err := parseMeter(v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// The meter belongs to the music, not to one staff of it: everybody
		// is counting the same bars. A score with 4/4 on one staff and 2/4 on
		// the next does not mean anything, so this goes on every part.
		at, hasAt := strconv.Atoi(q.Get("at"))
		for _, part := range a.score.Parts {
			if hasAt == nil && at > 0 {
				measureAt(part, at).Time = &t
			} else {
				part.Time = t
				if len(part.Measures) > 0 {
					part.Measures[0].Time = nil
				}
			}
		}
		if hasAt == nil && at > 0 {
			changed = append(changed, "time change")
		} else {
			changed = append(changed, "time")
		}
	}
	// A sound names the instrument itself — "bass clarinet", not "a
	// clarinet program" — and is what the catalogue offers.
	if v := q.Get("sound"); v != "" {
		if !applySound(p, v) {
			http.Error(w, "unknown instrument "+v, http.StatusBadRequest)
			return
		}
		// A staff that has never been called anything takes the name of what
		// it has just been told it is. A score read out of a PDF arrives as
		// a stack of nameless staves, and naming them one by one after
		// saying what they are would be saying it twice.
		//
		// Only where there is no name. A staff somebody called "Solo" is
		// still called "Solo" when they give it a flute, and a name quietly
		// replaced is a name lost.
		if p.Name == "" {
			if n, ok := model.SoundName(v); ok {
				// Capitalised the same way the staves made from scratch
				// are, so a staff named this way and a staff named by
				// Score setup are not spelt differently.
				p.Name = capitalised(n)
			}
		}
		changed = append(changed, "instrument")
		if q.Get("transpose") != "0" {
			want, _ := model.SoundTransposition(v)
			if want != p.Transpose {
				retranspose(p, p.Transpose, want)
				p.Transpose = want
				if !want.IsZero() {
					changed = append(changed, "transposition")
				}
			}
		}
	} else if v := q.Get("instrument"); isDrumKit(v) {
		p.Clef = model.Percussion
		// A staff of drums is in no key: its lines carry no pitches.
		p.Key = model.KeySignature{}
		p.Transpose = model.Transposition{}
		changed = append(changed, "drum kit")
	} else if v != "" {
		inst, ok := model.InstrumentFromName(v)
		if !ok {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 127 {
				http.Error(w, "unknown instrument "+v, http.StatusBadRequest)
				return
			}
			inst = model.Instrument(n)
		}
		p.Instrument = inst
		changed = append(changed, "instrument")

		// An instrument brings its transposition with it, and changing it has
		// to leave the music sounding as it did: what is printed moves, so
		// that what is heard does not. Choosing "alto sax" on a part in C
		// rewrites it in A, three sharps, a major sixth higher — which is
		// what the player needs to read to produce the same notes.
		if q.Get("transpose") != "0" {
			want := model.TranspositionFor(inst)
			if want != p.Transpose {
				retranspose(p, p.Transpose, want)
				p.Transpose = want
				if !want.IsZero() {
					changed = append(changed, "transposition")
				}
			}
		}
	}
	// Written as tablature, or back on a staff of pitches.
	//
	// Not a change to the music. The notes keep their pitches — that is
	// what sounds and what transposes — and this is how they are shown,
	// so a staff turned to tab and back is the staff it was.
	if v, ok := q["tab"]; ok && len(v) > 0 {
		switch name := strings.TrimSpace(v[0]); {
		case name == "" || name == "off":
			p.Tab = nil
			changed = append(changed, "staff of pitches")
		default:
			tab, found := model.TuningNamed(name)
			if !found {
				http.Error(w, "no tuning called "+name+" — /api/tunings lists them",
					http.StatusBadRequest)
				return
			}
			p.Tab = tab
			// Where the notes are played, for the ones that did not say.
			// A file that recorded a string keeps it: which string to
			// play a note on is a decision somebody made.
			restringPart(p)
			changed = append(changed, "tablature")
		}
	}
	if v := q.Get("hidden"); v != "" {
		// A decision about the page, not about the music: a hidden staff
		// still sounds, so a conductor can take the rest of the orchestra
		// off the screen and keep hearing it.
		p.Hidden = v == "1"
		changed = append(changed, "hidden")
	}
	if v := q.Get("name"); v != "" {
		p.Name = strings.TrimSpace(v)
		changed = append(changed, "name")
	}
	if len(changed) == 0 {
		http.Error(w, "nothing to change", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "changed": changed})
}

// parseMeter reads "4/4", or "4/4c" and "2/2c" for the two that have a symbol
// of their own.
func parseMeter(v string) (model.TimeSignature, error) {
	v = strings.TrimSpace(v)

	// The two meters with glyphs of their own are usually written as those
	// glyphs: somebody asked for common time by typing "C", which is what it
	// is called and what is printed, and got told it did not look like 4/4.
	switch strings.ToLower(v) {
	case "c", "common":
		return model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeCommon}, nil
	case "¢", "c|", "cut", "alla breve":
		return model.TimeSignature{Beats: 2, BeatValue: 2, Symbol: model.TimeCut}, nil
	}

	sym := model.TimeNumeric
	if strings.HasSuffix(strings.ToLower(v), "c") {
		v = v[:len(v)-1]
		sym = model.TimeCommon
		if v == "2/2" {
			sym = model.TimeCut
		}
	}
	n, d, ok := strings.Cut(v, "/")
	if !ok {
		return model.TimeSignature{}, editError(
			`a meter looks like 4/4, or C for common time and C| for cut`)
	}
	beats, err1 := strconv.Atoi(n)
	value, err2 := strconv.Atoi(d)
	if err1 != nil || err2 != nil || beats <= 0 || value <= 0 {
		return model.TimeSignature{}, editError(
			`a meter looks like 4/4, or C for common time and C| for cut`)
	}
	return model.TimeSignature{Beats: beats, BeatValue: value, Symbol: sym}, nil
}

// handleTempo sets the metronome mark.
func (a *App) handleTempo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	// Which bar it starts at. Bar one is the score's own tempo — the mark in
	// the title block is that same marking written over the first bar — so
	// saying nothing, and saying the first bar, are the same thing. Anywhere
	// else is a change written over that bar and held until the next one.
	bar := 0
	if v := q.Get("measure"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			http.Error(w, "which bar?", http.StatusBadRequest)
			return
		}
		bar = n
	}

	// Taking a change away is not setting one: the bar goes back to carrying
	// on at whatever speed it was given earlier.
	if q.Get("remove") == "1" {
		if bar == 0 {
			http.Error(w, "the piece has a speed whether or not anybody chose it",
				http.StatusBadRequest)
			return
		}
		a.lock()
		defer a.mu.Unlock()
		for _, p := range a.score.Parts {
			if bar < len(p.Measures) {
				p.Measures[bar].Tempo = nil
			}
		}
		writeJSON(w, map[string]any{"status": "ok", "measure": bar})
		return
	}

	bpm, err := strconv.Atoi(q.Get("bpm"))
	if err != nil || bpm < 10 || bpm > 400 {
		http.Error(w, "a tempo is between 10 and 400 beats a minute",
			http.StatusBadRequest)
		return
	}

	// The beat the number counts. A tempo of 120 means nothing without it.
	unit := model.Duration{Num: 1, Den: 4}
	if v := q.Get("unit"); v != "" {
		d, ok := durationNames[v]
		if !ok {
			http.Error(w, "unknown beat unit "+v, http.StatusBadRequest)
			return
		}
		unit = d
	}
	if q.Get("dotted") == "1" {
		unit = unit.Scale(3, 2)
	}

	t := model.Tempo{
		BPM: bpm, Unit: unit, Text: q.Get("text"),
		Parens: q.Get("parens") == "1",
	}

	a.lock()
	defer a.mu.Unlock()
	if bar == 0 {
		a.score.Tempo = t
		// A change written over bar one would print the same marking twice.
		for _, p := range a.score.Parts {
			if len(p.Measures) > 0 {
				p.Measures[0].Tempo = nil
			}
		}
		writeJSON(w, map[string]any{"status": "ok", "bpm": bpm, "measure": 0})
		return
	}
	if bar >= a.score.MeasureCount() {
		http.Error(w, "there is no such bar", http.StatusBadRequest)
		return
	}
	// On the first staff, because that is where a tempo is printed and who
	// it belongs to: it is the piece's speed, not one player's.
	for i, p := range a.score.Parts {
		if bar >= len(p.Measures) {
			continue
		}
		if i == 0 {
			p.Measures[bar].Tempo = &t
		} else {
			p.Measures[bar].Tempo = nil
		}
	}
	writeJSON(w, map[string]any{"status": "ok", "bpm": bpm, "measure": bar})
}

// handleHeader sets the title block.
func (a *App) handleHeader(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()
	// Only what was sent is changed, so a dialog can set one field alone.
	if q.Has("title") {
		a.score.Identity.Title = q.Get("title")
	}
	if q.Has("subtitle") {
		a.score.Identity.Subtitle = q.Get("subtitle")
	}
	if q.Has("composer") {
		a.score.Identity.Composer = q.Get("composer")
	}
	if q.Has("arranger") {
		a.score.Identity.Arranger = q.Get("arranger")
	}
	if q.Has("copyright") {
		a.score.Identity.Copyright = q.Get("copyright")
	}
	// And how those four are set, where the window that sent them knows.
	// The words and their formatting arrive together or not at all: a
	// caller that sends a field and no runs for it is saying the field is
	// plain, which is what clearing them says.
	for _, f := range []struct {
		name string
		into *[]model.Span
	}{
		{"title", &a.score.Identity.TitleRuns},
		{"subtitle", &a.score.Identity.SubtitleRuns},
		{"composer", &a.score.Identity.ComposerRuns},
		{"arranger", &a.score.Identity.ArrangerRuns},
	} {
		if !q.Has(f.name) {
			continue
		}
		*f.into = nil
		raw := q.Get(f.name + "Runs")
		if raw == "" {
			continue
		}
		var runs []model.Span
		if err := json.Unmarshal([]byte(raw), &runs); err != nil {
			http.Error(w, "the formatting of the "+f.name+" is not readable",
				http.StatusBadRequest)
			return
		}
		// Formatting that does not spell the words it is formatting is not
		// formatting of them. Refused here rather than at the far end,
		// where all that can be done about it is to ignore it silently.
		if model.PlainSpans(runs) != q.Get(f.name) {
			http.Error(w, "the formatting of the "+f.name+
				" does not spell the "+f.name, http.StatusBadRequest)
			return
		}
		*f.into = runs
	}
	// Tags come as one line, the way somebody types them, and are split
	// here rather than in the editor: a shelf built from the files has to
	// agree with the window about where one tag ends and the next begins,
	// and the only way to be sure of that is for one of them to decide.
	if q.Has("description") {
		a.score.Description = q.Get("description")
	}
	if q.Has("tags") {
		a.score.Tags = parseTags(q.Get("tags"))
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// parseTags reads a line of tags the way somebody writes one.
//
// The rules live with the model, because the command line puts tags on the
// same files this window does and the two have to agree about where one tag
// ends and the next begins.
func parseTags(line string) []string { return model.ParseTags(line) }

// handleInstruments lists what a staff can be set to sound as.
// handleInstruments is the list somebody picks an instrument from.
//
// The catalogue, clustered by what the thing is, and not the hundred and
// twenty-eight General MIDI programs in numerical order. The difference
// matters because the question being asked is "what is this piece for",
// and a flat list of programs cannot answer it: General MIDI has no bass
// clarinet, no cornet and one entry covering every guitar ever made.
//
// Each entry carries the sound id as well as the name, and that is what the
// editor sends back. A name is for reading; the id is what says which
// instrument this is, in the spelling MusicXML, MuseScore, Dorico and
// Finale all use, so a part chosen here survives being exported.
//
// ?flat=1 gives the old answer — the named General MIDI programs, as a flat
// list — for anything that was reading this endpoint before the catalogue
// existed.
func (a *App) handleInstruments(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Program int    `json:"program"`
		Name    string `json:"name"`
		Family  string `json:"family"`
		// Percussion marks an entry that is not a General MIDI program at
		// all. Choosing it gives a staff whose lines name drums.
		Percussion bool `json:"percussion,omitempty"`
	}

	if r.URL.Query().Get("flat") == "1" {
		// A drum kit belongs in the list people choose an instrument from,
		// because that is where somebody looks for it. It is not a program
		// number: General MIDI puts drums on their own channel rather than
		// giving them one, so nothing of the sort is written into the model
		// — what makes a staff a drum staff is its clef, and choosing this
		// sets that.
		out := []entry{{Program: -1, Name: drumKitName, Family: "percussion", Percussion: true}}
		for p := 0; p < 128; p++ {
			i := model.Instrument(p)
			n := i.String()
			// Only the ones with a name of their own; the rest are
			// "program 57" and nobody is choosing those from a list.
			if strings.HasPrefix(n, "program ") {
				continue
			}
			out = append(out, entry{Program: p, Name: n, Family: i.Family().String()})
		}
		writeJSON(w, out)
		return
	}

	type item struct {
		Group      string `json:"group"`
		Name       string `json:"name"`
		Sound      string `json:"sound"`
		Program    int    `json:"program"`
		Percussion bool   `json:"percussion,omitempty"`
		// Low and High are the written compass, as MIDI note numbers,
		// where gscore knows it. A section — "strings", "brass section" —
		// has no one range and claims none.
		Low  int `json:"low,omitempty"`
		High int `json:"high,omitempty"`
		// ScoreRank is where it goes down the page of a score, which
		// is not the order this list is in: the list is ordered for
		// finding an instrument and a score for reading one.
		ScoreRank int `json:"scoreRank"`
		// Written is how the staff is laid out — a staff of pitches, a
		// tablature, or both — and Tuning which tuning a tablature is
		// in. Only a fretted instrument is offered more than one way,
		// and the editor has to be told which way an entry is or every
		// one of them asks for a plain staff.
		Written string `json:"written,omitempty"`
		Tuning  string `json:"tuning,omitempty"`
		// Grand says this instrument is normally written on two staves
		// joined by a brace — a piano's two hands, a harp's. The editor
		// needs it to know a grand staff when it is looking at one: two
		// staves of a piano are one player and one entry wherever the
		// instruments are listed, where two staves of cello are two
		// players and two entries.
		Grand bool `json:"grand,omitempty"`
	}
	out := make([]item, 0, 320)
	for _, c := range model.Catalogue() {
		out = append(out, item{
			Group: c.Group, Name: c.Name, Sound: c.Sound,
			Program: int(c.Program), Percussion: c.Percussion,
			Low: c.Low, High: c.High, ScoreRank: c.ScoreRank,
			Written: c.Written, Tuning: c.Tuning,
			Grand: model.GrandStaff(c.Sound),
		})
	}
	writeJSON(w, struct {
		Groups      []string `json:"groups"`
		Instruments []item   `json:"instruments"`
	}{model.Groups, out})
}

// retranspose rewrites a part from one transposition to another, keeping what
// it sounds like.
//
// The written pitches move by the difference between the two intervals, and so
// does every key signature on the staff — including the changes written in the
// middle of it, which are just as much part of what the player reads.
func retranspose(p *model.Part, from, to model.Transposition) {
	steps := from.Steps() - to.Steps()
	semis := from.Semitones() - to.Semitones()
	if steps == 0 && semis == 0 {
		return
	}

	// The signature moves the way the notes do. A part in C read by an alto
	// saxophone is written a major sixth up and so in three more sharps, and
	// the sign here was the other way round: the notes went up and the
	// signature went down after them, which put the part in the wrong key
	// and an accidental on nearly every note of it.
	shiftKey := func(k model.KeySignature) model.KeySignature {
		out := k
		out.Fifths = k.Fifths + from.Fifths() - to.Fifths()
		for out.Fifths > 7 {
			out.Fifths -= 12
		}
		for out.Fifths < -7 {
			out.Fifths += 12
		}
		return out
	}
	p.Key = shiftKey(p.Key)

	for _, m := range p.Measures {
		if m.Key != nil {
			k := shiftKey(*m.Key)
			m.Key = &k
		}
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				switch n := el.(type) {
				case *model.Note:
					movePitch(n, steps, semis)
				case *model.Chord:
					for _, cn := range n.Notes {
						movePitch(cn, steps, semis)
					}
				}
			}
		}
	}
}

// movePitch transposes one note, keeping an accidental it was written with.
func movePitch(n *model.Note, steps, semis int) {
	before := n.Accidental
	n.Pitch = n.Pitch.Transpose(steps, semis)
	if before == model.NoAccidental {
		return
	}
	// The note was spelled with an accidental, so the transposed one is too,
	// whatever it turns out to be.
	switch n.Pitch.Alter {
	case 2:
		n.Accidental = model.DoubleSharp
	case 1:
		n.Accidental = model.Sharp
	case 0:
		n.Accidental = model.Natural
	case -1:
		n.Accidental = model.Flat
	case -2:
		n.Accidental = model.DoubleFlat
	}
}

// handleHairpin writes a crescendo or diminuendo over the selection.
//
// A hairpin is not a property of a note, it is a stretch of music: it needs a
// beginning and an end, which is exactly what a range selection is. One note
// selected gets a hairpin that opens and closes on it, which is how it is
// taken off again.
func (a *App) handleHairpin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	kind := model.Crescendo
	switch q.Get("hairpin") {
	case "crescendo", "cresc", "<":
	case "diminuendo", "dim", "decrescendo", ">":
		kind = model.Diminuendo
	case "none", "":
		marks, err := a.marksOf(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, m := range marks {
			m.Hairpin = nil
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	default:
		http.Error(w, "a hairpin is a crescendo or a diminuendo", http.StatusBadRequest)
		return
	}

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// One note is a hairpin on that note: a swell under a long note, drawn
	// across the room the note takes. It used to be refused — "select a
	// passage" — on the grounds that a wedge from a note to itself has no
	// width, which is true of the drawing and not of the music: somebody
	// marking one whole note had to pick up the note after it as well, and
	// got a hairpin reaching into the next bar.
	if len(marks) == 1 {
		marks[0].Hairpin = &model.Hairpin{Kind: kind, Start: true, Stop: true, Alone: true}
		writeJSON(w, map[string]int{"changed": 1})
		return
	}
	for i, m := range marks {
		h := &model.Hairpin{
			Kind:  kind,
			Start: i == 0,
			Stop:  i == len(marks)-1,
		}
		// Where this one begins, another may end — a diminuendo running
		// into a crescendo, which every edition writes point to point. The
		// end that is already there stays: taken away, as it was, the first
		// hairpin had no end left and ran on through the second to
		// wherever it found one.
		//
		// And the other way round at the far end, where this one ends on a
		// note another begins at. The kind belongs to whichever starts
		// there, because that is the one a kind is read for.
		if was := m.Hairpin; was != nil && !was.Alone {
			if h.Start && was.Stop && !was.Start {
				h.Stop = true
			}
			if h.Stop && was.Start && !was.Stop {
				h.Start, h.Kind = true, was.Kind
			}
		}
		m.Hairpin = h
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleOctave writes an 8va or 8vb line over the selection.
func (a *App) handleOctave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	amount := 0
	switch q.Get("octave") {
	case "8va", "up", "1":
		amount = 1
	case "8vb", "down", "-1":
		amount = -1
	case "15ma", "2":
		amount = 2
	case "15mb", "-2":
		amount = -2
	case "none", "":
		for _, m := range marks {
			m.Octave = nil
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	default:
		http.Error(w, "an octave line is 8va, 8vb, 15ma or 15mb", http.StatusBadRequest)
		return
	}

	for i, m := range marks {
		m.Octave = &model.OctaveShift{
			Amount: amount,
			Start:  i == 0,
			Stop:   i == len(marks)-1,
		}
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleEnding puts a volta bracket over a measure, on every staff.
//
// A repeat belongs to the piece: all the players go back at the same bar and
// all of them take the second ending the second time. So this is set on every
// part at once, whichever staff the selection happened to be on.
func (a *App) handleEnding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	first, _ := strconv.Atoi(q.Get("measure"))
	last := first
	if v, err := strconv.Atoi(q.Get("to")); err == nil && v > first {
		last = v
	}

	var numbers []int
	for _, f := range strings.Split(q.Get("numbers"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			numbers = append(numbers, n)
		}
	}
	remove := q.Get("ending") == "none"
	if !remove && len(numbers) == 0 {
		numbers = []int{1}
	}
	open := q.Get("open") == "1"

	a.lock()
	defer a.mu.Unlock()

	changed := 0
	for _, p := range a.score.Parts {
		for mi := first; mi <= last; mi++ {
			if mi < 0 || mi >= len(p.Measures) {
				continue
			}
			if remove {
				p.Measures[mi].Ending = nil
			} else {
				p.Measures[mi].Ending = &model.Ending{
					Numbers: numbers,
					Start:   mi == first,
					Stop:    mi == last,
					Open:    open,
				}
			}
			changed++
		}
	}
	if changed == 0 {
		http.Error(w, "there is no such measure", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": changed})
}

// handleJump writes a sign or a direction at a bar, on every staff.
//
// Like a repeat, these belong to the piece and not to a player: everybody
// goes back at the same bar, and everybody stops at the same Fine. Writing
// them on one staff would mean a score where the flutes take the D.S. and
// the horns do not.
func (a *App) handleJump(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	at, _ := strconv.Atoi(q.Get("measure"))

	// A sign, a direction, or both: a bar can carry the coda sign and a
	// D.S. al Coda at once, and asking for them in two calls would record
	// two steps of undo for one thing done.
	var sign model.Sign
	setSign := false
	if v := q.Get("sign"); v != "" {
		s, ok := model.SignFromName(v)
		if !ok {
			http.Error(w, "no such sign: "+v, http.StatusBadRequest)
			return
		}
		sign, setSign = s, true
	}
	var jump *model.Jump
	setJump := false
	if v := q.Get("jump"); v != "" {
		j, ok := model.JumpFromName(v)
		if !ok {
			http.Error(w, "no such direction: "+v, http.StatusBadRequest)
			return
		}
		jump, setJump = j, true
	}
	if !setSign && !setJump {
		http.Error(w, "say which sign or direction to write", http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()

	changed := 0
	for _, p := range a.score.Parts {
		if at < 0 || at >= len(p.Measures) {
			continue
		}
		if setSign {
			p.Measures[at].Sign = sign
		}
		if setJump {
			p.Measures[at].Jump = jump
		}
		changed++
	}
	if changed == 0 {
		http.Error(w, "there is no such measure", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": changed})
}

/*
 * Moving, copying and pasting what is selected
 */

// handleNudge moves the selected notes up or down.
//
// This is the arrow keys: a note is picked and pushed to the right pitch
// rather than deleted and written again. Moving by staff steps rather than by
// semitones is what makes it follow the key — a step up from F in G major is
// G, and a step up from F sharp is G as well.
func (a *App) handleNudge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	steps, err := strconv.Atoi(q.Get("steps"))
	if err != nil || steps == 0 {
		http.Error(w, "steps says how far to move", http.StatusBadRequest)
		return
	}

	notes, err := a.notesOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, n := range notes {
		n.Pitch = model.PitchFromDiatonicIndex(n.Pitch.DiatonicIndex() + steps)
		if n.Accidental == model.NoAccidental {
			// No accidental written, so the key decides how it sounds.
			n.Pitch.Alter = n.key.AlterFor(n.Pitch.Step)
		} else {
			n.Pitch.Alter = n.Accidental.Semitones()
		}
	}
	writeJSON(w, map[string]int{"changed": len(notes)})
}

// handleShift moves the selected run one place along the music.
//
// Ctrl or Cmd with the left and right arrows. The plain arrows move the
// selection and leave the notes alone, which is the common thing to want; this
// is the other one. A tuplet, or a shorter note, leaves a bar with room in it
// that was not there before, and the next thing anybody wants is to walk the
// following phrase back into that room.
//
// Only one element actually moves. Shifting a run right is the same thing as
// taking the note after it and putting it in front — the run keeps its own
// order and its own notes, and nothing is copied or rewritten. That is also
// what makes it work on a selection spanning several bars: whatever is
// between its ends is untouched, so there is no need for the run to sit
// inside one bar.
func (a *App) handleShift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	dir, err := strconv.Atoi(q.Get("dir"))
	if err != nil || dir == 0 {
		http.Error(w, "dir says which way to move", http.StatusBadRequest)
		return
	}
	targets, err := parseTargets(q.Get("sel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].measure != targets[j].measure {
			return targets[i].measure < targets[j].measure
		}
		return targets[i].index < targets[j].index
	})
	first, last := targets[0], targets[len(targets)-1]
	if first.part != last.part {
		http.Error(w, "a passage moves within one staff", http.StatusBadRequest)
		return
	}
	p, _, err := a.locate(first.part, first.measure)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The notes being moved, held by identity: the bars reflow underneath
	// them and the places the selection named them by are gone afterwards.
	moving := make([]model.Element, 0, len(targets))
	for _, t := range targets {
		if t.measure < 0 || t.measure >= len(p.Measures) {
			http.Error(w, "the selection is not in the music any more",
				http.StatusBadRequest)
			return
		}
		v := voiceOf(p, t.measure)
		if t.index < 0 || t.index >= len(v.Elements) {
			http.Error(w, "the selection is not in the music any more",
				http.StatusBadRequest)
			return
		}
		moving = append(moving, v.Elements[t.index])
	}

	// The one element that moves: whatever is on the side the run is going.
	var from, to target
	if dir > 0 {
		from, err = nextAlong(p, last)
		to = first
	} else {
		from, err = prevAlong(p, first)
		to = target{part: first.part, measure: last.measure, index: last.index + 1}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fv := voiceOf(p, from.measure)
	el := fv.Elements[from.index]
	// A tuplet is a group and moving one member out of it would leave the
	// figure printed over a note that is somewhere else.
	if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
		http.Error(w, "the note beside the selection is part of a tuplet",
			http.StatusBadRequest)
		return
	}
	fv.Elements = append(fv.Elements[:from.index], fv.Elements[from.index+1:]...)
	// Taking it out shifts everything after it in that bar down one, the
	// place the run is going included.
	if to.measure == from.measure && to.index > from.index {
		to.index--
	}

	tv := voiceOf(p, to.measure)
	if to.index > len(tv.Elements) {
		to.index = len(tv.Elements)
	}
	tail := append([]model.Element{}, tv.Elements[to.index:]...)
	tv.Elements = append(append(tv.Elements[:to.index:to.index], el), tail...)

	// Both bars changed length, and the earlier of the two is where the
	// reflow has to start from.
	start := from.measure
	if to.measure < start {
		start = to.measure
	}
	spill(p, start)
	refill(p, start)

	// Where the notes ended up, so that what was selected still is.
	now := make([]string, 0, len(moving))
	for _, m := range moving {
		measure, at := findElement(p, start, m)
		if at <= 0 {
			continue
		}
		now = append(now, fmt.Sprintf("%d:%d:%d", first.part, measure, at-1))
	}
	writeJSON(w, map[string]any{"status": "ok", "moved": len(moving), "now": now})
}

// refill draws music back into a bar left short, which is spill the other way
// round.
//
// Moving a passage across a barline takes an element out of one bar and puts
// it into the other, so one of the two ends up short however the music is
// read. The order of the notes is right either way — that is what the move
// is about — but with the barlines being tracked a bar that is suddenly a
// beat light looks like the arrow key broke something, when all it did was
// leave the beat on the far side of the line.
//
// Only what fits moves, and only from the bar straight after: this tidies up
// after a move rather than reflowing the piece. spill is still what handles
// the other direction, and the two are called together.
func refill(p *model.Part, from int) {
	if !TrackBarlines() {
		return
	}
	bar := p.Time.BarDuration()
	if bar.IsZero() {
		return
	}
	for i := from; i+1 < len(p.Measures); i++ {
		v := voiceOf(p, i)
		for {
			next := voiceOf(p, i+1)
			if len(next.Elements) == 0 {
				break
			}
			el := next.Elements[0]
			// A group is printed over its own notes, so taking its first
			// member over the barline on its own would put the figure in
			// one bar and the notes it counts in two.
			if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
				break
			}
			if v.Duration().Add(el.Duration()).Cmp(bar) > 0 {
				break
			}
			next.Elements = next.Elements[1:]
			v.Elements = append(v.Elements, el)
		}
		if v.Duration().Cmp(bar) < 0 {
			// Still short, and the bar after it had nothing that would fit:
			// there is nothing further back to draw on.
			return
		}
	}
}

// nextAlong is the element after t in the music, which may be in the bar after.
func nextAlong(p *model.Part, t target) (target, error) {
	if v := voiceOf(p, t.measure); t.index+1 < len(v.Elements) {
		return target{part: t.part, measure: t.measure, index: t.index + 1}, nil
	}
	for i := t.measure + 1; i < len(p.Measures); i++ {
		if len(voiceOf(p, i).Elements) > 0 {
			return target{part: t.part, measure: i, index: 0}, nil
		}
	}
	return target{}, editError("that is the end of the music")
}

// prevAlong is the element before t, which may be in the bar before.
func prevAlong(p *model.Part, t target) (target, error) {
	if t.index > 0 {
		return target{part: t.part, measure: t.measure, index: t.index - 1}, nil
	}
	for i := t.measure - 1; i >= 0; i-- {
		if n := len(voiceOf(p, i).Elements); n > 0 {
			return target{part: t.part, measure: i, index: n - 1}, nil
		}
	}
	return target{}, editError("that is the beginning of the music")
}

// voiceOf is a bar's first voice, made if the bar has none: an empty bar has
// no voice at all until something is written into it.
func voiceOf(p *model.Part, measure int) *model.Voice {
	m := p.Measures[measure]
	if len(m.Voices) == 0 {
		m.Voices = []*model.Voice{{}}
	}
	return m.Voices[0]
}

// clipboard is what was last copied: whole elements, lifted out of the score.
//
// It is kept on the server because that is where the music is. Copying in the
// browser would mean inventing a wire format for a Chord and keeping it in
// step with the model, for no gain — nothing outside gscore reads it.
type clipboard struct {
	elements []model.Element
}

// handleCopy takes a copy of the selection.
func (a *App) handleCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	targets, err := parseTargets(r.URL.Query().Get("sel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()

	var out []model.Element
	for _, t := range targets {
		_, m, err := a.locate(t.part, t.measure)
		if err != nil {
			continue
		}
		v := m.Voices[0]
		if t.index < 0 || t.index >= len(v.Elements) {
			continue
		}
		out = append(out, cloneElement(v.Elements[t.index]))
	}
	if len(out) == 0 {
		http.Error(w, "there is nothing there to copy", http.StatusBadRequest)
		return
	}
	*a.clipboard() = clipboard{elements: out}
	writeJSON(w, map[string]int{"copied": len(out)})
}

// handlePaste puts the copy back at the cursor.
func (a *App) handlePaste(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}

	a.lock()
	defer a.mu.Unlock()

	if len(a.clipboard().elements) == 0 {
		http.Error(w, "nothing has been copied", http.StatusBadRequest)
		return
	}
	// For writing, the same as a note is: pasting into a bar is putting
	// music in it.
	//
	// The editor draws one bar past the end of the music on every staff, so
	// that there is somewhere to click before there is anything to click
	// on, and that bar is made the moment something is written into it.
	// Paste asked for the bar as it stood instead and was told "no such
	// measure" — on the one bar of the piece a reader is most likely to aim
	// at, the empty one at the end, and on the staff just added under the
	// one being copied from.
	p, m, err := a.locateForWriting(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	// Into the bar, not after its rest. See openEmptyBar: a bar holding
	// nothing but its own bar of rest is an empty bar, and it is full, so
	// pasting after the rest carried the music into the bar after it.
	at = openEmptyBar(p, m, measIdx, at)

	// A fresh copy each time, so pasting twice does not give two references
	// to the same notes — changing one would silently change the other.
	clip := a.clipboard()
	add := make([]model.Element, len(clip.elements))
	for i, el := range clip.elements {
		add[i] = cloneElement(el)
	}

	// What is pasted takes the place of what was there, rather than shoving
	// it along. Pasting a bar of a part over another bar should give a bar,
	// not two bars' worth of music in one and everything after it moved; and
	// what is pasted over is what was selected to paste over.
	//
	// insert=1 is the other way, for adding to a bar rather than replacing
	// part of it.
	over := len(add)
	if r.URL.Query().Get("insert") == "1" {
		over = 0
	}
	if at+over > len(v.Elements) {
		over = len(v.Elements) - at
	}
	tail := append([]model.Element{}, v.Elements[at+over:]...)
	v.Elements = append(append(v.Elements[:at:at], add...), tail...)

	// What no longer fits the bar moves into the next one, as when writing.
	spill(p, measIdx)
	measure, idx := findElement(p, measIdx, add[len(add)-1])
	writeJSON(w, map[string]any{
		"pasted": len(add), "measure": measure, "at": idx,
	})
}

// cloneElement copies an element deeply enough that editing the copy cannot
// reach the original: the marks, and a chord's noteheads.
func cloneElement(el model.Element) model.Element {
	switch e := el.(type) {
	case *model.Note:
		n := *e
		n.Marks = cloneMarks(e.Marks)
		return &n
	case *model.Rest:
		rst := *e
		rst.Marks = cloneMarks(e.Marks)
		return &rst
	case *model.Chord:
		c := *e
		c.Marks = cloneMarks(e.Marks)
		c.Notes = make([]*model.Note, len(e.Notes))
		for i, n := range e.Notes {
			cn := *n
			cn.Marks = cloneMarks(n.Marks)
			c.Notes[i] = &cn
		}
		return &c
	case *model.SymbolElement:
		sy := *e
		return &sy
	}
	return el
}

func cloneMarks(m model.Marks) model.Marks {
	out := m
	if m.Articulations != nil {
		out.Articulations = append([]model.Articulation(nil), m.Articulations...)
	}
	if m.Symbols != nil {
		out.Symbols = append([]model.SymbolMark(nil), m.Symbols...)
	}
	if m.Tuplet != nil {
		t := *m.Tuplet
		out.Tuplet = &t
	}
	if m.Hairpin != nil {
		h := *m.Hairpin
		out.Hairpin = &h
	}
	if m.Octave != nil {
		o := *m.Octave
		out.Octave = &o
	}
	return out
}

/*
 * Lyrics
 */

// handleLyrics puts a verse of words under a staff.
//
// The words arrive as one block of text rather than syllable by syllable,
// which is how NoteWorthy Composer asks for them and is far and away the
// fastest way to enter a song: you type the verse as you would write it, and
// it is handed out to the notes in order. Hyphens break a word across notes
// and an underscore holds the last syllable over the next one — the
// conventions every notation program shares, because they are the ones
// singers' copies already use.
//
//	Glo-ri-a in ex-cel-sis De-o
//	A-men_ _
//
// Rests take no words. A syllable landing on a tied note goes on the note the
// syllable is actually sung on, which is the first of the tie.
func (a *App) handleLyrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	verse, err := strconv.Atoi(q.Get("verse"))
	if err != nil || verse < 1 {
		verse = 1
	}

	a.lock()
	defer a.mu.Unlock()

	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[partIdx]

	syllables := splitLyrics(q.Get("text"))
	sung := singableNotes(p)

	// Every note of this verse starts clear, so retyping the words replaces
	// them rather than leaving the tail of a longer version behind.
	for _, mk := range sung {
		out := mk.Lyrics[:0]
		for _, ly := range mk.Lyrics {
			if ly.Verse != verse {
				out = append(out, ly)
			}
		}
		mk.Lyrics = out
	}

	n := len(syllables)
	if n > len(sung) {
		n = len(sung)
	}
	for i := 0; i < n; i++ {
		s := syllables[i]
		s.Verse = verse
		sung[i].Lyrics = append(sung[i].Lyrics, s)
	}

	resp := map[string]any{"set": n, "notes": len(sung), "syllables": len(syllables)}
	if len(syllables) > len(sung) {
		resp["warning"] = fmt.Sprintf(
			"there are %d syllables and only %d notes to sing them on",
			len(syllables), len(sung))
	}
	writeJSON(w, resp)
}

// singableNotes lists the marks of everything in a part that a syllable can be
// written under, in reading order.
//
// Rests are left out, and so is the far side of a tie: a tied pair is one
// sound and gets one syllable.
func singableNotes(p *model.Part) []*model.Marks {
	var out []*model.Marks
	for _, m := range p.Measures {
		if len(m.Voices) == 0 {
			continue
		}
		for _, el := range m.Voices[0].Elements {
			mk := marksOfElement(el)
			if mk == nil {
				continue
			}
			switch el.(type) {
			case *model.Note, *model.Chord:
			default:
				continue
			}
			if mk.TieStop && !mk.TieStart {
				continue
			}
			out = append(out, mk)
		}
	}
	return out
}

// splitLyrics turns a typed verse into syllables.
//
// A hyphen inside a word splits it and marks the halves as joined, so the
// engraver can draw the hyphen between them. An underscore on its own holds
// the previous syllable over another note. Line breaks are nothing more than
// spaces: a verse laid out over several lines is still one verse.
func splitLyrics(text string) []model.Lyric {
	var out []model.Lyric
	for _, word := range strings.Fields(text) {
		if word == "_" {
			// Held over: the syllable before it keeps sounding.
			if len(out) > 0 {
				out[len(out)-1].Extend = true
			}
			out = append(out, model.Lyric{Syllabic: model.SyllableSingle, Extend: true})
			continue
		}
		parts := strings.Split(word, "-")
		if len(parts) == 1 {
			out = append(out, model.Lyric{Text: word, Syllabic: model.SyllableSingle})
			continue
		}
		for i, piece := range parts {
			if piece == "" {
				continue // "Glo--ri" is a typo, not an empty syllable
			}
			kind := model.SyllableMiddle
			switch i {
			case 0:
				kind = model.SyllableBegin
			case len(parts) - 1:
				kind = model.SyllableEnd
			}
			out = append(out, model.Lyric{Text: piece, Syllabic: kind})
		}
	}
	return out
}

// handleGetLyrics gives back a verse as typed, so it can be edited rather than
// written again from nothing.
func (a *App) handleGetLyrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	verse, err := strconv.Atoi(q.Get("verse"))
	if err != nil || verse < 1 {
		verse = 1
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[partIdx]

	var b strings.Builder
	verses := 0
	for _, mk := range singableNotes(p) {
		for _, ly := range mk.Lyrics {
			if ly.Verse > verses {
				verses = ly.Verse
			}
			if ly.Verse != verse {
				continue
			}
			if ly.Text == "" {
				b.WriteString(" _")
				continue
			}
			if b.Len() > 0 && !endsHyphenated(b.String()) {
				b.WriteByte(' ')
			}
			b.WriteString(ly.Text)
			if ly.NeedsHyphen() {
				b.WriteByte('-')
			}
		}
	}
	writeJSON(w, map[string]any{
		"text": b.String(), "verse": verse, "verses": verses,
		"notes": len(singableNotes(p)),
	})
}

func endsHyphenated(s string) bool { return strings.HasSuffix(s, "-") }

// handleText writes words beside a note: "rit.", "solo", a reminder.
func (a *App) handleText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(q.Get("text"))

	// What kind of writing this is, which decides how it is set and where
	// it goes. See model.TextStyle: words of one kind coming out alike
	// everywhere is most of what makes a page readable, and is not
	// something to settle one text at a time.
	style, ok := model.TextStyleFromName(q.Get("style"))
	if v := q.Get("style"); v != "" && !ok {
		http.Error(w, "no text style called "+v, http.StatusBadRequest)
		return
	}

	// Left alone, the style decides which side of the staff it goes on.
	place := model.PlacementAuto
	switch q.Get("placement") {
	case "below":
		place = model.PlacementBelow
	case "above":
		place = model.PlacementAbove
	}
	// Which of the note's texts this is about, where the editor is working
	// on one it picked up by clicking it. A note may carry more than one —
	// "pizz." over the staff and "dolce" under it — and changing the one
	// somebody pointed at must leave the other where it is.
	index := -1
	if v := q.Get("index"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			http.Error(w, "index is which of the note's texts", http.StatusBadRequest)
			return
		}
		index = n
	}

	// How the words are set, where the window that sent them knows: the
	// same arrangement the title and the credits use. They arrive with the
	// words they format, and are refused where they spell something else.
	var runs []model.Span
	if raw := q.Get("runs"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &runs); err != nil {
			http.Error(w, "the formatting of the words is not readable",
				http.StatusBadRequest)
			return
		}
		if model.PlainSpans(runs) != text {
			http.Error(w, "the formatting does not spell the words",
				http.StatusBadRequest)
			return
		}
	}

	for _, m := range marks {
		switch {
		case index >= 0 && index < len(m.Texts) && text == "":
			// The one that was pointed at goes, and the rest stay.
			m.Texts = append(m.Texts[:index:index], m.Texts[index+1:]...)
		case text == "":
			m.Texts = nil
		case index >= 0 && index < len(m.Texts):
			m.Texts[index] = model.TextMark{
				Text: text, Style: style, Placement: place, Runs: runs,
				Italic: q.Get("italic") == "1", Bold: q.Get("bold") == "1",
				OffsetX: m.Texts[index].OffsetX, OffsetY: m.Texts[index].OffsetY,
			}
		default:
			m.Texts = append(m.Texts, model.TextMark{
				Text: text, Style: style, Placement: place, Runs: runs,
				Italic: q.Get("italic") == "1", Bold: q.Get("bold") == "1",
			})
		}
		// Words go on one note, not on every note of a passage.
		break
	}
	writeJSON(w, map[string]any{
		"status": "ok", "text": text, "style": style.String(),
	})
}

// handleMarker puts a rehearsal mark on a bar, across the whole score.
func (a *App) handleMarker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	// What the marks in the sequence are called, which is a decision
	// about the whole score rather than about any one bar.
	if v, ok := q["style"]; ok && len(v) > 0 {
		style, good := model.MarkerStyleFromName(strings.TrimSpace(v[0]))
		if !good {
			http.Error(w, v[0]+" is not letters, numbers or bars",
				http.StatusBadRequest)
			return
		}
		a.score.Markers = style
		if _, naming := q["measure"]; !naming {
			a.writeMarks(w, -1)
			return
		}
	}

	measIdx, _ := strconv.Atoi(q.Get("measure"))
	text := strings.TrimSpace(q.Get("text"))
	// auto=1 puts the bar in the sequence and lets its name be worked out
	// from where it falls, which is what keeps the letters in order when
	// one is put in before the others or taken away. A text instead is a
	// mark written out as itself — "1a" at an insert, a word over a
	// chorus — and is left alone by the sequence.
	auto := q.Get("auto") == "1"
	if auto {
		text = ""
	}

	if !a.score.SetMarker(measIdx, text, auto) {
		http.Error(w, "there is no such measure", http.StatusBadRequest)
		return
	}
	a.writeMarks(w, measIdx)
}

// writeMarks answers with the sequence as it now stands.
//
// The whole sequence and not just the bar that changed, because changing
// one changes the others: a mark put in before the rest moves every letter
// after it along, and an editor told only about the bar it touched would
// draw the old letters everywhere else.
func (a *App) writeMarks(w http.ResponseWriter, at int) {
	marks := a.score.RehearsalMarks(engrave.BarNumbers(a.score))
	writeJSON(w, map[string]any{
		"status": "ok", "measure": at, "text": marks[at],
		"style": a.score.Markers.String(), "marks": marks,
	})
}

// handleView changes how the score is drawn rather than what it says.
func (a *App) handleView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	changed := []string{}
	if v := q.Get("names"); v != "" {
		a.engraveOpt.ShowPartNames = v == "1"
		changed = append(changed, "names")
	}
	if v := q.Get("numbers"); v != "" {
		a.engraveOpt.ShowMeasureNumbers = v == "1"
		changed = append(changed, "numbers")
	}
	// Bars of rest counted rather than drawn one by one. Off while writing,
	// because a bar that is not drawn is a bar there is nowhere to click in
	// — but worth having while looking at what will be printed.
	if v := q.Get("multirest"); v != "" {
		a.engraveOpt.MultiRests = v == "1"
		changed = append(changed, "multirest")
	}
	// The metronome mark and the word beside it, over the first bar.
	//
	// Worth taking off as well as putting on: a part being written is
	// read for its notes, and a piece whose speed is obvious from the
	// title does not need "= 120" printed over it. It is a decision
	// about the drawing and not about the music — the tempo is still
	// there, and still what the playback runs at.
	if v := q.Get("tempomark"); v != "" {
		a.engraveOpt.ShowTempoMark = v == "1"
		changed = append(changed, "tempomark")
	}

	// An arrow over every beat of the music: where the pulse falls, rather
	// than how fast it goes.
	//
	// A different question from the one above, which is why it is a
	// different entry. The marking at the head of the score says "crotchet
	// = 58" once and says nothing about where the beats are in any
	// particular bar — and where they are is what somebody counting a tied
	// entry or an odd meter needs. Off unless asked for: it is an aid to
	// reading and not part of the notation.
	if v := q.Get("beatmarks"); v != "" {
		a.engraveOpt.ShowBeatMarks = v == "1"
		changed = append(changed, "beatmarks")
	}

	// Showing the score as it sounds rather than as each player reads it.
	// Not an engraving option: it is a different score on the screen, made
	// by moving every transposing part. See concert.go.
	if v := q.Get("concert"); v != "" {
		a.concert = v == "1"
		changed = append(changed, "concert")
	}

	// Whether the music is drawn as pages or as one continuous strip. The
	// editor has always known this and the server has not, because every
	// drawing request carries it; it is asked for here as well so that the
	// score can remember which way it was being read.
	if v := q.Get("mode"); v != "" {
		if v == "page" {
			a.engraveOpt.Mode = engrave.Paginated
		} else {
			a.engraveOpt.Mode = engrave.Strip
		}
		changed = append(changed, "mode")
	}

	// Whatever has just changed, the score now knows how it is being looked
	// at, and saving it keeps that. See viewstate.go.
	if len(changed) > 0 {
		a.rememberView()
	}
	mode := "strip"
	if a.engraveOpt.Mode == engrave.Paginated {
		mode = "page"
	}
	writeJSON(w, map[string]any{
		"status":    "ok",
		"changed":   changed,
		"mode":      mode,
		"names":     a.engraveOpt.ShowPartNames,
		"numbers":   a.engraveOpt.ShowMeasureNumbers,
		"multirest": a.engraveOpt.MultiRests,
		"tempomark": a.engraveOpt.ShowTempoMark,
		"beatmarks": a.engraveOpt.ShowBeatMarks,
		"concert":   a.concert,
		// Whether the noteheads are coloured for *this* score, which is not
		// the same question as whether the person's own settings colour
		// them: a file may carry a palette of its own. The menu's tick has
		// to say what is on the page in front of it.
		"colourNotes": a.colourNotes(),
	})
}

// handleGroup joins staves with a brace or a bracket.
func (a *App) handleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	first, _ := strconv.Atoi(q.Get("first"))
	last, _ := strconv.Atoi(q.Get("last"))

	a.lock()
	defer a.mu.Unlock()

	if q.Get("symbol") == "none" {
		// Take away whatever group covers these staves.
		var kept []model.StaffGroup
		for _, g := range a.score.Groups {
			if g.First == first && g.Last == last {
				continue
			}
			kept = append(kept, g)
		}
		a.score.Groups = kept
		writeJSON(w, map[string]any{"status": "ok", "groups": len(a.score.Groups)})
		return
	}

	if first < 0 || last >= len(a.score.Parts) || last <= first {
		http.Error(w, "a group joins two staves or more", http.StatusBadRequest)
		return
	}
	symbol := model.GroupBracket
	switch q.Get("symbol") {
	case "brace":
		symbol = model.GroupBrace
	case "line":
		symbol = model.GroupLine
	}
	group := model.StaffGroup{
		First: first, Last: last, Symbol: symbol,
		Barlines: q.Get("barlines") == "1" || symbol == model.GroupBrace,
		Name:     q.Get("name"),
	}
	// Over the one already covering these staves, where there is one.
	// Appending instead drew the second symbol on top of the first, so
	// changing a bracket to a brace left both of them there and the only
	// way out was to take the group away and start again.
	for i, g := range a.score.Groups {
		if g.First == first && g.Last == last {
			a.score.Groups[i] = group
			writeJSON(w, map[string]any{"status": "ok", "groups": len(a.score.Groups)})
			return
		}
	}
	a.score.Groups = append(a.score.Groups, group)
	writeJSON(w, map[string]any{"status": "ok", "groups": len(a.score.Groups)})
}

// handleBeam beams the selection as one group, breaks it apart, or opens a
// new group at the note the cursor is on.
//
// The engraver groups by beat, which is right nearly always. "Nearly" is the
// problem: music phrased against the beat needs the beams drawn against it
// too, and that is a decision only the person writing it can make.
func (a *App) handleBeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch q.Get("beam") {
	case "join", "":
		if len(marks) < 2 {
			http.Error(w, "select two notes or more to beam them together",
				http.StatusBadRequest)
			return
		}
		for i, m := range marks {
			switch {
			case i == 0:
				m.Beam = model.BeamStart
			case i == len(marks)-1:
				m.Beam = model.BeamStop
			default:
				m.Beam = model.BeamContinue
			}
		}
	case "start":
		// The beam is cut to the left of this note and the note stays with
		// what follows it.
		//
		// Breaking at the fifth of eight took the fifth off the beam
		// altogether: it got a flag of its own, and the four after it had to
		// be selected and beamed again by hand to get the two groups that
		// were wanted. A break is a decision about the two groups either
		// side of it, not about one note — so this says where the second
		// group opens, and the engraver closes the first one against it.
		// What follows carries on into the new group, because a group opened
		// by hand is not broken by the beat.
		marks[0].Beam = model.BeamStart
	case "break", "none":
		for _, m := range marks {
			m.Beam = model.BeamNone
		}
	case "auto":
		for _, m := range marks {
			m.Beam = model.BeamAuto
		}
	default:
		http.Error(w, "beam is join, start, break or auto", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleChordSymbol writes a chord name over a note.
//
// The name is given the way a musician writes it — "Em6", "Bb7", "C/G",
// "N.C." — and taken apart here rather than in the browser, so that every
// reader of a .gscore gets the same symbol and the engraver can set the root,
// the quality and the bass note each at its own size.
func (a *App) handleChordSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(q.Get("chord"))
	if text == "" || text == "none" {
		for _, m := range marks {
			m.Chord = nil
		}
		writeJSON(w, map[string]any{"status": "ok", "chord": ""})
		return
	}

	sym := parseChordSymbol(text)
	if q.Get("name") == "0" {
		// The diagram on its own. The name is still asked for, because it
		// is what the fingering is a fingering of, but it is not written.
		sym.Root, sym.Quality, sym.Bass, sym.Text = "", "", "", ""
	}
	if f := q.Get("frame"); f != "" {
		sym.Frame = parseChordFrame(f, q.Get("firstfret"), q.Get("barre"))
	}
	if sym.String() == "" && sym.Frame == nil {
		// Neither the name nor the diagram: nothing to write.
		for _, m := range marks {
			m.Chord = nil
		}
		writeJSON(w, map[string]any{"status": "ok", "chord": ""})
		return
	}
	// One note carries it, not every note of a selection: a chord symbol
	// marks where the harmony changes, and repeating it on each note of the
	// bar would say it changes on every one.
	marks[0].Chord = sym
	writeJSON(w, map[string]any{"status": "ok", "chord": sym.String()})
}

// parseChordSymbol takes a chord name apart.
//
// "N.C." and anything else that is not a chord is kept whole: a symbol that
// is not a chord has no root to set large and no quality to set small.
func parseChordSymbol(s string) *model.ChordSymbol {
	root, rest := "", s
	if len(s) > 0 && s[0] >= 'A' && s[0] <= 'G' {
		root, rest = s[:1], s[1:]
		// The accidental belongs to the root, however it was typed.
		for len(rest) > 0 && (rest[0] == '#' || rest[0] == 'b') {
			root += rest[:1]
			rest = rest[1:]
		}
		for _, sign := range []string{"♯", "♭"} {
			for strings.HasPrefix(rest, sign) {
				root += rest[:len(sign)]
				rest = rest[len(sign):]
			}
		}
	}
	if root == "" {
		return &model.ChordSymbol{Text: s}
	}

	quality, bass := rest, ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		quality, bass = rest[:i], rest[i+1:]
	}
	return &model.ChordSymbol{Root: root, Quality: quality, Bass: bass}
}

// parseChordFrame reads a fingering as it is usually written down: one number
// per string, low to high, with x for a string that is not played.
//
//	"x02220"   an open shape
//	"133211"   a barre
func parseChordFrame(spec, firstFret, barre string) *model.ChordFrame {
	f := &model.ChordFrame{FirstFret: 1}
	if v, err := strconv.Atoi(firstFret); err == nil && v > 1 {
		f.FirstFret = v
	}
	if v, err := strconv.Atoi(barre); err == nil && v > 0 {
		f.Barre = v
	}

	fields := strings.Fields(spec)
	if len(fields) <= 1 {
		// Written as one run of characters, which only works up to fret 9 —
		// past that the spaced form is the one to use.
		for _, c := range spec {
			switch {
			case c == 'x' || c == 'X' || c == '-':
				f.Frets = append(f.Frets, -1)
			case c >= '0' && c <= '9':
				f.Frets = append(f.Frets, int(c-'0'))
			}
		}
		return f
	}
	for _, field := range fields {
		if field == "x" || field == "X" || field == "-" {
			f.Frets = append(f.Frets, -1)
			continue
		}
		if v, err := strconv.Atoi(field); err == nil {
			f.Frets = append(f.Frets, v)
		}
	}
	return f
}

/*
 * Staves
 */

// handleAddStaff adds a staff to the score.
//
// Without this a score is whatever one staff its file happened to have, and
// composing anything for more than one player means starting somewhere else
// and importing. A chart for piano, bass and drums begins by saying there are
// three of them.
//
// The new staff is given as many measures as the longest one already there, so
// the barlines line up from the first bar: a staff with no measures draws
// nothing and a staff with too few stops in the middle of the system.
func (a *App) handleAddStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	clef := model.Treble
	if c, ok := clefByStringName[q.Get("clef")]; ok {
		clef = c
	}
	inst := model.AcousticGrandPiano
	// How the staff is written down, for a fretted instrument, and in
	// what tuning.
	writtenAs := model.WrittenPitches
	tuningName := ""
	// sound and trans are what the staff is and how it transposes. A sound
	// answers both; an instrument chosen as a General MIDI program can only
	// answer the second, and only as well as the program allows.
	sound, named := "", ""
	trans := model.Transposition{}
	if v := q.Get("sound"); v != "" {
		probe := &model.Part{}
		if !applySound(probe, v) {
			http.Error(w, "unknown instrument "+v, http.StatusBadRequest)
			return
		}
		// How it is written down, which for a fretted instrument is a
		// question of its own: a staff of pitches, a tab, or the pair of
		// them a guitar part is usually printed as.
		written, ok := model.WrittenFromName(strings.TrimSpace(q.Get("written")))
		if !ok {
			http.Error(w, q.Get("written")+" is not pitches, tab or both",
				http.StatusBadRequest)
			return
		}
		if written != model.WrittenPitches && model.DefaultTuning(v) == "" {
			http.Error(w, "that instrument has no frets to write a tab for",
				http.StatusBadRequest)
			return
		}
		// Which tuning, and so how many lines the tab is ruled with: a
		// bass is four strings and often five, and the number of lines
		// is not a setting of its own — it is however many strings the
		// instrument has.
		tuningName = model.DefaultTuning(v)
		if t := strings.TrimSpace(q.Get("tuning")); t != "" {
			if _, ok := model.TuningNamed(t); !ok {
				http.Error(w, "no tuning called "+t+" — /api/tunings lists them",
					http.StatusBadRequest)
				return
			}
			tuningName = t
		}
		writtenAs = written
		sound, inst = v, probe.Instrument
		named, _ = model.SoundName(v)
		trans, _ = model.SoundTransposition(v)
		if probe.Clef == model.Percussion {
			clef = model.Percussion
		} else if _, given := clefByStringName[q.Get("clef")]; !given {
			// A bassoon in the treble clef is a mistake nobody wants to
			// correct by hand, so the clef follows the instrument unless
			// the caller said otherwise.
			clef = clefForSound(v)
		}
	} else if v := q.Get("instrument"); isDrumKit(v) {
		// A drum kit is a staff whose lines name drums, which is the clef.
		clef = model.Percussion
	} else if v != "" {
		switch i, ok := model.InstrumentFromName(v); {
		case ok:
			inst = i
		default:
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 127 {
				// Not silently a piano. A name nobody recognises is a
				// mistake worth hearing about, and a staff that quietly
				// came out as the wrong instrument is found much later.
				http.Error(w, "unknown instrument "+v, http.StatusBadRequest)
				return
			}
			inst = model.Instrument(n)
		}
		trans = model.TranspositionFor(inst)
	} else {
		trans = model.TranspositionFor(inst)
	}

	// The key and the meter come from the score, not from nothing: a new
	// staff in C major under a piece in five flats would be wrong on sight.
	//
	// From the score and not from the staff it is copied off. A signature is
	// what its own player reads, so the first part's is turned back into the
	// key of the music and then written again for whoever is joining: a
	// saxophone added to a piece in four sharps reads seven, and handing it
	// the four put it in the wrong key with an accidental on nearly every
	// note of it.
	key := model.KeySignature{}
	time := model.TimeSignature{Beats: 4, BeatValue: 4}
	bars := 1
	concert := model.KeySignature{}
	if len(a.score.Parts) > 0 {
		first := a.score.Parts[0]
		concert = first.Transpose.Sounding(first.Key)
		key, time = trans.Written(concert), first.Time
		for _, p := range a.score.Parts {
			if len(p.Measures) > bars {
				bars = len(p.Measures)
			}
		}
	}
	// A drum staff is not in any key: its lines carry no pitches.
	if clef == model.Percussion {
		key = model.KeySignature{}
	}

	part := &model.Part{
		Name: strings.TrimSpace(q.Get("name")), Instrument: inst,
		Sound: sound, Clef: clef, Key: key, Time: time,
		Transpose: trans,
	}
	if part.Name == "" && named != "" {
		part.Name = capitalised(named)
	}
	if part.Name == "" {
		// Capitalised, because this is printed down the left of the score
		// and on the title page of the part: every published score writes
		// "Flute" beside the staff, and the instrument table's own "flute"
		// is a name in a table. The drum kit below was already written this
		// way, which is what the other names should have been.
		part.Name = capitalised(inst.String())
		// A drum staff has no program, so the unused one must not name it.
		if clef == model.Percussion {
			part.Name = "Drum Kit"
		}
	}
	for i := 0; i < bars; i++ {
		part.Measures = append(part.Measures, &model.Measure{
			Voices: []*model.Voice{{}}, RightBarline: model.BarSingle,
		})
	}
	// The barlines of the staves already there, so a repeat or a final bar
	// runs down the whole system from the moment the staff appears.
	if len(a.score.Parts) > 0 {
		src := a.score.Parts[0]
		for i := 0; i < bars && i < len(src.Measures); i++ {
			part.Measures[i].RightBarline = src.Measures[i].RightBarline
			// A key change partway through is the music changing key, so it
			// too is read off the staff it is written on and written again
			// for this one.
			if k := src.Measures[i].Key; k != nil && clef != model.Percussion {
				c := trans.Written(src.Transpose.Sounding(*k))
				part.Measures[i].Key = &c
			}
			if t := src.Measures[i].Time; t != nil {
				c := *t
				part.Measures[i].Time = &c
			}
		}
	}

	at := len(a.score.Parts)
	if v, err := strconv.Atoi(q.Get("at")); err == nil && v >= 0 && v <= len(a.score.Parts) {
		at = v
	}
	a.score.Parts = append(a.score.Parts, nil)
	copy(a.score.Parts[at+1:], a.score.Parts[at:])
	a.score.Parts[at] = part
	shiftStaves(a.score, at, 1)

	// Written as tablature, or as the pair a guitar part is usually
	// printed as: the notation above and the tab below, joined down the
	// left with the barlines run through, because it is one player
	// reading one line written down twice.
	if writtenAs != model.WrittenPitches {
		tab, _ := model.TuningNamed(tuningName)
		if writtenAs == model.WrittenTab {
			part.Tab = tab
			restringPart(part)
		} else {
			below := part.Clone()
			below.Name = "" // named once, over the pair
			below.Tab = tab
			// It holds no music of its own: the notation above it is
			// the music, and this is that music written a second way.
			// Two copies would be two things to keep in step, and they
			// would not stay in step.
			below.Measures = nil
			below.MirrorOf = &at
			a.score.Parts = append(a.score.Parts, nil)
			copy(a.score.Parts[at+2:], a.score.Parts[at+1:])
			a.score.Parts[at+1] = below
			shiftStaves(a.score, at+1, 1)
			a.score.Groups = append(a.score.Groups, model.StaffGroup{
				First: at, Last: at + 1, Symbol: model.GroupLine,
				Barlines: true, Name: part.Name,
			})
		}
	}

	writeJSON(w, map[string]any{
		"status": "ok", "at": at, "staves": len(a.score.Parts),
	})
}

// handleRemoveStaff takes a staff out.
func (a *App) handleRemoveStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	at, _ := strconv.Atoi(r.URL.Query().Get("part"))

	a.lock()
	defer a.mu.Unlock()

	if at < 0 || at >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	if len(a.score.Parts) == 1 {
		http.Error(w, "a score has to have a staff", http.StatusBadRequest)
		return
	}
	a.score.Parts = append(a.score.Parts[:at], a.score.Parts[at+1:]...)
	shiftStaves(a.score, at, -1)
	writeJSON(w, map[string]any{"status": "ok", "staves": len(a.score.Parts)})
}

// handleMoveStaff changes the order the staves are drawn in.
//
// Score order is a real decision — woodwind at the top, strings at the
// bottom, the rhythm section together — and getting it wrong is not something
// to be fixed by starting again.
func (a *App) handleMoveStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	from, _ := strconv.Atoi(q.Get("part"))
	to, err := strconv.Atoi(q.Get("to"))
	if err != nil {
		to = from + 1
	}

	a.lock()
	defer a.mu.Unlock()

	n := len(a.score.Parts)
	if from < 0 || from >= n || to < 0 || to >= n || from == to {
		http.Error(w, "no such staff to move", http.StatusBadRequest)
		return
	}
	p := a.score.Parts[from]
	a.score.Parts = append(a.score.Parts[:from], a.score.Parts[from+1:]...)
	a.score.Parts = append(a.score.Parts, nil)
	copy(a.score.Parts[to+1:], a.score.Parts[to:])
	a.score.Parts[to] = p

	// A group names staves by position, and the positions have just changed.
	// Rather than guess what was meant, the groups that spanned the moved
	// staff are dropped and the rest are left alone.
	var kept []model.StaffGroup
	for _, g := range a.score.Groups {
		if from >= g.First && from <= g.Last {
			continue
		}
		kept = append(kept, g)
	}
	a.score.Groups = kept

	writeJSON(w, map[string]any{"status": "ok", "at": to})
}

// shiftGroups moves the staff numbers a group refers to when a staff is added
// or removed before it, and drops a group that no longer spans anything.
// shiftMirrors keeps a staff that mirrors another pointing at it when the
// staves above them move.
//
// shiftStaves renumbers the mirrors and the groups after staves have been
// put in or taken out at `at`. The rule itself lives in the model, with
// pairing, because pairing is the other thing that renumbers them and two
// copies of a renumbering rule is one copy too many.
func shiftStaves(s *model.Score, at, delta int) {
	model.ShiftStaffIndexes(s, at, delta)
}

// handleMeasureRepeat marks a bar as a repeat of the one before it.
func (a *App) handleMeasureRepeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	bars, _ := strconv.Atoi(q.Get("bars"))

	a.lock()
	defer a.mu.Unlock()

	_, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if bars > 0 && measIdx < bars {
		http.Error(w, "there is no earlier bar to repeat", http.StatusBadRequest)
		return
	}
	m.Repeat = bars
	writeJSON(w, map[string]any{"status": "ok", "bars": bars})
}

// handleSlur draws a slur over the selection.
//
// A slur covers a phrase, so it needs a beginning and an end — which is what
// a range selection is. The same shape as a hairpin, and refused for the same
// reason when there is only one note: a slur from a note to itself has
// nothing to arc over.
func (a *App) handleSlur(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	marks, err := a.marksOf(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("slur") == "none" {
		// ends says the selection is the two notes one curve runs between,
		// which is what clicking a slur and pressing Backspace means. Only
		// that curve goes: a note in the middle of a phrase closes one slur
		// and opens the next, so clearing both marks on both ends would
		// take away the slur on either side of the one that was asked for.
		ends := r.URL.Query().Get("ends") != ""
		for i, m := range marks {
			if !ends {
				m.SlurStart, m.SlurStop = 0, 0
				continue
			}
			if i == 0 {
				m.SlurStart = 0
			}
			if i == len(marks)-1 {
				m.SlurStop = 0
			}
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	}
	if len(marks) < 2 {
		http.Error(w, "a slur goes from one note to another — select a passage",
			http.StatusBadRequest)
		return
	}
	for i, m := range marks {
		m.SlurStart = model.SlurEnd(i == 0)
		m.SlurStop = model.SlurEnd(i == len(marks)-1)
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}

// handleTie joins notes of the same pitch so they sound as one.
//
// Unlike a slur, a tie is between neighbours: each note in the selection is
// tied to the one after it, so selecting three notes ties all three together
// rather than arcing from the first to the last.
//
// A chord is tied notehead by notehead. Holding the top note while the others
// move is ordinary writing, so "tie this chord" cannot be the only thing that
// can be said: a step names one notehead and ties only that, and no step at
// all means the whole chord.
func (a *App) handleTie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	targets, err := a.tieTargets(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// A tie means "hold this note into the next one", so one note is enough
	// to say it: the note it runs to is the one that follows, across the
	// barline if that is where it is. Asking for both was asking the reader
	// to say something the mark already says.
	if len(targets) == 1 && q.Get("tie") != "none" {
		a.lock()
		if next, ok := a.noteAfter(targets[0]); ok {
			targets = append(targets, next)
		}
		a.mu.Unlock()
	}

	step, hasStep := 0, false
	if v := q.Get("step"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			step, hasStep = n, true
		}
	}

	if q.Get("tie") == "none" {
		// ends says the selection is the two notes one curve runs between,
		// which is what clicking a tie and pressing Backspace means. A note
		// held through three bars stops one tie and starts the next on the
		// same notehead, so clearing both marks on both ends would take
		// away the ties on either side of the one that was asked for.
		ends := q.Get("ends") != ""
		for i, t := range targets {
			start, stop := true, true
			if ends {
				start, stop = i == 0, i == len(targets)-1
			}
			clearTie(t, step, hasStep, start, stop)
		}
		writeJSON(w, map[string]int{"changed": len(targets)})
		return
	}
	if len(targets) < 2 {
		// One note and nothing after it: the note it would be held into
		// does not exist. Said as what it is, because "select both" sounds
		// like a selection to make and there is no second note to select.
		http.Error(w, "there is no note after this one to tie it to",
			http.StatusBadRequest)
		return
	}

	for i, t := range targets {
		if !setTie(t, step, hasStep, i < len(targets)-1, i > 0) && hasStep {
			http.Error(w, "there is no notehead there to tie",
				http.StatusBadRequest)
			return
		}
	}
	writeJSON(w, map[string]int{"changed": len(targets)})
}

// tieTarget is one element of the selection, with the staff it is on so a
// notehead's position can be worked out, and where it sits so the note after
// it can be found.
type tieTarget struct {
	part    *model.Part
	el      model.Element
	measure int
	index   int
}

// tieTargets resolves the selection to elements, in reading order.
func (a *App) tieTargets(q url.Values) ([]tieTarget, error) {
	a.lock()
	defer a.mu.Unlock()

	var out []tieTarget
	add := func(p *model.Part, el model.Element, measure, index int) {
		switch el.(type) {
		case *model.Note, *model.Chord:
			out = append(out, tieTarget{
				part: p, el: el, measure: measure, index: index,
			})
		}
	}

	if sel := q.Get("sel"); sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			return nil, err
		}
		for _, t := range targets {
			p, m, err := a.locate(t.part, t.measure)
			if err != nil {
				continue
			}
			v := m.Voices[0]
			if t.index < 0 || t.index >= len(v.Elements) {
				continue
			}
			add(p, v.Elements[t.index], t.measure, t.index)
		}
		if len(out) == 0 {
			return nil, editError("there is no note in the selection")
		}
		return out, nil
	}

	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}
	p, m, lerr := a.locate(partIdx, measIdx)
	if lerr != nil {
		return nil, lerr
	}
	v := m.Voices[0]
	if len(v.Elements) == 0 {
		return nil, editError("the measure is empty")
	}
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	add(p, v.Elements[at-1], measIdx, at-1)
	if len(out) == 0 {
		return nil, editError("that is not a note")
	}
	return out, nil
}

// noteAfter finds the next note or chord on the same staff, carrying on into
// the following bars when the selected note is the last of its own.
func (a *App) noteAfter(t tieTarget) (tieTarget, bool) {
	p := t.part
	index := t.index + 1
	for m := t.measure; m < len(p.Measures); m++ {
		if len(p.Measures[m].Voices) == 0 {
			index = 0
			continue
		}
		els := p.Measures[m].Voices[0].Elements
		for ; index < len(els); index++ {
			switch els[index].(type) {
			case *model.Note, *model.Chord:
				return tieTarget{part: p, el: els[index], measure: m, index: index}, true
			}
		}
		index = 0
	}
	return tieTarget{}, false
}

// setTie marks one element, or one notehead of it. It reports whether it
// found anything to mark.
func setTie(t tieTarget, step int, hasStep, start, stop bool) bool {
	ch, isChord := t.el.(*model.Chord)
	if !isChord || !hasStep {
		m := marksOfElement(t.el)
		if m == nil {
			return false
		}
		m.TieStart = m.TieStart || start
		m.TieStop = m.TieStop || stop
		return true
	}
	// The clef in force where the note is, not the one the staff opens in:
	// after a clef change the same note sits at a different step, and a tie
	// aimed at a notehead by step found none of them.
	clef := t.part.ClefAt(t.measure)
	for _, n := range ch.Notes {
		if engrave.StaffStep(clef, n.Pitch) != step {
			continue
		}
		n.TieStart = n.TieStart || start
		n.TieStop = n.TieStop || stop
		return true
	}
	return false
}

// clearTie takes a tie off, from one notehead or from all of them.
func clearTie(t tieTarget, step int, hasStep, start, stop bool) {
	if ch, ok := t.el.(*model.Chord); ok {
		clef := t.part.ClefAt(t.measure)
		// A mark written on the chord itself covers every notehead in it.
		// Untying one of them leaves the others tied, so the chord's own
		// mark is handed down to its notes before it goes — otherwise the
		// notehead was cleared, the chord's mark still said "all of them",
		// and the curve was drawn again as though nothing had happened.
		if hasStep && (ch.TieStart || ch.TieStop) {
			for _, n := range ch.Notes {
				n.TieStart = n.TieStart || ch.TieStart
				n.TieStop = n.TieStop || ch.TieStop
			}
			ch.TieStart, ch.TieStop = false, false
		}
		for _, n := range ch.Notes {
			if hasStep && engrave.StaffStep(clef, n.Pitch) != step {
				continue
			}
			n.TieStart = n.TieStart && !start
			n.TieStop = n.TieStop && !stop
		}
		if !hasStep {
			ch.TieStart = ch.TieStart && !start
			ch.TieStop = ch.TieStop && !stop
		}
		return
	}
	if m := marksOfElement(t.el); m != nil {
		m.TieStart = m.TieStart && !start
		m.TieStop = m.TieStop && !stop
	}
}

// handlePercussion lists the drum kit: what is in it, where each piece is
// written and how it is drawn.
//
// The editor needs this to offer the kit by name. A drum staff's lines carry
// instruments rather than pitches, and the only way to know that the top
// space is a hi-hat is to be told — which is what a printed drum key does for
// a player, and what this does for the palette.
func (a *App) handlePercussion(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Name     string `json:"name"`
		Key      int    `json:"key"`
		Step     int    `json:"step"`
		Notehead string `json:"notehead"`
		// Shortcut is the letter that types this pad. It follows the pad's
		// place in the kit rather than the instrument, so a small kit gets
		// the easy keys.
		Shortcut string `json:"shortcut,omitempty"`
	}
	type kitInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Pads        int    `json:"pads"`
	}
	type reply struct {
		// Kit is the one being offered, and Kits are the sizes to choose
		// from: a rock groove wants six pads in front of it and a Latin
		// chart wants forty, and only the person writing it knows which.
		Kit  string    `json:"kit"`
		Kits []kitInfo `json:"kits"`
		Pads []entry   `json:"pads"`
	}

	kit := model.DrumKitByID(r.URL.Query().Get("kit"))
	out := reply{Kit: kit.ID}
	for _, k := range model.DrumKits {
		out.Kits = append(out.Kits, kitInfo{
			ID: k.ID, Name: k.Name, Description: k.Description, Pads: len(k.Pads()),
		})
	}
	for _, n := range kit.Pads() {
		out.Pads = append(out.Pads, entry{
			Name: n.Name, Key: n.Key, Step: n.Step, Notehead: n.Head.String(),
			Shortcut: n.Shortcut,
		})
	}
	writeJSON(w, out)
}

// drumKitName is what the kit is called in the list of instruments.
const drumKitName = "drum kit"

// applySound sets a staff to one of the catalogue's instruments.
//
// The sound is what the staff *is* and the program is only what plays it,
// so both are written: a contrabassoon staff keeps "wind.reed.contrabassoon"
// and sounds through a bassoon, because General MIDI has nothing nearer.
// Asked for the range, the staff answers as a contrabassoon — which is the
// whole reason the sound is kept.
//
// The transposition comes from the sound too. Reading it off the program
// cannot tell a clarinet in A from one in B flat, and the editor used to
// get that wrong by a semitone on every clarinet in A ever chosen.
func applySound(p *model.Part, id string) bool {
	if !model.SoundKnown(id) {
		return false
	}
	p.Sound = id

	if model.SoundIsPercussion(id) {
		// A staff of drums is in no key and reads no pitches: its lines
		// name instruments, which is what the percussion clef says.
		p.Clef = model.Percussion
		p.Key = model.KeySignature{}
		p.Transpose = model.Transposition{}
		return true
	}
	if prog, ok := model.SoundProgram(id); ok {
		p.Instrument = prog
	}
	return true
}

// clefForSound is the clef an instrument is written in.
//
// So that nobody has to tell the program that a cello reads the bass clef.
// It is a default and not a rule: the staff dialog can change it, and a
// part that wants a different one says so.
//
// A list and not a rule derived from the range, which was tried and is
// wrong. A clef is a convention about what a player reads, not a
// consequence of where the instrument sounds: a cello's top C is as high as
// a violin's open E and it still reads the bass clef, and a horn covers
// three and a half octaves centred below middle C and still reads the
// treble. Neither follows from a number, so neither is guessed from one.
func clefForSound(id string) model.Clef {
	switch id {
	case "strings.viola", "strings.viola-damore", "strings.viol.alto",
		"brass.trombone.alto", "brass.sackbutt.alto":
		return model.Alto
	}
	switch id {
	case "strings.cello", "strings.cello.piccolo", "strings.contrabass",
		"strings.octobass", "strings.viol", "strings.viol.tenor",
		"strings.viol.bass", "strings.viol.violone",
		"wind.reed.bassoon", "wind.reed.contrabassoon",
		"brass.trombone", "brass.trombone.tenor", "brass.trombone.bass",
		"brass.trombone.contrabass", "brass.sackbutt", "brass.sackbutt.tenor",
		"brass.sackbutt.bass", "brass.tuba", "brass.tuba.bass",
		"brass.tuba.subcontrabass", "brass.euphonium", "brass.baritone-horn",
		"brass.sousaphone", "brass.cimbasso", "brass.ophicleide",
		"brass.serpent", "brass.trumpet.bass",
		"pluck.bass", "pluck.bass.acoustic", "pluck.bass.electric",
		"pluck.bass.fretless", "pluck.bass.synth",
		"drum.timpani", "pitched-percussion.marimba.bass":
		return model.Bass
	}
	return model.Treble
}

// isDrumKit reports whether a chosen instrument is the kit rather than a
// General MIDI program.
func isDrumKit(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), drumKitName)
}

// beatName is the inverse of durationNames: the word for a beat, and whether
// it is dotted.
//
// A dotted beat is half as long again, so it is not in the table of plain
// values at all — it is one of them with a dot, and saying so is what lets a
// dialog show the note and the dot separately the way they are written.
func beatName(d model.Duration) (string, bool) {
	for _, dotted := range []bool{false, true} {
		want := d
		if dotted {
			if want.Num%3 != 0 {
				continue
			}
			want = want.Scale(2, 3)
		}
		want = want.Reduced()
		for name, v := range durationNames {
			if v.Reduced() == want {
				return name, dotted
			}
		}
	}
	return "quarter", false
}

// handleCursor records where the writing cursor is, so that saving the score
// keeps it and opening the file again puts it back.
//
// The editor sends it when the cursor moves rather than the server guessing
// from the last edit: moving the cursor and not writing anything is still
// where you are, and a piece you opened to read through should not come back
// with the cursor wherever you last happened to type.
func (a *App) handleCursor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	num := func(name string) int {
		n, _ := strconv.Atoi(q.Get(name))
		return n
	}
	part := num("part")

	a.lock()
	defer a.mu.Unlock()
	if part < 0 || part >= len(a.score.Parts) {
		http.Error(w, "no such staff", http.StatusBadRequest)
		return
	}
	a.score.Cursor = &model.Cursor{
		Part: part, Measure: num("measure"), At: num("at"), Step: num("step"),
	}
	writeJSON(w, map[string]any{"status": "ok"})
}

/*
 * Tuplets and tremolos
 */

// tupletNormal is how many ordinary notes a group of n is played in the time
// of: the "2" of a triplet's three-in-the-time-of-two.
//
// The answers are the ones every engraver uses, and they are not a formula —
// three goes in the time of two, five and six and seven in the time of four,
// nine in the time of eight. Where n is itself a power of two the group only
// means anything against a dotted beat, and then it is the three-quarters of
// it: four in the time of three, eight in the time of six. Anything larger
// falls back to the power of two below it, which is the rule the table is an
// exception to rather than the other way round.
func tupletNormal(n int) int {
	switch n {
	case 2:
		return 3
	case 3:
		return 2
	case 4:
		return 3
	case 5, 6, 7:
		return 4
	case 8:
		return 6
	case 9:
		return 8
	}
	p := 1
	for p*2 < n {
		p *= 2
	}
	return p
}

// tupletFit picks what a group of n is played in the time of, over a span.
//
// The table is the answer where the span can be divided that way. It often
// cannot: four in the time of three divides the beat in three, so it goes on
// a dotted note and nowhere else, and asked for on a plain quarter it would
// want a written value of a twelfth of a whole note — which is not a
// notehead and cannot be drawn.
//
// Rather than refuse, it comes down to the largest division that does work.
// Somebody asking for four notes in this space means the four notes; the
// ratio printed over them is the engraver's business, and the caller is told
// which one it used so it can say so.
func tupletFit(n int, span model.Duration) (normal int, ok bool) {
	try := func(d int) bool {
		if d < 1 || d >= n {
			return false
		}
		_, _, fits := span.Scale(1, d).Written()
		return fits
	}
	if d := tupletNormal(n); try(d) {
		return d, true
	}
	for d := n - 1; d >= 1; d-- {
		if try(d) {
			return d, true
		}
	}
	return 0, false
}

// restsFor spells a duration as the fewest rests that can be written, and
// says what it could not spell.
//
// Freed time is rarely one notehead: a triplet made out of three eighths
// gives an eighth back, which is, but one made out of three sixteenths in
// 6/8 need not be. Taking the largest writable value that fits and going
// round again spells whatever is left as two or three rests rather than
// refusing it.
//
// Some durations no number of plain rests spells at all. A twenty-fourth of
// a whole note is one: it is a tuplet's own fraction, and the remainder is
// returned rather than dropped so that the caller can spell it the way the
// tuplet spells things.
func restsFor(d model.Duration) (out []model.Element, left model.Duration) {
	zero := model.Duration{Num: 0, Den: 1}
	left = d.Reduced()
	for left.Cmp(zero) > 0 && len(out) < 8 {
		best := zero
		for v := model.DoubleWhole; v <= model.HundredTwentyEighth; v++ {
			for dots := 0; dots <= maxWrittenDots; dots++ {
				cand := dotted(v.Duration(), strconv.Itoa(dots))
				if cand.Cmp(left) <= 0 && cand.Cmp(best) > 0 {
					best = cand
				}
			}
		}
		if best.IsZero() {
			break
		}
		out = append(out, &model.Rest{Dur: best})
		left = left.Sub(best).Reduced()
	}
	return out, left
}

// restsGivenBack spells the time a group gave back, as rests to go after it.
//
// Plain rests where a notehead spells the time. Where none does, the rests
// become members of the group and are written against its ratio instead:
// shortening one note of a triplet leaves a twenty-fourth of a whole note
// over, and the only thing that spells a twenty-fourth is a sixteenth read as
// part of a triplet. Those rests carry the group's marks and so extend it,
// which is how a group of mixed values is written by hand.
func restsGivenBack(freed model.Duration, actual, normal int) []model.Element {
	out, left := restsFor(freed)
	if left.IsZero() {
		return out
	}
	if actual <= 0 || normal <= 0 {
		return out // nothing better to offer; the remainder is dropped
	}
	// The whole of it, inside the group, rather than part out and part in.
	inside, over := restsFor(freed.Scale(actual, normal))
	if !over.IsZero() {
		return out
	}
	for _, el := range inside {
		setDuration(el, el.Duration().Scale(normal, actual).Reduced())
		if mk := marksOfElement(el); mk != nil {
			mk.Tuplet = &model.Tuplet{Actual: actual, Normal: normal}
		}
	}
	return inside
}

// tupletFromSelection reads a run of notes already written as a group of n.
//
// This is the other half of the operation, and the one somebody reaches for
// after writing a passage out plain and then seeing it was a triplet all
// along. Asked for on a selection, the single-note form would have thrown
// the notes away and left n rests in their place — so the answer to "make
// these three a triplet" was to write them again.
//
// Here the notes stay and only their arithmetic changes: each keeps the
// notehead it has and is played in n-in-the-time-of-normal, which is exactly
// what the figure printed over them says. The group is then shorter than the
// notes were, and the time it gives back becomes a rest after it, so the bar
// still adds up and there is somewhere to move the following notes into.
//
// Fewer notes than the group needs is not an error either: a group of three
// asked for on two eighths is a triplet with its third member still to be
// written, and it comes in as a rest.
// drawBackSpill brings the end of a run back from the bar it ran into, when
// reading it as a tuplet makes room for it where it was written.
//
// The shape it answers to is the one writing a tuplet makes: some notes at
// the end of a bar, the rest at the start of the next, nothing in between
// and nothing of anybody else's among them. If the group is shorter once it
// is read as n in the time of normal — which is every tuplet that is not a
// duplet — and that length fits where the run began, the stragglers are
// moved back and the selection comes back pointing at them.
//
// Anything else is left exactly as it was, for the caller to refuse: a
// tuplet across a barline is not a thing notation can write, and moving
// somebody's music about to make one would be worse than saying no.
func drawBackSpill(p *model.Part, measIdx int, sorted []target, n int) ([]target, int, error) {
	home := sorted[0].measure
	if home < 0 || home >= len(p.Measures) {
		return sorted, measIdx, editError("the selection is not in this score any more")
	}
	var spilled []target
	for _, t := range sorted {
		switch t.measure {
		case home:
			if len(spilled) > 0 {
				// Back in the first bar after leaving it, which is not a
				// run at all.
				return sorted, measIdx, nil
			}
		case home + 1:
			spilled = append(spilled, t)
		default:
			return sorted, measIdx, nil
		}
	}
	if len(spilled) == 0 {
		return sorted, home, nil
	}

	from, into := p.Measures[home], p.Measures[home+1]
	if from == nil || into == nil || len(from.Voices) == 0 || len(into.Voices) == 0 {
		return sorted, measIdx, nil
	}
	head, tail := from.Voices[0], into.Voices[0]
	if head == nil || tail == nil {
		return sorted, measIdx, nil
	}

	// The stragglers have to be the start of the bar they are in, and what
	// they left behind has to be the end of the bar they came from, or this
	// is not one run of notes with a barline through it.
	for i, t := range spilled {
		if t.index != i {
			return sorted, measIdx, nil
		}
	}
	kept := sorted[:len(sorted)-len(spilled)]
	for i, t := range kept {
		if t.index != len(head.Elements)-len(kept)+i {
			return sorted, measIdx, nil
		}
	}

	// Does it fit? The group's own length once it is read as a tuplet,
	// against the room left in the bar it was written in.
	normal := tupletNormal(n)
	group := model.Duration{Num: 0, Den: 1}
	for _, t := range kept {
		group = group.Add(head.Elements[t.index].Duration())
	}
	for _, t := range spilled {
		group = group.Add(tail.Elements[t.index].Duration())
	}
	pad := model.Duration{Num: 0, Den: 1}
	if len(sorted) < n {
		first := head.Elements[kept[0].index].Duration()
		for i := len(sorted); i < n; i++ {
			pad = pad.Add(first)
		}
	}
	after := group.Add(pad).Scale(normal, n).Reduced()

	before := model.Duration{Num: 0, Den: 1}
	for i, el := range head.Elements {
		if i >= len(head.Elements)-len(kept) {
			break
		}
		before = before.Add(el.Duration())
	}
	if before.Add(after).Cmp(p.TimeAt(home).BarDuration()) > 0 {
		return sorted, measIdx, nil
	}

	// Home they come, in the order they were written.
	moved := make([]model.Element, len(spilled))
	for i := range spilled {
		moved[i] = tail.Elements[i]
	}
	tail.Elements = append([]model.Element{}, tail.Elements[len(spilled):]...)
	head.Elements = append(head.Elements, moved...)

	out := make([]target, 0, len(sorted))
	for i := len(head.Elements) - len(sorted); i < len(head.Elements); i++ {
		out = append(out, target{part: sorted[0].part, measure: home, index: i})
	}
	return out, home, nil
}

func tupletFromSelection(p *model.Part, measIdx int, targets []target, n int) (normal, members int, freed model.Duration, err error) {
	freed = model.Duration{Num: 0, Den: 1}
	sorted := append([]target{}, targets...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].measure != sorted[j].measure {
			return sorted[i].measure < sorted[j].measure
		}
		return sorted[i].index < sorted[j].index
	})
	for _, t := range sorted {
		if t.part != sorted[0].part {
			return 0, 0, freed, editError("a tuplet is made out of one staff")
		}
	}

	// A run that ran over the barline while it was being written.
	//
	// Three eighths take a beat and a half, and as a triplet they take one:
	// the group that would not fit before the figure was asked for fits
	// after it. So writing the third note of a triplet at the end of a bar
	// puts it in the next bar — there is nowhere else for it to go yet —
	// and asking for the triplet brings it home, which is what the asking
	// meant. Refusing it, which is what this did, left somebody with three
	// notes across a barline and no way to say what they are.
	sorted, measIdx, err = drawBackSpill(p, measIdx, sorted, n)
	if err != nil {
		return 0, 0, freed, err
	}
	for _, t := range sorted {
		if t.measure != measIdx {
			return 0, 0, freed, editError("a tuplet is made out of one bar of one staff")
		}
	}

	m := p.Measures[measIdx]
	// The voice the run is in, found the way every other selection finds it:
	// the first one that has an element where the selection says.
	var v *model.Voice
	for _, cand := range m.Voices {
		if cand != nil && sorted[0].index >= 0 && sorted[0].index < len(cand.Elements) {
			v = cand
			break
		}
	}
	if v == nil {
		return 0, 0, freed, editError("the selection is not in this bar any more")
	}

	for i, t := range sorted {
		if t.index < 0 || t.index >= len(v.Elements) {
			return 0, 0, freed, editError("the selection is not in this bar any more")
		}
		// Next to each other. Three notes picked out of a bar with others
		// between them are not a group, and grouping them would mean
		// moving the music about rather than reading it differently.
		if i > 0 && t.index != sorted[i-1].index+1 {
			return 0, 0, freed, editError("a tuplet is made out of notes next to each other")
		}
		if mk := marksOfElement(v.Elements[t.index]); mk != nil && mk.Tuplet != nil {
			return 0, 0, freed, editError("part of that selection is already in a tuplet")
		}
	}
	if len(sorted) > n {
		return 0, 0, freed, editError(fmt.Sprintf(
			"%d notes are selected, which is more than a group of %d",
			len(sorted), n))
	}

	normal = tupletNormal(n)
	first, last := sorted[0].index, sorted[len(sorted)-1].index
	before := spanOf(v.Elements[first : last+1])

	// The notehead the padding members take: the first one's, which is what
	// a group written out by hand would use.
	pad := v.Elements[first].Duration()

	after := model.Duration{Num: 0, Den: 1}
	for i := first; i <= last; i++ {
		el := v.Elements[i]
		each := el.Duration().Scale(normal, n).Reduced()
		setDuration(el, each)
		after = after.Add(each)
		mk := marksOfElement(el)
		if mk == nil {
			return 0, 0, freed, editError("that selection cannot carry a tuplet")
		}
		mk.Tuplet = &model.Tuplet{
			Actual: n, Normal: normal,
			Start: i == first, Stop: false,
		}
	}

	extra := make([]model.Element, 0, n-len(sorted))
	for i := len(sorted); i < n; i++ {
		each := pad.Scale(normal, n).Reduced()
		rest := &model.Rest{Dur: each}
		rest.Marks.Tuplet = &model.Tuplet{Actual: n, Normal: normal}
		extra = append(extra, rest)
		after = after.Add(each)
	}
	members = n

	// What the group gives back. A duplet — two in the time of three — takes
	// more time than it was given rather than less, and then there is
	// nothing to give back and the bar overflows into the next, which is
	// what every other overfull bar here does.
	//
	// The rests are there so the bar still adds up, which is the same job
	// spill does and answers to the same switch. Somebody who has turned
	// the arithmetic off is typing music whose bars are their own business
	// — the twelve-in-the-time-of-eight of the Altes among it — and a rest
	// they did not write, appearing at the end of the bar because they
	// grouped some notes, is the program correcting them again. Reported
	// as "it adds rests that are not written in the score", which is
	// exactly what it was.
	if before.Cmp(after) > 0 {
		freed = before.Sub(after).Reduced()
		if TrackBarlines() {
			extra = append(extra, restsGivenBack(freed, n, normal)...)
		}
	}

	// The group closes on its last member, which is the last of the rests it
	// was padded with, or the last of the freed rests where those turned out
	// to belong to the group as well.
	closeOn := v.Elements[last]
	for _, el := range extra {
		mk := marksOfElement(el)
		if mk == nil || mk.Tuplet == nil {
			break
		}
		closeOn = el
	}
	marksOfElement(closeOn).Tuplet.Stop = true

	if len(extra) > 0 {
		tail := append([]model.Element{}, v.Elements[last+1:]...)
		v.Elements = append(append(v.Elements[:last+1:last+1], extra...), tail...)
	}
	spill(p, measIdx)
	return normal, members, freed, nil
}

// handleTuplet turns the note or rest at the cursor into a group of n played
// in its time, or takes a group back to the single note it was made from.
//
// This is the operation that keeps a bar full. The other way round — writing
// three notes and then calling them a triplet — leaves the bar a beat short
// and the person who asked wondering what they did wrong.
func (a *App) handleTuplet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}
	n, _ := strconv.Atoi(q.Get("n"))

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if len(v.Elements) == 0 {
		http.Error(w, "the measure is empty", http.StatusBadRequest)
		return
	}
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	idx := at - 1

	// Taking the figure off and keeping the notes.
	//
	// What somebody clicking the 12 over twelve notes means: that run is
	// not a tuplet, leave the notes where they are. It is not what "back
	// to one note" means — that collapses the group into the single note
	// it would have been made from, which is right for a group that was
	// made from one note and throws away eleven of them otherwise.
	if q.Get("free") == "1" {
		freed := freeTuplet(v, idx)
		if freed == 0 {
			http.Error(w, "that note is not in a tuplet", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{
			"freed": freed, "measure": measIdx, "at": idx + 1,
		})
		return
	}

	if n <= 1 {
		changed := ungroupTuplet(v, idx)
		if changed == 0 {
			http.Error(w, "that note is not in a tuplet", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"removed": changed, "measure": measIdx, "at": idx + 1})
		return
	}
	if n > 16 {
		http.Error(w, "a group of more than sixteen is not a tuplet", http.StatusBadRequest)
		return
	}

	// Several notes picked up: those notes are the group. One note picked
	// up falls through to the form below, which makes the group out of it —
	// asking for a triplet on a single quarter still means three notes in
	// the time of that quarter.
	if targets, perr := parseTargets(q.Get("sel")); perr == nil && len(targets) > 1 {
		normal, members, freed, err := tupletFromSelection(p, measIdx, targets, n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{
			"tuplet": n, "normal": normal, "members": members,
			"freed": freed.String(), "kept": len(targets),
			"measure": measIdx, "at": targets[0].index + 1,
		})
		return
	}

	if mk := marksOfElement(v.Elements[idx]); mk != nil && mk.Tuplet != nil {
		http.Error(w, "that note is already in a tuplet", http.StatusBadRequest)
		return
	}

	span := v.Elements[idx].Duration()
	normal, ok := tupletFit(n, span)
	if !ok {
		http.Error(w, fmt.Sprintf(
			"a group of %d will not divide %s into noteheads", n, span.String()),
			http.StatusBadRequest)
		return
	}
	each := span.Scale(1, n)

	members := make([]model.Element, 0, n)
	for i := 0; i < n; i++ {
		var el model.Element
		if i == 0 {
			// The note that was there keeps its pitch and its marks: asking
			// for a triplet on a note is asking for that note three times
			// the speed, not for it to be thrown away.
			el = cloneElement(v.Elements[idx])
			setDuration(el, each)
		} else {
			el = &model.Rest{Dur: each}
		}
		mk := marksOfElement(el)
		if mk == nil {
			continue
		}
		mk.Tuplet = &model.Tuplet{
			Actual: n, Normal: normal,
			Start: i == 0, Stop: i == n-1,
		}
		members = append(members, el)
	}

	rest := append([]model.Element{}, v.Elements[idx+1:]...)
	v.Elements = append(append(v.Elements[:idx:idx], members...), rest...)
	spill(p, measIdx)

	writeJSON(w, map[string]any{
		"tuplet": n, "normal": normal, "measure": measIdx, "at": idx + 1,
	})
}

// ungroupTuplet takes the group the element at idx belongs to back to one
// note of the time it filled.
// tupletBounds is the whole group the element at idx belongs to, which is
// however far the tuplet marks run either way.
func tupletBounds(v *model.Voice, idx int) (first, last int, ok bool) {
	if idx < 0 || idx >= len(v.Elements) {
		return 0, 0, false
	}
	if mk := marksOfElement(v.Elements[idx]); mk == nil || mk.Tuplet == nil {
		return 0, 0, false
	}
	first, last = idx, idx
	for first > 0 {
		m := marksOfElement(v.Elements[first-1])
		if m == nil || m.Tuplet == nil {
			break
		}
		first--
		if m.Tuplet.Start {
			break
		}
	}
	for last+1 < len(v.Elements) {
		m := marksOfElement(v.Elements[last])
		if m != nil && m.Tuplet != nil && m.Tuplet.Stop {
			break
		}
		m = marksOfElement(v.Elements[last+1])
		if m == nil || m.Tuplet == nil {
			break
		}
		last++
	}
	return first, last, true
}

// spanOf is what a run of elements lasts.
//
// From nothing-at-all, which is 0/1: the zero value of a Duration has a zero
// denominator and adding to it gives 0/0.
func spanOf(els []model.Element) model.Duration {
	span := model.Duration{Num: 0, Den: 1}
	for _, el := range els {
		span = span.Add(el.Duration())
	}
	return span
}

// freeTuplet takes a whole group out of its tuplet, leaving the notes
// exactly as they are drawn, and says how many it freed.
//
// For when a tuplet is about to lose a note. Three in the time of two with
// one of them deleted is not a triplet any more — it is two notes with a
// 3 printed over them and no arithmetic that holds — and the state it left
// behind was worse than untidy. A member's stored length is its written
// length scaled by the figure, so a thirty-second in twelve-in-the-time-of
// -eight is a forty-eighth: a length no notehead spells. Those orphans
// could not be dotted ("that note's length is not one a dot can change"),
// could not be made into a tuplet again ("part of that selection is
// already in a tuplet"), and said both about notes with no group over them
// to see. Reported as exactly that, twice, on the same bar of the Altes.
//
// What is restored is the written length, so nothing on the page moves:
// a member drawn as a thirty-second goes on being a thirty-second. Only
// the arithmetic changes, and it changes from false to true.
func freeTuplet(v *model.Voice, idx int) int {
	first, last, ok := tupletBounds(v, idx)
	if !ok {
		return 0
	}
	freed := 0
	for i := first; i <= last; i++ {
		mk := marksOfElement(v.Elements[i])
		if mk == nil || mk.Tuplet == nil {
			continue
		}
		if actual, normal, ok := mk.Tuplet.Ratio(); ok {
			setDuration(v.Elements[i],
				v.Elements[i].Duration().Scale(actual, normal).Reduced())
		}
		mk.Tuplet = nil
		freed++
	}
	return freed
}

// freeTupletAt frees the group around an element that is about to be
// removed, where there is one. Called before the removal, since the group
// is found by walking the notes either side of it.
func freeTupletAt(v *model.Voice, idx int) {
	if idx < 0 || idx >= len(v.Elements) {
		return
	}
	if mk := marksOfElement(v.Elements[idx]); mk != nil && mk.Tuplet != nil {
		freeTuplet(v, idx)
	}
}

func ungroupTuplet(v *model.Voice, idx int) int {
	first, last, ok := tupletBounds(v, idx)
	if !ok {
		return 0
	}

	span := spanOf(v.Elements[first : last+1])
	kept := cloneElement(v.Elements[first])
	setDuration(kept, span)
	if m := marksOfElement(kept); m != nil {
		m.Tuplet = nil
	}
	v.Elements = append(v.Elements[:first:first],
		append([]model.Element{kept}, v.Elements[last+1:]...)...)
	return last - first + 1
}

// setDuration writes a duration onto whichever kind of element it is.
func setDuration(el model.Element, d model.Duration) {
	switch e := el.(type) {
	case *model.Note:
		e.Dur = d
	case *model.Rest:
		e.Dur = d
	case *model.Chord:
		e.Dur = d
		for _, n := range e.Notes {
			n.Dur = d
		}
	}
}

// handleTremolo writes, or takes off, the slashes that stand for a repeated
// note.
func (a *App) handleTremolo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	strokes, _ := strconv.Atoi(q.Get("strokes"))
	buzz := q.Get("buzz") == "1"
	if !buzz && (strokes < 0 || strokes > 5) {
		http.Error(w, "a tremolo has one to five slashes", http.StatusBadRequest)
		return
	}

	// A two-note tremolo is one mark on two notes, so it is not a question
	// about the selection's marks but about where in the bar the notes are:
	// the one chosen and the one written after it.
	if q.Get("pair") == "1" {
		n, err := a.pairTremolo(q, strokes, buzz)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]int{"changed": n})
		return
	}

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.lock()
	for _, m := range marks {
		if strokes == 0 && !buzz {
			m.Tremolo = nil
			continue
		}
		m.Tremolo = &model.Tremolo{Strokes: strokes, Buzz: buzz}
	}
	// Taking a two-note tremolo off one of its notes leaves the other
	// holding half a mark. Nothing is drawn, because half a pair is not a
	// pair, and the note plays on as though it had never been marked — but
	// the mark is still in the file, and the next thing written beside it
	// pairs with it again. So "no tremolo" means the whole of it.
	n := len(marks) + clearHalfTremolos(a.score)
	a.mu.Unlock()
	writeJSON(w, map[string]int{"changed": n})
}

// clearHalfTremolos takes off every two-note tremolo mark whose partner is
// not there, and says how many it took off.
func clearHalfTremolos(s *model.Score) int {
	n := 0
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for i, el := range v.Elements {
					mk := marksOfElement(el)
					if mk == nil || !mk.Tremolo.Paired() {
						continue
					}
					var other *model.Marks
					if mk.Tremolo.Start && i+1 < len(v.Elements) {
						other = marksOfElement(v.Elements[i+1])
						if other != nil && other.Tremolo.Paired() && other.Tremolo.Stop {
							continue
						}
					}
					if mk.Tremolo.Stop && i > 0 {
						other = marksOfElement(v.Elements[i-1])
						if other != nil && other.Tremolo.Paired() && other.Tremolo.Start {
							continue
						}
					}
					mk.Tremolo = nil
					n++
				}
			}
		}
	}
	return n
}

// pairTremolo marks the chosen note and the one after it as a two-note
// tremolo.
//
// Both ends or neither. Half a pair is a mark that draws nothing and plays
// as an ordinary note, which is worse than being told there is nowhere to
// put it.
func (a *App) pairTremolo(q url.Values, strokes int, buzz bool) (int, error) {
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}

	// Two notes chosen, which is how the mark is asked for with the mouse:
	// pick up the pair it goes between and say so. One note works too — it
	// pairs with the note after it — but having to know that is having to
	// know how the file is written rather than what the mark means.
	if sel := q.Get("sel"); sel != "" {
		targets, err := parseTargets(sel)
		if err != nil {
			return 0, err
		}
		if len(targets) > 2 {
			return 0, editError("a two-note tremolo goes between two notes, " +
				"and there are " + strconv.Itoa(len(targets)) + " selected")
		}
		first := targets[0]
		if len(targets) == 2 {
			second := targets[1]
			if second.index < first.index {
				first, second = second, first
			}
			if first.part != second.part || first.measure != second.measure {
				return 0, editError("the two notes of a tremolo are in the same bar")
			}
			if second.index != first.index+1 {
				return 0, editError("the two notes of a tremolo are next to each other")
			}
		}
		partIdx, measIdx, at = first.part, first.measure, first.index+1
	}

	a.lock()
	defer a.mu.Unlock()

	_, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		return 0, err
	}
	v := m.Voices[0]
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	i := at - 1
	if i < 0 || i+1 >= len(v.Elements) {
		return 0, editError("a two-note tremolo needs the note after this one, " +
			"and this is the last in the bar")
	}
	first, second := marksOfElement(v.Elements[i]), marksOfElement(v.Elements[i+1])
	if first == nil || second == nil {
		return 0, editError("both of them have to be notes")
	}
	if v.Elements[i].Duration() != v.Elements[i+1].Duration() {
		return 0, editError("the two notes of a tremolo are written the same length")
	}
	if strokes == 0 && !buzz {
		first.Tremolo, second.Tremolo = nil, nil
		return 2, nil
	}
	first.Tremolo = &model.Tremolo{Strokes: strokes, Buzz: buzz, Start: true}
	second.Tremolo = &model.Tremolo{Strokes: strokes, Buzz: buzz, Stop: true}
	return 2, nil
}

// handleDot lengthens the selected note by half, or takes that back.
//
// The dot is part of how long a note is: a dotted half is a half and a
// quarter, three quarters of a whole note. So this changes the duration
// rather than adding a mark, and the engraver finds the dot again on its
// way out — it draws whatever notehead and dots spell the length it is
// given.
//
// On a note that is already dotted it takes the dot off, because the thing
// in front of you is one button and pressing it twice should leave the note
// as it was.
func (a *App) handleDot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if len(v.Elements) == 0 {
		http.Error(w, "there is nothing there to dot", http.StatusBadRequest)
		return
	}
	if at <= 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	el := v.Elements[at-1]

	// A note inside a tuplet is written as one thing and lasts another: a
	// thirty-second in twelve-in-the-time-of-eight is stored as a
	// forty-eighth, because that is what it plays for. The dot belongs to
	// the written note — it is drawn beside that notehead and read as
	// half of it again — so the group's figure is divided out, the dot
	// added to what is written, and the figure put back.
	//
	// Without that, a forty-eighth was a length no notehead spells and the
	// dot was refused: "that note's length is not one a dot can change",
	// on a note a reader would call an ordinary thirty-second. Worse where
	// the arithmetic happened to land on a value that does spell — a
	// dotted quarter in a triplet is stored as a quarter — and the dot
	// would have been added to the wrong note and come out as something
	// else entirely.
	num, den := 1, 1
	if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
		if actual, normal, ok := mk.Tuplet.Ratio(); ok {
			num, den = actual, normal
		}
	}
	written := el.Duration().Scale(num, den).Reduced()

	// What the note is written as now. A length that no notehead spells at
	// all has no dot to add or take away.
	value, dots, ok := written.Written()
	if !ok {
		http.Error(w, "that note's length is not one a dot can change",
			http.StatusBadRequest)
		return
	}
	// Round and round: none, one, two, three, and none again. Pressing the
	// same button again is how anybody expects to reach a second dot, and
	// this was a toggle between none and one — so the only way to write a
	// double-dotted note was not to.
	plain := value.Duration()
	want := dots + 1
	if want > maxWrittenDots {
		want = 0
	}
	next := plain
	if want > 0 {
		next = dotted(plain, strconv.Itoa(want))
	}
	setDuration(el, next.Scale(den, num).Reduced())

	// What no longer fits the bar moves into the next one, as when writing.
	spill(p, measIdx)
	measure, idx := findElement(p, measIdx, el)
	writeJSON(w, map[string]any{
		// The written length, not the played one: it is the note on the
		// page that has just gained a dot, and inside a tuplet the two are
		// different numbers.
		"dots": want,
		"dur":  next.String(), "measure": measure, "at": idx,
	})
}

/*
 * Changing the length of notes already written.
 *
 * The note values in the palette are tools: they choose what the next click
 * on the staff writes. That is what they are for and it is not what somebody
 * who has just selected four sixteenths and noticed they should have been
 * thirty-seconds is asking for — they are asking for these notes to change,
 * and clicking the thirty-second did nothing at all, because nothing in
 * gscore could change the length of a note that was already there.
 */

// handleDuration rewrites the length of everything selected.
func (a *App) handleDuration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	value, ok := durationNames[q.Get("value")]
	if !ok {
		http.Error(w, "unknown note length "+q.Get("value"), http.StatusBadRequest)
		return
	}
	dots := 0
	if v := q.Get("dots"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 3 {
			http.Error(w, "a note carries up to three dots", http.StatusBadRequest)
			return
		}
		dots = n
	}
	want := value
	for i := 0; i < dots; i++ {
		want = dotted(want, "1")
	}

	targets, err := parseTargets(q.Get("sel"))
	if err != nil || len(targets) == 0 {
		http.Error(w, "select the notes to change first", http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()

	// Bar by bar, because changing a length changes where everything after
	// it falls and the bar has to be reflowed once the whole of it is done.
	// Reflowing after each note would move the notes still to be changed out
	// from under the places the selection names them by.
	// The elements themselves, kept so that they can be found again after
	// the bars reflow. Making notes longer pushes what no longer fits into
	// the following bars, so by the time this returns half of them are no
	// longer where the selection said they were — and a second change asked
	// for on the same selection reached only the ones that had not moved.
	type moved struct {
		part int
		from int
		el   model.Element
	}
	var done []moved
	touched := map[[2]int]bool{}
	// The tuplet groups a note in the selection belongs to, and what each
	// one lasted before anything was changed. A group is read against its
	// own arithmetic, so once its notes are shorter it gives time back and
	// the bar needs a rest for it — the same thing that happens when the
	// group is first made.
	type group struct {
		part, measure, first, last int
		actual, normal             int
		v                          *model.Voice
		was                        model.Duration
	}
	var groups []group
	seen := map[[4]int]bool{}
	for _, t := range targets {
		_, m, err := a.locate(t.part, t.measure)
		if err != nil {
			continue // a stale selection is not worth failing over
		}
		for _, v := range m.Voices {
			if v == nil || t.index < 0 || t.index >= len(v.Elements) {
				continue
			}
			el := v.Elements[t.index]
			// A note inside a tuplet keeps its place in the group: what
			// changes is the notehead it shows, and the group's figure says
			// what that notehead is played as. Giving it the plain length
			// would have made a triplet eighth last an eighth, so the group
			// no longer added up — which is why this used to refuse, and
			// why selecting the notes of a triplet and asking for a shorter
			// one did nothing at all.
			if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
				num, den, ok := mk.Tuplet.Ratio()
				if !ok {
					continue
				}
				if first, last, ok := tupletBounds(v, t.index); ok {
					key := [4]int{t.part, t.measure, first, last}
					if !seen[key] {
						seen[key] = true
						groups = append(groups, group{
							part: t.part, measure: t.measure,
							first: first, last: last, v: v,
							actual: num, normal: den,
							was: spanOf(v.Elements[first : last+1]),
						})
					}
				}
				setDuration(el, want.Scale(den, num).Reduced())
			} else {
				setDuration(el, want)
			}
			done = append(done, moved{part: t.part, from: t.measure, el: el})
			touched[[2]int{t.part, t.measure}] = true
			break // the first voice that has an element at this index
		}
	}

	// What the groups gave back, spelled as rests after each of them. Last
	// first, so that inserting after one group does not move the next one's
	// place out from under it.
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].measure != groups[j].measure {
			return groups[i].measure > groups[j].measure
		}
		return groups[i].first > groups[j].first
	})
	for _, g := range groups {
		if g.last >= len(g.v.Elements) {
			continue
		}
		freed := g.was.Sub(spanOf(g.v.Elements[g.first : g.last+1])).Reduced()
		if freed.Cmp(model.Duration{Num: 0, Den: 1}) <= 0 {
			continue
		}
		rests := restsGivenBack(freed, g.actual, g.normal)
		if len(rests) == 0 {
			continue
		}
		at := g.last + 1
		tail := append([]model.Element{}, g.v.Elements[at:]...)
		g.v.Elements = append(append(g.v.Elements[:at:at], rests...), tail...)
		// Where those rests joined the group, the group now closes on the
		// last of them instead of on the note it closed on before.
		if mk := marksOfElement(rests[len(rests)-1]); mk != nil && mk.Tuplet != nil {
			if was := marksOfElement(g.v.Elements[g.last]); was != nil && was.Tuplet != nil {
				was.Tuplet.Stop = false
			}
			mk.Tuplet.Stop = true
		}
		touched[[2]int{g.part, g.measure}] = true
	}
	if len(done) == 0 {
		http.Error(w, "nothing in the selection has a length to change",
			http.StatusBadRequest)
		return
	}

	// What no longer fits the bar moves into the next one, as when writing.
	for at := range touched {
		if p, _, err := a.locate(at[0], at[1]); err == nil {
			spill(p, at[1])
		}
	}

	// Where each of them ended up, so that what was selected still is.
	now := make([]string, 0, len(done))
	for _, d := range done {
		p, _, err := a.locate(d.part, d.from)
		if err != nil {
			continue
		}
		measure, after := findElement(p, d.from, d.el)
		if after <= 0 {
			continue
		}
		now = append(now, fmt.Sprintf("%d:%d:%d", d.part, measure, after-1))
	}
	writeJSON(w, map[string]any{
		"status": "ok", "changed": len(done), "now": now,
	})
}

/*
 * Ending a bar where you say.
 *
 * With the barlines kept to the meter, a bar ends when it is full and there
 * is nothing to decide. With them left loose — see TrackBarlines — nothing
 * ends a bar at all, and typing in a piece whose bars do not add up needs
 * some way of saying "this one ends here". Adding a measure appends an empty
 * one at the end of the piece, which is a different thing entirely.
 *
 * The split happens at the same moment in every staff, not at the same
 * element: measures are lined up across the staves by their position in the
 * list, so splitting one staff's bar and not the others would leave the
 * barlines of a piano part disagreeing with each other from there to the end
 * of the piece.
 */

// handleSplitMeasure ends the measure at the cursor, moving what follows into
// a new measure after it.
func (a *App) handleSplitMeasure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, _ := strconv.Atoi(q.Get("at"))

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	if at <= 0 || at >= len(v.Elements) {
		http.Error(w, "put the cursor where the bar should end", http.StatusBadRequest)
		return
	}

	// Where the cursor is, in time rather than in elements.
	offset := model.Duration{Num: 0, Den: 1}
	for _, el := range v.Elements[:at] {
		offset = offset.Add(el.Duration())
	}
	_ = p

	for _, part := range a.score.Parts {
		if measIdx >= len(part.Measures) {
			continue
		}
		src := part.Measures[measIdx]
		rest := &model.Measure{RightBarline: src.RightBarline}
		// The bar that is left keeps a plain barline; whatever closed it —
		// a repeat, a double bar — belongs to the end of what was written,
		// which is now the second half.
		src.RightBarline = model.BarSingle
		for _, voice := range src.Voices {
			kept := &model.Voice{}
			moved := &model.Voice{}
			cursor := model.Duration{Num: 0, Den: 1}
			for _, el := range voice.Elements {
				if cursor.Cmp(offset) >= 0 {
					moved.Elements = append(moved.Elements, el)
				} else {
					kept.Elements = append(kept.Elements, el)
				}
				cursor = cursor.Add(el.Duration())
			}
			voice.Elements = kept.Elements
			rest.Voices = append(rest.Voices, moved)
		}
		if len(rest.Voices) == 0 {
			rest.Voices = []*model.Voice{{}}
		}
		part.Measures = append(part.Measures, nil)
		copy(part.Measures[measIdx+2:], part.Measures[measIdx+1:])
		part.Measures[measIdx+1] = rest
	}
	writeJSON(w, map[string]any{"status": "ok", "measure": measIdx + 1})
}

// handleJoinMeasure takes a barline away, making the bar and the one after it
// into one bar.
//
// Taking a barline away is not the same as hiding one: an invisible barline
// still ends a bar, and the bars on either side of it are still counted
// separately and still have to add up on their own. This is the other thing,
// and the undoing of "Barline here" — the two bars become one, everything
// after it is renumbered, and nothing is moved.
//
// Nothing is reflowed afterwards, with the barlines kept to the meter or
// not. Joining two full bars makes one that is twice as long, and that is
// what was asked for; pushing the second half straight back out again would
// make the menu entry look as though it had done nothing.
func (a *App) handleJoinMeasure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	measIdx, err := strconv.Atoi(r.URL.Query().Get("measure"))
	if err != nil || measIdx < 0 {
		http.Error(w, "which barline?", http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()

	last := true
	for _, p := range a.score.Parts {
		if measIdx+1 < len(p.Measures) {
			last = false
		}
	}
	if last {
		http.Error(w, "that is the last barline in the piece, and a piece ends somewhere",
			http.StatusBadRequest)
		return
	}

	// A change of key or meter written at that barline would end up in the
	// middle of the joined bar, where it means something else. Better to say
	// so than to move it somewhere it was never written.
	for _, p := range a.score.Parts {
		if measIdx+1 >= len(p.Measures) {
			continue
		}
		switch next := p.Measures[measIdx+1]; {
		case next.Key != nil:
			http.Error(w, "the key changes at that barline; take the key change away first",
				http.StatusBadRequest)
			return
		case next.Time != nil:
			http.Error(w, "the meter changes at that barline; take the meter change away first",
				http.StatusBadRequest)
			return
		}
	}

	for _, p := range a.score.Parts {
		if measIdx+1 >= len(p.Measures) {
			continue
		}
		cur, next := p.Measures[measIdx], p.Measures[measIdx+1]
		for i, v := range next.Voices {
			for len(cur.Voices) <= i {
				cur.Voices = append(cur.Voices, &model.Voice{})
			}
			cur.Voices[i].Elements = append(cur.Voices[i].Elements, v.Elements...)
		}
		// What closed the second bar now closes the joined one.
		cur.RightBarline = next.RightBarline
		if cur.Ending == nil {
			cur.Ending = next.Ending
		}
		if next.Tempo != nil && cur.Tempo == nil {
			cur.Tempo = next.Tempo
		}
		p.Measures = append(p.Measures[:measIdx+1], p.Measures[measIdx+2:]...)
	}
	writeJSON(w, map[string]any{"status": "ok", "measure": measIdx})
}

// handleClearMeasure empties a bar, leaving the bar itself.
//
// Not the same as deleting a passage, which takes the notes out and lets
// what follows close the gap. A bar picked up as a bar is a container, and
// emptying it leaves an empty bar of the same length where it was — the
// barlines on either side of it do not move and nothing after it is
// renumbered.
//
// Every voice of it, because every voice of it is in the bar. The bulk
// delete works on the first voice only, which is right for a passage picked
// up note by note and wrong for "take what is in this bar out".
func (a *App) handleClearMeasure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))

	a.lock()
	defer a.mu.Unlock()

	_, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	removed := 0
	for _, v := range m.Voices {
		if v == nil {
			continue
		}
		removed += len(v.Elements)
		v.Elements = nil
	}
	// One empty voice to write into, so the bar can be typed in again.
	if len(m.Voices) == 0 {
		m.Voices = []*model.Voice{{}}
	} else {
		m.Voices = m.Voices[:1]
	}
	writeJSON(w, map[string]any{"status": "ok", "removed": removed})
}

// restEmptyMeasures puts a bar's worth of rest into every bar that has
// nothing in it.
//
// A bar with nothing written in it is a bar of silence, and silence is
// written down: an empty staff says "I have not got to this yet", a whole
// rest says "do not play". Every engraver draws the second one, and a score
// that shows neither leaves the reader to guess which it meant.
//
// Put in the music rather than drawn over the gap, so that it is a thing
// like any other: it can be picked up, dotted, moved, and taken out again by
// anybody who wants the bar left bare.
//
// Bars with no meter to measure are left alone. Without one there is no
// answer to how long the rest should be, and guessing is worse than the gap.
func restEmptyMeasures(s *model.Score) int {
	filled := 0
	for _, p := range s.Parts {
		for mi, m := range p.Measures {
			bar := p.TimeAt(mi).BarDuration()
			if bar.IsZero() {
				continue
			}
			used := false
			for _, v := range m.Voices {
				if v != nil && len(v.Elements) > 0 {
					used = true
					break
				}
			}
			if used {
				continue
			}
			if len(m.Voices) == 0 {
				m.Voices = []*model.Voice{{}}
			}
			m.Voices[0].Elements = []model.Element{&model.Rest{Dur: bar}}
			filled++
		}
	}
	return filled
}

// hasAccidental says a typed pitch carried one of its own, so the key
// signature is not to overrule it. "c" takes the key's C sharp; "cn" and
// "c#" mean what they say.
func hasAccidental(name string) bool {
	return writtenAccidental(name) != model.NoAccidental
}

// writtenAccidental is the sign a typed pitch spells, or none where it
// spells none.
//
// "b#" is a B with a sharp written in front of it, not merely a B that
// sounds a semitone higher. Reading only the alteration decided how the
// note sounded and left nothing to decide how it looked, so a sharp typed
// in was saved into the file and never drawn: the page said B and the music
// played B sharp.
//
// A natural counts. It alters nothing, and saying so out loud is the whole
// reason anybody writes one.
func writtenAccidental(name string) model.Accidental {
	alter, any := 0, false
scan:
	for i := 1; i < len(name); i++ {
		switch name[i] {
		case '#', 's', 'S':
			alter, any = alter+1, true
		case 'b', 'B':
			alter, any = alter-1, true
		case 'x', 'X':
			alter, any = alter+2, true
		case 'n', 'N':
			any = true
		default:
			break scan
		}
	}
	if !any {
		return model.NoAccidental
	}
	switch alter {
	case -2:
		return model.DoubleFlat
	case -1:
		return model.Flat
	case 0:
		return model.Natural
	case 1:
		return model.Sharp
	case 2:
		return model.DoubleSharp
	}
	return model.NoAccidental
}

// lastPitch is the note before the one being written, which is what a bare
// letter or an interval is counted from.
//
// The note before in the bar, or in the bar before it where this is the
// first thing here. Where there is no note at all — an empty score, the very
// first thing anybody types — the staff's middle line stands in, so that a
// tune typed into an empty bar starts somewhere reasonable rather than
// nowhere.
func lastPitch(p *model.Part, m *model.Measure, at int, fallback model.Pitch) model.Pitch {
	look := func(v *model.Voice, upTo int) (model.Pitch, bool) {
		if v == nil {
			return model.Pitch{}, false
		}
		if upTo < 0 || upTo > len(v.Elements) {
			upTo = len(v.Elements)
		}
		for i := upTo - 1; i >= 0; i-- {
			switch e := v.Elements[i].(type) {
			case *model.Note:
				return e.Pitch, true
			case *model.Chord:
				if len(e.Notes) > 0 {
					return e.Notes[len(e.Notes)-1].Pitch, true
				}
			}
		}
		return model.Pitch{}, false
	}
	if len(m.Voices) > 0 {
		if got, ok := look(m.Voices[0], at); ok {
			return got
		}
	}
	// Back through the bars before this one.
	for i := len(p.Measures) - 1; i >= 0; i-- {
		if p.Measures[i] == m {
			for j := i - 1; j >= 0; j-- {
				if len(p.Measures[j].Voices) == 0 {
					continue
				}
				if got, ok := look(p.Measures[j].Voices[0], -1); ok {
					return got
				}
			}
			break
		}
	}
	return fallback
}

// handleScripts reads and saves the programs kept with the score.
//
// A piece built from a process is not fully written down by its notes: the
// notes are the output and the process is the thought, and a composer who
// comes back a year later wants the thought. So the scripts travel in the
// file.
//
// What comes back is text, and the editor never runs it on its own. A score
// that could execute code on being opened is a document that can attack the
// person who opens it, which is the mistake Word made and spent a decade
// undoing. The console shows what the file carries, and a person reads it
// and decides.
func (a *App) handleScripts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.mu.RLock()
		out := append([]model.Script(nil), a.score.Scripts...)
		a.mu.RUnlock()
		if out == nil {
			out = []model.Script{}
		}
		writeJSON(w, map[string]any{"scripts": out})
		return
	case http.MethodPost:
		// Below.
	default:
		http.Error(w, "GET to read them, POST to replace them",
			http.StatusMethodNotAllowed)
		return
	}

	var in struct {
		Scripts []model.Script `json:"scripts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "a list of scripts: "+err.Error(), http.StatusBadRequest)
		return
	}
	seen := map[string]bool{}
	for i, sc := range in.Scripts {
		// A name, always. A score may carry several and they are the only
		// thing that tells them apart: an unnamed script is one nobody can
		// ask for, and two unnamed scripts are two nobody can tell apart.
		name := strings.TrimSpace(sc.Name)
		if name == "" {
			http.Error(w, "script "+strconv.Itoa(i+1)+" has no name, and a "+
				"script nobody can name is a script nobody can find again",
				http.StatusBadRequest)
			return
		}
		if seen[name] {
			http.Error(w, "there are two scripts called "+name,
				http.StatusBadRequest)
			return
		}
		seen[name] = true
		in.Scripts[i].Name = name
		if len(sc.Body) > maxScript {
			http.Error(w, name+" is too long to keep in a score",
				http.StatusBadRequest)
			return
		}
	}

	a.lock()
	defer a.mu.Unlock()
	a.score.Scripts = in.Scripts
	writeJSON(w, map[string]int{"scripts": len(in.Scripts)})
}

// maxScript is how long one script may be. Generous for a program and small
// for a file: a score is music with a note attached, not a source tree.
const maxScript = 64 * 1024

// Everything written on a note, not only its pitch and length.
//
// It was those two and nothing else, which made the console a thing
// that writes music and cannot read it: a script could generate a row
// but could not find the bars with no dynamic in them, or the lyric
// with no hyphen, or the note the horn goes above its range on. Most
// of what anybody wants a script for is looking rather than writing.
//
// Flat, and every field left out when it is not there. A script walks
// this with ordinary JavaScript — the power is in the language — so
// what it wants is plain data and not an object model to learn.
type noteContents struct {
	Pitch  string `json:"pitch"`
	Length int    `json:"length"`
	Dots   int    `json:"dots,omitempty"`
	Rest   bool   `json:"rest,omitempty"`
	// Chord carries the other notes of a chord, the lowest first.
	Chord []string `json:"chord,omitempty"`
	// MIDI is the pitch as a number, and ChordMIDI the rest of a chord
	// the same way.
	//
	// Given rather than left to the script, because every script that
	// compares pitches would otherwise parse "Bb3" itself — the letter,
	// any number of accidentals, the octave — and they would each get
	// the double flats wrong in their own way. Comparing two notes is
	// the commonest thing an analysis does.
	MIDI      int   `json:"midi,omitempty"`
	ChordMIDI []int `json:"chordMidi,omitempty"`
	// Voice is which line of the staff it belongs to, counted from 1.
	Voice int `json:"voice,omitempty"`
	// Where it falls in its bar and how long it sounds, as fractions
	// of a whole note. A script asking "what is on beat three" should
	// not have to add the lengths up itself.
	At    string `json:"at,omitempty"`
	Beats string `json:"beats,omitempty"`

	// What is written on it.
	Accidental    string   `json:"accidental,omitempty"`
	Articulations []string `json:"articulations,omitempty"`
	Dynamic       string   `json:"dynamic,omitempty"`
	Stem          string   `json:"stem,omitempty"`
	Lyrics        []string `json:"lyrics,omitempty"`
	Text          []string `json:"text,omitempty"`
	ChordSymbol   string   `json:"chordSymbol,omitempty"`
	Fingering     []string `json:"fingering,omitempty"`

	// Where a mark begins and ends. A script looking for unslurred
	// phrases wants the ends, not a slur object.
	TieStart  bool   `json:"tieStart,omitempty"`
	TieStop   bool   `json:"tieStop,omitempty"`
	SlurStart bool   `json:"slurStart,omitempty"`
	SlurStop  bool   `json:"slurStop,omitempty"`
	Hairpin   string `json:"hairpin,omitempty"`
	Octave    string `json:"octave,omitempty"`
	Trill     bool   `json:"trill,omitempty"`
	Grace     int    `json:"grace,omitempty"`
	Tremolo   int    `json:"tremolo,omitempty"`
	// Tuplet is the ratio written over the group — "3:2", "11:6" —
	// on every note of it, so a script need not hunt for the start.
	Tuplet string `json:"tuplet,omitempty"`
}

// What is written on a bar rather than on a note in it.
type barContents struct {
	// Number is what is printed over the bar, which is not the same as
	// where it sits in the score: an upbeat is not counted, and a
	// movement starts again at one.
	Number  int            `json:"number"`
	Clef    string         `json:"clef,omitempty"`
	Key     string         `json:"key,omitempty"`
	Time    string         `json:"time,omitempty"`
	Barline string         `json:"barline,omitempty"`
	Marker  string         `json:"marker,omitempty"`
	Section string         `json:"section,omitempty"`
	Tempo   string         `json:"tempo,omitempty"`
	Ending  []int          `json:"ending,omitempty"`
	Repeat  int            `json:"repeat,omitempty"`
	Notes   []noteContents `json:"notes"`
}
type staffContents struct {
	Name string `json:"name"`
	// What the staff is and how it reads, which a script checking a
	// player's range has to know before it can say anything useful.
	Instrument string `json:"instrument,omitempty"`
	Clef       string `json:"clef,omitempty"`
	Key        string `json:"key,omitempty"`
	Time       string `json:"time,omitempty"`
	// Transpose is how far the written pitch is from the sounding
	// one, in semitones: -2 for a B flat clarinet.
	Transpose int `json:"transpose,omitempty"`
	// Bars holds the notes alone, as it always did, so that scripts
	// written against the old shape go on working. Measures holds the
	// same notes and everything written on the bar around them.
	Bars     [][]noteContents `json:"bars"`
	Measures []barContents    `json:"measures"`
}

// handleContents says what is written in the score, bar by bar.
//
// The one thing the editor could not be asked. Everything else answers a
// question about where the ink went or how the music sounds; this answers
// "what is written here", which is what a script needs before it can copy a
// phrase, transpose it, invert it or answer it in another voice.
//
// The whole score at once, because the console reads it once when a script
// starts and then works from that: the verbs are synchronous so that a
// script reads as music rather than as plumbing, and a synchronous verb
// cannot go and ask.
func (a *App) handleContents(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// What each bar is called, from the engraver, which owns the rule.
	// A script saying "a parallel fifth in bar 41" has to name the bar a
	// player would read.
	numbers := engrave.BarNumbers(a.score)
	// And what each marked bar is called, with the sequence worked out.
	marks := a.score.RehearsalMarks(numbers)

	out := make([]staffContents, 0, len(a.score.Parts))
	for pi, p := range a.score.Parts {
		name := p.Name
		if name == "" {
			name = "Staff " + strconv.Itoa(pi+1)
		}
		st := staffContents{
			Name:       name,
			Instrument: p.Instrument.String(),
			Clef:       p.Clef.String(),
			Key:        keyText(p.Key),
			Time:       p.Time.String(),
			Transpose:  p.Transpose.Chromatic + 12*p.Transpose.OctaveChange,
			Bars:       make([][]noteContents, len(p.Measures)),
			Measures:   make([]barContents, len(p.Measures)),
		}
		for mi, m := range p.Measures {
			bd := barContents{Number: barNumberAt(numbers, mi), Notes: []noteContents{}}
			if m.Clef != nil {
				bd.Clef = m.Clef.String()
			}
			if m.Key != nil {
				bd.Key = keyText(*m.Key)
			}
			if m.Time != nil {
				bd.Time = m.Time.String()
			}
			if m.RightBarline != model.BarSingle {
				bd.Barline = m.RightBarline.String()
			}
			// What is printed, with the sequence worked out: a script
			// reporting "letter C" has to say what a conductor would
			// call, not the empty string a sequenced mark holds.
			bd.Marker, bd.Section, bd.Repeat = marks[mi], m.SectionTitle, m.Repeat
			if m.Tempo != nil {
				bd.Tempo = tempoText(*m.Tempo)
			}
			if m.Ending != nil {
				bd.Ending = append([]int(nil), m.Ending.Numbers...)
			}

			for vi, v := range m.Voices {
				// Where in the bar each element falls, added up as it
				// goes: a script asking what is on beat three should not
				// have to do this itself.
				// One over one, not the zero value: adding to a
				// duration of nought over nought gives nought over
				// nought, so every note came back at the start of the bar.
				at := model.Duration{Num: 0, Den: 1}
				for _, el := range v.Elements {
					d := el.Duration()
					// The length as it is written, which for a note in a
					// tuplet is not the time it sounds: eleven in the time
					// of six are written as eighths and sound as elevenths
					// of six of them. A script reporting "an eighth" is
					// reading the page; beats below is the time.
					written := d
					if mk := marksOn(el); mk != nil {
						written = mk.WrittenDuration(d)
					}
					n := noteContents{
						Length: lengthNumber(written), Dots: dotsOf(written),
						Voice: vi + 1,
						At:    at.String(), Beats: d.String(),
					}
					at = at.Add(d)
					switch e := el.(type) {
					case *model.Rest:
						n.Rest = true
					case *model.Note:
						n.Pitch = e.Pitch.String()
						n.MIDI = e.Pitch.MIDI()
						n.Accidental = accidentalWord(e.Accidental)
					case *model.Chord:
						for i, cn := range e.Notes {
							if i == 0 {
								n.Pitch = cn.Pitch.String()
								n.MIDI = cn.Pitch.MIDI()
								n.Accidental = accidentalWord(cn.Accidental)
								continue
							}
							n.Chord = append(n.Chord, cn.Pitch.String())
							n.ChordMIDI = append(n.ChordMIDI, cn.Pitch.MIDI())
						}
					default:
						continue
					}
					readMarks(&n, marksOn(el))
					bd.Notes = append(bd.Notes, n)
				}
			}
			st.Measures[mi] = bd
			// The notes alone as well, which is the shape scripts were
			// written against before there was anything else to say.
			st.Bars[mi] = bd.Notes
		}
		out = append(out, st)
	}
	writeJSON(w, map[string]any{"staves": out})
}

// lengthNumber is the figure a length is written as — 4 for a quarter note,
// 8 for an eighth — and dotsOf how many dots it carries.
//
// Counted apart, because a dotted quarter is a quarter with a dot and not a
// length of its own: that is how it is written, how it is typed, and how
// every other part of the editor holds it.
func lengthNumber(d model.Duration) int {
	n, _ := plainLength(d)
	return n
}

func dotsOf(d model.Duration) int {
	_, dots := plainLength(d)
	return dots
}

// plainLength undots a length: three halves of a value is that value with a
// dot, and seven quarters of it is that value with two.
func plainLength(d model.Duration) (length, dots int) {
	d = d.Reduced()
	if d.Num == 0 || d.Den == 0 {
		return 0, 0
	}
	// A value with k dots lasts (2^(k+1) - 1) / 2^k of the plain one: a
	// dot adds half, a second dot a quarter. So the plain value is found
	// by multiplying back by the other way round, and the answer is the
	// first k that leaves a length a notehead spells.
	//
	// Undotting one dot at a time looks simpler and is wrong: it divides
	// by three each time, and a double-dotted eighth is seven
	// thirty-seconds, which is not divisible by three. Every
	// double-dotted note in the score was reported as having no length at
	// all — the Altes has one in its second bar.
	for k := 0; k <= 3; k++ {
		pow := 1 << k
		plain := model.Duration{Num: d.Num * pow, Den: d.Den * (pow*2 - 1)}.Reduced()
		if plain.Num == 1 && powerOfTwo(plain.Den) {
			return plain.Den, k
		}
	}
	return 0, 0
}

// powerOfTwo says whether a length is one a notehead spells.
//
// Every written value halves the one before it, so the figure under the
// one is a power of two. A twelfth of a whole note is a third of a
// quarter — a triplet — and there is no notehead for it: what is written
// is a quarter, and the three over the group says the rest. Answering
// "twelve" there is answering with a number that means nothing, which is
// worse than saying there is no answer.
func powerOfTwo(n int) bool {
	return n > 0 && n&(n-1) == 0
}

// handleABC writes a line of ABC notation in at the cursor.
//
// The editor's own way of typing music is the short one — "8abcdef" is six
// quavers — and it is the one to reach for. ABC is the other: a notation a
// great many people already have in their fingers, with a tune book's worth
// of things gscore's line does not say at all, and no reason to make
// somebody who knows it learn a second shorthand to get a phrase in.
//
// Read by the same parser that opens an .abc file, so a phrase types the way
// a file reads. It is given the key and the meter already in force where the
// cursor is: ABC spells its accidentals against the key, and a fragment read
// on its own would come out in C.
func (a *App) handleABC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "could not read that", http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}

	a.lock()
	defer a.mu.Unlock()

	p, m, err := a.locateForWriting(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// In the key and the meter the cursor is standing in, not in ABC's own
	// defaults. A phrase typed into a piece in A major is in A major.
	read, err := abc.Fragment(string(body), p.KeyAt(measIdx), p.TimeAt(measIdx))
	if read == nil {
		http.Error(w, abcTrouble(err), http.StatusBadRequest)
		return
	}

	// Flattened, because where the bars fall is the score's business and
	// not the fragment's. The piece has a meter; writing music into it
	// fills the bar and carries the rest into the next one, which is what
	// happens when the notes are typed one at a time and what somebody
	// expects when they are typed all at once. A bar line in the ABC is
	// then not needed — but it is not ignored either: the kind of line it
	// is follows the note it came after, and is put on whichever bar that
	// note ends up in.
	var add []model.Element
	type closer struct {
		after model.Element
		style model.BarlineStyle
	}
	var closers []closer
	for _, bar := range read.Parts[0].Measures {
		if len(bar.Voices) == 0 || len(bar.Voices[0].Elements) == 0 {
			continue
		}
		for _, el := range bar.Voices[0].Elements {
			add = append(add, cloneElement(el))
		}
		if bar.RightBarline != model.BarSingle && bar.RightBarline != model.BarNone {
			closers = append(closers, closer{add[len(add)-1], bar.RightBarline})
		}
	}
	if len(add) == 0 {
		http.Error(w, "there is no music in that", http.StatusBadRequest)
		return
	}

	v := m.Voices[0]
	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	tail := append([]model.Element{}, v.Elements[at:]...)
	v.Elements = append(append(v.Elements[:at:at], add...), tail...)
	spill(p, measIdx)

	// The barlines, now that it is known which bar each one's note landed
	// in. A line on the last note of the fragment closes the bar it ended
	// in, which is where somebody writing "|:" meant it to go.
	for _, c := range closers {
		if i, _ := findElement(p, measIdx, c.after); i >= 0 {
			p.Measures[i].RightBarline = c.style
		}
	}

	measure, idx := findElement(p, measIdx, add[len(add)-1])
	writeJSON(w, map[string]any{
		"wrote": len(add), "measure": measure, "at": idx,
		// What the reader could not read, if anything. An ABC line with one
		// thing in it gscore does not know is still a line of music.
		"trouble": abcTrouble(err),
	})
}

// abcTrouble says what an ABC read could not manage, in one line.
func abcTrouble(err error) string {
	if err == nil {
		return ""
	}
	var partial format.Partial
	if errors.As(err, &partial) {
		if d := partial.Details(); len(d) > 0 {
			return strings.Join(d, "; ")
		}
	}
	// The parser says "abc: ..." because a file is being named; here the
	// line is the file and the prefix is noise.
	return strings.TrimPrefix(err.Error(), "abc: ")
}

// readMarks copies what is written on an element into what a script sees.
//
// Everything in one place, and every field left out when it is not there:
// a script walks this with ordinary JavaScript, so what it wants is plain
// data rather than an object model to learn.
func readMarks(n *noteContents, m *model.Marks) {
	if m == nil {
		return
	}
	for _, a := range m.Articulations {
		n.Articulations = append(n.Articulations, a.String())
	}
	if m.Dynamic != model.DynamicNone {
		n.Dynamic = m.Dynamic.String()
	}
	if m.Stem != model.StemAuto {
		n.Stem = m.Stem.String()
	}
	for _, l := range m.Lyrics {
		n.Lyrics = append(n.Lyrics, l.Text)
	}
	for _, t := range m.Texts {
		n.Text = append(n.Text, t.Text)
	}
	if m.Chord != nil {
		n.ChordSymbol = m.Chord.String()
	}
	n.TieStart, n.TieStop = m.TieStart, m.TieStop
	n.SlurStart, n.SlurStop = m.SlurStart > 0, m.SlurStop > 0
	if m.Hairpin != nil {
		n.Hairpin = hairpinWord(m.Hairpin)
	}
	if m.Octave != nil {
		n.Octave = octaveWord(m.Octave)
	}
	n.Trill = m.Trill != nil
	if m.Grace != nil {
		n.Grace = len(m.Grace.Notes)
	}
	if m.Tremolo.Beams() > 0 {
		n.Tremolo = m.Tremolo.Beams()
	}
	if num, den, ok := m.Tuplet.Ratio(); ok {
		n.Tuplet = strconv.Itoa(num) + ":" + strconv.Itoa(den)
	}
}

// barNumberAt is one bar's printed number, or where it sits if the score
// is shorter than it was when the numbers were worked out.
func barNumberAt(numbers []int, at int) int {
	return engrave.NumberOf(numbers, at)
}

// tempoText is a tempo as it is written: the word, the mark, or both.
func tempoText(t model.Tempo) string {
	mark := ""
	if t.BPM > 0 {
		mark = t.Unit.String() + " = " + strconv.Itoa(t.BPM)
	}
	switch {
	case t.Text != "" && mark != "":
		return t.Text + " (" + mark + ")"
	case t.Text != "":
		return t.Text
	}
	return mark
}

// hairpinWord and octaveWord name a swell and an octave line the way they
// are spoken about, rather than as the numbers they are stored as.
func hairpinWord(h *model.Hairpin) string {
	if h == nil {
		return ""
	}
	if h.Kind == model.Diminuendo {
		return "diminuendo"
	}
	return "crescendo"
}

func octaveWord(o *model.OctaveShift) string {
	if o == nil {
		return ""
	}
	return strconv.Itoa(o.Amount*7+1) + "va"
}

// accidentalWord names an accidental, and says nothing where none is
// written — which is most notes, and not the same as a natural.
func accidentalWord(a model.Accidental) string {
	if a == model.NoAccidental {
		return ""
	}
	return a.String()
}

// handleTextStyles lists the kinds of writing there are, and how each is
// set.
//
// The list comes from the program rather than being written out again in
// the editor. There is one set of these and one place that decides what
// each looks like; a second list beside it is a second answer waiting to
// disagree with the first.
func (a *App) handleTextStyles(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Name   string  `json:"name"`
		Label  string  `json:"label"`
		Size   float64 `json:"size"`
		Italic bool    `json:"italic"`
		Bold   bool    `json:"bold"`
		Place  string  `json:"place"`
		System bool    `json:"system"`
	}
	out := []entry{}
	for _, s := range model.TextStyles() {
		look := s.Look()
		out = append(out, entry{
			Name: s.String(), Label: s.Label(), Size: look.Size,
			Italic: look.Italic, Bold: look.Bold,
			Place: look.Place.String(), System: look.System,
		})
	}
	writeJSON(w, out)
}

// handleRemoveMeasure takes bars out of the score.
//
// The one thing that could not be done to a bar. They could be added,
// pushed in, split, joined and emptied, and an emptied bar is not a bar
// removed — it is a bar of rest, which the player counts. Somebody who
// typed two bars too many at the end of a piece had no way to say so.
func (a *App) handleRemoveMeasure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	bars := a.score.MeasureCount()
	// The last bar where none is named, which is the one somebody typing
	// at the end of a piece means.
	at := bars - 1
	if v := q.Get("measure"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= bars {
			http.Error(w, "there is no bar "+v, http.StatusBadRequest)
			return
		}
		at = n
	}
	count := 1
	if v := q.Get("count"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			http.Error(w, v+" is not a number of bars — 1 to 100",
				http.StatusBadRequest)
			return
		}
		count = n
	}

	gone := a.score.RemoveMeasures(at, count)
	if gone == 0 {
		http.Error(w, "there was nothing there to remove", http.StatusBadRequest)
		return
	}
	// The bar that is left where they were is a bar of rest until
	// something is written in it, as every empty bar is.
	restEmptyMeasures(a.score)

	writeJSON(w, map[string]any{
		"status": "ok", "removed": gone,
		"measures": a.score.MeasureCount(), "at": at,
	})
}
