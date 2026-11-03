/*
 * joinstaves_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which staves a brace or a bracket covers.
 *
 * A bracket gathers a family — the strings, the woodwind, a rhythm section
 * — so the run it covers is a question. Selecting across the staves answers
 * it; where nothing is selected it is asked, because taking every staff in
 * the score was right about as often as it was wrong.
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

const staffRange = new Function(grab("staffRange") + "\nreturn staffRange;")();

test("a range is read however somebody writes it", () => {
  for (const raw of ["2-4", "2–4", "2—4", "2,4", "2, 4", "2 4", " 2 - 4 "]) {
    assert.deepEqual(staffRange(raw, 6), { first: 1, last: 3 }, raw);
  }
});

test("a range given backwards is still the staves between", () => {
  assert.deepEqual(staffRange("4-2", 6), { first: 1, last: 3 });
});

test("staves are counted from one, the way the page numbers them", () => {
  assert.deepEqual(staffRange("1-2", 2), { first: 0, last: 1 });
});

test("one staff is not a group", () => {
  assert.equal(staffRange("3", 6), null);
  assert.equal(staffRange("3-3", 6), null);
});

test("a staff that is not there is refused", () => {
  assert.equal(staffRange("1-9", 6), null);
  assert.equal(staffRange("0-2", 6), null);
  assert.equal(staffRange("-1-2", 6), null);
});

test("nonsense is refused rather than guessed at", () => {
  for (const raw of ["", "   ", "strings", "2-x", "two to four"]) {
    assert.equal(staffRange(raw, 6), null, raw);
  }
});

test("more than two numbers takes the span they cover", () => {
  // "2,3,5" is the staves 2 to 5: a group is a run, and the run that
  // holds everything named is the only reading that is not a refusal.
  assert.deepEqual(staffRange("2,3,5", 6), { first: 1, last: 4 });
});

/*
 * And the menu action around it.
 */

test("joining staves asks which ones when nothing is selected", () => {
  const body = grab("joinStaves");
  assert.match(body, /state\.multi\.length/);
  assert.match(body, /await ask\(/);
  assert.match(body, /staffRange\(answer, parts\.length\)/);
});

test("taking a group away asks nothing", () => {
  // It is about a group that is already there, so there is nothing to ask.
  const body = grab("joinStaves");
  assert.match(body, /symbol !== "none"/);
});

test("a brace runs the barlines through the gap and a bracket does not", () => {
  const body = grab("joinStaves");
  assert.match(body, /if \(symbol === "brace"\) p\.set\("barlines", "1"\)/);
});

test("the bracket is offered for a family, not only for a piano", () => {
  const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
  assert.match(html, /data-action="group-bracket"/);
  assert.match(html, /Bracket — a family/);
});
