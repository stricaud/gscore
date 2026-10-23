package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// Everything asked for, exercised through the interface the browser uses.
//
// Each of these was reported broken at least once, and several were "fixed"
// in a place nothing reached. Going through the real HTTP handlers is the
// only check that means anything: it is the same path a click takes.

type harness struct {
	t   *testing.T
	app *App
	mux *http.ServeMux
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	a := New(model.NewScore(), "")
	mux := http.NewServeMux()
	a.routes(mux)
	return &harness{t: t, app: a, mux: mux}
}

// post calls a handler and fails the test if it does not succeed.
func (h *harness) post(path string) map[string]any {
	h.t.Helper()
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
	if rec.Code != http.StatusOK {
		h.t.Fatalf("POST %s: %d %s", path, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// postFails expects a refusal, with a reason.
func (h *harness) postFails(path string) string {
	h.t.Helper()
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
	if rec.Code == http.StatusOK {
		h.t.Fatalf("POST %s was meant to be refused, but succeeded", path)
	}
	return strings.TrimSpace(rec.Body.String())
}

func (h *harness) get(path string) string {
	h.t.Helper()
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		h.t.Fatalf("GET %s: %d", path, rec.Code)
	}
	b, _ := io.ReadAll(rec.Body)
	return string(b)
}

func (h *harness) note(part, measure, step int, dur string) {
	h.t.Helper()
	h.post("/api/insert?part=" + itoa(part) + "&measure=" + itoa(measure) +
		"&at=-1&step=" + itoa(step) + "&dur=" + dur + "&kind=note")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// A full bar sends what is written next into the bar after it, making one.
func TestWritingPastTheEndOfABar(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	got := h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=quarter&kind=note")
	if got["measure"] != float64(1) {
		t.Errorf("the fifth quarter stayed in bar 1: %v", got)
	}
	if n := len(h.app.score.Parts[0].Measures); n != 2 {
		t.Errorf("got %d measures, want 2", n)
	}
}

// Clicking above a note stacks a pitch on it.
func TestBuildingAChord(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/addpitch?part=0&measure=0&index=0&step=4")
	h.post("/api/addpitch?part=0&measure=0&index=0&step=6")
	el := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0]
	ch, ok := el.(*model.Chord)
	if !ok {
		t.Fatalf("got %T, want a chord", el)
	}
	if len(ch.Notes) != 3 {
		t.Errorf("chord has %d notes, want 3", len(ch.Notes))
	}
	if msg := h.postFails("/api/addpitch?part=0&measure=0&index=0&step=4"); msg == "" {
		t.Error("the same pitch twice was allowed")
	}
}

// An accidental written on a note that is already there.
func TestAccidentalOnASelectedNote(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/accidental?part=0&measure=0&at=1&accidental=flat")
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if n.Accidental != model.Flat {
		t.Errorf("got %v, want a flat", n.Accidental)
	}
	if n.Pitch.Alter != -1 {
		t.Errorf("the flat did not change how it sounds: alter=%d", n.Pitch.Alter)
	}
}

// A hairpin covers a passage, and is refused when it has nowhere to go.
func TestHairpinNeedsARange(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	msg := h.postFails("/api/hairpin?hairpin=crescendo&part=0&measure=0&at=1")
	if !strings.Contains(msg, "select a passage") {
		t.Errorf("unhelpful refusal: %q", msg)
	}
	h.post("/api/hairpin?hairpin=crescendo&sel=0:0:0,0:0:1,0:0:2,0:0:3")

	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	first := els[0].(*model.Note)
	last := els[3].(*model.Note)
	if first.Hairpin == nil || !first.Hairpin.Start {
		t.Error("the hairpin does not open on the first note")
	}
	if last.Hairpin == nil || !last.Hairpin.Stop {
		t.Error("the hairpin does not close on the last note")
	}
	// And it is actually drawn.
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	if countWedges(lay) < 2 {
		t.Error("the hairpin was not drawn")
	}
}

func countWedges(lay *engrave.Layout) int {
	n := 0
	for _, l := range lay.Lines {
		// A wedge is a short sloping line below the staff; staff lines and
		// stems are level or vertical.
		if l.Y1 != l.Y2 && l.X1 != l.X2 {
			n++
		}
	}
	return n
}

// A repeat ending goes over every staff, because a repeat belongs to the piece.
func TestEndingCoversEveryStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.note(0, 0, 2, "quarter")
	h.post("/api/ending?measure=0&numbers=1")
	for i, p := range h.app.score.Parts {
		if p.Measures[0].Ending == nil {
			t.Errorf("staff %d has no ending", i)
		}
	}
}

// A rehearsal mark likewise.
func TestMarkerCoversEveryStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/marker?measure=0&text=A")
	for i, p := range h.app.score.Parts {
		if p.Measures[0].Marker != "A" {
			t.Errorf("staff %d has no mark", i)
		}
	}
}

