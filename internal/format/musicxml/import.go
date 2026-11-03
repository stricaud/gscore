/*
 * import.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Import reads a MusicXML document, compressed or not.
// log collects what a file asked for that gscore could not give it exactly.
//
// A MusicXML file is a conversation between two programs that do not have the
// same ideas, and the gaps matter: a microtone silently rounded to the nearest
// semitone changes the music, and the person who wrote the file is the only
// one who can decide whether that is acceptable. So it is written down and
// handed back with the score rather than either discarded or treated as a
// reason to refuse the whole file.
type log struct {
	seen    map[string]bool
	skipped []string
}

func (l *log) say(format string, args ...any) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	if l.seen[msg] {
		return
	}
	l.seen[msg] = true
	l.skipped = append(l.skipped, msg)
}

func Import(r io.Reader) (*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("musicxml: %w", err)
	}
	if isZip(data) {
		data, err = unwrapMXL(data)
		if err != nil {
			return nil, err
		}
	}

	var doc scorePartwise
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("musicxml: %w", err)
	}
	if len(doc.Parts) == 0 {
		return nil, errors.New("musicxml: no parts; score-timewise is not supported")
	}
	lg := &log{}
	s, err := convert(&doc, lg)
	if err != nil {
		return nil, err
	}
	if len(lg.skipped) > 0 {
		return s, &PartialError{Skipped: lg.skipped}
	}
	return s, nil
}

func isZip(b []byte) bool {
	return len(b) > 4 && b[0] == 'P' && b[1] == 'K' && (b[2] == 3 || b[2] == 5)
}

// unwrapMXL pulls the score out of a compressed .mxl container.
//
// The container names the real document in META-INF/container.xml; when that
// is missing or unreadable, the first .xml or .musicxml file that is not
// container metadata is used instead.
func unwrapMXL(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("musicxml: bad .mxl container: %w", err)
	}

	wanted := ""
	for _, f := range zr.File {
		if f.Name != "META-INF/container.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		var c struct {
			Rootfiles []struct {
				FullPath string `xml:"full-path,attr"`
			} `xml:"rootfiles>rootfile"`
		}
		if xml.Unmarshal(b, &c) == nil && len(c.Rootfiles) > 0 {
			wanted = c.Rootfiles[0].FullPath
		}
		break
	}

	for _, f := range zr.File {
		name := f.Name
		if wanted != "" && name != wanted {
			continue
		}
		if wanted == "" {
			if strings.HasPrefix(name, "META-INF/") {
				continue
			}
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".xml") && !strings.HasSuffix(lower, ".musicxml") {
				continue
			}
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("musicxml: %w", err)
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, errors.New("musicxml: no score found inside the .mxl container")
}

func convert(doc *scorePartwise, lg *log) (*model.Score, error) {
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
	}
	if doc.Work != nil {
		s.Identity.Title = doc.Work.Title
	}
	if doc.Ident != nil {
		s.Identity.Copyright = doc.Ident.Rights
		for _, c := range doc.Ident.Creators {
			switch c.Type {
			case "composer":
				s.Identity.Composer = c.Name
			case "subtitle":
				s.Identity.Subtitle = c.Name
			}
		}
	}

	names := map[string]string{}
	programs := map[string]model.Instrument{}
	for _, sp := range doc.PartList.ScoreParts {
		names[sp.ID] = sp.Name
		// The name first, where it names something.
		//
		// A file can say one thing in the name and another in the program,
		// and when it does the name is the truer of the two: it is what is
		// printed over the staff and what a musician reads. The Lieder corpus
		// writes program 69 on a part called "Voice" — a stand-in for playing
		// a song back, 69 being the oboe — so every song in it was sung by an
		// oboe. The name said "Voice" the whole time.
		//
		// Only where the name names something gscore recognises. A part
		// called "Right hand", or anything the guesser is not sure of, leaves
		// the program to answer as it always did.
		if inst, ok := model.InstrumentNamed(sp.Name); ok {
			programs[sp.ID] = inst
		} else if sp.MidiInst != nil && sp.MidiInst.Program > 0 {
			// Counted from one in the file, from zero in General MIDI.
			programs[sp.ID] = model.Instrument(sp.MidiInst.Program - 1)
		}
	}

	tempoSet := false
	// The rehearsal marks, by the bar they name. Collected as the parts
	// are read and written onto every staff once they are all in, because
	// the mark belongs to the bar and not to whichever part the file
	// happened to write it on — "from letter B" has to mean one bar to
	// everybody in the room.
	marks := map[int]string{}
	for _, xp := range doc.Parts {
		// A part may be written on more than one staff — a piano is one
		// instrument played with two hands, and the file says so with
		// <staves>. gscore draws a staff per part, so each becomes a part
		// here and they are braced together again below.
		staves := 1
		for _, xm := range xp.Measures {
			for _, it := range xm.Content {
				if it.Attributes != nil && it.Attributes.Staves > staves {
					staves = it.Attributes.Staves
				}
			}
		}

		parts := make([]*model.Part, staves)
		for i := range parts {
			name := ""
			if i == 0 {
				// Named once. Printing "Piano" beside both staves of a
				// piano says there are two of them.
				name = names[xp.ID]
			}
			parts[i] = &model.Part{
				ID:         xp.ID,
				Name:       name,
				Instrument: programs[xp.ID],
				Clef:       model.Treble,
				Key:        model.KeySignature{Fifths: 0, Mode: model.Major},
				Time:       model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric},
			}
		}

		// Divisions carry over from measure to measure until restated.
		divisions := 1
		// What the last bar was called, for noticing where the file's
		// own numbering stops simply counting on.
		counted := 0
		for _, xm := range xp.Measures {
			for _, it := range xm.Content {
				a := it.Attributes
				if a == nil {
					continue
				}
				if a.Divisions > 0 {
					divisions = a.Divisions
				}
				for i, p := range parts {
					staff := i + 1
					if a.Key != nil {
						p.Key = model.KeySignature{
							Fifths: a.Key.Fifths,
							Mode:   modeValues[strings.ToLower(a.Key.Mode)],
						}
					}
					if a.Time != nil && a.Time.Type > 0 {
						p.Time = convertTime(a.Time, lg)
					}
					for ci := range a.Clefs {
						if staffOf(a.Clefs[ci].Number) == staff {
							p.Clef = convertClef(&a.Clefs[ci])
						}
					}
				}
			}

			// Which side of the bar each barline is written on.
			//
			// It used to take whichever came last and make it this bar's
			// right-hand one, with the location ignored. A repeat that opens
			// a passage is written on the left of the bar it opens, so it
			// landed on the right of that bar instead — one bar late, and
			// the music turned back to the wrong place. A bar carrying one
			// of each lost whichever came first.
			var barline, opening *barline
			for _, it := range xm.Content {
				if it.Barline != nil {
					// No location means the right-hand one: that is the
					// default in the specification, and most writers leave
					// it out.
					if strings.EqualFold(it.Barline.Location, "left") {
						opening = it.Barline
					} else {
						barline = it.Barline
					}
				}
				if it.Direction != nil && !tempoSet {
					if t, ok := convertTempo(*it.Direction); ok {
						s.Tempo = t
						tempoSet = true
					}
				}
				// A rehearsal mark names the bar for everybody in the
				// room, so it is kept against the bar and not against
				// the part it happened to be written on. Files write it
				// once, nearly always on the top staff; gscore puts it
				// on every staff of that bar once all the parts are in.
				if it.Direction != nil && it.Direction.Type.Rehearsal != "" {
					text := strings.TrimSpace(it.Direction.Type.Rehearsal)
					switch at := len(parts[0].Measures); {
					case marks[at] == "":
						marks[at] = text
					case marks[at] != text:
						// A bar carrying two of them — "12" and "bis"
						// over the same bar is in the specification's
						// own test suite. gscore names a bar once,
						// because a conductor calling it calls it one
						// thing, so the second is left out and said so.
						lg.say("bar %d carries more than one rehearsal "+
							"mark; %q was kept and %q left out",
							at+1, marks[at], text)
					}
				}
			}

			// What opens this bar closes the one before it: in a score the
			// two are one mark, written between them.
			//
			// A repeat opening the very first bar has no bar before it to be
			// written on. Nothing is lost by letting it go — a passage that
			// repeats from the beginning repeats from the beginning whether
			// or not anything says so.
			if open := convertBarline(opening); open != model.BarSingle {
				for _, p := range parts {
					if len(p.Measures) == 0 {
						continue
					}
					prev := p.Measures[len(p.Measures)-1]
					prev.RightBarline = joinBarlines(prev.RightBarline, open)
				}
			}

			ms := convertMeasure(xm, divisions, staves, lg)
			// What the file calls this bar, where that is not simply the
			// one after the last. See numberedFrom.
			from := numberedFrom(xm, len(parts[0].Measures), &counted)
			for i, p := range parts {
				ms[i].RightBarline = convertBarline(barline)
				if from != nil {
					n := *from
					ms[i].NumberFrom = &n
				}
				p.Measures = append(p.Measures, ms[i])
			}
		}

		if staves > 1 {
			// The brace, and barlines running down through the gap: what
			// makes two staves read as one instrument rather than two.
			s.Groups = append(s.Groups, model.StaffGroup{
				First:    len(s.Parts),
				Last:     len(s.Parts) + staves - 1,
				Symbol:   model.GroupBrace,
				Barlines: true,
				Name:     names[xp.ID],
			})
		}
		s.Parts = append(s.Parts, parts...)
	}

	// And now the marks, onto every staff of the bars they name.
	for at, text := range marks {
		for _, p := range s.Parts {
			if at < len(p.Measures) {
				p.Measures[at].Marker = text
			}
		}
	}
	return s, nil
}

// numberedFrom is the number the file gives a bar, where gscore would not
// arrive at it by counting.
//
// The engraver works bar numbers out for itself, and has to: most formats
// do not carry them, and the rule — bar one is the first full bar, so an
// upbeat is not counted — is what every edition does. MusicXML is the
// exception. It says what each bar is called, and says separately, with
// implicit="yes", when a bar is not to be counted. A file whose first bar
// is short but not implicit means a short first bar that is still bar
// one, and the engraver guessing an upbeat there numbered the whole piece
// one low: the Nocturne's bar 11 came out as 10, all the way to the end.
//
// So the file wins wherever it has an opinion. Only where it disagrees
// with plain counting is anything written down, which keeps the ordinary
// file — numbered 1, 2, 3 — free of a restart on every bar.
//
// A number that is not a number is left alone: "12a" is what a publisher
// writes for the second half of a split bar, and there is nothing useful
// to be done with it here.
func numberedFrom(xm xmlMeasure, at int, counted *int) *int {
	if xm.Implicit {
		// The file says this bar is not counted. That is the engraver's
		// own rule for an upbeat, so there is nothing to override.
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(xm.Number))
	if err != nil {
		return nil
	}
	was := *counted
	*counted = n
	// Counting on from the last bar, which is what the engraver does
	// anyway once it has been told where to start.
	if at > 0 && n == was+1 {
		return nil
	}
	return &n
}

// countBeats reads what stands above the line in a time signature.
//
// Usually a number. Sometimes a sum — "3+2" is a bar of five, counted three
// and two, and writing it that way is the whole point of a composite meter.
// gscore has one number there, so the sum is added up; the grouping is lost
// but the bar is the right length, which is the part the music depends on.
func countBeats(s string, lg *log) int {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	total, any := 0, false
	for _, part := range strings.Split(s, "+") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			total += n
			any = true
		}
	}
	if !any {
		lg.say("a time signature of %q could not be read; four beats assumed", s)
		return 4
	}
	lg.say("the composite time signature %q is counted as %d beats in a bar",
		s, total)
	return total
}

func convertTime(t *time, lg *log) model.TimeSignature {
	out := model.TimeSignature{Beats: countBeats(t.Beats, lg), BeatValue: t.Type}
	switch strings.ToLower(t.Symbol) {
	case "common":
		out.Symbol = model.TimeCommon
	case "cut":
		out.Symbol = model.TimeCut
	default:
		out.Symbol = model.TimeNumeric
	}
	return out
}

func convertClef(c *clef) model.Clef {
	switch strings.ToUpper(c.Sign) {
	case "F":
		return model.Bass
	case "C":
		if c.Line == 4 {
			return model.Tenor
		}
		return model.Alto
	case "PERCUSSION":
		return model.Percussion
	}
	return model.Treble
}

func convertTempo(d direction) (model.Tempo, bool) {
	m := d.Type.Metronome
	if m == nil {
		return model.Tempo{}, false
	}
	bpm := m.bpm()
	if bpm <= 0 {
		return model.Tempo{}, false
	}
	v, ok := typeValues[m.BeatUnit]
	if !ok {
		v = model.Quarter
	}
	unit := v.Duration()
	if m.Dot != nil {
		unit = unit.Scale(3, 2)
	}
	return model.Tempo{BPM: bpm, Unit: unit, Text: d.Type.Words}, true
}

// joinBarlines is the one mark written between two bars, from what closes the
// first and what opens the second.
//
// A repeat closing where another opens is one sign — dots, a heavy line,
// dots — and the model holds it as one style on the bar it stands after. Two
// separate marks is what MusicXML writes and not what a score shows.
func joinBarlines(closing, opening model.BarlineStyle) model.BarlineStyle {
	if opening != model.BarRepeatStart {
		return closing
	}
	switch closing {
	case model.BarRepeatEnd, model.BarRepeatBoth:
		return model.BarRepeatBoth
	case model.BarSingle, model.BarNone:
		return model.BarRepeatStart
	}
	// A repeat opening straight after a double or a final bar is not
	// something one mark can say, and the bar that is already written is the
	// more important of the two. Rare enough to leave alone rather than
	// guess at.
	return closing
}

func convertBarline(b *barline) model.BarlineStyle {
	if b == nil {
		return model.BarSingle
	}
	if b.Repeat != nil {
		if strings.EqualFold(b.Repeat.Direction, "forward") {
			return model.BarRepeatStart
		}
		return model.BarRepeatEnd
	}
	switch b.BarStyle {
	case "light-light":
		return model.BarDouble
	case "light-heavy":
		return model.BarFinal
	case "none":
		return model.BarNone
	}
	return model.BarSingle
}

// convertNotes turns one measure's notes into elements, gathering the runs
// marked <chord/> back into chords.
func convertNotes(notes []xmlNote, divisions int, v *model.Voice, lg *log) {
	var pending *model.Chord

	flush := func() {
		if pending != nil {
			v.Elements = append(v.Elements, pending)
			pending = nil
		}
	}

	// Grace notes waiting for the note they lean on. They take no time out
	// of the bar — that is what makes them grace notes — so they cannot go
	// into the voice on their own account and have to be carried until the
	// note they belong to turns up.
	var grace *model.GraceGroup
	takeGrace := func(m *model.Marks) {
		if grace != nil {
			m.Grace = grace
			grace = nil
		}
	}

	// The tuplets met, in order, so that where a group begins and ends can
	// be settled once the whole voice has been read.
	//
	// A file may say so itself, with <tuplet type="start"> on the first
	// note and "stop" on the last. It may also say nothing at all and give
	// only the arithmetic on each note, which is legal and common — so the
	// ends are inferred from a run of notes sharing the same ratio where
	// the file has not marked them. Without that a group of eleven comes
	// out as eleven groups of one, each with its own 11 over it.
	type tupletRun struct {
		marks    *model.Marks
		actual   int
		normal   int
		start    bool
		stop     bool
		explicit bool
	}
	var runs []tupletRun
	takeTuplet := func(m *model.Marks, xn *xmlNote) {
		tm := xn.TimeMod
		if tm == nil || tm.Actual <= 0 || tm.Normal <= 0 || tm.Actual == tm.Normal {
			return
		}
		r := tupletRun{marks: m, actual: tm.Actual, normal: tm.Normal}
		if xn.Notations != nil {
			for _, t := range xn.Notations.Tuplets {
				r.explicit = true
				switch strings.ToLower(t.Type) {
				case "start":
					r.start = true
					// Written down for the arithmetic and not to be read:
					// a measured tremolo is six sixteenths in the time of
					// four, and the strokes through the stem are how that
					// is written, not a 6.
					if strings.EqualFold(t.PrintObject, "no") ||
						strings.EqualFold(t.ShowNumber, "none") {
						m.Tuplet = &model.Tuplet{Hidden: true}
					}
				case "stop":
					r.stop = true
				}
			}
		}
		runs = append(runs, r)
	}

	for i := range notes {
		xn := &notes[i]
		dur := model.Duration{Num: xn.Duration, Den: 4 * divisions}.Reduced()
		dur = tupletDuration(xn, dur)

		if xn.Grace != nil {
			if xn.Pitch == nil {
				continue // a grace rest is not a thing anybody writes
			}
			if grace == nil {
				grace = &model.GraceGroup{}
			}
			if strings.EqualFold(xn.Grace.Slash, "yes") {
				grace.Slashed = true
			}
			g := convertNote(xn, writtenValue(xn.Type), lg)
			grace.Notes = append(grace.Notes, g)
			continue
		}

		if xn.Duration == 0 {
			// No duration and not marked as a grace note. There is nothing
			// to be done with it: giving it time it does not claim would
			// push the rest of the bar along.
			lg.say("a note with no duration was left out")
			continue
		}

		if xn.Rest != nil {
			flush()
			r := &model.Rest{Dur: dur}
			takeGrace(&r.Marks)
			takeTuplet(&r.Marks, xn)
			v.Elements = append(v.Elements, r)
			continue
		}
		if xn.Pitch == nil && xn.Unpitched == nil {
			lg.say("a note with neither a pitch nor a drum was left out")
			continue
		}

		n := convertNote(xn, dur, lg)
		takeGrace(&n.Marks)
		// Only the note that starts a chord: the rest of its notes sound
		// with it and are not a group of their own.
		if xn.Chord == nil {
			takeTuplet(&n.Marks, xn)
		}

		if xn.Chord != nil {
			// This note sounds with the one before it.
			if pending != nil {
				pending.Notes = append(pending.Notes, n)
				continue
			}
			if k := len(v.Elements); k > 0 {
				if prev, ok := v.Elements[k-1].(*model.Note); ok {
					v.Elements = v.Elements[:k-1]
					pending = &model.Chord{
						Notes: []*model.Note{prev, n},
						Dur:   prev.Dur,
						Marks: prev.Marks,
					}
					continue
				}
			}
			// Nothing to attach to; treat it as its own note.
		}

		flush()
		v.Elements = append(v.Elements, n)
	}
	flush()
	if grace != nil {
		// Grace notes at the very end of a bar, or after the note they lean
		// on, which MusicXML allows and gscore has nowhere to put: a grace
		// group hangs off the note in front of it, and there is no note in
		// front of it here.
		lg.say("%d grace note(s) at the end of a bar were left out",
			len(grace.Notes))
	}

	// And now the tuplets, which could not be settled until the whole
	// voice was read: a group's end is only known once something that is
	// not part of it turns up.
	for i := range runs {
		r := &runs[i]
		start, stop := r.start, r.stop
		if !r.explicit {
			// The file gave the arithmetic and not the brackets, so the
			// group is the run of notes sharing the ratio: this one
			// begins it if what came before was something else, and ends
			// it if what comes after is.
			start = i == 0 || runs[i-1].actual != r.actual ||
				runs[i-1].normal != r.normal || !touching(runs[i-1].marks, r.marks, v)
			stop = i == len(runs)-1 || runs[i+1].actual != r.actual ||
				runs[i+1].normal != r.normal || !touching(r.marks, runs[i+1].marks, v)
		}
		hidden := r.marks.Tuplet != nil && r.marks.Tuplet.Hidden
		r.marks.Tuplet = &model.Tuplet{
			Actual: r.actual, Normal: r.normal,
			Start: start, Stop: stop, Hidden: hidden,
		}
	}
}

// touching says whether two tupleted elements are next to each other in
// the voice.
//
// Two groups of three in a row, with the same ratio and no bracket in the
// file to tell them apart, are one group of six to anything that only
// looks at the numbers. What separates them is a note between them that
// is not in a tuplet at all — so this asks the voice, which is the only
// thing that knows the order.
func touching(a, b *model.Marks, v *model.Voice) bool {
	at := -1
	for i, el := range v.Elements {
		m := marksOfElement(el)
		if m == a {
			at = i
		} else if m == b {
			return at == i-1
		}
	}
	return false
}

// marksOfElement is the marks an element carries, where it carries any.
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

// convertMeasure walks a measure in document order, so that each dynamic
// lands on the note that follows it rather than all of them on the first.
// where is one place in a bar: which staff, which voice, and when.
type where struct {
	staff int
	voice string
}

// timed is one note with the moment it starts at.
type timed struct {
	at model.Duration
	n  xmlNote
}

/*
 * How a bar is actually written.
 *
 * MusicXML does not write a bar voice by voice or staff by staff. It writes
 * a stream of notes with a cursor, and <backup> winds the cursor back so
 * that something else can be written over what is already there. Whether
 * that something else is another voice of the same staff or the other hand
 * of a piano is not said by the backup at all — it is said by the <staff>
 * and <voice> on the notes that follow.
 *
 * So the cursor is what is followed here, and every note is filed under the
 * staff and voice it claims, at the moment the cursor had reached. The
 * staves become parts, the voices become voices, and a gap in either is a
 * rest that is not printed — the other line is already holding that beat.
 */

