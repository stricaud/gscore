/*
 * mus_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package enigma

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
	return b.staffClefs(n, transposition, name, musClefTreble, musClefTreble)
}

// staffClefs is a staff that is not read from the treble clef: the clef it
// is written in, and the one to read it from instead where it moves its clef
// as it transposes.
func (b *musBuild) staffClefs(n, transposition, name, clef, whenTransposed int) *musBuild {
	data := make([]byte, 36)
	data[musStaffClef] = byte(clef)
	data[musStaffClefTrans] = byte(whenTransposed)
	b.order.PutUint16(data[musStaffTrans:], uint16(transposition))
	b.order.PutUint16(data[musStaffName:], uint16(name))
	for i := 0; i < 3; i++ {
		b.otherRow(n, "IS", data[12*i:12*(i+1)])
	}
	return b
}

// Places in Finale's clef table, which is what a staff holds.
const (
	musClefTreble     = 0
	musClefBass       = 3
	musClefPercussion = 12
)

func (b *musBuild) frame(n, start, end int) *musBuild {
	return b.otherRow(n, "FR", b.u32s(start, end, 0))
}

// textBlock links a name to the words it stands for. A staff holds this
// record's number and not the block's, and the two are not the same one.
func (b *musBuild) textBlock(id, block int) *musBuild {
	return b.otherRow(id, "TX", b.u16s(block, 0, 0, 0, 0, 0))
}

// group joins staves first to last with one of Finale's bracket shapes.
// Two rows, which is as far along as the shape sits.
func (b *musBuild) group(first, last, shape int) *musBuild {
	data := make([]byte, 20)
	b.order.PutUint16(data[musGroupStart:], uint16(first))
	b.order.PutUint16(data[musGroupEnd:], uint16(last))
	b.order.PutUint16(data[musGroupShape:], uint16(shape))
	for i := 0; i < 2; i++ {
		b.detailRow(0, 1, "NG", data[10*i:10*(i+1)])
	}
	return b
}

// exprDef defines one expression: what it does when the score is played,
// and the words the library describes it by. Four rows, which is as far
// along as the words sit.
func (b *musBuild) exprDef(n, play int, words string) *musBuild {
	data := make([]byte, 48)
	data[musExprPlay] = byte(play)
	copy(data[musExprWords:], words)
	for i := 0; i < 4; i++ {
		b.otherRow(n, "DT", data[12*i:12*(i+1)])
	}
	return b
}

// enclose draws a box round a definition, or stores one and leaves it
// undrawn, which is what the top bit of its flags says.
func (b *musBuild) enclose(n int, drawn bool) *musBuild {
	data := make([]byte, 12)
	if !drawn {
		data[musEnclFlags] = musEnclHidden
	}
	return b.otherRow(n, "De", data)
}

// exprAt writes an expression against a bar, on one staff of it.
func (b *musBuild) exprAt(bar, def, staff int) *musBuild {
	data := make([]byte, 12)
	b.order.PutUint16(data[musPlaceExpr:], uint16(def))
	data[musPlaceStaff] = byte(staff)
	return b.otherRow(bar, "DY", data)
}

// aNote is one staff and one sounding note, for the questions about how a
// staff is written rather than what is on it. raw is the transposition word
// as the staff record holds it and name is the staff's name id.
func aNote(b *musBuild, raw, name int) *musBuild {
	b.staff(1, raw, name).used(1)
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1)
	b.hold(1, 1, 1)
	// The tonic of the key, which with no key is middle C.
	b.sounds(1, 0, 1024, 0)
	return b
}

// A frame hold: which frame fills a staff in a measure. The layers start at
// +4 in a file whose staff records are three rows.
func (b *musBuild) hold(staff, bar, frame int) *musBuild {
	data := make([]byte, 10)
	b.order.PutUint16(data[musFrameBase2005:], uint16(frame))
	return b.detailRow(staff, bar, "GF", data)
}

func (b *musBuild) used(staves ...int) *musBuild {
	for _, s := range staves {
		b.otherRow(0, "IU", b.u16s(s, 0, 0, 0, 0, 0))
	}
	return b
}

// usedPairs is the staff order as Bach's cantata BWV 1 writes it: the tag
// with a small u, two rows to a staff, the staff in the first of them and
// the same constant in every second one.
func (b *musBuild) usedPairs(staves ...int) *musBuild {
	for _, s := range staves {
		b.otherRow(0, "Iu", b.u16s(s, 0, 0, 0, 0, 0))
		b.otherRow(0, "Iu", b.u16s(1, 0, 0, 0, 0, 0))
	}
	return b
}

// holdEarly puts a bar's music where the earlier layout keeps it, which is
// two bytes before where hold puts it.
func (b *musBuild) holdEarly(staff, bar, frame int) *musBuild {
	data := make([]byte, 10)
	b.order.PutUint16(data[musFrameBase2001:], uint16(frame))
	return b.detailRow(staff, bar, "GF", data)
}

// staffEarly is a staff of the earlier layout, which keeps the clef it is
// written in one byte before where the later one does.
func (b *musBuild) staffEarly(n, name, clef int) *musBuild {
	data := make([]byte, 36)
	data[musStaffClefEarly] = byte(clef)
	b.order.PutUint16(data[musStaffName:], uint16(name))
	for i := 0; i < 3; i++ {
		b.otherRow(n, "IS", data[12*i:12*(i+1)])
	}
	return b
}

// groupFor is a group belonging to one of the parts made from a score,
// rather than to the score itself.
func (b *musBuild) groupFor(part, first, last, shape int) *musBuild {
	data := make([]byte, 20)
	b.order.PutUint16(data[musGroupStart:], uint16(first))
	b.order.PutUint16(data[musGroupEnd:], uint16(last))
	b.order.PutUint16(data[musGroupShape:], uint16(shape))
	for i := 0; i < 2; i++ {
		b.detailRow(part, 1, "NG", data[10*i:10*(i+1)])
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
		b.otherRow(i+1, "sL", b.u16s(0, 0, 0, 0, 0, 0))
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
// empty Finale document, and it opens as one.
//
// This test used to assert the opposite, on the reasoning that such a file
// would build into a score with no parts and so look like success to
// everything downstream. The reasoning was wrong and no real file had been
// read when it was written: the staves and the bars are in the document, not
// in the music, so what it builds is a ruled, named, empty score — which is
// exactly what a Finale template is, and what most .mus files turn out to be.
func TestAnEmptyDocumentOpens(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024)

	score, _, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("a document with no music was refused: %v", err)
	}
	if len(score.Parts) != 1 {
		t.Fatalf("got %d parts, want the one staff the document rules",
			len(score.Parts))
	}
	if len(score.Parts[0].Measures) != 1 {
		t.Errorf("got %d bars, want the one the document rules",
			len(score.Parts[0].Measures))
	}
}

// A document whose every frame names music that is not in the file opens too,
// and reports the bars it lost.
//
// Refusing it would be refusing a score for being damaged, which leaves
// somebody with nothing where they could have had the staves, the bars and
// the words — and no way to tell which bars went.
func TestADocumentWhoseMusicIsAllMissingStillOpens(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024)
	// A hold naming a frame that names entries the pool does not hold.
	b.frame(1, 900, 901).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("a document with unreachable music was refused: %v", err)
	}
	if len(score.Parts) != 1 {
		t.Fatalf("got %d parts, want the one staff the document rules",
			len(score.Parts))
	}
	found := false
	for _, s := range skipped {
		if strings.Contains(s, "notes are not in the file") {
			found = true
		}
	}
	if !found {
		t.Errorf("the bars that went were not reported; got %v", skipped)
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

// A repeat is a flag on a measure rather than a record of its own. A piece
// whose repeats have gone is a piece of a different length, which is the
// sort of thing somebody finds out in a rehearsal.
func TestARepeatClosesTheBarItIsWrittenOn(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).frame(1, 1, 1).hold(1, 1, 1)
	// A measure with a double barline and a backward repeat on it.
	b.otherRow(1, "MS", b.u16s(600, 0, 4, 1024, 0, 0x24))
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if got := score.Parts[0].Measures[0].RightBarline; got != model.BarRepeatEnd {
		t.Errorf("the bar closes with a %v, want a repeat", got)
	}
}

// Finale writes a forward repeat on the bar it opens, against that bar's
// left-hand edge. gscore stores one style per bar, on the bar the sign
// stands after, so the sign has to move back one: left where it was, a
// repeat opening bar five would be drawn closing it.
func TestAForwardRepeatIsDrawnBeforeTheBarItOpens(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1)
	b.bar(1, 4, 1024)
	// The second bar opens a repeat; the first closes with it.
	b.otherRow(2, "MS", b.u16s(600, 0, 4, 1024, 0, 0x18))
	b.frame(1, 1, 1).frame(2, 2, 2)
	b.hold(1, 1, 1).hold(1, 2, 2)
	b.sounds(1, 0, 4096, 0).sounds(2, 0, 4096, 1)

	ms := readFixture(t, b).Parts[0].Measures
	if len(ms) != 2 {
		t.Fatalf("got %d bars, want 2", len(ms))
	}
	if got := ms[0].RightBarline; got != model.BarRepeatStart {
		t.Errorf("the first bar closes with a %v, want the repeat opening", got)
	}
	// And the second bar is the last of the piece, so it closes it.
	if got := ms[1].RightBarline; got != model.BarFinal {
		t.Errorf("the second bar closes with a %v, want the final barline", got)
	}
}

// A section that ends where the next begins is one sign on the page — dots,
// a heavy line, dots — and one style in the model.
func TestARepeatClosingWhereAnotherOpensIsOneSign(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1)
	b.otherRow(1, "MS", b.u16s(600, 0, 4, 1024, 0, 0x14)) // closes one
	b.otherRow(2, "MS", b.u16s(600, 0, 4, 1024, 0, 0x18)) // opens the next
	b.frame(1, 1, 1).frame(2, 2, 2)
	b.hold(1, 1, 1).hold(1, 2, 2)
	b.sounds(1, 0, 4096, 0).sounds(2, 0, 4096, 1)

	ms := readFixture(t, b).Parts[0].Measures
	if got := ms[0].RightBarline; got != model.BarRepeatBoth {
		t.Errorf("the barline between the bars is a %v, want both signs", got)
	}
}

// A numbered ending is a different thing and is still only counted: the bit
// says one begins here and not which times through it is played nor where it
// stops, and a first-time bar played every time is a piece of the wrong
// length.
func TestANumberedEndingIsReportedRatherThanGuessedAt(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).frame(1, 1, 1).hold(1, 1, 1)
	b.otherRow(1, "MS", b.u16s(600, 0, 4, 1024, 0, 0x12))
	b.sounds(1, 0, 4096, 0)

	_, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if got := strings.Join(skipped, "\n"); !strings.Contains(got, "endings") {
		t.Errorf("the report is %q, which does not mention the ending", got)
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

/*
 * A blank document is a document.
 *
 * Most .mus files hold no music. A template is staves and bars and nothing
 * else, and so is the file an exercise starts from: of the hundred and two
 * in the Berklee Press collection, seventy-one are blank, among them a
 * thirty-six stave orchestral score with every instrument named and fifty-five
 * bars ruled, waiting to be filled in. Refusing those loses a usable document
 * and tells somebody their file is broken when it is not.
 */
