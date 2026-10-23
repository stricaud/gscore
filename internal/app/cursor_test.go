package app

import (
	"math"
	"os"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"

	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/nwc"
)

// The cursor has to be on the note that is sounding.
//
// Two things decide where it goes, and they are computed separately: the
// cursor is placed from the measure timeline, and the highlight from the
// event times. If those two clocks disagree by so much as a beat, the cursor
// points at one note while another sounds — which is what a conductor's line
// down a score is for, so it is the whole feature.
//
// The engraver and playback number elements identically, walking parts, then
// measures, then the first voice. That shared numbering is what lets this
// test ask, for every note in a score: when you sound, where is the cursor?
func checkCursorFollowsTheNotes(t *testing.T, s *model.Score) {
	t.Helper()

	pb := buildPlayback(s)
	lay := engrave.Engrave(s, engrave.DefaultOptions())

	// Where the cursor would be at a given moment, from the measure timeline
	// alone — the same arithmetic the interface does.
	bar := make(map[int]MeasureTime, len(pb.Measures))
	for _, m := range pb.Measures {
		bar[m.Index] = m
	}

	// Where each element was drawn, by the number both sides give it.
	type where struct {
		measure int
		at      float64 // its moment within the bar, 0 to 1
	}
	drawn := map[int]where{}
	for _, h := range lay.Hits {
		for _, el := range h.Elements {
			drawn[el.Ref] = where{measure: h.Measure, at: el.At}
		}
	}

	checked, worst, worstWhy := 0, 0.0, ""
	for _, ev := range pb.Events {
		d, ok := drawn[ev.Ref]
		if !ok {
			continue // a note in a bar the engraver did not draw
		}
		m, ok := bar[d.measure]
		if !ok || m.Dur <= 0 {
			continue
		}
		// The moment the cursor reaches this note's place in its bar.
		cursorReachesIt := m.Start + d.at*m.Dur
		off := math.Abs(cursorReachesIt - ev.Start)
		checked++
		if off > worst {
			worst = off
			worstWhy = "bar " + itoa(d.measure+1) + ": it sounds at " +
				ftoa(ev.Start) + "s, the cursor gets there at " +
				ftoa(cursorReachesIt) + "s"
		}
	}
	if checked == 0 {
		t.Fatal("nothing was checked")
	}
	t.Logf("%d notes checked, worst gap %.4f seconds", checked, worst)
	// A thousandth of a second is rounding; a hundredth would be visible.
	if worst > 0.005 {
		t.Errorf("the cursor is not on the note that sounds: %s", worstWhy)
	}
}

func ftoa(v float64) string {
	return itoa(int(v)) + "." + itoa(int(math.Abs(v*100))%100)
}

// A score built here, with bars that are deliberately uneven.
func TestCursorFollowsTheNotes(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/addstaff?instrument=cello&clef=bass")

	// Staff 0 fills every bar, staff 1 has a short bar, staff 2 stops early
	// and is padded — all three of which used to pull a part's clock away
	// from the barlines.
	for bar := 0; bar < 6; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		for i := 0; i < 4; i++ {
			h.note(0, bar, 2+i, "quarter")
		}
		if bar < 4 {
			h.note(1, bar, 4, "half")
		}
		if bar < 2 {
			for i := 0; i < 4; i++ {
				h.note(2, bar, 2+i, "quarter")
			}
		}
	}
	h.app.score.Normalise()
	checkCursorFollowsTheNotes(t, h.app.score)
}

// And on the real thing, where the problem showed up.
func TestCursorFollowsTheNotesInRealScores(t *testing.T) {
	for _, path := range []string{
		os.Getenv("HOME") + "/Downloads/wamrequiem/08-domin.nwc",
		os.Getenv("HOME") + "/Downloads/wamrequiem/10-sanct.nwc",
		os.Getenv("HOME") + "/Downloads/wamrequiem/02-kyrie.nwc",
	} {
		f, err := os.Open(path)
		if err != nil {
			t.Skipf("%s is not here", path)
		}
		imp, err := format.ImporterFor(path)
		if err != nil {
			f.Close()
			t.Fatal(err)
		}
		scores, _ := format.ImportAll(imp, f)
		f.Close()
		if len(scores) == 0 {
			continue
		}
		t.Run(path[len(path)-14:], func(t *testing.T) {
			checkCursorFollowsTheNotes(t, scores[0])
		})
	}
}

// The cursor crosses a bar at a steady rate, so the path it takes is only as
// smooth as the bars it is handed. What it relies on is that the bars run
// left to right without overlapping, and that the only thing between one and
// the next is the barline — which the cursor spans by aiming at where the
// next bar starts rather than at where this one's music stops.
func TestBarsRunLeftToRight(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/addstaff?instrument=cello&clef=bass")
	for bar := 0; bar < 8; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		// Deliberately uneven: a bar of four against a bar of one, and a
		// staff that stops early and gets padded. Each of these used to pull
		// the bars out of step with one another.
		h.note(0, bar, 2+bar%5, "whole")
		if bar%2 == 0 {
			for i := 0; i < 4; i++ {
				h.note(1, bar, 2+i, "quarter")
			}
		}
		if bar < 5 {
			h.note(2, bar, 4, "half")
			h.note(2, bar, 6, "half")
		}
	}
	h.app.score.Normalise()

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip // one system: the editing view
	lay := engrave.Engrave(h.app.score, opt)

	// Each bar's extent across every staff showing it, as the interface
	// works it out.
	type box struct{ x0, x1 float64 }
	boxes := map[int]box{}
	for _, hit := range lay.Hits {
		b, seen := boxes[hit.Measure]
		if !seen {
			b = box{x0: hit.XMin, x1: hit.XMax}
		}
		if hit.XMin < b.x0 {
			b.x0 = hit.XMin
		}
		if hit.XMax > b.x1 {
			b.x1 = hit.XMax
		}
		boxes[hit.Measure] = b
	}
	if len(boxes) < 8 {
		t.Fatalf("got %d bars", len(boxes))
	}

	gap := 0.0
	for i := 0; i < len(boxes); i++ {
		b := boxes[i]
		if b.x1 <= b.x0 {
			t.Errorf("bar %d has no width: %.2f..%.2f", i+1, b.x0, b.x1)
		}
		if i == 0 {
			continue
		}
		prev := boxes[i-1]
		// Strictly to the right, and never overlapping: either would send
		// the cursor backwards at a barline.
		if b.x0 <= prev.x0 {
			t.Errorf("bar %d starts at %.2f, not right of bar %d at %.2f",
				i+1, b.x0, i, prev.x0)
		}
		d := b.x0 - prev.x1
		if d < 0 {
			t.Errorf("bar %d overlaps bar %d by %.2f spaces", i+1, i, -d)
		}
		// The same barline gap every time. An uneven one would make the
		// cursor's pace change from bar to bar.
		if i == 1 {
			gap = d
		} else if math.Abs(d-gap) > 0.01 {
			t.Errorf("the gap before bar %d is %.2f, but %.2f elsewhere", i+1, d, gap)
		}
	}
	t.Logf("%d bars, a constant %.2f spaces of barline between them", len(boxes), gap)
}
