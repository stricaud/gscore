/*
 * engravingcancel_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Getting out of the Engraving window.
 *
 * It is the one window here that changes the score as you go, and it has to
 * be: nobody can judge a beam of 0.55 from the number, only from the page in
 * front of them. The cost is that there was no way to say none of it was
 * meant — the window had Reset all, which throws away the score's own
 * settings as well, and Done, which keeps whatever had happened on the way.
 *
 * So Cancel puts back what was there when the window opened, and the two
 * cases it has to tell apart are a measurement that was set to a number and
 * one that was merely inherited. Putting the second back as the number it
 * inherited would leave the score frozen at today's house style — looking
 * untouched and no longer following it.
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

/* ---------- a window to open ---------- */

// Enough of an element for the window to be built in: children, classes and
// the one listener the dialog itself needs.
function el(tag) {
  const e = {
    tagName: tag.toUpperCase(),
    children: [],
    dataset: {},
    classes: new Set(),
    listeners: {},
    textContent: "",
    appendChild(k) { this.children.push(k); return k; },
    querySelectorAll(sel) {
      const want = sel.toUpperCase();
      return this.children.filter((c) => c.tagName === want);
    },
    addEventListener(n, fn) { (this.listeners[n] = this.listeners[n] || []).push(fn); },
    removeEventListener(n, fn) {
      this.listeners[n] = (this.listeners[n] || []).filter((f) => f !== fn);
    },
    fire(n) { return Promise.all((this.listeners[n] || []).map((f) => f())); },
  };
  Object.defineProperty(e, "innerHTML", {
    get() { return ""; },
    set() { e.children.length = 0; },
  });
  Object.defineProperty(e, "className", {
    get() { return [...e.classes].join(" "); },
    set(v) { e.classes = new Set(String(v).split(" ").filter(Boolean)); },
  });
  e.classList = {
    add: (...c) => c.forEach((x) => e.classes.add(x)),
    remove: (...c) => c.forEach((x) => e.classes.delete(x)),
    contains: (c) => e.classes.has(c),
    toggle: (c, on) => { if (on) e.classes.add(c); else e.classes.delete(c); },
  };
  return e;
}

// Three measurements in two groups, shaped the way /api/engraving sends
// them. What they are does not matter; that one of them starts out set and
// the others do not is the whole point.
const KNOBS = [
  { name: "noteBase", label: "Note slot", group: "Spacing", unit: "",
    min: 3, max: 8, floor: 0.1, ceiling: 1000, default: 5 },
  { name: "staffGap", label: "Staff gap", group: "Spacing", unit: "",
    min: 8, max: 16, floor: 1, ceiling: 1000, default: 11 },
  { name: "beamThickness", label: "Beam", group: "Beams", unit: "",
    min: 0.4, max: 0.7, floor: 0.01, ceiling: 10, default: 0.5 },
];

// A standing-in /api/engraving: what the score has set, read and written.
function open({ set = {}, sure = true, of = "" } = {}) {
  const score = new Map(Object.entries(set));
  const posts = [];
  const drawn = [];
  const said = [];
  const asked = [];

  const reply = () => ({
    of,
    knobs: KNOBS.map((k) => ({
      ...k,
      value: score.has(k.name) ? score.get(k.name) : k.default,
      set: score.has(k.name),
    })),
  });

  const fetch = async (url, opts) => {
    const q = new URL(url, "http://gscore.test").searchParams;
    if (!opts || opts.method !== "POST") return { ok: true, json: async () => reply() };
    posts.push(Object.fromEntries(q));
    if (q.has("reset")) score.clear();
    else {
      for (const [name, raw] of q) {
        if (raw === "") score.delete(name);
        else score.set(name, Number(raw));
      }
    }
    return { ok: true, json: async () => reply() };
  };

  const ids = {
    "#engraving": el("dialog"),
    "#engraving-cats": el("nav"),
    "#engraving-rows": el("div"),
    "#engraving-whose": el("p"),
    "#engraving-reset": el("button"),
    "#engraving-cancel": el("button"),
    "#engraving-done": el("button"),
  };
  const dlg = ids["#engraving"];
  dlg.shown = false;
  dlg.showModal = () => { dlg.shown = true; };
  // What a real dialog does: closing it fires close, and nothing else.
  dlg.close = () => { dlg.shown = false; return dlg.fire("close"); };

  const scope = {
    $: (sel) => ids[sel] ?? null,
    document: { createElement: el },
    t: (s) => s,
    status: (s) => said.push(s),
    fetch,
    refreshScore: async () => { drawn.push(true); },
    confirmThat: async (q) => { asked.push(q); return sure; },
    knobPicture: () => "",
    KNOB_PICTURES: {},
  };
  const body = grab("knobTitle") + "\n" + grab("engravingDialog") +
    "\nreturn { engravingDialog };";
  const names = Object.keys(scope);
  const api = new Function(...names, body)(...names.map((n) => scope[n]));

  return { api, ids, dlg, score, posts, drawn, said, asked };
}

