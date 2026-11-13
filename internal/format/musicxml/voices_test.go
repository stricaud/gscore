/*
 * voices_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The second voice, which was read and never written.
 *
 * MusicXML puts two lines on one staff by writing the bar through once per
 * voice, with <backup> winding the position back to the start of the bar in
 * between. The importer has always honoured it. The writer wrote the first
 * voice and stopped — so a piano part whose right hand holds a note under a
 * moving line came out of gscore with the held note simply gone. No warning,
 * nothing in the file to say anything had been there, and the score still
 * looked right in gscore. That is the kind of loss nobody notices until they
 * open the file somewhere else.
 */

package musicxml

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// twoVoices is one staff whose bar holds four quarter notes over a held
// semibreve: the commonest two-voice shape there is.
func twoVoices() *model.Score {
	s := model.NewScore()
	s.Parts = nil
	p := &model.Part{
		Name: "Piano", Clef: model.Treble,
		Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	var upper []model.Element
	for i := 0; i < 4; i++ {
		upper = append(upper, &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 5},
			Dur:   model.Duration{Num: 1, Den: 4},
		})
	}
	lower := []model.Element{&model.Note{
		Pitch: model.Pitch{Step: model.StepD, Octave: 5},
		Dur:   model.Duration{Num: 1, Den: 1},
	}}
	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: upper}, {Elements: lower}},
		RightBarline: model.BarSingle,
	})
	s.Parts = append(s.Parts, p)
	s.Normalise()
	return s
}

var backupRe = regexp.MustCompile(`<backup>\s*<duration>(\d+)</duration>`)

func TestBothVoicesAreWritten(t *testing.T) {
	out := exported(t, twoVoices())

	if n := strings.Count(out, "<note>"); n != 5 {
		t.Errorf("%d notes in the file, wanted the 5 that were written", n)
	}
	if !strings.Contains(out, "<voice>1</voice>") {
		t.Error("the first voice is not numbered")
	}
	if !strings.Contains(out, "<voice>2</voice>") {
		t.Error("the second voice is not in the file at all")
	}
	// Wound back to the start of the bar, and no further. Four quarters at
	// four divisions to the quarter is sixteen.
	m := backupRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no <backup> between the voices:\n%s", out)
	}
	if m[1] != "16" {
		t.Errorf("the backup is %s divisions; the first voice wrote 16", m[1])
	}
}

func TestBothVoicesComeBack(t *testing.T) {
	back := reimported(t, exported(t, twoVoices()))
	bar := back.Parts[0].Measures[0]
	if len(bar.Voices) != 2 {
		t.Fatalf("the bar came back with %d voices", len(bar.Voices))
	}
	if n := len(bar.Voices[0].Elements); n != 4 {
		t.Errorf("the upper voice came back with %d notes, wanted 4", n)
	}
	if n := len(bar.Voices[1].Elements); n != 1 {
		t.Fatalf("the lower voice came back with %d notes, wanted 1", n)
	}
	// And it is still the whole bar long, which is the thing the backup
	// exists to get right: read without one, the held note is appended
	// after the four quarters and the bar holds twice the music.
	whole := model.Duration{Num: 1, Den: 1}
	if got := bar.Voices[1].Duration(); got.Cmp(whole) != 0 {
		t.Errorf("the held note came back as %v, wanted a whole bar", got)
	}
	if got := bar.Voices[0].Duration(); got.Cmp(whole) != 0 {
		t.Errorf("the upper voice came back as %v, wanted a whole bar", got)
	}
}

func TestOneVoiceIsStillNumbered(t *testing.T) {
	// Every note carries its voice, the first voice's included. A file
	// that numbers some of its notes and not others leaves the reader to
	// guess which voice the unnumbered ones belong to — and there is no
	// <backup> to write when there is only one line.
	s := twoVoices()
	s.Parts[0].Measures[0].Voices = s.Parts[0].Measures[0].Voices[:1]
	out := exported(t, s)

	if n := strings.Count(out, "<voice>1</voice>"); n != 4 {
		t.Errorf("%d of the 4 notes say which voice they are", n)
	}
	if backupRe.MatchString(out) {
		t.Error("a bar of one voice was written with a backup in it")
	}
}

func TestAnEmptyVoiceWritesNoBackup(t *testing.T) {
	// A voice slot with nothing in it is not a line of music. Writing a
	// backup for it would wind the bar back and then write nothing,
	// which some readers take as a bar of silence in a third voice.
	s := twoVoices()
	bar := s.Parts[0].Measures[0]
	bar.Voices = append(bar.Voices, &model.Voice{})
	out := exported(t, s)

	if n := len(backupRe.FindAllString(out, -1)); n != 1 {
		t.Errorf("%d backups written, wanted the 1 between the two real voices", n)
	}
	if strings.Contains(out, "<voice>3</voice>") {
		t.Error("the empty voice was written out")
	}
}
