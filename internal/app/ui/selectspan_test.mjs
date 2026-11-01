/*
 * selectspan_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Picking up music without dragging over it.
 *
 * Every command that works on a stretch of music needs a stretch picked
 * up, and until now the only way to pick one up was to drag the mouse over
 * it. On a long piece that is the wrong way round: to take bars 40 to 96
 * you have to find both ends and hold the button down over everything in
 * between.
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
  const m = src.match(new RegExp("^const " + name + " =([\\s\\S]*?);$", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return new Function("return (" + m[1] + ")")();
};

// a score of parts × bars, each bar holding that many notes.
function score({ parts = 2, bars = 4, each = 3 } = {}) {
  const hits = [];
  for (let p = 0; p < parts; p++) {
    for (let m = 0; m < bars; m++) {
      hits.push({
        part: p, measure: m,
        elements: new Array(each).fill(0).map((_, i) => ({ index: i, ref: p * 100 + m * 10 + i })),
      });
    }
  }
  return hits;
}

function picking(hits, selection) {
  const state = {
    hits, selection, multi: [], anchor: null, focus: null,
    info: { parts: [{ name: "Flute" }, { name: "Oboe" }] },
  };
  const said = [];
  const scope = {
    state,
    status: (s) => said.push(s),
    markSelection: () => {},
    scrollSelectionIntoView: () => {},
    selectBudget: konst("selectBudget"),
    orderedSelection: () => state.multi.slice().sort(
      (a, b) => a.part - b.part || a.measure - b.measure || a.index - b.index),
  };
  const body = grab("selectSpan") + "\n" + grab("selectAll") +
    "\nreturn { selectSpan, selectAll };";
  const f = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { state, said, ...f };
}

test("a stretch of bars is picked up whole", () => {
  const p = picking(score({ bars: 8, each: 3 }), { part: 0, measure: 0 });
  assert.equal(p.selectSpan(0, 2, 4, "bars 3–5"), true);
  assert.equal(p.state.multi.length, 9, "three bars of three notes");
  assert.deepEqual([...new Set(p.state.multi.map((m) => m.measure))], [2, 3, 4]);
});

// One staff, because that is what a selection is here: a slur or a
// crescendo over three staves at once is not a thing anybody can draw.
test("it stays on the staff it was asked for", () => {
  const p = picking(score({ parts: 3, bars: 4 }), { part: 1, measure: 0 });
  p.selectSpan(1, 0, 3, "all of it");
  assert.ok(p.state.multi.every((m) => m.part === 1),
            "it reached onto another staff");
});

test("picking up the whole staff takes every bar of it", () => {
  const p = picking(score({ parts: 2, bars: 6, each: 2 }), { part: 1, measure: 3 });
  p.selectAll();
  assert.equal(p.state.multi.length, 12, "six bars of two notes on one staff");
  assert.ok(p.state.multi.every((m) => m.part === 1), "it took the wrong staff");
  assert.match(p.said.join(" "), /Oboe/, "it does not say which staff it took");
});

// The ends are remembered the way a drag remembers them, so shift with an
// arrow carries on from the end rather than starting again.
test("both ends are remembered", () => {
  const p = picking(score({ bars: 4, each: 2 }), { part: 0, measure: 0 });
  p.selectSpan(0, 1, 2, "two bars");
  assert.deepEqual(p.state.anchor, { part: 0, measure: 1, index: 0 });
  assert.deepEqual(p.state.focus, { part: 0, measure: 2, index: 1 });
  assert.equal(p.state.selection.at, 1, "the caret is not at the first of them");
});

test("asking for bars with nothing in them picks up nothing", () => {
  const p = picking(score({ bars: 3 }), { part: 0, measure: 0 });
  assert.equal(p.selectSpan(0, 7, 9, "bars 8–10"), false);
  assert.deepEqual(p.state.multi, []);
  assert.match(p.said.join(" "), /nothing written/);
});

test("asking for a staff that is not there picks up nothing", () => {
  const p = picking(score({ parts: 1 }), { part: 0, measure: 0 });
  assert.equal(p.selectSpan(4, 0, 3, "all of it"), false);
  assert.match(p.said.join(" "), /nothing on that staff/);
});

// Every command that works on a selection names each note in the query it
// sends, so a selection has a size past which it is not a selection.
test("more than the budget is refused, and says so", () => {
  const budget = konst("selectBudget");
  const p = picking(score({ parts: 1, bars: budget, each: 2 }), { part: 0, measure: 0 });
  assert.equal(p.selectSpan(0, 0, budget, "all of it"), false);
  assert.deepEqual(p.state.multi, [], "a selection past the budget was kept anyway");
  assert.match(p.said.join(" "), new RegExp(String(budget)));
});

test("the menu and the key reach it", () => {
  assert.match(src, /case "select-all":/, "nothing handles the menu entry");
  assert.match(src, /case "select-region":/, "nothing handles the bars dialog");
  assert.match(src, /id: "select-all", menu: "select-all", keys: "Mod\+a"/,
               "the whole staff is not on a key");
});
