/*
 * reader.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package guitarpro

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// song is a Guitar Pro file as the file has it, before any of it is music:
// the three dialects are read into this one shape, and build turns it into
// a score once.
type song struct {
	version int // 3, 4 or 5
	// code tells the revisions of one version apart: 4.00 from 4.06, 5.00
	// from 5.10. They differ by a byte here and there and nowhere else.
	code int

	title, subtitle, artist, album string
	words, music, copyright        string
	tempo                          int
	key                            int // fifths: 0 C, 1 G, -1 F

	lyricTrack int
	lyrics     []lyricLine

	channels [64]channel
	headers  []header
	tracks   []track
}

type channel struct{ program int }

type lyricLine struct {
	from int // the bar the words start at, counting from one
	text string
}

// header is what every track shares about one bar.
type header struct {
	num, den    int
	repeatOpen  bool
	repeatClose int
	// alt is the endings this bar belongs to, as bits: bit 0 is the first
	// time through.
	alt    int
	marker string
	key    *int
}

type track struct {
	name string
	// strings are the open strings as MIDI notes, highest first, which is
	// the order the file lists them in and the order the tab draws them.
	strings    []int
	channel    int // General MIDI channel, counting from one; 10 is drums
	program    int
	frets      int
	capo       int
	percussion bool
	bassClef   bool
	measures   []measure
}

type measure struct {
	voices [][]beat
}

// beat is one moment in one voice: a rest, or the notes struck together.
type beat struct {
	rest  bool
	empty bool // a place-holder the file writes for a voice with nothing in it
	kind  int
	dur   int // as a denominator: 4 is a quarter
	dot   bool
	// tuplet is the file's division figure: 3 for a triplet, 5, 6, 7, and
	// so on up to 13. Zero is none.
	tuplet int
	notes  []note
	text   string
	chord  *chord
	tap    bool
	slap   bool
	pop    bool
	vib    bool
	fadeIn bool
	// GP3 writes harmonics on the beat rather than on the note.
	natural, artificial bool
	tempo               int // a tempo change at this beat, or -1
	// stroke is the brush across the strings: 1 down, 2 up, 0 none.
	stroke int
	// pick is the plectrum's direction: 1 up, 2 down, 0 unsaid.
	pick     int
	whammy   bool
	rasguedo bool
}

type chord struct {
	name      string
	firstFret int
	frets     []int // highest string first; -1 is not played
}

type note struct {
	str      int // from the top of the tab, 1 the highest
	fret     int
	tied     bool
	dead     bool
	ghost    bool
	accent   bool
	heavy    bool
	velocity int // the file's dynamic figure, 1 to 9; 0 is unsaid
	hammer   bool
	letRing  bool
	palmMute bool
	staccato bool
	vib      bool
	// slide is the file's kind: 1 shift, 2 legato, 3 out and down, 4 out
	// and up, -1 in from below, -2 in from above. GP3 says only that there
	// is one, which is 1 here.
	slide    int
	bend     *bend
	grace    *grace
	tremolo  int // tremolo picking: 1 eighths, 2 sixteenths, 3 thirty-seconds
	harmonic int // 1 natural, 2 artificial, 3 tapped, 4 pinch, 5 semi
	trill    *trill
	finger   int // left hand, -1 unsaid
}

// bend is how far the string is pushed. kind is the file's own: 1 a bend,
// 2 bend and release, 3 bend, release and bend again, 4 a pre-bend, 5 a
// pre-bend released. value is in the file's units, a hundred to the tone.
type bend struct {
	kind, value int
	peak        int
}

type grace struct {
	fret       int
	transition int // 0 none, 1 slide, 2 bend, 3 hammer
	duration   int // 1 a sixteenth, 2 a twenty-fourth, 3 a thirty-second
	dead       bool
	onBeat     bool
}

type trill struct{ fret, period int }

// reader walks the bytes. Every primitive records the first thing that
// goes wrong and answers zero after it, so the dialect readers can be
// written straight through and checked at the end: a file cut short is
// read as far as it goes and then reported, which is what a file cut short
// deserves.
type reader struct {
	r   *bufio.Reader
	pos int
	err error
	buf [8]byte
	// where is the part of the file being read, for the report when the
	// file ends before it should.
	where string
}

func newReader(r io.Reader) *reader {
	return &reader{r: bufio.NewReaderSize(r, 1<<16)}
}

func (r *reader) fail(err error) {
	if r.err == nil {
		r.err = fmt.Errorf("%w at byte %d", err, r.pos)
	}
}

func (r *reader) read(n int) []byte {
	if r.err != nil || n <= 0 {
		return nil
	}
	var b []byte
	if n <= len(r.buf) {
		b = r.buf[:n]
	} else {
		b = make([]byte, n)
	}
	got, err := io.ReadFull(r.r, b)
	r.pos += got
	if err != nil {
		r.fail(io.ErrUnexpectedEOF)
		return nil
	}
	return b
}

func (r *reader) skip(n int) {
	if r.err != nil || n <= 0 {
		return
	}
	got, err := r.r.Discard(n)
	r.pos += got
	if err != nil {
		r.fail(io.ErrUnexpectedEOF)
	}
}

// skipTrailing passes over padding the file may end without: Guitar Pro 5
// pads each bar with a byte and leaves the last one off as often as not.
func (r *reader) skipTrailing(n int) {
	if r.err != nil || n <= 0 {
		return
	}
	got, err := r.r.Discard(n)
	r.pos += got
	if err != nil && err != io.EOF {
		r.fail(io.ErrUnexpectedEOF)
	}
}

func (r *reader) ubyte() int {
	b := r.read(1)
	if b == nil {
		return 0
	}
	return int(b[0])
}

func (r *reader) sbyte() int { return int(int8(r.ubyte())) }

func (r *reader) boolean() bool { return r.ubyte() == 1 }

func (r *reader) int32() int {
	b := r.read(4)
	if b == nil {
		return 0
	}
	return int(int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24))
}

// maxString is longer than any string a Guitar Pro file has a reason to
// hold, and short enough that a count read off garbage does not ask for
// the memory.
const maxString = 1 << 20

// str reads a string stored in size bytes of which length are the text;
// where size is nought the text is the whole field.
func (r *reader) str(size, length int) string {
	if size < 0 || length < 0 || size > maxString || length > maxString {
		r.fail(fmt.Errorf("a string of %d bytes", max(size, length)))
		return ""
	}
	n := size
	if n == 0 {
		n = length
	}
	b := r.read(n)
	if b == nil {
		return ""
	}
	if length > len(b) {
		length = len(b)
	}
	return decode(b[:length])
}

// strInt is an int of length and then the text.
func (r *reader) strInt() string {
	n := r.int32()
	return r.str(0, n)
}

// strByte is a byte of length and then size bytes, or the length's worth
// where size is nought.
func (r *reader) strByte(size int) string {
	n := r.ubyte()
	return r.str(size, n)
}

// strIntByte is an int holding the field's size, then a byte of length,
// then the field less the byte: the shape most of the header is in.
func (r *reader) strIntByte() string {
	n := r.int32()
	return r.strByte(n - 1)
}

// decode reads the file's text, which Guitar Pro wrote in the Windows code
// page, with the odd newer file in UTF-8.
func decode(b []byte) string {
	if utf8.Valid(b) {
		ascii := true
		for _, c := range b {
			if c >= 0x80 {
				ascii = false
				break
			}
		}
		if ascii {
			return string(b)
		}
		return strings.ToValidUTF8(string(b), "")
	}
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(cp1252(c))
	}
	return sb.String()
}

// cp1252High maps the bytes 0x80 to 0x9F, the only range where Windows-1252
// differs from Latin-1: the curly quotes and the dashes a title has in it.
var cp1252High = [32]rune{
	'€', '\u0081', '‚', 'ƒ', '„', '…', '†', '‡',
	'ˆ', '‰', 'Š', '‹', 'Œ', '\u008D', 'Ž', '\u008F',
	'\u0090', '‘', '’', '“', '”', '•', '–', '—',
	'˜', '™', 'š', '›', 'œ', '\u009D', 'ž', 'Ÿ',
}

func cp1252(c byte) rune {
	if c >= 0x80 && c <= 0x9F {
		return cp1252High[c-0x80]
	}
	return rune(c)
}

/*
 * The file
 */

