/*
 * accidentals.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

/*
 * Which notes print an accidental.
 *
 * A note's pitch and the sign in front of it are two different things, which
 * is why Pitch.Alter and Note.Accidental are two different fields. Alter is
 * what sounds. The sign is what a reader needs to be told, and a reader needs
 * to be told only what they do not already know: the key signature at the top
 * of the line, and whatever has happened so far in this bar.
 *
 * Every file format states the first of those and none of them states the
 * second, so nothing a file says is the answer. A Finale file records how far
 * each note is altered from the key, which is true of the fourth F sharp in a
 * bar as much as the first; read as a sign to print, it prints four sharps
 * where one belongs. An ABC file writes ^F wherever the person typing it
 * found it clearer, and the second one in a bar is redundant. The music is
 * right in both and the page is wrong.
 *
 * So the sign is worked out here, from the music, by one rule:
 *
 *	a note prints an accidental when its alteration differs from what is
 *	remembered for that letter in that octave
 *
 * where what is remembered starts each bar as the key signature and is
 * changed by each note as it passes. That one comparison is the whole of it.
 * It prints the sharp that contradicts the key, and the natural that takes
 * back a sharp written earlier in the bar, and it prints neither of them
 * twice — three jobs that look separate and are not. Nothing here is a
 * special case, and that is the point: the special cases are what rot.
 */

// SpellAccidentals writes the accidental each note needs, over the whole
// score.
//
// It reads the music and writes nothing but Note.Accidental, so it is safe to
// run at any time and to run twice. A note marked Forced or Parens keeps the
// sign it has.
//
// Run it after reading a file and after any edit that can change the answer,
// which is nearly any edit at all: a changed pitch, a changed key, a bar
// inserted or removed, a note deleted. It is cheap enough not to think about.
func (s *Score) SpellAccidentals() {
	if s == nil {
		return
	}
	for _, group := range s.instrumentStaves() {
		spellStaves(s, group)
	}
}

// instrumentStaves groups the parts by the player reading them.
//
// What is remembered belongs to the instrument and not to the staff: a
// pianist reading a sharp in the left hand does not need telling again when
// the right hand plays the same note, because they are one reader following
// one line of music written on two staves. The same goes for the two voices
// on one staff, which fall out of this for free, since they are one part.
//
// A brace with its barlines run through the gap is what says two staves are
// one player — the same thing it says to a reader. A bracket does not: two
// bracketed flutes are two people, and a sharp in one tells the other
// nothing.
func (s *Score) instrumentStaves() [][]int {
	// Where each staff's player ends, for the staves that share one. Only
	// the first staff of a group carries an entry, so a walk in staff order
	// meets each player once and in the order they are printed.
	upto := map[int]int{}
	for _, g := range s.Groups {
		if g.Symbol != GroupBrace || !g.Barlines {
			continue
		}
		if g.First < 0 || g.Last >= len(s.Parts) || g.Last <= g.First {
			continue
		}
		if last, already := upto[g.First]; !already || g.Last > last {
			upto[g.First] = g.Last
		}
	}

	var out [][]int
	for i := 0; i < len(s.Parts); {
		last, grouped := upto[i]
		if !grouped {
			out = append(out, []int{i})
			i++
			continue
		}
		var player []int
		for ; i <= last; i++ {
			player = append(player, i)
		}
		out = append(out, player)
	}
	return out
}

// spellStaves walks one player's staves, bar by bar, together.
func spellStaves(s *Score, staves []int) {
	bars := 0
	for _, i := range staves {
		if n := len(s.Parts[i].Measures); n > bars {
			bars = n
		}
	}

	// The staves that take signs at all. A player reading notation over
	// tablature is one reader of two staves, and only the upper of them is
	// spelled — but the lower is a copy of it rather than music of its own,
	// so nothing is lost by leaving it out of the walk entirely.
	var written []int
	for _, i := range staves {
		if spellsAccidentals(s.Parts[i]) {
			written = append(written, i)
		}
	}
	staves = written
	if len(staves) == 0 {
		return
	}

	sp := &speller{}
	for bar := 0; bar < bars; bar++ {
		// Two staves of one player are spelled together only while they
		// are written in the same key. They always are in practice — a
		// brace joins two staves of one instrument — and where they are
		// not, sharing what is remembered between them would be sharing
		// it between two different readings. A transposing staff is
		// written in a key of its own, and that is the case this guards.
		shared := true
		var key KeySignature
		for n, i := range staves {
			k := s.Parts[i].KeyAt(bar)
			if n == 0 {
				key = k
			} else if k != key {
				shared = false
			}
		}

		if !shared {
			for _, i := range staves {
				sp.openBar(s.Parts[i].KeyAt(bar))
				sp.measure(s.Parts[i], bar)
			}
			// Nothing is carried across a bar that was read apart, since
			// there is no one state to carry.
			sp = &speller{}
			continue
		}

		sp.openBar(key)
		for _, i := range staves {
			sp.measure(s.Parts[i], bar)
		}
	}
}

