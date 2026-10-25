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
		"/opt/homebrew/share/sounds/sf2",
	)
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
