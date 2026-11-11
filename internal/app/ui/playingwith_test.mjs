/*
 * playingwith_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the status bar says is making the sound.
 *
 * There are two answers and they sound nothing alike: the recorded
 * instruments of a soundfont, or the oscillators the editor falls back to
 * where there is none. A score played through the second sounds wrong in a
 * way that is easy to blame on the score, and the line said neither.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

function grab(name) {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
}

// playingWith, run against a stub of the performance.
function run(live) {
  const make = new Function("live",
    grab("playingWith") + "\nreturn playingWith();");
  return make(live);
}

test("it names the soundfont that is actually playing", () => {
  // live.font is what the core answered with when the performance began,
  // so it is the bank being rendered through rather than whatever the
  // settings say ought to be.
  assert.equal(run({ font: "MuseScore General" }), "MuseScore General");
});

test("and says so when there is no soundfont at all", () => {
  // The oscillators do not pretend to be recordings, and somebody hearing
  // a flute part as a sine wave is owed the reason.
  assert.match(run(null), /built-in voices/);
  assert.match(run({ font: "" }), /built-in voices/);
});

test("the playing line names it", () => {
  const play = grab("playNow");
  assert.match(play, /Playing\$\{where\} with \$\{playingWith\(\)\}/,
    "the status line does not say what is making the sound");
  // And still says what it always said, which somebody reads to check the
  // score rather than the sound.
  assert.match(play, /notes at \$\{pb\.tempo\} bpm/);
});
