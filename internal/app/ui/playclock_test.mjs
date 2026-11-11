/*
 * playclock_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The clock the cursor follows while the score is playing.
 *
 * Reported as a shift between the note being heard and the note lit up on
 * the page, guessed at — rightly — as something to do with soundfonts. It
 * was two things, and both of them put the picture ahead of the sound:
 *
 *   - the moment the music was to start at was fixed a tenth of a second
 *     ahead, and only then was the server asked to render the opening of
 *     it. On a twenty-three staff orchestra through a soundfont of real
 *     recordings that render is not done within a tenth of a second, so
 *     what came back was scheduled late while the cursor had gone on time.
 *     The lag was the length of that render, which is why it followed the
 *     size of the soundfont.
 *
 *   - and the cursor was driven by performance.now() while the sound was
 *     scheduled against audio.currentTime: two clocks, and the speakers
 *     lagging both by the output latency, which over Bluetooth is a
 *     quaver's worth.
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

function lift(names, scope) {
  const body = names.map(grab).join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  return new Function("scope", "with (scope) { " + body + " }")(scope);
}

/* ---------- where the sound has got to ---------- */

test("the place being played is read off the audio clock", () => {
  const scope = { state: { playLag: 0 },
                  audio: { currentTime: 10, baseLatency: 0, outputLatency: 0 },
                  playAt: 4 };
  const { playedSeconds } = lift(["playedSeconds", "outputLag"], scope);
  assert.equal(playedSeconds(), 6);
  // And it moves with that clock, not with the wall clock.
  scope.audio.currentTime = 12.5;
  assert.equal(playedSeconds(), 8.5);
});

test("and is put back by however late the speakers are", () => {
  // The cursor should sit on the note being heard, and what is being heard
  // left the audio clock a moment ago.
  const scope = { state: { playLag: 0 },
                  audio: { currentTime: 10, outputLatency: 0.2 }, playAt: 4 };
  const { playedSeconds } = lift(["playedSeconds", "outputLag"], scope);
  assert.equal(Math.round(playedSeconds() * 1000), 5800);
});

const noLag = { state: { playLag: 0 } };

test("the lag is the real one where the browser knows it", () => {
  const { outputLag } = lift(["outputLag"],
    { ...noLag, audio: { outputLatency: 0.25, baseLatency: 0.01 } });
  assert.equal(outputLag(), 0.25);
});

test("and the part of it WebKit admits to where it does not", () => {
  // Safari, which is the Mac application: no outputLatency at all.
  const { outputLag } = lift(["outputLag"],
    { ...noLag, audio: { baseLatency: 0.012 } });
  assert.equal(outputLag(), 0.012);
  // And nothing at all rather than a guess, where neither is offered.
  assert.equal(lift(["outputLag"], { ...noLag, audio: {} }).outputLag(), 0);
  assert.equal(lift(["outputLag"], { ...noLag, audio: null }).outputLag(), 0);
});

/* ---------- and what the browser cannot know ---------- */

test("a measured delay is added to what the browser owns up to", () => {
  // Both, and not one or the other: the browser's figure is right about
  // the part of the journey it can see, and the measurement is for the
  // headphones on the end of it, which it cannot.
  const { outputLag } = lift(["outputLag"],
    { state: { playLag: 200 }, audio: { baseLatency: 0.01 } });
  assert.equal(Math.round(outputLag() * 1000), 210);
});

test("and it holds even where there is no audio at all yet", () => {
  const { outputLag } = lift(["outputLag"],
    { state: { playLag: 150 }, audio: null });
  assert.equal(outputLag(), 0.15);
});

test("a measured delay moves the cursor onto the note being heard", () => {
  const scope = {
    state: { playLag: 300 },
    audio: { currentTime: 10, baseLatency: 0 },
    playAt: 4,
  };
  const { playedSeconds } = lift(["playedSeconds", "outputLag"], scope);
  // Six seconds in by the audio clock, but what is being heard now left it
  // three-tenths ago.
  assert.equal(Math.round(playedSeconds() * 1000), 5700);
});