// The meter belongs to the music, not to one staff of it.
func TestMeterAppliesEverywhere(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/staff?part=0&time=6/8")
	for i, p := range h.app.score.Parts {
		if p.Time.Beats != 6 || p.Time.BeatValue != 8 {
			t.Errorf("staff %d is in %v", i, p.Time)
		}
	}
	// "C" means common time, which is what it is called and what is printed.
	h.post("/api/staff?part=0&time=C")
	if h.app.score.Parts[0].Time.Symbol != model.TimeCommon {
		t.Error("C did not give common time")
	}
	h.post("/api/staff?part=0&time=" + url.QueryEscape("C|"))
	if h.app.score.Parts[0].Time.Symbol != model.TimeCut {
		t.Error("C| did not give cut time")
	}
}

// A key change in the middle, adjusted for each staff's transposition.
func TestKeyChangeMidScoreAndTransposed(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=alto%20sax&clef=treble")
	for i := 0; i < 8; i++ {
		h.note(0, 0, 2, "quarter") // spills into later bars
	}
	h.post("/api/setkey?part=0&fifths=-2&at=1")

	concert := h.app.score.Parts[0]
	if concert.Measures[1].Key == nil || concert.Measures[1].Key.Fifths != -2 {
		t.Errorf("no key change on the concert-pitch staff: %+v", concert.Measures[1].Key)
	}
	sax := h.app.score.Parts[1]
	if sax.Transpose.IsZero() {
		t.Fatal("the saxophone staff is not transposing")
	}
	if len(sax.Measures) > 1 && sax.Measures[1].Key != nil {
		// Two flats in concert pitch is one sharp for an instrument in E flat.
		if got := sax.Measures[1].Key.Fifths; got != 1 {
			t.Errorf("the saxophone's key change is %d sharps, want 1", got)
		}
	}
}

// A barline runs down the whole system.
func TestBarlineCoversEveryStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/barline?part=0&measure=0&style=final")
	for i, p := range h.app.score.Parts {
		if p.Measures[0].RightBarline != model.BarFinal {
			t.Errorf("staff %d still has %v", i, p.Measures[0].RightBarline)
		}
	}
}

// Beaming a selection by hand.
func TestBeamingASelection(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "eighth")
	}
	h.post("/api/beam?beam=join&sel=0:0:0,0:0:1,0:0:2,0:0:3")
	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	if els[0].(*model.Note).Beam != model.BeamStart {
		t.Error("the beam does not start on the first note")
	}
	if els[3].(*model.Note).Beam != model.BeamStop {
		t.Error("the beam does not stop on the last note")
	}
}

// Copy, paste and nudge.
func TestClipboardAndNudge(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 4, "quarter")
	h.post("/api/copy?sel=0:0:0,0:0:1")
	h.post("/api/paste?part=0&measure=0&at=2")
	if n := countNotes(h.app.score); n != 4 {
		t.Errorf("after pasting two notes there are %d", n)
	}
	before := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note).Pitch
	h.post("/api/nudge?part=0&measure=0&at=1&steps=1")
	after := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note).Pitch
	if after.DiatonicIndex() != before.DiatonicIndex()+1 {
		t.Errorf("the note did not move up a step: %v -> %v", before, after)
	}
}

func countNotes(s *model.Score) int {
	n := 0
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				n += len(v.Elements)
			}
		}
	}
	return n
}

