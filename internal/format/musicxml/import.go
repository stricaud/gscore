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
	"sort"
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
	readHeader(s, doc)

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

	// Where the speed is set, by the bar it is set at.
	//
	// Kept by bar rather than applied as it is read, because a tempo
	// belongs to the piece and not to a staff: a score writes "a tempo"
	// over the top line only, and a part copied out of it writes the same
	// mark on its own line. Gathering them all and settling afterwards
	// means the first part to mention a bar speaks for everybody, and a
	// second saying the same thing changes nothing.
	tempoAt := map[int]model.Tempo{}
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
		// What each staff is written in as the reading stands.
		//
		// A signature is not a property of a part: a piece changes key and
		// meter, a hand drops into the bass clef and comes back, and a
		// collection like this one is three movements in a single file,
		// each with a key and a meter of its own. All of that used to be
		// written onto the part, where each change overwrote the last —
		// so the whole piece was drawn in whatever the file happened to
		// say last. Altès's 10me Solo opens in E flat and 3/4 and came out
		// in C and cut time, which is where the third movement ends up.
		//
		// The part keeps what the piece starts in, because that is what
		// gets drawn at the head of every system. Everything after it is
		// a change, written on the bar it happens at.
		type signature struct {
			key  model.KeySignature
			time model.TimeSignature
			clef model.Clef
		}
		state := make([]signature, staves)
		for i, p := range parts {
			// Seeded with what the part starts as, so that a file which
			// states nothing until its second bar is not read as changing
			// to the very thing it was already in.
			state[i] = signature{key: p.Key, time: p.Time, clef: p.Clef}
		}
		for _, xm := range xp.Measures {
			// What this bar changes, by staff, gathered before the bar is
			// built so it can be written onto it.
			type change struct {
				key   *model.KeySignature
				time  *model.TimeSignature
				clef  *model.Clef
				lines int
			}
			chg := make([]change, staves)
			for _, it := range xm.Content {
				a := it.Attributes
				if a == nil {
					continue
				}
				if a.Divisions > 0 {
					divisions = a.Divisions
				}
				for i := range parts {
					staff := i + 1
					for ki := range a.Keys {
						// No number means every staff of the part, which
						// is what a part on one staff always writes.
						if n := a.Keys[ki].Number; n != 0 && n != staff {
							continue
						}
						k := model.KeySignature{
							Fifths: a.Keys[ki].Fifths,
							Mode:   modeValues[strings.ToLower(a.Keys[ki].Mode)],
						}
						chg[i].key = &k
					}
					for ti := range a.Times {
						if n := a.Times[ti].Number; n != 0 && n != staff {
							continue
						}
						if a.Times[ti].Type <= 0 {
							continue
						}
						t := convertTime(&a.Times[ti], lg)
						chg[i].time = &t
					}
					for ci := range a.Clefs {
						if staffOf(a.Clefs[ci].Number) != staff {
							continue
						}
						c := convertClef(&a.Clefs[ci], lg)
						chg[i].clef = &c
					}
					// How the staff is ruled, which is a property of the
					// staff rather than of the bar: a drum with no pitch
					// is written on one line, and a file that says so is
					// not describing a five-line staff with something
					// odd about it.
					for di := range a.Details {
						if staffOf(a.Details[di].Number) != staff {
							continue
						}
						if n := a.Details[di].Lines; n != nil && *n > 0 && *n < 5 {
							chg[i].lines = *n
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
				// A speed, written where it changes.
				//
				// A piece is not one tempo. This file is three movements
				// — Allegro Moderato, Andantino, Allegro — and reading
				// only the first meant the whole of it played at the
				// first speed found, which was not even the opening one:
				// the opening said "Allegro Moderato" with the number in
				// <sound> and no metronome mark, so the reader walked
				// past it and took a metronome mark from bar 24.
				if it.Direction != nil {
					if t, ok := convertTempo(*it.Direction); ok {
						at := len(parts[0].Measures)
						if _, taken := tempoAt[at]; !taken {
							tempoAt[at] = t
						}
					}
				}
				// A rehearsal mark names the bar for everybody in the
				// room, so it is kept against the bar and not against
				// the part it happened to be written on. Files write it
				// once, nearly always on the top staff; gscore puts it
				// on every staff of that bar once all the parts are in.
				if it.Direction != nil && it.Direction.rehearsal() != "" {
					text := it.Direction.rehearsal()
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
				// The signature this bar is in, and whether it is news.
				//
				// A file restates the key and meter at the head of every
				// system, and at the start of a movement whether or not
				// anything changed. Written down as a change each time,
				// that would draw a signature in the middle of the music
				// saying what it already said — so only a difference
				// from what is in force counts.
				st := &state[i]
				first := len(p.Measures) == 0
				if k := chg[i].key; k != nil {
					switch {
					case first:
						p.Key = *k
					case *k != st.key:
						c := *k
						ms[i].Key = &c
					}
					st.key = *k
				}
				if tm := chg[i].time; tm != nil {
					switch {
					case first:
						p.Time = *tm
					case *tm != st.time:
						c := *tm
						ms[i].Time = &c
					}
					st.time = *tm
				}
				if c := chg[i].clef; c != nil {
					switch {
					case first:
						p.Clef = *c
					case *c != st.clef:
						v := *c
						ms[i].Clef = &v
					}
					st.clef = *c
				}
				// How many lines the staff is ruled with belongs to the
				// staff and not to a bar of it, so the first one that
				// says settles it.
				if n := chg[i].lines; n > 0 && p.StaffLines == 0 {
					p.StaffLines = n
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

	// And the speeds, in bar order, because each one only counts as a
	// change against the one before it. The piece's own tempo is whatever
	// stands at the first bar; a mark later on is written onto its bar.
	bars := make([]int, 0, len(tempoAt))
	for at := range tempoAt {
		bars = append(bars, at)
	}
	sort.Ints(bars)
	inForce := s.Tempo
	for i, at := range bars {
		t := tempoAt[at]
		if i == 0 && at == 0 {
			s.Tempo = t
			inForce = t
			continue
		}
		if t == inForce {
			continue
		}
		inForce = t
		for _, p := range s.Parts {
			if at < len(p.Measures) {
				c := t
				p.Measures[at].Tempo = &c
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
	// A meter the file says is not printed is in force all the same: the
	// bars are counted in it and nothing is drawn.
	if strings.EqualFold(t.PrintObject, "no") {
		out.Symbol = model.TimeHidden
	}
	return out
}

// convertClef is which of the five signs gscore draws this one is.
//
// MusicXML writes a clef as a letter and a line, which is seventeen signs
// and not five: the C clef on any line from one to five, the G clef on the
// first line, the F clef on the third, and any of them moved an octave.
// gscore draws treble, bass, alto, tenor and percussion, so the rest are
// read as the nearest of those and said out loud — a clef read as its
// neighbour puts every note on the staff a line or two from where it was
// written, which is not something to do quietly.
func convertClef(c *clef, lg *log) model.Clef {
	sign := strings.ToUpper(strings.TrimSpace(c.Sign))
	switch sign {
	case "F":
		if c.Line != 0 && c.Line != 4 {
			lg.say("an F clef on line %d is not one gscore draws; "+
				"it was read as the bass clef", c.Line)
		}
		return model.Bass
	case "C":
		switch c.Line {
		case 4:
			return model.Tenor
		case 0, 3:
			return model.Alto
		}
		lg.say("a C clef on line %d is not one gscore draws; "+
			"it was read as the alto clef", c.Line)
		return model.Alto
	case "PERCUSSION":
		return model.Percussion
	case "G":
		if c.Line != 0 && c.Line != 2 {
			lg.say("a G clef on line %d is not one gscore draws; "+
				"it was read as the treble clef", c.Line)
		}
		// The 8 under the sign: the clef a tenor part and a guitar part
		// are written in. The notes on the page are a treble staff's
		// notes, so only the sign and the sound differ.
		if c.OctaveChange == -1 {
			return model.Treble8vb
		}
		if c.OctaveChange != 0 {
			lg.say("a G clef moved %d octaves is not one gscore draws; "+
				"it was read as the treble clef", c.OctaveChange)
		}
	case "TAB", "JIANPU", "NONE", "":
		lg.say("a %q clef is not one gscore draws; it was read as the "+
			"treble clef", c.Sign)
	default:
		lg.say("a %q clef is not one gscore knows; it was read as the "+
			"treble clef", c.Sign)
	}
	return model.Treble
}

// convertTempo is the speed a direction sets, if it sets one.
//
// Two ways of saying it, and a file may use either. A metronome mark is
// the printed one — a note, an equals sign and a number — and is what a
// reader sees. <sound tempo="…"> is the played one, in quarter notes per
// minute, and carries no instruction for the page.
//
// Taking only the metronome mark was wrong twice over: a word with a
// <sound> behind it — "Allegro Moderato", 114 to the quarter, which is
// how MuseScore writes every tempo it is not asked to print — set no
// speed at all, and the word itself was thrown away with it.
func convertTempo(d direction) (model.Tempo, bool) {
	text := d.words()
	if m := d.metronome(); m != nil {
		if bpm := m.bpm(); bpm > 0 {
			v, ok := typeValues[m.BeatUnit]
			if !ok {
				v = model.Quarter
			}
			unit := v.Duration()
			if m.Dot != nil {
				unit = unit.Scale(3, 2)
			}
			return model.Tempo{BPM: bpm, Unit: unit, Text: text}, true
		}
	}
	// No mark to print, but a speed to play at. Quarter notes per minute
	// is what the attribute means, whatever the meter is.
	if d.Sound != nil && d.Sound.Tempo > 0 {
		return model.Tempo{
			BPM:  int(d.Sound.Tempo + 0.5),
			Unit: model.Duration{Num: 1, Den: 4},
			Text: text,
			// Nothing was written on the page beyond the word, so
			// nothing but the word is drawn.
			Silent: true,
		}, true
	}
	return model.Tempo{}, false
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
			// A tremolo's arithmetic is not a tuplet.
			//
			// Two half notes shaken against each other are written as two
			// half notes in the time of one, because that is how long the
			// shake lasts — and what a reader sees is the strokes, not a
			// 2. The file says so by leaving the <tuplet> element out, or
			// by marking it unprinted; where it leaves it out the ratio
			// would otherwise be inferred and a figure drawn over music
			// that has none.
			if o := xn.Notations.Ornaments; o != nil && len(o.Tremolo) > 0 {
				m.Tuplet = &model.Tuplet{Hidden: true}
				r.explicit, r.start, r.stop = true, true, true
			}
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
	// moveTuplet follows a run from one set of marks to another, for when
	// what carries them changes after the run was filed.
	moveTuplet := func(from, to *model.Marks) {
		for i := range runs {
			if runs[i].marks == from {
				runs[i].marks = to
			}
		}
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
			// A rest is marked too: the fermata over the pause at the
			// end, the breath before the next phrase, a syllable held
			// across it.
			applyNotations(&r.Marks, xn)
			// A rest the file says not to print is space and not
			// silence: the other voice's music is there.
			if strings.EqualFold(xn.PrintObject, "no") {
				r.Hidden = true
			}
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
				joinChordMarks(&pending.Marks, &n.Marks)
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
					// The chord takes the first note's marks by value,
					// and the tuplets are not settled until the whole
					// voice has been read — so a run still pointing at
					// that note writes its figure onto something no
					// longer in the music. Every triplet of chords in
					// the piano part of Altès's 10me Solo lost its 3
					// this way: the notes were the right length and
					// nothing was written over them.
					moveTuplet(&prev.Marks, &pending.Marks)
					joinChordMarks(&pending.Marks, &n.Marks)
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

// joinChordMarks carries up to a chord the marks written on one of its
// noteheads that belong to the whole chord.
//
// A chord is one event written as several <note> elements, and the file is
// free to put a mark on any of them. The spread of an arpeggio is written
// on each notehead in turn; the syllable under a chord is written once, on
// whichever note the editor was on; a fermata, a trill, a tremolo, the
// word sung — all of them land wherever they land. gscore keeps a chord's
// marks on the chord and reads only those, so everything written on the
// second notehead onward was read and then dropped: four arpeggios in
// fifteen, four syllables in twelve, two tremolos in five.
//
// Not everything moves. A tie belongs to the notehead it is written on —
// holding the top note while the others move is ordinary writing — and the
// engraver reads a chord's ties from the noteheads as well as the chord,
// so they are left where they are.
func joinChordMarks(dst, src *model.Marks) {
	if dst.Arpeggio == nil {
		dst.Arpeggio = src.Arpeggio
	}
	if dst.Tremolo == nil {
		dst.Tremolo = src.Tremolo
	}
	if dst.Trill == nil {
		dst.Trill = src.Trill
	}
	if dst.Glissando == nil {
		dst.Glissando = src.Glissando
	}
	if dst.Chord == nil {
		dst.Chord = src.Chord
	}
	if dst.Break == model.NoBreath {
		dst.Break = src.Break
	}
	if dst.Dynamic == model.DynamicNone {
		dst.Dynamic = src.Dynamic
	}
	// A slur drawn from inside a chord is a slur from the chord: there is
	// one curve, however many noteheads it was written against.
	dst.SlurStart = model.MoreSlurs(dst.SlurStart, src.SlurStart)
	dst.SlurStop = model.MoreSlurs(dst.SlurStop, src.SlurStop)
	dst.Lyrics = append(dst.Lyrics, src.Lyrics...)
	dst.Fingering = append(dst.Fingering, src.Fingering...)
	dst.Texts = append(dst.Texts, src.Texts...)
	for _, a := range src.Articulations {
		dst.AddArticulation(a)
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

	// What has been written on each staff and is waiting for something to
	// be written against.
	//
	// A direction stands before the music it applies to: the dynamic, the
	// hairpin's open end, the "cresc.", the pedal going down. It belongs
	// to the next thing on that staff, which has not been made yet — the
	// voices are built afterwards — so each one is kept with the staff and
	// the moment it was written at, and applied once there is something
	// there to apply it to.
	var standing []marking
	queue := func(staff int, at model.Duration, f func(*model.Marks)) {
		standing = append(standing, marking{staff: staffOf(staff), at: at, apply: f})
	}
	// Grace notes waiting for the note they lean on.
	var graces []xmlNote
	// The chord symbol last written, waiting for its note.
	var harmony *model.ChordSymbol

	for _, it := range xm.Content {
		switch {
		case it.Harmony != nil:
			h := convertHarmony(it.Harmony)
			switch {
			case h == nil:
			case harmony == nil:
				harmony = h
			default:
				// Two symbols before one note: the chord changes partway
				// through it. gscore writes one symbol over a note, so
				// the one that sounds with the note is kept and the other
				// is said out loud rather than dropped in silence.
				lg.say("the chord symbol %q shares a note with %q and was "+
					"left out", h.String(), harmony.String())
			}

		case it.Direction != nil:
			d := it.Direction
			// A direction may be several things at once — a word and the
			// line that carries it, a dynamic and the wedge that follows
			// — so each is asked about in turn rather than chosen between.
			at := cursor
			if dy := d.dynamic(); dy != model.DynamicNone {
				queue(d.Staff, at, func(m *model.Marks) { m.Dynamic = dy })
			}
			// A span's two ends may land on the same note — a hairpin
			// over a single whole note, an 8va lasting one bar — and the
			// model holds a span as one mark with an end at each side, so
			// the second to arrive is merged into the first rather than
			// put in its place. Replacing it left a hairpin that only
			// ever stopped.
			if w := d.wedge(); w != nil {
				if h, ok := convertWedge(w); ok {
					queue(d.Staff, at, func(m *model.Marks) {
						if m.Hairpin == nil {
							m.Hairpin = &h
							return
						}
						m.Hairpin.Start = m.Hairpin.Start || h.Start
						m.Hairpin.Stop = m.Hairpin.Stop || h.Stop
						if h.Start {
							m.Hairpin.Kind = h.Kind
						}
					})
				}
			}
			if o := d.octave(); o != nil {
				if sh, ok := convertOctave(o, lg); ok {
					queue(d.Staff, at, func(m *model.Marks) {
						if m.Octave == nil {
							m.Octave = &sh
							return
						}
						m.Octave.Start = m.Octave.Start || sh.Start
						m.Octave.Stop = m.Octave.Stop || sh.Stop
						if sh.Start {
							m.Octave.Amount = sh.Amount
						}
					})
				}
			}
			if p := d.pedal(); p != nil {
				if pm, ok := convertPedal(p); ok {
					queue(d.Staff, at, func(m *model.Marks) {
						if m.Pedal == nil {
							m.Pedal = &pm
							return
						}
						m.Pedal.Start = m.Pedal.Start || pm.Start
						m.Pedal.Stop = m.Pedal.Stop || pm.Stop
						m.Pedal.Change = m.Pedal.Change || pm.Change
					})
				}
			}
			// A word over the staff: "dolce", "cresc.", "con sordino".
			// Not a tempo — those are read in convert, where the speed
			// they set belongs to the piece — and not a rehearsal mark,
			// which names the bar rather than saying anything about it.
			if _, isTempo := convertTempo(*d); !isTempo && d.rehearsal() == "" {
				if t, ok := convertWords(d); ok {
					queue(d.Staff, at, func(m *model.Marks) {
						m.Texts = append(m.Texts, t)
					})
				}
			}

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

			// The chord symbol written just before this note stands over
			// it. Only over a note that starts something: a symbol does
			// not belong to the second note of a chord.
			if harmony != nil && xn.Chord == nil {
				h := harmony
				queue(w.staff, at, func(m *model.Marks) { m.Chord = h })
				harmony = nil
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
	for _, k := range standing {
		if k.staff >= 1 && k.staff <= len(out) {
			k.mark(out[k.staff-1])
		}
	}
	return out
}

// readHeader is what goes above the first system: the title, whatever
// stands under it, and the people whose names are at the right.
//
// MusicXML says this in three places and a file may use any of them. The
// elements under <identification> are the catalogued form. <credit> is
// what is actually printed on the first page, and is the only place the
// format has for a subtitle. <miscellaneous> is where an encoder puts
// what the format has no element for — MuseScore writes the subtitle
// there, which is how "Op. 36" arrives.
//
// Reading only the creators lost the subtitle every time, and lost the
// arranger always: "subtitle" is not a kind of creator and never matched,
// and nothing looked for "arranger" at all.
func readHeader(s *model.Score, doc *scorePartwise) {
	if doc.Work != nil {
		s.Identity.Title = doc.Work.Title
	}

	subtitle := ""
	if doc.Ident != nil {
		s.Identity.Copyright = doc.Ident.Rights
		for _, f := range doc.Ident.Misc {
			if strings.EqualFold(f.Name, "subtitle") && subtitle == "" {
				subtitle = strings.TrimSpace(f.Value)
			}
		}
		// gscore itself used to write the subtitle as a kind of creator,
		// which is not one of the kinds there are. Files written that way
		// are out in the world and are still read.
		for _, c := range doc.Ident.Creators {
			if strings.EqualFold(c.Type, "subtitle") && subtitle == "" {
				subtitle = strings.TrimSpace(c.Name)
			}
		}
	}

	// What is printed on the first page, taken as it is set there.
	//
	// The credits win over the catalogue where there are any, and are not
	// merged with it: a file writes "Joseph Henry Altès" as its composer
	// and "Joseph Henry Altès / Bearbeitet von: Rainer Marwan (2025)" as
	// its credit, and those are the same two people said twice. What the
	// page says is the better of the two, because somebody set it.
	var credit []string
	for i := range doc.Credits {
		c := &doc.Credits[i]
		lines := c.lines()
		if len(lines) == 0 {
			continue
		}
		kind := ""
		if len(c.Type) > 0 {
			kind = strings.ToLower(strings.TrimSpace(c.Type[0]))
		}
		switch kind {
		case "title":
			if strings.TrimSpace(s.Identity.Title) == "" {
				s.Identity.Title = c.text()
			}
		case "subtitle":
			if subtitle == "" {
				subtitle = c.text()
			}
		case "arranger":
			// Its own field, not a line of the composer's name. The two
			// are different people and every format worth reading says so.
			if strings.TrimSpace(s.Identity.Arranger) == "" {
				s.Identity.Arranger = c.text()
			}
		case "composer", "lyricist", "poet":
			credit = append(credit, lines...)
		case "rights":
			if strings.TrimSpace(s.Identity.Copyright) == "" {
				s.Identity.Copyright = c.text()
			}
		}
	}

	// And the catalogue where the page said nothing: the names in the
	// order a title page sets them, which is who wrote it, who wrote the
	// words, and who arranged it.
	if len(credit) == 0 && doc.Ident != nil {
		seen := map[string]bool{}
		add := func(v string) {
			v = strings.TrimSpace(v)
			if v == "" || seen[strings.ToLower(v)] {
				return
			}
			seen[strings.ToLower(v)] = true
			credit = append(credit, v)
		}
		for _, want := range []string{"composer", "lyricist", "poet", ""} {
			for _, c := range doc.Ident.Creators {
				if strings.EqualFold(strings.TrimSpace(c.Type), want) {
					add(c.Name)
				}
			}
		}
	}

	// The arranger, where the page did not say it and the catalogue does.
	if strings.TrimSpace(s.Identity.Arranger) == "" && doc.Ident != nil {
		for _, c := range doc.Ident.Creators {
			if strings.EqualFold(strings.TrimSpace(c.Type), "arranger") {
				s.Identity.Arranger = strings.TrimSpace(c.Name)
				break
			}
		}
	}

	if subtitle == "" && doc.Work != nil {
		// A work number is what a subtitle usually holds anyway.
		subtitle = strings.TrimSpace(doc.Work.Number)
	}
	s.Identity.Subtitle = subtitle
	s.Identity.Composer = strings.Join(credit, "\n")
}

// convertWedge is a hairpin: where a crescendo opens and where it closes.
//
// The kind is only on the opening: "stop" says a hairpin ends here and not
// which one, because the file has already said. gscore reads the kind from
// the note that opens it too, so the end carries whatever is convenient.
func convertWedge(w *wedgeElem) (model.Hairpin, bool) {
	switch strings.ToLower(w.Type) {
	case "crescendo":
		return model.Hairpin{Kind: model.Crescendo, Start: true}, true
	case "diminuendo":
		return model.Hairpin{Kind: model.Diminuendo, Start: true}, true
	case "stop":
		return model.Hairpin{Stop: true}, true
	}
	// "continue" says a hairpin carries on through here, which is already
	// true of every note between its two ends.
	return model.Hairpin{}, false
}

// convertOctave is 8va and its relatives.
//
// The size is in notes and the model counts octaves: 8 is one, 15 two, 22
// three. And the direction is the other way round from how a reader says
// it — type="down" is the sign written *above* the staff, because the
// notes are written down an octave from where they sound.
func convertOctave(o *octaveShiftElem, lg *log) (model.OctaveShift, bool) {
	size := o.Size
	if size == 0 {
		size = 8
	}
	amount := 0
	switch size {
	case 8:
		amount = 1
	case 15:
		amount = 2
	case 22:
		amount = 3
	default:
		lg.say("an octave shift of %d notes is not one anybody writes; "+
			"it was read as an octave", size)
		amount = 1
	}
	switch strings.ToLower(o.Type) {
	case "down":
		return model.OctaveShift{Amount: amount, Start: true}, true
	case "up":
		return model.OctaveShift{Amount: -amount, Start: true}, true
	case "stop":
		return model.OctaveShift{Stop: true}, true
	}
	return model.OctaveShift{}, false
}

// convertPedal is the sustaining pedal going down, coming up, or being
// flicked without being released.
func convertPedal(p *pedalElem) (model.PedalMark, bool) {
	out := model.PedalMark{
		// "line=no" is the word "Ped." with a hook at the end; anything
		// else is the bracketed line.
		Text: strings.EqualFold(p.Line, "no"),
	}
	switch strings.ToLower(p.Type) {
	case "start", "sostenuto":
		out.Start = true
	case "stop", "discontinue":
		out.Stop = true
	case "change":
		out.Change = true
	default:
		return model.PedalMark{}, false
	}
	return out, true
}

// convertWords is a word written over or under the staff.
func convertWords(d *direction) (model.TextMark, bool) {
	text := d.words()
	if text == "" {
		return model.TextMark{}, false
	}
	out := model.TextMark{Text: text, Style: model.TextExpression}
	switch strings.ToLower(d.Placement) {
	case "above":
		out.Placement = model.PlacementAbove
	case "below":
		out.Placement = model.PlacementBelow
	}
	// How it was set. A word in italic is an expression mark — "dolce",
	// "cantabile" — and one in roman upright is usually an instruction.
	for _, t := range d.Types {
		for _, w := range t.Words {
			if strings.EqualFold(w.Style, "italic") {
				out.Italic = true
			}
			if strings.EqualFold(w.Weight, "bold") {
				out.Bold = true
			}
		}
	}
	// A word carried on by a dashed line is a direction that lasts —
	// "cresc. - - -" is not said once and done with.
	if d.dashed() {
		out.Style = model.TextDirection
	}
	return out, true
}

// convertHarmony is a chord symbol.
func convertHarmony(h *harmonyElem) *model.ChordSymbol {
	out := &model.ChordSymbol{}
	if h.Root != nil {
		out.Root = h.Root.name()
	}
	if h.Bass != nil {
		out.Bass = h.Bass.name()
	}
	// What the file wants printed, where it says: "maj7" rather than the
	// element name "major-seventh". The name is the fallback.
	switch {
	case strings.TrimSpace(h.Kind.Text) != "":
		out.Quality = strings.TrimSpace(h.Kind.Text)
	case strings.EqualFold(h.Kind.Value, "major"):
		// A plain major chord is written as its letter and nothing else.
	case strings.TrimSpace(h.Kind.Value) != "":
		out.Quality = chordKinds[strings.ToLower(strings.TrimSpace(h.Kind.Value))]
		if out.Quality == "" {
			out.Quality = strings.TrimSpace(h.Kind.Value)
		}
	}
	if out.Root == "" && out.Quality == "" && out.Bass == "" {
		return nil
	}
	return out
}

// chordKinds is how the specification's names for chords are usually
// printed. Only the ones a chart actually uses; anything else is written
// out as the file names it, which is at least readable.
var chordKinds = map[string]string{
	"major":              "",
	"minor":              "m",
	"augmented":          "+",
	"diminished":         "dim",
	"dominant":           "7",
	"major-seventh":      "maj7",
	"minor-seventh":      "m7",
	"diminished-seventh": "dim7",
	"augmented-seventh":  "+7",
	"half-diminished":    "m7b5",
	"major-minor":        "mMaj7",
	"major-sixth":        "6",
	"minor-sixth":        "m6",
	"dominant-ninth":     "9",
	"major-ninth":        "maj9",
	"minor-ninth":        "m9",
	"dominant-11th":      "11",
	"major-11th":         "maj11",
	"minor-11th":         "m11",
	"dominant-13th":      "13",
	"major-13th":         "maj13",
	"minor-13th":         "m13",
	"suspended-second":   "sus2",
	"suspended-fourth":   "sus4",
	"power":              "5",
	"none":               "N.C.",
}

// marking is something written on a staff at a moment in the bar, waiting
// for the note it belongs to.
type marking struct {
	staff int
	at    model.Duration
	apply func(*model.Marks)
}

// mark writes the marking onto the note it belongs to.
//
// The first one at or after the moment it was written, which is what a
// direction standing in front of the music means. Failing that, the last
// note before it: a hairpin closes after the note it reaches, and a word
// may be written against the end of a bar — and either dropped for want of
// something to the right of it takes a crescendo's far end with it.
func (k marking) mark(m *model.Measure) {
	var fallback *model.Marks
	for _, v := range m.Voices {
		t := model.Duration{Num: 0, Den: 1}
		for _, el := range v.Elements {
			mk := marksOfElement(el)
			switch {
			case mk == nil:
			case t.Cmp(k.at) >= 0:
				k.apply(mk)
				return
			default:
				fallback = mk
			}
			t = t.Add(el.Duration())
		}
		if fallback != nil {
			break
		}
	}
	if fallback != nil {
		k.apply(fallback)
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
		// And it stands. MusicXML keeps the pitch and the sign apart —
		// <alter> is what sounds, <accidental> is what was printed — so
		// an <accidental> here is the other program telling us what was
		// on its page, decided by whatever its own rules and its own
		// user said. Recomputing it would be gscore overruling that, and
		// the two will usually agree anyway; where they do not, the file
		// is the one that was looking at the page.
		//
		// A note with <alter> and no <accidental> says nothing about the
		// sign, and SpellAccidentals works that one out — which is how
		// the second B flat of a bar comes out without a flat on it,
		// where the first has one.
		n.Forced = true
	} else if xn.Pitch.Alter == nil {
		// Neither element: the file has said nothing about this note at
		// all, and gscore must not answer a question nobody asked.
		//
		// Strictly, no <alter> is an alteration of nothing, so a C in the
		// key of C flat major would be a C natural and would need a
		// natural printed on it. Files do not mean that. The test suite
		// alone has half a dozen that set a key signature and then write
		// bare pitches under it — ten notes in D major with not one
		// <alter> between them — and taken at its word every F and C in
		// them grows a natural that no printed copy of the music has.
		//
		// So a note the file left bare is left bare. It costs nothing
		// real: a file that means a note to be altered says so with
		// <alter>, which is the case just above, and the accidental it
		// needs is worked out there.
		n.Forced = true
	}
	switch xn.Stem {
	case "up":
		n.Stem = model.StemUp
	case "down":
		n.Stem = model.StemDown
	}

	applyNotations(&n.Marks, xn)
	return n
}

// applyNotations writes onto an element everything the file says about it
// that is not its pitch or its length.
//
// Separate from convertNote because a rest carries marks too, and used to
// carry none: the pause with a fermata over it — which is how half the
// endings in the repertoire are written — came back as a plain pause, and
// a syllable sung over a rest was dropped with it.
func applyNotations(n *model.Marks, xn *xmlNote) {
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
		// Counted, not set. A note may begin or end more than one curve
		// — MusicXML numbers them for exactly that reason, and the
		// specification's own 33c sample opens slur 1, opens slur 2 inside
		// it, and closes them in the other order. Set rather than counted,
		// the inner phrase was read as the outer one again and drawn once.
		for _, sl := range nt.Slurs {
			switch sl.Type {
			case "start":
				n.SlurStart++
			case "stop":
				n.SlurStop++
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
			// Where a player breathes, and the two strokes that stop a
			// phrase dead. The specification keeps them among the
			// articulations; a reader sees them between the notes.
			if a.Breath != nil {
				n.Break = model.BreathComma
				if strings.EqualFold(strings.TrimSpace(a.Breath.Value), "tick") {
					n.Break = model.BreathTick
				}
			}
			if a.Caesura != nil {
				n.Break = model.Caesura
			}
		}
		if o := nt.Ornaments; o != nil {
			applyOrnaments(n, o)
		}
		if t := nt.Technical; t != nil {
			// Fingering, which is most of what an exercise book is for.
			for _, f := range t.Fingering {
				if f = strings.TrimSpace(f); f != "" {
					n.Fingering = append(n.Fingering, f)
				}
			}
			if t.UpBow != nil {
				n.AddArticulation(model.UpBow)
			}
			if t.DownBow != nil {
				n.AddArticulation(model.DownBow)
			}
		}
		if a := nt.Arpeggiate; a != nil {
			n.Arpeggio = &model.Arpeggio{
				Down: strings.EqualFold(a.Direction, "down"),
			}
		} else if nt.NonArpeggiate != nil {
			n.Arpeggio = &model.Arpeggio{}
		}
		// A slide between two notes. <glissando> draws a wavy line and
		// <slide> a straight one; to a player they are the same
		// instruction, so the only difference kept is the line.
		for _, g := range nt.Glissando {
			applyGliss(n, g, true)
		}
		for _, g := range nt.Slide {
			applyGliss(n, g, false)
		}
	}
	// The words sung on this note.
	for _, l := range xn.Lyrics {
		if ly, ok := convertLyric(l); ok {
			n.Lyrics = append(n.Lyrics, ly)
		}
	}
	// A cue note is drawn small: an orchestral part showing what the
	// clarinet plays, so the player knows when to come in.
	if xn.Cue != nil {
		n.Small = true
	}
}

// applyOrnaments writes what stands over a note: a trill and the line that
// carries it, a turn, a mordent, the strokes of a tremolo.
func applyOrnaments(n *model.Marks, o *ornaments) {
	// The shake, and the wavy line saying how far it reaches.
	//
	// Both ends may be on the same note, and a file that means a shake on
	// one note says so three times over — a trill-mark, a line that
	// starts and a line that stops, all on that note. So each end is
	// turned on and none of them turns the other off; written the other
	// way, as an assignment per element, the last one read won and every
	// such trill came back as an end with no beginning.
	if o.Trill != nil || len(o.WavyLine) > 0 {
		if n.Trill == nil {
			n.Trill = &model.Trill{}
		}
	}
	if o.Trill != nil {
		n.Trill.Start = true
	}
	for _, w := range o.WavyLine {
		switch strings.ToLower(w.Type) {
		case "start":
			n.Trill.Start = true
		case "stop":
			n.Trill.Stop = true
		}
	}
	// A trill-mark with no line after it is a shake on this note and no
	// further: it ends where it began.
	if o.Trill != nil && len(o.WavyLine) == 0 {
		n.Trill.Stop = true
	}
	if o.Turn != nil {
		n.AddArticulation(model.Turn)
	}
	if o.TurnInv != nil {
		n.AddArticulation(model.TurnInverted)
	}
	if o.Mordent != nil {
		n.AddArticulation(model.Mordent)
	}
	if o.MordInv != nil {
		n.AddArticulation(model.ShortTrill)
	}
	for i := range o.Tremolo {
		t := &o.Tremolo[i]
		strokes, _ := strconv.Atoi(strings.TrimSpace(t.Strokes))
		if strokes <= 0 {
			strokes = 3
		}
		tr := &model.Tremolo{Strokes: strokes}
		switch strings.ToLower(t.Type) {
		case "start":
			tr.Start = true
		case "stop":
			tr.Stop = true
		case "unmeasured":
			tr.Buzz = true
		}
		n.Tremolo = tr
	}
}

// applyGliss marks one end of a slide between two notes.
func applyGliss(n *model.Marks, g glissElem, wavy bool) {
	if n.Glissando == nil {
		n.Glissando = &model.Glissando{Wavy: wavy}
	}
	switch strings.ToLower(g.Type) {
	case "start":
		n.Glissando.Start = true
		if s := strings.TrimSpace(g.Text); s != "" {
			n.Glissando.Text = s
		}
	case "stop":
		n.Glissando.Stop = true
	}
}

// convertLyric is one syllable under a note.
func convertLyric(l lyricElem) (model.Lyric, bool) {
	text := strings.Join(l.Text, "")
	if strings.TrimSpace(text) == "" && l.Extend == nil {
		return model.Lyric{}, false
	}
	out := model.Lyric{Text: text, Verse: 1, Extend: l.Extend != nil}
	// Numbered by the file where there is more than one line of words.
	// The attribute is text, because a file may name a line rather than
	// number it ("chorus"), and a name has no place in the order.
	if n, err := strconv.Atoi(strings.TrimSpace(l.Number)); err == nil && n > 0 {
		out.Verse = n
	}
	// Which part of a word this is, and so whether a hyphen follows it.
	if len(l.Syllabic) > 0 {
		switch strings.ToLower(strings.TrimSpace(l.Syllabic[0])) {
		case "begin":
			out.Syllabic = model.SyllableBegin
		case "middle":
			out.Syllabic = model.SyllableMiddle
		case "end":
			out.Syllabic = model.SyllableEnd
		default:
			out.Syllabic = model.SyllableSingle
		}
	}
	return out, true
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
