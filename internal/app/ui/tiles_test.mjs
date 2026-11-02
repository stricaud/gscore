/*
 * tiles_test.mjs
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Lighting up the note being played, on a score too long to draw at once.
 *
 * Beethoven's ninth is not one picture. It is drawn a screenful at a time
 * into one SVG, and the pieces come and go as the view moves — which is the
 * whole reason it can be opened at all. Anything that files those shapes
 * away has to know that the ones it filed are not the ones on the page any
 * more, and watching the SVG element does not tell it: the element never
 * changes, only what is inside it.
 *
 * That was the bug. The index of noteheads by their number was built once,
 * on the first screenful, and never again — so every note drawn after that
 * moment lit nothing at all, which on this score is everything past
 * measure twenty or so.
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

// A page of pieces, each holding some shapes that carry a number.
//
// Enough of a document for the index to work on: a piece can be asked for
// the shapes in it, and a shape knows whether it is still on the page,
// which is how the index tells what has been taken away.
function page() {
  const svg = piece("svg");
  return svg;

  function piece(kind) {
    const el = {
      kind,
      children: [],
      isConnected: true,
      querySelectorAll() {
        const out = [];
        const walk = (n) => {
          for (const c of n.children) {
            if (c.ref !== undefined) out.push(c);
            walk(c);
          }
        };
        walk(el);
        return out;
      },
      add(ref) {
        el.children.push({
          ref, isConnected: true, children: [],
          getAttribute: (k) => (k === "data-ref" ? String(ref) : null),
          classList: { add() {}, remove() {} },
        });
        return el;
      },
      remove() {
        el.isConnected = false;
        for (const c of el.children) c.isConnected = false;
      },
    };
    return el;
  }
}

function tile(...refs) {
  const t = page();
  for (const r of refs) t.add(r);
  return t;
}

// Build refIndex with the module state it reads, so the test drives the
// real thing rather than a copy of it.
function indexOn(state) {
  const scope = {
    currentSVG: () => state.svg,
    get tileEls() {
      return state.tiles;
    },
    get drawGen() {
      return state.gen;
    },
  };
  // tileEls and drawGen are read on every call, so they are handed in as a
  // function of the state rather than copied once.
  const make = new Function(
    "currentSVG", "tilesOf", "genOf",
    grab("refIndex")
      .replace(/\btileEls\b/g, "tilesOf()")
      .replace(/\bdrawGen\b/g, "genOf()") +
      "\nreturn refIndex;")(
    scope.currentSVG, () => state.tiles, () => state.gen);
  return make();
}

test("a notehead in the first piece of the drawing is found", () => {
  const svg = page();
  const a = tile(1, 2, 3);
  svg.children.push(a);
  const state = { svg, tiles: new Map([[0, a]]), gen: 1 };
  const byRef = indexOn(state);

  assert.equal(byRef(2).length, 1, "a shape that is on the page was not found");
  assert.equal(byRef(9).length, 0);
});

// The piece that arrives while the music is playing.
test("a notehead in a piece drawn later is found too", () => {
  const svg = page();
  const a = tile(1, 2);
  svg.children.push(a);
  const state = { svg, tiles: new Map([[0, a]]), gen: 1 };
  const byRef = indexOn(state);
  assert.equal(byRef(1).length, 1);
  assert.equal(byRef(7).length, 0, "the later piece is not drawn yet");

  // The next screenful arrives.
  const b = tile(7, 8);
  svg.children.push(b);
  state.tiles.set(1, b);
  state.gen++;

  assert.equal(byRef(7).length, 1,
    "a note drawn after the index was built lights nothing, which is every " +
    "note past the first screenful");
  assert.equal(byRef(1).length, 1, "the first piece was lost on the way");
});

// And the piece that is dropped when the view has left it well behind.
test("a notehead in a piece that has gone is not found", () => {
  const svg = page();
  const a = tile(1, 2);
  const b = tile(7, 8);
  svg.children.push(a, b);
  const state = { svg, tiles: new Map([[0, a], [1, b]]), gen: 1 };
  const byRef = indexOn(state);
  assert.equal(byRef(1).length, 1);

  a.remove();
  state.tiles.delete(0);
  state.gen++;

  assert.equal(byRef(1).length, 0,
    "a shape that has been taken off the page is still being marked");
  assert.equal(byRef(7).length, 1, "the piece still on the page was dropped too");
});

// The index says when the drawing has changed under it, so that whatever
// was marked on the old shapes can be marked again on the new ones.
//
// Without it the note being played goes dark the moment the piece it is on
// is redrawn and stays dark for as long as it rings: what is sounding has
// not changed, so nothing would think to mark it again.
test("the index says when the drawing changed under it", () => {
  const svg = page();
  const a = tile(1);
  svg.children.push(a);
  const state = { svg, tiles: new Map([[0, a]]), gen: 1 };
  const byRef = indexOn(state);

  byRef(1);
  assert.equal(byRef.stale(), false, "nothing has changed and it says it has");

  state.gen++;
  assert.equal(byRef.stale(), true, "the drawing changed and it did not say so");
  byRef(1);
  assert.equal(byRef.stale(), false, "it did not catch up");
});

// A whole score drawn in one go has no pieces at all, and is one piece:
// itself.
test("a short score is indexed whole", () => {
  const svg = page();
  svg.add(4).add(5);
  const state = { svg, tiles: new Map(), gen: 1 };
  const byRef = indexOn(state);
  assert.equal(byRef(4).length, 1);
  assert.equal(byRef(5).length, 1);
});

// What the player does with that, which is the half that made the note go
// dark: a piece drawn since the last frame carries no marks, so everything
// sounding is marked again even though nothing about the sound changed.
test("the player marks the notes again when the paper is new", () => {
  const fn = grab("follow");
  assert.match(fn, /const redrawn = byRef\.stale\(\)/,
    "the player does not ask whether the drawing changed");
  assert.match(fn, /if \(redrawn\) current = new Set\(\)/,
    "it does not forget what it had marked on paper that has gone");
  assert.match(fn, /if \(redrawn \|\| !sameSet\(sounding, current\)\)/,
    "a redraw with the same notes still sounding does nothing, so the note " +
    "being played goes dark and stays dark");
  // And nothing rings for paper that has only just been drawn: those notes
  // were struck a while ago and are merely still sounding.
  assert.match(fn, /if \(!redrawn && struck < \d+\) ringAt\(n\)/,
    "every note on a newly drawn piece rings again, which is a second strike");
});

// The drawing says when it changed. Every place that adds or takes away a
// piece has to say so, or the index goes quietly out of date again.
test("every change to the drawing is announced", () => {
  for (const [name, why] of [
    ["showTile", "a piece arriving"],
    ["ensureTiles", "a piece being dropped"],
    ["forgetTiles", "the whole drawing being thrown away"],
  ]) {
    assert.match(grab(name), /drawnAgain\(\)/,
      `${why} is not announced, so the index would not know`);
  }
});

/*
 * What the ring costs.
 *
 * A ring over the note just struck is cheap on a quartet and dear on a
 * symphony: a tutti chord asks for two dozen of them in one frame, and if
 * each one measures the drawing to find out where the note is, each one
 * makes the browser lay out every shape on the screen first. A screenful of
 * the ninth is twenty thousand shapes.
 */