// Lyrics, typed as a verse and handed out to the notes.
func TestLyricsAsAVerse(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	h.post("/api/lyrics?part=0&verse=1&text=" + url.QueryEscape("Glo-ri-a in"))
	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	want := []struct {
		text string
		kind model.Syllabic
	}{
		{"Glo", model.SyllableBegin},
		{"ri", model.SyllableMiddle},
		{"a", model.SyllableEnd},
		{"in", model.SyllableSingle},
	}
	for i, w := range want {
		ly := els[i].(*model.Note).Lyrics
		if len(ly) != 1 || ly[0].Text != w.text || ly[0].Syllabic != w.kind {
			t.Errorf("note %d: got %+v, want %q %v", i, ly, w.text, w.kind)
		}
	}
	// And it comes back as it was typed, so a verse can be corrected.
	out := h.get("/api/getlyrics?part=0&verse=1")
	if !strings.Contains(out, "Glo-ri-a in") {
		t.Errorf("the verse did not come back: %s", out)
	}
}

// A drum staff writes drums, and they play as drums.
func TestPercussionWritesAndPlays(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&clef=percussion")
	for _, step := range []int{10, 5, 1, 5} {
		h.note(0, 0, step, "quarter")
	}
	pb := buildPlayback(h.app.score)
	if len(pb.Events) != 4 {
		t.Fatalf("got %d events, want 4", len(pb.Events))
	}
	want := []string{"Hi-hat closed", "Snare drum", "Bass drum", "Snare drum"}
	for i, ev := range pb.Events {
		if ev.Family != "percussion" {
			t.Errorf("event %d is %q, not percussion", i, ev.Family)
		}
		if ev.DrumName != want[i] {
			t.Errorf("event %d is %q, want %q", i, ev.DrumName, want[i])
		}
	}
}

// A repeat sign plays as the bar it stands for.
func TestMeasureRepeatPlays(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	h.post("/api/addmeasure")
	h.post("/api/measurerepeat?part=0&measure=1&bars=1")

	pb := buildPlayback(h.app.score)
	if len(pb.Events) != 8 {
		t.Errorf("a repeated bar gave %d events, want 8", len(pb.Events))
	}
	// And it is drawn as the sign, not as the notes.
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	signs := 0
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "repeat") && strings.Contains(g.Name, "Bar") {
			signs++
		}
	}
	if signs != 1 {
		t.Errorf("got %d repeat signs drawn, want 1", signs)
	}
}

// A hidden staff leaves the drawing but not the score, and still sounds.
func TestHidingAStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.note(0, 0, 2, "quarter")
	h.note(1, 0, 4, "quarter")

	before := len(buildPlayback(h.app.score).Events)
	h.post("/api/staff?part=1&hidden=1")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	staves := map[int]bool{}
	for _, hit := range lay.Hits {
		staves[hit.Part] = true
	}
	if staves[1] {
		t.Error("the hidden staff is still drawn")
	}
	if len(h.app.score.Parts) != 2 {
		t.Error("hiding a staff removed it from the score")
	}
	if after := len(buildPlayback(h.app.score).Events); after != before {
		t.Errorf("a hidden staff stopped sounding: %d -> %d", before, after)
	}
}

// Hiding a staff must not renumber the ones below it, or the playback
// highlight lands on the wrong notes.
func TestHidingKeepsStaffNumbers(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.post("/api/addstaff?instrument=oboe&clef=treble")
	h.note(0, 0, 2, "quarter")
	h.note(1, 0, 3, "quarter")
	h.note(2, 0, 4, "quarter")

	h.post("/api/staff?part=1&hidden=1")
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())

	// The third staff must still call itself the third.
	seen := map[int]bool{}
	for _, hit := range lay.Hits {
		seen[hit.Part] = true
	}
	if !seen[0] || !seen[2] || seen[1] {
		t.Errorf("drawn staves are %v, want 0 and 2", seen)
	}

	// And its notes must keep the numbers playback gives them.
	refs := map[int]bool{}
	for _, ev := range buildPlayback(h.app.score).Events {
		refs[ev.Ref] = true
	}
	for _, hit := range lay.Hits {
		for _, el := range hit.Elements {
			if !refs[el.Ref] {
				t.Errorf("staff %d has a note numbered %d that playback does not know",
					hit.Part, el.Ref)
			}
		}
	}
}

