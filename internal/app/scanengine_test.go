/*
 * scanengine_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading scanned pages: which program is run, chosen in Settings or
 * found on the machine, and the request that hands it pages, from files
 * or from the web.
 */

package app

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format/scanned"
)

func TestACommandLineIsCutIntoWords(t *testing.T) {
	cases := map[string][]string{
		"uvx --from 'homr[cpu]' homr":           {"uvx", "--from", "homr[cpu]", "homr"},
		`"/Applications/My Tools/homr" --gpu x`: {"/Applications/My Tools/homr", "--gpu", "x"},
		"  homr  ":                              {"homr"},
		"":                                      nil,
	}
	for in, want := range cases {
		got := splitCommand(in)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%q split as %q, want %q", in, got, want)
		}
	}
}

// fakeTool puts an executable of that name in a folder and returns the folder.
func fakeTool(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-ins are shell scripts")
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// cleanScanSettings leaves the page reader settings as they were found,
// since the settings file is shared by every test in the process.
func cleanScanSettings(t *testing.T, h *harness) {
	t.Helper()
	h.post("/api/settings?scanReader=cpu&scanCommand=")
	t.Cleanup(func() { h.post("/api/settings?scanReader=cpu&scanCommand=") })
}

func TestThePageReaderIsFoundInOrder(t *testing.T) {
	h := newHarness(t)
	cleanScanSettings(t, h)
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	t.Setenv(scanEngineEnv, "")

	// Nothing anywhere.
	if _, err := scanEngine(); err == nil || !strings.Contains(err.Error(), "uv") {
		t.Errorf("with nothing installed: %v", err)
	}

	// homr on the PATH.
	tools := fakeTool(t, t.TempDir(), "homr")
	t.Setenv("PATH", tools)
	eng, err := scanEngine()
	if err != nil || eng.Source != "the PATH" || filepath.Base(eng.Command[0]) != "homr" {
		t.Errorf("with homr on the PATH: %v %v", eng, err)
	}

	// uv beats it, and fetches the processor build unless told otherwise.
	fakeTool(t, tools, "uvx")
	eng, _ = scanEngine()
	if eng.Source != "uv on this machine" || strings.Join(eng.Command[1:], " ") != "--from homr[cpu] homr" {
		t.Errorf("with uv: %v", eng)
	}
	h.post("/api/settings?scanReader=cuda")
	if eng, _ = scanEngine(); !strings.Contains(eng.String(), "homr[cuda]") {
		t.Errorf("asked for the NVIDIA build: %v", eng)
	}

	// A command typed in Settings beats what is found.
	h.post("/api/settings?scanReader=custom&scanCommand=" +
		"poetry+run+homr+--coreml-encoder")
	eng, _ = scanEngine()
	if eng.Source != "the command in Settings" || eng.String() != "poetry run homr --coreml-encoder" {
		t.Errorf("with a command in Settings: %v", eng)
	}
	h.post("/api/settings?scanReader=custom&scanCommand=")
	if _, err := scanEngine(); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("an empty custom command: %v", err)
	}

	// And the environment beats everything.
	t.Setenv(scanEngineEnv, "uvx --from 'homr[cuda]' homr")
	eng, _ = scanEngine()
	if eng.Source != "the "+scanEngineEnv+" variable" || len(eng.Command) != 4 || eng.Command[2] != "homr[cuda]" {
		t.Errorf("from the environment: %v", eng)
	}
}

