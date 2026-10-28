/*
 * scoremap_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The shape of a piece.
 *
 * Who is playing when, and what repeats what — both questions about the
 * whole of it, which a window showing twelve bars cannot answer and which
 * anybody holding a score reads off it in a glance.
 *
 * The arithmetic is tested in internal/analysis. What is tested here is the
 * picture: that a staff's whole part fits on one line however long the piece
 * is, that resting and playing are told apart, that a return says how far it
 * has moved in the words a musician uses, and that every return can be gone
 * to.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const here = (f) => readFileSync(new URL("./" + f, import.meta.url), "utf8");
const src = here("app.js");
const html = here("index.html");
const css = here("style.css");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};
const konst = (name) =>
  Number(src.match(new RegExp("^const " + name + " = (\\d+);", "m"))[1]);

function panel(d) {
  const made = new Map();
  const scope = {
    Math,
    escapeHTML: (s) => String(s),
    barNumberOf: (n) => n + 1,
    mapColumns: konst("mapColumns"),
    mapLevels: konst("mapLevels"),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "", textContent: "" });
      return made.get(sel);
    },
  };
  const names = ["drawScoreMap", "mapRow", "mapStrip", "mapEcho", "movedBy"];
  const body = names.map((n) => grab(n)).join("\n") + "\nreturn { drawScoreMap };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawScoreMap(d);
  return made;
}

// A staff that plays for the first half of a long piece and rests for the
// second.
function halfAndHalf(bars) {
  const counts = [];
  for (let i = 0; i < bars; i++) counts.push(i < bars / 2 ? 6 : 0);
  return counts;
}

test("a staff's whole part fits on one line however long the piece is", () => {
  for (const bars of [8, 335, 2617]) {
    const made = panel({
      bars, busiest: 6,
      staves: [{ part: 0, name: "Flauto 1", bars: halfAndHalf(bars) }],
      echoes: [],
    });
    const svg = made.get("#map-body").innerHTML;
    const width = Number(svg.match(/viewBox="0 0 (\d+)/)[1]);
    assert.equal(width, konst("mapColumns"),
      `${bars} bars were drawn ${width} wide instead of in a fixed number of columns`);
  }
});

// Drawn only where there is something. A staff that rests for eighty bars
// leaves eighty bars of paper, which is the thing worth seeing.
test("resting and playing are told apart", () => {
  const svg = panel({
    bars: 100, busiest: 6,
    staves: [{ part: 0, name: "Oboe", bars: halfAndHalf(100) }],
    echoes: [],
  }).get("#map-body").innerHTML;

  // Only the rectangles drawn for music, not the pale one behind them.
  const playing = [...svg.matchAll(
    /<rect x="(\d+)" y="0" width="(\d+)" height="\d+" fill="var\(--accent\)"/g)]
    .map((m) => [Number(m[1]), Number(m[2])]);
  assert.equal(playing.length, 1,
    "a staff that plays for one unbroken half was drawn in more than one piece");
  assert.equal(playing[0][0], 0, "it does not start at the beginning");
  assert.ok(Math.abs(playing[0][1] - konst("mapColumns") / 2) <= 1,
    `the half it plays came out ${playing[0][1]} of ${konst("mapColumns")} columns wide`);
});

// An entry is the thing worth seeing, and averaging it with the rests around
// it is how an entry disappears.
test("one busy bar among many quiet ones still shows", () => {
  const bars = 2000;
  const counts = new Array(bars).fill(0);
  counts[1000] = 40;
  const svg = panel({
    bars, busiest: 40,
    staves: [{ part: 0, name: "Piccolo", bars: counts }],
    echoes: [],
  }).get("#map-body").innerHTML;
  assert.match(svg, /fill="var\(--accent\)"/,
    "a single bar among two thousand was averaged away to nothing");
});

test("a staff with nothing on it is still a line", () => {
  const svg = panel({
    bars: 20, busiest: 4,
    staves: [{ part: 0, name: "Trombone 3", bars: new Array(20).fill(0) }],
    echoes: [],
  }).get("#map-body").innerHTML;
  assert.match(svg, /Trombone 3/, "the staff is not listed at all");
  assert.match(svg, /map-rest/, "nothing is drawn where its part would be");
  assert.ok(!/fill="var\(--accent\)"/.test(svg), "it was drawn as playing");
});

// The recapitulation of a sonata brings its second subject back a fifth
// lower, and "−7 semitones" is the same fact in a language nobody reads
// music in.
test("a return says how far it has moved, in words", () => {
  const scope = { Math };
  const movedBy = new Function("Math", grab("movedBy") + "\nreturn movedBy;")(Math);
  assert.equal(movedBy(0), "the same notes");
  assert.equal(movedBy(-7), "a fifth lower");
  assert.equal(movedBy(2), "a tone higher");
  assert.equal(movedBy(12), "an octave higher");
  assert.equal(movedBy(-14), "a tone and an octave lower");
  void scope;
});

test("every return is listed and can be gone to", () => {
  const made = panel({
    bars: 300, busiest: 6,
    staves: [{ part: 0, name: "Piano", bars: new Array(300).fill(3) }],
    echoes: [
      { fromBar: 235, toBar: 242, likeBar: 133, bars: 8, transposed: 0 },
      { fromBar: 264, toBar: 274, likeBar: 168, bars: 11, transposed: -7 },
    ],
  });
  const body = made.get("#map-body").innerHTML;
  assert.match(body, /bars 236–243<\/b> are bars 134–141 again/,
    "a return does not say what it is a return of");
  assert.match(body, /a fifth lower/, "nor how far it has moved");
  assert.match(body, /data-bar="235"/, "the first return cannot be gone to");
  assert.match(body, /data-bar="264"/, "nor the second");
  assert.match(grab("setupScoreMapClicks"), /goToMeasure\(/,
    "clicking a return goes nowhere");
  assert.match(made.get("#map-where").textContent, /2 returns/);
});

test("a piece that never repeats itself says so", () => {
  const body = panel({
    bars: 30, busiest: 2,
    staves: [{ part: 0, name: "Piano", bars: new Array(30).fill(2) }],
    echoes: [],
  }).get("#map-body").innerHTML;
  assert.match(body, /Nothing in it comes back/);
});

test("an empty score says there is nothing yet", () => {
  assert.match(panel({ bars: 0 }).get("#map-body").innerHTML, /nothing written yet/);
});

test("the window holds still, closes from the top, and waits to be asked", () => {
  const dialog = html.match(/<dialog id="scoremap">[\s\S]*?<\/dialog>/)[0];
  assert.ok(dialog.indexOf('class="dlg-close"') < dialog.indexOf('id="map-body"'),
    "the close button is inside the part that scrolls");
  assert.match(css.match(/#scoremap \{[\s\S]*?\}/)[0], /height:\s*min\(/,
    "the window grows to whatever is in it");
  assert.match(css, /#scoremap\[open\] \{ display: flex; \}/,
    "the window is on the screen before anybody asks for it");
  assert.match(html, /data-action="scoremap"/, "there is no menu item for it");
  assert.match(src, /case "scoremap": await openScoreMap\(\)/,
    "the menu item does not open it");
});
