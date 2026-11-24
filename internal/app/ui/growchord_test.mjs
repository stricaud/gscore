/*
 * growchord_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * A chord picked up and Mod with up or down: a note a step past its top or
 * its bottom, left picked up on its own so the arrows move it.
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

function load(el, multi = []) {
  const hit = { part: 0, measure: 4, elements: [el] };
  const added = [];
  const picked = [];
  const state = { multi, hits: [hit] };
  const grow = new Function("state", "selectedElement", "elementBeforeCaret",
    "addPitch", "drawnAt", "select", "status", "modKeyName",
    grab("growChord") + "; return growChord;")(
    state, () => ({ hit, el }), () => null,
    async (h, e, step) => { added.push(step); el.steps = [...el.steps, step]; },
    () => el, (h, e, step) => picked.push(step), () => {}, () => "⌘");
  return { grow, added, picked };
}

test("up adds a step above the top note and picks that note up", async () => {
  const { grow, added, picked } = load({ index: 2, kind: "chord", steps: [0, 2, 4] });
  await grow(1);
  assert.deepEqual(added, [5]);
  assert.deepEqual(picked, [5]);
});

test("down adds a step below the bottom note", async () => {
  const { grow, added } = load({ index: 2, kind: "chord", steps: [3, 5] });
  await grow(-1);
  assert.deepEqual(added, [2]);
});

test("a single note becomes a chord the same way", async () => {
  const { grow, added } = load({ index: 0, kind: "note", steps: [4] });
  await grow(1);
  assert.deepEqual(added, [5]);
});

test("a rest has nothing to add to", async () => {
  const { grow, added } = load({ index: 0, kind: "rest", steps: [] });
  await grow(1);
  assert.deepEqual(added, []);
});

test("several things picked up is not one chord", async () => {
  const { grow, added } = load({ index: 0, kind: "note", steps: [4] }, [{}, {}]);
  await grow(1);
  assert.deepEqual(added, []);
});
