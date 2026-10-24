/*
 * soundfont.go
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

// Where the recorded instruments come from.
//
// A soundfont is a big file of somebody else's recordings and gscore does not
// carry one: the program is a few megabytes and a good orchestra is thirty,
// which is a strange thing to make every user download in order to write a
// tune they will play themselves. So it is fetched once, when somebody asks
// for a recording, and kept.
//
// Three ways in, because people arrive with different things already true of
// them: a file they already have, a link they were given, or nothing at all
// and a list to choose from.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/synth"
)

// source is a soundfont gscore knows how to go and get.
type source struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	About   string `json:"about"`
	License string `json:"license"`
}

// catalogue is the list offered to somebody who has no soundfont at all.
//
// Short on purpose. The point is that one click produces a good orchestra,
// not that gscore becomes a directory of soundfonts: anything else can be
// given by its link or picked off the disk, and both of those paths are the
// same code as this one.
var catalogue = []source{{
	ID:   "generaluser-gs",
	Name: "GeneralUser GS",
	URL:  "https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/main/GeneralUser-GS.sf2",
	Size: 32319396,
	About: "A complete General MIDI set in 32 MB, by S. Christian Collins. " +
		"The usual recommendation for a first soundfont: every program is " +
		"there and none of them is embarrassing.",
	License: "Free to use for any purpose, including commercial work.",
}}

// soundfontDir is where downloaded soundfonts are kept.
//
// Beside the program's other settings, not in the score's folder: a
// soundfont belongs to the person, not to the piece, and the next score they
// write should find it already there.
func soundfontDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Gscore", "soundfonts"), nil
}

// systemSoundfontDirs are the places other programs put soundfonts, so that
// somebody who already has one does not download it again.
func systemSoundfontDirs() []string {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, "Library", "Audio", "Sounds", "Banks"),
			filepath.Join(home, ".local", "share", "soundfonts"),
		)
	}
	return append(dirs,
		"/Library/Audio/Sounds/Banks",
		"/usr/share/sounds/sf2",
		"/usr/share/soundfonts",
		"/usr/local/share/sounds/sf2",
		"/opt/homebrew/share/sounds/sf2",
	)
}

// installed is one soundfont found on this machine.
type installed struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Where string `json:"where"`
}

// listSoundfonts finds every soundfont on this machine gscore knows to look
// for, the downloaded ones first.
func listSoundfonts() []installed {
	var out []installed
	seen := map[string]bool{}

	add := func(dir, where string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".sf2") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if seen[path] {
				continue
			}
			seen[path] = true
			size := int64(0)
			if info, err := e.Info(); err == nil {
				size = info.Size()
			}
			out = append(out, installed{
				Path:  path,
				Name:  strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
				Size:  size,
				Where: where,
			})
		}
	}

	if dir, err := soundfontDir(); err == nil {
		add(dir, "downloaded")
	}
	for _, dir := range systemSoundfontDirs() {
		add(dir, "on this machine")
	}
	return out
}

/*
 * Keeping them loaded
 */

// fonts holds the soundfonts already read, because reading one is thirty
// megabytes of samples and the dialog asks about them every time it is
// opened.
var fonts struct {
	sync.Mutex
	byPath map[string]*synth.Font
}

func loadFont(path string) (*synth.Font, error) {
	fonts.Lock()
	defer fonts.Unlock()
	if f, ok := fonts.byPath[path]; ok {
		return f, nil
	}
	f, err := synth.LoadFont(path)
	if err != nil {
		return nil, err
	}
	if fonts.byPath == nil {
		fonts.byPath = map[string]*synth.Font{}
	}
	fonts.byPath[path] = f
	return f, nil
}

/*
 * What the score needs
 */

// voice is one staff of the score and what will be heard playing it.
type voice struct {
	Part       int    `json:"part"`
	Name       string `json:"name"`
	Instrument string `json:"instrument"`
	Program    int    `json:"program"`
	Drum       bool   `json:"drum"`
	// Font is the soundfont this part is set to play through, empty for
	// whichever one is the default.
	Font string `json:"font,omitempty"`
	// Preset is what that soundfont will actually sound, and Missing says
	// it has nothing for this instrument at all.
	Preset  string `json:"preset,omitempty"`
	Missing bool   `json:"missing"`
}

// voices lists the parts of a score with the instrument each one sounds as.
//
// This is the review: what gscore thinks the piece is scored for, before
// anybody spends a minute rendering it and finds out that the staff they
// meant as a clarinet has been a piano all along.
func (a *App) voices(assign map[int]string, def string) []voice {
	var out []voice
	// Hidden staves included: hiding one is a decision about the page, and
	// it still sounds.
	for i, p := range a.score.Parts {
		v := voice{
			Part:       i,
			Name:       partName(p, i),
			Instrument: p.Instrument.String(),
			Program:    int(p.Instrument),
			Drum:       p.Clef == model.Percussion,
			Font:       assign[i],
		}
		if v.Drum {
			v.Instrument = "percussion"
		}
		path := v.Font
		if path == "" {
			path = def
		}
		if path != "" {
			if f, err := loadFont(path); err == nil {
				if preset, ok := f.Lookup(v.Program, v.Drum); ok {
					v.Preset = preset.Name
				} else {
					v.Missing = true
				}
			}
		}
		out = append(out, v)
	}
	return out
}

