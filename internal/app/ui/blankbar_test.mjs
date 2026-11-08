/*
 * blankbar_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The blank bar past the end of the music, and what may be written on it.
 *
 * It is drawn so that there is somewhere to click to carry on writing, and
 * the cursor is standing in it the moment the last note of the last bar is
 * written. It is not a bar of the piece: the server knows nothing about it
 * and answers "there is no such measure" to anything addressed to it.
 *
 * Asked for a rehearsal mark there, the editor worked out the next letter,
 * offered it, took the answer, posted it and put the server's refusal in
 * the status line — which from the other side of the screen is a letter
 * asked for and silently not drawn. Reported as "I click Insert, Rehearsal
 * Mark, A — next in the sequence, and nothing gets written".
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

function grab(name) {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
}

// The editor's marker dialog, run against a made-up score: four bars of
// music and the blank bar after them, which is what the engraver reports.
function run(measure, { marks = {} } = {}) {
  const asked = [];
  const posted = [];
  const said = [];
  const hits = [
    { measure: 0 }, { measure: 1 }, { measure: 2 }, { measure: 3 },
    { measure: 4, ahead: true },
  ];
  const scope = {
    state: {
      hits,
      selection: { part: 0, measure, at: 0 },
      info: { markerStyle: "letters", marks, sequenced: [] },
    },
    status: (s) => said.push(s),
    ask: async (q) => { asked.push(q); return "auto"; },
    barNumberOf: (i) => i + 1,
    refreshScore: async () => {},
    fetch: async (url) => { posted.push(url); return { ok: true }; },
    URLSearchParams,
  };
  const body = [grab("measureCount"), grab("blankBarPastTheEnd"),
                grab("nextMarkerName"), grab("rehearsalLetter"),
                grab("markerDialog")].join("\n") +
    "\nreturn { markerDialog };";
  const api = new Function(...Object.keys(scope), body)(
    ...Object.values(scope));
  return api.markerDialog().then(() => ({ asked, posted, said }));
}

test("a mark is written on a bar of the music", async () => {
  const { asked, posted, said } = await run(3);
  assert.equal(asked.length, 1, "it did not ask");
  assert.equal(posted.length, 1, "nothing was sent to the server");
  assert.ok(posted[0].includes("measure=3"), posted[0]);
  assert.ok(posted[0].includes("auto=1"), posted[0]);
  assert.deepEqual(said, ["Mark A"]);
});

test("the blank bar past the end is not offered a letter", async () => {
  const { asked, posted, said } = await run(4);
  assert.deepEqual(asked, [],
    "it asked for a mark on a bar that is not there");
  assert.deepEqual(posted, [],
    "it posted a mark the server can only refuse");
  assert.equal(said.length, 1);
  assert.ok(/blank bar past the end/.test(said[0]),
    "it did not say why nothing was written: " + said[0]);
  assert.ok(/click a bar/i.test(said[0]),
    "it did not say what to do instead: " + said[0]);
});

test("a bar the drawing says nothing about is left to the server", async () => {
  // Nothing drawn yet, so nothing is known to be past the end. Refusing
  // here would refuse bars that are really there.
  const scope = { state: { hits: [] } };
  const body = [grab("measureCount"), grab("blankBarPastTheEnd")].join("\n") +
    "\nreturn { blankBarPastTheEnd };";
  const { blankBarPastTheEnd } = new Function(...Object.keys(scope), body)(
    ...Object.values(scope));
  assert.equal(blankBarPastTheEnd(0), false);
  assert.equal(blankBarPastTheEnd(99), false);
});
