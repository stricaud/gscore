/*
 * flac.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package flac writes FLAC files.
//
// A recording of a score is written losslessly because it is made from the
// score: there is no reason to throw anything away on the way out, and a
// player reading it back gets exactly what was synthesised. FLAC is the
// format that does that and that everything plays.
//
// Only what is needed to write one is here. Reading is somebody else's job,
// and the encoder keeps to fixed predictors rather than the full linear
// prediction a dedicated encoder uses: the second-order difference of a
// synthesised line already codes to about half its size, and the remaining
// few percent are not worth the machinery. What is not optional is being
// exactly right, so the output is checked against the reference decoder in
// the tests.
package flac

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"io"
	"math/bits"
)

// blockSize is how many frames of audio go in one FLAC frame.
//
// 4096 is what every encoder uses: long enough that a frame header is a
// rounding error, short enough that the signal inside one is uniform enough
// to predict.
const blockSize = 4096

// Encode writes interleaved 16-bit samples as a FLAC stream.
//
// pcm holds the channels interleaved, as sound is always handed about: for
// stereo, left then right, repeating. Everything the header has to say —
// how many samples there are, and the checksum of them — is known before a
// byte is written, so nothing has to be seeked back to and patched.
func Encode(w io.Writer, pcm []int16, channels, sampleRate int) error {
	if channels < 1 || channels > 8 {
		return errors.New("flac: a stream has between one and eight channels")
	}
	if sampleRate < 1 || sampleRate >= 1<<20 {
		return errors.New("flac: that sample rate cannot be written")
	}
	if len(pcm)%channels != 0 {
		return errors.New("flac: the samples do not divide into whole frames")
	}
	total := len(pcm) / channels

	// The checksum is of the audio as it went in, not as it was coded: it is
	// what lets a decoder say it got back what was put there.
	sum := md5.New()
	raw := make([]byte, 2*len(pcm))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(raw[2*i:], uint16(s))
	}
	sum.Write(raw)

	if _, err := w.Write([]byte("fLaC")); err != nil {
		return err
	}
	if err := writeStreamInfo(w, total, channels, sampleRate, sum.Sum(nil)); err != nil {
		return err
	}

	// Each channel on its own, so a frame is built from columns rather than
	// from the interleaved rows the caller has.
	cols := make([][]int32, channels)
	for c := range cols {
		cols[c] = make([]int32, blockSize)
	}

	for start, frame := 0, uint64(0); start < total; start, frame = start+blockSize, frame+1 {
		n := blockSize
		if start+n > total {
			n = total - start
		}
		for c := 0; c < channels; c++ {
			col := cols[c][:n]
			for i := 0; i < n; i++ {
				col[i] = int32(pcm[(start+i)*channels+c])
			}
		}
		if err := writeFrame(w, cols, n, frame, channels); err != nil {
			return err
		}
	}
	return nil
}

// writeStreamInfo writes the one metadata block a stream must carry.
func writeStreamInfo(w io.Writer, total, channels, sampleRate int, sum []byte) error {
	b := newBits()
	b.write(1, 1)   // this is the last metadata block
	b.write(0, 7)   // of type STREAMINFO
	b.write(34, 24) // which is always 34 bytes

	b.write(blockSize, 16) // the smallest block
	b.write(blockSize, 16) // and the largest: every frame but the last
	b.write(0, 24)         // frame sizes unknown, which is allowed
	b.write(0, 24)
	b.write(uint64(sampleRate), 20)
	b.write(uint64(channels-1), 3)
	b.write(15, 5) // 16 bits per sample, written one less
	b.write(uint64(total), 36)
	b.bytes = append(b.bytes, sum...)

	_, err := w.Write(b.bytes)
	return err
}

/*
 * Frames
 */