// A chord symbol, taken apart so the engraver can set it properly.
func TestChordSymbolAndFrame(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Bb7sus4&frame=x13131&firstfret=1")
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if n.Chord == nil {
		t.Fatal("no chord symbol")
	}
	if n.Chord.Root != "Bb" || n.Chord.Quality != "7sus4" {
		t.Errorf("got root %q quality %q", n.Chord.Root, n.Chord.Quality)
	}
	if n.Chord.Frame == nil || len(n.Chord.Frame.Frets) != 6 {
		t.Fatalf("frame is %+v", n.Chord.Frame)
	}
	if n.Chord.Frame.Frets[0] != -1 {
		t.Error("x did not mean a string that is not played")
	}
	// "N.C." is not a chord and is kept whole.
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=" + url.QueryEscape("N.C."))
	if n.Chord.Text != "N.C." {
		t.Errorf("N.C. was taken apart: %+v", n.Chord)
	}
}

// Everything above a staff stacks instead of being drawn in the same place.
func TestThingsAboveTheStaffDoNotCollide(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Em7")
	h.post("/api/marker?measure=0&text=A")
	h.post("/api/text?part=0&measure=0&at=1&text=rit.")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	var chord, mark, text float64
	for _, x := range lay.Texts {
		switch x.S {
		case "E":
			chord = x.Y
		case "A":
			mark = x.Y
		case "rit.":
			text = x.Y
		}
	}
	if chord == 0 || mark == 0 || text == 0 {
		t.Fatalf("something was not drawn: chord=%v mark=%v text=%v", chord, mark, text)
	}
	// y grows downward, so each must be strictly above the one before.
	if !(text < mark && mark < chord) {
		t.Errorf("they overlap: text=%.2f mark=%.2f chord=%.2f", text, mark, chord)
	}
}

// Systems must not overlap, or a page break slices a staff off.
func TestSystemsDoNotOverlap(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	for i := 0; i < 64; i++ {
		h.note(0, 0, 2+(i%7), "quarter")
		h.note(1, 0, 2+(i%5), "quarter")
	}
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Em7&frame=022000")

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Paginated
	lay := engrave.Engrave(h.app.score, opt)
	if len(lay.Systems) < 2 {
		t.Fatalf("only %d systems; the test needs more", len(lay.Systems))
	}
	for i := 1; i < len(lay.Systems); i++ {
		if lay.Systems[i].Top < lay.Systems[i-1].Bottom {
			t.Errorf("system %d starts at %.2f, inside system %d which ends at %.2f",
				i, lay.Systems[i].Top, i-1, lay.Systems[i-1].Bottom)
		}
	}
}

// Every element says when it sounds, which is what the cursor follows.
func TestElementsCarryTheirTime(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "half")
	h.note(0, 0, 4, "quarter")
	h.note(0, 0, 6, "quarter")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	got := []float64{}
	for _, el := range lay.Hits[0].Elements {
		got = append(got, el.At)
	}
	want := []float64{0, 0.5, 0.75}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("element %d sounds at %v, want %v", i, got[i], want[i])
		}
	}
	// And the spacing is NOT proportional to time, which is the whole reason
	// the cursor cannot simply slide across the bar.
	e := lay.Hits[0].Elements
	gap1 := e[1].X - e[0].X
	gap2 := e[2].X - e[1].X
	if gap1 >= gap2*2 {
		t.Errorf("a half note is %.2f wide and a quarter %.2f: spacing looks linear",
			gap1, gap2)
	}
}

