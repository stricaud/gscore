/*
 * drumreport_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package guitarpro

import (
	"strings"
	"testing"
)

/*
 * Two different problems, said in two different sentences.
 *
 * They used to share one: "on instruments the standard kit has no staff
 * position for". For a key inside General MIDI's percussion range that is
 * true and is a gap in the kit — keys 48 and 59 were exactly that, and
 * fixing the kit was the answer. For a key outside the range it is not
 * true at all: no kit will ever have a line for it, because it names no
 * instrument, and the sentence sent the reader looking for one.
 */

func reportOf(drums map[int]int) string {
	b := &builder{missed: map[string]int{}, drums: drums}
	return strings.Join(b.report(), "\n")
}

func TestARealDrumWithNoLineIsReportedAsAGapInTheKit(t *testing.T) {
	got := reportOf(map[int]int{57: 3})
	if !strings.Contains(got, "no staff position") {
		t.Errorf("reported %q", got)
	}
	if strings.Contains(got, "General MIDI's") {
		t.Errorf("a real drum was called no instrument at all: %q", got)
	}
}

func TestAKeyThatIsNoInstrumentIsSaidToBeNone(t *testing.T) {
	got := reportOf(map[int]int{0: 14})
	if !strings.Contains(got, "no percussion instrument") {
		t.Errorf("reported %q", got)
	}
	if !strings.Contains(got, "35-81") {
		t.Errorf("the report does not say what the range is: %q", got)
	}
	if strings.Contains(got, "the standard kit has no staff position") {
		t.Errorf("key 0 was reported as a gap in the kit: %q", got)
	}
}

func TestTheTwoAreCountedApart(t *testing.T) {
	// Both at once: the counts must not be added together, or a file with
	// one stray key reads as though a whole kit were missing.
	got := reportOf(map[int]int{0: 14, 57: 3})
	if !strings.Contains(got, "3 drum strokes") {
		t.Errorf("the real drum's count is wrong: %q", got)
	}
	if !strings.Contains(got, "14 strokes") {
		t.Errorf("the stray key's count is wrong: %q", got)
	}
}

func TestTheDrumsThatPromptedThisAreNoLongerReported(t *testing.T) {
	// 48 and 59 are in the kit now, so nothing reaches the report at all:
	// the importer only tallies a key it could not place.
	if got := reportOf(map[int]int{}); got != "" {
		t.Errorf("reported %q with nothing to report", got)
	}
}
