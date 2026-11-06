/*
 * mus_test.go
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
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pkware"
)

/*
 * A .mus is somebody's music, so there are none in this repository to read.
 * These tests write one.
 *
 * That is weaker than reading a real file in one specific way — it cannot
 * catch a field this reader puts at the wrong offset, because the fixture
 * puts it at the same wrong offset — and it is not weak in any other. The
 * framing, the byte order, the two containers, the slot walk, the dead-slot
 * pruning, the sign-and-magnitude alteration and every refusal are all real
 * here. What is pinned against something outside this package is the one part
 * no fixture can vouch for: the compression, in internal/pkware, against the
 * vector its format's own reference implementation documents.
 *
 * Where a layout is taken on trust it is named as such in musdoc.go, so that
 * a real file disagreeing with it has somewhere obvious to point.
 */

// musBuild is a .mus under construction.
type musBuild struct {
	order   binary.ByteOrder
	era     musEra
	year    int
	title   string
	others  [][]byte
	details [][]byte
	entries []byte
	text    string
}

// A file from a PC, which is most of them.
func newMus() *musBuild {
	return &musBuild{order: binary.LittleEndian, era: eraDCL, year: 2001}
}

// otherRow adds one 16-byte row to the older era's record pool.
func (b *musBuild) otherRow(cmper int, tag string, data []byte) *musBuild {
	row := make([]byte, musRow)
	b.order.PutUint16(row, uint16(cmper))
	copy(row[2:], b.rawTag(tag))
	copy(row[4:], data)
	b.others = append(b.others, row)
	return b
}

func (b *musBuild) detailRow(c1, c2 int, tag string, data []byte) *musBuild {
	row := make([]byte, musRow)
	b.order.PutUint16(row, uint16(c1))
	b.order.PutUint16(row[2:], uint16(c2))
	copy(row[4:], b.rawTag(tag))
	copy(row[6:], data)
	b.details = append(b.details, row)
	return b
}

// rawTag lays a two-character tag down the way the format does: as a number
// holding the pair, so that on a file written little end first the characters
// come out back to front.
func (b *musBuild) rawTag(tag string) []byte {
	if b.order == binary.LittleEndian {
		return []byte{tag[1], tag[0]}
	}
	return []byte{tag[0], tag[1]}
}

// u16s lays out a row's data as numbers, which is what the pools hold.
func (b *musBuild) u16s(vals ...int) []byte {
	out := make([]byte, 2*len(vals))
	for i, v := range vals {
		b.order.PutUint16(out[2*i:], uint16(v))
	}
	return out
}

func (b *musBuild) u32s(vals ...int) []byte {
	out := make([]byte, 4*len(vals))
	for i, v := range vals {
		b.order.PutUint32(out[4*i:], uint32(v))
	}
	return out
}

// A measure: four beats of a quarter note each, unless told otherwise.
func (b *musBuild) bar(n, beats, div int) *musBuild {
	return b.otherRow(n, "MS", b.u16s(600, 0, beats, div, 0, 0))
}

// A staff. Three rows, which is what a 2001 file writes and what says where
// a frame hold keeps its layers.
func (b *musBuild) staff(n, transposition, name int) *musBuild {
	data := make([]byte, 36)
	b.order.PutUint16(data[musStaffTrans:], uint16(transposition))
	b.order.PutUint16(data[musStaffName:], uint16(name))
	for i := 0; i < 3; i++ {
		b.otherRow(n, "IS", data[12*i:12*(i+1)])
	}
	return b
}

func (b *musBuild) frame(n, start, end int) *musBuild {
	return b.otherRow(n, "FR", b.u32s(start, end, 0))
}

// A frame hold: which frame fills a staff in a measure. The layers start at
// +4 in a file whose staff records are three rows.
func (b *musBuild) hold(staff, bar, frame int) *musBuild {
	data := make([]byte, 10)
	b.order.PutUint16(data[musFrameBase2001:], uint16(frame))
	return b.detailRow(staff, bar, "GF", data)
}

func (b *musBuild) used(staves ...int) *musBuild {
	for _, s := range staves {
		b.otherRow(0, "IU", b.u16s(s, 0, 0, 0, 0, 0))
	}
	return b
}

