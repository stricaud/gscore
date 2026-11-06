/*
 * literals.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pkware

// Literals writes bytes as a stream Decompress will read back, and
// compresses nothing at all.
//
// It is here so that a test can build a file in this format rather than need
// one. A reader with no files to read is a reader nobody can check, and the
// Finale files this exists for are nobody's to commit: they are somebody's
// music. A test that writes its own stream and reads it back checks the
// framing around it — the pool records, the byte order, the entry slots —
// without any of that.
//
// The output is larger than the input, by a bit per byte and a little more.
// Writing a compressor to put in a test would be writing the harder half of
// the format for no gain, since what the test needs is a stream that is
// valid, not a stream that is small. A real stream from Finale carries
// back-references too, and those are exercised by the vector from blast.c.
func Literals(data []byte) []byte {
	var w bitWriter
	for _, b := range data {
		w.add(0, 1) // a literal follows
		w.add(int(b), 8)
	}
	// Stop: the length that means stop rather than copy. 519 is out of
	// reach of a real copy, and it is reached here as the largest base the
	// table has plus every extra bit set. It is a length, so it is
	// introduced by the same flag bit a copy is.
	w.add(1, 1)
	w.addCode(lengthCode, 15)
	w.add(endOfStream-lenBase[15], int(lenExtra[15]))

	// Uncoded literals and the smallest dictionary, which is what Finale
	// writes. The dictionary size makes no difference to a stream with
	// nothing in it to refer back to.
	return append([]byte{0x00, 0x04}, w.bytes()...)
}

// bitWriter lays bits down the way the format reads them: least significant
// bit of each byte first.
type bitWriter struct {
	out  []byte
	bits uint32
	n    uint
}

// add writes the low n bits of value.
func (w *bitWriter) add(value int, n int) {
	w.bits |= uint32(value&(1<<n-1)) << w.n
	w.n += uint(n)
	for w.n >= 8 {
		w.out = append(w.out, byte(w.bits))
		w.bits >>= 8
		w.n -= 8
	}
}

// addCode writes one Huffman symbol, inverted and longest-bit-first, which is
// the order decode reads it in.
func (w *bitWriter) addCode(h *huffman, sym int) {
	code, length, ok := h.code(sym)
	if !ok {
		panic("pkware: no code for that symbol")
	}
	for i := length - 1; i >= 0; i-- {
		w.add((code>>i&1)^1, 1)
	}
}

// bytes returns what has been written, padded out to a whole byte.
//
// The padding is read as the start of another symbol and never reached,
// because the end code comes first.
func (w *bitWriter) bytes() []byte {
	if w.n > 0 {
		return append(w.out, byte(w.bits))
	}
	return w.out
}

// code finds a symbol's canonical code and its length.
//
// It walks the table the same way decode does, so the two cannot disagree
// about what the codes are.
func (h *huffman) code(sym int) (code, length int, ok bool) {
	first, index := 0, 0
	for l := 1; l <= maxCodeBits; l++ {
		for i := 0; i < h.count[l]; i++ {
			if h.symbol[index+i] == sym {
				return first + i, l, true
			}
		}
		index += h.count[l]
		first = (first + h.count[l]) << 1
	}
	return 0, 0, false
}