func TestABlankDocumentOpensAsABlankDocument(t *testing.T) {
	b := newMus()
	b.title = "A Template"
	b.staff(1, 0, 1).staff(2, 0, 2).used(1, 2)
	for n := 1; n <= 8; n++ {
		b.bar(n, 4, 1024)
	}

	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("a blank template was refused: %v", err)
	}
	if score.Identity.Title != "A Template" {
		t.Errorf("title is %q, want the one in the file", score.Identity.Title)
	}
	if len(score.Parts) != 2 {
		t.Fatalf("got %d staves, want the 2 the file rules", len(score.Parts))
	}
	for i, p := range score.Parts {
		if len(p.Measures) != 8 {
			t.Errorf("staff %d has %d bars, want the 8 the file rules",
				i+1, len(p.Measures))
		}
	}
	if countNotes(score) != 0 {
		t.Error("a blank template came back with notes in it")
	}
	// Nothing was lost, so nothing is reported as lost. A template that
	// warned about its own emptiness would be telling somebody to go and
	// look for music that was never there.
	for _, s := range skipped {
		if strings.Contains(s, "cannot be followed") {
			t.Errorf("a blank document reported %q", s)
		}
	}
}

// Deleted music does not make a blank document look like a broken one.
//
// A .mus is written in place, so a template that once had music in it still
// carries the entries, reachable from nothing. That is the ordinary state of
// the jazz templates in that collection: two hundred and forty-six entries
// apiece, a hundred and ninety bars that each say they are empty, and not one
// note. The entries are the space where music used to be, not music that
// could not be read, and the file says which by naming no frame anywhere.
func TestLeftoverEntriesInABlankDocumentAreNotReportedAsLost(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 1).used(1)
	b.bar(1, 4, 1024).bar(2, 4, 1024)
	// Bars that exist and hold nothing, which is what Finale writes.
	b.hold(1, 1, 0).hold(1, 2, 0)
	for i := range 6 {
		b.sounds(100+i, 101+i, 1024, 0)
	}

	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("a blank document with leftovers was refused: %v", err)
	}
	if n := countNotes(score); n != 0 {
		t.Errorf("got %d notes; the leftovers are deleted music, not music", n)
	}
	for _, s := range skipped {
		if strings.Contains(s, "cannot be followed") {
			t.Errorf("deleted music was reported as lost: %q", s)
		}
	}
}

