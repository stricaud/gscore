/*
 * scoreaside_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The music leaves the page while a formatted box is open.
 *
 * Opening Title and composer on a symphony stopped the window dead: the
 * dialog never appeared and nothing answered again. A box you can type
 * formatting into keeps a caret, and while a caret is in one WebKit works
 * out the word around it once per drawn frame — it is for the system's own
 * correction candidates — by walking backwards out of the box and through
 * everything before it in the page. Before the dialogs is the music, and a
 * symphony is 223,000 elements of it, every frame.
 *
 * So the music comes out of the page while such a window is open. What has
 * to hold is that it goes back: the same nodes, in the same order, with the
 * view where it was, and not over a score that was drawn again in the
 * meantime.
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

// A page with some music in it and a window that can be closed.
function load(music = ["staff one", "staff two", "staff three"]) {
  const canvas = {
    kids: [...music],
    get firstChild() { return this.kids.length ? this.kids[0] : null; },
    get childNodes() { return this.kids; },
    replaceChildren(...kids) { this.kids = kids; },
  };
  const wrap = { scrollLeft: 0, scrollTop: 0 };
  const dlg = {
    on: [],
    addEventListener(what, fn) { if (what === "close") this.on.push(fn); },
    removeEventListener(what, fn) {
      if (what === "close") this.on = this.on.filter((f) => f !== fn);
    },
    close() { for (const fn of [...this.on]) fn(); },
  };
  const scope = {
    $: (sel) => (sel === "#canvas" ? canvas : sel === "#canvas-wrap" ? wrap : null),
  };
  const body = grab("scoreAside") + "\nreturn { scoreAside };";
  const { scoreAside } = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { scoreAside, canvas, wrap, dlg };
}

test("the music comes out of the page while the window is open", () => {
  const t = load();
  t.scoreAside(t.dlg);
  assert.deepEqual(t.canvas.kids, [],
    "the music was still in the page with a formatted box open");
});

test("and goes back, the same music in the same order, when it closes", () => {
  const t = load();
  t.scoreAside(t.dlg);
  t.dlg.close();
  assert.deepEqual(t.canvas.kids, ["staff one", "staff two", "staff three"]);
});

test("the view comes back to where it was", () => {
  // An empty page has nothing to scroll, so the place somebody was reading
  // is lost by the time the music returns unless it is put back by hand.
  const t = load();
  t.wrap.scrollLeft = 820;
  t.wrap.scrollTop = 1400;
  t.scoreAside(t.dlg);
  t.wrap.scrollLeft = 0;
  t.wrap.scrollTop = 0;
  t.dlg.close();
  assert.equal(t.wrap.scrollTop, 1400, "the view jumped back to the top of the score");
  assert.equal(t.wrap.scrollLeft, 820);
});

test("a score drawn again while the window was open is not painted over", () => {
  // Saving a new title lays the score out again. The music held aside is
  // the music from before the change, and putting it back over the new
  // drawing would show the old title until the next time anything moved.
  const t = load();
  t.scoreAside(t.dlg);
  t.canvas.replaceChildren("the score with its new title");
  t.dlg.close();
  assert.deepEqual(t.canvas.kids, ["the score with its new title"]);
});

test("closing twice does not put the music in twice", () => {
  const t = load();
  t.scoreAside(t.dlg);
  t.dlg.close();
  t.dlg.close();
  assert.deepEqual(t.canvas.kids, ["staff one", "staff two", "staff three"]);
  assert.equal(t.dlg.on.length, 0, "the listener was left on the dialog");
});

test("an empty score is left alone", () => {
  // Nothing to take out, so nothing is waiting to be put back: a new score
  // must not have an empty page restored over the first thing drawn in it.
  const t = load([]);
  t.scoreAside(t.dlg);
  assert.equal(t.dlg.on.length, 0);
  t.canvas.replaceChildren("the first bar somebody typed");
  t.dlg.close();
  assert.deepEqual(t.canvas.kids, ["the first bar somebody typed"]);
});

test("every window with a formatted box in it puts the music aside first", () => {
  // The rule is about the boxes and not about one dialog: both windows that
  // hold one have to do it, and before showing themselves, because the
  // caret goes into the box in the same breath as the window opens.
  for (const name of ["setHeaderDialog", "textDialog"]) {
    const body = grab(name);
    const aside = body.indexOf("scoreAside(dlg)");
    const shown = body.indexOf("dlg.showModal()");
    assert.ok(aside >= 0, name + " opens a box with formatting in it and leaves " +
      "the music in the page: a symphony will stop the window");
    assert.ok(shown >= 0, name + " no longer shows a dialog; this test is out of date");
    assert.ok(aside < shown,
      name + " puts the music aside only after showing the dialog");
  }
});
