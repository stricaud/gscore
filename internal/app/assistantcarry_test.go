package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A model that says the request is too long is asked again with only the
// bars around the cursor, said to be an excerpt — and what it writes then
// goes in as usual.
func TestATooLongRequestIsAskedAgainWithAnExcerpt(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		msgs, _ := req["messages"].([]any)
		user := msgs[len(msgs)-1].(map[string]any)["content"].(string)
		asked = append(asked, user)
		if len(asked) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
				"message": "This model's maximum context length is 4096 tokens: context window exceeded"}})
			return
		}
		sse(w, map[string]any{"choices": []map[string]any{
			{"delta": map[string]string{"content": "```abc\nV:1\nC4 | D4 |\n```\n"}}}})
		sse(w, map[string]any{"choices": []map[string]any{
			{"delta": map[string]string{}, "finish_reason": "stop"}}})
	}))
	withAssistant(t, srv, "local")
	h := newHarness(t)
	h.bars(40)

	lines, done, errMsg := compose(h, "two more bars")
	if done == nil {
		t.Fatalf("nothing written after asking again: %s", errMsg)
	}
	if len(asked) != 2 {
		t.Fatalf("asked %d times, want twice", len(asked))
	}
	if !strings.Contains(asked[1], "excerpt") || strings.Count(asked[1], "|") >= strings.Count(asked[0], "|") {
		t.Errorf("the second request was not an excerpt of the first:\n%s", asked[1])
	}
	said := false
	for _, l := range lines {
		if p, ok := l["progress"].(map[string]any); ok && p["note"] != nil {
			said = true
		}
	}
	if !said {
		t.Error("the window was not told the request was being asked again")
	}
}

// A model that gives out partway leaves the bars it had finished on the page,
// and the answer says which, and where to carry on from.
func TestBarsWrittenBeforeAFailureAreKept(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, map[string]any{"choices": []map[string]any{
			{"delta": map[string]string{"content": "```abc\nV:1\nC4 | D4 | E4 |"}}}})
		// And then the line goes dead.
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	withAssistant(t, srv, "local")
	h := newHarness(t)
	before := h.app.score.MeasureCount()

	lines, done, _ := compose(h, "a few bars")
	if done != nil {
		t.Fatal("an answer cut off partway was taken as finished")
	}
	var kept map[string]any
	for _, l := range lines {
		if k, ok := l["kept"].(map[string]any); ok {
			kept = k
		}
	}
	if kept == nil {
		t.Fatalf("the bars written were not kept: %v", lines)
	}
	if n, _ := kept["bars"].(float64); n < 1 {
		t.Errorf("kept %v bars", kept["bars"])
	}
	if h.app.score.MeasureCount() <= before && !scoreHasNotes(h.app.score) {
		t.Error("the score has nothing of what was written")
	}
	if kept["next"] == nil {
		t.Error("the answer does not say where to carry on from")
	}
}

// scoreHasNotes says any bar has a note in it.
func scoreHasNotes(s *model.Score) bool {
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if _, ok := el.(*model.Note); ok {
						return true
					}
				}
			}
		}
	}
	return false
}

// The ways the services say a request was too long are all heard as that,
// and an ordinary refusal is not.
func TestContextOverflowIsRecognised(t *testing.T) {
	for _, msg := range []string{
		"context window exceeded",
		"This model's maximum context length is 8192 tokens",
		"prompt is too long: 210000 tokens > 200000 maximum",
		"the input is too long for the context size",
	} {
		if !contextOverflow(errString(msg)) {
			t.Errorf("not recognised: %q", msg)
		}
	}
	if contextOverflow(errString("invalid API key")) {
		t.Error("a bad key was taken for a request too long")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// How long a request is waited for is a setting, kept within reason, and a
// request that runs out of time is said to have, not to have been too long.
func TestHowLongToWaitIsASetting(t *testing.T) {
	if d := (assistantSettings{}).timeout(); d != assistantTimeout {
		t.Errorf("unset, the wait is %v; want the default %v", d, assistantTimeout)
	}
	if d := (assistantSettings{TimeoutMinutes: 30}).timeout(); d.Minutes() != 30 {
		t.Errorf("set to 30 minutes, the wait is %v", d)
	}
	if d := (assistantSettings{TimeoutMinutes: 999}).timeout(); d.Minutes() != 120 {
		t.Errorf("set past two hours, the wait is %v", d)
	}
	if contextOverflow(errString(`Post "https://api.mistral.ai/v1/chat/completions": context deadline exceeded`)) {
		t.Error("a request that ran out of time was taken for one too long for the model")
	}
	if !timedOut(errString("context deadline exceeded")) {
		t.Error("running out of time was not heard as that")
	}

	var saw []*http.Request
	withAssistant(t, openAILike(t, twoVoices, &saw), "local")
	h := newHarness(t)
	h.post("/api/assistant/settings?timeout=45")
	if got := loadSettings().Assistant.TimeoutMinutes; got != 45 {
		t.Errorf("the setting saved as %d minutes", got)
	}
	if msg := h.postFails("/api/assistant/settings?timeout=500"); msg == "" {
		t.Error("a wait of 500 minutes was accepted")
	}
}
