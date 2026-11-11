/*
 * staffnames_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The staff names that follow the view.
 *
 * A score of twenty-three staves is read across, and the only thing saying
 * which line belongs to which instrument is a column of words at the far
 * left. Four screens along, finding out what you are looking at means going
 * all the way back and all the way out again.
 *
 * So the names follow, and they are the mirror of the sprint buttons: those
 * appear only at the head of the score, these only once it has been left.
 * Neither is ever on screen at the same time as the thing it stands in for.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

function grab(name) {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
}

// The editor's name strip, run against a made-up score of three staves.
function run({ scrollLeft = 400, mode = "strip", names = true,
  parts = [{ name: "Flauto" }, { name: "Oboi" }, { name: "" }] } = {}) {
  const box = {
    hidden: true, textContent: "", children: [],
    appendChild(el) { this.children.push(el); },
    getBoundingClientRect: () => ({ top: 0 }),
  };
  const wrap = { scrollLeft, clientHeight: 600, clientWidth: 800 };
  const svg = { getBoundingClientRect: () => ({ top: 0 }) };
  const placed = [];

  const state = {
    mode,
    zoom: 1,
    hits: [
      { part: 0, staffTop: 10 }, { part: 0, staffTop: 10 },
      { part: 1, staffTop: 60 },
      { part: 2, staffTop: 110 },
    ],
    info: { parts },
    view: { names },
  };
  const scope = {
    $: (sel) => ({ "#staff-names": box, "#canvas-wrap": wrap }[sel] || null),
    currentSVG: () => svg,
    state,
    staffHeightOf: () => 32,
    sprintRows: null, // filled in below
    placeSprints: (rows, b) => {
      placed.push(...rows.map((r) => r.name));
      // What the real one does: hide the labels whose staff is off screen.
      b.children.forEach((el, i) => { el.hidden = false; void i; });
    },
    document: {
      createElement: () => ({
        className: "", textContent: "", title: "", hidden: false,
        style: {},
      }),
    },
  };
  // staffNameKey lives beside the function in app.js, so the lifted
  // copy needs one of its own.
  const body = 'let staffNameKey = "";\n'
    + grab("sprintRows") + "\n" + grab("nameOf") + "\n"
    + grab("drawStaffNames");
  const make = new Function(
    "$", "currentSVG", "state", "staffHeightOf", "placeSprints", "document",
    body + "\nreturn { drawStaffNames, sprintRows };");
  const api = make(scope.$, scope.currentSVG, scope.state, scope.staffHeightOf,
    scope.placeSprints, scope.document);
  api.drawStaffNames();
  return { box, placed };
}

test("the names follow once the head of the score has been left", () => {
  const { box, placed } = run({ scrollLeft: 400 });
  assert.equal(box.hidden, false, "the name strip is hidden");
  assert.deepEqual(box.children.map((c) => c.textContent), ["Flauto", "Oboi"],
    "one label per named staff, in the order they are drawn down the page");
  assert.deepEqual(placed, ["Flauto", "Oboi"],
    "the labels are put beside their own staves");
});

test("and are not there while the real ones are", () => {
  const { box } = run({ scrollLeft: 0 });
  assert.equal(box.hidden, true,
    "the names are written on the page at the head of the score, and a " +
    "second copy over the first is clutter");
  assert.equal(box.children.length, 0);
});

test("a page of music is not a strip, and has its names on every system", () => {
  const { box } = run({ scrollLeft: 400, mode: "page" });
  assert.equal(box.hidden, true);
});

test("the strip is pinned to the panel and does not take the mouse", () => {
  const rule = css.match(/#staff-names\s*\{[^}]*\}/);
  assert.ok(rule, "style.css has no rule for the name strip");
  assert.match(rule[0], /position:\s*sticky/,
    "the names have to stay put while the music moves under them");
  assert.match(rule[0], /pointer-events:\s*none/,
    "they stand over the music, so a click has to reach the bar beneath");
});

test("the page has somewhere to put them", () => {
  assert.match(html, /id="staff-names"/);
});

test("a staff nobody has named is not labelled", () => {
  // sprintRows calls it "Staff 3" so its button can say what it points at.
  // A name somebody cleared is one they want gone, and a made-up one beside
  // the staff is the label coming back under another name.
  const { box } = run();
  assert.ok(!box.children.some((c) => /^Staff \d+$/.test(c.textContent)),
    "an unnamed staff was given a name of the editor's own");
});

test("View, Staff names takes these away too", () => {
  const { box } = run({ names: false });
  assert.equal(box.hidden, true,
    "the names were turned off, and these stand in for them");
  assert.equal(box.children.length, 0);
});

test("and a score with no named staves at all shows nothing", () => {
  const { box } = run({ parts: [{ name: "" }, { name: "" }, { name: "" }] });
  assert.equal(box.children.length, 0);
});

test("the strip opens away, before anything has been scrolled", () => {
  // drawStaffNames runs on scrolls and resizes, so until the first one the
  // page is on its own: a strip that opens visible is an empty box over the
  // head of the score.
  const m = html.match(/<div id="staff-names"[^>]*>/);
  assert.ok(m, "index.html has no name strip");
  assert.match(m[0], /\bhidden\b/);
});