// A bar whose notes are missing from the file is a loss, and says so.
//
// This is the one case where a .mus holds music that cannot be read: a frame
// naming a chain of entries the pool does not contain. The frame has to be
// dropped — the score builder refuses a chain it cannot walk, so keeping it
// would cost the whole document — and a bar that goes quietly is a bar
// somebody finds missing in a rehearsal. It is the opposite of an entry no
// frame names, which is deleted music and no loss at all.
func TestABarWhoseNotesAreMissingIsReported(t *testing.T) {
	b := aTune(newMus())
	// A third bar whose frame names entries that were never written.
	b.bar(3, 4, 1024)
	b.frame(3, 900, 903)
	b.hold(1, 3, 3)

	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	// The rest of the music still arrives.
	if n := countNotes(score); n != 8 {
		t.Errorf("got %d notes, want the 8 that are in the file", n)
	}
	found := false
	for _, s := range skipped {
		if strings.Contains(s, "notes are not in the file") {
			found = true
		}
	}
	if !found {
		t.Errorf("a bar whose notes are missing was not reported; got %v", skipped)
	}
}

// A measure with a key signature word, written whole as the format packs it.
func (b *musBuild) barWithKey(n, beats, div, key int) *musBuild {
	return b.otherRow(n, "MS", b.u16s(600, key, beats, div, 0, 0))
}

/*
 * A key signature is two fields in one word.
 *
 * The low byte counts the accidentals, sharps up and flats down as a signed
 * byte; the high byte is the mode. Read whole, the word is right for sharps
 * by accident and wrong for every flat: two flats is 0xFE, and 254 is not a
 * signature anybody has, so the file's key was thrown away as nonstandard and
 * the staff came out bare. A Bach fugue in G minor engraved with no flats at
 * all is how this was found, and the two-flat word that file carries — 0x01FE
 * — is the case below.
 */
func TestAKeySignatureIsACountAndAMode(t *testing.T) {
	for _, c := range []struct {
		what   string
		word   int
		fifths int
		mode   model.Mode
	}{
		{"D major, the word a .mus carries for two sharps", 0x0002, 2, model.Major},
		{"E major", 0x0004, 4, model.Major},
		{"F major, one flat", 0x00ff, -1, model.Major},
		{"G minor, two flats and the minor bit", 0x01fe, -2, model.Minor},
		{"A minor, no accidentals and the minor bit", 0x0100, 0, model.Minor},
		{"seven flats", 0x00f9, -7, model.Major},
		{"seven sharps", 0x0007, 7, model.Major},
	} {
		b := newMus()
		b.staff(1, 0, 1).used(1)
		b.barWithKey(1, 4, 1024, c.word)
		b.frame(1, 1, 1).hold(1, 1, 1)
		b.sounds(1, 0, 4096, 0)

		score, skipped, err := readMus(b.bytes())
		if err != nil {
			t.Errorf("%s: did not read: %v", c.what, err)
			continue
		}
		for _, s := range skipped {
			if strings.Contains(s, "key signature") || strings.Contains(s, "mode") {
				t.Errorf("%s: reported %q", c.what, s)
			}
		}
		key := score.Parts[0].KeyAt(0)
		if key.Fifths != c.fifths {
			t.Errorf("%s: got %d accidentals, want %d",
				c.what, key.Fifths, c.fifths)
		}
		if key.Mode != c.mode {
			t.Errorf("%s: got the %v mode, want %v", c.what, key.Mode, c.mode)
		}
	}
}

// A signature that is not a count of sharps or flats has nowhere to go in a
// gscore key, so it is left out and said so rather than drawn as something
// else.
func TestANonstandardKeySignatureIsReported(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 1).used(1)
	// Nine flats, which is not a signature in the ordinary sense.
	b.barWithKey(1, 4, 1024, 0x00f7)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if f := score.Parts[0].KeyAt(0).Fifths; f != 0 {
		t.Errorf("a nonstandard signature was drawn as %d accidentals", f)
	}
	found := false
	for _, s := range skipped {
		if strings.Contains(s, "nonstandard key") {
			found = true
		}
	}
	if !found {
		t.Errorf("a nonstandard signature was not reported; got %v", skipped)
	}
}

