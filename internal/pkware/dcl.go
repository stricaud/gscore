/*
 * dcl.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The knowledge of the format in this file — the three tables of code
 * lengths, the inverted canonical code assignment, and the length and
 * distance encodings — is Mark Adler's, from blast.c in zlib's
 * contrib/blast, under the zlib licence. This is an altered version: it is
 * Go, it builds the whole output rather than flushing a sliding window, and
 * its limits and errors are gscore's. See the LICENSE file at the root, which
 * carries the notice that licence asks to be carried.
 */

// Package pkware decompresses the PKWARE Data Compression Library format,
// the one its own documentation calls "implode".
//
// It is here for one reason: a Finale file from 2001 to 2005 stores its music
// in four of these streams, and nothing in Go's standard library reads them.
// It is not the implode of a zip archive, which is a different format with
// the same name, and it is not deflate; a file compressed this way carries no
// magic number, only a two-byte header that is almost all zero.
//
// The format is a bitstream read from the least significant bit up. Each step
// is a flag bit: a 0 introduces one literal byte, a 1 a back-reference into
// what has already been produced. Three fixed Huffman tables — for literals,
// for lengths and for distances — are built into the format rather than
// stored in the file, which is why two bytes of header are enough.
package pkware

import "fmt"

// Error is a stream that cannot be decompressed, for whatever reason.
//
// A caller that reads a file format built on this one wants to say "this is
// not a Finale file" rather than pass a bit-level complaint to somebody who
// never asked about bits, so the reasons are one type rather than many.
type Error struct {
	Reason string
}

func (e *Error) Error() string { return "pkware: " + e.Reason }

func errf(format string, args ...any) error {
	return &Error{Reason: fmt.Sprintf(format, args...)}
}

// The length a decoded length symbol stands for, and how many bits follow it.
//
// The first two are out of order on purpose: that is the format, not a
// mistake. Symbol 0 means three bytes and symbol 1 means two.
var (
	lenBase  = [16]int{3, 2, 4, 5, 6, 7, 8, 9, 10, 12, 16, 24, 40, 72, 136, 264}
	lenExtra = [16]uint{0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}
)

// endOfStream is the length that means stop rather than copy.
//
// It cannot arise from a real copy: the longest is 518 bytes, so 519 is the
// one value left over.
const endOfStream = 519

// maxCodeBits is the longest Huffman code the three tables contain.
const maxCodeBits = 13

// The bit lengths of the three fixed tables, run-length coded the way blast.c
// writes them: in each byte the low four bits are a code length and the high
// four are one less than how many symbols in a row have it.
//
// These are the format. They are not derived from anything and there is
// nothing to check them against but the test vector.
var (
	literalLengths = []byte{
		11, 124, 8, 7, 28, 7, 188, 13, 76, 4, 10, 8, 12, 10, 12, 10, 8, 23, 8,
		9, 7, 6, 7, 8, 7, 6, 55, 8, 23, 24, 12, 11, 7, 9, 11, 12, 6, 7, 22, 5,
		7, 24, 6, 11, 9, 6, 7, 22, 7, 11, 38, 7, 9, 8, 25, 11, 8, 11, 9, 12,
		8, 12, 5, 38, 5, 38, 5, 11, 7, 5, 6, 21, 6, 10, 53, 8, 7, 24, 10, 27,
		44, 253, 253, 253, 252, 252, 252, 13, 12, 45, 12, 45, 12, 61, 12, 45,
		44, 173,
	}
	lengthLengths   = []byte{2, 35, 36, 53, 38, 23}
	distanceLengths = []byte{2, 20, 53, 230, 247, 151, 248}
)

// huffman is a canonical code table held the way decoding wants it: how many
// codes there are of each length, and the symbols in the order the codes run.
type huffman struct {
	count  [maxCodeBits + 1]int
	symbol []int
}

// newHuffman expands one of the run-length coded tables above.
func newHuffman(packed []byte) *huffman {
	var lengths []int
	for _, b := range packed {
		for n := int(b>>4) + 1; n > 0; n-- {
			lengths = append(lengths, int(b&0x0f))
		}
	}

	h := &huffman{symbol: make([]int, len(lengths))}
	for _, l := range lengths {
		h.count[l]++
	}
	// Where each length's symbols start in the table: the codes of length n
	// come after every shorter code.
	var offset [maxCodeBits + 2]int
	for l := 1; l <= maxCodeBits; l++ {
		offset[l+1] = offset[l] + h.count[l]
	}
	for sym, l := range lengths {
		if l != 0 {
			h.symbol[offset[l]] = sym
			offset[l]++
		}
	}
	return h
}

