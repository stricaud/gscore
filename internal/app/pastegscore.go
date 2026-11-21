/*
 * pastegscore.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Pasting what was copied in another gscore.
 *
 * Each gscore that is running holds what it copied in its own memory, which
 * is shared between its windows and seen by nothing else. Two of them open
 * side by side are two programs, and copying in one and pasting in the
 * other pasted whatever the second had copied last, or nothing at all.
 *
 * So a copy is also put on the system clipboard, under a type of gscore's
 * own, and paste reads it back when the clipboard has changed since this
 * gscore last copied — the same test that lets a copy made in MuseScore win
 * over an older one made here. See pastemusescore.go.
 *
 * What goes on the clipboard is the elements, in gscore's own file format so
 * that nothing on them is lost on the way, and where each one stood: the
 * shape the copy was taken in. See clipboard.where.
 */

package app

import (
	"bytes"
	"encoding/json"

	"github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
)

// gscoreClipboardType is the type gscore's own copy is put on the system
// clipboard under.
const gscoreClipboardType = "org.gscore.music"

// clipDoc is a copy as it travels between two gscores.
type clipDoc struct {
	// Version is bumped only when an older gscore could no longer make
	// sense of what a newer one copied.
	Version int `json:"version"`
	// Music is a .gscore holding the copied elements one after another in
	// a single line of a single bar, with the custom symbols they use.
	Music []byte `json:"music"`
	// Where is where each of them stood: bar, line of music and staff.
	Where [][3]int `json:"where"`
}

const clipDocVersion = 1

// publishClip puts a copy just made on the system clipboard, for another
// gscore to paste, and returns the clipboard's change count after: the
// count at which this gscore's own copy is the latest thing copied.
func (a *App) publishClip(c *clipboard) int {
	if !a.local || a.setPasteboard == nil {
		return a.pasteboardChange()
	}
	data, err := encodeClip(c, a)
	if err != nil {
		return a.pasteboardChange()
	}
	if change := a.setPasteboard(gscoreClipboardType, data); change != 0 {
		return change
	}
	return a.pasteboardChange()
}

// otherGscoreClip is what another gscore copied, where that is what was
// copied last; nil where it is not. The clipboard's change count comes back
// beside it either way.
func (a *App) otherGscoreClip() (*clipboard, int) {
	data, change := a.pasteboard(gscoreClipboardType)
	if len(data) == 0 {
		return nil, change
	}
	if len(a.clipboard().elements) > 0 && change == a.copiedAt {
		return nil, change // this gscore's own copy, which it already holds
	}
	clip, err := a.decodeClip(data)
	if err != nil || len(clip.elements) == 0 {
		return nil, change
	}
	return clip, change
}

func encodeClip(c *clipboard, a *App) ([]byte, error) {
	els := make([]model.Element, len(c.elements))
	for i, el := range c.elements {
		els[i] = cloneElement(el)
	}
	s := model.NewScore()
	s.Parts[0].Measures[0].Voices[0].Elements = els

	var music bytes.Buffer
	if err := gscore.Write(&music, s, a.symbols); err != nil {
		return nil, err
	}
	doc := clipDoc{Version: clipDocVersion, Music: music.Bytes()}
	for i := range c.elements {
		var w clipAt
		if i < len(c.where) {
			w = c.where[i]
		}
		doc.Where = append(doc.Where, [3]int{w.bar, w.voice, w.staff})
	}
	return json.Marshal(doc)
}

// decodeClip reads another gscore's copy. The custom symbols it uses are
// added to this score's where this one has none of that name; a symbol of
// the same name here is this person's own and is kept.
func (a *App) decodeClip(data []byte) (*clipboard, error) {
	var doc clipDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Version > clipDocVersion {
		return &clipboard{}, nil // a newer gscore's, which this one cannot read
	}
	s, reg, err := gscore.Read(bytes.NewReader(doc.Music))
	if err != nil {
		return nil, err
	}
	clip := &clipboard{}
	if len(s.Parts) > 0 && len(s.Parts[0].Measures) > 0 && len(s.Parts[0].Measures[0].Voices) > 0 {
		clip.elements = s.Parts[0].Measures[0].Voices[0].Elements
	}
	for i := range clip.elements {
		var w clipAt
		if i < len(doc.Where) {
			w = clipAt{bar: doc.Where[i][0], voice: doc.Where[i][1], staff: doc.Where[i][2]}
		}
		clip.where = append(clip.where, w)
	}
	if reg != nil && a.symbols != nil {
		for _, name := range reg.Names() {
			if _, ok := a.symbols.Lookup(name); ok {
				continue
			}
			if sym, ok := reg.Lookup(name); ok {
				_ = a.symbols.Add(sym)
			}
		}
	}
	return clip, nil
}
