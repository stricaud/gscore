/*
 * clipboard_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Copy, cut and paste by the keys everybody already knows.
 *
 * They are bound as Mod+c, Mod+x and Mod+v like every other command, and in
 * a browser that is the whole story. A native window is not a browser: a web
 * view answers those three itself, as the editing commands they are
 * everywhere else on the system, and tells the page about them as copy, cut
 * and paste events rather than as keystrokes. Bound to the keystroke alone
 * they did nothing at all in the window most people run gscore in.
 *
 * Both have to work and neither may fire twice.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// installed runs setupClipboard against a document of our own and gives back
// the listeners it registered, with a note of what each command did.
function installed({ dialogOpen = false, pastingSymbol = false,
                     picture = null } = {}) {
  const done = [];
  const listeners = {};
  const state = { pastingSymbol };
  const document = {
    addEventListener: (name, fn) => { listeners[name] = fn; },
    querySelector: (sel) =>
      sel === "dialog[open]" && dialogOpen ? {} : null,
  };
  new Function("document", "state", "status", "pictureInPaste",
    "useClipboardPicture", "copySelection", "cutSelection", "pasteClipboard",
    grab("setupClipboard") + "\nreturn setupClipboard;")(
      document,
      state,
      (m) => done.push("said: " + m),
      () => picture,
      () => done.push("picture"),
      () => done.push("copy"),
      () => done.push("cut"),
      () => done.push("paste"))();
  return { listeners, done, state };
}

// An event as a web view sends it, with somewhere for preventDefault to be
// recorded.
function clipboardEvent(target) {
  return {
    target,
    stopped: false,
    preventDefault() { this.stopped = true; },
  };
}

test("the three events reach the music", () => {
  const { listeners, done } = installed();
  for (const name of ["copy", "cut", "paste"]) {
    assert.ok(listeners[name], "nothing is listening for " + name);
    listeners[name](clipboardEvent(null));
  }
  assert.deepEqual(done, ["copy", "cut", "paste"]);
});

// Stopped where it is handled, or the web view copies the page as well.
test("the event is stopped", () => {
  const { listeners } = installed();
  const e = clipboardEvent(null);
  listeners.copy(e);
  assert.equal(e.stopped, true);
});

// Words in a box are the browser's business: a verse is copied and pasted as
// words, by the same keys, and nothing here may take that away.
test("text in a field is left to the browser", () => {
  const { listeners, done } = installed();
  const field = { matches: (sel) => sel === "input, textarea" };
  for (const name of ["copy", "cut", "paste"]) {
    const e = clipboardEvent(field);
    listeners[name](e);
    assert.equal(e.stopped, false, name + " was taken from the text field");
  }
  assert.deepEqual(done, []);
});

// A dialog has its own keys. The same rule the keystrokes follow.
test("an open dialog keeps them", () => {
  const { listeners, done } = installed({ dialogOpen: true });
  for (const name of ["copy", "cut", "paste"]) {
    listeners[name](clipboardEvent(null));
  }
  assert.deepEqual(done, []);
});

// And the keystrokes are still bound, so a browser that delivers the key
// rather than the event is served by the same three commands.
test("the keystrokes are bound as well", () => {
  for (const [id, keys] of [["copy", "Mod\\+c"], ["cut", "Mod\\+x"],
                            ["paste", "Mod\\+v"]]) {
    const re = new RegExp(`id: "${id}"[\\s\\S]{0,120}?keys: "${keys}"`);
    assert.ok(re.test(src), id + " is not on its keystroke any more");
  }
});

// The keystroke path stops the key where it handles it, which is what keeps
// the editing command behind it from happening and sending an event too.
// Without it every cut would cut twice and every paste paste twice.
test("a handled keystroke is stopped, so no event follows it", () => {
  const keys = grab("setupKeys");
  const at = keys.indexOf("const bound = bindingFor(e);");
  assert.ok(at > 0, "setupKeys no longer looks up a binding");
  const after = keys.slice(at);
  assert.ok(after.indexOf("e.preventDefault()") < after.indexOf("bound.run()"),
    "a bound keystroke is run without being stopped first");
});

/*
 * A run of bars, picked up from the band over them.
 *
 * The band took one bar and had no way to say any more, so a passage — which
 * is almost never one bar — had to be picked up note by note. Reported as "I
 * want to be able to select multiple bars, here there is only one".
 */

