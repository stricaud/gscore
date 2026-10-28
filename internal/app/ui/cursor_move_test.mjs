/*
 * cursor_move_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Walking the cursor off the end of the music.
 *
 * The bar after the last one is drawn on every staff so that there is
 * somewhere to click before there is anything to click on. The arrow keys
 * could not reach it: they stepped from note to note and an empty bar holds
 * no notes, so the cursor stopped at the last thing written and there was no
 * way from the keyboard to go and start the next bar. Reported as "I am at
 * the right of the caret and it does not push the note writing to the next
 * measure — even if the measure does not exist I should be able to move
 * there to start entering notes".
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

// Bar 7 holds the rest, bar 8 the note, and bar 9 is the empty one drawn
// after the music — the picture in the report.
function freshState(selection) {
  return {
    hits: [
      { part: 0, measure: 6, elements: [{ ref: 1, index: 0, kind: "rest", steps: [] }] },
      { part: 0, measure: 7, elements: [{ ref: 2, index: 0, kind: "note", steps: [8] }] },
      { part: 0, measure: 8, elements: [] },
    ],
    multi: [], selection, anchor: null, focus: null,
  };
}

function load(selection) {
  const state = freshState(selection);
  const said = [];
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection() {},
    scrollSelectionIntoView() {},
    // Bars are numbered from one in every movement. This score has one, so
    // the number over a bar is its place in the file.
    barNumberOf: (m) => m + 1,
  };
  const body = grab("moveCursorBy") + "\nreturn { moveCursorBy };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { state, said, ...api };
}

test("the cursor walks into the empty bar after the music", () => {
  const { state, moveCursorBy } =
    load({ part: 0, measure: 7, at: 1, step: 8, ref: 2 });
  moveCursorBy(1);
  assert.equal(state.selection.measure, 8, "the cursor would not leave the last note");
  assert.equal(state.selection.at, 0, "it should be at the front of the empty bar");
  assert.equal(state.selection.ref, null, "nothing is picked up in an empty bar");
});

test("it says which bar it went to", () => {
  const { said, moveCursorBy } = load({ part: 0, measure: 7, at: 1, step: 8, ref: 2 });
  moveCursorBy(1);
  assert.ok(said.some((s) => s.includes("9")), `said ${JSON.stringify(said)}`);
});

test("and it keeps the pitch it was on, to write at", () => {
  const { state, moveCursorBy } =
    load({ part: 0, measure: 7, at: 1, step: 8, ref: 2 });
  moveCursorBy(1);
  assert.equal(state.selection.step, 8);
});

test("coming back out of the empty bar lands on the last note", () => {
  const { state, moveCursorBy } =
    load({ part: 0, measure: 8, at: 0, step: 8, ref: null });
  moveCursorBy(-1);
  assert.equal(state.selection.measure, 7);
  assert.equal(state.selection.ref, 2);
});

test("walking note to note still works", () => {
  const { state, moveCursorBy } =
    load({ part: 0, measure: 6, at: 1, step: 4, ref: 1 });
  moveCursorBy(1);
  assert.equal(state.selection.measure, 7);
  assert.equal(state.selection.ref, 2);
});

test("the beginning of the music is still the beginning", () => {
  const { state, moveCursorBy } =
    load({ part: 0, measure: 6, at: 1, step: 4, ref: 1 });
  moveCursorBy(-1);
  assert.equal(state.selection.measure, 6);
  assert.equal(state.selection.ref, 1);
});
