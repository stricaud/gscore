/*
 * pdfreview_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The drawing that shows what was read out of a PDF.
 *
 * It exists because the reader's one unavoidable guess — how many staves
 * make a system — is invisible in its output. A band score read as single
 * staves arrives as one part thousands of bars long: a perfectly well formed
 * score with nothing in it to say that it is wrong. The drawing is the only
 * place that mistake becomes visible, so the drawing has to land exactly on
 * the page it describes. One that is merely near the music is worse than
 * none, because it invites somebody to correct a reading that was right.
 *
 * The trap is the y axis. A PDF counts upward from the foot of the page and
 * SVG downward from the head. Turning the picture over with a transform is
 * the obvious fix and the wrong one: it mirrors the pitch labels too, and an
 * upside-down pitch is worse than no pitch. So the turn is in the
 * arithmetic, and these tests pin it there.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// A document just real enough to draw into, which records what was made.
function fakeDOM() {
  const made = [];
  const node = (name) => {
    const n = {
      name,
      attrs: {},
      text: undefined,
      kids: [],
      setAttribute(k, v) { this.attrs[k] = v; },
      appendChild(c) { this.kids.push(c); return c; },
      set textContent(v) { this.text = v; },
      get textContent() { return this.text; },
    };
    made.push(n);
    return n;
  };
  const frag = node("#fragment");
  return {
    made,
    document: {
      createElementNS: (_ns, name) => node(name),
      createDocumentFragment: () => frag,
    },
  };
}

// one page: two staves of the same system, one notehead on the lower
const page = {
  width: 600,
  height: 800,
  staves: [
    { y: [700, 704, 708, 712, 716], x0: 50, x1: 550, inter: 4, system: 0, part: 0 },
    { y: [600, 604, 608, 612, 616], x0: 50, x1: 550, inter: 4, system: 0, part: 1 },
  ],
  marks: [
    { x: 100, y: 608, kind: "note", pitch: "g4", staff: 1 },
    { x: 60, y: 606, kind: "clef", staff: 1 },
  ],
  rules: [{ x1: 50, y1: 700, x2: 550, y2: 700, w: 0.5 }],
};

function draw(zoom) {
  const dom = fakeDOM();
  const svg = {
    attrs: {},
    kids: [],
    setAttribute(k, v) { this.attrs[k] = v; },
    appendChild(c) { this.kids.push(c); },
    set innerHTML(_v) { this.kids = []; },
  };
  const scope = {
    document: dom.document,
    pdfSurvey: { pages: [page], parts: 2, perSystem: 2, detected: 1 },
    $: (sel) => (sel === "#pdf-overlay" ? svg
      : sel === "#pdf-zoom" ? { value: String(zoom) }
        : sel === "#pdf-page" ? { value: "0" } : null),
  };
  const fn = new Function(
    "scope",
    "with (scope) { " + grab("drawPDFPage") + " return drawPDFPage; }");
  fn(scope)(0);
  return { svg, made: dom.made };
}

test("the page is drawn at its own size, so the drawing can be laid over it", () => {
  const { svg } = draw(2);
  assert.equal(svg.attrs.viewBox, "0 0 600 800");
  assert.equal(svg.attrs.width, "1200");
  assert.equal(svg.attrs.height, "1600");
});

test("a PDF's upward y becomes SVG's downward y by arithmetic, not a flip", () => {
  const { made } = draw(2);
  // The staff whose lines run 700..716 up the page occupies 800-716..800-700
  // down it: the rectangle's top is the staff's *highest* line.
  const rects = made.filter((n) => n.name === "rect");
  assert.equal(rects.length, 2, "one rectangle to a staff");
  const first = rects[0];
  assert.equal(Number(first.attrs.y), 800 - 716);
  assert.equal(Number(first.attrs.height), 16);
  assert.equal(Number(first.attrs.x), 50);
  assert.equal(Number(first.attrs.width), 500);
  // And nothing is turned over wholesale, which would mirror the words.
  assert.ok(!JSON.stringify(made).includes("scale(1,-1)"));
  assert.ok(!JSON.stringify(made).includes("scale(1, -1)"));
});

test("a notehead is ringed where it sits, with the pitch that was read", () => {
  const { made } = draw(2);
  const circles = made.filter((n) => n.name === "circle");
  const ring = circles.find((c) => c.attrs.stroke === "#dc2626");
  assert.ok(ring, "the notehead is not ringed");
  assert.equal(Number(ring.attrs.cy), 800 - 608);
  const labels = made.filter((n) => n.name === "text" && n.text === "g4");
  assert.equal(labels.length, 1, "the pitch read is not written beside it");
  // Above the notehead, not on it: a label over the head hides the thing
  // being checked.
  assert.ok(Number(labels[0].attrs.y) < Number(ring.attrs.cy));
});

test("pitches are left off when they would be too small to read", () => {
  const { made } = draw(1);
  assert.equal(made.filter((n) => n.name === "text" && n.text === "g4").length, 0,
    "a pitch was written at a zoom where it is a smudge");
  // The staves are still labelled: which system and which part is the thing
  // being judged, and it stays legible because there are few of them.
  assert.ok(made.some((n) => n.name === "text" && /^s1·p[12]$/.test(n.text)));
});

// Legibility is a size on the glass, not a zoom factor. A band score sets
// its staves at three and a half points and a solo at ten; the same zoom
// makes one readable and the other a smear, so the zoom alone cannot be the
// test. This is the rule that was wrong first: at zoom 2 on a real band
// score every pitch came out six pixels tall.
test("whether a pitch is written depends on its size on the glass", () => {
  const small = JSON.parse(JSON.stringify(page));
  for (const st of small.staves) st.inter = 3.5;
  const shown = (p, zoom) => {
    const saved = page.staves.map((s) => s.inter);
    page.staves.forEach((s, i) => (s.inter = p.staves[i].inter));
    const { made } = draw(zoom);
    page.staves.forEach((s, i) => (s.inter = saved[i]));
    return made.filter((n) => n.name === "text" && n.text === "g4").length;
  };
  assert.equal(shown(small, 1), 0, "a 3.5pt staff at 1x is not readable");
  assert.equal(shown(small, 4), 1, "a 3.5pt staff at 4x is readable");
  const big = JSON.parse(JSON.stringify(page));
  for (const st of big.staves) st.inter = 10;
  assert.equal(shown(big, 1), 1, "a 10pt staff is readable without zooming");
});

test("each staff is labelled with the system and the part it went into", () => {
  const { made } = draw(2);
  const labels = made.filter((n) => n.name === "text" && /^s\d+·p\d+$/.test(n.text));
  assert.deepEqual(labels.map((l) => l.text), ["s1·p1", "s1·p2"]);
  // Different colours, because the mistake this drawing is for shows as a
  // change of colour down the page instead of across it.
  assert.notEqual(labels[0].attrs.fill, labels[1].attrs.fill);
});

test("the strokes the page is drawn with come through, faintly", () => {
  const { made } = draw(2);
  const lines = made.filter((n) => n.name === "line");
  assert.equal(lines.length, 1);
  assert.equal(Number(lines[0].attrs.y1), 800 - 700);
});

// What the words beside the drawing say. One part out of a file whose pages
// hold several staves is the failure the whole window exists for, so it is
// named rather than left to be noticed.
function describe(sv) {
  const el = { textContent: "", classList: { toggle() {} } };
  const scope = {
    plural: new Function("n", "one", "many",
      "return n + ' ' + (n === 1 ? one : (many || one + 's'));"),
  };
  const fn = new Function(
    "scope", "with (scope) { " + grab("describePDFReading") +
    " return describePDFReading; }");
  fn(scope)(el, sv);
  return el.textContent;
}

// The number in the box is an instruction. It was once only an instruction
// if the button beside it was pressed: typing 8 and dismissing the window
// with its own "Go on" threw the 8 away and left the file read as before,
// which from outside looks exactly like the reader ignoring what it was
// told. These pin that every way of leaving the box applies it.
function wiring() {
  const calls = [];
  const el = () => {
    const on = {};
    return {
      value: "0",
      classList: { toggle(k, v) { this.state = !!v; } },
      addEventListener(k, f) { on[k] = f; },
      fire(k, e) { return on[k] && on[k](e || {}); },
      has(k) { return !!on[k]; },
    };
  };
  const nodes = {
    "#pdf-reread": el(), "#pdf-page": el(), "#pdf-zoom": el(),
    "#pdf-staves": el(), "#pdf-note": el(),
  };
  const opened = [];
  const scope = {
    rereadPDF: () => calls.push(Number(nodes["#pdf-staves"].value)),
    drawPDFPage: () => {},
    offerScoreSetup: async () => { opened.push(true); },
    $: (sel) => nodes[sel] || null,
  };
  const body =
    "let pdfReadAs = null, pdfSoon = null;\n" +
    grab("setupPDFReview") + "\n" + grab("pdfStaffBox") + "\n" +
    grab("markPDFPending") + "\n" +
    "return { setupPDFReview, readAs: (v) => { pdfReadAs = v; } };";
  const made = new Function("scope", "with (scope) { " + body + " }")(scope);
  made.setupPDFReview();
  return { nodes, calls, opened, readAs: made.readAs };
}

test("a number typed in the box is applied when the window is dismissed", () => {
  const w = wiring();
  w.readAs(0);                      // the file was read with the guess
  w.nodes["#pdf-staves"].value = "8";
  w.nodes["#pdf-note"].fire("close");
  assert.deepEqual(w.calls, [8], "the 8 was thrown away by Go on");
});

test("dismissing without changing the number reads nothing again", () => {
  const w = wiring();
  w.readAs(8);
  w.nodes["#pdf-staves"].value = "8";
  w.nodes["#pdf-note"].fire("close");
  assert.deepEqual(w.calls, [], "the same file was read twice for nothing");
});

// Before a survey has come back there is no number to disagree with, and a
// dialog shut on a file that could not be read at all must not go and read
// it again.
test("a window closed before anything was read stays quiet", () => {
  const w = wiring();
  w.nodes["#pdf-note"].fire("close");
  assert.deepEqual(w.calls, []);
});

// A page says how many staves there are and nothing about what plays them,
// so the import leaves a question standing. Somebody who does not already
// know there is a window for answering it has no reason to go looking, so
// the window comes to them when the PDF one is done with.
test("closing the PDF window leads on to naming the staves", async () => {
  const w = wiring();
  w.readAs(9);
  w.nodes["#pdf-staves"].value = "9";
  w.nodes["#pdf-note"].fire("close");
  await new Promise((r) => setTimeout(r, 0));
  assert.deepEqual(w.opened, [true], "the import stopped at the reading");
});

test("it leads on only after the file has been read the way it was told", async () => {
  // Otherwise Score setup opens on the staves of the old grouping and is
  // replaced under the cursor a moment later.
  const w = wiring();
  w.readAs(1);
  w.nodes["#pdf-staves"].value = "9";
  w.nodes["#pdf-note"].fire("close");
  await new Promise((r) => setTimeout(r, 0));
  assert.deepEqual(w.calls, [9]);
  assert.deepEqual(w.opened, [true]);
});

test("the button and Enter still work, and the box shows when it is owed", () => {
  const w = wiring();
  w.readAs(0);
  w.nodes["#pdf-staves"].value = "4";
  w.nodes["#pdf-staves"].fire("input");
  assert.equal(w.nodes["#pdf-reread"].classList.state, true,
    "a number waiting to be used is not shown as waiting");
  w.nodes["#pdf-staves"].fire("keydown", { key: "Enter", preventDefault() {} });
  assert.deepEqual(w.calls, [4]);
  w.nodes["#pdf-reread"].fire("click", { preventDefault() {} });
  assert.deepEqual(w.calls, [4, 4]);
});

test("the verdict says what the file became, and names the usual failure", () => {
  const bad = describe({
    parts: 1, perSystem: 1, detected: 1,
    pages: [{ staves: [{}, {}, {}] }],
  });
  assert.match(bad, /1 part/);
  assert.match(bad, /one long line/,
    "a whole band read as one part is not pointed out");

  const fine = describe({
    parts: 4, perSystem: 4, detected: 4,
    pages: [{ staves: [{}, {}, {}, {}] }],
  });
  assert.match(fine, /4 parts/);
  assert.doesNotMatch(fine, /one long line/);
  // The guess is only mentioned when it disagrees with what was used;
  // agreeing, it is one fact said twice.
  assert.doesNotMatch(fine, /would have given/);

  const told = describe({
    parts: 4, perSystem: 4, detected: 1,
    pages: [{ staves: [{}, {}, {}, {}] }],
  });
  assert.match(told, /would have given 1 staff/);
});

// The quieter failure: a number that is obeyed and is wrong. The grouping
// does what it is told, parts come out, and each system is cut across the
// middle. A band score whose pages hold 9, 11, 10 and 11 staves cannot be
// read with any single number, and the verdict should say which pages
// disagree rather than leave somebody to count them.
test("a number that does not divide the page is said so, with the pages", () => {
  const staves = (n) => ({ staves: Array.from({ length: n }, () => ({})) });
  const s = describe({
    parts: 8, perSystem: 8, detected: 1,
    pages: [staves(9), staves(11), staves(10), staves(11)],
  });
  assert.match(s, /Page 1 holds 9/);
  assert.match(s, /page 2 holds 11/);
  assert.match(s, /not a whole number of systems of 8/);
  // Only the first few are named; a hundred-page score should not print a
  // hundred page numbers.
  assert.match(s, /and others/);

  // A number that divides every page says nothing about it.
  const fine = describe({
    parts: 2, perSystem: 2, detected: 2,
    pages: [staves(10), staves(8)],
  });
  assert.doesNotMatch(fine, /not a whole number/);

  // Nor does the guess-it-yourself setting, which has no number to divide by.
  const guess = describe({
    parts: 3, perSystem: 0, detected: 3,
    pages: [staves(9)],
  });
  assert.doesNotMatch(guess, /not a whole number/);
});