// pickBars runs selectMeasure against stubs and gives back what it set.
function pickBars(before, ...args) {
  const state = { selection: null, multi: [], anchor: null, focus: null,
                  hits: [], barAnchor: null, ...before };
  const said = [];
  const fn = new Function("state", "markSelection", "status", "barNumberOf",
    "modKeyName", grab("selectMeasure") + "\nreturn selectMeasure;")(
      state, () => {}, (m) => said.push(m), (n) => n + 1, () => "Ctrl");
  fn(...args);
  return { state, said };
}

test("one bar is a run of one", () => {
  const { state } = pickBars({}, 0, 4);
  assert.equal(state.selection.measure, 4);
  assert.equal(state.selection.barTo, 4);
  assert.equal(state.selection.bar, true);
});

test("a run of bars is kept end to end", () => {
  const { state, said } = pickBars({}, 0, 4, 7);
  assert.equal(state.selection.measure, 4);
  assert.equal(state.selection.barTo, 7);
  assert.match(said.join(" "), /4 bars/);
});

// Shift-clicking before the bar already picked up takes in the bars between
// just the same, rather than asking for a run that runs backwards.
test("a run picked up backwards is still a run", () => {
  const { state } = pickBars({}, 0, 7, 4);
  assert.equal(state.selection.measure, 4);
  assert.equal(state.selection.barTo, 7);
});

// The anchor is where a run grows from, so it is set by a plain click and
// left alone by one that extends.
test("extending does not move the anchor", () => {
  const first = pickBars({}, 0, 4);
  assert.deepEqual(first.state.barAnchor, { part: 0, measure: 4 });
  const again = pickBars({ barAnchor: { part: 0, measure: 4 } }, 0, 4, 9);
  assert.deepEqual(again.state.barAnchor, { part: 0, measure: 4 });
});

/*
 * And a picture pasted in, where one was asked for.
 *
 * Insert ▸ Symbol ▸ From the clipboard reads the clipboard where it can. A
 * web view may not let it, and then the paste the person makes themselves
 * carries the same picture — so the next one is taken as the answer, once.
 */

test("a picture is taken when one was asked for", () => {
  const { listeners, done, state } = installed(
    { pastingSymbol: true, picture: { size: 10 } });
  listeners.paste(clipboardEvent(null));
  assert.deepEqual(done, ["picture"]);
  assert.equal(state.pastingSymbol, false, "it is still waiting for another");
});

// One paste only: the keys go back to meaning music straight after.
test("the next paste is music again", () => {
  const { listeners, done } = installed(
    { pastingSymbol: true, picture: { size: 10 } });
  listeners.paste(clipboardEvent(null));
  listeners.paste(clipboardEvent(null));
  assert.deepEqual(done, ["picture", "paste"]);
});

// Asked for a picture and given none: say so, and carry on meaning music.
test("a paste with no picture in it says so", () => {
  const { listeners, done } = installed({ pastingSymbol: true, picture: null });
  listeners.paste(clipboardEvent(null));
  assert.deepEqual(done, ["said: There is no picture on the clipboard", "paste"]);
});

// Nothing was asked for, so a picture on the clipboard is none of our
// business: the keys mean music.
test("a picture is not taken unless it was asked for", () => {
  const { listeners, done } = installed({ picture: { size: 10 } });
  listeners.paste(clipboardEvent(null));
  assert.deepEqual(done, ["paste"]);
});
