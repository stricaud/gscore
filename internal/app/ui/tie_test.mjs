/*
 * tie_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Tying from one note.
 *
 * A tie means "hold this note into the next one", so the note it runs to is
 * the one that follows and nobody should have to say which. The server has
 * worked that out for a while; the editor went on refusing before it asked,
 * so T on one note answered "a tie joins two notes" and did nothing.
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

// The editor's own function, with the request written down rather than sent
// and the status line kept for reading back.
function load(state) {
  const sent = [];
  const said = [];
  const scope = {
    state,
    async mark(endpoint, params, what) {
      sent.push({ endpoint, params });
      said.push(what);
    },
    status: (s) => said.push(s),
  };
  const body = grab("setTie") + "\nreturn { setTie };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { sent, said, ...api };
}

test("one note selected ties it to the next", async () => {
  const { sent, setTie } = load({ multi: [], selection: { ref: 3, head: null } });
  await setTie("on", false);
  assert.equal(sent.length, 1, "the editor refused instead of asking the server");
  assert.equal(sent[0].endpoint, "tie");
  assert.equal(sent[0].params.tie, "on");
});

test("a note picked up by clicking is the same case", async () => {
  const { sent, setTie } = load({ multi: [{ index: 2 }], selection: { ref: 3, head: null } });
  await setTie("on", false);
  assert.equal(sent.length, 1);
});

test("a range of notes still ties neighbour to neighbour", async () => {
  const { sent, setTie } =
    load({ multi: [{ index: 0 }, { index: 1 }, { index: 2 }], selection: { ref: 1, head: null } });
  await setTie("on", false);
  assert.equal(sent.length, 1);
  assert.equal(sent[0].params.tie, "on");
});

test("one notehead of a chord is tied by itself", async () => {
  const { sent, setTie } = load({ multi: [], selection: { ref: 3, head: 6 } });
  await setTie("on", true);
  assert.equal(sent[0].params.step, "6");
});

test("taking a tie off never needed two either", async () => {
  const { sent, setTie } = load({ multi: [], selection: { ref: 3, head: null } });
  await setTie("none", false);
  assert.equal(sent[0].params.tie, "none");
});
