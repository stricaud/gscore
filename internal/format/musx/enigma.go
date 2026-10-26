/*
 * enigma.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * EnigmaXML, which is Finale's document model written out as XML.
 *
 * It is not a description of a score the way MusicXML is. It is a dump of the
 * program's own tables, and reading it means walking the same indirections
 * Finale walks:
 *
 *     others/instUsed cmper=0   the staves of the score, in score order
 *     others/staffSpec          what each staff is
 *     others/measSpec           what each bar is: its meter, its barline
 *     details/gfhold            staff × bar → the clef, and up to four frames
 *     others/frameSpec          a frame → the first and last entry in it
 *     entries/entry             the notes, in a doubly-linked list
 *
 * So the music of one bar of one staff is four lookups away from the bar, and
 * the notes themselves are a linked list that has to be walked rather than a
 * range that can be sliced: the documentation is explicit that entry numbers
 * carry no arithmetic meaning.
 *
 * What is read here is the music: staves, bars, meters, clefs, notes, chords
 * and rests, with their pitches and their lengths. What is not read is
 * everything Finale writes around the music — slurs, articulations, text,
 * lyrics, repeats, tuplets, page layout — and each of those is reported
 * rather than dropped quietly, so a reader can see what did not come through.
 *
 * The structure is documented, unofficially, at
 * github.com/Project-Attacca/enigmaxml-documentation.
 */

// Finale measures time in EDUs, of which a quarter note is 1024 — so a whole
// note is 4096, and a duration is that fraction of a whole note.
const eduWhole = 4096

type document struct {
	Others struct {
		Staves     []staffSpec        `xml:"staffSpec"`
		Bars       []measSpec         `xml:"measSpec"`
		Frames     []frameSpec        `xml:"frameSpec"`
		Used       []instUsed         `xml:"instUsed"`
		Blocks     []textBlock        `xml:"textBlock"`
		Artics     []articDef         `xml:"articDef"`
		Shapes     []smartShape       `xml:"smartShape"`
		Exprs      []textExprDef      `xml:"textExprDef"`
		ExprPlaces []measExprAssign   `xml:"measExprAssign"`
		Categories []markingsCategory `xml:"markingsCategory"`
	} `xml:"others"`
	Details struct {
		Holds  []gfhold      `xml:"gfhold"`
		Artics []articAssign `xml:"articAssign"`
	} `xml:"details"`
	Entries struct {
		Entries []entry `xml:"entry"`
	} `xml:"entries"`
	Texts struct {
		Blocks []blockText `xml:"blockText"`
	} `xml:"texts"`
}

// A staff's name is three lookups away from the staff: staffSpec names a
// textBlock, the textBlock names a line of raw text, and the raw text is
// Finale's own markup with the words somewhere inside it.
type textBlock struct {
	Cmper  int    `xml:"cmper,attr"`
	Part   string `xml:"part,attr"`
	TextID int    `xml:"textID"`
}

type blockText struct {
	Number int    `xml:"number,attr"`
	Text   string `xml:",chardata"`
}

// articDef is one kind of articulation mark, identified by the character it
// is drawn with in Finale's music font.
type articDef struct {
	Cmper    int    `xml:"cmper,attr"`
	Part     string `xml:"part,attr"`
	CharMain int    `xml:"charMain"`
}

// articAssign puts one of those on one entry.
type articAssign struct {
	Entry    int    `xml:"entnum,attr"`
	Part     string `xml:"part,attr"`
	ArticDef int    `xml:"articDef"`
}

// smartShape is a line drawn over the music: a slur, a hairpin, an ottava.
//
// Finale keeps them away from the notes, in a list of their own with each
// end naming where it lands. A slur is entry-based and names the entry; a
// hairpin is attached to a beat and names a position in the bar, in EDUs.
// Both name the staff and the measure.
type smartShape struct {
	Cmper      int       `xml:"cmper,attr"`
	Part       string    `xml:"part,attr"`
	ShapeType  string    `xml:"shapeType"`
	EntryBased *struct{} `xml:"entryBased"`
	Hidden     *struct{} `xml:"hidden"`
	Start      endPoint  `xml:"startTermSeg>endPt"`
	End        endPoint  `xml:"endTermSeg>endPt"`
}

// endPoint is where one end of a shape lands: inst is the staff, meas the
// measure, and then either the entry it hangs on or a position in the bar.
type endPoint struct {
	Inst  int `xml:"inst"`
	Meas  int `xml:"meas"`
	Edu   int `xml:"edu"`
	Entry int `xml:"entryNum"`
}

