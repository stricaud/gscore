package gscore

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
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
