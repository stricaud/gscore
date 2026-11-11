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

/*
 * Opening on a score that is already there.
 *
 * This is where the dialog was wrong, and the way it was wrong is worth
 * writing down. It built its list out of the parts' sound ids and threw
 * away the ones that were empty — so a score read out of a PDF, whose
 * staves carry no sound at all, opened it on nothing. Eight staves on the
 * page, an empty list in the window that exists to say who is playing.
 *
 * The filter was not idle: it collapsed a run of equal sounds, because a
 * piano is one instrument on two staves and listing it twice adds a second
 * piano every time the dialog is reopened. That still has to hold. What
 * must not hold is collapsing eight staves that say nothing — they are not
 * one instrument, they are eight unanswered questions.
 */
// A catalogue that knows which instruments are written on two staves, as
// the server's own does: the editor must not keep a second list of them.
const GRAND = [
  ...CATALOGUE,
  { sound: "keyboard.piano", name: "piano", group: "Keyboard", scoreRank: 100, grand: true },
];

function scoreOf(parts, groups) {
  const scope = {
    state: { info: { parts, groups: groups || [] } },
    setupNextId: 1,
    setupChosen: [],
    // Nothing has been taken apart by hand: this is the reading as it
    // comes off a score just opened.
    setupApart: new Set(),
    instrumentCatalogue: GRAND,
    setupOrdered: () => scope.setupChosen,
  };
  const body = grab("soundChosen") + "\n" + grab("grandSound") + "\n" +
    grab("sameSoundRun") + "\n" +
    grab("setupFromScore") + "\n" + grab("setupInPlace") +
    "\nreturn { setupFromScore, setupInPlace, grandSound };";
  const made = new Function("scope", "with (scope) { " + body + " }")(scope);
  return { ...made, scope };
}

test("a staff that was never told what it is comes back as a piano", () => {
  // Which is what it plays. The alternative is the empty list that sent
  // somebody looking for this window in the first place.
  const { setupFromScore } = scoreOf([{ name: "Staff" }, { name: "Staff" }]);
  const got = setupFromScore();
  assert.equal(got.length, 2, "staves with no sound were dropped");
  assert.deepEqual(got.map((it) => it.sound),
    ["keyboard.piano", "keyboard.piano"]);
  assert.deepEqual(got.map((it) => it.staves), [[0], [1]]);
  // And it is recorded that the file did not in fact say so.
  assert.deepEqual(got.map((it) => it.said), [false, false]);
});

test("a grand staff is one instrument, not two", () => {
  const { setupFromScore } = scoreOf([
    { sound: "keyboard.piano" }, { sound: "keyboard.piano" },
    { sound: "strings.cello" },
  ]);
  const got = setupFromScore();
  assert.deepEqual(got.map((it) => it.sound),
    ["keyboard.piano", "strings.cello"]);
  // The pair is one entry over two staves, so changing it changes both.
  assert.deepEqual(got.map((it) => it.staves), [[0, 1], [2]]);
});

// What actually makes two staves one instrument is the brace. Everything
// else about a piano's two staves and two cellos' two staves is the same,
// so the brace is read rather than guessed at wherever there is one.
test("the brace is what says one player on two staves", () => {
  const { setupFromScore } = scoreOf(
    [{ sound: "strings.cello" }, { sound: "strings.cello" }],
    [{ first: 0, last: 1, symbol: "brace" }]);
  const got = setupFromScore();
  assert.equal(got.length, 1, "a braced pair was listed twice");
  assert.deepEqual(got[0].staves, [0, 1]);
});

test("a bracket does not make two players one", () => {
  // A bracket joins a family — two cellos, the whole string section. They
  // are still two people and still two entries.
  const { setupFromScore } = scoreOf(
    [{ sound: "strings.cello" }, { sound: "strings.cello" }],
    [{ first: 0, last: 1, symbol: "bracket" }]);
  assert.equal(setupFromScore().length, 2);
});