// A ragged or empty staff is tidied on the way in.
func TestNormaliseRaggedParts(t *testing.T) {
	s := model.NewScore()
	s.Parts = append(s.Parts, &model.Part{Clef: model.Bass}) // no measures at all
	s.Parts[0].Measures = append(s.Parts[0].Measures,
		&model.Measure{Voices: []*model.Voice{{}}},
		&model.Measure{Voices: []*model.Voice{{}}})
	s.Normalise()
	if len(s.Parts) != 1 {
		t.Errorf("the empty trailing staff was kept: %d parts", len(s.Parts))
	}

	s2 := model.NewScore()
	s2.Parts = append(s2.Parts, &model.Part{
		Clef: model.Bass, Measures: []*model.Measure{{Voices: []*model.Voice{{}}}},
	})
	s2.Parts[0].Measures = append(s2.Parts[0].Measures,
		&model.Measure{Voices: []*model.Voice{{}}},
		&model.Measure{Voices: []*model.Voice{{}}})
	s2.Normalise()
	if a, b := len(s2.Parts[0].Measures), len(s2.Parts[1].Measures); a != b {
		t.Errorf("staves have %d and %d measures after normalising", a, b)
	}
}

// An unknown instrument is refused rather than silently becoming a piano.
func TestUnknownInstrumentIsRefused(t *testing.T) {
	h := newHarness(t)
	if msg := h.postFails("/api/addstaff?instrument=kazoo"); !strings.Contains(msg, "kazoo") {
		t.Errorf("unhelpful refusal: %q", msg)
	}
}

// The clef, key and meter at the head of a system can be pointed at.
func TestHeaderRegionsAreReported(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	kinds := map[string]bool{}
	for _, hd := range lay.Headers {
		kinds[hd.Kind] = true
		if hd.X1 <= hd.X0 {
			t.Errorf("%s has no width", hd.Kind)
		}
	}
	for _, want := range []string{"clef", "time"} {
		if !kinds[want] {
			t.Errorf("nothing reported for the %s", want)
		}
	}
}

// Undo takes an edit back, and redo puts it on again.
func TestUndoAndRedo(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 4, "quarter")
	if n := countNotes(h.app.score); n != 2 {
		t.Fatalf("got %d notes", n)
	}

	h.post("/api/undo")
	if n := countNotes(h.app.score); n != 1 {
		t.Errorf("after one undo there are %d notes, want 1", n)
	}
	h.post("/api/undo")
	if n := countNotes(h.app.score); n != 0 {
		t.Errorf("after two undos there are %d notes, want 0", n)
	}
	if msg := h.postFails("/api/undo"); !strings.Contains(msg, "nothing to undo") {
		t.Errorf("unhelpful refusal at the end of the history: %q", msg)
	}

	h.post("/api/redo")
	h.post("/api/redo")
	if n := countNotes(h.app.score); n != 2 {
		t.Errorf("after redoing both there are %d notes, want 2", n)
	}
	if msg := h.postFails("/api/redo"); !strings.Contains(msg, "nothing to redo") {
		t.Errorf("unhelpful refusal: %q", msg)
	}
}

// A new edit after an undo is a new branch: the redo is gone.
func TestEditingAfterUndoDropsTheRedo(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/undo")
	h.note(0, 0, 6, "quarter")
	if msg := h.postFails("/api/redo"); msg == "" {
		t.Error("redo survived an edit made after the undo")
	}
}

// A request that changes nothing must not cost a press of undo.
func TestRefusedEditsAreNotUndoable(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	// A hairpin on one note is refused.
	h.postFails("/api/hairpin?hairpin=crescendo&part=0&measure=0&at=1")
	h.post("/api/undo")
	if n := countNotes(h.app.score); n != 0 {
		t.Errorf("undo went back only as far as the refused request: %d notes left", n)
	}
}

