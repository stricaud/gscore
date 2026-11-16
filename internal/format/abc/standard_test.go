/*
 * standard_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package abc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The decorations of the ABC standard, as abcm2ps draws them.
 *
 * Each one that gscore has a mark of its own for is read as that mark and
 * written back as the same name, so that a score full of them goes round
 * through ABC whole. They used to come in as glyphs by name, or as names
 * no font has, which drew something or nothing and never played.
 */

func TestTheStandardsDecorationsAreReadAsMarks(t *testing.T) {
	src := "X:1\nM:4/4\nL:1/8\nK:C\n" +
		"!arpeggio![ceg]2 !breath!c2 !ped!c2 !ped-up!c2 | !8va(!c2 !8va)!c2 !1!c !3!c !trem2!c2 |\n" +
		"!mordent!c !turn!c !invertedturn!c !uppermordent!c !wedge!c !invertedfermata!c Mc !sfp!c |\n" +
		".(c d) !courtesy!=c6 |\n"
	s, err := Import(strings.NewReader(src))
	if s == nil {
		t.Fatal(err)
	}
	bars := s.Parts[0].Measures
	mk := func(bar, i int) *model.Marks {
		el := bars[bar].Voices[0].Elements[i]
		switch e := el.(type) {
		case *model.Note:
			return &e.Marks
		case *model.Chord:
			return &e.Marks
		}
		t.Fatalf("bar %d element %d is %T", bar+1, i+1, el)
		return nil
	}
	if mk(0, 0).Arpeggio == nil {
		t.Error("!arpeggio! did not make an arpeggio")
	}
	if mk(0, 1).Break != model.BreathComma {
		t.Error("!breath! did not make a breath mark")
	}
	if p := mk(0, 2).Pedal; p == nil || !p.Start {
		t.Error("!ped! did not start a pedal")
	}
	if p := mk(0, 3).Pedal; p == nil || !p.Stop {
		t.Error("!ped-up! did not end the pedal")
	}
	if o := mk(1, 0).Octave; o == nil || !o.Start || o.Amount != 1 {
		t.Errorf("!8va(! came out as %+v", o)
	}
	if o := mk(1, 1).Octave; o == nil || !o.Stop {
		t.Errorf("!8va)! came out as %+v", o)
	}
	if f := mk(1, 2).Fingering; len(f) != 1 || f[0] != "1" {
		t.Errorf("!1! came out as %v", f)
	}
	if tr := mk(1, 4).Tremolo; tr == nil || tr.Strokes != 2 || tr.Start {
		t.Errorf("!trem2! came out as %+v", tr)
	}
	want := []model.Articulation{model.Mordent, model.Turn, model.TurnInverted, model.ShortTrill,
		model.Staccatissimo, model.Fermata, model.Mordent}
	for i, a := range want {
		if got := mk(2, i).Articulations; len(got) != 1 || got[0] != a {
			t.Errorf("ornament %d came out as %v, want %v", i+1, got, a)
		}
	}
	if pl := mk(2, 5).ArticulationPlace; len(pl) != 1 || pl[0] != model.PlacementBelow {
		t.Errorf("!invertedfermata! is placed %v, want below", pl)
	}
	if mk(2, 7).Dynamic != model.FortePiano {
		t.Errorf("!sfp! came out as %v", mk(2, 7).Dynamic)
	}
	// A dotted slur is a slur, not a staccato on its first note.
	if m := mk(3, 0); m.SlurStart == 0 || len(m.Articulations) != 0 {
		t.Errorf(".( came out as slur=%d articulations=%v", m.SlurStart, m.Articulations)
	}
	if n := bars[3].Voices[0].Elements[2].(*model.Note); !n.Parens || n.Accidental != model.Natural {
		t.Errorf("!courtesy! came out as parens=%v accidental=%v", n.Parens, n.Accidental)
	}

	// And they all go back out under their own names.
	var out bytes.Buffer
	if err := Export(&out, s); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"!arpeggio!", "!breath!", "!ped!", "!ped-up!", "!8va(!", "!8va)!",
		"!1!", "!3!", "!trem2!", "!mordent!", "!turn!", "!invertedturn!", "!uppermordent!",
		"!staccatissimo!", "!invertedfermata!", "!fp!", "!courtesy!=c6"} {
		if !strings.Contains(out.String(), name) {
			t.Errorf("%s did not go back out:\n%s", name, out.String())
		}
	}
}
