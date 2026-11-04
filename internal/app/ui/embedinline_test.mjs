/*
 * embedinline_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Music written into a page rather than kept in a file beside it.
 *
 * A file is the wrong shape for eight bars in the middle of a paragraph,
 * so the page carries the tune and a script engraves it where it stands.
 * The two halves have to agree about one thing — the message that hands
 * the music over — and this is what holds them to it.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const src = readFileSync(new URL("./app.js", import.meta.url), "utf8");
const embed = readFileSync(
  new URL("../../../web/site/embed.js", import.meta.url), "utf8");
const docs = readFileSync(
  new URL("../../../docs/embedding.md", import.meta.url), "utf8");
const build = readFileSync(
  new URL("../../../web/build.sh", import.meta.url), "utf8");
const api = readFileSync(
  new URL("../../../docs/openapi.yaml", import.meta.url), "utf8");

const grab = (where, name) => {
  const m = where.match(
    new RegExp("^(?:\\s*)?function " + name + "\\([\\s\\S]*?^  \\}", "m")) ||
    where.match(new RegExp("function " + name + "\\([\\s\\S]*?\\n  \\}", "m"));
  if (!m) throw new Error("no " + name);
  return m[0];
};

test("the editor takes a score as the page hands it over", () => {
  assert.match(src, /case "open": \{/);
  // Text for ABC, bytes for the zipped formats: both are what File takes,
  // and the name picks the reader as it does for a file being opened.
  assert.match(src, /typeof m\.value === "string"/);
  assert.match(src, /new Uint8Array\(m\.value\)/);
  assert.match(src, /new File\(\[what\], String\(m\.name \|\| "tune\.abc"\)\)/);
});

test("the script waits to be told the frame is up", () => {
  // Sending before then sends into a page with no listener, and the tune
  // is silently never read.
  assert.match(embed, /m\.gscore !== "ready"/);
  assert.match(embed, /gscore: "open"/);
  // And only to its own frame: a page with three tunes on it has three
  // listeners waiting at once.
  assert.match(embed, /e\.source !== frame\.contentWindow/);
});

// ABC cares about the left margin, and nobody should have to break the
// shape of their page to write music in it.
test("the block's own indentation comes off", () => {
  const body = grab(embed, "dedent");
  assert.match(body, /Math\.min/);
  assert.match(embed, /dedent\(el\.textContent/);
});

// The class says the program, not the format: a block may hold MusicXML
// or a MuseScore file as readily as ABC.
test("what it looks for is not named after one format", () => {
  assert.match(embed, /".gscore"/);
  assert.doesNotMatch(embed, /abc-source/);
  assert.doesNotMatch(docs, /abc-source/);
});

test("the page says what the music is written in", () => {
  const body = grab(embed, "formatOf");
  assert.match(body, /dataset\.format/);
  // What a fenced code block in markdown already produces.
  assert.match(body, /language\|lang/);
  assert.match(body, /return "abc"/);
});

test("something that is not text can be written in as base64", () => {
  const body = grab(embed, "contentOf");
  assert.match(body, /base64/);
  assert.match(body, /atob/);
  assert.match(body, /Uint8Array/);
});

// The author's own text stays where they put it.
test("the source is kept in the page", () => {
  assert.match(embed, /el\.hidden = true/);
  assert.match(embed, /insertBefore\(figure, el\.nextSibling\)/);
  assert.doesNotMatch(embed, /el\.remove\(\)|innerHTML = ""/);
});

test("the script is published with the site, and documented", () => {
  assert.match(build, /\$site\/embed\.js/);
  assert.match(docs, /Music written into the page/);
  assert.match(docs, /data-encoding=base64|data-encoding="base64"/);
  assert.match(docs, /gscore: "open"/);
});

/*
 * What a reader sees first.
 *
 * A score embedded in a page was drawn as the editor draws it: one
 * unbroken strip, with the title block above it. In a frame a few hundred
 * pixels wide that is a title the reader has just read in the page, a
 * scrollbar, and two bars of music. Reported as "the embed looks ugly,
 * cannot see the score right away".
 */

test("a player wraps the music to the frame", () => {
  const body = grab(src, "playerWidth");
  assert.match(body, /clientWidth/);
  assert.match(src, /if \(!opt\.mode\) state\.mode = "page"/);
  // The server works the margins out, because it is the one that knows
  // what they are.
  assert.match(src, /p\.set\("fit", String\(fit\)\)/);
  assert.match(api, /name: fit\b/);
});

test("a player leaves the title block out", () => {
  assert.match(src, /if \(state\.player\) p\.set\("title", "0"\)/);
  assert.match(api, /name: title\b/);
  // And hands the title to the page, which writes it in its own type.
  assert.match(embed, /figcaption/);
  assert.match(embed, /m\.title/);
});

