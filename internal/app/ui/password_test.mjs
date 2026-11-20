/*
 * password_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A score locked under a password: set from the File menu, asked for when
 * a locked file is opened.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

test("the File menu offers the password beside Save as", () => {
  const menu = html.slice(html.indexOf('data-action="save-as"'), html.indexOf('data-action="save-as"') + 400);
  assert.match(menu, /data-action="protect"><span>Protect with a password…<\/span>/);
});

test("the window asks for the password twice and can take it off", () => {
  const dlg = html.slice(html.indexOf('<dialog id="protect">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  assert.match(body, /<input type="password" id="protect-pw"/);
  assert.match(body, /<input type="password" id="protect-again"/);
  assert.match(body, /id="protect-remove"/);
  // It says there is no PDF in front of a locked file, so that nobody
  // expects to share one for the picture.
  assert.match(body, /carries\s+no PDF of the music in front of it/);
});

test("the password travels in a header, never in the address", () => {
  assert.match(src, /"X-Gscore-Password": password/);
  assert.doesNotMatch(src, /password=\$\{/);
  const fn = src.slice(src.indexOf("async function protectDialog("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /fetch\("\/api\/password", \{\s*method: "POST", headers: passwordHeader\(pw\)/);
  assert.match(body, /\/api\/password\?clear=1/);
});

test("opening asks again while the server says the password is needed", () => {
  const fn = src.slice(src.indexOf("async function openWithPassword("));
  const body = fn.slice(0, fn.indexOf("\n}\n"));
  assert.match(body, /res\.status !== 401 \|\| !res\.headers\.get\("X-Needs-Password"\)/);
  assert.match(body, /secret: true/);
  // Both ways of opening go through it.
  assert.match(src, /openWithPassword\(\(password\) =>\s*fetch\("\/api\/open\?name="/);
  assert.match(src, /openWithPassword\(\(password\) =>\s*fetch\("\/api\/openpath\?path="/);
});

test("a password typed in the ask window is not shown", () => {
  const fn = src.slice(src.indexOf("function ask({"));
  const body = fn.slice(0, 600);
  assert.match(body, /secret = false/);
  assert.match(body, /input\.type = secret \? "password" : "text"/);
});

test("the words are in French", () => {
  for (const s of ["Protect with a password…", "Password", "Remove the password"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
});
