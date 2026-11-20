/*
 * pastemusescore_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A paste that came from MuseScore says so, and says what could not be read.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const fn = src.slice(src.indexOf("async function pasteClipboard("));
const body = fn.slice(0, fn.indexOf("\n}\n"));

test("the status names where the music came from", () => {
  assert.match(body, /if \(d\.from\) msg \+= ` from \$\{d\.from\}`/);
  assert.match(body, /d\.skipped\?\.length/);
  assert.match(body, /could not be read/);
});

test("the paste itself asks nothing new of the server", () => {
  // The server reads the clipboard; the page sends the same request as
  // ever, since it cannot see what MuseScore put there.
  assert.match(body, /fetch\("\/api\/paste\?" \+ p\.toString\(\), \{ method: "POST" \}\)/);
});
