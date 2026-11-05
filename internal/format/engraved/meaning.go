/*
 * meaning.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engraved

import (
	"strings"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
	"github.com/stricaud/gscore/internal/smufl"
)

// kind is what a glyph turns out to be.
type kind int

const (
	kOther kind = iota
	kNotehead
	kRest
	kClef
	kAccidental
	kFlag
	kDot
)

// sense is a glyph read as music.
type sense struct {
	kind  kind
	value model.NoteValue // for a notehead or a rest
	clef  model.Clef
	acc   model.Accidental
}

// readGlyph works out what a glyph means, from whichever of the two naming
// conventions the file uses.
//
// The two producers that matter name their glyphs in entirely different ways
// and neither is wrong. LilyPond ships its own font and writes an encoding
// table into the PDF naming every code in its own vocabulary — "noteheads.s0",
// "clefs.G". MuseScore ships a SMuFL font and names nothing, but maps each
// code to the codepoint the standard assigns it, and in SMuFL the codepoint
// is the identity: U+E0A4 is a black notehead wherever you meet it.
//
// So we ask for a SMuFL name first, since that one is a standard and settles
// the question, and fall back to reading LilyPond's vocabulary.
func readGlyph(g pdf.Glyph) sense {
	if g.Rune != 0 {
		if name, ok := smufl.ByRune(g.Rune); ok {
			if s, ok := bySmuflName(name); ok {
				return s
			}
		}
	}
	if g.Name != "" {
		return byLilyName(g.Name)
	}
	return sense{}
}

// bySmuflName reads the standard's vocabulary.
//
// Matching is by prefix because SMuFL qualifies its names — noteheadBlack,
// noteheadBlackSmall, noteheadWholeSmall — and a grace note is still a note.
func bySmuflName(n string) (sense, bool) {
	switch {
	case strings.HasPrefix(n, "noteheadDoubleWhole"):
		return sense{kind: kNotehead, value: model.DoubleWhole}, true
	case strings.HasPrefix(n, "noteheadWhole"):
		return sense{kind: kNotehead, value: model.Whole}, true
	case strings.HasPrefix(n, "noteheadHalf"):
		return sense{kind: kNotehead, value: model.Half}, true
	case strings.HasPrefix(n, "noteheadBlack"):
		return sense{kind: kNotehead, value: model.Quarter}, true
	case strings.HasPrefix(n, "flag"):
		// flag8thUp, flag16thDown, flag32ndUp...
		switch {
		case strings.HasPrefix(n, "flag8th"):
			return sense{kind: kFlag, value: model.Eighth}, true
		case strings.HasPrefix(n, "flag16th"):
			return sense{kind: kFlag, value: model.Sixteenth}, true
		case strings.HasPrefix(n, "flag32nd"):
			return sense{kind: kFlag, value: model.ThirtySecond}, true
		case strings.HasPrefix(n, "flag64th"):
			return sense{kind: kFlag, value: model.SixtyFourth}, true
		}
		return sense{kind: kFlag, value: model.Eighth}, true
	case n == "augmentationDot":
		return sense{kind: kDot}, true
	case n == "restWhole" || n == "restWholeLegerLine":
		return sense{kind: kRest, value: model.Whole}, true
	case n == "restHalf" || n == "restHalfLegerLine":
		return sense{kind: kRest, value: model.Half}, true
	case n == "restQuarter":
		return sense{kind: kRest, value: model.Quarter}, true
	case n == "rest8th":
		return sense{kind: kRest, value: model.Eighth}, true
	case n == "rest16th":
		return sense{kind: kRest, value: model.Sixteenth}, true
	case n == "gClef":
		return sense{kind: kClef, clef: model.Treble}, true
	case n == "fClef":
		return sense{kind: kClef, clef: model.Bass}, true
	case n == "cClef":
		return sense{kind: kClef, clef: model.Alto}, true
	case n == "accidentalSharp":
		return sense{kind: kAccidental, acc: model.Sharp}, true
	case n == "accidentalFlat":
		return sense{kind: kAccidental, acc: model.Flat}, true
	case n == "accidentalNatural":
		return sense{kind: kAccidental, acc: model.Natural}, true
	case n == "accidentalDoubleSharp":
		return sense{kind: kAccidental, acc: model.DoubleSharp}, true
	case n == "accidentalDoubleFlat":
		return sense{kind: kAccidental, acc: model.DoubleFlat}, true
	}
	return sense{}, false
}

// byLilyName reads LilyPond's vocabulary.
//
// Its noteheads are numbered by how many flags the note would carry, not by
// what the head looks like: s0 is the whole note, s1 the half, s2 the black
// head that serves the quarter and everything shorter. The suffixes that may
// follow — "noteheads.s2harmonic" — do not change that, so the test is on the
// prefix.
func byLilyName(n string) sense {
	switch {
	case strings.HasPrefix(n, "noteheads.sM1") || strings.HasPrefix(n, "noteheads.s-1"):
		return sense{kind: kNotehead, value: model.DoubleWhole}
	case strings.HasPrefix(n, "noteheads.s0"):
		return sense{kind: kNotehead, value: model.Whole}
	case strings.HasPrefix(n, "noteheads.s1"):
		return sense{kind: kNotehead, value: model.Half}
	case strings.HasPrefix(n, "noteheads.s2") || strings.HasPrefix(n, "noteheads.d2"):
		return sense{kind: kNotehead, value: model.Quarter}
	case strings.HasPrefix(n, "flags.u") || strings.HasPrefix(n, "flags.d"):
		// LilyPond numbers flags as it numbers noteheads: 3 is the eighth,
		// 4 the sixteenth, and so on upward.
		v := model.Eighth
		if len(n) > 7 {
			switch n[7] {
			case '4':
				v = model.Sixteenth
			case '5':
				v = model.ThirtySecond
			case '6':
				v = model.SixtyFourth
			}
		}
		return sense{kind: kFlag, value: v}
	case n == "dots.dot":
		return sense{kind: kDot}
	case strings.HasPrefix(n, "noteheads."):
		// an unnumbered head — a cross, a diamond — is still a note, and a
		// quarter is the least wrong guess at its value
		return sense{kind: kNotehead, value: model.Quarter}
	case n == "rests.0" || n == "rests.0o":
		return sense{kind: kRest, value: model.Whole}
	case n == "rests.1" || n == "rests.1o":
		return sense{kind: kRest, value: model.Half}
	case n == "rests.2":
		return sense{kind: kRest, value: model.Quarter}
	case n == "rests.3":
		return sense{kind: kRest, value: model.Eighth}
	case n == "rests.4":
		return sense{kind: kRest, value: model.Sixteenth}
	case strings.HasPrefix(n, "clefs.G"):
		return sense{kind: kClef, clef: model.Treble}
	case strings.HasPrefix(n, "clefs.F"):
		return sense{kind: kClef, clef: model.Bass}
	case strings.HasPrefix(n, "clefs.C"):
		return sense{kind: kClef, clef: model.Alto}
	case n == "accidentals.sharp":
		return sense{kind: kAccidental, acc: model.Sharp}
	case n == "accidentals.flat":
		return sense{kind: kAccidental, acc: model.Flat}
	case n == "accidentals.natural":
		return sense{kind: kAccidental, acc: model.Natural}
	case n == "accidentals.doublesharp":
		return sense{kind: kAccidental, acc: model.DoubleSharp}
	case n == "accidentals.flatflat":
		return sense{kind: kAccidental, acc: model.DoubleFlat}
	}
	return sense{}
}

// bottomPitch is the pitch of a staff's lowest line under a given clef.
//
// Everything read off a staff is counted from here, so getting it wrong
// transposes the whole part and nothing else looks amiss.
func bottomPitch(c model.Clef) model.Pitch {
	switch c {
	case model.Bass:
		return model.Pitch{Step: model.StepG, Octave: 2}
	case model.Alto:
		return model.Pitch{Step: model.StepF, Octave: 3}
	case model.Tenor:
		return model.Pitch{Step: model.StepD, Octave: 3}
	default: // treble
		return model.Pitch{Step: model.StepE, Octave: 4}
	}
}
