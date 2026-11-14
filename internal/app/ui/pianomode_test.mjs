/*
 * pianomode_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The letters as a piano keyboard.
 *
 * Asked for as "I would love to use the keyboard like a piano so I can
 * select the range easily", with a two-part study full of thirds and
 * sixths to type in: dyad after dyad, which is two keys at once.
 *
 * Three rows, three octaves, every row starting on C. One finger is one
 * note name whichever row it is in, so there is a row to learn rather than
 * twenty-six keys, and three octaves are under the hands at once.
 *
 * Two things here are worth a test of their own. The map is built once and
 * the drawn keyboard is built from the same table, so the picture cannot
 * come to disagree with what the keys do — that is most of this file. And
 * the mode has to give the letters back: every letter means a note while
 * it is on, so a way out that does not depend on a letter is not a detail,
 * it is the difference between a mode and a trap.
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

// The map, run out of app.js rather than written out again here: a copy of
// it would be the second opinion this file exists to rule out.
function piano(shift = 0, labels = []) {
  const rows = /const PIANO_ROWS = \[[\s\S]*?\];/.exec(src)[0];
  const keys = /const PIANO_KEYS = new Map\(\);[\s\S]*?\n\}/.exec(src)[0];
  const body = rows + "\n" + keys + `
    const PIANO_NAMES = ["C", "D", "E", "F", "G", "A", "B"];
    const PIANO_SEMIS = [0, 2, 4, 5, 7, 9, 11];
    const pianoIn = {
      held: new Set(), shift: ${shift},
      labels: new Map(${JSON.stringify(labels)}),
    };
    ` + grab("pianoNoteOf") + "\n" + grab("pianoLabelOf") + "\n" +
    grab("pianoWhites") +
    "\nreturn { pianoNoteOf, pianoLabelOf, pianoWhites, PIANO_ROWS, PIANO_KEYS };";
  return new Function(body)();
}

const p = piano();

// A key is named by where it is, so the rows are rows whatever is printed
// on the keyboard. Said in letters here for the sake of reading.
const at = (letter) => "Key" + letter.toUpperCase();

test("three rows, three octaves, every row starting on C", () => {
  assert.equal(p.PIANO_ROWS.length, 3);
  for (const row of p.PIANO_ROWS) {
    assert.equal(p.pianoNoteOf(at(row.keys[0])).name[0], "C",
                 `the ${row.keys[0]} row does not start on C`);
  }
  // Bottom to top, which is the order the rows are written in and the
  // order the picture is drawn in reverse of.
  assert.deepEqual(p.PIANO_ROWS.map((r) => r.octave), [3, 4, 5]);
});

// The thing the layout is for: the same finger is the same note name
// wherever the hand is, so the row is learnt once.
test("one finger is one note name in every row", () => {
  for (let i = 0; i < 7; i++) {
    const names = p.PIANO_ROWS.map((r) => p.pianoNoteOf(at(r.keys[i])).name[0]);
    assert.equal(new Set(names).size, 1,
                 `the ${i + 1}th key of each row is ${names.join(", ")}`);
  }
});

test("the home row is the one under the hands", () => {
  assert.equal(p.pianoNoteOf(at("a")).name, "C4");
  assert.equal(p.pianoNoteOf(at("g")).name, "G4");
  assert.equal(p.pianoNoteOf(at("h")).name, "A4");
  // Middle C is middle C, and the rest is counted from it.
  assert.equal(p.pianoNoteOf(at("a")).midi, 60);
  assert.equal(p.pianoNoteOf(at("h")).midi, 69, "A4 is not at 440");
});

/* ---------- whatever is printed on the keyboard ---------- */

// Three rows one above another, the same finger the same note name in
// each. That is a fact about where the keys are and not about what they
// print, and on an AZERTY keyboard the letters are somewhere else: a map
// kept by letter would have the rows crossing over each other.
test("the keys are known by where they are, not by what they print", () => {
  for (const code of p.PIANO_KEYS.keys()) {
    assert.match(code, /^Key[A-Z]$/, `${code} is a letter and not a place`);
  }
  const handler = /document\.addEventListener\("keydown"[\s\S]*?\n  \}\);/.exec(src)[0];
  assert.match(handler, /PIANO_KEYS\.has\(e\.code\)/);
  assert.match(handler, /pianoKeyDown\(e\.code\)/);
  // And let go of by the same name, or a key would stay down for ever.
  assert.match(src, /keyup[\s\S]{0,120}pianoKeyUp\(e\.code\)/);
  // Asked for by that name everywhere, and not only where the keystrokes
  // come in. A lookup left saying "a" finds nothing and reads .name off
  // it, which is how the octave shift threw rather than saying where the
  // keyboard had got to.
  for (const m of src.matchAll(/pianoNoteOf\("([^"]*)"\)/g)) {
    assert.match(m[1], /^Key[A-Z]$/,
      `pianoNoteOf("${m[1]}") is asking by letter and will find nothing`);
  }
});

