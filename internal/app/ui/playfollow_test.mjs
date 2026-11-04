/*
 * playfollow_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Following the music while it plays, on a score with a great deal of it.
 *
 * Every frame asked three questions of the whole score: which events are
 * sounding (a walk down every event in the performance), where this bar is
 * (a walk down every clickable region), and which shapes carry a given
 * number (an attribute selector run over the whole drawing). On Beethoven's
 * ninth that is a quarter of a million events, a hundred and thirty-eight
 * thousand regions and nine hundred thousand shapes — sixty times a second.
 * Pressing Play did nothing anybody would call playing.
 *
 * Indexed, each of those is a lookup. What the indexes answer has to be
 * exactly what the walks answered.
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

function build(names, scope) {
  const body = names.map(grab).join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
}

// A performance shaped like an orchestral one: many parts, notes of mixed
// length, and one very long held note to catch a window that is too short.
function performance(parts, bars) {
  const events = [];
  let ref = 1;
  for (let b = 0; b < bars; b++) {
    for (let p = 0; p < parts; p++) {
      events.push({ ref: ref++, part: p, start: b * 2, dur: 0.5 });
      events.push({ ref: ref++, part: p, start: b * 2 + 1, dur: 1.5 });
    }
  }
  // A pedal note under the whole of it.
  events.push({ ref: ref++, part: 0, start: 4, dur: 40 });
  return events;
}

test("what is sounding is what the walk down every event said", () => {
  const api = build(["eventIndex", "soundingAt"], { Math });
  const events = performance(12, 200);
  const when = api.eventIndex(events);
  const plain = (at) => events.filter((e) => at >= e.start && at < e.start + e.dur);

  for (const at of [0, 0.25, 0.5, 1, 1.4, 2, 5, 17.3, 41.9, 44, 100, 399.9]) {
    const got = api.soundingAt(when, at)
      .filter((e) => at >= e.start && at < e.start + e.dur)
      .map((e) => e.ref).sort((a, b) => a - b);
    const want = plain(at).map((e) => e.ref).sort((a, b) => a - b);
    assert.deepEqual(got, want, "at " + at + " seconds");
  }
});

test("a note held under the whole piece is still found", () => {
  // The window reaches back by the longest note there is, so a pedal note
  // forty seconds long is still sounding thirty seconds after it began.
  const api = build(["eventIndex", "soundingAt"], { Math });
  const held = [{ ref: 7, part: 0, start: 4, dur: 40 }];
  const when = api.eventIndex(held);
  const got = api.soundingAt(when, 38).map((e) => e.ref);
  assert.deepEqual(got, [7], "a long note dropped out of the window");
});

test("the window does not grow with the length of the piece", () => {
  const api = build(["eventIndex", "soundingAt"], { Math });
  const short = api.eventIndex(performance(12, 50));
  const long = api.eventIndex(performance(12, 5000));
  const a = api.soundingAt(short, 60).length;
  const b = api.soundingAt(long, 60).length;
  assert.equal(a, b,
    `a piece a hundred times longer looked at ${b} events where a short one ` +
    `looked at ${a}; the window is not bounded`);
});

test("a bar's box is taken from the index, not from every region", () => {
  const state = { hits: [], hitsByBar: null };
  const api = build(["indexHits", "hitsIn"], { state });
  const hits = [];
  for (let b = 0; b < 500; b++) {
    for (let p = 0; p < 10; p++) hits.push({ measure: b, part: p, x0: b, x1: b + 1 });
  }
  state.hits = hits;
  state.hitsByBar = api.indexHits(hits);
  const got = api.hitsIn(123).map((h) => h.part).sort((a, b) => a - b);
  assert.deepEqual(got, [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
  assert.deepEqual(api.hitsIn(99999), [], "a bar that is not there answered with something");

  // And without an index — which is how it behaves before the first draw —
  // it still answers, by walking.
  state.hitsByBar = null;
  assert.equal(api.hitsIn(123).length, 10, "the fallback walk stopped working");
});

test("a sounding note is found by number rather than searched for", () => {
  const fn = src.match(/^function follow\([\s\S]*?^}/m)[0];
  assert.ok(!/querySelectorAll\(`\[data-ref=/.test(fn),
    "every frame still runs an attribute selector over the whole drawing");
  assert.ok(fn.includes("refIndex("), "the shapes are not filed by their number");
  assert.ok(!/for \(const ev of pb\.events\)/.test(fn),
    "every frame still walks every event in the performance");
});

/*
 * Keeping the music on screen.
 *
 * Two faults, which together meant an embedded score played on off the
 * bottom of its frame while the reader scrolled by hand after it.
 *
 * Reported as "when I play the embedded content the page mode does not
 * follow the note being played, so I have to manually move to the right
 * place".
 */

