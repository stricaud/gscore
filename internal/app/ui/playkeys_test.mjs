/*
 * playkeys_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The piano along the bottom of the window while the music plays: eighty-
 * eight keys, and the ones sounding go down in the colour of their staff.
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

function load() {
  const keys = new Map();
  const box = {
    innerHTML: "",
    querySelector(sel) {
      const m = Number(sel.match(/data-midi="(\d+)"/)[1]);
      if (!keys.has(m)) {
        const cls = new Set();
        keys.set(m, { classList: { add: (c) => cls.add(c), remove: (c) => cls.delete(c) }, cls });
      }
      return keys.get(m);
    },
  };
  const playKeys = { on: true, lit: new Map() };
  const fns = new Function("$", "playKeys",
    "const PLAYKEYS_LOW = 21, PLAYKEYS_HIGH = 108;" +
    grab("isBlackKey") + grab("renderPlayKeys") + grab("lightPlayKeys") +
    "; return { renderPlayKeys, lightPlayKeys, isBlackKey };")(() => box, playKeys);
  return { ...fns, box, keys, playKeys };
}

test("eighty-eight keys, fifty-two white and thirty-six black", () => {
  const { renderPlayKeys, box } = load();
  renderPlayKeys();
  assert.equal((box.innerHTML.match(/class="pk-white"/g) || []).length, 52);
  assert.equal((box.innerHTML.match(/class="pk-black"/g) || []).length, 36);
  assert.match(box.innerHTML, />C4</, "middle C is named");
});

test("a key goes down in its staff's colour and comes up again", () => {
  const { lightPlayKeys, keys } = load();
  lightPlayKeys(new Map([[60, 0], [43, 1]]));
  assert.ok(keys.get(60).cls.has("pk-on-0"));
  assert.ok(keys.get(43).cls.has("pk-on-1"));
  lightPlayKeys(new Map([[60, 0]]));
  assert.ok(keys.get(60).cls.has("pk-on-0"), "still held");
  assert.equal(keys.get(43).cls.size, 0, "let go");
});

test("black keys are the sharps", () => {
  const { isBlackKey } = load();
  assert.deepEqual([60, 61, 62, 63, 64, 65, 66].map(isBlackKey),
    [false, true, false, true, false, false, true]);
});

test("a pause leaves the keys down and a stop lifts them", () => {
  const pause = grab("pause");
  const stop = grab("stop");
  const silence = grab("silence");
  assert.doesNotMatch(silence, /lightPlayKeys/, "silence is what a pause does too");
  assert.doesNotMatch(pause, /lightPlayKeys/);
  assert.match(stop, /lightPlayKeys\(new Map\(\)\)/);
});
