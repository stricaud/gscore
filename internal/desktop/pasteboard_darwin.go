//go:build darwin && cgo

/*
 * pasteboard_darwin.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The system clipboard on macOS, for what other programs put on it.
 */

package desktop

/*
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>

// gscore_pasteboard copies what the general pasteboard holds under a type
// into malloc'd memory, or returns NULL. The pasteboard's change count,
// which goes up every time anything is put on it, comes back in *change.
char *gscore_pasteboard(const char *kind, int *n, long *change);
*/
import "C"

import "unsafe"

// Pasteboard is what the system clipboard holds under a type, and the
// clipboard's change count — a number that goes up each time anything is put
// on it, so that a copy made inside gscore can be told from one made since in
// another program. The data is nil where the clipboard holds nothing of that
// type.
//
// The type is a MIME type as the program that wrote it names it. A Qt
// program such as MuseScore writes it under the name Qt gives a MIME type
// on this platform, which is tried as well.
func Pasteboard(kind string) ([]byte, int) {
	for _, name := range []string{kind, qtPasteboardName(kind)} {
		ckind := C.CString(name)
		var n C.int
		var change C.long
		p := C.gscore_pasteboard(ckind, &n, &change)
		C.free(unsafe.Pointer(ckind))
		if p == nil {
			if name == kind {
				continue
			}
			return nil, int(change)
		}
		data := C.GoBytes(unsafe.Pointer(p), n)
		C.free(unsafe.Pointer(p))
		return data, int(change)
	}
	return nil, 0
}