// A staff holds the number of a record that names its words, and not the
// number of the words themselves. The two are different, and reading the
// first as the second finds nothing — a nameless staff, which is a staff
// played as a piano and never reported, because a staff with no name is not
// an error. A saxophone quartet came through as four pianos that way.
func TestAStaffIsNamedThroughItsTextBlock(t *testing.T) {
	b := newMus()
	aNote(b, 0, 50)
	b.textBlock(50, 18)
	b.text = "^block(18)^font(Times,8191)^size(12)Alto Sax.^end"

	score := readFixture(t, b)
	if got := score.Parts[0].Name; got != "Alto Sax." {
		t.Errorf("the staff is called %q, want %q", got, "Alto Sax.")
	}
	// And the name is what says what the staff is, Finale's own instrument
	// code being a key into a library that is not in the file.
	if got := score.Parts[0].Instrument; got != model.AltoSax {
		t.Errorf("the staff plays a %v, want an alto sax", got)
	}
}

// Where there is no such record the block numbers are taken at face value,
// which is what a file without them has to be read by.
func TestWithoutThoseRecordsTheBlockNumberIsTheName(t *testing.T) {
	score := readFixture(t, aTune(newMus()))
	if got := score.Parts[0].Name; got != "Flute" {
		t.Errorf("the staff is called %q, want %q", got, "Flute")
	}
}

// A transposing staff is written at an interval from where it sounds, and
// the interval has two halves that have to agree: so many steps up the
// staff, and so many semitones.
//
// A .mus stores only the second, as the number of sharps the staff's key
// gains. Taking that alone and leaving the steps at nothing wrote every note
// where it sounds and then made up the difference in accidentals — a C with
// nine semitones of sharp on it, which is an A, spelled as nothing anybody
// could read. A page of double sharps is what it looked like.
func TestATransposingStaffIsWrittenAtAnIntervalAndNotAnAccidental(t *testing.T) {
	// Three sharps added: an instrument in E flat, written a sixth up.
	score := readFixture(t, aNote(newMus(), 3, 0))

	part := score.Parts[0]
	if got := part.KeyAt(0).Fifths; got != 3 {
		t.Errorf("the staff is written in %d sharps, want 3", got)
	}
	note := part.Measures[0].Voices[0].Elements[0].(*model.Note)
	if got := note.Pitch.String(); got != "A4" {
		t.Errorf("middle C sounding is written %q, want %q", got, "A4")
	}
	if note.Pitch.Alter != 0 {
		t.Errorf("the written note carries %d semitones of accidental, want none",
			note.Pitch.Alter)
	}
}

// The one thing the interval does not say is how many octaves it spans:
// Finale folds it into a residue within an octave and keeps the octaves in
// its instrument library, which is not in the file. A tenor saxophone and a
// soprano store the same bytes.
//
// So the name settles it, where the name is one gscore knows — and a tenor
// saxophone is a tenor saxophone wherever it is written down.
func TestTheOctaveOfATransposingStaffComesBackFromItsName(t *testing.T) {
	b := newMus()
	// Two sharps added, which an instrument in B flat is: a second up, and
	// for a tenor an octave more.
	aNote(b, 2, 50)
	b.textBlock(50, 18)
	b.text = "^block(18)^font(Times,8191)^size(12)Tenor Sax.^end"

	score := readFixture(t, b)
	want := model.Transposition{Diatonic: -1, Chromatic: -2, OctaveChange: -1}
	if got := score.Parts[0].Transpose; got != want {
		t.Errorf("the staff transposes %+v, want %+v", got, want)
	}
}

// The name is trusted to settle the question the file leaves open and never
// to overrule one it answers. A staff named for an instrument that does not
// transpose the way the file says it does keeps what the file says, because
// at that point the name is the thing that is wrong.
func TestTheNameDoesNotOverruleTheFilesOwnInterval(t *testing.T) {
	b := newMus()
	aNote(b, 2, 50)
	b.textBlock(50, 18)
	b.text = "^block(18)^font(Times,8191)^size(12)Flute^end"

	score := readFixture(t, b)
	want := model.Transposition{Diatonic: -1, Chromatic: -2}
	if got := score.Parts[0].Transpose; got != want {
		t.Errorf("the staff transposes %+v, want %+v", got, want)
	}
}

// Staves that belong together are joined down the left of the system. Without
// the record that says so a saxophone quartet is four unconnected staves,
// which is four soloists sharing a page.
func TestStavesThatBelongTogetherAreBracketed(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).staff(2, 0, 0).used(1, 2)
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1)
	b.hold(1, 1, 1).hold(2, 1, 1)
	b.sounds(1, 0, 1024, 0)
	b.group(1, 2, 6) // Finale's number for the straight bracket.

	score := readFixture(t, b)
	if len(score.Groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(score.Groups))
	}
	g := score.Groups[0]
	if g.First != 0 || g.Last != 1 {
		t.Errorf("the group covers staves %d to %d, want 0 to 1", g.First, g.Last)
	}
	if g.Symbol != model.GroupBracket {
		t.Errorf("the group is drawn as %v, want a bracket", g.Symbol)
	}
	// A bracket is what says these staves are played together, and the
	// barlines run through to say it again.
	if !g.Barlines {
		t.Error("the barlines stop at each staff, want them through the group")
	}
}

// twoBars is a staff of two bars, for the marks that have to land on the
// right one.
func twoBars(b *musBuild) *musBuild {
	b.staff(1, 0, 0).used(1)
	b.bar(1, 4, 1024).bar(2, 4, 1024)
	b.frame(1, 1, 1).frame(2, 2, 2)
	b.hold(1, 1, 1).hold(1, 2, 2)
	b.sounds(1, 0, 4096, 0).sounds(2, 0, 4096, 1)
	return b
}

