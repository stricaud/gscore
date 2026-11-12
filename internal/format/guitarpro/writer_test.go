/*
 * writer_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A Guitar Pro writer, for the tests only.
 *
 * The files in the wild are somebody's transcriptions and are not free to
 * copy, so the tests write their own: a song is put together as the reader's
 * own structs, written out in any of the three dialects, and read back. The
 * writer is the reader's mirror, byte for byte, which also makes it the
 * plainest statement of what the reader expects to find.
 */

package guitarpro

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

type writer struct {
	buf  bytes.Buffer
	v    int
	code int
}

func (w *writer) u8(b int)    { w.buf.WriteByte(byte(b)) }
func (w *writer) i8(b int)    { w.buf.WriteByte(byte(int8(b))) }
func (w *writer) zeros(n int) { w.buf.Write(make([]byte, n)) }
func (w *writer) i32(n int) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(int32(n)))
	w.buf.Write(b[:])
}

// strIntByte is the header's shape: an int of the field's size, a byte of
// the length, the text.
func (w *writer) strIntByte(s string) {
	w.i32(len(s) + 1)
	w.u8(len(s))
	w.buf.WriteString(s)
}

// strByte is a byte of length and then a field of size bytes.
func (w *writer) strByte(size int, s string) {
	if len(s) > size {
		s = s[:size]
	}
	w.u8(len(s))
	w.buf.WriteString(s)
	w.zeros(size - len(s))
}

func (w *writer) strInt(s string) {
	w.i32(len(s))
	w.buf.WriteString(s)
}

// banner is the version line each dialect opens with.
func banner(v, code int) string {
	switch {
	case v == 3:
		return "FICHIER GUITAR PRO v3.00"
	case v == 4 && code == 0:
		return "FICHIER GUITAR PRO v4.00"
	case v == 4:
		return "FICHIER GUITAR PRO v4.06"
	case v == 5 && code == 0:
		return "FICHIER GUITAR PRO v5.00"
	}
	return "FICHIER GUITAR PRO v5.10"
}

