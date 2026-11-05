/*
 * session.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * More than one score open at once, in one server.
 *
 * An App is a document: a score, the book it came from, its undo history, the
 * view it is being looked at in, whether it has been saved. Everything on it
 * is about one piece of music. So an App is already a session, and the whole
 * of this file is a way of having more than one of them and saying which.
 *
 * Which is why it is small. The alternative was to give every one of the
 * hundred and twenty-eight handlers a session to look things up in, and that
 * is a change to two hundred and seventy places that each have to be got
 * right, for no behaviour that this does not already give. Here each App
 * builds its own router once and the server picks which router answers. The
 * handlers do not know sessions exist.
 *
 * Two identifiers, doing two different jobs, and the difference matters.
 *
 *   The token is what a client sends. It is random, because it is a
 *   credential: whoever has it can read and overwrite that score.
 *
 *   The document key is a UUIDv5 of the file's path. It is deterministic on
 *   purpose — that is what makes "open the file I already have open" find the
 *   session it is already in rather than making a second, diverging copy of
 *   the same piece.
 *
 * A name-based UUID is the right answer to the second question and the wrong
 * answer to the first. v5 is a hash of a namespace and a name, so anybody who
 * knows both can work it out; used as the credential it would mean that
 * knowing somebody's file path is knowing how to overwrite their work, which
 * on a server started with -serve is a real person on a real network.
 */

// sessionParam is what a request calls its session.
//
// A query parameter rather than a cookie. The API is driven from curl and
// from scripts at least as often as from the window, and a credential a
// browser sends by itself is one a script cannot see — and one that invites
// cross-site requests to a mutating API from any page the person has open.
// Visible and passed on purpose is the right shape for this.
const sessionParam = "session"

// defaultSession is the session a request that names none is talking to.
//
// Sessions are an addition, not a change. Everything written against gscore
// before they existed — the player in an embedded page, the WebAssembly
// build, the whole of the curl documentation — sends no session and means
// "the score this server has". That keeps working, exactly as it did, and
// nothing has to be rewritten to take advantage of a feature it does not use.
const defaultSession = "default"

// sessionNamespace is the UUID namespace the document keys are built in.
//
// A fixed, arbitrary UUID, as the standard asks for. It is in the source
// rather than generated per run because the whole value of a name-based key
// is that it is the same answer every time: generated at startup, reopening
// a file after a restart would no longer find it.
var sessionNamespace = [16]byte{
	0x9f, 0x1a, 0x47, 0x2c, 0x7d, 0x53, 0x4e, 0x18,
	0xb6, 0x0c, 0x31, 0xe5, 0xa8, 0x74, 0x92, 0xd3,
}

// Server holds the open documents and hands each request to the right one.
type Server struct {
	mu       sync.RWMutex
	sessions map[string]*session
	// byDoc finds the session a file is already open in, by its document
	// key. See documentKey.
	byDoc map[string]string

	// clip is the clipboard, and it is the server's rather than any one
	// document's.
	//
	// Copying in one window and pasting in another is the first thing
	// anybody tries once there are two windows, and a clipboard per
	// document would quietly do nothing. It is the one piece of state here
	// that is deliberately shared.
	clip clipboard

	// newApp makes a session's App. A field so that a test can watch what
	// is made without reaching into the package.
	newApp func() *App

	// idle is how long a saved, untouched session is kept. Zero means for
	// ever, which is what a desktop gscore wants.
	idle time.Duration
	// most is how many sessions may exist at once. Zero is no limit.
	most int
}

// session is one open document and what the server knows about it.
type session struct {
	app   *App
	token string
	// doc is the document key this session is registered under, where it
	// has a file. Empty for a score that has never been saved.
	doc  string
	used time.Time
}

// NewServer wraps an App as the default session and serves the rest beside
// it.
//
// The App given is the document gscore was started on — the file named on
// the command line, or an empty score. It becomes the session that requests
// naming none are talking to, so a server with sessions behaves exactly like
// one without until somebody asks for a second document.
func NewServer(first *App, newApp func() *App) *Server {
	s := &Server{
		sessions: map[string]*session{},
		byDoc:    map[string]string{},
		newApp:   newApp,
	}
	s.sessions[defaultSession] = &session{
		app: first, token: defaultSession, used: time.Now(),
	}
	if first != nil {
		if key := documentKey(first.filename); key != "" {
			s.sessions[defaultSession].doc = key
			s.byDoc[key] = defaultSession
		}
	}
	return s
}