// readVersion reads the thirty-byte banner every Guitar Pro file opens
// with and says which dialect follows it.
func (r *reader) readVersion() (version, code int, err error) {
	raw := r.read(31)
	if raw == nil {
		return 0, 0, fmt.Errorf("the file is shorter than a Guitar Pro header")
	}
	n, b := int(raw[0]), raw[1:]
	if n > 30 {
		n = 30
	}
	banner := strings.TrimRight(string(b[:n]), "\x00")
	const prefix = "FICHIER GUITAR PRO "
	if !strings.HasPrefix(banner, prefix) {
		head := string(raw[:4])
		if strings.HasPrefix(head, "PK") || head == "BCFZ" || head == "BCFS" {
			return 0, 0, fmt.Errorf("this is a Guitar Pro 6 or 7 file, which gscore cannot read — " +
				"export it from Guitar Pro as a .gp5 and open that")
		}
		return 0, 0, fmt.Errorf("this is not a Guitar Pro file: it does not open with the Guitar Pro banner")
	}
	v := strings.TrimLeft(banner[len(prefix):], "vL")
	switch v {
	case "3.00":
		return 3, 0, nil
	case "4.00":
		return 4, 0, nil
	case "4.06":
		return 4, 1, nil
	case "5.00":
		return 5, 0, nil
	case "5.10":
		return 5, 1, nil
	}
	return 0, 0, fmt.Errorf("Guitar Pro %s files are not read; gscore reads versions 3, 4 and 5 — "+
		"save the file as one of those and open that", v)
}

