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
	"path/filepath"
	"strings"
	"sync"
)

// settings is everything kept between sessions.
type settings struct {
	// PlaybackFont is the soundfont the editor plays through, by path.
	// Empty means whichever one looks best among those installed.
	PlaybackFont string `json:"playbackFont,omitempty"`

	// Recent is the scores last opened or saved, newest first.
	//
	// Paths and not contents: the file on the disk is the score, and a
	// list that remembered anything about it would be a second copy to go
	// stale. A path that has since been moved or deleted is dropped from
	// the list when the list is read, which is the only moment it matters.
	Recent []string `json:"recent,omitempty"`
}

// mostRecent is how many files the list holds.
//
// Ten is what every application settles on: long enough to cover the thing
// you were working on last week, short enough to read without looking.
const mostRecent = 10

var stored struct {
	sync.Mutex
	settings settings
	read     bool
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
	b, err := readSettings()
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

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeSettings(append(b, '\n'))
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
		// Loaded into this process, or still where it was on the disk.
		if f, err := loadFont(chosen); err == nil && f != nil {
			return chosen
		}
		if fileExists(chosen) {
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

// remember puts a file at the top of the recent list.
//
// Called where a score is opened or saved and nowhere else: a PDF or an
// audio file that came out of gscore is not a score to be reopened, and a
// list that offered to open one would be offering to fail.
func (a *App) remember(path string) {
	if path == "" || !a.local {
		// Served over a network there is no "this machine" to have a
		// recent file on, and the paths would be the server's.
		return
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	s := loadSettings()
	out := []string{path}
	for _, old := range s.Recent {
		if old != path && len(out) < mostRecent {
			out = append(out, old)
		}
	}
	s.Recent = out
	_ = saveSettings(s)
}

// recentFiles is the list as it stands, with whatever has gone since
// dropped.
//
// Checked at the moment of asking rather than kept true: a file lives on a
// disk that gscore does not own, and the only honest time to ask whether it
// is still there is when somebody is about to be offered it.
func (a *App) recentFiles() []recentFile {
	if !a.local {
		return nil
	}
	s := loadSettings()
	out := make([]recentFile, 0, len(s.Recent))
	kept := make([]string, 0, len(s.Recent))
	for _, path := range s.Recent {
		if !fileExists(path) {
			continue // the same question: is this file still there
		}
		kept = append(kept, path)
		out = append(out, recentFile{Path: path, Name: filepath.Base(path)})
	}
	if len(kept) != len(s.Recent) {
		s.Recent = kept
		_ = saveSettings(s)
	}
	return out
}

// recentFile is one entry: where it is, and what to call it in a menu.
type recentFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// handleRecent lists the files last opened, and empties the list.
func (a *App) handleRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Query().Get("clear") == "1" {
		s := loadSettings()
		s.Recent = nil
		_ = saveSettings(s)
	}
	writeJSON(w, map[string]any{"files": a.recentFiles()})
}

// handleSettings reads and writes what is kept.
func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s := loadSettings()
		if v, ok := r.URL.Query()["playbackFont"]; ok && len(v) > 0 {
			s.PlaybackFont = v[0]
		}
		// A failure to write is not a failure to set: saveSettings has
		// already kept it for this session, and in the browser there is no
		// disk to keep it on at all. Reporting that as an error would be an
		// error message about something nobody asked for.
		_ = saveSettings(s)
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
