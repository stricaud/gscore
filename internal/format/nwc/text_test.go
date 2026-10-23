package nwc

import "testing"

// Names written before UTF-8 existed still have to come out as names.
func TestDecodeText(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"plain ascii", []byte("Treble"), "Treble"},
		{"already utf-8", []byte("Flûte"), "Flûte"},
		// One byte for the û, as a machine of the period wrote it.
		{"latin-1", []byte{'F', 'l', 0xFB, 't', 'e'}, "Flûte"},
		{"latin-1 accents", []byte{'V', 'i', 'o', 'l', 'o', 'n', 'c', 'e', 'l', 'l', 0xE9}, "Violoncellé"},
		// Windows-1252 put real punctuation where Latin-1 has control codes.
		{"cp1252 quotes", []byte{0x93, 'A', 0x94}, "“A”"},
		{"empty", []byte{}, ""},
	}
	for _, c := range cases {
		if got := decodeText(c.in); got != c.want {
			t.Errorf("%s: decodeText(% 02X) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// Telling a name in another language from note data read as text.
func TestLooksLikeText(t *testing.T) {
	yes := []string{
		"", "Treble", "1st violins", "Flûte", "Violoncello",
		"フルート", // a name in another script is still a name
		"Horn in F\t",
	}
	for _, s := range yes {
		if !looksLikeText(s) {
			t.Errorf("looksLikeText(%q) said no", s)
		}
	}
	// These are the actual bytes the reader produced from a desynced walk.
	no := []string{"\xff\b", "\x05\r\b", "\x00", "A\x01B", "\x7f"}
	for _, s := range no {
		if looksLikeText(s) {
			t.Errorf("looksLikeText(%q) said yes", s)
		}
	}
}
