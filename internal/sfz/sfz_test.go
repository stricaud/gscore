/*
 * sfz_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading an SFZ instrument.
 *
 * The format is plain text and looks simple, and one thing in it is not: a
 * value may contain spaces, so an opcode's value does not end at the first
 * one. "sample=Solo Violin Sustain.wav lokey=60" is a filename of three
 * words followed by another opcode, and telling those apart is the whole
 * difficulty. Most of these tests are about that.
 */

package sfz

import (
	"io"
	"strings"
	"testing"
)

func read(t *testing.T, text string) *Instrument {
	t.Helper()
	inst, err := Parse(strings.NewReader(text), "test.sfz", nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return inst
}

func TestARegionIsReadWithWhatItInherits(t *testing.T) {
	inst := read(t, `
<global> volume=-3
<group> loop_mode=loop_continuous ampeg_release=0.4
<region> sample=a.wav lokey=c4 hikey=e4 pitch_keycenter=d4
<region> sample=b.wav lokey=f4 hikey=a4 pitch_keycenter=g4 ampeg_release=1.2
`)
	if len(inst.Regions) != 2 {
		t.Fatalf("%d regions, want 2", len(inst.Regions))
	}
	a := inst.Regions[0]
	if a.Str("sample") != "a.wav" {
		t.Errorf("the first plays %q", a.Str("sample"))
	}
	// From <global> and <group>, which a region does not repeat.
	if a.Float("volume", 0) != -3 {
		t.Errorf("volume came out %v, want -3 from <global>", a.Float("volume", 0))
	}
	if a.Str("loop_mode") != "loop_continuous" {
		t.Errorf("loop_mode came out %q, want it from <group>", a.Str("loop_mode"))
	}
	if a.Float("ampeg_release", 0) != 0.4 {
		t.Errorf("release came out %v, want 0.4 from <group>", a.Float("ampeg_release", 0))
	}
	// And a region's own beats the group's.
	if got := inst.Regions[1].Float("ampeg_release", 0); got != 1.2 {
		t.Errorf("the second region's release is %v, want its own 1.2", got)
	}
}

// A new <group> clears the one before it, and a new <global> clears
// everything: a value is not supposed to leak sideways into an instrument
// that never asked for it.
func TestAHeaderClearsTheOnesInsideIt(t *testing.T) {
	inst := read(t, `
<group> volume=-6 pan=-50
<region> sample=a.wav
<group> volume=0
<region> sample=b.wav
`)
	if got := inst.Regions[1].Str("pan"); got != "" {
		t.Errorf("the second region has pan=%q from the group before it", got)
	}
	if got := inst.Regions[1].Float("volume", 99); got != 0 {
		t.Errorf("the second region's volume is %v, want its own group's 0", got)
	}
}

func TestAValueMayHaveSpacesInIt(t *testing.T) {
	for _, c := range []struct {
		what, text, sample string
		lokey              int
	}{
		{"a plain name", `<region> sample=Flute.wav lokey=60`, "Flute.wav", 60},
		{"spaces in the name",
			`<region> sample=Solo Violin Sustain.wav lokey=60`,
			"Solo Violin Sustain.wav", 60},
		{"a folder with spaces",
			`<region> sample=Woodwinds/Flute susNV/LD A4.wav lokey=69`,
			"Woodwinds/Flute susNV/LD A4.wav", 69},
		{"nothing after it",
			`<region> lokey=60 sample=Solo Violin.wav`, "Solo Violin.wav", 60},
	} {
		t.Run(c.what, func(t *testing.T) {
			inst := read(t, c.text)
			r := inst.Regions[0]
			if got := r.Str("sample"); got != c.sample {
				t.Errorf("sample came out %q, want %q", got, c.sample)
			}
			if got := r.Note("lokey", -1); got != c.lokey {
				t.Errorf("lokey came out %d, want %d", got, c.lokey)
			}
		})
	}
}

// A region may be written on one line or on six, and SFZ means the same
// thing by both: the line break carries nothing.
func TestLineBreaksCarryNothing(t *testing.T) {
	one := read(t, `<region> sample=a.wav lokey=60 hikey=72 pitch_keycenter=64`)
	many := read(t, `
<region>
 sample=a.wav
 lokey=60
 hikey=72
 pitch_keycenter=64
`)
	if len(many.Regions) != 1 {
		t.Fatalf("%d regions across six lines, want 1", len(many.Regions))
	}
	for _, k := range []string{"sample", "lokey", "hikey", "pitch_keycenter"} {
		if one.Regions[0].Str(k) != many.Regions[0].Str(k) {
			t.Errorf("%s: %q on one line, %q on six",
				k, one.Regions[0].Str(k), many.Regions[0].Str(k))
		}
	}
}

// Two regions on one line, which a generated file does.
func TestSeveralRegionsOnOneLine(t *testing.T) {
	inst := read(t,
		`<region> sample=a.wav lokey=60 <region> sample=b.wav lokey=62`)
	if len(inst.Regions) != 2 {
		t.Fatalf("%d regions, want 2", len(inst.Regions))
	}
	if inst.Regions[0].Str("sample") != "a.wav" ||
		inst.Regions[1].Str("sample") != "b.wav" {
		t.Errorf("came out %q and %q", inst.Regions[0].Str("sample"),
			inst.Regions[1].Str("sample"))
	}
}

func TestNotesAreReadByNameOrByNumber(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"60", 60}, {"c4", 60}, {"C4", 60}, {"c#4", 61}, {"db4", 61},
		{"a4", 69}, {"c-1", 0}, {"g9", 127}, {"c0", 12},
	} {
		got, err := ParseNote(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q came out %d, want %d", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"", "h4", "x", "c", "middle"} {
		if _, err := ParseNote(bad); err == nil {
			t.Errorf("%q was read as a note", bad)
		}
	}
}

