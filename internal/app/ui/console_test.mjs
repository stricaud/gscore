/*
 * console_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A place to write a program that writes music.
 *
 * Four things make it a tool rather than a liability, and all four are
 * tested here because every one of them is the sort of thing that is easy
 * to lose in a later change and impossible to notice:
 *
 *   - the verbs queue, so a script reads as music and not as plumbing
 *   - a script that throws has asked for nothing, so nothing has happened
 *   - a loop that writes for ever stops
 *   - nothing in a file ever runs by itself
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

// The verbs, with the queue they fill.
function verbs(budget) {
  const out = [], said = [];
  const scope = {
    Date, Math, JSON, String, Number, Array,
    typeLengths: konst("typeLengths"),
    consoleBudget: budget ?? konst("consoleBudget"),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  // A score of one staff, which is what a new one is: staff() counts from
  // what is already there.
  return { out, said, v: make(out, said, { staves: [{ name: "Staff", bars: [[]] }] }) };
}

// Calling note() writes nothing: it remembers that a note was asked for,
// and the whole queue is sent in order once the script has finished. That
// is what lets a script read as music rather than as plumbing, with no
// await anywhere to forget.
test("the verbs queue rather than write", () => {
  const { out, v } = verbs();
  v.note("c4", 8);
  v.rest(4);
  v.step(3);
  v.bar();
  assert.deepEqual(out.map((s) => s.do), ["note", "rest", "step", "bar"]);
  assert.equal(out[0].pitch, "c4");
  assert.equal(out[0].dur, "eighth", "the length was not turned into a name");
  assert.equal(out[1].dur, "quarter");
  assert.equal(out[2].interval, 3);
});

test("a length the parser does not have is refused where it is written", () => {
  const { v } = verbs();
  assert.throws(() => v.note("c4", 5), /not a note length/);
});

test("a chord is a list of notes and needs notes in it", () => {
  const { out, v } = verbs();
  v.chord(["c4", "e4", "g4"], 2);
  assert.deepEqual(out[0].pitches, ["c4", "e4", "g4"]);
  assert.equal(out[0].dur, "half");
  assert.throws(() => v.chord([]), /needs notes/);
});

// at() counts bars and staves the way the page does, from one.
test("at counts from one, the way the page is numbered", () => {
  const { out, v } = verbs();
  v.at(1, 1);
  assert.deepEqual([out[0].bar, out[0].staff], [0, 0]);
  v.at(12);
  assert.equal(out[1].bar, 11);
  assert.equal(out[1].staff, undefined, "a bar with no staff moved staff anyway");
});

test("say collects what the script wants to tell you", () => {
  const { said, v } = verbs();
  v.say("row", [0, 1, 2]);
  assert.equal(said[0], "row [0,1,2]");
});

// A loop that writes for ever stops, and says so.
test("a script that writes for ever is stopped", () => {
  const { v } = verbs({ steps: 10, ms: 60000 });
  assert.throws(() => { for (;;) v.note("c4"); }, /still going/);
});

// And one that writes slowly for ever.
test("a script that takes too long is stopped", () => {
  const { v } = verbs({ steps: 1e9, ms: -1 });
  assert.throws(() => v.note("c4"), /running too long/);
});

// The budget cannot catch a loop that calls nothing at all, and the help
// says so rather than pretending otherwise.
test("the help admits what the budget cannot catch", () => {
  assert.match(grab("consoleVerbs") + src.slice(0, 0) +
    src.match(/\/\/ consoleBudget is[\s\S]*?const consoleBudget/)[0],
    /while \(true\) \{\}/,
    "nothing says that a loop calling nothing will still hang the page");
});

// A script that throws has asked for nothing, so nothing has happened.
test("a script that fails halfway writes nothing at all", () => {
  const fn = grab("runConsole");
  const script = fn.indexOf("run(...Object.values(verbs)");
  const begin = fn.indexOf("script/begin");
  assert.ok(script > 0 && begin > script,
    "the script is run after the queue starts being written, so half of it " +
    "reaches the score before the error does");
  assert.match(fn, /catch \(e\)[\s\S]{0,120}return \{ said, error/,
    "an error does not come back without writing anything");
});

test("everything one script writes is one undo", () => {
  const fn = grab("runConsole");
  assert.match(fn, /api\/script\/begin/);
  assert.match(fn, /api\/script\/end/);
  assert.match(fn, /finally/,
    "a script that fails halfway leaves the batch open, and every edit " +
    "after it is swallowed into one undo nobody asked for");
});

// The whole of the promise: a score may arrive from anybody, and opening
// one must never be a way for them to run something on your machine.
test("a script in a file is opened and never run", () => {
  const load = grab("loadScripts");
  assert.ok(!/runConsole|new Function|eval/.test(load),
    "opening a score runs what it carries");
  assert.match(load, /api\/scripts/, "it does not read them at all");
  const setup = grab("setupConsole");
  assert.match(setup, /source\.value = s\.Body/,
    "clicking a saved script does not put it in the box");
  assert.ok(!/runConsole/.test(
    setup.slice(setup.indexOf("console-saved"))),
    "clicking a saved script runs it");
  assert.match(html, /never run|not run/,
    "nothing in the page says that a score's scripts are not run");
});

// It is reached on purpose and from one place.
test("it is reached from the menu", () => {
  assert.match(html, /data-action="console"/, "there is no menu item");
  assert.match(src, /case "console": await openConsole\(\)/,
    "the menu item does nothing");
  assert.match(html, /<dialog id="console"/, "there is no window for it");
});

// The whole vocabulary fits beside the buttons, which is the point of there
// being so few words. It is allowed to grow a word at a time and not a
// paragraph at a time: a console somebody has to study is a console nobody
// opens, and the moment this bound has to be raised twice the language has
// stopped being small.
test("the vocabulary is small enough to read at a glance", () => {
  const help = konst("consoleHelp");
  for (const verb of ["note(", "rest(", "chord(", "step(", "bar()", "at(",
                      "staff(", "key(", "dynamic(", "write(", "say("]) {
    assert.ok(help.includes(verb), `${verb} is not in the help line`);
  }
  assert.ok(help.length < 240, "the help line has become a paragraph");
});

/*
 * Reading what is already written.
 *
 * The other half of a console. A script copies a phrase, transposes it,
 * inverts it, answers it in the bass — and none of that is possible without
 * being able to ask what is there.
 */
