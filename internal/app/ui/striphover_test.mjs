/*
 * striphover_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Finding the bar under the pointer, on a score with a great many of them.
 *
 * The band that lights up under the pointer was found and rebuilt on every
 * movement: querySelectorAll over the whole drawing to take the old one
 * away, insertBefore to put a new one back, and a walk down every bar of
 * every staff to decide where it went. On Beethoven's ninth the drawing is
 * nine hundred thousand shapes and there are a hundred and thirty-eight
 * thousand bars, so each of those was a walk of the whole score and a
 * re-layout of the whole drawing — several times for every frame of a
 * scroll. Scrolling with the pointer over the music was unusable.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^(?:const |function )" + name +
    "[\\s\\S]*?^(?:}|const stripBin = \\d+;)", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// indexStrips and stripAt, with the bucket width they share.
function load(strips) {
  const bin = src.match(/^const stripBin = (\d+);/m);
  assert.ok(bin, "app.js no longer says how wide a bucket is");
  const state = { strips, stripIndex: null };
  const scope = { state, Math };
  const body = "const stripBin = " + bin[1] + ";\n" +
    grab("indexStrips") + "\n" + grab("stripAt") +
    "\nreturn { indexStrips, stripAt };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  state.stripIndex = api.indexStrips(strips);
  return { ...api, state };
}

// A score shaped like the symphony: many staves, many bars, laid out across
// a page three hundred thousand spaces wide.
function orchestra(staves, bars) {
  const out = [];
  for (let p = 0; p < staves; p++) {
    for (let m = 0; m < bars; m++) {
      out.push({
        measure: m, part: p,
        x0: m * 12, x1: m * 12 + 11,
        top: p * 10, bottom: p * 10 + 8,
      });
    }
  }
  return out;
}

test("the bar under the pointer is the one the plain walk would find", () => {
  const strips = orchestra(8, 400);
  const t = load(strips);
  const plain = (x, y) =>
    strips.find((st) => x >= st.x0 && x <= st.x1 && y >= st.top && y <= st.bottom) || null;

  // Over bars, between them, off the top and off the right-hand end.
  for (const [x, y] of [[0, 0], [5, 3], [11, 3], [11.5, 3], [12, 3],
                        [1200, 44], [4799, 75], [4800, 75], [5, -1], [5, 1000]]) {
    const got = t.stripAt(x, y);
    const want = plain(x, y);
    assert.deepEqual(got, want,
      `at ${x},${y} the index found ${JSON.stringify(got)} and the walk ${JSON.stringify(want)}`);
  }
});

test("a bar that straddles two buckets is found from either side", () => {
  const bin = Number(src.match(/^const stripBin = (\d+);/m)[1]);
  // One bar sitting across a bucket boundary.
  const st = { measure: 1, part: 0, x0: bin - 2, x1: bin + 2, top: 0, bottom: 8 };
  const t = load([st]);
  assert.equal(t.stripAt(bin - 1, 4), st, "not found in the bucket it starts in");
  assert.equal(t.stripAt(bin + 1, 4), st, "not found in the bucket it ends in");
});

test("the index looks at a handful of bars, not all of them", () => {
  // The point of the thing: what it costs must not grow with the score.
  const strips = orchestra(10, 2000); // twenty thousand bars
  const t = load(strips);
  let looked = 0;
  const counted = strips.map((s) => new Proxy(s, {
    get(o, k) { if (k === "x0") looked++; return o[k]; },
  }));
  const t2 = load(counted);
  // Building the index reads them all once, which is the point of building
  // it. What must not grow with the score is answering.
  looked = 0;
  t2.stripAt(12000, 45);
  assert.ok(looked > 0, "nothing was looked at at all");
  assert.ok(looked < 200,
    `${looked} bars were compared for one position; the index is not being used`);
});

test("the hover band is moved rather than made again", () => {
  const fn = src.match(/^function setupStripHover\(\)[\s\S]*?^}/m)[0];
  assert.ok(!fn.includes(".querySelectorAll("),
    "the band is still found by searching the whole drawing");
  assert.ok((fn.match(/createElementNS/g) || []).length === 1,
    "the band is still built on every movement of the pointer");
  assert.ok(fn.includes("requestAnimationFrame"),
    "the pointer is answered more often than the screen is drawn");
});
