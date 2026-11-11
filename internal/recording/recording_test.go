/*
 * recording_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading a recorded note.
 *
 * The takes here are made rather than recorded, because a test needs to
 * know the answer before it asks the question: a tone built at 392 Hz with
 * a tenth of a second of room either side has one right answer about its
 * pitch and two about where it starts and stops. What they are built to
 * have is what real takes have and what the reading has to survive — a
 * breathy attack, harmonics strong enough to be mistaken for the
 * fundamental, vibrato, a swell, and a room that is never quite silent.
 */

package recording

import (
	"math"
	"math/rand"
	"testing"
)

/* ---------- making a take ---------- */

// take is a note built to order: so many seconds at a pitch, with room at
// either end and whatever shape of envelope and harmonics are asked for.
type take struct {
	rate     int
	hz       float64
	seconds  float64
	lead     float64 // silence before the note
	tail     float64 // and after it
	attack   float64 // how long the note takes to come up
	decay    float64 // and to die away
	partials []float64
	vibrato  float64 // depth in cents
	swell    float64 // how much louder it gets over its length
	noise    float64 // room, as a fraction of the note
}

func (c take) make() []int16 {
	if c.rate == 0 {
		c.rate = 44100
	}
	if len(c.partials) == 0 {
		c.partials = []float64{1}
	}
	if c.attack == 0 {
		c.attack = 0.04
	}
	if c.decay == 0 {
		c.decay = 0.08
	}
	rng := rand.New(rand.NewSource(1))
	n := int((c.lead + c.seconds + c.tail) * float64(c.rate))
	out := make([]int16, n)
	lead := int(c.lead * float64(c.rate))
	note := int(c.seconds * float64(c.rate))

	phase := 0.0
	for i := 0; i < n; i++ {
		v := 0.0
		if i >= lead && i < lead+note {
			t := float64(i-lead) / float64(c.rate)
			// Where in the note, for the envelope.
			env := 1.0
			if t < c.attack {
				env = t / c.attack
			}
			left := c.seconds - t
			if left < c.decay {
				env = left / c.decay
			}
			if env < 0 {
				env = 0
			}
			env *= 1 + c.swell*t/c.seconds

			hz := c.hz
			if c.vibrato > 0 {
				hz *= math.Pow(2, c.vibrato*math.Sin(2*math.Pi*5*t)/1200)
			}
			phase += 2 * math.Pi * hz / float64(c.rate)
			for k, g := range c.partials {
				v += g * math.Sin(float64(k+1)*phase)
			}
			v *= env * 0.5
		}
		if c.noise > 0 {
			v += c.noise * (rng.Float64()*2 - 1)
		}
		out[i] = int16(math.Max(-1, math.Min(1, v)) * 30000)
	}
	return out
}

// flute is what most of these tests ask about: a breathy tone with a
// little second harmonic, vibrato, and a room behind it.
func flute(hz, seconds float64) take {
	return take{
		rate: 44100, hz: hz, seconds: seconds, lead: 0.15, tail: 0.25,
		attack: 0.07, decay: 0.12,
		partials: []float64{1, 0.25, 0.08},
		vibrato:  8, noise: 0.004,
	}
}

/* ---------- which note ---------- */

func TestThePitchIsTheNoteThatWasPlayed(t *testing.T) {
	for _, hz := range []float64{196, 261.63, 440, 880, 1567.98} {
		got, ok := Pitch(flute(hz, 1.2).make(), 44100)
		if !ok {
			t.Errorf("%.2f Hz was heard as no pitch at all", hz)
			continue
		}
		// Within five cents, which is closer than anybody tunes.
		if off := 1200 * math.Log2(got/hz); math.Abs(off) > 5 {
			t.Errorf("%.2f Hz was heard as %.2f (%.0f cents out)",
				hz, got, off)
		}
	}
}

