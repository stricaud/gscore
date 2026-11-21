/*
 * newscore_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A new score is Mod+N, not n, and asks before throwing unsaved work away.
 * Reported as: "I typed n by mistake and lost all the changes I made."
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

const fnBody = (name) => {
  const fn = src.slice(src.indexOf(`function ${name}(`));
  return fn.slice(0, fn.indexOf("\n}\n"));
};

test("New takes the modifier", () => {
  assert.match(src, /\{ id: "new", menu: "new", keys: "Mod\+n",/);
  assert.doesNotMatch(src, /id: "new", menu: "new", keys: "n"/);
});

test("New asks before replacing an unsaved score", () => {
  const i = src.indexOf('    case "new":');
  const body = src.slice(i, src.indexOf('    case "open":', i));
  assert.ok(body.indexOf("offerToSave") < body.indexOf('"/api/new"'),
    "the score is replaced before anybody is asked");
  const offer = fnBody("offerToSave");
  assert.match(offer, /if \(state\.readOnly \|\| !state\.info\?\.unsaved\) return true/);
  assert.match(offer, /if \(answer === "discard"\) return true/);
  assert.match(offer, /if \(answer !== "ok"\) return false/);
  assert.match(offer, /await saveScore\(false\)/);
  // A cancelled save is no reason to go on.
  assert.match(offer, /return !state\.info\?\.unsaved/);
});

test("the window offers save, don't save and cancel, Enter saving", () => {
  const dlg = html.slice(html.indexOf('<dialog id="unsaved">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  assert.match(body, /<button value="ok"[^>]*class="primary">Save</);
  assert.match(body, /<button value="discard"/);
  assert.match(body, /<button value="cancel"/);
});

test("opening another score asks the same, every way in", () => {
  for (const name of ["openScore", "importPDF", "importImages", "openRecent", "openWebDialog"]) {
    assert.match(fnBody(name), /if \(!\(await offerToSave\("Open another score"\)\)\)/, name);
  }
  // Dropped on the window too.
  assert.match(src, /if \(!files\.length\) return;\s*if \(!\(await offerToSave\("Open another score"\)\)\)/);
});
