/*
 * articmove_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A mark on a note, clicked, then Up or Down: over the note or under it.
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
  const sent = [];
  const fn = new Function("state", "mark", "status", "t", "articulationLabel",
    "pickedHead", "markSelection",
    grab("articulationPicked") + grab("moveArticulation") + "; return moveArticulation;")(
    state, async (ep, params) => { sent.push([ep, params]); return {}; },
    () => {}, (s) => s, (s) => s, () => ({}), () => {});
  return { move: fn, sent };
}

test("up puts an accent under the note over it", async () => {
  const a = { mark: "accent", above: false };
  const { move, sent } = load({ selection: { articulation: a }, multi: [] });
  assert.equal(await move(1), true);
  assert.deepEqual(sent, [["articulation", { articulation: "accent", placement: "above" }]]);
  assert.equal(a.above, true);
});

test("down puts it back under", async () => {
  const a = { mark: "accent", above: true };
  const { move, sent } = load({ selection: { articulation: a }, multi: [] });
  await move(-1);
  assert.equal(sent[0][1].placement, "below");
});

test("already on that side, nothing is sent", async () => {
  const { move, sent } = load({ selection: { articulation: { mark: "accent", above: true } }, multi: [] });
  assert.equal(await move(1), true);
  assert.equal(sent.length, 0);
});

test("with no mark picked up the arrows are left to the notes", async () => {
  const { move, sent } = load({ selection: { ref: 3 }, multi: [] });
  assert.equal(await move(1), false);
  assert.equal(sent.length, 0);
});
