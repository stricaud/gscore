/*
 * carrycaret_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Filling a bar carries the writing point into the next one.
 *
 * That is what makes typing music in feel like typing: the last note of a bar
 * is followed by the first note of the one after it, and nobody has to say
 * so. It asked whether the bar was full, which on a staff two voices share is
 * the wrong question twice over — a bar whose music is all in the second
 * voice never reported itself full at all, and a caret at the end of one
 * voice was measured against the notes of both. Reported as "entering the
 * last note should have moved to the next bar. Why it didn't?"
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

// carry runs carryCaretOn against a staff of its own and gives back where the
// caret finished up.
function carry(hits, sel) {
  const state = { hits };
  const fn = new Function("state",
    grab("carryCaretOn") + "\nreturn carryCaretOn;")(state);
  const where = { ...sel };
  fn(where);
  return where;
}

// bar is one measure of the first staff, with n notes in each named voice.
function bar(measure, voices, fullVoices) {
  const elements = [];
  for (const [voice, n] of Object.entries(voices)) {
    for (let i = 0; i < n; i++) {
      elements.push({ voice: Number(voice), index: i, ref: measure * 100 + i });
    }
  }
  return { part: 0, measure, elements, fullVoices,
           full: (fullVoices || []).length > 0 };
}

test("a full first voice carries the caret on", () => {
  const hits = [bar(0, { 0: 4 }, [0]), bar(1, { 0: 0 }, [])];
  const got = carry(hits, { part: 0, measure: 0, at: 4, voice: 0 });
  assert.equal(got.measure, 1);
  assert.equal(got.at, 0);
});

// The staff's music written in its second voice is the staff's music.
test("a full second voice carries it too", () => {
  const hits = [bar(0, { 1: 4 }, [1]), bar(1, { 1: 0 }, [])];
  const got = carry(hits, { part: 0, measure: 0, at: 4, voice: 1 });
  assert.equal(got.measure, 1, "the caret stayed in a bar that was full");
  assert.equal(got.at, 0);
});

// One voice full is not the other voice full: the line the pen is on is what
// decides, not the bar around it.
test("a full other voice leaves this one alone", () => {
  const hits = [bar(0, { 0: 1, 1: 1 }, [0]), bar(1, {}, [])];
  const got = carry(hits, { part: 0, measure: 0, at: 1, voice: 1 });
  assert.equal(got.measure, 0, "a note in the second voice jumped the barline");
});

// The caret is counted against its own line's notes. Measured against the
// whole bar, a caret at the end of a full second voice looks like a caret
// halfway along.
test("the caret is counted in its own voice", () => {
  const hits = [bar(0, { 0: 4, 1: 4 }, [0, 1]), bar(1, {}, [])];
  const got = carry(hits, { part: 0, measure: 0, at: 4, voice: 1 });
  assert.equal(got.measure, 1);
});

// Mid-bar, nothing moves: this is about writing at the end of a bar.
test("a caret in the middle of a bar stays put", () => {
  const hits = [bar(0, { 0: 4 }, [0]), bar(1, {}, [])];
  const got = carry(hits, { part: 0, measure: 0, at: 2, voice: 0 });
  assert.equal(got.measure, 0);
  assert.equal(got.at, 2);
});

// Nothing after it yet: the server makes the bar, so the caret waits.
test("there has to be a bar to carry on into", () => {
  const hits = [bar(0, { 0: 4 }, [0])];
  const got = carry(hits, { part: 0, measure: 0, at: 4, voice: 0 });
  assert.equal(got.measure, 0);
});

// A layout from a build that does not say which voices are full still works
// for the first voice, which is what it used to answer for.
test("an older layout still carries the first voice", () => {
  const hits = [{ part: 0, measure: 0, full: true,
                  elements: [{ voice: 0, index: 0 }] },
                { part: 0, measure: 1, elements: [] }];
  const got = carry(hits, { part: 0, measure: 0, at: 1, voice: 0 });
  assert.equal(got.measure, 1);
});

/*
 * And writing is not the only way a bar fills.
 *
 * A dot is a note getting longer, and the note that gets longer is usually
 * the last one of the bar. Writing carried the caret over the barline when it
 * filled a bar; dotting did not, so the caret sat inside a bar with no room
 * left in it. Bar 46 of Bumpty Bump ends on a dotted quarter, which is how it
 * was found: "entering the last note should have moved to the next bar. Why
 * it didn't?"
 */

