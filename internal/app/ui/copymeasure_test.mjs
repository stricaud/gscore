/*
 * copymeasure_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Copying the bar the cursor stands in.
 *
 * Standing in a bar with nothing picked up and pressing Copy answered
 * "Select something to copy", which is no answer at all to somebody who can
 * see where they are. A place in a bar and no note held means the bar.
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

// A bar of three notes, and whatever selection the test is about.
function targets(selection, multi = []) {
  const state = {
    selection,
    multi,
    hits: [{
      part: 0, measure: 13,
      elements: [{ index: 0, ref: 7 }, { index: 1, ref: 8 }, { index: 2, ref: 9 }],
    }],
  };
  const body = grab("selOf") + "\n" + grab("orderedSelection") + "\n" + grab("selectionTargets") + "\n" +
    grab("measureTargets") + "\nreturn () => selectionTargets() || measureTargets();";
  return new Function("state", body)(state)();
}

const place = { part: 0, measure: 13, at: 2, step: 0, ref: null };

test("a cursor in a bar copies the bar", () => {
  assert.equal(targets(place), "0:13:0,0:13:1,0:13:2",
               "standing in a bar, Copy found nothing to copy");
});

// A note in the hand is what was asked for, and the bar around it is not.
test("a note picked up copies that note", () => {
  assert.equal(targets({ part: 0, measure: 13, at: 2, ref: 8 }), "0:13:1");
});

// So is a dragged run: the bar it lies in is not what was dragged over.
test("a dragged run copies the run", () => {
  const multi = [{ part: 0, measure: 13, index: 1 }];
  assert.equal(targets({ ...place, ref: 8 }, multi), "0:13:1");
});

// A whole bar picked up by its band goes on working as it did.
test("a bar picked up copies the bar", () => {
  assert.equal(targets({ ...place, at: 0, bar: true }), "0:13:0,0:13:1,0:13:2");
});

// An empty bar has nothing in it, and saying so beats sending an empty list
// to the server.
test("a cursor in an empty bar copies nothing", () => {
  const state = { selection: place, multi: [], hits: [{ part: 0, measure: 13, elements: [] }] };
  const body = grab("selOf") + "\n" + grab("measureTargets") + "\nreturn measureTargets;";
  assert.equal(new Function("state", body)(state)(), null);
});

// Cut is not given the same fallback: a keystroke meant for one note must
// not empty a bar.
test("cut does not fall back to the bar", () => {
  const cut = grab("cutSelection");
  assert.ok(!cut.includes("measureTargets"),
            "cutSelection falls back to the whole bar, which would empty it");
});
