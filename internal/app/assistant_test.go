/*
 * assistant_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stricaud/gscore/internal/model"
)

// sse writes one server-sent event.
func sse(w http.ResponseWriter, v any) {
	raw, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", raw)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// compose asks the assistant and reads its streamed answer: the lines of
// progress, and the last line, which is done or error.
func compose(h *harness, prompt string) (lines []map[string]any, done map[string]any, errMsg string) {
	h.t.Helper()
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/api/assistant/compose?prompt="+url.QueryEscape(prompt), nil))
	if rec.Code != http.StatusOK {
		h.t.Fatalf("compose: %d %s", rec.Code, rec.Body.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(rec.Body.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) != nil {
			h.t.Fatalf("a line of the answer is not JSON: %q", line)
		}
		lines = append(lines, m)
		if d, ok := m["done"].(map[string]any); ok {
			done = d
		}
		if e, ok := m["error"].(string); ok {
			errMsg = e
		}
	}
	return lines, done, errMsg
}

// composed asks and expects the music to have been written.
func composed(h *harness, prompt string) map[string]any {
	h.t.Helper()
	_, done, errMsg := compose(h, prompt)
	if done == nil {
		h.t.Fatalf("the assistant did not write: %s", errMsg)
	}
	return done
}

/*
 * The assistant: a model asked for bars, and the bars written in.
 *
 * The services are stood in for by a server of the test's own, which
 * answers in each one's shape. What is checked is the conversation — the
 * right path, the right headers, the score sent along — and what gscore
 * does with the answer.
 */

// withAssistant switches the assistant on for one test and gives it a
// service to talk to.
func withAssistant(t *testing.T, srv *httptest.Server, provider string) {
	t.Helper()
	was := assistantOn
	assistantOn = true
	stored.Lock()
	stored.settings.Assistant = assistantSettings{Provider: provider, BaseURL: srv.URL, APIKey: "k-test"}
	stored.read = true
	stored.Unlock()
	t.Cleanup(func() {
		assistantOn = was
		stored.Lock()
		stored.settings.Assistant = assistantSettings{}
		stored.Unlock()
		srv.Close()
	})
}

// twoVoices is an answer: two bars for two staves, as a model is told to
// write them, with a sentence in front.
const twoVoices = "Here are two bars.\n```abc\nV:1\nC2 E2 G2 c2 | d2 c2 B2 A2 |\nV:2\nC,4 G,4 | F,4 G,4 |\n```\n"

// openAILike is a server in the OpenAI shape, which Mistral and a local
// model share.
func openAILike(t *testing.T, reply string, saw *[]*http.Request) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*saw = append(*saw, r)
		switch r.URL.Path {
		case "/models":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{
				{"id": "llama3"}, {"id": "gemma"}}})
		case "/chat/completions":
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			msgs, _ := req["messages"].([]any)
			if len(msgs) != 2 {
				t.Errorf("the request carried %d messages, want a system and a user one", len(msgs))
			}
			if req["stream"] != true {
				t.Errorf("the answer was not asked for as a stream")
			}
			user := msgs[len(msgs)-1].(map[string]any)["content"].(string)
			if !strings.Contains(user, "V:1") || !strings.Contains(user, "Request:") {
				t.Errorf("the score and the request were not sent: %q", user)
			}
			// In pieces, as a model writes it, with a pause partway so
			// that what has arrived is put on the page before the rest.
			half := len(reply) / 2
			if cut := strings.LastIndex(reply[:half], "|"); cut > 0 {
				half = cut + 1
			}
			for i, piece := range []string{reply[:half], reply[half:]} {
				if i == 1 {
					time.Sleep(450 * time.Millisecond)
				}
				sse(w, map[string]any{"choices": []map[string]any{
					{"delta": map[string]string{"content": piece}}}})
			}
			sse(w, map[string]any{"choices": []map[string]any{
				{"delta": map[string]string{}, "finish_reason": "stop"}}})
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
}

