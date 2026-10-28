/*
 * import.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musescore

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Import reads a MuseScore file, zipped (.mscz) or not (.mscx).
func Import(r io.Reader) (*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("musescore: %w", err)
	}
	if isZip(data) {
		data, err = unwrap(data)
		if err != nil {
			return nil, err
		}
	}

	rd := &reader{seen: map[string]bool{}}
	doc, err := rd.read(data)
	if err != nil {
		return nil, err
	}
	s, err := doc.assemble(rd)
	if err != nil {
		return nil, err
	}
	for _, p := range s.Parts {
		balanceSlurs(p)
	}
	breakTiesAtMovements(s)
	if len(rd.skipped) > 0 {
		return s, &PartialError{Skipped: rd.skipped}
	}
	return s, nil
}

// breakTiesAtMovements unties a note held across the end of a movement.
//
// A held note does not survive the piece it is held in: the Allegretto ends
// and the Presto begins, in another key and another meter, and nothing is
// still ringing from before.
//
// Done here rather than while the staves are read, because where the
// movements divide is written on one staff and belongs to all of them —
// MuseScore puts the break on the first staff and the rest of the score
// never hears about it.
func breakTiesAtMovements(s *model.Score) {
	secs := s.Sections()
	if len(secs) < 2 {
		return
	}
	last := map[int]bool{}
	for _, sec := range secs[:len(secs)-1] {
		last[sec.Last] = true
	}
	for _, p := range s.Parts {
		for mi, m := range p.Measures {
			if !last[mi] {
				continue
			}
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					for _, n := range tiedNotesOf(el) {
						n.TieStart = false
					}
				}
			}
		}
	}
}

// tiedNotesOf is the noteheads of an element that can carry a tie.
func tiedNotesOf(el model.Element) []*model.Note {
	switch n := el.(type) {
	case *model.Note:
		return []*model.Note{n}
	case *model.Chord:
		return n.Notes
	}
	return nil
}

// balanceSlurs drops a slur that has only one of its two ends.
//
// MuseScore names both ends, so one without the other is one this reader
// failed to follow: a slur whose far end is on the other staff of a piano
// score, or on a chord it could not read, or in a voice that was not there.
// The Moonlight sonata is written in MuseScore 2 and has a few of each.
//
// It matters because of what the drawing does with one: a slur still open
// when a system ends is carried to the right margin and started again on
// the system after it, and the one after that, for the rest of the piece.
// One missing end is a line drawn across every remaining page.
//
// Dropping it loses a slur. Keeping it loses the music behind the line.
//
// The walk follows the same rules the engraver does, so what is left is
// exactly what will be drawn: one slur open at a time per line of music, a
// grace note slurring into the note it leans on, and a start arriving while
// one is already open being no new slur at all.
func balanceSlurs(p *model.Part) {
	most := 0
	for _, m := range p.Measures {
		if len(m.Voices) > most {
			most = len(m.Voices)
		}
	}
	for vi := 0; vi < most; vi++ {
		// Whatever carries the start of the slur now open: an element's own
		// marks, or a grace note leaning into one.
		var open *model.Marks
		var openGrace *model.Note

		for _, m := range p.Measures {
			if vi >= len(m.Voices) {
				continue
			}
			for _, el := range m.Voices[vi].Elements {
				mk := marksOf(el)
				if mk == nil {
					continue
				}
				// A grace note slurs into the note it decorates, so its
				// start comes before that note's own marks.
				if mk.Grace != nil {
					for _, g := range mk.Grace.Notes {
						if !g.Marks.SlurStart {
							continue
						}
						if open == nil && openGrace == nil {
							openGrace = g
						} else {
							// Nothing is drawn from it: there is already a
							// slur open on this line.
							g.Marks.SlurStart = false
						}
					}
				}
				if mk.SlurStop {
					if open == nil && openGrace == nil {
						mk.SlurStop = false // nothing to end
					} else {
						open, openGrace = nil, nil
					}
				}
				if mk.SlurStart {
					if open == nil && openGrace == nil {
						open = mk
					} else {
						mk.SlurStart = false
					}
				}
			}
		}
		// Whatever is still open was never ended.
		if open != nil {
			open.SlurStart = false
		}
		if openGrace != nil {
			openGrace.Marks.SlurStart = false
		}
	}
}

func isZip(b []byte) bool {
	return len(b) > 4 && b[0] == 'P' && b[1] == 'K' && (b[2] == 3 || b[2] == 5)
}

// unwrap pulls the score out of a .mscz, which is a zip holding one .mscx —
// the score as XML — beside a style sheet, a thumbnail and the playback
// settings.
//
// META-INF/container.xml lists what is inside, but unlike an .mxl it lists
// everything rather than naming the score, so the extension is what decides.
func unwrap(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("musescore: that is not a readable .mscz: %w", err)
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "META-INF/") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(f.Name), ".mscx") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("musescore: %w", err)
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, errors.New("musescore: no .mscx score inside the .mscz")
}

/*
 * The reader
 */

// reader walks the document and writes down what it could not use.
type reader struct {
	dec     *xml.Decoder
	version string
	skipped []string
	// seen keeps each complaint to one line. A tremolo gscore cannot draw is
	// worth saying once; said six hundred times it buries everything else.
	seen map[string]bool
}

// report records something that could not be read, once per distinct message.
func (r *reader) report(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if r.seen[msg] {
		return
	}
	r.seen[msg] = true
	r.skipped = append(r.skipped, msg)
}

// unhandled records an element the reader has no use for, and skips it.
func (r *reader) unhandled(where string, e xml.StartElement) error {
	r.report("%s in a %s is not read", e.Name.Local, where)
	return r.dec.Skip()
}

