/*
 * range_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The range panel: what it says, and what it does not.
 *
 * Two kinds of answer come back from the core and the panel has to keep them
 * apart. A note outside the instrument altogether is a fact: it will not be
 * played. A part sitting above the comfortable stretch is a judgement: the
 * top of a trumpet part is where the piece arrives, and that is the point of
 * it. Run together, the facts become arguable and the judgements become
 * binding, and both of those are worse than saying nothing.
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
  const el = () => ({ textContent: "", innerHTML: "" });
  const scope = {
    Math,
    escapeHTML: (s) => String(s),
    barNumberOf: (n) => n + 1,
    rangeLowest: konst("rangeLowest"),
    rangeHighest: konst("rangeHighest"),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, el());
      return made.get(sel);
    },
  };
  const names = ["drawRange", "rangeStaff", "rangeStrip", "rangeFindings", "midiName"];
  const body = names.map((n) => grab(n)).join("\n") + "\nreturn { drawRange };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawRange(d);
  return made;
}

const flute = {
  part: 0, name: "Flauto 1", instrument: "flute", notes: 900,
  lowest: { midi: 62, name: "D4", bar: 3, ref: 11 },
  highest: { midi: 93, name: "A6", bar: 2405, ref: 9001 },
  tessitura: { low: 72, high: 86 },
  playable: { low: 60, high: 98 },
  easy: { low: 62, high: 91 },
  findings: [{
    severity: "strain",
    text: "13 bars above G6, the top of the flute's comfortable range",
    fromBar: 2393, toBar: 2405, notes: 13,
    worst: { midi: 93, name: "A6", bar: 2399, ref: 9000 },
  }],
};

test("a staff says what it covers and where it sits", () => {
  const body = panel({ parts: [flute] }).get("#range-body").innerHTML;
  assert.match(body, /Flauto 1/, "the staff is not named");
  assert.match(body, /flute/, "the instrument is not named");
  assert.match(body, /D4 to A6/, "the two ends are not given");
  assert.match(body, /sitting/, "where the part lives is not given");
});

// The two kinds of finding are marked differently, because they are
// different kinds of claim.
test("a judgement is offered and a fact is stated", () => {
  const strained = panel({ parts: [flute] }).get("#range-body").innerHTML;
  assert.match(strained, /range-strain/, "a strain is not marked as one");
  assert.match(strained, /worth a look/,
    "a judgement about the writing is put as a ruling");

  const hard = JSON.parse(JSON.stringify(flute));
  hard.findings[0].severity = "impossible";
  hard.findings[0].text = "2 notes above D7, the top of the flute's range";
  const out = panel({ parts: [hard] }).get("#range-body").innerHTML;
  assert.match(out, /range-impossible/, "a fact is not marked as one");
  assert.match(out, /cannot be played/, "a fact about the instrument is hedged");
});

// Every finding points at a bar, because a finding nobody can go to is a
// finding nobody can act on.
test("a finding can be gone to, on its own staff", () => {
  const body = panel({ parts: [flute] }).get("#range-body").innerHTML;
  assert.match(body, /data-part="0"/, "the finding does not say which staff");
  assert.match(body, /data-bar="2399"/,
    "it does not point at the worst bar in the stretch");
  assert.match(body, /bars 2394–2406/, "it does not say which bars, as they are numbered");
});

test("the head counts what there is to look at", () => {
  const quiet = panel({ parts: [{ ...flute, findings: [] }] });
  assert.match(quiet.get("#range-where").textContent, /nothing out of reach/);
  const noisy = panel({ parts: [flute] });
  assert.match(noisy.get("#range-where").textContent, /1 thing to look at/);
});

// A staff of drums is read by position on a kit, not by pitch.
test("a drum staff is not given a range at all", () => {
  const body = panel({
    parts: [{ part: 0, name: "Gran Tamburo", instrument: "drum kit", percussion: true }],
  }).get("#range-body").innerHTML;
  assert.match(body, /Gran Tamburo/, "the staff is not even listed");
  assert.match(body, /places on a kit/, "it is not said why there is nothing to say");
  assert.ok(!/range-strip/.test(body), "a pitch range was drawn for a drum");
});

test("an empty staff is listed and left alone", () => {
  const body = panel({
    parts: [{ part: 0, name: "Oboe 2", instrument: "oboe", notes: 0 }],
  }).get("#range-body").innerHTML;
  assert.match(body, /Oboe 2/);
  assert.match(body, /Nothing written/);
});

// An instrument gscore does not know gets the range the part covers and no
// opinion: the strip is drawn without a track behind it.
test("an unknown instrument is drawn without a range to judge it by", () => {
  const body = panel({
    parts: [{
      part: 0, name: "Theremin", instrument: "program 100", notes: 5,
      lowest: { midi: 40, name: "E2", bar: 0, ref: 1 },
      highest: { midi: 90, name: "F♯6", bar: 4, ref: 9 },
      tessitura: { low: 50, high: 70 }, findings: [],
    }],
  }).get("#range-body").innerHTML;
  assert.match(body, /range-strip/, "the part's own range was not drawn");
  assert.ok(!/fill-opacity=".1[26]"/.test(body),
    "a range was drawn for an instrument gscore has none for");
});

test("the window holds still and closes from the top", () => {
  const dialog = html.match(/<dialog id="range">[\s\S]*?<\/dialog>/)[0];
  assert.ok(dialog.indexOf('class="dlg-close"') < dialog.indexOf('id="range-body"'),
    "the close button is inside the part that scrolls");
  const rule = css.match(/#range \{[\s\S]*?\}/)[0];
  assert.match(rule, /height:\s*min\(/, "the window grows to whatever is in it");
  assert.match(css, /#range\[open\] \{ display: flex; \}/,
    "the window is displayed whether or not it was asked for");
});

test("the panel is only ever opened by asking for it", () => {
  assert.match(src, /case "range": await openRange\(\)/,
    "the menu item does not open it");
  assert.equal([...src.matchAll(/openRange\(\)/g)].length, 2,
    "something other than the menu item opens the panel");
  assert.match(html, /data-action="range"/, "there is no menu item for it");
});
