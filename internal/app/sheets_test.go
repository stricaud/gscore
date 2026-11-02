/*
 * sheets_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The parts.
 *
 * The one thing that goes wrong with parts in every program that has them
 * is that the score and the part come to disagree: a note fixed in one and
 * not the other, and nobody finds out until the rehearsal. A part here
 * holds no music, so most of what is checked below is that it still holds
 * none — that showing one is a view and not a copy, and that editing
 * through one reaches the score.
 */

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
)

func sheetsNow(t *testing.T, h *harness) (int, []sheetInfo) {
	t.Helper()
	var got struct {
		Showing int         `json:"showing"`
		Sheets  []sheetInfo `json:"sheets"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/sheets")), &got); err != nil {
		t.Fatalf("the endpoint did not answer: %v", err)
	}
	return got.Showing, got.Sheets
}

// A score opens as itself, however many parts it carries.
func TestAScoreOpensAsTheScore(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	showing, sheets := sheetsNow(t, h)
	if showing != -1 {
		t.Errorf("it opened showing part %d", showing)
	}
	if len(sheets) != 0 {
		t.Errorf("a new score came with %d parts", len(sheets))
	}
}

// One part per player: one per staff, except where a braced group with its
// barlines run through says one player reads several of them.
func TestAPartIsMadeForEveryPlayer(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/addstaff?instrument=piano&clef=treble")
	h.post("/api/addstaff?instrument=piano&clef=bass")
	// The two hands of the piano, braced, with the barlines through the gap:
	// one player, one part.
	h.post("/api/group?first=2&last=3&symbol=brace&barlines=1")

	h.post("/api/sheets?make=1")
	_, sheets := sheetsNow(t, h)
	if len(sheets) != 3 {
		var names []string
		for _, s := range sheets {
			names = append(names, s.Name)
		}
		t.Fatalf("%d parts for four staves: %v", len(sheets), names)
	}
	if len(sheets[2].Staves) != 2 {
		t.Errorf("the piano got %v, want both its staves in one part",
			sheets[2].Staves)
	}
	for _, s := range sheets {
		if !s.MultiRests {
			t.Errorf("%q prints its bars of rest one by one", s.Name)
		}
		if s.Name == "" {
			t.Error("a part with no name")
		}
	}
}

// Made once. Running it again would throw away whatever was named or
// re-laid-out, which is the work it was meant to save.
func TestThePartsAreNotMadeTwice(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")
	_, was := sheetsNow(t, h)
	h.post("/api/sheets?at=0&name=Flauto")

	if why := h.postFails("/api/sheets?make=1"); why == "" {
		t.Error("a second asking was accepted")
	}
	_, now := sheetsNow(t, h)
	if len(now) != len(was) || now[0].Name != "Flauto" {
		t.Errorf("the parts are now %+v", now)
	}
}

// A part shows its own staves and nobody else's.
func TestAPartShowsItsOwnStavesAlone(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.note(0, 0, 2, "quarter")
	h.note(1, 0, 4, "quarter")
	h.post("/api/sheets?make=1")

	h.post("/api/sheets?show=1")
	showing, sheets := sheetsNow(t, h)
	if showing != 1 || !sheets[1].Showing {
		t.Fatalf("asked for part 1 and got %d", showing)
	}

	// The copy the engraver is handed: the part's staff visible and the
	// rest hidden. Hidden rather than removed, so that a note clicked on a
	// part still goes into the staff the score calls that one.
	view := sheetOf(h.app.score, 1)
	if view == h.app.score {
		t.Fatal("showing a part handed the engraver the score itself")
	}
	for i, p := range view.Parts {
		if want := i != 1; p.Hidden != want {
			t.Errorf("staff %d is hidden=%v, want %v", i, p.Hidden, want)
		}
	}
	if len(view.Parts) != len(h.app.score.Parts) {
		t.Errorf("the part has %d staves and the score %d: a staff was "+
			"removed, so a note clicked on the part would land on the wrong one",
			len(view.Parts), len(h.app.score.Parts))
	}
}

// Showing a part is not an edit. Which part is being looked at belongs to
// the window, not to the file: a score that went unsaved because somebody
// turned to the oboe part would be a score nobody could close.
func TestShowingAPartIsNotAnEdit(t *testing.T) {
	h := newHarness(t)
	h.app.local = true // saving to a path is this machine's business
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.note(0, 0, 2, "quarter")
	h.post("/api/sheets?make=1")
	h.post("/api/save?path=" + t.TempDir() + "/x.gscore")

	before := h.app.unsaved
	h.post("/api/sheets?show=0")
	if h.app.unsaved != before {
		t.Error("turning to a part made the score unsaved")
	}
}

// The music is the score's. A part holds no notes at all, which is why a
// score and a part can never disagree about one.
func TestAPartHoldsNoMusic(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.note(1, 0, 4, "quarter")
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?show=1")

	// A note written while a part is showing goes into the score, where the
	// full score will find it.
	h.note(1, 1, 5, "quarter")

	found := 0
	for _, m := range h.app.score.Parts[1].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if _, ok := el.(*model.Note); ok {
					found++
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("the score has %d notes on that staff, want 2: an edit made "+
			"through a part did not reach the score", found)
	}
}

// A part lays itself out: its rests counted, its own page turns, no staff
// names down the left.
func TestAPartLaysItselfOut(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")

	sh := h.app.score.Sheets[0]
	if !sh.MultiRests {
		t.Error("a part prints its bars of rest one by one")
	}
	if sh.StaffSpace <= 0 {
		t.Error("a part is drawn at the score's size, which is too small to " +
			"read at a stand")
	}
	if sh.Numbering.Every == 0 {
		t.Error("a part numbers its bars wherever the engraver likes, which " +
			"is no use to somebody who has just counted thirty-two of them")
	}
}

// Its own page turns, in place of the score's. The score's fall wherever
// its fuller systems ran out of room and mean nothing on a part's page.
func TestAPartDoesNotInheritTheScoresPageTurns(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.bars(8)
	h.post("/api/sheets?make=1")
	h.post("/api/systems?at=4&on=1") // a break the score wanted

	h.app.score.Sheets[0].Breaks = []int{2}
	view := sheetOf(h.app.score, 0)
	for _, p := range view.Parts {
		for i, m := range p.Measures {
			if want := i == 2; m.SystemBreak != want {
				t.Errorf("bar %d breaks=%v in the part, want %v",
					i, m.SystemBreak, want)
			}
		}
	}
	// And the score keeps its own.
	if !h.app.score.Parts[0].Measures[4].SystemBreak {
		t.Error("making the part took the score's break away")
	}
}

// A part can be renamed, told not to count its rests, and taken away.
func TestAPartCanBeChangedAndRemoved(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")
	_, sheets := sheetsNow(t, h)
	if len(sheets) != 2 {
		t.Fatalf("%d parts", len(sheets))
	}

	h.post("/api/sheets?at=0&name=Flauto%20primo&multirest=0")
	_, sheets = sheetsNow(t, h)
	if sheets[0].Name != "Flauto primo" || sheets[0].MultiRests {
		t.Errorf("the part is now %+v", sheets[0])
	}

	h.post("/api/sheets?show=1")
	h.post("/api/sheets?at=1&remove=1")
	showing, sheets := sheetsNow(t, h)
	if len(sheets) != 1 {
		t.Errorf("%d parts after removing one", len(sheets))
	}
	// The one being shown was the one removed, so there is nothing to show
	// but the score.
	if showing != -1 {
		t.Errorf("it is showing part %d, which is not there any more", showing)
	}
}

func TestNonsenseAboutPartsIsRefused(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")
	for _, q := range []string{
		"show=9", "show=abc", "at=9&name=x", "at=abc&name=x", "at=0&name=",
		"name=x",
	} {
		if why := h.postFails("/api/sheets?" + q); why == "" {
			t.Errorf("%q was accepted", q)
		}
	}
	_, sheets := sheetsNow(t, h)
	if len(sheets) != 2 || sheets[0].Name == "" {
		t.Errorf("a refused request changed something: %+v", sheets)
	}
}

// A part survives being written to a file and read back, because somebody
// named it, sized it and laid it out and would not want to do that twice.
func TestThePartsSurviveTheFile(t *testing.T) {
	h := newHarness(t)
	h.app.local = true
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&name=Flauto")
	h.app.score.Sheets[0].Breaks = []int{3}

	path := filepath.Join(t.TempDir(), "parts.gscore")
	h.post("/api/save?path=" + path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("nothing was written: %v", err)
	}
	defer f.Close()
	back, _, err := gscore.Read(f)
	if err != nil {
		t.Fatalf("it could not be read back: %v", err)
	}

	if len(back.Sheets) != 2 {
		t.Fatalf("%d parts came back", len(back.Sheets))
	}
	got := back.Sheets[0]
	if got.Name != "Flauto" {
		t.Errorf("it is called %q", got.Name)
	}
	if len(got.Breaks) != 1 || got.Breaks[0] != 3 {
		t.Errorf("its page turns came back as %v", got.Breaks)
	}
	if !got.MultiRests || got.StaffSpace <= 0 {
		t.Errorf("its layout came back as %+v", got)
	}
}

// Undo restores the score it was taken from, parts and all, and a part
// whose list of staves was shared with the snapshot would have its staves
// changed by the edit that was supposed to have been undone.
func TestAPartIsCopiedWithTheScore(t *testing.T) {
	s := model.NewScore()
	s.Sheets = []model.Sheet{{
		Name: "Flute", Staves: []int{0}, Breaks: []int{2, 7},
		Engraving: map[string]float64{"margin": 3},
	}}
	out := s.Clone()
	out.Sheets[0].Name = "changed"
	out.Sheets[0].Staves[0] = 9
	out.Sheets[0].Breaks[0] = 99
	out.Sheets[0].Engraving["margin"] = 9

	got := s.Sheets[0]
	if got.Name != "Flute" || got.Staves[0] != 0 || got.Breaks[0] != 2 ||
		got.Engraving["margin"] != 3 {
		t.Errorf("the copy and the original share something: %+v", got)
	}
}

// A part is called something a player would read off a title page. Where
// the staff was never named the instrument is the best guess there is, and
// it is capitalised: "Flute" on a part, not the "flute" an instrument table
// calls it.
func TestAPartIsNamedTheWayAPartIsNamed(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")
	_, sheets := sheetsNow(t, h)

	for _, s := range sheets {
		if s.Name == "" {
			t.Fatal("a part with no name")
		}
		if first := s.Name[:1]; first != strings.ToUpper(first) {
			t.Errorf("a part is called %q, which is not how a part is named "+
				"on its own title page", s.Name)
		}
	}
}

// Printing the parts prints the parts, where somebody has made them: those
// are the documents that were named and laid out for the stands.
func TestPrintingThePartsUsesThePartsThatWereMade(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.note(0, 0, 2, "quarter")
	h.note(1, 0, 4, "quarter")

	// Before they are made, a staff apiece: the best guess there is.
	before := h.get("/print?what=parts")
	if !strings.Contains(before, "Flute") {
		t.Error("printing offered no flute part at all")
	}

	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=1&name=Flauto%20solo")
	after := h.get("/print?what=parts")
	if !strings.Contains(after, "Flauto solo") {
		t.Error("printing ignored the part that was made and named, and " +
			"printed a staff apiece instead")
	}
}

/*
 * The windows edit what is on the screen.
 *
 * A part turns pages in different places from the score, is engraved larger
 * and numbers its bars more often. Rather than a second set of windows for
 * all of that, the windows that already set those things write to whichever
 * document is being looked at — which is the only rule anybody could guess:
 * somebody who has turned to the oboe part and moves a line break has moved
 * the oboe's.
 */

// Where the lines end, on a part, is the part's own.
func TestALineBreakGoesToThePartBeingLookedAt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.bars(8)
	h.post("/api/sheets?make=1")

	// On the score first.
	h.post("/api/systems?at=4&on=1")
	if !h.app.score.Parts[0].Measures[4].SystemBreak {
		t.Fatal("the score's own break was not written")
	}

	// Then on a part: its own list, and the score untouched.
	h.post("/api/sheets?show=1")
	h.post("/api/systems?at=2&on=1")
	sh := h.app.score.Sheets[1]
	if len(sh.Breaks) != 1 || sh.Breaks[0] != 2 {
		t.Errorf("the part's lines begin at %v, want [2]", sh.Breaks)
	}
	if h.app.score.Parts[0].Measures[2].SystemBreak {
		t.Error("a break set on a part was written into the score as well")
	}

	// And the window says whose lines it is showing, so that moving one is
	// not a surprise.
	var got struct {
		Breaks []int  `json:"breaks"`
		Of     string `json:"of"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/systems")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Of == "" {
		t.Error("the window does not say which part's lines these are")
	}
	if len(got.Breaks) != 1 || got.Breaks[0] != 2 {
		t.Errorf("it reports %v", got.Breaks)
	}

	// Taking it away again takes away the part's and not the score's.
	h.post("/api/systems?at=2&on=0")
	if len(h.app.score.Sheets[1].Breaks) != 0 {
		t.Errorf("the break survived being removed: %v",
			h.app.score.Sheets[1].Breaks)
	}
	if !h.app.score.Parts[0].Measures[4].SystemBreak {
		t.Error("the score lost the break it had")
	}
}

