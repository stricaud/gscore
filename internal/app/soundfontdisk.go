//go:build !js

/*
 * soundfontdisk.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

// Soundfonts and settings as files, for every build that has a filesystem.
//
// The browser is the one that has not, and asking it anyway is not a matter
// of getting an error back: the page answers its own requests on the one
// thread it has, and a filesystem call from inside that answer waits for a
// reply that cannot be delivered until the answer is finished. It deadlocks.
// So the question is not asked there at all, which is what the other half of
// this pair is for.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

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
		"/usr/share/sounds/sf2/zanderjaz",
		"/usr/local/share/sounds/sf2",
		// Where packaging/linux/install.sh puts the bundled soundfont when it
		// is run as root, so an installed copy finds its own instruments.
		"/usr/local/share/soundfonts",
		"/opt/homebrew/share/sounds/sf2",
	)
}

// soundfontDir is where downloaded soundfonts are kept.
//
// In the Gscore folder, beside the scores — soundfonts, under the folder the
// open and save panels already start in.
//
// They used to go under the program's settings, which is where a program
// puts the things it alone touches, and a soundfont is not one of those. It
// is a file somebody downloads, swaps, borrows from another program, or is
// sent by somebody they play with, and a folder they cannot find is a folder
// they cannot put anything in. On a Mac the old one was inside the Library,
// which the Finder hides.
//
// A soundfont still belongs to the person rather than to the piece: it sits
// beside the scores and not inside any of them, so the next score they write
// finds it already there.
func soundfontDir() (string, error) {
	dir := ScoresDir()
	if dir == "" || dir == "." {
		return "", errors.New("there is nowhere to keep a soundfont")
	}
	return filepath.Join(dir, "soundfonts"), nil
}

// oldSoundfontDir is where they were kept before, under the program's
// settings. It is still read and never written.
//
// Nothing was moved. A download somebody already waited twenty minutes for
// is not something to shuffle around behind their back — a hundred and fifty
// megabytes going quietly missing from where they left it is worse than a
// folder in two places — so the old one is simply still looked in, and
// anything in it goes on working. New ones arrive in the new place.
func oldSoundfontDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Gscore", "soundfonts"), nil
}

// diskSoundfonts finds every soundfont on this machine gscore knows to look
// for, the downloaded ones first.
func diskSoundfonts() []installed {
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
	// And where they were kept before this folder moved into Documents.
	// Nothing was moved with it, so anything downloaded then is still here.
	if dir, err := oldSoundfontDir(); err == nil {
		add(dir, "downloaded")
	}
	for _, dir := range bundledSoundfontDirs() {
		add(dir, "with Gscore")
	}
	for _, dir := range systemSoundfontDirs() {
		add(dir, "on this machine")
	}
	return out
}

// fileExists says whether a file is still where it was. Asked of a
// soundfont chosen long ago, and of the scores in the recent list.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readSettings and writeSettings keep what gscore remembers between
// sessions.
func readSettings() ([]byte, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func writeSettings(b []byte) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func settingsPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Gscore", "settings.json"), nil
}
