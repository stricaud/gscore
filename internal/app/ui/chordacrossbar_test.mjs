/*
 * chordacrossbar_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Building a chord on the note that filled a bar.
 *
 * Writing the note that fills a bar carries the caret over the barline, so
 * that the next note written carries on in the next bar rather than aiming
 * at a bar with no room in it. The note just written stays selected behind
 * it: the caret says where the next note goes and the selection says what
 * the last one was, and once a bar is full those are two different places.
 *
 * Everything that looked up that note by counting back from the caret then
 * asked for element -1 of a bar the note is not in. Reported as: "I want to
 * add a chord, but pressing ctrl+space does not add the note to the previous
 * one as a chord once it moved to the next measure".
 *
 * Mod+Space was the loud one. Three more went the same way quietly: Mod+C
 * and Mod+X aimed at nothing, Backspace said "nothing selected to delete",
 * and an accidental clicked in the palette decided the note had no notehead
 * to write it on and armed the pen instead.
 *
 * Two lookups came out of it, because what was written last and what is
 * picked up are not the same question. elementBeforeCaret follows the caret
 * and steps back over the barline with it; selectedElement follows the ref,
 * which names one element wherever it is drawn. Mod+Space has to be the
 * first kind: aiming the notehead with an arrow key lets go of the note, on
 * purpose — a chord is built from the bottom up, so the arrow between the
 * two keystrokes is the whole gesture — and the chord being built is still
 * the one behind the caret.
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

const NAMES = [
  "withVoice", "drawnAt", "selectedElement", "elementBeforeCaret", "selOf",
  "hasSelection", "pickedNotes", "pickedUpIsPitched", "pickedHead",
  "selectionTargets", "measureTargets", "carryCaretOn", "moveCursorPitch",
  "pitchAtCursor", "addPitch", "chordAtCursor", "deleteSelected",
];

// Four quarters fill bar 0; bar 1 is the empty one the editor keeps after
// the music so there is somewhere to carry on. The selection is what
// insertAtSelection leaves behind once the fourth quarter has filled bar 0:
// the caret at the front of bar 1, the note it just wrote still named.
function filled(over = {}) {
  const said = [];
  const sent = [];
  const elements = [0, 1, 2, 3].map((i) => ({
    ref: i + 1, index: i, voice: 0, kind: "note", steps: [i],
  }));
  const state = {
    hits: [
      { part: 0, measure: 0, full: true, elements },
      { part: 0, measure: 1, full: false, elements: [] },
    ],
    graces: [],
    multi: [], anchor: null, focus: null, voice: 0,
    tool: { accidental: "none" },
    info: { parts: [{ clef: "treble" }] },
    selection: {
      part: 0, measure: 1, at: 0, voice: 0, step: 3, ref: 4,
      head: null, replace: false, wrote: true,
      ...(over.selection || {}),
    },
    ...(over.state || {}),
  };
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection() {},
    scrollSelectionIntoView() {},
    soundAsWritten() {},
    useAccidental() { state.tool.accidental = "none"; },
    barNumberOf: (m) => m + 1,
    orderedSelection: () => [],
    pickedSymbol: null,
    async refreshScore() {},
    URLSearchParams,
    fetch: async (url) => {
      sent.push(url);
      return {
        ok: true,
        async json() { return over.reply ?? {}; },
        async text() { return ""; },
      };
    },
  };
  const body = NAMES.map(grab).join("\n") +
    "\nreturn { " + NAMES.join(", ") + " };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, state, said, sent };
}

const query = (url) => new URLSearchParams(url.split("?")[1]);

test("the notehead goes on the note behind the barline", async () => {
  const t = filled();
  // Up a step to aim it, which is the gesture between the two keystrokes —
  // and which lets go of the note.
  t.moveCursorPitch(2);
  assert.equal(t.state.selection.ref, null, "the arrow is meant to let go");
  await t.chordAtCursor();
  assert.equal(t.sent.length, 1,
               "Mod+Space sent nothing: " + JSON.stringify(t.said));
  const q = query(t.sent[0]);
  assert.equal(q.get("measure"), "0", "the notehead was aimed at the empty bar");
  assert.equal(q.get("index"), "3", "it belongs on the note that filled bar 1");
  assert.equal(q.get("step"), "5");
});

test("and the caret is left where the next note goes, not against the barline",
     async () => {
       const t = filled();
       t.moveCursorPitch(2);
       await t.chordAtCursor();
       assert.equal(t.state.selection.measure, 1);
       assert.equal(t.state.selection.at, 0,
                    "the caret came back over the barline into a full bar");
     });

// Mod+Space with the caret still on the line it wrote is somebody who has
// not moved it yet. Saying which note is in the way is the whole point of
// checking here rather than letting the server refuse.
test("a notehead already on that line is named, not merely refused", async () => {
  const t = filled();
  await t.chordAtCursor();
  assert.deepEqual(t.sent, []);
  assert.match(t.said[0], /already a A4 in that chord/);
});

// A caret at the front of a bar that follows an empty one is not standing
// after anything. It steps back one barline, not through the score.
test("nothing is behind a caret that follows an empty bar", async () => {
  const t = filled({ selection: { ref: null } });
  t.state.hits[0].elements = [];
  t.state.hits[0].full = false;
  await t.chordAtCursor();
  assert.deepEqual(t.sent, []);
  assert.equal(t.said[0], "Write a note first");
});

// Mid-bar, which is the case that always worked: unchanged.
test("mid-bar the caret names the note before it as it always did", async () => {
  const t = filled({ selection: { measure: 0, at: 2, step: 9, ref: 2 } });
  await t.chordAtCursor();
  const q = query(t.sent[0]);
  assert.equal(q.get("measure"), "0");
  assert.equal(q.get("index"), "1");
});

test("copy and cut aim at the note that filled the bar", () => {
  const t = filled();
  assert.equal(t.selectionTargets(), "0:0:3",
               "copying after a bar filled asked for element -1 of the next bar");
});

test("an accidental still has a notehead to go on", () => {
  const t = filled();
  assert.equal(t.hasSelection(), true);
  assert.equal(t.pickedUpIsPitched(), true,
               "the palette decided the note was a rest and armed the pen");
});

test("Backspace takes the note that filled the bar", async () => {
  const t = filled({ reply: { left: 0 } });
  await t.deleteSelected();
  assert.equal(t.sent.length, 1, "Backspace sent nothing: " + JSON.stringify(t.said));
  const q = query(t.sent[0]);
  assert.equal(q.get("measure"), "0");
  assert.equal(q.get("at"), "3");
  // And the caret steps back into the gap it left, which is in that bar.
  assert.equal(t.state.selection.measure, 0);
  assert.equal(t.state.selection.at, 3);
});

test("Backspace mid-bar is unchanged", async () => {
  const t = filled({ selection: { measure: 0, at: 3, ref: 3 }, reply: { left: 0 } });
  await t.deleteSelected();
  const q = query(t.sent[0]);
  assert.equal(q.get("measure"), "0");
  assert.equal(q.get("at"), "2");
  assert.equal(t.state.selection.at, 2);
});

// A grace note names the note it leans on, so the two lookups have to agree
// about it: selecting one must go on deleting that small note and not the
// big one it hangs off.
test("a grace note picked up is still the one deleted", async () => {
  const t = filled({
    selection: { measure: 0, at: 4, step: 6, ref: 4, grace: 0 },
    reply: { left: 0 },
  });
  await t.deleteSelected();
  const q = query(t.sent[0]);
  assert.equal(q.get("at"), "3");
  assert.equal(q.get("grace"), "0");
});
