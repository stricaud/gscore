/*
 * drawsymbol_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Saving a drawing, and moving the mark once it is in.
 *
 * The window must not close on a drawing it has not saved. Somebody who
 * has just pressed Save is looking at the button they pressed, so the
 * reason goes beside the box it is about and not only in the status bar
 * at the other end of the window — and the drawing stays on the canvas,
 * because a drawing that vanished along with the reason is one they have
 * to make again.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

test("a drawing with no name does not close the window", () => {
  const body = grab("saveDrawnSymbol");
  // It stops before it ever asks the server, and it does not close.
  assert.match(body, /if \(!name\) \{\s*symbolNameIsWrong\(/);
  const beforeReturn = body.slice(0, body.indexOf("symbolNameIsWrong") + 400);
  assert.ok(!beforeReturn.includes('$("#draw").close()'),
    "the window closes before the name is checked");
});

test("the reason is said beside the box, not only in the status bar", () => {
  const body = grab("symbolNameIsWrong");
  assert.match(body, /#sym-name-why/);
  assert.match(body, /said\.hidden = false/);
  // And in the status bar as well, for anyone watching that instead.
  assert.match(body, /status\(why\)/);
});

test("the box is highlighted and ready to be typed over", () => {
  const body = grab("symbolNameIsWrong");
  assert.match(body, /classList\.add\("wrong"\)/);
  assert.match(body, /box\.focus\(\)/);
  assert.match(body, /box\.select\(\)/);
});

test("the complaint clears as soon as it is being answered", () => {
  assert.match(src, /#sym-name"\)\.addEventListener\("input", \(\) => symbolNameIsFine\(\)\)/);
  const body = grab("symbolNameIsFine");
  assert.match(body, /classList\.remove\("wrong"\)/);
  assert.match(body, /why\.hidden = true/);
});

test("a name already taken is offered as a replacement, not done quietly", () => {
  const body = grab("saveDrawnSymbol");
  assert.match(body, /res\.status === 409/);
  assert.match(body, /await confirmThat\(/);
  // Turned down, the window stays open and says why.
  assert.match(body, /if \(!sure\) \{\s*symbolNameIsWrong\(/);
  // Accepted, it says so again with replace=1.
  assert.match(body, /res = await send\(true\)/);
});

test("the window and the stylesheet have what the complaint needs", () => {
  assert.match(html, /id="sym-name-why"/);
  assert.match(html, /role="alert"/);
  assert.match(css, /input\.wrong/);
  assert.match(css, /\.field-why/);
});

/*
 * Moving the mark.
 */

test("a mark stays picked after the mouse lets go", () => {
  // So the arrow keys can take over: a mark is put roughly in place with
  // the mouse and settled with the keys.
  assert.match(src, /pickSymbol\(\{ \.\.\.where, name: img\.dataset\.symbol \}\)/);
});

test("the arrow keys move the picked mark", () => {
  assert.match(src, /if \(pickedSymbol && !e\.ctrlKey && !e\.metaKey\)/);
  assert.match(src, /ArrowLeft: \[-step, 0\]/);
  assert.match(src, /ArrowRight: \[step, 0\]/);
  assert.match(src, /ArrowUp: \[0, -step\]/);
  assert.match(src, /ArrowDown: \[0, step\]/);
});

test("shift moves it a whole space and a plain press a quarter", () => {
  assert.match(src, /const step = e\.shiftKey \? 1 : 0\.25/);
});

test("escape lets the mark go and gives the keys back", () => {
  assert.match(src, /if \(e\.key === "Escape"\) \{[\s\S]*?dropSymbol\(\);/);
});

test("the nudge is sent in staff spaces and adds to what is there", () => {
  const body = grab("nudgeSymbol");
  assert.match(body, /api\/movesymbol/);
  assert.match(body, /dx: dx\.toFixed\(3\)/);
  // No absolute=1: each press adds to the last, so a run of them adds up.
  assert.ok(!body.includes("absolute"), "a nudge replaces the offset");
});

test("the picked mark is outlined so it is clear what will move", () => {
  assert.match(grab("markPickedSymbol"), /classList\.toggle\("picked"/);
  assert.match(css, /image\[data-symbol\]\.picked/);
});

test("the selection is kept outside the drawing", () => {
  // The drawing is thrown away and made again after every edit, so a
  // selection held as an element of the page would be lost exactly when
  // it is wanted.
  assert.match(src, /let pickedSymbol = null/);
});

/*
 * Picking one up, and taking it off.
 */

test("any drawn symbol can be picked, however it was drawn", () => {
  // A grid comes out as an <image> and an outline as a <path>. Asking
  // only for the first made every symbol from the folder unpickable:
  // visible on the page and deaf to the mouse.
  assert.match(src, /e\.target\.closest\("\[data-symbol\]"\)/);
  assert.match(src, /\$\$\("#canvas-wrap \[data-symbol\]"\)/);
});

test("an outline is dragged by its transform, a grid by its corner", () => {
  assert.match(src, /isPath: img\.tagName\.toLowerCase\(\) === "path"/);
  assert.match(src, /if \(dragging\.isPath\) \{[\s\S]*?setAttribute\("transform"/);
});

test("delete takes away the picked symbol", () => {
  const body = grab("deleteSelected");
  assert.match(body, /if \(pickedSymbol\) \{/);
  assert.match(body, /api\/removesymbol/);
  // And it is asked about first: a symbol sitting over a note is what
  // the click landed on, so it is what delete means.
  const atSymbol = body.indexOf("pickedSymbol");
  const atHairpin = body.indexOf("state.selection?.hairpin");
  assert.ok(atSymbol >= 0 && atSymbol < atHairpin,
    "the symbol is not asked about before the other things");
});

test("the symbol is let go once it has gone", () => {
  const body = grab("deleteSelected");
  assert.match(body, /pickedSymbol = null;\s*await refreshScore\(\)/);
});
