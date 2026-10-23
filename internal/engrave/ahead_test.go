package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestWriteAheadBar(t *testing.T) {
	build := func(final bool) *model.Score {
		s := model.NewScore()
		p := s.Parts[0]
		p.Measures[0].Voices[0].Elements = []model.Element{
			&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4},
				Dur: model.Duration{Num: 1, Den: 1}},
		}
		if final {
			p.Measures[0].RightBarline = model.BarFinal
		}
		return s
	}

	cases := []struct {
		name  string
		final bool
		mode  Mode
		ahead bool
		want  int
	}{
		{"editing, unfinished", false, Strip, true, 2},
		{"editing, final barline", true, Strip, true, 1},
		{"printing", false, Paginated, true, 1},
		{"turned off", false, Strip, false, 1},
	}
	for _, c := range cases {
		opt := DefaultOptions()
		opt.Mode = c.mode
		opt.WriteAhead = c.ahead
		lay := Engrave(build(c.final), opt)
		bars := map[int]bool{}
		for _, h := range lay.Hits {
			bars[h.Measure] = true
		}
		if len(bars) != c.want {
			t.Errorf("%s: %d bars to click in, want %d", c.name, len(bars), c.want)
		}
		// The extra bar has no barline of its own.
		if c.want == 2 && len(lay.Barlines) != 1 {
			t.Errorf("%s: %d barlines, want 1", c.name, len(lay.Barlines))
		}
	}
}
