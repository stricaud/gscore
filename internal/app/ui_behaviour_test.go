/*
 * ui_behaviour_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"os/exec"
	"strings"
	"testing"
)

// Some of the interface is arithmetic rather than wiring, and arithmetic can
// be run.
//
// ui_wiring_test.go does the checks a compiler would do by reading the files
// as text, which is as far as reading can get: whether shift with an arrow
// grows a selection or collapses it is a question about what the code does,
// not about what it mentions. So the functions that answer it are lifted out
// of app.js and run, from here, so that they go with go test like everything
// else rather than needing a second command nobody remembers.
//
// Node is not a dependency of gscore and the release does not need it, so a
// machine without it skips rather than fails.
func TestTheEditorsSelectionArithmetic(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed, so the interface's own tests are skipped")
	}
	out, err := exec.Command(node, "--test", "ui/").CombinedOutput()
	if err != nil {
		t.Errorf("the interface's tests failed:\n%s", out)
		return
	}
	// A run that matched no test file also exits zero, which would make this
	// pass by finding nothing.
	if !strings.Contains(string(out), "# pass ") &&
		!strings.Contains(string(out), "pass 1") &&
		!strings.Contains(string(out), "✔") {
		t.Errorf("no interface tests ran:\n%s", out)
	}
}
