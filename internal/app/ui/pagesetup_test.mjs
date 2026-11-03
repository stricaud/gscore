/*
 * pagesetup_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The paper.
 *
 * Page setup is not only how the score comes out of the printer: the paper
 * decides how wide a system may be, so changing it re-engraves the music.
 * The thing most likely to be wrong in a dialog of eight number fields is
 * the units, and millimetres read as points are a page a third of the size
 * with nothing to say it went wrong.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const fr = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");
const api = readFileSync(
  new URL("../../../docs/openapi.yaml", import.meta.url), "utf8");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

test("the menu entry is live and in French", () => {
  const m = html.match(/data-action="page-setup"[^>]*>/);
  assert.ok(m, "no page setup entry");
  assert.ok(!m[0].includes("disabled"), "it is still disabled");
  assert.match(src, /case "page-setup": await pageSetupDialog\(\)/);
  assert.ok(fr.includes('"Page setup…":'), "no French");
});

// The sizes come from the program, not from a list written out again here:
// two lists of paper sizes is one list too many.
test("the paper sizes are asked for rather than written down twice", () => {
  const body = grab("pageSetupDialog");
  assert.match(body, /await fetch\("\/api\/pagesetup"\)/);
  assert.match(body, /now\.papers/);
  assert.doesNotMatch(body, /210|297|595/,
    "a paper size is written into the editor");
});

test("every field the dialog fills in is one the endpoint takes", () => {
  const body = grab("pageSetupDialog");
  const sent = [...body.matchAll(/^\s*(\w+): \$\("#ps-/gm)].map((m) => m[1]);
  for (const name of sent) {
    assert.match(api, new RegExp("- name: " + name + "\\b"),
      `the dialog sends ${name} and /api/pagesetup does not take it`);
  }
  // And the four margins are all there, since three of four is the sort
  // of thing nobody notices.
  for (const n of ["marginTop", "marginBottom", "marginLeft", "marginRight"]) {
    assert.ok(body.includes(n + ":"), `${n} is not sent`);
  }
});

// Millimetres everywhere except the type size, which is an engraving
// measurement and is quoted in points like every other one.
test("the dialog says which unit each field is in", () => {
  assert.match(html, /Margins, in millimetres/);
  assert.match(html, /pt —/, "the staff sizes are not labelled in points");
});

// Going back to the default is not the same as writing today's defaults
// into the file: one keeps following the program, the other freezes it.
test("there is a way back to no choice at all", () => {
  assert.match(html, /value="reset"/);
  assert.match(grab("pageSetupDialog"), /api\/pagesetup\?reset=1/);
});

test("a size the list does not offer is kept rather than silently changed", () => {
  // Set by a script, or by a later version. Replacing it with the nearest
  // offer would change the score because somebody opened a window.
  assert.match(grab("pageSetupDialog"), /space\.add\(new Option/);
});

test("the room left to print on is worked out as the fields are typed", () => {
  const body = grab("pageSetupDialog");
  assert.match(body, /showRoom/);
  assert.match(body, /no room for any music/);
  // Turned on its side, the width and height swap — and the room left is
  // measured after that, not before.
  assert.match(body, /if \(sideways && w < h\) \[w, h\] = \[h, w\]/);
});

test("a custom size only asks for its measurements when it is chosen", () => {
  assert.match(grab("pageSetupDialog"), /\$\("#ps-custom"\)\.hidden = !custom/);
  assert.match(html, /id="ps-custom" hidden/);
});

test("the score is redrawn, since the paper changes the engraving", () => {
  assert.match(grab("pageSetupDialog"), /await refreshScore\(\)/);
});

test("every class the dialog uses has a rule", () => {
  assert.match(css, /\.ps-row[\s,.:{]/);
  assert.match(css, /#ps-room[\s,.:{]/);
});
