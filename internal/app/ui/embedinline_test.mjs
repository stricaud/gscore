/*
 * embedinline_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Music written into a page rather than kept in a file beside it.
 *
 * A file is the wrong shape for eight bars in the middle of a paragraph,
 * so the page carries the tune and a script engraves it where it stands.
 * The two halves have to agree about one thing — the message that hands
 * the music over — and this is what holds them to it.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const embed = readFileSync(
  new URL("../../../web/site/embed.js", import.meta.url), "utf8");
const docs = readFileSync(
  new URL("../../../docs/embedding.md", import.meta.url), "utf8");
const build = readFileSync(
  new URL("../../../web/build.sh", import.meta.url), "utf8");

const grab = (where, name) => {
  const m = where.match(
    new RegExp("^(?:\\s*)?function " + name + "\\([\\s\\S]*?^  \\}", "m")) ||
    where.match(new RegExp("function " + name + "\\([\\s\\S]*?\\n  \\}", "m"));
  if (!m) throw new Error("no " + name);
  return m[0];
};

test("the editor takes a score as the page hands it over", () => {
  assert.match(src, /case "open": \{/);
  // Text for ABC, bytes for the zipped formats: both are what File takes,
  // and the name picks the reader as it does for a file being opened.
  assert.match(src, /typeof m\.value === "string"/);
  assert.match(src, /new Uint8Array\(m\.value\)/);
  assert.match(src, /new File\(\[what\], String\(m\.name \|\| "tune\.abc"\)\)/);
});

test("the script waits to be told the frame is up", () => {
  // Sending before then sends into a page with no listener, and the tune
  // is silently never read.
  assert.match(embed, /m\.gscore !== "ready"/);
  assert.match(embed, /gscore: "open"/);
  // And only to its own frame: a page with three tunes on it has three
  // listeners waiting at once.
  assert.match(embed, /e\.source !== frame\.contentWindow/);
});

// ABC cares about the left margin, and nobody should have to break the
// shape of their page to write music in it.
test("the block's own indentation comes off", () => {
  const body = grab(embed, "dedent");
  assert.match(body, /Math\.min/);
  assert.match(embed, /dedent\(el\.textContent/);
});

// The class says the program, not the format: a block may hold MusicXML
// or a MuseScore file as readily as ABC.
test("what it looks for is not named after one format", () => {
  assert.match(embed, /".gscore"/);
  assert.doesNotMatch(embed, /abc-source/);
  assert.doesNotMatch(docs, /abc-source/);
});

test("the page says what the music is written in", () => {
  const body = grab(embed, "formatOf");
  assert.match(body, /dataset\.format/);
  // What a fenced code block in markdown already produces.
  assert.match(body, /language\|lang/);
  assert.match(body, /return "abc"/);
});

test("something that is not text can be written in as base64", () => {
  const body = grab(embed, "contentOf");
  assert.match(body, /base64/);
  assert.match(body, /atob/);
  assert.match(body, /Uint8Array/);
});

// The author's own text stays where they put it.
test("the source is kept in the page", () => {
  assert.match(embed, /el\.hidden = true/);
  assert.match(embed, /insertBefore\(frame, el\.nextSibling\)/);
  assert.doesNotMatch(embed, /el\.remove\(\)|innerHTML = ""/);
});

test("the script is published with the site, and documented", () => {
  assert.match(build, /\$site\/embed\.js/);
  assert.match(docs, /Music written into the page/);
  assert.match(docs, /data-encoding=base64|data-encoding="base64"/);
  assert.match(docs, /gscore: "open"/);
});