// readSong reads the whole file. What it returns is as much as could be
// read; r.err says whether that was all of it.
func (r *reader) readSong() (*song, error) {
	v, code, err := r.readVersion()
	if err != nil {
		return nil, err
	}
	sg := &song{version: v, code: code}
	r.where = "in the header"

	// The header: who wrote it and what it is called.
	sg.title = r.strIntByte()
	sg.subtitle = r.strIntByte()
	sg.artist = r.strIntByte()
	sg.album = r.strIntByte()
	if v == 5 {
		sg.words = r.strIntByte()
		sg.music = r.strIntByte()
	} else {
		sg.music = r.strIntByte()
	}
	sg.copyright = r.strIntByte()
	r.strIntByte() // the tabber
	r.strIntByte() // instructions
	comments := r.int32()
	if comments < 0 || comments > 1000 {
		r.fail(fmt.Errorf("%d comments", comments))
		return sg, r.err
	}
	for i := 0; i < comments && r.err == nil; i++ {
		r.strIntByte()
	}

	if v < 5 {
		r.boolean() // triplet feel, for the whole piece
	}
	if v >= 4 {
		sg.lyricTrack = r.int32()
		for i := 0; i < 5; i++ {
			from := r.int32()
			text := r.strInt()
			if text != "" {
				sg.lyrics = append(sg.lyrics, lyricLine{from: from, text: text})
			}
		}
	}
	if v == 5 {
		// Page setup: paper size, margins, and the eleven templates for
		// the words printed on the page.
		if code > 0 {
			r.skip(49)
		} else {
			r.skip(30)
		}
		for i := 0; i < 11; i++ {
			r.skip(4)
			r.strByte(0)
		}
	}
	sg.tempo = r.int32()
	if v == 5 && code > 0 {
		r.skip(1)
	}
	sg.key = r.sbyte()
	r.skip(3)
	if v >= 4 {
		r.sbyte() // octave
	}
	for i := range sg.channels {
		sg.channels[i].program = r.int32()
		r.skip(6) // volume, balance, chorus, reverb, phaser, tremolo
		r.skip(2)
	}
	if v == 5 {
		r.skip(42)
	}
	bars := r.int32()
	tracks := r.int32()
	if r.err != nil {
		return sg, r.err
	}
	if bars < 0 || bars > 100000 || tracks < 0 || tracks > 256 {
		r.fail(fmt.Errorf("%d bars and %d tracks", bars, tracks))
		return sg, r.err
	}

	r.where = "in the bar headers"
	for i := 0; i < bars && r.err == nil; i++ {
		if v == 5 && i > 0 {
			r.skip(1)
		}
		sg.headers = append(sg.headers, r.readHeader(sg, i))
	}
	r.where = "in the track list"
	for i := 0; i < tracks && r.err == nil; i++ {
		sg.tracks = append(sg.tracks, r.readTrack(sg, i+1))
	}
	if v == 5 {
		if code == 0 {
			r.skip(2)
		} else {
			r.skip(1)
		}
	}
	for i := 0; i < bars && r.err == nil; i++ {
		for t := range sg.tracks {
			if r.err != nil {
				break
			}
			tr := &sg.tracks[t]
			r.where = fmt.Sprintf("in bar %d of %q", i+1, tr.name)
			tr.measures = append(tr.measures, r.readMeasure(sg, tr))
			if v == 5 {
				r.skipTrailing(1)
			}
		}
	}
	return sg, r.err
}

