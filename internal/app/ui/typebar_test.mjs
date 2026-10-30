/*
 * typebar_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Typing the music in.
 *
 * NoteWriter II had a line like this in 1988 and called it the command
 * window. What made it fast is that your hands never leave the keys: a note
 * is two facts, which pitch and how long, and a mouse can only say one of
 * them at a time.
 *
 * The one thing to learn is that a number on its own is a length and a
 * number inside a note is its octave — "4 c4" is a quarter note on middle
 * C. That is why lengths are numbers at all: the letters are needed for the
 * notes, and e is the note E long before it is an eighth.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const here = (f) => readFileSync(new URL("./" + f, import.meta.url), "utf8");
const src = here("app.js");
const html = here("index.html");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};
const konst = (name) => {
  const m = src.match(new RegExp("^const " + name + " =([\\s\\S]*?);$", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return new Function("return (" + m[1] + ")")();
};

const parse = (() => {
  const scope = {
    typeLengths: konst("typeLengths"),
    typeBudget: konst("typeBudget"),
    typeMarks: konst("typeMarks"),
    typeBars: konst("typeBars"),
  };
  const body = [grab("typeNote"), grab("splitRun"), grab("typeChord"),
                grab("parseTyping")].join("\n") +
    "\nreturn parseTyping;";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
})();

test("a length is a number, and it sticks", () => {
  const { steps } = parse("4 c d e");
  assert.equal(steps.length, 3, "three notes were not read");
  for (const s of steps) {
    assert.equal(s.dur, "quarter", "the length did not carry to the next note");
  }
  assert.deepEqual(steps.map((s) => s.pitch), ["c", "d", "e"]);
});

test("a length changes when another is given", () => {
  const { steps } = parse("4 c 8 d e 2 f");
  assert.deepEqual(steps.map((s) => s.dur),
    ["quarter", "eighth", "eighth", "half"]);
});

test("dots go on the length", () => {
  const { steps } = parse("4. c 8 d");
  assert.equal(steps[0].dots, 1, "the dot was not read");
  assert.equal(steps[1].dots, 0, "the dot carried to a length that has none");
});

// Six notes is one gesture in the hand and ought to be one in the typing.
// Spaces are then only needed where a word begins.
test("a run of notes needs no spaces in it", () => {
  const { steps } = parse("8abcdef");
  assert.equal(steps.length, 6, "the run was not broken into its notes");
  assert.deepEqual(steps.map((s) => s.pitch), ["a", "b", "c", "d", "e", "f"]);
  for (const st of steps) assert.equal(st.dur, "eighth");
});

test("a word is still a word, and not the notes its letters spell", () => {
  const { steps } = parse("tie 8abcdef");
  assert.equal(steps[0].kind, "mark", "tie came out as T, I and the note E");
  assert.equal(steps[0].mark, "tie");
  assert.equal(steps.length, 7);
});

test("an octave can be glued in too", () => {
  const { steps } = parse("8a4b5c4");
  assert.deepEqual(steps.map((s) => [s.pitch, s.dur]), [
    ["a4", "eighth"], ["b5", "eighth"], ["c4", "eighth"],
  ]);
});

// The digits after a note are its octave, so a length cannot change in the
// middle of a run: "8a4b16cd" is A4 and then a B in octave sixteen. Rather
// than hand the server a note nobody can play, it says what to do.
test("a length inside a run is told it needs a space", () => {
  const got = parse("8a4b16cd");
  assert.ok(got.error, "a B in octave sixteen was accepted");
  assert.match(got.error, /needs a space in front of it/,
    "it does not say what to do about it");
});

test("a length with a space in front of it changes mid-line", () => {
  const { steps } = parse("8a4b 16cd");
  assert.deepEqual(steps.map((s) => [s.pitch, s.dur]), [
    ["a4", "eighth"], ["b", "eighth"],
    ["c", "sixteenth"], ["d", "sixteenth"],
  ]);
});

test("rests and barlines can be glued in too", () => {
  const { steps } = parse("4cr|d");
  assert.deepEqual(steps.map((s) => s.kind), ["note", "rest", "bar", "note"]);
});

// In a run the b is the note B — it has to be, or the commonest thing
// anybody types comes out as A flat and five notes. So a flat inside a run
// is written _, the way ABC notation has spelled it since the 1990s.
test("inside a run b is the note and _ is the flat", () => {
  assert.deepEqual(parse("8ab").steps.map((s) => s.pitch), ["a", "b"]);
  assert.deepEqual(parse("8a_b_").steps.map((s) => s.pitch), ["ab", "bb"]);
});

// A note standing on its own as a whole token still takes b, because "bb4"
// is how everybody writes B flat and a token with one note in it cannot be
// misread.
test("a note on its own still takes b for the flat", () => {
  assert.equal(parse("bb4").steps[0].pitch, "bb4");
  assert.equal(parse("8 ab").steps[0].pitch, "ab");
});

test("a run of letters that is not music is still a command", () => {
  const { steps } = parse("transpose");
  assert.equal(steps[0].kind, "command", "a word was broken up into notes");
});

// The one thing to learn.
test("a number inside a note is its octave, not its length", () => {
  const { steps } = parse("4 c4 f#3 bb5");
  assert.deepEqual(steps.map((s) => s.pitch), ["c4", "f#3", "bb5"]);
  for (const s of steps) assert.equal(s.dur, "quarter");
});

// "b" on its own is the note B; "bb" is B flat. The letter is the first
// character and everything after it is an accidental.
test("b is the note and bb is the flat", () => {
  assert.equal(parse("b").steps[0].pitch, "b");
  assert.equal(parse("bb").steps[0].pitch, "bb");
  assert.equal(parse("bb4").steps[0].pitch, "bb4");
});

test("a natural is a thing you can write", () => {
  assert.equal(parse("fn4").steps[0].pitch, "fn4");
});

// Notes sounded together go in brackets, as ABC notation has written them
// since the 1990s and as everybody who copied it writes them — the same
// reason the barlines and the flats are spelled the way they are here.
test("a chord is its notes in brackets, and one thing", () => {
  const { steps, error } = parse("4 [ceg]");
  assert.ok(!error, String(error));
  assert.equal(steps.length, 1, "a chord was read as its separate notes");
  assert.equal(steps[0].kind, "chord");
  assert.deepEqual(steps[0].pitches, ["c", "e", "g"]);
  assert.equal(steps[0].dur, "quarter");
});

// A chord is one sound and a sound has one length, so the length goes where
// a note's length goes — in front, and sticky.
test("a chord takes the length in force", () => {
  const { steps } = parse("2 [ceg] [dfa]");
  assert.deepEqual(steps.map((s) => s.dur), ["half", "half"]);
  assert.deepEqual(steps[1].pitches, ["d", "f", "a"]);
});

test("a chord can be glued into a run", () => {
  const { steps, error } = parse("8[ceg]d");
  assert.ok(!error, String(error));
  assert.deepEqual(steps.map((s) => s.kind), ["chord", "note"]);
  assert.deepEqual(steps.map((s) => s.dur), ["eighth", "eighth"]);
});

// Inside the brackets the notes are read the way a run is read, because
// that is what they are: b is the note B and a flat is written _.
test("inside a chord b is the note and _ is the flat", () => {
  assert.deepEqual(parse("[e_gb]").steps[0].pitches, ["eb", "g", "b"]);
});

test("a chord wider than an octave is written out", () => {
  assert.deepEqual(parse("[c4g4e5]").steps[0].pitches, ["c4", "g4", "e5"]);
});

test("a length inside the brackets is told where it goes", () => {
  const got = parse("[8ce]");
  assert.ok(got.error, "a length inside a chord was accepted");
  assert.match(got.error, /length goes in front/, "it does not say what to do");
});

// A line is split at its spaces before any of it is read, so a chord with
// spaces in it arrives in pieces. Saying so beats saying there is no
// command called "[c".
test("a chord with spaces in it says so", () => {
  const got = parse("4 [c e g]");
  assert.ok(got.error, "a chord in pieces was accepted");
  assert.match(got.error, /no spaces inside the brackets/);
});

test("a chord with nothing in it is refused", () => {
  assert.match(parse("4 []").error, /needs some notes in it/);
});

test("only notes can be in a chord", () => {
  assert.match(parse("[cr]").error, /cannot be in a chord/);
});

// The editor builds a chord by adding noteheads to a note already written,
// which is what the mouse does by clicking the same column twice.
test("a chord is written as its bottom note and then the rest", () => {
  const one = grab("typeOne");
  assert.match(one, /api\/insert/, "the bottom note is never written");
  assert.match(one, /typeNoteheads\(/, "the other noteheads are never added");
  const heads = grab("typeNoteheads");
  assert.match(heads, /api\/addpitch/, "it does not add noteheads");
  // Each notehead stands above the one before it: the nearest A to a C is
  // the one below it, and a chord is read from the bottom up.
  assert.match(heads, /above: "1"/, "the chord is not stacked upwards");
});

test("the hint mentions chords", () => {
  assert.match(konst("typeHint"), /\[ceg\]/, "the hint does not mention them");
});

// Signed, always: a bare number is a length, and a melody said out loud is
// "up a third" rather than "three".
test("an interval is signed", () => {
  const { steps } = parse("8 c4 +2 -1 +7");
  assert.deepEqual(steps.slice(1).map((s) => s.interval), [2, -1, 7]);
  assert.equal(steps[1].pitch, undefined, "an interval was read as a pitch");
});

// The minus sign a keyboard with a numeric pad types is not a hyphen.
test("a typographic minus counts as a minus", () => {
  assert.equal(parse("−3").steps[0].interval, -3);
});

test("r is a rest and | is the next bar", () => {
  const { steps } = parse("4 c r | d");
  assert.deepEqual(steps.map((s) => s.kind), ["note", "rest", "bar", "note"]);
});

// A mark is a thing a phrase has, not a thing you do to one note, and a
// typed phrase is where you say so. "tie 8 a b d e f" ties the whole run.
test("a mark turns on for everything typed after it", () => {
  const { steps } = parse("tie 8 a b d");
  assert.equal(steps[0].kind, "mark", "tie was taken as a one-off command");
  assert.equal(steps[0].mark, "tie");
  assert.deepEqual(steps.slice(1).map((s) => s.kind), ["note", "note", "note"]);
});

// The same word again turns it off, which is how NoteWriter's command line
// behaved and the only rule simple enough not to need remembering.
test("the same word again turns the mark off", () => {
  const { steps } = parse("tie 8 a b tie c");
  assert.deepEqual(steps.map((s) => s.kind),
    ["mark", "note", "note", "mark", "note"]);
});

test("the articulations are marks too, long or short", () => {
  for (const word of ["staccato", "stacc", "tenuto", "ten", "accent",
                      "marcato", "fermata", "slur"]) {
    assert.equal(parse(word).steps[0].kind, "mark", `${word} is not a mark`);
  }
});

// Anything that is neither music nor a mark is a command, which is the
// other half of what NoteWriter's command line was for.
test("what is neither music nor a mark is taken as a command", () => {
  const { steps } = parse("4 c transpose");
  assert.equal(steps[1].kind, "command");
  assert.equal(steps[1].name, "transpose");
});

// A tie joins a note to the one after it and a slur goes from one note to
// another, so neither can be said until both ends are on the page.
test("the marks go on after the notes, not as each is written", () => {
  const fn = grab("runTyping");
  assert.match(fn, /applyTypedMarks\(written\)/,
    "the marks are put on as the notes are written, and a tie then has " +
    "nothing to tie to");
  const marks = grab("applyTypedMarks");
  assert.match(marks, /api\/tie/, "a tie is never written");
  assert.match(marks, /api\/slur/, "nor a slur");
  assert.match(marks, /api\/articulation/, "nor an articulation");
  assert.match(marks, /run\.length < 2/,
    "a mark that needs two notes is asked for with one");
});

// A line with a mistake in it writes nothing: half a phrase, with the
// editor stopped in the middle of it, is worse than being told what the
// mistake was.
test("a length nobody has stops the line before anything is written", () => {
  const got = parse("5 c d");
  assert.ok(got.error, "a length of 5 was accepted");
  assert.match(got.error, /1, 2, 4, 8, 16 or 32/, "it does not say what to try");
  assert.equal(got.steps, undefined, "it offered to write something anyway");
});

test("an interval further than an interval goes is refused", () => {
  assert.ok(parse("+40").error);
});

// A line is a phrase somebody typed, not a program: that is what the
// console is for.
test("a line longer than the budget is refused", () => {
  const long = new Array(konst("typeBudget") + 5).fill("c").join(" ");
  const got = parse("4 " + long);
  assert.ok(got.error, "a line of hundreds of notes was accepted");
  assert.match(got.error, /at once/);
});

test("an empty line asks for nothing", () => {
  assert.deepEqual(parse("   ").steps, []);
});

// It is docked under the music, not standing at the cursor.
//
// It floated at the cursor first and that was wrong twice over: it sat
// squarely on the bar being written, which is the one thing you have to see
// while writing it, and it only appeared when the cursor's bar happened to
// be among the regions drawn — so closing it and opening it again in a bar
// past the end of the music showed nothing at all.
function shows(state) {
  const made = new Map();
  const scope = {
    state,
    typeHint: konst("typeHint"),
    barNumberOf: (n) => n + 1,
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { textContent: "", value: "", hidden: false });
      return made.get(sel);
    },
  };
  new Function(...Object.keys(scope),
    grab("placeTypeBar") + "\nreturn placeTypeBar;")(...Object.values(scope))();
  return made;
}

test("the line is shown whenever the mode is on, wherever the cursor is", () => {
  // The case that was broken: a cursor in a bar nothing is drawn for.
  const made = shows({ typing: true, selection: { part: 0, measure: 400 }, hits: [], info: {} });
  assert.equal(made.get("#typebar").hidden, false,
    "it stayed hidden because the cursor's bar was not among the regions drawn");
});

test("and hidden whenever it is off", () => {
  assert.equal(shows({ typing: false }).get("#typebar").hidden, true);
});

// What is left of following the cursor is the part worth having: it says
// where the next thing typed will go, in words rather than in pixels.
test("it says which staff and bar it is writing into", () => {
  const made = shows({
    typing: true, selection: { part: 1, measure: 11 },
    info: { parts: [{ name: "Piano" }, { name: "Left hand" }] },
  });
  assert.equal(made.get("#type-where").textContent, "Left hand · bar 12");
  assert.match(grab("markSelection"), /placeTypeBar\(\)/,
    "it does not follow the cursor as the cursor walks");
});

test("with nowhere to write it says so rather than naming a staff", () => {
  const made = shows({ typing: true, selection: null, info: {} });
  assert.match(made.get("#type-where").textContent, /Click a bar first/);
});

// It must not stand on the music. Docked, not positioned.
test("it is docked under the music and not floated over it", () => {
  const css = here("style.css");
  const rule = css.match(/#typebar \{[\s\S]*?\}/)[0];
  assert.ok(!/position:\s*absolute/.test(rule),
    "it floats over the score it is writing");
  assert.match(rule, /border-top/, "it is not docked under the music");
  assert.ok(!/placeTypeBar/.test(grab("drawScore") || ""), "parsed");
});

// A line is one thought, so taking it back takes back the phrase and not
// the last note of it.
test("a typed line is one undo", () => {
  const fn = grab("runTyping");
  assert.match(fn, /api\/script\/begin/, "the line is not bracketed");
  assert.match(fn, /api\/script\/end/, "nor closed");
  assert.match(fn, /finally/,
    "a line that fails halfway leaves the batch open, and every edit after " +
    "it is swallowed into one undo nobody asked for");
});

// Typing a note must not also press the key that note is bound to.
test("keystrokes in the line do not reach the score", () => {
  const fn = grab("setupTypeBar");
  assert.match(fn, /stopPropagation/, "keys typed into the box reach the editor");
  assert.match(fn, /Escape/, "there is no way out of it");
  assert.match(fn, /Enter/, "there is no way to run it");
});

test("it is a mode, reached from the menu and from a key", () => {
  assert.match(html, /data-action="type-music"/, "there is no menu item");
  assert.match(src, /case "type-music": toggleTypeBar\(\)/,
    "the menu item does nothing");
  assert.match(src, /id: "type-music".*\n.*\n.*toggleTypeBar/,
    "it has no key of its own");
  assert.match(html, /id="typebar" hidden/,
    "the line is on the screen before anybody asks for it");
});

/*
 * The reference for it.
 *
 * Built from the same tables the parser reads, rather than written out:
 * a reference kept by hand is wrong by the second change, and a wrong
 * reference is worse than none because it is believed.
 */
