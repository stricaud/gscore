/*
 * plugins.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/*
 * Plugins: scripts that belong to you rather than to a score.
 *
 * A script kept inside a score travels with that score, which is right for
 * the thought that made the piece and wrong for a tool. A check for
 * parallel fifths is not about one piece; it is something you want on the
 * menu for every piece, the way every other program of this kind has a
 * Plugins menu with thirty things on it.
 *
 * So a plugin is a file in a folder of yours — Gscore/plugins, beside the
 * scores — and the editor reads that folder and puts what it finds on a
 * menu. Nothing is registered, installed or compiled: putting a file there
 * is installing it, and deleting it is uninstalling it.
 *
 * A few samples are written there the first time the folder is wanted, so
 * that the menu has something on it and somebody wanting to write one has
 * something to read. Only the first time: a sample you deleted stays
 * deleted, which it would not if the folder were topped up on every start.
 *
 * Why not .gscore, which is what a script carried by a score lives in: a
 * .gscore is a score everywhere else in the program. One in this folder
 * would be opened as a score by a double-click, offered by the open panel,
 * and carry an engraved PDF inside it. These are JavaScript and they are
 * named so.
 *
 * What a plugin says about itself is read, never run. The name on the menu
 * comes out of a comment at the top of the file:
 *
 *	// @name  Parallel fifths and octaves
 *	// @about reads the outer parts and says where they move in parallel
 *	// @menu  Scripts
 *
 * @menu is the registration. A file without it is a file in the folder and
 * not an entry on a menu — somewhere to keep a half-written script, or a
 * piece one of the others reads.
 */

// PluginsFolder is the folder plugins live in, inside the Gscore folder.
const PluginsFolder = "plugins"

// The samples, carried in the binary and written out the first time the
// folder is wanted. One each for the three things a plugin is usually for:
// looking at the music and reporting, writing music from what is already
// there, and reading a text.
//
//go:embed plugins/*.js
var samplePlugins embed.FS

// pluginFileMax is as much of a file as is read. A plugin is a page or two
// of script; anything past this is not one, and reading it into memory to
// find out would be reading whatever happened to be dropped in the folder.
const pluginFileMax = 256 << 10

// pluginInfo is one plugin, as the editor is told about it.
type pluginInfo struct {
	// Name is what the menu says, About what the tooltip says, Menu which
	// menu it asked to be on.
	Name  string `json:"name"`
	About string `json:"about"`
	Menu  string `json:"menu"`
	// File is the name on the disk, which is what identifies it: two
	// plugins may call themselves the same thing.
	File string `json:"file"`
	// Body is the script itself. Sent with the list because running one is
	// then one step rather than two, and because the console shows it: what
	// is run is what is on the screen.
	Body string `json:"body"`
}

// PluginsDir is the folder plugins are kept in, made and filled with the
// samples the first time it is asked for.
//
// It falls back rather than failing, the way the scores folder does: a home
// that cannot be written to is no reason to refuse to run the editor. The
// menu is then empty, which is what an empty folder means anyway.
func PluginsDir() string {
	return pluginsDirIn(ScoresDir())
}

