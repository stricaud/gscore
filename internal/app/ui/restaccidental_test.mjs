/*
 * restaccidental_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * An accidental clicked while a rest is picked up.
 *
 * An accidental means two things depending on what is in front of you: with
 * a note selected it is written on that note, and with nothing selected it
 * is armed for the next note. With a rest picked up it is neither — a rest
 * has no notehead to put a sign on, so writing it there failed with "there
 * is no note in the selection", and because the click had been spent on
 * failing, the pen was not armed either.
 *
 * Clicking a bar of rest and reaching for a sharp is somebody about to write
 * a sharpened note into that bar. The answer they got was that they could
 * not write anything at all. Reported as "I cannot enter notes anymore".
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

// One bar holding a rest and a note, and whichever of them the test picks up.
function load(picked) {
  const REST = { ref: 1, index: 0, voice: 0, kind: "rest", steps: [] };
  const NOTE = { ref: 2, index: 1, voice: 0, kind: "note", steps: [4] };
  const state = {
    hits: [{ part: 0, measure: 0, elements: [REST, NOTE] }],
    multi: [],
    selection: { part: 0, measure: 0, at: picked.index + 1, voice: 0,
                 ref: picked.ref, step: 0 },
    tool: { duration: "quarter", dots: 0, accidental: "none", rest: false },
    voice: 0,
  };
  const said = [];
  const written = [];
  const scope = {
    state,
    status: (s) => said.push(s),
    markTools() {},
    async setAccidental(acc, label) { written.push(acc); },
    async setSelectionDuration() {},
    heldForChanging: () => false,
    hasSelection: () => true,
    mark: async () => null,
    pickedHead: () => ({}),
    setVoice(n) { state.voice = n; },
    t: (s) => s,
  };
  const body = grab("drawnAt") + "\n" + grab("pickedUpIsPitched") + "\n" +
    grab("paletteAction") + "\nreturn { paletteAction, pickedUpIsPitched };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { state, said, written, ...api, REST, NOTE };
}

const SHARP = { g: "", label: "Sharp", tool: "acc", acc: "sharp" };

test("a rest has no notehead to put a sign on", () => {
  const { pickedUpIsPitched } = load({ ref: 1, index: 0 });
  assert.equal(pickedUpIsPitched(), false);
});

test("a note has", () => {
  const { pickedUpIsPitched } = load({ ref: 2, index: 1 });
  assert.equal(pickedUpIsPitched(), true);
});

test("with a rest picked up the accidental is armed for the next note", async () => {
  const { state, said, written, paletteAction } = load({ ref: 1, index: 0 });
  await paletteAction(SHARP);
  assert.equal(written.length, 0,
               "it tried to write the sign on the rest, which cannot carry one");
  assert.equal(state.tool.accidental, "sharp",
               "the pen was not armed, so the next note cannot be sharpened");
  assert.match(said.join(" "), /next note/);
});

test("with a note picked up it is still written on that note", async () => {
  const { state, written, paletteAction } = load({ ref: 2, index: 1 });
  await paletteAction(SHARP);
  assert.deepEqual(written, ["sharp"]);
  assert.equal(state.tool.accidental, "none",
               "the pen was armed as well, so every note after it gets a sharp");
});
