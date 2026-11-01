/*
 * undoplace_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the caret is after taking something back.
 *
 * Undo went through the same path as opening a file: the selection was
 * dropped and the caret put back wherever the score was last left, which on
 * a piece being written from the top is bar one. Writing a wrong note in
 * bar 40 and taking it back sent you to the beginning, and the way to carry
 * on was to find bar 40 again.
 *
 * Undo is not a change of place. It is the same place with one thing less
 * in it.
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

// putBack runs the real restoreCaret against a score of the given shape.
function putBack(was, { parts = 1, bars = 8, elements = 4 } = {}) {
  const hits = [];
  for (let p = 0; p < parts; p++) {
    for (let m = 0; m < bars; m++) {
      hits.push({
        part: p, measure: m,
        elements: new Array(elements).fill(0).map((_, i) => ({ ref: i })),
      });
    }
  }
  const state = {
    selection: null, multi: [{ part: 0, measure: 0, index: 0 }],
    anchor: { part: 0, measure: 0, index: 0 }, focus: { part: 0, measure: 0, index: 1 },
    hits, info: { parts: new Array(parts).fill({ name: "" }) },
  };
  const scope = { state, markSelection: () => { state.marked = true; } };
  const body = grab("restoreCaret") + "\nreturn restoreCaret;";
  new Function(...Object.keys(scope), body)(...Object.values(scope))(was);
  return state;
}

test("the caret stays in the bar it was in", () => {
  const s = putBack({ part: 0, measure: 39, at: 2, step: 4 }, { bars: 64 });
  assert.equal(s.selection.measure, 39, "it was carried off to another bar");
  assert.equal(s.selection.at, 2);
  assert.equal(s.selection.step, 4, "the pitch it was pointing at was lost");
});

// What it was pointing at is what was taken back, so there is nothing
// picked up and nothing for a mark to land on.
test("nothing is left picked up", () => {
  const s = putBack({ part: 0, measure: 3, at: 2, step: 4, ref: 11, replace: true });
  assert.equal(s.selection.ref, null, "a stale note is still picked up");
  assert.equal(s.selection.replace, false);
  assert.deepEqual(s.multi, [], "a dragged run of stale refs was kept");
  assert.equal(s.anchor, null);
  assert.equal(s.focus, null);
});

// The bar the caret was in may be the bar that was taken away.
test("a caret past the end of the music comes back to the last bar", () => {
  const s = putBack({ part: 0, measure: 20, at: 1, step: 4 }, { bars: 8 });
  assert.equal(s.selection.measure, 7, "it pointed at a bar that is no longer there");
});

// And the note it stood after may be the note that was taken back.
test("a caret past the last note comes back to the end of the bar", () => {
  const s = putBack({ part: 0, measure: 2, at: 9, step: 4 }, { elements: 3 });
  assert.equal(s.selection.at, 3, "it pointed past the end of the bar");
});

test("a caret on a staff that is gone comes back to one that is there", () => {
  const s = putBack({ part: 5, measure: 1, at: 0, step: 4 }, { parts: 2 });
  assert.equal(s.selection.part, 1);
});

test("with nothing written there is nothing to put back", () => {
  const s = putBack({ part: 0, measure: 0, at: 0, step: 4 }, { bars: 0 });
  assert.equal(s.selection, null);
});

// The view must not move either: the caret being in the right place is no
// use if the page has scrolled to the top.
test("undo redraws where it stands, and does not reopen the score", () => {
  const fn = grab("stepHistory");
  assert.match(fn, /restoreCaret\(was\)/, "the caret is not put back");
  assert.match(fn, /await refreshScore\(\)/, "the score is not redrawn");
  assert.ok(!/scrollSelectionIntoView/.test(fn),
            "undo scrolls the view, which is the thing it must not do");
  // reload() is the open-a-file path: it drops the selection and places the
  // caret where the score was last left. It is right for one case only.
  const parts = fn.split("await reload();");
  assert.equal(parts.length, 2,
    "undo still reopens the score; that is what carried the caret away");
  assert.match(parts[0], /d\.tune !== state\.info\?\.current/,
    "the score is reopened for something other than landing in another tune");
});

test("restoreCaret does not scroll", () => {
  assert.ok(!/scrollSelectionIntoView/.test(grab("restoreCaret")),
            "putting the caret back moves the view with it");
});
