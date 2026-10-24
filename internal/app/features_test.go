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

// postBody calls a handler with a body, which is how a file arrives: the
// browser sends its bytes rather than a path it has no business knowing.
func (h *harness) postBody(path, body string) map[string]any {
	h.t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	h.mux.ServeHTTP(rec, req)
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
	// y grows downward, so each must be strictly above the one before:
	// outwards from the staff it is the chord symbol, then the direction,
	// then the letter.
	//
	// The letter is outermost because it is the thing somebody calls across
	// a room — "from B" — and it has to be findable at a glance; a direction
	// is read by the player who is already looking at the bar. They used to
	// come out the other way round, from the days when each was written at a
	// fixed distance from the staff and the order fell out of the numbers
	// rather than out of what the marks are for.
	if !(mark < text && text < chord) {
		t.Errorf("they overlap or stack the wrong way: "+
			"mark=%.2f text=%.2f chord=%.2f", mark, text, chord)
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
			name:  "nudge",
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

// Clicking in the empty bar the engraver draws past the end of the music
// starts a real bar there, on every staff.
func TestWritingIntoTheBarAhead(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	if n := len(h.app.score.Parts[0].Measures); n != 1 {
		t.Fatalf("got %d bars", n)
	}

	// The engraver offers a second bar to click in.
	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	lay := engrave.Engrave(h.app.score, opt)
	bars := map[int]bool{}
	for _, hit := range lay.Hits {
		bars[hit.Measure] = true
	}
	if !bars[1] {
		t.Fatal("no bar to write into past the end")
	}

	// Writing in it makes it real, on both staves.
	h.note(0, 1, 4, "quarter")
	for i, p := range h.app.score.Parts {
		if len(p.Measures) != 2 {
			t.Errorf("staff %d has %d bars, want 2", i, len(p.Measures))
		}
	}

	// A final barline ends the offer.
	h.post("/api/barline?part=0&measure=1&style=final")
	lay = engrave.Engrave(h.app.score, opt)
	bars = map[int]bool{}
	for _, hit := range lay.Hits {
		bars[hit.Measure] = true
	}
	if bars[2] {
		t.Error("still offering a bar after the final barline")
	}
}

// A key or meter change written in the middle of a piece can be taken away
// again, on every staff at once.
func TestRemovingASignatureChange(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	for bar := 0; bar < 4; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, bar, 2, "quarter")
	}

	h.post("/api/setkey?part=0&fifths=-3&at=2")
	h.post("/api/staff?part=0&time=6/8&at=2")
	for i, p := range h.app.score.Parts {
		if p.Measures[2].Key == nil || p.Measures[2].Time == nil {
			t.Fatalf("staff %d did not get the changes", i)
		}
	}

	h.post("/api/setkey?part=0&at=2&remove=1")
	h.post("/api/staff?part=0&time=none&at=2")
	for i, p := range h.app.score.Parts {
		if p.Measures[2].Key != nil {
			t.Errorf("staff %d still has a key change", i)
		}
		if p.Measures[2].Time != nil {
			t.Errorf("staff %d still has a meter change", i)
		}
	}

	// Asking to remove one that is not there says so rather than pretending.
	if msg := h.postFails("/api/setkey?part=0&at=3&remove=1"); msg == "" {
		t.Error("removing a change that is not there was allowed")
	}
	if msg := h.postFails("/api/staff?part=0&time=none&at=0"); msg == "" {
		t.Error("removing the opening meter was allowed")
	}
}

// The editor is told which bars carry a signature change, so it can offer to
// take one away only where there is one.
func TestBarsReportTheirChanges(t *testing.T) {
	h := newHarness(t)
	for bar := 0; bar < 4; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, bar, 2, "quarter")
	}
	h.post("/api/setkey?part=0&fifths=2&at=1")
	h.post("/api/staff?part=0&time=3/4&at=2")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	key, meter := map[int]bool{}, map[int]bool{}
	for _, hit := range lay.Hits {
		if hit.KeyChange {
			key[hit.Measure] = true
		}
		if hit.TimeChange {
			meter[hit.Measure] = true
		}
	}
	if !key[1] || len(key) != 1 {
		t.Errorf("key changes reported at %v, want only bar 2", key)
	}
	if !meter[2] || len(meter) != 1 {
		t.Errorf("meter changes reported at %v, want only bar 3", meter)
	}
}

