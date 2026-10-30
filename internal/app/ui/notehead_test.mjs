/*
 * notehead_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Picking up one notehead of a chord.
 *
 * Clicking a note of a chord picked up the whole chord, so a sharp asked for
 * there went on every note in it and there was no way at all to sharpen the
 * third. Reported as: "I want to add the sharp in front of a specific note
 * but I cannot."
 *
 * A click says which notehead. A drag still says the whole chord, which is
 * what makes it the way to sharpen all of them at once.
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

// E4, G4 and B4 in the treble: the bottom line, the line above it, and the
// middle line. Steps are half a space each, counted from the bottom line.
const CHORD = { ref: 7, index: 0, kind: "chord", x: 10, y: 4, y2: 6,
                width: 1.2, steps: [0, 2, 4] };
const LONE = { ref: 9, index: 1, kind: "note", x: 14, y: 5, y2: 5,
               width: 1.2, steps: [1] };
const HITS = [{ part: 0, measure: 0, staffTop: 6, elements: [CHORD, LONE] }];

function load(names, extra = {}) {
  const said = [];
  const state = { hits: HITS, multi: [], selection: null, anchor: null,
                  focus: null, zoom: 8, ...(extra.state || {}) };
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection() {},
    updateFingering() {},
    barNumberOf: (m) => m + 1,
  };
  const body = names.map(grab).join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, state, said };
}

test("clicking a notehead of a chord says which one", () => {
  const t = load(["select"]);
  t.select(HITS[0], CHORD, 2);
  assert.equal(t.state.selection.head, 2, "the middle notehead was clicked");
  assert.equal(t.state.selection.ref, 7, "and the chord is still what is picked up");
});

// A notehead is a space tall, so anywhere within one step of its centre is
// on it. Asking for an exact match meant the middle quarter of the head:
// everywhere else on it picked the chord up and then could not say which
// note had been pointed at.
test("the whole notehead counts, not its exact middle", () => {
  const t = load(["select"]);
  t.select(HITS[0], CHORD, 5);
  assert.equal(t.state.selection.head, 4, "the top notehead, clicked high on it");
});

test("a click between two noteheads takes the nearer one", () => {
  const t = load(["select"]);
  t.select(HITS[0], CHORD, 1);
  assert.ok(t.state.selection.head === 0 || t.state.selection.head === 2,
            "got " + t.state.selection.head);
});

test("a click on no notehead at all leaves the chord whole", () => {
  const t = load(["select"]);
  t.select(HITS[0], CHORD, 9);
  assert.equal(t.state.selection.head, null,
               "nothing was pointed at, so the accidental belongs to the chord");
});

test("the status line names the notehead", () => {
  const t = load(["select"]);
  t.select(HITS[0], CHORD, 2);
  assert.ok(t.said.join(" ").includes("notehead"),
            "nothing said which note is picked up: " + t.said.join(" | "));

  const u = load(["select"]);
  u.select(HITS[0], LONE, 1);
  assert.ok(!u.said.join(" ").includes("notehead"),
            "a note on its own has no notehead to name: " + u.said.join(" | "));
});

test("a notehead is sent with the accidental, and only from a click", () => {
  const t = load(["pickedHead"],
    { state: { selection: { part: 0, measure: 0, ref: 7, head: 2 } } });
  assert.deepEqual(t.pickedHead(), { head: "2" });

  // A dragged range is whole chords: a step names a different note in each.
  const d = load(["pickedHead"], { state: {
    selection: { part: 0, measure: 0, ref: 7, head: 2 },
    multi: [{ part: 0, measure: 0, ref: 7 }, { part: 0, measure: 0, ref: 9 }],
  } });
  assert.deepEqual(d.pickedHead(), {}, "a drag sent one notehead");

  const n = load(["pickedHead"],
    { state: { selection: { part: 0, measure: 0, ref: 9, head: null } } });
  assert.deepEqual(n.pickedHead(), {});
});

test("what is said afterwards tells one note from the whole chord", () => {
  const t = load(["onThatNotehead"],
    { state: { selection: { part: 0, measure: 0, ref: 7, head: 2 } } });
  assert.equal(t.onThatNotehead(), " on that notehead");

  // A note on its own is its own notehead; saying so would be noise.
  const l = load(["onThatNotehead"],
    { state: { selection: { part: 0, measure: 0, ref: 9, head: 1 } } });
  assert.equal(l.onThatNotehead(), "");
});

// The noteheads of a chord all carry the chord's ref, which is what makes
// clicking any of them find the chord. Telling them apart is a matter of
// where they are drawn.
test("the right notehead is found among the ones sharing a ref", () => {
  const z = 8;
  const glyphs = [
    { ref: 7, y: (6 + 4 - 0 / 2) * z },
    { ref: 7, y: (6 + 4 - 2 / 2) * z },
    { ref: 7, y: (6 + 4 - 4 / 2) * z },
  ].map((g, i) => ({
    i,
    getAttribute: (k) => (k === "y" ? String(g.y) : "0"),
  }));
  const svg = { querySelectorAll: () => glyphs };
  const t = load(["headGlyphAt"]);

  assert.equal(t.headGlyphAt(svg, 7, 6 + 4 - 2 / 2, z).i, 1);
  assert.equal(t.headGlyphAt(svg, 7, 6 + 4 - 4 / 2, z).i, 2);
  // Redrawn since it was picked up, and the head is no longer there.
  assert.equal(t.headGlyphAt(svg, 7, 30, z), null);
});

// The box round one notehead is drawn instead of the box round the chord,
// and everything else a selection carries still has to be drawn with it.
// The cursor is the one that was lost: the chord is picked up and the next
// note still goes after it.
test("the write cursor is drawn over a picked-up notehead too", () => {
  const fn = grab("markSelection");
  const branch = fn.slice(fn.indexOf("One notehead of a chord"));
  const end = branch.indexOf("What the rectangle has to enclose");
  assert.ok(end > 0, "markSelection no longer has the notehead branch");
  assert.match(branch.slice(0, end), /drawInsertCursor\(/,
    "the branch returns without drawing the cursor");
});
