/*
 * setupsounds_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The per-instrument soundfonts in Score setup, and finding them.
 *
 * The section took itself off the page whenever it had nothing in it to
 * set, which is the worst thing a setting can do: somebody looking for it
 * finds a dialog with no such section and no way to tell whether they are
 * in the wrong window or whether it is not there yet. Reported as "I am not
 * seeing where I can select the specific soundfont — if this is not where I
 * can set it up, where is it?".
 *
 * It had nothing in it because it asked each staff for its catalogue name,
 * and a score read from a format that records the General MIDI program and
 * not the catalogue has staves that know they are flutes without knowing
 * what gscore calls a flute.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

function grab(name) {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
}

// soundForInstrumentName, against a catalogue like the server's.
function lookup(name, catalogue) {
  const make = new Function("instrumentCatalogue",
    grab("soundForInstrumentName") + "\nreturn soundForInstrumentName;");
  return make(catalogue)(name);
}

const catalogue = [
  { name: "piano", sound: "keyboard.piano" },
  { name: "flute", sound: "wind.flutes.flute" },
  { name: "drum kit", sound: "drum.kit", percussion: true },
];

test("a staff that knows only its program still finds its instrument", () => {
  assert.equal(lookup("flute", catalogue), "wind.flutes.flute");
  assert.equal(lookup("Flute", catalogue), "wind.flutes.flute");
  assert.equal(lookup("piano", catalogue), "keyboard.piano");
});

test("an instrument nothing recognises finds nothing", () => {
  assert.equal(lookup("ondes martenot", catalogue), "");
  assert.equal(lookup("", catalogue), "");
  assert.equal(lookup(undefined, catalogue), "");
});

test("a drum staff is not an instrument among the others", () => {
  // The same patch number means a snare in one bank and a harpsichord in
  // the other; what makes a staff percussion is its clef.
  assert.equal(lookup("drum kit", catalogue), "");
});

test("the section stays on the page when it has nothing to set", () => {
  const body = grab("drawSetupSounds");
  // The only thing that takes it away is a score with no staves at all,
  // and then the whole dialog is about making some.
  const hides = body.match(/box\.hidden = true;/g) || [];
  assert.equal(hides.length, 1,
    "it is hidden in more than one case: " + hides.length);
  assert.match(body, /parts \|\| \[\]\)\.length[\s\S]{0,80}box\.hidden = true/,
    "the one case it hides for is not an empty score");
  // And where there is nothing to choose between, it says so rather than
  // leaving an empty table.
  assert.match(body, /nothing to \n?\s*\/\/?\s*choose between yet|nothing to "\s*\n?\s*\+ "choose between yet/,
    "it does not explain why it is empty");
});

test("the dialog has the section at all", () => {
  assert.match(html, /id="setup-sounds"/);
  assert.match(html, /Which recordings each instrument plays through/);
});
