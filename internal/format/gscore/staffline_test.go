package gscore

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A line from one staff to the other is saved with both its ends, on a note
// and on a rest.
func TestALineToTheOtherStaffIsSaved(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	note := &model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 5}, Dur: q}
	note.StaffLine = &model.StaffLine{Start: true}
	rest := &model.Rest{Dur: q}
	rest.StaffLine = &model.StaffLine{Stop: true}
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{note}}}}}
	s.Parts = append(s.Parts, &model.Part{Clef: model.Bass, Time: s.Parts[0].Time,
		Measures: []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{rest}}}}}})

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	n := back.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	r := back.Parts[1].Measures[0].Voices[0].Elements[0].(*model.Rest)
	if n.StaffLine == nil || !n.StaffLine.Start || r.StaffLine == nil || !r.StaffLine.Stop {
		t.Errorf("the line came back as %+v and %+v", n.StaffLine, r.StaffLine)
	}
}
