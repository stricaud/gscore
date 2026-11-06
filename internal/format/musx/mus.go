/*
 * mus.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// This file and the two beside it — musrecords.go and musdoc.go — read
// Finale's *older* format, the binary .mus that it wrote until 2012. It is a
// different container from the .musx in container.go and shares not one byte
// of framing with it: no zip, no XML, no stream cipher. What the two do share
// is the document inside, which is why build() in enigma.go is where they
// meet and the only place they do.
//
// Keeping them apart matters more than the code they save. A .mus is not a
// .musx with a shorter name — it is two formats under one extension, since
// Finale changed how it compressed the payload in 2011 — and a reader that
// blurred them would answer a question about one with a fact about the other.
// So the importer, the errors and the report are this format's own, and
// nothing here is tried on a .musx or the other way round.
//
//	2001, 2004, 2005   four PKWARE DCL streams from 0x200, each labelled
//	                   with the pool it holds; records are 16-byte rows
//	                   carrying ETF's two-character tags
//	2011, 2012         a chain of zlib streams from about 0x216, unlabelled;
//	                   records are self-identifying and variable-length
//
// Both store the music itself identically, as a flat array of 38-byte entry
// slots, which musrecords.go reads once for both.
//
// What this knows of the format is not Finale's doing: none of it was ever
// published beyond one preliminary document from 1996, and the rest was
// worked out from files by the finale-file-parser project, whose notes are
// cited in docs/finale-mus.md. Finale was discontinued in 2024, so "open it
// and save it as something else" stopped being advice anybody could take.

package musx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(musImporter{})
}

// musImporter is deliberately not the importer in musx.go.
//
// One type reading both would have to carry which format it was reading in a
// field, and every message it wrote would have to remember to look. Two
// types cannot get it wrong.
type musImporter struct{}

func (musImporter) Name() string         { return "Finale, 2012 and earlier" }
func (musImporter) Extensions() []string { return []string{".mus"} }

func (musImporter) Import(r io.Reader) (*model.Score, error) { return ImportMus(r) }

// MusPartialError is a .mus read with things left out.
//
// Its own type, not the one .musx uses, so that whatever shows it to somebody
// can say which format the file was. The two formats lose different things.
type MusPartialError struct {
	Skipped []string
}

func (e *MusPartialError) Error() string {
	if len(e.Skipped) == 1 {
		return "one kind of thing in this Finale file was not read"
	}
	return fmt.Sprintf("%d kinds of thing in this Finale file were not read",
		len(e.Skipped))
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *MusPartialError) Partial() {}

// Details implements format.Partial.
func (e *MusPartialError) Details() []string { return e.Skipped }

const (
	// musMagic is what the first bytes of the plaintext header say. It is
	// used to tell somebody what they have rather than to decide whether to
	// read it: the check that decides is the container walk, which is the
	// one that has been run against real files.
	musMagic = "ENIGMA BINARY FILE"

	// The version banner: a fixed-size field holding something like
	// "Finale(R) 2005 for Macintosh".
	musBannerAt   = 0x20
	musBannerSize = 0x40

	// Where the document's own words sit, when it has any. Two files from
	// one collection are identical up to here and differ at the first
	// letter of the title.
	musTitleAt = 0xD8

	// Where the pool container starts, in both eras.
	musContainerAt = 0x200

	// The last year that wrote PKWARE DCL. After it, zlib.
	musLastDCLYear = 2005

	// musMaxPayload is every pool of one document added up. A Finale score
	// decompresses to at most about 700 KiB; this is generous by two orders
	// of magnitude and exists only to stop a malformed file asking for
	// every byte of memory there is.
	musMaxPayload = 64 << 20
)

// musBannerYear finds the year in the version banner.
var musBannerYear = regexp.MustCompile(`^Finale\(R\)\s+(\d{4})\b`)

// ImportMus reads a Finale .mus score.
//
// Like every importer here it would rather hand back the music and a list of
// what it could not read than refuse the file: somebody with a .mus and no
// Finale cannot go and fetch a better copy.
func ImportMus(r io.Reader) (*model.Score, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("mus: %w", err)
	}
	score, skipped, err := readMus(raw)
	if err != nil {
		return nil, err
	}
	if len(skipped) > 0 {
		return score, &MusPartialError{Skipped: skipped}
	}
	return score, nil
}

// readMus is ImportMus with the bytes already in hand, so a test can call it.
func readMus(raw []byte) (*model.Score, []string, error) {
	if len(raw) < musContainerAt {
		return nil, nil, fmt.Errorf(
			"mus: the file is %d bytes, too short to be a Finale score at all",
			len(raw))
	}

	pools, order, era, err := musPools(raw)
	if err != nil {
		return nil, nil, err
	}

	doc, census, err := musDocument(pools, order, era)
	if err != nil {
		return nil, nil, err
	}
	return build(doc, musMetadata(raw), census)
}

// musEra is which of the two formats under this extension a file is.
type musEra int

const (
	// eraDCL is 2001 to 2005: four labelled PKWARE DCL streams.
	eraDCL musEra = iota
	// eraZlib is 2011 and 2012: a chain of unlabelled zlib streams.
	eraZlib
)

func (e musEra) String() string {
	if e == eraDCL {
		return "2001 to 2005"
	}
	return "2011 or 2012"
}

// musYear reads the year out of the version banner, or 0 if it cannot.
//
// The banner field is not cleared when Finale rewrites it, so a shorter
// banner can leave the tail of a longer one behind it. Everything from the
// first zero byte on is therefore thrown away.
func musYear(raw []byte) int {
	if len(raw) < musBannerAt+musBannerSize {
		return 0
	}
	field := raw[musBannerAt : musBannerAt+musBannerSize]
	if i := bytes.IndexByte(field, 0); i >= 0 {
		field = field[:i]
	}
	m := musBannerYear.FindSubmatch(field)
	if m == nil {
		return 0
	}
	year := 0
	for _, c := range m[1] {
		year = year*10 + int(c-'0')
	}
	return year
}

// musPools decompresses every pool of the document.
//
// The year in the banner chooses which scheme to try first and nothing more:
// an unreadable banner, or a banner from a year this does not know, is not a
// reason to refuse a file whose payload decodes perfectly well. So both are
// tried, best guess first, and only a file that neither can read is an error.
func musPools(raw []byte) ([]musPool, binary.ByteOrder, musEra, error) {
	type scheme struct {
		era  musEra
		read func([]byte) ([]musPool, binary.ByteOrder, error)
	}
	dcl := scheme{eraDCL, musDCLPools}
	zip := scheme{eraZlib, musZlibPools}

	order := []scheme{zip, dcl}
	if y := musYear(raw); y != 0 && y <= musLastDCLYear {
		order = []scheme{dcl, zip}
	}

	var why []string
	for _, s := range order {
		pools, byteOrder, err := s.read(raw)
		if err == nil {
			return pools, byteOrder, s.era, nil
		}
		why = append(why, fmt.Sprintf("as a %s file, %v", s.era, err))
	}

	// Both failed. What to say depends on whether this looks like a Finale
	// file at all, because the two cases send somebody somewhere different.
	if !bytes.Contains(raw[:min(len(raw), musContainerAt)], []byte(musMagic)) {
		return nil, nil, 0, fmt.Errorf(
			"mus: this is not a Finale .mus file — it does not say %q anywhere in its header",
			musMagic)
	}
	return nil, nil, 0, fmt.Errorf(
		"mus: this is a Finale file, but not one gscore can follow: %s",
		joinSemi(why))
}

func joinSemi(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += "; "
		}
		out += v
	}
	return out
}

// musMetadata reads what the document says about itself.
//
// This is the least certain thing in the file and the least costly to get
// wrong: a title read as empty is a score with no title on it, where a bar
// read wrong is a score that is not the music. The block is fixed in size
// and usually all zero — plenty of Finale documents simply carry no title —
// so anything that is not plain printable text ends the reading rather than
// being guessed at.
func musMetadata(raw []byte) metadata {
	var meta metadata
	if len(raw) <= musTitleAt {
		return meta
	}
	block := raw[musTitleAt:min(len(raw), musContainerAt)]

	into := []*string{&meta.Title, &meta.Composer, &meta.Copyright}
	for _, field := range into {
		i := bytes.IndexByte(block, 0)
		if i < 0 {
			break
		}
		text := string(block[:i])
		if !printableASCII(text) {
			break
		}
		*field = text
		block = block[i+1:]
	}
	return meta
}

func printableASCII(s string) bool {
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}
