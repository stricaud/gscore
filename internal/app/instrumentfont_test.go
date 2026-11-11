//go:build !js

/*
 * instrumentfont_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which soundfont each staff of a score is played through.
 *
 * Three answers, and the nearest one wins: what this export was told about
 * this staff, what has been said about the instrument it is playing, and
 * failing both the one chosen for the whole score. The middle one is for
 * somebody who owns a real violin and a General MIDI bank for everything
 * else, and who would otherwise set that on every staff of every score.
 *
 * And a download that is on its way in says how far it has got, because a
 * sentence reading "downloading" that has not changed in four minutes is
 * indistinguishable from a program that has stopped.
 */

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// scoreOf makes a score whose staves play these instruments, by sound id.
func scoreOf(sounds ...string) *model.Score {
	s := &model.Score{}
	for _, id := range sounds {
		s.Parts = append(s.Parts, &model.Part{Sound: id, Clef: model.Treble})
	}
	return s
}

func TestAStaffFollowsItsInstrumentsSoundfont(t *testing.T) {
	withSettings(t, func() {
		violin := bank(t, "violin.sf2")
		a := &App{score: scoreOf("strings.violin", "wind.flutes.flute")}

		s := loadSettings()
		s.InstrumentFonts = map[string]string{"strings.violin": violin}
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}

		got := a.partFonts(nil, "/sounds/general.sf2")
		if got[0] != violin {
			t.Errorf("the violin plays through %q, want the one chosen for "+
				"violins", got[0])
		}
		// The flute was never spoken for, so it follows the score's own
		// soundfont — which is said by leaving it out.
		if path, ok := got[1]; ok {
			t.Errorf("the flute was put on %q, and nothing was said about "+
				"flutes", path)
		}
	})
}

// What the export was told about this staff beats what was said about the
// instrument: the dialog is in front of somebody making a decision now.
func TestWhatTheExportSaysAboutAStaffWins(t *testing.T) {
	withSettings(t, func() {
		standing, justNow := bank(t, "standing.sf2"), bank(t, "justnow.sf2")
		a := &App{score: scoreOf("strings.violin")}

		s := loadSettings()
		s.InstrumentFonts = map[string]string{"strings.violin": standing}
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}

		got := a.partFonts(map[int]string{0: justNow}, "/sounds/general.sf2")
		if got[0] != justNow {
			t.Errorf("the staff plays through %q, want the one this export "+
				"was told", got[0])
		}
	})
}

// A library chosen a year ago and since moved would take a staff silent,
// and silent in a way nothing on the screen explains. It falls back to the
// chosen soundfont, which at least plays something.
func TestAnInstrumentsSoundfontThatHasGoneIsNotUsed(t *testing.T) {
	withSettings(t, func() {
		a := &App{score: scoreOf("strings.violin")}
		s := loadSettings()
		s.InstrumentFonts = map[string]string{
			"strings.violin": "/sounds/moved-last-year.sf2"}
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}
		if path, ok := a.partFonts(nil, "/sounds/general.sf2")[0]; ok {
			t.Errorf("the staff was put on %q, which is not there", path)
		}
	})
}

// Saying the same thing twice is saying nothing: a staff whose instrument
// points at the soundfont already chosen is left to follow it.
func TestAnInstrumentOnTheChosenSoundfontIsNotSingledOut(t *testing.T) {
	withSettings(t, func() {
		chosen := bank(t, "chosen.sf2")
		a := &App{score: scoreOf("strings.violin")}
		s := loadSettings()
		s.InstrumentFonts = map[string]string{"strings.violin": chosen}
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}
		if _, ok := a.partFonts(nil, chosen)[0]; ok {
			t.Error("the staff was singled out onto the soundfont it was " +
				"already going to play through")
		}
	})
}

