/*
 * setupchange_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Changing a staff's instrument in Score setup, over a score that already
 * has music in it.
 *
 * Everything else in that dialog goes straight to the score — adding a
 * staff, taking one away, moving one, taking a pair apart — and the star
 * beside the filename follows the score, so it appears as soon as any of
 * them is done. Changing an instrument did not: it changed the list in the
 * window and waited for a button, so the one thing somebody is most likely
 * to open the dialog for was the one change that could be made, closed on,
 * and lost, with nothing anywhere saying there was work to save.
 *
 * Reported as "changing something in score setup should show the '*' next
 * to the file to know we need to save it".
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

// A dialog over a score, with the requests it makes written down rather
// than sent.
function dialog({ parts, picked, answer, live }) {
  const sent = [];
  const scope = {
    state: { info: { parts } },
    setupChosen: parts.map((p, i) => ({
      id: i + 1, sound: p.sound, said: !!p.sound, staves: [i],
    })),
    setupGroups: [],
    setupApart: new Set(),
    setupNextId: parts.length + 1,
    said: [],
    $: () => null,
    ask: async () => answer,
    instrumentOptions: async () => [],
    busy: () => {},
    idle: () => {},
    status: (s) => scope.said.push(s),
    instrumentNamed: (s) => s.split(".").pop(),
    soundChosen: (s) => ({ sound: s }),
    grandSound: (s) => s === "keyboard.piano",
    setupLive: () => live,
    setupOrdered: () => scope.setupChosen,
    drawSetupChosen: () => {},
    setupReload: async () => scope.said.push("reloaded"),
    setupPicked: () => picked.map((i) => scope.setupChosen[i]),
    fetch: async (url) => {
      sent.push(url);
      return { ok: true, json: async () => ({}), text: async () => "" };
    },
  };
  const run = new Function("scope",
    "with (scope) { " + grab("setupChange") + "\nreturn setupChange; }")(scope);
  return { run, sent, scope };
}

test("a staff changed over a live score is changed in the score", async () => {
  const d = dialog({
    parts: [{ sound: "keyboard.piano" }, { sound: "keyboard.piano" }],
    picked: [1], answer: "wind.flutes.flute", live: true,
  });
  await d.run();
  assert.deepEqual(d.sent, ["/api/staff?part=1&sound=wind.flutes.flute"]);
  // Which is what makes the star appear: the flag belongs to the score and
  // the page reads it back.
  assert.ok(d.scope.said.includes("reloaded"),
    "the score was changed and the page was not read back");
});

test("and it says which staff it was, in the singular", async () => {
  const d = dialog({
    parts: [{ sound: "keyboard.piano" }, { sound: "keyboard.piano" }],
    picked: [1], answer: "wind.flutes.flute", live: true,
  });
  await d.run();
  assert.ok(d.scope.said.some((s) => /^Staff 2 now plays flute$/.test(s)),
    "said: " + JSON.stringify(d.scope.said));
});

test("two staves made one piano are braced in the score itself", async () => {
  const d = dialog({
    parts: [{ sound: "" }, { sound: "" }],
    picked: [0, 1], answer: "keyboard.piano", live: true,
  });
  await d.run();
  assert.deepEqual(d.sent, [
    "/api/staff?part=0&sound=keyboard.piano",
    "/api/staff?part=1&sound=keyboard.piano",
    "/api/group?first=0&last=1&symbol=brace&barlines=1",
  ]);
});

test("two staves of something written on one are left two staves", async () => {
  const d = dialog({
    parts: [{ sound: "" }, { sound: "" }],
    picked: [0, 1], answer: "wind.flutes.flute", live: true,
  });
  await d.run();
  assert.deepEqual(d.sent, [
    "/api/staff?part=0&sound=wind.flutes.flute",
    "/api/staff?part=1&sound=wind.flutes.flute",
  ], "two flutes were braced into one player");
});

test("a staff taken apart by hand and then braced is together again", async () => {
  const d = dialog({
    parts: [{ sound: "keyboard.piano" }, { sound: "keyboard.piano" }],
    picked: [0, 1], answer: "keyboard.piano", live: true,
  });
  d.scope.setupApart.add(1);
  await d.run();
  assert.equal(d.scope.setupApart.has(1), false,
    "the score now braces them and the dialog still holds them apart");
});

test("a score with no staves yet still waits for the button", async () => {
  // Nothing to change in the score, because there is no score: the list is
  // what is being built, and it goes across when it is applied.
  const d = dialog({
    parts: [{ sound: "" }, { sound: "" }],
    picked: [0], answer: "wind.flutes.flute", live: false,
  });
  await d.run();
  assert.deepEqual(d.sent, []);
  assert.equal(d.scope.setupChosen[0].sound, "wind.flutes.flute");
  assert.equal(d.scope.setupChosen[0].said, true);
});

test("nothing is sent when the question is answered with nothing", async () => {
  const d = dialog({
    parts: [{ sound: "keyboard.piano" }], picked: [0],
    answer: null, live: true,
  });
  await d.run();
  assert.deepEqual(d.sent, []);
});
