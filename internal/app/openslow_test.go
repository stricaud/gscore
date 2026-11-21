/*
 * openslow_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Opening pictures of pages from the machine's own files: a scanned PDF
 * that goes to the page reader, several pictures picked together, the
 * reply that is kept alive while the reader works, and the lock decided
 * on Save as.
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
	"time"

	"github.com/stricaud/gscore/internal/format/engraved"
	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/format/scanned"
)

// blankPDF is a PDF of one empty page: nothing engraved in it, which is
// what a scan looks like to the glyph reader.
const blankPDF = "%PDF-1.4\n" +
	"1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n" +
	"2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n" +
	"3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R >> endobj\n" +
	"4 0 obj << /Length 0 >> stream\n\nendstream endobj\n" +
	"trailer << /Root 1 0 R >>\n%%EOF\n"

func TestAPDFGoesToThePageReaderFirst(t *testing.T) {
	h := newHarness(t)
	// Without a reader, a scan is refused and the refusal says what to
	// install; an engraved PDF is still read glyph by glyph.
	was := scanned.Find
	scanned.Find = func() (scanned.Engine, error) { return scanned.Engine{}, errNoScanEngine }
	msg := h.postBodyFails("/api/open?name=scan.pdf", blankPDF)
	if !strings.Contains(msg, "scan") || !strings.Contains(msg, "page reader") {
		t.Errorf("a scan with no reader was answered %q", msg)
	}
	engravedPDF, err := os.ReadFile("../format/engraved/testdata/" + engravedFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	d := h.postBody("/api/open?name=engraved.pdf", string(engravedPDF))
	if d["status"] != "ok" || d["engraved"] != true {
		t.Errorf("an engraved PDF with no reader was answered %v, want read glyph by glyph", d)
	}
	scanned.Find = was

	// With a reader, every PDF goes to it, engraved or not.
	fakePageReader(t)
	d = h.postBody("/api/open?name=engraved.pdf", string(engravedPDF))
	if d["status"] != "ok" || d["engraved"] != false {
		t.Errorf("an engraved PDF with a reader was answered %v, want read as pictures", d)
	}
	d = h.postBody("/api/open?name=scan.pdf", blankPDF)
	if d["status"] != "ok" || d["engraved"] != false {
		t.Errorf("the scan was answered %v, want ok and not engraved", d)
	}
	if got := pitches(h); !strings.Contains(got, "E5") {
		t.Errorf("the score read is %q", got)
	}
	// Nothing kept to review: the glyph reader read nothing.
	if h.app.sourceName != "" {
		t.Errorf("a scan left %q as the PDF to review", h.app.sourceName)
	}
}

func TestSeveralPicturesPickedTogetherAreOneScore(t *testing.T) {
	asked := fakePageReader(t)
	h := newHarness(t)
	h.app.local = true
	dir := t.TempDir()
	one, two := filepath.Join(dir, "p1.jpg"), filepath.Join(dir, "p2.png")
	os.WriteFile(one, pageJPEG, 0o600)
	os.WriteFile(two, pagePNG, 0o600)
	wasPick := pickFiles
	pickFiles = func() ([]string, error) { return []string{one, two}, nil }
	defer func() { pickFiles = wasPick }()

	// Picked together, they go back to be put in order, and come in
	// again by path in the order chosen.
	d := h.post("/api/pickfile")
	pages, _ := d["pages"].([]any)
	if d["status"] != "pages" || len(pages) != 2 {
		t.Fatalf("two pictures picked were answered %v", d)
	}
	d = h.post("/api/openscan?path=" + two + "&path=" + one)
	if d["status"] != "ok" || d["pages"] != float64(2) || d["filename"] != "p2.gscore" {
		t.Errorf("the pages in the order chosen were answered %v", d)
	}
	if said, _ := os.ReadFile(asked); !strings.Contains(string(said), "page-01.png page-02.jpg") {
		t.Errorf("the reader was given %q", said)
	}
	// Import ▸ Images asks for the list even of one.
	pickFiles = func() ([]string, error) { return []string{one}, nil }
	if d := h.post("/api/pickfile?list=1"); d["status"] != "pages" {
		t.Errorf("one picture listed was answered %v", d)
	}

	// A score and a picture together is not a thing to open.
	pickFiles = func() ([]string, error) { return []string{one, filepath.Join(dir, "x.abc")}, nil }
	if msg := h.postFails("/api/pickfile"); !strings.Contains(msg, "pictures of pages") {
		t.Errorf("a mixed pick was answered %q", msg)
	}
	// And reading by path is for this machine.
	h.app.local = false
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/openscan?path="+one, nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("a remote read by path was answered %d", rec.Code)
	}
	h.app.local = true
	// One picture goes the ordinary way, through the same reader.
	pickFiles = func() ([]string, error) { return []string{two}, nil }
	if d := h.post("/api/pickfile"); d["status"] != "ok" || d["filename"] != "p2.png" {
		t.Errorf("one picture picked was answered %v", d)
	}
}

// A reply that is not ready soon is opened and kept alive, and the answer
// — or the error — comes behind the spaces.
func TestASlowAnswerIsKeptAlive(t *testing.T) {
	wasAfter, wasTick := slowAfter, scanKeepalive
	slowAfter = 10 * time.Millisecond
	defer func() { slowAfter = wasAfter; _ = wasTick }()

	rec := httptest.NewRecorder()
	slowly(rec, func() (any, error) {
		time.Sleep(30 * time.Millisecond)
		return map[string]int{"pages": 2}, nil
	}, badRequest)
	var d map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil || d["pages"] != float64(2) {
		t.Errorf("a slow success came back as %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	slowly(rec, func() (any, error) {
		time.Sleep(30 * time.Millisecond)
		return nil, errors.New("the reader fell over")
	}, badRequest)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"error":"the reader fell over"`) {
		t.Errorf("a slow failure came back as %d %q", rec.Code, rec.Body.String())
	}

	// A quick failure is still a status of its own.
	rec = httptest.NewRecorder()
	slowly(rec, func() (any, error) { return nil, errors.New("no") }, badRequest)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a quick failure came back as %d", rec.Code)
	}
}

func TestTheLockIsDecidedOnSaveAs(t *testing.T) {
	h := newHarness(t)
	h.app.local = true
	h.post("/api/new")
	h.bars(1)
	path := filepath.Join(t.TempDir(), "piece.gscore")

	save := func(query, password string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/save?path="+path+query, nil)
		if password != "" {
			req.Header.Set(passwordHeader, password)
		}
		rec := httptest.NewRecorder()
		h.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("save %q: %d %s", query, rec.Code, rec.Body.String())
		}
	}
	locked := func() bool {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = gscorefmt.Read(bytes.NewReader(data))
		return errors.Is(err, gscorefmt.ErrPasswordNeeded)
	}

	save("&lock=1", "hunter2")
	if !locked() || h.app.password != "hunter2" {
		t.Error("lock=1 with a password did not lock the file")
	}
	// A plain save keeps it as it is.
	save("", "")
	if !locked() {
		t.Error("a plain save after locking saved the file open")
	}
	// lock=1 alone keeps the password it has.
	save("&lock=1", "")
	if !locked() || h.app.password != "hunter2" {
		t.Error("lock=1 without a password dropped the one it had")
	}
	save("&lock=0", "")
	if locked() || h.app.password != "" {
		t.Error("lock=0 did not take the password off")
	}
}

// engravedFixture is a PDF the glyph reader reads, from the engraved
// importer's own tests.
func engravedFixture(t *testing.T) string {
	t.Helper()
	names, _ := filepath.Glob("../format/engraved/testdata/*.pdf")
	if len(names) == 0 {
		t.Skip("no engraved PDF to test with")
	}
	return filepath.Base(names[0])
}

// A PDF that could be read either way is asked about when the request
// says ask=1, and read the way reader= says.
func TestAPDFWithGlyphsIsAskedAbout(t *testing.T) {
	fakePageReader(t)
	h := newHarness(t)
	engravedPDF, err := os.ReadFile("../format/engraved/testdata/" + engravedFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	d := h.postBody("/api/open?name=engraved.pdf&ask=1", string(engravedPDF))
	staves, _ := d["staves"].(float64)
	if d["status"] != "choose" || staves < 1 {
		t.Fatalf("asked about an engraved PDF: %v", d)
	}
	d = h.postBody("/api/open?name=engraved.pdf&reader=glyphs", string(engravedPDF))
	if d["status"] != "ok" || d["engraved"] != true {
		t.Errorf("reader=glyphs: %v", d)
	}
	d = h.postBody("/api/open?name=engraved.pdf&reader=pages", string(engravedPDF))
	if d["status"] != "ok" || d["engraved"] != false {
		t.Errorf("reader=pages: %v", d)
	}
	// A scan has nothing to ask about: the page reader it is.
	d = h.postBody("/api/open?name=scan.pdf&ask=1", blankPDF)
	if d["status"] != "ok" || d["engraved"] != false {
		t.Errorf("a scan asked about: %v", d)
	}
	// And by path, from the dialog or the recent list, the same.
	h.app.local = true
	path := filepath.Join(t.TempDir(), "engraved.pdf")
	os.WriteFile(path, engravedPDF, 0o600)
	wasPick := pickFiles
	pickFiles = func() ([]string, error) { return []string{path}, nil }
	defer func() { pickFiles = wasPick }()
	if d := h.post("/api/pickfile?ask=1"); d["status"] != "choose" || d["filename"] != path {
		t.Errorf("picked with ask=1: %v", d)
	}
	if d := h.post("/api/openpath?path=" + path + "&reader=glyphs"); d["engraved"] != true {
		t.Errorf("by path with reader=glyphs: %v", d)
	}
}

// Where the page reader is, for the page to show while it waits.
func TestTheReadersProgressIsServed(t *testing.T) {
	h := newHarness(t)
	var d map[string]any
	json.Unmarshal([]byte(h.get("/api/scanprogress")), &d)
	if d["active"] != false {
		t.Errorf("idle, the progress says %v", d)
	}
	h.app.scan.begin()
	h.app.scan.report(scanned.Progress{Page: 2, Pages: 4, Staff: 3, Staves: 6, Percent: 43})
	json.Unmarshal([]byte(h.get("/api/scanprogress")), &d)
	if d["active"] != true || d["percent"] != float64(43) || d["what"] != "page 2 of 4, staff 3 of 6" {
		t.Errorf("at work, the progress says %v", d)
	}
	h.app.scan.end()
	json.Unmarshal([]byte(h.get("/api/scanprogress")), &d)
	if d["active"] != false {
		t.Errorf("done, the progress says %v", d)
	}
}

// The page reader keeps a staff or two of a system and drops the rest.
// Where the PDF's glyphs say how many staves there were, the answer warns,
// and says how to keep them all.
func TestPartsDroppedByThePageReaderAreWarnedAbout(t *testing.T) {
	// A PDF of a score of three staves, from gscore's own engraver.
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/addstaff")
	h.post("/api/addstaff")
	for p := 0; p < 3; p++ {
		h.note(p, 0, 2, "quarter")
	}
	// Bracketed with the barlines joined, which is what tells the glyph
	// reader the three staves are one system rather than three in a row.
	h.post("/api/group?first=0&last=2&symbol=bracket&barlines=1")
	pdf := h.get("/api/pdf")
	if !strings.HasPrefix(pdf, "%PDF") {
		t.Fatalf("no PDF to test with: %q", pdf[:min(20, len(pdf))])
	}

	if g, err := engraved.ImportWith(strings.NewReader(pdf), engraved.Options{}); err != nil || len(g.Parts) != 3 {
		t.Fatalf("the glyph reader does not see three staves in gscore's own PDF: %v %v", g, err)
	}
	// The stand-in reader answers with one part.
	fakePageReader(t)
	h2 := newHarness(t)
	d := h2.postBody("/api/open?name=trio.pdf&reader=pages", pdf)
	warning, _ := d["warning"].(string)
	if d["status"] != "ok" || !strings.Contains(warning, "1 part from systems of 3 staves") ||
		!strings.Contains(warning, "glyphs") {
		t.Errorf("the page reader's reading of a trio was answered %v", d)
	}
	// Taken once: the next open does not repeat it.
	d = h2.postBody("/api/open?name=trio.pdf&reader=glyphs", pdf)
	if _, again := d["warning"]; again || d["engraved"] != true {
		t.Errorf("the glyph reading was answered %v", d)
	}
	// And by path, the same.
	h2.app.local = true
	path := filepath.Join(t.TempDir(), "trio.pdf")
	os.WriteFile(path, []byte(pdf), 0o600)
	d = h2.post("/api/openpath?path=" + path + "&reader=pages")
	if w, _ := d["warning"].(string); !strings.Contains(w, "3 staves") {
		t.Errorf("by path, the warning is %v", d)
	}
}
