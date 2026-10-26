/*
 * drop_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Dropping a score on the page.
 *
 * What a browser does with a file dropped on a page that has not asked for
 * it is open the file. The editor asked for it on the canvas and nowhere
 * else, so a score dropped on the toolbar, on the menus or in the margin
 * took the browser away to the file — and from a page served over https it
 * then refused the file:// it had just been sent to: "Security Error:
 * Content at https://stricaud.github.io/gscore/app/ may not load or link to
 * file:///". Reported from Firefox, opening a .musx.
 *
 * So the window takes the drop, and there is nowhere on the page that is not
 * handled.
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

// The editor's own setup, with a page that notes where every listener went.
function load() {
  const on = { window: {}, element: {} };
  const element = {
    addEventListener(kind, fn) { (on.element[kind] ||= []).push(fn); },
  };
  const win = {
    addEventListener(kind, fn) { (on.window[kind] ||= []).push(fn); },
  };
  const opened = [];
  const scope = {
    $: () => element,
    window: win,
    freeTheFilePanels() {},
    async openFile(file) { opened.push(file); return true; },
  };
  const body = grab("setupFileInput") + "\nreturn { setupFileInput };";
  const api = new Function(...Object.keys(scope), body)(...Object.values(scope));
  api.setupFileInput();
  return { on, opened };
}

// A drop event as a browser hands it over, noting whether the page took it.
function dropOf(files) {
  let prevented = false;
  return {
    event: {
      dataTransfer: { files },
      preventDefault() { prevented = true; },
    },
    wasPrevented: () => prevented,
  };
}

test("the whole window takes the drop, not one corner of the page", () => {
  const { on } = load();
  assert.ok(on.window.drop?.length,
            "nothing listens for a drop on the window, so a score dropped " +
            "beside the music is followed by the browser");
  assert.ok(on.window.dragover?.length,
            "without dragover the drop never arrives at all");
});

test("a score dropped on the page opens instead of being followed", async () => {
  const { on, opened } = load();
  const file = { name: "Sanctus - Rossini.musx" };
  const d = dropOf([file]);

  await on.window.drop[0](d.event);

  assert.ok(d.wasPrevented(),
            "the drop was not taken, so the browser goes to the file itself");
  assert.deepEqual(opened, [file]);
});

test("the browser is still stopped when what was dropped is not a file", async () => {
  // Dragging a selection, a link or an image gives a drop with no files in
  // it. Opening nothing is right; letting the browser have it is not.
  const { on, opened } = load();
  const d = dropOf([]);

  await on.window.drop[0](d.event);

  assert.ok(d.wasPrevented(), "an empty drop was handed back to the browser");
  assert.deepEqual(opened, []);
});

test("a drop carrying nothing at all is survived", async () => {
  const { on, opened } = load();
  let prevented = false;
  await on.window.drop[0]({ preventDefault() { prevented = true; } });
  assert.ok(prevented);
  assert.deepEqual(opened, []);
});

test("dragover lets the page keep the file", () => {
  const { on } = load();
  let prevented = false;
  on.window.dragover[0]({ preventDefault() { prevented = true; } });
  assert.ok(prevented, "the page did not claim the drag, so no drop follows");
});
