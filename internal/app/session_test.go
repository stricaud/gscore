/*
 * session_test.go
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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stricaud/gscore/internal/model"
)

// More than one score open in one server.
//
// What has to be true: a request that names no session goes on working the
// way it always did, two sessions cannot see each other's music, the same
// file opened twice is the same session rather than two diverging copies,
// and nothing ever quietly throws away unsaved work.

// server builds a Server with its own default document, as the real one is
// built in Serve.
func testServer(t *testing.T) *Server {
	t.Helper()
	first := New(model.NewScore(), "")
	s := NewServer(first, first.newScoreApp())
	first.server = s
	return s
}

func do(t *testing.T, s *Server, method, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func mustPost(t *testing.T, s *Server, path string) map[string]any {
	t.Helper()
	code, out := do(t, s, http.MethodPost, path)
	if code != http.StatusOK {
		t.Fatalf("POST %s: %d", path, code)
	}
	return out
}

// Everything written before sessions existed sends none, and means "the
// score this server has". That is the whole of the migration plan, so it is
// the first thing that has to hold.
func TestARequestWithNoSessionIsTheDefaultOne(t *testing.T) {
	s := testServer(t)
	if _, out := do(t, s, http.MethodGet, "/api/info"); out == nil {
		t.Fatal("a request naming no session was not answered")
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/api/abc?part=0&measure=0&at=0", strings.NewReader("C D E F")))
	if rec.Code != http.StatusOK {
		t.Fatalf("writing with no session named: %d %s",
			rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	if s.Sessions() != 1 {
		t.Errorf("%d sessions after talking to the default one, want 1",
			s.Sessions())
	}
}

// Two documents, and neither can see the other's music. This is the point of
// the whole mechanism.
func TestTwoSessionsDoNotShareMusic(t *testing.T) {
	s := testServer(t)

	out := mustPost(t, s, "/api/session")
	second, _ := out["session"].(string)
	if second == "" {
		t.Fatal("no session token came back")
	}
	if second == defaultSession {
		t.Fatal("a new session reused the default one")
	}

	notes := func(session string) float64 {
		path := "/api/info"
		if session != "" {
			path += "?" + sessionParam + "=" + session
		}
		code, out := do(t, s, http.MethodGet, path)
		if code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, code)
		}
		parts, _ := out["parts"].([]any)
		if len(parts) == 0 {
			return 0
		}
		p, _ := parts[0].(map[string]any)
		n, _ := p["notes"].(float64)
		return n
	}

	// What each holds before anything is written. A brand new score is not
	// empty — it has a bar with a rest in it — so this is measured as a
	// change rather than against zero.
	wasDefault, wasSecond := notes(""), notes(second)

	// Write into the default session only.
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/api/abc?part=0&measure=0&at=0", strings.NewReader("C D E F")))
	if rec.Code != http.StatusOK {
		t.Fatalf("writing into the default session: %d %s",
			rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	if got := notes(""); got <= wasDefault {
		t.Fatalf("the default session went from %v to %v notes; "+
			"four were written into it", wasDefault, got)
	}
	if got := notes(second); got != wasSecond {
		t.Errorf("the second session went from %v to %v notes without "+
			"being written to", wasSecond, got)
	}
}

// A session that is not there is said to be not there, rather than quietly
// becoming a new empty score somebody then writes into and loses.
func TestAnUnknownSessionIsRefused(t *testing.T) {
	s := testServer(t)
	code, _ := do(t, s, http.MethodGet, "/api/info?"+sessionParam+"=nosuchthing")
	if code != http.StatusNotFound {
		t.Errorf("an unknown session answered %d, want 404", code)
	}
}

// Opening a file that is already open finds the session it is open in.
// Two copies of one file is how people lose work: they edit one, save the
// other, and the program was right both times.
func TestOpeningTheSameFileTwiceReattaches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tune.abc")
	if err := os.WriteFile(path, []byte("X:1\nT:Tune\nK:C\nCDEF|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := testServer(t)

	first := mustPost(t, s, "/api/session?path="+path)
	token, _ := first["session"].(string)
	if token == "" {
		t.Fatal("opening a file gave no session")
	}
	if again, _ := first["reattached"].(bool); again {
		t.Error("the first open claimed to reattach")
	}

	second := mustPost(t, s, "/api/session?path="+path)
	if got, _ := second["session"].(string); got != token {
		t.Errorf("opening it again gave session %q, want %q", got, token)
	}
	if again, _ := second["reattached"].(bool); !again {
		t.Error("opening it again did not say it had reattached")
	}
	if s.Sessions() != 2 { // the default, and the file
		t.Errorf("%d sessions, want 2 — the file was opened twice", s.Sessions())
	}
}

// The clipboard is the server's, so copying in one window and pasting in
// another does what anybody would expect.
func TestTheClipboardIsSharedBetweenSessions(t *testing.T) {
	s := testServer(t)
	a := s.Default()
	b := New(model.NewScore(), "")
	b.server = s

	if a.clipboard() != b.clipboard() {
		t.Error("two documents of one server have different clipboards")
	}
	// And a document with no server still has one of its own, which is what
	// every test that builds an App by itself relies on.
	lonely := New(model.NewScore(), "")
	if lonely.clipboard() == nil {
		t.Error("a document with no server has no clipboard")
	}
	if lonely.clipboard() == a.clipboard() {
		t.Error("a document with no server shares the server's clipboard")
	}
}

// Nothing ever drops unsaved work. A document somebody edited and did not
// save is the one thing here that cannot be reconstructed.
func TestSweepNeverDropsUnsavedWork(t *testing.T) {
	s := testServer(t)
	s.SetLimits(time.Nanosecond, 0) // everything is idle at once

	out := mustPost(t, s, "/api/session")
	token, _ := out["session"].(string)

	s.mu.Lock()
	s.sessions[token].app.unsaved = true
	s.mu.Unlock()

	if dropped := s.Sweep(time.Now().Add(time.Hour)); dropped != 0 {
		t.Fatalf("swept %d sessions; the only one had unsaved work", dropped)
	}
	if _, ok := s.use(token); !ok {
		t.Error("a session with unsaved work was dropped anyway")
	}

	// Saved, it goes.
	s.mu.Lock()
	s.sessions[token].app.unsaved = false
	s.mu.Unlock()
	if dropped := s.Sweep(time.Now().Add(time.Hour)); dropped != 1 {
		t.Errorf("swept %d sessions, want 1", dropped)
	}
}

// The document the server was started on is not swept, whatever happens:
// it is what a request naming no session is talking to.
func TestSweepKeepsTheDefaultSession(t *testing.T) {
	s := testServer(t)
	s.SetLimits(time.Nanosecond, 0)
	s.Sweep(time.Now().Add(time.Hour))
	if s.Default() == nil {
		t.Fatal("the default session was swept away")
	}
	if code, _ := do(t, s, http.MethodGet, "/api/info"); code != http.StatusOK {
		t.Errorf("after a sweep, a request naming no session answered %d", code)
	}
}

// A limit on how many may be open refuses rather than discarding, once
// everything open has unsaved work in it.
func TestAFullServerRefusesRatherThanDiscard(t *testing.T) {
	s := testServer(t)
	s.SetLimits(0, 2)

	out := mustPost(t, s, "/api/session")
	token, _ := out["session"].(string)
	s.mu.Lock()
	s.sessions[token].app.unsaved = true
	s.mu.Unlock()

	code, _ := do(t, s, http.MethodPost, "/api/session")
	if code != http.StatusBadRequest && code != http.StatusInternalServerError {
		t.Errorf("a full server of unsaved work answered %d; "+
			"it should refuse", code)
	}
	if _, ok := s.use(token); !ok {
		t.Error("the unsaved session was discarded to make room")
	}
}

func TestClosingASession(t *testing.T) {
	s := testServer(t)
	out := mustPost(t, s, "/api/session")
	token, _ := out["session"].(string)

	mustPost(t, s, "/api/session?close="+token)
	if code, _ := do(t, s, http.MethodGet, "/api/info?"+sessionParam+"="+token); code != http.StatusNotFound {
		t.Errorf("a closed session still answers %d", code)
	}
	// And the default one cannot be closed: it is what every request that
	// names nothing is talking to.
	code, _ := do(t, s, http.MethodPost, "/api/session?close="+defaultSession)
	if code != http.StatusBadRequest {
		t.Errorf("closing the default session answered %d, want 400", code)
	}
}

func TestSessionsAreListed(t *testing.T) {
	s := testServer(t)
	mustPost(t, s, "/api/session")
	code, out := do(t, s, http.MethodGet, "/api/sessions")
	if code != http.StatusOK {
		t.Fatalf("listing answered %d", code)
	}
	list, _ := out["sessions"].([]any)
	if len(list) != 2 {
		t.Fatalf("%d sessions listed, want 2", len(list))
	}
}

/*
 * The two identifiers
 */

