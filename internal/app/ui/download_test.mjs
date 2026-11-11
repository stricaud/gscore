/*
 * download_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The bar a soundfont arrives behind.
 *
 * Thirty to two hundred megabytes, which on an ordinary line is between ten
 * seconds and five minutes. A sentence reading "downloading the soundfont…"
 * that has not changed in four minutes is indistinguishable from a program
 * that has stopped, so this says how much has arrived, how fast, and how
 * long is left.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const html = readFileSync(new URL("./index.html", import.meta.url), "utf8");

function grab(name) {
  const m = src.match(
    new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
}

// The dialog's two drawing functions, run against a stub of the page.
function run(answer) {
  const bar = { value: null, removed: false,
    removeAttribute() { this.removed = true; this.value = null; } };
  const numbers = { className: "", textContent: "" };
  const nodes = { "#sfdownload-bar": bar, "#sfdownload-numbers": numbers };
  const body = grab("drawDownload") + "\n" + grab("timeLeft") + "\n"
    + grab("megabytes");
  const make = new Function("$",
    body + "\nreturn { drawDownload, timeLeft };");
  const api = make((sel) => nodes[sel]);
  api.drawDownload(answer);
  return { bar, numbers, timeLeft: api.timeLeft };
}

test("it says how much has arrived, how fast, and how long is left", () => {
  const { bar, numbers } = run({
    received: 73400320, total: 215614036,
    bytesPerSecond: 2650000, secondsLeft: 53.6,
  });
  assert.ok(Math.abs(bar.value - 73400320 / 215614036) < 1e-9,
    "the bar is at " + bar.value);
  assert.match(numbers.textContent, /73\.4 MB of 216 MB/);
  assert.match(numbers.textContent, /2\.6 MB\/s/);
  assert.match(numbers.textContent, /54s left/);
});

test("a server that will not say how big it is gets a running bar", () => {
  // A bar that pretends to a fraction it cannot know is worse than one
  // that only says "still going".
  const { bar, numbers } = run({ received: 1048576, total: 0 });
  assert.equal(bar.removed, true, "the bar claimed a fraction of nothing");
  assert.match(numbers.textContent, /1\.0 MB/);
  assert.doesNotMatch(numbers.textContent, /of/,
    "it says what it is a fraction of, and it does not know");
});

test("what went wrong is the one thing a bar cannot show", () => {
  const { numbers } = run({
    received: 10, total: 100, error: "the download answered 404 Not Found",
  });
  assert.equal(numbers.className, "failed");
  assert.match(numbers.textContent, /404/);
});

test("the wait is given in units somebody waiting can use", () => {
  const { timeLeft } = run({ received: 0, total: 0 });
  assert.equal(timeLeft(0.4), "nearly there");
  assert.equal(timeLeft(45), "45s left");
  assert.equal(timeLeft(125), "2m 5s left");
});

test("a finished download says it is being read, not that it is waiting", () => {
  // Reading two hundred megabytes of samples takes a moment of its own,
  // and a bar sitting full with no word is the same silence again.
  const { numbers } = run({ received: 100, total: 100, done: true });
  assert.match(numbers.textContent, /reading it/);
  assert.doesNotMatch(numbers.textContent, /left/,
    "it offers a wait for something that has already arrived");
});

test("the page has a dialog to put it in", () => {
  assert.match(html, /id="sfdownload"/);
  assert.match(html, /id="sfdownload-bar"/);
  assert.match(html, /aria-live="polite"/,
    "the numbers change without focus moving, so they have to be announced");
});
