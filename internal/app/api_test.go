/*
 * api_test.go
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
	"regexp"
	"sort"
	"strings"
	"testing"

	// What main.go registers, so the formats checked here are the ones a
	// built gscore actually offers and not the two this package imports on
	// its own account.
	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
	_ "github.com/stricaud/gscore/internal/format/lilypond"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
	_ "github.com/stricaud/gscore/internal/format/nwc"
)

// The published API and the routes that answer it have to be the same list.
//
// docs/openapi.yaml is what somebody driving gscore from a script reads, and
// a document that falls behind the program is worse than none: it sends them
// to a route that does not exist, or hides one that does. Neither list is
// written by hand twice — this reads both and compares them, so adding a
// route without documenting it is a failing test rather than a quiet lie.
//
// Read as text, which is what ui_wiring_test.go does to app.js for the same
// reason: the route table is a literal in app.go and the paths are keys in a
// YAML file, and both are plainer to read than to parse.
func TestEveryRouteIsInTheOpenAPISpec(t *testing.T) {
	routes := routesInSource(t)
	documented := pathsInSpec(t)

	for _, path := range routes {
		if !documented[path] {
			t.Errorf("%s is served but not in docs/openapi.yaml", path)
		}
	}
	for path := range documented {
		if !contains(routes, path) {
			t.Errorf("docs/openapi.yaml describes %s, which nothing serves", path)
		}
	}
}

// routesInSource reads the route table out of app.go.
//
// Two shapes: the plain registrations, and the table of editing routes that
// are wrapped in undo. The static file server at / and the two pictures
// beside it are not API.
func routesInSource(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("reading app.go: %v", err)
	}
	s := string(src)

	var out []string
	plain := regexp.MustCompile(`mux\.HandleFunc\("(/[^"]*)", a\.(handle\w+)\)`)
	for _, m := range plain.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	undoable := regexp.MustCompile(`"(/api/[^"]+)":\s*a\.handle\w+,`)
	for _, m := range undoable.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	if len(out) < 50 {
		t.Fatalf("only found %d routes in app.go, so this is reading it wrong", len(out))
	}

	// The pages gscore serves rather than the API it answers: the editor
	// itself, and the pictures in its About box.
	notAPI := map[string]bool{"/gilbert.png": true, "/gilbert.svg": true}
	var keep []string
	for _, p := range out {
		if !notAPI[p] {
			keep = append(keep, p)
		}
	}
	sort.Strings(keep)
	return keep
}

// pathsInSpec reads the keys under paths: out of the YAML.
//
// One level of indentation under paths:, which is every path and nothing
// else — the components below it are at the top level again.
func pathsInSpec(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("reading the spec: %v", err)
	}
	out := map[string]bool{}
	inPaths := false
	for _, line := range strings.Split(string(src), "\n") {
		if line == "paths:" {
			inPaths = true
			continue
		}
		if inPaths && line != "" && !strings.HasPrefix(line, " ") {
			break // back to the top level: components, and the spec is over
		}
		if !inPaths {
			continue
		}
		if m := regexp.MustCompile(`^  (/\S*):\s*$`).FindStringSubmatch(line); m != nil {
			out[m[1]] = true
		}
	}
	if len(out) < 50 {
		t.Fatalf("only found %d paths in the spec, so this is reading it wrong", len(out))
	}
	return out
}

// Every documented operation has to be a method the route actually answers.
//
// A handler that insists on POST, documented as a get, is a worked example
// that answers 405 — which is the kind of mistake a document makes and a
// program cannot.
func TestTheSpecAgreesAboutMethods(t *testing.T) {
	src, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("reading the spec: %v", err)
	}

	// Which handler answers each path, and whether that handler turns away
	// anything but a POST.
	app, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("reading app.go: %v", err)
	}
	handler := map[string]string{}
	for _, m := range regexp.MustCompile(`mux\.HandleFunc\("(/[^"]*)", a\.(handle\w+)\)`).
		FindAllStringSubmatch(string(app), -1) {
		handler[m[1]] = m[2]
	}
	for _, m := range regexp.MustCompile(`"(/api/[^"]+)":\s*a\.(handle\w+),`).
		FindAllStringSubmatch(string(app), -1) {
		handler[m[1]] = m[2]
	}

	// Every file in the package, not a list of them: a handler in a file
	// nobody remembered to name would be skipped, and a check that silently
	// skips is a check that passes.
	sources := map[string]string{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if b, err := os.ReadFile(name); err == nil {
			sources[name] = string(b)
		}
	}
	postOnly := func(h string) bool {
		for _, s := range sources {
			i := strings.Index(s, "func (a *App) "+h+"(")
			if i < 0 {
				continue
			}
			end := strings.Index(s[i:], "\n}\n")
			if end < 0 {
				end = len(s) - i
			}
			return strings.Contains(s[i:i+end], "r.Method != http.MethodPost")
		}
		return false
	}

	path := ""
	for _, line := range strings.Split(string(src), "\n") {
		if m := regexp.MustCompile(`^  (/\S*):\s*$`).FindStringSubmatch(line); m != nil {
			path = m[1]
			continue
		}
		if path == "" {
			continue
		}
		if m := regexp.MustCompile(`^    (get|post|put|delete):\s*$`).FindStringSubmatch(line); m != nil {
			h, ok := handler[path]
			if !ok {
				continue // the other test reports this
			}
			if m[1] == "get" && postOnly(h) {
				t.Errorf("the spec documents GET %s, but %s answers only POST", path, h)
			}
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// The worked example in the spec's own description has to work.
//
// Four calls from nothing to a PDF, which is what somebody reading the
// document will type first. A spec whose opening example is wrong teaches
// everybody who reads it the wrong thing.
func TestTheSpecsWorkedExampleRuns(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=quarter&kind=note")
	h.post("/api/beam?beam=join&sel=0:0:0,0:0:1")

	// A new score comes with one staff, which is what the example relies on
	// and what makes /api/addstaff part=1 rather than part=0.
	if n := len(h.app.score.Parts); n != 1 {
		t.Fatalf("a new score has %d staves, and the spec says one", n)
	}

	pdf := h.get("/api/pdf")
	if !strings.HasPrefix(pdf, "%PDF-") {
		t.Fatalf("GET /api/pdf gave %d bytes that are not a PDF", len(pdf))
	}

	// And every other way out named in the spec. These are the extensions a
	// built gscore carries, which is why this file imports what main.go
	// imports: the registry is filled by those imports and by nothing else.
	for _, as := range []string{"abc", "gscore", "ly", "musicxml", "xml", "mxl"} {
		if out := h.get("/api/convert?as=" + as); out == "" {
			t.Errorf("the spec offers %s but GET /api/convert?as=%s gave nothing", as, as)
		}
	}

	// info.formats is the other list — what can be opened, by name — and the
	// spec says so because saying the other thing is the obvious mistake.
	var info struct {
		Formats []string `json:"formats"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/info")), &info); err != nil {
		t.Fatalf("info: %v", err)
	}
	if !contains(info.Formats, "MuseScore") {
		t.Errorf("info.formats is %v, which does not look like the list of readers", info.Formats)
	}
	if midi := h.get("/api/midi"); !strings.HasPrefix(midi, "MThd") {
		t.Error("GET /api/midi is not a MIDI file")
	}
	if page := h.get("/api/page"); !strings.Contains(page, "<svg") {
		t.Error("GET /api/page has no engraving in it")
	}
	if svg := h.get("/score.svg"); !strings.Contains(svg, "<svg") {
		t.Error("GET /score.svg is not SVG")
	}
}

// The spec the server hands out is the spec in the repository.
//
// Embedded rather than copied, so this can only fail if somebody introduces
// a second copy — which is the whole reason the embed lives in docs/ beside
// the file instead of in a folder of its own.
func TestTheServerServesTheSpec(t *testing.T) {
	h := newHarness(t)
	served := h.get("/openapi.yaml")

	onDisk, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("reading the spec: %v", err)
	}
	if served != string(onDisk) {
		t.Errorf("/openapi.yaml serves %d bytes, the file is %d", len(served), len(onDisk))
	}

	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/yaml") {
		t.Errorf("it is served as %q, and a spec is application/yaml", got)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("it cannot be fetched from anywhere else, which is where it is read from")
	}
}
