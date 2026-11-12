/*
 * json.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package gscore

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/stricaud/gscore/internal/model"
)

// The model is written through these types rather than tagged directly.
//
// Element is an interface, so a file has to say which kind each one is, and a
// saved file should not change shape every time a Go field is renamed. Keeping
// the wire format separate is what lets the model be refactored freely while
// old files still open.

type scoreDoc struct {
	Groups    []groupDoc `json:"groups,omitempty"`
	Title     string     `json:"title,omitempty"`
	Subtitle  string     `json:"subtitle,omitempty"`
	Composer  string     `json:"composer,omitempty"`
	Arranger  string     `json:"arranger,omitempty"`
	Copyright string     `json:"copyright,omitempty"`
	// How the words above are set, where somebody has set them in
	// anything. The words themselves stay where they are, so a file
	// written by a gscore that knew nothing about this still opens and
	// still says what the piece is called.
	TitleRuns    []spanDoc  `json:"titleRuns,omitempty"`
	SubtitleRuns []spanDoc  `json:"subtitleRuns,omitempty"`
	ComposerRuns []spanDoc  `json:"composerRuns,omitempty"`
	ArrangerRuns []spanDoc  `json:"arrangerRuns,omitempty"`
	Tempo        tempoDoc   `json:"tempo"`
	Parts        []partDoc  `json:"parts"`
	Cursor       *cursorDoc `json:"cursor,omitempty"`
	// MarkerStyle is what the marks in the rehearsal sequence are called:
	// letters, numbers, or the number of the bar each one stands on. One
	// decision about the whole score, and the companion of markerAuto on a
	// bar — the bar says it is a place in the sequence and this says what
	// the sequence calls its places. Omitted for letters, which is what a
	// score that has never been asked uses.
	MarkerStyle string `json:"markerStyle,omitempty"`
	// Scripts are the programs somebody wrote to make this music. Kept
	// because a piece built from a process is not fully written down by
	// its notes: the notes are the output and the process is the thought.
	//
	// Text, and only text. Nothing in a file runs on being opened — a
	// score that could execute code is a document that can attack the
	// person who opens it, which is the mistake Word made and spent a
	// decade undoing.
	Scripts []scriptDoc `json:"scripts,omitempty"`

	// Engraving is what this score wants done differently from the house
	// style, by the engraver's own names for those measurements. Only what
	// was changed: a file that wrote down every default would freeze the
	// day's defaults into itself and never see an improvement to them.
	// Description is a few words about the piece, for whoever is looking
	// at a shelf of them. Not printed on the page; the subtitle is the
	// one that is.
	Description string `json:"description,omitempty"`
	// Tags are what this piece is, in the words whoever keeps it would
	// look for it by. In the file, so that a score carries its own
	// description wherever it is copied to.
	Tags []string `json:"tags,omitempty"`

	Engraving map[string]float64 `json:"engraving,omitempty"`

	// Page is the paper the score is printed on. Omitted where nothing
	// was chosen, which is every score written before there was a page
	// setup and every new one since: those print on whatever the program
	// would do, and that is not a decision to freeze into a file.
	Page *pageDoc `json:"page,omitempty"`

	// Numbering is how often the bar numbers are printed and whether they
	// are boxed. Omitted where the score has not said, which is the
	// ordinary case.
	Numbering *numberingDoc `json:"numbering,omitempty"`

	// View is how the score was last being looked at: which drawing options
	// were on, and the colours the noteheads were read in.
	//
	// Not music — the same kind of thing as the cursor, and kept for the
	// same reason: it is work somebody did on this score. Omitted entirely
	// where nothing has been chosen, so that every file written before this
	// existed goes on being drawn however the reader's own settings say,
	// rather than being read as "every option off".
	View *viewDoc `json:"view,omitempty"`

	// Sheets are the parts: the separate documents this score is printed
	// as. No music in them — a part says which staves it prints and how it
	// wants them laid out, and the notes it shows are the score's own, which
	// is why a score and a part here can never come to disagree about one.
	//
	// In the file because somebody named them, sized them and put the page
	// turns where a player has time to turn, and that is work they would
	// not want to do again. Omitted where there are none, which is most
	// scores: a piano piece, a song and a lead sheet are their own part.
	Sheets []sheetDoc `json:"sheets,omitempty"`
}

// One printed part.
type sheetDoc struct {
	Name   string `json:"name,omitempty"`
	Staves []int  `json:"staves,omitempty"`
	// Engraving is what this part wants differently from the score, by the
	// engraver's own names, and StaffSpace how large it is drawn — which is
	// not an engraving measurement but the one thing every printed part
	// changes, being read at arm's length rather than at a desk.
	Engraving  map[string]float64 `json:"engraving,omitempty"`
	StaffSpace float64            `json:"staffSpace,omitempty"`
	Numbering  *numberingDoc      `json:"numbering,omitempty"`
	MultiRests bool               `json:"multiRests,omitempty"`
	// Breaks are the bars this part begins a system at. Its own, because
	// the score's fall wherever its fuller systems ran out of room.
	Breaks []int `json:"breaks,omitempty"`
	// Cues are the bars of somebody else's line printed small in this part
	// so the player can hear their entry coming. Which bars of which staff,
	// and no notes: the notes are fetched from that staff every time the
	// part is drawn, so a cue cannot go stale.
	Cues []cueDoc `json:"cues,omitempty"`
}

// One cue in a part.
type cueDoc struct {
	At    int    `json:"at"`
	Bars  int    `json:"bars"`
	From  int    `json:"from"`
	Label string `json:"label,omitempty"`
}

// How a score's bars are numbered.
type numberingDoc struct {
	Every int  `json:"every,omitempty"`
	Boxed bool `json:"boxed,omitempty"`
}

// How the score was last being looked at. See model.ViewState.
type viewDoc struct {
	Paginated      bool              `json:"paginated,omitempty"`
	StaffNames     bool              `json:"staffNames,omitempty"`
	MeasureNumbers bool              `json:"measureNumbers,omitempty"`
	MultiRests     bool              `json:"multiRests,omitempty"`
	TempoMark      bool              `json:"tempoMark,omitempty"`
	BeatMarks      bool              `json:"beatMarks,omitempty"`
	ConcertPitch   bool              `json:"concertPitch,omitempty"`
	ColourNotes    bool              `json:"colourNotes,omitempty"`
	NoteColours    map[string]string `json:"noteColours,omitempty"`
}

type scriptDoc struct {
	Name string `json:"name,omitempty"`
	Body string `json:"body"`
}

// Where the writing cursor was left. Not music: it is where somebody was
// working, kept so that opening a piece puts you back where you were.
type cursorDoc struct {
	Part    int `json:"part"`
	Measure int `json:"measure"`
	At      int `json:"at"`
	Step    int `json:"step"`
}

type groupDoc struct {
	First    int    `json:"first"`
	Last     int    `json:"last"`
	Symbol   string `json:"symbol,omitempty"`
	Barlines bool   `json:"barlines,omitempty"`
	Name     string `json:"name,omitempty"`
}

type tempoDoc struct {
	BPM  int    `json:"bpm"`
	Unit string `json:"unit"`
	Text string `json:"text,omitempty"`
}

type partDoc struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Instrument string `json:"instrument,omitempty"`
	Program    int    `json:"program,omitempty"`
	// Sound is what the staff is written for, named the way MusicXML's
	// standard instrument sounds name it. The program says what to play it
	// with and this says what it is, which a program number cannot: there
	// is one bassoon in General MIDI and no contrabassoon.
	Sound   string `json:"sound,omitempty"`
	Clef    string `json:"clef"`
	Fifths  int    `json:"fifths"`
	Mode    string `json:"mode"`
	Beats   int    `json:"beats"`
	Beat    int    `json:"beatValue"`
	TimeSym string `json:"timeSymbol,omitempty"`
	// Transpose is how far below what is printed this staff sounds, written
	// the way MusicXML writes it so the two agree.
	Transpose *transposeDoc `json:"transpose,omitempty"`
	// StaffLines is how many lines the staff is ruled with where it is not
	// the usual five: one, for a drum with no pitch.
	StaffLines int          `json:"staffLines,omitempty"`
	Measures   []measureDoc `json:"measures"`
}

type transposeDoc struct {
	Diatonic     int `json:"diatonic,omitempty"`
	Chromatic    int `json:"chromatic,omitempty"`
	OctaveChange int `json:"octaveChange,omitempty"`
}

type measureDoc struct {
	Barline string      `json:"barline,omitempty"`
	Voices  [][]elemDoc `json:"voices"`

	// A key or meter written here changes it from this bar on; absent means
	// no change, which is not the same as a change to C major.
	Fifths  *int   `json:"fifths,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Beats   int    `json:"beats,omitempty"`
	Beat    int    `json:"beatValue,omitempty"`
	TimeSym string `json:"timeSymbol,omitempty"`

	// Tempo is a new speed written over this bar, held until another says
	// otherwise. Absent means the bar carries on at whatever it was given.
	Tempo *tempoDoc `json:"tempo,omitempty"`

	// Ending is a volta bracket over this bar.
	Ending *endingDoc `json:"ending,omitempty"`
	// Marker is a rehearsal mark, and Repeat the sign that says to play the
	// bar before again.
	Marker string `json:"marker,omitempty"`
	// MarkerAuto says the bar is a place in the rehearsal sequence rather
	// than a mark written out as itself, so that what it is called is
	// worked out from where it falls. Without it in the file, every mark
	// made as "the next letter" was saved as a bar with nothing on it:
	// Marker is empty for those, the letter being the sequence's to say.
	MarkerAuto bool `json:"markerAuto,omitempty"`
	// NumberFrom restarts the bar numbering at this bar, at the number it
	// holds. A pointer, because restarting at 1 is a thing you can mean.
	NumberFrom *int `json:"numberFrom,omitempty"`
	// SystemBreak starts a new line of music at this bar.
	SystemBreak bool `json:"systemBreak,omitempty"`
	Repeat      int  `json:"repeat,omitempty"`

	// Sign is the segno, coda, Fine or To Coda written at this bar, and Jump
	// the direction written at the end of it. Both are stored by name rather
	// than by number, so that a file keeps its meaning if the order of the
	// constants ever changes.
	Sign string `json:"sign,omitempty"`
	Jump string `json:"jump,omitempty"`
}

type endingDoc struct {
	Numbers []int `json:"numbers"`
	Start   bool  `json:"start,omitempty"`
	Stop    bool  `json:"stop,omitempty"`
	Open    bool  `json:"open,omitempty"`
}

type elemDoc struct {
	Kind string `json:"kind"` // "note", "rest", "chord" or "symbol"
	Dur  string `json:"dur,omitempty"`

	Pitch *pitchDoc  `json:"pitch,omitempty"`
	Notes []pitchDoc `json:"notes,omitempty"`
	Acc   string     `json:"accidental,omitempty"`
	// Parens puts the accidental in brackets: a reminder rather than an
	// instruction, so it belongs to the sign and not to the pitch.
	Parens bool `json:"accidentalParens,omitempty"`
	// Forced keeps the sign whatever the rules of notation would print.
	// Without it, a score read from a format that states what was printed
	// — MusicXML does — would be respelled the next time it is opened,
	// and would not come back as it was saved.
	Forced bool `json:"accidentalForced,omitempty"`
	Hidden bool `json:"hidden,omitempty"`
	// Head is the notehead's shape and Percussion the drum it plays, both
	// of which a drum staff needs and neither of which the pitch can say.
	Head       string     `json:"notehead,omitempty"`
	Percussion int        `json:"percussion,omitempty"`
	Marks      *marksDoc  `json:"marks,omitempty"`
	Symbol     *symbolRef `json:"symbol,omitempty"`
}

type pitchDoc struct {
	Step       string `json:"step"`
	Octave     int    `json:"octave"`
	Alter      int    `json:"alter,omitempty"`
	Acc        string `json:"accidental,omitempty"`
	Parens     bool   `json:"accidentalParens,omitempty"`
	Forced     bool   `json:"accidentalForced,omitempty"`
	Head       string `json:"notehead,omitempty"`
	Percussion int    `json:"percussion,omitempty"`
}

type symbolRef struct {
	Name      string  `json:"name"`
	Placement string  `json:"placement,omitempty"`
	OffsetX   float64 `json:"offsetX,omitempty"`
	OffsetY   float64 `json:"offsetY,omitempty"`
}

type marksDoc struct {
	Articulations []string `json:"articulations,omitempty"`
	// Which side of the note each of those is written on, index for index
	// with them, where somebody has said. An empty string, or a list that
	// runs out, is left to the engraver.
	ArticulationPlace []string    `json:"articulationPlace,omitempty"`
	Techniques        []string    `json:"techniques,omitempty"`
	Cross             string      `json:"cross,omitempty"`
	Symbols           []symbolRef `json:"symbols,omitempty"`
	TieStart          bool        `json:"tieStart,omitempty"`
	TieStop           bool        `json:"tieStop,omitempty"`
	// Clef and Key are a change written in the middle of a bar, which
	// happens where a hand drops into the bass for a phrase or a scene
	// turns at a double bar inside the measure. They belong to the
	// element, so they are written with it: kept on the measure alone,
	// both came back at the head of the bar and the notes in front of
	// them were read against the wrong sign.
	Clef      string `json:"clef,omitempty"`
	KeyFifths *int   `json:"keyFifths,omitempty"`
	KeyMode   string `json:"keyMode,omitempty"`
	// Grace are the little notes crushed in before this one. They take no
	// time out of the bar, so they are not elements of their own and have
	// to be written here or not at all — and not at all is what happened
	// to every one of them until now.
	Grace        []elemDoc   `json:"grace,omitempty"`
	GraceSlashed bool        `json:"graceSlashed,omitempty"`
	SlurStart    slurEnds    `json:"slurStart,omitempty"`
	SlurStop     slurEnds    `json:"slurStop,omitempty"`
	Tuplet       *tupletDoc  `json:"tuplet,omitempty"`
	Stem         string      `json:"stem,omitempty"`
	Dynamic      string      `json:"dynamic,omitempty"`
	Hairpin      *hairpinDoc `json:"hairpin,omitempty"`
	Octave       *octaveDoc  `json:"octave,omitempty"`
	Trill        *trillDoc   `json:"trill,omitempty"`
	Tremolo      *tremoloDoc `json:"tremolo,omitempty"`
	Lyrics       []lyricDoc  `json:"lyrics,omitempty"`
	Beam         string      `json:"beam,omitempty"`
	Chord        *chordDoc   `json:"chord,omitempty"`
	Texts        []textDoc   `json:"texts,omitempty"`
	// Arpeggio rolls the chord, Break is the breath or pause after the
	// note, Fingering which fingers play it, Glissando the slide to the
	// next, Pedal the sustaining pedal, and Small draws it at cue size.
	Arpeggio  *arpeggioDoc `json:"arpeggio,omitempty"`
	Break     string       `json:"break,omitempty"`
	Fingering []string     `json:"fingering,omitempty"`
	Glissando *glissDoc    `json:"glissando,omitempty"`
	Pedal     *pedalDoc    `json:"pedal,omitempty"`
	Small     bool         `json:"small,omitempty"`
}

type arpeggioDoc struct {
	Down bool `json:"down,omitempty"`
}

type glissDoc struct {
	Start bool   `json:"start,omitempty"`
	Stop  bool   `json:"stop,omitempty"`
	Wavy  bool   `json:"wavy,omitempty"`
	Text  string `json:"text,omitempty"`
}

type pedalDoc struct {
	Start  bool `json:"start,omitempty"`
	Stop   bool `json:"stop,omitempty"`
	Change bool `json:"change,omitempty"`
	Text   bool `json:"text,omitempty"`
	Half   bool `json:"half,omitempty"`
}

type chordDoc struct {
	Root    string    `json:"root,omitempty"`
	Quality string    `json:"quality,omitempty"`
	Bass    string    `json:"bass,omitempty"`
	Text    string    `json:"text,omitempty"`
	Frame   *frameDoc `json:"frame,omitempty"`
}

type frameDoc struct {
	Frets     []int `json:"frets"`
	Fingers   []int `json:"fingers,omitempty"`
	FirstFret int   `json:"firstFret,omitempty"`
	Barre     int   `json:"barre,omitempty"`
	BarreFrom int   `json:"barreFrom,omitempty"`
	BarreTo   int   `json:"barreTo,omitempty"`
}

type textDoc struct {
	Text      string  `json:"text"`
	Style     string  `json:"style,omitempty"`
	Placement string  `json:"placement,omitempty"`
	OffsetX   float64 `json:"offsetX,omitempty"`
	OffsetY   float64 `json:"offsetY,omitempty"`
	Italic    bool    `json:"italic,omitempty"`
	Bold      bool    `json:"bold,omitempty"`
	// Runs is how the words are set, where somebody has set them in
	// anything. The words themselves stay in Text.
	Runs []spanDoc `json:"runs,omitempty"`
}

type hairpinDoc struct {
	Kind  string `json:"kind"` // "crescendo" or "diminuendo"
	Start bool   `json:"start,omitempty"`
	Stop  bool   `json:"stop,omitempty"`
	// Alone says the wedge begins and ends on this element, which is not
	// the same as carrying both ends: see model.Hairpin.
	Alone bool `json:"alone,omitempty"`
}

type octaveDoc struct {
	Amount int  `json:"amount"` // octaves, signed: 1 is 8va, -1 is 8vb
	Start  bool `json:"start,omitempty"`
	Stop   bool `json:"stop,omitempty"`
}

// A trill is a spanner: "tr" where it begins and a wavy line to where it
// stops. Both ends on one note is a trill on that note alone.
// A tremolo is the slashes and whether it is measured at all, which is all
// there is to one: how fast it goes follows from the number of slashes.
type tremoloDoc struct {
	Strokes int  `json:"strokes,omitempty"`
	Buzz    bool `json:"buzz,omitempty"`
}

type trillDoc struct {
	Start bool `json:"start,omitempty"`
	Stop  bool `json:"stop,omitempty"`
}

type lyricDoc struct {
	Text     string `json:"text"`
	Verse    int    `json:"verse,omitempty"`
	Syllabic string `json:"syllabic,omitempty"` // "", "begin", "middle", "end"
	Extend   bool   `json:"extend,omitempty"`
}

type tupletDoc struct {
	Actual int  `json:"actual"`
	Normal int  `json:"normal"`
	Start  bool `json:"start,omitempty"`
	Stop   bool `json:"stop,omitempty"`
}

/*
 * Names. Written out in full so a file reads sensibly and does not depend on
 * the numeric value of a Go constant.
 */

