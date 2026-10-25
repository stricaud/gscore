/*
 * golden_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"

	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
)

/*
 * What every score in the tree currently engraves to, frozen.
 *
 * The engraver is the part of gscore with the most behaviour and the least
 * of it stated anywhere: no test says where the third notehead of a bar
 * belongs, because nothing sensible could. What can be said is that it
 * should not move without somebody meaning it to.
 *
 * So every score the repository has — the samples, and the 182 files of the
 * MusicXML suite — is engraved and the drawing is reduced to a line of text
 * per mark on the page. The hash of that goes in testdata/layouts.txt. A
 * change that was meant to alter one thing and alters two hundred says so
 * here, before it is discovered on somebody's score.
 *
 * This is not a claim that any of it is *right*. It is a record of what it
 * is, which is the thing that makes a change in the engraver safe to make.
 *
 *     go test ./internal/engrave -run Golden -update
 *
 * rewrites the record, and the diff is then the change, to be read before it
 * is committed.
 */

var update = flag.Bool("update", false,
	"rewrite testdata/layouts.txt from what the engraver does now")

const goldenFile = "testdata/layouts.txt"

// corpus is every score in the repository, by a name that does not move.
func corpus(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	groups := []struct{ label, glob string }{
		{"samples", "../../samples/*.abc"},
		{"abc", "../format/abc/testdata/*.abc"},
		{"suite", "../format/musicxml/testdata/suite/*.musicxml"},
	}
	for _, g := range groups {
		files, err := filepath.Glob(g.glob)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			out[g.label+"/"+filepath.Base(f)] = f
		}
	}
	if len(out) < 150 {
		t.Fatalf("only %d scores found: the corpus is not where it was", len(out))
	}
	return out
}

func load(path string) (*model.Score, error) {
	imp, err := format.ImporterFor(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s, err := imp.Import(f)
	if s == nil {
		return nil, err
	}
	return s, nil // a partial read still engraves, and still has to stay put
}

// describe reduces a drawing to text: one line per mark, in the order the
// engraver put them down, with the numbers rounded to a thousandth of a
// staff space so that a rebuild on another machine says the same thing.
func describe(lay *engrave.Layout) string {
	var b strings.Builder
	f := func(v float64) string { return fmt.Sprintf("%.3f", v) }

	fmt.Fprintf(&b, "size %s %s\n", f(lay.Width), f(lay.Height))
	for _, g := range lay.Glyphs {
		fmt.Fprintf(&b, "glyph %s %s %s %s\n", g.Name, f(g.X), f(g.Y), f(g.Size))
	}
	for _, l := range lay.Lines {
		fmt.Fprintf(&b, "line %s %s %s %s %s\n",
			f(l.X1), f(l.Y1), f(l.X2), f(l.Y2), f(l.Thickness))
	}
	for _, t := range lay.Texts {
		fmt.Fprintf(&b, "text %q %s %s %s %s\n", t.S, f(t.X), f(t.Y), f(t.Size), t.Anchor)
	}
	for _, c := range lay.Curves {
		fmt.Fprintf(&b, "curve %s %s %s %s %s %s\n",
			f(c.X1), f(c.Y1), f(c.CX), f(c.CY), f(c.X2), f(c.Y2))
	}
	for _, c := range lay.Circles {
		fmt.Fprintf(&b, "circle %s %s %s\n", f(c.X), f(c.Y), f(c.Radius))
	}
	for _, q := range lay.Quads {
		fmt.Fprintf(&b, "quad %s %s %s %s %s %s %s %s\n",
			f(q.X[0]), f(q.Y[0]), f(q.X[1]), f(q.Y[1]),
			f(q.X[2]), f(q.Y[2]), f(q.X[3]), f(q.Y[3]))
	}
	return b.String()
}

// marks is how many things are on the page, kept beside the hash so that a
// failure says whether the drawing changed a little or is simply gone.
func marks(lay *engrave.Layout) int {
	return len(lay.Glyphs) + len(lay.Lines) + len(lay.Texts) +
		len(lay.Curves) + len(lay.Circles) + len(lay.Quads)
}

func engraveAll(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for name, path := range corpus(t) {
		s, err := load(path)
		if err != nil && s == nil {
			out[name] = "unreadable"
			continue
		}
		lay := engrave.Engrave(s, engrave.DefaultOptions())
		sum := sha256.Sum256([]byte(describe(lay)))
		out[name] = fmt.Sprintf("%s %d", hex.EncodeToString(sum[:8]), marks(lay))
	}
	return out
}

func TestGoldenLayouts(t *testing.T) {
	got := engraveAll(t)

	if *update {
		var b strings.Builder
		b.WriteString("# What every score in the tree engraves to: a hash of the\n" +
			"# drawing, and how many marks are on the page. Written by\n" +
			"#     go test ./internal/engrave -run Golden -update\n" +
			"# A diff here is a change in the engraver. Read it before committing.\n")
		names := make([]string, 0, len(got))
		for n := range got {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(&b, "%s %s\n", n, got[n])
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s for %d scores", goldenFile, len(got))
		return
	}

	want := map[string]string{}
	raw, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("%v\nrun: go test ./internal/engrave -run Golden -update", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 3 {
			t.Fatalf("%s: cannot read %q", goldenFile, line)
		}
		want[parts[0]] = parts[1] + " " + parts[2]
	}

	var changed, added, gone []string
	for n, g := range got {
		w, ok := want[n]
		switch {
		case !ok:
			added = append(added, n)
		case w != g:
			changed = append(changed, fmt.Sprintf("%s: was %s, now %s", n, w, g))
		}
	}
	for n := range want {
		if _, ok := got[n]; !ok {
			gone = append(gone, n)
		}
	}
	sort.Strings(changed)
	sort.Strings(added)
	sort.Strings(gone)

	if len(changed) > 0 {
		t.Errorf("%d of %d scores engrave differently than they did:",
			len(changed), len(got))
		for _, c := range changed[:min(len(changed), 25)] {
			t.Errorf("   %s", c)
		}
		if len(changed) > 25 {
			t.Errorf("   … and %d more", len(changed)-25)
		}
		t.Errorf("if that was the point, rerun with -update and read the diff")
	}
	for _, n := range added {
		t.Errorf("%s is new; rerun with -update to record it", n)
	}
	for _, n := range gone {
		t.Errorf("%s is in the record but not in the tree any more", n)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
