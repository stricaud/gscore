package engrave_test

import (
	"os"
	"path/filepath"
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
	// Two sorts of staff have noteheads with no stems on purpose.
	//
	//   tablature       the notes are fret numbers on strings; there is no
	//                   stem to draw and nowhere to draw it
	//   tremolo chords  the slashes stand between two stems that the
	//                   tremolo draws itself
	//
	// Listed rather than quietly tolerated, so that the day a stem goes
	// missing somewhere else the count says so.
	expected := map[string]int{
		// Fret numbers on strings: there is no stem to draw.
		"71e-TabStaves.musicxml": 4,
		// A tremolo draws its own stems, with the slashes between them.
		"21g-Chords-Tremolos.musicxml": 6,
		// A real gap, and the one worth closing: a note written on one
		// staff of a piano and played by the other hand. The notehead goes
		// where the file says and the stem has nowhere to run to, because
		// the beam it belongs to is on the staff above or below.
		"43d-MultiStaff-StaffChange.musicxml": 2,
	}

	total := 0
	scores := corpus(t)
	// One file, for looking into a score that is not in the tree.
	if one := os.Getenv("SCORE"); one != "" {
		scores = map[string]string{filepath.Base(one): one}
	}
	for name, path := range scores {
		s, _ := load(path)
		if s == nil {
			continue
		}
		base := filepath.Base(name)
		want := expected[base]
		if n := stemless(t, name, s); n != want {
			t.Errorf("%s: %d noteheads with no stem, expected %d", base, n, want)
			total += n
		}
	}
	if total > 0 {
		t.Errorf("%d noteheads were drawn with no stem", total)
	}
}

func stemless(t *testing.T, name string, s *model.Score) int {
	opt := engrave.DefaultOptions()
	if os.Getenv("STRIP") != "" {
		// What the web pages are drawn in: one unbroken run of music.
		opt.Mode = engrave.Strip
	}
	lay := engrave.Engrave(s, opt)

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

	// Every notehead, not only the ones sharing an x. A chord holding a
	// second has one of its heads pushed sideways, so it is two columns of
	// one head each — and a check that only looked at columns of two or
	// more never looked at those at all.
	bad := 0
	for _, h := range heads {
		if strings.Contains(h.name, "Whole") {
			continue // a whole note has no stem by right
		}
		found := false
		for _, l := range stems {
			// A stem stands at one edge of its notehead or the other.
			if l.X1 < h.x-1.6 || l.X1 > h.x+1.8 {
				continue
			}
			top, bot := l.Y1, l.Y2
			if top > bot {
				top, bot = bot, top
			}
			if bot >= h.y-0.6 && top <= h.y+0.6 {
				found = true
				break
			}
		}
		if !found {
			bad++
			if bad <= 4 {
				t.Logf("  %s: no stem on %s at x=%.2f y=%.2f",
					name, h.name, h.x, h.y)
			}
		}
	}
	return bad
}
