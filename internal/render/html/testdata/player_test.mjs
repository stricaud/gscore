/*
 * player_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The player that travels with a shared page, run.
 *
 * It is JavaScript held in Go as a string, so nothing about building gscore
 * says whether it works. What it has to do is make the sounds at the right
 * times, and the way it failed was in how many it asked for at once: a
 * symphony's worth of oscillators, all created before the first one sounded,
 * which a browser will not do. It got through the first few staves and the
 * rest of the orchestra was silent — on a page that looked fine and played
 * something, which is the worst way for it to be wrong.
 *
 * So the audio clock and the page are faked here and the real player is run
 * against them. player_node_test.go writes it out and points this at it.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(process.env.GSCORE_PLAYER, "utf8");

// A page, as the player finds one: a clock we wind by hand, an audio graph
// that counts what it is asked for, and the few elements it looks up.
function mount({ notes, duration, recording = false }) {
  const sound = {
    notes,
    bars: [{ i: 0, t: 0, d: 2 }, { i: 1, t: 2, d: 2 }],
    boxes: [{ i: 0, a: 0, b: 10, c: 0, e: 8 }, { i: 1, a: 10, b: 20, c: 0, e: 8 }],
    duration,
    tempo: 120,
  };

  const made = { osc: 0 };
  const starts = [];                 // when each note was told to sound
  const chains = [];                 // {when, until}, one per note sounded
  let clock = 0;

  const knob = () => ({ setValueAtTime() {}, linearRampToValueAtTime() {},
                        exponentialRampToValueAtTime() {} });
  const plain = () => ({ type: "", frequency: knob(), gain: knob(),
                         connect() {}, start() {}, stop() {} });
  class Ctx {
    get currentTime() { return clock; }
    get state() { return "running"; }
    resume() {}
    createBiquadFilter() { return plain(); }
    createGain() { return plain(); }
    get destination() { return plain(); }
    createOscillator() {
      made.osc++;
      const o = plain();
      const chain = { when: 0, until: 0 };
      o.start = (when) => { chain.when = when; starts.push(when); };
      o.stop = (when) => { chain.until = when; chains.push(chain); };
      return o;
    }
  }

  const media = {
    currentTime: 0, duration, ended: false, paused: true, on: {},
    play() { media.paused = false; media.played = true; return Promise.resolve(); },
    pause() { media.paused = true; },
    addEventListener(ev, fn) { media.on[ev] = fn; },
    fire(ev) { if (media.on[ev]) media.on[ev](); },
  };

  const lit = new Set();
  const svg = {
    appendChild() {},
    querySelectorAll: (sel) => {
      const m = /data-ref="(\d+)"/.exec(sel);
      if (!m) return [];
      return [{ classList: { add: () => lit.add(m[1]),
                             remove: () => lit.delete(m[1]) } }];
    },
  };
  const button = { innerHTML: "", disabled: false, on: {},
                   addEventListener(ev, fn) { button.on[ev] = fn; } };
  const clockEl = { textContent: "" };

  globalThis.window = { AudioContext: Ctx };
  globalThis.document = {
    getElementById(id) {
      if (id === "gscore-sound") return { textContent: JSON.stringify(sound) };
      if (id === "gscore-audio") return recording ? media : null;
      if (id === "play") return button;
      if (id === "clock") return clockEl;
      return null;
    },
    querySelector: () => svg,
    addEventListener() {},
    createElementNS: () => ({ setAttribute() {}, remove() {}, isConnected: true,
                              getBoundingClientRect: () => ({ top: 10, bottom: 20 }),
                              scrollIntoView() {} }),
  };

  let frame = null;
  globalThis.requestAnimationFrame = (fn) => { frame = fn; return 1; };
  globalThis.cancelAnimationFrame = () => { frame = null; };
  let pump = null, pumps = 0;
  globalThis.setInterval = (fn) => { pump = fn; pumps++; return 7; };
  globalThis.clearInterval = () => { pump = null; };

  new Function(src)();

  return {
    sound, media, button, clockEl, lit, made, starts, chains,
    press: () => button.on.click(),
    tick: () => { if (frame) frame(); },
    wind: (to) => { clock = to; if (pump) pump(); },
    pumps: () => pumps,
    now: () => clock,
  };
}

// A score shaped like an orchestral one: every part covering the whole
// piece, so the order they are listed in and the order they sound differ as
// much as they do in the real file.
function orchestra(parts, perPart, duration) {
  const notes = [];
  for (let p = 0; p < parts; p++) {
    for (let k = 0; k < perPart; k++) {
      notes.push({ t: +((k / perPart) * duration).toFixed(3), d: 0.4,
                   m: 60 + (p % 24), v: 80, f: "bowed",
                   r: p * perPart + k + 1 });
    }
  }
  return notes;
}

test("a long score is not handed to the audio clock all at once", () => {
  const notes = orchestra(54, 500, 3450);
  const p = mount({ notes, duration: 3450 });
  p.press();

  // Before this, pressing play created three audio nodes for every note in
  // the piece — eighty thousand of them here, four hundred thousand on the
  // ninth — in one turn, before a sound was made.
  assert.ok(p.made.osc < 2000,
    `play() made ${p.made.osc} oscillators up front, of ${notes.length} notes`);
  assert.ok(p.pumps() === 1, "no scheduler was started");
});

test("every note still sounds, and at its own time", () => {
  const notes = orchestra(8, 200, 400);
  const p = mount({ notes, duration: 400 });
  p.press();
  for (let t = 0; t <= 405; t += 0.2) p.wind(t);

  assert.equal(p.starts.length, notes.length,
    `${p.starts.length} notes sounded, of ${notes.length}`);

  const want = notes.map((n) => n.t).sort((a, b) => a - b);
  const got = p.starts.slice().sort((a, b) => a - b);
  let worst = 0;
  for (let i = 0; i < want.length; i++) {
    worst = Math.max(worst, Math.abs(got[i] - want[i] - 0.08));
  }
  assert.ok(worst < 0.01, `a note was ${(worst * 1000).toFixed(1)}ms out`);
});

test("only a handful of notes are alive at any one moment", () => {
  const notes = orchestra(12, 200, 400);
  const p = mount({ notes, duration: 400 });
  p.press();
  for (let t = 0; t <= 405; t += 0.2) p.wind(t);

  const edges = [];
  for (const c of p.chains) { edges.push([c.when, 1], [c.until, -1]); }
  edges.sort((a, b) => a[0] - b[0] || a[1] - b[1]);
  let alive = 0, peak = 0;
  for (const [, d] of edges) { alive += d; peak = Math.max(peak, alive); }

  // Twelve parts, so twelve notes together and no more.
  assert.ok(peak <= 24, `${peak} notes were alive at once`);
});

test("a page with a recording of itself synthesises nothing", () => {
  const p = mount({
    notes: [{ t: 0, d: 1, r: 1 }, { t: 2, d: 1, r: 2 }],
    duration: 82.25, recording: true,
  });
  p.press();

  assert.ok(p.media.played, "pressing play did not start the recording");
  assert.equal(p.made.osc, 0, "the page made its own sound as well");
  assert.equal(p.pumps(), 0, "a note scheduler was started needlessly");
  assert.match(p.button.innerHTML, /Pause/);
});

test("the cursor and the highlight follow the recording's own clock", () => {
  const p = mount({
    notes: [{ t: 0, d: 1, r: 1 }, { t: 2, d: 1, r: 2 }],
    duration: 82.25, recording: true,
  });
  p.press();

  p.media.currentTime = 2.5;
  p.tick();
  assert.equal(p.clockEl.textContent, "0:02");
  assert.ok(p.lit.has("2"), `lit ${[...p.lit]} at 2.5s`);

  // Moving back through a recording must not leave the highlight stranded
  // ahead of it: it only ever walks forward, so it has to notice.
  p.media.currentTime = 0.5;
  p.tick();
  assert.ok(p.lit.has("1") && !p.lit.has("2"),
    `lit ${[...p.lit]} after going back to 0.5s`);
});

test("a recording that will not load is said so, not played silently", () => {
  const p = mount({ notes: [{ t: 0, d: 1, r: 1 }], duration: 10, recording: true });
  p.press();
  p.media.fire("error");

  assert.ok(p.button.disabled, "the button still offers to play nothing");
  assert.equal(p.clockEl.textContent, "no recording");
});
