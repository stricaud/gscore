/*
 * roundtrip_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package abc_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"

	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/musescore"
)

/*
 * samples/altes-10e-solo.abc is the whole 10me Solo, written out by gscore's
 * own ABC exporter from a MuseScore transcription of it. This compares it
 * back against that transcription, note for note and mark for mark.
 *
 * A format that can write a file it cannot read back is not a format, and
 * this is how that gets noticed. It found a great deal: every hairpin in the
 * piece, because the importer never copied them out of its pending marks;
 * every trill's wavy line, because a trill was read as a picture rather than
 * as a thing with two ends; every mark written at a rest, because rests were
 * assumed to carry none; both of the key changes and all three changes of
 * meter, because a key or a meter read in the middle of a tune was taken as
 * a statement about the whole staff rather than a change from that bar on —
 * which also left the second and third staves in the meter the piece ends in
 * and beamed their bars accordingly, all 131 of them.
 *
 * The whole piece is a large file and is not kept in the repository. Point at
 * wherever it lives to run this:
 *
 *     ALTES=~/scores/altes.mscz go test ./internal/format/abc
 */
func TestTheWholePieceSurvivesBeingWrittenOut(t *testing.T) {
	source := os.Getenv("ALTES")
	if source == "" {
		t.Skip("set ALTES to the MuseScore file this sample was made from")
	}
	orig := open(t, source)
	mine := open(t, "../../../samples/altes-10e-solo.abc")

	if len(orig.Parts) != len(mine.Parts) {
		t.Fatalf("%d staves, want %d", len(mine.Parts), len(orig.Parts))
	}
	a, b := describeScore(orig), describeScore(mine)
	if len(a) != len(b) {
		t.Fatalf("%d things written out, %d read back", len(a), len(b))
	}
	// What was put in on purpose, and the one thing ABC does not carry.
	//
	// Listed by the bar it happens in rather than tolerated silently, so
	// that a third difference appearing somewhere else says so. The ties
	// that cross from one voice into another used to be here too; they are
	// carried now, because a tie belongs to the staff and not to the line
	// of music it happens to be written in.
	allowed := map[string]string{
		"staff 0 bar   7": "Solo. put back and a modern performance note taken out, per the 1888 edition",
		"staff 1 bar  52": "a bar whose empty middle voice is not written out",
	}
	bad := 0
	for i := range a {
		if a[i] == b[i] {
			continue
		}
		if _, ok := allowed[a[i][:15]]; ok {
			continue
		}
		bad++
		if bad <= 10 {
			t.Errorf("  %s\n  read back as %s", a[i], b[i])
		}
	}
	if bad > 10 {
		t.Errorf("  … and %d more", bad-10)
	}
}

// describeScore reduces a score to one line per note, rest and mark, in the
// order they are played, so that two readings can be compared as text.
func describeScore(s *model.Score) []string {
	var out []string
	for pi, p := range s.Parts {
		for mi, m := range p.Measures {
			at := fmt.Sprintf("staff %d bar %3d:", pi, mi+1)
			if m.Key != nil {
				out = append(out, fmt.Sprintf("%s key %v", at, *m.Key))
			}
			if m.Time != nil {
				out = append(out, fmt.Sprintf("%s meter %v", at, *m.Time))
			}
			if m.Tempo != nil {
				out = append(out, fmt.Sprintf("%s tempo %d", at, m.Tempo.BPM))
			}
			if m.Clef != nil {
				out = append(out, fmt.Sprintf("%s clef %v", at, *m.Clef))
			}
			for vi, v := range m.Voices {
				for _, el := range v.Elements {
					out = append(out, at+fmt.Sprintf(" v%d ", vi)+describeElement(el))
				}
			}
		}
	}
	return out
}

func describeElement(el model.Element) string {
	var b strings.Builder
	switch e := el.(type) {
	case *model.Note:
		fmt.Fprintf(&b, "note %v %v%s", e.Pitch, e.Dur, tieText(e.TieStart, e.TieStop))
	case *model.Chord:
		b.WriteString("chord [")
		for _, n := range e.Notes {
			fmt.Fprintf(&b, "%v%s,", n.Pitch, tieText(n.TieStart, n.TieStop))
		}
		fmt.Fprintf(&b, "] %v", e.Dur)
	case *model.Rest:
		fmt.Fprintf(&b, "rest %v", e.Dur)
	default:
		fmt.Fprintf(&b, "%T", el)
	}
	m := marksOfElement(el)
	if m == nil {
		return b.String()
	}
	for _, a := range m.Articulations {
		fmt.Fprintf(&b, " %v", a)
	}
	if m.Dynamic != model.DynamicNone {
		fmt.Fprintf(&b, " %v", m.Dynamic)
	}
	if m.Hairpin != nil {
		fmt.Fprintf(&b, " hairpin(%v start=%v stop=%v)", m.Hairpin.Kind, m.Hairpin.Start, m.Hairpin.Stop)
	}
	if m.Trill != nil {
		fmt.Fprintf(&b, " trill(start=%v stop=%v)", m.Trill.Start, m.Trill.Stop)
	}
	if m.Tuplet != nil {
		fmt.Fprintf(&b, " tuplet(%d:%d)", m.Tuplet.Actual, m.Tuplet.Normal)
	}
	if m.SlurStart {
		b.WriteString(" slur(")
	}
	if m.SlurStop {
		b.WriteString(" slur)")
	}
	if m.Beam != model.BeamAuto {
		fmt.Fprintf(&b, " beam=%v", m.Beam)
	}
	if m.Clef != nil {
		fmt.Fprintf(&b, " clef=%v", *m.Clef)
	}
	if m.Grace != nil {
		fmt.Fprintf(&b, " grace×%d", len(m.Grace.Notes))
	}
	for _, tx := range m.Texts {
		fmt.Fprintf(&b, " text=%q", tx.Text)
	}
	return b.String()
}

func tieText(start, stop bool) string {
	switch {
	case start && stop:
		return "~both"
	case start:
		return "~start"
	case stop:
		return "~stop"
	}
	return ""
}

func marksOfElement(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	}
	return nil
}

func open(t *testing.T, path string) *model.Score {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	imp, err := format.ImporterFor(path)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := imp.Import(f)
	if s == nil {
		t.Fatalf("%s: nothing could be read", path)
	}
	return s
}