// ServeHTTP hands the request to the session it names.
//
// The whole of the session mechanism is this function. Each App built its own
// router when it was made, so the work here is choosing which router, and
// everything behind it is the program as it was.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The server's own routes first: making a session cannot be the job of
	// a session, and closing one cannot be done from inside it.
	switch r.URL.Path {
	case "/api/session":
		s.handleSession(w, r)
		return
	case "/api/sessions":
		s.handleSessions(w, r)
		return
	}

	token := r.URL.Query().Get(sessionParam)
	if token == "" {
		token = defaultSession
	}
	app, ok := s.use(token)
	if !ok {
		// Said plainly rather than quietly making one. A client holding a
		// session that has gone has a bug or has been away too long, and
		// handing it a new empty score to write into would look like
		// working right up until somebody asked where their music was.
		http.Error(w, "no session "+token+
			": it was closed, or the server was restarted",
			http.StatusNotFound)
		return
	}
	app.mux.ServeHTTP(w, r)
}

// use finds a session and marks it as still in use.
func (s *Server) use(token string) (*App, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	got, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	got.used = time.Now()
	return got.app, true
}

// handleSession makes a session, or closes one.
//
//	POST /api/session                 a new empty score
//	POST /api/session?path=<file>     that file, opened — or the session it
//	                                  is already open in
//	POST /api/session?close=<token>   done with it
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	if token := q.Get("close"); token != "" {
		if token == defaultSession {
			http.Error(w, "the default session cannot be closed",
				http.StatusBadRequest)
			return
		}
		unsaved, ok := s.close(token)
		if !ok {
			http.Error(w, "no session "+token, http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"status": "ok", "unsaved": unsaved})
		return
	}

	path := q.Get("path")

	// Already open? Then this is the session for it. Opening a file twice
	// and getting two copies of it is how people lose work: they edit one,
	// save the other, and the program was right both times.
	if path != "" {
		if key := documentKey(path); key != "" {
			s.mu.RLock()
			token, open := s.byDoc[key]
			s.mu.RUnlock()
			if open {
				if _, ok := s.use(token); ok {
					writeJSON(w, map[string]any{
						"status": "ok", "session": token, "reattached": true,
					})
					return
				}
			}
		}
	}

	app, token, err := s.open(path)
	if err != nil {
		http.Error(w, err.Error(), statusFor(err))
		return
	}
	writeJSON(w, map[string]any{
		"status": "ok", "session": token, "reattached": false,
		"filename": app.filename,
	})
}

// open makes a session, reading a file into it where one is named.
func (s *Server) open(path string) (*App, string, error) {
	if s.newApp == nil {
		return nil, "", fmt.Errorf("this server cannot open another score")
	}
	app := s.newApp()
	if path != "" {
		if err := app.openPath(path); err != nil {
			return nil, "", err
		}
	}

	token, err := newToken()
	if err != nil {
		return nil, "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Room made before the session is added, so a server at its limit says
	// so rather than going one over.
	if s.most > 0 && len(s.sessions) >= s.most {
		if !s.evictOneLocked() {
			return nil, "", fmt.Errorf(
				"there are already %d scores open, and every one of them has "+
					"unsaved changes", len(s.sessions))
		}
	}
	got := &session{app: app, token: token, used: time.Now()}
	if key := documentKey(app.filename); key != "" {
		got.doc = key
		s.byDoc[key] = token
	}
	s.sessions[token] = got
	return app, token, nil
}

// close forgets a session, and says whether it had unsaved work in it.
//
// It does not refuse. Closing is somebody saying they are done, and a
// program that will not let go of a document is worse than one that lets
// go of an edit — but it says what it dropped, so the interface can ask
// first and a script can notice.
func (s *Server) close(token string) (unsaved bool, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	got, ok := s.sessions[token]
	if !ok {
		return false, false
	}
	unsaved = got.app.hasUnsaved()
	delete(s.sessions, token)
	if got.doc != "" && s.byDoc[got.doc] == token {
		delete(s.byDoc, got.doc)
	}
	return unsaved, true
}

// handleSessions lists what is open.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]map[string]any, 0, len(s.sessions))
	for token, got := range s.sessions {
		out = append(out, map[string]any{
			"session":  token,
			"filename": got.app.filename,
			"title":    got.app.score.Identity.Title,
			"unsaved":  got.app.hasUnsaved(),
			"default":  token == defaultSession,
		})
	}
	// A stable order, so that a script reading this twice sees the same
	// thing twice: a map's own order is deliberately not one.
	sort.Slice(out, func(i, j int) bool {
		return out[i]["session"].(string) < out[j]["session"].(string)
	})
	writeJSON(w, map[string]any{"sessions": out})
}