// convertMeasure splits one written measure into one gscore measure per
// staff, each with its own voices.
func convertMeasure(xm xmlMeasure, divisions, staves int, lg *log) []*model.Measure {
	zero := model.Duration{Num: 0, Den: 1}
	cursor := zero
	last := zero // where the note before this one began, for <chord/>

	notes := map[where][]timed{}
	var order []where
	file := func(w where, t timed) {
		if _, seen := notes[w]; !seen {
			order = append(order, w)
		}
		notes[w] = append(notes[w], t)
	}

	// A dynamic belongs to the next thing written on its staff.
	pending := map[int]model.Dynamic{}
	var dyns []dynMark
	// Grace notes waiting for the note they lean on.
	var graces []xmlNote

	for _, it := range xm.Content {
		switch {
		case it.Shift != nil:
			by := model.Duration{Num: it.Shift.Duration, Den: 4 * divisions}.Reduced()
			if it.Shift.Back {
				cursor = cursor.Sub(by)
				if cursor.Cmp(zero) < 0 {
					lg.say("a backup reached past the start of its bar; " +
						"it was read as the start")
					cursor = zero
				}
			} else {
				cursor = cursor.Add(by)
			}

		case it.Direction != nil && it.Direction.Type.Dynamics != nil:
			if d := it.Direction.Type.Dynamics.get(); d != model.DynamicNone {
				pending[staffOf(it.Direction.Staff)] = d
			}

		case it.Note != nil:
			xn := *it.Note
			if xn.Grace != nil {
				// Held until the note it leans on turns up, and filed with
				// that note rather than with its own <staff>. A grace note
				// may be written on the other staff of a piano from the
				// note it decorates — the hand reaches across — and a
				// grace group hangs off a note, so put on a staff where
				// there is no such note it has nothing to hang from and is
				// lost.
				graces = append(graces, xn)
				continue
			}
			w := where{staff: staffOf(xn.Staff), voice: xn.Voice}
			at := cursor
			for _, g := range graces {
				file(w, timed{at: at, n: g})
			}
			graces = nil
			if xn.Chord != nil {
				// Sounds with the note before it, and took no time of its
				// own when that one was counted.
				at = last
			}
			file(w, timed{at: at, n: xn})

			if d, ok := pending[w.staff]; ok {
				// Where the marking falls, to be attached once the voices
				// exist: an element cannot be marked before it is made.
				dyns = append(dyns, dynMark{staff: w.staff, at: at, d: d})
				delete(pending, w.staff)
			}

			if xn.Chord == nil {
				last = cursor
				cursor = cursor.Add(model.Duration{
					Num: xn.Duration, Den: 4 * divisions,
				}.Reduced())
			}
		}
	}

	if len(graces) > 0 {
		lg.say("%d grace note(s) at the end of a bar were left out", len(graces))
	}

	out := make([]*model.Measure, staves)
	for i := range out {
		out[i] = &model.Measure{}
	}
	// Voices in the order they were first written, so that the upper line
	// of a staff is voice one and gets its stems up.
	for _, w := range order {
		if w.staff < 1 || w.staff > staves {
			lg.say("a note on staff %d was moved to staff 1; the part has %d",
				w.staff, staves)
			w.staff = 1
		}
		v := buildVoice(notes[w], divisions, lg)
		if v == nil {
			continue
		}
		m := out[w.staff-1]
		m.Voices = append(m.Voices, v)
	}
	for _, m := range out {
		if len(m.Voices) == 0 {
			m.Voices = append(m.Voices, &model.Voice{})
		}
	}
	for _, d := range dyns {
		if d.staff >= 1 && d.staff <= len(out) {
			markDynamic(out[d.staff-1], d.at, d.d)
		}
	}
	return out
}

