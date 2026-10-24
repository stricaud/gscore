/*
 * bridge.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Command bridge is the engine of the macOS application, compiled as a
// library rather than as a program.
//
// The Mac application is AppKit: a real menu bar, real panels, real document
// handling, written in Swift, in ui/macosx. Inside its window is a WKWebView
// showing the same interface the web build shows, served by the same Go
// server the command line serves. Nothing here is a second gscore — it is the
// one gscore, with an application built around it instead of a window.
//
// Why a library and not a helper process. A score editor that spawns a server
// and talks to it over a socket has two lifetimes to keep in step, an extra
// process in the dock and the activity monitor, and a port that something
// else might be holding. Linked in, there is one process, and when the
// application quits the server is gone with it.
//
// Go owns a background goroutine and its own threads; AppKit owns the main
// thread and the run loop. They do not meet: Swift calls in, gets a URL back,
// and everything after that is HTTP.
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"github.com/stricaud/gscore/internal/app"
	"github.com/stricaud/gscore/internal/model"

	// The importers register themselves, exactly as in the other builds.
	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
	_ "github.com/stricaud/gscore/internal/format/nwc"
)

// current is the editor this process is showing. One window, one score —
// a second window would be a second score and the application does not open
// those yet.
var current *app.App

// GscoreStart boots the editor and returns the address its pages are at.
//
// The caller owns the string and frees it. An empty string means it could not
// listen, which on a loopback port means something is very wrong and the
// application says so rather than opening an empty window.
//
//export GscoreStart
func GscoreStart() *C.char {
	current = app.NewBook([]*model.Score{model.NewScore()}, "")
	url, err := current.Serve("127.0.0.1:0", true)
	if err != nil {
		return C.CString("")
	}
	return C.CString(url)
}

// GscoreOpen reads a file into the running editor, for a score opened from
// the Finder or from the application's own Open panel.
//
// It answers with the trouble it had, or an empty string for none: a file
// that is partly readable is opened and the warning shown, because a score
// with one bad bar in it is still a score.
//
//export GscoreOpen
func GscoreOpen(path *C.char) *C.char {
	if current == nil {
		return C.CString("the editor has not started")
	}
	if err := current.OpenPath(C.GoString(path)); err != nil {
		return C.CString(err.Error())
	}
	return C.CString("")
}

// main is required of a package built as an archive, and never runs.
func main() {}
