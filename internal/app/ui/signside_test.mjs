/*
 * signside_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A double click on a sign asks which end of its bar it goes at.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

test("the dialog offers the two ends of the bar", () => {
  const dlg = html.slice(html.indexOf('<dialog id="sign-side">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  assert.match(body, /name="sign-side" value="left"/);
  assert.match(body, /name="sign-side" value="right"/);
  assert.match(body, /Left — at the head of the bar/);
  assert.match(body, /Right — at the end of the bar/);
});

test("a double click on a sign opens it", () => {
  const dbl = src.slice(src.indexOf('addEventListener("dblclick"'));
  const handler = dbl.slice(0, dbl.indexOf('addEventListener("click"'));
  assert.match(handler, /signAt\(x, y\)/);
  assert.match(handler, /await signSideDialog\(/);
});

test("the choice goes to the server as a side", () => {
  const fn = src.slice(src.indexOf("async function signSideDialog("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /\/api\/jump\?measure=\$\{s\.measure\}&side=\$\{side\}/);
  // Shown where it is now, from the box the engraver sent.
  assert.match(body, /s\.atEnd/);
});

test("the words are in French too", () => {
  for (const s of ["Where the sign goes", "Left — at the head of the bar",
                   "Right — at the end of the bar"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
