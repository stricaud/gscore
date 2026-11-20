/*
 * clipboard.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Music copied in MuseScore, read off the clipboard.
 *
 * A range copied in MuseScore goes onto the system clipboard under the type
 * application/musescore/stafflist as a fragment of its own file format: one
 * <Staff> per staff copied, holding the chords and rests of the range laid
 * end to end, with the marks that fall among them. The same elements as in a
 * .mscx file, read by the same reader; what differs is the shape around them.
 *
 * There are no bars. The fragment knows how long it is and where it was cut
 * from, and the chords follow one another; a <location> says where the
 * cursor jumps — to another voice, or back to the start of the range for the
 * next voice. Where the barlines fall is the business of the score it is
 * pasted into, which is as MuseScore itself treats it.
 */

package musescore

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/stricaud/gscore/internal/model"
)

// ClipboardType is the type MuseScore puts a copied range on the clipboard
// under.
const ClipboardType = "application/musescore/stafflist"

// Pasted is music copied in MuseScore, as its clipboard holds it: the staves
// of the range from the top down.
type Pasted struct {
	Staves []PastedStaff
	// Length is how much music the range covers, in whole notes.
	Length model.Duration
}

// PastedStaff is one staff of the range, by the voice each run of music was
// in. A voice nothing was copied from is nil.
type PastedStaff struct {
	Voices [museVoices][]PastedElement
}

// PastedElement is one element of the copy and where it begins, counted in
// whole notes from the start of the range.
type PastedElement struct {
	Element model.Element
	At      model.Duration
}

// museVoices is how many voices a MuseScore staff has.
const museVoices = 4

// ReadPasted reads what MuseScore put on the clipboard. What could not be
// read is reported in a *PartialError beside the music, as when a file is
// opened.
func ReadPasted(data []byte) (*Pasted, error) {
	r := &reader{seen: map[string]bool{}}
	r.dec = xml.NewDecoder(bytes.NewReader(data))
	r.dec.Strict = false

	var out *Pasted
	for out == nil {
		tok, err := r.dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("musescore: %w", err)
		}
		e, ok := tok.(xml.StartElement)
		if !ok || e.Name.Local != "StaffList" {
			continue
		}
		r.version = attr(e, "version")
		out = &Pasted{Length: parseFraction(attr(e, "len")).Reduced()}
		if err := r.staffList(out, parseFraction(attr(e, "tick"))); err != nil && err != io.EOF {
			return nil, fmt.Errorf("musescore: %w", err)
		}
	}
	if out == nil {
		return nil, errors.New("musescore: this is not music copied in MuseScore")
	}
	if len(out.Staves) == 0 {
		return nil, errors.New("musescore: nothing was copied")
	}
	if len(r.skipped) > 0 {
		return out, &PartialError{Skipped: r.skipped}
	}
	return out, nil
}

// staffList reads the staves of the range, which began at start.
func (r *reader) staffList(out *Pasted, start model.Duration) error {
	// The cursor runs on from one staff to the next: the second staff opens
	// with a jump back to the start of the range from wherever the first
	// ended. MuseScore writes its position as a difference from the last
	// one, and only when it changes.
	cur := model.Duration{Num: 0, Den: 1}
	return r.children(func(e xml.StartElement) error {
		if e.Name.Local != "Staff" {
			return r.dec.Skip()
		}
		st, err := r.pastedStaff(start, &cur)
		if err != nil {
			return err
		}
		out.Staves = append(out.Staves, st)
		return nil
	})
}

// pastedStaff reads one staff of the range.
func (r *reader) pastedStaff(start model.Duration, cur *model.Duration) (PastedStaff, error) {
	// A scratch bar, because the reader puts music in bars. Everything of
	// this staff goes in the one, voice by voice, and is cut into bars by
	// whoever pastes it.
	m := &model.Measure{}
	st := &msStaff{spans: map[string]*msSpan{}}
	doc := &document{staves: map[string]*msStaff{}, chordNames: map[string]string{}}
	var voices [museVoices]*voiceState
	var at [museVoices][]model.Duration
	voice := 0
	line := func() *voiceState {
		if voices[voice] == nil {
			voices[voice] = newVoice(m)
		}
		return voices[voice]
	}

	err := r.children(func(e xml.StartElement) error {
		switch e.Name.Local {
		case "voiceOffset", "transposeDiatonic":
			return r.dec.Skip()
		case "transposeChromatic":
			// The clipboard carries what sounds, as the file does; the
			// staff's own transposition takes it back to what is written.
			n, err := r.number()
			st.transChr = n
			return err
		case "location":
			return r.children(func(c xml.StartElement) error {
				switch c.Name.Local {
				case "fractions":
					s, err := r.text()
					*cur = cur.Add(parseFraction(s))
					return err
				case "voices":
					n, err := r.number()
					voice += n
					if voice < 0 || voice >= museVoices {
						voice = 0
					}
					return err
				default:
					return r.dec.Skip()
				}
			})
		case "Tuplet":
			return r.tuplet(line())
		case "endTuplet":
			r.endTuplet(line())
			return r.dec.Skip()
		}
		vs := line()
		before := len(vs.v.Elements)
		if err := r.voiceElement(doc, st, vs, e); err != nil {
			return err
		}
		for _, el := range vs.v.Elements[before:] {
			at[voice] = append(at[voice], cur.Sub(start))
			*cur = cur.Add(el.Duration())
		}
		return nil
	})
	if err != nil {
		return PastedStaff{}, err
	}

	// Close what is open, as at the end of a bar of a file, and tie and
	// slur across the staff as across the bars of one: the ties were
	// written note by note and need their far ends found. A tie or a slur
	// whose other end was outside the range has nothing to hold on to here
	// and is dropped.
	part := &model.Part{Measures: []*model.Measure{m}}
	for v, vs := range voices {
		if vs == nil {
			continue
		}
		r.closeVoice(vs)
		// Every slot up to this one holds a voice, if an empty one: the
		// slur balancing walks them all.
		for len(m.Voices) <= v {
			m.Voices = append(m.Voices, &model.Voice{})
		}
		m.Voices[v] = vs.v
		for _, el := range vs.v.Elements {
			notes, _ := tieNotes(el)
			for _, n := range notes {
				n.TieStop = false
			}
		}
	}
	closeTies(part.Measures)
	balanceSlurs(part)

	var out PastedStaff
	for v, vs := range voices {
		if vs == nil {
			continue
		}
		for i, el := range vs.v.Elements {
			out.Voices[v] = append(out.Voices[v], PastedElement{Element: el, At: at[v][i]})
		}
	}
	return out, nil
}
