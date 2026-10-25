/*
 * keybindings_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"regexp"
	"strings"
	"testing"
)

// The keyboard is a table now, and a table can be checked.
//
// It was sixty cases of a switch, which meant nothing could list them for
// the person using the program, nothing noticed two of them landing on the
// same key, and nothing could let anybody change one. These are the things
// worth knowing about a table of bindings, and none of them could be asked
// of the switch.

// bindings pulls the table out of app.js: each entry's id, keys, group and
// label.
func bindings(t *testing.T) []map[string]string {
	t.Helper()
	js := readUI(t, "app.js")
	start := strings.Index(js, "const KEY_BINDINGS = [")
	if start < 0 {
		t.Fatal("app.js has no KEY_BINDINGS table")
	}
	end := strings.Index(js[start:], "\n];")
	if end < 0 {
		t.Fatal("the KEY_BINDINGS table is not closed")
	}
	body := js[start : start+end]

	field := func(entry, name string) string {
		m := regexp.MustCompile(name + `:\s*"((?:[^"\\]|\\.)*)"`).FindStringSubmatch(entry)
		if m == nil {
			return ""
		}
		return m[1]
	}
	var out []map[string]string
	// Each entry, from its id to the start of the next one. The id is put
	// back because splitting on it takes it away.
	for _, entry := range regexp.MustCompile(`\{\s*id:`).Split(body, -1)[1:] {
		entry = "id:" + entry
		out = append(out, map[string]string{
			"id":    field(entry, "id"),
			"keys":  field(entry, "keys"),
			"group": field(entry, "group"),
			"label": field(entry, "label"),
		})
	}
	if len(out) < 40 {
		t.Fatalf("only %d bindings were read, which is fewer than there are", len(out))
	}
	return out
}

// Every binding has to say what it is, what it does and where it is listed,
// or the shortcuts window has a blank line in it.
func TestEveryBindingIsDescribed(t *testing.T) {
	for _, b := range bindings(t) {
		for _, want := range []string{"id", "keys", "group", "label"} {
			if b[want] == "" {
				t.Errorf("the binding %q has no %s", b["id"], want)
			}
		}
	}
}

// Two commands on one key means one of them cannot be reached, and the
// switch gave no sign of it: whichever case came first simply won.
func TestNoTwoBindingsShareAKey(t *testing.T) {
	seen := map[string]string{}
	for _, b := range bindings(t) {
		if first, ok := seen[b["keys"]]; ok {
			t.Errorf("%q and %q are both on %s", first, b["id"], b["keys"])
			continue
		}
		seen[b["keys"]] = b["id"]
	}
}

// And no two entries share an id, since the id is what a changed binding is
// remembered against: two of them would mean changing one key changed both.
func TestEveryBindingHasItsOwnName(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range bindings(t) {
		if seen[b["id"]] {
			t.Errorf("there are two bindings called %q", b["id"])
		}
		seen[b["id"]] = true
	}
}

// The keystroke is written the way chordOf reads one, or it is a binding
// that can never fire: the modifiers come in a fixed order, and Shift is
// only named for keys that do not print a character.
func TestEveryKeystrokeIsWrittenTheWayItIsRead(t *testing.T) {
	for _, b := range bindings(t) {
		parts := strings.Split(b["keys"], "+")
		// A lone "+" or "-" as the key leaves an empty last piece.
		if n := len(parts); n > 1 && parts[n-1] == "" {
			parts = append(parts[:n-2], "+")
		}
		key := parts[len(parts)-1]
		mods := parts[:len(parts)-1]

		order := map[string]int{"Mod": 1, "Alt": 2, "Shift": 3}
		last := 0
		for _, m := range mods {
			rank, ok := order[m]
			if !ok {
				t.Errorf("%q has %q where a modifier belongs, in %s",
					b["id"], m, b["keys"])
				continue
			}
			if rank <= last {
				t.Errorf("%q writes its modifiers out of order: %s", b["id"], b["keys"])
			}
			last = rank
		}
		// Shift on a key that prints a character would never match, because
		// the character is what arrives.
		for _, m := range mods {
			if m == "Shift" && len([]rune(key)) == 1 {
				t.Errorf("%q is written %s; a printed character carries its "+
					"own shift, so this can never fire", b["id"], b["keys"])
			}
		}
		if key == " " {
			t.Errorf("%q uses a literal space; the key is written Space", b["id"])
		}
	}
}

// A binding that dispatches a menu action has to name one that exists —
// the same check the menu itself gets, from the other side.
func TestEveryBindingsActionExists(t *testing.T) {
	js := readUI(t, "app.js")
	start := strings.Index(js, "const KEY_BINDINGS = [")
	end := strings.Index(js[start:], "\n];")
	body := js[start : start+end]

	// What the action dispatcher answers to.
	handled := map[string]bool{}
	for _, m := range regexp.MustCompile(
		`case "([a-z0-9-]+)":`).FindAllStringSubmatch(js, -1) {
		handled[m[1]] = true
	}
	for _, m := range regexp.MustCompile(
		`action\("([a-z0-9-]+)"\)`).FindAllStringSubmatch(body, -1) {
		if !handled[m[1]] {
			t.Errorf("a key runs the action %q, which nothing handles", m[1])
		}
	}
}

// The shortcuts window is reachable, and resets are wired: a window nobody
// can open documents nothing.
func TestTheShortcutsWindowIsReachable(t *testing.T) {
	html := readUI(t, "index.html")
	js := readUI(t, "app.js")
	for _, want := range []string{
		`data-action="shortcuts"`, `id="shortcuts"`,
		`id="shortcuts-list"`, `id="shortcuts-reset"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html is missing %s", want)
		}
	}
	for _, want := range []string{
		"shortcutsDialog", "boundKeys", "chordOf", "keyFor", "saveKeys", "resetKeys",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js is missing %s", want)
		}
	}
}