// musNoteOf is one six-byte note record.
func (b *musBuild) note(harmLev, harmAlt int, tie uint32) []byte {
	alt := harmAlt
	if alt < 0 {
		alt = -alt | musTCDAltBelow
	}
	out := make([]byte, musNote)
	b.order.PutUint16(out, uint16(harmLev<<musTCDAltBits|alt))
	// Every legal note record carries the set bit, and a note id above it.
	b.order.PutUint32(out[2:], 0x80010000|tie)
	return out
}

// entryOf writes one entry and however many slots it needs.
func (b *musBuild) entry(num, next, dura int, flag uint32, notes ...[]byte) *musBuild {
	slots := make([]byte, musSlot*musSlotSpan(len(notes)))

	b.order.PutUint32(slots, uint32(num))
	b.order.PutUint16(slots[4:], 0)
	b.order.PutUint32(slots[10:], uint32(next))
	b.order.PutUint16(slots[14:], uint16(dura))
	b.order.PutUint32(slots[18:], flag)
	b.order.PutUint16(slots[24:], uint16(len(notes)))

	for i, at := range musNoteOffsets(0, len(notes)) {
		copy(slots[at:], notes[i])
	}
	// The slots an entry runs on into carry its number and their own index.
	for s := 1; s < musSlotSpan(len(notes)); s++ {
		b.order.PutUint32(slots[musSlot*s:], uint32(num))
		b.order.PutUint16(slots[musSlot*s+4:], uint16(s))
	}

	b.entries = append(b.entries, slots...)
	return b
}

// A plain note at the given place on the scale.
func (b *musBuild) sounds(num, next, dura, harmLev int) *musBuild {
	return b.entry(num, next, dura, musSetBit|musNoteBit, b.note(harmLev, 0, 0))
}

// bytes assembles the file.
func (b *musBuild) bytes() []byte {
	head := make([]byte, musContainerAt)
	copy(head, musMagic)
	copy(head[musBannerAt:], fmt.Sprintf("Finale(R) %d for Testing", b.year))
	if b.title != "" {
		copy(head[musTitleAt:], append([]byte(b.title), 0))
	}

	pools := [][]byte{
		bytes.Join(b.others, nil),
		bytes.Join(b.details, nil),
		b.entries,
		[]byte(b.text),
	}
	kinds := []int{poolOthers, poolDetails, poolEntries, poolText}

	out := head
	for i, pool := range pools {
		if b.era == eraDCL {
			out = append(out, b.dclRecord(kinds[i], pool)...)
			continue
		}
		if i > 0 {
			// The newer container keeps the same ten-byte header between
			// its streams, and the walk steps over it.
			out = append(out, b.header(kinds[i], 0)...)
		}
		out = append(out, b.deflate(pool)...)
	}
	return out
}

func (b *musBuild) header(kind, length int) []byte {
	head := make([]byte, musRecordHeader)
	b.order.PutUint16(head, uint16(kind))
	b.order.PutUint32(head[2:], uint32(length))
	return head
}

func (b *musBuild) dclRecord(kind int, pool []byte) []byte {
	if len(pool) == 0 {
		// An empty pool is its kind and its length and nothing else.
		head := make([]byte, musEmptyRecord)
		b.order.PutUint16(head, uint16(kind))
		b.order.PutUint32(head[2:], musEmptyRecord)
		return head
	}
	stream := pkware.Literals(pool)
	return append(b.header(kind, musRecordHeader+len(stream)), stream...)
}

func (b *musBuild) deflate(pool []byte) []byte {
	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	zw.Write(pool)
	zw.Close()
	return out.Bytes()
}

// aTune is the smallest thing that is a score: one staff, two bars of four
// quarter notes, C D E F and G A B C.
func aTune(b *musBuild) *musBuild {
	b.title = "A Tune"
	b.staff(1, 0, 1).used(1)
	b.bar(1, 4, 1024).bar(2, 4, 1024)
	b.frame(1, 1, 4).frame(2, 5, 8)
	b.hold(1, 1, 1).hold(1, 2, 2)
	for i, step := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
		next := i + 2
		if i == 3 || i == 7 {
			next = 0
		}
		b.sounds(i+1, next, 1024, step)
	}
	b.text = "^block(1)^font(Times,8191)^size(24)Flute^end"
	return b
}

func readFixture(t *testing.T, b *musBuild) *model.Score {
	t.Helper()
	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if score == nil {
		t.Fatal("no score and no error")
	}
	t.Logf("left out: %v", skipped)
	return score
}

