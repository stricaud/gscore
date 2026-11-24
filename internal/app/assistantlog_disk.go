//go:build !js

/*
 * assistantlog_disk.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// assistantLogPath is where the log is kept: ~/.gscore/ai.log.
var assistantLogPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gscore", "ai.log"), nil
}

// logAssistant adds an entry to the end of the log. The folder is made if it
// is not there, and the file is readable by its owner alone: what is in it
// is what somebody asked, and the music they are writing.
//
// A log that cannot be written is said on the terminal and is not a reason
// to fail the request: the music was asked for, not the record of it.
func logAssistant(e assistantEntry) { writeAssistantLog(e.Answered()) }

// logAssistantAsked starts an entry: the question, written as it is asked.
func logAssistantAsked(e assistantEntry) { writeAssistantLog(e.Asked()) }

func writeAssistantLog(text string) {
	path, err := assistantLogPath()
	if err == nil {
		err = folderFor(filepath.Dir(path))
	}
	var f *os.File
	if err == nil {
		f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	}
	if err == nil {
		_, err = f.WriteString(text)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gscore: the assistant's log could not be written: %v\n", err)
	}
}

// folderFor makes the folder the log goes in.
//
// gscore 0.0.9 left a file called .gscore in the home folder — one line, its
// own name and version — where this wants a folder of that name. Made over
// it, the folder could not be made at all and nothing was logged. The old
// file goes inside the new folder as it was, under the name of the version
// that wrote it, so nothing in it is lost.
func folderFor(dir string) error {
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		aside := dir + ".moving"
		if err := os.Rename(dir, aside); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			os.Rename(aside, dir) // put it back as it was
			return err
		}
		if err := os.Rename(aside, filepath.Join(dir, "gscore-0.0.9")); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "gscore: %s was a file left by an older gscore; "+
			"it is now %s, and the folder holds the assistant's log\n",
			dir, filepath.Join(dir, "gscore-0.0.9"))
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}
