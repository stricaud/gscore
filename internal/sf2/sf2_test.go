/*
 * sf2_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing a soundfont, checked by playing it.
 *
 * A soundfont that is wrong is not wrong in a way reading it back with the
 * same code would show: both halves would agree about a file nothing else
 * accepts. So the test reads what is written with meltysynth — the
 * synthesiser gscore actually plays and records through, written by
 * somebody else from the specification — and then plays notes on it and
 * listens to what comes out. A file that loads, sounds, sounds at the right
 * pitch, and holds a long note on a short recording is a file that works,
 * whatever this package believes about it.
 */

package sf2

import (
	"bytes"
	"math"
	"testing"

	"github.com/sinshu/go-meltysynth/meltysynth"
)

/* ---------- making something to write ---------- */

// tone is a recording of a sine wave at hz, as a microphone would have
// given it: a whole number of cycles, so that it loops without a click.
func tone(hz float64, rate int, cycles int) []int16 {
	frames := int(math.Round(float64(cycles) * float64(rate) / hz))
	out := make([]int16, frames)
	for i := range out {
		v := math.Sin(2 * math.Pi * hz * float64(i) / float64(rate))
		out[i] = int16(v * 20000)
	}
	return out
}

// a440 is one second of the A above middle C, which is MIDI key 69.
func a440(rate int) Sample {
	data := tone(440, rate, 440)
	return Sample{
		Name: "A4", Rate: rate, Data: data, Root: 69,
		LoopStart: 0, LoopEnd: len(data),
	}
}

// oneFlute is the smallest soundfont that is a soundfont: one recording,
// one instrument playing it over the whole keyboard, one preset.
func oneFlute(rate int) *Font {
	return &Font{
		Name:    "My flute",
		By:      "Somebody with a microphone",
		Samples: []Sample{a440(rate)},
		Instruments: []Instrument{{
			Name: "Flute",
			Regions: []Region{{
				Sample: 0, KeyLo: 0, KeyHi: 127, Loop: true, Release: 0.3,
			}},
		}},
		Presets: []Preset{{Name: "Flute", Bank: 0, Program: 73,
			Zones: []PresetZone{{Instrument: 0}}}},
	}
}

// written writes a font and hands back what meltysynth makes of it.
func written(t *testing.T, f *Font) *meltysynth.SoundFont {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatalf("writing the soundfont: %v", err)
	}
	sf, err := meltysynth.NewSoundFont(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("the soundfont that was written cannot be read: %v", err)
	}
	return sf
}

/* ---------- is it a soundfont ---------- */

func TestWhatIsWrittenIsASoundfont(t *testing.T) {
	sf := written(t, oneFlute(44100))

	if got := sf.Info.BankName; got != "My flute" {
		t.Errorf("the bank is called %q, want %q", got, "My flute")
	}
	if len(sf.Presets) != 1 {
		t.Fatalf("%d presets were read, want 1", len(sf.Presets))
	}
	if got := sf.Presets[0].PatchNumber; got != 73 {
		t.Errorf("the preset is program %d, want 73 — a flute", got)
	}
	if len(sf.Instruments) != 1 {
		t.Fatalf("%d instruments were read, want 1", len(sf.Instruments))
	}
	if len(sf.SampleHeaders) != 1 {
		t.Fatalf("%d sample headers were read, want 1", len(sf.SampleHeaders))
	}
	if got := sf.SampleHeaders[0].Name; got != "A4" {
		t.Errorf("the recording is called %q, want %q", got, "A4")
	}
	if got := sf.SampleHeaders[0].OriginalPitch; got != 69 {
		t.Errorf("the recording sounds at key %d, want 69", got)
	}
	if got := sf.SampleHeaders[0].SampleRate; got != 44100 {
		t.Errorf("the recording is at %d Hz, want 44100", got)
	}
}

func TestTheRegionIsARegionAndNotASetOfDefaults(t *testing.T) {
	// The one mistake that produces a file every reader accepts and none
	// of them plays: a zone whose generators do not end with the sample is
	// read as a global zone, and the region it was meant to be is not
	// there at all.
	sf := written(t, oneFlute(44100))
	regions := sf.Instruments[0].Regions
	if len(regions) != 1 {
		t.Fatalf("%d regions were read, want 1 — the zone was taken for a "+
			"global one", len(regions))
	}
	if got := regions[0].Sample.Name; got != "A4" {
		t.Errorf("the region plays %q, want the recording", got)
	}
}

