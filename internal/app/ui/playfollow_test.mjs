/*
 * playfollow_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Following the music while it plays, on a score with a great deal of it.
 *
 * Every frame asked three questions of the whole score: which events are
 * sounding (a walk down every event in the performance), where this bar is
 * (a walk down every clickable region), and which shapes carry a given
 * number (an attribute selector run over the whole drawing). On Beethoven's
 * ninth that is a quarter of a million events, a hundred and thirty-eight
 * thousand regions and nine hundred thousand shapes — sixty times a second.
 * Pressing Play did nothing anybody would call playing.
 *
 * Indexed, each of those is a lookup. What the indexes answer has to be
 * exactly what the walks answered.
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

function build(names, scope) {
  const body = names.map(grab).join("\n") +
    "\nreturn { " + names.join(", ") + " };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
}

// A performance shaped like an orchestral one: many parts, notes of mixed
// length, and one very long held note to catch a window that is too short.
function performance(parts, bars) {
  const events = [];
  let ref = 1;
  for (let b = 0; b < bars; b++) {
    for (let p = 0; p < parts; p++) {
      events.push({ ref: ref++, part: p, start: b * 2, dur: 0.5 });
      events.push({ ref: ref++, part: p, start: b * 2 + 1, dur: 1.5 });
    }
  }
  // A pedal note under the whole of it.
  events.push({ ref: ref++, part: 0, start: 4, dur: 40 });
  return events;
}

test("what is sounding is what the walk down every event said", () => {
  const api = build(["eventIndex", "soundingAt"], { Math });
  const events = performance(12, 200);
  const when = api.eventIndex(events);
  const plain = (at) => events.filter((e) => at >= e.start && at < e.start + e.dur);

  for (const at of [0, 0.25, 0.5, 1, 1.4, 2, 5, 17.3, 41.9, 44, 100, 399.9]) {
    const got = api.soundingAt(when, at)
      .filter((e) => at >= e.start && at < e.start + e.dur)
      .map((e) => e.ref).sort((a, b) => a - b);
    const want = plain(at).map((e) => e.ref).sort((a, b) => a - b);
    assert.deepEqual(got, want, "at " + at + " seconds");
  }
});

test("a note held under the whole piece is still found", () => {
  // The window reaches back by the longest note there is, so a pedal note
  // forty seconds long is still sounding thirty seconds after it began.
  const api = build(["eventIndex", "soundingAt"], { Math });
  const held = [{ ref: 7, part: 0, start: 4, dur: 40 }];
  const when = api.eventIndex(held);
  const got = api.soundingAt(when, 38).map((e) => e.ref);
  assert.deepEqual(got, [7], "a long note dropped out of the window");
});

test("the window does not grow with the length of the piece", () => {
  const api = build(["eventIndex", "soundingAt"], { Math });
  const short = api.eventIndex(performance(12, 50));
  const long = api.eventIndex(performance(12, 5000));
  const a = api.soundingAt(short, 60).length;
  const b = api.soundingAt(long, 60).length;
  assert.equal(a, b,
    `a piece a hundred times longer looked at ${b} events where a short one ` +
    `looked at ${a}; the window is not bounded`);
});

test("a bar's box is taken from the index, not from every region", () => {
  const state = { hits: [], hitsByBar: null };
  const api = build(["indexHits", "hitsIn"], { state });
  const hits = [];
  for (let b = 0; b < 500; b++) {
    for (let p = 0; p < 10; p++) hits.push({ measure: b, part: p, x0: b, x1: b + 1 });
  }
  state.hits = hits;
  state.hitsByBar = api.indexHits(hits);
  const got = api.hitsIn(123).map((h) => h.part).sort((a, b) => a - b);
  assert.deepEqual(got, [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
  assert.deepEqual(api.hitsIn(99999), [], "a bar that is not there answered with something");

  // And without an index — which is how it behaves before the first draw —
  // it still answers, by walking.
  state.hitsByBar = null;
  assert.equal(api.hitsIn(123).length, 10, "the fallback walk stopped working");
});

test("a sounding note is found by number rather than searched for", () => {
  const fn = src.match(/^function follow\([\s\S]*?^}/m)[0];
  assert.ok(!/querySelectorAll\(`\[data-ref=/.test(fn),
    "every frame still runs an attribute selector over the whole drawing");
  assert.ok(fn.includes("refIndex("), "the shapes are not filed by their number");
  assert.ok(!/for \(const ev of pb\.events\)/.test(fn),
    "every frame still walks every event in the performance");
});
