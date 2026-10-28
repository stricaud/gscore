/*
 * harmpanel_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The harmonics panel holds still.
 *
 * What it draws is tested next door, in harmonics_test.mjs. This is about
 * the window it is drawn in.
 *
 * The panel follows whatever is picked up, and what it has to say about one
 * note — a line, and a table with no rows in it — is a good deal shorter
 * than what it has to say about a chord of four, which is six rows and a
 * crowded spectrum. Sized to its contents, the window grew and shrank under
 * the pointer at every note, taking the close button with it and moving the
 * thing being read out from under the eye.
 *
 * So it is one size whatever it is showing, the contents scroll inside it,
 * and the way out and the name of the staff being read are at the top where
 * they can always be found.
 */

import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";
import test from "node:test";

const here = (f) => readFileSync(new URL("./" + f, import.meta.url), "utf8");
const src = here("app.js");
const html = here("index.html");
const css = here("style.css");

const grab = (name) => {
  const m = src.match(new RegExp("^(?:async )?function " + name + "\\([\\s\\S]*?^}", "m"));
  if (!m) throw new Error("app.js has no " + name);
  return m[0];
};

// The dialog's markup, from <dialog id="harmonics"> to its close.
const dialog = html.match(/<dialog id="harmonics">[\s\S]*?<\/dialog>/)[0];

test("the window is given a height rather than taking one", () => {
  const rule = css.match(/#harmonics \{[\s\S]*?\}/)[0];
  assert.match(rule, /height:\s*min\(/,
    "the window still grows to whatever is in it");
  assert.match(rule, /flex-direction:\s*column/,
    "the head and the body are not stacked, so one cannot stay while the other scrolls");
  const body = css.match(/#harm-body \{[\s\S]*?\}/)[0];
  assert.match(body, /overflow-y:\s*auto/, "the contents do not scroll");
  assert.match(body, /min-height:\s*0/,
    "a flex child without this refuses to shrink, and the window grows after all");
});

test("the way out is at the top, above everything that scrolls", () => {
  const head = dialog.indexOf('class="dlg-head"');
  const close = dialog.indexOf('class="dlg-close"');
  const body = dialog.indexOf('id="harm-body"');
  assert.ok(head >= 0, "there is no head to the window");
  assert.ok(close > head && close < body,
    "the close button is not in the head, above the part that scrolls");
  assert.ok(/<form method="dialog">/.test(dialog.slice(head, body)),
    "the button does not close the dialog on its own");
});

test("the head says which staff is being read", () => {
  assert.ok(dialog.indexOf('id="harm-staff"') < dialog.indexOf('id="harm-body"'),
    "the staff is named inside the part that scrolls, so it can scroll away");
  assert.match(grab("drawHarmonics"), /harm-staff/,
    "nothing ever fills it in");
});

// A tiny stand-in for the two elements the head is made of.
function panel(d) {
  const made = new Map();
  const el = () => ({ textContent: "", innerHTML: "", title: "", style: {} });
  const scope = {
    harmColours: ["#111", "#222", "#333"],
    Math,
    escapeHTML: (s) => String(s),
    drawSpectrum: () => {},
    drawIntervals: () => {},
    drawChord: () => {},
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, el());
      return made.get(sel);
    },
  };
  const names = ["drawHarmonics", "harmStaves", "staffName"];
  const body = names.map((n) => grab(n)).join("\n") + "\nreturn { drawHarmonics };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawHarmonics(d);
  return made;
}

test("one staff is named once, in the head", () => {
  const made = panel({
    at: 1.5, roughness: 0.2, verdict: "smooth",
    staves: ["Piano", "voice 2"],
    notes: [{ name: "C♯4", part: 0 }, { name: "E4", part: 0 }],
  });
  assert.match(made.get("#harm-staff").textContent, /^Piano — at 1\.50s$/,
    "the head does not say what is being read and when");
  assert.ok(!/Piano/.test(made.get("#harm-where").innerHTML),
    "the staff is repeated beside every note, which on one staff is noise");
  assert.match(made.get("#harm-where").innerHTML, /C♯4/, "the notes are not listed");
});

test("a chord across two staves names the staff beside each note", () => {
  const made = panel({
    at: 2, staves: ["Piano", "voice 2"],
    notes: [{ name: "C♯4", part: 0 }, { name: "G♯2", part: 1 }],
  });
  assert.match(made.get("#harm-staff").textContent, /Piano · voice 2/,
    "the head does not name both staves");
  const where = made.get("#harm-where").innerHTML;
  assert.match(where, /C♯4<\/em> <span class="harm-on">\(Piano\)/,
    "the first note does not say which staff it is on");
  assert.match(where, /\(voice 2\)/, "nor does the second");
});

test("with nothing sounding the head still says when", () => {
  const made = panel({ at: 4.25, staves: ["Piano"], notes: [] });
  assert.match(made.get("#harm-staff").textContent, /Nothing sounding at 4\.25s/);
});

// A staff with no name of its own is still a staff, and the panel has to
// call it something.
test("an unnamed staff is numbered", () => {
  const made = panel({ at: 0, staves: [], notes: [{ name: "C4", part: 2 }] });
  assert.match(made.get("#harm-staff").textContent, /^Staff 3 —/);
});

/*
 * And the trap that giving it a size walked straight into.
 *
 * A <dialog> is hidden by the browser's own rule, `dialog:not([open])
 * { display: none }`. That is one element and one class's worth of
 * specificity, so a `display` written against a dialog's id beats it — and
 * the dialog is then on the screen from the moment the page loads, with
 * closing it making no difference, because closing only takes the `open`
 * attribute off.
 *
 * Laying the harmonics panel out as a flex column did exactly that: the
 * window was up before anybody asked for it and would not go away. The rule
 * is general, so the test is: whatever a dialog is given, it is given it
 * only while it is open.
 */
test("no dialog is given a display except while it is open", () => {
  const ids = [...html.matchAll(/<dialog id="([^"]+)"/g)].map((m) => m[1]);
  assert.ok(ids.length > 2, "no dialogs were found, so this is testing nothing");

  const plain = css.replace(/\/\*[\s\S]*?\*\//g, "");
  const rules = plain.split("}").map((chunk) => {
    const i = chunk.indexOf("{");
    return i < 0 ? null : { sel: chunk.slice(0, i).trim(), body: chunk.slice(i + 1) };
  }).filter(Boolean);

  for (const { sel, body } of rules) {
    // `display: none` is the safe direction: it hides a thing that was
    // going to be hidden anyway. Only a display that shows something can
    // take a dialog out of the browser's hands.
    const shows = [...body.matchAll(/(?:^|[;\s])display\s*:\s*([\w-]+)/g)]
      .some((m) => m[1] !== "none");
    if (!shows) continue;
    for (const one of sel.split(",")) {
      // The subject of the selector, which is the last thing in the chain:
      // `#ask label` styles a label inside a dialog and says nothing about
      // the dialog itself.
      const parts = one.trim().split(/[\s>+~]+/).filter(Boolean);
      const subject = parts[parts.length - 1] || "";
      for (const id of ids) {
        if (!new RegExp("#" + id + "(?![\\w-])").test(subject)) continue;
        assert.match(subject, /\[open\]/,
          `"${one.trim()}" sets display, so #${id} is on the screen ` +
          `before anybody asks for it and closing it changes nothing`);
      }
    }
  }
});

test("the panel is only ever opened by asking for it", () => {
  const calls = [...src.matchAll(/openHarmonics\(\)/g)].length;
  // Its own definition, and the one menu action that calls it.
  assert.equal(calls, 2,
    "something other than the Harmonics menu item opens the panel");
  assert.match(src, /case "harmonics": await openHarmonics\(\)/,
    "the menu item no longer opens it");
});

/*
 * What the chord is called, and what it is doing.
 *
 * Two different claims, and the panel has to make them differently. The name
 * is a fact about the notes: A, C♯ and E with E at the bottom is A major in
 * second inversion whatever else is going on. The numeral is a fact about
 * the notes and the key together — in E that is IV, in A it is the tonic —
 * and it is the one that says what the chord is for.
 *
 * The key is read from the phrase around the chord rather than from the key
 * signature, so a stretch of music that does not settle anywhere comes back
 * saying so. A bare fifth is in a dozen keys at once, and naming one of them
 * would be a guess told in the voice of a fact.
 */
function chordLine(d) {
  const made = new Map();
  const scope = {
    escapeHTML: (s) => String(s),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "", textContent: "" });
      return made.get(sel);
    },
  };
  const body = grab("drawChord") + "\nreturn { drawChord };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawChord(d);
  return made.get("#harm-chord").innerHTML;
}

