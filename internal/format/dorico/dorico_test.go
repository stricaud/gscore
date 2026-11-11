/*
 * dorico_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading a Dorico project.
 *
 * The projects here are built rather than recorded, for the usual reason —
 * a test has to know the answer before it asks — and for one particular to
 * this format: a real project is six megabytes of somebody else's work,
 * and a fixture that large says nothing about which part of the reading is
 * wrong when it breaks.
 *
 * Building them means writing the format as well as reading it, which is
 * worth the twenty lines on its own: a decoder tested only against files it
 * cannot produce is a decoder tested against its own assumptions.
 */

package dorico

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/* ---------- writing a project to read ---------- */

// enc builds a score.dtn.
type enc struct {
	names, values []string
	nameAt        map[string]int
	valueAt       map[string]int

	name  []int
	value []int
	kids  [][]int32
}

func newEnc() *enc {
	e := &enc{nameAt: map[string]int{}, valueAt: map[string]int{}}
	// The first value is always the empty one.
	e.intern("", false)
	return e
}

func (e *enc) intern(s string, isName bool) int {
	at, list := e.valueAt, &e.values
	if isName {
		at, list = e.nameAt, &e.names
	}
	if i, ok := at[s]; ok {
		return i
	}
	i := len(*list)
	*list = append(*list, s)
	at[s] = i
	return i
}

// leaf adds a node holding a value and no children.
func (e *enc) leaf(name, value string) int32 {
	e.name = append(e.name, e.intern(name, true))
	e.value = append(e.value, e.intern(value, false))
	e.kids = append(e.kids, nil)
	return int32(len(e.name) - 1)
}

// node adds a node holding children.
func (e *enc) node(name string, kids ...int32) int32 {
	id := e.leaf(name, "")
	e.kids[id] = kids
	return id
}

// list wraps children in the "array" node Dorico puts round every list.
func (e *enc) list(name string, items ...int32) int32 {
	return e.node(name, e.node("array", items...))
}

// bytes writes the file, with root as node 0, which is where a reader
// starts.
func (e *enc) bytes(root int32) []byte {
	// Everything is renumbered so that the root comes first.
	order := []int32{root}
	seen := map[int32]bool{root: true}
	for i := 0; i < len(order); i++ {
		for _, k := range e.kids[order[i]] {
			if !seen[k] {
				seen[k] = true
				order = append(order, k)
			}
		}
	}
	at := map[int32]int32{}
	for i, id := range order {
		at[id] = int32(i)
	}

	var b bytes.Buffer
	u32 := func(v int) {
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], uint32(v))
		b.Write(tmp[:])
	}
	varint := func(v uint64) {
		for {
			c := byte(v & 0x7f)
			v >>= 7
			if v != 0 {
				c |= 0x80
			}
			b.WriteByte(c)
			if v == 0 {
				return
			}
		}
	}
	u32(4)
	u32(1)
	u32(len(e.names))
	for _, s := range e.names {
		b.WriteString(s)
		b.WriteByte(0)
	}
	u32(len(e.values))
	for _, s := range e.values {
		b.WriteString(s)
		b.WriteByte(0)
	}
	u32(len(order))
	for _, id := range order {
		kids := e.kids[id]
		tag := byte(0x44)
		if len(kids) > 0 {
			tag = 0x46
		}
		b.WriteByte(tag)
		varint(uint64(e.name[id]))
		varint(uint64(e.value[id]))
		if len(kids) > 0 {
			varint(uint64(len(kids)))
			for _, k := range kids {
				varint(uint64(at[k]))
			}
		}
	}
	return b.Bytes()
}