// children calls fn for every child element, and is the only way this reader
// descends. fn consumes the element it is given — by reading it, by skipping
// it, or by descending again — and the loop stops at the closing tag of the
// element children was called inside.
func (r *reader) children(fn func(xml.StartElement) error) error {
	for {
		tok, err := r.dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := fn(t); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// text reads the element that has just started as a string, markup and all.
//
// A MuseScore <text> is rich: fonts, symbols and bold runs are elements
// inside it. The words are what gscore can use, so the tags are dropped and
// the characters kept — all but <sym>, whose content is the name of a glyph
// rather than anything anybody wrote. Kept, a tempo marking came out reading
// "metNoteQuarterUp = 150".
func (r *reader) text() (string, error) {
	var buf strings.Builder
	depth := 0
	for {
		tok, err := r.dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			buf.Write(t)
		case xml.StartElement:
			if t.Name.Local == "sym" {
				if err := r.dec.Skip(); err != nil {
					return "", err
				}
				continue
			}
			depth++
		case xml.EndElement:
			if depth == 0 {
				return strings.TrimSpace(buf.String()), nil
			}
			depth--
		}
	}
}

// number reads an element whose content is an integer. A number that is not
// one reads as zero rather than stopping the file.
func (r *reader) number() (int, error) {
	s, err := r.text()
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(s)
	return n, nil
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

/*
 * The document as MuseScore lays it out
 *
 * A file is a list of <Part>s, which say what the instruments are and which
 * staves each one is printed on, followed by a <Staff> per staff holding the
 * bars. The two are joined by an id, which is why neither can be turned into
 * a gscore part until both have been read.
 */

type document struct {
	meta   map[string]string
	parts  []*msPart
	staves map[string]*msStaff
	order  []string
	tempo  model.Tempo
	// nextPartStaff and nextMusicStaff number the staves that are written
	// without an id. MuseScore 4 stopped putting one on a part's staves and
	// numbers them in the order they appear instead, so the two lists are
	// joined by counting rather than by matching.
	nextPartStaff  int
	nextMusicStaff int
	// title and the rest come from the text frame at the head of the score,
	// which is what is actually printed. The metaTags are the fallback: they
	// are often left at "Untitled score" while the page says otherwise.
	title, subtitle, composer string
	// tempoSet says the speed the piece starts at has been read, so that a
	// later marking is a change rather than the opening tempo again.
	tempoSet bool
}

type msPart struct {
	name string
	// sound is the instrument's own name for itself, from <instrumentId>.
	sound    string
	program  int
	hasProg  bool
	transDia int
	transChr int
	staffIDs []string
	bracket  string
	barSpan  bool

	// staffClefs is the clef each of this part's staves starts in, where the
	// staff says so itself, and staffGroups what kind of staff it is.
	//
	// They are needed because MuseScore writes a <Clef> into the music only
	// where the clef is not the one the instrument would have anyway: a
	// piano's left hand carries no clef at all in its bars, and reading the
	// bars alone put it in the treble.
	staffClefs  []string
	staffGroups []string

	// tuning is the open strings of a fretted instrument, in the order the
	// file lists them — lowest first — and frets is how many it has. A part
	// written as tablature needs both: the lines are the strings, and what
	// is written on them is counted from the open pitch.
	tuning []model.Pitch
	frets  int

	// instClef is the clef the instrument is written in, and instClefN the
	// clefs it gives particular staves — a piano names F for its second.
	instClef  string
	instClefN map[int]string
}

type msStaff struct {
	id       string
	clef     model.Clef
	clefSet  bool
	measures []*model.Measure

	// tick is where this staff has got to, counted from the start of the
	// score. MuseScore's <tick> is an absolute position, not an offset into
	// the bar it is written in, so the bar's own start has to be subtracted
	// from it — and the only way to know that is to have been counting.
	tick int

	// clefWait is a change of clef that has not reached a note yet.
	//
	// MuseScore writes the change at the tick it happens, and a change
	// that happens at a barline is written after the last note of the bar
	// before — where it is also drawn, as a warning to the player. There
	// is no note left in that bar to carry it, so it was dropped: the
	// Moonlight sonata's left hand went up to the treble in bar 189 and
	// never came back, and ninety bars of it were drawn two clefs low,
	// five and six ledger lines under the staff.
	clefWait *model.Clef

	// spans are the long marks a MuseScore 1 or 2 file has opened and not
	// closed yet, by the id its <endSpanner> will name.
	spans map[string]*msSpan

	// lastEl is the last note or rest read on this staff, in whichever bar
	// and voice it fell. An <endSpanner> is written at the tick where the
	// mark stops, which is often the first thing in the bar after it, so
	// the note it ends on can be in the bar before.
	lastEl model.Element

	// title is a movement's name, waiting for the bar it belongs to.
	//
	// MuseScore writes it in a text frame standing between two bars, so it
	// arrives before the music it heads. A frame before any bar at all is
	// the score's own title block and belongs to the whole piece; one
	// further down names the movement that starts after it.
	title string

	// transChr is the staff's transposition in semitones, written to
	// sounding, taken from the part that owns it. MuseScore records the
	// sounding pitch of every note and gscore records the written one.
	transChr int

	// octave is the ottava in force, in octaves. The same difference again:
	// MuseScore records what sounds and draws it shifted, so the line has to
	// be taken back off to get what is printed.
	octave int
	// octaveEnds closes the shift after the element that carries the end of
	// the line, which is the last one under it.
	octaveEnds bool
}

func (r *reader) read(data []byte) (*document, error) {
	r.dec = xml.NewDecoder(bytes.NewReader(data))
	// A score may carry text in a character set the declaration does not
	// name, and refusing to open it over one byte would be absurd.
	r.dec.Strict = false
	doc := &document{
		meta:           map[string]string{},
		staves:         map[string]*msStaff{},
		tempo:          model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		nextPartStaff:  1,
		nextMusicStaff: 1,
	}

	found := false
	for !found {
		tok, err := r.dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("musescore: %w", err)
		}
		e, ok := tok.(xml.StartElement)
		if !ok || e.Name.Local != "museScore" {
			continue
		}
		found = true
		r.version = attr(e, "version")
		if err := r.museScore(doc); err != nil && err != io.EOF {
			return nil, fmt.Errorf("musescore: %w", err)
		}
	}
	if !found {
		return nil, errors.New("musescore: no museScore element; this is not a MuseScore file")
	}
	if len(doc.order) == 0 {
		return nil, errors.New("musescore: the file holds no staves")
	}
	if major := strings.SplitN(r.version, ".", 2)[0]; major != "4" && major != "" {
		r.report("this is a MuseScore %s file; the reader is written for 4", r.version)
	}
	return doc, nil
}

func (r *reader) museScore(doc *document) error {
	return r.children(func(e xml.StartElement) error {
		if e.Name.Local != "Score" {
			return r.dec.Skip()
		}
		return r.score(doc)
	})
}

func (r *reader) score(doc *document) error {
	return r.children(func(e xml.StartElement) error {
		switch e.Name.Local {
		case "metaTag":
			v, err := r.text()
			if err != nil {
				return err
			}
			doc.meta[attr(e, "name")] = v
			return nil
		case "Part":
			return r.part(doc)
		case "Staff":
			return r.staff(doc, numbered(attr(e, "id"), &doc.nextMusicStaff))
		default:
			// A nested <Score> is a linked part: the same music printed on
			// its own, which gscore makes from the score instead. Everything
			// else here is style, ordering and synthesiser settings — how
			// MuseScore shows the file rather than what is in it.
			return r.dec.Skip()
		}
	})
}

// numbered gives a staff its number, counting when the file does not say.
//
// MuseScore 4 writes a part's staves without an id and relies on their order:
// the first part's staves are 1 and 2, the next part's is 3, and so on. The
// counter is kept in step with the ids that are given, so a file that writes
// some of them and not others still lines up.
func numbered(given string, next *int) string {
	if given != "" {
		if n, err := strconv.Atoi(given); err == nil && n >= *next {
			*next = n + 1
		}
		return given
	}
	id := strconv.Itoa(*next)
	*next++
	return id
}

func (r *reader) part(doc *document) error {
	p := &msPart{}
	err := r.children(func(e xml.StartElement) error {
		switch e.Name.Local {
		case "Staff":
			id := numbered(attr(e, "id"), &doc.nextPartStaff)
			clef, group, err := r.partStaff(p)
			p.staffIDs = append(p.staffIDs, id)
			p.staffClefs = append(p.staffClefs, clef)
			p.staffGroups = append(p.staffGroups, group)
			return err
		case "trackName":
			s, err := r.text()
			if err != nil {
				return err
			}
			if p.name == "" {
				p.name = s
			}
			return nil
		case "Instrument":
			return r.instrument(p)
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	doc.parts = append(doc.parts, p)
	return nil
}

func (r *reader) partStaff(p *msPart) (clef, group string, err error) {
	err = r.children(func(e xml.StartElement) error {
		switch e.Name.Local {
		case "bracket":
			if p.bracket == "" {
				p.bracket = attr(e, "type")
			}
			return r.dec.Skip()
		case "barLineSpan":
			n, err := r.number()
			p.barSpan = p.barSpan || n > 0
			return err
		case "defaultClef", "defaultConcertClef":
			s, err := r.text()
			if s != "" {
				clef = s
			}
			return err
		case "StaffType":
			group = attr(e, "group")
			return r.dec.Skip()
		default:
			return r.dec.Skip()
		}
	})
	return clef, group, err
}

// stringData reads how a fretted instrument is tuned.
//
// The strings are listed lowest first, as MIDI numbers. They are kept in
// that order here and turned round where the part is built, because the tab
// is drawn from the top line down and everything after that point counts the
// same way.
func (r *reader) stringData(p *msPart) error {
	return r.children(func(e xml.StartElement) error {
		switch e.Name.Local {
		case "string":
			n, err := r.number()
			if err != nil {
				return err
			}
			p.tuning = append(p.tuning, midiPitch(n))
			return nil
		case "frets":
			n, err := r.number()
			p.frets = n
			return err
		default:
			return r.dec.Skip()
		}
	})
}

func (r *reader) instrument(p *msPart) error {
	return r.children(func(e xml.StartElement) error {
		switch e.Name.Local {
		case "longName":
			s, err := r.text()
			if err != nil {
				return err
			}
			if s != "" {
				p.name = s
			}
			return nil
		case "trackName":
			s, err := r.text()
			if err != nil {
				return err
			}
			if p.name == "" {
				p.name = s
			}
			return nil
		case "instrumentId":
			// What the instrument is, as against what it is played with.
			// MuseScore writes both and gscore read only the program, so
			// the Ninth's Controfagotto arrived as a bassoon: there is no
			// contrabassoon in General MIDI, and nothing else in the file
			// says which of the two it is.
			id, err := r.text()
			if id != "" {
				p.sound = id
			}
			return err
		case "StringData":
			return r.stringData(p)
		case "transposeDiatonic":
			n, err := r.number()
			p.transDia = n
			return err
		case "transposeChromatic":
			n, err := r.number()
			p.transChr = n
			return err
		case "concertClef":
			s, err := r.text()
			if s != "" {
				p.instClef = s
			}
			return err
		case "clef":
			// <clef staff="2">F</clef>: what the instrument writes its
			// second staff in, which is how a piano says where its left
			// hand goes.
			n, _ := strconv.Atoi(attr(e, "staff"))
			s, err := r.text()
			if n > 0 && s != "" {
				if p.instClefN == nil {
					p.instClefN = map[int]string{}
				}
				p.instClefN[n] = s
			}
			return err
		case "Channel":
			return r.children(func(c xml.StartElement) error {
				if c.Name.Local == "program" && !p.hasProg {
					if n, err := strconv.Atoi(attr(c, "value")); err == nil {
						p.program, p.hasProg = n, true
					}
				}
				return r.dec.Skip()
			})
		default:
			return r.dec.Skip()
		}
	})
}

/*
 * The music
 */

func (r *reader) staff(doc *document, id string) error {
	st, ok := doc.staves[id]
	if !ok {
		st = &msStaff{id: id, transChr: doc.transposeOf(id)}
		st.clef = doc.defaultClef(id)
		doc.staves[id] = st
		doc.order = append(doc.order, id)
	}
	err := r.children(func(e xml.StartElement) error {
		switch e.Name.Local {
		case "Measure":
			return r.measure(doc, st)
		case "VBox", "HBox", "TBox", "FBox":
			return r.frame(doc, st)
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	closeTies(st.measures)
	return nil
}

/*
 * Ties
 */

// closeTies joins each tie to the note it runs to.
//
// MuseScore writes a tie on the note it starts from and writes nothing at
// all on the note it reaches: the end is simply wherever the next note of
// that pitch in that voice falls, barline or no barline. Read literally,
// every tie opened and none ever shut — a Goldberg Variations came in with
// six hundred and eighty-five ties open and no stops, and the engraver drew
// each of them running off to the end of its system. A page of that looks
// like slurs that never finish.
//
// A tie that reaches nothing is dropped rather than left hanging, because a
// curve to nowhere is worse than no curve.
func closeTies(measures []*model.Measure) {
	// A voice carries on across the barline but is stored per bar, so the
	// voices are walked by index to get one run of music at a time.
	most := 0
	for _, m := range measures {
		if n := len(m.Voices); n > most {
			most = n
		}
	}
	for v := 0; v < most; v++ {
		var seq []model.Element
		// Which bar each of them is in, because how far apart two notes are
		// is what says whether one can be tied to the other.
		var at []int
		for mi, m := range measures {
			if v < len(m.Voices) && m.Voices[v] != nil {
				for _, el := range m.Voices[v].Elements {
					seq = append(seq, el)
					at = append(at, mi)
				}
			}
		}
		closeTieRun(seq, at)
	}
}

// nextBarAlong says the note after this one is near enough to be tied to.
//
// A tie joins a note to the one that follows it, so the two are in the same
// bar or on either side of one barline and never further apart. "The next
// element of this voice" is not the same thing: a voice that stops and
// starts again leaves a gap, and the next element after the gap may be
// anywhere.
//
// The left hand of the Moonlight sonata has a voice that holds a note in
// bar 131 and does not appear again until bar 142, in the next movement.
// The tie was drawn the whole way — a line across two systems and through
// two staves of music, from a note in the Allegretto to one in the Presto.
func nextBarAlong(at []int, i int) bool {
	return i+1 < len(at) && at[i+1]-at[i] <= 1
}

// closeTieRun marks the far end of every tie in one run of a voice.
func closeTieRun(seq []model.Element, at []int) {
	// A tie with nothing to tie to is not drawn.
	//
	// It used to be left as the file wrote it, on the reasoning that
	// guessing a file is wrong is how an importer loses music somebody
	// meant. That is true of the music and false of the drawing: a tie the
	// engraver cannot close is carried to the right margin of its system
	// and begun again on the next, and the next, to the end of the piece.
	// The left hand of the Moonlight sonata has one — a voice that holds a
	// note in bar 131 and does not appear again until bar 142, in another
	// movement — and it was drawn as a line through two staves of the
	// Presto.
	//
	// So the start goes with the stop: either the tie has both ends or it
	// has neither.
	for i, el := range seq {
		notes, whole := tieNotes(el)
		made := false
		for _, n := range notes {
			if !n.TieStart && !whole {
				continue
			}
			var to *model.Note
			if nextBarAlong(at, i) {
				to = tieTarget(seq, i+1, n.Pitch)
			}
			if to != nil {
				to.TieStop = true
				made = true
				continue
			}
			n.TieStart = false
		}
		// A chord may be tied as a whole rather than notehead by notehead.
		// Where not one of its notes found anywhere to go, the chord has
		// nothing left to say either.
		if c, ok := el.(*model.Chord); ok && c.TieStart && !made {
			c.TieStart = false
		}
	}
}

// tieNotes is the noteheads of an element, and whether the element itself
// carries a tie covering all of them.
func tieNotes(el model.Element) ([]*model.Note, bool) {
	switch v := el.(type) {
	case *model.Note:
		return []*model.Note{v}, false
	case *model.Chord:
		return v.Notes, v.TieStart
	}
	return nil, false
}

// tieTarget is the notehead a tie starting at pitch runs to.
//
// The very next element, and no further: a tie joins one note to the note
// after it. Anything else — a rest, a different pitch — means the tie has
// nowhere to go, which is a file saying something it does not mean.
func tieTarget(seq []model.Element, from int, pitch model.Pitch) *model.Note {
	if from >= len(seq) {
		return nil
	}
	notes, _ := tieNotes(seq[from])
	for _, n := range notes {
		if n.Pitch == pitch {
			return n
		}
	}
	return nil
}

// transposeOf finds the transposition of whichever part owns a staff. The
// parts are written before the staves, so by the time a staff's bars are read
// its instrument is already known.
func (doc *document) transposeOf(id string) int {
	for _, p := range doc.parts {
		for _, s := range p.staffIDs {
			if s == id {
				return p.transChr
			}
		}
	}
	return 0
}

// defaultClef is the clef a staff starts in when its bars do not say.
//
// MuseScore writes a <Clef> into the music only where it differs from what
// the instrument would use, so for most staves the answer is here and nowhere
// else.
func (doc *document) defaultClef(id string) model.Clef {
	name, group := "", ""
	for _, p := range doc.parts {
		for i, sid := range p.staffIDs {
			if sid != id {
				continue
			}
			if i < len(p.staffGroups) {
				group = p.staffGroups[i]
			}
			switch {
			case i < len(p.staffClefs) && p.staffClefs[i] != "":
				name = p.staffClefs[i]
			case p.instClefN[i+1] != "":
				name = p.instClefN[i+1]
			default:
				name = p.instClef
			}
		}
	}
	if c, ok := clefKinds[name]; ok {
		return c
	}
	if group == "percussion" {
		return model.Percussion
	}
	return model.Treble
}

// frame reads the box at the head of the score, which is where the title and
// the composer are actually written.
// frame reads a text frame: the score's title block, or the name of the
// movement that begins after it.
//
// Which it is, is where it stands. A frame before any bar has been read
// heads the whole piece; one between two bars heads what follows it — and
// those were thrown away as a second title, so the Moonlight sonata's
// "Movement III" was nowhere on the page and the Presto began with nothing
// to say it had.
func (r *reader) frame(doc *document, st *msStaff) error {
	return r.children(func(e xml.StartElement) error {
		if e.Name.Local != "Text" {
			return r.dec.Skip()
		}
		style, body := "", ""
		err := r.children(func(c xml.StartElement) error {
			switch c.Name.Local {
			case "style":
				s, err := r.text()
				style = strings.ToLower(s)
				return err
			case "text":
				s, err := r.text()
				body = s
				return err
			default:
				return r.dec.Skip()
			}
		})
		if err != nil {
			return err
		}
		switch style {
		case "movement title":
			// Said outright: the name of the movement that begins after
			// this frame. MuseScore heads each one with its own, under the
			// work's title — "Movement III" over the Presto.
			if st != nil {
				st.title = body
			}
		case "title":
			if st != nil && len(st.measures) > 0 {
				// Down the score rather than at the head of it, so it names
				// what comes next rather than the whole piece. Thrown away
				// as a second title, a movement's name was nowhere on the
				// page and the music simply began.
				st.title = body
				return nil
			}
			if doc.title == "" {
				doc.title = body
			}
		case "subtitle":
			if doc.subtitle == "" {
				doc.subtitle = body
			}
		case "composer":
			if doc.composer == "" {
				doc.composer = body
			}
		}
		return nil
	})
}

/*
 * One bar
 */

func (r *reader) measure(doc *document, st *msStaff) error {
	m := &model.Measure{RightBarline: model.BarSingle}
	if st.title != "" {
		// The frame that named this movement stood just before this bar.
		m.SectionTitle, st.title = st.title, ""
	}
	start := st.tick
	var loose *voiceState   // MuseScore 2 wrote voice one without a wrapper
	at, pending := 0, false // where <tick> last asked for, and whether one has
	startRepeat := false

	// fresh opens a voice, handing the first one of the bar any change of
	// clef the bar before ran out of notes to carry.
	carried := false
	fresh := func() *voiceState {
		vs := newVoice(m)
		if !carried && st.clefWait != nil {
			carried = true
			clef := *st.clefWait
			vs.pending = append(vs.pending, func(mk *model.Marks) {
				mk.Clef = &clef
				st.clefWait = nil
			})
		}
		return vs
	}

	// place moves the write position to where the last <tick> asked for.
	//
	// MuseScore 2 and earlier write <tick> before anything whose place in
	// the bar is not simply "after the note before it", and a rewind to
	// somewhere earlier is how they begin the bar's next voice. Acting on
	// every one of them cut bars into pieces: a hairpin that ends halfway
	// through a dotted half is written with a <tick> too, and the voice it
	// belonged to was closed under it and the mark thrown away with it.
	// Only a tick that a note or a rest follows moves anything.
	// queued are the long marks read since the last <tick>, waiting for
	// the voice that tick opens. MuseScore writes them at the position the
	// tick names, and until a note or a rest arrives there it is not known
	// whether that position belongs to the voice already here or begins
	// the next one.
	var queued []func(*voiceState)

	place := func() {
		n, moved := at, pending
		at, pending = 0, false
		filled := 0
		if loose != nil {
			filled = voiceTicks(loose.v)
		}
		switch {
		case loose == nil, moved && n < filled:
			// A rewind: the bar's next voice starts here. Instead of
			// wrapping each in <voice>, MuseScore 2 writes them one after
			// another inside the bar and rewinds before each. Read
			// straight through, the second voice was laid end to end
			// after the first and the bar came out holding twice the
			// music it should — which is what put the two staves of a
			// piano out of step with each other, and why a bar of
			// three-four could hold five notes.
			if loose != nil {
				r.closeVoice(loose)
				m.Voices = append(m.Voices, loose.v)
			}
			loose = fresh()
			// A voice that comes in part way through the bar is spaced to
			// its entry by a rest that takes room without being printed.
			// MuseScore leaves those rests out: the voice below is
			// already holding that beat and a second rest on top of it
			// would only be something else to read.
			if d, ok := tickDuration(n); ok && n > 0 {
				loose.v.Elements = append(loose.v.Elements,
					&model.Rest{Dur: d, Hidden: true})
			}
		case moved && n > filled:
			// A gap in the voice that is already here, spaced the same way.
			if d, ok := tickDuration(n - filled); ok {
				loose.v.Elements = append(loose.v.Elements,
					&model.Rest{Dur: d, Hidden: true})
			}
		}
		for _, f := range queued {
			f(loose)
		}
		queued = nil
	}

	// apply gives a long mark the voice it belongs to, which is the voice
	// at the write position — and while a <tick> is still waiting, that
	// voice may not have been made yet. Bar 54 of the Moonlight sonata
	// opens a crescendo on the left hand at the first beat, behind a right
	// hand that already fills the bar: read against the voice standing
	// there, the mark was thrown away when that voice was closed.
	apply := func(f func(*voiceState)) {
		if pending {
			queued = append(queued, f)
			return
		}
		if loose == nil {
			loose = fresh()
		}
		f(loose)
	}

	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "voice":
			vs := fresh()
			if err := r.voice(doc, st, vs); err != nil {
				return err
			}
			m.Voices = append(m.Voices, vs.v)
			return nil
		case "tick":
			n, err := r.number()
			if err != nil {
				return err
			}
			// Absolute, from the start of the score. In the first bar the
			// two readings agree, which is exactly long enough to be fooled
			// by: by bar forty a tick taken as an offset put thirty-nine
			// whole notes of silence in front of the voice, and the bar was
			// drawn forty times too wide with its notes at either end.
			at, pending = n-start, true
			return nil
		case "startRepeat":
			startRepeat = true
			return r.dec.Skip()
		case "endRepeat":
			if _, err := r.number(); err != nil {
				return err
			}
			m.RightBarline = model.BarRepeatEnd
			return nil
		case "Marker":
			return r.marker(m)
		case "Jump":
			return r.jump(m)
		case "LayoutBreak":
			// Most of these are about where the page turns and say nothing
			// about the music. A section break does: it is where one
			// movement ends and another begins, and everything that sends
			// a player somewhere else is written as though its own
			// movement were the whole piece. Without it the Moonlight
			// sonata's "D.C. al Fine" at the end of the second movement
			// went back to the first bar of the first, stopped at a Fine
			// halfway through the second, and the third movement was never
			// played at all.
			return r.children(func(c xml.StartElement) error {
				if c.Name.Local != "subtype" {
					return r.dec.Skip()
				}
				v, err := r.text()
				if v == "section" {
					m.EndsSection = true
				}
				return err
			})
		case "MeasureNumber", "vspacerUp", "vspacerDown",
			"noOffset", "irregular", "stretch", "multiMeasureRest",
			// breakMultiMeasureRest asks MuseScore not to fold this bar
			// into the long rest around it. gscore does not fold bars into
			// long rests, so there is nothing for it to ask: reported as
			// unread it was the one complaint Beethoven's ninth raised,
			// about a bar that reads perfectly.
			"breakMultiMeasureRest",
			"measureRepeatCount", "excludeFromOtherParts", "eid", "visible",
			"sysInitBarLineType", "lineBreak", "pageBreak", "StaffTypeChange":
			return r.dec.Skip()
		default:
			f, err := r.oldSpan(st, c)
			if err != nil {
				return err
			}
			if f != nil {
				apply(f)
				return nil
			}
			place()
			return r.voiceElement(doc, st, loose, c)
		}
	})
	if err != nil {
		return err
	}
	if len(queued) > 0 {
		// A mark written after the bar's last note, where no tick of its
		// own ever arrived. It belongs to the music that is there.
		if loose == nil {
			loose = fresh()
		}
		for _, f := range queued {
			f(loose)
		}
	}
	if loose != nil {
		r.closeVoice(loose)
		m.Voices = append(m.Voices, loose.v)
	}
	if len(m.Voices) == 0 {
		m.Voices = append(m.Voices, &model.Voice{})
	}
	// Carry the count on to the next bar. The bar's length is the longest
	// of its voices: they all fill it, and whichever reaches furthest says
	// how far it reaches.
	longest := 0
	for _, v := range m.Voices {
		if n := voiceTicks(v); n > longest {
			longest = n
		}
	}
	st.tick += longest
	if startRepeat {
		// A repeat opens on the barline between this bar and the one before
		// it, and gscore keeps a barline on the bar it closes.
		if n := len(st.measures); n > 0 {
			prev := st.measures[n-1]
			if prev.RightBarline == model.BarRepeatEnd {
				prev.RightBarline = model.BarRepeatBoth
			} else {
				prev.RightBarline = model.BarRepeatStart
			}
		} else {
			r.report("a repeat opening on the first bar is not drawn")
		}
	}
	st.measures = append(st.measures, m)
	return nil
}

func (r *reader) marker(m *model.Measure) error {
	kind, named, label := "", "", ""
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "markerType":
			s, err := r.text()
			kind = strings.ToLower(s)
			return err
		case "label":
			// MuseScore 4 often writes only this: the type is left out and
			// the label — "segno", "codab" — is the only thing that says
			// what the mark is.
			s, err := r.text()
			named = strings.ToLower(s)
			return err
		case "text":
			s, err := r.text()
			label = s
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	if kind == "" {
		kind = named
	}
	switch kind {
	case "segno", "varsegno":
		m.Sign = model.Segno
	case "coda", "codab", "varcoda":
		m.Sign = model.Coda
	case "fine":
		m.Sign = model.Fine
	case "tocoda", "varcodab":
		m.Sign = model.ToCoda
	case "user", "":
		// A rehearsal mark written as a marker rather than as a
		// RehearsalMark, which is what the boxed letters in a chart are.
		if label != "" && m.Marker == "" {
			m.Marker = label
		}
	default:
		r.report("the %q marker is not read", kind)
	}
	return nil
}

func (r *reader) jump(m *model.Measure) error {
	to, until := "", ""
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "jumpTo":
			s, err := r.text()
			to = strings.ToLower(s)
			return err
		case "playUntil":
			s, err := r.text()
			until = strings.ToLower(s)
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	j := &model.Jump{}
	if strings.Contains(to, "segno") {
		j.From = model.FromSegno
	}
	switch {
	case strings.Contains(until, "fine"):
		j.Until = model.UntilFine
	case strings.Contains(until, "coda"):
		j.Until = model.UntilCoda
	}
	m.Jump = j
	return nil
}

/*
 * One voice
 *
 * voiceState is what a voice carries from one element to the next: the marks
 * that are written before the note they belong to, and the tuplet that is
 * open.
 */

type voiceState struct {
	v *model.Voice
	m *model.Measure

	// pending are marks waiting for the note they belong to. MuseScore
	// writes a slur's end, a dynamic or a chord symbol at the tick it falls
	// on, which comes before the note at that tick.
	pending  []func(*model.Marks)
	dynamic  model.Dynamic
	artics   []model.Articulation
	hairpin  *model.Hairpin
	octave   *model.OctaveShift
	chordSym *model.ChordSymbol
	texts    []model.TextMark
	grace    *model.GraceGroup

	tuplet   *model.Tuplet
	opens    bool // the next element opens the tuplet
	inTuplet []model.Element

	last model.Element
}

func newVoice(m *model.Measure) *voiceState {
	return &voiceState{v: &model.Voice{}, m: m}
}

func (r *reader) voice(doc *document, st *msStaff, vs *voiceState) error {
	err := r.children(func(e xml.StartElement) error {
		return r.voiceElement(doc, st, vs, e)
	})
	if err != nil {
		return err
	}
	r.closeVoice(vs)
	return nil
}

// closeVoice puts whatever is still waiting onto the last note of the voice.
//
// A hairpin or an octave line that ends at the end of a bar is written after
// the note it ends on, with nothing following it, so there is no next element
// to hang it on. The last one is the one it means.
func (r *reader) closeVoice(vs *voiceState) {
	marks := marksOf(vs.last)
	if marks == nil {
		return
	}
	if vs.hairpin != nil && vs.hairpin.Stop {
		if marks.Hairpin == nil {
			marks.Hairpin = vs.hairpin
		} else {
			marks.Hairpin.Stop = true
		}
	}
	if vs.octave != nil && vs.octave.Stop {
		if marks.Octave == nil {
			marks.Octave = vs.octave
		} else {
			marks.Octave.Stop = true
		}
	}
	vs.hairpin, vs.octave = nil, nil
}

func (r *reader) voiceElement(doc *document, st *msStaff, vs *voiceState, e xml.StartElement) error {
	switch e.Name.Local {
	case "Chord":
		if err := r.chord(st, vs); err != nil {
			return err
		}
		st.lastEl = vs.last
		r.endOctave(st)
		return nil
	case "Rest":
		if err := r.rest(vs); err != nil {
			return err
		}
		st.lastEl = vs.last
		r.endOctave(st)
		return nil
	case "Clef":
		return r.clef(st, vs)
	case "KeySig":
		return r.keySig(vs.m)
	case "TimeSig":
		return r.timeSig(vs.m)
	case "Tempo":
		return r.tempo(doc, st, vs)
	case "Dynamic":
		return r.dynamic(vs)
	case "Fermata":
		vs.artics = append(vs.artics, model.Fermata)
		return r.dec.Skip()
	case "Harmony":
		return r.harmony(vs)
	case "RehearsalMark":
		// Its own <text>, not everything inside it: read whole, a rehearsal
		// mark came out as the file's internal id for it.
		s, err := r.innerText()
		if err != nil {
			return err
		}
		if s != "" && vs.m.Marker == "" {
			vs.m.Marker = s
		}
		return nil
	case "StaffText", "SystemText", "Expression":
		return r.staffText(vs)
	case "BarLine":
		return r.barLine(vs.m)
	case "Spanner":
		return r.spanner(st, vs, attr(e, "type"))
	case "HairPin", "Ottava", "Volta", "Pedal", "TextLine", "Trill",
		"LetRing", "PalmMute", "Vibrato", "endSpanner", "Slur":
		f, err := r.oldSpan(st, e)
		if err != nil {
			return err
		}
		if f != nil {
			f(vs)
		}
		return nil
	case "Tuplet":
		return r.tuplet(vs)
	case "endTuplet":
		r.endTuplet(vs)
		return r.dec.Skip()
	case "MeasureRepeat":
		return r.measureRepeat(vs)
	case "location", "eid", "lid", "linkedMain", "linked", "track", "voice",
		"Beam", "beam", "LayoutBreak", "Image", "Symbol", "irregular",
		"visible", "Ambitus", "FretDiagram", "InstrumentChange", "Marker",
		"Jump", "StaffTypeChange", "noOffset", "stretch", "Segment",
		"vspacerUp", "vspacerDown", "vspacerFixed", "measureNumberMode",
		"MeasureNumber", "SystemDivider", "Spacer":
		// Furniture: how the page is laid out, what is linked to what, and
		// the file's own bookkeeping. None of it is music, and reporting it
		// would bury what is.
		return r.dec.Skip()
	default:
		return r.unhandled("bar", e)
	}
}

// endOctave closes an octave line after the note that carries its end.
func (r *reader) endOctave(st *msStaff) {
	if st.octaveEnds {
		st.octave, st.octaveEnds = 0, false
	}
}

func (r *reader) clef(st *msStaff, vs *voiceState) error {
	name := ""
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "concertClefType", "transposingClefType", "subtype":
			s, err := r.text()
			if name == "" {
				name = s
			}
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	k, ok := clefKinds[name]
	if !ok {
		if name != "" {
			r.report("the %s clef is not one gscore draws", name)
		}
		return nil
	}
	// Only a clef in the first bar is the clef the staff starts in. One
	// written later is a change of clef from that point on.
	//
	// This used to take the first <Clef> it met wherever it was. A piano's
	// right hand that dips into the bass once, forty bars in, had that F
	// clef taken as the clef it begins in — and the whole part was drawn
	// an octave and a half out, from the first note, with no clef change
	// in sight to explain it. What the part actually begins in is in the
	// instrument definition, which had already been read correctly.
	if st.clefSet || len(st.measures) > 0 {
		// Where it happens, which is where MuseScore writes it: before the
		// note at that tick, which may be anywhere in the bar. Altes puts
		// four of them in two bars, so that the right hand can take a
		// phrase in the bass and come back without a ledger line in sight.
		clef := k
		st.clefWait = &clef
		if vs != nil {
			vs.pending = append(vs.pending, func(m *model.Marks) {
				m.Clef = &clef
				st.clefWait = nil
			})
		}
		return nil
	}
	st.clef, st.clefSet = k, true
	return nil
}

// keySig reads a key signature, written as the key that sounds and — on a
// staff that transposes — the key that is printed.
//
// Both are needed and they are different numbers. A score in concert D has
// <concertKey>2</concertKey> on every staff, and the alto saxophone's part
// carries <actualKey>5</actualKey> beside it because that player reads B
// major. gscore stores what is printed, so the second is the one to take;
// reading only the first put every transposing part in the wrong key, which
// is wrong on the page and wrong under the fingers.
func (r *reader) keySig(m *model.Measure) error {
	concert, written := 0, 0
	haveConcert, haveWritten := false, false
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "concertKey":
			n, err := r.number()
			concert, haveConcert = n, true
			return err
		case "actualKey":
			n, err := r.number()
			written, haveWritten = n, true
			return err
		case "accidental":
			// MuseScore 2 and 3 wrote the printed signature here and said
			// nothing about the concert key.
			n, err := r.number()
			written, haveWritten = n, true
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	fifths, have := written, haveWritten
	if !have {
		fifths, have = concert, haveConcert
	}
	if !have {
		return nil
	}
	if fifths < -7 || fifths > 7 {
		r.report("a key signature of %d accidentals is out of range", fifths)
		return nil
	}
	k := model.KeySignature{Fifths: fifths, Mode: model.Major}
	m.Key = &k
	return nil
}

func (r *reader) timeSig(m *model.Measure) error {
	n, d := 0, 0
	sym := ""
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "sigN":
			v, err := r.number()
			n = v
			return err
		case "sigD":
			v, err := r.number()
			d = v
			return err
		case "subtype":
			s, err := r.text()
			sym = s
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil || n <= 0 || d <= 0 {
		return err
	}
	t := model.TimeSignature{Beats: n, BeatValue: d}
	switch {
	case sym == "common" || (sym == "1" && n == 4 && d == 4):
		t.Symbol = model.TimeCommon
	case sym == "cut" || (sym == "2" && n == 2 && d == 2):
		t.Symbol = model.TimeCut
	}
	m.Time = &t
	return nil
}

func (r *reader) tempo(doc *document, st *msStaff, vs *voiceState) error {
	var qps float64
	words := ""
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "tempo":
			s, err := r.text()
			if err != nil {
				return err
			}
			qps, _ = strconv.ParseFloat(s, 64)
			return nil
		case "text":
			s, err := r.text()
			words = s
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil || qps <= 0 {
		return err
	}
	// MuseScore keeps a tempo as quarter notes per second, which is the
	// metronome mark over sixty.
	t := model.Tempo{
		BPM:  int(qps*60 + 0.5),
		Unit: model.Duration{Num: 1, Den: 4},
		Text: tempoWords(words),
	}

	// The one written over the first bar is the speed the piece starts at,
	// which is the score's own; anywhere else is a change held from that bar
	// on. The bar being read is the one about to be appended, so its index
	// is how many this staff has so far.
	if len(st.measures) == 0 {
		if !doc.tempoSet {
			doc.tempo, doc.tempoSet = t, true
		}
		return nil
	}
	vs.m.Tempo = &t
	return nil
}

// tempoWords keeps the words of a tempo marking and drops the metronome mark
// written beside them, which gscore draws from the numbers instead.
func tempoWords(s string) string {
	if i := strings.IndexByte(s, '='); i >= 0 {
		s = s[:i]
	}
	s = dropMusicSymbols(s)
	s = strings.Trim(strings.TrimSpace(s), "()[]")
	// What is left of "♩ = 150" once the note symbol is gone is nothing.
	if strings.Trim(s, " .,-") == "" {
		return ""
	}
	return strings.TrimSpace(s)
}

// dropMusicSymbols removes note symbols from a line of words.
//
// MuseScore writes a metronome mark as text, and the note in it is a real
// character: "𝅘𝅥 = 72" begins with U+1D15F, from Unicode's Musical Symbols
// block. gscore draws its own metronome mark from the numbers, in the music
// font, so the character is not wanted — and worse than not wanted, because
// the words beside a tempo are set in a text font, and no text font has a
// glyph for it. It came out as the empty box that means a font was asked for
// a character it does not have.
//
// Two ranges go: Unicode's musical symbols, and the private-use area where
// SMuFL lives, which is where a file that embeds Bravura's own codepoints
// puts them.
func dropMusicSymbols(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0x1D100 && r <= 0x1D1FF: // Musical Symbols
			return -1
		case r >= 0xE000 && r <= 0xF8FF: // private use: SMuFL
			return -1
		}
		return r
	}, s)
}

func (r *reader) dynamic(vs *voiceState) error {
	sub := ""
	err := r.children(func(c xml.StartElement) error {
		if c.Name.Local == "subtype" {
			s, err := r.text()
			sub = s
			return err
		}
		return r.dec.Skip()
	})
	if err != nil {
		return err
	}
	d, ok := dynamicFor(sub)
	if !ok {
		if sub != "" && sub != "other-dynamics" {
			r.report("the dynamic %q is not one gscore knows", sub)
		}
		return nil
	}
	vs.dynamic = d
	return nil
}

// innerText reads the <text> of an element that also carries bookkeeping —
// an id, a style, an offset — whose characters are not words.
func (r *reader) innerText() (string, error) {
	body := ""
	err := r.children(func(c xml.StartElement) error {
		if c.Name.Local == "text" {
			s, err := r.text()
			if body == "" {
				body = s
			}
			return err
		}
		return r.dec.Skip()
	})
	return body, err
}

func (r *reader) staffText(vs *voiceState) error {
	body, err := r.innerText()
	if err != nil {
		return err
	}
	if body != "" {
		vs.texts = append(vs.texts, model.TextMark{Text: body})
	}
	return nil
}

// harmony reads a chord symbol. Its root is a tonal pitch class rather than a
// letter, so it is spelled through the same table the notes use.
func (r *reader) harmony(vs *voiceState) error {
	root, bass := 0, 0
	haveRoot, haveBass := false, false
	name, text := "", ""
	var fields func(c xml.StartElement) error
	fields = func(c xml.StartElement) error {
		switch c.Name.Local {
		case "harmonyInfo":
			// MuseScore 4.4 put the chord's own fields a level deeper, in
			// a wrapper of their own. Read as furniture and skipped, every
			// chord symbol in a file written by it came back empty — a
			// lead sheet with its chords gone and nothing to say so.
			return r.children(fields)
		case "root":
			n, err := r.number()
			root, haveRoot = n, true
			return err
		case "base", "bass":
			n, err := r.number()
			bass, haveBass = n, true
			return err
		case "name":
			s, err := r.text()
			name = s
			return err
		case "text":
			s, err := r.text()
			text = s
			return err
		default:
			return r.dec.Skip()
		}
	}
	if err := r.children(fields); err != nil {
		return err
	}
	sym := &model.ChordSymbol{Quality: name}
	if haveRoot {
		sym.Root = tpcName(root)
	}
	if haveBass {
		sym.Bass = tpcName(bass)
	}
	if sym.Root == "" {
		if text == "" {
			return nil
		}
		sym.Quality, sym.Text = "", text
	}
	vs.chordSym = sym
	return nil
}

// tpcName spells a tonal pitch class as the letter and accidental a chord
// symbol is written with.
func tpcName(tpc int) string {
	p, ok := tpcPitch(tpc, 60)
	if !ok {
		return ""
	}
	s := p.Step.String()
	switch {
	case p.Alter > 0:
		s += strings.Repeat("#", p.Alter)
	case p.Alter < 0:
		s += strings.Repeat("b", -p.Alter)
	}
	return s
}

func (r *reader) barLine(m *model.Measure) error {
	sub := ""
	err := r.children(func(c xml.StartElement) error {
		if c.Name.Local == "subtype" {
			s, err := r.text()
			sub = s
			return err
		}
		return r.dec.Skip()
	})
	if err != nil || sub == "" {
		return err
	}
	// A repeat is already written on the measure and outranks the line's own
	// style, which MuseScore leaves at whatever it was.
	switch m.RightBarline {
	case model.BarRepeatEnd, model.BarRepeatStart, model.BarRepeatBoth:
		return nil
	}
	style, ok := barlineStyles[sub]
	if !ok {
		r.report("the %q barline is drawn as a plain one", sub)
		return nil
	}
	m.RightBarline = style
	return nil
}

func (r *reader) measureRepeat(vs *voiceState) error {
	n := 1
	err := r.children(func(c xml.StartElement) error {
		if c.Name.Local == "subtype" {
			v, err := r.number()
			if v > 0 {
				n = v
			}
			return err
		}
		return r.dec.Skip()
	})
	if err != nil {
		return err
	}
	if n > 2 {
		n = 2
	}
	vs.m.Repeat = n
	return nil
}

/*
 * Spanners
 *
 * Everything that reaches from one place to another — a slur, a tie, a
 * hairpin, an octave line, a volta — is written at both ends: the opening
 * carries <next> and the closing carries <prev>. That saves this reader from
 * working out where the other end landed, which MuseScore states as a number
 * of bars and a fraction of a bar away.
 */

func (r *reader) spanner(st *msStaff, vs *voiceState, kind string) error {
	start, stop := false, false
	sub, label := "", ""
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "next":
			start = true
			return r.dec.Skip()
		case "prev":
			stop = true
			return r.dec.Skip()
		case "Slur", "Tie", "HairPin", "Ottava", "Volta", "Pedal", "TextLine",
			"Trill", "Glissando", "LetRing", "PalmMute", "Vibrato":
			start = true
			return r.children(func(g xml.StartElement) error {
				switch g.Name.Local {
				case "subtype":
					s, err := r.text()
					sub = s
					return err
				case "beginText", "endings":
					s, err := r.text()
					if label == "" {
						label = s
					}
					return err
				default:
					return r.dec.Skip()
				}
			})
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}

	switch kind {
	case "Slur":
		if start {
			vs.markNext(func(m *model.Marks) { m.SlurStart = true })
		}
		if stop {
			vs.markNext(func(m *model.Marks) { m.SlurStop = true })
		}
	case "HairPin":
		vs.hairpin = &model.Hairpin{Kind: hairpinKind(sub), Start: start, Stop: stop}
	case "Ottava":
		amount, ok := octaveAmounts[sub]
		if !ok {
			amount = 1
		}
		vs.octave = &model.OctaveShift{Amount: amount, Start: start, Stop: stop}
		if start {
			st.octave = amount
		}
		if stop {
			st.octaveEnds = true
		}
	case "Trill":
		// A trill is an ornament that can last: "tr" at the note it begins
		// on and a wavy line to where it stops.
		vs.markNext(func(m *model.Marks) {
			if m.Trill == nil {
				m.Trill = &model.Trill{}
			}
			m.Trill.Start = m.Trill.Start || start
			m.Trill.Stop = m.Trill.Stop || stop
		})
	case "Volta":
		volta(vs.m, label, start, stop)
	case "Tie":
		// A tie belongs to a notehead and is read there; one out here is a
		// stray and means nothing on its own.
	default:
		r.report("a %s spanner is not read", kind)
	}
	return nil
}