// A .mus has no categories — Finale did not group its expressions until
// long after these files were written — so nothing in it says that a mark is
// a rehearsal mark. What says it is the box: a marking drawn in one, against
// a bar, is the one a conductor calls that bar by, which is what the box is
// for in the first place.
//
// And a mark Finale numbers for itself carries no words, only the library's
// description of it. What it prints is the bar it stands on, and that is a
// thing a reader can work out rather than guess.
func TestABoxedMarkAgainstABarNamesThatBar(t *testing.T) {
	b := twoBars(newMus())
	b.exprDef(1, 0, "Boxed Rehearsal Number").enclose(1, true)
	b.exprAt(2, 1, 1)

	ms := readFixture(t, b).Parts[0].Measures
	if got := ms[1].Marker; got != "2" {
		t.Errorf("the second bar is called %q, want %q", got, "2")
	}
	if got := ms[0].Marker; got != "" {
		t.Errorf("the first bar is called %q, want nothing", got)
	}
}

// Where the mark has words of its own they are what it is called, because a
// lettered mark is a letter and not the bar it happens to fall on.
func TestAMarkWithWordsOfItsOwnKeepsThem(t *testing.T) {
	b := twoBars(newMus())
	b.exprDef(1, 0, "B").enclose(1, true)
	b.exprAt(2, 1, 1)

	if got := readFixture(t, b).Parts[0].Measures[1].Marker; got != "B" {
		t.Errorf("the bar is called %q, want %q", got, "B")
	}
}

// A marking against a bar that is not in a box is something else — a
// direction, a tempo word, a note to the player — and naming the bar after
// it would put a word in the one place on a page that has to mean an
// address. It is counted instead.
func TestAMarkThatIsNotBoxedDoesNotNameTheBar(t *testing.T) {
	b := twoBars(newMus())
	b.exprDef(1, 0, "Solo").enclose(1, false)
	b.exprAt(2, 1, 1)

	score, skipped, err := readMus(b.bytes())
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if got := score.Parts[0].Measures[1].Marker; got != "" {
		t.Errorf("the bar is called %q, want nothing", got)
	}
	if got := strings.Join(skipped, "\n"); !strings.Contains(got, "against a bar") {
		t.Errorf("the report is %q, which does not mention the marking", got)
	}
}

// Nor is a dynamic, which says so itself: a definition that sets the
// loudness carries that in the one byte of it saying what it does to
// playback. Boxing one would be unusual and reading it as an address would
// still be wrong.
func TestADynamicIsNotTakenForARehearsalMark(t *testing.T) {
	b := twoBars(newMus())
	b.exprDef(1, musPlayAmplitude, "ff").enclose(1, true)
	b.exprAt(2, 1, 1)

	if got := readFixture(t, b).Parts[0].Measures[1].Marker; got != "" {
		t.Errorf("the bar is called %q, want nothing", got)
	}
}

// Finale does not store the sign at the end of a piece: the style it keeps
// for the last bar is the ordinary one, and the thin-thick line is drawn
// because the music has stopped. So it is drawn here for the same reason —
// a score that ends on a plain barline reads as a page torn off.
func TestThePieceEndsWithAFinalBarline(t *testing.T) {
	ms := readFixture(t, twoBars(newMus())).Parts[0].Measures
	if got := ms[0].RightBarline; got != model.BarSingle {
		t.Errorf("the first bar closes with a %v, want a plain barline", got)
	}
	if got := ms[len(ms)-1].RightBarline; got != model.BarFinal {
		t.Errorf("the piece ends with a %v, want the final barline", got)
	}
}

// Unless the last bar says something of its own. A piece can end on a repeat
// — go back and play it again, and that is the end of it — and one that ends
// on a double bar meant the double bar.
func TestALastBarThatSaysSomethingElseKeepsIt(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).frame(1, 1, 1).hold(1, 1, 1)
	b.otherRow(1, "MS", b.u16s(600, 0, 4, 1024, 0, 0x14)) // a backward repeat
	b.sounds(1, 0, 4096, 0)

	ms := readFixture(t, b).Parts[0].Measures
	if got := ms[0].RightBarline; got != model.BarRepeatEnd {
		t.Errorf("the piece ends with a %v, want the repeat", got)
	}
}

/*
 * What the files from 2007, 2011 and 2012 turned out to need
 *
 * The reader was built against Macs: a hundred and two files from one
 * collection, every one written in 2005 by a Finale for the Mac. Everything
 * below is something a file from somewhere else got wrong, found by reading
 * ninety-three more against the MusicXML that Finale itself exported beside
 * them — which says what the staves are called, what key they are in and what
 * they transpose by, and so can be compared with rather than looked at.
 */

// A staff transposed by a fixed interval does not change key.
//
// Finale transposes either by key — a clarinet in B flat is written two
// sharps sharper than it sounds — or chromatically, which moves the notes and
// leaves the signature alone. A guitar is the second kind: it sounds an
// octave below where it is written. Reading the key adjustment out of a
// chromatic word put seven sharps on every staff of a guitar octet, because
// the seven in it counts steps and not sharps.
func TestAChromaticTranspositionLeavesTheKeyAlone(t *testing.T) {
	for _, c := range []struct {
		what  string
		word  int
		key   int
		which model.Instrument
	}{
		{"a guitar, written an octave above where it sounds", 0x6007, 1, model.AcousticGuitar},
		{"a clarinet in B flat", 0x0042, 3, model.Clarinet},
		{"a horn in F", 0x0101, 2, model.FrenchHorn},
		{"a staff that does not transpose", 0x4000, 1, model.Violin},
	} {
		b := newMus()
		b.staff(1, c.word, 1).used(1).textBlock(1, 1)
		b.text = "^block(1)^font(Times,0)Guitar^end"
		b.barWithKey(1, 4, 1024, 1) // one sharp
		b.frame(1, 1, 1).hold(1, 1, 1)
		b.sounds(1, 0, 4096, 0)

		got := readFixture(t, b).Parts[0].Key.Fifths
		if got != c.key {
			t.Errorf("%s is written with %d sharps, want %d", c.what, got, c.key)
		}
	}
}

