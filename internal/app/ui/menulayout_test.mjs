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
// have to be positioned against the window rather than against it — and,
// while open, taken out of it altogether.
test("a flyout is fixed to the window, not hung off the scrolling panel", () => {
  assert.match(rule(".submenu > .menu-items,\n.menu-items.flyout"),
    /position:\s*fixed/);
});

/*
 * Fixed is not enough on its own, which took a long time to find.
 *
 * WebKit clips a position: fixed element inside a scrolling box away to
 * nothing, and nothing about it reads as clipping: the panel is open, it
 * is display: block, it is at the right place on the page, and
 * elementFromPoint at its middle answers with the panel. Every way of
 * asking says it is there and the screen shows nothing. So an open flyout
 * is parked on the body, where there is no scrolling ancestor to be
 * clipped by, in any engine.
 *
 * Reported as "I cannot open the submenus", and narrowed by the person
 * reporting it to "only edit and insert" — which is the list of menus
 * long enough to scroll.
 */
test("an open flyout is taken out of the scrolling menu", () => {
  const body = grab("openSubmenu");
  assert.match(body, /document\.body\.appendChild\(panel\)/);
  assert.match(body, /classList\.add\("flyout"\)/);
  // And it shows because it is parked, not because of where it sits.
  assert.match(css, /\.menu-items\.flyout \{ display: block; \}/);
  assert.doesNotMatch(css, /\.submenu\.open > \.menu-items/,
    "a rule that only matches a panel still inside its menu");
});

test("a flyout put back is a flyout no longer", () => {
  const body = grab("closeSubmenu");
  assert.match(body, /classList\.remove\("flyout"\)/);
  assert.match(body, /sub\.appendChild\(panel\)/);
  // Everything that shuts one goes through it, or a panel is left on the
  // body with nothing to put it back.
  const src2 = src.replace(grab("closeSubmenu"), "");
  assert.doesNotMatch(src2, /classList\.remove\("open", "flip"\)/,
    "something closes a submenu without putting its panel back");
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

/* ---------- reaching a submenu that was pulled up the window ---------- */

/*
 * Fixing the clipping moved the panels, and moving them broke getting to
 * them. A submenu too tall for the room under its entry is pulled up the
 * window to fit, so its panel ends up ABOVE the entry that opened it — and
 * the line a hand takes from the entry to the panel runs back across three
 * or four other entries of the same menu.
 *
 * Every one of those entries had something to say about it. A plain one
 * closed the open submenu; another submenu's title opened its own, which
 * closes the open one. So the panel appeared and was gone again before the
 * pointer arrived. Reported as "I cannot open the submenus", and then,
 * precisely: Staff and View open, none in Insert, Edit also does not work —
 * which is the list of menus short enough not to need pulling up, against
 * the two that are not.
 *
 * Both halves wait now, which is how every menu bar tells a pointer
 * crossing an entry from a pointer choosing it.
 */

const grab = (name) => {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

test("crossing an entry does not shut a submenu at once", () => {
  assert.match(src, /b\.addEventListener\("mouseenter", closeSubmenusSoon\)/,
    "a plain entry still closes the submenu the moment it is crossed");
  assert.match(grab("closeSubmenusSoon"), /setTimeout/);
  assert.match(grab("keepSubmenus"), /clearTimeout/);
});

test("crossing a submenu title does not open it at once", () => {
  assert.match(grab("openSubmenuSoon"), /setTimeout/);
  assert.match(grab("cancelSubmenuOpen"), /clearTimeout/);
  // The title's own listeners: open on settling, call it off on moving on.
  const setup = grab("setupMenus");
  assert.match(setup, /"mouseenter", \(\) => \{\s*openSubmenuSoon\(sub\);/);
  assert.match(setup, /"mouseleave",\s*cancelSubmenuOpen\)/);
});

// Arriving is the signal that the journey was a journey.
test("reaching the panel calls off both", () => {
  const setup = grab("setupMenus");
  const panel = setup.slice(setup.indexOf("panelOf(sub)?.addEventListener"));
  assert.match(panel, /cancelSubmenuOpen\(\);/);
  assert.match(panel, /keepSubmenus\(\);/);
});

// Long enough to cross three entries without hurrying, short enough that a
// menu left behind does not sit there.
test("the waits are of a length a hand can use", () => {
  const grace = Number(/const submenuGrace = (\d+)/.exec(src)[1]);
  const hover = Number(/const submenuHoverDelay = (\d+)/.exec(src)[1]);
  assert.ok(grace >= 250 && grace <= 800, `the grace is ${grace}ms`);
  assert.ok(hover >= 100 && hover <= 300, `the hover wait is ${hover}ms`);
  assert.ok(hover < grace,
    "opening must settle sooner than closing, or crossing a title shuts " +
    "the panel being reached for and nothing takes its place");
});

/*
 * And a menu does not take the focus.
 *
 * Pressing a button focuses it, and focusing something inside a box that
 * scrolls makes the box scroll it into view. A menu too tall for the window
 * is such a box, so the press moved the entry out from under the finger,
 * the release landed on whatever had taken its place, and a click is only
 * delivered when the two are the same thing. The entry did nothing at all —
 * in the Edit and Insert menus, which are the two long enough to scroll,
 * and nowhere else.
 */

test("pressing a menu entry does not focus it", () => {
  const setup = grab("setupMenus");
  assert.match(setup, /const dontFocus = \(e\) => \{[\s\S]*?preventDefault\(\)/);
  assert.match(setup, /"mousedown", dontFocus/);
});

// preventDefault on a pointerdown may suppress the mouse events that
// follow it, the click among them — which would be this bug again by
// another route.
test("the focus is held off on mousedown, not on pointerdown", () => {
  const setup = grab("setupMenus");
  assert.doesNotMatch(setup, /"pointerdown"/,
    "a pointerdown handler here can swallow the click it is meant to save");
});

// The press is the half of the gesture that is certainly delivered.
test("a submenu opens on the press rather than on the click", () => {
  const setup = grab("setupMenus");
  const at = setup.indexOf('title.addEventListener("mousedown"');
  assert.ok(at > 0, "the submenu title has no mousedown handler");
  const press = setup.slice(at, setup.indexOf("});", at));
  assert.match(press, /openSubmenu\(sub\)/);
  assert.match(press, /e\.button !== 0/, "any button of the mouse opens it");
  // And the click that follows the press must not undo it.
  assert.match(setup, /if \(justPressed\)/);
});
