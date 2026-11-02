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

// A plugin, read from the folder the samples are kept in and carried into
// the binary from. These are scripts somebody will run, so they are run
// here: a verb renamed under one of them is a sample that fails on the
// first click, in a folder written to every new installation.
const plugin = (f) =>
  readFileSync(new URL("../plugins/" + f, import.meta.url), "utf8");

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
// A table whose entries hold whole statements — the key bindings carry what
// each key does — cannot be found by looking for the first line that ends in
// a semicolon, because the first one is inside an entry. It ends at the
// closing bracket in the first column, which nothing inside it can be.
const table = (name) => {
  const m = src.match(new RegExp("^const " + name + " =([\\s\\S]*?\\n\\]);$", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return new Function("return (" + m[1] + ")")();
};

// The verbs, with the queue they fill.
function verbs(budget, read) {
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
  return {
    out, said,
    v: make(out, said, read || { staves: [{ name: "Staff", bars: [[]] }] }),
  };
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
  // And the place inside the bar, which is what putting the cursor back
  // where it was needs. Without one it is the start of the bar.
  assert.equal(out[1].place, undefined);
  v.at(12, 2, 3);
  assert.deepEqual([out[2].bar, out[2].staff, out[2].place], [11, 1, 3]);
});

/*
 * What the person picked up.
 *
 * "Do this to what I have selected" is the commonest thing a script is
 * for, and until this was here a script could only be told about the whole
 * piece. It is read once before the script runs, like the music, because
 * the cursor moves as the queue is written — and a script that put the
 * cursor back by reading where it is now would put it back where it had
 * just left it.
 */
function pickedBy(state) {
  return new Function("state",
    grab("pickedSelection") + "\nreturn pickedSelection();")(state);
}

test("a bare cursor is a place with nothing picked up", () => {
  const got = pickedBy({ selection: { part: 1, measure: 40, at: 3 }, multi: [] });
  assert.deepEqual(got,
    { staff: 2, bar: 41, at: 3, count: 0, from: null, to: null },
    "the editor's own counting from nought reached the script");
});

test("nothing picked up is nothing, not a selection of the whole score", () => {
  assert.equal(pickedBy({ selection: null, multi: [] }), null);
  const v = reading([{ name: "Piano", bars: [[]] }]);
  assert.equal(v.selection(), null,
    "a script is told something is selected when nothing is");
});

test("a stretch says where it begins and ends, in reading order", () => {
  const got = pickedBy({
    selection: { part: 0, measure: 5, at: 2 },
    // Dragged backwards, which is how half of them are made.
    multi: [
      { part: 0, measure: 7, index: 1 },
      { part: 0, measure: 5, index: 1 },
      { part: 0, measure: 6, index: 0 },
    ],
  });
  assert.equal(got.count, 3);
  assert.deepEqual(got.from, { staff: 1, bar: 6, index: 1 });
  assert.deepEqual(got.to, { staff: 1, bar: 8, index: 1 });
});

// A copy, so a script cannot reach into the editor through what it is told.
test("what a script is told about the selection is a copy", () => {
  const scope = {
    Date, Math, JSON, String, Number, Array,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  const picked = { staff: 1, bar: 4, at: 2, count: 1,
                   from: { staff: 1, bar: 4, index: 1 },
                   to: { staff: 1, bar: 4, index: 1 } };
  const v = make([], [], { staves: [] }, null, { picked });

  v.selection().bar = 99;
  v.selection().from.bar = 99;
  assert.equal(v.selection().bar, 4, "a script could move the cursor by reading it");
  assert.equal(v.selection().from.bar, 4, "nor the end of the stretch");
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
  assert.match(fn, /catch \(e\)[\s\S]{0,160}return \{ said, drawings, error/,
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
// opens.
//
// Two lines, and the second one is not slack to be filled: a third line
// would mean the language had stopped being small, and the answer then is a
// help that is not the bar at the bottom of the window.
test("the vocabulary is small enough to read at a glance", () => {
  const help = konst("consoleHelp");
  assert.ok(Array.isArray(help) && help.length === 3,
    "the help is no longer the three lines it is drawn as");
  const all = help.join(" ");
  for (const verb of ["note(", "rest(", "chord(", "step(", "bar()", "at(",
                      "staff(", "key(", "dynamic(", "write(", "run(",
                      "notes(", "measure(", "staves(", "selection(",
                      "chordNotes(", "progress(", "draw(", "say("]) {
    assert.ok(all.includes(verb), `${verb} is not in the help`);
  }
  for (const line of help) {
    assert.ok(line.length < 240, "a help line has become a paragraph");
  }
  // Every word in it is a word the console answers to. A help that offers a
  // verb nobody wrote is worse than one that is missing a word: it is read,
  // tried, and fails.
  const scope = {
    Date, Math, JSON, String, Number, Array,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  const v = make([], [], { staves: [] });
  for (const m of all.matchAll(/([a-zA-Z]+)\(/g)) {
    assert.equal(typeof v[m[1]], "function", `${m[1]}() is offered and is not there`);
  }
});

/*
 * Reading what is already written.
 *
 * The other half of a console. A script copies a phrase, transposes it,
 * inverts it, answers it in the bass — and none of that is possible without
 * being able to ask what is there.
 */
function readVerbs(staves, picked) {
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
  return { out, said, v: make(out, said, { staves }, null, { picked }) };
}

const reading = (staves) => readVerbs(staves).v;

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

/*
 * Dividing a Latin word into the syllables it is sung on.
 *
 * Setting a text under music means knowing where the words break, and a
 * Mass is four hundred words to hyphenate before a note of it can be
 * typed. The rules are the ones printed at the front of the Liber Usualis,
 * which is what choral editions follow, so every case below is a word
 * somebody can look up and check — which is the point of writing them out
 * rather than testing the rules against themselves.
 */
const syllables = (() => {
  const scope = {
    latinPlain: konst("latinPlain"),
    latinVowels: konst("latinVowels"),
    latinMutes: konst("latinMutes"),
    latinAfterS: konst("latinAfterS"),
  };
  return new Function(...Object.keys(scope),
    grab("latinSyllables") + "\n" + grab("latinCarry") +
    "\nreturn latinSyllables;")(...Object.values(scope));
})();

test("a Latin word is divided where it is sung", () => {
  for (const [word, want] of [
    // One consonant between two vowels begins the next syllable.
    ["Dominus", "Do-mi-nus"],
    ["miserere", "mi-se-re-re"],
    ["Kyrie", "Ky-ri-e"],
    ["Gloria", "Glo-ri-a"],
    // Two are divided, the first closing the syllable.
    ["omnis", "om-nis"],
    ["ecce", "ec-ce"],
    ["terra", "ter-ra"],
    // Unless the first is s, or a mute before l or r.
    ["hostis", "ho-stis"],
    ["patris", "pa-tris"],
    ["Christus", "Chri-stus"],
    // Three or more: the first closes, the rest begin the next.
    ["sanctus", "san-ctus"],
    ["monstro", "mon-stro"],
    ["exemplum", "ex-em-plum"],
    // The pairs that are one consonant.
    ["Agnus", "A-gnus"],
    ["machina", "ma-chi-na"],
    ["aqua", "a-qua"],
    ["qui", "qui"],
    ["lingua", "lin-gua"],
    ["sanguis", "san-guis"],
    // x closes the syllable in front of it.
    ["dixit", "dix-it"],
    ["excelsis", "ex-cel-sis"],
    ["pax", "pax"],
    // ae, oe and au are sung on one note.
    ["caelum", "cae-lum"],
    ["poena", "poe-na"],
    ["laudate", "lau-da-te"],
    ["coeli", "coe-li"],
    // And eu is not: Latin's own words sing it as two.
    ["Deus", "De-us"],
    ["meus", "me-us"],
    // i before a vowel is the consonant J where it begins a word or
    // stands between two vowels, and a vowel after a consonant.
    ["Iesus", "Ie-sus"],
    ["eius", "e-ius"],
    ["alleluia", "al-le-lu-ia"],
    ["eleison", "e-le-i-son"],
    // The Hebrew names, where the a and the e are sung separately.
    ["Israel", "Is-ra-el"],
    ["Michael", "Mi-cha-el"],
    ["Raphael", "Ra-pha-el"],
  ]) {
    assert.equal((syllables(word) || []).join("-"), want, word);
  }
});

// The syllables come back spelled as they were given, so that joining them
// gives the word back — accents and capitals and all. A chant book marks
// the stress with an accent, and a syllable that lost it is a syllable
// nobody can set under the note it belongs to.
test("the word comes back as it was given", () => {
  for (const word of ["Dómini", "voluntátis", "hóminibus", "regína",
                      "DOMINUS", "Sabaoth"]) {
    const got = syllables(word);
    assert.ok(got, word);
    assert.equal(got.join(""), word,
      `${word} came back as ${got.join("-")}, which is not the word`);
  }
});

// Nothing is guessed, and nothing that is not a word is divided.
test("a word that cannot be Latin comes back as nothing", () => {
  for (const word of ["willow", "sword", "", "  ", "a1", "two words",
                      "x!", "123", "mmm"]) {
    assert.equal(syllables(word), null, JSON.stringify(word));
  }
});

// English is the reason this is named for Latin: these are the words it
// would be asked about and the answers would be wrong. "heaven" is heav-en
// and "beaver" is bea-ver, and nothing in the letters says which — so the
// name is the contract, because the letters cannot be.
test("the name is the contract, because English cannot be read by rule", () => {
  // It answers, because it cannot tell: "mea" and "idea" are the same
  // shape. What is checked is that the help and the verb are named for
  // Latin, so that nobody reads an answer as being about English.
  assert.ok(syllables("heaven"), "a word made of Latin letters was refused");
  const help = konst("consoleHelp").join(" ");
  assert.match(help, /latinSyllables\("alleluia"\)/,
    "the help does not say which language it divides, or offers a Latin " +
    "word as the example of what to hand it");
});

/*
 * What notes a chord symbol is made of.
 *
 * The spelling is the whole of it. Four semitones above C is E or F flat,
 * and what makes it E is being the third — so each tone is spelled on the
 * letter its degree lands on and given whatever accidental makes it sound
 * right. Done the other way about, by picking the nearest black note and
 * calling it a sharp, Cmaj7 comes out with a B and E flat major with a D
 * sharp, and a page of those is a page nobody can read.
 *
 * What a chord is comes from the server, which owns it; what letter a
 * degree is spelled on is here. The table below is a few rows of the
 * server's own, written out so that a failure here is a failure in the
 * letters and not in the table.
 */
const QUALITIES = {
  "": [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 4 },
       { degree: 4, semitones: 7 }],
  m: [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 3 },
      { degree: 4, semitones: 7 }],
  maj7: [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 4 },
         { degree: 4, semitones: 7 }, { degree: 6, semitones: 11 }],
  M7: [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 4 },
       { degree: 4, semitones: 7 }, { degree: 6, semitones: 11 }],
  m7: [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 3 },
       { degree: 4, semitones: 7 }, { degree: 6, semitones: 10 }],
  7: [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 4 },
      { degree: 4, semitones: 7 }, { degree: 6, semitones: 10 }],
  dim7: [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 3 },
         { degree: 4, semitones: 6 }, { degree: 6, semitones: 9 }],
  9: [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 4 },
      { degree: 4, semitones: 7 }, { degree: 6, semitones: 10 },
      { degree: 8, semitones: 14 }],
};