// The last staff record of a newer file is not a staff.
//
// Every file from 2007 on carries one at 32767, holding what a new staff in
// that document would start from. It is in all fifty-one of them and in none
// of the hundred and forty-four older ones.
//
// What it costs to count it is not an extra staff on the page — staffOrder
// has always left the number out of its fallback — but a miscount against
// the score's own staff list, which names the real staves and not this one.
// With the defaults counted the list is always one short of the records, so
// every newer file reported a staff left out of it that is not a staff.
func TestTheStaffDefaultsRecordIsNotAStaff(t *testing.T) {
	b := newerMus()
	b.record(&b.others, musRecOthersHead, 231, musStaffDefaults, 0, make([]byte, 36))
	b.record(&b.others, musRecOthersHead, 159, 0, 0, b.usedEntry(1))

	score, skipped, err := readMus(b.bytes())
	if err != nil && score == nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if n := len(score.Parts); n != 1 {
		t.Errorf("got %d staves, want the 1 that is a staff", n)
	}
	if hasSkip(skipped, "leaves out") {
		t.Errorf("the defaults record was counted as a staff the score "+
			"leaves out; got %v", skipped)
	}
}

// The newer era writes the whole staff order into the first incidence.
//
// The older writes one row per staff. The newer writes the list once and then
// repeats it an entry at a time, starting one staff along — so reading the
// repeats as a continuation names one staff twice and loses another.
func TestTheNewerStaffOrderIsInTheFirstRecord(t *testing.T) {
	b := newerMus()
	// A second staff, with a name on it, so the order can be read off the
	// page instead of guessed at from where the music landed. The first
	// staff is the one newerMus made, and has no name.
	named := make([]byte, 36)
	b.order.PutUint16(named[musStaffName:], 2)
	b.record(&b.others, musRecOthersHead, 231, 2, 0, named)
	b.record(&b.others, musRecOthersHead, 183, 2, 0, b.u16s(7))
	b.text = "^block(7)Second staff, read first^end"

	// The score reads the second staff first.
	b.record(&b.others, musRecOthersHead, 159, 0, 0,
		append(b.usedEntry(2), b.usedEntry(1)...))
	// And then repeats the list an entry at a time, starting one along.
	b.record(&b.others, musRecOthersHead, 159, 0, 1, b.usedEntry(1))
	b.record(&b.others, musRecOthersHead, 159, 0, 2, b.usedEntry(2))

	score := readFixture(t, b)
	if len(score.Parts) != 2 {
		t.Fatalf("got %d staves, want 2", len(score.Parts))
	}
	if got := score.Parts[0].Name; got != "Second staff, read first" {
		t.Errorf("the top staff is %q, want the one the order names first; "+
			"the order was not read, or was read rotated", got)
	}
}

// A staff the score's own list leaves out is not in the score.
//
// Somebody takes a staff out of a score and Finale stops listing it; the
// record stays in the file with whatever was on it. Requiring the list to
// name every record — and falling back to the records when it did not — put
// those staves back, which is how a piano piece came through on three staves,
// the grand staff it prints and an empty one over the top.
func TestAStaffTheScoreLeavesOutIsNotInIt(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).staff(2, 0, 0)
	b.used(2) // only the second is in the score
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(2, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score, skipped, err := readMus(b.bytes())
	if err != nil && score == nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if n := len(score.Parts); n != 1 {
		t.Errorf("got %d staves, want the 1 the score lists", n)
	}
	// And it is said out loud, because that staff may have music on it.
	if !hasSkip(skipped, "leaves out") {
		t.Errorf("the staff left out was not reported; got %v", skipped)
	}
}

// The newer files write their formatting as bytes, not as words.
//
// An older file says ^font(Times New Roman,8192)^size(12); a newer one says
// the same thing as a caret and a byte above 127 with its arguments after it.
// Nothing stripped those, so a staff arrived called
// "^\x85\x01\x01\x01\x03^\x86\x01\x01\x01\x0fGuitar 1" — a name with all of
// its formatting still in front of it, which is no instrument gscore knows,
// which is a staff played on a piano.
func TestTheNewerMarkupIsNotPartOfTheName(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 1).used(1).textBlock(1, 1)
	b.text = "^block(1)^\x85\x01\x01\x01\x03^\x86\x01\x01\x01\x0fGuitar 1^end"
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if got := score.Parts[0].Name; got != "Guitar 1" {
		t.Errorf("the staff is called %q, want %q", got, "Guitar 1")
	}
	if got := score.Parts[0].Instrument; got != model.AcousticGuitar {
		t.Errorf("the staff is played on a %v, want a guitar", got)
	}
}

// A name that is a line of text keeps its line break, which is the reason the
// markup is measured rather than skipped to the first readable byte.
func TestANameOfTwoLinesKeepsBoth(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 1).used(1).textBlock(1, 1)
	b.text = "^block(1)^\x84\x01\x01\x01\x01\r\nVIOLONCELLE^end"
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	if got := readFixture(t, b).Parts[0].Name; got != "VIOLONCELLE" {
		t.Errorf("the staff is called %q; the line break before it was read "+
			"as part of the markup", got)
	}
}

