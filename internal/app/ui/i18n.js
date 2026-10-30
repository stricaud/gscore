/*
 * i18n.js
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The interface in somebody's own language.
 *
 * The key is the English, not an invented name. Five hundred keys of the
 * shape "menu.edit.pitch.up" are five hundred things to keep in step with
 * the words they stand for, and the first one that drifts is a menu entry
 * reading MENU.EDIT.PITCH.UP. With the English as the key there is nothing
 * to keep in step: a string with no translation comes back as itself, so a
 * half-finished language is an interface half in French rather than a page
 * of broken keys.
 *
 * The markup is not marked up. Every visible word is already in index.html,
 * and putting a data-i18n on each of them would be the same five hundred
 * names by another road. Instead the page is walked once when it loads and
 * every piece of text in it is looked up — which also means a word added to
 * a menu is translatable the moment somebody writes the French for it, with
 * nothing else to remember.
 */

// LANGUAGES is what can be chosen, by code, with the name each is called in
// itself. A language is named in its own tongue wherever it is offered, so
// that somebody looking for their own finds it without reading English.
const LANGUAGES = [
  { code: "en", name: "English" },
  { code: "fr", name: "Français" },
];

// MESSAGES is the English of the interface against its translations.
//
// Written as one table per language rather than one entry per string: the
// person writing a translation works down a list in their own language,
// which is how a translation is actually done.
const MESSAGES = { fr: {} };

// lang is the language in force. "en" is not a translation but the absence
// of one, so it has no table.
let lang = "en";

// t is one string in the language in force.
//
// Anything with no translation comes back as it went in. That is the whole
// fallback: an interface in two languages at once is ugly and perfectly
// usable, and it is what lets a translation be written a hundred words at a
// time instead of all at once.
function t(s) {
  if (typeof s !== "string" || !s) return s;
  const table = MESSAGES[lang];
  if (!table) return s;
  const got = table[s];
  return got === undefined ? s : got;
}

// currentLanguage is the code in force, and languageChoice what was asked
// for — "auto" where nobody has asked for anything and the browser decides.
function currentLanguage() { return lang; }
let chosenLanguage = "auto";
function languageChoice() { return chosenLanguage; }

// browserLanguage is the language the browser asks for, where it is one
// gscore has.
//
// The first of the list it understands, because navigator.languages is in
// the order somebody put their languages in and the first match is the one
// they would rather read. "fr-CA" is French: the region says which French
// and gscore has only the one.
function browserLanguage(nav) {
  const list = (nav && (nav.languages || (nav.language ? [nav.language] : []))) || [];
  for (const want of list) {
    const code = String(want).toLowerCase().split("-")[0];
    if (LANGUAGES.some((l) => l.code === code)) return code;
  }
  return "en";
}

// setLanguage puts a language in force. "auto" means the browser's.
//
// It returns the code that ended up in force, which is not always the one
// asked for: a language gscore does not have falls back to English rather
// than leaving the interface in whatever was there before.
function setLanguage(choice, nav) {
  chosenLanguage = choice || "auto";
  const want = chosenLanguage === "auto"
    ? browserLanguage(nav || (typeof navigator === "undefined" ? null : navigator))
    : String(choice).toLowerCase().split("-")[0];
  lang = LANGUAGES.some((l) => l.code === want) ? want : "en";
  return lang;
}

// SKIP are the elements whose text is not prose.
//
// A keystroke is a keystroke in every language — ⌘Z is ⌘Z — and the score
// itself is music, which is the one thing that needs no translating at all.
const SKIP = new Set(["SCRIPT", "STYLE", "KBD", "SVG", "TEXTAREA", "CODE"]);

// TRANSLATED_ATTRS are the attributes a reader sees.
const TRANSLATED_ATTRS = ["placeholder", "title", "aria-label", "label"];

// translateDOM puts the page into the language in force.
//
// Every text node under the root, and the attributes a reader sees. It is
// run once when the page loads and again when the language is changed; the
// original English is kept on the node the first time, so changing language
// twice does not translate a translation.
function translateDOM(root, doc) {
  const d = doc || (typeof document === "undefined" ? null : document);
  if (!root || !d) return;

  const walk = (node) => {
    if (node.nodeType === 3) { // a piece of text
      const was = node.gscoreEnglish !== undefined
        ? node.gscoreEnglish : node.nodeValue;
      const trimmed = String(was).trim();
      if (!trimmed) return;
      node.gscoreEnglish = was;
      node.nodeValue = String(was).replace(trimmed, t(trimmed));
      return;
    }
    if (node.nodeType !== 1) return;
    if (SKIP.has(node.tagName) || node.hasAttribute("data-no-i18n")) return;
    for (const name of TRANSLATED_ATTRS) {
      if (!node.hasAttribute(name)) continue;
      const key = "gscoreEnglish:" + name;
      const was = node[key] !== undefined ? node[key] : node.getAttribute(name);
      node[key] = was;
      node.setAttribute(name, t(was));
    }
    for (const child of Array.from(node.childNodes)) walk(child);
  };
  walk(root);
}
