/*
 * pasteboard.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package desktop

import "strings"

// qtPasteboardName is the name a Qt program gives a MIME type on the macOS
// pasteboard: its own prefix, and the slashes turned to double dashes.
// MuseScore's application/musescore/stafflist arrives as
// com.trolltech.anymime.application--musescore--stafflist.
func qtPasteboardName(mime string) string {
	return "com.trolltech.anymime." + strings.ReplaceAll(mime, "/", "--")
}