const spell = (() => {
  const scope = { letterSemis: konst("letterSemis") };
  return new Function(...Object.keys(scope),
    grab("chordNotesOf") + "\nreturn chordNotesOf;")(...Object.values(scope));
})();

test("a chord is spelled on the letters its degrees land on", () => {
  const notes = (sym, oct) => spell(sym, oct, QUALITIES);

  assert.deepEqual(notes("C"), ["cn4", "en4", "gn4"]);
  assert.deepEqual(notes("Cmaj7"), ["cn4", "en4", "gn4", "bn4"],
    "the seventh of C major came out as something other than B");
  // The flat keys are where a nearest-black-note reading goes wrong: E flat
  // minor seventh is spelled with a G flat and a D flat, not an F sharp and
  // a C sharp.
  assert.deepEqual(notes("Ebm7"), ["eb4", "gb4", "bb4", "db5"]);
  assert.deepEqual(notes("F#7"), ["f#4", "a#4", "c#5", "en5"]);
  // And the sharp keys the other way: B sharp major really is spelled with
  // two double sharps, which is why anybody writes C instead.
  assert.deepEqual(notes("B#"), ["b#4", "dx5", "fx5"]);
});

// The one doubly diminished interval anybody writes. C diminished seventh is
// C, E flat, G flat, B double flat — the seventh is a diminished seventh and
// not a minor one, and a program that spelled it A would be writing a
// different chord.
test("the diminished seventh keeps its double flat", () => {
  assert.deepEqual(spell("Cdim7", 4, QUALITIES),
    ["cn4", "eb4", "gb4", "bbb4"]);
});

// Notes above the root climb out of its octave as they must, so the ninth of
// a chord is in the octave above whatever the root is in.
test("a chord climbs out of its octave rather than folding back into it", () => {
  assert.deepEqual(spell("C9", 3, QUALITIES),
    ["cn3", "en3", "gn3", "bb3", "dn4"]);
  assert.deepEqual(spell("G7", 2, QUALITIES),
    ["gn2", "bn2", "dn3", "fn3"], "the fifth did not cross into the next octave");
});

// Case matters in exactly one place: M7 is a major seventh and m7 a minor
// one. The spelling is looked up as it was typed before anything is lowered.
test("M7 and m7 are not the same chord", () => {
  assert.deepEqual(spell("CM7", 4, QUALITIES), ["cn4", "en4", "gn4", "bn4"]);
  assert.deepEqual(spell("Cm7", 4, QUALITIES), ["cn4", "eb4", "gn4", "bb4"]);
});

