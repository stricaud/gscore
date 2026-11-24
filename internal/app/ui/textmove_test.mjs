/*
 * textmove_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Words on a note, clicked, then an arrow: the words move, and stay picked
 * up so the next press moves them on.
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

function load(state) {
  const sent = [];
  const fetch = async (url) => { sent.push(url); return { ok: true, text: async () => "" }; };
  const move = new Function("state", "fetch", "refTarget", "refreshScore",
    "markSelection", "status", "withVoice",
    grab("moveTextMark") + "; return moveTextMark;")(
    state, fetch, () => "0:27:0", async () => {}, () => {}, () => {}, () => {});
  return { move, sent };
}

test("an arrow moves the words that were clicked", async () => {
  const lh = { ref: 7, index: 1, text: "l.h." };
  const state = { selection: { part: 0, measure: 27, at: 1, textMark: lh },
                  multi: [], textMarks: [{ ...lh, x: 3 }] };
  const { move, sent } = load(state);
  await move(-0.25, 0);
  assert.equal(sent.length, 1);
  const q = new URL("http://x" + sent[0]).searchParams;
  assert.equal(q.get("index"), "1");
  assert.equal(q.get("dx"), "-0.25");
  assert.equal(q.get("dy"), "0");
  assert.equal(q.get("sel"), "0:27:0");
  assert.equal(state.selection.textMark.x, 3, "picked up again where it is now");
});

test("with no words picked up nothing is sent", async () => {
  const { move, sent } = load({ selection: { part: 0, measure: 0, at: 1 }, multi: [] });
  await move(1, 0);
  assert.equal(sent.length, 0);
});

test("words picked up on the second voice keep its voice", () => {
  const state = { multi: [] };
  const pick = new Function("state", "refTarget", "readSel", "markSelection", "status", "t",
    grab("selectTextMark") + "; return selectTextMark;")(
    state, () => "0:16:1/0", new Function(grab("readSel") + "; return readSel;")(),
    () => {}, () => {}, (s) => s);
  pick({ ref: 5, part: 0, measure: 16, index: 0, text: "Page 4" });
  assert.equal(state.selection.voice, 1);
  assert.equal(state.selection.at, 1);
});