// msSpan is a long mark opened in a MuseScore 1 or 2 file and waiting for
// the <endSpanner> that closes it.
type msSpan struct {
	kind  string
	sub   string
	label string
	at    model.Element // the last note read when it opened
}

// oldSpan reads a long mark the way MuseScore 1 and 2 wrote one.
//
// Those versions open the mark in the bar where it starts, name it with an
// id, and close it with an <endSpanner> of the same id at the tick where it
// stops — often a bar or more later. MuseScore 3 replaced the pair with a
// single <Spanner> written at both ends, and that was the only form this
// reader knew: the Moonlight sonata came in with all ninety-eight of its
// hairpins, both of its voltas and every one of its trills missing, and the
// file said so in four lines of warning that named no bar.
//
// What it hands back puts the mark on a voice, which is a step the caller
// has to take later: the mark is written at a position, and which voice
// that position belongs to is only known once a note arrives there. It
// returns nil, having read nothing, for an element that is not one of these
// marks.
func (r *reader) oldSpan(st *msStaff, e xml.StartElement) (func(*voiceState), error) {
	switch e.Name.Local {
	case "Slur":
		// MuseScore 1 and 2 write the slur's shape here and its two ends
		// on the chords it joins, where they are read. There is no music
		// in this one.
		return func(*voiceState) {}, r.dec.Skip()
	case "endSpanner":
		id := attr(e, "id")
		if err := r.dec.Skip(); err != nil {
			return nil, err
		}
		return func(vs *voiceState) { r.closeSpan(st, vs, id) }, nil
	case "HairPin", "Ottava", "Volta", "Pedal", "TextLine", "Trill",
		"LetRing", "PalmMute", "Vibrato":
	default:
		return nil, nil
	}

	sp := &msSpan{kind: e.Name.Local, at: st.lastEl}
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "subtype":
			s, err := r.text()
			sp.sub = s
			return err
		case "endings":
			s, err := r.text()
			if sp.label == "" {
				sp.label = s
			}
			return err
		case "beginText":
			// <beginText><text>1.</text></beginText>
			s, err := r.innerText()
			if sp.label == "" {
				sp.label = s
			}
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return nil, err
	}
	if id := attr(e, "id"); id != "" {
		if st.spans == nil {
			st.spans = map[string]*msSpan{}
		}
		st.spans[id] = sp
	}
	return func(vs *voiceState) { r.openSpan(st, vs, sp) }, nil
}

