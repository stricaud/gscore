/*
 * crypt_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A .gscore locked under a password: the music opens only with it, there
 * is no PDF in front of it to read, and the file is still a .gscore to
 * everything that looks for the zip directory.
 */

package gscore

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/symbol"
)

func lockedScore(t *testing.T) (*model.Score, []byte) {
	t.Helper()
	s := model.NewScore()
	s.Identity.Title = "Secret tune"
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	p.Measures = []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{
		&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 1}},
	}}}}}
	s.Parts = []*model.Part{p}
	var buf bytes.Buffer
	if err := WriteEncrypted(&buf, s, symbol.NewRegistry(), "hunter2"); err != nil {
		t.Fatal(err)
	}
	return s, buf.Bytes()
}

func TestALockedFileOpensOnlyWithItsPassword(t *testing.T) {
	_, data := lockedScore(t)

	if _, _, err := Read(bytes.NewReader(data)); !errors.Is(err, ErrPasswordNeeded) {
		t.Errorf("read with no password: %v, want ErrPasswordNeeded", err)
	}
	if _, _, err := ReadWithPassword(bytes.NewReader(data), "hunter3"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("read with the wrong password: %v, want ErrWrongPassword", err)
	}
	back, reg, err := ReadWithPassword(bytes.NewReader(data), "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if back.Identity.Title != "Secret tune" || len(back.Parts) != 1 {
		t.Errorf("the score came back as %q with %d parts", back.Identity.Title, len(back.Parts))
	}
	if reg == nil {
		t.Error("no registry came back")
	}
	// The symbols are reached the same way.
	if _, err := ReadSymbols(bytes.NewReader(data)); !errors.Is(err, ErrPasswordNeeded) {
		t.Errorf("symbols read with no password: %v", err)
	}
	if _, err := ReadSymbolsWithPassword(bytes.NewReader(data), "hunter2"); err != nil {
		t.Errorf("symbols read with the password: %v", err)
	}
}

func TestTheMusicOfALockedFileIsNotInTheClear(t *testing.T) {
	_, data := lockedScore(t)
	// No PDF in front: the file is the sealed container alone.
	if got := PDFPrefix(data); len(got) != 0 {
		t.Errorf("a locked file carries %d bytes of PDF in front", len(got))
	}
	// The container holds nothing readable: no score.json, and the title
	// nowhere in it, deflated or not.
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var info []byte
	for _, f := range zr.File {
		if f.Name == "score.json" {
			t.Error("score.json is in the clear")
		}
		b, err := readAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("Secret tune")) {
			t.Errorf("%s holds the title in the clear", f.Name)
		}
		if f.Name == "META-INF/format.json" {
			info = b
		}
	}
	if !bytes.Contains(info, []byte(`"cipher": "AES-256-GCM"`)) {
		t.Errorf("format.json does not say how the file is locked: %s", info)
	}
}

// An unlocked file ignores a password: somebody typing one into a file
// that has none is not refused.
func TestAnUnlockedFileIgnoresAPassword(t *testing.T) {
	s := model.NewScore()
	s.Identity.Title = "Open tune"
	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := ReadWithPassword(bytes.NewReader(buf.Bytes()), "whatever")
	if err != nil || back.Identity.Title != "Open tune" {
		t.Errorf("an unlocked file read with a password: %v, %v", back, err)
	}
}
