/*
 * followcost_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What following the music costs per frame, and how often it moves the page.
 *
 * Two things made it heavy on a long score rather than wrong on a short one,
 * which is why neither showed up until the ninth symphony. Finding the bar
 * walked down every bar from the first, so a frame cost what the piece had
 * already played. And moving down the page asked for a smooth scroll on
 * every frame the cursor was out of view, each request restarting the easing
 * the last had got partway through.
 *
 * Both are about how much work a frame does, so that is what is measured
 * here: the answers must not change, and the work must not grow.
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

// The module-level places these two keep between frames are declared here,
// because that is what they are in app.js and the behaviour under test is
// precisely that they are kept.
function build(names, scope, prelude = "") {
  const body = prelude + "\n" + names.map(grab).join("\n") +
    "\nreturn { " + names.join(", ") + ", peek: () => ({ cursorFrom, followTop })" +
    (/setScrolling/.test(prelude) ? ", setScrolling" : "") + " };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
}

// Bars of a fixed length, in playing order: starts only increase.
function bars(n, dur = 2) {
  const out = [];
  for (let i = 0; i < n; i++) out.push({ index: i, start: i * dur, dur });
  return out;
}

// A list that counts how many times a frame looks at a bar's start, which is
// the work the walk down the bars does.
function counted(list) {
  let reads = 0;
  const seen = list.map((m) => {
    const o = { index: m.index, dur: m.dur };
    Object.defineProperty(o, "start", { get() { reads++; return m.start; } });
    return o;
  });
  return { seen, reads: () => reads };
}

const BOX = { x0: 0, x1: 10, top: 0, bottom: 8 };
const cursorScope = {
  Math,
  measureBox: (i) => (i >= 0 ? { ...BOX, x0: i * 10, x1: i * 10 + 10 } : null),
};

test("the bar under the cursor is the one the walk from the first bar found", () => {
  const api = build(["cursorAt", "cursorX"], cursorScope, "let cursorFrom = 0;");
  const ms = bars(400);

  // The reference: what app.js did before, kept here as the thing the fast
  // answer has to agree with.
  const slow = (measures, now) => {
    let m = measures[0];
    for (const cand of measures) {
      if (now >= cand.start) m = cand;
      else break;
    }
    return m.index;
  };

  // Forward through the piece, the way playing goes.
  for (let t = 0; t < 800; t += 0.25) {
    assert.equal(api.cursorAt(ms, t)?.measure, slow(ms, t), `at ${t}s`);
  }
  // Before the first bar, and past the last.
  assert.equal(api.cursorAt(ms, -5)?.measure, slow(ms, -5));
  assert.equal(api.cursorAt(ms, 99999)?.measure, slow(ms, 99999));
});

test("a seek backwards and a repeat going back still land on the right bar", () => {
  const api = build(["cursorAt", "cursorX"], cursorScope, "let cursorFrom = 0;");
  const ms = bars(400);

  assert.equal(api.cursorAt(ms, 700)?.measure, 350);
  // Straight back to the top, as pressing stop then play does.
  assert.equal(api.cursorAt(ms, 0)?.measure, 0);
  assert.equal(api.cursorAt(ms, 700)?.measure, 350);
  // And a short way back, as a repeat does.
  assert.equal(api.cursorAt(ms, 690)?.measure, 345);
  assert.equal(api.cursorAt(ms, 692)?.measure, 346);
});

test("a frame late in the piece costs no more than a frame early in it", () => {
  const api = build(["cursorAt", "cursorX"], cursorScope, "let cursorFrom = 0;");
  const ms = bars(3222);                       // the ninth, in bars
  const { seen, reads } = counted(ms);

  // What one frame costs, as bar starts looked at, averaged over a second
  // or so of playing.
  const perFrame = (from, to) => {
    const step = 1 / 60;
    const before = reads();
    let n = 0;
    for (let t = from; t < to; t += step) { api.cursorAt(seen, t); n++; }
    return (reads() - before) / n;
  };

  const early = perFrame(0, 10);
  perFrame(10, 6000);                          // play on, deep into it
  const late = perFrame(6000, 6010);

  assert.ok(late < 5, `a late frame read ${late.toFixed(1)} bar starts`);
  assert.ok(late <= early * 2,
    `a late frame (${late.toFixed(1)}) cost more than an early one (${early.toFixed(1)})`);

  // And the walk this replaced, measured the same way, to show that what is
  // counted here is the thing that was heavy: at 6000 seconds in it reads
  // every bar the piece has already played.
  const { seen: seen2, reads: reads2 } = counted(ms);
  const before = reads2();
  for (let t = 6000; t < 6010; t += 1 / 60) {
    let m = seen2[0];
    for (const cand of seen2) {
      if (t >= cand.start) m = cand;
      else break;
    }
  }
  const slowLate = (reads2() - before) / 600;
  assert.ok(slowLate > 2000,
    `the walk from the first bar should be thousands a frame, was ${slowLate.toFixed(0)}`);
  console.log(`      bar starts read per frame at 6000s: ${late.toFixed(1)} now, ` +
              `${slowLate.toFixed(0)} before`);
});

// ── moving the page down ────────────────────────────────────────────────
function viewport({ clientHeight = 600, scrollTop = 0, scrollWidth = 900 } = {}) {
  const moved = [];
  const wrap = {
    clientHeight, clientWidth: 900, scrollWidth, scrollHeight: 100000,
    scrollTop, scrollLeft: 0,
    scrollTo: (to) => { moved.push(to); wrap.scrollTop = to.top; },
  };
  return { wrap, moved };
}

function follower(wrap) {
  return build(["followCursor"], {
    Math,
    $: () => wrap,
    state: { zoom: 1, showAll: false },
    tuneOffsetY: () => 0,
    cursorLead: 0.32,
  }, "let userScrolled = false;\nlet followTop = null;\nlet cursorFrom = 0;\n" +
     // The reader is not touching the page in these: what is under test is
     // what the music does when it is left to itself. There is a test of
     // its own below for what happens when they are.
     "let scrolling = false;\nfunction readerIsScrolling() { return scrolling; }\n" +
     "function setScrolling(v) { scrolling = v; }");
}

test("one move down the page is one scroll, not one on every frame", () => {
  const { wrap, moved } = viewport();
  const api = follower(wrap);

  // A system well below the view. The cursor sits on it for many frames,
  // which is what playing through a system is.
  const pt = { x: 100, top: 4000, bottom: 4100, measure: 7 };
  for (let f = 0; f < 60; f++) api.followCursor(pt);

  assert.equal(moved.length, 1,
    `asked for ${moved.length} scrolls for one move down the page`);
  assert.equal(moved[0].behavior, "smooth");
  assert.equal(moved[0].top, 4000 - 600 * 0.3);
});

test("the next system down is a scroll of its own", () => {
  const { wrap, moved } = viewport();
  const api = follower(wrap);

  for (let f = 0; f < 30; f++) {
    api.followCursor({ x: 100, top: 4000, bottom: 4100, measure: 7 });
  }
  // The smooth scroll has landed, so the view now holds that system; a
  // frame there puts following back on and forgets where it was sent.
  wrap.scrollTop = 3820;
  api.followCursor({ x: 100, top: 4000, bottom: 4100, measure: 7 });
  assert.equal(api.peek().followTop, null, "arriving did not clear the target");

  for (let f = 0; f < 30; f++) {
    api.followCursor({ x: 100, top: 4600, bottom: 4700, measure: 11 });
  }
  assert.equal(moved.length, 2,
    `two systems should be two scrolls, got ${moved.length}`);
  assert.equal(moved[1].top, 4600 - 600 * 0.3);
});

test("sideways is still set every frame, so the cursor is never left behind", () => {
  const { wrap } = viewport();
  const api = follower(wrap);
  wrap.scrollWidth = 10000;

  api.followCursor({ x: 500, top: 0, bottom: 100, measure: 1 });
  assert.equal(wrap.scrollLeft, 500 - 900 * 0.32);
  api.followCursor({ x: 600, top: 0, bottom: 100, measure: 1 });
  assert.equal(wrap.scrollLeft, 600 - 900 * 0.32);
});

/*
 * Looking at the other staves while it plays.
 */

