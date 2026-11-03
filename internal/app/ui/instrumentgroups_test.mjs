/*
 * instrumentgroups_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The instrument list is drawn under headings.
 *
 * Three hundred instruments in one flat list is a wall: everybody who has
 * ever looked for a cor anglais in one has scrolled past it twice. Grouped
 * by what the thing is — keyboard, voice, woodwind, brass, the two kinds of
 * string, the two kinds of percussion — it is a list somebody reads.
 *
 * What is checked here is the drawing: that a run of options sharing a
 * heading comes out as one <optgroup>, that a list carrying no headings is
 * drawn flat the way it always was, and that filtering keeps the headings
 * over whatever survives it.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

// The renderer lives inside ask(), which is a dialog; lifting the function
// out is how the rest of these tests reach into app.js too.
function renderer() {
  const m = src.match(/const render = \(list, keep\) => \{[\s\S]*?\n {4}\};/);
  if (!m) throw new Error("app.js has no option renderer");
  const select = { innerHTML: "", value: "", selectedIndex: -1, options: [] };
  const escapeHTML = (s) =>
    String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;")
      .replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  const body = m[0] + "\nreturn render;";
  return {
    render: new Function("select", "escapeHTML", body)(select, escapeHTML),
    select,
  };
}

const WOODWIND = [
  { value: "wind.flutes.flute", label: "flute", group: "Woodwind" },
  { value: "wind.reed.oboe", label: "oboe", group: "Woodwind" },
];
const BRASS = [
  { value: "brass.trumpet", label: "trumpet", group: "Brass" },
  { value: "brass.tuba", label: "tuba", group: "Brass" },
];

test("options carrying a heading are drawn under it", () => {
  const { render, select } = renderer();
  render([...WOODWIND, ...BRASS], "");

  const groups = [...select.innerHTML.matchAll(/<optgroup label="([^"]*)">/g)]
    .map((m) => m[1]);
  assert.deepEqual(groups, ["Woodwind", "Brass"]);
  // One heading per run, each closed before the next opens.
  assert.equal((select.innerHTML.match(/<\/optgroup>/g) || []).length, 2);
  assert.match(select.innerHTML,
    /<optgroup label="Woodwind">.*flute.*oboe.*<\/optgroup><optgroup label="Brass">/s);
});

test("a list with no headings is drawn flat, as it always was", () => {
  const { render, select } = renderer();
  render([{ value: "single", label: "Single", group: "" },
          { value: "double", label: "Double", group: "" }], "");

  assert.equal(select.innerHTML.includes("optgroup"), false);
  assert.match(select.innerHTML, /<option value="single">Single<\/option>/);
});

test("every option ends up inside its heading", () => {
  const { render, select } = renderer();
  render([...WOODWIND, ...BRASS], "");

  // Four instruments, four options: nothing is dropped by the grouping.
  assert.equal((select.innerHTML.match(/<option /g) || []).length, 4);
  // And none of them escaped before the first heading opened.
  assert.equal(select.innerHTML.indexOf("<option"),
               select.innerHTML.indexOf("<optgroup") + 1
               + '<optgroup label="Woodwind">'.length - 1);
});

test("a filtered list keeps the headings of what survives", () => {
  const { render, select } = renderer();
  // What the filter hands the renderer: one brass instrument matched.
  render([{ value: "brass.tuba", label: "tuba", group: "Brass" }], "");

  assert.match(select.innerHTML, /<optgroup label="Brass">/);
  assert.equal((select.innerHTML.match(/<optgroup /g) || []).length, 1);
  assert.equal(select.innerHTML.includes("Woodwind"), false);
});

test("a heading with a quote in it cannot break out of the markup", () => {
  const { render, select } = renderer();
  render([{ value: "x", label: "x", group: 'Strings "bowed"' }], "");

  assert.match(select.innerHTML, /<optgroup label="Strings &quot;bowed&quot;">/);
});

test("the choice already made is kept when the list is redrawn", () => {
  const { render, select } = renderer();
  render([...WOODWIND, ...BRASS], "brass.tuba");
  assert.equal(select.value, "brass.tuba");
});

/*
 * And the two places that ask the question.
 */

test("both instrument pickers read the catalogue", () => {
  // One place fetches it, both share it: the list is three hundred
  // instruments and fetching it twice per dialog is wasted.
  assert.match(src, /async function instrumentOptions\(\)/);
  assert.match(src, /instrumentCatalogue = d\.instruments \|\| \[\]/);

  const add = src.match(/async function addStaffDialog\(\)[\s\S]*?\n\}/)[0];
  const set = src.match(/async function setInstrumentDialog\(\)[\s\S]*?\n\}/)[0];
  for (const [name, body] of [["addStaffDialog", add], ["setInstrumentDialog", set]]) {
    assert.ok(body.includes("await instrumentOptions()"),
      name + " does not use the catalogue");
    assert.ok(!body.includes("x.family"),
      name + " still labels options with the General MIDI family");
  }
});

test("the picker sends the sound id, not the name", () => {
  // Several instruments share a name — two are called timpani — so the
  // name cannot be what identifies the choice. For a fretted instrument
  // the way it is written down travels with it, since "guitar —
  // notation and tablature" is one thing somebody picks.
  const add = src.match(/async function addStaffDialog\(\)[\s\S]*?\n\}/)[0];
  assert.match(add, /const chosen = soundChosen\(inst\)/);
  assert.match(add, /new URLSearchParams\(chosen\)/);

  const set = src.match(/async function setInstrumentDialog\(\)[\s\S]*?\n\}/)[0];
  assert.match(set, /setStaffProperty\("sound", soundChosen\(v\)\.sound\)/);
});

test("the way it is written is split back off the choice", () => {
  const chosen = new Function(
    src.match(/^function soundChosen\([\s\S]*?^}/m)[0] + "\nreturn soundChosen;")();
  assert.deepEqual(chosen("pluck.guitar"), { sound: "pluck.guitar" });
  assert.deepEqual(chosen("pluck.guitar/both"),
    { sound: "pluck.guitar", written: "both" });
  assert.deepEqual(chosen("pluck.bass/tab"),
    { sound: "pluck.bass", written: "tab" });
});

test("the person is told the instrument's name, never its id", () => {
  assert.match(src, /function instrumentNamed\(value\)/);
  const add = src.match(/async function addStaffDialog\(\)[\s\S]*?\n\}/)[0];
  assert.match(add, /status\("Added a staff for " \+ instrumentNamed\(inst\)\)/);
});