test("dotting carries the caret on", () => {
  const src2 = grab("dotSelection");
  assert.ok(src2.includes("carryCaretOn"),
    "dotSelection no longer carries the caret over a filled barline");
});

test("changing a note's length carries the caret on", () => {
  const src2 = grab("setSelectionDuration");
  assert.ok(src2.includes("carryCaretOn"),
    "a note made longer no longer carries the caret");
});

// The caret and the note part company the moment a bar fills, so a command
// that names a place in a bar has to name the note's place, not the caret's.
// Otherwise the second dot of a double dot lands in the bar after.
function aimAt(hits, sel) {
  const state = { hits };
  const fn = new Function("state",
    grab("aimedAt") + "\nreturn aimedAt;")(state);
  return fn(sel);
}

test("a picked-up note is aimed at by its own place", () => {
  const hits = [
    { part: 0, measure: 0,
      elements: [{ ref: 7, index: 4, voice: 0 }] },
    { part: 0, measure: 1, elements: [] },
  ];
  // The caret has been carried into the next bar; the note has not moved.
  const got = aimAt(hits, { part: 0, measure: 1, at: 0, voice: 0, ref: 7 });
  assert.deepEqual(got, { part: 0, measure: 0, at: 5, voice: 0 });
});

test("with nothing picked up it is the caret", () => {
  const got = aimAt([], { part: 0, measure: 3, at: 2, voice: 1, ref: null });
  assert.deepEqual(got, { part: 0, measure: 3, at: 2, voice: 1 });
});

// picked asks emptyBarPicked which bar Delete would take away.
function picked(hits, selection, multi = []) {
  const state = { hits, selection, multi };
  return new Function("state",
    grab("emptyBarPicked") + "\n" + grab("barHasMusic") + "\n" +
    grab("selectedBars") + "\nreturn emptyBarPicked;")(state)();
}

test("delete on an empty bar means the bar", () => {
  const note = { kind: "note", index: 0 };
  const rest = { kind: "rest", index: 0 };
  const hits = [
    { part: 0, measure: 3, elements: [note] },
    { part: 0, measure: 4, elements: [] },
    { part: 0, measure: 5, elements: [rest] },
    { part: 1, measure: 3, elements: [] },
    { part: 1, measure: 4, elements: [] },
    { part: 1, measure: 5, elements: [] },
    { part: 0, measure: 6, elements: [] },
    { part: 1, measure: 6, elements: [note] },
  ];
  // Clicked into, which draws the bar boxed: that bar, not the note before.
  assert.equal(picked(hits, { part: 0, measure: 4, at: 0, ref: null, placed: true }), 4);
  // A bar of rest is an empty bar.
  assert.equal(picked(hits, { part: 0, measure: 5, at: 1, ref: null, placed: true }), 5);
  // Picked up as a bar.
  assert.equal(picked(hits, { part: 0, measure: 4, bar: true, barTo: 4 }), 4);
  // Something in it: Delete empties it, as it did.
  assert.equal(picked(hits, { part: 0, measure: 3, bar: true, barTo: 3 }), null);
  // Empty on this staff and not on another: the bar stays.
  assert.equal(picked(hits, { part: 0, measure: 6, at: 0, ref: null, placed: true }), null);
  // A run of bars is emptied, not removed.
  assert.equal(picked(hits, { part: 0, measure: 4, bar: true, barTo: 5 }), null);
  // The caret carried on by writing is still behind the note it wrote.
  assert.equal(picked(hits, { part: 0, measure: 4, at: 0, ref: null }), null);
  assert.equal(
    carry([{ part: 0, measure: 3, elements: [{ voice: 0 }], full: true },
           { part: 0, measure: 4, elements: [] }],
          { part: 0, measure: 3, at: 1, placed: true }).placed, false);
});
