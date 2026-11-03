/*
 * tab_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Typing a tab.
 *
 * Two things here are easy to get wrong and invisible when they are. A
 * fret is typed a digit at a time and frets run past nine, so "1" then "2"
 * has to be the twelfth fret and not the first followed by the second. And
 * the line the cursor sits on has to be turned into a string number, which
 * is counted from the other end: the cursor measures up from the bottom
 * line and string one is at the top.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// The steps the engraver puts the strings at.
//
// internal/engrave/tab.go: tabStep(str) = staffHeight*2 - (str-1)*2, with
// staffHeight the 4 that stepY and stepAt both measure from. So string one
// is at step 8 — the top line — and each string below it is two steps
// lower, whatever the instrument. Written out here rather than taken from
// the formula, because the formula is the thing under test.
const STRING_STEPS = [8, 6, 4, 2, 0, -2];

// STAFF_HEIGHT read from app.js rather than written down again, so this
// cannot pass against a 4 the program has stopped using.
const STAFF_HEIGHT = Number(/const STAFF_HEIGHT = (\d+)/.exec(src)[1]);

// stringAtCursor reads state and the staff height, so it is given both.
function stringReader(lines) {
  const state = { info: { parts: [{ tab: { strings: new Array(lines).fill("E4") } }] } };
  const fn = new Function("state", "STAFF_HEIGHT",
    grab("stringAtCursor") + "\nreturn stringAtCursor;")(state, STAFF_HEIGHT);
  return (step) => fn({ part: 0, step });
}

test("app.js still measures from the staff height the engraver does", () => {
  assert.equal(STAFF_HEIGHT, 4);
});

// The cursor has to name the string whose line it is actually sitting on.
//
// It did not: the strings were counted up from the bottom, which put every
// one of them a line too high — a fret typed on the line the cursor was on
// was written on the line below it — and left the bottom string with no
// step that reached it at all.
test("the top line is string one", () => {
  const at = stringReader(6);
  assert.equal(at(STRING_STEPS[0]), 1);
});

test("the bottom line of a six-string tab is the sixth string", () => {
  const at = stringReader(6);
  assert.equal(at(STRING_STEPS[5]), 6);
});

test("every line of the tab is a string, and in order", () => {
  const at = stringReader(6);
  assert.deepEqual(STRING_STEPS.map(at), [1, 2, 3, 4, 5, 6]);
});

// A bass tab is the same lines from the top down, four of them. The sum
// does not mention how many strings there are, which is the whole of what
// was wrong with it before.
test("a four-string tab counts to four from the same top line", () => {
  const at = stringReader(4);
  assert.deepEqual(STRING_STEPS.slice(0, 4).map(at), [1, 2, 3, 4]);
});

test("a five-string bass reaches its fifth string", () => {
  const at = stringReader(5);
  assert.equal(at(STRING_STEPS[4]), 5);
});

test("off the staff is not a string", () => {
  const at = stringReader(6);
  // Above the top line, and below the bottom string of a six-string neck.
  assert.equal(at(10), 0);
  assert.equal(at(-4), 0);
});

// And a tab with fewer lines stops where its neck does rather than
// carrying on into the space under the staff.
test("a four-string tab has nothing on the lines a guitar would use", () => {
  const at = stringReader(4);
  assert.equal(at(0), 0);
  assert.equal(at(-2), 0);
});

test("a staff of pitches has no strings at all", () => {
  const state = { info: { parts: [{}] } };
  const fn = new Function("state", "STAFF_HEIGHT",
    grab("stringAtCursor") + "\nreturn stringAtCursor;")(state, STAFF_HEIGHT);
  assert.equal(fn({ part: 0, step: 0 }), 0);
});

/*
 * The two-digit fret.
 */

test("a second digit extends the first rather than starting again", () => {
  const body = grab("typeFret");
  assert.match(body, /fretTyping\.fret \* 10 \+ digit/);
  assert.match(body, /p\.set\("replace", "1"\)/);
});

test("extending only counts while the player is still typing", () => {
  const body = grab("typeFret");
  assert.match(body, /now - fretTyping\.at < fretTypingWindow/);
  // And only on the same note: moving the cursor ends the number.
  assert.match(body, /fretTyping\.part === sel\.part/);
  assert.match(body, /fretTyping\.measure === sel\.measure/);
  assert.match(body, /fretTyping\.string === str/);
});

