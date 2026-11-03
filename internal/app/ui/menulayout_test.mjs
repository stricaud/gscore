/*
 * menulayout_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A menu taller than the window.
 *
 * The Insert menu is the long one: on a laptop screen its last entry,
 * Symbol, was drawn below the foot of the window — half-legible, impossible
 * to click, and nothing said it was there.
 *
 * Making it scroll is three rules that have to hold together, and dropping
 * any one of them breaks the menus in a way no other test here would catch.
 * A scrolling box clips what sticks out of it, and overflow-y without
 * overflow-x does not leave the x axis alone — it becomes auto too — so the
 * moment the panel scrolls, every flyout is cut off at its right edge
 * unless the flyouts have been taken out of the panel first. That is what
 * position: fixed is for, and a fixed panel has no parent to be positioned
 * against, so placeSubmenu has to put it somewhere itself.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

// The block of declarations under a selector, for asking what it sets.
function rule(selector) {
  const at = css.indexOf(selector + " {");
  if (at < 0) throw new Error("style.css has no rule for " + selector);
  return css.slice(at, css.indexOf("}", at));
}

test("a menu too tall for the window scrolls instead of running off it", () => {
  const r = rule(".menu > .menu-items");
  assert.match(r, /max-height:/);
  assert.match(r, /overflow-y:\s*auto/);
  // And the score behind it does not scroll on when the menu reaches its
  // end, which is what makes a scrolling menu feel like a stuck one.
  assert.match(r, /overscroll-behavior:\s*contain/);
});

// The room it is held to has to leave the bar above it out, or the panel is
// as tall as the window and its foot is off the bottom by the height of the
// bar — which is the bug, with an extra step.
test("the room the menu is held to leaves out the bar above it", () => {
  const r = rule(".menu > .menu-items");
  const m = /max-height:\s*calc\(100vh\s*-\s*(\d+)px\)/.exec(r);
  assert.ok(m, "max-height is not measured from the window height: " + r);
  const top = Number(/top:\s*(\d+)px/.exec(rule(".menu-items"))[1]);
  assert.ok(Number(m[1]) >= top,
    `the panel starts ${top}px down and is allowed ` +
    `100vh - ${m[1]}px, so it hangs over by ${top - Number(m[1])}px`);
});

// The panel the submenus live in is the one that scrolls, so the submenus
// have to be positioned against the window rather than against it.
test("a flyout is fixed to the window, not hung off the scrolling panel", () => {
  assert.match(rule(".submenu > .menu-items"), /position:\s*fixed/);
});

// A fixed panel ignores left/top meant for an absolute one, so leaving the
// old ones behind would put every flyout in the corner of the window.
test("the old absolute placement is gone from the stylesheet", () => {
  const r = rule(".submenu > .menu-items");
  assert.doesNotMatch(r, /left:\s*100%/,
    "left: 100% means 100% of the window for a fixed panel");
  assert.doesNotMatch(r, /margin-left:/);
  assert.doesNotMatch(css, /\.submenu\.flip > \.menu-items\s*\{[^}]*right:\s*100%/,
    "flipping by right: 100% does not work on a fixed panel either");
});

// Which leaves script to place it, both ways and in both directions.
test("script puts the flyout somewhere", () => {
  const body = src.match(/^function placeSubmenu\([\s\S]*?^}/m)[0];
  assert.match(body, /items\.style\.left =/);
  assert.match(body, /items\.style\.top =/);
  // Measured from the entry that owns it, which is what "level with it"
  // and "beside it" both mean.
  assert.match(body, /sub\.getBoundingClientRect\(\)/);
  // Kept inside the window on all four sides.
  assert.match(body, /window\.innerWidth/);
  assert.match(body, /window\.innerHeight/);
  assert.match(body, /Math\.max\(8/);
});

test("a flyout that will not fit on the right goes to the left", () => {
  const body = src.match(/^function placeSubmenu\([\s\S]*?^}/m)[0];
  assert.match(body, /classList\.add\("flip"\)/);
});

// The entry it is level with moves when the menu scrolls, and a fixed panel
// does not move with it.
test("the flyout is moved back level when the menu is scrolled", () => {
  const body = src.match(/^function setupMenus\([\s\S]*?^}/m)[0];
  assert.match(body, /addEventListener\("scroll"/);
  assert.match(body, /placeSubmenu\(open\)/);
});

// The menu this was all for. If it ever gets short enough to fit on its
// own, the rules above stop being load-bearing and this test says so
// rather than leaving them to be wondered about.
test("the Insert menu is still the long one", () => {
  const block = /<div class="menu" data-menu="insert"[\s\S]*?(?=<div class="menu"|<\/nav>)/
    .exec(html)[0];
  const rows = (block.match(/<button data-action=/g) || []).length +
               (block.match(/class="submenu-title"/g) || []).length;
  assert.ok(rows >= 20,
    `the Insert menu is down to ${rows} rows; it may now fit on a short ` +
    `window, in which case the scrolling above is no longer what keeps ` +
    `Symbol reachable`);
});

// Symbol is the entry that was being lost, so it is named here.
test("Symbol is the last entry of the Insert menu", () => {
  const block = /<div class="menu" data-menu="insert"[\s\S]*?(?=<div class="menu"|<\/nav>)/
    .exec(html)[0];
  const titles = [...block.matchAll(/class="submenu-title"><span>([^<]+)</g)]
    .map((m) => m[1]);
  assert.equal(titles[titles.length - 1], "Symbol");
});
