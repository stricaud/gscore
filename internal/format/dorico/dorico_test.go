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
	// modifier is the qualifier printed in front of the letters, as the
	// number the file holds it as and not as a word: 2 is "più". Nothing
	// names them anywhere in the file, so the reader goes by the number
	// and so does this.
	modifier int
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

// tempos and clefChanges are what a test asked to be written onto the
// global and staff streams, by the key their helpers gave back.
var tempos = map[string]struct {
	at, text  string
	micros    string
	unit, dot int
}{}

var clefChanges = map[string]struct{ at, clef string }{}

// tempoAt asks for a tempo mark: so many microseconds to the beat, counted
// in a note named the way Dorico names one.
func tempoAt(at, text, micros string, unit, dot int) string {
	key := "tempo " + at + " " + itoa(len(tempos))
	tempos[key] = struct {
		at, text  string
		micros    string
		unit, dot int
	}{at, text, micros, unit, dot}
	return key
}

// clefAt asks for a clef, at the start of the movement or partway through.
func clefAt(at, clef string) string {
	key := "clef " + at + " " + itoa(len(clefChanges))
	clefChanges[key] = struct{ at, clef string }{at, clef}
	return key
}

// noClefEvent asks for a staff whose stream says nothing about its clef,
// which is the ordinary case: a flow writes one only where something
// happens.
const noClefEvent = "no clef event"

// meters holds extra bar divisions a test asked for.
var meters = map[string][]struct{ at, num, den string }{}

// meterAt asks for bar divisions after the first: each one a barline, and
// a time signature that may or may not be a change.
func meterAt(sigs ...[3]string) string {
	key := "meters " + itoa(len(meters))
	var out []struct{ at, num, den string }
	for _, sig := range sigs {
		out = append(out, struct{ at, num, den string }{sig[0], sig[1], sig[2]})
	}
	meters[key] = out
	return key
}

// slurs holds the phrase marks a test asked for.
var slurSpans = map[string]struct{ at, dur string }{}

// slurOver asks for a slur from a moment, reaching so far. Dorico measures
// it to the start of the last note under it, so a slur "for one crotchet"
// joins two crotchets.
func slurOver(at, dur string) string {
	key := "slur " + at + " " + dur + " " + itoa(len(slurSpans))
	slurSpans[key] = struct{ at, dur string }{at, dur}
	return key
}

// graces holds the little notes a test asked for.
var graceNotes = map[string][]struct {
	at, order, dur string
	pitch          int
	slashed        bool
}{}

// graceBefore asks for grace notes leaning into the note at `at`. Dorico
// counts them backwards from it, so the earlier of a pair is the more
// negative.
func graceBefore(at string, each ...[3]string) string {
	key := "grace " + at + " " + itoa(len(graceNotes))
	var out []struct {
		at, order, dur string
		pitch          int
		slashed        bool
	}
	for _, g := range each {
		pitch := 0
		for _, c := range g[1] {
			pitch = pitch*10 + int(c-'0')
		}
		out = append(out, struct {
			at, order, dur string
			pitch          int
			slashed        bool
		}{at, g[0], g[2], pitch, false})
	}
	graceNotes[key] = out
	return key
}

// rawEvents holds events a test asked to be put on a stream as they are,
// which is the general way to ask for one: the registries above are for
// the kinds that need something built around them.
var rawEvents = map[string]struct {
	stream string // "voice", "global" or "stave"
	name   string
	fields [][2]string
	// child is one nested node, for the events that carry their data in
	// one: a pause's pauseData, a tempo's data.
	child     string
	childKids [][2]string
}{}

// eventOn asks for one event on one of a flow's streams.
func eventOn(stream, name string, fields ...[2]string) string {
	key := "event " + name + " " + itoa(len(rawEvents))
	rawEvents[key] = struct {
		stream    string
		name      string
		fields    [][2]string
		child     string
		childKids [][2]string
	}{stream: stream, name: name, fields: fields}
	return key
}

// eventWith is the same, with a nested node of its own.
func eventWith(stream, name string, fields [][2]string, child string,
	kids ...[2]string) string {
	key := eventOn(stream, name, fields...)
	e := rawEvents[key]
	e.child, e.childKids = child, kids
	rawEvents[key] = e
	return key
}

// made builds one of those with an encoder.
func made(e *enc, key string) int32 {
	r := rawEvents[key]
	var kids []int32
	for _, f := range r.fields {
		kids = append(kids, e.leaf(f[0], f[1]))
	}
	if r.child != "" {
		var inner []int32
		for _, f := range r.childKids {
			inner = append(inner, e.leaf(f[0], f[1]))
		}
		kids = append(kids, e.node(r.child, inner...))
	}
	return e.node(r.name, kids...)
}

// textEvents and voltas are the two that carry their data deeper than a
// field or two: a text is a document of paragraphs and fragments, and a
// volta keeps the length of each time through in its property table.
var textEvents = map[string]struct{ at, words string }{}
var voltas = map[string]struct {
	at, dur  string
	segments []string
}{}

func textAt(at, words string) string {
	key := "text " + at + " " + itoa(len(textEvents))
	textEvents[key] = struct{ at, words string }{at, words}
	return key
}

func voltaAt(at, dur string, segments ...string) string {
	key := "volta " + at + " " + itoa(len(voltas))
	voltas[key] = struct {
		at, dur  string
		segments []string
	}{at, dur, segments}
	return key
}

// keyTurns holds the key changes a test asked for on a staff.
var keyTurns = map[string]struct{ at, note string }{}

