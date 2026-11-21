//go:build !(darwin && cgo)

/*
 * pasteboard_other.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package desktop

// Pasteboard is what the system clipboard holds under a type. Off macOS, or
// built without cgo, the clipboard cannot be reached and there is nothing.
func Pasteboard(kind string) ([]byte, int) { return nil, 0 }

// SetPasteboard puts data on the system clipboard. Off macOS, or built
// without cgo, there is no clipboard to put it on, and nothing is done.
func SetPasteboard(kind string, data []byte) int { return 0 }
