package gscore

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A bar marked while the score was being checked is still marked when the
// file is opened again, and a mark this version does not know is dropped
// rather than read as something it is not.
func TestReviewMarksAreSavedWithTheScore(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures = append(p.Measures, &model.Measure{Voices: []*model.Voice{{}}},
		&model.Measure{Voices: []*model.Voice{{}}})
	p.Measures[0].Review = model.ReviewGood
	p.Measures[2].Review = model.ReviewFix

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, m := range back.Parts[0].Measures {
		got = append(got, m.Review)
	}
	want := []string{model.ReviewGood, "", model.ReviewFix}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bar %d is marked %q, want %q", i+1, got[i], want[i])
		}
	}

	doc := encodeScore(s)
	doc.Parts[0].Measures[2].Review = "maybe"
	back, err = decodeScore(doc)
	if err != nil {
		t.Fatal(err)
	}
	if r := back.Parts[0].Measures[2].Review; r != "" {
		t.Errorf("an unknown mark was read as %q", r)
	}
}
