package pdf

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

func TestLayoutDebug(t *testing.T) {
	s := buildDemo()
	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.LineWidth = 76
	eopt.ShowPartNames = true
	lay := engrave.Engrave(s, eopt)

	t.Logf("height %.1f, parts %d", lay.Height, len(s.Parts))
	for i, sys := range lay.Systems {
		t.Logf("system %d: %.1f .. %.1f bars %d-%d", i, sys.Top, sys.Bottom, sys.First, sys.Last)
	}
	// Which staff tops exist: a staff line is a horizontal line 5 of which
	// share an x range.
	tops := map[float64]int{}
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.X2-l.X1 > 30 {
			tops[l.Y1]++
		}
	}
	t.Logf("%d long horizontal lines (staff lines)", len(tops))
	seen := map[int]bool{}
	for _, h := range lay.Hits {
		if !seen[h.Part] {
			seen[h.Part] = true
			t.Logf("  part %d first staffTop %.1f", h.Part, h.StaffTop)
		}
	}
	opt := DefaultOptions()
	r := &renderer{lay: lay, opt: opt}
	for i, b := range r.pages() {
		t.Logf("page %d: %.1f .. %.1f", i, b.top, b.bottom)
	}
	t.Logf("usable %.1f", (opt.PageHeight-opt.MarginTop-opt.MarginBottom)/opt.StaffSpace)
}
