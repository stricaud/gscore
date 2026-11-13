/*
 * voice.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"net/url"
	"strconv"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Which line of music on the staff a command is addressed to.
 *
 * A measure holds voices and a voice holds elements; two independent lines
 * written on one staff — a held note under a moving part, two singers on one
 * stave — are two voices, written across each other rather than after each
 * other. The model has always said so and the engraver has always drawn it:
 * opposed stems, rests moved aside, each voice's slurs its own.
 *
 * The editor could only ever reach the first. Everything was written at
 * Voices[0] and read back from it, so a second voice could be imported,
 * drawn, played, saved and printed, and never touched again. Worse than
 * that: the drawing reported an element's place in the bar's *drawing*
 * order, every voice flattened into one list, and the editor sent that
 * number back as an index into Voices[0]. With one voice the two are the
 * same number. With two they come apart at the first note, and every click
 * in the bar edited something other than what was clicked.
 *
 * So the voice travels with the position, everywhere: in the selection
 * grammar, in the query of the routes that write, and back out in the
 * layout. One number, defaulting to zero, which is what every score that
 * has never heard of voices already means.
 */

// maxVoices is how many lines one staff may carry.
//
// Four, which is what MusicXML's own writers settled on, what a chorale
// needs, and more than anybody is reading off one staff at a stand. The
// number is here so that a bad request cannot make a thousand empty voices.
const maxVoices = 4

// voiceAt is the voice a command names, for reading.
//
// It does not make one: reading from a voice that is not there is a
// question about music that was never written, and answering it with an
// empty voice would turn every mistyped index into a silent no-op.
func voiceAt(m *model.Measure, n int) (*model.Voice, error) {
	if n < 0 || n >= maxVoices {
		return nil, editError("there is no such voice")
	}
	if n >= len(m.Voices) || m.Voices[n] == nil {
		return nil, editError("that staff has no voice " + strconv.Itoa(n+1) + " in this bar")
	}
	return m.Voices[n], nil
}

// voiceForWriting is the voice a command names, made if it is not there.
//
// Writing the first note of a second voice is how a second voice begins —
// there is no other moment it could, and asking somebody to create one
// first would be asking them to say twice what they have already said by
// choosing it. The voices between are made as well, so that the numbering
// means what it says: voice three is the third line of the staff whether or
// not anybody has written in the second yet.
func voiceForWriting(m *model.Measure, n int) (*model.Voice, error) {
	if n < 0 || n >= maxVoices {
		return nil, editError("there is no such voice")
	}
	for len(m.Voices) <= n {
		m.Voices = append(m.Voices, &model.Voice{})
	}
	if m.Voices[n] == nil {
		m.Voices[n] = &model.Voice{}
	}
	return m.Voices[n], nil
}

// voiceParam reads the voice a request names. Absent is the first, which is
// what every request written before voices existed means.
func voiceParam(q url.Values) int {
	n, err := strconv.Atoi(q.Get("voice"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// tidyVoices drops the empty voices from the end of a bar.
//
// Deleting the last note of a second voice should leave a bar of one voice,
// not a bar of one voice and an empty line nobody can see. An empty voice
// is not nothing: it is counted in the numbering, exported as a voice, and
// makes the staff answer "two voices" to everything that asks.
//
// Only from the end. A gap in the middle — voice three written and voice two
// emptied — is somebody's half-finished work, and closing the gap would
// renumber voice three under them.
func tidyVoices(m *model.Measure) {
	for len(m.Voices) > 1 {
		last := m.Voices[len(m.Voices)-1]
		if last != nil && len(last.Elements) > 0 {
			return
		}
		m.Voices = m.Voices[:len(m.Voices)-1]
	}
}

// voiceCount is how many lines of music a bar actually carries.
func voiceCount(m *model.Measure) int {
	n := 0
	for i, v := range m.Voices {
		if v != nil && len(v.Elements) > 0 {
			n = i + 1
		}
	}
	if n == 0 {
		return 1
	}
	return n
}

// lineOf is a measure's nth voice, falling back to the first.
//
// For reading a selection, where a voice that is not there means a stale
// selection — a bar edited since it was made — and the honest answer is the
// line that certainly exists rather than an error thrown at somebody who
// did nothing wrong. The callers all range-check the index afterwards, so a
// fallback costs nothing and a nil would cost a panic.
func lineOf(m *model.Measure, n int) *model.Voice {
	if len(m.Voices) == 0 {
		m.Voices = []*model.Voice{{}}
	}
	if n <= 0 || n >= len(m.Voices) || m.Voices[n] == nil {
		return m.Voices[0]
	}
	return m.Voices[n]
}
