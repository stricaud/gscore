/*
 * fingering_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which note the fingering panel is being asked about.
 *
 * It reads the note off the page — a staff position, which the clef turns
 * into a pitch. The engraver counts diatonic steps from middle C, because
 * that is where a staff is; this counted them from C0, so every note came
 * out four octaves below itself. A flute part was answered with "G0 — out of
 * the range of this chart", which is true of G0 and says nothing at all
 * about the G in front of the player. Reported as "the fingering does not
 * work, this is clearly not out of range".
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};
const midiForStep = new Function(grab("midiForStep") + "\nreturn midiForStep;")();

// The staff positions the engraver gives these pitches, and the MIDI numbers
// they sound at. Worked out the way the engraver does it: a diatonic index
// of octave*7 + letter counted from middle C, plus the clef's own offset.
const PLACES = [
  ["treble", -3, 59, "B3"],
  ["treble", -2, 60, "C4 — the ledger line below"],
  ["treble", 0, 64, "E4 — the bottom line"],
  ["treble", 2, 67, "G4"],
  ["treble", 7, 76, "E5"],
  ["treble", 8, 77, "F5 — the top line"],
  ["treble", 14, 88, "E6, three ledger lines up"],
  ["bass", 0, 43, "G2 — the bottom line"],
  ["bass", 3, 48, "C3"],
  ["bass", 8, 57, "A3 — the top line"],
  ["alto", 4, 60, "C4 — the middle line"],
  ["tenor", 6, 60, "C4 — the fourth line"],
];

test("a staff position is the note that is written there", () => {
  for (const [clef, step, midi, what] of PLACES) {
    assert.equal(midiForStep(clef, step), midi, `${clef} ${step}: ${what}`);
  }
});

// Four octaves is exactly what was wrong, so it is worth saying out loud.
test("the notes of a staff are the ones a player can play", () => {
  // A flute reads from middle C up; nothing it is asked to play is near the
  // bottom of a piano, which is where every answer used to land.
  for (const [clef, step] of PLACES.filter(([c]) => c === "treble")) {
    const midi = midiForStep(clef, step);
    assert.ok(midi >= 48 && midi <= 96,
              `a treble note came out at MIDI ${midi}, which is nowhere near a staff`);
  }
});

// The sign on the note is part of which note it is. A chart asked for an F
// sharp and handed the fingering for F is worse than no chart at all.
test("the written sign is counted", () => {
  const grabIt = (name) => {
    const m = src.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
    if (!m) throw new Error("app.js has no " + name);
    return m[0];
  };
  const asked = [];
  const state = {
    info: { parts: [{ instrument: "flute", clef: "treble" }] },
    selection: { part: 0, measure: 0, ref: 7 },
    hits: [{
      part: 0, measure: 0,
      elements: [{ ref: 7, index: 0, steps: [2], alters: [1] }],
    }],
  };
  const scope = {
    state,
    fingering: { on: true, staff: 0, last: null },
    showFingering: (midi, inst) => asked.push([midi, inst]),
  };
  const body = [grabIt("updateFingering"), grabIt("midiForStep")].join("\n") +
    "\nreturn { updateFingering };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).updateFingering();

  assert.deepEqual(asked, [[68, "flute"]],
    "G sharp 4 is MIDI 68; the plain G would be 67");
});
