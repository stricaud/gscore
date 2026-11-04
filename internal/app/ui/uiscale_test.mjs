/*
 * uiscale_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How big the interface is drawn.
 *
 * Everything in it is measured in pixels, and a pixel is a physical size
 * that depends on the screen: on a large high-resolution display the whole
 * interface came out tiny while the score beside it was perfectly
 * readable. Reported as "the resolution is high and makes everything tiny",
 * and then narrowed: "my problem is mostly with the application than the
 * score".
 *
 * So the score is deliberately left out of this. It is scaled by View ▸
 * Zoom, in staff spaces, which is the measurement music is written in.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");
const api = readFileSync(
  new URL("../../../docs/openapi.yaml", import.meta.url), "utf8");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

test("the menu entry is there and in French", () => {
  assert.match(html, /<span>Interface size<\/span>/);
  assert.match(html, /id="uiscale-menu"/);
  assert.ok(fr.includes('"Interface size":'), "no French");
  assert.ok(fr.includes('"Designed size":'), "no French for the normal size");
});

// One list of sizes, in the program. The menu is built from it.
test("the sizes are written down once", () => {
  assert.match(src, /const UI_SCALES = \[/);
  assert.match(grab("fillUIScaleMenu"), /for \(const v of UI_SCALES\)/);
  assert.doesNotMatch(html, /ui-scale-1\d\d/,
    "a size is written into the markup as well as the program");
});

/*
 * zoom is the obvious tool for this and it is the wrong one. With a zoom
 * anywhere on the page, Chromium stops sending mouse events to the panels
 * fixed to the body — the menus' own flyouts — while drawing them where
 * they belong and telling elementFromPoint they are there. Every variant
 * fails the same way: the zoom on the panel or on an ancestor, the panel
 * fixed or absolute, a transform instead of a zoom.
 */
test("the interface is scaled by its type, not by zoom", () => {
  assert.doesNotMatch(css, /^\s*zoom:/m,
    "zoom is back; the menus' flyouts stop taking clicks when it is");
  // Every type size follows the scale.
  const plain = [...css.matchAll(/font-size:\s*(\d+(?:\.\d+)?px)/g)];
  assert.deepEqual(plain.map((m) => m[1]), [],
    "a type size that does not grow with the interface");
  assert.match(css, /:root \{ --ui-scale: 1; \}/);
});

// The boxes have to grow with the words in them, or the words are cut off.
test("the menus' own measurements grow too", () => {
  for (const want of [
    /#menubar \{[\s\S]*?height: calc\(34px \* var\(--ui-scale\)\)/,
    /min-width: calc\(210px \* var\(--ui-scale\)\)/,
    /top: calc\(33px \* var\(--ui-scale\)\)/,
    /grid-template-columns:\s*\n?\s*calc\(164px \* var\(--ui-scale\)\)/,
  ]) {
    assert.match(css, want);
  }
});

test("the score is left out of it", () => {
  const body = grab("applyUIScale");
  assert.doesNotMatch(body, /state\.zoom|drawScore|refreshScore/,
    "the interface scale is changing the score as well");
});

test("it is remembered, and the endpoint takes it", () => {
  assert.match(grab("setUIScale"), /\/api\/settings\?uiScale=/);
  assert.match(api, /name: uiScale/);
  // And applied before anything is drawn, so the interface does not come
  // up at one size and jump to another.
  assert.match(src, /applyUIScale\(kept\.uiScale \|\| 1\)/);
});

// An open flyout is placed by script against the window, and the panel it
// places is not scaled by anything — so the numbers it writes are the
// window's own and need no conversion.
test("a flyout is placed in the window's own pixels", () => {
  const body = grab("placeSubmenu");
  assert.match(body, /items\.style\.left = left \+ "px"/);
  assert.match(body, /items\.style\.top = top \+ "px"/);
  assert.doesNotMatch(body, /offsetWidth|offsetHeight/,
    "a measurement in the panel's own pixel, mixed with the window's");
});