// Everything the window does is behind a resolved promise, so one turn of
// the event loop is enough to let it all settle.
const settle = () => new Promise((r) => setTimeout(r, 0));

// The field for a measurement, opening the heading it is under if need be.
function field(w, name) {
  const want = "engraving-" + name;
  const find = () => {
    for (const row of w.ids["#engraving-rows"].children) {
      for (const c of row.children) if (c.id === want) return c;
    }
    return null;
  };
  let f = find();
  if (f) return f;
  const group = KNOBS.find((k) => k.name === name).group;
  w.ids["#engraving-cats"].children.find((b) => b.dataset.group === group).onclick();
  f = find();
  assert.ok(f, "no field for " + name);
  return f;
}

// Typing a number into a field, as a browser would: the value changes and
// the field says so.
async function type(w, name, value) {
  const f = field(w, name);
  f.value = String(value);
  await f.fire("change");
  await settle();
}

/* ---------- the window is there to be got out of ---------- */

test("the window has a way out as well as a way to keep it", () => {
  const dlg = /<dialog id="engraving"[\s\S]*?<\/dialog>/.exec(html)[0];
  assert.match(dlg, /id="engraving-cancel"/, "no Cancel button");
  assert.match(dlg, /id="engraving-done"[^>]*class="primary"/,
    "Done is no longer the one Enter presses");
  // And Cancel is not the primary one, since it throws work away.
  assert.doesNotMatch(dlg, /id="engraving-cancel"[^>]*primary/);
  assert.ok(fr.includes('"Cancel":'), "no French");
});

test("the window says what Cancel will do", () => {
  const dlg = /<dialog id="engraving"[\s\S]*?<\/dialog>/.exec(html)[0];
  assert.match(dlg, /Cancel puts everything back/);
});

/* ---------- cancel ---------- */

test("cancel puts back the number the score had", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();

  await type(w, "noteBase", 7);
  assert.equal(w.score.get("noteBase"), 7, "the change was not applied");

  w.ids["#engraving-cancel"].onclick();
  await settle();
  assert.equal(w.score.get("noteBase"), 6);
  assert.equal(w.dlg.shown, false, "the window is still open");
});

// The case that is easy to get wrong. A measurement nobody had set is put
// back by having no answer of its own again — not by being written with the
// number it was inheriting, which would look the same today and stop
// following the house style tomorrow.
test("a measurement set inside the window goes back to being inherited", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();

  await type(w, "beamThickness", 0.62);
  assert.equal(w.score.get("beamThickness"), 0.62);

  w.ids["#engraving-cancel"].onclick();
  await settle();
  assert.deepEqual([...w.score.entries()], [["noteBase", 6]],
    "the score is not back to what it was");
  assert.ok(!w.score.has("beamThickness"),
    "it was frozen at the house style instead of going back to following it");
});

test("cancelling having changed nothing asks the score for nothing", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();
  w.ids["#engraving-cancel"].onclick();
  await settle();
  assert.deepEqual(w.posts, [], "it wrote to the score anyway");
  assert.deepEqual(w.drawn, [], "it redrew the page for nothing");
});

// Several changes go back in one request. One at a time would redraw the
// page once per measurement and could stop half way, leaving a score that
// is neither what it was nor what was asked for.
test("everything goes back at once", async () => {
  const w = open();
  await w.api.engravingDialog();
  await settle();
  await type(w, "noteBase", 7);
  await type(w, "staffGap", 14);
  await type(w, "beamThickness", 0.62);

  const before = w.posts.length;
  w.ids["#engraving-cancel"].onclick();
  await settle();
  assert.equal(w.posts.length, before + 1, "cancel wrote more than once");
  assert.deepEqual(w.posts[before],
    { noteBase: "", staffGap: "", beamThickness: "" });
  assert.deepEqual([...w.score.entries()], []);
});

