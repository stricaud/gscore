/*
 * pastemusescore.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Pasting what was copied in MuseScore.
 *
 * MuseScore puts a copied range on the system clipboard in its own format,
 * which the browser's paste event cannot see: a web page is shown only text
 * and pictures. The server can see it, on the machine it runs on, and it
 * is read into the same shape as gscore's own clipboard — the elements and
 * which bar, voice and staff of the copy each belongs to — so that paste
 * then has one thing to do.
 *
 * Which clipboard wins is a matter of which was written to last. The
 * system clipboard counts its changes, and the count at gscore's last copy
 * is remembered: a count that has moved on since is a copy made somewhere
 * else, and if it is MuseScore's it is what the person means to paste.
 */

package app

import (
	"errors"
	"fmt"

	"github.com/stricaud/gscore/internal/format/musescore"
	"github.com/stricaud/gscore/internal/model"
)

// pasteboardChange is the system clipboard's change count now.
func (a *App) pasteboardChange() int {
	if !a.local || a.pasteboard == nil {
		return 0
	}
	_, change := a.pasteboard(musescore.ClipboardType)
	return change
}

// outsideClipboard is what MuseScore copied, as a clipboard of gscore's own,
// where that is what was copied last; nil where it is not. The music is cut
// into bars by the meter of the bar it is pasted into, since the copy
// carries none of its own: where the barlines fall is this score's business.
//
// Whatever in the copy could not be read is returned beside it, for the
// person to be told, as when a file is opened.
func (a *App) outsideClipboard(p *model.Part, measIdx int) (*clipboard, []string, error) {
	// The machine's own clipboard, which a server reached from elsewhere
	// has no business showing anybody.
	if !a.local || a.pasteboard == nil {
		return nil, nil, nil
	}
	data, change := a.pasteboard(musescore.ClipboardType)
	if len(data) == 0 {
		return nil, nil, nil
	}
	if len(a.clipboard().elements) > 0 && change == a.copiedAt {
		return nil, nil, nil // gscore's own copy is the later one
	}

	pasted, err := musescore.ReadPasted(data)
	var partial *musescore.PartialError
	var skipped []string
	switch {
	case errors.As(err, &partial):
		skipped = partial.Skipped
	case err != nil:
		return nil, nil, fmt.Errorf("what MuseScore copied could not be read: %w", err)
	}

	bar := p.TimeAt(measIdx).BarDuration()
	clip := &clipboard{}
	for si, st := range pasted.Staves {
		for v, line := range st.Voices {
			for _, el := range line {
				clip.elements = append(clip.elements, el.Element)
				clip.where = append(clip.where, clipAt{bar: barIndexOf(el.At, bar), voice: v, staff: si})
			}
		}
	}
	return clip, skipped, nil
}

// barIndexOf is which bar, counting from nought, a point that far into the copy
// falls in, when the bars are that long.
func barIndexOf(at, bar model.Duration) int {
	if bar.IsZero() || at.Num <= 0 || at.Den <= 0 {
		return 0
	}
	return at.Num * bar.Den / (at.Den * bar.Num)
}