// openSpan hangs the mark's opening on the note that is coming.
func (r *reader) openSpan(st *msStaff, vs *voiceState, sp *msSpan) {
	switch sp.kind {
	case "HairPin":
		vs.hairpin = &model.Hairpin{Kind: hairpinKind(sp.sub), Start: true}
	case "Ottava":
		amount, ok := octaveAmounts[sp.sub]
		if !ok {
			amount = 1
		}
		vs.octave = &model.OctaveShift{Amount: amount, Start: true}
		st.octave = amount
	case "Trill":
		vs.markNext(func(m *model.Marks) {
			if m.Trill == nil {
				m.Trill = &model.Trill{}
			}
			m.Trill.Start = true
		})
	case "Volta":
		volta(vs.m, sp.label, true, false)
	default:
		r.report("a %s spanner is not read", sp.kind)
	}
}

// closeSpan ends the mark an <endSpanner> names.
func (r *reader) closeSpan(st *msStaff, vs *voiceState, id string) {
	sp := st.spans[id]
	if sp == nil {
		return
	}
	delete(st.spans, id)
	if sp.kind == "Volta" {
		// A volta's end is written at the tick where the bracket stops,
		// which is the start of the bar after the one it covers: the
		// bracket is over bar 65 and the <endSpanner> is the first thing
		// in bar 66. Anywhere else in a bar and it is that bar's own.
		m := vs.m
		if vs.last == nil && len(st.measures) > 0 {
			m = st.measures[len(st.measures)-1]
		}
		volta(m, sp.label, false, true)
		return
	}
	marks := marksOf(st.lastEl)
	if marks == nil || st.lastEl == sp.at {
		// Nothing was written between the two ends, so the note the
		// opening is still waiting for carries the closing as well.
		switch sp.kind {
		case "HairPin":
			if vs.hairpin != nil {
				vs.hairpin.Stop = true
			}
		case "Ottava":
			if vs.octave != nil {
				vs.octave.Stop = true
			}
		case "Trill":
			vs.markNext(func(m *model.Marks) {
				if m.Trill == nil {
					m.Trill = &model.Trill{}
				}
				m.Trill.Stop = true
			})
		}
		return
	}
	switch sp.kind {
	case "HairPin":
		if marks.Hairpin == nil {
			marks.Hairpin = &model.Hairpin{Kind: hairpinKind(sp.sub)}
		}
		marks.Hairpin.Stop = true
	case "Ottava":
		if marks.Octave == nil {
			marks.Octave = &model.OctaveShift{Amount: st.octave}
		}
		marks.Octave.Stop = true
		st.octaveEnds = true
	case "Trill":
		if marks.Trill == nil {
			marks.Trill = &model.Trill{}
		}
		marks.Trill.Stop = true
	}
}

