/*
 * barset_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Bars picked up one at a time.
 *
 * The band over a bar picks the bar up, and shift-click takes in every bar
 * between it and another. There was no way to take in a bar without the
 * ones between: Command-click did nothing at all, being a pan that went
 * nowhere. Reported as "I select a bar and with the meta key I can click on
 * other bars to select to copy and paste somewhere else".
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

// Five bars of two notes each on one staff, the second note of each in
// the second voice.
function freshState() {
  const hits = [];
  for (let measure = 0; measure < 5; measure++) {
    hits.push({
      part: 0, measure,
      elements: [
        { index: 0, ref: measure * 10 + 1, voice: 0 },
        { index: 0, ref: measure * 10 + 2, voice: 1 },
      ],
    });
    // A second staff under it, one note a bar.
    hits.push({ part: 1, measure, elements: [{ index: 0, ref: measure * 10 + 5, voice: 0 }] });
  }
  return { selection: null, multi: [], anchor: null, focus: null,
           barAnchor: null, hits };
}

// The editor's own functions, with what they lean on stubbed.
function editor(state) {
  const said = [];
  const body = [
    "selOf", "orderedSelection", "selectedElement", "selectedBars", "barRuns",
    "selectMeasure", "toggleMeasure", "clearSelection", "selectionTargets",
  ].map(grab).join("\n") +
    "\nreturn { selectMeasure, toggleMeasure, selectionTargets, selectedBars, barRuns };";
  const api = new Function("state", "markSelection", "status", "barNumberOf",
    "modKeyName", body)(
      state, () => {}, (m) => said.push(m), (n) => n + 1, () => "Ctrl");
  return { ...api, said };
}

test("a modified click adds a bar to the ones picked up", () => {
  const state = freshState();
  const ed = editor(state);
  ed.selectMeasure(0, 1);
  ed.toggleMeasure(0, 3);
  assert.deepEqual(ed.selectedBars(state.selection).map((b) => b.measure), [1, 3]);
  assert.equal(state.selection.bar, true);
  assert.equal(state.selection.measure, 1, "the set starts at its first bar");
  assert.equal(state.selection.barTo, 3);
  assert.match(ed.said[ed.said.length - 1], /Bars 2, 4/);
});

// The bar between is not picked up: that is the whole point of adding
// bars one at a time rather than shift-clicking.
test("copying the set skips the bars between", () => {
  const state = freshState();
  const ed = editor(state);
  ed.selectMeasure(0, 1);
  ed.toggleMeasure(0, 3);
  assert.equal(ed.selectionTargets(), "0:1:0,0:1:1/0,0:3:0,0:3:1/0");
});

// Every voice of the bar comes with it, which is what picking up a bar
// means and what a range of notes does not do.
test("the set holds every voice of each bar", () => {
  const state = freshState();
  const ed = editor(state);
  ed.toggleMeasure(0, 2);
  const picked = ed.selectionTargets().split(",");
  assert.equal(picked.length, 2);
  assert.ok(picked.some((t) => t.endsWith(":1/0")), "the second voice was left behind");
});

test("a modified click on a bar already picked up puts it down", () => {
  const state = freshState();
  const ed = editor(state);
  ed.selectMeasure(0, 1);
  ed.toggleMeasure(0, 3);
  ed.toggleMeasure(0, 4);
  ed.toggleMeasure(0, 3);
  assert.deepEqual(ed.selectedBars(state.selection).map((b) => b.measure), [1, 4]);
  ed.toggleMeasure(0, 1);
  ed.toggleMeasure(0, 4);
  assert.equal(state.selection, null, "putting the last bar down left a selection");
});

// With nothing picked up, or a note picked up, the modified click is the
// first bar of a new set rather than an addition to nothing.
test("a modified click with no bar in hand starts a set", () => {
  const state = freshState();
  const ed = editor(state);
  state.selection = { part: 0, measure: 2, at: 1, ref: 21 };
  ed.toggleMeasure(0, 4);
  assert.deepEqual(ed.selectedBars(state.selection), [{ part: 0, measure: 4 }]);
  assert.equal(state.barAnchor.measure, 4, "the new set is where shift-click grows from");
});

// A run is still a run: shift-click after a set replaces it with the bars
// between, as it did before.
test("a run picked up by shift-click reads as every bar between", () => {
  const state = freshState();
  const ed = editor(state);
  ed.selectMeasure(0, 1, 3);
  assert.deepEqual(ed.selectedBars(state.selection).map((b) => b.measure), [1, 2, 3]);
  assert.deepEqual(ed.barRuns(state.selection), [{ part: 0, from: 1, to: 3 }]);
});

// Emptying the bars asks the server a run at a time, so a set of 1, 2 and
// 4 is two requests and bar 3 is left alone.
test("a set is emptied a run at a time", () => {
  const state = freshState();
  const ed = editor(state);
  ed.selectMeasure(0, 1);
  ed.toggleMeasure(0, 2);
  ed.toggleMeasure(0, 4);
  assert.deepEqual(ed.barRuns(state.selection),
    [{ part: 0, from: 1, to: 2 }, { part: 0, from: 4, to: 4 }]);
  assert.match(grab("deleteSelected"), /barRuns\(sel\)/,
    "Backspace on a set of bars still empties from the first to the last");
});

// The click handler lets a modified click through to the bars, where it
// used to be dropped as a pan gesture.
test("a modified click reaches the bar under it", () => {
  const clicks = grab("setupCanvasClicks");
  assert.ok(!/if \(e\.ctrlKey \|\| e\.metaKey \|\| panned\) return;/.test(clicks),
    "a click with Command held is still thrown away");
  const at = clicks.indexOf("if (e.ctrlKey || e.metaKey) {");
  assert.ok(at > 0, "the click handler does not look for a modified click");
  const branch = clicks.slice(at, at + 700);
  assert.match(branch, /stripAt\(x, y\)/, "the band over the bar is not tried");
  assert.match(branch, /hitAt\(x, y\)/, "the bar itself is not tried");
  assert.match(branch, /toggleMeasure\(/);
});

// Everything that reads a bar selection reads it through selectedBars, so
// that none of them walks from the first bar to the last and takes in bars
// that were never picked up.
test("nothing walks a bar selection from measure to barTo", () => {
  for (const name of ["markSelection", "selectionTargets"]) {
    const fn = grab(name);
    assert.ok(!/for \(let i = sel\.measure; i <= \(sel\.barTo/.test(fn),
      name + " still walks every bar from the first to the last");
    assert.match(fn, /selectedBars\(sel\)/);
  }
});

// Standing in a bar with nothing picked up is being in that bar, which is
// what Copy already takes it as. The first modified click adds to it rather
// than throwing it away for the bar clicked.
test("the bar the cursor stands in is the first of the set", () => {
  const state = freshState();
  const ed = editor(state);
  state.selection = { part: 0, measure: 1, at: 2, step: 0, ref: null };
  ed.toggleMeasure(0, 2);
  assert.deepEqual(ed.selectedBars(state.selection).map((b) => b.measure), [1, 2]);
  assert.equal(state.barAnchor.measure, 1);
});

// The bass under the treble: a bar on the staff below joins the set, and
// the copy spells each bar on its own staff.
test("a modified click on the staff below adds its bar", () => {
  const state = freshState();
  const ed = editor(state);
  ed.selectMeasure(0, 2);
  ed.toggleMeasure(1, 2);
  ed.toggleMeasure(1, 3);
  ed.toggleMeasure(0, 3);
  assert.deepEqual(ed.selectedBars(state.selection), [
    { part: 0, measure: 2 }, { part: 0, measure: 3 },
    { part: 1, measure: 2 }, { part: 1, measure: 3 },
  ]);
  assert.equal(state.selection.part, 0, "a paste lands on the top staff of the set");
  assert.equal(state.selection.measure, 2);
  assert.equal(ed.selectionTargets(),
    "0:2:0,0:2:1/0,0:3:0,0:3:1/0,1:2:0,1:3:0");
  assert.deepEqual(ed.barRuns(state.selection),
    [{ part: 0, from: 2, to: 3 }, { part: 1, from: 2, to: 3 }]);
  assert.match(ed.said[ed.said.length - 1], /on 2 staves/);
});
