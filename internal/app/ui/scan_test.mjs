/*
 * scan_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Pictures of pages: picked, dropped or fetched from the web, read by the
 * page reader into one score; and the settings that say what reads them.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

test("File has an Import submenu: a PDF, pictures of pages, the web", () => {
  const menu = html.slice(html.indexOf('<div class="submenu" id="import-menu">'));
  const body = menu.slice(0, menu.indexOf("</div>\n      </div>"));
  assert.match(body, /<button class="submenu-title"><span>Import<\/span>/);
  assert.match(body, /data-action="import-pdf"><span>PDF…<\/span>/);
  assert.match(body, /data-action="import-images"><span>Pictures of pages…<\/span>/);
  // Fetched by the machine the editor runs on, so only offered there.
  assert.match(body, /data-action="import-web" data-needs-files><span>From the web…<\/span>/);
  assert.match(src, /case "import-web":\s*await openWebDialog\(\)/);
  assert.match(src, /case "import-pdf":\s*await importPDF\(\)/);
  assert.match(src, /case "import-images":\s*await importImages\(\)/);
  const dlg = html.slice(html.indexOf('<dialog id="openweb">'));
  assert.match(dlg.slice(0, dlg.indexOf("</dialog>")), /<textarea id="openweb-urls"/);
});

test("several pictures are put in order before they are read", () => {
  const dlg = html.slice(html.indexOf('<dialog id="pageorder">'));
  assert.match(dlg.slice(0, dlg.indexOf("</dialog>")), /<ol id="pageorder-list"/);
  const fn = src.slice(src.indexOf("async function pageOrderDialog("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /if \(pages\.length < 2\) return pages/);
  assert.match(body, /sort\(byPageNumber\)/);
  assert.match(src, /numeric: true/);
  // From the system's dialog, the pages come back to be ordered and go in
  // again by path; from the browser's, the files themselves are ordered.
  assert.match(src, /if \(d\.status === "pages"\) \{\s*await readPagesInOrder\(d\.pages\)/);
  assert.match(src, /for \(const f of paths \|\| \[\]\) p\.append\("path", f\.path\)/);
  const chosen = src.slice(src.indexOf("async function openChosen("));
  assert.match(chosen.slice(0, chosen.indexOf("\n}\n")), /pageOrderDialog\(files\.map/);
});

test("pasting into the address box is left to the browser", () => {
  // The music's own copy and paste step aside for a text box and for any
  // open window, which is what lets the usual keys work in the box.
  assert.match(src, /e\.target\.matches\("input, textarea"\)\) &&\s*!document\.querySelector\("dialog\[open\]"\)/);
});

test("pictures of pages can be picked, several at once", () => {
  assert.match(html, /<input type="file" id="file-input" accept="[^"]*\.png,\.jpg,\.jpeg" multiple hidden>/);
  const fn = src.slice(src.indexOf("async function openChosen("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /files\.every\(\(f\) => isScanName\(f\.name\)\)\) \{\s*if \(files\.length === 1\) return openScans\(files\)/);
  assert.match(body, /return openFile\(files\[0\]\)/);
  // Picking and dropping both go through it.
  assert.match(src, /openChosen\(\[\.\.\.e\.target\.files\]\)/);
  assert.match(src, /await openChosen\(files\);/);
});

test("the pages go to the reader as one request, in order", () => {
  const fn = src.slice(src.indexOf("async function openScans("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /body\.append\("page", f, f\.name\)/);
  assert.match(body, /fetch\("\/api\/openscan", \{ method: "POST", body \}\)/);
  assert.match(body, /p\.append\("url", u\)/);
  // The answer may carry the error, since the line was held open.
  assert.match(body, /if \(d\.error\)/);
  assert.match(body, /busy\(`Reading/);
});

test("the settings choose what the reader runs on", () => {
  const dlg = html.slice(html.indexOf('<dialog id="settings">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  for (const v of ["cpu", "cuda", "custom"]) {
    assert.match(body, new RegExp(`<select id="setting-scan">[\\s\\S]*<option value="${v}">`));
  }
  assert.match(body, /<input type="text" id="setting-scan-command"/);
  const fn = src.slice(src.indexOf("async function settingsDialog("));
  const js = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(js, /p\.set\("scanReader", scan\.value\)/);
  assert.match(js, /if \(scan\.value === "custom"\) p\.set\("scanCommand", scanCmd\.value\.trim\(\)\)/);
  assert.match(js, /now\.scanEngine/);
});

test("the words are in French", () => {
  for (const s of ["Import", "Pictures of pages…", "From the web…", "The order of the pages", "Reading scanned pages", "Run it on", "A command of my own"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
