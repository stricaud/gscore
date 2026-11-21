/*
 * pdfreader_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A PDF that could be read from its glyphs or as pictures is asked about,
 * and an import shows the page reader's progress while it waits.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

const fnBody = (name) => {
  const i = src.indexOf(`function ${name}(`);
  assert.ok(i >= 0, `app.js has no ${name}`);
  const fn = src.slice(i);
  return fn.slice(0, fn.indexOf("\n}\n"));
};

test("the question offers homr or the glyphs, one to a line, and suggests by the staves", () => {
  // Each choice on a line of its own, as the other questions are laid out.
  const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
  assert.match(css, /#pdfreader-form > label, #savelock-form > label \{ display: block/);
  const dlg = html.slice(html.indexOf('<dialog id="pdfreader">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  assert.match(body, /name="pdfreader" value="glyphs"/);
  assert.match(body, /name="pdfreader" value="pages"/);
  assert.match(body, /With homr, as pictures of the pages/);
  assert.match(body, /folded into one\s+piano part/);
  const js = fnBody("pdfReaderDialog");
  assert.match(js, /const suggest = staves > 2 \? "glyphs" : "pages"/);
});

test("every way in asks first, then sends the answer", () => {
  assert.match(fnBody("openFile"), /let reader = isPDF\(file\.name\) \? "&ask=1" : ""/);
  assert.match(fnBody("openFile"), /if \(d\.status === "choose"\)[\s\S]*reader = "&reader=" \+ how/);
  assert.match(fnBody("openScore"), /fetch\("\/api\/pickfile\?ask=1"/);
  assert.match(fnBody("openScore"), /if \(d\.status === "choose"\)[\s\S]*importByPath\(d\.filename/);
  assert.match(fnBody("importByPath"), /let reader = how \? "&reader=" \+ how : "&ask=1"/);
});

test("an import says it is importing and shows the reader's progress", () => {
  // The dialog only picks; the reading is a request of its own, so the
  // status is not left saying "choose".
  const pdf = fnBody("importPDF");
  assert.match(pdf, /fetch\("\/api\/pickfile\?list=1"/);
  assert.match(pdf, /await importByPath\(first\.path, first\.name\)/);
  const by = fnBody("importByPath");
  assert.match(by, /const label = `Importing \$\{name\}…`/);
  assert.match(by, /busy\(label\)/);
  assert.match(by, /const stop = watchScanProgress\(label\)/);
  const watch = fnBody("watchScanProgress");
  assert.match(watch, /fetch\("\/api\/scanprogress"\)/);
  assert.match(watch, /\$\{label\}\$\{pct\}/);
  assert.match(watch, /if \(shown\) idle\(\)/);
  // The pictures' reading shows it too.
  assert.match(fnBody("openScans"), /watchScanProgress\(`Reading \$\{what\}…`\)/);
});

test("the words are in French", () => {
  for (const s of ["How to read this PDF", "staves to a system, homr reads it well."]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
