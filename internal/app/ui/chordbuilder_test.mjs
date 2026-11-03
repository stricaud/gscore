/*
 * chordbuilder_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The chord builder, and moving a tab part.
 *
 * A shape is written lowest-sounding string first — "x02210" is an A minor
 * — and a tab staff numbers its strings the other way about, string one
 * being the top line. The builder draws string one at the top, so it turns
 * the two round, and that turning-round is the thing most likely to be
 * wrong and least likely to show: a shape read backwards is still a chord,
 * just not the one that was clicked.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// The shape functions read and write one module-level variable, so they are
// given one and handed back.
function shapeScope(shape, typed) {
  const el = { value: typed ?? "" };
  const $ = () => el;
  const fn = new Function("$", "shapeStart",
    "let chordShape = shapeStart;" +
    grab("shapeText") + grab("showShapeText") + grab("readShapeText") +
    "\nreturn { shapeText, showShapeText, readShapeText," +
    " get shape() { return chordShape; }, el: $() };")($, shape);
  return fn;
}

/* ---------- writing the shape down ---------- */

test("a shape is written lowest string first", () => {
  // x on the bottom string, then 0 2 2 1 0 going up: an A minor.
  const s = shapeScope([null, 0, 2, 2, 1, 0]);
  assert.equal(s.shapeText(), "x02210");
});

test("an open string is a 0 and an unplayed one is an x", () => {
  const s = shapeScope([3, 2, 0, 0, 0, 3]);
  assert.equal(s.shapeText(), "320003");
  assert.equal(shapeScope([null, null, 0, 2, 3, 2]).shapeText(), "xx0232");
});

// "x0221000" cannot say a tenth fret, and a chord high on the neck is not
// unusual, so once anything runs past nine the fields are separated.
test("a fret past the ninth is written with separators", () => {
  const s = shapeScope([null, 0, 2, 2, 10, 0]);
  assert.equal(s.shapeText(), "x,0,2,2,10,0");
});

test("nine and under stays tight", () => {
  assert.equal(shapeScope([null, 0, 2, 2, 9, 0]).shapeText(), "x02290");
});

/* ---------- reading it back ---------- */

test("what is typed comes back as the same shape", () => {
  for (const [typed, want] of [
    ["x02210", [null, 0, 2, 2, 1, 0]],
    ["320003", [3, 2, 0, 0, 0, 3]],
    ["x,0,2,2,10,0", [null, 0, 2, 2, 10, 0]],
    ["x 0 2 2 10 0", [null, 0, 2, 2, 10, 0]],
    ["X02210", [null, 0, 2, 2, 1, 0]],
  ]) {
    const s = shapeScope([9, 9, 9, 9, 9, 9], typed);
    s.readShapeText(6);
    assert.deepEqual(s.shape, want, typed);
  }
});

// Somebody halfway through typing a shape has not made a mistake, so a
// half-written one leaves the grid alone rather than emptying it.
test("a shape that is not finished leaves the grid as it was", () => {
  for (const typed of ["x02", "", "x0221", "x0221q0"]) {
    const was = [null, 0, 2, 2, 1, 0];
    const s = shapeScope(was.slice(), typed);
    s.readShapeText(6);
    assert.deepEqual(s.shape, was, typed);
  }
});

test("the text and the grid round-trip", () => {
  for (const text of ["x02210", "320003", "xx0232", "x,0,2,2,10,0"]) {
    const s = shapeScope([0, 0, 0, 0, 0, 0], text);
    s.readShapeText(6);
    const back = shapeScope(s.shape);
    assert.equal(back.shapeText(), text);
  }
});

/* ---------- the grid ---------- */

// String one at the top, the way the tab staff under the dialog is drawn,
// and the shape is lowest-first — so the row and the place in the shape
// run opposite ways. Getting this backwards draws every chord upside down.
test("the top row of the grid is the last field of the shape", () => {
  const body = grab("drawFretboard");
  assert.match(body, /const str = row \+ 1;/);
  assert.match(body, /const idx = lines - str;/);
});

test("clicking what is already down takes it off again", () => {
  const body = grab("drawFretboard");
  assert.match(body, /chordShape\[idx\] = \(was === f\) \? null : f/);
});

