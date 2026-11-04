/*
 * flutechart_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Looking through every fingering the flute has.
 *
 * The question a window like this has to answer is not "what is in the
 * table" but "what do my hands do", and that is a shape rather than a
 * sentence — so every answer is drawn. What must not happen is the
 * drawing and the table drifting apart: the keys, their names, which
 * hand they belong to and what notes there are all come from the
 * endpoint, and nothing here writes them down again.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const chart = readFileSync(new URL("./fingering.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");
const api = readFileSync(
  new URL("../../../docs/openapi.yaml", import.meta.url), "utf8");

const grab = (where, name) => {
  const m = where.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("no " + name);
  return m[0];
};

test("the menu entry is live and in French", () => {
  const m = html.match(/data-action="flute-chart"[^>]*>/);
  assert.ok(m, "no menu entry");
  assert.ok(!m[0].includes("disabled"));
  assert.match(src, /case "flute-chart": await fluteDialog\(\)/);
  assert.ok(fr.includes('"Flute fingerings…":'), "no French");
});

// Two lists of what a flute is would be one too many: the second is
// where a key quietly moves and the pictures stop matching the numbers.
test("the instrument comes from the endpoint, not from the editor", () => {
  const body = grab(src, "fluteDialog");
  assert.match(body, /fetch\("\/api\/flute\?/);
  assert.match(body, /kit\.keys/);
  assert.match(body, /kit\.left/);
  assert.match(body, /kit\.notes/);
  // No note names and no key names written into the window.
  assert.doesNotMatch(body, /"C4"|"A#4"|"gizmo"|"tr1"/,
    "the window carries its own copy of the instrument");
});

test("every answer is drawn", () => {
  const body = grab(src, "fluteDialog");
  assert.match(body, /fluteChart\(/);
  const draw = grab(chart, "fluteChart");
  assert.match(draw, /<svg /);
  assert.match(draw, /currentColor/,
    "the chart is drawn in a fixed colour, so one theme or the other loses it");
  // The key that shakes is neither down nor up, so it is neither filled
  // nor plain: it is ringed.
  assert.match(draw, /stroke-dasharray/);
});

// A fingering a flute may not be able to play at all has to say so, and
// there is nothing to draw for it: split E is a mechanism, not a finger.
test("what cannot be drawn is said in words", () => {
  const body = grab(chart, "fluteNeeds");
  assert.match(body, /split E/);
  assert.match(grab(src, "fluteDialog"), /fluteNeeds\(/);
});

test("the two questions are asked separately", () => {
  const body = grab(src, "fluteDialog");
  // One note's alternatives, and a trill, which is a pair of notes.
  assert.match(body, /from: note, to,/);
  assert.match(api, /name: from\b/);
  assert.match(api, /name: to\b/);
  // A trill has nothing to filter by tuning or distance, so the filters
  // go away rather than sitting there doing nothing.
  assert.match(body, /\$\("#flute-filters"\)\.hidden = onTrills/);
  // display: flex beats the hidden attribute, which is how they stayed.
  assert.match(css, /\.fl-filters\[hidden\][^{]*\{ display: none; \}/);
});

test("a long answer is paged rather than poured onto the page", () => {
  const body = grab(src, "fluteDialog");
  assert.match(body, /offset: String\(shown\)/);
  assert.match(body, /#flute-more/);
  assert.match(html, /id="flute-more"/);
});

test("every class the window uses has a rule", () => {
  for (const c of [".fl-card", ".fl-standard", ".fl-modes", ".fl-filters",
                   ".fl-chart", "#flute-notes", "#flute-list"]) {
    assert.match(css, new RegExp(c.replace("#", "#").replace(".", "\\.") +
      "[\\s,.:\\[{]"), "no rule for " + c);
  }
});

/*
 * The paper does not change with the theme, so neither can the ink on it.
 *
 * --ink is the writing on the interface and flips to a light colour in
 * the dark theme; --paper stays near white, because a score is printed on
 * white whatever the screen is set to. Pairing the two gives light grey
 * on white, which is what the fingering charts came out as. Reported as
 * "in dark mode I am not seeing black on white but light grey on white".
 */
test("nothing drawn on paper takes the interface's ink", () => {
  const bad = [];
  for (const m of css.matchAll(/([^{}]+)\{([^}]*)\}/g)) {
    const body = m[2];
    if (!body.includes("background: var(--paper)")) continue;
    if (/color:\s*var\(--ink\)|color:\s*var\(--muted\)/.test(body)) {
      bad.push(m[1].trim().split("\n").pop());
    }
  }
  assert.deepEqual(bad, [],
    "these are drawn on paper and take an ink that flips with the theme");
  // And the paper's own inks are defined in both themes, or the dark one
  // falls back to nothing.
  const dark = css.slice(css.indexOf("prefers-color-scheme: dark"));
  for (const v of ["--paper-ink", "--paper-muted"]) {
    assert.match(css, new RegExp("\\s" + v + ":"), v + " is not defined");
    assert.match(dark, new RegExp("\\s" + v + ":"),
      v + " is not defined for the dark theme");
  }
});

// A label that wraps away from the menu it names is a label naming
// nothing: they travel together.
test("each filter keeps its menu beside it", () => {
  assert.match(html, /<div class="fl-filters"[\s\S]*?<span>\s*<label for="flute-away"/);
  assert.match(css, /\.fl-filters > span \{/);
});
