/*
 * hairpindashes_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A double click on "cresc." asks how long the line after it is.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

test("the dialog offers the end, none, or a count", () => {
  const dlg = html.slice(html.indexOf('<dialog id="hairpin-line">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  for (const v of ["end", "none", "count"]) {
    assert.match(body, new RegExp(`name="hairpin-line" value="${v}"`));
  }
  assert.match(body, /id="hairpin-line-count"[^>]*min="1"/);
});

test("a double click on the words opens it, and not on a wedge", () => {
  const dbl = src.slice(src.indexOf('addEventListener("dblclick"'));
  const handler = dbl.slice(0, dbl.indexOf('addEventListener("click"'));
  assert.match(handler, /hairpinAt\(x, y\)/);
  assert.match(handler, /hair && hair\.words/);
  assert.match(handler, /await hairpinLineDialog\(hair\)/);
});

test("the answer goes to the server as dashes on the starting note", () => {
  const fn = src.slice(src.indexOf("async function hairpinLineDialog("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /\/api\/hairpin\?dashes=\$\{dashes\}&sel=/);
  assert.match(body, /refTarget\(h\.from\)/);
  assert.match(body, /h\.dashes/);
});

test("the words are in French", () => {
  for (const s of ["The line after the word", "To where it stops",
                   "None — the word alone", "This many dashes"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
