/*
 * withpdf_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package gscore

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A saved score can carry a PDF of itself in front of the music, so that the
// one file is both: renamed .pdf it opens in anything that reads one, and it
// is still the score.
//
// The two formats are read from opposite ends — a PDF from its head, a zip
// from its tail — which is what lets them share a file.

func aScore() *model.Score {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 3; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{&model.Note{
				Pitch: model.Pitch{Step: model.StepG, Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 1},
			}}}},
			RightBarline: model.BarSingle,
		})
	}
	s := &model.Score{
		Identity: model.Identity{Title: "Carried"},
		Tempo:    model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts:    []*model.Part{p},
	}
	s.Normalise()
	return s
}

// A stand-in for an engraved PDF: this package does not draw, so what it is
// handed is bytes, and all that matters is that they are left in front
// untouched and that the music can still be found behind them.
var pretendPDF = []byte("%PDF-1.6\n% a drawing would be here\n%%EOF\n")

func TestAScoreCanCarryAPDFInFrontOfIt(t *testing.T) {
	var b bytes.Buffer
	if err := WriteWithPDF(&b, aScore(), nil, pretendPDF); err != nil {
		t.Fatal(err)
	}
	out := b.Bytes()

	// The file begins as the PDF does, which is what a PDF reader looks for.
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Errorf("it begins %q, so nothing will take it for a PDF", out[:8])
	}
	if !bytes.HasPrefix(out, pretendPDF) {
		t.Error("the drawing was not left in front untouched")
	}
	// And it ends as a zip does, which is what a zip reader looks for.
	if !bytes.Contains(out[len(out)-64:], []byte("PK\x05\x06")) {
		t.Error("the container's directory is not at the end of the file")
	}

	// It is still the score.
	got, _, err := Read(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("it did not read back as a score: %v", err)
	}
	if got.Identity.Title != "Carried" {
		t.Errorf("the title came back as %q", got.Identity.Title)
	}
	if n := len(got.Parts[0].Measures); n != 3 {
		t.Errorf("%d bars came back, want 3", n)
	}

	// And an ordinary zip reader finds it too, prefix and all: a file only
	// this program can open is not worth the trick.
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("a plain zip reader could not open it: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if len(names) < 2 {
		t.Errorf("the container holds %v", names)
	}
}

// Without one it is the container and nothing else, which is three decimal
// orders smaller for a short piece.
func TestWithoutAPDFItIsJustTheScore(t *testing.T) {
	var with, without bytes.Buffer
	if err := WriteWithPDF(&with, aScore(), nil, pretendPDF); err != nil {
		t.Fatal(err)
	}
	if err := WriteWithPDF(&without, aScore(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(without.Bytes(), []byte("PK")) {
		t.Error("it does not begin as a container")
	}
	if without.Len() >= with.Len() {
		t.Errorf("carrying nothing cost %d bytes against %d",
			without.Len(), with.Len())
	}

	// Both are the same score.
	for _, c := range []struct {
		what string
		b    *bytes.Buffer
	}{{"with", &with}, {"without", &without}} {
		got, _, err := Read(bytes.NewReader(c.b.Bytes()))
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if got.Identity.Title != "Carried" {
			t.Errorf("%s: the title came back as %q", c.what, got.Identity.Title)
		}
	}
}

// Write is left alone: it is what everything else in the program uses, and
// what a plain container means.
func TestTheOrdinaryWriterIsUnchanged(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, aScore(), nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b.Bytes(), []byte("PK")) {
		t.Error("Write no longer writes a plain container")
	}
}
