//go:build !js

package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// keepLog points the assistant's log at a file of the test's own, and says
// where.
func keepLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".gscore", "ai.log")
	was := assistantLogPath
	assistantLogPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { assistantLogPath = was })
	return path
}

// What was asked of the assistant and what it answered are kept in a log,
// one entry after another: the prompt, the reply and what was done with it,
// and the errors as well as the answers.
func TestTheAssistantKeepsALogOfWhatWasSaid(t *testing.T) {
	var saw []*http.Request
	withAssistant(t, openAILike(t, twoVoices, &saw), "local")
	path := keepLog(t)
	h := newHarness(t)

	composed(h, "two bars of a waltz")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no log was written: %v", err)
	}
	log := string(raw)
	for _, want := range []string{"two bars of a waltz", "--- reply ---",
		"Here are two bars.", "--- result, after", "bars placed from bar"} {
		if !strings.Contains(log, want) {
			t.Errorf("the log does not have %q:\n%s", want, log)
		}
	}
	if info, err := os.Stat(path); err == nil && info.Mode().Perm() != 0o600 {
		t.Errorf("the log can be read by others: %v", info.Mode().Perm())
	}

	// A second request goes after the first, not over it.
	composed(h, "and two more")
	raw, _ = os.ReadFile(path)
	if !strings.Contains(string(raw), "two bars of a waltz") ||
		!strings.Contains(string(raw), "and two more") {
		t.Error("the second request did not go after the first")
	}
}

// A .gscore file left in the home folder by gscore 0.0.9 does not stop the
// log: it becomes the folder, with the old file inside it as it was.
func TestAnOldDotGscoreFileMakesWayForTheLog(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".gscore")
	if err := os.WriteFile(dir, []byte("Gscore 0.0.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	was := assistantLogPath
	assistantLogPath = func() (string, error) { return filepath.Join(dir, "ai.log"), nil }
	t.Cleanup(func() { assistantLogPath = was })

	e := assistantEntry{Prompt: "eight bars", Reply: "here they are"}
	logAssistantAsked(e)
	logAssistant(e)
	raw, err := os.ReadFile(filepath.Join(dir, "ai.log"))
	if err != nil || !strings.Contains(string(raw), "eight bars") {
		t.Fatalf("no log: %v", err)
	}
	old, err := os.ReadFile(filepath.Join(dir, "gscore-0.0.9"))
	if err != nil || string(old) != "Gscore 0.0.9\n" {
		t.Errorf("the old file was not kept as it was: %q, %v", old, err)
	}
}

// The question is in the log while the model is still at work on it, and
// a request that comes back with nothing says so, with how long it took.
func TestTheLogHasTheQuestionBeforeTheAnswer(t *testing.T) {
	path := keepLog(t)
	e := assistantEntry{Provider: "mistral", Model: "m", Prompt: "a band arrangement"}
	logAssistantAsked(e)
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "a band arrangement") {
		t.Fatal("the question is not in the log while it is being answered")
	}
	e.Err = "the model did not finish within 10m0s"
	logAssistant(e)
	raw, _ = os.ReadFile(path)
	for _, want := range []string{"(nothing arrived from the model)", "--- error, after"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the log does not have %q:\n%s", want, raw)
		}
	}
}

// What the model thinks goes into the log as it arrives, under its own
// heading and before the reply, and is not written a second time at the end.
func TestTheThinkingIsLoggedAsItArrives(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, bit := range []string{"The bass goes ", "to the trombone."} {
			sse(w, map[string]any{"choices": []map[string]any{
				{"delta": map[string]string{"reasoning_content": bit}}}})
		}
		sse(w, map[string]any{"choices": []map[string]any{
			{"delta": map[string]string{"content": "```abc\nV:1\nC4 | D4 |\n```\n"}}}})
		sse(w, map[string]any{"choices": []map[string]any{
			{"delta": map[string]string{}, "finish_reason": "stop"}}})
	}))
	withAssistant(t, srv, "local")
	path := keepLog(t)
	h := newHarness(t)
	composed(h, "for a band")

	raw, _ := os.ReadFile(path)
	log := string(raw)
	think := strings.Index(log, "--- thinking ---\nThe bass goes to the trombone.")
	reply := strings.Index(log, "--- reply ---")
	if think < 0 {
		t.Fatalf("the thinking is not in the log:\n%s", log)
	}
	if reply < think {
		t.Errorf("the reply is not after the thinking:\n%s", log)
	}
	if strings.Count(log, "The bass goes") != 1 || strings.Count(log, "--- reply ---") != 1 {
		t.Errorf("written twice:\n%s", log)
	}
	if !strings.Contains(log, "--- result, after") {
		t.Errorf("no outcome:\n%s", log)
	}
}
