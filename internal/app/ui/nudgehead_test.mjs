/*
 * nudgehead_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Moving one notehead of a chord.
 *
 * Clicking a note of a chord and pressing Up moved the whole chord: the
 * one note being corrected came back wrong along with the two that were
 * already right. The endpoint could always move a single notehead — one
 * is named by the staff step it sits on, the way an accidental on one
 * notehead is — and the arrows were not telling it which.
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

// nudging runs the real nudge against a fake mark(), and reports what it
// would have asked the server for.
async function nudging(selection, steps, ok = true) {
  const asked = [];
  const state = { selection, multi: [] };
  const scope = {
    state,
    status: () => {},
    markSelection: () => {},
    pickedNotes: () => true,
    mark: async (endpoint, params) => {
      asked.push({ endpoint, params });
      return ok ? { changed: 1 } : null;
    },
  };
  const body = grab("pickedHead") + "\n" + grab("nudge") + "\nreturn nudge;";
  const fn = new Function(...Object.keys(scope), body)(...Object.values(scope));
  await fn(steps);
  return { asked, state };
}

test("the notehead that was clicked is the one that moves", async () => {
  const { asked } = await nudging(
    { part: 0, measure: 0, at: 1, ref: 7, head: 2 }, 1);
  assert.equal(asked.length, 1, "it did not ask for anything");
  assert.equal(asked[0].endpoint, "nudge");
  assert.equal(asked[0].params.head, "2",
               "it moved the chord instead of the notehead that was clicked");
  assert.equal(asked[0].params.steps, "1");
});

// A notehead is named by where it sits, so moving it moves its name with
// it: a second press has to be aimed at where the note is now.
test("the notehead keeps being the one that moved", async () => {
  const { state } = await nudging(
    { part: 0, measure: 0, at: 1, ref: 7, head: 2 }, 1);
  assert.equal(state.selection.head, 3,
               "a second press would be aimed at the line the note has left");

  const down = await nudging(
    { part: 0, measure: 0, at: 1, ref: 7, head: 4 }, -7);
  assert.equal(down.state.selection.head, -3, "an octave down");
});

// Where the server would not do it, the notehead has not moved and neither
// should its name.
test("a refusal leaves the notehead where it was", async () => {
  const { state } = await nudging(
    { part: 0, measure: 0, at: 1, ref: 7, head: 2 }, 1, false);
  assert.equal(state.selection.head, 2);
});

// A note on its own has one notehead and moves whichever way it is asked;
// naming a notehead there would say nothing.
test("a single note is moved without naming a notehead", async () => {
  const { asked } = await nudging({ part: 0, measure: 0, at: 1, ref: 7 }, 1);
  assert.equal(asked[0].params.head, undefined);
});

// A dragged run is whole chords: a staff step shared by two of them is a
// different note in each, so there is no one notehead to name.
test("a dragged run moves whole chords", async () => {
  const asked = [];
  const state = {
    selection: { part: 0, measure: 0, at: 1, ref: 7, head: 2 },
    multi: [{ part: 0, measure: 0, index: 0 }, { part: 0, measure: 0, index: 1 }],
  };
  const scope = {
    state,
    status: () => {},
    markSelection: () => {},
    pickedNotes: () => true,
    mark: async (endpoint, params) => {
      asked.push(params);
      return { changed: 2 };
    },
  };
  const body = grab("pickedHead") + "\n" + grab("nudge") + "\nreturn nudge;";
  await new Function(...Object.keys(scope), body)(...Object.values(scope))(1);
  assert.equal(asked[0].head, undefined,
               "a notehead was named for a selection of whole chords");
});
