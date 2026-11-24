//go:build !js

package app

import (
	"net/http"
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
		"Here are two bars.", "--- result ---", "bars placed from bar"} {
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

	logAssistant(assistantEntry{Prompt: "eight bars", Reply: "here they are"})
	raw, err := os.ReadFile(filepath.Join(dir, "ai.log"))
	if err != nil || !strings.Contains(string(raw), "eight bars") {
		t.Fatalf("no log: %v", err)
	}
	old, err := os.ReadFile(filepath.Join(dir, "gscore-0.0.9"))
	if err != nil || string(old) != "Gscore 0.0.9\n" {
		t.Errorf("the old file was not kept as it was: %q, %v", old, err)
	}
}
