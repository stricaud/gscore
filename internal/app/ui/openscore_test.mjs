/*
 * openscore_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What File > Open says when it cannot open the file.
 *
 * The application asks the system for a dialog and the server opens whatever
 * is chosen, so one answer covers both halves. Three of them can come back
 * and they are not the same thing: the file opened, there is no panel here,
 * or the file was chosen and could not be read.
 *
 * The last two both fell through to the browser's own file input — and in
 * the application window that does nothing at all, because there is no file
 * panel behind it. Choosing a Finale file in a build that could not read one
 * left the panel closed, no score, and "Choose a score…" still on the status
 * line. The reason was in the answer, and the answer was thrown away.
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

// The editor, with a server that answers once and a note of what the person
// was told and whether the browser's picker was reached for.
function load(answer) {
  const said = [];
  let clicked = false;
  const scope = {
    state: { info: { local: true } },
    status: (s) => said.push(s),
    $: () => ({ click() { clicked = true; } }),
    async fetch() {
      if (answer instanceof Error) throw answer;
      return answer;
    },
    async reopen() {},
    // Gilbert, who comes up over the window while a score is being read.
    // Nothing here is slow enough to see him, but the function still calls
    // for him and a scope without him throws before it says anything.
    busy() {},
    idle() {},
  };
  const body = grab("openScore") + "\nreturn { openScore };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, said, clicked: () => clicked };
}

const answer = (status, body, json) => ({
  ok: status === 200,
  status,
  statusText: "",
  async text() { return body; },
  async json() { return json; },
});

test("a score that opens is announced", async () => {
  const t = load(answer(200, "", { filename: "Sanctus - Rossini.musx" }));
  await t.openScore();
  assert.ok(t.said.some((s) => s.includes("Sanctus")), t.said.join(" | "));
  assert.equal(t.clicked(), false);
});

test("a file that could not be read says why", async () => {
  const t = load(answer(400, "nothing here reads .musx files"));
  await t.openScore();

  const last = t.said[t.said.length - 1];
  assert.ok(last.includes(".musx"),
            "the person was left with " + JSON.stringify(last) +
            ", and the reason was in the answer");
  assert.equal(t.clicked(), false,
               "the browser's picker was opened instead, and in the " +
               "application window it does nothing");
});

test("no panel here is still answered with the browser's own", async () => {
  // 501 is the server saying it has no dialog to show — served over a
  // network, or a machine with neither zenity nor kdialog. That one is
  // answered by the file input, which is what it is there for.
  const t = load(answer(501, "no dialog when gscore is served"));
  await t.openScore();
  assert.equal(t.clicked(), true);
});

test("a server that does not answer at all falls back too", async () => {
  const t = load(new Error("network"));
  await t.openScore();
  assert.equal(t.clicked(), true);
});

test("cancelling leaves the score alone and says nothing more", async () => {
  const t = load(answer(200, "", { cancelled: true }));
  await t.openScore();
  assert.equal(t.said[t.said.length - 1], "Ready");
  assert.equal(t.clicked(), false);
});
