/*
 * symbolguide_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the guides say in words.
 *
 * Two questions somebody placing a second copy of a symbol asks and cannot
 * answer by looking: is it the same size as the one over there, and is it in
 * line with it. The dashed lines show it; this is the same thing said, for
 * anybody who would rather read it than count lines on the page.
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

const words = new Function(
  grab("listOf") + "\n" + grab("guideWords") + "\nreturn guideWords;")();
const listOf = new Function(grab("listOf") + "\nreturn listOf;")();
const label = new Function(grab("symbolLabel") + "\nreturn symbolLabel;")();

test("nothing in common is nothing said", () => {
  assert.equal(words(0, []), "");
});

test("one of the same size", () => {
  assert.equal(words(1, []), "same size as one other");
});

test("several of the same size", () => {
  assert.equal(words(3, []), "same size as 3 others");
});

test("size and alignment together", () => {
  assert.equal(words(1, ["tops"]),
    "same size as one other, in line by tops");
});

// A list a program wrote reads "tops and bottoms and middles". English puts
// commas between and keeps the "and" for the last.
test("a list is read as English reads one", () => {
  assert.equal(listOf(["tops"]), "tops");
  assert.equal(listOf(["tops", "bottoms"]), "tops and bottoms");
  assert.equal(listOf(["tops", "bottoms", "middles"]),
    "tops, bottoms and middles");
});

// A pasted symbol is named by a UUID. Thirty-six characters of hexadecimal
// in the middle of a sentence is not a name anybody reads.
test("a UUID is shortened to something readable", () => {
  assert.equal(label("a7cc3160-f599-4405-8e0a-c64581720805"), "a7cc3160…");
});

test("a name somebody chose is left alone", () => {
  assert.equal(label("segno-with-a-flourish"), "segno-with-a-flourish");
  assert.equal(label("device"), "device");
});

// The guides are drawn against the other symbols on the page and the words
// are built from the same count, so the drawing and the sentence cannot
// disagree about what was found.
test("the guides and the words come from one place", () => {
  const draw = grab("drawSymbolGuides");
  assert.ok(draw.includes("symbolGuideText = guideWords("),
    "drawSymbolGuides no longer says what it drew");
  assert.ok(draw.includes("symbol-twin"), "nothing marks a matching size");
  assert.ok(draw.includes("symbol-guide"), "nothing draws an alignment line");
});

// Shown, not enforced. Nothing here may move a symbol: the guides say what
// is true and the arrow keys are what changes it.
test("the guides move nothing", () => {
  const draw = grab("drawSymbolGuides");
  for (const moving of ["/api/movesymbol", "/api/symbolscale", "fetch("]) {
    assert.ok(!draw.includes(moving),
      "drawSymbolGuides changes the score: " + moving);
  }
});
