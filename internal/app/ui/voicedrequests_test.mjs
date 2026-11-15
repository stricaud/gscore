/*
 * voicedrequests_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A command aimed at a place must say which line of music it means.
 *
 * Every request that reaches inside a bar names a staff, a bar and how far
 * in. On a staff where two voices share the lines that is not enough: the
 * same place holds a note of each, and the server answers about the first
 * unless it is told otherwise.
 *
 * The server takes a voice on every one of these routes and the editor has
 * withVoice to put one in — and still the voice was left out of one site
 * after another, each found only when somebody's score turned up written in
 * the second voice with the first left empty. Writing CHORUS over a chorus
 * did nothing at all: the words were aimed at a line with nothing in it, so
 * the answer was "the bar is empty".
 *
 * This is the whole family in one place. It reads the routes the server
 * resolves by voice out of the Go source — so a route that learns about
 * voices is covered the day it does, without anybody remembering to add it
 * here — and insists that every call the editor makes to one of them says
 * which line it means.
 */

import { readFileSync, readdirSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const here = new URL("./", import.meta.url);
const src = readFileSync(new URL("app.js", here), "utf8");
const lines = src.split("\n");

// goSource is every handler the server has, read as one.
const goDir = new URL("../", here);
const goSource = readdirSync(goDir)
  .filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"))
  .map((f) => readFileSync(new URL(f, goDir), "utf8"))
  .join("\n");

// routeHandlers maps /api/thing to the function that answers it.
function routeHandlers() {
  const out = new Map();
  const re = /"(\/api\/[a-z/]+)"\s*:\s*a\.(\w+)|mux\.HandleFunc\("(\/api\/[a-z/]+)",\s*a\.(\w+)\)/g;
  for (const m of goSource.matchAll(re)) {
    out.set(m[1] ?? m[3], m[2] ?? m[4]);
  }
  return out;
}

// bodyOf is one Go handler, from its signature to the closing brace in the
// first column.
function bodyOf(name) {
  const re = new RegExp(
    "^func \\(a \\*App\\) " + name + "\\(w http\\.ResponseWriter[\\s\\S]*?^}", "m");
  const m = goSource.match(re);
  return m ? m[0] : "";
}

// The ways a handler says it resolves a position by voice: either it reads
// the parameter itself, or it goes through one of the resolvers that does.
const RESOLVES_BY_VOICE =
  /voiceParam\(|\bmarksOf\(|\bmarkAt\(|\bnotesOf\(|\brestsOf\(|loneSymbolAt\(|\btieTargets\(/;

// voicedRoutes is every route whose answer depends on which line is meant.
function voicedRoutes() {
  const out = [];
  for (const [route, handler] of routeHandlers()) {
    if (RESOLVES_BY_VOICE.test(bodyOf(handler))) out.push(route);
  }
  return out.sort();
}

// callSites finds every place the editor sends one of these requests, as
// {line, route, params} — params being the name of the URLSearchParams it
// is built from.
function callSites(routes) {
  const want = new Set(routes.map((r) => r.replace("/api/", "")));
  const out = [];
  for (let i = 0; i < lines.length; i++) {
    // The request is counted from the line its fetch( begins on, and read
    // together with the one after it: some of them are wrapped, and a
    // wrapped request is one request and not two.
    if (!lines[i].includes("fetch(")) continue;
    const m = /fetch\(\s*"\/api\/(\w+)\?"\s*\+\s*(\w+)/.exec(
      lines[i] + "\n" + (lines[i + 1] ?? ""));
    if (!m || !want.has(m[1])) continue;
    out.push({ line: i + 1, route: m[1], params: m[2] });
  }
  return out;
}

// saysWhichLine looks back from the request to how it was built, and is
// true where the line is in it one way or another: withVoice put it there,
// the request set it by hand, or the position is written in the selection
// grammar, which carries the voice in the index.
function saysWhichLine(site) {
  const back = lines.slice(Math.max(0, site.line - 41), site.line + 1).join("\n");
  return back.includes("withVoice(" + site.params) ||
    new RegExp("\\b" + site.params + "\\.set\\(\"voice\"").test(back) ||
    /\bsel:/.test(back) || /"sel="/.test(back) ||
    // A preview asks what something would be called and writes nothing,
    // so there is no element for it to land on.
    /preview: "1"/.test(back);
}

test("the server resolves a good many routes by voice", () => {
  const routes = voicedRoutes();
  assert.ok(routes.length > 25,
    "only " + routes.length + " voiced routes found — the scan has stopped " +
    "reading the Go source");
  for (const r of ["/api/text", "/api/insert", "/api/tie", "/api/clef",
                   "/api/symbolscale", "/api/cursor"]) {
    assert.ok(routes.includes(r), r + " no longer resolves by voice");
  }
});

test("every request that reaches into a bar says which line it means", () => {
  const sites = callSites(voicedRoutes());
  assert.ok(sites.length > 30, "only " + sites.length + " call sites found");
  const silent = sites.filter((s) => !saysWhichLine(s))
    .map((s) => `app.js:${s.line} /api/${s.route}`);
  assert.deepEqual(silent, [],
    "these aim at a place without saying which line:\n  " + silent.join("\n  "));
});

// The one that was reported: words written beside a note.
test("the words go beside the note that was clicked", () => {
  const m = /^async function textDialog\([\s\S]*?^}/m.exec(src);
  assert.ok(m, "app.js has no textDialog");
  assert.match(m[0], /withVoice\(p, sel\.voice\)/);
});

// And the cursor the file remembers, which is where the pen starts next
// time the score is opened.
test("the remembered cursor carries its line, both ways", () => {
  const remember = /^function rememberCursor\([\s\S]*?^}/m.exec(src)[0];
  assert.match(remember, /withVoice\(q, sel\.voice\)/);
  const place = /^function placeCursorOnOpen\([\s\S]*?^}/m.exec(src)[0];
  assert.match(place, /voice: c\.voice \|\| 0/);
  assert.match(place, /setVoice\(c\.voice \|\| 0\)/,
    "the pen does not go to the line the cursor was left on");
});
