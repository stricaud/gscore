package gscore

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/symbol"
)

// A score carrying everything added since the format was first written has to
// come back the same. The container is the one thing that must not quietly
// lose a mark: it is where the work is kept.
func TestRoundTripKeepsEverything(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	p.Transpose = model.TranspositionFor(model.AltoSax)
	p.Instrument = model.AltoSax

	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 4},
		Accidental: model.Sharp,
	}
	n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}
	n.Octave = &model.OctaveShift{Amount: 1, Start: true}
	n.Lyrics = []model.Lyric{{Text: "Glo", Verse: 1, Syllabic: model.SyllableBegin}}
	n.Trill = &model.Trill{Start: true}
	n.Arpeggio = &model.Arpeggio{Down: true}
	n.Break = model.BreathComma
	n.Fingering = []string{"1-2", "4"}
	n.Glissando = &model.Glissando{Start: true, Wavy: true, Text: "gliss."}
	n.Pedal = &model.PedalMark{Start: true, Half: true}
	n.Small = true

	m2 := &model.Note{
		Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: model.Duration{Num: 1, Den: 4},
	}
	m2.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
	m2.Octave = &model.OctaveShift{Amount: 1, Stop: true}
	m2.Lyrics = []model.Lyric{{Text: "ri", Verse: 1, Syllabic: model.SyllableEnd, Extend: true}}
	m2.Glissando = &model.Glissando{Stop: true, Wavy: true}
	m2.Pedal = &model.PedalMark{Stop: true}
	m2.Break = model.Caesura

	p.Measures[0].Voices[0].Elements = []model.Element{n, m2}

	key := model.KeySignature{Fifths: -3, Mode: model.Minor}
	meter := model.TimeSignature{Beats: 6, BeatValue: 8}
	p.Measures = append(p.Measures, &model.Measure{
		Voices: []*model.Voice{{}},
		Key:    &key, Time: &meter,
		Ending: &model.Ending{Numbers: []int{1, 2}, Start: true, Stop: true},
		// A change of speed written over this bar, counted in a dotted
		// quarter — which is how 6/8 is counted, and a beat that is not in
		// the table of plain note values at all.
		Tempo: &model.Tempo{
			BPM: 72, Unit: model.Duration{Num: 3, Den: 8}, Text: "Slower",
		},
	})

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	bp := back.Parts[0]
	if bp.Transpose != p.Transpose {
		t.Errorf("transposition: got %+v want %+v", bp.Transpose, p.Transpose)
	}
	els := bp.Measures[0].Voices[0].Elements
	if len(els) != 2 {
		t.Fatalf("got %d elements, want 2", len(els))
	}
	first := els[0].(*model.Note)
	if first.Hairpin == nil || !first.Hairpin.Start {
		t.Error("the hairpin did not survive")
	}
	if first.Octave == nil || first.Octave.Amount != 1 {
		t.Error("the octave line did not survive")
	}
	if first.Trill == nil || !first.Trill.Start {
		t.Error("the trill did not survive")
	}
	if len(first.Lyrics) != 1 || first.Lyrics[0].Text != "Glo" ||
		first.Lyrics[0].Syllabic != model.SyllableBegin {
		t.Errorf("the lyric did not survive: %+v", first.Lyrics)
	}
	if !els[1].(*model.Note).Lyrics[0].Extend {
		t.Error("the held syllable did not survive")
	}

	m := bp.Measures[1]
	if m.Key == nil || m.Key.Fifths != -3 {
		t.Errorf("the key change did not survive: %+v", m.Key)
	}
	if m.Time == nil || m.Time.Beats != 6 || m.Time.BeatValue != 8 {
		t.Errorf("the meter change did not survive: %+v", m.Time)
	}
	if m.Ending == nil || len(m.Ending.Numbers) != 2 || !m.Ending.Start {
		t.Errorf("the ending did not survive: %+v", m.Ending)
	}
	if m.Tempo == nil {
		t.Fatal("the tempo change did not survive")
	}
	if m.Tempo.BPM != 72 || m.Tempo.Text != "Slower" {
		t.Errorf("the tempo change came back as %+v", m.Tempo)
	}
	if want := (model.Duration{Num: 3, Den: 8}); m.Tempo.Unit.Reduced() != want {
		t.Errorf("the beat came back as %v, want %v", m.Tempo.Unit, want)
	}
}