func TestAStrongHarmonicIsNotMistakenForTheNote(t *testing.T) {
	// The mistake plain autocorrelation makes, and the reason for the
	// normalisation in yin: a tone whose second harmonic is as loud as its
	// fundamental is heard an octave high, and one with a weak fundamental
	// an octave low. Both are a semitone-perfect sample of the wrong note.
	c := flute(220, 1.2)
	c.partials = []float64{0.4, 1, 0.6, 0.3} // the octave is the loudest thing in it
	got, ok := Pitch(c.make(), 44100)
	if !ok {
		t.Fatal("a tone rich in harmonics was heard as no pitch")
	}
	if off := 1200 * math.Log2(got/220); math.Abs(off) > 10 {
		t.Errorf("heard as %.1f Hz, want 220 — %.0f cents out, which is "+
			"%.1f octaves", got, off, off/1200)
	}
}

func TestVibratoIsStillOneNote(t *testing.T) {
	c := flute(440, 1.5)
	c.vibrato = 40 // a wide one, nearly a quarter-tone either way
	got, ok := Pitch(c.make(), 44100)
	if !ok {
		t.Fatal("a note with vibrato on it was heard as no pitch")
	}
	if off := 1200 * math.Log2(got/440); math.Abs(off) > 15 {
		t.Errorf("heard as %.1f Hz, want about 440 (%.0f cents out)", got, off)
	}
}

func TestSomethingWithNoPitchIsNotGivenOne(t *testing.T) {
	// A cymbal, a key click, a cough. Saying 93.7 Hz about it would put a
	// wrong note on the keyboard rather than admit there is none.
	c := take{rate: 44100, seconds: 1, lead: 0.1, tail: 0.1, noise: 0.5}
	if hz, ok := Pitch(c.make(), 44100); ok {
		t.Errorf("noise was heard as %.1f Hz", hz)
	}
}

func TestAKeyAndHowFarFromIt(t *testing.T) {
	for _, c := range []struct {
		hz    float64
		key   int
		cents int
	}{
		{440, 69, 0},
		{261.63, 60, 0},
		{442, 69, 8},      // sharp, as an orchestra often is
		{437, 69, -12},    // and flat
		{27.5, 21, 0},     // the bottom of a piano
		{4186.01, 108, 0}, // and the top
	} {
		key, cents := Key(c.hz)
		if key != c.key || math.Abs(float64(cents-c.cents)) > 1 {
			t.Errorf("%.2f Hz is key %d %+d cents, want key %d %+d",
				c.hz, key, cents, c.key, c.cents)
		}
	}
}

func TestAKeyIsNeverMoreThanHalfASemitoneOut(t *testing.T) {
	// Which is what makes the correction fit the field a soundfont has for
	// it: rounding to the nearest key leaves at most fifty cents.
	for hz := 50.0; hz < 2000; hz *= 1.01 {
		_, cents := Key(hz)
		if cents < -50 || cents > 50 {
			t.Fatalf("%.1f Hz came back %+d cents from its key", hz, cents)
		}
	}
}

/* ---------- where it starts and stops ---------- */

func TestTheSilenceAtEitherEndIsCutOff(t *testing.T) {
	c := flute(440, 1.0)
	c.lead, c.tail = 0.4, 0.5
	from, to := Trim(c.make(), 44100)

	// The note starts at 0.4 s. What is cut has to stop short of that.
	if got := float64(from) / 44100; got > 0.4 {
		t.Errorf("the take was cut at %.3f s, into a note that starts at "+
			"0.400", got)
	}
	// And not so far short that most of the lead-in silence is kept.
	if got := float64(from) / 44100; got < 0.3 {
		t.Errorf("the take was cut at %.3f s, leaving %.0f ms of room "+
			"before the note", got, (0.4-got)*1000)
	}
	// The note ends at 1.4 s; the tail should go.
	if got := float64(to) / 44100; got > 1.55 {
		t.Errorf("the take runs to %.3f s, keeping the silence after a "+
			"note that ends at 1.400", got)
	}
	if got := float64(to) / 44100; got < 1.35 {
		t.Errorf("the take runs only to %.3f s, cutting the end off a "+
			"note that ends at 1.400", got)
	}
}

