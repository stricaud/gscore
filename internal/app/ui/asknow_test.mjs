/*
 * asknow_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * An option that is a thing to do, not a thing to choose.
 *
 * "Download another…" is not an answer to "play through which soundfont",
 * it is the next window. Making somebody pick it and then press OK is a
 * press that decides nothing, so picking it opens what it names.
 *
 * With one guard. A closed dropdown on Windows and Linux moves its value
 * with the arrow keys and fires the same event a mouse does, so somebody
 * feeling their way down the list would open the download window on the way
 * past. A mouse press on the list comes first where it was a real choice.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");

test("the options that act are the ones that are actions", () => {
  // The soundfonts themselves must not act on being picked: picking one is
  // the answer, and OK is what says so.
  const dialog = src.match(
    /async function playbackSound\(\)[\s\S]*?^}/m);
  assert.ok(dialog, "app.js has no playbackSound");
  const body = dialog[0];

  for (const what of ["get", "file"]) {
    const line = new RegExp('value: "' + what + '"[\\s\\S]{0,200}?now: true');
    assert.match(body, line, what + " does not act when it is picked");
  }
  // The list of installed soundfonts is built separately and carries no
  // such flag, so choosing one still waits for OK.
  const fonts = body.match(/record\.fonts\.map\([\s\S]*?\}\)\);/);
  assert.ok(fonts, "playbackSound no longer lists the soundfonts");
  assert.doesNotMatch(fonts[0], /now:/,
    "picking a soundfont closes the dialog before it is confirmed");
});

test("picking one acts only when a pointer picked it", () => {
  const ask = src.match(/^function ask\(\{[\s\S]*?^}/m);
  assert.ok(ask, "app.js has no ask");
  const body = ask[0];

  assert.match(body, /addEventListener\("pointerdown"/,
    "nothing watches for a real press");
  assert.match(body, /pointedAt/,
    "the press is not remembered, so a keyboard change cannot be told apart");
  // And the listeners come off again: ask is one dialog reused by every
  // question in the program, and a listener left behind acts on the next.
  assert.match(body, /removeEventListener\("pointerdown"/);
  assert.match(body, /removeEventListener\("change"/);
});

test("it is off unless an option asks for it", () => {
  const ask = src.match(/^function ask\(\{[\s\S]*?^}/m);
  // No option carrying the flag means no listeners at all, so every
  // existing question behaves exactly as it did.
  assert.match(ask[0], /if \(actNow\.size/,
    "the listeners are added whether or not anything wants them");
});

test("choosing several at once is left alone", () => {
  // "Which instruments?" is a list you tick, and a dialog that closed on
  // the first tick would be unusable.
  const ask = src.match(/^function ask\(\{[\s\S]*?^}/m);
  assert.match(ask[0], /actNow\.size && !many/);
});