// hairpinKind reads which way the wedge opens. MuseScore numbers them
// crescendo, diminuendo, and then the same two again written as text.
func hairpinKind(sub string) model.HairpinKind {
	if sub == "1" || sub == "3" {
		return model.Diminuendo
	}
	return model.Crescendo
}

// markNext remembers a mark for the next note in the voice.
func (vs *voiceState) markNext(set func(*model.Marks)) {
	vs.pending = append(vs.pending, set)
}

func volta(m *model.Measure, label string, start, stop bool) {
	if m.Ending == nil {
		m.Ending = &model.Ending{}
	}
	if len(m.Ending.Numbers) == 0 {
		m.Ending.Numbers = voltaNumbers(label)
	}
	m.Ending.Start = m.Ending.Start || start
	m.Ending.Stop = m.Ending.Stop || stop
}

// voltaNumbers reads the times through a bracket covers out of the way it is
// labelled: "1.", "1,2" and "1. 2." all mean the same thing.
func voltaNumbers(label string) []int {
	var out []int
	cur, has := 0, false
	for i := 0; i <= len(label); i++ {
		if i < len(label) && label[i] >= '0' && label[i] <= '9' {
			cur, has = cur*10+int(label[i]-'0'), true
			continue
		}
		if has {
			out = append(out, cur)
			cur, has = 0, false
		}
	}
	return out
}

