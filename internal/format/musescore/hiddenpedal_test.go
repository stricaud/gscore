/*
 * hiddenpedal_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A pedal MuseScore keeps for playback and does not print.
 *
 * Summertime has one under every bass note, <visible>0</visible>, and drawn
 * they were a "Ped." on top of every low note of the left hand.
 */

package musescore

import "testing"

func TestAnInvisiblePedalIsHidden(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.02">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Piano</trackName></Part>
    <Staff id="1"><Measure><voice>
      <Spanner type="Pedal"><Pedal><lineVisible>0</lineVisible><visible>0</visible></Pedal>
        <next><location><fractions>1/2</fractions></location></next></Spanner>
      <Chord><durationType>half</durationType><Note><pitch>36</pitch><tpc>14</tpc></Note></Chord>
      <Spanner type="Pedal"><prev><location><fractions>-1/2</fractions></location></prev></Spanner>
      <Spanner type="Pedal"><Pedal></Pedal>
        <next><location><fractions>1/2</fractions></location></next></Spanner>
      <Chord><durationType>half</durationType><Note><pitch>38</pitch><tpc>16</tpc></Note></Chord>
    </voice></Measure>
    <Measure><voice>
      <Spanner type="Pedal"><prev><location><fractions>-1/2</fractions></location></prev></Spanner>
      <Rest><durationType>measure</durationType><duration>4/4</duration></Rest>
    </voice></Measure></Staff>
  </Score>
</museScore>`
	els := read(t, src).Parts[0].Measures[0].Voices[0].Elements
	first, second := marksOf(els[0]).Pedal, marksOf(els[1]).Pedal
	if first == nil || !first.Start || !first.Hidden {
		t.Errorf("the invisible pedal came out %+v, want a hidden start", first)
	}
	if second == nil || !second.Start || second.Hidden {
		t.Errorf("the printed pedal came out %+v, want a start that is drawn", second)
	}
}

// A tempo MuseScore keeps for playback only: Summertime speeds up through a
// "Poco accel." with a run of them, and printed they were a metronome mark
// over every bar.
func TestAnInvisibleTempoIsHidden(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.02">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Piano</trackName></Part>
    <Staff id="1">
      <Measure><voice><Rest><durationType>measure</durationType><duration>4/4</duration></Rest></voice></Measure>
      <Measure><voice>
        <Tempo><tempo>1.7666666666666666</tempo><visible>0</visible><text>= 106</text></Tempo>
        <Rest><durationType>measure</durationType><duration>4/4</duration></Rest></voice></Measure>
    </Staff>
  </Score>
</museScore>`
	tp := read(t, src).Parts[0].Measures[1].Tempo
	if tp == nil || tp.BPM != 106 || !tp.Hidden {
		t.Errorf("the invisible tempo came out %+v, want 106 and hidden", tp)
	}
}