func (r *reader) readHeader(sg *song, index int) header {
	var h header
	flags := r.ubyte()
	// The meter carries from the bar before unless this one says.
	if index > 0 {
		h.num, h.den = sg.headers[index-1].num, sg.headers[index-1].den
	} else {
		h.num, h.den = 4, 4
	}
	if flags&0x01 != 0 {
		h.num = r.sbyte()
	}
	if flags&0x02 != 0 {
		h.den = r.sbyte()
	}
	h.repeatOpen = flags&0x04 != 0
	if flags&0x08 != 0 {
		if sg.version == 5 {
			h.repeatClose = r.ubyte() - 1
		} else {
			h.repeatClose = r.sbyte()
		}
	}
	if sg.version < 5 {
		if flags&0x10 != 0 {
			h.alt = endingsFromCount(sg.headers, r.ubyte())
		}
		if flags&0x20 != 0 {
			h.marker = r.strIntByte()
			r.skip(4) // its colour
		}
		if flags&0x40 != 0 {
			k := r.sbyte()
			h.key = &k
			r.skip(1)
		}
		return h
	}
	if flags&0x20 != 0 {
		h.marker = r.strIntByte()
		r.skip(4)
	}
	if flags&0x40 != 0 {
		k := r.sbyte()
		h.key = &k
		r.skip(1)
	}
	if flags&0x03 != 0 {
		r.skip(4) // the beaming of the new meter
	}
	if flags&0x10 != 0 {
		h.alt = r.ubyte()
	} else {
		r.skip(1)
	}
	r.sbyte() // triplet feel, bar by bar
	return h
}

// endingsFromCount turns the older files' ending figure into the bits the
// newer ones write.
//
// GP3 and GP4 write how many endings the repeat has so far rather than
// which this bar is: a second-time bar is written "2". Which it is, then,
// is every ending up to that count that no bar since the repeat opened has
// claimed.
func endingsFromCount(before []header, count int) int {
	used := 0
	for _, h := range before {
		if h.repeatOpen {
			used = 0
		}
		used |= h.alt
	}
	alt := 0
	for i := 0; i < 8; i++ {
		if count > i && used&(1<<i) == 0 {
			alt |= 1 << i
		}
	}
	return alt
}

