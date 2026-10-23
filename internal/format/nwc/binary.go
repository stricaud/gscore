/*
 * binary.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package nwc

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// Reading the binary .nwc container.
//
// The layout follows the posa description in fileformat.posa/nwc.posa, which
// was written against Scriptorium files and verified on versions 1.70, 1.75,
// 2.00 and 2.75:
//
//	"[NWZ]" 00 <zlib stream>
//
// and inflating that gives a score beginning "[NoteWorthy ArtWare]".
//
// From version 2.75 the inflated body is no longer a binary token stream: it
// is NWCTXT, the same text the .nwctxt files carry. That is what makes modern
// .nwc files readable here — the container is unwrapped and the text inside is
// handed to the parser that already exists.
//
// Earlier versions keep the music in a binary token stream whose walk is
// version-specific and undescribed, so those are reported rather than guessed.

const (
	nwzMagic      = "[NWZ]"
	vendorMarker  = "[NoteWorthy "
	nwctxtMarker  = "!NoteWorthy"
	containerSkip = 6 // the magic plus one filler byte
)

// inflate unwraps the [NWZ] container, or passes through an already-inflated
// score, and reports the version it declares.
func inflate(data []byte) (raw []byte, major, minor int, err error) {
	switch {
	case bytes.HasPrefix(data, []byte(nwzMagic)):
		if len(data) <= containerSkip {
			return nil, 0, 0, fmt.Errorf("nwc: the container is truncated")
		}
		zr, zerr := zlib.NewReader(bytes.NewReader(data[containerSkip:]))
		if zerr != nil {
			return nil, 0, 0, fmt.Errorf("nwc: cannot inflate the container: %w", zerr)
		}
		defer zr.Close()
		raw, err = io.ReadAll(zr)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("nwc: cannot inflate the container: %w", err)
		}
	case bytes.HasPrefix(data, []byte(vendorMarker)):
		// Some tools leave an inflated score on disk.
		raw = data
	default:
		return nil, 0, 0, fmt.Errorf("nwc: not a NoteWorthy file")
	}

	r := &reader{b: raw}
	if _, err := r.cstring(); err != nil { // vendor, "[NoteWorthy ArtWare]"
		return nil, 0, 0, err
	}
	if err := r.skip(2); err != nil {
		return nil, 0, 0, err
	}
	if _, err := r.cstring(); err != nil { // product, "[NoteWorthy Composer]"
		return nil, 0, 0, err
	}
	// The version is stored minor first.
	lo, err := r.byteAt()
	if err != nil {
		return nil, 0, 0, err
	}
	hi, err := r.byteAt()
	if err != nil {
		return nil, 0, 0, err
	}
	return raw, int(hi), int(lo), nil
}

// findNWCTXT locates the text body a 2.75 or later score carries.
//
// The remaining header fields are counted rather than skipped by a fixed
// length, because the padding differs between builds; the marker cannot occur
// before the text begins.
func findNWCTXT(raw []byte) ([]byte, bool) {
	if i := bytes.Index(raw, []byte(nwctxtMarker)); i >= 0 {
		return raw[i:], true
	}
	return nil, false
}

// reader walks the header's NUL-terminated strings and bytes.
type reader struct {
	b []byte
	i int
}

func (r *reader) cstring() (string, error) {
	j := bytes.IndexByte(r.b[r.i:], 0)
	if j < 0 {
		return "", fmt.Errorf("nwc: the header ends inside a string")
	}
	s := string(r.b[r.i : r.i+j])
	r.i += j + 1
	return s, nil
}

func (r *reader) byteAt() (byte, error) {
	if r.i >= len(r.b) {
		return 0, fmt.Errorf("nwc: the header is truncated")
	}
	c := r.b[r.i]
	r.i++
	return c, nil
}

func (r *reader) skip(n int) error {
	if r.i+n > len(r.b) {
		return fmt.Errorf("nwc: the header is truncated")
	}
	r.i += n
	return nil
}
