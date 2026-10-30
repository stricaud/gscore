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
		Suffixes   []chordSuffixPart  `xml:"chordSuffix"`
		Styles     []staffStyle       `xml:"staffStyle"`
		StyleUsed  []staffStyleAssign `xml:"staffStyleAssign"`
		PercNotes  []percussionNote   `xml:"percussionNoteInfo"`
		PageTexts  []pageTextAssign   `xml:"pageTextAssign"`
		Exprs      []textExprDef      `xml:"textExprDef"`
		ExprPlaces []measExprAssign   `xml:"measExprAssign"`
		Categories []markingsCategory `xml:"markingsCategory"`
	} `xml:"others"`
	Details struct {
		Holds  []gfhold         `xml:"gfhold"`
		Artics []articAssign    `xml:"articAssign"`
		Chords []chordAssign    `xml:"chordAssign"`
		Perc   []percussionCode `xml:"percussionNoteCode"`
		// A group is kept with the other records that name two things at
		// once rather than with the staves themselves.
		Groups []staffGroup `xml:"staffGroup"`
	} `xml:"details"`
	Entries struct {
		Entries []entry `xml:"entry"`
	} `xml:"entries"`
	Texts struct {
		Blocks []blockText `xml:"blockText"`
		// The words of the marks written on the music, which Finale keeps
		// apart from the words laid on the page. A text block says which
		// of the two pools it draws from, and a reader that knows only
		// about one of them finds every tempo word and direction empty.
		Exprs []blockText `xml:"expression"`
	} `xml:"texts"`
}