// keyTurnAt asks for a key change on the staff at a moment, named the way
// Dorico names one: the note it is rooted on, read as a minor key.
func keyTurnAt(at, note string) string {
	key := "keyturn " + at + " " + itoa(len(keyTurns))
	keyTurns[key] = struct{ at, note string }{at, note}
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
		if _, ok := tempos[kind]; ok {
			continue // built onto the global stream below
		}
		if _, ok := clefChanges[kind]; ok {
			continue // built onto the staff below
		}
		if _, ok := meters[kind]; ok {
			continue // built onto the global stream below
		}
		if _, ok := voltas[kind]; ok {
			continue // built onto the global stream below
		}
		if _, ok := keyTurns[kind]; ok {
			continue // built onto the staff below
		}
		if kind == noClefEvent {
			continue
		}
		if r, ok := rawEvents[kind]; ok {
			if r.stream == "voice" {
				events = append(events, made(e, kind))
			}
			continue
		}
		if tx, ok := textEvents[kind]; ok {
			events = append(events, e.node("TextEventDefinition",
				e.leaf("position", tx.at),
				e.leaf("duration", "0"),
				e.node("content",
					e.node("paragraphs",
						e.node("paragraph",
							e.node("fragments",
								e.node("fragment",
									e.leaf("content", tx.words))))))))
			continue
		}
		if run, ok := graceNotes[kind]; ok {
			for _, g := range run {
				events = append(events, e.node("GraceNoteEventDefinition",
					e.leaf("position", g.at),
					e.leaf("secondaryPosition", g.order),
					e.leaf("duration", "0"),
					e.leaf("notatedDuration", g.dur),
					e.leaf("slashed", "false"),
					e.leaf("pitch", itoa(g.pitch))))
			}
			continue
		}
		if sl, ok := slurSpans[kind]; ok {
			events = append(events, e.node("SlurEventDefinition",
				e.leaf("position", sl.at),
				e.leaf("duration", sl.dur)))
			continue
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

	globalEvents := []int32{
		e.node("BarDivisionEventDefinition",
			e.leaf("position", "0"),
			e.leaf("duration", "0"),
			e.leaf("barDivisionElementID", "1")),
		e.node("TonalityDivisionEventDefinition",
			e.leaf("position", "0"),
			e.leaf("duration", "0"),
			e.leaf("tonalityDivisionElementID", "2")),
	}
	for _, kind := range extra {
		if r, ok := rawEvents[kind]; ok && r.stream == "global" {
			globalEvents = append(globalEvents, made(e, kind))
		}
		if v, ok := voltas[kind]; ok {
			entries := []int32{e.node("entry",
				e.leaf("propertyType", "kRepeatEndingLineNumSegments"),
				e.leaf("value", "int: "+itoa(len(v.segments))))}
			for _, seg := range v.segments {
				entries = append(entries, e.node("entry",
					e.leaf("propertyType",
						"kRepeatEndingLineSegmentDuration"),
					e.leaf("value", `string: "`+seg+`"`)))
			}
			globalEvents = append(globalEvents,
				e.node("RepeatEndingLineEventDefinition",
					e.leaf("position", v.at),
					e.leaf("duration", v.dur),
					e.node("propertyTable", e.node("entries", entries...))))
		}
	}
	var moreSigs []int32
	for _, kind := range extra {
		for i, sig := range meters[kind] {
			id := itoa(900 + i)
			moreSigs = append(moreSigs,
				e.node("BarDivisionElementDefinition",
					e.leaf("elementID", id),
					e.node("barDivisionData",
						e.node("timeSignature",
							e.node("timeSignaturesAndDivisions",
								e.node("timeSignatureAndDivision",
									e.node("timeSignature",
										e.leaf("numerator", sig.num),
										e.leaf("denominator", sig.den))))))))
			globalEvents = append(globalEvents,
				e.node("BarDivisionEventDefinition",
					e.leaf("position", sig.at),
					e.leaf("duration", "0"),
					e.leaf("barDivisionElementID", id)))
		}
	}
	for _, kind := range extra {
		tp, ok := tempos[kind]
		if !ok {
			continue
		}
		data := []int32{e.leaf("text", tp.text)}
		if tp.micros != "" {
			data = append(data, e.node("absoluteTempo",
				e.leaf("tempoValue", tp.micros),
				e.leaf("beatUnits", "steamVariantList:list: {list: {int: "+
					itoa(tp.unit)+"}, {int: "+itoa(tp.dot)+"}}")))
		}
		globalEvents = append(globalEvents,
			e.node("ImmediateTempoChangeEventDefinition",
				e.leaf("position", tp.at),
				e.node("data", data...)))
	}
	globalBlock := e.node("block",
		e.leaf("blockID", "20"),
		e.leaf("parentEventStreamType", "kGlobalTimebaseStream"),
		e.node("events", globalEvents...))

	var staveEvents []int32
	quiet := false
	for _, kind := range extra {
		if kind == noClefEvent {
			quiet = true
		}
	}
	if !quiet {
		staveEvents = append(staveEvents,
			e.node("ClefEventDefinition",
				e.leaf("position", "0"),
				e.node("entityIDs",
					e.leaf("idForTransposingLayouts", "clef.bass"))))
	}
	for _, kind := range extra {
		c, ok := clefChanges[kind]
		if !ok {
			continue
		}
		staveEvents = append(staveEvents,
			e.node("ClefEventDefinition",
				e.leaf("position", c.at),
				e.node("entityIDs",
					e.leaf("idForTransposingLayouts", c.clef))))
	}
	var moreKeys []int32
	var dynDefs []int32
	for _, kind := range extra {
		if r, ok := rawEvents[kind]; ok && r.stream == "stave" {
			staveEvents = append(staveEvents, made(e, kind))
		}
		kt, ok := keyTurns[kind]
		if !ok {
			continue
		}
		id := "77"
		moreKeys = append(moreKeys, e.node("TonalityDivisionElementDefinition",
			e.leaf("elementID", id),
			e.node("tonalityDivisionData",
				e.node("keySignature",
					e.node("root",
						e.leaf("tonalityType", "kKeySigMinor"),
						e.node("root",
							e.leaf("noteName", kt.note),
							e.leaf("accidentalID",
								"accidental.12et.western.natural")))))))
		staveEvents = append(staveEvents,
			e.node("TonalityDivisionEventDefinition",
				e.leaf("position", kt.at),
				e.leaf("duration", "0"),
				e.leaf("tonalityDivisionElementID", id)))
	}
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
				mod := "steamVariantList:list: {}, {}"
				if sub.modifier != 0 {
					mod = "steamVariantList:list: {}, {int: " +
						itoa(sub.modifier) + "}"
				}
				subs = append(subs, e.node("ImmediateChangeDynamicDefinition",
					e.leaf("position", sub.at),
					e.leaf("intensity",
						"steamVariantList:list: {int: "+itoa(sub.side)+
							"}, {}, {bool: false}, {int: "+
							itoa(sub.letters)+"}"),
					e.leaf("intensityModifier", mod)))
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
				e.node("array", append([]int32{timeSig}, moreSigs...)...)),
			e.node("TonalityDivisionElementTableDefinition",
				e.node("array", append([]int32{keySig}, moreKeys...)...)),
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
					e.leaf("musicXMLSoundID", "wind.flutes.flute"),
					// What its staves are written in, which is what a
					// staff with no clef event of its own takes.
					e.node("clefs",
						e.node("clefIDsForEachStave",
							e.node("staveClefIDs",
								e.leaf("idForTransposingLayouts",
									"clef.alto"))))))),
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

func TestAQualifiedDynamicKeepsItsWord(t *testing.T) {
	// "più p" is a p with a word in front of it, and the word is the whole
	// instruction: a p that is already p and is asked for again says
	// nothing, while "più p" says softer than you are playing.
	//
	// gscore has no qualifier of its own, so the word is written beside the
	// mark as an expression — italic and under the staff, the side the
	// dynamic is on, which is where Dorico prints it.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
	}, dynamicAt("0", dynSub{kind: "immediate", at: "0", side: 1,
		letters: 1, modifier: 2}))

	p := read(t, raw)[0].Parts[0]
	mk := marksOf(p.Measures[0].Voices[0].Elements[0])
	if mk.Dynamic != model.Piano {
		t.Errorf("the note carries %v, want p", mk.Dynamic)
	}
	if len(mk.Texts) != 1 {
		t.Fatalf("the note carries %d words beside the p, want the one: %+v",
			len(mk.Texts), mk.Texts)
	}
	if mk.Texts[0].Text != "più" {
		t.Errorf("the word beside the p is %q, want \"più\"", mk.Texts[0].Text)
	}
	if mk.Texts[0].Style != model.TextExpression {
		t.Errorf("the word is set as %v, and belongs with the dynamics",
			mk.Texts[0].Style)
	}
}

func TestAnUnknownQualifierIsReportedAndTheDynamicKept(t *testing.T) {
	// Only "più" was read off Dorico's own engraving of the project. The
	// rest are numbers nothing names, so a dynamic that has one is still a
	// dynamic and the word is said to be missing rather than guessed at.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
	}, dynamicAt("0", dynSub{kind: "immediate", at: "0", side: 0,
		letters: 1, modifier: 7}))

	scores, err := Import(bytes.NewReader(raw))
	if len(scores) == 0 {
		t.Fatal("the music was thrown away over a word in front of a dynamic")
	}
	mk := marksOf(scores[0].Parts[0].Measures[0].Voices[0].Elements[0])
	if mk.Dynamic != model.Forte {
		t.Errorf("the note carries %v, want f: the qualifier is not the dynamic",
			mk.Dynamic)
	}
	if len(mk.Texts) != 0 {
		t.Errorf("a word nobody can name was written anyway: %+v", mk.Texts)
	}
	pe, ok := err.(*PartialError)
	if !ok {
		t.Fatalf("a qualifier gscore does not know reported %v", err)
	}
	pe.Partial()
	if !strings.Contains(strings.Join(pe.Details(), " "), "in front of a dynamic") {
		t.Errorf("the report says %v, and does not mention the word in front "+
			"of the dynamic", pe.Details())
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

/* ---------- the clefs, and the speed ---------- */

// builtNoClef is a project whose staff stream says nothing about its clef.
func builtNoClef(t *testing.T, notes []played) []byte {
	t.Helper()
	return built(t, [2]string{"4", "4"}, "C", notes, noClefEvent)
}

// builtTwoMeters restates the opening meter once and then really changes
// it, which is what Dorico's bar divisions look like.
func builtTwoMeters(t *testing.T) []byte {
	t.Helper()
	return built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
		{at: "8", dur: "3", pitch: 64},
	}, meterAt([3]string{"4", "4", "4"}, [3]string{"8", "3", "4"}))
}

func TestAStaffWithNoClefEventTakesItsInstrumentsOwn(t *testing.T) {
	// A flow writes a clef event only where something happens, so a double
	// bass that reads bass clef all movement has none at all. Read from
	// the events alone it comes out in whatever the reader's default was,
	// which is how one ended up in the treble clef with every note eight
	// ledger lines under the staff.
	raw := builtNoClef(t, []played{{at: "0", dur: "4", pitch: 48}})
	p := read(t, raw)[0].Parts[0]
	if p.Clef != model.Alto {
		t.Errorf("the staff is in the %v clef, want the alto its "+
			"instrument is written in", p.Clef)
	}
}

func TestAClefAtTheStartWinsOverTheInstruments(t *testing.T) {
	// The instrument says alto; this flow's own stream says bass at the
	// very first moment, and what the flow says goes.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 48},
	})
	if got := read(t, raw)[0].Parts[0].Clef; got != model.Bass {
		t.Errorf("the staff is in the %v clef, want bass", got)
	}
}

func TestAClefChangeIsKeptAgainstItsBar(t *testing.T) {
	// A cello reads bass and goes up to tenor for a high passage. Read as
	// though the earliest event were the opening clef, it starts the
	// movement in tenor and the whole first page sits wrong.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 48},
		{at: "4", dur: "4", pitch: 60},
		{at: "8", dur: "4", pitch: 48},
	}, clefAt("4", "clef.tenor"))

	p := read(t, raw)[0].Parts[0]
	if p.Clef != model.Bass {
		t.Errorf("the staff opens in the %v clef, want bass", p.Clef)
	}
	if p.Measures[1].Clef == nil || *p.Measures[1].Clef != model.Tenor {
		t.Errorf("the second bar does not change to tenor: %v",
			p.Measures[1].Clef)
	}
	if p.Measures[0].Clef != nil {
		t.Error("the first bar carries a clef change it should not")
	}
}