// The notes that went in are the notes that come out, from the older
// container.
func TestAnOldFinaleFileOpens(t *testing.T) {
	score := readFixture(t, aTune(newMus()))

	if len(score.Parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(score.Parts))
	}
	if score.Identity.Title != "A Tune" {
		t.Errorf("the title is %q, want %q", score.Identity.Title, "A Tune")
	}

	part := score.Parts[0]
	if len(part.Measures) != 2 {
		t.Fatalf("got %d measures, want 2", len(part.Measures))
	}

	var got []string
	for _, m := range part.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				n, ok := el.(*model.Note)
				if !ok {
					t.Fatalf("got a %T in the music, want a note", el)
				}
				got = append(got, n.Pitch.String())
			}
		}
	}
	if want := "C4 D4 E4 F4 G4 A4 B4 C5"; strings.Join(got, " ") != want {
		t.Errorf("the music reads %q, want %q", strings.Join(got, " "), want)
	}
}

// The same music written by a Mac, which puts its numbers the other way
// round. Nothing in the file says which way it is; the reader works it out,
// and if it did not the notes would be nonsense rather than slightly wrong.
func TestTheByteOrderIsReadOffTheFile(t *testing.T) {
	pc := newMus()
	mac := newMus()
	mac.order = binary.BigEndian
	mac.year = 2005

	one := readFixture(t, aTune(pc))
	two := readFixture(t, aTune(mac))

	first := func(s *model.Score) string {
		var out []string
		for _, m := range s.Parts[0].Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if n, ok := el.(*model.Note); ok {
						out = append(out, n.Pitch.String())
					}
				}
			}
		}
		return strings.Join(out, " ")
	}
	if a, b := first(one), first(two); a != b {
		t.Errorf("a Mac file reads %q and a PC file %q", b, a)
	}
}

// A 2005 file keeps its frames two bytes further into the record than a 2001
// one, and what says which is how many rows a staff record takes. A staff of
// six rows must therefore be read with the later offset.
func TestTheNewerFrameOffsetOfA2005File(t *testing.T) {
	b := newMus()
	b.year = 2005

	// Six rows, which is what 2005 writes.
	data := make([]byte, 72)
	for i := 0; i < 6; i++ {
		b.otherRow(1, "IS", data[12*i:12*(i+1)])
	}
	b.used(1).bar(1, 4, 1024).frame(1, 1, 1)
	hold := make([]byte, 10)
	b.order.PutUint16(hold[musFrameBase2005:], 1)
	b.detailRow(1, 1, "GF", hold)
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if len(score.Parts) != 1 || len(score.Parts[0].Measures) != 1 {
		t.Fatalf("got %d parts", len(score.Parts))
	}
	if n := countNotes(score); n != 1 {
		t.Errorf("got %d notes, want 1 — the frames were read at the wrong offset", n)
	}
}

// The newer container: a chain of zlib streams rather than four compressed
// pool records, and nothing in it saying which pool is which.
func TestANewerFinaleFileOpens(t *testing.T) {
	b := newMus()
	b.era = eraZlib
	b.year = 2012

	// That era frames its records rather than tabulating them, so the
	// fixture has to speak the other framing.
	b.others = nil
	b.details = nil
	b.record(&b.others, musRecOthersHead, 231, 1, 0, make([]byte, 36))
	b.record(&b.others, musRecOthersHead, 176, 1, 0, b.u16s(600, 0, 4, 1024, 0, 0))
	b.record(&b.others, musRecOthersHead, 146, 1, 0, b.u32s(1, 4))
	hold := make([]byte, 12)
	b.order.PutUint16(hold[musFrameBase2005:], 1)
	b.record(&b.details, musRecDetailsHead, 1044, 1, 1, hold)
	for i, step := range []int{0, 2, 4, 5} {
		next := i + 2
		if i == 3 {
			next = 0
		}
		b.sounds(i+1, next, 1024, step)
	}

	// It must really be read as the newer container, not fall through to
	// the older one and happen to work.
	if _, _, era, err := musPools(b.bytes()); err != nil || era != eraZlib {
		t.Fatalf("read as era %v (%v), want the zlib container", era, err)
	}

	score := readFixture(t, b)
	if len(score.Parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(score.Parts))
	}
	var got []string
	for _, m := range score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					got = append(got, n.Pitch.String())
				}
			}
		}
	}
	if want := "C4 E4 G4 A4"; strings.Join(got, " ") != want {
		t.Errorf("the music reads %q, want %q", strings.Join(got, " "), want)
	}
}

