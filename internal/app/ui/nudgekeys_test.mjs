/*
 * nudgekeys_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the up and down arrows do.
 *
 * With nothing picked up they move the cursor, which is where the next note
 * will be written. With music picked up they move the music. The question
 * they asked was whether a *dragged* selection existed, so a note picked up
 * by clicking it fell through to the cursor — and moving the cursor lets go
 * of what is picked up, so the arrow appeared to do nothing and dropped the
 * selection on its way.
 *
 * A marking is not music. Every one of them names the note it was written
 * on so that Backspace can find it, and an arrow must not take that as an
 * invitation to move the note.
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

function asking(state) {
  const scope = { state };
  const body = grab("pickedNotes") + "\nreturn { pickedNotes };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope)).pickedNotes();
}

test("a note picked up by clicking it is music", () => {
  assert.equal(asking({ multi: [], selection: { part: 0, measure: 0, at: 1, ref: 7 } }),
               true);
});

test("a dragged run is music", () => {
  assert.equal(asking({
    multi: [{ part: 0, measure: 0, index: 0 }, { part: 0, measure: 0, index: 1 }],
    selection: { part: 0, measure: 0, at: 1, ref: 7 },
  }), true);
});

test("a place in a bar with nothing picked up is not", () => {
  assert.equal(asking({ multi: [], selection: { part: 0, measure: 0, at: 1, ref: null } }),
               false);
  assert.equal(asking({ multi: [], selection: null }), false);
});

// Each of these names the note it was written on. Moving that note is not
// what clicking the marking asked for.
test("a marking is not music", () => {
  const marks = {
    arc: { kind: "slur" },
    hairpin: { kind: "crescendo" },
    tremolo: { from: 7 },
    dynamic: "ff",
    chordSymbol: "C7",
    tempo: 3,
  };
  for (const [name, value] of Object.entries(marks)) {
    assert.equal(
      asking({ multi: [], selection: { part: 0, measure: 0, ref: 7, [name]: value } }),
      false, `a picked-up ${name} was taken for music`);
  }
  // The tempo of the first bar is bar 0, which must not read as "no tempo".
  assert.equal(asking({ multi: [], selection: { part: 0, measure: 0, ref: 7, tempo: 0 } }),
               false, "the tempo at the head of the piece");
});

test("a whole bar picked up is not a note to move", () => {
  assert.equal(asking({
    multi: [], selection: { part: 0, measure: 2, ref: null, bar: true },
  }), false);
});

// The bindings themselves, so that the question they ask stays this one.
test("the arrows ask whether music is picked up, not whether a drag happened", () => {
  for (const id of ["cursor-up", "cursor-down", "cursor-up-octave", "cursor-down-octave"]) {
    const m = src.match(new RegExp(`\\{ id: "${id}",[\\s\\S]*?\\n  \\},?\\n`));
    assert.ok(m, "no binding for " + id);
    assert.match(m[0], /pickedNotes\(\)/, id + " asks the wrong question");
    assert.ok(!/state\.multi\.length/.test(m[0]),
              id + " still asks whether a run was dragged");
  }
});
