/*
 * dyntext_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A dynamic written by hand, and the guess at how loud it plays.
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
const dynamicLevelOf = new Function(grab("dynamicLevelOf") + "\nreturn dynamicLevelOf;")();

// The last standard marking in the words is the level offered: mp-f is a
// swell that ends loud, p subito is p.
test("the level is guessed from the last marking in the words", () => {
  assert.equal(dynamicLevelOf("mp-f"), "f");
  assert.equal(dynamicLevelOf("p subito"), "p");
  assert.equal(dynamicLevelOf("più f"), "f");
  assert.equal(dynamicLevelOf("ff/pp"), "pp");
  assert.equal(dynamicLevelOf("n"), "niente");
});

test("words with no marking in them offer nothing", () => {
  assert.equal(dynamicLevelOf("dolce"), "");
  assert.equal(dynamicLevelOf(""), "");
});

test("the menu offers it and the dialog is there", () => {
  assert.match(html, /data-action="dyn-custom"/);
  assert.match(html, /<dialog id="dyntext">/);
  assert.match(src, /case "dyn-custom": await customDynamicDialog\(\);/);
});

// What is sent: the words and how loud, which is what the server insists on.
test("the dialog writes the words with the level they play at", () => {
  const fn = grab("customDynamicDialog");
  assert.match(fn, /mark\("dynamic", \{ dynamic: plays\.value, text: written \}/);
});

// Two clicks on a dynamic open it for changing, with the marking filled in.
test("a double click on a dynamic opens the dialog on it", () => {
  const fn = grab("setupCanvasClicks");
  const at = fn.indexOf('addEventListener("dblclick"');
  assert.ok(at > 0, "the canvas does not listen for a double click");
  const handler = fn.slice(at, fn.indexOf('addEventListener("click"', at));
  assert.match(handler, /dynamicAt\(x, y\)/, "the double click does not look for a dynamic");
  assert.match(handler, /selectDynamic\(dyn\)/, "the dynamic is not picked up first, so the dialog opens empty");
  assert.match(handler, /customDynamicDialog\(\)/);
});
