package engrave

import (
	"math"
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A dot belongs to every notehead of the chord, not to the top one.
//
// The dot says the note is half as long again, and all of the chord's notes
// are: one dot beside the highest head left the rest of the chord reading as
// the plain value, which is not what the music says and not what the model
// holds.
func TestEveryNoteheadOfADottedChordIsDotted(t *testing.T) {
	dotted := model.Duration{Num: 3, Den: 4}
	note := func(s model.Step, oct int) *model.Note {
		return &model.Note{Pitch: model.Pitch{Step: s, Octave: oct}, Dur: dotted}
	}

	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		&model.Chord{Dur: dotted, Notes: []*model.Note{
			note(model.StepC, 4), note(model.StepE, 4), note(model.StepG, 4)}},
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
	}

	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	dots := dotGlyphs(lay)
	if len(dots) != 3 {
		t.Fatalf("got %d dots for a three-note chord, want 3", len(dots))
	}
	// One column: the dots of a chord line up, they do not step out after
	// the heads they belong to.
	for _, d := range dots[1:] {
		if math.Abs(d.X-dots[0].X) > 0.001 {
			t.Errorf("dots at x %.2f and %.2f, want them in one column",
				dots[0].X, d.X)
		}
	}
	// Each one against its own notehead, half a space up out of the line.
	heads := noteheadGlyphs(lay)
	if len(heads) != 3 {
		t.Fatalf("got %d noteheads, want 3", len(heads))
	}
	for i := range heads {
		if gap := heads[i].Y - dots[i].Y; math.Abs(gap-0.5) > 0.001 {
			t.Errorf("the dot for the notehead at y %.2f sits %.2f above it, "+
				"want half a space", heads[i].Y, gap)
		}
	}
}

// Two dots cannot share a space, so one of them gives way.
//
// A notehead on a line has its dot in the space above. When the note a
// second higher is already sitting in that space the two dots would be
// printed on top of each other, and the lower one goes into the space below
// its own head instead.
func TestTwoDotsASecondApartDoNotShareASpace(t *testing.T) {
	dotted := model.Duration{Num: 3, Den: 4}
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		&model.Chord{Dur: dotted, Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: dotted},
			{Pitch: model.Pitch{Step: model.StepF, Octave: 4}, Dur: dotted}}},
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
	}

	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	dots := dotGlyphs(lay)
	if len(dots) != 2 {
		t.Fatalf("got %d dots, want 2", len(dots))
	}
	if math.Abs(dots[0].Y-dots[1].Y) < 0.4 {
		t.Errorf("both dots at y %.2f and %.2f — they are on top of each other",
			dots[0].Y, dots[1].Y)
	}
	// Clear of the noteheads as well: E is on a line, F in the space above
	// it, so the E's dot goes into the space below the line.
	e := StaffStep(model.Treble, model.Pitch{Step: model.StepE, Octave: 4})
	f := StaffStep(model.Treble, model.Pitch{Step: model.StepF, Octave: 4})
	if got := chordDotSteps([]int{e, f}); got[0] != e-1 || got[1] != f {
		t.Errorf("dots at steps %v for noteheads at %d and %d, want %d and %d",
			got, e, f, e-1, f)
	}
}

// Two noteheads at the same pitch are one note to the reader, and carry one
// dot between them.
func TestAUnisonInAChordGetsOneDot(t *testing.T) {
	e := StaffStep(model.Treble, model.Pitch{Step: model.StepE, Octave: 4})
	if got := chordDotSteps([]int{e, e}); len(got) != 1 || got[0] != e+1 {
		t.Errorf("dots at steps %v for two noteheads both at %d, want one at %d",
			got, e, e+1)
	}
}

// The dots clear a notehead pushed across the stem.
//
// Noteheads a second apart cannot both sit on the same side of the stem, so
// one of them is drawn a notehead further right. Dots measured from the
// column alone were printed on top of it.
func TestAChordsDotsClearItsDisplacedNotehead(t *testing.T) {
	dotted := model.Duration{Num: 3, Den: 4}
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		&model.Chord{Dur: dotted, Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: dotted},
			{Pitch: model.Pitch{Step: model.StepF, Octave: 4}, Dur: dotted}}},
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
	}

	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	heads := noteheadGlyphs(lay)
	right := 0.0
	for _, h := range heads {
		if h.X > right {
			right = h.X
		}
	}
	for _, d := range dotGlyphs(lay) {
		if d.X <= right {
			t.Errorf("a dot at x %.2f sits on the notehead at x %.2f",
				d.X, right)
		}
	}
}

// Both dots of a double-dotted chord reach every notehead.
func TestADoubleDottedChordGetsTwoDotsPerNotehead(t *testing.T) {
	dur := model.Duration{Num: 7, Den: 8}
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		&model.Chord{Dur: dur, Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: dur},
			{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: dur}}},
		&model.Rest{Dur: model.Duration{Num: 1, Den: 8}},
	}

	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	if got := len(dotGlyphs(lay)); got != 4 {
		t.Errorf("got %d dots for a double-dotted two-note chord, want 4", got)
	}
}

// dotGlyphs returns the augmentation dots of a drawing, bottom of the staff
// first — which is the order the noteheads come back in too.
func dotGlyphs(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if g.Name == "augmentationDot" {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Y > out[j].Y })
	return out
}

func noteheadGlyphs(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		switch g.Name {
		case "noteheadWhole", "noteheadHalf", "noteheadBlack":
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Y > out[j].Y })
	return out
}
