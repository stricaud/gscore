package gscore

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A two-note tremolo is saved as one: both ends of it, and the hidden half
// of its time that each note takes. Bar 55 of Black and Tan Fantasy, made a
// pair, saved, and opened again as two notes each shaken on its own under a
// printed 2.
func TestATwoNoteTremoloIsSavedAsOne(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	end := func(s model.Step, start bool) *model.Note {
		n := &model.Note{Pitch: model.Pitch{Step: s, Octave: 4}, Dur: half}
		n.Tremolo = &model.Tremolo{Strokes: 3, Start: start, Stop: !start}
		n.Tuplet = &model.Tuplet{Hidden: true, Actual: 2, Normal: 1, Start: true, Stop: true}
		return n
	}
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{Voices: []*model.Voice{{
		Elements: []model.Element{end(model.StepF, true), end(model.StepD, false)}}}}}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	els := back.Parts[0].Measures[0].Voices[0].Elements
	a, b := els[0].(*model.Note), els[1].(*model.Note)
	if !a.Tremolo.Start || !b.Tremolo.Stop {
		t.Errorf("the pair came back as %+v and %+v", a.Tremolo, b.Tremolo)
	}
	if !a.Tuplet.Hidden || !b.Tuplet.Hidden {
		t.Error("the half of its time each note takes came back with a 2 printed over it")
	}
	if a.Dur != half {
		t.Errorf("the first note lasts %v, want %v", a.Dur, half)
	}
}
