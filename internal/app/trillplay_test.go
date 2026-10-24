package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A trill is an alternation, not a long note. Played as written it was just
// the note held, which is the one thing a trill is not.
func trilledScore() *model.Score {
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepG, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 1},
	}
	n.Trill = &model.Trill{Start: true, Stop: true}
	return &model.Score{
		Tempo: model.Tempo{BPM: 60, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{n}}},
				RightBarline: model.BarFinal,
			}},
		}},
	}
}

func TestATrillIsPlayedAsAnAlternation(t *testing.T) {
	pb := buildPlayback(trilledScore())

	// Four seconds of whole note at 60 to the quarter, twelve a second.
	if len(pb.Events) < 40 {
		t.Fatalf("the trill sounded as %d notes; it is an alternation, not a held note",
			len(pb.Events))
	}

	// Between the written note and the one above it in the key: G and A.
	g, a := model.Pitch{Step: model.StepG, Octave: 4}.MIDI(),
		model.Pitch{Step: model.StepA, Octave: 4}.MIDI()
	for i, ev := range pb.Events {
		want := g
		if i%2 == 1 {
			want = a
		}
		if ev.MIDI != want {
			t.Fatalf("note %d of the trill sounds %d, want %d", i, ev.MIDI, want)
		}
	}
	// And it lasts as long as the note it ornaments, not longer.
	last := pb.Events[len(pb.Events)-1]
	if end := last.Start + last.Dur; end < 3.99 || end > 4.01 {
		t.Errorf("the trill ends at %.3fs, want the note's own 4s", end)
	}
}

// A trill in a flat key trills by whatever step the key has there.
func TestATrillFollowsTheKey(t *testing.T) {
	s := trilledScore()
	// B flat major: the note above G is A, still natural here.
	s.Parts[0].Key = model.KeySignature{Fifths: -2, Mode: model.Major}
	pb := buildPlayback(s)
	a := model.Pitch{Step: model.StepA, Octave: 4}.MIDI()
	if pb.Events[1].MIDI != a {
		t.Errorf("the upper note sounds %d, want %d", pb.Events[1].MIDI, a)
	}

	// In D flat major the A is flattened, and the trill follows it.
	s.Parts[0].Key = model.KeySignature{Fifths: -5, Mode: model.Major}
	pb = buildPlayback(s)
	if want := a - 1; pb.Events[1].MIDI != want {
		t.Errorf("in five flats the upper note sounds %d, want %d",
			pb.Events[1].MIDI, want)
	}
}
