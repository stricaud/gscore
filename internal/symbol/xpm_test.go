package symbol

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decodeFile(t *testing.T, path string) *Bitmap {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()
	b, err := DecodeXPM(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return b
}

func TestDecodeNotehead(t *testing.T) {
	b := decodeFile(t, filepath.Join("testdata", "noteheads-2.xpm"))

	if b.W != 9 || b.H != 8 {
		t.Fatalf("got %dx%d, want 9x8", b.W, b.H)
	}

	// A notehead is one staff space tall, which is the check that the import
	// scale is right: 8 pixels at an eighth of a space each.
	if got := b.HeightInSpaces(); got != 1 {
		t.Errorf("height = %v staff spaces, want 1", got)
	}
	if got := b.WidthInSpaces(); got != 9.0/8.0 {
		t.Errorf("width = %v staff spaces, want 1.125", got)
	}

	// The origin defaults to the left edge, vertically centred.
	if b.OriginX != 0 || b.OriginY != 4 {
		t.Errorf("origin = (%v, %v), want (0, 4)", b.OriginX, b.OriginY)
	}

	// The file's two colours are " c None" and ". c #000000". The first row is
	// "   ......", so three transparent pixels then six black ones.
	black := color.NRGBA{0, 0, 0, 255}
	for x := 0; x < 3; x++ {
		if got := b.At(x, 0); got.A != 0 {
			t.Errorf("pixel (%d,0) = %v, want transparent", x, got)
		}
	}
	for x := 3; x < 9; x++ {
		if got := b.At(x, 0); got != black {
			t.Errorf("pixel (%d,0) = %v, want black", x, got)
		}
	}

	// The glyph is solid enough to be a notehead and not an empty grid.
	if n := b.Opaque(); n < 40 {
		t.Errorf("only %d opaque pixels in a 9x8 notehead", n)
	}
}

func TestDecodeGClefProportions(t *testing.T) {
	b := decodeFile(t, filepath.Join("testdata", "clefs-G.xpm"))
	if b.W != 18 || b.H != 53 {
		t.Fatalf("got %dx%d, want 18x53", b.W, b.H)
	}
	// A treble clef spans roughly seven staff spaces, reaching above and below
	// the staff. At the import scale this one comes to 6.625.
	if got := b.HeightInSpaces(); got < 6 || got > 7.5 {
		t.Errorf("height = %v staff spaces, want about 6.6", got)
	}
}

func TestOutOfRangeReadsAreTransparent(t *testing.T) {
	b := decodeFile(t, filepath.Join("testdata", "noteheads-2.xpm"))
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {9, 0}, {0, 8}, {100, 100}} {
		if got := b.At(p[0], p[1]); got != (color.NRGBA{}) {
			t.Errorf("At(%d,%d) = %v, want the zero colour", p[0], p[1], got)
		}
	}
}

func TestEveryFixtureDecodes(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.xpm"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			b := decodeFile(t, p)
			if b.Opaque() == 0 {
				t.Error("decoded to an entirely transparent grid")
			}
		})
	}
}

func TestColorForms(t *testing.T) {
	cases := []struct {
		spec string
		want color.NRGBA
	}{
		{"None", color.NRGBA{}},
		{"none", color.NRGBA{}},
		{"#000000", color.NRGBA{0, 0, 0, 255}},
		{"#FFFFFF", color.NRGBA{255, 255, 255, 255}},
		{"#FF0000", color.NRGBA{255, 0, 0, 255}},
		{"#f00", color.NRGBA{255, 0, 0, 255}},
		{"#FFFF00000000", color.NRGBA{255, 0, 0, 255}},
		{"black", color.NRGBA{0, 0, 0, 255}},
		{"White", color.NRGBA{255, 255, 255, 255}},
	}
	for _, c := range cases {
		got, err := parseXPMColor(c.spec)
		if err != nil {
			t.Errorf("%q: %v", c.spec, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q = %v, want %v", c.spec, got, c.want)
		}
	}
	if _, err := parseXPMColor("chartreuse"); err == nil {
		t.Error("an unknown colour name should be an error, not a guess")
	}
}

func TestMalformedInputIsRejected(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"not xpm", "this is not an xpm file at all"},
		{"bad header", `"nonsense"` + "\n"},
		{"truncated", `"4 4 1 1"` + "\n" + `" c None"` + "\n" + `"    "` + "\n"},
		{"undefined key", `"2 1 1 1"` + "\n" + `" c None"` + "\n" + `"XY"` + "\n"},
		{"short row", `"4 1 1 1"` + "\n" + `" c None"` + "\n" + `"  "` + "\n"},
	}
	for _, c := range cases {
		if _, err := DecodeXPM(strings.NewReader(c.src)); err == nil {
			t.Errorf("%s: decoded without error", c.name)
		}
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	if err := r.Add(nil); err == nil {
		t.Error("adding nil should fail")
	}
	if err := r.Add(&Symbol{}); err == nil {
		t.Error("adding an unnamed symbol should fail")
	}
	if err := r.Add(&Symbol{Name: "x", Kind: FromPixels}); err == nil {
		t.Error("a pixel symbol with no grid should fail")
	}
	if err := r.Add(&Symbol{Name: "x", Kind: FromVector}); err == nil {
		t.Error("a vector symbol with no path should fail")
	}

	glyph := &Symbol{Name: "noteheadBlack", Kind: FromFont, Codepoint: 0xE0A4}
	if err := r.Add(glyph); err != nil {
		t.Fatal(err)
	}
	drawn := &Symbol{Name: "myOwnMark", Kind: FromPixels, Pixels: NewBitmap(4, 4)}
	if err := r.Add(drawn); err != nil {
		t.Fatal(err)
	}

	if got, ok := r.Lookup("noteheadBlack"); !ok || got != glyph {
		t.Error("lookup of a font glyph failed")
	}
	if _, ok := r.Lookup("nothing"); ok {
		t.Error("lookup of an absent symbol succeeded")
	}
	if got, want := r.Names(), []string{"myOwnMark", "noteheadBlack"}; len(got) != 2 ||
		got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Names() = %v, want %v", got, want)
	}

	// A user drawing must be able to take over a standard name; that is how a
	// custom symbol replaces one the font already has.
	override := &Symbol{Name: "noteheadBlack", Kind: FromPixels, Pixels: NewBitmap(9, 8)}
	if err := r.Add(override); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Lookup("noteheadBlack"); got != override {
		t.Error("a later definition should win")
	}
	if r.Len() != 2 {
		t.Errorf("registry holds %d symbols, want 2", r.Len())
	}
}