// Every bar has a band over it to pick it up by, and the bands cover the
// whole system without gaps — a dead patch over a barline would be a bar you
// could not select by aiming at it.
func TestEveryBarHasAClickStrip(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	for bar := 0; bar < 6; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		for i := 0; i < 4; i++ {
			h.note(0, bar, 2+i, "quarter") // a full bar: no empty space to click
		}
	}

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	lay := engrave.Engrave(h.app.score, opt)

	strips := map[int]engrave.StripPos{}
	for _, st := range lay.Strips {
		strips[st.Measure] = st
	}
	for i := 0; i < 6; i++ {
		st, ok := strips[i]
		if !ok {
			t.Errorf("bar %d has no band to click", i+1)
			continue
		}
		if st.X1 <= st.X0 {
			t.Errorf("bar %d's band has no width", i+1)
		}
		if st.Bottom <= st.Top {
			t.Errorf("bar %d's band has no height", i+1)
		}
	}

	// One band ends where the next begins.
	for i := 1; i < 6; i++ {
		if gap := strips[i].X0 - strips[i-1].X1; gap > 0.01 || gap < -0.01 {
			t.Errorf("a %.2f-space gap between the bands of bars %d and %d",
				gap, i, i+1)
		}
	}

	// The band sits above the music, not over it.
	for _, hit := range lay.Hits {
		st, ok := strips[hit.Measure]
		if !ok {
			continue
		}
		if st.Bottom > hit.StaffTop {
			t.Errorf("bar %d's band reaches down to %.2f, onto the staff at %.2f",
				hit.Measure+1, st.Bottom, hit.StaffTop)
		}
	}
}

// A slur arcs over the selected passage, and is drawn.
func TestSlurOverASelection(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}

	// One note is not a passage.
	if msg := h.postFails("/api/slur?part=0&measure=0&at=1"); !strings.Contains(msg, "select a passage") {
		t.Errorf("unhelpful refusal: %q", msg)
	}

	before := len(engrave.Engrave(h.app.score, engrave.DefaultOptions()).Curves)
	h.post("/api/slur?sel=0:0:0,0:0:1,0:0:2,0:0:3")

	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	if !els[0].(*model.Note).SlurStart {
		t.Error("the slur does not begin on the first note")
	}
	if !els[3].(*model.Note).SlurStop {
		t.Error("the slur does not end on the last note")
	}
	for i := 1; i < 3; i++ {
		n := els[i].(*model.Note)
		if n.SlurStart || n.SlurStop {
			t.Errorf("note %d is marked as an end of the slur", i)
		}
	}

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	if len(lay.Curves) != before+1 {
		t.Fatalf("got %d curves, want one more than %d", len(lay.Curves), before)
	}
	c := lay.Curves[len(lay.Curves)-1]
	if c.X2 <= c.X1 {
		t.Error("the slur does not run left to right")
	}
	// It arcs away from the notes rather than lying flat across them.
	mid := (c.Y1 + c.Y2) / 2
	if c.CY == mid {
		t.Error("the slur has no arch")
	}

	// And it can be taken off again.
	h.post("/api/slur?slur=none&sel=0:0:0,0:0:1,0:0:2,0:0:3")
	if len(engrave.Engrave(h.app.score, engrave.DefaultOptions()).Curves) != before {
		t.Error("the slur is still drawn after being removed")
	}
}

// A tie joins each note to the next, so three selected notes are all joined
// rather than the first arcing to the last.
func TestTieJoinsNeighbours(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 4, "quarter") // the same pitch, as a tie needs
	}
	h.post("/api/tie?sel=0:0:0,0:0:1,0:0:2")

	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	want := []struct{ start, stop bool }{
		{true, false}, {true, true}, {false, true},
	}
	for i, w := range want {
		n := els[i].(*model.Note)
		if n.TieStart != w.start || n.TieStop != w.stop {
			t.Errorf("note %d: start=%v stop=%v, want start=%v stop=%v",
				i, n.TieStart, n.TieStop, w.start, w.stop)
		}
	}

	// Two ties are drawn: one between each pair.
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	if len(lay.Curves) != 2 {
		t.Errorf("got %d curves, want 2", len(lay.Curves))
	}
	// And the three notes sound as one.
	if n := len(buildPlayback(h.app.score).Events); n != 1 {
		t.Errorf("three tied notes gave %d sounds, want 1", n)
	}
}