// write turns a song into a file in the dialect asked for.
func write(sg *song, v, code int) []byte {
	w := &writer{v: v, code: code}
	w.strByte(30, banner(v, code))

	w.strIntByte(sg.title)
	w.strIntByte(sg.subtitle)
	w.strIntByte(sg.artist)
	w.strIntByte(sg.album)
	if v == 5 {
		w.strIntByte(sg.words)
	}
	w.strIntByte(sg.music)
	w.strIntByte(sg.copyright)
	w.strIntByte("") // tabber
	w.strIntByte("") // instructions
	w.i32(0)         // comments

	if v < 5 {
		w.u8(0) // triplet feel
	}
	if v >= 4 {
		w.i32(sg.lyricTrack)
		for i := 0; i < 5; i++ {
			if i < len(sg.lyrics) {
				w.i32(sg.lyrics[i].from)
				w.strInt(sg.lyrics[i].text)
			} else {
				w.i32(1)
				w.strInt("")
			}
		}
	}
	if v == 5 {
		if code > 0 {
			w.zeros(49)
		} else {
			w.zeros(30)
		}
		for i := 0; i < 11; i++ {
			w.zeros(4)
			w.u8(0)
		}
	}
	w.i32(sg.tempo)
	if v == 5 && code > 0 {
		w.u8(0)
	}
	w.i8(sg.key)
	w.zeros(3)
	if v >= 4 {
		w.u8(0) // octave
	}
	for i := range sg.channels {
		w.i32(sg.channels[i].program)
		w.zeros(8)
	}
	if v == 5 {
		w.zeros(42)
	}
	w.i32(len(sg.headers))
	w.i32(len(sg.tracks))

	prevNum, prevDen := 0, 0
	for i, h := range sg.headers {
		if v == 5 && i > 0 {
			w.u8(0)
		}
		flags := 0
		if h.num != prevNum {
			flags |= 0x01
		}
		if h.den != prevDen {
			flags |= 0x02
		}
		if h.repeatOpen {
			flags |= 0x04
		}
		if h.repeatClose > 0 {
			flags |= 0x08
		}
		if h.alt != 0 {
			flags |= 0x10
		}
		if h.marker != "" {
			flags |= 0x20
		}
		if h.key != nil {
			flags |= 0x40
		}
		w.u8(flags)
		if flags&0x01 != 0 {
			w.i8(h.num)
		}
		if flags&0x02 != 0 {
			w.i8(h.den)
		}
		prevNum, prevDen = h.num, h.den
		if flags&0x08 != 0 {
			if v == 5 {
				w.u8(h.repeatClose + 1)
			} else {
				w.i8(h.repeatClose)
			}
		}
		if v < 5 {
			if flags&0x10 != 0 {
				// The older files count endings: the highest this bar is
				// in is what they write.
				count := 0
				for b := 0; b < 8; b++ {
					if h.alt&(1<<b) != 0 {
						count = b + 1
					}
				}
				w.u8(count)
			}
			if flags&0x20 != 0 {
				w.strIntByte(h.marker)
				w.zeros(4)
			}
			if flags&0x40 != 0 {
				w.i8(*h.key)
				w.u8(0)
			}
			continue
		}
		if flags&0x20 != 0 {
			w.strIntByte(h.marker)
			w.zeros(4)
		}
		if flags&0x40 != 0 {
			w.i8(*h.key)
			w.u8(0)
		}
		if flags&0x03 != 0 {
			w.zeros(4)
		}
		if flags&0x10 != 0 {
			w.u8(h.alt)
		} else {
			w.u8(0)
		}
		w.u8(0) // triplet feel
	}

	for i, t := range sg.tracks {
		w.u8(0)
		if v == 5 && (i == 0 || code == 0) {
			w.u8(0)
		}
		w.strByte(40, t.name)
		w.i32(len(t.strings))
		for s := 0; s < 7; s++ {
			if s < len(t.strings) {
				w.i32(t.strings[s])
			} else {
				w.i32(0)
			}
		}
		w.i32(1) // port
		w.i32(t.channel)
		w.i32(t.channel)
		w.i32(t.frets)
		w.i32(t.capo)
		w.zeros(4)
		if v == 5 {
			w.zeros(5)
			if t.bassClef {
				w.i32(12)
			} else {
				w.i32(0)
			}
			if code > 0 {
				w.zeros(40)
				w.strIntByte("")
				w.strIntByte("")
			} else {
				w.zeros(35)
			}
		}
	}
	if v == 5 {
		if code == 0 {
			w.zeros(2)
		} else {
			w.u8(1)
		}
	}

	for bar := range sg.headers {
		for ti := range sg.tracks {
			t := &sg.tracks[ti]
			var m measure
			if bar < len(t.measures) {
				m = t.measures[bar]
			}
			voices := 1
			if v == 5 {
				voices = 2
			}
			for vi := 0; vi < voices; vi++ {
				var beats []beat
				if vi < len(m.voices) {
					beats = m.voices[vi]
				}
				if len(beats) == 0 {
					// What the file writes for nothing: one empty quarter.
					beats = []beat{{empty: true, rest: true, dur: 4, tempo: -1}}
				}
				w.i32(len(beats))
				for _, b := range beats {
					w.beat(t, b)
				}
			}
			if v == 5 {
				w.u8(0)
			}
		}
	}
	return w.buf.Bytes()
}

