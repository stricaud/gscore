/*
 * pointing_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * What a click means, and where play starts from.
 *
 * A click used to write a note, and shift-click was the only way to pick one
 * up. That is the wrong way round: picking something up is the commonest
 * thing anybody does to a score, and it needed two hands every time.
 *
 * So the mouse points and the keyboard writes, and Insert ▸ Write with the
 * mouse turns the old way back on for anyone who prefers it — with shift
 * still picking up while it is on.
 *
 * And because a click now picks something up, pressing play starts from
 * whatever that is. Going to bar four hundred and pressing play used to
 * start the piece from the top.
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

test("a click points, unless writing with the mouse is on", () => {
  const fn = grab("setupCanvasClicks");
  assert.ok(/const picking = !state\.mouseWrites \|\| e\.shiftKey;/.test(fn),
    "a plain click still writes a note instead of picking one up");
  // And nothing is written by a click that is picking something up: that is
  // what stops a click building a chord on the note it meant to select.
  assert.ok(/stackable =\s*\n?\s*!picking/.test(fn),
    "a click that picks something up can still stack a pitch on it");
});

test("the switch starts off, is remembered, and ticks its menu", () => {
  assert.ok(/mouseWrites: false,/.test(src),
    "the mouse writes by default, which is the thing being changed");

  const set = grab("setMouseWrites");
  assert.ok(set.includes("localStorage.setItem"),
    "the choice is forgotten the moment another file is opened");
  assert.ok(set.includes("markViewTicks()"), "the menu does not say which way it is set");
  const load = grab("loadMouseWrites");
  assert.ok(load.includes("localStorage.getItem"), "nothing brings the choice back");

  assert.ok(html.includes('id="mouse-tick"'), "the menu item has no tick");
  const ticks = grab("markViewTicks");
  assert.ok(ticks.includes("#mouse-tick"), "the tick is never filled in");

  // And the menu item is wired to the switch rather than to a help line.
  const act = src.match(/case "note-input":[\s\S]*?break;/)[0];
  assert.ok(act.includes("setMouseWrites("), "the menu item still only explains itself");
});

// startOfSelection, with a performance and whatever is picked up.
function player({ selection = null, multi = [] } = {}) {
  const state = { selection, multi };
  const scope = { state, orderedSelection: () => multi };
  const api = new Function(...Object.keys(scope),
    grab("startOfSelection") + "\nreturn { startOfSelection };")(...Object.values(scope));
  return api.startOfSelection;
}

// Four bars, the second of which is played twice because it is repeated.
const performance = {
  events: [
    { ref: 11, start: 0, dur: 1 },
    { ref: 12, start: 1, dur: 1 },
    { ref: 13, start: 2, dur: 1 },
  ],
  measures: [
    { index: 0, start: 0, dur: 2 },
    { index: 1, start: 2, dur: 2 },
    { index: 1, start: 4, dur: 2 },
    { index: 2, start: 6, dur: 2 },
  ],
};

test("a note picked up is where play starts", () => {
  const at = player({ selection: { measure: 0, ref: 13 } });
  assert.equal(at(performance), 2, "play did not start at the note that was picked up");
});

test("a bar picked up is where play starts, and a repeated bar is its first time through", () => {
  // A bar has no number of its own, so the answer is where the bar begins.
  assert.equal(player({ selection: { measure: 2, ref: null, bar: true } })(performance), 6);
  assert.equal(player({ selection: { measure: 1, ref: null, bar: true } })(performance), 2,
    "a bar inside a repeat started from its second time through");
});

test("a cursor standing in a bar is where play starts", () => {
  // Which is what a click on empty staff now leaves behind, and what going
  // to a measure leaves behind.
  assert.equal(player({ selection: { measure: 2, ref: null, at: 1, step: 4 } })(performance), 6);
});

test("a note whose number the performance does not know falls back to its bar", () => {
  // A rest, say, or an element that makes no sound: it is still in a bar.
  assert.equal(player({ selection: { measure: 2, ref: 999 } })(performance), 6);
});

test("with nothing picked up, play starts at the beginning", () => {
  assert.equal(player({ selection: null })(performance), null);
  assert.equal(player({ selection: { ref: null } })(performance), null,
    "a selection with no bar behind it claimed a place in the music");
});

test("play asks where the selection is rather than only looking for a note", () => {
  const fn = src.match(/^async function playNow\(opts\)[\s\S]*?^}/m)[0];
  assert.ok(fn.includes("startOfSelection(pb)"),
    "play still only starts from a note, so a bar starts the piece from the top");
  assert.ok(fn.includes("opts.fromStart"),
    "there is no way left to ask for the beginning");
});