function helpPage(lengths, marks) {
  const made = new Map();
  const scope = {
    Object, String, Number, Map,
    typeLengths: lengths ?? konst("typeLengths"),
    typeMarks: marks ?? konst("typeMarks"),
    escapeHTML: (s) => String(s),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "" });
      return made.get(sel);
    },
  };
  const body = [grab("syntaxRows"), grab("drawTypingHelp")].join("\n") +
    "\nreturn drawTypingHelp;";
  new Function(...Object.keys(scope), body)(...Object.values(scope))();
  return made.get("#typing-help-body").innerHTML;
}

test("the reference lists every length the parser knows", () => {
  const page = helpPage();
  for (const n of Object.keys(konst("typeLengths"))) {
    assert.match(page, new RegExp(`<dt>${n}</dt>`),
      `${n} is a length the parser takes and the reference does not list`);
  }
});

test("and every mark, by every word that reaches it", () => {
  const page = helpPage();
  for (const word of Object.keys(konst("typeMarks"))) {
    assert.ok(page.includes(word),
      `${word} turns a mark on and the reference does not mention it`);
  }
});

// The point of building it: a length added to the program appears here
// without anybody remembering to add it.
test("a length added to the parser appears without being written out", () => {
  const page = helpPage({ 4: "quarter", 128: "128th" });
  assert.match(page, /<dt>128<\/dt>/, "a new length did not reach the reference");
  assert.match(page, /128th note/, "nor its name");
});

