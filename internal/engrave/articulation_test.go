package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// An articulation has to clear both the note it belongs to and the staff.
//
// Clearing one is not enough, and the two pull in opposite directions: a note
// inside the staff needs its mark pushed out past the lines, while a note
// outside them needs it pushed further out still. Satisfying only the staff
// dragged the marks of high and low notes back onto the noteheads.
func TestArticulationClearsNoteAndStaff(t *testing.T) {
	// Steps from well below the staff to well above it. The bottom line is
	// 0 and the top is 8, so these run from four spaces under to four over.
	for step := -8; step <= 16; step++ {
		s := model.NewScore()
		p := s.Parts[0]
		n := &model.Note{
			Pitch: PitchForStep(model.Treble, step),
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		n.AddArticulation(model.Marcato)
		n.AddArticulation(model.Staccato)
		p.Measures[0].Voices[0].Elements = []model.Element{n}

		opt := DefaultOptions()
		opt.Mode = Strip
		opt.WriteAhead = false
		lay := Engrave(s, opt)

		staffTop := lay.Hits[0].StaffTop
		noteY := 0.0
		var marks []Glyph
		for _, g := range lay.Glyphs {
			switch g.Name {
			case "noteheadBlack":
				noteY = g.Y
			case "articMarcatoAbove", "articMarcatoBelow",
				"articStaccatoAbove", "articStaccatoBelow":
				marks = append(marks, g)
			}
		}
		if len(marks) != 2 {
			t.Fatalf("step %d: got %d articulations, want 2", step, len(marks))
		}

		for _, g := range marks {
			above := g.Y < noteY
			// Clear of the notehead by at least a space.
			if gap := noteY - g.Y; above && gap < 1 {
				t.Errorf("step %d: a mark above sits %.2f from the notehead", step, gap)
			}
			if gap := g.Y - noteY; !above && gap < 1 {
				t.Errorf("step %d: a mark below sits %.2f from the notehead", step, gap)
			}
			// And outside the staff, never on the lines between them.
			inStaff := g.Y > staffTop-0.01 && g.Y < staffTop+staffHeight+0.01
			if inStaff {
				t.Errorf("step %d: a mark at y=%.2f is inside the staff (%.2f..%.2f)",
					step, g.Y, staffTop, staffTop+staffHeight)
			}
		}

		// The two marks must not sit on each other either.
		if d := marks[0].Y - marks[1].Y; d < 1 && d > -1 {
			t.Errorf("step %d: two articulations %.2f apart", step, d)
		}
	}
}
