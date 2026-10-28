/*
 * websave_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Saving and exporting from a page.
 *
 * gscore in a browser has no disk: no machine to write to, no path to write
 * at. Save there means "give me the file", and the panel that asks where it
 * goes belongs to the browser — so every save in a page is a save-as. It
 * asked the server to write a file instead, which answered that it did not
 * know where, and the menu entry did nothing.
 *
 * The exports have the same shape and one more trap: two of the formats
 * gscore writes are zips, and a zip read as text comes back with every byte
 * that is not valid UTF-8 replaced, which is a file nothing can open.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// The editor against a server that answers once, with a note of what was
// asked for, what the person was told, and what the browser was handed.
function load({ info, body, headers = {}, ok = true, status = 200 }) {
  const asked = [];
  const said = [];
  const kept = [];
  let readAsText = false;

  const res = {
    ok,
    status,
    headers: { get: (k) => headers[k.toLowerCase()] ?? null },
    async text() { readAsText = true; return typeof body === "string" ? body : ""; },
    async blob() { return { bytes: body }; },
    async json() { return body; },
  };
  const scope = {
    state: { info, scripts: info.scripts || [] },
    status: (s) => said.push(s),
    async fetch(url) { asked.push(url); return res; },
    async loadInfo() {},
    confirmThat: async () => true,
    URL: { createObjectURL: () => "blob:x", revokeObjectURL() {} },
    document: {
      createElement() {
        const a = { click() { kept.push({ name: a.download, href: a.href }); } };
        return a;
      },
    },
  };
  const body_ = [grab("keepFile"), grab("saveScore"), grab("exportAs"),
                 grab("keepsScripts")].join("\n") +
    "\nreturn { saveScore, exportAs };";
  const api = new Function(...Object.keys(scope), body_)(...Object.values(scope));
  return { ...api, asked, said, kept, readAsText: () => readAsText };
}

const PAGE = { local: false, native: false, title: "Sanctus" };
const DESKTOP = { local: true, native: true, title: "Sanctus", filename: "/tmp/x.gscore" };

test("saving in a page hands the file to the browser", async () => {
  const t = load({
    info: PAGE,
    body: "PK...",
    headers: { "content-disposition": 'attachment; filename="Sanctus.gscore"' },
  });

  await t.saveScore(false);

  assert.deepEqual(t.asked, ["/api/save"], "the page asked for something else");
  assert.equal(t.kept.length, 1, "nothing was handed to the browser");
  assert.equal(t.kept[0].name, "Sanctus.gscore",
               "the name the server gave the file was not used");
  assert.ok(t.said[t.said.length - 1].startsWith("Saved"), t.said.join(" | "));
});

test("saving in a page never asks the server to write a path", async () => {
  // There is no disk. A path would be the server's, and on a public one it
  // is refused — which is how this came back as "could not save".
  const t = load({ info: PAGE, body: "PK", headers: {} });
  await t.saveScore(false);
  for (const url of t.asked) {
    assert.ok(!url.includes("path="), "it asked to write " + url);
  }
});

test("a save that fails says why", async () => {
  const t = load({ info: PAGE, ok: false, status: 400, body: "say where to save it" });
  await t.saveScore(false);
  assert.ok(t.said[t.said.length - 1].includes("say where"), t.said.join(" | "));
  assert.equal(t.kept.length, 0);
});

test("the desktop still saves to its own file", async () => {
  const t = load({ info: DESKTOP, body: { path: "/tmp/x.gscore" },
                   headers: { "content-type": "application/json" } });
  await t.saveScore(false);
  assert.ok(t.asked[0].includes("path="), "the desktop stopped saving in place");
  assert.equal(t.kept.length, 0, "the desktop downloaded a file instead of writing one");
});

test("an export keeps its bytes instead of reading them as text", async () => {
  // .mxl and .gscore are zips. This is the whole reason keepFile exists.
  const t = load({ info: PAGE, body: "PK\u0003\u0004",
                   headers: { "content-disposition": 'attachment; filename="Sanctus.mxl"' } });

  await t.exportAs("mxl", "application/vnd.recordare.musicxml", "compressed MusicXML");

  assert.equal(t.readAsText(), false,
               "the answer was read as text, which breaks every zip");
  assert.equal(t.kept[0].name, "Sanctus.mxl");
});

test("an export that fails says why and hands over nothing", async () => {
  const t = load({ info: PAGE, ok: false, status: 400, body: "nothing writes .docx" });
  await t.exportAs("docx", "text/plain", "Word");
  assert.ok(t.said[t.said.length - 1].includes(".docx"), t.said.join(" | "));
  assert.equal(t.kept.length, 0);
});