test("the ring does not measure the drawing to find a note", () => {
  // Not the prose. The comment above it names the thing it stopped doing,
  // which is the useful thing to write down and not a use of it.
  const fn = grab("ringAt")
    .split("\n")
    .filter((l) => {
      const code = l.trim();
      return !code.startsWith("//") && !code.startsWith("*");
    })
    .join("\n");
  assert.ok(!/getBBox/.test(fn),
    "the ring measures the drawing, which forces the browser to lay out " +
    "every shape on screen — two dozen times in a frame on a tutti chord");
  assert.match(fn, /getAttribute\("x"\)/,
    "a notehead is a glyph set at a place and the place is written on it");
  // And it still cleans up after itself, however it ends.
  assert.match(fn, /animationend/);
  assert.match(fn, /setTimeout\(gone/,
    "a ring whose animation never fires is a ring that stays on the page");
});

/*
 * What colour the note being played is.
 *
 * Red, and said once. There were two rules for it — the red one and, two
 * hundred lines further down, a leftover setting it back to the accent
 * blue. Both have the same specificity, so the later one won and the colour
 * chosen at the top of the file was never what reached the page. Nothing
 * says so: a stylesheet with two rules for one thing is not an error, it is
 * just a stylesheet where one of them does nothing.
 */
test("the note being played is red, and only one rule says so", () => {
  const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
  const rules = [...css.matchAll(/#canvas\s+text\.playing\s*\{([^}]*)\}/g)];
  assert.equal(rules.length, 1,
    "there is more than one rule for the note being played, and the last " +
    "one written is the one that counts whatever the others say");
  assert.match(rules[0][1], /fill:\s*var\(--sounding\)/,
    "the colour of a sounding note is its own, so that changing it is one " +
    "edit and not a search");

  // And that colour is a red, in both themes.
  for (const m of css.matchAll(/--sounding:\s*#([0-9a-f]{6})/gi)) {
    const [r, g, b] = [0, 2, 4].map((i) => parseInt(m[1].slice(i, i + 2), 16));
    assert.ok(r > g && r > b && r > 150,
      `--sounding is #${m[1]}, which is not a red`);
  }
});

/*
 * What the scrolling costs while the music plays.
 *
 * Following the music moves the view every frame, and moving the view is a
 * scroll, so everything hung off scrolling runs sixty times a second for as
 * long as the piece lasts. On a score small enough to be drawn whole that
 * is nothing. On one drawn in pieces it is the work of deciding which
 * pieces are wanted — a question whose answer changes a handful of times a
 * minute — asked three and a half thousand times a minute.
 */

test("a piece of the score is turned into shapes before it is wanted", () => {
  const prep = grab("prepareTile");
  assert.match(prep, /buildTile\(/,
    "nothing turns a piece into shapes ahead of time");
  assert.match(prep, /await fetchTile/);

  const show = grab("showTile");
  assert.match(show, /tileReady\.get\(i\)/,
    "putting a piece on the page does not use the one already made");
  // And when it does, that is all it does: the expensive half is behind it.
  const quick = show.slice(show.indexOf("tileReady.get(i)"));
  assert.ok(!/innerHTML/.test(quick.slice(0, quick.indexOf("Claimed"))),
    "a piece that was made ahead is parsed again on the way in");

  // The making is asked for in a gap between frames, not in the scroll
  // that noticed it was needed.
  assert.match(grab("ensureTiles"), /whenIdle\(\(\) => void prepareTile/,
    "a piece is made in the middle of the scrolling that asked for it, " +
    "which is the stall it was meant to prevent");
});

test("the scroll does not ask what it needs on every frame", () => {
  const fn = grab("setupVirtualDraw");
  assert.match(fn, /askEvery/,
    "every frame of every scroll works out which pieces are wanted, and " +
    "while the music plays that is sixty times a second");
  const every = Number(/const askEvery = (\d+)/.exec(fn)?.[1]);
  assert.ok(every >= 60 && every <= 500,
    `it asks every ${every}ms, which is either still too often or slow ` +
    "enough to be noticed");
});

test("the sprint buttons do not touch the page to say what it says", () => {
  const fn = grab("drawSprints");
  // The path taken on every frame of every scroll once the view has left
  // the start of the staff.
  assert.match(fn, /if \(!box\.hidden\) \{/,
    "the hidden path writes to the page whether or not anything changed");
});

/*
 * Being able to look at the rest of the score.
 *
 * The music sits in a cell of the window's grid, and a grid cell will not
 * shrink below the thing inside it unless it is told it may. A score of
 * fifty-three staves is eight thousand pixels deep: told nothing, the cell
 * grows to that, the window cuts it off at the fold, and the overflow that
 * was supposed to give a scrollbar has nothing to do — so every stave but
 * the first few is on the page and unreachable.
 *
 * The width had been said and the height had not, which is exactly how a
 * long score scrolled along and a tall one did not scroll at all.
 */
test("the music can be shorter and narrower than the score", () => {
  const css = readFileSync(new URL("./style.css", import.meta.url), "utf8");
  const rule = /#canvas-wrap\s*\{([\s\S]*?)\}/.exec(css);
  assert.ok(rule, "there is no rule for the music's own cell");
  const body = rule[1];
  assert.match(body, /overflow:\s*auto/,
    "the music cannot be scrolled at all");
  for (const side of ["min-width", "min-height"]) {
    assert.match(body, new RegExp(side + ":\\s*0"),
      `${side} is not held down, so the cell grows to the score and the ` +
      "part of it past the window cannot be reached");
  }
});

/*
 * Looking at the other staves while the music plays.
 *
 * Following drags the page to the cursor, which is what you want while you
 * are watching it and exactly what you do not want the moment you look
 * somewhere else — and on a score of fifty-three staves, looking somewhere
 * else is most of what you do.
 */

test("the music does not write to the page while a gesture is in flight", () => {
  const fn = grab("followCursor");
  assert.match(fn, /if \(readerIsScrolling\(\)\) return;/,
    "the sideways follow writes a scroll offset on every frame, and a write " +
    "ends whatever scrolling was already under way — so a two-finger scroll " +
    "down the staves is cancelled sixty times a second");
  // Before it writes anything, not after.
  assert.ok(fn.indexOf("readerIsScrolling()") < fn.indexOf("wrap.scrollLeft = want"),
    "it writes first and asks afterwards, which is the same as not asking");

  // Any gesture counts, sideways included: a write would end that one too.
  const watch = grab("setupFollowRelease");
  assert.match(watch, /readerScrolledAt = performance\.now\(\)/);
  assert.ok(watch.indexOf("readerScrolledAt") < watch.indexOf("if (e.type === \"wheel\""),
    "a sideways swipe is not counted, so the music fights that one");
});

test("music deeper than the window is not followed down the page", () => {
  const fn = grab("followCursor");
  assert.match(fn, /if \(bottom - top > wrap\.clientHeight\) return;/,
    "a system taller than the window has no scroll position that shows all " +
    "of it, so the test below never passes and the page is pinned to the " +
    "top of the score for the whole piece");
  // And that question is asked before the one it would otherwise defeat.
  assert.ok(fn.indexOf("bottom - top > wrap.clientHeight") <
            fn.indexOf("top >= wrap.scrollTop"),
    "the impossible test is reached first");
});