test("a two-digit fret that does not exist starts again as one digit", () => {
  // "1" then "5" on a twelve-fret neck is a first fret the player has
  // finished with, not a fifteenth that does not exist.
  const body = grab("typeFret");
  assert.match(body, /if \(extending\) \{\s*fretTyping = null;\s*return typeFret\(digit\);/);
});

test("the fret is written with the duration the tool is holding", () => {
  const body = grab("typeFret");
  assert.match(body, /dur: state\.tool\.duration/);
  assert.match(body, /state\.tool\.dots/);
});

/*
 * The keys and the menu.
 */

test("digits type frets only on a tab staff", () => {
  // Scoped the way the drum letters are, so the digits go on meaning
  // what they mean everywhere else.
  assert.match(src, /isTabStaff\(state\.selection\.part\) && \/\^\[0-9\]\$\/\.test\(e\.key\)/);
});

test("a staff knows whether it is a tab", () => {
  const isTab = new Function("state",
    grab("isTabStaff") + "\nreturn isTabStaff;")(
      { info: { parts: [{ tab: { strings: [] } }, {}] } });
  assert.equal(isTab(0), true);
  assert.equal(isTab(1), false);
});

test("the tab entries are on the Staff menu and are not disabled", () => {
  for (const action of ["tab", "string-up", "string-down"]) {
    const m = html.match(new RegExp(`data-action="${action}"[^>]*>`));
    assert.ok(m, "no menu entry for " + action);
    assert.ok(!m[0].includes("disabled"), action + " is still disabled");
  }
});

test("the tuning dialog names the strings, since that is what tells them apart", () => {
  const body = grab("tabDialog");
  assert.match(body, /t\.strings\.join\(" "\)/);
  // And offers a way back to a staff of pitches.
  assert.match(body, /value: "off"/);
});

test("moving a note along the neck keeps what it sounds", () => {
  const body = grab("moveToString");
  assert.match(body, /api\/restring/);
  assert.match(body, /move: dir/);
  // It is about a note, so there has to be one.
  assert.match(body, /sel\.index == null/);
});

/*
 * The caret after a fret.
 *
 * It used to stay where it was, so the next digit landed on top of the
 * last one and the only way to write a second fret was to move the caret
 * by hand first.
 */

test("the fret goes in at the caret", () => {
  const body = grab("typeFret");
  assert.match(body, /p\.set\("at", String\(sel\.at\)\)/);
});

test("the caret moves past what was just written", () => {
  const body = grab("typeFret");
  assert.match(body, /sel\.at = d\.at \+ 1/);
  assert.match(body, /carryCaretOn\(sel\)/);
  assert.match(body, /scrollSelectionIntoView\(\)/);
});

test("a second digit still aims at the note it is correcting", () => {
  // "1" then "2" is the twelfth fret, and the caret has already moved
  // past that note — so the correction has its own index to aim at and
  // the caret is left alone.
  const body = grab("typeFret");
  assert.match(body, /if \(extending\) \{\s*p\.set\("at", String\(fretTyping\.index\)\)/);
  assert.match(body, /if \(!extending\) \{[\s\S]*?sel\.at = d\.at \+ 1/);
});

test("a fret leaves the note selected but not picked up", () => {
  // So the arrows move the caret from string to string rather than
  // dragging the note that was just written.
  const body = grab("typeFret");
  assert.match(body, /sel\.wrote = true/);
  assert.match(body, /state\.multi = \[\]/);
});

/*
 * Running out of room.
 */

test("the caret follows the fret into whichever bar it landed in", () => {
  const body = grab("typeFret");
  // A full bar passes the note to the next one, so the bar it went in at
  // is not always the bar it ended up in.
  assert.match(body, /d\.measure == null \? sel\.measure : d\.measure/);
  assert.match(body, /sel\.measure = landed/);
});

test("how full the bar is is read after the score comes back", () => {
  // carryCaretOn asks the bar whether it has room left. Called before the
  // refresh it read the bar as it was one note ago, so the caret stayed
  // in a bar that was already full.
  const body = grab("typeFret");
  const refresh = body.indexOf("await refreshScore()");
  const carry = body.indexOf("carryCaretOn(sel)");
  assert.ok(refresh > 0 && carry > 0, "typeFret no longer does both");
  assert.ok(carry > refresh,
    "carryCaretOn runs before the refresh, so it reads stale hits");
});

test("a second digit leaves the caret alone", () => {
  // It is correcting the note the caret has already moved past.
  const body = grab("typeFret");
  assert.match(body, /if \(!extending\) \{\s*sel\.measure = landed/);
});
