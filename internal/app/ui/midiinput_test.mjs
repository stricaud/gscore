/*
 * midiinput_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Playing the music in.
 *
 * What a keyboard sends is messier than it looks, and each piece of the
 * mess silently writes the wrong thing: a note-on with no velocity is how
 * most keyboards send note-off, so taking it for a key press writes every
 * note twice; two writes in flight at once both aim at where the cursor
 * was before either landed, which on a chord is five notes in the wrong
 * bar; and the channel in the bottom nibble has to be ignored or a
 * keyboard set to channel 3 is not heard at all.
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

// onMidiMessage is given a fake midiIn so what it decided can be read back.
function listener() {
  const played = [];
  const fn = new Function("midiIn", "playedNote",
    grab("onMidiMessage") + "\nreturn onMidiMessage;")(
      {}, (n) => played.push(n));
  return { fn, played };
}

test("a key press is written", () => {
  const { fn, played } = listener();
  fn({ data: [0x90, 60, 100] });
  assert.deepEqual(played, [60]);
});

// A note-on with no velocity is note-off on most keyboards. Taken for a
// press, every note is written twice.
test("a note-on with no velocity is a key being let go", () => {
  const { fn, played } = listener();
  fn({ data: [0x90, 60, 0] });
  assert.deepEqual(played, []);
});

test("a note-off is not a key press", () => {
  const { fn, played } = listener();
  fn({ data: [0x80, 60, 64] });
  assert.deepEqual(played, []);
});

// The bottom nibble is the channel. A keyboard set to channel 3 sends
// 0x92, and matching on the whole byte would never hear it.
test("the channel is ignored", () => {
  const { fn, played } = listener();
  for (let ch = 0; ch < 16; ch++) fn({ data: [0x90 | ch, 60 + ch, 100] });
  assert.equal(played.length, 16, "some channels were not heard");
});

test("the knobs and wheels are not notes", () => {
  const { fn, played } = listener();
  fn({ data: [0xb0, 7, 100] });   // a controller
  fn({ data: [0xe0, 0, 64] });    // pitch bend
  fn({ data: [0xf8] });           // clock
  assert.deepEqual(played, []);
});

/* ---------- one at a time ---------- */

// Each write asks the server where the cursor ends up. Two in flight both
// work from where it was before either landed.
test("notes are written one at a time, in order", () => {
  const body = grab("drainMidi");
  assert.match(body, /if \(midiIn\.busy\) return/);
  assert.match(body, /midiIn\.queue\.shift\(\)/);
  assert.match(body, /await writePlayedNote/);
  // And the guard is always let go, or one failed write stops the
  // keyboard for the rest of the session.
  assert.match(body, /finally \{\s*midiIn\.busy = false/);
});

test("a key press joins the queue rather than writing straight away", () => {
  const body = grab("playedNote");
  assert.match(body, /midiIn\.queue\.push/);
  assert.match(body, /drainMidi\(\)/);
});

/* ---------- chords ---------- */

// No two fingers land at the same millisecond, so "together" is a window.
test("keys pressed together go onto one note", () => {
  const body = grab("writePlayedNote");
  assert.match(body, /at - midiIn\.chord\.at < midiIn\.window/);
  assert.match(body, /api\/addpitch/);
  // And only where the cursor has not been moved in between.
  assert.match(body, /midiIn\.chord\.part === sel\.part/);
  assert.match(body, /midiIn\.chord\.measure === sel\.measure/);
});

test("the window is short enough that two notes are two", () => {
  const m = /window:\s*(\d+)/.exec(src.slice(src.indexOf("const midiIn = {")));
  assert.ok(m, "midiIn has no window");
  const ms = Number(m[1]);
  assert.ok(ms >= 20 && ms <= 120, `${ms}ms is not a hand's worth`);
});

/* ---------- the pitch ---------- */

// The number is sent as it is and spelled on the server, where the key
// signature is. Spelling it here would be a second opinion about the same
// note.
test("the number is sent as it arrived, to be spelled by the key", () => {
  const body = grab("writePlayedNote");
  assert.match(body, /midi: String\(note\)/);
  assert.doesNotMatch(body, /sharp|flat|%12/i,
    "the editor is spelling the note itself");
});

test("the note is written with the length the toolbar holds", () => {
  const body = grab("writePlayedNote");
  assert.match(body, /dur: state\.tool\.duration/);
  assert.match(body, /state\.tool\.dots/);
});

test("the caret follows the note into whichever bar it landed in", () => {
  const body = grab("writePlayedNote");
  assert.match(body, /sel\.measure = where\.measure/);
  assert.match(body, /carryCaretOn\(sel\)/);
});

/* ---------- reaching the keyboard ---------- */

// Web MIDI is in Chrome, Edge and Opera and not in Safari, which is what
// the macOS application is built on. Saying so beats failing silently.
test("a build that cannot listen says so", () => {
  const body = grab("midiDialog");
  assert.match(body, /midiSupported\(\)/);
  assert.match(body, /Chrome, Edge or Opera/);
  const probe = grab("midiSupported");
  assert.match(probe, /navigator\.requestMIDIAccess/);
});

test("sysex is not asked for", () => {
  // Nothing here needs it, and asking turns a mild permission prompt into
  // one most people refuse.
  assert.match(grab("openMidi"), /sysex: false/);
});

test("a keyboard plugged in later is still found", () => {
  assert.match(grab("openMidi"), /onstatechange/);
});

test("only one keyboard is listened to at a time", () => {
  const body = grab("listenToMidi");
  assert.match(body, /for \(const port of midiIn\.inputs\) port\.onmidimessage = null/);
});

test("the menu entry is there, live, and in French", () => {
  const m = html.match(/data-action="midi-input"[^>]*>/);
  assert.ok(m, "no MIDI menu entry");
  assert.ok(!m[0].includes("disabled"));
  assert.match(src, /case "midi-input": await midiDialog\(\)/);
  assert.ok(fr.includes('"MIDI keyboard…":'), "no French for the entry");
});
