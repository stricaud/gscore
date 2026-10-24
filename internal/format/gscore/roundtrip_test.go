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

	m2 := &model.Note{
		Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: model.Duration{Num: 1, Den: 4},
	}
	m2.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
	m2.Octave = &model.OctaveShift{Amount: 1, Stop: true}
	m2.Lyrics = []model.Lyric{{Text: "ri", Verse: 1, Syllabic: model.SyllableEnd, Extend: true}}

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