func (w *writer) beat(t *track, b beat) {
	flags := 0
	if b.dot {
		flags |= 0x01
	}
	if b.chord != nil {
		flags |= 0x02
	}
	if b.text != "" {
		flags |= 0x04
	}
	effects := b.tap || b.slap || b.pop || b.vib || b.fadeIn || b.natural ||
		b.artificial || b.stroke != 0 || b.pick != 0 || b.whammy || b.rasguedo
	if effects {
		flags |= 0x08
	}
	if b.tempo >= 0 {
		flags |= 0x10
	}
	if b.tuplet != 0 {
		flags |= 0x20
	}
	if b.rest || b.empty || len(b.notes) == 0 {
		flags |= 0x40
	}
	w.u8(flags)
	if flags&0x40 != 0 {
		if b.empty {
			w.u8(0)
		} else {
			w.u8(2)
		}
	}
	d := -2
	for dd := 1; dd < b.dur; dd *= 2 {
		d++
	}
	w.i8(d)
	if flags&0x20 != 0 {
		w.i32(b.tuplet)
	}
	if flags&0x02 != 0 {
		w.chord(b.chord)
	}
	if flags&0x04 != 0 {
		w.strIntByte(b.text)
	}
	if flags&0x08 != 0 {
		w.beatEffects(b)
	}
	if flags&0x10 != 0 {
		w.mixChange(b.tempo)
	}
	strings := 0
	for _, n := range b.notes {
		strings |= 1 << (7 - n.str)
	}
	w.u8(strings)
	for s := 1; s <= 7; s++ {
		for _, n := range b.notes {
			if n.str == s {
				w.note(n)
			}
		}
	}
	if w.v == 5 {
		w.u8(0)
		w.u8(0)
	}
}

func (w *writer) chord(c *chord) {
	if w.v == 5 {
		w.zeros(17)
		w.strByte(21, c.name)
		w.zeros(4)
		w.i32(c.firstFret)
		for i := 0; i < 7; i++ {
			if i < len(c.frets) {
				w.i32(c.frets[i])
			} else {
				w.i32(-1)
			}
		}
		w.zeros(32)
		return
	}
	w.u8(0) // the old shape
	w.strIntByte(c.name)
	w.i32(c.firstFret)
	if c.firstFret != 0 {
		for i := 0; i < 6; i++ {
			if i < len(c.frets) {
				w.i32(c.frets[i])
			} else {
				w.i32(-1)
			}
		}
	}
}

func (w *writer) beatEffects(b beat) {
	if w.v == 3 {
		f := 0
		if b.vib {
			f |= 0x01
		}
		if b.natural {
			f |= 0x04
		}
		if b.artificial {
			f |= 0x08
		}
		if b.fadeIn {
			f |= 0x10
		}
		if b.tap || b.slap || b.pop || b.whammy {
			f |= 0x20
		}
		if b.stroke != 0 {
			f |= 0x40
		}
		w.u8(f)
		if f&0x20 != 0 {
			switch {
			case b.whammy:
				w.u8(0)
				w.i32(100)
			case b.tap:
				w.u8(1)
				w.i32(0)
			case b.slap:
				w.u8(2)
				w.i32(0)
			default:
				w.u8(3)
				w.i32(0)
			}
		}
		if f&0x40 != 0 {
			w.strokes(b.stroke, false)
		}
		return
	}
	f1, f2 := 0, 0
	if b.vib {
		f1 |= 0x02
	}
	if b.fadeIn {
		f1 |= 0x10
	}
	if b.tap || b.slap || b.pop {
		f1 |= 0x20
	}
	if b.stroke != 0 {
		f1 |= 0x40
	}
	if b.rasguedo {
		f2 |= 0x01
	}
	if b.pick != 0 {
		f2 |= 0x02
	}
	if b.whammy {
		f2 |= 0x04
	}
	w.u8(f1)
	w.u8(f2)
	if f1&0x20 != 0 {
		switch {
		case b.tap:
			w.u8(1)
		case b.slap:
			w.u8(2)
		default:
			w.u8(3)
		}
	}
	if f2&0x04 != 0 {
		w.i8(6)
		w.i32(100)
		w.i32(0)
	}
	if f1&0x40 != 0 {
		w.strokes(b.stroke, w.v == 5)
	}
	if f2&0x02 != 0 {
		w.i8(b.pick)
	}
}