func (r *reader) readTrack(sg *song, number int) track {
	var t track
	flags := r.ubyte()
	if sg.version == 5 && (number == 1 || sg.code == 0) {
		r.skip(1)
	}
	t.name = r.strByte(40)
	n := r.int32()
	for i := 0; i < 7; i++ {
		tuning := r.int32()
		if i < n {
			t.strings = append(t.strings, tuning)
		}
	}
	r.int32() // the MIDI port
	t.channel = r.int32()
	r.int32() // the effects channel
	t.frets = r.int32()
	t.capo = r.int32()
	r.skip(4) // its colour
	if sg.version == 5 {
		r.skip(5)
		t.bassClef = r.int32() == 12
		if sg.code > 0 {
			r.skip(40)
			r.strIntByte()
			r.strIntByte()
		} else {
			r.skip(35)
		}
	}
	// The drum channel is what makes a drum track. GP5's track flags have
	// a bit that looks like it should, and Guitar Pro's own demo sets it on
	// the bass.
	_ = flags
	t.percussion = t.channel == 10
	if t.channel >= 1 && t.channel <= len(sg.channels) {
		t.program = sg.channels[t.channel-1].program
	}
	if t.frets <= 0 || t.frets > 36 {
		t.frets = 24
	}
	return t
}

// maxBeats is more beats than a bar can hold: sixty-fourths in a bar of
// 32/4, with room to spare.
const maxBeats = 4096

func (r *reader) readMeasure(sg *song, t *track) measure {
	var m measure
	voices := 1
	if sg.version == 5 {
		voices = 2
	}
	for v := 0; v < voices && r.err == nil; v++ {
		count := r.int32()
		if count < 0 || count > maxBeats {
			r.fail(fmt.Errorf("%d beats in a bar", count))
			return m
		}
		var beats []beat
		for i := 0; i < count && r.err == nil; i++ {
			beats = append(beats, r.readBeat(sg, t))
		}
		m.voices = append(m.voices, beats)
	}
	return m
}

func (r *reader) readBeat(sg *song, t *track) beat {
	b := beat{tempo: -1}
	flags := r.ubyte()
	if flags&0x40 != 0 {
		// The status byte: 2 is a rest, and 0 a beat with nothing in it,
		// which is how a bar with nothing in it is written — one empty
		// quarter, to be read as a bar of rest.
		kind := r.ubyte()
		b.kind = kind
		b.empty = kind&0x02 == 0
		b.rest = true
	}
	// The length is a power of two counted from the quarter: -2 is a
	// whole, 0 a quarter, 4 a sixty-fourth.
	d := r.sbyte()
	if d < -2 || d > 4 {
		d = 0
	}
	b.dur = 1 << (d + 2)
	b.dot = flags&0x01 != 0
	if flags&0x20 != 0 {
		b.tuplet = r.int32()
	}
	if flags&0x02 != 0 {
		b.chord = r.readChord(sg, len(t.strings))
	}
	if flags&0x04 != 0 {
		b.text = r.strIntByte()
	}
	if flags&0x08 != 0 {
		r.readBeatEffects(sg, &b)
	}
	if flags&0x10 != 0 {
		b.tempo = r.readMixChange(sg)
	}
	strings := r.ubyte()
	for i := 6; i >= 0; i-- {
		if strings&(1<<i) != 0 && 6-i < len(t.strings) {
			n := r.readNote(sg, 7-i)
			if b.natural && n.harmonic == 0 {
				n.harmonic = 1
			}
			if b.artificial && n.harmonic == 0 {
				n.harmonic = 2
			}
			b.notes = append(b.notes, n)
		}
	}
	if len(b.notes) > 0 {
		b.rest = false
	}
	if sg.version == 5 {
		r.skip(1)
		if r.ubyte()&0x08 != 0 {
			r.skip(1)
		}
	}
	return b
}