func TestTheAttackIsKept(t *testing.T) {
	// The part a sampler must not throw away: the first tenth of a second
	// of a flute is the breath taking hold, and it is most of what makes
	// it sound like a flute. A take trimmed to its steady part fades in
	// from nowhere.
	c := flute(440, 1.0)
	c.attack = 0.12
	data := c.make()
	from, _ := Trim(data, 44100)

	// Whatever is at the start of the trimmed take must be quiet — the
	// beginning of the swell, not the middle of it.
	head := peakOf(data[from : from+441]) // the first ten milliseconds
	all := peakOf(data)
	if head > all/3 {
		t.Errorf("the take begins at %.3f of full level against a peak of "+
			"%.3f: the attack was cut off", head, all)
	}
	if head == 0 {
		t.Error("the take begins in silence, so the attack is not there at all")
	}
}

func TestATakeBeginsAtACrossingOfZero(t *testing.T) {
	// A sample that starts part way up a waveform starts with a click, and
	// a click at the front of every note is what a home-made soundfont
	// sounds like when nobody has thought about this.
	data := flute(330, 1.0).make()
	from, _ := Trim(data, 44100)
	if from > 0 && math.Abs(float64(data[from])) > 2000 {
		t.Errorf("the take begins at %d of 32768, which is a click",
			data[from])
	}
}

func TestATakeWithNothingInItIsRefused(t *testing.T) {
	quiet := make([]int16, 44100)
	if _, err := Read(quiet, 44100); err == nil {
		t.Error("a take of pure silence was read as a recording")
	}
	if _, err := Read(nil, 44100); err == nil {
		t.Error("a take of nothing at all was read as a recording")
	}
	if _, err := Read(flute(440, 1).make(), 0); err == nil {
		t.Error("a take that does not say its rate was read anyway")
	}
}

/* ---------- where it loops ---------- */

func TestTheLoopIsAWholeNumberOfPeriods(t *testing.T) {
	const hz = 440
	data := flute(hz, 2.0).make()
	start, end, ok := Loop(data, 44100, hz)
	if !ok {
		t.Fatal("no loop was found in two seconds of steady flute")
	}
	period := 44100.0 / hz
	cycles := float64(end-start) / period
	if off := math.Abs(cycles - math.Round(cycles)); off > 0.1 {
		t.Errorf("the loop is %.2f periods long, which does not come back "+
			"to where it started", cycles)
	}
	if cycles < 4 {
		t.Errorf("the loop is only %.1f periods long, which will buzz", cycles)
	}
}

func TestTheTwoEndsOfTheLoopMatch(t *testing.T) {
	// The whole of what makes a loop inaudible: the sound at the end has
	// to carry on into the sound at the start without a step.
	const hz = 392
	data := flute(hz, 2.0).make()
	start, end, ok := Loop(data, 44100, hz)
	if !ok {
		t.Fatal("no loop was found")
	}
	window := int(2 * 44100.0 / hz)
	if got := difference(data, start, end, window); got > loopTolerance {
		t.Errorf("the ends of the loop differ by %.3f, want under %.2f",
			got, loopTolerance)
	}
}

func TestTheLoopIsInTheSteadyPart(t *testing.T) {
	// Not over the attack, which is different every time round, and not
	// over the decay, which would make a held note fade and come back.
	const hz = 440
	c := flute(hz, 2.0)
	c.attack, c.decay = 0.2, 0.3
	data := c.make()
	from, to := Trim(data, 44100)
	cut := data[from:to]
	start, end, ok := Loop(cut, 44100, hz)
	if !ok {
		t.Fatal("no loop was found")
	}
	if float64(start)/44100 < 0.2 {
		t.Errorf("the loop starts at %.3f s, inside the attack",
			float64(start)/44100)
	}
	if float64(len(cut)-end)/44100 < 0.05 {
		t.Errorf("the loop ends %.3f s from the end of the take, inside "+
			"the decay", float64(len(cut)-end)/44100)
	}
}