// A homr shipped beside the binary is found by the folder it is laid out
// in: for this architecture first, then for any.
func TestAReaderBesideTheBinaryIsFound(t *testing.T) {
	dir := t.TempDir()
	if got := scanEngineIn(dir, "arm64"); got != "" {
		t.Errorf("an empty folder yields %q", got)
	}
	os.MkdirAll(filepath.Join(dir, "homr", "bin"), 0o755)
	os.WriteFile(filepath.Join(dir, "homr", "bin", "homr"), []byte("#!/bin/sh\n"), 0o755)
	if got := scanEngineIn(dir, "arm64"); got != filepath.Join(dir, "homr", "bin", "homr") {
		t.Errorf("the plain folder yields %q", got)
	}
	os.MkdirAll(filepath.Join(dir, "homr-arm64", "bin"), 0o755)
	os.WriteFile(filepath.Join(dir, "homr-arm64", "bin", "homr"), []byte("#!/bin/sh\n"), 0o755)
	if got := scanEngineIn(dir, "arm64"); got != filepath.Join(dir, "homr-arm64", "bin", "homr") {
		t.Errorf("with a folder for the architecture: %q", got)
	}
	if got := scanEngineIn(dir, "amd64"); got != filepath.Join(dir, "homr", "bin", "homr") {
		t.Errorf("another architecture falls back to %q", got)
	}
	// Windows lays it out with a .cmd at the top; a macOS bundle keeps it
	// in Resources beside MacOS.
	win := t.TempDir()
	os.MkdirAll(filepath.Join(win, "homr"), 0o755)
	os.WriteFile(filepath.Join(win, "homr", "homr.cmd"), []byte("@echo off\r\n"), 0o755)
	if got := scanEngineIn(win, "amd64"); got != filepath.Join(win, "homr", "homr.cmd") {
		t.Errorf("the Windows layout yields %q", got)
	}
	app := t.TempDir()
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	os.MkdirAll(filepath.Join(app, "Contents", "Resources", "homr-arm64", "bin"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "Resources", "homr-arm64", "bin", "homr"), []byte("#!/bin/sh\n"), 0o755)
	if got := scanEngineIn(filepath.Join(app, "Contents", "MacOS"), "arm64"); !strings.HasSuffix(got, "Resources/homr-arm64/bin/homr") {
		t.Errorf("the bundle layout yields %q", got)
	}
}

func TestSettingsRememberThePageReader(t *testing.T) {
	h := newHarness(t)
	cleanScanSettings(t, h)
	t.Setenv(scanEngineEnv, "")
	var got map[string]any
	json.Unmarshal([]byte(h.get("/api/settings")), &got)
	if got["scanReader"] != "cpu" {
		t.Errorf("the reader starts as %v, want cpu", got["scanReader"])
	}
	if _, ok := got["scanEngine"].(map[string]any); !ok {
		t.Errorf("no word of which engine is found: %v", got["scanEngine"])
	}
	h.post("/api/settings?scanReader=custom&scanCommand=homr+--debug")
	json.Unmarshal([]byte(h.get("/api/settings")), &got)
	if got["scanReader"] != "custom" || got["scanCommand"] != "homr --debug" {
		t.Errorf("after setting a command: %v %v", got["scanReader"], got["scanCommand"])
	}
	eng := got["scanEngine"].(map[string]any)
	if eng["command"] != "homr --debug" || eng["source"] != "the command in Settings" {
		t.Errorf("the engine reported is %v", eng)
	}
	if msg := h.postFails("/api/settings?scanReader=abacus"); !strings.Contains(msg, "cpu, cuda") {
		t.Errorf("a reader that is not one was answered %q", msg)
	}
	// Back to the processor is stored as nothing chosen.
	h.post("/api/settings?scanReader=cpu")
	if s := loadSettings(); s.ScanReader != "" {
		t.Errorf("cpu is stored as %q", s.ScanReader)
	}
}

// A page of one bar, as the stand-in reader writes it.
const scannedPage = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0">
  <part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>
  <part id="P1"><measure number="1">
    <attributes><divisions>1</divisions><clef><sign>G</sign><line>2</line></clef></attributes>
    <note><pitch><step>E</step><octave>5</octave></pitch><duration>4</duration><type>whole</type></note>
  </measure></part>