func TestTheSpeedIsReadAndSoIsWhatItIsCountedIn(t *testing.T) {
	// Microseconds to the beat, which is MIDI's way of saying it, counted
	// in a note named by a number falling from a whole one. Beethoven's
	// own mark for this symphony is minim 108, and 555555 microseconds is
	// 108 to the minute.
	for _, c := range []struct {
		micros    string
		unit, dot int
		bpm       int
		want      model.Duration
	}{
		{"555555", 9, 0, 108, model.Duration{Num: 1, Den: 2}},  // minim
		{"652173", 7, 0, 92, model.Duration{Num: 1, Den: 8}},   // quaver
		{"625000", 9, 1, 96, model.Duration{Num: 3, Den: 4}},   // dotted minim
		{"535714", 10, 0, 112, model.Duration{Num: 1, Den: 1}}, // semibreve
		{"500000", 8, 0, 120, model.Duration{Num: 1, Den: 4}},  // crotchet
	} {
		raw := built(t, [2]string{"4", "4"}, "C", []played{
			{at: "0", dur: "4", pitch: 60},
		}, tempoAt("0", "Allegro", c.micros, c.unit, c.dot))
		s := read(t, raw)[0]
		if s.Tempo.BPM != c.bpm {
			t.Errorf("%s microseconds came out as %d a minute, want %d",
				c.micros, s.Tempo.BPM, c.bpm)
		}
		if s.Tempo.Unit.Cmp(c.want) != 0 {
			t.Errorf("unit %d with %d dots came out as %v, want %v",
				c.unit, c.dot, s.Tempo.Unit, c.want)
		}
		if s.Tempo.Text != "Allegro" {
			t.Errorf("the words came out as %q", s.Tempo.Text)
		}
	}
}

func TestAChangeOfSpeedIsKeptAgainstItsBar(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
	},
		tempoAt("0", "Allegro con brio", "555555", 9, 0),
		tempoAt("4", "Adagio", "3000000", 8, 0))

	s := read(t, raw)[0]
	if s.Tempo.BPM != 108 {
		t.Errorf("the movement opens at %d", s.Tempo.BPM)
	}
	m := s.Parts[0].Measures[1]
	if m.Tempo == nil {
		t.Fatal("the second bar carries no change of speed")
	}
	if m.Tempo.BPM != 20 || m.Tempo.Text != "Adagio" {
		t.Errorf("the second bar is %d %q, want 20 Adagio",
			m.Tempo.BPM, m.Tempo.Text)
	}
}

func TestTempoIGoesBackToTheOpeningSpeed(t *testing.T) {
	// "Tempo I" and "a tempo" name no number: they send the player back.
	// The words are printed and no metronome mark with them, which is how
	// every score writes it.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
	},
		tempoAt("0", "Allegro", "555555", 9, 0),
		tempoAt("4", "Tempo I", "", 0, 0))

	m := read(t, raw)[0].Parts[0].Measures[1]
	if m.Tempo == nil {
		t.Fatal("the second bar carries no tempo")
	}
	if m.Tempo.BPM != 108 {
		t.Errorf("Tempo I came back at %d, want the opening 108", m.Tempo.BPM)
	}
	if !m.Tempo.Silent {
		t.Error("Tempo I prints a metronome mark nobody wrote")
	}
	if m.Tempo.Text != "Tempo I" {
		t.Errorf("the words came out as %q", m.Tempo.Text)
	}
}

func TestAMeterRestatedIsNotAMeterChange(t *testing.T) {
	// Dorico writes a bar division wherever the barline arrangement is
	// settled — a repeat end, the join of two sections — and most of them
	// restate the meter already in force. Printed as written, the finale
	// of this symphony carries ten four-fours, nine of them in the middle
	// of music that never leaves four-four.
	raw := builtTwoMeters(t)
	p := read(t, raw)[0].Parts[0]
	sigs := 0
	for _, m := range p.Measures {
		if m.Time != nil {
			sigs++
		}
	}
	if sigs != 2 {
		t.Errorf("%d time signatures were written, want 2: the opening "+
			"one and the real change", sigs)
	}
	if p.Measures[0].Time == nil {
		t.Error("the first bar carries no time signature")
	}
}

/* ---------- the slurs ---------- */

// phrase lists a part's slur marks bar by bar, as "C4( D4 E4)".
func phrase(p *model.Part, bar int) string {
	var out []string
	for _, el := range p.Measures[bar].Voices[0].Elements {
		mk := marksOf(el)
		name := ""
		switch e := el.(type) {
		case *model.Note:
			name = e.Pitch.String()
		case *model.Chord:
			name = "chord"
		case *model.Rest:
			name = "r"
		}
		if mk != nil && mk.SlurStart > 0 {
			name += "("
		}
		if mk != nil && mk.SlurStop > 0 {
			name += ")"
		}
		out = append(out, name)
	}
	return strings.Join(out, " ")
}

func TestASlurOpensAndClosesOnTheRightNotes(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 60},
		{at: "1", dur: "1", pitch: 62},
		{at: "2", dur: "1", pitch: 64},
		{at: "3", dur: "1", pitch: 65},
	}, slurOver("0", "2"))

	// The span reaches to the start of the third note, so the phrase is
	// the first three and not the first two.
	if got := phrase(read(t, raw)[0].Parts[0], 0); got != "C4( D4 E4) F4" {
		t.Errorf("the bar reads %q", got)
	}
}

func TestTheSmallestSlurJoinsTwoNotes(t *testing.T) {
	// "For one crotchet" is two crotchets under one slur. Measured to the
	// end of the last note instead, every phrase in the score would take
	// in the note after it.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 60},
		{at: "1", dur: "1", pitch: 62},
		{at: "2", dur: "1", pitch: 64},
	}, slurOver("0", "1"))

	// The fourth beat of the bar is silence, which the reader fills in.
	if got := phrase(read(t, raw)[0].Parts[0], 0); got != "C4( D4) E4 r" {
		t.Errorf("the bar reads %q", got)
	}
}

func TestASlurOverABarlineOpensOnceAndClosesOnce(t *testing.T) {
	// A note held across a barline is written as two tied noteheads, and
	// each bar is laid out on its own — so the opening of a phrase was
	// being written again at the start of every bar the first note
	// reached into, leaving the score with more slurs opened than closed.
	raw := built(t, [2]string{"2", "4"}, "C", []played{
		{at: "0", dur: "3", pitch: 60}, // over the barline
		{at: "3", dur: "1", pitch: 62},
	}, slurOver("0", "3"))

	p := read(t, raw)[0].Parts[0]
	starts, stops := 0, 0
	for _, m := range p.Measures {
		for _, el := range m.Voices[0].Elements {
			mk := marksOf(el)
			if mk == nil {
				continue
			}
			if mk.SlurStart > 0 {
				starts++
			}
			if mk.SlurStop > 0 {
				stops++
			}
		}
	}
	if starts != 1 || stops != 1 {
		t.Errorf("the phrase opens %d times and closes %d, want once each",
			starts, stops)
	}
}

func TestASlurIsDrawnBetweenNotesAndNotRests(t *testing.T) {
	// Dorico's span is a length of time, so it can reach over a rest at
	// either end. Hung on the rest, nothing draws it and nothing closes
	// it.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 60},
		{at: "1", dur: "1", pitch: 62},
		{at: "2", dur: "1", rest: true},
	}, slurOver("0", "2"))

	if got := phrase(read(t, raw)[0].Parts[0], 0); got != "C4( D4) r r" {
		t.Errorf("the bar reads %q", got)
	}
}

func TestAGraceNotesOwnSlurIsReportedRatherThanHungOnNothing(t *testing.T) {
	// Every slur of no length in this corpus is a grace note's, leaning it
	// into the note it decorates. Until the grace notes are read there is
	// nothing to hang it on.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
	}, slurOver("0", "0"))

	scores, err := Import(bytes.NewReader(raw))
	if len(scores) == 0 {
		t.Fatal("the music was thrown away")
	}
	pe, ok := err.(*PartialError)
	if !ok {
		t.Fatalf("nothing was reported: %v", err)
	}
	found := false
	for _, line := range pe.Details() {
		if strings.Contains(line, "grace") {
			found = true
		}
	}
	if !found {
		t.Errorf("the report says %v", pe.Details())
	}
	if marksOf(scores[0].Parts[0].Measures[0].Voices[0].Elements[0]).SlurStart > 0 {
		t.Error("a slur with one note under it was drawn anyway")
	}
}

/* ---------- the grace notes ---------- */