// writeFrame codes one block of every channel.
func writeFrame(w io.Writer, cols [][]int32, n int, frame uint64, channels int) error {
	// Stereo is coded from the two channels together where that is smaller:
	// the two sides of a recording are mostly the same sound, so their
	// difference codes far more tightly than either of them does.
	assignment, parts := stereoChoice(cols, n, channels)

	b := newBits()
	b.write(0x3FFE, 14) // the sync code
	b.write(0, 1)       // reserved
	b.write(0, 1)       // every frame but the last holds blockSize samples
	if n == blockSize {
		b.write(12, 4) // 256 * 2^4
	} else {
		b.write(7, 4) // the size follows the header, as 16 bits
	}
	b.write(0, 4) // the sample rate is in STREAMINFO
	b.write(uint64(assignment), 4)
	b.write(4, 3) // 16 bits per sample
	b.write(0, 1) // reserved
	b.writeUTF8(frame)
	if n != blockSize {
		b.write(uint64(n-1), 16)
	}
	b.write(uint64(crc8(b.bytes)), 8)

	for i, col := range parts {
		depth := 16
		// The difference of two channels needs one bit more than they do:
		// it ranges twice as far.
		if side(assignment, i) {
			depth = 17
		}
		writeSubframe(b, col[:n], depth)
	}
	b.align()
	b.write(uint64(crc16(b.bytes)), 16)

	_, err := w.Write(b.bytes)
	return err
}

// stereoChoice picks how a stereo frame is coded, and returns the channels to
// code under that choice.
//
// Four ways are allowed: the two channels as they are, or either of them
// alongside their difference, or their average alongside their difference.
// Which is smallest is not worth predicting, so all four are measured.
func stereoChoice(cols [][]int32, n, channels int) (int, [][]int32) {
	if channels != 2 {
		return channels - 1, cols
	}
	left, right := cols[0][:n], cols[1][:n]
	mid := make([]int32, n)
	diff := make([]int32, n)
	for i := range left {
		diff[i] = left[i] - right[i]
		// The average, rounded down, which is what the format specifies:
		// the decoder can recover both channels from it and the difference.
		mid[i] = (left[i] + right[i]) >> 1
	}

	l, r, m, d := estimate(left), estimate(right), estimate(mid), estimate(diff)
	type option struct {
		assignment int
		cost       int
		cols       [][]int32
	}
	best := option{1, l + r, [][]int32{left, right}}
	for _, o := range []option{
		{8, l + d, [][]int32{left, diff}},
		{9, d + r, [][]int32{diff, right}},
		{10, m + d, [][]int32{mid, diff}},
	} {
		if o.cost < best.cost {
			best = o
		}
	}
	return best.assignment, best.cols
}

// side says whether this channel of a frame is a difference rather than a
// recorded channel, and so needs the extra bit.
func side(assignment, channel int) bool {
	switch assignment {
	case 8:
		return channel == 1
	case 9:
		return channel == 0
	case 10:
		return channel == 1
	}
	return false
}

// estimate is roughly how many bits a channel costs, used only to compare
// one way of coding a frame against another.
func estimate(x []int32) int {
	best := 1 << 30
	for order := 0; order <= 4; order++ {
		if order >= len(x) {
			break
		}
		res := residuals(x, order)
		total := 0
		for _, r := range res {
			total += bits.Len32(zigzag(r)) + 1
		}
		if total < best {
			best = total
		}
	}
	return best
}

/*
 * Subframes
 */

// writeSubframe codes one channel of one frame.
func writeSubframe(b *bitWriter, x []int32, depth int) {
	if constant(x) {
		b.write(0, 1)
		b.write(0, 6) // CONSTANT
		b.write(0, 1)
		b.write(uint64(uint32(x[0])), uint(depth))
		return
	}

	order, cost := bestFixedOrder(x, depth)
	// Storing the samples as they are is the floor: a signal that predicts
	// badly — noise, or a waveform that jumps — must not be made larger by
	// being coded.
	if verbatim := len(x) * depth; verbatim <= cost {
		b.write(0, 1)
		b.write(1, 6) // VERBATIM
		b.write(0, 1)
		for _, s := range x {
			b.write(uint64(uint32(s)), uint(depth))
		}
		return
	}

	b.write(0, 1)
	b.write(uint64(8+order), 6) // FIXED, of this order
	b.write(0, 1)
	for i := 0; i < order; i++ {
		b.write(uint64(uint32(x[i])), uint(depth))
	}
	writeResidual(b, residuals(x, order), order, len(x))
}

