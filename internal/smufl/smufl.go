/*
 * smufl.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package smufl is the music font layer.
//
// SMuFL, the Standard Music Font Layout, assigns every notation glyph a
// canonical name and a codepoint in the Unicode private use area, and
// publishes the metrics an engraver needs alongside them. Working against the
// standard rather than one font means the font is swappable: Bravura ships
// here, and Leland, Petaluma or a user's own build drop in without the
// engraver changing.
//
// It replaces the 287 XPM bitmaps of gscore 0.0.9, which were fixed-size
// pictures with no metrics at all. Hand-drawn symbols are still supported for
// anything no font carries; see internal/symbol.
package smufl

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

//go:embed data/glyphnames.json
var glyphNamesJSON []byte

//go:embed data/Bravura.otf
var bravuraOTF []byte

// FontData returns the embedded font file, for a renderer that needs to carry
// the font with its output.
func FontData() []byte { return bravuraOTF }

// FontFamily is the name the embedded font is registered under.
const FontFamily = "Bravura"

type glyphEntry struct {
	Codepoint   string `json:"codepoint"`
	Description string `json:"description"`
}

var glyphs map[string]rune

func init() {
	var raw map[string]glyphEntry
	if err := json.Unmarshal(glyphNamesJSON, &raw); err != nil {
		panic("smufl: cannot read glyphnames.json: " + err.Error())
	}
	glyphs = make(map[string]rune, len(raw))
	for name, e := range raw {
		r, err := parseCodepoint(e.Codepoint)
		if err != nil {
			continue
		}
		glyphs[name] = r
	}
}

// parseCodepoint reads SMuFL's "U+E0A4" form.
func parseCodepoint(s string) (rune, error) {
	t := strings.TrimPrefix(strings.TrimSpace(s), "U+")
	v, err := strconv.ParseUint(t, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("smufl: bad codepoint %q", s)
	}
	return rune(v), nil
}

// Glyph returns the codepoint for a canonical SMuFL glyph name.
func Glyph(name string) (rune, bool) {
	r, ok := glyphs[name]
	return r, ok
}

// MustGlyph returns the codepoint for a name that is expected to exist.
func MustGlyph(name string) rune {
	r, ok := glyphs[name]
	if !ok {
		panic("smufl: unknown glyph " + name)
	}
	return r
}

// Count reports how many glyph names are known.
func Count() int { return len(glyphs) }

// EngravingDefaults holds the line weights and distances a font recommends,
// all in staff spaces.
//
// This is the same idea as the Spacings_t tree in legacy/src/include/spacings.h,
// which gscore had in 2004 before the standard existed: named, tunable
// engraving measurements rather than constants buried in drawing code. The
// names here follow SMuFL so a font can supply them.
type EngravingDefaults struct {
	StaffLineThickness         float64 `json:"staffLineThickness"`
	StemThickness              float64 `json:"stemThickness"`
	BeamThickness              float64 `json:"beamThickness"`
	BeamSpacing                float64 `json:"beamSpacing"`
	LegerLineThickness         float64 `json:"legerLineThickness"`
	LegerLineExtension         float64 `json:"legerLineExtension"`
	SlurEndpointThickness      float64 `json:"slurEndpointThickness"`
	SlurMidpointThickness      float64 `json:"slurMidpointThickness"`
	TieEndpointThickness       float64 `json:"tieEndpointThickness"`
	TieMidpointThickness       float64 `json:"tieMidpointThickness"`
	ThinBarlineThickness       float64 `json:"thinBarlineThickness"`
	ThickBarlineThickness      float64 `json:"thickBarlineThickness"`
	BarlineSeparation          float64 `json:"barlineSeparation"`
	RepeatBarlineDotSeparation float64 `json:"repeatBarlineDotSeparation"`
	HairpinThickness           float64 `json:"hairpinThickness"`
	OctaveLineThickness        float64 `json:"octaveLineThickness"`
	TupletBracketThickness     float64 `json:"tupletBracketThickness"`
}

// DefaultEngraving returns Bravura's published engraving defaults.
//
// They are built in rather than read from bravura_metadata.json because that
// file is not redistributed alongside the font, and an engraver with no
// measurements cannot draw anything at all. LoadMetadata overrides them when
// a metadata file is available.
func DefaultEngraving() EngravingDefaults {
	return EngravingDefaults{
		StaffLineThickness:         0.13,
		StemThickness:              0.12,
		BeamThickness:              0.5,
		BeamSpacing:                0.25,
		LegerLineThickness:         0.16,
		LegerLineExtension:         0.4,
		SlurEndpointThickness:      0.1,
		SlurMidpointThickness:      0.22,
		TieEndpointThickness:       0.1,
		TieMidpointThickness:       0.22,
		ThinBarlineThickness:       0.16,
		ThickBarlineThickness:      0.5,
		BarlineSeparation:          0.4,
		RepeatBarlineDotSeparation: 0.16,
		HairpinThickness:           0.16,
		OctaveLineThickness:        0.16,
		TupletBracketThickness:     0.16,
	}
}

// LoadMetadata merges a SMuFL font metadata file over the defaults, so a
// different font can supply its own measurements and advance widths.
func LoadMetadata(data []byte) (EngravingDefaults, map[string]float64, error) {
	var meta struct {
		EngravingDefaults  EngravingDefaults  `json:"engravingDefaults"`
		GlyphAdvanceWidths map[string]float64 `json:"glyphAdvanceWidths"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return DefaultEngraving(), nil, fmt.Errorf("smufl: bad metadata: %w", err)
	}
	return meta.EngravingDefaults, meta.GlyphAdvanceWidths, nil
}

// advanceWidths are Bravura's advance widths, in staff spaces, for the glyphs
// the engraver places. They let elements be spaced without measuring outlines.
var advanceWidths = map[string]float64{
	"gClef": 2.684, "fClef": 2.736, "cClef": 2.796, "unpitchedPercussionClef1": 2.0,
	"noteheadDoubleWhole": 2.512, "noteheadWhole": 1.688,
	"noteheadHalf": 1.18, "noteheadBlack": 1.18,
	"accidentalSharp": 0.98, "accidentalFlat": 0.76, "accidentalNatural": 0.672,
	"accidentalDoubleSharp": 0.98, "accidentalDoubleFlat": 1.472,
	"restDoubleWhole": 1.0, "restWhole": 1.52, "restHalf": 1.52,
	"restQuarter": 1.14, "rest8th": 1.0, "rest16th": 1.22,
	"rest32nd": 1.4, "rest64th": 1.6, "rest128th": 1.8,
	"augmentationDot": 0.4,
	"flag8thUp":       0.884, "flag8thDown": 0.884,
	"flag16thUp": 0.884, "flag16thDown": 0.884,
	"flag32ndUp": 0.884, "flag32ndDown": 0.884,
	"flag64thUp": 0.884, "flag64thDown": 0.884,
	"flag128thUp": 0.884, "flag128thDown": 0.884,
	"timeSig0": 1.372, "timeSig1": 1.372, "timeSig2": 1.372, "timeSig3": 1.372,
	"timeSig4": 1.372, "timeSig5": 1.372, "timeSig6": 1.372, "timeSig7": 1.372,
	"timeSig8": 1.372, "timeSig9": 1.372,
	"timeSigCommon": 1.664, "timeSigCutCommon": 1.664,
	"articAccentAbove": 1.188, "articAccentBelow": 1.188,
	"articStaccatoAbove": 0.304, "articStaccatoBelow": 0.304,
	"articTenutoAbove": 1.016, "articTenutoBelow": 1.016,
	"articMarcatoAbove": 0.918, "articMarcatoBelow": 0.918,
	"articStaccatissimoAbove": 0.32, "articStaccatissimoBelow": 0.32,
	"dynamicNiente": 1.0, "dynamicPPPP": 3.0, "dynamicPPP": 2.4,
	"dynamicPP": 1.8, "dynamicPiano": 1.03, "dynamicMP": 2.2,
	"dynamicMF": 2.4, "dynamicForte": 1.23, "dynamicFF": 2.1,
	"dynamicFFF": 3.0, "dynamicFFFF": 3.9,
	"dynamicSforzando1": 2.0, "dynamicSforzato": 1.9,
	"dynamicFortePiano": 2.3, "dynamicRinforzando2": 2.3,
	"fermataAbove": 1.584, "fermataBelow": 1.584,
	"stringsUpBow": 0.852, "stringsDownBow": 1.096,
}

// Advance returns a glyph's advance width in staff spaces. Unknown glyphs get
// one staff space, which keeps an unrecognised symbol from collapsing onto its
// neighbour.
func Advance(name string) float64 {
	if w, ok := advanceWidths[name]; ok {
		return w
	}
	return 1.0
}

/*
 * Mapping music onto glyph names
 */

