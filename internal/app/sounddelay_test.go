//go:build !js

/*
 * sounddelay_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How far behind the picture the sound is, as measured by the person
 * listening.
 *
 * The cursor is meant to sit on the note being heard, so it is held back
 * by however long the sound takes to get out of the machine. A browser
 * that implements outputLatency answers that and this is not needed;
 * WebKit — the Mac application, and every browser on a Mac — offers only
 * baseLatency, which counts the part of the journey inside the browser and
 * knows nothing about the Bluetooth headphones on the other end of it.
 * Nothing in the program can measure that. The person can.
 */

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// lag asks the settings route what the sound delay is.
func lag(t *testing.T, a *App) int {
	t.Helper()
	rec := httptest.NewRecorder()
	a.handleSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	var got struct {
		PlayLag int `json:"playLag"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("the settings answered %q: %v", rec.Body.String(), err)
	}
	return got.PlayLag
}

// setLag sends one, and answers with what the server made of it.
func setLag(t *testing.T, a *App, v string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	a.handleSettings(rec,
		httptest.NewRequest(http.MethodPost, "/api/settings?playLag="+v, nil))
	return rec.Code
}

func TestNobodyHasMeasuredTheSoundDelay(t *testing.T) {
	withSettings(t, func() {
		a := &App{}
		// Nought, which is "believe the browser": the correction is for
		// what the browser cannot see, and on a browser that can see all
		// of it there is nothing to correct.
		if got := lag(t, a); got != 0 {
			t.Errorf("the sound delay starts at %d ms, want 0", got)
		}
	})
}

func TestTheSoundDelayIsKept(t *testing.T) {
	withSettings(t, func() {
		a := &App{}
		if code := setLag(t, a, "180"); code != http.StatusOK {
			t.Fatalf("setting the delay answered %d", code)
		}
		if got := lag(t, a); got != 180 {
			t.Errorf("the sound delay is %d ms, want 180", got)
		}
		// And it is on the disk, not merely in this process: the whole
		// point is that it is measured once.
		if got := loadSettings().PlayLag; got != 180 {
			t.Errorf("the settings file holds %d ms, want 180", got)
		}
	})
}

func TestTheCorrectionCanGoTheOtherWay(t *testing.T) {
	// A browser that over-reports its own latency leaves the cursor behind
	// the sound, and somebody has to be able to pull it back.
	withSettings(t, func() {
		a := &App{}
		if code := setLag(t, a, "-60"); code != http.StatusOK {
			t.Fatalf("a negative delay answered %d", code)
		}
		if got := lag(t, a); got != -60 {
			t.Errorf("the sound delay is %d ms, want -60", got)
		}
	})
}

func TestASoundDelayNobodyCouldHaveMeasuredIsRefused(t *testing.T) {
	withSettings(t, func() {
		a := &App{}
		if code := setLag(t, a, "250"); code != http.StatusOK {
			t.Fatalf("a quarter of a second answered %d, which is ordinary "+
				"for headphones", code)
		}
		for _, bad := range []string{"4000", "-5000", "soon", "", "1.5"} {
			if code := setLag(t, a, bad); code != http.StatusBadRequest {
				t.Errorf("a delay of %q answered %d, want it refused",
					bad, code)
			}
		}
		// And a refused one changes nothing: the last good answer stands.
		if got := lag(t, a); got != 250 {
			t.Errorf("the sound delay became %d ms after refusals, want 250",
				got)
		}
	})
}

func TestASettingsFileSomebodyEditedBadlyIsIgnored(t *testing.T) {
	// The file is the person's and they may write anything in it. A delay
	// of a minute is not a headphone, and taking it would stop the cursor
	// for a minute with no way to tell why.
	withSettings(t, func() {
		s := loadSettings()
		s.PlayLag = 60000
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}
		if got := PlayLag(); got != 0 {
			t.Errorf("a delay of a minute was read as %d, want it ignored", got)
		}
	})
}