// textExprDef is one expression as the document defines it — the dynamic
// "ff" itself rather than any of the places it is written.
//
// The mark is drawn as a character of Finale's music font, which is of no
// use to anybody else: "ë" is fortississimo only if you have Maestro and
// know its layout. What the definition also carries is what the mark does,
// and that reads the same everywhere — playType "amplitude" with a velocity
// in value, which is the dynamic said in numbers.
type textExprDef struct {
	Cmper    int    `xml:"cmper,attr"`
	Part     string `xml:"part,attr"`
	Category int    `xml:"categoryID"`
	PlayType string `xml:"playType"`
	Value    int    `xml:"value"`
	Desc     string `xml:"descStr"`
}

// measExprAssign is one expression written at one place in the music: which
// bar, which staff, and how far into the bar.
type measExprAssign struct {
	Meas   int    `xml:"cmper,attr"`
	Part   string `xml:"part,attr"`
	Expr   int    `xml:"textExprID"`
	Staff  int    `xml:"staffAssign"`
	EduOff int    `xml:"horzEduOff"`
}

// markingsCategory says what a group of expressions is for, which is how a
// dynamic is told from a tempo mark or a rehearsal direction.
type markingsCategory struct {
	Cmper int    `xml:"cmper,attr"`
	Part  string `xml:"part,attr"`
	Type  string `xml:"categoryType"`
}

// Part and Shared mark the copies Finale keeps for each extracted part. The
// score's own copy carries neither, and taking the lot would give every bar
// several times over.
type staffSpec struct {
	Cmper       int    `xml:"cmper,attr"`
	Part        string `xml:"part,attr"`
	StaffLines  int    `xml:"staffLines"`
	DefaultClef int    `xml:"defaultClef"`
	FullName    int    `xml:"fullName"`
	AbbrvName   int    `xml:"abbrvName"`
	// How far the staff is written from where it sounds, as a diatonic
	// interval and a number of sharps added to the key signature — which is
	// how a player's part is described rather than how a tuner describes
	// it. A B flat clarinet is one step up and two sharps.
	Transposition struct {
		Interval int `xml:"keysig>interval"`
		Adjust   int `xml:"keysig>adjust"`
	} `xml:"transposition"`
}

type measSpec struct {
	Cmper   int    `xml:"cmper,attr"`
	Part    string `xml:"part,attr"`
	Beats   int    `xml:"beats"`
	DivBeat int    `xml:"divbeat"`
	Barline string `xml:"barline"`
	Key     *int   `xml:"keySig>key"`
}

type frameSpec struct {
	Cmper      int    `xml:"cmper,attr"`
	Part       string `xml:"part,attr"`
	StartEntry int    `xml:"startEntry"`
	EndEntry   int    `xml:"endEntry"`
}

type instUsed struct {
	Cmper int    `xml:"cmper,attr"`
	Inci  int    `xml:"inci,attr"`
	Part  string `xml:"part,attr"`
	Inst  int    `xml:"inst"`
}

type gfhold struct {
	Staff  int    `xml:"cmper1,attr"`
	Meas   int    `xml:"cmper2,attr"`
	Part   string `xml:"part,attr"`
	ClefID *int   `xml:"clefID"`
	Frame1 int    `xml:"frame1"`
	Frame2 int    `xml:"frame2"`
	Frame3 int    `xml:"frame3"`
	Frame4 int    `xml:"frame4"`
}

type entry struct {
	Num    int        `xml:"entnum,attr"`
	Next   int        `xml:"next,attr"`
	Dura   int        `xml:"dura"`
	IsNote *struct{}  `xml:"isNote"`
	Notes  []enigNote `xml:"note"`
}

type enigNote struct {
	// HarmLev is the note's place on the scale, counted in steps from
	// middle C: 0 is middle C, 7 the octave above it, -1 the B below.
	HarmLev int `xml:"harmLev"`
	// HarmAlt is what is done to it by hand, in semitones, on top of
	// whatever the key signature already says about that step.
	HarmAlt int `xml:"harmAlt"`
	// A tie is a property of the notehead, not of the entry, so a chord may
	// hold one note while the others move.
	TieStart *struct{} `xml:"tieStart"`
	TieEnd   *struct{} `xml:"tieEnd"`
}

// reader turns one EnigmaXML document into a score, and remembers what it
// had to leave behind.
type reader struct {
	doc     document
	skipped map[string]int
	// The articulations on each entry, and what each kind of articulation
	// is drawn with.
	artics    map[int][]int
	articChar map[int]int

	// Where every element landed, so that the marks Finale keeps apart
	// from the music can be put back onto it afterwards. places is keyed
	// by staff and bar and holds the elements in the order they are
	// played, with how far into the bar each one starts; byEntry finds one
	// by the entry it came from, which is how a slur names its ends.
	places  map[[2]int][]place
	byEntry map[int]*model.Marks
	// Which staff and bar are being read, since an element does not know.
	atStaff, atBar int
}

// place is one element of one bar: how far into the bar it starts, in EDUs,
// and what can be written on it.
type place struct {
	edu   int
	marks *model.Marks
}

// marksOf is what a note, a chord or a rest can carry.
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

func (r *reader) skip(what string) { r.skipped[what]++ }

