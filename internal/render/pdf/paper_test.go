/*
 * paper_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

func TestLandscapeTurnsThePaperOnce(t *testing.T) {
	o := DefaultOptions().Landscape()
	if o.PageWidth <= o.PageHeight {
		t.Fatalf("landscape A4 is %.2f by %.2f", o.PageWidth, o.PageHeight)
	}
	// Asking twice is asking for the same thing, not for the sheet back.
	again := o.Landscape()
	if again.PageWidth != o.PageWidth || again.PageHeight != o.PageHeight {
		t.Errorf("turning it twice gave %.2f by %.2f", again.PageWidth, again.PageHeight)
	}
}

// How long a line of music the paper holds has to follow from the paper. It
// was a constant, so turning the sheet over bought nothing and a large type
// size drew a system wider than the page.
func TestLineWidthFollowsThePaper(t *testing.T) {
	sp := engrave.DefaultSpacings()

	upright := DefaultOptions()
	sideways := upright.Landscape()
	if w, l := upright.LineWidth(sp), sideways.LineWidth(sp); l <= w {
		t.Errorf("a system is %.2f spaces upright and %.2f on its side", w, l)
	}

	// And a bigger type size means fewer spaces across, not the same number
	// of larger ones running off the edge.
	big := upright
	big.StaffSpace = 12
	if w, b := upright.LineWidth(sp), big.LineWidth(sp); b >= w {
		t.Errorf("at 6.2pt a system is %.2f spaces and at 12pt %.2f", w, b)
	}

	// The music has to stay inside the margins it was given. The engraver
	// leaves its own border at each side, and the staff lines are drawn from
	// one to the other.
	for _, o := range []Options{upright, sideways, big, Letter(), Letter().Landscape()} {
		right := o.MarginLeft + (o.LineWidth(sp)+sp.Margin)*o.StaffSpace
		if right > o.PageWidth-o.MarginRight+0.01 {
			t.Errorf("on %.0fx%.0f paper the music reaches %.2f, past the margin at %.2f",
				o.PageWidth, o.PageHeight, right, o.PageWidth-o.MarginRight)
		}
	}
}

// The page's own box is what a printer reads to decide which way round the
// sheet goes, so it is worth checking it came out of the renderer and not
// just out of the options.
func TestALandscapeScoreIsWrittenOnLandscapePages(t *testing.T) {
	opt := DefaultOptions().Landscape()

	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.LineWidth = opt.LineWidth(eopt.Spacings)
	lay := engrave.Engrave(buildDemo(), eopt)

	var buf bytes.Buffer
	if err := Render(&buf, lay, opt); err != nil {
		t.Fatal(err)
	}
	box := fmt.Sprintf("/MediaBox [0 0 %.2f %.2f]", opt.PageWidth, opt.PageHeight)
	if !bytes.Contains(buf.Bytes(), []byte(box)) {
		t.Errorf("no page declares %s", box)
	}
}