func TestGraceNotesLeanOnTheNoteAndTakeNoTimeFromTheBar(t *testing.T) {
	// A bar of four quarters with a grace note before the third beat is
	// still a bar of four quarters: the little notes hang off the note
	// they decorate rather than standing in the voice beside it.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "2", pitch: 60},
		{at: "2", dur: "2", pitch: 67},
	}, graceBefore("2", [3]string{"-1/4", "65", "1/4"}))

	p := read(t, raw)[0].Parts[0]
	m := p.Measures[0]
	if got := m.Voices[0].Duration(); got.Cmp(model.Duration{Num: 1, Den: 1}) != 0 {
		t.Errorf("the bar holds %v, want a whole note's worth", got)
	}
	if len(m.Voices[0].Elements) != 2 {
		t.Fatalf("the bar holds %d elements, want 2",
			len(m.Voices[0].Elements))
	}
	lean, ok := m.Voices[0].Elements[1].(*model.Note)
	if !ok {
		t.Fatalf("the second element is %T", m.Voices[0].Elements[1])
	}
	if lean.Grace == nil || len(lean.Grace.Notes) != 1 {
		t.Fatalf("the note it leans into carries %v", lean.Grace)
	}
	if got := lean.Grace.Notes[0].Pitch.MIDI(); got != 65 {
		t.Errorf("the little note is %d, want 65", got)
	}
	// Written as a semiquaver: a quarter of a crotchet.
	if got := lean.Grace.Notes[0].Dur; got.Cmp(
		model.Duration{Num: 1, Den: 16}) != 0 {
		t.Errorf("the little note is written as %v, want a sixteenth", got)
	}
}

func TestAPairOfGraceNotesIsInPlayingOrder(t *testing.T) {
	// Dorico counts them backwards from the beat, so the one at -1/2 is
	// heard before the one at -1/4. Written in the order they are found,
	// a pair comes out backwards.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 72},
	}, graceBefore("0",
		[3]string{"-1/4", "69", "1/4"},
		[3]string{"-1/2", "67", "1/4"}))

	lean := read(t, raw)[0].Parts[0].
		Measures[0].Voices[0].Elements[0].(*model.Note)
	if lean.Grace == nil || len(lean.Grace.Notes) != 2 {
		t.Fatalf("the note carries %v", lean.Grace)
	}
	if a, b := lean.Grace.Notes[0].Pitch.MIDI(),
		lean.Grace.Notes[1].Pitch.MIDI(); a != 67 || b != 69 {
		t.Errorf("the pair reads %d %d, want 67 then 69", a, b)
	}
}

func TestAGraceNoteIsSlurredIntoItsNote(t *testing.T) {
	// A slur of no length at the same moment is the grace note's own: the
	// curve leaves the little note and closes on the one it decorates.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 72},
	},
		graceBefore("0", [3]string{"-1/4", "71", "1/4"}),
		slurOver("0", "0"))

	lean := read(t, raw)[0].Parts[0].
		Measures[0].Voices[0].Elements[0].(*model.Note)
	if lean.Grace == nil || len(lean.Grace.Notes) != 1 {
		t.Fatalf("the note carries %v", lean.Grace)
	}
	if lean.Grace.Notes[0].SlurStart == 0 {
		t.Error("the curve does not leave the little note")
	}
	if lean.SlurStop == 0 {
		t.Error("the curve does not close on the note it leans into")
	}
}

func TestGraceNotesDoNotCrowdOutAPhrase(t *testing.T) {
	// gscore writes one slur to a note. Where a phrase already closes on
	// the note the grace notes lean into, the phrase is the longer
	// statement and keeps the mark.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "2", pitch: 60},
		{at: "2", dur: "2", pitch: 67},
	},
		graceBefore("2", [3]string{"-1/4", "65", "1/4"}),
		slurOver("2", "0"),
		slurOver("0", "2"))

	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	first := els[0].(*model.Note)
	lean := els[1].(*model.Note)
	if first.SlurStart == 0 {
		t.Error("the phrase does not open on the first note")
	}
	if lean.SlurStop == 0 {
		t.Error("the phrase does not close on the second")
	}
	if lean.Grace == nil {
		t.Error("the grace note was lost to the phrase")
	}
}

func TestTwoCurvesCanEndOnOneNote(t *testing.T) {
	// A phrase closing on the note an appoggiatura leans into is a
	// cadence, and the commonest one in keyboard music of the period. Both
	// curves are drawn: the little note's opened last and so closes first,
	// which is what the nesting rule says and what the eye reads.
	//
	// It used to be one or the other. gscore held a slur as two flags on
	// an element, so a note could be the end of one curve and no more, and
	// the grace note's was the one dropped.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "2", pitch: 60},
		{at: "2", dur: "2", pitch: 67},
	},
		graceBefore("2", [3]string{"-1/4", "65", "1/4"}),
		slurOver("2", "0"), // the grace note's own
		slurOver("0", "2")) // and a phrase ending on the same note

	scores, err := Import(bytes.NewReader(raw))
	if len(scores) == 0 {
		t.Fatal("the music was thrown away")
	}
	if pe, ok := err.(*PartialError); ok {
		for _, line := range pe.Details() {
			if strings.Contains(line, "phrase already ends") {
				t.Errorf("a curve was still dropped: %s", line)
			}
		}
	}
	els := scores[0].Parts[0].Measures[0].Voices[0].Elements
	first := els[0].(*model.Note)
	lean := els[1].(*model.Note)
	if first.SlurStart != 1 {
		t.Errorf("the phrase opens %d times on the first note, want once",
			first.SlurStart)
	}
	if lean.Grace == nil || len(lean.Grace.Notes) != 1 {
		t.Fatalf("the grace note was lost: %v", lean.Grace)
	}
	if lean.Grace.Notes[0].SlurStart == 0 {
		t.Error("the little note opens no curve")
	}
	// Two curves end here: the phrase and the grace note's.
	if lean.SlurStop != 2 {
		t.Errorf("%d curves close on the note, want 2 — the phrase and "+
			"the appoggiatura", lean.SlurStop)
	}
}

/* ---------- everything written over the notes ---------- */

// fourNotes is a bar of four crotchets to hang marks on.
func fourNotes() []played {
	return []played{
		{at: "0", dur: "1", pitch: 60},
		{at: "1", dur: "1", pitch: 62},
		{at: "2", dur: "1", pitch: 64},
		{at: "3", dur: "1", pitch: 65},
	}
}

// builtWithText is a bar with a word written over its first note.
func builtWithText(t *testing.T, words string) []byte {
	t.Helper()
	return built(t, [2]string{"4", "4"}, "C", fourNotes(), textAt("0", words))
}

// builtWithVolta is three bars, the last two of them a first and second
// time bar.
func builtWithVolta(t *testing.T) []byte {
	t.Helper()
	return built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
		{at: "8", dur: "4", pitch: 64},
	}, voltaAt("4", "8", "4", "4"))
}

func TestATrillIsHeldOverTheNotesItCovers(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", fourNotes(),
		eventOn("voice", "TrillEventDefinition",
			[2]string{"position", "1"}, [2]string{"duration", "2"}))

	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	if tr := marksOf(els[1]).Trill; tr == nil || !tr.Start {
		t.Errorf("the trill does not open on the second note: %v", tr)
	}
	if tr := marksOf(els[3]).Trill; tr == nil || !tr.Stop {
		t.Errorf("the trill does not close on the fourth: %v", tr)
	}
	if marksOf(els[0]).Trill != nil {
		t.Error("the trill reaches a note before it begins")
	}
}

func TestATrillOnOneNoteCarriesBothEnds(t *testing.T) {
	// "tr" over a single note is a trill on that note, and the difference
	// from one held four bars is the whole instruction.
	raw := built(t, [2]string{"4", "4"}, "C", fourNotes(),
		eventOn("voice", "TrillEventDefinition",
			[2]string{"position", "0"}, [2]string{"duration", "0"}))

	tr := marksOf(read(t, raw)[0].Parts[0].
		Measures[0].Voices[0].Elements[0]).Trill
	if tr == nil || !tr.Start || !tr.Stop {
		t.Errorf("a trill on one note came out as %v", tr)
	}
}

func TestAFermataIsHeldByEverybody(t *testing.T) {
	// It is on the stream the whole system shares, because the orchestra
	// holds together.
	raw := built(t, [2]string{"4", "4"}, "C", fourNotes(),
		eventWith("global", "PauseEventDefinition",
			[][2]string{{"position", "2"}, {"duration", "0"}},
			"pauseData", [2]string{"pauseType", "kFermata"}))

	p := read(t, raw)[0].Parts[0]
	held := false
	for _, a := range marksOf(p.Measures[0].Voices[0].Elements[2]).Articulations {
		if a == model.Fermata {
			held = true
		}
	}
	if !held {
		t.Error("the third note carries no fermata")
	}
}