func TestTheRecordingSurvivesTheJourney(t *testing.T) {
	f := oneFlute(22050)
	sf := written(t, f)

	h := sf.SampleHeaders[0]
	frames := int(h.End - h.Start)
	if frames != len(f.Samples[0].Data) {
		t.Fatalf("%d frames came back from a recording of %d",
			frames, len(f.Samples[0].Data))
	}
	for i, want := range f.Samples[0].Data {
		if got := sf.WaveData[int(h.Start)+i]; got != want {
			t.Fatalf("frame %d came back as %d, want %d", i, got, want)
		}
	}
	// And the silence that keeps one recording out of the next is there.
	for i := 0; i < gap; i++ {
		if got := sf.WaveData[int(h.End)+i]; got != 0 {
			t.Fatalf("the gap after the recording holds %d at %d, want "+
				"silence", got, i)
		}
	}
}

func TestSeveralRecordingsEachKeepTheirPlace(t *testing.T) {
	rate := 22050
	f := oneFlute(rate)
	low := tone(220, rate, 110)
	f.Samples = append(f.Samples, Sample{
		Name: "A3", Rate: rate, Data: low, Root: 57,
		LoopStart: 0, LoopEnd: len(low),
	})
	f.Instruments[0].Regions = []Region{
		{Sample: 1, KeyLo: 0, KeyHi: 63, Loop: true},
		{Sample: 0, KeyLo: 64, KeyHi: 127, Loop: true},
	}
	sf := written(t, f)

	if len(sf.SampleHeaders) != 2 {
		t.Fatalf("%d recordings came back, want 2", len(sf.SampleHeaders))
	}
	for i, want := range f.Samples {
		h := sf.SampleHeaders[i]
		if got := int(h.End - h.Start); got != len(want.Data) {
			t.Errorf("recording %d is %d frames, want %d", i+1, got,
				len(want.Data))
		}
		if got := sf.WaveData[h.Start+5]; got != want.Data[5] {
			t.Errorf("recording %d starts in the wrong place: %d, want %d",
				i+1, got, want.Data[5])
		}
	}
	// And the regions came back in the order they were written, each
	// answering its own stretch of keyboard.
	regions := sf.Instruments[0].Regions
	if len(regions) != 2 {
		t.Fatalf("%d regions, want 2", len(regions))
	}
	if got := regions[0].GetKeyRangeEnd(); got != 63 {
		t.Errorf("the lower region ends at key %d, want 63", got)
	}
	if got := regions[1].GetKeyRangeStart(); got != 64 {
		t.Errorf("the upper region starts at key %d, want 64", got)
	}
}

/* ---------- does it sound ---------- */

// play renders one note and hands back the left channel.
func play(t *testing.T, sf *meltysynth.SoundFont, key int32, seconds float64,
	hold float64) []float32 {
	t.Helper()
	const rate = 44100
	settings := meltysynth.NewSynthesizerSettings(rate)
	// Dry, because what is being listened to is the soundfont. The
	// reverb is on by default and its tail goes on for a second or more
	// after the note, which is indistinguishable from a note that will
	// not stop.
	settings.EnableReverbAndChorus = false
	synth, err := meltysynth.NewSynthesizer(sf, settings)
	if err != nil {
		t.Fatalf("the soundfont will not play: %v", err)
	}
	frames := int(seconds * rate)
	left := make([]float32, frames)
	right := make([]float32, frames)

	synth.NoteOn(0, key, 100)
	held := int(hold * rate)
	if held <= 0 || held > frames {
		held = frames
	}
	synth.Render(left[:held], right[:held])
	if held < frames {
		synth.NoteOff(0, key)
		synth.Render(left[held:], right[held:])
	}
	return left
}

// peak is the loudest thing in a stretch of sound.
func peak(xs []float32) float64 {
	m := 0.0
	for _, v := range xs {
		if a := math.Abs(float64(v)); a > m {
			m = a
		}
	}
	return m
}

