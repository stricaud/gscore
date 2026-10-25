/*
 * beam_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the b key means, which depends on how much is picked up.
 *
 * A group of notes is a thing to join. One note is not a group, and asking
 * to beam it was an error — so there the key breaks the beam before it
 * instead, which is the other half of the same job and the thing somebody
 * standing on the fifth of eight notes is trying to do.
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

// The editor's own function, with the request to the server written down
// rather than sent.
function load(multi) {
  const sent = [];
  const scope = {
    state: { multi },
    async mark(endpoint, params, said) {
      sent.push({ endpoint, params, said });
    },
  };
  const body = grab("beamOrBreak") + "\nreturn { beamOrBreak };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { sent, ...api };
}

test("a range of notes is beamed together", async () => {
  const { sent, beamOrBreak } = load([{ index: 0 }, { index: 1 }, { index: 2 }]);
  await beamOrBreak();
  assert.equal(sent.length, 1);
  assert.equal(sent[0].endpoint, "beam");
  assert.equal(sent[0].params.beam, "join");
});

test("one note breaks the beam before it, and stays on the one after", async () => {
  const { sent, beamOrBreak } = load([]);
  await beamOrBreak();
  assert.equal(sent.length, 1);
  assert.equal(sent[0].params.beam, "start",
               "the note was taken off the beam instead of opening the next group");
  assert.notEqual(sent[0].params.beam, "break");
});

test("a single note picked up by clicking is the same case", async () => {
  // A click fills multi with the one note, which is not a group either.
  const { sent, beamOrBreak } = load([{ index: 4 }]);
  await beamOrBreak();
  assert.equal(sent[0].params.beam, "start");
});
