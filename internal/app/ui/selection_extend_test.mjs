/*
 * selection_extend_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Shift with an arrow after a selection made with the mouse.
 *
 * A selection has two ends: one pinned and one that moves. A drag sets both,
 * and shift with an arrow carries on from the end that was dragged to — the
 * way it does in a line of text. Both ways of making a range left the moving
 * end unset, so the first shift-arrow after one started again from the first
 * note and the selection collapsed to two notes instead of growing by one.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

// The functions under test, taken out of the editor as they are written.
const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// One staff of twelve notes in three bars, which is enough for a range to
// grow along and to cross a barline.
function freshState() {
  const hits = [];
  for (let measure = 0; measure < 3; measure++) {
    const elements = [];
    for (let i = 0; i < 4; i++) {
      elements.push({ ref: measure * 4 + i + 1, index: i, kind: "note", steps: [4] });
    }
    hits.push({ part: 0, measure, elements, staffTop: 0, x0: 0, x1: 10 });
  }
  return { hits, multi: [], selection: null, anchor: null, focus: null };
}

const api = (state) => {
  const scope = {
    state,
    markSelection() {},
    status() {},
    rememberCursor() {},
    currentSVG: () => null,
    refTarget: () => null,
    // Bars are numbered from one in every movement. This score has one, so
    // the number over a bar is its place in the file.
    barNumberOf: (m) => m + 1,
    orderedSelection() {
      return [...state.multi].sort((a, b) =>
        a.measure !== b.measure ? a.measure - b.measure : a.index - b.index);
    },
  };
  const body =
    grab("selectRange") + "\n" + grab("elementAlong") + "\n" +
    grab("extendSelection") + "\n" +
    "return { selectRange, elementAlong, extendSelection };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
};

const spread = (state) =>
  state.multi.map((m) => `${m.measure}:${m.index}`).join(" ");

test("shift-arrow after a dragged range grows it from the dragged end", () => {
  const state = freshState();
  const f = api(state);
  // Dragged across the first three notes of the first bar.
  f.selectRange({ part: 0, measure: 0, index: 0 }, { part: 0, measure: 0, index: 2 });
  assert.equal(spread(state), "0:0 0:1 0:2", "the drag itself");

  f.extendSelection(1);
  assert.equal(spread(state), "0:0 0:1 0:2 0:3",
    "shift-right should add the fourth note, not collapse onto the first two");
});

test("shift-arrow the other way takes the dragged end back", () => {
  const state = freshState();
  const f = api(state);
  f.selectRange({ part: 0, measure: 0, index: 0 }, { part: 0, measure: 0, index: 3 });
  f.extendSelection(-1);
  assert.equal(spread(state), "0:0 0:1 0:2",
    "shift-left should give back the note at the moving end");
});

test("a range dragged right to left keeps its own pinned end", () => {
  const state = freshState();
  const f = api(state);
  // Dragged backwards: the pin is the later note and the moving end the
  // earlier one, so shift-left grows the selection.
  f.selectRange({ part: 0, measure: 0, index: 3 }, { part: 0, measure: 0, index: 1 });
  assert.equal(spread(state), "0:1 0:2 0:3", "the drag itself");
  f.extendSelection(-1);
  assert.equal(spread(state), "0:0 0:1 0:2 0:3",
    "shift-left should reach back past the start of the drag");
});

test("growing a range runs on into the next bar", () => {
  const state = freshState();
  const f = api(state);
  f.selectRange({ part: 0, measure: 0, index: 2 }, { part: 0, measure: 0, index: 3 });
  f.extendSelection(1);
  assert.equal(spread(state), "0:2 0:3 1:0",
    "the end of a bar is not the end of a selection");
});

test("the end of the music is the end", () => {
  const state = freshState();
  const f = api(state);
  f.selectRange({ part: 0, measure: 2, index: 2 }, { part: 0, measure: 2, index: 3 });
  f.extendSelection(1);
  assert.equal(spread(state), "2:2 2:3", "nothing to add, and nothing lost either");
});
