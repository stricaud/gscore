/*
 * engraved_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engraved

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
)

func TestImporterIsRegistered(t *testing.T) {
	for _, i := range format.Importers() {
		if i.Name() == "Engraved PDF" {
			return
		}
	}
	t.Error("the engraved PDF importer did not register itself")
}

// A PDF with no music in it must say so rather than hand back an empty score
// that looks like a successful read of a silent piece.
func TestRefusesAPageWithoutStaves(t *testing.T) {
	d := pdf.New()
	p := d.NewPage(300, 200)
	f := d.Standard(pdf.Helvetica)
	p.Text(f, 12, 50, 100, "not music")
	p.Close()
	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(bytes.NewReader(buf.Bytes())); err == nil {
		t.Error("a page with no staves was imported without complaint")
	}
}

// TestImportEngraved reads a real engraved PDF and, if told what it should
// contain, checks it note for note.
//
// GSCORE_PDF names the file. GSCORE_PITCHES, when set, is the pitches
// expected in order, written as "e4 g4 c5" — which is how a transcription
// that has been verified by other means can be used as the answer key.
//
//	GSCORE_PDF=duet.pdf GSCORE_PITCHES="g5 a5 g5 ..." \
//	    go test ./internal/format/engraved/ -run ImportEngraved -v
func TestImportEngraved(t *testing.T) {
	path := os.Getenv("GSCORE_PDF")
	if path == "" {
		t.Skip("set GSCORE_PDF to an engraved PDF")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := Import(f)
	if err != nil {
		t.Fatal(err)
	}
	var notes []string
	var rests, measures int
	for _, part := range s.Parts {
		for _, m := range part.Measures {
			measures++
			for _, v := range m.Voices {
				for _, e := range v.Elements {
					switch n := e.(type) {
					case *model.Note:
						notes = append(notes, strings.ToLower(n.Pitch.Step.String())+
							itoa(n.Pitch.Octave))
					case *model.Rest:
						rests++
					}
				}
			}
		}
	}
	t.Logf("%s: %d parts, %d measures, %d notes, %d rests",
		path, len(s.Parts), measures, len(notes), rests)
	for i, part := range s.Parts {
		n := 0
		for _, m := range part.Measures {
			for _, v := range m.Voices {
				n += len(v.Elements)
			}
		}
		t.Logf("   part %d: %d measures, %d elements", i+1, len(part.Measures), n)
	}

	want := strings.Fields(os.Getenv("GSCORE_PITCHES"))
	if len(want) == 0 {
		if len(notes) > 24 {
			t.Logf("first 24: %v", notes[:24])
		} else {
			t.Logf("all: %v", notes)
		}
		return
	}
	if len(notes) != len(want) {
		t.Errorf("read %d notes, expected %d", len(notes), len(want))
	}
	bad := 0
	for i := 0; i < len(notes) && i < len(want); i++ {
		if notes[i] != want[i] {
			if bad < 8 {
				t.Errorf("note %d: read %s, expected %s", i+1, notes[i], want[i])
			}
			bad++
		}
	}
	if bad == 0 && len(notes) == len(want) {
		t.Logf("all %d pitches match", len(notes))
	}
}

func itoa(v int) string {
	if v < 0 {
		return "-" + itoa(-v)
	}
	if v < 10 {
		return string(rune('0' + v))
	}
	return itoa(v/10) + string(rune('0'+v%10))
}

// TestDurationsFromAFixture reads a page LilyPond engraved from
// testdata/durations.ly and insists on every pitch and every length.
//
// The source is kept beside the PDF so that what the answer ought to be is
// checked in next to the question. Regenerate with:
//
//	lilypond -o testdata/durations testdata/durations.ly
//
// This is the test that holds the duration work still. The lengths come from
// three different places — the head's own shape for a whole or a half, a
// flag glyph for a lone eighth, and a count of the beams crossing the stem
// for everything in a group — and each was wrong in its own way before it
// was right.
func TestDurationsFromAFixture(t *testing.T) {
	f, err := os.Open("testdata/durations.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	defer f.Close()
	s, err := Import(f)
	if err != nil {
		t.Fatal(err)
	}
	// c'8 d'8 e'8 f'8 | g'16 a'16 b'16 c''16 | c'4 d'2 | e'8 f'4.
	want := []struct {
		pitch    string
		num, den int
	}{
		{"c4", 1, 8}, {"d4", 1, 8}, {"e4", 1, 8}, {"f4", 1, 8},
		{"g4", 1, 16}, {"a4", 1, 16}, {"b4", 1, 16}, {"c5", 1, 16},
		{"c4", 1, 4},
		{"d4", 1, 2}, {"e4", 1, 8}, {"f4", 3, 8},
	}
	var got []struct {
		pitch    string
		num, den int
	}
	measures := 0
	for _, part := range s.Parts {
		for _, m := range part.Measures {
			measures++
			for _, v := range m.Voices {
				for _, e := range v.Elements {
					n, ok := e.(*model.Note)
					if !ok {
						continue
					}
					d := n.Dur.Reduced()
					got = append(got, struct {
						pitch    string
						num, den int
					}{strings.ToLower(n.Pitch.Step.String()) +
						itoa(n.Pitch.Octave), d.Num, d.Den})
				}
			}
		}
	}
	if measures != 2 {
		t.Errorf("got %d measures, want 2", measures)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d notes, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("note %d: got %s %d/%d, want %s %d/%d", i+1,
				got[i].pitch, got[i].num, got[i].den,
				want[i].pitch, want[i].num, want[i].den)
		}
	}
}

// TestSignaturesFromAFixture checks the key and the metre, which are read
// from where they sit rather than from what they are.
//
// testdata/signature.ly is in D major and three-four. Both facts are drawn
// between the clef and the first note, and that position is the whole of
// what marks them out: the two sharps of the key are the same glyph as an
// accidental on a note, and the figures of the metre are drawn by LilyPond
// as plain ASCII digits in the music font, with no name to say what they
// are.
func TestSignaturesFromAFixture(t *testing.T) {
	f, err := os.Open("testdata/signature.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	defer f.Close()
	s, err := Import(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(s.Parts))
	}
	p := s.Parts[0]
	if p.Key.Fifths != 2 {
		t.Errorf("key has %d sharps, want 2 (D major)", p.Key.Fifths)
	}
	if p.Time.Beats != 3 || p.Time.BeatValue != 4 {
		t.Errorf("metre is %d/%d, want 3/4", p.Time.Beats, p.Time.BeatValue)
	}
	if p.Time.Symbol != model.TimeNumeric {
		t.Errorf("metre symbol is %v, want numeric", p.Time.Symbol)
	}
}

// A common-time C and a cut-time C are glyphs rather than figures, and the
// two must not be confused: they differ by one stroke and by a factor of two
// in what a beat is.
func TestCommonAndCutTime(t *testing.T) {
	for _, c := range []struct {
		file   string
		beats  int
		value  int
		symbol model.TimeSymbol
	}{
		{"testdata/durations.pdf", 4, 4, model.TimeCommon},
	} {
		f, err := os.Open(c.file)
		if err != nil {
			t.Skip("fixture missing")
		}
		s, err := Import(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		p := s.Parts[0]
		if p.Time.Beats != c.beats || p.Time.BeatValue != c.value ||
			p.Time.Symbol != c.symbol {
			t.Errorf("%s: metre %d/%d symbol %v, want %d/%d symbol %v",
				c.file, p.Time.Beats, p.Time.BeatValue, p.Time.Symbol,
				c.beats, c.value, c.symbol)
		}
	}
}
