/*
 * barleft_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A bar picked up and an arrow pressed: the caret leaves the bar by the side
 * the arrow points, and does not go to the end of the score.
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
const placeNear = new Function(grab("placeNear") + "; return placeNear;")();

// Bars 26, 27, 28, 29 with three notes each, end to end.
const line = [];
for (const measure of [26, 27, 28, 29]) {
  for (const index of [0, 1, 2]) line.push({ measure, index, ref: measure * 10 + index });
}
const pos = (i) => [line[i].measure, line[i].index];

test("left from a bar goes to just before it, not to the last bar", () => {
  const sel = { measure: 28, at: 0, ref: null, bar: true, barTo: 28 };
  assert.deepEqual(pos(placeNear(line, sel, -1)), [27, 2]);
});

test("right from a bar goes to its end", () => {
  const sel = { measure: 28, at: 0, ref: null, bar: true, barTo: 28 };
  assert.deepEqual(pos(placeNear(line, sel, 1)), [28, 2]);
});

test("right from a run of bars goes to the end of the run", () => {
  const sel = { measure: 27, at: 0, ref: null, bar: true, barTo: 28 };
  assert.deepEqual(pos(placeNear(line, sel, 1)), [28, 2]);
});

test("from the head of a bar, left is the note before and right its first", () => {
  const sel = { measure: 28, at: 0, ref: null };
  assert.deepEqual(pos(placeNear(line, sel, -1)), [27, 2]);
  assert.deepEqual(pos(placeNear(line, sel, 1)), [28, 0]);
});

test("from the first bar, left stays at the start", () => {
  const sel = { measure: 26, at: 0, ref: null, bar: true, barTo: 26 };
  assert.equal(placeNear(line, sel, -1), 0);
});
