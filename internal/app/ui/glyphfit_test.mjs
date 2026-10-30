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
  const konst = (name) => {
    const m = src.match(new RegExp("^const " + name + " = [^\\n]*?;$", "m"));
    if (!m) throw new Error("palette.js has no const " + name);
    return m[0];
  };
  const body = [konst("markGap"), grab("markOn"), grab("glyphSVG")].join("\n") +
    "\nreturn { glyphSVG, markOn };";
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
  assert.equal(Number(text.attrs.x), 0);
  assert.equal(Number(text.attrs.y), 0);
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
  const size = app.match(/^const menuIconSize = .*?;$/m);
  assert.ok(size, "app.js has no menuIconSize");

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
        const n = {
          name, className: "", dataset: {}, textContent: "",
          classes: [],
          classList: { add(c) { n.classes.push(c); } },
        };
        made.push(n);
        return n;
      },
    },
    fitGlyphs() {},
  };
  const body = [icons[0], size[0], grabApp("syncMenuIcons")].join("\n") +
    "\nreturn { syncMenuIcons };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).syncMenuIcons();

  assert.equal(made.length, 3, "not every entry got a box");
  for (const n of made) assert.equal(n.className, "menu-icon");

  // A staccato is a note with the dot under it: the note in the box and
  // the mark beside it, so the drawing can colour them differently.
  assert.equal(buttons[0].kids[0].textContent, "", "the note");
  assert.equal(buttons[0].kids[0].dataset.mark, "", "the dot");
  assert.deepEqual(buttons[0].kids[0].classes, [],
                   "a note with a mark on it is not coloured all over");

  // A dynamic is the whole of what the entry writes, so it is one glyph
  // and the element itself carries the colour.
  assert.equal(buttons[2].kids[0].textContent, "", "ff");
  assert.equal(buttons[2].kids[0].dataset.mark, undefined);
  assert.deepEqual(buttons[2].kids[0].classes, ["is-mark"]);
  // And an empty box where there is none, so the words still line up.
  assert.equal(buttons[1].kids[0].textContent, "",
               "an entry that writes nothing was given a picture of something");
});

// Measuring before the music font has arrived.
//
// The font is declared in the stylesheet and fetched when something needs
// it. document.fonts.ready resolves when nothing is *pending*, and a font
// nothing has been drawn in yet is not pending — so it resolved at once,
// every glyph was measured in whatever the browser fell back to, and the
// answer was cached. These codepoints are in no other font, so what the
// menus came up with was nothing at all. Whether it happened depended on
// whether the palette had pulled the font in first, which is what made it
// "I did a go run and don't see the icon sometimes".
test("a measurement taken before the font arrives is not kept", () => {
  const pal = readFileSync(new URL("./palette.js", import.meta.url), "utf8");
  const grabPal = (name) => {
    const m = pal.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
    if (!m) throw new Error("palette.js has no " + name);
    return m[0];
  };

  let loaded = false;
  let measured = 0;
  const scope = {
    glyphFont: "Bravura",
    glyphInkCache: new Map(),
    document: {
      fonts: { check: () => loaded },
      createElement: () => ({
        getContext: () => ({
          font: "",
          measureText: () => {
            measured++;
            // Before the font: the fallback draws nothing for a codepoint
            // it has never heard of. After: a real box.
            return loaded
              ? { actualBoundingBoxLeft: 0, actualBoundingBoxRight: 60,
                  actualBoundingBoxAscent: 170, actualBoundingBoxDescent: 30 }
              : { actualBoundingBoxLeft: 0, actualBoundingBoxRight: 0,
                  actualBoundingBoxAscent: 0, actualBoundingBoxDescent: 0 };
          },
        }),
      }),
    },
  };
  const body = [grabPal("glyphFontReady"), grabPal("glyphInk")].join("\n") +
    "\nreturn { glyphInk };";
  const { glyphInk } = new Function(...Object.keys(scope), body)(
    ...Object.values(scope));

  assert.equal(glyphInk(""), null, "a glyph with no font behind it");
  assert.equal(measured, 1);

  // Asked again before the font arrives, it measures again rather than
  // handing back the answer it could not trust.
  assert.equal(glyphInk(""), null);
  assert.equal(measured, 2, "the bad answer was cached");

  // And once the font is there, the real box — and that one is kept.
  loaded = true;
  const ink = glyphInk("");
  assert.ok(ink, "the glyph was still not measured with the font loaded");
  assert.equal(ink.h.toFixed(2), "1.00", "170 up and 30 down of 200");
  assert.equal(measured, 3);
  glyphInk("");
  assert.equal(measured, 3, "a good answer was measured twice");
});

