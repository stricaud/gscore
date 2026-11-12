/*
 * corpus_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Every Guitar Pro file in a directory, read and engraved.
 *
 * The files are not in the repository. Point at wherever they are:
 *
 *     GP_CORPUS=~/tabs go test ./internal/format/guitarpro -run Corpus -v
 *
 * A file that cannot be read fails the test; a file read in part is listed
 * with what was left out, so that a dialect this reader has not met shows
 * up as a list of files rather than as a crash.
 */

package guitarpro

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
)

func TestTheWholeCorpusIsReadAndEngraved(t *testing.T) {
	root := os.Getenv("GP_CORPUS")
	if root == "" {
		t.Skip("set GP_CORPUS to a directory of .gp3, .gp4 and .gp5 files")
	}
	var files []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			switch strings.ToLower(filepath.Ext(path)) {
			case ".gp3", ".gp4", ".gp5":
				files = append(files, path)
			}
		}
		return nil
	})
	if len(files) == 0 {
		t.Fatalf("no Guitar Pro files under %s", root)
	}
	clean, partial, failed := 0, 0, 0
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		s, err := Import(f)
		f.Close()
		rel, _ := filepath.Rel(root, path)
		if s == nil {
			failed++
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if err != nil {
			partial++
			if p, ok := err.(format.Partial); ok {
				t.Logf("%s: %s", rel, strings.Join(p.Details(), "; "))
			} else {
				t.Logf("%s: %v", rel, err)
			}
		} else {
			clean++
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: engraving panicked: %v", rel, r)
				}
			}()
			opt := engrave.DefaultOptions()
			lay := engrave.Engrave(s, opt)
			if lay == nil {
				t.Errorf("%s: no layout", rel)
			}
		}()
		bars := 0
		if len(s.Parts) > 0 {
			bars = len(s.Parts[0].Measures)
		}
		// A bar whose voice does not add up to the meter is the surest
		// sign of a byte read in the wrong place.
		short := 0
		for _, p := range s.Parts {
			for i, m := range p.Measures {
				want := p.TimeAt(i).BarDuration()
				for _, v := range m.Voices {
					if got := v.Duration(); len(v.Elements) > 0 && got.Cmp(want) != 0 {
						short++
						if short <= 3 {
							t.Logf("%s: %s bar %d holds %s of %s", rel, p.Name, i+1, got, want)
						}
					}
				}
			}
		}
		t.Logf("%s: %d staves, %d bars, %d voices off the meter, %q", rel, len(s.Parts), bars, short, s.Identity.Title)
	}
	fmt.Printf("corpus: %d files — %d clean, %d read in part, %d failed\n", len(files), clean, partial, failed)
}
