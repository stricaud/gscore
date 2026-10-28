/*
 * tremoloclick_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Picking up a two-note tremolo.
 *
 * The beams between the two stems are the whole of the mark: it is not a
 * glyph on a note, and there is nothing else on the page that stands for
 * it. A click on them fell through to whatever was underneath, so a
 * tremolo could not be picked up — and could not be seen to have been
 * picked up, which from where anybody is sitting is the same thing.
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

// One tremolo, drawn between two notes a little way into the bar.
const trem = {
  part: 0, measure: 4, from: 11, to: 12,
  x: 20, y: 6, width: 3.4, height: 2, beams: 2,
};

function editor() {
  const state = {
    tremolos: [trem], multi: [], selection: null, anchor: null, focus: null,
  };
  const said = [];
  const scope = {
    state,
    Math,
    status: (s) => said.push(s),
    markSelection: () => {},
  };
  const names = ["tremoloAt", "selectTremolo"];
  const body = names.map((n) => grab(n)).join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { state, said, ...api };
}

test("a click on the beams finds the tremolo", () => {
  const t = editor();
  assert.equal(t.tremoloAt(21.5, 7), trem, "the middle of the beams is not on them");
  assert.equal(t.tremoloAt(20, 6), trem, "the corner of the beams is not on them");
});

// The box is drawn tight on purpose: the beams run over the noteheads of
// the pair they join, and a generous target here is a notehead nobody can
// click.
test("a click beside the beams finds nothing", () => {
  const t = editor();
  assert.equal(t.tremoloAt(19, 7), null, "it reaches out past its left edge");
  assert.equal(t.tremoloAt(24.5, 7), null, "it reaches out past its right edge");
  assert.equal(t.tremoloAt(21.5, 3), null, "it reaches up above the beams");
  assert.equal(t.tremoloAt(21.5, 11), null, "it reaches down below the beams");
});

test("picking it up picks up the note it is written on, and shows the beams", () => {
  const t = editor();
  t.selectTremolo(trem);
  assert.equal(t.state.selection.ref, trem.from,
    "what is picked up is not the note the mark is written on");
  assert.equal(t.state.selection.part, trem.part);
  assert.equal(t.state.selection.measure, trem.measure);
  assert.equal(t.state.selection.tremolo, trem,
    "nothing says which beams were clicked, so nothing can be drawn round them");
  assert.deepEqual(t.state.multi, [], "a passage was left picked up beside it");
  assert.match(t.said.join(" "), /tremolo/i, "it does not say what was picked up");
  assert.match(t.said.join(" "), /Backspace/, "it does not say how to take it away");
});

// What is drawn round it, since the selection is a note and the box is not
// that note's.
test("the box goes round the beams, not round the note", () => {
  const fn = grab("markSelection");
  assert.ok(/sel\.tremolo/.test(fn), "a picked-up tremolo is not drawn at all");
  assert.ok(/sel\.tremolo[\s\S]{0,200}selectionRect\(/.test(fn),
    "nothing is drawn round the beams that were clicked");
});

// Backspace takes the whole of it away: half a mark draws nothing and plays
// as an ordinary note, but it is still in the file.
test("Backspace takes it off both of its notes", () => {
  const fn = grab("deleteSelected");
  assert.ok(/state\.selection\?\.tremolo/.test(fn),
    "Backspace does not know a tremolo can be picked up");
  assert.ok(/api\/tremolo\?strokes=0/.test(fn),
    "it does not ask for the tremolo to be taken off");
});

// Shift used to be how music was picked up with the mouse, and the pointer
// turned into a hand to say so. A plain click picks it up now.
test("holding Shift no longer turns the pointer into a hand", () => {
  assert.ok(!src.includes("shift-select"),
    "the score still shows a hand while Shift is held");
  assert.ok(!src.includes("setupShiftHint"),
    "the hint is still being set up");
  const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
  assert.ok(!/body\.shift-select/.test(css),
    "the rule that drew the hand is still there");
});
