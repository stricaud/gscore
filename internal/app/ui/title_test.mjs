/*
 * title_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the score is called, and the star on it.
 *
 * A star while there is work that is not on the disk — in the status line,
 * and in the Mac application's title bar, which is where somebody looking at
 * the window looks for it. The application sets a title when the Finder
 * hands it a file and then never hears about the editing, so the page has to
 * tell it.
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

// The editor's own functions, with the application replaced by a note of
// what it was told.
function load({ native = true } = {}) {
  const posted = [];
  const window = native
    ? { webkit: { messageHandlers: { gscore: { postMessage: (m) => posted.push(m) } } } }
    : {};
  const body = grab("documentName") + "\n" + grab("tellNativeTitle") +
    "\nreturn { documentName, tellNativeTitle };";
  const api = new Function("window", body)(window);
  return { posted, ...api };
}

test("a saved score is called by its name", () => {
  const { documentName } = load();
  assert.equal(documentName({ filename: "altes-2e-solo-op21.gscore", unsaved: false }),
               "altes-2e-solo-op21.gscore");
});

test("an edited score carries a star", () => {
  const { documentName } = load();
  assert.equal(documentName({ filename: "altes-2e-solo-op21.gscore", unsaved: true }),
               "altes-2e-solo-op21.gscore *");
});

test("a score that has never been saved is untitled, star and all", () => {
  const { documentName } = load();
  assert.equal(documentName({ unsaved: false }), "untitled");
  assert.equal(documentName({ unsaved: true }), "untitled *");
});

test("the application is told the name, star included", () => {
  const { posted, tellNativeTitle } = load();
  tellNativeTitle("altes-2e-solo-op21.gscore *");
  assert.deepEqual(posted, ["title:altes-2e-solo-op21.gscore *"]);
});

test("outside the application it is nothing at all", () => {
  const { posted, tellNativeTitle } = load({ native: false });
  tellNativeTitle("untitled *"); // must not throw in a plain browser
  assert.deepEqual(posted, []);
});