// A view onto a score, with every write to it recorded. Shaped like the
// real element: scrollLeft and scrollTop are properties that can be read
// and written, scrollTo is the smooth one.
function view({ clientHeight, clientWidth = 800, scrollWidth = 800 }) {
  const wrote = { left: 0, smooth: [] };
  let left = 0;
  return {
    wrote,
    el: {
      clientHeight, clientWidth, scrollWidth,
      scrollHeight: 20000,
      scrollTop: 0,
      get scrollLeft() { return left; },
      set scrollLeft(v) { left = v; wrote.left++; },
      scrollTo(opts) { wrote.smooth.push(Math.round(opts.top)); },
    },
  };
}

// followCursor and the handful of things it reaches for. The two flags it
// keeps between frames are passed in, so that assigning to them inside
// behaves as the module's own variables do.
function follower({ player = false, clientHeight = 700 } = {}) {
  const v = view({ clientHeight });
  const api = build(["followCursor"], {
    $: () => v.el,
    state: { zoom: 9, player },
    tuneOffsetY: () => 0,
    readerIsScrolling: () => false,
    userScrolled: false,
    followTop: null,
    cursorLead: 0.32,
    Math,
  });
  return { ...api, ...v };
}

// A score wrapped to the width of its frame has nowhere to go sideways.
// Writing the offset anyway is not free: a write ends whatever scrolling
// is under way, and what was under way was the smooth scroll down to the
// next system that the frame before had asked for. Written sixty times a
// second it cancelled that scroll before it could start, every time.
test("a score with nothing to scroll sideways is left alone sideways", () => {
  const f = follower();
  for (let i = 0; i < 60; i++) {
    f.followCursor({ x: i, top: 0, bottom: 20, measure: 0 });
  }
  assert.equal(f.wrote.left, 0,
    "the sideways offset was written " + f.wrote.left + " times on a score " +
    "that cannot scroll sideways, which cancels the scroll down the page");
});

// And where there is somewhere to go, it still goes.
test("a strip is still followed sideways", () => {
  const f = follower();
  f.el.scrollWidth = 40000;
  f.followCursor({ x: 500, top: 0, bottom: 20, measure: 0 });
  assert.ok(f.wrote.left > 0, "a strip stopped following sideways");
  assert.ok(f.el.scrollLeft > 0, "the strip did not move");
});

// A player's frame is sized to about one system on purpose, so the system
// and the room for it are within a few pixels of each other and the rule
// written for a fifty-three-stave score in a window decided it at random.
// On the wrong side of it, following stopped at the first system.
test("a player follows a system taller than its frame", () => {
  const f = follower({ player: true, clientHeight: 190 });
  // Four staves at nine pixels a space: deeper than the frame.
  f.followCursor({ x: 0, top: 269, bottom: 297, measure: 20 });
  assert.ok(f.wrote.smooth.length > 0,
    "a player stopped following as soon as a system did not fit");
  // And it shows the top of the system, with a little room over it for
  // the ledger lines of a high passage.
  const at = f.wrote.smooth[0];
  assert.ok(at > 0 && at < 269 * 9,
    "scrolled to " + at + ", which is not the top of the system");
});

// The editor keeps the other answer: which of fifty-three staves to watch
// is the reader's decision, so the page is left where they put it.
test("the editor leaves a system deeper than the window alone", () => {
  const f = follower({ player: false, clientHeight: 190 });
  f.followCursor({ x: 0, top: 269, bottom: 297, measure: 20 });
  assert.equal(f.wrote.smooth.length, 0,
    "the editor dragged the page to the top of a system too deep to show");
});

// Within one system nothing moves: the music steps from system to system,
// and a page that re-scrolled on every frame would never settle.
test("following down the page is once per system, not once per frame", () => {
  const f = follower({ clientHeight: 300 });
  for (let i = 0; i < 60; i++) {
    f.followCursor({ x: i, top: 500, bottom: 520, measure: 30 });
  }
  assert.equal(f.wrote.smooth.length, 1,
    "asked for " + f.wrote.smooth.length + " scrolls to the same system");
});
