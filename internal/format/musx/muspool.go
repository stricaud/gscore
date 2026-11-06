/*
 * muspool.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musx

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/stricaud/gscore/internal/pkware"
)

// A .mus holds its document in four pools. The older container names each
// one; the newer does not, and writes them in this order.
const (
	poolOthers  = 15
	poolDetails = 16
	poolEntries = 17
	poolText    = 18
)

// musPool is one pool, decompressed.
type musPool struct {
	// kind is the container's own name for it, or 0 where the container
	// does not name its pools. Zero means "ask the bytes", not "no pool".
	kind int
	data []byte
}

const (
	// A pool record's header in the older container: the pool it holds,
	// how long the whole record is, and a checksum this does not verify.
	musRecordHeader = 10
	// A pool with nothing in it: the kind and the length, and then the next
	// record. It has to come back as an empty pool rather than be skipped,
	// or every pool after it is taken for the wrong one.
	musEmptyRecord = 6
	// Enough to stop a runaway. Real files carry four.
	musMaxPools = 64
	// How much the first zlib stream has to produce to be believed. Nothing
	// frames it — it is found by looking — so a chance pair of bytes that
	// inflates could start the chain in the wrong place and drag every
	// later offset along with it.
	musFirstStreamLeast = 4096
)

// musDCLPools walks the chain of PKWARE DCL records of a 2001 to 2005 file.
//
// The records tile the file from the container start to its last byte with no
// gaps and nothing after. A walk that would step past the end, or that meets
// a record too short to hold its own header, is refused rather than truncated:
// half a document read as a whole one is worse than an error, because nothing
// downstream can tell.
func musDCLPools(raw []byte) ([]musPool, binary.ByteOrder, error) {
	order, err := musByteOrder(raw)
	if err != nil {
		return nil, nil, err
	}

	var pools []musPool
	budget := musMaxPayload
	for at := musContainerAt; at < len(raw); {
		if len(pools) >= musMaxPools {
			return nil, nil, fmt.Errorf(
				"the pool chain runs to more than %d records", musMaxPools)
		}
		if at+musEmptyRecord > len(raw) {
			return nil, nil, fmt.Errorf("the pool record at %#x is cut short", at)
		}
		kind := int(order.Uint16(raw[at:]))
		length := int(order.Uint32(raw[at+2:]))
		if length < musEmptyRecord || at+length > len(raw) {
			return nil, nil, fmt.Errorf(
				"the pool record at %#x says it is %d bytes, which does not fit "+
					"the %d that are left", at, length, len(raw)-at)
		}

		if length == musEmptyRecord {
			pools = append(pools, musPool{kind: kind})
			at += length
			continue
		}
		if length < musRecordHeader {
			return nil, nil, fmt.Errorf(
				"the pool record at %#x says it is %d bytes, too short for its "+
					"%d-byte header", at, length, musRecordHeader)
		}

		// Sliced to this record, so that a malformed stream cannot read on
		// into the next one and come back with something plausible.
		stream := raw[at+musRecordHeader : at+length]
		data, err := pkware.Decompress(stream, budget)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"the pool at %#x did not decompress: %w", at, err)
		}
		budget -= len(data)
		pools = append(pools, musPool{kind: kind, data: data})
		at += length
	}

	if len(pools) == 0 {
		return nil, nil, fmt.Errorf("the pool container holds no records")
	}
	return pools, order, nil
}

// musByteOrder works out which way round this file writes its numbers.
//
// A Mac wrote them big end first and a PC little, and nothing in the file
// says which. What says it is the first record's kind, which is the others
// pool in every file anyone has looked at: 15 is only 15 one way round, and
// 3840 the other. So the check that finds the byte order is the same check
// that there is a container here at all, which is why it gives up rather than
// picking one.
func musByteOrder(raw []byte) (binary.ByteOrder, error) {
	if len(raw) < musContainerAt+musEmptyRecord {
		return nil, fmt.Errorf("the file is too short to hold a pool container")
	}
	head := raw[musContainerAt:]
	if binary.BigEndian.Uint16(head) == poolOthers {
		return binary.BigEndian, nil
	}
	if binary.LittleEndian.Uint16(head) == poolOthers {
		return binary.LittleEndian, nil
	}
	return nil, fmt.Errorf(
		"there is no pool container at %#x: the first record calls itself %d "+
			"one way round and %d the other, and a pool chain starts with %d",
		musContainerAt, binary.BigEndian.Uint16(head),
		binary.LittleEndian.Uint16(head), poolOthers)
}

// musZlibPools walks the chain of zlib streams of a 2011 or 2012 file.
//
// Only the first stream is searched for. The rest are framed: each stream is
// followed by the same ten-byte record header the older container uses and
// then the next stream, so the chain can be followed rather than scanned for,
// which is what stops a small pool being passed over for looking like noise.
//
// This container does not say which pool is which, so the pools come back
// unnamed and musDocument works it out from the bytes.
func musZlibPools(raw []byte) ([]musPool, binary.ByteOrder, error) {
	at := musFirstZlibStream(raw)
	if at < 0 {
		return nil, nil, fmt.Errorf("there is no zlib stream in this file")
	}

	var pools []musPool
	budget := musMaxPayload
	for at >= 0 && at < len(raw)-1 && len(pools) < musMaxPools {
		data, used, err := musInflate(raw[at:], budget)
		if err != nil {
			break
		}
		budget -= len(data)
		pools = append(pools, musPool{data: data})

		// The framed position first, then one stream hard against the
		// next. Both are exact, so neither can drift — and a record header
		// cannot be mistaken for a zlib one, because its first byte fails
		// the test below.
		end := at + max(used, 1)
		at = -1
		for _, next := range []int{end + musRecordHeader, end} {
			if musIsZlibHeader(raw, next) {
				at = next
				break
			}
		}
	}

	if len(pools) == 0 {
		return nil, nil, fmt.Errorf("nothing in this file inflated")
	}
	// Every file of this era carries its numbers little end first; no file
	// anyone has looked at says otherwise, and unlike the older container
	// there is nothing here to read it off.
	return pools, binary.LittleEndian, nil
}

// musFirstZlibStream finds where the chain starts: the first header that
// inflates to something substantial.
//
// Every offset in the file is tried, because the preamble in front of the
// payload is not a fixed length. In a file of any size a pair of bytes that
// looks like a zlib header turns up by chance a few thousand times, so what
// each of those costs decides what this costs: it reads only as far as it
// needs to to answer the question, and throws the bytes away. Inflating each
// candidate in full would mean building megabytes of rubbish a few thousand
// times over, out of a file nobody here wrote.
func musFirstZlibStream(raw []byte) int {
	for at := 0; at < len(raw)-1; at++ {
		if !musIsZlibHeader(raw, at) {
			continue
		}
		if musInflatesAtLeast(raw[at:], musFirstStreamLeast) {
			return at
		}
	}
	return -1
}

// musInflatesAtLeast says whether a stream produces at least n bytes, without
// keeping them.
func musInflatesAtLeast(raw []byte, n int) bool {
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return false
	}
	defer zr.Close()
	got, err := io.CopyN(io.Discard, zr, int64(n))
	return err == nil && got == int64(n)
}

// musIsZlibHeader applies zlib's own rule rather than looking for the one
// pair of bytes Finale happens to write, so that a file saved at another
// compression level is not passed over.
func musIsZlibHeader(raw []byte, at int) bool {
	if at < 0 || at+2 > len(raw) {
		return false
	}
	cmf, flg := raw[at], raw[at+1]
	const deflate = 8
	return cmf&0x0f == deflate && (int(cmf)<<8|int(flg))%31 == 0
}

// musInflate inflates one zlib stream from the front of raw, and says how
// many bytes of input it took, so the caller can step to the next one.
//
// The checksum at the end of a zlib stream is what makes this safe to point
// at a guessed offset: two bytes that look like a header by chance are common
// in a large file, and a stream that is not one will almost never add up. So
// there is no need for the stored-block test a decompressor without a
// checksum would want here — this is zlib, not raw deflate, on purpose.
func musInflate(raw []byte, limit int) ([]byte, int, error) {
	if limit <= 0 {
		return nil, 0, fmt.Errorf("no room left to inflate into")
	}
	from := bytes.NewReader(raw)
	zr, err := zlib.NewReader(from)
	if err != nil {
		return nil, 0, err
	}
	defer zr.Close()

	// One byte more than the limit, so that hitting it is told apart from
	// ending on it.
	var out bytes.Buffer
	n, err := io.Copy(&out, io.LimitReader(zr, int64(limit)+1))
	if err != nil {
		return nil, 0, err
	}
	if n > int64(limit) {
		return nil, 0, fmt.Errorf("the stream inflates past the %d-byte limit", limit)
	}
	return out.Bytes(), len(raw) - from.Len(), nil
}