var uuidV5 = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// The document key is a real v5 UUID and is the same answer every time,
// which is what makes reattaching work across a restart.
func TestTheDocumentKeyIsAStableV5UUID(t *testing.T) {
	a := documentKey("/tmp/some/score.gscore")
	b := documentKey("/tmp/some/score.gscore")
	if a != b {
		t.Errorf("the same path gave two keys: %q and %q", a, b)
	}
	if !uuidV5.MatchString(a) {
		t.Errorf("%q is not a v5 UUID", a)
	}
	if documentKey("/tmp/some/other.gscore") == a {
		t.Error("two different paths share a key")
	}
	if documentKey("") != "" {
		t.Error("a score with no file was given a key")
	}
}

// The token is a credential, so it is random: two of them differ, and
// neither can be worked out from anything the caller knows.
func TestTheSessionTokenIsRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		tok, err := newToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 32 {
			t.Fatalf("token %q is %d characters, want 32", tok, len(tok))
		}
		if seen[tok] {
			t.Fatalf("the same token came up twice: %q", tok)
		}
		seen[tok] = true
	}
	// And it is not the document key by another name: the whole reason
	// there are two identifiers is that this one cannot be guessed from
	// the file's path.
	if tok, _ := newToken(); tok == documentKey("/tmp/some/score.gscore") {
		t.Error("the token is derived from the path")
	}
}