func twoStaves(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.app.score.Parts[0].Name = "Trumpet"
	h.app.score.Parts = append(h.app.score.Parts, &model.Part{
		Name: "Bass", Clef: model.Bass, Time: h.app.score.Parts[0].Time,
		Measures: []*model.Measure{model.NewMeasure()},
	})
	return h
}

func TestTheAssistantIsNotThereUnlessAskedFor(t *testing.T) {
	was := assistantOn
	assistantOn = false
	t.Cleanup(func() { assistantOn = was })
	h := newHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/assistant/settings", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("the assistant answered %d without AI_ASSISTANT=1", rec.Code)
	}
	if strings.Contains(h.get("/api/info"), "aiAssistant") {
		t.Error("the window is told about an assistant that is not there")
	}
}

func TestTheAssistantWritesTheBarsItIsAskedFor(t *testing.T) {
	var saw []*http.Request
	withAssistant(t, openAILike(t, twoVoices, &saw), "local")
	h := twoStaves(t)
	if !strings.Contains(h.get("/api/info"), `"aiAssistant":true`) {
		t.Error("the window is not told the assistant is there")
	}

	lines, d, _ := compose(h, "write two bars for everyone")
	if d == nil {
		t.Fatalf("nothing was written: %v", lines)
	}
	// The page was written on while the answer was still arriving.
	early := false
	for _, l := range lines {
		if p, ok := l["progress"].(map[string]any); ok && p["placed"] != nil {
			early = true
		}
	}
	if !early {
		t.Errorf("no bars were put on the page before the answer was finished: %v", lines)
	}
	if d["bars"] != 2.0 || d["at"] != 1.0 {
		t.Errorf("wrote %v bars at %v, want 2 at bar 1", d["bars"], d["at"])
	}
	if d["prose"] != "Here are two bars." {
		t.Errorf("the model's words came out as %q", d["prose"])
	}
	s := h.app.score
	if s.MeasureCount() != 2 {
		t.Fatalf("the score has %d bars, want 2: the empty first bar and one more", s.MeasureCount())
	}
	if n := len(s.Parts[0].Measures[0].Voices[0].Elements); n != 4 {
		t.Errorf("the trumpet's first bar holds %d notes, want 4", n)
	}
	if n := len(s.Parts[1].Measures[1].Voices[0].Elements); n != 2 {
		t.Errorf("the bass's second bar holds %d notes, want 2", n)
	}
	// The key went as a bearer token, the OpenAI way.
	if got := saw[len(saw)-1].Header.Get("Authorization"); got != "Bearer k-test" {
		t.Errorf("the key was sent as %q", got)
	}
	// And the whole thing can be taken back.
	h.post("/api/undo")
	if h.app.score.MeasureCount() != 1 {
		t.Errorf("undo left %d bars", h.app.score.MeasureCount())
	}
}

