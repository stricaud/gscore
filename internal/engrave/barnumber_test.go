/*
 * barnumber_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Bar one is the first full bar.
//
// A piece that begins before the first beat has that opening fragment as its
// bar nought, which is how every edition numbers one and what NoteWorthy's
// own ruler shows. Numbered from the first bar whatever it held, Für Elise
// ran one ahead of the file it came from on every page: the bar NoteWorthy
// calls 65 was printed as 66, which is exactly the difference that matters
// when two people are reading from different copies.

// numberedBars builds a part whose bars hold the given durations, in 4/4.
func numberedBars(durs ...model.Duration) *model.Score {
	var measures []*model.Measure
	for _, d := range durs {
		measures = append(measures, &model.Measure{Voices: []*model.Voice{{
			Elements: []model.Element{&model.Rest{Dur: d}},
		}}})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: measures,
		}},
	}
}

// numbersOn is every bar number the engraver wrote, in order.
func numbersOn(s *model.Score) string {
	opt := DefaultOptions()
	opt.Mode = Strip // a strip numbers every bar, a page only the first of a system
	out := ""
	for _, t := range Engrave(s, opt).Texts {
		if len(t.S) == 0 || t.S[0] < '0' || t.S[0] > '9' {
			continue
		}
		if out != "" {
			out += " "
		}
		out += t.S
	}
	return out
}

func TestAPickupIsNotBarOne(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	whole := model.Duration{Num: 1, Den: 1}

	// Short, then three full bars. The short one is bar nought and the three
	// after it are one, two and three; bar one needs no number and nor does
	// nought, so two and three are written, and four is the empty bar a
	// strip always keeps open past the end of the music.
	if got, want := numbersOn(numberedBars(quarter, whole, whole, whole)), "2 3 4"; got != want {
		t.Errorf("with a pickup the bars are numbered %q, want %q", got, want)
	}

	// The same four bars with a full one in front are one to four.
	if got, want := numbersOn(numberedBars(whole, whole, whole, whole)), "2 3 4 5"; got != want {
		t.Errorf("with no pickup the bars are numbered %q, want %q", got, want)
	}
}

// Short is not enough on its own.
//
// A bar still being typed into is short, and so is every bar of a piece
// whose meter gscore has read wrongly — two of the MusicXML suite's rhythm
// cases open with a bar that is short by their own meter and stay short.
// Renumbering those would move every number on the page because the reader
// could not count, so a pickup has to be short and then whole.
func TestAShortOpeningIsNotAlwaysAPickup(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	half := model.Duration{Num: 1, Den: 2}

	if got, want := numbersOn(numberedBars(quarter, half, half, half)), "2 3 4 5"; got != want {
		t.Errorf("a piece of short bars is numbered %q, want %q", got, want)
	}
	// Nor is a single short bar on its own.
	if opensWithPickup(numberedBars(quarter)) {
		t.Error("a one-bar score was taken for a pickup")
	}
}
