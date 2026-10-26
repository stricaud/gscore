/*
 * harmonics_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The harmonics panel: what it draws from what the core says.
 *
 * The arithmetic is the core's and is tested there. What is tested here is
 * the drawing: that every partial is on the page, that the axis is
 * logarithmic — an octave is a doubling, so on a log axis every octave is
 * the same width and a note's partials are spread out rather than crowded
 * at the bottom — and that the intervals table names both notes of each
 * pair, which it did not when two fields shared a JSON tag.
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

// The panel's own two drawing functions, with the page replaced by a note of
// what was written into it.
function load() {
  const boxes = {};
  const scope = {
    $: (sel) => (boxes[sel] ||= { innerHTML: "", textContent: "", style: {} }),
  };
  const body = grabConst("harmColours") + "\n" +
    grab("drawSpectrum") + "\n" + grab("drawIntervals") +
    "\nreturn { drawSpectrum, drawIntervals };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { boxes, ...api };
}

// A fifth: two notes, four partials each, with one meeting.
const fifth = {
  at: 1.5,
  notes: [{ name: "C4", freq: 261.63 }, { name: "G4", freq: 392 }],
  partials: [
    { note: 0, n: 1, freq: 261.63, amp: 1 },
    { note: 0, n: 2, freq: 523.25, amp: 0.5 },
    { note: 0, n: 3, freq: 784.88, amp: 0.33 },
    { note: 1, n: 1, freq: 392, amp: 1 },
    { note: 1, n: 2, freq: 784, amp: 0.5 },
  ],
  coincident: [{ a: 0, b: 1, na: 3, nb: 2, freq: 784.44, cents: -1.96, beats: 0.88 }],
  intervals: [{ a: 0, b: 1, semitones: 7, name: "perfect fifth", ratio: "3:2", cents: -1.96 }],
  roughness: 0.188,
  verdict: "smooth — the partials line up",
};

test("every partial is drawn", () => {
  const { boxes, drawSpectrum } = load();
  drawSpectrum(fifth);
  const svg = boxes["#harm-spectrum"].innerHTML;
  const lines = svg.match(/class="harm-partial"/g) || [];
  assert.equal(lines.length, fifth.partials.length);
});

test("the two notes are told apart by colour", () => {
  const { boxes, drawSpectrum } = load();
  drawSpectrum(fifth);
  const svg = boxes["#harm-spectrum"].innerHTML;
  for (const colour of ["#2f6fb3", "#d1702f"]) {
    assert.ok(svg.includes(colour), `the partials of one note are not drawn in ${colour}`);
  }
});

test("the axis is logarithmic, so an octave is an octave wherever it falls", () => {
  const { boxes, drawSpectrum } = load();
  // Three partials an octave apart: on a log axis they must be evenly
  // spaced, and on a linear one they would not be.
  drawSpectrum({
    ...fifth,
    coincident: [],
    partials: [
      { note: 0, n: 1, freq: 100, amp: 1 },
      { note: 0, n: 2, freq: 200, amp: 1 },
      { note: 0, n: 4, freq: 400, amp: 1 },
    ],
  });
  const xs = [...boxes["#harm-spectrum"].innerHTML.matchAll(/x1="([\d.]+)"/g)]
    .map((m) => Number(m[1]));
  // The gridlines come first; the partials are the last three.
  const [a, b, c] = xs.slice(-3).sort((p, q) => p - q);
  assert.ok(Math.abs((b - a) - (c - b)) < 0.5,
    `octaves came out ${(b - a).toFixed(1)} and ${(c - b).toFixed(1)} wide`);
});

test("where partials meet is marked", () => {
  const { boxes, drawSpectrum } = load();
  drawSpectrum(fifth);
  assert.ok(boxes["#harm-spectrum"].innerHTML.includes("harm-meet"));
  assert.match(boxes["#harm-legend"].textContent, /One pair of partials meets/);
});

test("and when nothing meets it says so", () => {
  const { boxes, drawSpectrum } = load();
  drawSpectrum({ ...fifth, coincident: [] });
  assert.match(boxes["#harm-legend"].textContent, /No partials meet/);
});

test("the intervals table names both notes of the pair", () => {
  // Both: `A, B int` under one JSON tag sent only the first, and the table
  // showed a question mark for every second note.
  const { boxes, drawIntervals } = load();
  drawIntervals(fifth);
  const html = boxes["#harm-intervals"].innerHTML;
  assert.ok(html.includes("C4"), "the lower note is missing");
  assert.ok(html.includes("G4"), "the upper note is missing");
  assert.ok(!html.includes("?"), "a note came out unnamed");
  assert.ok(html.includes("perfect fifth") && html.includes("3:2"));
});

test("a fifth's two cents is quiet, a third's fourteen is not", () => {
  const { boxes, drawIntervals } = load();
  drawIntervals({
    notes: [{ name: "C4" }, { name: "E4" }, { name: "G4" }],
    intervals: [
      { a: 0, b: 2, name: "perfect fifth", ratio: "3:2", cents: -1.96 },
      { a: 0, b: 1, name: "major third", ratio: "5:4", cents: 13.69 },
    ],
  });
  const html = boxes["#harm-intervals"].innerHTML;
  assert.ok(html.includes(`class="harm-near">-2.0¢`), "the fifth is not marked as near");
  assert.ok(html.includes(`class="harm-far">+13.7¢`), "the third is not marked as far");
});

test("nothing sounding draws nothing rather than throwing", () => {
  const { boxes, drawSpectrum, drawIntervals } = load();
  drawSpectrum({ partials: [], coincident: [] });
  drawIntervals({ notes: [], intervals: [] });
  assert.match(boxes["#harm-spectrum"].innerHTML, /Nothing is sounding/);
  assert.equal(boxes["#harm-intervals"].innerHTML, "");
});
