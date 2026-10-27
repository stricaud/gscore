/*
 * askdefault_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What Enter presses in a dialog that asks a question.
 *
 * A form's default button is its first submit button, and in every dialog in
 * this program that was Cancel. So "Go to measure" took a number, took the
 * Enter after it, and did nothing at all — and so did every other question
 * the program asks. The button Enter should press is written first now, and
 * the stylesheet puts it back on the right.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const js = readFileSync(new URL("./app.js", import.meta.url), "utf8");

// Every run of dialog buttons in the file, with the values in writing order.
function buttonRuns() {
  const runs = [];
  const re = /<(form|div)[^>]*class="ask-buttons"[^>]*>([\s\S]*?)<\/\1>/g;
  let m;
  while ((m = re.exec(html))) {
    const values = [...m[2].matchAll(/<button\s+value="([^"]+)"[^>]*type="submit"/g)]
      .map((b) => b[1]);
    if (values.length > 1) runs.push(values);
  }
  return runs;
}

test("Enter presses the button that does the thing, not Cancel", () => {
  const runs = buttonRuns();
  assert.ok(runs.length >= 6, `only found ${runs.length} rows of dialog buttons`);
  for (const values of runs) {
    const first = values[0];
    // Where there is something to confirm, Enter confirms it. Where there is
    // not — a dialog you pick from rather than type into — Enter closes it.
    const want = values.includes("ok") ? "ok" : "cancel";
    assert.equal(first, want,
      `a dialog whose buttons are ${values.join(", ")} has Enter pressing "${first}"`);
  }
});

test("the stylesheet puts the buttons back in their usual order", () => {
  for (const sel of ['button[value="remove"]', 'button[value="cancel"]', "button.primary"]) {
    assert.ok(css.includes(".ask-buttons " + sel),
      `style.css does not order ${sel}, so writing it first would move it`);
  }
  // Ascending, so the primary still reads last.
  const order = (sel) => {
    const m = css.match(new RegExp("\\.ask-buttons " + sel.replace(/[.[\]"]/g, "\\$&") + "\\s*\\{[^}]*order:\\s*(\\d+)"));
    return m ? Number(m[1]) : NaN;
  };
  const remove = order('button[value="remove"]');
  const cancel = order('button[value="cancel"]');
  const primary = order("button.primary");
  assert.ok(remove < cancel && cancel < primary,
    `the buttons are ordered ${remove}, ${cancel}, ${primary}`);
});

test("a box that opens with something in it opens with it selected", () => {
  const m = js.match(/^function ask\([\s\S]*?^}/m);
  assert.ok(m, "app.js has no ask");
  assert.match(m[0], /input\.select\(\)/,
    "the dialog does not select what is in the box, so it has to be deleted first");
});
