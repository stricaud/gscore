package smufl

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestGlyphNamesLoaded(t *testing.T) {
	if n := Count(); n < 2000 {
		t.Fatalf("only %d glyph names loaded, expected the full SMuFL set", n)
	}
	// Spot-check the codepoints the standard fixes, so a bad metadata file is
	// caught rather than quietly producing the wrong characters.
	cases := map[string]rune{
		"gClef":           0xE050,
		"fClef":           0xE062,
		"noteheadBlack":   0xE0A4,
		"noteheadWhole":   0xE0A2,
		"accidentalSharp": 0xE262,
		"accidentalFlat":  0xE260,
		"restQuarter":     0xE4E5,
		"timeSig4":        0xE084,
		"augmentationDot": 0xE1E7,
	}
	for name, want := range cases {
		got, ok := Glyph(name)
		if !ok {
			t.Errorf("%s is missing", name)
			continue
		}
		if got != want {
			t.Errorf("%s = U+%04X, want U+%04X", name, got, want)
		}
	}
	if _, ok := Glyph("noSuchGlyphAnywhere"); ok {
		t.Error("an unknown name should not resolve")
	}
}

func TestFontEmbedded(t *testing.T) {
	d := FontData()
	if len(d) < 100000 {
		t.Fatalf("the embedded font is %d bytes, which cannot be right", len(d))
	}
	// OpenType with CFF outlines starts with the tag "OTTO".
	if string(d[:4]) != "OTTO" {
		t.Errorf("the embedded font starts with %q, want an OpenType signature", d[:4])
	}
}

func TestClefGlyphs(t *testing.T) {
	cases := []struct {
		clef model.Clef
		name string
		line float64
	}{
		{model.Treble, "gClef", 2},
		{model.Bass, "fClef", 6},
		{model.Alto, "cClef", 4},
		{model.Tenor, "cClef", 6},
	}
	for _, c := range cases {
		name, line := ClefGlyph(c.clef)
		if name != c.name || line != c.line {
			t.Errorf("%v = %s on line %v, want %s on %v", c.clef, name, line, c.name, c.line)
		}
		if _, ok := Glyph(name); !ok {
			t.Errorf("%v maps to %s, which the font has no glyph for", c.clef, name)
		}
	}
}

func TestNoteheadsAndRestsResolve(t *testing.T) {
	for v := model.DoubleWhole; v <= model.HundredTwentyEighth; v++ {
		head := NoteheadGlyph(v)
		if _, ok := Glyph(head); !ok {
			t.Errorf("%v gives notehead %q, which is not a glyph", v, head)
		}
		rest := RestGlyph(v)
		if _, ok := Glyph(rest); !ok {
			t.Errorf("%v gives rest %q, which is not a glyph", v, rest)
		}
	}
	// Only the short values get a notehead with a filled head.
	if NoteheadGlyph(model.Half) == NoteheadGlyph(model.Quarter) {
		t.Error("a half and a quarter should not share a notehead")
	}
}

func TestFlags(t *testing.T) {
	// Nothing longer than a quarter carries a flag.
	for _, v := range []model.NoteValue{model.DoubleWhole, model.Whole, model.Half, model.Quarter} {
		if g := FlagGlyph(v, true); g != "" {
			t.Errorf("%v should have no flag, got %q", v, g)
		}
	}
	for _, v := range []model.NoteValue{model.Eighth, model.Sixteenth, model.ThirtySecond, model.SixtyFourth} {
		for _, up := range []bool{true, false} {
			g := FlagGlyph(v, up)
			if g == "" {
				t.Errorf("%v up=%v has no flag", v, up)
				continue
			}
			if _, ok := Glyph(g); !ok {
				t.Errorf("%v up=%v gives %q, which is not a glyph", v, up, g)
			}
		}
	}
	if FlagGlyph(model.Eighth, true) == FlagGlyph(model.Eighth, false) {
		t.Error("up and down flags must differ")
	}
}

func TestAccidentalsAndArticulations(t *testing.T) {
	for _, a := range []model.Accidental{
		model.Sharp, model.Flat, model.Natural, model.DoubleSharp, model.DoubleFlat,
	} {
		g := AccidentalGlyph(a)
		if g == "" {
			t.Errorf("%v has no glyph name", a)
			continue
		}
		if _, ok := Glyph(g); !ok {
			t.Errorf("%v gives %q, which is not a glyph", a, g)
		}
	}
	if AccidentalGlyph(model.NoAccidental) != "" {
		t.Error("no accidental should give no glyph")
	}

	for _, art := range []model.Articulation{
		model.Staccato, model.Accent, model.Tenuto, model.Marcato,
		model.Fermata, model.UpBow, model.DownBow,
	} {
		for _, above := range []bool{true, false} {
			g := ArticulationGlyph(art, above)
			if g == "" {
				t.Errorf("%v above=%v has no glyph name", art, above)
				continue
			}
			if _, ok := Glyph(g); !ok {
				t.Errorf("%v above=%v gives %q, which is not a glyph", art, above, g)
			}
		}
	}
}

