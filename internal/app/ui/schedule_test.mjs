/*
 * schedule_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Playing a long score without a soundfont.
 *
 * Served over a network there are no soundfonts to play through — they are
 * files on somebody's machine — so the page builds the sound out of
 * oscillators. One note is a handful of Web Audio nodes, which is fine for a
 * song and hopeless for a symphony: Beethoven's ninth is a hundred and
 * thirty-two thousand notes across fifty-seven minutes, and building all of
 * it before the first note sounds meant half a million nodes up front. The
 * browser either refused or took so long that the music had gone past by the
 * time it started.
 *
 * So the notes are built a few seconds ahead of the music and topped up as
 * it goes. Three things have to hold: nothing far ahead is built early,
 * everything is built by the time it is due, and the handles on notes that
 * have finished are let go rather than held for the whole piece.
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
const constant = (name) => {
  const m = src.match(new RegExp("^const " + name + " = (\\d+(?:\\.\\d+)?);", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return Number(m[1]);
};

const ahead = constant("scheduleAhead");
const every = constant("scheduleEvery");

// The scheduler, with a clock the test winds forward by hand and a stand-in
// for building a note.
function player(events) {
  let now = 0;
  const built = [];
  let timer = null;
  const scope = {
    built,
    audio: { get currentTime() { return now; } },
    setInterval: (fn) => { timer = fn; return 1; },
    clearInterval: () => { timer = null; },
    Math,
  };
  const body =
    "let voices = []; let scheduleTimer = null;\n" +
    "const scheduleAhead = " + ahead + "; const scheduleEvery = " + every + ";\n" +
    "function sound(ev, start) { built.push({ ev, start }); voices.push({}); }\n" +
    grab("stopScheduling") + "\n" + grab("scheduleOscillators") +
    "\nreturn { scheduleOscillators, held: () => voices.length," +
    "         running: () => scheduleTimer !== null };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return {
    ...api, built,
    // Wind the clock on and let the scheduler top up, as the browser's own
    // timer would.
    run(to) {
      while (now < to) {
        now = Math.min(to, now + every / 1000);
        if (timer) timer();
      }
    },
    at: () => now,
  };
}

// A symphony: a note every quarter second on each of fifty-two staves, for
// an hour.
function symphony(staves = 52, minutes = 57) {
  const out = [];
  const beats = minutes * 60 * 4;
  for (let i = 0; i < beats; i++) {
    for (let p = 0; p < staves; p++) {
      out.push({ start: i * 0.25, dur: 0.25, gate: 1, midi: 60, part: p });
    }
  }
  return out;
}

test("a symphony is not built before its first note sounds", () => {
  // The shape of the real thing: fifty-two staves, a note each every
  // quarter of a second, for fifty-seven minutes.
  const events = symphony();
  const t = player(events);
  t.scheduleOscillators(events, 0, 0, null);
  assert.ok(t.built.length > 0, "nothing was built, so nothing would sound");
  assert.ok(t.built.length < 1000,
    `${t.built.length} of ${events.length} notes were built before the first ` +
    `one sounded; the browser has to make a handful of nodes for each`);
});

// A smaller, cheaper shape for the detailed checks.
function symphonyOnce() {
  const out = [];
  for (let i = 0; i < 4000; i++) {
    out.push({ start: i * 0.25, dur: 0.25, gate: 1, midi: 60, part: i % 8 });
  }
  return out;
}

test("only the next few seconds are built, however long the piece is", () => {
  const events = symphonyOnce(); // a thousand seconds of music
  const t = player(events);
  t.scheduleOscillators(events, 0, 0, null);
  // At the moment Play is pressed, only the window is in hand.
  const first = t.built.length;
  assert.ok(first > 0, "nothing was built, so nothing would sound");
  assert.ok(first <= (ahead + 0.5) * 4 + 4,
    `${first} notes were built before the first one sounded, out of ${events.length}`);
});

test("every note is built by the time it is due", () => {
  const events = symphonyOnce();
  const t = player(events);
  t.scheduleOscillators(events, 0, 0, null);
  for (let when = 0; when < 60; when += 1) {
    t.run(when);
    const due = events.filter((e) => e.start <= when).length;
    assert.ok(t.built.length >= due,
      `at ${when}s, ${t.built.length} notes were built and ${due} were due`);
    // And still not the whole piece.
    assert.ok(t.built.length < events.length,
      "the whole piece was built after all");
  }
});

test("handles on notes that have finished are let go", () => {
  const events = symphonyOnce();
  const t = player(events);
  t.scheduleOscillators(events, 0, 0, null);
  t.run(120);
  // Two minutes in, nearly five hundred notes have been played. What is
  // still held has to be a window's worth, not all of them: holding them is
  // what keeps the browser from freeing the nodes.
  assert.ok(t.built.length > 400, "the music did not get going");
  assert.ok(t.held() < 100,
    `${t.held()} notes are still held after ${t.built.length} have been played`);
});

test("stopping stops the building", () => {
  const events = symphonyOnce();
  const t = player(events);
  t.scheduleOscillators(events, 0, 0, null);
  assert.equal(t.running(), true);
  const fn = src.match(/^function silence\(\)[\s\S]*?^}/m)[0];
  assert.ok(fn.includes("stopScheduling()"),
    "stopping the music leaves the window filling itself, and the piece plays on");
});

test("a piece short enough to build in one go is built in one go", () => {
  const song = [];
  for (let i = 0; i < 20; i++) song.push({ start: i * 0.05, dur: 0.05, midi: 60, part: 0 });
  const t = player(song);
  t.scheduleOscillators(song, 0, 0, null);
  assert.equal(t.built.length, song.length, "a short song was dribbled out");
  assert.equal(t.running(), false, "a timer is still ticking over a finished song");
});