// The three words a document says about itself sit at fixed places, and the
// places are not cleared when something shorter is written over them.
//
// Read one after another, each starting where the last one's zero left off,
// the leftovers of whatever the file was made from become the composer: a
// traditional Japanese song came through written by "rry Blossom Song)",
// which is the tail of the title of the file it was copied from.
func TestTheComposerIsNotTheTailOfAnOlderTitle(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).used(1).bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)
	raw := b.bytes()
	// A title, and behind the zero that ends it the tail of a longer one.
	copy(raw[musTitleAt:], "Takai-Yama\x00rry Blossom Song)\x00")
	copy(raw[musTitleAt+musComposerAt:], "Traditional Japanese Song\x00")

	score, _, err := readMus(raw)
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if got := score.Identity.Title; got != "Takai-Yama" {
		t.Errorf("the title is %q, want %q", got, "Takai-Yama")
	}
	if got := score.Identity.Composer; got != "Traditional Japanese Song" {
		t.Errorf("the composer is %q, want %q", got, "Traditional Japanese Song")
	}
}

// The pools do not always start at 0x200.
//
// They start where the header says, and that is 0x200 in every file written
// on a Mac because the three words in front of them are given a fixed block
// there. Finale for Windows wrote them as text of whatever length they are,
// so the pools start past the end of it. Reading 0x200 as the answer rather
// than as the usual value of the answer refused thirty-eight real files with
// a message saying there was no container in a file that has one.
func TestThePoolsStartWhereTheHeaderSays(t *testing.T) {
	b := aTune(newMus())
	raw := b.bytes()

	// Push the container on by a round number and say so in the header.
	const moved = 0x80
	shifted := make([]byte, len(raw)+moved)
	copy(shifted, raw[:musContainerAt])
	copy(shifted[musContainerAt+moved:], raw[musContainerAt:])
	b.order.PutUint16(shifted[musContainerWhere:], uint16(musContainerAt+moved))

	score, _, err := readMus(shifted)
	if err != nil {
		t.Fatalf("a file whose header says where its pools are did not read: %v", err)
	}
	if n := countNotes(score); n != 8 {
		t.Errorf("got %d notes, want the 8 in the fixture", n)
	}
}

// And a header that says something absurd still finds them where they
// usually are, because the offset is checked by looking for the pool chain
// where it points rather than taken on its word.
func TestAnImpossibleOffsetFallsBackToTheUsualPlace(t *testing.T) {
	raw := aTune(newMus()).bytes()
	binary.LittleEndian.PutUint16(raw[musContainerWhere:], 0xfffe)

	score, _, err := readMus(raw)
	if err != nil {
		t.Fatalf("the fixture did not read: %v", err)
	}
	if n := countNotes(score); n != 8 {
		t.Errorf("got %d notes, want 8", n)
	}
}

// newerMus is the smallest newer-era file: one staff, one bar, one note.
func newerMus() *musBuild {
	b := newMus()
	b.era = eraZlib
	b.year = 2011
	b.record(&b.others, musRecOthersHead, 231, 1, 0, make([]byte, 36))
	b.record(&b.others, musRecOthersHead, 176, 1, 0, b.u16s(600, 0, 4, 1024, 0, 0))
	b.record(&b.others, musRecOthersHead, 146, 1, 0, b.u32s(1, 1))
	hold := make([]byte, 12)
	b.order.PutUint16(hold[musFrameBase2005:], 1)
	b.record(&b.details, musRecDetailsHead, 1044, 1, 1, hold)
	b.sounds(1, 0, 4096, 0)
	return b
}

// usedEntry is one staff's place in the newer era's staff order.
func (b *musBuild) usedEntry(staff int) []byte {
	out := make([]byte, musUsedEntry)
	b.order.PutUint16(out, uint16(staff))
	return out
}

// hasSkip says whether anything left out was reported in these words.
func hasSkip(skipped []string, words string) bool {
	for _, s := range skipped {
		if strings.Contains(s, words) {
			return true
		}
	}
	return false
}

func TestAStaffIsReadFromItsOwnClef(t *testing.T) {
	b := newMus()
	b.staffClefs(1, 0, 1, musClefBass, musClefTreble).used(1).textBlock(1, 1)
	b.text = "^block(1)Trombone^end"
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if got := score.Parts[0].Clef; got != model.Bass {
		t.Errorf("the trombone is read from the %v clef, want the bass", got)
	}
}

func TestAStaffThatMovesItsClefAsItTransposesIsReadFromTheOtherOne(t *testing.T) {
	// A baritone saxophone: it sounds where the bass clef is and the player
	// reads the treble, so the file holds both and says which to use.
	b := newMus()
	b.staffClefs(1, musTransSetToClef|3, 1, musClefBass, musClefTreble)
	b.used(1).textBlock(1, 1)
	b.text = "^block(1)Baritone Sax^end"
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if got := score.Parts[0].Clef; got != model.Treble {
		t.Errorf("the baritone saxophone is read from the %v clef, want the treble", got)
	}
}

func TestAFlatInAStaffsNameIsPartOfTheName(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 1).used(1).textBlock(1, 1)
	b.text = "^block(1)^font(JazzText,8191)Trumpet in B^flat() 1^end"
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if got, want := score.Parts[0].Name, "Trumpet in B♭ 1"; got != want {
		t.Errorf("the staff is called %q, want %q", got, want)
	}
	if got := score.Parts[0].Instrument; got != model.Trumpet {
		t.Errorf("the staff is played on a %v, want a trumpet", got)
	}
}