// record writes one of the newer era's self-identifying records, and pads the
// pool out past the size the first stream has to clear.
func (b *musBuild) record(into *[][]byte, head, tag, num, num2 int, payload []byte) {
	rec := make([]byte, head)
	b.order.PutUint16(rec, uint16(tag))
	b.order.PutUint16(rec[2:], uint16(num))
	if head == musRecDetailsHead {
		b.order.PutUint16(rec[4:], uint16(num2))
	}
	b.order.PutUint32(rec[head-4:], uint32(len(payload)))
	rec = append(rec, payload...)
	// The four bytes after a payload are the length of a further block,
	// which is zero on almost every record there is.
	rec = append(rec, make([]byte, musRecExtra)...)
	*into = append(*into, rec)

	// The first stream of this container is found by looking rather than by
	// framing, so it has to inflate to something substantial before it is
	// believed. A real pool is tens of kilobytes; this one is padded with
	// the filler words that separate sections, which the walk steps over.
	if head == musRecOthersHead {
		*into = append(*into, make([]byte, 2*1024))
	}
}

// An entry the frames do not reach is not music. A .mus is written in place,
// so a deleted entry keeps its slot until the file is compacted, and reading
// the pool as a list of the music puts deleted bars back into the score.
func TestDeletedMusicIsNotRead(t *testing.T) {
	b := aTune(newMus())
	// Four more entries, chained to each other and named by nothing.
	for i := range 4 {
		b.sounds(100+i, 101+i, 1024, 0)
	}

	score := readFixture(t, b)
	if n := countNotes(score); n != 8 {
		t.Errorf("got %d notes, want the 8 the frames reach", n)
	}
}

// A rest dragged off the middle of the staff stops being a floating rest and
// stores a note record to say how far it moved. Read by the note bit it
// becomes a note; read by the floating-rest bit it stays a rest.
func TestAMovedRestIsStillARest(t *testing.T) {
	for _, c := range []struct {
		name  string
		flag  uint32
		notes [][]byte
	}{
		{"a floating rest", musSetBit | musFloatRest, [][]byte{nil}},
		{"a rest moved by hand", musSetBit, [][]byte{nil}},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newMus()
			b.staff(1, 0, 0).used(1).bar(1, 4, 1024).frame(1, 1, 1).hold(1, 1, 1)
			notes := make([][]byte, len(c.notes))
			for i := range notes {
				notes[i] = b.note(0, 0, 0)
			}
			b.entry(1, 0, 4096, c.flag, notes...)

			score := readFixture(t, b)
			for _, m := range score.Parts[0].Measures {
				for _, v := range m.Voices {
					for _, el := range v.Elements {
						if _, ok := el.(*model.Rest); !ok {
							t.Errorf("got a %T, want a rest", el)
						}
					}
				}
			}
		})
	}
}

// The alteration in a note record is a sign and a size, not two's
// complement, whatever the 1996 document says. The two readings agree up to a
// sharp and part company at a flat, so getting it wrong survives until
// somebody opens a piece with a flat in it.
func TestTheAlterationIsASignAndASize(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024).frame(1, 1, 2).hold(1, 1, 1)
	// B flat, then F sharp: harmLev 6 lowered, harmLev 3 raised.
	b.entry(1, 2, 2048, musSetBit|musNoteBit, b.note(6, -1, 0))
	b.entry(2, 0, 2048, musSetBit|musNoteBit, b.note(3, 1, 0))

	score := readFixture(t, b)
	var got []string
	for _, m := range score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					got = append(got, n.Pitch.String())
				}
			}
		}
	}
	if want := "Bb4 F#4"; strings.Join(got, " ") != want {
		t.Errorf("the notes read %q, want %q — a two's complement reading of "+
			"the low nibble gives a B double-flat or worse",
			strings.Join(got, " "), want)
	}
}

