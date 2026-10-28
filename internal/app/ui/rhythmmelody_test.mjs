/*
 * rhythmmelody_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the notes fall against the beat, and what each line is made of.
 *
 * Both are descriptions and not marks. Syncopation is not a fault, it is
 * most of what makes music move; a line of leaps is a fanfare and not a
 * failed tune. What the panels have to do is say which one you have written
 * — and keep the two real checks, a bar that does not add up and a passage
 * nobody can articulate, apart from the describing.
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

function run(names, entry, d, boxes = {}) {
  const made = new Map();
  const scope = {
    Math,
    escapeHTML: (s) => String(s),
    barNumberOf: (n) => n + 1,
    mapColumns: 360, mapLevels: 5,
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "", textContent: "" });
      return made.get(sel);
    },
    ...boxes,
  };
  const body = names.map((n) => grab(n)).join("\n") + `\nreturn { ${entry} };`;
  new Function(...Object.keys(scope), body)(...Object.values(scope))[entry](d);
  return made;
}

const rhythmNames = ["drawRhythm", "beatRow", "oddRow", "rushedRow", "maxOf", "mapStrip"];
const melodyNames = ["drawMelody", "shapeRow", "awkwardRow", "intervalWord"];

const square = {
  bars: 8,
  pulses: [{ part: 0, name: "Piano", notes: 100, downbeats: 25, onBeat: 75,
             offBeat: 0, syncopated: 0, bars: new Array(8).fill(0) }],
  odd: [], rushed: [],
};

test("where the notes fall is shown as a share of the line, not a count alone", () => {
  const body = run(rhythmNames, "drawRhythm", square).get("#rhythm-body").innerHTML;
  assert.match(body, /25% on the downbeat/, "the shares are not given");
  const widths = [...body.matchAll(/width:([\d.]+)%/g)].map((m) => Number(m[1]));
  assert.ok(Math.abs(widths[0] - 25) < 0.01 && Math.abs(widths[1] - 75) < 0.01,
    `the bar came out ${widths.join(" and ")} per cent`);
});

// Syncopation is not a fault and the panel must not call it one.
test("syncopation is described, never blamed", () => {
  const body = run(rhythmNames, "drawRhythm", {
    ...square,
    pulses: [{ ...square.pulses[0], offBeat: 40, onBeat: 35,
               syncopated: 12, bars: [0, 3, 4, 2, 0, 0, 3, 0] }],
  }).get("#rhythm-body").innerHTML;
  assert.match(body, /12 syncopated/, "it is not counted at all");
  for (const word of [/\berror\b/i, /\bwrong\b/i, /\bavoid\b/i, /\bshould\b/i]) {
    assert.ok(!word.test(body), `the panel says ${word} about syncopation`);
  }
  // And where it happens is drawn across the piece.
  assert.match(body, /map-busy/, "where it pulls against the metre is not drawn");
});

// The two checks are kept apart from the description, under their own
// headings, because they are a different kind of claim.
test("the checks are kept apart from the description", () => {
  const body = run(rhythmNames, "drawRhythm", {
    ...square,
    odd: [{ bar: 66, held: 1.25, says: 1, text: "this bar holds more than 4/4 says" }],
    rushed: [{ part: 0, name: "Tuba", fromBar: 320, toBar: 320, notes: 8,
               each: 0.032, limit: 0.095, ref: 9 }],
  }).get("#rhythm-body").innerHTML;

  assert.ok(body.indexOf("where the notes fall") < body.indexOf("do not add up"),
    "the checks come before the description");
  assert.match(body, /bars that do not add up/);
  assert.match(body, /faster than it can be played/);
  assert.match(body, /32 milliseconds/, "it does not say how short the notes are");
  assert.match(body, /95 to speak/, "nor what the instrument needs");
  assert.match(body, /data-bar="66"/, "a bar that does not add up cannot be gone to");
  assert.match(body, /data-bar="320"/, "nor a passage too fast to play");
});

test("a score with nothing in it says so rather than drawing nothing", () => {
  assert.match(run(rhythmNames, "drawRhythm", { bars: 0, pulses: [] })
    .get("#rhythm-body").innerHTML, /nothing written yet/);
});

const tune = {
  shapes: [{ line: 0, name: "soprano", notes: 100, moves: 99,
             same: 10, steps: 70, leaps: 19, widest: 7, widestBar: 12,
             commonest: 2, up: 50, down: 49 }],
  awkward: [],
};

test("a line says what it is made of, and whether it can be hummed", () => {
  const body = run(melodyNames, "drawMelody", tune).get("#melody-body").innerHTML;
  assert.match(body, /soprano/, "the line is not named");
  assert.match(body, /mostly steps — singable/, "it does not say what kind of line it is");
  assert.match(body, /71% steps/, "nor how it breaks down");
  assert.match(body, /mostly a tone/, "the commonest interval is not named in words");
  assert.match(body, /widest a fifth in bar 13/, "nor the widest move and where");
});

test("a line of leaps is named as one, not failed", () => {
  const body = run(melodyNames, "drawMelody", {
    shapes: [{ ...tune.shapes[0], same: 2, steps: 8, leaps: 89 }], awkward: [],
  }).get("#melody-body").innerHTML;
  assert.match(body, /mostly leaps/);
  for (const word of [/\berror\b/i, /\bwrong\b/i, /\bshould\b/i, /\bunsingable\b/i,
                      /\bbad\b/i, /\bavoid\b/i]) {
    assert.ok(!word.test(body), `the panel says ${word} about a line of leaps`);
  }
});

test("a move worth a second look says what, where, and how often", () => {
  const body = run(melodyNames, "drawMelody", {
    ...tune,
    awkward: [{ kind: "augmented second", name: "alto", bar: 18, toBar: 18, times: 2,
                ref: 44, semitones: 3,
                text: "an augmented second, D4 to E#4 — the step singers have complained about" }],
  }).get("#melody-body").innerHTML;
  assert.match(body, /D4 to E#4/, "it does not say which notes");
  assert.match(body, /alto/, "nor which line");
  assert.match(body, /bar 19, 2 times/, "nor where, or how often");
  assert.match(body, /data-bar="18"/, "and it cannot be gone to");
});

test("both windows hold still, close from the top, and wait to be asked", () => {
  for (const id of ["rhythm", "melody"]) {
    const dialog = html.match(new RegExp(`<dialog id="${id}">[\\s\\S]*?</dialog>`))[0];
    assert.ok(dialog.indexOf('class="dlg-close"') < dialog.indexOf(`id="${id}-body"`),
      `${id}: the close button is inside the part that scrolls`);
    assert.match(html, new RegExp(`data-action="${id}"`), `${id}: there is no menu item`);
  }
  assert.match(css, /#rhythm\[open\], #melody\[open\] \{ display: flex; \}/,
    "a window is on the screen before anybody asks for it");
  assert.match(css.match(/#rhythm, #melody \{[\s\S]*?\}/)[0], /height:\s*min\(/,
    "a window grows to whatever is in it");
  assert.match(src, /case "rhythm": await openRhythm\(\)/);
  assert.match(src, /case "melody": await openMelody\(\)/);
});
