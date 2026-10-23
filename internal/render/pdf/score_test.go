package pdf

import (
	"bytes"
	"os"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// A score built from nothing, using every mark a lead sheet needs, rendered
// to a PDF.
//
// This is the test that would have caught most of what went wrong while these
// features were being written: a chord symbol drawn over the staff above, a
// staff name through its own staff lines, a brace that did not reach, a page
// break through the middle of a system. None of those show up in a unit test
// of the thing that drew them — they only show up on a page.
//
// It checks what can be checked without eyes: that it renders, that it is a
// PDF, that it has the pages it should and that nothing has been left out of
// the drawing.
func TestFullScoreToPDF(t *testing.T) {
	s := buildDemo()

	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.LineWidth = 76
	eopt.ShowPartNames = true
	eopt.ShowMeasureNumbers = true
	lay := engrave.Engrave(s, eopt)

	if len(lay.Systems) == 0 {
		t.Fatal("the layout recorded no systems, so it cannot be paginated")
	}
	if len(lay.Glyphs) == 0 || len(lay.Texts) == 0 || len(lay.Lines) == 0 {
		t.Fatalf("thin layout: %d glyphs, %d texts, %d lines",
			len(lay.Glyphs), len(lay.Texts), len(lay.Lines))
	}

	// Everything asked for has to be on the page somewhere.
	texts := map[string]bool{}
	for _, x := range lay.Texts {
		texts[x.S] = true
	}
	for _, want := range []string{"Piano", "Bass", "Drums", "A", "Moderately", "rit."} {
		if !texts[want] {
			t.Errorf("%q was not drawn", want)
		}
	}

	var buf bytes.Buffer
	opt := DefaultOptions()
	opt.Title = s.Identity.Title
	opt.Author = s.Identity.Composer
	if err := Render(&buf, lay, opt); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	if !bytes.HasPrefix(out, []byte("%PDF-1.")) {
		t.Fatal("that is not a PDF")
	}
	if !bytes.HasSuffix(bytes.TrimSpace(out), []byte("%%EOF")) {
		t.Fatal("the PDF has no end marker")
	}
	if n := bytes.Count(out, []byte("/Type /Page ")); n < 1 {
		t.Fatalf("got %d pages", n)
	}
	// The music font has to be in the file, or it prints as empty boxes on
	// any machine that does not happen to have Bravura installed.
	if !bytes.Contains(out, []byte("/FontFile3")) {
		t.Error("the music font was not embedded")
	}
	if !bytes.Contains(out, []byte("/Bravura")) {
		t.Error("the embedded font is not the one the score was drawn with")
	}

	if p := os.Getenv("PDF_OUT"); p != "" {
		if err := os.WriteFile(p, out, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s, %d bytes, %d systems", p, len(out), len(lay.Systems))
	}
}

// buildDemo writes a short original piece using the marks a lead sheet and a
// trio chart need. The music is nothing special; the point is the furniture
// around it.
func buildDemo() *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	e := model.Duration{Num: 1, Den: 8}

	note := func(step model.Step, oct int, d model.Duration) *model.Note {
		return &model.Note{Pitch: model.Pitch{Step: step, Octave: oct}, Dur: d}
	}

	piano := &model.Part{
		Name: "Piano", Instrument: model.AcousticGrandPiano,
		Clef: model.Treble,
		Key:  model.KeySignature{Fifths: 1, Mode: model.Minor},
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	bass := &model.Part{
		Name: "Bass", Instrument: model.AcousticBass,
		Clef: model.Bass,
		Key:  model.KeySignature{Fifths: 1, Mode: model.Minor},
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	drums := &model.Part{
		Name: "Drums", Instrument: model.AcousticGrandPiano,
		Clef: model.Percussion,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}

	chords := []struct {
		root, quality string
		frame         string
	}{
		{"E", "m", "022000"},
		{"A", "m7", "x02010"},
		{"B", "7", "x21202"},
		{"E", "m", "022000"},
		{"C", "maj7", "x32000"},
		{"B", "7sus4", "x22200"},
		{"E", "m", "022000"},
		{"E", "m", "022000"},
	}

	for bar := 0; bar < 8; bar++ {
		// Piano: a phrase that climbs and comes back down.
		steps := []model.Step{model.StepE, model.StepG, model.StepB, model.StepA}
		var els []model.Element
		for i, st := range steps {
			n := note(st, 4+i/3, e)
			if i == 0 {
				c := chords[bar]
				n.Chord = &model.ChordSymbol{
					Root: c.root, Quality: c.quality,
					Frame: frameOf(c.frame),
				}
			}
			els = append(els, n)
		}
		// A held note to finish the bar.
		tail := note(model.StepB, 4, q)
		if bar == 0 {
			tail.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}
		}
		if bar == 1 {
			tail.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
			tail.Dynamic = model.Forte
		}
		if bar == 6 {
			tail.Texts = []model.TextMark{{Text: "rit.", Italic: true}}
		}
		els = append(els, tail, &model.Rest{Dur: q})

		m := &model.Measure{Voices: []*model.Voice{{Elements: els}}}
		if bar == 0 {
			m.Marker = "A"
		}
		if bar == 4 {
			m.Marker = "B"
			k := model.KeySignature{Fifths: -2, Mode: model.Major}
			m.Key = &k
		}
		if bar == 6 {
			m.Ending = &model.Ending{Numbers: []int{1}, Start: true, Stop: true}
			m.RightBarline = model.BarRepeatEnd
		}
		if bar == 7 {
			m.Ending = &model.Ending{Numbers: []int{2}, Start: true, Stop: true, Open: true}
			m.RightBarline = model.BarFinal
		}
		piano.Measures = append(piano.Measures, m)

		// Bass: roots, with the words of a line underneath to prove lyrics
		// sit where they should.
		b1 := note(model.StepE, 2, q)
		b2 := note(model.StepB, 2, q)
		b3 := note(model.StepE, 3, q)
		b4 := note(model.StepB, 2, q)
		if bar == 0 {
			b1.Lyrics = []model.Lyric{{Text: "Walk", Verse: 1}}
			b2.Lyrics = []model.Lyric{{Text: "ing", Verse: 1}}
			b3.Lyrics = []model.Lyric{{Text: "the", Verse: 1}}
			b4.Lyrics = []model.Lyric{{Text: "line", Verse: 1}}
		}
		bm := &model.Measure{Voices: []*model.Voice{{
			Elements: []model.Element{b1, b2, b3, b4},
		}}}
		bm.RightBarline = piano.Measures[bar].RightBarline
		bass.Measures = append(bass.Measures, bm)

		// Drums: hi-hat, snare, kick — the standard pattern.
		drumNote := func(step int, d model.Duration) *model.Note {
			k, _ := model.PercussionFor(step)
			return &model.Note{
				Pitch: model.PitchFromDiatonicIndex(k.Step + 10),
				Dur:   d, Head: k.Head, Percussion: k.Key,
			}
		}
		dm := &model.Measure{Voices: []*model.Voice{{
			Elements: []model.Element{
				drumNote(1, q), drumNote(5, q), drumNote(1, q), drumNote(5, q),
			},
		}}}
		dm.RightBarline = piano.Measures[bar].RightBarline
		drums.Measures = append(drums.Measures, dm)
	}

	// The style direction over the first bar.
	first := piano.Measures[0].Voices[0].Elements[0].(*model.Note)
	first.Texts = append(first.Texts, model.TextMark{Text: "Moderately", Bold: true})

	s := &model.Score{
		Identity: model.Identity{
			Title:    "Walking the Line",
			Subtitle: "a trio chart",
			Composer: "music by nobody in particular",
		},
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{piano, bass, drums},
		Groups: []model.StaffGroup{
			{First: 0, Last: 1, Symbol: model.GroupBracket, Name: ""},
		},
	}
	return s
}

func frameOf(spec string) *model.ChordFrame {
	f := &model.ChordFrame{FirstFret: 1}
	for _, c := range spec {
		switch {
		case c == 'x':
			f.Frets = append(f.Frets, -1)
		case c >= '0' && c <= '9':
			f.Frets = append(f.Frets, int(c-'0'))
		}
	}
	return f
}
