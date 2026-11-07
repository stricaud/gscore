/*
 * textstyle_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Choosing what kind of writing a piece of text is, and whether the
 * metronome marking is shown.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// The kinds come from the program. Two lists of them is one list too many:
// the second is where an expression marking stops being italic in one
// place and goes on being italic in the other.
test("the kinds are asked for, not written down again here", () => {
  const body = grab("textDialog");
  assert.match(body, /await fetch\("\/api\/textstyles"\)/);
  assert.doesNotMatch(body, /italic: true|"expression".*italic/,
    "the editor is deciding how a kind looks");
});

test("what a kind means is said in words a reader would use", () => {
  const body = grab("textDialog");
  // Where it goes and who it is for, rather than a list of font settings.
  assert.match(body, /Over the whole system, for everybody/);
  assert.match(body, /Under the staff/);
  assert.match(body, /Over the staff/);
});

// Somebody putting the expression markings into a piece writes several in
// a row.
test("the kind chosen last time is offered again", () => {
  assert.match(src, /let lastTextStyle = "expression"/);
  assert.match(grab("textDialog"), /lastTextStyle/);
  assert.match(grab("textDialog"), /lastTextStyle = pick\.value/);
});

// Words picked up by clicking them open in the window as they are, or
// changing a text means writing it again from nothing and guessing which
// kind it was.
test("words being changed open on what they say", () => {
  const body = grab("textDialog");
  assert.match(body, /state\.selection\?\.textMark/);
  assert.match(body, /\$\("#tm-words"\)\.value = editing \? editing\.text/);
  assert.match(body, /\$\("#tm-place"\)\.value = editing/);
  // And they are changed rather than added to: a note may carry two.
  assert.match(body, /p\.set\("index", String\(editing\.index\)\)/);
});

// Which side of the staff, where the music wants one mark to differ from
// the rest of its kind.
test("the window can say which side of the staff", () => {
  assert.match(html, /<dialog id="textmark"[\s\S]*?id="tm-place"/);
  const dlg = /<dialog id="textmark"[\s\S]*?<\/dialog>/.exec(html)[0];
  for (const v of ["above", "below"]) {
    assert.ok(dlg.includes(`value="${v}"`), `no ${v} choice`);
  }
  assert.match(grab("textDialog"), /placement: \$\("#tm-place"\)\.value/);
});

test("the dialog can remove the words as well as write them", () => {
  assert.match(html, /<dialog id="textmark"[\s\S]*?value="remove"/);
  assert.match(grab("textDialog"), /answer === "remove"/);
});

test("cancelling writes nothing", () => {
  const body = grab("textDialog");
  const cancel = body.indexOf('answer === "cancel"');
  const send = body.indexOf('fetch("/api/text?');
  assert.ok(cancel > 0 && send > cancel,
    "the request is made before the answer is looked at");
});

test("the dialog's way out skips validation", () => {
  const dlg = /<dialog id="textmark"[\s\S]*?<\/dialog>/.exec(html)[0];
  for (const v of ["cancel", "remove"]) {
    const m = new RegExp(`<button value="${v}"[^>]*>`).exec(dlg);
    assert.ok(m, `no ${v} button`);
    assert.match(m[0], /formnovalidate/);
  }
});

/* ---------- the metronome marking ---------- */

test("the metronome marking can be shown and hidden", () => {
  const m = html.match(/data-action="view-tempomark"[^>]*>/);
  assert.ok(m, "no menu entry");
  assert.ok(!m[0].includes("disabled"));
  assert.match(src, /case "view-tempomark": await setView\("tempomark"/);
  assert.ok(fr.includes('"Metronome marking":'), "no French");
});

// A tick that starts out lying is worse than no tick: what the editor
// believes has to match what the engraver does untold.
test("the tick starts out saying what the engraver actually does", () => {
  assert.match(src, /view: \{[^}]*tempomark: true/s);
});

test("the tick follows the answer rather than the click", () => {
  const body = grab("setView");
  assert.match(body, /state\.view\.tempomark = !!d\.tempomark/);
  const ticks = grab("markViewTicks");
  assert.match(ticks, /tempomark-tick/);
});

// Hiding it changes the drawing and not the music, and the status line
// says so — otherwise it reads as having changed the speed.
test("hiding it says the piece still plays at that speed", () => {
  assert.match(grab("setView"), /still plays at that speed/);
});