func TestASoundThatNeverSettlesHasNoLoop(t *testing.T) {
	// Better to say so than to write a loop that clicks on every repeat:
	// a note held without a loop merely stops, which is a limitation. A
	// bad loop is a fault you hear for as long as the note lasts.
	c := take{rate: 44100, hz: 440, seconds: 2, lead: 0.1, tail: 0.1,
		partials: []float64{1}, noise: 0.9}
	data := c.make()
	if _, _, ok := Loop(data, 44100, 440); ok {
		t.Error("a loop was found in a sound that is mostly noise")
	}
}

func TestTooShortToLoopIsNotLooped(t *testing.T) {
	const hz = 440
	// Three periods: there is nothing here to repeat.
	c := take{rate: 44100, hz: hz, seconds: 3 / hz, partials: []float64{1}}
	if _, _, ok := Loop(c.make(), 44100, hz); ok {
		t.Error("a loop was found in three cycles of sound")
	}
	if _, _, ok := Loop(flute(hz, 2).make(), 44100, 0); ok {
		t.Error("a loop was found in something with no pitch")
	}
}

/* ---------- all of it together ---------- */

func TestReadingATake(t *testing.T) {
	const hz = 523.25 // C5
	data := flute(hz, 2.0).make()
	got, err := Read(data, 44100)
	if err != nil {
		t.Fatalf("reading the take: %v", err)
	}
	if got.Key != 72 {
		t.Errorf("the take is key %d, want 72 — C5", got.Key)
	}
	if got.Cents < -6 || got.Cents > 6 {
		t.Errorf("the take is %+d cents from the key, want near nought",
			got.Cents)
	}
	if got.LoopEnd <= got.LoopStart {
		t.Error("the take came back with no loop in two seconds of flute")
	}
	if got.LoopEnd > len(got.Data) {
		t.Errorf("the loop ends at %d, past the end of a take of %d frames",
			got.LoopEnd, len(got.Data))
	}
	if len(got.Data) >= len(data) {
		t.Error("nothing was trimmed from a take with silence either end")
	}
	if got.Peak < 0.1 {
		t.Errorf("the take peaks at %.3f, which is not a usable recording",
			got.Peak)
	}
	if got.Rate != 44100 {
		t.Errorf("the rate came back as %d", got.Rate)
	}
}

func TestADrumIsReadAsADrum(t *testing.T) {
	// Something with no pitch still comes back: a percussion bank is a
	// soundfont too, and a program that refused the take would be refusing
	// the instrument. See the comment on Read.
	c := take{rate: 44100, seconds: 0.4, lead: 0.1, tail: 0.5,
		attack: 0.001, decay: 0.3, noise: 0.5}
	got, err := Read(c.make(), 44100)
	if err != nil {
		t.Fatalf("a cymbal was refused: %v", err)
	}
	if got.Hz != 0 {
		t.Errorf("a pitch of %.1f Hz was invented for it", got.Hz)
	}
	if got.LoopEnd != 0 {
		t.Error("a cymbal was given a loop")
	}
	if len(got.Data) == 0 {
		t.Error("nothing came back")
	}
}

func TestATakeThatSwellsStillLoops(t *testing.T) {
	// Nobody holds a note at one level. The loop finder settles for the
	// closest match it can find rather than insisting on a perfect one,
	// and a note that grows through its length still has one.
	const hz = 349.23
	c := flute(hz, 2.5)
	c.swell = 0.4
	got, err := Read(c.make(), 44100)
	if err != nil {
		t.Fatal(err)
	}
	if got.LoopEnd <= got.LoopStart {
		t.Error("a note that swells was given no loop")
	}
}
