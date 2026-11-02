/*
 * plugins_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Scripts that belong to the person rather than to a score.
 *
 * The folder is the installation: putting a file in it installs it and
 * deleting the file uninstalls it. Everything here is about that sentence
 * being true — a folder that topped itself up would bring back a sample
 * somebody threw away, and a menu built by running what is in the folder
 * would run everything in it every time the editor started.
 */

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pluginsIn makes a plugins folder under a temporary home and gives back
// its path, with the samples in it.
func pluginsIn(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	scores := filepath.Join(home, "Documents", ScoresFolder)
	if err := os.MkdirAll(scores, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := pluginsDirIn(scores)
	if dir == "" {
		t.Fatal("the plugins folder was not made")
	}
	return dir
}

// The samples are written the first time the folder is wanted, so that the
// menu has something on it and anybody wanting to write one has something
// to read.
func TestTheSamplesAreWrittenWhenTheFolderIsMade(t *testing.T) {
	dir := pluginsIn(t)
	got := readPluginsIn(dir)
	if len(got) < 3 {
		t.Fatalf("%d samples were written", len(got))
	}
	for _, p := range got {
		if p.Name == "" || p.Body == "" {
			t.Errorf("%s came back as %+v", p.File, p)
		}
		if p.Menu == "" {
			t.Errorf("the sample %s does not ask to be on a menu, so nobody "+
				"installing gscore would ever see it", p.File)
		}
		if !strings.HasSuffix(p.File, ".js") {
			t.Errorf("%s is not a script", p.File)
		}
	}
	// Readable and editable: a plugin is a file somebody opens and changes.
	st, err := os.Stat(filepath.Join(dir, got[0].File))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Errorf("%s is %v, which is not a file its owner can edit and read",
			got[0].File, st.Mode().Perm())
	}
}

// A sample you deleted stays deleted. It would not if the folder were
// topped up every time it was asked for, and a folder that disagrees with
// you about what is in it is not your folder.
func TestASampleThatWasDeletedStaysDeleted(t *testing.T) {
	dir := pluginsIn(t)
	was := readPluginsIn(dir)
	if len(was) < 2 {
		t.Fatalf("%d samples to delete one of", len(was))
	}
	if err := os.Remove(filepath.Join(dir, was[0].File)); err != nil {
		t.Fatal(err)
	}

	// Asked for again, the way the next start of the program asks for it.
	again := pluginsDirIn(filepath.Dir(dir))
	if again != dir {
		t.Fatalf("the folder moved to %q", again)
	}
	now := readPluginsIn(dir)
	if len(now) != len(was)-1 {
		t.Errorf("%d plugins after deleting one of %d: the folder put it back",
			len(now), len(was))
	}
}

