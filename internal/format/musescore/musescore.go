/*
 * musescore.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package musescore reads MuseScore's own files, .mscz and .mscx.
//
// MuseScore writes MusicXML too, and internal/format/musicxml reads it. The
// reason for this package is that nobody keeps their work as MusicXML: a
// score on disk is a .mscz, and asking somebody to open MuseScore and export
// it first is asking them to not bother. A file manager double-click should
// open the file that is there.
//
// What the format is. A .mscz is a zip holding one .mscx — the score as XML —
// beside a style sheet, a thumbnail and some playback settings. The .mscx
// names its version in the root element: this reads 4.x, which is what
// MuseScore 4 writes, and gets most of the way through 2.x and 3.x as well,
// since the parts of the format that carry the notes have barely moved. Where
// it meets something it has no idea about it writes the tag down and carries
// on, because a file is worth opening for the music that is in it.
//
// Two things it is not. It is not a writer: MuseScore's format is an internal
// serialisation that changes between releases, and writing it would be a
// treadmill — MusicXML is the way out. And it is not complete, because the
// format is not a published standard with an end to it; what is missing is
// reported rather than guessed at.
package musescore

import (
	"fmt"
	"io"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string         { return "MuseScore" }
func (importer) Extensions() []string { return []string{".mscz", ".mscx"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// PartialError reports what in the file could not be read.
//
// A score is a long file and gscore models less of music than MuseScore does.
// Refusing the whole thing over a tremolo would be the wrong trade, so what
// was understood comes back with a list of what was not, which the editor
// shows as a warning.
type PartialError struct {
	Skipped []string
}

func (e *PartialError) Error() string {
	if len(e.Skipped) == 1 {
		return "musescore: " + e.Skipped[0]
	}
	return fmt.Sprintf("musescore: %d things could not be read, the first being: %s",
		len(e.Skipped), e.Skipped[0])
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details implements format.Partial.
func (e *PartialError) Details() []string { return e.Skipped }

/*
 * Tonal pitch classes
 *
 * MuseScore spells a note with two numbers: <pitch> is what sounds, as a MIDI
 * number, and <tpc> is its tonal pitch class, a position on the circle of
 * fifths that says which letter it is written as. The second is needed
 * because the first cannot tell C sharp from D flat, and a score that spells
 * them the same way is wrong on the page however right it sounds.
 */

// tpcLetters is the circle of fifths the class counts along, starting where
// MuseScore starts it: class 0 is C flat flat, 14 is C, 20 is F sharp.
var tpcLetters = [7]model.Step{
	model.StepF, model.StepC, model.StepG, model.StepD,
	model.StepA, model.StepE, model.StepB,
}

// semitonesFromC is each letter's distance above C, for putting a spelling
// back together with the MIDI number it has to agree with.
var semitonesFromC = [7]int{0, 2, 4, 5, 7, 9, 11}

// tpcPitch spells a MIDI note the way its tonal pitch class says it is
// written.
//
// The octave comes from the two together rather than from the MIDI number
// alone. B sharp and C flat are written in one octave and sound in the next,
// so taking the octave from the number would spell them an octave out — which
// is a note in the wrong place on the staff, not a detail.
func tpcPitch(tpc, midi int) (model.Pitch, bool) {
	if tpc < -1 || tpc > 33 {
		return model.Pitch{}, false
	}
	i := tpc + 1
	step := tpcLetters[i%7]
	alter := i/7 - 2
	octave := floorDiv(midi-alter-semitonesFromC[step], 12) - 1
	return model.Pitch{Step: step, Octave: octave, Alter: alter}, true
}

// midiPitch is the fallback when a note carries no tonal pitch class: spell it
// with sharps, which is what every program does when it has nothing to go on.
func midiPitch(midi int) model.Pitch {
	sharp := [12]struct {
		step  model.Step
		alter int
	}{
		{model.StepC, 0}, {model.StepC, 1}, {model.StepD, 0}, {model.StepD, 1},
		{model.StepE, 0}, {model.StepF, 0}, {model.StepF, 1}, {model.StepG, 0},
		{model.StepG, 1}, {model.StepA, 0}, {model.StepA, 1}, {model.StepB, 0},
	}
	n := sharp[((midi%12)+12)%12]
	return model.Pitch{Step: n.step, Octave: floorDiv(midi, 12) - 1, Alter: n.alter}
}