func TestNewBarsGoAfterTheBarAtTheCursorWhenItIsWritten(t *testing.T) {
	var saw []*http.Request
	withAssistant(t, openAILike(t, twoVoices, &saw), "mistral")
	h := twoStaves(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=whole&kind=note")
	h.app.score.Cursor = &model.Cursor{Part: 0, Measure: 0}

	d := composed(h, "two more")
	if d["at"] != 2.0 {
		t.Errorf("the new bars begin at bar %v, want 2, after the one with music in it", d["at"])
	}
	if n := h.app.score.MeasureCount(); n != 3 {
		t.Errorf("the score has %d bars, want 3", n)
	}
	if n := len(h.app.score.Parts[0].Measures[0].Voices[0].Elements); n != 1 {
		t.Errorf("the written bar was overwritten: %d elements", n)
	}
}

func TestTheAssistantSpeaksAnthropic(t *testing.T) {
	var saw []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = append(saw, r)
		if r.Header.Get("x-api-key") != "k-test" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("an Anthropic request without its key and version headers: %v", r.Header)
		}
		switch r.URL.Path {
		case "/models":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{
				{"id": "claude-opus-5-5"}, {"id": "claude-sonnet-5-5"}}})
		case "/messages":
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			if req["system"] == nil || req["max_tokens"] == nil || req["stream"] != true {
				t.Errorf("the request lacks a system prompt, max_tokens or stream: %v", req)
			}
			sse(w, map[string]any{"type": "message_start"})
			sse(w, map[string]any{"type": "content_block_delta",
				"delta": map[string]string{"type": "thinking_delta", "thinking": ""}})
			sse(w, map[string]any{"type": "content_block_delta",
				"delta": map[string]string{"type": "text_delta", "text": twoVoices}})
			sse(w, map[string]any{"type": "message_delta",
				"delta": map[string]string{"stop_reason": "end_turn"}})
			sse(w, map[string]any{"type": "message_stop"})
		default:
			http.NotFound(w, r)
		}
	}))
	withAssistant(t, srv, "anthropic")
	h := twoStaves(t)

	var listed struct{ Models []string }
	json.Unmarshal([]byte(h.get("/api/assistant/models")), &listed)
	if len(listed.Models) != 2 || listed.Models[0] != "claude-opus-5-5" {
		t.Errorf("the models came out as %v", listed.Models)
	}
	d := composed(h, "two bars")
	if d["bars"] != 2.0 {
		t.Errorf("wrote %v bars, want 2", d["bars"])
	}
}

func TestADeclinedRequestIsSaidSo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, map[string]any{"type": "message_start"})
		sse(w, map[string]any{"type": "message_delta", "delta": map[string]any{
			"stop_reason":  "refusal",
			"stop_details": map[string]string{"type": "refusal", "explanation": "not this one"},
		}})
	}))
	withAssistant(t, srv, "anthropic")
	h := twoStaves(t)
	_, done, msg := compose(h, "x")
	if done != nil || !strings.Contains(msg, "not this one") {
		t.Errorf("the refusal came out as %q", msg)
	}
}

func TestAServiceErrorIsPassedOnInItsOwnWords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "bad key"}})
	}))
	withAssistant(t, srv, "openai")
	h := twoStaves(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/assistant/models", nil))
	if rec.Code == http.StatusOK || !strings.Contains(rec.Body.String(), "bad key") {
		t.Errorf("the service's refusal came out as %d %q", rec.Code, rec.Body.String())
	}
}

func TestTheSettingsAreKeptAndTheModelsAskedForWithWhatIsTyped(t *testing.T) {
	var saw []*http.Request
	withAssistant(t, openAILike(t, twoVoices, &saw), "local")
	h := twoStaves(t)
	h.post("/api/assistant/settings?provider=openai&apiKey=sk-x&model=gpt-4o")
	var s struct{ Provider, Model, APIKey string }
	json.Unmarshal([]byte(h.get("/api/assistant/settings")), &s)
	if s.Provider != "openai" || s.Model != "gpt-4o" || s.APIKey != "sk-x" {
		t.Errorf("the settings came back as %+v", s)
	}
	// Listing with the local service named in the request, not the saved
	// OpenAI one: a key is tried before it is kept.
	base := stored.settings.Assistant.BaseURL
	stored.settings.Assistant.BaseURL = ""
	t.Cleanup(func() { stored.settings.Assistant.BaseURL = base })
	var listed struct{ Models []string }
	json.Unmarshal([]byte(h.get("/api/assistant/models?provider=local&baseURL="+url.QueryEscape(base))), &listed)
	if len(listed.Models) != 2 {
		t.Errorf("the local service's models came out as %v", listed.Models)
	}
}