func TestTimeSignatureGlyphs(t *testing.T) {
	num, den, sym := TimeSignatureGlyphs(model.TimeSignature{Beats: 4, BeatValue: 4})
	if sym != "" {
		t.Errorf("a numeric meter should have no symbol, got %q", sym)
	}
	if len(num) != 1 || num[0] != "timeSig4" || len(den) != 1 || den[0] != "timeSig4" {
		t.Errorf("4/4 gave %v over %v", num, den)
	}

	// Two digits each need their own glyph.
	num, den, _ = TimeSignatureGlyphs(model.TimeSignature{Beats: 12, BeatValue: 16})
	if len(num) != 2 || len(den) != 2 {
		t.Errorf("12/16 gave %v over %v, want two digits each", num, den)
	}

	_, _, sym = TimeSignatureGlyphs(model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeCommon})
	if sym != "timeSigCommon" {
		t.Errorf("common time gave %q", sym)
	}
	_, _, sym = TimeSignatureGlyphs(model.TimeSignature{Beats: 2, BeatValue: 2, Symbol: model.TimeCut})
	if sym != "timeSigCutCommon" {
		t.Errorf("cut time gave %q", sym)
	}
}

func TestAdvanceWidths(t *testing.T) {
	// A notehead is a little over one staff space wide; the engraver depends
	// on these being in staff spaces, not font units.
	if w := Advance("noteheadBlack"); w < 0.8 || w > 2 {
		t.Errorf("noteheadBlack advance = %v, which is not in staff spaces", w)
	}
	if w := Advance("gClef"); w < 2 || w > 4 {
		t.Errorf("gClef advance = %v", w)
	}
	// An unknown glyph must still take room, or symbols would pile up.
	if w := Advance("somethingUnknown"); w <= 0 {
		t.Errorf("an unknown glyph got advance %v, want a positive fallback", w)
	}
}

func TestEngravingDefaults(t *testing.T) {
	d := DefaultEngraving()
	// These are Bravura's published values; a zero would make a line vanish.
	checks := map[string]float64{
		"staffLineThickness": d.StaffLineThickness,
		"stemThickness":      d.StemThickness,
		"beamThickness":      d.BeamThickness,
		"legerLineThickness": d.LegerLineThickness,
		"thinBarline":        d.ThinBarlineThickness,
		"thickBarline":       d.ThickBarlineThickness,
	}
	for name, v := range checks {
		if v <= 0 || v > 1 {
			t.Errorf("%s = %v, which is not a sane staff-space thickness", name, v)
		}
	}
	if d.ThickBarlineThickness <= d.ThinBarlineThickness {
		t.Error("the thick barline must be thicker than the thin one")
	}
	if d.BeamThickness <= d.StemThickness {
		t.Error("a beam must be thicker than a stem")
	}
}

func TestLoadMetadataOverrides(t *testing.T) {
	meta := []byte(`{"engravingDefaults":{"stemThickness":0.25},
		"glyphAdvanceWidths":{"noteheadBlack":1.5}}`)
	d, widths, err := LoadMetadata(meta)
	if err != nil {
		t.Fatal(err)
	}
	if d.StemThickness != 0.25 {
		t.Errorf("stemThickness = %v, want the file's 0.25", d.StemThickness)
	}
	if widths["noteheadBlack"] != 1.5 {
		t.Errorf("advance width not read back: %v", widths)
	}
	if _, _, err := LoadMetadata([]byte("not json")); err == nil {
		t.Error("bad metadata should be an error")
	}
}

func TestDynamicGlyphsResolve(t *testing.T) {
	// Every dynamic the model knows must have a glyph in the font, or it
	// would be silently dropped from the page.
	for d := model.Niente; d <= model.Rinforzando; d++ {
		name := DynamicGlyph(d)
		if name == "" {
			t.Errorf("%v has no glyph name", d)
			continue
		}
		if _, ok := Glyph(name); !ok {
			t.Errorf("%v gives %q, which the font does not have", d, name)
		}
	}
	if DynamicGlyph(model.DynamicNone) != "" {
		t.Error("no dynamic should give no glyph")
	}
	// A dynamic is wider than a notehead and the engraver spaces by these.
	if w := Advance("dynamicFFF"); w < 2 {
		t.Errorf("dynamicFFF advance = %v, too narrow to be right", w)
	}
}

// TestAdvanceTableMatchesTheFont keeps the cached widths honest.
//
// The table above exists so the common path is a map lookup, not a font
// parse. A number in it that disagrees with the font is worse than no number
// at all: the spacing would be wrong everywhere and the font would say
// otherwise if anybody asked it.
func TestAdvanceTableMatchesTheFont(t *testing.T) {
	for name, want := range advanceWidths {
		got, ok := measured(name)
		if !ok {
			t.Errorf("%s is in the table and not in the font", name)
			continue
		}
		if diff := got - want; diff > 0.01 || diff < -0.01 {
			t.Errorf("%s: the table says %.3f and the font says %.3f", name, want, got)
		}
	}
}

// TestAnUnlistedGlyphIsMeasured is the check that would have caught the coda
// sign being guessed at a staff space when it is nearly four.
func TestAnUnlistedGlyphIsMeasured(t *testing.T) {
	for _, name := range []string{"segno", "coda"} {
		if _, listed := advanceWidths[name]; listed {
			continue // fine either way; the test above covers it
		}
		if got := Advance(name); got <= 1.0 {
			t.Errorf("%s measures %.3f, which is the fallback: it was not looked up", name, got)
		}
	}
}

// TestAnUnknownNameStillGetsRoom: a symbol that is in no font at all must not
// collapse onto its neighbour.
func TestAnUnknownNameStillGetsRoom(t *testing.T) {
	if got := Advance("nosuchglyphanywhere"); got != 1.0 {
		t.Errorf("an unknown glyph measures %.3f, want the one-space fallback", got)
	}
}