// partName is what to call a staff in a list of them.
func partName(p *model.Part, i int) string {
	if n := strings.TrimSpace(p.Name); n != "" {
		return n
	}
	return "Staff " + strconv.Itoa(i+1)
}

/*
 * The API
 */

// handleSoundfonts answers with everything the recording dialog shows: the
// soundfonts on this machine, the ones that can be fetched, and what the
// score is scored for.
func (a *App) handleSoundfonts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	def := q.Get("font")
	assign := parseAssignment(q.Get("assign"))

	a.mu.RLock()
	parts := a.voices(assign, def)
	a.mu.RUnlock()

	dir, _ := soundfontDir()
	writeJSON(w, map[string]any{
		"installed":   listSoundfonts(),
		"catalogue":   catalogue,
		"dir":         dir,
		"canDownload": a.local,
		"voices":      parts,
	})
}

// parseAssignment reads "0=/a.sf2,2=/b.sf2": which part plays through which
// soundfont, for everything that is not on the default one.
func parseAssignment(s string) map[int]string {
	out := map[int]string{}
	for _, pair := range strings.Split(s, ",") {
		key, path, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if i, err := strconv.Atoi(strings.TrimSpace(key)); err == nil && path != "" {
			out[i] = path
		}
	}
	return out
}

// handleSoundfontFetch downloads a soundfont and keeps it.
//
// Either one from the list, by id, or any link at all. What arrives is read
// as a soundfont before it is kept: a link that has rotted into a web page,
// or a download that stopped halfway, is a file that would otherwise sit
// there looking installed and playing nothing.
func (a *App) handleSoundfontFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !a.local {
		http.Error(w, "soundfonts can only be downloaded on this machine",
			http.StatusForbidden)
		return
	}
	q := r.URL.Query()
	url, name := q.Get("url"), q.Get("name")
	if id := q.Get("id"); id != "" {
		found := false
		for _, s := range catalogue {
			if s.ID == id {
				url, name, found = s.URL, s.Name, true
			}
		}
		if !found {
			http.Error(w, "no such soundfont", http.StatusBadRequest)
			return
		}
	}
	if !strings.HasPrefix(url, "https://") {
		http.Error(w, "a soundfont is fetched over https", http.StatusBadRequest)
		return
	}

	path, err := fetchSoundfont(url, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	f, err := loadFont(path)
	if err != nil {
		os.Remove(path)
		http.Error(w, "what arrived is not a soundfont: "+err.Error(),
			http.StatusBadGateway)
		return
	}
	size := int64(0)
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
	}
	writeJSON(w, map[string]any{
		"status": "ok",
		"font": installed{
			Path: path, Name: f.Name, Size: size, Where: "downloaded",
		},
	})
}

// fetchSoundfont downloads to a temporary name beside the destination and
// moves it into place when it is whole, so that an interrupted download is
// never mistaken for a soundfont.
func fetchSoundfont(url, name string) (string, error) {
	dir, err := soundfontDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(url), filepath.Ext(url))
	}
	path := filepath.Join(dir, safeFileName(name)+".sf2")

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the download answered %s", resp.Status)
	}

	tmp, err := os.CreateTemp(dir, "download-*.part")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(tmp, resp.Body)
	tmp.Close()
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return path, nil
}

func safeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "soundfont"
	}
	return out
}

// handleSoundfontPick opens the system's file panel on a soundfont.
func (a *App) handleSoundfontPick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !a.local {
		http.Error(w, "only on this machine", http.StatusForbidden)
		return
	}
	path, err := pickFile()
	if err != nil {
		writeJSON(w, map[string]any{"unavailable": true})
		return
	}
	if path == "" {
		writeJSON(w, map[string]any{"cancelled": true})
		return
	}
	f, err := loadFont(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	size := int64(0)
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
	}
	writeJSON(w, map[string]any{
		"status": "ok",
		"font":   installed{Path: path, Name: f.Name, Size: size, Where: "chosen"},
	})
}

// presetsJSON is a soundfont's whole contents, for somebody who wants to see
// what is in one.
func (a *App) handleSoundfontPresets(w http.ResponseWriter, r *http.Request) {
	f, err := loadFont(r.URL.Query().Get("font"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b, _ := json.Marshal(f.Presets())
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

var errNoSoundfont = errors.New(
	"there is no soundfont to play this with: choose or download one first")
