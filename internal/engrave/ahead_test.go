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

// The write-ahead bar says what it is.
//
// It looks exactly like an empty bar of the piece from outside, and the
// editor counts bars from these. Counted as one, the editor believes the
// score is a bar longer than it is: it then writes into that bar without
// first making it, and the bar comes into being on the one staff that was
// written to while every other staff stays a bar short of it.
func TestTheWriteAheadBarIsMarkedAsOne(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4},
			Dur: model.Duration{Num: 1, Den: 1}},
	}

	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = true
	lay := Engrave(s, opt)

	real, ahead := 0, 0
	for _, h := range lay.Hits {
		if h.Ahead {
			ahead++
			continue
		}
		real++
	}
	if real != 1 {
		t.Errorf("%d bars of the piece, want 1", real)
	}
	if ahead != 1 {
		t.Errorf("%d bars marked as room to write, want 1", ahead)
	}
}