// read parses the XML and builds the score.
func read(data []byte, meta metadata) (*model.Score, []string, error) {
	rd := &reader{
		skipped: map[string]int{},
		places:  map[[2]int][]place{},
		byEntry: map[int]*model.Marks{},
	}
	if err := xml.Unmarshal(data, &rd.doc); err != nil {
		return nil, nil, fmt.Errorf("musx: the score inside is not readable: %w", err)
	}

	score := model.NewScore()
	score.Parts = nil
	score.Identity = model.Identity{
		Title:     meta.Title,
		Subtitle:  meta.Subtitle,
		Composer:  meta.Composer,
		Copyright: meta.Copyright,
	}

	bars := rd.bars()
	if len(bars) == 0 {
		return nil, nil, fmt.Errorf("musx: the file holds no bars of music")
	}
	staves := rd.staffOrder()
	if len(staves) == 0 {
		return nil, nil, fmt.Errorf("musx: the file holds no staves")
	}

	entries := map[int]*entry{}
	for i := range rd.doc.Entries.Entries {
		e := &rd.doc.Entries.Entries[i]
		entries[e.Num] = e
	}
	frames := map[int]frameSpec{}
	for _, f := range rd.doc.Others.Frames {
		if f.Part == "" {
			frames[f.Cmper] = f
		}
	}
	holds := map[[2]int]gfhold{}
	for _, h := range rd.doc.Details.Holds {
		if h.Part == "" {
			holds[[2]int{h.Staff, h.Meas}] = h
		}
	}
	specs := map[int]staffSpec{}
	for _, s := range rd.doc.Others.Staves {
		if s.Part == "" {
			specs[s.Cmper] = s
		}
	}

	// The articulations, by the entry they are attached to.
	artics := map[int][]int{}
	for _, a := range rd.doc.Details.Artics {
		if a.Part == "" {
			artics[a.Entry] = append(artics[a.Entry], a.ArticDef)
		}
	}
	rd.artics = artics
	rd.articChar = map[int]int{}
	for _, d := range rd.doc.Others.Artics {
		if d.Part == "" {
			rd.articChar[d.Cmper] = d.CharMain
		}
	}

	for _, staffID := range staves {
		spec := specs[staffID]
		part := &model.Part{
			Name:       rd.text(spec.FullName),
			Instrument: model.AcousticGrandPiano,
			Clef:       model.Treble,
			Key:        keyOf(bars[0], rd),
			Time:       meterOf(bars[0], rd),
			Transpose:  transpositionOf(spec),
		}
		// The name is also the best guess at what the staff is: Finale
		// identifies its instruments by a code of its own, and the word
		// over the staff is what a reader has to go on anyway.
		if inst, ok := instrumentNamed(part.Name); ok {
			part.Instrument = inst
		}
		if spec.StaffLines != 0 && spec.StaffLines != 5 {
			rd.skip("staves ruled with something other than five lines")
		}
		// The clef of the first bar is the staff's clef; a change partway
		// through is a thing gscore writes into the bar, and reading those
		// is for a later pass.
		if h, ok := holds[[2]int{staffID, bars[0].Cmper}]; ok && h.ClefID != nil {
			part.Clef = clefOf(*h.ClefID, rd)
		}

		for _, bar := range bars {
			m := &model.Measure{
				Voices:       []*model.Voice{{}},
				RightBarline: barlineOf(bar.Barline, rd),
			}
			rd.atStaff, rd.atBar = staffID, bar.Cmper
			if h, ok := holds[[2]int{staffID, bar.Cmper}]; ok {
				if h.Frame2 != 0 || h.Frame3 != 0 || h.Frame4 != 0 {
					rd.skip("second, third and fourth voices on a staff")
				}
				if h.Frame1 != 0 {
					m.Voices[0].Elements = rd.frame(frames[h.Frame1], entries, bar)
				}
			}
			// A bar with nothing in it is a bar of rest. Finale writes no
			// entries for one and draws the rest itself — and the
			// documentation says as much: completely empty measures may
			// omit their hold. Left empty here they came out as blank bars
			// in the middle of the music, which is not what the part says.
			if len(m.Voices[0].Elements) == 0 {
				rest := &model.Rest{Dur: barDuration(bar)}
				m.Voices[0].Elements = []model.Element{rest}
				// A bar of rest can still carry a dynamic for what comes
				// after it, so it is remembered like anything else.
				rd.put(0, 0, rest)
			}
			part.Measures = append(part.Measures, m)
		}
		rd.pairTies(part)
		score.Parts = append(score.Parts, part)
	}

	// The marks Finale keeps away from the music, put back on it.
	rd.expressions()
	rd.shapes()

	// And everything in the file this reader never looks at, which is most
	// of what Finale writes. Counted from the document rather than from
	// what was attempted, because a thing that is never read is never
	// skipped either, and a log that stays silent about the slurs in a
	// score full of slurs is worse than no log at all.
	rd.survey(data)

	// What was left behind, said once each rather than once per note.
	var notes []string
	for _, what := range sortedKeys(rd.skipped) {
		notes = append(notes, fmt.Sprintf("%s: not read (%d found)", what, rd.skipped[what]))
	}
	return score, notes, nil
}

