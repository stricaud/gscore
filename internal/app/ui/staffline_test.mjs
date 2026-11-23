/*
 * staffline_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A line from one staff to the other is picked up by clicking it, anywhere
 * along it but at its ends, which belong to the note and the rest.
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

const line = { from: 12, to: 40, x1: 10, y1: 6, x2: 12, y2: 20 };
const at = new Function("state",
  grab("awayFromSegment") + grab("staffLineAt") + "; return staffLineAt;")(
  { staffLines: [line] });

test("a click on the middle of the line picks it up", () => {
  assert.equal(at(11.1, 13), line);
});

test("a click beside it, too far off, does not", () => {
  assert.equal(at(13, 13), null);
});

test("a click at either end is left to the note or the rest there", () => {
  assert.equal(at(10, 6.2), null);
  assert.equal(at(12, 19.9), null);
});