func constant(x []int32) bool {
	for _, s := range x[1:] {
		if s != x[0] {
			return false
		}
	}
	return true
}

// bestFixedOrder picks the predictor, and says what it will cost in bits.
//
// The fixed predictors are the first few differences of the signal: order 1
// is how much it changed, order 2 how much that change changed, and so on.
// A tone that is nearly a straight line between samples has almost nothing
// left after the second one.
func bestFixedOrder(x []int32, depth int) (int, int) {
	bestOrder, bestCost := 0, 1<<30
	for order := 0; order <= 4; order++ {
		if order >= len(x) {
			break
		}
		cost := order*depth + residualCost(residuals(x, order), order, len(x))
		if cost < bestCost {
			bestOrder, bestCost = order, cost
		}
	}
	return bestOrder, bestCost
}

// residuals is what the fixed predictor of this order failed to predict.
func residuals(x []int32, order int) []int32 {
	out := make([]int32, 0, len(x))
	for i := order; i < len(x); i++ {
		switch order {
		case 0:
			out = append(out, x[i])
		case 1:
			out = append(out, x[i]-x[i-1])
		case 2:
			out = append(out, x[i]-2*x[i-1]+x[i-2])
		case 3:
			out = append(out, x[i]-3*x[i-1]+3*x[i-2]-x[i-3])
		case 4:
			out = append(out, x[i]-4*x[i-1]+6*x[i-2]-4*x[i-3]+x[i-4])
		}
	}
	return out
}

/*
 * Residual coding
 *
 * What is left after prediction is small and clusters around zero, so it is
 * written in Rice codes: the low k bits of a value as they are, and what is
 * above them in unary. Choosing k well is the whole of the compression, and
 * the right k drifts through a piece — a loud passage needs a larger one —
 * so the block is split into partitions with a k each.
 */

// maxPartitionOrder is how finely a block may be split. Beyond this the
// partitions are shorter than the parameter that describes them is worth.
const maxPartitionOrder = 6

// writeResidual codes what the predictor left over.
func writeResidual(b *bitWriter, res []int32, order, n int) {
	part, params := bestPartition(res, order, n)

	// Two codings exist, differing only in how wide the parameter is. The
	// narrow one covers everything but a very loud, very unpredictable
	// signal.
	wide := false
	for _, k := range params {
		if k > 14 {
			wide = true
		}
	}
	if wide {
		b.write(1, 2)
	} else {
		b.write(0, 2)
	}
	b.write(uint64(part), 4)

	width := uint(4)
	if wide {
		width = 5
	}
	at := 0
	for i, k := range params {
		b.write(uint64(k), width)
		count := n>>uint(part) - order*boolToInt(i == 0)
		for _, r := range res[at : at+count] {
			writeRice(b, zigzag(r), k)
		}
		at += count
	}
}

// bestPartition splits the residual into partitions and picks a Rice
// parameter for each, returning the split that costs least.
func bestPartition(res []int32, order, n int) (int, []int) {
	bestOrder, bestParams, bestCost := 0, []int(nil), 1<<60
	for p := 0; p <= maxPartitionOrder; p++ {
		if n%(1<<uint(p)) != 0 {
			break
		}
		// The first partition is short by the warm-up samples the predictor
		// needed, and a split that leaves it empty is not a split.
		if n>>uint(p) <= order {
			break
		}
		params := make([]int, 1<<uint(p))
		cost, at := 0, 0
		for i := range params {
			count := n>>uint(p) - order*boolToInt(i == 0)
			k, c := bestRice(res[at : at+count])
			params[i], cost = k, cost+c
			at += count
		}
		if cost < bestCost {
			bestOrder, bestParams, bestCost = p, params, cost
		}
	}
	return bestOrder, bestParams
}

