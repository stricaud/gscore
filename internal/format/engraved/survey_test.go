/*
 * survey_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engraved

import (
	"bytes"
	"os"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
)

// The drawing has to land on the page it came from, or it is worse than
// nothing: an overlay that is merely near the music invites somebody to
// "correct" a reading that was right.
func TestSurveyLandsOnThePage(t *testing.T) {
	f, err := os.Open("testdata/durations.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	defer f.Close()
	sv, err := Look(f, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sv.Pages) == 0 {
		t.Fatal("nothing was surveyed")
	}
	p := sv.Pages[0]
	if p.Width <= 0 || p.Height <= 0 {
		t.Fatalf("page measures %gx%g", p.Width, p.Height)
	}
	if len(p.Staves) == 0 {
		t.Fatal("no staves in the survey")
	}
	for i, st := range p.Staves {
		if st.Inter <= 0 {
			t.Errorf("staff %d has interline %g", i, st.Inter)
		}
		if st.X1 <= st.X0 {
			t.Errorf("staff %d runs from %g to %g", i, st.X0, st.X1)
		}
		for k := 1; k < 5; k++ {
			if st.Y[k] <= st.Y[k-1] {
				t.Errorf("staff %d: line %d is not above line %d", i, k, k-1)
			}
		}
		if st.Y[0] < 0 || st.Y[4] > p.Height {
			t.Errorf("staff %d lies outside the page: %g..%g of %g",
				i, st.Y[0], st.Y[4], p.Height)
		}
	}
	notes := 0
	for _, m := range p.Marks {
		if m.Staff < 0 || m.Staff >= len(p.Staves) {
			t.Fatalf("mark points at staff %d of %d", m.Staff, len(p.Staves))
		}
		if m.Kind == "note" {
			notes++
			if m.Pitch == "" {
				t.Error("a notehead came back without a pitch")
			}
		}
	}
	if notes == 0 {
		t.Error("the survey found no noteheads")
	}
	if len(p.Rules) == 0 {
		t.Error("the survey kept none of the page's strokes")
	}
}

// The pitch written beside a notehead must be the pitch that would be
// imported. A drawing that says one thing while the importer does another
// would hide the very mistakes it is there to show.
func TestSurveyPitchesMatchTheImport(t *testing.T) {
	data, err := os.ReadFile("testdata/durations.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	sv, err := Look(bytes.NewReader(data), Options{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Import(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var drawn []string
	for _, p := range sv.Pages {
		for _, m := range p.Marks {
			if m.Kind == "note" {
				drawn = append(drawn, m.Pitch)
			}
		}
	}
	imported := importedPitches(s)
	if len(drawn) != len(imported) {
		t.Fatalf("the drawing shows %d notes, the import holds %d: %v vs %v",
			len(drawn), len(imported), drawn, imported)
	}
	for i := range drawn {
		if drawn[i] != imported[i] {
			t.Errorf("note %d: drawn %s, imported %s", i+1,
				drawn[i], imported[i])
		}
	}
}

// Told how many staves a system holds, the reader must group by that and not
// guess. The arithmetic is tested on its own, because the fixtures this
// package carries are single-staff pages: on those, every grouping rule
// agrees, which is exactly where a bug would hide.
func TestStavesPerSystemGroupsInBlocks(t *testing.T) {
	five := make([]staff, 6)
	for i := range five {
		five[i].inter = 10
		// bottom line first, and systems are already in reading order
		for k := 0; k < 5; k++ {
			five[i].y[k] = float64(1000 - 100*i + 10*k)
		}
		five[i].x0, five[i].x1 = 0, 500
	}
	for _, c := range []struct {
		per  int
		want []int // staves in each system
	}{
		{1, []int{1, 1, 1, 1, 1, 1}},
		{2, []int{2, 2, 2}},
		{3, []int{3, 3}},
		{4, []int{4, 2}}, // a page that ends mid-system keeps the short block
	} {
		got := systems(&pdf.Content{}, five, c.per)
		var sizes []int
		for _, g := range got {
			sizes = append(sizes, len(g))
		}
		if len(sizes) != len(c.want) {
			t.Errorf("%d per system: %v systems, want %v", c.per, sizes, c.want)
			continue
		}
		for i := range sizes {
			if sizes[i] != c.want[i] {
				t.Errorf("%d per system: %v, want %v", c.per, sizes, c.want)
				break
			}
		}
		// Every staff must land in exactly one system: a grouping that
		// drops one drops its music, silently.
		n := 0
		for _, g := range got {
			n += len(g)
		}
		if n != len(five) {
			t.Errorf("%d per system: %d staves came back of %d",
				c.per, n, len(five))
		}
	}
}

// End to end, on a page that really holds one staff per system: saying so
// must change nothing, and must not lose a note.
func TestTellingItOneStaffPerSystemKeepsTheMusic(t *testing.T) {
	data, err := os.ReadFile("testdata/durations.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	guessed, err := Import(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	told, err := ImportWith(bytes.NewReader(data), Options{StavesPerSystem: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(told.Parts) != 1 {
		t.Errorf("asked for one staff per system, got %d parts",
			len(told.Parts))
	}
	a, b := importedPitches(guessed), importedPitches(told)
	if len(a) != len(b) {
		t.Fatalf("guessing holds %d notes, being told holds %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("note %d: guessed %s, told %s", i+1, a[i], b[i])
		}
	}
	// And the survey must report both numbers, so the drawing can show that
	// the guess and the instruction disagree when they do.
	sv, err := Look(bytes.NewReader(data), Options{StavesPerSystem: 2})
	if err != nil {
		t.Fatal(err)
	}
	if sv.PerSystem != 2 {
		t.Errorf("survey used %d staves per system, asked for 2", sv.PerSystem)
	}
	if sv.Detected == 0 {
		t.Error("the survey does not say what the page-reading guess was")
	}
}

// A PDF with no music must not come back as an empty drawing that looks like
// a successful reading of a blank page.
func TestLookRefusesAPageWithoutStaves(t *testing.T) {
	d := pdf.New()
	p := d.NewPage(300, 200)
	f := d.Standard(pdf.Helvetica)
	p.Text(f, 12, 50, 100, "not music")
	p.Close()
	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		t.Fatal(err)
	}
	sv, err := Look(bytes.NewReader(buf.Bytes()), Options{})
	if err != nil {
		return // refused outright, which is also an answer
	}
	for _, pg := range sv.Pages {
		if len(pg.Staves) != 0 {
			t.Errorf("found %d staves on a page of prose", len(pg.Staves))
		}
	}
	if sv.Parts != 0 {
		t.Errorf("a page of prose surveyed as %d parts", sv.Parts)
	}
}

// importedPitches is every notehead the importer kept, in reading order.
func importedPitches(s *model.Score) []string {
	var out []string
	for _, part := range s.Parts {
		for _, m := range part.Measures {
			for _, v := range m.Voices {
				for _, e := range v.Elements {
					if n, ok := e.(*model.Note); ok {
						out = append(out, lower(n.Pitch.Step.String())+
							itoa(n.Pitch.Octave))
					}
				}
			}
		}
	}
	return out
}

func countNotes(s *model.Score) int { return len(importedPitches(s)) }
