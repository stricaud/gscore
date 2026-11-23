/*
 * review_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Marking bars while a score is being checked: which bars a mark goes on,
 * and the tint each marked bar is drawn with.
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

const barsOf = (state) => new Function("state", "selectedBars",
  grab("reviewBars") + "; return reviewBars();")(
  state, (sel) => sel.bars || [{ part: sel.part, measure: sel.measure }]);

test("a mark goes on the bars picked up, the notes' bars, or the cursor's", () => {
  // Bars picked up by their bands, on two staves.
  assert.deepEqual(barsOf({ multi: [], selection: { bar: true,
    bars: [{ part: 0, measure: 2 }, { part: 1, measure: 2 }] } }),
  [{ part: 0, measure: 2 }, { part: 1, measure: 2 }]);
  // Notes picked up: each bar once.
  assert.deepEqual(barsOf({ multi: [
    { part: 0, measure: 4, index: 0 }, { part: 0, measure: 4, index: 1 },
    { part: 0, measure: 5, index: 0 }], selection: { part: 0, measure: 4 } }),
  [{ part: 0, measure: 4 }, { part: 0, measure: 5 }]);
  // Nothing but the cursor.
  assert.deepEqual(barsOf({ multi: [], selection: { part: 1, measure: 7, at: 0 } }),
    [{ part: 1, measure: 7 }]);
  assert.deepEqual(barsOf({ multi: [], selection: null }), []);
});

test("each marked bar is tinted in its mark's colour, and none when hidden", () => {
  const kids = [];
  const svg = {
    firstChild: null,
    insertBefore(n) { kids.unshift(n); return n; },
    querySelectorAll: () => kids.filter((k) => k.cls === "review-band")
      .map((k) => ({ remove: () => kids.splice(kids.indexOf(k), 1) })),
  };
  const el = () => ({ cls: "", attrs: {}, style: {},
    setAttribute(k, v) { this.attrs[k] = v; if (k === "class") this.cls = v; } });
  const hit = { part: 0, measure: 3, x0: 10, x1: 20, staffTop: 5 };
  const state = {
    zoom: 2, hitsByBar: new Map([[3, [hit]]]),
    reviews: [{ part: 0, measure: 3, review: "fix" }, { part: 1, measure: 9, review: "good" }],
    review: { show: true, colours: { good: "#0f0", check: "#fa0", fix: "#f00" } },
  };
  const draw = new Function("state", "document", "staffHeightOf",
    grab("drawReviews") + "; return drawReviews;")(
    state, { createElementNS: () => el() }, () => 4);

  draw(svg);
  assert.equal(kids.length, 1, "one band, for the bar that is on the page");
  assert.equal(kids[0].style.fill, "#f00");
  assert.equal(kids[0].attrs.x, "20");
  assert.equal(kids[0].attrs.width, "20");

  draw(svg);
  assert.equal(kids.length, 1, "drawn again, not drawn twice");

  state.review.show = false;
  draw(svg);
  assert.equal(kids.length, 0, "hidden takes them all away");
});

// The tints are placed again whenever the bars' positions arrive. Switching
// from pages to the strip loads new positions, and the tints stayed at the
// pages' coordinates — off the strip, so the marks looked lost.
test("new bar positions redraw the review tints from them", async () => {
  const state = { reviews: [] };
  let drawnWith = null;
  const fetch = async () => ({
    json: async () => ({
      hits: [{ part: 0, measure: 3, x0: 500, x1: 520, staffTop: 5 }],
      reviews: [{ part: 0, measure: 3, review: "good" }],
    }),
  });
  const load = new Function("state", "fetch", "viewParams", "indexStrips",
    "indexHits", "currentSVG", "drawReviews",
    grab("loadHits") + "; return loadHits;")(
    state, fetch, () => new URLSearchParams(), () => null,
    (hits) => new Map(hits.map((h) => [h.measure, [h]])),
    () => ({}), () => { drawnWith = state.hitsByBar.get(3)?.[0]; });
  await load();
  assert.ok(drawnWith, "the tints were not redrawn when the positions came");
  assert.equal(drawnWith.x0, 500, "redrawn from the old positions, not the new");
  assert.equal(state.reviews.length, 1);
});
