/*
 * voicestems_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which way the stems point when a staff carries two lines.
 *
 * The upper line takes its stems up and the lower takes them down, however
 * high or low either happens to sit. That is what tells a reader which note
 * belongs to which line, and it is the whole reason two voices can share a
 * staff at all.
 *
 * Usually the first voice is the upper one, which is what the numbering
 * means. Not always: a part may write the held chords as the first voice
 * and the moving line as the second, and then the numbering is the wrong
 * way round and the stems follow the music rather than the numbers.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

func pitchAt(s model.Step, octave int) model.Pitch {
	return model.Pitch{Step: s, Octave: octave}
}

func noteAt(s model.Step, octave int, d model.Duration) *model.Note {
	return &model.Note{Pitch: pitchAt(s, octave), Dur: d}
}

func stackAt(d model.Duration, ps ...model.Pitch) *model.Chord {
	var notes []*model.Note
	for _, p := range ps {
		notes = append(notes, &model.Note{Pitch: p, Dur: d})
	}
	return &model.Chord{Notes: notes, Dur: d}
}

// twoVoices is one bar of cut time with two lines written on the staff.
func twoVoices(first, second []model.Element) *model.Score {
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 2, BeatValue: 2},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{
					{Elements: first}, {Elements: second},
				},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// stemThrough is the stem that runs through a notehead, which for a chord is
// not the same thing as one starting at it: the stem grows out of the head
// at one end and the others merely stand along it.
//
// It reports whether that stem points up. Y1 is the head it grows out of and
// Y2 the free end, so the comparison is the direction itself.
func stemThrough(t *testing.T, lay *Layout, g Glyph) bool {
	t.Helper()
	found, up := 0, false
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) > 0.01 {
			continue // horizontal: a staff line, a ledger line or a beam
		}
		if l.X1 < g.X-0.3 || l.X1 > g.X+2 {
			continue
		}
		top, bottom := math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2)
		if g.Y < top-0.2 || g.Y > bottom+0.2 {
			continue
		}
		found, up = found+1, l.Y2 < l.Y1
	}
	if found != 1 {
		t.Fatalf("%d stems run through the notehead at y=%.2f", found, g.Y)
	}
	return up
}

// outerHeads are the lowest and highest noteheads on the page, which in a
// bar of two voices are one from each line.
func outerHeads(t *testing.T, lay *Layout) (low, high Glyph) {
	t.Helper()
	first := true
	for _, g := range lay.Glyphs {
		if !has(g.Name, "notehead") {
			continue
		}
		if first {
			low, high, first = g, g, false
			continue
		}
		if g.Y > low.Y { // further down the page is lower in pitch
			low = g
		}
		if g.Y < high.Y {
			high = g
		}
	}
	if first {
		t.Fatal("no noteheads were drawn")
	}
	return low, high
}

// The case the rule is for: held chords written as the first voice with the
// moving line above them as the second.
func TestTheUpperLineStemsUpEvenWrittenAsTheSecondVoice(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	quarter := model.Duration{Num: 1, Den: 4}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(twoVoices(
		[]model.Element{
			stackAt(half, pitchAt(model.StepE, 4), pitchAt(model.StepG, 4)),
			stackAt(half, pitchAt(model.StepD, 4), pitchAt(model.StepF, 4)),
		},
		[]model.Element{
			noteAt(model.StepD, 5, quarter), noteAt(model.StepC, 5, quarter),
			noteAt(model.StepD, 5, quarter), noteAt(model.StepC, 5, quarter),
		},
	), opt)

	low, high := outerHeads(t, lay)
	if stemThrough(t, lay, low) {
		t.Error("the lower line stems up, into the line above it")
	}
	if !stemThrough(t, lay, high) {
		t.Error("the upper line stems down, into the line below it")
	}
}

// Touching is not crossing.
//
// This asked for every note of the one voice strictly below every note of
// the other, so two lines sharing a single pitch — at different moments —
// read as a crossing and turned every stem in the bar round. Writing one B
// into the upper line of a right hand whose lower line already touched that
// B flipped the whole bar: reported as a bar that went unreadable the
// moment a note was added to it.
func TestVoicesThatTouchAtOnePitchAreStillNotCrossed(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	quarter := model.Duration{Num: 1, Den: 4}
	eighth := model.Duration{Num: 1, Den: 8}
	opt := DefaultOptions()
	opt.Mode = Strip
	// The lower line reaches B4 on the first beat; the upper line comes
	// down to the same B4 at the end. They meet and never cross.
	lay := Engrave(twoVoices(
		[]model.Element{
			stackAt(half, pitchAt(model.StepE, 4), pitchAt(model.StepB, 4)),
			stackAt(half, pitchAt(model.StepD, 4), pitchAt(model.StepG, 4)),
		},
		[]model.Element{
			noteAt(model.StepD, 5, quarter), noteAt(model.StepC, 5, quarter),
			noteAt(model.StepB, 4, eighth),
		},
	), opt)

	low, high := outerHeads(t, lay)
	if stemThrough(t, lay, low) {
		t.Error("one note added to the upper line turned the lower line's " +
			"stems up through the chord above them")
	}
	if !stemThrough(t, lay, high) {
		t.Error("one note added to the upper line turned its own stems down " +
			"into the chord below them")
	}
}

// And a real crossing is not an inversion. The numbering is what tells the
// reader which line is which, so where the lines genuinely cross it is the
// numbering that settles the stems.
func TestVoicesThatCrossKeepTheirNumbering(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(twoVoices(
		[]model.Element{
			noteAt(model.StepC, 4, quarter), noteAt(model.StepE, 5, quarter),
			noteAt(model.StepC, 4, quarter), noteAt(model.StepE, 5, quarter),
		},
		[]model.Element{
			noteAt(model.StepG, 5, quarter), noteAt(model.StepD, 4, quarter),
			noteAt(model.StepG, 5, quarter), noteAt(model.StepD, 4, quarter),
		},
	), opt)

	// The first voice is the first voice wherever it happens to sit, so
	// its lowest note stems up with the rest of its line.
	low, _ := outerHeads(t, lay)
	if !stemThrough(t, lay, low) {
		t.Error("crossing lines were read as the numbering being backwards")
	}
}

// Two voices that are the same one note are not an inversion either: there
// is no upper and no lower, so the numbering stands.
func TestAUnisonInTwoVoicesIsNotAnInversion(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(twoVoices(
		[]model.Element{noteAt(model.StepB, 4, half), noteAt(model.StepB, 4, half)},
		[]model.Element{noteAt(model.StepB, 4, half), noteAt(model.StepB, 4, half)},
	), opt)

	low, _ := outerHeads(t, lay)
	if !stemThrough(t, lay, low) {
		t.Error("the first voice of a unison stems down")
	}
}

/*
 * And room for the flag that direction implies.
 *
 * A flag hangs off the right of an up stem, past where the note's duration
 * says its ink stops, so the column has to be widened for it. Which notes
 * need that is a question about the whole bar — an eighth in a beam has no
 * flag at all, and one with its stem turned down hangs its flag over its
 * own head and asks for nothing — and the room was being worked out one
 * voice at a time. A melody written as the second voice of a piano right
 * hand, read by itself, is a run of high notes and stems down, so nothing
 * was booked; drawn as the upper line of the staff it stems up, and its
 * flag came down over the note after it. Reported as "there is a collision
 * between the 8th and the quarter".
 */
