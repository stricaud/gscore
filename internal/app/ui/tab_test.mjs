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

// stringAtCursor reads state, so it is given one.
function stringReader(lines) {
  const state = { info: { parts: [{ tab: { strings: new Array(lines).fill("E4") } }] } };
  const fn = new Function("state", grab("stringAtCursor") + "\nreturn stringAtCursor;")(state);
  return (step) => fn({ part: 0, step });
}

test("the bottom line of a six-string tab is the sixth string", () => {
  const at = stringReader(6);
  assert.equal(at(0), 6);
});

test("the top line is string one", () => {
  const at = stringReader(6);
  // Five lines above the bottom, two half spaces to a line.
  assert.equal(at(10), 1);
});

test("every line of the tab is a string, and in order", () => {
  const at = stringReader(6);
  assert.deepEqual([0, 2, 4, 6, 8, 10].map(at), [6, 5, 4, 3, 2, 1]);
});

test("a four-string tab counts to four", () => {
  const at = stringReader(4);
  assert.deepEqual([0, 2, 4, 6].map(at), [4, 3, 2, 1]);
});

test("off the staff is not a string", () => {
  const at = stringReader(6);
  assert.equal(at(-2), 0);
  assert.equal(at(12), 0);
});

test("a staff of pitches has no strings at all", () => {
  const state = { info: { parts: [{}] } };
  const fn = new Function("state", grab("stringAtCursor") + "\nreturn stringAtCursor;")(state);
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
