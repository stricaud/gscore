/*
 * settings.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	"strconv"
	"strings"
	"sync"

	"github.com/stricaud/gscore/internal/engrave"
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

	// NoScorePDF turns off the PDF that a .gscore file carries in front of
	// the music.
	//
	// On by default, which is why the setting is the negative of it: a
	// .gscore renamed .pdf opens in anything that reads one, and that is
	// most of what the format is for. Somebody who only ever opens them in
	// gscore is paying a second rendering of the whole score on every save
	// for a page nobody looks at, and this is how they stop.
	NoScorePDF bool `json:"noScorePDF,omitempty"`

	// Lang is the language the interface is shown in, as a two-letter code.
	//
	// Empty means whichever the browser asks for, which is right nearly
	// always and is why there is no default to store. It is kept here so
	// that somebody who wants French on an English machine — or English on
	// a French one — says so once.
	Lang string `json:"lang,omitempty"`

	// Keys is the keyboard bindings the person has changed, as the name of
	// the command against the keystroke they want for it.
	//
	// Only the changes. The defaults live in the editor beside the commands
	// they run, which is the only place they can be kept honest, and a copy
	// of them here would be a second list to go stale — and would freeze
	// somebody's bindings to whatever gscore did the day they first ran it.
	Keys map[string]string `json:"keys,omitempty"`

	// LooseBarlines stops gscore moving notes across a barline to make the
	// bars add up.
	//
	// On by default, which is why this is the negative of it: a bar that is
	// too long is nearly always a mistake, and moving what will not fit into
	// the next bar is what makes typing music in feel like typing. It is not
	// always a mistake. Plenty of old music is barred by phrase rather than
	// by beat, or sketches the harmony and leaves the count to the player,
	// and none of it can be typed in at all while the program keeps
	// correcting the arithmetic.
	LooseBarlines bool `json:"looseBarlines,omitempty"`

	// ColourNotes draws every notehead in a colour that says which note it
	// is, and NoteColours is the palette, as the note's letter against a
	// CSS hex colour: {"C": "#d0312d"}.
	//
	// Kept for the person and not in the score, which is the whole of what
	// makes it useful. Somebody learning to read is learning to read, not
	// learning to read this piece: the colours have to be there in the next
	// score they open and in the one after that, or they have to be set up
	// again every time — and a beginner who has to configure the program
	// before they can practise does not practise.
	//
	// It also means the printed page agrees with the screen. The PDF is
	// rendered here, from these, so what comes out of the printer is what
	// was being read on the way to it.
	//
	// A letter and not a number as the key, so that a settings file can be
	// read and edited by a person: "C" says which note it is and 0 does
	// not. Missing letters fall back to the built-in palette, so a file
	// naming one colour changes one colour.
	ColourNotes bool              `json:"colourNotes,omitempty"`
	NoteColours map[string]string `json:"noteColours,omitempty"`

	// UIScale is how big the interface is drawn, where one is the size it
	// was designed at.
	//
	// The menus, the palette and the panels are measured in pixels, which
	// on a large high-resolution display is physically small: the whole
	// interface came out tiny on a 2000-pixel-wide screen while the score
	// beside it was readable. This is a property of the screen rather than
	// of any score, so it is remembered for the person and not kept in a
	// file. Zero means nobody has chosen, which is one.
	UIScale float64 `json:"uiScale,omitempty"`
}

// noteLetters is the order the engraver indexes a palette in, which is
// model.Step's own order: the scale spelled from C.
var noteLetters = [7]string{"C", "D", "E", "F", "G", "A", "B"}

// cleanNoteColours keeps the entries that name a note and a colour gscore can
// actually draw.
//
// Checked here rather than trusted, because this is written to a file that
// outlives the session and is read back without the browser in the way: a
// colour nobody can parse, stored, is a score that comes out wrong every time
// it is opened from then on, with nothing on the screen saying why.
func cleanNoteColours(in map[string]string) map[string]string {
	out := map[string]string{}
	for _, letter := range noteLetters {
		v, ok := in[letter]
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, _, _, good := engrave.RGB(v); !good {
			continue
		}
		out[letter] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NoteColourMap is the palette as it stands, letter by letter, with the
// built-in colours filling in whatever has not been changed.
//
// Always a full set. The editor's colour picker shows seven colours whether
// or not anybody has chosen any, and a picker with blanks in it is a picker
// that cannot say what the score currently looks like.
func NoteColourMap() map[string]string {
	stored := loadSettings().NoteColours
	out := make(map[string]string, len(noteLetters))
	for i, letter := range noteLetters {
		if v, ok := stored[letter]; ok {
			out[letter] = v
			continue
		}
		out[letter] = engrave.DefaultNoteColours[i]
	}
	return out
}

// NoteColours is that palette in the form the engraver takes it.
func NoteColours() engrave.NoteColours {
	m := NoteColourMap()
	var out engrave.NoteColours
	for i, letter := range noteLetters {
		out[i] = m[letter]
	}
	return out
}

// ColourNotes says whether the noteheads are drawn by what note they are.
func ColourNotes() bool { return loadSettings().ColourNotes }

// engraving is the engraving options with the person's own settings folded
// in, which is what every drawing of the score is made with.
//
// Read at the moment of drawing rather than copied into the app when they
// change. The colours belong to the person and not to the score, so they
// are not part of the document's state, and there is more than one way into
// a drawing — the editor's own view, the print, the PDF a .gscore carries,
// each export. Every one of them has to agree, and the only way to be sure
// they do is for every one of them to ask the same question here. A copy
// kept on the app is a copy that is right until somebody adds a fourth
// route and forgets it.
func (a *App) engraving() engrave.Options {
	opt := a.engraveOpt
	opt.ColourNotes = ColourNotes()
	opt.NoteColours = NoteColours()
	return opt
}

// uiScaleRange is what the interface may be scaled to.
//
// Below three-quarters the labels stop being legible, and past twice the
// size the menus no longer fit a laptop screen — at which point the thing
// to change is the window, not this.
const (
	uiScaleMin = 0.75
	uiScaleMax = 2.0
)

// UIScale is how big to draw the interface, with nobody's choice meaning
// the size it was designed at.
func UIScale() float64 {
	v := loadSettings().UIScale
	if v < uiScaleMin || v > uiScaleMax {
		return 1
	}
	return v
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
		if v, ok := r.URL.Query()["scorePDF"]; ok && len(v) > 0 {
			s.NoScorePDF = v[0] == "0"
		}
		if v, ok := r.URL.Query()["trackBarlines"]; ok && len(v) > 0 {
			s.LooseBarlines = v[0] == "0"
		}
		if v, ok := r.URL.Query()["uiScale"]; ok && len(v) > 0 {
			n, err := strconv.ParseFloat(v[0], 64)
			if err != nil || n < uiScaleMin || n > uiScaleMax {
				http.Error(w, "the interface scale is a number between "+
					trimFloat(uiScaleMin)+" and "+trimFloat(uiScaleMax),
					http.StatusBadRequest)
				return
			}
			s.UIScale = n
		}
		if v, ok := r.URL.Query()["lang"]; ok && len(v) > 0 {
			// "auto" is not a language; it is the absence of a choice, and
			// an empty setting is how that is stored.
			if v[0] == "auto" {
				s.Lang = ""
			} else {
				s.Lang = v[0]
			}
		}
		if v, ok := r.URL.Query()["colourNotes"]; ok && len(v) > 0 {
			s.ColourNotes = v[0] == "1"
		}
		if v, ok := r.URL.Query()["noteColours"]; ok && len(v) > 0 {
			// The whole palette each time, the same way the bindings are
			// sent: going back to the built-in colours is sending nothing.
			// Unreadable is a request gscore did not send and is left
			// alone rather than quietly wiping somebody's palette.
			var cols map[string]string
			if err := json.Unmarshal([]byte(v[0]), &cols); err == nil {
				s.NoteColours = cleanNoteColours(cols)
			}
		}
		if v, ok := r.URL.Query()["keys"]; ok && len(v) > 0 {
			// The whole set each time, so taking a binding back to its
			// default is sending a set without it. Unreadable means a
			// request gscore did not send, and is left alone rather than
			// quietly emptying somebody's bindings.
			var keys map[string]string
			if err := json.Unmarshal([]byte(v[0]), &keys); err == nil {
				if len(keys) == 0 {
					keys = nil
				}
				s.Keys = keys
			}
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
		"scorePDF":         SaveScorePDF(),
		"trackBarlines":    TrackBarlines(),
		"keys":             loadSettings().Keys,
		"lang":             loadSettings().Lang,
		"colourNotes":      loadSettings().ColourNotes,
		"noteColours":      NoteColourMap(),
		"uiScale":          UIScale(),
	})
}

// SaveScorePDF says whether a .gscore file should carry a PDF of the score.
func SaveScorePDF() bool { return !loadSettings().NoScorePDF }

// TrackBarlines says whether notes that overflow a bar are moved into the
// next one.
func TrackBarlines() bool { return !loadSettings().LooseBarlines }
