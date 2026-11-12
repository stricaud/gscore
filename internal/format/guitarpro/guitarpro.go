/*
 * guitarpro.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package guitarpro reads Guitar Pro files: the .gp3, .gp4 and .gp5 that
// twenty years of guitar transcriptions were saved in, and that TuxGuitar
// reads and writes.
//
// The format is binary and was never published. What is known of it is
// what the programs that read it worked out — TuxGuitar's readers, which
// this follows byte for byte, and PyGuitarPro's, which agree with them. The
// three versions are three dialects of one layout: a banner naming the
// version, the title and the credits, a table of sixty-four MIDI channels,
// then a header for every bar, a header for every track, and the bars
// themselves, track by track within each bar. GP4 adds lyrics and a second
// byte of effects to each note; GP5 adds a second voice to every bar, a
// page setup, and padding whose meaning nobody has recorded, which is
// skipped by count.
//
// Guitar Pro 6 and 7 write something else altogether — a compressed
// container of XML — and are not read here. Both export the older format.
//
// What a track becomes is a tablature staff with the pitches the tuning and
// the frets give: a note is played where the file says it is played, which
// is what a tab is for. A track on the drum channel is a percussion staff,
// with each note written where drummers write that instrument. Hammer-ons,
// slides, bends, harmonics and the rest become the techniques the model has
// names for; what it has no name for — the whammy bar's curve, a fade-in —
// is counted and reported beside the score rather than dropped in silence.
package guitarpro

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string { return "Guitar Pro" }

func (importer) Extensions() []string { return []string{".gp3", ".gp4", ".gp5"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// Partial is what is returned beside a score some of which was not read:
// what the file had that the model cannot hold, counted, and where the
// reading stopped if it stopped short.
type Partial struct {
	Notes []string
}

func (p *Partial) Error() string {
	if len(p.Notes) == 1 {
		return "guitarpro: " + p.Notes[0]
	}
	return fmt.Sprintf("guitarpro: %d things in the file were not read", len(p.Notes))
}

func (p *Partial) Partial()          {}
func (p *Partial) Details() []string { return p.Notes }

// Import reads a Guitar Pro file into a score.
func Import(r io.Reader) (*model.Score, error) {
	rd := newReader(r)
	sg, err := rd.readSong()
	if sg == nil {
		return nil, fmt.Errorf("guitarpro: %w", err)
	}
	b := &builder{sg: sg, missed: map[string]int{}, drums: map[int]int{}}
	s := b.build()
	notes := b.report()
	if err != nil {
		notes = append([]string{fmt.Sprintf("the file ends early, %s: %v; the rest was not read", rd.where, err)}, notes...)
	}
	if len(notes) > 0 {
		return s, &Partial{Notes: notes}
	}
	return s, nil
}

// builder turns a song into a score.
type builder struct {
	sg     *song
	missed map[string]int
	// drums are the General MIDI keys the standard kit has no staff
	// position for, each with how often it was struck.
	drums map[int]int
}

// miss counts something the model has no place for, named in the
// singular and the plural.
func (b *builder) miss(one, many string) { b.missed[one+"\x00"+many]++ }

func (b *builder) report() []string {
	var keys []string
	for k := range b.missed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		one, many, _ := strings.Cut(k, "\x00")
		n := b.missed[k]
		if n == 1 {
			out = append(out, "one "+one+" was not read")
		} else {
			out = append(out, fmt.Sprintf("%d %s were not read", n, many))
		}
	}
	if len(b.drums) > 0 {
		var keys []int
		total := 0
		for k, n := range b.drums {
			keys = append(keys, k)
			total += n
		}
		sort.Ints(keys)
		var names []string
		for _, k := range keys {
			names = append(names, fmt.Sprint(k))
		}
		out = append(out, fmt.Sprintf(
			"%d drum strokes are on instruments the standard kit has no staff position for (keys %s); they are written on the middle line",
			total, strings.Join(names, ", ")))
	}
	return out
}

func (b *builder) build() *model.Score {
	sg := b.sg
	s := &model.Score{
		Tempo: model.Tempo{BPM: sg.tempo, Unit: model.Duration{Num: 1, Den: 4}},
	}
	if s.Tempo.BPM <= 0 {
		s.Tempo.BPM = 120
	}
	s.Identity.Title = sg.title
	s.Identity.Subtitle = sg.subtitle
	if s.Identity.Subtitle == "" {
		s.Identity.Subtitle = sg.artist
	}
	s.Identity.Composer = sg.music
	s.Identity.Copyright = sg.copyright
	if sg.words != "" && sg.words != sg.music {
		s.Identity.Arranger = "Words: " + sg.words
	}

	key := model.KeySignature{Fifths: clampKey(sg.key), Mode: model.Major}
	time := model.TimeSignature{Beats: 4, BeatValue: 4}
	if len(sg.headers) > 0 {
		time = meterOf(sg.headers[0])
	}

	for ti := range sg.tracks {
		s.Parts = append(s.Parts, b.part(&sg.tracks[ti], ti, key, time))
	}
	if len(s.Parts) == 0 {
		s.Parts = []*model.Part{{Clef: model.Treble8vb, Key: key, Time: time,
			Measures: []*model.Measure{model.NewMeasure()}}}
	}
	b.lyrics(s)
	s.Normalise()
	return s
}

func clampKey(k int) int {
	if k > 7 {
		k = 7
	}
	if k < -7 {
		k = -7
	}
	return k
}

func meterOf(h header) model.TimeSignature {
	t := model.TimeSignature{Beats: h.num, BeatValue: h.den}
	if t.Beats <= 0 {
		t.Beats = 4
	}
	switch t.BeatValue {
	case 1, 2, 4, 8, 16, 32:
	default:
		t.BeatValue = 4
	}
	return t
}

// soundOf is what a track's General MIDI program is written for.
//
// The catalogue knows the programs it has a word for; the guitars and
// basses between them — the overdriven and distorted guitars, the fretless
// and the slap basses — are what Guitar Pro files are mostly made of, so
// they are answered here rather than left blank.
func soundOf(program int) string {
	if sound, ok := model.SoundForInstrument(model.Instrument(program)); ok {
		return sound
	}
	switch {
	case program == 24:
		return "pluck.guitar.nylon-string"
	case program == 25:
		return "pluck.guitar.steel-string"
	case program >= 26 && program <= 31:
		return "pluck.guitar.electric"
	case program == 32:
		return "pluck.bass.acoustic"
	case program == 35:
		return "pluck.bass.fretless"
	case program == 38 || program == 39:
		return "pluck.bass.synth"
	case program >= 33 && program <= 37:
		return "pluck.bass.electric"
	}
	return ""
}

// bassLimit is the open string below which a track is written in the bass
// clef: a guitar's bottom E is 40, a bass's top G is 43, and 34 is TuxGuitar's
// line between them.
const bassLimit = 34

func (b *builder) part(t *track, index int, key model.KeySignature, time model.TimeSignature) *model.Part {
	p := &model.Part{
		Name: strings.TrimSpace(t.name),
		Key:  key,
		Time: time,
	}
	if p.Name == "" {
		p.Name = fmt.Sprintf("Track %d", index+1)
	}
	if t.percussion {
		p.Clef = model.Percussion
		p.Sound = "drum.group.set"
	} else {
		p.Instrument = model.Instrument(t.program)
		p.Sound = soundOf(t.program)
		tab := &model.Tablature{Frets: t.frets}
		bass := t.bassClef
		for _, open := range t.strings {
			// A capo is a new nut: every open string sounds that much
			// higher, and the frets are counted from it.
			tab.Strings = append(tab.Strings, pitchFromMIDI(open+t.capo))
			if open <= bassLimit {
				bass = true
			}
		}
		p.Tab = tab
		p.Clef = model.Treble8vb
		if bass {
			p.Clef = model.Bass
		}
	}

	st := &trackState{
		part:     p,
		last:     map[int]*model.Note{},
		pending:  map[int]*model.Note{},
		dynamic:  0, // the first note's dynamic is written, and then only changes
		tempo:    b.sg.tempo,
		key:      key,
		time:     time,
		capoText: t.capo,
	}
	for hi, h := range b.sg.headers {
		var m measure
		if hi < len(t.measures) {
			m = t.measures[hi]
		}
		p.Measures = append(p.Measures, b.measure(st, t, h, hi, m))
	}
	if n := len(p.Measures); n > 0 {
		if last := p.Measures[n-1]; last.RightBarline == model.BarSingle {
			last.RightBarline = model.BarFinal
		}
	}
	return p
}

// trackState is what carries from bar to bar within one track.
type trackState struct {
	part *model.Part
	// last is the note last struck on each string, for a tie to reach
	// back to; pending is a note with a slide on it waiting for the next
	// note on its string, to know which way the slide goes.
	last, pending map[int]*model.Note
	dynamic       int
	tempo         int
	key           model.KeySignature
	time          model.TimeSignature
	capoText      int
	// endings is the bits of the ending the bar before was in.
	endings int
}

func (b *builder) measure(st *trackState, t *track, h header, hi int, m measure) *model.Measure {
	out := &model.Measure{RightBarline: model.BarSingle}
	p := st.part

	// The bar's own signs: meter, key, marker, repeats and endings.
	meter := meterOf(h)
	if hi > 0 && meter != st.time {
		mt := meter
		out.Time = &mt
	}
	st.time = meter
	if h.key != nil {
		k := model.KeySignature{Fifths: clampKey(*h.key), Mode: model.Major}
		if hi > 0 && k != st.key {
			kk := k
			out.Key = &kk
		}
		st.key = k
	}
	// A marker belongs to the bar across the score, so it is written once,
	// on the first staff.
	if h.marker != "" && t == &b.sg.tracks[0] {
		out.Marker = h.marker
	}
	if h.repeatOpen && hi > 0 {
		prev := p.Measures[hi-1]
		if prev.RightBarline == model.BarRepeatEnd {
			prev.RightBarline = model.BarRepeatBoth
		} else {
			prev.RightBarline = model.BarRepeatStart
		}
	}
	if h.repeatClose > 0 {
		out.RightBarline = model.BarRepeatEnd
	}
	if h.alt != 0 {
		e := &model.Ending{}
		for i := 0; i < 8; i++ {
			if h.alt&(1<<i) != 0 {
				e.Numbers = append(e.Numbers, i+1)
			}
		}
		e.Start = h.alt != st.endings
		next := 0
		if hi+1 < len(b.sg.headers) {
			next = b.sg.headers[hi+1].alt
		}
		e.Stop = next != h.alt
		e.Open = e.Stop && h.repeatClose == 0
		out.Ending = e
	}
	st.endings = h.alt

	// The music.
	for vi, beats := range m.voices {
		v := &model.Voice{}
		b.voice(st, t, out, v, beats, hi)
		if vi > 0 && len(v.Elements) == 0 {
			continue
		}
		out.Voices = append(out.Voices, v)
	}
	if len(out.Voices) == 0 {
		out.Voices = []*model.Voice{{}}
	}
	if len(out.Voices[0].Elements) == 0 {
		// A bar the file left empty is a bar of rest.
		out.Voices[0].Elements = []model.Element{&model.Rest{Dur: meter.BarDuration()}}
	}
	if st.capoText > 0 {
		if mk := firstMarks(out); mk != nil {
			mk.Texts = append(mk.Texts, model.TextMark{Text: fmt.Sprintf("Capo %d", st.capoText), Style: model.TextTechnique})
			st.capoText = 0
		}
	}
	return out
}

// firstMarks is the marks of the first element of a bar, for a word that
// goes at its head.
func firstMarks(m *model.Measure) *model.Marks {
	for _, v := range m.Voices {
		for _, el := range v.Elements {
			switch e := el.(type) {
			case *model.Note:
				return &e.Marks
			case *model.Chord:
				return &e.Marks
			case *model.Rest:
				return &e.Marks
			}
		}
	}
	return nil
}

// tupletOf is the ratio the file's division figure means: three in the
// time of two, five in the time of four, and so on.
func tupletOf(n int) (actual, normal int) {
	switch n {
	case 3:
		return 3, 2
	case 5, 6, 7:
		return n, 4
	case 9, 10, 11, 12, 13:
		return n, 8
	}
	return 0, 0
}

// tupletRun is the tuplet group being built in a voice: the beats so far,
// their written length, and the shortest of them.
type tupletRun struct {
	figure   int
	elements []*model.Marks
	written  model.Duration
	shortest model.Duration
}

func (r *tupletRun) close() {
	if len(r.elements) > 0 {
		r.elements[0].Tuplet.Start = true
		r.elements[len(r.elements)-1].Tuplet.Stop = true
	}
	*r = tupletRun{}
}

func (b *builder) voice(st *trackState, t *track, m *model.Measure, v *model.Voice, beats []beat, hi int) {
	var run tupletRun
	for _, bt := range beats {
		if bt.empty {
			continue
		}
		written := model.Duration{Num: 1, Den: bt.dur}
		if bt.dot {
			written = written.Scale(3, 2)
		}
		dur := written
		actual, normal := tupletOf(bt.tuplet)
		if actual > 0 {
			dur = written.Scale(normal, actual)
		}

		if bt.tempo >= 0 && bt.tempo != st.tempo {
			st.tempo = bt.tempo
			m.Tempo = &model.Tempo{BPM: bt.tempo, Unit: model.Duration{Num: 1, Den: 4}}
		}

		var el model.Element
		var mk *model.Marks
		if bt.rest || len(bt.notes) == 0 {
			r := &model.Rest{Dur: dur}
			el, mk = r, &r.Marks
		} else {
			var notes []*model.Note
			for _, n := range bt.notes {
				if note := b.note(st, t, &bt, n, dur); note != nil {
					notes = append(notes, note)
				}
			}
			if len(notes) == 0 {
				r := &model.Rest{Dur: dur}
				el, mk = r, &r.Marks
			} else if len(notes) == 1 {
				el, mk = notes[0], &notes[0].Marks
			} else {
				c := &model.Chord{Notes: notes, Dur: dur}
				el, mk = c, &c.Marks
			}
		}

		// What belongs to the beat rather than to one of its notes.
		if bt.text != "" {
			mk.Texts = append(mk.Texts, model.TextMark{Text: bt.text})
		}
		if bt.chord != nil {
			if sym := chordSymbol(bt.chord); sym != nil {
				mk.Chord = sym
			}
		}
		if bt.slap {
			mk.Texts = append(mk.Texts, model.TextMark{Text: "slap", Style: model.TextTechnique})
		}
		if bt.pop {
			mk.Texts = append(mk.Texts, model.TextMark{Text: "pop", Style: model.TextTechnique})
		}
		if bt.fadeIn {
			mk.Texts = append(mk.Texts, model.TextMark{Text: "fade in", Style: model.TextTechnique})
		}
		if bt.rasguedo {
			mk.Texts = append(mk.Texts, model.TextMark{Text: "rasg.", Style: model.TextTechnique})
		}
		if bt.whammy {
			b.miss("whammy bar dive", "whammy bar dives")
		}
		if bt.stroke != 0 {
			if _, ok := el.(*model.Chord); ok {
				// A brush up the strings is heard from the bottom, which is
				// the plain arpeggio; a brush down is heard from the top.
				mk.Arpeggio = &model.Arpeggio{Down: bt.stroke == 1}
			}
		}
		switch bt.pick {
		case 1:
			mk.Articulations = append(mk.Articulations, model.UpBow)
		case 2:
			mk.Articulations = append(mk.Articulations, model.DownBow)
		}

		// The tuplet, grouped by run: the bracket closes when the written
		// lengths add up to the figure's worth of the shortest note in it,
		// which is three eighths under a 3 and five sixteenths under a 5,
		// and a quarter and an eighth under a 3 as well.
		if actual > 0 {
			mk.Tuplet = &model.Tuplet{Actual: actual, Normal: normal}
			if run.figure != bt.tuplet {
				run.close()
				run.figure = bt.tuplet
				run.written = model.Duration{Num: 0, Den: 1}
			}
			run.elements = append(run.elements, mk)
			run.written = run.written.Add(written)
			if run.shortest.IsZero() || written.Cmp(run.shortest) < 0 {
				run.shortest = written
			}
			if run.written.Cmp(run.shortest.Scale(actual, 1)) >= 0 {
				run.close()
			}
		} else {
			run.close()
		}

		v.Elements = append(v.Elements, el)
	}
	run.close()
}

// dynamics is the file's dynamic figure, one to eight, as the marking it
// stands for.
var dynamics = [...]model.Dynamic{
	model.DynamicNone, model.PPP, model.PP, model.Piano, model.MP,
	model.MF, model.Forte, model.FF, model.FFF,
}

func (b *builder) note(st *trackState, t *track, bt *beat, n note, dur model.Duration) *model.Note {
	out := &model.Note{Dur: dur, String: n.str, Fret: n.fret}
	if n.str < 1 || n.str > len(t.strings) {
		b.miss("note on a string the track does not have", "notes on strings the track does not have")
		return nil
	}

	if n.tied {
		prev := st.last[n.str]
		if prev == nil {
			// A tie to nothing: the note is sounded, since the file says
			// it is there, and the tie is not drawn.
			b.miss("tie with nothing before it", "ties with nothing before them")
		} else {
			prev.TieStart = true
			out.TieStop = true
			out.Fret = prev.Fret
			n.fret = prev.Fret
		}
	}

	if t.percussion {
		pn, known := model.PercussionByKey(n.fret)
		if known {
			out.Pitch = model.PitchFromDiatonicIndex(pn.Step - percussionMiddle)
			out.Head = pn.Head
			out.Percussion = pn.Key
		} else {
			out.Pitch = model.PitchFromDiatonicIndex(0)
			out.Percussion = n.fret
			b.drums[n.fret]++
		}
		out.String, out.Fret = 0, 0
	} else {
		out.Pitch = pitchFromMIDI(st.part.Tab.Strings[n.str-1].MIDI() + n.fret)
	}

	// A slide waiting on this note learns which way it went.
	if from := st.pending[n.str]; from != nil {
		up := out.Pitch.MIDI() > from.Pitch.MIDI()
		for i, tc := range from.Techniques {
			if tc == model.SlideUp && !up {
				from.Techniques[i] = model.SlideDown
			}
		}
		delete(st.pending, n.str)
	}
	st.last[n.str] = out

	if n.dead {
		out.Head = model.HeadX
	}
	if n.ghost {
		out.Small = true
	}
	if n.velocity >= 1 && n.velocity <= 8 && n.velocity != st.dynamic {
		st.dynamic = n.velocity
		out.Dynamic = dynamics[n.velocity]
	}
	if n.finger >= 0 && n.finger <= 4 {
		out.Fingering = []string{fmt.Sprint(n.finger)}
	}

	tech := func(tc model.Technique) { out.Techniques = append(out.Techniques, tc) }
	if n.hammer {
		tech(model.HammerOn)
	}
	switch n.slide {
	case 1, 2:
		// Up until the next note on the string says otherwise.
		tech(model.SlideUp)
		st.pending[n.str] = out
	case 3, -2:
		tech(model.SlideDown)
	case 4, -1:
		tech(model.SlideUp)
	}
	if n.bend != nil {
		tech(bendTechnique(n.bend))
	}
	if n.letRing {
		tech(model.LetRing)
	}
	if n.palmMute {
		tech(model.PalmMute)
	}
	if n.vib || bt.vib {
		tech(model.Vibrato)
	}
	switch n.harmonic {
	case 1:
		tech(model.NaturalHarmonic)
	case 2, 3, 5:
		tech(model.ArtificialHarmonic)
	case 4:
		tech(model.PinchHarmonic)
	}
	if bt.tap {
		tech(model.Tap)
	}
	if n.staccato {
		out.Articulations = append(out.Articulations, model.Staccato)
	}
	if n.heavy {
		out.Articulations = append(out.Articulations, model.Marcato)
	} else if n.accent {
		out.Articulations = append(out.Articulations, model.Accent)
	}
	if n.tremolo >= 1 && n.tremolo <= 3 {
		out.Tremolo = &model.Tremolo{Strokes: n.tremolo}
	}
	if n.trill != nil {
		out.Trill = &model.Trill{Start: true, Stop: true}
	}
	if n.grace != nil && !t.percussion {
		out.Grace = b.grace(st, n, out)
	}
	return out
}

// bendTechnique is the mark for a bend: how far for a plain one, and the
// shape for the others.
func bendTechnique(bd *bend) model.Technique {
	switch bd.kind {
	case 2, 3:
		return model.BendRelease
	case 4, 5:
		return model.PreBend
	}
	// A hundred is a whole tone; anything from three quarters up is drawn
	// as "full", and less as a half.
	v := bd.value
	if bd.peak > v {
		v = bd.peak
	}
	if v >= 75 {
		return model.BendFull
	}
	return model.BendHalf
}

// grace is the small note before this one.
func (b *builder) grace(st *trackState, n note, main *model.Note) *model.GraceGroup {
	g := n.grace
	gn := &model.Note{
		Pitch:  pitchFromMIDI(st.part.Tab.Strings[n.str-1].MIDI() + g.fret),
		String: n.str,
		Fret:   g.fret,
		Dur:    model.Duration{Num: 1, Den: 16},
	}
	if g.duration == 3 {
		gn.Dur = model.Duration{Num: 1, Den: 32}
	}
	if g.dead {
		gn.Head = model.HeadX
	}
	switch g.transition {
	case 1:
		if main.Pitch.MIDI() < gn.Pitch.MIDI() {
			gn.Techniques = append(gn.Techniques, model.SlideDown)
		} else {
			gn.Techniques = append(gn.Techniques, model.SlideUp)
		}
	case 2:
		gn.Techniques = append(gn.Techniques, model.BendFull)
	case 3:
		gn.Techniques = append(gn.Techniques, model.HammerOn)
	}
	return &model.GraceGroup{Notes: []*model.Note{gn}, Slashed: !g.onBeat}
}

// chordSymbol is the chord name over a beat, with its diagram where the
// file drew one.
func chordSymbol(c *chord) *model.ChordSymbol {
	name := strings.TrimSpace(c.name)
	var frame *model.ChordFrame
	played := false
	for _, f := range c.frets {
		if f >= 0 {
			played = true
		}
	}
	if played {
		frame = &model.ChordFrame{FirstFret: c.firstFret}
		if frame.FirstFret < 1 {
			frame.FirstFret = 1
		}
		// The file lists the highest string first; the diagram is spelled
		// from the lowest.
		for i := len(c.frets) - 1; i >= 0; i-- {
			f := c.frets[i]
			if f < 0 {
				f = -1
			}
			frame.Frets = append(frame.Frets, f)
		}
	}
	if name == "" && frame == nil {
		return nil
	}
	sym := &model.ChordSymbol{Frame: frame}
	if name == "" {
		return sym
	}
	// "C#m7/G#": the root is a letter and its accidental, the bass follows
	// a slash, and the quality is what is left.
	root := ""
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'G' {
		root = name[:1]
		rest := name[1:]
		if strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, "b") {
			root += rest[:1]
			rest = rest[1:]
		}
		// A slash is a bass note when a note name follows it, and part of
		// the quality otherwise: C/E has a bass, C11/9- does not.
		if i := strings.LastIndex(rest, "/"); i >= 0 && isNoteName(rest[i+1:]) {
			sym.Bass = rest[i+1:]
			rest = rest[:i]
		}
		sym.Root = root
		sym.Quality = rest
	} else {
		sym.Text = name
	}
	return sym
}

// isNoteName says whether a string is a bare note name: a letter and at
// most one accidental.
func isNoteName(s string) bool {
	if len(s) == 0 || s[0] < 'A' || s[0] > 'G' {
		return false
	}
	return len(s) == 1 || len(s) == 2 && (s[1] == '#' || s[1] == 'b')
}

// lyrics puts the words under the track the file says they are sung by.
//
// The words come as text with the syllables of one word joined by hyphens
// and words parted by spaces, starting at a bar the file names; each
// syllable goes under the next struck note of the track's first voice.
func (b *builder) lyrics(s *model.Score) {
	sg := b.sg
	if sg.lyricTrack < 1 || sg.lyricTrack > len(s.Parts) {
		return
	}
	p := s.Parts[sg.lyricTrack-1]
	for verse, line := range sg.lyrics {
		syllables := splitSyllables(line.text)
		if len(syllables) == 0 {
			continue
		}
		bar := line.from - 1
		if bar < 0 {
			bar = 0
		}
		i := 0
		for mi := bar; mi < len(p.Measures) && i < len(syllables); mi++ {
			m := p.Measures[mi]
			if len(m.Voices) == 0 {
				continue
			}
			for _, el := range m.Voices[0].Elements {
				if i >= len(syllables) {
					break
				}
				var mk *model.Marks
				switch e := el.(type) {
				case *model.Note:
					if e.TieStop {
						continue
					}
					mk = &e.Marks
				case *model.Chord:
					mk = &e.Marks
				default:
					continue
				}
				syl := syllables[i]
				syl.Verse = verse + 1
				mk.Lyrics = append(mk.Lyrics, syl)
				i++
			}
		}
	}
}

// splitSyllables breaks a lyric line the way Guitar Pro writes it: words
// apart on spaces or line ends, a hyphen joining the syllables of one word,
// and a plus sign holding a syllable over the next note.
func splitSyllables(text string) []model.Lyric {
	var out []model.Lyric
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\r' || r == '\t'
	}) {
		parts := strings.Split(word, "-")
		for i, part := range parts {
			if part == "" {
				continue
			}
			l := model.Lyric{Text: strings.TrimSuffix(part, "+")}
			switch {
			case len(parts) == 1:
				l.Syllabic = model.SyllableSingle
			case i == 0:
				l.Syllabic = model.SyllableBegin
			case i == len(parts)-1:
				l.Syllabic = model.SyllableEnd
			default:
				l.Syllabic = model.SyllableMiddle
			}
			l.Extend = strings.HasSuffix(part, "+")
			out = append(out, l)
		}
	}
	return out
}

// percussionMiddle is the staff step the middle line is, counting half
// spaces from the bottom line — where model.DrumSet's positions are measured
// against, and where PitchFromDiatonicIndex(0) lands.
const percussionMiddle = 4

// pitchFromMIDI spells a note number with sharps, which is how a fretted
// instrument's players name the notes between the naturals.
func pitchFromMIDI(m int) model.Pitch {
	steps := []model.Step{model.StepC, model.StepC, model.StepD, model.StepD, model.StepE,
		model.StepF, model.StepF, model.StepG, model.StepG, model.StepA, model.StepA, model.StepB}
	alters := []int{0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 1, 0}
	pc := ((m % 12) + 12) % 12
	oct := m / 12
	if m < 0 && m%12 != 0 {
		oct--
	}
	return model.Pitch{Step: steps[pc], Octave: oct - 1, Alter: alters[pc]}
}
