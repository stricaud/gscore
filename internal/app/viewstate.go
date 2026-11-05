/*
 * viewstate.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * How a score was last being looked at, kept in the file with it.
 *
 * The drawing options are not music and they are not decisions about the
 * printed page either — they are the state of somebody's desk. Until now
 * they lived only in memory, so turning the beat marks on to work out a
 * syncopated entry lasted exactly as long as the program did, and colouring
 * a part for a pupil had to be done again every time the file was opened.
 *
 * So they travel in the file, the same way the cursor does, and for the same
 * reason: it is work somebody did on this score.
 *
 * Two directions, and they are deliberately not symmetrical.
 *
 * rememberView runs whenever something changes, and writes what the editor
 * is doing into the score. applyView runs when a document is opened, and
 * pushes what the file said back into the editor. A file that says nothing
 * is left alone in both directions — which is every score written before
 * this existed, and every score that comes from a format with no notion of
 * a view, which is all of them but .gscore.
 */

// applyView makes the editor show the score the way the file says it was
// last shown.
//
// Only what the file actually recorded. A score with no view of its own is
// not a score that wants every option turned off: it is a score that has
// never been asked, and the answer for one of those is whatever the person's
// own settings and the editor's defaults already say.
//
// Called with the lock held.
func (a *App) applyView() {
	v := a.score.View
	if v == nil {
		return
	}
	a.engraveOpt.ShowPartNames = v.StaffNames
	a.engraveOpt.ShowMeasureNumbers = v.MeasureNumbers
	a.engraveOpt.MultiRests = v.MultiRests
	a.engraveOpt.ShowTempoMark = v.TempoMark
	a.engraveOpt.ShowBeatMarks = v.BeatMarks
	a.concert = v.ConcertPitch
	if v.Paginated {
		a.engraveOpt.Mode = engrave.Paginated
	} else {
		a.engraveOpt.Mode = engrave.Strip
	}
}

// rememberView writes what the editor is doing into the score, so that
// saving the file keeps it.
//
// Called with the lock held, from every place a drawing option changes.
// Writing it at save time instead would have been one call rather than
// several, and would also have been wrong: the editor saves a score it is
// given rather than the one it has — an export, a part, the copy a .gscore
// carries — and the view belongs to the document, not to the moment it
// happened to be written out.
func (a *App) rememberView() {
	if a.score == nil {
		return
	}
	v := &model.ViewState{
		Paginated:      a.engraveOpt.Mode == engrave.Paginated,
		StaffNames:     a.engraveOpt.ShowPartNames,
		MeasureNumbers: a.engraveOpt.ShowMeasureNumbers,
		MultiRests:     a.engraveOpt.MultiRests,
		TempoMark:      a.engraveOpt.ShowTempoMark,
		BeatMarks:      a.engraveOpt.ShowBeatMarks,
		ConcertPitch:   a.concert,
	}
	// The colours as they are actually being drawn, which is the score's own
	// where it has any and the person's where it has not. Recorded either
	// way, so that a part prepared for a pupil arrives in the colours it was
	// prepared in rather than in whoever opens it's.
	v.ColourNotes = a.colourNotes()
	v.NoteColours = a.noteColours()
	a.score.View = v
}

// adoptSettingsColours makes this score take the palette from the person's
// own settings, and remembers it.
//
// Called when somebody changes the colours in the dialog. Without it the
// dialog would be dead on any score that already carried colours of its own:
// the file wins at drawing time, so a new palette chosen while such a score
// was open would be saved to the settings, change nothing on the page, and
// look broken. Choosing a colour is an instruction about what is in front of
// you, so it lands on what is in front of you.
//
// Called with the lock held.
func (a *App) adoptSettingsColours() {
	if a.score == nil {
		return
	}
	if a.score.View != nil {
		// Dropped rather than overwritten, so that rememberView below reads
		// through to the settings and stamps the new palette.
		a.score.View.NoteColours = nil
	}
	a.rememberView()
}

// colourNotes and noteColours are the colours this score is drawn in: its
// own where the file named any, and the person's own settings where it did
// not.
//
// The file wins, which is the whole point of putting them in it. A score
// arrives looking the way it was sent, and a reader's own palette is the
// default for scores that have not been given one rather than an override of
// those that have.
//
// Called with the lock held.
func (a *App) colourNotes() bool {
	if v := a.score.View; v != nil && len(v.NoteColours) > 0 {
		return v.ColourNotes
	}
	return ColourNotes()
}

func (a *App) noteColours() map[string]string {
	if v := a.score.View; v != nil && len(v.NoteColours) > 0 {
		out := make(map[string]string, len(v.NoteColours))
		for k, c := range v.NoteColours {
			out[k] = c
		}
		return out
	}
	return NoteColourMap()
}

// scoreNoteColours is that palette in the form the engraver takes it.
//
// Called with the lock held.
func (a *App) scoreNoteColours() engrave.NoteColours {
	m := a.noteColours()
	var out engrave.NoteColours
	for i, letter := range noteLetters {
		out[i] = m[letter]
	}
	return out
}
