/*
 * marquee_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The rubber band: a box dragged with the mouse picks up everything under
 * it, across as many staves as it crosses, and nothing outside it.
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

// Two staves, two bars each, three notes a bar; the upper staff's top line
// at y=10, the lower at y=30. Notes climb a step at a time.
function score() {
  const hits = [];
  for (let p = 0; p < 2; p++) {
    for (let m = 0; m < 2; m++) {
      const x0 = 10 + m * 20;
      hits.push({
        part: p, measure: m, x0, x1: x0 + 20, staffTop: 10 + p * 20,
        elements: [0, 1, 2].map((i) => ({
          index: i, voice: 0, ref: `${p}-${m}-${i}`, kind: "note",
          x: x0 + 3 + i * 5, width: 1.2, y: 10 + p * 20 + 3 - i * 0.5,
        })),
      });
    }
  }
  return hits;
}

const inBox = (hits) => new Function("state",
  grab("elementsInBox") + "; return elementsInBox;")({ hits });

test("a box over one staff picks up only what is inside it", () => {
  const box = inBox(score());
  // Across the second and third notes of bar 1 and the first of bar 2.
  const got = box(17, 9, 33.5, 15).map((m) => m.ref);
  assert.deepEqual(got, ["0-0-1", "0-0-2", "0-1-0"]);
});

test("a box across both staves picks up both", () => {
  const box = inBox(score());
  const got = box(12, 5, 15, 40).map((m) => m.ref);
  assert.deepEqual(got, ["0-0-0", "1-0-0"]);
});

test("a box clear of every note picks up nothing", () => {
  const box = inBox(score());
  assert.deepEqual(box(12, 18, 50, 24), []);
});

test("a box reaching just to the edge of a notehead takes it", () => {
  const box = inBox(score());
  // The first note's head is centred at y=13; its lower edge is at 13.5.
  assert.deepEqual(box(12, 13.4, 15, 16).map((m) => m.ref), ["0-0-0"]);
});

test("a plain drag selects; ctrl and command still pan", () => {
  const fn = grab("setupMarquee");
  assert.match(fn, /e\.ctrlKey \|\| e\.metaKey/);
  assert.doesNotMatch(fn, /if \(!e\.shiftKey\) return/);
});

const konst = (name) => {
  const m = src.match(new RegExp("^const " + name + " = ([\\s\\S]*?);$", "m"));
  if (!m) throw new Error("app.js has no const " + name);
  return m[0];
};

const boxScope = (state) => new Function("state", "elementByRef",
  [grab("arcPoints"), konst("boxKinds"), grab("marksInBox")].join("\n") +
  "; return marksInBox;")(state, () => null);

// A box picks up every kind of marking it touches, not only the notes.
test("a box picks up the markings it touches and none it does not", () => {
  const LONG = { kind: "slur", from: 3, to: 9, step: 0, x1: 20, y1: 8, cx: 80, cy: 6, x2: 140, y2: 7 };
  const find = boxScope({
    arcs: [LONG, { ...LONG, x1: 0, x2: 10, cx: 5 }], // the same slur's other half
    dynamics: [{ x: 70, y: 20, width: 2, height: 1, part: 0, measure: 3 }],
    hairpins: [{ x: 200, y: 20, width: 8, height: 1 }],
    tempos: [{ x: 72, y: 2, width: 4, height: 1, measure: 3 }],
    graces: [{ x: 74, y: 14, width: 0.8, part: 0, measure: 3, index: 0, ord: 0 }],
  });
  const got = find(65, 0, 80, 25, null).map((m) => m.kind).sort();
  assert.deepEqual(got, ["arc", "dynamic", "grace", "tempo"]);
  // The slur's other half is not a second slur to take away.
  assert.equal(find(0, 0, 200, 25, null).filter((m) => m.kind === "arc").length, 1);
});

// Taken away in an order where nothing is looked for after what it was
// found by has moved: by-number marks, then bar marks, then grace notes
// last of a group first.
test("the things in a box go in an order that keeps the rest findable", () => {
  const order = new Function(konst("boxedRank") + ";" + grab("boxedOrder") +
    "; return boxedOrder;")();
  const g = (ord) => ({ kind: "grace", item: { part: 0, measure: 1, index: 0, ord } });
  const got = order([g(0), { kind: "tempo", item: {} }, g(1),
    { kind: "arc", item: {} }, { kind: "dynamic", item: {} }]);
  assert.deepEqual(got.map((m) => m.kind + (m.item.ord ?? "")),
    ["arc", "dynamic", "tempo", "grace1", "grace0"]);
});

// One Delete, one Undo: the removals are bracketed as a script's are.
test("deleting a box is one undo", async () => {
  const asked = [];
  const removed = [];
  const state = {
    multi: [{ part: 0, measure: 0, index: 2, ref: 5 }],
    selection: { boxed: [{ kind: "dynamic", item: { n: 1 } }, { kind: "arc", item: { n: 2 } }] },
  };
  const scope = {
    state, pickedSymbol: null, markPickedSymbol() {}, status() {},
    clearSelection() { state.selection = null; state.multi = []; },
    fetch: async (u) => { asked.push(u); return { ok: true }; },
    selectBoxed: (m) => { state.selection = { picked: m.kind }; return true; },
    deleteSelected: async () => removed.push(state.multi.length
      ? `notes:${state.multi.length}` : state.selection.picked),
    boxedOrder: (m) => m,
  };
  const run = new Function(...Object.keys(scope),
    grab("deleteBoxed") + "; return deleteBoxed;")(...Object.values(scope));
  await run();
  assert.deepEqual(removed, ["dynamic", "arc", "notes:1"]);
  assert.equal(asked[0], "/api/script/begin");
  assert.match(asked[asked.length - 1], /^\/api\/script\/end/);
});

// A box round a grace note and the note it leans on, then S: the grace note
// is slurred into its note. It was read as a phrase of one note and did
// nothing. A box round a passage that holds a grace note stays a phrase.
test("S on a boxed grace note and its note slurs the grace note", () => {
  const hits = [{ part: 0, measure: 29, elements: [
    { ref: 175, index: 0, voice: 0 }, { ref: 176, index: 1, voice: 0 }] }];
  const grace = { kind: "grace", item: { part: 0, measure: 29, index: 0, ord: 0, ref: 175 } };
  const run = (multi) => new Function("state", "selOf",
    grab("boxedGraceCarriers") + "; return boxedGraceCarriers();")(
    { hits, multi, selection: { boxed: [grace, { kind: "dynamic", item: {} }] } },
    (p, m, i, v) => `${p}:${m}:${v ? v + "/" : ""}${i}`);
  assert.deepEqual(run([{ ref: 175 }]), ["0:29:0"]);
  assert.deepEqual(run([]), ["0:29:0"]);
  assert.equal(run([{ ref: 175 }, { ref: 176 }]), null);
});
