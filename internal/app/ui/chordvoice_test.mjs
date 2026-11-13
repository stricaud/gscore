/*
 * chordvoice_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Adding a notehead to a chord, and what is left behind afterwards.
 *
 * A chord is built from the bottom up: write the lowest note, move the
 * cursor, add the next. Two things got in the way of that.
 *
 * The chord was left picked up, so the arrow that comes next moved the chord
 * instead of the cursor — the note just added went back where it came from
 * and took the rest of the chord with it. Reported as: "when I add a note to
 * a chord, I want to keep typing notes but if I hit up or down it moves the
 * chord".
 *
 * And the line of the staff never travelled with the request, so in a bar of
 * two voices the notehead was stacked on whatever shared that index in the
 * voice above. Reported as: "I am in voice 2, I just entered this note,
 * however if I use the arrow key down it makes all the notes same, or look
 * unreadable/ugly".
 *
 * Underneath that second one: the cursor never knew which line it was in.
 * The pen chose the voice and the request carried it, but the cursor left
 * behind still said the first — and at counts the elements of one voice, not
 * of the bar, so everything that then looked up "the note just written"
 * found somebody else's note.
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

// One bar of two voices: a chord in the upper line and a held note in the
// lower one. Both are element 0 of their own voice, which is the whole
// difficulty — counting along the drawing finds the upper one twice.
const UPPER = { ref: 1, index: 0, voice: 0, kind: "chord", steps: [0, 2] };
const LOWER = { ref: 2, index: 0, voice: 1, kind: "note", steps: [-2] };

function load(names, extra = {}) {
  const said = [];
  const sent = [];
  const state = {
    hits: [{ part: 0, measure: 0, elements: [UPPER, LOWER] }],
    multi: [], selection: null, anchor: null, focus: null,
    voice: 0, tool: { accidental: "none" },
    ...(extra.state || {}),
  };
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection() {},
    soundAsWritten() {},
    useAccidental() { state.tool.accidental = "none"; },
    async refreshScore() {},
    URLSearchParams,
    fetch: async (url) => {
      sent.push(url);
      return { ok: true, async json() { return {}; }, async text() { return ""; } };
    },
  };
  const body = [grab("withVoice"), grab("drawnAt"), ...names.map(grab)].join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, state, said, sent };
}

test("the notehead goes into the line the note it joins is written in", async () => {
  const t = load(["addPitch"], { state: { voice: 1 } });
  await t.addPitch(t.state.hits[0], LOWER, 0);
  assert.equal(t.sent.length, 1);
  const q = new URLSearchParams(t.sent[0].split("?")[1]);
  assert.equal(q.get("voice"), "1", "the second voice was not named, so the " +
               "notehead was stacked on the first voice's chord");
  assert.equal(q.get("index"), "0");
});

// The first voice is what a request with no voice at all already means, so
// it is left out rather than written as zero.
test("the first voice is left unsaid", async () => {
  const t = load(["addPitch"]);
  await t.addPitch(t.state.hits[0], UPPER, 4);
  const q = new URLSearchParams(t.sent[0].split("?")[1]);
  assert.equal(q.get("voice"), null);
});

test("the chord is not left picked up, so the next arrow aims the next note", async () => {
  const t = load(["addPitch", "pickedNotes", "heldForMoving"]);
  await t.addPitch(t.state.hits[0], UPPER, 4);
  assert.equal(t.state.selection.wrote, true);
  assert.equal(t.pickedNotes(), true,
               "it is still selected, so a dynamic or a tie lands on it");
  assert.equal(t.heldForMoving(), false,
               "the arrow moved the chord instead of the cursor");
});

test("what was picked up before is let go of", async () => {
  const t = load(["addPitch"], {
    state: {
      multi: [{ part: 0, measure: 3, index: 1, voice: 0 }],
      anchor: { part: 0, measure: 3, index: 1, voice: 0 },
    },
  });
  await t.addPitch(t.state.hits[0], UPPER, 4);
  assert.deepEqual(t.state.multi, []);
  assert.deepEqual(t.state.anchor,
                   { part: 0, measure: 0, index: 0, voice: 0 },
                   "a shift-click next would select from somewhere else");
});

test("the cursor stays in the line the chord is in", async () => {
  const t = load(["addPitch"], { state: { voice: 1 } });
  await t.addPitch(t.state.hits[0], LOWER, 0);
  assert.equal(t.state.selection.voice, 1);
  assert.equal(t.state.selection.ref, 2,
               "the ref was read off the drawing by counting, not by voice");
});

// Which notehead this is, so that a sign or a Backspace straight afterwards
// is about the note just added rather than about the whole chord.
test("the notehead just added is the one named", async () => {
  const t = load(["addPitch", "pickedHead"]);
  await t.addPitch(t.state.hits[0], UPPER, 4);
  assert.equal(t.state.selection.head, 4);
  assert.deepEqual(t.pickedHead(), { head: "4" });
});

/*
 * Writing a note into a line of the staff.
 *
 * The cursor has to come out of it knowing which line it is in, because at
 * counts the elements of one voice. Left saying the first, the lookup that
 * selects what was just written counted along the drawing instead — every
 * voice flattened into one list — and picked up a note of the line above.
 */

function writing(extra = {}) {
  const said = [];
  const sent = [];
  const state = {
    hits: [{ part: 0, measure: 0, full: false, elements: [UPPER, LOWER] }],
    multi: [], anchor: null, focus: null,
    voice: 1,
    tool: { duration: "quarter", dots: 0, accidental: "none", drum: null, rest: false },
    selection: { part: 0, measure: 0, at: 0, step: 0, voice: 0, head: 3, ref: null },
    info: { parts: [{ clef: "treble" }] },
    ...(extra.state || {}),
  };
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection() {},
    scrollSelectionIntoView() {},
    carryCaretOn() {},
    soundAsWritten() {},
    useAccidental() { state.tool.accidental = "none"; },
    isDrumStaff: () => false,
    drumName: () => "",
    barNumberOf: (m) => m + 1,
    async refreshScore() {},
    URLSearchParams,
    fetch: async (url) => {
      sent.push(url);
      return {
        ok: true,
        async json() { return { measure: 0, at: 1 }; },
        async text() { return ""; },
      };
    },
  };
  const names = ["withVoice", "drawnAt", "insertAtSelection"];
  const body = names.map(grab).join("\n") + "\nreturn { insertAtSelection };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, state, said, sent };
}

test("a note is written into the line the pen is on", async () => {
  const t = writing();
  await t.insertAtSelection(false);
  const q = new URLSearchParams(t.sent[0].split("?")[1]);
  assert.equal(q.get("voice"), "1");
});

test("and the cursor comes out of it in that line", async () => {
  const t = writing();
  await t.insertAtSelection(false);
  assert.equal(t.state.selection.voice, 1,
               "the cursor still claimed the first voice, so at meant " +
               "something else from here on");
});

test("what was written is what is selected, not the note drawn beside it", async () => {
  const t = writing();
  await t.insertAtSelection(false);
  assert.equal(t.state.selection.ref, 2,
               "counting along the drawing picked up the other voice's chord");
  assert.equal(t.state.selection.wrote, true);
});

// A notehead of a chord picked up earlier has nothing to do with the note
// written now, and left standing it aimed the next accidental at a staff
// step this note may not even have.
test("the notehead of whatever was picked up before is let go of", async () => {
  const t = writing();
  await t.insertAtSelection(false);
  assert.equal(t.state.selection.head, null);
});