// unread names the records this importer walks past, in words rather than in
// Finale's own vocabulary: somebody reading the log wants to know that their
// slurs are gone, not that a smartShape was not handled.
//
// Several names can mean one thing to a reader — an expression is a
// textExprDef, a measExprAssign and a textBlock between them — so they share
// a phrase and are counted together.
var unread = map[string]string{
	// What is placed in the music. Finale also carries libraries of every
	// articulation, chord shape and drum map it knows whether or not the
	// piece uses one, and counting those would report two thousand chord
	// diagrams in a mass that has none — so only the records that put
	// something into this score are counted.
	// Articulations, ties, slurs, hairpins and dynamics are read, so they
	// are not in this list. What is left of those — a tempo word, an
	// ottava — is counted where it is passed over, which says how many
	// were left rather than how many the file holds.
	"chordAssign":       "chord symbols",
	"tupletDef":         "tuplets",
	"staffGroup":        "the braces and brackets joining staves together",
	"pageTextAssign":    "text blocks standing on the page",
	"repeatBack":        "repeats and their endings",
	"backRepeatBar":     "repeats and their endings",
	"repeatEndingStart": "repeats and their endings",
	"verse":             "lyrics",
	"chorus":            "lyrics",
	"section":           "lyrics",
}

// survey counts what is in the file that this reader does not read.
//
// A token scan rather than another unmarshal: the records are only being
// counted by name, and naming each one in a struct would mean this list
// growing a field for every kind of thing gscore cannot yet do.
func (r *reader) survey(data []byte) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	found := map[string]int{}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if el, ok := tok.(xml.StartElement); ok {
			if what, ok := unread[el.Name.Local]; ok {
				// The score's own copy, not the one kept for each
				// extracted part, or a slur is counted once per part.
				shared := false
				for _, a := range el.Attr {
					if a.Name.Local == "part" {
						shared = true
					}
				}
				if !shared {
					found[what]++
				}
			}
		}
	}
	for what, n := range found {
		r.skipped[what] += n
	}
}

// bars is the score's own measures, in order.
func (r *reader) bars() []measSpec {
	var out []measSpec
	for _, m := range r.doc.Others.Bars {
		if m.Part == "" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Cmper < out[b].Cmper })
	return out
}

// staffOrder is which staves the score shows, top to bottom.
//
// Not the order the staffSpecs are in: those follow the order the staves were
// added to the document, which is the order somebody built the score in and
// not the order it is read in. instUsed for the score view is the answer, and
// its incis are the rows of the page.
func (r *reader) staffOrder() []int {
	var used []instUsed
	for _, u := range r.doc.Others.Used {
		if u.Cmper == 0 && u.Part == "" {
			used = append(used, u)
		}
	}
	sort.Slice(used, func(a, b int) bool { return used[a].Inci < used[b].Inci })
	var out []int
	for _, u := range used {
		out = append(out, u.Inst)
	}
	if len(out) > 0 {
		return out
	}
	// A file with no score view: fall back to the staves themselves, which
	// is at least every staff once.
	for _, s := range r.doc.Others.Staves {
		if s.Part == "" && s.Cmper < 32767 {
			out = append(out, s.Cmper)
		}
	}
	sort.Ints(out)
	return out
}

// frame walks one bar of one staff: a linked list from the frame's first
// entry to its last.
//
// Followed rather than sliced. The entry numbers are a pool, not an order —
// the documentation says in as many words that arithmetic on them means
// nothing — so the only way through is next, and the only way to stop is to
// recognise the end.
func (r *reader) frame(f frameSpec, entries map[int]*entry, bar measSpec) []model.Element {
	var out []model.Element
	seen := map[int]bool{}
	// How far into the bar the next entry starts. Finale measures it in
	// EDUs, which is what a smart shape and an expression are placed by.
	at := 0
	for num := f.StartEntry; num != 0; {
		e, ok := entries[num]
		if !ok || seen[num] {
			break // a frame pointing at nothing, or a list that loops
		}
		seen[num] = true
		if el := r.element(e, bar); el != nil {
			out = append(out, el)
			r.put(at, e.Num, el)
		}
		at += e.Dura
		if num == f.EndEntry {
			break
		}
		num = e.Next
	}
	return out
}

// put remembers where an element landed.
func (r *reader) put(edu, entryNum int, el model.Element) {
	mk := marksOf(el)
	if mk == nil {
		return
	}
	key := [2]int{r.atStaff, r.atBar}
	r.places[key] = append(r.places[key], place{edu: edu, marks: mk})
	if entryNum != 0 {
		r.byEntry[entryNum] = mk
	}
}

