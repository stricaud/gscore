/*
 * abcline_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Typing a line of ABC notation in.
 *
 * The editor's own shorthand stays what a bare line is read as: it is the
 * shorter of the two and the one the hint under the box describes. A line
 * that opens with "abc" and a space hands the rest to ABC, which is a
 * notation a great many people already have in their fingers.
 *
 * The reading itself is done in Go, by the same parser that opens an .abc
 * file. What is checked here is the one decision made in the page: which of
 * the two a line is.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

const konst = (name) => {
  const m = src.match(new RegExp("^const " + name + " =([\\s\\S]*?);$", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return new Function("return (" + m[1] + ")")();
};

const abcPrefix = konst("abcPrefix");
const rest = (line) => {
  const m = abcPrefix.exec(line);
  return m ? line.slice(m[0].length) : null;
};

test("a line that opens with abc and a space is ABC", () => {
  assert.equal(rest("abc cdef|gabc'|"), "cdef|gabc'|");
  assert.equal(rest("ABC cdef"), "cdef");
  // More than one space between them is still one word followed by music.
  assert.equal(rest("abc   cdef"), "cdef");
});

test("the editor's own shorthand is untouched", () => {
  // A run of the notes a, b and c is written with a length in front of it,
  // which is not the word.
  assert.equal(rest("8abc"), null);
  assert.equal(rest("4 c d e"), null);
  assert.equal(rest("abcdef"), null, "a bare run was taken for the prefix");
});

test("the word on its own is not an ABC line", () => {
  // Nothing after it at all falls through to the editor's own reading,
  // which will say what it makes of the word.
  assert.equal(rest("abc"), null);
});

test("the word and a space is an empty ABC line", () => {
  // Somebody who meant to type a phrase and hit return early. It goes to
  // the reader, which answers that there is nothing to read — which is the
  // truth, and better than the editor's own reading answering that there
  // is no command called abc.
  assert.equal(rest("abc "), "");
});

test("the hint says the prefix is there", () => {
  const m = src.match(/^const typeHint =([\s\S]*?);$/m);
  assert.ok(m, "app.js has no typeHint");
  assert.match(m[1], /abc/, "the hint does not mention ABC");
});

test("the line is sent to the ABC endpoint, as one thing to undo", () => {
  const m = src.match(/^async function runABC\([\s\S]*?^}/m);
  assert.ok(m, "app.js has no runABC");
  assert.match(m[0], /\/api\/abc\?/, "it does not call the ABC endpoint");
  assert.match(m[0], /script\/begin/, "it is not bracketed for undo");
  assert.match(m[0], /script\/end/, "the undo bracket is not closed");
});
