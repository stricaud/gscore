/*
 * keymap_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the music goes.
 *
 * Not what the key signature says. A signature is written once and stands
 * until somebody writes another one; the music moves through keys inside it
 * all the time, and that is most of what there is to understand about a
 * piece — the development of a sonata is the signature of the exposition and
 * almost none of its keys.
 *
 * The arithmetic is tested in internal/analysis. What is tested here is the
 * picture: that the whole piece is on one line with each stretch as wide as
 * it is long, that a reading which does not settle is shown as one rather
 * than stated, and that every stretch can be gone to.
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

function panel(d, movements = []) {
  const made = new Map();
  const state = { info: { movements } };
  const scope = {
    state, Math,
    escapeHTML: (s) => String(s),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "", textContent: "" });
      return made.get(sel);
    },
  };
  const names = ["drawKeyMap", "keyColour", "barsLabel", "barsAt",
                 "movementAt", "barNumberOf"];
  const body = names.map((n) => grab(n)).join("\n") + "\nreturn { drawKeyMap };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawKeyMap(d);
  return made;
}

// A piece that opens in C and moves to its dominant halfway.
const moves = {
  bars: 100,
  keys: [
    { fromBar: 0, toBar: 49, key: { tonic: 0, minor: false }, name: "C major",
      fit: { correlation: 0.95, margin: 0.28 }, clear: true },
    { fromBar: 50, toBar: 99, key: { tonic: 7, minor: false }, name: "G major",
      fit: { correlation: 0.93, margin: 0.26 }, clear: true },
  ],
};

test("the whole piece is one line, each stretch as wide as it is long", () => {
  const strip = panel(moves).get("#keymap-strip").innerHTML;
  const widths = [...strip.matchAll(/width:([\d.]+)%/g)].map((m) => Number(m[1]));
  assert.equal(widths.length, 2, "the piece is not drawn as its stretches");
  assert.ok(Math.abs(widths[0] - 50) < 0.01 && Math.abs(widths[1] - 50) < 0.01,
    `two halves came out ${widths.join(" and ")} per cent wide`);
});

// Related keys sit next to each other in colour because the hue follows the
// circle of fifths, which is the thing the picture is for: a move to the
// dominant should look like a small step and a move across the circle like
// a large one.
test("the colours follow the circle of fifths, not the keyboard", () => {
  const hue = (k) => Number(panel({ bars: 1, keys: [k] })
    .get("#keymap-strip").innerHTML.match(/hsl\((\d+)/)[1]);
  const key = (tonic, minor = false) =>
    ({ fromBar: 0, toBar: 0, key: { tonic, minor }, name: "x", clear: true });

  const c = hue(key(0)), g = hue(key(7)), d = hue(key(2));
  assert.equal(Math.abs(g - c), 30, "C and its dominant are not one step apart");
  assert.equal(Math.abs(d - g), 30, "G and its dominant are not either");
  // The neighbour on the keyboard is the far side of the circle.
  assert.equal(Math.abs(hue(key(1)) - c), 210,
    "C and D♭ are next to each other in colour, and they are not neighbours");
  // A minor key keeps its tonic's hue and is told apart by being duller.
  assert.equal(hue(key(9, true)), hue(key(9)), "A minor is not coloured as an A");
});

// A reading that does not settle is shown as one rather than stated. A bare
// stretch of fifths is in a dozen keys at once and naming one of them would
// be a guess told in the voice of a fact.
test("an unsettled stretch is shown as unsettled", () => {
  const made = panel({
    bars: 10,
    keys: [{ fromBar: 0, toBar: 9, key: { tonic: 0 }, name: "C major",
             fit: { correlation: 0.81, margin: 0.04 }, clear: false }],
  });
  assert.match(made.get("#keymap-strip").innerHTML, /keymap-seg unsettled/,
    "the strip states it as plainly as a settled key");
  assert.match(made.get("#keymap-body").innerHTML, /unsettled/,
    "the list does not say so either");
});

// Every stretch can be gone to, because a map nobody can travel on is a
// picture and not a tool.
test("every stretch goes to where it begins", () => {
  const body = panel(moves).get("#keymap-body").innerHTML;
  assert.match(body, /data-bar="0"/, "the first stretch cannot be gone to");
  assert.match(body, /data-bar="50"/, "nor the second");
  assert.match(body, /bars 1–50/, "the bars are not named as they are numbered");
  assert.match(grab("setupKeyMapClicks"), /goToMeasure\(/,
    "clicking a stretch does not go anywhere");
});

// In a piece with movements the bars are numbered from the start of each
// one, which is what is printed over them.
test("the bars are numbered the way the page numbers them", () => {
  const made = panel({
    bars: 200,
    keys: [{ fromBar: 140, toBar: 160, key: { tonic: 1, minor: true },
             name: "C♯ minor", fit: { margin: 0.3 }, clear: true }],
  }, [
    { first: 0, last: 68, title: "Movement I" },
    { first: 69, last: 132, title: "Movement II" },
    { first: 133, last: 199, title: "Movement III" },
  ]);
  assert.match(made.get("#keymap-body").innerHTML, /bars 8–28/,
    "bar 141 of the file is the Presto's eighth, which is what is printed over it");
});

test("the head says how far the piece travels", () => {
  assert.match(panel(moves).get("#keymap-where").textContent,
    /100 bars, through 2 keys/);
  const still = { bars: 40, keys: [{ ...moves.keys[0], toBar: 39 }] };
  assert.match(panel(still).get("#keymap-where").textContent, /never leaves C major/);
});

test("too little music to read a key from says so", () => {
  const made = panel({ bars: 0, keys: [] });
  assert.match(made.get("#keymap-body").innerHTML, /not enough music/);
  assert.equal(made.get("#keymap-strip").innerHTML, "");
});

test("the window holds still, closes from the top, and waits to be asked", () => {
  const dialog = html.match(/<dialog id="keymap">[\s\S]*?<\/dialog>/)[0];
  assert.ok(dialog.indexOf('class="dlg-close"') < dialog.indexOf('id="keymap-body"'),
    "the close button is inside the part that scrolls");
  assert.match(css.match(/#keymap \{[\s\S]*?\}/)[0], /height:\s*min\(/,
    "the window grows to whatever is in it");
  assert.match(css, /#keymap\[open\] \{ display: flex; \}/,
    "the window is on the screen before anybody asks for it");
  assert.match(html, /data-action="keymap"/, "there is no menu item for it");
  assert.match(src, /case "keymap": await openKeyMap\(\)/,
    "the menu item does not open it");
});