</score-partwise>
`

var pagePNG = []byte("\x89PNG\r\n\x1a\npage")
var pageJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 'p', 'a', 'g', 'e'}

// fakePageReader stands in for homr for the length of the test: a script
// that writes the page above beside the first picture and notes what it
// was given.
func fakePageReader(t *testing.T) (asked string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in reader is a shell script")
	}
	dir := t.TempDir()
	xml := filepath.Join(dir, "page.musicxml")
	asked = filepath.Join(dir, "asked.txt")
	os.WriteFile(xml, []byte(scannedPage), 0o600)
	script := filepath.Join(dir, "homr")
	os.WriteFile(script, []byte("#!/bin/sh\n"+
		"echo \"$*\" > '"+asked+"'\n"+
		"for a in \"$@\"; do case \"$a\" in -*) ;; *) first=\"$a\"; break;; esac; done\n"+
		"cp '"+xml+"' \"${first%.*}.musicxml\"\n"), 0o700)
	was := scanned.Find
	scanned.Find = func() (scanned.Engine, error) {
		return scanned.Engine{Command: []string{script}, Source: "the test"}, nil
	}
	t.Cleanup(func() { scanned.Find = was })
	return asked
}

func TestPagesSentAreReadAsOneScore(t *testing.T) {
	asked := fakePageReader(t)
	h := newHarness(t)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range []struct {
		name string
		data []byte
	}{{"spell1.jpg", pageJPEG}, {"spell2.png", pagePNG}} {
		fw, _ := mw.CreateFormFile("page", f.name)
		fw.Write(f.data)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/openscan", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var d map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("the answer %q: %v", rec.Body.String(), err)
	}
	if d["status"] != "ok" || d["pages"] != float64(2) || d["filename"] != "spell1.gscore" {
		t.Errorf("the answer was %v", d)
	}
	if got := pitches(h); !strings.Contains(got, "E5") {
		t.Errorf("the score read is %q", got)
	}
	if said, _ := os.ReadFile(asked); !strings.Contains(string(said), "page-01.jpg page-02.png") {
		t.Errorf("the reader was given %q", said)
	}
}

func TestPagesFromTheWebAreFetchedInOrder(t *testing.T) {
	asked := fakePageReader(t)
	h := newHarness(t)
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sheet1.jpg":
			w.Write(pageJPEG)
		case "/sheet2.png":
			w.Write(pagePNG)
		default:
			http.NotFound(w, r)
		}
	}))
	defer web.Close()
	q := "/api/openscan?url=" + web.URL + "/sheet1.jpg&url=" + web.URL + "/sheet2.png"

	// Not for a server reached from elsewhere.
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, q, nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("a remote fetch was answered %d", rec.Code)
	}

	h.app.local = true
	d := h.post(q)
	if d["status"] != "ok" || d["pages"] != float64(2) || d["filename"] != "sheet1.gscore" {
		t.Errorf("the answer was %v", d)
	}
	if said, _ := os.ReadFile(asked); !strings.Contains(string(said), "page-01.jpg page-02.png") {
		t.Errorf("the reader was given %q", said)
	}
	// A page that is not there is said so, before anything is read.
	if msg := h.postFails("/api/openscan?url=" + web.URL + "/missing.jpg"); !strings.Contains(msg, "404") {
		t.Errorf("a missing page was answered %q", msg)
	}
	if msg := h.postFails("/api/openscan?url=ftp://x/y.jpg"); !strings.Contains(msg, "not a web address") {
		t.Errorf("an address that is not one was answered %q", msg)
	}
}

func TestWithoutAReaderTheRequestSaysWhatToInstall(t *testing.T) {
	was := scanned.Find
	scanned.Find = func() (scanned.Engine, error) { return scanned.Engine{}, errNoScanEngine }
	defer func() { scanned.Find = was }()
	h := newHarness(t)
	if msg := h.postFails("/api/openscan"); !strings.Contains(msg, "uv") || !strings.Contains(msg, scanEngineEnv) {
		t.Errorf("with no reader: %q", msg)
	}
}