// element turns one entry into a note, a chord or a rest.
func (r *reader) element(e *entry, bar measSpec) model.Element {
	dur, ok := duration(e.Dura)
	if !ok {
		r.skip("notes whose length has no written form, such as a tuplet member")
		return nil
	}
	if e.IsNote == nil || len(e.Notes) == 0 {
		rest := &model.Rest{Dur: dur}
		r.articulate(&rest.Marks, e.Num)
		return rest
	}
	if len(e.Notes) > 15 {
		r.skip("entries with more noteheads than Finale allows")
	}

	key := keyOf(bar, r)
	notes := make([]*model.Note, 0, len(e.Notes))
	for _, n := range e.Notes {
		notes = append(notes, r.note(n, dur, key))
	}
	if len(notes) == 1 {
		r.articulate(&notes[0].Marks, e.Num)
		return notes[0]
	}
	// Lowest first, which is the order everything downstream expects.
	sort.Slice(notes, func(a, b int) bool {
		return notes[a].Pitch.MIDI() < notes[b].Pitch.MIDI()
	})
	chord := &model.Chord{Notes: notes, Dur: dur}
	// A tie belongs to the notehead and is already on it; an articulation
	// belongs to the entry, so on a chord it belongs to the chord.
	r.articulate(&chord.Marks, e.Num)
	return chord
}

// note turns one notehead into a pitch.
//
// harmLev counts scale steps from middle C, so it gives the step and the
// octave on its own; harmAlt is what was done to the note by hand, over and
// above the key signature, so it gives both how the note sounds and whether
// an accidental is printed in front of it.
func (r *reader) note(n enigNote, dur model.Duration, key model.KeySignature) *model.Note {
	step := model.Step(mod(n.HarmLev, 7))
	octave := 4 + floorDiv(n.HarmLev, 7)

	out := &model.Note{
		Pitch: model.Pitch{
			Step:   step,
			Octave: octave,
			Alter:  key.AlterFor(step) + n.HarmAlt,
		},
		Dur: dur,
	}
	if n.HarmAlt != 0 {
		out.Accidental = accidentalFor(n.HarmAlt, r)
	}
	// A tie is written on the notehead it leaves and on the one it arrives
	// at, which is how gscore holds it too.
	out.TieStart = n.TieStart != nil
	out.TieStop = n.TieEnd != nil
	return out
}

// articulate puts an entry's articulation marks on what was made of it.
//
// Finale names each kind of mark by the character it is drawn with in its
// own music font, so a staccato is a full stop and a marcato is a caret —
// the glyph is the identity. These are the characters of the font Finale has
// shipped with since the beginning; a document whose articulations have been
// redrawn by hand would need its own reading, and says so.
func (r *reader) articulate(m *model.Marks, entry int) {
	for _, def := range r.artics[entry] {
		char, ok := r.articChar[def]
		if !ok {
			r.skip("articulation marks of a kind this file defines itself")
			continue
		}
		switch char {
		case '.':
			m.AddArticulation(model.Staccato)
		case '>':
			m.AddArticulation(model.Accent)
		case '^':
			m.AddArticulation(model.Marcato)
		case 'v':
			m.AddArticulation(model.MarcatoDown)
		case '-':
			m.AddArticulation(model.Tenuto)
		case 'U':
			m.AddArticulation(model.DownBow)
		case 'V':
			m.AddArticulation(model.UpBow)
		case 'u':
			m.AddArticulation(model.Fermata)
		default:
			r.skip("articulation marks gscore has no name for")
		}
	}
}

func accidentalFor(alt int, r *reader) model.Accidental {
	switch alt {
	case 1:
		return model.Sharp
	case -1:
		return model.Flat
	case 2:
		return model.DoubleSharp
	case -2:
		return model.DoubleFlat
	}
	r.skip("accidentals of more than two semitones")
	return model.NoAccidental
}

// duration turns Finale's EDUs into a written note value.
//
// A quarter note is 1024 of them, so the duration is simply that fraction of
// a whole note — but only some fractions have a written form. A value that
// cannot be spelled as a note with up to three dots belongs to a tuplet,
// whose members are stored with their real sounding length rather than the
// length they are written at, and that needs the tuplet records to undo.
func duration(edu int) (model.Duration, bool) {
	if edu <= 0 {
		return model.Duration{}, false
	}
	d := model.Duration{Num: edu, Den: eduWhole}.Reduced()
	if _, _, ok := d.Written(); !ok {
		return model.Duration{}, false
	}
	return d, true
}

