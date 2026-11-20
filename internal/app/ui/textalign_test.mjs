/*
 * textalign_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A double click on words opens the window that changes them, and the
 * window says where along the bar they go.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

test("the text window offers the note, the left, the middle and the right", () => {
  const dlg = html.slice(html.indexOf('<dialog id="textmark">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  assert.match(body, /<select id="tm-align">/);
  for (const v of ["left", "centre", "right"]) {
    assert.match(body, new RegExp(`<option value="${v}">`));
  }
});

test("the window opens on the alignment there and sends the one chosen", () => {
  const fn = src.slice(src.indexOf("async function textDialog("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /\$\("#tm-align"\)\.value = editing \? \(editing\.align \|\| ""\) : ""/);
  assert.match(body, /align: \$\("#tm-align"\)\.value/);
});

test("a double click on words opens the window", () => {
  const dbl = src.slice(src.indexOf('addEventListener("dblclick"'));
  const handler = dbl.slice(0, dbl.indexOf('addEventListener("click"'));
  assert.match(handler, /textMarkAt\(x, y\)/);
  assert.match(handler, /selectTextMark\(words\);\s*await textDialog\(\)/);
});

test("the words are in French", () => {
  for (const s of ["Along the bar", "Beside the note", "At the left of the bar",
                   "In the middle of the bar", "At the right of the bar"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