// The same, handing back every box it wrote into rather than only the one.
function chordLineAll(d) {
  const made = new Map();
  const scope = {
    escapeHTML: (s) => String(s),
    $: (sel) => {
      if (!made.has(sel)) made.set(sel, { innerHTML: "", textContent: "", hidden: false });
      return made.get(sel);
    },
  };
  const body = grab("drawChord") + "\nreturn { drawChord };";
  new Function(...Object.keys(scope), body)(...Object.values(scope)).drawChord(d);
  return made;
}

const settled = {
  chord: { named: true, name: "G dominant seventh", root: 7, quality: "dominant seventh" },
  keyName: "C major", keyFit: { correlation: 0.95, margin: 0.28 },
  numeral: "V7", function: "dominant",
};

test("a chord in a settled key is named and given its numeral", () => {
  const out = chordLine(settled);
  assert.match(out, /V7/, "the numeral is missing");
  assert.match(out, /G dominant seventh/, "the chord is not named");
  assert.match(out, /dominant/, "what it is doing is not said");
  assert.match(out, /in C major/, "the key it is being read in is not said");
});

// Where the key is not settled the chord still has a name — that is a fact
// about the notes — but the numeral is withheld, because a numeral is a
// claim about a key and there is not one to make it about.
test("an unsettled key withholds the numeral and says why", () => {
  const out = chordLine({ ...settled, keyFit: { correlation: 0.81, margin: 0.04 } });
  assert.match(out, /G dominant seventh/, "the chord lost its name as well");
  assert.ok(!/V7/.test(out), "a numeral was given for a key that is not settled");
  assert.match(out, /not settled/, "nothing says why the numeral is missing");
});

// The orchestrator's version of the roughness number: whether the bottom of
// this chord will speak where it is written.
test("a chord spaced too low to speak says so", () => {
  const made = chordLineAll({
    ...settled,
    thick: { text: "the bottom of this chord is spaced below where it usually speaks clearly" },
  });
  const warn = made.get("#harm-thick");
  assert.equal(warn.hidden, false, "the warning is hidden when there is one");
  assert.match(warn.textContent, /speaks clearly/, "it says nothing");
});

test("a chord that speaks fine says nothing about it", () => {
  const warn = chordLineAll(settled).get("#harm-thick");
  assert.equal(warn.hidden, true, "an empty warning is left on the screen");
  assert.equal(warn.textContent, "");
});

test("notes that are not a chord are not named at all", () => {
  assert.equal(chordLine({ chord: { named: false }, keyName: "C major" }), "",
    "a cluster was given a name");
  assert.equal(chordLine({}), "", "nothing sounding was given one");
});
