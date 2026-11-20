/*
 * tokey_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The transpose window offers a key as well as an interval.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

const dlg = html.slice(html.indexOf('<dialog id="transpose">'));
const body = dlg.slice(0, dlg.indexOf("</dialog>"));

test("the window asks whether by an interval or to a key", () => {
  assert.match(body, /name="transpose-how" value="interval" checked/);
  assert.match(body, /name="transpose-how" value="key"/);
  assert.match(body, /<select id="transpose-tokey">/);
  for (const v of ["closest", "up", "down"]) {
    assert.match(body, new RegExp(`name="transpose-way" value="${v}"`));
  }
  // The interval controls are still there, in their own block.
  assert.match(body, /<div id="transpose-by-interval">[\s\S]*id="transpose-interval"/);
});

test("to a key sends the key and the way, not an interval", () => {
  const fn = src.slice(src.indexOf("async function transposeBy("));
  const tb = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(tb, /p\.set\("tokey", String\(opt\.toKey\)\)/);
  assert.match(tb, /p\.set\("direction", opt\.direction \|\| "closest"\)/);
  assert.match(tb, /p\.delete\("semitones"\)/);
  const dl = src.slice(src.indexOf("async function transposeDialog("));
  const td = dl.slice(0, dl.indexOf("\n}\n"));
  assert.match(td, /transposeBy\(null, 0, \{ toKey, direction, key:/);
  // Opening on the key the music is in now.
  assert.match(td, /\$\("#transpose-tokey"\)\.value = String\(now\)/);
});

test("the words are in French", () => {
  for (const s of ["By an interval", "To a key", "Whichever is nearer"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
