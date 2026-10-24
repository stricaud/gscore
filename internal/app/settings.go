/*
 * settings.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package app

// What gscore remembers between sessions.
//
// Almost nothing, and on purpose: a score carries its own settings and the
// program should not have opinions that outlive it. The exception is the
// instruments, because a soundfont is a file somewhere on this machine that
// the person went and found, and asking them which one every time they press
// play would be asking them to answer a question they have already answered.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// settings is everything kept between sessions.
type settings struct {
	// PlaybackFont is the soundfont the editor plays through, by path.
	// Empty means whichever one looks best among those installed.
	PlaybackFont string `json:"playbackFont,omitempty"`
}

var stored struct {
	sync.Mutex
	settings settings
	read     bool
}

func settingsPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Gscore", "settings.json"), nil
}

// loadSettings reads what was kept, once, and never fails: a settings file
// that cannot be read is a settings file that has not been written yet, and
// every value in it has an answer for that case.
func loadSettings() settings {
	stored.Lock()
	defer stored.Unlock()
	if stored.read {
		return stored.settings
	}
	stored.read = true
	path, err := settingsPath()
	if err != nil {
		return stored.settings
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return stored.settings
	}
	_ = json.Unmarshal(b, &stored.settings)
	return stored.settings
}

func saveSettings(s settings) error {
	stored.Lock()
	stored.settings, stored.read = s, true
	stored.Unlock()

	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// playbackFont is the soundfont to play through.
//
// Whatever was chosen, if it is still there. Failing that the best one
// installed, preferring FluidR3 — the bank FluidSynth plays MIDI files with,
// and the fullest of the General MIDI sets — then GeneralUser GS, which is
// the one the application is shipped with. A soundfont of one instrument is
// a poor guess for a whole score and is only reached if it is all there is.
func (a *App) playbackFont() string {
	if chosen := loadSettings().PlaybackFont; chosen != "" {
		if _, err := os.Stat(chosen); err == nil {
			return chosen
		}
	}
	installed := listSoundfonts()
	for _, want := range []string{"fluidr3", "generaluser"} {
		for _, f := range installed {
			name := strings.ToLower(f.Name + " " + filepath.Base(f.Path))
			if strings.Contains(strings.ReplaceAll(name, " ", ""), want) {
				return f.Path
			}
		}
	}
	if len(installed) > 0 {
		return installed[0].Path
	}
	return ""
}

// handleSettings reads and writes what is kept.
func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s := loadSettings()
		if v, ok := r.URL.Query()["playbackFont"]; ok && len(v) > 0 {
			s.PlaybackFont = v[0]
		}
		if err := saveSettings(s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	chosen := a.playbackFont()
	name := ""
	if chosen != "" {
		if f, err := loadFont(chosen); err == nil {
			name = f.Name
		}
	}
	writeJSON(w, map[string]any{
		"playbackFont":     chosen,
		"playbackFontName": name,
		"stored":           loadSettings().PlaybackFont,
	})
}