/*
 * Tuplets
 */

// noTupletNumber is MuseScore's "do not write the figure over this group".
//
// The field counts up through what may be written there — the number, then
// the ratio — and ends at nothing at all.
const noTupletNumber = 2

func (r *reader) tuplet(vs *voiceState) error {
	actual, normal, number := 0, 0, 0
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "actualNotes":
			n, err := r.number()
			actual = n
			return err
		case "normalNotes":
			n, err := r.number()
			normal = n
			return err
		case "numberType":
			// Whether the group is written as a tuplet at all, or is only
			// counted as one. A measured tremolo is six sixteenths in the
			// time of four and is written with strokes through the stem,
			// so the 6 and its bracket are turned off — and drawing them
			// anyway put a bracketed 6 over every tremolo in Beethoven's
			// ninth.
			n, err := r.number()
			number = n
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil || actual <= 0 || normal <= 0 {
		return err
	}
	vs.tuplet = &model.Tuplet{
		Actual: actual, Normal: normal,
		Hidden: number == noTupletNumber,
	}
	vs.opens = true
	vs.inTuplet = nil
	return nil
}

func (r *reader) endTuplet(vs *voiceState) {
	if n := len(vs.inTuplet); n > 0 {
		if marks := marksOf(vs.inTuplet[n-1]); marks != nil && marks.Tuplet != nil {
			marks.Tuplet.Stop = true
		}
	}
	vs.tuplet, vs.opens, vs.inTuplet = nil, false, nil
}

/*
 * Notes and rests
 */

func (r *reader) rest(vs *voiceState) error {
	kind, dots := "", 0
	var whole model.Duration
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "durationType":
			s, err := r.text()
			kind = s
			return err
		case "dots":
			n, err := r.number()
			dots = n
			return err
		case "duration":
			// MuseScore 4 writes the fraction as text and MuseScore 2
			// wrote it as a pair of attributes. Read only as text, every
			// whole-bar rest written by the older one came out a whole
			// note long: the bar of a quarter that carries the Trio's
			// upbeat in the Moonlight sonata was read as four, and from
			// there the staff was a bar and a half ahead of the file it
			// was being read from — so every <tick> after it landed in the
			// wrong place and the second voice of a bar was laid end to
			// end after the first instead of under it.
			if z, n := attr(c, "z"), attr(c, "n"); z != "" && n != "" {
				whole = parseFraction(z + "/" + n)
				return r.dec.Skip()
			}
			s, err := r.text()
			if err != nil {
				return err
			}
			whole = parseFraction(s)
			return nil
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	dur, ok := r.duration(kind, dots, whole, vs)
	if !ok {
		return nil
	}
	rest := &model.Rest{Dur: dur}
	vs.attach(&rest.Marks)
	vs.add(rest)
	return nil
}

// chordData is one <Chord> as it is read, before it is decided whether it is
// a note, a chord, or a group of grace notes leaning on the next one.
type chordData struct {
	kind    string
	dots    int
	whole   model.Duration
	notes   []*model.Note
	marks   model.Marks
	stem    model.Stem
	beam    model.Beam
	grace   bool
	slashed bool
	tremolo *model.Tremolo
}

func (r *reader) chord(st *msStaff, vs *voiceState) error {
	ch := &chordData{}
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "durationType":
			s, err := r.text()
			ch.kind = s
			return err
		case "dots":
			n, err := r.number()
			ch.dots = n
			return err
		case "duration":
			s, err := r.text()
			if err != nil {
				return err
			}
			ch.whole = parseFraction(s)
			return nil
		case "Note":
			return r.noteElement(st, ch)
		case "Ornament":
			// MuseScore 4 writes the ornaments in an element of their own
			// rather than among the articulations. They are the same marks
			// named the same way, so they are read the same way.
			return r.articulation(ch)
		case "Articulation":
			return r.articulation(ch)
		case "Lyrics":
			return r.lyrics(ch)
		case "Spanner":
			return r.chordSpanner(ch, attr(c, "type"))
		case "Slur", "Tie":
			// How MuseScore 1 and 2 wrote a slur: an empty element on the
			// chord at each end, naming the slur by id, with a definition
			// of the same id standing in the bar. MuseScore 3 replaced it
			// with the <Spanner> above, which is the only form this reader
			// understood — so every slur in a file written before 2019 was
			// dropped here without a word. The ends are all that is needed:
			// which notes they fall on is what a slur is.
			kind := attr(c, "type")
			if err := r.dec.Skip(); err != nil {
				return err
			}
			if c.Name.Local == "Slur" {
				switch kind {
				case "start":
					ch.marks.SlurStart = true
				case "stop":
					ch.marks.SlurStop = true
				}
			}
			return nil
		case "StemDirection":
			s, err := r.text()
			switch s {
			case "up":
				ch.stem = model.StemUp
			case "down":
				ch.stem = model.StemDown
			}
			return err
		case "BeamMode":
			s, err := r.text()
			switch s {
			case "no", "none":
				ch.beam = model.BeamNone
			case "begin":
				ch.beam = model.BeamStart
			case "mid":
				ch.beam = model.BeamContinue
			case "end":
				ch.beam = model.BeamStop
			}
			return err
		case "acciaccatura":
			ch.grace, ch.slashed = true, true
			return r.dec.Skip()
		case "appoggiatura", "grace4", "grace8", "grace16", "grace32",
			"grace64", "grace4after", "grace8after", "grace16after",
			"grace32after":
			ch.grace = true
			return r.dec.Skip()
		case "Tremolo", "TremoloSingleChord", "TremoloTwoChord":
			return r.tremolo(ch, c)
		case "Arpeggio", "ChordLine":
			return r.unhandled("chord", c)
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	if len(ch.notes) == 0 {
		return nil
	}

	// A grace note takes no time out of the bar, so it waits for the note it
	// leans on rather than going into the voice on its own account.
	if ch.grace {
		if vs.grace == nil {
			vs.grace = &model.GraceGroup{}
		}
		vs.grace.Slashed = vs.grace.Slashed || ch.slashed
		written := r.writtenValue(ch.kind, ch.dots)
		first := len(vs.grace.Notes)
		for _, n := range ch.notes {
			n.Dur = written
			vs.grace.Notes = append(vs.grace.Notes, n)
		}
		// A slur that begins on a grace note — an appoggiatura leaning into
		// the note it decorates — is the commonest slur in this music, and
		// the marks on a grace chord were going out with the chord itself.
		// They move to the grace note, which is where the curve starts.
		if ch.marks.SlurStart || ch.marks.SlurStop {
			n := vs.grace.Notes[first]
			n.Marks.SlurStart = n.Marks.SlurStart || ch.marks.SlurStart
			n.Marks.SlurStop = n.Marks.SlurStop || ch.marks.SlurStop
		}
		return nil
	}

	dur, ok := r.duration(ch.kind, ch.dots, ch.whole, vs)
	if !ok {
		return nil
	}

	var el model.Element
	var marks *model.Marks
	if len(ch.notes) == 1 {
		n := ch.notes[0]
		n.Dur = dur
		n.Marks = mergeMarks(n.Marks, ch.marks)
		el, marks = n, &n.Marks
	} else {
		c := &model.Chord{Notes: ch.notes, Dur: dur, Marks: ch.marks}
		for _, n := range c.Notes {
			n.Dur = dur
		}
		el, marks = c, &c.Marks
	}
	marks.Stem = ch.stem
	marks.Beam = ch.beam
	if ch.tremolo != nil {
		marks.Tremolo = ch.tremolo
	}
	vs.attach(marks)
	vs.add(el)
	// The far end of a two-note tremolo.
	//
	// The beams are drawn between a pair of chords, and MuseScore writes
	// the mark on the first of them and nothing at all on the second. The
	// second is the next chord of this voice, which is known here and
	// nowhere else.
	if t := marks.Tremolo; t != nil && t.Start {
		strokes, buzz := t.Strokes, t.Buzz
		vs.markNext(func(m *model.Marks) {
			m.Tremolo = &model.Tremolo{Strokes: strokes, Buzz: buzz, Stop: true}
		})
	}
	return nil
}

