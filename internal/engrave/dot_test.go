package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// Augmentation dots: three things that were all out by the width of a dot.
func TestDotPlacement(t *testing.T) {
	note := func(step model.Step, oct, num, den int) *model.Note {
		return &model.Note{Pitch: model.Pitch{Step: step, Octave: oct},
			Dur: model.Duration{Num: num, Den: den}}
	}
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		// High, so the stem goes down and the dot has the notehead's right
		// side to itself.
		note(model.StepA, 5, 3, 8), note(model.StepA, 5, 1, 8),
		// Low, so the stem goes up the side the dot is on.
		note(model.StepB, 4, 3, 8), note(model.StepB, 4, 1, 8),
	}
	p.Measures = append(p.Measures, &model.Measure{Voices: []*model.Voice{{
		// Double dotted: two dots in a row.
		Elements: []model.Element{note(model.StepC, 5, 7, 16),
			note(model.StepC, 5, 1, 16)},
	}}})

	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	adv := smufl.Advance("augmentationDot")
	var dots []Glyph
	var heads []Glyph
	for _, g := range lay.Glyphs {
		switch g.Name {
		case "augmentationDot":
			dots = append(dots, g)
		case "noteheadBlack":
			heads = append(heads, g)
		}
	}
	if len(dots) != 4 {
		t.Fatalf("got %d dots, want 4 (one, one, and a pair)", len(dots))
	}

	// No dot may sit on the one before it.
	for i := 1; i < len(dots); i++ {
		if dots[i].Y != dots[i-1].Y {
			continue // a different note
		}
		if gap := dots[i].X - (dots[i-1].X + adv); gap < 0 {
			t.Errorf("dot %d overlaps the one before it by %.2f spaces", i, -gap)
		}
	}

	// A dot never sits on a staff line: the step it is drawn at must be odd,
	// counting half-spaces from the bottom line.
	for i, d := range dots {
		step := (staffHeight - (d.Y - lay.Hits[0].StaffTop)) * 2
		if int(step)%2 == 0 {
			t.Errorf("dot %d sits on a line, at step %.0f", i, step)
		}
	}

	// Every dot clears the notehead it belongs to, and the stem when the
	// stem is on the same side.
	for _, d := range dots {
		var head *Glyph
		for i := range heads {
			h := heads[i]
			if h.X < d.X && (head == nil || h.X > head.X) {
				head = &heads[i]
			}
		}
		if head == nil {
			continue
		}
		gap := d.X - (head.X + smufl.Advance(head.Name))
		if gap < 0.3 {
			t.Errorf("a dot sits %.2f after its notehead, which is too close", gap)
		}
	}
}

// A note whose stem goes up has it on the side the dots are, so the dots
// start further out than they would on a note whose stem goes down.
func TestDotClearsAnUpStem(t *testing.T) {
	opt := DefaultOptions()
	e := &engraver{opt: opt}
	down := e.dotGap(false)
	up := e.dotGap(true)
	if up <= down {
		t.Errorf("a dot after an up-stem note is placed at %.2f, no further "+
			"out than after a down-stem one at %.2f", up, down)
	}
	if down != opt.Spacings.DotToNote {
		t.Errorf("a down-stem note's dot gap is %.2f, want the plain %.2f",
			down, opt.Spacings.DotToNote)
	}
}

// The room reserved for a dotted note has to be the room its dots take.
func TestDottedNoteIsBudgetedForItsDots(t *testing.T) {
	opt := DefaultOptions()
	e := &engraver{opt: opt}
	plain := e.elementWidth(&model.Note{Dur: model.Duration{Num: 1, Den: 4}})
	dotted := e.elementWidth(&model.Note{Dur: model.Duration{Num: 3, Den: 8}})

	// At least the dot and the air on either side of it.
	least := opt.Spacings.DotToNote + smufl.Advance("augmentationDot")
	if dotted-plain < least {
		t.Errorf("a dotted quarter gets %.2f more room than a quarter, "+
			"but its dot alone needs %.2f", dotted-plain, least)
	}
}
