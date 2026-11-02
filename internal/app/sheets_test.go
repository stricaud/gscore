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

/*
 * Cues.
 *
 * A few bars of somebody else's line, printed small in a part so the player
 * can hear their entry coming. What a player does after forty bars of rest
 * is count, and counting goes wrong.
 *
 * The cue holds no notes: it says which bars of which staff, and the notes
 * are fetched from that staff every time the part is drawn. That is the
 * thing most worth testing, because a cue whose notes had been copied would
 * go stale the first time somebody edited the line it came from and would be
 * wrong in the one place nobody thinks to look.
 */

// cuedScore is a flute part with an oboe under it: four bars each, the oboe
// playing throughout and the flute resting through the middle two — which is
// the only place a cue can go, since a cue is for a bar you are not playing.
func cuedScore(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.post("/api/addstaff?instrument=oboe&clef=treble")
	for i := 1; i < 4; i++ {
		h.post("/api/addmeasure")
	}
	for b := 0; b < 4; b++ {
		h.note(1, b, 2+b, "quarter")
	}
	// The flute plays the first bar and the last, and counts the two
	// between them.
	h.note(0, 0, 2, "quarter")
	h.note(0, 3, 5, "quarter")
	h.post("/api/sheets?make=1")
	return h
}

func TestACueShowsAnotherStavesBarsSmall(t *testing.T) {
	h := cuedScore(t)
	// The flute part, cueing the oboe's bars two and three.
	h.post("/api/sheets?show=0")
	h.post("/api/sheets?at=0&cue=1&bar=1&bars=2")

	view := sheetOf(h.app.score, 0)
	small, notes := 0, 0
	for mi, m := range view.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				n, ok := el.(*model.Note)
				if !ok {
					continue
				}
				notes++
				if n.Marks.Small {
					small++
				}
				if n.Marks.Small && (mi < 1 || mi > 2) {
					t.Errorf("bar %d is small and is not in the cue", mi)
				}
			}
		}
	}
	if small != 2 {
		t.Errorf("%d small notes of %d in the part, want 2 — the oboe's two "+
			"cued bars", small, notes)
	}

	// And the score itself is untouched: the oboe's line is on the oboe's
	// staff at full size, where it belongs.
	for _, m := range h.app.score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok && n.Marks.Small {
					t.Error("a cue was written into the score")
				}
			}
		}
	}
}

// Whose line it is, over the first bar of it. A cue with no name is a few
// notes appearing from nowhere, which is worse than no cue: the player does
// not know who to listen for.
func TestACueSaysWhoseLineItIs(t *testing.T) {
	h := cuedScore(t)
	h.post("/api/sheets?at=0&cue=1&bar=1&bars=2")

	view := sheetOf(h.app.score, 0)
	found := ""
	for _, v := range view.Parts[0].Measures[1].Voices {
		for _, el := range v.Elements {
			if m := marksOn(el); m != nil {
				for _, tx := range m.Texts {
					found = tx.Text
				}
			}
		}
	}
	if found == "" {
		t.Fatal("nothing says whose line the cue is")
	}
	if !strings.EqualFold(found, "oboe") {
		t.Errorf("it is labelled %q", found)
	}

	// A label of one's own wins, for the editions that write "Ob." or
	// "Oboe 1".
	h.post("/api/sheets?at=0&uncue=0")
	h.post("/api/sheets?at=0&cue=1&bar=1&bars=1&label=Ob.")
	view = sheetOf(h.app.score, 0)
	got := ""
	for _, v := range view.Parts[0].Measures[1].Voices {
		for _, el := range v.Elements {
			if m := marksOn(el); m != nil && len(m.Texts) > 0 {
				got = m.Texts[0].Text
			}
		}
	}
	if got != "Ob." {
		t.Errorf("it is labelled %q, want the name that was given", got)
	}
}

// The notes are fetched every time, so editing the line a cue comes from
// changes the cue. This is the whole reason a cue holds no notes.
func TestACueFollowsTheLineItCameFrom(t *testing.T) {
	h := cuedScore(t)
	h.post("/api/sheets?at=0&cue=1&bar=1&bars=1")

	before := cuedPitch(t, sheetOf(h.app.score, 0))
	// Move the oboe's note, as an edit to the score would.
	for _, v := range h.app.score.Parts[1].Measures[1].Voices {
		for _, el := range v.Elements {
			if n, ok := el.(*model.Note); ok {
				n.Pitch.Step = model.Step((int(n.Pitch.Step) + 3) % 7)
			}
		}
	}
	after := cuedPitch(t, sheetOf(h.app.score, 0))
	if before == after {
		t.Errorf("the cue still shows %q after the oboe's note was changed: "+
			"it was copied rather than fetched, so it will be wrong in the "+
			"one place nobody looks", before)
	}
}

