/*
 * scannote_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * When the page reader dropped parts of a PDF, the person is told, and
 * offered the reading that keeps them.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

const fnBody = (name) => {
  const fn = src.slice(src.indexOf(`function ${name}(`));
  return fn.slice(0, fn.indexOf("\n}\n"));
};

test("the warning is a window with the way back to the glyphs", () => {
  const dlg = html.slice(html.indexOf('<dialog id="scan-note">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  assert.match(body, /<h2>Not every part was read<\/h2>/);
  // Enter reads again, which is the thing to do; keeping is the cancel.
  assert.match(body, /value="ok"[^>]*>Read it from the glyphs instead</);
  assert.match(body, /value="cancel"[^>]*>Keep this reading</);
  assert.match(fnBody("scanNoteDialog"), /return answer === "ok"/);
});

test("every way a PDF comes in shows it and can read again", () => {
  for (const name of ["openFile", "importByPath"]) {
    assert.match(fnBody(name), /if \(d\.warning && await scanNoteDialog\(d\.warning\)\) \{\s*reader = "&reader=glyphs";\s*continue;/, name);
  }
  assert.match(fnBody("openRecent"), /if \(d\.warning && await scanNoteDialog\(d\.warning\)\)[\s\S]*importByPath\(path, [^,]+, "glyphs"\)/);
  assert.match(fnBody("openScore"), /if \(d\.warning && await scanNoteDialog\(d\.warning\)\)[\s\S]*importByPath\(d\.filename, [^,]+, "glyphs"\)/);
});

test("the words are in French", () => {
  for (const s of ["Not every part was read", "Read it from the glyphs instead", "Keep this reading"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
