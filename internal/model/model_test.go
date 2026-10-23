package model

import "testing"

func TestPitchMIDI(t *testing.T) {
	cases := []struct {
		p    Pitch
		midi int
	}{
		{Pitch{StepC, 4, 0}, 60}, // middle C
		{Pitch{StepA, 4, 0}, 69}, // concert A
		{Pitch{StepC, 4, 1}, 61},
		{Pitch{StepB, 3, 0}, 59},
		{Pitch{StepC, -1, 0}, 0},
		{Pitch{StepD, 4, -1}, 61}, // D flat sounds as C sharp
	}
	for _, c := range cases {
		if got := c.p.MIDI(); got != c.midi {
			t.Errorf("%v.MIDI() = %d, want %d", c.p, got, c.midi)
		}
	}
}

func TestDiatonicIndexIgnoresAlteration(t *testing.T) {
	// An accidental never moves a notehead, so it must not move the index.
	plain := Pitch{StepF, 4, 0}
	sharp := Pitch{StepF, 4, 1}
	if plain.DiatonicIndex() != sharp.DiatonicIndex() {
		t.Errorf("F4 and F#4 have different staff positions: %d vs %d",
			plain.DiatonicIndex(), sharp.DiatonicIndex())
	}
	cases := []struct {
		p    Pitch
		want int
	}{
		{Pitch{StepC, 4, 0}, 0},
		{Pitch{StepD, 4, 0}, 1},
		{Pitch{StepB, 4, 0}, 6},
		{Pitch{StepC, 5, 0}, 7},
		{Pitch{StepB, 3, 0}, -1},
		{Pitch{StepC, 3, 0}, -7},
	}
	for _, c := range cases {
		if got := c.p.DiatonicIndex(); got != c.want {
			t.Errorf("%v.DiatonicIndex() = %d, want %d", c.p, got, c.want)
		}
	}
}

func TestPitchFromDiatonicIndexRoundTrips(t *testing.T) {
	for i := -40; i <= 40; i++ {
		if got := PitchFromDiatonicIndex(i).DiatonicIndex(); got != i {
			t.Errorf("PitchFromDiatonicIndex(%d).DiatonicIndex() = %d", i, got)
		}
	}
}

func TestTransposePreservesSpelling(t *testing.T) {
	// The interval's diatonic size and its semitone size together fix the
	// spelling of the result; this is what the old staff-position-only pitch
	// could not express.
	c4 := Pitch{StepC, 4, 0}
	cases := []struct {
		name            string
		steps, semitone int
		from            Pitch
		want            string
	}{
		{"major third up", 2, 4, c4, "E4"},
		{"minor third up", 2, 3, c4, "Eb4"},
		{"diminished fourth up", 3, 4, c4, "Fb4"},
		{"perfect fifth up", 4, 7, c4, "G4"},
		{"octave up", 7, 12, c4, "C5"},
		{"minor third down", -2, -3, c4, "A3"},
		{"augmented second up", 1, 3, c4, "D#4"},
	}
	for _, c := range cases {
		if got := c.from.Transpose(c.steps, c.semitone).String(); got != c.want {
			t.Errorf("%s: %v.Transpose(%d, %d) = %s, want %s",
				c.name, c.from, c.steps, c.semitone, got, c.want)
		}
	}
}

func TestKeySignatureAlterFor(t *testing.T) {
	gMajor := KeySignature{Fifths: 1, Mode: Major}
	if got := gMajor.AlterFor(StepF); got != 1 {
		t.Errorf("G major alters F by %d, want 1", got)
	}
	for _, s := range []Step{StepC, StepD, StepE, StepG, StepA, StepB} {
		if got := gMajor.AlterFor(s); got != 0 {
			t.Errorf("G major alters %v by %d, want 0", s, got)
		}
	}

	bFlat := KeySignature{Fifths: -2, Mode: Major}
	for _, s := range []Step{StepB, StepE} {
		if got := bFlat.AlterFor(s); got != -1 {
			t.Errorf("B flat major alters %v by %d, want -1", s, got)
		}
	}
	if got := bFlat.AlterFor(StepA); got != 0 {
		t.Errorf("B flat major alters A by %d, want 0", got)
	}

	// Sharps and flats must arrive in the order they are written.
	if got := (KeySignature{Fifths: 3}).AlterFor(StepG); got != 1 {
		t.Errorf("three sharps should alter G, got %d", got)
	}
	if got := (KeySignature{Fifths: 2}).AlterFor(StepG); got != 0 {
		t.Errorf("two sharps should not alter G, got %d", got)
	}
}

