/*
 * scoresetup_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Choosing who is playing.
 *
 * The thing worth testing here is the list with an order: adding, taking
 * away and moving are all done against a list that is sometimes being
 * sorted under the person's hands, and the indices in the box are the
 * sorted ones rather than the list's own.
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

// A catalogue with score ranks: flute above trumpet above cello, which is
// how a score is laid out and not how the picker is ordered.
const CATALOGUE = [
  { sound: "strings.cello", name: "cello", group: "Strings — bowed", scoreRank: 140 },
  { sound: "wind.flutes.flute", name: "flute", group: "Woodwind", scoreRank: 1 },
  { sound: "brass.trumpet", name: "trumpet", group: "Brass", scoreRank: 40 },
];

function load() {
  const scope = {
    instrumentCatalogue: CATALOGUE,
    instrumentNamed: (s) => (CATALOGUE.find((x) => x.sound === s) || {}).name || s,
  };
  // inScoreOrder reads the way an instrument is written off the value,
  // since where a thing goes down the page is about the instrument.
  const body = grab("soundChosen") + "\n" + grab("inScoreOrder") +
    "\nreturn { inScoreOrder };";
  return new Function(...Object.keys(scope), body)(...Object.values(scope));
}

// The chosen list holds an entry per staff, each with an id of its own, so
// a group survives the list being reordered under it.
let nextId = 1;
const entries = (...sounds) => sounds.map((sound) => ({ id: nextId++, sound }));
const sounds = (list) => list.map((it) => it.sound);

test("the chosen instruments are put in score order", () => {
  const { inScoreOrder } = load();
  const got = inScoreOrder(entries("strings.cello", "brass.trumpet", "wind.flutes.flute"));
  assert.deepEqual(sounds(got),
    ["wind.flutes.flute", "brass.trumpet", "strings.cello"]);
});

test("two of the same keep the order they were chosen in", () => {
  // The first flute stays the first flute.
  const { inScoreOrder } = load();
  const first = entries("wind.flutes.flute")[0];
  const second = entries("wind.flutes.flute")[0];
  const got = inScoreOrder([first, second, ...entries("brass.trumpet")]);
  assert.equal(got[0].id, first.id);
  assert.equal(got[1].id, second.id);
});

test("an instrument the catalogue does not know goes last, not first", () => {
  const { inScoreOrder } = load();
  const got = inScoreOrder(entries("wind.reed.kazoo", "wind.flutes.flute"));
  assert.deepEqual(sounds(got), ["wind.flutes.flute", "wind.reed.kazoo"]);
});

/*
 * The list the dialog keeps.
 */

test("moving by hand turns the automatic order off", () => {
  // The two cannot both be in charge, and the one somebody just used
  // their hands on is the one that wins.
  const body = grab("setupArrangedByHand");
  assert.match(body, /order\?\.checked/);
  assert.match(body, /order\.checked = false/);
  // And the list is frozen in the order shown before the swap, or the
  // move would be against indices that are about to change.
  assert.match(body, /setupChosen = inScoreOrder\(setupChosen\)/);
  assert.match(grab("setupMove"), /setupArrangedByHand\(\)/);
});

test("removing works against the order on the screen", () => {
  // The numbers in the box are the sorted positions, not the list's own,
  // so what is picked is read through the order that is on show.
  assert.match(grab("setupPicked"), /const list = setupOrdered\(\)/);
  const body = grab("setupRemove");
  assert.match(body, /setupOrdered\(\)\.filter/);
  // And a staff taken away leaves the groups it was in.
  assert.match(body, /setupGroups = setupGroups/);
});

test("reopening shows the score as it stands", () => {
  const body = grab("scoreSetupDialog");
  assert.match(body, /state\.info\?\.parts \|\| \[\]/);
  // A grand staff is two staves and one instrument; counting both would
  // add a second piano every time the dialog was reopened.
  assert.match(body, /s !== all\[i - 1\]/);
});

/*
 * Bracketing by hand.
 */

test("a bracket needs two staves or more", () => {
  const body = grab("setupBracket");
  assert.match(body, /picked\.length < 2/);
});

test("staves picked apart from each other are brought together", () => {
  // A bracket is a line down the side of a run, so it covers everything
  // between its ends: there is no bracketing the first and the third and
  // not the second.
  const body = grab("setupBracket");
  assert.match(body, /const moved = setupChosen\.filter/);
  assert.match(body, /const rest = setupChosen\.filter/);
  assert.match(body, /setupChosen = rest\.slice\(0, at\)\.concat\(moved, rest\.slice\(at\)\)/);
  // And it says so, because it moves the score about.
  assert.match(body, /brought together/);
});

test("a staff is in one bracket at a time", () => {
  const body = grab("setupBracket");
  assert.match(body, /setupGroups = setupGroups/);
  assert.match(body, /\.filter\(\(g\) => g\.length > 1\)/);
});

test("the groups are sent as runs over the list that is sent", () => {
  const body = grab("applyScoreSetup");
  assert.match(body, /const runs = setupGroups\.map/);
  assert.match(body, /p\.set\("brackets", runs\.join\(","\)\)/);
  // The instruments sent are the ones in the order shown, so the runs
  // and the list agree.
  assert.match(body, /const list = setupOrdered\(\)/);
});

test("the groups are kept by id, not by position", () => {
  // A group named by position stops meaning what it meant the moment
  // anything is moved, and moving things is most of what this is for.
  assert.match(src, /let setupGroups = \[\]/);
  assert.match(grab("setupBracket"), /picked\.map\(\(it\) => it\.id\)/);
});

test("it asks before throwing music away", () => {
  const body = grab("applyScoreSetup");
  assert.match(body, /scoreHasMusic\(\)/);
  assert.match(body, /await confirmThat\(/);
  assert.match(body, /p\.set\("replace", "1"\)/);
});

test("the server is told to keep the order when the dialog has arranged it", () => {
  const body = grab("applyScoreSetup");
  assert.match(body, /if \(!\$\("#setup-order"\)\?\.checked\) p\.set\("order", "keep"\)/);
});

test("the picker is grouped the way the instrument list is", () => {
  const body = grab("drawSetupPick");
  assert.match(body, /<optgroup label=/);
  // Searching matches the heading too, so "brass" finds the brass.
  assert.match(body, /o\.group\.toLowerCase\(\)\.includes\(q\)/);
});

/*
 * Where it lives.
 */

test("score setup is on the Score menu", () => {
  const menu = html.match(/<div class="menu" data-menu="score">[\s\S]*?<\/div>\s*<\/div>/);
  assert.ok(menu, "there is no Score menu");
  assert.match(menu[0], /data-action="score-setup"/);
});

test("the dialog has the controls the list needs", () => {
  for (const id of ["setup-filter", "setup-pick", "setup-chosen", "setup-add",
                    "setup-remove", "setup-up", "setup-down", "setup-order",
                    "setup-apply"]) {
    assert.ok(html.includes(`id="${id}"`), "index.html has no #" + id);
  }
});
