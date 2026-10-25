/*
 * accidental_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The accidental held for the next note.
 *
 * Clicking the natural with nothing selected arms it, and that was a mode
 * with nothing on the screen to say so: the palette marks one tool and the
 * note value always won, so the button never lit. It then stayed armed, and
 * every note written after it carried a natural it was never asked for.
 * Reported as "I did not see the natural was selected, and now all the notes
 * I am adding carry it".
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// The editor's own functions, with the palette replaced by a note of which
// buttons it was told to light.
function load(tool) {
  const state = { tool: { duration: "quarter", dots: 0, accidental: "none", rest: false, drum: null } };
  Object.assign(state.tool, tool);
  const lit = new Set();
  const scope = {
    state,
    markPaletteTool(predicate) {
      lit.clear();
      for (const label of ["Quarter", "Eighth", "Quarter rest", "Natural", "Sharp",
                           "Flat", "Double sharp", "Double flat"]) {
        if (predicate(label)) lit.add(label);
      }
    },
  };
  const body = grab("currentToolLabel") + "\n" + grab("currentAccidentalLabel") + "\n" +
    grab("markTools") + "\n" + grab("useAccidental") + "\n" +
    "return { markTools, useAccidental };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { state, lit, ...api };
}

test("the button of a held accidental is lit, and so is the note value", () => {
  const { lit, markTools } = load({ accidental: "natural" });
  markTools();
  assert.ok(lit.has("Natural"), "the natural is held and its button is not lit");
  assert.ok(lit.has("Quarter"), "the note value stopped being lit");
  assert.ok(!lit.has("Sharp"), "a button for an accidental that is not held is lit");
});

test("nothing held lights only the note value", () => {
  const { lit, markTools } = load({});
  markTools();
  assert.deepEqual([...lit], ["Quarter"]);
});

test("the accidental goes off once the note it was for is written", () => {
  const { state, lit, markTools, useAccidental } = load({ accidental: "natural" });
  markTools();
  useAccidental();
  assert.equal(state.tool.accidental, "none",
               "the accidental is still held after the note that carried it");
  assert.ok(!lit.has("Natural"), "its button is still lit");
  assert.ok(lit.has("Quarter"), "the note value went out with it");
});

test("writing a second note with none held changes nothing", () => {
  const { state, markTools, useAccidental } = load({});
  markTools();
  useAccidental();
  assert.equal(state.tool.accidental, "none");
});