// A slash bass says where the chord sits, not what it is made of.
test("a slash bass is not a note of the chord", () => {
  assert.deepEqual(spell("G7/B", 4, QUALITIES), spell("G7", 4, QUALITIES));
});

// Nothing is guessed. A quality gscore does not know gives back nothing, so
// that the script can say so — a chord quietly filled in with the triad
// somebody probably meant is a chord that will be read, believed and played.
test("a chord nobody can read comes back as nothing", () => {
  for (const sym of ["Hmaj7", "Czonk", "", "7", "Cm7b5b9"]) {
    assert.equal(spell(sym, 4, QUALITIES), null, `${sym} was given a reading`);
  }
});

// Some roots and some chords cannot be written down together. The sharpened
// ninth of B sharp wants a triple sharp, and notation stops at two: B sharp
// is a root nobody writes for exactly this reason. Nothing is nearly right
// in that case — it comes back as nothing, so the script says so instead of
// writing a note a semitone from the one it meant.
test("a chord that cannot be spelled comes back as nothing", () => {
  const sharpNine = {
    "7#9": [{ degree: 0, semitones: 0 }, { degree: 2, semitones: 4 },
            { degree: 4, semitones: 7 }, { degree: 6, semitones: 10 },
            { degree: 8, semitones: 15 }],
  };
  assert.deepEqual(spell("C7#9", 4, sharpNine),
    ["cn4", "en4", "gn4", "bb4", "d#5"]);
  assert.equal(spell("B#7#9", 4, sharpNine), null,
    "a note was written two semitones from where the chord says it sounds");
});

test("chordNotes needs the table and says so when it has none", () => {
  const scope = {
    Date, Math, JSON, String, Number, Array,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
    letterSemis: konst("letterSemis"),
    chordNotesOf: spell,
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));

  const with_ = make([], [], { staves: [] }, null, { chords: QUALITIES });
  assert.deepEqual(with_.chordNotes("Am7"), ["an4", "cn5", "en5", "gn5"]);

  const without = make([], [], { staves: [] }, null, {});
  assert.throws(() => without.chordNotes("Am7"), /could not be read/);
});

/*
 * How far a long script has got.
 *
 * A script runs on the same thread that draws the window, so a loop over
 * two and a half thousand bars holds the screen exactly as it was until it
 * finishes. A progress bar written without handing a frame back is a
 * progress bar nobody ever sees move, which is why the await is part of
 * the verb and not an optional nicety.
 */
