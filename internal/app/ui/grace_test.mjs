/*
 * grace_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Making a grace note from the editor.
 *
 * Everything else about them was already here — the model holds them, the
 * engraver sets them at cue size with their own beams and slur, playback
 * steals their time, and both the ABC and the Guitar Pro readers bring them
 * in. There was no way to write one: they could be imported and played and
 * never made. The same thing the Ornament submenu's own comment says about
 * the trill and the roll.
 *
 * What is tested here is the wiring, which is where a menu entry goes wrong:
 * an entry on the menu with no case to answer it does nothing at all, and
 * nothing on the page says so.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

const ACTIONS = ["grace", "grace-appoggiatura", "grace-none"];

test("every grace entry on the menu has a case that answers it", () => {
  for (const action of ACTIONS) {
    assert.ok(html.includes(`data-action="${action}"`),
      `the menu has no ${action} entry`);
    assert.ok(src.includes(`case "${action}":`),
      `${action} is on the menu with nothing to answer it`);
  }
});

test("the grace entries sit with the other ornaments", () => {
  // A trill, a roll and a grace note are the same kind of thing: a mark
  // written on a note. Put under Pedal or Octave it would not be found.
  const at = html.indexOf("<span>Ornament</span>");
  assert.ok(at > 0, "there is no Ornament submenu");
  const end = html.indexOf("</div>", html.indexOf("menu-items", at));
  const section = html.slice(at, end);
  for (const action of ACTIONS) {
    assert.ok(section.includes(`data-action="${action}"`),
      `${action} is not in the Ornament submenu`);
  }
});

// setGrace, with the posting replaced by a note of what it asked for.
function load() {
  const m = src.match(/^async function setGrace\([\s\S]*?^}/m);
  if (!m) throw new Error("app.js has no setGrace");
  const calls = [];
  const mark = async (endpoint, params, said) => {
    calls.push({ endpoint, params, said });
  };
  const body = m[0] + "\nreturn setGrace;";
  return { calls, setGrace: new Function("mark", body)(mark) };
}

test("a grace note is slashed and an appoggiatura is not", () => {
  // The stroke is not decoration: a crush is played before the beat and an
  // appoggiatura takes its time out of the note it leans on, and a player
  // does two different things with them.
  const { calls, setGrace } = load();
  setGrace("slashed");
  setGrace("appoggiatura");
  assert.equal(calls[0].endpoint, "grace");
  assert.equal(calls[0].params.slashed, "1");
  assert.equal(calls[1].params.slashed, "0");
});

test("making a group says so, rather than restyling one already there", () => {
  // slashed on its own changes the stroke on a group that exists; with
  // make it writes a new one. Without the flag, asking for an
  // appoggiatura on a plain note would quietly do nothing.
  const { calls, setGrace } = load();
  setGrace("appoggiatura");
  assert.equal(calls[0].params.make, "1");
});

test("taking them off asks for none and says so", () => {
  const { calls, setGrace } = load();
  setGrace("none");
  assert.equal(calls[0].params.grace, "none");
  assert.ok(/removed/i.test(calls[0].said),
    `the status line says ${JSON.stringify(calls[0].said)}`);
});