// A staff's name is three lookups away from the staff: staffSpec names a
// textBlock, the textBlock names a line of raw text, and the raw text is
// Finale's own markup with the words somewhere inside it.
type textBlock struct {
	Cmper  int    `xml:"cmper,attr"`
	Part   string `xml:"part,attr"`
	TextID int    `xml:"textID"`
	// Tag is which pool the words are in: "expression" for a mark on the
	// music, "block" for something laid on the page.
	Tag string `xml:"textTag"`
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

// chordAssign is a chord symbol written over one bar of one staff.
//
// The root is a scale degree rather than a letter — 0 is the tonic, which is
// the same counting harmLev uses — so the symbol only means anything once
// the key is known. The quality is a number naming a row of chordSuffix
// records, which spell it out one character at a time.
type chordAssign struct {
	Staff int    `xml:"cmper1,attr"`
	Meas  int    `xml:"cmper2,attr"`
	Part  string `xml:"part,attr"`

	RootScaleNum int       `xml:"rootScaleNum"`
	RootAlter    int       `xml:"rootAlter"`
	RootLower    *struct{} `xml:"rootLowerCase"`
	Suffix       int       `xml:"suffix"`
	BassScaleNum int       `xml:"bassScaleNum"`
	BassAlter    int       `xml:"bassAlter"`
	BassLower    *struct{} `xml:"bassLowerCase"`
	ShowAltBass  *struct{} `xml:"showAltBass"`
	HideRoot     *struct{} `xml:"hideRoot"`
	// HorzEdu is how far into the bar it is written, which is how two
	// chords in one bar are told apart.
	HorzEdu int `xml:"horzEdu"`
}

// chordSuffixPart is one character of a chord's quality.
//
// Finale keeps the suffix as a row of these — "m" then "7", or a 6 raised
// above a 9 — each with the font it is drawn in. A number is held as the
// number; anything else is the code of a character.
type chordSuffixPart struct {
	Cmper    int       `xml:"cmper,attr"`
	Inci     int       `xml:"inci,attr"`
	Part     string    `xml:"part,attr"`
	Value    int       `xml:"suffix"`
	IsNumber *struct{} `xml:"isNumber"`
}

// staffStyle is a way of writing a staff that is not notes: slashes for a
// rhythm part, a bar repeat sign, a staff left blank.
type staffStyle struct {
	Cmper int    `xml:"cmper,attr"`
	Part  string `xml:"part,attr"`
	Alt   string `xml:"altNotation"`
}

// staffStyleAssign puts one of those over a run of bars of one staff.
type staffStyleAssign struct {
	Staff     int    `xml:"cmper,attr"`
	Part      string `xml:"part,attr"`
	Style     int    `xml:"style"`
	StartMeas int    `xml:"startMeas"`
	EndMeas   int    `xml:"endMeas"`
}

// percussionNote is one row of a percussion layout: an instrument, the line
// it is written on, and the notehead it is drawn with.
type percussionNote struct {
	Cmper    int    `xml:"cmper,attr"`
	Part     string `xml:"part,attr"`
	NoteType int    `xml:"percNoteType"`
	HarmLev  int    `xml:"harmLev"`
	// Closed is the filled notehead, as a character of the music font —
	// which is SMuFL, so the number says what the shape is.
	Closed int `xml:"closedNotehead"`
}

// percussionCode says which instrument a note on a drum staff is.
type percussionCode struct {
	Entry int    `xml:"entnum,attr"`
	Part  string `xml:"part,attr"`
	Note  int    `xml:"noteID"`
	Code  int    `xml:"noteCode"`
}

// staffGroup joins staves that belong together: the two hands of a piano,
// the woodwind of a band.
type staffGroup struct {
	Part      string `xml:"part,attr"`
	StartInst int    `xml:"startInst"`
	EndInst   int    `xml:"endInst"`
	FullName  int    `xml:"fullID"`
	Bracket   struct {
		ID int `xml:"id"`
	} `xml:"bracket"`
	BarlineStyle string `xml:"groupBarlineStyle"`
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
	// Block is the text block holding the words the mark is printed with.
	// Desc beside it is the library's description of the mark — "Allegro
	// (no playback effect)" — which is written for whoever is choosing one
	// out of a list and never appears on a page.
	Block int `xml:"textIDKey"`
	// Beat is the note the speed is counted in, in Finale's own divisions:
	// 1024 of them to a quarter. Only a mark that sets the speed has one.
	Beat int `xml:"auxdata1"`
	// Vert says which side of the staff the mark is written on.
	Vert string `xml:"vertMeasExprAlign"`
}

// measExprAssign is one expression written at one place in the music: which
// bar, which staff, and how far into the bar.
type measExprAssign struct {
	Meas   int    `xml:"cmper,attr"`
	Part   string `xml:"part,attr"`
	Expr   int    `xml:"textExprID"`
	Staff  int    `xml:"staffAssign"`
	EduOff int    `xml:"horzEduOff"`
	// Inci orders the marks written on one bar, which is the order they
	// were put there and the only thing that tells two of them apart.
	Inci int `xml:"inci,attr"`
}

// pageTextAssign is a block of words laid on a page rather than on the
// music: a header, a footer, a title, a page number.
type pageTextAssign struct {
	Part  string `xml:"part,attr"`
	Block int    `xml:"block"`
	Page  int    `xml:"startPage"`
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
	Cmper      int    `xml:"cmper,attr"`
	Part       string `xml:"part,attr"`
	StaffLines int    `xml:"staffLines"`
	// Notation says a staff is not written as pitches: a drum staff says
	// "percussion", and what its notes mean is a position on the staff
	// rather than a note of the scale.
	Notation    string `xml:"notationStyle"`
	DefaultClef int    `xml:"defaultClef"`
	// The clef the staff is read from when it is read transposed, which is
	// not always the one it sounds in. Left out of the file when it is the
	// treble, which is what most of the staves that change clef are read
	// from, so an absent field is a treble clef and not a missing answer.
	TransposedClef int `xml:"transposedClef"`
	FullName       int `xml:"fullName"`
	AbbrvName      int `xml:"abbrvName"`
	// How far the staff is written from where it sounds, as a diatonic
	// interval and a number of sharps added to the key signature — which is
	// how a player's part is described rather than how a tuner describes
	// it. A B flat clarinet is one step up and two sharps.
	Transposition struct {
		Interval int `xml:"keysig>interval"`
		Adjust   int `xml:"keysig>adjust"`
		// Present when the staff changes clef along with its pitches.
		// An empty element: there to be there, with nothing in it.
		SetToClef *struct{} `xml:"setToClef"`
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
	// HarmLev is the note's place on the scale, counted in steps from the
	// tonic of the key: 0 is the tonic in the octave that starts at middle
	// C, 7 the octave above it, -1 the step below. In C major that is
	// middle C itself, which is why reading it as middle C always was
	// right until a piece in another key came along — see tonicStep.
	HarmLev int `xml:"harmLev"`
	// HarmAlt is what is done to it by hand, in semitones, on top of
	// whatever the key signature already says about that step.
	//
	// Both describe the note as it sounds. A transposing staff shows
	// another note, which asWritten works out.
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
	// examples is a few of what each kind of skipped thing actually was,
	// in the file's own words: "Solo" at bar 12 says more to whoever opens
	// the score than "one direction was not read" ever can.
	examples map[string][]string
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
	// barAt turns Finale's number for a bar into its place in the score,
	// and partAt turns a staff's number into the part built from it. The
	// marks that govern a bar rather than a note — a change of speed, a
	// rehearsal letter — are written against those numbers and have to be
	// put back onto the right measure of the right part.
	barAt  map[int]int
	partAt map[int]int
	// Which staff and bar are being read, since an element does not know,
	// and how that staff transposes — what Finale stores is the note that
	// sounds, and a transposing staff shows another one.
	atStaff, atBar int
	transpose      model.Transposition
	// percussion says the staff being read is a drum staff, where a note
	// is a place on the staff rather than a pitch.
	percussion bool
	// perc is the instrument each note is, by the entry it belongs to.
	perc map[int]int
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

// skipExamples is how many of a kind of skipped thing are named in the
// report before it settles for counting the rest.
//
// Enough to recognise what was lost and few enough to read: a score with
// forty directions in it should say what three of them were and then say
// there are thirty-seven more, not list the lot.
const skipExamples = 3

// skipped records one thing not read, in the file's own words.
//
// The words matter more than the count. "Directions: not read (2 found)"
// sends somebody back to Finale to work out which two; "Andante at bar 1,
// Andantino mosso at bar 10" tells them what to type back in, and tells
// them at a glance whether what was lost was worth anything.
func (r *reader) skipWhat(what, detail string) {
	r.skipped[what]++
	if detail == "" || len(r.examples[what]) >= skipExamples {
		return
	}
	if r.examples == nil {
		r.examples = map[string][]string{}
	}
	r.examples[what] = append(r.examples[what], detail)
}

// atBarOf names where something is, for the report. Finale counts its bars
// from one, as a reader does.
func atBar(bar int) string { return fmt.Sprintf("at bar %d", bar) }

// read parses the XML and builds the score.
func read(data []byte, meta metadata) (*model.Score, []string, error) {
	rd := &reader{
		skipped:  map[string]int{},
		examples: map[string][]string{},
		barAt:    map[int]int{},
		partAt:   map[int]int{},
		places:   map[[2]int][]place{},
		byEntry:  map[int]*model.Marks{},
		perc:     map[int]int{},
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
	// Finale's numbers for the bars and the staves, against the places
	// they end up in here. Both are needed by the marks that are written
	// against a bar rather than hung on a note.
	for i, b := range bars {
		rd.barAt[b.Cmper] = i
	}
	for i, id := range staves {
		rd.partAt[id] = i
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
	for _, p := range rd.doc.Details.Perc {
		if p.Part == "" {
			rd.perc[p.Entry] = p.Code
		}
	}
	rd.articChar = map[int]int{}
	for _, d := range rd.doc.Others.Artics {
		if d.Part == "" {
			rd.articChar[d.Cmper] = d.CharMain
		}
	}

	for _, staffID := range staves {
		spec := specs[staffID]
		rd.transpose = transpositionOf(spec)
		rd.percussion = spec.Notation == "percussion"
		part := &model.Part{
			Name:       rd.text(spec.FullName),
			Instrument: model.AcousticGrandPiano,
			Clef:       model.Treble,
			Key:        writtenKey(keyOf(bars[0], rd), spec),
			Time:       meterOf(bars[0], rd),
			Transpose:  rd.transpose,
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
		var first *int
		if h, ok := holds[[2]int{staffID, bars[0].Cmper}]; ok {
			first = h.ClefID
		}
		part.Clef = writtenClef(spec, first, rd)

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
	rd.expressions(score)
	rd.pageTexts()
	rd.shapes()
	rd.chordSymbols(score)
	rd.staffGroups(score, staves)
	// Last, because it replaces what is in the bars it covers and keeps
	// the chord symbol that was put there a moment ago.
	rd.slashStaves(score, staves)

	// And everything in the file this reader never looks at, which is most
	// of what Finale writes. Counted from the document rather than from
	// what was attempted, because a thing that is never read is never
	// skipped either, and a log that stays silent about the slurs in a
	// score full of slurs is worse than no log at all.
	rd.survey(data)

	// What was left behind, said once each rather than once per note, and
	// with a few of them named: a list of kinds says what sort of thing is
	// missing and a list of examples says whether it mattered.
	var notes []string
	for _, what := range sortedKeys(rd.skipped) {
		n := rd.skipped[what]
		line := fmt.Sprintf("%s: not read (%d found)", what, n)
		if eg := rd.examples[what]; len(eg) > 0 {
			line += ": " + strings.Join(eg, ", ")
			if rest := n - len(eg); rest > 0 {
				line += fmt.Sprintf(", and %d more", rest)
			}
		}
		notes = append(notes, line)
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

	"tupletDef": "tuplets",

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
		note := r.note(n, dur, key)
		if r.percussion {
			// Which instrument it is decides how it is drawn: a cross for
			// a cymbal, a plain head for a drum.
			note.Head = r.headFor(r.perc[e.Num], n.HarmLev)
		}
		notes = append(notes, note)
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
	// harmLev counts from the tonic of the key, and the tonic is not always
	// middle C.
	//
	// Read from middle C whatever the key, every note of every piece that
	// is not in C came out wrong by the distance from C to its tonic — a
	// fourth in G, a fifth in F — with the music itself intact, since every
	// note moved together. Jingle Bells in G arrived with its guitar
	// playing F under a chord symbol saying C.
	//
	// It is also why this was not noticed: the Rossini that went through
	// this reader before is in C major, where the tonic is middle C and the
	// old reading is the right one.
	if r.percussion {
		// A drum staff has no key and no transposition: what the number
		// says is which line the note is written on. Read as a pitch it
		// came out a sixth above the staff, where Finale draws it on the
		// top line.
		idx := n.HarmLev - percussionOctave
		return &model.Note{
			Pitch: model.Pitch{
				Step:   model.Step(mod(idx, 7)),
				Octave: 4 + floorDiv(idx, 7),
			},
			Dur: dur,
		}
	}

	idx := n.HarmLev + tonicStep(key)
	sounds := model.Pitch{
		Step:   model.Step(mod(idx, 7)),
		Octave: 4 + floorDiv(idx, 7),
	}
	sounds.Alter = key.AlterFor(sounds.Step) + n.HarmAlt

	out := &model.Note{
		// What Finale keeps is the note as it sounds. A clarinet's staff
		// holds the note the player reads, which is another note entirely,
		// and everything downstream — the page, the transposition, the
		// playback — is built on the written one.
		Pitch: asWritten(sounds, r.transpose),
		Dur:   dur,
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
	id, tag := 0, ""
	for _, b := range r.doc.Others.Blocks {
		if b.Part == "" && b.Cmper == block {
			id, tag = b.TextID, b.Tag
			break
		}
	}
	if id == 0 {
		return ""
	}
	pool := r.doc.Texts.Blocks
	if tag == "expression" {
		pool = r.doc.Texts.Exprs
	}
	for _, t := range pool {
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
				r.skip("ties Finale left on a note that is not held into " +
					"the next one, and does not draw either")
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

// pageTexts reports the words laid on a page rather than on the music.
//
// Most of them are not words at all. Finale builds its headers and footers
// out of inserts — a title block is the single token ^title(), a footer is
// ^page(0) — and fills them in from what the file says it is called. gscore
// has those from the same place and writes its own, so a page made of
// inserts has lost nothing and saying "seven text blocks were not read"
// about it sends somebody looking for seven things that are already there.
//
// What is worth reporting is a page carrying words somebody typed: a
// dedication, a note to the player, an instruction at the front. Those have
// nowhere to go in a score that keeps its text with the music, so they are
// named — in their own words, which is the only way to judge whether they
// mattered.
func (r *reader) pageTexts() {
	for _, t := range r.doc.Others.PageTexts {
		if t.Part != "" {
			continue
		}
		words := r.text(t.Block)
		if words == "" {
			continue
		}
		where := "on the page"
		if t.Page > 0 {
			where = fmt.Sprintf("on page %d", t.Page)
		}
		r.skipWhat("words laid on the page rather than on the music",
			fmt.Sprintf("%q %s", words, where))
	}
}

// expressions writes the marks Finale keeps away from the music back onto
// it: the dynamics, the tempo words, the rehearsal letters and the
// directions to the player.
//
// Finale says such a mark in three records. A markingsCategory says what
// kind of thing it is, a textExprDef is the mark itself — its words, and
// what it does to playback — and a measExprAssign is one place it is
// written. Which kind it is decides where it belongs in a score: a tempo
// word governs a bar, a rehearsal letter names one, and a direction hangs
// on the note it stands over.
func (r *reader) expressions(score *model.Score) {
	cats := map[int]string{}
	for _, c := range r.doc.Others.Categories {
		if c.Part == "" {
			cats[c.Cmper] = c.Type
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
		if !ok {
			r.skip("expressions whose definition is missing from the file")
			continue
		}
		words := r.text(def.Block)
		switch cats[def.Category] {
		case "dynamics":
			r.dynamicMark(a, def, words)
		case "tempoMarks", "tempoAlts":
			r.tempoMark(score, a, def, words)
		case "rehearsalMarks":
			r.rehearsalMark(score, a, words)
		default:
			// Expressive text, technique text, and whatever a document has
			// made its own categories for: all of them are words written
			// on a note, which is what a direction is.
			r.direction(a, def, words)
		}
	}
}

// dynamicMark writes a dynamic onto the note it stands over.
//
// The mark is drawn as a character of Finale's music font, which is of no
// use to anybody else: "ë" is fortississimo only if you have Maestro and
// know its layout. What the definition also carries is what the mark does,
// and that reads the same everywhere — a velocity, which is the dynamic
// said in numbers.
func (r *reader) dynamicMark(a measExprAssign, def textExprDef, words string) {
	d := dynamicFor(def.Value)
	if def.PlayType != "amplitude" || d == model.DynamicNone {
		// A word filed under dynamics that does not sound like one: "sfz"
		// spelled out, "sempre p". It is a direction, and reads as one.
		r.direction(a, def, words)
		return
	}
	if a.Staff <= 0 {
		r.skipWhat("dynamics written across a group of staves at once",
			detailOf(words, a.Meas))
		return
	}
	mk := r.at(a.Staff, a.Meas, a.EduOff)
	if mk == nil {
		r.skipWhat("dynamics on a bar this reader found nothing in",
			detailOf(words, a.Meas))
		return
	}
	mk.Dynamic = d
}

// tempoMark writes a change of speed over its bar.
//
// A tempo word and a metronome mark are one thing in Finale and one thing
// here: "Andantino mosso" with no speed behind it is a mark with words and
// no number, and the score goes on at the speed it was already going.
func (r *reader) tempoMark(score *model.Score, a measExprAssign, def textExprDef, words string) {
	t := model.Tempo{Text: words}
	// Only a mark that drives playback carries a speed. The rest are words
	// — which is most of them, and is why a tempo with no number has to be
	// a thing this score can hold.
	if def.PlayType == "time" && def.Value > 0 {
		if unit, ok := duration(def.Beat); ok {
			t.BPM, t.Unit = def.Value, unit
		}
	}
	if t.Text == "" && t.BPM == 0 {
		r.skipWhat("tempo marks with no words and no speed in them", atBar(a.Meas))
		return
	}
	m := r.measureFor(score, a.Staff, a.Meas)
	if m == nil {
		r.skipWhat("tempo marks on a bar this reader did not build",
			detailOf(words, a.Meas))
		return
	}
	if m.Tempo != nil {
		// The same mark written over every staff of the system, which is
		// how Finale spells one direction to the whole band.
		return
	}
	m.Tempo = &t
}

// rehearsalMark names a bar with the letter a conductor calls it by.
func (r *reader) rehearsalMark(score *model.Score, a measExprAssign, words string) {
	if words == "" {
		// Finale can letter a score as it goes, in which case the mark
		// carries a style and no letter and the letters are whatever the
		// sequence makes them. Working that out means following every
		// marker in the piece, and a letter guessed wrong is worse than a
		// letter missing: a rehearsal mark is an address.
		r.skipWhat("rehearsal marks Finale letters by itself", atBar(a.Meas))
		return
	}
	m := r.measureFor(score, a.Staff, a.Meas)
	if m == nil {
		r.skipWhat("rehearsal marks on a bar this reader did not build",
			detailOf(words, a.Meas))
		return
	}
	if m.Marker == "" {
		m.Marker = words
	}
}

// direction writes words on the note they stand over: "solo", "pizz.",
// "dolce", "rit." — anything said to the player that is not a dynamic.
func (r *reader) direction(a measExprAssign, def textExprDef, words string) {
	if words == "" {
		// A mark drawn as a character of Finale's own music font rather
		// than as words: the sign for a breath, a pedal, a bowing. Reading
		// it would mean carrying a table of one font's layout and being
		// wrong for every document set in another.
		r.skipWhat("marks drawn as characters of Finale's music font", atBar(a.Meas))
		return
	}
	if a.Staff <= 0 {
		r.skipWhat("directions written across a group of staves at once",
			detailOf(words, a.Meas))
		return
	}
	mk := r.at(a.Staff, a.Meas, a.EduOff)
	if mk == nil {
		r.skipWhat("directions on a bar this reader found nothing in",
			detailOf(words, a.Meas))
		return
	}
	where := model.PlacementAuto
	switch def.Vert {
	case "aboveStaffOrEntry":
		where = model.PlacementAbove
	case "belowStaffOrEntry":
		where = model.PlacementBelow
	}
	mk.Texts = append(mk.Texts, model.TextMark{Text: words, Placement: where})
}

// measureFor is the bar a mark written over a staff belongs to.
//
// A mark written against a list of staves rather than one of them — which
// is how Finale says "this is for the whole band" — belongs to the top
// staff, because that is where a score prints it.
func (r *reader) measureFor(score *model.Score, staff, bar int) *model.Measure {
	i, ok := r.barAt[bar]
	if !ok {
		return nil
	}
	pi := 0
	if staff > 0 {
		p, ok := r.partAt[staff]
		if !ok {
			return nil
		}
		pi = p
	}
	if pi >= len(score.Parts) || i >= len(score.Parts[pi].Measures) {
		return nil
	}
	return score.Parts[pi].Measures[i]
}

// detailOf is one line of the report: what the mark said and where it was.
func detailOf(words string, bar int) string {
	if words == "" {
		return atBar(bar)
	}
	return fmt.Sprintf("%q %s", words, atBar(bar))
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

/*
 * Where harmLev counts from, and what it counts
 *
 * Two things about a Finale note are not what they look like. It is written
 * down as a displacement from the tonic of the key rather than from middle
 * C, and the note it describes is the one that sounds rather than the one on
 * the staff. Both are invisible in a piece in C major played by instruments
 * that do not transpose, which is exactly the kind of file this reader was
 * built against.
 */

// tonicStep is how far the key's tonic is above middle C, in diatonic steps.
//
// A fifth up the circle is four steps up the scale, which is all this is: C
// is 0, G is 4, D is 1, F is 3. A minor key counts from its own tonic, five
// steps above the major one that shares its signature.
func tonicStep(key model.KeySignature) int {
	step := mod(4*key.Fifths, 7)
	if key.Mode == model.Minor {
		step = mod(step+5, 7)
	}
	return step
}

// asWritten turns the note that sounds into the note on the staff.
//
// A transposition says what a staff sounds against what it shows — sounding
// is written plus the interval — so the written note is the sounding one
// with that interval taken back off. The diatonic part decides which line
// the note sits on and the chromatic part decides its accidental, and they
// are not the same number: a B flat clarinet reads a step up and sounds two
// semitones down.
func asWritten(sounds model.Pitch, t model.Transposition) model.Pitch {
	if t == (model.Transposition{}) {
		return sounds
	}
	idx := sounds.DiatonicIndex() - (t.Diatonic + 7*t.OctaveChange)
	out := model.Pitch{
		Step:   model.Step(mod(idx, 7)),
		Octave: 4 + floorDiv(idx, 7),
	}
	// Whatever accidental makes the written note sound like the one Finale
	// wrote down, which is the one thing that must not be guessed at.
	plain := model.Pitch{Step: out.Step, Octave: out.Octave}
	out.Alter = sounds.MIDI() - (t.Chromatic + 12*t.OctaveChange) - plain.MIDI()
	return out
}

// writtenClef is the clef a staff reads from.
//
// gscore keeps a transposing staff's music as the player reads it, so the
// clef has to be the written one as well, and for some instruments that is
// not the clef the music sounds in. Finale keeps the two apart: the bars
// carry the sounding clef, and the staff carries a second clef to be read
// from instead whenever it says to set the clef along with the pitches.
//
// A baritone saxophone is the case that showed it. It sounds where the bass
// clef is and is read in the treble, and given the sounding clef its part
// came out with every note three ledger lines above the staff.
func writtenClef(s staffSpec, bar *int, r *reader) model.Clef {
	if s.Transposition.SetToClef != nil {
		return clefOf(s.TransposedClef, r)
	}
	if bar != nil {
		return clefOf(*bar, r)
	}
	// A staff whose first bar says nothing about its clef still has one.
	return clefOf(s.DefaultClef, r)
}

// writtenKey is the key signature a transposing staff reads from.
//
// Finale keeps the number of fifths the written key is away from the one the
// piece sounds in: a B flat clarinet in a piece in C major reads in D, two
// sharps. Left as the concert key, the staff printed the right notes against
// the wrong signature and every one of them wanted an accidental.
func writtenKey(key model.KeySignature, s staffSpec) model.KeySignature {
	key.Fifths += s.Transposition.Adjust
	return key
}

/*
 * The chord symbols
 *
 * A chart's chords are not notes and are not text. Finale keeps the root as
 * a scale degree — 0 is the tonic, the same counting harmLev uses — and the
 * quality as a row of characters in a library, so a symbol only means
 * anything once the key is known and the row is spelled out.
 */

// chordSymbols writes the chords over the staves they belong to.
func (r *reader) chordSymbols(score *model.Score) {
	// The quality of each chord, spelled once rather than per bar.
	quality := map[int]string{}
	parts := map[int][]chordSuffixPart{}
	for _, p := range r.doc.Others.Suffixes {
		if p.Part == "" {
			parts[p.Cmper] = append(parts[p.Cmper], p)
		}
	}
	for cmper, ps := range parts {
		sort.Slice(ps, func(a, b int) bool { return ps[a].Inci < ps[b].Inci })
		quality[cmper] = suffixText(ps)
	}

	for _, c := range r.doc.Details.Chords {
		if c.Part != "" {
			continue
		}
		mk := r.at(c.Staff, c.Meas, c.HorzEdu)
		if mk == nil {
			r.skip("chord symbols over a bar this reader found nothing in")
			continue
		}
		key := r.keyAt(c.Meas)
		sym := &model.ChordSymbol{
			Root:    degreeName(key, c.RootScaleNum, c.RootAlter, c.RootLower != nil),
			Quality: quality[c.Suffix],
		}
		if c.ShowAltBass != nil {
			sym.Bass = degreeName(key, c.BassScaleNum, c.BassAlter, c.BassLower != nil)
		}
		if c.HideRoot != nil {
			sym.Root = ""
		}
		if sym.Root == "" && sym.Quality == "" && sym.Bass == "" {
			continue
		}
		mk.Chord = sym
	}
}

// degreeName spells a scale degree as a chord root: the letter the key gives
// it, and whatever the symbol does to it on top.
func degreeName(key model.KeySignature, degree, alter int, lower bool) string {
	step := model.Step(mod(degree+tonicStep(key), 7))
	name := step.String()
	if lower {
		name = strings.ToLower(name)
	}
	switch n := key.AlterFor(step) + alter; {
	case n > 0:
		name += strings.Repeat("#", n)
	case n < 0:
		name += strings.Repeat("b", -n)
	}
	return name
}

// suffixText spells a chord's quality out of the characters Finale draws it
// with.
//
// A number is the number. Anything else is a character code, and the ones
// worth knowing are the four that are not letters: the symbols Finale's
// chord font puts where another program would write "maj", "dim", "m" and
// "+". A code that means nothing here is left out rather than printed as
// whatever letter happens to share its number, which would turn a triangle
// into a capital D.
func suffixText(parts []chordSuffixPart) string {
	var b strings.Builder
	for _, p := range parts {
		switch {
		case p.IsNumber != nil:
			b.WriteString(itoa(p.Value))
		case p.Value >= ' ' && p.Value <= '~':
			b.WriteByte(byte(p.Value))
		default:
			if s, ok := chordFontSymbols[p.Value]; ok {
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

// chordFontSymbols are the characters of Finale's chord font that stand for
// something rather than spelling it.
var chordFontSymbols = map[int]string{
	0xE870: "°", // diminished
	0xE871: "ø", // half-diminished
	0xE872: "Δ", // major seventh
	0xE873: "-", // minor
	0xE874: "+", // augmented
	0x2206: "Δ", // the same triangle, where the font uses the increment sign
	0x00B0: "°",
	0x00F8: "ø",
	0x2205: "ø",
}

// itoa is strconv without the import, which this package does not otherwise
// need.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// keyAt is the key a bar is in, by the bar's own number.
func (r *reader) keyAt(meas int) model.KeySignature {
	for _, b := range r.doc.Others.Bars {
		if b.Part == "" && b.Cmper == meas {
			return keyOf(b, r)
		}
	}
	return model.KeySignature{}
}

/*
 * The staves that belong together
 *
 * A score is not a stack of unrelated lines. The two hands of a piano are
 * one instrument and are drawn with a brace and their barlines joined; the
 * woodwind of a band are several and are drawn with a bracket. Finale keeps
 * that as a group over a range of staves, with the name on the group rather
 * than on either staff — which is why the piano's two staves in Jingle Bells
 * came out with no names at all.
 */

// staffGroups joins the staves that Finale says belong together.
//
// order is the staves as they were read, since a group names the staves by
// their own numbers and the score knows them by their place on the page.
func (r *reader) staffGroups(score *model.Score, order []int) {
	place := map[int]int{}
	for i, staffID := range order {
		place[staffID] = i
	}
	for _, g := range r.doc.Details.Groups {
		if g.Part != "" {
			continue
		}
		first, ok := place[g.StartInst]
		last, ok2 := place[g.EndInst]
		if !ok || !ok2 || last < first {
			r.skip("groups over staves this reader did not read")
			continue
		}
		name := r.text(g.FullName)
		group := model.StaffGroup{
			First:  first,
			Last:   last,
			Symbol: groupSymbol(g.Bracket.ID),
			// "group" is Finale's name for barlines drawn through the gap,
			// which is what says "play these together".
			Barlines: g.BarlineStyle == "group",
			Name:     name,
		}
		score.Groups = append(score.Groups, group)
		// The name belongs to the group, and the staves under it have
		// none: Finale writes "Pianoforte" once, against the brace, and
		// leaves both of its staves unlabelled. gscore draws the brace but
		// not the group's name, so the name goes on the first staff of the
		// group — once, where the brace begins — rather than on every
		// staff, which would say it twice down the side of one instrument.
		if name != "" && first < len(score.Parts) && score.Parts[first].Name == "" {
			score.Parts[first].Name = name
		}
	}
}

// groupSymbol turns Finale's bracket number into what is drawn down the left
// of the staves.
//
// Finale numbers its bracket shapes and a brace is one of them. The ones
// that matter are the curly brace an instrument of two staves gets and the
// straight bracket a family of instruments gets; everything else is drawn as
// a plain line, which says they belong together without saying how.
func groupSymbol(id int) model.GroupSymbol {
	switch id {
	case 3: // the piano brace
		return model.GroupBrace
	case 0:
		return model.GroupLine
	}
	return model.GroupBracket
}

/*
 * Drum staves and rhythm staves
 *
 * Neither is written in pitches. A drum staff puts an instrument on a line
 * and draws it with a notehead that says which instrument it is; a rhythm
 * staff draws a slash a beat and keeps the chord symbols above. Finale holds
 * both as something other than the notes — a percussion layout, a staff
 * style — and read as notes they are nonsense on the page: a wall of chords
 * where the guitar should have slashes, and drum beats a sixth above the
 * staff they belong on.
 */

// percussionOctave is how far a drum staff's numbers sit from the lines
// gscore draws them on.
//
// Finale and gscore both put a percussion staff's notes by diatonic step,
// and they disagree by an octave about where middle C is on one. Fixed
// against the file: Jingle Bells writes its cymbal as 11, and Finale prints
// it on the top line, which is where 4 lands here.
const percussionOctave = 7

// headFor is the notehead a drum note is drawn with.
//
// The layout rows say it as a character of the music font, and the font is
// SMuFL, so the number is the shape: a plain black head, a cross for a
// cymbal, a diamond for a bell.
func (r *reader) headFor(code, harmLev int) model.NoteheadKind {
	for _, p := range r.doc.Others.PercNotes {
		if p.Part != "" || p.NoteType != code || p.HarmLev != harmLev {
			continue
		}
		if head, ok := smuflHeads[p.Closed]; ok {
			return head
		}
	}
	return model.HeadNormal
}

// smuflHeads are the noteheads a percussion layout names, by their place in
// the music font.
var smuflHeads = map[int]model.NoteheadKind{
	0xE0A4: model.HeadNormal,   // noteheadBlack
	0xE0A3: model.HeadNormal,   // noteheadHalf
	0xE0A9: model.HeadX,        // noteheadXBlack
	0xE0A8: model.HeadX,        // noteheadXHalf
	0xE0B3: model.HeadCircleX,  // noteheadCircleX
	0xE0DB: model.HeadDiamond,  // noteheadDiamondBlack
	0xE0DA: model.HeadDiamond,  // noteheadDiamondHalf
	0xE0BE: model.HeadTriangle, // noteheadTriangleUpBlack
	0xE0BD: model.HeadTriangle, // noteheadTriangleUpHalf
}

// middleLine is the pitch that sits on a staff's middle line, which is where
// a rhythm slash is drawn whatever the music under it was.
var middleLine = map[model.Clef]model.Pitch{
	model.Treble:     {Step: model.StepB, Octave: 4},
	model.Bass:       {Step: model.StepD, Octave: 3},
	model.Alto:       {Step: model.StepC, Octave: 4},
	model.Tenor:      {Step: model.StepA, Octave: 3},
	model.Percussion: {Step: model.StepC, Octave: 4},
}

// slashStaves replaces the notes of a rhythm staff with the slashes Finale
// draws instead of them.
//
// A guitar part written this way holds a chord voicing that is never printed
// — Finale hides it and draws a slash a beat, with the chord symbol above —
// and reading those notes put a stack of five noteheads on every beat of
// Jingle Bells, dragging the staff's whole range down with it.
//
// The voicing is not kept. A slash is one note and a chord is five, and the
// bar cannot hold both; what is printed is what Finale prints, and the log
// says the harmony behind it was let go.
func (r *reader) slashStaves(score *model.Score, order []int) {
	slashes := map[int]bool{}
	for _, st := range r.doc.Others.Styles {
		if st.Part == "" && st.Alt == "slashBeats" {
			slashes[st.Cmper] = true
		}
	}
	place := map[int]int{}
	for i, staffID := range order {
		place[staffID] = i
	}

	for _, a := range r.doc.Others.StyleUsed {
		if a.Part != "" || !slashes[a.Style] {
			continue
		}
		at, ok := place[a.Staff]
		if !ok || at >= len(score.Parts) {
			continue
		}
		part := score.Parts[at]
		for i := a.StartMeas - 1; i < a.EndMeas && i < len(part.Measures); i++ {
			if i < 0 {
				continue
			}
			m := part.Measures[i]
			if len(m.Voices) == 0 {
				continue
			}
			beats := part.TimeAt(i)
			if beats.Beats <= 0 || beats.BeatValue <= 0 {
				continue
			}
			var out []model.Element
			for b := 0; b < beats.Beats; b++ {
				out = append(out, &model.Note{
					Pitch: middleLine[part.ClefAt(i)],
					Dur:   model.Duration{Num: 1, Den: beats.BeatValue},
					Head:  model.HeadSlash,
				})
			}
			// What stood above the bar belongs to the bar, not to the
			// notes that were in it: the chord symbol goes on the first
			// slash so it is still printed.
			if first := firstMarks(m); first != nil {
				if mk := marksOf(out[0]); mk != nil {
					mk.Chord = first.Chord
					mk.Dynamic = first.Dynamic
				}
			}
			m.Voices[0].Elements = out
			for _, v := range m.Voices[1:] {
				v.Elements = nil
			}
			r.skip("bars written as slashes, whose chord voicing is not kept")
		}
	}
}

// firstMarks is what is written on the first thing in a bar.
func firstMarks(m *model.Measure) *model.Marks {
	for _, v := range m.Voices {
		for _, el := range v.Elements {
			if mk := marksOf(el); mk != nil {
				return mk
			}
		}
	}
	return nil
}