test("two of the same instrument are two players unless it takes two staves", () => {
  // Flute 1 and Flute 2 sit side by side in every band score ever printed.
  const flutes = scoreOf([
    { sound: "wind.flutes.flute" }, { sound: "wind.flutes.flute" },
  ]);
  assert.equal(flutes.setupFromScore().length, 2);

  // Three pianos in a row are three pianos, and none of them is paired
  // with another: a grand staff is a pair, so pairing the first two and
  // leaving the third would be picking which of three identical staves
  // gets a partner. Where the score means a pianist it says so with a
  // brace, and a brace is read rather than guessed at.
  const three = scoreOf([
    { sound: "keyboard.piano" }, { sound: "keyboard.piano" },
    { sound: "keyboard.piano" },
  ]);
  const got = three.setupFromScore();
  assert.deepEqual(got.map((it) => it.staves), [[0], [1], [2]]);
});

test("staves that name no instrument never pair off", () => {
  // They would all pair: the default they are shown as is a piano, and a
  // piano takes two staves. Eight staves read out of a PDF would come back
  // as four pianos, which is a guess at the music dressed up as a reading
  // of it. Only an instrument the file actually named can pair.
  const { setupFromScore } = scoreOf(Array.from({ length: 4 }, () => ({})));
  const got = setupFromScore();
  assert.equal(got.length, 4);
  assert.deepEqual(got.map((it) => it.sound[0] && it.said), [false, false, false, false]);
});

test("eight staves of nothing in particular are eight staves", () => {
  // The bug exactly: two blanks are not a grand staff, they are two staves
  // nobody has answered for yet.
  const { setupFromScore } = scoreOf(Array.from({ length: 8 }, () => ({})));
  assert.equal(setupFromScore().length, 8);
});

test("the staves can be changed in place while the list still is the score", () => {
  const s = scoreOf([{}, {}, {}]);
  s.scope.setupChosen = s.setupFromScore();
  assert.ok(s.setupInPlace(), "a list that is the score was refused");

  // Adding an instrument is building a different ensemble, which cannot be
  // done without laying the staves out again.
  s.scope.setupChosen = s.setupFromScore()
    .concat([{ id: 99, sound: "strings.cello" }]);
  assert.equal(s.setupInPlace(), null, "an added instrument was taken for a change of instrument");

  // So is taking one away.
  s.scope.setupChosen = s.setupFromScore().slice(0, 2);
  assert.equal(s.setupInPlace(), null);

  // And so is moving one: the music would have to move with it.
  const moved = s.setupFromScore();
  [moved[0], moved[1]] = [moved[1], moved[0]];
  s.scope.setupChosen = moved;
  assert.equal(s.setupInPlace(), null);

  // Changing what a staff plays is none of those things.
  const changed = s.setupFromScore();
  changed[1].sound = "wind.flutes.flute";
  s.scope.setupChosen = changed;
  assert.ok(s.setupInPlace());
});

test("a score nobody has started yet opens on an empty list", () => {
  // gscore opens with one staff ready so a tune can be typed in without
  // filling in a form first. That staff is not an answer to "who is
  // playing", and listing it would mean taking a piano out of the score
  // before a wind quintet could go in.
  const body = grab("scoreSetupDialog");
  assert.match(body, /!scoreHasMusic\(\) &&/);
  assert.match(body, /every\(\(p\) => !p\.sound\)/);
  assert.match(body, /blank \? \[\] : setupFromScore\(\)/);
  // And the staves it does list are in the order they are on the page, not
  // the order a score is conventionally laid out in.
  assert.match(body, /order\.checked = !scoreHasMusic\(\)/);
});

/*
 * A score that already has music in it.
 *
 * This is the bug that lost somebody's import, and it is worth being exact
 * about what it was. The window held two kinds of operation and gave no
 * sign of which was which. Change kept the music; Add and Remove quietly
 * built a *different* ensemble in a list beside the score; and the button
 * at the bottom laid that ensemble out over the top of the old one, empty
 * bars and all. Somebody naming eight imported staves reaches for Add and
 * Remove, because those are the obvious controls — and the music is gone.
 *
 * So over a score with music there is no second kind. Every operation goes
 * straight to the score, one undoable step at a time, and the list is read
 * back from the score afterwards. There is no path through this window
 * that empties a staff.
 */

