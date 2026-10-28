/*
 * movement_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which bar is bar four.
 *
 * Bars are numbered from one in every movement, which is how a movement is
 * read and how it is cited. So "go to bar four" means the fourth bar of the
 * movement you are standing in — the number printed over the bar you are
 * looking at. Counted from the first bar of the whole sonata it took
 * somebody working on the Presto back to the Adagio, two movements away.
 *
 * Everything the editor holds — the clickable regions, the performance, the
 * cursor — counts bars from the beginning of the file. Only what is written
 * on the page and typed into the box is counted from the movement, and the
 * two must not be confused: that is what these are about.
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

// The Moonlight sonata's shape: three movements of a score 335 bars long.
const sonata = [
  { first: 0, last: 68, title: "Movement I" },
  { first: 69, last: 132, title: "Movement II" },
  { first: 133, last: 334, title: "Movement III" },
];

function editor(movements, measure) {
  const state = { info: { movements }, selection: measure === null ? null : { measure } };
  const names = ["movementAt", "movementHere", "barNumberOf", "barIndexOf"];
  const scope = { state };
  const body = names.map(grab).join("\n") + "\nreturn { " + names.join(", ") + " };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
}

test("a bar's number is its place in its own movement", () => {
  const t = editor(sonata, 0);
  assert.equal(t.barNumberOf(0), 1, "the first bar of the piece");
  assert.equal(t.barNumberOf(68), 69, "the last bar of the first movement");
  assert.equal(t.barNumberOf(69), 1, "the first bar of the second");
  assert.equal(t.barNumberOf(133), 1, "the first bar of the third");
  assert.equal(t.barNumberOf(136), 4,
    "bar 137 of the file is the Presto's fourth, which is what is printed over it");
});

test("a number typed is a bar of the movement it was typed in", () => {
  const t = editor(sonata, 136); // standing in the Presto
  const here = t.movementHere();
  assert.equal(here.title, "Movement III", "the cursor is in the wrong movement");
  assert.equal(t.barIndexOf(4, here), 136,
    "going to bar 4 from the Presto landed somewhere else");
  // And the two are inverses, wherever you stand.
  for (const bar of [0, 1, 68, 69, 132, 133, 200, 334]) {
    const m = t.movementAt(bar);
    assert.equal(t.barIndexOf(t.barNumberOf(bar), m), bar,
      "bar " + bar + " does not survive the round trip");
  }
});

test("a score with one movement counts straight through", () => {
  // Which is every ordinary score, and where none of this applies.
  for (const movements of [undefined, [], [{ first: 0, last: 40 }]]) {
    const t = editor(movements, 0);
    assert.equal(t.movementAt(7), null, "a score of one movement claimed to have several");
    assert.equal(t.barNumberOf(7), 8, "bar 8 is bar 8");
    assert.equal(t.barIndexOf(8, null), 7);
  }
});

test("a bar past the end still belongs to the last movement", () => {
  // The editor draws one empty bar past the music to write into, and it is
  // in the movement the music ended in.
  const t = editor(sonata, 0);
  assert.equal(t.movementAt(335).title, "Movement III");
  assert.equal(t.movementAt(9999).title, "Movement III");
});

test("the jump asks and answers in the movement it is standing in", () => {
  const fn = grab("jumpToMeasure");
  assert.ok(fn.includes("movementHere()"),
    "the jump counts from the first bar of the file, not of the movement");
  assert.ok(fn.includes("barIndexOf("),
    "what was typed is taken as a bar of the whole score");
  assert.ok(/where\.last - where\.first \+ 1/.test(fn),
    "the box offers the whole score's length rather than the movement's");
  assert.ok(fn.includes("where.title"),
    "nothing says which movement the number will be counted in");
});

test("nothing says a bar number without saying the movement's", () => {
  // A status line that calls the Presto's fourth bar 137 contradicts the
  // 4 printed over it.
  const bad = [];
  const re = /`[^`]*[Mm]easures?[^`]*\$\{[^}]*\.?measure \+ [12]\}/g;
  let m;
  while ((m = re.exec(src))) bad.push(m[0]);
  assert.deepEqual(bad, [],
    "these still print a bar's place in the file where the page prints its " +
    "place in its movement: " + bad.join(" | "));
});

/*
 * The arrow beside a staff.
 *
 * It says "carry on at the end of this staff", and a score with movements
 * draws the same staff several times over, one band under another. Reading
 * the furthest bar the part reaches anywhere on the page, the arrow beside
 * the first movement of the Moonlight sonata went to the last bar of the
 * third — a different piece of music, two movements away, where nothing the
 * player had been looking at was on the screen any more.
 */
function sprinter(hits, movements) {
  const state = {
    hits,
    info: { movements, parts: [{ name: "Piano" }] },
    selection: null,
  };
  const said = [];
  const scope = {
    state,
    Math,
    status: (s) => said.push(s),
    markSelection: () => {},
    staffHeightOf: () => 8,
    glideTo: async () => {},
    ensureTiles: async () => {},
    flashMeasure: () => {},
  };
  const names = ["sprintToStaffEnd", "movementAt", "barNumberOf"];
  const body = names.map((n) => grab(n)).join("\n") +
    "\nreturn { sprintToStaffEnd };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { state, said, run: api.sprintToStaffEnd };
}

// One part drawn twice: the first movement at the top of the page, the
// third further down.
const bands = [];
for (let i = 0; i <= 68; i++) {
  bands.push({ part: 0, measure: i, staffTop: 10, x0: i, x1: i + 1, elements: [1] });
}
for (let i = 133; i <= 334; i++) {
  bands.push({ part: 0, measure: i, staffTop: 300, x0: i, x1: i + 1, elements: [1, 2] });
}

test("the arrow runs to the end of the band it stands beside", async () => {
  const t = sprinter(bands, sonata);
  await t.run(0, 10);
  assert.equal(t.state.selection.measure, 68,
    "it left the first movement for the end of the third");
  assert.equal(t.state.selection.at, 1, "the cursor is not after the last note written");
  assert.match(t.said.join(" "), /measure 69 of Movement I/,
    "it does not say where it arrived, in the numbers over those bars");
});

test("the arrow beside a lower band runs to the end of that one", async () => {
  const t = sprinter(bands, sonata);
  await t.run(0, 300);
  assert.equal(t.state.selection.measure, 334, "the third movement does not end where it ends");
  assert.match(t.said.join(" "), /measure 202 of Movement III/,
    "the bar is not numbered from its own movement");
});

test("without a band to stand in, the arrow still finds the staff's end", async () => {
  const t = sprinter(bands, []);
  await t.run(0);
  assert.equal(t.state.selection.measure, 334,
    "a score of one movement lost the end of its staff");
});