var clefNames = map[model.Clef]string{
	model.Treble: "treble", model.Bass: "bass", model.Alto: "alto",
	model.Tenor: "tenor", model.Percussion: "percussion",
}
var clefValues = invert(clefNames)

var modeNames = map[model.Mode]string{
	model.Major: "major", model.Minor: "minor", model.Dorian: "dorian",
	model.Phrygian: "phrygian", model.Lydian: "lydian",
	model.Mixolydian: "mixolydian", model.Locrian: "locrian",
}
var modeValues = invert(modeNames)

var barNames = map[model.BarlineStyle]string{
	model.BarSingle: "single", model.BarDouble: "double", model.BarFinal: "final",
	model.BarRepeatStart: "repeat-start", model.BarRepeatEnd: "repeat-end",
	model.BarRepeatBoth: "repeat-both", model.BarNone: "none",
}
var barValues = invert(barNames)

var timeSymNames = map[model.TimeSymbol]string{
	model.TimeNumeric: "", model.TimeCommon: "common", model.TimeCut: "cut",
}
var timeSymValues = invert(timeSymNames)

var accNames = map[model.Accidental]string{
	model.NoAccidental: "", model.Natural: "natural", model.Sharp: "sharp",
	model.Flat: "flat", model.DoubleSharp: "double-sharp", model.DoubleFlat: "double-flat",
}
var accValues = invert(accNames)