func TestARehearsalMarkCountsAlongWithTheOthers(t *testing.T) {
	// Stored as a place in the sequence rather than as the letter, so
	// that putting one in before it renumbers the rest instead of leaving
	// the page reading A, B, D, C.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
	}, eventOn("global", "RehearsalMarkEventDefinition",
		[2]string{"position", "4"}, [2]string{"duration", "0"}))

	p := read(t, raw)[0].Parts[0]
	if !p.Measures[1].MarkerAuto {
		t.Error("the second bar carries no rehearsal mark")
	}
	if p.Measures[0].MarkerAuto {
		t.Error("the first bar carries one it should not")
	}
}

func TestTheWordsOverTheMusicAreKept(t *testing.T) {
	// A Dorico text is a document — paragraphs of fragments, each with
	// its own styling — because somebody may have written a sentence in
	// three fonts. What gscore keeps is the words.
	raw := builtWithText(t, "dolce")
	got := marksOf(read(t, raw)[0].Parts[0].
		Measures[0].Voices[0].Elements[0]).Texts
	if len(got) != 1 || got[0].Text != "dolce" {
		t.Errorf("the note carries %v, want dolce", got)
	}
}

func TestPizzAndArcoAreWrittenAsWords(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", fourNotes(),
		eventOn("voice", "PlayingTechniqueEventDefinition",
			[2]string{"position", "0"}, [2]string{"duration", "0"},
			[2]string{"appearanceEntityID", "pta.pizzText"}),
		eventOn("voice", "PlayingTechniqueEventDefinition",
			[2]string{"position", "2"}, [2]string{"duration", "0"},
			[2]string{"appearanceEntityID", "pta.arcoText"}))

	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	if got := marksOf(els[0]).Texts; len(got) != 1 || got[0].Text != "pizz." {
		t.Errorf("the first note carries %v, want pizz.", got)
	}
	if got := marksOf(els[2]).Texts; len(got) != 1 || got[0].Text != "arco" {
		t.Errorf("the third note carries %v, want arco", got)
	}
}

func TestAnOctaveLineIsWrittenWhereItIsPrinted(t *testing.T) {
	// An 8va means "these are written an octave below what you play".
	// Dorico keeps the pitch that sounds, so under the line its notes are
	// already up there; gscore keeps the pitch that is written and raises
	// it at playback, as a reader does.
	//
	// Written at sounding pitch with a bracket over it, the page would say
	// to play it an octave higher still — right to the ear only because
	// nothing read the bracket, and wrong to anybody holding the part.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 84}, // C6, as Dorico stores it
		{at: "1", dur: "1", pitch: 86},
		{at: "2", dur: "1", pitch: 88},
		{at: "3", dur: "1", pitch: 89},
	}, eventOn("voice", "OttavaLineEventDefinition",
		[2]string{"position", "0"}, [2]string{"duration", "3"},
		[2]string{"entityID", "line.ottava.up.8"}))

	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	first := els[0].(*model.Note)
	if got := first.Pitch.MIDI(); got != 72 {
		t.Errorf("the first note under the line is written %d, want 72 — "+
			"an octave below the 84 that sounds", got)
	}
	o := marksOf(els[0]).Octave
	if o == nil || !o.Start || o.Amount != 1 {
		t.Fatalf("the line does not open on the first note: %v", o)
	}
	if o.SoundsAsWritten {
		t.Error("the line says its notes are already at the sounding " +
			"pitch, and they have just been written an octave down")
	}
	if end := marksOf(els[3]).Octave; end == nil || !end.Stop {
		t.Errorf("the line does not close on the fourth note: %v", end)
	}
	// And a note outside the line is left alone.
	raw2 := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 84},
		{at: "3", dur: "1", pitch: 84},
	}, eventOn("voice", "OttavaLineEventDefinition",
		[2]string{"position", "0"}, [2]string{"duration", "1"},
		[2]string{"entityID", "line.ottava.up.8"}))
	out := read(t, raw2)[0].Parts[0].Measures[0].Voices[0].Elements
	under := out[0].(*model.Note)
	clear := out[len(out)-1].(*model.Note)
	if under.Pitch.MIDI() != 72 {
		t.Errorf("the note under the line is %d, want 72", under.Pitch.MIDI())
	}
	if clear.Pitch.MIDI() != 84 {
		t.Errorf("the note past the end of the line is %d, want the 84 it "+
			"was written as", clear.Pitch.MIDI())
	}
}

func TestAnOctaveLineDownRaisesWhatIsUnderIt(t *testing.T) {
	// 8vb the other way: the notes are written an octave above what they
	// sound.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 36}, // C2
	}, eventOn("voice", "OttavaLineEventDefinition",
		[2]string{"position", "0"}, [2]string{"duration", "0"},
		[2]string{"entityID", "line.ottava.down.8"}))

	el := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements[0]
	n := el.(*model.Note)
	if got := n.Pitch.MIDI(); got != 48 {
		t.Errorf("the note under an 8vb is written %d, want 48 — an "+
			"octave above the 36 that sounds", got)
	}
	if o := marksOf(el).Octave; o == nil || o.Amount != -1 {
		t.Errorf("the line is %v, want an octave down", o)
	}
}

func TestFineIsASignAndNotAWord(t *testing.T) {
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
	}, eventOn("global", "RepeatMarkerEventDefinition",
		[2]string{"position", "4"}, [2]string{"duration", "0"},
		[2]string{"sectionType", "kFine"}))

	if got := read(t, raw)[0].Parts[0].Measures[1].Sign; got != model.Fine {
		t.Errorf("the second bar carries %v, want Fine", got)
	}
}

func TestABarRepeatBelongsToOneStaff(t *testing.T) {
	// A drummer repeating a bar while the horns play something new is the
	// ordinary case, and in fact most of what the sign is for.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
	}, eventOn("stave", "GenerativeBarRepeatEventDefinition",
		[2]string{"position", "4"}, [2]string{"duration", "4"},
		[2]string{"numberOfBarsToRepeat", "k1"}))

	p := read(t, raw)[0].Parts[0]
	if p.Measures[1].Repeat != 1 {
		t.Errorf("the second bar repeats %d bars, want 1", p.Measures[1].Repeat)
	}
	if p.Measures[0].Repeat != 0 {
		t.Error("the first bar carries a repeat sign it should not")
	}
}

func TestARegionThatOnlyCountsItsBarsIsNotARepeatSign(t *testing.T) {
	// Dorico marks a passage of bars that are alike with a region of this
	// kind whichever way it is to be printed, and numberBarsOnly is the way
	// that prints no sign at all: the music stays written out, bar by bar.
	//
	// Beethoven's horns hold one note from bar 62 to bar 74 under a region
	// like this one, written in Dorico's own engraving of that project as
	// thirteen tied half notes. Read as a repeat, gscore drew a % over
	// every one of them — and over a passage beginning after eight bars of
	// silence, where the sign says to play the silence again, and where
	// gscore duly played it.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "4", dur: "4", pitch: 62},
	}, eventOn("stave", "GenerativeBarRepeatEventDefinition",
		[2]string{"position", "4"}, [2]string{"duration", "4"},
		[2]string{"numberOfBarsToRepeat", "k1"},
		[2]string{"numberBarsOnly", "true"}))

	scores, err := Import(bytes.NewReader(raw))
	if len(scores) == 0 {
		t.Fatal("the music was thrown away over a counted passage")
	}
	p := scores[0].Parts[0]
	for i, m := range p.Measures {
		if m.Repeat != 0 {
			t.Errorf("bar %d carries a repeat sign for %d bars, and the "+
				"region only counts them", i+1, m.Repeat)
		}
	}
	// And the note that was under it is still there and still a note.
	els := p.Measures[1].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("the second bar holds %d things", len(els))
	}
	if _, ok := els[0].(*model.Note); !ok {
		t.Errorf("the second bar holds a %T where the note was", els[0])
	}
	// Said rather than passed over in silence: somebody comparing the two
	// pages finds the counting missing and has to be told why.
	pe, ok := err.(*PartialError)
	if !ok {
		t.Fatalf("a counted passage reported %v", err)
	}
	pe.Partial()
	if !strings.Contains(strings.Join(pe.Details(), " "), "counted bars") {
		t.Errorf("the report says %v, and does not mention the counting",
			pe.Details())
	}
}

func TestAVoltaIsReadAsFirstAndSecondTimeBars(t *testing.T) {
	// Dorico keeps the lengths of each time through in the event's
	// property table, one entry a segment.
	raw := builtWithVolta(t)
	p := read(t, raw)[0].Parts[0]
	first, second := p.Measures[1].Ending, p.Measures[2].Ending
	if first == nil || len(first.Numbers) == 0 || first.Numbers[0] != 1 {
		t.Fatalf("the second bar is not the first time bar: %v", first)
	}
	if second == nil || len(second.Numbers) == 0 || second.Numbers[0] != 2 {
		t.Fatalf("the third bar is not the second time bar: %v", second)
	}
	if !second.Open {
		t.Error("the last time through still has a hook to go back for")
	}
}