// All twenty-six places, so every letter on every keyboard is a note and
// the menus can say so without knowing the layout.
test("every letter is a note", () => {
  assert.equal(p.PIANO_KEYS.size, 26);
  assert.match(grab("syncMenuKeys"), /\/\^\[a-z\]\$\/\.test\(key\)/);
});

// What is written on the key is what is drawn on the picture, and the two
// are learnt apart: the browser says where it will, and the keys say so
// themselves where it will not.
test("the drawn letters follow the keyboard in front of you", () => {
  const azerty = piano(0, [["KeyA", "q"], ["KeyQ", "a"], ["KeyW", "z"], ["KeyZ", "w"]]);
  assert.equal(azerty.pianoLabelOf("KeyA"), "q", "C4 is drawn with the wrong letter");
  assert.equal(azerty.pianoNoteOf("KeyA").name, "C4", "C4 moved off the home row");
  assert.equal(azerty.pianoLabelOf("KeyS"), "s", "a key nobody has corrected");
  assert.match(grab("readKeyboardLayout"), /navigator\.keyboard\?\.getLayoutMap/);
  assert.match(grab("notePianoLabel"), /pianoIn\.labels\.set\(code, ch\)/);
  const handler = /document\.addEventListener\("keydown"[\s\S]*?\n  \}\);/.exec(src)[0];
  assert.match(handler, /notePianoLabel\(e\.code, e\.key\)/);
});

// Three octaves without reaching for anything, which is the range the mode
// was asked for.
test("three octaves are under the hands at once", () => {
  const all = p.pianoWhites();
  assert.equal(all[0].name, "C3");
  assert.equal(all[all.length - 1].name, "E6");
  // Diatonic and unbroken: a gap in the middle is a note that cannot be
  // typed at all, which is worse than one the hand has to stretch for.
  for (let i = 1; i < all.length; i++) {
    const step = all[i].midi - all[i - 1].midi;
    assert.ok(step === 1 || step === 2,
              `${all[i - 1].name} to ${all[i].name} is not one step`);
  }
});

// The rows overlap where the keys run past an octave, and the picture says
// so rather than quietly picking one: both letters are printed on the key.
test("a note two letters reach is drawn with both of them", () => {
  const c5 = p.pianoWhites().find((w) => w.name === "C5");
  assert.deepEqual(c5.keys, [at("k"), at("q")],
                   "C5 is reached from the home row and the top row");
});