func (r *reader) readChord(sg *song, strings int) *chord {
	c := &chord{}
	readFrets := func(count int) {
		for i := 0; i < count; i++ {
			f := r.int32()
			if i < strings {
				c.frets = append(c.frets, f)
			}
		}
	}
	if sg.version == 5 {
		r.skip(17)
		c.name = r.strByte(21)
		r.skip(4)
		c.firstFret = r.int32()
		readFrets(7)
		r.skip(32)
		return c
	}
	hdr := r.ubyte()
	if hdr&0x01 == 0 {
		c.name = r.strIntByte()
		c.firstFret = r.int32()
		if c.firstFret != 0 {
			readFrets(6)
		}
		return c
	}
	if sg.version == 3 {
		r.skip(25)
		c.name = r.strByte(34)
		c.firstFret = r.int32()
		readFrets(6)
		r.skip(36)
		return c
	}
	r.skip(16)
	c.name = r.strByte(21)
	r.skip(4)
	c.firstFret = r.int32()
	readFrets(7)
	r.skip(32)
	return c
}

func (r *reader) readBeatEffects(sg *song, b *beat) {
	if sg.version == 3 {
		f := r.ubyte()
		b.vib = f&0x03 != 0
		b.fadeIn = f&0x10 != 0
		b.natural = f&0x04 != 0
		b.artificial = f&0x08 != 0
		if f&0x20 != 0 {
			switch r.ubyte() {
			case 0:
				r.int32() // the whammy bar's dip
				b.whammy = true
			case 1:
				b.tap = true
				r.int32()
			case 2:
				b.slap = true
				r.int32()
			default:
				b.pop = true
				r.int32()
			}
		}
		if f&0x40 != 0 {
			down := r.sbyte()
			up := r.sbyte()
			b.stroke = strokeOf(down, up)
		}
		return
	}
	f1 := r.ubyte()
	f2 := r.ubyte()
	b.fadeIn = f1&0x10 != 0
	b.vib = f1&0x02 != 0
	b.rasguedo = f2&0x01 != 0
	if f1&0x20 != 0 {
		switch r.ubyte() {
		case 1:
			b.tap = true
		case 2:
			b.slap = true
		case 3:
			b.pop = true
		}
	}
	if f2&0x04 != 0 {
		r.readBendPoints()
		b.whammy = true
	}
	if f1&0x40 != 0 {
		first := r.sbyte()
		second := r.sbyte()
		if sg.version == 5 {
			b.stroke = strokeOf(second, first)
		} else {
			b.stroke = strokeOf(first, second)
		}
	}
	if f2&0x02 != 0 {
		switch r.sbyte() & 0x03 {
		case 1:
			b.pick = 1
		case 2:
			b.pick = 2
		}
	}
}

func strokeOf(down, up int) int {
	switch {
	case down > 0:
		return 1
	case up > 0:
		return 2
	}
	return 0
}

// readBendPoints reads the curve of a bend or a whammy dip and answers how
// high it gets, in the file's units.
func (r *reader) readBendPoints() (kind, value, peak int) {
	kind = r.sbyte()
	value = r.int32()
	n := r.int32()
	if n < 0 || n > 1000 {
		r.fail(fmt.Errorf("a bend of %d points", n))
		return
	}
	for i := 0; i < n && r.err == nil; i++ {
		r.int32() // position
		v := r.int32()
		r.sbyte() // vibrato
		if v > peak {
			peak = v
		}
	}
	return
}

func (r *reader) readMixChange(sg *song) int {
	r.sbyte() // instrument
	if sg.version == 5 {
		r.skip(16)
	}
	var levels [6]int
	for i := range levels {
		levels[i] = r.sbyte()
	}
	if sg.version == 5 {
		r.strIntByte() // the tempo's name
	}
	tempo := r.int32()
	for _, l := range levels {
		if l >= 0 {
			r.sbyte() // over how many beats
		}
	}
	if tempo >= 0 {
		r.sbyte()
		if sg.version == 5 && sg.code > 0 {
			r.skip(1)
		}
	}
	if sg.version >= 4 {
		r.sbyte() // which of the changes apply to every track
	}
	if sg.version == 5 {
		r.skip(1)
		if sg.code > 0 {
			r.strIntByte()
			r.strIntByte()
		}
	}
	return tempo
}