func TestDurationArithmetic(t *testing.T) {
	quarter := Duration{1, 4}
	if got := quarter.Add(quarter).Reduced(); got != (Duration{1, 2}) {
		t.Errorf("1/4 + 1/4 = %v, want 1/2", got)
	}
	if got := quarter.Scale(3, 2); got != (Duration{3, 8}) {
		t.Errorf("1/4 * 3/2 = %v, want 3/8", got)
	}
	if got := (Duration{1, 2}).Sub(quarter); got != quarter {
		t.Errorf("1/2 - 1/4 = %v, want 1/4", got)
	}
	if (Duration{2, 8}).Cmp(quarter) != 0 {
		t.Error("2/8 should compare equal to 1/4")
	}
	if quarter.Cmp(Duration{1, 2}) != -1 {
		t.Error("1/4 should sort before 1/2")
	}
}

func TestNoteValueDurations(t *testing.T) {
	cases := []struct {
		v    NoteValue
		want Duration
	}{
		{DoubleWhole, Duration{2, 1}},
		{Whole, Duration{1, 1}},
		{Half, Duration{1, 2}},
		{Quarter, Duration{1, 4}},
		{Eighth, Duration{1, 8}},
		{SixtyFourth, Duration{1, 64}},
	}
	for _, c := range cases {
		if got := c.v.Duration(); got != c.want {
			t.Errorf("%v.Duration() = %v, want %v", c.v, got, c.want)
		}
	}
	if got := Eighth.Flags(); got != 1 {
		t.Errorf("an eighth has %d flags, want 1", got)
	}
	if got := Quarter.Flags(); got != 0 {
		t.Errorf("a quarter has %d flags, want 0", got)
	}
}

func TestDurationWritten(t *testing.T) {
	cases := []struct {
		d     Duration
		value NoteValue
		dots  int
		ok    bool
	}{
		{Duration{1, 4}, Quarter, 0, true},
		{Duration{3, 8}, Quarter, 1, true},  // dotted quarter
		{Duration{7, 16}, Quarter, 2, true}, // double dotted quarter
		{Duration{1, 1}, Whole, 0, true},
		{Duration{3, 2}, Whole, 1, true},
		{Duration{2, 1}, DoubleWhole, 0, true},
		{Duration{2, 8}, Quarter, 0, true}, // reduces to 1/4 first
		// A triplet eighth is a third of a quarter and no notehead spells it;
		// the caller has to know that rather than be handed a rounded answer.
		{Duration{1, 12}, 0, 0, false},
		{Duration{0, 1}, 0, 0, false},
	}
	for _, c := range cases {
		v, dots, ok := c.d.Written()
		if ok != c.ok {
			t.Errorf("%v.Written() ok = %v, want %v", c.d, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if v != c.value || dots != c.dots {
			t.Errorf("%v.Written() = %v with %d dots, want %v with %d dots",
				c.d, v, dots, c.value, c.dots)
		}
	}
}

func TestTimeSignatureBarDuration(t *testing.T) {
	cases := []struct {
		t    TimeSignature
		want Duration
	}{
		{TimeSignature{4, 4, TimeCommon}, Duration{1, 1}},
		{TimeSignature{3, 4, TimeNumeric}, Duration{3, 4}},
		{TimeSignature{6, 8, TimeNumeric}, Duration{3, 4}},
		{TimeSignature{2, 2, TimeCut}, Duration{1, 1}},
	}
	for _, c := range cases {
		if got := c.t.BarDuration(); got != c.want {
			t.Errorf("%v.BarDuration() = %v, want %v", c.t, got, c.want)
		}
	}
}

func TestVoiceDuration(t *testing.T) {
	v := &Voice{Elements: []Element{
		&Note{Dur: Duration{1, 4}},
		&Rest{Dur: Duration{1, 4}},
		&Chord{Dur: Duration{1, 2}},
	}}
	if got := v.Duration(); got != (Duration{1, 1}) {
		t.Errorf("voice duration = %v, want 1/1", got)
	}
}
