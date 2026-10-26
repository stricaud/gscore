/*
 * dialogclosed_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"errors"
	"os/exec"
	"testing"
)

// A dialog that was closed is not a dialog that failed.
//
// On Linux the panel is zenity or kdialog, and from the outside a person
// closing one and a machine unable to show one look identical: no output and
// a status that is not zero. Every failure was read as a cancellation, so
// File > Open on a machine where the dialog could not start did nothing at
// all — no score, no message, nothing to say anything had been asked for.
//
// The difference matters because of what the caller does with it. A
// cancellation means the person changed their mind and gscore should stand
// still. A failure means there is no panel here, and the answer to that is
// to offer the browser's own file picker — which only happens if the failure
// is reported as one.
func TestAClosedDialogIsNotAFailedOne(t *testing.T) {
	// A tool that exits 1, which is what both of them do when the panel is
	// closed.
	closedErr := run(t, "exit 1")
	if !closed(closedErr) {
		t.Errorf("exit 1 read as a failure (%v), want a cancellation", closedErr)
	}

	// zenity answers 255 when it cannot show a panel at all.
	brokenErr := run(t, "exit 255")
	if closed(brokenErr) {
		t.Error("exit 255 read as a cancellation, so the failure is silent")
	}

	// And a tool that will not start at all never gets a code.
	if _, err := exec.Command("gscore-no-such-dialog-tool").Output(); err == nil {
		t.Skip("there is a program by that name on this machine")
	} else if closed(err) {
		t.Error("a dialog that could not be started read as a cancellation")
	}

	// Nothing wrong at all is not a cancellation either.
	if closed(nil) {
		t.Error("a dialog that worked read as a cancellation")
	}
}

// run is the error from a command that exits with the given status.
func run(t *testing.T, script string) error {
	t.Helper()
	_, err := exec.Command("sh", "-c", script).Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("sh -c %q gave %v, want an exit status", script, err)
	}
	return err
}
