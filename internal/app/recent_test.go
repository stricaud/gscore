/*
 * recent_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"os"
	"path/filepath"
	"testing"
)

// The files opened before.
//
// A list of paths on this machine, which is why it exists only where there
// is a machine: served to somebody else's browser the paths would be the
// server's, and offering them would be offering to open files the person
// asking has never seen.

// settingsIn points the settings at a directory of this test's own, so that
// running the tests does not rewrite what is on the machine.
func settingsIn(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // what os.UserConfigDir reads on Linux
	t.Setenv("HOME", dir)            // and on macOS
	stored.Lock()
	stored.settings, stored.read = settings{}, false
	stored.Unlock()
	t.Cleanup(func() {
		stored.Lock()
		stored.settings, stored.read = settings{}, false
		stored.Unlock()
	})
}

func touch(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("X:1\nK:C\nc4|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheNewestFileIsFirstAndNothingIsListedTwice(t *testing.T) {
	settingsIn(t)
	a := &App{local: true}

	one, two := touch(t, "one.abc"), touch(t, "two.abc")
	a.remember(one)
	a.remember(two)
	a.remember(one) // opened again

	got := a.recentFiles()
	if len(got) != 2 {
		t.Fatalf("%d files remembered, want 2: %v", len(got), got)
	}
	if got[0].Path != one || got[1].Path != two {
		t.Errorf("got %v, want the one opened last at the top", got)
	}
	if got[0].Name != "one.abc" {
		t.Errorf("the name shown is %q", got[0].Name)
	}
}

// A file that has been moved or deleted is not offered.
func TestAFileThatHasGoneIsDropped(t *testing.T) {
	settingsIn(t)
	a := &App{local: true}

	kept, gone := touch(t, "kept.abc"), touch(t, "gone.abc")
	a.remember(kept)
	a.remember(gone)
	os.Remove(gone)

	got := a.recentFiles()
	if len(got) != 1 || got[0].Path != kept {
		t.Fatalf("got %v, want only the file that is still there", got)
	}
	// And it is forgotten rather than checked again every time.
	if r := loadSettings().Recent; len(r) != 1 {
		t.Errorf("the list still holds %d paths", len(r))
	}
}

// The list is capped, oldest out.
func TestTheListDoesNotGrowForever(t *testing.T) {
	settingsIn(t)
	a := &App{local: true}
	dir := t.TempDir()
	for i := 0; i < mostRecent+5; i++ {
		path := filepath.Join(dir, string(rune('a'+i))+".abc")
		if err := os.WriteFile(path, []byte("X:1\nK:C\nc4|\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		a.remember(path)
	}
	if got := len(a.recentFiles()); got != mostRecent {
		t.Errorf("%d files remembered, want at most %d", got, mostRecent)
	}
}

// Served over a network there is no list: the paths are the server's, and
// the person asking has never seen them.
func TestServedOverANetworkThereIsNoList(t *testing.T) {
	settingsIn(t)
	here := &App{local: true}
	here.remember(touch(t, "mine.abc"))

	away := &App{local: false}
	away.remember(touch(t, "theirs.abc"))
	if got := away.recentFiles(); got != nil {
		t.Errorf("a served gscore offered %v", got)
	}
	if got := loadSettings().Recent; len(got) != 1 {
		t.Errorf("a served gscore wrote to the list: %v", got)
	}
}