// floorDiv divides rounding downwards, which is what pitch arithmetic needs
// and what Go's / does not do for negative numbers.
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

/*
 * Tables
 */

// durationValues maps MuseScore's names for note values onto gscore's.
var durationValues = map[string]model.NoteValue{
	"long":    model.DoubleWhole,
	"breve":   model.DoubleWhole,
	"whole":   model.Whole,
	"half":    model.Half,
	"quarter": model.Quarter,
	"eighth":  model.Eighth,
	"16th":    model.Sixteenth,
	"32nd":    model.ThirtySecond,
	"64th":    model.SixtyFourth,
	"128th":   model.HundredTwentyEighth,
}

// clefKinds maps MuseScore's clef names. A clef that only differs by an
// octave — the tenor's G with an 8 under it, the bass guitar's F — reads as
// the clef it is drawn as, because that is where the notes sit on the staff.
var clefKinds = map[string]model.Clef{
	"G": model.Treble, "G8va": model.Treble, "G8vb": model.Treble8vb,
	"G15ma": model.Treble, "G15mb": model.Treble, "G1": model.Treble,
	"F": model.Bass, "F8va": model.Bass, "F8vb": model.Bass,
	"F15ma": model.Bass, "F15mb": model.Bass, "F_8": model.Bass,
	"F3": model.Bass, "F5": model.Bass,
	"C1": model.Alto, "C2": model.Alto, "C3": model.Alto,
	"C4": model.Tenor, "C5": model.Tenor,
	"PERC": model.Percussion, "PERC2": model.Percussion,
	// A tablature staff's clef says nothing a staff of pitches can draw;
	// the tab itself is drawn from the part. See model.Tablature.
	"TAB": model.Treble, "TAB4": model.Treble, "TAB_SERIF": model.Treble,
	"TAB4_SERIF": model.Treble, "TAB2": model.Treble,
}

// articulations maps the marks a note can carry. MuseScore 4 names them after
// their SMuFL glyphs and the versions before it used short words, so both are
// here: the same mark under two names is still the same mark.
var articulations = map[string]model.Articulation{
	"articStaccatoAbove": model.Staccato, "articStaccatoBelow": model.Staccato,
	"staccato": model.Staccato,

	"articStaccatissimoAbove":      model.Staccatissimo,
	"articStaccatissimoBelow":      model.Staccatissimo,
	"articStaccatissimoWedgeAbove": model.Staccatissimo,
	"articStaccatissimoWedgeBelow": model.Staccatissimo,
	"staccatissimo":                model.Staccatissimo,

	"articAccentAbove": model.Accent, "articAccentBelow": model.Accent,
	"sforzato": model.Accent, "sforzatoStaccato": model.Accent,

	"articTenutoAbove": model.Tenuto, "articTenutoBelow": model.Tenuto,
	"tenuto": model.Tenuto, "articTenutoStaccatoAbove": model.Tenuto,
	"articTenutoStaccatoBelow": model.Tenuto,

	"articMarcatoAbove": model.Marcato, "articMarcatoBelow": model.Marcato,
	"marcato": model.Marcato, "umarcato": model.Marcato, "dmarcato": model.MarcatoDown,

	"stringsUpBow": model.UpBow, "upbow": model.UpBow,
	"stringsDownBow": model.DownBow, "downbow": model.DownBow,

	"fermataAbove": model.Fermata, "fermataBelow": model.Fermata,
	"fermataShortAbove": model.Fermata, "fermataShortBelow": model.Fermata,
	"fermataLongAbove": model.Fermata, "fermataLongBelow": model.Fermata,
	"fermata": model.Fermata, "longfermata": model.Fermata,
	"shortfermata": model.Fermata,
}