test("the keyboard moves an octave at a time, for the parts outside it", () => {
  assert.equal(piano(-1).pianoNoteOf(at("a")).name, "C3");
  assert.equal(piano(2).pianoNoteOf(at("a")).name, "C6");
  const body = grab("shiftPianoOctave");
  assert.match(body, /Math\.max\(-3, Math\.min\(3,/, "it can be moved off the piano");
  assert.match(body, /renderPianoKeys\(\)/, "the picture is not redrawn");
});

/* ---------- a chord is two keys held together ---------- */

// The reason for the mode. A MIDI keyboard can only say that a key went
// down, so it is answered with a stopwatch; the typing keyboard can see
// that the first key is still held, which is both exact and what a piano
// does.
// Run rather than read. The two mistakes this file has already made were
// both a name left saying what it used to say — playedNote(midi) after the
// argument became the note, and pianoNoteOf("a") after the keys became
// places — and a test that greps for a pattern will happily pin either of
// them in place. Pressing the keys and watching what comes out cannot.
function pressing() {
  const played = [];
  const marked = [];
  const scope = {
    pianoIn: { held: new Set(), shift: 0, labels: new Map() },
    PIANO_KEYS: p.PIANO_KEYS,
    PIANO_NAMES: ["C", "D", "E", "F", "G", "A", "B"],
    PIANO_SEMIS: [0, 2, 4, 5, 7, 9, 11],
    playedNote: (note, join) => played.push({ note, join }),
    markPianoKeys: () => marked.push(true),
  };
  const body = [grab("pianoNoteOf"), grab("pianoPress"), grab("pianoRelease"),
                grab("pianoKeyDown"), grab("pianoKeyUp")].join("\n") +
    "\nreturn { pianoKeyDown, pianoKeyUp, pianoPress, held: pianoIn.held };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return { ...api, played };
}

test("a key pressed writes the note that is written on it", () => {
  const t = pressing();
  assert.equal(t.pianoKeyDown("KeyA"), true);
  assert.deepEqual(t.played, [{ note: "C4", join: false }],
                   "the key did not write a C4");
});

test("a key pressed while another is held is a notehead on it", () => {
  const t = pressing();
  t.pianoKeyDown(at("g"));           // G4, and still held
  t.pianoKeyDown(at("j"));           // B4, pressed on top of it
  assert.deepEqual(t.played.map((x) => [x.note, x.join]),
                   [["G4", false], ["B4", true]]);
});

test("and one pressed after the first is let go of is the next note", () => {
  const t = pressing();
  t.pianoKeyDown(at("g"));
  t.pianoKeyUp(at("g"));
  t.pianoKeyDown(at("j"));
  assert.deepEqual(t.played.map((x) => x.join), [false, false]);
});

test("a key that is not one of them writes nothing", () => {
  const t = pressing();
  assert.equal(t.pianoKeyDown("Digit4"), false);
  assert.deepEqual(t.played, []);
});

test("the chord is decided as the key goes down, not when the note lands", () => {
  const body = grab("pianoPress");
  const decided = body.indexOf("const join");
  const written = body.indexOf("playedNote(");
  assert.ok(decided > 0 && decided < written,
            "the write is a round trip and the hand has moved on by then");
});

/* ---------- a letter is a letter, not a pitch ---------- */

// The letters are the notes of the scale and the key signature says which
// notes those are. Sent as a sounding pitch instead, every white key the
// signature altered came out as the natural of that letter with a natural
// printed in front of it — three notes of every scale in three flats.
// Reported as "why are natural put? There is no need, I have to remove
// this all the time".
test("a letter key is sent as a letter, for the key to spell", () => {
  assert.match(grab("pianoKeyDown"), /pianoPress\(code, note\.name\)/);
  const render = grab("renderPianoKeys");
  assert.match(render, /pianoPlayable\(key, note\.keys\[0\], note\.name\)/,
               "a white key clicked with the mouse sends a pitch");
});

// A black key is not a letter of the scale and has nothing but its sound
// to go on, so it goes as a number and the key spells it: in E flat major
// the black key under B is a B flat and prints nothing.
test("a black key is sent as the pitch it sounds", () => {
  assert.match(grab("renderPianoKeys"),
               /pianoPlayable\(black, "#" \+ \(note\.midi \+ 1\), note\.midi \+ 1\)/);
});

test("which of the two it is decides what the request asks", () => {
  const body = grab("playedParam");
  assert.match(body, /typeof note === "number" \? "midi" : "pitch"/);
  // And both ways in use it, or one of them goes on asking the old
  // question: a note of its own and a notehead stacked on one.
  const write = grab("writePlayedNote");
  assert.equal((write.match(/playedParam\(p, note\)/g) || []).length, 2);
  assert.ok(!/midi: String\(note\)/.test(write),
            "a request still names the note as a number whatever it is");
});

test("letting go of a key is what ends the chord", () => {
  assert.match(grab("pianoKeyUp"), /pianoRelease\(code\)/);
  assert.match(grab("pianoRelease"), /pianoIn\.held\.delete\(id\)/);
});

// A key held down repeats. Holding is how a chord is asked for here, so a
// repeat taken for a finger writes the lower note over and over while the
// upper one is being reached for.
test("a key that repeats does not write the note again", () => {
  const handler = /document\.addEventListener\("keydown"[\s\S]*?\n  \}\);/.exec(src)[0];
  assert.match(handler, /if \(!e\.repeat\) pianoKeyDown\(e\.code\)/);
});

// A key down when the window loses the focus never sends its keyup, and a
// key the program still thinks is down makes the next note a chord with
// one written minutes ago.
test("a window that loses the focus has let go of everything", () => {
  assert.match(src, /window\.addEventListener\("blur", lettingGo\)/);
  assert.match(grab("lettingGo"), /pianoRelease\(k\)/);
});

/* ---------- the letters, and giving them back ---------- */

test("the letters are a note only while the mode is on", () => {
  const handler = /document\.addEventListener\("keydown"[\s\S]*?\n  \}\);/.exec(src)[0];
  assert.match(handler, /if \(state\.piano && !e\.shiftKey && !e\.altKey && !e\.ctrlKey && !e\.metaKey\)/);
});

// The staves that already type with letters and digits keep them: those
// are what that staff is written with, and nobody switched them on.
test("a drum staff and a tab staff still win on their own staves", () => {
  const handler = /document\.addEventListener\("keydown"[\s\S]*?\n  \}\);/.exec(src)[0];
  const drums = handler.indexOf("drumPadForKey(e.key)");
  const frets = handler.indexOf("typeFret");
  const piano = handler.indexOf("state.piano &&");
  assert.ok(drums > 0 && drums < piano, "piano mode takes a drum staff's letters");
  assert.ok(frets > 0 && frets < piano, "piano mode is reached before the frets");
});