function withProgress(budget) {
  const shown = [];
  const frames = [];
  const scope = {
    Date, Math, JSON, String, Number, Array, Promise,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: budget ?? konst("consoleBudget"),
    showConsoleProgress: (done, total) => shown.push([done, total]),
    // Stands in for the browser handing the frame back, and takes a
    // measurable moment doing it.
    requestAnimationFrame: (fn) => frames.push(fn),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  const out = [], said = [];
  return { shown, frames, out, v: make(out, said, { staves: [] }) };
}

test("progress draws and hands the frame back", async () => {
  const { shown, frames, v } = withProgress();
  const waiting = v.progress(41, 2500);
  assert.deepEqual(shown, [[41, 2500]]);
  assert.equal(frames.length, 1, "the screen was never given a chance to draw");

  let done = false;
  waiting.then(() => { done = true; });
  await Promise.resolve();
  assert.equal(done, false, "it carried on without waiting for the frame");
  frames.pop()();
  await waiting;
});

// Asked for every bar of a long score, it must not hand back a frame every
// bar: two and a half thousand frames is forty seconds of waiting for a
// script that would otherwise have taken no time at all.
test("progress hands back a frame a few times a second, not every call", () => {
  const { shown, frames, v } = withProgress();
  for (let i = 0; i < 500; i++) v.progress(i, 500);
  assert.equal(shown.length, 1, `it drew ${shown.length} times in no time at all`);
  assert.equal(frames.length, 1);
});

// The reading time is not the script's — but the counting is, and a script
// that called this instead of writing would otherwise never be stopped.
test("a script that only reports its progress is still on the clock", () => {
  const { v } = withProgress({ steps: 4000, ms: -1 });
  assert.throws(() => v.progress(1, 2), /running too long/);
});

// Called without the await, which is what somebody will write first. It
// still has to be the budget that stops a runaway loop, and a throw inside
// a promise nobody is waiting for stops nothing.
test("progress without the await is still stopped by the budget", () => {
  const { v } = withProgress({ steps: 4000, ms: -1 });
  assert.throws(() => { for (;;) v.progress(1, 2); }, /running too long/);
});

// And it is taken away when the script stops, however it stops. A bar left
// at four fifths is a script that looks as though it is still going.
test("the bar is taken away when the script stops", () => {
  const fn = grab("runConsole");
  assert.match(fn, /finally \{[\s\S]{0,400}showConsoleProgress\(null\)/,
    "a script that threw halfway leaves its progress bar on the screen");
});

test("the bar shows a count, or nothing where there is no total", () => {
  const made = {};
  const el = (sel) => ((id) => (made[id] = made[id] ||
    { id, hidden: false, value: 0, max: 1, textContent: "", attrs: {},
      setAttribute(k, v) { this.attrs[k] = v; },
      removeAttribute(k) { delete this.attrs[k]; } }))(
    String(sel).replace(/^#/, ""));
  const show = new Function("$",
    grab("showConsoleProgress") + "\nreturn showConsoleProgress;")(el);

  show(41, 2500);
  assert.equal(el("console-progress").hidden, false);
  assert.equal(el("console-progress-text").textContent, "41 of 2500");
  assert.deepEqual([el("console-progress-bar").value,
                    el("console-progress-bar").max], [41, 2500]);

  // Past the end: a script that miscounted its own total should not draw a
  // bar wider than the bar.
  show(3000, 2500);
  assert.equal(el("console-progress-bar").value, 2500);

  // No total at all: how far it has got, with nothing to measure it
  // against, so the bar shows nothing rather than guessing.
  show(7);
  assert.equal(el("console-progress-text").textContent, "7");
  assert.equal(el("console-progress-bar").value, 0);
  assert.equal(el("console-progress-bar").attrs.waiting ||
               el("console-progress-bar").attrs["data-waiting"], "1");

  show(null);
  assert.equal(el("console-progress").hidden, true);
});

/*
 * Running the editor's own commands.
 *
 * Most of what a plug-in is made of elsewhere. Everything the editor can do
 * to the music is written and named already, so run("tie") is the whole of
 * what a script needs for it — and the question is not whether it works but
 * what it is pointed at. A script that could save, or undo, or put the
 * window somewhere else, is a script doing something that was never a
 * script's to decide, and a score arrives from anybody.
 */
const commands = (() => {
  const scope = {
    KEY_BINDINGS: table("KEY_BINDINGS"),
    WORD_COMMANDS: table("WORD_COMMANDS"),
    SCRIPT_GROUPS: konst("SCRIPT_GROUPS"),
    SCRIPT_NOT: konst("SCRIPT_NOT"),
    SCRIPT_ALSO: konst("SCRIPT_ALSO"),
  };
  return new Function(...Object.keys(scope),
    [grab("commandNamed"), grab("scriptMayRun"), grab("scriptRefusal")].join("\n") +
    "\nreturn { commandNamed, scriptMayRun, scriptRefusal, KEY_BINDINGS, WORD_COMMANDS };")(
    ...Object.values(scope));
})();

// A group named in the rule that no longer exists is a rule that refuses
// everything in silence: scripts would simply stop being able to write a
// tie, and nothing anywhere would say why.
test("every group a script may run from is a group there is", () => {
  const groups = new Set(commands.KEY_BINDINGS.map((c) => c.group));
  for (const g of konst("SCRIPT_GROUPS")) {
    assert.ok(groups.has(g), `no command is listed under "${g}" any more`);
  }
  for (const id of konst("SCRIPT_NOT")) {
    assert.ok(commands.commandNamed(id), `${id} is held back and does not exist`);
  }
  for (const id of konst("SCRIPT_ALSO")) {
    assert.ok(commands.commandNamed(id), `${id} is allowed and does not exist`);
  }
});

// The whole of the promise again, said about commands this time. The file
// is the one a script must not have: a script that can save is a script
// that can write over your work from a score somebody sent you.
test("a script cannot reach the file, undo, playing or the window", () => {
  for (const id of ["new", "open", "save", "save-pdf", "save-svg", "save-page",
                    "print", "undo", "redo", "redo-y", "copy", "cut", "paste",
                    "play", "play-from-start", "zoom-in", "zoom-reset",
                    "fullscreen", "mouse-writes", "type-music", "jump"]) {
    const cmd = commands.commandNamed(id);
    assert.ok(cmd, `${id} is not a command any more, so this says nothing`);
    assert.equal(commands.scriptMayRun(cmd), false, `a script can run ${id}`);
    assert.match(commands.scriptRefusal(cmd), /not a script's to run/);
  }
});

// And it can reach the editing, which is the point of having it at all.
test("a script can run the commands that write music", () => {
  for (const id of ["tie", "slur", "beam", "stem-up", "artic-staccato",
                    "hairpin-cresc", "tuplet-3", "dot", "sharpen",
                    "add-measure", "split-measure", "delete", "select-all",
                    "cursor-right", "nudge-up", "transpose-up"]) {
    const cmd = commands.commandNamed(id);
    assert.ok(cmd, `there is no ${id}`);
    assert.ok(commands.scriptMayRun(cmd), `a script cannot run ${id}`);
    assert.equal(commands.scriptRefusal(cmd), "");
  }
});

// Nothing is refused by having been forgotten. A command added to a group
// a script may not run is refused by having been added there, which is the
// whole reason the rule is written in groups.
test("a command in no group at all is refused unless it is named", () => {
  assert.equal(commands.scriptMayRun({ id: "whatever" }), false);
  assert.equal(commands.scriptMayRun({ id: "whatever", group: "The file" }), false);
  assert.equal(commands.scriptMayRun({ id: "whatever", group: "Marks" }), true);
});

// In the console the name is looked up where it is written, so a
// misspelled command is an error with nothing yet done to the score.
test("run queues a command and refuses one it will not do", () => {
  const scope = {
    Date, Math, JSON, String, Number, Array,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
    commandNamed: commands.commandNamed,
    scriptRefusal: commands.scriptRefusal,
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  const out = [], said = [];
  const v = make(out, said, { staves: [] });

  v.run("tie");
  // The start of the name will do, the same way the typing bar takes it,
  // and what is queued is the whole id rather than what was typed.
  v.run("artic-ten");
  assert.deepEqual(out, [{ do: "run", name: "tie" },
                         { do: "run", name: "artic-tenuto" }]);

  assert.throws(() => v.run("undo"), /not a script's to run/);
  assert.throws(() => v.run("save"), /not a script's to run/);
  assert.throws(() => v.run("nonsense"), /no command called nonsense/);
  assert.throws(() => v.run(""), /needs the name of a command/);
  assert.equal(out.length, 2, "a refused command was queued anyway");
});

// A bare word in a typed line is a command, so write() is the same door by
// another name — and it was open while run() was shut.
test("a command typed in a line is held to the same rule", () => {
  const fn = grab("runTyping");
  assert.match(fn, /async function runTyping\(line, fromScript\)/,
    "runTyping cannot tell a script from somebody typing");
  assert.match(fn, /if \(fromScript\)[\s\S]{0,120}scriptRefusal\(cmd\)/,
    "write(\"save\") runs the save that run(\"save\") refuses");
  assert.match(grab("doConsoleStep"), /runTyping\(step\.line, true\)/,
    "the console's own lines are not marked as coming from a script");
});

/*
 * What is written on a bar, and who is playing it.
 *
 * notes() answers "what is in this bar". It cannot answer "what is this bar
 * called", "does it carry a rehearsal mark", "which staff is the clarinet" —
 * and a script that reports a finding has to name the bar a player would
 * read, not the index it counted to.
 */
const marked = [{
  name: "Clarinet in A",
  instrument: "clarinet",
  clef: "treble",
  key: "D major",
  time: "3/4",
  transpose: -3,
  bars: [[{ pitch: "C4", length: 4, midi: 60, at: "0/1" }], []],
  measures: [
    { number: 12, marker: "A", tempo: "Allegro", barline: "double",
      notes: [{ pitch: "C4", length: 4, midi: 60, at: "0/1" }] },
    { number: 13, notes: [] },
  ],
}];

test("a bar says what is written on it, not only what is in it", () => {
  const v = reading(marked);
  const m = v.measure(1, 1);
  assert.equal(m.number, 12, "it did not come back with the number it prints");
  assert.equal(m.marker, "A");
  assert.equal(m.tempo, "Allegro");
  assert.equal(m.barline, "double");
  assert.equal(m.notes.length, 1, "the notes did not come with it");
  assert.equal(v.measure(2, 1).number, 13);
});

// The printed number and the index are different things. A score that
// starts with a pick-up bar, or restarts its count at a rehearsal letter,
// has a bar 12 that is not the twelfth — and "a parallel fifth in bar 12"
// has to mean the bar on the stand.
test("the number a bar prints is not assumed to be its position", () => {
  const v = reading(marked);
  assert.notEqual(v.measure(1, 1).number, 1,
    "the number was counted from the index rather than read from the score");
});

test("a bar nobody wrote comes back as nothing rather than throwing", () => {
  const v = reading(marked);
  assert.equal(v.measure(99, 1), null);
  assert.equal(v.measure(1, 9), null);
});

test("what a bar gives back is a copy too", () => {
  const v = reading(marked);
  v.measure(1, 1).marker = "Z";
  v.measure(1, 1).notes[0].pitch = "changed";
  assert.equal(v.measure(1, 1).marker, "A", "a script could edit the score by reading it");
  assert.equal(v.measure(1, 1).notes[0].pitch, "C4", "nor a note in it");
});

// A script checking whether the clarinet goes over its range has to know
// the staff is a clarinet, and how far it is written from what it sounds.
test("the staves say who is playing and in what", () => {
  const v = reading(marked);
  const [st] = v.staves();
  assert.equal(st.number, 1, "they are numbered the way the page is, from one");
  assert.equal(st.name, "Clarinet in A");
  assert.equal(st.instrument, "clarinet");
  assert.equal(st.clef, "treble");
  assert.equal(st.key, "D major");
  assert.equal(st.time, "3/4");
  assert.equal(st.transpose, -3, "the part would be read in the wrong key");
  assert.equal(st.bars, 2);
});

test("a staff with nothing said about it still answers", () => {
  const v = reading([{ name: "Staff 1", bars: [[]] }]);
  const [st] = v.staves();
  assert.deepEqual(
    [st.instrument, st.clef, st.key, st.time, st.transpose, st.bars],
    ["", "", "", "", 0, 0],
    "an unfilled field came back as undefined rather than as nothing");
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
  // A score with music in it: the staves asked for go beside it.
  const { out, v } = verbs(undefined, {
    staves: [{ name: "Piano", bars: [[{ pitch: "C4", length: 4 }]] }],
  });
  assert.equal(v.staff("flute"), 2, "the first new staff of a one-staff score");
  assert.equal(v.staff("cello", { clef: "bass", name: "cello I" }), 3);
  assert.deepEqual(out, [
    { do: "staff", take: false, instrument: "flute", name: "", clef: "" },
    { do: "staff", take: false, instrument: "cello", name: "cello I", clef: "bass" },
  ]);
});

// A score that has just been made is one staff with nothing on it. A script
// writing for four instruments should finish with four staves, not five:
// the first one it asks for moves into the staff that is already there.
test("the first staff moves into the empty one a new score comes with", () => {
  const blank = { staves: [{ name: "Staff", bars: [[{ rest: true, length: 1 }]] }] };
  const { out, v } = verbs(undefined, blank);
  assert.equal(v.staff("flute"), 1, "it stood beside the empty staff");
  assert.equal(v.staff("cello"), 2, "and the second one lost count");
  assert.equal(out[0].take, true);
  assert.equal(out[1].take, false, "only the first one takes it over");

  // But an empty staff in the middle of somebody's own music is a staff
  // they meant to have, and is left where it is.
  const theirs = verbs(undefined, {
    staves: [
      { name: "Piano", bars: [[{ pitch: "C4", length: 4 }]] },
      { name: "Spare", bars: [[{ rest: true, length: 1 }]] },
    ],
  });
  assert.equal(theirs.v.staff("flute"), 3, "it took over a staff of theirs");
  assert.equal(theirs.out[0].take, false);
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
const example = (name) => {
  // Anchored on what comes after it: an example's own text has lines that
  // close an array, and a lazy match stops at the first of them.
  const m = src.match(
    /^const consoleExamples = \[([\s\S]*?)\n\];\n\nfunction drawExamples/m);
  if (!m) throw new Error("app.js has no consoleExamples");
  const all = new Function("return ([" + m[1] + "])")();
  const got = all.find((e) => e.name === name);
  if (!got) throw new Error("there is no example called " + name);
  return got;
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
    `"use strict";\n${example("After the Moonlight").body}\n`);
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
    [["piano", "oboe", "contrabass", "alto sax", "drum kit"], "12"]);
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
  assert.ok(bars.length >= 40, `only ${bars.length} bars were written`);
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

/*
 * The other example: a check that reads and changes nothing.
 *
 * The two examples are there to show the two halves. One writes a piece and
 * never reads the score; this one reads the score and never writes. It is
 * the check every notation program ships, which makes it the one place
 * where a mistake in what /api/contents carries would be seen by somebody
 * who trusted the answer — a missing midi, an "at" that is wrong, a bar
 * number counted rather than read, and it reports parallels that are not
 * there or misses the ones that are.
 *
 * So it is run here on a fixture whose every answer is known by hand.
 */
async function runCheck(staves, picked) {
  const { said, v } = readVerbs(staves, picked);
  const bars = Math.max(...staves.map((s) => s.measures.length));
  // An async function, because the check awaits the drawing of its own
  // progress — which is the shape of every script that walks a long score.
  const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
  await new AsyncFunction(...Object.keys(v), "score", "Math",
    `"use strict";\n${plugin("parallels.js")}\n`)(
    ...Object.values(v), { bars, staves: staves.length }, Math);
  return said;
}

// Two voices, a bar at a time, each bar an answer worked out by hand. The
// notes carry what the reader really sends: where in the bar they fall and
// what they sound, which is the pair the whole check rests on.
const pair = (upper, lower) => {
  const staff = (name, bars) => ({
    name,
    bars: bars.map((b) => b.map((n) => ({ ...n, length: 4 }))),
    measures: bars.map((b, i) => ({
      number: i + 1,
      notes: b.map((n) => ({ ...n, length: 4 })),
    })),
  });
  return [staff("Soprano", upper), staff("Bass", lower)];
};

test("the check finds the parallels a theory teacher would ring", async () => {
  // Bar 1  C5→D5 over F3→G3: a twelfth both times, both rising. Fifths.
  // Bar 2  C5→D5 over C4→D4: an octave both times, both rising. Octaves.
  // Bar 3  C5 twice over F3 twice: a fifth that does not move at all.
  // Bar 4  C5→D5 over C5→D5: a unison, which is a different argument.
  // Bar 5  C5→D5 over F3 held: the fifth becomes a sixth. Nothing.
  // Bar 6  C5→D5 over G3→F3: contrary motion into a fourth. Nothing.
  const said = await runCheck(pair([
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 72, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
  ], [
    [{ midi: 53, at: "0/1" }, { midi: 55, at: "1/2" }],
    [{ midi: 60, at: "0/1" }, { midi: 62, at: "1/2" }],
    [{ midi: 53, at: "0/1" }, { midi: 53, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 53, at: "0/1" }, { midi: 53, at: "1/2" }],
    [{ midi: 55, at: "0/1" }, { midi: 53, at: "1/2" }],
  ]));

  const found = said.filter((l) => l.startsWith("Bar "));
  assert.deepEqual(found, [
    "Bar 1: parallel fifths, beat 0/1 to 1/2.",
    "Bar 2: parallel octaves, beat 0/1 to 1/2.",
  ], "the check does not say what it found, or says what is not there");
  assert.match(said.join("\n"), /Looking at Soprano against Bass\./,
    "it does not say which two parts it compared");
  assert.match(said.join("\n"), /2 to look at/);
});

// "Do this to what I have selected" is what a check is usually wanted for:
// somebody has written four bars and wants those four looked at, not the
// ninety they wrote last week. And it says which bars it looked at, so
// nobody reads "nothing found" as being about the whole piece.
test("the check looks at the stretch that was picked up", async () => {
  const bars = (lower) => pair([
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
  ], lower);
  // Parallel fifths in the first bar and the third, and the second clean.
  const lower = [
    [{ midi: 53, at: "0/1" }, { midi: 55, at: "1/2" }],
    [{ midi: 53, at: "0/1" }, { midi: 53, at: "1/2" }],
    [{ midi: 53, at: "0/1" }, { midi: 55, at: "1/2" }],
  ];

  const all = await runCheck(bars(lower));
  assert.equal(all.filter((l) => l.startsWith("Bar ")).length, 2);

  const some = await runCheck(bars(lower), {
    staff: 1, bar: 2, at: 0, count: 2,
    from: { staff: 1, bar: 2, index: 0 },
    to: { staff: 1, bar: 2, index: 1 },
  });
  assert.deepEqual(some.filter((l) => l.startsWith("Bar ")), [],
    "it looked outside the bars that were picked up");
  assert.match(some.join("\n"), /bars 2 to 2\./,
    "it does not say which bars it looked at, so nothing found reads as " +
    "nothing found anywhere");
});

// Rests, and notes that do not fall together, are not motion. Two parts
// whose notes land between each other are not moving in parallel whatever
// the intervals look like written down.
test("the check holds its tongue where there is nothing to hear", async () => {
  const apart = await runCheck(pair([
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
    [{ midi: 72, at: "0/1" }, { midi: 74, at: "1/2" }],
  ], [
    [{ midi: 53, at: "1/4" }, { midi: 55, at: "3/4" }],
    [{ rest: true, at: "0/1" }, { midi: 55, at: "1/2" }],
  ]));
  assert.ok(!apart.some((l) => l.startsWith("Bar ")),
    "parallels were reported between notes that never sound together");
  assert.match(apart.join("\n"), /No parallel fifths or octaves/);

  // And a score of one staff has nothing to compare.
  const alone = await runCheck([{
    name: "Piano",
    bars: [[{ midi: 60, at: "0/1" }]],
    measures: [{ number: 1, notes: [{ midi: 60, at: "0/1" }] }],
  }]);
  assert.match(alone.join("\n"), /needs two staves/);
});

// A fifth is a fifth however far apart the two parts are written: a
// twelfth and a nineteenth are the same interval to the ear and to the
// rule, and a check that only knew about the plain fifth would pass most
// of a piano texture without looking at it.
test("the check counts compound fifths and octaves as what they are", async () => {
  const said = await runCheck(pair([
    [{ midi: 84, at: "0/1" }, { midi: 86, at: "1/2" }],
    [{ midi: 84, at: "0/1" }, { midi: 86, at: "1/2" }],
  ], [
    [{ midi: 53, at: "0/1" }, { midi: 55, at: "1/2" }],
    [{ midi: 48, at: "0/1" }, { midi: 50, at: "1/2" }],
  ]));
  assert.deepEqual(said.filter((l) => l.startsWith("Bar ")), [
    "Bar 1: parallel fifths, beat 0/1 to 1/2.",
    "Bar 2: parallel octaves, beat 0/1 to 1/2.",
  ], "a fifth two octaves wide was not seen as a fifth");
});

/*
 * What a script has to show, and where it is shown.
 *
 * "I ran it and I don't see anything" was the complaint, and it was fair:
 * what a script said went into the console's own window, which is modal and
 * sits over the music the script is about. So the window closes when Run is
 * pressed and the report goes to a panel in the corner — shown, not shown
 * modally, so the score stays usable underneath it.
 */
test("running a script closes the console and opens the panel", () => {
  const fn = grab("runConsoleAndShow");
  assert.ok(fn.indexOf("dlg.close()") < fn.indexOf("await runConsole"),
    "the console is still over the music while the script runs");
  assert.match(fn, /showScriptResult\(/, "nothing shows what it had to show");

  const show = grab("showScriptResult");
  assert.match(show, /dlg\.show\(\)/,
    "the panel is shown modally, so the score cannot be looked at");
  assert.ok(!/showModal/.test(show));
  assert.match(html, /<dialog id="script-result">/, "there is no panel");
});

// Always, even when there is nothing to say: a panel that says the script
// looked and found nothing is the answer to "nothing happened".
test("the panel comes up even when the script found nothing", () => {
  const fn = grab("runConsoleAndShow");
  assert.ok(!/if\s*\([\s\S]{0,60}\)\s*showScriptResult/.test(fn),
    "the panel is only shown when there is something in it, so a script " +
    "that found nothing looks like a script that did nothing");
  assert.match(fn, /"Nothing to do\."/);
});

// And it says which script, and what that script is for — which is the
// other half of the complaint: not knowing what a plugin does.
test("the panel says which script ran and what it is for", () => {
  assert.match(html, /id="script-result-name"/);
  assert.match(html, /id="script-result-about"/);
  const run = grab("runPlugin");
  assert.match(run, /runConsoleAndShow\(found\.body, found\.name, found\.about\)/,
    "a plugin's own description is not passed on, so the panel cannot say " +
    "what the thing that just ran was for");
});

/*
 * Pictures.
 *
 * A great deal of what a script finds out is not a sentence. A flute
 * fingering is a row of circles with some of them filled in; said in words
 * it is something to decode rather than something to read.
 */
test("draw collects pictures beside what was said", () => {
  const out = [], said = [], drawings = [];
  const scope = {
    Date, Math, JSON, String, Number, Array,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  const v = make(out, said, { staves: [] }, null, {}, drawings);

  v.draw("<svg></svg>", "Bar 12");
  v.draw("<svg></svg>");
  assert.deepEqual(drawings, [
    { svg: "<svg></svg>", caption: "Bar 12" },
    { svg: "<svg></svg>", caption: "" },
  ]);
  assert.equal(out.length, 0, "drawing a picture wrote to the score");
  assert.throws(() => v.draw("   "), /needs something to draw/);
});

// A script that draws for ever is stopped, the way one that writes for ever
// is: the budget counts what is written and this is not written, so it
// needs its own bound.
test("a script cannot draw without end", () => {
  const drawings = [];
  const scope = {
    Date, Math, JSON, String, Number, Array,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  const v = make([], [], { staves: [] }, null, {}, drawings);
  assert.throws(() => { for (;;) v.draw("<svg/>"); }, /200 pictures/);
});

test("what a script drew reaches the panel", () => {
  assert.match(grab("runConsole"), /const queue = \[\], said = \[\], drawings = \[\]/);
  assert.match(grab("runConsole"), /return \{ said, drawings, wrote, error \}/);
  const show = grab("showScriptResult");
  assert.match(show, /figcaption/, "a picture's caption is dropped");
  assert.match(show, /picture\.svg/);
  assert.match(html, /id="script-result-drawn"/);
});

/*
 * The Scripts menu.
 *
 * Built out of the folder, which is what makes the folder the installation.
 * The things worth holding on to are that it is built from what the server
 * read rather than from a list the page keeps, that it never appears in
 * somebody else's browser, and that a plugin which did not ask to be on a
 * menu is not on it.
 */
test("the Scripts menu is built out of the folder", () => {
  assert.match(html, /<div class="menu" data-menu="scripts"[^>]*>/,
    "there is no Scripts menu");
  const block = /<div class="menu" data-menu="scripts"([\s\S]*?)<\/div>\s*<\/div>/
    .exec(html);
  assert.ok(block, "the Scripts menu has no items list");
  assert.match(block[0], /id="scripts-items"/,
    "nothing in it for the scripts to be put into");
  // The folder is the server's wherever gscore is not the application on
  // this machine, so the whole menu goes — the same rule the recent files
  // live under.
  assert.match(block[0], /data-needs-files/,
    "a browser elsewhere is offered the server owner's scripts");

  const fn = grab("loadPlugins");
  assert.match(fn, /api\/plugins/, "it does not read the folder");
  assert.match(fn, /filter\(\(p\) => p\.menu\)/,
    "a script that never asked to be on a menu is put on it");
  assert.match(fn, /syncMenuIcons\(\)/,
    "the entries it makes have no pictures beside them");
  assert.match(fn, /rebuildNativeMenus\(\)/,
    "the application's own menu bar is never told the list changed");
});

// A plugin is named by its file and not by what it calls itself: two
// plugins calling themselves the same thing are still two commands.
test("a plugin is run by the name of its file", () => {
  const fn = grab("action");
  assert.match(fn, /name\.startsWith\("plugin:"\)[\s\S]{0,200}runPlugin/,
    "picking one off the menu does nothing");
  const run = grab("runPlugin");
  assert.match(run, /plugins\.find\(\(p\) => p\.file === file\)/,
    "it is found by what it calls itself, which two of them may share");
  // The script goes into the console box without the window being opened,
  // so that "Open in the console" has it there to read afterwards — and so
  // that opening the console after a plugin ran shows what ran.
  assert.ok(!/openConsole\(\)/.test(run),
    "running a plugin opens the console over the music it just changed");
  assert.match(run, /console-source[\s\S]{0,160}found\.body/,
    "the script that ran is not put where it can be read");
});

// One place where a script is run and its output written, because there are
// two ways in: the Run button and the Scripts menu. A plugin that said
// nothing where the same script run from the button said five things would
// be a plugin that looked broken.
test("both ways of running a script show what it said", () => {
  const fn = grab("runConsoleAndShow");
  assert.match(fn, /runConsole\(source\)/);
  assert.match(fn, /consoleSay\(/);
  assert.match(src, /#console-run[\s\S]{0,120}runConsoleAndShow/,
    "the Run button has its own copy of this");
  assert.match(grab("runPlugin"), /runConsoleAndShow/,
    "the menu has its own copy of this");
});

/*
 * The samples in the plugins folder.
 *
 * They are written into Gscore/plugins on every new installation and sit on
 * the Scripts menu, so a verb renamed under one of them is a sample that
 * fails on the first click somebody ever gives it. Run here, against the
 * same verbs the console gives them.
 */
async function runPlugin(file, { staves, answers = [], picked, chords } = {}) {
  const out = [], said = [], drawings = [];
  const scope = {
    Date, Math, JSON, String, Number, Array, Object, Promise,
    state: { selection: { part: 0 } },
    typeLengths: konst("typeLengths"),
    consoleBudget: konst("consoleBudget"),
    transposeName: new Function("return " + grab("transposeName"))(),
    letterSemis: konst("letterSemis"),
    latinPlain: konst("latinPlain"),
    latinVowels: konst("latinVowels"),
    latinMutes: konst("latinMutes"),
    latinAfterS: konst("latinAfterS"),
    chordNotesOf: spell,
    latinSyllables: syllables,
  };
  const make = new Function(...Object.keys(scope),
    grab("consoleVerbs") + "\nreturn consoleVerbs;")(...Object.values(scope));
  let asked = 0;
  const v = make(out, said, { staves: staves || [] },
                 async () => answers[asked++],
                 { picked, chords: chords ?? QUALITIES }, drawings);
  const bars = Math.max(1, ...(staves || []).map((s) => (s.measures || []).length));
  const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
  await new AsyncFunction(...Object.keys(v), "score", "Math", "JSON",
    `"use strict";\n${plugin(file)}\n`)(
    ...Object.values(v), { bars, staves: (staves || []).length }, Math, JSON);
  return { out, said, drawings };
}

// A lead sheet: a tune with chord symbols over it. What the accompaniment
// is made of is what is written above the notes, and nothing else.
const leadSheet = () => {
  const bar = (n, symbol) => ({
    number: n,
    notes: [{ pitch: "C5", length: 4, midi: 72, at: "0/1", chordSymbol: symbol }],
  });
  const bars = [bar(1, "C"), bar(2, "Am7"), bar(3, "Dm7"), bar(4, "G7")];
  return [{
    name: "Tune",
    bars: bars.map((b) => b.notes),
    measures: bars,
  }];
};

test("the accompaniment sample writes a chord under every symbol", async () => {
  const { out, said } = await runPlugin("accompaniment.js", {
    staves: leadSheet(),
    answers: ["1", "root then chord"],
  });
  assert.match(said.join("\n"), /4 chords to set/);
  assert.match(said.join("\n"), /Wrote 4 of them/);

  // It writes onto a staff of its own, and never into the staff it read.
  const made = out.filter((s) => s.do === "staff");
  assert.equal(made.length, 1, "it did not make a staff to write on");
  assert.equal(made[0].clef, "bass");
  for (const step of out) {
    if (step.do === "at") {
      assert.equal(step.staff, 1,
        "it wrote into the staff the tune is on, which is the one thing a " +
        "script must not do to music somebody is looking at");
    }
  }

  // Every note it wrote is spelled out — accidental or natural — because
  // the staff it goes on may be in a key the script was not thinking in.
  const written = [];
  for (const s of out) {
    if (s.do === "note") written.push(s.pitch);
    if (s.do === "chord") written.push(...s.pitches);
  }
  assert.ok(written.length >= 12, `only ${written.length} notes were written`);
  for (const name of written) {
    assert.match(name, /^[a-g](#|x|b|bb|n)-?\d$/,
      `${name} was left for the key signature to spell`);
  }
});

// A chord nobody can read does not stop the run: a lead sheet with one
// unreadable symbol in it is still a lead sheet worth accompanying.
test("the accompaniment sample says which chords it could not read", async () => {
  const sheet = leadSheet();
  sheet[0].measures[1].notes[0].chordSymbol = "Czonk";
  sheet[0].bars = sheet[0].measures.map((m) => m.notes);

  const { said } = await runPlugin("accompaniment.js", {
    staves: sheet, answers: ["1", "root then chord"],
  });
  assert.match(said.join("\n"), /Wrote 3 of them/);
  assert.match(said.join("\n"), /Czonk/);
});

test("the accompaniment sample writes nothing when it is not answered", async () => {
  for (const answers of [[null], ["1", null], ["9", "root then chord"]]) {
    const { out } = await runPlugin("accompaniment.js",
      { staves: leadSheet(), answers });
    assert.equal(out.length, 0,
      `it wrote anyway after ${JSON.stringify(answers)}`);
  }
});

// A flute part: a few notes either side of the break, which is where the
// fingering changes almost completely and where a passage trips.
const flutePart = (midis) => {
  const bars = midis.map((row, i) => ({
    number: i + 1,
    notes: row.map((m, j) => ({
      pitch: "x", length: 8, midi: m, at: j + "/8",
    })),
  }));
  return [{
    name: "Flute", instrument: "flute", clef: "treble",
    bars: bars.map((b) => b.notes),
    measures: bars,
  }];
};

test("the fingering sample draws the keys for every note", async () => {
  // G4, A4, B♭4, B4, C5, C♯5, D5 — up through the break.
  const { out, said, drawings } = await runPlugin("flute-fingerings.js", {
    staves: flutePart([[67, 69, 70, 71], [72, 73, 74]]),
  });
  assert.equal(out.length, 0, "a script that only reads wrote to the score");
  const text = said.join("\n");
  assert.match(text, /Reading Flute/,
    "it did not find the flute staff on its own");
  assert.match(text, /7 notes, 7 of them different, from G4 to D5/);
  // The intonation, which is the thing the notes on the page cannot say.
  assert.match(text, /Watch the tuning on[\s\S]*C#5 \(25 cents sharp\)/);

  // One diagram per note used, plus the B flat it was asked about.
  assert.ok(drawings.length >= 7, `only ${drawings.length} drawings`);
  for (const d of drawings) {
    assert.match(d.svg, /^<svg viewBox="0 0 306 100"/);
    assert.match(d.svg, /<circle /, "a diagram with no keys on it");
    assert.ok(!/var\(/.test(d.svg),
      "a theme colour in an SVG attribute, which is never resolved");
  }
});

// The fingering of each note is the chart's, and the chart is checked
// against the two anybody can verify: low D is all three fingers of both
// hands, and C sharp above the break is the E flat key alone.
test("the fingering sample draws the fingerings the chart has", async () => {
  const { drawings } = await runPlugin("flute-fingerings.js", {
    staves: flutePart([[62], [73]]),
  });
  const of = (name) => drawings.find((d) => d.caption.startsWith(name));
  assert.ok(of("D4"), "no diagram for low D");
  assert.match(of("D4").svg, /aria-label="D4: Th 1 2 3 \| 1 2 3"/);
  assert.match(of("C#5").svg, /aria-label="C#5: \| D#"/);
});

// The move, which is what is actually being practised: a passage is learnt
// join by join, not note by note.
test("the fingering sample marks which fingers move next", async () => {
  const { said, drawings } = await runPlugin("flute-fingerings.js", {
    staves: flutePart([[72, 74, 72, 74]]),
    picked: {
      staff: 1, bar: 1, at: 0, count: 4,
      from: { staff: 1, bar: 1, index: 0 },
      to: { staff: 1, bar: 1, index: 3 },
    },
  });
  // C5 to D5 is the break: almost every finger changes, and they change in
  // both directions at once.
  assert.match(said.join("\n"), /The joins to practise:/);
  assert.match(said.join("\n"),
    /bar 1, C5 to D5 — 6 down, 2 up, both at once/);
  // And the picture says it: triangles for the keys that move.
  const first = drawings.find((d) => d.caption.includes("C5 → D5"));
  assert.ok(first, "no diagram of the move");
  assert.match(first.svg, /<polygon /, "nothing in the picture says what moves");
});

// The one real fingering decision on a flute, and the one it is worth a
// script knowing: which of the three B flats to use here.
test("the fingering sample chooses between the three B flats", async () => {
  // B♭ with a B natural beside it: the thumb cannot be in two places.
  const near = await runPlugin("flute-fingerings.js", {
    staves: flutePart([[70, 71, 70]]),
  });
  assert.match(near.said.join("\n"), /one and one in bar 1/);
  assert.match(near.said.join("\n"), /B natural is next to it/);

  // B♭ going to C: what the lever is for.
  const toC = await runPlugin("flute-fingerings.js", {
    staves: flutePart([[70, 72, 70]]),
  });
  assert.match(toC.said.join("\n"), /the B♭ lever in bar 1/);

  // And on its own, in a flat key: the thumb.
  const alone = await runPlugin("flute-fingerings.js", {
    staves: flutePart([[65, 70, 65]]),
  });
  assert.match(alone.said.join("\n"), /thumb B♭ in bar 1/);
});

// A note no flute can play is said rather than drawn as something.
test("the fingering sample says what is outside the range", async () => {
  const { said } = await runPlugin("flute-fingerings.js", {
    staves: flutePart([[48, 62, 108]]),
  });
  assert.match(said.join("\n"), /Outside the flute's range[^\n]*C3/);
  assert.match(said.join("\n"), /Outside the flute's range[^\n]*C8/);
});

test("the Latin sample hyphenates a text and changes nothing", async () => {
  const { out, said } = await runPlugin("latin-text.js", {
    staves: leadSheet(),
    answers: ["Kyrie eleison.\nAgnus Dei, qui tollis peccata mundi."],
  });
  assert.equal(out.length, 0, "a script that only reads wrote to the score");
  const text = said.join("\n");
  assert.match(text, /Ky-ri-e e-le-i-son\./);
  assert.match(text, /A-gnus De-i, qui tol-lis pec-ca-ta mun-di\./);
  // The count beside each line, so it can be checked against the notes:
  // Ky-ri-e e-le-i-son is seven, and the Agnus line twelve.
  assert.match(text, /\(7\)/);
  assert.match(text, /\(12\)/);
  assert.match(text, /8 words, 19 syllables\./);
});

test("the Latin sample leaves a word it cannot read whole, and says so", async () => {
  const { said } = await runPlugin("latin-text.js", {
    staves: leadSheet(), answers: ["Kyrie willow eleison"],
  });
  const text = said.join("\n");
  assert.match(text, /willow/);
  assert.match(text, /Left whole/);
  assert.ok(!/wil-low|wil-lo/.test(text), "it divided a word it cannot read");
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
  const { out } = await runExample([["alto sax", "piano", "french horn"], "8"]);
  const written = [];
  for (const s of out) {
    if (s.do === "note") written.push(s.pitch);
    if (s.do === "chord") written.push(...s.pitches);
  }
  assert.ok(written.length > 50, "nothing was written to look at");
  for (const name of written) {
    assert.match(name, /^[a-g](#|x|b|bb|n)-?\d$/,
      `${name} was left for the key signature to spell`);
  }
  // And a transposing part really is moved: the alto saxophone reads a
  // major sixth above what it sounds, so it has notes the piano has not.
  const pitches = new Set(written);
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
