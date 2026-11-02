/*
 * menuicons_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The little pictures beside the menu entries.
 *
 * They are kept in two tables — the music ones as codepoints in the music
 * font, the rest as drawings of our own — and the menus are kept in the
 * markup. Three lists that have to agree, which is three lists that drift:
 * a codepoint off by one is a glyph that is not there and an empty square
 * on the menu, and two entries given the same drawing is a menu where two
 * commands look like the same command.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const glyphNames = JSON.parse(
  readFileSync(new URL("../../smufl/data/glyphnames.json", import.meta.url), "utf8"));

// The codepoints SMuFL defines, as the characters they are.
const SMUFL = new Set(
  Object.values(glyphNames).map((g) => String.fromCodePoint(
    parseInt(g.codepoint.replace("U+", ""), 16))));

function table(name) {
  const at = src.indexOf(`const ${name} = {`);
  if (at < 0) throw new Error(`app.js has no ${name}`);
  const end = src.indexOf("\n};", at);
  return src.slice(at, end);
}

const iconBlock = table("MENU_ICONS");
const drawBlock = table("MENU_DRAWINGS");
const removeMark = /const REMOVE_MARK = '(.*?)';/s.exec(src)?.[1];

const icons = new Map();
for (const m of iconBlock.matchAll(/"([\w-]+)":\s*"((?:\\u[0-9a-fA-F]{4})+)"/g)) {
  icons.set(m[1], m[2].replace(/\\u([0-9a-fA-F]{4})/g,
    (_, h) => String.fromCodePoint(parseInt(h, 16))));
}

const drawings = new Map();
for (const m of drawBlock.matchAll(
  /"?([\w-]+)"?:\s*(REMOVE_MARK|'[^']*'(?:\s*\+\s*\n?\s*'[^']*')*)/g)) {
  const body = m[2] === "REMOVE_MARK"
    ? removeMark
    : [...m[2].matchAll(/'([^']*)'/g)].map((q) => q[1]).join("");
  drawings.set(m[1], body);
}

test("the tables were read", () => {
  assert.ok(icons.size > 50, `only ${icons.size} music icons found`);
  assert.ok(drawings.size > 50, `only ${drawings.size} drawings found`);
  assert.ok(removeMark, "REMOVE_MARK is not there");
});

// A codepoint that is not a SMuFL glyph is an empty square on the menu, and
// nothing anywhere says so: the font simply has nothing at that place.
test("every music icon is a glyph the font has", () => {
  const bad = [];
  for (const [action, chars] of icons) {
    for (const ch of chars) {
      if (!SMUFL.has(ch)) {
        bad.push(`${action}: U+${ch.codePointAt(0).toString(16).toUpperCase()}`);
      }
    }
  }
  assert.deepEqual(bad, [], "codepoints no SMuFL glyph is at");
});

// Drawn into the same 24-point square the music glyphs are fitted into, out
// of the handful of shapes menuDrawing wraps. Anything else is either drawn
// outside the square or is markup menuDrawing will not render.
test("every drawing stays inside its square", () => {
  const allowed = /^(path|circle|rect|line|polyline|polygon)$/;
  const bad = [];
  for (const [action, body] of drawings) {
    const rest = body.replace(/<(\w+)\b[^>]*\/>/g, (whole, tag) => {
      if (!allowed.test(tag)) bad.push(`${action}: <${tag}>`);
      return "";
    });
    if (rest.trim()) bad.push(`${action}: leftover markup ${rest.trim().slice(0, 40)}`);
    // Not a digit that follows a dot: an opacity of ".55" is not a
    // coordinate of 55, and reading it as one failed this on two entries
    // that were drawn perfectly well.
    for (const n of body.matchAll(/(?<![\d.])-?\d+(?:\.\d+)?/g)) {
      const v = Number(n[0]);
      // What is being checked is that no coordinate wanders off the
      // 24-point square; stroke widths and radii are small anyway.
      if (v > 26) bad.push(`${action}: ${v} is outside the square`);
    }
  }
  assert.deepEqual(bad, [], "drawings that will not render as meant");
});

// Two entries drawn the same way are two commands that look like one
// command. The one exception is the mark every "take it away" entry
// carries, which is deliberately shared and says so in its name.
test("no two entries share a drawing by accident", () => {
  const byBody = new Map();
  for (const [action, body] of drawings) {
    if (body === removeMark) continue;
    const had = byBody.get(body);
    if (had) {
      assert.fail(`"${had}" and "${action}" are drawn identically`);
    }
    byBody.set(body, action);
  }
});

// Nothing is in both tables: MENU_ICONS wins in syncMenuIcons, so a drawing
// written for an entry that also has a codepoint is a drawing nobody ever
// sees.
test("no entry is in both tables", () => {
  const both = [...drawings.keys()].filter((a) => icons.has(a));
  assert.deepEqual(both, [], "entries whose drawing is shadowed by a glyph");
});

// Every entry on every menu has a picture. Half a column of pictures reads
// worse than none at all, so a new entry without one is a gap worth being
// told about rather than something to notice later in a screenshot.
test("every menu entry has a picture", () => {
  const menus = [];
  for (const m of html.matchAll(/<div class="menu"[\s\S]*?(?=<div class="menu"|<\/nav>)/g)) {
    const block = m[0];
    const name = /<button class="menu-title"[^>]*>([\s\S]*?)<\/button>/.exec(block);
    menus.push({
      name: name ? name[1].replace(/<[^>]+>/g, "").trim() : "?",
      actions: [...block.matchAll(/data-action="([^"]+)"/g)].map((a) => a[1]),
    });
  }
  assert.ok(menus.length >= 8, `only ${menus.length} menus found in index.html`);

  const missing = [];
  for (const menu of menus) {
    for (const action of menu.actions) {
      if (!icons.has(action) && !drawings.has(action)) {
        missing.push(`${menu.name} ▸ ${action}`);
      }
    }
  }
  assert.deepEqual(missing, [], "menu entries with no picture beside them");
});

// No theme colour is passed as an SVG presentation attribute.
//
// `stroke="var(--mark)"` looks like it should work and does not: a
// presentation attribute is not a CSS declaration, so a custom property in
// one is never resolved. The attribute is thrown away and the shape falls
// back to what it inherits — which is how the engraving window's beams and
// measure marks came to be drawn in the light theme's ink on a dark
// ground, while the staff lines beside them were right because
// currentColor is a real attribute keyword.
//
// The fix is always the same: give the shape a class and put the colour in
// the stylesheet. This is here because the failure is silent and looks
// like a theme problem rather than a markup one.
test("no theme colour is written into an SVG attribute", () => {
  const bad = [];
  src.split("\n").forEach((line, i) => {
    // Not the prose. The paragraph above this test names the pattern it
    // forbids, which is the useful thing to write down and not a use of
    // it; a lint that cannot tell them apart would forbid explaining
    // itself.
    const code = line.trim();
    if (code.startsWith("//") || code.startsWith("*")) return;
    const m = /\b(fill|stroke|color|stop-color)\s*=\s*["'`][^"'`]*var\(/.exec(line);
    if (m) bad.push(`app.js:${i + 1}: ${m[0].trim()}…`);
  });
  assert.deepEqual(bad, [],
    "use a class and style it in style.css; var() is not resolved here");
});

// And the classes that replaced those attributes are actually styled.
//
// Moving a colour out of an attribute and into a class fixes nothing if
// the rule is never written: the shape falls back to black exactly as
// before, and the markup now looks correct while the page does not.
test("every colour class the drawings use has a rule", () => {
  const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
  for (const cls of ["knob-pic .mark", "map-busy", "range-tessitura"]) {
    assert.match(css, new RegExp("\\." + cls.replace(" ", " .?") + "\\s*\\{"),
      `.${cls} is used in app.js but has no rule in style.css`);
  }
});