// The way out cannot be a letter, because every letter is a note. This is
// the difference between a mode and a trap.
test("Escape gives the letters back", () => {
  const handler = /document\.addEventListener\("keydown"[\s\S]*?\n  \}\);/.exec(src)[0];
  const branch = /if \(state\.piano[\s\S]*?\n    \}/.exec(handler)[0];
  assert.match(branch, /e\.key === "Escape"[\s\S]*?setPiano\(false\)/);
  // And the key that starts it is not one of them either, for the same
  // reason: it could never stop it.
  const bind = /\{ id: "piano-mode",[\s\S]*?\},\n/.exec(src)[0];
  const keys = /keys: "([^"]+)"/.exec(bind)[1];
  assert.ok(!/^[a-z]$/.test(keys), `${keys} is a note while the mode is on`);
});

test("nothing but the letters is taken", () => {
  // The digits are the note lengths, the full stop is the dot and the
  // signs are the accidentals. Not one of their places is in the map.
  const places = ["Digit0", "Digit4", "Digit9", "Period", "Comma", "Slash",
                  "Equal", "Minus", "Semicolon", "Quote", "Backquote",
                  "Backslash", "Space", "Enter", "Backspace", "Tab"];
  for (const k of places) {
    assert.ok(!p.PIANO_KEYS.has(k), `${k} was taken from what it already did`);
  }
});

// A mode that has quietly taken every letter in the alphabet is one you
// want to have switched on yourself, not to find already on in the morning.
test("the mode is not remembered for next time", () => {
  assert.match(src, /piano: false,/);
  assert.ok(!/localStorage[^\n]*piano/i.test(src), "the mode is saved");
});

test("the menus say which keys are on loan", () => {
  assert.match(grab("syncMenuKeys"), /test\(key\)\) el\.dataset\.lent/);
  const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
  assert.match(css, /body\.piano-mode[^{]*kbd\[data-lent\]/);
  // Struck through and not disabled: the command still works from the
  // menu, it is only its key that is lent.
  assert.match(css, /body\.piano-mode[^{]*kbd\[data-lent\] \{[^}]*line-through/s);
});

/* ---------- the picture ---------- */

// A keyboard drawn with the letters on its keys, because where a letter
// falls on the keyboard is the whole of what there is to learn. Asked for
// as "maybe we could see a piano and the keys written on it".
test("the keyboard is drawn from the same table the keys obey", () => {
  const body = grab("renderPianoKeys");
  assert.match(body, /pianoWhites\(\)/);
  assert.match(grab("pianoWhites"), /PIANO_KEYS\.keys\(\)/);
  assert.match(body, /piano-white/);
  assert.match(body, /piano-black/);
});

// Two and three, which is the shape that makes a drawn keyboard readable:
// no black key over E or B.
test("the black keys fall where they do on a piano", () => {
  const body = grab("renderPianoKeys");
  assert.match(body, /new Set\(\[0, 1, 3, 4, 5\]\)/);
  // And not hanging off the end, where there is no next key to sit against.
  assert.match(body, /i < whites\.length - 1/);
});

test("every drawn key plays, by the same rules as the letters", () => {
  const body = grab("pianoPlayable");
  assert.match(body, /mousedown/);
  assert.match(body, /mouseup/);
  assert.match(body, /mouseleave/, "a key dragged off stays down for ever");
  assert.match(body, /pianoPress\(id, note\)/);
});

test("the keyboard is on the page and hidden until it is wanted", () => {
  assert.match(html, /<div id="pianokeys" hidden>/);
  assert.match(html, /<div id="pianokeys-board"><\/div>/);
  assert.match(grab("setPiano"), /strip\.hidden = !state\.piano/);
});

test("switching the mode says so everywhere somebody might look", () => {
  const body = grab("setPiano");
  assert.match(body, /document\.body\.classList\.toggle\("piano-mode"/);
  assert.match(body, /markViewTicks\(\)/);
  assert.match(body, /status\(/);
  assert.match(grab("markViewTicks"), /#piano-tick/);
  assert.match(html, /<kbd id="piano-tick">/);
});

test("the menu entry is there, with its French", () => {
  assert.match(html, /data-action="piano-mode"><span>Piano mode<\/span>/);
  assert.ok(fr.includes('"Piano mode":'), "no French");
});

// A read-only score has nothing to write into, and a mode that takes the
// letters to write nothing with is the worst of both.
test("a score that cannot be written to does not take the letters", () => {
  assert.match(grab("setPiano"), /state\.piano = !!on && !state\.readOnly/);
});