// dynamics maps MuseScore's name for a dynamic onto gscore's. Most are the
// letters themselves and go through DynamicFromName; these are the ones whose
// spelling differs or that gscore reads as a near neighbour.
var dynamics = map[string]model.Dynamic{
	"sffz": model.SForzando, "sfp": model.FortePiano, "sfpp": model.FortePiano,
	"fz": model.SForzato, "rf": model.Rinforzando, "rfz": model.Rinforzando,
	"sff": model.FF, "sfff": model.FFF, "pf": model.Forte, "ffp": model.FortePiano,
	"n":      model.Niente,
	"pppppp": model.PPPP, "ppppp": model.PPPP,
	"ffffff": model.FFFF, "fffff": model.FFFF,
}

// octaveAmounts reads an ottava's kind. MuseScore 4 writes the name and the
// versions before it wrote a number, in this order.
var octaveAmounts = map[string]int{
	"8va": 1, "15ma": 2, "22ma": 3,
	"8vb": -1, "15mb": -2, "22mb": -3,
	"0": 1, "1": 2, "2": 3, "3": -1, "4": -2, "5": -3,
}

// barlineStyles maps a barline's kind. A repeat is not among them: MuseScore
// writes repeats as properties of the measure, not as a barline.
var barlineStyles = map[string]model.BarlineStyle{
	"normal":       model.BarSingle,
	"double":       model.BarDouble,
	"end":          model.BarFinal,
	"dotted":       model.BarSingle,
	"dashed":       model.BarSingle,
	"heavy":        model.BarFinal,
	"double-heavy": model.BarFinal,
}

// dynamicFor reads a dynamic written either way round.
func dynamicFor(name string) (model.Dynamic, bool) {
	name = strings.TrimSpace(name)
	if d, ok := model.DynamicFromName(name); ok {
		return d, true
	}
	d, ok := dynamics[name]
	return d, ok
}

/*
 * More of MuseScore's vocabulary
 *
 * Gathered by counting what 838 MuseScore files on one machine actually
 * contain — the program's own visual-test scores, three versions of
 * musescore.com downloads, and a drawer of a working musician's own — and
 * reading what the program writes for each. Every table here is something
 * those files had and the reader used to pass over.
 */

// twoArticulations is a MuseScore mark that gscore writes as two: a
// portato is a tenuto and a staccato on one note, and MuseScore 4 names
// every such pair as one glyph.
var twoArticulations = map[string][2]model.Articulation{
	"portato":                   {model.Tenuto, model.Staccato},
	"articTenutoStaccatoAbove":  {model.Tenuto, model.Staccato},
	"articTenutoStaccatoBelow":  {model.Tenuto, model.Staccato},
	"articAccentStaccatoAbove":  {model.Accent, model.Staccato},
	"articAccentStaccatoBelow":  {model.Accent, model.Staccato},
	"articMarcatoStaccatoAbove": {model.Marcato, model.Staccato},
	"articMarcatoStaccatoBelow": {model.Marcato, model.Staccato},
	"articTenutoAccentAbove":    {model.Tenuto, model.Accent},
	"articTenutoAccentBelow":    {model.Tenuto, model.Accent},
	"articMarcatoTenutoAbove":   {model.Marcato, model.Tenuto},
	"articMarcatoTenutoBelow":   {model.Marcato, model.Tenuto},
	"sforzatoStaccato":          {model.Accent, model.Staccato},
}

// oldOrnaments are the names MuseScore 1 and 2 gave the marks that gscore
// draws as its own ornaments.
var oldOrnaments = map[string]model.Articulation{
	"mordent": model.Mordent, "prall": model.ShortTrill,
	"turn": model.Turn, "reverseturn": model.TurnInverted,
	"ornamentTurnInverted": model.TurnInverted,
	"fermataVeryLongAbove": model.Fermata, "fermataVeryLongBelow": model.Fermata,
	"verylongfermata": model.Fermata,
}