// pluginsDirIn is the same, under a folder it is given rather than under the
// one the program found. Separate because ScoresDir is worked out once and
// remembered for the life of the process, which is right for a program and
// no use to a test that has to be able to say where the home is.
func pluginsDirIn(scores string) string {
	if scores == "" {
		return ""
	}
	dir := filepath.Join(scores, PluginsFolder)
	if st, err := os.Stat(dir); err == nil {
		if !st.IsDir() {
			// Something that is not a folder is already called that. It is
			// not gscore's to move.
			return ""
		}
		writeSamplePlugins(dir)
		return dir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	writeSamplePlugins(dir)
	return dir
}

// installedList is where the samples that have been written are recorded.
// A dotfile, in the folder it is about: it belongs to that folder and
// travels with it, and it is not a plugin so it is not on the menu.
const installedList = ".installed"

// writeSamplePlugins puts any built-in sample that has not been put there
// before into the folder.
//
// Not "any that is not there", which is the obvious rule and the wrong one:
// a sample you deleted would come back the next time gscore started, and a
// folder that disagrees with you about what is in it is not your folder.
// And not "only when the folder is new" either, which was the rule before
// and meant that a sample added to a later version of gscore never reached
// anybody who had already run an earlier one.
//
// So what is remembered is what has been written, rather than what is
// there. A sample listed in .installed is one you have already been given:
// if it is gone you threw it away, and if it has changed it is yours now.
// One that is not listed has never been offered, so it is written.
func writeSamplePlugins(dir string) {
	names, err := fs.Glob(samplePlugins, "plugins/*.js")
	if err != nil {
		return
	}
	given := map[string]bool{}
	if body, err := os.ReadFile(filepath.Join(dir, installedList)); err == nil {
		for _, line := range strings.Split(string(body), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				given[line] = true
			}
		}
	}

	// A folder that already holds a sample gscore has no record of giving is
	// a folder from before this was written down. Counting what is there as
	// given keeps those alone: the first run after an upgrade should not
	// write over three files somebody may have edited.
	for _, name := range names {
		base := filepath.Base(name)
		if _, err := os.Stat(filepath.Join(dir, base)); err == nil {
			given[base] = true
		}
	}

	wrote := false
	for _, name := range names {
		base := filepath.Base(name)
		if given[base] {
			continue
		}
		body, err := samplePlugins.ReadFile(name)
		if err != nil {
			continue
		}
		if writeSample(filepath.Join(dir, base), body) == nil {
			given[base] = true
			wrote = true
		}
	}

	// Written back whenever anything was added, and also the first time, so
	// that the list exists from then on.
	if wrote || len(given) > 0 {
		var lines []string
		for name := range given {
			lines = append(lines, name)
		}
		sort.Strings(lines)
		_ = writeSample(filepath.Join(dir, installedList),
			[]byte(strings.Join(lines, "\n")+"\n"))
	}
}

// writeSample writes one sample file, beside and then into place.
//
// The same rule everything gscore writes follows, for the same reason: a
// file written straight to its path is empty from the moment it is opened
// until the moment it is finished, and a sample that is half on the disk is
// a script that would run as far as it got and stop in the middle of
// writing music.
func writeSample(path string, body []byte) error {
	dir, name := filepath.Dir(path), filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name()) // a no-op once the rename has happened
	}()
	if _, err := tmp.Write(body); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// The permissions a new file would have had: CreateTemp makes one its
	// owner alone can read, and a plugin is a file to be read and edited.
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// readPlugins is what is in the folder, in the order a menu should show it.
func readPlugins() []pluginInfo {
	return readPluginsIn(PluginsDir())
}

func readPluginsIn(dir string) []pluginInfo {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []pluginInfo
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".js") {
			continue
		}
		if st, err := e.Info(); err == nil && st.Size() > pluginFileMax {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(body) > pluginFileMax {
			continue
		}
		out = append(out, pluginHeader(e.Name(), string(body)))
	}
	// By the name on the menu, which is the order somebody reads them in.
	// Not by filename: a plugin called "Parallel fifths" in a file called
	// 01-parallels.js should sit under P.
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name); a != b {
			return a < b
		}
		return out[i].File < out[j].File
	})
	return out
}

// pluginHeader reads what a plugin says about itself.
//
// Read and not run. The comment at the top of the file is the whole of what
// the editor learns before somebody asks for it to be run, which is what
// lets a menu be built out of a folder of scripts without any of them
// having been executed to find out what they are called.
func pluginHeader(file, body string) pluginInfo {
	// The name falls back to the filename without its extension, so a
	// plugin somebody wrote in a hurry still reads as something on a menu.
	out := pluginInfo{
		File: file,
		Name: strings.TrimSuffix(file, filepath.Ext(file)),
		Body: body,
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			// The header is the comment at the top. Once the script has
			// started, an @name in it is part of the script.
			break
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "//"))
		field, value, found := strings.Cut(rest, " ")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(field) {
		case "@name":
			if value != "" {
				out.Name = value
			}
		case "@about":
			out.About = value
		case "@menu":
			out.Menu = value
		}
	}
	return out
}

// handlePlugins says what is in the plugins folder.
//
// Only where gscore is the application on this machine. Served to a browser
// somewhere else, the folder is the server's and its scripts are the
// server's owner's; offering them on somebody else's menu would be offering
// to run one person's files from another person's window.
func (a *App) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if !a.local {
		writeJSON(w, map[string]any{"plugins": []pluginInfo{}, "folder": ""})
		return
	}
	plugins := readPlugins()
	if plugins == nil {
		plugins = []pluginInfo{}
	}
	writeJSON(w, map[string]any{"plugins": plugins, "folder": PluginsDir()})
}