// speller is what is remembered as the music is walked.
type speller struct {
	// base is the key signature: what every letter is, before this bar did
	// anything to it.
	base KeySignature
	// state is what each letter has become, in each octave, so far in this
	// bar. Keyed by letter and octave together, because an accidental
	// belongs to the note it is written on and not to the letter
	// everywhere: a sharp on the F below the staff says nothing about the
	// F two octaves above it.
	state map[[2]int]int
	// open are the ties waiting for the note they land on, with the
	// alteration they left at. A tie makes the note it arrives at a
	// continuation rather than a new note, and a continuation is not
	// announced.
	open map[[2]int]int
}

// openBar starts a new bar: what is remembered goes back to the key
// signature, and nothing a bar did carries into the next one.
//
// A tie does carry, because it is the one thing in notation that reaches
// across a barline by design.
func (sp *speller) openBar(key KeySignature) {
	sp.base = key
	sp.state = map[[2]int]int{}
}

// spellsAccidentals says whether signs are written on this staff at all.
//
// Two kinds of staff carry notes and no accidentals, and on both of them a
// sign is not a refinement but a mark that means nothing.
//
// Tablature says which fret to stop, and the fret is the pitch: a sharp
// beside a 3 tells a guitarist nothing they can act on, and the suite's
// eight tab staves grew fourteen of them the first time this ran.
//
// A drum staff's positions name instruments rather than pitches, so an
// accidental there would be a sharpened cymbal.
func spellsAccidentals(p *Part) bool {
	return p != nil && p.Tab == nil && !p.Unpitched()
}

// measure spells one bar of one staff.
func (sp *speller) measure(p *Part, bar int) {
	if bar >= len(p.Measures) {
		return
	}
	m := p.Measures[bar]
	if m == nil {
		return
	}
	// In the one order everything in the program agrees on. The engraver
	// walks a bar this way and so does the playback, for the same reason it
	// matters here: two walks that have to agree are two walks that will
	// stop agreeing.
	for _, v := range m.Ordered() {
		// A key change written inside the bar turns the page from here
		// on: every letter means what the new signature says it means,
		// and what the bar had done to them before the turn is behind
		// the double bar with the old key. Kept going instead, the first
		// four beats' accidentals would be announced again against a
		// signature that already has them.
		if mk := marksOf(v.El); mk != nil && mk.Key != nil {
			sp.openBar(*mk.Key)
		}
		switch el := v.El.(type) {
		case *Note:
			sp.note(el)
		case *Chord:
			// The grace notes in front of a chord come before all of it.
			sp.grace(el.Grace)
			for _, n := range el.Notes {
				sp.spell(n)
			}
		}
	}
}

func (sp *speller) note(n *Note) {
	sp.grace(n.Grace)
	sp.spell(n)
}

// grace spells the little notes leaning on a note.
//
// They take no time out of the bar but they are read before the note they
// lean on, and a sharp on one of them alters what follows exactly as any
// other sharp does.
func (sp *speller) grace(g *GraceGroup) {
	if g == nil {
		return
	}
	for _, n := range g.Notes {
		sp.spell(n)
	}
}

// spell is the rule, and the only place Note.Accidental is written.
func (sp *speller) spell(n *Note) {
	if n == nil {
		return
	}
	at := [2]int{int(n.Pitch.Step), n.Pitch.Octave}
	want := n.Pitch.Alter

	// A tied-to note is the same note still sounding, so it is not
	// announced again — unless it has been written down differently on
	// arrival. An F sharp tied to a G flat is one sound and two spellings,
	// and the second spelling has to be printed or the page says a G
	// natural.
	if n.TieStop {
		if was, tied := sp.open[at]; tied && was == want {
			if !n.Forced && !n.Parens {
				n.Accidental = NoAccidental
			}
			// And it alters nothing, which is the part that is easy to
			// get wrong. A tie carries the sound across the barline but
			// not the announcement: nothing is printed in the new bar to
			// say this note is sharp, so a later separate note of the
			// same letter has to be told afresh. Remembering it here
			// would quietly undo the reset the barline just did, and a
			// reader would meet an F with no sign on it in a bar where
			// no sharp was ever printed.
			sp.ties(n, at, want)
			return
		}
	}

	if n.Forced || n.Parens {
		// Somebody has said what goes here, or the file the music came
		// from has. Either way it stands — but it still alters the bar,
		// so what is remembered follows it.
		sp.remember(n, at, want)
		return
	}

	have, seen := sp.state[at]
	if !seen {
		have = sp.base.AlterFor(n.Pitch.Step)
	}

	if want == have {
		n.Accidental = NoAccidental
		sp.remember(n, at, want)
		return
	}

	sign := accidentalFor(want)
	if sign == NoAccidental && want != 0 {
		// Altered further than any accidental gscore can draw. The sign
		// that is there is left alone rather than rubbed out, since
		// removing it would leave nothing at all to say the note is not
		// what the key signature makes it.
		sp.remember(n, at, want)
		return
	}
	n.Accidental = sign
	sp.remember(n, at, want)
}

