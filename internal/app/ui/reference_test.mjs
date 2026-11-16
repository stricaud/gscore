/*
 * reference_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

/*
 * Where the score came from.
 *
 * Score Setup has a box for it, saves it with the rest of what the score
 * says about itself, and puts a link under the box when what is typed is a
 * web address — so the page a score was taken from is a click away.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// A page of two elements: the box and the line under it.
function page(value) {
  const made = [];
  const link = { textContent: "", children: [], appendChild(el) { this.children.push(el); } };
  const els = {
    "#score-about-reference": { value },
    "#score-about-reference-link": link,
  };
  const document = { createElement: (tag) => { const el = { tag }; made.push(el); return el; } };
  const $ = (sel) => els[sel];
  const body = grab("showReferenceLink") + "\nreturn { showReferenceLink };";
  const api = new Function("$", "document", body)($, document);
  return { link, ...api };
}

test("the dialog has a box for the reference, and saves it", () => {
  assert.ok(html.includes('id="score-about-reference"'), "no reference box in Score Setup");
  assert.ok(src.includes('reference: $("#score-about-reference").value'),
    "saving Score Setup does not send the reference");
  assert.ok(src.includes('field("reference", i.reference)'),
    "opening Score Setup does not show the reference");
});

test("a web address gets a link to open it", () => {
  const { link, showReferenceLink } = page("https://musescore.com/user/1/scores/2");
  showReferenceLink();
  assert.equal(link.children.length, 1);
  const a = link.children[0];
  assert.equal(a.tag, "a");
  assert.equal(a.href, "https://musescore.com/user/1/scores/2");
  assert.equal(a.target, "_blank", "the original opens beside the editor, not over it");
  assert.equal(a.rel, "noopener");
});

test("words that are not an address get no link", () => {
  for (const words of ["Peters edition, 1902", "", "   ", "ftp://x"]) {
    const { link, showReferenceLink } = page(words);
    showReferenceLink();
    assert.equal(link.children.length, 0, JSON.stringify(words));
  }
});
