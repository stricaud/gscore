/*
 * container.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musx

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
)

/*
 * Getting at the music inside a Finale file.
 *
 * A .musx is a zip archive, and the archive is the easy part:
 *
 *     mimetype                 application/vnd.makemusic.notation
 *     META-INF/container.xml   says which member holds the score
 *     NotationMetadata.xml     the title, the composer, what wrote it
 *     score.dat                the music
 *     presets/…                playback settings
 *
 * score.dat is the music, obfuscated and then gzipped — in that order, so it
 * has to be unobfuscated before it looks like anything at all. What comes out
 * is EnigmaXML, which is Finale's own document model written as XML and is
 * what enigma.go reads.
 *
 * The obfuscation is a stream cipher with no key: a 32-bit linear
 * congruential generator from a fixed seed, one byte of it XORed over each
 * byte of the file, restarted every 128 KiB. It protects nothing and is not
 * meant to — it is a lock on a door that is already open — but it does mean
 * that a Finale file cannot be read without knowing the four numbers below.
 *
 * Those numbers are not ours. They were worked out by the people who wrote
 * Denigma (Robert Patterson) and are used here as they appear in musx2mxl,
 * which is MIT licensed; the structure of what comes out is documented by
 * Project Attacca at github.com/Project-Attacca/enigmaxml-documentation.
 * Nothing here is derived from Finale itself.
 */

const (
	// cipherSeed is where the generator starts, and where it starts again.
	cipherSeed = 0x28006D45
	// cipherMultiplier and cipherIncrement are the generator's own two
	// constants: state = state*a + c, as every such generator is written.
	cipherMultiplier = 0x41C64E6D
	cipherIncrement  = 0x3039
	// cipherReset is how far it runs before going back to the seed. Without
	// this a file over 128 KiB comes out as music for a while and then as
	// rubbish, which is a far harder thing to find than a file that fails
	// at the first byte.
	cipherReset = 0x20000
)

// decipher unwraps score.dat, in place.
//
// Its own inverse: XOR twice with the same stream and you have what you
// started with, which is what the round-trip test relies on.
func decipher(b []byte) {
	state := uint32(cipherSeed)
	for i := range b {
		if i%cipherReset == 0 {
			state = cipherSeed
		}
		state = state*cipherMultiplier + cipherIncrement
		upper := uint32(state >> 16)
		b[i] ^= byte(upper + upper/255)
	}
}

// container is what META-INF/container.xml says: which member of the archive
// holds the score. Read rather than assumed, because the name is the
// archive's to choose and a reader that hard-codes "score.dat" is one Finale
// version away from being wrong.
type container struct {
	RootFiles []struct {
		Path string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

// metadata is NotationMetadata.xml, which carries what the file is called.
//
// The title and the composer are here rather than in the music, so a reader
// that only opened score.dat would show a piece with no name on it.
type metadata struct {
	Title       string `xml:"fileInfo>title"`
	Composer    string `xml:"fileInfo>composer"`
	Copyright   string `xml:"fileInfo>copyright"`
	Subtitle    string `xml:"fileInfo>subtitle"`
	Application string `xml:"fileInfo>created>application"`
}

// open reads a .musx and hands back its EnigmaXML and what it says about
// itself.
func open(r io.Reader) ([]byte, metadata, error) {
	var meta metadata

	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, meta, fmt.Errorf("musx: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, meta, fmt.Errorf("musx: not a Finale file: %w", err)
	}

	members := map[string]*zip.File{}
	for _, f := range zr.File {
		members[f.Name] = f
	}

	// Which member is the score.
	path := "score.dat"
	if f := members["META-INF/container.xml"]; f != nil {
		if b, err := member(f); err == nil {
			var c container
			if xml.Unmarshal(b, &c) == nil && len(c.RootFiles) > 0 && c.RootFiles[0].Path != "" {
				path = c.RootFiles[0].Path
			}
		}
	}
	// What it is called. A file with no metadata is still a file with music
	// in it, so this never fails the read.
	if f := members["NotationMetadata.xml"]; f != nil {
		if b, err := member(f); err == nil {
			_ = xml.Unmarshal(b, &meta)
		}
	}

	f := members[path]
	if f == nil {
		return nil, meta, fmt.Errorf("musx: the archive has no %s in it", path)
	}
	data, err := member(f)
	if err != nil {
		return nil, meta, fmt.Errorf("musx: %w", err)
	}

	decipher(data)

	zg, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		// The obfuscation is the only thing between the archive and the
		// gzip, so this is where a change in it would show.
		return nil, meta, fmt.Errorf(
			"musx: %s did not unwrap into a score — this may be a Finale version gscore has not met: %w",
			path, err)
	}
	defer zg.Close()
	out, err := io.ReadAll(zg)
	if err != nil {
		return nil, meta, fmt.Errorf("musx: %s is damaged: %w", path, err)
	}
	return out, meta, nil
}

// member reads one file out of the archive.
func member(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	// Finale's own files run to a few megabytes once unwrapped; the cap is
	// against an archive that claims to be one and is a decompression bomb.
	return io.ReadAll(io.LimitReader(rc, 256<<20))
}
