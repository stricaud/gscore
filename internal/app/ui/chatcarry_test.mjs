/*
 * chatcarry_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * A request to the assistant that stopped partway: the chat says what was
 * kept, and offers to carry on from the bar after it.
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

test("carrying on names the request and the bar to carry on from", () => {
  const carry = new Function(grab("carryOnPrompt") + "; return carryOnPrompt;")();
  const p = carry("thirty bars of a blues", 17);
  assert.match(p, /thirty bars of a blues/);
  assert.match(p, /up to bar 16/);
  assert.match(p, /from bar 17/);
});

test("kept bars are said, and Continue asks again from the next bar", () => {
  const said = [];
  const offers = [];
  const asked = [];
  const guide = new Function("chatSay", "chatOffer", "askAssistant", "modKeyName", "t",
    grab("carryOnPrompt") + grab("chatGuide") + "; return chatGuide;")(
    (kind, text) => { said.push([kind, text]); return {}; },
    (p, label, run) => offers.push({ label, run }),
    (again) => asked.push(again), () => "⌘", (s) => s);
  guide("thirty bars", { bars: 8, at: 9, next: 17 }, true);
  assert.match(said[0][1], /8 bars/);
  assert.match(said[0][1], /9–16/);
  assert.equal(offers.length, 1);
  assert.match(offers[0].label, /17/);
  offers[0].run();
  assert.equal(asked[0].bar, 17);
  assert.equal(asked[0].original, "thirty bars");
});

test("too long with nothing kept says what to do instead", () => {
  const said = [];
  const guide = new Function("chatSay", "chatOffer", "askAssistant", "modKeyName", "t",
    grab("carryOnPrompt") + grab("chatGuide") + "; return chatGuide;")(
    (kind, text) => { said.push(text); return {}; }, () => {}, () => {}, () => "⌘", (s) => s);
  guide("a whole symphony", null, true);
  assert.match(said[0], /fewer bars/);
  assert.match(said[0], /ai\.log/);
});
