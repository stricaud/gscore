/*
 * script_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Programs kept with the music, and run as one thing.
 *
 * Two rules decide whether this is a tool or a liability. A script that
 * writes two hundred notes has to come back with one press, or nobody will
 * dare run one. And a score that could execute code on being opened is a
 * document that can attack the person who opens it, which is the mistake
 * Word made and spent a decade undoing — so what a file carries is text,
 * and a person reads it and decides.
 */

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func scriptsIn(t *testing.T, h *harness) []model.Script {
	t.Helper()
	var out struct {
		Scripts []model.Script `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/scripts")), &out); err != nil {
		t.Fatalf("reading them back: %v", err)
	}
	return out.Scripts
}

func writtenCount(t *testing.T, h *harness) int {
	t.Helper()
	h.app.mu.RLock()
	defer h.app.mu.RUnlock()
	n := 0
	for _, m := range h.app.score.Parts[0].Measures {
		for _, v := range m.Voices {
			n += len(v.Elements)
		}
	}
	return n
}

// Everything a script writes is taken back by one press.
func TestAScriptsEditsUndoAsOne(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	was := writtenCount(t, h)

	h.post("/api/script/begin")
	for i := 0; i < 8; i++ {
		h.note(0, 0, 6, "quarter")
	}
	h.post("/api/script/end")

	if writtenCount(t, h) <= was {
		t.Fatal("the script wrote nothing")
	}
	h.post("/api/undo")
	if got := writtenCount(t, h); got != was {
		t.Errorf("after one undo the score holds %d things, want the %d it "+
			"held before the script", got, was)
	}
}

// A script that calls another is still the one thing the person asked for.
func TestNestedScriptsAreOneUndo(t *testing.T) {
	h := newHarness(t)
	was := writtenCount(t, h)

	h.post("/api/script/begin")
	h.note(0, 0, 6, "quarter")
	h.post("/api/script/begin")
	h.note(0, 0, 6, "quarter")
	h.post("/api/script/end")
	h.note(0, 0, 6, "quarter")
	h.post("/api/script/end")

	h.post("/api/undo")
	if got := writtenCount(t, h); got != was {
		t.Errorf("after one undo the score holds %d things, want %d", got, was)
	}
}

// A script that did nothing leaves nothing to take back. An undo that does
// nothing is worse than no undo: it looks broken.
func TestAScriptThatDidNothingPushesNoUndo(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	h.note(0, 0, 8, "quarter")
	was := writtenCount(t, h)

	h.post("/api/script/begin")
	h.post("/api/script/end?changed=0")

	h.post("/api/undo")
	// The undo takes back the note before the script, not nothing at all.
	if got := writtenCount(t, h); got != was-1 {
		t.Errorf("the score holds %d things, want the %d before the last note",
			got, was-1)
	}
}

// The scripts go in the score and come back out of it.
func TestScriptsAreKeptWithTheScore(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/scripts",
		`{"scripts":[{"Name":"row","Body":"note(\"c4\")"}]}`)

	got := scriptsIn(t, h)
	if len(got) != 1 || got[0].Name != "row" || got[0].Body != `note("c4")` {
		t.Fatalf("they came back as %+v", got)
	}
}

func TestAScoreStartsWithNoScripts(t *testing.T) {
	if got := scriptsIn(t, newHarness(t)); len(got) != 0 {
		t.Errorf("a new score came with %+v", got)
	}
}

// A score is music with a note attached, not a source tree.
func TestAScriptTooLongIsRefused(t *testing.T) {
	h := newHarness(t)
	long := make([]byte, maxScript+1)
	for i := range long {
		long[i] = 'x'
	}
	body, _ := json.Marshal(map[string]any{
		"scripts": []model.Script{{Name: "big", Body: string(long)}},
	})
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scripts",
		strings.NewReader(string(body))))
	if rec.Code < 400 {
		t.Errorf("a %d byte script was accepted", len(long))
	}
}

// A script must have a name. A score may carry several and the name is the
// only thing that tells them apart: an unnamed script is one nobody can ask
// for, and two unnamed scripts are two nobody can tell apart.
func TestAScriptWithoutANameIsRefused(t *testing.T) {
	h := newHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scripts",
		strings.NewReader(`{"scripts":[{"Body":"note(\"c4\")"}]}`)))
	if rec.Code < 400 {
		t.Fatal("a script with no name was kept")
	}
	if !strings.Contains(rec.Body.String(), "name") {
		t.Errorf("it was refused without saying why: %q", rec.Body.String())
	}
	if got := scriptsIn(t, h); len(got) != 0 {
		t.Errorf("it was kept anyway: %+v", got)
	}
}

func TestTwoScriptsCannotShareAName(t *testing.T) {
	h := newHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scripts",
		strings.NewReader(`{"scripts":[{"Name":"row","Body":"a"},{"Name":"row","Body":"b"}]}`)))
	if rec.Code < 400 {
		t.Error("two scripts called row were kept")
	}
}

// A name of nothing but spaces is a name nobody can type.
func TestANameOfSpacesIsNoName(t *testing.T) {
	h := newHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scripts",
		strings.NewReader(`{"scripts":[{"Name":"   ","Body":"a"}]}`)))
	if rec.Code < 400 {
		t.Error("a script named with spaces was kept")
	}
}