// A tied chord gets one tie per notehead, arcing outward so they do not
// cross the noteheads between them.
func TestTiedChordTiesEveryNotehead(t *testing.T) {
	h := newHarness(t)
	// Two bars, each holding the same two-note chord, tied across the bar.
	for bar := 0; bar < 2; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, bar, 2, "whole")
		h.post("/api/addpitch?part=0&measure=" + itoa(bar) + "&index=0&step=6")
	}
	h.post("/api/tie?sel=0:0:0,0:1:0")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	if len(lay.Curves) != 2 {
		t.Fatalf("a two-note chord tied across the bar drew %d curves, want 2",
			len(lay.Curves))
	}
	// One arcs up and one down, away from each other.
	a, b := lay.Curves[0], lay.Curves[1]
	upA := a.CY < (a.Y1+a.Y2)/2
	upB := b.CY < (b.Y1+b.Y2)/2
	if upA == upB {
		t.Error("both ties arc the same way; they should open away from each other")
	}
	// The one that arcs up belongs to the upper notehead.
	hi, lo := a, b
	if !upA {
		hi, lo = b, a
	}
	if hi.Y1 >= lo.Y1 {
		t.Error("the tie arcing up is not on the upper notehead")
	}
}

// One note selected is enough to say "hold it into the next one", which is
// what a tie means. The note it runs to is the one that follows, across the
// barline if that is where it is.
func TestTieFromOneNote(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 4, "half")
	h.note(0, 0, 4, "half")
	h.post("/api/addmeasure")
	h.note(0, 1, 4, "whole")

	// Within the bar.
	h.post("/api/tie?part=0&measure=0&at=1")
	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	if !els[0].(*model.Note).TieStart {
		t.Error("the first note does not start a tie")
	}
	if !els[1].(*model.Note).TieStop {
		t.Error("the second note does not end it")
	}

	// And across the barline, from the last note of a bar.
	h.post("/api/tie?part=0&measure=0&at=2")
	if !els[1].(*model.Note).TieStart {
		t.Error("the last note of the bar does not start a tie")
	}
	if !h.app.score.Parts[0].Measures[1].Voices[0].Elements[0].(*model.Note).TieStop {
		t.Error("the first note of the next bar does not end it")
	}

	// All four halves sound as one note.
	if n := len(buildPlayback(h.app.score).Events); n != 1 {
		t.Errorf("the tied chain gave %d sounds, want 1", n)
	}

	// Nothing after the last note, so there is nothing to tie to.
	if msg := h.postFails("/api/tie?part=0&measure=1&at=1"); msg == "" {
		t.Error("tying the last note of the piece was allowed")
	}
}

// A chord can be tied whole, or one notehead at a time.
func TestTieOneNoteheadOfAChord(t *testing.T) {
	h := newHarness(t)
	for bar := 0; bar < 2; bar++ {
		if bar > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, bar, 2, "whole")
		h.post("/api/addpitch?part=0&measure=" + itoa(bar) + "&index=0&step=6")
	}

	// Just the upper notehead, at step 6.
	h.post("/api/tie?part=0&measure=0&at=1&step=6")

	ch := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Chord)
	if ch.TieStart {
		t.Error("the whole chord was tied, not one notehead")
	}
	tied := 0
	for _, n := range ch.Notes {
		if n.TieStart {
			tied++
		}
	}
	if tied != 1 {
		t.Errorf("%d noteheads were tied, want 1", tied)
	}

	// One curve, not two.
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Errorf("got %d curves, want 1", len(lay.Curves))
	}

	// Asking for a notehead that is not there says so.
	if msg := h.postFails("/api/tie?part=0&measure=0&at=1&step=99"); msg == "" {
		t.Error("tying a notehead that does not exist was allowed")
	}
}

