/*
 * tabmarks_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The Insert ▸ Tab menu.
 *
 * Sixteen marks, each of which has to reach the endpoint under the name the
 * endpoint knows it by. A name that does not match is a menu entry that
 * looks right and does nothing but put a complaint in the status line, and
 * the only place the two lists meet is here.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");
const api = readFileSync(
  new URL("../../../docs/openapi.yaml", import.meta.url), "utf8");

// The names the endpoint knows, read from the one place they are written
// down for anyone outside the program.
const known = /enum: \[(hammer-on[^\]]*)\]/.exec(api)[1]
  .split(",").map((s) => s.trim());

test("the endpoint knows sixteen marks", () => {
  assert.equal(known.length, 16, known.join(" "));
});

test("every mark the endpoint knows is on the menu", () => {
  const missing = known.filter(
    (n) => !html.includes(`data-action="tab-${n}"`));
  assert.deepEqual(missing, [], "marks with no way to reach them");
});

test("every entry on the menu sends a name the endpoint knows", () => {
  const sent = [...src.matchAll(/case "tab-([a-z-]+)": await setTechnique\("([a-z-]+)"\)/g)];
  assert.equal(sent.length, 16, `${sent.length} entries are wired`);
  for (const [, action, name] of sent) {
    assert.equal(action, name,
      `the entry is tab-${action} and it sends ${name}`);
    assert.ok(known.includes(name), `${name} is not a mark the endpoint knows`);
  }
});

test("none of them is left disabled", () => {
  for (const m of html.matchAll(/data-action="(tab-[a-z-]+)"([^>]*)/g)) {
    if (m[1] === "tab-transposition") continue; // not built yet, and it says so
    assert.ok(!m[2].includes("disabled"), `${m[1]} is disabled`);
  }
});

// A dead note is the shape of the note, not a mark on it: the same Head a
// drum staff gives a cymbal. Two ways to say one thing is how a file ends
// up with two marks that mean the same and no way to tell which was meant.
test("a dead note goes through the notehead endpoint", () => {
  const m = /const setDeadNote = \(\) =>\s*\n\s*mark\("notehead", \{ shape: "x" \}/.exec(src);
  assert.ok(m, "setDeadNote does not ask for an x notehead");
  assert.ok(!known.includes("dead-note"),
    "there is a dead-note mark as well as the notehead");
});

// The status line says the mark back in the words the menu used, so that
// what was clicked and what happened read the same.
test("the status line names the mark the way the menu does", () => {
  const table = /const TAB_MARKS = \{([\s\S]*?)\n\};/.exec(src)[1];
  const named = new Map(
    [...table.matchAll(/"([a-z-]+)":\s*"([^"]+)"/g)].map((m) => [m[1], m[2]]));
  assert.equal(named.size, 16, "not every mark has a name");
  for (const name of known) {
    const said = named.get(name);
    assert.ok(said, `${name} has no name for the status line`);
    assert.ok(html.includes(`<span>${said}</span>`),
      `the status line says "${said}" and no menu entry does`);
  }
});

test("the menu is in French too", () => {
  for (const m of html.matchAll(/data-action="tab-[a-z-]+"[^>]*><span>([^<]+)<\/span>/g)) {
    assert.ok(fr.includes(`"${m[1]}":`), `"${m[1]}" has no French`);
  }
});
