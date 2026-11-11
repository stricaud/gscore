/*
 * entrysound_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Hearing a note as it is written.
 *
 * A note sounded when it is typed is how every instrument anybody has
 * learnt behaves, and it is how a wrong note is caught where it is made
 * rather than three bars later on playback. It is not always wanted —
 * copying a part out at speed, or working next to other people — so it is
 * a checkbox in Settings and it is remembered.
 *
 * What sounds is the server's answer about the note it just wrote: see
 * addSounding in edit.go. Nothing here works out a pitch.
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

// A page with an audio context that writes down what it is asked to play.
function page(over) {
  const scope = {
    played: [],
    woken: 0,
    state: { entrySound: true, playing: false,
             tool: { duration: "quarter", dots: 0 },
             playback: { tempo: 120 } },
    audio: {
      state: "running",
      currentTime: 10,
      resume: () => {},
      createGain: () => ({
        gain: { value: 0 }, connect: () => {}, disconnect: () => {},
      }),
      destination: {},
    },
    wakeAudio: () => { scope.woken++; },
    sound: (ev, at) => scope.played.push({ ...ev, at }),
    setTimeout: () => 0,
  };
  const base = scope.state;
  Object.assign(scope, over || {});
  // The state is merged rather than replaced, so that a test saying one
  // thing about it keeps the rest.
  scope.state = { ...base, ...((over && over.state) || {}) };
  const run = new Function("scope",
    "with (scope) { const entrySeconds = 1;\n" + grab("soundAsWritten") +
    "\nreturn soundAsWritten; }")(scope);
  return { run, scope };
}

test("the note that was written is the note that is played", () => {
  const p = page();
  p.run({ midi: 61, family: "reed" });
  assert.equal(p.scope.played.length, 1);
  assert.equal(p.scope.played[0].midi, 61);
  // And on the voice the staff is played with, so a flute staff does not
  // answer in a piano.
  assert.equal(p.scope.played[0].family, "reed");
});

test("nothing is worked out here — a pitch the server did not send is not played", () => {
  // The clef, the key, the accidental and the transposition decide what a
  // staff position sounds like, and all four are the server's. A page that
  // guessed would be wrong about a clarinet.
  const p = page();
  p.run({ family: "reed" });
  p.run(null);
  p.run({});
  assert.deepEqual(p.scope.played, []);
});

test("a rest is silence", () => {
  // A rest comes back with no pitch and no drum, which is the same answer
  // as a note the server declined to describe, and the same silence.
  const p = page();
  p.run({ status: "ok", measure: 0, at: 1 });
  assert.deepEqual(p.scope.played, []);
});

test("a drum is struck, not pitched", () => {
  const p = page();
  p.run({ drum: 38, family: "percussion" });
  assert.equal(p.scope.played.length, 1);
  // What sounds is the drum. sound() sends anything with a drum on it to
  // the percussion half, where the number names the instrument.
  assert.equal(p.scope.played[0].drum, 38);
  assert.equal(p.scope.played[0].midi, 38);
});

test("turned off, it writes in silence", () => {
  const p = page({ state: { entrySound: false } });
  p.run({ midi: 60, family: "keyboard" });
  assert.deepEqual(p.scope.played, []);
  assert.equal(p.scope.woken, 0, "the audio was started for nothing");
});

test("and it keeps quiet while the score itself is playing", () => {
  // Writing during playback is a thing people do. The note belongs to the
  // performance then, and typing should not double it.
  const p = page({ state: { playing: true } });
  p.run({ midi: 60, family: "keyboard" });
  assert.deepEqual(p.scope.played, []);
});

test("a context the browser has not let start is not an error", () => {
  const p = page({
    audio: {
      state: "suspended", currentTime: 0, resume: () => {},
      createGain: () => { throw new Error("should not be reached"); },
      destination: {},
    },
  });
  p.run({ midi: 60, family: "keyboard" });
  assert.deepEqual(p.scope.played, []);
});

test("a note sounds for a second, whatever was written", () => {
  // Not for its own written length. A semibreve at a slow tempo is four
  // seconds, and four seconds of sound for one keystroke is somebody
  // waiting for the program rather than hearing what they wrote.
  for (const duration of ["thirtysecond", "quarter", "whole", "breve"]) {
    const p = page({ state: { tool: { duration, dots: 0 },
                              playback: { tempo: 40 } } });
    p.run({ midi: 60, family: "keyboard" });
    assert.equal(p.scope.played[0].dur, 1,
      "a " + duration + " sounded for " + p.scope.played[0].dur + "s");
  }
  // And the length is a number in the page, not worked out from the score.
  assert.match(src, /^const entrySeconds = 1;$/m);
});

test("it is on the Settings dialog, and says what it does", () => {
  assert.match(html, /id="setting-entry-sound"/);
  assert.match(html, /Sound each note as it is written/);
  // And says why it is the editor's own voices and not the soundfont,
  // which is the first thing somebody with a soundfont will ask.
  assert.match(html, /editor’s own voices and not the soundfont/);
});

test("the checkbox is read and kept", () => {
  const body = grab("settingsDialog");
  assert.match(body, /hear\.checked = now\.entrySound !== false/,
    "the dialog does not open on the setting as it is");
  assert.match(body, /entrySound", hear\.checked \? "1" : "0"/,
    "the choice is not sent to the server");
  assert.match(body, /state\.entrySound = hear\.checked/,
    "the choice is not held in the page, so every note would be a request");
});

test("a note written goes through it", () => {
  // The two ways a note is written by hand: typed or clicked at the
  // cursor, and played on a MIDI keyboard.
  assert.match(grab("insertAtSelection"), /soundAsWritten\(where\)/);
  assert.match(grab("addPitch"), /soundAsWritten\(await res\.json\(\)/);
});