// Typing a number and clicking Cancel is one gesture, and the field
// commits what was typed as it loses the focus — so the change is still in
// the air when Cancel is pressed. Cancel worked out what to put back from
// before it landed, and the number nobody wanted was the one that survived.
test("a number typed as cancel is clicked does not survive it", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();

  const f = field(w, "noteBase");
  f.value = "7";
  f.fire("change");                       // not waited for, as a browser does
  w.ids["#engraving-cancel"].onclick();
  await settle();
  await settle();
  assert.equal(w.score.get("noteBase"), 6);
});

// And the other way round, for whatever arrives late: once the window has
// been given up on, nothing it contains gets to write to the score again.
test("a change that arrives after cancel is dropped", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();
  await type(w, "noteBase", 7);

  w.ids["#engraving-cancel"].onclick();
  await settle();
  await type(w, "noteBase", 8);
  assert.equal(w.score.get("noteBase"), 6,
    "a field in a window nobody is in any more wrote to the score");
});

test("done keeps what was changed", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();
  await type(w, "noteBase", 7);
  w.ids["#engraving-done"].onclick();
  await settle();
  assert.equal(w.score.get("noteBase"), 7);
  assert.equal(w.dlg.shown, false);
});

// Dismissing a window is how most people say they did not mean it, and a
// window with a Cancel button in it that keeps the changes when it is
// dismissed is a window that lies.
test("escape is cancel", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();
  await type(w, "noteBase", 7);

  await w.dlg.close();        // what Escape does
  await settle();
  assert.equal(w.score.get("noteBase"), 6);
});

// Opening it again starts from where the score now is, rather than from the
// first time it was ever opened.
test("the window does not remember a previous visit", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();
  await type(w, "noteBase", 7);
  w.ids["#engraving-done"].onclick();
  await settle();

  await w.api.engravingDialog();
  await settle();
  w.ids["#engraving-cancel"].onclick();
  await settle();
  assert.equal(w.score.get("noteBase"), 7, "it went back to a closed window");
});

/* ---------- reset all ---------- */

test("reset all asks before throwing the score's own settings away", async () => {
  const w = open({ set: { noteBase: 6, beamThickness: 0.55 } });
  await w.api.engravingDialog();
  await settle();

  await w.ids["#engraving-reset"].onclick();
  await settle();
  assert.equal(w.asked.length, 1, "it did not ask");
  assert.match(w.asked[0].title, /remove all the specific settings you used/);
  assert.match(w.asked[0].title, /are you sure/);
  // What goes, in numbers, so the answer can be given knowing the size of
  // it.
  assert.match(w.asked[0].note, /\b2 measurements\b/);
  assert.deepEqual([...w.score.entries()], []);
});

// Said in English rather than assembled from a number and a word: "The 1
// measurement set for this score go back" is what the window asked before.
test("the question counts in words that read", async () => {
  const w = open({ set: { noteBase: 6 } });
  await w.api.engravingDialog();
  await settle();
  await w.ids["#engraving-reset"].onclick();
  await settle();
  assert.match(w.asked[0].note, /The one measurement .* goes back/);
});

test("saying no to reset all changes nothing", async () => {
  const w = open({ set: { noteBase: 6 }, sure: false });
  await w.api.engravingDialog();
  await settle();

  await w.ids["#engraving-reset"].onclick();
  await settle();
  assert.equal(w.asked.length, 1);
  assert.deepEqual([...w.score.entries()], [["noteBase", 6]]);
  assert.deepEqual(w.posts, [], "it reset anyway and then put it back");
});

// A question whose answer changes nothing is a question not worth asking.
test("reset all does not ask when there is nothing to lose", async () => {
  const w = open();
  await w.api.engravingDialog();
  await settle();
  await w.ids["#engraving-reset"].onclick();
  await settle();
  assert.deepEqual(w.asked, []);
});

test("the question names the part when a part is being looked at", async () => {
  const w = open({ set: { noteBase: 6 }, of: "Violin II" });
  await w.api.engravingDialog();
  await settle();
  await w.ids["#engraving-reset"].onclick();
  await settle();
  assert.match(w.asked[0].note, /the Violin II part/);
});

// Which is what makes the question safe to answer: saying yes to it is not
// the last word.
test("resetting everything is itself something cancel takes back", async () => {
  const w = open({ set: { noteBase: 6, beamThickness: 0.55 } });
  await w.api.engravingDialog();
  await settle();

  await w.ids["#engraving-reset"].onclick();
  await settle();
  assert.deepEqual([...w.score.entries()], []);

  w.ids["#engraving-cancel"].onclick();
  await settle();
  assert.deepEqual([...w.score.entries()].sort(),
    [["beamThickness", 0.55], ["noteBase", 6]]);
});
