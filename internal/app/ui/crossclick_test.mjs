/*
 * crossclick_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A left-hand note written on the treble staff is picked up by clicking it,
 * though the treble staff is the nearer one.
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

function load(hits) {
  return new Function("state", "staffHeightOf",
    grab("elementAt") + grab("awayFromElement") + grab("hitAt") + "; return hitAt;")(
    { hits }, () => 4);
}

// The treble staff at 0..4 with a right-hand note at x 3; the bass staff
// at 12..16, whose left-hand note at x 6 is drawn up on the treble staff.
const treble = { part: 0, measure: 27, x0: 0, x1: 10, staffTop: 0,
  elements: [{ x: 3, y: 3, width: 1.2 }] };
const bass = { part: 1, measure: 27, x0: 0, x1: 10, staffTop: 12,
  elements: [{ x: 6, y: 3.5, width: 1.2 }] };

test("a click on the left hand's note on the treble staff is the bass staff's", () => {
  const hitAt = load([treble, bass]);
  assert.equal(hitAt(6.5, 3.5), bass);
});

test("a click on the right hand's note there is still the treble's", () => {
  const hitAt = load([treble, bass]);
  assert.equal(hitAt(3.5, 3), treble);
});

test("a click on empty treble staff goes to the nearer staff, as before", () => {
  const hitAt = load([treble, bass]);
  assert.equal(hitAt(9, 1), treble);
});