// meterOf reads a bar's time signature.
//
// Finale stores how many beats the bar has and how long a beat is, which is
// not a time signature until it is asked what kind of beat it is. A beat of
// three parts — a dotted value — is a compound meter, and six eighths in two
// beats is written 6/8 rather than 2/1536.
func meterOf(bar measSpec, r *reader) model.TimeSignature {
	beats, div := bar.Beats, bar.DivBeat
	if beats <= 0 || div <= 0 {
		r.skip("bars with no time signature")
		return model.TimeSignature{Beats: 4, BeatValue: 4}
	}
	if div%3 == 0 {
		if unit := div / 3; isNoteValue(unit) {
			return model.TimeSignature{Beats: beats * 3, BeatValue: eduWhole / unit}
		}
	}
	if isNoteValue(div) {
		return model.TimeSignature{Beats: beats, BeatValue: eduWhole / div}
	}
	r.skip("time signatures whose beat is not a note value")
	return model.TimeSignature{Beats: 4, BeatValue: 4}
}

// isNoteValue says whether an EDU count is a plain note value — a power of
// two that divides a whole note.
func isNoteValue(edu int) bool {
	return edu > 0 && edu <= eduWhole && eduWhole%edu == 0 && edu&(edu-1) == 0
}

// keyOf reads a bar's key signature.
//
// Finale counts sharps up and flats down, as everything does; what it does
// not do is say so in a way that survives the modes and the nonstandard
// signatures it also allows, and those are left for a later pass rather than
// guessed at.
func keyOf(bar measSpec, r *reader) model.KeySignature {
	if bar.Key == nil {
		return model.KeySignature{}
	}
	k := *bar.Key
	if k >= -7 && k <= 7 {
		return model.KeySignature{Fifths: k}
	}
	r.skip("modes and nonstandard key signatures")
	return model.KeySignature{}
}

// clefOf maps Finale's clef numbers onto the clefs gscore draws.
//
// The numbers are positions in Finale's own clef table, which can be changed
// per document; these are the defaults, which is what all but a hand-altered
// file uses.
func clefOf(id int, r *reader) model.Clef {
	switch id {
	case 0:
		return model.Treble
	case 1:
		return model.Alto
	case 2:
		return model.Tenor
	case 3:
		return model.Bass
	case 4, 12:
		return model.Percussion
	case 5:
		// Treble with an 8 under it, which a tenor reads as sounding an
		// octave down. Drawn here as a treble clef: the notes are in the
		// right places and only the little 8 is missing.
		r.skip("the octave mark under a tenor's treble clef")
		return model.Treble
	}
	r.skip("clefs gscore has no name for")
	return model.Treble
}

