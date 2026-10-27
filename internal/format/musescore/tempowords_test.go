package musescore

import "testing"

// MuseScore writes the metronome mark as text with a real note character in
// it, and the words beside a tempo are set in a text font that has no glyph
// for one. It was drawn as an empty box.
func TestANoteSymbolIsNotLeftInTheTempoWords(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\U0001D15F = 72 ", ""}, // the Goldberg file, exactly
		{"\U0001D15F. = 90", ""}, // dotted
		{" = 108", ""},          // Bravura's own codepoint
		{"Andante \U0001D15F = 72", "Andante"},
		{"Allegro", "Allegro"},
		{"(Aria)", "Aria"},
	}
	for _, c := range cases {
		if got := tempoWords(c.in); got != c.want {
			t.Errorf("tempoWords(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