// Where the writer was left is kept with the score. Not music — it is the
// same courtesy a text editor does in remembering the line you were on — and
// without it, opening a piece means finding your place again every time.
func TestRoundTripKeepsTheCursor(t *testing.T) {
	s := model.NewScore()
	s.Cursor = &model.Cursor{Part: 0, Measure: 3, At: 2, Step: 6}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if back.Cursor == nil {
		t.Fatal("the cursor did not survive")
	}
	if *back.Cursor != *s.Cursor {
		t.Errorf("the cursor came back as %+v, want %+v", *back.Cursor, *s.Cursor)
	}

	// A score that never had one does not gain a cursor at the origin,
	// which would be a claim about where somebody was working.
	plain := model.NewScore()
	buf.Reset()
	if err := Write(&buf, plain, nil); err != nil {
		t.Fatal(err)
	}
	again, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if again.Cursor != nil {
		t.Errorf("a score with no cursor came back with %+v", *again.Cursor)
	}
}

// What a staff is written for survives being saved and opened again.
//
// The program says what to play it with and the sound says what it is, and
// the second is the one the range is asked of: saved as a program alone, a
// contrabassoon comes back a bassoon and is judged as one.
func TestTheInstrumentSoundSurvivesARoundTrip(t *testing.T) {
	s := model.NewScore()
	s.Parts[0].Instrument = model.Bassoon
	s.Parts[0].Sound = "wind.reed.contrabassoon"

	var b bytes.Buffer
	if err := Write(&b, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	back, _, err := Read(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if got := back.Parts[0].Sound; got != "wind.reed.contrabassoon" {
		t.Errorf("the part came back written for %q", got)
	}
	if got := back.Parts[0].SoundName(); got != "contrabassoon" {
		t.Errorf("and is called %q", got)
	}
}

// A script somebody wrote to make the music travels with it.
//
// A piece built from a process is not fully written down by its notes: the
// notes are the output and the process is the thought, and a composer who
// comes back a year later wants the thought.
func TestAScriptSurvivesARoundTrip(t *testing.T) {
	s := model.NewScore()
	s.Scripts = []model.Script{
		{Name: "row", Body: "for (let i = 0; i < 12; i++) note(row[i], \"q\");"},
		{Body: "transpose(+2)"},
	}

	var b bytes.Buffer
	if err := Write(&b, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	back, _, err := Read(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(back.Scripts) != 2 {
		t.Fatalf("%d scripts came back, want 2", len(back.Scripts))
	}
	if back.Scripts[0].Name != "row" || back.Scripts[0].Body != s.Scripts[0].Body {
		t.Errorf("the first came back as %+v", back.Scripts[0])
	}
	if back.Scripts[1].Body != "transpose(+2)" {
		t.Errorf("a script with no name lost its body: %+v", back.Scripts[1])
	}
}

// A score with no scripts carries none, rather than an empty list that
// grows a key in every file ever saved.
func TestAScoreWithNoScriptsWritesNone(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, model.NewScore(), nil); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b.Bytes(), []byte(`"scripts"`)) {
		t.Error("an empty score was given a scripts key")
	}
}

// A bracketed accidental survives being written out and read back.
//
// The brackets are a decision somebody made about how the music reads, and
// a format that loses them hands back a different score from the one that
// was saved.
func TestBracketedAccidentalSurvivesARoundTrip(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	q := model.Duration{Num: 1, Den: 4}
	note := &model.Note{
		Pitch:      model.Pitch{Step: model.StepF, Octave: 4, Alter: 1},
		Dur:        q,
		Accidental: model.Sharp,
		Parens:     true,
	}
	chord := &model.Chord{Dur: q, Notes: []*model.Note{
		{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: q},
		{Pitch: model.Pitch{Step: model.StepE, Octave: 4, Alter: -1}, Dur: q,
			Accidental: model.Flat, Parens: true},
	}}
	p.Measures[0].Voices[0].Elements = []model.Element{note, chord,
		&model.Rest{Dur: model.Duration{Num: 1, Den: 2}}}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	els := back.Parts[0].Measures[0].Voices[0].Elements
	got, ok := els[0].(*model.Note)
	if !ok {
		t.Fatalf("the first element came back as a %T", els[0])
	}
	if !got.Parens {
		t.Error("the note's brackets were lost")
	}
	ch, ok := els[1].(*model.Chord)
	if !ok {
		t.Fatalf("the second element came back as a %T", els[1])
	}
	if ch.Notes[0].Parens {
		t.Error("a notehead that had no brackets came back with them")
	}
	if !ch.Notes[1].Parens {
		t.Error("the chord's bracketed notehead lost them")
	}
}

// What a score is engraved by travels with it.
//
// These are the measurements somebody chose for this piece — how wide a
// quarter note's slot is, how heavy a beam is — and a file that forgot
// them would open looking like a different edition every time.
func TestEngravingSettingsSurviveARoundTrip(t *testing.T) {
	s := model.NewScore()
	s.Engraving = map[string]float64{"noteBase": 3.4, "beamThickness": 0.56}

	var b bytes.Buffer
	if err := Write(&b, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	back, _, err := Read(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(back.Engraving) != 2 {
		t.Fatalf("%d measurements came back, want 2: %v", len(back.Engraving), back.Engraving)
	}
	if back.Engraving["noteBase"] != 3.4 || back.Engraving["beamThickness"] != 0.56 {
		t.Errorf("they came back as %v", back.Engraving)
	}
}

// A score that has chosen nothing carries nothing, rather than a key in
// every file ever saved holding the defaults of the day it was written.
func TestAScoreWithNoEngravingSettingsWritesNone(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, model.NewScore(), nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if bytes.Contains(b.Bytes(), []byte("\"engraving\"")) {
		t.Error("a score that set nothing wrote an engraving block anyway")
	}
}

// How the bars are numbered travels with the score.
//
// Where the numbers fall is a decision somebody made about this piece —
// often enough to rehearse by, boxed enough to find — and a file that
// forgot it would print differently every time it was opened.
func TestNumberingSurvivesARoundTrip(t *testing.T) {
	s := model.NewScore()
	s.Numbering = model.MeasureNumbering{Every: 5, Boxed: true}
	at := 12
	s.Parts[0].Measures[0].NumberFrom = &at

	var b bytes.Buffer
	if err := Write(&b, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	back, _, err := Read(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if back.Numbering != s.Numbering {
		t.Errorf("the numbering came back as %+v, want %+v", back.Numbering, s.Numbering)
	}
	got := back.Parts[0].Measures[0].NumberFrom
	if got == nil || *got != 12 {
		t.Errorf("the restart came back as %v, want 12", got)
	}
}

// A score that has said nothing about its numbering writes nothing, rather
// than a key in every file ever saved.
func TestAScoreWithNoNumberingWritesNone(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, model.NewScore(), nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	for _, key := range []string{"\"numbering\"", "\"numberFrom\""} {
		if bytes.Contains(b.Bytes(), []byte(key)) {
			t.Errorf("a score that said nothing wrote %s anyway", key)
		}
	}
}

// Where the lines of music end travels with the score.
//
// A writer who has evened out the systems of a part has made a decision
// that cannot be worked out again from the notes, so a file that forgot it
// would reflow to something else the next time it was opened.
func TestSystemBreaksSurviveARoundTrip(t *testing.T) {
	s := model.NewScore()
	for len(s.Parts[0].Measures) < 6 {
		s.Parts[0].Measures = append(s.Parts[0].Measures,
			&model.Measure{Voices: []*model.Voice{{}}})
	}
	s.Parts[0].Measures[2].SystemBreak = true
	s.Parts[0].Measures[4].SystemBreak = true

	var b bytes.Buffer
	if err := Write(&b, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	back, _, err := Read(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	var at []int
	for i, m := range back.Parts[0].Measures {
		if m.SystemBreak {
			at = append(at, i)
		}
	}
	if len(at) != 2 || at[0] != 2 || at[1] != 4 {
		t.Errorf("the breaks came back at %v, want 2 and 4", at)
	}
}

// What a piece is travels with it.
//
// Tags are in the file rather than in a folder name or a list kept
// beside it, so that a score copied, renamed or sent to somebody takes
// its own description along — which is the whole reason for having them
// where they are.
func TestTagsSurviveARoundTrip(t *testing.T) {
	s := model.NewScore()
	s.Tags = []string{"advent", "flute duet", "grade 3"}

	var b bytes.Buffer
	if err := Write(&b, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	back, _, err := Read(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(back.Tags) != 3 || back.Tags[1] != "flute duet" {
		t.Errorf("the tags came back as %v", back.Tags)
	}
}

// A score with no tags writes none, rather than an empty list in every
// file ever saved.
func TestAScoreWithNoTagsWritesNone(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, model.NewScore(), nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if bytes.Contains(b.Bytes(), []byte("\"tags\"")) {
		t.Error("a score with no tags wrote a tags block anyway")
	}
}

// How a fretted instrument plays the note survives being saved.
//
// A mark that does not come back is a mark lost the first time the work is
// put away, and nothing would say so: the score would simply open with one
// fewer hammer-on than it was written with. Every one of them is checked
// rather than a sample, because the way this breaks is one name left out of
// a table and the rest working.
func TestEveryTabMarkSurvivesARoundTrip(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]

	// One note per mark, so that a name dropped on the way out can be
	// traced to the mark it belongs to.
	all := model.Techniques()
	for _, tech := range all {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepE, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		n.AddTechnique(tech)
		// And the shape, since a dead note is written as one of these
		// rather than as a mark and travels by another road entirely.
		n.Head = model.HeadX
		p.Measures[0].Voices[0].Elements = append(
			p.Measures[0].Voices[0].Elements, n)
	}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	els := back.Parts[0].Measures[0].Voices[0].Elements
	if len(els) != len(all) {
		t.Fatalf("%d notes came back, want %d", len(els), len(all))
	}
	for i, tech := range all {
		n, ok := els[i].(*model.Note)
		if !ok {
			t.Fatalf("element %d came back as %T", i, els[i])
		}
		if !n.HasTechnique(tech) {
			t.Errorf("%v was lost: the note came back with %v", tech, n.Techniques)
		}
		if n.Head != model.HeadX {
			t.Errorf("note %d came back drawn as %v", i, n.Head)
		}
	}
}

// The paper survives being saved, and a score that chose nothing writes
// nothing — which is the difference between following the program's
// defaults and freezing the day's into the file.
func TestThePaperSurvivesARoundTrip(t *testing.T) {
	s := model.NewScore()
	s.Page = model.Page{
		Paper: "a3", Landscape: true,
		MarginLeft: 30, MarginRight: 12, MarginTop: 18, MarginBottom: 20,
		StaffSpace: 5.5,
	}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if back.Page != s.Page {
		t.Errorf("the page came back as %+v, want %+v", back.Page, s.Page)
	}
}

// A custom page keeps both of its measurements.
//
// They are two numbers and were written under one name, so the height was
// lost and the page came back square. go vet is what found it; nothing on
// the page would have said so except the shape of the paper.
func TestACustomPageKeepsItsWidthAndItsHeight(t *testing.T) {
	s := model.NewScore()
	s.Page = model.Page{Paper: "custom", CustomWidth: 300, CustomHeight: 190}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if w, h := back.Page.Size(); w != 300 || h != 190 {
		t.Errorf("it came back %v by %v, want 300 by 190", w, h)
	}
}

func TestAScoreThatChoseNoPaperWritesNone(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, model.NewScore(), nil); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte(`"page"`)) {
		t.Error("a score that chose nothing wrote a page setup anyway")
	}
}

// A note written on the staff next door keeps being written there.
//
// Lost on the way through a file, the note comes back on its own staff:
// the music is unchanged and the page is wrong, which is the kind of
// mistake nobody finds until they print it.
func TestCrossStaffSurvivesARoundTrip(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	for _, dir := range []model.CrossStaff{
		model.OnStaffBelow, model.OnStaffAbove, model.OnOwnStaff,
	} {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepC, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 8},
		}
		n.Cross = dir
		p.Measures[0].Voices[0].Elements = append(
			p.Measures[0].Voices[0].Elements, n)
	}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	els := back.Parts[0].Measures[0].Voices[0].Elements
	for i, want := range []model.CrossStaff{
		model.OnStaffBelow, model.OnStaffAbove, model.OnOwnStaff,
	} {
		n := els[i].(*model.Note)
		if n.Cross != want {
			t.Errorf("note %d came back written on %v, want %v", i, n.Cross, want)
		}
	}
}

// What kind of writing a text is survives being saved. Lost, every word
// on the page comes back set as an expression marking: the tempo stops
// being bold and moves from over the system to under one staff.
func TestTextStylesSurviveARoundTrip(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepC, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	for _, st := range model.TextStyles() {
		n.Texts = append(n.Texts, model.TextMark{Text: st.String(), Style: st})
	}
	p.Measures[0].Voices[0].Elements = []model.Element{n}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := back.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if len(got.Texts) != len(model.TextStyles()) {
		t.Fatalf("%d texts came back, want %d", len(got.Texts), len(model.TextStyles()))
	}
	for i, st := range model.TextStyles() {
		if got.Texts[i].Style != st {
			t.Errorf("%v came back as %v", st, got.Texts[i].Style)
		}
	}
}

// And a mark that left its placement to its style keeps leaving it to the
// style, rather than coming back pinned to wherever it happened to be.
func TestATextThatLeftItsPlaceToItsStyleStillDoes(t *testing.T) {
	s := model.NewScore()
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepC, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	n.Texts = []model.TextMark{{Text: "dolce", Style: model.TextExpression}}
	s.Parts[0].Measures[0].Voices[0].Elements = []model.Element{n}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := back.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if got.Texts[0].Placement != model.PlacementAuto {
		t.Errorf("it came back pinned to %v", got.Texts[0].Placement)
	}
}

// A sign somebody insisted on survives the file.
//
// Without this, a score read from a format that states what was printed —
// MusicXML does, and a redundant accidental it asked for is its decision to
// have made — would be respelled the next time it was opened, and would not
// come back as it was saved. The flag is what tells SpellAccidentals to
// leave the note alone, so if the flag does not survive, the sign does not
// either.
func TestAForcedAccidentalSurvivesARoundTrip(t *testing.T) {
	s := viewScore()
	p := s.Parts[0]
	m := model.NewMeasure()
	m.Voices = []*model.Voice{{Elements: []model.Element{
		&model.Note{
			Pitch:      model.Pitch{Step: model.StepF, Octave: 4, Alter: 1},
			Dur:        model.Duration{Num: 1, Den: 4},
			Accidental: model.Sharp, Forced: true,
		},
		&model.Chord{
			Dur: model.Duration{Num: 1, Den: 4},
			Notes: []*model.Note{{
				Pitch:      model.Pitch{Step: model.StepC, Octave: 5, Alter: 1},
				Accidental: model.Sharp, Forced: true,
			}},
		},
	}}}
	p.Measures = []*model.Measure{m}

	got := roundTrip(t, s)

	back := got.Parts[0].Measures[0].Voices[0].Elements
	if n, ok := back[0].(*model.Note); !ok || !n.Forced {
		t.Error("the note came back without its sign held")
	}
	if c, ok := back[1].(*model.Chord); !ok || !c.Notes[0].Forced {
		t.Error("the chord's notehead came back without its sign held")
	}

	// And the proof that it matters: respelling must leave them alone.
	got.SpellAccidentals()
	back = got.Parts[0].Measures[0].Voices[0].Elements
	if n := back[0].(*model.Note); n.Accidental != model.Sharp {
		t.Errorf("the sign became %v after respelling", n.Accidental)
	}
}

// The header keeps its arranger and how it is set.
func TestRoundTripKeepsTheCreditsAndHowTheyAreSet(t *testing.T) {
	s := model.NewScore()
	s.Identity = model.Identity{
		Title:    "Sonata in G minor",
		Composer: "Music by J. S. Bach",
		Arranger: "A. Smith",
		TitleRuns: []model.Span{
			{Text: "Sonata "},
			{Text: "in G minor", Italic: true, Size: 0.75},
		},
		ComposerRuns: []model.Span{
			{Text: "Music by "},
			{Text: "J. S. Bach", Bold: true, Colour: "#c01020"},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	got, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}

	if got.Identity.Arranger != "A. Smith" {
		t.Errorf("the arranger came back as %q", got.Identity.Arranger)
	}
	if len(got.Identity.TitleRuns) != 2 {
		t.Fatalf("the title came back in %d pieces, want 2", len(got.Identity.TitleRuns))
	}
	if r := got.Identity.TitleRuns[1]; r.Text != "in G minor" || !r.Italic || r.Size != 0.75 {
		t.Errorf("the second piece of the title came back as %+v", r)
	}
	if r := got.Identity.ComposerRuns[1]; r.Colour != "#c01020" || !r.Bold {
		t.Errorf("the name came back as %+v", r)
	}
	// And a copy of the score shares none of it, or editing one would
	// change the other.
	clone := got.Clone()
	clone.Identity.TitleRuns[0].Text = "changed"
	if got.Identity.TitleRuns[0].Text != "Sonata " {
		t.Error("a copy of the score shares its title's formatting with it")
	}
}

// Which side of a note a mark is written on survives a save.
func TestRoundTripKeepsWhereAMarkIsWritten(t *testing.T) {
	s := model.NewScore()
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepG, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	n.Marks.AddArticulation(model.Staccato)
	n.Marks.AddArticulation(model.Fermata)
	n.Marks.PlaceArticulation(model.Fermata, model.PlacementBelow)
	s.Parts[0].Measures[0].Voices[0].Elements = []model.Element{n}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	got, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	back := got.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if len(back.Articulations) != 2 {
		t.Fatalf("%d marks came back", len(back.Articulations))
	}
	if back.ArticulationPlacement(0) != model.PlacementAuto {
		t.Error("the staccato came back pinned to a side nobody asked for")
	}
	if got := back.ArticulationPlacement(1); got != model.PlacementBelow {
		t.Errorf("the fermata came back %v, want below", got)
	}
}

// TestNestedSlursSurviveTheFile.
//
// A phrase with another inside it, and a note two curves end on. They are
// written as numbers where the file has always held true or false, so the
// old shape has to keep working both ways: one curve is still written as
// true, which a gscore that has not heard of more than one can still open.
func TestNestedSlursSurviveTheFile(t *testing.T) {
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	var notes []*model.Note
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.Step(i), Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		notes = append(notes, n)
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	notes[0].SlurStart = 1
	notes[1].SlurStart = 1
	notes[2].SlurStop = 1
	notes[3].SlurStop = 2

	raw, err := json.Marshal(encodeScore(s))
	if err != nil {
		t.Fatal(err)
	}
	// One curve is still written the way it always was.
	if !strings.Contains(string(raw), `"slurStart":true`) {
		t.Error("a single curve is no longer written as true: an older " +
			"gscore would not read this file")
	}
	if !strings.Contains(string(raw), `"slurStop":2`) {
		t.Error("two curves ending on one note are not written as a number")
	}

	var doc scoreDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	back, err := decodeScore(&doc)
	if err != nil {
		t.Fatal(err)
	}
	got := back.Parts[0].Measures[0].Voices[0].Elements
	for i, want := range []struct{ start, stop int }{
		{1, 0}, {1, 0}, {0, 1}, {0, 2},
	} {
		n := got[i].(*model.Note)
		if n.SlurStart != want.start || n.SlurStop != want.stop {
			t.Errorf("note %d came back %d/%d, want %d/%d",
				i+1, n.SlurStart, n.SlurStop, want.start, want.stop)
		}
	}
}

// TestAFileFromBeforeTheCountsStillOpens.
//
// Every .gscore in existence writes these as true and false. Read as
// numbers without a word, they would come back as nothing at all — every
// slur in every saved score gone.
func TestAFileFromBeforeTheCountsStillOpens(t *testing.T) {
	var md marksDoc
	if err := json.Unmarshal([]byte(`{"slurStart":true,"slurStop":false}`),
		&md); err != nil {
		t.Fatal(err)
	}
	var m model.Marks
	decodeMarks(&md, &m)
	if m.SlurStart != 1 {
		t.Errorf("a curve written as true came back as %d", m.SlurStart)
	}
	if m.SlurStop != 0 {
		t.Errorf("false came back as %d", m.SlurStop)
	}
}

// TestWhatIsWrittenInsideABarSurvivesTheFile.
//
// Three things hang off an element rather than off its bar, because that is
// where they happen: a clef that changes partway through, a key that turns
// at a double bar inside the measure, and the little notes crushed in
// before a note. None of the three was written to the file at all — so a
// score read from Dorico with two hundred grace notes in it, saved and
// opened again, had none.
func TestWhatIsWrittenInsideABarSurvivesTheFile(t *testing.T) {
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	bass := model.Bass
	key := model.KeySignature{Fifths: 2, Mode: model.Minor}
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepC, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	n.Clef = &bass
	n.Key = &key
	n.Grace = &model.GraceGroup{Slashed: true, Notes: []*model.Note{
		{Pitch: model.Pitch{Step: model.StepD, Octave: 4},
			Dur: model.Duration{Num: 1, Den: 16}},
		{Pitch: model.Pitch{Step: model.StepE, Octave: 4},
			Dur: model.Duration{Num: 1, Den: 16}},
	}}
	m.Voices[0].Elements = append(m.Voices[0].Elements, n)

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := back.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)

	if got.Clef == nil || *got.Clef != model.Bass {
		t.Errorf("the clef written inside the bar came back as %v", got.Clef)
	}
	if got.Key == nil {
		t.Fatal("the key written inside the bar did not come back")
	}
	if got.Key.Fifths != 2 || got.Key.Mode != model.Minor {
		t.Errorf("the key came back as %d %v, want two sharps minor",
			got.Key.Fifths, got.Key.Mode)
	}
	if got.Grace == nil {
		t.Fatal("the grace notes did not come back")
	}
	if len(got.Grace.Notes) != 2 {
		t.Errorf("%d little notes came back, want 2", len(got.Grace.Notes))
	}
	if !got.Grace.Slashed {
		t.Error("the group came back unslashed, so an acciaccatura became " +
			"an appoggiatura")
	}
	if got.Grace.Notes[0].Pitch.Step != model.StepD {
		t.Errorf("the first little note is %v, want D",
			got.Grace.Notes[0].Pitch)
	}
}

// A comment survives being saved and opened again.
//
// It is not music, which is exactly why it is the kind of thing a file
// format forgets: the notes are checked by everybody and the reasoning
// behind them by nobody.
func TestCommentsSurviveTheRoundTrip(t *testing.T) {
	s := model.NewScore()
	s.Comments = []model.Comment{
		{Text: "Kept in B flat", Author: "Ben", At: 3, Bars: 2, Part: 1,
			When: 1764258281},
		{Text: "Check the autograph", At: 0, Bars: 1, Part: -1},
	}
	var b bytes.Buffer
	if err := Write(&b, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Comments) != 2 {
		t.Fatalf("%d comments came back", len(back.Comments))
	}
	if back.Comments[0] != s.Comments[0] {
		t.Errorf("the first came back as %+v", back.Comments[0])
	}
	if back.Comments[1].Text != "Check the autograph" ||
		back.Comments[1].Part != -1 {
		t.Errorf("the second came back as %+v", back.Comments[1])
	}
}

// The pictures printed around the music come back too.
//
// A cover and a stamp are bytes rather than words, and the container is the
// one thing that must not quietly lose them: a score that forgot its cover
// every time it was saved would be a cover somebody set once and never saw
// again.
func TestRoundTripKeepsTheCoverAndTheStamp(t *testing.T) {
	s := model.NewScore()
	s.Cover = &model.Artwork{Data: []byte{0x89, 'P', 'N', 'G', 1, 2, 3}, Format: "png"}
	s.Stamp = &model.Artwork{Data: []byte("<svg/>"), Format: "svg", Align: "right"}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	got, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Cover == nil || !bytes.Equal(got.Cover.Data, s.Cover.Data) {
		t.Errorf("the cover came back as %v", got.Cover)
	}
	if got.Cover != nil && got.Cover.Format != "png" {
		t.Errorf("the cover came back as a %q", got.Cover.Format)
	}
	if got.Stamp == nil || got.Stamp.Align != "right" ||
		!bytes.Equal(got.Stamp.Data, s.Stamp.Data) {
		t.Errorf("the stamp came back as %v", got.Stamp)
	}
}

// And how large a placed symbol is drawn.
//
// Per placement, so a drawing used in two places at two sizes comes back
// at both: the size is on the placement and not on the symbol.
func TestRoundTripKeepsHowLargeASymbolIsDrawn(t *testing.T) {
	s := model.NewScore()
	v := s.Parts[0].Measures[0].Voices[0]
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepB, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	n.Symbols = []model.SymbolMark{
		{Name: "mine", Scale: 2.5},
		{Name: "mine"}, // the same drawing, left as it was made
	}
	v.Elements = append(v.Elements, n)
	v.Elements = append(v.Elements, &model.SymbolElement{
		Symbol: model.SymbolMark{Name: "mine", Scale: 0.5},
	})

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	got, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	back := got.Parts[0].Measures[0].Voices[0].Elements
	marks := marksOfNote(back[0])
	if marks == nil || len(marks.Symbols) != 2 {
		t.Fatalf("the note came back with %v", marks)
	}
	if marks.Symbols[0].ScaleOr1() != 2.5 {
		t.Errorf("the first is at %v", marks.Symbols[0].ScaleOr1())
	}
	if marks.Symbols[1].Scale != 0 {
		t.Errorf("the second came back carrying a scale of %v",
			marks.Symbols[1].Scale)
	}
	lone, ok := back[1].(*model.SymbolElement)
	if !ok {
		t.Fatalf("the lone symbol came back as %T", back[1])
	}
	if lone.Symbol.ScaleOr1() != 0.5 {
		t.Errorf("the lone symbol is at %v", lone.Symbol.ScaleOr1())
	}
}

func marksOfNote(el model.Element) *model.Marks {
	if n, ok := el.(*model.Note); ok {
		return &n.Marks
	}
	return nil
}

// An outline symbol survives being saved.
//
// Only grids of pixels were written, so a symbol made from an SVG was on
// the page until the file was saved and gone when it was opened again.
// Nothing said so: the score kept the name and the registry had nothing
// under it, so the mark simply did not appear.
func TestRoundTripKeepsAnOutlineSymbol(t *testing.T) {
	reg := symbol.NewRegistry()
	if err := reg.Add(&symbol.Symbol{
		Name: "mark", Kind: symbol.FromVector, Path: "M0 0 L1 0 L1 1 Z",
	}); err != nil {
		t.Fatal(err)
	}
	s := model.NewScore()
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepB, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	n.Symbols = []model.SymbolMark{{Name: "mark"}}
	s.Parts[0].Measures[0].Voices[0].Elements = []model.Element{n}

	var buf bytes.Buffer
	if err := Write(&buf, s, reg); err != nil {
		t.Fatal(err)
	}
	_, back, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := back.Lookup("mark")
	if !ok {
		t.Fatal("the outline is gone from the file")
	}
	if got.Kind != symbol.FromVector || got.Path != "M0 0 L1 0 L1 1 Z" {
		t.Errorf("it came back as %v with path %q", got.Kind, got.Path)
	}
}
