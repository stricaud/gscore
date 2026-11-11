/*
 * sf2.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package sf2 writes soundfonts: recordings of an instrument, laid out as
// the .sf2 file every synthesiser reads.
//
// gscore could already play a soundfont and could not make one. This is the
// other direction, and it is here so that somebody can record their own
// flute — one note at a time, into a microphone — and then hear their own
// score played back with it. The recording, the trimming and the asking are
// elsewhere; what is here takes samples that are already decided on and
// writes the file.
//
// Why SF2 and not SFZ, when SFZ is the pleasant one to write. Because of
// what happens next: a soundfont written here is handed to meltysynth,
// which gscore already plays and records through, and which reads SF2 and
// nothing else. An SFZ instrument would need a second synthesiser, or a
// renderer feeding the first one, before a single note could be heard. See
// package sfz for the reading half of that other format.
//
// # The shape of the file
//
// An .sf2 is a RIFF file of three lists. INFO says what the bank is called
// and which version of the specification it follows. sdta is every
// recording, concatenated into one long run of 16-bit samples. pdta is the
// paperwork: nine tables that between them say which stretch of that run is
// which note of which instrument of which preset.
//
// The paperwork is indirect on purpose, because the format is built for
// sharing: a preset points at instruments, an instrument points at samples,
// and each arrow can carry settings of its own. A preset is what a MIDI
// program change selects; an instrument is a keyboard's worth of regions;
// a region is one recording and the keys it answers to.
//
// Three details of the encoding are load-bearing, and all three are the
// kind that produce a file every reader rejects:
//
//   - every one of the nine tables ends with a terminal record, which is
//     not an entry but a marker holding the index one past the last real
//     one. A reader works out how long each entry is by subtracting its
//     index from the next entry's, so without the terminator the last real
//     entry has no length;
//   - a zone's generators are a list, and the last of them must be the one
//     that names what the zone resolves to — sampleID in an instrument,
//     instrument in a preset. A zone whose list ends with anything else is
//     read as a global zone, a set of defaults for its neighbours, and the
//     region it was meant to be is silently not there;
//   - and the recordings are separated by at least forty-six frames of
//     silence, so that a synthesiser interpolating over the end of one
//     sample reads silence rather than the beginning of the next.
package sf2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

// Sample is one recording, and what it is a recording of.
//
// Data is mono: a soundfont stores stereo as two mono samples linked to
// each other, and nothing gscore does with these yet needs that. A flute
// recorded in a room is mono whatever the microphone was.
type Sample struct {
	// Name is what the sample is called inside the file, which is what
	// every editor shows in its list. Twenty characters, cut if longer.
	Name string

	// Rate is the samples per second it was recorded at. Kept as it was
	// rather than resampled: the synthesiser has to resample anyway to
	// play any note but the one it was recorded at, so converting here
	// would be one resampling for nothing.
	Rate int

	// Data is the recording itself.
	Data []int16

	// Root is the MIDI key the recording sounds at: 69 for the A above
	// middle C. Playing any other key shifts it by that many semitones.
	Root int

	// Cents is how far off that key it actually is, from -50 to 50, so
	// that a recording a few cents sharp can be played in tune. What the
	// specification calls the pitch correction.
	Cents int

	// LoopStart and LoopEnd are the stretch to repeat while the key is
	// held, in frames, LoopEnd being the frame after the last one of it.
	// Both zero means the sample does not loop, and a note is then as long
	// as the recording.
	//
	// This is what makes a two-second recording hold a long note. It is
	// also the hardest part of making a soundfont sound like an instrument
	// rather than like a loop, which is why what goes here is worked out
	// from the recording rather than guessed at — not in this package.
	LoopStart, LoopEnd int
}

// Region is one sample and the keys and velocities it answers to.
type Region struct {
	// Sample is which recording, by its place in Font.Samples.
	Sample int

	// KeyLo and KeyHi are the keys this region plays for, as MIDI note
	// numbers, inclusive. A region covering one key is lo == hi.
	KeyLo, KeyHi int

	// VelLo and VelHi are the velocities it answers to, inclusive. Both
	// zero means the whole range, which is what a soundfont of one
	// dynamic wants and saying 0 to 127 would also say.
	VelLo, VelHi int

	// Loop says to repeat the sample's loop while the key is held.
	//
	// Ignored where the sample has no loop: a region that says to loop a
	// sample with nothing to loop would be a note that never ends.
	Loop bool

	// Root overrides the sample's own root key for this region. Zero
	// means the sample's.
	Root int

	// Attenuation is how far down to turn this region, in decibels, where
	// zero is as recorded. Only down: the specification has no way to say
	// louder, because a sample that needs it was recorded too quietly.
	//
	// Decibels as they will actually sound. See attenuationUnits, which is
	// not the number the specification gives.
	Attenuation float64

	// Pan is where it sits, from -1 hard left to 1 hard right.
	Pan float64

	// Release is how long the sound takes to fade after the key is let
	// go, in seconds. Zero leaves the synthesiser's default, which is
	// immediate — right for a plucked string sampled to its end, wrong
	// for anything looping, which stops dead without it.
	Release float64
}

// Instrument is a keyboard's worth of regions under one name.
type Instrument struct {
	Name    string
	Regions []Region
}

// PresetZone points a preset at an instrument, optionally over part of the
// keyboard only.
type PresetZone struct {
	// Instrument is which one, by its place in Font.Instruments.
	Instrument int
	// KeyLo and KeyHi narrow it, as MIDI note numbers. Both zero means
	// the whole keyboard.
	KeyLo, KeyHi int
	// VelLo and VelHi narrow it by velocity. Both zero means all of them.
	VelLo, VelHi int
}

// Preset is what a MIDI program change selects: an instrument, or several,
// under a bank and program number.
type Preset struct {
	Name string
	// Bank and Program are where a score finds it. Program is 0 to 127;
	// bank 128 is the percussion bank, by convention older than the
	// format and obeyed by everything that reads one.
	Bank, Program int
	Zones         []PresetZone
}

// Font is a whole soundfont, ready to be written.
type Font struct {
	// Name is the bank's name, as every editor and gscore's own soundfont
	// list will show it.
	Name string

	// Engine is the synthesiser the bank was made for. Nothing reads it
	// and everything writes "EMU8000", after the chip in the Sound
	// Blaster AWE32 the format was invented for; left empty, so does this.
	Engine string

	// By and Comment are whose it is and anything to say about it. Both
	// optional, both shown by editors, and the place to record that a
	// bank is somebody's own flute rather than a download.
	By, Comment string

	Samples     []Sample
	Instruments []Instrument
	Presets     []Preset
}

// Generator numbers, from the specification's table. Only the ones written
// here; there are fifty-eight.
const (
	genPan          = 17
	genReleaseVol   = 38
	genKeyRange     = 43
	genVelRange     = 44
	genAttenuation  = 48
	genFineTune     = 52
	genSampleID     = 53
	genSampleModes  = 54
	genOverrideRoot = 58
)

// attenuationUnits is how many of the attenuation generator's units make a
// decibel.
//
// The specification says a unit is a tenth of a decibel, and that is not
// what anything does. The EMU8000 the format was written around treated it
// as four tenths, and every synthesiser since has followed the chip rather
// than the paper: FluidSynth multiplies by 0.4, and so does meltysynth, in
// voice.go, which is what gscore plays through. A unit is therefore a
// twenty-fifth of a decibel in practice, and twenty-five of them is what
// gets written for each decibel asked for.
//
// Which means a region written here as 12 dB down is 12 dB down when it is
// played, and would be read as 4.8 dB by anything that follows the text of
// the specification instead. There is no third choice: the two readings
// cannot both be satisfied by one number, and the one that matters is the
// one somebody will hear when they play their own soundfont back.
const attenuationUnits = 25

// loopContinuous is the sampleModes value for a sample that repeats for as
// long as the key is held. The other two are no loop, and a loop that plays
// on to the end of the sample once the key is let go.
const loopContinuous = 1

// gap is the silence between one recording and the next, in frames.
//
// Forty-six is the specification's figure, and it is not decoration: a
// synthesiser interpolating near the end of a sample reads a few frames
// past it, and what it reads should be silence rather than the attack of
// whatever was written next.
const gap = 46

// maxName is how long a name may be in any of the tables. Twenty bytes
// including the terminating zero, so nineteen characters.
const maxName = 20

// Write writes f as a soundfont.
//
// It is all written in one pass to an io.Writer and never seeks, so the
// destination can be a file, a buffer, or the body of a reply.
func Write(w io.Writer, f *Font) error {
	if err := f.check(); err != nil {
		return err
	}

	// The sample data first, because writing it decides where each
	// recording starts, and the sample headers have to say.
	wave, heads := f.layOutSamples()

	info := riffList("INFO", concat(
		chunk("ifil", version(2, 1)),
		chunk("isng", zstring(or(f.Engine, "EMU8000"))),
		chunk("INAM", zstring(f.Name)),
		optional("IENG", f.By),
		optional("ICMT", f.Comment),
		chunk("ISFT", zstring("gscore")),
	))

	sdta := riffList("sdta", chunk("smpl", wave))

	pdta, err := f.paperwork(heads)
	if err != nil {
		return err
	}

	body := concat([]byte("sfbk"), info, sdta, pdta)
	if _, err := w.Write(chunk("RIFF", body)); err != nil {
		return err
	}
	return nil
}

// check says what is wrong with a font before any of it is written.
//
// A soundfont that is wrong in these ways is not read by anything, and the
// error it produces in whatever eventually opens it says something about
// chunks rather than about the recording somebody made. Caught here, it can
// say which sample and what about it.
func (f *Font) check() error {
	if f == nil {
		return errors.New("sf2: there is no soundfont to write")
	}
	if strings.TrimSpace(f.Name) == "" {
		return errors.New("sf2: a soundfont has to be called something")
	}
	if len(f.Samples) == 0 {
		return errors.New("sf2: a soundfont with no recordings in it is " +
			"a soundfont that plays nothing")
	}
	if len(f.Instruments) == 0 {
		return errors.New("sf2: there are recordings but no instrument " +
			"laying them out on the keyboard")
	}
	if len(f.Presets) == 0 {
		return errors.New("sf2: there are instruments but no preset, so " +
			"nothing can select one")
	}
	for i, s := range f.Samples {
		where := fmt.Sprintf("sf2: recording %d (%q)", i+1, s.Name)
		if len(s.Data) == 0 {
			return fmt.Errorf("%s has nothing in it", where)
		}
		if s.Rate <= 0 {
			return fmt.Errorf("%s does not say what rate it was recorded "+
				"at", where)
		}
		if s.Root < 0 || s.Root > 127 {
			return fmt.Errorf("%s sounds at key %d, which is not a key",
				where, s.Root)
		}
		if s.Cents < -50 || s.Cents > 50 {
			return fmt.Errorf("%s is %d cents out, which is more than half "+
				"a semitone — it is a different note", where, s.Cents)
		}
		if s.LoopStart != 0 || s.LoopEnd != 0 {
			if s.LoopStart < 0 || s.LoopEnd > len(s.Data) ||
				s.LoopStart >= s.LoopEnd {
				return fmt.Errorf("%s loops from %d to %d, which is not "+
					"inside a recording of %d frames", where,
					s.LoopStart, s.LoopEnd, len(s.Data))
			}
		}
	}
	for i, in := range f.Instruments {
		if len(in.Regions) == 0 {
			return fmt.Errorf("sf2: instrument %d (%q) has no regions, so "+
				"no key plays anything", i+1, in.Name)
		}
		for j, r := range in.Regions {
			where := fmt.Sprintf("sf2: region %d of instrument %q",
				j+1, in.Name)
			if r.Sample < 0 || r.Sample >= len(f.Samples) {
				return fmt.Errorf("%s plays recording %d, and there are %d",
					where, r.Sample+1, len(f.Samples))
			}
			if err := keys(where, r.KeyLo, r.KeyHi); err != nil {
				return err
			}
			if err := vels(where, r.VelLo, r.VelHi); err != nil {
				return err
			}
			if r.Root < 0 || r.Root > 127 {
				return fmt.Errorf("%s is rooted at key %d, which is not a "+
					"key", where, r.Root)
			}
			if r.Attenuation < 0 {
				return fmt.Errorf("%s asks to be %g dB louder than it was "+
					"recorded, which a soundfont has no way to say",
					where, -r.Attenuation)
			}
		}
	}
	for i, p := range f.Presets {
		where := fmt.Sprintf("sf2: preset %d (%q)", i+1, p.Name)
		if len(p.Zones) == 0 {
			return fmt.Errorf("%s names no instrument", where)
		}
		if p.Bank < 0 || p.Bank > 128 {
			return fmt.Errorf("%s is in bank %d; there are 0 to 127, and "+
				"128 for percussion", where, p.Bank)
		}
		if p.Program < 0 || p.Program > 127 {
			return fmt.Errorf("%s is program %d, and there are 128",
				where, p.Program)
		}
		for _, z := range p.Zones {
			if z.Instrument < 0 || z.Instrument >= len(f.Instruments) {
				return fmt.Errorf("%s plays instrument %d, and there are %d",
					where, z.Instrument+1, len(f.Instruments))
			}
			if err := keys(where, z.KeyLo, z.KeyHi); err != nil {
				return err
			}
			if err := vels(where, z.VelLo, z.VelHi); err != nil {
				return err
			}
		}
	}
	return nil
}

func keys(where string, lo, hi int) error {
	if lo == 0 && hi == 0 {
		return nil
	}
	if lo < 0 || hi > 127 || lo > hi {
		return fmt.Errorf("%s covers keys %d to %d, which is not a stretch "+
			"of keyboard", where, lo, hi)
	}
	return nil
}

func vels(where string, lo, hi int) error {
	if lo == 0 && hi == 0 {
		return nil
	}
	if lo < 0 || hi > 127 || lo > hi {
		return fmt.Errorf("%s answers velocities %d to %d, which is not a "+
			"range", where, lo, hi)
	}
	return nil
}

// head is where one recording ended up in the single run of samples.
type head struct {
	start, end         int
	loopStart, loopEnd int
	rate, root, cents  int
	name               string
}

// layOutSamples writes every recording into one run of samples, separated
// by silence, and says where each of them went.
func (f *Font) layOutSamples() ([]byte, []head) {
	frames := 0
	for _, s := range f.Samples {
		frames += len(s.Data) + gap
	}
	wave := make([]byte, 0, frames*2)
	heads := make([]head, len(f.Samples))

	at := 0
	for i, s := range f.Samples {
		h := head{
			start: at,
			end:   at + len(s.Data),
			rate:  s.Rate,
			root:  s.Root,
			cents: s.Cents,
			name:  s.Name,
		}
		// A sample that does not loop still has to say something, and what
		// it says is a loop of the whole of it: readers differ about what
		// they do with a loop of zero length, and none of them plays it,
		// because whether to loop at all is the region's business and this
		// one will not ask for it.
		if s.LoopEnd > s.LoopStart {
			h.loopStart = at + s.LoopStart
			h.loopEnd = at + s.LoopEnd
		} else {
			h.loopStart = at
			h.loopEnd = at + len(s.Data)
		}
		heads[i] = h

		for _, v := range s.Data {
			wave = append(wave, byte(uint16(v)), byte(uint16(v)>>8))
		}
		wave = append(wave, make([]byte, gap*2)...)
		at += len(s.Data) + gap
	}
	return wave, heads
}

// paperwork writes the nine tables of the pdta list.
//
// They are written together because they refer to one another by index:
// every zone is a row in a bag table pointing into the generator table, and
// the indices only line up if one pass writes both.
func (f *Font) paperwork(heads []head) ([]byte, error) {
	var phdr, pbag, pgen, inst, ibag, igen, shdr []byte

	// The instruments, and the zones that make them up.
	for _, in := range f.Instruments {
		inst = append(inst, name20(in.Name)...)
		inst = append(inst, u16(len(ibag)/4)...)
		for _, r := range in.Regions {
			ibag = append(ibag, u16(len(igen)/4)...)
			ibag = append(ibag, u16(0)...) // no modulators
			igen = append(igen, f.generators(r, heads)...)
		}
	}
	inst = append(inst, name20("EOI")...)
	inst = append(inst, u16(len(ibag)/4)...)
	ibag = append(ibag, u16(len(igen)/4)...)
	ibag = append(ibag, u16(0)...)
	igen = append(igen, gen(0, 0)...)

	// The presets, the same way.
	for _, p := range f.Presets {
		phdr = append(phdr, name20(p.Name)...)
		phdr = append(phdr, u16(p.Program)...)
		phdr = append(phdr, u16(p.Bank)...)
		phdr = append(phdr, u16(len(pbag)/4)...)
		phdr = append(phdr, u32(0)...) // library
		phdr = append(phdr, u32(0)...) // genre
		phdr = append(phdr, u32(0)...) // morphology
		for _, z := range p.Zones {
			pbag = append(pbag, u16(len(pgen)/4)...)
			pbag = append(pbag, u16(0)...)
			if z.KeyLo != 0 || z.KeyHi != 0 {
				pgen = append(pgen, gen(genKeyRange, rng(z.KeyLo, z.KeyHi))...)
			}
			if z.VelLo != 0 || z.VelHi != 0 {
				pgen = append(pgen, gen(genVelRange, rng(z.VelLo, z.VelHi))...)
			}
			// Last, and that is the whole of why this is a zone rather
			// than a set of defaults for the zones after it.
			pgen = append(pgen, gen(genInstrument, uint16(z.Instrument))...)
		}
	}
	phdr = append(phdr, name20("EOP")...)
	phdr = append(phdr, u16(0)...)
	phdr = append(phdr, u16(0)...)
	phdr = append(phdr, u16(len(pbag)/4)...)
	phdr = append(phdr, make([]byte, 12)...)
	pbag = append(pbag, u16(len(pgen)/4)...)
	pbag = append(pbag, u16(0)...)
	pgen = append(pgen, gen(0, 0)...)

	// And the sample headers, which say which stretch of the run is which.
	for _, h := range heads {
		shdr = append(shdr, name20(h.name)...)
		shdr = append(shdr, u32(h.start)...)
		shdr = append(shdr, u32(h.end)...)
		shdr = append(shdr, u32(h.loopStart)...)
		shdr = append(shdr, u32(h.loopEnd)...)
		shdr = append(shdr, u32(h.rate)...)
		shdr = append(shdr, byte(h.root))
		shdr = append(shdr, byte(int8(h.cents)))
		shdr = append(shdr, u16(0)...) // not linked to another channel
		shdr = append(shdr, u16(1)...) // a mono sample
	}
	shdr = append(shdr, name20("EOS")...)
	shdr = append(shdr, make([]byte, 26)...)

	return riffList("pdta", concat(
		chunk("phdr", phdr),
		chunk("pbag", pbag),
		chunk("pmod", make([]byte, 10)), // the terminal record, and no more
		chunk("pgen", pgen),
		chunk("inst", inst),
		chunk("ibag", ibag),
		chunk("imod", make([]byte, 10)),
		chunk("igen", igen),
		chunk("shdr", shdr),
	)), nil
}

// genInstrument is the generator that points a preset zone at an
// instrument. It belongs with the others and is here because it is the
// only one of them that may not appear in an instrument zone.
const genInstrument = 41

// generators writes one region's settings, ending with the one that names
// the sample.
//
// The order is the specification's: the key range first, the velocity range
// second, the sample last. What is in between may be in any order, and is
// written in the order of the generator numbers because a file that is
// predictable is a file that can be compared.
func (f *Font) generators(r Region, heads []head) []byte {
	var out []byte
	if r.KeyLo != 0 || r.KeyHi != 0 {
		out = append(out, gen(genKeyRange, rng(r.KeyLo, r.KeyHi))...)
	}
	if r.VelLo != 0 || r.VelHi != 0 {
		out = append(out, gen(genVelRange, rng(r.VelLo, r.VelHi))...)
	}
	if r.Pan != 0 {
		out = append(out, gen(genPan, uint16(int16(clamp(r.Pan*500, -500, 500))))...)
	}
	if r.Release > 0 {
		out = append(out, gen(genReleaseVol, uint16(int16(timecents(r.Release))))...)
	}
	if r.Attenuation > 0 {
		out = append(out, gen(genAttenuation,
			uint16(clamp(r.Attenuation*attenuationUnits, 0, 1440)))...)
	}
	s := f.Samples[r.Sample]
	if s.Cents != 0 {
		// The sample header carries this too, and some readers only look
		// here. Writing both costs four bytes and is read correctly by
		// everything: the two agree, so whichever is consulted is right.
		out = append(out, gen(genFineTune, uint16(int16(s.Cents)))...)
	}
	// A loop is only asked for where there is one to ask for, since a
	// region looping a sample with nothing to loop is a note with no end.
	if r.Loop && s.LoopEnd > s.LoopStart {
		out = append(out, gen(genSampleModes, loopContinuous)...)
	}
	root := s.Root
	if r.Root != 0 {
		root = r.Root
	}
	out = append(out, gen(genOverrideRoot, uint16(root))...)
	out = append(out, gen(genSampleID, uint16(r.Sample))...)
	return out
}

/* ---------- the RIFF underneath ---------- */

