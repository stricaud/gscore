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
function mount({ notes, duration, recording = false, bars, boxes, bands,
                 latency = 0 }) {
  const sound = {
    notes,
    bars: bars || [{ i: 0, t: 0, d: 2 }, { i: 1, t: 2, d: 2 }],
    boxes: boxes ||
      [{ i: 0, a: 0, b: 10, c: 0, d: 8 }, { i: 1, a: 10, b: 20, c: 0, d: 8 }],
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
    // How long what has just been written takes to reach the speaker, as a
    // real device reports it.
    get outputLatency() { return latency; }
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

  // The drawing, as one head per note carrying the engraver's number for
  // it. querySelectorAll is counted: on a real symphony it is a walk down a
  // million shapes, so the player must not be doing it every frame.
  const lit = new Set();
  const queries = [];
  const allHeads = notes.filter((n) => n.r).map((n, i) => ({
    ref: String(n.r),
    getAttribute: (a) => (a === "data-ref" ? String(n.r) : null),
    getBBox: () => ({ x: i * 10, y: 20, width: 4.7, height: 4 }),
    classList: { add() { lit.add(String(n.r)); },
                 remove() { lit.delete(String(n.r)); } },
  }));
  // Whatever the player puts on the drawing, so the rings can be counted.
  const added = [];
  // The drawing. A banded one is the score's full size with nothing in it
  // until pieces arrive, so it answers for its viewBox and its rendered size
  // the way a real one does.
  const drawnWide = 1024, drawnHigh = bands ? 2350422.9 : 1000;
  const svg = {
    firstChild: null,
    getAttribute: (a) =>
      (a === "viewBox" ? `0 0 ${drawnWide} ${drawnHigh}` : null),
    getBoundingClientRect: () => ({
      top: 0 - (window.scrollY || 0), left: 0,
      width: drawnWide, height: drawnHigh,
    }),
    appendChild(el) { added.push(el); el.parentNode = svg; return el; },
    insertBefore(el) { added.push(el); el.parentNode = svg; return el; },
    removeChild(el) {
      const i = added.indexOf(el);
      if (i >= 0) added.splice(i, 1);
      el.parentNode = null;
    },
    querySelectorAll: (sel) => {
      queries.push(sel);
      if (sel === "[data-ref]") return allHeads;
      const m = /data-ref="(\d+)"/.exec(sel);
      if (!m) return [];
      return allHeads.filter((h) => h.ref === m[1]);
    },
  };
  const rings = () => added.filter((e) => e.cls === "gscore-ping");
  const button = { innerHTML: "", disabled: false, on: {},
                   addEventListener(ev, fn) { button.on[ev] = fn; } };
  const clockEl = { textContent: "" };

  // The window, and every scroll the player asks of it. The cursor is as
  // tall as its system, which on a big score is far taller than any window —
  // so getBoundingClientRect answers with a box that cannot fit, and how
  // often the player reacts to that is the thing under test.
  const scrolls = [];
  const listeners = {};
  globalThis.window = {
    AudioContext: Ctx,
    innerHeight: 900,
    scrollY: 0,
    scrollTo(opt) { scrolls.push(opt); window.scrollY = opt.top; },
    addEventListener(ev, fn) { (listeners[ev] = listeners[ev] || []).push(fn); },
    removeEventListener() {},
  };
  globalThis.document = {
    getElementById(id) {
      if (id === "gscore-sound") return { textContent: JSON.stringify(sound) };
      if (id === "gscore-audio") return recording ? media : null;
      if (id === "gscore-bands") {
        return bands ? { textContent: JSON.stringify(bands) } : null;
      }
      if (id === "play") return button;
      if (id === "clock") return clockEl;
      return null;
    },
    importNode: (n) => ({ ...n, imported: true }),
    querySelector: () => svg,
    addEventListener() {},
    createElementNS: (ns, name) => ({
      tag: name, cls: null, attrs: {}, parentNode: null, isConnected: true,
      setAttribute(k, v) { this.attrs[k] = v; if (k === "class") this.cls = v; },
      appendChild(el) { el.parentNode = this; return el; },
      addEventListener() {},
      remove() { if (this.parentNode) this.parentNode.removeChild(this); },
      // A system of fifty-four staves: 4,930 units tall, which no window
      // will ever hold. This is the shape of the real thing.
      getBoundingClientRect() {
        const y = Number(this.attrs.y || 0);
        const h = Number(this.attrs.height || 0);
        return { top: y - window.scrollY, bottom: y + h - window.scrollY,
                 height: h };
      },
      scrollIntoView() { scrolls.push({ via: "scrollIntoView" }); },
    }),
  };

  let frame = null;
  globalThis.requestAnimationFrame = (fn) => { frame = fn; return 1; };
  globalThis.cancelAnimationFrame = () => { frame = null; };
  let pump = null, pumps = 0;
  globalThis.setInterval = (fn) => { pump = fn; pumps++; return 7; };
  globalThis.clearInterval = () => { pump = null; };

  // The pieces of the drawing, served as files.
  const fetched = [];
  globalThis.fetch = (url) => {
    fetched.push(url);
    return Promise.resolve({
      ok: true,
      text: () => Promise.resolve(
        '<text class="g" x="1" y="2" data-ref="9001"></text>' +
        '<line x1="0" y1="0" x2="1" y2="1"/>'),
    });
  };
  globalThis.DOMParser = class {
    parseFromString() {
      // As many children as the fragment above has. A list that is read
      // from rather than drained: importNode copies, so a loop that waits
      // for this to empty never finishes.
      return {
        documentElement: { childNodes: [{ tag: "text" }, { tag: "line" }] },
        getElementsByTagName: () => [],
      };
    }
  };

  new Function(src)();

  return {
    sound, media, button, clockEl, lit, made, starts, chains, queries,
    added, rings, scrolls, fetched,
    // Which pieces of the drawing are on the page now.
    onPage: () => added
      .filter((e) => e.attrs && e.attrs["data-band"] !== undefined)
      .map((e) => Number(e.attrs["data-band"])).sort((a, b) => a - b),
    // Let the fetches settle.
    settle: async () => { for (let i = 0; i < 8; i++) await Promise.resolve(); },
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

test("the highlight shows the note being heard, not the one being written", () => {
  // Four notes, a second each, and a device a quarter of a second behind the
  // audio clock — a pair of Bluetooth headphones.
  const notes = [{ t: 0, d: 1, r: 1 }, { t: 1, d: 1, r: 2 },
                 { t: 2, d: 1, r: 3 }, { t: 3, d: 1, r: 4 }];
  const p = mount({ notes, duration: 4, latency: 0.25 });
  p.press();

  // The audio clock has written 1.08 seconds, which is the second note. A
  // quarter of a second of that is still in the device, so what the listener
  // is hearing is the first — and the first is what must be lit.
  p.wind(1.08);
  p.tick();
  assert.ok(p.lit.has("1") && !p.lit.has("2"),
    `lit ${[...p.lit]} while the second note is still on its way out`);

  // And once it has arrived, the second.
  p.wind(1.40);
  p.tick();
  assert.ok(p.lit.has("2") && !p.lit.has("1"),
    `lit ${[...p.lit]} once the second note has reached the speaker`);

  // The sound itself has not moved: the latency is the display's business,
  // and a note made to sound later than it was written would be late.
  assert.equal(p.starts[0], 0.08, "the first note was still sounded at once");

  // The piece is not cut off before its tail has been heard. A quarter of a
  // second past the end, the audio clock is at the end and the listener is
  // still a quarter of a second short of it.
  p.wind(4.3);
  p.tick();
  assert.equal(p.button.innerHTML.includes("Pause"), true,
    "stopped while the last note was still in the device");
});

test("every staff sounding at once is lit, not just the first", () => {
  // Four parts playing a chord together, as four staves of a score do.
  const notes = [
    { t: 0, d: 2, m: 72, v: 80, f: "bowed", r: 1 },
    { t: 0, d: 2, m: 67, v: 80, f: "bowed", r: 2 },
    { t: 0, d: 2, m: 64, v: 80, f: "bowed", r: 3 },
    { t: 0, d: 2, m: 48, v: 80, f: "bowed", r: 4 },
    // And one that comes in later, so the set has to change.
    { t: 1, d: 1, m: 55, v: 80, f: "bowed", r: 5 },
  ];
  const p = mount({ notes, duration: 4 });
  p.press();

  p.wind(0.5);
  p.tick();
  assert.deepEqual([...p.lit].sort(), ["1", "2", "3", "4"],
    `lit ${[...p.lit]} — a chord across four staves should light four heads`);

  p.wind(1.5);
  p.tick();
  assert.deepEqual([...p.lit].sort(), ["1", "2", "3", "4", "5"],
    `lit ${[...p.lit]} after the fifth part comes in`);

  // And when they stop, they all stop.
  p.wind(2.5);
  p.tick();
  assert.deepEqual([...p.lit], [], `lit ${[...p.lit]} after the chord ended`);
});

test("a note struck rings once, and a note held does not ring again", () => {
  const notes = [
    { t: 0, d: 2, m: 72, v: 80, f: "bowed", r: 1 },
    { t: 0, d: 2, m: 67, v: 80, f: "bowed", r: 2 },
    { t: 0, d: 2, m: 64, v: 80, f: "bowed", r: 3 },
    { t: 1, d: 1, m: 55, v: 80, f: "bowed", r: 4 },
  ];
  const p = mount({ notes, duration: 4 });
  p.press();

  p.wind(0.2); p.tick();
  assert.equal(p.rings().length, 3, "three heads struck together should ring three times");

  // Held across many frames: still three.
  for (let t = 0.3; t < 1; t += 1 / 60) { p.wind(t); p.tick(); }
  assert.equal(p.rings().length, 3, "a held chord rang again while it was held");

  // The fourth part comes in and rings on its own.
  p.wind(1.2); p.tick();
  assert.equal(p.rings().length, 4, "the note that came in did not ring");
});

test("the playhead has a wash behind it, and both go when it stops", () => {
  const p = mount({ notes: [{ t: 0, d: 1, m: 60, v: 80, f: "flute", r: 1 }], duration: 4 });
  p.press();
  p.wind(0.5); p.tick();

  const wash = p.added.filter((e) => e.cls === "gscore-wash");
  const head = p.added.filter((e) => e.cls === "gscore-cursor");
  assert.equal(wash.length, 1, "no wash was drawn behind the playhead");
  assert.equal(head.length, 1, "no playhead was drawn");
  // The wash reaches back from the playhead, not forward over music unplayed.
  const x = Number(wash[0].attrs.x), w = Number(wash[0].attrs.width);
  assert.ok(x + w <= Number(head[0].attrs.x) + 1.5,
    `the wash (${x}..${x + w}) runs past the playhead (${head[0].attrs.x})`);

  p.media.ended = true;           // no recording here, so end it by the clock
  p.wind(5); p.tick();
  assert.equal(p.added.filter((e) => e.cls === "gscore-wash").length, 0,
    "the wash was left on the page after stopping");
});

// The fault that made pressing Play on the ninth symphony do nothing.
//
// Its systems are fifty-four staves — 4,930 units tall — so the cursor could
// never be wholly on screen, the "is it in view" test was always false, and a
// smooth scroll was asked for on every frame, each one measuring a drawing of
// 1.37 million shapes. The page stopped answering.
test("a system taller than the window is not scrolled at on every frame", () => {
  const bars = [], boxes = [], notes = [];
  for (let i = 0; i < 400; i++) {
    bars.push({ i, t: i * 1.36, d: 1.36 });
    // Each system holds 10 bars and is as tall as the real thing.
    const system = Math.floor(i / 10);
    boxes.push({ i, a: 140 + (i % 10) * 95, b: 140 + (i % 10) * 95 + 90,
                 c: 191 + system * 5200, d: 5121 + system * 5200 });
    notes.push({ t: i * 1.36, d: 1, m: 60, v: 80, f: "bowed", r: i + 1 });
  }
  const p = mount({ notes, bars, boxes, duration: 400 * 1.36 });
  p.press();

  let frames = 0;
  for (let t = 0; t < 400 * 1.36; t += 1 / 60) {
    p.media.currentTime = t;       // no recording here; the synth clock moves
    p.wind(t);
    p.tick();
    frames++;
  }

  // 40 systems, so a few dozen scrolls. Once a frame would be 32,000.
  assert.ok(p.scrolls.length < 200,
    `asked for ${p.scrolls.length} scrolls over ${frames} frames`);
  assert.equal(p.scrolls.filter((s) => s.via === "scrollIntoView").length, 0,
    "scrollIntoView was used on a box that cannot fit the window");
  console.log(`      ${p.scrolls.length} scrolls over ${frames} frames ` +
              `(one a frame would be ${frames})`);
});

// The playhead has a height, which for a long time it did not.
//
// A bar's box is written with its bottom under one key and was read from
// another, so the bottom came back undefined and the height — bottom less
// top — came out NaN. An attribute set to NaN draws nothing: there was no
// visible cursor on any shared page, on any score, and nothing said so.
test("the playhead is as tall as the system it is in", () => {
  const p = mount({
    notes: [{ t: 0, d: 2, m: 60, v: 80, f: "flute", r: 1 }],
    bars: [{ i: 0, t: 0, d: 2 }],
    boxes: [{ i: 0, a: 140, b: 900, c: 191, d: 5121 }],
    duration: 4,
  });
  p.press();
  p.wind(0.5); p.tick();

  const head = p.added.filter((e) => e.cls === "gscore-cursor")[0];
  assert.ok(head, "no playhead was drawn at all");
  const h = Number(head.attrs.height);
  assert.ok(Number.isFinite(h), `the playhead's height is ${head.attrs.height}`);
  // The system is 191..5121, and the cursor is drawn a little past either end.
  assert.ok(h > 4900 && h < 5000, `the playhead is ${h} tall, the system 4930`);
  const y = Number(head.attrs.y);
  assert.ok(Number.isFinite(y) && y < 191, `the playhead starts at ${head.attrs.y}`);
});

test("the drawing is walked once, not once a frame", () => {
  const notes = orchestra(54, 50, 100);
  const p = mount({ notes, duration: 100 });
  p.press();
  for (let t = 0; t <= 100; t += 1 / 60) { p.wind(t); p.tick(); }

  // One pass to find every head, and no per-note attribute selectors after.
  const broad = p.queries.filter((q) => q === "[data-ref]").length;
  const narrow = p.queries.filter((q) => q !== "[data-ref]").length;
  assert.ok(broad <= 1, `the drawing was walked ${broad} times`);
  assert.equal(narrow, 0, `${narrow} per-note selectors were run over the drawing`);
});

// ── the drawing in pieces ───────────────────────────────────────────────
//
// A long score is not written into the page: the page carries the shell and
// fetches the drawing a few systems at a time, because the ninth symphony
// inline is nine hundred thousand shapes and a browser given all of them has
// nothing left over to animate with.

// 104 pieces of four systems each, as gscore writes them for the ninth.
function symphonyBands() {
  const out = [];
  let y = 0;
  for (let i = 0; i < 104; i++) {
    const h = 22600;
    out.push({ i, y0: y, y1: y + h, first: i * 25, last: i * 25 + 24,
               u: `play.${String(i).padStart(3, "0")}.svg` });
    y += h;
  }
  return out;
}

function symphonyScore(bands) {
  const bars = [], boxes = [], notes = [];
  let bar = 0;
  for (const b of bands) {
    for (let k = 0; k <= b.last - b.first; k++, bar++) {
      bars.push({ i: bar, t: bar * 1.36, d: 1.36 });
      boxes.push({ i: bar, a: 140, b: 900,
                   c: b.y0 + 180, d: b.y0 + 4950 });
      notes.push({ t: bar * 1.36, d: 1, r: bar + 1 });
    }
  }
  return { bars, boxes, notes, duration: bar * 1.36 };
}

test("the page asks only for the pieces of the drawing it needs", async () => {
  const bands = symphonyBands();
  const s = symphonyScore(bands);
  const p = mount({ notes: s.notes, bars: s.bars, boxes: s.boxes,
                    duration: s.duration, bands });
  await p.settle();

  assert.ok(p.fetched.length > 0, "no piece of the drawing was asked for");
  assert.ok(p.fetched.length <= 4,
    `asked for ${p.fetched.length} of 104 pieces just to open the page`);
  assert.ok(p.fetched.includes("play.000.svg"),
    `the first piece was not among ${p.fetched}`);
  assert.deepEqual(p.onPage(), [0], `pieces on the page: ${p.onPage()}`);
});

test("playing on brings the next pieces and lets go of the ones behind", async () => {
  const bands = symphonyBands();
  const s = symphonyScore(bands);
  const p = mount({ notes: s.notes, bars: s.bars, boxes: s.boxes,
                    duration: s.duration, bands });
  await p.settle();
  p.press();

  // Play into the fiftieth piece.
  const intoBand = 50;
  const at = bands[intoBand].first * 1.36 + 0.5;
  p.media.currentTime = at;
  p.wind(at);
  p.tick();
  await p.settle();

  const here = p.onPage();
  assert.ok(here.includes(intoBand),
    `playing in piece ${intoBand}, but the page holds [${here}]; ` +
    `fetched [${p.fetched}]`);
  assert.ok(!here.includes(0),
    `the first piece is still on the page while playing piece ${intoBand}`);
  assert.ok(here.length <= 5,
    `${here.length} pieces are on the page at once`);
});

test("a piece arriving makes the notehead index be built again", async () => {
  const bands = symphonyBands();
  const s = symphonyScore(bands);
  const p = mount({ notes: s.notes, bars: s.bars, boxes: s.boxes,
                    duration: s.duration, bands });
  await p.settle();
  p.press();
  p.wind(0.2); p.tick();
  const before = p.queries.filter((q) => q === "[data-ref]").length;

  // Somewhere else entirely, so the drawing is replaced underneath.
  const at = bands[60].first * 1.36 + 0.5;
  p.media.currentTime = at;
  p.wind(at); p.tick();
  await p.settle();
  p.wind(at + 0.1); p.tick();

  const after = p.queries.filter((q) => q === "[data-ref]").length;
  assert.ok(after > before,
    "the heads were not looked up again after the drawing changed, so the " +
    "highlight would be pointing at shapes that have been taken off the page");
});

test("a score written into the page asks for nothing", async () => {
  const p = mount({ notes: [{ t: 0, d: 1, m: 60, v: 80, f: "flute", r: 1 }],
                    duration: 4 });
  await p.settle();
  assert.equal(p.fetched.length, 0,
    `a page carrying its own drawing fetched ${p.fetched}`);
});

test("a recording that will not load is said so, not played silently", () => {
  const p = mount({ notes: [{ t: 0, d: 1, r: 1 }], duration: 10, recording: true });
  p.press();
  p.media.fire("error");

  assert.ok(p.button.disabled, "the button still offers to play nothing");
  assert.equal(p.clockEl.textContent, "no recording");
});