// A chord of more than two notes runs on into another slot, five notes to a
// slot, and a walk that stepped a fixed 38 bytes would drift out of step with
// the pool from there on.
func TestAChordRunsOnIntoAnotherSlot(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024).frame(1, 1, 2).hold(1, 1, 1)
	// A four-note chord, then one plain note after it. The second entry is
	// only readable if the first one's extra slot was counted.
	b.entry(1, 2, 2048, musSetBit|musNoteBit,
		b.note(0, 0, 0), b.note(2, 0, 0), b.note(4, 0, 0), b.note(7, 0, 0))
	b.sounds(2, 0, 2048, 9)

	score := readFixture(t, b)
	var kinds []string
	for _, m := range score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				switch e := el.(type) {
				case *model.Chord:
					kinds = append(kinds, fmt.Sprintf("chord of %d", len(e.Notes)))
				case *model.Note:
					kinds = append(kinds, "note "+e.Pitch.String())
				default:
					kinds = append(kinds, fmt.Sprintf("%T", el))
				}
			}
		}
	}
	if want := "chord of 4, note E5"; strings.Join(kinds, ", ") != want {
		t.Errorf("the bar reads %q, want %q", strings.Join(kinds, ", "), want)
	}
}

// A tie is a property of a notehead, so it survives on one note of a chord.
func TestTiesComeThrough(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024).frame(1, 1, 2).hold(1, 1, 1)
	b.entry(1, 2, 2048, musSetBit|musNoteBit, b.note(0, 0, musTieStart))
	b.entry(2, 0, 2048, musSetBit|musNoteBit, b.note(0, 0, musTieEnd))

	score := readFixture(t, b)
	var ties []bool
	for _, m := range score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					ties = append(ties, n.TieStart, n.TieStop)
				}
			}
		}
	}
	if len(ties) != 4 || !ties[0] || ties[1] || ties[2] || !ties[3] {
		t.Errorf("the ties read %v, want the first note starting one and the "+
			"second ending it", ties)
	}
}

// A staff takes its name from the pool of words, which a .mus points at
// directly where a .musx goes through a record in between.
func TestAStaffIsNamedFromTheTextPool(t *testing.T) {
	score := readFixture(t, aTune(newMus()))
	if got := score.Parts[0].Name; got != "Flute" {
		t.Errorf("the staff is called %q, want %q", got, "Flute")
	}
}

// The meter comes out of the beats and the length of a beat, and six eighths
// in two dotted beats is 6/8 rather than 2/1536.
func TestTheMeterOfAMus(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 2, 1536).frame(1, 1, 1).hold(1, 1, 1)
	b.entry(1, 0, 3072, musSetBit|musFloatRest, nil)

	score := readFixture(t, b)
	if got := score.Parts[0].TimeAt(0); got.Beats != 6 || got.BeatValue != 8 {
		t.Errorf("the meter is %d/%d, want 6/8", got.Beats, got.BeatValue)
	}
}

// What was left behind is said, because a Finale document is mostly things
// this reader does not read and a score that arrives without its slurs
// should say so rather than look complete.
func TestAMusSaysWhatItDidNotRead(t *testing.T) {
	b := aTune(newMus())
	// A handful of records of kinds this reader has no name for, which is
	// what the marks on the music are stored as.
	for i := range 5 {
		b.otherRow(i+1, "TX", b.u16s(0, 0, 0, 0, 0, 0))
	}
	// And one grace note, which comes through at the wrong length.
	b.entry(99, 0, 1024, musSetBit|musNoteBit|musGraceNote, b.note(0, 0, 0))

	_, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	report := strings.Join(skipped, "\n")
	for _, want := range []string{"other kinds of record", "grace notes"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report is %q, which does not mention %q", report, want)
		}
	}
}

// A document with staves and bars but nothing saying what goes where is an
// empty Finale document, and it has to say so. Built as a score it would be a
// score with no parts, which looks like success to everything downstream.
func TestAnEmptyDocumentIsRefused(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024)

	_, _, err := readMus(b.bytes())
	if err == nil {
		t.Fatal("a document with no music was read as a score")
	}
	if !strings.Contains(err.Error(), "no score in it") {
		t.Errorf("the error is %q, which does not say what is wrong", err)
	}
}

// Music in the pool that nothing places is not a score either, and it is a
// different fault from having no music at all.
func TestMusicNothingPlacesIsRefused(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024)
	// A hold naming a frame that names entries the pool does not hold.
	b.frame(1, 900, 901).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	_, _, err := readMus(b.bytes())
	if err == nil {
		t.Fatal("a document whose music is unreachable was read as a score")
	}
}