// dynMark is a dynamic and the moment in the bar it was written at.
type dynMark struct {
	staff int
	at    model.Duration
	d     model.Dynamic
}

// markDynamic puts a dynamic on the first note of a staff at or after the
// moment it was written.
func markDynamic(m *model.Measure, at model.Duration, d model.Dynamic) {
	for _, v := range m.Voices {
		t := model.Duration{Num: 0, Den: 1}
		for _, el := range v.Elements {
			if t.Cmp(at) >= 0 {
				switch e := el.(type) {
				case *model.Note:
					e.Dynamic = d
					return
				case *model.Chord:
					e.Dynamic = d
					return
				}
			}
			t = t.Add(el.Duration())
		}
	}
}

// buildVoice turns one staff-and-voice's notes into a line of music, with a
// rest that is not printed wherever it was not playing.
func buildVoice(ts []timed, divisions int, lg *log) *model.Voice {
	v := &model.Voice{}
	at := model.Duration{Num: 0, Den: 1}

	var run []xmlNote
	runAt := at
	flush := func() {
		if len(run) == 0 {
			return
		}
		if gap := runAt.Sub(at); gap.Cmp(model.Duration{Num: 0, Den: 1}) > 0 {
			v.Elements = append(v.Elements, &model.Rest{Dur: gap, Hidden: true})
			at = runAt
		}
		before := len(v.Elements)
		convertNotes(run, divisions, v, lg)
		for _, el := range v.Elements[before:] {
			at = at.Add(el.Duration())
		}
		run = nil
	}

	for _, t := range ts {
		// A run is as much as can be converted together: consecutive notes
		// that follow on from one another. A jump means the voice was
		// silent in between.
		if len(run) == 0 {
			runAt = t.at
		} else if t.n.Chord == nil && t.n.Grace == nil && t.at.Cmp(at.Add(runLength(run, divisions))) != 0 {
			flush()
			runAt = t.at
		}
		run = append(run, t.n)
	}
	flush()
	if len(v.Elements) == 0 {
		return nil
	}
	return v
}