// ClefGlyph returns the glyph for a clef, and the staff position its reference
// line sits on, counted in half spaces upward from the bottom staff line.
func ClefGlyph(c model.Clef) (name string, line float64) {
	switch c {
	case model.Bass:
		return "fClef", 6
	case model.Alto:
		return "cClef", 4
	case model.Tenor:
		return "cClef", 6
	case model.Percussion:
		return "unpitchedPercussionClef1", 4
	}
	return "gClef", 2
}

// NoteheadGlyph returns the notehead for a written note value.
func NoteheadGlyph(v model.NoteValue) string {
	switch {
	case v <= model.DoubleWhole:
		return "noteheadDoubleWhole"
	case v == model.Whole:
		return "noteheadWhole"
	case v == model.Half:
		return "noteheadHalf"
	}
	return "noteheadBlack"
}

// NoteheadGlyphOf returns the notehead for a value drawn in a given shape.
//
// Only the ordinary shape has a different glyph for each duration: an X is an
// X whether it is a crotchet or a minim, and the two that exist — filled and
// open — are what say the difference.
func NoteheadGlyphOf(v model.NoteValue, kind model.NoteheadKind) string {
	if kind == model.HeadNormal {
		return NoteheadGlyph(v)
	}
	open := v <= model.Half
	switch kind {
	case model.HeadX:
		if open {
			return "noteheadXHalf"
		}
		return "noteheadXBlack"
	case model.HeadCircleX:
		return "noteheadCircleX"
	case model.HeadTriangle:
		if open {
			return "noteheadTriangleUpHalf"
		}
		return "noteheadTriangleUpBlack"
	case model.HeadDiamond:
		if open {
			return "noteheadDiamondHalf"
		}
		return "noteheadDiamondBlack"
	case model.HeadSlash:
		if open {
			return "noteheadSlashHorizontalEnds"
		}
		return "noteheadSlashVerticalEnds"
	}
	return NoteheadGlyph(v)
}

