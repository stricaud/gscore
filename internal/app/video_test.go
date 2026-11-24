//go:build !js

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// The cursor glides from one note to the next, and jumps where a repeat
// goes back, rather than sliding back across the music it has played.
func TestTheFilmsCursorGlidesAndJumpsBack(t *testing.T) {
	f := &film{path: []cursorPoint{{0, 10}, {1, 20}, {2, 30}, {3, 10}, {4, 20}}}
	for _, c := range []struct{ t, want float64 }{
		{0, 10}, {0.5, 15}, {1.5, 25}, {2.5, 30}, {3.5, 15}, {9, 20},
	} {
		if got := f.cursorAt(c.t); got != c.want {
			t.Errorf("at %.1fs the cursor is at %.1f, want %.1f", c.t, got, c.want)
		}
	}
}

// How far a film has got is said as a percentage of its frames.
func TestTheFilmSaysHowFarItHasGot(t *testing.T) {
	var p progress
	p.set("sound", 0, 1)
	if ph, pct := p.percent(); ph != "sound" || pct != 0 {
		t.Errorf("making the sound: %s %d", ph, pct)
	}
	p.set("frames", 300, 1200)
	if _, pct := p.percent(); pct != 25 {
		t.Errorf("a quarter of the frames is %d%%", pct)
	}
	p.set("frames", 1200, 1200)
	if _, pct := p.percent(); pct != 99 {
		t.Errorf("all the frames, before the film is finished, is %d%%", pct)
	}
}

// The window can ask whether a film can be made, and is told what to
// install where it cannot.
func TestTheWindowCanAskWhetherAVideoCanBeMade(t *testing.T) {
	h := newHarness(t)
	var d struct {
		Missing []string `json:"missing"`
		Install string   `json:"install"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/videotools")), &d); err != nil {
		t.Fatal(err)
	}
	if d.Missing == nil {
		t.Error("missing is not a list")
	}
	if len(d.Missing) > 0 && d.Install == "" {
		t.Error("something is missing and nothing says how to install it")
	}
}

// The keys sounding are pressed, in their staff's colour.
func TestTheFilmsPianoPressesTheKeysSounding(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1040, 100))
	drawKeys(img, img.Bounds(), map[int]int{60: 1})
	// Middle C is the 24th white key from A0: at 23.5 keys of 20 pixels.
	if c := img.RGBAAt(23*20+10, 90); c != videoHands[1] {
		t.Errorf("middle C is %v, want the second staff's colour", c)
	}
	if c := img.RGBAAt(22*20+10, 90); c != videoWhite {
		t.Errorf("the B below it is %v, want an unpressed key", c)
	}
}

// A whole film, where the machine can make one: a short one, of a bar.
func TestAFilmIsMade(t *testing.T) {
	_, _, missing := videoTools()
	font := os.Getenv("GSCORE_TEST_SOUNDFONT")
	if len(missing) > 0 || font == "" {
		t.Skip("needs ffmpeg, rsvg-convert and GSCORE_TEST_SOUNDFONT")
	}
	s := model.NewScore()
	q := model.Duration{Num: 1, Den: 4}
	var els []model.Element
	for _, st := range []model.Step{model.StepC, model.StepE, model.StepG, model.StepC} {
		els = append(els, &model.Note{Pitch: model.Pitch{Step: st, Octave: 4}, Dur: q})
	}
	s.Parts[0].Measures = []*model.Measure{{Voices: []*model.Voice{{Elements: els}}}}
	var buf bytes.Buffer
	if err := recordVideo(context.Background(), &buf, s,
		recording{Font: font, SampleRate: 44100}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "film.mp4")
	os.WriteFile(out, buf.Bytes(), 0o600)
	probe, err := exec.Command("ffprobe", "-v", "error", "-show_entries",
		"stream=codec_name", "-of", "csv=p=0", out).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(probe, []byte("h264")) || !bytes.Contains(probe, []byte("aac")) {
		t.Errorf("the film is not H.264 with AAC sound: %s", probe)
	}
}