// chunk wraps a body in its four-character name and its length.
//
// RIFF chunks are padded to an even length and the padding is not counted,
// which is the one place in the format where the length in the file and the
// bytes that follow it disagree on purpose.
func chunk(id string, body []byte) []byte {
	out := make([]byte, 0, len(body)+9)
	out = append(out, id...)
	out = append(out, u32(len(body))...)
	out = append(out, body...)
	if len(body)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// riffList is a chunk whose body begins with a second four-character name.
func riffList(kind string, body []byte) []byte {
	return chunk("LIST", append([]byte(kind), body...))
}

// optional writes a text chunk, or nothing where there is nothing to say.
func optional(id, text string) []byte {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return chunk(id, zstring(text))
}

func concat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// version is the ifil body: a major and a minor, each two bytes.
func version(major, minor int) []byte {
	return append(u16(major), u16(minor)...)
}

// zstring is text with a zero after it, padded to an even length as RIFF
// requires of everything.
func zstring(s string) []byte {
	b := append([]byte(s), 0)
	if len(b)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

// name20 is a name as the tables hold one: twenty bytes, zero-padded, and
// cut rather than refused where it is too long.
//
// Cut, because the alternative is refusing to write somebody's soundfont
// over the name of one sample in it. Nineteen characters survive; the
// twentieth byte is the zero that ends it, which some readers rely on.
func name20(s string) []byte {
	b := make([]byte, maxName)
	if len(s) > maxName-1 {
		s = s[:maxName-1]
	}
	copy(b, s)
	return b
}

// gen is one generator: which setting, and its value.
func gen(oper int, amount uint16) []byte {
	return append(u16(oper), u16(int(amount))...)
}

// rng packs a low and a high into the two bytes a range generator holds.
func rng(lo, hi int) uint16 {
	return uint16(lo&0xff) | uint16(hi&0xff)<<8
}

// timecents is how the format counts time: 1200 to the doubling, from one
// second. A third of a second is -1900 or so, and a whole second is 0.
func timecents(seconds float64) int {
	if seconds <= 0 {
		return 0
	}
	return int(math.Round(1200 * math.Log2(seconds)))
}

func clamp(v, lo, hi float64) float64 {
	return math.Min(math.Max(v, lo), hi)
}

func or(a, b string) string {
	if strings.TrimSpace(a) == "" {
		return b
	}
	return a
}

func u16(v int) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}

func u32(v int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return b
}