func TestWhatIsNotAMusFile(t *testing.T) {
	for _, c := range []struct {
		name string
		in   []byte
		says string
	}{
		{"nothing", nil, "too short"},
		{"a few bytes", []byte("MThd"), "too short"},
		{
			"something else entirely",
			bytes.Repeat([]byte("not a score at all "), 100),
			"not a Finale",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := readMus(c.in)
			if err == nil {
				t.Fatal("this was read as a Finale file")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the error is %q, which does not mention %q", err, c.says)
			}
			if strings.Contains(err.Error(), "musx") {
				t.Errorf("the error is %q, which blames the other format", err)
			}
		})
	}
}

// A file that says it is a Finale file and then is not readable gets a
// different answer from a file that was never one, because the two send
// somebody somewhere different.
func TestAFinaleFileThatCannotBeFollowed(t *testing.T) {
	raw := make([]byte, musContainerAt+64)
	copy(raw, musMagic)
	copy(raw[musBannerAt:], "Finale(R) 2001 for Testing")

	_, _, err := readMus(raw)
	if err == nil {
		t.Fatal("a file with no container was read")
	}
	if !strings.Contains(err.Error(), "is a Finale file") {
		t.Errorf("the error is %q, which does not say it recognised the file", err)
	}
}

// The year chooses which scheme to try first and nothing more. A file whose
// banner cannot be read, or whose year nobody catalogued, still opens if its
// payload does.
func TestAnUnreadableBannerIsNotFatal(t *testing.T) {
	b := aTune(newMus())
	raw := b.bytes()
	// Wipe the banner.
	for i := musBannerAt; i < musBannerAt+musBannerSize; i++ {
		raw[i] = 0
	}

	if _, _, err := readMus(raw); err != nil {
		t.Errorf("a file with no banner was refused: %v", err)
	}
	if y := musYear(raw); y != 0 {
		t.Errorf("musYear found %d in a wiped banner", y)
	}
}

func TestTheYearInTheBanner(t *testing.T) {
	for _, c := range []struct {
		banner string
		want   int
	}{
		{"Finale(R) 2001 for Macintosh", 2001},
		{"Finale(R) 2012", 2012},
		{"Finale(R)  2005 ", 2005},
		{"Sibelius 7", 0},
		{"", 0},
		// The banner field is not cleared when it is rewritten, so a
		// shorter banner can leave the tail of a longer one behind it.
		{"Finale(R) 2005\x00(R) 2012 for Windows", 2005},
	} {
		raw := make([]byte, musContainerAt)
		copy(raw[musBannerAt:], c.banner)
		if got := musYear(raw); got != c.want {
			t.Errorf("%q: got year %d, want %d", c.banner, got, c.want)
		}
	}
}

// The slots have to tile the pool exactly, which is most of what identifies
// the pool in a container that does not name its pools.
func TestTheEntryPoolIsRecognisedByItsShape(t *testing.T) {
	b := newMus()
	b.sounds(1, 2, 1024, 0).sounds(2, 0, 1024, 1)
	pool := b.entries

	if !musLooksLikeEntries(pool, binary.LittleEndian) {
		t.Error("a real entry pool was not recognised")
	}
	if musLooksLikeEntries(pool, binary.BigEndian) {
		t.Error("an entry pool was recognised read the wrong way round, so the " +
			"test cannot tell the byte orders apart")
	}
	if musLooksLikeEntries(pool[:len(pool)-1], binary.LittleEndian) {
		t.Error("a pool that is not a whole number of slots was recognised")
	}
	if musLooksLikeEntries(bytes.Repeat([]byte{0}, musSlot*4), binary.LittleEndian) {
		t.Error("a pool of zeroes was recognised; nothing in it carries the set bit")
	}
	if musLooksLikeEntries(nil, binary.LittleEndian) {
		t.Error("an empty pool was recognised as the entry pool")
	}
}

// An empty pool is a real answer, and it has to come back as one: skipping
// the record would hand every pool after it the wrong name.
func TestAnEmptyPoolKeepsItsPlace(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024).frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)
	// No words at all, so the text pool is written empty.
	b.text = ""

	pools, order, era, err := musPools(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if era != eraDCL || order != binary.LittleEndian {
		t.Errorf("read as era %v, order %v", era, order)
	}
	if len(pools) != 4 {
		t.Fatalf("got %d pools, want 4", len(pools))
	}
	if pools[3].kind != poolText || len(pools[3].data) != 0 {
		t.Errorf("the fourth pool is kind %d with %d bytes, want an empty text pool",
			pools[3].kind, len(pools[3].data))
	}
}

