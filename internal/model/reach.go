/*
 * reach.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// Reach is how low and how high an instrument plays, as sounding MIDI
// numbers.
//
// Sounding, not written: the limit is a physical one — the length of a tube,
// the longest string, how far a hand reaches — and it does not move because
// the part is written in a transposing key. A clarinet's bottom note is the
// same note whichever way it is spelled.
type Reach struct {
	// Low and High are the ends of the instrument. Below one or above the
	// other there is no note to be had, however the music asks for it.
	Low, High int

	// EasyLow and EasyHigh are the stretch inside that where the
	// instrument plays well: the player is not working for the note, the
	// tone is even, and a long passage up there is no trouble.
	//
	// Writing outside it is not a mistake — the top of a trumpet part is
	// where the piece arrives, and that is the point of it. Writing
	// outside it for forty bars usually is, and it is the kind of mistake
	// nobody makes on purpose and nobody hears in their head.
	EasyLow, EasyHigh int
}

// Holds says whether a sounding pitch is one the instrument has at all.
func (r Reach) Holds(midi int) bool { return midi >= r.Low && midi <= r.High }

// Easy says whether a sounding pitch is in the comfortable stretch.
func (r Reach) Easy(midi int) bool { return midi >= r.EasyLow && midi <= r.EasyHigh }

// reaches is what each instrument gscore names can play.
//
// Conservative, and from ordinary orchestration practice rather than from
// what a virtuoso has been recorded doing: the top of a flute is given as D7
// because that is what an orchestra's flute section plays, not because
// nobody has ever played higher. The whole value of the check is that a
// composer who does not play the instrument can trust it, and a table that
// allows everything says nothing.
//
// An instrument not in here has no entry rather than a guessed one. What
// comes back then is the range the part actually covers and no opinion about
// it, which is the honest answer.
var reaches = map[Instrument]Reach{
	AcousticGrandPiano: {21, 108, 28, 96},
	BrightPiano:        {21, 108, 28, 96},
	ElectricPiano:      {28, 103, 36, 96},
	Harpsichord:        {29, 89, 29, 89},
	// Sounds an octave above what is written, so its written C3 to C7 is
	// this C4 to C8.
	Celesta:      {60, 108, 60, 108},
	Glockenspiel: {79, 108, 79, 108},
	MusicBox:     {72, 108, 72, 108},
	Vibraphone:   {53, 89, 53, 89},
	Marimba:      {36, 96, 41, 93},
	ChurchOrgan:  {36, 96, 36, 96},
	Accordion:    {41, 93, 45, 89},
	Harmonica:    {60, 96, 62, 93},
	// Guitars sound an octave below what is written; these are the sounding
	// notes, E2 upwards.
	AcousticGuitar: {40, 88, 40, 83},
	ElectricGuitar: {40, 88, 40, 83},
	AcousticBass:   {28, 60, 28, 52},

	Violin:     {55, 100, 55, 88},
	Viola:      {48, 88, 48, 81},
	Cello:      {36, 84, 36, 76},
	Contrabass: {28, 67, 28, 55},
	Harp:       {23, 104, 28, 100},
	// A set of drums, not one drum: this is what a player with four of
	// them in front of them can reach.
	Timpani: {38, 60, 40, 57},
	// No entry for the string ensemble, on purpose. It is a section patch
	// and not an instrument: a staff set to it may be the first violins
	// or the double basses, and a range running from the bass's bottom
	// string to the top of a violin is so wide that the only findings it
	// can produce are wrong. Beethoven's Contrabbassi, which MuseScore
	// maps to it, came back with a hundred and sixty-two notes supposedly
	// below the instrument — true of a double bass, meaningless as a
	// statement about "strings". A check that cannot be trusted is worse
	// than no check, so the part's own range is reported and no opinion
	// offered.
	// A mixed choir: the bass's bottom E to the soprano's top C. Given as
	// the whole choir because that is what the instrument is — a part
	// named "Soprano" and set to this gets the choir's range, which is the
	// honest answer to a question asked about a choir.
	ChoirAahs: {40, 84, 43, 79},
	VoiceOohs: {40, 84, 43, 79},

	Trumpet:    {52, 86, 55, 82},
	Trombone:   {40, 77, 43, 70},
	Tuba:       {26, 65, 29, 58},
	FrenchHorn: {35, 77, 43, 72},

	SopranoSax:  {56, 88, 58, 86},
	AltoSax:     {49, 80, 51, 77},
	TenorSax:    {44, 76, 46, 74},
	BaritoneSax: {37, 68, 39, 65},
	Oboe:        {58, 93, 60, 86},
	EnglishHorn: {52, 81, 55, 76},
	Bassoon:     {34, 76, 38, 72},
	Clarinet:    {50, 94, 52, 84},

	// Sounds an octave above what is written, so its written D4 to C7 is
	// this D5 to C8. Written an octave below that by mistake, the second
	// movement of Beethoven's ninth came back with thirty-six notes the
	// piccolo supposedly could not reach, every one of which it plays in
	// the middle of its range — which is how a table that is wrong looks
	// exactly like a score that is wrong.
	Piccolo:  {74, 108, 76, 103},
	Flute:    {60, 98, 62, 91},
	Recorder: {72, 98, 72, 96},
	PanFlute: {60, 96, 62, 93},
	Ocarina:  {69, 89, 69, 86},
}

// Reach returns what an instrument can play, and whether gscore knows.
func (i Instrument) Reach() (Reach, bool) {
	r, ok := reaches[i]
	return r, ok
}
