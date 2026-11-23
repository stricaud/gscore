/*
 * arpeggiopick_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The wavy line rolling a chord is picked up by clicking it.
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

const roll = { part: 0, measure: 3, ref: 9, x: 10, y: 2, width: 0.6, height: 4 };
const at = new Function("state", grab("arpeggioAt") + "; return arpeggioAt;")(
  { arpeggios: [roll] });

test("a click on the wavy line picks it up", () => {
  assert.equal(at(10.3, 4), roll);
});

test("a click a little beside it still does: it is a thin line", () => {
  assert.equal(at(9.7, 4), roll);
});

test("a click on the chord beside it does not", () => {
  assert.equal(at(12, 4), null);
});