// The two places somebody would otherwise be surprised are stated in words.
test("it states the two rules that need stating", () => {
  const page = helpPage();
  assert.match(page, /digits straight after a note are its octave/,
    "it does not say why a length inside a run needs a space");
  assert.match(page, /8abcdef<\/em> would start on an A flat/,
    "it does not say why a flat inside a run is written _");
});

test("it says how a chord is written and which way it is read", () => {
  const page = helpPage();
  assert.match(page, /\[ceg\]/, "it does not say how a chord is written");
  assert.match(page, /is a sixth/, "it does not say a chord reads upwards");
});

test("it says a line is one undo and writes nothing if it is wrong", () => {
  const page = helpPage();
  assert.match(page, /one press of undo/);
  assert.match(page, /writes nothing/);
});

test("it is reached from the Help menu", () => {
  assert.match(html, /data-action="typing-help"/, "there is no menu item");
  assert.match(src, /case "typing-help": openTypingHelp\(\)/,
    "the menu item does nothing");
  assert.match(html, /<dialog id="typing-help"/, "there is no window for it");
});

// The barlines, written the way they are written everywhere music is typed.
//
// A plain | goes on to the next bar and leaves its barline alone. The others
// say what the bar being left ends with — which is the one thing a typed
// line could not say, so a repeat had to be clicked for afterwards.
test("a bar can say what barline it ends with", () => {
  const want = {
    "|": "",
    "||": "double",
    "|:": "repeat-start",
    ":|": "repeat-end",
    "|]": "final",
  };
  for (const [written, style] of Object.entries(want)) {
    const { steps, error } = parse(`4 c ${written} d`);
    assert.ok(!error, `${written}: ${error}`);
    assert.equal(steps.length, 3, `${written} did not read as one thing`);
    assert.equal(steps[1].kind, "bar", written);
    assert.equal(steps[1].barline, style, written);
  }
});

// Two characters, read as two characters. A close repeat read one at a time
// is a colon nobody asked for followed by a new bar.
test("a barline glued into a run is still one thing", () => {
  const { steps, error } = parse("8ab:|cd");
  assert.ok(!error, String(error));
  assert.deepEqual(steps.map((s) => s.kind),
                   ["note", "note", "bar", "note", "note"]);
  assert.equal(steps[2].barline, "repeat-end");

  const open = parse("8ab|:cd");
  assert.equal(open.steps[2].barline, "repeat-start");
  const double = parse("8ab||cd");
  assert.equal(double.steps[2].barline, "double");
});

// A colon is not music on its own, so a word holding one is a command — and
// saying so is what keeps "D.S." and the rest reachable.
test("a lone colon is not a barline", () => {
  const { steps } = parse("8 a :");
  assert.equal(steps[steps.length - 1].kind, "command");
});
