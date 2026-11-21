/*
 * scanned_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The page reader is a program run over copies of the pictures, and what
 * it writes is read as MusicXML. A stand-in reader, a shell script, plays
 * the program.
 */

package scanned

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

// A page of one bar: a whole note C.
const onePage = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0">
  <part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>
  <part id="P1">
    <measure number="1">
      <attributes><divisions>1</divisions><time><beats>4</beats><beat-type>4</beat-type></time>
        <clef><sign>G</sign><line>2</line></clef></attributes>
      <note><pitch><step>C</step><octave>5</octave></pitch><duration>4</duration><type>whole</type></note>
    </measure>
  </part>
</score-partwise>
`

// pngBytes is the smallest thing that is a PNG to anybody looking at its
// first bytes, and jpegBytes a JPEG.
var (
	pngBytes  = []byte("\x89PNG\r\n\x1a\n" + "not really a picture")
	jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 'J', 'F', 'I', 'F'}
)

// fakeReader is a stand-in for homr: a script that writes the page above
// beside the first picture it is given, and notes what it was asked.
func fakeReader(t *testing.T) (Engine, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in reader is a shell script")
	}
	dir := t.TempDir()
	xml := filepath.Join(dir, "page.musicxml")
	asked := filepath.Join(dir, "asked.txt")
	if err := os.WriteFile(xml, []byte(onePage), 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "homr")
	body := "#!/bin/sh\n" +
		"echo \"$PWD $*\" > '" + asked + "'\n" +
		"for a in \"$@\"; do case \"$a\" in -*) ;; *) first=\"$a\"; break;; esac; done\n" +
		"cp '" + xml + "' \"${first%.*}.musicxml\"\n" +
		"echo 'Finished' >&2\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return Engine{Command: []string{script}, Source: "the test"}, asked
}

func pitchesOf(s *model.Score) string {
	var out []string
	for _, m := range s.Parts[0].Measures {
		for _, el := range m.Voices[0].Elements {
			if n, ok := el.(*model.Note); ok {
				out = append(out, n.Pitch.String())
			}
		}
	}
	return strings.Join(out, " ")
}

func TestPagesAreCopiedNamedInOrderAndReadAsOne(t *testing.T) {
	eng, asked := fakeReader(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "iputaspellonyousheetmusic1.jpg")
	b := filepath.Join(dir, "second.png")
	os.WriteFile(a, jpegBytes, 0o600)
	os.WriteFile(b, pngBytes, 0o600)

	s, err := ImportFiles(context.Background(), eng, []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if got := pitchesOf(s); got != "C5" {
		t.Errorf("the page read as %q", got)
	}
	said, _ := os.ReadFile(asked)
	// Run in a folder of its own, on the pages in order, without the title
	// detection, and named by what they are rather than what they were
	// called.
	if !strings.Contains(string(said), "--no-title page-01.jpg page-02.png") {
		t.Errorf("the reader was asked %q", said)
	}
	if strings.Contains(string(said), dir) {
		t.Errorf("the reader was run on the originals: %q", said)
	}
}

func TestOnePageComesThroughTheRegistry(t *testing.T) {
	eng, asked := fakeReader(t)
	was := Find
	Find = func() (Engine, error) { return eng, nil }
	defer func() { Find = was }()

	imp, err := format.ImporterFor("photo.JPG")
	if err != nil {
		t.Fatal(err)
	}
	if imp.Name() != "Scanned page" {
		t.Fatalf("a .JPG goes to %q", imp.Name())
	}
	// The bytes say PNG whatever the name said.
	s, err := imp.Import(strings.NewReader(string(pngBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if got := pitchesOf(s); got != "C5" {
		t.Errorf("the page read as %q", got)
	}
	if said, _ := os.ReadFile(asked); !strings.Contains(string(said), "page-01.png") {
		t.Errorf("the picture was given to the reader as %q", said)
	}
}

func TestWithoutAReaderNothingIsRead(t *testing.T) {
	was := Find
	Find = func() (Engine, error) { return Engine{}, ErrNoEngine }
	defer func() { Find = was }()
	_, err := Import(strings.NewReader(string(pngBytes)))
	if err != ErrNoEngine {
		t.Errorf("with no reader: %v", err)
	}
	_, err = Import(strings.NewReader("<html>a page of the other kind</html>"))
	if err == nil || !strings.Contains(err.Error(), "neither a PNG") {
		t.Errorf("a web page was answered %v", err)
	}
}

func TestAReaderThatFailsSaysWhy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "homr")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'lots of progress'\necho 'CUDA device not found' >&2\nexit 3\n"), 0o700)
	page := filepath.Join(dir, "p.png")
	os.WriteFile(page, pngBytes, 0o600)
	_, err := ImportFiles(context.Background(), Engine{Command: []string{script}}, []string{page})
	if err == nil || !strings.Contains(err.Error(), "CUDA device not found") {
		t.Errorf("the failure was reported as %v", err)
	}

	// And one that says nothing and writes nothing.
	os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o700)
	_, err = ImportFiles(context.Background(), Engine{Command: []string{script}}, []string{page})
	if err == nil || !strings.Contains(err.Error(), "wrote no music") {
		t.Errorf("silence was reported as %v", err)
	}
}

func TestKind(t *testing.T) {
	if Kind(pngBytes) != ".png" || Kind(jpegBytes) != ".jpg" || Kind([]byte("%PDF-1.4")) != ".pdf" || Kind([]byte("<svg/>")) != "" {
		t.Error("the kinds are not told apart by their first bytes")
	}
}

// The reader's own account of its work is read into a position, line by
// line, as homr writes it.
func TestTheReadersProgressIsFollowed(t *testing.T) {
	var seen []Progress
	w := &progressWriter{report: func(p Progress) { seen = append(seen, p) }}
	lines := []string{
		"Merging 2 files: ['page-01.jpg', 'page-02.jpg']",
		"=========================================",
		"Processing page-01.jpg",
		"Starting Inference.",
		"Found 936 staff line fragments",
		"Found 4 staffs",
		"Dewarping staff 0",
		"Running TrOmr inference on staff image 0",
		"Running TrOmr inference on staff image 1",
		"Running TrOmr inference on staff image 2",
		"Running TrOmr inference on staff image 3",
		"Finished parsing 2 staves",
		"Finished page-01.jpg",
		"Processing page-02.jpg",
		"Found 2 staffs",
		"Running TrOmr inference on staff image 0",
	}
	// In pieces, as a pipe delivers them: a line may arrive in two writes.
	text := strings.Join(lines, "\n") + "\n"
	w.Write([]byte(text[:40]))
	w.Write([]byte(text[40:]))
	if len(seen) == 0 {
		t.Fatal("nothing was reported")
	}
	last := seen[len(seen)-1]
	if last.Page != 2 || last.Pages != 2 || last.Staff != 1 || last.Staves != 2 {
		t.Errorf("the position at the end is %+v", last)
	}
	// Half a page through the second of two: a little over three quarters.
	if last.Percent < 70 || last.Percent > 80 {
		t.Errorf("the percentage at the end is %d", last.Percent)
	}
	if last.String() != "page 2 of 2, staff 1 of 2" {
		t.Errorf("said as %q", last.String())
	}
	// The percentage only ever climbs, and never says done.
	for i := 1; i < len(seen); i++ {
		if seen[i].Percent < seen[i-1].Percent {
			t.Errorf("the percentage fell from %d to %d at %+v", seen[i-1].Percent, seen[i].Percent, seen[i])
		}
		if seen[i].Percent >= 100 {
			t.Errorf("the percentage reached %d before the end", seen[i].Percent)
		}
	}
	// Reported from the reader itself, when it is given somewhere to say so.
	eng, _ := fakeReader(t)
	page := filepath.Join(t.TempDir(), "p.png")
	os.WriteFile(page, pngBytes, 0o600)
	var told int
	if _, err := ImportFilesWith(context.Background(), eng, []string{page}, func(Progress) { told++ }); err != nil {
		t.Fatal(err)
	}
	_ = told // the stand-in says only "Finished", which is no position
}