// hz guesses the pitch of a stretch of sound by counting how often it
// crosses zero going upwards, which for a sine wave is exact enough to tell
// one semitone from another.
func hz(xs []float32, rate float64) float64 {
	first, last, n := -1, -1, 0
	for i := 1; i < len(xs); i++ {
		if xs[i-1] <= 0 && xs[i] > 0 {
			if first < 0 {
				first = i
			}
			last = i
			n++
		}
	}
	if n < 2 {
		return 0
	}
	return float64(n-1) * rate / float64(last-first)
}

func TestThePlayedNoteIsTheNoteThatWasRecorded(t *testing.T) {
	sf := written(t, oneFlute(44100))
	// The key the recording was made at, so nothing is shifted: what comes
	// out should be the 440 Hz that went in.
	got := hz(play(t, sf, 69, 0.5, 0.5), 44100)
	if math.Abs(got-440) > 4 {
		t.Errorf("key 69 sounded at %.1f Hz, want 440", got)
	}
}

func TestEveryOtherKeyIsThatRecordingShifted(t *testing.T) {
	sf := written(t, oneFlute(44100))
	for _, c := range []struct {
		key  int32
		want float64
	}{
		{81, 880}, // an octave up
		{57, 220}, // an octave down
		{76, 659.25},
		{64, 329.63},
	} {
		got := hz(play(t, sf, c.key, 0.5, 0.5), 44100)
		// Within a twentieth of a semitone, which is as close as counting
		// zero crossings over half a second can say.
		if math.Abs(1200*math.Log2(got/c.want)) > 5 {
			t.Errorf("key %d sounded at %.1f Hz, want %.1f",
				c.key, got, c.want)
		}
	}
}

func TestALoopHoldsANoteLongerThanTheRecording(t *testing.T) {
	// The whole reason a sample has a loop: a two-second recording has to
	// be able to hold a long note. Here the recording is a tenth of a
	// second and the note is held for a second and a half.
	rate := 44100
	short := tone(440, rate, 44) // a tenth of a second
	f := oneFlute(rate)
	f.Samples[0] = Sample{
		Name: "A4", Rate: rate, Data: short, Root: 69,
		LoopStart: 0, LoopEnd: len(short),
	}
	sf := written(t, f)

	out := play(t, sf, 69, 1.5, 1.5)
	// Still sounding at the end, a second after the recording itself ran
	// out, and still at the right pitch.
	tail := out[len(out)-rate/10:]
	if got := peak(tail); got < 0.05 {
		t.Errorf("a second after the recording ended the note is at %.4f, "+
			"want it still sounding", got)
	}
	if got := hz(tail, float64(rate)); math.Abs(got-440) > 6 {
		t.Errorf("the looped note is at %.1f Hz by the end, want 440", got)
	}
}

func TestWithoutALoopTheNoteIsAsLongAsTheRecording(t *testing.T) {
	rate := 44100
	short := tone(440, rate, 44)
	f := oneFlute(rate)
	f.Samples[0] = Sample{Name: "A4", Rate: rate, Data: short, Root: 69}
	f.Instruments[0].Regions[0].Loop = false
	sf := written(t, f)

	out := play(t, sf, 69, 1.0, 1.0)
	if got := peak(out[:rate/20]); got < 0.05 {
		t.Fatalf("the note did not sound at all: %.4f", got)
	}
	if got := peak(out[rate/2:]); got > 0.001 {
		t.Errorf("half a second after a tenth-of-a-second recording the "+
			"sound is at %.4f, want silence", got)
	}
}

func TestAKeyOutsideTheRegionPlaysNothing(t *testing.T) {
	rate := 44100
	f := oneFlute(rate)
	f.Instruments[0].Regions[0].KeyLo = 60
	f.Instruments[0].Regions[0].KeyHi = 72
	sf := written(t, f)

	if got := peak(play(t, sf, 65, 0.3, 0.3)); got < 0.05 {
		t.Errorf("a key inside the region is at %.4f, want it sounding", got)
	}
	if got := peak(play(t, sf, 40, 0.3, 0.3)); got > 0.001 {
		t.Errorf("a key outside the region is at %.4f, want silence", got)
	}
}