// The music keeps its hands off the page while the reader is moving it.
//
// Setting a scroll offset does not merely move the page: it ends whatever
// scrolling was already under way, momentum and all. Following sets the
// sideways offset on every frame, so a two-finger scroll down the staves of
// a large score was being cancelled sixty times a second — the gesture
// arrived, moved the page a hair, and was killed before it could carry.
test("a gesture is not written over by the following", () => {
  // A score long enough to have somewhere to follow to.
  const { wrap, moved } = viewport({ scrollWidth: 100000 });
  const api = follower(wrap);

  // Playing along, with the page to itself: the sideways offset is set.
  api.followCursor({ x: 2000, top: 10, bottom: 110, measure: 1 });
  assert.ok(wrap.scrollLeft > 0, "the music did not follow at all");

  // The reader starts scrolling, and puts the page where they want it.
  api.setScrolling(true);
  wrap.scrollLeft = 4242;
  wrap.scrollTop = 1234;
  for (let f = 0; f < 60; f++) {
    api.followCursor({ x: 3000, top: 10, bottom: 110, measure: 3 });
  }
  assert.equal(wrap.scrollLeft, 4242,
    "the music wrote over the reader's own scrolling, which on a real page " +
    "ends the gesture that was under way");
  assert.equal(wrap.scrollTop, 1234, "and it moved them down the page as well");
  assert.equal(moved.length, 0, "it asked for a scroll of its own as well");

  // They stop, and the music picks the cursor up again.
  api.setScrolling(false);
  api.followCursor({ x: 3000, top: 10, bottom: 110, measure: 3 });
  assert.ok(wrap.scrollLeft !== 4242, "following never resumed");
});

// A system deeper than the window has no scroll position that shows all of
// it, so there is no bringing it into view — which of fifty-three staves to
// watch is the reader's to choose.
test("music deeper than the window is left where the reader put it", () => {
  const { wrap, moved } = viewport({ scrollWidth: 100000 });
  const api = follower(wrap);
  wrap.scrollTop = 2500;

  // The band is eight thousand deep and the window six hundred.
  for (let f = 0; f < 60; f++) {
    api.followCursor({ x: 2000, top: 0, bottom: 8000, measure: 2 });
  }
  assert.equal(moved.length, 0,
    "it tried to bring a system taller than the window into view, which " +
    "pins the page to the top of the score for the whole piece");
  assert.equal(wrap.scrollTop, 2500, "and moved the reader off their staves");

  // Sideways still follows: the strip is one long line and the cursor has
  // to be kept on screen or there is nothing to watch.
  assert.ok(wrap.scrollLeft > 0, "it stopped following sideways too");
});