// The score goes to the model without gscore's beaming marks, which are
// the engraver's business and only noise to a composer.
func TestTheScoreIsSentWithoutItsBeamingMarks(t *testing.T) {
	got := assistantRequest("K:C\nV:1\n!beamstart!C/2 !beamcont!D/2 !nobeam!E |", "more", 1)
	if strings.Contains(got, "!beam") || strings.Contains(got, "!nobeam!") {
		t.Errorf("the beaming marks were sent: %q", got)
	}
	if !strings.Contains(got, "C/2 D/2 E |") {
		t.Errorf("the music was not sent whole: %q", got)
	}
}

func TestTheABCIsFoundInAReply(t *testing.T) {
	header := "X:1\nM:4/4\nL:1/8\nK:C"
	abc, prose := abcOfReply("Sure.\n```abc\nV:1\nC4 D4 |\n```\nDone.", header)
	if !strings.HasPrefix(abc, header) || !strings.Contains(abc, "C4 D4 |") {
		t.Errorf("the ABC came out as %q", abc)
	}
	if prose != "Sure. Done." {
		t.Errorf("the prose came out as %q", prose)
	}
	// A reply with headers of its own keeps them, and an unfenced one is
	// taken whole.
	abc, _ = abcOfReply("M:3/4\nL:1/4\nK:G\nV:1\nG A B |", header)
	if strings.Contains(abc, "M:4/4") || !strings.HasPrefix(abc, "X:1\nM:3/4") {
		t.Errorf("the reply's own headers were not kept: %q", abc)
	}
}

// A request for instruments the score does not have is a request for
// those staves: the answer's extra voices become staves, named and clefed
// as the answer wrote them, and the one nameless empty staff a new score
// opens with takes the first voice's name.
func TestVoicesTheScoreHasNoStaffForBecomeStaves(t *testing.T) {
	reply := "```abc\nV:1 clef=treble name=\"Flute\"\nc4 d4 |\nV:2 clef=treble name=\"Alto Sax\"\nG4 A4 |\nV:3 clef=bass name=\"Bass\"\nC,4 D,4 |\n```"
	var saw []*http.Request
	withAssistant(t, openAILike(t, reply, &saw), "local")
	h := newHarness(t)
	d := composed(h, "flute, sax and bass")
	if d["voices"] != 3.0 || d["staves"] != 3.0 {
		t.Errorf("the answer's %v voices went onto %v staves", d["voices"], d["staves"])
	}
	s := h.app.score
	if len(s.Parts) != 3 {
		t.Fatalf("the score has %d staves, want 3", len(s.Parts))
	}
	for i, want := range []string{"Flute", "Alto Sax", "Bass"} {
		if s.Parts[i].Name != want {
			t.Errorf("staff %d is called %q, want %q", i+1, s.Parts[i].Name, want)
		}
		if n := len(s.Parts[i].Measures[0].Voices[0].Elements); n != 2 {
			t.Errorf("staff %d holds %d notes, want 2", i+1, n)
		}
	}
	if s.Parts[2].Clef != model.Bass {
		t.Errorf("the bass is in clef %v", s.Parts[2].Clef)
	}
	// And each plays what its name says, not a piano: the answer wrote
	// no program numbers, so the names are what there is to go on.
	for i, want := range []string{"wind.flutes.flute", "wind.reed.saxophone.alto"} {
		if s.Parts[i].Sound != want {
			t.Errorf("staff %d sounds as %q, want %q", i+1, s.Parts[i].Sound, want)
		}
	}
	if s.Parts[0].Instrument == 0 {
		t.Error("the flute has no program to play with")
	}
}

