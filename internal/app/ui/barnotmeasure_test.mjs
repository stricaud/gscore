/*
 * barnotmeasure_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * One word for it, and the word is bar.
 *
 * A bar and a measure are the same thing; "bar" is what British English
 * calls it and "measure" is what American English calls it. gscore is
 * written in British English — colour, grey, neighbour, recognise, all of
 * them, nowhere near a close thing — so it is a bar.
 *
 * It was both. Edit went to a measure while the status line said bar; the
 * View menu showed "Measure numbers" and the Score menu set "Bar numbers";
 * the same command was called "Go to measure…" in the menu and "Go to a
 * bar" in the shortcuts window. Each one on its own is nothing. Together
 * they are a program that cannot make up its mind, and somebody looking
 * for a command has to guess which half of it they are in.
 *
 * So this test reads what the person reads — menu labels, the status
 * line, dialogs, the French dictionary's English side — and fails on the
 * other word. The names underneath are left alone on purpose: the score
 * model, the endpoints and the selection grammar all say measure, and
 * renaming those would break every saved script and every link without
 * changing a word anybody reads.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const here = (f) => readFileSync(new URL("./" + f, import.meta.url), "utf8");
const html = here("index.html");
const js = here("app.js");
const i18n = here("i18n.js");

// Not every "measure" is the musical one.
//
//   measurement, measured  — engraving sizes, and measuring a glyph
//   measure(bar)           — a verb of the script language, named in the
//                            console's help; bar() already means something
//                            else there, so it cannot be renamed
//   /api/…, measure=       — endpoint and query names
const ALLOWED = /measurement|measured|measures\(|measure\(|\/api\/|measure=|#tempo-measure/;

// A string the person reads, as against a name the program uses: prose has
// spaces in it. "add-measure" is an action id, "Click a measure first" is a
// sentence somebody is shown.
//
// What is inside ${…} of a template is code and not prose — sel.measure is
// a field of the cursor and barNumberOf(hit.measure) is a call — so it
// comes out before the words are read. That is also the line the rename
// itself had to hold: the sentence says bar, the field it reads stays
// measure, and they sit in the same backticks.
function offendingProse(src) {
  const literal = /"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|`(?:[^`\\]|\\.)*`/gs;
  const bad = [];
  for (const m of src.match(literal) || []) {
    const words = m.replace(/\$\{[^}]*\}/g, " ");
    if (!/\bmeasures?\b/i.test(words)) continue;
    if (ALLOWED.test(words)) continue;
    if (!/\s/.test(m)) continue; // one word, so a name and not a sentence
    bad.push(m.slice(0, 90));
  }
  return bad;
}

test("no menu entry says measure", () => {
  const labels = [
    ...[...html.matchAll(/data-action="[^"]*"[^>]*><span[^>]*>([^<]+)</g)],
    ...[...html.matchAll(/class="submenu-title"><span>([^<]+)</g)],
    ...[...html.matchAll(/class="menu-title">([^<]+)</g)],
  ].map((m) => m[1]);
  const bad = labels.filter((l) => /\bmeasures?\b/i.test(l));
  assert.deepEqual(bad, [], "menu entries saying measure rather than bar");
});

test("nothing the editor says out loud says measure", () => {
  assert.deepEqual(offendingProse(js), [],
                   "status lines or dialogs saying measure rather than bar");
});

test("no text on the page says measure", () => {
  // The markup outside the tags: dialog prose, help, headings.
  const text = html.replace(/<!--[\s\S]*?-->/g, "").replace(/<[^>]*>/g, " ");
  const bad = [...text.matchAll(/[^.!?]*\bmeasures?\b[^.!?]*/gi)]
    .map((m) => m[0].trim().replace(/\s+/g, " "))
    .filter((s) => !ALLOWED.test(s));
  assert.deepEqual(bad, [], "words on the page saying measure rather than bar");
});

// The dictionary is keyed by the English, so a key left behind is a label
// that has quietly lost its French.
test("the French dictionary is keyed on the same word", () => {
  const keys = [...i18n.matchAll(/^\s*"((?:[^"\\]|\\.)*)":\s*"/gm)].map((m) => m[1]);
  const bad = keys.filter((k) => /\bmeasures?\b/i.test(k) && !ALLOWED.test(k));
  assert.deepEqual(bad, [], "English keys saying measure rather than bar");
});

// And every menu label still has a French one, which is the half of a
// rename that is easy to forget: the label changes, the key does not, and
// the French silently stops being used.
test("every menu label still has its French", () => {
  const keys = new Set(
    [...i18n.matchAll(/^\s*"((?:[^"\\]|\\.)*)":\s*"/gm)].map((m) => m[1]));
  const labels = [
    ...[...html.matchAll(/data-action="[^"]*"[^>]*><span[^>]*>([^<]+)</g)],
    ...[...html.matchAll(/class="submenu-title"><span>([^<]+)</g)],
    ...[...html.matchAll(/class="menu-title">([^<]+)</g)],
  ].map((m) => m[1]);
  // A file format and a dynamic are the same word in every language.
  const needsNone = /^(?:[pmf]{1,4}|PDF…|SVG…|MIDI…|ABC…|MusicXML…|MusicXML, compressed…|LilyPond…|GUIDO…)$/;
  const missing = [...new Set(labels)]
    .filter((l) => !needsNone.test(l) && !keys.has(l));
  assert.deepEqual(missing, [], "menu labels with no French");
});