// residualCost is what a residual will take to write, used to choose between
// predictors before any of them is written.
func residualCost(res []int32, order, n int) int {
	_, params := bestPartition(res, order, n)
	cost, at := 0, 0
	part := 0
	for 1<<uint(part) < len(params) {
		part++
	}
	for i, k := range params {
		count := n>>uint(part) - order*boolToInt(i == 0)
		_, c := riceCost(res[at:at+count], k)
		cost += c
		at += count
	}
	return cost
}

// bestRice finds the parameter that writes this partition smallest.
func bestRice(res []int32) (int, int) {
	bestK, bestCost := 0, 1<<60
	for k := 0; k <= 30; k++ {
		_, cost := riceCost(res, k)
		if cost < bestCost {
			bestK, bestCost = k, cost
		}
		// The cost falls to a minimum and then climbs, so once it climbs
		// there is nothing further to find.
		if cost > bestCost*2 {
			break
		}
	}
	return bestK, bestCost
}

func riceCost(res []int32, k int) (int, int) {
	total := 0
	for _, r := range res {
		total += int(zigzag(r)>>uint(k)) + 1 + k
	}
	return k, total
}

// writeRice writes one value: what is above the low k bits in unary, then
// those bits as they are.
func writeRice(b *bitWriter, v uint32, k int) {
	high := v >> uint(k)
	for ; high >= 32; high -= 32 {
		b.write(0, 32)
	}
	b.write(1, uint(high)+1)
	if k > 0 {
		b.write(uint64(v&(1<<uint(k)-1)), uint(k))
	}
}

// zigzag folds a signed value onto an unsigned one, so that small negative
// numbers are small: -1 becomes 1, 1 becomes 2, -2 becomes 3.
func zigzag(v int32) uint32 { return uint32(v<<1) ^ uint32(v>>31) }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

/*
 * Bits and checksums
 */

type bitWriter struct {
	bytes []byte
	acc   uint64
	n     uint
}

func newBits() *bitWriter { return &bitWriter{} }

// write puts the low n bits of v on the end of the stream, most significant
// first.
func (b *bitWriter) write(v uint64, n uint) {
	for n > 0 {
		take := n
		if room := 64 - b.n; take > room {
			take = room
		}
		b.acc = b.acc<<take | (v>>(n-take))&(1<<take-1)
		b.n += take
		n -= take
		for b.n >= 8 {
			b.n -= 8
			b.bytes = append(b.bytes, byte(b.acc>>b.n))
		}
		b.acc &= 1<<b.n - 1
	}
}

// align pads with zeroes to the next byte, which is where a frame's checksum
// has to start.
func (b *bitWriter) align() {
	if b.n > 0 {
		b.write(0, 8-b.n)
	}
}

// writeUTF8 writes a number the way a frame header counts frames: like UTF-8,
// but stretched to hold 36 bits.
func (b *bitWriter) writeUTF8(v uint64) {
	switch {
	case v < 0x80:
		b.write(v, 8)
		return
	}
	// How many continuation bytes are needed, and so how many ones the first
	// byte opens with.
	n := 1
	for limit := uint64(0x800); v >= limit && n < 6; limit <<= 5 {
		n++
	}
	lead := byte(0xFF << (7 - n))
	b.write(uint64(lead|byte(v>>uint(6*n))), 8)
	for i := n - 1; i >= 0; i-- {
		b.write(uint64(0x80|byte(v>>uint(6*i))&0x3F), 8)
	}
}

func crc8(data []byte) byte {
	var crc byte
	for _, c := range data {
		crc ^= c
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = crc<<1 ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func crc16(data []byte) uint16 {
	var crc uint16
	for _, c := range data {
		crc ^= uint16(c) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x8005
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