// Asked for by name, which is what actually fetches it.
test("the font is asked for rather than waited for in general", () => {
  const pal = readFileSync(new URL("./palette.js", import.meta.url), "utf8");
  const m = pal.match(/^function fitGlyphs\([\s\S]*?^}/m);
  assert.ok(m, "palette.js has no fitGlyphs");
  // The code, not the comments: the comment explains why fonts.ready is
  // the wrong thing to wait on, and naming it there is the point.
  const code = m[0].replace(/^\s*\/\/.*$/gm, "");
  assert.match(code, /document\.fonts\.load\(/,
    "it does not ask for the font by name");
  assert.ok(!/document\.fonts\.ready/.test(code),
    "it still waits on fonts.ready, which resolves before the font is there");
});

// A note with a mark on it: black note, red mark, the mark hung under the
// note and centred on it by its own ink.
test("a mark is drawn under the note in the colour kept for it", () => {
  const { glyphSVG } = load();
  // A quarter note: head on the baseline, stem reaching up.
  const note = { x: 0, y: -0.86, w: 0.3, h: 1.11 };
  // A staccato dot, small and sitting just under its own baseline.
  const dot = { x: 0, y: -0.06, w: 0.12, h: 0.12 };

  const svg = glyphSVG("", note, 15, "100%", "", dot);
  assert.equal(svg.children.length, 2, "the mark was not drawn");

  const [drawnNote, drawnMark] = svg.children;
  assert.equal(drawnNote.attrs.fill, "currentColor",
               "the note should be the colour of the words beside it");
  assert.equal(drawnMark.attrs.fill, "var(--mark)",
               "the mark should be the colour kept for what an entry writes");

  // Under the note, by the gap and no more.
  const markTop = dot.y + Number(drawnMark.attrs.y);
  const noteBottom = note.y + note.h;
  assert.ok(markTop > noteBottom, "the mark is not below the note");
  assert.ok(markTop - noteBottom < 0.1,
            `the mark hangs ${(markTop - noteBottom).toFixed(3)} below the note`);

  // And centred on it.
  const markMid = dot.x + Number(drawnMark.attrs.x) + dot.w / 2;
  assert.ok(Math.abs(markMid - (note.x + note.w / 2)) < 1e-9,
            "the mark is not centred under the note");

  // The view holds both of them.
  const [vx, vy, vw, vh] = svg.attrs.viewBox.split(" ").map(Number);
  assert.ok(vy <= note.y + 1e-9, "the top of the note is outside the view");
  assert.ok(vy + vh >= markTop + dot.h - 1e-9,
            "the bottom of the mark is outside the view");
  assert.ok(vx <= Math.min(note.x, dot.x + Number(drawnMark.attrs.x)) + 1e-9);
});

// A mark on its own is the whole of what the entry writes, so nothing in it
// is drawn in the colour of the words.
test("a mark on its own is one glyph", () => {
  const { glyphSVG } = load();
  const svg = glyphSVG("", { x: 0, y: -0.5, w: 0.25, h: 1 }, 15, "100%");
  assert.equal(svg.children.length, 1);
  assert.equal(svg.children[0].attrs.fill, "currentColor",
               "its colour is set on the element, not in the drawing");
});
