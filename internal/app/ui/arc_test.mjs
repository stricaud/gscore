/*
 * arc_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Picking up a tie or a slur.
 *
 * A curve belongs to the two notes at its ends and to none of the ones it
 * arcs over, so there was nothing on the page to click: the only way to take
 * a slur off was to select the notes under it again and ask for no slur.
 *
 * Clicking the curve picks up the curve. Backspace takes it away and leaves
 * the music alone — and takes away only that one, which is the whole
 * difficulty: a note in the middle of a phrase closes one slur and opens the
 * next, and a note held through three bars stops one tie and starts another
 * on the same notehead.
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
const konst = (name) => {
  const m = src.match(new RegExp("^const " + name + " = .*?;$", "m"));
  if (!m) throw new Error("app.js has no const " + name);
  return m[0];
};

// A tie between two notes a bar apart, arcing above them, and a slur over
// four notes climbing from the staff. Both as the engraver reports them.
const TIE = {
  part: 0, measure: 0, from: 1, to: 2, step: 2, kind: "tie",
  x1: 12, y1: 16, cx: 14, cy: 14.6, x2: 16, y2: 16,
};
const SLUR = {
  part: 0, measure: 0, from: 3, to: 6, step: 0, kind: "slur",
  x1: 20, y1: 14, cx: 26, cy: 7, x2: 32, y2: 12,
};
const HITS = [{
  part: 0, measure: 0, staffTop: 12,
  elements: [
    { ref: 1, index: 0, x: 11.4, y: 16, y2: 16, width: 1.2, kind: "note", steps: [2] },
    { ref: 2, index: 1, x: 15.4, y: 16, y2: 16, width: 1.2, kind: "note", steps: [2] },
    { ref: 3, index: 2, x: 19.4, y: 14, y2: 14, width: 1.2, kind: "note", steps: [6] },
    { ref: 6, index: 3, x: 31.4, y: 12, y2: 12, width: 1.2, kind: "note", steps: [10] },
  ],
}];

// at is a point on a quadratic, so a test can aim at the curve itself.
function at(a, t) {
  const u = 1 - t;
  return {
    x: u * u * a.x1 + 2 * u * t * a.cx + t * t * a.x2,
    y: u * u * a.y1 + 2 * u * t * a.cy + t * t * a.y2,
  };
}

function load(names, extra = {}) {
  const said = [];
  const asked = [];
  const state = {
    hits: HITS, arcs: [TIE, SLUR], multi: [], selection: null,
    anchor: null, focus: null, zoom: 8, ...(extra.state || {}),
  };
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection() {},
    refreshScore: async () => {},
    URLSearchParams,
    fetch: async (url) => {
      asked.push(url);
      return { ok: true, text: async () => "", json: async () => ({}) };
    },
  };
  const body = [konst("arcReach"), ...names.map(grab)].join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, state, said, asked };
}

test("a click on the middle of a curve finds it", () => {
  const t = load(["awayFromArc", "arcAt"]);
  for (const a of [TIE, SLUR]) {
    const p = at(a, 0.5);
    assert.equal(t.arcAt(p.x, p.y)?.kind, a.kind, `the ${a.kind}'s own middle`);
  }
});

// The ends sit on the two noteheads the curve joins, and a click there is a
// click on the note.
test("a click on the note a curve starts from is not a click on the curve", () => {
  const t = load(["awayFromArc", "arcAt"]);
  for (const a of [TIE, SLUR]) {
    assert.equal(t.arcAt(a.x1, a.y1), null, `the ${a.kind}'s first note`);
    assert.equal(t.arcAt(a.x2, a.y2), null, `the ${a.kind}'s last note`);
  }
});

test("a click away from every curve finds none of them", () => {
  const t = load(["awayFromArc", "arcAt"]);
  assert.equal(t.arcAt(14, 20), null, "below the staff, under the tie");
  assert.equal(t.arcAt(26, 16), null, "under the slur, among the notes");
  assert.equal(t.arcAt(60, 10), null, "off the end of the music");
});

// Two curves on one page: the nearer one answers.
test("the nearer of two curves is the one picked up", () => {
  const t = load(["awayFromArc", "arcAt"]);
  const p = at(SLUR, 0.5);
  assert.equal(t.arcAt(p.x, p.y)?.kind, "slur");
  const q = at(TIE, 0.5);
  assert.equal(t.arcAt(q.x, q.y)?.kind, "tie");
});

test("the box drawn round a curve holds all of it", () => {
  const t = load(["arcBox"]);
  const b = t.arcBox(SLUR);
  for (let i = 0; i <= 10; i++) {
    const p = at(SLUR, i / 10);
    assert.ok(p.x >= b.x - 1e-9 && p.x <= b.x + b.width + 1e-9 &&
              p.y >= b.y - 1e-9 && p.y <= b.y + b.height + 1e-9,
              `the curve leaves the box at t=${i / 10}`);
  }
  // And nothing much more: a box round a slur must not be a box round the
  // music under it.
  assert.ok(b.y + b.height <= Math.max(SLUR.y1, SLUR.y2) + 1e-9,
            "the box reaches below both ends of the curve");
});

test("picking one up selects the curve, not the notes at its ends", () => {
  const t = load(["selectArc"]);
  t.selectArc(SLUR);
  assert.equal(t.state.selection.arc, SLUR);
  assert.equal(t.state.selection.ref, 3, "the note it starts on, for the server");
  assert.deepEqual(t.state.multi, [], "no passage is picked up with it");
  assert.ok(t.said.join(" ").includes("Backspace"),
            "nothing said it can be taken away: " + t.said.join(" | "));
});

test("Backspace asks for that curve alone", async () => {
  const t = load(["refTarget", "deleteSelected"],
    { state: { selection: { part: 0, measure: 0, ref: 1, arc: TIE } } });
  await t.deleteSelected();

  assert.equal(t.asked.length, 1, "asked for " + t.asked.join(" | "));
  const url = new URL(t.asked[0], "http://x");
  assert.equal(url.pathname, "/api/tie");
  assert.equal(url.searchParams.get("tie"), "none");
  assert.equal(url.searchParams.get("ends"), "1",
               "without this the ties either side of it go too");
  assert.equal(url.searchParams.get("step"), "2",
               "a chord is tied notehead by notehead");
  assert.equal(url.searchParams.get("sel"), "0:0:0,0:0:1");
  assert.ok(t.said.join(" ").includes("Tie removed"));
});

test("Backspace on a slur asks the slur for it, with no notehead", async () => {
  const t = load(["refTarget", "deleteSelected"],
    { state: { selection: { part: 0, measure: 0, ref: 3, arc: SLUR } } });
  await t.deleteSelected();

  const url = new URL(t.asked[0], "http://x");
  assert.equal(url.pathname, "/api/slur");
  assert.equal(url.searchParams.get("slur"), "none");
  assert.equal(url.searchParams.get("ends"), "1");
  assert.equal(url.searchParams.get("step"), null,
               "a slur is not written notehead by notehead");
  assert.equal(url.searchParams.get("sel"), "0:0:2,0:0:3");
});

// A curve whose notes have been edited away since it was drawn.
test("a curve that is no longer there is said so rather than half removed", async () => {
  const t = load(["refTarget", "deleteSelected"], {
    state: {
      arcs: [], hits: [],
      selection: { part: 0, measure: 0, ref: 1, arc: TIE },
    },
  });
  await t.deleteSelected();
  assert.equal(t.asked.length, 0, "it asked anyway: " + t.asked.join(" | "));
  assert.ok(t.said.join(" ").includes("Could not find"),
            "nothing said why: " + t.said.join(" | "));
});
