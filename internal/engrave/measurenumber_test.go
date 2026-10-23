package engrave

import (
	"strconv"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestMeasureNumbersShow(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	for i := 0; i < 12; i++ {
		if i > 0 {
			p.Measures = append(p.Measures, &model.Measure{Voices: []*model.Voice{{}}})
		}
		p.Measures[i].Voices[0].Elements = []model.Element{
			&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4},
				Dur: model.Duration{Num: 1, Den: 1}},
		}
	}
	for _, mode := range []Mode{Strip, Paginated} {
		for _, on := range []bool{false, true} {
			opt := DefaultOptions()
			opt.Mode = mode
			opt.LineWidth = 40
			opt.ShowMeasureNumbers = on
			lay := Engrave(s, opt)
			n := 0
			for _, x := range lay.Texts {
				if v, err := strconv.Atoi(x.S); err == nil && v >= 2 && v <= 12 {
					n++
				}
			}
			name := "strip"
			if mode == Paginated {
				name = "page"
			}
			t.Logf("%-5s numbers=%-5v -> %d bar numbers drawn", name, on, n)
			if on && n == 0 {
				t.Errorf("%s: asked for measure numbers and got none", name)
			}
			if !on && n != 0 {
				t.Errorf("%s: drew %d numbers without being asked", name, n)
			}
		}
	}
}