// remember records what this note did to its letter, and keeps the ties.
func (sp *speller) remember(n *Note, at [2]int, alter int) {
	if sp.state == nil {
		sp.state = map[[2]int]int{}
	}
	sp.state[at] = alter
	sp.ties(n, at, alter)
}

// ties keeps track of the ties waiting for the notes they land on.
//
// Kept apart from remember because a note that a tie lands on does the one
// without the other: it is already sounding, so it alters nothing, and it
// still ends its tie and may begin another.
func (sp *speller) ties(n *Note, at [2]int, alter int) {
	// A tie that lands is spent; one that starts waits for its note, which
	// may be in the next bar.
	if n.TieStop && sp.open != nil {
		delete(sp.open, at)
	}
	if n.TieStart {
		if sp.open == nil {
			sp.open = map[[2]int]int{}
		}
		sp.open[at] = alter
	}
}

// accidentalFor is the sign that says a note is altered this far.
//
// A natural is the sign for no alteration at all, which is why this is not
// quite a table of the obvious: 0 prints something, and it is the one case
// where the sign and the alteration do not look alike.
func accidentalFor(alter int) Accidental {
	switch alter {
	case 0:
		return Natural
	case 1:
		return Sharp
	case -1:
		return Flat
	case 2:
		return DoubleSharp
	case -2:
		return DoubleFlat
	}
	return NoAccidental
}

// AlterInForce is what a note written at a point in a bar is altered by when
// nobody has said otherwise: whatever the bar has already done to that letter
// in that octave, and the key signature where it has done nothing.
//
// This is the reading rule turned round. SpellAccidentals asks whether a note
// needs a sign in front of it; a staff being written on asks the same
// question from the other end, because the person writing is reading as they
// go. A B written after a B flat in the same bar is a B flat — that is what
// the player in front of it will read, and it is what the writer meant, or
// they would have said otherwise.
//
// Answered with the key signature alone it comes out a B natural; and then,
// correctly, a natural sign is printed in front of it to say so. Which is the
// part that reads as a bug rather than a disagreement: a sign appears that
// nobody asked for, on a note nobody altered, and the only way to be rid of
// it is to flatten the note that was flat already.
//
// voice is which line of music on the staff, and at is where in that line the
// note is going. Everything that starts before it is read first.
func (p *Part) AlterInForce(bar, voice, at int, pitch Pitch) int {
	if p == nil {
		return 0
	}
	key := p.KeyAt(bar)
	// A staff that prints no accidentals alters nothing: a fret number and
	// a drum are not sharp or flat.
	if !spellsAccidentals(p) {
		return 0
	}
	plain := key.AlterFor(pitch.Step)
	if bar < 0 || bar >= len(p.Measures) || p.Measures[bar] == nil {
		return plain
	}
	m := p.Measures[bar]
	if voice < 0 || voice >= len(m.Voices) || m.Voices[voice] == nil {
		return plain
	}

	// When the new note starts, which is what decides how much of the bar
	// is in front of it. Notes in another voice that start at the same
	// moment are beside it rather than before it, and are left out: they
	// are drawn in the same column, and which of them a reader meets first
	// is not a thing the music says.
	start := Duration{Num: 0, Den: 1}
	for i, el := range m.Voices[voice].Elements {
		if i >= at {
			break
		}
		start = start.Add(el.Duration())
	}

	// Reading, not writing: nothing here touches a note. What a note did to
	// its letter is its alteration — true of a tied note and of a grace
	// note as much as of any other, since a reader meets them all the same
	// way — and the last one before this point is what stands.
	want := [2]int{int(pitch.Step), pitch.Octave}
	alter := plain
	read := func(n *Note) {
		if n == nil {
			return
		}
		if [2]int{int(n.Pitch.Step), n.Pitch.Octave} == want {
			alter = n.Pitch.Alter
		}
	}
	readGrace := func(g *GraceGroup) {
		if g == nil {
			return
		}
		for _, n := range g.Notes {
			read(n)
		}
	}
	for _, v := range m.Ordered() {
		if v.At.Cmp(start) >= 0 {
			break
		}
		switch el := v.El.(type) {
		case *Note:
			readGrace(el.Grace)
			read(el)
		case *Chord:
			readGrace(el.Grace)
			for _, n := range el.Notes {
				read(n)
			}
		}
	}
	return alter
}