// And a sample you edited is yours. The same rule, and the one that matters
// more: somebody who changed a sample into their own tool would lose it.
func TestASampleThatWasEditedIsNotWrittenOver(t *testing.T) {
	dir := pluginsIn(t)
	was := readPluginsIn(dir)
	mine := filepath.Join(dir, was[0].File)
	if err := os.WriteFile(mine, []byte("// @name Mine\n// @menu Scripts\nsay(1);\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pluginsDirIn(filepath.Dir(dir))
	body, err := os.ReadFile(mine)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Mine") {
		t.Error("the sample was written back over somebody's own script")
	}
}

// What a plugin says about itself is read out of a comment at the top of the
// file. Read, never run: the menu is built out of a folder of scripts
// without any of them having been executed to find out what they are
// called.
func TestAPluginSaysWhatItIsInAComment(t *testing.T) {
	got := pluginHeader("thing.js", `// @name  Parallel fifths
// @about  what it does
// @menu   Scripts

say("hello");
`)
	if got.Name != "Parallel fifths" {
		t.Errorf("it is called %q", got.Name)
	}
	if got.About != "what it does" {
		t.Errorf("about is %q", got.About)
	}
	if got.Menu != "Scripts" {
		t.Errorf("menu is %q", got.Menu)
	}
	if got.File != "thing.js" {
		t.Errorf("file is %q", got.File)
	}
}

// Only the comment at the top. Once the script has started, an @name in it
// is part of the script — a line of someone's prose, or a string — and not
// a claim about what the file is called.
func TestOnlyTheHeaderAtTheTopIsRead(t *testing.T) {
	got := pluginHeader("thing.js", `// @name  The real name

say("x");
// @name  Not the name
// @menu  Scripts
`)
	if got.Name != "The real name" {
		t.Errorf("it is called %q", got.Name)
	}
	if got.Menu != "" {
		t.Errorf("it registered itself on the %q menu from inside its own "+
			"body, where anything at all could say so", got.Menu)
	}
}

// A file with nothing at the top of it is still a file in the folder. It is
// named after itself so that it reads as something rather than as nothing.
func TestAPluginWithNoHeaderIsNamedAfterItsFile(t *testing.T) {
	got := pluginHeader("my-counter.js", "say(score.bars);\n")
	if got.Name != "my-counter" {
		t.Errorf("it is called %q", got.Name)
	}
	// And it is not on a menu, because it never asked to be. A folder is
	// somewhere to keep a half-written script too.
	if got.Menu != "" {
		t.Errorf("it is on the %q menu without having asked", got.Menu)
	}
}

// Sorted by what the menu says, not by the filename: a plugin called
// "Parallel fifths" in a file called 01-parallels.js belongs under P.
func TestThePluginsAreInTheOrderAMenuReadsThemIn(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"01-zebra.js": "// @name Zebra\n// @menu Scripts\n",
		"02-apple.js": "// @name Apple\n// @menu Scripts\n",
		"03-mango.js": "// @name Mango\n// @menu Scripts\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var names []string
	for _, p := range readPluginsIn(dir) {
		names = append(names, p.Name)
	}
	if strings.Join(names, " ") != "Apple Mango Zebra" {
		t.Errorf("the menu would read %v", names)
	}
}

// Only scripts, and only scripts of a size a script is. The folder is a
// folder on somebody's disk and whatever else is in it is not gscore's to
// read into memory.
func TestOnlyScriptsAreRead(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, body []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("good.js", []byte("// @name Good\n// @menu Scripts\n"))
	write("notes.txt", []byte("// @name Notes\n// @menu Scripts\n"))
	write("score.gscore", []byte("// @name Score\n// @menu Scripts\n"))
	write("huge.js", make([]byte, pluginFileMax+1))
	if err := os.Mkdir(filepath.Join(dir, "folder.js"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := readPluginsIn(dir)
	if len(got) != 1 || got[0].Name != "Good" {
		var names []string
		for _, p := range got {
			names = append(names, p.File)
		}
		t.Errorf("read %v, want good.js alone", names)
	}
}

// Served to a browser somewhere else, the folder is the server's and its
// scripts are its owner's. Offering them on another person's menu would be
// offering to run one person's files from another person's window.
func TestTheFolderIsNotOfferedToSomebodyElsesBrowser(t *testing.T) {
	h := newHarness(t)
	h.app.local = false

	var got struct {
		Folder  string       `json:"folder"`
		Plugins []pluginInfo `json:"plugins"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/plugins")), &got); err != nil {
		t.Fatalf("the endpoint did not answer: %v", err)
	}
	if len(got.Plugins) != 0 || got.Folder != "" {
		t.Errorf("a browser elsewhere was offered %d scripts from %q",
			len(got.Plugins), got.Folder)
	}
}

// A sample added to a later version of gscore reaches somebody who has
// already run an earlier one.
//
// The rule used to be "fill the folder when it is made", which meant a new
// sample never arrived for anybody who had ever started gscore before. What
// is remembered now is what has been written rather than what is there, so
// a sample that has never been offered is offered and one that was thrown
// away stays thrown away.
func TestANewSampleReachesAFolderThatAlreadyExists(t *testing.T) {
	dir := pluginsIn(t)
	was := readPluginsIn(dir)
	if len(was) < 2 {
		t.Fatalf("%d samples to work with", len(was))
	}

	// As though this version carried one more than the last did: take one
	// out of the folder and out of the record, which is the state of
	// somebody who has never been given it.
	gone := was[0].File
	if err := os.Remove(filepath.Join(dir, gone)); err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, p := range was[1:] {
		kept = append(kept, p.File)
	}
	if err := os.WriteFile(filepath.Join(dir, installedList),
		[]byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pluginsDirIn(filepath.Dir(dir))
	if _, err := os.Stat(filepath.Join(dir, gone)); err != nil {
		t.Errorf("%s was never written, so a sample added in a later "+
			"version would reach nobody who had run an earlier one", gone)
	}
}

// And one that was thrown away stays thrown away, which is the same record
// read the other way round.
func TestADeletedSampleIsRecordedAsGiven(t *testing.T) {
	dir := pluginsIn(t)
	was := readPluginsIn(dir)
	body, err := os.ReadFile(filepath.Join(dir, installedList))
	if err != nil {
		t.Fatalf("nothing was written down about what was given: %v", err)
	}
	for _, p := range was {
		if !strings.Contains(string(body), p.File) {
			t.Errorf("%s was written and not recorded, so deleting it would "+
				"bring it back", p.File)
		}
	}
}

// The record is not a plugin. A folder of scripts with a list of filenames
// in it would otherwise put the list on the menu.
func TestTheRecordIsNotOfferedAsAPlugin(t *testing.T) {
	dir := pluginsIn(t)
	for _, p := range readPluginsIn(dir) {
		if p.File == installedList {
			t.Error("the record of what was installed is on the Scripts menu")
		}
	}
}