func TestADrumIsWrittenWhereDrummersWriteIt(t *testing.T) {
	// A note on a drum staff of this era is the instrument's own General
	// MIDI key, not a place on the staff: D below the bass staff is the
	// snare drum, the E flat above it the ride cymbal.
	b := newMus()
	b.staffClefs(1, 0, 1, musClefPercussion, musClefTreble)
	b.used(1).textBlock(1, 1)
	b.text = "^block(1)Drum Set^end"
	b.bar(1, 4, 1024)
	b.frame(1, 1, 2).hold(1, 1, 1)
	b.entry(1, 2, 2048, musSetBit|musNoteBit, b.note(-13, 0, 0))
	b.entry(2, 0, 2048, musSetBit|musNoteBit, b.note(-5, -1, 0))

	score := readFixture(t, b)
	part := score.Parts[0]
	if got := part.Clef; got != model.Percussion {
		t.Fatalf("the drum staff is read from the %v clef", got)
	}
	notes := part.Measures[0].Voices[0].Elements
	if len(notes) != 2 {
		t.Fatalf("got %d things in the bar, want 2", len(notes))
	}
	want := []struct {
		key  int
		head model.NoteheadKind
		step int
	}{
		{38, model.HeadNormal, 5}, // the snare, in the third space
		{51, model.HeadX, 8},      // the ride, on the top line
	}
	for i, w := range want {
		note, ok := notes[i].(*model.Note)
		if !ok {
			t.Fatalf("note %d came back as a %T", i, notes[i])
		}
		if note.Percussion != w.key {
			t.Errorf("note %d plays drum %d, want %d", i, note.Percussion, w.key)
		}
		if note.Head != w.head {
			t.Errorf("note %d is drawn with a %v head, want %v", i, note.Head, w.head)
		}
		if got := note.Pitch.DiatonicIndex() + percussionMiddle; got != w.step {
			t.Errorf("note %d sits on step %d of the staff, want %d", i, got, w.step)
		}
	}
}

// Where a bar's music is kept moved between 2001 and 2005, and the year on
// the file does not say which way round this one is: Bach's cantata BWV 1,
// written by Finale 2003, has the later era's staff records and the earlier
// era's holds. Read at the later offset, not one of its two thousand eight
// hundred frames was found and all sixteen staves of it came through as four
// hundred and eighteen bars of rest.
//
// So both are read and the one the file agrees with is kept. These are the
// two files that disagree.
func TestTheMusicOfABarIsFoundWhicheverWayTheHoldIsWritten(t *testing.T) {
	for _, c := range []struct {
		what string
		hold func(b *musBuild, staff, bar, frame int) *musBuild
	}{
		{"the earlier way", (*musBuild).holdEarly},
		{"the later way", (*musBuild).hold},
	} {
		t.Run(c.what, func(t *testing.T) {
			b := newMus()
			b.staff(1, 0, 0).used(1).bar(1, 4, 1024).frame(1, 1, 1)
			c.hold(b, 1, 1, 1)
			b.sounds(1, 0, 4096, 0)

			score := readFixture(t, b)
			notes := 0
			for _, m := range score.Parts[0].Measures {
				for _, v := range m.Voices {
					for _, el := range v.Elements {
						if _, ok := el.(*model.Note); ok {
							notes++
						}
					}
				}
			}
			if notes != 1 {
				t.Errorf("got %d notes, want the 1 in the file — the frame "+
					"was looked for at the wrong offset, and a bar whose "+
					"frame is not found is a bar of rest", notes)
			}
		})
	}
}

// The staff order written two rows to a staff.
//
// Read a row at a time it names the second row's constant as a staff, which
// is a staff named twice as soon as there are two of them — so the list is
// thrown out and the staves fall back to the order their records happen to
// be in. BWV 1 keeps its second horn, second oboe and second concertante
// violin at the end of the file, and came out with each of those below the
// continuo, twelve staves from the one it is paired with.
func TestTheOlderStaffOrderIsReadTwoRowsToAStaff(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0)
	b.staff(2, 0, 7).textBlock(7, 7)
	b.text = "^block(7)Second staff, read first^end"
	b.usedPairs(2, 1)
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if len(score.Parts) != 2 {
		t.Fatalf("got %d staves, want 2", len(score.Parts))
	}
	if got := score.Parts[0].Name; got != "Second staff, read first" {
		t.Errorf("the top staff is %q, want the one the order names first; "+
			"the order was read a row at a time, or not at all", got)
	}
}

// The groups a file carries for its parts are not the score's.
//
// A file that has had parts made from it holds the whole set of groups again
// for each of them, keyed to the part. BWV 1 carries nine for the score and
// two hundred and forty more for fifty parts; read together they bracket the
// score fifteen ways at once, and the braces — drawn to the height of what
// they join — come out as a black slab down the side of the page.
func TestThePartsGroupsAreNotTheScores(t *testing.T) {
	b := newMus()
	b.staff(1, 0, 0).staff(2, 0, 0).used(1, 2)
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).hold(1, 1, 1)
	b.sounds(1, 0, 4096, 0)
	b.group(1, 2, 6)        // the score's own: a straight bracket
	b.groupFor(41, 1, 2, 3) // and a part's copy of it, as a brace

	score := readFixture(t, b)
	if n := len(score.Groups); n != 1 {
		t.Fatalf("got %d groups, want the 1 the score has", n)
	}
	if got := score.Groups[0].Symbol; got != model.GroupBracket {
		t.Errorf("the group is a %v, want the bracket the score has — the "+
			"part's copy was read as well", got)
	}
}

// The clef moved with the frame, so the one reading settles both.
//
// A file of the earlier layout keeps the clef it is written in one byte
// before where the later one does, and has nothing in the later byte at all.
// Bach's cantata BWV 1 is one, and read the later way its violas, its
// tenors, its basses and its continuo all came out in the treble clef.
//
// Nothing in the file says which layout it is; what says it is where the
// frames turned out to be, which musFrameBase works out from the frames
// themselves. So this fixture holds its music the earlier way and is
// expected to have its clef read the earlier way too.
func TestTheClefOfTheEarlierLayoutIsFoundWithItsFrames(t *testing.T) {
	b := newMus()
	b.staffEarly(1, 1, musClefBass).used(1).textBlock(1, 1)
	b.text = "^block(1)Trombone^end"
	b.bar(1, 4, 1024)
	b.frame(1, 1, 1).holdEarly(1, 1, 1)
	b.sounds(1, 0, 4096, 0)

	score := readFixture(t, b)
	if got := score.Parts[0].Clef; got != model.Bass {
		t.Errorf("the trombone is read from the %v clef, want the bass — "+
			"the clef was looked for where the later layout keeps it, "+
			"which in a file like this one is empty", got)
	}
}
