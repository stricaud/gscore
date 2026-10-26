/*
 * builds_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package format_test

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

/*
 * Every build of gscore reads the same files.
 *
 * An importer joins the registry by being imported for its side effect, so
 * what a build can open is decided by a list of blank imports — and there
 * are three of those, one per way gscore is built. They have to say the same
 * thing and nothing made them.
 *
 * Both of the copies drifted. The Finale importer was added to the command
 * line build and to neither of the others, so gscore on the desktop opened a
 * .musx, gscore on the web said it did not know the format, and the macOS
 * application put up "nothing here reads .musx files" about a file the same
 * program had just opened from a terminal. Reported as "I don't understand:
 * go run . shows the score, the installed app does nothing".
 *
 * None of that is visible from inside a build. There is no error, no gap in
 * a table, nothing to notice until somebody opens a file in the one place it
 * does not work. The only way to see it is to put the lists side by side.
 */

// theBuilds is every place gscore decides what it can read, by the path of
// the file that holds the list.
var theBuilds = map[string]string{
	"the command line and desktop build": "../../main.go",
	"the web build":                      "../../web/main_wasm.go",
	"the macOS application":              "../../ui/macosx/bridge/bridge.go",
}

func TestEveryBuildReadsTheSameFiles(t *testing.T) {
	lists := map[string][]string{}
	for name, path := range theBuilds {
		lists[name] = formatImports(t, path)
		if len(lists[name]) == 0 {
			t.Fatalf("%s (%s) registers no formats at all", name, path)
		}
	}

	// Everything any build has, every build has.
	all := map[string]bool{}
	for _, formats := range lists {
		for _, f := range formats {
			all[f] = true
		}
	}
	var every []string
	for f := range all {
		every = append(every, f)
	}
	sort.Strings(every)

	for name, formats := range lists {
		has := map[string]bool{}
		for _, f := range formats {
			has[f] = true
		}
		var missing []string
		for _, f := range every {
			if !has[f] {
				missing = append(missing, f)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s (%s) does not register %v, so it cannot open those "+
				"files while the other builds can;\n\tit has %v, and between "+
				"them the builds have %v",
				name, theBuilds[name], missing, formats, every)
		}
	}
}

// And the lists hold the formats gscore is for, which is what stops the test
// above from passing because every build registers nothing.
func TestTheBuildsRegisterTheFormatsThatMatter(t *testing.T) {
	for name, path := range theBuilds {
		have := strings.Join(formatImports(t, path), " ")
		for _, want := range []string{"abc", "gscore", "musicxml", "musx", "nwc"} {
			if !strings.Contains(have, want) {
				t.Errorf("%s does not register %s (it has %s)", name, want, have)
			}
		}
	}
}

// formatImports is the formats a build registers, by package name.
func formatImports(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v", err)
	}
	re := regexp.MustCompile(`_ "github\.com/stricaud/gscore/internal/format/(\w+)"`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}
