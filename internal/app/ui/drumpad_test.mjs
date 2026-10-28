/*
 * drumpad_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The drum pads: when they are there, and what pressing one does.
 *
 * They used to be up for the whole of any score with a drum staff anywhere
 * in it. On an orchestral score that is a kit under the music the whole
 * time, for one staff out of fifty, and most of the time the cursor is
 * nowhere near it. A drum kit is for writing drums on.
 *
 * And pressing a pad used to only choose a drum: the drum then had to be
 * written by clicking the staff as well. Two gestures for one note, and the
 * second of them a click on five lines that name instruments rather than
 * pitches — which is exactly the guessing the pads are here to replace.
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

// showDrumPad, with a panel that is nothing but a hidden flag.
function pad({ kit = { pads: [] }, readOnly = false, parts = [], selection = null } = {}) {
  const panel = { hidden: true };
  const state = { kit, readOnly, selection, info: { parts } };
  const scope = { state, $: (sel) => (sel === "#drumpad" ? panel : null) };
  const body = grab("isDrumStaff") + "\n" + grab("showDrumPad") +
    "\nreturn { showDrumPad };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, panel };
}

const orchestra = [
  { clef: "treble" }, { clef: "treble" }, { clef: "bass" },
  { clef: "percussion" }, { clef: "treble" },
];

test("the pads are there when the cursor is on a drum staff", () => {
  const t = pad({ parts: orchestra, selection: { part: 3 } });
  t.showDrumPad();
  assert.equal(t.panel.hidden, false, "a drummer was given no pads");
});

test("and not when it is anywhere else in the same score", () => {
  for (const part of [0, 1, 2, 4]) {
    const t = pad({ parts: orchestra, selection: { part } });
    t.showDrumPad();
    assert.equal(t.panel.hidden, true,
      `staff ${part} is not a drum staff and had a kit under it`);
  }
});

test("nor with nothing picked up at all", () => {
  const t = pad({ parts: orchestra, selection: null });
  t.showDrumPad();
  assert.equal(t.panel.hidden, true, "the pads were up with no cursor anywhere");
});

test("nor on a score that cannot be written in", () => {
  const t = pad({ parts: orchestra, selection: { part: 3 }, readOnly: true });
  t.showDrumPad();
  assert.equal(t.panel.hidden, true, "a score that cannot be edited offered pads");
});

test("nor before the kit has arrived", () => {
  const t = pad({ kit: null, parts: orchestra, selection: { part: 3 } });
  t.showDrumPad();
  assert.equal(t.panel.hidden, true, "pads were drawn from a kit that is not here yet");
});

test("the pads follow the cursor as it moves", () => {
  const fn = grab("markSelection");
  assert.ok(fn.includes("showDrumPad()"),
    "the cursor moves to another staff and the kit stays where it was");
});

test("pressing a pad writes the drum", () => {
  const fn = grab("hitPad");
  assert.ok(fn.includes("pickDrum("), "pressing a pad does not choose its drum");
  assert.ok(fn.includes("insertAtSelection("),
    "pressing a pad chooses a drum and writes nothing, so the staff must be " +
    "clicked as well");

  // The letter on the pad and the pad itself are one thing, not two that
  // can drift apart.
  const typed = grab("typeDrum");
  assert.ok(typed.includes("hitPad("),
    "a pad's letter writes by its own route, which will not stay the same " +
    "as the pad's");

  // And the pad's click reaches it.
  const render = grab("renderDrumPads");
  assert.ok(/addEventListener\("click", \(\) => void hitPad\(/.test(render),
    "a pad press does not write");
});

test("choosing a drum from the palette writes it too, where there is somewhere to write", () => {
  // The same two meanings a note value and an accidental already have:
  // with something in front of you it acts on it, with nothing it is a
  // decision about the next note.
  const fn = grab("paletteAction");
  const bit = fn.slice(fn.indexOf("item.drum !== undefined"));
  assert.ok(bit.includes("insertAtSelection("),
    "the palette's drums only arm the pen, where its note values write");
  assert.ok(bit.includes("isDrumStaff("),
    "the palette writes a drum wherever the cursor happens to be");
  // And the advice is right: a click points rather than writes, so what is
  // missing is a cursor on a drum staff, not another click.
  assert.ok(bit.includes("put the cursor on a drum staff"),
    "with nowhere to write it, it tells you to click the staff — which now " +
    "only picks music up");
});