test("the correction can go the other way", () => {
  // A browser that over-reports its own latency leaves the cursor behind
  // the sound instead of ahead of it.
  const { outputLag } = lift(["outputLag"],
    { state: { playLag: -60 }, audio: { outputLatency: 0.1 } });
  assert.equal(Math.round(outputLag() * 1000), 40);
});

test("the dialog refuses a number nobody could have measured", () => {
  const body = grab("soundDelayDialog");
  assert.match(body, /n < -250 \|\| n > 1000/);
  // And keeps it for this performance before trying to write it down: the
  // page has no disk to write it to at all.
  const kept = body.indexOf("state.playLag = n");
  const wrote = body.indexOf("/api/settings?playLag=");
  assert.ok(kept > 0 && wrote > kept,
    "the setting is written away before it is applied");
});

test("a score not playing yet is at nought, not at a crash", () => {
  const { playedSeconds } = lift(["playedSeconds", "outputLag"],
    { state: { playLag: 0 }, audio: null, playAt: 0 });
  assert.equal(playedSeconds(), 0);
});

/* ---------- the music is in hand before the clock starts ---------- */

function liveScope(chunks) {
  const scope = {
    live: null,
    voices: [],
    started: [],
    pumped: 0,
    canSound: () => true,
    state: { info: {} },
    audio: {
      sampleRate: 44100,
      currentTime: 100,
      createBufferSource: () => ({
        connect: () => {},
        start: (at) => scope.started.push(at),
      }),
    },
    fetch: async () => ({
      ok: true,
      json: async () => ({ id: "p1", rate: 44100, font: "MuseScore General" }),
    }),
    liveChunk: async () => chunks.shift() || null,
    pumpLive: () => { scope.pumped++; },
    endLive: () => { scope.live = null; },
  };
  return scope;
}

test("the opening is rendered before startLive answers", async () => {
  const scope = liveScope([{ duration: 2 }]);
  const { startLive } = lift(["startLive"], scope);
  assert.equal(await startLive(0, {}), true);
  // In hand, and not yet scheduled: there is no moment to schedule it at
  // until the clock is started around it.
  assert.deepEqual(scope.live.first, { duration: 2 });
  assert.deepEqual(scope.started, []);
  assert.equal(scope.pumped, 0);
});

test("nothing to render is no performance, and the oscillators take it", async () => {
  const scope = liveScope([]);
  const { startLive } = lift(["startLive"], scope);
  assert.equal(await startLive(0, {}), false);
  assert.equal(scope.live, null, "a performance was left open behind it");
});

test("the rendered opening starts exactly on the clock, not after it", async () => {
  // The whole of the fix: it is scheduled at the moment the cursor, the
  // clicks and the oscillators are all counted from.
  const scope = liveScope([]);
  scope.live = { at: 0, first: { duration: 2.5 }, master: {}, ended: false };
  const { beginLive } = lift(["beginLive"], scope);
  beginLive(100.1);
  assert.deepEqual(scope.started, [100.1]);
  // And the next piece of music goes on the end of it.
  assert.equal(scope.live.at, 102.6);
  assert.equal(scope.live.first, null, "it would be scheduled twice");
  assert.equal(scope.pumped, 1, "nothing went on to render the rest");
  assert.equal(scope.voices.length, 1, "stopping would not silence it");
});

test("the clock is started after the render, not before it", () => {
  // Said of the order in playNow itself, because the order is the bug: a
  // t0 worked out first is a t0 the render can be late for.
  const play = grab("playNow");
  const render = play.indexOf("await startLive(");
  const clock = play.indexOf("const t0 = audio.currentTime");
  assert.ok(render > 0 && clock > 0, "playNow no longer reads that way");
  assert.ok(render < clock,
    "the moment to start at is fixed before the music to start is rendered");
  // And the cursor is set from that same moment, in the score's own time.
  assert.match(play, /playAt = t0 - from/);
});
