/*
 * names_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * One name, one thing.
 *
 * app.js is one script of nine thousand lines and three hundred functions.
 * Two of them with the same name is not an error anywhere — the later one
 * quietly becomes the only one, and whatever called the first now calls
 * something else entirely. A voice-leading panel was written with a
 * drawVoices in it, and there was already a drawVoices drawing the list of
 * voices in the recording panel: the recording panel would have started
 * drawing parallel fifths and nothing would have said a word.
 *
 * Nothing here is about what the functions do. It is about there being one
 * of each.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

function declaredTwice(pattern) {
  const seen = new Map();
  for (const m of src.matchAll(pattern)) {
    seen.set(m[1], (seen.get(m[1]) || 0) + 1);
  }
  return [...seen].filter(([, n]) => n > 1).map(([name, n]) => `${name} (${n})`);
}

test("no function is declared twice", () => {
  const twice = declaredTwice(/^(?:async )?function ([A-Za-z0-9_$]+)\s*\(/gm);
  assert.deepEqual(twice, [],
    "these are declared more than once, and only the last of each exists");
});

test("no top-level const is declared twice", () => {
  // Two consts of one name is a SyntaxError rather than a silent swap, so
  // this one is a loaded gun rather than a trap — but it is the same check
  // and the same line of defence.
  const twice = declaredTwice(/^const ([A-Za-z0-9_$]+) =/gm);
  assert.deepEqual(twice, [], "these are declared more than once");
});

// The thing the check is for: the script has to run at all.
test("the script parses", () => {
  assert.doesNotThrow(() => new Function(src));
});
