/*
 * voicing_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A part that takes one player's line off a shared staff.
 *
 * The rule is stored on the part and the notes are fetched through it when
 * the part is drawn. What this checks is the two halves of that: that the
 * part shows the right notes, and that the score still has all of them.
 */

package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// sharedStaff is a score whose one staff carries two players: a bar of
// chords, then a bar both play in unison.
func sharedStaff(t *testing.T, h *harness) {
	t.Helper()
	s := h.app.score
	s.Parts = s.Parts[:1]
	p := s.Parts[0]
	half := model.Duration{Num: 1, Den: 2}

	pitch := func(name string) model.Pitch {
		t.Helper()
		pp, _, ok := model.ParsePitch(name)
		if !ok {
			t.Fatalf("%q is not a pitch", name)
		}
		return pp
	}
	p.Measures = []*model.Measure{
		{Voices: []*model.Voice{{Elements: []model.Element{
			&model.Chord{Dur: half, Notes: []*model.Note{
				{Pitch: pitch("C5"), Dur: half}, {Pitch: pitch("E4"), Dur: half},
			}},
			&model.Chord{Dur: half, Notes: []*model.Note{
				{Pitch: pitch("D5"), Dur: half}, {Pitch: pitch("F4"), Dur: half},
			}},
		}}}, RightBarline: model.BarSingle},
		{Voices: []*model.Voice{{Elements: []model.Element{
			&model.Note{Pitch: pitch("G4"), Dur: half},
			&model.Note{Pitch: pitch("A4"), Dur: half},
		}}}, RightBarline: model.BarSingle},
	}
	s.Normalise()
}

