/*
 * player_node_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package html

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The player in a shared page is JavaScript kept here as a string, so
// building gscore says nothing about whether it works.
//
// What it got wrong was a matter of how much it asked for at once rather than
// of what it asked for, and no reading of it would have said so: it created
// three audio nodes for every note in the score before the first one sounded,
// which a browser does up to a point and then stops. A symphony came out as
// its first few staves over an orchestra of silent ones. That is the kind of
// fault that has to be run to be found, so it is run.
//
// Node is not a dependency of gscore and the release does not need it, so a
// machine without it skips rather than fails — the same bargain the editor's
// own tests make in internal/app.
func TestThePlayerPlays(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed, so the player's own tests are skipped")
	}

	// The player as a page would receive it, written where node can read it.
	dir := t.TempDir()
	path := filepath.Join(dir, "player.js")
	if err := os.WriteFile(path, []byte(PlayerJS()), 0o644); err != nil {
		t.Fatalf("writing the player out: %v", err)
	}

	script := filepath.Join("testdata", "player_test.mjs")
	cmd := exec.Command(node, "--test", script)
	cmd.Env = append(os.Environ(), "GSCORE_PLAYER="+path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("the player's tests failed:\n%s", out)
		return
	}
	// A run that matched no test file also exits zero, which would make this
	// pass by finding nothing.
	if !strings.Contains(string(out), "# pass ") && !strings.Contains(string(out), "✔") {
		t.Errorf("no player tests ran:\n%s", out)
	}
}
