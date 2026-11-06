/*
 * metrics.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

// How wide the built-in fonts are.
//
// The fourteen standard fonts are not in the PDF, so a reader supplies them —
// but the widths still have to be known here, because a title is centred
// before anybody opens the file. These are Adobe's own numbers from the font
// metrics that have shipped with PostScript since 1984, in thousandths of the
// type size, for the printable ASCII range.
//
// Only that range is tabulated. Everything above it gets the average, which
// is wrong by a hair for an accented letter in a composer's name and wrong by
// nothing at all for everything gscore actually centres.

// The tables themselves live in internal/textmetric, because the engraver
// needs them too: a rehearsal letter is boxed and a staff name reserves its
// margin while the page is being laid out, long before anybody writes a
// file. Kept here as well they would be two copies of Adobe's numbers, and
// two copies of a table are two tables that drift.

import "github.com/stricaud/gscore/internal/textmetric"

// widthOf is how wide a character is in one of the built-in fonts, in
// thousandths of the type size.
func widthOf(name StandardFont, r rune) int {
	return textmetric.StdWidth(string(name), r)
}
