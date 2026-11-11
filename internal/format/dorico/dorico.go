/*
 * dorico.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package dorico reads Dorico projects.
//
// Steinberg documents none of this. A .dorico file is a zip — an OASIS
// container, the same arrangement EPUB uses — and inside it score.dtn holds
// the music as a binary tree. The container is open and legible; the tree
// is not, and what is here was worked out from the files themselves. See
// dtn.go for the shape of it.
//
// What that tree holds is Dorico's own model rather than a notated page:
// flows, players, instruments, and per-voice streams of events at exact
// rational positions measured in crotchets. That is a good deal closer to
// what gscore keeps than most formats are — no divisions, no ties to
// reassemble, no backup elements — and the reading is correspondingly
// direct. What it does not hold is how any of it looks: beams, bar numbers
// and casting-off are worked out by Dorico when it draws, and are worked
// out here by the engraver for the same reason.
//
// This reads the music: the movements, who is playing, the notes and rests,
// the time and key signatures, and the clefs. It does not yet read the
// marks on them — slurs, dynamics, tuplets, grace notes, trills, repeats —
// and it says so rather than passing over them in silence, so that nobody
// finds out by looking for a crescendo that is not there.
package dorico

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string { return "Dorico project" }

// Extensions are what the files are called.
//
// The zipped form is offered too, because that is how the larger projects
// are published — a .dorico is already a zip, and zipping it again is what
// a web server that mangles unknown types forces on people.
func (importer) Extensions() []string { return []string{".dorico"} }

func (importer) Import(r io.Reader) (*model.Score, error) {
	scores, err := Import(r)
	if len(scores) == 0 {
		return nil, err
	}
	return scores[0], err
}

func (importer) ImportAll(r io.Reader) ([]*model.Score, error) { return Import(r) }

// PartialError reports what a project held that gscore did not read.
//
// The music is returned with it. A Dorico project is full of things gscore
// has no place for yet, and a reader that refused the notes over a missing
// slur would be no use to anybody.
type PartialError struct {
	Skipped []string
}

// Error says how much was left behind, in kinds rather than in count.
//
// "17 things were not read, the first being: dynamics (3549 of them)" was
// the first try, and somebody reading it asked whether that meant
// seventeen or three and a half thousand. It meant seventeen kinds, of
// which the largest was three and a half thousand dynamics, and a sentence
// that has to be asked about has not said anything. "First" was wrong as
// well: the list is ordered by how many there are of each, so the one
// named is the commonest and not the earliest.
func (e *PartialError) Error() string {
	if len(e.Skipped) == 1 {
		return "dorico: one kind of thing was not read: " + e.Skipped[0]
	}
	return fmt.Sprintf("dorico: %d kinds of thing were not read, the "+
		"commonest being %s", len(e.Skipped), e.Skipped[0])
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details implements format.Partial.
func (e *PartialError) Details() []string { return e.Skipped }

// Import reads a Dorico project: one score for each of its flows.
//
// A flow is a movement, or a song in a collection, or a single piece in a
// project that holds only one. gscore's book is the same idea, so the four
// movements of a symphony arrive as four scores rather than as one very
// long one with double barlines in it.
func Import(r io.Reader) ([]*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("dorico: reading the project: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("dorico: this is not a Dorico project — the " +
			"file is not a container gscore can open")
	}

	member := rootFile(zr)
	raw, err := readMember(zr, member)
	if err != nil {
		return nil, err
	}
	t, err := decodeDTN(raw)
	if err != nil {
		return nil, fmt.Errorf("dorico: %w", err)
	}

	info := readScoreInfo(zr)
	return build(t, info)
}

// rootFile asks the container which member holds the score.
//
// It is nearly always "score.dtn" and is asked for anyway: the container
// says, the name is not fixed by anything, and reading the manifest is
// three lines.
func rootFile(zr *zip.Reader) string {
	raw, err := readMember(zr, "META-INF/container.xml")
	if err != nil {
		return "score.dtn"
	}
	var doc struct {
		Rootfiles []struct {
			Path string `xml:"full-path,attr"`
			Type string `xml:"media-type,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return "score.dtn"
	}
	for _, f := range doc.Rootfiles {
		if strings.HasSuffix(f.Path, ".dtn") {
			return f.Path
		}
	}
	return "score.dtn"
}

func readMember(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("dorico: opening %s: %w", name, err)
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("dorico: this project has no %s in it", name)
}

// projectInfo is what the container says about the project, as opposed to
// what the score says about itself.
//
// Dorico writes it as plain XML beside the score so that a file browser can
// show the title of a project without understanding the format. gscore
// reads it for the same reason: it is the one part of a project that needs
// no reverse engineering at all.
type projectInfo struct {
	Title    string `xml:"title"`
	Subtitle string `xml:"subtitle"`
	Composer string `xml:"composer"`
	Arranger string `xml:"arranger"`
	Lyricist string `xml:"lyricist"`
	Other    string `xml:"otherInfo"`
}

func readScoreInfo(zr *zip.Reader) projectInfo {
	var info projectInfo
	raw, err := readMember(zr, "supplementary_data/scoreinfo/scoreinfo.xml")
	if err != nil {
		return info
	}
	_ = xml.Unmarshal(raw, &info)
	return info
}

// sortedStrings is used wherever a report is gathered out of a map, so that
// the same project always reports the same way round.
func sortedStrings(in map[string]int) []string {
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
