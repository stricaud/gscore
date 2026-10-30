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
    // Not the underscore NoteWorthy uses: the flat has it, because the
    // underscore is also how a flat is written wherever music is typed
    // rather than drawn, and an accidental is reached far oftener than a
    // tenuto. Asked for as "_ would be bemol".
    "artic-tenuto": ":",
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

// The signs as they are written, which is what a hand reaches for and what
// the typing bar already takes.
test("the accidental signs are on the keys that draw them", () => {
  const want = {
    "accidental-sharp": "#",
    "accidental-flat": "_",
    "accidental-natural": "=",
  };
  for (const [id, key] of Object.entries(want)) {
    const b = all.find((x) => x.id === id);
    assert.ok(b, "there is no binding called " + id);
    assert.equal(b.keys, key, id + " is on the wrong key");
    assert.equal(b.group, "Accidentals", id + " is filed somewhere else");
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

// A sign that cannot be typed without a modifier.
//
// On a good many keyboards # or _ is Option or AltGr with a digit. The
// modifier is how the character is made, not part of a chord, so a binding
// written against the character alone was unreachable for anyone whose
// layout makes it that way.
test("a sign typed with Option still reaches its binding", () => {
  const scope = {
    state: { keys: {} },
    KEY_BINDINGS: [
      { id: "accidental-sharp", keys: "#" },
      { id: "accidental-flat", keys: "_" },
      { id: "nudge-up", keys: "Alt+ArrowUp" },
      { id: "take-alt", keys: "Alt+$" },
      { id: "plain-a", keys: "a" },
    ],
  };
  const grab = (name) => {
    const m = src.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
    if (!m) throw new Error("app.js has no " + name);
    return m[0];
  };
  const body = [grab("chordOf"), grab("boundKeys"), grab("bindingFor")].join("\n") +
    "\nreturn { bindingFor };";
  const { bindingFor } = new Function(...Object.keys(scope), body)(
    ...Object.values(scope));

  const press = (key, mods = {}) => bindingFor({ key, ...mods })?.id ?? null;

  assert.equal(press("#"), "accidental-sharp", "the plain character");
  assert.equal(press("#", { altKey: true }), "accidental-sharp",
               "typed with Option, as several layouts require");
  assert.equal(press("_", { altKey: true }), "accidental-flat");

  // A binding that really is on Alt keeps it.
  assert.equal(press("$", { altKey: true }), "take-alt");
  assert.equal(press("ArrowUp", { altKey: true }), "nudge-up");
  // Letters and digits are chords, not characters made with a modifier.
  assert.equal(press("a", { altKey: true }), null, "Alt+a is a chord");
  // And Ctrl or Command with it is a chord whatever the character.
  assert.equal(press("#", { altKey: true, ctrlKey: true }), null);
});

// What a sign key does depends on what is in front of you, exactly as the
// palette's buttons do: write it on what is picked up, or hold it for the
// next note where nothing is.
test("a sign key writes on what is picked up, or arms the pen", () => {
  const grab = (name) => {
    const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
    if (!m) throw new Error("app.js has no " + name);
    return m[0];
  };
  function run(state) {
    const wrote = [];
    const said = [];
    const scope = {
      state,
      setAccidental: (name, label) => { wrote.push(name); },
      markTools() {},
      status: (s) => said.push(s),
      isDrumStaff: (part) => part === 9,
    };
    const body = [grab("pickedNotes"), grab("typeAccidental")].join("\n") +
      "\nreturn { typeAccidental };";
    const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
    api.typeAccidental("sharp", "Sharp");
    return { wrote, said, tool: state.tool.accidental };
  }

  const picked = run({
    multi: [], tool: { accidental: "none" },
    selection: { part: 0, measure: 0, ref: 7 },
  });
  assert.deepEqual(picked.wrote, ["sharp"], "a picked-up note was not sharpened");
  assert.equal(picked.tool, "none", "the pen was armed as well");

  const empty = run({
    multi: [], tool: { accidental: "none" },
    selection: { part: 0, measure: 0, ref: null },
  });
  assert.deepEqual(empty.wrote, [], "it wrote on a note that is not picked up");
  assert.equal(empty.tool, "sharp", "the pen was not armed");
  assert.ok(empty.said.join(" ").includes("next note"));

  // A marking names the note it was written on. A sharp on a slur means a
  // sharp on that note, which is not what pointing at the slur asked for.
  const slur = run({
    multi: [], tool: { accidental: "none" },
    selection: { part: 0, measure: 0, ref: 7, arc: { kind: "slur" } },
  });
  assert.deepEqual(slur.wrote, [], "a picked-up slur sharpened the note under it");

  // A drum staff has no accidentals, and arming one for it would be a pen
  // held for a note that can never take it.
  const drums = run({
    multi: [], tool: { accidental: "none" },
    selection: { part: 9, measure: 0, ref: null },
  });
  assert.deepEqual(drums.wrote, []);
  assert.equal(drums.tool, "none", "the pen was armed on a drum staff");
});
