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
 * accept is a list of extensions, and iOS reads it by turning each one into
 * a type it knows. It knows none of gscore's: .mscz, .gscore, .abc, .nwctxt
 * and .sf2 are nobody's registered type, so the Files panel opened on an
 * iPhone with every score greyed out. Reported as "I want to open a
 * musescore4 score from my phone and I am not allowed to select it".
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
  const body = grab("onAppleTouch") + "\n" + grab("freeTheFilePanels") +
    "\nreturn { onAppleTouch, freeTheFilePanels };";
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

test("an iPad is an iPhone here, whatever it calls itself", () => {
  const { inputs, onAppleTouch, freeTheFilePanels } = load(IPADOS);
  assert.ok(onAppleTouch(), "an iPad reporting itself as a Macintosh was missed");
  freeTheFilePanels();
  assert.equal(inputs[0].accept, undefined);
});

test("a desktop keeps its filter, which is useful there", () => {
  const { inputs, onAppleTouch, freeTheFilePanels } = load(MAC);
  assert.equal(onAppleTouch(), false);
  freeTheFilePanels();
  assert.ok(inputs[0].accept.includes(".mscz"));
  assert.ok(inputs[1].accept.includes(".sf2"));
});

test("a browser that says nothing at all is left alone", () => {
  const { inputs, freeTheFilePanels } = load({});
  freeTheFilePanels();
  assert.ok(inputs[0].accept.includes(".gscore"));
});
