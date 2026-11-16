/*
 * chat_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

/*
 * The chat prompt.
 *
 * A panel beside the score, toggled from Insert > Chat prompt, that asks a
 * language model for bars. None of it is shown unless the server says the
 * assistant exists, which it does only when gscore was started with
 * AI_ASSISTANT=1.
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

test("the chat prompt is in the Insert menu and hidden until the assistant exists", () => {
  const item = html.match(/<button data-action="chat-prompt"[^>]*>/);
  assert.ok(item, "no Chat prompt item in the Insert menu");
  assert.ok(item[0].includes("data-needs-assistant"), "the item is not tied to the assistant");
  assert.ok(item[0].includes("hidden"), "the item is shown before the server has said");
  const insert = html.indexOf('data-menu="insert"');
  assert.ok(html.indexOf('data-action="chat-prompt"') > insert, "the item is not under Insert");
  assert.ok(html.includes('<aside id="chat" data-needs-assistant hidden>'), "no chat panel");
  assert.ok(html.includes('id="setting-assistant" data-needs-assistant hidden'),
    "the settings section is not tied to the assistant");
  assert.ok(src.includes('$$("[data-needs-assistant]").forEach((el) => (el.hidden = !i.aiAssistant))'),
    "loadInfo does not show or hide the assistant's things");
});

test("the settings offer the four services and ask for the models", () => {
  for (const v of ["openai", "anthropic", "mistral", "local"]) {
    assert.ok(html.includes(`<option value="${v}">`), "no " + v + " choice");
  }
  assert.ok(src.includes('fetch("/api/assistant/models?"'), "the models are not asked for");
  assert.ok(src.includes('fetch("/api/assistant/settings?"'), "the settings are not saved");
});

test("toggling the chat opens the panel, ticks the menu, and puts the cursor in the box", () => {
  const classes = new Set();
  const tick = { textContent: "" };
  let focused = 0;
  const flip = (c) => {
    if (classes.has(c)) classes.delete(c); else classes.add(c);
    return classes.has(c);
  };
  const document = { body: { classList: { toggle: flip } } };
  const $ = (sel) => ({ "#chat-tick": tick, "#chat-prompt": { focus: () => focused++ } })[sel];
  const { toggleChat } = new Function("$", "document", grab("toggleChat") + "\nreturn { toggleChat };")($, document);
  toggleChat();
  assert.ok(classes.has("chat"));
  assert.equal(tick.textContent, "✓");
  assert.equal(focused, 1);
  toggleChat();
  assert.ok(!classes.has("chat"));
  assert.equal(tick.textContent, "");
});

// A box, as chatRecall sees it: the words and where the cursor is.
function box(value) {
  return { value, selectionStart: value.length, selectionEnd: value.length };
}

// chatRecall with a history of its own.
function recaller(history) {
  const body = "let chatHistory = " + JSON.stringify(history) +
    "; let chatAt = chatHistory.length;\n" + grab("chatRecall") +
    "\nreturn { chatRecall, at: () => chatAt };";
  return new Function(body)();
}

test("the arrows bring back what was asked, newest first, and down past it empties the box", () => {
  const { chatRecall } = recaller(["one bar", "two bars"]);
  const b = box("");
  assert.ok(chatRecall(b, -1));
  assert.equal(b.value, "two bars");
  assert.equal(b.selectionStart, "two bars".length, "the cursor is at the end, ready to send");
  assert.ok(chatRecall(b, -1));
  assert.equal(b.value, "one bar");
  assert.ok(!chatRecall(b, -1), "there is nothing before the first");
  assert.equal(b.value, "one bar");
  chatRecall(b, 1);
  chatRecall(b, 1);
  assert.equal(b.value, "", "down past the latest is the empty box");
});

test("the arrows still move within a request of several lines", () => {
  const { chatRecall } = recaller(["one bar"]);
  const b = { value: "first line\nsecond", selectionStart: 15, selectionEnd: 15 };
  assert.ok(!chatRecall(b, -1), "up from the second line is a cursor move, not history");
  assert.equal(b.value, "first line\nsecond");
  const top = { value: "first line\nsecond", selectionStart: 2, selectionEnd: 2 };
  assert.ok(!chatRecall(top, 1), "down from the first line is a cursor move too");
});

test("the box is cleared when a request is sent and the request is kept", () => {
  const ask = grab("askAssistant");
  assert.ok(ask.includes("rememberAsked(prompt);"), "the request is not remembered");
  assert.ok(ask.indexOf('box.value = "";') < ask.indexOf("await fetch("),
    "the box is not cleared until the answer comes");
  assert.ok(ask.includes("for await (const line of readLines(res.body))"),
    "the answer is not read as it streams");
  assert.ok(ask.includes("void refreshScore();"), "the page is not redrawn as bars land");
});

test("the panel is a column of its own only while it is open", () => {
  assert.ok(css.includes("body.chat #chat { display: flex; }"));
  assert.ok(/body\.chat #main \{[\s\S]*?grid-template-columns:[\s\S]*?300px/.test(css),
    "the open chat does not get a column");
});
