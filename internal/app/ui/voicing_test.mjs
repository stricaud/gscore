/*
 * voicing_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The voicing controls in Part setup.
 *
 * One rule per staff, and the controls show the rule for whichever staff
 * is picked. The trap is the one every dialog of this shape falls into: a
 * staff is chosen, its rule is read into the boxes, another staff is
 * chosen and the boxes are not refilled — so the first staff's rule is
 * read back out and written onto the second.
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

const voicingOf = new Function(grab("voicingOf") + "\nreturn voicingOf;")();

test("a staff with no rule reads as every note", () => {
  const got = voicingOf({ voicing: [] }, 2);
  assert.equal(got.pick, "all");
  assert.equal(got.layer, 0);
  assert.equal(got.staff, 2);
});

test("a staff's own rule is found among several", () => {
  const sh = {
    voicing: [
      { staff: 0, pick: "top", nth: [], layer: 0 },
      { staff: 3, pick: "bottom", nth: [], layer: 2 },
    ],
  };
  assert.equal(voicingOf(sh, 3).pick, "bottom");
  assert.equal(voicingOf(sh, 3).layer, 2);
  assert.equal(voicingOf(sh, 0).pick, "top");
  // And a staff with no rule of its own is not given somebody else's.
  assert.equal(voicingOf(sh, 1).pick, "all");
});

/*
 * The controls.
 */

test("the chord controls only show for a rule that counts notes", () => {
  const body = grab("syncVoicingRows");
  assert.match(body, /#voice-pick.*?=== "nth"/s);
  assert.match(body, /#voice-nth-row/);
  assert.match(body, /#voice-single-row/);
});

test("the edit-in-the-score notice shows whenever notes are chosen", () => {
  // Both ways of choosing: picking out of chords, and picking a layer.
  const body = grab("syncVoicingRows");
  assert.match(body, /!== "all"/);
  assert.match(body, /#voice-layer.*?> 0/s);
  assert.match(body, /#voice-note/);
});

test("changing the staff refills the controls", () => {
  const body = grab("setupPartDialog");
  assert.match(body, /#voice-staff.*?addEventListener\("change"/s);
  assert.match(body, /showVoicingFor\(sh\)/);
});

test("the staff picker offers the part's own staves and no others", () => {
  const body = grab("drawVoicing");
  assert.match(body, /sh\.staves \|\| \[\]/);
  // Named, not just numbered: "2" is not an answer to which staff.
  assert.match(body, /names\[n\]\?\.name/);
});

test("a rule counting notes must say which", () => {
  const body = grab("applyVoicing");
  assert.match(body, /if \(nth === ""\)/);
  assert.match(body, /return false/);
});

test("applying the part sends the voicing first and stops if it fails", () => {
  const body = grab("applyPartSetup");
  assert.match(body, /if \(!await applyVoicing\(\)\) return;/);
});

test("the dialog has every control the rule needs", () => {
  for (const id of ["voice-staff", "voice-pick", "voice-nth", "voice-from",
                    "voice-single", "voice-layer", "voice-note"]) {
    assert.ok(html.includes(`id="${id}"`), "index.html has no #" + id);
  }
});

test("the wording says what a shared note does, because that is the trap", () => {
  assert.match(html, /written once is played by both/);
});