// markGlyphs are the marks gscore has no name of its own for, drawn by
// the SMuFL glyph MuseScore itself draws them with. The old short names
// of MuseScore 1 and 2 are mapped to the same glyphs MuseScore maps them
// to when it opens such a file.
var markGlyphs = map[string]string{
	"prallprall": "ornamentTremblement", "prallmordent": "ornamentPrallMordent",
	"upprall": "ornamentUpPrall", "upmordent": "ornamentUpMordent",
	"downmordent": "ornamentDownMordent", "pralldown": "ornamentPrallDown",
	"prallup": "ornamentPrallUp", "lineprall": "ornamentLinePrall",
	"schleifer": "ornamentPrecompSlide", "downprall": "ornamentPrecompMordentUpperPrefix",
	"ouvert": "brassMuteOpen", "plusstop": "brassMuteClosed",
	"snappizzicato": "pluckedSnapPizzicatoAbove", "thumb": "stringsThumbPosition",
	"fadein": "guitarFadeIn", "fadeout": "guitarFadeOut", "volumeswell": "guitarVolumeSwell",
	"wigglesawtooth": "wiggleSawtooth", "wigglesawtoothwide": "wiggleSawtoothWide",
	"wigglevibratolargefaster":  "wiggleVibratoLargeFaster",
	"wigglevibratolargeslowest": "wiggleVibratoLargeSlowest",
	"lutefingeringthumb":        "luteFingeringRHThumb", "lutefingering1st": "luteFingeringRHFirst",
	"lutefingering2nd": "luteFingeringRHSecond", "lutefingering3rd": "luteFingeringRHThird",
}

// breathGlyphs are the breath marks, by MuseScore 3's symbol names.
var breathGlyphs = map[string]model.Breath{
	"breathMarkComma": model.BreathComma, "breathMarkTick": model.BreathTick,
	"breathMarkUpbow": model.BreathTick, "breathMarkSalzedo": model.BreathComma,
	"caesura": model.Caesura, "caesuraCurved": model.CaesuraCurved,
	"caesuraShort": model.Caesura, "caesuraThick": model.Caesura,
	"caesuraSingleStroke": model.Caesura,
}

// oldBreaths are the same marks as MuseScore 2 numbered them.
var oldBreaths = [...]model.Breath{model.BreathComma, model.BreathComma,
	model.CaesuraCurved, model.Caesura}

// chordLineGlyphs are the falls and scoops a jazz chart writes beside a
// note, by MuseScore's number for each.
var chordLineGlyphs = map[string]string{
	"1": "brassFallLipShort", "2": "brassDoitShort",
	"3": "brassPlop", "4": "brassScoop",
}

// dynamicLetters spells the symbols a MuseScore 4 dynamic is written
// with, so that "più f" or "sfp" can be read as the letters they are.
var dynamicLetters = map[string]string{
	"dynamicPiano": "p", "dynamicForte": "f", "dynamicMezzo": "m",
	"dynamicSforzando": "s", "dynamicZ": "z", "dynamicRinforzando": "r",
	"dynamicSubito": "sub.",
	"dynamicNiente": "n", "dynamicMF": "mf", "dynamicMP": "mp",
	"dynamicFF": "ff", "dynamicPP": "pp", "dynamicFFF": "fff",
	"dynamicPPP": "ppp", "dynamicSforzato": "sf", "dynamicFortePiano": "fp",
}

// oldClefs are MuseScore 1's clefs, which it numbered rather than named.
var oldClefs = [...]string{"G", "G8va", "G15ma", "G8vb", "F", "F8vb", "F15mb",
	"F_B", "F_C", "C1", "C2", "C3", "C4", "TAB", "PERC", "C5", "G1", "F_8va",
	"F_15ma", "PERC2", "TAB2"}

// oldBarlines are MuseScore 1's barlines, which it numbered.
var oldBarlines = [...]string{"normal", "double", "start-repeat", "end-repeat",
	"dashed", "end", "end-start-repeat", "dotted"}

// techniqueSpanners are the guitar lines that are a way of playing every
// note under them.
var techniqueSpanners = map[string]model.Technique{
	"LetRing": model.LetRing, "PalmMute": model.PalmMute, "Vibrato": model.Vibrato,
}
