/*
 * window_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Drawing a long score a piece at a time.
 *
 * A big orchestral score is not a big picture, it is a great many small
 * ones: Beethoven's ninth is fifty-three staves of two and a half thousand
 * bars and comes to nine hundred thousand shapes. Handed all of them a
 * browser spends seconds laying them out and then walks the lot on every
 * repaint, so scrolling stutters and playing stops dead while it catches
 * up. It is the problem a list widget has with a million rows, and the
 * answer is the same: put what can be seen in the page and fetch the rest
 * as it comes into view.
 *
 * It is done by pieces inside one drawing rather than by replacing what is
 * in view, because replacing it meant parsing everything on screen again
 * every few seconds of scrolling — and sweeping away the cursor, the
 * selection and the lit noteheads with it. Moving along now adds the piece
 * ahead and drops the one left behind.
 *
 * Four things have to hold. An ordinary score must be drawn whole, as
 * before, and never fetched again for being scrolled. A long one must
 * always have the view covered, at either end and in the middle. Moving
 * along must cost one piece, not all of them. And no piece may be big
 * enough to be the problem this is here to solve.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name, kind = "function") => {
  const re = kind === "function"
    ? new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m")
    : new RegExp("^const " + name + " = [^\\n]*$", "m");
  const m = src.match(re);
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};
const constant = (name) => {
  const m = src.match(new RegExp("^const " + name + " = (\\d+(?:\\.\\d+)?);", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return Number(m[1]);
};

const windowArea = constant("windowArea");
const tilePad = constant("tilePad");
const tileKeep = constant("tileKeep");

// The editor's geometry: a panel of a given size scrolled to a given place
// over a drawing of a given size, in CSS pixels as the browser reports it.
function editor({ view = { w: 1400, h: 900 }, zoom = 8 } = {}) {
  const scroll = { x: 0, y: 0 };
  const state = { zoom };
  const wrap = {
    clientWidth: view.w,
    clientHeight: view.h,
    getBoundingClientRect: () => ({ left: 0, top: 0, width: view.w, height: view.h }),
  };
  const svg = { getBoundingClientRect: () => ({ left: -scroll.x, top: -scroll.y }) };
  const scope = {
    state, Math,
    $: (sel) => (sel === "#canvas-wrap" ? wrap : null),
    currentSVG: () => svg,
    windowArea, tilePad, tileKeep,
  };
  const names = ["viewBox", "tilePlan", "tileBox", "tileRange"];
  const body = names.map((n) => grab(n)).join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, state, scrollTo(x, y) { scroll.x = x; scroll.y = y; } };
}

// Beethoven's ninth as the editor's strip: forty thousand staff spaces of
// music across and fifty-three staves down.
const symphony = { width: 40830, height: 748 };
// The same score as pages: a page wide and the length of the music.
const pages = { width: 128, height: 248591 };
// An ordinary song, which fits on screen twice over.
const song = { width: 120, height: 90 };
// A flute study: wide, because the strip is always wide, but only a few
// screenfuls of music and nothing a browser minds.
const study = { width: 3198, height: 89 };

test("the constants are the ones these tests assume", () => {
  assert.equal(grab("windowArea", "const"), "const windowArea = 550000;");
  assert.equal(grab("tilePad", "const"), "const tilePad = 1;");
  assert.equal(grab("tileKeep", "const"), "const tileKeep = 3;");
});

test("a score small enough is drawn whole and never cut up", () => {
  const t = editor();
  assert.equal(t.tilePlan(song), null, "an ordinary score was cut into pieces");
  // Wide is not the same as big: the strip makes every score wide.
  assert.equal(t.tilePlan(study), null, "a study was cut into pieces it does not need");
  // And nothing at all to go on is nothing to cut up.
  assert.equal(t.tilePlan(null), null);
  assert.equal(t.tilePlan({ width: 0, height: 0 }), null);
});

test("the strip is cut across its length, with every staff in each piece", () => {
  const t = editor();
  const plan = t.tilePlan(symphony);
  assert.ok(plan, "the symphony was handed over whole");
  assert.equal(plan.downTheLength, false, "the strip was cut down its depth");
  const box = t.tileBox(plan, symphony, 7);
  assert.equal(box.y0, 0, "a piece does not start at the top staff");
  assert.equal(box.y1, symphony.height, "a piece stops short of the bottom staff");
  assert.ok(box.x1 > box.x0, "a piece covers no music at all");
});

test("pages are cut down their length, with the whole width in each piece", () => {
  const t = editor();
  const plan = t.tilePlan(pages);
  assert.ok(plan, "a paginated symphony was handed over whole");
  assert.equal(plan.downTheLength, true, "pages were cut across their width");
  const box = t.tileBox(plan, pages, 3);
  assert.equal(box.x0, 0);
  assert.equal(box.x1, pages.width);
});

test("no piece is big enough to be the problem this is here to solve", () => {
  for (const [name, size] of [["strip", symphony], ["pages", pages]]) {
    for (const zoom of [5, 8, 16, 26]) {
      const t = editor({ zoom });
      const plan = t.tilePlan(size);
      if (!plan) continue;
      const area = plan.span * plan.across;
      assert.ok(area <= windowArea + 1,
        `${name} at zoom ${zoom}: a piece holds ${Math.round(area)} square ` +
        `spaces of music, and the budget is ${windowArea}`);
    }
  }
});

test("the pieces drawn cover the view, wherever it is", () => {
  const t = editor();
  const plan = t.tilePlan(symphony);
  const endPx = symphony.width * t.state.zoom;
  for (const x of [0, 100, endPx / 3, endPx / 2, endPx - 1400, endPx]) {
    t.scrollTo(x, 0);
    const vis = t.viewBox();
    const r = t.tileRange(plan, symphony, tilePad);
    const first = t.tileBox(plan, symphony, r.lo);
    const last = t.tileBox(plan, symphony, r.hi);
    assert.ok(first.x0 <= vis.x0 + 1e-9,
      `at x=${Math.round(x)} the first piece starts at ${first.x0} and the view at ${vis.x0}`);
    assert.ok(last.x1 >= Math.min(symphony.width, vis.x1) - 1e-9,
      `at x=${Math.round(x)} the last piece ends at ${last.x1} and the view at ${vis.x1}`);
    assert.ok(r.lo >= 0, "a piece before the beginning of the score");
    assert.ok(t.tileBox(plan, symphony, r.hi).x1 <= symphony.width + 1e-9,
      "a piece past the end of the score");
  }
});

test("there is a screenful of music drawn beyond each edge of the view", () => {
  const t = editor();
  const plan = t.tilePlan(symphony);
  t.scrollTo(symphony.width * 4, 0); // well inside the piece
  const vis = t.viewBox();
  const r = t.tileRange(plan, symphony, tilePad);
  const first = t.tileBox(plan, symphony, r.lo);
  const last = t.tileBox(plan, symphony, r.hi);
  const screen = vis.x1 - vis.x0;
  assert.ok(vis.x0 - first.x0 >= screen - plan.span,
    "there is less than a screenful drawn behind the view");
  assert.ok(last.x1 - vis.x1 >= screen - plan.span,
    "there is less than a screenful drawn ahead of the view");
});

test("moving along costs a piece, not a drawing", () => {
  const t = editor();
  const plan = t.tilePlan(symphony);
  const endPx = symphony.width * t.state.zoom;
  let have = new Set();
  let added = 0, biggestStep = 0, first = true;

  for (let x = 0; x <= endPx; x += 60) { // a frame at a time, briskly
    t.scrollTo(x, 0);
    const want = t.tileRange(plan, symphony, tilePad);
    let step = 0;
    for (let i = want.lo; i <= want.hi; i++) {
      if (!have.has(i)) {
        have.add(i);
        added++;
        step++;
      }
    }
    // The first pass fills the view and its margins; every one after it is
    // a frame of scrolling, and that is what must be cheap.
    if (!first && step > biggestStep) biggestStep = step;
    first = false;
    // What is far enough behind to let go of.
    const keep = t.tileRange(plan, symphony, tileKeep);
    for (const i of [...have]) {
      if (i < keep.lo || i > keep.hi) have.delete(i);
    }
    assert.ok(have.size < 12,
      `${have.size} pieces are on the page at once, at x=${x}`);
  }

  const all = Math.ceil(symphony.width / plan.span);
  assert.ok(added >= all, "the whole piece was covered without drawing all of it");
  assert.ok(added < all * 1.5,
    `${added} pieces were drawn for a score that is ${all} pieces long; ` +
    `they are being dropped and fetched again`);
  assert.equal(biggestStep, 1,
    `a single frame of scrolling asked for ${biggestStep} pieces`);
});

test("a piece already behind is kept a while before it is let go", () => {
  // Scrolling back a little should not have to fetch what was just there.
  const t = editor();
  const plan = t.tilePlan(symphony);
  t.scrollTo(symphony.width * 4, 0);
  const want = t.tileRange(plan, symphony, tilePad);
  const keep = t.tileRange(plan, symphony, tileKeep);
  assert.ok(keep.lo < want.lo, "nothing behind the view is kept");
  assert.ok(keep.hi > want.hi, "nothing ahead of the view is kept");
});

test("the drawing is one <svg> with the pieces inside it", () => {
  const draw = grab("drawScore");
  assert.ok(draw.includes("tilePlan("), "drawScore never works out how to cut the score up");
  assert.ok(draw.includes("tileHome"), "there is nowhere for the pieces to go");
  assert.ok(/class", "tiles"/.test(draw),
    "the pieces are not kept together, so one arriving late is drawn over the cursor");
  assert.ok(draw.includes("ensureTiles("), "nothing puts the first pieces on the page");

  const show = grab("showTile");
  assert.ok(show.includes("innerHTML"),
    "a piece is not put into the drawing that is already there");
  assert.ok(!show.includes("$(\"#canvas\")"),
    "a piece replaces the whole canvas, which is what this is here to stop");
});

test("the piece ahead is asked for before it is wanted", () => {
  const fn = grab("ensureTiles");
  assert.ok(/want\.lo - 1, want\.hi \+ 1/.test(fn),
    "nothing is fetched beyond what is drawn, so arriving at it waits for it");
  assert.ok(fn.includes("fetchTile("), "the piece ahead is not fetched");

  // And one piece is asked for once, however many times it is wanted.
  const fetchOne = grab("fetchTile");
  assert.ok(/tileText\.get\(i\)/.test(fetchOne),
    "a piece in the air is asked for again by the next pass over it");
});

test("the empty drawing really is empty", () => {
  // The <svg> the pieces go inside is asked for with a window nothing is
  // in. The coordinates are sent to the hundredth of a space, so a window
  // thinner than that rounds to nothing — and a window of no width at all
  // is read by the server as no window, which answers with the whole
  // score. Fifty-six megabytes of it, silently, on every open.
  const draw = grab("drawScore");
  const m = draw.match(/window:\s*(\{[^}]*\})/);
  assert.ok(m, "drawScore no longer asks for an empty drawing");
  const box = new Function("return " + m[1])();

  const state = { zoom: 8, mode: "strip" };
  const scope = { state, URLSearchParams, Date, Math };
  const api = new Function(...Object.keys(scope),
    grab("viewParams") + "\n" + grab("scoreURL") + "\nreturn { scoreURL };"
  )(...Object.values(scope));
  const q = new URLSearchParams(api.scoreURL({ window: box }).split("?")[1]);

  for (const [a, b] of [["x0", "x1"], ["y0", "y1"]]) {
    assert.ok(q.get(a) !== null && q.get(b) !== null,
      "the empty drawing asks for no window at all");
    assert.notEqual(q.get(a), q.get(b),
      `the window is sent as ${a}=${q.get(a)} ${b}=${q.get(b)}, which is no ` +
      `window, and the whole score comes back`);
  }
  // And nothing is in it: off the edge of the music rather than inside it.
  assert.ok(box.x1 <= 0 && box.y1 <= 0,
    "the empty drawing's window reaches into the score, so it is not empty");
});

test("a tune book is not cut up", () => {
  const fn = grab("drawScore");
  const book = fn.slice(0, fn.indexOf("await drawBook()"));
  assert.ok(/state\.tiles\s*=\s*null/.test(book) && book.includes("forgetTiles()"),
    "drawScore goes off to draw the book still thinking it has pieces to place");
});