// Sweep drops sessions nobody has touched for a while.
//
// Called on a timer by whoever runs the server. Memory is the reason: a
// session holds a score and the layout engraved from it, and a long-running
// server that opened a thousand scores would still be holding all thousand.
//
// It will not drop unsaved work. A document somebody has edited and not
// saved is the one thing here that cannot be reconstructed, and dropping it
// because they went to lunch would be the worst bug in the program. Those
// are kept however long they idle; the limit on how many sessions there may
// be is what stops that growing without end, and it refuses rather than
// discards.
func (s *Server) Sweep(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idle <= 0 {
		return 0
	}
	dropped := 0
	for token, got := range s.sessions {
		if token == defaultSession {
			continue // the document the server was started on stays
		}
		if got.app.hasUnsaved() {
			continue
		}
		if now.Sub(got.used) < s.idle {
			continue
		}
		delete(s.sessions, token)
		if got.doc != "" && s.byDoc[got.doc] == token {
			delete(s.byDoc, got.doc)
		}
		dropped++
	}
	return dropped
}

// evictOneLocked drops the least recently used saved session, so that one
// more may be opened. Called with the lock held.
func (s *Server) evictOneLocked() bool {
	oldest, at := "", time.Time{}
	for token, got := range s.sessions {
		if token == defaultSession || got.app.hasUnsaved() {
			continue
		}
		if oldest == "" || got.used.Before(at) {
			oldest, at = token, got.used
		}
	}
	if oldest == "" {
		return false
	}
	got := s.sessions[oldest]
	delete(s.sessions, oldest)
	if got.doc != "" && s.byDoc[got.doc] == oldest {
		delete(s.byDoc, got.doc)
	}
	return true
}

// SetLimits says how long an idle session is kept and how many there may be.
//
// A desktop gscore wants neither: the person has as many scores open as they
// opened and closes them when they are done. A server with -serve wants both.
func (s *Server) SetLimits(idle time.Duration, most int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idle, s.most = idle, most
}

// Sessions is how many documents are open, for a test or a status line.
func (s *Server) Sessions() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// Default is the session a request naming none talks to.
func (s *Server) Default() *App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if got, ok := s.sessions[defaultSession]; ok {
		return got.app
	}
	return nil
}

// hasUnsaved says whether this document has edits that are not on disk.
func (a *App) hasUnsaved() bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.unsaved
}

// newToken makes a session credential: 128 random bits, as hex.
//
// Random and not derived from anything. Whoever holds this can read and
// overwrite the score it names, so it is a password, and a password worked
// out from the file's name is one anybody who can guess the file's name
// already has.
func newToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("could not make a session: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// documentKey is the UUIDv5 of a file's absolute path.
//
// Deterministic, which is the whole point: the same file is the same key
// today and after a restart, so opening a score twice finds the session it
// is already open in. Empty for a score with no file, which has nothing to
// be the same as.
//
// Built here rather than taken from a library: a v5 UUID is a SHA-1 of the
// namespace and the name with six bits set, and that is less code than the
// dependency would be.
func documentKey(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	h := sha1.New()
	h.Write(sessionNamespace[:])
	h.Write([]byte(abs))
	var u [16]byte
	copy(u[:], h.Sum(nil))
	u[6] = (u[6] & 0x0f) | 0x50 // version 5
	u[8] = (u[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// newScoreApp is what a server makes when somebody asks for another
// document: an App exactly like the one gscore started with, but empty.
func (a *App) newScoreApp() func() *App {
	return func() *App {
		next := New(model.NewScore(), "")
		next.local, next.native, next.inBrowser = a.local, a.native, a.inBrowser
		next.server = a.server
		return next
	}
}
