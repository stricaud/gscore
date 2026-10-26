/*
 * filepanel_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Choosing a file on a phone.
 *
 * accept is a list of extensions, and it was a guess at what gscore reads
 * made in the one place that does not know: the browser. iOS turns it into
 * types the system has heard of and has heard of none of gscore's, so the
 * Files panel opened on an iPhone with every score greyed out. On a desktop
 * it greyed out a Finale file somebody wanted to try. Reported twice: "I want
 * to open a musescore4 score from my phone and I am not allowed to select
 * it", and "I want to open ~/Downloads/....musx and I cannot even choose it".
 *
 * So there is no filter anywhere now, and the importers — which are where
 * the knowledge is — say what they cannot read and what to do about it.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// Four file inputs, as index.html has them, and a browser that says who it
// is.
function load(navigator) {
  const inputs = [
    { accept: ".abc,.mscz,.mscx,.musicxml,.xml,.mxl,.gscore,.nwctxt,.nwc" },
    { accept: ".sf2,.SF2" },
    { accept: ".xpm,.png,.gif,.jpg,.jpeg" },
    { accept: ".gscore" },
  ].map((o) => ({
    ...o,
    removeAttribute(name) { delete this[name]; },
  }));
  const document = {
    querySelectorAll(sel) {
      assert.equal(sel, 'input[type="file"][accept]');
      return inputs.filter((i) => i.accept !== undefined);
    },
  };
  const body = grab("freeTheFilePanels") + "\nreturn { freeTheFilePanels };";
  const api = new Function("navigator", "document", body)(navigator, document);
  return { inputs, ...api };
}

const IPHONE = {
  userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) " +
    "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1",
  maxTouchPoints: 5,
};
const IPADOS = { // calls itself a Macintosh, and has been since iPadOS 13
  userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
    "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
  maxTouchPoints: 5,
};
const MAC = {
  userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
    "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
  maxTouchPoints: 0,
};

test("an iPhone can reach every file in the panel", () => {
  const { inputs, freeTheFilePanels } = load(IPHONE);
  freeTheFilePanels();
  for (const input of inputs) {
    assert.equal(input.accept, undefined,
                 "an accept list is still there, so the file is still greyed out");
  }
});

test("an iPad is the same, whatever it calls itself", () => {
  const { inputs, freeTheFilePanels } = load(IPADOS);
  freeTheFilePanels();
  assert.equal(inputs[0].accept, undefined);
});

test("a desktop can reach every file too", () => {
  // A Finale file in Downloads was greyed out here, which is the same wall
  // in a different place: the browser deciding what gscore can read.
  const { inputs, freeTheFilePanels } = load(MAC);
  freeTheFilePanels();
  for (const input of inputs) {
    assert.equal(input.accept, undefined);
  }
});

test("a browser that says nothing at all is freed as well", () => {
  const { inputs, freeTheFilePanels } = load({});
  freeTheFilePanels();
  assert.equal(inputs[0].accept, undefined);
});