// The two Finale formats must not be able to be mistaken for one another.
func TestTheTwoFinaleFormatsAreSeparate(t *testing.T) {
	var mus musImporter
	var musx importer

	if mus.Name() == musx.Name() {
		t.Errorf("both importers call themselves %q", mus.Name())
	}
	for _, e := range mus.Extensions() {
		for _, f := range musx.Extensions() {
			if e == f {
				t.Errorf("both importers claim %s", e)
			}
		}
	}

	// And a .musx handed to the .mus reader is refused as what it is,
	// rather than being half-read.
	zipped := fixture(t, `<finale><others/><details/><entries/><texts/></finale>`)
	if _, _, err := readMus(zipped); err == nil {
		t.Error("the .mus reader accepted a .musx")
	}

	// The other way round too.
	if _, err := Import(bytes.NewReader(aTune(newMus()).bytes())); err == nil {
		t.Error("the .musx reader accepted a .mus")
	}
}

// Four F sharps in a bar print one sharp.
//
// Finale records how far each note is altered from the key, which is as true
// of the fourth as of the first — it is an alteration, not a sign to print.
// Read as a sign this file printed four sharps, and a chromatic passage from
// Finale came out with one on every note of it.
//
// SpellAccidentals is run by the importer for every format, so this asks the
// question of a real Finale file rather than of a score built by hand.
func TestFourSharpsInABarPrintOne(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024).frame(1, 1, 4).hold(1, 1, 1)
	for i := range 4 {
		next := i + 2
		if i == 3 {
			next = 0
		}
		// harmLev 3 is F, harmAlt 1 is a sharp.
		b.entry(i+1, next, 1024, musSetBit|musNoteBit, b.note(3, 1, 0))
	}

	score, _, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	score.SpellAccidentals()

	sharps, notes := 0, 0
	for _, m := range score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				n, ok := el.(*model.Note)
				if !ok {
					continue
				}
				notes++
				if n.Pitch.Alter != 1 {
					t.Errorf("a note came through altered by %d, want a sharp",
						n.Pitch.Alter)
				}
				if n.Accidental != model.NoAccidental {
					sharps++
				}
			}
		}
	}
	if notes != 4 {
		t.Fatalf("got %d notes, want 4", notes)
	}
	if sharps != 1 {
		t.Errorf("the page shows %d sharps, want 1 — the three after it are "+
			"already sharp, and the file saying so again is not a second sign",
			sharps)
	}
}

// A repeat is a flag on a measure rather than a record of its own, so it
// would be dropped without anything to count — and a piece whose repeats
// have gone is a piece of a different length.
func TestRepeatsAreCounted(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).frame(1, 1, 1).hold(1, 1, 1)
	// A measure with a double barline and a backward repeat on it.
	b.otherRow(1, "MS", b.u16s(600, 0, 4, 1024, 0, 0x24))
	b.sounds(1, 0, 4096, 0)

	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if got := strings.Join(skipped, "\n"); !strings.Contains(got, "repeats") {
		t.Errorf("the report is %q, which does not mention the repeat", got)
	}
	// The top half of the same byte is still read, so finding the repeat
	// must not cost the barline.
	if got := score.Parts[0].Measures[0].RightBarline; got == 0 {
		t.Error("the double barline was lost along with the repeat")
	}
}

// A file this cannot read has to be refused quickly and in bounded memory,
// because somebody opening the wrong file is an ordinary mistake and a
// deliberately bad file is a possibility.
//
// What makes this worth a test rather than an argument: finding where the
// newer container's payload starts means trying every offset in the file,
// and in a megabyte of anything a pair of bytes that looks like a zlib
// header turns up a few thousand times by chance. Inflating each of those in
// full — which is the obvious way to write it — spends seconds and hundreds
// of megabytes on a file that is not a score at all.
func TestNoiseIsRefusedQuickly(t *testing.T) {
	raw := make([]byte, 4<<20)
	rand.New(rand.NewSource(1)).Read(raw)
	// Says it is a Finale file, so nothing takes the cheap way out.
	copy(raw, musMagic)

	done := make(chan error, 1)
	go func() {
		_, _, err := readMus(raw)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("four megabytes of noise was read as a score")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("four megabytes of noise took more than ten seconds to refuse")
	}
}

func countNotes(score *model.Score) int {
	n := 0
	for _, p := range score.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					switch e := el.(type) {
					case *model.Note:
						n++
					case *model.Chord:
						n += len(e.Notes)
					}
				}
			}
		}
	}
	return n
}
