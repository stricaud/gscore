/*
 * busy_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Saying that the program is working.
 *
 * Opening a big score takes seconds — Beethoven's ninth is fifty-three
 * staves of two and a half thousand bars — and for those seconds the window
 * did nothing and said nothing, which looks exactly like a window that has
 * stopped working. Gilbert comes up over it now.
 *
 * Three things have to hold. He must not appear for a score that opens
 * instantly, because a mascot that flashes for one frame reads as a glitch.
 * He must go away when the work is done. And he must go away when the work
 * fails, because a mascot left over a window that has given up is worse than
 * none at all.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");
const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// The two functions with a clock that only moves when the test moves it, and
// an overlay that is nothing but a hidden flag.
function load() {
  const overlay = { hidden: true };
  const text = { textContent: "" };
  let pending = null;
  const scope = {
    $: (sel) => (sel === "#busy" ? overlay : sel === "#busy-text" ? text : null),
    setTimeout: (fn) => { pending = fn; return 1; },
    clearTimeout: () => { pending = null; },
  };
  const body = "let busyDepth = 0; let busyTimer = 0;\n" +
    grab("busy") + "\n" + grab("idle") +
    "\nreturn { busy, idle };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  return {
    ...api,
    overlay,
    text,
    // What the browser does when the wait has gone on long enough.
    tick: () => { const fn = pending; pending = null; if (fn) fn(); },
  };
}

test("a score that opens at once never shows him", () => {
  const t = load();
  t.busy("Opening…");
  t.idle();
  t.tick(); // whatever was left of the timer fires into nothing
  assert.equal(t.overlay.hidden, true,
    "Gilbert appeared for a score that was already open");
});

test("a score that takes a moment shows him, and he goes when it is done", () => {
  const t = load();
  t.busy("Opening Symphony No. 9…");
  assert.equal(t.overlay.hidden, true, "he appeared before the wait was a wait");
  t.tick();
  assert.equal(t.overlay.hidden, false, "the wait went on and nothing was said");
  assert.ok(t.text.textContent.includes("Symphony No. 9"),
    "he came up without saying what for: " + JSON.stringify(t.text.textContent));
  t.idle();
  assert.equal(t.overlay.hidden, true, "he stayed over the finished score");
});

test("one slow thing inside another leaves him up until both are done", () => {
  const t = load();
  t.busy("Opening…");
  t.tick();
  t.busy("Laying it out…");
  t.idle();
  assert.equal(t.overlay.hidden, false,
    "the inner piece of work finishing took him away from the outer one");
  t.idle();
  assert.equal(t.overlay.hidden, true);
});

test("the open paths put him away whatever happens", () => {
  // A file that cannot be read returns early, and a fetch that throws does
  // not return at all. Both have to leave the window usable, which is what
  // finally is for.
  for (const name of ["openFile", "openRecent", "reload"]) {
    const fn = grab(name);
    assert.ok(fn.includes("busy("), name + " never says it is working");
    assert.ok(/finally\s*\{[\s\S]*?idle\(\)/.test(fn),
      name + " puts Gilbert away on the way out rather than in a finally, " +
      "so a failed open leaves him over the window");
  }
});

test("a score opened from the command line gets him too", () => {
  // The window is started with a file on the command line, so the server
  // has the score before the page exists and nothing in the page ever calls
  // openFile. What it does call is reload, which is where the waiting is.
  const fn = grab("reload");
  assert.ok(fn.includes("busy("),
    "reload says nothing while it reads, lays out and draws the score");
});

test("the page has him and the stylesheet puts him over it", () => {
  assert.ok(/id="busy"/.test(html), "index.html has no busy overlay");
  assert.ok(/id="busy-logo"[^>]*gilbert\.svg|gilbert\.svg[^>]*id="busy-logo"/.test(html) ||
    /<img id="busy-logo"[\s\S]*?gilbert\.svg/.test(html),
    "the overlay does not show Gilbert");
  assert.ok(/#busy\s*\{[\s\S]*?position:\s*fixed/.test(css),
    "the overlay is not over the window");
  assert.ok(/prefers-reduced-motion[\s\S]*?#busy-logo\s*\{\s*animation:\s*none/.test(css),
    "the mascot keeps moving for somebody who asked not to be moved about");
});

/*
 * And where a score opens.
 *
 * It used to open at the last bar, on the reasoning that a piece somebody
 * opens to add to is a piece they want to carry on writing. That is true of
 * a twelve-bar tune and wrong about everything else: open a symphony and
 * the page is at bar two thousand six hundred, which is nowhere anybody
 * asked to be. A score opens at its beginning, like any other document, and
 * the sprints are how somebody asks for the end of a staff.
 */
test("there is one sprint per staff, and it says which staff", () => {
  const fn = src.match(/^function drawSprints\(\)[\s\S]*?^}/m)[0];
  assert.ok(fn.includes("sprintRows("), "the sprints are not built from the staves");
  assert.ok(fn.includes("Continue editing "),
    "a sprint does not say what it is for");
  assert.ok(/state\.mode !== "strip"/.test(fn),
    "sprints are offered on the paginated view, where there is no end to run to");
  assert.ok(/clientWidth \* 1\.5/.test(fn),
    "a score whose end is already on screen still gets a row of buttons");
  // They are a way of leaving the start, so once the view has moved along
  // they are pointing at where you already are and are in front of the
  // music.
  assert.ok(/scrollLeft <= \d+/.test(fn),
    "the sprints stay on the page after the view has moved along the staff");
});

test("play says it is getting ready, and lets go before the first sound", () => {
  const fn = src.match(/^async function play\(opts\)[\s\S]*?^}/m)[0];
  assert.ok(fn.includes("busy("), "nothing is said while a symphony is prepared");
  assert.ok(/finally\s*\{[\s\S]*?idle\(\)/.test(fn),
    "Gilbert is left over the music once it starts playing");
  assert.ok(fn.includes("playNow("),
    "the preparation is not wrapped, so an early return strands the mascot");
});

test("a sprint goes to the last bar of its own staff and leaves the cursor in it", () => {
  const fn = src.match(/^async function sprintToStaffEnd\([\s\S]*?^}/m)[0];
  assert.ok(/h\.part !== part/.test(fn), "it does not look at one staff in particular");
  assert.ok(/h\.measure > last\.measure/.test(fn), "it does not find the last bar");
  assert.ok(/at: \(last\.elements \|\| \[\]\)\.length/.test(fn),
    "the cursor does not land after the last thing written");
  assert.ok(fn.includes("glideTo("), "it jumps rather than travels");
  assert.ok(fn.includes("flashMeasure("), "it arrives without saying where");
});
