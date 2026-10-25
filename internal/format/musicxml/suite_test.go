/*
 * suite_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/format/musicxml"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * The W3C MusicXML test suite, read and engraved on every run.
 *
 * 182 files, each one written to exercise a single feature and nothing else.
 * Where unit tests say "this is what I thought of", a suite like this says
 * "this is what the format actually contains" — and the difference is where
 * the bugs are. Every gap found so far was found here and nowhere else:
 * microtones written as decimals, composite time signatures written "3+2",
 * voices named "X", grace notes, and drums with a position instead of a
 * pitch. Each of those ended the reading of a whole file at the first one.
 *
 * testdata/suite/README.md says where the files came from.
 */

const suiteDir = "testdata/suite"

func suiteFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(suiteDir, "*.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 150 {
		t.Fatalf("only %d files in %s: the suite is not there", len(files), suiteDir)
	}
	sort.Strings(files)
	return files
}

// read imports one file the way the program does, keeping whatever came
// through even when some of it did not.
func read(t *testing.T, path string) (*model.Score, []string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := musicxml.Import(bytes.NewReader(b))
	if err == nil {
		return s, nil
	}
	var partial format.Partial
	if !asPartial(err, &partial) {
		return s, []string{"FAILED: " + err.Error()}
	}
	return s, partial.Details()
}

func asPartial(err error, out *format.Partial) bool {
	p, ok := err.(format.Partial)
	if ok {
		*out = p
	}
	return ok
}

// A file gscore cannot read in full still has to give back the music that
// was in it. Refusing the whole file over one thing it did not recognise
// loses work that nobody else has a copy of.
func TestNothingInTheSuiteIsRefused(t *testing.T) {
	var refused []string
	for _, f := range suiteFiles(t) {
		s, notes := read(t, f)
		name := filepath.Base(f)
		if s == nil {
			refused = append(refused, name+": nothing came back")
			continue
		}
		for _, n := range notes {
			if strings.HasPrefix(n, "FAILED: ") {
				refused = append(refused, name+": "+strings.TrimPrefix(n, "FAILED: "))
			}
		}
	}
	if len(refused) > 0 {
		t.Errorf("%d file(s) were refused outright:", len(refused))
		for _, r := range refused {
			t.Errorf("   %s", r)
		}
	}
}

// And everything that is read has to engrave: no panic, and something on the
// page at the end of it.
func TestEverythingInTheSuiteEngraves(t *testing.T) {
	for _, f := range suiteFiles(t) {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			s, _ := read(t, f)
			if s == nil {
				t.Skip("nothing was read; the refusal test covers that")
			}
			lay := engrave.Engrave(s, engrave.DefaultOptions())
			if lay == nil {
				t.Fatal("engraving produced no layout")
			}
			if lay.Width <= 0 || lay.Height <= 0 {
				t.Errorf("the drawing is %gx%g", lay.Width, lay.Height)
			}
			if len(lay.Glyphs) == 0 && len(lay.Lines) == 0 {
				t.Error("nothing was drawn at all")
			}
		})
	}
}

// noteheadsDrawn counts what ended up on the page.
func noteheadsDrawn(lay *engrave.Layout) int {
	n := 0
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") {
			n++
		}
	}
	return n
}

var (
	noteTag = regexp.MustCompile(`<note[ >]`)
	restTag = regexp.MustCompile(`<rest\b`)
)

// noteheadsWanted counts what the file asked for: every note that is not a
// rest wants a notehead, grace notes and drums included.
func noteheadsWanted(b []byte) int {
	return len(noteTag.FindAll(b, -1)) - len(restTag.FindAll(b, -1))
}

// shortBy is every file gscore does not yet draw in full, and how many
// noteheads it manages. The list is the honest state of the importer: it
// only ever shrinks, and a file that drops below its number has regressed.
//
// All of what is left is the same gap. A grace group in gscore hangs off the
// note it leans on, so grace notes written after that note — or at the very
// end of a bar, with nothing after them at all — have nowhere to hang. They
// are counted and logged rather than dropped in silence.
var shortBy = map[string]int{
	"24a-GraceNotes.musicxml":              25, // of 26
	"24c-GraceNote-MeasureEnd.musicxml":    2,  // of 4
	"24d-AfterGrace.musicxml":              5,  // of 7
	"24g-GraceNote-Dynamics.musicxml":      1,  // of 5
	"24h-GraceNote-Simultaneous.musicxml":  7,  // of 8
	"33f-Trill-EndingOnGraceNote.musicxml": 5,  // of 7
}

// The one that finds real bugs: count the noteheads in the file, count the
// noteheads on the page, and make them agree. A feature that is quietly not
// imported shows up here as music that went missing, which is the only
// symptom that matters.
func TestEveryNoteheadArrives(t *testing.T) {
	improved := map[string]int{}
	for _, f := range suiteFiles(t) {
		name := filepath.Base(f)
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		want := noteheadsWanted(b)
		if want <= 0 {
			continue
		}
		s, _ := read(t, f)
		if s == nil {
			continue
		}
		got := noteheadsDrawn(engrave.Engrave(s, engrave.DefaultOptions()))

		if least, known := shortBy[name]; known {
			if got < least {
				t.Errorf("%s drew %d noteheads of %d, and used to draw %d: "+
					"this has gone backwards", name, got, want, least)
			}
			if got > least {
				improved[name] = got
			}
			continue
		}
		if got < want {
			t.Errorf("%s drew %d noteheads, and the file holds %d: "+
				"something in it is not being read", name, got, want)
		}
	}

	// Good news is still news: a gap that has closed should come off the
	// list, or the list stops being the truth.
	for name, got := range improved {
		t.Errorf("%s now draws %d noteheads, more than the %d recorded in "+
			"shortBy — raise or remove its entry", name, got, shortBy[name])
	}
}