// Every edit that changes the music has to be undoable. Each of these was
// written as a separate handler, and each had to be remembered.
func TestEveryKindOfEditIsUndoable(t *testing.T) {
	edits := []struct {
		name string
		// setup runs before the comparison starts, for edits that need
		// something to act on. Without it, "insert a note then delete it"
		// ends where it began and proves nothing.
		setup func(h *harness)
		do    func(h *harness)
	}{
		{name: "insert", do: func(h *harness) { h.note(0, 0, 2, "quarter") }},
		{name: "addpitch", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/addpitch?part=0&measure=0&index=0&step=4")
		}},
		{name: "accidental", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/accidental?part=0&measure=0&at=1&accidental=sharp")
		}},
		{name: "dynamic", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/dynamic?part=0&measure=0&at=1&dynamic=ff")
		}},
		{name: "stem", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/stem?part=0&measure=0&at=1&stem=down")
		}},
		{name: "articulation", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/articulation?part=0&measure=0&at=1&articulation=staccato")
		}},
		{name: "chordsymbol", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Em")
		}},
		{name: "text", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/text?part=0&measure=0&at=1&text=rit.")
		}},
		{name: "lyrics", do: func(h *harness) {
			h.note(0, 0, 2, "quarter")
			h.post("/api/lyrics?part=0&verse=1&text=la")
		}},
		{name: "barline", do: func(h *harness) { h.post("/api/barline?part=0&measure=0&style=final") }},
		{name: "marker", do: func(h *harness) { h.post("/api/marker?measure=0&text=A") }},
		{name: "addmeasure", do: func(h *harness) { h.post("/api/addmeasure") }},
		{name: "setkey", do: func(h *harness) { h.post("/api/setkey?part=0&fifths=3") }},
		{name: "tempo", do: func(h *harness) { h.post("/api/tempo?bpm=84&unit=quarter") }},
		{name: "header", do: func(h *harness) { h.post("/api/header?title=X&composer=Y") }},
		{name: "staff", do: func(h *harness) { h.post("/api/staff?part=0&clef=bass") }},
		{name: "addstaff", do: func(h *harness) { h.post("/api/addstaff?instrument=flute") }},
		{name: "measurerepeat", do: func(h *harness) {
			h.post("/api/addmeasure")
			h.post("/api/measurerepeat?part=0&measure=1&bars=1")
		}},
		{
			name:  "delete",
			setup: func(h *harness) { h.note(0, 0, 2, "quarter") },
			// Note: /api/delete counts from zero, unlike the marking
			// handlers, which take the cursor position after the element.
			do: func(h *harness) { h.post("/api/delete?part=0&measure=0&at=0") },
		},
		{
			name: "removestaff",
			setup: func(h *harness) {
				h.post("/api/addstaff?instrument=flute&clef=treble")
			},
			do: func(h *harness) { h.post("/api/removestaff?part=1") },
		},
		{
			name: "nudge",
			setup: func(h *harness) { h.note(0, 0, 2, "quarter") },
			do:    func(h *harness) { h.post("/api/nudge?part=0&measure=0&at=1&steps=2") },
		},
		{
			name: "paste",
			setup: func(h *harness) {
				h.note(0, 0, 2, "quarter")
				h.post("/api/copy?sel=0:0:0")
			},
			do: func(h *harness) { h.post("/api/paste?part=0&measure=0&at=1") },
		},
	}

	for _, e := range edits {
		t.Run(e.name, func(t *testing.T) {
			h := newHarness(t)
			if e.setup != nil {
				e.setup(h)
			}
			// How deep the history is once the setup is done. Undoing has to
			// stop here, not unwind the setup as well.
			base := len(h.app.undoStack)
			before := describe(h.app.score)

			e.do(h)
			if describe(h.app.score) == before {
				t.Fatalf("%s changed nothing, so the test proves nothing", e.name)
			}
			if len(h.app.undoStack) <= base {
				t.Fatalf("%s recorded nothing to undo: the route is not wrapped", e.name)
			}

			// However many steps it took, back to where the setup left off.
			for len(h.app.undoStack) > base {
				rec := httptest.NewRecorder()
				h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/undo", nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: undo refused with %d left", e.name, len(h.app.undoStack))
				}
			}
			if got := describe(h.app.score); got != before {
				t.Errorf("%s could not be undone:\n before %s\n after  %s", e.name, before, got)
			}
		})
	}
}

