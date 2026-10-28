/*
 * markkeys_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Typing the marks on a note.
 *
 * Every articulation was a menu item and a palette button and nothing else,
 * which is three gestures for a staccato dot. They are typed far more often
 * than anything else in a score, so they have keys now — the ones a hand
 * that has written music elsewhere already knows, and the mark's own shape
 * where it has one.
 *
 * Two keys are the thing to get wrong here. One is binding a key that is
 * already doing something, which silently takes it away from whatever had
 * it. The other is a menu item claiming a key it does not have, or having
 * one it does not say — the hints are filled in from the table, so a menu
 * item with no <kbd> in it is a key nobody will ever find.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

// Every binding in the table, as {id, keys, menu, group}.
function bindings() {
  const table = src.slice(src.indexOf("const KEY_BINDINGS"));
  const out = [];
  const re = /\{\s*id:\s*"([^"]+)",([\s\S]*?)\n\s*(?=\{|\];)/g;
  let m;
  while ((m = re.exec(table))) {
    const body = m[2];
    const keys = body.match(/keys:\s*"((?:[^"\\]|\\.)*)"/);
    const menu = body.match(/menu:\s*"([^"]+)"/);
    const group = body.match(/group:\s*"([^"]+)"/);
    out.push({
      id: m[1],
      keys: keys ? keys[1] : null,
      menu: menu ? menu[1] : null,
      group: group ? group[1] : null,
    });
  }
  return out;
}

const all = bindings();

test("the table was read", () => {
  assert.ok(all.length > 60, `only ${all.length} bindings found`);
  assert.ok(all.some((b) => b.id === "tie" && b.keys === "T"));
});

test("the marks NoteWorthy types are typed here too", () => {
  const want = {
    "artic-accent": "!",
    "artic-marcato": "^",
    "artic-staccato": ",",
    "artic-staccatissimo": "'",
    "artic-tenuto": "_",
    "hairpin-cresc": "<",
    "hairpin-dim": ">",
    "tie-slash": "/",
    "slur-semicolon": ";",
  };
  for (const [id, key] of Object.entries(want)) {
    const b = all.find((x) => x.id === id);
    assert.ok(b, "there is no binding called " + id);
    assert.equal(b.keys, key, id + " is on the wrong key");
    assert.equal(b.group, "Marks", id + " is filed somewhere other than Marks");
  }
});

test("no key was taken from something that already had it", () => {
  const seen = new Map();
  for (const b of all) {
    if (!b.keys) continue;
    const had = seen.get(b.keys);
    assert.ok(!had,
      `${b.id} and ${had} are both on ${b.keys}; the later one wins and the ` +
      `earlier one stops working`);
    seen.set(b.keys, b.id);
  }
});

test("a mark's menu item has somewhere to show its key", () => {
  // The hints are filled in from the table by syncMenuKeys, which looks for
  // the first <kbd> inside the menu item. Without one the key exists and
  // nothing says so.
  for (const b of all) {
    if (!b.menu) continue;
    const item = html.match(
      new RegExp(`<button[^>]*data-action="${b.menu}"[^>]*>[\\s\\S]*?</button>`));
    if (!item) continue; // some menus are built in script
    assert.ok(item[0].includes("<kbd"),
      `${b.menu} is bound to ${b.keys} and has no <kbd> to show it in`);
  }
});

test("two keys for one mark do not fight over the menu hint", () => {
  // syncMenuKeys writes into the first <kbd> of the item named by `menu`,
  // so a second binding for the same thing must not name one — it would
  // overwrite the first one's hint with its own key.
  for (const id of ["tie-slash", "slur-semicolon"]) {
    const b = all.find((x) => x.id === id);
    assert.equal(b.menu, null,
      `${id} names a menu item that another binding already labels`);
  }
});

test("a chord is typed by stacking onto the note just written", () => {
  const b = all.find((x) => x.id === "chord-note");
  assert.ok(b, "there is no way to add a notehead from the keyboard");
  assert.ok(b.keys, "the chord key is unbound");
  assert.ok(src.includes("async function chordAtCursor()"),
    "the chord key points at nothing");
});
