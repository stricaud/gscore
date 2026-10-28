/*
 * spacing_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How low the bottom of the harmony is written.
 *
 * The roughness of the harmonics panel pointed outward: that one answers "is
 * this chord rough" with a number, and an orchestrator is asking the same
 * question the other way round and wants it against a bar.
 *
 * Advice and never a fault. A piano's left hand does this all day and is
 * fine, and a composer writing a low third in the bassoons may want exactly
 * that rumble — so the panel is tested for saying so as much as for what it
 * draws.
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
const konst = (name) => {
  const m = src.match(new RegExp("^const " + name + " =([\\s\\S]*?);$", "m"));
  if (!m) throw new Error("app.js has no " + name);
  // Bracketed: a value starting on the next line comes back undefined,
  // because a bare return ends at the line break.
  return new Function("return (" + m[1] + ")")();
};

function panel(d) {
  const made = new Map();
  const scope = {
    Math,
    escapeHTML: (s) => String(s),
    barNumberOf: (n) => n + 1,
    spacingNote: konst("spacingNote"),
    spacingDeepest: konst("spacingDeepest"),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "", textContent: "" });
      return made.get(sel);
    },
  };
  const body = [grab("drawSpacing"), grab("spacingRow")].join("\n") +
    "\nreturn { drawSpacing };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawSpacing(d);
  return made;
}

const low = {
  thick: [{
    text: "a major third on D2, which is usually written no lower than A♯2",
    fromBar: 264, toBar: 264, times: 5,
    low: 38, high: 42, semitones: 4, limit: 46, below: 8, ref: 4648,
  }],
};

// Said before anything is judged, the way the voice-leading panel names its
// tradition: a guideline applied without saying it is one is a guideline
// pretending to be a fact.
test("it says it is advice, and where it does not apply", () => {
  const body = panel(low).get("#spacing-body").innerHTML;
  assert.match(body, /guideline/, "it does not say what kind of thing it is");
  assert.match(body, /piano's left hand/,
    "it does not say where the advice does not apply");
  assert.ok(body.indexOf("guideline") < body.indexOf("major third"),
    "it judges before it says what it is judging by");
});

test("no finding is called a fault", () => {
  const body = panel(low).get("#spacing-body").innerHTML
    .replace(/<p class="voices-tradition">[\s\S]*?<\/p>/, "");
  for (const word of [/\berror\b/i, /\bwrong\b/i, /\bmust\b/i, /\bshould\b/i,
                      /\bbad\b/i, /\bavoid\b/i]) {
    assert.ok(!word.test(body), `a finding says ${word}`);
  }
});

test("a finding says what, where, and how far under", () => {
  const body = panel(low).get("#spacing-body").innerHTML;
  assert.match(body, /major third on D2/, "what the interval is");
  assert.match(body, /no lower than A♯2/, "and where it is usually written");
  assert.match(body, /bar 265, 5 times/, "where, and how often");
  assert.match(body, /data-bar="264"/, "it cannot be gone to");
  assert.match(grab("setupSpacingClicks"), /goToMeasure\(/,
    "clicking a finding goes nowhere");
});

// The difference between one semitone under and ten is the whole of the
// advice, so it is drawn rather than left as a number to read.
test("how far under is drawn, and deeper is wider", () => {
  const width = (below) => {
    const html = panel({ thick: [{ ...low.thick[0], below }] })
      .get("#spacing-body").innerHTML;
    return Number(html.match(/width:(\d+)%/)[1]);
  };
  assert.ok(width(2) < width(8), "two semitones under is drawn as deep as eight");
  assert.equal(width(konst("spacingDeepest") * 2), 100,
    "very far under runs off the end of the bar");
  const body = panel(low).get("#spacing-body").innerHTML;
  assert.match(body, /title="8 semitones below the usual limit"/,
    "the number is not there for anybody who wants it");
});

test("nothing held low says so", () => {
  const made = panel({ thick: [] });
  assert.match(made.get("#spacing-body").innerHTML, /Nothing held low enough/);
  assert.match(made.get("#spacing-where").textContent, /Nothing to listen to/);
});

test("the head counts what there is", () => {
  assert.match(panel(low).get("#spacing-where").textContent, /1 place to listen to/);
});

test("the window holds still, closes from the top, and waits to be asked", () => {
  const dialog = html.match(/<dialog id="spacing">[\s\S]*?<\/dialog>/)[0];
  assert.ok(dialog.indexOf('class="dlg-close"') < dialog.indexOf('id="spacing-body"'),
    "the close button is inside the part that scrolls");
  assert.match(css.match(/#spacing \{[\s\S]*?\}/)[0], /height:\s*min\(/,
    "the window grows to whatever is in it");
  assert.match(css, /#spacing\[open\] \{ display: flex; \}/,
    "the window is on the screen before anybody asks for it");
  assert.match(html, /data-action="spacing"/, "there is no menu item for it");
  assert.match(src, /case "spacing": await openSpacing\(\)/,
    "the menu item does not open it");
});
