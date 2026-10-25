/*
 * jump.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strings"

// Jumps: the directions that send a player somewhere else in the score.
//
// Repeat signs and volta brackets handle going round again. Everything else —
// D.C. al Fine, D.S. al Coda, the sign, the tail — is a second set of
// directions laid over the first, and a player follows both at once.
//
// They are kept apart from repeats here because they behave differently. A
// repeat is taken as you come to it and taken again next time. A D.S. is
// taken once, and it changes how the rest of the piece is read: the written
// repeats are not observed on the way through again, and Fine and To Coda,
// which did nothing the first time, suddenly mean something. That second
// reading is the whole point of the notation, and it is why the directions
// cannot be modelled as another kind of barline.

// Sign is a place in the score that a direction refers to.
type Sign int

// The places a direction can send a player to or away from.
const (
	NoSign Sign = iota
	// Segno is the sign, which a D.S. sends you back to.
	Segno
	// Coda is the tail: where the music carries on after To Coda.
	Coda
	// Fine is where the piece ends on the way through again. In a piece
	// marked D.C. al Fine the last bar written is not the last bar played,
	// which is the whole reason the word is there.
	Fine
	// ToCoda is where you leave the music and go to the coda, on the way
	// through again.
	ToCoda
)

var signNames = [...]string{"none", "segno", "coda", "fine", "tocoda"}

func (s Sign) String() string {
	if s < 0 || int(s) >= len(signNames) {
		return "none"
	}
	return signNames[s]
}

// Label is how the sign is written on the page. The segno and the coda are
// drawn as their symbols, so they have no words of their own.
func (s Sign) Label() string {
	switch s {
	case Fine:
		return "Fine"
	case ToCoda:
		return "To Coda"
	}
	return ""
}

// Glyph is the SMuFL glyph for the sign, or the empty string where the sign
// is written as words.
func (s Sign) Glyph() string {
	switch s {
	case Segno:
		return "segno"
	case Coda, ToCoda:
		return "coda"
	}
	return ""
}

// SignFromName reads a sign written as a name, for the editor and for import.
func SignFromName(s string) (Sign, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none", "":
		return NoSign, true
	case "segno":
		return Segno, true
	case "coda":
		return Coda, true
	case "fine":
		return Fine, true
	case "tocoda", "to coda", "to-coda":
		return ToCoda, true
	}
	return NoSign, false
}

// JumpFrom says where a jump sends the player back to.
type JumpFrom int

const (
	// FromStart is da capo: back to the head of the piece.
	FromStart JumpFrom = iota
	// FromSegno is dal segno: back to the sign.
	FromSegno
)

// JumpUntil says where the music goes once the jump has been taken.
type JumpUntil int

const (
	// UntilEnd carries on to the last bar.
	UntilEnd JumpUntil = iota
	// UntilFine stops at the bar marked Fine.
	UntilFine
	// UntilCoda leaves the music at To Coda and goes on at the coda.
	UntilCoda
)

// Jump is a direction written at the end of a bar.
type Jump struct {
	From  JumpFrom
	Until JumpUntil
}

// Label spells the direction the way it is printed.
func (j *Jump) Label() string {
	if j == nil {
		return ""
	}
	head := "D.C."
	if j.From == FromSegno {
		head = "D.S."
	}
	switch j.Until {
	case UntilFine:
		return head + " al Fine"
	case UntilCoda:
		return head + " al Coda"
	}
	return head
}

// Name is the jump written as a name, for the editor and for saving.
func (j *Jump) Name() string {
	if j == nil {
		return "none"
	}
	head := "dc"
	if j.From == FromSegno {
		head = "ds"
	}
	switch j.Until {
	case UntilFine:
		return head + "-fine"
	case UntilCoda:
		return head + "-coda"
	}
	return head
}

// JumpFromName reads a jump written as a name. The second result is false for
// a name that is not one of them; "none" is a jump of no jump at all.
func JumpFromName(s string) (*Jump, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none", "":
		return nil, true
	case "dc", "d.c.", "da capo":
		return &Jump{FromStart, UntilEnd}, true
	case "dc-fine", "d.c. al fine":
		return &Jump{FromStart, UntilFine}, true
	case "dc-coda", "d.c. al coda":
		return &Jump{FromStart, UntilCoda}, true
	case "ds", "d.s.", "dal segno":
		return &Jump{FromSegno, UntilEnd}, true
	case "ds-fine", "d.s. al fine":
		return &Jump{FromSegno, UntilFine}, true
	case "ds-coda", "d.s. al coda":
		return &Jump{FromSegno, UntilCoda}, true
	}
	return nil, false
}