function reading(staves) {
  const out = [], said = [];
  const scope = {
    Date, Math, JSON, String, Number, Array,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
    transposeName: new Function("return " + grab("transposeName"))(),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  return make(out, said, { staves });
}

const twoBars = [{
  name: "Piano",
  bars: [
    [{ pitch: "C4", length: 4 }, { pitch: "E4", length: 4 },
     { pitch: "", length: 8, dots: 1, rest: true }],
    [{ pitch: "G4", length: 2, chord: ["B4", "D5"] }],
  ],
}];

test("a bar can be read, counting from one the way the page does", () => {
  const v = reading(twoBars);
  const got = v.notes(1, 1);
  assert.equal(got.length, 3, "the bar did not come back whole");
  assert.deepEqual(got[0], { pitch: "C4", length: 4, chord: undefined });
  assert.equal(got[2].rest, true, "the rest came back as a note");
  assert.equal(got[2].dots, 1, "it lost its dot");
  assert.deepEqual(v.notes(2, 1)[0].chord, ["B4", "D5"], "a chord lost its notes");
});

test("a bar nobody wrote comes back empty rather than throwing", () => {
  const v = reading(twoBars);
  assert.deepEqual(v.notes(99, 1), []);
  assert.deepEqual(v.notes(1, 9), []);
});

// A copy, so that a script may do as it likes with what it is given.
test("what is read is a copy of the score, not the score", () => {
  const v = reading(twoBars);
  v.notes(1, 1)[0].pitch = "changed";
  assert.equal(v.notes(1, 1)[0].pitch, "C4", "a script could edit the score by reading it");
  const chord = v.notes(2, 1)[0].chord;
  chord.push("F5");
  assert.equal(v.notes(2, 1)[0].chord.length, 2, "nor a chord in it");
});

// Steps and not semitones, because the key signature spells what comes out:
// a fifth above F is C and not B sharp, and no amount of counting semitones
// will say which was meant.
test("transpose moves by diatonic steps and keeps the spelling to the key", () => {
  const v = reading(twoBars);
  assert.equal(v.transpose("c4", 4), "g4", "up a fifth");
  assert.equal(v.transpose("f4", 4), "c5", "up a fifth across the octave");
  assert.equal(v.transpose("c4", -7), "c3", "down an octave");
  assert.equal(v.transpose("c4", -1), "b3", "down a step, across the octave");
  assert.equal(v.transpose("f#4", 1), "g4", "an accidental is dropped for the key to spell");
  assert.equal(v.transpose("c", 2), "e", "a note with no octave keeps having none");
});

test("transpose leaves alone what is not a note", () => {
  const v = reading(twoBars);
  assert.equal(v.transpose("hello", 2), "hello");
});

// The score is read once before the script runs, because the verbs are
// synchronous and a synchronous verb cannot go and ask.
test("the score is read once, before the script runs", () => {
  const fn = grab("runConsole");
  const read = fn.indexOf("api/contents");
  const run = fn.indexOf("run(...Object.values(verbs)");
  assert.ok(read > 0 && read < run,
    "the score is read after the script has already run, or not at all");
});

/*
 * A script must have a name, and some formats cannot carry it at all.
 */
test("a script cannot be kept without a name", () => {
  const fn = grab("saveScript");
  assert.match(fn, /Give it a name first/,
    "a script with no name is kept, and nobody can ask for it again");
  assert.ok(fn.indexOf("Give it a name first") < fn.indexOf("api/scripts"),
    "it is sent before the name is checked");
});

// The thing being lost is not the file — the file is fine — it is the work
// that went into it, and finding that out afterwards is finding it out too
// late.
test("exporting to a format that cannot carry the scripts says so first", () => {
  const fn = grab("exportAs");
  assert.match(fn, /keepsScripts\(ext\)/, "it does not check the format");
  assert.match(fn, /state\.scripts\.length/, "nor whether there are any");
  assert.ok(fn.indexOf("confirmThat") < fn.indexOf("api/convert"),
    "it exports first and asks afterwards");
  const keeps = new Function("return " + grab("keepsScripts"))();
  assert.equal(keeps("gscore"), true, "gscore's own format cannot carry them");
  for (const ext of ["musicxml", "mxl", "mid", "abc", "ly", "pdf"]) {
    assert.equal(keeps(ext), false, `${ext} is said to carry scripts`);
  }
});

// And the warning has to say the score itself is untouched: somebody who
// has just been told they are about to lose something needs to know they
// have not lost it yet.
test("the warning says the scripts stay in the score", () => {
  const fn = grab("exportAs");
  assert.match(fn, /stays?\s*[`"]?\s*\+?[\s\S]{0,60}in this score/,
    "it does not say the scripts are kept where they are");
});

// Read with the score, not when the console is opened: a warning about
// losing them is no use if the editor only finds out there are any once
// you have gone looking for them.
test("the scripts are read when the score is", () => {
  assert.match(grab("loadInfo"), /loadScripts\(\)/,
    "a score's scripts are only read when the console is opened, so " +
    "exporting before opening it loses them without a word");
});

/*
 * Making staves, and a worked example that uses them.
 *
 * A script that writes for several instruments has to be able to make the
 * staves to write them on, and it has to know which number each one came
 * out as without stopping to ask — which is why staff() answers before the
 * staff exists. Staves are added at the end, in the order they are asked
 * for, so the answer is arithmetic rather than a guess.
 */
test("staff queues the staff and says which number it will be", () => {
  const { out, v } = verbs();
  assert.equal(v.staff("flute"), 2, "the first new staff of a one-staff score");
  assert.equal(v.staff("cello", { clef: "bass", name: "cello I" }), 3);
  assert.deepEqual(out, [
    { do: "staff", instrument: "flute", name: "", clef: "" },
    { do: "staff", instrument: "cello", name: "cello I", clef: "bass" },
  ]);
});

// A signature nobody can write is caught in the script rather than at the
// seventeenth fetch, where the first sixteen have already happened.
test("key refuses a signature that does not exist", () => {
  const { out, v } = verbs();
  v.key(-7);
  v.key(4);
  assert.deepEqual(out.map((s) => s.fifths), [-7, 4]);
  assert.throws(() => v.key(8), /not a key signature/);
  assert.throws(() => v.key(-9), /not a key signature/);
});

test("dynamic marks the note just written", () => {
  const { out, v } = verbs();
  v.note("c4", 8);
  v.dynamic("sfz");
  assert.deepEqual(out[1], { do: "dynamic", name: "sfz" });
});

/*
 * The worked example.
 *
 * One example that really writes a piece says more about what the console
 * is for than a page about what each verb does — but only while it still
 * runs. It is written against the same verbs as everything else, so a
 * change to any of them can quietly break it, and nobody would find out
 * until somebody opened it and pressed Run.
 *
 * So it is run here, on the dice as they fall, and what comes out is
 * checked as music: every bar it writes has to hold exactly one whole
 * note's worth. A generator that writes 15 sixteenths into a bar of four
 * four is not a generator, it is a bug with a nice comment on it.
 */
const example = () => {
  // Anchored on what comes after it: an example's own text has lines that
  // close an array, and a lazy match stops at the first of them.
  const m = src.match(
    /^const consoleExamples = \[([\s\S]*?)\n\];\n\nfunction drawExamples/m);
  if (!m) throw new Error("app.js has no consoleExamples");
  return new Function("return ([" + m[1] + "])")()[0];
};

async function runExample(answers) {
  const out = [], said = [];
  const scope = {
    Date, Math, JSON, String, Number, Array, Object,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
    transposeName: new Function("return " + grab("transposeName"))(),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  let asked = 0;
  const v = make(out, said, { staves: [{ name: "Staff", bars: [[]] }] },
                 async () => answers[asked++]);
  const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
  const run = new AsyncFunction(...Object.keys(v), "score", "Math", "JSON",
    `"use strict";\n${example().body}\n`);
  await run(...Object.values(v), { bars: 1, staves: 1, staffNames: ["Staff"] },
            Math, JSON);
  return { out, said };
}

// How long a queued step lasts, in whole notes. A line of the typing
// language is read the way the parser reads it, because that is what the
// score will do with it.
function lengthOf(step, lengths) {
  const value = (name) => 1 / Number(
    Object.keys(lengths).find((k) => lengths[k] === name));
  if (step.do === "line") {
    let dur = 0, held = 0;
    for (const word of step.line.trim().split(/\s+/)) {
      if (/^\d+$/.test(word)) { held = 1 / Number(word); continue; }
      if (/^[a-g]/i.test(word)) dur += held;
    }
    return dur;
  }
  if (["note", "rest", "chord", "step", "drum"].includes(step.do)) {
    return step.dur ? value(step.dur) : 0;
  }
  return 0;
}

test("the example writes bars that hold exactly one bar of music", async () => {
  const lengths = konst("typeLengths");
  const { out } = await runExample(
    [["piano", "oboe", "contrabass", "alto sax", "drum kit"], "10"]);
  let bar = 0, staves = 0, sfz = 0;
  const bars = [];
  for (const step of out) {
    if (step.do === "staff") { staves++; continue; }
    if (step.do === "at" || step.do === "bar") {
      if (bar) bars.push(bar);
      bar = 0;
      continue;
    }
    if (step.do === "dynamic" && step.name === "sfz") { sfz++; continue; }
    bar += lengthOf(step, lengths);
  }
  if (bar) bars.push(bar);

  assert.equal(staves, 6, "piano takes two staves, the other four one each");
  assert.ok(bars.length >= 50, `only ${bars.length} bars were written`);
  for (const [i, held] of bars.entries()) {
    assert.ok(Math.abs(held - 1) < 1e-9,
      `bar ${i + 1} holds ${held} of a bar, not one`);
  }
  assert.ok(sfz > 0, "nothing was marked sforzando");
});

// Answering nothing is answering. A script that carries on regardless is a
// script that writes something nobody asked for.
test("the example writes nothing when the question is not answered", async () => {
  for (const answers of [[null], [[]], [["piano"], null]]) {
    const { out } = await runExample(answers);
    assert.equal(out.length, 0, `it wrote anyway after ${JSON.stringify(answers)}`);
  }
});

// Opened, never run — the same rule a script a file carries lives under.
test("an example opens in the box rather than running", () => {
  assert.match(html, /<div id="console-examples">/, "there is nowhere to show them");
  const fn = grab("setupConsole");
  assert.match(fn, /console-examples/, "clicking one does nothing");
  assert.ok(!/consoleExamples[\s\S]{0,200}runConsole/.test(src),
    "an example is run rather than opened");
});

/*
 * How many bars there are.
 *
 * The editor draws a blank bar past the end of the music so there is
 * somewhere to click to carry on writing. It is not a bar of the piece, and
 * counting it as one is how a script that writes sixteen bars ends up with
 * a score where one staff has seventeen: bar() believes the next bar is
 * already there, writes into it without making it, and the bar comes into
 * being on that staff alone while every other staff is left a bar short and
 * without even a rest in it.
 */
test("the bar drawn past the end is not counted as a bar", () => {
  const count = new Function("state",
    grab("measureCount") + "\nreturn measureCount();");
  const bars = (n) => Array.from({ length: n }, (_, i) => ({ measure: i }));

  assert.equal(count({ hits: bars(4) }), 4);
  assert.equal(count({ hits: [...bars(4), { measure: 4, ahead: true }] }), 4,
    "the room to write was counted as a bar of the piece");
  assert.equal(count({ hits: [{ measure: 0, ahead: true }] }), 0,
    "an empty score was said to have a bar in it");
});

// A part for an instrument in another key is written in that key, which
// means every note it is given has to be spelled out — the accidental and,
// where there is none, the natural. A plain letter is left to the key
// signature, and the key signature on a saxophone's staff is not the one
// the script was thinking in.
test("the example spells out every note it writes", async () => {
  const { out } = await runExample([["alto sax", "piano", "french horn"], "6"]);
  const notes = out.filter((s) => s.do === "note");
  assert.ok(notes.length > 50, "nothing was written to look at");
  for (const s of notes) {
    assert.match(s.pitch, /^[a-g](#|x|b|bb|n)-?\d$/,
      `${s.pitch} was left for the key signature to spell`);
  }
  // And a transposing part really is moved: the alto saxophone reads a
  // major sixth above what it sounds, so it has notes the piano has not.
  const pitches = new Set(notes.map((s) => s.pitch));
  assert.ok([...pitches].some((p) => p.includes("x")),
    "nothing came out double sharp, so nothing was transposed far enough");
});

/*
 * The clock, and what it is on.
 *
 * The budget is there to stop a loop that writes for ever. It is not there
 * to stop somebody reading the question they have just been asked — and a
 * script waiting on an answer is the one script in the editor that cannot
 * possibly be in a loop, because it is stopped. Counted together, the
 * example was killed for running too long while its first dialog was still
 * on the screen.
 */
test("the clock stops while a question is on the screen", async () => {
  const out = [], said = [];
  const scope = {
    Date, Math, JSON, String, Number, Array, Object,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: { steps: 4000, ms: 40 },
    transposeName: new Function("return " + grab("transposeName"))(),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  const slow = () => new Promise((go) => setTimeout(() => go("yes"), 120));
  const v = make(out, said, { staves: [] }, slow);

  assert.equal(await v.ask("ready?"), "yes");
  assert.equal(await v.ask("still there?"), "yes");
  // A quarter of a second has gone by on a budget of forty milliseconds,
  // and all of it was theirs.
  v.note("c4", 4);
  assert.equal(out.length, 1, "the script was stopped for their reading time");
});
