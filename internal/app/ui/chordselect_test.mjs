/*
 * chordselect_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Picking up a chord symbol.
 *
 * A chord is written on a line of its own above the staff, nowhere near the
 * note it belongs to, so a click on it used to find the empty air above the
 * music — and with a note tool in hand, write a note there. The only way to
 * change a chord was to select the bar under it and know that was how.
 *
 * Clicking it picks up the symbol itself: Enter opens it for changing with
 * what is there already in the box, Backspace takes the chord away and
 * leaves the music alone.
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

// Two chords over one bar, as a chart is written: "F7" over the first half
// and "Bb7" over the second.
const CHORDS = [
  { part: 0, measure: 0, ref: 11, x: 10, y: 2, width: 3, height: 2.5, name: "F7" },
  { part: 0, measure: 0, ref: 14, x: 20, y: 2, width: 3.6, height: 2.5, name: "Bb7" },
];
const HITS = [{
  part: 0, measure: 0, staffTop: 6,
  elements: [
    { ref: 11, index: 0 }, { ref: 12, index: 1 },
    { ref: 14, index: 2 }, { ref: 15, index: 3 },
  ],
}];

function load() {
  const said = [];
  const state = { chordMarks: CHORDS, hits: HITS, multi: [], selection: null };
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection() {},
  };
  const body = grab("chordSymbolAt") + "\n" + grab("refTarget") + "\n" +
    grab("selectChordSymbol") + "\nreturn { chordSymbolAt, selectChordSymbol };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, state, said };
}

test("a click on a chord symbol finds it", () => {
  const t = load();
  assert.equal(t.chordSymbolAt(11, 3)?.name, "F7");
  assert.equal(t.chordSymbolAt(21, 3)?.name, "Bb7");
});

test("a click beside one finds nothing, so the music still answers", () => {
  const t = load();
  assert.equal(t.chordSymbolAt(16, 3), null, "the gap between two chords");
  assert.equal(t.chordSymbolAt(11, 8), null, "well below them, on the staff");
});

test("picking one up selects the symbol, not the note under it", () => {
  const t = load();
  t.selectChordSymbol(CHORDS[0]);

  assert.equal(t.state.selection.chordSymbol, "F7");
  assert.equal(t.state.selection.ref, 11);
  assert.ok(t.said.some((s) => s.includes("Enter") && s.includes("Backspace")),
            "the person was not told what they can do with it: " + t.said.join(" | "));
});

test("the second chord in a bar is the one that is picked up", () => {
  // Both live in bar one. Addressed by the bar alone, changing the second
  // would rewrite the first.
  const t = load();
  t.selectChordSymbol(CHORDS[1]);

  assert.equal(t.state.selection.chordSymbol, "Bb7");
  // at is one-based and counts elements in the bar: the third one here.
  assert.equal(t.state.selection.at, 3,
               "the selection points at element " + t.state.selection.at);
});

test("picking up a chord drops whatever else was selected", () => {
  const t = load();
  t.state.multi = [{ part: 0, measure: 0, index: 0 }];
  t.selectChordSymbol(CHORDS[0]);
  assert.deepEqual(t.state.multi, [], "a multiple selection survived");
});

test("Enter changes the chord that is picked up, rather than writing a rest", () => {
  // The binding is a one-liner in the key table, so it is read from there.
  const binding = src.match(/id: "write-rest"[\s\S]*?\},\n/);
  assert.ok(binding, "the Enter binding is gone");
  assert.ok(/chordSymbol[\s\S]*chordSymbolDialog/.test(binding[0]),
            "Enter still writes a rest under a chord somebody just pointed at");
});