test("the grid and the written shape follow each other", () => {
  const body = grab("chordBuilderDialog");
  assert.match(body, /readShapeText\(/);
  assert.match(body, /drawFretboard\(/);
});

/* ---------- what it talks to ---------- */

test("the builder previews rather than writing on every click", () => {
  const body = grab("nameShape");
  assert.match(body, /api\/fretchord/);
  assert.match(body, /preview: "1"/);
});

test("writing it puts the caret past the chord, in the bar it landed in", () => {
  const body = grab("writeFretChord");
  assert.match(body, /sel\.measure = d\.measure/);
  assert.match(body, /sel\.at = \(d\.at \?\? 0\) \+ 1/);
  assert.match(body, /dur: state\.tool\.duration/);
});

/* ---------- moving a tab part ---------- */

// The two things "put it up a tone" can mean, which are different edits.
test("the dialog offers both kinds of move", () => {
  assert.match(html, /name="tabmove-how" value="music"/);
  assert.match(html, /name="tabmove-how" value="tuning"/);
});

test("retuning goes to the retune endpoint and transposing to the other", () => {
  const body = grab("tabMoveDialog");
  assert.match(body, /api\/retune/);
  assert.match(body, /transposeBy\(/);
});

// An interval is two numbers and not one, and the second is filled in
// where the move is made rather than here: a caller that gives only the
// distance gets the ordinary reading of it. Sending a steps of zero would
// not do — zero letters and one semitone is an augmented unison, a real
// interval — so it is left out of the request altogether.
test("moving the music sends the distance and lets the server spell it", () => {
  const body = grab("tabMoveDialog");
  assert.match(body, /transposeBy\(null, semitones/);
  const send = grab("transposeBy");
  assert.match(send, /if \(steps !== null && steps !== undefined\) p\.set\("steps"/);
  assert.doesNotMatch(send, /steps: String\(steps\)/,
    "a steps of null would be sent as the word null");
});

/* ---------- and it is reachable ---------- */

test("both menu entries are live", () => {
  for (const action of ["chord-builder", "tab-transposition"]) {
    const m = html.match(new RegExp(`data-action="${action}"[^>]*>`));
    assert.ok(m, "no menu entry for " + action);
    assert.ok(!m[0].includes("disabled"), action + " is still disabled");
  }
});

test("both are wired to a handler", () => {
  assert.match(src, /case "chord-builder": await chordBuilderDialog\(\)/);
  assert.match(src, /case "tab-transposition": await tabMoveDialog\(\)/);
});

// Either half of a pair will do: the cursor is usually on the staff with
// the notes on it, and the tuning is on the other one.
test("the tuning is found from either half of a pair", () => {
  const parts = [{ name: "Guitar" }, { name: "Tab", tab: {}, mirrorOf: 0 }];
  const fn = new Function("state",
    grab("tabWithTuning") + "\nreturn tabWithTuning;")({ info: { parts } });
  assert.equal(fn(0), 1, "asked of the notation half");
  assert.equal(fn(1), 1, "asked of the tab half");
  const plain = new Function("state",
    grab("tabWithTuning") + "\nreturn tabWithTuning;")({ info: { parts: [{}] } });
  assert.equal(plain(0), -1, "a staff of pitches has no tuning");
});

// mirrorOf has to actually arrive, or the pair can never be joined up.
test("the editor is told which staff mirrors which", () => {
  const api = readFileSync(
    new URL("../../../docs/openapi.yaml", import.meta.url), "utf8");
  assert.match(api, /mirrorOf:/);
});

/* ---------- it is styled ---------- */

// The grid is built out of classes that exist nowhere else, so a missing
// rule is a row of unstyled buttons rather than a neck.
test("every class the fretboard uses has a rule", () => {
  for (const cls of ["fb-row", "fb-name", "fb-open", "fb-mute", "fb-cell",
                     "fb-feet"]) {
    assert.match(css, new RegExp("\\." + cls + "[\\s,.:{]"),
      `.${cls} is used in app.js but has no rule in style.css`);
  }
  for (const id of ["fretboard-grid", "fretboard-bar", "fretboard-name"]) {
    assert.match(css, new RegExp("#" + id + "[\\s,.:{]"),
      `#${id} has no rule in style.css`);
  }
});

test("the dialogs are in French too", () => {
  for (const s of ["Chord builder…", "Tab transposition…"]) {
    assert.ok(fr.includes(`"${s}":`), `"${s}" has no French`);
  }
});