// describe renders a score to a string, for comparing one to another.
func describe(s *model.Score) string {
	var b strings.Builder
	b.WriteString(s.Identity.Title + "|" + s.Identity.Composer + "|")
	b.WriteString(itoa(s.Tempo.BPM) + "|")
	for _, p := range s.Parts {
		b.WriteString("[" + p.Name + " " + p.Clef.String() +
			" " + itoa(p.Key.Fifths) + " " + p.Time.String())
		for _, m := range p.Measures {
			b.WriteString("{" + m.RightBarline.String() + m.Marker +
				itoa(m.Repeat))
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					switch e := el.(type) {
					case *model.Note:
						b.WriteString(e.Pitch.String() + e.Accidental.String() +
							e.Dur.String() + e.Dynamic.String() +
							e.Stem.String() + e.Chord.String())
						for _, a := range e.Articulations {
							b.WriteString(a.String())
						}
						for _, l := range e.Lyrics {
							b.WriteString(l.Text)
						}
						for _, x := range e.Texts {
							b.WriteString(x.Text)
						}
					case *model.Chord:
						b.WriteString("chord" + itoa(len(e.Notes)))
					case *model.Rest:
						b.WriteString("r" + e.Dur.String())
					}
					b.WriteString(",")
				}
			}
			b.WriteString("}")
		}
		b.WriteString("]")
	}
	return b.String()
}

// Playback follows the repeats: a repeated section sounds twice, and the
// cursor goes round with it.
func TestPlaybackFollowsRepeats(t *testing.T) {
	h := newHarness(t)
	// Four bars, a quarter note in each, with a repeat closing bar 2.
	for bar := 0; bar < 4; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, bar, 2+bar, "quarter")
	}
	straight := buildPlayback(h.app.score)

	h.post("/api/barline?part=0&measure=1&style=repeat-end")
	withRepeat := buildPlayback(h.app.score)

	if len(withRepeat.Events) != len(straight.Events)+2 {
		t.Errorf("a repeat of two bars gave %d events, want %d",
			len(withRepeat.Events), len(straight.Events)+2)
	}
	// Bars 1 and 2 appear twice in the cursor's path, in order.
	var played []int
	for _, m := range withRepeat.Measures {
		played = append(played, m.Index)
	}
	want := []int{0, 1, 0, 1, 2, 3}
	if len(played) != len(want) {
		t.Fatalf("the cursor visits %v, want %v", played, want)
	}
	for i := range want {
		if played[i] != want[i] {
			t.Fatalf("the cursor visits %v, want %v", played, want)
		}
	}
	// And the timeline runs forward throughout, even going back.
	for i := 1; i < len(withRepeat.Measures); i++ {
		if withRepeat.Measures[i].Start <= withRepeat.Measures[i-1].Start {
			t.Errorf("time went backwards at step %d", i)
		}
	}
}

// A note played twice keeps the number its notehead was drawn with, so the
// highlight finds it both times.
func TestRepeatedNotesKeepTheirNumbers(t *testing.T) {
	h := newHarness(t)
	for bar := 0; bar < 2; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, bar, 2+bar, "quarter")
	}
	h.post("/api/barline?part=0&measure=0&style=repeat-end")

	pb := buildPlayback(h.app.score)
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	drawn := map[int]bool{}
	for _, hit := range lay.Hits {
		for _, el := range hit.Elements {
			drawn[el.Ref] = true
		}
	}
	for _, ev := range pb.Events {
		if ev.Ref != 0 && !drawn[ev.Ref] {
			t.Errorf("a note sounds with number %d, which nothing on the page has", ev.Ref)
		}
	}
	// The first bar sounds twice, with the same note both times.
	first := 0
	for _, ev := range pb.Events {
		if ev.Ref == 1 {
			first++
		}
	}
	if first != 2 {
		t.Errorf("the repeated note sounded %d times, want 2", first)
	}
}

// 1st and 2nd time bars.
func TestPlaybackTakesTheRightEnding(t *testing.T) {
	h := newHarness(t)
	for bar := 0; bar < 4; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, bar, 2+bar, "quarter")
	}
	// |: 1 2 [1. 3 :][2. 4 ]
	h.post("/api/ending?measure=2&numbers=1")
	h.post("/api/barline?part=0&measure=2&style=repeat-end")
	h.post("/api/ending?measure=3&numbers=2&open=1")

	var played []int
	for _, m := range buildPlayback(h.app.score).Measures {
		played = append(played, m.Index)
	}
	want := []int{0, 1, 2, 0, 1, 3}
	if len(played) != len(want) {
		t.Fatalf("played %v, want %v", played, want)
	}
	for i := range want {
		if played[i] != want[i] {
			t.Fatalf("played %v, want %v", played, want)
		}
	}
}
