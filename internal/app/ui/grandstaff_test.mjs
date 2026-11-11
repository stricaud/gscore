/*
 * grandstaff_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which staves Score setup shows as one instrument.
 *
 * Two staves of piano in a row are one pianist's hands. Ten in a row are
 * not: they are a score whose staves have not been told what they are, and
 * pairing them off down the page turns twenty-three instruments into eleven
 * two-handed pianists — after which the only way to set one staff is to
 * break a pair the score never had, and breaking it loses what the pair
 * said it was.
 *
 * Reported as "I want to set the flute only for staff 2, but I see 2+3; I
 * ungroup to get the third one and that changes 2+3 to piano".
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

function grab(name) {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
}

function runOf(parts, idx) {
  const make = new Function(grab("sameSoundRun") + "\nreturn sameSoundRun;");
  return make()(parts, idx);
}

const piano = { sound: "keyboard.piano" };

test("a pianist's two hands are a run of two", () => {
  assert.equal(runOf([piano, piano], 0), 2);
  assert.equal(runOf([piano, piano], 1), 2);
});

test("a page of unnamed staves is a run of however many there are", () => {
  // Asked of the second of ten, it still answers ten: the run and not the
  // pair is what says they are not a grand staff.
  const ten = Array.from({ length: 10 }, () => piano);
  assert.equal(runOf(ten, 1), 10);
  assert.equal(runOf(ten, 9), 10);
});

test("a run stops at a staff playing something else", () => {
  const parts = [piano, piano, { sound: "strings.violin" }, piano];
  assert.equal(runOf(parts, 0), 2);
  assert.equal(runOf(parts, 2), 1);
  assert.equal(runOf(parts, 3), 1);
});

test("a staff that has not been told what it is joins nothing", () => {
  // No sound at all is not the same answer as "the same as the one above":
  // a score of staves that say nothing must not be paired off.
  assert.equal(runOf([{ sound: "" }, { sound: "" }], 0), 0);
  assert.equal(runOf([{}, {}], 1), 0);
});

// Taking a pair apart.
//
// An entry reading "2+3" is two staves on one line, and the only way to
// give the third staff an instrument of its own is to get it onto a line
// of its own first. Ungroup is that word, and it was only taking brackets
// away: on a pair the score never bracketed it did nothing visible at all.
function apart(parts, groups, pick) {
  const scope = {
    state: { info: { parts, groups: groups || [] } },
    setupNextId: 1,
    setupApart: new Set(),
    setupGroups: [],
    setupChosen: [],
    instrumentCatalogue: [
      { sound: "keyboard.piano", name: "piano", grand: true },
    ],
    grandSound: (s) => s === "keyboard.piano",
    setupLive: () => false,
    setupOrdered: () => scope.setupChosen,
    drawSetupChosen: () => {},
    status: () => {},
    setupPicked: () => pick.map((i) => scope.setupChosen[i]),
  };
  const body = grab("setupFromScore") + "\n" + grab("sameSoundRun") + "\n" +
    grab("setupUngroup") +
    "\nreturn { setupFromScore, setupUngroup };";
  const made = new Function("scope", "with (scope) { " + body + " }")(scope);
  scope.setupChosen = made.setupFromScore();
  const before = scope.setupChosen.map((it) => it.staves);
  made.setupUngroup();
  return { before, after: scope.setupChosen.map((it) => it.staves), scope };
}

test("ungrouping a pair puts each staff on a line of its own", () => {
  const two = [{ sound: "keyboard.piano" }, { sound: "keyboard.piano" }];
  const got = apart(two, [], [0]);
  assert.deepEqual(got.before, [[0, 1]], "the pair was not read as one entry");
  assert.deepEqual(got.after, [[0], [1]]);
  // Each line is still a piano: taking the pair apart says nothing about
  // what either staff plays.
  assert.deepEqual(got.scope.setupChosen.map((it) => it.sound),
    ["keyboard.piano", "keyboard.piano"]);
  // And the two lines are two entries, not the same one listed twice, or
  // setting one would set both.
  assert.notEqual(got.scope.setupChosen[0].id, got.scope.setupChosen[1].id);
});

test("a braced pair comes apart too, and stays apart when read again", () => {
  // The brace is the score speaking, so taking it away is a change to the
  // score; but the reading that follows must not put the pair back.
  const got = apart(
    [{ sound: "strings.cello" }, { sound: "strings.cello" }],
    [{ first: 0, last: 1, symbol: "brace" }], [0]);
  assert.deepEqual(got.before, [[0, 1]]);
  assert.deepEqual(got.after, [[0], [1]]);
  assert.deepEqual(
    new Function("scope", "with (scope) { " + grab("setupFromScore") +
      "\n" + grab("sameSoundRun") + "\nreturn setupFromScore(); }")(got.scope)
      .map((it) => it.staves),
    [[0], [1]], "the pair was read back together");
});

test("ungrouping a staff that is already alone leaves the list as it was", () => {
  const got = apart(
    [{ sound: "wind.flutes.flute" }, { sound: "wind.flutes.flute" }], [], [1]);
  assert.deepEqual(got.before, [[0], [1]]);
  assert.deepEqual(got.after, [[0], [1]]);
});

test("the pairing asks for a run of exactly two", () => {
  const body = grab("setupFromScore");
  assert.match(body, /run === 2/,
    "staves are still paired off however many of them play the same thing");
  // And a brace still joins whatever the score says it joins: that is the
  // score speaking, not a guess.
  assert.match(body, /sameBrace \|\| looksGrand/);
});