// Fitting a stretch to so many bars a line does the same, on whichever
// document is being looked at.
func TestFittingLinesGoesToThePartBeingLookedAt(t *testing.T) {
	h := newHarness(t)
	h.bars(12)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?show=0")
	h.post("/api/systems?fit=4")

	if got := h.app.score.Sheets[0].Breaks; len(got) != 2 ||
		got[0] != 4 || got[1] != 8 {
		t.Errorf("the part's lines begin at %v, want [4 8]", got)
	}
	for i, m := range h.app.score.Parts[0].Measures {
		if m.SystemBreak {
			t.Errorf("the score was laid out too, at bar %d", i)
		}
	}
}

// The engraving measurements, likewise: a part is engraved to be read at a
// stand and the score at a desk, so they want different answers.
func TestEngravingSettingsGoToThePartBeingLookedAt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")

	h.post("/api/engraving?margin=4")
	if h.app.score.Engraving["margin"] != 4 {
		t.Fatalf("the score's own margin is %v", h.app.score.Engraving)
	}

	h.post("/api/sheets?show=1")
	h.post("/api/engraving?margin=9")
	if got := h.app.score.Sheets[1].Engraving["margin"]; got != 9 {
		t.Errorf("the part's margin is %v", got)
	}
	if h.app.score.Engraving["margin"] != 4 {
		t.Error("setting it on a part changed the score's")
	}

	// And the window shows what it would change, with whose it is.
	var got struct {
		Of    string `json:"of"`
		Knobs []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
			Set   bool    `json:"set"`
		} `json:"knobs"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/engraving")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Of == "" {
		t.Error("the window does not say whose measurements these are")
	}
	for _, k := range got.Knobs {
		if k.Name == "margin" && (!k.Set || k.Value != 9) {
			t.Errorf("it shows margin as %+v, which is the score's and not "+
				"the part's", k)
		}
	}

	// Resetting resets the part alone.
	h.post("/api/engraving?reset=1")
	if h.app.score.Sheets[1].Engraving != nil {
		t.Error("the part kept its measurements")
	}
	if h.app.score.Engraving["margin"] != 4 {
		t.Error("resetting a part reset the score")
	}
}

// How often the bars are numbered is a part's own. Where the count begins
// again is not: a part that called bar forty something else would be
// unrehearsable with the score it came from.
func TestNumberingGoesToThePartButRestartsStayTheScoresOwn(t *testing.T) {
	h := newHarness(t)
	h.bars(8)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?show=0")

	h.post("/api/numbering?every=5&boxed=1")
	sh := h.app.score.Sheets[0]
	if sh.Numbering.Every != 5 || !sh.Numbering.Boxed {
		t.Errorf("the part numbers its bars %+v", sh.Numbering)
	}
	if h.app.score.Numbering.Every != 0 {
		t.Error("the score was numbered too")
	}

	h.post("/api/numbering?at=4&number=1")
	if h.app.score.Parts[0].Measures[4].NumberFrom == nil {
		t.Error("a restart asked for while a part was showing went nowhere: " +
			"what a bar is called has to be the same in the score and in " +
			"every part, or nobody can rehearse from them together")
	}
}

// What a part wants of its own page, set from one window.
func TestAPartsOwnPageCanBeSet(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/addstaff?instrument=oboe&clef=treble")
	h.post("/api/sheets?make=1")

	h.post("/api/sheets?at=0&name=Piano&staves=1,2&size=12&multirest=0")
	_, sheets := sheetsNow(t, h)
	got := sheets[0]
	if got.Name != "Piano" {
		t.Errorf("it is called %q", got.Name)
	}
	if len(got.Staves) != 2 || got.Staves[0] != 0 || got.Staves[1] != 1 {
		t.Errorf("it prints staves %v", got.Staves)
	}
	if got.StaffSpace != 12 {
		t.Errorf("it is drawn at %v", got.StaffSpace)
	}
	if got.MultiRests {
		t.Error("it still counts its bars of rest")
	}

	// And the staves it prints really are the ones shown.
	view := sheetOf(h.app.score, 0)
	for i, p := range view.Parts {
		if want := i > 1; p.Hidden != want {
			t.Errorf("staff %d hidden=%v, want %v", i, p.Hidden, want)
		}
	}
}

func TestNonsenseAboutAPartsPageIsRefused(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/sheets?make=1")
	for _, q := range []string{
		"at=0&size=1", "at=0&size=99", "at=0&size=abc",
		"at=0&staves=9", "at=0&staves=0", "at=0&staves=abc", "at=0&staves=",
	} {
		if why := h.postFails("/api/sheets?" + q); why == "" {
			t.Errorf("%q was accepted", q)
		}
	}
	// Nothing was changed by any of them.
	sh := h.app.score.Sheets[0]
	if len(sh.Staves) != 1 || sh.Staves[0] != 0 {
		t.Errorf("a refused request changed the staves: %v", sh.Staves)
	}
}
