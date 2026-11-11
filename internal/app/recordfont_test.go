//go:build !js

/*
 * recordfont_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The soundfont a recording is rendered through.
 *
 * Not always the one the editor plays through. Playing is answered while
 * you are typing, so a light bank is worth having there; a recording is
 * rendered once and listened to by somebody else, so the two hundred
 * megabyte one earns its download. Choosing the big one for an export
 * should not make every keystroke load it, and choosing it should not have
 * to be done again on the next export.
 */

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withSettings runs f against a settings file of its own, so that a test
// never reads or writes the one belonging to whoever is running it.
func withSettings(t *testing.T, f func()) {
	t.Helper()
	dir := t.TempDir()
	// Both, because the two systems find it in different places: Linux
	// reads XDG_CONFIG_HOME and macOS reads Library under HOME.
	t.Setenv("XDG_CONFIG_HOME", dir)
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	// The settings are read once and kept, so the cache has to be emptied
	// around this or the test reads whoever is running it.
	forget := func() {
		stored.Lock()
		stored.settings, stored.read = settings{}, false
		stored.Unlock()
	}
	forget()
	defer forget()
	f()
}

// bank makes a file standing in for a soundfont. It only has to exist:
// playbackFont and recordFont ask whether the chosen one is still on the
// disk, and fall back to whatever is installed when it is not — which on
// the machine running the test is a real soundfont and not the fixture.
func bank(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("stands in for a soundfont"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestARecordingUsesWhatThePlaybackUsesUntilToldOtherwise(t *testing.T) {
	withSettings(t, func() {
		a := &App{}
		light := bank(t, "light.sf2")
		s := loadSettings()
		s.PlaybackFont = light
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}
		// Nobody has chosen one for recording, so it is the one the editor
		// plays through — the right answer for anybody who has never
		// thought about the question.
		if got := a.recordFont(); got != light {
			t.Errorf("a recording would use %q, want the playback one", got)
		}
	})
}

func TestARecordingKeepsItsOwnChoice(t *testing.T) {
	withSettings(t, func() {
		a := &App{}
		light, big := bank(t, "light.sf2"), bank(t, "big.sf2")
		s := loadSettings()
		s.PlaybackFont = light
		s.RecordFont = big
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}

		if got := a.recordFont(); got != big {
			t.Errorf("a recording would use %q, want the one chosen for it", got)
		}
		// And the editor goes on playing through the light one: choosing
		// the big bank for an export is not choosing it for typing.
		if got := a.playbackFont(); got != light {
			t.Errorf("the editor would play through %q", got)
		}
	})
}

// A soundfont is a file, and a file chosen months ago may have been deleted
// or moved. The choice is remembered either way; what is used falls back.
func TestAChoiceThatHasGoneFallsBack(t *testing.T) {
	withSettings(t, func() {
		a := &App{}
		light := bank(t, "light.sf2")
		s := loadSettings()
		s.PlaybackFont = light
		s.RecordFont = "/sounds/deleted-last-year.sf2"
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}

		if got := a.recordFont(); got != light {
			t.Errorf("a recording would use %q, want it to fall back", got)
		}
		// Remembered all the same: the file may come back, and forgetting
		// what somebody asked for is not the same as not finding it.
		if got := loadSettings().RecordFont; got == "" {
			t.Error("the choice was forgotten as well as unusable")
		}
	})
}

// The editor reads this to open its dialog on the right one, so both have
// to be in the answer.
func TestTheSettingsSayWhatARecordingWillUse(t *testing.T) {
	withSettings(t, func() {
		s := loadSettings()
		s.PlaybackFont = "/sounds/light.sf2"
		s.RecordFont = "/sounds/big.sf2"
		if err := saveSettings(s); err != nil {
			t.Fatal(err)
		}

		h := newHarness(t)
		var out map[string]any
		if err := json.Unmarshal([]byte(h.get("/api/settings")), &out); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"playbackFont", "recordFont", "recordFontStored"} {
			if _, ok := out[k]; !ok {
				t.Errorf("the settings say nothing about %s", k)
			}
		}
		if got := out["recordFontStored"]; got != "/sounds/big.sf2" {
			t.Errorf("the stored choice came back as %v", got)
		}
	})
}
