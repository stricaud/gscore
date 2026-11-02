/*
 * savefile_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Saving writes beside the file and moves it into place.
 *
 * It used to write straight onto the path, which truncates what is there
 * before anything has been written. Two things followed, both bad.
 *
 * A write that failed left nothing. Saving a score opened from a
 * MuseScore file writes back to the file it came from, gscore has no
 * exporter for that format, and the refusal arrived after the original
 * had been emptied: six megabytes of somebody's symphony gone.
 *
 * And a write that is slow left nothing for as long as it took. A
 * .gscore carries a PDF of the score, and rendering one of a large score
 * takes a while — during which the file on disk was there and empty.
 */

package app

import (
	"os"
	"path/filepath"
	"testing"
)

// A save gscore cannot do must not touch what is already there.
func TestAFailedSaveLeavesTheFileAlone(t *testing.T) {
	h := newHarness(t)
	// Saving to a path is only allowed from this machine, which is what
	// the server decides from the address it is listening on. The
	// harness listens on nothing, so it has to be said here.
	h.app.local = true
	h.note(0, 0, 2, "quarter")

	// A format with no exporter, holding something worth keeping.
	path := filepath.Join(t.TempDir(), "score.mscz")
	was := []byte("the original file, which is not gscore's to destroy")
	if err := os.WriteFile(path, was, 0o644); err != nil {
		t.Fatal(err)
	}

	if why := h.postFails("/api/save?path=" + path); why == "" {
		t.Fatal("saving to a format gscore cannot write was accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file is gone: %v", err)
	}
	if string(got) != string(was) {
		t.Errorf("the file is now %d bytes, was %d", len(got), len(was))
	}
}

// And must not leave a new one behind either: a refused save that creates
// an empty file is a file somebody has to notice and delete.
func TestAFailedSaveWritesNoFileAtAll(t *testing.T) {
	h := newHarness(t)
	// Saving to a path is only allowed from this machine, which is what
	// the server decides from the address it is listening on. The
	// harness listens on nothing, so it has to be said here.
	h.app.local = true
	h.note(0, 0, 2, "quarter")

	dir := t.TempDir()
	path := filepath.Join(dir, "score.mscz")
	if why := h.postFails("/api/save?path=" + path); why == "" {
		t.Fatal("saving to a format gscore cannot write was accepted")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a refused save left a file behind")
	}
	// Nor the file it writes beside, which is cleaned up either way.
	left, _ := os.ReadDir(dir)
	for _, e := range left {
		t.Errorf("%q was left in the directory", e.Name())
	}
}

// What it does write is whole, and readable by somebody who is sent it.
func TestASavedScoreIsWholeAndReadable(t *testing.T) {
	h := newHarness(t)
	// Saving to a path is only allowed from this machine, which is what
	// the server decides from the address it is listening on. The
	// harness listens on nothing, so it has to be said here.
	h.app.local = true
	h.note(0, 0, 2, "quarter")

	path := filepath.Join(t.TempDir(), "score.gscore")
	h.post("/api/save?path=" + path)

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("nothing was written: %v", err)
	}
	if st.Size() == 0 {
		t.Fatal("an empty file was written")
	}
	// CreateTemp makes a file its owner alone can read, which is not what
	// anybody expects of a score they are about to send to a choir.
	if mode := st.Mode().Perm(); mode != 0o644 {
		t.Errorf("the file is mode %o, want 644", mode)
	}
}

// Saving over a score that is already there replaces it.
func TestSavingOverAScoreReplacesIt(t *testing.T) {
	h := newHarness(t)
	// Saving to a path is only allowed from this machine, which is what
	// the server decides from the address it is listening on. The
	// harness listens on nothing, so it has to be said here.
	h.app.local = true
	h.note(0, 0, 2, "quarter")

	path := filepath.Join(t.TempDir(), "score.gscore")
	if err := os.WriteFile(path, []byte("an older save"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.post("/api/save?path=" + path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "an older save" {
		t.Error("the older file is still there")
	}
	if len(got) < 100 {
		t.Errorf("what was written is %d bytes, which is not a score", len(got))
	}
}