func sheetsReply(t *testing.T, h *harness) struct {
	Sheets []struct {
		Name    string `json:"name"`
		Voicing []struct {
			Staff      int    `json:"staff"`
			Pick       string `json:"pick"`
			Nth        []int  `json:"nth"`
			FromBottom bool   `json:"fromBottom"`
			Single     bool   `json:"single"`
			Layer      int    `json:"layer"`
		} `json:"voicing"`
	} `json:"sheets"`
	Showing int `json:"showing"`
} {
	t.Helper()
	var out struct {
		Sheets []struct {
			Name    string `json:"name"`
			Voicing []struct {
				Staff      int    `json:"staff"`
				Pick       string `json:"pick"`
				Nth        []int  `json:"nth"`
				FromBottom bool   `json:"fromBottom"`
				Single     bool   `json:"single"`
				Layer      int    `json:"layer"`
			} `json:"voicing"`
		} `json:"sheets"`
		Showing int `json:"showing"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/sheets")), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// writtenOn counts everything written on a staff of the score, across all
// its bars: a note added to a bar that is already full lands in the next
// one, so counting a single bar says nothing.
func writtenOn(h *harness, staff int) int {
	n := 0
	for _, m := range h.app.score.Parts[staff].Measures {
		for _, v := range m.Voices {
			n += len(v.Elements)
		}
	}
	return n
}

// pitchesDrawn is what the part on the screen actually shows.
func pitchesDrawn(t *testing.T, h *harness) []string {
	t.Helper()
	var out []string
	for _, p := range sheetOf(h.app.score, h.app.sheet).Parts {
		if p.Hidden {
			continue
		}
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					switch e := el.(type) {
					case *model.Note:
						out = append(out, e.Pitch.String())
					case *model.Chord:
						var names []string
						for _, n := range e.Notes {
							names = append(names, n.Pitch.String())
						}
						out = append(out, "["+strings.Join(names, " ")+"]")
					case *model.Rest:
						out = append(out, "r")
					}
				}
			}
		}
	}
	return out
}

func TestAPartCanTakeTheBottomNoteOfASharedStaff(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=bottom")
	h.post("/api/sheets?show=0")

	got := pitchesDrawn(t, h)
	want := []string{"E4", "F4", "G4", "A4"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the second flute reads %v, want %v", got, want)
	}
}

func TestAPartCanTakeTheTopNoteOfASharedStaff(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=top")
	h.post("/api/sheets?show=0")

	got := pitchesDrawn(t, h)
	want := []string{"C5", "D5", "G4", "A4"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the first flute reads %v, want %v", got, want)
	}
}

// The unison bar is in both parts. A part that dropped it would have the
// player resting through a passage they are playing.
func TestBothPartsGetTheUnisonBar(t *testing.T) {
	for _, pick := range []string{"top", "bottom"} {
		h := newHarness(t)
		sharedStaff(t, h)
		h.post("/api/sheets?make=1")
		h.post("/api/sheets?at=0&voice=0&pick=" + pick)
		h.post("/api/sheets?show=0")

		got := strings.Join(pitchesDrawn(t, h), " ")
		if !strings.Contains(got, "G4 A4") {
			t.Errorf("%s does not play the a2 bar: %s", pick, got)
		}
	}
}

// And the score still has every note. The part holds a rule, not a copy.
func TestTheScoreKeepsEveryNote(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=bottom")
	h.post("/api/sheets?show=0")
	pitchesDrawn(t, h) // draw it, which is when the rule is applied

	c, ok := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Chord)
	if !ok {
		t.Fatalf("the score's first element is a %T",
			h.app.score.Parts[0].Measures[0].Voices[0].Elements[0])
	}
	if len(c.Notes) != 2 {
		t.Errorf("the score's chord has %d notes left", len(c.Notes))
	}
}

// Turning back to the score shows all of it again: the voicing belongs to
// the part and not to the music.
func TestTheFullScoreIsUnvoiced(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=bottom")
	h.post("/api/sheets?show=score")

	if got := strings.Join(pitchesDrawn(t, h), " "); !strings.Contains(got, "[C5 E4]") {
		t.Errorf("the score shows %s, want the chord whole", got)
	}
}

func TestAVoicingIsRememberedAndReported(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=nth&nth=2,3&frombottom=1&single=1&layer=2")

	v := sheetsReply(t, h).Sheets[0].Voicing
	if len(v) != 1 {
		t.Fatalf("%d rules", len(v))
	}
	if v[0].Pick != "nth" || len(v[0].Nth) != 2 || v[0].Nth[0] != 2 ||
		!v[0].FromBottom || !v[0].Single || v[0].Layer != 2 {
		t.Errorf("the rule came back as %+v", v[0])
	}
}

// Setting it back to all notes takes the rule away rather than storing a
// rule that does nothing.
func TestAskingForAllNotesTakesTheRuleAway(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=bottom")
	h.post("/api/sheets?at=0&voice=0&pick=all")

	if v := sheetsReply(t, h).Sheets[0].Voicing; len(v) != 0 {
		t.Errorf("%d rules left: %+v", len(v), v)
	}
}

func TestNonsenseVoicingIsRefused(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")

	for _, q := range []string{
		"at=0&voice=0&pick=sideways",
		"at=0&voice=0&pick=nth",
		"at=0&voice=0&pick=nth&nth=0",
		"at=0&voice=x&pick=top",
		"at=0&voice=0&pick=top&layer=99",
	} {
		if got := h.postFails("/api/sheets?" + q); got == "" {
			t.Errorf("%q was allowed", q)
		}
	}
	if v := sheetsReply(t, h).Sheets[0].Voicing; len(v) != 0 {
		t.Errorf("a refused rule was stored: %+v", v)
	}
}

/*
 * What a voiced part will not let you do.
 *
 * The notes drawn are chosen from the staff, so the third thing on the page
 * is not the third thing in the bar and an edit cannot be aimed. Finale
 * arrived at the same answer and says so in its own dialog: notes in a part
 * with a specified voicing are edited in the score.
 */

func TestAVoicedPartIsNotWrittenTo(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=bottom")
	h.post("/api/sheets?show=0")

	got := h.postFails("/api/insert?part=0&measure=0&step=4&dur=quarter")
	if !strings.Contains(got, "full score") {
		t.Errorf("writing to a voiced part said %q", got)
	}
}

// And the same staff is writable again from the score.
func TestTheScoreIsStillWritable(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?at=0&voice=0&pick=bottom")
	h.post("/api/sheets?show=score")

	before := writtenOn(h, 0)
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter")
	if got := writtenOn(h, 0); got <= before {
		t.Errorf("the score went from %d elements to %d", before, got)
	}
}

// A part with no voicing is editable as it always was. The guard is about
// voiced staves and must not have closed the ordinary case.
func TestAnUnvoicedPartIsStillWritable(t *testing.T) {
	h := newHarness(t)
	sharedStaff(t, h)
	h.post("/api/sheets?make=1")
	h.post("/api/sheets?show=0")

	before := writtenOn(h, 0)
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter")
	if got := writtenOn(h, 0); got <= before {
		t.Errorf("an ordinary part refused an edit: %d elements, was %d", got, before)
	}
}
