/*
 * keys_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the keys do.
 *
 * The keyboard was a switch of sixty cases and is now a table, and a table
 * that resolves the wrong keystroke is worse than the switch was: the switch
 * at least could not quietly put two commands on one key. So the keystrokes
 * are pressed here, through the same chordOf and boundKeys the editor uses,
 * and checked against the command they should reach.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

// The table and the two functions that read it, taken out as they are
// written. Nothing in them is called, so the commands they would run need
// not exist here.
function load(keys = {}) {
  const table = src.slice(
    src.indexOf("const KEY_BINDINGS = ["),
    src.indexOf("\n];", src.indexOf("const KEY_BINDINGS = [")) + 3);
  const grab = (name) => {
    const m = src.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
    if (!m) throw new Error("app.js has no " + name);
    return m[0];
  };
  const body = table + "\n" + grab("chordOf") + "\n" + grab("keyFor") + "\n" +
    grab("boundKeys") + "\n" + grab("shortcutText") + "\n" + grab("chordParts") + "\n" +
    "return { KEY_BINDINGS, chordOf, keyFor, boundKeys, shortcutText };";
  return new Function("state", "navigator", body)(
    { keys, multi: [], zoom: 8 }, { platform: "MacIntel" });
}

// press builds the event a real keypress would be.
const press = (key, mods = "") => ({
  key,
  ctrlKey: mods.includes("c"),
  metaKey: mods.includes("m"),
  altKey: mods.includes("a"),
  shiftKey: mods.includes("s"),
});

test("a keystroke reaches the command it is written against", () => {
  const f = load();
  const map = f.boundKeys();
  const cases = [
    // [key, modifiers, the command it should reach]
    [" ", "", "write-note"],
    [" ", "s", "play"],
    [" ", "c", "chord-note"],
    [" ", "cs", "play-from-start"],
    ["Enter", "", "write-rest"],
    ["Tab", "", "split-measure"],
    ["Backspace", "", "delete"],
    ["Escape", "", "clear-selection"],
    ["4", "", "dur-quarter"],
    ["5", "", "dur-quarter-dot"],
    [".", "", "dot"],
    ["+", "", "sharpen"],
    ["-", "", "flatten"],
    ["=", "", "accidental-natural"],
    // The tie is shift with t, which arrives as a capital T.
    ["T", "s", "tie"],
    ["S", "s", "slur"],
    ["t", "", "insert-text"],
    ["s", "", "save"],
    ["b", "", "beam"],
    ["ArrowLeft", "", "cursor-left"],
    ["ArrowLeft", "s", "extend-left"],
    ["ArrowLeft", "c", "shift-earlier"],
    ["ArrowRight", "m", "shift-later"],
    ["ArrowUp", "", "cursor-up"],
    ["ArrowUp", "s", "cursor-up-octave"],
    ["ArrowUp", "a", "nudge-up"],
    ["c", "c", "copy"],
    ["c", "m", "copy"],
    ["z", "c", "undo"],
    ["Z", "cs", "redo"],
    ["3", "c", "tuplet-3"],
    ["0", "c", "zoom-reset"],
    ["+", "c", "zoom-in"],
    ["-", "c", "zoom-out"],
  ];
  for (const [key, mods, want] of cases) {
    const chord = f.chordOf(press(key, mods));
    const got = map.get(chord);
    assert.ok(got, `${mods}+${key} resolved to ${chord}, which nothing is on`);
    assert.equal(got.id, want, `${mods}+${key} is ${chord}`);
  }
});

test("every command in the table can be reached by its keys", () => {
  const f = load();
  const map = f.boundKeys();
  for (const b of f.KEY_BINDINGS) {
    assert.equal(map.get(b.keys)?.id, b.id,
      `${b.id} is on ${b.keys}, which reaches ${map.get(b.keys)?.id}`);
  }
});

test("a changed binding takes effect and gives up the key it had", () => {
  const f = load({ save: "Mod+s" });
  const map = f.boundKeys();
  assert.equal(map.get("Mod+s")?.id, "save", "the new key should save");
  assert.equal(map.get("s"), undefined,
    "the old key should do nothing once the binding has moved off it");
});

test("a changed binding beats a default sitting on the same key", () => {
  // Somebody puts Slur on the key Save came with. Save keeps answering to
  // its own default and so both would be on "s"; the one that was asked for
  // has to win, or the change does nothing and looks broken.
  const f = load({ slur: "s" });
  const map = f.boundKeys();
  assert.equal(map.get("s")?.id, "slur");
});

test("the modifiers on their own are not a keystroke", () => {
  const f = load();
  const map = f.boundKeys();
  for (const key of ["Shift", "Control", "Alt", "Meta"]) {
    assert.equal(map.get(f.chordOf(press(key))), undefined,
      `${key} on its own should not run anything`);
  }
});

test("a keystroke reads the way it is pressed", () => {
  const f = load();
  assert.equal(f.shortcutText("Mod+s"), "⌘ S");
  assert.equal(f.shortcutText("Shift+ArrowLeft"), "⇧ ←");
  assert.equal(f.shortcutText("Space"), "Space");
  assert.equal(f.shortcutText("Mod++"), "⌘ +");
  assert.equal(f.shortcutText("+"), "+");
  // A capital letter is shift held and that letter, and the reader has to
  // be told to hold it.
  assert.equal(f.shortcutText("T"), "⇧ T");
  assert.equal(f.shortcutText("Mod+Z"), "⌘ ⇧ Z");
  // Compact, for the menus, where the hints sit in a tight column.
  assert.equal(f.shortcutText("Mod+s", true), "⌘S");
  assert.equal(f.shortcutText("T", true), "⇧T");
});