func cuedPitch(t *testing.T, view *model.Score) string {
	t.Helper()
	for _, v := range view.Parts[0].Measures[1].Voices {
		for _, el := range v.Elements {
			if n, ok := el.(*model.Note); ok && n.Marks.Small {
				return n.Pitch.String()
			}
		}
	}
	return ""
}

// What belongs to the other player does not come with it. Their dynamics,
// their words and their chord symbols read as instructions to whoever is
// holding this page.
func TestACueLeavesTheOtherPlayersMarkingsBehind(t *testing.T) {
	h := cuedScore(t)
	h.post("/api/dynamic?part=1&measure=1&at=1&dynamic=ff")
	h.post("/api/sheets?at=0&cue=1&bar=1&bars=1")

	for _, v := range sheetOf(h.app.score, 0).Parts[0].Measures[1].Voices {
		for _, el := range v.Elements {
			if m := marksOn(el); m != nil && m.Dynamic != model.DynamicNone {
				t.Error("the other player's dynamic was printed in this part")
			}
		}
	}
	// And it is still on the oboe's own staff, where it was written.
	kept := false
	for _, v := range h.app.score.Parts[1].Measures[1].Voices {
		for _, el := range v.Elements {
			if m := marksOn(el); m != nil && m.Dynamic != model.DynamicNone {
				kept = true
			}
		}
	}
	if !kept {
		t.Error("the oboe lost its own dynamic")
	}
}

// A cue of a staff this part already prints would be the same bars twice.
func TestNonsenseAboutACueIsRefused(t *testing.T) {
	h := cuedScore(t)
	for _, q := range []string{
		"at=0&cue=0", // its own staff
		"at=0&cue=9", // no such staff
		"at=0&cue=abc",
		"at=0&cue=1&bar=99",
		"at=0&cue=1&bars=0",
		"at=0&cue=1&bars=99",
		"at=0&uncue=0", // there are none yet
	} {
		if why := h.postFails("/api/sheets?" + q); why == "" {
			t.Errorf("%q was accepted", q)
		}
	}
	if len(h.app.score.Sheets[0].Cues) != 0 {
		t.Errorf("a refused request added a cue: %v", h.app.score.Sheets[0].Cues)
	}
}

// A cue survives the file, because somebody chose which bars of whose line
// would tell them where they are.
func TestCuesSurviveTheFile(t *testing.T) {
	h := cuedScore(t)
	h.app.local = true
	h.post("/api/sheets?at=0&cue=1&bar=1&bars=3&label=Ob.")

	path := filepath.Join(t.TempDir(), "cued.gscore")
	h.post("/api/save?path=" + path)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	back, _, err := gscore.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Sheets) == 0 || len(back.Sheets[0].Cues) != 1 {
		t.Fatalf("the cues came back as %+v", back.Sheets)
	}
	got := back.Sheets[0].Cues[0]
	if got.At != 1 || got.Bars != 3 || got.From != 1 || got.Label != "Ob." {
		t.Errorf("the cue came back as %+v", got)
	}
}

// A cue goes where the player is not playing, and stops at their entry.
//
// Printed over a bar of their own it would hide the thing they are holding
// the page for, which is the one thing a part must never do.
func TestACueStopsAtTheEntryItIsThereFor(t *testing.T) {
	h := cuedScore(t)
	// Three bars of cue from bar two: bars two and three rest, and the
	// fourth is the flute's own entry.
	h.post("/api/sheets?at=0&cue=1&bar=1&bars=3")

	view := sheetOf(h.app.score, 0)
	for mi, m := range view.Parts[0].Measures {
		small := false
		mine := false
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				n, ok := el.(*model.Note)
				if !ok {
					continue
				}
				if n.Marks.Small {
					small = true
				} else {
					mine = true
				}
			}
		}
		switch mi {
		case 1, 2:
			if !small {
				t.Errorf("bar %d rests and carries no cue", mi+1)
			}
		case 0, 3:
			if small {
				t.Errorf("bar %d is the flute's own and was printed over "+
					"with a cue", mi+1)
			}
			if !mine {
				t.Errorf("bar %d lost the flute's own music", mi+1)
			}
		}
	}
}

// And a cue that has nowhere to go is refused at the time it is asked for,
// rather than drawn as nothing: somebody who asks and sees no change has no
// way to know which of the two happened.
func TestACueWithNowhereToGoSaysSo(t *testing.T) {
	h := cuedScore(t)
	why := h.postFails("/api/sheets?at=0&cue=1&bar=0&bars=1")
	if why == "" {
		t.Fatal("a cue over the flute's own first bar was accepted")
	}
	if !strings.Contains(why, "playing in those bars") {
		t.Errorf("it was refused with %q, which does not say why", why)
	}
}