var articNames = map[model.Articulation]string{
	model.Staccato: "staccato", model.Accent: "accent", model.Tenuto: "tenuto",
	model.Marcato: "marcato", model.Fermata: "fermata",
	model.UpBow: "up-bow", model.DownBow: "down-bow",
}
var articValues = invert(articNames)

var stemNames = map[model.Stem]string{
	model.StemAuto: "", model.StemUp: "up", model.StemDown: "down",
}
var stemValues = invert(stemNames)

var placementNames = map[model.Placement]string{
	model.PlacementAuto: "", model.PlacementAbove: "above", model.PlacementBelow: "below",
}
var placementValues = invert(placementNames)

func invert[K comparable, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

func durString(d model.Duration) string {
	r := d.Reduced()
	return fmt.Sprintf("%d/%d", r.Num, r.Den)
}

func parseDur(s string) (model.Duration, error) {
	var num, den int
	if _, err := fmt.Sscanf(s, "%d/%d", &num, &den); err != nil || den == 0 {
		return model.Duration{}, fmt.Errorf("gscore: bad duration %q", s)
	}
	return model.Duration{Num: num, Den: den}, nil
}

/*
 * Encoding
 */

func encodeScore(s *model.Score) *scoreDoc {
	doc := &scoreDoc{
		Title:        s.Identity.Title,
		Subtitle:     s.Identity.Subtitle,
		Composer:     s.Identity.Composer,
		Arranger:     s.Identity.Arranger,
		Copyright:    s.Identity.Copyright,
		TitleRuns:    spansToDoc(s.Identity.TitleRuns),
		SubtitleRuns: spansToDoc(s.Identity.SubtitleRuns),
		ComposerRuns: spansToDoc(s.Identity.ComposerRuns),
		ArrangerRuns: spansToDoc(s.Identity.ArrangerRuns),
		Cursor:       cursorToDoc(s.Cursor),
		Tempo: tempoDoc{
			BPM: s.Tempo.BPM, Unit: durString(s.Tempo.Unit), Text: s.Tempo.Text,
		},
	}
	for _, g := range s.Groups {
		doc.Groups = append(doc.Groups, groupDoc{
			First: g.First, Last: g.Last, Symbol: g.Symbol.String(),
			Barlines: g.Barlines, Name: g.Name,
		})
	}
	if s.Markers != model.MarkerLetters {
		doc.MarkerStyle = s.Markers.String()
	}
	doc.Description = s.Description
	if len(s.Tags) > 0 {
		doc.Tags = append([]string(nil), s.Tags...)
	}
	if s.Numbering != (model.MeasureNumbering{}) {
		doc.Numbering = &numberingDoc{
			Every: s.Numbering.Every, Boxed: s.Numbering.Boxed,
		}
	}
	if v := s.View; v != nil {
		d := &viewDoc{
			Paginated: v.Paginated, StaffNames: v.StaffNames,
			MeasureNumbers: v.MeasureNumbers, MultiRests: v.MultiRests,
			TempoMark: v.TempoMark, BeatMarks: v.BeatMarks,
			ConcertPitch: v.ConcertPitch, ColourNotes: v.ColourNotes,
		}
		// Copied rather than shared: the score goes on being edited after
		// it has been written out, and a map handed to the encoder is a map
		// the editor can still change underneath it.
		if len(v.NoteColours) > 0 {
			d.NoteColours = make(map[string]string, len(v.NoteColours))
			for k, c := range v.NoteColours {
				d.NoteColours[k] = c
			}
		}
		doc.View = d
	}
	for _, sh := range s.Sheets {
		d := sheetDoc{
			Name: sh.Name, StaffSpace: sh.StaffSpace,
			MultiRests: sh.MultiRests,
		}
		if sh.Staves != nil {
			d.Staves = append([]int(nil), sh.Staves...)
		}
		if sh.Breaks != nil {
			d.Breaks = append([]int(nil), sh.Breaks...)
		}
		for _, c := range sh.Cues {
			d.Cues = append(d.Cues, cueDoc{
				At: c.At, Bars: c.Bars, From: c.From, Label: c.Label,
			})
		}
		if len(sh.Engraving) > 0 {
			d.Engraving = make(map[string]float64, len(sh.Engraving))
			for k, v := range sh.Engraving {
				d.Engraving[k] = v
			}
		}
		if sh.Numbering != (model.MeasureNumbering{}) {
			d.Numbering = &numberingDoc{
				Every: sh.Numbering.Every, Boxed: sh.Numbering.Boxed,
			}
		}
		doc.Sheets = append(doc.Sheets, d)
	}
	if len(s.Engraving) > 0 {
		doc.Engraving = make(map[string]float64, len(s.Engraving))
		for k, v := range s.Engraving {
			doc.Engraving[k] = v
		}
	}
	if !s.Page.IsZero() {
		doc.Page = &pageDoc{
			Paper: s.Page.Paper, Landscape: s.Page.Landscape,
			CustomWidth: s.Page.CustomWidth, CustomHeight: s.Page.CustomHeight,
			MarginLeft: s.Page.MarginLeft, MarginRight: s.Page.MarginRight,
			MarginTop: s.Page.MarginTop, MarginBottom: s.Page.MarginBottom,
			StaffSpace: s.Page.StaffSpace,
		}
	}
	for _, sc := range s.Scripts {
		doc.Scripts = append(doc.Scripts, scriptDoc{Name: sc.Name, Body: sc.Body})
	}
	for _, p := range s.Parts {
		pd := partDoc{
			ID: p.ID, Name: p.Name,
			Instrument: p.Instrument.String(),
			Program:    int(p.Instrument),
			Sound:      p.Sound,
			Clef:       clefNames[p.Clef],
			Fifths:     p.Key.Fifths,
			Mode:       modeNames[p.Key.Mode],
			Beats:      p.Time.Beats,
			Beat:       p.Time.BeatValue,
			TimeSym:    timeSymNames[p.Time.Symbol],
			StaffLines: p.StaffLines,
		}
		if !p.Transpose.IsZero() {
			pd.Transpose = &transposeDoc{
				Diatonic: p.Transpose.Diatonic, Chromatic: p.Transpose.Chromatic,
				OctaveChange: p.Transpose.OctaveChange,
			}
		}
		for _, m := range p.Measures {
			md := measureDoc{Barline: barNames[m.RightBarline]}
			if m.Key != nil {
				f := m.Key.Fifths
				md.Fifths, md.Mode = &f, modeNames[m.Key.Mode]
			}
			if m.Time != nil {
				md.Beats, md.Beat = m.Time.Beats, m.Time.BeatValue
				md.TimeSym = timeSymNames[m.Time.Symbol]
			}
			if m.Tempo != nil {
				md.Tempo = &tempoDoc{
					BPM: m.Tempo.BPM, Unit: durString(m.Tempo.Unit),
					Text: m.Tempo.Text,
				}
			}
			if m.Ending != nil {
				md.Ending = &endingDoc{
					Numbers: m.Ending.Numbers, Start: m.Ending.Start,
					Stop: m.Ending.Stop, Open: m.Ending.Open,
				}
			}
			md.Marker, md.Repeat = m.Marker, m.Repeat
			md.MarkerAuto = m.MarkerAuto
			md.NumberFrom = m.NumberFrom
			md.SystemBreak = m.SystemBreak
			if m.Sign != model.NoSign {
				md.Sign = m.Sign.String()
			}
			if m.Jump != nil {
				md.Jump = m.Jump.Name()
			}
			for _, v := range m.Voices {
				var ed []elemDoc
				for _, el := range v.Elements {
					ed = append(ed, encodeElement(el))
				}
				md.Voices = append(md.Voices, ed)
			}
			pd.Measures = append(pd.Measures, md)
		}
		doc.Parts = append(doc.Parts, pd)
	}
	return doc
}

func encodeElement(el model.Element) elemDoc {
	switch e := el.(type) {
	case *model.Note:
		return elemDoc{
			Kind: "note", Dur: durString(e.Dur),
			Pitch: &pitchDoc{
				Step: e.Pitch.Step.String(), Octave: e.Pitch.Octave, Alter: e.Pitch.Alter,
			},
			Acc:        accNames[e.Accidental],
			Parens:     e.Parens,
			Forced:     e.Forced,
			Head:       headNames[e.Head],
			Percussion: e.Percussion,
			Marks:      encodeMarks(&e.Marks),
		}
	case *model.Rest:
		return elemDoc{
			Kind: "rest", Dur: durString(e.Dur), Hidden: e.Hidden,
			Marks: encodeMarks(&e.Marks),
		}
	case *model.Chord:
		d := elemDoc{Kind: "chord", Dur: durString(e.Dur), Marks: encodeMarks(&e.Marks)}
		for _, n := range e.Notes {
			d.Notes = append(d.Notes, pitchDoc{
				Step: n.Pitch.Step.String(), Octave: n.Pitch.Octave,
				Alter: n.Pitch.Alter, Acc: accNames[n.Accidental],
				Parens: n.Parens, Forced: n.Forced,
				Head: headNames[n.Head], Percussion: n.Percussion,
			})
		}
		return d
	case *model.SymbolElement:
		return elemDoc{Kind: "symbol", Symbol: &symbolRef{
			Name: e.Symbol.Name, Placement: placementNames[e.Symbol.Placement],
			OffsetX: e.Symbol.OffsetX, OffsetY: e.Symbol.OffsetY,
		}}
	}
	return elemDoc{Kind: "rest", Dur: "0/1"}
}

// pageDoc is the paper, in millimetres, as the file writes it.
type pageDoc struct {
	Paper        string  `json:"paper,omitempty"`
	CustomWidth  float64 `json:"customWidth,omitempty"`
	CustomHeight float64 `json:"customHeight,omitempty"`
	Landscape    bool    `json:"landscape,omitempty"`
	MarginLeft   float64 `json:"marginLeft,omitempty"`
	MarginRight  float64 `json:"marginRight,omitempty"`
	MarginTop    float64 `json:"marginTop,omitempty"`
	MarginBottom float64 `json:"marginBottom,omitempty"`
	StaffSpace   float64 `json:"staffSpace,omitempty"`
}

func encodeMarks(m *model.Marks) *marksDoc {
	if len(m.Articulations) == 0 && len(m.Techniques) == 0 &&
		m.Cross == model.OnOwnStaff &&
		len(m.Symbols) == 0 && m.Tuplet == nil &&
		!m.TieStart && !m.TieStop && m.SlurStart == 0 && m.SlurStop == 0 &&
		m.Stem == model.StemAuto && m.Dynamic == model.DynamicNone &&
		m.Hairpin == nil && m.Octave == nil && len(m.Lyrics) == 0 &&
		m.Tremolo == nil && m.Trill == nil &&
		m.Beam == model.BeamAuto && m.Chord == nil && len(m.Texts) == 0 &&
		m.Arpeggio == nil && m.Break == model.NoBreath &&
		len(m.Fingering) == 0 && m.Glissando == nil && m.Pedal == nil &&
		m.Clef == nil && m.Key == nil && m.Grace == nil &&
		!m.Small {
		return nil
	}
	out := &marksDoc{
		TieStart: m.TieStart, TieStop: m.TieStop,
		SlurStart: slurEnds(m.SlurStart), SlurStop: slurEnds(m.SlurStop),
		Stem:    stemNames[m.Stem],
		Dynamic: m.Dynamic.String(),
	}
	if m.Clef != nil {
		out.Clef = clefNames[*m.Clef]
	}
	if m.Key != nil {
		f := m.Key.Fifths
		out.KeyFifths, out.KeyMode = &f, modeNames[m.Key.Mode]
	}
	if m.Grace != nil {
		out.GraceSlashed = m.Grace.Slashed
		for _, n := range m.Grace.Notes {
			out.Grace = append(out.Grace, encodeElement(n))
		}
	}
	for i, a := range m.Articulations {
		out.Articulations = append(out.Articulations, articNames[a])
		// Only where somebody said, and only as far as the last one who
		// did: a list of empty strings is noise in a file.
		if where := m.ArticulationPlacement(i); where != model.PlacementAuto {
			for len(out.ArticulationPlace) < i {
				out.ArticulationPlace = append(out.ArticulationPlace, "")
			}
			out.ArticulationPlace = append(out.ArticulationPlace,
				placementNames[where])
		}
	}
	for _, t := range m.Techniques {
		out.Techniques = append(out.Techniques, t.String())
	}
	if m.Cross != model.OnOwnStaff {
		out.Cross = m.Cross.String()
	}
	for _, s := range m.Symbols {
		out.Symbols = append(out.Symbols, symbolRef{
			Name: s.Name, Placement: placementNames[s.Placement],
			OffsetX: s.OffsetX, OffsetY: s.OffsetY,
		})
	}
	if m.Tuplet != nil {
		out.Tuplet = &tupletDoc{
			Actual: m.Tuplet.Actual, Normal: m.Tuplet.Normal,
			Start: m.Tuplet.Start, Stop: m.Tuplet.Stop,
		}
	}
	if m.Hairpin != nil {
		out.Hairpin = &hairpinDoc{
			Kind:  m.Hairpin.Kind.String(),
			Start: m.Hairpin.Start, Stop: m.Hairpin.Stop,
			Alone: m.Hairpin.Alone,
		}
	}
	if m.Octave != nil {
		out.Octave = &octaveDoc{
			Amount: m.Octave.Amount, Start: m.Octave.Start, Stop: m.Octave.Stop,
		}
	}
	if m.Arpeggio != nil {
		out.Arpeggio = &arpeggioDoc{Down: m.Arpeggio.Down}
	}
	if m.Break != model.NoBreath {
		out.Break = m.Break.String()
	}
	if len(m.Fingering) > 0 {
		out.Fingering = append([]string(nil), m.Fingering...)
	}
	if g := m.Glissando; g != nil {
		out.Glissando = &glissDoc{
			Start: g.Start, Stop: g.Stop, Wavy: g.Wavy, Text: g.Text,
		}
	}
	if pd := m.Pedal; pd != nil {
		out.Pedal = &pedalDoc{
			Start: pd.Start, Stop: pd.Stop, Change: pd.Change,
			Text: pd.Text, Half: pd.Half,
		}
	}
	out.Small = m.Small
	if m.Trill != nil {
		out.Trill = &trillDoc{Start: m.Trill.Start, Stop: m.Trill.Stop}
	}
	if m.Tremolo != nil {
		out.Tremolo = &tremoloDoc{
			Strokes: m.Tremolo.Strokes, Buzz: m.Tremolo.Buzz,
		}
	}
	for _, ly := range m.Lyrics {
		out.Lyrics = append(out.Lyrics, lyricDoc{
			Text: ly.Text, Verse: ly.Verse,
			Syllabic: syllabicNames[ly.Syllabic], Extend: ly.Extend,
		})
	}
	if m.Beam != model.BeamAuto {
		out.Beam = m.Beam.String()
	}
	if c := m.Chord; c != nil {
		cd := &chordDoc{Root: c.Root, Quality: c.Quality, Bass: c.Bass, Text: c.Text}
		if f := c.Frame; f != nil {
			cd.Frame = &frameDoc{
				Frets: f.Frets, Fingers: f.Fingers, FirstFret: f.FirstFret,
				Barre: f.Barre, BarreFrom: f.BarreFrom, BarreTo: f.BarreTo,
			}
		}
		out.Chord = cd
	}
	for _, t := range m.Texts {
		out.Texts = append(out.Texts, textDoc{
			Text: t.Text, Style: t.Style.String(),
			Placement: placementNames[t.Placement],
			OffsetX:   t.OffsetX, OffsetY: t.OffsetY,
			Italic: t.Italic, Bold: t.Bold,
			Runs: spansToDoc(t.Runs),
		})
	}
	return out
}

var headNames = map[model.NoteheadKind]string{
	model.HeadNormal: "", model.HeadX: "x", model.HeadCircleX: "circle-x",
	model.HeadTriangle: "triangle", model.HeadDiamond: "diamond",
	model.HeadSlash: "slash",
}
var headValues = invert(headNames)

var groupSymbolNames = map[model.GroupSymbol]string{
	model.GroupBracket: "bracket", model.GroupBrace: "brace",
	model.GroupLine: "line",
}
var groupSymbolValues = invert(groupSymbolNames)

var syllabicNames = map[model.Syllabic]string{
	model.SyllableSingle: "", model.SyllableBegin: "begin",
	model.SyllableMiddle: "middle", model.SyllableEnd: "end",
}
var syllabicValues = invert(syllabicNames)

/*
 * Decoding
 */

func decodeScore(doc *scoreDoc) (*model.Score, error) {
	s := &model.Score{}
	s.Identity = model.Identity{
		Title: doc.Title, Subtitle: doc.Subtitle,
		Composer: doc.Composer, Arranger: doc.Arranger,
		Copyright:    doc.Copyright,
		TitleRuns:    spansFromDoc(doc.TitleRuns),
		SubtitleRuns: spansFromDoc(doc.SubtitleRuns),
		ComposerRuns: spansFromDoc(doc.ComposerRuns),
		ArrangerRuns: spansFromDoc(doc.ArrangerRuns),
	}
	unit := model.Duration{Num: 1, Den: 4}
	if doc.Tempo.Unit != "" {
		if d, err := parseDur(doc.Tempo.Unit); err == nil {
			unit = d
		}
	}
	bpm := doc.Tempo.BPM
	if bpm <= 0 {
		bpm = 120
	}
	s.Tempo = model.Tempo{BPM: bpm, Unit: unit, Text: doc.Tempo.Text}
	if c := doc.Cursor; c != nil {
		s.Cursor = &model.Cursor{
			Part: c.Part, Measure: c.Measure, At: c.At, Step: c.Step,
		}
	}

	// An unknown name is read as letters rather than refused: a file is
	// worth opening even where one word in it came from a later gscore.
	if style, ok := model.MarkerStyleFromName(doc.MarkerStyle); ok {
		s.Markers = style
	}

	s.Description = doc.Description
	if len(doc.Tags) > 0 {
		s.Tags = append([]string(nil), doc.Tags...)
	}
	for _, d := range doc.Sheets {
		sh := model.Sheet{
			Name: d.Name, StaffSpace: d.StaffSpace,
			MultiRests: d.MultiRests,
		}
		if d.Staves != nil {
			sh.Staves = append([]int(nil), d.Staves...)
		}
		if d.Breaks != nil {
			sh.Breaks = append([]int(nil), d.Breaks...)
		}
		for _, c := range d.Cues {
			sh.Cues = append(sh.Cues, model.Cue{
				At: c.At, Bars: c.Bars, From: c.From, Label: c.Label,
			})
		}
		if len(d.Engraving) > 0 {
			sh.Engraving = make(map[string]float64, len(d.Engraving))
			for k, v := range d.Engraving {
				sh.Engraving[k] = v
			}
		}
		if d.Numbering != nil {
			sh.Numbering = model.MeasureNumbering{
				Every: d.Numbering.Every, Boxed: d.Numbering.Boxed,
			}
		}
		s.Sheets = append(s.Sheets, sh)
	}
	if doc.Numbering != nil {
		s.Numbering = model.MeasureNumbering{
			Every: doc.Numbering.Every, Boxed: doc.Numbering.Boxed,
		}
	}
	if v := doc.View; v != nil {
		st := &model.ViewState{
			Paginated: v.Paginated, StaffNames: v.StaffNames,
			MeasureNumbers: v.MeasureNumbers, MultiRests: v.MultiRests,
			TempoMark: v.TempoMark, BeatMarks: v.BeatMarks,
			ConcertPitch: v.ConcertPitch, ColourNotes: v.ColourNotes,
		}
		if len(v.NoteColours) > 0 {
			st.NoteColours = make(map[string]string, len(v.NoteColours))
			for k, c := range v.NoteColours {
				st.NoteColours[k] = c
			}
		}
		s.View = st
	}
	if p := doc.Page; p != nil {
		s.Page = model.Page{
			Paper: p.Paper, Landscape: p.Landscape,
			CustomWidth: p.CustomWidth, CustomHeight: p.CustomHeight,
			MarginLeft: p.MarginLeft, MarginRight: p.MarginRight,
			MarginTop: p.MarginTop, MarginBottom: p.MarginBottom,
			StaffSpace: p.StaffSpace,
		}
	}
	if len(doc.Engraving) > 0 {
		s.Engraving = make(map[string]float64, len(doc.Engraving))
		for k, v := range doc.Engraving {
			s.Engraving[k] = v
		}
	}
	for _, sd := range doc.Scripts {
		s.Scripts = append(s.Scripts, model.Script{Name: sd.Name, Body: sd.Body})
	}
	for _, pd := range doc.Parts {
		p := &model.Part{
			ID: pd.ID, Name: pd.Name,
			Instrument: model.Instrument(pd.Program),
			Sound:      pd.Sound,
			Clef:       clefValues[pd.Clef],
			Key:        model.KeySignature{Fifths: pd.Fifths, Mode: modeValues[pd.Mode]},
			Time: model.TimeSignature{
				Beats: pd.Beats, BeatValue: pd.Beat, Symbol: timeSymValues[pd.TimeSym],
			},
			StaffLines: pd.StaffLines,
		}
		if p.Time.BeatValue == 0 {
			p.Time = model.TimeSignature{Beats: 4, BeatValue: 4}
		}
		if t := pd.Transpose; t != nil {
			p.Transpose = model.Transposition{
				Diatonic: t.Diatonic, Chromatic: t.Chromatic,
				OctaveChange: t.OctaveChange,
			}
		}
		for _, md := range pd.Measures {
			m := &model.Measure{RightBarline: barValues[md.Barline]}
			if md.Fifths != nil {
				m.Key = &model.KeySignature{Fifths: *md.Fifths, Mode: modeValues[md.Mode]}
			}
			if md.Beat > 0 {
				m.Time = &model.TimeSignature{
					Beats: md.Beats, BeatValue: md.Beat,
					Symbol: timeSymValues[md.TimeSym],
				}
			}
			if td := md.Tempo; td != nil {
				unit := model.Duration{Num: 1, Den: 4}
				if td.Unit != "" {
					if d, err := parseDur(td.Unit); err == nil {
						unit = d
					}
				}
				m.Tempo = &model.Tempo{BPM: td.BPM, Unit: unit, Text: td.Text}
			}
			if md.Ending != nil {
				m.Ending = &model.Ending{
					Numbers: md.Ending.Numbers, Start: md.Ending.Start,
					Stop: md.Ending.Stop, Open: md.Ending.Open,
				}
			}
			m.Marker, m.Repeat = md.Marker, md.Repeat
			m.MarkerAuto = md.MarkerAuto
			m.NumberFrom = md.NumberFrom
			m.SystemBreak = md.SystemBreak
			if sign, ok := model.SignFromName(md.Sign); ok {
				m.Sign = sign
			}
			if jump, ok := model.JumpFromName(md.Jump); ok {
				m.Jump = jump
			}
			for _, vd := range md.Voices {
				v := &model.Voice{}
				for _, ed := range vd {
					el, err := decodeElement(ed)
					if err != nil {
						return nil, err
					}
					if el != nil {
						v.Elements = append(v.Elements, el)
					}
				}
				m.Voices = append(m.Voices, v)
			}
			if len(m.Voices) == 0 {
				m.Voices = []*model.Voice{{}}
			}
			p.Measures = append(p.Measures, m)
		}
		s.Parts = append(s.Parts, p)
	}
	if len(s.Parts) == 0 {
		return nil, fmt.Errorf("gscore: the score has no parts")
	}
	for _, gd := range doc.Groups {
		if gd.First < 0 || gd.Last >= len(s.Parts) || gd.Last <= gd.First {
			continue // a group naming staves that are not there is dropped
		}
		s.Groups = append(s.Groups, model.StaffGroup{
			First: gd.First, Last: gd.Last,
			Symbol:   groupSymbolValues[gd.Symbol],
			Barlines: gd.Barlines, Name: gd.Name,
		})
	}
	return s, nil
}

func decodeElement(ed elemDoc) (model.Element, error) {
	dur := model.Duration{Num: 0, Den: 1}
	if ed.Dur != "" {
		d, err := parseDur(ed.Dur)
		if err != nil {
			return nil, err
		}
		dur = d
	}

	switch ed.Kind {
	case "note":
		if ed.Pitch == nil {
			return nil, fmt.Errorf("gscore: a note with no pitch")
		}
		n := &model.Note{
			Dur: dur, Pitch: decodePitch(*ed.Pitch),
			Accidental: accValues[ed.Acc], Parens: ed.Parens,
			Forced: ed.Forced,
			Head:   headValues[ed.Head], Percussion: ed.Percussion,
		}
		decodeMarks(ed.Marks, &n.Marks)
		return n, nil
	case "rest":
		r := &model.Rest{Dur: dur, Hidden: ed.Hidden}
		decodeMarks(ed.Marks, &r.Marks)
		return r, nil
	case "chord":
		c := &model.Chord{Dur: dur}
		for _, pd := range ed.Notes {
			c.Notes = append(c.Notes, &model.Note{
				Pitch: decodePitch(pd), Dur: dur, Accidental: accValues[pd.Acc],
				Parens: pd.Parens, Forced: pd.Forced,
				Head: headValues[pd.Head], Percussion: pd.Percussion,
			})
		}
		decodeMarks(ed.Marks, &c.Marks)
		return c, nil
	case "symbol":
		if ed.Symbol == nil {
			return nil, nil
		}
		return &model.SymbolElement{Symbol: model.SymbolMark{
			Name: ed.Symbol.Name, Placement: placementValues[ed.Symbol.Placement],
			OffsetX: ed.Symbol.OffsetX, OffsetY: ed.Symbol.OffsetY,
		}}, nil
	}
	return nil, fmt.Errorf("gscore: unknown element kind %q", ed.Kind)
}

func decodePitch(pd pitchDoc) model.Pitch {
	step, _ := model.StepFromLetter(firstByte(pd.Step))
	return model.Pitch{Step: step, Octave: pd.Octave, Alter: pd.Alter}
}

func firstByte(s string) byte {
	if s == "" {
		return 'C'
	}
	return s[0]
}

func decodeMarks(md *marksDoc, m *model.Marks) {
	if md == nil {
		return
	}
	m.TieStart, m.TieStop = md.TieStart, md.TieStop
	m.SlurStart, m.SlurStop = int(md.SlurStart), int(md.SlurStop)
	if md.Clef != "" {
		if c, ok := clefValues[md.Clef]; ok {
			m.Clef = &c
		}
	}
	if md.KeyFifths != nil {
		m.Key = &model.KeySignature{
			Fifths: *md.KeyFifths, Mode: modeValues[md.KeyMode],
		}
	}
	if len(md.Grace) > 0 {
		g := &model.GraceGroup{Slashed: md.GraceSlashed}
		for _, ed := range md.Grace {
			el, err := decodeElement(ed)
			if err != nil {
				continue
			}
			if n, ok := el.(*model.Note); ok {
				g.Notes = append(g.Notes, n)
			}
		}
		if len(g.Notes) > 0 {
			m.Grace = g
		}
	}
	m.Stem = stemValues[md.Stem]
	if d, ok := model.DynamicFromName(md.Dynamic); ok {
		m.Dynamic = d
	}
	for i, a := range md.Articulations {
		v, ok := articValues[a]
		if !ok {
			continue
		}
		m.AddArticulation(v)
		if i < len(md.ArticulationPlace) {
			if where := placementValues[md.ArticulationPlace[i]]; where != model.PlacementAuto {
				m.PlaceArticulation(v, where)
			}
		}
	}
	// A technique the file names and this build does not know is dropped
	// rather than refused: a score written by a later gscore should still
	// open, minus the one mark. The same rule the articulations above are
	// read by, and for the same reason.
	for _, t := range md.Techniques {
		if v, ok := model.TechniqueFromName(t); ok {
			m.AddTechnique(v)
		}
	}
	if c, ok := model.CrossStaffFromName(md.Cross); ok {
		m.Cross = c
	}
	for _, s := range md.Symbols {
		m.Symbols = append(m.Symbols, model.SymbolMark{
			Name: s.Name, Placement: placementValues[s.Placement],
			OffsetX: s.OffsetX, OffsetY: s.OffsetY,
		})
	}
	if md.Tuplet != nil && md.Tuplet.Actual > 0 && md.Tuplet.Normal > 0 {
		m.Tuplet = &model.Tuplet{
			Actual: md.Tuplet.Actual, Normal: md.Tuplet.Normal,
			Start: md.Tuplet.Start, Stop: md.Tuplet.Stop,
		}
	}
	if h := md.Hairpin; h != nil {
		kind := model.Crescendo
		if h.Kind == "diminuendo" {
			kind = model.Diminuendo
		}
		m.Hairpin = &model.Hairpin{
			Kind: kind, Start: h.Start, Stop: h.Stop, Alone: h.Alone,
		}
	}
	if o := md.Octave; o != nil && o.Amount != 0 {
		m.Octave = &model.OctaveShift{Amount: o.Amount, Start: o.Start, Stop: o.Stop}
	}
	if tm := md.Tremolo; tm != nil {
		m.Tremolo = &model.Tremolo{Strokes: tm.Strokes, Buzz: tm.Buzz}
	}
	if tr := md.Trill; tr != nil {
		m.Trill = &model.Trill{Start: tr.Start, Stop: tr.Stop}
	}
	if ad := md.Arpeggio; ad != nil {
		m.Arpeggio = &model.Arpeggio{Down: ad.Down}
	}
	if b, ok := model.BreathFromName(md.Break); ok {
		m.Break = b
	}
	if len(md.Fingering) > 0 {
		m.Fingering = append([]string(nil), md.Fingering...)
	}
	if g := md.Glissando; g != nil {
		m.Glissando = &model.Glissando{
			Start: g.Start, Stop: g.Stop, Wavy: g.Wavy, Text: g.Text,
		}
	}
	if pd := md.Pedal; pd != nil {
		m.Pedal = &model.PedalMark{
			Start: pd.Start, Stop: pd.Stop, Change: pd.Change,
			Text: pd.Text, Half: pd.Half,
		}
	}
	m.Small = md.Small
	if b, ok := model.BeamFromName(md.Beam); ok {
		m.Beam = b
	}
	if cd := md.Chord; cd != nil {
		c := &model.ChordSymbol{
			Root: cd.Root, Quality: cd.Quality, Bass: cd.Bass, Text: cd.Text,
		}
		if f := cd.Frame; f != nil {
			c.Frame = &model.ChordFrame{
				Frets: f.Frets, Fingers: f.Fingers, FirstFret: f.FirstFret,
				Barre: f.Barre, BarreFrom: f.BarreFrom, BarreTo: f.BarreTo,
			}
		}
		m.Chord = c
	}
	for _, t := range md.Texts {
		// A style this build does not know is read as plain rather than
		// refused: a score from a later gscore should open with its
		// words on the page, set simply, rather than not open.
		style, _ := model.TextStyleFromName(t.Style)
		m.Texts = append(m.Texts, model.TextMark{
			Text: t.Text, Style: style,
			Placement: placementValues[t.Placement],
			OffsetX:   t.OffsetX, OffsetY: t.OffsetY,
			Italic: t.Italic, Bold: t.Bold,
			Runs: spansFromDoc(t.Runs),
		})
	}
	for _, ly := range md.Lyrics {
		verse := ly.Verse
		if verse < 1 {
			verse = 1
		}
		m.Lyrics = append(m.Lyrics, model.Lyric{
			Text: ly.Text, Verse: verse,
			Syllabic: syllabicValues[ly.Syllabic], Extend: ly.Extend,
		})
	}
}

func cursorToDoc(c *model.Cursor) *cursorDoc {
	if c == nil {
		return nil
	}
	return &cursorDoc{Part: c.Part, Measure: c.Measure, At: c.At, Step: c.Step}
}

// spanDoc is one run of a header field, as it is written to a file.
//
// Its own shape rather than the model's, for the reason everything else here
// has one: the file is a promise and the struct is not.
type spanDoc struct {
	Text   string  `json:"text"`
	Bold   bool    `json:"bold,omitempty"`
	Italic bool    `json:"italic,omitempty"`
	Colour string  `json:"colour,omitempty"`
	Size   float64 `json:"size,omitempty"`
}

func spansToDoc(runs []model.Span) []spanDoc {
	if len(runs) == 0 {
		return nil
	}
	out := make([]spanDoc, 0, len(runs))
	for _, r := range runs {
		out = append(out, spanDoc{
			Text: r.Text, Bold: r.Bold, Italic: r.Italic,
			Colour: r.Colour, Size: r.Size,
		})
	}
	return out
}

func spansFromDoc(runs []spanDoc) []model.Span {
	if len(runs) == 0 {
		return nil
	}
	out := make([]model.Span, 0, len(runs))
	for _, r := range runs {
		out = append(out, model.Span{
			Text: r.Text, Bold: r.Bold, Italic: r.Italic,
			Colour: r.Colour, Size: r.Size,
		})
	}
	return out
}

// slurEnds is how many curves open or close on a note, written so that a
// file gscore saves can still be opened by a gscore that has not heard of
// more than one.
//
// One curve is written as true, which is what the field has always held and
// what every file in existence holds. Two or more — a phrase with another
// inside it, a grace note slurred into the note a phrase ends on — are
// written as the number, which an older reader will refuse rather than
// silently misread. Reading goes the other way: true is one, false is none,
// and a number is itself.
type slurEnds int

func (s slurEnds) MarshalJSON() ([]byte, error) {
	switch {
	case s <= 0:
		return []byte("false"), nil
	case s == 1:
		return []byte("true"), nil
	}
	return []byte(strconv.Itoa(int(s))), nil
}

func (s *slurEnds) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true":
		*s = 1
		return nil
	case "false", "null":
		*s = 0
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	if n < 0 {
		n = 0
	}
	*s = slurEnds(n)
	return nil
}
