/*
 * removestaff_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Asking before taking a staff away.
 *
 * It used to ask you to type the staff's name, and refused unless you got
 * it right. That is a lock rather than a question: slow when it works, and
 * on a staff with no name of its own it asked for the words "that staff",
 * which nobody would guess — so the staff simply never went away, with
 * "the name did not match" as the only explanation.
 *
 * A question that needs "go on" or "don't" is two buttons. What makes it
 * safe is that it can be taken back, and a test in internal/app checks that
 * the staff and its music both come back.
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

test("removing a staff asks a question rather than setting a test", () => {
  const fn = grab("removeStaff");
  assert.ok(fn.includes("confirmThat("), "it still asks for something to be typed");
  assert.ok(!fn.includes("ask({"), "it still opens a box to type in");
  assert.ok(!/toLowerCase\(\)/.test(fn), "it still compares what was typed to the name");
  assert.ok(!fn.includes("the name did not match"),
    "it can still refuse over a name");
  // And it says what will happen and how to take it back.
  assert.ok(/Undo brings it back/.test(fn),
    "nothing says the removal can be undone");
  assert.ok(fn.includes("A score has to have a staff"),
    "removing the only staff is left to fail at the server with no explanation");
});

test("the staff about to go is named, or numbered when it has no name", () => {
  const fn = grab("removeStaff");
  assert.ok(/parts\[part\]\?\.name \|\| `staff \$\{part \+ 1\}`/.test(fn),
    'an unnamed staff is called something nobody can act on — it used to be ' +
    'the words "that staff", which was also what you had to type');
});

// confirmThat, with the dialog as a handful of flags.
function dialog() {
  const el = {};
  const names = ["#ask", "#ask-title", "#ask-note", "#ask-label",
                 "#ask-input", "#ask-select", "#ask-filter", "#ask-ok"];
  for (const n of names) el[n] = { hidden: false, textContent: "" };
  let onClose = null;
  el["#ask"].addEventListener = (_, fn) => { onClose = fn; };
  el["#ask"].removeEventListener = () => { onClose = null; };
  el["#ask"].showModal = () => {};
  el["#ask-ok"].focus = () => {};
  el["#ask-ok"].textContent = "OK";

  const scope = { $: (s) => el[s], setTimeout: (fn) => fn(), Promise };
  const api = new Function(...Object.keys(scope),
    grab("confirmThat") + "\nreturn { confirmThat };")(...Object.values(scope));
  return {
    ...api, el,
    answer(v) { el["#ask"].returnValue = v; if (onClose) onClose(); },
  };
}

test("the question has nothing to type in it", async () => {
  const d = dialog();
  const p = d.confirmThat({ title: "Remove this staff?", note: "Gone.", ok: "Remove" });
  assert.equal(d.el["#ask-input"].hidden, true, "there is still a box to type in");
  assert.equal(d.el["#ask-select"].hidden, true);
  assert.equal(d.el["#ask-filter"].hidden, true);
  assert.equal(d.el["#ask-label"].hidden, true, "the label for the empty box is still there");
  assert.equal(d.el["#ask-title"].textContent, "Remove this staff?");
  assert.equal(d.el["#ask-note"].textContent, "Gone.");
  assert.equal(d.el["#ask-ok"].textContent, "Remove",
    "the button says OK rather than what it will do");
  d.answer("ok");
  assert.equal(await p, true);
});

test("anything but yes is no", async () => {
  for (const said of ["cancel", "", undefined]) {
    const d = dialog();
    const p = d.confirmThat({ title: "Remove this staff?" });
    d.answer(said);
    assert.equal(await p, false, `closing with ${JSON.stringify(said)} removed it`);
  }
});

test("the dialog is left as the next question will want it", async () => {
  const d = dialog();
  const p = d.confirmThat({ title: "Remove this staff?", ok: "Remove" });
  d.answer("ok");
  await p;
  assert.equal(d.el["#ask-label"].hidden, false,
    "the next question that needs a label will not have one");
  assert.equal(d.el["#ask-ok"].textContent, "OK",
    "the next question's button still says Remove");
});

test("the dialog it borrows is the one in the page", () => {
  for (const id of ["ask", "ask-title", "ask-note", "ask-label",
                    "ask-input", "ask-select", "ask-filter", "ask-ok"]) {
    assert.ok(html.includes(`id="${id}"`), "index.html has no " + id);
  }
});