// A drum staff the answer adds is a drum staff: percussion clef, no key,
// and a kit to play with.
func TestADrumVoiceBecomesADrumStaff(t *testing.T) {
	reply := "```abc\nV:1 clef=treble name=\"Flute\"\nc4 d4 |\nV:2 clef=perc name=\"Drums\"\nB4 B4 |\n```"
	var saw []*http.Request
	withAssistant(t, openAILike(t, reply, &saw), "local")
	h := newHarness(t)
	composed(h, "flute and drums")
	s := h.app.score
	if len(s.Parts) != 2 {
		t.Fatalf("the score has %d staves, want 2", len(s.Parts))
	}
	d := s.Parts[1]
	if d.Clef != model.Percussion || !model.SoundIsPercussion(d.Sound) {
		t.Errorf("the drums are in clef %v and sound as %q", d.Clef, d.Sound)
	}
}

// A request that changes what is written is answered with the whole score,
// which replaces what was there: here the piece is moved to G and given a
// title.
func TestAWholeScoreAnswerReplacesTheMusic(t *testing.T) {
	reply := "Moved to G.\n```abc\n%%whole\nX:1\nT:Air\nM:4/4\nL:1/8\nQ:1/4=90\nK:G\nV:1 clef=treble name=\"Flute\"\nG4 A4 | B8 |\n```"
	var saw []*http.Request
	withAssistant(t, openAILike(t, reply, &saw), "local")
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=whole&kind=note")
	h.app.score.Cursor = &model.Cursor{Part: 0, Measure: 0}
	d := composed(h, "put it in G")
	if d["whole"] != true || d["bars"] != 2.0 {
		t.Errorf("the answer was taken as %v", d)
	}
	s := h.app.score
	if s.MeasureCount() != 2 || s.Parts[0].Key.Fifths != 1 || s.Parts[0].Name != "Flute" {
		t.Errorf("the score is now %d bars, %d sharps, staff %q", s.MeasureCount(), s.Parts[0].Key.Fifths, s.Parts[0].Name)
	}
	if s.Identity.Title != "Air" || s.Tempo.BPM != 90 {
		t.Errorf("the title and tempo came out as %q and %d", s.Identity.Title, s.Tempo.BPM)
	}
	h.post("/api/undo")
	if h.app.score.Parts[0].Key.Fifths != 0 {
		t.Error("undo did not bring the old score back")
	}
}

// Some Mistral models write the words as a list of parts rather than a
// string; read as a string that was no answer at all.
func TestAnAnswerWrittenInPartsIsRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{
			"content": []map[string]string{{"type": "text", "text": twoVoices}}}}}})
		sse(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{"content": nil},
			"finish_reason": "stop"}}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	withAssistant(t, srv, "mistral")
	h := twoStaves(t)
	if d := composed(h, "two bars"); d["bars"] != 2.0 {
		t.Errorf("wrote %v bars, want 2", d["bars"])
	}
}

// A model that thinks before it writes sends its reasoning first, as
// thinking parts, and the words only after. The panel has to hear about
// the thinking, or a hard request shows nothing for its first minute.
func TestAModelsThinkingIsReportedWhileItThinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{
			"content": []map[string]any{{"type": "thinking", "closed": true,
				"thinking": []map[string]string{{"type": "text", "text": "The user wants two bars"}}}}}}}})
		time.Sleep(450 * time.Millisecond)
		sse(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{
			"content": []map[string]any{
				{"type": "thinking", "closed": true, "thinking": []map[string]string{{"type": "text", "text": " in C."}}},
				{"type": "text", "text": twoVoices}}},
			"finish_reason": "stop"}}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	withAssistant(t, srv, "mistral")
	h := twoStaves(t)
	lines, done, errMsg := compose(h, "two bars")
	if done == nil {
		t.Fatalf("nothing was written: %s", errMsg)
	}
	thought := false
	for _, l := range lines {
		if p, ok := l["progress"].(map[string]any); ok {
			if th, _ := p["thinking"].(string); strings.Contains(th, "two bars") {
				thought = true
			}
		}
	}
	if !thought {
		t.Errorf("the thinking was not reported: %v", lines)
	}
	if done["bars"] != 2.0 {
		t.Errorf("wrote %v bars, want 2", done["bars"])
	}
}