/* ---------- the silence between the notes ---------- */

func TestAVoiceIsOnlyOnTheStaffWhereItPlays(t *testing.T) {
	// A voice in Dorico lasts the whole flow once it exists, and the bars
	// where it says nothing are not written down anywhere — Dorico simply
	// does not draw it there. gscore wants every voice to add up to its
	// bar, so the silence has to be worked out, and printed it is a line of
	// rests bar after bar under music somebody else is playing. The left
	// hand of Clair de lune has five such voices and came out with four bar
	// rests stacked under every chord.
	//
	// Two notes at once in the first bar and one in the second: two voices
	// where there are two, and one where there is one.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "0", dur: "2", pitch: 64},
		{at: "4", dur: "4", pitch: 67},
	})
	p := read(t, raw)[0].Parts[0]
	if n := len(p.Measures[0].Voices); n != 2 {
		t.Errorf("the first bar has %d voices, and two notes sound at once", n)
	}
	if n := len(p.Measures[1].Voices); n != 1 {
		t.Errorf("the second bar has %d voices, and only one of them plays "+
			"there", n)
	}
}

func TestAVoiceAloneInTheBarKeepsItsRests(t *testing.T) {
	// The other half of the rule, and Dorico's own page again: Clair de
	// lune opens on a rest — the right hand comes in a quarter late over a
	// bar nobody else is playing — and the rest is printed. A staff that is
	// doing nothing else has to account for its whole bar.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "1", dur: "3", pitch: 60},
	})
	els := read(t, raw)[0].Parts[0].Measures[0].Voices[0].Elements
	if len(els) == 0 {
		t.Fatal("the bar came out empty")
	}
	r, ok := els[0].(*model.Rest)
	if !ok {
		t.Fatalf("the bar opens with a %T, and the note comes in a beat "+
			"late", els[0])
	}
	if r.Hidden {
		t.Error("the rest the bar opens with is not printed")
	}
}

func TestTheSilenceAroundASharedVoiceIsNotPrinted(t *testing.T) {
	// And where the staff is doing something else, that silence is
	// spacing rather than notation: rests for every voice at once are a
	// thicket, in lengths nothing chose.
	raw := built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "4", pitch: 60},
		{at: "0", dur: "1", pitch: 64},
	})
	vs := read(t, raw)[0].Parts[0].Measures[0].Voices
	if len(vs) != 2 {
		t.Fatalf("%d voices, want the two", len(vs))
	}
	// The short one is padded out to the bar, and none of that is drawn.
	var short *model.Voice
	for _, v := range vs {
		if len(v.Elements) > 1 {
			short = v
		}
	}
	if short == nil {
		t.Fatal("neither voice was padded out to the bar")
	}
	for i, el := range short.Elements {
		r, ok := el.(*model.Rest)
		if !ok {
			continue
		}
		if !r.Hidden {
			t.Errorf("element %d is a printed rest beside another voice's "+
				"notes", i+1)
		}
	}
}

func TestACompoundBarIsWrittenAlongItsBeats(t *testing.T) {
	// Length alone is not enough to write a length down. Nine eighths is
	// nine eighths however it is counted, and spelled by size it comes out
	// as a semibreve tied to a quaver: the right number of beats and the
	// wrong notes, written across the beats instead of along them. The left
	// hand of Clair de lune holds one chord for a bar at a time, and every
	// one of them was a whole note in a piece that has no whole notes in
	// it.
	beat := model.Duration{Num: 3, Den: 8}
	whole := model.Duration{Num: 9, Den: 8}
	got := spellAt(whole, model.Duration{Num: 0, Den: 1}, beat)
	want := []model.Duration{{Num: 3, Den: 4}, {Num: 3, Den: 8}}
	if len(got) != len(want) {
		t.Fatalf("a bar of 9/8 came out in %d pieces: %v", len(got), got)
	}
	for i := range want {
		if got[i].Reduced() != want[i].Reduced() {
			t.Errorf("piece %d is %v, want %v", i+1, got[i], want[i])
		}
	}
}

func TestANoteOffTheBeatFillsOutItsBeatFirst(t *testing.T) {
	// Starting an eighth into the bar and running to the end of it: the
	// rest of that beat, and then whole beats. A crotchet tied to a dotted
	// minim, rather than one semibreve laid across all three beats.
	beat := model.Duration{Num: 3, Den: 8}
	got := spellAt(model.Duration{Num: 1, Den: 1},
		model.Duration{Num: 1, Den: 8}, beat)
	want := []model.Duration{{Num: 1, Den: 4}, {Num: 3, Den: 4}}
	if len(got) != len(want) {
		t.Fatalf("it came out in %d pieces: %v", len(got), got)
	}
	for i := range want {
		if got[i].Reduced() != want[i].Reduced() {
			t.Errorf("piece %d is %v, want %v", i+1, got[i], want[i])
		}
	}
}

func TestSimpleTimeIsSpelledBySizeAsBefore(t *testing.T) {
	// Four-four has no beat of its own to count in — the beat is the
	// written value — so nothing changes there.
	d := model.Duration{Num: 5, Den: 8}
	got := spellAt(d, model.Duration{Num: 0, Den: 1}, beatOf(
		model.TimeSignature{Beats: 4, BeatValue: 4}))
	want := spell(d)
	if len(got) != len(want) {
		t.Fatalf("five eighths came out in %d pieces, want %d: %v", len(got),
			len(want), got)
	}
	for i := range want {
		if got[i].Reduced() != want[i].Reduced() {
			t.Errorf("piece %d is %v, want %v", i+1, got[i], want[i])
		}
	}
}

/* ---------- a player who doubles ---------- */

// builtWithDoubling is one player holding two instruments, the way somebody
// who changes instrument between movements is held.
//
// Dorico puts every instrument a player might pick up into every flow and
// says nowhere which one they are on: a flow that uses the first looks
// exactly like a flow that uses the second, apart from where the notes are.
// So the second one here is given a bar's rest or a note, by the argument,
// and nothing else about the two differs.
func builtWithDoubling(t *testing.T, plays string) []byte {
	t.Helper()
	return builtPlayer(t, plays, false)
}

// builtPlayer is that fixture, and the same one with both staves under a
// single instrument instead — which is a piano and not a doubling. Which of
// the two has notes is "first", "both" or "neither".
func builtPlayer(t *testing.T, plays string, grand bool) []byte {
	t.Helper()
	e := newEnc()

	note := func(id, pitch string) int32 {
		return e.node("block",
			e.leaf("blockID", id),
			e.leaf("parentEventStreamType", "kVoiceStream"),
			e.node("events",
				e.node("NoteEventDefinition",
					e.leaf("position", "0"),
					e.leaf("duration", "12"),
					e.leaf("pitch", pitch))))
	}
	// A bar's rest is still an event, and a staff of them is still an empty
	// staff: what says the instrument was picked up is a note.
	silent := func(id string) int32 {
		return e.node("block",
			e.leaf("blockID", id),
			e.leaf("parentEventStreamType", "kVoiceStream"),
			e.node("events",
				e.node("RestEventDefinition",
					e.leaf("position", "0"),
					e.leaf("duration", "12"))))
	}
	timeSig := e.node("BarDivisionElementDefinition",
		e.leaf("elementID", "1"),
		e.node("barDivisionData",
			e.node("timeSignature",
				e.node("timeSignaturesAndDivisions",
					e.node("timeSignatureAndDivision",
						e.node("timeSignature",
							e.leaf("numerator", "4"),
							e.leaf("denominator", "4")))))))
	globalBlock := e.node("block",
		e.leaf("blockID", "20"),
		e.leaf("parentEventStreamType", "kGlobalTimebaseStream"),
		e.node("events",
			e.node("BarDivisionEventDefinition",
				e.leaf("position", "0"),
				e.leaf("duration", "0"),
				e.leaf("barDivisionElementID", "1"))))

	held := func(inst, stave, staveStream, voiceStream string) int32 {
		return e.node("flowPlayerInstrument",
			e.leaf("instrumentID", inst),
			e.list("staveStreamInfos",
				e.node("staveStreamInfo",
					e.leaf("staveStreamID", staveStream),
					e.leaf("instrumentID", inst),
					e.leaf("staveID", stave))),
			e.list("voiceStreamInfos",
				e.node("voiceStreamInfo",
					e.leaf("voiceStreamID", voiceStream),
					e.leaf("instrumentID", inst),
					e.leaf("defaultLogicalStaveID", stave))))
	}
	stream := func(id, instance, kind string) int32 {
		return e.node("eventStream",
			e.leaf("streamID", id),
			e.leaf("blockInstanceIDs", instance),
			e.leaf("eventStreamType", kind))
	}
	instance := func(id, block string) int32 {
		return e.node("blockInstance",
			e.leaf("blockInstanceID", id),
			e.leaf("blockID", block),
			e.leaf("position", "0"))
	}
	first, second := note("30", "60"), silent("31")
	switch plays {
	case "both":
		second = note("31", "67")
	case "neither":
		first = silent("30")
	}

	// One instrument on two staves is a grand staff; two instruments on one
	// staff each is a player who doubles. The difference is the nesting,
	// and it is the whole of what braceStaves goes by.
	var insts []int32
	if grand {
		insts = []int32{e.node("flowPlayerInstrument",
			e.leaf("instrumentID", "0"),
			e.list("staveStreamInfos",
				e.node("staveStreamInfo",
					e.leaf("staveStreamID", "10"),
					e.leaf("instrumentID", "0"),
					e.leaf("staveID", "0")),
				e.node("staveStreamInfo",
					e.leaf("staveStreamID", "11"),
					e.leaf("instrumentID", "0"),
					e.leaf("staveID", "1"))),
			e.list("voiceStreamInfos",
				e.node("voiceStreamInfo",
					e.leaf("voiceStreamID", "1"),
					e.leaf("instrumentID", "0"),
					e.leaf("defaultLogicalStaveID", "0")),
				e.node("voiceStreamInfo",
					e.leaf("voiceStreamID", "2"),
					e.leaf("instrumentID", "0"),
					e.leaf("defaultLogicalStaveID", "1"))))}
	} else {
		insts = []int32{held("0", "0", "10", "1"), held("1", "1", "11", "2")}
	}
	name := "Horn (E Flat) 1 & Horn (C) 1"
	if grand {
		name = "Piano"
	}

	flow := e.node("flow",
		e.leaf("flowID", "0"),
		e.leaf("flowName", "A movement"),
		e.list("flowPlayers",
			e.node("flowPlayer",
				e.leaf("playerID", "0"),
				e.list("flowPlayerInstruments", insts...))),
		e.list("elementTables",
			e.node("BarDivisionElementTableDefinition",
				e.node("array", timeSig))),
		e.list("blocks", first, second, globalBlock),
		e.list("blockInstances",
			instance("100", "30"), instance("101", "31"),
			instance("200", "20")),
		e.list("eventStreams",
			stream("1", "100", "kVoiceStream"),
			stream("2", "101", "kVoiceStream"),
			stream("0", "200", "kGlobalTimebaseStream")))

	root := e.node("kScore",
		e.leaf("fileVersion", "1.1417"),
		e.list("scorePlayers",
			e.node("scorePlayer",
				e.leaf("playerID", "0"),
				e.leaf("baseName", name))),
		e.list("flows", flow))

	return project(e.bytes(root), "")
}

