package app

import (
	"os"
	"path/filepath"
	"testing"
)

// The folder is made when it is first asked for, under Documents.
func TestScoresDirIsMadeUnderDocuments(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows

	got := makeScoresDir()
	want := filepath.Join(home, "Documents", ScoresFolder)
	if got != want {
		t.Fatalf("scores go to %q, want %q", got, want)
	}
	st, err := os.Stat(got)
	if err != nil {
		t.Fatalf("the folder was not made: %v", err)
	}
	if !st.IsDir() {
		t.Error("it is not a directory")
	}
}

// A home with no Documents still gets a folder, beside whatever else is there.
func TestScoresDirWithoutDocuments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := makeScoresDir()
	if want := filepath.Join(home, ScoresFolder); got != want {
		t.Fatalf("scores go to %q, want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("the folder was not made: %v", err)
	}
}

// Something already called Gscore that is not a folder is left alone.
func TestScoresDirLeavesAFileAlone(t *testing.T) {
	home := t.TempDir()
	docs := filepath.Join(home, "Documents")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, ScoresFolder), []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if got := makeScoresDir(); got != home {
		t.Errorf("fell back to %q, want the home directory %q", got, home)
	}
}
