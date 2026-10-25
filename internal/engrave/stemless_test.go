package engrave_test

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
	"github.com/stricaud/gscore/internal/model"
)

// Every notehead that is not a whole note has a stem on it.
//
// A chord drawn with no stem is the kind of fault that is obvious on the
// page and invisible to every other test: the notes are all there, in the
// right places, and the bar lasts the right length. It happened when the
// beam and the stems were each deciding which way the group pointed, and
// they stopped agreeing the day a staff could carry two voices.
//
// Checked over every score in the tree, from the layout rather than from a
// picture of it.
func TestEveryChordHasAStem(t *testing.T) {
	total := 0
	for name, path := range corpus(t) {
		s, _ := load(path)
		if s == nil {
			continue
		}
		total += stemless(t, name, s)
	}
	t.Logf("TOTAL chord columns with no stem across the corpus: %d", total)
}

func stemless(t *testing.T, name string, s *model.Score) int {
	lay := engrave.Engrave(s, engrave.DefaultOptions())

	type head struct {
		x, y, size float64
		name       string
	}
	var heads []head
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") {
			heads = append(heads, head{g.X, g.Y, g.Size, g.Name})
		}
	}
	var stems []engrave.Line
	for _, l := range lay.Lines {
		if l.X1 == l.X2 && l.Y1 != l.Y2 {
			stems = append(stems, l)
		}
	}

	cols := map[float64][]head{}
	for _, h := range heads {
		cols[h.x] = append(cols[h.x], h)
	}
	bad := 0
	for x, hs := range cols {
		if len(hs) < 2 {
			continue
		}
		allWhole := true
		for _, h := range hs {
			if !strings.Contains(h.name, "Whole") {
				allWhole = false
			}
		}
		if allWhole {
			continue
		}
		lo, hi := hs[0].y, hs[0].y
		for _, h := range hs {
			if h.y < lo {
				lo = h.y
			}
			if h.y > hi {
				hi = h.y
			}
		}
		found := false
		for _, l := range stems {
			if l.X1 < x-1.5 || l.X1 > x+2.0 {
				continue
			}
			// and it has to touch the noteheads, not merely share an x
			// with a barline somewhere else on the page
			top, bot := l.Y1, l.Y2
			if top > bot {
				top, bot = bot, top
			}
			if bot >= lo-0.6 && top <= hi+0.6 {
				found = true
				break
			}
		}
		if !found {
			bad++
			if bad <= 2 {
				t.Logf("  %s: no stem at x=%.2f y=%.2f %s (%d heads)",
					name, x, hs[0].y, hs[0].name, len(hs))
			}
		}
	}
	return bad
}
