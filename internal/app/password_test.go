/*
 * password_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Opening and saving a score locked under a password.
 */

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/symbol"
)

// lockedFile is a .gscore saved under "hunter2".
func lockedFile(t *testing.T) []byte {
	t.Helper()
	s := model.NewScore()
	s.Identity.Title = "Locked"
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	p.Measures = []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{
		&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 1}},
	}}}}}
	s.Parts = []*model.Part{p}
	var buf bytes.Buffer
	if err := gscorefmt.WriteEncrypted(&buf, s, symbol.NewRegistry(), "hunter2"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (h *harness) openWith(body []byte, password string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/open?name=locked.gscore", bytes.NewReader(body))
	if password != "" {
		req.Header.Set(passwordHeader, password)
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func TestALockedFileAsksForItsPassword(t *testing.T) {
	h := newHarness(t)
	data := lockedFile(t)

	rec := h.openWith(data, "")
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("X-Needs-Password") != "1" {
		t.Fatalf("opened with no password: %d %q, header %q", rec.Code, rec.Body.String(), rec.Header().Get("X-Needs-Password"))
	}
	rec = h.openWith(data, "wrong")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "not the password") {
		t.Fatalf("opened with the wrong password: %d %q", rec.Code, rec.Body.String())
	}
	rec = h.openWith(data, "hunter2")
	if rec.Code != http.StatusOK {
		t.Fatalf("opened with the password: %d %q", rec.Code, rec.Body.String())
	}
	if h.app.score.Identity.Title != "Locked" || h.app.password != "hunter2" {
		t.Errorf("the score is %q and the password kept is %q", h.app.score.Identity.Title, h.app.password)
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/info")), &info); err != nil {
		t.Fatal(err)
	}
	if info["protected"] != true {
		t.Errorf("info says protected %v, want true", info["protected"])
	}
}

func TestAPasswordLocksTheNextSave(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.bars(1)

	// Setting one needs the header, and is a change to the document.
	if msg := h.postFails("/api/password"); !strings.Contains(msg, passwordHeader) {
		t.Errorf("setting no password was answered %q", msg)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/password", nil)
	req.Header.Set(passwordHeader, "hunter2")
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || h.app.password != "hunter2" || !h.app.unsaved {
		t.Fatalf("setting the password: %d %q, kept %q, unsaved %v", rec.Code, rec.Body.String(), h.app.password, h.app.unsaved)
	}

	path := filepath.Join(t.TempDir(), "locked.gscore")
	if err := h.app.writeScore(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := gscorefmt.Read(bytes.NewReader(data)); !errors.Is(err, gscorefmt.ErrPasswordNeeded) {
		t.Errorf("the saved file opens without a password: %v", err)
	}
	if _, _, err := gscorefmt.ReadWithPassword(bytes.NewReader(data), "hunter2"); err != nil {
		t.Errorf("the saved file does not open with its password: %v", err)
	}
	// And nothing in front of it: a locked file carries no PDF.
	if pdf := gscorefmt.PDFPrefix(data); len(pdf) != 0 {
		t.Errorf("the locked file carries %d bytes of PDF in front", len(pdf))
	}

	// Taking it off saves an open file again.
	h.post("/api/password?clear=1")
	if h.app.password != "" {
		t.Errorf("after clearing, the password is still %q", h.app.password)
	}
	if err := h.app.writeScore(path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if _, _, err := gscorefmt.Read(bytes.NewReader(data)); err != nil {
		t.Errorf("the file saved unlocked does not open: %v", err)
	}

	// And a new score starts with no password.
	req = httptest.NewRequest(http.MethodPost, "/api/password", nil)
	req.Header.Set(passwordHeader, "again")
	h.mux.ServeHTTP(httptest.NewRecorder(), req)
	h.post("/api/new")
	if h.app.password != "" {
		t.Errorf("a new score carries the old password %q", h.app.password)
	}
}

// A locked file the shell hands over is put aside for the interface to ask
// about, rather than refused.
func TestALockedFileFromTheShellWaitsForItsPassword(t *testing.T) {
	h := newHarness(t)
	// Opening by path is for the machine the editor runs on.
	h.app.local = true
	path := filepath.Join(t.TempDir(), "shell.gscore")
	if err := os.WriteFile(path, lockedFile(t), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.app.OpenPath(path); err != nil {
		t.Fatalf("OpenPath: %v", err)
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/info")), &info); err != nil {
		t.Fatal(err)
	}
	if info["askPassword"] != path {
		t.Errorf("info asks for the password of %v, want %q", info["askPassword"], path)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/openpath?path="+path, nil)
	req.Header.Set(passwordHeader, "hunter2")
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("opening by path with the password: %d %q", rec.Code, rec.Body.String())
	}
	// Into a fresh map: decoding into the one above would keep its keys.
	var after map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/info")), &after); err != nil {
		t.Fatal(err)
	}
	if _, still := after["askPassword"]; still {
		t.Error("the file is still waiting for its password after being opened")
	}
	if h.app.score.Identity.Title != "Locked" {
		t.Errorf("the score is %q", h.app.score.Identity.Title)
	}
}