// strokes writes a brush as its two bytes, down then up — or up then down
// in GP5, which turned them round.
func (w *writer) strokes(stroke int, upFirst bool) {
	down, up := 0, 0
	if stroke == 1 {
		down = 4
	} else {
		up = 4
	}
	if upFirst {
		w.i8(up)
		w.i8(down)
	} else {
		w.i8(down)
		w.i8(up)
	}
}

func (w *writer) mixChange(tempo int) {
	w.i8(-1)
	if w.v == 5 {
		w.zeros(16)
	}
	for i := 0; i < 6; i++ {
		w.i8(-1)
	}
	if w.v == 5 {
		w.strIntByte("")
	}
	w.i32(tempo)
	if tempo >= 0 {
		w.u8(0)
		if w.v == 5 && w.code > 0 {
			w.u8(0)
		}
	}
	if w.v >= 4 {
		w.u8(0)
	}
	if w.v == 5 {
		w.u8(0)
		if w.code > 0 {
			w.strIntByte("")
			w.strIntByte("")
		}
	}
}

func (w *writer) note(n note) {
	flags := 0x20 // the kind of note is always said
	if n.ghost {
		flags |= 0x04
	}
	if n.accent && w.v >= 4 {
		flags |= 0x40
	}
	if n.heavy && w.v == 5 {
		flags |= 0x02
	}
	if n.velocity > 0 {
		flags |= 0x10
	}
	if n.finger >= 0 {
		flags |= 0x80
	}
	effects := n.hammer || n.slide != 0 || n.letRing || n.bend != nil || n.grace != nil ||
		n.tremolo != 0 || n.harmonic != 0 || n.trill != nil || n.vib || n.palmMute || n.staccato
	if effects {
		flags |= 0x08
	}
	w.u8(flags)
	switch {
	case n.tied:
		w.u8(2)
	case n.dead:
		w.u8(3)
	default:
		w.u8(1)
	}
	if flags&0x10 != 0 {
		w.i8(n.velocity)
	}
	w.i8(n.fret)
	if flags&0x80 != 0 {
		w.i8(n.finger)
		w.i8(-1)
	}
	if w.v == 5 {
		w.u8(0)
	}
	if flags&0x08 != 0 {
		w.noteEffects(n)
	}
}

func (w *writer) bend(b *bend) {
	w.i8(b.kind)
	w.i32(b.value)
	w.i32(1)
	w.i32(30)
	w.i32(b.peak)
	w.u8(0)
}

func (w *writer) grace(g *grace) {
	if g.dead && w.v < 5 {
		w.u8(255)
	} else {
		w.u8(g.fret)
	}
	w.u8(6)
	w.i8(g.transition)
	w.u8(g.duration)
	if w.v == 5 {
		f := 0
		if g.dead {
			f |= 0x01
		}
		if g.onBeat {
			f |= 0x02
		}
		w.u8(f)
	}
}

func (w *writer) noteEffects(n note) {
	if w.v == 3 {
		f := 0
		if n.bend != nil {
			f |= 0x01
		}
		if n.hammer {
			f |= 0x02
		}
		if n.slide != 0 {
			f |= 0x04
		}
		if n.letRing {
			f |= 0x08
		}
		if n.grace != nil {
			f |= 0x10
		}
		w.u8(f)
		if n.bend != nil {
			w.bend(n.bend)
		}
		if n.grace != nil {
			w.grace(n.grace)
		}
		return
	}
	f1, f2 := 0, 0
	if n.bend != nil {
		f1 |= 0x01
	}
	if n.hammer {
		f1 |= 0x02
	}
	if n.letRing {
		f1 |= 0x08
	}
	if n.grace != nil {
		f1 |= 0x10
	}
	if n.staccato {
		f2 |= 0x01
	}
	if n.palmMute {
		f2 |= 0x02
	}
	if n.tremolo != 0 {
		f2 |= 0x04
	}
	if n.slide != 0 {
		f2 |= 0x08
	}
	if n.harmonic != 0 {
		f2 |= 0x10
	}
	if n.trill != nil {
		f2 |= 0x20
	}
	if n.vib {
		f2 |= 0x40
	}
	w.u8(f1)
	w.u8(f2)
	if n.bend != nil {
		w.bend(n.bend)
	}
	if n.grace != nil {
		w.grace(n.grace)
	}
	if n.tremolo != 0 {
		w.u8(n.tremolo)
	}
	if n.slide != 0 {
		w.i8(n.slide)
	}
	if n.harmonic != 0 {
		if w.v == 5 {
			w.i8(n.harmonic)
			switch n.harmonic {
			case 2:
				w.zeros(3)
			case 3:
				w.u8(12)
			}
		} else if n.harmonic == 2 {
			w.i8(15)
		} else {
			w.i8(n.harmonic)
		}
	}
	if n.trill != nil {
		w.i8(n.trill.fret)
		w.i8(n.trill.period)
	}
}