// The title arrives with the score that was sent, not with the empty
// frame that was waiting for it.
test("the frame says what it is once it has a score", () => {
  assert.match(src, /case "open": \{[\s\S]*?tellParent\("ready"\)/);
  assert.match(grab(src, "tellParent"), /title:/);
  assert.match(grab(src, "tellParent"), /height:/);
  // Two sizes, because they answer different questions: one system of
  // the music, and all of it.
  assert.match(grab(src, "tellParent"), /system: oneSystem\(\)/);
});

// A frame guessed at is a frame that cuts a system in half — and a frame
// as tall as the whole piece is a page with one tune on it. One system of
// every instrument is the unit the music is read in.
test("a frame opens as tall as one system", () => {
  assert.match(embed, /m\.system/);
  assert.match(embed, /!el\.dataset\.height/);
  // Still the whole music where there is no system to measure.
  assert.match(embed, /m\.height/);
  assert.match(docs, /e\.data\.system/);
  assert.match(docs, /e\.data\.height/);
});

// A page that wants less room than that says so, and the music scrolls:
// the height it gives is used as it stands and nothing overrides it.
test("a page can ask for a shorter frame", () => {
  assert.match(embed, /el\.dataset\.height/);
  assert.match(docs, /data-height/);
});

// Measured from the staves as they were drawn, so that whatever a system
// carries above and below it — bar numbers, dynamics, a volta — is in the
// number. The first bar of the music is in the first system of it.
test("one system is every staff of the first bar", () => {
  // Read from the file rather than from grab(), whose idea of where a
  // function ends is the first line that closes a block at two spaces —
  // which here is the loop over the staves and not the function.
  const one = src.slice(src.indexOf("function oneSystem()"));
  assert.match(one, /state\.hits/);
  assert.match(one, /h\.staffTop \+ h\.staffHeight/);
  // In pixels, which is what a frame is measured in.
  assert.match(one, /\(bottom \+ 2\) \* state\.zoom/);
});

// The music is re-wrapped when the frame changes size, or a phone turned
// on its side shows the first half of each system.
test("it re-wraps when the frame is resized", () => {
  assert.match(grab(src, "applyPlayerMode"), /addEventListener\("resize"/);
  assert.match(grab(src, "applyPlayerMode"), /refreshScore/);
});

// Where the notes are depends on how wide the systems were allowed to be,
// so asking about one picture and drawing another puts every note
// somewhere it is not: a click lands in the wrong bar, and a score
// wrapped to a frame measures as though it had never been wrapped.
test("the clickable places are asked about the picture on the screen", () => {
  assert.match(grab(src, "loadHits"), /viewParams\(\)/);
  assert.doesNotMatch(grab(src, "loadHits"), /"\/api\/hits\?mode="/);
});

/*
 * Lines, or one line.
 *
 * Wrapped is right for music somebody is going to read, and the line is
 * right for a band across a page and for something to play along with.
 * The editor has had both under View since the beginning; what was
 * missing was any way for the page doing the embedding to say which it
 * wanted — so every embedded score was wrapped, whatever it was for.
 */

test("a page can ask for one unbroken line", () => {
  // Through the address, for a frame written by hand.
  assert.match(src, /mode: q\.get\("mode"\)/);
  assert.match(src, /opt\.mode === "strip" \|\| opt\.mode === "page"/);
  // Through the element, for a score written into the page.
  assert.match(grab(embed, "address"), /"mode"/);
  // And both are written down.
  assert.match(docs, /mode=strip/);
  assert.match(docs, /data-mode=strip/);
});

// Changing it without reloading the frame, so that a page offering its
// reader the choice does not throw away where the music had got to.
test("the layout can be changed while it is open", () => {
  assert.match(src, /case "mode": \{/);
  assert.match(src, /case "mode": \{[\s\S]*?state\.mode = want/);
  // And the page is told, because the music is a different shape now and
  // whatever sized the frame has to size it again.
  assert.match(src, /case "mode": \{[\s\S]*?tellParent\("ready"\)/);
});

// The tool that writes the snippet offers everything the snippet can
// carry, or it is a tool for half the job.
test("the embed page offers the choice", () => {
  const page = readFileSync(
    new URL("../../../web/site/embed.html.in", import.meta.url), "utf8");
  assert.match(page, /id="mode"/);
  assert.match(page, /value="strip"/);
  assert.match(page, /data-mode="/);
  // And says what each setting does, rather than only offering three of
  // them in menus: somebody who came to learn how this works should be
  // able to.
  for (const name of ["data-format", "data-encoding", "data-mode",
                      "data-height", "data-player", "data-zoom",
                      "data-autoplay", "data-loop", "data-tune",
                      "data-title", "data-name", "data-selector",
                      "data-app"]) {
    assert.match(page, new RegExp("<code>" + name + "</code>"),
      name + " is not documented on the Embed it page");
  }
});

/*
 * The line that follows the music, and the notes that light up.
 *
 * They are two answers to one question and they do not agree: the line
 * crosses each bar at a steady rate because that is what time does, and
 * the engraver spaces a bar by what is in it — so they meet at the
 * barlines and nowhere in between. The editor wants the line anyway; it
 * says where you are in the music rather than which note is sounding,
 * and it is where playing picks up from after a pause. A player, which
 * lights every sounding notehead on every staff, wants only the notes.
 */

test("a player follows the music with the notes alone", () => {
  assert.match(src, /if \(!state\.player\) moveCursor\(pt\)/);
  assert.match(src, /if \(!state\.player\) makeCursor\(\)/);
  // The editor still has it.
  assert.match(src, /function makeCursor\(\)/);
  assert.match(src, /classList\.add\("playing"\)/);
});
