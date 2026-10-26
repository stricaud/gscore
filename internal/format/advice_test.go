/*
 * advice_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package format_test

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"

	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
	_ "github.com/stricaud/gscore/internal/format/musx"
	_ "github.com/stricaud/gscore/internal/format/nwc"
)

// What a file gscore will not open is told.
//
// "no importer registered for .sib" is true and useless: somebody holding a
// Sibelius file learns that gscore said no and nothing about what to do. The
// answer is the same for all of them — every one of these programs writes
// MusicXML — so it may as well be given.
func TestAFileThatCannotBeOpenedSaysWhatToDo(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"Overture.sib", "Sibelius"},
		{"old.mus", "Finale"},
		{"score.dorico", "Dorico"},
		{"a scan.pdf", "picture of a score"},
	} {
		_, err := format.ImporterFor(tc.name)
		if err == nil {
			t.Errorf("%s was claimed by an importer", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %q does not mention %s", tc.name, err, tc.want)
		}
		if strings.Contains(err.Error(), "no importer registered") {
			t.Errorf("%s: %q is the old message", tc.name, err)
		}
	}

	// And an extension nobody claims says what gscore does read, rather
	// than only what it does not.
	_, err := format.ImporterFor("notes.txt")
	if err == nil {
		t.Fatal(".txt was claimed by an importer")
	}
	for _, want := range []string{".abc", ".gscore", ".musx"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not offer %s", err, want)
		}
	}
}

// Finale files open, which is the whole point of the musx package.
func TestFinaleFilesAreClaimed(t *testing.T) {
	imp, err := format.ImporterFor("Sanctus.musx")
	if err != nil {
		t.Fatalf("a .musx is not claimed by anything: %v", err)
	}
	if imp.Name() != "Finale" {
		t.Errorf("%q claims it, want Finale", imp.Name())
	}
}

// Readable is what the file panels and the messages are built from, so it
// has to be the registry and not a list written out beside it.
func TestReadableListsEveryImporter(t *testing.T) {
	got := strings.Join(format.Readable(), " ")
	for _, want := range []string{".abc", ".gscore", ".mscz", ".musicxml", ".mxl", ".nwc", ".nwctxt", ".musx"} {
		if !strings.Contains(got, want) {
			t.Errorf("Readable() is %q, which leaves out %s", got, want)
		}
	}
}