// builtGrandStaff is one instrument written on two staves, with music on
// both: a piano.
func builtGrandStaff(t *testing.T) []byte {
	t.Helper()
	return builtPlayer(t, "both", true)
}

func TestTheStavesOfOneInstrumentAreBraced(t *testing.T) {
	// A piano is one instrument on two staves and has to read as one. Read
	// without the brace, Clair de lune came out as two staves one above the
	// other, each labelled "Piano", with nothing to say they were the same
	// pair of hands.
	s := read(t, builtGrandStaff(t))[0]
	if len(s.Parts) != 2 {
		t.Fatalf("%d staves, want the two the instrument is written on",
			len(s.Parts))
	}
	if len(s.Groups) != 1 {
		t.Fatalf("%d groups of staves, want the one: %+v", len(s.Groups), s.Groups)
	}
	g := s.Groups[0]
	if g.First != 0 || g.Last != 1 {
		t.Errorf("the group covers staves %d to %d", g.First, g.Last)
	}
	if g.Symbol != model.GroupBrace {
		t.Errorf("the staves are joined with a %v, and one instrument on two "+
			"staves takes a brace", g.Symbol)
	}
	if !g.Barlines {
		t.Error("the barlines stop at each staff: the two hands keep time together")
	}
	if g.Name != "Piano" {
		t.Errorf("the group is called %q", g.Name)
	}
}

func TestTwoInstrumentsOnOneStaffEachAreNotBraced(t *testing.T) {
	// The other nesting: a player holding two instruments is not a grand
	// staff, whatever it looks like from the outside.
	s := read(t, builtPlayer(t, "both", false))[0]
	if len(s.Groups) != 0 {
		t.Errorf("the two staves of a doubling player were braced: %+v",
			s.Groups)
	}
}

func TestAnInstrumentNobodyPicksUpIsNotAStaff(t *testing.T) {
	// Beethoven's first horn holds a crook in E flat and a crook in C,
	// because it changes between movements, and every movement carries
	// both. Read as they stand, each movement has two horn staves with one
	// name on both and nothing on one of them from end to end; Dorico's own
	// engraving of that project has one.
	scores, err := Import(bytes.NewReader(builtWithDoubling(t, "first")))
	if len(scores) == 0 {
		t.Fatal("the movement was thrown away over a doubling")
	}
	s := scores[0]
	if len(s.Parts) != 1 {
		var names []string
		for _, p := range s.Parts {
			names = append(names, p.Name)
		}
		t.Fatalf("the movement has %d staves for one player: %v",
			len(s.Parts), names)
	}
	// And the one kept is the one with the notes on it.
	els := s.Parts[0].Measures[0].Voices[0].Elements
	if len(els) == 0 {
		t.Fatal("the staff that was kept is the empty one")
	}
	if _, ok := els[0].(*model.Note); !ok {
		t.Errorf("the staff that was kept holds a %T, not the note", els[0])
	}
	// Said rather than passed over: a staff that is in the file and not on
	// the page is worth a line.
	pe, ok := err.(*PartialError)
	if !ok {
		t.Fatalf("leaving a staff out reported %v", err)
	}
	pe.Partial()
	if !strings.Contains(strings.Join(pe.Details(), " "), "doubles on") {
		t.Errorf("the report says %v, and does not mention the doubling",
			pe.Details())
	}
}

func TestAStaffIsNamedForTheInstrumentAndNotTheDoubling(t *testing.T) {
	// The player is named for everything they are holding — "Horn (E Flat)
	// 1 & Horn (C) 1" — and a staff is one of those, not the list. Put on
	// the staff, the whole list reads as two players on one line.
	s := read(t, builtWithDoubling(t, "first"))[0]
	if got := s.Parts[0].Name; got != "Horn (E Flat) 1" {
		t.Errorf("the staff is called %q, want the instrument it is", got)
	}
}

func TestAnInstrumentChangedToWithinTheMovementKeepsItsStaff(t *testing.T) {
	// The Scherzo of the same symphony really does use both: the horns
	// play 211 notes on the E flat crook and four on the C, changing for
	// the run into the finale. Both staves have to stay, and each has to
	// say which crook it is.
	s := read(t, builtWithDoubling(t, "both"))[0]
	if len(s.Parts) != 2 {
		t.Fatalf("%d staves, and the player plays both instruments here",
			len(s.Parts))
	}
	if s.Parts[0].Name == s.Parts[1].Name {
		t.Errorf("both staves are called %q", s.Parts[0].Name)
	}
	if s.Parts[0].Name != "Horn (E Flat) 1" || s.Parts[1].Name != "Horn (C) 1" {
		t.Errorf("the staves are called %q and %q",
			s.Parts[0].Name, s.Parts[1].Name)
	}
}

func TestAPlayerSilentThroughoutKeepsOneStaff(t *testing.T) {
	// Tacet is not the same as not in the movement. A player who says
	// nothing from end to end is still in the score — the trombones of
	// this symphony say nothing until the finale — so they keep a staff,
	// and a doubling player keeps one rather than one each.
	s := read(t, builtWithDoubling(t, "neither"))[0]
	if len(s.Parts) != 1 {
		t.Fatalf("a player who says nothing all movement came out as %d "+
			"staves, want the one", len(s.Parts))
	}
	// The first of the instruments they hold, since nothing says otherwise.
	if got := s.Parts[0].Name; got != "Horn (E Flat) 1" {
		t.Errorf("the staff that was kept is called %q", got)
	}
}

/* ---------- the cues ---------- */

