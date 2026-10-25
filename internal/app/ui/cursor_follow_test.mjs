/*
 * cursor_follow_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The page follows the caret while you write.
 *
 * Typing a bar of notes walks the caret to the right of the window and then
 * off it, and the page used to stand still: the note being written was the
 * one note that could not be seen. Reported as "when I use the keyboard to
 * type I am on the right of the screen and then I cannot follow the caret,
 * it is out of display".
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
const grabConst = (name) => {
  const m = src.match(new RegExp("^const " + name + " = [^;]+;", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// A window 800 wide onto a strip of music 4000 wide, with the caret wherever
// the test puts it.
function load({ caret, notehead, scrollLeft = 0, scrollTop = 0 }) {
  const moved = [];
  const wrap = {
    scrollLeft, scrollTop,
    scrollWidth: 4000, clientWidth: 800,
    scrollHeight: 2000, clientHeight: 400,
    getBoundingClientRect: () => ({ left: 0, top: 0, right: 800, bottom: 400 }),
    scrollTo: (to) => moved.push(to),
  };
  const box = (b) => ({ getBoundingClientRect: () => b });
  const svg = {
    querySelector(sel) {
      if (sel === ".write-block") return caret ? box(caret) : null;
      return notehead ? box(notehead) : null;
    },
  };
  const scope = {
    state: { zoom: 10, selection: { ref: 7 } },
    $: () => wrap,
    currentSVG: () => svg,
  };
  const body = grabConst("cursorLead") + "\n" + grabConst("cursorMargin") + "\n" +
    grab("scrollSelectionIntoView") + "\nreturn { scrollSelectionIntoView };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { moved, wrap, ...api };
}

test("a caret near the right edge pulls the page after it", () => {
  const { moved, scrollSelectionIntoView } =
    load({ caret: { left: 790, right: 794, top: 100, bottom: 140 } });
  scrollSelectionIntoView();
  assert.equal(moved.length, 1, "the page did not move and the caret is off it");
  // A third of the way across, which leaves most of a screen of room ahead.
  assert.equal(moved[0].left, 790 - 800 * 0.32);
});

test("the caret left on screen after the move, with room to write into", () => {
  const { moved, scrollSelectionIntoView } =
    load({ caret: { left: 790, right: 794, top: 100, bottom: 140 } });
  scrollSelectionIntoView();
  const seen = 790 - moved[0].left;
  assert.ok(seen > 0 && seen < 800, `the caret is at ${seen}, off a window 800 wide`);
  assert.ok(800 - seen > 400, "less than half a screen of room ahead of the caret");
});

test("a caret in the middle of the view leaves the page alone", () => {
  const { moved, scrollSelectionIntoView } =
    load({ caret: { left: 300, right: 304, top: 100, bottom: 140 } });
  scrollSelectionIntoView();
  assert.equal(moved.length, 0);
});

test("going back the other way puts the room behind it", () => {
  const { moved, scrollSelectionIntoView } =
    load({ caret: { left: 10, right: 14, top: 100, bottom: 140 }, scrollLeft: 1000 });
  scrollSelectionIntoView();
  assert.equal(moved.length, 1);
  assert.equal(moved[0].left, 1010 - 800 * (1 - 0.32));
});

test("the caret is followed even with nothing picked up", () => {
  // Writing a bar of notes picks nothing up, which is when the old
  // scrolling gave up: it looked for a selected notehead and there is none.
  const { moved, scrollSelectionIntoView } =
    load({ caret: { left: 790, right: 794, top: 100, bottom: 140 }, notehead: null });
  scrollSelectionIntoView();
  assert.equal(moved.length, 1);
});

test("a caret below the view brings the next system up", () => {
  const { moved, scrollSelectionIntoView } =
    load({ caret: { left: 300, right: 304, top: 600, bottom: 640 } });
  scrollSelectionIntoView();
  assert.equal(moved.length, 1);
  assert.equal(moved[0].top, 600 - 400 * 0.3);
});

test("the page does not scroll past the end of the music", () => {
  const { moved, scrollSelectionIntoView } =
    load({ caret: { left: 790, right: 794, top: 100, bottom: 140 }, scrollLeft: 3100 });
  scrollSelectionIntoView();
  assert.ok(moved[0].left <= 4000 - 800, "scrolled past the end of the strip");
});