func (r *reader) readNote(sg *song, str int) note {
	n := note{str: str, finger: -1}
	flags := r.ubyte()
	n.ghost = flags&0x04 != 0
	if sg.version >= 4 {
		n.accent = flags&0x40 != 0
	}
	if sg.version == 5 {
		n.heavy = flags&0x02 != 0
	}
	if flags&0x20 != 0 {
		switch r.ubyte() {
		case 2:
			n.tied = true
		case 3:
			n.dead = true
		}
	}
	if sg.version < 5 && flags&0x01 != 0 {
		r.skip(2) // the note's own length, which the beat already has
	}
	if flags&0x10 != 0 {
		n.velocity = r.sbyte()
	}
	if flags&0x20 != 0 {
		n.fret = r.sbyte()
	}
	if flags&0x80 != 0 {
		left := r.sbyte()
		r.sbyte() // the right hand
		n.finger = left
	}
	if sg.version == 5 {
		if flags&0x01 != 0 {
			r.skip(8) // the note's own length, as a double
		}
		r.skip(1)
	}
	if flags&0x08 != 0 {
		r.readNoteEffects(sg, &n)
	}
	return n
}

func (r *reader) readNoteEffects(sg *song, n *note) {
	if sg.version == 3 {
		f := r.ubyte()
		n.hammer = f&0x02 != 0
		if f&0x04 != 0 {
			n.slide = 1
		}
		n.letRing = f&0x08 != 0
		if f&0x01 != 0 {
			n.bend = r.readBend()
		}
		if f&0x10 != 0 {
			n.grace = r.readGrace(sg)
		}
		return
	}
	f1 := r.ubyte()
	f2 := r.ubyte()
	if f1&0x01 != 0 {
		n.bend = r.readBend()
	}
	if f1&0x10 != 0 {
		n.grace = r.readGrace(sg)
	}
	if f2&0x04 != 0 {
		n.tremolo = r.ubyte()
	}
	if f2&0x08 != 0 {
		n.slide = r.sbyte()
		if n.slide == 0 {
			n.slide = 1
		}
	}
	if f2&0x10 != 0 {
		kind := r.sbyte()
		if sg.version == 5 {
			switch kind {
			case 2:
				r.skip(3) // the note, its accidental and its octave
			case 3:
				r.sbyte() // the right hand's fret
			}
			if kind >= 1 && kind <= 5 {
				n.harmonic = kind
			}
		} else {
			switch kind {
			case 1, 3, 4, 5:
				n.harmonic = kind
			case 15, 17, 22:
				n.harmonic = 2
			}
		}
	}
	if f2&0x20 != 0 {
		t := &trill{}
		t.fret = r.sbyte()
		t.period = r.sbyte()
		n.trill = t
	}
	n.hammer = f1&0x02 != 0
	n.letRing = f1&0x08 != 0
	n.vib = f2&0x40 != 0
	n.palmMute = f2&0x02 != 0
	n.staccato = f2&0x01 != 0
}

func (r *reader) readBend() *bend {
	kind, value, peak := r.readBendPoints()
	if value <= 0 && peak <= 0 && kind <= 0 {
		return nil
	}
	return &bend{kind: kind, value: value, peak: peak}
}

func (r *reader) readGrace(sg *song) *grace {
	g := &grace{}
	fret := r.ubyte()
	r.ubyte() // its dynamic
	g.transition = r.sbyte()
	g.duration = r.ubyte()
	if sg.version == 5 {
		flags := r.ubyte()
		g.dead = flags&0x01 != 0
		g.onBeat = flags&0x02 != 0
	} else {
		g.dead = fret == 255
	}
	if !g.dead {
		g.fret = fret
	}
	return g
}