func TestCommentsAreNotValues(t *testing.T) {
	inst := read(t, `
// the flute, an octave at a time
<region> sample=a.wav lokey=60 // the low one
<region> sample=C:/Sounds//b.wav lokey=62
`)
	if got := inst.Regions[0].Str("lokey"); got != "60" {
		t.Errorf("lokey came out %q with a comment after it", got)
	}
	// A doubled separator in a path is not a comment, which is a real
	// shape in files written by hand on Windows.
	if got := inst.Regions[1].Str("sample"); got != "C:/Sounds//b.wav" {
		t.Errorf("the path came out %q", got)
	}
}

func TestControlSetsTheFolderTheSamplesAreIn(t *testing.T) {
	inst := read(t, `
<control> default_path=Samples/Flute/
<region> sample=A4.wav
<region> sample=sub\B4.wav
`)
	if got := inst.Sample(inst.Regions[0]); got != "Samples/Flute/A4.wav" {
		t.Errorf("the sample is at %q", got)
	}
	// A path written with the separator Windows uses is read everywhere.
	if got := inst.Sample(inst.Regions[1]); got != "Samples/Flute/sub/B4.wav" {
		t.Errorf("the sample is at %q", got)
	}
}

func TestDefinesAreSubstituted(t *testing.T) {
	inst := read(t, `
#define $DIR Woodwinds/Flute
#define $VEL 64
<region> sample=$DIR/A4.wav lovel=$VEL
`)
	r := inst.Regions[0]
	if got := r.Str("sample"); got != "Woodwinds/Flute/A4.wav" {
		t.Errorf("sample came out %q", got)
	}
	if got := r.Int("lovel", -1); got != 64 {
		t.Errorf("lovel came out %d", got)
	}
}

// A name that starts with another name is not eaten by it.
func TestTheLongestDefineWins(t *testing.T) {
	inst := read(t, `
#define $VEL 10
#define $VELOCITY 99
<region> sample=a.wav lovel=$VELOCITY hivel=$VEL
`)
	r := inst.Regions[0]
	if got := r.Int("lovel", -1); got != 99 {
		t.Errorf("lovel came out %d, want 99", got)
	}
	if got := r.Int("hivel", -1); got != 10 {
		t.Errorf("hivel came out %d, want 10", got)
	}
}

func TestIncludeReadsTheOtherFile(t *testing.T) {
	files := map[string]string{
		"parts/strings.sfz": "<region> sample=violin.wav lokey=60\n",
	}
	open := func(name string) (io.ReadCloser, error) {
		text, ok := files[name]
		if !ok {
			t.Errorf("asked for %q, which is not where it is", name)
			return nil, io.EOF
		}
		return io.NopCloser(strings.NewReader(text)), nil
	}
	inst, err := Parse(strings.NewReader(
		"<global> volume=-2\n#include \"parts/strings.sfz\"\n"),
		"orchestra.sfz", open)
	if err != nil {
		t.Fatal(err)
	}
	if len(inst.Regions) != 1 {
		t.Fatalf("%d regions, want the one from the included file", len(inst.Regions))
	}
	// What the including file said still holds inside it.
	if got := inst.Regions[0].Float("volume", 0); got != -2 {
		t.Errorf("volume came out %v, want -2", got)
	}
}

// An instrument that includes itself is read once and refused, rather than
// read for ever.
func TestAnIncludeLoopIsRefused(t *testing.T) {
	open := func(string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(
			"#include \"loop.sfz\"\n<region> sample=a.wav\n")), nil
	}
	_, err := Parse(strings.NewReader("#include \"loop.sfz\"\n"),
		"loop.sfz", open)
	if err == nil {
		t.Fatal("a file that includes itself was read to the end")
	}
	if !strings.Contains(err.Error(), "deep") {
		t.Errorf("it failed with %q, which does not say what happened", err)
	}
}

// Headers this package does not read are stepped over, and what is in them
// does not become part of the next region.
func TestUnknownHeadersAreNotFoldedIn(t *testing.T) {
	inst := read(t, `
<region> sample=a.wav
<curve> curve_index=7 v000=0 v127=1
<region> sample=b.wav
`)
	if len(inst.Regions) != 2 {
		t.Fatalf("%d regions, want 2", len(inst.Regions))
	}
	if inst.Regions[1].Has("curve_index") || inst.Regions[1].Has("v000") {
		t.Errorf("the second region picked up the curve: %v",
			inst.Regions[1].Opcodes)
	}
}

// A file with nothing to play is a mistake worth hearing about, rather than
// an instrument that is silent.
func TestAFileWithNoRegionsIsRefused(t *testing.T) {
	_, err := Parse(strings.NewReader("<global> volume=0\n"), "empty.sfz", nil)
	if err == nil {
		t.Fatal("a file with no regions was read as an instrument")
	}
}
