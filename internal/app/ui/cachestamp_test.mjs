/*
 * cachestamp_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The editor and the core are one program in two files.
 *
 * A browser that has been to the site before serves itself the app.js it
 * kept and fetches the core it has not got, and then the two halves are from
 * different builds: the editor asks the core for something it did not used
 * to do, or does not ask for what it now can. Nothing about that reads as a
 * failure — the page comes up, the menus work, one thing quietly does not —
 * and the only way out is knowing to empty the cache. Reported as "it works,
 * but I have to kill the cache".
 *
 * So the site stamps every script with the version it was built with, and
 * boot.js passes the same stamp on to the WebAssembly, which is the file no
 * page tag names.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const boot = readFileSync(new URL("../../../web/boot.js", import.meta.url), "utf8");

// The one expression that reads the stamp, run against a page that says
// where boot.js came from.
function stampOf(scriptSrc) {
  const m = boot.match(/const stamp = \([\s\S]*?\|\| "";/);
  if (!m) throw new Error("boot.js no longer takes a version stamp from its own URL");
  const document = scriptSrc === null ? {} : { currentScript: { src: scriptSrc } };
  const location = { href: "https://stricaud.github.io/gscore/app/" };
  return new Function("document", "location", "URL", m[0] + "\nreturn stamp;")(
    document, location, URL);
}

test("the core is fetched with the version the page was stamped with", () => {
  assert.equal(stampOf("https://stricaud.github.io/gscore/app/boot.js?v=0.1.6"), "?v=0.1.6");
});

test("a page with no stamp asks for the core plainly", () => {
  // The desktop build serves the interface itself and has no boot.js at all;
  // a site built without a version is simply unstamped. Neither should end
  // up asking for "gscore.wasm?".
  assert.equal(stampOf("https://stricaud.github.io/gscore/app/boot.js"), "");
  assert.equal(stampOf(null), "");
});

test("the stamp goes on the core, which no page tag names", () => {
  assert.ok(boot.includes('realFetch("gscore.wasm" + stamp)'),
            "boot.js fetches the WebAssembly without the stamp, so a kept " +
            "app.js can still be paired with a fresh core");
});

test("the stamp is read while boot.js is running, not later", () => {
  // document.currentScript is only this script while this script is being
  // evaluated; read inside start() it is null and every page is unstamped.
  const top = boot.indexOf("const stamp =");
  const inStart = boot.indexOf("async function start()");
  assert.ok(top > 0 && top < inStart,
            "the stamp is read after start() is defined, where currentScript " +
            "is no longer boot.js");
});