// The tables are the same for every stream, so they are built once.
var (
	literalCode  = newHuffman(literalLengths)
	lengthCode   = newHuffman(lengthLengths)
	distanceCode = newHuffman(distanceLengths)
)

// reader reads the bitstream, least significant bit first.
type reader struct {
	data []byte
	pos  int
	bits uint32
	n    uint
}

// take returns the next need bits.
func (r *reader) take(need uint) (int, error) {
	val := r.bits
	for r.n < need {
		if r.pos >= len(r.data) {
			return 0, errf("the stream ends in the middle of a symbol")
		}
		val |= uint32(r.data[r.pos]) << r.n
		r.pos++
		r.n += 8
	}
	r.bits = val >> need
	r.n -= need
	return int(val & (1<<need - 1)), nil
}

// decode reads one Huffman symbol.
//
// PKWARE stores its canonical codes inverted — a 1 bit in the file means a 0
// bit of the code — which is what the flip below is for. Leaving it out does
// not fail: it decodes to plausible rubbish, which is a far worse way to be
// wrong.
func (r *reader) decode(h *huffman) (int, error) {
	code, first, index := 0, 0, 0
	for l := 1; l <= maxCodeBits; l++ {
		b, err := r.take(1)
		if err != nil {
			return 0, err
		}
		code |= b ^ 1
		if count := h.count[l]; code < first+count {
			return h.symbol[index+code-first], nil
		}
		index += h.count[l]
		first = (first + h.count[l]) << 1
		code <<= 1
	}
	return 0, errf("no code in the tables matches these %d bits", maxCodeBits)
}

// Decompress returns what the stream at the start of data expands to.
//
// The limit is not optional, because this is a decompressor pointed at
// somebody else's file: a stream of a few hundred bytes can ask for every
// byte of memory there is, and the only moment to refuse is while it is being
// built rather than after. A stream that asks for more than limit is an
// error, not a truncated success.
func Decompress(data []byte, limit int) ([]byte, error) {
	if limit <= 0 {
		return nil, errf("asked to decompress with a limit of %d bytes", limit)
	}
	if len(data) < 2 {
		return nil, errf("a stream needs at least a two-byte header; got %d", len(data))
	}

	// The header: whether literals are Huffman coded, and how big the
	// dictionary is. Both are tightly constrained, and that is the whole of
	// what says these bytes are a stream of this kind at all — there is no
	// magic number. Finale writes 0 and 4.
	coded, dict := data[0], uint(data[1])
	if coded > 1 {
		return nil, errf("the literal flag is %d, which is not 0 or 1, "+
			"so this is not a stream of this kind", coded)
	}
	if dict < 4 || dict > 6 {
		return nil, errf("the dictionary size is %d, which is not 4, 5 or 6, "+
			"so this is not a stream of this kind", dict)
	}

	r := &reader{data: data, pos: 2}
	out := make([]byte, 0, min(limit, 4*len(data)))
	for {
		flag, err := r.take(1)
		if err != nil {
			return nil, err
		}

		if flag == 0 {
			// A literal: one byte, either straight from the stream or
			// through the literal table.
			var b int
			if coded == 0 {
				b, err = r.take(8)
			} else {
				b, err = r.decode(literalCode)
			}
			if err != nil {
				return nil, err
			}
			out = append(out, byte(b))
			if len(out) > limit {
				return nil, errf("the stream expands past the %d-byte limit", limit)
			}
			continue
		}

		// A back-reference: how many bytes, then how far back.
		sym, err := r.decode(lengthCode)
		if err != nil {
			return nil, err
		}
		extra, err := r.take(lenExtra[sym])
		if err != nil {
			return nil, err
		}
		length := lenBase[sym] + extra
		if length == endOfStream {
			return out, nil
		}

		sym, err = r.decode(distanceCode)
		if err != nil {
			return nil, err
		}
		// A copy of two bytes takes its low bits from a fixed two rather
		// than from the dictionary size, since nothing that short needs to
		// reach far.
		low := dict
		if length == 2 {
			low = 2
		}
		off, err := r.take(low)
		if err != nil {
			return nil, err
		}
		dist := (sym << low) + off + 1

		if dist > len(out) {
			return nil, errf("a reference reaches %d bytes back, "+
				"which is before the start of the output", dist)
		}
		if len(out)+length > limit {
			return nil, errf("the stream expands past the %d-byte limit", limit)
		}
		// One byte at a time, and deliberately: a distance of one and a
		// length of five hundred is how this format writes a run of the
		// same byte, so each byte copied has to be able to see the one
		// written just before it. A slice copy would read the old contents
		// and get it wrong.
		for i := len(out) - dist; length > 0; i, length = i+1, length-1 {
			out = append(out, out[i])
		}
	}
}
