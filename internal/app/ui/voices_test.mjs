/*
 * voices_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How the lines move against each other.
 *
 * The one panel that judges rather than measures, so most of what is tested
 * here is its tone. Parallel fifths are not wrong; they are wrong in a
 * tradition, and they are the whole sound of organum, of Debussy and of
 * every power chord ever played. A panel that said "error" would be saying
 * something false about every one of those, and a composer who was told it
 * once would never open the panel again.
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
const konstStr = (name) => {
  const m = src.match(new RegExp("^const " + name + " =([\\s\\S]*?);$", "m"));
  if (!m) throw new Error("app.js has no " + name);
  // Wrapped in brackets: a value starting on the next line is
  // returned as undefined, because a bare return ends there.
  return new Function("return (" + m[1] + ")")();
};

function panel(d) {
  const made = new Map();
  const scope = {
    escapeHTML: (s) => String(s),
    barNumberOf: (n) => n + 1,
    voicesTradition: konstStr("voicesTradition"),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "", textContent: "" });
      return made.get(sel);
    },
  };
  const names = ["drawVoiceLeading", "voiceLeadingRow"];
  const body = names.map((n) => grab(n)).join("\n") + "\nreturn { drawVoiceLeading };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawVoiceLeading(d);
  return made;
}

const fifths = {
  lines: 4, names: ["soprano", "alto", "tenor", "bass"],
  motions: [{
    kind: "parallel fifths", text: "soprano and bass move in parallel fifths",
    fromBar: 12, toBar: 12, times: 1, upper: "soprano", lower: "bass", ref: 91,
  }],
};

// The tradition is named, once, at the top. A rule applied without saying
// which tradition it belongs to is a rule pretending to be a fact.
test("it says what it is judging by, before it judges anything", () => {
  const body = panel(fifths).get("#voices-body").innerHTML;
  assert.match(body, /Palestrina/, "the tradition is not named");
  assert.ok(body.indexOf("Palestrina") < body.indexOf("parallel fifths"),
    "it judges before it says what it judges by");
  assert.match(body, /Debussy|power chord/,
    "it does not say where the rule does not apply");
});

// Never "error", never "wrong", never "mistake". The word decides whether
// the panel is a colleague or a marking scheme.
test("no finding is called an error", () => {
  // The findings, not the paragraph above them: that one is allowed to use
  // the word, because what it says with it is "nothing here is an error".
  const body = panel(fifths).get("#voices-body").innerHTML
    .replace(/<p class="voices-tradition">[\s\S]*?<\/p>/, "");
  for (const word of [/\berror\b/i, /\bwrong\b/i, /\bmistake\b/i,
                      /\billegal\b/i, /\bviolation\b/i, /\bforbidden\b/i,
                      /\bshould\b/i, /\bmust\b/i]) {
    assert.ok(!word.test(body), `a finding says ${word}`);
  }
  // And the words the core puts in them do not either.
  const go = readFileSync(new URL("../../analysis/voices.go", import.meta.url), "utf8");
  const sentences = [...go.matchAll(/add\("[^"]+",\s*\n?\s*([\s\S]*?)\n\s*[a-z]\w*\.Bar/g)]
    .map((m) => m[1]).join(" ");
  for (const word of [/\berror\b/i, /\bwrong\b/i, /\bshould\b/i]) {
    assert.ok(!word.test(sentences), `the core writes ${word} into a finding`);
  }
});

test("a finding says what, where, and between which lines", () => {
  const body = panel(fifths).get("#voices-body").innerHTML;
  assert.match(body, /parallel fifths/, "what it is");
  assert.match(body, /soprano and bass/, "which lines");
  assert.match(body, /bar 13/, "which bar, as the page numbers it");
  assert.match(body, /data-bar="12"/, "and it cannot be gone to");
  assert.match(grab("setupVoiceLeadingClicks"), /goToMeasure\(/,
    "clicking a finding goes nowhere");
});

// A left hand in octaves does the same thing in every bar for pages, and
// the count is what says so rather than three hundred separate findings.
test("a run says how far it runs and how often", () => {
  const body = panel({
    lines: 2, motions: [{
      kind: "parallel octaves", text: "Staff 2 and Staff 2, note 2 down move in parallel octaves",
      fromBar: 1, toBar: 20, times: 26, upper: "a", lower: "b", ref: 5,
    }],
  }).get("#voices-body").innerHTML;
  assert.match(body, /bars 2–21, 26 times/,
    "a run of findings does not say how far it runs or how often");
});

test("a single finding is not given a count", () => {
  assert.ok(!/1 times/.test(panel(fifths).get("#voices-body").innerHTML),
    "one occurrence was counted at you");
});

test("nothing to report says nothing to report", () => {
  const made = panel({ lines: 4, motions: [] });
  assert.match(made.get("#voices-body").innerHTML, /Nothing to report/);
  assert.match(made.get("#voices-where").textContent, /nothing to look at/);
});

// The rules are about a chorale, a quartet, a keyboard piece. A symphony
// would be thousands of findings about instruments that never hear each
// other, which is a way of saying nothing at great length.
test("a score too large for the rules is told so, and why", () => {
  const made = panel({ lines: 61, tooMany: 16, motions: [] });
  const body = made.get("#voices-body").innerHTML;
  assert.match(body, /61 lines/, "it does not say how many lines there are");
  assert.match(body, /16 or fewer/, "nor what it can manage");
  assert.match(body, /Palestrina/, "the tradition is not named even here");
  assert.match(made.get("#voices-where").textContent, /too many/);
});

test("the window holds still, closes from the top, and waits to be asked", () => {
  const dialog = html.match(/<dialog id="voices">[\s\S]*?<\/dialog>/)[0];
  assert.ok(dialog.indexOf('class="dlg-close"') < dialog.indexOf('id="voices-body"'),
    "the close button is inside the part that scrolls");
  assert.match(css.match(/#voices \{[\s\S]*?\}/)[0], /height:\s*min\(/,
    "the window grows to whatever is in it");
  assert.match(css, /#voices\[open\] \{ display: flex; \}/,
    "the window is on the screen before anybody asks for it");
  assert.match(html, /data-action="voices"/, "there is no menu item for it");
  assert.match(src, /case "voices": await openVoiceLeading()/,
    "the menu item does not open it");
});
