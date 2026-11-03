/*
 * dialogcancel_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Getting out of a dialog.
 *
 * Every dialog here is a form, and its buttons submit it. A form will not
 * submit while any field in it is invalid — so a number out of range, or
 * one the browser decides is off the step, blocks every button including
 * Cancel. The person is then shut in a window they did not want to be in,
 * with a tooltip saying "Enter a valid value" and no way to agree to
 * nothing.
 *
 * It is worse than it sounds because the trap can be sprung by a field
 * nobody touched: a margin of 16.2 against step="0.5" is invalid before
 * anybody types, so the Page setup dialog could not be cancelled at all.
 *
 * Two rules keep it shut: a button that is not the confirming one skips
 * validation, and a measurement that can take any value says so.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

// Each dialog, with the buttons in it.
function dialogs() {
  const out = [];
  for (const m of html.matchAll(/<dialog id="([^"]+)"[\s\S]*?<\/dialog>/g)) {
    out.push({ id: m[1], body: m[0] });
  }
  return out;
}

test("the dialogs were found", () => {
  assert.ok(dialogs().length > 10, `only ${dialogs().length} dialogs`);
});

// Cancel, Remove, Reset, Clear and Close all mean "do not use what is in
// these fields", so what is in them cannot be allowed to stop them.
test("every way out of a dialog skips the form's validation", () => {
  const bad = [];
  for (const { id, body } of dialogs()) {
    for (const b of body.matchAll(/<button value="([a-z-]+)"[^>]*type="submit"[^>]*>/g)) {
      const [whole, value] = b;
      if (value === "ok" || whole.includes('class="primary"')) continue;
      if (!whole.includes("formnovalidate")) {
        bad.push(`${id} ▸ ${value}`);
      }
    }
  }
  assert.deepEqual(bad, [],
    "buttons a bad field value can block; give them formnovalidate");
});

// And the confirming button keeps it, which is the half that makes the
// rule worth having: that is where a number out of range should be caught.
test("the button that uses the values still checks them", () => {
  const confirming = [...html.matchAll(
    /<button value="ok"[^>]*type="submit"[^>]*>/g)];
  assert.ok(confirming.length > 5, "no confirming buttons found");
  for (const [whole] of confirming) {
    assert.ok(!whole.includes("formnovalidate"),
      "the confirming button stopped checking: " + whole);
  }
});

// A step the field's own starting value fails is a dialog that is invalid
// before anybody touches it. The page margins are the case: they come to
// 16.2 and 16.9 millimetres, which step="0.5" calls wrong.
test("no number field starts out holding a value its own step forbids", () => {
  const bad = [];
  for (const m of html.matchAll(/<input[^>]*type="number"[^>]*>/g)) {
    const tag = m[0];
    const step = /step="([^"]+)"/.exec(tag)?.[1];
    const value = /value="([^"]+)"/.exec(tag)?.[1];
    const min = /min="([^"]+)"/.exec(tag)?.[1];
    if (!step || step === "any" || value === undefined) continue;
    const base = min !== undefined ? Number(min) : 0;
    const n = (Number(value) - base) / Number(step);
    if (Math.abs(n - Math.round(n)) > 1e-9) {
      bad.push(`${/id="([^"]+)"/.exec(tag)?.[1]}: value ${value} against step ${step}`);
    }
  }
  assert.deepEqual(bad, []);
});

// The page margins in particular, since they are filled in from the score
// and are not whole numbers.
test("a measurement that can be anything says step=any", () => {
  for (const id of ["ps-top", "ps-bottom", "ps-left", "ps-right",
                    "ps-width", "ps-height"]) {
    const m = new RegExp(`<input[^>]*id="${id}"[^>]*>`).exec(html);
    assert.ok(m, `no field ${id}`);
    assert.match(m[0], /step="any"/,
      `${id} is filled in from the score and cannot be held to a step`);
  }
});