// A drum is written by name, at the place that drum belongs — not wherever
// the click happened to land.
func TestWritingADrumByName(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&clef=percussion")

	snare, ok := model.PercussionByKey(38)
	if !ok {
		t.Fatal("the kit has no snare drum")
	}
	// Ask for the snare while clicking at the top of the staff, where the
	// hi-hat lives. The snare must still come out, in the snare's place.
	h.post("/api/insert?part=0&measure=0&at=-1&step=10&dur=quarter&kind=note&drum=38")

	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if n.Percussion != 38 {
		t.Errorf("wrote drum %d, want the snare at 38", n.Percussion)
	}
	if got := engrave.StaffStep(model.Percussion, n.Pitch); got != snare.Step {
		t.Errorf("the snare was drawn at step %d, want %d", got, snare.Step)
	}
	if n.Head != snare.Head {
		t.Errorf("the snare was drawn with the %v notehead, want %v", n.Head, snare.Head)
	}

	// It plays as a snare.
	ev := buildPlayback(h.app.score).Events
	if len(ev) != 1 || ev[0].DrumName != "Snare drum" {
		t.Errorf("it sounded as %+v", ev)
	}

	// A key nothing in the kit uses is refused rather than guessed at.
	if msg := h.postFails(
		"/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note&drum=99"); msg == "" {
		t.Error("an unknown drum was accepted")
	}

	// Without a drum named, the position still decides, as before.
	h.post("/api/insert?part=0&measure=0&at=-1&step=10&dur=quarter&kind=note")
	last := h.app.score.Parts[0].Measures[0].Voices[0].Elements[1].(*model.Note)
	if last.Percussion != 42 {
		t.Errorf("clicking the top space wrote drum %d, want the hi-hat at 42",
			last.Percussion)
	}
}

// kitReply is the shape of /api/percussion: the kit being offered, the sizes
// to choose from, and the pads of the one in force.
type kitReply struct {
	Kit  string `json:"kit"`
	Kits []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Pads        int    `json:"pads"`
	} `json:"kits"`
	Pads []struct {
		Name     string `json:"name"`
		Key      int    `json:"key"`
		Step     int    `json:"step"`
		Notehead string `json:"notehead"`
		Shortcut string `json:"shortcut"`
	} `json:"pads"`
}

