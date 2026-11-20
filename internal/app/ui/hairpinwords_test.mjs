/*
 * hairpinwords_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A crescendo can be written as the word from the menu and the palette.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const palette = readFileSync(new URL("./palette.js", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

test("the menu offers cresc. and dim. beside the wedges", () => {
  assert.match(html, /data-action="hairpin-cresc-words"><span>cresc\. — as a word<\/span>/);
  assert.match(html, /data-action="hairpin-dim-words"><span>dim\. — as a word<\/span>/);
  // In the same list as the wedges, so that it is found where they are.
  const line = html.slice(html.indexOf('data-action="hairpin-cresc"'), html.indexOf('data-action="hairpin-none"'));
  assert.match(line, /hairpin-cresc-words/);
});

test("the actions ask the server for the words", () => {
  assert.match(src, /case "hairpin-cresc-words": await setHairpin\("cresc\."/);
  assert.match(src, /case "hairpin-dim-words": await setHairpin\("dim\."/);
});

test("the palette has them too, needing a stretch", () => {
  assert.match(palette, /text: "cresc\.".*value: "cresc\.",\s*needsRange: true/s);
  assert.match(palette, /text: "dim\.".*value: "dim\.",\s*needsRange: true/s);
});

test("the words are in French", () => {
  assert.ok(fr.includes('"cresc. — as a word":'));
  assert.ok(fr.includes('"dim. — as a word":'));
});