func TestAnInstrumentsSoundfontIsSetAndTakenAwayOneAtATime(t *testing.T) {
	withSettings(t, func() {
		h := newHarness(t)
		h.post("/api/settings?instrumentFont=strings.violin=/a.sf2" +
			"&instrumentFont=wind.flutes.flute=/b.sf2")

		read := func() map[string]any {
			var out struct {
				InstrumentFonts map[string]any `json:"instrumentFonts"`
			}
			if err := json.Unmarshal([]byte(h.get("/api/settings")), &out); err != nil {
				t.Fatal(err)
			}
			return out.InstrumentFonts
		}
		if got := read(); len(got) != 2 {
			t.Fatalf("%d instruments were remembered: %v", len(got), got)
		}
		// Nothing after the "=" takes one off, and leaves the others.
		h.post("/api/settings?instrumentFont=strings.violin=")
		got := read()
		if _, still := got["strings.violin"]; still {
			t.Error("the violin was not taken off again")
		}
		if _, gone := got["wind.flutes.flute"]; !gone {
			t.Error("taking the violin off took the flute with it")
		}
		// And a value that is not sound=path is a mistake worth hearing
		// about rather than one to store.
		if msg := h.postFails("/api/settings?instrumentFont=nonsense"); msg == "" {
			t.Error("a malformed instrument soundfont was accepted")
		}
	})
}

/*
 * How far a download has got
 */

func TestADownloadNobodyStartedIsNotAnError(t *testing.T) {
	h := newHarness(t)
	var d map[string]any
	if err := json.Unmarshal(
		[]byte(h.get("/api/soundfont/progress?job=never-started")), &d); err != nil {
		t.Fatal(err)
	}
	// The page may well ask before the download has been registered, and a
	// refusal there is a progress bar that flashes a failure on its way up.
	if d["done"] != false {
		t.Errorf("a job nobody started came back as %v", d["done"])
	}
	if d["received"] != float64(0) {
		t.Errorf("it claims %v bytes have arrived", d["received"])
	}
}

func TestADownloadSaysHowFarItHasGot(t *testing.T) {
	d := watchDownload("a-job")
	d.mu.Lock()
	d.total = 1000
	d.mu.Unlock()
	d.add(250)

	h := newHarness(t)
	var out map[string]any
	if err := json.Unmarshal(
		[]byte(h.get("/api/soundfont/progress?job=a-job")), &out); err != nil {
		t.Fatal(err)
	}
	if out["received"] != float64(250) || out["total"] != float64(1000) {
		t.Errorf("it says %v of %v", out["received"], out["total"])
	}
	if out["done"] != false {
		t.Error("it says it has finished")
	}

	// And when it goes wrong, what went wrong — which is the one thing a
	// bar cannot show.
	d.finish(os.ErrPermission)
	if err := json.Unmarshal(
		[]byte(h.get("/api/soundfont/progress?job=a-job")), &out); err != nil {
		t.Fatal(err)
	}
	if out["done"] != true {
		t.Error("a finished download still says it is going")
	}
	if out["error"] == nil {
		t.Error("it failed and says nothing about why")
	}
}

// The clock stops when the download does, so the speed beside a finished
// one is the speed it ran at rather than a number falling away to nothing.
func TestTheSpeedIsMeasuredOverTheDownload(t *testing.T) {
	d := watchDownload("done-job")
	d.add(1000)
	d.finish(nil)
	_, _, _, _, first := d.state()
	_, _, _, _, second := d.state()
	if second != first {
		t.Errorf("the elapsed time grew from %v to %v after it finished",
			first, second)
	}
}

// A finished download is forgotten eventually, and not while it is still
// being asked about.
func TestAFinishedDownloadIsForgottenLater(t *testing.T) {
	d := watchDownload("old-job")
	d.finish(nil)
	if d.over(downloadKept) {
		t.Error("a download that has only just finished is already forgotten")
	}
	if !d.over(0) {
		t.Error("a finished download is never forgotten")
	}
	// One still going is not forgotten whatever the age.
	if watchDownload("running").over(0) {
		t.Error("a download still running was forgotten")
	}
}

// bank's folder is cleaned up by the test framework, so nothing here is
// left on the disk. Named here because the other test file makes one too.
var _ = filepath.Join