func TestAFlagInTheUpperVoiceIsGivenItsRoom(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	quarter := model.Duration{Num: 1, Den: 4}
	eighth := model.Duration{Num: 1, Den: 8}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(twoVoices(
		[]model.Element{
			stackAt(half, pitchAt(model.StepE, 4), pitchAt(model.StepB, 4)),
			stackAt(half, pitchAt(model.StepD, 4), pitchAt(model.StepG, 4)),
		},
		[]model.Element{
			noteAt(model.StepD, 5, quarter), noteAt(model.StepC, 5, quarter),
			// A dotted quarter after it, not another eighth: two eighths on
			// one beat are beamed together and a beamed note has no flag.
			noteAt(model.StepB, 4, eighth),
			noteAt(model.StepD, 5, model.Duration{Num: 3, Den: 8}),
		},
	), opt)

	var flagged, after *ElementPos
	for _, h := range lay.Hits {
		for i, el := range h.Elements {
			if el.Voice != 1 {
				continue
			}
			switch el.Index {
			case 2:
				flagged = &h.Elements[i]
			case 3:
				after = &h.Elements[i]
			}
		}
	}
	if flagged == nil || after == nil {
		t.Fatal("the upper voice was not drawn as four notes")
	}
	// What drawSingleStem will put there: the stem stands at the right edge
	// of the head and the flag hangs off it.
	need := noteheadWidth(model.Eighth) - opt.Defaults.StemThickness/2 +
		smufl.Advance("flag8thUp")
	if got := after.X - flagged.X; got < need {
		t.Errorf("the note after the flagged eighth is %.2f away and the "+
			"flag reaches %.2f: it is drawn under the curl", got, need)
	}
}
