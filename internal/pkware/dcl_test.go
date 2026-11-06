/*
 * dcl_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pkware

import (
	"bytes"
	"strings"
	"testing"
)

// The tables in dcl.go are a hundred magic numbers that nothing in the
// program can check. This is what checks them: the stream blast.c documents
// in its own header comment, and the string it says comes out.
//
// It is a small vector but not a weak one. Thirteen bytes from eight means
// the literals, the length table, the distance table and an overlapping copy
// all have to be right at once — "AIAIAIAIAIAIA" is a two-byte literal start
// and then one back-reference eleven bytes long reaching two bytes back,
// which is exactly the case a slice copy would get wrong.
func TestTheVectorBlastDocuments(t *testing.T) {
	in := []byte{0x00, 0x04, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f}

	out, err := Decompress(in, 1<<20)
	if err != nil {
		t.Fatalf("the documented vector did not decompress: %v", err)
	}
	if got, want := string(out), "AIAIAIAIAIAIA"; got != want {
		t.Errorf("decompressed to %q, want %q", got, want)
	}
}

// A literal-only stream, which is what a writer with nothing to gain from
// compressing produces: a flag bit of 0 and eight raw bits, per byte, then
// the end code. If this works, a test elsewhere can write one.
func TestAStreamOfNothingButLiterals(t *testing.T) {
	want := []byte("Enigma, and the notes inside it.")

	out, err := Decompress(Literals(want), 1<<20)
	if err != nil {
		t.Fatalf("Literals wrote something Decompress will not read: %v", err)
	}
	if !bytes.Equal(out, want) {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestLiteralsOfNothing(t *testing.T) {
	out, err := Decompress(Literals(nil), 1<<20)
	if err != nil {
		t.Fatalf("an empty stream did not decompress: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("an empty stream produced %d bytes", len(out))
	}
}

// Every byte value, so that the eight raw bits are read the right way round
// over the whole range rather than only where it happens not to matter.
func TestEveryByteValueSurvives(t *testing.T) {
	want := make([]byte, 256)
	for i := range want {
		want[i] = byte(i)
	}

	out, err := Decompress(Literals(want), 1<<20)
	if err != nil {
		t.Fatalf("decompressing all 256 byte values: %v", err)
	}
	if !bytes.Equal(out, want) {
		t.Error("the bytes did not come back as they went in")
	}
}

func TestWhatIsNotAStreamOfThisKind(t *testing.T) {
	for _, c := range []struct {
		name string
		in   []byte
		says string
	}{
		{"no header at all", nil, "two-byte header"},
		{"one byte", []byte{0}, "two-byte header"},
		{"literal flag of two", []byte{2, 4, 0}, "literal flag"},
		{"dictionary of three", []byte{0, 3, 0}, "dictionary size"},
		{"dictionary of seven", []byte{0, 7, 0}, "dictionary size"},
		// A plausible header and then nothing, which is what a truncated
		// file looks like. It must not come back as an empty success.
		{"a header and no bits", []byte{0, 4}, "ends in the middle"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decompress(c.in, 1<<20)
			if err == nil {
				t.Fatal("this was accepted, and should not have been")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the error says %q, which does not mention %q",
					err, c.says)
			}
		})
	}
}

// The limit is the only thing standing between a hostile file and all the
// memory there is, so it is worth a test of its own. Half a kilobyte of
// output against a limit of a hundred bytes must be refused, and refused
// rather than truncated.
func TestTheLimitIsEnforced(t *testing.T) {
	long := bytes.Repeat([]byte("x"), 500)

	out, err := Decompress(Literals(long), 100)
	if err == nil {
		t.Fatalf("500 bytes came through a 100-byte limit as %d bytes", len(out))
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("the error says %q, which does not mention the limit", err)
	}
	if out != nil {
		t.Error("a refused stream still returned output")
	}
}

func TestALimitOfNothingIsRefused(t *testing.T) {
	if _, err := Decompress(Literals([]byte("x")), 0); err == nil {
		t.Error("a limit of zero was accepted")
	}
}

// A reference reaching back before anything has been written is the shape of
// a stream decoded at the wrong offset, which is exactly what the container
// walk will do if it gets its arithmetic wrong. It has to be an error rather
// than a read of whatever is in memory.
func TestAReferenceBeforeTheStart(t *testing.T) {
	// Flag 1 (a copy), length symbol for three, then a distance. With
	// nothing written yet, any distance at all is too far.
	var w bitWriter
	w.add(1, 1)
	w.addCode(lengthCode, 0) // symbol 0 is a length of three
	w.addCode(distanceCode, 0)
	w.add(0, 4)
	in := append([]byte{0x00, 0x04}, w.bytes()...)

	_, err := Decompress(in, 1<<20)
	if err == nil {
		t.Fatal("a reference before the start of output was accepted")
	}
	if !strings.Contains(err.Error(), "before the start") {
		t.Errorf("the error says %q, which does not say what is wrong", err)
	}
}

func TestTheErrorsAreOneType(t *testing.T) {
	_, err := Decompress([]byte{9, 9}, 1<<20)
	if _, ok := err.(*Error); !ok {
		t.Errorf("got a %T, want a *pkware.Error", err)
	}
}