// project wraps a score.dtn in the container a .dorico file is.
func project(dtn []byte, info string) []byte {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	write := func(name string, body []byte) {
		w, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		w.Write(body)
	}
	write("META-INF/container.xml", []byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="score.dtn" media-type="application/vnd.steinberg.dorico-project"/></rootfiles>
</container>`))
	if info != "" {
		write("supplementary_data/scoreinfo/scoreinfo.xml", []byte(info))
	}
	write("score.dtn", dtn)
	zw.Close()
	return out.Bytes()
}

// played is one event for the builder below.
type played struct {
	at, dur string
	pitch   int
	rest    bool
}

// tuplets holds the tuplets a test asked for, by the key tupletOver gave
// back, so that they can be built with the project's own encoder.
var tuplets = map[string]struct{ at, dur, def string }{}

// tupletOver asks for a tuplet over a stretch of the music: so many notes
// in the time of so many, written as Dorico writes it.
func tupletOver(at, dur string, actual, normal int) string {
	key := "tuplet " + at + " " + dur + " " + itoa(actual) + ":" + itoa(normal)
	tuplets[key] = struct{ at, dur, def string }{at, dur,
		"steamVariantList:list: {int: " + itoa(actual) + "}, {int: " +
			itoa(normal) + "}, {list: {int: 7}, {int: 0}}, {bool: true}"}
	return key
}

// dynSub is one piece of a dynamic gesture: a mark, a hairpin, or one of
// the struck dynamics.
type dynSub struct {
	kind    string // "immediate", "gradual" or "force"
	at      string // where in the group it sits
	side    int    // 0 loud, 1 soft
	letters int    // how many f's or p's
	dur     string // a hairpin's length
	down    bool   // and whether it falls
	force   string // the name of a struck dynamic
}

var dynGroups = map[string]struct {
	at   string
	subs []dynSub
}{}

// dynamicAt asks for a dynamic gesture over the music, written the way
// Dorico writes one: a group on the staff, holding marks of its own.
func dynamicAt(at string, subs ...dynSub) string {
	key := "dynamic " + at + " " + itoa(len(dynGroups))
	dynGroups[key] = struct {
		at   string
		subs []dynSub
	}{at, subs}
	return key
}

// built makes a one-flow project: one player on one staff, in the time and
// key given, playing these notes.
func built(t *testing.T, time [2]string, key string, notes []played,
	extra ...string) []byte {
	t.Helper()
	e := newEnc()

	var events []int32
	for _, n := range notes {
		kind, pitch := "NoteEventDefinition", e.leaf("pitch", itoa(n.pitch))
		if n.rest {
			kind = "RestEventDefinition"
		}
		kids := []int32{
			e.leaf("eventID", "1"),
			e.leaf("position", n.at),
			e.leaf("duration", n.dur),
		}
		if !n.rest {
			kids = append(kids, pitch)
		}
		events = append(events, e.node(kind, kids...))
	}
	for _, kind := range extra {
		if _, ok := dynGroups[kind]; ok {
			continue // built onto the staff below
		}
		if t, ok := tuplets[kind]; ok {
			events = append(events, e.node("TupletEventDefinition",
				e.leaf("position", t.at),
				e.leaf("duration", t.dur),
				e.leaf("tupletDefinition", t.def)))
			continue
		}
		events = append(events, e.node(kind,
			e.leaf("position", "0"), e.leaf("duration", "2")))
	}
	voiceBlock := e.node("block",
		e.leaf("blockID", "10"),
		e.leaf("parentEventStreamType", "kVoiceStream"),
		e.node("events", events...))

	timeSig := e.node("BarDivisionElementDefinition",
		e.leaf("elementID", "1"),
		e.node("barDivisionData",
			e.node("timeSignature",
				e.node("timeSignaturesAndDivisions",
					e.node("timeSignatureAndDivision",
						e.node("timeSignature",
							e.leaf("numerator", time[0]),
							e.leaf("denominator", time[1])))))))
	keySig := e.node("TonalityDivisionElementDefinition",
		e.leaf("elementID", "2"),
		e.node("tonalityDivisionData",
			e.node("keySignature",
				e.node("root",
					e.leaf("tonalityType", "kKeySigMinor"),
					e.node("root",
						e.leaf("noteName", key),
						e.leaf("accidentalID",
							"accidental.12et.western.natural"))))))

	globalBlock := e.node("block",
		e.leaf("blockID", "20"),
		e.leaf("parentEventStreamType", "kGlobalTimebaseStream"),
		e.node("events",
			e.node("BarDivisionEventDefinition",
				e.leaf("position", "0"),
				e.leaf("duration", "0"),
				e.leaf("barDivisionElementID", "1")),
			e.node("TonalityDivisionEventDefinition",
				e.leaf("position", "0"),
				e.leaf("duration", "0"),
				e.leaf("tonalityDivisionElementID", "2"))))

	staveEvents := []int32{
		e.node("ClefEventDefinition",
			e.leaf("position", "0"),
			e.node("entityIDs",
				e.leaf("idForTransposingLayouts", "clef.bass"))),
	}
	var dynDefs []int32
	for _, kind := range extra {
		g, ok := dynGroups[kind]
		if !ok {
			continue
		}
		id := itoa(500 + len(dynDefs))
		staveEvents = append(staveEvents,
			e.node("DynamicGroupEventDefinition",
				e.leaf("position", g.at),
				e.leaf("duration", "0"),
				e.leaf("dynamicGroupID", id)))
		var subs []int32
		for _, sub := range g.subs {
			switch sub.kind {
			case "gradual":
				dir := "kGradualDynamicUp"
				if sub.down {
					dir = "kGradualDynamicDown"
				}
				subs = append(subs, e.node("GradualChangeDynamicDefinition",
					e.leaf("position", sub.at),
					e.leaf("type", "kCrescOrDim"),
					e.leaf("direction", dir),
					e.leaf("duration", sub.dur)))
			case "force":
				subs = append(subs, e.node("ForceDynamicDefinition",
					e.leaf("position", sub.at),
					e.leaf("type", sub.force),
					e.leaf("intensityLevel", "1")))
			default:
				subs = append(subs, e.node("ImmediateChangeDynamicDefinition",
					e.leaf("position", sub.at),
					e.leaf("intensity",
						"steamVariantList:list: {int: "+itoa(sub.side)+
							"}, {}, {bool: false}, {int: "+
							itoa(sub.letters)+"}"),
					e.leaf("intensityModifier",
						"steamVariantList:list: {}, {}")))
			}
		}
		dynDefs = append(dynDefs, e.node("DynamicGroupElementDefinition",
			e.leaf("elementID", id),
			e.node("subElements", subs...)))
	}
	staveBlock := e.node("block",
		e.leaf("blockID", "30"),
		e.leaf("parentEventStreamType", "kInstrumentStaveStream"),
		e.node("events", staveEvents...))

	flow := e.node("flow",
		e.leaf("flowID", "0"),
		e.leaf("flowName", "A movement"),
		e.list("flowPlayers",
			e.node("flowPlayer",
				e.leaf("playerID", "0"),
				e.list("flowPlayerInstruments",
					e.node("flowPlayerInstrument",
						e.leaf("instrumentID", "0"),
						e.list("staveStreamInfos",
							e.node("staveStreamInfo",
								e.leaf("staveStreamID", "3"),
								e.leaf("instrumentID", "0"),
								e.leaf("staveID", "0"))),
						e.list("voiceStreamInfos",
							e.node("voiceStreamInfo",
								e.leaf("voiceStreamID", "1"),
								e.leaf("instrumentID", "0"),
								e.leaf("defaultLogicalStaveID", "0"))))))),
		e.list("elementTables",
			e.node("BarDivisionElementTableDefinition",
				e.node("array", timeSig)),
			e.node("TonalityDivisionElementTableDefinition",
				e.node("array", keySig)),
			e.node("DynamicGroupElementTableDefinition",
				e.node("array", dynDefs...))),
		e.list("blocks", voiceBlock, globalBlock, staveBlock),
		e.list("blockInstances",
			e.node("blockInstance",
				e.leaf("blockInstanceID", "100"),
				e.leaf("blockID", "10"),
				e.leaf("position", "0")),
			e.node("blockInstance",
				e.leaf("blockInstanceID", "200"),
				e.leaf("blockID", "20"),
				e.leaf("position", "0")),
			e.node("blockInstance",
				e.leaf("blockInstanceID", "300"),
				e.leaf("blockID", "30"),
				e.leaf("position", "0"))),
		e.list("eventStreams",
			e.node("eventStream",
				e.leaf("streamID", "1"),
				e.leaf("blockInstanceIDs", "100"),
				e.leaf("eventStreamType", "kVoiceStream")),
			e.node("eventStream",
				e.leaf("streamID", "0"),
				e.leaf("blockInstanceIDs", "200"),
				e.leaf("eventStreamType", "kGlobalTimebaseStream")),
			e.node("eventStream",
				e.leaf("streamID", "3"),
				e.leaf("blockInstanceIDs", "300"),
				e.leaf("eventStreamType", "kInstrumentStaveStream"))))

	root := e.node("kScore",
		e.leaf("fileVersion", "1.1417"),
		// The library says what an instrument is; the player says which
		// of them it is holding. The two are kept apart in the file and
		// are put together by the reader.
		e.node("library",
			e.list("instrumentEntityDefinitions",
				e.node("InstrumentEntityDefinition",
					e.leaf("entityID", "instrument.wind.flute"),
					e.leaf("name", "Flute"),
					e.leaf("musicXMLSoundID", "wind.flutes.flute")))),
		e.list("scorePlayers",
			e.node("scorePlayer",
				e.leaf("playerID", "0"),
				e.leaf("baseName", "Cello"),
				e.list("instruments",
					e.node("instrument",
						e.leaf("instrumentID", "0"),
						e.leaf("entityID", "instrument.wind.flute"))))),
		e.list("flows", flow))

	return project(e.bytes(root), `<?xml version="1.0"?><Score>
<title>A piece</title><composer>Somebody</composer></Score>`)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// read imports a built project.
func read(t *testing.T, raw []byte) []*model.Score {
	t.Helper()
	scores, err := Import(bytes.NewReader(raw))
	if err != nil {
		if _, ok := err.(*PartialError); !ok {
			t.Fatalf("reading the project: %v", err)
		}
	}
	if len(scores) == 0 {
		t.Fatal("nothing was read")
	}
	return scores
}

// notesOf lists one bar of one part as text: "G4/8 Eb4/2", a rest as "r".
func notesOf(t *testing.T, p *model.Part, bar int) string {
	t.Helper()
	if bar >= len(p.Measures) {
		t.Fatalf("the part has %d bars; bar %d was asked for",
			len(p.Measures), bar+1)
	}
	var out []string
	for _, v := range p.Measures[bar].Voices {
		for _, el := range v.Elements {
			switch e := el.(type) {
			case *model.Note:
				out = append(out, e.Pitch.String()+"/"+e.Dur.String())
			case *model.Chord:
				var names []string
				for _, n := range e.Notes {
					names = append(names, n.Pitch.String())
				}
				out = append(out, "["+strings.Join(names, " ")+"]/"+e.Dur.String())
			case *model.Rest:
				out = append(out, "r/"+e.Dur.String())
			}
		}
		out = append(out, "|")
	}
	return strings.Join(out, " ")
}

/* ---------- the tree ---------- */

func TestTheTreeIsReadExactly(t *testing.T) {
	e := newEnc()
	root := e.node("kScore",
		e.leaf("fileVersion", "1.1417"),
		e.node("flows", e.leaf("flowName", "One")))
	tr, err := decodeDTN(e.bytes(root))
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if got := tr.nameOf(0); got != "kScore" {
		t.Errorf("the root is %q, want kScore", got)
	}
	if got := tr.str(0, "fileVersion"); got != "1.1417" {
		t.Errorf("the version came back as %q", got)
	}
	if got := tr.text(tr.path(0, "flows", "flowName")); got != "One" {
		t.Errorf("the flow name came back as %q", got)
	}
}

func TestAFileThatIsNotAScoreIsRefused(t *testing.T) {
	for _, c := range []struct {
		why  string
		data []byte
	}{
		{"empty", nil},
		{"too short", []byte{4, 0, 0, 0}},
		{"the wrong magic", append([]byte{9, 9, 9, 9}, make([]byte, 40)...)},
	} {
		if _, err := decodeDTN(c.data); err == nil {
			t.Errorf("a file that is %s was read as a score", c.why)
		}
	}
}

func TestAScoreCutShortIsNotGuessedAt(t *testing.T) {
	// The node count is in the header, so a correct read ends on the last
	// byte of the file. Anything else means the records are not the shape
	// this believes, and a tree misread that way is not a score with a
	// mistake in it — it is a different score, built of whatever the bytes
	// happened to say. Better to refuse.
	e := newEnc()
	root := e.node("kScore", e.leaf("fileVersion", "1.1417"))
	full := e.bytes(root)
	for _, cut := range []int{1, 4, 11} {
		if _, err := decodeDTN(full[:len(full)-cut]); err == nil {
			t.Errorf("a score missing its last %d bytes was read anyway", cut)
		}
	}
	trailing := append(append([]byte{}, full...), 0, 0, 0)
	if _, err := decodeDTN(trailing); err == nil {
		t.Error("a score with bytes left over at the end was read anyway")
	}
}

/* ---------- the music ---------- */

func TestTheOpeningOfTheFifth(t *testing.T) {
	// Three quavers and a held note, after a quaver's rest, in two four,
	// in C minor. It is the clearest thing in music to be wrong about.
	raw := built(t, [2]string{"2", "4"}, "C", []played{
		{at: "1/2", dur: "1/2", pitch: 67},
		{at: "1", dur: "1/2", pitch: 67},
		{at: "3/2", dur: "1/2", pitch: 67},
		{at: "2", dur: "2", pitch: 63},
	})
	scores := read(t, raw)
	p := scores[0].Parts[0]

	if got := notesOf(t, p, 0); got != "r/1/8 G4/1/8 G4/1/8 G4/1/8 |" {
		t.Errorf("the first bar is %q", got)
	}
	// The held note is two crotchets, which is the whole of a two-four
	// bar: it fills the second bar and needs no tie.
	if got := notesOf(t, p, 1); got != "Eb4/1/2 |" {
		t.Errorf("the second bar is %q", got)
	}
	// And it is an E flat and not a D sharp, which is what reading the
	// pitches against the key in force rather than against no key at all
	// is for. The same number, and the wrong letter.
	if strings.Contains(notesOf(t, p, 1), "D#") {
		t.Error("the E flat was spelled as a D sharp: the key was not read")
	}
}

func TestTheBarlinesComeFromTheTimeSignature(t *testing.T) {
	// Dorico stores no bars at all. Where they fall follows from the
	// meter, and nothing else in the file says.
	raw := built(t, [2]string{"3", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 60},
		{at: "1", dur: "1", pitch: 62},
		{at: "2", dur: "1", pitch: 64},
		{at: "3", dur: "3", pitch: 65},
	})
	p := read(t, raw)[0].Parts[0]
	if len(p.Measures) != 2 {
		t.Fatalf("six crotchets of three four came out as %d bars, want 2",
			len(p.Measures))
	}
	if got := notesOf(t, p, 0); got != "C4/1/4 D4/1/4 E4/1/4 |" {
		t.Errorf("the first bar is %q", got)
	}
	if got := p.Time.Beats; got != 3 {
		t.Errorf("the part is in %d time, want 3", got)
	}
}

func TestANoteOverABarlineIsSplitAndTied(t *testing.T) {
	raw := built(t, [2]string{"2", "4"}, "C", []played{
		{at: "1", dur: "2", pitch: 60},
	})
	p := read(t, raw)[0].Parts[0]
	if len(p.Measures) < 2 {
		t.Fatalf("%d bars", len(p.Measures))
	}
	first, second := p.Measures[0].Voices[0], p.Measures[1].Voices[0]
	a, ok := first.Elements[len(first.Elements)-1].(*model.Note)
	if !ok {
		t.Fatalf("the first bar ends with %T, want a note", first.Elements[1])
	}
	b, ok := second.Elements[0].(*model.Note)
	if !ok {
		t.Fatalf("the second bar opens with %T, want a note", second.Elements[0])
	}
	if !a.TieStart {
		t.Error("the note is not tied on over the barline")
	}
	if !b.TieStop {
		t.Error("the note after the barline is not tied back")
	}
	if a.Pitch.MIDI() != b.Pitch.MIDI() {
		t.Error("the two halves of the tied note are different notes")
	}
}

func TestNotesTogetherAreAChord(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "0", dur: "4", pitch: 64},
		{at: "0", dur: "4", pitch: 67},
	})
	p := read(t, raw)[0].Parts[0]
	m := p.Measures[0]
	if len(m.Voices) != 1 {
		t.Fatalf("a chord came out as %d voices, want 1", len(m.Voices))
	}
	ch, ok := m.Voices[0].Elements[0].(*model.Chord)
	if !ok {
		t.Fatalf("the bar holds %T, want a chord", m.Voices[0].Elements[0])
	}
	if len(ch.Notes) != 3 {
		t.Errorf("the chord has %d noteheads, want 3", len(ch.Notes))
	}
}

func TestNotesThatOverlapBecomeTwoVoices(t *testing.T) {
	// A held note under a moving line, written in one of Dorico's voices
	// because its events carry their own lengths and nothing stops them
	// overlapping. gscore's voice is a line of one element after another,
	// so the same music written there would add up to twice the bar.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60}, // held for the whole bar
		{at: "0", dur: "1", pitch: 72},
		{at: "1", dur: "1", pitch: 74},
		{at: "2", dur: "1", pitch: 76},
		{at: "3", dur: "1", pitch: 77},
	})
	p := read(t, raw)[0].Parts[0]
	m := p.Measures[0]
	if len(m.Voices) != 2 {
		t.Fatalf("the bar came out as %d voices, want 2", len(m.Voices))
	}
	want := model.Duration{Num: 1, Den: 1}
	for i, v := range m.Voices {
		if got := v.Duration(); got.Cmp(want) != 0 {
			t.Errorf("voice %d holds %v, and the bar holds %v", i+1, got, want)
		}
	}
	if len(p.Measures) != 1 {
		t.Errorf("one bar of music came out as %d bars", len(p.Measures))
	}
}

func TestTheStaffKeepsItsClefAndItsPlayersName(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 48},
	})
	s := read(t, raw)[0]
	p := s.Parts[0]
	if p.Name != "Cello" {
		t.Errorf("the staff is called %q, want Cello", p.Name)
	}
	if p.Clef != model.Bass {
		t.Errorf("the staff is in the %v clef, want bass", p.Clef)
	}
	if s.Identity.Title != "A piece" {
		t.Errorf("the title is %q", s.Identity.Title)
	}
	if s.Identity.Composer != "Somebody" {
		t.Errorf("the composer is %q", s.Identity.Composer)
	}
}

func TestAMinorKeyIsTheSignatureOfItsRelativeMajor(t *testing.T) {
	// C minor is written with three flats, which is E flat major's
	// signature. gscore counts the flats; Dorico says "C" and "minor".
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
	})
	p := read(t, raw)[0].Parts[0]
	if p.Key.Fifths != -3 {
		t.Errorf("C minor came out with %d sharps, want three flats",
			p.Key.Fifths)
	}
	if p.Key.Mode != model.Minor {
		t.Errorf("the mode is %v, want minor", p.Key.Mode)
	}
}

func TestWhatWasNotReadIsReported(t *testing.T) {
	// The music is kept and the rest is said plainly. A reader that
	// refused the notes over a slur would be no use to anybody, and one
	// that passed over the slur in silence would be worse: somebody finds
	// out by looking for a crescendo that is not there.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
	}, "SlurEventDefinition")

	scores, err := Import(bytes.NewReader(raw))
	if len(scores) == 0 {
		t.Fatal("the music was thrown away over a slur")
	}
	pe, ok := err.(*PartialError)
	if !ok {
		t.Fatalf("a project with a slur in it reported %v", err)
	}
	pe.Partial()
	if len(pe.Details()) == 0 || !strings.Contains(pe.Details()[0], "slur") {
		t.Errorf("the report says %v, and does not mention the slur",
			pe.Details())
	}
}

func TestAProjectThatIsNotOneSaysSo(t *testing.T) {
	if _, err := Import(strings.NewReader("this is not a zip")); err == nil {
		t.Error("a file that is not a container was read as a project")
	}
	// A container with nothing in it is not a project either.
	var empty bytes.Buffer
	zip.NewWriter(&empty).Close()
	if _, err := Import(bytes.NewReader(empty.Bytes())); err == nil {
		t.Error("an empty container was read as a project")
	}
}

func TestAProjectWithNoMusicInItIsStillOpened(t *testing.T) {
	// A flow with no notes is a movement somebody has set up and not
	// written yet, and opening it on an empty staff is what they want.
	raw := built(t, [2]string{"4", "4"}, "C", nil)
	p := read(t, raw)[0].Parts[0]
	if len(p.Measures) != 1 {
		t.Fatalf("an empty movement came out as %d bars, want 1",
			len(p.Measures))
	}
	if _, ok := p.Measures[0].Voices[0].Elements[0].(*model.Rest); !ok {
		t.Error("the empty bar does not hold a rest")
	}
}

/* ---------- the numbers ---------- */

func TestTheRationalsAreExact(t *testing.T) {
	// A triplet quaver is a third of a crotchet, and three of them make a
	// crotchet to the barline and to nothing else. Rounding any of this to
	// a decimal loses bars.
	for _, c := range []struct {
		in   string
		n, d int64
	}{
		{"0", 0, 1}, {"35", 35, 1}, {"69/2", 69, 2}, {"1/3", 1, 3},
		{"2137/4", 2137, 4}, {"-1/2", -1, 2}, {"4/8", 1, 2},
	} {
		got, ok := parseRat(c.in)
		if !ok && c.in != "0" {
			t.Errorf("%q was not read", c.in)
			continue
		}
		if got.n != c.n || got.d != c.d {
			t.Errorf("%q came out as %d/%d, want %d/%d",
				c.in, got.n, got.d, c.n, c.d)
		}
	}
	for _, bad := range []string{"", "x", "1/0", "1/x", "/2"} {
		if _, ok := parseRat(bad); ok {
			t.Errorf("%q was read as a number", bad)
		}
	}
	// Three triplet quavers are a crotchet exactly.
	third, _ := parseRat("1/3")
	sum := third.add(third).add(third)
	if sum.n != 1 || sum.d != 1 {
		t.Errorf("three thirds came to %d/%d", sum.n, sum.d)
	}
	// And a crotchet is a quarter of a whole note, which is how gscore
	// counts.
	if got := (rat{1, 1}).dur(); got != (model.Duration{Num: 1, Den: 4}) {
		t.Errorf("a crotchet came out as %v", got)
	}
}

/* ---------- tuplets, and the instruments ---------- */

func TestATripletIsReadAsATriplet(t *testing.T) {
	// Three notes in the time of two. The durations alone cannot say it —
	// a triplet quaver is a twelfth of a whole note and no notehead spells
	// a twelfth — so a reader that drops the tuplet leaves gscore drawing
	// crotchets where the page wants three beamed quavers under a 3.
	raw := built(t, [2]string{"2", "4"}, "C", []played{
		{at: "0", dur: "1/3", pitch: 60},
		{at: "1/3", dur: "1/3", pitch: 62},
		{at: "2/3", dur: "1/3", pitch: 64},
		{at: "1", dur: "1", pitch: 65},
	}, tupletOver("0", "1", 3, 2))

	p := read(t, raw)[0].Parts[0]
	els := p.Measures[0].Voices[0].Elements
	if len(els) != 4 {
		t.Fatalf("the bar holds %d elements, want 4", len(els))
	}
	for i := 0; i < 3; i++ {
		n, ok := els[i].(*model.Note)
		if !ok {
			t.Fatalf("element %d is %T, want a note", i+1, els[i])
		}
		if n.Tuplet == nil {
			t.Fatalf("note %d is not in a tuplet", i+1)
		}
		if n.Tuplet.Actual != 3 || n.Tuplet.Normal != 2 {
			t.Errorf("note %d is %d in the time of %d, want 3 in 2",
				i+1, n.Tuplet.Actual, n.Tuplet.Normal)
		}
		// And written, it is a quaver: the sounding twelfth scaled back up
		// by the ratio, which is what makes it drawable at all.
		if got := n.WrittenDuration(n.Dur); got.Cmp(
			model.Duration{Num: 1, Den: 8}) != 0 {
			t.Errorf("note %d is written as %v, want an eighth", i+1, got)
		}
	}
	first := els[0].(*model.Note)
	last := els[2].(*model.Note)
	if !first.Tuplet.Start || first.Tuplet.Stop {
		t.Error("the first note of the triplet does not open the bracket")
	}
	if last.Tuplet.Stop != true || last.Tuplet.Start {
		t.Error("the last note of the triplet does not close the bracket")
	}
	// The note after it is not in the tuplet.
	if n, ok := els[3].(*model.Note); ok && n.Tuplet != nil {
		t.Error("the note after the triplet was swept into it")
	}
}

func TestEveryDurationCanBeDrawn(t *testing.T) {
	// A stretch cut at a barline is any length at all, and most lengths
	// have no notehead: five eighths of a whole note is a minim tied to a
	// quaver. Written as one it comes out as a crotchet, because that is
	// the engraver's fallback for a duration nobody can spell.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1/2", pitch: 60},
		{at: "1/2", dur: "5", pitch: 62}, // over the barline, and odd
	})
	p := read(t, raw)[0].Parts[0]
	for i, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if _, _, ok := el.Duration().Written(); !ok {
					t.Errorf("bar %d holds a %v, which no notehead spells",
						i+1, el.Duration())
				}
			}
		}
	}
}

func TestTheStaffPlaysItsOwnInstrument(t *testing.T) {
	// Without this every staff of an orchestral score is a piano: gscore
	// falls back to one when a file names no instrument, and this file
	// names them all.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
	})
	p := read(t, raw)[0].Parts[0]
	if p.Sound != "wind.flutes.flute" {
		t.Errorf("the staff plays %q, want wind.flutes.flute", p.Sound)
	}
	if p.Instrument == model.AcousticGrandPiano {
		t.Error("the staff came out as a piano")
	}
	if got, _ := model.SoundProgram("wind.flutes.flute"); p.Instrument != got {
		t.Errorf("the staff is played with %v, want %v", p.Instrument, got)
	}
}

/* ---------- the dynamics ---------- */

func TestADynamicLandsOnTheNoteItStandsOver(t *testing.T) {
	raw := built(t, [2]string{"2", "4"}, "C", []played{
		{at: "0", dur: "1/2", pitch: 60},
		{at: "1/2", dur: "1/2", pitch: 62},
		{at: "1", dur: "1", pitch: 64},
	}, dynamicAt("1/2", dynSub{kind: "immediate", at: "0", side: 0, letters: 2}))

	p := read(t, raw)[0].Parts[0]
	els := p.Measures[0].Voices[0].Elements
	if got := marksOf(els[0]).Dynamic; got != model.DynamicNone {
		t.Errorf("the first note carries %v, and the mark is over the second",
			got)
	}
	if got := marksOf(els[1]).Dynamic; got != model.FF {
		t.Errorf("the second note carries %v, want ff", got)
	}
}

func TestTheLoudnessesAreReadRight(t *testing.T) {
	// Which side of the middle, and how many letters. The opening of this
	// symphony is ff and is the hardest thing in music to be wrong about,
	// so it is what the reading was anchored on.
	for _, c := range []struct {
		side, letters int
		want          model.Dynamic
	}{
		{0, 1, model.Forte}, {0, 2, model.FF}, {0, 3, model.FFF},
		{1, 1, model.Piano}, {1, 2, model.PP}, {1, 3, model.PPP},
		{0, 0, model.MF}, {1, 0, model.MP},
	} {
		raw := built(t, [2]string{"4", "4"}, "C", []played{
			{at: "0", dur: "4", pitch: 60},
		}, dynamicAt("0", dynSub{kind: "immediate", at: "0",
			side: c.side, letters: c.letters}))
		p := read(t, raw)[0].Parts[0]
		got := marksOf(p.Measures[0].Voices[0].Elements[0]).Dynamic
		if got != c.want {
			t.Errorf("side %d with %d letters came out as %v, want %v",
				c.side, c.letters, got, c.want)
		}
	}
}

func TestAHairpinOpensAndCloses(t *testing.T) {
	// p, growing, to f — which Dorico keeps as one gesture of three parts,
	// and gscore as a dynamic, a wedge opening, and a wedge closing.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 60},
		{at: "1", dur: "1", pitch: 62},
		{at: "2", dur: "1", pitch: 64},
		{at: "3", dur: "1", pitch: 65},
	}, dynamicAt("0",
		dynSub{kind: "immediate", at: "0", side: 1, letters: 1},
		dynSub{kind: "gradual", at: "0", dur: "3"},
		dynSub{kind: "immediate", at: "3", side: 0, letters: 1}))

	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	if got := marksOf(els[0]).Dynamic; got != model.Piano {
		t.Errorf("the gesture opens on %v, want p", got)
	}
	open := marksOf(els[0]).Hairpin
	if open == nil || !open.Start || open.Kind != model.Crescendo {
		t.Fatalf("the crescendo does not open on the first note: %+v", open)
	}
	shut := marksOf(els[3]).Hairpin
	if shut == nil || !shut.Stop {
		t.Fatalf("the crescendo does not close on the fourth note: %+v", shut)
	}
	if got := marksOf(els[3]).Dynamic; got != model.Forte {
		t.Errorf("the gesture arrives at %v, want f", got)
	}
}

func TestADiminuendoFalls(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "2", pitch: 60},
		{at: "2", dur: "2", pitch: 62},
	}, dynamicAt("0", dynSub{kind: "gradual", at: "0", dur: "2", down: true}))

	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	h := marksOf(els[0]).Hairpin
	if h == nil || h.Kind != model.Diminuendo {
		t.Fatalf("the hairpin is %+v, want a diminuendo", h)
	}
}

func TestASforzandoIsNotALoudness(t *testing.T) {
	// The struck dynamics are a kind of their own in Dorico, named rather
	// than numbered — which is a mercy, because sf, sfz, fz and rfz look
	// alike on the page and are four different instructions.
	for _, c := range []struct {
		kind string
		want model.Dynamic
	}{
		{"kForceDynamicSfz", model.SForzando},
		{"kForceDynamicSf", model.SForzato},
		{"kForceDynamicRfz", model.Rinforzando},
		{"kForceDynamicFp", model.FortePiano},
	} {
		raw := built(t, [2]string{"4", "4"}, "C", []played{
			{at: "0", dur: "4", pitch: 60},
		}, dynamicAt("0", dynSub{kind: "force", at: "0", force: c.kind}))
		got := marksOf(read(t, raw)[0].Parts[0].
			Measures[0].Voices[0].Elements[0]).Dynamic
		if got != c.want {
			t.Errorf("%s came out as %v, want %v", c.kind, got, c.want)
		}
	}
}

func TestADynamicOverALongNoteBelongsToThatNote(t *testing.T) {
	// Written where no note begins, which happens: the mark sits over the
	// middle of a held note. It belongs to the note being held.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
	}, dynamicAt("2", dynSub{kind: "immediate", at: "0", side: 1, letters: 2}))

	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	if got := marksOf(els[0]).Dynamic; got != model.PP {
		t.Errorf("the held note carries %v, want pp", got)
	}
}
