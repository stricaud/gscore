/*
 * window_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Drawing a long score a screenful at a time.
 *
 * A big orchestral score is not a big picture, it is a great many small
 * ones: Beethoven's ninth is fifty-three staves of two and a half thousand
 * bars and comes to nine hundred thousand shapes. Handed all of them a
 * browser spends seconds laying them out and then walks the lot on every
 * repaint, so scrolling stutters and playing stops dead while it catches up.
 * It is the problem a list widget has with a million rows, and the answer is
 * the same: put what can be seen in the page and fetch the rest as it comes
 * into view.
 *
 * Three things have to hold. An ordinary score must be drawn whole, exactly
 * as before, and must never be fetched again for being scrolled. A long one
 * must always have the view well inside what is drawn, so that scrolling
 * never reaches a blank edge. And a scroll right across the piece must cost
 * a few fetches rather than one per frame.
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

// The editor's geometry, with a panel of a given size scrolled to a given
// place over a drawing of a given size. Everything is in CSS pixels, as the
// browser reports it, and the functions under test turn it into staff
// spaces.
function editor({ view = { w: 1400, h: 900 }, size, zoom = 8 } = {}) {
  const scroll = { x: 0, y: 0 };
  const state = { zoom, size, drawn: null };
  const wrap = {
    clientWidth: view.w,
    clientHeight: view.h,
    getBoundingClientRect: () => ({ left: 0, top: 0, width: view.w, height: view.h }),
  };
  // The drawing sits under the panel and moves up and left as it scrolls.
  const svg = {
    getBoundingClientRect: () => ({ left: -scroll.x, top: -scroll.y }),
  };
  const asked = [];
  const scope = {
    state, Math,
    $: (sel) => (sel === "#canvas-wrap" ? wrap : null),
    currentSVG: () => svg,
    drawPad: 1.0,
    drawSlack: 0.25,
    windowArea: 550000,
  };
  const body = grab("viewBox") + "\n" + grab("drawWindow") + "\n" + grab("stillCovered") +
    "\nreturn { viewBox, drawWindow, stillCovered };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return {
    ...api, state, scroll, asked,
    scrollTo(x, y) { scroll.x = x; scroll.y = y; },
  };
}

// Beethoven's ninth as the editor's strip: forty thousand staff spaces of
// music across and fifty-three staves down.
const symphony = { width: 40830, height: 748 };
// And an ordinary song, which fits on screen twice over.
const song = { width: 120, height: 90 };
// A flute study: three thousand staff spaces of one stave, which is wide but
// is only a few screenfuls of music and nothing a browser minds.
const study = { width: 3198, height: 89 };

test("the constants are the ones these tests assume", () => {
  assert.equal(grab("drawPad", "const"), "const drawPad = 1.0;");
  assert.equal(grab("drawSlack", "const"), "const drawSlack = 0.25;");
  assert.equal(grab("windowArea", "const"), "const windowArea = 550000;");
});

test("a score that is merely wide is still drawn whole", () => {
  // The editing strip is one long system, so every score is wide. What
  // decides is how much music there is, not how far across the page it
  // goes: a flute study is a third of the budget and a browser does not
  // mind it, and windowing it would only mean fetching again on every
  // scroll.
  const t = editor({ size: study });
  assert.equal(t.drawWindow(study), null,
    "a study was cut into windows it does not need");
  // The symphony, which is a hundred times as much music, is not.
  const big = editor({ size: symphony });
  assert.ok(big.drawWindow(symphony), "the symphony was handed over whole");
});

test("a score that fits is drawn whole and never redrawn", () => {
  const t = editor({ size: song });
  assert.equal(t.drawWindow(song), null,
    "an ordinary score was cut into windows");
  // And nothing watches it: with nothing drawn as a window there is nothing
  // for a scroll to invalidate.
  assert.equal(t.stillCovered(null, song), true);
});

test("a score with no size behind it is drawn whole", () => {
  // Served by something without the route, or before the first answer.
  const t = editor({ size: null });
  assert.equal(t.drawWindow(null), null);
  assert.equal(t.drawWindow({ width: 0, height: 0 }), null);
});

test("the window round the view is a screenful past each edge of the long side", () => {
  const t = editor({ size: symphony });
  t.scrollTo(16000, 1600); // in pixels, so 2000 x 200 staff spaces
  const vis = t.viewBox();
  assert.equal(vis.x0, 2000);
  assert.equal(vis.y0, 200);
  assert.equal(vis.x1, 2000 + 1400 / 8);
  const box = t.drawWindow(symphony);
  const w = vis.x1 - vis.x0;
  assert.ok(Math.abs(box.x0 - (vis.x0 - w)) < 1e-9, "no margin to the left");
  assert.ok(Math.abs(box.x1 - (vis.x1 + w)) < 1e-9, "no margin to the right");
});

test("the short side is taken whole, so moving along it fetches nothing", () => {
  // The editing strip is one system running the length of the piece and
  // only as tall as the orchestra, so the whole height is a column of a few
  // thousand more shapes. Worth it: scrolling down the staves while the
  // music plays used to stop the playing at every step, and leave a band of
  // blank paper under the last staff it had drawn.
  const t = editor({ size: symphony });
  t.scrollTo(16000, 0);
  const box = t.drawWindow(symphony);
  assert.equal(box.y0, 0, "the column does not start at the top staff");
  assert.equal(box.y1, symphony.height, "the column stops short of the bottom staff");

  // And scrolling all the way down it asks for nothing.
  const drawn = box;
  for (let y = 0; y <= symphony.height * 8; y += 40) {
    t.scrollTo(16000, y);
    assert.equal(t.stillCovered(drawn, symphony), true,
      "scrolling down the staves wanted a fetch, at y = " + y);
  }
});

test("a paginated score is the same rule the other way up", () => {
  // Pages are a page wide and as long as the music, so it is the width that
  // is taken whole and the length that is drawn a piece at a time.
  const pages = { width: 128, height: 248591 };
  const t = editor({ size: pages });
  t.scrollTo(0, 800000);
  const box = t.drawWindow(pages);
  assert.equal(box.x0, 0, "the window does not reach the left margin");
  assert.equal(box.x1, pages.width, "the window stops short of the right margin");
  assert.ok(box.y0 > 0 && box.y1 < pages.height, "the length was not windowed");
});

test("the window is clipped to the score, not off the end of it", () => {
  const t = editor({ size: symphony });
  t.scrollTo(0, 0);
  const box = t.drawWindow(symphony);
  assert.equal(box.x0, 0, "the window began before the music");
  assert.equal(box.y0, 0);
  t.scrollTo(symphony.width * 8, symphony.height * 8);
  const end = t.drawWindow(symphony);
  assert.equal(end.x1, symphony.width, "the window ran past the end of the music");
  assert.equal(end.y1, symphony.height);
});

test("the view stays well inside what is drawn until it has moved a long way", () => {
  const t = editor({ size: symphony });
  t.scrollTo(16000, 0);
  const drawn = t.drawWindow(symphony);
  assert.equal(t.stillCovered(drawn, symphony), true, "it wanted redrawing at once");

  // A quarter of a screen of movement is still covered; most of a screenful
  // is not, and that is when the next stretch is fetched.
  t.scrollTo(16000 + 1400 * 0.2, 0);
  assert.equal(t.stillCovered(drawn, symphony), true,
    "a small scroll threw the drawing away");
  t.scrollTo(16000 + 1400 * 0.8, 0);
  assert.equal(t.stillCovered(drawn, symphony), false,
    "the view was about to run off the edge of what is drawn");
});

test("at the end of the score there is nothing more to fetch", () => {
  const t = editor({ size: symphony });
  t.scrollTo(symphony.width * 8, 0);
  const drawn = t.drawWindow(symphony);
  // Scrolled hard against the right-hand end: the window cannot reach any
  // further, so being near its edge must not ask for it again for ever.
  assert.equal(t.stillCovered(drawn, symphony), true,
    "the end of the score asks to be redrawn on every frame");
});

test("scrolling the whole way across costs a handful of fetches, not one a frame", () => {
  const t = editor({ size: symphony });
  let drawn = t.drawWindow(symphony);
  let fetches = 1;
  // A frame at a time, at a brisk reading pace, the whole length of the
  // piece.
  const stepPx = 40;
  for (let x = 0; x <= symphony.width * 8; x += stepPx) {
    t.scrollTo(x, 0);
    if (!t.stillCovered(drawn, symphony)) {
      drawn = t.drawWindow(symphony);
      fetches++;
      assert.equal(t.stillCovered(drawn, symphony), true,
        "a fresh window did not cover the view it was fetched for, at x=" + x);
    }
  }
  // What matters is how far the view moves between fetches: a screenful
  // either side, redrawn with a quarter of it still in hand, means roughly
  // three quarters of a screen of scrolling buys one fetch. Anything near
  // one per frame would be the old problem in a new place.
  const travelled = symphony.width * 8;
  const perFetch = travelled / fetches;
  assert.ok(perFetch > 1400 * 0.5,
    `a fetch every ${Math.round(perFetch)} pixels, which is less than half a screen`);
  assert.ok(fetches > 10, "the whole piece was covered without refetching at all");
});

test("the drawing asks the server for the stretch, and for the whole thing otherwise", () => {
  const fn = grab("scoreURL");
  assert.ok(fn.includes("opts.window"), "scoreURL cannot ask for a stretch");
  for (const k of ["x0", "y0", "x1", "y1"]) {
    assert.ok(fn.includes(`"${k}"`), "scoreURL does not send " + k);
  }
  const draw = grab("drawScore");
  assert.ok(draw.includes("loadSize("), "drawScore never finds out how big the score is");
  assert.ok(draw.includes("drawWindow("), "drawScore never works out which stretch to draw");
  const redraw = grab("redrawWindow");
  assert.ok(redraw.includes("markSelection("),
    "replacing the drawing loses the selection and the cursor");
});

test("a tune book turns the windowing off", () => {
  // A book is drawn whole, one tune after another, by a different path. A
  // window left recorded from the single-tune view would have the next
  // scroll fetch one tune's stretch and put it in place of the book.
  const fn = grab("drawScore");
  const book = fn.slice(0, fn.indexOf("await drawBook()"));
  assert.ok(/state\.drawn\s*=\s*null/.test(book),
    "drawScore goes off to draw the book still thinking it has a window");
});

test("playback finds the drawing again after it has been replaced", () => {
  // A long score is drawn a screenful at a time, so the drawing is replaced
  // underneath playback as the music moves along. A list of shapes made
  // from the drawing that has gone points at nothing, and the notes stop
  // lighting up halfway through the piece.
  const fn = grab("refIndex");
  assert.ok(fn.includes("currentSVG()"),
    "refIndex holds on to one drawing for ever");
  assert.ok(/made\s*!==\s*svg/.test(fn),
    "refIndex never notices that the drawing has changed");
  const follow = src.match(/^function follow\(pb\)[\s\S]*?^}/m)[0];
  assert.ok(follow.includes("refIndex()"),
    "follow still hands refIndex a drawing that will go stale");
});

test("only one fetch is in the air at a time", () => {
  // Scrolling asks far more often than the answers come back, and answers
  // arriving out of order would leave the wrong stretch drawn last.
  const fn = grab("redrawWindow");
  assert.ok(fn.includes("windowFetch"), "nothing stops the fetches piling up");
  assert.ok(fn.includes("windowAgain"), "a move made during a fetch is forgotten");
});
