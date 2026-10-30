/*
 * glyphfit_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Drawing a glyph so that all of it is there.
 *
 * A SMuFL glyph is drawn against a staff that is not there. Its ink reaches
 * far above and below the baseline while the line box the browser gives it
 * is one em tall and centred on that baseline, so centring the line box in a
 * square button leaves most of the ink outside it — and the button clips.
 * Reported as "notes are being eaten by the top, no margin, same with
 * barlines and clefs".
 *
 * Each glyph is drawn in a view that is exactly its own ink. What it must
 * not do is make everything the same size: a dot fitted to its button would
 * be as big as a treble clef.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./palette.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("palette.js has no " + name);
  return m[0];
};

// Ink boxes as Bravura really measures them, in ems: a treble clef reaching
// far above and below its baseline, a notehead barely either way, a dot
// smaller again.
const INK = {
  clef:     { x: 0, y: -1.42, w: 0.66, h: 1.86 },
  quarter:  { x: 0, y: -0.86, w: 0.3, h: 1.11 },
  dot:      { x: 0, y: -0.12, w: 0.12, h: 0.12 },
};

function load() {
  const made = [];
  const el = (ns, name) => {
    const n = {
      ns, name, attrs: {}, style: {}, children: [],
      setAttribute(k, v) { this.attrs[k] = v; },
      appendChild(c) { this.children.push(c); return c; },
    };
    made.push(n);
    return n;
  };
  const scope = {
    document: { createElementNS: el },
    glyphFont: "Bravura",
    svgNS: "http://www.w3.org/2000/svg",
  };
  const body = grab("glyphSVG") + "\nreturn { glyphSVG };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
}

test("the view is the glyph's own ink, so nothing is outside it", () => {
  const { glyphSVG } = load();
  const svg = glyphSVG("", INK.clef, 13, "84%");
  assert.equal(svg.attrs.viewBox, "0.0000 -1.4200 0.6600 1.8600");
  // Undistorted inside whatever box the cap leaves it.
  assert.equal(svg.attrs.preserveAspectRatio, "xMidYMid meet");
  // The glyph is written from its baseline at the origin, which is what
  // makes the view above a measurement from that same point.
  const text = svg.children[0];
  assert.equal(text.attrs.x, "0");
  assert.equal(text.attrs.y, "0");
  assert.equal(text.attrs["font-size"], "1");
  assert.equal(text.textContent, "");
});

test("a glyph keeps the size its section asks for", () => {
  const { glyphSVG } = load();
  const clef = glyphSVG("", INK.clef, 13, "84%");
  const dot = glyphSVG("", INK.dot, 19, "84%");

  assert.equal(clef.style.height, (1.86 * 13).toFixed(2) + "px");
  assert.equal(dot.style.height, (0.12 * 19).toFixed(2) + "px");
  // The thing that must not happen: everything the same size.
  assert.ok(parseFloat(clef.style.height) > parseFloat(dot.style.height) * 5,
            "a dot came out nearly as big as a clef");
});

test("nothing is allowed past its share of the button", () => {
  const { glyphSVG } = load();
  for (const [name, ink] of Object.entries(INK)) {
    const svg = glyphSVG("x", ink, 19, "84%");
    assert.equal(svg.style.maxWidth, "84%", name);
    assert.equal(svg.style.maxHeight, "84%", name);
  }
});

test("the drawing is hidden from anything that reads the page aloud", () => {
  const { glyphSVG } = load();
  const svg = glyphSVG("", INK.clef, 13, "84%");
  assert.equal(svg.attrs["aria-hidden"], "true",
               "the label beside it is the thing to read");
  assert.equal(svg.attrs.focusable, "false");
});

// The pictures in the menus, written in from the table.
test("every menu entry gets an icon box, and the right glyph in it", () => {
  const app = readFileSync(new URL("./app.js", import.meta.url), "utf8");
  const grabApp = (name) => {
    const m = app.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
    if (!m) throw new Error("app.js has no " + name);
    return m[0];
  };
  const icons = app.match(/^const MENU_ICONS = \{[\s\S]*?^\};$/m);
  assert.ok(icons, "app.js has no menu icon table");

  const buttons = [
    { dataset: { action: "artic-staccato" } },
    { dataset: { action: "open" } },
    { dataset: { action: "dyn-ff" } },
  ].map((b) => ({
    ...b,
    kids: [],
    querySelector: () => null,
    insertBefore(node) { this.kids.unshift(node); return node; },
  }));

  const made = [];
  const scope = {
    document: {
      querySelectorAll: (sel) => (sel === "[data-action]" ? buttons : []),
      createElement: (name) => {
        const n = { name, className: "", dataset: {}, textContent: "" };
        made.push(n);
        return n;
      },
    },
    fitGlyphs() {},
  };
  const body = icons[0] + "\n" + grabApp("syncMenuIcons") + "\nreturn { syncMenuIcons };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).syncMenuIcons();

  assert.equal(made.length, 3, "not every entry got a box");
  for (const n of made) assert.equal(n.className, "menu-icon");

  // The mark itself where there is one.
  assert.equal(buttons[0].kids[0].textContent, "", "staccato");
  assert.equal(buttons[2].kids[0].textContent, "", "ff");
  // And an empty box where there is none, so the words still line up.
  assert.equal(buttons[1].kids[0].textContent, "",
               "an entry that writes nothing was given a picture of something");
});
