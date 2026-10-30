/*
 * i18n_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The interface in somebody's own language.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./i18n.js", import.meta.url), "utf8");
const load = () =>
  new Function(src + "\nreturn { t, setLanguage, currentLanguage, " +
    "languageChoice, browserLanguage, translateDOM, LANGUAGES, MESSAGES };")();

test("a string with no translation comes back as itself", () => {
  const i = load();
  i.setLanguage("fr", { languages: [] });
  assert.equal(i.t("Édition absente du dictionnaire"),
               "Édition absente du dictionnaire");
  // And the whole of English is untranslated by definition.
  i.setLanguage("en", { languages: [] });
  assert.equal(i.t("Staccato"), "Staccato");
  assert.equal(i.t("Quarter"), "Quarter");
});

test("French is French", () => {
  const i = load();
  i.setLanguage("fr", { languages: [] });
  assert.equal(i.t("Quarter"), "Noire");
  assert.equal(i.t("Whole rest"), "Pause");
  assert.equal(i.t("Sharp"), "Dièse");
  assert.equal(i.t("Treble"), "Clé de sol");
  assert.equal(i.t("Triplet"), "Triolet");
  assert.equal(i.t("Key signature…"), "Armature…");
});

// The browser's own list, in the order somebody put their languages in.
test("the browser decides when nobody has", () => {
  const i = load();
  assert.equal(i.browserLanguage({ languages: ["fr-CA", "en-US"] }), "fr",
               "a region says which French, and there is only one");
  assert.equal(i.browserLanguage({ languages: ["en-GB", "fr"] }), "en",
               "the first one understood wins");
  assert.equal(i.browserLanguage({ languages: ["de", "it"] }), "en",
               "a language gscore has not falls back to English");
  assert.equal(i.browserLanguage({ language: "fr" }), "fr",
               "an older browser with one language");
  assert.equal(i.browserLanguage({}), "en");
  assert.equal(i.browserLanguage(null), "en");
});

test("asking for a language outright beats the browser", () => {
  const i = load();
  assert.equal(i.setLanguage("fr", { languages: ["en"] }), "fr");
  assert.equal(i.currentLanguage(), "fr");
  assert.equal(i.languageChoice(), "fr");

  assert.equal(i.setLanguage("auto", { languages: ["fr"] }), "fr");
  assert.equal(i.languageChoice(), "auto", "auto is remembered as auto");

  // One gscore does not have falls back rather than leaving the interface
  // in whatever was there before.
  assert.equal(i.setLanguage("de", { languages: ["fr"] }), "en");
});

// The page is walked once and every piece of text in it is looked up.
test("the page is put into the language", () => {
  const i = load();
  i.setLanguage("fr", { languages: [] });

  const text = (s) => ({ nodeType: 3, nodeValue: s });
  const el = (tag, kids, attrs) => ({
    nodeType: 1, tagName: tag, childNodes: kids || [],
    attrs: attrs || {},
    hasAttribute(n) { return n in this.attrs; },
    getAttribute(n) { return this.attrs[n]; },
    setAttribute(n, v) { this.attrs[n] = v; },
  });

  const label = text("Staccato");
  const keycap = text("⌘Z");
  const notehead = text("");
  const field = el("INPUT", [], { placeholder: "Type to search…" });
  const body = el("DIV", [
    el("BUTTON", [label]),
    el("KBD", [keycap]),
    el("svg", [el("text", [notehead])]),
    field,
  ]);

  i.translateDOM(body, {});
  assert.equal(label.nodeValue, "Staccato");          // same word in French
  assert.equal(field.attrs.placeholder, "Saisir pour rechercher…");
  assert.equal(keycap.nodeValue, "⌘Z", "a keystroke is a keystroke");
  assert.equal(notehead.nodeValue, "", "the music is not translated");

  const quarter = text("Quarter");
  i.translateDOM(el("DIV", [quarter]), {});
  assert.equal(quarter.nodeValue, "Noire");
});

// Changing language twice must not translate a translation.
test("the English is kept, so the language can be changed back", () => {
  const i = load();
  const node = { nodeType: 3, nodeValue: "Quarter" };
  const el = (kids) => ({
    nodeType: 1, tagName: "DIV", childNodes: kids, attrs: {},
    hasAttribute() { return false; },
    getAttribute() { return null; },
    setAttribute() {},
  });

  i.setLanguage("fr", { languages: [] });
  i.translateDOM(el([node]), {});
  assert.equal(node.nodeValue, "Noire");

  i.setLanguage("en", { languages: [] });
  i.translateDOM(el([node]), {});
  assert.equal(node.nodeValue, "Quarter", "the English was lost on the way");
});

// The space around a word is the page's layout, not the word.
test("the spacing around a word survives being translated", () => {
  const i = load();
  i.setLanguage("fr", { languages: [] });
  const node = { nodeType: 3, nodeValue: "\n      Quarter\n    " };
  const el = (kids) => ({
    nodeType: 1, tagName: "DIV", childNodes: kids, attrs: {},
    hasAttribute() { return false; },
    getAttribute() { return null; },
    setAttribute() {},
  });
  i.translateDOM(el([node]), {});
  assert.equal(node.nodeValue, "\n      Noire\n    ");
});

test("every language offered has a name in its own tongue", () => {
  const i = load();
  assert.ok(i.LANGUAGES.length >= 2);
  for (const l of i.LANGUAGES) {
    assert.match(l.code, /^[a-z]{2}$/, JSON.stringify(l));
    assert.ok(l.name && l.name.length > 1, JSON.stringify(l));
  }
  assert.ok(i.LANGUAGES.some((l) => l.code === "fr" && l.name === "Français"));
});