func (h *harness) kit(t *testing.T, query string) kitReply {
	t.Helper()
	var out kitReply
	if err := json.Unmarshal([]byte(h.get("/api/percussion"+query)), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// The kit is listed so the editor can offer it by name.
func TestKitIsListed(t *testing.T) {
	h := newHarness(t)
	kit := h.kit(t, "")
	if len(kit.Pads) < 10 {
		t.Fatalf("the kit has %d pieces", len(kit.Pads))
	}
	seen := map[string]bool{}
	for _, d := range kit.Pads {
		if d.Name == "" || d.Key == 0 {
			t.Errorf("a kit entry is incomplete: %+v", d)
		}
		if d.Notehead == "" {
			t.Errorf("%s has no notehead shape", d.Name)
		}
		seen[d.Name] = true
	}
	for _, want := range []string{"Snare drum", "Bass drum", "Hi-hat closed", "Ride cymbal"} {
		if !seen[want] {
			t.Errorf("the kit has no %s", want)
		}
	}
}

// A kit comes in sizes, because a rock groove wants six pads in front of it
// and a Latin chart wants forty.
func TestDrumKitSizes(t *testing.T) {
	h := newHarness(t)
	kit := h.kit(t, "")
	if len(kit.Kits) < 3 {
		t.Fatalf("got %d kit sizes, want at least three", len(kit.Kits))
	}
	for _, k := range kit.Kits {
		if k.ID == "" || k.Name == "" || k.Description == "" {
			t.Errorf("a kit size is not described: %+v", k)
		}
		if k.Pads == 0 {
			t.Errorf("%s has no pads", k.ID)
		}
	}

	// Smallest first, and they really are different sizes: a picker whose
	// entries all offer the same thing is a picker with nothing to pick.
	for i := 1; i < len(kit.Kits); i++ {
		if kit.Kits[i].Pads <= kit.Kits[i-1].Pads {
			t.Errorf("%s has %d pads and %s has %d: the sizes must grow",
				kit.Kits[i-1].ID, kit.Kits[i-1].Pads,
				kit.Kits[i].ID, kit.Kits[i].Pads)
		}
	}

	basic := h.kit(t, "?kit=basic")
	if basic.Kit != "basic" {
		t.Fatalf("asked for the basic kit, got %q", basic.Kit)
	}
	large := h.kit(t, "?kit=large")
	if len(large.Pads) <= len(basic.Pads) {
		t.Errorf("the large kit has %d pads and the basic one %d",
			len(large.Pads), len(basic.Pads))
	}

	// Every kit can write the groove everybody writes.
	for _, want := range []string{"Bass drum", "Snare drum", "Hi-hat closed"} {
		found := false
		for _, p := range basic.Pads {
			found = found || p.Name == want
		}
		if !found {
			t.Errorf("the basic kit cannot write a %s", want)
		}
	}

	// An unknown size opens something usable rather than nothing.
	if fallback := h.kit(t, "?kit=nonesuch"); len(fallback.Pads) == 0 {
		t.Error("an unknown kit name left the editor with no pads at all")
	}
}

// Each pad is typed with a letter, and no two pads share one.
func TestDrumPadShortcutsAreUnique(t *testing.T) {
	h := newHarness(t)
	for _, size := range []string{"basic", "standard", "large"} {
		kit := h.kit(t, "?kit="+size)
		seen := map[string]string{}
		for _, p := range kit.Pads {
			if p.Shortcut == "" {
				continue // a kit larger than the alphabet: mouse only
			}
			if other, dup := seen[p.Shortcut]; dup {
				t.Errorf("%s: %q types both %s and %s", size, p.Shortcut, other, p.Name)
			}
			seen[p.Shortcut] = p.Name
		}
		if len(seen) == 0 {
			t.Errorf("%s: no pad can be typed", size)
		}
	}
}

// The pads read down the staff the way the kit sits on it.
func TestDrumPadsAreOrderedByPosition(t *testing.T) {
	h := newHarness(t)
	kit := h.kit(t, "?kit=large")
	for i := 1; i < len(kit.Pads); i++ {
		if kit.Pads[i].Step > kit.Pads[i-1].Step {
			t.Fatalf("pad %d (%s, step %d) sits above pad %d (%s, step %d)",
				i, kit.Pads[i].Name, kit.Pads[i].Step,
				i-1, kit.Pads[i-1].Name, kit.Pads[i-1].Step)
		}
	}
}

// The drum kit is one of the instruments a staff can be, chosen the same way
// as any other — not a command of its own.
func TestDrumKitIsAnInstrumentChoice(t *testing.T) {
	h := newHarness(t)

	// It is in the list people choose from.
	var list []struct {
		Program    int    `json:"program"`
		Name       string `json:"name"`
		Family     string `json:"family"`
		Percussion bool   `json:"percussion"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/instruments")), &list); err != nil {
		t.Fatal(err)
	}
	var kit *struct {
		Program    int    `json:"program"`
		Name       string `json:"name"`
		Family     string `json:"family"`
		Percussion bool   `json:"percussion"`
	}
	for i := range list {
		if list[i].Percussion {
			kit = &list[i]
		}
	}
	if kit == nil {
		t.Fatal("the instrument list has no drum kit")
	}
	if kit.Name != "drum kit" || kit.Family != "percussion" {
		t.Errorf("the kit is listed as %+v", *kit)
	}
	// It is not a General MIDI program, and must not pretend to be.
	if kit.Program >= 0 {
		t.Errorf("the kit claims program %d; drums have none", kit.Program)
	}

	// Choosing it gives a staff whose lines name drums.
	h.post("/api/addstaff?instrument=drum%20kit")
	p := h.app.score.Parts[1]
	if p.Clef != model.Percussion {
		t.Errorf("the staff is in %v, want the percussion clef", p.Clef)
	}
	if p.Key.Fifths != 0 {
		t.Errorf("a drum staff is in %d sharps; it should be in no key", p.Key.Fifths)
	}

	// And it reports itself as a kit rather than as whatever the unused
	// program field happens to hold.
	var info struct {
		Parts []struct {
			Instrument string `json:"instrument"`
			Clef       string `json:"clef"`
		} `json:"parts"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/info")), &info); err != nil {
		t.Fatal(err)
	}
	if got := info.Parts[1].Instrument; got != "drum kit" {
		t.Errorf("the drum staff says it is a %q", got)
	}

	// An existing staff can be turned into one, and it writes drums.
	h.post("/api/staff?part=0&instrument=drum%20kit")
	if h.app.score.Parts[0].Clef != model.Percussion {
		t.Error("changing a staff to the kit did not change its clef")
	}
	h.post("/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note")
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if n.Percussion == 0 {
		t.Error("writing on the converted staff gave a pitch, not a drum")
	}
}

// A file the importer could only partly read reports every line it skipped,
// not a count of them.
//
// The person who wrote the file is the one who can correct it, so they get
// the list. Reducing it to the first line before anybody saw it left them
// with nothing to go on.
func TestImportLogListsEverySkippedLine(t *testing.T) {
	h := newHarness(t)

	// Two sentences in the middle of a tune: read as music they would each
	// become a bar of nonsense.
	src := "X:1\nT:Partly readable\nK:C\nabc\n" +
		"There should be a quaver rest after the last note.\n" +
		"def\n" +
		"The second strain is in the minor throughout.\n" +
		"gab\n"
	h.postBody("/api/open?name=partial.abc", src)

	var got struct {
		Warning string   `json:"warning"`
		Log     []string `json:"log"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/info")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Warning == "" {
		t.Fatal("a partly read file reported no warning at all")
	}
	if len(got.Log) != 2 {
		t.Fatalf("the log has %d entries, want 2: %v", len(got.Log), got.Log)
	}
	for _, line := range got.Log {
		if !strings.Contains(line, "line ") {
			t.Errorf("a log entry does not say which line: %q", line)
		}
	}

	// And the music around them was kept: that is the whole point of reading
	// past a mistake rather than refusing the file.
	if !strings.Contains(h.get("/api/info"), "Partly readable") {
		t.Error("the tune was not opened")
	}
}

// A file that reads cleanly leaves the log empty, so that the log window
// having anything in it means something.
func TestACleanFileLeavesNoLog(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/open?name=clean.abc", "X:1\nT:Clean\nK:C\nabcd|efga|\n")

	var got struct {
		Warning string   `json:"warning"`
		Log     []string `json:"log"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/info")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Warning != "" || len(got.Log) != 0 {
		t.Errorf("a clean file reported %q and %v", got.Warning, got.Log)
	}
}

/*
 * Repeats, endings and jumps
 */

// A volta bracket can be any number, and can cover a run of bars.
func TestEndingsTakeAnyNumber(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 6; i++ {
		h.post("/api/addmeasure")
	}
	h.post("/api/ending?measure=1&numbers=1")
	h.post("/api/ending?measure=2&to=3&numbers=2")
	h.post("/api/ending?measure=4&numbers=3")

	s := h.app.score
	want := []string{"", "1.", "2.", "2.", "3."}
	for i, w := range want {
		got := ""
		if e := s.Parts[0].Measures[i].Ending; e != nil {
			got = e.Label()
		}
		if got != w {
			t.Errorf("bar %d has ending %q, want %q", i+1, got, w)
		}
	}
	// The bracket over two bars opens on the first and closes on the second,
	// which is what the engraver draws the hook from.
	if e := s.Parts[0].Measures[2].Ending; e == nil || !e.Start || e.Stop {
		t.Errorf("the first bar of a two-bar bracket is %+v", e)
	}
	if e := s.Parts[0].Measures[3].Ending; e == nil || e.Start || !e.Stop {
		t.Errorf("the last bar of a two-bar bracket is %+v", e)
	}
}

// A bracket marked "1., 2." holds both numbers.
func TestAnEndingCanHoldSeveralNumbers(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addmeasure")
	h.post("/api/ending?measure=1&numbers=1,2")
	e := h.app.score.Parts[0].Measures[1].Ending
	if e == nil || e.Label() != "1., 2." {
		t.Fatalf("the bracket reads %q, want %q", e.Label(), "1., 2.")
	}
}

// Signs and directions are written on every staff, because they belong to the
// piece: a score where the flutes take the D.S. and the horns do not is not a
// score anybody can play from.
func TestAJumpIsWrittenOnEveryStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=40")
	for i := 0; i < 3; i++ {
		h.post("/api/addmeasure")
	}
	h.post("/api/jump?measure=0&sign=segno")
	h.post("/api/jump?measure=2&jump=ds-coda")

	for pi, p := range h.app.score.Parts {
		if p.Measures[0].Sign != model.Segno {
			t.Errorf("staff %d has no segno at bar 1", pi+1)
		}
		if j := p.Measures[2].Jump; j == nil || j.Name() != "ds-coda" {
			t.Errorf("staff %d has %v at bar 3, want ds-coda", pi+1, j)
		}
	}
}

// A sign and a direction can be written at the same bar in one go: a chart
// puts the coda sign and a D.S. al Coda in the same place often enough, and
// two calls would record two steps of undo for one thing done.
func TestASignAndADirectionTogether(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addmeasure")
	h.post("/api/jump?measure=1&sign=tocoda&jump=dc-coda")

	m := h.app.score.Parts[0].Measures[1]
	if m.Sign != model.ToCoda {
		t.Errorf("sign = %v, want tocoda", m.Sign)
	}
	if m.Jump == nil || m.Jump.Name() != "dc-coda" {
		t.Errorf("jump = %v, want dc-coda", m.Jump)
	}
}

// Writing one is undoable, like every other edit.
func TestAJumpCanBeTakenBack(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addmeasure")
	h.post("/api/jump?measure=1&jump=dc")
	if h.app.score.Parts[0].Measures[1].Jump == nil {
		t.Fatal("the direction was not written")
	}
	h.post("/api/undo")
	if j := h.app.score.Parts[0].Measures[1].Jump; j != nil {
		t.Errorf("after undo the bar still has %v", j)
	}
}

// Clearing one is how a direction written in the wrong bar is moved.
func TestAJumpCanBeCleared(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addmeasure")
	h.post("/api/jump?measure=1&sign=fine&jump=dc-fine")
	h.post("/api/jump?measure=1&sign=none&jump=none")

	m := h.app.score.Parts[0].Measures[1]
	if m.Sign != model.NoSign || m.Jump != nil {
		t.Errorf("the bar still has sign %v and jump %v", m.Sign, m.Jump)
	}
}

// A name that is not a direction is refused rather than quietly ignored: a
// button that writes nothing and says nothing is the worst kind of bug.
func TestANonsenseJumpIsRefused(t *testing.T) {
	h := newHarness(t)
	if msg := h.postFails("/api/jump?measure=0&jump=nonesuch"); msg == "" {
		t.Error("the refusal gave no reason")
	}
	if msg := h.postFails("/api/jump?measure=0&sign=nonesuch"); msg == "" {
		t.Error("the refusal gave no reason")
	}
	// And asking for neither is a mistake, not a no-op.
	h.postFails("/api/jump?measure=0")
}

// Playback follows the directions, which is the whole point of writing them.
func TestPlaybackFollowsAJump(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.post("/api/addmeasure")
	}
	for m := 0; m < 4; m++ {
		h.note(0, m, 4, "whole")
	}
	before := len(h.app.score.PlayOrder())

	h.post("/api/jump?measure=1&sign=fine")
	h.post("/api/jump?measure=3&jump=dc-fine")

	after := h.app.score.PlayOrder()
	if len(after) <= before {
		t.Fatalf("the piece plays %d bars with a D.C. al Fine and %d without", len(after), before)
	}
	// Down the four bars, then back for the first two, stopping at Fine.
	want := []int{0, 1, 2, 3, 0, 1}
	if len(after) != len(want) {
		t.Fatalf("plays %v, want %v", after, want)
	}
	for i := range want {
		if after[i] != want[i] {
			t.Fatalf("plays %v, want %v", after, want)
		}
	}
}