/*
 * Songs the tests are made of.
 */

// standard is a guitar's six strings, highest first, as the file has them.
var standard = []int{64, 59, 55, 50, 45, 40}

// guitar is a track with nothing in it yet.
func guitar(name string) track {
	return track{name: name, strings: standard, channel: 1, frets: 24}
}

// bar makes a one-voice bar out of its beats.
func bar(beats ...beat) measure { return measure{voices: [][]beat{beats}} }

// q is a quarter note on one string.
func q(str, fret int) beat {
	return beat{dur: 4, tempo: -1, notes: []note{{str: str, fret: fret, finger: -1}}}
}

// e is an eighth.
func e(str, fret int) beat {
	return beat{dur: 8, tempo: -1, notes: []note{{str: str, fret: fret, finger: -1}}}
}

// rest is a rest of the given denominator.
func rest(den int) beat { return beat{dur: den, rest: true, tempo: -1} }

// with changes the beat's one note.
func with(b beat, f func(n *note)) beat {
	f(&b.notes[0])
	return b
}

// fourFour is as many bars of 4/4 as asked.
func fourFour(n int) []header {
	out := make([]header, n)
	for i := range out {
		out[i] = header{num: 4, den: 4}
	}
	return out
}

func newSong(bars int, tracks ...track) *song {
	sg := &song{title: "Test", tempo: 120, headers: fourFour(bars), tracks: tracks}
	sg.channels[0].program = 25
	sg.channels[1].program = 33
	return sg
}

// readBack writes the song in one dialect and reads it.
func readBack(t *testing.T, sg *song, v, code int) (*model.Score, error) {
	t.Helper()
	s, err := Import(bytes.NewReader(write(sg, v, code)))
	if s == nil {
		t.Fatalf("v%d.%d: no score: %v", v, code, err)
	}
	return s, err
}

// every runs a test in each of the five dialects.
func every(t *testing.T, f func(t *testing.T, v, code int)) {
	for _, d := range [][2]int{{3, 0}, {4, 0}, {4, 1}, {5, 0}, {5, 1}} {
		t.Run(fmt.Sprintf("v%d.%d", d[0], d[1]), func(t *testing.T) { f(t, d[0], d[1]) })
	}
}

// elements is the first voice of a bar of the first part.
func elements(s *model.Score, part, bar int) []model.Element {
	return s.Parts[part].Measures[bar].Voices[0].Elements
}

// notesOf is every note in a bar, in chords or on their own.
func notesOf(els []model.Element) []*model.Note {
	var out []*model.Note
	for _, el := range els {
		switch e := el.(type) {
		case *model.Note:
			out = append(out, e)
		case *model.Chord:
			out = append(out, e.Notes...)
		}
	}
	return out
}

func hasTechnique(n *model.Note, want model.Technique) bool {
	for _, tc := range n.Techniques {
		if tc == want {
			return true
		}
	}
	return false
}

func techniques(n *model.Note) string {
	var names []string
	for _, tc := range n.Techniques {
		names = append(names, tc.String())
	}
	return strings.Join(names, ",")
}