// builtTwoStaves is a flow of two players, the second of whom has a cue
// taken from the first.
//
// Written out rather than built on top of the one-staff fixture, because
// what is being tested is the join between two staves: which one the cue
// is written in and which one it takes from.
func builtTwoStaves(t *testing.T, at, dur, from string) []byte {
	t.Helper()
	e := newEnc()

	// A bar of music for each of them.
	voice := func(id, pitch string) int32 {
		return e.node("block",
			e.leaf("blockID", id),
			e.leaf("parentEventStreamType", "kVoiceStream"),
			e.node("events",
				e.node("NoteEventDefinition",
					e.leaf("position", "0"),
					e.leaf("duration", "12"),
					e.leaf("pitch", pitch))))
	}
	timeSig := e.node("BarDivisionElementDefinition",
		e.leaf("elementID", "1"),
		e.node("barDivisionData",
			e.node("timeSignature",
				e.node("timeSignaturesAndDivisions",
					e.node("timeSignatureAndDivision",
						e.node("timeSignature",
							e.leaf("numerator", "4"),
							e.leaf("denominator", "4")))))))
	globalBlock := e.node("block",
		e.leaf("blockID", "20"),
		e.leaf("parentEventStreamType", "kGlobalTimebaseStream"),
		e.node("events",
			e.node("BarDivisionEventDefinition",
				e.leaf("position", "0"),
				e.leaf("duration", "0"),
				e.leaf("barDivisionElementID", "1"))))

	// The cue, on the second staff's own stream.
	staveBlock := func(id string, cue bool) int32 {
		var events []int32
		if cue {
			events = append(events, e.node("CueEventDefinition",
				e.leaf("position", at),
				e.leaf("duration", dur),
				e.node("propertyTable",
					e.node("entries",
						e.node("entry",
							e.leaf("propertyType", "kCuedStave"),
							e.leaf("value", "int: "+from))))))
		}
		return e.node("block",
			e.leaf("blockID", id),
			e.leaf("parentEventStreamType", "kInstrumentStaveStream"),
			e.node("events", events...))
	}

	player := func(pid, inst, stave, staveStream, voiceStream string) int32 {
		return e.node("flowPlayer",
			e.leaf("playerID", pid),
			e.list("flowPlayerInstruments",
				e.node("flowPlayerInstrument",
					e.leaf("instrumentID", inst),
					e.list("staveStreamInfos",
						e.node("staveStreamInfo",
							e.leaf("staveStreamID", staveStream),
							e.leaf("instrumentID", inst),
							e.leaf("staveID", stave))),
					e.list("voiceStreamInfos",
						e.node("voiceStreamInfo",
							e.leaf("voiceStreamID", voiceStream),
							e.leaf("instrumentID", inst),
							e.leaf("defaultLogicalStaveID", stave))))))
	}

	stream := func(id, instance, kind string) int32 {
		return e.node("eventStream",
			e.leaf("streamID", id),
			e.leaf("blockInstanceIDs", instance),
			e.leaf("eventStreamType", kind))
	}
	instance := func(id, block string) int32 {
		return e.node("blockInstance",
			e.leaf("blockInstanceID", id),
			e.leaf("blockID", block),
			e.leaf("position", "0"))
	}

	flow := e.node("flow",
		e.leaf("flowID", "0"),
		e.leaf("flowName", "A movement"),
		e.list("flowPlayers",
			player("0", "0", "0", "10", "1"),
			player("1", "1", "1", "11", "2")),
		e.list("elementTables",
			e.node("BarDivisionElementTableDefinition",
				e.node("array", timeSig))),
		e.list("blocks",
			voice("30", "60"), voice("31", "67"),
			globalBlock, staveBlock("40", false), staveBlock("41", true)),
		e.list("blockInstances",
			instance("100", "30"), instance("101", "31"),
			instance("200", "20"),
			instance("300", "40"), instance("301", "41")),
		e.list("eventStreams",
			stream("1", "100", "kVoiceStream"),
			stream("2", "101", "kVoiceStream"),
			stream("0", "200", "kGlobalTimebaseStream"),
			stream("10", "300", "kInstrumentStaveStream"),
			stream("11", "301", "kInstrumentStaveStream")))

	root := e.node("kScore",
		e.leaf("fileVersion", "1.1417"),
		e.list("scorePlayers",
			e.node("scorePlayer",
				e.leaf("playerID", "0"),
				e.leaf("baseName", "Violin I")),
			e.node("scorePlayer",
				e.leaf("playerID", "1"),
				e.leaf("baseName", "Flute 1"))),
		e.list("flows", flow))

	return project(e.bytes(root), "")
}

// builtWithCue is two staves, the second cueing the first for two bars.
//
// A cue names a staff to take from and a stretch to take, and holds no
// music of its own — which is also how gscore keeps one, so nothing is
// copied in the reading.
func builtWithCue(t *testing.T, at, dur, from string) []byte {
	t.Helper()
	return builtTwoStaves(t, at, dur, from)
}

func TestACueSaysWhichBarsOfWhoseLine(t *testing.T) {
	raw := builtWithCue(t, "4", "8", "0")
	s := read(t, raw)[0]
	if len(s.Sheets) != 1 {
		t.Fatalf("%d parts carry cues, want 1", len(s.Sheets))
	}
	sh := s.Sheets[0]
	if len(sh.Cues) != 1 {
		t.Fatalf("the part carries %d cues, want 1", len(sh.Cues))
	}
	c := sh.Cues[0]
	if c.At != 1 || c.Bars != 2 {
		t.Errorf("the cue covers bars %d to %d, want the second and third",
			c.At+1, c.At+c.Bars)
	}
	if c.From != 0 {
		t.Errorf("the cue is taken from staff %d, want the first", c.From)
	}
	// And the part it belongs to is the staff that was cued, not the one
	// the notes came from.
	if len(sh.Staves) != 1 || sh.Staves[0] != 1 {
		t.Errorf("the cue was put in the part for staff %v", sh.Staves)
	}
}

func TestACueHoldsNoMusicOfItsOwn(t *testing.T) {
	// The other player's line already stands on its own staff. A cue that
	// copied it would be a copy to keep in step, and printed in the score
	// it would be the same music twice.
	raw := builtWithCue(t, "4", "8", "0")
	s := read(t, raw)[0]
	for _, p := range s.Parts {
		for i, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if mk := marksOf(el); mk != nil && mk.Small {
						t.Fatalf("%s bar %d holds cue-sized music in the "+
							"score itself", p.Name, i+1)
					}
				}
			}
		}
	}
}

func TestACueFromAStaffThatIsNotThereIsReported(t *testing.T) {
	raw := builtWithCue(t, "4", "8", "77")
	scores, err := Import(bytes.NewReader(raw))
	if len(scores) == 0 {
		t.Fatal("the music was thrown away over a cue")
	}
	if len(scores[0].Sheets) != 0 {
		t.Error("a cue was written from a staff that is not in the flow")
	}
	pe, ok := err.(*PartialError)
	if !ok {
		t.Fatalf("nothing was reported: %v", err)
	}
	found := false
	for _, line := range pe.Details() {
		if strings.Contains(line, "cue") {
			found = true
		}
	}
	if !found {
		t.Errorf("the report says %v", pe.Details())
	}
}

// builtKeyTurn is two bars of four-four, opening in C minor — three flats,
// which is what built writes for "C" — and turning to E minor at the moment
// given, counted in crotchets from the start. Two different signatures, so
// that the turn can be told from the key the piece opened in.
func builtKeyTurn(t *testing.T, at string) []byte {
	t.Helper()
	return built(t, [2]string{"4", "4"}, "C", []played{
		{at: "0", dur: "1", pitch: 60},
		{at: "1", dur: "1", pitch: 62},
		{at: "2", dur: "1", pitch: 64},
		{at: "3", dur: "1", pitch: 65},
		{at: "4", dur: "4", pitch: 67},
	}, keyTurnAt(at, "E"))
}

func TestAKeyThatTurnsInsideABarStaysThere(t *testing.T) {
	// A double bar in the middle of a measure is ordinary in opera and in
	// anything written to a cue: the scene turns, the key turns with it,
	// and the bar it lands in is left as long as it was. Hoisted to the
	// head of the bar, every note before the turn is read in the new key.
	raw := builtKeyTurn(t, "2") // two crotchets into a four-four bar
	p := read(t, raw)[0].Parts[0]
	// The first bar carries the key the piece opens in, as every first
	// bar does. What it must not carry is the turn.
	if k := p.Measures[0].Key; k != nil && k.Fifths == 1 {
		t.Error("the turn was written at the head of the bar, where the " +
			"notes before it are read against it")
	}
	els := p.Measures[0].Voices[0].Elements
	var found *model.KeySignature
	for _, el := range els {
		if mk := marksOf(el); mk != nil && mk.Key != nil {
			found = mk.Key
		}
	}
	if found == nil {
		t.Fatal("the key turn is nowhere in the bar")
	}
	if found.Fifths != 1 {
		t.Errorf("the key turned to %d, want E minor's one sharp",
			found.Fifths)
	}
	// And it holds past the barline.
	if got := p.KeyAt(1); got.Fifths != 1 {
		t.Errorf("the bar after the turn is in %d, want the one sharp it "+
			"was turned to", got.Fifths)
	}
}

func TestAKeyAtTheHeadOfABarIsTheBarsOwn(t *testing.T) {
	raw := builtKeyTurn(t, "4") // exactly at the second bar's head
	p := read(t, raw)[0].Parts[0]
	if p.Measures[1].Key == nil {
		t.Fatal("a key written at a barline is not the bar's own")
	}
	if p.Measures[1].Key.Fifths != 1 {
		t.Errorf("the second bar is in %d, want E minor's one sharp",
			p.Measures[1].Key.Fifths)
	}
}