test("over a score with music, nothing lays the staves out again", () => {
  const body = grab("drawSetupBar");
  assert.match(body, /const live = setupLive\(\)/);
  // The button that replaces the score is not merely demoted. It is gone.
  assert.match(body, /apply\.hidden = live/);
  assert.match(body, /inst\.hidden = !live/);
  // And the list is redrawn through the bar, so the bar cannot fall behind
  // the list it describes.
  assert.match(grab("drawSetupChosen"), /drawSetupBar\(\)/);
});

test("adding and removing go to the score, not to a list beside it", () => {
  const add = grab("setupAdd");
  assert.match(add, /if \(setupLive\(\)\)/);
  assert.match(add, /\/api\/addstaff\?/);
  const rm = grab("setupRemove");
  assert.match(rm, /if \(setupLive\(\)\)/);
  assert.match(rm, /\/api\/removestaff\?part=/);
  // Highest staff first, or taking one away renumbers the ones still to go.
  assert.match(rm, /sort\(\(a, b\) => b - a\)/);
  // Taking away a staff with music on it is the one thing here that loses
  // any, so it is the one thing that asks first.
  assert.match(rm, /notes > 0/);
  assert.match(rm, /confirmThat/);
});

test("moving a staff moves the staff, and a pair stays the right way up", () => {
  const body = grab("setupMoveStaves");
  assert.match(body, /\/api\/movestaff\?part=/);
  // A staff is moved by taking it out and putting it back, so a pair is two
  // of those and the order matters: going down, the lower goes first.
  assert.match(body, /if \(dir > 0\) steps\.reverse\(\)/);
});

test("bracketing a score's own staves needs them to be next to each other", () => {
  // A bracket is a line down the side of a run. In a list the staves can be
  // shuffled to suit it; in the score they are where they are, and moving
  // them is a bigger thing than drawing a bracket — so it is said, not done.
  const body = grab("setupBracket");
  assert.match(body, /not next to each other/);
  assert.match(body, /symbol: "bracket"/);
});

// Making a grand staff out of a score that arrived without one: a PDF of
// piano music read two staves to a system is two nameless staves and one
// pianist, and this is where that is said.
test("two staves given one two-staff instrument become one entry, braced", () => {
  const body = grab("setupChange");
  assert.match(body, /picked\.length === 2 && grandSound\(v\)/);
  // Side by side, and each still a single staff: a pair, not a run.
  assert.match(body, /picked\[1\]\.staves\[0\] === picked\[0\]\.staves\[0\] \+ 1/);
  assert.match(body, /picked\[0\]\.brace = true/);
  // And the brace is drawn when it is applied, with the barlines through
  // the gap, which is what says the two staves are played together.
  const apply = grab("applySetupInstruments");
  assert.match(apply, /it\.brace && it\.staves\.length === 2/);
  assert.match(apply, /symbol === "brace"\) p\.set\("barlines", "1"\)/);
});

test("the two-staff instruments are the server's list, not a second copy", () => {
  const { grandSound } = scoreOf([]);
  assert.equal(grandSound("keyboard.piano"), true);
  assert.equal(grandSound("wind.flutes.flute"), false);
  // An instrument nothing is known about is not one of them.
  assert.equal(grandSound("wind.reed.kazoo"), false);
});

test("setting the instruments leaves alone the staves that already play that", () => {
  // Opening the window and pressing the button should not come back with
  // every staff transposed.
  const body = grab("applySetupInstruments");
  assert.match(body, /parts\[n\]\?\.sound \|\| ""\) === it\.sound/);
  // It goes through the request that already changes one staff's
  // instrument, rather than inventing a second way to say it.
  assert.match(body, /\/api\/staff\?/);
  assert.doesNotMatch(body, /\/api\/setup/);
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
