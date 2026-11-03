/*
 * technique.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// How a note is played on a fretted instrument.
//
// A tab says which string and how far up it, and nothing else — which is
// enough for a scale and nothing like enough for a riff. The difference
// between picking two notes and hammering the second one is the difference
// between the part as written and the part as played, and a tab has its own
// shorthand for every one of those differences: 5h7 is a hammer-on, 7p5 a
// pull-off, 7b9 a bend, 5/7 a slide.
//
// They are not all guitar's own. Vibrato, let ring and the harmonics are
// written on wind and string parts too, and are here because this is where a
// guitarist looks for them rather than because they belong to the guitar.
//
// What is not here is what gscore already had. A trill is Marks.Trill, which
// can last; tremolo picking is Marks.Tremolo; an accent, a staccato dot and a
// fermata are Articulations; a dead note is Head set to HeadX, which the drum
// staves already use and which the tab draws as the x it is. Adding a second
// way to say any of those would have given a score two marks that mean the
// same thing and no way to tell which one a file meant.

// Technique is one playing instruction attached to a note.
type Technique int

const (
	// The four that join this note to the next one. See Links.
	//
	// They are written between the two fret numbers because that is what
	// they are about: a hammer-on is not something done to a note, it is
	// how you get from one note to the one after it.
	HammerOn Technique = iota
	PullOff
	SlideUp
	SlideDown

	// Bends, which belong to the one note: the string is pushed after it
	// is struck, so the pitch rises without anything being picked again.
	//
	// A bend is written by how far it goes rather than to what, because
	// that is what the hand does — "full" is a whole tone and "1/2" a
	// semitone, whatever fret it starts from.
	BendFull
	BendHalf
	// BendRelease is bent, then let back down to where it started.
	BendRelease
	// PreBend is bent before it is struck, so the note sounds already
	// high and the bend is never heard as a bend.
	PreBend

	// Ways of sounding the one note.
	Vibrato
	WideVibrato
	PalmMute
	LetRing
	NaturalHarmonic
	ArtificialHarmonic
	PinchHarmonic
	Tap
)

var techniqueNames = [...]string{
	"hammer-on", "pull-off", "slide-up", "slide-down",
	"bend", "bend-half", "bend-release", "pre-bend",
	"vibrato", "wide-vibrato", "palm-mute", "let-ring",
	"natural-harmonic", "artificial-harmonic", "pinch-harmonic", "tap",
}

// techniqueLabels are what is printed on the page.
//
// Short, because they are set between two fret numbers a quarter of an inch
// apart or crowded over a run of sixteenths, and the long forms — "hammer
// on", "palm mute" — are what the legend at the front of the book is for.
// These are the abbreviations the guitar books agree on.
var techniqueLabels = [...]string{
	"H", "P", "", "",
	"full", "1/2", "", "",
	"", "", "P.M.", "let ring",
	"N.H.", "A.H.", "P.H.", "T",
}

func (t Technique) String() string {
	if t < HammerOn || int(t) >= len(techniqueNames) {
		return "unknown"
	}
	return techniqueNames[t]
}

// Label is the text printed for the mark, or "" for one that is drawn
// rather than written: a slide is a stroke between the numbers, and vibrato
// a wave over them.
func (t Technique) Label() string {
	if t < HammerOn || int(t) >= len(techniqueLabels) {
		return ""
	}
	return techniqueLabels[t]
}

// TechniqueFromName is the inverse of String, for files and query strings.
func TechniqueFromName(s string) (Technique, bool) {
	for i, n := range techniqueNames {
		if n == s {
			return Technique(i), true
		}
	}
	return HammerOn, false
}

// Links says the mark is about getting from this note to the next one
// rather than about this note on its own.
//
// It decides where the mark is drawn — between the two notes rather than
// over the one — and it decides what a mark on the last note of a part
// means, which is nothing: there is no next note to hammer onto.
func (t Technique) Links() bool {
	return t >= HammerOn && t <= SlideDown
}

// Bends says the mark raises the pitch of the note after it is struck,
// which is drawn as an arrow leaving the note rather than as a word over it.
func (t Technique) Bends() bool {
	return t >= BendFull && t <= PreBend
}

// AddTechnique records a technique, ignoring a repeat of one already there.
//
// A note can carry more than one — palm-muted and hammered onto, or bent
// and then given vibrato — so these accumulate the way articulations do
// rather than replacing each other.
func (m *Marks) AddTechnique(t Technique) {
	for _, have := range m.Techniques {
		if have == t {
			return
		}
	}
	m.Techniques = append(m.Techniques, t)
}

// HasTechnique reports whether the mark is present.
func (m *Marks) HasTechnique(t Technique) bool {
	for _, have := range m.Techniques {
		if have == t {
			return true
		}
	}
	return false
}

// RemoveTechnique takes one away, and says whether it was there.
func (m *Marks) RemoveTechnique(t Technique) bool {
	for i, have := range m.Techniques {
		if have == t {
			m.Techniques = append(m.Techniques[:i], m.Techniques[i+1:]...)
			return true
		}
	}
	return false
}

// LinkTechniques are the four that reach to the next note, and
// BendTechniques the four that raise a note's pitch after it is struck.
//
// They are groups a note can only hold one of — it is hammered onto or slid
// from, not both, and it bends one distance — so whatever applies one has
// to be able to ask which others it displaces.
func LinkTechniques() []Technique {
	return []Technique{HammerOn, PullOff, SlideUp, SlideDown}
}

func BendTechniques() []Technique {
	return []Technique{BendFull, BendHalf, BendRelease, PreBend}
}

// Techniques is every mark there is, in the order they are offered.
func Techniques() []Technique {
	out := make([]Technique, len(techniqueNames))
	for i := range out {
		out[i] = Technique(i)
	}
	return out
}
