/*
 * text.go
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
	"strings"
	"unicode/utf8"
)

// Reading the words out of a file written before anybody agreed on encodings.
//
// NoteWorthy Composer 1.7x predates UTF-8 being the obvious answer, so a
// staff called "Flûte" holds one byte for the û, in whatever the machine that
// typed it was using — almost always Windows-1252. Read as UTF-8 that byte is
// invalid and the name arrives as a replacement character.
//
// The same function has a second job, which is the one that matters more.
// A name full of control bytes is not a name in some other encoding: it is
// what you get when the walk through the file has lost its place and is
// reading note data as though it were text. Telling those apart is what lets
// the reader stop at the point it went wrong rather than appending staves
// made of wreckage.

// decodeText turns bytes from a file into a string.
//
// Valid UTF-8 is taken as it is, because later versions of the format do use
// it. Anything else is read as Windows-1252, which is what wrote these files
// and which, unlike UTF-8, cannot fail — every byte means something.
func decodeText(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if r, ok := cp1252[c]; ok {
			sb.WriteRune(r)
			continue
		}
		// Latin-1 proper: the byte is the code point.
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

// cp1252 is the part of Windows-1252 that differs from Latin-1: the range
// 0x80–0x9F, where Latin-1 has unused control codes and Windows put the
// punctuation people actually type.
var cp1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„',
	0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ',
	0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ',
	0x8E: 'Ž', 0x91: '‘', 0x92: '’', 0x93: '“',
	0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
	0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›',
	0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}

// looksLikeText reports whether a string is something a person typed.
//
// The test is control characters, not unusual letters: a name may be in any
// language and any script, but nobody has ever called a staff by a backspace.
// Their presence means the reader is looking at note data, not at a name.
func looksLikeText(s string) bool {
	if s == "" {
		return true // an unnamed staff is perfectly ordinary
	}
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x20 && r != '\t' {
			return false
		}
		if r == 0x7F || (r >= 0x80 && r <= 0x9F) {
			return false
		}
	}
	return true
}
