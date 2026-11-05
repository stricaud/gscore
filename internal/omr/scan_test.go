/*
 * scan_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package omr

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"testing"
)

// TestRealScan runs the finder over an actual page and prints what it found.
//
// It is skipped unless GSCORE_OMR_SCAN names an image file, because the
// scans this was developed against are hundreds of megabytes of somebody
// else's copy of a library book and have no business in the repository. The
// synthetic tests are the ones that must pass on every machine; this one is
// how you check the synthetic ones were not lying.
//
//	GSCORE_OMR_SCAN=/tmp/page.png go test ./internal/omr/ -run RealScan -v
func TestRealScan(t *testing.T) {
	path := os.Getenv("GSCORE_OMR_SCAN")
	if path == "" {
		t.Skip("set GSCORE_OMR_SCAN to an image of a page")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	src, format, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	g := FromImage(src)
	t.Logf("%s: %s, %d×%d", path, format, g.W, g.H)

	// The window has to be wider than a notehead and narrower than the
	// lighting drift, and we do not know the interline yet. Thirteen times a
	// first guess of a hundredth of the page height lands in that range for
	// anything between a postcard and a folio.
	guess := g.H / 100
	if guess < 4 {
		guess = 4
	}
	b := g.Binarise(13*guess, 20)
	staves, inter := FindStaves(b)
	t.Logf("interline %.2f px, %d staves", inter, len(staves))
	for i, s := range staves {
		t.Log(fmt.Sprintf("  staff %2d: bottom line y=%7.1f to %7.1f (tilt %+.1f px)",
			i+1, s.Lines[4].Y(0), s.Lines[4].Y(float64(g.W-1)),
			s.Lines[4].Y(float64(g.W-1))-s.Lines[4].Y(0)))
	}
	if len(staves) == 0 {
		t.Error("no staves found")
	}
	if inter < 10 {
		t.Logf("NOTE: %.1f px of interline is below what any reader can use;"+
			" expect the notes to be unreadable even where the staves are not", inter)
	}
}
