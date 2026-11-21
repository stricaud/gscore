/*
 * savelock_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A .gscore is locked under a password on Save as, not from a menu entry;
 * and a locked file opened simply asks for its password.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

const fnBody = (name) => {
  const fn = src.slice(src.indexOf(`async function ${name}(`));
  return fn.slice(0, fn.indexOf("\n}\n"));
};

test("the password is no menu entry", () => {
  assert.doesNotMatch(html, /data-action="protect"/);
  assert.doesNotMatch(src, /case "protect":/);
  assert.doesNotMatch(src, /function protectDialog/);
});

test("Save as asks whether to lock, a plain save does not", () => {
  const dlg = html.slice(html.indexOf('<dialog id="savelock">'));
  const body = dlg.slice(0, dlg.indexOf("</dialog>"));
  assert.match(body, /name="savelock" value="open" checked/);
  assert.match(body, /name="savelock" value="locked"/);
  assert.match(body, /<input type="password" id="savelock-pw"/);
  assert.match(body, /<input type="password" id="savelock-again"/);
  assert.match(body, /carries no PDF of the\s+music in front of it/);
  const save = fnBody("saveScore");
  // Before the panel only where the panel cannot ask it itself.
  assert.match(save, /if \(saveAs && !state\.info\?\.savePanelLocks\) \{\s*lock = await saveLockDialog\(\);/);
  assert.match(save, /withLock\(""\), "Saving…", lock\.headers/);
});

test("the answer is a lock flag and a header, never a password in the address", () => {
  const body = fnBody("saveLockDialog");
  assert.match(body, /return \{ query: "lock=0", headers: \{\} \}/);
  assert.match(body, /return \{ query: "lock=1", headers: passwordHeader\(pw\) \}/);
  // A score opened locked keeps its password when the boxes are left empty.
  assert.match(body, /if \(!pw && locked\) return \{ query: "lock=1", headers: \{\} \}/);
  assert.doesNotMatch(src, /password=\$\{/);
  assert.match(src, /"X-Gscore-Password": password/);
});

test("a locked file opened asks for its password, as often as needed", () => {
  const body = fnBody("openWithPassword");
  assert.match(body, /res\.status !== 401 \|\| !res\.headers\.get\("X-Needs-Password"\)/);
  assert.match(body, /secret: true/);
  assert.match(src, /openWithPassword\(\(password\) =>\s*fetch\("\/api\/open\?name="/);
  assert.match(src, /openWithPassword\(\(password\) =>\s*fetch\("\/api\/openpath\?path="/);
});

test("the words are in French", () => {
  for (const s of ["Save it open", "Lock it under a password", "Save as .gscore"]) {
    assert.ok(fr.includes(`"${s}":`), `${s} has no French`);
  }
  assert.ok(!fr.includes('"Protect with a password…":'), "the menu entry's French is left behind");
});