// tremolo reads the slashes on a chord.
//
// MuseScore names them by the note they stand for rather than by how many
// slashes that is — r8 is a repeated eighth and so one slash, r32 is three
// — which is the same fact counted from the other end.
//
// c8, c16 and so on are the two-note kind: beams drawn between a pair of
// chords, meaning alternate between them for as long as the two of them
// last. They were reported as unread, because the mark belongs to the pair
// rather than to either chord — so two half notes of a measured tremolo
// came out as two half notes, and a pianist reading them played two half
// notes. The mark is on the first of the pair and the second is marked
// where the chord is added, which is the only place that knows what comes
// next.
func (r *reader) tremolo(ch *chordData, el xml.StartElement) error {
	sub := ""
	err := r.children(func(c xml.StartElement) error {
		if c.Name.Local == "subtype" {
			s, err := r.text()
			sub = s
			return err
		}
		return r.dec.Skip()
	})
	if err != nil {
		return err
	}
	switch sub {
	case "r8":
		ch.tremolo = &model.Tremolo{Strokes: 1}
	case "r16":
		ch.tremolo = &model.Tremolo{Strokes: 2}
	case "r32":
		ch.tremolo = &model.Tremolo{Strokes: 3}
	case "r64":
		ch.tremolo = &model.Tremolo{Strokes: 4}
	case "c8":
		ch.tremolo = &model.Tremolo{Strokes: 1, Start: true}
	case "c16":
		ch.tremolo = &model.Tremolo{Strokes: 2, Start: true}
	case "c32":
		ch.tremolo = &model.Tremolo{Strokes: 3, Start: true}
	case "c64":
		ch.tremolo = &model.Tremolo{Strokes: 4, Start: true}
	case "buzz", "buzzRoll":
		ch.tremolo = &model.Tremolo{Buzz: true}
	case "":
		// No subtype at all. On a chord of its own that is one slash; on a
		// two-chord tremolo, which MuseScore 4 names in the element rather
		// than in a subtype, it is the pair with one beam.
		ch.tremolo = &model.Tremolo{Strokes: 1, Start: el.Name.Local == "TremoloTwoChord"}
	default:
		r.report("a %s of %q is not read", el.Name.Local, sub)
	}
	return nil
}

func (r *reader) noteElement(st *msStaff, ch *chordData) error {
	pitch, tpc, tpc2 := 0, 0, 0
	havePitch, haveTpc, haveTpc2 := false, false, false
	n := &model.Note{}
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "pitch":
			v, err := r.number()
			pitch, havePitch = v, true
			return err
		case "tpc":
			v, err := r.number()
			tpc, haveTpc = v, true
			return err
		case "tpc2":
			v, err := r.number()
			tpc2, haveTpc2 = v, true
			return err
		case "string":
			// Counted from the top line of the tab down, and from zero;
			// gscore counts from one so that nothing means nothing.
			v, err := r.number()
			n.String = v + 1
			return err
		case "fret":
			v, err := r.number()
			n.Fret = v
			return err
		case "Accidental":
			return r.accidental(n)
		case "head", "headGroup":
			s, err := r.text()
			if err != nil {
				return err
			}
			if k, ok := headKinds[s]; ok {
				n.Head = k
			}
			return nil
		case "Spanner":
			if attr(c, "type") == "Tie" {
				return r.tieSpanner(n)
			}
			return r.dec.Skip()
		case "Tie":
			n.TieStart = true
			return r.dec.Skip()
		case "Fingering":
			s, err := r.innerText()
			if err != nil {
				return err
			}
			if s != "" {
				n.Texts = append(n.Texts, model.TextMark{Text: s})
			}
			return nil
		default:
			return r.dec.Skip()
		}
	})
	if err != nil {
		return err
	}
	if !havePitch {
		return nil
	}

	// <pitch> is what sounds. gscore stores what is written, which on a
	// transposing staff is a different note: an alto saxophone's written C
	// sounds the E flat a major sixth below, and printing what sounds would
	// put every note of the part in the wrong place.
	written := pitch - st.transChr
	spelling, haveSpelling := tpc, haveTpc
	if haveTpc2 {
		// tpc2 is how the note is spelled on the page, tpc how it sounds.
		spelling, haveSpelling = tpc2, true
	}
	if p, ok := tpcPitch(spelling, written); ok && haveSpelling {
		n.Pitch = p
	} else {
		n.Pitch = midiPitch(written)
	}
	// An octave line shifts what is printed away from what sounds, and
	// MuseScore records what sounds.
	n.Pitch.Octave -= st.octave

	ch.notes = append(ch.notes, n)
	return nil
}

func (r *reader) accidental(n *model.Note) error {
	sub := ""
	err := r.children(func(c xml.StartElement) error {
		if c.Name.Local == "subtype" {
			s, err := r.text()
			sub = s
			return err
		}
		return r.dec.Skip()
	})
	if err != nil {
		return err
	}
	if a, ok := accidentals[sub]; ok {
		n.Accidental = a
	} else if sub != "" {
		r.report("the %q accidental is not one gscore draws", sub)
	}
	return nil
}

func (r *reader) tieSpanner(n *model.Note) error {
	start, stop := false, false
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "next", "Tie":
			start = true
		case "prev":
			stop = true
		}
		return r.dec.Skip()
	})
	if err != nil {
		return err
	}
	n.TieStart = n.TieStart || start
	n.TieStop = n.TieStop || stop
	return nil
}

func (r *reader) articulation(ch *chordData) error {
	sub := ""
	err := r.children(func(c xml.StartElement) error {
		if c.Name.Local == "subtype" {
			s, err := r.text()
			sub = s
			return err
		}
		return r.dec.Skip()
	})
	if err != nil {
		return err
	}
	if a, ok := articulations[sub]; ok {
		ch.marks.AddArticulation(a)
		return nil
	}
	if a, ok := ornaments[sub]; ok {
		ch.marks.AddArticulation(a)
		return nil
	}
	// An ornament written on the note rather than as a spanner is a trill on
	// that note alone: both ends of it are the same note.
	if sub == "ornamentTrill" || sub == "trill" {
		if ch.marks.Trill == nil {
			ch.marks.Trill = &model.Trill{Start: true, Stop: true}
		}
		return nil
	}
	if sub != "" {
		r.report("the %q mark is not one gscore draws", sub)
	}
	return nil
}

// ornaments are the marks that say to play notes which are not written.
//
// MuseScore names them by their SMuFL glyph, which is what they are, so the
// table is short and does not have to be guessed at.
var ornaments = map[string]model.Articulation{
	"ornamentMordent":      model.Mordent,
	"ornamentShortTrill":   model.ShortTrill,
	"ornamentPrallMordent": model.ShortTrill,
	"ornamentTurn":         model.Turn,
	"ornamentTurnInverted": model.TurnInverted,
}

func (r *reader) lyrics(ch *chordData) error {
	verse, syll, text := 1, "", ""
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "no":
			n, err := r.number()
			verse = n + 1 // MuseScore counts verses from zero
			return err
		case "syllabic":
			s, err := r.text()
			syll = s
			return err
		case "text":
			s, err := r.text()
			text = s
			return err
		default:
			return r.dec.Skip()
		}
	})
	if err != nil || text == "" {
		return err
	}
	l := model.Lyric{Text: text, Verse: verse}
	switch syll {
	case "begin":
		l.Syllabic = model.SyllableBegin
	case "middle":
		l.Syllabic = model.SyllableMiddle
	case "end":
		l.Syllabic = model.SyllableEnd
	}
	ch.marks.Lyrics = append(ch.marks.Lyrics, l)
	return nil
}

func (r *reader) chordSpanner(ch *chordData, kind string) error {
	start, stop := false, false
	err := r.children(func(c xml.StartElement) error {
		switch c.Name.Local {
		case "next", "Slur":
			start = true
		case "prev":
			stop = true
		}
		return r.dec.Skip()
	})
	if err != nil || kind != "Slur" {
		return err
	}
	ch.marks.SlurStart = ch.marks.SlurStart || start
	ch.marks.SlurStop = ch.marks.SlurStop || stop
	return nil
}

/*
 * Durations
 */