// RestGlyph returns the rest for a written note value.
func RestGlyph(v model.NoteValue) string {
	switch v {
	case model.DoubleWhole:
		return "restDoubleWhole"
	case model.Whole:
		return "restWhole"
	case model.Half:
		return "restHalf"
	case model.Quarter:
		return "restQuarter"
	case model.Eighth:
		return "rest8th"
	case model.Sixteenth:
		return "rest16th"
	case model.ThirtySecond:
		return "rest32nd"
	case model.SixtyFourth:
		return "rest64th"
	}
	return "rest128th"
}

// FlagGlyph returns the flag for a note value, or "" where none is drawn.
func FlagGlyph(v model.NoteValue, up bool) string {
	if v <= model.Quarter {
		return ""
	}
	n := map[model.NoteValue]string{
		model.Eighth: "8th", model.Sixteenth: "16th", model.ThirtySecond: "32nd",
		model.SixtyFourth: "64th", model.HundredTwentyEighth: "128th",
	}[v]
	if n == "" {
		return ""
	}
	if up {
		return "flag" + n + "Up"
	}
	return "flag" + n + "Down"
}

// AccidentalGlyph returns the glyph for a written accidental.
func AccidentalGlyph(a model.Accidental) string {
	switch a {
	case model.Sharp:
		return "accidentalSharp"
	case model.Flat:
		return "accidentalFlat"
	case model.Natural:
		return "accidentalNatural"
	case model.DoubleSharp:
		return "accidentalDoubleSharp"
	case model.DoubleFlat:
		return "accidentalDoubleFlat"
	}
	return ""
}

// DynamicGlyph returns the glyph for a dynamic marking.
//
// SMuFL gives each of these a single glyph rather than letters set side by
// side, which is why a dynamic looks like engraving rather than like italics.
func DynamicGlyph(d model.Dynamic) string {
	switch d {
	case model.Niente:
		return "dynamicNiente"
	case model.PPPP:
		return "dynamicPPPP"
	case model.PPP:
		return "dynamicPPP"
	case model.PP:
		return "dynamicPP"
	case model.Piano:
		return "dynamicPiano"
	case model.MP:
		return "dynamicMP"
	case model.MF:
		return "dynamicMF"
	case model.Forte:
		return "dynamicForte"
	case model.FF:
		return "dynamicFF"
	case model.FFF:
		return "dynamicFFF"
	case model.FFFF:
		return "dynamicFFFF"
	case model.SForzando:
		return "dynamicSforzando1"
	case model.SForzato:
		return "dynamicSforzato"
	case model.FortePiano:
		return "dynamicFortePiano"
	case model.Rinforzando:
		return "dynamicRinforzando2"
	}
	return ""
}

// ArticulationGlyph returns the glyph for an articulation on the given side.
func ArticulationGlyph(a model.Articulation, above bool) string {
	side := "Below"
	if above {
		side = "Above"
	}
	switch a {
	case model.Staccato:
		return "articStaccato" + side
	case model.Staccatissimo:
		return "articStaccatissimo" + side
	case model.Accent:
		return "articAccent" + side
	case model.Tenuto:
		return "articTenuto" + side
	case model.Marcato:
		return "articMarcato" + side
	case model.MarcatoDown:
		// The downward wedge is a glyph of its own, not the same mark flipped.
		if above {
			return "articMarcatoAbove"
		}
		return "articMarcatoBelow"
	case model.Fermata:
		return "fermata" + side
	case model.UpBow:
		return "stringsUpBow"
	case model.DownBow:
		return "stringsDownBow"
	}
	return ""
}

// TimeSignatureGlyphs returns the glyph names for a time signature's numerator
// and denominator, or a single symbol glyph for common and cut time.
func TimeSignatureGlyphs(t model.TimeSignature) (num, den []string, symbol string) {
	switch t.Symbol {
	case model.TimeCommon:
		return nil, nil, "timeSigCommon"
	case model.TimeCut:
		return nil, nil, "timeSigCutCommon"
	}
	return digitGlyphs(t.Beats), digitGlyphs(t.BeatValue), ""
}

func digitGlyphs(n int) []string {
	s := strconv.Itoa(n)
	out := make([]string, 0, len(s))
	for _, c := range s {
		out = append(out, "timeSig"+string(c))
	}
	return out
}