func barlineOf(style string, r *reader) model.BarlineStyle {
	switch style {
	case "", "normal", "default":
		return model.BarSingle
	case "double", "thin":
		return model.BarDouble
	case "final", "heavy":
		return model.BarFinal
	case "none", "invisible":
		return model.BarNone
	}
	r.skip("barlines of a kind gscore does not draw")
	return model.BarSingle
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func mod(a, b int) int {
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// text resolves a staff's name: a textBlock cmper, which names a line of raw
// text, which is Finale's markup with the words somewhere inside it.
//
//	^fontid(2)^size(14)^nfx(0)Clarinette en Sib
//
// The tags say what it is set in, which gscore decides for itself, so what
// is wanted is everything that is not a tag.
func (r *reader) text(block int) string {
	if block == 0 {
		return ""
	}
	id := 0
	for _, b := range r.doc.Others.Blocks {
		if b.Part == "" && b.Cmper == block {
			id = b.TextID
			break
		}
	}
	if id == 0 {
		return ""
	}
	for _, t := range r.doc.Texts.Blocks {
		if t.Number == id {
			return strings.TrimSpace(enigmaTags.ReplaceAllString(t.Text, ""))
		}
	}
	return ""
}

// enigmaTags matches Finale's inline markup: a caret, a word, and what it
// takes in brackets.
var enigmaTags = regexp.MustCompile(`\^[A-Za-z]+\([^)]*\)|\^[A-Za-z]+`)

// transpositionOf reads how far a staff is written from where it sounds.
//
// Finale says it the way a player's part is described rather than the way a
// tuner would: so many scale steps up, and so many sharps added to the key
// signature. A B flat clarinet is one step up and two sharps; a baritone
// saxophone is twelve steps — an octave and a sixth — and three sharps.
//
// The semitones come from the sharps: each sharp is a fifth, and seven
// fifths wrap round to the same note a semitone at a time, so the interval
// within the octave is the number of sharps times seven. The octaves come
// from the steps.
func transpositionOf(s staffSpec) model.Transposition {
	interval, adjust := s.Transposition.Interval, s.Transposition.Adjust
	if interval == 0 && adjust == 0 {
		return model.Transposition{}
	}
	// Written above sounding, so the staff sounds below what is written and
	// every part of this is negative.
	return model.Transposition{
		Diatonic:     -mod(interval, 7),
		Chromatic:    -mod(adjust*7, 12),
		OctaveChange: -floorDiv(interval, 7),
	}
}

// barDuration is how much time a bar holds, for the rest that fills an empty
// one.
func barDuration(bar measSpec) model.Duration {
	if bar.Beats > 0 && bar.DivBeat > 0 {
		return model.Duration{Num: bar.Beats * bar.DivBeat, Den: eduWhole}.Reduced()
	}
	return model.Duration{Num: 1, Den: 1}
}

// instrumentNamed guesses the instrument from the word written over the
// staff.
//
// Finale identifies its instruments by a code of its own — a UUID per entry
// in its library — and the library is not in the file. The name is what is
// left, and it is what a reader has to go on anyway. French names are tried
// as well as English ones: the files this was written against are French,
// and a saxophone that plays back as a piano is a worse answer than a guess.
func instrumentNamed(name string) (model.Instrument, bool) {
	if name == "" {
		return 0, false
	}
	plain := strings.ToLower(strings.TrimSpace(name))
	for from, to := range map[string]string{
		"é": "e", "è": "e", "ê": "e", "û": "u", "ù": "u", "ô": "o", "î": "i", "à": "a", "ç": "c",
	} {
		plain = strings.ReplaceAll(plain, from, to)
	}
	if inst, ok := model.InstrumentFromName(plain); ok {
		return inst, true
	}
	// The common band and orchestra names, in French and in the shapes
	// people actually write them in.
	for _, alias := range []struct {
		match string
		inst  model.Instrument
	}{
		{"flute", model.Flute},
		{"piccolo", model.Piccolo},
		{"hautbois", model.Oboe},
		{"oboe", model.Oboe},
		{"clarinette basse", model.Clarinet},
		{"clarinette", model.Clarinet},
		{"clarinet", model.Clarinet},
		{"basson", model.Bassoon},
		{"bassoon", model.Bassoon},
		{"soprano sax", model.SopranoSax},
		{"alto sax", model.AltoSax},
		{"tenor sax", model.TenorSax},
		{"baritone sax", model.BaritoneSax},
		{"saxophone soprano", model.SopranoSax},
		{"saxophone alto", model.AltoSax},
		{"saxophone tenor", model.TenorSax},
		{"saxophone baryton", model.BaritoneSax},
		{"trompette", model.Trumpet},
		{"trumpet", model.Trumpet},
		{"cor", model.FrenchHorn},
		{"horn", model.FrenchHorn},
		{"trombone", model.Trombone},
		{"tuba", model.Tuba},
		{"violon", model.Violin},
		{"violin", model.Violin},
		{"alto", model.Viola},
		{"viola", model.Viola},
		{"violoncelle", model.Cello},
		{"cello", model.Cello},
		{"contrebasse", model.Contrabass},
		{"timbales", model.Timpani},
		{"timpani", model.Timpani},
		{"orgue", model.ChurchOrgan},
		{"piano", model.AcousticGrandPiano},
	} {
		if strings.Contains(plain, alias.match) {
			return alias.inst, true
		}
	}
	return 0, false
}

// pairTies joins each tied note to the note it is held into, and takes away
// the ties that go nowhere.
//
// Finale marks the note a tie leaves, and only sometimes the note it arrives
// at: in the score this was written against, ninety notes start a tie and
// thirty-five end one. The rest rely on the reader to find the note of the
// same pitch that follows — and following it means leaving the bar, because
// a bar's notes are their own linked list and a tie over a barline points
// out of it.
//
// Left to itself the engraver ties a note to the next note of that pitch
// wherever it is, so an unpaired tie came out as a curve sweeping across
// whole systems — which is what the flute part looked like before this: a
// line of music with slurs drawn over five bars at a time.
func (r *reader) pairTies(p *model.Part) {
	// Every note and chord of the staff in reading order, bars and all.
	var line []model.Element
	for _, m := range p.Measures {
		if len(m.Voices) == 0 {
			continue
		}
		for _, el := range m.Voices[0].Elements {
			switch el.(type) {
			case *model.Note, *model.Chord:
				line = append(line, el)
			}
		}
	}

	heads := func(el model.Element) []*model.Note {
		switch e := el.(type) {
		case *model.Note:
			return []*model.Note{e}
		case *model.Chord:
			return e.Notes
		}
		return nil
	}

	for i, el := range line {
		var next []*model.Note
		if i+1 < len(line) {
			next = heads(line[i+1])
		}
		for _, n := range heads(el) {
			if !n.TieStart {
				continue
			}
			tied := false
			for _, m := range next {
				if m.Pitch == n.Pitch {
					m.TieStop = true
					tied = true
					break
				}
			}
			if !tied {
				// A tie into nothing is not a tie. Saying so beats drawing
				// one across the page.
				n.TieStart = false
				r.skip("ties whose note is not held into the one after it")
			}
		}
	}
}

/*
 * The marks Finale keeps apart from the notes
 *
 * A dynamic, a slur and a hairpin are not written on the note in a Finale
 * file. They are rows in lists of their own, and each one says where it
 * lands: a staff, a bar, and either an entry or a position in the bar
 * measured in EDUs. Nothing in the music points back at them, so they are
 * read last, once every note is in place and can be found again.
 */

// at finds what is written on the music at a point in a bar.
//
// The element starting exactly there, or else the last one starting before
// it — which is what "under this beat" means when the mark is placed
// between two notes.
func (r *reader) at(staff, bar, edu int) *model.Marks {
	ps := r.places[[2]int{staff, bar}]
	if len(ps) == 0 {
		return nil
	}
	best := ps[0]
	for _, p := range ps {
		if p.edu > edu {
			break
		}
		best = p
	}
	return best.marks
}

// finaleDynamics is Finale's own ladder of velocities, which is how a
// dynamic is written down in a way that can be read without its font.
//
// The mark itself is a character of Maestro — "ë" for fortississimo — and
// reading that would mean carrying a table of one font's layout and being
// wrong for every document that uses another. The velocity says the same
// thing in a number: Finale's defaults climb in thirteens, and a document
// that has been edited still lands nearest the rung it means.
var finaleDynamics = []struct {
	velocity int
	dynamic  model.Dynamic
}{
	{10, model.PPPP}, {23, model.PPP}, {36, model.PP}, {49, model.Piano},
	{62, model.MP}, {75, model.MF}, {88, model.Forte}, {101, model.FF},
	{114, model.FFF}, {127, model.FFFF},
}

// dynamicFor turns a velocity into the dynamic nearest it.
func dynamicFor(velocity int) model.Dynamic {
	best, by := model.DynamicNone, 0
	for _, d := range finaleDynamics {
		gap := velocity - d.velocity
		if gap < 0 {
			gap = -gap
		}
		if best == model.DynamicNone || gap < by {
			best, by = d.dynamic, gap
		}
	}
	return best
}

// expressions writes the dynamics onto the notes they stand over.
func (r *reader) expressions() {
	dynamics := map[int]bool{}
	for _, c := range r.doc.Others.Categories {
		if c.Part == "" && c.Type == "dynamics" {
			dynamics[c.Cmper] = true
		}
	}
	defs := map[int]textExprDef{}
	for _, d := range r.doc.Others.Exprs {
		if d.Part == "" {
			defs[d.Cmper] = d
		}
	}
	for _, a := range r.doc.Others.ExprPlaces {
		if a.Part != "" {
			continue
		}
		def, ok := defs[a.Expr]
		switch {
		case !ok:
			r.skip("expressions whose definition is missing from the file")
			continue
		case !dynamics[def.Category] || def.PlayType != "amplitude":
			// A tempo word, a rehearsal direction, a technique: read as
			// text they would all be guesses about where they belong.
			r.skip("expressions other than dynamics: tempo words and directions")
			continue
		case a.Staff <= 0:
			// Written against a list of staves rather than one of them.
			r.skip("dynamics written across a group of staves at once")
			continue
		}
		mk := r.at(a.Staff, a.Meas, a.EduOff)
		if mk == nil {
			r.skip("dynamics on a bar this reader found nothing in")
			continue
		}
		if d := dynamicFor(def.Value); d != model.DynamicNone {
			mk.Dynamic = d
		}
	}
}

// shapes draws the slurs and hairpins back over the music.
func (r *reader) shapes() {
	for _, sh := range r.doc.Others.Shapes {
		if sh.Part != "" || sh.Hidden != nil {
			continue
		}
		switch {
		case strings.Contains(strings.ToLower(sh.ShapeType), "slur"):
			// A slur hangs on its two entries, which is the one kind of
			// shape that says exactly which notes it joins.
			from, to := r.byEntry[sh.Start.Entry], r.byEntry[sh.End.Entry]
			if from == nil || to == nil {
				r.skip("slurs whose notes are not in the part this reader built")
				continue
			}
			if from == to {
				// A slur from a note to itself is nothing to draw, and
				// both ends on one note would be a curve over no music.
				continue
			}
			from.SlurStart = true
			to.SlurStop = true
		case sh.ShapeType == "cresc", sh.ShapeType == "decresc":
			kind := model.Crescendo
			if sh.ShapeType == "decresc" {
				kind = model.Diminuendo
			}
			from := r.at(sh.Start.Inst, sh.Start.Meas, sh.Start.Edu)
			to := r.at(sh.End.Inst, sh.End.Meas, sh.End.Edu)
			if from == nil || to == nil {
				r.skip("hairpins over a bar this reader found nothing in")
				continue
			}
			if from == to {
				from.Hairpin = &model.Hairpin{Kind: kind, Start: true, Stop: true}
				continue
			}
			from.Hairpin = &model.Hairpin{Kind: kind, Start: true}
			to.Hairpin = &model.Hairpin{Kind: kind, Stop: true}
		default:
			// Ottavas, trills, glissandi, bracketed lines: each needs its
			// own reading and none of them is a guess worth making.
			r.skip("lines over the music other than slurs and hairpins")
		}
	}
}
