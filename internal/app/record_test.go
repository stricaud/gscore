/*
 * record_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/synth"
)

// trio is a score of three staves, each a different instrument, which is the
// case the whole recording dialog exists for: one staff tells you nothing
// about whether a soundfont covers the piece.
func trio() *model.Score {
	s := &model.Score{Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}}}
	for i, inst := range []model.Instrument{model.Flute, model.Trumpet, model.Cello} {
		p := &model.Part{
			Name:       []string{"Flute", "Trumpet", "Cello"}[i],
			Clef:       model.Treble,
			Instrument: inst,
			Time:       model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		var els []model.Element
		for k := 0; k < 4; k++ {
			els = append(els, &model.Note{
				Pitch: model.Pitch{Step: model.Step(k), Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 4},
			})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
		s.Parts = append(s.Parts, p)
	}
	s.Normalise()
	return s
}

// The review is what the dialog shows before anything is recorded: every
// staff, what it is scored for, and — once a soundfont is chosen — what will
// actually be heard playing it.
func TestEveryStaffIsListedWithItsInstrument(t *testing.T) {
	a := &App{score: trio()}
	got := a.voices(nil, "")
	if len(got) != 3 {
		t.Fatalf("%d staves reviewed, want 3", len(got))
	}
	want := []struct {
		name       string
		instrument string
		program    int
	}{
		{"Flute", "flute", 73},
		{"Trumpet", "trumpet", 56},
		{"Cello", "cello", 42},
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].Instrument != w.instrument ||
			got[i].Program != w.program {
			t.Errorf("staff %d came back as %q/%q/program %d, want %q/%q/program %d",
				i, got[i].Name, got[i].Instrument, got[i].Program,
				w.name, w.instrument, w.program)
		}
		// With no soundfont chosen there is nothing to say about what will
		// sound, and nothing is missing either: the question has not been
		// asked yet.
		if got[i].Preset != "" || got[i].Missing {
			t.Errorf("staff %d claims to know what will play it with no soundfont chosen", i)
		}
	}
}

// A hidden staff still sounds, so it is still in the review. Hiding one is a
// decision about the page.
func TestAHiddenStaffIsStillRecorded(t *testing.T) {
	s := trio()
	s.Parts[1].Hidden = true
	if got := (&App{score: s}).voices(nil, ""); len(got) != 3 {
		t.Errorf("%d staves reviewed with one hidden, want all 3", len(got))
	}
}

func TestReadingWhichStaffPlaysThroughWhat(t *testing.T) {
	got := parseAssignment("0=/one.sf2,2=/two.sf2")
	if len(got) != 2 || got[0] != "/one.sf2" || got[2] != "/two.sf2" {
		t.Errorf("got %v", got)
	}
	// Nothing said is not an error, and neither is nonsense: the recording
	// falls back on the one soundfont rather than refusing to be made.
	for _, s := range []string{"", "rubbish", "=", "x=/a.sf2", "3="} {
		if len(parseAssignment(s)) != 0 {
			t.Errorf("%q was read as an assignment", s)
		}
	}
}

func TestTheRecordingSettingsHaveSensibleAnswers(t *testing.T) {
	rec := readRecording(url.Values{})
	if rec.SampleRate != 44100 || rec.Gain != 1 || !rec.Reverb || !rec.Normalize {
		t.Errorf("the defaults came out as %+v", rec.Options)
	}
	rec = readRecording(url.Values{
		"rate": {"48000"}, "reverb": {"0"}, "normalize": {"0"}, "tail": {"0"},
	})
	if rec.SampleRate != 48000 || rec.Reverb || rec.Normalize || rec.Tail != 0 {
		t.Errorf("the settings were not read: %+v", rec.Options)
	}
}

// Asked to record with no soundfont, gscore says so. Silence would be the
// other way to answer and is the one that wastes somebody's afternoon.
func TestRecordingWithoutASoundfontSaysSo(t *testing.T) {
	err := recordFLAC(&bytes.Buffer{}, trio(), recording{})
	if err == nil {
		t.Fatal("recording with no soundfont succeeded")
	}
}

// And with one, it makes a FLAC file with sound in it.
//
// This needs a soundfont, which is somebody else's thirty megabytes and is
// not in the repository, so it runs where one has been downloaded and says
// nothing where one has not.
func TestARecordingHasSoundInIt(t *testing.T) {
	installed := listSoundfonts()
	if len(installed) == 0 {
		t.Skip("no soundfont on this machine; download one to run this")
	}
	var buf bytes.Buffer
	err := recordFLAC(&buf, trio(), recording{
		Font: installed[0].Path, Options: fastRecording(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("fLaC")) {
		t.Fatal("what came out is not a FLAC file")
	}
	// Four seconds of music at CD quality is megabytes. A few kilobytes
	// would mean every note came out silent, which is what a soundfont
	// missing the instrument does.
	if buf.Len() < 100_000 {
		t.Errorf("the recording is %d bytes, which is too small to be music",
			buf.Len())
	}
	if testing.Verbose() {
		t.Logf("recorded %d bytes through %s", buf.Len(), installed[0].Name)
	}
}

// fastRecording is the same recording with the slow parts turned off, so the
// test spends its time on whether there is sound rather than on the room it
// is heard in.
func fastRecording() synth.Options {
	o := synth.DefaultOptions()
	o.Reverb = false
	o.Tail = 0.2
	return o
}
