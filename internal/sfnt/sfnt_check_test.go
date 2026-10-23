// An external test package: it reads the font through smufl, and smufl now
// measures glyph widths through sfnt. Testing from outside breaks the cycle
// that would otherwise make, which is what an external test package is for.
package sfnt_test

import (
	"testing"

	"github.com/stricaud/gscore/internal/sfnt"
	"github.com/stricaud/gscore/internal/smufl"
)

func TestBravura(t *testing.T) {
	f, err := sfnt.Parse(smufl.FontData())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("name=%q upem=%d glyphs=%d ascent=%d descent=%d cap=%d bbox=%d,%d,%d,%d cff=%v",
		f.PostScriptName, f.UnitsPerEm, f.NumGlyphs, f.Ascent, f.Descent,
		f.CapHeight, f.XMin, f.YMin, f.XMax, f.YMax, f.IsCFF)
	for _, r := range []rune{0xE050, 0xE0A4, 0xE1D5, 0xE084} {
		g := f.GlyphIndex(r)
		if g == 0 {
			t.Errorf("U+%04X not in the font", r)
		}
		t.Logf("U+%04X -> glyph %d, advance %d", r, g, f.Advance(g))
	}
}
