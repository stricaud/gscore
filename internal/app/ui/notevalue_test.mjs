/*
 * notevalue_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Choosing a note value while typing.
 *
 * A note value clicked with four sixteenths picked up means "those four
 * are wrong" and has to retime them. The same click a moment after
 * writing a note means "the next one is shorter" — somebody looking at a
 * bar with a beat left in it, choosing what to fill it with — and
 * retiming what they just wrote is the opposite of what they asked for.
 *
 * Notes stay selected after they are written on purpose, so that a
 * dynamic or an accidental still lands on them. Selected is not picked
 * up, and this is where the two part company.
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

// held builds heldForChanging over a given state.
function held(state) {
  const body = grab("hasSelection") + "\n" + grab("heldForChanging") +
    "\nreturn heldForChanging;";
  return new Function("state", body)(state);
}

const aNote = { part: 0, measure: 0, at: 1, ref: 7 };

test("a note just written is not picked up", () => {
  const f = held({ multi: [], selection: { ...aNote, wrote: true } });
  assert.equal(f(), false);
});

test("a note pointed at is picked up", () => {
  const f = held({ multi: [], selection: { ...aNote } });
  assert.equal(f(), true);
});

test("a range dragged out is picked up, written or not", () => {
  const f = held({ multi: [{ ref: 1 }, { ref: 2 }], selection: { ...aNote, wrote: true } });
  assert.equal(f(), true);
});

test("nothing selected is not picked up", () => {
  assert.equal(held({ multi: [], selection: null })(), false);
  assert.equal(held({ multi: [], selection: { part: 0, measure: 0, at: 1 } })(), false);
});

/*
 * What the palette does with it.
 */

test("a note value retimes only what was picked up", () => {
  const body = grab("paletteAction");
  // Every place a value retimes something asks the new question first:
  // the note-value branch and the rest branch both.
  const guards = [...body.matchAll(/if \(([\s\S]*?)\) \{\s*await setSelectionDuration/g)]
    .map((m) => m[1]);
  assert.equal(guards.length, 2, "expected the note and rest branches");
  for (const cond of guards) {
    assert.ok(cond.includes("heldForChanging"),
      `a value still retimes on "${cond}"`);
  }
  assert.ok(!/hasSelection\(\)[^;]*\{\s*await setSelectionDuration/.test(body),
    "a value still retimes a note that was merely written");
});

// An accidental is the opposite case and must not change: hitting space
// and then clicking a flat is the thing that already works.
test("an accidental still lands on the note just written", () => {
  const body = grab("paletteAction");
  assert.match(body, /if \(hasSelection\(\)\) \{\s*await setAccidental/);
});

test("the value is still taken up as the pen either way", () => {
  // Whether or not it retimed anything, the click chooses what comes
  // next: that is what makes it usable for the bar you are filling.
  const body = grab("paletteAction");
  assert.match(body, /state\.tool\.duration = item\.tool/);
  assert.match(body, /state\.tool\.rest = true;\s*if \(item\.dur\) state\.tool\.duration = item\.dur/);
});

/*
 * And the arrow, which is the gesture that says "I have finished with
 * that one".
 */

test("moving the cursor leaves nothing written behind it", () => {
  // moveCursorBy builds the selection afresh, so the note it lands on is
  // somewhere to write and not something just written.
  const body = grab("moveCursorBy");
  assert.match(body, /state\.selection = \{/);
  assert.ok(!body.includes("wrote"),
    "moving the cursor carries the written flag along with it");
  assert.match(body, /replace: false/);
});

test("the two questions are asked the same way", () => {
  // heldForMoving and heldForChanging draw the same line for the same
  // reason, and should go on agreeing about what "picked up" means.
  for (const name of ["heldForMoving", "heldForChanging"]) {
    const body = grab(name);
    assert.match(body, /if \(state\.multi\.length\) return true/);
    assert.match(body, /state\.selection\.wrote\) return false/);
  }
});

/*
 * And the arrows after writing.
 *
 * The chain that made the up and down arrows move the note behind the
 * cursor: writing set "selected but not picked up", clicking a note value
 * then rebuilt the selection without it and left a range standing, and
 * heldForMoving asks about a range before it asks about the flag. So the
 * arrow aimed at the next note transposed the last one.
 */

test("writing a note lets go of whatever was picked up", () => {
  const body = grab("insertAtSelection");
  assert.match(body, /state\.multi = \[\]/);
  assert.match(body, /state\.anchor = null/);
  assert.match(body, /state\.focus = null/);
  // And it still says the note is selected, so a dynamic lands on it.
  assert.match(body, /sel\.wrote = true/);
});

test("a range still beats the written flag, which is why writing clears it", () => {
  // heldForMoving asks about a range first on purpose — twenty notes
  // picked up are picked up. That is only safe because writing no longer
  // leaves one behind.
  const body = grab("heldForMoving");
  const range = body.indexOf("state.multi.length");
  const flag = body.indexOf("state.selection.wrote");
  assert.ok(range >= 0 && flag >= 0 && range < flag);
});

test("after writing, the arrows aim the next note", () => {
  const moving = new Function("state",
    grab("pickedNotes") + "\n" + grab("heldForMoving") + "\nreturn heldForMoving;");
  // What insertAtSelection leaves behind: nothing picked up, and the
  // note it wrote still selected.
  const after = { multi: [], selection: { part: 0, measure: 0, at: 2, ref: 9, wrote: true } };
  assert.equal(moving(after)(), false, "the arrow would move the note instead");
});