// runLength is how much time a run of notes has taken so far.
func runLength(run []xmlNote, divisions int) model.Duration {
	total := model.Duration{Num: 0, Den: 1}
	for _, n := range run {
		if n.Chord != nil || n.Grace != nil {
			continue
		}
		total = total.Add(model.Duration{
			Num: n.Duration, Den: 4 * divisions,
		}.Reduced())
	}
	return total
}

// attachDynamic puts a dynamic on the first sounding element of a run.
func attachDynamic(els []model.Element, d model.Dynamic) {
	for _, el := range els {
		switch e := el.(type) {
		case *model.Note:
			e.Dynamic = d
			return
		case *model.Chord:
			e.Dynamic = d
			return
		}
	}
}

func convertNote(xn *xmlNote, dur model.Duration, lg *log) *model.Note {
	// A drum is written where it is played, not at a pitch: the file says
	// which line of the staff the notehead sits on and nothing about what
	// note it is. That line is a position, so it is kept as one.
	if xn.Pitch == nil && xn.Unpitched != nil {
		step, _ := model.StepFromLetter(firstByte(xn.Unpitched.Step))
		return &model.Note{
			Pitch: model.Pitch{Step: step, Octave: xn.Unpitched.Octave},
			Dur:   dur,
		}
	}
	step, _ := model.StepFromLetter(firstByte(xn.Pitch.Step))
	alter := 0
	if xn.Pitch.Alter != nil {
		// A decimal here is a microtone — -1.5 is a sesquiflat — and gscore
		// writes pitches in whole semitones. Rounding is the only thing it
		// can do, and changing somebody's music without telling them is the
		// one thing it must not do.
		alter = int(math.Round(*xn.Pitch.Alter))
		if *xn.Pitch.Alter != float64(alter) {
			lg.say("a microtone (alter %g) was rounded to %d semitones",
				*xn.Pitch.Alter, alter)
		}
	}
	n := &model.Note{
		Pitch: model.Pitch{Step: step, Octave: xn.Pitch.Octave, Alter: alter},
		Dur:   dur,
	}
	if a, ok := accidentalValues[xn.Accidental]; ok {
		n.Accidental = a
	}
	switch xn.Stem {
	case "up":
		n.Stem = model.StemUp
	case "down":
		n.Stem = model.StemDown
	}

	for _, t := range xn.Tie {
		switch t.Type {
		case "start":
			n.TieStart = true
		case "stop":
			n.TieStop = true
		}
	}
	if nt := xn.Notations; nt != nil {
		for _, t := range nt.Tied {
			switch t.Type {
			case "start":
				n.TieStart = true
			case "stop":
				n.TieStop = true
			}
		}
		for _, sl := range nt.Slurs {
			switch sl.Type {
			case "start":
				n.SlurStart = true
			case "stop":
				n.SlurStop = true
			}
		}
		if nt.Fermata != nil {
			n.AddArticulation(model.Fermata)
		}
		if a := nt.Articulations; a != nil {
			if a.Staccato != nil {
				n.AddArticulation(model.Staccato)
			}
			if a.Staccatissimo != nil {
				n.AddArticulation(model.Staccatissimo)
			}
			if a.Accent != nil {
				n.AddArticulation(model.Accent)
			}
			if a.Tenuto != nil {
				n.AddArticulation(model.Tenuto)
			}
			if a.Marcato != nil {
				n.AddArticulation(model.Marcato)
			}
			if a.UpBow != nil {
				n.AddArticulation(model.UpBow)
			}
			if a.DownBow != nil {
				n.AddArticulation(model.DownBow)
			}
		}
	}
	return n
}

