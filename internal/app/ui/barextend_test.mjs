/*
 * barextend_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A bar picked up, then shift with a click on another bar of the same staff:
 * every bar between them. Only the band over the bar did this; a barline or
 * a click in the bar started a new selection instead.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

function load(state) {
  const picked = [];
  const extend = new Function("state", "selectMeasure",
    grab("extendBars") + "; return extendBars;")(
    state, (part, from, to) => picked.push([part, from, to]));
  return { extend, picked };
}

test("shift on another bar of the staff takes in the bars between", () => {
  const { extend, picked } = load({
    selection: { part: 0, measure: 3, bar: true, barTo: 3 },
    barAnchor: { part: 0, measure: 3 },
  });
  assert.equal(extend(0, 8), true);
  assert.deepEqual(picked, [[0, 3, 8]]);
});

test("backwards too, from the bar it began at", () => {
  const { extend, picked } = load({
    selection: { part: 1, measure: 5, bar: true, barTo: 9 },
    barAnchor: { part: 1, measure: 9 },
  });
  assert.equal(extend(1, 2), true);
  assert.deepEqual(picked, [[1, 9, 2]]);
});

test("nothing to extend: no bar picked up, or another staff", () => {
  assert.equal(load({ selection: { part: 0, measure: 3, ref: 7 },
    barAnchor: { part: 0, measure: 3 } }).extend(0, 8), false);
  assert.equal(load({ selection: { part: 0, measure: 3, bar: true },
    barAnchor: { part: 0, measure: 3 } }).extend(1, 8), false);
  assert.equal(load({ selection: null, barAnchor: null }).extend(0, 8), false);
});

// Both ways in reach it: a barline, and a click anywhere in the bar.
test("the barline and the bar both ask for it with shift", () => {
  const click = src.slice(src.indexOf('$("#canvas").addEventListener("click"'));
  assert.match(click, /if \(!extendBars\(bar\.part, bar\.measure\)\) selectMeasure/);
  assert.match(click, /if \(e\.shiftKey && extendBars\(hit\.part, hit\.measure\)\) return;/);
});

// The case reported: a note or the cursor standing in bar 1, then shift in
// the empty part of bar 4. That bar is drawn boxed, so it reads as a bar
// picked up, and the shift-click only moved the cursor.
test("shift in a bar's empty part takes in the bars from the cursor's", () => {
  const picked = [];
  const state = { selection: { part: 0, measure: 0, ref: null, at: 1 }, barAnchor: null };
  const start = new Function("state", "selectMeasure",
    grab("startBarsFrom") + "; return startBarsFrom;")(
    state, (part, from, to) => picked.push([part, from, to]));
  assert.equal(start(0, 3), true);
  assert.deepEqual(picked, [[0, 0, 3]]);
  assert.deepEqual(state.barAnchor, { part: 0, measure: 0 },
    "shift-clicking on carries on from the bar it started from");
  // Another staff, or nothing in hand, is a click like any other.
  state.selection = { part: 1, measure: 0, at: 1 };
  assert.equal(start(0, 3), false);
  state.selection = null;
  assert.equal(start(0, 3), false);
});

test("the empty part of a bar asks for it with shift", () => {
  const click = src.slice(src.indexOf('$("#canvas").addEventListener("click"'));
  assert.match(click, /if \(e\.shiftKey && startBarsFrom\(hit\.part, hit\.measure\)\) return;/);
});