func TestLettingTheKeyGoLetsTheNoteFade(t *testing.T) {
	// A looping sample with no release stops dead, which sounds like the
	// cable being pulled out. The region asks for a third of a second.
	sf := written(t, oneFlute(44100))
	out := play(t, sf, 69, 1.0, 0.4)
	const rate = 44100
	// Just after the key is let go it is still sounding.
	if got := peak(out[int(0.42*rate):int(0.45*rate)]); got < 0.02 {
		t.Errorf("just after the key went up the note is at %.4f, want it "+
			"still fading", got)
	}
	// And well after the fade it is gone.
	if got := peak(out[int(0.95*rate):]); got > 0.005 {
		t.Errorf("half a second after a 0.3s fade the note is at %.4f, "+
			"want silence", got)
	}
}

func TestOneRecordingCanBeQuieterThanAnother(t *testing.T) {
	rate := 44100
	loud := written(t, oneFlute(rate))
	f := oneFlute(rate)
	f.Instruments[0].Regions[0].Attenuation = 12 // decibels
	quiet := written(t, f)

	a := peak(play(t, loud, 69, 0.3, 0.3))
	b := peak(play(t, quiet, 69, 0.3, 0.3))
	if b >= a {
		t.Fatalf("turned down 12 dB it came out at %.4f against %.4f", b, a)
	}
	// Twelve decibels is a quarter of the amplitude, and the envelope is
	// the same either way, so the ratio should be close.
	if got := 20 * math.Log10(a/b); math.Abs(got-12) > 2 {
		t.Errorf("it came out %.1f dB down, want 12", got)
	}
}

/* ---------- what is refused ---------- */

func TestASoundfontThatCannotBeWrittenSaysWhy(t *testing.T) {
	rate := 22050
	for _, c := range []struct {
		why  string
		make func(*Font)
	}{
		{"no name", func(f *Font) { f.Name = "" }},
		{"no recordings", func(f *Font) { f.Samples = nil }},
		{"no instrument", func(f *Font) { f.Instruments = nil }},
		{"no preset", func(f *Font) { f.Presets = nil }},
		{"an empty recording", func(f *Font) { f.Samples[0].Data = nil }},
		{"no rate", func(f *Font) { f.Samples[0].Rate = 0 }},
		{"a root that is not a key", func(f *Font) { f.Samples[0].Root = 500 }},
		{"a correction of a whole tone", func(f *Font) { f.Samples[0].Cents = 200 }},
		{"a loop outside the recording", func(f *Font) {
			f.Samples[0].LoopEnd = len(f.Samples[0].Data) + 99
		}},
		{"a loop that ends before it starts", func(f *Font) {
			f.Samples[0].LoopStart, f.Samples[0].LoopEnd = 500, 100
		}},
		{"an instrument with no regions", func(f *Font) {
			f.Instruments[0].Regions = nil
		}},
		{"a region playing a recording that is not there", func(f *Font) {
			f.Instruments[0].Regions[0].Sample = 7
		}},
		{"a backwards key range", func(f *Font) {
			f.Instruments[0].Regions[0].KeyLo = 90
			f.Instruments[0].Regions[0].KeyHi = 40
		}},
		{"a region asking to be louder than it was recorded", func(f *Font) {
			f.Instruments[0].Regions[0].Attenuation = -6
		}},
		{"a preset in no bank there is", func(f *Font) { f.Presets[0].Bank = 300 }},
		{"a preset playing an instrument that is not there", func(f *Font) {
			f.Presets[0].Zones[0].Instrument = 4
		}},
	} {
		f := oneFlute(rate)
		c.make(f)
		var buf bytes.Buffer
		err := Write(&buf, f)
		if err == nil {
			t.Errorf("a soundfont with %s was written anyway", c.why)
			continue
		}
		if buf.Len() != 0 {
			t.Errorf("a soundfont with %s wrote %d bytes before refusing",
				c.why, buf.Len())
		}
	}
	if err := Write(&bytes.Buffer{}, nil); err == nil {
		t.Error("writing no soundfont at all did not say so")
	}
}

func TestALongNameIsCutAndNotRefused(t *testing.T) {
	// Refusing to write somebody's soundfont over the length of the name
	// of one recording in it would be a poor trade.
	f := oneFlute(22050)
	f.Samples[0].Name = "the A above middle C, played softly, take four"
	sf := written(t, f)
	if got := sf.SampleHeaders[0].Name; len(got) != maxName-1 {
		t.Errorf("the name came back as %q (%d characters), want it cut "+
			"to %d", got, len(got), maxName-1)
	}
}