func firstByte(s string) byte {
	if s == "" {
		return 'C'
	}
	return s[0]
}

// writtenValue is how long a grace note is drawn as.
//
// A grace note carries no <duration>, because it takes no time out of the
// bar, but it is still written as something — an eighth with one flag, a
// sixteenth with two — and that is what decides how it looks.
// tupletDuration is how long a note in a tuplet really sounds.
//
// <duration> is in divisions and a tuplet rarely divides them evenly:
// eleven notes in the time of six eighths of a 6/4 bar come to 130.9
// divisions each, and the file writes 130 — so the eleven of them are
// ten divisions short of the time they fill, and scaling that back up by
// eleven sixths gives 143/1152 of a whole note, which is not a length any
// notehead spells. The notes then had no flags, so no beam, so the figure
// over them was given a bracket to hang from.
//
// The file says what they are: <type>eighth</type>, and eleven in the
// time of six. Taken from there the arithmetic is exact and the rounding
// never happens. Only for tuplets, because everywhere else <duration> is
// the thing to trust — a file may hold a note longer than its type says
// on purpose, and that is not this.
func tupletDuration(xn *xmlNote, dur model.Duration) model.Duration {
	tm := xn.TimeMod
	if tm == nil || tm.Actual <= 0 || tm.Normal <= 0 || xn.Type == "" {
		return dur
	}
	if _, ok := typeValues[xn.Type]; !ok {
		return dur // a type this does not know; the duration is all there is
	}
	w := writtenValue(xn.Type)
	for range xn.Dots {
		w = model.Duration{Num: w.Num * 3, Den: w.Den * 2}.Reduced()
	}
	return model.Duration{
		Num: w.Num * tm.Normal,
		Den: w.Den * tm.Actual,
	}.Reduced()
}

func writtenValue(kind string) model.Duration {
	if v, ok := typeValues[kind]; ok {
		return v.Duration()
	}
	return model.Eighth.Duration()
}