// writtenValue is what the notehead shows, before a tuplet is taken into
// account.
func (r *reader) writtenValue(kind string, dots int) model.Duration {
	v, ok := durationValues[kind]
	if !ok {
		switch kind {
		case "256th", "512th", "1024th":
			r.report("a %s note is shorter than gscore writes; it is a 128th here", kind)
			v = model.HundredTwentyEighth
		default:
			return model.Duration{}
		}
	}
	base := v.Duration()
	d := base
	for i := 0; i < dots && i < 4; i++ {
		d = d.Add(base.Scale(1, 1<<(i+1)))
	}
	return d
}

// duration turns a written value into the time it takes in the bar, which
// inside a tuplet is less than the notehead says.
func (r *reader) duration(kind string, dots int, whole model.Duration,
	vs *voiceState) (model.Duration, bool) {

	if kind == "measure" {
		if !whole.IsZero() && whole.Den != 0 {
			return whole, true
		}
		// A whole-bar rest with no length given fills whatever the bar is.
		return model.Duration{Num: 1, Den: 1}, true
	}
	d := r.writtenValue(kind, dots)
	if d.IsZero() {
		r.report("a note written as %q is not a value gscore has", kind)
		return model.Duration{}, false
	}
	if vs.tuplet != nil {
		d = d.Scale(vs.tuplet.Normal, vs.tuplet.Actual)
	}
	return d, true
}

func parseFraction(s string) model.Duration {
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return model.Duration{}
	}
	n, err1 := strconv.Atoi(strings.TrimSpace(s[:i]))
	d, err2 := strconv.Atoi(strings.TrimSpace(s[i+1:]))
	if err1 != nil || err2 != nil || d == 0 {
		return model.Duration{}
	}
	return model.Duration{Num: n, Den: d}
}

/*
 * Putting an element into a voice
 */

// attach puts the marks that were written before this element onto it.
func (vs *voiceState) attach(m *model.Marks) {
	for _, set := range vs.pending {
		set(m)
	}
	vs.pending = nil

	if vs.dynamic != model.DynamicNone {
		m.Dynamic = vs.dynamic
		vs.dynamic = model.DynamicNone
	}
	for _, a := range vs.artics {
		m.AddArticulation(a)
	}
	vs.artics = nil
	if vs.hairpin != nil {
		m.Hairpin = vs.hairpin
		vs.hairpin = nil
	}
	if vs.octave != nil {
		m.Octave = vs.octave
		vs.octave = nil
	}
	if vs.chordSym != nil {
		m.Chord = vs.chordSym
		vs.chordSym = nil
	}
	if len(vs.texts) > 0 {
		m.Texts = append(m.Texts, vs.texts...)
		vs.texts = nil
	}
	if vs.grace != nil {
		m.Grace = vs.grace
		vs.grace = nil
	}
	if vs.tuplet != nil {
		t := *vs.tuplet
		t.Start = vs.opens
		vs.opens = false
		m.Tuplet = &t
	}
}

// add puts the element into the voice.
func (vs *voiceState) add(el model.Element) {
	vs.v.Elements = append(vs.v.Elements, el)
	vs.last = el
	if vs.tuplet != nil {
		vs.inTuplet = append(vs.inTuplet, el)
	}
}

// marksOf reaches the marks of whichever kind of element this is.
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

// mergeMarks folds a chord's marks into the single note that was its only
// member, keeping what the note itself already carries.
func mergeMarks(note, chord model.Marks) model.Marks {
	out := chord
	out.TieStart = out.TieStart || note.TieStart
	out.TieStop = out.TieStop || note.TieStop
	out.Texts = append(out.Texts, note.Texts...)
	return out
}

/*
 * From the document to a score
 */

// placeholders are what MuseScore leaves in the metadata of a score whose
// title was only ever typed onto the page. Carrying them through would print
// "Untitled score" over a piece that has a name.
var placeholders = map[string]bool{
	"Untitled score": true, "Subtitle": true, "Composer / arranger": true,
	"Composer": true, "Lyricist": true,
}

func (doc *document) assemble(r *reader) (*model.Score, error) {
	s := &model.Score{Tempo: doc.tempo}
	s.Identity.Title = firstOf(doc.title, doc.meta["workTitle"])
	s.Identity.Subtitle = firstOf(doc.subtitle, doc.meta["subtitle"])
	s.Identity.Composer = firstOf(doc.composer, doc.meta["composer"])
	s.Identity.Copyright = doc.meta["copyright"]

	// A MuseScore part can be printed on more than one staff — a piano is two
	// — while a gscore part is one staff. So each staff becomes a part, and
	// the ones that belong to the same instrument are grouped.
	used := map[string]bool{}
	for _, p := range doc.parts {
		first := len(s.Parts)
		for i, id := range p.staffIDs {
			st, ok := doc.staves[id]
			if !ok {
				continue
			}
			used[id] = true
			s.Parts = append(s.Parts, toPart(p, st, i))
		}
		if len(s.Parts)-first > 1 {
			s.Groups = append(s.Groups, model.StaffGroup{
				First: first, Last: len(s.Parts) - 1,
				Symbol:   bracketSymbol(p.bracket),
				Barlines: p.barSpan,
				Name:     p.name,
			})
		}
	}
	// A staff no part claimed is still music, and leaving it out would lose
	// it without a word.
	var orphans []string
	for _, id := range doc.order {
		if !used[id] {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	for _, id := range orphans {
		r.report("staff %s belongs to no part and is shown on its own", id)
		s.Parts = append(s.Parts, toPart(&msPart{}, doc.staves[id], 0))
	}

	if len(s.Parts) == 0 {
		return nil, errors.New("musescore: the file has no staves with music in them")
	}
	closeTheScore(s)
	return s, nil
}

// closeTheScore writes the barline MuseScore draws at the end and does not
// record.
//
// The last bar of a score ends with a final barline unless something else is
// written there, so MuseScore has nothing to save: the file carries a
// <BarLine> only where the barline is not what it would have been anyway.
// Read literally, every score came out ending on a plain barline, which is
// how a page says "turn over" rather than "that is the end".
//
// It goes on the longest part, which is where the piece actually ends; the
// score's own rule about repeats and final barlines then gives it to the
// staves that stop earlier, at the bar they stop being drawn at.
func closeTheScore(s *model.Score) {
	last := 0
	for _, p := range s.Parts {
		if n := len(p.Measures); n > last {
			last = n
		}
	}
	if last == 0 {
		return
	}
	for _, p := range s.Parts {
		if len(p.Measures) != last {
			continue
		}
		if m := p.Measures[last-1]; m.RightBarline == model.BarSingle {
			m.RightBarline = model.BarFinal
		}
	}
}

func toPart(p *msPart, st *msStaff, index int) *model.Part {
	out := &model.Part{
		ID:       st.id,
		Clef:     st.clef,
		Measures: st.measures,
		Key:      model.KeySignature{Fifths: 0, Mode: model.Major},
		Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	// A staff the file calls tablature is written as tablature, if the
	// instrument said how it is tuned. Without a tuning there is nothing to
	// count frets from, and the part is better read on a staff of pitches
	// than as numbers that mean nothing.
	if index < len(p.staffGroups) && p.staffGroups[index] == "tablature" &&
		len(p.tuning) > 0 {
		// The file lists the strings lowest first; the tab is drawn from the
		// top line down.
		strings := make([]model.Pitch, len(p.tuning))
		for i, s := range p.tuning {
			strings[len(p.tuning)-1-i] = s
		}
		out.Tab = &model.Tablature{Strings: strings, Frets: p.frets}
	}
	// Only the first staff of an instrument carries its name. A piano is one
	// instrument on two staves, and writing "Piano" twice down the left of
	// the system says it is two.
	if index == 0 {
		out.Name = p.name
	}
	out.Sound = p.sound
	if p.hasProg {
		out.Instrument = model.Instrument(p.program)
	} else if prog, ok := model.SoundProgram(p.sound); ok {
		// A file that names the instrument and leaves the program out:
		// the name is enough to pick something to play it with.
		out.Instrument = prog
	}
	if p.transDia != 0 || p.transChr != 0 {
		out.Transpose = model.Transposition{Diatonic: p.transDia, Chromatic: p.transChr}
	}
	// The key and meter a staff opens in are written on its first bar, and
	// the part carries them so that the head of a system can be drawn before
	// any bar is. Left on the bar as well they would be announced twice.
	if len(st.measures) > 0 {
		if k := st.measures[0].Key; k != nil {
			out.Key = *k
			st.measures[0].Key = nil
		}
		if t := st.measures[0].Time; t != nil {
			out.Time = *t
			st.measures[0].Time = nil
		}
	}
	return out
}

func bracketSymbol(kind string) model.GroupSymbol {
	switch kind {
	case "1":
		return model.GroupBrace
	case "3":
		return model.GroupLine
	}
	return model.GroupBracket
}

func firstOf(vs ...string) string {
	for _, v := range vs {
		if s := strings.TrimSpace(v); s != "" && !placeholders[s] {
			return s
		}
	}
	return ""
}

/*
 * Small tables that belong with the reader rather than with the format
 */

var accidentals = map[string]model.Accidental{
	"accidentalSharp": model.Sharp, "sharp": model.Sharp,
	"accidentalFlat": model.Flat, "flat": model.Flat,
	"accidentalNatural": model.Natural, "natural": model.Natural,
	"accidentalDoubleSharp": model.DoubleSharp, "sharp sharp": model.DoubleSharp,
	"accidentalDoubleFlat": model.DoubleFlat, "flat flat": model.DoubleFlat,
}

var headKinds = map[string]model.NoteheadKind{
	"cross": model.HeadX, "x": model.HeadX,
	"circled": model.HeadCircleX, "cross-circled": model.HeadCircleX,
	"triangle": model.HeadTriangle, "triangle-down": model.HeadTriangle,
	"diamond": model.HeadDiamond,
	"slash":   model.HeadSlash,
}

// msTicksPerQuarter is how MuseScore divides a quarter note.
const msTicksPerQuarter = 480

// tickDuration turns a position in a bar into the silence before it.
// voiceTicks is how much of its bar a voice has filled.
func voiceTicks(v *model.Voice) int {
	total := model.Duration{Num: 0, Den: 1}
	for _, el := range v.Elements {
		total = total.Add(el.Duration())
	}
	return durationTicks(total)
}

func tickDuration(ticks int) (model.Duration, bool) {
	if ticks <= 0 {
		return model.Duration{}, false
	}
	// Ticks count quarter notes; a Duration counts whole ones.
	return model.Duration{
		Num: ticks, Den: msTicksPerQuarter * 4,
	}.Reduced(), true
}

// durationTicks is a length of music in MuseScore's own ticks.
func durationTicks(d model.Duration) int {
	if d.Den == 0 {
		return 0
	}
	return d.Num * msTicksPerQuarter * 4 / d.Den
}
