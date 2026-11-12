/*
 * gscore - a musical score editor
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * The editor front end: menus, toolbox, the engraved score, and playback.
 * The score is drawn by the Go engraver and arrives as SVG, so what runs here
 * only has to present it, scroll it and make a noise.
 */

const $ = (sel) => document.querySelector(sel);

// importLog holds the lines the importer could not read, for the log window.
let importLog = [];

const $$ = (sel) => Array.from(document.querySelectorAll(sel));

// currentSVG is the drawing being worked on.
//
// When a whole book is on screen the canvas holds one drawing per tune, and
// asking for "#canvas svg" hands back the first of them whatever is selected.
// Everything that places a cursor, highlights a note or measures a click has
// to ask for the right one.
function currentSVG() {
  if (state.showAll) {
    const d = $(`#canvas .tune[data-tune="${state.info?.current ?? 0}"] svg`);
    if (d) return d;
  }
  return $("#canvas svg");
}

const state = {
  zoom: 8,
  // Embedded in a page as a player rather than opened as the editor.
  // See applyPlayerMode.
  player: false,
  // How big the interface is drawn, where one is the size it was designed
  // at. Read from the settings at boot; see applyUIScale.
  uiScale: 1,
  // How far behind the audio clock the sound reaches the ears, in
  // milliseconds, where the browser will not say and somebody has measured
  // it for their own headphones. Read from the settings at boot; see
  // outputLag.
  playLag: 0,
  // Whether a note is sounded as it is written. Read from the settings at
  // boot; see soundAsWritten.
  entrySound: true,
  // The continuous strip is gscore's own view and stays the default: the
  // program called itself non-page-oriented, and the strip is what makes it
  // good to edit in. Pagination is for printing.
  mode: "strip",
  info: null,
  playing: false,
  // What is selected.
  selection: null,
  // Whether a click on the staff writes a note or picks one up.
  //
  // Off, which is to say the mouse points and the keyboard writes. A click
  // means "this one" — the thing everybody expects a click to mean, and the
  // thing you do far more often than writing. Notes are typed, which is
  // faster than clicking them in anyway once the cursor is where you want
  // it.
  //
  // Writing with the mouse used to be the only way round: a click wrote a
  // note and shift-click was the only way to pick one up, so selecting —
  // the commonest thing anybody does — needed two hands. Insert ▸ Write
  // with the mouse turns it back on for anyone who prefers it, and shift
  // still selects while it is on.
  mouseWrites: false,
  // Whether a click sounds on every beat while the music plays. Off until
  // somebody asks for it: nobody presses Play expecting a metronome.
  // Remembered between sessions, like the mouse. See setMetronome.
  metronome: false,
  // Whether the noteheads are coloured by what note they are. Kept on the
  // server, because the printed page is drawn there too; this is a copy for
  // the tick in the menu. See noteColoursDialog.
  colourNotes: false,

  hits: [],
  // The performance, kept until the music changes. See playbackData.
  playback: null,
  // How big the engraved score is, in staff spaces, and how it is cut into
  // pieces — null when the whole of it goes in the page at once. See
  // tilePlan.
  size: null,
  tiles: null,
  tool: { duration: "quarter", dots: 0, accidental: "none", rest: false, drum: null },
  // The kit, once fetched. A score without a drum staff never asks for it.
  kit: null,
  // A file may hold many scores; showAll puts the whole book on screen at
  // once rather than one tune at a time. It is decided when a file is opened:
  // opening a book should show the book.
  showAll: false,
  showAllDecided: false,
  // Everything a dragged selection picked up, as staff:measure:index.
  multi: [],
  // Where a shift-click measures from: the last note picked up on its own.
  anchor: null,
  // Player mode: nothing edits, and playing may go round again at the end.
  readOnly: false,
  loop: false,
  // Where playing was paused, in seconds of the score's own time, or null
  // for "not paused". Playing again carries on from here.
  pausedAt: null,
  // And what was selected at the time, so that picking a different note
  // while paused can be told from not having touched anything.
  pausedSel: "",
  // Drawing options that belong to the view rather than to the file.
  // Measure numbers are on: the engraver's DefaultOptions says so, and this
  // has to say the same thing or the tick beside "Measure numbers" in the
  // View menu starts out contradicting the page.
  // What the drawing shows. The metronome marking starts on, which is
  // what DefaultOptions does on the other side — a tick that starts out
  // lying is worse than no tick.
  // The beat marks start off: they are an aid to reading rather than
  // notation, and a score nobody asked to have marked up should open
  // looking like the score.
  view: { names: false, numbers: true, multirest: false, tempomark: true,
          beatmarks: false, concert: false },
  // Whether a bar that is too long has its last notes moved along. Read from
  // the server at startup, because it is remembered between sessions.
  trackBarlines: true,
  // The two ends of a selection being grown from the keyboard: the one that
  // stays put and the one the arrows move.
  anchor: null,
  focus: null,
  // Whether a saved .gscore carries a PDF of itself in front of the music.
  //
  // It does by default: renamed .pdf the same file opens in anything that
  // reads one, which is most of what somebody might send it to. The cost is
  // the size — the engraving is far larger than the music — so it can be
  // turned off.
  // Where each measure's closing barline was drawn, so one can be clicked.
  barlines: [],
  // The clef, key and meter at the head of each system, likewise.
  headers: [],
  // The band over each bar, which is how a whole bar is picked up.
  strips: [],
};

/* ---------- toolbox ---------- */

// hint says what the pointer is over, and unhint puts back whatever the
// status line was saying before.
//
// A name shown while the pointer rests on a button is the cheapest
// documentation there is: forty small pictures in a grid are forty things
// to guess at otherwise, and a tooltip is a second late and in the way.
let hintWas = null;

function hint(text) {
  const bar = document.getElementById("status");
  if (!bar) return;
  if (hintWas === null) hintWas = bar.textContent;
  bar.textContent = text;
}

function unhint() {
  const bar = document.getElementById("status");
  if (!bar || hintWas === null) return;
  bar.textContent = hintWas;
  hintWas = null;
}


// The palette lives in palette.js, which holds what is in it. This is what
// happens when one is clicked.
async function paletteAction(item) {
  // A mark picked up by clicking it on the page, and then an articulation
  // chosen from the palette: that is asking for this mark to be written
  // there instead — which, when it is the same mark, means move it to the
  // side the palette's own Above and Below say.
  if (item.apply === "articulation" && state.selection?.articulation &&
      !state.multi.length) {
    const was = state.selection.articulation;
    if (item.value === was.mark) {
      await mark("articulation",
        { articulation: was.mark, placement: was.above ? "below" : "above",
          ...pickedHead() },
        `${t(item.label)} moved ${was.above ? "under" : "over"} the note`);
      return;
    }
  }

  if (item.tool) {
    switch (item.tool) {
      case "rest":
        state.tool.rest = true;
        if (item.dur) state.tool.duration = item.dur;
        if (item.dur && heldForChanging()) {
          await setSelectionDuration(item.dur, item.label);
        }
        break;
      case "acc":
        // An accidental means two things depending on what is in front of
        // you. With a note selected it belongs to that note and is written
        // on it now, and nothing is left held: the click was asked to change
        // one note and changing one note is all of it. Arming the pen as
        // well wrote a natural on every note that followed, and the button
        // that said so was never lit.
        if (hasSelection()) {
          await setAccidental(item.acc, item.label);
          markTools();
          return;
        }
        // With nothing selected it is a decision about the next note, the
        // way the palette's other tools are — and only about the next one.
        state.tool.accidental = item.acc;
        markTools();
        status(item.label + " — on the next note written");
        return;
      default:
        state.tool.rest = false;
        state.tool.duration = item.tool;
        // A note value means two things depending on what is in front of
        // you, exactly as an accidental does. With nothing selected it is a
        // decision about the next note; with notes picked up it is those
        // notes that are meant — somebody who has selected four sixteenths
        // and clicked the thirty-second is not choosing a pen, they are
        // telling you the four notes are wrong.
        //
        // The note just written is the exception, and it is the common
        // case while typing: the value clicked then is for the note about
        // to be written, not for the one behind the cursor. See
        // heldForChanging.
        if (heldForChanging()) {
          await setSelectionDuration(item.tool, item.label);
        }
    }
    markTools();
    status(item.label);
    return;
  }

  if (item.apply === "dot") {
    // The same dot the full stop types: with something picked up it dots
    // that, and with nothing picked up it belongs to the value being held
    // for the next note. The button used to do only the second of those, so
    // selecting a rest and pressing the dot changed what the next note
    // would be and left the rest alone.
    await dotSelection();
    return;
  }
  // An ending is a bracket over whole bars, so it acts on the measure the
  // cursor is in rather than on the note.
  // A drum means two things depending on what is in front of you, exactly
  // as a note value and an accidental do. With the cursor on a drum staff
  // it is that drum, there, now; with nothing to write on it is a decision
  // about the next note. It used to be only the second of those, so
  // choosing the snare and then having to go and click the staff was two
  // gestures for one note — and the second of them a click on five lines
  // that name instruments rather than pitches, which is the guessing the
  // drum list is here to replace.
  if (item.drum !== undefined) {
    state.tool.drum = item.drum;
    state.tool.rest = false;
    markPaletteTool((label) => label === item.label);
    if (state.selection && isDrumStaff(state.selection.part)) {
      await insertAtSelection(false);
      return;
    }
    // Not "click the drum staff": a click points, it does not write. What
    // is missing is a cursor on a staff that takes drums.
    status(item.label + " — put the cursor on a drum staff to write it");
    return;
  }
  if (item.ending !== undefined) {
    await setEnding(item.ending);
    return;
  }
  if (item.sign !== undefined) {
    await setJump({ sign: item.sign }, item.label);
    return;
  }
  if (item.jump !== undefined) {
    await setJump({ jump: item.jump }, item.label);
    return;
  }
  // An item that does exactly what a menu entry does, named by that entry,
  // so the palette and the menu cannot drift apart.
  if (item.action) {
    await action(item.action);
    return;
  }
  if (item.apply) {
    // A mark that covers a stretch of music needs a stretch selected. Without
    // this it was sent for a single note, where a hairpin opens and closes in
    // the same place and so draws nothing at all — the button appeared to do
    // nothing, which is worse than refusing.
    if (item.needsRange && state.multi.length < 2) {
      status(item.label +
        " covers a passage \u2014 click a note, then shift-click a later one");
      return;
    }
    await mark(item.apply, { [item.apply]: item.value }, item.label);
    return;
  }
  if (item.measure === "barline") {
    await setBarline(item.value);
    return;
  }
  if (item.staff) {
    await setStaffProperty(item.staff, item.value);
  }
}

// pickDuration chooses what the next note written is worth.
//
// It is the palette's job done from the keyboard, and it says so: a value
// chosen and nothing written yet is only visible in the toolbox and the
// status line, so both are brought up to date.
//
// It turns the rest tool off. A number asks for a note; a rest of that value
// is the rest tool, or Enter, which writes one at the cursor whatever is
// held.
function pickDuration(dur, dots) {
  state.tool.duration = dur;
  state.tool.dots = dots || 0;
  state.tool.rest = false;
  markTools();
  status(durationName(dur, state.tool.dots));
}

// removeMeasure takes the bar being looked at out of the score.
//
// The bar the selection is in, or the last one where nothing is picked up,
// which is what somebody who has typed two bars too many at the end of a
// piece means. Every staff loses it: a bar is a bar across the score, and
// taking one out of a single staff would leave that staff a bar short of
// the others for the rest of the piece.
//
// A bar with music in it is worth a question first. It can be taken back —
// everything here can — but a passage that goes because somebody meant to
// press the entry above is a passage they have to notice going.
async function removeMeasure() {
  const sel = state.selection;
  const bars = state.info?.measures ?? 0;
  const at = sel && sel.measure !== undefined && sel.measure !== null
    ? sel.measure
    : bars - 1;
  if (at < 0) {
    status("There is no bar to remove");
    return;
  }

  if (barHasMusic(at)) {
    const sure = await confirmThat({
      title: `Bar ${at + 1} has music in it — remove it anyway?`,
      note: "The bar goes from every staff and the ones after it move up. " +
            "Undo puts it back.",
      ok: "Remove it",
    });
    if (!sure) return;
  }

  const res = await fetch("/api/removemeasure?measure=" + at,
                          { method: "POST" });
  if (!res.ok) {
    status("Could not remove it: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  state.selection = null;
  state.multi = [];
  await reload();
  status(`Bar ${at + 1} removed — ${d.measures} ` +
    (d.measures === 1 ? "bar" : "bars") + " left");
}

// barHasMusic says whether anything is written in a bar, on any staff.
//
// A rest is not music for this: an empty bar is drawn as a bar of rest, so
// asking about one would be asking about every untouched bar in the score.
function barHasMusic(measure) {
  for (const h of state.hits || []) {
    if (h.measure !== measure) continue;
    for (const el of h.elements || []) {
      if (el.kind && el.kind !== "rest") return true;
    }
  }
  return false;
}

// dotSelection lengthens the note that is selected by half.
//
// The dot in the palette belongs to the value being held — "3" is the half
// button and then the dot — and this is the same dot aimed at a note that
// is already on the page. With nothing selected it falls back to the value
// held, so the key does something sensible either way.
async function dotSelection() {
  const sel = state.selection;
  if (!hasSelection()) {
    state.tool.dots = state.tool.dots > 0 ? 0 : 1;
    markTools();
    status(durationName(state.tool.duration, state.tool.dots));
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at),
  });
  const res = await fetch("/api/dot?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not dot that: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  await refreshScore();
  markSelection();
  status(d.dots > 0 ? "Dotted — now " + d.dur : "Dot removed — now " + d.dur);
}

// durationName is what a value is called, for saying so.
function durationName(dur, dots) {
  const name = { whole: "whole", half: "half", quarter: "quarter",
                 eighth: "eighth", sixteenth: "sixteenth", "32nd": "32nd",
                 "64th": "64th" }[dur] || dur;
  const dotted = ["", "Dotted ", "Double-dotted "][dots || 0] || "";
  return dotted + name + " note";
}

// currentToolLabel names the tool held, so the palette can show it.
function currentToolLabel() {
  if (state.tool.rest) {
    return { whole: "Whole rest", half: "Half rest", quarter: "Quarter rest",
             eighth: "Eighth rest", sixteenth: "16th rest", "32nd": "32nd rest" }[state.tool.duration];
  }
  return { whole: "Whole", half: "Half", quarter: "Quarter", eighth: "Eighth",
           sixteenth: "16th", "32nd": "32nd", "64th": "64th" }[state.tool.duration];
}

// currentAccidentalLabel is the palette's name for the accidental being
// held, or null when there is none. The names are the buttons' own, because
// the button is what has to light up.
function currentAccidentalLabel() {
  return { sharp: "Sharp", flat: "Flat", natural: "Natural",
           "double sharp": "Double sharp",
           "double flat": "Double flat" }[state.tool.accidental] || null;
}

// markTools lights what is held, which is a note value and, sometimes, an
// accidental to write on it.
//
// Two tools, one at a time, was the whole of the trouble: markPaletteTool
// takes a single predicate and the note value always won it, so an armed
// accidental was a mode with nothing on the screen to say it was on.
function markTools() {
  const acc = currentAccidentalLabel();
  markPaletteTool((label) => label === currentToolLabel() || label === acc);
}

// useAccidental takes the held accidental off, the note it was for having
// been written.
//
// An accidental belongs to a note and not to the pen. Held on past the note
// it was clicked for, it is a mode somebody has to know to turn off, and the
// only way to find out it was still on was to read back what you had just
// written.
function useAccidental() {
  if (state.tool.accidental === "none") return;
  state.tool.accidental = "none";
  markTools();
}

// setStaffProperty changes something about the staff itself, rather than
// about a note on it.
async function setStaffProperty(what, value) {
  const sel = state.selection;
  const part = sel ? sel.part : 0;
  const res = await fetch(`/api/staff?part=${part}&${what}=${encodeURIComponent(value)}`,
                          { method: "POST" });
  if (!res.ok) {
    status("Could not change the staff: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  // "sound" is the name of a field, not a word to say to somebody: what
  // they chose was an instrument.
  status(what === "sound"
    ? `Instrument: ${instrumentNamed(value)}`
    : `${what}: ${value}`);
}

/* ---------- menus ---------- */

function setupMenus() {
  // A menu does not take the focus.
  //
  // Pressing a button focuses it, and focusing something inside a box that
  // scrolls makes the box scroll it into view. A menu too tall for the
  // window is exactly such a box — so the press moved the entry out from
  // under the finger, the release landed somewhere else, and a click is
  // only delivered when the press and the release are on the same thing.
  // No click: the entry did nothing at all.
  //
  // Which is why it was the Edit and Insert menus and no others. They are
  // the two long enough to scroll, so they are the two whose entries moved
  // when pressed. Everything short enough to fit worked, in every engine,
  // which is why it never showed up in a browser on a big screen.
  //
  // mousedown rather than pointerdown: calling preventDefault on a
  // pointerdown is allowed to suppress the mouse events that follow it,
  // the click among them, which would be this bug again by another route.
  const dontFocus = (e) => {
    if (e.target.closest && e.target.closest("button")) e.preventDefault();
  };

  $$(".menu").forEach((menu) => {
    menu.querySelector(".menu-items")?.addEventListener("mousedown", dontFocus);
    menu.querySelector(".menu-title").addEventListener("click", (e) => {
      e.stopPropagation();
      const wasOpen = menu.classList.contains("open");
      closeMenus();
      if (!wasOpen) menu.classList.add("open");
      // A last chance for any picture that could not be drawn when the page
      // was built, because the music font had not arrived yet. Everything
      // already drawn is left alone, so opening a menu costs nothing.
      if (!wasOpen) fitGlyphs(menu);
    });
    // Moving back up to the plain items closes the submenu, so the panel
    // does not sit over the item being reached for.
    // The menu's own items, not the ones inside its submenus: those are
    // matched by the same selector, so hovering "Crescendo" would have
    // closed the submenu it is in.
    menu.querySelectorAll(":scope > .menu-items > button").forEach((b) => {
      b.addEventListener("mouseenter", closeSubmenusSoon);
    });
    menu.addEventListener("mouseenter", () => {
      // Once one menu is open, sliding along the bar opens the others, the
      // way a menu bar is expected to behave.
      if ($(".menu.open")) {
        closeMenus();
        menu.classList.add("open");
      }
    });
  });
  // A submenu opens on hover, but a click has to work too: on a touch screen
  // and with a keyboard there is no hover to open it with.
  $$(".submenu").forEach((sub) => {
    const title = sub.querySelector(".submenu-title");
    // Opened by the press, not by the click. The press is the half of the
    // gesture that is certainly delivered: a click needs the release to
    // land on the same entry, and in a menu that scrolls it may not.
    //
    // preventDefault here too, for the focus — this is the same button the
    // panel's own handler covers, and saying it twice costs nothing and
    // means the rule holds wherever this listener is reached from.
    let justPressed = false;
    title.addEventListener("mousedown", (e) => {
      // Only the plain button of the mouse opens things.
      if (e.button !== 0) return;
      e.preventDefault();
      e.stopPropagation();
      justPressed = true;
      if (sub.classList.contains("open")) {
        closeSubmenu(sub);
        justPressed = false;
        return;
      }
      openSubmenu(sub);
    });
    title.addEventListener("click", (e) => {
      e.stopPropagation();
      // The press already decided. Without this the click that follows it
      // would close what the press opened, and the panel would flash.
      if (justPressed) {
        justPressed = false;
        return;
      }
      if (sub.classList.contains("open")) {
        closeSubmenu(sub);
        return;
      }
      openSubmenu(sub);
    });
    // Hovering the title opens it and closes every other, which is how a
    // menu bar behaves. Hovering the open panel itself must not close it, so
    // the listener is on the title and not on the submenu as a whole.
    sub.querySelector(".submenu-title").addEventListener("mouseenter", () => {
      openSubmenuSoon(sub);
    });
    // Moved on without stopping: it was being crossed, not chosen.
    sub.querySelector(".submenu-title").addEventListener("mouseleave",
      cancelSubmenuOpen);
    // Reaching the panel is arriving. Whatever was pending because the
    // pointer crossed other entries on the way here — a close from the
    // plain ones, an open from the other titles — is called off.
    panelOf(sub)?.addEventListener("mouseenter", () => {
      cancelSubmenuOpen();
      keepSubmenus();
    });
    // Parked on the body, a flyout is outside the menu panel that holds
    // the rule off the focus, so it carries its own.
    panelOf(sub)?.addEventListener("mousedown", dontFocus);
  });

  // A menu too tall for the window scrolls, and a submenu opened out of it
  // is fixed to the window rather than to the panel — so it stays put while
  // the entry that owns it moves away under it. Moved back level each time
  // the panel scrolls, which is cheap: there is at most one open.
  $$(".menu > .menu-items").forEach((panel) => {
    panel.addEventListener("scroll", () => {
      const open = panel.querySelector(".submenu.open");
      if (open) placeSubmenu(open);
    });
  });

  document.addEventListener("click", closeMenus);

  $$("[data-action]").forEach((b) => {
    b.addEventListener("click", () => {
      if (b.disabled) return;
      closeMenus();
      action(b.dataset.action);
    });
  });
}

function closeMenus() {
  keepSubmenus();
  cancelSubmenuOpen();
  $$(".menu").forEach((m) => m.classList.remove("open"));
  shutSubmenus();
}

/* ---------- how big the interface is drawn ---------- */

// uiScale is the size of the menus, the palette and the panels, where one
// is the size they were designed at.
//
// Everything in the interface is measured in pixels, and a pixel is a
// physical size that depends on the screen: on a large high-resolution
// display the whole interface came out tiny while the score beside it was
// perfectly readable. The score is not scaled by this — it has a size of
// its own, in staff spaces, under View ▸ Zoom — because the two are
// different questions.
//
// Remembered for the person rather than kept in a score: it is a fact
// about the screen in front of them.
const UI_SCALES = [0.75, 0.9, 1, 1.15, 1.3, 1.5, 1.75, 2];

// fillUIScaleMenu writes the sizes into the menu from the one list of
// them, so that adding a size is adding it in one place.
function fillUIScaleMenu() {
  const box = $("#uiscale-menu");
  if (!box) return;
  box.innerHTML = "";
  for (const v of UI_SCALES) {
    const b = document.createElement("button");
    b.dataset.action = "ui-scale-" + Math.round(v * 100);
    const name = document.createElement("span");
    name.textContent = v === 1
      ? t("Designed size")
      : Math.round(v * 100) + "%";
    const tick = document.createElement("kbd");
    b.append(name, tick);
    b.addEventListener("click", () => {
      closeMenus();
      void setUIScale(v);
    });
    box.appendChild(b);
  }
  markUIScaleTicks();
}

// rememberedUIScale is the size to come up at.
//
// The settings first, where a desktop build keeps it. One is what they say
// when nobody has chosen — there is no telling that from a deliberate one
// — so the browser's own store answers for a page, which has no settings
// file behind it at all.
function rememberedUIScale(kept) {
  const said = Number(kept && kept.uiScale);
  if (said > 0 && said !== 1) return said;
  try {
    const mine = Number(localStorage.getItem("uiScale"));
    if (mine > 0) return mine;
  } catch (_) {
    // A private window may refuse to say.
  }
  // Nobody has chosen, so whatever this way of running gscore comes up
  // at: a page sets its own, the desktop build sets none. See
  // gscoreDefaults in web/boot.js.
  const page = Number(window.gscoreDefaults && window.gscoreDefaults.uiScale);
  if (page > 0) return page;
  return said > 0 ? said : 1;
}

// startingZoom is how big the score is drawn to begin with, in pixels to
// a staff space.
//
// The same question as the interface size and the same answer: a page
// comes up at what a page should come up at, the desktop build at what it
// always did, and a reader who moves the slider has moved it for good.
function startingZoom() {
  try {
    const mine = Number(localStorage.getItem("zoom"));
    if (mine >= 5 && mine <= 26) return mine;
  } catch (_) {
    // A private window may refuse to say.
  }
  const page = Number(window.gscoreDefaults && window.gscoreDefaults.zoom);
  if (page >= 5 && page <= 26) return page;
  return state.zoom;
}

function applyUIScale(v) {
  if (!(v > 0)) v = 1;
  state.uiScale = v;
  document.documentElement.style.setProperty("--ui-scale", String(v));
  markUIScaleTicks();
}

function markUIScaleTicks() {
  for (const b of $$("[data-action^='ui-scale-']")) {
    const want = Number(b.dataset.action.slice("ui-scale-".length)) / 100;
    const tick = b.querySelector("kbd");
    if (tick) tick.textContent = Math.abs(want - state.uiScale) < 1e-6 ? "✓" : "";
  }
}

// setUIScale changes it and remembers it.
//
// In two places, as the language is: the settings file, which is where a
// desktop build remembers things, and the browser's own store, which is
// the only one a page served from a web server has. Written together so
// they cannot disagree, and the page's store is what answers when the
// settings have nothing to say — a page has no disk to keep a file on, so
// without this the menus went back to their old size on every visit.
async function setUIScale(v) {
  const res = await fetch("/api/settings?uiScale=" + encodeURIComponent(v),
                          { method: "POST" }).catch(() => null);
  if (res && !res.ok) {
    status("Could not change the interface size: " + (await res.text()).trim());
    return;
  }
  try {
    localStorage.setItem("uiScale", String(v));
  } catch (_) {
    // A private window may refuse. The size still changes for this visit.
  }
  applyUIScale(v);
  // The menu panels are placed by script against the window, and the
  // window has not changed size — what changed is the pixel they are
  // measured in. Anything open is in the wrong place until it is put
  // there again.
  for (const sub of $$(".submenu.open")) placeSubmenu(sub);
  status(v === 1
    ? "Interface at its designed size"
    : `Interface at ${Math.round(v * 100)}%`);
}

// How long the pointer has to reach a submenu it opened, in milliseconds.
//
// Moving to another entry closes the open submenu, so that its panel is not
// left sitting over the item being reached for. That is right when the
// pointer is leaving and wrong while it is arriving, and the two look
// identical at the moment an entry is crossed — so the close waits to see
// which it was.
//
// It has to wait, because the journey crosses other entries. A submenu too
// tall for the room under it is pulled up the window to fit, which puts its
// panel above its own entry: the line from Insert ▸ Tab to the panel it
// just opened runs back over three plain items, and each of them shut it.
// The panel flashed open and was gone before the hand arrived. Reported as
// "I cannot open the submenus".
// The other half of the same journey: a submenu title the pointer is only
// passing over must not open. The Insert menu has six of them, so going up
// to a panel that was pulled above its entry crosses three or four, each
// opening itself and closing the one being reached for. Waiting for the
// pointer to settle is how every menu bar tells crossing from choosing.
const submenuGrace = 400;
const submenuHoverDelay = 180;
let closingSubmenus = null;
let openingSubmenu = null;

function shutSubmenus() {
  $$(".submenu").forEach(closeSubmenu);
}

// closeSubmenusSoon closes them unless the pointer arrives somewhere that
// says it was on its way.
function closeSubmenusSoon() {
  keepSubmenus();
  closingSubmenus = setTimeout(() => {
    closingSubmenus = null;
    shutSubmenus();
  }, submenuGrace);
}

// keepSubmenus calls off a close that has not happened yet.
function keepSubmenus() {
  if (closingSubmenus === null) return;
  clearTimeout(closingSubmenus);
  closingSubmenus = null;
}

// openSubmenuSoon opens one if the pointer is still on it a moment from
// now, which is what tells a choice from a pointer on its way past.
function openSubmenuSoon(sub) {
  cancelSubmenuOpen();
  openingSubmenu = setTimeout(() => {
    openingSubmenu = null;
    openSubmenu(sub);
  }, submenuHoverDelay);
}

// cancelSubmenuOpen calls off an open that has not happened yet: the
// pointer moved on, so it was passing over and not choosing.
function cancelSubmenuOpen() {
  if (openingSubmenu === null) return;
  clearTimeout(openingSubmenu);
  openingSubmenu = null;
}

// The panel belonging to each submenu, found wherever it is.
//
// It is not always where the markup put it: while a submenu is open its
// panel is parked on the body. See openSubmenu.
const flyouts = new WeakMap();

function panelOf(sub) {
  let panel = flyouts.get(sub);
  if (!panel) {
    panel = sub.querySelector(".menu-items");
    if (panel) flyouts.set(sub, panel);
  }
  return panel;
}

// openSubmenu shows one submenu and closes the rest.
//
// Exactly one is open at a time, and this is the only thing that decides it.
//
// The panel is lifted out of the menu and parked on the body for as long as
// it is open, and this is not tidiness: a menu too tall for the window
// scrolls, and WebKit clips a position: fixed element inside a scrolling
// box away to nothing. Nothing about that reads as clipping. The panel is
// open, it is display: block, it is where it should be on the page, and
// elementFromPoint at its middle answers with the panel — every way of
// asking says it is there, and the screen shows nothing. Hence a submenu
// that could not be opened in the Edit and Insert menus, which are the two
// long enough to scroll, and nowhere else and in no browser.
//
// On the body there is no scrolling ancestor to be clipped by, in any
// engine. Everything else about the panel stays as it was, its listeners
// with it.
function openSubmenu(sub) {
  // Opening one is arriving somewhere, so nothing is still on its way.
  keepSubmenus();
  cancelSubmenuOpen();
  $$(".submenu").forEach((s) => {
    if (s !== sub) closeSubmenu(s);
  });
  sub.classList.add("open");
  const panel = panelOf(sub);
  const park = flyoutHome();
  if (panel && park && panel.parentNode !== park) {
    panel.classList.add("flyout");
    park.appendChild(panel);
  }
  placeSubmenu(sub);
}

// closeSubmenu shuts one and puts its panel back where the markup has it.
//
// Back, rather than left on the body: the markup is where the panel
// belongs, and a page with eighteen stray panels on its body is one where
// nothing can be found by reading it.
function closeSubmenu(sub) {
  sub.classList.remove("open", "flip");
  const panel = flyouts.get(sub);
  if (panel && panel.parentNode === flyoutHome()) {
    panel.classList.remove("flyout");
    sub.appendChild(panel);
  }
}

// flyoutHome is where an open submenu's panel is parked.
//
// The body, clear of every box that could clip it or scale it. Both of
// those were tried: inside the menu it lives in, WebKit clips it away to
// nothing, and inside anything the interface scale is applied to, the
// panel is drawn where it belongs and the clicks on it go to whatever is
// behind — in Chromium at any scale but one, in WebKit at none.
//
// So it is parked where nothing is done to it, and the interface scale
// reaches it as type size rather than as a scaling of the whole box. See
// .menu-items.flyout in the stylesheet.
function flyoutHome() {
  return document.body;
}

// placeSubmenu puts a submenu where it can be read.
//
// The panel is fixed to the window rather than hung off the entry that owns
// it, because the menu it lives in scrolls and a panel inside a scrolling
// box is clipped by it. Fixed means nothing places it, so everything about
// where it goes is decided here.
//
// Level with its own entry and to the right of it; to the left where there
// is no room on the right, which there is not near the window edge; and up
// where it would run off the bottom, which a submenu near the foot of a
// long menu does. Cut, its entries were drawn and could not be reached, and
// nothing said they were there. Reported as "menus are sometimes cut".
function placeSubmenu(sub) {
  sub.classList.remove("flip");
  const items = panelOf(sub);
  if (!items) return;

  // Measured where it will be read: the panel is already shown by the time
  // this runs, and its size is what decides which side it goes on.
  //
  // Every measurement here is the window's own: the row it sits beside,
  // its own size, innerWidth and innerHeight. One kind of pixel
  // throughout, which is what keeps the arithmetic below honest.
  const row = sub.getBoundingClientRect();
  const box = items.getBoundingClientRect();
  const w = box.width;
  const h = box.height;

  // To the right, unless that runs off the edge. Then to the left — and if
  // there is no room either side, the right is still the better guess,
  // because the menu itself is at the left of the window.
  let left = row.right + 2;
  if (left + w > window.innerWidth - 8 && row.left - w - 2 >= 8) {
    left = row.left - w - 2;
    sub.classList.add("flip");
  }
  left = Math.max(8, Math.min(left, window.innerWidth - w - 8));

  // Level with the entry, pulled up by however much hangs over the foot,
  // and never above the top of the window: a panel taller than the window
  // scrolls, which its own max-height already arranges.
  let top = row.top - 5;
  const over = top + h - (window.innerHeight - 8);
  if (over > 0) top -= over;
  top = Math.max(8, top);

  // In the window's own pixels, which is what everything here is
  // measured in: the rectangles, innerWidth and innerHeight all answer in
  // those, and the panel is fixed to the window and scaled by nothing. Its
  // size grows with the interface because the type in it does, which is a
  // real layout and not a scaling of one — see .menu-items.flyout.
  items.style.left = left + "px";
  items.style.top = top + "px";
}

async function action(name) {
  // The files opened before are a list that changes, so their commands are
  // numbered rather than named. Everything that drives the editor comes
  // through here — the page's menus and the application's — so they have to
  // be reachable by name like the rest.
  if (name.startsWith("open-recent-")) {
    await openRecent(recentPaths[Number(name.slice("open-recent-".length))]);
    return;
  }
  // The scripts in the plugins folder, which are a list that changes for the
  // same reason: they are named by their file, so that two plugins calling
  // themselves the same thing are still two commands.
  if (name.startsWith("plugin:")) {
    await runPlugin(name.slice("plugin:".length));
    return;
  }
  // And the parts, which are a list that changes because the score says
  // what is in it.
  if (name.startsWith("show-part-")) {
    await showSheet(Number(name.slice("show-part-".length)));
    return;
  }
  switch (name) {
    case "new":
      await fetch("/api/new", { method: "POST" });
      await reopen();
      status("New score");
      break;
    case "open":
      await openScore();
      break;
    case "save-pdf":
      await exportPDF();
      break;
    case "save-svg":
      await exportSVG();
      break;
    case "save-page":
      await exportPage();
      break;
    case "save-midi":
      await exportMIDI();
      break;
    case "save-musicxml":
      await exportAs("musicxml", "application/vnd.recordare.musicxml+xml", "MusicXML");
      break;
    case "save-mxl":
      await exportAs("mxl", "application/vnd.recordare.musicxml", "compressed MusicXML");
      break;
    case "save-abc":
      await exportAs("abc", "text/vnd.abc", "ABC");
      break;
    case "save-lilypond":
      await exportAs("ly", "text/plain", "LilyPond");
      break;
    case "save-guido":
      await exportAs("gmn", "text/plain", "GUIDO");
      break;
    case "save-audio":
      await recordAudio();
      break;
    case "clear-recent":
      await fetch("/api/recent?clear=1", { method: "POST" });
      await loadRecent();
      status("The list of recent files was cleared");
      break;
    case "playback-sound":
      await playbackSound();
      break;
    case "sound-delay":
      await soundDelayDialog();
      break;
    case "print":
      await printScore();
      break;
    case "zoom-in":
      setZoom(state.zoom + 1);
      break;
    case "zoom-out":
      setZoom(state.zoom - 1);
      break;
    case "zoom-reset":
      setZoom(8);
      break;
    case "fullscreen":
      await toggleFullscreen();
      break;
    case "show-all-tunes":
      state.showAll = !state.showAll;
      state.showAllDecided = true;
      $("#show-all").checked = state.showAll;
      await drawScore();
      status(state.showAll ? "Showing every tune" : "Showing one tune");
      break;
    case "mode-page":
    case "mode-strip":
      state.mode = name === "mode-strip" ? "strip" : "page";
      markMode();
      // Told to the core as well, which has always been sent the mode with
      // each drawing but never kept it. It keeps it now so the score can
      // remember which way it was being read, and saving the file keeps
      // that with it.
      void fetch(`/api/view?mode=${state.mode}`, { method: "POST" })
        .catch(() => { /* the drawing still carries the mode */ });
      await drawScore();
      await loadHits();
      status(state.mode === "strip" ? "Continuous strip" : "Paginated");
      break;
    case "play":
      // The menus say Play / Pause and the space bar does the same thing, so
      // this is the same one function and not a second opinion about it.
      void togglePlay();
      break;
    case "play-from-start":
      await playFromStart();
      break;
    case "stop":
      stop();
      break;
    case "metronome":
      setMetronome(!state.metronome);
      break;
    case "type-music": toggleTypeBar(); break;
    case "typing-help": openTypingHelp(); break;
    case "show-score": await showSheet("score"); break;
    case "part-setup": await partSetupDialog(); break;
    case "make-parts": await makeParts(); break;
    case "console": await openConsole(); break;
    case "note-input":
      setMouseWrites(!state.mouseWrites);
      break;
    case "insert-note":
      await insertAtSelection(false);
      break;
    case "insert-rest":
      await insertAtSelection(true);
      break;
    case "split-measure":
      await splitMeasure();
      break;
    case "add-measure":
      await fetch("/api/addmeasure", { method: "POST" });
      await reload();
      status("Measure added");
      break;
    case "insert-measures": await insertMeasuresDialog(); break;
    case "remove-measure": await removeMeasure(); break;
    case "flute-chart": await fluteDialog(); break;
    case "delete":
      await deleteSelected();
      break;
    case "undo": await stepHistory("undo"); break;
    case "redo": await stepHistory("redo"); break;

    case "copy": await copySelection(); break;
    case "cut": await cutSelection(); break;
    case "paste": await pasteClipboard(); break;

    case "slur": await setSlur("on"); break;
    case "slur-none": await setSlur("none"); break;
    case "tie": await setTie("on", false); break;
    case "tie-head": await setTie("on", true); break;
    case "tie-none": await setTie("none", false); break;

    case "harmonics": await openHarmonics(); break;
    case "range": await openRange(); break;
    case "keymap": await openKeyMap(); break;
    case "scoremap": await openScoreMap(); break;
    case "voices": await openVoiceLeading(); break;
    case "spacing": await openSpacing(); break;
    case "rhythm": await openRhythm(); break;
    case "melody": await openMelody(); break;

    case "beam-join": await mark("beam", { beam: "join" }, "Beamed together"); break;
    case "beam-here": await beamOrBreak(); break;
    case "beam-break": await mark("beam", { beam: "break" }, "Beam broken"); break;
    case "beam-auto": await mark("beam", { beam: "auto" }, "Beaming left to the engraver"); break;

    case "pitch-up": await nudge(1); break;
    case "pitch-down": await nudge(-1); break;
    case "octave-up": await nudge(7); break;
    case "octave-down": await nudge(-7); break;

    case "acc-sharp": await setAccidental("sharp", "Sharp"); break;
    case "acc-flat": await setAccidental("flat", "Flat"); break;
    case "shortcuts": await shortcutsDialog(); break;
    case "acc-natural": await setAccidental("natural", "Natural"); break;
    case "acc-dsharp": await setAccidental("double sharp", "Double sharp"); break;
    case "acc-dflat": await setAccidental("double flat", "Double flat"); break;
    case "acc-parens": await bracketAccidental(); break;
    case "hairpin-cresc": await setHairpin("crescendo", "Crescendo"); break;
    case "hairpin-dim": await setHairpin("diminuendo", "Diminuendo"); break;
    case "hairpin-none": await setHairpin("none", "Hairpin removed"); break;

    case "octave-8va": await setOctave("8va", "8va"); break;
    case "octave-8vb": await setOctave("8vb", "8vb"); break;
    case "octave-15ma": await setOctave("15ma", "15ma"); break;
    case "octave-15mb": await setOctave("15mb", "15mb"); break;
    case "octave-none": await setOctave("none", "Octave line removed"); break;

    case "staff-end":
      await goToStaffEnd();
      break;
    case "jump": await jumpToMeasure(); break;

    case "tuplet-3": await setTuplet(3); break;
    case "tuplet-4": await setTuplet(4); break;
    case "tuplet-5": await setTuplet(5); break;
    case "tuplet-6": await setTuplet(6); break;
    case "tuplet-7": await setTuplet(7); break;
    case "tuplet-8": await setTuplet(8); break;
    case "tuplet-9": await setTuplet(9); break;
    case "tuplet-other": {
      const n = await ask({
        title: "Tuplet",
        label: "How many notes in the time of one",
        value: "13",
        note: "Two to sixteen. A cadenza marked ad libitum may run thirteen " +
              "into a beat, and the figure is printed over the group.",
      });
      if (n === null) break;
      const want = parseInt(String(n).trim(), 10);
      if (!Number.isFinite(want) || want < 2 || want > 16) {
        status("A tuplet is between 2 and 16 notes");
        break;
      }
      await setTuplet(want);
      break;
    }
    case "tuplet-none": await setTuplet(0); break;

    case "tremolo-1": await setTremolo(1, false); break;
    case "tremolo-2": await setTremolo(2, false); break;
    case "tremolo-3": await setTremolo(3, false); break;
    case "tremolo-4": await setTremolo(4, false); break;
    case "tremolo-pair-1": await setTremolo(1, false, true); break;
    case "tremolo-pair-2": await setTremolo(2, false, true); break;
    case "tremolo-pair-3": await setTremolo(3, false, true); break;
    case "tremolo-buzz": await setTremolo(0, true); break;
    case "tremolo-none": await setTremolo(0, false); break;

    case "ending-1": await setEnding("1"); break;
    case "ending-2": await setEnding("2"); break;
    case "ending-12": await setEnding("1,2"); break;
    case "ending-none": await setEnding(""); break;

    case "bar-single": await setBarline("single"); break;
    case "bar-double": await setBarline("double"); break;
    case "bar-final": await setBarline("final"); break;
    case "bar-open": await setBarline("repeat-start"); break;
    case "bar-close": await setBarline("repeat-end"); break;
    case "stem-up": await setStem("up"); break;
    case "stem-down": await setStem("down"); break;
    case "stem-auto": await setStem("auto"); break;

    case "dyn-pppp": await setDynamic("pppp"); break;
    case "dyn-ppp": await setDynamic("ppp"); break;
    case "dyn-pp": await setDynamic("pp"); break;
    case "dyn-p": await setDynamic("p"); break;
    case "dyn-mp": await setDynamic("mp"); break;
    case "dyn-mf": await setDynamic("mf"); break;
    case "dyn-f": await setDynamic("f"); break;
    case "dyn-ff": await setDynamic("ff"); break;
    case "dyn-fff": await setDynamic("fff"); break;
    case "dyn-none": await setDynamic("none"); break;

    case "artic-none": await setArticulation(""); break;
    case "artic-staccato": await setArticulation("staccato"); break;
    case "artic-staccatissimo": await setArticulation("staccatissimo"); break;
    case "artic-tenuto": await setArticulation("tenuto"); break;
    case "artic-accent": await setArticulation("accent"); break;
    case "artic-marcato-up": await setArticulation("marcato"); break;
    case "artic-marcato-down": await setArticulation("marcato down"); break;
    case "breath-none": await setBreath("none"); break;
    case "breath-comma": await setBreath("comma"); break;
    case "breath-tick": await setBreath("tick"); break;
    case "breath-caesura": await setBreath("caesura"); break;
    case "breath-caesura-curved": await setBreath("curved caesura"); break;

    // Piano writing: the note is drawn on the staff next door and
    // nothing about the music changes.
    case "cross-below": await setCrossStaff("below"); break;
    case "cross-above": await setCrossStaff("above"); break;
    case "cross-own": await setCrossStaff("own"); break;

    // How a fretted instrument plays the note. Written out one by one
    // rather than matched by their prefix, so that the entry on the menu
    // and the case that answers it can be read against each other.
    case "tab-hammer-on": await setTechnique("hammer-on"); break;
    case "tab-pull-off": await setTechnique("pull-off"); break;
    case "tab-slide-up": await setTechnique("slide-up"); break;
    case "tab-slide-down": await setTechnique("slide-down"); break;
    case "tab-bend": await setTechnique("bend"); break;
    case "tab-bend-half": await setTechnique("bend-half"); break;
    case "tab-bend-release": await setTechnique("bend-release"); break;
    case "tab-pre-bend": await setTechnique("pre-bend"); break;
    case "tab-vibrato": await setTechnique("vibrato"); break;
    case "tab-wide-vibrato": await setTechnique("wide-vibrato"); break;
    case "tab-palm-mute": await setTechnique("palm-mute"); break;
    case "tab-let-ring": await setTechnique("let-ring"); break;
    case "tab-natural-harmonic": await setTechnique("natural-harmonic"); break;
    case "tab-artificial-harmonic": await setTechnique("artificial-harmonic"); break;
    case "tab-pinch-harmonic": await setTechnique("pinch-harmonic"); break;
    case "tab-tap": await setTechnique("tap"); break;
    case "tab-dead-note": await setDeadNote(); break;
    case "tab-none": await setTechnique(""); break;

    case "set-pickup":
      await pickupDialog();
      break;
    case "score-options":
      await engravingDialog();
      break;
    case "set-numbering":
      await numberingDialog();
      break;
    case "set-systems":
      await systemsDialog();
      break;
    case "find-text":
      await findDialog();
      break;
    case "select-all":
      selectAll();
      break;
    case "select-region":
      await selectRegionDialog();
      break;
    case "transpose":
      await transposeDialog();
      break;
    case "set-tempo":
      await setTempoDialog();
      break;
    case "set-header":
      await setHeaderDialog();
      break;
    case "set-key":
      await openKeyPicker(state.selection?.part ?? 0);
      break;
    case "set-time":
      await setTimeDialog();
      break;
    case "staff-lines":
      await setStaffLinesDialog(state.selection?.part ?? 0);
      break;
    case "set-instrument":
      await setInstrumentDialog();
      break;
    case "add-staff": await addStaffDialog(); break;
    case "add-grand-staff": await addGrandStaff(); break;
    case "remove-staff": await removeStaff(); break;
    case "hide-staff": await toggleStaffShown(); break;
    case "show-all-staves": await showAllStaves(); break;
    case "staff-up": await moveStaff(-1); break;
    case "staff-down": await moveStaff(1); break;
    case "page-setup": await pageSetupDialog(); break;
    case "midi-input": await midiDialog(); midiTick(); break;
    case "tab": await tabDialog(); break;
    case "chord-builder": await chordBuilderDialog(); break;
    case "tab-transposition": await tabMoveDialog(); break;
    case "string-up": await moveToString("up"); break;
    case "string-down": await moveToString("down"); break;

    case "repeat-bar": await setMeasureRepeat(1); break;
    case "repeat-2bars": await setMeasureRepeat(2); break;
    case "repeat-none": await setMeasureRepeat(0); break;

    case "set-lyrics":
      await lyricsDialog();
      break;
    case "fingering":
      toggleFingering();
      break;
    case "fingering-off":
      fingering.on = false;
      $("#fingering-group").hidden = true;
      markFingeringMenu();
      status("Ready");
      break;

    case "view-names": await setView("names", !state.view.names); break;
    case "view-numbers": await setView("numbers", !state.view.numbers); break;
    case "view-multirest": await setView("multirest", !state.view.multirest); break;
    case "view-tempomark": await setView("tempomark", !state.view.tempomark); break;
    case "view-beatmarks": await setView("beatmarks", !state.view.beatmarks); break;
    case "note-colours": await noteColoursDialog(); break;
    case "view-concert": await setView("concert", !state.view.concert); break;

    case "insert-chord": await chordSymbolDialog(); break;
    case "insert-text": await textDialog(); break;
    case "insert-marker": await markerDialog(); break;
    case "marker-style": await markerStyleDialog(); break;
    case "score-setup": await scoreSetupDialog(); break;

    case "group-brace": await joinStaves("brace"); break;
    case "group-bracket": await joinStaves("bracket"); break;
    case "group-none": await joinStaves("none"); break;
    case "save":
      await saveScore(false);
      break;
    case "save-as":
      await saveScore(true);
      break;
    case "settings":
      await settingsDialog();
      break;
    case "track-barlines":
      await setTrackBarlines(!state.trackBarlines);
      break;
    case "import-symbols":
      await importSymbolPack();
      break;
    case "export-symbols":
      await exportSymbolPack();
      break;
    case "draw-symbol":
      drawInit(draw.size);
      $("#draw").showModal();
      break;
    case "symbol-palette":
      await openPalette();
      break;
    case "report-bug":
      window.open("https://github.com/stricaud/gscore/issues", "_blank");
      break;
    case "import-log":
      showImportLog();
      break;
    case "about":
      $("#about").showModal();
      break;
  }
}

// showImportLog lists the lines the importer had to pass over.
//
// Reading a hand-written tune book means reading past the mistakes in it, and
// a reader that does that silently is a reader you cannot trust. This is where
// it owns up: one line per line it could not read, so the file can be fixed
// where it is wrong rather than the score being wrong where nobody looks.
function showImportLog() {
  const list = $("#import-log-list");
  list.textContent = "";
  if (!importLog.length) {
    const li = document.createElement("li");
    li.textContent = "Every line of this file was read.";
    list.append(li);
  }
  for (const line of importLog) {
    const li = document.createElement("li");
    li.textContent = line;
    list.append(li);
  }
  $("#import-log").showModal();
}

function markMode() {
  const strip = $('[data-action="mode-strip"] kbd');
  const page = $('[data-action="mode-page"] kbd');
  if (strip) strip.remove();
  if (page) page.remove();
  const target = $(`[data-action="mode-${state.mode === "strip" ? "strip" : "page"}"]`);
  const tick = document.createElement("kbd");
  tick.textContent = "✓";
  target.appendChild(tick);
}

/* ---------- score ---------- */

// viewParams says which picture is being asked about: which tune, how big a
// staff space is, strip or pages, how wide a system may be.
//
// Shared, because two routes answer questions about the same picture — what
// it looks like and how big it is — and a size that belongs to a different
// picture sends the editor to the wrong place in the music.
function viewParams(opts = {}) {
  const p = new URLSearchParams({ space: String(opts.space ?? state.zoom) });
  if (opts.tune !== undefined) p.set("tune", String(opts.tune));
  // Always say which, rather than naming the strip and letting silence mean
  // the other one. The server starts in the strip, so an unnamed mode meant
  // the strip too — and choosing Paginated, which said nothing, drew the
  // strip again. The menu ticked over and the page did not move.
  const mode = opts.mode ?? state.mode;
  p.set("mode", mode === "strip" ? "strip" : "page");
  if (opts.width) p.set("width", String(opts.width));
  // How much room there is on the screen, in pixels. The server turns
  // that into a system width, because the margins it has to take off are
  // an engraving setting and it is the one that knows them.
  const fit = playerWidth();
  if (fit) p.set("fit", String(fit));
  // A player shows the music, not the name of the piece: the page around
  // it has said that already, in its own type, and the title block
  // engraved inside the frame takes the top of it. What a reader saw
  // first was a title they had just read, with the music below the fold.
  // The title goes to the embedding page instead — see announceToParent.
  if (state.player) p.set("title", "0");
  return p;
}

// playerWidth is how wide a system may be when the score is embedded, in
// staff spaces.
//
// A frame in a page is a few hundred pixels wide and the music has to
// fit it: drawn as one unbroken strip, a tune of forty bars is forty
// bars wide and the reader sees the first two with a scrollbar under
// them. Wrapped to the frame, the whole tune is on the screen.
//
// Zero when gscore is not embedded, which leaves the layout as it was:
// the editor scrolls, and its strip is the point of it.
function playerWidth() {
  if (!state.player || state.mode === "strip") return 0;
  const room = $("#canvas-wrap");
  const px = (room ? room.clientWidth : window.innerWidth) - 16;
  return Math.max(160, Math.round(px));
}

function scoreURL(opts = {}) {
  const p = viewParams(opts);
  // A stretch of the score rather than all of it, and whether to send the
  // shapes alone to go inside a drawing the page already has. See tilePlan.
  const w = opts.window;
  if (w) {
    p.set("x0", w.x0.toFixed(2));
    p.set("y0", w.y0.toFixed(2));
    p.set("x1", w.x1.toFixed(2));
    p.set("y1", w.y1.toFixed(2));
  }
  if (opts.fragment) p.set("frag", "1");
  p.set("_", String(Date.now()));
  return "/score.svg?" + p.toString();
}

/* ---------- drawing a long score a screenful at a time ---------- */

// A browser is given the shapes it can see, and a screenful either way, and
// not the rest.
//
// A big orchestral score is not a big picture, it is a great many small
// ones: Beethoven's ninth is fifty-three staves of two and a half thousand
// bars and comes to nine hundred thousand shapes. Handed all of them, the
// browser spends seconds laying them out, keeps hundreds of megabytes, and
// then walks the lot on every repaint to work out what is on screen — so
// scrolling stutters and playing stops dead while it catches up. It is the
// same problem a list widget has with a million rows, and the same answer:
// put the rows you can see in the page and fetch the others as they come
// into view.
//
// A score that fits is drawn whole and nothing below does anything, which
// is every ordinary one.

// drawPad is how far beyond the view to draw, as a fraction of the view: a
// screenful past each edge, so that ordinary scrolling never reaches the end
// of what is drawn before the next stretch has arrived.
const drawPad = 1.0;

// drawSlack is how much of that margin may be used up before the next
// stretch is fetched. A quarter of a screen in hand is enough to cover the
// fetch, and redrawing at a quarter rather than at the edge means a scroll
// of one screenful costs one fetch rather than several.
const drawSlack = 0.25;

// windowArea is how much music goes in the page at once, in square staff
// spaces.
//
// In the score's own units rather than in screenfuls, because what a browser
// minds is how many shapes it is holding, and that is a property of the
// music and not of how far it is zoomed in. Measured in screenfuls the same
// score wanted cutting up differently at every zoom, and the seam moved
// about with it.
//
// Beethoven's ninth comes to about thirty-five square spaces per shape, so
// this is some fifteen thousand of them — comfortable for a browser, and
// about a twentieth of one per cent of that score. A flute study is a third
// of it and is handed over whole.
const windowArea = 550000;

// viewBox is the stretch of the score on screen, in staff spaces.
function viewBox() {
  const wrap = $("#canvas-wrap");
  if (!wrap) return null;
  const z = state.zoom || 8;
  let x0 = 0, y0 = 0;
  const svg = currentSVG();
  if (svg) {
    // Measured against the drawing rather than from the scroll offsets,
    // because the paper is inset from the edge of the panel and the drawing
    // is where the coordinates in it are measured from.
    const wr = wrap.getBoundingClientRect();
    const sr = svg.getBoundingClientRect();
    x0 = (wr.left - sr.left) / z;
    y0 = (wr.top - sr.top) / z;
  }
  return {
    x0, y0,
    x1: x0 + wrap.clientWidth / z,
    y1: y0 + wrap.clientHeight / z,
  };
}

// The drawing is one <svg> the size of the whole score, filled with pieces.
//
// A piece is a stretch of the long side — a few bars of the strip, a page or
// two of the paginated view — drawn as a <g> and put inside the drawing that
// is already there. Moving along adds the piece ahead and drops the one left
// behind, so a scroll costs one piece rather than everything in view, and
// the cursor, the selection and the lit noteheads are not swept away and
// rebuilt every time the view moves.
//
// Rebuilding the lot was the first way this worked and it showed: every few
// seconds of scrolling the whole of what was on screen was thrown away and
// parsed again.

// tilePad is how far beyond the view to keep drawn, in screenfuls. One
// either way means an ordinary scroll never reaches an edge that is not
// already there.
const tilePad = 1;

// tileKeep is how far beyond that to keep a piece before dropping it.
// Scrolling back a little should not have to fetch what was just there.
const tileKeep = 3;

// tilePlan says how the score is cut up, or null to draw the whole of it in
// one piece — which is every ordinary score.
//
// Along the long side, because that is the one being windowed: the editing
// strip is cut across its length and a paginated score down its length. The
// short side is taken whole, so moving along it never fetches anything.
function tilePlan(size) {
  const vis = viewBox();
  if (!vis || !size || !(size.width > 0) || !(size.height > 0)) return null;
  const viewW = vis.x1 - vis.x0;
  const viewH = vis.y1 - vis.y0;
  if (!(viewW > 0) || !(viewH > 0)) return null;
  if (size.width * size.height <= windowArea) return null;

  // Which side to cut, and how much of the other side one piece may hold
  // before it is too much to hand a browser at once.
  const downTheLength = size.height > size.width;
  const across = downTheLength ? size.width : size.height;
  const view = downTheLength ? viewH : viewW;

  // A piece is a screenful of the long side, or less where the short side
  // is so deep that a screenful of both would be too much at once.
  let span = view;
  if (across > 0 && span * across > windowArea) span = windowArea / across;
  if (!(span > 0)) return null;

  return { downTheLength, span, across, view };
}

// tileBox is the stretch of score one piece covers.
function tileBox(plan, size, i) {
  const a = i * plan.span;
  const b = a + plan.span;
  if (plan.downTheLength) {
    return { x0: 0, y0: a, x1: size.width, y1: Math.min(size.height, b) };
  }
  return { x0: a, y0: 0, x1: Math.min(size.width, b), y1: size.height };
}

// tileRange is the pieces covering the view with a margin round it, in
// screenfuls.
function tileRange(plan, size, margin) {
  const vis = viewBox();
  if (!vis) return null;
  const from = plan.downTheLength ? vis.y0 : vis.x0;
  const to = plan.downTheLength ? vis.y1 : vis.x1;
  const pad = plan.view * margin;
  const end = plan.downTheLength ? size.height : size.width;
  const lo = Math.max(0, Math.floor((from - pad) / plan.span));
  const hi = Math.min(Math.ceil(end / plan.span) - 1,
                      Math.floor((to + pad) / plan.span));
  return { lo, hi };
}

// loadSize asks how big the engraved score is, in staff spaces, for the view
// being shown. It is what decides whether the score needs any of this.
async function loadSize() {
  try {
    const res = await fetch("/api/layout?" + viewParams().toString());
    if (!res.ok) return null;
    const d = await res.json();
    if (!(d.width > 0) || !(d.height > 0)) return null;
    return d;
  } catch (_) {
    // Served without the route behind it. The whole score is drawn, as
    // before.
    return null;
  }
}

// The pieces on the page, and the ones already asked for.
//
// tileText holds what has come back but may not be on the page yet: the
// piece ahead is asked for before it is wanted, so that arriving at it costs
// only the drawing of it.
let tileEls = new Map();
let tileText = new Map();
// Pieces already turned into shapes and waiting to be put on the page.
//
// Turning a piece of a long score into shapes is the expensive half of
// drawing it: a screenful of Beethoven's ninth is some hundred and eighty
// thousand characters of markup and a few thousand shapes, and the browser
// does that work on the one thread that also moves the music. Done at the moment the piece is
// wanted, it lands in the middle of playing and the scrolling stops dead
// for as long as it takes.
//
// So it is done early, while nothing else is happening, and what is left
// for the moment the piece is wanted is putting it on the page.
let tileReady = new Map();
let tileHome = null;

// forgetTiles throws the drawing away, which is what a new score, a new
// zoom or a new view needs.
function forgetTiles() {
  tileEls = new Map();
  tileText = new Map();
  tileReady = new Map();
  tileHome = null;
  drawnAgain();
}

// fetchTile asks for one piece, once. The answer is kept, so asking for it
// again while it is in the air waits for the same one.
function fetchTile(plan, size, i) {
  const got = tileText.get(i);
  if (got) return got;
  const box = tileBox(plan, size, i);
  const p = fetch(scoreURL({ window: box, fragment: true }))
    .then((res) => (res.ok ? res.text() : ""))
    .catch(() => "");
  tileText.set(i, p);
  return p;
}

// buildTile turns one piece of markup into shapes, off the page.
//
// Off the page on purpose: a subtree built on its own costs nothing to make
// beyond the making, where the same markup parsed into the drawing makes
// the browser lay the whole thing out again as it goes.
function buildTile(text, i) {
  const g = document.createElementNS("http://www.w3.org/2000/svg", "g");
  g.setAttribute("data-tile", String(i));
  g.innerHTML = text;
  return g;
}

// prepareTile turns a piece into shapes before anybody needs it.
async function prepareTile(plan, size, i) {
  if (tileReady.has(i) || tileEls.has(i)) return;
  const text = await fetchTile(plan, size, i);
  if (!text) return;
  // Asked for in earnest while we waited, or thrown away.
  if (tileReady.has(i) || tileEls.has(i) || !tileHome) return;
  tileReady.set(i, buildTile(text, i));
}

// showTile puts one piece on the page.
async function showTile(plan, size, i) {
  if (tileEls.has(i)) return;

  // Already turned into shapes while nothing was happening, which is the
  // whole point: what is left to do now is one append.
  const ready = tileReady.get(i);
  if (ready) {
    tileReady.delete(i);
    tileEls.set(i, ready);
    tileHome.appendChild(ready);
    drawnAgain();
    return;
  }

  // Claimed before it is awaited, so two passes over the same piece do not
  // both draw it.
  tileEls.set(i, null);
  const text = await fetchTile(plan, size, i);
  if (!tileHome || !tileHome.isConnected) return; // the drawing has gone
  if (tileEls.get(i) !== null) return;            // forgotten while we waited
  const g = buildTile(text, i);
  tileHome.appendChild(g);
  tileEls.set(i, g);
  drawnAgain();
}

/*
 * When the drawing last changed under whatever is reading it.
 *
 * A long score is not one picture: it is drawn a piece at a time into one
 * SVG, and the pieces come and go as the view moves. Anything that files
 * the shapes away — the index that finds a notehead by its number, so that
 * the note being played can be lit — has to know that the shapes it filed
 * are not the shapes on the page any more.
 *
 * It was watching the SVG element instead, which never changes: the pieces
 * are added inside it. So on a score long enough to be tiled, the index was
 * built once and never again, and every note drawn after that moment lit
 * nothing. On Beethoven's ninth that is everything past the first screenful.
 */
let drawGen = 0;

function drawnAgain() {
  drawGen++;
}

// ensureTiles draws what the view needs, drops what it has left well behind,
// and asks for what it is about to want.
async function ensureTiles() {
  const plan = state.tiles;
  const size = state.size;
  if (!plan || !size || !tileHome) return;

  const want = tileRange(plan, size, tilePad);
  if (!want) return;

  // Away from the middle outwards, so that what is under the eye arrives
  // first when several are missing at once.
  const mid = Math.round((want.lo + want.hi) / 2);
  const order = [];
  for (let i = want.lo; i <= want.hi; i++) order.push(i);
  order.sort((a, b) => Math.abs(a - mid) - Math.abs(b - mid));
  for (const i of order) await showTile(plan, size, i);

  // What is far enough behind to be worth the room it takes.
  const keep = tileRange(plan, size, tileKeep);
  for (const [i, g] of [...tileEls]) {
    if (keep && i >= keep.lo && i <= keep.hi) continue;
    if (g) g.remove();
    tileEls.delete(i);
    tileText.delete(i);
    drawnAgain();
  }

  // Pieces turned into shapes and then left far behind are shapes nobody
  // is going to look at.
  for (const i of [...tileReady.keys()]) {
    if (keep && i >= keep.lo && i <= keep.hi) continue;
    tileReady.delete(i);
  }

  // And the next one along each way, turned into shapes but not put on the
  // page, so that reaching it is one append and not a megabyte of parsing
  // in the middle of the music.
  //
  // When there is a moment, not now: this is called from the scrolling, and
  // doing it here would be the very stall it is there to prevent.
  for (const i of [want.lo - 1, want.hi + 1]) {
    if (i < 0 || tileReady.has(i) || tileEls.has(i)) continue;
    const end = plan.downTheLength ? size.height : size.width;
    if (i * plan.span >= end) continue;
    whenIdle(() => void prepareTile(plan, size, i));
  }
}

// whenIdle runs something in a gap between frames.
//
// A fallback for where there are no idle callbacks: a timeout is not the
// same promise — it will run whether there is a gap or not — but it does at
// least leave the frame it was asked in, which is most of the benefit.
function whenIdle(fn) {
  if (typeof requestIdleCallback === "function") {
    requestIdleCallback(fn, { timeout: 2000 });
    return;
  }
  setTimeout(fn, 0);
}

/* ---------- the end of a staff ---------- */

// A score opens at its beginning, the way a document does. Somebody who
// opened it to carry on writing wants the end of a staff, and on a score of
// fifty-three staves the end of *which* staff is a real question — so there
// is one of these per staff, down the left of the music, and it says which.
//
// Stuck to the edge of the panel rather than drawn into the margin of the
// score, so that they are still reachable after the view has moved along.

// sprintRows is one entry per staff on the page: where it is, and what it
// is called.
function sprintRows() {
  const seen = new Map();
  for (const h of state.hits || []) {
    if (seen.has(h.part)) continue;
    seen.set(h.part, {
      part: h.part,
      top: h.staffTop,
      height: staffHeightOf(h),
      name: (state.info?.parts || [])[h.part]?.name || `Staff ${h.part + 1}`,
    });
  }
  return [...seen.values()].sort((a, b) => a.top - b.top);
}

// staffEntry is the bar a staff first has a note in.
//
// Not the first bar it has: an orchestral staff begins with rests, and on
// the Rossini the piccolo holds twenty-eight of them before it plays. The
// bar somebody wants to be taken to is the one the instrument comes in at.
//
// Null for a staff that never plays — a part somebody has added and not
// written yet, and a staff of nothing but rests through a whole movement.
function staffEntry(part, top) {
  let best = null;
  for (const h of state.hits || []) {
    if (h.part !== part) continue;
    if (top != null && Math.abs(h.staffTop - top) > 0.01) continue;
    if (best && h.measure >= best.measure) continue;
    // A note or a chord. A rest is the staff counting, not playing, and a
    // symbol is something written over the music rather than in it.
    const plays = (h.elements || []).some(
      (el) => el.kind === "note" || el.kind === "chord");
    if (plays) best = h;
  }
  return best;
}

// What is on the page now, so that scrolling only moves the buttons rather
// than building them again.
let sprintKey = "";

function drawSprints() {
  const box = $("#staff-sprints");
  if (!box) return;
  const wrap = $("#canvas-wrap");
  const svg = currentSVG();
  // Only where there is somewhere to sprint to. A score that already fits
  // across the panel has its end on screen, and a row of buttons pointing
  // at it would be furniture.
  const far = wrap && (state.size?.width || 0) * state.zoom > wrap.clientWidth * 1.5;
  // And only at the beginning of the staff.
  //
  // They are a way of leaving the start: once the view has moved along,
  // they are pointing at where you already are and are in front of the
  // music. Scrolling back to the beginning brings them back.
  const atStart = wrap && wrap.scrollLeft <= 16;
  if (!wrap || !svg || !far || !atStart || state.mode !== "strip" || state.readOnly) {
    // Only where it is not already away. This runs on every frame of every
    // scroll, and writing to the page to say what it already says is still
    // writing to the page — which while the music plays is sixty needless
    // touches a second.
    if (!box.hidden) {
      box.hidden = true;
      box.textContent = "";
      sprintKey = "";
    }
    return;
  }
  box.hidden = false;

  const rows = sprintRows();
  const key = rows.map((r) => r.part + "@" + r.top.toFixed(2)).join(",");
  if (key !== sprintKey) {
    sprintKey = key;
    box.textContent = "";
    for (const r of rows) {
      const entry = staffEntry(r.part, r.top);
      const b = document.createElement("button");
      b.type = "button";
      b.className = "staff-sprint";
      b.dataset.name = r.name;
      // Which bar, in the tooltip, because that is the question somebody
      // is asking when they look at a staff of rests: not "take me
      // somewhere" but "when does this come in".
      //
      // A staff with no notes at all has no answer to that, so it gives
      // the other one: the end, which is where writing it would start.
      const what = entry
        ? r.name + " comes in at measure " + barNumberOf(entry.measure)
        : "Go to the end of " + r.name + ", which has no notes yet";
      b.title = what;
      b.setAttribute("aria-label", what);
      // A note: go to where this instrument plays.
      b.textContent = "♪";
      b.addEventListener("click", () => void sprintToStaffEntry(r.part, r.top));
      box.appendChild(b);
    }
  }
  placeSprints(rows, box, svg, wrap);
}

// What the name strip is showing now, so that scrolling moves the labels
// rather than building them again.
let staffNameKey = "";

// drawStaffNames keeps each staff's name beside it once the names written
// at the head of the score have scrolled away.
//
// A score of twenty-three staves is read across, and the only thing saying
// which line belongs to which instrument is a column of words at the far
// left. Four screens along, finding out what you are looking at means
// going all the way back and all the way out again — on the William Tell
// overture that is four hundred and seventy-seven bars of travel to read
// one word.
//
// So the names follow. They are the other half of the sprint buttons above
// and the mirror of them: those are a way of leaving the beginning and
// appear only there, these are for everywhere else and appear as soon as
// the real names are gone. Neither is ever on screen at the same time as
// the thing it stands in for.
function drawStaffNames() {
  const box = $("#staff-names");
  if (!box) return;
  const wrap = $("#canvas-wrap");
  const svg = currentSVG();
  // Only where the names have actually gone. At the head of the score they
  // are on the page already, and a second copy over the first is clutter.
  const away = wrap && wrap.scrollLeft > 16;
  // And only where the score is naming its staves at all.
  //
  // View, Staff names takes the words off the page, and somebody who has
  // just taken them off does not want them back three bars later in a
  // different shape. These stand in for the names at the head of the
  // score; where there are none to stand in for, there is nothing to do.
  const named = state.view?.names !== false;
  if (!wrap || !svg || !away || !named || state.mode !== "strip") {
    // Only where it is not already away: this runs on every frame of every
    // scroll, and writing to the page to say what it already says is still
    // writing to the page.
    if (!box.hidden) {
      box.hidden = true;
      box.textContent = "";
      staffNameKey = "";
    }
    return;
  }
  box.hidden = false;

  // A staff that is not called anything is not labelled.
  //
  // sprintRows makes up "Staff 4" for one with no name, which is right for
  // a button that has to say out loud what it points at and wrong here: a
  // staff whose name somebody has cleared is a staff they want unnamed,
  // and "Staff 4" beside it is the label coming back under another name.
  const rows = sprintRows().filter((r) => nameOf(r.part) !== "");
  const key = rows.map((r) => r.part + ":" + r.name + "@" + r.top.toFixed(2))
    .join(",");
  if (key !== staffNameKey) {
    staffNameKey = key;
    box.textContent = "";
    for (const r of rows) {
      // The label is the button.
      //
      // It began as a label and the sprint beside it as a button, each
      // showing where the other did not: the buttons at the head of the
      // score, the names once it had scrolled away. That was right while
      // the button meant "run to the end of this staff", which is a thing
      // you ask for from the beginning. It means "take me to where this
      // instrument comes in" now, and that is wanted wherever you are —
      // most of all in the middle of a page of rests, which is exactly
      // where the buttons had gone and only the names were left.
      const el = document.createElement("button");
      el.type = "button";
      el.className = "staff-name";
      el.textContent = r.name;
      const entry = staffEntry(r.part, r.top);
      el.title = entry
        ? r.name + " comes in at measure " + barNumberOf(entry.measure) +
          " — click to go there"
        : "Go to the end of " + r.name + ", which has no notes yet";
      el.addEventListener("click", () => void sprintToStaffEntry(r.part, r.top));
      box.appendChild(el);
    }
  }
  placeSprints(rows, box, svg, wrap);
}

// nameOf is what a staff is called, as the score itself has it — empty for
// one nobody has named.
function nameOf(part) {
  return ((state.info?.parts || [])[part]?.name || "").trim();
}

// placeSprints puts each button beside its own staff, and takes away the
// ones whose staff has scrolled off the panel.
function placeSprints(rows, box, svg, wrap) {
  const z = state.zoom;
  const here = box.getBoundingClientRect();
  const drawing = svg.getBoundingClientRect();
  const tall = wrap.clientHeight;
  const kids = box.children;
  for (let i = 0; i < rows.length && i < kids.length; i++) {
    const mid = drawing.top - here.top + (rows[i].top + rows[i].height / 2) * z;
    const el = kids[i];
    el.hidden = mid < 2 || mid > tall - 10;
    el.style.top = (mid - 10) + "px";
  }
}

// sprintToStaffEnd goes to the last bar of one staff and leaves the cursor
// ready to write in it.
//
// top is where that staff is drawn. A score with movements draws each one
// in its own band, one under the other, so the same part is on the page
// several times over; "as far as this goes" means as far as this band
// goes. Without it the arrow beside the first movement of the Moonlight
// sonata ran to the last bar of the third.
// sprintToStaffEntry goes to the bar this staff first plays in.
//
// What the button beside a staff's name does. It used to go to the staff's
// last bar — "carry on writing here" — which is a reasonable thing to want
// and a strange thing to offer from a label at the head of the score: one
// press and you are four hundred bars away, at the end of a piece you were
// looking at the beginning of. Carrying on writing is now asked for from
// the menu, where a thing that moves you that far belongs.
// goToStaffEnd takes the cursor to the end of the staff it is on.
//
// The staff it is on, because from a menu there is nothing else to mean:
// the buttons at the head of the score knew which staff they stood beside
// and a menu entry does not. Somebody who wants another staff's end puts
// the cursor on it first, which is one click and says plainly which one
// they meant.
async function goToStaffEnd() {
  const sel = state.selection;
  if (!sel) {
    status("Click a staff first — this goes to the end of the one you are on");
    return;
  }
  await sprintToStaffEnd(sel.part, null);
}

async function sprintToStaffEntry(part, top) {
  const name = (state.info?.parts || [])[part]?.name || `Staff ${part + 1}`;
  const entry = staffEntry(part, top);
  if (!entry) {
    // Nothing to come in at. The end of the staff is the other place
    // worth being — it is where writing this part would start — so that
    // is where it goes rather than refusing to move.
    await sprintToStaffEnd(part, top);
    return;
  }
  // On the first thing written in the bar rather than after it: this is a
  // place to look at, where the end of the staff is a place to write from.
  state.selection = {
    part, measure: entry.measure, at: 0,
    step: 4, ref: null, replace: false,
  };
  markSelection();

  const box = {
    x0: entry.x0, x1: entry.x1,
    top: entry.staffTop, bottom: entry.staffTop + staffHeightOf(entry),
  };
  await glideTo(box);
  await ensureTiles();
  flashMeasure(box);
  const where = movementAt(entry.measure);
  const named = where && where.title ? ` of ${where.title}` : "";
  status(`${name} comes in at measure ${barNumberOf(entry.measure)}${named}`);
}

async function sprintToStaffEnd(part, top) {
  let last = null;
  for (const h of state.hits || []) {
    if (h.part !== part) continue;
    if (top != null && Math.abs(h.staffTop - top) > 0.01) continue;
    if (!last || h.measure > last.measure) last = h;
  }
  const name = (state.info?.parts || [])[part]?.name || `Staff ${part + 1}`;
  if (!last) {
    status("There is nothing on " + name + " to go to");
    return;
  }
  // After the last thing written in the bar, which is where carrying on
  // starts. The middle line, having no reason to be anywhere else.
  state.selection = {
    part, measure: last.measure,
    at: (last.elements || []).length,
    step: 4, ref: null, replace: false,
  };
  markSelection();

  const box = {
    x0: last.x0, x1: last.x1,
    top: last.staffTop, bottom: last.staffTop + staffHeightOf(last),
  };
  await glideTo(box);
  // The view has moved a long way, so the pieces under it are different
  // ones. Waiting for the glide to finish first means asking once rather
  // than once for every frame of it.
  await ensureTiles();
  flashMeasure(box);
  const where = movementAt(last.measure);
  const named = where && where.title ? ` of ${where.title}` : "";
  status(`${name}: measure ${barNumberOf(last.measure)}${named}`);
}

// setupVirtualDraw fetches the next stretch as the view approaches the edge
// of what is drawn.
function setupVirtualDraw() {
  const wrap = $("#canvas-wrap");
  if (!wrap) return;
  let frame = 0;
  let ran = 0;
  const check = () => {
    frame = 0;
    ran = performance.now();
    // The buttons and the names are pinned to the panel and their staves
    // are not, so they move whenever the view does.
    drawSprints();
    drawStaffNames();
    if (!state.tiles) return; // the whole score is in the page
    void ensureTiles();
  };
  // A few times a second, not sixty.
  //
  // While the music plays the view is moved every frame, so this is asked
  // every frame — and what it does is work out which pieces of a long score
  // are wanted, which changes a handful of times a minute. The pieces are
  // turned into shapes two screenfuls ahead, so there is a great deal of
  // margin to spend on not asking.
  //
  // Looking rather than scrolling is the same question asked at the same
  // rate, and an eighth of a second is still quicker than a page turn.
  const askEvery = 125;
  const ask = () => {
    if (frame) return;
    const since = performance.now() - ran;
    if (since >= askEvery) {
      frame = requestAnimationFrame(check);
      return;
    }
    frame = -1;
    setTimeout(() => {
      frame = 0;
      ask();
    }, askEvery - since);
  };
  wrap.addEventListener("scroll", ask, { passive: true });
  window.addEventListener("resize", ask);
}

async function drawScore() {
  if (state.showAll && (state.info?.tunes || []).length > 1) {
    // A book is drawn whole, one tune after another, by its own path.
    // Saying so matters: a tiling left over from the single-tune view
    // would have the next scroll put one tune's piece into the book.
    state.tiles = null;
    forgetTiles();
    await drawBook();
    return;
  }
  // How big it is, which is what says whether it has to be drawn a piece
  // at a time at all.
  state.size = await loadSize();
  state.tiles = tilePlan(state.size);
  forgetTiles();

  if (!state.tiles) {
    // Small enough to hand over whole, which is every ordinary score.
    const res = await fetch(scoreURL());
    if (!res.ok) {
      status("Could not draw the score");
      return;
    }
    $("#canvas").innerHTML = await res.text();
    drawSprints();
    return;
  }

  // The drawing itself, the size of the whole score and empty: a window off
  // the edge of the music, which nothing is in, so what comes back is the
  // <svg> and the rules the shapes are drawn under and nothing else. The
  // pieces go inside it.
  //
  // Off the edge rather than a sliver at the origin, because the
  // coordinates are sent to the hundredth of a space and a sliver thinner
  // than that rounds to nothing at all — which the server reads as no
  // window, and answers with the whole score. Fifty-six megabytes of it.
  const shell = await fetch(scoreURL({
    window: { x0: -2, y0: -2, x1: -1, y1: -1 },
  }));
  if (!shell.ok) {
    status("Could not draw the score");
    return;
  }
  $("#canvas").innerHTML = await shell.text();
  const svg = currentSVG();
  if (!svg) return;
  // First, so that everything put in afterwards — the selection, the
  // cursor, the lit noteheads — is drawn over the music rather than under
  // it, and so that a piece arriving later does not jump in front of them.
  tileHome = document.createElementNS("http://www.w3.org/2000/svg", "g");
  tileHome.setAttribute("class", "tiles");
  svg.appendChild(tileHome);

  await ensureTiles();
  drawSprints();
}

// drawBook puts every score in the file on screen, one under another.
//
// Each is engraved on its own — they are separate pieces with their own
// clefs, keys and meters, and running them together would be a lie about the
// music. Stacking the drawings is the honest way to see a whole book at once.
async function drawBook() {
  const tunes = state.info.tunes;
  const canvas = $("#canvas");
  canvas.innerHTML = "";
  status(`Drawing ${tunes.length} tunes…`);

  const parts = await Promise.all(
    tunes.map((t) => fetch(scoreURL({ tune: t.index })).then((r) => r.text()))
  );
  canvas.innerHTML = parts
    .map((svg, i) => `<div class="tune" data-tune="${tunes[i].index}">${svg}</div>`)
    .join("");

  markCurrentTune();
  status(`${tunes.length} tunes`);
}

function markCurrentTune() {
  $$("#canvas .tune").forEach((d) =>
    d.classList.toggle("current", Number(d.dataset.tune) === state.info?.current)
  );
}

async function selectTune(i) {
  const res = await fetch("/api/tune?i=" + i, { method: "POST" });
  if (!res.ok) {
    status("Could not switch: " + (await res.text()).trim());
    return;
  }
  stop();
  state.selection = null;
  state.multi = [];
  await loadInfo();
  if (state.showAll) markCurrentTune();
  else await drawScore();
  await loadHits();
  const t = state.info.tunes?.[i]?.title || "Tune " + (i + 1);
  status(`${t} — press space to play it`);
}

async function loadInfo() {
  const res = await fetch("/api/info");
  state.info = await res.json();
  const i = state.info;

  // What the score carries, read with the score rather than when the
  // console is opened. A warning about losing the scripts on export is no
  // use if the editor only finds out there are any once you have gone
  // looking for them.
  void loadScripts();

  // In a native window the title bar already says gscore; repeating it
  // inside the window is what makes a web view look like a web page.
  document.body.classList.toggle("native", !!i.native);
  // Some things need gscore to be able to make a sound, which means a
  // soundfont it can reach: on this machine, or fetched into the page when
  // the core is the page. Served to somebody else's browser they are not
  // offered at all — an item that is there and answers with an apology is
  // worse than one that was never there.
  $$("[data-needs-instruments]").forEach((el) => (el.hidden = !canSound()));
  // And the things that need a disk with the person's own files on it.
  $$("[data-needs-files]").forEach((el) => (el.hidden = !i.local));
  if (i.local) void loadRecent();
  // And the scripts in the plugins folder, read once when the window opens:
  // the folder is the person's and nothing in the page changes it, so there
  // is nothing to keep in step while they work.
  if (i.local) void loadPlugins();
  // And the parts, which belong to the score and so are read with it.
  void loadSheets();
  $("#version").textContent = "Gscore " + i.version;
  $("#about-version").textContent = "Version " + i.version;
  // A star beside the name while there is work that is not on the disk yet,
  // which is how every editor says it. The flag is the server's own — the
  // same one that makes quitting stop and ask — so the star and the question
  // at the end can never disagree about whether anything was lost.
  $("#filename").textContent = documentName(i);
  // And in the Mac application's own title bar, which is where somebody
  // looking at the window looks: the status line along the bottom is not
  // where a Mac says whether a document is saved.
  tellNativeTitle(documentName(i));

  const rows = [];
  if (i.title) rows.push(["Title", i.title]);
  if (i.subtitle) rows.push(["Subtitle", i.subtitle]);
  if (i.composer) rows.push(["Composer", i.composer]);
  $("#score-info").innerHTML =
    rows.map(([k, v]) => `<div><span>${k}</span><b>${escapeHTML(v)}</b></div>`).join("") +
    `<div><span>Tempo</span><button class="linklike" id="tempo-btn">${i.tempo} bpm</button></div>`;
  $("#tempo-btn")?.addEventListener("click", setTempoDialog);

  $("#part-info").innerHTML = i.parts
    .map(
      (p, idx) => `<div class="part-card${p.hidden ? " hidden-staff" : ""}">
        <div class="name">
          <button class="eye" data-show-part="${idx}"
                  title="${p.hidden ? "Show this staff" : "Hide this staff"}"
                  aria-pressed="${p.hidden ? "true" : "false"}">${p.hidden ? "◌" : "●"}</button>
          <span>${escapeHTML(p.name)}</span>
        </div>
        <div><span>Clef</span><b>${p.clef}</b></div>
        <div><span>Key</span><button class="linklike" data-key-part="${idx}">${p.key}</button></div>
        <div><span>Time</span><button class="linklike" data-time-part="${idx}">${p.time}</button></div>
        <div><span>Sounds as</span><button class="linklike" data-inst-part="${idx}">${escapeHTML(p.instrument || "piano")}</button></div>
        <div><span>Measures</span><b>${p.measures}</b></div>
        ${p.hidden
          ? `<button class="show-again" data-show-part="${idx}">Hidden — show it again</button>`
          : ""}
      </div>`
    )
    .join("");
  $$("[data-show-part]").forEach((b) =>
    b.addEventListener("click", async (ev) => {
      ev.stopPropagation();
      const idx = Number(b.dataset.showPart);
      const now = !!state.info?.parts?.[idx]?.hidden;
      const res = await fetch(`/api/staff?part=${idx}&hidden=${now ? 0 : 1}`,
                              { method: "POST" });
      if (!res.ok) {
        status("Could not change it: " + (await res.text()).trim());
        return;
      }
      await reload();
      status(now ? "Staff shown" : "Staff hidden — it still plays");
    })
  );
  $$("[data-key-part]").forEach((b) =>
    b.addEventListener("click", () => openKeyPicker(Number(b.dataset.keyPart)))
  );
  $$("[data-time-part]").forEach((b) =>
    b.addEventListener("click", () => {
      state.selection = { ...(state.selection || {}), part: Number(b.dataset.timePart) };
      setTimeDialog();
    })
  );
  $$("[data-inst-part]").forEach((b) =>
    b.addEventListener("click", () => {
      state.selection = { ...(state.selection || {}), part: Number(b.dataset.instPart) };
      setInstrumentDialog();
    })
  );

  const tunes = i.tunes || [];

  // Opening a file that holds many scores shows all of them. Being handed
  // one exercise out of forty-eight, with no sign the others exist, is not
  // what anyone meant by opening the file.
  if (!state.showAllDecided) {
    state.showAll = tunes.length > 1;
    state.showAllDecided = true;
  }

  $("#tune-group").hidden = tunes.length < 2;
  if (tunes.length > 1) {
    $("#tune-count").textContent = `${tunes.length} tunes in this file`;
    const sel = $("#tune-select");
    sel.innerHTML = tunes
      .map((t) => `<option value="${t.index}">${escapeHTML(t.title)}</option>`)
      .join("");
    sel.value = String(i.current);
    $("#show-all").checked = state.showAll;
  }

  $("#empty-hint").hidden = !i.parts.every((p) => p.notes === 0);
  markFingeringMenu();
  markViewTicks();
  await loadKit();
  markHistory(!!i.canUndo, !!i.canRedo);

  // An importer that only understood part of a file says so here, rather than
  // leaving half a score on screen with no explanation. The whole list is in
  // Help ▸ Lines that could not be read; the status bar gets the count.
  importLog = i.log || [];
  if (i.warning) {
    status("Opened with warnings — " + i.warning +
           " — see Help ▸ Lines that could not be read");
  }
}

// reload re-reads everything after the file itself has changed.
//
// It does not forget how the score is being looked at. Changing a key
// signature used to put the view back to "every tune at once", which from the
// outside looks like the score suddenly shrinking — the change was to one
// exercise and the answer was the whole book.
async function reload() {
  // The one place every open and every redraw of the whole score goes
  // through, which is why Gilbert is put up here rather than beside each of
  // them. A score opened from the command line never touches openFile at
  // all — the server has it before the window exists — and that is exactly
  // the case where the wait is longest.
  //
  // No words: whoever started the work has usually already said what it is,
  // and busy only replaces the line when it is given something to say.
  busy();
  try {
    stop();
    state.selection = null;
    state.multi = [];
    await loadInfo();
    // Before the drawing, because the drawing is made with the mode this
    // reads. A .gscore carries the drawing options it was saved with, and
    // this is where they arrive.
    await syncView();
    await drawScore();
    await loadHits();
    placeCursorOnOpen();
    // The sprints come from the hit regions, which are only just here.
    drawSprints();
  } finally {
    idle();
  }
  // And then, with the score on screen and nothing being waited on, work
  // out how to play it.
  //
  // Nothing needs the performance until Play is pressed, and on a symphony
  // it is a hundred and thirty-two thousand notes and sixteen megabytes —
  // a second of work in the page, where the core is WebAssembly. Done
  // while the score is being read rather than when the button is pressed,
  // it is already in hand. Not awaited: the score is on screen and
  // editable, and if Play comes first it simply waits for this.
  warmPlayback();
}

// warmPlayback works the performance out in the background, once the
// browser has nothing better to do.
function warmPlayback() {
  const go = () => {
    if (state.playback || state.playing) return;
    void playbackData();
  };
  if (window.requestIdleCallback) window.requestIdleCallback(go, { timeout: 3000 });
  else setTimeout(go, 400);
}

// placeCursorOnOpen puts the writing cursor on the page as soon as the score
// is there.
//
// A score with no cursor on it looks like something to read rather than
// something to write, and the first thing anybody had to do on opening a
// piece was click it to find out where they were. The server says where:
// the place the file recorded, or the end of the first staff for a file from
// a format that records none.
function placeCursorOnOpen() {
  if (state.selection || state.readOnly) return;
  const c = state.info?.cursor;
  if (!c) return;
  state.selection = {
    part: c.part, measure: c.measure, at: c.at, step: c.step, ref: null,
    replace: false,
  };
  markSelection();
  scrollSelectionIntoView();
}

// rememberCursor tells the server where the cursor is, so that saving keeps
// it and opening the file again puts it back.
//
// Sent a moment after it settles rather than on every keystroke: arrowing
// along a bar would otherwise be one request per note, and only where it
// stops is worth recording.
let cursorTimer = null;
function rememberCursor() {
  const sel = state.selection;
  if (!sel || state.readOnly) return;
  if (cursorTimer) clearTimeout(cursorTimer);
  cursorTimer = setTimeout(() => {
    cursorTimer = null;
    const q = new URLSearchParams({
      part: String(sel.part), measure: String(sel.measure),
      at: String(sel.at), step: String(sel.step ?? 0),
    });
    void fetch("/api/cursor?" + q.toString(), { method: "POST" })
      .catch(() => { /* where the cursor is is not worth an error */ });
  }, 500);
}

// reopen is for a different file: the view starts over with it.
async function reopen() {
  state.showAllDecided = false;
  await reload();
}

// showZoom sets the size without drawing anything, for the one moment
// that happens before there is anything to draw.
function showZoom(z) {
  state.zoom = Math.min(26, Math.max(5, Math.round(z)));
  const slider = $("#zoom");
  if (slider) slider.value = String(state.zoom);
  const readout = $("#zoom-readout");
  if (readout) readout.textContent = state.zoom + " px/space";
}

function setZoom(z) {
  state.zoom = Math.min(26, Math.max(5, Math.round(z)));
  $("#zoom").value = String(state.zoom);
  $("#zoom-readout").textContent = state.zoom + " px/space";
  // Remembered, so that a reader who found the size they like finds it
  // again next time. See startingZoom.
  try {
    localStorage.setItem("zoom", String(state.zoom));
  } catch (_) {
    // A private window may refuse. The size still holds for this visit.
  }
  drawScore().then(markSelection);
}

// A native window cannot download. Clicking a blob link does nothing there:
// a web view has no download machinery behind it, which is why this appeared
// to do nothing at all. Running as an application, the file is written by the
// server through a real save panel; served to a browser, the link still works.
// exportPDF writes the score as a PDF, which is what a finished score is for.
//
// It is made here rather than by a browser's print dialog: gscore's own
// engraver lays out the pages and embeds the music font, so the file is the
// same on every machine and can be made by a script with no screen attached.
async function exportPDF() {
  const many = (state.info?.tunes?.length ?? 0) > 1;
  const parts = (state.info?.parts?.length ?? 0) > 1;
  const choice = await askPrint(many, parts, "Export PDF", "Export", true);
  if (!choice) return;

  const q = new URLSearchParams({ as: "pdf", what: choice.what });
  if (choice.landscape) q.set("orientation", "landscape");
  if (!choice.pageNumbers) q.set("pagenumbers", "0");
  if (choice.multiRests) q.set("multirest", "1");
  if (choice.concert) q.set("concert", "1");

  if (state.info?.local) {
    let res = await saveRequest(q.toString(), "Writing the file…");
    if (!res.ok) {
      status("Could not export: " + (await res.text()).trim());
      return;
    }
    let d = await res.json();
    if (d.needPath) {
      const where = await ask({
        title: "Export PDF", label: "Save to",
        value: (d.suggested || "score.pdf").replace(/\.gscore$/, ".pdf"),
        note: "The system's save panel would not open here, so type the path.",
      });
      if (where === null || where.trim() === "") {
        status("Ready");
        return;
      }
      q.set("path", where.trim());
      res = await saveRequest(q.toString(), "Writing the file…");
      if (!res.ok) {
        status("Could not export: " + (await res.text()).trim());
        return;
      }
      d = await res.json();
    }
    status(d.cancelled ? "Export cancelled" : "Wrote " + d.path);
    return;
  }

  // Served rather than run: the browser downloads it.
  status("Making the PDF…");
  const res = await fetch("/api/pdf?" + q.toString());
  if (!res.ok) {
    status("Could not export: " + (await res.text()).trim());
    return;
  }
  const blob = await res.blob();
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = (state.info?.title || "score").replace(/[^\w-]+/g, "-") + ".pdf";
  a.click();
  URL.revokeObjectURL(a.href);
  status("PDF downloaded");
}

async function exportSVG() {
  if (state.info?.local) {
    const p = new URLSearchParams({ as: "svg", mode: state.mode, space: String(state.zoom) });
    const res = await saveRequest(p.toString(), "Writing the file…");
    if (!res.ok) {
      status("Could not export: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    status(d.cancelled ? "Ready" : "Exported " + d.path);
    return;
  }
  const res = await fetch(scoreURL());
  const blob = new Blob([await res.text()], { type: "image/svg+xml" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = (state.info?.title || "score").replace(/[^\w-]+/g, "-") + ".svg";
  a.click();
  URL.revokeObjectURL(a.href);
  status("Exported SVG");
}

// exportPage writes the score as a web page that carries itself.
//
// For sending to somebody who has no music program: they open it and read
// it, with nothing to install and nothing fetched. It is still the score —
// it can be opened here again and gone on with — which a PDF is not.
async function exportPage() {
  if (state.info?.local) {
    const p = new URLSearchParams({
      as: "html", mode: state.mode, space: String(state.zoom),
    });
    const res = await saveRequest(p.toString(), "Writing the file…");
    if (!res.ok) {
      status("Could not export: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    status(d.cancelled ? "Ready" : "Exported " + d.path);
    return;
  }
  // As a page there is no filesystem to write to, so the file comes back
  // and the browser does what it does with a download.
  const p = new URLSearchParams({ mode: state.mode, space: String(state.zoom) });
  const res = await fetch("/api/page?" + p.toString());
  if (!res.ok) {
    status("Could not export: " + (await res.text()).trim());
    return;
  }
  const blob = new Blob([await res.text()], { type: "text/html" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = (state.info?.title || "score").replace(/[^\w-]+/g, "-") + ".html";
  a.click();
  URL.revokeObjectURL(a.href);
  status("Exported a web page");
}

// exportAs writes the score through one of the exporters the core
// registers, by the extension that names it.
//
// On this machine the server writes the file through a save panel; served
// to a browser it comes back and the browser downloads it. The same two
// paths every export takes.
// keepFile hands an answer to the browser to put somewhere.
//
// The name comes from the server when it sent one, because the server is
// what knows the score's title and the format it just wrote.
async function keepFile(res, fallbackName) {
  const blob = await res.blob();
  let name = fallbackName;
  const told = res.headers.get("content-disposition") || "";
  const m = told.match(/filename="?([^";]+)"?/);
  if (m) name = m[1];

  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
  return name;
}

// keepsScripts says a format can carry the scripts a score holds.
//
// Only gscore's own can. Every other format in the world is a format for
// music, and a script is not music: it is the thought that made it.
function keepsScripts(ext) {
  return String(ext).toLowerCase() === "gscore";
}

async function exportAs(ext, type, what) {
  // Said before anything is written, because the thing being lost is not
  // the file — the file is fine — it is the work that went into the file,
  // and finding that out afterwards is finding it out too late.
  //
  // The score itself is untouched, and the warning says so: exporting is
  // not saving, and somebody who has just been told they are about to lose
  // something needs to know they have not lost it yet.
  if (!keepsScripts(ext) && state.scripts.length) {
    const n = state.scripts.length;
    const ok = await confirmThat({
      title: `Export without the ${n === 1 ? "script" : `${n} scripts`}?`,
      note: `A ${String(ext).toUpperCase()} file holds music, and a script ` +
        `is not music — it is the process that made it. ${n === 1
          ? "The script stays"
          : "They stay"} in this score, and in any .gscore you save; ` +
        `${n === 1 ? "it" : "they"} will not be in the exported file.`,
      ok: "Export anyway",
    });
    if (!ok) {
      status("Ready");
      return;
    }
  }
  if (state.info?.local) {
    const p = new URLSearchParams({ as: ext });
    const res = await saveRequest(p.toString(), "Writing the file…");
    if (!res.ok) {
      status("Could not export: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    status(d.cancelled ? "Ready" : "Exported " + d.path);
    return;
  }
  const res = await fetch("/api/convert?as=" + encodeURIComponent(ext));
  if (!res.ok) {
    status("Could not export: " + (await res.text()).trim());
    return;
  }
  // The bytes as they are. Read as text, a .gscore and a compressed
  // MusicXML — both zips — come out with every byte that is not valid
  // UTF-8 replaced, which is a file nothing can open.
  await keepFile(res,
    (state.info?.title || "score").replace(/[^\w-]+/g, "-") + "." + ext);
  status("Exported " + what);
}

// exportMIDI writes the score as a standard MIDI file.
//
// Not the page and not the drawing: the performance. What comes out is what
// pressing play sounds, with the dynamics, the swells, the slurs held and
// the staccato notes short, for anything else that makes sound out of it.
async function exportMIDI() {
  if (state.info?.local) {
    const p = new URLSearchParams({ as: "mid" });
    const res = await saveRequest(p.toString(), "Writing the file…");
    if (!res.ok) {
      status("Could not export: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    status(d.cancelled ? "Ready" : "Exported " + d.path);
    return;
  }
  const res = await fetch("/api/midi");
  if (!res.ok) {
    status("Could not export: " + (await res.text()).trim());
    return;
  }
  const blob = await res.blob();
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = (state.info?.title || "score").replace(/[^\w-]+/g, "-") + ".mid";
  a.click();
  URL.revokeObjectURL(a.href);
  status("Exported MIDI");
}

/* ---------- playing through a soundfont ---------- */

// How far ahead of the listener to keep the rendered music, in seconds.
//
// Enough that a slow render or a busy moment in the browser cannot open a
// gap you would hear, short enough that stopping does not throw much away.
//
// Three was enough for a quartet and nowhere near enough for an orchestra.
// Rendering fifty-two staves runs at about five times real time on this
// machine and perhaps twice that slowly in the page, where the core is
// WebAssembly and shares the one thread with drawing the score and moving
// the cursor — so every time the view fetched the next stretch of music,
// the rendering stopped for as long as that took and the three seconds ran
// out. The music stopped, caught up, and went on, every few seconds.
const liveAhead = 12;

// How many chunks to render in one go before letting the page breathe.
//
// Filling twelve seconds of orchestra is several seconds of work, and doing
// it in one pass would hold the only thread there is for all of it: the
// cursor would stand still and the window would not answer. Two at a time
// with a moment in between fills the buffer about as fast and leaves the
// page usable while it does.
const liveBurst = 2;

// And how soon to come back: at once while there is catching up to do,
// otherwise at the pace the music is consumed.
const livePumpSoon = 30;
const livePumpIdle = 250;

// startLive asks the server to play the score through a soundfont and
// schedules what comes back.
//
// Returns false if it cannot — no soundfont, or served from somewhere that
// has no disk — and the oscillators take over, which is what gscore has
// always done and still sounds like something.
async function startLive(from, master) {
  if (!canSound()) return false;
  // In the page the instruments have to be in hand before anything can be
  // played with them. Loading them from the browser's own store takes a
  // moment and no network; the first time, when there is nothing stored,
  // playing is not held up for a thirty-megabyte download nobody asked for.
  if (state.info?.inBrowser && !pageFont.loaded) {
    if (!(await loadStoredInstruments())) return false;
  }
  const p = new URLSearchParams({
    from: String(from),
    // At the rate the browser is mixing at, so that nothing is resampled on
    // the way in.
    rate: String(Math.round(audio.sampleRate)),
  });
  let d;
  try {
    const res = await fetch("/api/play/start?" + p.toString(), { method: "POST" });
    if (!res.ok) return false;
    d = await res.json();
  } catch (_) {
    return false;
  }

  live = { id: d.id, rate: d.rate, at: 0, ended: false, timer: null, master,
           font: d.font, first: null };

  // The first stretch of music is rendered before the clock is started, not
  // after it.
  //
  // It used to be the other way about: the moment the music was to begin at
  // was fixed a tenth of a second ahead, and then the server was asked to
  // open a performance and render the first seconds of it. On a quartet
  // through the built-in voices that is quick enough to fit in the tenth of
  // a second. On the Rossini — twenty-three staves through a soundfont of
  // real recordings — it is not, and what came back was scheduled at the
  // first moment still in the future rather than at the moment it was
  // meant for. The sound started late; the cursor had started on time; and
  // the gap between them was however long that first render took, which is
  // why it grew with the size of the soundfont and vanished with the
  // oscillators.
  //
  // Rendered first, the clock is started around music that is already in
  // hand, and the cursor, the clicks and the notes lighting up are all one
  // thing again.
  const first = await liveChunk(live);
  if (!first) {
    endLive();
    return false;
  }
  live.first = first;
  return true;
}

// beginLive starts the rendered performance, on the clock everything else
// in this piece is on.
function beginLive(t0) {
  const mine = live;
  if (!mine) return;
  mine.at = t0;
  if (mine.first) {
    const src = audio.createBufferSource();
    src.buffer = mine.first;
    src.connect(mine.master);
    src.start(mine.at);
    mine.at += mine.first.duration;
    voices.push(src);
    mine.first = null;
  }
  pumpLive();
}

// liveChunk asks for the next stretch of rendered music, as something that
// can be scheduled.
//
// null when there is no more to come: the end of the piece, or a
// performance that has gone away from under us.
async function liveChunk(mine) {
  let res;
  try {
    res = await fetch("/api/play/chunk?id=" + encodeURIComponent(mine.id));
  } catch (_) {
    mine.ended = true; // stopped, or the server went away
    return null;
  }
  if (mine !== live) return null; // something else started while we waited
  if (!res.ok) {
    mine.ended = true;
    return null;
  }
  const pcm = new Int16Array(await res.arrayBuffer());
  if (res.headers.get("X-Gscore-End")) mine.ended = true;
  if (!pcm.length) return null;

  const frames = pcm.length / 2;
  const buf = audio.createBuffer(2, frames, mine.rate);
  const L = buf.getChannelData(0);
  const R = buf.getChannelData(1);
  for (let i = 0; i < frames; i++) {
    L[i] = pcm[2 * i] / 32768;
    R[i] = pcm[2 * i + 1] / 32768;
  }
  return buf;
}

// pumpLive keeps the music rendered a few seconds ahead of where it is being
// listened to.
//
// One piece at a time, each scheduled to start exactly where the last one
// ended, so the joins are not joins: the server is rendering one continuous
// performance and these are only the lengths it was handed over in.
async function pumpLive() {
  const mine = live;
  if (!mine || mine.ended) return;
  let made = 0;
  while (mine === live && !mine.ended && made < liveBurst &&
         mine.at - audio.currentTime < liveAhead) {
    made++;
    const buf = await liveChunk(mine);
    if (mine !== live) return; // something else started while we waited
    if (!buf) break;

    const src = audio.createBufferSource();
    src.buffer = buf;
    src.connect(mine.master);
    // Never in the past: a render that took longer than the music it
    // produced would otherwise schedule itself behind the clock and be
    // dropped, leaving a hole.
    mine.at = Math.max(mine.at, audio.currentTime + 0.02);
    src.start(mine.at);
    mine.at += buf.duration;
    voices.push(src);
  }
  if (mine === live && !mine.ended) {
    // Still short of the buffer it wants, so straight back to it; otherwise
    // at the pace the music is being listened to.
    const behind = mine.at - audio.currentTime < liveAhead;
    mine.timer = setTimeout(pumpLive, behind ? livePumpSoon : livePumpIdle);
  }
}

// endLive stops asking for music and tells the server it can let go.
function endLive() {
  const mine = live;
  live = null;
  if (!mine) return;
  if (mine.timer) clearTimeout(mine.timer);
  // Nothing waits on this: the performance is over here whatever the server
  // makes of it, and it would be dropped on its own soon enough.
  fetch("/api/play/stop?id=" + encodeURIComponent(mine.id), { method: "POST" })
    .catch(() => {});
}

// playbackSound lets somebody choose what the editor plays through.
// soundDelayDialog asks how far behind the picture the sound is.
//
// The cursor is put on the note being heard, which means knowing how long
// the sound takes to get out of the machine. Where the browser says — and
// Chrome and Firefox do — nothing here is needed and the answer is zero.
// WebKit does not, which is the Mac application and every browser on a
// Mac, and over Bluetooth headphones the gap is a fifth of a second: a
// quaver at a brisk tempo, and plainly visible.
//
// Nothing can measure it but the person listening, so they are asked, and
// told how to tell which way it is out: whether the notehead lights before
// the note or after it.
async function soundDelayDialog() {
  const now = Number(state.playLag) || 0;
  const answer = await ask({
    title: "Sound delay",
    label: "Milliseconds the sound is behind the picture",
    value: String(now),
    note: "Play something and watch the cursor. If a note lights up " +
          "before you hear it, put the number up; if you hear it first, " +
          "put it down. 0 is right where the browser reports its own " +
          "latency; wireless headphones are usually 150 to 300.",
  });
  if (answer === null) return;
  const n = Math.round(Number(String(answer).trim()));
  if (!Number.isFinite(n) || n < -250 || n > 1000) {
    status("The sound delay is a number of milliseconds between -250 and 1000");
    return;
  }
  // Kept first, so that it holds for this performance whether or not there
  // is a disk to write it to — the page has none at all.
  state.playLag = n;
  try {
    await fetch("/api/settings?playLag=" + encodeURIComponent(String(n)),
      { method: "POST" });
  } catch (_) {
    // Nothing to keep it on, which is not a reason to refuse the setting.
  }
  status(n === 0
    ? "The cursor follows whatever the browser says the delay is"
    : `The cursor is held back ${n} ms to meet the sound`);
}

async function playbackSound() {
  if (!canSound()) {
    status("Choosing the playback sound needs gscore running on your own machine");
    return;
  }
  await loadSoundfonts();
  const current = await (await fetch("/api/settings")).json();

  const options = record.fonts.map((f) => ({
    value: f.path,
    label: f.name + (f.path === current.playbackFont ? " — playing now" : ""),
  }));
  // The three that are things to do rather than soundfonts to play
  // through. Picked, they open what they are: the question has been
  // answered by the picking and there is nothing for OK to add.
  options.push({ value: "get", label: "Download another…", now: true });
  options.push({
    value: "file",
    label: state.info?.inBrowser
      ? "Open a .sf2 from this computer…"
      : "Choose a .sf2 on this computer…",
    now: true,
  });
  if (state.info?.inBrowser && pageFont.url) {
    options.push({
      value: "forget", label: "Forget the kept instruments", now: true,
    });
  }

  const choice = await ask({
    title: "Playback sound",
    label: "Play through",
    value: current.playbackFont || "",
    note: record.fonts.length
      ? "The editor plays through this. It is also what a new recording starts with."
      : state.info?.inBrowser
        ? "There are no instruments yet. Download a set once and this page keeps "
          + "it, so the next visit has it already and nothing is fetched again."
        : "There are no soundfonts yet. Download one and the editor will play through it.",
    options,
  });
  if (!choice) return;
  if (choice === "get") {
    await getSoundfont();
    if (!record.font) return;
    await saveSetting("playbackFont", record.font);
    return;
  }
  if (choice === "file") {
    await pickSoundfont();
    return;
  }
  if (choice === "forget") {
    await forgetInstruments();
    return;
  }
  await saveSetting("playbackFont", choice);
}

// explainDownload says what the page cannot do and how to do it instead.
//
// A refusal is not useful on its own. The file is right there, the person's
// browser will fetch it perfectly well, and the only thing in the way is a
// rule about which page asked. So: the link, in as many words, and the
// button that takes the file once they have it.
async function explainDownload(name, url) {
  const dlg = $("#get-font");
  let host = url;
  try {
    host = new URL(url, location.href).host;
  } catch (_) {
    /* a link we could not parse is still a link they can click */
  }
  $("#get-font-why").textContent =
    `${host} does not let this page fetch files from it, so gscore cannot `
    + `download ${name} for you. Your browser can — the rule is about which `
    + `page asked, not about the file.`;
  const link = $("#get-font-link");
  link.href = url;
  link.textContent = url;

  return new Promise((resolve) => {
    const close = (open) => {
      dlg.close();
      $("#get-font-open").onclick = null;
      $("#get-font-cancel").onclick = null;
      if (open) {
        void pickSoundfont().then(() => resolve(true));
      } else {
        status("Ready");
        resolve(false);
      }
    };
    $("#get-font-open").onclick = () => close(true);
    $("#get-font-cancel").onclick = () => close(false);
    dlg.showModal();
  });
}

// pickLocalFile shows the browser's file picker and waits for an answer.
function pickLocalFile(sel) {
  const input = $(sel);
  if (!input) return Promise.resolve(null);
  return new Promise((resolve) => {
    const done = () => {
      input.removeEventListener("change", done);
      const file = input.files && input.files[0];
      input.value = "";
      resolve(file || null);
    };
    input.addEventListener("change", done);
    input.click();
  });
}

async function saveSetting(key, value) {
  const p = new URLSearchParams({ [key]: value });
  const res = await fetch("/api/settings?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not keep that: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  // Anything already playing is playing through the old one.
  if (state.playing) stop();
  status("Playing through " + (d.playbackFontName || "the chosen soundfont"));
}

/* ---------- the files opened before ---------- */

// The paths behind the numbered commands, in the order the menu shows them.
let recentPaths = [];

// loadRecent fills the Open recent submenu.
//
// Rebuilt from the server each time rather than kept in step by hand: the
// list belongs to the program and not to this window, and a second window —
// or the same one after an hour of work — would otherwise be offering a
// list that was true when it opened.
async function loadRecent() {
  const menu = $("#recent-menu");
  const items = $("#recent-items");
  if (!menu || !items) return;

  let files = [];
  try {
    const res = await fetch("/api/recent");
    if (res.ok) files = (await res.json()).files || [];
  } catch (_) {
    /* no list is the same as an empty one */
  }

  recentPaths = files.map((f) => f.path);
  items.innerHTML = "";
  const item = (label, command, title) => {
    const b = document.createElement("button");
    b.innerHTML = "<span></span>";
    b.querySelector("span").textContent = label;
    if (command) {
      b.dataset.action = command;
      // Wired here and not by setupMenus, which ran before these existed.
      // The name is also what the application's own menu bar sends, so
      // both ways of picking a file reach the same command.
      b.addEventListener("click", () => {
        if (b.disabled) return;
        closeMenus();
        void action(command);
      });
    }
    if (title) b.title = title;
    items.append(b);
    return b;
  };

  if (files.length === 0) {
    item("Nothing opened yet", "").disabled = true;
  } else {
    files.forEach((f, i) => {
      // The whole path in the tooltip, where the name alone would not say
      // which file this is: two folders of parts both hold "Flute.xml".
      item(f.name, "open-recent-" + i, f.path);
    });
    items.append(document.createElement("hr"));
    item("Clear the list", "clear-recent");
  }
  // The native menu bar is built from this one, so it has to be told that
  // what it copied has changed.
  rebuildNativeMenus();
}

/* ---------- the parts ---------- */

// The parts, as the server last read them off the score, and which one is
// being shown. -1 is the score itself.
let sheets = [];
let showingSheet = -1;

// loadSheets fills the Parts menu.
//
// Built from the score rather than from the markup, because which parts
// there are is a fact about the score: a quartet has four and a song has
// none, and nothing in a menu written by hand could know that.
async function loadSheets() {
  const items = $("#parts-items");
  if (!items) return;

  sheets = [];
  showingSheet = -1;
  try {
    const res = await fetch("/api/sheets");
    if (res.ok) {
      const d = await res.json();
      sheets = d.sheets || [];
      showingSheet = d.showing ?? -1;
    }
  } catch (_) {
    /* a score with no parts is the same as none read */
  }

  items.innerHTML = "";
  const item = (label, command, title) => {
    const b = document.createElement("button");
    b.innerHTML = "<span></span>";
    b.querySelector("span").textContent = label;
    if (command) {
      b.dataset.action = command;
      b.addEventListener("click", () => {
        if (b.disabled) return;
        closeMenus();
        void action(command);
      });
    }
    if (title) b.title = title;
    items.append(b);
    return b;
  };

  // The score is the first thing on the list and is a part of nothing: it
  // is what you come back to, and it has to be as easy to reach as any of
  // them.
  const score = item("Full score", "show-score");
  if (showingSheet < 0) tickItem(score);

  if (!sheets.length) {
    items.append(document.createElement("hr"));
    item("Make a part for every player", "make-parts");
  } else {
    items.append(document.createElement("hr"));
    sheets.forEach((s, i) => {
      const staves = s.staves.length === 1
        ? "staff " + (s.staves[0] + 1)
        : "staves " + s.staves.map((n) => n + 1).join(" and ");
      const b = item(s.name, "show-part-" + i, staves);
      if (i === showingSheet) tickItem(b);
    });
    items.append(document.createElement("hr"));
    // Only while a part is on the screen: it is about that part, and an
    // entry that answers "turn to a part first" is worse than no entry.
    const setup = item("Part setup…", "part-setup");
    if (showingSheet < 0) setup.disabled = true;
  }

  // These were made after the menus were drawn, so they have no pictures
  // beside them yet.
  syncMenuIcons();
  rebuildNativeMenus();
}

// partSetupDialog is what one part wants of its own page.
//
// Only the few things that are a part's and not the score's. Everything
// else a part can want differently — its spacing, its bar numbers, where
// its lines end — is set in the window that already sets those: those
// windows write to whichever document is on the screen, which is the only
// rule anybody could guess and means there is one spacing window and not
// two.
async function partSetupDialog() {
  if (showingSheet < 0) {
    status("Turn to a part first — this is the full score");
    return;
  }
  const dlg = $("#part-setup");
  if (!dlg) return;
  const sh = sheets[showingSheet];
  if (!sh) return;

  $("#part-setup h2").textContent = sh.name;
  $("#part-name").value = sh.name;
  $("#part-staves").value = sh.staves.map((n) => n + 1).join(", ");
  $("#part-size").value = sh.staffSpace || "";
  $("#part-multirest").checked = !!sh.multiRests;

  // Which staves there are to choose from, named, so that "1, 2" is a
  // choice rather than a guess.
  const names = (state.info?.parts || [])
    .map((p, i) => (i + 1) + " " + (p.name || "Staff " + (i + 1)));
  $("#part-staves-hint").textContent = names.length
    ? "Of " + names.join(", ")
    : "";

  drawCues(sh);
  drawVoicing(sh);
  if (!dlg.open) dlg.showModal();
}

// drawVoicing fills the voicing controls from the part.
//
// One staff at a time, because a rule is about a staff: a part printing a
// flute staff and a piccolo staff takes the bottom note of one and all of
// the other. The staff picker holds the part's own staves and nobody
// else's — a rule about a staff this part does not print would do nothing.
function drawVoicing(sh) {
  const pick = $("#voice-staff");
  if (!pick) return;
  const names = state.info?.parts || [];
  pick.innerHTML = (sh.staves || []).map((n) =>
    `<option value="${n}">${escapeHTML((n + 1) + " " + (names[n]?.name || "Staff " + (n + 1)))}</option>`
  ).join("");
  showVoicingFor(sh);
}

// voicingOf is the rule a part has for a staff, or the empty rule.
function voicingOf(sh, staff) {
  return (sh.voicing || []).find((v) => v.staff === staff) ||
    { staff, pick: "all", nth: [], fromBottom: false, single: false, layer: 0 };
}

// showVoicingFor puts one staff's rule into the controls.
function showVoicingFor(sh) {
  const staff = Number($("#voice-staff")?.value ?? -1);
  if (!Number.isInteger(staff) || staff < 0) return;
  const v = voicingOf(sh, staff);
  $("#voice-pick").value = v.pick || "all";
  $("#voice-nth").value = (v.nth || []).join(", ");
  $("#voice-from").value = v.fromBottom ? "bottom" : "top";
  $("#voice-single").checked = !!v.single;
  $("#voice-layer").value = String(v.layer || 0);
  syncVoicingRows();
}

// syncVoicingRows hides the controls that only mean something for a rule
// counting notes through a chord.
function syncVoicingRows() {
  const nth = $("#voice-pick")?.value === "nth";
  const row = $("#voice-nth-row");
  const single = $("#voice-single-row");
  if (row) row.hidden = !nth;
  if (single) single.hidden = !nth;
  // The warning is about a part whose notes are chosen, whichever way.
  const chosen = ($("#voice-pick")?.value || "all") !== "all" ||
    Number($("#voice-layer")?.value || 0) > 0;
  const note = $("#voice-note");
  if (note) note.hidden = !chosen;
}

// applyVoicing sends the rule for the staff on show.
async function applyVoicing() {
  if (showingSheet < 0) return true;
  const staff = Number($("#voice-staff")?.value ?? -1);
  if (!Number.isInteger(staff) || staff < 0) return true;

  const q = new URLSearchParams({
    at: String(showingSheet), voice: String(staff),
    pick: $("#voice-pick").value,
    layer: $("#voice-layer").value,
  });
  if ($("#voice-pick").value === "nth") {
    const nth = $("#voice-nth").value.trim();
    if (nth === "") {
      status("Say which notes of the chord — 2, or 1,3");
      return false;
    }
    q.set("nth", nth);
    if ($("#voice-from").value === "bottom") q.set("frombottom", "1");
    if ($("#voice-single").checked) q.set("single", "1");
  }
  const res = await fetch("/api/sheets?" + q, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return false;
  }
  return true;
}

// drawCues lists what this part already cues, and offers the staves it
// could cue from.
//
// Not its own staves: the player can see their own line, and a cue of it
// would be the same bars printed twice.
function drawCues(sh) {
  const box = $("#part-cues");
  if (box) {
    box.innerHTML = "";
    if (!(sh.cues || []).length) {
      const p = document.createElement("p");
      p.className = "small";
      p.textContent = "None yet.";
      box.append(p);
    }
    (sh.cues || []).forEach((c, i) => {
      const row = document.createElement("div");
      row.className = "tempo-bar";
      const what = document.createElement("span");
      const last = c.at + c.bars;
      what.textContent = c.label + ", bar" + (c.bars === 1 ? " " : "s ") +
        (c.bars === 1 ? String(c.at + 1) : (c.at + 1) + "–" + last);
      const go = document.createElement("button");
      go.type = "button";
      go.textContent = "Take it away";
      go.addEventListener("click", () => void removeCue(i));
      row.append(what, go);
      box.append(row);
    });
  }

  const from = $("#cue-from");
  if (from) {
    const own = new Set(sh.staves);
    from.innerHTML = "";
    (state.info?.parts || []).forEach((p, i) => {
      if (own.has(i)) return;
      const o = document.createElement("option");
      o.value = String(i);
      o.textContent = p.name || "Staff " + (i + 1);
      from.append(o);
    });
    from.disabled = from.options.length === 0;
  }
  const bar = $("#cue-bar");
  if (bar && !bar.value) bar.value = "1";
  const add = $("#cue-add");
  if (add) add.disabled = !from || from.options.length === 0;
}

async function addCue() {
  if (showingSheet < 0) return;
  const q = new URLSearchParams({
    at: String(showingSheet),
    cue: $("#cue-from").value,
    bar: String(Math.max(0, Number($("#cue-bar").value || 1) - 1)),
    bars: $("#cue-bars").value || "2",
  });
  const res = await fetch("/api/sheets?" + q, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  await loadSheets();
  drawCues(sheets[showingSheet]);
  await reopen();
  status("Cue added");
}

async function removeCue(i) {
  if (showingSheet < 0) return;
  const res = await fetch(
    "/api/sheets?at=" + showingSheet + "&uncue=" + i, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  await loadSheets();
  drawCues(sheets[showingSheet]);
  await reopen();
  status("Cue taken away");
}

async function applyPartSetup() {
  if (showingSheet < 0) return;
  // The voicing first: it is a separate call, and sending it after the
  // rest would leave the dialog closed if it were the thing that failed.
  if (!await applyVoicing()) return;
  const q = new URLSearchParams({
    at: String(showingSheet),
    name: $("#part-name").value,
    staves: $("#part-staves").value,
    size: $("#part-size").value,
    multirest: $("#part-multirest").checked ? "1" : "0",
  });
  const res = await fetch("/api/sheets?" + q, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  $("#part-setup")?.close();
  await loadSheets();
  // A different size, different staves or counted rests is a different
  // page, so the drawing has to be asked for again.
  await reopen();
  status("Part set");
}

async function removeShowingPart() {
  if (showingSheet < 0) return;
  const name = sheets[showingSheet]?.name || "that part";
  const sure = await confirmThat({
    title: `Take away the ${name} part?`,
    note: "The music stays in the score; it is the part that goes.",
    ok: "Take it away",
  });
  if (!sure) return;
  const res = await fetch("/api/sheets?at=" + showingSheet + "&remove=1",
                          { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  $("#part-setup")?.close();
  await loadSheets();
  await reopen();
  status("Took away " + name);
}

function setupPartDialog() {
  $("#part-apply")?.addEventListener("click", () => void applyPartSetup());
  $("#cue-add")?.addEventListener("click", () => void addCue());
  $("#part-remove")?.addEventListener("click", () => void removeShowingPart());

  // Turning to another staff shows that staff's rule rather than leaving
  // the one before it in the boxes, which would be read as this staff's
  // and applied to it on the way out.
  $("#voice-staff")?.addEventListener("change", () => {
    const sh = sheets[showingSheet];
    if (sh) showVoicingFor(sh);
  });
  $("#voice-pick")?.addEventListener("change", syncVoicingRows);
  $("#voice-layer")?.addEventListener("change", syncVoicingRows);
}

// tickItem marks the entry that is the one being shown. A list of things
// you can switch between has to say which one you are on.
function tickItem(b) {
  if (!b) return;
  const tick = document.createElement("kbd");
  tick.textContent = "✓";
  b.append(tick);
}

// showSheet puts a part on the screen, or the score.
//
// It is not an edit: which part is being looked at belongs to the window
// and not to the file, so nothing here makes the score unsaved.
async function showSheet(which) {
  const res = await fetch("/api/sheets?show=" + encodeURIComponent(which),
                          { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  await loadSheets();
  // The whole picture changes — a different number of staves, a different
  // size, bars of rest counted instead of drawn — so the cursor's bar is
  // no longer where it was and the layout has to be asked for again.
  await reopen();
  const name = which === "score" ? "the full score" : (sheets[which] || {}).name;
  status("Showing " + (name || "the full score"));
}

async function makeParts() {
  const res = await fetch("/api/sheets?make=1", { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  await loadSheets();
  status(sheets.length === 1
    ? "Made 1 part"
    : "Made " + sheets.length + " parts");
}

/* ---------- the scripts in the plugins folder ---------- */

// The plugins, as the server last read them out of the folder.
let plugins = [];
let pluginsFolder = "";

// loadPlugins fills the Scripts menu out of Gscore/plugins.
//
// Built from the folder rather than from a list the editor keeps, for the
// same reason the recent files are: the folder is the installation. A file
// dropped in there is on the menu next time the window opens, and a file
// deleted is gone, with nothing to register and nothing to tidy up.
//
// What each one is called comes out of a comment at the top of the file,
// which the server reads without running any of it. A menu built by
// executing what is in a folder to find out what it is called would be a
// menu that runs everything in the folder every time the editor starts.
async function loadPlugins() {
  const items = $("#scripts-items");
  if (!items) return;

  plugins = [];
  pluginsFolder = "";
  try {
    const res = await fetch("/api/plugins");
    if (res.ok) {
      const d = await res.json();
      plugins = d.plugins || [];
      pluginsFolder = d.folder || "";
    }
  } catch (_) {
    /* no folder is the same as an empty one */
  }

  items.innerHTML = "";
  const item = (label, command, title) => {
    const b = document.createElement("button");
    b.innerHTML = "<span></span>";
    b.querySelector("span").textContent = label;
    if (command) {
      b.dataset.action = command;
      // Wired here and not by setupMenus, which ran before these existed.
      b.addEventListener("click", () => {
        if (b.disabled) return;
        closeMenus();
        void action(command);
      });
    }
    if (title) b.title = title;
    items.append(b);
    return b;
  };

  // Only the ones that asked to be on a menu. A file with no @menu in it is
  // a half-written script, or one of the pieces another of them reads, and
  // either way it is in the folder on purpose and not on the menu.
  const onMenu = plugins.filter((p) => p.menu);
  if (!onMenu.length) {
    item("No scripts in the plugins folder", "").disabled = true;
  } else {
    for (const p of onMenu) {
      item(p.name, "plugin:" + p.file, p.about || p.file);
    }
  }
  items.append(document.createElement("hr"));
  // Where they live, said rather than opened: there is no asking the system
  // to show a folder from inside the page, and a path somebody can read is
  // more use than a menu entry that does nothing.
  item(pluginsFolder || "Scripts live in Gscore/plugins", "").disabled = true;
  item("Script console…", "console");

  // These were made after the menus were drawn, so they have no pictures
  // beside them yet. Half a column of pictures reads worse than none.
  syncMenuIcons();
  // The native menu bar is built from this one, so it has to be told that
  // what it copied has changed.
  rebuildNativeMenus();
}

// runPlugin runs one of them, with the console open on it.
//
// Open on it, which is the point: what ran is on the screen, in full, with
// whatever it said underneath. A plugin that reported five parallel fifths
// has somewhere to report them, and a plugin that did something surprising
// can be read afterwards to find out what.
//
// It runs, where a script carried by a score is only ever opened. The
// difference is whose file it is: a score arrives from anybody and may not
// run anything, and this folder is yours — putting a file in it is the
// consent, and clicking its name is the asking.
async function runPlugin(file) {
  const found = plugins.find((p) => p.file === file);
  if (!found) {
    status("There is no script called " + file + " in the plugins folder");
    return;
  }
  // Put it in the console box without opening the window, so that "Open in
  // the console" from the result panel has it there to read, and so that
  // opening the console after a plugin ran shows what ran.
  const source = $("#console-source");
  if (source) source.value = found.body;
  const name = $("#console-name");
  if (name) name.value = found.name;
  await runConsoleAndShow(found.body, found.name, found.about);
}

// setupScriptResult wires the corner panel's two buttons.
function setupScriptResult() {
  $("#script-result-again")?.addEventListener("click", () => {
    if (lastScript.source) {
      void runConsoleAndShow(lastScript.source, lastScript.name, lastScript.about);
    }
  });
  $("#script-result-open")?.addEventListener("click", () => {
    $("#script-result")?.close();
    const source = $("#console-source");
    if (source) source.value = lastScript.source;
    void openConsole();
  });
}

// documentName is what the score is called, with a star while there is work
// that is not on the disk. The window and the status line say the same thing
// because they are the same string.
function documentName(i) {
  return (i.filename || "untitled") + (i.unsaved ? " *" : "");
}

// tellNativeTitle gives the application the name to put in its title bar.
//
// The page is the only thing that knows: the application sets a title when
// it opens a file for you and then never hears about the editing, or about
// a file opened from inside the page. Everywhere but the Mac application
// nothing is listening and this does nothing.
function tellNativeTitle(name) {
  try {
    window.webkit?.messageHandlers?.gscore?.postMessage("title:" + name);
  } catch (_) {
    /* not inside the application */
  }
}

// rebuildNativeMenus asks the application to read the page's menus again.
//
// Only the Mac application has a menu bar of its own and only it is
// listening; everywhere else this is nothing.
function rebuildNativeMenus() {
  try {
    window.webkit?.messageHandlers?.gscore?.postMessage("menus");
  } catch (_) {
    /* not inside the application */
  }
}

async function openRecent(path) {
  status("Opening " + path + "…");
  busy("Opening " + path + "…");
  try {
    const res = await fetch("/api/openpath?path=" + encodeURIComponent(path), {
      method: "POST",
    });
    if (!res.ok) {
      status("Could not open it: " + (await res.text()).trim());
      // Gone or unreadable: the list says so next time it is asked.
      await loadRecent();
      return;
    }
    await reopen();
    await loadRecent();
    status("Opened " + (state.info?.filename || path));
  } finally {
    idle();
  }
}

/* ---------- instruments in the page ---------- */

// gscore running as a web page is the whole program: the core is compiled
// into it and answers its own requests. What it has not got is a disk, and a
// soundfont is thirty megabytes of somebody's recordings sitting on one.
//
// So the page fetches it once and keeps it in the browser's own store, which
// is what that store is for. After that it is loaded from there — no network,
// nothing to re-download — and handed to the core as bytes. Everything from
// that point is the same code that plays a soundfont on the desktop.

// instrumentStore is the cache the soundfont is kept in, named so that
// clearing it is an obvious thing to do and clears only this.
const instrumentStore = "gscore-instruments";
// And where the choice is remembered between visits.
const instrumentKey = "gscore-instrument-url";

const pageFont = { loaded: false, name: "", url: "" };

// canSound says whether gscore can play real instruments here at all.
function canSound() {
  return !!(state.info?.local || state.info?.inBrowser);
}

// storedInstrument is what was fetched last time, if the browser still has
// it. Nothing is downloaded here: a miss is a miss.
async function storedInstrument() {
  const url = (() => {
    try {
      return localStorage.getItem(instrumentKey) || "";
    } catch (_) {
      return "";
    }
  })();
  if (!url || !window.caches) return null;
  try {
    const cache = await caches.open(instrumentStore);
    const hit = await cache.match(url);
    if (!hit) return null;
    return { url, bytes: await hit.arrayBuffer() };
  } catch (_) {
    return null;
  }
}

// loadStoredInstruments hands the core whatever the browser already has.
//
// Quiet about failing: with nothing stored there is nothing to say, and the
// oscillators play. Saying "no instruments" every time somebody pressed play
// would be nagging about a download they have not asked for.
async function loadStoredInstruments() {
  if (pageFont.loaded) return true;
  const stored = await storedInstrument();
  if (!stored) return false;
  status("Loading the instruments…");
  const ok = await giveInstrumentsToCore(stored.bytes, nameFromURL(stored.url));
  if (ok) pageFont.url = stored.url;
  return ok;
}

// giveInstrumentsToCore hands over the bytes and remembers what came back.
async function giveInstrumentsToCore(bytes, name) {
  const res = await fetch("/api/soundfont/load?name=" + encodeURIComponent(name), {
    method: "POST",
    body: bytes,
  });
  if (!res.ok) {
    status("Those instruments could not be read: " + (await res.text()).trim());
    return false;
  }
  const d = await res.json();
  pageFont.loaded = true;
  pageFont.name = d.font.name;
  record.font = d.font.path;
  await saveSetting("playbackFont", d.font.path);
  return true;
}

function nameFromURL(url) {
  try {
    const base = decodeURIComponent(new URL(url, location.href).pathname.split("/").pop());
    return base.replace(/\.sf2$/i, "") || "soundfont";
  } catch (_) {
    return "soundfont";
  }
}

// fetchInstruments downloads a soundfont into the page and keeps it.
//
// With a running count, because this is tens of megabytes over somebody's
// connection and a page that says nothing for a minute is a page that has
// crashed.
async function fetchInstruments(url, label) {
  status("Downloading " + label + "…");
  let res;
  try {
    res = await fetch(url);
  } catch (e) {
    // Almost always the same rule, found the hard way: a link somebody
    // pasted, from a site nobody had checked.
    await explainDownload(label, url);
    return false;
  }
  if (!res.ok) {
    status("Could not download " + label + ": " + res.status);
    return false;
  }

  const total = Number(res.headers.get("Content-Length")) || 0;
  let bytes;
  if (res.body && res.body.getReader) {
    const reader = res.body.getReader();
    const parts = [];
    let got = 0;
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      parts.push(value);
      got += value.length;
      status(total
        ? `Downloading ${label}… ${Math.round((100 * got) / total)}%`
        : `Downloading ${label}… ${megabytes(got)}`);
    }
    bytes = new Blob(parts);
    bytes = await bytes.arrayBuffer();
  } else {
    bytes = await res.arrayBuffer();
  }

  // Kept before it is parsed: a soundfont the core refuses is still a
  // soundfont somebody waited to download, and the next attempt should not
  // have to wait again.
  try {
    if (window.caches) {
      const cache = await caches.open(instrumentStore);
      await cache.put(url, new Response(bytes.slice(0), {
        headers: { "Content-Type": "application/octet-stream" },
      }));
    }
    localStorage.setItem(instrumentKey, url);
  } catch (_) {
    // A browser with no room, or in private mode: it still plays, it just
    // downloads again next time.
    status("Downloaded, but there was no room to keep it for next time");
  }

  pageFont.loaded = false;
  if (!(await giveInstrumentsToCore(bytes, nameFromURL(url)))) return false;
  pageFont.url = url;
  status("Playing through " + pageFont.name);
  return true;
}

// forgetInstruments throws away what the page kept, for somebody who wants
// the room back or picked the wrong one.
async function forgetInstruments() {
  try {
    if (window.caches) await caches.delete(instrumentStore);
    localStorage.removeItem(instrumentKey);
  } catch (_) {
    /* nothing kept it in the first place */
  }
  // What the core has loaded stays loaded: it is in memory and throwing it
  // away would silence a page that is working perfectly well. The store is
  // what was asked about.
  status("The instruments will be downloaded again next time");
}

/* ---------- recording ---------- */

// What the recording dialog is showing: the soundfonts it knows about, which
// one is chosen, and which staves have been pointed at a different one.
const record = { fonts: [], catalogue: [], font: "", assign: {}, voices: [] };

// recordAudio writes the score out as sound.
//
// "Export as audio" in the menu rather than "record": recording is what you
// do to a performance that is happening, and nothing here is happening — the
// score is played through a soundfont and written to a file, which is an
// export like the others.
//
// The dialog exists because of the soundfont. A recording is only as good as
// the instruments it was played on, those are somebody else's thirty-megabyte
// file, and whether the one you have covers the instruments in this
// particular score is a real question with a real answer — a brass band
// scored into a piano soundfont comes out as nine pianos, silently.
async function recordAudio() {
  if (!canSound()) {
    // The menu item is not there where gscore cannot make a sound, so this
    // is only reachable by a keyboard shortcut or a stale page.
    status("Exporting audio needs gscore running on your own machine");
    return;
  }
  if (state.info?.inBrowser && !pageFont.loaded && !(await loadStoredInstruments())) {
    status("Choose the instruments first — Play ▸ Playback sound…");
    return;
  }
  await loadSoundfonts();
  const dlg = $("#record");

  const go = async () => {
    dlg.close();
    const p = new URLSearchParams({ as: "flac", ...recordSettings() });
    status("Exporting audio… this takes a few seconds");
    const res = await saveRequest(p.toString(), "Writing the file…");
    if (!res.ok) {
      status("Could not export: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    status(d.cancelled ? "Ready" : "Exported " + d.path);
  };

  $("#record-go").onclick = go;
  $("#record-cancel").onclick = () => dlg.close();
  $("#record-pick").onclick = pickSoundfont;
  $("#record-get").onclick = getSoundfont;
  // Everything back onto the one chosen above. The staves that were put on
  // soundfonts of their own are not remembered afterwards: this is the
  // simple answer to "how do I undo all that", and somebody who wants one
  // of them back sets it again, which is one press rather than the hunt
  // through the list they would otherwise be doing.
  $("#record-all").onclick = async () => {
    const own = Object.values(record.assign).filter(Boolean).length;
    if (!own) return;
    record.assign = {};
    await refreshVoices();
    status(own === 1
      ? "One staff moved onto the chosen soundfont"
      : own + " staves moved onto the chosen soundfont");
  };
  $("#record-font").onchange = async (e) => {
    record.font = e.target.value;
    // Remembered, so the next export opens on it. Written here rather than
    // when the recording is made, because somebody who changes their mind
    // and closes the dialog has still said which one they want.
    void saveSetting("recordFont", record.font);
    await refreshVoices();
  };
  dlg.showModal();
}

// recordSettings is the dialog as query parameters.
function recordSettings() {
  const assign = Object.entries(record.assign)
    .filter(([, path]) => path && path !== record.font)
    .map(([part, path]) => part + "=" + path)
    .join(",");
  return {
    font: record.font,
    assign,
    rate: $("#record-rate").value,
    tail: $("#record-tail").value,
    reverb: $("#record-reverb").checked ? "1" : "0",
    normalize: $("#record-normalize").checked ? "1" : "0",
  };
}

async function loadSoundfonts() {
  // The one chosen last time, before anything is asked for: a recording is
  // rendered through whichever bank was picked for it, and picking it again
  // on every export is the kind of question a program should only ask once.
  if (!record.font) record.font = await rememberedRecordFont();
  const p = new URLSearchParams({ font: record.font });
  const res = await fetch("/api/soundfonts?" + p.toString());
  if (!res.ok) return;
  const d = await res.json();
  record.fonts = d.installed || [];
  record.catalogue = d.catalogue || [];
  record.collection = d.collection || null;
  record.voices = d.voices || [];
  record.dir = d.dir;
  // Whatever was remembered has to still be there: a soundfont is a file,
  // and a file that has been deleted or moved since would leave the dialog
  // pointing at nothing with no way to tell.
  if (record.font && !record.fonts.some((f) => f.path === record.font)) {
    record.font = "";
  }
  if (!record.font && record.fonts.length) record.font = record.fonts[0].path;
  drawSoundfonts();
  await refreshVoices();
}

function drawSoundfonts() {
  const sel = $("#record-font");
  sel.innerHTML = "";
  for (const f of record.fonts) {
    const o = document.createElement("option");
    o.value = f.path;
    o.textContent = f.name + " — " + megabytes(f.size) + ", " + f.where;
    o.selected = f.path === record.font;
    sel.append(o);
  }
  if (!record.fonts.length) {
    const o = document.createElement("option");
    o.value = "";
    o.textContent = "none yet — download one";
    sel.append(o);
  }
  $("#record-font-note").textContent = record.fonts.length
    ? ""
    : "Gscore does not come with instruments: they are recordings, and a good "
      + "set is tens of megabytes. Download one here, or point at any .sf2 you "
      + "already have.";
}

// downloadSoundfont fetches one and shows how it is getting on.
//
// The request answers when the file has arrived, which for two hundred
// megabytes is minutes; the counting is a second conversation, asked for
// under a job the server files the progress under. So the download is
// started and deliberately not waited for, the bar is driven by asking, and
// the two meet at the end.
//
// Returns the response where it worked, and nothing where it did not —
// having already said why.
async function downloadSoundfont(url, name) {
  const job = "dl" + Date.now() + "-" + Math.random().toString(36).slice(2, 8);
  const query = new URLSearchParams(name ? { url, name, job } : { url, job });

  const dlg = $("#sfdownload");
  $("#sfdownload-title").textContent = "Downloading " + (name || "a soundfont");
  $("#sfdownload-what").textContent =
    "It is kept in " + (record.dir || "your Gscore folder")
    + ", and every score you write afterwards will find it.";
  $("#sfdownload-bar").value = 0;
  $("#sfdownload-bar").removeAttribute("value"); // indeterminate until the size is known
  const numbers = $("#sfdownload-numbers");
  numbers.className = "";
  numbers.textContent = "Starting…";
  dlg.showModal();

  const fetching = fetch("/api/soundfont/fetch?" + query.toString(),
    { method: "POST" });

  let watching = true;
  const watch = async () => {
    while (watching) {
      await new Promise((r) => setTimeout(r, 200));
      if (!watching) break;
      try {
        const r = await fetch("/api/soundfont/progress?job=" + job);
        if (!r.ok) continue;
        const d = await r.json();
        drawDownload(d);
        if (d.done) break;
      } catch (_) { /* the answer comes from the download itself */ }
    }
  };
  void watch();

  let res = null;
  try {
    res = await fetching;
  } catch (e) {
    watching = false;
    if (dlg.open) dlg.close();
    status("Could not download: " + e);
    return null;
  }
  watching = false;
  if (dlg.open) dlg.close();

  if (!res.ok) {
    status("Could not download: " + (await res.text()).trim());
    return null;
  }
  return res;
}

// drawDownload puts one answer from the server on the bar.
function drawDownload(d) {
  const bar = $("#sfdownload-bar");
  const numbers = $("#sfdownload-numbers");
  const got = d.received || 0;
  const total = d.total || 0;
  if (total > 0) {
    bar.value = Math.min(1, got / total);
  } else {
    // A server that would not say how big it is. The bar runs rather than
    // filling, because a bar that pretends to a fraction it cannot know is
    // worse than one that only says "still going".
    bar.removeAttribute("value");
  }
  if (d.error) {
    numbers.className = "failed";
    numbers.textContent = d.error;
    return;
  }
  const parts = [total > 0 ? megabytes(got) + " of " + megabytes(total)
    : megabytes(got)];
  if (d.bytesPerSecond) parts.push(megabytes(d.bytesPerSecond) + "/s");
  if (d.secondsLeft > 0 && !d.done) parts.push(timeLeft(d.secondsLeft));
  if (d.done) parts.push("reading it");
  numbers.textContent = parts.join(" · ");
}

// timeLeft says how long is left, in units somebody waiting can use.
function timeLeft(s) {
  if (s < 1) return "nearly there";
  if (s < 60) return Math.round(s) + "s left";
  const m = Math.floor(s / 60);
  return m + "m " + Math.round(s - m * 60) + "s left";
}

// rememberedRecordFont is the soundfont the last recording was rendered
// through, or the one the editor plays through where there is none.
//
// Asked of the settings rather than kept in the page, because the page is
// reloaded whenever the application is opened and the answer is supposed to
// outlive that. An answer that cannot be had is no answer: the dialog then
// falls back to the first bank installed, as it always did.
async function rememberedRecordFont() {
  try {
    const res = await fetch("/api/settings");
    if (!res.ok) return "";
    const d = await res.json();
    return d.recordFont || d.playbackFont || "";
  } catch (_) {
    return "";
  }
}

function megabytes(n) {
  return (n / 1e6).toFixed(n > 1e8 ? 0 : 1) + " MB";
}

// refreshVoices asks what each staff is scored for and what the chosen
// soundfont will actually play it with.
async function refreshVoices() {
  const assign = Object.entries(record.assign)
    .filter(([, path]) => path)
    .map(([part, path]) => part + "=" + path)
    .join(",");
  const p = new URLSearchParams({ font: record.font, assign });
  const res = await fetch("/api/soundfonts?" + p.toString());
  if (res.ok) record.voices = (await res.json()).voices || [];
  drawVoices();
}

function drawVoices() {
  const body = $("#record-voices").querySelector("tbody");
  body.innerHTML = "";
  let missing = 0;
  for (const v of record.voices) {
    const tr = document.createElement("tr");

    const name = document.createElement("td");
    name.innerHTML = "<b></b><br><span class='small'></span>";
    name.querySelector("b").textContent = v.name;
    name.querySelector("span").textContent = v.drum
      ? "percussion"
      : v.instrument + " — program " + v.program;
    tr.append(name);

    const sound = document.createElement("td");
    if (v.missing) {
      missing++;
      sound.className = "missing";
      sound.textContent = "nothing in this soundfont";
    } else if (v.substitute) {
      // A soundfont of one instrument files it wherever it likes, so this is
      // the ordinary case for those and a mistake for a General MIDI bank.
      // Which it is depends on what was chosen, so it is shown and not
      // judged.
      sound.className = "substitute";
      sound.textContent = v.preset + " — the only thing it has";
    } else {
      sound.textContent = v.preset || "—";
    }
    tr.append(sound);

    // Any staff can be played through a different soundfont: a piano
    // library for the piano and an orchestra for the rest is an ordinary
    // thing to want, and nothing else can do it once the file is written.
    const pick = document.createElement("td");
    const sel = document.createElement("select");
    const def = document.createElement("option");
    def.value = "";
    def.textContent = "the chosen soundfont";
    sel.append(def);
    for (const f of record.fonts) {
      const o = document.createElement("option");
      o.value = f.path;
      o.textContent = f.name;
      o.selected = record.assign[v.part] === f.path;
      sel.append(o);
    }
    sel.onchange = async (e) => {
      record.assign[v.part] = e.target.value;
      await refreshVoices();
    };
    pick.append(sel);
    tr.append(pick);

    body.append(tr);
  }
  // How many staves have been put on a soundfont of their own, which is
  // what the way back is for and what tells somebody it is there at all.
  const own = Object.values(record.assign).filter(Boolean).length;
  const all = $("#record-all");
  if (all) {
    all.hidden = own === 0;
    all.textContent = own === 1
      ? "Use this for every staff (1 is on another)"
      : "Use this for every staff (" + own + " are on others)";
  }
  $("#record-voices-note").textContent = missing
    ? "This soundfont has nothing for " + missing
      + (missing === 1 ? " instrument" : " instruments")
      + " below. They will be silent — choose another soundfont for them."
    : own
      ? "What each staff is scored for. " + (own === 1 ? "One staff is" : own
        + " staves are") + " on a soundfont of their own; the rest follow "
        + "the one chosen above."
      : "What each staff is scored for, and what the soundfont will play it "
        + "with. Any staff can be put on a different one.";
  $("#record-go").disabled = !record.font;
}

// browseCollection walks the ZanderJaz collection: a category, then one of
// the instruments in it.
//
// Two questions rather than a list of five hundred, because the categories
// are the only thing anybody knows about a collection they have not heard:
// somebody looking for a Rhodes looks under pianos, not through everything
// beginning with R.
async function browseCollection() {
  const res = await fetch("/api/soundfont/collection");
  if (!res.ok) return null;
  const d = await res.json();

  const category = await ask({
    title: "ZanderJaz collection",
    label: "Which kind",
    note: d.about,
    options: d.categories.map((c) => ({
      value: c.name,
      label: c.name.replace(/-/g, " ") + " — " + c.fonts.length,
    })),
  });
  if (!category) return null;

  const found = d.categories.find((c) => c.name === category);
  if (!found) return null;
  const url = await ask({
    title: "ZanderJaz — " + category.replace(/-/g, " "),
    label: "Which one",
    search: true,
    options: found.fonts.map((f) => ({ value: f.url, label: f.name })),
  });
  if (!url) return null;
  return { url, name: found.fonts.find((f) => f.url === url)?.name || "" };
}

async function pickSoundfont() {
  if (state.info?.inBrowser) {
    const file = await pickLocalFile("#sf2-input");
    if (!file) return;
    status("Reading " + file.name + "…");
    pageFont.loaded = false;
    if (await giveInstrumentsToCore(await file.arrayBuffer(), file.name)) {
      status("Playing through " + pageFont.name);
    }
    return;
  }
  status("Choose a soundfont…");
  const res = await fetch("/api/soundfont/pick", { method: "POST" });
  if (!res.ok) {
    status("Could not read that: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  if (d.cancelled || d.unavailable) {
    status("Ready");
    return;
  }
  record.fonts = [d.font, ...record.fonts.filter((f) => f.path !== d.font.path)];
  record.font = d.font.path;
  drawSoundfonts();
  await refreshVoices();
  status("Using " + d.font.name);
}

// getSoundfont downloads one, either from the short list or from a link.
async function getSoundfont() {
  // Everything is offered, here and in the page alike. Some of it the page
  // is not allowed to fetch for itself — a browser will not hand a page
  // bytes from a site that has not said other sites may read them — and
  // those are not dropped from the list, because the person's own browser
  // is under no such rule and can fetch any of them. What the page cannot
  // do for them, it tells them how to do.
  const page = !!state.info?.inBrowser;
  const options = record.catalogue.map((c) => ({
    value: c.id,
    label: c.name + " — " + megabytes(c.size)
      + (page && !c.browser ? " (download it yourself)" : ""),
  }));
  if (record.collection?.count) {
    options.push({
      value: "collection",
      label: "ZanderJaz collection — " + record.collection.count + " instruments…"
        + (page && !record.collection.browser ? " (download them yourself)" : ""),
    });
  }
  // A soundfont already on this machine, which is the first thing to try
  // and was the one thing this list did not offer.
  //
  // It was offered only in the page, which is backwards: in a browser it
  // is a file input and the bytes have to be handed to the core, and on a
  // machine it is the system's own open panel over the whole disk. Anybody
  // who already owns a soundfont — and anybody who has downloaded one
  // outside gscore — was being asked for a web address for a file sitting
  // in their Downloads folder.
  options.unshift({
    value: "file",
    label: page
      ? "Open a .sf2 from this computer…"
      : "Choose a .sf2 on this computer…",
  });
  options.push({ value: "url", label: "From a link to a .sf2 file…" });

  // What each one is goes in the note rather than in the list: a list of
  // soundfonts tells you nothing, and the sentence about what is in one and
  // whether you may use it is the part worth reading.
  const about = record.catalogue
    .map((c) => c.name + ": " + c.about + " " + c.license)
    .join("\n");
  const choice = await ask({
    title: "Download a soundfont",
    label: "Which one",
    options,
    note: page
      ? about + "\nKept by this page, so a later visit has it already and "
        + "nothing is fetched twice. The ones marked “download it yourself” "
        + "are on sites that do not let a page fetch from them; pick one and "
        + "gscore will show you the link and take the file."
      : about + "\nKept in " + (record.dir || "your settings folder")
        + ", where every score you write afterwards will find it.",
  });
  if (!choice) return;

  let url = "";
  let name = "";
  let fetchable = true;
  if (choice === "file") {
    await pickSoundfont();
    return;
  }
  if (choice === "url") {
    url = await ask({ title: "Download a soundfont", label: "Link to a .sf2 file" });
    if (!url) return;
  } else if (choice === "collection") {
    const picked = await browseCollection();
    if (!picked) return;
    url = picked.url;
    name = picked.name;
    fetchable = !!record.collection?.browser;
  } else {
    const entry = record.catalogue.find((c) => c.id === choice);
    if (!entry) return;
    url = entry.url;
    name = entry.name;
    fetchable = !!entry.browser;
  }

  // In the page there is nowhere to put a file, so it is kept in the
  // browser's own store and handed to the core as bytes — where the page is
  // allowed to fetch it at all. Where it is not, the person is shown how.
  if (state.info?.inBrowser) {
    if (!fetchable) {
      await explainDownload(name || nameFromURL(url), url);
      return;
    }
    await fetchInstruments(url, name || nameFromURL(url));
    return;
  }

  const res = await downloadSoundfont(url, name);
  if (!res) return;
  const d = await res.json();
  record.fonts = [d.font, ...record.fonts.filter((f) => f.path !== d.font.path)];
  record.font = d.font.path;
  drawSoundfonts();
  await refreshVoices();
  status("Downloaded " + d.font.name);
}

/* ---------- editing ---------- */

// Every edit changes the model on the server and the score is drawn again.
// Nothing is patched in place, because the drawing holds no music and the
// music holds no geometry; that separation is what makes this simple.

// playbackData is the performance, worked out once and kept until the music
// changes.
//
// Working one out is every note of every repeat with its own time, its own
// loudness and its own instrument: a hundred and thirty-two thousand of
// them on a symphony, and eighteen megabytes of JSON. In the page the core
// writing that is WebAssembly, so it is a second rather than a moment —
// and it was paid again every time Play was pressed, for a performance that
// had not changed since the last one.
//
// Let go whenever the hit regions are, which is whenever the music has been
// touched.
async function playbackData() {
  if (state.playback) return state.playback;
  try {
    const d = await (await fetch("/api/playback")).json();
    // In the order it is played, once. Everything downstream either wants
    // that order or does not care, and sorting a hundred and thirty-two
    // thousand notes is not work to do twice.
    (d.events || []).sort((a, b) => a.start - b.start);
    state.playback = d;
    return d;
  } catch (_) {
    return null;
  }
}

async function loadHits() {
  // The music may have changed, so the performance worked out from it is no
  // longer the performance.
  state.playback = null;
  // The same picture the score is drawn from — viewParams and not the mode
  // alone. A staff space or a width that belongs to a different picture
  // puts the notes somewhere they are not: the editor sends a click to the
  // wrong bar, and an embedded score, whose systems are wrapped to the
  // frame, measures a system the reader is not looking at.
  const res = await fetch("/api/hits?" + viewParams().toString());
  const d = await res.json();
  state.hits = d.hits || [];
  state.dynamics = d.dynamics || [];
  state.textMarks = d.textMarks || [];
  state.markers = d.markers || [];
  state.articulations = d.articulations || [];
  state.chordMarks = d.chords || [];
  state.tempos = d.tempos || [];
  state.hairpins = d.hairpins || [];
  state.arcs = d.arcs || [];
  state.tremolos = d.tremolos || [];
  state.tuplets = d.tuplets || [];
  state.barlines = d.barlines || [];
  state.headers = d.headers || [];
  state.strips = d.strips || [];
  state.stripIndex = indexStrips(state.strips);
  state.hitsByBar = indexHits(state.hits);
}

// indexHits files the clickable regions by which bar they are part of.
function indexHits(hits) {
  const bars = new Map();
  for (const h of hits || []) {
    const got = bars.get(h.measure);
    if (got) got.push(h);
    else bars.set(h.measure, [h]);
  }
  return bars;
}

// hitsIn is the regions belonging to one bar, on every staff.
function hitsIn(index) {
  const bars = state.hitsByBar;
  if (bars) return bars.get(index) || [];
  return (state.hits || []).filter((h) => h.measure === index);
}

// dynamicAt finds the dynamic a click landed on.
//
// A dynamic sits under the staff, which is inside the bar's clickable region
// but nowhere near the note it belongs to. So a click on an ff used to be
// read as a click on the empty space below that note — which, with a note
// tool in hand, wrote a note there. Writing one was easy and pointing at the
// result was impossible.
//
// The box comes from the engraver, because only the engraver knows how far
// down this stretch of music pushed it.
function dynamicAt(x, y) {
  for (const d of state.dynamics || []) {
    if (x >= d.x - 0.4 && x <= d.x + d.width + 0.4 &&
        y >= d.y - 0.4 && y <= d.y + d.height + 0.4) {
      return d;
    }
  }
  return null;
}

// articulationAt finds the mark on a note that a click landed on.
//
// A staccato dot or a fermata is not where its note is: it is drawn clear
// of the notehead, the stem and the staff, as far out as that stretch of
// music needed. The box comes from the engraver for the same reason every
// other one does.
function articulationAt(x, y) {
  for (const a of state.articulations || []) {
    if (x >= a.x - 0.4 && x <= a.x + a.width + 0.4 &&
        y >= a.y - 0.4 && y <= a.y + a.height + 0.4) {
      return a;
    }
  }
  return null;
}

// markerAt finds the rehearsal mark a click landed on.
//
// A letter belongs to a bar and not to any note in it, and is written in a
// row above the music as far up as that bar needed — so there was nothing on
// the page to point at: taking one away meant finding the bar, selecting a
// note in it and asking the window. The box comes from the engraver, which
// is the only thing that knows where the row ended up.
function markerAt(x, y) {
  for (const m of state.markers || []) {
    if (x >= m.x - 0.4 && x <= m.x + m.width + 0.4 &&
        y >= m.y - 0.4 && y <= m.y + m.height + 0.4) {
      return m;
    }
  }
  return null;
}

// textMarkAt finds the words a click landed on.
//
// The box comes from the engraver for the reason a dynamic's does: words
// written on a note are not where the note is. They sit as far above or
// below the staff as that stretch of music pushed them, and until the staff
// was drawn nothing knew how far. Without this they could be written and
// never pointed at again — which is how somebody ends up with a "pizz." on
// the page and no way to take it off.
function textMarkAt(x, y) {
  for (const t of state.textMarks || []) {
    if (x >= t.x - 0.4 && x <= t.x + t.width + 0.4 &&
        y >= t.y - 0.4 && y <= t.y + t.height + 0.4) {
      return t;
    }
  }
  return null;
}

// chordSymbolAt finds the chord symbol a click landed on.
//
// The box comes from the engraver for the same reason a dynamic's does: the
// symbols of a staff are written on one line above all of its music, as far
// up as the highest note pushed them, and nothing else knows where that
// turned out to be. A click on "C7" used to find the empty air above the
// notes and, with a note tool in hand, write a note into it.
function chordSymbolAt(x, y) {
  for (const c of state.chordMarks || []) {
    if (x >= c.x - 0.4 && x <= c.x + c.width + 0.4 &&
        y >= c.y - 0.4 && y <= c.y + c.height + 0.4) {
      return c;
    }
  }
  return null;
}

// hairpinAt finds the swell a click landed on.
//
// A hairpin is a wedge under a stretch of music, belonging to the two notes
// at its ends and to none of the ones between. Taking one away means taking
// away both ends, which is why the engraver reports both.
function hairpinAt(x, y) {
  for (const h of state.hairpins || []) {
    if (x >= h.x - 0.3 && x <= h.x + h.width + 0.3 &&
        y >= h.y - 0.5 && y <= h.y + h.height + 0.5) {
      return h;
    }
  }
  return null;
}

// arcAt finds the tie or slur a click landed on.
//
// Against the curve itself, not a box round it. A slur arcs over the music it
// covers, and anything enclosing the curve encloses that music with it: a box
// would have swallowed every click on the notes of a slurred phrase.
//
// The ends are left out. They sit on the two noteheads the curve joins, where
// a click means the note — so only the middle of the arc answers, which is
// the part drawn in clear air anyway.
function arcAt(x, y) {
  let best = null;
  let away = Infinity;
  for (const a of state.arcs || []) {
    const d = awayFromArc(a, x, y);
    if (d > (arcReach[a.kind] ?? arcReach.tie) || d >= away) continue;
    best = a;
    away = d;
  }
  return best;
}

// arcReach is how near a click has to be to a curve to count as on it, in
// staff spaces.
//
// A curve is drawn a fifth of a space thick, and asking for a click within
// that was asking for the line itself: at eight pixels to the space a slur
// was a three-pixel target in open air, and finding it was a hunt. So the
// reach is the curve's neighbourhood rather than the curve.
//
// Wider for a slur than for a tie. A slur is drawn clear of the music it
// covers and there is nothing else up there to hit, so a generous reach
// costs nothing. A tie is tucked between the two noteheads it joins, and
// the gap between them is where a click means "put the cursor here".
const arcReach = { slur: 1.2, tie: 0.7 };

// awayFromArc is how far a point is from a quadratic, by walking it.
//
// Sampled rather than solved: the exact distance from a point to a quadratic
// is the root of a cubic, and a curve two or three spaces wide is answered
// to well within a notehead by thirty steps along it.
function awayFromArc(a, x, y) {
  // The ends belong to the notes the curve joins. A ring is left clear
  // around each of them, wider than the reach — so no click near enough to
  // count as on the curve can also be a click on the note it springs from,
  // or on the air above that note, which is where clicking means building a
  // chord on it.
  const free = (arcReach[a.kind] ?? arcReach.tie) + 0.1;
  let best = Infinity;
  let middle = Infinity;
  for (let i = 0; i <= 30; i++) {
    const t = i / 30;
    const u = 1 - t;
    const px = u * u * a.x1 + 2 * u * t * a.cx + t * t * a.x2;
    const py = u * u * a.y1 + 2 * u * t * a.cy + t * t * a.y2;
    const d = Math.hypot(px - x, py - y);
    if (i === 15) middle = d;
    if (Math.hypot(px - a.x1, py - a.y1) < free) continue;
    if (Math.hypot(px - a.x2, py - a.y2) < free) continue;
    if (d < best) best = d;
  }
  // A curve too short to have any middle outside those rings — a tie
  // between two neighbours — is answered by its midpoint alone, which is
  // the one point on it that is not an end.
  return best === Infinity ? middle : best;
}

// tremoloAt finds the two-note tremolo a click landed on.
//
// The beams between the two stems are the whole of the mark: it is not a
// glyph on a note and there is nothing else on the page that stands for it.
// A click on them fell through to whatever was underneath, so there was no
// way to pick a tremolo up — or to see that one had been picked up, which
// is the same thing from where anybody is sitting.
//
// The box is drawn tight. The beams run over the noteheads of the two notes
// they join, and a generous target here would be a notehead that cannot be
// clicked.
function tremoloAt(x, y) {
  for (const t of state.tremolos || []) {
    if (x >= t.x && x <= t.x + t.width &&
        y >= t.y - 0.2 && y <= t.y + t.height + 0.2) {
      return t;
    }
  }
  return null;
}

// tupletAt finds the tuplet figure a click landed on.
//
// The figure — the 3 over a triplet, the 12 over a run — belongs to all of
// the notes under it and to none of them, so nothing in the music could be
// clicked to reach it. The box comes from the engraver, which is the only
// thing that knows how far over the group the figure had to go.
//
// Over a beamed group the box is the figure alone, not the width of the
// run: the notes are right underneath, and a box across them would swallow
// every click meant for a note.
function tupletAt(x, y) {
  for (const t of state.tuplets || []) {
    if (x >= t.x - 0.3 && x <= t.x + t.width + 0.3 &&
        y >= t.y - 0.3 && y <= t.y + t.height + 0.3) {
      return t;
    }
  }
  return null;
}

// tempoAt finds the tempo marking a click landed on.
//
// Like a dynamic, a tempo marking belongs to a bar rather than to a note and
// is written off the staff, so nothing in the music can be clicked to reach
// it. Unlike a dynamic it sits above everything, where a click used to find
// nothing at all.
function tempoMarkAt(x, y) {
  for (const t of state.tempos || []) {
    if (x >= t.x - 0.4 && x <= t.x + t.width + 0.4 &&
        y >= t.y - 0.4 && y <= t.y + t.height + 0.4) {
      return t;
    }
  }
  return null;
}

// headerAt finds the clef, key signature or meter a click landed on.
function headerAt(x, y) {
  for (const h of state.headers) {
    if (x >= h.x0 && x <= h.x1 && y >= h.top && y <= h.bottom) return h;
  }
  return null;
}

// stripAt finds the bar whose band a point is in.
//
// The band sits over the music, where the bar numbers are. It is how a whole
// bar is selected: "key change from here" is something said about a bar, and
// clicking inside one finds a note instead — a full bar has no empty space to
// click in at all.
// stripBin is how wide one bucket of the index below is, in staff spaces.
// A bar is narrower than this, so a strip falls in one bucket or two.
const stripBin = 64;

// indexStrips files the bars by where they are across the page.
//
// stripAt is asked on every movement of the pointer, and walking every bar
// of every staff to answer it is a hundred and thirty-eight thousand
// comparisons on a symphony — for one frame of hover. Bucketed, it is the
// few bars that share a stretch of the page.
function indexStrips(strips) {
  const bins = new Map();
  for (const st of strips) {
    const lo = Math.floor(st.x0 / stripBin);
    const hi = Math.floor(st.x1 / stripBin);
    for (let b = lo; b <= hi; b++) {
      const got = bins.get(b);
      if (got) got.push(st);
      else bins.set(b, [st]);
    }
  }
  return bins;
}

function stripAt(x, y) {
  const bins = state.stripIndex;
  const here = bins ? bins.get(Math.floor(x / stripBin)) : state.strips;
  for (const st of here || []) {
    if (x >= st.x0 && x <= st.x1 && y >= st.top && y <= st.bottom) return st;
  }
  return null;
}

// barlineAt finds the barline a click landed on.
//
// A barline is a thin line and nobody clicks a thin line exactly, so the
// target is a band around it — narrow enough not to swallow clicks meant for
// the notes on either side.
function barlineAt(x, y) {
  for (const b of state.barlines) {
    if (Math.abs(x - b.x) > 0.7) continue;
    if (y < b.top - 1 || y > b.bottom + 1) continue;
    return b;
  }
  return null;
}

// hitAt finds the measure under a point given in staff spaces.
function hitAt(x, y) {
  // As far as the music actually goes, not a guess at it.
  //
  // The reach above and below used to be a fixed five spaces, which covers
  // a couple of ledger lines and no more. A flute run climbing to four and
  // a half spaces above the staff was therefore right on the edge of being
  // clickable at all, and the notes at the top of it could not be picked up
  // without landing the pointer inside half a space — which is what made
  // selecting a long run across the top of the staff a struggle.
  let best = null;
  let bestDist = Infinity;
  for (const h of state.hits) {
    if (x < h.x0 || x > h.x1) continue;
    const staffBottom = h.staffTop + staffHeightOf(h);
    let top = h.staffTop - 5;
    let bottom = staffBottom + 5;
    for (const el of h.elements || []) {
      const a = Math.min(el.y, el.y2 ?? el.y) - 1.5;
      const b = Math.max(el.y, el.y2 ?? el.y) + 1.5;
      if (a < top) top = a;
      if (b > bottom) bottom = b;
    }
    if (y < top || y > bottom) continue;
    // Where two staves reach the same place — the two hands of a piano, a
    // run below one and a run above the other — the nearer staff owns it.
    const d = y < h.staffTop ? h.staffTop - y
      : Math.max(0, y - staffBottom);
    if (d < bestDist) {
      best = h;
      bestDist = d;
    }
  }
  return best;
}

// stepAt converts a y position into a staff step: 0 is the bottom line, and
// each step is half a space, so the value is snapped to the nearest line or
// space rather than left continuous.
function stepAt(hit, y) {
  return Math.round((hit.staffTop + 4 - y) * 2);
}

// insertIndex decides where in the measure a click falls.
function insertIndex(hit, x) {
  const els = hit.elements || [];
  for (let i = 0; i < els.length; i++) {
    if (x < els[i].x) return i;
  }
  return els.length;
}

// columnAt finds the element whose column a point falls in, whatever its
// height.
//
// Height says which pitch, not which note: a click a sixth above a written
// note is still a click on that note's column, and treating it as one is what
// lets a chord be built by clicking above what is already there.
function columnAt(hit, x) {
  for (const el of hit.elements || []) {
    if (x >= el.x - 0.4 && x <= el.x + el.width + 0.4) return el;
  }
  return null;
}

// elementAt finds the element under a point, if the click is near enough to
// one to have meant it.
function elementAt(hit, x, y) {
  let best = null;
  let bestDist = Infinity;
  for (const el of hit.elements || []) {
    // Horizontally within the notehead with a little slack; vertically within
    // about a staff space of the element, which for a chord means anywhere
    // between its lowest and highest notehead.
    const dx = x < el.x ? el.x - x : Math.max(0, x - (el.x + el.width));
    const top = Math.min(el.y, el.y2 ?? el.y);
    const bottom = Math.max(el.y, el.y2 ?? el.y);
    const dy = y < top ? top - y : Math.max(0, y - bottom);
    if (dx > 0.6 || dy > 1.2) continue;

    const d = dx * dx + dy * dy;
    if (d < bestDist) {
      best = el;
      bestDist = d;
    }
  }
  return best;
}

// setupStripHover shows the band under the pointer.
//
// An invisible click target that nobody discovers is the same as no click
// target. It appears on approach and goes when the pointer leaves, so it
// costs nothing to look at and explains itself the first time the pointer
// crosses it.
function setupStripHover() {
  const wrap = $("#canvas-wrap");
  let shown = null;
  // The band itself, made once and moved, rather than made and thrown away.
  //
  // It used to be found with querySelectorAll over the drawing and then put
  // back with insertBefore. On a symphony the drawing is nine hundred
  // thousand shapes: the search walked all of them and the insertion made
  // the browser lay all of them out again, twice for every movement of the
  // pointer. Scrolling with the pointer over the music was unusable.
  let band = null;
  let bandIn = null;

  const bandFor = (svg) => {
    if (band && bandIn === svg) return band;
    band = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    band.setAttribute("class", "strip");
    svg.insertBefore(band, svg.firstChild);
    bandIn = svg;
    return band;
  };

  const clear = () => {
    // Hidden where it is. Taking it out of the drawing is what cost the
    // layout; leaving it there and empty costs nothing.
    if (band) band.setAttribute("width", "0");
    document.body.classList.remove("over-strip");
    shown = null;
  };

  // At most one of these per frame. A mouse reports its position far more
  // often than the screen is redrawn, and the work below is only worth
  // doing once per picture.
  let queued = null;
  let frame = 0;

  const show = () => {
    frame = 0;
    const e = queued;
    queued = null;
    if (!e) return;
    if (state.readOnly || state.playing) {
      if (shown !== null) clear();
      return;
    }
    const svg = currentSVG();
    if (!svg) return;
    const r = svg.getBoundingClientRect();
    const x = (e.x - r.left) / state.zoom;
    const y = (e.y - r.top) / state.zoom;

    const strip = stripAt(x, y);
    if (!strip) {
      if (shown !== null) clear();
      return;
    }
    if (shown === strip.measure) return;
    shown = strip.measure;

    const z = state.zoom;
    const b = bandFor(svg);
    b.setAttribute("x", String(strip.x0 * z));
    b.setAttribute("y", String(strip.top * z));
    b.setAttribute("width", String((strip.x1 - strip.x0) * z));
    b.setAttribute("height", String((strip.bottom - strip.top) * z));
    b.setAttribute("rx", String(0.3 * z));
    document.body.classList.add("over-strip");
  };

  wrap.addEventListener("mousemove", (e) => {
    queued = { x: e.clientX, y: e.clientY };
    if (!frame) frame = requestAnimationFrame(show);
  });

  wrap.addEventListener("mouseleave", () => {
    queued = null;
    clear();
    document.body.classList.remove("over-strip");
  });
}

function setupCanvasClicks() {
  $("#canvas").addEventListener("click", async (e) => {
    // A modified click is a pan gesture, and the click that ends a drag is
    // not a click at all.
    if (e.ctrlKey || e.metaKey || panned) return;
    if (suppressClick) {
      suppressClick = false;
      return;
    }

    // In a book, a click on a tune that is not the current one switches to
    // it. Playing, editing and the cursor all follow the current tune, so
    // picking one is the first thing a click can mean.
    const block = e.target.closest ? e.target.closest(".tune") : null;
    if (block) {
      const i = Number(block.dataset.tune);
      if (i !== state.info?.current) {
        await selectTune(i);
        return;
      }
    }

    const svg = currentSVG();
    if (!svg) return;
    const r = svg.getBoundingClientRect();
    const x = (e.clientX - r.left) / state.zoom;
    const y = (e.clientY - r.top) / state.zoom;

    // The band over a bar picks up the whole bar.
    const strip = stripAt(x, y);
    if (strip) {
      selectMeasure(state.selection?.part ?? 0, strip.measure);
      return;
    }

    // A barline is its own thing to click: the line between two bars, which
    // belongs to neither of them. Plain, it asks what kind of barline to
    // draw. With shift it picks up the bar it closes, which is how you say
    // "from here" to a key or a meter — and taking the click for the dialog
    // whatever was held meant the bar beside a barline could not be chosen
    // at all.
    const bar = barlineAt(x, y);
    if (bar) {
      if (e.shiftKey) {
        selectMeasure(bar.part, bar.measure);
      } else {
        await openBarlinePicker(bar);
      }
      return;
    }

    // So are the clef, the key signature and the meter at the head of a
    // system. Clicking the thing you want to change is the obvious way to
    // change it, and far better than hunting through a menu for its name.
    const head = headerAt(x, y);
    if (head) {
      await openHeaderPicker(head);
      return;
    }

    // Before the staff, because the box is under it: a click here means the
    // marking, not the space below the note.
    const dyn = dynamicAt(x, y);
    if (dyn) {
      selectDynamic(dyn);
      return;
    }

    // And above it, for the same reason.
    const tempo = tempoMarkAt(x, y);
    if (tempo) {
      selectTempo(tempo);
      return;
    }

    // The marks written on a note, which are above it or below it.
    const artic = articulationAt(x, y);
    if (artic) {
      selectArticulation(artic);
      return;
    }

    // The rehearsal marks are in the row above everything.
    const letter = markerAt(x, y);
    if (letter) {
      selectMarker(letter);
      return;
    }

    // Words written on a note, which are above the staff or below it.
    const words = textMarkAt(x, y);
    if (words) {
      selectTextMark(words);
      return;
    }

    // The chord symbols are up there too.
    const chord = chordSymbolAt(x, y);
    if (chord) {
      selectChordSymbol(chord);
      return;
    }

    // The swells run under the staff with the dynamics. Tested after them
    // because a hairpin is wide and a dynamic sitting on one of its ends
    // would otherwise be unreachable.
    const hair = hairpinAt(x, y);
    if (hair) {
      selectHairpin(hair);
      return;
    }

    // The beams of a two-note tremolo, which stand between the notes rather
    // than on either of them. Before the notes, because that is where they
    // are drawn: over the heads of the pair they join.
    const trem = tremoloAt(x, y);
    if (trem) {
      selectTremolo(trem);
      return;
    }

    // The figure over a group. Before the notes, since over a beamed group
    // it sits just above them.
    const tup = tupletAt(x, y);
    if (tup) {
      selectTuplet(tup);
      return;
    }

    const hit = hitAt(x, y);

    // The ties and the slurs arc over the music, so they are looked for
    // before the staff they are drawn above — but never in front of a
    // notehead. A curve ends on the two notes it joins and passes over the
    // ones between, and every one of those clicks belongs to its note.
    if (!hit || !elementAt(hit, x, y)) {
      const arc = arcAt(x, y);
      if (arc) {
        selectArc(arc);
        return;
      }
    }
    if (!hit) {
      clearSelection();
      status("Ready");
      return;
    }

    const step = stepAt(hit, y);
    // A click picks music up, unless writing with the mouse has been turned
    // on — and then shift picks up, the way it always did.
    const picking = !state.mouseWrites || e.shiftKey;
    const el = picking ? elementAt(hit, x, y) : columnAt(hit, x);

    // A click landing on something already there picks it up — writing a note
    // exactly on top of one is never what was meant. But with a note tool
    // held, a click in the same column at a height nothing occupies stacks a
    // second pitch on what is there: that is how a chord is written. gscore
    // asked for Ctrl to be held for this; the column does the asking now.
    if (el) {
      // Shift with something already picked up means "and everything from
      // there to here": the usual way of selecting a passage, and what makes
      // it possible to put one crescendo over eight bars.
      if (e.shiftKey && state.anchor) {
        selectRange(state.anchor, { part: hit.part, measure: hit.measure, index: el.index });
        return;
      }
      // On the notehead, or above it.
      //
      // Clicking a note's column at a height the note does not occupy adds
      // that pitch to it, which is how a chord is written. Whether you are
      // on the note was an exact match on the step the click rounds to —
      // and a step is half a space, so the part of a notehead that counted
      // as the notehead was the middle quarter of it. Everywhere else on
      // the same notehead built a chord instead of picking the note up,
      // which is why clicking a note sitting on a staff line so often gave
      // you two notes.
      //
      // A notehead is a space tall, so anywhere within one step of its
      // centre is on it. Further out than that is the gesture that means a
      // chord, and it still does.
      const onNote = (el.steps || []).some((st) => Math.abs(st - step) <= 1);
      const stackable =
        !picking && !state.tool.rest &&
        (el.kind === "note" || el.kind === "chord") && !onNote;
      if (stackable) {
        await addPitch(hit, el, step);
        return;
      }
      select(hit, el, step);
      return;
    }

    state.multi = [];
    // Where in the bar the click fell, and whether there is already
    // something sounding there.
    //
    // A bar that is full has no empty part: the white space to the right of
    // a whole-bar rest is the four beats that rest lasts, and clicking it
    // means "write over that", not "add after it". Appending there found the
    // bar full and put the note in the next bar instead, which is how
    // clicking one bar wrote in the one beside it.
    const idx = insertIndex(hit, x);
    const over = hit.full && (hit.elements || []).length > 0;
    state.selection = {
      part: hit.part, measure: hit.measure,
      // A click left of everything in a full bar is still a click on the
      // first thing in it, so there is always something to write over.
      at: over ? Math.max(1, idx) : idx,
      step: step, ref: null,
      replace: over,
    };

    // Picking up never writes. What is left is a cursor in the bar, which
    // is where the keyboard will write.
    if (picking || state.readOnly) {
      markSelection();
      status(`Measure ${barNumberOf(hit.measure)}`);
      return;
    }
    await insertAtSelection(state.tool.rest, false);
  });
}

// extendSelection reaches one note further along the music, the way shift
// with an arrow reaches one character further in a line of text.
//
// Selecting a long run by clicking the first note and shift-clicking the
// last works, and wants the pointer on both of them; a run of sixty-fourths
// climbing above the staff gives you two targets half a space wide at either
// end of it. From the keyboard there is nothing to aim at.
function extendSelection(dir) {
  const sel = state.selection;
  if (!sel) {
    status("Pick a note first");
    return;
  }
  // The end that stays put. Set on a plain click, so shift with an arrow
  // after clicking a note grows the selection away from that note.
  if (!state.anchor) {
    state.anchor = {
      part: sel.part, measure: sel.measure,
      index: Math.max(0, (sel.at || 1) - 1),
    };
  }
  const from = state.focus || state.anchor;
  const next = elementAlong(from, dir);
  if (!next) {
    status(dir > 0 ? "That is the end of the music" : "That is the beginning");
    return;
  }
  state.focus = next;
  selectRange(state.anchor, next);
  // selectRange points the caret at the start of the range; the end that is
  // moving is the one to keep typing from.
  const hit = state.hits.find(
    (h) => h.part === next.part && h.measure === next.measure);
  const el = hit && (hit.elements || []).find((e) => e.index === next.index);
  if (el) {
    state.selection = {
      part: next.part, measure: next.measure, at: next.index + 1,
      step: (el.steps && el.steps.length) ? el.steps[0] : 0, ref: el.ref,
    };
  }
}

// slideSelection moves the selection itself along the music, keeping its size.
//
// The left and right arrows with a passage in hand. Shift with them grows the
// selection by a note; plain, they carry the whole of it one note further on,
// so a run of four stays a run of four. The notes are not touched — what moves
// is which notes are picked up.
//
// Both ends move together, so the far end runs off the end of the music
// before the near end does: when it will not fit the selection stays where it
// is rather than quietly getting shorter.
function slideSelection(dir) {
  const run = orderedSelection();
  if (!run.length) return;

  const moved = [];
  for (const m of run) {
    const next = elementAlong(m, dir);
    if (!next) {
      status(dir > 0 ? "That is the end of the music" : "That is the beginning");
      return;
    }
    const hit = state.hits.find(
      (h) => h.part === next.part && h.measure === next.measure);
    const el = hit && (hit.elements || []).find((e) => e.index === next.index);
    if (!el) return;
    moved.push({ part: next.part, measure: next.measure, index: next.index, ref: el.ref });
  }

  state.multi = moved;
  const f = moved[0];
  const l = moved[moved.length - 1];
  state.selection = {
    part: f.part, measure: f.measure, at: f.index + 1, step: 0, ref: f.ref,
    replace: true,
  };
  // The pinned end and the moving end travel with it, so shift with an arrow
  // after sliding still grows the selection from the end it was growing from.
  state.anchor = { part: f.part, measure: f.measure, index: f.index };
  state.focus = { part: l.part, measure: l.measure, index: l.index };
  markSelection();
  status(moved.length === 1 ? "1 selected" : `${moved.length} selected`);
}

// elementAlong is the element one step either side of this one, crossing
// barlines as it goes: the music is a line, not a series of boxes.
function elementAlong(at, dir) {
  const bars = state.hits
    .filter((h) => h.part === at.part)
    .sort((a, b) => a.measure - b.measure);
  const here = bars.findIndex((h) => h.measure === at.measure);
  if (here < 0) return null;

  let i = here;
  let index = at.index + dir;
  while (i >= 0 && i < bars.length) {
    const els = bars[i].elements || [];
    if (index >= 0 && index < els.length) {
      return { part: at.part, measure: bars[i].measure, index };
    }
    i += dir;
    if (i < 0 || i >= bars.length) return null;
    const next = bars[i].elements || [];
    index = dir > 0 ? 0 : next.length - 1;
    if (!next.length) continue;
    return { part: at.part, measure: bars[i].measure, index };
  }
  return null;
}

// selectRange picks up everything between two points, in reading order.
//
// A passage is a range, not a list of clicks: from this note to that one,
// taking in the bars between. It stays on one staff, because a selection
// across staves is a different thing and a crescendo over it would mean
// something else again.
function selectRange(from, to) {
  const part = from.part;
  let a = from, b = to;
  if (b.measure < a.measure || (b.measure === a.measure && b.index < a.index)) {
    [a, b] = [b, a];
  }

  state.multi = [];
  for (const h of state.hits) {
    if (h.part !== part || h.measure < a.measure || h.measure > b.measure) continue;
    for (const el of h.elements || []) {
      if (h.measure === a.measure && el.index < a.index) continue;
      if (h.measure === b.measure && el.index > b.index) continue;
      state.multi.push({ part, measure: h.measure, index: el.index, ref: el.ref });
    }
  }
  state.multi = orderedSelection();
  const first = state.multi[0];
  if (first) {
    state.selection = {
      part, measure: first.measure, at: first.index + 1, step: 0, ref: first.ref,
    };
  }
  // Which end is pinned and which end moves, kept as they were asked for
  // rather than as they sort. Shift with an arrow carries on from the end
  // the selection was dragged to; without this it started again from the
  // first note, and the first shift-arrow after a selection shrank it to
  // two notes instead of growing it by one.
  state.anchor = { part, measure: from.measure, index: from.index };
  state.focus = { part, measure: to.measure, index: to.index };
  markSelection();
  status(state.multi.length === 1
    ? "1 selected"
    : `${state.multi.length} selected \u2014 measures ${barNumberOf(a.measure)}\u2013${barNumberOf(b.measure)}`);
}

// selectBudget is the most that can be picked up at once.
//
// Every command that works on a selection names each note in the query it
// sends, so a selection is a few bytes a note on the way to the server.
// Thousands is a working selection; a hundred thousand is a symphony, and
// nobody applies a staccato to a symphony.
const selectBudget = 5000;

// selectSpan picks up everything on one staff between two bars.
//
// One staff, because that is what a selection is here: a slur or a
// crescendo over notes on three staves at once means nothing anybody can
// draw, and the commands that take a selection are mostly those. Picking
// up a whole piece is not needed for the ones that do span staves —
// transposing with nothing picked up moves all of it.
function selectSpan(part, firstBar, lastBar, what) {
  const bars = state.hits.filter((h) => h.part === part);
  if (!bars.length) {
    status("There is nothing on that staff");
    return false;
  }
  const picked = [];
  for (const h of bars) {
    if (h.measure < firstBar || h.measure > lastBar) continue;
    for (const el of h.elements || []) {
      picked.push({ part, measure: h.measure, index: el.index, ref: el.ref });
    }
  }
  if (!picked.length) {
    status("There is nothing written in those bars");
    return false;
  }
  if (picked.length > selectBudget) {
    status(`That is ${picked.length} things; ${selectBudget} is the most ` +
      `that can be picked up at once`);
    return false;
  }

  state.multi = picked;
  state.multi = orderedSelection();
  const first = state.multi[0];
  const last = state.multi[state.multi.length - 1];
  state.selection = {
    part, measure: first.measure, at: first.index + 1, step: 0, ref: first.ref,
  };
  state.anchor = { part, measure: first.measure, index: first.index };
  state.focus = { part, measure: last.measure, index: last.index };
  markSelection();
  scrollSelectionIntoView();
  status(`${picked.length} picked up — ${what}`);
  return true;
}

// selectAll picks up the whole of the staff the cursor is on.
function selectAll() {
  const part = state.selection?.part ?? 0;
  const last = state.hits.reduce(
    (n, h) => (h.part === part ? Math.max(n, h.measure) : n), 0);
  const name = (state.info?.parts || [])[part]?.name;
  selectSpan(part, 0, last, name ? `every bar of the ${name}` : "every bar of the staff");
}

// selectRegionDialog asks for a stretch of bars by number.
//
// Dragging is the other way, and on a long piece it is the wrong way: to
// pick up bars 40 to 96 you have to find both ends and hold the mouse down
// over everything in between. Saying so in numbers is two fields.
async function selectRegionDialog() {
  const dlg = $("#region");
  const parts = state.info?.parts || [];
  const bars = state.hits.reduce((n, h) => Math.max(n, h.measure + 1), 0);

  const staff = $("#region-staff");
  staff.innerHTML = parts.map((p, i) =>
    `<option value="${i}">${escapeHTML(p.name || `Staff ${i + 1}`)}</option>`).join("");
  staff.value = String(state.selection?.part ?? 0);
  // Opening on what is already picked up, so the dialog is a way of
  // adjusting a selection as well as of making one.
  const from = $("#region-from"), to = $("#region-to");
  from.max = to.max = String(Math.max(1, bars));
  from.value = String(barNumberOf(state.selection?.measure ?? 0));
  to.value = String(Math.max(Number(from.value), barNumberOf(bars - 1)));

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    from.focus();
    from.select();
  });
  if (answer !== "ok") {
    status("Ready");
    return;
  }

  // Back from the numbers on the page to the bars behind them. Within the
  // movement the cursor is in, which is what the numbers over the bars say
  // and what Go to bar reads them as.
  const part = Number(staff.value) || 0;
  const where = movementHere();
  let a = barIndexOf(Number(from.value), where);
  let b = barIndexOf(Number(to.value), where);
  if (b < a) [a, b] = [b, a];
  selectSpan(part, a, b, `bars ${barNumberOf(a)}–${barNumberOf(b)}`);
}

// selectDynamic picks up a dynamic marking.
//
// What is selected is the marking, not the note under it — so Backspace
// takes the ff away and leaves the music alone, which is what somebody who
// clicked the ff meant by pressing it.
function selectDynamic(d) {
  state.multi = [];
  state.anchor = state.focus = null;
  state.selection = {
    part: d.part, measure: d.measure, ref: d.ref,
    at: 0, step: 0, dynamic: d.name,
  };
  markSelection();
  status(`${d.name} \u2014 press Backspace to take it away`);
}

// selectChordSymbol picks up a chord written over the music.
//
// What is selected is the symbol, not the note under it: Backspace takes the
// chord away and leaves the music alone, and Enter opens it for changing
// with what is there already in the box. Which is what somebody who clicked
// a chord meant by either of them.
function selectChordSymbol(c) {
  state.multi = [];
  state.anchor = state.focus = null;
  // Which element in the bar holds it, because a bar can hold more than
  // one: "F7 Bb7" over two halves of a bar are two symbols, and changing
  // the second must not rewrite the first.
  let at = 1;
  const target = refTarget(c.ref);
  if (target) at = Number(target.split(":")[2]) + 1;

  state.selection = {
    part: c.part, measure: c.measure, ref: c.ref,
    at, step: 0, chordSymbol: c.name || c.frets || "chord",
  };
  markSelection();
  status(`${state.selection.chordSymbol} \u2014 press Enter to change it, ` +
         `Backspace to take it away`);
}

// selectArticulation picks up a mark written on a note.
//
// What is selected is the mark and not the note: Backspace takes the dot
// away and leaves the music, and the status line says which mark it is —
// which is also the answer to "what is this one called".
function selectArticulation(a) {
  state.multi = [];
  state.anchor = state.focus = null;
  let at = 1;
  const target = refTarget(a.ref);
  if (target) at = Number(target.split(":")[2]) + 1;

  state.selection = {
    part: a.part, measure: a.measure, ref: a.ref, at, step: 0, articulation: a,
  };
  markSelection();
  status(`${t(articulationLabel(a.mark))} \u2014 ` +
         `${a.above ? "over" : "under"} the note. Press Backspace to take ` +
         `it away, or the palette to move it.`);
}

// articulationLabel is what a mark is called in front of somebody, from the
// palette that writes it: one list of names, not two.
function articulationLabel(name) {
  for (const section of (typeof PALETTE === "undefined" ? [] : PALETTE)) {
    for (const item of section.items || []) {
      if (item.apply === "articulation" && item.value === name) return item.label;
    }
  }
  return name;
}

// selectMarker picks up a rehearsal mark.
//
// What is selected is the mark, so Backspace takes the letter away and
// leaves the bar, and Enter opens the window that names it.
function selectMarker(m) {
  state.multi = [];
  state.anchor = state.focus = null;
  state.selection = {
    part: 0, measure: m.measure, ref: null, at: 0, step: 0, marker: m,
  };
  markSelection();
  status(`Mark ${m.text} at bar ${barNumberOf(m.measure)} — press Enter to ` +
         `change it, Backspace to take it away`);
}

// selectTextMark picks up the words written on a note.
//
// What is selected is the words and not the note under them: Backspace takes
// them away and leaves the music, and Enter opens them for changing with
// what they say already in the box.
function selectTextMark(t) {
  state.multi = [];
  state.anchor = state.focus = null;
  let at = 1;
  const target = refTarget(t.ref);
  if (target) at = Number(target.split(":")[2]) + 1;

  state.selection = {
    part: t.part, measure: t.measure, ref: t.ref,
    at, step: 0, textMark: t,
  };
  markSelection();
  status(`\u201c${t.text}\u201d \u2014 press Enter to change it, ` +
         `Backspace to take it away`);
}

// selectTempo picks up a tempo marking.
//
// What is selected is the marking, so Backspace takes the change away and
// the bar carries on at whatever speed it had before it. The marking at bar
// one is the piece's own speed and is not a change: a piece has a speed
// whether or not anybody chose it, so that one can be selected and looked at
// but not removed.
function selectTempo(t) {
  state.multi = [];
  state.anchor = state.focus = null;
  state.selection = {
    part: 0, measure: t.measure, ref: null,
    at: 0, step: 0, tempo: t.measure,
  };
  markSelection();
  status(t.measure === 0
    ? `${t.label} — the speed the piece starts at`
    : `${t.label} at bar ${barNumberOf(t.measure)} — press Backspace to take it away`);
}

// selectHairpin picks up a swell.
function selectHairpin(h) {
  state.multi = [];
  state.anchor = state.focus = null;
  state.selection = {
    part: h.part, measure: h.measure, ref: h.from,
    at: 0, step: 0, hairpin: h,
  };
  markSelection();
  status(`${h.kind} \u2014 press Backspace to take it away`);
}

// selectArc picks up a tie or a slur.
//
// What is picked up is the curve, not the notes under it: Backspace takes
// the curve away and leaves the music alone. A tie on one notehead of a
// chord is one curve of several, and it is that one.
function selectArc(a) {
  state.multi = [];
  state.anchor = state.focus = null;
  state.selection = {
    part: a.part, measure: a.measure, ref: a.from,
    at: 0, step: 0, arc: a,
  };
  markSelection();
  status(`${a.kind === "tie" ? "Tie" : "Slur"} \u2014 press Backspace to take it away`);
}

// selectTremolo picks up the mark between two notes.
//
// What is picked up is the first of the two, because that is the note the
// mark is written on and the note every command that touches it names —
// taking it off either end takes off the whole of it. The box, though, goes
// round the beams, because the beams are what was clicked.
function selectTremolo(t) {
  state.multi = [];
  state.anchor = state.focus = null;
  state.selection = {
    part: t.part, measure: t.measure, ref: t.from,
    at: 0, step: 0, tremolo: t,
  };
  markSelection();
  const beams = t.beams === 1 ? "one beam" : `${t.beams} beams`;
  status(`Two-note tremolo, ${beams} \u2014 press Backspace to take it away`);
}

// selectTuplet picks up a group by its figure.
//
// Taking it off leaves the notes where they are: a run of twelve under a 12
// that turns out not to be a tuplet is still twelve notes, and the menu's
// "back to one note" — right for a group made out of one note — would throw
// eleven of them away.
function selectTuplet(t) {
  state.multi = [];
  state.anchor = state.focus = null;
  state.selection = {
    part: t.part, measure: t.measure, ref: t.from,
    at: 0, step: 0, tuplet: t,
  };
  markSelection();
  status(`${t.figure} in the time of the notes under it — ` +
    "press Backspace to take the grouping off and keep the notes");
}

// selectMeasure picks up a whole bar rather than a note in it.
//
// This is what a key or a meter needs: "from this bar on" is a thing you say
// about a bar, and there was no way to say it except by finding a note in the
// bar and selecting that instead.
function selectMeasure(part, measure) {
  state.multi = [];
  state.anchor = state.focus = null;
  // bar says the whole measure is what is picked up, not a place in it.
  // Copying needs to know: a bar has no ref of its own, and without this
  // "copy" found nothing selected and said so, which is the one thing
  // somebody who has just clicked a bar knows to be untrue.
  state.selection = { part, measure, at: 0, step: 0, ref: null, bar: true };
  markSelection();
  const hit = state.hits.find((h) => h.part === part && h.measure === measure);
  const has = [];
  if (hit?.keyChange) has.push("a key change");
  if (hit?.timeChange) has.push("a meter change");
  status(`Measure ${barNumberOf(measure)}` +
    (has.length ? ` — ${has.join(" and ")} here` : "") +
    " · Score ▸ Key or Time applies from here");
}

// select picks up an existing element, so the next command acts on it.
function select(hit, el, step) {
  // This note is both ends of the selection until an arrow or a shift-click
  // moves one of them.
  state.anchor = { part: hit.part, measure: hit.measure, index: el.index };
  state.focus = null;
  state.multi = [];
  // Which notehead of a chord was clicked, so a tie or an accidental can be
  // put on that one rather than on all of them. Null means the element as a
  // whole, which is what a dragged selection always picks up.
  //
  // The nearest head within a step, which is the same reach that decides
  // whether the click was on a notehead or in the space above one. An exact
  // match asked for the middle quarter of a notehead: everywhere else on the
  // same head picked up the chord and then could not say which note of it
  // had been pointed at.
  let head = null;
  if (step !== undefined) {
    for (const st of el.steps || []) {
      if (Math.abs(st - step) > 1) continue;
      if (head === null || Math.abs(st - step) < Math.abs(head - step)) head = st;
    }
  }
  state.selection = {
    part: hit.part,
    measure: hit.measure,
    // The marking commands act on the element before the cursor, so the
    // cursor sits just after what is selected.
    at: el.index + 1,
    step: (el.steps && el.steps.length) ? el.steps[0] : 0,
    head,
    ref: el.ref,
    // Pointed at, so typing changes it rather than adding to it. Writing at
    // the cursor still writes; this is only ever set by picking something up.
    replace: true,
  };
  markSelection();
  updateFingering();
  // Naming the notehead, because what the next accidental changes is one
  // note of the chord and the box round that head is easy to read as a
  // box round a small chord.
  const what = el.kind === "chord" && head !== null && (el.steps || []).length > 1
    ? "one notehead of chord" : el.kind;
  status(`Selected ${what} ${el.index + 1} of measure ${barNumberOf(hit.measure)}`);
}

function clearSelection() {
  state.selection = null;
  state.multi = [];
  state.anchor = state.focus = null;
  markSelection();
}


/* ---------- the paper ---------- */

/*
 * Page setup.
 *
 * Not only how the score comes out of the printer. The paper decides how
 * wide a system may be, and the engraver has to be told that before it
 * lays anything out — so a score set up for A3 on its side is a different
 * engraving from the same music on A4 upright, with different bars on
 * different lines. That is why this is kept with the score and not asked
 * at every print.
 */

async function pageSetupDialog() {
  const dlg = $("#pagesetup");
  const now = await (await fetch("/api/pagesetup")).json();

  const papers = $("#ps-paper");
  papers.innerHTML = now.papers
    .map((p) => `<option value="${escapeHTML(p.name)}">${escapeHTML(p.label)}</option>`)
    .join("");
  papers.value = now.paper;
  $$('#pagesetup-form input[name="ps-orient"]').forEach((r) => {
    r.checked = (r.value === "landscape") === !!now.landscape;
  });
  $("#ps-width").value = String(Math.round(now.width));
  $("#ps-height").value = String(Math.round(now.height));
  $("#ps-top").value = String(now.marginTop);
  $("#ps-bottom").value = String(now.marginBottom);
  $("#ps-left").value = String(now.marginLeft);
  $("#ps-right").value = String(now.marginRight);
  // A size the list does not offer — set by a script, or by a later
  // version — is added to it rather than silently becoming something else.
  const space = $("#ps-space");
  if (![...space.options].some((o) => Number(o.value) === Number(now.space))) {
    space.add(new Option(now.space + " pt", String(now.space)));
  }
  space.value = String(now.space);

  // What is left to print on, worked out as the fields are typed in. The
  // number that matters is the width: it is what a system has to fit into,
  // and margins that leave forty millimetres are margins nobody meant.
  const showRoom = () => {
    const custom = papers.value === "custom";
    $("#ps-custom").hidden = !custom;
    let w, h;
    if (custom) {
      w = Number($("#ps-width").value) || 0;
      h = Number($("#ps-height").value) || 0;
    } else {
      const p = now.papers.find((x) => x.name === papers.value);
      w = p ? p.width : 0;
      h = p ? p.height : 0;
    }
    const sideways = $$('#pagesetup-form input[name="ps-orient"]')
      .find((r) => r.checked)?.value === "landscape";
    if (sideways && w < h) [w, h] = [h, w];
    const across = w - (Number($("#ps-left").value) || 0) -
      (Number($("#ps-right").value) || 0);
    const down = h - (Number($("#ps-top").value) || 0) -
      (Number($("#ps-bottom").value) || 0);
    $("#ps-room").textContent = across > 0 && down > 0
      ? `${Math.round(across)} × ${Math.round(down)} mm to print on.`
      : "Those margins leave no room for any music.";
  };
  papers.oninput = showRoom;
  $$("#pagesetup-form input").forEach((i) => { i.oninput = showRoom; });
  showRoom();

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  if (answer === "cancel" || answer === "") {
    status("Ready");
    return;
  }
  if (answer === "reset") {
    const res = await fetch("/api/pagesetup?reset=1", { method: "POST" });
    if (!res.ok) {
      status("Could not change the paper: " + (await res.text()).trim());
      return;
    }
    await refreshScore();
    status("Back to the default paper");
    return;
  }

  const p = new URLSearchParams({
    paper: papers.value,
    landscape: $$('#pagesetup-form input[name="ps-orient"]')
      .find((r) => r.checked)?.value === "landscape" ? "1" : "0",
    marginTop: $("#ps-top").value,
    marginBottom: $("#ps-bottom").value,
    marginLeft: $("#ps-left").value,
    marginRight: $("#ps-right").value,
    space: $("#ps-space").value,
  });
  if (papers.value === "custom") {
    p.set("width", $("#ps-width").value);
    p.set("height", $("#ps-height").value);
  }
  const res = await fetch("/api/pagesetup?" + p, { method: "POST" });
  if (!res.ok) {
    status("Could not change the paper: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  await refreshScore();
  status(`${Math.round(d.width)} × ${Math.round(d.height)} mm`);
}

/* ---------- playing the music in ---------- */

/*
 * A MIDI keyboard, writing where the cursor is.
 *
 * This is how most people put music into a notation program, and gscore
 * had no way to do it at all: every note had to be clicked or typed.
 *
 * Step time rather than real time. A key pressed writes one note of
 * whatever length the toolbar is holding, and the cursor moves on; the
 * rhythm comes from the toolbar and not from how long the key was held.
 * Real time — playing in tempo and having the rhythm worked out — is a
 * different and much harder thing, and it is not what somebody entering a
 * part wants: they want the notes right.
 *
 * Keys pressed together are a chord. Together means within a short window,
 * because no two fingers land at the same millisecond: the first key
 * writes a note and the ones that follow it closely are added to that
 * note as noteheads. A hand playing a triad gets a triad, and a hand
 * playing a scale gets a scale, with no mode to switch between them.
 *
 * What arrives is a sounding pitch and nothing else — the same number for
 * D sharp and E flat — so the spelling is left to the server, where the
 * key signature is. See PitchInKey.
 */

const midiIn = {
  // access is the MIDIAccess object once it has been granted, and inputs
  // the ports it offered.
  access: null,
  inputs: [],
  // port is the one being listened to, or null for none. Kept by id, not
  // by object, so a device unplugged and plugged back in is recognised.
  portId: null,
  // chord is the note being built: where it was written, and when, so a
  // key pressed soon after is added to it rather than written after it.
  chord: null,
  // Milliseconds within which two keys count as played together. Long
  // enough for a hand, short enough that two deliberate notes are two.
  window: 60,
  // busy guards against a second note arriving while the first is still
  // being written: the server decides where the cursor goes next, and two
  // writes racing would both aim at where it used to be.
  busy: false,
  queue: [],
};

// midiSupported says whether this build can listen at all.
//
// Web MIDI is in Chrome, Edge and Opera, and not in Safari — which is what
// the macOS application is built on. So the menu entry says why rather
// than failing silently when it is clicked.
function midiSupported() {
  return typeof navigator !== "undefined" &&
    typeof navigator.requestMIDIAccess === "function";
}

// openMidi asks for the keyboards and remembers what it was given.
async function openMidi() {
  if (midiIn.access) return midiIn.access;
  if (!midiSupported()) return null;
  // sysex is not asked for: nothing here needs it, and asking raises the
  // permission prompt from "use your MIDI devices" to one most people say
  // no to.
  const access = await navigator.requestMIDIAccess({ sysex: false });
  midiIn.access = access;
  readMidiPorts();
  // A keyboard switched on after the program was started is still a
  // keyboard: the list is rebuilt rather than being read once.
  access.onstatechange = () => {
    readMidiPorts();
    if (midiIn.portId && !midiIn.inputs.some((i) => i.id === midiIn.portId)) {
      status("The MIDI keyboard was unplugged");
      listenToMidi(null);
    }
  };
  return access;
}

function readMidiPorts() {
  midiIn.inputs = midiIn.access ? [...midiIn.access.inputs.values()] : [];
}

// listenToMidi starts listening to one port and stops listening to the
// rest. Exactly one at a time, which is what a person has hands for.
function listenToMidi(id) {
  for (const port of midiIn.inputs) port.onmidimessage = null;
  midiIn.portId = id;
  midiIn.chord = null;
  if (!id) return;
  const port = midiIn.inputs.find((p) => p.id === id);
  if (port) port.onmidimessage = onMidiMessage;
}

// onMidiMessage picks the note-ons out of what the keyboard sends.
function onMidiMessage(e) {
  const [a, note, velocity] = e.data;
  // The bottom nibble is the channel, which is ignored: a keyboard on
  // channel 3 is still the person playing.
  const kind = a & 0xf0;
  // A note-on with no velocity is how a great many keyboards send
  // note-off, and taking it for a key press writes every note twice.
  if (kind !== 0x90 || !velocity) return;
  playedNote(note);
}

// playedNote writes one key press, or adds it to the note being built.
//
// Queued rather than written at once. Each write asks the server where the
// cursor ends up, and two of them in flight would both be working from
// where it was before either had landed — which on a chord played with
// five fingers is five notes written one after another in the wrong bar.
function playedNote(note) {
  midiIn.queue.push({ note, at: Date.now() });
  drainMidi();
}

async function drainMidi() {
  if (midiIn.busy) return;
  midiIn.busy = true;
  try {
    while (midiIn.queue.length) {
      const { note, at } = midiIn.queue.shift();
      await writePlayedNote(note, at);
    }
  } finally {
    midiIn.busy = false;
  }
}

async function writePlayedNote(note, at) {
  const sel = state.selection;
  if (!sel) {
    status("Click where the music should go first");
    return;
  }
  // Part of the note already being written, if it arrived soon enough and
  // the cursor has not been moved in between.
  const together = midiIn.chord &&
    at - midiIn.chord.at < midiIn.window &&
    midiIn.chord.part === sel.part && midiIn.chord.measure === sel.measure;

  if (together) {
    const p = new URLSearchParams({
      part: String(midiIn.chord.part), measure: String(midiIn.chord.measure),
      index: String(midiIn.chord.index), midi: String(note),
    });
    const res = await fetch("/api/addpitch?" + p, { method: "POST" });
    if (!res.ok) {
      status((await res.text()).trim());
      return;
    }
    soundAsWritten(await res.json().catch(() => null));
    await refreshScore();
    markSelection();
    return;
  }

  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure),
    at: String(sel.at), midi: String(note), dur: state.tool.duration,
    kind: "note",
  });
  if (state.tool.dots > 0) p.set("dots", String(state.tool.dots));
  const res = await fetch("/api/insert?" + p, { method: "POST" });
  if (!res.ok) {
    status("Could not write it: " + (await res.text()).trim());
    return;
  }
  const where = await res.json().catch(() => null);
  soundAsWritten(where);
  if (where && where.measure !== undefined) {
    sel.measure = where.measure;
    sel.at = where.at;
  } else {
    sel.at += 1;
  }
  sel.replace = false;
  sel.ref = null;
  sel.wrote = true;
  state.multi = [];
  state.anchor = null;
  state.focus = null;
  // Where the note landed, so the keys pressed with it are added to it
  // rather than written after it. at is already past the note, so the
  // note itself is the one before.
  midiIn.chord = {
    at, part: sel.part, measure: sel.measure, index: sel.at - 1,
  };
  await refreshScore();
  carryCaretOn(sel);
  markSelection();
  scrollSelectionIntoView();
}

// midiDialog offers the keyboards that are plugged in.
async function midiDialog() {
  if (!midiSupported()) {
    status("This build cannot reach a MIDI keyboard — " +
           "open gscore in Chrome, Edge or Opera to play one in");
    return;
  }
  let access = null;
  try {
    access = await openMidi();
  } catch (e) {
    status("Could not reach the MIDI keyboards: " + (e.message || e));
    return;
  }
  if (!access) return;
  readMidiPorts();

  if (!midiIn.inputs.length) {
    status("No MIDI keyboard is plugged in");
    return;
  }
  const options = [{ value: "", label: "none — do not listen" }].concat(
    midiIn.inputs.map((i) => ({
      value: i.id,
      label: i.name + (i.manufacturer ? " — " + i.manufacturer : ""),
    })));
  const chosen = await ask({
    title: "MIDI keyboard",
    note: "A key pressed writes a note where the cursor is, with the " +
          "length the toolbar is holding. Keys pressed together are a chord.",
    label: "Listen to",
    options,
    value: midiIn.portId || "",
  });
  if (chosen === null) return;
  listenToMidi(chosen || null);
  const port = midiIn.inputs.find((p) => p.id === chosen);
  status(port ? "Listening to " + port.name : "Not listening to any keyboard");
}

// midiTick is the mark beside the menu entry, so the menu says whether
// anything is being listened to.
function midiTick() {
  const el = $("#midi-tick");
  if (el) el.textContent = midiIn.portId ? "\u2713" : "";
}

async function insertAtSelection(rest, keepSelection) {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  // What was picked up is written over; what the cursor is pointing between
  // is written into. A range of notes is neither: it is not one thing to
  // replace, so it falls back to writing.
  const replacing = !!sel.replace && state.multi.length <= 1 && sel.at > 0;
  const p = new URLSearchParams({
    part: String(sel.part),
    measure: String(sel.measure),
    at: String(replacing ? sel.at - 1 : sel.at),
    step: String(sel.step),
    dur: state.tool.duration,
    kind: rest ? "rest" : "note",
  });
  if (state.tool.dots > 0) p.set("dots", String(state.tool.dots));
  if (replacing) p.set("replace", "1");
  if (!rest && state.tool.accidental !== "none") {
    p.set("accidental", state.tool.accidental);
  }
  // On a drum staff, which drum rather than where the click landed. A
  // drummer picks the snare and then says where in the bar; making them
  // remember which line it is on and hit it is the wrong way round.
  if (!rest && state.tool.drum !== null && isDrumStaff(sel.part)) {
    p.set("drum", String(state.tool.drum));
  }
  const res = await fetch("/api/insert?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not insert: " + (await res.text()).trim());
    return;
  }
  // The note is written, so the accidental that was held has been used up.
  // Not after a rest, which never carried it.
  if (!rest) useAccidental();
  // A bar that is already full sends what is written next into the bar after
  // it, making one if there is none, so the answer says where the note ended
  // up rather than where it was aimed. The cursor follows it there, which is
  // what makes writing bar after bar a matter of carrying on clicking.
  const where = await res.json().catch(() => null);
  // Heard as it is written, before the score is drawn again: the drawing
  // is the slow part and the point of this is that it answers the moment
  // the key is pressed.
  soundAsWritten(where);
  const moved = where && where.measure !== undefined && where.measure !== sel.measure;
  if (where && where.measure !== undefined) {
    // where.at is already the place after the element, whether it was
    // written in or written over.
    sel.measure = where.measure;
    sel.at = where.at;
  } else {
    sel.at += 1;
  }
  // One correction at a time: having written over what was picked up, the
  // cursor is past it and the next thing typed is written, not another
  // replacement.
  sel.replace = false;
  sel.ref = null;
  // And nothing is picked up any more. Writing is not a selecting
  // gesture: whatever was held before it — a range dragged out, a note
  // pointed at — is behind you now.
  //
  // It was left standing, and heldForMoving asks about a range before it
  // asks whether the note was merely written, so the up and down arrows
  // went on moving whatever had been picked up earlier instead of aiming
  // the next note. The arrow after writing a note is somebody saying
  // where the next one goes.
  state.multi = [];
  state.anchor = null;
  state.focus = null;
  await refreshScore();
  // Select what was just written, so a dynamic or a stem change lands on it.
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  const el = hit && (hit.elements || [])[sel.at - 1];
  if (el) sel.ref = el.ref;
  // Selected, but not picked up: see heldForMoving. What was just written
  // is what a dynamic or a stem change should land on, and it is not what
  // an arrow key should move — the gesture after writing a note is to go
  // up or down and write the next one.
  sel.wrote = true;
  // A bar with no room left in it is no place for the caret.
  //
  // The next note written there goes into the bar after anyway — the server
  // sends it on — so a caret sitting past the last note of a full bar is
  // pointing at somewhere nothing can be written, hard against the barline.
  // It belongs at the start of the next bar, which is where the writing is
  // about to carry on.
  //
  // What was just written stays selected, so a dynamic or a stem change
  // still lands on it: the caret says where the next note goes and the
  // selection says what the last one was, and after a bar fills those are
  // two different places.
  carryCaretOn(sel);
  markSelection();
  // Writing is the one thing that moves the caret a note at a time, and the
  // only thing that moves it without anybody having asked to go anywhere.
  scrollSelectionIntoView();
  let what = rest ? "Rest" : state.tool.duration + " note";
  if (replacing) {
    what = "Replaced with a " + (rest ? "rest" : state.tool.duration + " note");
  }
  // On a drum staff, say which drum. "quarter note" tells a drummer nothing
  // about what they just wrote.
  if (!rest && isDrumStaff(sel.part)) {
    const name = drumName(state.tool.drum, sel.step);
    if (name) what = name + ", " + state.tool.duration;
  }
  status(moved ? `${what} — measure ${barNumberOf(sel.measure)}` : what + " inserted");
  void keepSelection;
}

// addPitch stacks another notehead on a note or chord already written.
async function addPitch(hit, el, step) {
  const p = new URLSearchParams({
    part: String(hit.part),
    measure: String(hit.measure),
    index: String(el.index),
    step: String(step),
  });
  if (state.tool.accidental !== "none") {
    p.set("accidental", state.tool.accidental);
  }
  const res = await fetch("/api/addpitch?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  soundAsWritten(await res.json().catch(() => null));
  // The notehead carries it now, so the pen does not.
  useAccidental();
  state.multi = [];
  state.selection = {
    part: hit.part, measure: hit.measure, at: el.index + 1, step: step, ref: null,
  };
  await refreshScore();
  const h = state.hits.find((x) => x.part === hit.part && x.measure === hit.measure);
  const now = h && (h.elements || [])[el.index];
  if (now) state.selection.ref = now.ref;
  markSelection();
  status("Added to the chord");
}

async function deleteSelected() {
  // A symbol was picked up by clicking it. It is picked the way a
  // hairpin is and is not part of the note selection, so it has to be
  // asked about here — and it comes first, because a symbol sitting over
  // a note is what the click landed on and is what somebody pressing
  // delete means.
  if (pickedSymbol) {
    const p = new URLSearchParams({
      part: String(pickedSymbol.part), measure: String(pickedSymbol.measure),
      at: String(pickedSymbol.index + 1), name: pickedSymbol.name,
    });
    const res = await fetch("/api/removesymbol?" + p, { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    const gone = pickedSymbol.name;
    pickedSymbol = null;
    await refreshScore();
    status("Removed " + gone);
    return;
  }

  // A swell was picked up by clicking it. Both of its ends go: a hairpin
  // with its start cleared and its end left behind opens and never closes,
  // and is drawn to the end of the system.
  const hair = state.selection?.hairpin;
  if (hair && !state.multi.length) {
    const ends = [refTarget(hair.from), refTarget(hair.to)].filter(Boolean);
    if (!ends.length) {
      status("Could not find that hairpin any more");
      return;
    }
    const res = await fetch(
      "/api/hairpin?hairpin=none&sel=" + encodeURIComponent(ends.join(",")),
      { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    state.selection.hairpin = null;
    await refreshScore();
    status(`${hair.kind} removed`);
    return;
  }
  // A tie or a slur was picked up by clicking it. Both of its ends go, and
  // only its own ends: a note in the middle of a phrase closes one slur and
  // opens the next, and a note held through three bars stops one tie and
  // starts another on the same notehead.
  const arc = state.selection?.arc;
  if (arc && !state.multi.length) {
    const ends = [refTarget(arc.from), refTarget(arc.to)].filter(Boolean);
    if (!ends.length) {
      status(`Could not find that ${arc.kind} any more`);
      return;
    }
    const p = new URLSearchParams({
      ends: "1", sel: ends.join(","),
      [arc.kind]: "none",
    });
    // Which notehead of the chord, for a tie: a chord is tied notehead by
    // notehead and the others keep theirs.
    if (arc.kind === "tie") p.set("step", String(arc.step));
    const res = await fetch(`/api/${arc.kind}?` + p.toString(), { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    state.selection.arc = null;
    await refreshScore();
    status(arc.kind === "tie" ? "Tie removed" : "Slur removed");
    return;
  }

  // A tremolo was picked up by clicking its beams. Both of its notes go
  // back to being notes: half a mark draws nothing and plays as an ordinary
  // note, but it is still in the file for the next thing written beside it
  // to pair with.
  const trem = state.selection?.tremolo;
  if (trem && !state.multi.length) {
    const first = refTarget(trem.from);
    if (!first) {
      status("Could not find that tremolo any more");
      return;
    }
    const res = await fetch(
      "/api/tremolo?strokes=0&sel=" + encodeURIComponent(first),
      { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    state.selection.tremolo = null;
    await refreshScore();
    status("Tremolo removed");
    return;
  }

  // A group was picked up by its figure. The figure goes and the notes
  // stay, each written as it was drawn.
  const tup = state.selection?.tuplet;
  if (tup && !state.multi.length) {
    const where = refTarget(tup.from);
    if (!where) {
      status("Could not find that tuplet any more");
      return;
    }
    const [part, measure, index] = where.split(":");
    const p = new URLSearchParams({
      free: "1", part, measure, at: String(Number(index) + 1),
    });
    const res = await fetch("/api/tuplet?" + p.toString(), { method: "POST" });
    if (!res.ok) {
      status("Could not take it off: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    state.selection.tuplet = null;
    await refreshScore();
    status(`Grouping taken off — ${d.freed} ` +
      (d.freed === 1 ? "note is" : "notes are") + " written as they stand");
    return;
  }

  // A tempo marking was picked up by clicking it, so it is the change that
  // goes and the bar carries on at the speed it had before.
  const atBar = state.selection?.tempo;
  if (atBar !== undefined && atBar !== null && !state.multi.length) {
    if (atBar === 0) {
      status("The piece has a speed whether or not anybody chose it");
      return;
    }
    const res = await fetch("/api/tempo?remove=1&measure=" + atBar,
                            { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    state.selection.tempo = null;
    await refreshScore();
    status(`Bar ${atBar + 1} carries on at the tempo before it`);
    return;
  }
  // A chord symbol was picked up by clicking it, so it is the chord that
  // goes and not the note holding it up.
  if (state.selection?.chordSymbol && !state.multi.length) {
    const was = state.selection.chordSymbol;
    await markChord({ chord: "none" }, `${was} removed`);
    if (state.selection) state.selection.chordSymbol = null;
    return;
  }
  // A mark on a note was picked up by clicking it, so it is the mark that
  // goes and the note stays.
  if (state.selection?.articulation && !state.multi.length) {
    const a = state.selection.articulation;
    const d = await mark("articulation",
      { articulation: a.mark, remove: "1", ...pickedHead() },
      `${t(articulationLabel(a.mark))} removed`);
    if (d && state.selection) state.selection.articulation = null;
    return;
  }
  // A rehearsal mark was picked up by clicking it, so it is the letter
  // that goes and the bar stays. The marks after it move up the sequence
  // by themselves, which is what makes them worth keeping as places.
  if (state.selection?.marker && !state.multi.length) {
    const m = state.selection.marker;
    const res = await fetch(
      "/api/marker?measure=" + m.measure + "&text=", { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    if (state.selection) state.selection.marker = null;
    await refreshScore();
    status(`Mark ${m.text} removed`);
    return;
  }
  // Words were picked up by clicking them, so it is the words that go and
  // not the note holding them up.
  if (state.selection?.textMark && !state.multi.length) {
    const t = state.selection.textMark;
    const p = new URLSearchParams({
      part: String(t.part), measure: String(t.measure),
      at: String(state.selection.at), text: "", index: String(t.index),
    });
    const res = await fetch("/api/text?" + p.toString(), { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    if (state.selection) state.selection.textMark = null;
    await refreshScore();
    status(`\u201c${t.text}\u201d removed`);
    return;
  }
  // A dynamic was picked up by clicking it, so it is the dynamic that goes.
  if (state.selection?.dynamic && !state.multi.length) {
    const was = state.selection.dynamic;
    await setDynamic("none");
    // There is nothing there to have selected any more.
    if (state.selection) state.selection.dynamic = null;
    status(`${was} removed`);
    return;
  }
  if (state.multi.length) {
    const sel = state.multi.map((m) => `${m.part}:${m.measure}:${m.index}`).join(",");
    const res = await fetch("/api/deleteselection?sel=" + encodeURIComponent(sel),
                            { method: "POST" });
    if (!res.ok) {
      status("Could not delete: " + (await res.text()).trim());
      return;
    }
    const n = state.multi.length;
    state.multi = [];
    await refreshScore();
    status(`Deleted ${n}`);
    return;
  }
  const sel = state.selection;

  // A whole bar picked up: everything in it goes and the empty bar stays.
  // A bar has no element of its own to point at, so this used to fall
  // through to "nothing selected to delete" — which is the one thing
  // somebody who has just clicked a bar knows to be untrue.
  if (sel && sel.bar) {
    const p = new URLSearchParams({
      part: String(sel.part), measure: String(sel.measure),
    });
    const res = await fetch("/api/clearmeasure?" + p.toString(),
                            { method: "POST" });
    if (!res.ok) {
      status("Could not empty it: " + (await res.text()).trim());
      return;
    }
    const d = await res.json().catch(() => ({}));
    await refreshScore();
    status(d.removed
      ? `Measure ${barNumberOf(sel.measure)} emptied — ${d.removed} removed`
      : `Measure ${barNumberOf(sel.measure)} was already empty`);
    return;
  }

  if (!sel || sel.at <= 0) {
    status("Nothing selected to delete");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at - 1),
    // The notehead that was clicked, where one was: a note of a chord
    // picked up and deleted is that note, not the chord it is in.
    ...pickedHead(),
  });
  const res = await fetch("/api/delete?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not delete: " + (await res.text()).trim());
    return;
  }
  const d = await res.json().catch(() => ({}));
  // What is left says what happened. A notehead taken out of a chord
  // leaves the chord there and the caret where it was; anything else
  // leaves a gap, and the caret steps back into it.
  if (d && d.left > 0) {
    sel.head = null;
    await refreshScore();
    markSelection();
    status(d.left === 1 ? "Notehead removed — one note left"
                        : `Notehead removed — ${d.left} left`);
    return;
  }
  sel.at -= 1;
  sel.head = null;
  await refreshScore();
  status("Deleted");
}

async function setBarline(style) {
  const sel = state.selection;
  if (!sel) {
    status("Click the measure whose barline you want to change");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), style,
  });
  const res = await fetch("/api/barline?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set the barline: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status("Barline: " + style);
}

// refreshScore redraws and reloads the hit regions, which move whenever the
// music changes.
async function refreshScore() {
  await drawScore();
  await loadHits();
  await loadInfo();
  markSelection();
}

// markSelection draws what is selected.
//
// A selection should look like one thing, not like a handful of recoloured
// noteheads: a rounded rectangle drawn around exactly what is picked up, the
// way every notation program worth using does it. The notes are tinted too,
// but the rectangle is what the eye reads.
function markSelection() {
  rememberCursor();
  // The typing line stands where the cursor is, so it moves when the
  // cursor does.
  placeTypeBar();
  // The drum pads belong to the staff the cursor is on, and the cursor has
  // just moved.
  showDrumPad();
  // The harmonics panel follows what is picked up, so that writing a chord
  // and looking at it are the same gesture.
  if (state.harm?.open && !state.harm.following) void refreshHarmonics();
  const svg = currentSVG();
  if (!svg) return;
  // Everything the last selection drew. Picking something up replaces what
  // was picked up before, so none of this survives into the next one.
  svg.querySelectorAll("rect.selection").forEach((r) => r.remove());
  // The lit tie or slur, which is drawn rather than tinted and so has to be
  // taken away rather than untinted. Left behind, every curve ever clicked
  // stayed lit: clicking a slur, then a tie, then a note showed all three
  // picked up at once.
  svg.querySelectorAll("path.selection-arc").forEach((n) => n.remove());
  svg.querySelectorAll(".selected").forEach((n) => n.classList.remove("selected"));
  // And the writing cursor, which is redrawn below. Appending a new one
  // without taking the old one away left a trail of them down the bar: every
  // place the cursor had been was still on the page.
  svg.querySelectorAll(".write-cursor").forEach((n) => n.remove());

  const sel = state.selection;
  if (!sel) return;
  const z = state.zoom;

  // A dynamic picked up by clicking it: the box goes round the marking
  // itself, under the staff, not round the note it is written on. Seeing a
  // notehead light up after clicking an ff would say the wrong thing about
  // what Backspace is going to take away.
  // A whole bar picked up: the box goes round the bar.
  //
  // Nothing was drawn at all before, which is why the gesture was a guess.
  // A barline has a bar on either side of it, so shift-clicking one and
  // seeing no change left nothing to say which of the two had been picked
  // up — and then Delete emptied whichever it was. Shown, there is nothing
  // to wonder about.
  if (sel.bar) {
    const hit = state.hits.find(
      (h) => h.part === sel.part && h.measure === sel.measure);
    if (hit) {
      let top = hit.staffTop - 1.6;
      let bottom = hit.staffTop + staffHeightOf(hit) + 1.6;
      // Round the music, not only round the staff: a run climbing above the
      // staff is as much a part of the bar as the notes inside it.
      for (const el of hit.elements || []) {
        const a = Math.min(el.y, el.y2 ?? el.y) - 1.4;
        const b = Math.max(el.y, el.y2 ?? el.y) + 1.4;
        if (a < top) top = a;
        if (b > bottom) bottom = b;
      }
      svg.insertBefore(
        selectionRect(hit.x0, top, hit.x1 - hit.x0, bottom - top, z),
        svg.firstChild);
      return;
    }
  }

  if (sel.hairpin) {
    const h = sel.hairpin;
    svg.insertBefore(
      selectionRect(h.x - 0.3, h.y - 0.5, h.width + 0.6, h.height + 1.0, z),
      svg.firstChild);
    return;
  }

  // A curve picked up by clicking it. The curve itself is lit: it is the
  // whole of the mark, and lighting the notes at its ends would say
  // Backspace was about to take those away.
  if (sel.arc) {
    svg.insertBefore(selectionArc(sel.arc, z), svg.firstChild);
    return;
  }

  if (sel.tremolo) {
    const t = sel.tremolo;
    svg.insertBefore(
      selectionRect(t.x - 0.4, t.y - 0.4, t.width + 0.8, t.height + 0.8, z),
      svg.firstChild);
    return;
  }

  // The figure alone, not the notes under it: lighting those would say
  // Backspace was about to take the music away.
  if (sel.tuplet) {
    const t = sel.tuplet;
    svg.insertBefore(
      selectionRect(t.x - 0.3, t.y - 0.3, t.width + 0.6, t.height + 0.6, z),
      svg.firstChild);
    return;
  }

  if (sel.tempo !== undefined && sel.tempo !== null) {
    const t = (state.tempos || []).find((x) => x.measure === sel.tempo);
    if (t) {
      svg.insertBefore(
        selectionRect(t.x - 0.4, t.y - 0.4, t.width + 0.8, t.height + 0.8, z),
        svg.firstChild);
      return;
    }
  }

  if (sel.dynamic) {
    const d = (state.dynamics || []).find(
      (x) => x.part === sel.part && x.measure === sel.measure && x.ref === sel.ref);
    if (d) {
      svg.insertBefore(
        selectionRect(d.x - 0.4, d.y - 0.4, d.width + 0.8, d.height + 0.8, z),
        svg.firstChild);
      return;
    }
  }

  if (sel.articulation) {
    const a = (state.articulations || []).find(
      (x) => x.part === sel.part && x.measure === sel.measure &&
             x.ref === sel.ref && x.mark === sel.articulation.mark);
    if (a) {
      svg.insertBefore(
        selectionRect(a.x - 0.4, a.y - 0.4, a.width + 0.8, a.height + 0.8, z),
        svg.firstChild);
      return;
    }
  }

  if (sel.marker) {
    const m = (state.markers || []).find(
      (x) => x.measure === sel.marker.measure && !x.repeat);
    if (m) {
      svg.insertBefore(
        selectionRect(m.x - 0.4, m.y - 0.4, m.width + 0.8, m.height + 0.8, z),
        svg.firstChild);
      return;
    }
  }

  if (sel.textMark) {
    const t = (state.textMarks || []).find(
      (x) => x.part === sel.part && x.measure === sel.measure &&
             x.ref === sel.ref && x.index === sel.textMark.index);
    if (t) {
      svg.insertBefore(
        selectionRect(t.x - 0.4, t.y - 0.4, t.width + 0.8, t.height + 0.8, z),
        svg.firstChild);
      return;
    }
  }

  if (sel.chordSymbol) {
    const c = (state.chordMarks || []).find(
      (x) => x.part === sel.part && x.measure === sel.measure && x.ref === sel.ref);
    if (c) {
      svg.insertBefore(
        selectionRect(c.x - 0.4, c.y - 0.4, c.width + 0.8, c.height + 0.8, z),
        svg.firstChild);
      return;
    }
  }

  // One notehead of a chord, picked up by clicking it. The box goes round
  // that head alone: the next accidental is going to change one note of the
  // chord, and a box round the whole chord would say otherwise.
  //
  // Only ever from a click. A dragged selection picks up whole chords, which
  // is what makes it the way to sharpen all of them at once.
  if (!state.multi.length && sel.ref !== null && sel.ref !== undefined &&
      sel.head !== null && sel.head !== undefined) {
    const hit = state.hits.find(
      (h) => h.part === sel.part && h.measure === sel.measure);
    const el = hit && (hit.elements || []).find((e) => e.ref === sel.ref);
    if (el && el.kind === "chord" && (el.steps || []).length > 1) {
      const g = headGlyphAt(svg, sel.ref, hit.staffTop + 4 - sel.head / 2, z);
      if (g) {
        g.classList.add("selected");
        const x = Number(g.getAttribute("x")) / z;
        const y = Number(g.getAttribute("y")) / z;
        svg.insertBefore(
          selectionRect(x - 0.4, y - 0.9, el.width + 0.8, 1.8, z),
          svg.firstChild);
        // Over it, as it is over any other selection: the chord is picked
        // up and the next note still goes after it.
        drawInsertCursor(svg, hit, sel, z);
        return;
      }
    }
  }

  // What the rectangle has to enclose.
  let picked = [];
  if (state.multi.length) {
    picked = state.multi;
  } else if (sel.ref) {
    picked = [{ part: sel.part, measure: sel.measure, ref: sel.ref }];
  }

  for (const m of picked) {
    // All of them: a chord is drawn as several noteheads that share one ref,
    // and a note may carry a hand-drawn symbol with the same ref again.
    // Lighting up only the first is what made a selected chord look like a
    // selected note.
    svg.querySelectorAll(`[data-ref="${m.ref}"]`).forEach((el) =>
      el.classList.add("selected"));
  }

  if (picked.length) {
    // One rectangle per staff, since a selection may run across measures but
    // a box spanning two staves would enclose music it did not select.
    const byStaff = new Map();
    for (const m of picked) {
      const hit = state.hits.find(
        (h) => h.part === m.part && h.measure === m.measure
      );
      if (!hit) continue;
      const el = (hit.elements || []).find((e) => e.ref === m.ref);
      if (!el) continue;
      const key = hit.staffTop;
      const b = byStaff.get(key) || {
        x0: Infinity, x1: -Infinity,
        top: hit.staffTop - 1.6, bottom: hit.staffTop + staffHeightOf(hit) + 1.6,
      };
      b.x0 = Math.min(b.x0, el.x - 0.5);
      b.x1 = Math.max(b.x1, el.x + el.width + 0.5);
      // The box has to reach whatever is selected, not just the staff: a
      // note on a ledger line, the top of a chord, or a symbol drawn well
      // above the music all belong inside it.
      const top = Math.min(el.y, el.y2 ?? el.y, el.top ?? Infinity);
      const bottom = Math.max(el.y, el.y2 ?? el.y, el.bottom ?? -Infinity);
      b.top = Math.min(b.top, top - 1.4);
      b.bottom = Math.max(b.bottom, bottom + 1.4);
      byStaff.set(key, b);
    }
    for (const b of byStaff.values()) {
      svg.insertBefore(
        selectionRect(b.x0, b.top, b.x1 - b.x0, b.bottom - b.top, z), svg.firstChild);
    }
    // The cursor is drawn over the selection as well, not instead of it: a
    // note has just been written and the next one goes after it, and typing
    // a melody means watching the place the next note will land while the
    // one before it is still picked up.
    const on = state.hits.find(
      (h) => h.part === sel.part && h.measure === sel.measure
    );
    if (on) drawInsertCursor(svg, on, sel, z);
    return;
  }

  // Nothing picked, but a measure is current: outline the measure, and show
  // where in it the next note would go and at what pitch.
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  if (!hit) return;
  svg.insertBefore(
    selectionRect(hit.x0, hit.staffTop - 1.6, hit.x1 - hit.x0, 7.2, z),
    svg.firstChild
  );
  drawInsertCursor(svg, hit, sel, z);
}

// CURSOR_W and CURSOR_H are the block, in staff spaces.
//
// They are the C program's, converted: key_cursor.c drew a rectangle ten
// pixels by six on a staff whose spaces were eight, in a grey of 20000 out of
// 65535. A block rather than a ghost notehead — it does not pretend to be
// music, which is what stops you reading it as a note you have already
// written.
const CURSOR_W = 10 / 8;
const CURSOR_H = 6 / 8;

// drawInsertCursor shows the place the next note will be written and the
// pitch it will be written at.
//
// Both halves matter. A mark that says where in the bar but not which line
// means the first thing you do after pressing a key is look at what came out;
// sitting the block on the line says it before you commit to it, which is
// what makes typing a melody something you can do without stopping.
function drawInsertCursor(svg, hit, sel, z) {
  const els = hit.elements || [];
  let x;
  const next = els.find((e) => e.index === sel.at);
  if (next) {
    x = next.x - 0.9;
  } else if (els.length) {
    const last = els[els.length - 1];
    x = last.x + last.width + 1.1;
  } else {
    x = hit.x0 + 1;
  }
  const step = sel.step ?? 0;
  const y = hit.staffTop + STAFF_HEIGHT - step / 2;

  // The ledger lines first, so the block sits on top of them the way a
  // notehead does. Without them a cursor below the staff floats in space and
  // says nothing about which note it is.
  for (let s = 10; s <= step; s += 2) ledger(svg, x, hit.staffTop, s, z);
  for (let s = -2; s >= step; s -= 2) ledger(svg, x, hit.staffTop, s, z);

  const block = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  block.setAttribute("class", "write-cursor write-block");
  block.setAttribute("x", String((x - CURSOR_W / 2) * z));
  block.setAttribute("y", String((y - CURSOR_H / 2) * z));
  block.setAttribute("width", String(Math.max(2, CURSOR_W * z)));
  block.setAttribute("height", String(Math.max(2, CURSOR_H * z)));
  svg.appendChild(block);
}

function ledger(svg, x, staffTop, step, z) {
  const line = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  line.setAttribute("class", "write-cursor write-ledger");
  // Shorter than the staff's own ledger lines, so that a cursor standing
  // next to a note below the staff does not run its line into the note's.
  line.setAttribute("x", String((x - 0.85) * z));
  line.setAttribute("y", String((staffTop + STAFF_HEIGHT - step / 2 - 0.065) * z));
  line.setAttribute("width", String(1.7 * z));
  line.setAttribute("height", String(Math.max(1, 0.13 * z)));
  svg.appendChild(line);
}

// headGlyphAt finds one notehead of a chord: the glyph of that element
// nearest the height given.
//
// Every notehead of a chord carries the chord's ref, which is what makes
// clicking any of them find the chord. Telling them apart afterwards is a
// matter of where they are drawn, and the only thing that knows a notehead's
// height is the step it was clicked at.
function headGlyphAt(svg, ref, y, z) {
  let best = null;
  let away = Infinity;
  for (const g of svg.querySelectorAll(`[data-ref="${ref}"]`)) {
    const d = Math.abs(Number(g.getAttribute("y")) - y * z);
    if (d < away) { best = g; away = d; }
  }
  // Within half a space, or the chord has been redrawn since it was picked
  // up and the head that was clicked is no longer where it was.
  return away <= z * 0.5 ? best : null;
}

// selectionArc is the highlight drawn along a picked-up tie or slur.
//
// Not a rectangle. A slur arcs over everything it covers, so the smallest
// box holding the curve holds all of that music too — a phrase across two
// bars came back as a pale band over both of them, and what the eye read
// was "these two bars are selected". Reported as "it does not seem obvious
// the slur was selected, the selection looks too large on two measures".
//
// So the curve is traced instead, thick and in the selection colour, and
// drawn behind the black arc it belongs to: the one shape on the page that
// is the mark, lit up as itself.
function selectionArc(a, z) {
  const p = document.createElementNS("http://www.w3.org/2000/svg", "path");
  p.setAttribute("class", "selection-arc");
  p.setAttribute("d", `M ${a.x1 * z} ${a.y1 * z} Q ${a.cx * z} ${a.cy * z} ` +
    `${a.x2 * z} ${a.y2 * z}`);
  // Wide enough to show on both sides of the black curve it sits behind,
  // which is what makes it read as the curve lit up rather than as a curve
  // drawn slightly badly. In staff spaces, so it holds at any zoom.
  p.setAttribute("stroke-width", String(0.75 * z));
  return p;
}

function selectionRect(x, y, w, h, z) {
  const r = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  r.setAttribute("class", "selection");
  r.setAttribute("x", String(x * z));
  r.setAttribute("y", String(y * z));
  r.setAttribute("width", String(Math.max(w * z, 2)));
  r.setAttribute("height", String(h * z));
  // Corners in proportion to the staff, so the shape holds at any zoom.
  r.setAttribute("rx", String(0.45 * z));
  return r;
}

// mark applies something to the element the cursor sits after — the note just
// placed or just clicked.
// orderedSelection sorts what is selected into the order it is read in.
function orderedSelection() {
  return state.multi.slice().sort(
    (a, b) => a.part - b.part || a.measure - b.measure || a.index - b.index);
}

async function mark(endpoint, params, said) {
  const sel = state.selection;
  if (!sel) {
    status("Click a note first");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at),
    ...params,
  });
  if (state.multi.length) {
    // In order. A hairpin opens on the first and closes on the last, so a
    // selection handed over shuffled would draw a crescendo backwards.
    p.set("sel", orderedSelection().map((m) => `${m.part}:${m.measure}:${m.index}`).join(","));
  } else if (sel.ref !== null && sel.ref !== undefined) {
    // One note picked up: the mark goes on that note, wherever the caret
    // has moved on to. The caret and the selection part company as soon as
    // a bar fills — the caret carries on into the next bar while what was
    // written stays selected behind it.
    const target = refTarget(sel.ref);
    if (target) p.set("sel", target);
  }
  const res = await fetch(`/api/${endpoint}?` + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not apply: " + (await res.text()).trim());
    return null;
  }
  const d = await res.json().catch(() => ({}));
  await refreshScore();
  status(said);
  return d;
}

// carryCaretOn moves the caret into the next bar when this one is full.
function carryCaretOn(sel) {
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure);
  if (!hit || !hit.full) return;
  if (sel.at < (hit.elements || []).length) return; // not at the end of it
  const next = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure + 1);
  if (!next) return; // nothing after it yet; the server will make one
  sel.measure += 1;
  sel.at = 0;
  sel.replace = false;
}

// refTarget says where a note is, as the server names places.
function refTarget(ref) {
  for (const h of state.hits) {
    const el = (h.elements || []).find((e) => e.ref === ref);
    if (el) return `${h.part}:${h.measure}:${el.index}`;
  }
  return null;
}

// hasSelection says whether a note is picked up, rather than merely a place
// in a measure being remembered.
function hasSelection() {
  if (state.multi.length) return true;
  const sel = state.selection;
  return !!(sel && sel.ref !== null && sel.ref !== undefined);
}

// pickedNotes says whether what is picked up is music, rather than a marking
// written over music.
//
// Every marking names the note it was written on, so that Backspace can find
// it — a dynamic, a hairpin, a tempo, a tie, a slur, the beams of a tremolo.
// That makes hasSelection true for all of them, which is right for "is there
// something to act on" and wrong for the arrows: moving a slur up a step
// means moving the note under it, which is not what clicking the slur asked
// for.
function pickedNotes() {
  if (state.multi.length) return true;
  const sel = state.selection;
  if (!sel || sel.ref === null || sel.ref === undefined) return false;
  if (sel.bar) return false;
  return !(sel.arc || sel.hairpin || sel.tremolo || sel.tuplet || sel.dynamic ||
    sel.chordSymbol || sel.textMark || sel.marker || sel.articulation ||
    (sel.tempo !== undefined && sel.tempo !== null));
}

// heldForMoving says whether an arrow key should move the notes that are
// picked up, rather than move the caret.
//
// A note just written is selected — it has to be, or a dynamic, a stem
// change or a tie typed straight after it would have nothing to land on —
// but being selected is not the same as being picked up to move. Writing a
// note and then pressing Up means "the next one is a step higher", and
// while that counted as a picked-up note the arrow moved the note that had
// just been written instead: the way to write a scale from the keyboard
// was to write a note, click somewhere else, click back and write the
// next. Clicking a note still picks it up, and Alt with an arrow still
// moves whatever is selected, written this moment or not.
function heldForMoving() {
  if (state.multi.length) return true;
  if (state.selection && state.selection.wrote) return false;
  return pickedNotes();
}

// heldForChanging says the selection is music somebody picked up and meant
// to change, rather than the note they have just this moment written.
//
// The difference is the whole of what makes typing bearable. A note value
// clicked with four sixteenths picked up means "those four are wrong" and
// has to retime them. The same click a moment after writing a note means
// "the next one is shorter" — somebody looking at a bar with a beat left
// in it, choosing what to fill it with — and retiming what they just
// wrote is the opposite of what they asked for, undone by hand every time.
//
// Written notes stay selected on purpose, so a dynamic or a stem change
// still lands on them. This is the same line heldForMoving draws, for the
// same reason, and in the same words: selected is not picked up.
function heldForChanging() {
  if (state.multi.length) return true;
  if (state.selection && state.selection.wrote) return false;
  return hasSelection();
}

// pickedHead names the notehead of a chord that was clicked, for the
// commands that act on one note rather than on the whole element.
//
// Nothing when a range is picked up: a dragged selection is whole chords,
// and a staff step shared by two of them is a different note in each.
function pickedHead() {
  const sel = state.selection;
  if (state.multi.length || !sel) return {};
  if (sel.head === null || sel.head === undefined) return {};
  return { head: String(sel.head) };
}

// onThatNotehead says so, when the accidental went on one note of a chord
// rather than on all of them.
function onThatNotehead() {
  const sel = state.selection;
  if (state.multi.length || !sel || sel.head === null || sel.head === undefined) {
    return "";
  }
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure);
  const el = hit && (hit.elements || []).find((e) => e.ref === sel.ref);
  return el && el.kind === "chord" && (el.steps || []).length > 1
    ? " on that notehead" : "";
}

// typeAccidental is what a sign key does, which is what the palette's button
// of the same name does.
//
// With music picked up it belongs to that music and is written now. With
// nothing picked up it is a decision about the next note, and the button
// lights to say the pen is holding it. A marking picked up is neither: a
// sharp on a slur means a sharp on the note under it, which is not what
// pointing at the slur asked for.
function typeAccidental(name, said) {
  if (pickedNotes()) {
    void setAccidental(name, said);
    return;
  }
  if (state.selection && isDrumStaff(state.selection.part)) {
    status("A drum staff has no accidentals");
    return;
  }
  state.tool.accidental = name;
  markTools();
  status(said + " — on the next note written");
}

// bracketAccidental puts the sign already on a note into brackets, or takes
// the brackets off again.
//
// A reminder rather than an instruction: the note sounds the same either
// way, so this says nothing about which sign is there, and a note without
// one has nothing to bracket.
const bracketAccidental = () =>
  mark("accidental", { parens: "toggle", ...pickedHead() }, "Brackets");

// cycleAccidental moves what is picked up one step sharper or flatter.
const cycleAccidental = (dir) =>
  mark("accidental", { cycle: dir > 0 ? "up" : "down", ...pickedHead() },
       (dir > 0 ? "Sharpened" : "Flattened") + onThatNotehead());

// setAccidental writes a sign on what is picked up, and says what came of
// it.
//
// A sign asked for where the music already says it is printed anyway — a
// sharp on an F in a key that sharpens every F, which a beginner reading
// the part has not learnt yet — and the status bar says that is why, so
// that a sign nothing required does not look like a mistake. Asking again
// takes it off.
const setAccidental = async (acc, said) => {
  const d = await mark("accidental", { accidental: acc, ...pickedHead() },
    (acc === "none" ? "Accidental removed" : "Written as " + said.toLowerCase()) +
    onThatNotehead());
  if (!d) return d;
  if (d.forced) {
    status(said + " — the music already said it, so it is printed because " +
           "you asked for it. Ask again to take it off.");
  } else if (d.unforced) {
    status(said + " taken off — the music said it anyway, so the note is " +
           "unchanged.");
  }
  return d;
};

// beamOrBreak is what the b key does, which depends on how much is picked up.
//
// Two notes or more are a group, and a group is a thing to join: that is
// what beaming is, and it is what the key has always done. One note is not a
// group, and asking to beam it was an error — so there the key means the
// other half of the same job, which is where a group ends.
//
// Breaking at the fifth of eight is a break between the fourth and the
// fifth, not something done to the fifth: it belongs to the group that
// opens, and is left joined to the three after it. Taking it off the beam
// instead meant selecting those four and beaming them again to get what the
// break was asked for in the first place.
async function beamOrBreak() {
  if (state.multi.length > 1) {
    await mark("beam", { beam: "join" }, "Beamed together");
    return;
  }
  await mark("beam", { beam: "start" }, "Beam broken before this note");
}

// A hairpin and an octave line both cover a stretch of music, so they act on
// the range that is selected rather than on one note.
const setHairpin = (kind, said) =>
  mark("hairpin", { hairpin: kind }, said);

const setOctave = (amount, said) =>
  mark("octave", { octave: amount }, said);

// setEnding puts a volta bracket over the selected measures, on every staff.
async function setEnding(numbers) {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  // A range selection may cover several bars; the bracket spans all of them.
  let first = sel.measure, last = sel.measure;
  for (const m of state.multi) {
    first = Math.min(first, m.measure);
    last = Math.max(last, m.measure);
  }
  const p = new URLSearchParams({ measure: String(first), to: String(last) });
  if (numbers === "") p.set("ending", "none");
  else p.set("numbers", numbers);

  const res = await fetch("/api/ending?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(numbers === "" ? "Ending removed"
                        : `Ending ${numbers.replace(",", " and ")} over ` +
                          (first === last ? `measure ${first + 1}`
                                          : `measures ${first + 1}–${last + 1}`));
}

// setJump writes a sign or a direction at the bar the cursor is in.
//
// These act on a bar and not on a note: a D.S. belongs at the end of a bar,
// and a segno at the head of one. The bar is the selection's, so clicking
// anywhere in it is enough.
async function setJump(what, label) {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  const p = new URLSearchParams({ measure: String(sel.measure), ...what });
  const res = await fetch("/api/jump?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not write it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  const none = what.sign === "none" || what.jump === "none";
  status(none ? `Cleared at measure ${barNumberOf(sel.measure)}`
              : `${label.split(" \u2014 ")[0]} at measure ${barNumberOf(sel.measure)}`);
}

// A slur arcs over a passage and a tie joins neighbours, so both want a range
// selected: click a note, then shift-click a later one.
async function setSlur(what) {
  if (what !== "none" && state.multi.length < 2) {
    status("A slur covers a passage — click a note, then shift-click a later one");
    return;
  }
  await mark("slur", { slur: what },
             what === "none" ? "Slur removed" : "Slurred");
}

// setTie joins the selected notes. "head" ties only the notehead that was
// clicked, which is how one note of a chord is held while the others move.
//
// One note is enough. A tie means "hold this note into the next one", and
// the note it runs to is the one that follows — the server has found it for
// a while now, across the barline if that is where it is. This went on
// asking for both anyway and never sent the request, so the only way to tie
// was to shift-click the second note and say a thing the mark already says.
// Where there is genuinely nothing after it, the server says so.
async function setTie(what, head) {
  const params = { tie: what };
  if (head && state.selection?.head !== null && state.selection?.head !== undefined) {
    params.step = String(state.selection.head);
  }
  await mark("tie", params,
    what === "none" ? "Tie removed"
      : params.step !== undefined ? "Tied that notehead" : "Tied");
}

// A tremolo is a mark on the note, so it goes through the same door as the
// rest of them — including the selection, which is what the two-note form
// is asked for with: pick the two notes the beams go between. One note
// still works, pairing with whatever is written next to it, but having to
// know that is having to know how the file is stored rather than what the
// mark means.
const tremoloNames = ["no tremolo", "one slash", "two slashes", "three slashes",
                      "four slashes", "five slashes"];

const setTremolo = (strokes, buzz, pair = false) =>
  mark("tremolo",
       { strokes: String(strokes), buzz: buzz ? "1" : "0", pair: pair ? "1" : "0" },
       pair ? "Two-note tremolo — " + strokes + " beams"
            : buzz ? "Buzz roll" : strokes === 0 ? "Tremolo removed"
            : "Tremolo — " + tremoloNames[strokes]);

// shiftPassage moves what is picked up one place along the music.
//
// Ctrl or Cmd with the left and right arrows, where the plain ones move the
// selection and leave the notes where they are. Making a passage into a
// tuplet, or shortening it, leaves room in the bar that was not there before,
// and walking the following phrase back into that room is the next thing
// anybody wants.
//
// The notes move, so the places the selection named them by are gone by the
// time this returns; the server says where each of them ended up and the
// selection follows them there, the way changing a length does.
async function shiftPassage(dir) {
  const sel = state.selection;
  if (!sel) return;
  const run = state.multi.length
    ? orderedSelection()
    : [{ part: sel.part, measure: sel.measure, index: Math.max(0, (sel.at || 1) - 1) }];

  const p = new URLSearchParams({
    dir: String(dir),
    sel: run.map((m) => `${m.part}:${m.measure}:${m.index}`).join(","),
  });
  const res = await fetch("/api/shift?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  const d = await res.json().catch(() => ({}));
  await refreshScore();

  if (Array.isArray(d.now) && d.now.length) {
    const was = state.multi.length;
    state.multi = d.now.map((at) => {
      const [part, measure, index] = at.split(":").map(Number);
      const hit = state.hits.find((h) => h.part === part && h.measure === measure);
      const el = hit && (hit.elements || []).find((e) => e.index === index);
      return { part, measure, index, ref: el ? el.ref : null };
    }).filter((m) => m.ref !== null);
    const f = state.multi[0];
    const l = state.multi[state.multi.length - 1];
    if (f) {
      state.selection = {
        part: f.part, measure: f.measure, at: f.index + 1, step: 0, ref: f.ref,
      };
      // Still the same passage, so shift with an arrow still grows it from
      // the end it was growing from before.
      state.anchor = { part: f.part, measure: f.measure, index: f.index };
      state.focus = { part: l.part, measure: l.measure, index: l.index };
    }
    // One note moved on its own stays one note, not a passage of one.
    if (!was) state.multi = [];
  }
  markSelection();
  status(dir > 0 ? "Moved later" : "Moved earlier");
}

// setTuplet turns the note at the cursor into a group played in its time.
//
// It is not a mark but a change to the music, so it has its own call: the
// one note becomes n of them, the first keeping the pitch and the rest
// coming in as rests to be written over.
//
// With several notes picked up it works the other way: those notes are the
// group. Nothing is thrown away — they keep their noteheads and are read as
// n in the time of normal, and the time the group gives back becomes a rest
// after it. Which is what you want after writing a passage out plain and
// then noticing it was a triplet.
const tupletNames = {
  2: "Duplet", 3: "Triplet", 4: "Quadruplet", 5: "Quintuplet",
  6: "Sextuplet", 7: "Septuplet", 8: "Octuplet", 9: "Nonuplet",
};

async function setTuplet(n) {
  const sel = state.selection;
  if (!sel) {
    status("Click a note or a rest first");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure),
    at: String(sel.at), n: String(n),
  });
  if (state.multi.length > 1) {
    // In order, because the group opens on the first and closes on the last.
    p.set("sel", orderedSelection()
      .map((m) => `${m.part}:${m.measure}:${m.index}`).join(","));
  }
  const res = await fetch("/api/tuplet?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not do that: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  state.multi = [];
  sel.ref = null;
  await refreshScore();
  markSelection();
  if (n <= 1) {
    status(`Tuplet removed — ${d.removed} notes back to one`);
    return;
  }
  // Say the ratio as well as the name: asked for four on a plain quarter it
  // is four in the time of two, not the four-in-three a dotted beat gets,
  // and the figure printed over the notes says so.
  const name = tupletNames[n] || n + "-tuplet";
  let said = `${name} — ${d.tuplet} in the time of ${d.normal}`;
  if (d.kept > 1) {
    // What became of the notes, since the bar now has room in it that was
    // not there before and the next thing to do is usually to move
    // something into it.
    said += `, ${d.kept} note${d.kept === 1 ? "" : "s"} kept`;
    if (d.members > d.kept) said += ` and ${d.members - d.kept} to write`;
    if (d.freed && d.freed !== "0/1") said += `; ${d.freed} freed as a rest`;
  }
  status(said);
}

// setSelectionDuration rewrites the length of the notes that are picked up,
// and keeps them picked up.
//
// Making notes longer pushes what no longer fits into the bars after them,
// so the places the selection named them by are not where they are any more.
// Left alone, a second change asked for on the same selection reached only
// the notes that happened not to have moved — which on a run of sixty-fourths
// is the first group and nothing else. The server says where each of them
// ended up and the selection follows them there.
async function setSelectionDuration(value, label) {
  const d = await mark("duration", { value, dots: String(0) },
                       "Changed to " + (label || value).toLowerCase());
  if (!d || !Array.isArray(d.now) || !d.now.length) return;

  state.multi = d.now.map((at) => {
    const [part, measure, index] = at.split(":").map(Number);
    const hit = state.hits.find((h) => h.part === part && h.measure === measure);
    const el = hit && (hit.elements || []).find((e) => e.index === index);
    return { part, measure, index, ref: el ? el.ref : null };
  }).filter((m) => m.ref !== null);

  const first = state.multi[0];
  if (first) {
    state.selection = {
      part: first.part, measure: first.measure,
      at: first.index + 1, step: 0, ref: first.ref,
    };
  }
  markSelection();
}

const setStem = (dir) =>
  mark("stem", { stem: dir },
       dir === "auto" ? "Stem left to the engraver" : "Stem " + dir);

const setDynamic = (d) =>
  mark("dynamic", { dynamic: d }, d === "none" ? "Dynamic removed" : d);

const setArticulation = (a) =>
  mark("articulation", { articulation: a },
       a === "" ? "Articulations cleared" : a);

// Where the player breathes. Said back in the words a player uses — a breath
// mark is the comma, and the others are named for the sign they draw.
const BREATH_MARKS = {
  "comma": "Breath mark",
  "tick": "Breath mark, tick",
  "caesura": "Caesura",
  "curved caesura": "Curved caesura",
};

const setBreath = (b) =>
  mark("breath", { breath: b },
       b === "none" ? "Breath mark removed" : (BREATH_MARKS[b] || b));

// How a fretted instrument plays the note. Named for what a guitarist
// calls it rather than for the endpoint's word, since that is what the
// status line says back.
const TAB_MARKS = {
  "hammer-on": "Hammer-on",
  "pull-off": "Pull-off",
  "slide-up": "Slide up",
  "slide-down": "Slide down",
  "bend": "Bend a whole tone",
  "bend-half": "Bend a semitone",
  "bend-release": "Bend and release",
  "pre-bend": "Pre-bend",
  "vibrato": "Vibrato",
  "wide-vibrato": "Wide vibrato",
  "palm-mute": "Palm mute",
  "let-ring": "Let ring",
  "natural-harmonic": "Natural harmonic",
  "artificial-harmonic": "Artificial harmonic",
  "pinch-harmonic": "Pinch harmonic",
  "tap": "Tap",
};

const setCrossStaff = (to) =>
  mark("crossstaff", { to },
       to === "own" ? "Back on its own staff"
                    : "Written on the staff " + to);

const setTechnique = (t) =>
  mark("technique", { technique: t },
       t === "" ? "Tab marks cleared" : (TAB_MARKS[t] || t));

// A dead note is a notehead and not a technique: the drum staves already
// write one, and on a tab it comes out as the x the books print. Going
// through the notehead endpoint rather than adding a second way to say it
// is what keeps a file from having two marks that mean the same thing.
const setDeadNote = () =>
  mark("notehead", { shape: "x" }, "Dead note");

/* ---------- drawing a symbol ---------- */

// gscore drew all 287 of its symbols as XPM files — grids of characters anyone
// could edit in a text editor. This is that, with a mouse: the only way to get
// a mark onto the page that no music font carries.

const draw = { size: 16, pixels: null, scale: 0.125, painting: false, erase: false };

function drawInit(size) {
  draw.size = size;
  draw.pixels = new Int8Array(size * size);
  drawRender();
}

function drawRender() {
  const c = $("#sym-canvas");
  const ctx = c.getContext("2d");
  const n = draw.size;
  const cell = c.width / n;

  ctx.clearRect(0, 0, c.width, c.height);
  ctx.fillStyle = "#fff";
  ctx.fillRect(0, 0, c.width, c.height);

  // The grid, faint, so the pixels can be counted.
  ctx.strokeStyle = "#e2e0dc";
  ctx.lineWidth = 1;
  for (let i = 0; i <= n; i++) {
    const p = Math.round(i * cell) + 0.5;
    ctx.beginPath(); ctx.moveTo(p, 0); ctx.lineTo(p, c.height); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(0, p); ctx.lineTo(c.width, p); ctx.stroke();
  }

  // Where the staff line falls, which is the symbol's own origin.
  ctx.strokeStyle = "#2f6f9f";
  ctx.setLineDash([4, 4]);
  const mid = Math.round(c.height / 2) + 0.5;
  ctx.beginPath(); ctx.moveTo(0, mid); ctx.lineTo(c.width, mid); ctx.stroke();
  ctx.setLineDash([]);

  ctx.fillStyle = "#000";
  for (let y = 0; y < n; y++) {
    for (let x = 0; x < n; x++) {
      if (draw.pixels[y * n + x]) {
        ctx.fillRect(Math.round(x * cell), Math.round(y * cell),
                     Math.ceil(cell), Math.ceil(cell));
      }
    }
  }
}

function drawPaintAt(e) {
  const c = $("#sym-canvas");
  const r = c.getBoundingClientRect();
  const n = draw.size;
  const x = Math.floor(((e.clientX - r.left) / r.width) * n);
  const y = Math.floor(((e.clientY - r.top) / r.height) * n);
  if (x < 0 || y < 0 || x >= n || y >= n) return;
  draw.pixels[y * n + x] = draw.erase ? 0 : 1;
  drawRender();
}

function setupDrawing() {
  const c = $("#sym-canvas");
  c.addEventListener("pointerdown", (e) => {
    draw.painting = true;
    draw.erase = e.altKey;
    c.setPointerCapture(e.pointerId);
    drawPaintAt(e);
  });
  c.addEventListener("pointermove", (e) => {
    if (draw.painting) drawPaintAt(e);
  });
  c.addEventListener("pointerup", () => { draw.painting = false; });

  $("#sym-size").addEventListener("change", (e) => drawInit(Number(e.target.value)));
  $("#sym-scale").addEventListener("change", (e) => { draw.scale = Number(e.target.value); });
  $("#sym-clear").addEventListener("click", () => drawInit(draw.size));
  $("#sym-import").addEventListener("click", () => void importPicture());

  $("#pack-file").addEventListener("change", async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    const res = await fetch("/api/importsymbols", {
      method: "POST", body: await file.arrayBuffer(),
    });
    if (!res.ok) {
      status("Could not import: " + (await res.text()).trim());
      return;
    }
    await afterPackImport(await res.json());
    e.target.value = "";
  });

  $("#sym-file").addEventListener("change", async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    const res = await fetch("/api/symbolimport", {
      method: "POST",
      body: await file.arrayBuffer(),
    });
    if (!res.ok) {
      status("Could not read that picture: " + (await res.text()).trim());
      return;
    }
    usePicture(await res.json(), file.name);
    e.target.value = "";
  });

  $("#sym-name").addEventListener("input", () => symbolNameIsFine());
  $("#sym-save").addEventListener("click", () => void saveDrawnSymbol());
}

// importPicture asks for a picture to trace a symbol from.
//
// Through the native dialog where there is one. The window has a hidden
// file box for this, and in the native shell clicking it does nothing at
// all: <input type="file"> needs the shell to put a panel up and this one
// does not, so the button sat there doing nothing with no way to tell
// why. The same route the symbol packs already take.
async function importPicture() {
  if (state.info?.local) {
    const res = await fetch("/api/pickfile?picture=1", { method: "POST" });
    if (res.ok) {
      const d = await res.json();
      if (!d.cancelled) {
        usePicture(d, d.name || "");
      } else {
        status("Ready");
      }
      return;
    }
    // Served rather than local, or no dialog to be had: fall through to
    // the file box, which is what a browser answers with.
  }
  $("#sym-file").click();
}

// usePicture puts an imported picture onto the drawing canvas.
function usePicture(d, filename) {
  // Fit the imported picture into a grid the editor offers.
  const n = Math.max(d.w, d.h);
  const choices = [12, 16, 24, 32, 48].filter((v) => v >= n);
  drawInit(choices.length ? choices[0] : 48);
  for (let y = 0; y < d.h && y < draw.size; y++) {
    for (let x = 0; x < d.w && x < draw.size; x++) {
      draw.pixels[y * draw.size + x] = d.pixels[y * d.w + x];
    }
  }
  if (d.pixelSize) {
    draw.scale = d.pixelSize;
    $("#sym-scale").value = String(d.pixelSize);
  }
  const named = String(filename || "").replace(/\.[^.]+$/, "").replace(/[^\w-]+/g, "");
  if (!$("#sym-name").value && named) $("#sym-name").value = named;
  drawRender();
  status(filename
    ? "Imported " + filename + " — edit and save"
    : "Imported — edit and save");
}

// symbolNameIsFine clears the complaint about the name as soon as it is
// being answered, so the box stops being red while somebody is typing.
function symbolNameIsFine() {
  $("#sym-name")?.classList.remove("wrong");
  const why = $("#sym-name-why");
  if (why) {
    why.hidden = true;
    why.textContent = "";
  }
}

// symbolNameIsWrong says why the window is not closing, beside the box it
// is about.
//
// Beside the box and not only in the status bar, and the window stays
// open: somebody who has just pressed Save is looking at the button they
// pressed, and a drawing that vanished along with the reason it would not
// save is a drawing they have to make again.
function symbolNameIsWrong(why) {
  const box = $("#sym-name");
  const said = $("#sym-name-why");
  if (said) {
    said.textContent = why;
    said.hidden = false;
  }
  if (box) {
    box.classList.add("wrong");
    box.focus();
    box.select();
  }
  status(why);
}

// saveDrawnSymbol keeps the drawing until it has somewhere to go.
async function saveDrawnSymbol() {
  const name = $("#sym-name").value.trim();
  if (!name) {
    symbolNameIsWrong("The symbol needs a name before it can be saved");
    return;
  }

  const send = (replace) => fetch(
    "/api/symbols" + (replace ? "?replace=1" : ""),
    {
      method: "POST",
      body: JSON.stringify({
        name,
        w: draw.size,
        h: draw.size,
        pixels: Array.from(draw.pixels),
        pixelSize: draw.scale,
        originX: 0,
        originY: draw.size / 2,
      }),
    });

  let res = await send(false);
  if (res.status === 409) {
    // A name already taken. The one that would go is a symbol they drew
    // earlier and may already have used, so it is their decision.
    const sure = await confirmThat({
      title: `There is already a symbol called ${name}`,
      note: "Replacing it changes that symbol everywhere it has been " +
            "used, in this score and in any other.",
      ok: "Replace it",
    });
    if (!sure) {
      symbolNameIsWrong(`There is already a symbol called ${name} — ` +
        "give this one another name");
      return;
    }
    res = await send(true);
  }
  if (!res.ok) {
    symbolNameIsWrong("Could not save: " + (await res.text()).trim());
    return;
  }

  symbolNameIsFine();
  status("Saved the symbol " + name);
  $("#draw").close();
  await openPalette();
}

// The symbols the palette is showing, with their pictures, so that
// searching does not fetch them all again on every keystroke.
let paletteSymbols = [];

async function openPalette() {
  const list = await (await fetch("/api/symbols")).json();
  $("#palette-empty").hidden = list.length > 0;

  // Fetched rather than given as an <img src>. Running as a page there is no
  // server: requests are answered by the core compiled to WebAssembly, and
  // that can only intercept fetch — an <img> goes straight to the network
  // and finds nothing there.
  const pics = await Promise.all(
    list.map(async (s) => {
      const r = await fetch("/symbol.png?name=" + encodeURIComponent(s.name));
      return r.ok ? URL.createObjectURL(await r.blob()) : "";
    })
  );
  paletteSymbols = list.map((s, i) => ({ ...s, pic: pics[i] }));

  // A search box once there are more than a screenful. A folder of forty
  // marks is a folder somebody looks something up in; four is a row of
  // pictures and a box over it would only be in the way.
  const search = $("#palette-search");
  if (search) search.hidden = paletteSymbols.length < 8;
  const filter = $("#palette-filter");
  if (filter) {
    filter.value = "";
    filter.oninput = () => drawPalette(filter.value);
  }

  const folder = paletteSymbols.filter((s) => s.source === "folder").length;
  const note = $("#palette-folder");
  if (note) {
    note.textContent = folder
      ? `${folder} of these are in your symbols folder, so they are in every score you open.`
      : "";
  }

  drawPalette("");
  $("#palette").showModal();
}

// drawPalette fills the grid with whatever matches.
function drawPalette(query) {
  const q = String(query || "").trim().toLowerCase();
  const shown = q === ""
    ? paletteSymbols
    : paletteSymbols.filter((s) => s.name.toLowerCase().includes(q));

  const none = $("#palette-none");
  if (none) none.hidden = shown.length > 0 || paletteSymbols.length === 0;

  $("#palette-grid").innerHTML = shown
    .map(
      (s) => `<button class="palette-item" data-symbol="${escapeHTML(s.name)}">
        <img src="${s.pic}" alt="">
        <span>${escapeHTML(s.name)}</span>
        ${s.source === "folder" ? '<span class="from">folder</span>' : ""}
      </button>`
    )
    .join("");
  $$("#palette-grid .palette-item").forEach((b) =>
    b.addEventListener("click", () => insertSymbol(b.dataset.symbol))
  );
}

async function insertSymbol(name) {
  const sel = state.selection;
  if (!sel) {
    status("Click the measure to put it in first");
    return;
  }
  const p = new URLSearchParams({
    name,
    part: String(sel.part),
    measure: String(sel.measure),
    at: String(sel.at),
    attach: "1",
  });
  const res = await fetch("/api/insertsymbol?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not insert: " + (await res.text()).trim());
    return;
  }
  $("#palette").close();
  await refreshScore();
  status("Inserted " + name);
}

// A .gscore carries its symbols, so one can be used as a library: somebody
// draws a set of marks their music needs, saves it, and anyone can take them.
async function importSymbolPack() {
  if (state.info?.local) {
    const res = await fetch("/api/pickfile?symbols=1", { method: "POST" });
    if (res.ok) {
      const d = await res.json();
      if (!d.cancelled) {
        await afterPackImport(d);
        return;
      }
      status("Ready");
      return;
    }
  }
  $("#pack-file").click();
}

async function afterPackImport(d) {
  await loadInfo();
  await openPalette();
  status(`Imported ${d.added} symbols` + (d.replaced ? `, replaced ${d.replaced}` : ""));
}

async function exportSymbolPack() {
  if (state.info?.local) {
    let res = await fetch("/api/exportsymbols?dialog=1", { method: "POST" });
    if (res.ok) {
      let d = await res.json();
      if (d.needPath) {
        const where = await ask({
          title: "Share symbols", label: "Save to",
          value: d.suggested || "symbols.gscore",
          note: "The system's save panel would not open here, so type the path.",
        });
        if (where === null || where.trim() === "") {
          status("Ready");
          return;
        }
        res = await fetch("/api/exportsymbols?path=" + encodeURIComponent(where.trim()),
                          { method: "POST" });
        if (!res.ok) {
          status("Could not share: " + (await res.text()).trim());
          return;
        }
        d = await res.json();
      }
      status(d.cancelled ? "Save cancelled" : `Shared ${d.symbols} symbols`);
      return;
    }
    status("Could not share: " + (await res.text()).trim());
    return;
  }
  // Served rather than run: hand the file to the browser.
  const res = await fetch("/api/exportsymbols", { method: "POST" });
  if (!res.ok) {
    status("Could not share: " + (await res.text()).trim());
    return;
  }
  const blob = await res.blob();
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = "symbols.gscore";
  a.click();
  URL.revokeObjectURL(a.href);
  status("Symbols downloaded");
}

// saveScore writes the score out: back where it came from, or wherever the
// save panel is told to put it.
//
// Save and Save as are the same work with one question between them, and the
// question is whether there is already an answer. A score opened from a file
// has one, and writing it back is what Save means everywhere else; a score
// that came from nowhere, or from a format gscore cannot write, falls
// through to the panel rather than refusing.
//
// Whether a .gscore carries a PDF of itself is not asked here. It is a
// setting, which is what it always was — a decision about the program, not
// about this file — and it used to be a second menu entry that had to be
// found and understood before the first one could be trusted.
// saveAs says to ask where this goes rather than writing it back where it
// came from, which is what Save as… means and what a score with no file of
// its own needs anyway.
//
// Not called "ask": that is the name of the window-asking helper, and a
// parameter of that name shadowed it — so the one path that needs to ask
// for a name, a browser with no system save panel, called a boolean and
// threw. Save did nothing at all there, and said nothing about why.
async function saveScore(saveAs) {
  const here = state.info?.filename || "";

  // Nowhere to write to: the file comes back and the browser asks where it
  // goes, so saving in a page is always a save-as. The bytes are the ones
  // the desktop would have written — the drawings and the PDF of the page
  // are in there, which the plain exporter does not carry.
  if (!state.info?.local && !state.info?.native) {
    const res = await saveRequest("", "Saving…");
    if (!res.ok) {
      status("Could not save: " + (await res.text()).trim());
      return;
    }
    const name = await keepFile(res, "score.gscore");
    await loadInfo();
    status("Saved " + name);
    return;
  }

  if (!saveAs && here) {
    const back = await saveRequest("path=" + encodeURIComponent(here), "Saving…");
    if (back.ok) {
      const d = await back.json();
      await loadInfo();
      status("Saved " + d.path);
      return;
    }
    // gscore cannot write that format back. Ask where it should go instead.
  }
  let res = await saveRequest("", "Saving…");
  if (!res.ok) {
    status("Could not save: " + (await res.text()).trim());
    return;
  }
  let d = await res.json();

  // The system's save panel could not be shown. That is not a reason for the
  // menu entry to do nothing, which is what it did before: gscore asks for
  // the name itself and writes the file.
  if (d.needPath) {
    const where = await ask({
      title: "Save as .gscore",
      label: "Save to",
      value: d.suggested || "untitled.gscore",
      note: "The system's save panel would not open here, so type the path.",
    });
    if (where === null || where.trim() === "") {
      status("Ready");
      return;
    }
    res = await saveRequest("path=" + encodeURIComponent(where.trim()),
                          "Saving…");
    if (!res.ok) {
      status("Could not save: " + (await res.text()).trim());
      return;
    }
    d = await res.json();
  }

  if (d.cancelled) {
    status("Save cancelled");
    return;
  }
  await loadInfo();
  status("Saved " + d.path);
}

// splitMeasure ends the bar where the cursor is.
async function splitMeasure() {
  const sel = state.selection;
  if (!sel || !sel.at) {
    status("Put the cursor where the bar should end");
    return;
  }
  const q = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at),
  });
  const res = await fetch("/api/splitmeasure?" + q.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not put a barline there: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status("Barline added");
}

// setTrackBarlines turns the arithmetic on a bar's length on and off.
//
// Turning it back on changes nothing that is already written: the bars that
// do not add up stay as they are, and only the next thing typed is moved
// along if it will not fit. That is the point of the switch — somebody
// typing in a piece of old music wants the program to stop correcting them
// for a while, not to be told later that it has gone back and tidied up.
async function setTrackBarlines(on) {
  const res = await fetch("/api/settings?trackBarlines=" + (on ? "1" : "0"),
                          { method: "POST" });
  if (!res.ok) {
    status("Could not change that");
    return;
  }
  state.trackBarlines = on;
  markBarlineTick();
  status(on
    ? "Bars are kept to the meter — what will not fit moves into the next bar"
    : "Bars are left as written — notes stay in the bar they are typed in");
}

function markBarlineTick() {
  const t = $("#barline-tick");
  if (t) t.textContent = state.trackBarlines ? "✓" : "";
}

// settingsDialog is what gscore remembers between sessions.
// The notes, in the order the palette is indexed in: the scale spelled
// from C, which is model.Step's own order on the other side.
const NOTE_LETTERS = ["C", "D", "E", "F", "G", "A", "B"];

// noteColoursDialog turns the coloured noteheads on and off, and changes the
// colours.
//
// Everything in it takes effect as it is changed, with the score behind the
// window redrawn each time. There is nothing to confirm: what the colours
// look like is the whole question, and a window that hid the answer until it
// was closed would be asking somebody to choose blind. The one button that
// is not immediate is the one that puts the palette back, because that is
// the only thing in here that throws work away.
//
// The colours are sent to the server rather than kept in the page. They are
// remembered for the person and they have to reach the engraver, which is
// where the PDF and the printed page are drawn: a palette that lived in the
// browser would colour the screen and print black.
async function noteColoursDialog() {
  const dlg = $("#notecolours");
  const on = $("#notecolours-on");
  const list = $("#notecolours-list");

  let now = await (await fetch("/api/settings")).json().catch(() => ({}));
  let colours = { ...(now.noteColours || {}) };

  // Sent as the whole palette, the way the key bindings are: the server
  // takes a complete set, and nothing in between is a half-written palette.
  const send = async (params) => {
    const res = await fetch("/api/settings?" + new URLSearchParams(params),
                            { method: "POST" });
    if (!res.ok) {
      status("Could not change the colours");
      return;
    }
    now = await res.json().catch(() => now);
    colours = { ...(now.noteColours || {}) };
    state.colourNotes = !!now.colourNotes;
    markViewTicks();
    draw();
    await refreshScore();
  };

  const draw = () => {
    on.checked = !!now.colourNotes;
    list.innerHTML = "";
    for (const letter of NOTE_LETTERS) {
      const row = document.createElement("label");
      row.className = "tempo-bar";
      const says = document.createElement("span");
      says.textContent = letter;
      row.appendChild(says);
      const pick = document.createElement("input");
      pick.type = "color";
      pick.value = colours[letter] || "#000000";
      // On change and not on input: dragging round a colour wheel fires
      // every few pixels, and each one is a round trip and a redrawn
      // score. The colour follows the pointer in the picker itself either
      // way, so nothing is lost by waiting for the choice.
      pick.onchange = () => {
        colours[letter] = pick.value;
        void send({ noteColours: JSON.stringify(colours) });
      };
      row.appendChild(pick);
      list.appendChild(row);
    }
  };

  on.onchange = () => void send({ colourNotes: on.checked ? "1" : "0" });
  draw();

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  // An empty palette is how the server is told to go back to its own
  // colours, which is the same way an empty set of bindings works.
  if (answer === "reset") await send({ noteColours: "{}" });
  status("Ready");
}

async function settingsDialog() {
  const dlg = $("#settings");
  const pdf = $("#setting-score-pdf");
  const hear = $("#setting-entry-sound");
  const langBox = $("#setting-lang");
  const now = await (await fetch("/api/settings")).json().catch(() => ({}));
  pdf.checked = now.scorePDF !== false;
  if (hear) hear.checked = now.entrySound !== false;

  if (langBox) {
    langBox.innerHTML = "";
    // "Automatic" first, because it is right nearly always and is what
    // somebody who has never thought about it already has.
    const auto = document.createElement("option");
    auto.value = "auto";
    auto.textContent = t("Automatic (your browser’s)");
    langBox.append(auto);
    for (const l of LANGUAGES) {
      const o = document.createElement("option");
      o.value = l.code;
      // Each language named in itself, so somebody looking for their own
      // finds it without reading English first.
      o.textContent = l.name;
      langBox.append(o);
    }
    langBox.value = languageChoice();
  }

  await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve();
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });

  const want = langBox ? langBox.value : languageChoice();
  const p = new URLSearchParams({ scorePDF: pdf.checked ? "1" : "0", lang: want });
  if (hear) {
    p.set("entrySound", hear.checked ? "1" : "0");
    // Held here as well as written away, because this one is read on every
    // keystroke and reading it back off the server each time would be a
    // request per note.
    state.entrySound = hear.checked;
  }
  await fetch("/api/settings?" + p.toString(), { method: "POST" });
  if (want !== languageChoice()) {
    await chooseLanguage(want);
    return;
  }
  status(pdf.checked
    ? "Saved scores will carry a PDF — rename one .pdf to read it anywhere"
    : "Saved scores will hold the music only");
}

// chooseLanguage puts the interface into a language and keeps the choice.
//
// Kept in two places: the settings file, which is where a desktop build
// remembers things, and the browser's own store, which is the only one a
// page served from a web server has. Whichever answers first is read at
// startup, and they are written together so they cannot disagree.
async function chooseLanguage(choice) {
  setLanguage(choice);
  try {
    localStorage.setItem("lang", choice);
  } catch (_) {
    // A private window may refuse. The setting on the server still holds.
  }
  applyLanguage();
  status(t("The language the menus and panels are written in."));
}

// applyLanguage puts every word on the page into the language in force.
//
// The markup, then the parts built from tables: the palette is drawn from
// its own list and the menus carry the names of the keystrokes, and neither
// is in the page until script puts it there.
function applyLanguage() {
  document.documentElement.lang = currentLanguage();
  translateDOM(document.body);
  buildPalette(paletteAction);
  markTools();
  syncMenuKeys();
  syncMenuIcons();
}

// startLanguage settles which language the interface opens in.
//
// What somebody asked for, if they ever did; otherwise whatever the browser
// asks for. The server is asked first because that is where a desktop build
// keeps it, and the browser's own store answers when there is no server
// setting — a page on the web has no settings file behind it.
async function startLanguage(now) {
  let choice = (now && now.lang) || "";
  if (!choice) {
    try {
      choice = localStorage.getItem("lang") || "";
    } catch (_) {
      choice = "";
    }
  }
  setLanguage(choice || "auto");
  document.documentElement.lang = currentLanguage();
  translateDOM(document.body);
}

// ask puts one question and returns the answer, or null if it was dismissed.
//
// It replaces prompt(), which does not exist in a native window: the web
// views used by the desktop builds implement no JavaScript dialog delegate,
// so prompt() quietly returns null and whatever asked for the value does
// nothing at all.
function ask({ title, label, value = "", note = "", options = null,
               search = false, many = false }) {
  const dlg = $("#ask");
  const input = $("#ask-input");
  const select = $("#ask-select");
  const filter = $("#ask-filter");

  $("#ask-title").textContent = title;
  $("#ask-label").textContent = label;
  $("#ask-note").textContent = note;
  $("#ask-note").hidden = !note;

  const useList = Array.isArray(options) && options.length > 0;
  // A list long enough to scroll gets a search box. A hundred and twenty-eight
  // instruments is not a list you read, it is one you look something up in.
  const useFilter = useList && (search || options.length > 12);
  input.hidden = useList;
  select.hidden = !useList;
  filter.hidden = !useFilter;

  const asOption = (o) => ({
    value: typeof o === "string" ? o : o.value,
    label: typeof o === "string" ? o : o.label,
    // A heading to gather this one under. Options carrying one are drawn
    // in groups; a list where nothing carries one is drawn flat, as it
    // always was.
    group: typeof o === "string" ? "" : (o.group || ""),
    // An option that is a thing to do rather than a thing to choose.
    // Picking it answers the question there and then: "Download
    // another…" is not an answer to "play through which soundfont", it
    // is the next window, and making somebody press OK to get to it is
    // a press that decides nothing.
    now: typeof o === "string" ? false : !!o.now,
  });
  const all = useList ? options.map(asOption) : [];
  let onFilter = null;

  if (useList) {
    // Drawn under headings where the options carry them, which is what
    // turns three hundred instruments from a wall into a list somebody can
    // find a cor anglais in. The options arrive in order and a group is in
    // one piece, so this is a single pass: a heading opens when the group
    // changes and closes when the next one does.
    const render = (list, keep) => {
      let html = "", open = "";
      for (const o of list) {
        const g = o.group || "";
        if (g !== open) {
          if (open) html += "</optgroup>";
          if (g) html += `<optgroup label="${escapeHTML(g)}">`;
          open = g;
        }
        html += `<option value="${escapeHTML(o.value)}">${escapeHTML(o.label)}</option>`;
      }
      if (open) html += "</optgroup>";
      select.innerHTML = html;
      if (keep && list.some((o) => o.value === keep)) select.value = keep;
      else if (list.length) select.selectedIndex = 0;
    };
    render(all, value);
    // Several at once, where several is what the question is about: "which
    // instruments" has no single answer and asking it once per instrument
    // is asking it badly.
    select.multiple = !!many;
    select.size = many ? Math.min(10, all.length)
      : useFilter ? Math.min(10, all.length) : 0;
    if (many) {
      const want = [].concat(value || []);
      for (const o of select.options) o.selected = want.includes(o.value);
    }
    if (useFilter) {
      filter.value = "";
      onFilter = () => {
        const q = filter.value.trim().toLowerCase();
        const keep = select.value;
        // The heading counts as part of what is being searched: typing
        // "brass" should find the brass, and nothing in that group has
        // the word in its name.
        const hit = (o) => o.label.toLowerCase().includes(q) ||
          (o.group || "").toLowerCase().includes(q);
        render(q ? all.filter(hit) : all, keep);
      };
      filter.addEventListener("input", onFilter);
    }
    $("#ask-label").setAttribute("for", useFilter ? "ask-filter" : "ask-select");
  } else {
    input.value = value;
    $("#ask-label").setAttribute("for", "ask-input");
  }

  // The options that act the moment they are picked, and the guard that
  // keeps them from acting when nobody picked them.
  //
  // A closed dropdown on Windows and Linux moves its value with the arrow
  // keys and fires the same event the mouse does, so somebody feeling
  // their way down the list with the keyboard would open the download
  // window on the way past. A mouse press on the list comes first where
  // it was a real choice, and where it did not, the dialog waits for OK
  // exactly as it always has.
  const actNow = new Set(all.filter((o) => o.now).map((o) => o.value));
  let pointedAt = 0;
  const onPoint = () => { pointedAt = Date.now(); };
  const onPick = () => {
    if (!actNow.has(select.value)) return;
    if (Date.now() - pointedAt > 1500) return;
    dlg.close("ok");
  };
  if (actNow.size && !many) {
    select.addEventListener("pointerdown", onPoint);
    select.addEventListener("change", onPick);
  }

  return new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      select.removeEventListener("pointerdown", onPoint);
      select.removeEventListener("change", onPick);
      if (onFilter) filter.removeEventListener("input", onFilter);
      if (dlg.returnValue !== "ok") {
        resolve(null);
        return;
      }
      if (many) {
        resolve([...select.selectedOptions].map((o) => o.value));
        return;
      }
      resolve((useList ? select.value : input.value).trim());
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    // Ready to type, which is the point of a one-question dialog — and for
    // a box that opens with something in it, ready to type *over*. The Go
    // to measure box opens at the bar you are standing in, and nobody
    // wants to delete that before typing the one they want.
    setTimeout(() => {
      const el = useFilter ? filter : useList ? select : input;
      el.focus();
      if (el === input && input.value) input.select();
    }, 0);
  });
}

// confirmThat asks a question that only needs "go on" or "don't".
//
// The same one-question dialog ask uses, with nothing to type in it. A
// question of this shape should not be a box to type into: making somebody
// retype the name of the staff they are standing on is a lock rather than a
// question, and it did not even work — a staff with no name of its own
// asked for the words "that staff", which nobody would guess and nobody
// should have to.
//
// Enter answers yes, because the dialog's own rule is that Enter presses the
// first button, and because everything it guards can be taken back.
function confirmThat({ title, note = "", ok = "OK" }) {
  const dlg = $("#ask");
  const label = $("#ask-label");
  const okBtn = $("#ask-ok");

  $("#ask-title").textContent = title;
  $("#ask-note").textContent = note;
  $("#ask-note").hidden = !note;
  label.hidden = true;
  $("#ask-input").hidden = true;
  $("#ask-select").hidden = true;
  $("#ask-select").multiple = false;
  $("#ask-filter").hidden = true;
  const wasOK = okBtn.textContent;
  okBtn.textContent = ok;

  return new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      // Put the dialog back the way the next question will want it.
      label.hidden = false;
      okBtn.textContent = wasOK;
      resolve(dlg.returnValue === "ok");
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    setTimeout(() => okBtn.focus(), 0);
  });
}

// The things that belong to the score or the staff rather than to a note.
// Each asks for one value and sends it; nothing here is worth a panel.

// setTempoDialog asks for a speed and the beat it is counted in.
//
// The beat matters as much as the number: 6/8 goes at so many dotted quarters
// a minute and not at so many quarters, and a tempo of 120 with no note
// beside it does not say anything. So the note is picked by looking at it,
// which is the way a musician reads a metronome mark in the first place.
//
// Where it lands is the bar that is selected, and the piece itself when
// nothing is — which is the same thing as the first bar, because the mark in
// the title block is that marking written over bar one.
// pickupDialog makes the first bar an upbeat, or a whole bar again.
async function pickupDialog() {
  const dlg = $("#pickup");
  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  if (answer !== "ok") {
    status("Ready");
    return;
  }
  const [dur, dots] = $("#pickup-dur").value.split(",");
  const p = new URLSearchParams({ dur });
  if (dots) p.set("dots", dots);

  const res = await fetch("/api/pickup?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  const d = await res.json().catch(() => null);
  await refreshScore();
  markSelection();
  status(d && d.full
    ? "The first bar is a whole bar again"
    : "The first bar is an upbeat — bar one is the first full bar");
}

/* ---------- finding words, and changing them ---------- */

// findDialog finds words anywhere in the score and can change them all.
//
// The matches are listed as they are typed rather than on a button,
// because "replace all" without having seen what all of it is is how
// somebody renames an instrument and silently rewrites a lyric that
// happened to contain the same letters. The list says what will be
// changed before anything is.
async function findDialog() {
  const dlg = $("#find");
  const what = $("#find-what");
  const with_ = $("#find-with");
  const fold = $("#find-case");
  const list = $("#find-hits");

  const show = (hits) => {
    list.innerHTML = "";
    if (!hits.length) {
      list.innerHTML = `<p class="small">${t("Nothing found.")}</p>`;
      return;
    }
    const head = document.createElement("p");
    head.className = "small";
    head.textContent = `${hits.length} ${hits.length === 1
      ? t("place") : t("places")}`;
    list.appendChild(head);
    // The first twenty. A score with four hundred lyrics matching "the"
    // would otherwise build four hundred rows nobody reads, and the
    // count above already says how many there are.
    for (const h of hits.slice(0, 20)) {
      const row = document.createElement("div");
      row.className = "find-hit";
      const where = document.createElement("span");
      where.className = "find-where";
      where.textContent = h.measure >= 0
        ? `${t(h.where)} · ${t("bar")} ${barNumberOf(h.measure)}`
        : t(h.where);
      row.appendChild(where);
      const text = document.createElement("span");
      text.textContent = h.text;
      row.appendChild(text);
      list.appendChild(row);
    }
    if (hits.length > 20) {
      const more = document.createElement("p");
      more.className = "small";
      more.textContent = `… ${t("and")} ${hits.length - 20} ${t("more")}`;
      list.appendChild(more);
    }
  };

  const look = async () => {
    const q = what.value;
    if (!q) {
      list.innerHTML = "";
      return;
    }
    const p = new URLSearchParams({ q });
    if (fold.checked) p.set("case", "1");
    const res = await fetch("/api/find?" + p.toString());
    if (!res.ok) {
      list.innerHTML = "";
      return;
    }
    show((await res.json()).hits || []);
  };

  what.oninput = () => void look();
  fold.onchange = () => void look();
  what.value = "";
  with_.value = "";
  list.innerHTML = "";

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    what.focus();
  });
  if (answer !== "ok" || !what.value) {
    status("Ready");
    return;
  }

  const p = new URLSearchParams({ q: what.value, with: with_.value });
  if (fold.checked) p.set("case", "1");
  const res = await fetch("/api/find?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not replace: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  await refreshScore();
  markSelection();
  status(d.changed
    ? `Changed ${d.changed} ${d.changed === 1 ? "place" : "places"}`
    : "Nothing to change");
}

/* ---------- where the lines of music end ---------- */

// systemsDialog lays the bars out so many to a line, or lets them reflow.
//
// The two buttons are the two things anybody wants here and they are
// opposites, which is why both are on the window rather than one being
// hidden behind the other: having said "four to a line" there has to be a
// way back, and hunting for it among the breaks you just made is not one.
async function systemsDialog() {
  const dlg = $("#systems");
  const fit = $("#systems-fit");
  const from = $("#systems-from");
  const to = $("#systems-to");
  const says = $("#systems-says");

  const bars = state.hits.reduce((n, h) => Math.max(n, h.measure + 1), 0);
  if (!bars) {
    status("There is nothing written yet");
    return;
  }
  const where = movementHere();
  from.max = to.max = String(bars);
  from.value = String(barNumberOf(state.selection?.measure ?? 0));
  to.value = String(barNumberOf(bars - 1));

  const show = (d) => {
    const n = (d.breaks || []).length;
    says.textContent = n
      ? `${n} ${n === 1 ? t("line is set where it is") : t("lines are set where they are")}`
      : t("The bars are laid out as they fall.");
  };

  const read = await fetch("/api/systems");
  if (!read.ok) {
    status("Could not read the layout");
    return;
  }
  show(await read.json());

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  if (answer !== "ok" && answer !== "clear") {
    status("Ready");
    return;
  }

  // The fields ask for bars by the names the score calls them; the
  // endpoint wants the indices they are stored under.
  const p = new URLSearchParams({
    from: String(barIndexOf(Number(from.value), where)),
    to: String(barIndexOf(Number(to.value), where)),
  });
  // Clearing first either way: laying out afresh means forgetting what
  // was there, not adding to it.
  p.set("clear", "1");
  if (answer === "ok") p.set("fit", fit.value);

  const res = await fetch("/api/systems?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not lay it out: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  await refreshScore();
  markSelection();
  status(answer === "ok"
    ? `${fit.value} bars to a line, from bar ${from.value} to ${to.value}`
    : "The bars will fall where they will again");
  show(d);
}

/* ---------- how the bars are numbered ---------- */

// numberingDialog settles where the bar numbers are printed.
//
// Three things, and the third is a different shape from the other two: how
// often and whether boxed belong to the score, while starting the count
// again belongs to a bar. So the window has a list of the restarts already
// written, each of which can be taken away — without it there would be no
// way to find one again except by scrolling the score looking for a number
// that goes backwards.
async function numberingDialog() {
  const dlg = $("#numbering");
  const every = $("#numbering-every");
  const boxed = $("#numbering-boxed");
  const at = $("#numbering-at");
  const number = $("#numbering-number");
  const list = $("#numbering-list");

  const show = (d) => {
    every.value = String(d.every || 0);
    boxed.checked = !!d.boxed;
    const bars = Object.keys(d.restarts || {})
      .map(Number).sort((a, b) => a - b);
    list.innerHTML = "";
    if (!bars.length) {
      list.textContent = t("The count runs straight through.");
      return;
    }
    for (const bar of bars) {
      const row = document.createElement("div");
      row.className = "tempo-bar";
      const says = document.createElement("span");
      // The bar as the editor names it, which is not the same as the index
      // it is stored under: a piece with an upbeat counts from zero.
      says.textContent = `${t("Bar")} ${barNumberOf(bar)} → ${d.restarts[bar]}`;
      row.appendChild(says);
      const drop = document.createElement("button");
      drop.type = "button";
      drop.className = "linklike";
      drop.textContent = t("Remove");
      drop.onclick = () => void send({ at: String(bar), number: "none" });
      row.appendChild(drop);
      list.appendChild(row);
    }
  };

  const send = async (params) => {
    const res = await fetch("/api/numbering?" + new URLSearchParams(params),
                            { method: "POST" });
    if (!res.ok) {
      status("Could not set it: " + (await res.text()).trim());
      return false;
    }
    show(await res.json());
    await refreshScore();
    return true;
  };

  // How often and whether boxed take effect as they are changed, because
  // what they do is only visible on the page behind the window.
  every.onchange = () => void send({ every: every.value });
  boxed.onchange = () => void send({ boxed: boxed.checked ? "1" : "0" });

  const res = await fetch("/api/numbering");
  if (!res.ok) {
    status("Could not read the numbering");
    return;
  }
  show(await res.json());
  at.value = String(barNumberOf(state.selection?.measure ?? 0));
  // Left empty on purpose. Enter confirms this window, and somebody who
  // opened it to change how often the numbers are printed should not get
  // a restart written at whatever bar they happened to be standing on.
  number.value = "";

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  if (answer !== "ok" || number.value.trim() === "") {
    status("Ready");
    return;
  }
  // The field asks for the bar by the name the score calls it; the
  // endpoint wants the index it is stored under. The same conversion the
  // Go-to-measure dialog uses, and within the movement the cursor is in,
  // because that is what the number in the field means to the person who
  // typed it.
  const bar = barIndexOf(Number(at.value), movementHere());
  if (!Number.isInteger(bar) || bar < 0) {
    status(`There is no bar ${at.value}`);
    return;
  }
  if (await send({ at: String(bar), number: number.value })) {
    status(`Bar ${at.value} is now called ${number.value}`);
  }
}


/* ---------- what each engraving measurement does ---------- */

// A picture for each measurement, showing which gap it is.
//
// The number on its own says nothing: nobody knows what 0.4 of a staff
// space between an accidental and its note looks like, and a dialog of
// thirty-four numbered fields is thirty-four guesses. gscore 0.0.9 drew a
// little picture beside each one — the music in the ink of the words, and
// the distance being set marked in red — and that is the whole of what
// makes such a window usable. Same idea here, and the same two colours:
// black is what is written, red is what the field changes.
//
// Built from a description rather than drawn by hand, because thirty-four
// hand-drawn SVGs are thirty-four chances to mark the wrong gap. Each
// picture names its glyphs and where they stand, and the red mark is one
// span across or down.
//
//   g: [codepoint, x, staffStep]  a music glyph, by its place on the staff
//   l: [x0, y0, x1, y1, weight]   a plain line: a barline, a beam, a staff
//   h: [x0, x1, step]             the red mark, measuring across
//   v: [x, step0, step1]          the red mark, measuring down
//   staff: step                   five staff lines behind it all
//
// x is in staff spaces from the left. A step is half a staff space, zero
// at the middle line and positive upwards, which is how the engraver
// counts everywhere else.
const KNOB_PICTURES = {
  // Notes and rests. The width of a slot is the distance from one note to
  // the next, so two notes and the gap between them.
  noteBase: { g: [["\ue1d5", 1, 0], ["\ue1d5", 6, 0]], h: [1.6, 6.6, -5] },
  // Two slots of different lengths, because the exponent is what sets how
  // much wider a long note's slot is than a short one's.
  noteExponent: { g: [["\ue1d3", 0.8, 0], ["\ue1d5", 5.6, 0], ["\ue1d5", 7.6, 0]],
    h: [1.4, 6.2, -5], h2: [6.2, 8.2, -5] },
  minNote: { g: [["\ue1d5", 1, 0], ["\ue1d5", 3.2, 0]], h: [1.6, 3.8, -5] },

  // Accidentals.
  beforeAccidental: { g: [["\ue1d5", 0.6, 0], ["\ue260", 4.3, 0], ["\ue1d5", 5.4, 0]],
    h: [1.6, 4.3, -5] },
  accidentalToNote: { g: [["\ue260", 1.6, 0], ["\ue1d5", 3.2, 0]], h: [2.5, 3.2, -5] },
  betweenAccidentals: { g: [["\ue260", 1, 0], ["\ue260", 2.4, 0], ["\ue1d5", 4, 0]],
    h: [1.9, 2.4, -5] },

  // Augmentation dots.
  dotToNote: { g: [["\ue1d5", 1, 0], ["\ue1e7", 3, 1]], h: [2.1, 3, -5] },
  betweenDots: { g: [["\ue1d5", 1, 0], ["\ue1e7", 2.6, 1], ["\ue1e7", 3.5, 1]],
    h: [2.9, 3.5, -5] },

  // Clefs.
  clefBefore: { g: [["\ue050", 2.2, -2]], h: [0.4, 2.2, -6] },
  clefAfter: { g: [["\ue050", 0.6, -2], ["\ue1d5", 4.4, 0]], h: [2.9, 4.4, -6] },

  // Key signatures.
  betweenKeyAccidentals: { g: [["\ue050", 0.6, -2], ["\ue260", 3.1, 4], ["\ue260", 4.2, 1]],
    h: [3.5, 4.2, -6] },
  afterKeyCancel: { g: [["\ue261", 0.6, 4], ["\ue261", 1.5, 1], ["\ue262", 3.6, 4],
    ["\ue262", 4.6, 1]], h: [2.2, 3.6, -6] },
  afterKeySignature: { g: [["\ue260", 0.6, 4], ["\ue260", 1.7, 1], ["\ue1d5", 4.4, 0]],
    h: [2.4, 4.4, -6] },

  // Time signatures.
  afterTimeSignature: { g: [["\ue084", 0.8, 2], ["\ue084", 0.8, -2], ["\ue1d5", 4.2, 0]],
    h: [2.2, 4.2, -6] },

  /* Barlines. A thickness is shown by drawing the thing itself in the
   * colour the field changes, beside one drawn normally — which is how
   * gscore 0.0.9 showed them, and it reads at a glance where a pair of
   * calipers half a staff space tall does not. */
  thinBarlineThickness: { l: [[2, 2, 2, -2, 0.16], [3.4, 2, 3.4, -2, 0.16, 1]] },
  thickBarlineThickness: { l: [[2, 2, 2, -2, 0.16], [3.6, 2, 3.6, -2, 0.5, 1]] },
  barlineSeparation: { l: [[1.8, 2, 1.8, -2, 0.16], [3, 2, 3, -2, 0.16]],
    h: [1.8, 3, -5] },
  afterBarline: { l: [[1, 2, 1, -2, 0.16]], g: [["\ue1d5", 3.2, 0]], h: [1, 3.2, -5] },
  betweenMeasures: { l: [[1, 2, 1, -2, 0.16], [5, 2, 5, -2, 0.16]],
    g: [["\ue1d5", 1.5, 0]], h: [2.7, 5, -5] },

  // Beams, drawn rather than set: the font carries beams only as part of
  // whole notes and what is shown here is the beam alone.
  beamThickness: { l: [[2, 2, 2, -1, 0.12], [5, 2, 5, -1, 0.12],
    [1.94, 2, 5.06, 2, 0.5, 1]] },
  beamSpacing: { l: [[2, 2.6, 2, -1, 0.12], [5, 2.6, 5, -1, 0.12],
    [1.94, 2.6, 5.06, 2.6, 0.5], [1.94, 1.6, 5.06, 1.6, 0.5]],
    v: [6, 2.35, 1.85] },

  // Stems and ledger lines.
  stemThickness: { g: [["\ue1d5", 1.6, -2]], l: [[2.76, -2, 2.76, 5, 0.12, 1]] },
  legerLineThickness: { g: [["\ue0a4", 2, 6]], l: [[1.6, 6, 3.6, 6, 0.16, 1]] },
  legerLineExtension: { g: [["\ue0a4", 2, 6]], l: [[1.6, 6, 3.6, 6, 0.16]],
    h: [3.18, 3.6, 7.4] },

  // Slurs and ties, whose weight is measured at the middle of the curve.
  slurMidpointThickness: { g: [["\ue1d5", 1, -4], ["\ue1d5", 5, -4]],
    c: [[1.6, -1, 5.6, -1, 3.6, 4, 1]] },
  tieMidpointThickness: { g: [["\ue0a3", 1, -2], ["\ue0a3", 4, -2]],
    c: [[1.6, 1, 4.6, 1, 3.1, 4, 1]] },

  // The lines that are not notes.
  staffLineThickness: { redLine: 0 },
  hairpinThickness: { l: [[1, -6, 5.4, -7, 0.16, 1], [1, -6, 5.4, -5, 0.16, 1]] },
  octaveLineThickness: { g: [["\ue511", 0.6, 6.4]],
    l: [[2.7, 5.6, 6.2, 5.6, 0.16, 1]] },
  tupletBracketThickness: { l: [[1.4, 5, 1.4, 4, 0.16, 1],
    [1.4, 5, 5.4, 5, 0.16, 1], [5.4, 5, 5.4, 4, 0.16, 1]],
    g: [["\ue083", 2.9, 6.6]] },

  // The page. Two staves, and the room between or around them. Drawn
  // closer together than they really stand, because the picture has to
  // fit beside a number: what it says is which gap, not how big.
  staffGap: { staff2: -12, h: 17, z: 4, v: [3, -4, -8] },
  systemGap: { staff2: -14, h: 19, z: 4, v: [3, -4, -10], divider: true },
  margin: { page: true, h: [0.4, 2.2, 0] },
  marginY: { page: true, v: [4.6, 5.6, 4.2] },
};

// knobPicture draws one of those descriptions.
//
// In staff spaces throughout, scaled once at the end, so the numbers above
// read as music — "the flat stands a space and a half to the left of the
// note" — rather than as pixels.
function knobPicture(spec) {
  if (!spec) return "";
  // Pixels to a staff space. The two-staff pictures ask for less: a
  // picture of two staves and the gap between them is three times the
  // height of a picture of one, and a row of this list cannot be.
  const z = spec.z || 7;
  const w = 9.6;
  const h = spec.h && !Array.isArray(spec.h) ? spec.h : 8;
  // The staff sits in the middle, which leaves about four spaces above it
  // and four below for what is written there — a stem, a ledger line, an
  // octave line, a hairpin.
  const sy = (step) => h / 2 - step / 2;
  // The colour goes on as a class, not as an attribute.
  //
  // stroke="var(--mark)" does not work: a presentation attribute is not a
  // CSS declaration, and a custom property in one is not resolved — the
  // attribute is thrown away and the shape falls back to whatever it
  // inherits. In dark mode that left the beams and the measure marks
  // drawn in the light theme's ink on a dark ground. currentColor is a
  // real attribute keyword and does work, which is why the staff lines
  // were right and nothing else was.
  const mark = (red) => (red ? ' class="mark"' : '');
  const out = [];

  const staffAt = (at, redLine) => {
    for (let i = -4; i <= 4; i += 2) {
      const on = redLine !== undefined && i === redLine;
      out.push(`<line${mark(on)} x1="0.3" y1="${sy(at + i)}" x2="${w - 0.3}" ` +
        `y2="${sy(at + i)}" stroke="currentColor" ` +
        `stroke-width="${on ? 0.22 : 0.1}" opacity="${on ? 1 : 0.55}"/>`);
    }
  };
  if (spec.page) {
    out.push(`<rect x="0.4" y="0.6" width="${w - 0.8}" height="${h - 1.2}" ` +
      `fill="none" stroke="currentColor" stroke-width="0.12" opacity=".55"/>`);
    for (let i = 0; i < 3; i++) {
      out.push(`<line x1="2.2" y1="${2 + i * 0.9}" x2="${w - 2.2}" y2="${2 + i * 0.9}" ` +
        `stroke="currentColor" stroke-width="0.1" opacity=".45"/>`);
    }
  } else {
    staffAt(0, spec.redLine);
    if (spec.staff2 !== undefined) staffAt(spec.staff2);
    // The line between two systems, so that the gap above it reads as the
    // gap between systems and not as another pair of staves.
    if (spec.divider) {
      // Midway between the first staff's bottom line and the second's
      // top line, which in steps is simply half of where the second
      // staff's middle is.
      const at = sy(spec.staff2 / 2);
      out.push(`<line x1="0.3" y1="${at}" x2="1.5" y2="${at}" ` +
        `stroke="currentColor" stroke-width="0.12" opacity=".55"/>`);
    }
  }

  for (const [x0, s0, x1, s1, t, red] of spec.l || []) {
    out.push(`<line${mark(red)} x1="${x0}" y1="${sy(s0)}" x2="${x1}" ` +
      `y2="${sy(s1)}" stroke="currentColor" stroke-width="${t}" ` +
      `stroke-linecap="butt"/>`);
  }
  for (const [x0, s0, x1, s1, cx, cs, red] of spec.c || []) {
    out.push(`<path${mark(red)} d="M${x0} ${sy(s0)} Q${cx} ${sy(cs)} ` +
      `${x1} ${sy(s1)}" fill="none" stroke="currentColor" stroke-width="0.16"/>`);
  }
  for (const [ch, x, step, red] of spec.g || []) {
    out.push(`<text${mark(red)} x="${x}" y="${sy(step)}" font-family="Bravura" ` +
      `font-size="4" fill="currentColor">${ch}</text>`);
  }

  // The red mark for a distance. End ticks and not an arrowhead: at this
  // size an arrow is a blob, and two ticks say "from here to here" just as
  // plainly.
  const red = 'class="mark" stroke="currentColor" stroke-width="0.14" ' +
    'stroke-linecap="butt"';
  const across = (x0, x1, step) => {
    const y = sy(step);
    out.push(`<line x1="${x0}" y1="${y}" x2="${x1}" y2="${y}" ${red}/>`);
    out.push(`<line x1="${x0}" y1="${y - 0.35}" x2="${x0}" y2="${y + 0.35}" ${red}/>`);
    out.push(`<line x1="${x1}" y1="${y - 0.35}" x2="${x1}" y2="${y + 0.35}" ${red}/>`);
  };
  if (Array.isArray(spec.h)) across(...spec.h);
  if (spec.h2) across(...spec.h2);
  if (spec.v) {
    const [x, s0, s1] = spec.v, y0 = sy(s0), y1 = sy(s1);
    out.push(`<line x1="${x}" y1="${y0}" x2="${x}" y2="${y1}" ${red}/>`);
    out.push(`<line x1="${x - 0.35}" y1="${y0}" x2="${x + 0.35}" y2="${y0}" ${red}/>`);
    out.push(`<line x1="${x - 0.35}" y1="${y1}" x2="${x + 0.35}" y2="${y1}" ${red}/>`);
  }

  return `<svg class="knob-pic" viewBox="0 0 ${w} ${h}" width="${w * z}" ` +
    `height="${h * z}" aria-hidden="true">${out.join("")}</svg>`;
}

/* ---------- the flute's fingerings ---------- */

// fluteDialog is the whole table of them, to be looked through.
//
// Two questions, which are different questions. "How else can I play this
// note" is asked of one note and answered by its alternates, nearest the
// standard chart first. "How do I trill these two" is asked of a pair and
// answered by the fingerings that differ by the one key that shakes —
// which is what a trill is, and why the trill chart in a method book is
// not derivable from the fingering chart beside it.
//
// Nothing about the instrument is written here. The keys, their names,
// which hand they belong to and what notes there are all come from the
// endpoint, so the picture and the table cannot drift apart.
async function fluteDialog() {
  const dlg = $("#flute");
  const list = $("#flute-list");
  const count = $("#flute-count");

  let kit = null;              // the instrument: keys, left, notes
  let note = "D5";             // the note being looked at
  let to = null;               // the other note, for a trill
  let trills = false;
  let shown = 0;               // how many of the answer are on the page
  const page = 24;

  const ask = async (extra) => {
    const p = new URLSearchParams(extra || {});
    const res = await fetch("/api/flute?" + p.toString());
    if (!res.ok) {
      status("Could not read the fingerings: " + (await res.text()).trim());
      return null;
    }
    return res.json();
  };

  // The note buttons, which are the whole range of the instrument.
  const drawNotes = () => {
    const box = $("#flute-notes");
    box.innerHTML = "";
    for (const n of kit.notes) {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = n.replace("#", "♯");
      b.className = n === note ? "on" : "";
      b.onclick = () => {
        note = n;
        if (trills) to = nextNote(n);
        drawNotes();
        fillTo();
        void show(true);
      };
      box.appendChild(b);
    }
  };

  // The note a trill goes to, which is nearly always the one above.
  const nextNote = (n) => {
    const i = kit.notes.indexOf(n);
    return kit.notes[Math.min(i + 1, kit.notes.length - 1)];
  };

  const fillTo = () => {
    const pick = $("#flute-to");
    pick.innerHTML = "";
    for (const n of kit.notes) {
      if (n === note) continue;
      const o = new Option(n.replace("#", "♯"), n);
      pick.add(o);
    }
    pick.value = to || nextNote(note);
    to = pick.value;
  };

  // The keys a fingering can be asked to hold, by the names the table
  // uses: the list is the instrument's, not a second copy of it.
  const fillUsing = () => {
    const pick = $("#flute-using");
    pick.innerHTML = "";
    pick.add(new Option(t("any key"), ""));
    const seen = new Set();
    kit.keys.forEach((k, i) => {
      const name = (i < kit.left ? "left " : "right ") + k;
      if (seen.has(k)) return;
      seen.add(k);
      pick.add(new Option(name, k));
    });
  };

  const card = (html, caption) => {
    const box = document.createElement("figure");
    box.className = "fl-card";
    box.innerHTML = html + "<figcaption>" + caption + "</figcaption>";
    return box;
  };

  const show = async (fresh) => {
    if (fresh) {
      list.innerHTML = "";
      shown = 0;
    }
    const d = trills
      ? await ask({ from: note, to, limit: String(page), offset: String(shown) })
      : await ask({
          note,
          away: $("#flute-away").value,
          within: $("#flute-within").value,
          using: $("#flute-using").value,
          limit: String(page), offset: String(shown),
        });
    if (!d) return;

    if (fresh && !trills) {
      // The standard chart first and on its own: it is the answer to the
      // question nine times in ten, and burying it in five thousand
      // alternates would be hiding it.
      const std = d.standard;
      const box = card(
        fluteChart(std.keys, kit.keys, kit.left),
        `<b>${note.replace("#", "♯")}</b> — the standard fingering, ` +
        fluteTuning(std.cents));
      box.classList.add("fl-standard");
      list.appendChild(box);
    }

    const rows = trills ? d.trills : d.shown;
    for (const row of rows || []) {
      if (trills) {
        const side = row.hold.spell.split("|")[0].includes(row.key) ? "L" : "R";
        box(list, card(
          fluteChart(row.hold.keys, kit.keys, kit.left,
                     { shake: { id: row.key, side } }),
          `shake <b>${row.key}</b> ${row.down ? "down" : "off"} — ` +
          `${note.replace("#", "♯")} ${fluteTuning(row.hold.cents)}, ` +
          `${to.replace("#", "♯")} ${fluteTuning(row.upper)}` +
          (row.hold.away ? `, ${row.hold.away} from the chart` : ", on the chart") +
          (fluteNeeds(row.hold.keys, kit.keys).length ? " — needs <b>split E</b>" : "")));
      } else {
        const needs = fluteNeeds(row.keys, kit.keys);
        box(list, card(
          fluteChart(row.keys, kit.keys, kit.left),
          `${fluteTuning(row.cents)} — ` +
          (row.away === 1 ? "1 key" : row.away + " keys") + " from the chart" +
          (needs.length ? ` — needs <b>${needs.join(", ")}</b>` : "")));
      }
    }
    shown += (rows || []).length;
    count.textContent = trills
      ? `${d.total} ways of trilling ${note.replace("#", "♯")} to ` +
        `${to.replace("#", "♯")} — showing ${shown}`
      : `${d.total} other fingerings for ${note.replace("#", "♯")}` +
        (d.total ? ` — showing ${shown}` : "");
    $("#flute-more").hidden = shown >= d.total;
  };

  const box = (into, el) => into.appendChild(el);

  if (!kit) {
    kit = await ask({});
    if (!kit) return;
  }
  if (!kit.notes.includes(note)) note = kit.notes[0];
  to = nextNote(note);
  drawNotes();
  fillTo();
  fillUsing();

  const mode = (onTrills) => {
    trills = onTrills;
    $("#flute-mode-note").classList.toggle("on", !onTrills);
    $("#flute-mode-trill").classList.toggle("on", onTrills);
    $("#flute-trill-to").hidden = !onTrills;
    $("#flute-filters").hidden = onTrills;
    void show(true);
  };
  $("#flute-mode-note").onclick = () => mode(false);
  $("#flute-mode-trill").onclick = () => mode(true);
  $("#flute-to").onchange = () => { to = $("#flute-to").value; void show(true); };
  for (const id of ["#flute-away", "#flute-within", "#flute-using"]) {
    $(id).onchange = () => void show(true);
  }
  $("#flute-more").onclick = () => void show(false);
  $("#flute-done").onclick = () => dlg.close();

  await show(true);
  dlg.showModal();
}

/* ---------- how this score is engraved ---------- */

// engravingDialog shows the measurements the engraver works from, and lets
// this score set its own.
//
// Nothing about them is written here. The endpoint hands over the
// engraver's own list — name, label, the heading it belongs under, what it
// is allowed to be — and the window is built from that, so a measurement
// added to the engraver appears here without this function being touched.
// A dialog with fifty fields written out by hand is a dialog that is wrong
// a week after it is written.
//
// Each change is applied as it is made rather than on a button, because
// what these do cannot be judged from the number. Nobody knows what a beam
// of 0.55 looks like; they know whether the page in front of them looks
// right. The score is redrawn behind the open window after every change,
// which is the whole point of doing it this way.
// knobTitle is what hovering a measurement says: what it is now, what the
// house style is, and the range most scores are in.
//
// The range is advice and says so. It used to be the range the field would
// take, which made it a rule — and a rule about somebody's house style is
// not the program's to have.
function knobTitle(k) {
  const unit = k.unit ? " " + k.unit : "";
  const usual = `Most scores are between ${k.min} and ${k.max}${unit}, ` +
    "but you can set what you like.";
  return k.set
    ? `${t(k.label)}: ${k.value}${unit} — the house style is ${k.default}. ` +
      `Empty the field to go back to it. ${usual}`
    : `${t(k.label)}: the house style, ${k.default}${unit}. ${usual}`;
}

async function engravingDialog() {
  const dlg = $("#engraving");
  const cats = $("#engraving-cats");
  const rows = $("#engraving-rows");
  // Which heading is open. Kept across a redraw so that changing a number
  // does not throw you back to the first page of the window.
  let open = null;

  // The fields on the screen, by the measurement each one sets.
  //
  // Kept so that a change can be shown without building the window again.
  // Building it again was the bug this exists to prevent: a number field
  // that is replaced while somebody is clicking its arrows is an arrow that
  // answers once and then does nothing, because the thing under their
  // finger has gone. It looked like a measurement that would not go past
  // 0.91.
  const fields = new Map();

  // What this score was engraved by when the window opened, and what it is
  // engraved by now.
  //
  // Every change in here is applied as it is made, which it has to be:
  // nobody can judge a beam of 0.55 from the number, only from the page.
  // But a window that changes the score as you go still needs a way of
  // saying none of it was meant, and that is what these two are for —
  // Cancel is the difference between them, sent back the other way.
  //
  // opened maps each measurement's name to the value this score had set,
  // or null for "nothing of its own". null and a number are told apart on
  // purpose: a measurement that was inherited has to go back to being
  // inherited, not be frozen at the number it happened to inherit.
  let opened = null;
  let latest = [];
  // The change still being sent, and whether we have given up on this
  // visit.
  //
  // Both are for one gesture: typing a number and clicking Cancel without
  // stopping in between. The field commits what was typed as it loses the
  // focus, so a change is in the air at the moment Cancel is clicked — and
  // if Cancel does not wait for it, it works out what to put back from
  // before the number landed, and the number nobody wanted is what
  // survives. Afterwards the other way round: a change that arrives once
  // the window has been given up on is dropped, rather than writing itself
  // over what was just put back.
  let sending = Promise.resolve();
  let giveUp = false;
  // Whose measurements are on the screen, for the questions that have to
  // name them. The window writes to whatever is being looked at.
  let whose = "this score";

  const snapshot = (knobs) => {
    const m = new Map();
    for (const k of knobs) m.set(k.name, k.set ? k.value : null);
    return m;
  };

  const draw = (knobs) => {
    fields.clear();
    const groups = [];
    for (const k of knobs) {
      let g = groups.find((x) => x.name === k.group);
      if (!g) groups.push(g = { name: k.group, rows: [] });
      g.rows.push(k);
    }
    if (!groups.some((g) => g.name === open)) open = groups[0]?.name ?? null;

    cats.innerHTML = "";
    for (const g of groups) {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = t(g.name);
      // The heading's own name, kept where a later change can find it: the
      // dot that says "something is set under here" is drawn by the
      // stylesheet, so reading it back off the text would not work.
      b.dataset.group = g.name;
      b.className = g.name === open ? "on" : "";
      // A heading with something set under it says so, so that a score's
      // own choices can be found without opening every page in turn.
      if (g.rows.some((k) => k.set)) b.classList.add("has-set");
      b.onclick = () => { open = g.name; draw(knobs); };
      cats.appendChild(b);
    }

    rows.innerHTML = "";
    for (const k of (groups.find((g) => g.name === open)?.rows) || []) {
      const row = document.createElement("div");
      row.className = "knob";

      const name = document.createElement("label");
      name.htmlFor = "engraving-" + k.name;
      name.textContent = t(k.label);
      row.appendChild(name);

      const field = document.createElement("input");
      field.type = "number";
      field.id = "engraving-" + k.name;
      // What the number may be, which is what can be drawn — not what is
      // usual. A field that will not take the number somebody wants is a
      // window telling them their taste is wrong, which is not its place:
      // the usual range is in the tooltip, as advice, and the arrows step
      // through it.
      field.min = String(k.floor ?? k.min);
      field.max = String(k.ceiling ?? k.max);
      // A hundredth of a staff space: the difference between a beam of
      // 0.5 and one of 0.55 is one somebody can see.
      field.step = "0.01";
      field.value = String(k.value);
      // What it would be if this score said nothing, so a field that has
      // been changed says so. Without it there is no telling whether 2.8
      // was chosen or merely inherited, and a field like that is one
      // nobody dares touch.
      field.placeholder = String(k.default);
      if (k.set) field.classList.add("is-set");
      field.title = knobTitle(k);
      field.addEventListener("change", () => {
        sending = set(k.name, field.value);
      });
      row.appendChild(field);
      fields.set(k.name, field);

      const pic = document.createElement("i");
      pic.className = "knob-picture";
      pic.innerHTML = knobPicture(KNOB_PICTURES[k.name]);
      row.appendChild(pic);

      rows.appendChild(row);
    }
  };

  const said = (d) => {
    const knobs = d.knobs || [];
    const n = knobs.filter((k) => k.set).length;
    whose = d.of ? `the ${d.of} part` : "this score";
    // Whose measurements these are. The window writes to whatever is on
    // the screen, so a part being looked at is a part being engraved, and
    // saying "this score" over a part's numbers would be a lie.
    const line = $("#engraving-whose");
    if (line && d.of) {
      line.textContent = t("What this part is engraved by, in staff spaces. Its own, over the score's: empty a field to go back to what the score says.");
    }
    status(n
      ? `Engraving — ${n} ${n === 1 ? "measurement" : "measurements"} set for ${whose}`
      : `Engraving — ${whose} is on the house style`);
  };

  const show = (d) => {
    latest = d.knobs || [];
    // Only the first time the window is filled in, so that resetting
    // everything inside it is itself something Cancel can take back.
    if (!opened) opened = snapshot(latest);
    draw(latest);
    said(d);
  };

  // patch shows what changed without building the window again: which
  // headings have something set under them, which fields say so, and what
  // each one now holds.
  const patch = (d) => {
    latest = d.knobs || [];
    for (const k of d.knobs || []) {
      const field = fields.get(k.name);
      if (!field) continue;
      field.classList.toggle("is-set", !!k.set);
      field.title = knobTitle(k);
      // Only when it really differs, so that a field somebody is still
      // typing in is not rewritten underneath them — and when it does
      // differ it is written, because a number the engraver would not take
      // has to be seen rather than left sitting there looking as though it
      // took.
      if (Number(field.value) !== k.value) field.value = String(k.value);
    }
    const under = {};
    for (const k of d.knobs || []) {
      under[k.group] = under[k.group] || k.set;
    }
    for (const b of cats.querySelectorAll("button")) {
      b.classList.toggle("has-set", !!under[b.dataset.group]);
    }
    said(d);
  };

  const set = async (name, value) => {
    if (giveUp) return;
    const p = new URLSearchParams();
    p.set(name, value);
    const res = await fetch("/api/engraving?" + p.toString(), { method: "POST" });
    if (!res.ok) {
      status("Could not set it: " + (await res.text()).trim());
      // Back to what it really is, rather than leaving a refused number
      // sitting in the field looking as though it took.
      await load();
      return;
    }
    patch(await res.json());
    await refreshScore();
  };

  const load = async (how) => {
    const res = await fetch("/api/engraving" + (how || ""), how ? { method: "POST" } : {});
    if (!res.ok) {
      status("Could not read the engraving settings");
      return;
    }
    show(await res.json());
  };

  // restore puts back what was there when the window opened.
  //
  // Everything that differs in one request, because the endpoint reads the
  // lot before writing any of it: a measurement at a time would redraw the
  // page once per measurement and could stop half way, leaving a score
  // that is neither what it was nor what was asked for. An empty value
  // means "have no answer of your own for this one", which is how a
  // measurement set inside this window is unset again.
  const restore = async () => {
    if (!opened) return;
    // Whatever was typed on the way out, first, so that what is put back is
    // worked out from everything that happened and not from most of it.
    await sending.catch(() => {});
    giveUp = true;
    const p = new URLSearchParams();
    for (const k of latest) {
      const was = opened.has(k.name) ? opened.get(k.name) : null;
      const is = k.set ? k.value : null;
      if (was === is) continue;
      p.set(k.name, was === null ? "" : String(was));
    }
    const changed = [...p.keys()].length;
    if (!changed) return;
    const res = await fetch("/api/engraving?" + p.toString(),
      { method: "POST" });
    if (!res.ok) {
      status("Could not put the engraving back: " + (await res.text()).trim());
      return;
    }
    latest = (await res.json()).knobs || [];
    await refreshScore();
    status(`Engraving — left as it was, ${changed} ` +
      (changed === 1 ? "measurement" : "measurements") + " put back");
  };

  $("#engraving-reset").onclick = async () => {
    const n = latest.filter((k) => k.set).length;
    // Nothing set is nothing to lose, and a question whose answer changes
    // nothing is a question not worth asking.
    if (n) {
      const sure = await confirmThat({
        title: "This will remove all the specific settings you used — " +
               "are you sure?",
        note: (n === 1
          ? `The one measurement set for ${whose} goes back to the house style.`
          : `The ${n} measurements set for ${whose} go back to the house style.`) +
          " Cancel still puts everything back the way it was when this " +
          "window opened.",
        ok: "Reset all",
      });
      if (!sure) return;
    }
    await load("?reset");
    await refreshScore();
  };

  // Esc is Cancel rather than Done. A window with a Cancel button in it
  // that keeps the changes when it is dismissed is a window that lies, and
  // dismissing one is how most people say they did not mean it.
  let keeping = false;
  const closed = async () => {
    dlg.removeEventListener("close", closed);
    if (!keeping) await restore();
  };
  dlg.addEventListener("close", closed);
  $("#engraving-cancel").onclick = () => dlg.close();
  $("#engraving-done").onclick = () => { keeping = true; dlg.close(); };

  await load();
  dlg.showModal();
}

/* ---------- transposing ---------- */

// intervalNames are what each interval is called, for saying afterwards
// what was done. Keyed by the two numbers the interval actually is.
const intervalNames = {
  "1,1": "a semitone", "1,2": "a tone",
  "2,3": "a minor third", "2,4": "a major third",
  "3,5": "a fourth", "3,6": "an augmented fourth",
  "4,6": "a diminished fifth", "4,7": "a fifth",
  "5,8": "a minor sixth", "5,9": "a major sixth",
  "6,10": "a minor seventh", "6,11": "a major seventh",
  "7,12": "an octave",
};

// transposeSelection is what the music being moved is: the notes picked up,
// or the whole piece where nothing is.
//
// The same rule the dialog shows and the two one-key commands follow, so
// that "transpose up" means the same thing however it was asked for.
function transposeSelection() {
  const refs = pickedRefs();
  return refs.length ? refs.join(",") : "";
}

// pickedRefs is every note picked up, as the server names them.
function pickedRefs() {
  const out = [];
  for (const m of state.multi) out.push(`${m.part}:${m.measure}:${m.index}`);
  if (out.length) return out;
  const sel = state.selection;
  if (sel && sel.ref !== null && sel.ref !== undefined && !sel.bar &&
      sel.at > 0 && pickedNotes()) {
    out.push(`${sel.part}:${sel.measure}:${sel.at - 1}`);
  }
  return out;
}

// transposeBy moves the music and says what it did.
async function transposeBy(steps, semitones, opt) {
  opt = opt || {};
  const sel = transposeSelection();
  const p = new URLSearchParams({ semitones: String(semitones) });
  // Left out rather than sent as zero where the caller did not say.
  //
  // Zero letters is a real interval — an augmented unison, C to C sharp —
  // so the two cannot be the same request. Saying nothing asks for the
  // ordinary reading of the distance, which is worked out where the move
  // is made.
  if (steps !== null && steps !== undefined) p.set("steps", String(steps));
  if (sel) p.set("sel", sel);
  if (opt.diatonic) p.set("mode", "diatonic");
  // The signature only ever moves when the whole piece does; the server
  // holds to that as well, and says so here so the two cannot drift.
  p.set("key", !sel && opt.key !== false && !opt.diatonic ? "1" : "0");

  const res = await fetch("/api/transpose?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not transpose: " + (await res.text()).trim());
    return false;
  }
  const d = await res.json().catch(() => null);
  await refreshScore();
  markSelection();

  const name = (steps === null || steps === undefined
    ? null
    : intervalNames[`${Math.abs(steps)},${Math.abs(semitones)}`]) ||
    `${Math.abs(semitones)} semitones`;
  const way = (semitones || steps) > 0 ? "up" : "down";
  const what = sel ? "The selection" : "The piece";
  let said = `${what} moved ${way} ${name}`;
  if (d && d.fifths !== undefined) said += `, now in ${keyNamed(d.fifths)}`;
  // A drum staff is instruments rather than pitches and a tab staff is
  // fingers rather than pitches, so neither moved. Said rather than done
  // quietly: a part that did not move is the thing somebody would notice
  // last and mind most.
  if (d && d.left) {
    said += ` — ${d.left} ${d.left === 1 ? "staff" : "staves"} left alone ` +
      `(drums and tablature are not pitches)`;
  }
  status(said);
  return true;
}

// keyNamed is what a signature is called.
//
// The names are fetched by the key dialog and kept; before it has ever
// been opened there is nothing to name a key with, and the count of sharps
// or flats says it plainly enough to be going on with.
function keyNamed(fifths) {
  const k = (keyOptions || []).find((o) => o.fifths === fifths);
  if (k) return k.major + " major";
  if (fifths === 0) return "C major";
  const n = Math.abs(fifths);
  return `${n} ${fifths > 0 ? "sharp" : "flat"}${n === 1 ? "" : "s"}`;
}

// transposeDialog asks how far, and which way.
async function transposeDialog() {
  const dlg = $("#transpose");
  const sel = transposeSelection();
  const n = sel ? sel.split(",").length : 0;
  $("#transpose-where").textContent = sel
    ? `${n} ${n === 1 ? "note" : "notes"} picked up. The key signature stays as it is.`
    : "The whole piece, every staff of it.";
  // Only a whole piece changes key: a phrase moved inside a piece is still
  // in the key it was in, and is written with accidentals.
  $("#transpose-key-row").hidden = !!sel;

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  if (answer !== "ok") {
    status("Ready");
    return;
  }

  const picked = $("#transpose-interval").value.split(",");
  let steps = Number(picked[0]), semitones = Number(picked[1]);
  const down = $$('#transpose-form input[name="transpose-dir"]')
    .find((r) => r.checked)?.value === "down";
  if (down) {
    steps = -steps;
    semitones = -semitones;
  }
  const diatonic = $$('#transpose-form input[name="transpose-mode"]')
    .find((r) => r.checked)?.value === "diatonic";
  await transposeBy(steps, semitones, {
    diatonic, key: $("#transpose-key").checked,
  });
}

async function setTempoDialog() {
  const dlg = $("#tempo");

  // Where the marking lands, shown and changeable rather than taken from
  // whatever happened to be selected. Asking for a change of speed with
  // nothing selected used to rewrite the mark at the top of the score and
  // say nothing about it, so the bar somebody wanted it in stayed as it was.
  const measure = $("#tempo-measure");
  measure.value = String((state.selection?.measure ?? 0) + 1);
  measure.max = String(
    (state.hits || []).reduce((n, h) => Math.max(n, h.measure + 1), 1));

  const barNow = () => Math.max(1, parseInt(measure.value, 10) || 1);

  // What the rest of the dialog shows follows the bar, so changing the bar
  // shows what is written there rather than what was written where it was.
  const describe = () => {
    const at = barNow();
    const here = tempoAtBar(at - 1);
    $("#tempo-where").textContent = at === 1
      ? "The speed the piece starts at."
      : `A change of speed from bar ${at} on.`;
    $("#tempo-bpm").value = String(here.bpm);
    $("#tempo-text").value = here.text;
    $("#tempo-dotted").checked = !!here.dotted;
    $("#tempo-parens").checked = !!here.parens;
    $$('#tempo-notes input[name="tempo-unit"]').forEach(
      (r) => (r.checked = r.value === here.unit),
    );
    // There is nothing to remove at bar one: the piece has a speed whether
    // or not anybody chose it.
    $("#tempo-remove").hidden = at === 1 || !here.written;
  };
  measure.oninput = describe;
  describe();

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    $("#tempo-bpm").focus();
    $("#tempo-bpm").select();
  });
  if (answer !== "ok" && answer !== "remove") return;

  const bar = barNow() - 1;
  const q = new URLSearchParams();
  if (bar > 0) q.set("measure", String(bar));

  if (answer === "remove") {
    q.set("remove", "1");
  } else {
    const bpm = parseInt($("#tempo-bpm").value, 10);
    if (!Number.isFinite(bpm) || bpm < 10 || bpm > 400) {
      status("A tempo is between 10 and 400 beats a minute");
      return;
    }
    const unit = $$('#tempo-notes input[name="tempo-unit"]').find((r) => r.checked);
    q.set("bpm", String(bpm));
    q.set("unit", unit ? unit.value : "quarter");
    if ($("#tempo-dotted").checked) q.set("dotted", "1");
    if ($("#tempo-parens").checked) q.set("parens", "1");
    q.set("text", $("#tempo-text").value.trim());
  }

  const res = await fetch("/api/tempo?" + q.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set the tempo: " + (await res.text()).trim());
    return;
  }
  const d = await res.json().catch(() => ({}));
  await refreshScore();
  if (answer === "remove") {
    status(`Bar ${bar + 1} carries on at the tempo before it`);
    return;
  }
  status(bar === 0 ? `${d.bpm ?? ""} bpm` : `Bar ${bar + 1}: ${d.bpm ?? ""} bpm`);
}

// tempoAtBar is what the dialog should open showing: the marking written at
// this bar if there is one, and otherwise the one in force there, so that
// changing the number does not silently change the beat as well.
function tempoAtBar(bar) {
  const changes = state.info?.tempoChanges || {};
  const written = changes[String(bar)];
  if (written) return { ...written, written: true };
  if (bar === 0) {
    return {
      bpm: state.info?.tempo || 120,
      unit: state.info?.tempoUnit || "quarter",
      dotted: !!state.info?.tempoDotted,
      text: state.info?.tempoText || "",
      parens: !!state.info?.tempoParens,
      written: true,
    };
  }
  // Walk back to whatever is in force here.
  let best = {
    bpm: state.info?.tempo || 120,
    unit: state.info?.tempoUnit || "quarter",
    dotted: !!state.info?.tempoDotted,
    text: state.info?.tempoText || "",
    parens: !!state.info?.tempoParens,
  };
  for (let i = 1; i <= bar; i++) {
    const c = changes[String(i)];
    if (c) best = c;
  }
  return { ...best, text: "", written: false };
}

/* ---------- the header's rich text ---------- */

/*
 * A title and a credit are set, not just typed.
 *
 * "Music by" plain and the name bold after it; an arranger's line in grey
 * under a composer's in black; a subtitle smaller than the rest of the
 * subtitle. None of that is decoration — it is how a printed score says who
 * did what — and a box that can only hold one face in one colour cannot
 * write it down.
 *
 * So each of these fields is a small editor. What it holds is kept as the
 * words and the runs they are set in: the words go on being the words, so
 * everything that reads a score — a file, an export, a shelf of pieces —
 * sees what it always saw, and the runs are extra.
 *
 * The runs are read back off the DOM by asking for the *computed* style of
 * each piece of text rather than by trusting the tags. That is what makes
 * pasted text work: whatever a browser or another program puts in the box,
 * what comes out is what it looks like.
 */

// richBoxes is each field's box, by the name the server knows it as.
const richBoxes = {};

// scoreAside takes the engraved music out of the page while a window with a
// formatted box in it is open, and puts it back when that window closes.
//
// A box you can type formatting into and a full score cannot be in one page
// together. While a caret sits in such a box, WebKit works out the word
// around it once for every frame it draws — it is for the system's own
// correction candidates — and it finds the beginning of that word by walking
// backwards from the caret through the page, out of the box and into
// everything that comes before it. The music comes before the dialogs, so
// that walk is over the whole score: 223,000 elements for a symphony, every
// frame. The window stops dead. Nothing is drawn, so the dialog never
// appears, and nothing answers afterwards either — which is exactly what
// opening Title and composer on a symphony did.
//
// Hiding the music is not enough: the walk is over the nodes and not over
// what is drawn, so nodes nobody can see are walked just the same. They have
// to leave the page. They are kept here, in order, and go back untouched —
// taking a symphony out costs 80ms and putting it back 260ms, against a
// window that never came back at all.
//
// Nothing is lost by it. A modal window covers the music and there is
// nothing to be done to the score while one is open.
function scoreAside(dlg) {
  const canvas = $("#canvas");
  if (!dlg || !canvas || !canvas.firstChild) return;
  const wrap = $("#canvas-wrap");
  const kept = [...canvas.childNodes];
  const x = wrap ? wrap.scrollLeft : 0;
  const y = wrap ? wrap.scrollTop : 0;
  canvas.replaceChildren();
  const back = () => {
    dlg.removeEventListener("close", back);
    // Unless something has drawn in the meantime. Saving a new title lays
    // the score out again, and the music held here is the music from before
    // the change: putting it back over the new drawing would show the old
    // title until the next time anything moved.
    if (canvas.firstChild) return;
    canvas.replaceChildren(...kept);
    // The view was somewhere. An empty page has nothing to scroll, so the
    // place it was at is gone by now and is put back by hand.
    if (wrap) {
      wrap.scrollLeft = x;
      wrap.scrollTop = y;
    }
  };
  dlg.addEventListener("close", back);
}

// richFields builds the boxes, once, out of the placeholders in the markup.
function richFields() {
  for (const field of $$(".rich-field")) {
    const name = field.dataset.field;
    if (richBoxes[name]) continue;
    const id = "score-about-" + name;
    field.innerHTML =
      `<div class="rich-head"><label class="small" for="${id}">${field.dataset.label}</label>` +
      `<span class="rich-bar">` +
      `<button type="button" data-rich="bold" title="Bold"><b>B</b></button>` +
      `<button type="button" data-rich="italic" title="Italic"><i>I</i></button>` +
      `<select data-rich="size" title="Size" aria-label="Size">` +
      `<option value="1">Size</option>` +
      `<option value="0.75">Smaller</option>` +
      `<option value="1">Normal</option>` +
      `<option value="1.4">Larger</option>` +
      `<option value="2">Largest</option>` +
      `</select>` +
      `<input type="color" data-rich="colour" value="#000000" title="Colour" aria-label="Colour">` +
      `<button type="button" data-rich="plain" title="Take the formatting off">Plain</button>` +
      `</span></div>` +
      `<div class="rich-box" id="${id}" contenteditable="true" role="textbox" ` +
      `aria-multiline="true" aria-label="${field.dataset.label}"></div>`;
    const box = $("#" + id);
    richBoxes[name] = box;
    for (const b of field.querySelectorAll("[data-rich]")) {
      const what = b.dataset.rich;
      if (b.tagName === "BUTTON") {
        // mousedown, not click: by the time a click lands the box has
        // lost the selection to the button.
        b.addEventListener("mousedown", (e) => {
          e.preventDefault();
          richApply(box, what);
        });
      } else {
        b.addEventListener("input", () => richApply(box, what, b.value));
        b.addEventListener("change", () => richApply(box, what, b.value));
      }
    }
  }
}

// richApply puts one piece of formatting on whatever is picked out in a box.
//
// Nothing picked out is nothing to format: a face chosen with no selection
// would have to be remembered until the next thing typed, which is a mode,
// and a window with a mode in it is a window that surprises somebody.
function richApply(box, what, value) {
  const sel = window.getSelection();
  if (!sel || !sel.rangeCount) return;
  const range = sel.getRangeAt(0);
  if (range.collapsed || !box.contains(range.commonAncestorContainer)) {
    status("Pick out the words to change first");
    return;
  }
  if (what === "plain") {
    const text = range.toString();
    range.deleteContents();
    range.insertNode(document.createTextNode(text));
    richNormalise(box);
    return;
  }
  const at = range.startContainer.nodeType === 3
    ? range.startContainer.parentElement : range.startContainer;
  const now = getComputedStyle(at);
  const span = document.createElement("span");
  switch (what) {
    case "bold":
      span.style.fontWeight = richIsBold(now) ? "normal" : "bold";
      break;
    case "italic":
      span.style.fontStyle = now.fontStyle === "italic" ? "normal" : "italic";
      break;
    case "colour":
      span.style.color = value;
      break;
    case "size": {
      // The size asked for is a multiple of the box's own, and the span
      // goes inside whatever is already there — so what it is given is the
      // step from where this piece of text already is to where it should
      // be. Without that, setting the same size twice doubles it.
      const base = parseFloat(getComputedStyle(box).fontSize);
      const here = parseFloat(now.fontSize) / base;
      span.style.fontSize = (Number(value) / here) + "em";
      break;
    }
    default:
      return;
  }
  span.appendChild(range.extractContents());
  range.insertNode(span);
  sel.removeAllRanges();
  const kept = document.createRange();
  kept.selectNodeContents(span);
  sel.addRange(kept);
}

function richIsBold(style) {
  const w = style.fontWeight;
  return w === "bold" || w === "bolder" || parseInt(w, 10) >= 600;
}

// richNormalise writes a box out and reads it back, which flattens whatever
// nest of tags an edit or a paste left behind into the runs it looks like.
function richNormalise(box) {
  richSet(box, richRuns(box), "");
}

// richRuns is what is in a box: the runs of text with how each is set.
//
// Read from the computed style of each piece of text, so that what comes out
// is what the box looks like however it got that way — typed, pasted from a
// word processor, or built by richSet.
function richRuns(box) {
  const base = getComputedStyle(box);
  const baseSize = parseFloat(base.fontSize);
  const baseColour = base.color;
  const out = [];
  const add = (text, el) => {
    if (!text) return;
    const st = el ? getComputedStyle(el) : base;
    const run = { text: text };
    if (richIsBold(st)) run.bold = true;
    if (st.fontStyle === "italic") run.italic = true;
    if (st.color !== baseColour) run.colour = richHex(st.color);
    const size = parseFloat(st.fontSize) / baseSize;
    if (Math.abs(size - 1) > 0.01) run.size = Math.round(size * 100) / 100;
    const last = out[out.length - 1];
    if (last && !!last.bold === !!run.bold && !!last.italic === !!run.italic &&
        (last.colour || "") === (run.colour || "") &&
        (last.size || 1) === (run.size || 1)) {
      last.text += text;
      return;
    }
    out.push(run);
  };
  const walk = (node) => {
    for (const n of node.childNodes) {
      if (n.nodeType === 3) {
        add(n.nodeValue, n.parentElement);
      } else if (n.nodeName === "BR") {
        add("\n", n.parentElement);
      } else if (n.nodeType === 1) {
        // A block — which is what a browser leaves behind when Return is
        // pressed — begins a line of its own.
        if (out.length && getComputedStyle(n).display !== "inline") {
          add("\n", n);
        }
        walk(n);
      }
    }
  };
  walk(box);
  return out;
}

// richSet fills a box from runs, or from plain words where there are none.
function richSet(box, runs, text) {
  box.textContent = "";
  const list = (runs && runs.length) ? runs : (text ? [{ text: text }] : []);
  for (const r of list) {
    const parts = String(r.text).split("\n");
    parts.forEach((part, i) => {
      if (i) box.appendChild(document.createElement("br"));
      if (!part) return;
      const plain = !r.bold && !r.italic && !r.colour &&
        (!r.size || Math.abs(r.size - 1) < 0.01);
      if (plain) {
        box.appendChild(document.createTextNode(part));
        return;
      }
      const span = document.createElement("span");
      if (r.bold) span.style.fontWeight = "bold";
      if (r.italic) span.style.fontStyle = "italic";
      if (r.colour) span.style.color = r.colour;
      if (r.size && Math.abs(r.size - 1) >= 0.01) {
        span.style.fontSize = r.size + "em";
      }
      span.textContent = part;
      box.appendChild(span);
    });
  }
}

// richHex turns whatever colour a browser reports into the #rrggbb a file
// keeps and a colour picker shows.
function richHex(colour) {
  const m = /^rgba?\(([^)]+)\)$/.exec(colour);
  if (!m) return colour;
  const parts = m[1].split(",").map((v) => parseFloat(v));
  const hex = (v) => Math.max(0, Math.min(255, Math.round(v)))
    .toString(16).padStart(2, "0");
  return "#" + hex(parts[0]) + hex(parts[1]) + hex(parts[2]);
}

async function setHeaderDialog() {
  const dlg = $("#score-about");
  const i = state.info || {};
  const field = (id, v) => { $("#score-about-" + id).value = v || ""; };
  richFields();
  richSet(richBoxes.title, i.titleRuns, i.title);
  richSet(richBoxes.subtitle, i.subtitleRuns, i.subtitle);
  richSet(richBoxes.composer, i.composerRuns, i.composer);
  richSet(richBoxes.arranger, i.arrangerRuns, i.arranger);
  field("copyright", i.copyright);
  field("description", i.description);
  field("tags", (i.tags || []).join(", "));

  // What gscore will make of the tag line, shown as it is typed.
  //
  // A comma-separated line is easy to write and easy to get subtly
  // wrong — a stray comma, the same word twice, a space that looks like
  // a separator and is not. Reading it back as the tags it becomes is
  // the difference between a field somebody trusts and one they check
  // afterwards by saving and reopening.
  //
  // The same rule the server uses, which is the one that counts: this is
  // a preview of what will be stored, so a second opinion here would be
  // a lie half the time.
  const tags = $("#score-about-tags");
  const tagged = $("#score-about-tagged");
  const preview = () => {
    tagged.innerHTML = "";
    const seen = new Set();
    for (const raw of tags.value.split(",")) {
      const t = raw.trim();
      if (!t || seen.has(t.toLowerCase())) continue;
      seen.add(t.toLowerCase());
      const chip = document.createElement("span");
      chip.textContent = t;
      tagged.appendChild(chip);
    }
  };
  tags.oninput = preview;
  preview();

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    scoreAside(dlg);
    dlg.showModal();
    richBoxes.title.focus();
  });
  if (answer !== "ok") {
    status("Ready");
    return;
  }

  // The words and their formatting go together: each field is sent as the
  // text it holds and the runs that spell it, and the server refuses a pair
  // that disagree rather than keeping formatting of words nobody wrote.
  const p = new URLSearchParams({
    copyright: $("#score-about-copyright").value,
    description: $("#score-about-description").value,
    tags: tags.value,
  });
  for (const name of ["title", "subtitle", "composer", "arranger"]) {
    const runs = richRuns(richBoxes[name]);
    const text = runs.map((r) => r.text).join("");
    p.set(name, text);
    // Plain words need no runs, and leaving them out keeps a file that
    // nobody formatted exactly as small and as readable as it was.
    const set = runs.some((r) => r.bold || r.italic || r.colour ||
      (r.size && Math.abs(r.size - 1) >= 0.01));
    if (set) p.set(name + "Runs", JSON.stringify(runs));
  }
  const res = await fetch("/api/header?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status("Saved what this score is");
}

// The meters worth a button. Everything else is typed, which is what the
// escape hatch at the bottom of the picker is for.
const METERS = [
  { v: "C",    label: "Common" },
  { v: "C|",   label: "Cut" },
  { v: "2/4",  label: "2/4" },
  { v: "3/4",  label: "3/4" },
  { v: "4/4",  label: "4/4" },
  { v: "5/4",  label: "5/4" },
  { v: "3/8",  label: "3/8" },
  { v: "6/8",  label: "6/8" },
  { v: "9/8",  label: "9/8" },
  { v: "12/8", label: "12/8" },
  { v: "2/2",  label: "2/2" },
  { v: "7/8",  label: "7/8" },
];

// setTimeDialog shows the meters rather than asking for one to be typed.
//
// You recognise the shape of 6/8 faster than you decide what to type, and the
// two with glyphs of their own have no obvious spelling at all — typing "C"
// for common time and being told it does not look like 4/4 was the box being
// wrong, not the person using it.
async function setTimeDialog(at) {
  const part = state.selection?.part ?? 0;
  const now = state.info?.parts?.[part]?.time || "4/4";
  // Where it takes effect: the bar that was clicked, or the one the cursor
  // is in. Zero means the piece starts in it.
  const measure = at !== undefined ? at : (state.selection?.measure ?? 0);

  // Drawn by the engraver and put into the page, rather than loaded as
  // images: an SVG inside an <img> is an isolated document and cannot reach
  // the music font, so every glyph in it would come out as a missing
  // character box.
  const drawings = await Promise.all(
    METERS.map((m) =>
      fetch("/timesig.svg?space=9&meter=" + encodeURIComponent(m.v))
        .then((r) => (r.ok ? r.text() : ""))
        .catch(() => ""))
  );

  $("#timesig-grid").innerHTML = METERS.map((m, i) =>
    `<button class="timesig-option" data-meter="${escapeHTML(m.v)}"
             ${m.v === now || (m.v === "C" && now === "4/4") ? 'aria-current="true"' : ""}>
       <span class="timesig-art">${drawings[i]}</span>
       <span>${escapeHTML(m.label)}</span>
     </button>`).join("");

  // Say where it takes effect. A meter changed from bar nine looks exactly
  // like a meter that did nothing, if nothing says which bar it was.
  const where = $("#timesig-where");
  if (where) {
    where.textContent = measure > 0
      ? `From measure ${barNumberOf(measure)} to the end, on every staff.`
      : "For the whole piece, on every staff.";
  }
  const hitHere = state.hits.find(
    (h) => h.part === part && h.measure === measure);
  const removeBtn = $("#timesig-remove");
  if (removeBtn) removeBtn.hidden = !(measure > 0 && hitHere?.timeChange);

  const dlg = $("#timesig");
  let typed = false;

  const pick = (v) => { dlg.returnValue = "picked:" + v; dlg.close(); };
  $$("#timesig-grid .timesig-option").forEach((b) =>
    b.addEventListener("click", () => pick(b.dataset.meter)));
  $("#timesig-other").onclick = () => { typed = true; dlg.close(); };

  const result = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.returnValue = "";
    dlg.showModal();
  });
  $("#timesig-other").onclick = null;

  if (result === "remove") {
    const q = new URLSearchParams({
      part: String(part), at: String(measure), time: "none",
    });
    const res = await fetch("/api/staff?" + q.toString(), { method: "POST" });
    if (!res.ok) {
      status((await res.text()).trim());
      return;
    }
    await refreshScore();
    status(`The meter change at measure ${barNumberOf(measure)} is gone`);
    return;
  }

  let meter = null;
  if (typed) {
    meter = await ask({
      title: "Time signature", label: "Meter", value: now,
      note: "Written as 7/8. C is common time and C| is cut time.",
    });
    if (meter === null || meter === "") return;
  } else if (result.startsWith("picked:")) {
    meter = result.slice(7);
  }
  if (!meter) return;

  const p = new URLSearchParams({ part: String(part), time: meter });
  if (measure > 0) p.set("at", String(measure));
  const res = await fetch("/api/staff?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set the meter: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(measure > 0
    ? `${meter} from measure ${barNumberOf(measure)}, on every staff`
    : `${meter}, on every staff`);
}

// instrumentOptions is the catalogue as the picker wants it: one option per
// instrument, carrying the heading it belongs under.
//
// The value is the sound id and not the name, because the id is what says
// which instrument this is. Two of them are called "timpani" and several
// are called some kind of guitar; the names are for reading and the id is
// what gets sent back and written into the file.
let instrumentCatalogue = null;
async function instrumentOptions() {
  if (!instrumentCatalogue) {
    const d = await (await fetch("/api/instruments")).json();
    instrumentCatalogue = d.instruments || [];
  }
  return instrumentCatalogue.map((x) => ({
    // The way it is written down travels with the instrument, because
    // that is how it is chosen: "guitar — notation and tablature" is one
    // thing somebody picks, not an instrument and then a setting.
    value: x.written ? x.sound + "/" + x.written : x.sound,
    label: x.name, group: x.group,
  }));
}

async function setInstrumentDialog() {
  const options = await instrumentOptions();
  const part = state.selection?.part ?? 0;
  const staff = state.info?.parts?.[part];
  const v = await ask({
    title: "Instrument",
    label: staff ? `What ${staff.name} plays` : "Instrument",
    value: staff?.sound || "keyboard.piano",
    note: "Sets the clef, the transposition and the range it is checked " +
          "against, as well as what it is played with. Type to search.",
    search: true,
    options,
  });
  if (v === null) return;
  // Changing what a staff is, which is not the same question as how it
  // is written: that is Staff ▸ Write as tablature, on the staff itself.
  await setStaffProperty("sound", soundChosen(v).sound);
}

// openBarlinePicker asks what the clicked barline should be.
async function openBarlinePicker(bar) {
  const v = await ask({
    title: `Barline after measure ${barNumberOf(bar.measure)}`,
    label: "Draw it as",
    value: bar.style,
    note: "A repeat sends the player back to the matching open repeat.",
    options: [
      { value: "single", label: "Single" },
      { value: "double", label: "Double" },
      { value: "final", label: "Final" },
      { value: "repeat-start", label: "Open repeat" },
      { value: "repeat-end", label: "Close repeat" },
      { value: "repeat-both", label: "Close then open" },
      { value: "none", label: "Invisible" },
      // Not a way of drawing it: a way of not having it. Invisible still
      // ends a bar and still counts as one; this makes the two bars one.
      { value: "remove", label: "Remove it — join the two bars" },
    ],
  });
  if (v === null) return;

  if (v === "remove") {
    const res = await fetch("/api/joinmeasure?measure=" + bar.measure,
                            { method: "POST" });
    if (!res.ok) {
      status("Could not remove it: " + (await res.text()).trim());
      return;
    }
    clearSelection();
    await refreshScore();
    status(`Measures ${barNumberOf(bar.measure)} and ${barNumberOf(bar.measure + 1)} are now one`);
    return;
  }

  const p = new URLSearchParams({
    part: String(bar.part), measure: String(bar.measure), style: v,
  });
  const res = await fetch("/api/barline?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not change it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status("Barline changed");
}

// lyricsDialog types a whole verse at once, which is how NoteWorthy asks for
// words and is still the quickest way anyone has found.
async function lyricsDialog() {
  const parts = state.info?.parts || [];
  if (!parts.length) return;

  const dlg = $("#lyrics");
  const staff = $("#lyrics-staff");
  const verse = $("#lyrics-verse");
  const text = $("#lyrics-text");
  const count = $("#lyrics-count");

  staff.innerHTML = parts
    .map((p, i) => `<option value="${i}">${escapeHTML(p.name || "Staff " + (i + 1))}</option>`)
    .join("");
  staff.value = String(state.selection?.part ?? 0);
  verse.value = "1";

  // Whatever is already there comes back as it would be typed, so a verse is
  // corrected rather than written again.
  const load = async () => {
    const p = new URLSearchParams({ part: staff.value, verse: verse.value });
    const res = await fetch("/api/getlyrics?" + p.toString());
    if (!res.ok) return;
    const d = await res.json();
    text.value = d.text || "";
    count.textContent = `${d.notes} note${d.notes === 1 ? "" : "s"} to sing on` +
      (d.verses > 1 ? ` · ${d.verses} verses written so far` : "");
  };
  await load();
  staff.onchange = load;
  verse.onchange = load;

  const chosen = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue === "ok");
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    setTimeout(() => text.focus(), 0);
  });
  staff.onchange = null;
  verse.onchange = null;
  if (!chosen) return;

  const p = new URLSearchParams({
    part: staff.value, verse: verse.value, text: text.value,
  });
  const res = await fetch("/api/lyrics?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set the words: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  await refreshScore();
  status(d.warning ? d.warning
                   : `${d.set} syllable${d.set === 1 ? "" : "s"} written`);
}

// toggleFingering switches the fingering chart on and off.
//
// It is a plugin in the sense that matters: it watches what is playing and
// what is selected, and draws beside them. Nothing else in the editor knows
// it exists, so a chart for the clarinet is a chart, not a change to gscore.
function toggleFingering() {
  fingering.on = !fingering.on;
  fingering.last = null;
  // Default to the first staff that has a chart, so switching it on shows
  // something rather than "no chart for this staff".
  const parts = state.info?.parts || [];
  if (fingering.on && !fingeringChartFor(parts[fingering.staff]?.instrument)) {
    const i = parts.findIndex((p) => fingeringChartFor(p.instrument));
    if (i >= 0) fingering.staff = i;
  }
  $("#fingering-group").hidden = !fingering.on;
  markFingeringMenu();
  const item = $('[data-action="fingering"] span');
  if (item) item.textContent = fingering.on ? "Hide fingering" : "Show fingering";
  if (fingering.on) {
    updateFingering();
    status("Fingering shown — the note that sounds, as fingers");
  } else {
    status("Ready");
  }
}

// updateFingering draws the chart for whatever is current: the note being
// played, or failing that the one selected.
//
// It is the *written* pitch that is shown, because fingers follow the page. A
// saxophone part written in A is fingered in A whatever note leaves the bell,
// and showing the sounding pitch would teach the wrong thing.
function updateFingering(ev) {
  if (!fingering.on) return;
  const parts = state.info?.parts || [];

  if (ev) {
    // Only the staff being followed. A score has many, and a student is
    // learning one instrument.
    if (ev.part !== fingering.staff) return;
    showFingering(writtenMidiOf(ev), parts[ev.part]?.instrument);
    return;
  }
  const sel = state.selection;
  const part = parts[fingering.staff] || parts[sel?.part ?? 0];
  if (!sel || sel.part !== fingering.staff || sel.ref === null || sel.ref === undefined) {
    fingering.last = null;
    showFingering(null, part?.instrument);
    return;
  }
  const hit = state.hits.find((h) => h.part === sel.part && h.measure === sel.measure);
  const el = hit && (hit.elements || []).find((x) => x.ref === sel.ref);
  if (!el || !el.steps || !el.steps.length) {
    fingering.last = null;
    showFingering(null, part?.instrument);
    return;
  }
  // The staff step is what is on the page; the clef turns it into a pitch,
  // and the sign written on it — or the one the signature gives it — says
  // how far from the plain letter it is. Without that, a player asking for
  // the F sharp in front of them was handed the fingering for F.
  const which = el.steps.length - 1;
  const alter = (el.alters || [])[which] || 0;
  showFingering(midiForStep(part?.clef || "treble", el.steps[which]) + alter,
                part?.instrument);
}

// writtenMidiOf undoes the transposition playback applied, to get back to
// what is printed on the staff.
function writtenMidiOf(ev) {
  const t = state.info?.parts?.[ev.part]?.transposeSemitones || 0;
  return ev.midi - t;
}

// midiForStep turns a staff position into a pitch, mirroring the engraver.
function midiForStep(clef, step) {
  const offsets = { treble: -2, bass: 10, alto: 4, tenor: 6, percussion: 4 };
  const index = step - (offsets[clef] ?? -2);
  // Diatonic index 0 is middle C, not C0: the engraver counts from the
  // middle of the keyboard, because that is where a staff is. Counted from
  // C0 instead, every note came out four octaves below itself — a flute
  // part was answered with "G0, out of the range of this chart", which is
  // true of G0 and says nothing about the G the player is looking at.
  const octave = Math.floor(index / 7) + 4;
  const degree = ((index % 7) + 7) % 7;
  const semis = [0, 2, 4, 5, 7, 9, 11][degree];
  return (octave + 1) * 12 + semis;
}

// setView turns one of the drawing options on or off. These change how the
// score is drawn, not what it says, so they belong to the view and not to the
// file.
async function setView(what, on) {
  const res = await fetch(`/api/view?${what}=${on ? 1 : 0}`, { method: "POST" });
  if (!res.ok) {
    status("Could not change the view");
    return;
  }
  const d = await res.json();
  state.view.names = !!d.names;
  state.view.numbers = !!d.numbers;
  state.view.multirest = !!d.multirest;
  state.view.tempomark = !!d.tempomark;
  state.view.beatmarks = !!d.beatmarks;
  state.view.concert = !!d.concert;
  markViewTicks();
  await refreshScore();
  const said = {
    names: on ? "Staff names shown" : "Staff names hidden",
    numbers: on ? "Measure numbers shown" : "Measure numbers hidden",
    // Said in full, because what it does to the page is not obvious from
    // the words: the bars are still there, and still played.
    multirest: on
      ? "Bars of rest counted — they are still there, and still played"
      : "Every bar drawn",
    // Said in full too: taking the mark off does not change the speed.
    tempomark: on
      ? "Tempo marking shown"
      : "Tempo marking hidden — the piece still plays at that speed",
    // Said in full as well, because what it draws is not notation and
    // nobody should print it by accident.
    beatmarks: on
      ? "Beats marked — an arrow over every beat, to read by rather than to print"
      : "Beat marks hidden",
    // Said in full as well: what is stored has not changed, and writing
    // on a transposing staff still goes in as what its player reads.
    concert: on
      ? "Shown as it sounds — the parts are still written as they are read"
      : "Shown as each player reads it",
  };
  status(said[what] || "Ready");
}

// syncView reads back how the core says this score is being looked at.
//
// A .gscore carries the drawing options it was saved with — which staff
// names, which marks, pages or strip, and the colours — and the core applies
// them as it reads the file. Without this the editor would go on showing the
// ticks and the mode of the score before it, and the menus would disagree
// with the page.
//
// It is read rather than pushed because the core is where the answer is: for
// every other reason the editor reloads, the two already agree and this
// re-asserts what is already true.
async function syncView() {
  try {
    const d = await (await fetch("/api/view", { method: "POST" })).json();
    state.view.names = !!d.names;
    state.view.numbers = !!d.numbers;
    state.view.multirest = !!d.multirest;
    state.view.tempomark = !!d.tempomark;
    state.view.beatmarks = !!d.beatmarks;
    state.view.concert = !!d.concert;
    state.colourNotes = !!d.colourNotes;
    if (d.mode) state.mode = d.mode === "page" ? "page" : "strip";
    markViewTicks();
    markMode();
  } catch (_) {
    /* No answer: the ticks go on saying what they said, which is better
       than a menu that has gone blank. */
  }
}

function markViewTicks() {
  const n = $("#names-tick"), m = $("#numbers-tick"), r = $("#multirest-tick");
  const c = $("#concert-tick"), t = $("#tempomark-tick");
  const b = $("#beatmarks-tick");
  if (n) n.textContent = state.view.names ? "✓" : "";
  if (m) m.textContent = state.view.numbers ? "✓" : "";
  if (r) r.textContent = state.view.multirest ? "✓" : "";
  if (t) t.textContent = state.view.tempomark ? "✓" : "";
  if (b) b.textContent = state.view.beatmarks ? "✓" : "";
  if (c) c.textContent = state.view.concert ? "✓" : "";
  const k = $("#metronome-tick");
  if (k) k.textContent = state.metronome ? "✓" : "";
  const nc = $("#notecolours-tick");
  if (nc) nc.textContent = state.colourNotes ? "✓" : "";
  const w = $("#mouse-tick");
  if (w) w.textContent = state.mouseWrites ? "✓" : "";
}

// setMouseWrites switches between pointing and writing, and remembers which.
//
// Remembered here rather than with the score, because it is a fact about
// how somebody works and not about the music: open another file and the
// mouse should still do what it did a moment ago.
function setMouseWrites(on) {
  state.mouseWrites = !!on;
  try {
    localStorage.setItem("mousewrites", on ? "1" : "0");
  } catch (_) {
    /* a private window may refuse; it still holds for this session */
  }
  markViewTicks();
  status(on
    ? "Writing with the mouse — a click writes a note, shift picks one up"
    : "Pointing with the mouse — a click picks music up, and the keyboard writes");
}

// loadMouseWrites brings back what was chosen last time.
function loadMouseWrites() {
  try {
    state.mouseWrites = localStorage.getItem("mousewrites") === "1";
  } catch (_) {
    /* the default stands */
  }
  markViewTicks();
}

// setMetronome turns the click on the beat on or off, and remembers which.
//
// Remembered the same way and for the same reason as the mouse: somebody
// practising with a click wants it still there when they open the next
// piece, and somebody who has never asked for one should never get one.
//
// It takes effect at the next play rather than in the middle of this one.
// Starting a click halfway through a bar is a click out of time, and a
// metronome out of time is the one thing a metronome must not be.
function setMetronome(on) {
  state.metronome = !!on;
  try {
    localStorage.setItem("metronome", on ? "1" : "0");
  } catch (_) {
    /* a private window may refuse; it still holds for this session */
  }
  markViewTicks();
  status(on
    ? (state.playing
      ? "Metronome on — it starts with the next play"
      : "Metronome on — a click on every beat while the music plays")
    : "Metronome off");
}

// loadMetronome brings back what was chosen last time.
function loadMetronome() {
  try {
    state.metronome = localStorage.getItem("metronome") === "1";
  } catch (_) {
    /* off, which is the default */
  }
  markViewTicks();
}

// chordSymbolDialog writes a chord over the music.
//
// It shows what it is about to write: the same engraver draws the picture in
// the box and the symbol on the page, so what you see is what lands there.
// Typing is also the only way to find out what the program makes of "C7#9"
// without writing it into the score first.
//
// The name and the diagram are separate: either, or both. And it does not
// ask for a note — the bar being selected is enough, and the symbol goes on
// the first thing in it, which is where a chord that governs a bar belongs.
async function chordSymbolDialog() {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first — the chord goes above it");
    return;
  }
  const dlg = $("#chord");
  const name = $("#chord-name");
  const frets = $("#chord-frets");
  const useName = $("#chord-show-name");
  const useFrame = $("#chord-show-frame");
  const image = $("#chord-image");

  // What is there already, so changing a chord is changing it and not
  // typing it again.
  const had = chordAt(sel);
  name.value = had.name;
  frets.value = had.frets;
  useName.checked = had.name !== "" || had.frets === "";
  useFrame.checked = had.frets !== "";

  // The fingering is filled in from the name until somebody types their
  // own, and then it is theirs. Clearing the field asks for the suggestion
  // again.
  let suggested = frets.value;
  let timer = null;

  const suggest = async () => {
    const text = name.value.trim();
    if (text === "" || (frets.value !== "" && frets.value !== suggested)) {
      return; // nothing to go on, or a fingering of their own
    }
    const res = await fetch("/api/chordfingering?chord=" + encodeURIComponent(text));
    if (!res.ok) return;
    const d = await res.json();
    if (frets.value !== "" && frets.value !== suggested) return; // typed meanwhile
    suggested = d.frets || "";
    frets.value = suggested;
  };

  const draw = () => {
    const p = new URLSearchParams({ chord: name.value.trim(), space: "13" });
    if (!useName.checked) p.set("name", "0");
    if (useFrame.checked && frets.value.trim() !== "") {
      p.set("frame", frets.value.trim());
    }
    image.src = "/api/chordpreview?" + p.toString();
    frets.disabled = !useFrame.checked;
    $("#chord-frets-label").style.opacity = useFrame.checked ? "" : ".5";
  };
  // A moment behind the typing: a request a keystroke is a request a
  // keystroke, and the picture only has to keep up with the eye.
  const later = () => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(async () => {
      await suggest();
      draw();
    }, 150);
  };
  name.addEventListener("input", later);
  frets.addEventListener("input", draw);
  // Turning the diagram on with nothing in the field asks for the shape of
  // whatever has been typed.
  const onFrame = async () => {
    if (useFrame.checked && frets.value.trim() === "") await suggest();
    draw();
  };
  useFrame.addEventListener("change", onFrame);
  useName.addEventListener("change", draw);
  draw();

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      if (timer) clearTimeout(timer);
      name.removeEventListener("input", later);
      frets.removeEventListener("input", draw);
      useFrame.removeEventListener("change", onFrame);
      useName.removeEventListener("change", draw);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    setTimeout(() => name.focus(), 0);
  });
  if (answer === "cancel" || answer === "") return;

  if (answer === "remove") {
    await markChord({ chord: "none" }, "Chord symbol removed");
    return;
  }

  const text = name.value.trim();
  const frame = frets.value.trim();
  if ((!useName.checked || text === "") && (!useFrame.checked || frame === "")) {
    await markChord({ chord: "none" }, "Chord symbol removed");
    return;
  }
  const params = { chord: text };
  if (!useName.checked) params.name = "0";
  if (useFrame.checked && frame !== "") params.frame = frame;
  await markChord(params,
    useName.checked && text !== ""
      ? (params.frame ? text + " with a diagram" : text)
      : "Chord diagram");
}

// markChord writes the symbol where the selection says, which is the note
// picked up or, failing that, the first thing in the bar.
async function markChord(params, said) {
  const sel = state.selection;
  const at = hasSelection() && sel.at > 0 ? sel.at : 1;
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(at),
    ...params,
  });
  const res = await fetch("/api/chordsymbol?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not write it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  markSelection();
  status(said);
}

// chordAt is what is written over the selection already — the name and the
// fingering apart, which is how the dialog holds them.
function chordAt(sel) {
  const empty = { name: "", frets: "" };
  if (!sel) return empty;
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure);
  const els = (hit && hit.elements) || [];
  const el = els.find((e) => e.ref === sel.ref) || els[0];
  if (!el) return empty;
  return { name: el.chord || "", frets: el.chordFrets || "" };
}

// textDialog writes words beside the selected note.
async function textDialog() {
  if (!hasSelection()) {
    status("Click a note first — the words go beside it");
    return;
  }
  const dlg = $("#textmark");
  const styles = await (await fetch("/api/textstyles")).json();

  const pick = $("#tm-style");
  pick.innerHTML = styles
    .map((s) => `<option value="${escapeHTML(s.name)}">${escapeHTML(s.label)}</option>`)
    .join("");
  // Words that were picked up by clicking them: the window opens on what
  // they say, in the kind they are and on the side they were put, so that
  // changing them is changing them rather than writing them again. Without
  // it the only way back to a text was to write over the note and hope.
  const editing = state.selection?.textMark || null;
  // The one most words are. Remembered between windows, because somebody
  // writing in the expression markings writes several in a row.
  pick.value = editing ? (editing.style || lastTextStyle) : lastTextStyle;
  richFields();
  richSet(richBoxes["tm-words"], editing && editing.runs,
          editing ? editing.text : "");
  $("#tm-place").value = editing ? (editing.place || "") : "";

  // What the choice means, in the words a reader would use rather than as
  // a list of font settings: where it goes and who it is for.
  const describe = () => {
    const s = styles.find((x) => x.name === pick.value);
    if (!s) {
      $("#tm-look").textContent = "";
      return;
    }
    const chosen = $("#tm-place").value;
    const where = chosen === "above" ? "Over the staff"
      : chosen === "below" ? "Under the staff"
      : s.system
      ? "Over the whole system, for everybody"
      : s.place === "below" ? "Under the staff" : "Over the staff";
    const how = [s.italic ? "italic" : "", s.bold ? "bold" : ""]
      .filter(Boolean).join(" and ");
    $("#tm-look").textContent = how ? `${where}, ${how}.` : `${where}.`;
  };
  pick.oninput = describe;
  $("#tm-place").oninput = describe;
  describe();

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    scoreAside(dlg);
    dlg.showModal();
    richBoxes["tm-words"].focus();
  });
  if (answer === "cancel" || answer === "") {
    status("Ready");
    return;
  }

  const sel = state.selection;
  const runs = richRuns(richBoxes["tm-words"]);
  const words = answer === "remove" ? "" : runs.map((r) => r.text).join("");
  if (answer !== "remove") lastTextStyle = pick.value;
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at),
    text: words, style: pick.value, placement: $("#tm-place").value,
  });
  // Plain words need no runs, and leaving them out keeps a score nobody
  // formatted as small and as readable as it was.
  if (answer !== "remove" && runs.some((r) => r.bold || r.italic || r.colour ||
      (r.size && Math.abs(r.size - 1) >= 0.01))) {
    p.set("runs", JSON.stringify(runs));
  }
  // Changing the words that were picked up, rather than adding more to the
  // note: a note may carry two, and the other one stays as it is.
  if (editing) p.set("index", String(editing.index));
  const res = await fetch("/api/text?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not write it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(words === "" ? "Text removed" : "Text written");
}

// The kind chosen last time. Somebody putting the expression markings into
// a piece writes several in a row, and choosing the same thing each time
// is the sort of asking a program should only do once.
let lastTextStyle = "expression";

// markerDialog puts a rehearsal mark on the current bar, across every staff.
//
// Two kinds, and the difference is what happens next. A mark in the
// sequence is kept as a place and named from where it falls, so putting
// one in before the others moves every letter after it along by itself. A
// mark written out as itself — "1a" at an insert, a word over a chorus —
// is left alone by the sequence and does not advance it.
//
// The sequence is offered first because it is what nearly every mark is.
async function markerDialog() {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  // The cursor is standing in the blank bar past the end the moment the
  // last note of the last bar is written, which is exactly when somebody
  // reaches for a letter — and that bar is somewhere to click to carry on
  // writing rather than a bar of the piece, so there is nothing there for
  // a mark to be written over. Said before the question rather than after
  // it: the sequence named the mark, offered it, took the answer and then
  // put "there is no such measure" in the status line, which from the
  // other side of the screen is a letter that was asked for and silently
  // not drawn.
  if (blankBarPastTheEnd(sel.measure)) {
    status("That is the blank bar past the end — click a bar of the music " +
           "to mark it");
    return;
  }
  const style = state.info?.markerStyle || "letters";
  const next = nextMarkerName(sel.measure);
  const bar = String(barNumberOf(sel.measure));
  const here = (state.info?.marks || {})[sel.measure];

  // The bar number is offered here as well as in the window that sets the
  // style, because this is where somebody is standing when they want it:
  // a mark that says which bar it is on is a mark nobody has to think of a
  // name for, and typing the number by hand writes a mark that stops being
  // true the moment a bar is put in before it.
  //
  // It is still one decision about the whole score — half the marks
  // lettered and half numbered would be two systems of names for one set
  // of places — so the label says that is what it does.
  const byBar = style === "bars"
    ? []
    : [{ value: "bars", label: bar + " — the number of its bar, and the rest by theirs" }];

  const v = await ask({
    title: `Rehearsal mark at measure ${barNumberOf(sel.measure)}`,
    label: "Mark",
    value: "auto",
    note: "Written over every staff, because “from B” has to mean the same " +
          "bar to everyone. A mark in the sequence renames itself when " +
          "another is put in before it, and one that says the number of " +
          "its bar follows the bar.",
    options: [
      {
        value: "auto",
        label: next + (style === "bars"
          ? " — the number of this bar" : " — next in the sequence"),
      },
    ].concat(byBar, [
      { value: "custom", label: "Something else…" },
    ]).concat(here ? [{ value: "remove", label: "Take the mark away" }] : []),
  });
  if (v === null) return;

  const p = new URLSearchParams({ measure: String(sel.measure) });
  let said = "";
  switch (v) {
    case "auto":
      p.set("auto", "1");
      said = "Mark " + next;
      break;
    case "bars":
      // The style and the mark in one go: the server takes both, and
      // doing it in two would redraw the score twice over.
      p.set("auto", "1");
      p.set("style", "bars");
      said = "Mark " + bar + " — rehearsal marks are numbered by their bars";
      break;
    case "remove":
      p.set("text", "");
      said = "Mark removed";
      break;
    default: {
      const text = await ask({
        title: `Rehearsal mark at measure ${barNumberOf(sel.measure)}`,
        label: "Call it",
        value: here && here !== next ? here : "",
        note: "Left out of the sequence, so the letters either side of it " +
              "are unaffected.",
      });
      if (text === null) return;
      if (text.trim() === "") return;
      p.set("text", text);
      said = "Mark " + text;
    }
  }

  const res = await fetch("/api/marker?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  // Setting one mark renames the others, so the editor reads the whole
  // score again rather than patching the bar it touched.
  await refreshScore();
  status(said);
}

// markerStyleDialog chooses what the marks in the sequence are called.
//
// A decision about the whole score. A conductor calls the places one way
// through a piece, and half of them lettered and half numbered would be
// two systems of names for one set of places.
async function markerStyleDialog() {
  const now = state.info?.markerStyle || "letters";
  const v = await ask({
    title: "Rehearsal marks",
    label: "Call them",
    value: now,
    note: "Marks written out as themselves keep what they say, whichever " +
          "this is.",
    options: [
      { value: "letters", label: "Letters — A, B, C" },
      { value: "numbers", label: "Numbers — 1, 2, 3" },
      { value: "bars", label: "The number of the bar they stand on" },
    ],
  });
  if (v === null || v === now) return;

  const res = await fetch("/api/marker?style=" + encodeURIComponent(v),
                          { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status("Rehearsal marks are " + v);
}

// nextMarkerName is what a mark put at this bar would be called.
//
// Asked of the score rather than counted here, because what a mark is
// called depends on every mark before it in its own movement — and
// because counting the marks already written, which is what this used to
// do, offered the letter after the last one wherever the cursor was. A
// mark put in between B and C was then offered D, and the page read A, B,
// D, C.
function nextMarkerName(at) {
  const style = state.info?.markerStyle || "letters";
  if (style === "bars") return String(barNumberOf(at));

  // How many sequence marks come before this bar. The ones written out as
  // themselves are not places the sequence counts through, so they are
  // not among these.
  const n = (state.info?.sequenced || []).filter((bar) => bar < at).length;
  return style === "numbers" ? String(n + 1) : rehearsalLetter(n);
}

// rehearsalLetter is the nth name in the sequence, counting from zero.
//
// A to Z and then AA, BB, CC — doubling, because a conductor shouting
// "A B" across a hall is not heard as one thing. I is skipped: it reads as
// a one at a glance, which is the one thing a rehearsal mark must never be
// mistaken for in a room where people are also calling out bar numbers.
function rehearsalLetter(n) {
  const letters = "ABCDEFGHJKLMNOPQRSTUVWXYZ"; // no I
  if (n < 0) return "";
  const c = letters[n % letters.length];
  return c.repeat(Math.floor(n / letters.length) + 1);
}

// staffRange reads "2-4" as the staves it names, counted from one the way
// they are numbered down the side of the score.
//
// A dash, an en dash or a comma, because all three are what somebody types
// for a range and none of them is wrong. A single number is a range of one
// staff, which is not a group, and is refused by the caller rather than
// quietly turned into something else.
function staffRange(raw, have) {
  const text = String(raw).trim();
  // Digits and separators, beginning with a digit. Checked before it is
  // split rather than after: splitting "-1-2" throws the sign away and
  // reads it as staves 1 to 2, which is an answer to a question nobody
  // asked.
  if (!/^\d[\d\s,–—-]*$/.test(text)) return null;
  const n = text.split(/[-–—,\s]+/).map((s) => s.trim())
    .filter((s) => s !== "").map(Number);
  if (!n.length || n.some((v) => !Number.isInteger(v) || v < 1 || v > have)) {
    return null;
  }
  const first = Math.min(...n) - 1, last = Math.max(...n) - 1;
  return last > first ? { first, last } : null;
}

// joinStaves draws a brace or a bracket down a run of staves.
//
// Not only a pianist's two. A bracket gathers a family — the strings, the
// woodwind, a rhythm section — and that is most of what it is for, so which
// staves it covers is a question and not an assumption. Selecting across
// the staves answers it; where nothing is selected the question is asked,
// because the old behaviour of silently taking every staff in the score was
// right about as often as it was wrong.
async function joinStaves(symbol) {
  const parts = state.info?.parts || [];
  if (parts.length < 2) {
    status("There is only one staff to join");
    return;
  }
  // Which staves: the ones touched by the selection, or whatever is asked
  // for. Taking a group away is about a group that is already there, so it
  // asks nothing.
  let first = 0, last = parts.length - 1;
  if (state.multi.length) {
    first = Math.min(...state.multi.map((m) => m.part));
    last = Math.max(...state.multi.map((m) => m.part));
  }
  if (last <= first && symbol !== "none") {
    const named = parts.map((p, i) => (i + 1) + " " + (p.name || "Staff " + (i + 1)));
    const answer = await ask({
      title: symbol === "brace" ? "Brace which staves?" : "Bracket which staves?",
      label: "Staves",
      value: "1–" + parts.length,
      note: "A range, as 2–4. Of " + named.join(", ") + ".",
    });
    if (answer === null) return;
    const got = staffRange(answer, parts.length);
    if (!got) {
      status(`${answer} is not a range of two staves or more`);
      return;
    }
    ({ first, last } = got);
  }
  if (last <= first) { first = 0; last = parts.length - 1; }

  const p = new URLSearchParams({
    first: String(first), last: String(last), symbol,
  });
  if (symbol === "brace") p.set("barlines", "1");
  const res = await fetch("/api/group?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not join them: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(symbol === "none"
    ? "Staves no longer joined"
    : `Staves ${first + 1}–${last + 1} joined with a ${symbol}`);
}

// markFingeringMenu builds the list of staves to follow, since which staff is
// worth watching depends on what is in the score.
function markFingeringMenu() {
  const box = $("#fingering-menu");
  if (!box) return;
  const parts = state.info?.parts || [];
  box.innerHTML =
    `<button data-action="fingering-off"><span>Off</span>` +
    `<kbd>${fingering.on ? "" : "✓"}</kbd></button>` +
    (parts.length ? "<hr>" : "") +
    parts.map((p, i) => {
      const has = !!fingeringChartFor(p.instrument);
      const tick = fingering.on && fingering.staff === i ? "✓" : "";
      return `<button data-fingering-staff="${i}"${has ? "" : " disabled"}>` +
             `<span>${escapeHTML(p.name || "Staff " + (i + 1))}` +
             ` — ${escapeHTML(p.instrument || "no instrument")}</span>` +
             `<kbd>${tick}</kbd></button>`;
    }).join("");

  $$("[data-fingering-staff]").forEach((b) =>
    b.addEventListener("click", () => {
      if (b.disabled) return;
      closeMenus();
      fingering.on = true;
      fingering.staff = Number(b.dataset.fingeringStaff);
      fingering.last = null;
      $("#fingering-group").hidden = false;
      markFingeringMenu();
      updateFingering();
      status("Following " + (state.info?.parts?.[fingering.staff]?.instrument || "that staff"));
    })
  );
}

// openHeaderPicker changes whatever was clicked at the head of a staff.
//
// Where it takes effect is where it was clicked: the one at the very start of
// the piece is the piece's, and the one restated at the head of a later system
// changes things from that bar on. Either way it goes on every staff, because
// a key and a meter belong to the music and not to one player — one staff in
// 4/4 and the next in 2/4 does not mean anything.
async function openHeaderPicker(head) {
  state.selection = {
    ...(state.selection || {}),
    part: head.part, measure: 0, at: 0, step: 0, ref: null,
  };
  switch (head.kind) {
    case "clef":
      await setClefDialog(head.part);
      return;
    case "key":
      await openKeyPicker(head.part, 0);
      return;
    case "time":
      await setTimeDialog(0);
      return;
  }
}

// setStaffLinesDialog changes how many lines a staff is ruled with.
//
// One line is the ordinary way to write a drum that has no pitch, and the
// reason this is here rather than hidden in the importers: a percussion
// staff typed into gscore should be able to look like one.
//
// What is drawn changes and where the notes go does not — the staff keeps
// the height and the positions of a five-line staff — so this can be
// changed back and forth without touching the music.
async function setStaffLinesDialog(part) {
  const now = String(state.info?.parts?.[part]?.staffLines || 5);
  const v = await ask({
    title: "Staff lines", label: "Rule this staff with", value: now,
    note: "One line is how a drum with no pitch is written.",
    options: [
      { value: "5", label: "Five — the usual staff" },
      { value: "1", label: "One — a drum with no pitch" },
      { value: "2", label: "Two" },
      { value: "3", label: "Three" },
      { value: "4", label: "Four" },
    ],
  });
  if (v === null || v === now) return;
  const res = await fetch(`/api/staff?part=${part}&lines=${encodeURIComponent(v)}`,
                          { method: "POST" });
  if (!res.ok) {
    status("Could not change the staff: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(v === "1" ? "One line" : v + " lines");
}

// setClefDialog changes one staff's clef, which unlike the key and the meter
// really does belong to the staff alone.
async function setClefDialog(part) {
  const now = state.info?.parts?.[part]?.clef || "treble";
  const v = await ask({
    title: "Clef", label: "Draw this staff in", value: now,
    options: [
      { value: "treble", label: "Treble" },
      { value: "bass", label: "Bass" },
      { value: "alto", label: "Alto" },
      { value: "tenor", label: "Tenor" },
      { value: "percussion", label: "Percussion" },
    ],
  });
  if (v === null || v === now) return;
  const res = await fetch(`/api/staff?part=${part}&clef=${encodeURIComponent(v)}`,
                          { method: "POST" });
  if (!res.ok) {
    status("Could not change the clef: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status("Clef changed");
}

// stepHistory takes an edit back, or puts it on again.
//
// The score is redrawn from the server's copy rather than patched here: the
// browser holds no music, so there is nothing on this side to roll back.
async function stepHistory(which) {
  const res = await fetch("/api/" + which, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  const d = await res.json();
  // Undoing can land on a different tune of a book, if that is where the
  // edit was made. That is a different score on the screen, so the whole of
  // it is read again and the caret goes wherever that score left it.
  if (d.tune !== undefined && d.tune !== state.info?.current) {
    await reload();
    markHistory(d.canUndo, d.canRedo);
    status(which === "undo" ? "Undone" : "Redone");
    return;
  }

  // Otherwise the caret stays where it is.
  //
  // Taking a note back went through the same path as opening a file: the
  // selection was dropped and the caret put back wherever the score was
  // last left, which on a piece being written from the top is bar one. So
  // writing a wrong note in bar 40 and taking it back sent you to the
  // beginning, and the way to carry on was to find bar 40 again. Undo is
  // not a change of place — it is the same place with one thing less in
  // it.
  const was = state.selection ? { ...state.selection } : null;
  await refreshScore();
  restoreCaret(was);
  markHistory(d.canUndo, d.canRedo);
  status(which === "undo" ? "Undone" : "Redone");
}

// restoreCaret puts the caret back where it was after the music under it
// has changed, as far as what is left allows.
//
// What it was pointing at may be gone — that is what was undone — so
// nothing is picked up and nothing is held for a mark to land on. The place
// is kept and clamped: the bar it was in may have been the one taken away,
// and the note it stood after may have been the note taken back.
function restoreCaret(was) {
  if (!was) return;
  const parts = state.info?.parts?.length || 1;
  const part = Math.max(0, Math.min(was.part ?? 0, parts - 1));
  const bars = state.hits.filter((h) => h.part === part).map((h) => h.measure);
  if (!bars.length) return;
  const measure = Math.max(0, Math.min(was.measure ?? 0, Math.max(...bars)));
  const hit = state.hits.find((h) => h.part === part && h.measure === measure);
  const held = hit ? (hit.elements || []).length : 0;
  state.selection = {
    part, measure,
    at: Math.max(0, Math.min(was.at ?? 0, held)),
    step: was.step ?? 0,
    ref: null,
    replace: false,
  };
  // Whatever was picked up was picked up in the score as it was a moment
  // ago, and every one of those refs is stale.
  state.multi = [];
  state.anchor = null;
  state.focus = null;
  // Marked, not scrolled to: the point of this is that the view does not
  // move.
  markSelection();
}

// markHistory greys the entries when there is nothing to take back, so the
// menu says what it will do rather than looking the same either way.
function markHistory(canUndo, canRedo) {
  const u = $('[data-action="undo"]');
  const r = $('[data-action="redo"]');
  if (u) u.disabled = !canUndo;
  if (r) r.disabled = !canRedo;
}

// drumName is what was written: the drum that was chosen, or failing that
// the one nearest the position that was clicked, which is what the engraver
// will have put there.
function drumName(key, step) {
  if (!state.kit) return "";
  if (key !== null && key !== undefined) {
    return state.kit.find((d) => d.key === key)?.name || "";
  }
  let best = null;
  for (const d of state.kit) {
    if (best === null || Math.abs(d.step - step) < Math.abs(best.step - step)) {
      best = d;
    }
  }
  return best ? best.name : "";
}

// isDrumStaff says whether a staff's lines name instruments rather than
// pitches.
function isDrumStaff(part) {
  return state.info?.parts?.[part]?.clef === "percussion";
}

// loadKit fetches the drum kit and shows the pads, for a score that has a
// drum staff and not otherwise.
async function loadKit() {
  const wanted = (state.info?.parts || []).some((p) => p.clef === "percussion");
  if (!wanted) {
    fillPaletteSection("drums", [], paletteAction);
    $("#drumpad").hidden = true;
    return;
  }
  if (!state.kit) {
    await fetchKit(savedKitID());
    if (!state.kit) return;
  }
  // The palette keeps a list of the kit as well. It is the long way round —
  // scroll, read, click — but it is where every other mark in the score is
  // chosen from, and somebody who has learnt the palette should not have to
  // learn somewhere else for drums.
  const items = state.kit.pads.map((d) => ({
    g: HEAD_GLYPH[d.notehead] || HEAD_GLYPH.normal,
    label: d.name,
    drum: d.key,
  }));
  fillPaletteSection("drums", items, paletteAction, 15);
  renderDrumPads();
}

// fetchKit asks for one of the kit sizes and remembers which was chosen.
async function fetchKit(id) {
  const res = await fetch("/api/percussion" + (id ? "?kit=" + encodeURIComponent(id) : ""));
  if (!res.ok) return;
  state.kit = await res.json();
  try {
    localStorage.setItem("drumkit", state.kit.kit);
  } catch (_) {
    /* a private window may refuse; the kit still works for this session */
  }
}

// savedKitID is the size last chosen, so a drummer who works on the large kit
// does not pick it again every time they open a chart.
function savedKitID() {
  try {
    return localStorage.getItem("drumkit") || "";
  } catch (_) {
    return "";
  }
}

// renderDrumPads draws the grid.
//
// Each pad shows the notehead it writes, what the instrument is called, the
// letter that types it and the General MIDI key it sounds on. The letter is
// on the pad rather than in a help page because that is the only place it
// will be read, and the MIDI key is there because a part that has to line up
// with a sampler is lined up by number.
function renderDrumPads() {
  const grid = $("#drumpad-grid");
  if (!state.kit) return;
  grid.textContent = "";

  // A pad is chosen already, or the snare is: starting with nothing chosen
  // means the first click on the staff writes whatever the position happens
  // to name, which is the guessing the pads exist to replace.
  if (!state.kit.pads.some((d) => d.key === state.tool.drum)) {
    const snare = state.kit.pads.find((d) => /snare/i.test(d.name));
    state.tool.drum = (snare || state.kit.pads[0] || {}).key ?? null;
  }

  for (const d of state.kit.pads) {
    const pad = document.createElement("button");
    pad.className = "pad" + (d.key === state.tool.drum ? " on" : "");
    pad.dataset.drum = String(d.key);
    pad.title = `${d.name} — MIDI key ${d.key}`;

    const head = document.createElement("span");
    head.className = "pad-head";
    head.textContent = HEAD_GLYPH[d.notehead] || HEAD_GLYPH.normal;

    const name = document.createElement("span");
    name.className = "pad-name";
    name.textContent = d.name;

    const keys = document.createElement("span");
    keys.className = "pad-key";
    if (d.shortcut) {
      const kbd = document.createElement("kbd");
      kbd.textContent = d.shortcut;
      keys.append(kbd);
    }
    const midi = document.createElement("span");
    midi.className = "pad-midi";
    midi.textContent = d.key;
    keys.append(midi);

    pad.append(head, name, keys);
    pad.addEventListener("click", () => void hitPad(d.key));
    grid.append(pad);
  }

  const sel = $("#drumpad-kit");
  sel.textContent = "";
  for (const k of state.kit.kits) {
    const opt = document.createElement("option");
    opt.value = k.id;
    opt.textContent = `${k.name} (${k.pads})`;
    opt.title = k.description;
    opt.selected = k.id === state.kit.kit;
    sel.append(opt);
  }
  const chosen = state.kit.kits.find((k) => k.id === state.kit.kit);
  $("#drumpad-note").textContent = chosen ? chosen.description : "";
  showDrumPad();
}

// showDrumPad puts the pads up when the cursor is on a drum staff and takes
// them away when it is not.
//
// They used to be up for the whole of any score with a drum staff anywhere
// in it. On an orchestral score that means a kit under the music the whole
// time, for one staff out of fifty — and most of the time the cursor is
// nowhere near it. A drum kit is for writing drums on.
function showDrumPad() {
  const pad = $("#drumpad");
  if (!pad) return;
  pad.hidden = !(state.kit && !state.readOnly &&
                 isDrumStaff(state.selection?.part));
}

// hitPad writes a pad's drum at the cursor.
//
// A pad used to only choose a drum, and the drum then had to be written by
// clicking the staff as well: two gestures for one note, and the second of
// them a click on five lines that name instruments rather than pitches —
// which is exactly the guessing the pads are here to replace. A pad is a
// drum. Pressing it writes it, the way its letter already does.
async function hitPad(key) {
  pickDrum(key);
  await insertAtSelection(false);
}

// pickDrum chooses what the next note written on a drum staff will be, and
// lights the pad that says so.
function pickDrum(key) {
  state.tool.drum = key;
  state.tool.rest = false;
  for (const pad of $$("#drumpad .pad")) {
    pad.classList.toggle("on", Number(pad.dataset.drum) === key);
  }
}

// drumPadForKey finds the pad a letter types, or nothing if the letter is not
// one of them.
function drumPadForKey(letter) {
  return (state.kit?.pads || []).find((d) => d.shortcut === letter);
}

// typeDrum writes a pad's drum at the cursor, which is what a pad's letter is
// for: a groove is typed, not clicked thirty-two times.
async function typeDrum(letter) {
  const pad = drumPadForKey(letter);
  if (!pad) return false;
  await hitPad(pad.key);
  return true;
}

// changeDrumKit switches how much of the kit is offered.
async function changeDrumKit(id) {
  await fetchKit(id);
  renderDrumPads();
  await loadKit();
}

/* ---------- staves ---------- */

// addStaffDialog adds a staff, asking what it is for.
//
// A score starts with one staff and a piece for three players needs three.
// Without this the only way to a trio chart was to write it somewhere else
// and import it, which is not a notation program.
async function addStaffDialog() {
  const options = await instrumentOptions();
  const inst = await ask({
    title: "Add a staff",
    label: "What plays it",
    value: "wind.flutes.flute",
    note: "The clef and the transposition follow from the instrument. " +
          "Choose a drum or a kit for a staff whose lines name instruments " +
          "rather than pitches. Type to search.",
    search: true,
    options,
  });
  if (inst === null) return;

  const chosen = soundChosen(inst);
  // How many lines the tab is ruled with is not a setting of its own: it
  // is however many strings the instrument has, so the question to ask
  // is the tuning. A bass is four strings and often five or six.
  if (chosen.written) {
    const tuning = await askTuningFor(chosen.sound);
    if (tuning === null) return;
    if (tuning) chosen.tuning = tuning;
  }
  const p = new URLSearchParams(chosen);
  // Below the staff the cursor is on, which is where somebody adding a
  // staff while looking at one almost always means.
  if (state.selection) p.set("at", String((state.selection.part ?? 0) + 1));

  const res = await fetch("/api/addstaff?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not add it: " + (await res.text()).trim());
    return;
  }
  await reload();
  status("Added a staff for " + instrumentNamed(inst));
}

// askTuningFor asks which tuning a tab staff is written in, where the
// instrument has more than one worth offering.
//
// Silent where it has only one: a mandolin is tuned one way and being
// asked about it is a question with a single answer. The strings are
// named beside each, because that is what tells a five-string bass from
// a four and drop D from DADGAD.
async function askTuningFor(sound) {
  const want = (instrumentCatalogue || []).find((x) => x.sound === sound);
  const theirs = want && want.tuning;
  if (!theirs) return "";

  const all = await (await fetch("/api/tunings")).json();
  // The ones for this instrument: a tuning is named after what it is
  // for, so the family is the first word of the default's name.
  const family = String(theirs).split(" ")[0];
  const mine = all.filter((t) => t.name.split(" ")[0] === family);
  if (mine.length < 2) return "";

  return ask({
    title: "Tuning",
    label: "Written for",
    value: theirs,
    note: "The tab is ruled with a line for each string, so this is also " +
          "how many lines it has.",
    search: mine.length > 8,
    options: mine.map((t) => ({
      value: t.name,
      label: `${t.name} — ${t.strings.length} strings — ${t.strings.join(" ")}`,
    })),
  });
}

// soundChosen splits what the picker sends back into the instrument and
// the way it is written down.
function soundChosen(value) {
  const at = String(value).indexOf("/");
  if (at < 0) return { sound: value };
  return { sound: value.slice(0, at), written: value.slice(at + 1) };
}

// instrumentNamed is what to call a sound id in a sentence to the person,
// who chose "bass clarinet" and should not be told about
// "wind.reed.clarinet.bass".
function instrumentNamed(value) {
  const { sound, written } = soundChosen(value);
  const found = (instrumentCatalogue || []).find(
    (x) => x.sound === sound && (x.written || "") === (written || ""));
  return found ? found.name : sound;
}

// insertMeasuresDialog puts empty bars in after the bar the cursor is on.
//
// Two questions, and the second one only where there is more than one
// staff to ask it about. Every staff is nearly always right — a bar is a
// bar across the score and the barlines have to stay in a column — so it
// is what the dialog opens on, and the other answer is there for the case
// it exists for: a line typed a bar short against the rest.
async function insertMeasuresDialog() {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  const at = sel.measure;
  const howMany = await ask({
    title: `Insert after measure ${barNumberOf(at)}`,
    label: "How many",
    value: "1",
    note: "Empty bars, to the right of this one. The rest of the piece " +
          "moves over, and what is written about a bar — a page turn, a " +
          "cue, a repeat — moves with it.",
  });
  if (howMany === null) return;
  const count = Number(String(howMany).trim());
  if (!Number.isInteger(count) || count < 1 || count > 100) {
    status(`${howMany} is not a number of bars — 1 to 100`);
    return;
  }

  const p = new URLSearchParams({ measure: String(at), count: String(count) });
  const staves = (state.info?.parts || []).length;
  if (staves > 1) {
    const name = state.info?.parts?.[sel.part]?.name || "this staff";
    const where = await ask({
      title: `Insert ${count === 1 ? "a bar" : count + " bars"}`,
      label: "Into",
      value: "all",
      note: "Into one staff only moves that staff's music along against " +
            "everybody else's, and the other staves are made up at the " +
            "end so the score stays the same length.",
      options: [
        { value: "all", label: "Every staff" },
        { value: "staff", label: `${name} only` },
      ],
    });
    if (where === null) return;
    if (where === "staff") {
      p.set("where", "staff");
      p.set("part", String(sel.part));
    }
  }

  const res = await fetch("/api/insertmeasure?" + p, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  await reload();
  status(count === 1 ? "A bar put in" : count + " bars put in");
}

/* ---------- score setup ---------- */

// Who is playing, chosen all at once.
//
// gscore opens with a staff ready and that is right: a tune goes in before
// anybody has decided what it is for, and being asked to fill in a form
// first is how a good idea gets lost. This is the other case — somebody
// who already knows the piece is a wind quintet, and who should not have
// to add five staves one at a time, order them by hand and then bracket
// them.
//
// The chosen list is kept here rather than read back off the dialog,
// because it is a list with an order and a select is a set.
//
// Each entry carries an id of its own, and the groups are kept as sets of
// those ids rather than of positions: a group named by position stops
// meaning what it meant the moment anything is moved or taken away, and
// moving things about is most of what this dialog is for.
let setupChosen = [];
let setupGroups = [];
let setupNextId = 1;

// setupApart holds the staves somebody has taken out of an entry above
// them, by staff number.
//
// Two staves are listed as one entry either because the score braces them
// or because the editor read a pair of them as one instrument's two hands.
// Ungroup has to undo both, and undoing the second cannot be done to the
// score — there is nothing in the file to take away. So it is remembered
// here, and the reading consults it: a staff in this set begins its own
// entry however much it looks like the one above.
//
// It is cleared when the dialog opens, because it is an answer about the
// staves in front of somebody and staff numbers do not survive a score
// being laid out again.
let setupApart = new Set();

async function scoreSetupDialog() {
  const dlg = $("#score-setup");
  if (!dlg) return;
  const options = await instrumentOptions();

  // What the score already holds, so the dialog opens on it rather than
  // on nothing: somebody reopening it means to change the ensemble, not
  // to start again from an empty list.
  // An empty score is being built, not described. gscore opens with one
  // staff ready so that a tune can be typed in without filling in a form
  // first, and that staff is not an answer to "who is playing" — listing it
  // would make somebody take a piano out of the score before they could put
  // a wind quintet in.
  const blank = !scoreHasMusic() &&
    (state.info?.parts || []).every((p) => !p.sound);
  setupApart = new Set();
  setupChosen = blank ? [] : setupFromScore();
  setupGroups = [];

  // The order the staves are actually in, not the order a score is
  // conventionally laid out in: a dialog opened on an existing score is
  // showing that score, and a list that silently reshuffles it cannot be
  // read against the page. Putting them in order is still a tick away.
  const order = $("#setup-order");
  if (order) order.checked = !scoreHasMusic();

  drawSetupPick(options, "");
  drawSetupChosen();
  void drawSetupSounds();

  const filter = $("#setup-filter");
  if (filter) {
    filter.value = "";
    filter.oninput = () => drawSetupPick(options, filter.value);
  }
  if (!dlg.open) dlg.showModal();
}

// drawSetupSounds lists the instruments in this score and which recordings
// each of them is played with.
//
// One row per instrument and not per staff: a score with four horns in it
// is one answer about horns, and asking it four times is asking somebody to
// keep four answers in step by hand. The staves follow the instrument they
// are playing.
//
// Only where there is a choice to make. A machine with one soundfont on it
// has nothing to say here, and a row of dropdowns all reading the same
// thing is furniture.
async function drawSetupSounds() {
  const box = $("#setup-sounds");
  if (!box) return;
  const body = $("#setup-sounds-table").querySelector("tbody");
  body.innerHTML = "";

  // The instruments in this score, each once, in the order the staves are.
  //
  // By the catalogue name where the score has one, and by what the staff
  // sounds as where it does not. A score read from a format that records
  // the General MIDI program and not the catalogue — which is most of
  // them — has staves that know they are flutes without knowing what
  // gscore calls a flute, and asking only for the catalogue name left
  // this list empty on exactly the scores that most want it.
  const seen = new Map();
  for (const p of state.info?.parts || []) {
    const sound = p.sound || soundForInstrumentName(p.instrument);
    if (!sound || seen.has(sound)) continue;
    seen.set(sound, p.name || p.instrument || sound);
  }
  let fonts = [];
  let chosen = {};
  try {
    const [sf, st] = await Promise.all([
      fetch("/api/soundfonts").then((r) => r.ok ? r.json() : null),
      fetch("/api/settings").then((r) => r.ok ? r.json() : null),
    ]);
    fonts = (sf && sf.installed) || [];
    chosen = (st && st.instrumentFonts) || {};
  } catch (_) { /* nothing installed, nothing to choose */ }

  // Shown even when there is nothing in it to set, and told why.
  //
  // It used to take itself off the page whenever it had nothing to say,
  // which is the worst thing a setting can do: somebody looking for it
  // finds a dialog with no such section and no way to tell whether they
  // are in the wrong window or whether it is not there yet. The only time
  // it stays away is when there are no staves at all, and then the whole
  // dialog is about making some.
  const note = $("#setup-sounds-note");
  if (!(state.info?.parts || []).length) {
    box.hidden = true;
    return;
  }
  box.hidden = false;
  if (fonts.length < 2) {
    note.textContent = fonts.length === 1
      ? "There is one soundfont on this machine, so there is nothing to "
        + "choose between yet. Download another under Play, Playback sound "
        + "and the instruments will be listed here."
      : "There are no soundfonts on this machine yet. Download one under "
        + "Play, Playback sound and the instruments will be listed here.";
    return;
  }
  note.textContent = "Said once about the instrument, and it holds in every "
    + "score you write. Kept on this machine rather than in the score, "
    + "because these are files on your own disk: a score you send somebody "
    + "names instruments, not the recordings you happen to own.";

  for (const [sound, name] of seen) {
    const tr = document.createElement("tr");
    const label = document.createElement("td");
    label.textContent = name;
    tr.append(label);

    const cell = document.createElement("td");
    const sel = document.createElement("select");
    const def = document.createElement("option");
    def.value = "";
    def.textContent = "whichever is chosen at the time";
    sel.append(def);
    for (const f of fonts) {
      const o = document.createElement("option");
      o.value = f.path;
      o.textContent = f.name;
      o.selected = chosen[sound] === f.path;
      sel.append(o);
    }
    sel.onchange = async (e) => {
      await saveSetting("instrumentFont", sound + "=" + e.target.value);
      status(e.target.value
        ? name + " will play through " + (fonts.find(
          (f) => f.path === e.target.value)?.name || "that soundfont")
        : name + " follows whichever soundfont is chosen");
    };
    cell.append(sel);
    tr.append(cell);
    body.append(tr);
  }
}

// soundForInstrumentName is the catalogue's name for what a staff sounds
// as, found by the name the score reports it under.
//
// A part says "flute" because that is what its General MIDI program is
// called, and the catalogue calls the same thing "wind.flutes.flute".
// Matched by name against the catalogue the server already sent, so there
// is one list of instruments and this is not a second opinion about it.
function soundForInstrumentName(name) {
  const want = (name || "").trim().toLowerCase();
  if (!want) return "";
  const hit = (instrumentCatalogue || []).find(
    (c) => !c.percussion && (c.name || "").toLowerCase() === want);
  return hit ? hit.sound : "";
}

// scoreHasMusic says there is something here worth not throwing away.
function scoreHasMusic() {
  return (state.info?.parts || []).some((p) => (p.notes || 0) > 0);
}

// setupFromScore is the staves that are in the score, as entries this
// dialog can work on.
//
// One entry to a staff, and each remembers which staff or staves it came
// from. That is what makes it possible to change an instrument without
// rebuilding anything: the entry knows where to send the answer.
//
// A staff that was never told what it is comes back as a piano, because a
// piano is what it plays. Saying nothing would be truer to the file and
// useless here — a PDF read as eight staves arrives with eight blanks, and
// a window that answers "who is playing" with an empty list is the window
// that was wrong before. The blank is recorded all the same, in `said`, so
// nothing claims the file named an instrument it did not.
//
// Two staves of one instrument — a piano's two hands, a harp's — are one
// entry, or reopening the dialog would add a second piano every time.
//
// What makes them one is the brace. That is not a detail of drawing: a
// brace is how a score says "one player, two staves", and a bracket says
// "different players, same family". Two staves of a piano and two staves of
// cello look alike in every other way, so the brace is the only thing that
// tells them apart, and it is read rather than guessed at.
//
// Where there is no brace — a file that came in without one — a pair is
// taken for a grand staff if it is exactly two staves, side by side, of the
// same named instrument, and that instrument is one that is written on two
// staves at all. Each clause is doing work. Two flutes in a row are Flute 1
// and Flute 2 and must stay two; three pianos in a row are three pianos and
// not one on three staves; and eight staves read out of a PDF, which name
// no instrument whatever, are eight staves and not four pianos, which is
// why a sound that was merely defaulted to never pairs.
function setupFromScore() {
  const parts = state.info?.parts || [];
  const braced = new Map();
  for (const g of state.info?.groups || []) {
    if (g.symbol !== "brace") continue;
    for (let n = g.first; n <= g.last; n++) braced.set(n, g.first);
  }
  const out = [];
  parts.forEach((p, idx) => {
    const last = out[out.length - 1];
    const after = last && last.staves[last.staves.length - 1] === idx - 1;
    // Joined by a brace to the staff above: the same instrument, said so
    // by the score itself.
    const sameBrace = after && !setupApart.has(idx) && braced.has(idx) &&
      braced.get(idx) === braced.get(last.staves[0]);
    // Or a pair that looks like one and says nothing to the contrary.
    //
    // A pair, and only a pair. Two staves of piano in a row are one
    // pianist's hands; ten in a row are not, they are a score whose
    // staves have not been told what they are, and pairing them off down
    // the page turns twenty-three instruments into eleven two-handed
    // pianists. An orchestral score read from a file that records no
    // instruments arrives exactly that way, and then the only way to set
    // one staff is to break a pair the score never had.
    //
    // So the run of staves playing the same thing has to be two long.
    // A piano sonata is two and merges; a piano and a violin is two and
    // merges; a page of unnamed staves is not and is left alone.
    const run = sameSoundRun(parts, idx);
    const looksGrand = after && !setupApart.has(idx) &&
      !braced.has(idx) && !braced.has(last.staves[0]) &&
      p.sound && last.said && last.sound === p.sound &&
      last.staves.length === 1 && grandSound(p.sound) && run === 2;
    if (sameBrace || looksGrand) {
      last.staves.push(idx);
      return;
    }
    out.push({
      id: setupNextId++,
      sound: p.sound || "keyboard.piano",
      said: !!p.sound,
      staves: [idx],
    });
  });
  return out;
}

// sameSoundRun is how many staves in a row play what this one plays,
// counting from wherever that run begins.
//
// The run and not the pair: asked of the second staff of ten pianos it
// answers ten, which is what says they are not a grand staff.
function sameSoundRun(parts, idx) {
  const sound = parts[idx] && parts[idx].sound;
  if (!sound) return 0;
  let first = idx;
  while (first > 0 && parts[first - 1].sound === sound) first--;
  let last = idx;
  while (last + 1 < parts.length && parts[last + 1].sound === sound) last++;
  return last - first + 1;
}

// grandSound says an instrument is normally written on two staves joined by
// a brace. Asked of the catalogue the server sends, so there is one list of
// them and it is the one the staves are made from.
function grandSound(value) {
  const { sound } = soundChosen(value);
  return !!(instrumentCatalogue || []).find((x) => x.sound === sound)?.grand;
}

// setupInPlace is the chosen list where it still describes the score's own
// staves, one for one and in their order — and null where it has become a
// different ensemble.
//
// Adding an instrument, removing one or moving one about is a request to
// lay the score out again, which throws the music away and has to be asked
// about. Changing what a staff plays is not, and must not be charged the
// same price. The test is exact rather than a guess at intent: every entry
// still carries the staves it came from, and together they still come to
// every staff in the score, in order.
function setupInPlace() {
  const list = setupOrdered();
  const parts = state.info?.parts || [];
  const seen = [];
  for (const it of list) {
    if (!it.staves) return null;
    seen.push(...it.staves);
  }
  if (seen.length !== parts.length) return null;
  if (seen.some((n, i) => n !== i)) return null;
  return list;
}

// drawSetupPick fills the list of instruments to choose from, under the
// headings the catalogue is grouped by.
function drawSetupPick(options, query) {
  const pick = $("#setup-pick");
  if (!pick) return;
  const q = query.trim().toLowerCase();
  const hit = (o) => q === "" || o.label.toLowerCase().includes(q) ||
    o.group.toLowerCase().includes(q);

  let html = "", open = "";
  for (const o of options) {
    if (!hit(o)) continue;
    if (o.group !== open) {
      if (open) html += "</optgroup>";
      html += `<optgroup label="${escapeHTML(o.group)}">`;
      open = o.group;
    }
    html += `<option value="${escapeHTML(o.value)}">${escapeHTML(o.label)}</option>`;
  }
  if (open) html += "</optgroup>";
  pick.innerHTML = html;
}

// drawSetupChosen fills the list of what is in the score, in the order it
// will be laid out.
function drawSetupChosen() {
  const box = $("#setup-chosen");
  if (!box) return;
  const list = setupOrdered();
  box.innerHTML = list.map((it, i) => {
    // A mark against the ones that are bracketed together, so a group is
    // something you can see rather than something you have to remember.
    const g = setupGroups.findIndex((ids) => ids.includes(it.id));
    const tag = g < 0 ? "" : "  ⌈" + (g + 1) + "⌉";
    // Which staff on the page this is. Eight staves all reading "Piano" is
    // a list nobody can change the third of; with the staff numbers on them
    // the list can be read against the score.
    const no = it.staves
      ? it.staves.map((n) => n + 1).join("+") + ". "
      : "";
    return `<option value="${i}">${escapeHTML(no + instrumentNamed(it.sound) + tag)}</option>`;
  }).join("");
  drawSetupBar();
}

// drawSetupBar settles which of the two things this dialog can do is the
// one being offered.
//
// Both stay reachable. Which is primary follows from what is in front of
// somebody: a score with music whose staves are all still listed is a score
// whose instruments are being named, and making the staves again would
// throw away what they came here to keep.
function drawSetupBar() {
  const inst = $("#setup-instruments");
  const apply = $("#setup-apply");
  if (!inst || !apply) return;
  const live = setupLive();
  // Over a score with music there is only one button, and it is the one
  // that keeps the music. Laying the staves out again is not offered,
  // because there is nothing it could do here that would not throw away
  // what somebody came to name.
  inst.hidden = !live;
  inst.classList.toggle("primary", live);
  apply.hidden = live;
  apply.classList.toggle("primary", !live);
  const warn = $("#setup-warning");
  if (warn) warn.hidden = true;
  const note = $("#setup-live");
  if (note) note.hidden = !live;
  // And the window says which of the two things it is doing, rather than
  // describing the one it is not.
  const blurb = $("#setup-blurb");
  if (blurb) blurb.hidden = live;
  const blurbLive = $("#setup-blurb-live");
  if (blurbLive) blurbLive.hidden = !live;
  // The automatic order rearranges a list; over a score it would be asking
  // for the staves themselves to be moved, which is what the arrows are
  // for and is done one at a time.
  const order = $("#setup-order");
  if (order) {
    order.disabled = live;
    order.closest("label")?.classList.toggle("off", live);
  }
}

// setupChange asks what the staff that is picked should play, and changes
// that entry rather than adding another.
//
// Remove-then-add would do the same to the list and not to the score: it
// drops the staff the entry stood for, and with it the only means of
// changing an instrument without laying every staff out again.
// Picking two staves at once and naming one instrument is how a grand staff
// gets made out of a score that arrived without one: a PDF of piano music
// read two staves to a system comes in as two nameless staves, and what it
// is is one pianist. So two staves side by side, given an instrument that is
// written on two staves, become one entry and are braced when it is applied.
// Two staves given a flute stay two flutes, because two flutes are two
// players however close together they are printed.
async function setupChange() {
  const picked = setupPicked();
  if (!picked.length) {
    status("Pick a staff to change");
    return;
  }
  const staves = picked.flatMap((it) => it.staves || []);
  const v = await ask({
    title: "Instrument",
    label: staves.length
      ? "What staff " + staves.map((n) => n + 1).join(" and ") + " plays"
      : "Instrument",
    value: picked[0].sound,
    note: "Sets the clef, the transposition and the range it is checked " +
          "against, as well as what it is played with. Type to search.",
    search: true,
    options: await instrumentOptions(),
  });
  if (v === null) return;
  // Two single staves, one under the other, of an instrument that is
  // written on two: one player, and from here on one entry.
  const livePair = picked.length === 2 && grandSound(v) &&
    picked.every((it) => (it.staves || []).length === 1) &&
    staves.length === 2 && staves[1] === staves[0] + 1;
  // Over a score that already has staves, this goes to the score now.
  //
  // Everything else in this dialog does — adding a staff, taking one away,
  // moving one, taking a pair apart — so an instrument that only changed
  // the list was the one change somebody could make here, close the window
  // on, and lose, with the name beside the file never starring to say
  // there was anything to save. It is one press of undo to take back, the
  // same as the rest.
  if (setupLive() && staves.length) {
    const sound = soundChosen(v).sound;
    busy("Setting the instrument…");
    try {
      for (const n of staves) {
        const p = new URLSearchParams({ part: String(n), sound });
        const res = await fetch("/api/staff?" + p, { method: "POST" });
        if (!res.ok) {
          status("Could not set staff " + (n + 1) + ": " +
            (await res.text()).trim());
          return;
        }
      }
      // Two staves made one instrument that is written on two get the
      // brace that says so, with the barlines run through the gap.
      if (livePair) {
        const p = new URLSearchParams({
          first: String(staves[0]), last: String(staves[1]),
          symbol: "brace", barlines: "1",
        });
        await fetch("/api/group?" + p, { method: "POST" });
        // And they are no longer apart: the score itself now says they
        // are one player.
        for (const n of staves) setupApart.delete(n);
      }
    } finally {
      idle();
    }
    await setupReload();
    status((staves.length === 1 ? "Staff " : "Staves ") +
      staves.map((n) => n + 1).join(" and ") + " now play" +
      (staves.length === 1 ? "s " : " ") + instrumentNamed(v));
    return;
  }
  for (const it of picked) {
    it.sound = v;
    it.said = true;
  }

  // Two single staves, one under the other, of an instrument that is
  // written on two: one player, and from here on one entry.
  const pair = picked.length === 2 && grandSound(v) &&
    picked.every((it) => (it.staves || []).length === 1) &&
    picked[1].staves[0] === picked[0].staves[0] + 1;
  if (pair) {
    picked[0].staves = [picked[0].staves[0], picked[1].staves[0]];
    picked[0].brace = true;
    const gone = picked[1].id;
    setupChosen = setupOrdered().filter((it) => it.id !== gone);
    setupGroups = setupGroups
      .map((ids) => ids.filter((id) => id !== gone))
      .filter((ids) => ids.length > 1);
    status("Staves " + picked[0].staves.map((n) => n + 1).join(" and ") +
      " are one " + instrumentNamed(v));
  }
  drawSetupChosen();
}

// applySetupInstruments tells the staves that are already in the score what
// they play, and leaves every note where it is.
//
// One request per staff rather than one for the lot, because that is the
// request that already exists and already does the whole job — the clef,
// the transposition, the sound. A second way of saying the same thing to
// the server would be a second way for it to be said differently.
async function applySetupInstruments() {
  const list = setupInPlace();
  if (!list) {
    status("The staves have been added to or moved — make them instead");
    return;
  }
  const parts = state.info?.parts || [];
  const work = [];
  for (const it of list) {
    for (const n of it.staves) {
      // A staff already playing this is left alone: an untouched staff
      // should not come back transposed because the dialog was opened.
      if ((parts[n]?.sound || "") === it.sound) continue;
      work.push([n, it.sound]);
    }
  }
  // Any bracket drawn here goes on too, since there is a way of drawing one
  // round staves that already exist. Only the ones drawn here: this dialog
  // never read the brackets the score came with, so it is in no position to
  // take one away.
  const runs = setupGroups.map((ids) => {
    const of = list.filter((it) => ids.includes(it.id))
      .flatMap((it) => it.staves);
    return of.length > 1 ? [Math.min(...of), Math.max(...of), "bracket"] : null;
  }).filter(Boolean);
  // And a brace round each pair that has been made one instrument, with the
  // barlines run through the gap: that is what says the two staves are
  // played together rather than merely printed together.
  for (const it of list) {
    if (it.brace && it.staves.length === 2) {
      runs.push([it.staves[0], it.staves[1], "brace"]);
    }
  }

  if (!work.length && !runs.length) {
    $("#score-setup")?.close();
    status("Nothing to change — the staves already play that");
    return;
  }
  busy("Setting the instruments…");
  try {
    for (const [n, sound] of work) {
      const p = new URLSearchParams({
        part: String(n), sound: soundChosen(sound).sound,
      });
      const res = await fetch("/api/staff?" + p, { method: "POST" });
      if (!res.ok) {
        status("Could not set staff " + (n + 1) + ": " +
          (await res.text()).trim());
        return;
      }
    }
    for (const [first, last, symbol] of runs) {
      const p = new URLSearchParams({
        first: String(first), last: String(last), symbol,
      });
      if (symbol === "brace") p.set("barlines", "1");
      await fetch("/api/group?" + p, { method: "POST" });
    }
  } finally {
    idle();
  }
  $("#score-setup")?.close();
  await reload();
  status(work.length === 1
    ? "Staff " + (work[0][0] + 1) + " is now " + instrumentNamed(work[0][1])
    : work.length
      ? "Set " + work.length + " staves"
      : "Staves joined");
}

// setupOrdered is the chosen instruments in the order they will be laid
// out, which is what the box shows and what the positions mean.
function setupOrdered() {
  return $("#setup-order")?.checked ? inScoreOrder(setupChosen) : setupChosen;
}

// inScoreOrder sorts the chosen instruments the way a score is laid out.
//
// Asked of the server's own ranking rather than worked out again here: a
// second copy of the convention is a second thing to be wrong, and the
// order is already written down where the staves are made.
function inScoreOrder(chosen) {
  const rank = (value) => {
    // The value may carry how it is written — "pluck.guitar/both" — and
    // where a thing goes down the page is about the instrument, not
    // about the way it is written down.
    const { sound } = soundChosen(value);
    const i = (instrumentCatalogue || []).findIndex((x) => x.sound === sound);
    return i < 0 ? 1e9 : (instrumentCatalogue[i].scoreRank ?? 1e9);
  };
  return chosen.map((it, i) => [it, i]).sort((a, b) => {
    const d = rank(a[0].sound) - rank(b[0].sound);
    return d !== 0 ? d : a[1] - b[1]; // two of the same keep their order
  }).map(([it]) => it);
}

// setupLive says this window is working on a score that already has music
// in it, and so must not replace it.
//
// This is the lesson of a score read out of a PDF and then emptied. The
// window had two kinds of operation in it and no sign of which was which:
// Change kept the music, Add and Remove quietly built a *different*
// ensemble, and the button at the bottom laid that ensemble out over the
// top of the old one. Somebody naming eight imported staves reaches for
// Add and Remove — they are the obvious controls — and the music is gone.
//
// So over a score with music there is no second kind. Adding, removing,
// moving and bracketing go straight to the score as they are done, each
// one undoable on its own, and the list is read back from the score
// afterwards rather than kept beside it. "Make the staves" is not offered
// at all: there is no way through this window to empty a staff.
function setupLive() {
  return scoreHasMusic();
}

// setupReload reads the list back off the score, after a change that went
// straight to it.
async function setupReload() {
  await reload();
  setupChosen = setupFromScore();
  setupGroups = [];
  drawSetupChosen();
}

// setupAdd puts the instrument the cursor is on into the score.
async function setupAdd() {
  const pick = $("#setup-pick");
  if (!pick || !pick.value) return;
  if (setupLive()) {
    const { sound, written } = soundChosen(pick.value);
    const p = new URLSearchParams({ sound });
    if (written) p.set("written", written);
    const res = await fetch("/api/addstaff?" + p, { method: "POST" });
    if (!res.ok) {
      status("Could not add it: " + (await res.text()).trim());
      return;
    }
    await setupReload();
    status("Added " + instrumentNamed(pick.value));
    return;
  }
  setupChosen.push({ id: setupNextId++, sound: pick.value });
  drawSetupChosen();
}

async function setupRemove() {
  const picked = setupPicked();
  if (!picked.length) return;
  if (setupLive()) {
    // Highest first, so that taking one away does not renumber the ones
    // still to go.
    const staves = picked.flatMap((it) => it.staves || [])
      .sort((a, b) => b - a);
    if (!staves.length) return;
    const parts = state.info?.parts || [];
    const notes = staves.reduce((n, i) => n + (parts[i]?.notes || 0), 0);
    if (notes > 0) {
      // Taking away a staff that has music on it is the one thing here
      // that loses any, so it is the one thing that asks.
      const sure = await confirmThat({
        title: staves.length === 1 ? "Remove this staff?" : "Remove these staves?",
        note: "There " + (notes === 1 ? "is a note" : "are " + notes + " notes") +
              " on " + (staves.length === 1 ? "it" : "them") +
              ". Undo brings it back.",
        ok: "Remove",
      });
      if (!sure) return;
    }
    for (const n of staves) {
      const res = await fetch("/api/removestaff?part=" + n, { method: "POST" });
      if (!res.ok) {
        status("Could not remove it: " + (await res.text()).trim());
        break;
      }
    }
    await setupReload();
    return;
  }
  const gone = new Set(picked.map((it) => it.id));
  setupChosen = setupOrdered().filter((it) => !gone.has(it.id));
  // And out of whatever groups they were in, so a group does not go on
  // holding a staff that is no longer in the score.
  setupGroups = setupGroups
    .map((ids) => ids.filter((id) => !gone.has(id)))
    .filter((ids) => ids.length > 1);
  drawSetupChosen();
}

// setupPicked is what is selected in the chosen list, as entries.
function setupPicked() {
  const box = $("#setup-chosen");
  if (!box) return [];
  const list = setupOrdered();
  return [...box.selectedOptions]
    .map((o) => list[Number(o.value)]).filter(Boolean);
}

// setupMove moves what is picked up or down the list.
//
// Moving by hand is arranging the score, so it turns the automatic order
// off: the two cannot both be in charge, and the one somebody just used
// their hands on is the one that wins.
async function setupMove(dir) {
  const box = $("#setup-chosen");
  if (!box || box.selectedIndex < 0) return;
  if (setupLive()) {
    await setupMoveStaves(box.selectedIndex, dir);
    return;
  }
  setupArrangedByHand();
  const i = box.selectedIndex;
  const j = i + dir;
  if (j < 0 || j >= setupChosen.length) return;
  [setupChosen[i], setupChosen[j]] = [setupChosen[j], setupChosen[i]];
  drawSetupChosen();
  box.selectedIndex = j;
}

// setupMoveStaves moves one entry past the one beside it, in the score
// itself.
//
// A staff is moved by taking it out and putting it back somewhere else, so
// a pair of staves is two of those, and the order they are done in matters:
// going up, the top staff of the pair goes first and the rest follow it;
// going down, the bottom goes first. Done the other way round, the pair
// comes out inside out.
async function setupMoveStaves(at, dir) {
  const list = setupOrdered();
  const me = list[at];
  const other = list[at + dir];
  if (!me || !other || !me.staves || !other.staves) return;
  const n = me.staves.length;
  const from = me.staves[0];
  const to = dir < 0 ? other.staves[0] : from + other.staves.length;
  const steps = [];
  for (let i = 0; i < n; i++) steps.push([from + i, to + i]);
  if (dir > 0) steps.reverse();
  for (const [f, t] of steps) {
    const res = await fetch(`/api/movestaff?part=${f}&to=${t}`,
      { method: "POST" });
    if (!res.ok) {
      status("Could not move it: " + (await res.text()).trim());
      break;
    }
  }
  await setupReload();
  const box = $("#setup-chosen");
  if (box) box.selectedIndex = at + dir;
}

// setupArrangedByHand freezes the order somebody is about to rearrange.
//
// Moving by hand turns the automatic order off: the two cannot both be in
// charge, and the one somebody just used their hands on is the one that
// wins.
function setupArrangedByHand() {
  const order = $("#setup-order");
  if (order?.checked) {
    setupChosen = inScoreOrder(setupChosen);
    order.checked = false;
  }
}

// setupBracket draws a bracket round the staves that are selected.
//
// A bracket is a vertical line down the left of a run of staves, so it
// covers everything between its ends — there is no bracketing the first
// and the third and not the second. Staves picked apart from each other
// are therefore brought together first, under the topmost of them, which
// is what somebody picking them apart is asking for. It is said out loud
// rather than done quietly, because it moves the score about.
async function setupBracket() {
  const picked = setupPicked();
  if (picked.length < 2) {
    status("Pick two staves or more to bracket them");
    return;
  }
  if (setupLive()) {
    const of = picked.flatMap((it) => it.staves || []).sort((a, b) => a - b);
    // A bracket is a line down the side of a run, so the staves have to be
    // a run. In the score itself they are where they are, and moving them
    // to suit a bracket is a bigger thing than drawing one — so it is said
    // rather than done.
    if (of.some((n, i) => i > 0 && n !== of[i - 1] + 1)) {
      status("Those staves are not next to each other — move them together first");
      return;
    }
    const p = new URLSearchParams({
      first: String(of[0]), last: String(of[of.length - 1]),
      symbol: "bracket",
    });
    const res = await fetch("/api/group?" + p, { method: "POST" });
    if (!res.ok) {
      status("Could not bracket them: " + (await res.text()).trim());
      return;
    }
    await setupReload();
    status("Staves " + (of[0] + 1) + "–" + (of[of.length - 1] + 1) + " bracketed");
    return;
  }
  setupArrangedByHand();

  const ids = picked.map((it) => it.id);
  const want = new Set(ids);
  const at = setupChosen.findIndex((it) => want.has(it.id));
  const moved = setupChosen.filter((it) => want.has(it.id));
  const rest = setupChosen.filter((it) => !want.has(it.id));
  const apart = moved.some((it, i) =>
    setupChosen.indexOf(it) !== setupChosen.indexOf(moved[0]) + i);

  setupChosen = rest.slice(0, at).concat(moved, rest.slice(at));
  // Out of any group they were in before: a staff is in one bracket.
  setupGroups = setupGroups
    .map((g) => g.filter((id) => !want.has(id)))
    .filter((g) => g.length > 1);
  setupGroups.push(ids);

  drawSetupChosen();
  status(apart
    ? "Bracketed, and brought together — a bracket covers the staves between its ends"
    : "Bracketed");
}

// setupUngroup takes the selected staves apart: out of their brackets, and
// out of each other.
//
// An entry reading "2+3" is two staves listed as one instrument, and
// somebody who wants the third staff to be an oboe has to get it onto a
// line of its own first. That is the same word for both, and it was only
// doing the bracket — so on a pair it appeared to do nothing at all, and
// on a braced pair it took the brace away and read the pair back exactly
// as it was.
async function setupUngroup() {
  const picked = setupPicked();
  if (!picked.length) return;
  // Every staff but the first of each entry begins its own entry from
  // here on, whatever the score braces or the reading would make of it.
  const split = picked.filter((it) => (it.staves || []).length > 1);
  for (const it of split) for (const n of it.staves.slice(1)) setupApart.add(n);
  if (setupLive()) {
    const of = picked.flatMap((it) => it.staves || []).sort((a, b) => a - b);
    if (!of.length) return;
    const p = new URLSearchParams({
      first: String(of[0]), last: String(of[of.length - 1]), symbol: "none",
    });
    const res = await fetch("/api/group?" + p, { method: "POST" });
    if (!res.ok) {
      status("Could not ungroup them: " + (await res.text()).trim());
      return;
    }
    await setupReload();
    status(split.length ? "Taken apart, one entry a staff"
                        : "Taken out of the bracket");
    return;
  }
  const want = new Set(picked.map((it) => it.id));
  setupGroups = setupGroups
    .map((g) => g.filter((id) => !want.has(id)))
    .filter((g) => g.length > 1);
  // And a two-staff entry becomes one entry a staff, in its place in the
  // list, each still playing what the entry played.
  if (split.length) {
    setupChosen = setupChosen.flatMap((it) =>
      split.includes(it)
        ? it.staves.map((n, i) => i === 0
            ? { ...it, staves: [n] }
            : { ...it, id: setupNextId++, staves: [n] })
        : [it]);
  }
  drawSetupChosen();
  status(split.length ? "Taken apart, one entry a staff"
                      : "Taken out of the bracket");
}

async function applyScoreSetup() {
  if (!setupChosen.length) {
    status("Choose an instrument or two");
    return;
  }
  if (scoreHasMusic()) {
    const sure = await confirmThat({
      title: "Make the staves again?",
      note: "This score has music in it, and laying it out again throws " +
            "it away.",
      ok: "Make them",
    });
    if (!sure) return;
  }
  const list = setupOrdered();
  const p = new URLSearchParams({
    instruments: list.map((it) => it.sound).join(","),
  });
  // The server orders them unless the dialog already has.
  if (!$("#setup-order")?.checked) p.set("order", "keep");
  // The groups, as runs over the list being sent. They are contiguous by
  // construction — bracketing brings the staves together — so a run is
  // all that has to go across.
  const runs = setupGroups.map((ids) => {
    const at = list.findIndex((it) => ids.includes(it.id));
    return at < 0 ? null : at + "-" + (at + ids.length - 1);
  }).filter(Boolean);
  if (runs.length) p.set("brackets", runs.join(","));
  if (scoreHasMusic()) p.set("replace", "1");

  const res = await fetch("/api/setup?" + p, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  const d = await res.json();
  $("#score-setup")?.close();
  await reload();
  status(d.staves === 1 ? "Made 1 staff" : "Made " + d.staves + " staves");
}

function setupScoreSetupDialog() {
  $("#setup-add")?.addEventListener("click", () => void setupAdd());
  $("#setup-remove")?.addEventListener("click", () => void setupRemove());
  $("#setup-up")?.addEventListener("click", () => void setupMove(-1));
  $("#setup-bracket")?.addEventListener("click", () => void setupBracket());
  $("#setup-ungroup")?.addEventListener("click", () => void setupUngroup());
  $("#setup-down")?.addEventListener("click", () => void setupMove(1));
  $("#setup-order")?.addEventListener("change", drawSetupChosen);
  $("#setup-apply")?.addEventListener("click", () => void applyScoreSetup());
  $("#setup-change")?.addEventListener("click", () => void setupChange());
  $("#setup-instruments")?.addEventListener("click",
    () => void applySetupInstruments());
  // What the piece is called is the same window it has always been, opened
  // over this one: the staves picked here are still picked when it closes.
  // Not a data-action, which would put a menu icon in the middle of a row
  // of plain buttons.
  $("#setup-about")?.addEventListener("click", () => void setHeaderDialog());
  // Double-clicking an instrument is how everybody expects to add one.
  $("#setup-pick")?.addEventListener("dblclick", () => void setupAdd());
  // And double-clicking a staff is how everybody expects to change it.
  $("#setup-chosen")?.addEventListener("dblclick", () => void setupChange());
}

/* ---------- tablature ---------- */

// isTabStaff reports whether a staff is written as tab rather than as a
// staff of pitches.
function isTabStaff(part) {
  return !!state.info?.parts?.[part]?.tab;
}

// tabWithTuning is the staff that carries the tuning for the one named.
//
// Itself where it is a tablature, and otherwise the staff that mirrors it
// and is one. A guitar written as a staff of pitches with a tab under it is
// one instrument on two staves, and the cursor is usually on the half with
// the notes on it — so anything asked of the instrument has to find the
// other half rather than refuse.
function tabWithTuning(part) {
  const parts = state.info?.parts || [];
  if (parts[part]?.tab) return part;
  for (let i = 0; i < parts.length; i++) {
    if (parts[i].tab && parts[i].mirrorOf === part) return i;
  }
  return -1;
}

/* ---------- moving a tab ---------- */

// tabMoveDialog asks how far a fretted part moves, and which of the two
// things that means.
//
// The music moving and the instrument moving are both "put it up a tone"
// and they are different edits: one changes the notes and leaves the
// instrument alone, the other changes the instrument and leaves the hand
// alone. Nothing in the request says which, so it is asked.
async function tabMoveDialog() {
  const at = state.selection?.part ?? 0;
  const tabAt = tabWithTuning(at);
  if (tabAt < 0) {
    status("That staff is not written as tablature");
    return;
  }
  const staff = state.info?.parts?.[tabAt];
  const dlg = $("#tabmove");
  $("#tabmove-where").textContent = staff?.name
    ? `${staff.name} — ${staff.tab.strings.length} strings.`
    : "This staff.";
  const showTuning = () => {
    $("#tabmove-tuning").textContent =
      "Tuned to " + (staff.tab.strings || []).join(" ") + ".";
  };
  showTuning();

  const answer = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  if (answer !== "ok") {
    status("Ready");
    return;
  }

  let semitones = Number($("#tabmove-interval").value);
  if ($$('#tabmove-form input[name="tabmove-dir"]').find((r) => r.checked)
      ?.value === "down") {
    semitones = -semitones;
  }
  const how = $$('#tabmove-form input[name="tabmove-how"]')
    .find((r) => r.checked)?.value;

  if (how === "tuning") {
    const p = new URLSearchParams({
      part: String(tabAt), semitones: String(semitones),
    });
    const res = await fetch("/api/retune?" + p, { method: "POST" });
    if (!res.ok) {
      status("Could not retune: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    await refreshScore();
    status(`Retuned to ${d.strings.join(" ")} — the fingering is unchanged`);
    return;
  }

  // The music moves, which is ordinary transposition: it works out the
  // fingering again afterwards, so the tab follows.
  //
  // The semitones alone, because the interval is spelled where the move is
  // made. A caller that gives only the distance gets the ordinary reading
  // of it — four semitones is a major third, not a diminished fourth — and
  // working that out a second time here would be a second place for it to
  // be wrong.
  await transposeBy(null, semitones, { diatonic: false, key: true });
}

/* ---------- the chord builder ---------- */

// A chord on a fretted instrument is a shape, not six pitches.
//
// Typed one string at a time it is six frets on six different lines with the
// caret moved between every one of them, and the shape — the thing actually
// being written down — never appears anywhere. So this is the shape: a grid
// of strings and frets to click at, and the chord it turns out to be
// underneath, which is how you find out you have built an F sharp minor
// seventh rather than having to know it first.

// chordShape holds what the builder is showing: one entry a string, lowest
// string first, each a fret number or null for a string that is not played.
let chordShape = null;

// fretboardSpan is how many frets are shown at once. Four frets and the nut
// is what a chord diagram shows, because that is as far as a hand reaches.
const fretboardSpan = 4;

async function chordBuilderDialog() {
  const at = state.selection?.part ?? 0;
  const tabAt = tabWithTuning(at);
  if (tabAt < 0) {
    status("Chord builder is for a staff written as tablature");
    return;
  }
  const staff = state.info.parts[tabAt];
  const strings = staff.tab.strings;
  const dlg = $("#fretboard");

  $("#fretboard-where").textContent =
    `${staff.tab.strings.length} strings, tuned ${strings.join(" ")}.`;
  chordShape = new Array(strings.length).fill(null);
  $("#fretboard-at").value = "0";
  $("#fretboard-at").max = String(Math.max(0, (staff.tab.frets || 24) - fretboardSpan));
  $("#fretboard-shape").value = "";
  $("#fretboard-name").textContent = "\u00a0";

  // The grid and the written shape are two views of one thing, so each
  // redraws the other rather than keeping a copy.
  $("#fretboard-at").oninput = () => drawFretboard(strings, tabAt);
  $("#fretboard-shape").oninput = () => {
    readShapeText(strings.length);
    drawFretboard(strings, tabAt);
    nameShape(tabAt);
  };
  drawFretboard(strings, tabAt);

  const answer = await new Promise((resolve) => {
    const done = () => {
      // Clear is a button inside the form, so it closes the dialog; what
      // it means is empty the grid and carry on. Opening it again is left
      // to the next turn of the loop, because showModal on a dialog that
      // is in the middle of closing is not reliably allowed.
      if (dlg.returnValue === "clear") {
        chordShape = chordShape.map(() => null);
        showShapeText();
        drawFretboard(strings, tabAt);
        nameShape(tabAt);
        setTimeout(() => dlg.showModal(), 0);
        return;
      }
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
  if (answer !== "ok") {
    status("Ready");
    return;
  }
  await writeFretChord(at);
}

// shapeText is the shape as a chord diagram writes it: one field a string,
// lowest first, x for a string that is not played.
//
// Separated by commas rather than run together whenever a fret runs past
// nine, because "x0221000" cannot say a tenth fret and a chord high on the
// neck is not unusual.
function shapeText() {
  if (!chordShape) return "";
  const wide = chordShape.some((f) => f !== null && f > 9);
  const one = (f) => (f === null ? "x" : String(f));
  return wide ? chordShape.map(one).join(",") : chordShape.map(one).join("");
}

function showShapeText() {
  $("#fretboard-shape").value = shapeText();
}

// readShapeText takes what was typed back into the grid, so the two agree.
// Anything that is not a shape yet leaves the grid as it was: somebody
// halfway through typing one has not made a mistake.
function readShapeText(lines) {
  const raw = $("#fretboard-shape").value.trim();
  const fields = /[ ,-]/.test(raw) ? raw.split(/[ ,-]+/).filter(Boolean)
                                   : raw.split("");
  if (fields.length !== lines) return;
  const next = fields.map((f) => {
    if (/^[xX.]$/.test(f)) return null;
    const n = Number(f);
    return Number.isInteger(n) && n >= 0 ? n : undefined;
  });
  if (next.some((f) => f === undefined)) return;
  chordShape = next;
}

// drawFretboard draws the neck: a row for each string, a column for each
// fret in view, and the dots where the fingers are.
//
// String one at the top, the way the tab staff itself is drawn and the way
// the player is already reading. The shape is written lowest-string-first,
// which is the other way about, so the rows are walked in reverse — that
// turning-round happens here and once.
function drawFretboard(strings, tabAt) {
  const grid = $("#fretboard-grid");
  const from = Math.max(0, Number($("#fretboard-at").value) || 0);
  const lines = strings.length;
  const cols = fretboardSpan + 1;

  const rows = [];
  for (let row = 0; row < lines; row++) {
    const str = row + 1;                 // string one at the top
    const idx = lines - str;             // its place in the shape
    const fret = chordShape[idx];
    const cells = [];
    // Open or muted, in front of the nut, which is where a diagram puts it.
    cells.push(`<button type="button" class="fb-open${fret === 0 ? " on" : ""}"
       data-str="${idx}" data-fret="0" title="Open">o</button>`);
    cells.push(`<button type="button" class="fb-mute${fret === null ? " on" : ""}"
       data-str="${idx}" data-fret="x" title="Not played">x</button>`);
    for (let c = 1; c <= fretboardSpan; c++) {
      const f = from + c;
      cells.push(`<button type="button" class="fb-cell${fret === f ? " on" : ""}"
         data-str="${idx}" data-fret="${f}"></button>`);
    }
    rows.push(`<div class="fb-row"><span class="fb-name">${
      escapeHTML(strings[row])}</span>${cells.join("")}</div>`);
  }
  // The fret numbers along the bottom, so a shape up the neck says where.
  const foot = ['<div class="fb-row fb-feet"><span class="fb-name"></span>',
                '<span class="fb-open"></span><span class="fb-mute"></span>'];
  for (let c = 1; c <= fretboardSpan; c++) {
    foot.push(`<span class="fb-cell">${from + c}</span>`);
  }
  foot.push("</div>");
  grid.innerHTML = rows.join("") + foot.join("");

  $$("#fretboard-grid button").forEach((b) => {
    b.onclick = () => {
      const idx = Number(b.dataset.str);
      const was = chordShape[idx];
      const f = b.dataset.fret === "x" ? null : Number(b.dataset.fret);
      // Clicking what is already there takes it off, so a finger put down
      // by mistake comes up again with a second click rather than needing
      // the string muted and un-muted.
      chordShape[idx] = (was === f) ? null : f;
      showShapeText();
      drawFretboard(strings, tabAt);
      nameShape(tabAt);
    };
  });
}

// nameShape asks what the shape is called and shows it.
//
// Asked of the program rather than worked out here, because the program
// already knows: the same naming the analysis window uses, so the builder
// and the analysis cannot disagree about what a shape is.
async function nameShape(tabAt) {
  const label = $("#fretboard-name");
  const frets = shapeText();
  if (!chordShape || chordShape.every((f) => f === null)) {
    label.textContent = "\u00a0";
    return;
  }
  const p = new URLSearchParams({
    part: String(tabAt), measure: String(state.selection?.measure ?? 0),
    frets, preview: "1",
  });
  const res = await fetch("/api/fretchord?" + p, { method: "POST" });
  if (!res.ok) {
    label.textContent = (await res.text()).trim();
    return;
  }
  const d = await res.json();
  label.textContent = d.name
    ? `${d.name} — ${d.notes.join(" ")}`
    : `No name for it — ${d.notes.join(" ")}`;
}

// writeFretChord puts the shape into the music at the cursor.
async function writeFretChord(part) {
  const sel = state.selection;
  if (!sel) {
    status("Click the bar to put it in first");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at),
    frets: shapeText(), dur: state.tool.duration,
  });
  if (state.tool.dots) p.set("dots", String(state.tool.dots));
  const res = await fetch("/api/fretchord?" + p, { method: "POST" });
  if (!res.ok) {
    status("Could not write it: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  // The caret goes past what was written, into whichever bar it landed in,
  // the way a single fret does.
  sel.measure = d.measure ?? sel.measure;
  sel.at = (d.at ?? 0) + 1;
  sel.ref = null;
  sel.wrote = true;
  state.multi = [];
  state.anchor = null;
  state.focus = null;
  await refreshScore();
  carryCaretOn(sel);
  markSelection();
  scrollSelectionIntoView();
  status(d.name ? "Wrote " + d.name : "Wrote the chord");
}

// tabDialog chooses the tuning a staff is written in, or takes the tab
// away.
//
// A tablature is a tuning and nothing else: the same 3 on the same line is
// a G on a guitar and a C on a ukulele, so there is no such thing as a tab
// staff without one and the question cannot be skipped.
async function tabDialog() {
  const part = state.selection?.part ?? 0;
  const staff = state.info?.parts?.[part];
  const tunings = await (await fetch("/api/tunings")).json();

  const options = [{ value: "off", label: "a staff of pitches" }].concat(
    tunings.map((t) => ({
      value: t.name,
      // The strings, because that is what tells two tunings apart, and
      // somebody choosing between drop D and DADGAD is choosing by them.
      label: `${t.name} — ${t.strings.join(" ")}`,
    })));

  const now = staff?.tab?.tuning;
  const v = await ask({
    title: staff ? `${staff.name} as tablature` : "Tablature",
    label: "Written as",
    value: now && now !== "custom" ? now : "guitar",
    note: "The notes keep their pitches either way — a tab is how the " +
          "music is written, not what it is. Type a fret number to write " +
          "a note on the line the cursor is on.",
    search: true,
    options,
  });
  if (v === null) return;

  const res = await fetch(`/api/staff?part=${part}&tab=${encodeURIComponent(v)}`,
                          { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  await refreshScore();
  status(v === "off" ? "Written on a staff of pitches" : "Written as " + v);
}

// moveToString moves the selected note a string along the neck, keeping
// what it sounds.
//
// The same pitch is in several places and they do not finger alike: the
// open B string and the fourth fret of the G are the same note and a
// different thing to play. Which one the music wants is a decision, and
// this is where it is made.
async function moveToString(dir) {
  const sel = state.selection;
  if (!sel || sel.index == null) {
    status("Pick a note first");
    return;
  }
  if (!isTabStaff(sel.part)) {
    status("That staff is not written as tablature");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure),
    at: String(sel.index), move: dir,
  });
  const res = await fetch("/api/restring?" + p, { method: "POST" });
  if (!res.ok) {
    status((await res.text()).trim());
    return;
  }
  const d = await res.json();
  await refreshScore();
  status(`String ${d.string}, fret ${d.fret}`);
}

// typing holds the fret being typed, while it is being typed.
//
// Frets run past nine and are typed a digit at a time, so "1" then "2" is
// the twelfth fret and not the first followed by the second. The only
// thing that can tell them apart is whether the player is still typing,
// which is what this remembers: a digit arriving soon after another, on
// the same note, extends it instead of starting a new one.
let fretTyping = null;
const fretTypingWindow = 900;

// typeFret writes the digit the player typed onto the line the cursor is
// on.
async function typeFret(digit) {
  const sel = state.selection;
  if (!sel) return false;
  const str = stringAtCursor(sel);
  if (!str) return false;

  const now = Date.now();
  const extending = fretTyping &&
    now - fretTyping.at < fretTypingWindow &&
    fretTyping.part === sel.part && fretTyping.measure === sel.measure &&
    fretTyping.string === str;

  const fret = extending ? fretTyping.fret * 10 + digit : digit;
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure),
    string: String(str), fret: String(fret), dur: state.tool.duration,
  });
  if (state.tool.dots) p.set("dots", String(state.tool.dots));
  if (extending) {
    p.set("at", String(fretTyping.index));
    p.set("replace", "1");
  } else {
    // At the caret, the way a note typed on a staff of pitches goes in
    // at the caret. Appending to the end of the bar instead meant the
    // caret said nothing about where the next fret would land, so there
    // was no way to put one in between two others.
    p.set("at", String(sel.at));
  }
  const res = await fetch("/api/fret?" + p, { method: "POST" });
  if (!res.ok) {
    // A two-digit fret that does not exist is almost always a first digit
    // the player has finished with — "1" then "5" on a twelve-fret neck —
    // so the second digit starts again rather than being refused.
    if (extending) {
      fretTyping = null;
      return typeFret(digit);
    }
    status((await res.text()).trim());
    return true;
  }
  const d = await res.json();
  // Where it actually landed, which is not always the bar it was aimed
  // at: a bar with no room left in it passes the note to the next one.
  const landed = d.measure == null ? sel.measure : d.measure;
  fretTyping = {
    at: now, part: sel.part, measure: landed,
    string: str, fret, index: d.at,
  };

  // The caret goes past what was just written, the way it does when a
  // note is typed on a staff of pitches.
  //
  // It used to stay where it was, so the next digit landed on top of the
  // last one and the only way to write a second fret was to move the
  // caret by hand first. A second digit arriving while the player is
  // still typing is the one case that means the note already written —
  // that is the twelfth fret rather than a first and a second — and it
  // has its own index to aim at, so moving on does not disturb it.
  if (!extending) {
    sel.measure = landed;
    sel.at = d.at + 1;
    sel.ref = null;
    // Selected, not picked up: the arrows should move the caret from
    // string to string, not drag the note that was just written.
    sel.wrote = true;
    state.multi = [];
    state.anchor = null;
    state.focus = null;
  }
  await refreshScore();
  if (!extending) {
    // A bar with no room left in it is no place to leave the caret: the
    // next fret goes into the bar after, and that is where the caret
    // belongs. It is read after the refresh, because it asks the bar how
    // full it is and the answer it had before was one note out of date.
    carryCaretOn(sel);
  }
  markSelection();
  scrollSelectionIntoView();
  return true;
}

// stringAtCursor is which string the cursor is sitting on, counted from
// the top line of the tab down.
//
// The cursor's step is in half spaces from the bottom line, which is how
// the whole editor measures height; a tab's strings are its lines, so the
// string is which line that lands on, counted the other way about.
function stringAtCursor(sel) {
  const tab = state.info?.parts?.[sel.part]?.tab;
  if (!tab) return 0;
  const lines = tab.strings.length;
  const step = sel.step ?? 0;
  // Counted down from the top line, because that is where the engraver
  // anchors it: string one is the top line of a tab whatever the
  // instrument, and each string below it is a line — two half spaces —
  // lower. So the sum does not mention how many strings there are, and
  // that is the tell. Counting up from the bottom instead looks like the
  // same sum and is not: it puts string one on the top line only when the
  // tab happens to have as many lines as stepAt's zero is spaces above
  // the bottom, which for a six-string guitar it does not.
  //
  // What that cost: every string came out one too high, so a fret typed
  // on the line the cursor was sitting on was written on the line below
  // it, and the bottom string could not be reached at all — the step it
  // sits at came out as one string past the end of the neck.
  const fromTop = Math.round(STAFF_HEIGHT - step / 2);
  const str = 1 + fromTop;
  if (str < 1 || str > lines) return 0;
  return str;
}

// addGrandStaff adds the two staves a pianist reads, joined by a brace with
// the barlines running through — which is what makes them one instrument on
// the page rather than two next to each other.
async function addGrandStaff() {
  const at = state.info?.parts?.length ?? 0;
  for (const clef of ["treble", "bass"]) {
    const p = new URLSearchParams({ instrument: "piano", clef, name: "Piano" });
    const res = await fetch("/api/addstaff?" + p.toString(), { method: "POST" });
    if (!res.ok) {
      status("Could not add it: " + (await res.text()).trim());
      return;
    }
  }
  const g = new URLSearchParams({
    first: String(at), last: String(at + 1),
    symbol: "brace", barlines: "1", name: "Piano",
  });
  await fetch("/api/group?" + g.toString(), { method: "POST" });
  await reload();
  status("Added a grand staff");
}

// toggleStaffShown takes the staff the cursor is on out of the drawing, or
// puts it back. The music stays either way, and so does the sound.
async function toggleStaffShown() {
  const part = state.selection?.part ?? 0;
  const now = !!state.info?.parts?.[part]?.hidden;
  const res = await fetch(`/api/staff?part=${part}&hidden=${now ? 0 : 1}`,
                          { method: "POST" });
  if (!res.ok) {
    status("Could not change it: " + (await res.text()).trim());
    return;
  }
  await reload();
  status(now ? "Staff shown" : "Staff hidden — it still plays");
}

async function showAllStaves() {
  const parts = state.info?.parts || [];
  let n = 0;
  for (let i = 0; i < parts.length; i++) {
    if (!parts[i].hidden) continue;
    await fetch(`/api/staff?part=${i}&hidden=0`, { method: "POST" });
    n++;
  }
  if (n === 0) {
    status("Every staff is already shown");
    return;
  }
  await reload();
  status(`Showing ${n} staff${n === 1 ? "" : "s"} again`);
}

async function removeStaff() {
  const parts = state.info?.parts || [];
  const part = state.selection?.part ?? 0;
  if (parts.length <= 1) {
    status("A score has to have a staff");
    return;
  }
  // Named where it has a name, and numbered where it has not: a staff
  // somebody is about to lose should be said back to them, and "staff 4" is
  // a better answer than nothing.
  const name = parts[part]?.name || `staff ${part + 1}`;
  const sure = await confirmThat({
    title: "Remove this staff?",
    note: `This takes away ${name} and the music on it. Undo brings it back.`,
    ok: "Remove",
  });
  if (!sure) return;
  const res = await fetch("/api/removestaff?part=" + part, { method: "POST" });
  if (!res.ok) {
    status("Could not remove it: " + (await res.text()).trim());
    return;
  }
  await reload();
  status(`Removed ${name} — Undo brings it back`);
}

async function moveStaff(delta) {
  const from = state.selection?.part ?? 0;
  const to = from + delta;
  const n = state.info?.parts?.length ?? 0;
  if (to < 0 || to >= n) {
    status(delta < 0 ? "That staff is already at the top" : "Already at the bottom");
    return;
  }
  const res = await fetch(`/api/movestaff?part=${from}&to=${to}`, { method: "POST" });
  if (!res.ok) {
    status("Could not move it: " + (await res.text()).trim());
    return;
  }
  if (state.selection) state.selection.part = to;
  await refreshScore();
  status("Staff moved");
}

// setMeasureRepeat marks the bar as a repeat of the one before it.
async function setMeasureRepeat(bars) {
  const sel = state.selection;
  if (!sel) {
    status("Click the bar first");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), bars: String(bars),
  });
  const res = await fetch("/api/measurerepeat?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(bars === 0 ? "Bar written out"
                    : bars === 1 ? "Repeats the bar before"
                                 : `Repeats the ${bars} bars before`);
}

/* ---------- key signature picker ---------- */

// The picker shows each signature engraved on a staff rather than named, which
// is how gscore 0.0.9 did it with its treble_*.xpm pictures, and how Sibelius
// does it. You recognise two sharps faster than you decode the words "D major".
let keyOptions = null;

async function openKeyPicker(partIndex, at) {
  keyOptions = keyOptions || (await (await fetch("/api/keysignatures")).json());
  const staff = state.info?.parts?.[partIndex];
  const clef = staff?.clef || "treble";

  // Where it takes effect. A key set from bar nine looks exactly like a key
  // that did nothing, unless something says which bar it was.
  const measure = at !== undefined ? at : (state.selection?.measure ?? 0);
  $("#keysig-where").textContent = measure > 0
    ? `From measure ${barNumberOf(measure)} to the end, on every staff.`
    : "For the whole piece, on every staff.";

  // What the staff is in now: where the slider starts, and what a change at
  // this bar would be a change *from*, which is what decides the naturals.
  const from = keyFifthsOf(staff, measure);

  // The pictures are fetched and put straight into the page rather than
  // loaded as images. An SVG in an <img> is an isolated document: it cannot
  // reach /font.otf, so every glyph in it comes out as a missing-character
  // box. Inline, it uses the page's own font.
  //
  // All fifteen at once, because sliding along them has to be instant: a
  // picture that arrives after the slider has moved on is a flicker.
  const drawings = await Promise.all(
    keyOptions.map((k) => {
      const q = new URLSearchParams({
        fifths: String(k.fifths), clef, space: "11",
      });
      // At a bar partway through, the picture is of the change: the naturals
      // that cancel what is no longer sharp or flat, then the new key.
      if (measure > 0 && from !== null) q.set("from", String(from));
      return fetch("/keysig.svg?" + q.toString()).then((r) => r.text());
    })
  );

  const slider = $("#keysig-slider");
  const show = (fifths) => {
    const i = keyOptions.findIndex((k) => k.fifths === fifths);
    if (i < 0) return;
    $("#keysig-art").innerHTML = drawings[i];
    const k = keyOptions[i];
    $("#keysig-name").textContent = `${k.major} major or ${k.minor.replace("m", "")} minor`;
  };
  slider.value = String(from ?? 0);
  show(Number(slider.value));
  slider.oninput = () => show(Number(slider.value));

  // Offer to take a change away only where there is one.
  const hit = state.hits.find(
    (h) => h.part === partIndex && h.measure === measure);
  const remove = $("#keysig-remove");
  if (remove) remove.hidden = !(measure > 0 && hit?.keyChange);

  const dlg = $("#keysig");
  const result = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.returnValue = "";
    dlg.showModal();
    slider.focus();
  });

  if (result === "remove") {
    const q = new URLSearchParams({
      part: String(partIndex), at: String(measure), remove: "1",
    });
    const res = await fetch("/api/setkey?" + q.toString(), { method: "POST" });
    if (!res.ok) {
      status((await res.text()).trim());
      return;
    }
    await reload();
    status(`The key change at measure ${barNumberOf(measure)} is gone`);
    return;
  }
  if (result !== "ok") return;

  const fifths = Number(slider.value);
  const q = new URLSearchParams({
    part: String(partIndex), fifths: String(fifths),
  });
  if (measure > 0) q.set("at", String(measure));
  const res = await fetch("/api/setkey?" + q.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set the key: " + (await res.text()).trim());
    return;
  }
  await reload();
  const named = keyOptions.find((k) => k.fifths === fifths);
  status(measure > 0
    ? `${named ? named.major + " major" : "Key"} from measure ${barNumberOf(measure)}, on every staff`
    : `${named ? named.major + " major" : "Key"}, on every staff`);
}

// keyFifthsOf reads how many sharps or flats a staff has at a bar, out of the
// signature the info endpoint spells for it — "3\u266f", "2\u266d", "\u266e" for none.
//
// It is the written signature of that staff, not the concert key. For the
// picker that is the right one to open on: it is what the player in front of
// you is reading.
function keyFifthsOf(staff, measure) {
  const text = staff?.key;
  if (!text) return 0;
  if (text.startsWith("\u266e")) return 0;
  const n = parseInt(text, 10);
  if (!Number.isFinite(n)) return 0;
  if (text.includes("\u266d")) return -n;
  if (text.includes("\u266f")) return n;
  void measure;
  return 0;
}

/* ---------- printing ---------- */

// Printing always paginates, whatever the editor is showing: a continuous
// strip has no page breaks to print at. The width is chosen so a system fits
// the usable part of a portrait page.
// printScore asks what to print, then hands the page to the system's own
// print dialog, where the paper, the margins and "Save as PDF" already live.
async function printScore() {
  const many = (state.info?.tunes?.length ?? 0) > 1;
  const parts = (state.info?.parts?.length ?? 0) > 1;
  const choice = await askPrint(many, parts, "Print", "Print");
  if (!choice) return;

  const path = "/print?what=" + encodeURIComponent(choice.what) +
    (choice.multiRests ? "&multirest=1" : "") +
    (choice.concert ? "&concert=1" : "");

  // A native window has no print machinery: window.print() does nothing and
  // window.open answers with nothing, so none of what follows can work in
  // there.
  if (state.info?.local && state.info?.native) {
    // Where the application can print for us, it does, and it prints the
    // PDF rather than this page: the engraver has already cut those pages
    // to the paper, so the panel's preview is the sheet that comes out.
    if (window.gscoreNativePrint && window.webkit?.messageHandlers?.gscore) {
      window.webkit.messageHandlers.gscore.postMessage("print:/api/pdf?what=" +
        encodeURIComponent(choice.what) +
        (choice.multiRests ? "&multirest=1" : "") +
        (choice.concert ? "&concert=1" : ""));
      status("Opening the print panel…");
      return;
    }
    // And where it cannot — an older application than this page — the
    // pages go to the real browser, which has both. It prints itself.
    const res = await fetch("/api/openbrowser?path=" + encodeURIComponent(path),
                            { method: "POST" });
    status(res.ok ? "Opened the print view in your browser"
                  : "Could not open the print view");
    return;
  }

  status("Preparing to print…");
  const res = await fetch(path);
  if (!res.ok) {
    status("Could not prepare the print view");
    return;
  }
  const page = await res.text();

  const w = window.open("", "_blank");
  if (!w) {
    status("The browser blocked the print window");
    return;
  }
  // The page prints itself once its font has loaded, which is a decision the
  // server already made; writing it whole keeps that in one place.
  w.document.open();
  w.document.write(page);
  w.document.close();
  w.focus();
  status("Ready");
}

// askPrint shows the choices that apply to this file and nothing else.
//
// It answers with an object rather than a string because the paper is a
// second question: gscore lays the PDF out itself, so turning the sheet on
// its side is something it has to be told. Printing does not ask — the paper
// there belongs to the system's own dialog, which opens next.
function askPrint(many, parts, title, verb, paper) {
  const dlg = $("#print");
  dlg.querySelector("h2").textContent = title || "Print";
  dlg.querySelector(".primary").textContent = verb || "Print";
  $("#print-book-row").hidden = !many;
  $("#print-parts-row").hidden = !parts;
  $("#print-paper").hidden = !paper;
  $("#print-note").textContent = paper
    ? "Choose what to export. Gscore engraves the pages itself, so the paper is set here."
    : "Choose what to print. Paper size, margins and “Save as PDF” are in your "
      + "system’s print dialog, which opens next.";
  // A book is almost always wanted whole; a single tune has nothing else to
  // offer, so the default is whichever is the obvious answer.
  const want = many ? "book" : "tune";
  $$('#print-what input[name="what"]').forEach((r) => (r.checked = r.value === want));
  // Starting from what is on the screen: somebody looking at the counted
  // bars is about to print them, and somebody who has never turned them on
  // is not asking for them by opening this.
  $("#print-multirest").checked = !!state.view.multirest;
  $("#print-concert").checked = !!state.view.concert;

  return new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      if (dlg.returnValue !== "ok") {
        resolve(null);
        return;
      }
      const picked = $$('#print-what input[name="what"]').find((r) => r.checked);
      resolve({
        what: picked ? picked.value : "tune",
        landscape: paper && $("#print-landscape").checked,
        pageNumbers: !paper || $("#print-numbers").checked,
        multiRests: $("#print-multirest").checked,
        concert: $("#print-concert").checked,
      });
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
}

/* ---------- opening a file ---------- */

// isPDF says whether what was opened was a PDF, by its name.
//
// By the name and not by what the importer did, because the importer is on
// the other side of the wire and hands back a score like any other. The
// extension is what the person chose, which is the thing being answered.
function isPDF(name) {
  return typeof name === "string" && /\.pdf$/i.test(name.trim());
}

// tellAboutPDF explains, once per file opened, what a PDF does not carry,
// and shows what was made of it.
//
// A status line was not enough. The gap between the page somebody was
// looking at a moment ago and the score in front of them now is wide — no
// slurs, no dynamics, no words — and a line that fades is a line that was
// not read. This stops and says it.
//
// It also draws the reading back over the page, because one thing a PDF
// never says is how many staves make a system, and getting that wrong is
// not a small error: a band score read as single staves arrives as one part
// thousands of bars long, perfectly well formed and completely wrong. The
// drawing is the only way to see it, and the number beside it is the way to
// say otherwise.
function tellAboutPDF() {
  const dlg = $("#pdf-note");
  if (!dlg || dlg.open) return;
  dlg.showModal();
  loadPDFSurvey();
}

// pdfSurvey holds the last reading fetched, so that changing page or zoom
// redraws without asking the server again.
let pdfSurvey = null;

async function loadPDFSurvey(staves) {
  const box = $("#pdf-review");
  if (!box) return;
  const verdict = $("#pdf-verdict");
  try {
    let url = "/api/pdfsurvey";
    if (staves !== undefined) url += "?staves=" + encodeURIComponent(staves);
    const res = await fetch(url);
    if (!res.ok) {
      // Not every PDF gets this far: a scan has no glyphs to report. The
      // explanation above is still worth showing, so only the drawing goes.
      box.hidden = true;
      return;
    }
    const d = await res.json();
    pdfSurvey = d.survey || null;
    box.hidden = false;
    const pages = (pdfSurvey && pdfSurvey.pages) || [];
    const sel = $("#pdf-page");
    if (sel) {
      sel.innerHTML = "";
      pages.forEach((_, i) => {
        const o = document.createElement("option");
        o.value = String(i);
        o.textContent = String(i + 1);
        sel.appendChild(o);
      });
      sel.disabled = pages.length < 2;
    }
    // What the score was in fact read with, which is what a number typed
    // into the box is afterwards compared against. Without it there is no
    // telling an untouched box from one that has been changed and not yet
    // acted on.
    pdfReadAs = pdfSurvey ? pdfSurvey.perSystem || 0 : 0;
    const num = $("#pdf-staves");
    if (num && document.activeElement !== num) {
      num.value = String(pdfReadAs);
    }
    markPDFPending();
    if (verdict) describePDFReading(verdict, pdfSurvey);
    drawPDFPage(0);
  } catch (e) {
    box.hidden = true;
  }
}

// describePDFReading says in words what the drawing says in lines.
//
// Both, because they answer different questions. The drawing says whether
// this page was read right; the sentence says what the whole file became,
// which is the thing somebody is about to work in.
function describePDFReading(el, sv) {
  if (!sv) {
    el.textContent = "";
    return;
  }
  const per = sv.perSystem || 0;
  const parts = sv.parts || 0;
  let s = "Read as " + plural(parts, "part") +
    ", " + plural(per, "staff", "staves") + " to a system.";
  // The guess and the instruction are shown together only when they differ:
  // agreeing, they are one fact said twice.
  if (sv.detected && sv.detected !== per) {
    s += " Reading the page alone would have given " +
      plural(sv.detected, "staff", "staves") + ".";
  }
  // One part out of a file whose pages hold several staves is the failure
  // this window exists for, so it is named rather than left to be noticed.
  const many = (sv.pages || []).some((p) => (p.staves || []).length > 1);
  const one = parts === 1 && many;
  // A number that does not divide the page is the other way this goes wrong,
  // and it is the quieter one: the grouping obeys, the parts come out, and
  // they are cut across the middle of each system. It shows as a staff count
  // per page that the number leaves a remainder in — a band score whose
  // pages hold 9, 11, 10 and 11 staves cannot be read with any one number at
  // all, and saying which pages disagree is more use than saying so vaguely.
  const odd = (sv.pages || [])
    .map((p, i) => [i + 1, (p.staves || []).length])
    .filter(([, n]) => per > 0 && n > 0 && n % per !== 0);
  if (odd.length) {
    s += " Page " + odd.slice(0, 3).map(([i, n]) => i + " holds " + n)
      .join(", page ") +
      (odd.length > 3 ? ", and others" : "") +
      " — not a whole number of systems of " + per + ".";
  }
  el.textContent = s;
  el.classList.toggle("warn", one || odd.length > 0);
  if (one) {
    el.textContent += " Every staff became part of one long line — if the" +
      " page shows players reading at once, say how many here.";
  }
}

function plural(n, one, many) {
  return n + " " + (n === 1 ? one : (many || one + "s"));
}

// drawPDFPage puts the reading back on the page it was read from.
function drawPDFPage(i) {
  const svg = $("#pdf-overlay");
  if (!svg || !pdfSurvey) return;
  const pages = pdfSurvey.pages || [];
  const p = pages[i];
  if (!p) return;
  const zoom = parseFloat(($("#pdf-zoom") || {}).value || "1.5") || 1.5;
  const W = p.width || 612, H = p.height || 792;
  svg.setAttribute("viewBox", "0 0 " + W + " " + H);
  svg.setAttribute("width", String(Math.round(W * zoom)));
  svg.setAttribute("height", String(Math.round(H * zoom)));

  // A PDF counts upward from the foot of the page and SVG downward from the
  // head, so every y is turned over here rather than by a transform: a
  // flipped transform would mirror the text too, and a pitch written upside
  // down is worse than none.
  const Y = (y) => H - y;
  const ns = "http://www.w3.org/2000/svg";
  const el = (name, attrs, text) => {
    const n = document.createElementNS(ns, name);
    for (const k in attrs) n.setAttribute(k, String(attrs[k]));
    if (text !== undefined) n.textContent = text;
    return n;
  };
  const frag = document.createDocumentFragment();

  for (const r of p.rules || []) {
    frag.appendChild(el("line", {
      x1: r.x1, y1: Y(r.y1), x2: r.x2, y2: Y(r.y2),
      stroke: "#cbd5e1", "stroke-width": Math.max(r.w || 0.4, 0.3),
    }));
  }

  // Each part gets its own colour, so a page that should show one system of
  // five staves and shows five systems of one is a change of colour down the
  // page instead of across it.
  const hue = (k) => "hsl(" + ((k * 47) % 360) + ", 70%, 45%)";
  for (const st of p.staves || []) {
    const top = Y(st.y[4]), bot = Y(st.y[0]);
    frag.appendChild(el("rect", {
      x: st.x0, y: top, width: Math.max(st.x1 - st.x0, 1),
      height: Math.max(bot - top, 1),
      fill: hue(st.part), "fill-opacity": 0.12,
      stroke: hue(st.part), "stroke-width": 0.4, "stroke-opacity": 0.6,
    }));
    frag.appendChild(el("text", {
      x: st.x0 - 2, y: (top + bot) / 2 + st.inter / 2,
      "text-anchor": "end", "font-size": Math.max(st.inter * 1.4, 8 / zoom),
      fill: hue(st.part),
    }, "s" + (st.system + 1) + "·p" + (st.part + 1)));
  }

  for (const m of p.marks || []) {
    const st = (p.staves || [])[m.staff];
    const r = st ? Math.max(st.inter * 0.5, 0.8) : 2;
    if (m.kind === "note") {
      frag.appendChild(el("circle", {
        cx: m.x + r, cy: Y(m.y), r: r,
        fill: "none", stroke: "#dc2626", "stroke-width": r * 0.3,
      }));
      // Shown when they can be read, which is not the same as when the
      // zoom is high: a band score sets its staves at three and a half
      // points, a solo at ten, and the same zoom makes one legible and the
      // other a smear. What matters is the size on the glass, so that is
      // what is measured — and the type is held to a floor in those terms,
      // or a label on a small staff shrinks with it to nothing.
      if (m.pitch && st && st.inter * zoom >= 6) {
        frag.appendChild(el("text", {
          x: m.x + r, y: Y(m.y) - r * 1.7, "text-anchor": "middle",
          "font-size": Math.max(r * 1.6, 7 / zoom), fill: "#dc2626",
        }, m.pitch));
      }
    } else {
      frag.appendChild(el("circle", {
        cx: m.x, cy: Y(m.y), r: r * 0.5,
        fill: "#9ca3af", "fill-opacity": 0.8,
      }));
    }
  }

  svg.innerHTML = "";
  svg.appendChild(frag);
}

// setupPDFReview wires the controls of the drawing.
//
// Changing the page or the zoom only redraws what is already here; the number
// of staves goes back to the server, because only it changes the score.
//
// The number is an instruction, not a note to be typed and then confirmed
// somewhere else. That was wrong first: nothing but the button beside the box
// ever read what was in it, so typing 8 and pressing the dialog's own "Go on"
// threw the 8 away and left the file read exactly as it had been. What that
// looked like from outside was the reader failing — "I enter 8 and still get
// a single line" — when the reader had never been asked. So the number now
// takes effect on its own: when the box is left, when the window is
// dismissed, and still on the button or on Enter.
let pdfReadAs = null; // the number the open score was last read with
let pdfSoon = null; // a re-read waiting for the typing to settle

function setupPDFReview() {
  $("#pdf-reread")?.addEventListener("click", (e) => {
    e.preventDefault();
    rereadPDF();
  });
  $("#pdf-page")?.addEventListener("change", (e) =>
    drawPDFPage(Number(e.target.value) || 0));
  $("#pdf-zoom")?.addEventListener("input", () =>
    drawPDFPage(Number(($("#pdf-page") || {}).value) || 0));
  // Enter in the number box means the same as the button beside it; a form
  // that swallows it and closes the dialog would look like the reading had
  // been accepted.
  $("#pdf-staves")?.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      rereadPDF();
    }
  });
  // Typing marks the button as owing something, so a number sitting unread
  // in the box is visible rather than silent.
  $("#pdf-staves")?.addEventListener("input", markPDFPending);
  // Leaving the box applies it. The pause is for the spinner arrows: each
  // click is a change of its own, and a thirty-six page band score read once
  // per click would be unusable.
  $("#pdf-staves")?.addEventListener("change", () => {
    clearTimeout(pdfSoon);
    pdfSoon = setTimeout(() => {
      pdfSoon = null;
      if (pdfStaffBox() !== pdfReadAs) rereadPDF();
    }, 400);
  });
  // And dismissing the window applies it at once. "Go on" is the button of
  // somebody who has finished deciding, not of somebody throwing away what
  // they decided.
  $("#pdf-note")?.addEventListener("close", () => void (async () => {
    clearTimeout(pdfSoon);
    pdfSoon = null;
    if (pdfReadAs !== null && pdfStaffBox() !== pdfReadAs) await rereadPDF();
    await offerScoreSetup();
  })());
}

// offerScoreSetup opens Score setup on staves that were just read off a
// page and do not yet say what they are.
//
// This is the other half of reading a PDF, and it was missing. A page says
// how many staves there are and nothing about what plays them, so an import
// of a band score lands as eight staves called "Staff", all sounding like
// pianos — and somebody who does not already know there is a window for
// naming them has no reason to go looking for one. So the window comes to
// them, once, at the moment the question arises.
//
// Not where there is nothing to ask. One staff is a solo and needs no
// deciding, and a file that already named its instruments has answered.
async function offerScoreSetup() {
  const parts = state.info?.parts || [];
  if (parts.length < 2 || parts.some((p) => p.sound)) return;
  await scoreSetupDialog();
}

// pdfStaffBox is what the number box says, held to what the server accepts.
function pdfStaffBox() {
  const num = $("#pdf-staves");
  if (!num) return null;
  return Math.max(0, Math.min(64, parseInt(num.value, 10) || 0));
}

// markPDFPending shows whether the number in the box is still the number the
// score was read with.
function markPDFPending() {
  const b = $("#pdf-reread");
  if (!b) return;
  b.classList.toggle("pending",
    pdfReadAs !== null && pdfStaffBox() !== pdfReadAs);
}

// rereadPDF reads the same file again with the number that was given.
async function rereadPDF() {
  const num = $("#pdf-staves");
  if (!num) return;
  clearTimeout(pdfSoon);
  pdfSoon = null;
  const n = pdfStaffBox();
  busy("Reading again…");
  try {
    const res = await fetch("/api/pdfreread?staves=" + n, { method: "POST" });
    if (!res.ok) {
      status("Could not read again: " + (await res.text()).trim());
      return;
    }
    const d = await res.json();
    pdfReadAs = n;
    markPDFPending();
    await reopen();
    status("Read again as " + plural(d.parts || 0, "part"));
    await loadPDFSurvey(n);
  } finally {
    idle();
  }
}

async function openFile(file) {
  if (!file) return;
  status("Opening " + file.name + "…");
  busy("Opening " + file.name + "…");
  try {
    const res = await fetch("/api/open?name=" + encodeURIComponent(file.name), {
      method: "POST",
      body: await file.arrayBuffer(),
    });
    if (!res.ok) {
      status("Could not open: " + (await res.text()).trim());
      return false;
    }
    await reopen();
    status("Opened " + file.name);
    if (isPDF(file.name)) tellAboutPDF();
    return true;
  } finally {
    // Whatever happened, including nothing: a mascot left spinning over a
    // window that has given up is worse than no mascot at all.
    idle();
  }
}

// openScore asks the system for a file when gscore is running as an
// application, and falls back to the browser's own picker when it is served.
// The native window has no file panel behind <input type="file">, so that
// element alone would do nothing at all.
async function openScore() {
  if (state.info?.local) {
    status("Choose a score…");
    try {
      const res = await fetch("/api/pickfile", { method: "POST" });
      if (res.ok) {
        const d = await res.json();
        if (d.cancelled) {
          status("Ready");
          return;
        }
        // From here on it is reading and laying out, which is the part that
        // takes the time. The panel itself was the person's own doing.
        busy("Opening " + d.filename + "…");
        try {
          await reopen();
        } finally {
          idle();
        }
        status("Opened " + d.filename);
        if (isPDF(d.filename)) tellAboutPDF();
        return;
      }
      // A file that was chosen and could not be read is not the same as
      // there being no panel to choose it with, and only one of them is
      // answered by offering another panel.
      //
      // Both fell through here, and in the application window the fallback
      // does nothing at all — there is no file panel behind the hidden
      // input, which is the whole reason the system is asked for a dialog.
      // So picking a Finale file told the person nothing: the panel closed,
      // no score arrived, and the status line still read "Choose a score…".
      // The server had said why in the answer that was thrown away.
      if (res.status !== 501) {
        status("Could not open: " + ((await res.text()).trim() || res.statusText));
        return;
      }
    } catch (_) {
      /* no answer at all: the browser picker is the next thing to try */
    }
  }
  $("#file-input").click();
}

// freeTheFilePanels lets a file be chosen.
//
// accept is a list of extensions, and it was a guess at what gscore reads
// made in the one place that does not know: the browser. It was wrong twice.
// On iOS the list is turned into types the system has heard of, and it has
// heard of none of ours — .mscz, .gscore, .abc, .nwctxt and .sf2 are nobody's
// registered type — so the Files panel opened with every score greyed out and
// an iPhone could not open a score at all. On a desktop it is a filter, which
// is pleasant until the file you want is a kind gscore has just learned to
// read, or one you want to try: a .musx sitting in Downloads, greyed out,
// with nothing to say why.
//
// So the panel offers everything and the importers answer. They are where
// the knowledge is, they say what they cannot read and why — "export it from
// Finale as MusicXML" — and a sentence beats a file that cannot be clicked.
function freeTheFilePanels() {
  document.querySelectorAll('input[type="file"][accept]')
    .forEach((input) => input.removeAttribute("accept"));
}

function setupFileInput() {
  freeTheFilePanels();
  $("#file-input").addEventListener("change", async (e) => {
    await openFile(e.target.files[0]);
    e.target.value = "";
  });

  // A score dropped anywhere on the page opens, because a score dropped
  // anywhere on the page must not be followed.
  //
  // This listened on the canvas alone, and what a browser does with a file
  // dropped on a page it has not been told about is open the file. Dropping
  // a score on the toolbar, on the menus, or in the margin beside the music
  // took the editor away to it — and from a page served over https the
  // browser then refused the file:// it had just been sent to: "Security
  // Error: Content at https://stricaud.github.io/gscore/app/ may not load
  // or link to file:///". The score did not open and the editor was gone.
  //
  // The window takes it instead, so there is nowhere on the page that is
  // not handled. preventDefault on dragover is what says the page will
  // deal with the drop; without it the drop never arrives.
  window.addEventListener("dragover", (e) => e.preventDefault());
  window.addEventListener("drop", async (e) => {
    e.preventDefault();
    const file = e.dataTransfer?.files?.[0];
    if (file) await openFile(file);
  });
}

/* ---------- playback ---------- */

let audio = null;

// wakeAudio makes the sound context and asks it to start, without waiting.
//
// It has to be called straight from whatever the user did, so this is
// deliberately not async: the asking is what has to happen inside the
// gesture, not the finishing.
function wakeAudio() {
  if (!audio) {
    audio = new (window.AudioContext || window.webkitAudioContext)();
    noise = null;
  }
  if (audio.state !== "running") {
    // "interrupted" is WebKit's, for a context the system took away.
    audio.resume().catch(() => { /* answered by the check in play() */ });
  }
  return audio;
}
let voices = [];
let rafID = null;

// playAt is where the score's own time zero sits on the audio clock, so
// that the place being heard is audio.currentTime - outputLag() - playAt.
//
// The audio clock and not the wall clock. The sound is scheduled against
// audio.currentTime, and the cursor used to be driven by performance.now():
// two clocks, counted from one moment, which agree at the start and need
// not agree later. They are different oscillators, and the audio one stops
// when the system takes the context away and picks up where it left off —
// so on a long piece the cursor drifts away from the music it is following,
// which is the one thing it exists not to do.
let playAt = 0;

// playedSeconds is where in the score the sound coming out of the speakers
// has got to.
//
// Coming out of the speakers, which is later than audio.currentTime by
// whatever the machine takes to play what has been handed to it. A fifth of
// a second over Bluetooth headphones is ordinary, and a fifth of a second
// is a quaver at a brisk tempo: the notehead lights up, and the note is
// heard while the next one is lit.
function playedSeconds() {
  if (!audio) return 0;
  return audio.currentTime - outputLag() - playAt;
}

// outputLag is how far behind the audio clock the speakers are.
//
// outputLatency is the whole of it and is what to use where there is one.
// WebKit has no such property — which is the Mac application and every
// browser on a Mac — and baseLatency, which it does have, counts only the
// part of the journey inside the browser. It is better than nothing and
// wrong about Bluetooth; there is no third thing to ask.
function outputLag() {
  // What somebody has measured for their own ears, if they have, on top of
  // whatever the browser owns up to. Both: the browser's figure is right
  // about the part it can see, and the correction is for the part it
  // cannot — the headphones, the amplifier, the room.
  const mine = Number(state.playLag) || 0;
  if (!audio) return mine / 1000;
  const out = Number(audio.outputLatency);
  const own = Number.isFinite(out) && out > 0 ? out : 0;
  const base = Number(audio.baseLatency);
  const said = own || (Number.isFinite(base) && base > 0 ? base : 0);
  return said + mine / 1000;
}

// The performance being rendered on the other side, when the score is being
// played through a soundfont rather than through the oscillators.
//
// null whenever it is not: everything else about playing — the clock, the
// cursor, the notes lighting up — is driven by playAt and does not know
// or care which of the two is making the noise.
let live = null;
// The refs lit up right now, which is a set because a chord is several
// noteheads and a score is several staves.
let current = new Set();
let cursorEl = null;

// sameSet avoids redrawing when nothing has changed, which at sixty frames a
// second across five staves is most of the time.
function sameSet(a, b) {
  if (a.size !== b.size) return false;
  for (const v of a) {
    if (!b.has(v)) return false;
  }
  return true;
}

const midiToHz = (m) => 440 * Math.pow(2, (m - 69) / 12);

/* ---------- the voices ---------- */

// A staff says what it sounds as, and this is where that becomes a sound.
//
// These are not samples and do not pretend to be: each family is built from
// the one or two things that actually tell it apart by ear — how fast the note
// arrives, whether it holds or dies away, and how much is going on above the
// fundamental. A flute is nearly a sine wave that fades in; a plucked string
// is bright at the start and gone by the end; brass arrives late and loud with
// the top opening up as it does. Those differences carry further than
// waveform names do.
//
// The families come from the server, from General MIDI by way of
// model.Instrument.Family, so adding an instrument needs nothing here.
const VOICES = {
  flute: {
    wave: "sine", partial: { wave: "triangle", gain: 0.12, ratio: 2 },
    attack: 0.06, decay: 0.05, sustain: 0.9, release: 0.08,
    cutoff: 3000, vibrato: { hz: 5, cents: 9, delay: 0.18 },
  },
  reed: {
    wave: "sawtooth", attack: 0.03, decay: 0.08, sustain: 0.72, release: 0.07,
    cutoff: 2200, vibrato: { hz: 5.5, cents: 7, delay: 0.25 },
  },
  brass: {
    wave: "sawtooth", attack: 0.05, decay: 0.09, sustain: 0.78, release: 0.06,
    // The brightness swells with the note, which is most of what says brass.
    cutoff: 1200, open: 3200,
  },
  bowed: {
    wave: "sawtooth", attack: 0.08, decay: 0.1, sustain: 0.8, release: 0.12,
    cutoff: 2600, vibrato: { hz: 5, cents: 12, delay: 0.3 },
  },
  plucked: {
    wave: "triangle", partial: { wave: "sawtooth", gain: 0.25, ratio: 1 },
    attack: 0.002, decay: 0.5, sustain: 0, release: 0.05, cutoff: 3500,
  },
  // Struck and left to ring. The hammer is softer than a plectrum, so the
  // attack is slower and there is no sawtooth at the fundamental to buzz
  // against it; the octave above gives it body instead. It keeps a little
  // sustain so the note lasts as long as it is written, rather than ringing
  // down to nothing halfway through — the decay here is capped at half the
  // note, and a whole note that stops after two beats is not a piano.
  piano: {
    wave: "triangle", partial: { wave: "sine", gain: 0.35, ratio: 2 },
    attack: 0.004, decay: 0.9, sustain: 0.16, release: 0.18, cutoff: 2400,
  },
  tuned: {
    wave: "sine", partial: { wave: "sine", gain: 0.3, ratio: 3 },
    attack: 0.001, decay: 0.35, sustain: 0.05, release: 0.06, cutoff: 5000,
  },
  sustained: {
    wave: "triangle", attack: 0.02, decay: 0.06, sustain: 0.8, release: 0.06,
    cutoff: 3000,
  },
};

// The kit.
//
// A drum is not a note. It has no pitch to speak of, it does not sustain, and
// what tells a snare from a kick is mostly how fast it dies away and how much
// of it is noise rather than tone. So these are built from the two things a
// drum is made of — a band of noise and a short falling tone — in the
// proportions that make each one recognisable.
//
// The keys are General MIDI percussion numbers, the same ones the server
// sends, so a drum part read out of a NoteWorthy or MusicXML file plays
// without anything in between.
const DRUMS = {
  35: { noise: 0.15, tone: 70, drop: 36, decay: 0.26, type: "lowpass", cut: 220 },  // acoustic kick
  36: { noise: 0.18, tone: 80, drop: 42, decay: 0.22, type: "lowpass", cut: 260 },  // kick
  37: { noise: 0.7,  tone: 420, drop: 300, decay: 0.07, type: "bandpass", cut: 1800 }, // side stick
  38: { noise: 0.85, tone: 190, drop: 90, decay: 0.17, type: "highpass", cut: 400 }, // snare
  40: { noise: 0.9,  tone: 210, drop: 100, decay: 0.13, type: "highpass", cut: 500 }, // electric snare
  41: { noise: 0.25, tone: 110, drop: 55, decay: 0.34, type: "lowpass", cut: 500 },  // low floor tom
  43: { noise: 0.25, tone: 135, drop: 65, decay: 0.32, type: "lowpass", cut: 600 },  // high floor tom
  45: { noise: 0.25, tone: 165, drop: 80, decay: 0.30, type: "lowpass", cut: 700 },  // low tom
  47: { noise: 0.25, tone: 200, drop: 95, decay: 0.28, type: "lowpass", cut: 800 },  // low-mid tom
  48: { noise: 0.25, tone: 240, drop: 110, decay: 0.26, type: "lowpass", cut: 900 }, // high-mid tom
  50: { noise: 0.25, tone: 290, drop: 130, decay: 0.24, type: "lowpass", cut: 1000 },// high tom
  42: { noise: 1,    tone: 0,   drop: 0,  decay: 0.055, type: "highpass", cut: 7000 }, // hi-hat closed
  44: { noise: 1,    tone: 0,   drop: 0,  decay: 0.07, type: "highpass", cut: 6500 }, // hi-hat pedal
  46: { noise: 1,    tone: 0,   drop: 0,  decay: 0.42, type: "highpass", cut: 6000 }, // hi-hat open
  49: { noise: 1,    tone: 0,   drop: 0,  decay: 1.5,  type: "highpass", cut: 4000 }, // crash
  51: { noise: 1,    tone: 0,   drop: 0,  decay: 0.65, type: "highpass", cut: 6500 }, // ride
  52: { noise: 1,    tone: 0,   drop: 0,  decay: 1.1,  type: "highpass", cut: 3500 }, // chinese
  53: { noise: 0.75, tone: 1200, drop: 0, decay: 0.5,  type: "highpass", cut: 5000 }, // ride bell
  55: { noise: 1,    tone: 0,   drop: 0,  decay: 0.55, type: "highpass", cut: 7000 }, // splash
  56: { noise: 0.3,  tone: 835, drop: 0,  decay: 0.28, type: "bandpass", cut: 2500 }, // cowbell
};

// noiseBuffer is the white noise every drum is partly made of. One buffer is
// made and shared: generating two seconds of random numbers for every hi-hat
// in a four-minute piece is thousands of times more work than reusing it.
let noise = null;
function noiseSource() {
  if (!noise) {
    const n = audio.sampleRate * 2;
    noise = audio.createBuffer(1, n, audio.sampleRate);
    const d = noise.getChannelData(0);
    for (let i = 0; i < n; i++) d[i] = Math.random() * 2 - 1;
  }
  const src = audio.createBufferSource();
  src.buffer = noise;
  src.loop = true;
  return src;
}

// hit plays one drum.
function hit(ev, start, master) {
  const k = DRUMS[ev.drum] || DRUMS[38];
  const gain = audio.createGain();
  const filter = audio.createBiquadFilter();
  filter.type = k.type;
  filter.frequency.value = k.cut;
  filter.Q.value = k.type === "bandpass" ? 2.5 : 0.8;

  const peak = (0.35 + 0.65 * ((ev.velocity ?? 90) / 127));
  const g = gain.gain;
  g.setValueAtTime(0, start);
  g.linearRampToValueAtTime(peak, start + 0.001);
  // A drum is decay and nothing else: no sustain to hold and no release to
  // taper, which is exactly what makes it read as a hit.
  g.exponentialRampToValueAtTime(0.0008, start + k.decay);

  if (k.noise > 0) {
    const src = noiseSource();
    const ng = audio.createGain();
    ng.gain.value = k.noise;
    src.connect(ng);
    ng.connect(filter);
    src.start(start);
    src.stop(start + k.decay + 0.05);
    voices.push(src);
  }
  if (k.tone > 0) {
    const osc = audio.createOscillator();
    osc.type = "sine";
    osc.frequency.setValueAtTime(k.tone, start);
    if (k.drop > 0) {
      // The pitch falling away is what makes a kick a kick rather than a
      // thump: the head tightens and lets go.
      osc.frequency.exponentialRampToValueAtTime(
        Math.max(20, k.tone - k.drop), start + k.decay * 0.7);
    }
    const tg = audio.createGain();
    tg.gain.value = 1 - k.noise * 0.6;
    osc.connect(tg);
    tg.connect(filter);
    osc.start(start);
    osc.stop(start + k.decay + 0.05);
    voices.push(osc);
  }

  filter.connect(gain);
  gain.connect(master);
}

// sound plays one event. Everything it makes stops by itself and is collected
// in `voices` so that pressing stop can cut it short.
/* ---------- playing without a soundfont ---------- */

// Served over a network there are no soundfonts to play through — they are
// files on somebody's machine — so the page builds the sound itself out of
// oscillators. One note is a handful of Web Audio nodes, and that is fine
// for a song and hopeless for a symphony: Beethoven's ninth is a hundred
// and thirty-two thousand notes and fifty-seven minutes long, and building
// all of it before the first note sounds means half a million nodes made up
// front. The browser either refuses or takes so long that the music has
// gone past by the time it starts.
//
// So the notes are built a few seconds ahead of the music and topped up as
// it goes, and the handles on the ones that have finished are let go. There
// are a few dozen nodes in hand at any moment rather than half a million,
// whatever the piece.

// scheduleAhead is how far in front of the music the notes are built, in
// seconds, and scheduleEvery how often that window moves forward.
//
// Two seconds is far more than the quarter of a second it takes to top up,
// so a browser busy with something else still has the next notes ready; and
// it is short enough that stopping is immediate, since nothing beyond it
// has been built.
const scheduleAhead = 2;
const scheduleEvery = 250;

let scheduleTimer = null;

// stopScheduling drops whatever was still to be built. What has already
// been scheduled is silenced separately, with everything else.
function stopScheduling() {
  if (scheduleTimer) clearInterval(scheduleTimer);
  scheduleTimer = null;
}

function scheduleOscillators(events, from, t0, master) {
  stopScheduling();

  // Only what is still to be heard: everything over before we came in is
  // nothing to build. Already in the order it is played — see
  // playbackData — so moving the window forward is a walk rather than a
  // search.
  const todo = events.filter((e) => e.start + e.dur > from);
  if (!todo.length) return;

  // The longest note in the piece, which is how long after building one it
  // may still be ringing — and so how long its handle has to be kept.
  let longest = 0;
  for (const e of todo) {
    if (e.dur > longest) longest = e.dur;
  }

  let next = 0;
  // Where the built notes reach to, so that handles on the ones that have
  // certainly finished can be let go rather than held for the whole piece.
  let marks = [];

  const tick = () => {
    // The music's own clock: where the piece has got to, plus the window.
    const until = audio.currentTime - t0 + from + scheduleAhead;
    while (next < todo.length && todo[next].start <= until) {
      const ev = todo[next++];
      if (ev.start >= from) {
        sound(ev, t0 + ev.start - from, master);
      } else {
        // A note already ringing when we come in: taken up part way, for
        // what is left of it, rather than struck again.
        sound({ ...ev, dur: ev.start + ev.dur - from }, t0, master);
      }
    }

    marks.push({ at: audio.currentTime, upto: voices.length });
    // Anything built longer ago than the longest note can last is over.
    const safe = audio.currentTime - longest - scheduleAhead - 1;
    let done = 0;
    while (marks.length && marks[0].at < safe) {
      done = marks[0].upto;
      marks.shift();
    }
    if (done > 0) {
      voices = voices.slice(done);
      marks = marks.map((m) => ({ at: m.at, upto: m.upto - done }));
    }

    if (next >= todo.length) stopScheduling();
  };

  tick();
  // A piece short enough to build in one go needs nothing to tick over it.
  if (next < todo.length) scheduleTimer = setInterval(tick, scheduleEvery);
}

/* ---------- the metronome ---------- */

// A click on every beat while the music plays.
//
// The same beats the View menu draws arrows over — both are counted in
// model.TimeSignature.BeatUnit, on the Go side, so a 6/8 jig clicks twice a
// bar and is marked twice a bar. A metronome that ticks somewhere the page
// does not show a beat is worse than none: the player trusts the click, and
// then the click and the score disagree.
//
// It is not an instrument of the score. It is scheduled alongside whatever is
// playing the notes — the soundfont or the oscillators, it makes no
// difference — into a gain of its own, so that its level is its own and it
// cannot be mistaken for something somebody wrote.

// metronomeBeats turns the bar timeline into the moments a click sounds.
//
// Each bar is divided by the number of beats the score says it holds, which
// is counted from the music in it rather than from the signature over it: a
// pickup is the bar where those differ, and a metronome that ticks a full bar
// through a one-beat pickup is three ticks of silence before the music.
function metronomeBeats(measures) {
  const out = [];
  for (const m of measures || []) {
    const beats = m.beats || 0;
    if (beats < 1 || !(m.dur > 0)) continue;
    const step = m.dur / beats;
    for (let i = 0; i < beats; i++) {
      // The first beat of a bar is the one you count from, so it is the one
      // that sounds different. A row of identical clicks tells you the
      // speed and not the place, and the place is the harder thing to keep.
      out.push({ at: m.start + i * step, strong: i === 0 });
    }
  }
  // The bars come in the order they are played, repeats and all, so the
  // clicks already do too.
  return out;
}

let metronomeTimer = null;

// stopMetronome drops whatever clicks were still to be built. What has been
// scheduled already is silenced with everything else, in silence().
function stopMetronome() {
  if (metronomeTimer) clearInterval(metronomeTimer);
  metronomeTimer = null;
}

// scheduleMetronome builds the clicks a window ahead of the music, the way
// the notes are built.
//
// A click is three Web Audio nodes and a symphony is fourteen thousand of
// them; built all at once that is the same wall the notes ran into.
function scheduleMetronome(measures, from, t0) {
  stopMetronome();
  if (!state.metronome) return;

  const beats = metronomeBeats(measures).filter((b) => b.at >= from);
  if (!beats.length) return;

  const dest = audio.createGain();
  // Under the music rather than over it. The click is there to be followed,
  // not listened to, and a metronome louder than the piece is one somebody
  // turns off.
  dest.gain.value = 0.5;
  dest.connect(audio.destination);

  let next = 0;
  const tick = () => {
    const until = audio.currentTime - t0 + from + scheduleAhead;
    while (next < beats.length && beats[next].at <= until) {
      const b = beats[next++];
      click(t0 + b.at - from, b.strong, dest);
    }
    if (next >= beats.length) stopMetronome();
  };

  tick();
  if (next < beats.length) metronomeTimer = setInterval(tick, scheduleEvery);
}

// click is one tick of the metronome.
//
// A short burst of a high tone with the edge taken off it, which is what a
// mechanical metronome sounds like and what carries over a piano without
// being part of it. Pitched rather than noisy on purpose: a noise burst
// disappears into a loud bar, and the thing has to be audible at the exact
// moment it is needed or it is no use at all.
function click(when, strong, dest) {
  const osc = audio.createOscillator();
  osc.type = "square";
  // The downbeat higher than the rest, which is the convention every
  // metronome with a bell follows.
  osc.frequency.value = strong ? 1600 : 1100;

  const gain = audio.createGain();
  const peak = strong ? 0.5 : 0.3;
  // Struck and gone: an attack short enough to be a click rather than a
  // beep, and a decay that is over well before the next beat at any speed
  // anybody plays at. Ramped and not switched, because a gain that jumps to
  // zero clicks a second time on its own.
  gain.gain.setValueAtTime(0.0001, when);
  gain.gain.exponentialRampToValueAtTime(peak, when + 0.001);
  gain.gain.exponentialRampToValueAtTime(0.0001, when + 0.05);

  osc.connect(gain);
  gain.connect(dest);
  osc.start(when);
  osc.stop(when + 0.06);
  // Collected with the notes so that stopping cuts the click short too.
  voices.push(osc);
}

// soundAsWritten plays the note that has just been written, where that is
// wanted.
//
// What it plays is the server's answer about the note it just wrote: the
// pitch it sounds at, through the part's transposition, or the drum the
// staff position names. The page works none of that out — see addSounding
// in edit.go.
//
// Through the editor's own oscillators rather than the soundfont, even
// where there is one. A note typed should be heard as it is typed, and the
// soundfont lives on the other side of a request that renders in blocks of
// nearly a second: by the time it answered, the next note would be written.
// The oscillators are in the page and start in a tenth of that.
//
// It is never part of the performance: a note sounded while writing is not
// scheduled, not collected with the voices, and stopping the score does not
// have to silence it, because it is over in under a second.
function soundAsWritten(wrote) {
  if (!wrote || !state.entrySound) return;
  if (state.playing) return; // the score is already making a noise
  const drum = Number(wrote.drum) || 0;
  const midi = Number(wrote.midi);
  if (!drum && !Number.isFinite(midi)) return;
  try {
    wakeAudio();
    if (audio.state && audio.state !== "running") {
      // A context the browser has not let start yet. Nothing to say about
      // it: the score is still being written, and the first click on the
      // page will start it.
      void audio.resume();
      return;
    }
    const master = audio.createGain();
    // A touch under the level the oscillators play a score at, because
    // this is one note on its own and not a texture to be heard through.
    master.gain.value = 0.3;
    master.connect(audio.destination);
    // A second, whatever is being written.
    //
    // Not the note's own length. A semibreve at a slow tempo is four
    // seconds, and four seconds of sound for one keystroke is somebody
    // waiting for the program rather than hearing what they wrote. What
    // this is for is the pitch, and a second is plenty of it.
    const dur = entrySeconds;
    const at = audio.currentTime + 0.005;
    sound({
      midi: drum || midi, dur, gate: 0.9, velocity: 96,
      family: wrote.family || "", drum,
    }, at, master);
    // The gain is let go once the note is over, rather than left hanging
    // off the destination for as long as the window is open.
    setTimeout(() => {
      try {
        master.disconnect();
      } catch (_) {
        /* already gone */
      }
    }, (dur + 0.5) * 1000);
  } catch (_) {
    // No audio in this browser, or it refused. Writing music does not
    // depend on hearing it.
  }
}

// entrySeconds is how long a note sounds when it is written, in seconds.
//
// One, for everything from a semiquaver to a semibreve. The written
// length is not used: it is the pitch somebody is listening for, and a
// semibreve at a slow tempo would hold the keyboard up for four seconds
// a note.
const entrySeconds = 1;

function sound(ev, start, master) {
  if (ev.drum) {
    hit(ev, start, master);
    return;
  }
  // Anything the score does not name is played as a piano, which is what a
  // keyboard with nothing else said about it should sound like. Sustained —
  // an organ — was the old answer, and a score that said nothing about its
  // instrument came out droning; plucked was the next one, and it buzzed.
  const v = VOICES[ev.family] || VOICES.piano;
  const hz = midiToHz(ev.midi);

  // How much of its written length a note actually sounds is the marks on
  // it: staccato is short, tenuto is held to the end, a slurred note runs
  // into the next so the two are one breath. The server works that out and
  // sends it as the gate, because it is a question about music.
  //
  // Whatever the gate, two notes of the same pitch in a row have to be heard
  // as two. What separates them is not the silence between — a hair of it is
  // enough — but the attack at the front of each: a soft ramp makes the
  // second note sound like the first one still ringing. So a detached note
  // keeps at least that hair of silence, and a slurred one, whose gate is
  // over 1, overlaps what follows instead.
  let gap = ev.dur * (1 - (ev.gate > 0 ? ev.gate : 0.88));
  if (gap > 0) gap = Math.max(0.02, gap);
  const dur = Math.max(0.06, ev.dur - gap);

  const gain = audio.createGain();
  const filter = audio.createBiquadFilter();
  filter.type = "lowpass";
  filter.Q.value = 0.7;
  // Follow the pitch, so a high note is not duller than a low one.
  const cutoff = Math.min(audio.sampleRate / 2.5, Math.max(v.cutoff, hz * 4));
  filter.frequency.setValueAtTime(v.open ? Math.max(v.open * 0.4, hz * 3) : cutoff, start);
  if (v.open) {
    filter.frequency.linearRampToValueAtTime(
      Math.min(audio.sampleRate / 2.5, Math.max(v.open, hz * 5)),
      start + Math.min(0.12, dur * 0.4));
  }

  const made = [];
  const osc = audio.createOscillator();
  osc.type = v.wave;
  osc.frequency.setValueAtTime(hz, start);
  osc.connect(filter);
  made.push(osc);

  if (v.partial) {
    const p = audio.createOscillator();
    const pg = audio.createGain();
    p.type = v.partial.wave;
    p.frequency.setValueAtTime(hz * v.partial.ratio, start);
    pg.gain.value = v.partial.gain;
    p.connect(pg);
    pg.connect(filter);
    made.push(p);
  }

  // Vibrato starts a little after the note does, the way a player's does.
  if (v.vibrato && dur > v.vibrato.delay + 0.1) {
    const lfo = audio.createOscillator();
    const depth = audio.createGain();
    lfo.frequency.value = v.vibrato.hz;
    // Cents to hertz at this pitch.
    depth.gain.setValueAtTime(0, start);
    depth.gain.setValueAtTime(0, start + v.vibrato.delay);
    depth.gain.linearRampToValueAtTime(
      hz * (Math.pow(2, v.vibrato.cents / 1200) - 1), start + v.vibrato.delay + 0.15);
    lfo.connect(depth);
    for (const o of made) depth.connect(o.frequency);
    made.push(lfo);
  }

  // How loud, from the dynamic the server resolved.
  const peak = 0.3 + 0.7 * ((ev.velocity ?? 80) / 127);
  const attack = Math.min(v.attack, dur * 0.5);
  const decay = Math.min(v.decay, Math.max(0.01, (dur - attack) * 0.5));
  const release = Math.min(v.release, Math.max(0.01, dur * 0.35));
  const sustain = peak * v.sustain;

  const g = gain.gain;
  g.setValueAtTime(0, start);
  g.linearRampToValueAtTime(peak, start + attack);
  if (v.sustain === 0) {
    // Nothing holds it, so it rings down and the note ends when it is over.
    g.exponentialRampToValueAtTime(0.0005, start + Math.min(dur, decay));
  } else {
    g.linearRampToValueAtTime(sustain, start + attack + decay);
    g.setValueAtTime(sustain, Math.max(start + attack + decay, start + dur - release));
    g.linearRampToValueAtTime(0, start + dur);
  }

  filter.connect(gain);
  gain.connect(master);
  for (const o of made) {
    o.start(start);
    o.stop(start + dur + 0.05);
    voices.push(o);
  }
}

// playFromStart goes back to the top of the book before playing.
//
// A file of exercises holds many tunes and playing follows the one being
// looked at, so "play" means "play this one". Getting back to the first bar of
// the first tune should not mean scrolling up to find it and clicking.
async function playFromStart() {
  wakeAudio(); // in the gesture, before anything is awaited — see togglePlay
  stop();
  if ((state.info?.tunes?.length ?? 0) > 1 && (state.info?.current ?? 0) !== 0) {
    await selectTune(0);
  }
  scrollToTop();
  await play({ fromStart: true });
}

function scrollToTop() {
  const wrap = $("#canvas-wrap") || $("#canvas")?.parentElement;
  if (wrap && wrap.scrollTo) wrap.scrollTo({ top: 0, left: 0, behavior: "smooth" });
}

// play starts the music.
//
// From the note that is selected, if one is: having picked out the bar you
// are working on, hearing the piece from the top again is rarely what you
// wanted. Shift with the space bar still goes back to the beginning, and so
// does playing with nothing selected.
// startOfSelection is where in the performance whatever is picked up
// begins, or null when nothing is.
//
// A note carries the number the engraver gave it and the performance knows
// it by the same one, so it is found exactly. A bar has no number of its
// own, and neither has a cursor standing in one — and those are most of
// what anybody picks up. Without an answer for them, going to bar four
// hundred and pressing play started the piece from the top, which is never
// what somebody who has just gone to bar four hundred meant.
//
// Starting from the beginning is going to the beginning and pressing play,
// which is one gesture more than it was and the right way round: the common
// thing should be the easy one.
function startOfSelection(pb) {
  const sel = state.selection;
  const ref = state.multi.length ? orderedSelection()[0]?.ref : sel?.ref;
  if (ref !== null && ref !== undefined) {
    const ev = (pb.events || []).find((x) => x.ref === ref);
    if (ev) return ev.start;
  }
  if (!sel || sel.measure === null || sel.measure === undefined) return null;
  // The first time through: a bar inside a repeat is played more than once,
  // and the one being looked at is the first of them.
  const m = (pb.measures || []).find((x) => x.index === sel.measure);
  return m ? m.start : null;
}

// play says what it is doing while it gets ready.
//
// Working out a performance and handing the first of it to the sound card
// takes a moment on a big score — a second or two on a symphony — and in
// that moment nothing happens at all, which is indistinguishable from the
// button not having worked. Gilbert waits a quarter of a second before
// appearing, so a score that starts at once never shows him.
async function play(opts) {
  busy("Getting ready to play…");
  try {
    return await playNow(opts);
  } finally {
    // Before the first sound rather than after the piece: what follows this
    // is scheduled, not waited on.
    idle();
  }
}

async function playNow(opts) {
  if (state.playing) stop();

  // Where a pause left off, if this is carrying on from one. Nothing about
  // resuming is up for discussion — not the place in the music and not, just
  // below, which tune is being played.
  const at = opts && opts.at > 0 ? opts.at : 0;

  // In a book, play the tune being looked at. Scrolling down to a tune and
  // pressing space is how somebody asks to hear it; without this the first
  // tune played instead, and since the cursor went with it the view jumped
  // back to the top of the book.
  //
  // A selection says which tune more precisely than the scroll position does,
  // so it wins — clicking a note in one tune and scrolling to another should
  // still play the note that was clicked.
  if (!(opts && opts.fromStart) && !at && !hasSelection()) {
    const seen = visibleTune();
    if (seen !== null && seen !== state.info?.current) {
      await selectTune(seen);
    }
  }

  const pb = await playbackData();
  if (!pb || !pb.events || pb.events.length === 0) {
    status("Nothing to play");
    return;
  }

  // Where to begin. A note played more than once — anything inside a repeat
  // — is started at the first time through, which is the one being looked at.
  let from = 0;
  if (at) {
    // Carrying on from a pause wins over the selection: the selection has
    // not moved, and going back to it is the one thing resuming is not.
    from = at;
    // And a little before it: back to the start of the note that was
    // sounding when the music stopped.
    //
    // A pause lands wherever it lands, which is usually in the middle of a
    // note, and taking that note up from its middle strikes it halfway
    // through its own sustain — a sound nothing in the score asks for. The
    // note it goes back to is the last one to have started, so this is never
    // more than one note's worth of rewind, and it is what a player does
    // anyway. Anything still ringing from further back is taken up part way
    // as before: a held bass note would otherwise drag the whole piece back
    // to where it began.
    let began = -1;
    for (const ev of pb.events) {
      if (ev.start <= from && ev.start + ev.dur > from && ev.start > began) {
        began = ev.start;
      }
    }
    if (began >= 0) from = began;
    // A piece that has been edited since, or shortened, may no longer reach
    // that far. Then there is nothing to carry on from and the top is the
    // only sensible answer.
    if (from >= pb.duration) from = 0;
  } else if (!(opts && opts.fromStart)) {
    const at = startOfSelection(pb);
    if (at !== null) from = at;
  }
  // Whatever was paused has now been taken up or dropped on purpose.
  state.pausedAt = null;

  // The cursor is placed from the engraved positions, so they have to be in
  // hand before it can be shown — but they usually already are. They are
  // loaded when the score is opened and again after every edit, and nothing
  // else moves them. Asking for them again cost a round trip of every
  // clickable region in the score, which on a symphony is a hundred and
  // thirty-eight thousand of them and seventy megabytes: in the page, where
  // the core writing that out is WebAssembly, it was most of the wait
  // between pressing Play and hearing anything.
  if (!(state.hits || []).length) await loadHits();

  wakeAudio();
  await audio.resume();
  // Only where the context says outright that it is not running: a browser
  // that does not report a state at all is not a browser that is refusing.
  if (audio.state && audio.state !== "running") {
    // Say so rather than appear to play. A context that will not start is
    // silence that looks exactly like success: the cursor moves, the
    // noteheads light up, and nothing comes out.
    status("The browser will not let the sound start — click the score and try again");
    return;
  }

  const master = audio.createGain();
  master.connect(audio.destination);

  // The music first, the clock after: whatever rendering the first seconds
  // takes happens before there is a moment for them to be late for. See
  // startLive.
  const rendered = await startLive(from, master);

  const lead = 0.1;
  const t0 = audio.currentTime + lead;
  // The clock is kept in the score's own time, not in time since the button
  // was pressed, so everything downstream — the cursor, the highlight, the
  // end of the piece — needs no telling that it started in the middle.
  playAt = t0 - from;
  voices = [];

  let sounded = 0;
  for (const ev of pb.events) {
    if (ev.start + ev.dur > from) sounded++;
  }

  // Real instruments where there are any, and the oscillators where there
  // are not. Which one it is changes nothing else about playing: both end up
  // as sound scheduled against the same clock.
  master.gain.value = 1;
  if (rendered) {
    beginLive(t0);
  } else {
    master.gain.value = 0.22; // keep simultaneous notes from clipping
    scheduleOscillators(pb.events, from, t0, master);
  }
  // The click runs alongside whichever of those is playing the notes, on the
  // same clock and into a gain of its own. It is not one of the instruments.
  scheduleMetronome(pb.measures, from, t0);

  state.playing = true;
  // Starting is asking to be shown the music, so the page follows again
  // however it was left.
  userScrolled = false;
  followTop = null;
  // Starting anywhere but where we stopped means the walk down the bars has
  // to begin again rather than carry on from the last playing.
  cursorFrom = 0;
  $("#play-btn").classList.add("active");
  const where = from > 0
    ? ` from measure ${(cursorAt(pb.measures, from)?.measure ?? 0) + 1}`
    : "";
  // What is making the sound, which until now the line did not say.
  //
  // There are two answers and they sound nothing alike: the recorded
  // instruments of a soundfont, or the oscillators the editor falls back
  // to when there is none — and a score played through the second sounds
  // wrong in a way that is easy to blame on the score. Naming the
  // soundfont also settles "which one is it using", which otherwise means
  // opening a dialog to find out.
  status(`Playing${where} with ${playingWith()} — `
    + `${sounded} notes at ${pb.tempo} bpm`);

  // The cursor goes up before a note sounds, at the start of the music, so
  // pressing play is visibly answered even when the piece opens with rests —
  // Mozart's Introitus rests for nine bars before anyone sings.
  //
  // Not in a player, which lights the notes instead and has a clock
  // running in its own transport to answer the button. See follow.
  if (!state.player) makeCursor();
  const at0 = cursorAt(pb.measures, from);
  if (at0) {
    if (!state.player) moveCursor(at0);
    followCursor(at0);
  }
  follow(pb);
}

// playingWith is what is making the sound, in words.
//
// The soundfont by name where one is playing — live.font is what the core
// answered with when the performance began, so it is the bank actually
// being rendered through rather than whatever the settings say should be.
// Failing that, the editor's own voices, named as what they are: they are
// built from oscillators and do not pretend to be recordings, and somebody
// hearing a flute part as a sine wave is owed the reason.
function playingWith() {
  if (live && live.font) return live.font;
  return "the built-in voices";
}

// measureBox gives a measure's horizontal extent, shared by every staff
// showing it. The columns are aligned across parts by the engraver, so one
// range serves them all.
// measureBox is the box round one bar, across every staff it is drawn on.
//
// Taken from the index rather than by walking every region in the score:
// the playing cursor asks for this bar and the next one on every frame, and
// on a symphony a region is one of a hundred and thirty-eight thousand.
function measureBox(index) {
  let x0 = Infinity, x1 = -Infinity;
  let top = Infinity, bottom = -Infinity;
  for (const h of hitsIn(index)) {
    x0 = Math.min(x0, h.x0);
    x1 = Math.max(x1, h.x1);
    top = Math.min(top, h.staffTop);
    bottom = Math.max(bottom, h.staffTop + staffHeightOf(h));
  }
  if (x0 === Infinity) return null;
  return { x0, x1, top, bottom };
}

// How far down the bars the last search for the moment got. See cursorAt.
let cursorFrom = 0;

// cursorAt converts a moment into a place on the page.
//
// The cursor follows the timeline, not the notes. A score has one clock and
// many staves, each with its notes in different places, so a cursor built from
// note positions lurches backwards and forwards between parts. Walking the
// measures and sliding across each one keeps it moving the way a conductor's
// eye does.
function cursorAt(measures, now) {
  if (!measures || !measures.length) return null;

  // Walked on from where the last frame left off, not from the first bar.
  //
  // Starting at the beginning every time costs what the music has already
  // played: by the last movement of the ninth that is three thousand bars
  // looked at sixty times a second, and the following got heavier the
  // further in you went — worst exactly where the music is busiest.
  //
  // Playback only moves forward, so the next frame's bar is this one's or a
  // later one. A moment earlier than the last — somebody seeking, or a
  // repeat going back — starts the walk again. The bars are in playing
  // order, so their starts only ever increase even where their numbers do
  // not.
  if (cursorFrom >= measures.length || measures[cursorFrom].start > now) {
    cursorFrom = 0;
  }
  while (cursorFrom + 1 < measures.length &&
         measures[cursorFrom + 1].start <= now) {
    cursorFrom++;
  }
  const m = measures[cursorFrom];
  const box = measureBox(m.index);
  if (!box) return null;

  const f = m.dur > 0 ? Math.max(0, Math.min(1, (now - m.start) / m.dur)) : 0;
  return {
    x: cursorX(f, box, measureBox(m.index + 1)),
    top: box.top,
    bottom: box.bottom,
    measure: m.index,
  };
}

// cursorX turns a moment within a bar into a place along it.
//
// Linearly: the cursor crosses a bar at a steady rate, so it moves at the
// pace of the music rather than hurrying and dawdling between noteheads.
//
// It was briefly anchored to the notes instead, on the theory that the cursor
// should sit exactly on whatever is sounding. That cannot work while the
// engraver stretches each staff's bar by its own content: a half note on one
// staff and four eighths on another both fall halfway through the bar and are
// drawn at different places, so a list of anchors gathered across the staves
// and sorted by time has x values that do not increase. Interpolating through
// it walked the cursor backwards — small, but a cursor that ever goes
// backwards is worse than one that is a little ahead of a notehead.
//
// A steady rate is also the honest thing to draw. The cursor's job is to show
// where in the music you are, and that is a fact about time.
//
// The bar is crossed from its own start to where the next one begins, not to
// where its own music stops: a bar's region ends at its last notehead, and
// the barline and the air after it lie beyond. Stopping at the music left the
// cursor standing still at the end of every bar and then jumping the barline.
function cursorX(f, box, next) {
  let end = box.x1;
  // Only when the next bar really is to the right on the same staff line.
  // At the end of a system the next bar is back at the left margin, and the
  // cursor should finish this line rather than fly backwards across it.
  if (next && next.x0 > box.x0 && next.top === box.top) {
    end = next.x0;
  }
  return box.x0 + (end - box.x0) * f;
}

// systemSpan gives the vertical extent of every staff showing a measure, so
// the cursor can run the whole height of a system the way a conductor reads
// it, rather than sitting on one staff.
const STAFF_HEIGHT = 4;

// staffHeightOf is how tall one staff is, which is not always four spaces: a
// tablature has a line for each string. The engraver says, per staff; the
// constant is the answer for anything asked before a layout has arrived.
function staffHeightOf(hit) {
  return hit && hit.staffHeight > 0 ? hit.staffHeight : STAFF_HEIGHT;
}

function systemSpan(measure) {
  let top = Infinity;
  let bottom = -Infinity;
  for (const h of state.hits) {
    if (h.measure !== measure) continue;
    top = Math.min(top, h.staffTop);
    bottom = Math.max(bottom, h.staffTop + staffHeightOf(h));
  }
  if (top === Infinity) {
    // The measure is not on this page; fall back to the first system.
    const h = state.hits[0];
    if (!h) return null;
    return { top: h.staffTop, bottom: h.staffTop + staffHeightOf(h) };
  }
  return { top, bottom };
}

// eventIndex files a performance's events by the second each one begins in,
// and records how long the longest of them lasts.
//
// One bucket per second is fine enough: a frame then looks at the second it
// is in and at the few before it that could still be ringing, instead of at
// the whole piece.
function eventIndex(events) {
  const bins = new Map();
  let longest = 0;
  for (const ev of events || []) {
    const b = Math.floor(ev.start);
    const got = bins.get(b);
    if (got) got.push(ev);
    else bins.set(b, [ev]);
    if (ev.dur > longest) longest = ev.dur;
  }
  return { bins, longest };
}

// soundingAt is every event that could be ringing at a moment: the ones
// starting in this second and in the seconds a long note could reach across.
function soundingAt(when, at) {
  const out = [];
  const here = Math.floor(at);
  // A note cannot have started more than its own length ago and still be
  // sounding, so the longest note in the piece bounds how far back to look.
  const first = Math.floor(at - when.longest) - 1;
  for (let b = first; b <= here; b++) {
    const got = when.bins.get(b);
    if (got) out.push(...got);
  }
  return out;
}

// refIndex files a drawing's shapes by the number the engraver gave them.
//
// Lighting up a sounding note used to ask the drawing for the shapes with a
// given number, which on a symphony is an attribute selector run over nine
// hundred thousand of them — twice per note, sixty times a second. Filed
// once when the piece starts playing, it is a lookup.
//
// Built the first time it is asked for, because a score that is never played
// should not pay for it.
function refIndex() {
  // The shapes of each piece of the drawing, so that a piece arriving or
  // leaving costs only itself.
  //
  // Not a rebuild of the whole index: on a score long enough to be drawn in
  // pieces, the whole index is every shape on several screenfuls of
  // fifty-three staves, and rebuilding it each time a piece arrived — which
  // while the music plays is every few seconds — would cost more than the
  // search it was there to avoid.
  const map = new Map();      // ref -> the shapes carrying it
  const perTile = new Map();  // a piece of the drawing -> the refs in it
  let gen = -1;
  let host = null;
  const none = [];

  const take = (el) => {
    const refs = [];
    for (const n of el.querySelectorAll("[data-ref]")) {
      const k = n.getAttribute("data-ref");
      const got = map.get(k);
      if (got) got.push(n);
      else map.set(k, [n]);
      refs.push(k);
    }
    perTile.set(el, refs);
  };

  const drop = (el) => {
    for (const k of perTile.get(el) || []) {
      const got = map.get(k);
      if (!got) continue;
      // Whatever is still on the page. A shape in a piece that has been
      // taken away is no longer connected to it, which is the cheapest
      // true test there is and needs no bookkeeping of its own.
      const left = got.filter((n) => n.isConnected);
      if (left.length) map.set(k, left);
      else map.delete(k);
    }
    perTile.delete(el);
  };

  const sync = () => {
    const svg = currentSVG();
    if (!svg) return false;
    if (host !== svg) {
      // A different drawing altogether — another tune, or the score drawn
      // again from scratch. Nothing filed about the old one is any use.
      map.clear();
      perTile.clear();
      host = svg;
    }
    if (gen === drawGen && perTile.size > 0) return true;
    gen = drawGen;

    // The pieces on the page now. A score short enough to be drawn whole
    // has none, and is one piece: itself.
    const now = new Set();
    for (const g of tileEls.values()) {
      if (g) now.add(g);
    }
    if (now.size === 0) now.add(svg);

    for (const el of [...perTile.keys()]) {
      if (!now.has(el) || !el.isConnected) drop(el);
    }
    for (const el of now) {
      if (!perTile.has(el)) take(el);
    }
    return true;
  };

  const index = (ref) => {
    if (!sync()) return none;
    return map.get(String(ref)) || none;
  };
  // Whether the drawing has changed since this index last looked, so that
  // whoever is using it can put back anything it had marked on the shapes
  // that have gone.
  index.stale = () => gen !== drawGen;
  return index;
}

// follow moves the cursor and lights up whichever notehead is sounding. The
// engraver and the playback builder number elements identically, so an
// event's ref finds its glyph directly.
function follow(pb) {
  const total = pb.duration;
  // The events filed by the second they start in, and how long the longest
  // note in the piece lasts.
  //
  // What is sounding has to be worked out for every frame, and asking it of
  // every event in the performance means walking the whole piece sixty
  // times a second: Beethoven's ninth is a quarter of a million events, so
  // the question took longer to answer than the frame it was asked for and
  // playing it stopped dead. Filed by second, a frame looks at the handful
  // of events that could still be ringing.
  const when = eventIndex(pb.events);
  // And the glyphs filed by the number the engraver gave them, so that
  // lighting one up is a lookup rather than a search of the whole drawing.
  // It finds the drawing for itself each time: a long score is drawn a
  // screenful at a time, so the drawing is replaced underneath playback as
  // it moves along, and a list made from the one that has gone points at
  // nothing.
  const byRef = refIndex();

  const tick = () => {
    const now = playedSeconds();
    const shown = Math.max(0, Math.min(now, total));
    $("#clock").textContent = clock(shown);

    // And the harmonics panel, if it is open: while the music is playing
    // the moment it shows is wherever the cursor has got to. Asked at most
    // a few times a second — the answer cannot change faster than the notes
    // do, and the panel is read rather than watched.
    if (state.harm?.open) {
      state.harm.following = true;
      state.harm.at = shown;
      if (performance.now() - harmAsked > 200) {
        harmAsked = performance.now();
        void refreshHarmonics();
      }
    }

    // Where the cursor is, from the clock rather than from any one staff.
    const pt = cursorAt(pb.measures, shown);
    if (pt) {
      // Drawn, except in a player.
      //
      // The line crosses each bar at a steady rate because that is what
      // time does, and the engraver spaces a bar by what is in it — so
      // the two agree at the barlines and nowhere in between. The line
      // leaves the barline while the first notehead is still a little to
      // the right of it, and has overtaken by the middle of the bar. The
      // editor wants it anyway: it is the one thing on the page that says
      // where you are in the music rather than which note is sounding,
      // and it is where playing will pick up from if you pause.
      //
      // A player wants neither. The sounding noteheads are already lit,
      // exactly, on every staff at once — so the line adds nothing except
      // a second opinion about the same moment, and a reader watching the
      // two disagree believes the one that is wrong. See cursorX.
      if (!state.player) moveCursor(pt);
      followCursor(pt);
      lastMeasure = pt.measure;
    }

    // And which notes are sounding, which is not the same question: a note
    // keeps ringing after the cursor has moved past its head.
    //
    // Notes, plural. A score has as many lines as it has players and they
    // sound together — that is what a score is. Lighting up only the first
    // one found showed a Requiem for five voices as a solo.
    const sounding = new Set();
    let lead = null;
    for (const ev of soundingAt(when, shown)) {
      if (shown >= ev.start && shown < ev.start + ev.dur) {
        sounding.add(ev.ref);
        if (lead === null || ev.part === fingering.staff) lead = ev;
      }
    }
    // A piece of the score drawn since the last frame is new paper and
    // carries no marks. What is sounding may not have changed, so nothing
    // below would touch it, and the note being played would go dark and
    // stay dark for as long as it rang — which on a score long enough to be
    // drawn in pieces is from the first scroll onwards.
    const redrawn = byRef.stale();
    if (redrawn) current = new Set();

    if (redrawn || !sameSet(sounding, current)) {
      for (const ref of current) {
        if (sounding.has(ref)) continue;
        for (const n of byRef(ref)) n.classList.remove("playing");
      }
      // What has just come in is what gets a ring: a note held across thirty
      // frames was struck once, and should say so once.
      let struck = 0;
      for (const ref of sounding) {
        if (current.has(ref)) continue;
        for (const n of byRef(ref)) {
          n.classList.add("playing");
          // A tutti chord is fifty-four rings in one instant, which is a lot
          // of drawing for something nobody can pick out anyway. Past a
          // certain number the colour says it on its own.
          //
          // And nothing rings for paper that has only just been drawn: the
          // notes on it were struck a while ago and are merely still
          // sounding, so a ring there would be a second strike.
          if (!redrawn && struck < 24) ringAt(n);
        }
        struck++;
      }
      current = sounding;
      if (lead) updateFingering(lead);
    }

    if (now >= total + 0.3) {
      stop();
      window.dispatchEvent(new CustomEvent("gscore:ended"));
      // A page that embeds a scale or an exercise often wants it to go round
      // again, so the learner can keep playing along without reaching for
      // the mouse.
      if (state.loop) setTimeout(() => { if (state.loop) void play(); }, 400);
      return;
    }
    rafID = requestAnimationFrame(tick);
  };
  rafID = requestAnimationFrame(tick);
}

let lastMeasure = null;

// removeCursors takes every playback cursor off the page — the one being
// tracked, and any that is there without being tracked, which is the way
// this goes wrong.
function removeCursors() {
  cursorEl?.remove();
  cursorEl = null;
  currentSVG()?.querySelectorAll("rect.cursor").forEach((r) => r.remove());
}

function makeCursor() {
  const svg = currentSVG();
  if (!svg) return;
  // Take away whatever cursor is already on the page before putting one up.
  //
  // Playing used to be preceded by a stop, which removed it, so appending
  // was safe. Pausing keeps the cursor where it is on purpose — it is the
  // sign of where the music will pick up — and carrying on then left the old
  // one behind: a pale bar at every place the piece had ever been paused.
  removeCursors();
  cursorEl = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  cursorEl.setAttribute("class", "cursor");
  // A height from the start, or there is nothing to see until the first note.
  cursorEl.setAttribute("width", String(Math.max(2, state.zoom * 0.22)));
  cursorEl.setAttribute("height", String(7 * state.zoom));
  cursorEl.setAttribute("x", "-100");
  cursorEl.setAttribute("y", "0");
  svg.appendChild(cursorEl);
  lastMeasure = null;
}

// moveCursor puts the bar at a point, spanning every staff of the system.
//
// It puts the cursor back first when it has gone. The cursor is a rect inside
// the drawing, and a redraw replaces the drawing whole — so zooming while the
// music was playing left this holding an element that was no longer in the
// document, and the cursor vanished until playback was restarted. Checking
// here rather than at each redraw covers every way the drawing can be
// replaced, which is several and will be more.
function moveCursor(pt) {
  if (!pt) return;
  if (!cursorEl || !cursorEl.isConnected) {
    makeCursor();
    // The lit noteheads went with the old drawing as well, and the next tick
    // only repaints them when the chord changes — so forget which were lit
    // and let it light them again.
    current = new Set();
    if (!cursorEl) return;
  }
  const z = state.zoom;
  // A little above the top staff and below the bottom one, so it reads as one
  // line through the whole system.
  cursorEl.setAttribute("x", String((pt.x - 0.2) * z));
  cursorEl.setAttribute("y", String((pt.top - 1.5) * z));
  cursorEl.setAttribute("width", String(Math.max(2, z * 0.22)));
  cursorEl.setAttribute("height", String((pt.bottom - pt.top + 3) * z));
}

// cursorLead is how far across the view the cursor is held, as a fraction of
// its width. A third leaves most of a screen of music ahead of it.
const cursorLead = 0.32;

// tuneOffsetY is how far down the canvas the current tune's drawing starts.
//
// In a book every tune is engraved on its own and the drawings are stacked,
// so a position out of the engraver is measured from the top of its own tune
// and not from the top of the page. Scrolling to it without adding this took
// the view to the top of the first tune whichever one was playing — which
// looks exactly like playback having lost its place.
function tuneOffsetY() {
  if (!state.showAll) return 0;
  const svg = currentSVG();
  const wrap = $("#canvas-wrap");
  if (!svg || !wrap) return 0;
  return svg.getBoundingClientRect().top - wrap.getBoundingClientRect().top + wrap.scrollTop;
}

// visibleTune is the tune the canvas is showing, or null outside a book.
//
// With the whole book on screen the keyboard has no way of knowing which tune
// somebody means, so it means the one they are looking at. Scrolling down to
// the last tune and pressing space used to play the first one.
function visibleTune() {
  if (!state.showAll) return null;
  const wrap = $("#canvas-wrap");
  if (!wrap) return null;
  const view = wrap.getBoundingClientRect();
  const mid = view.top + view.height / 2;

  let best = null;
  let bestGap = Infinity;
  for (const block of $$("#canvas .tune")) {
    const r = block.getBoundingClientRect();
    // A tune across the middle of the view wins outright; otherwise the one
    // whose edge is nearest to it, so a tune taller or shorter than the
    // window is still chosen the way it looks.
    const gap =
      r.top <= mid && r.bottom >= mid
        ? 0
        : Math.min(Math.abs(r.top - mid), Math.abs(r.bottom - mid));
    if (gap < bestGap) {
      best = Number(block.dataset.tune);
      bestGap = gap;
    }
  }
  return best;
}

// ringAt leaves a ring at a notehead as it speaks.
//
// The colour says which notes are sounding; the ring says they started
// together, which on a score of many staves is the thing the eye cannot get
// from colour alone — fifty-four red heads look the same whether they were
// struck as a chord or arrived one at a time.
//
// Drawn and thrown away: it is a moment, not a state. Nothing reads it back,
// so it is put straight into the drawing and taken out when the animation
// says it is over — or shortly after, for a browser that never ran it and so
// never says so.
function ringAt(el) {
  const svg = currentSVG();
  if (!svg) return;

  // Where the notehead is, read off the shape rather than measured.
  //
  // getBBox is a measurement, and a measurement forces the browser to work
  // out the layout of the whole drawing before it can answer. On a
  // screenful of Beethoven's ninth that is twenty thousand shapes, and a
  // tutti chord asks for it two dozen times in one frame — which is the
  // playback going from smooth to stepping. A notehead is a glyph set at a
  // place, and the place is written on it.
  const x = Number(el.getAttribute("x"));
  const y = Number(el.getAttribute("y"));
  if (!Number.isFinite(x) || !Number.isFinite(y)) return;

  // A notehead is about one staff space tall and a little wider, and the
  // staff space is what the drawing is scaled by.
  const space = state.zoom || 8;

  const c = document.createElementNS("http://www.w3.org/2000/svg", "circle");
  c.setAttribute("class", "ping");
  c.setAttribute("cx", x + space * 0.58);
  c.setAttribute("cy", y - space * 0.1);
  c.setAttribute("r", Math.max(2.2, space * 0.5));
  svg.appendChild(c);

  const gone = () => c.remove();
  c.addEventListener("animationend", gone);
  setTimeout(gone, 1200);
}

// followCursor keeps the music ahead of the cursor on screen.
//
// The view is locked to the cursor and moves with it, every frame, the way a
// side-scrolling game's camera is locked to the player. What it replaces
// waited until the cursor had nearly left the screen and then jumped the view
// forward, which meant the notes about to be played were the ones you could
// not see — exactly backwards for reading along.
//
// Going back at a repeat needs nothing special. The cursor itself goes back
// to the bar the repeat opens at, because playback follows the written
// repeats, and the view follows the cursor.
function followCursor(pt) {
  const wrap = $("#canvas-wrap");
  if (!wrap || !pt) return;
  const z = state.zoom;
  const off = tuneOffsetY();

  // Hands off while the page is being moved by the person reading it.
  //
  // Setting a scroll offset does not merely move the page: it ends whatever
  // scrolling was already under way, momentum and all. Following sets the
  // sideways offset on every frame, so a two-finger scroll down the staves
  // was being cancelled sixty times a second — the gesture arrived, moved
  // the page a hair, and was killed before it could carry. From the chair
  // it looks as though the score will not scroll at all while it plays,
  // which is exactly what it was reported as.
  //
  // So for a moment after each gesture the music does not touch the page.
  // When the reader stops, following picks the cursor up again.
  if (readerIsScrolling()) return;

  const max = Math.max(0, wrap.scrollWidth - wrap.clientWidth);
  const want = Math.min(max, Math.max(0, pt.x * z - wrap.clientWidth * cursorLead));
  // Set rather than animated: it is already a frame at a time, and asking
  // for a smooth scroll sixty times a second makes each one fight the last.
  //
  // Only when it would move. Writing a scroll offset is not free even when
  // the number is the one already there: it ends whatever scrolling is
  // under way, which on a wrapped score is the smooth scroll down to the
  // next system that this function asked for a frame ago. Written every
  // frame, it cancelled that scroll before it could start, over and over
  // — so a wrapped score in a frame never moved, and the music played on
  // off the bottom of it while the reader scrolled by hand.
  if (Math.abs(wrap.scrollLeft - want) >= 1) wrap.scrollLeft = want;

  // Down the page the music does not move continuously — it steps from one
  // system to the next — so this moves only when the cursor has left the
  // one in view.
  const top = pt.top * z + off;
  const bottom = pt.bottom * z + off;
  // Whether a whole system fits in the room there is.
  const fits = bottom - top <= wrap.clientHeight;

  // Nothing to bring into view when the music is deeper than the window.
  //
  // A system of fifty-three staves is eight thousand pixels of it and the
  // window holds nine hundred, so no scroll position shows the cursor's
  // whole band. Taking the top of the band as the thing to show instead
  // pinned the page to the first few staves of the score for the whole
  // piece.
  //
  // There is no right answer to pick here: which of fifty-three staves to
  // watch is a decision, and it is the reader's. So the page is left where
  // they put it.
  //
  // A player is the other case, and the rule above turned it into a bug.
  // Its frame is sized to about one system on purpose — that is what an
  // embedded score opens at — so the band and the room for it are within a
  // few pixels of each other, and which side of this test a given score
  // falls on is a coin toss. A three-staff exercise in a frame nine pixels
  // too short stopped following at the first system and left the reader
  // scrolling by hand through a piece that was playing. And there is no
  // decision of the reader's to protect: there is nothing to edit, nothing
  // else on the screen, and one system is all the frame was ever going to
  // hold. So it follows anyway, and shows the top of the system.
  if (!fits && !state.player) return;

  // Whether the music is on screen, which is a different question when it
  // is taller than the screen: then the top of the system is as much of it
  // as any scroll position can show, and asking for all of it is a test
  // that never passes — which would leave following switched off from the
  // first time the reader touched the page.
  const inView = fits
    ? top >= wrap.scrollTop && bottom <= wrap.scrollTop + wrap.clientHeight
    : top >= wrap.scrollTop && top < wrap.scrollTop + wrap.clientHeight;
  if (inView) {
    // The music is on screen. Whatever you were looking at, you are looking
    // at it again, so following takes over from here.
    userScrolled = false;
    followTop = null;
    return;
  }
  // Out of view, which is either the music having moved on or you having
  // gone to look at something else. Only the first is ours to answer.
  if (userScrolled) return;

  // A little of the system above the fold, so it does not sit hard against
  // the top edge.
  //
  // Less of it where the system is already taller than the view, because
  // every pixel spent above it is a pixel of music lost — but not none:
  // what is measured here is the top staff *line*, and the ledger lines
  // and slurs of a high passage are drawn above that. At nothing the
  // flute's top notes were scrolled off the top of the frame, which is
  // the one part of a solo nobody wants cut.
  const lead = fits ? wrap.clientHeight * 0.3 : Math.min(wrap.clientHeight * 0.15, 4 * z);
  const target = Math.max(0, top - lead);
  // Asked for once per move, not once per frame.
  //
  // A smooth scroll takes a few hundred milliseconds to arrive, and for the
  // first of them the cursor is still outside the view that is on its way —
  // so this asked for the same scroll again on the next frame, and the one
  // after, each request restarting the easing from wherever the last had
  // got to. The view crept towards the new system instead of going to it,
  // and because the easing began again every frame it was always slowest
  // just as it should have been arriving. Sideways has never had the
  // problem, for the reason written above it: it is set, not animated.
  if (followTop !== null && Math.abs(followTop - target) < 1) return;
  followTop = target;
  wrap.scrollTo({ top: target, behavior: "smooth" });
}

// Whether the page belongs to the music or to the person reading it.
//
// Following drags the page to wherever the cursor is. That is what you want
// while you are watching it play and exactly what you do not want the moment
// you look somewhere else — the other staves of a big score, the bar you are
// about to fix — because the cursor is then out of view by definition and
// every frame drags you back to it.
//
// So scrolling up or down yourself takes the page off following, and
// bringing the music back into view puts it back on, without anything to
// press either way. Sideways is not affected: the strip is one long line and
// the cursor has to be kept on screen or there is nothing to watch.
let userScrolled = false;

// When the reader last moved the page themselves.
//
// Separate from userScrolled, which is about whether to follow down the
// page at all. This is about not writing to the page while a gesture is
// still in flight, whichever way it is going: a write ends the gesture.
let readerScrolledAt = 0;

// readerQuiet is how long after a gesture the music leaves the page alone.
//
// Long enough to cover the gaps between the events of one continuous
// scroll — a trackpad sends them in bursts — and short enough that letting
// go hands the page back to the music within about a page turn.
const readerQuiet = 400;

function readerIsScrolling() {
  return performance.now() - readerScrolledAt < readerQuiet;
}

// Where the view was last sent, so that one move down the page is one
// scroll and not one on every frame of it. See followCursor.
let followTop = null;

// setupFollowRelease watches for the scrolling being somebody's own.
//
// The gestures, not the scroll event: following sets scrollLeft on every
// frame, so by the time it is a scroll event there is no telling whose it
// was.
function setupFollowRelease() {
  const wrap = $("#canvas-wrap");
  if (!wrap) return;
  const mine = (e) => {
    // Any gesture at all means the page is theirs for the moment, sideways
    // included: a write would end that one too.
    readerScrolledAt = performance.now();
    // A sideways swipe is not looking away.
    if (e.type === "wheel" && !e.deltaY) return;
    if (state.playing) userScrolled = true;
  };
  wrap.addEventListener("wheel", mine, { passive: true });
  wrap.addEventListener("touchmove", mine, { passive: true });
}


// silence takes the sound and the clock down and leaves everything else
// where it is.
//
// Stopping and pausing both do this much. What divides them is what becomes
// of the place in the music: stopping forgets it, pausing keeps it.
function silence() {
  if (rafID) cancelAnimationFrame(rafID);
  rafID = null;
  endLive();
  // Nothing more is to be built. Without this the window went on filling
  // itself after the music had been stopped, and the piece played on.
  stopScheduling();
  // And the clicks, which are built on a window of their own: stopped
  // without this, the music went quiet and the metronome kept going.
  stopMetronome();
  for (const v of voices) {
    try {
      v.stop();
    } catch (_) {
      /* already finished */
    }
  }
  voices = [];
  const svg = currentSVG();
  // Clear every notehead that was lit, on every staff, and any ring still
  // on its way out — stopping should leave the page still, not fading.
  svg?.querySelectorAll(".playing").forEach((n) => n.classList.remove("playing"));
  svg?.querySelectorAll("circle.ping").forEach((n) => n.remove());
  current = new Set();
  state.playing = false;
  // The harmonics panel goes back to following what is picked up: with
  // nothing playing, the moment it should show is the one being written.
  if (state.harm) {
    state.harm.following = false;
    if (state.harm.open) void refreshHarmonics();
  }
  $("#play-btn").classList.remove("active");
}

// stop goes back to the beginning.
function stop() {
  silence();
  state.pausedAt = null;
  removeCursors();
  lastMeasure = null;
  $("#clock").textContent = "0:00";
}

// pause holds the music where it is, so that playing again carries on from
// there rather than starting the piece over.
//
// The cursor stays on the page and the clock keeps its reading. They are the
// only sign of where the music will pick up; taking them away leaves a paused
// score looking exactly like a stopped one, and then the next keypress is a
// guess.
function pause() {
  if (!state.playing) return;
  // The clock is kept in the score's own time, so this is a place in the
  // music and not a length of time since a button was pressed — which is
  // what makes it something playing can be handed back.
  const at = playedSeconds();
  silence();
  state.pausedAt = Math.max(0, at);
  state.pausedSel = selectionKey();
  status(lastMeasure === null
    ? `Paused at ${clock(state.pausedAt)}`
    : `Paused at measure ${lastMeasure + 1} — ${clock(state.pausedAt)}`);
}

// togglePlay is the one thing the space bar, the play button and an embedding
// page all reach, so that pausing and carrying on cannot mean one thing in
// one place and something else in another.
function togglePlay() {
  // Wake the sound up here, in the keypress or the click itself, before
  // anything at all is awaited.
  //
  // A browser only lets a page start making sound from something the user
  // did, and the permission belongs to that one turn of the event loop:
  // await anything — the playback, the engraved positions — and it is gone.
  // play() asks the context to resume well after both, which works only
  // because the context is usually running already from the last time. Let
  // it go idle, as stopping and leaving it a while does, and the resume
  // quietly does nothing: the notes are scheduled on a context that is not
  // running and the piece plays in silence.
  wakeAudio();
  if (state.playing) {
    pause();
    return undefined;
  }
  // Clicking a different note while paused is a new instruction and not a
  // resume: playing from what is selected is what clicking a note and
  // pressing the space bar has always meant, and being paused should not
  // quietly take that away.
  const moved = selectionKey() !== state.pausedSel;
  return play({ at: moved ? null : state.pausedAt });
}

// selectionKey is what is selected, as one short string, for telling whether
// it is still what it was.
function selectionKey() {
  const s = state.selection;
  if (!s) return "";
  return [s.part, s.measure, s.at, s.ref ?? "", state.multi.length].join(":");
}

function clock(sec) {
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return m + ":" + String(s).padStart(2, "0");
}

/* ---------- full screen ---------- */

// Giving the whole display to the score.
//
// This is a browser's own full screen, not a window the program manages, so
// it works wherever gscore is opened as a page — including the one published
// from the repository. A page can only ask for it from a real click, which is
// why there is no way to start up in it.
function fullscreenElement() {
  return document.fullscreenElement || document.webkitFullscreenElement || null;
}

async function toggleFullscreen() {
  try {
    if (fullscreenElement()) {
      const exit = document.exitFullscreen || document.webkitExitFullscreen;
      await exit.call(document);
      return;
    }
    const el = document.documentElement;
    const enter = el.requestFullscreen || el.webkitRequestFullscreen;
    if (!enter) {
      status("This window cannot go full screen");
      return;
    }
    await enter.call(el);
  } catch (err) {
    // A native window may refuse; the window's own controls still work.
    status("Could not go full screen: " + (err && err.message ? err.message : err));
  }
}

function setupFullscreen() {
  const sync = () => {
    const on = !!fullscreenElement();
    document.body.classList.toggle("fullscreen", on);
    $("#fs-label").textContent = on ? "Leave full screen" : "Full screen";
    $("#fs-btn").textContent = on ? "⤡" : "⤢";
    $("#fs-btn").title = (on ? "Leave full screen" : "Full screen") + " (F)";
  };
  document.addEventListener("fullscreenchange", sync);
  document.addEventListener("webkitfullscreenchange", sync);
  $("#fs-btn").addEventListener("click", toggleFullscreen);
  sync();
}

/* ---------- misc ---------- */

function status(msg) {
  // A message of its own replaces whatever the pointer was saying.
  hintWas = null;
  $("#status").textContent = msg;
}

/* ---------- saying that it is working ---------- */

// Opening a score is not instant on a big one. Beethoven's ninth is six
// megabytes of MuseScore, fifty-three staves and two and a half thousand
// bars, and reading it, laying it out and drawing it takes seconds. For
// those seconds the window did nothing at all and said nothing at all, which
// is indistinguishable from a window that has stopped working.
//
// So Gilbert comes up over it. Counted rather than switched, because one
// slow thing can start inside another and the inner one finishing does not
// mean the outer one has.
let busyDepth = 0;
let busyTimer = 0;

function busy(msg) {
  busyDepth++;
  const t = $("#busy-text");
  if (t && msg) t.textContent = msg;
  if (busyTimer) return;
  // Only once it has actually taken a moment. Most scores open faster than
  // the eye, and a mascot that appears and vanishes inside a frame reads as
  // a flicker rather than as progress.
  busyTimer = setTimeout(() => {
    busyTimer = 0;
    const el = $("#busy");
    if (el) el.hidden = false;
  }, 250);
}

// saveRequest asks the server to write a file, with Gilbert over the
// window while it does.
//
// Writing is not instant on a big score. A .gscore carries a PDF of the
// music inside it, and drawing one of Beethoven's ninth — fifty-three
// staves of two and a half thousand bars — takes half a minute. For that
// half minute the window said nothing, which is how somebody comes to
// look at the folder, see the file not there yet, and think it saved
// nothing.
//
// Gilbert waits a quarter of a second before appearing, so an ordinary
// save never shows him at all.
//
// Where the system's own save panel is part of the request — a save with
// no path asks the server to put the panel up — he comes up behind it
// while somebody is still choosing a name. That is the panel's own
// moment and he is behind it; when it closes he is already there, which
// is when he is wanted.
async function saveRequest(query, saying) {
  busy(saying);
  try {
    return await fetch("/api/save" + (query ? "?" + query : ""),
                       { method: "POST" });
  } finally {
    idle();
  }
}

function idle() {
  if (busyDepth > 0) busyDepth--;
  if (busyDepth > 0) return;
  if (busyTimer) {
    clearTimeout(busyTimer);
    busyTimer = 0;
  }
  const el = $("#busy");
  if (el) el.hidden = true;
}

function escapeHTML(s) {
  const d = document.createElement("div");
  d.textContent = String(s);
  return d.innerHTML;
}

// Ctrl or Command with a drag pans the score, and so does the middle button.
// In a long strip the music runs far off to the right, and reaching for a
// scrollbar to follow it is the wrong gesture.
let panned = false;
// A drag ends by firing a click as well. Without this the click that ends a
// rubber-band drag runs the ordinary click handler, which clears the very
// selection the drag just made — and, with a note tool held, writes a note.
let suppressClick = false;

// The rubber band.
//
// selection.c:320 tests only the x range — "(object_x + start_x - 30) >=
// x_origin && <= x" — against the staff the press landed on. So a drag picks
// up everything between two points on one staff, not a two-dimensional box,
// and that is what is reproduced here.
function setupMarquee() {
  const wrap = $("#canvas-wrap");
  let band = null;
  let startX = 0, startHit = null;

  wrap.addEventListener("mousedown", (e) => {
    // Shift and the left button. Ctrl and
    // Command are already panning and zooming, so Shift is the one left —
    // and holding it to select is what every other editor does anyway.
    if (!e.shiftKey) return;
    if (e.button !== 0 || e.ctrlKey || e.metaKey) return;
    // Even with selection off, a drag can still begin one in some browsers.
    e.preventDefault();
    const svg = currentSVG();
    if (!svg) return;
    const r = svg.getBoundingClientRect();
    const x = (e.clientX - r.left) / state.zoom;
    const y = (e.clientY - r.top) / state.zoom;

    const hit = hitAt(x, y);
    if (!hit) return;
    startHit = hit;
    startX = x;

    // As tall as the staff's music, not a fixed band around the staff.
    //
    // Seven spaces covers a staff and a ledger line either side, which is
    // most music and not this: a cadenza running four or five spaces above
    // the staff was selected correctly — the drag picks notes by where they
    // are along the bar, not by height — while the band drawn under the
    // pointer stopped short of them. So it looked as though the notes were
    // being missed, and the natural response is to drag again more
    // carefully, which changes nothing.
    let top = hit.staffTop - 1.6;
    let bottom = hit.staffTop + staffHeightOf(hit) + 1.6;
    for (const h of state.hits) {
      if (h.part !== hit.part || h.staffTop !== hit.staffTop) continue;
      for (const el of h.elements || []) {
        const a = Math.min(el.y, el.y2 ?? el.y) - 1.4;
        const b = Math.max(el.y, el.y2 ?? el.y) + 1.4;
        if (a < top) top = a;
        if (b > bottom) bottom = b;
      }
    }
    band = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    band.setAttribute("class", "marquee");
    band.setAttribute("y", String(top * state.zoom));
    band.setAttribute("height", String((bottom - top) * state.zoom));
    band.setAttribute("rx", String(0.45 * state.zoom));
    svg.appendChild(band);
    e.preventDefault();
  });

  window.addEventListener("mousemove", (e) => {
    if (!band) return;
    const svg = currentSVG();
    const r = svg.getBoundingClientRect();
    const x = (e.clientX - r.left) / state.zoom;
    const lo = Math.min(startX, x), hi = Math.max(startX, x);
    band.setAttribute("x", String(lo * state.zoom));
    band.setAttribute("width", String((hi - lo) * state.zoom));
  });

  window.addEventListener("mouseup", (e) => {
    if (!band) return;
    const svg = currentSVG();
    const r = svg.getBoundingClientRect();
    const x = (e.clientX - r.left) / state.zoom;
    // Clear anything the browser managed to select despite the above.
    window.getSelection()?.removeAllRanges();
    band.remove();
    band = null;

    const lo = Math.min(startX, x), hi = Math.max(startX, x);
    if (hi - lo < 0.5) return; // a click, not a drag
    suppressClick = true;

    // One staff, every element whose position falls in the range. The drag
    // may run past the measure it began in, so every measure of that staff is
    // considered.
    state.multi = [];
    for (const h of state.hits) {
      if (h.part !== startHit.part || h.staffTop !== startHit.staffTop) continue;
      for (const el of h.elements || []) {
        if (el.x >= lo && el.x <= hi) {
          state.multi.push({ part: h.part, measure: h.measure, index: el.index, ref: el.ref });
        }
      }
    }
    state.multi = orderedSelection();
    if (state.multi.length) {
      const f = state.multi[0];
      state.selection = {
        part: f.part, measure: f.measure, at: f.index + 1, step: 0, ref: f.ref,
      };
      // The far end of the drag is where shift with an arrow carries on
      // from, so a dragged selection grows the way a dragged one in a line
      // of text does.
      const l = state.multi[state.multi.length - 1];
      state.anchor = { part: f.part, measure: f.measure, index: f.index };
      state.focus = { part: l.part, measure: l.measure, index: l.index };
    }
    markSelection();
    status(state.multi.length
      ? `${state.multi.length} selected`
      : "Nothing in that range");
  });
}

// Dragging a symbol.
//
// A hand-drawn mark lands where the engraver puts it, which is above or below
// the note it belongs to. That is a guess, and the person who drew it knows
// better: a mark of their own invention may want to sit anywhere. So it can be
// picked up and moved, and where it is moved to is saved with the score as an
// offset from the note — not as a position on the page, which would be wrong
// again the moment anything around it changed.
function setupSymbolDragging() {
  const wrap = $("#canvas-wrap");
  let dragging = null;

  wrap.addEventListener("pointerdown", (e) => {
    // Any drawn symbol, whichever way it was drawn. A grid comes out as
    // an <image> and an outline as a <path>, and asking only for the
    // first made every symbol from the folder unpickable — visible on
    // the page and deaf to the mouse.
    const img = e.target.closest ? e.target.closest("[data-symbol]") : null;
    if (!img || e.button !== 0) return;
    const ref = Number(img.dataset.ref || 0);
    const where = elementByRef(ref);
    if (!where) return;

    e.preventDefault();
    e.stopPropagation();
    dragging = {
      img, where, name: img.dataset.symbol,
      startX: e.clientX, startY: e.clientY,
      x: parseFloat(img.getAttribute("x") || "0"),
      y: parseFloat(img.getAttribute("y") || "0"),
      // An outline is placed by its transform rather than by x and y, so
      // it is moved by adding one in front of the one it has.
      transform: img.getAttribute("transform") || "",
      isPath: img.tagName.toLowerCase() === "path",
    };
    // And it stays picked after the mouse lets go, so the arrow keys can
    // take over. A mark is put roughly where it goes with the mouse and
    // then settled with the keys, which is how anybody does this.
    pickSymbol({ ...where, name: img.dataset.symbol });
    img.setPointerCapture(e.pointerId);
    img.classList.add("dragging");
    status("Moving " + dragging.name);
  });

  wrap.addEventListener("pointermove", (e) => {
    if (!dragging) return;
    // Moved live, so it is clear what is happening; the score is redrawn
    // from the model once the mark is let go.
    const mx = e.clientX - dragging.startX;
    const my = e.clientY - dragging.startY;
    if (dragging.isPath) {
      dragging.img.setAttribute("transform",
        `translate(${mx},${my}) ` + dragging.transform);
      return;
    }
    dragging.img.setAttribute("x", String(dragging.x + mx));
    dragging.img.setAttribute("y", String(dragging.y + my));
  });

  const drop = async (e) => {
    if (!dragging) return;
    const d = dragging;
    dragging = null;
    d.img.classList.remove("dragging");
    suppressClick = true;

    // In staff spaces, which is what the model keeps: a nudge that still
    // means the same thing at any zoom and on any paper.
    const dx = (e.clientX - d.startX) / state.zoom;
    const dy = (e.clientY - d.startY) / state.zoom;
    if (Math.abs(dx) < 0.05 && Math.abs(dy) < 0.05) {
      status("Ready");
      return;
    }

    const p = new URLSearchParams({
      part: String(d.where.part), measure: String(d.where.measure),
      at: String(d.where.index + 1), name: d.name,
      dx: dx.toFixed(3), dy: dy.toFixed(3),
    });
    const res = await fetch("/api/movesymbol?" + p.toString(), { method: "POST" });
    if (!res.ok) {
      status("Could not move it: " + (await res.text()).trim());
      await refreshScore();
      return;
    }
    await refreshScore();
    status("Moved " + d.name);
  };
  wrap.addEventListener("pointerup", drop);
  wrap.addEventListener("pointercancel", drop);
}

// The symbol the arrow keys will move, or nothing.
//
// Kept here rather than read off the drawing, because the drawing is
// thrown away and made again after every edit: a selection held as an
// element of the page would be lost the moment the thing it points at
// moved, which is exactly when it is wanted.
let pickedSymbol = null;

function pickSymbol(what) {
  pickedSymbol = what;
  markPickedSymbol();
}

function dropSymbol() {
  if (!pickedSymbol) return;
  pickedSymbol = null;
  markPickedSymbol();
  status("Ready");
}

// markPickedSymbol outlines the one that is picked, so that it is clear
// what the arrow keys are about to move.
function markPickedSymbol() {
  $$("#canvas-wrap [data-symbol]").forEach((img) => {
    const where = elementByRef(Number(img.dataset.ref || 0));
    const mine = pickedSymbol && where &&
      where.part === pickedSymbol.part &&
      where.measure === pickedSymbol.measure &&
      where.index === pickedSymbol.index &&
      img.dataset.symbol === pickedSymbol.name;
    img.classList.toggle("picked", !!mine);
  });
}

// nudgeSymbol moves the picked symbol by a step, in staff spaces.
//
// A quarter of a space a press, and a whole one with shift: the first is
// small enough to settle a mark by eye and the second is large enough to
// cross the staff without holding the key down.
async function nudgeSymbol(dx, dy) {
  if (!pickedSymbol) return false;
  const p = new URLSearchParams({
    part: String(pickedSymbol.part), measure: String(pickedSymbol.measure),
    at: String(pickedSymbol.index + 1), name: pickedSymbol.name,
    dx: dx.toFixed(3), dy: dy.toFixed(3),
  });
  const res = await fetch("/api/movesymbol?" + p, { method: "POST" });
  if (!res.ok) {
    status("Could not move it: " + (await res.text()).trim());
    return true;
  }
  await refreshScore();
  markPickedSymbol();
  status(`Moved ${pickedSymbol.name}`);
  return true;
}

// elementByRef finds which measure and position an element's ref belongs to.
function elementByRef(ref) {
  if (!ref) return null;
  for (const h of state.hits) {
    for (const el of h.elements || []) {
      if (el.ref === ref) {
        return { part: h.part, measure: h.measure, index: el.index };
      }
    }
  }
  return null;
}

function setupPanning() {
  const wrap = $("#canvas-wrap");
  let panning = false;
  let startX = 0, startY = 0, scrollX = 0, scrollY = 0;

  wrap.addEventListener("mousedown", (e) => {
    const wants = e.button === 1 || ((e.ctrlKey || e.metaKey) && e.button === 0);
    if (!wants) return;
    e.preventDefault();
    panning = true;
    panned = false;
    startX = e.clientX;
    startY = e.clientY;
    scrollX = wrap.scrollLeft;
    scrollY = wrap.scrollTop;
    wrap.classList.add("panning");
  });

  window.addEventListener("mousemove", (e) => {
    if (!panning) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    // A few pixels of movement is a click with a modifier held, not a drag.
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) panned = true;
    wrap.scrollLeft = scrollX - dx;
    wrap.scrollTop = scrollY - dy;
  });

  window.addEventListener("mouseup", () => {
    if (!panning) return;
    panning = false;
    wrap.classList.remove("panning");
    // Let the click that ends the drag pass before accepting notes again.
    setTimeout(() => { panned = false; }, 0);
  });

  // macOS turns Ctrl-click into a context menu; suppress it over the score so
  // the gesture is usable there.
  wrap.addEventListener("contextmenu", (e) => {
    if (e.ctrlKey || e.metaKey) e.preventDefault();
  });
}

function setupZoomGestures() {
  // Ctrl or Command with the wheel zooms, which is what every other canvas
  // does; without the modifier the wheel scrolls as usual.
  $("#canvas-wrap").addEventListener(
    "wheel",
    (e) => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      zoomAccum -= e.deltaY;
      // The trackpad sends many small deltas; accumulate so one gesture is
      // not a dozen redraws.
      if (Math.abs(zoomAccum) >= 40) {
        setZoom(state.zoom + Math.sign(zoomAccum));
        zoomAccum = 0;
      }
    },
    { passive: false }
  );
}
let zoomAccum = 0;

/* ---------- the keyboard ---------- */

// Every key the editor answers to, in one table.
//
// It was a switch, and a switch is a fine way to write sixty bindings and a
// hopeless way to read them: nothing could list them for the person using
// the program, nothing could tell you two of them had quietly ended up on
// the same key, and nothing could let anybody change one. The keystroke is
// data now, and the switch is gone.
//
// A chord is written the way it is read, with the modifiers in a fixed order:
// Mod for ctrl-or-cmd — the same binding on both, because it is the same
// finger — then Alt, then Shift, then the key. Shift is only named for keys
// that do not print a character, since the character already says it: the
// tie is "T" and not "Shift+t".
//
// group is the heading the shortcuts window lists it under, and label is the
// line it reads. Both are for the person, so they say what the key does and
// not what the function is called.
const KEY_BINDINGS = [
  // Writing notes.
  { id: "write-note", keys: "Space", group: "Writing",
    label: "Write the note the palette is showing",
    run: () => void insertAtSelection(false) },
  { id: "write-rest", keys: "Enter", group: "Writing",
    label: "Write a rest of the same length",
    // A chord symbol picked up by clicking it is what Enter is about:
    // opening it for changing, with what is there already in the box. The
    // status line says so when one is selected, and nothing else is: a
    // rest written under a symbol somebody has just pointed at is not what
    // the key was pressed for.
    run: () => void (state.selection?.chordSymbol
      ? chordSymbolDialog()
      : state.selection?.textMark
        ? textDialog()
        : state.selection?.marker
          ? markerDialog()
          : insertAtSelection(true)) },
  { id: "chord-note", keys: "Mod+Space", group: "Writing",
    label: "Add another notehead to the note just written",
    run: () => void chordAtCursor() },
  { id: "split-measure", menu: "split-measure", keys: "Tab", group: "Writing",
    label: "End the bar here", run: () => action("split-measure") },
  { id: "add-measure", menu: "add-measure", keys: "m", group: "Writing",
    label: "Add a bar at the end", run: () => action("add-measure") },
  { id: "delete", menu: "delete", keys: "Backspace", group: "Writing",
    label: "Delete what is picked up", run: () => action("delete") },
  { id: "mouse-writes", menu: "note-input", keys: "i", group: "Writing",
    label: "Write with the mouse, rather than pointing with it",
    run: () => setMouseWrites(!state.mouseWrites) },
  { id: "type-music", menu: "type-music", keys: "Mod+k", group: "Writing",
    label: "Type the music in, at the cursor",
    run: () => toggleTypeBar() },

  // How long the next note is. The figures run down the values, with the
  // dotted ones beside the plain ones they lengthen.
  { id: "dur-whole", keys: "1", group: "Note lengths",
    label: "Whole note", run: () => pickDuration("whole", 0) },
  { id: "dur-half", keys: "2", group: "Note lengths",
    label: "Half note", run: () => pickDuration("half", 0) },
  { id: "dur-half-dot", keys: "3", group: "Note lengths",
    label: "Dotted half", run: () => pickDuration("half", 1) },
  { id: "dur-quarter", keys: "4", group: "Note lengths",
    label: "Quarter note", run: () => pickDuration("quarter", 0) },
  { id: "dur-quarter-dot", keys: "5", group: "Note lengths",
    label: "Dotted quarter", run: () => pickDuration("quarter", 1) },
  { id: "dur-eighth", keys: "8", group: "Note lengths",
    label: "Eighth note", run: () => pickDuration("eighth", 0) },
  { id: "dur-sixteenth", keys: "9", group: "Note lengths",
    label: "Sixteenth note", run: () => pickDuration("sixteenth", 0) },
  { id: "dur-32nd", keys: "0", group: "Note lengths",
    label: "Thirty-second note", run: () => pickDuration("32nd", 0) },
  { id: "dot", keys: ".", group: "Note lengths",
    label: "Dot what is picked up — again for two dots, three, then none",
    run: () => void dotSelection() },

  // Accidentals, on the keys that mean raise and lower everywhere else.
  { id: "sharpen", keys: "+", group: "Accidentals",
    label: "Sharpen — again for a double sharp, again for none",
    run: () => void cycleAccidental(1) },
  { id: "flatten", keys: "-", group: "Accidentals",
    label: "Flatten — again for a double flat, again for none",
    run: () => void cycleAccidental(-1) },
  // The signs as they are written, which is what a hand reaches for: a
  // sharp is #, a flat is _ and a natural is =. They do what the palette's
  // buttons do — write the sign on what is picked up, or hold it for the
  // next note where nothing is.
  { id: "accidental-sharp", keys: "#", group: "Accidentals",
    label: "Sharp", run: () => typeAccidental("sharp", "Sharp") },
  { id: "accidental-flat", keys: "_", group: "Accidentals",
    label: "Flat", run: () => typeAccidental("flat", "Flat") },
  { id: "accidental-natural", keys: "=", group: "Accidentals",
    label: "Natural — or tie, with nothing picked up",
    run: () => {
      // The sign that belongs between + and -, which is what the key is
      // next to and what = means wherever a tune is typed out in words.
      //
      // It took the written accidental away instead, and that looks like
      // nothing happening: the note goes back to whatever the key signature
      // says for it and no sign is drawn at all. What was wanted was the
      // natural — the sign that says this one is not sharp, out loud, which
      // is the whole reason anybody writes one. Pressing it again on a note
      // that already has one still takes it off, because every accidental
      // here is its own undo.
      //
      // The key is free when nothing is picked up, and tying two notes is
      // what it has always done there.
      if (pickedNotes()) void setAccidental("natural", "Natural");
      else action("tie");
    } },

  // Marks on the notes.
  { id: "tie", menu: "tie", keys: "T", group: "Marks",
    label: "Tie", run: () => action("tie") },
  { id: "slur", menu: "slur", keys: "S", group: "Marks",
    label: "Slur", run: () => action("slur") },
  { id: "beam", menu: "beam-join", keys: "b", group: "Marks",
    label: "Beam the selected notes together — on one note, break the beam before it",
    run: () => action("beam-here") },
  { id: "stem-up", menu: "stem-up", keys: "u", group: "Marks",
    label: "Stem up", run: () => action("stem-up") },
  { id: "stem-down", menu: "stem-down", keys: "d", group: "Marks",
    label: "Stem down", run: () => action("stem-down") },
  { id: "repeat-bar", menu: "repeat-bar", keys: "r", group: "Marks",
    label: "Repeat the bar before", run: () => action("repeat-bar") },
  { id: "insert-text", menu: "insert-text", keys: "t", group: "Marks",
    label: "Write a word over the music", run: () => action("insert-text") },
  { id: "insert-chord", menu: "insert-chord", keys: "c", group: "Marks",
    label: "Chord symbol", run: () => action("insert-chord") },
  { id: "set-lyrics", menu: "set-lyrics", keys: "l", group: "Marks",
    label: "Lyrics", run: () => action("set-lyrics") },
  { id: "fingering", keys: "F2", group: "Marks",
    label: "Show the fingering", run: () => toggleFingering() },

  // The marks that go on a note, on the keys NoteWorthy puts them on.
  //
  // Each of these was already a menu item and a palette button and nothing
  // else, which is three gestures for a staccato dot. They are typed far
  // more often than anything else in a score is, and the keys are the ones
  // a hand that has written music before already knows: the mark's own
  // shape where it has one, so > is a diminuendo and ^ a marcato.
  { id: "artic-accent", menu: "artic-accent", keys: "!", group: "Marks",
    label: "Accent", run: () => action("artic-accent") },
  { id: "artic-marcato", menu: "artic-marcato-up", keys: "^", group: "Marks",
    label: "Marcato", run: () => action("artic-marcato-up") },
  { id: "artic-staccato", menu: "artic-staccato", keys: ",", group: "Marks",
    label: "Staccato", run: () => action("artic-staccato") },
  { id: "artic-staccatissimo", menu: "artic-staccatissimo", keys: "'", group: "Marks",
    label: "Staccatissimo", run: () => action("artic-staccatissimo") },
  // The underscore pictures a tenuto, and it is also how a flat is written
  // wherever music is typed rather than drawn. The flat has it: an
  // accidental is reached far oftener than a tenuto, and nothing else
  // spells one. No free key pictures a tenuto, so this one sits with the
  // other articulations instead — , is staccato and ’ staccatissimo,
  // and : is the next key along.
  { id: "artic-tenuto", menu: "artic-tenuto", keys: ":", group: "Marks",
    label: "Tenuto", run: () => action("artic-tenuto") },
  { id: "hairpin-cresc", menu: "hairpin-cresc", keys: "<", group: "Marks",
    label: "Crescendo over what is picked up", run: () => action("hairpin-cresc") },
  { id: "hairpin-dim", menu: "hairpin-dim", keys: ">", group: "Marks",
    label: "Diminuendo over what is picked up", run: () => action("hairpin-dim") },
  // The tie and the slur already answer to T and S. These are the keys
  // NoteWorthy uses, kept as well rather than instead: a second key for a
  // thing already bound costs nothing, and either hand is right.
  { id: "tie-slash", keys: "/", group: "Marks",
    label: "Tie — the same as T", run: () => action("tie") },
  { id: "slur-semicolon", keys: ";", group: "Marks",
    label: "Slur — the same as S", run: () => action("slur") },

  // Tuplets, where every other notation program puts them. Written out one
  // by one rather than worked out from a list, because this table is read as
  // often as it is run: a row nobody can grep for is a row nobody can check.
  { id: "tuplet-1", keys: "Mod+1", group: "Tuplets",
    label: "No tuplet \u2014 the group back to one note",
    run: () => void setTuplet(1) },
  { id: "tuplet-2", keys: "Mod+2", group: "Tuplets",
    label: "Duplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(2) },
  { id: "tuplet-3", keys: "Mod+3", group: "Tuplets",
    label: "Triplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(3) },
  { id: "tuplet-4", keys: "Mod+4", group: "Tuplets",
    label: "Quadruplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(4) },
  { id: "tuplet-5", keys: "Mod+5", group: "Tuplets",
    label: "Quintuplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(5) },
  { id: "tuplet-6", keys: "Mod+6", group: "Tuplets",
    label: "Sextuplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(6) },
  { id: "tuplet-7", keys: "Mod+7", group: "Tuplets",
    label: "Septuplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(7) },
  { id: "tuplet-8", keys: "Mod+8", group: "Tuplets",
    label: "Octuplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(8) },
  { id: "tuplet-9", keys: "Mod+9", group: "Tuplets",
    label: "Nonuplet \u2014 on a selection, those notes become the group",
    run: () => void setTuplet(9) },

  // Moving about, and what is picked up.
  { id: "cursor-left", keys: "ArrowLeft", group: "Selecting and moving",
    label: "Back one note — or move the selection back",
    run: () => { if (state.multi.length) slideSelection(-1); else moveCursorBy(-1); } },
  { id: "cursor-right", keys: "ArrowRight", group: "Selecting and moving",
    label: "On one note — or move the selection on",
    run: () => { if (state.multi.length) slideSelection(1); else moveCursorBy(1); } },
  { id: "extend-left", keys: "Shift+ArrowLeft", group: "Selecting and moving",
    label: "Take the selection one note further back",
    run: () => extendSelection(-1) },
  { id: "extend-right", keys: "Shift+ArrowRight", group: "Selecting and moving",
    label: "Take the selection one note further on",
    run: () => extendSelection(1) },
  // One note picked up is picked up, the same as twenty. Asked of the
  // dragged selection alone, these moved the cursor instead wherever a note
  // had been picked up by clicking it — and moving the cursor lets go of
  // the note, so the arrow appeared to do nothing and quietly dropped the
  // selection as well.
  { id: "cursor-up", menu: "pitch-up", keys: "ArrowUp", group: "Selecting and moving",
    label: "Up a step — or move the selected notes up",
    run: () => { if (heldForMoving()) void nudge(1); else moveCursorPitch(1); } },
  { id: "cursor-down", menu: "pitch-down", keys: "ArrowDown", group: "Selecting and moving",
    label: "Down a step — or move the selected notes down",
    run: () => { if (heldForMoving()) void nudge(-1); else moveCursorPitch(-1); } },
  { id: "cursor-up-octave", menu: "octave-up", keys: "Shift+ArrowUp", group: "Selecting and moving",
    label: "Up an octave",
    run: () => { if (heldForMoving()) void nudge(7); else moveCursorPitch(7); } },
  { id: "cursor-down-octave", menu: "octave-down", keys: "Shift+ArrowDown", group: "Selecting and moving",
    label: "Down an octave",
    run: () => { if (heldForMoving()) void nudge(-7); else moveCursorPitch(-7); } },
  { id: "nudge-up", keys: "Alt+ArrowUp", group: "Selecting and moving",
    label: "Move the note up a step", run: () => void nudge(1) },
  { id: "nudge-down", keys: "Alt+ArrowDown", group: "Selecting and moving",
    label: "Move the note down a step", run: () => void nudge(-1) },
  // Moving the music to another key, which is not the same thing as moving
  // notes about the staff: a semitone here is a minor second, so the notes
  // land spelled the way the new key spells them — C to D flat going up,
  // and not to a C sharp the signature knows nothing about.
  { id: "shift-earlier", keys: "Mod+ArrowLeft", group: "Selecting and moving",
    label: "Move the notes themselves earlier", run: () => void shiftPassage(-1) },
  { id: "shift-later", keys: "Mod+ArrowRight", group: "Selecting and moving",
    label: "Move the notes themselves later", run: () => void shiftPassage(1) },
  { id: "select-all", menu: "select-all", keys: "Mod+a", group: "Selecting and moving",
    label: "Pick up the whole staff", run: () => selectAll() },
  { id: "clear-selection", keys: "Escape", group: "Selecting and moving",
    label: "Stop playing, or let go of the selection",
    run: () => { if (state.playing || state.pausedAt !== null) stop(); else clearSelection(); } },
  { id: "jump", menu: "jump", keys: "j", group: "Selecting and moving",
    label: "Go to a bar", run: () => action("jump") },
  { id: "jump-mod", keys: "Mod+g", group: "Selecting and moving",
    label: "Go to a bar", run: () => void jumpToMeasure() },

  // The clipboard and the history, where every other program puts them.
  { id: "copy", menu: "copy", keys: "Mod+c", group: "Editing",
    label: "Copy", run: () => void copySelection() },
  { id: "cut", menu: "cut", keys: "Mod+x", group: "Editing",
    label: "Cut", run: () => void cutSelection() },
  { id: "paste", menu: "paste", keys: "Mod+v", group: "Editing",
    label: "Paste", run: () => void pasteClipboard() },
  { id: "undo", menu: "undo", keys: "Mod+z", group: "Editing",
    label: "Undo", run: () => void stepHistory("undo") },
  { id: "redo", menu: "redo", keys: "Mod+Z", group: "Editing",
    label: "Redo", run: () => void stepHistory("redo") },
  { id: "redo-y", keys: "Mod+y", group: "Editing",
    label: "Redo", run: () => void stepHistory("redo") },

  // Playing it.
  { id: "play", keys: "Shift+Space", group: "Playing",
    label: "Play, pause, and carry on from where it paused",
    run: () => void togglePlay() },
  { id: "play-from-start", keys: "Mod+Shift+Space", group: "Playing",
    label: "Play from the beginning", run: () => void playFromStart() },

  // The file, and the window.
  { id: "new", menu: "new", keys: "n", group: "The file",
    label: "New score", run: () => action("new") },
  { id: "open", menu: "open", keys: "o", group: "The file",
    label: "Open", run: () => action("open") },
  // And the key every other program on the machine saves with, which is
  // Mod — ⌘ on a Mac, Ctrl everywhere else. The bare letter stays: it is
  // how the rest of these read, and taking it away would move the key
  // under the hands of anybody already using it.
  { id: "save", menu: "save", keys: "s", also: "Mod+s", group: "The file",
    label: "Save", run: () => action("save") },
  { id: "print", menu: "print", keys: "p", group: "The file",
    label: "Print", run: () => action("print") },
  { id: "save-pdf", menu: "save-pdf", keys: "P", group: "The file",
    label: "Export as PDF", run: () => action("save-pdf") },
  { id: "save-svg", menu: "save-svg", keys: "e", group: "The file",
    label: "Export as SVG", run: () => action("save-svg") },
  { id: "save-page", menu: "save-page", keys: "H", group: "The file",
    label: "Export as a web page", run: () => action("save-page") },
  { id: "fullscreen", keys: "f", group: "The window",
    label: "Fill the screen", run: () => action("fullscreen") },
  { id: "fullscreen-f11", keys: "F11", group: "The window",
    label: "Fill the screen", run: () => action("fullscreen") },
  { id: "zoom-in", keys: "Mod++", group: "The window",
    label: "Bigger", run: () => setZoom(state.zoom + 1) },
  { id: "zoom-in-equals", keys: "Mod+=", group: "The window",
    label: "Bigger", run: () => setZoom(state.zoom + 1) },
  { id: "zoom-out", keys: "Mod+-", group: "The window",
    label: "Smaller", run: () => setZoom(state.zoom - 1) },
  { id: "zoom-out-under", keys: "Mod+_", group: "The window",
    label: "Smaller", run: () => setZoom(state.zoom - 1) },
  { id: "zoom-reset", keys: "Mod+0", group: "The window",
    label: "Back to the usual size", run: () => setZoom(8) },
];

// chordParts splits a keystroke into the modifiers and the key.
//
// Plus is both the separator and a key, so "Mod++" splits into three pieces
// of which the last two are empty: the key is the plus that was lost between
// them.
function chordParts(chord) {
  const parts = chord.split("+");
  if (parts.length > 1 && parts[parts.length - 1] === "") {
    return { mods: parts.slice(0, -2), key: "+" };
  }
  return { mods: parts.slice(0, -1), key: parts[parts.length - 1] };
}

// shortcutText is the keystroke as it should be read, which is not quite how
// it is stored: Mod is a key with a different name on each platform, and the
// space bar is not a space.
function shortcutText(chord, compact) {
  const mac = navigator.platform.startsWith("Mac");
  const { mods, key } = chordParts(chord);
  // A capital letter is shift and that letter, and reading it as the letter
  // alone would have the tie look like a press of t.
  const named = /^[A-Z]$/.test(key) && !mods.includes("Shift")
    ? mods.concat("Shift") : mods;
  // Keycaps are read in upper case whether or not shift is held — the key
  // next to A is S, and what distinguishes Save from Slur is the arrow
  // above, not the case of the letter.
  const shown = named.map((m) => {
    if (m === "Mod") return mac ? "\u2318" : "Ctrl";
    if (m === "Alt") return mac ? "\u2325" : "Alt";
    if (m === "Shift") return "\u21e7";
    return m;
  });
  shown.push(((part) => {
    if (/^[a-z]$/.test(part)) return part.toUpperCase();
    switch (part) {
      case "Space": return "Space";
      case "ArrowLeft": return "\u2190";
      case "ArrowRight": return "\u2192";
      case "ArrowUp": return "\u2191";
      case "ArrowDown": return "\u2193";
      case "Enter": return "\u21b5";
      case "Backspace": return "\u232b";
      case "Escape": return "Esc";
      case "Tab": return "\u21e5";
    }
    return part;
  })(key));
  return shown.join(compact ? "" : " ");
}

// syncMenuKeys writes the keys into the menus.
//
// The hints were in the markup, which was fine while the keys could not
// change and is a promise the program cannot keep now that they can: a menu
// still offering Ctrl+S after somebody moved Save is worse than no hint. The
// dynamics and the hairpins keep theirs, because what is in their kbd is the
// name of the marking and not a key at all.
/*
 * The pictures in the menus.
 *
 * gscore 0.0.9 drew the mark beside every menu entry that writes one, so a
 * staccato was found by looking for the dot rather than by reading seven
 * words. Asked for again: "I had icons in menu to help the user to quickly
 * find what they wanted."
 *
 * They are glyphs from the score's own font, so a menu entry shows exactly
 * the mark it writes — not a picture of one. Entries that write nothing on
 * the page have none, because an icon that stands for nothing is one more
 * thing to read.
 *
 * Colour carries the meaning, as it did in 0.0.9: what the entry writes is
 * red, and anything there only to show where it goes is the black of the
 * text around it. So a staccato is a black note with a red dot under it,
 * and an accidental — which is the whole of what that entry writes — is
 * red on its own. "It helps to know what it does more easily."
 *
 * Two characters means a note with a mark on it: the first is the note and
 * the second is the mark, hung under it and centred by its own ink so that
 * a dot and a wedge both land where they should.
 *
 * The names in the comments are the canonical SMuFL ones, so the characters
 * can be checked against the font rather than taken on trust.
 */
const MENU_ICONS = {
  // Going somewhere, and taking music with you.
  //
  // The arrow pointing along is what the old program put beside "jump to
  // measure", and it is the right picture: the others here move a note up
  // or down and this one moves you along the piece. Transposing is the
  // music moving up and along at once, which is what the diagonal says.
  "jump": "\ueb62",               // arrowBlackRight
  // The end of a staff is further along the same arrow, with the barline
  // it stops at: a thin and a thick line, which is how a piece ends.
  "staff-end": "\ue032",          // barlineFinal
  "transpose": "\ueb61",          // arrowBlackUpRight

  // Moving a note up and down the staff.
  "pitch-up": "\ueb60",            // arrowBlackUp
  "pitch-down": "\ueb64",          // arrowBlackDown
  "octave-up": "\ueb60",           // arrowBlackUp
  "octave-down": "\ueb64",         // arrowBlackDown

  // Stems: the note drawn the way its stem will point says what the entry
  // does, and a bare stem glyph says nothing at all.
  "stem-up": "\ue1d5",             // noteQuarterUp
  "stem-down": "\ue1d6",           // noteQuarterDown

  // Accidentals.
  "acc-sharp": "\ue262",           // accidentalSharp
  "acc-flat": "\ue260",            // accidentalFlat
  "acc-natural": "\ue261",         // accidentalNatural
  "acc-dsharp": "\ue263",          // accidentalDoubleSharp
  "acc-dflat": "\ue264",           // accidentalDoubleFlat
  "acc-parens": "\ue26a",          // accidentalParensLeft

  // Articulations, each on the note it is written on. Under it, because the
  // note is drawn with its stem up and an articulation goes on the side
  // away from the stem.
  "artic-staccato": "\ue1d5\ue4a3",      // noteQuarterUp articStaccatoBelow
  "artic-staccatissimo": "\ue1d5\ue4a7", // noteQuarterUp articStaccatissimoBelow
  "artic-tenuto": "\ue1d5\ue4a5",        // noteQuarterUp articTenutoBelow
  "artic-accent": "\ue1d5\ue4a1",        // noteQuarterUp articAccentBelow
  // The two marcatos differ by which way the wedge points, so they show the
  // wedge they write rather than both showing the same one.
  "artic-marcato-up": "\ue1d5\ue4ac",    // noteQuarterUp articMarcatoAbove
  "artic-marcato-down": "\ue1d5\ue4ad",  // noteQuarterUp articMarcatoBelow

  // Where the player breathes: the sign itself, since each of these is a
  // different sign rather than a different way of writing one. They go
  // after the note, so the note comes first here too.
  "breath-comma": "\ue1d5\ue4ce",        // noteQuarterUp breathMarkComma
  "breath-tick": "\ue1d5\ue4cf",         // noteQuarterUp breathMarkTick
  "breath-caesura": "\ue1d5\ue4d1",      // noteQuarterUp caesura
  "breath-caesura-curved": "\ue1d5\ue4d4", // noteQuarterUp caesuraCurved

  // Ties and slurs.
  "slur": "\ue1fd",                // textTie
  "tie": "\ue1fd",                 // textTie
  "tie-head": "\ue1fd",            // textTie

  // What is written on a bar.
  "bar-single": "\ue030",          // barlineSingle
  "bar-double": "\ue031",          // barlineDouble
  "bar-final": "\ue032",           // barlineFinal
  "bar-open": "\ue040",            // repeatLeft
  "bar-close": "\ue041",           // repeatRight
  "repeat-bar": "\ue500",          // repeat1Bar
  "repeat-2bars": "\ue501",        // repeat2Bars

  // Dynamics.
  "dyn-pppp": "\ue529",            // dynamicPPPP
  "dyn-ppp": "\ue52a",             // dynamicPPP
  "dyn-pp": "\ue52b",              // dynamicPP
  "dyn-p": "\ue520",               // dynamicPiano
  "dyn-mp": "\ue52c",              // dynamicMP
  "dyn-mf": "\ue52d",              // dynamicMF
  "dyn-f": "\ue522",               // dynamicForte
  "dyn-ff": "\ue52f",              // dynamicFF
  "dyn-fff": "\ue530",             // dynamicFFF

  // Lines over a stretch of music.
  "hairpin-cresc": "\ue53e",       // dynamicCrescendoHairpin
  "hairpin-dim": "\ue53f",         // dynamicDiminuendoHairpin
  "octave-8va": "\ue511",          // ottavaAlta
  "octave-8vb": "\ue51c",          // ottavaBassaVb
  "octave-15ma": "\ue515",         // quindicesimaAlta
  "octave-15mb": "\ue51d",         // quindicesimaBassaMb

  // Tuplets and tremolos.
  "tuplet-3": "\ue883",            // tuplet3
  "tuplet-4": "\ue884",            // tuplet4
  "tuplet-5": "\ue885",            // tuplet5
  "tuplet-6": "\ue886",            // tuplet6
  "tuplet-7": "\ue887",            // tuplet7
  "tuplet-8": "\ue888",            // tuplet8
  "tuplet-9": "\ue889",            // tuplet9
  "tremolo-1": "\ue220",           // tremolo1
  "tremolo-2": "\ue221",           // tremolo2
  "tremolo-3": "\ue222",           // tremolo3
  "tremolo-4": "\ue223",           // tremolo4
  "tremolo-pair-1": "\ue220",      // tremolo1
  "tremolo-pair-2": "\ue221",      // tremolo2
  "tremolo-pair-3": "\ue222",      // tremolo3
  "tremolo-buzz": "\ue22a",        // buzzRoll

  // How a fretted instrument plays the note. The font carries the ones
  // that are signs — the vibrato strokes, the tapping hand, the harmonic
  // circle, the laissez vibrer tick — and the arrows the bends are drawn
  // with. The rest are drawn, down in MENU_DRAWINGS: a hammer-on is a
  // slur with a letter over it, and no font has that.
  "tab-bend": "\ueb61",            // arrowBlackUpRight
  "tab-bend-half": "\ueb71",       // arrowOpenUpRight
  "tab-bend-release": "\ueb63",    // arrowBlackDownRight
  "tab-pre-bend": "\ueb60",        // arrowBlackUp
  "tab-vibrato": "\ueab2",         // guitarVibratoStroke
  "tab-wide-vibrato": "\ueab3",    // guitarWideVibratoStroke
  "tab-palm-mute": "\ue638",       // pluckedDamp
  "tab-let-ring": "\ue4ba",        // articLaissezVibrerAbove
  "tab-natural-harmonic": "\ue614", // stringsHarmonic
  "tab-tap": "\ue841",             // guitarRightHandTapping
  "tab-dead-note": "\ue0a9",       // noteheadXBlack

  // The metronome mark: the note the beats are counted in, which is the
  // left half of what the mark says.
  "view-tempomark": "\ueca5",      // metNoteQuarterUp
  "view-beatmarks": "\ueb64",      // arrowBlackDown: the mark it draws
  "metronome": "\ueca5",           // metNoteQuarterUp: the beat it ticks
  "note-colours": "\ue0a4",       // noteheadBlack: the head it colours

  // What is written on a note or a staff.
  "insert-note": "\ue1d5",         // noteQuarterUp
  "insert-rest": "\ue4e5",         // restQuarter
  "set-key": "\ue262",             // accidentalSharp
  "set-time": "\ue08a",            // timeSigCommon
  "group-brace": "\ue000",         // brace
  "group-bracket": "\ue002",       // bracket

  // Writing notes: the same note the entry is about to put down.
  "note-input": "\ue1d5",          // noteQuarterUp

  // Bars. A plain barline for the one that adds a bar, and the dashed one
  // for the entry that cuts a bar in two — a dashed line is where a thing
  // is to be divided, which is what the entry does.
  "add-measure": "\ue030",         // barlineSingle
  // Bars pushed in rather than added on: the repeat-dots barline, which
  // is the one that reads as "and more of it here".
  "insert-measures": "\ue034",     // barlineHeavy
  // And the bar taken out again: the final barline, which is what the end
  // of the music looks like once the spare bars after it have gone.
  "remove-measure": "\ue032",      // barlineFinal
  "split-measure": "\ue036",       // barlineDashed
  "track-barlines": "\ue030",      // barlineSingle

  // The guitar frame, which is the one of these the font really has.
  "chord-builder": "\ue856",       // fretboard6String

  // The tempo mark, which is written as a note and a number.
  "set-tempo": "\ueca5",           // metNoteQuarterUp

  // Staves that are not five lines of pitches, and what is written on them.
  "tab": "\ue06d",                 // 6stringTabClef
  // Moving the written pitch of a tab staff is transposing, so it is the
  // arrow the transpose entry uses and not another tab clef: the entry is
  // about the move, not about the staff.
  "tab-transposition": "\ueb61",   // arrowBlackUpRight
  // Moving a note along the neck is a move up or down the page, and that
  // is what the arrows say. Not a string number: the entry is about
  // taking the note somewhere, not about where it ends up.
  "string-up": "\ueb60",           // arrowBlackUp
  "string-down": "\ueb64",         // arrowBlackDown
  "fingering": "\ued11",           // fingering1
  "add-grand-staff": "\ue000",     // brace

  // What a run of empty bars is drawn as, which is what the entry turns on.
  "view-multirest": "\ue4ee",      // restHBar

  // Looking at the music rather than writing it.
  "harmonics": "\ue614",           // stringsHarmonic
  "rhythm": "\ue101",              // noteheadSlashHorizontalEnds
};

// menuIconSize is the staff height a menu picture is drawn at, in pixels.
// Larger than the words beside it: a notehead set at text size is a dot.
const menuIconSize = 21;

// syncMenuIcons draws the mark beside every menu entry that writes one.
//
// Written in from the table rather than into the markup, for the reason the
// keys are: a hundred and sixty entries kept in step by hand are a hundred
// and sixty chances to put a sharp beside the flat.
//
// Every entry gets the element, including the ones with no icon. It is empty
// for those, and it holds the column open so the words line up — a menu
// where half the labels are indented and half are not is harder to read than
// one with no pictures at all.
function syncMenuIcons() {
  for (const b of document.querySelectorAll("[data-action]")) {
    if (b.querySelector(".menu-icon")) continue;
    const i = document.createElement("i");
    i.className = "menu-icon";
    const ch = MENU_ICONS[b.dataset.action];
    if (ch) {
      // Two characters is a note with a mark on it. The note is drawn in
      // the colour of the words beside it and the mark in the colour kept
      // for what an entry writes.
      i.textContent = [...ch][0];
      const mark = [...ch].slice(1).join("");
      if (mark) i.dataset.mark = mark;
      else i.classList.add("is-mark");
      i.dataset.size = String(menuIconSize);
    } else {
      // Not music, so not the music font and not the marks' colour: a
      // drawing of our own, in the ink the words are written in.
      //
      // A command made at run time — one of the scripts in the plugins
      // folder — is named for the file it is in, which nobody can draw a
      // picture of in advance. Those are named kind:rest, so the kind in
      // front of the colon carries the drawing and every one of them gets
      // it.
      const kind = b.dataset.action.split(":")[0];
      const drawing = MENU_DRAWINGS[b.dataset.action] || MENU_DRAWINGS[kind];
      if (drawing) i.innerHTML = menuDrawing(drawing);
    }
    b.insertBefore(i, b.firstChild);
  }
  fitGlyphs(document);
}

function syncMenuKeys() {
  for (const b of KEY_BINDINGS) {
    if (!b.menu) continue;
    const el = document.querySelector(`[data-action="${b.menu}"] kbd`);
    if (!el) continue;
    // Both keys where there are two, the command's own first. A menu that
    // showed one of them would be telling half the truth about Save.
    let text = shortcutText(keyFor(b), true);
    if (b.also && !(b.id in (state.keys || {}))) {
      text += " " + shortcutText(b.also, true);
    }
    el.textContent = text;
  }
}

// shortcutsDialog lists every key, and lets one be changed.
//
// The list is built from the table rather than written out, so a binding
// added to the program is a binding that shows up here. Nothing to keep in
// step, which is the only reason this can be trusted to be complete.
// redrawShortcuts is the open window's own draw, so that Reset all can take
// effect in front of the person rather than after they close it.
let redrawShortcuts = null;

async function shortcutsDialog() {
  const dlg = $("#shortcuts");
  const body = $("#shortcuts-list");

  const draw = () => {
    const bound = boundKeys();
    body.innerHTML = "";
    const groups = [];
    for (const b of KEY_BINDINGS) {
      let g = groups.find((x) => x.name === b.group);
      if (!g) groups.push(g = { name: b.group, rows: [] });
      g.rows.push(b);
    }
    for (const g of groups) {
      const h = document.createElement("h3");
      h.textContent = t(g.name);
      body.appendChild(h);
      for (const b of g.rows) {
        const chord = keyFor(b);
        const row = document.createElement("div");
        row.className = "shortcut";

        const key = document.createElement("kbd");
        key.textContent = shortcutText(chord);
        if (b.also && !(b.id in (state.keys || {}))) {
          key.textContent += "  " + shortcutText(b.also);
        }
        // A binding another one has taken over is still listed, because it
        // is still in the program; saying so is better than showing a key
        // that does nothing.
        if (bound.get(chord) !== b) {
          row.classList.add("shadowed");
          key.title = t("Taken by") + " " + t(bound.get(chord).label);
        }
        row.appendChild(key);

        const what = document.createElement("span");
        what.textContent = t(b.label);
        row.appendChild(what);

        const change = document.createElement("button");
        change.type = "button";
        change.className = "linklike";
        change.textContent = (state.keys || {})[b.id]
          ? t("Change") + " \u00b7 " + t("reset") : t("Change");
        change.addEventListener("click", () => {
          if ((state.keys || {})[b.id] && change.textContent.endsWith("reset")) {
            // Two things on one button is one thing too many, so the second
            // click is what it offers once there is something to reset.
            delete state.keys[b.id];
            void saveKeys();
            draw();
            return;
          }
          key.textContent = "Press a key\u2026";
          row.classList.add("catching");
          state.catchKey = (got) => {
            row.classList.remove("catching");
            if (got) {
              state.keys = state.keys || {};
              state.keys[b.id] = got;
              void saveKeys();
            }
            draw();
          };
        });
        row.appendChild(change);
        body.appendChild(row);
      }
    }
  };
  redrawShortcuts = draw;
  draw();

  await new Promise((resolve) => {
    const done = () => {
      state.catchKey = null;
      redrawShortcuts = null;
      dlg.removeEventListener("close", done);
      resolve();
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
}

// saveKeys keeps the changed bindings, and only the changed ones.
async function saveKeys() {
  syncMenuKeys();
  try {
    await fetch("/api/settings?keys=" +
                encodeURIComponent(JSON.stringify(state.keys || {})),
                { method: "POST" });
  } catch (_) {
    // Served without a disk behind it: the bindings hold for this session.
  }
}

// resetKeys puts every binding back to the key it came with.
async function resetKeys() {
  state.keys = {};
  await saveKeys();
  status("Every key back to its default");
}

// chordOf is the keystroke, written the way the table writes it.
function chordOf(e) {
  const parts = [];
  if (e.ctrlKey || e.metaKey) parts.push("Mod");
  if (e.altKey) parts.push("Alt");
  let key = e.key === " " ? "Space" : e.key;
  // Shift is in the character for anything a keyboard prints, so naming it
  // as well would make the tie "Shift+T" and match nothing.
  if (e.shiftKey && key.length > 1) parts.push("Shift");
  parts.push(key);
  return parts.join("+");
}

// keyFor is the keystroke a command is on, the one that was asked for or the
// one it came with.
function keyFor(b) {
  const mine = (state.keys || {})[b.id];
  return mine || b.keys;
}

// boundKeys is what each keystroke does, built fresh so that changing one
// takes effect without reloading.
//
// Later wins, so a keystroke somebody has asked for beats a default on the
// same keys — otherwise taking Mod+c for something else would leave Copy
// still answering to it and the new binding dead.
function boundKeys() {
  const out = new Map();
  // The second key a command answers to as well as its own, where it has
  // one. Laid down first, so a binding that holds that chord in its own
  // right — its default or somebody's own — beats it.
  for (const b of KEY_BINDINGS) {
    if (b.also) out.set(b.also, b);
  }
  for (const b of KEY_BINDINGS) {
    if (!(b.id in (state.keys || {}))) out.set(b.keys, b);
  }
  for (const b of KEY_BINDINGS) {
    const mine = (state.keys || {})[b.id];
    if (mine) out.set(mine, b);
  }
  return out;
}

// bindingFor is what a keystroke does, or nothing where it does nothing.
//
// The chord as it was pressed, and then the character alone where the
// modifiers were how that character was typed at all. On a good many
// layouts # or _ is Option or AltGr with a digit, so a binding written
// against the character would be unreachable for the people whose keyboards
// make it that way — which is most of them outside the US.
//
// Only where nothing claims the chord, so a real Alt binding still wins, and
// only for punctuation: Alt with a letter, a digit or a named key is a chord
// and is left alone.
function bindingFor(e) {
  const keys = boundKeys();
  const bound = keys.get(chordOf(e));
  if (bound) return bound;
  if (e.altKey && !e.ctrlKey && !e.metaKey &&
      e.key.length === 1 && !/[\p{L}\p{N}]/u.test(e.key)) {
    return keys.get(e.key) || null;
  }
  return null;
}

function setupKeys() {
  document.addEventListener("keydown", (e) => {
    if (e.target.matches("input, textarea")) return;

    // Catching a keystroke for the shortcuts window, which is the one thing
    // that wants the key itself rather than what it does.
    //
    // Before the dialog guard below, and not after it: the window doing the
    // asking is a dialog, so a check for an open one turns every key away
    // and nothing could ever be caught.
    if (state.catchKey) {
      e.preventDefault();
      // The modifiers on their own are not a keystroke; they are what is
      // being held while one is pressed.
      if (["Shift", "Control", "Alt", "Meta"].includes(e.key)) return;
      const catcher = state.catchKey;
      state.catchKey = null;
      catcher(e.key === "Escape" ? null : chordOf(e));
      return;
    }

    // A dialog has its own keys: enter means its OK button and space means
    // whatever has focus. Writing a note behind an open dialog would be a
    // keystroke landing in two places at once.
    if (document.querySelector("dialog[open]")) return;

    // On a drum staff the letters type the kit. A groove is typed, not
    // clicked thirty-two times, and each pad shows the letter that writes it.
    // The rule is the cursor: while it is on a drum staff those letters mean
    // drums, and everywhere else they mean what they mean everywhere else.
    // Scoping it to the staff is what lets both sets of letters be the easy
    // ones to reach.
    if (!e.shiftKey && !e.altKey && !e.ctrlKey && !e.metaKey && state.selection &&
        isDrumStaff(state.selection.part) && drumPadForKey(e.key)) {
      e.preventDefault();
      void typeDrum(e.key);
      return;
    }

    // A symbol that has been picked up takes the arrow keys, so that a
    // mark put roughly in place with the mouse can be settled with the
    // keys. Escape lets it go and gives the keys back.
    if (pickedSymbol && !e.ctrlKey && !e.metaKey) {
      const step = e.shiftKey ? 1 : 0.25;
      const by = {
        ArrowLeft: [-step, 0], ArrowRight: [step, 0],
        ArrowUp: [0, -step], ArrowDown: [0, step],
      }[e.key];
      if (by) {
        e.preventDefault();
        void nudgeSymbol(by[0], by[1]);
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        dropSymbol();
        return;
      }
    }

    // And on a tab staff the digits type frets, by the same rule and for
    // the same reason: a tab is written by saying which fret, and the
    // number is what the player is reading. Scoped to the staff, so the
    // digits go on meaning what they mean everywhere else.
    if (!e.shiftKey && !e.altKey && !e.ctrlKey && !e.metaKey && state.selection &&
        isTabStaff(state.selection.part) && /^[0-9]$/.test(e.key)) {
      e.preventDefault();
      void typeFret(Number(e.key));
      return;
    }

    const bound = bindingFor(e);
    if (!bound) return;
    // Inputs and open dialogs are already out, so there is no default worth
    // keeping: Tab would move the focus off the music and Backspace would
    // ask the browser to go back a page.
    e.preventDefault();
    bound.run();
  });
}

/* ---------- moving, copying and pasting ---------- */

// nudge moves what is selected up or down the staff.
//
// Steps, not semitones: a step up from F in G major is G, and from F sharp it
// is still G. The arrow keys are for moving a note to the right place on the
// staff, which is what a player reads.
async function nudge(steps) {
  if (!pickedNotes()) {
    status("Click a note first");
    return;
  }
  // The notehead that was clicked, where one was: clicking a note of a
  // chord and pressing Up means that note, not the chord. Without it the
  // whole chord went up a step and the one note somebody was correcting
  // came back wrong along with the two that were already right.
  //
  // Nothing on a single note, which has one notehead and moves whichever
  // way it is asked, and nothing on a dragged run, where what is picked
  // up is whole chords.
  const head = pickedHead();
  const d = await mark("nudge", { steps: String(steps), ...head },
             steps > 0 ? (steps === 7 ? "Up an octave" : "Up")
                       : (steps === -7 ? "Down an octave" : "Down"));
  // A notehead is named by where it sits, so moving it moves its name
  // with it. Left alone, a second press would be aimed at the line the
  // note has just left — which holds nothing, or worse, holds another
  // note of the same chord.
  if (d && head.head !== undefined && state.selection) {
    state.selection.head = Number(head.head) + steps;
    markSelection();
  }
}

// moveCursorBy steps the selection along the music, one element at a time,
// over barlines and on to the next measure.
// moveCursorPitch moves the cursor up or down the staff, which is where the
// next note will be written.
//
// Steps, not semitones: the cursor is a place on the staff and a step up from
// F is G whether or not either is sharpened. Seven of them is an octave,
// which is why shift with the arrow is an octave and not twelve semitones of
// something that does not land on a line.
function moveCursorPitch(steps) {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  // Four ledger lines either way is as far as anybody writes without
  // changing clef, and a cursor that can wander off the page is a cursor you
  // have to hunt for.
  const step = Math.max(-10, Math.min(18, (sel.step ?? 0) + steps));
  if (step === sel.step) return;
  sel.step = step;
  // Moving the cursor is not selecting a note: what it is pointing at is a
  // place to write, so nothing is picked up and a mark typed next would have
  // nothing to land on.
  sel.ref = null;
  state.multi = [];
  markSelection();
  scrollSelectionIntoView();
  status(pitchAtCursor());
}

// pitchAtCursor names the note the cursor is sitting on, so that moving it is
// something you can do without looking.
function pitchAtCursor() {
  const sel = state.selection;
  if (!sel) return "";
  const p = state.info?.parts?.[sel.part];
  const names = ["C", "D", "E", "F", "G", "A", "B"];
  // Where the bottom line is, by clef, counted in diatonic steps from middle
  // C: E4 for the treble, G2 for the bass, F3 for the alto, D3 for the
  // tenor. A staff step is one diatonic degree — half a space — so the two
  // add straight up.
  const bottom = { treble: 2, bass: -10, alto: -4, tenor: -6 }[p?.clef] ?? 2;
  const idx = bottom + (sel.step ?? 0);
  const n = ((idx % 7) + 7) % 7;
  const octave = 4 + Math.floor(idx / 7);
  return names[n] + octave;
}

// chordAtCursor stacks another notehead on the note just written.
//
// A chord is built from the bottom up, a note at a time: write the lowest,
// move the cursor, add the next. That is how it is played and how it reads
// to type.
async function chordAtCursor() {
  const sel = state.selection;
  if (!sel) {
    status("Write a note first");
    return;
  }
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  if (!hit) {
    status("Write a note first");
    return;
  }
  // The element before the cursor is the one just written, because that is
  // where the cursor is left.
  const el = (hit.elements || []).find((x) => x.index === sel.at - 1);
  if (!el || el.kind === "rest") {
    status("There is no note to add to");
    return;
  }
  // Two noteheads cannot share a line: there is one place on the staff for
  // that note and something is already on it. The server refuses this too,
  // but saying which note it is here is the difference between knowing what
  // happened and being told no.
  if ((el.steps || []).includes(sel.step ?? 0)) {
    status(`There is already a ${pitchAtCursor()} in that chord — ` +
           `move the cursor to another line first`);
    return;
  }
  await addPitch(hit, el, sel.step ?? 0);
}

function moveCursorBy(delta) {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  const staff = state.hits
    .filter((h) => h.part === sel.part)
    .sort((a, b) => a.measure - b.measure);
  if (!staff.length) return;

  // Everything on this staff, end to end, so moving off a bar simply lands
  // on the first note of the next.
  const line = [];
  for (const h of staff) {
    const els = h.elements || [];
    // A bar with nothing in it is somewhere to be, not something to step
    // over. The editor draws one after the last bar of the music so there
    // is a place to start the next one, and walking from note to note
    // skipped it — so the end of what was written was the end of where the
    // arrows could go, and the only way to begin the next bar was to reach
    // for the mouse. The cursor goes to the front of it, which is where
    // the writing would start.
    if (!els.length) {
      line.push({ measure: h.measure, index: -1, ref: null });
      continue;
    }
    for (const el of els) {
      line.push({
        measure: h.measure, index: el.index, ref: el.ref,
        step: el.steps && el.steps.length ? el.steps[0] : undefined,
      });
    }
  }
  if (!line.length) return;

  let at = line.findIndex((p) => p.measure === sel.measure && p.ref === sel.ref);
  if (at < 0) {
    at = line.findIndex((p) => p.measure === sel.measure && p.index === sel.at - 1);
  }
  at = at < 0 ? (delta > 0 ? 0 : line.length - 1) : at + delta;
  at = Math.max(0, Math.min(line.length - 1, at));

  const p = line[at];
  state.multi = [];
  // An empty bar has nothing to anchor a selection to, so shift with an
  // arrow from there starts from the cursor rather than from a note that
  // is not there.
  state.anchor = p.ref === null
    ? null
    : { part: sel.part, measure: p.measure, index: p.index };
  state.focus = null;
  // The cursor takes the pitch of whatever it lands on, so that arrowing
  // onto a note and then pressing up writes the next one a step above it
  // rather than jumping back to the bottom line.
  state.selection = {
    part: sel.part, measure: p.measure, at: p.index + 1,
    step: p.step ?? sel.step ?? 0, ref: p.ref,
    // Moving along the music is going somewhere to write, so what is written
    // goes in at the cursor. Replacing is for what you pointed at.
    replace: false,
  };
  markSelection();
  scrollSelectionIntoView();
  status(p.ref === null
    ? `Measure ${barNumberOf(p.measure)} — nothing written here yet`
    : `Measure ${barNumberOf(p.measure)}, note ${p.index + 1}`);
}

// scrollSelectionIntoView keeps the cursor on screen as it walks the music.
/* ---------- going to a measure ---------- */

// jumpToMeasure asks which bar and takes the page there.
//
// A long piece is a long way to scroll. Hunting for bar 184 by dragging is
// the kind of thing a computer should be doing, and it is the first thing
// anybody wants after being told "from bar 184" across a rehearsal room.
/* ---------- movements ---------- */

// Bars are numbered from one in every movement, so a bar number means
// nothing without saying which movement it is in.
//
// Going to bar four of the Presto is what somebody standing in the Presto
// means by "go to bar four" — it is the number printed over the bar they
// are looking at. Counted from the first bar of the whole sonata it took
// them to the Adagio, two movements back.

// movementAt is the movement a bar belongs to, or null in a score that has
// only one — which is every ordinary score, and where none of this applies.
function movementAt(measure) {
  const all = state.info?.movements || [];
  if (all.length < 2) return null;
  for (const m of all) {
    if (measure >= m.first && measure <= m.last) return m;
  }
  return all[all.length - 1];
}

// movementHere is the movement the cursor is standing in.
function movementHere() {
  return movementAt(state.selection?.measure ?? 0);
}

// barNumberOf is the number printed over a bar: its place in its own
// movement.
function barNumberOf(measure) {
  const m = movementAt(measure);
  return measure - (m ? m.first : 0) + 1;
}

// barIndexOf turns a number somebody typed back into a bar of the score,
// within the movement they are standing in.
function barIndexOf(n, movement) {
  return (movement ? movement.first : 0) + n - 1;
}

async function jumpToMeasure() {
  const total = measureCount();
  if (!total) {
    status("There is nothing to go to yet");
    return;
  }
  // Within the movement the cursor is in, because that is what the numbers
  // over the bars say.
  const where = movementHere();
  const last = where ? where.last - where.first + 1 : total;
  const named = where && where.title ? ` of ${where.title}` : "";

  const answer = await ask({
    title: "Go to measure",
    label: `Measure number (1 to ${last})${named}`,
    value: String(barNumberOf(state.selection?.measure ?? 0)),
    note: where
      ? `${where.title || "This movement"} is numbered from its own first ` +
        `full bar, as it is on the page.`
      : "The score is numbered from the first full bar.",
  });
  if (answer === null) return;

  const n = Math.round(Number(String(answer).trim()));
  if (!Number.isFinite(n) || n < 1) {
    status("That is not a measure number");
    return;
  }
  if (n > last) {
    status(`There ${last === 1 ? "is" : "are"} only ${last} measure` +
           `${last === 1 ? "" : "s"}${named}`);
    return;
  }
  await goToMeasure(barIndexOf(n, where));
}

// measureCount is how many bars the part being written in has.
// measureCount is how many bars the piece has.
//
// The blank bar past the end does not count. It is drawn so there is
// somewhere to click to carry on writing, and it is not a bar until
// something is written in it — counted as one, the editor believes in a bar
// that is not there, writes into it, and the bar comes into being on that
// one staff while every other staff is left a bar short of it.
function measureCount() {
  let most = 0;
  for (const h of state.hits) {
    if (h.ahead) continue;
    if (h.measure + 1 > most) most = h.measure + 1;
  }
  return most;
}

// blankBarPastTheEnd says a bar is the blank one drawn after the music.
//
// The engraver says so of the region it drew, which is the thing to ask:
// the bar has a number, a clickable region and a cursor that lands in it
// like any other, and the only thing that distinguishes it is that it is
// not there. Whatever is to be written *on* a bar rather than in it — a
// rehearsal letter over the bar line — has nothing to be written on.
//
// With no drawing to go by, nothing is past the end: it is better to ask
// the server and be told no than to refuse a bar that is really there.
function blankBarPastTheEnd(measure) {
  const hits = state.hits || [];
  const here = hits.find((h) => h.measure === measure);
  if (here) return !!here.ahead;
  return hits.length > 0 && measure >= measureCount();
}

// goToMeasure moves the page to a bar, selects it and says where it landed.
//
// It takes a bar of the score, not a number somebody typed: everything else
// the editor holds — the clickable regions, the performance, the cursor —
// counts bars from the beginning of the file, and only what is written on
// the page and typed into the box is counted from the movement.
async function goToMeasure(index, part) {
  const box = measureBox(index);
  if (!box) {
    status(`Measure ${barNumberOf(index)} is not on the page`);
    return;
  }
  selectMeasure(part ?? state.selection?.part ?? 0, index);
  await glideTo(box);
  // The view has moved a long way, so the pieces under it are different
  // ones.
  await ensureTiles();
  flashMeasure(box);
  const where = movementAt(index);
  status(`Measure ${barNumberOf(index)}` +
         (where && where.title ? ` — ${where.title}` : ""));
}

// glideTo scrolls the view to a box, over time rather than in one jump.
//
// A page that simply appears somewhere else leaves you looking for the bar
// you asked for: nothing on the screen connects where you were to where you
// are. Moving there, even quickly, carries the eye along with it.
//
// The scroll is driven here rather than handed to the browser's own smooth
// behaviour because that is a single speed for any distance — a crawl across
// a long score, a lurch across a short one. This takes longer for a longer
// journey, but not proportionally, and never longer than about half a
// second.
function glideTo(box) {
  const wrap = $("#canvas-wrap");
  if (!wrap) return Promise.resolve();

  const z = state.zoom;
  const off = tuneOffsetY();
  // A third in from the left, so there is music either side of the bar: the
  // bar before it is as much a part of "where am I" as the bar itself.
  const wantX = Math.max(0, box.x0 * z - wrap.clientWidth * 0.33);
  const wantY = Math.max(0, (box.top * z + off) - wrap.clientHeight * 0.3);
  const maxX = Math.max(0, wrap.scrollWidth - wrap.clientWidth);
  const maxY = Math.max(0, wrap.scrollHeight - wrap.clientHeight);
  const toX = Math.min(wantX, maxX);
  const toY = Math.min(wantY, maxY);

  const fromX = wrap.scrollLeft;
  const fromY = wrap.scrollTop;
  const far = Math.hypot(toX - fromX, toY - fromY);
  if (far < 2) return Promise.resolve();

  // Somebody who has asked for less movement gets none of this.
  const still = window.matchMedia &&
                window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (still) {
    wrap.scrollLeft = toX;
    wrap.scrollTop = toY;
    return Promise.resolve();
  }

  // Long enough to follow, short enough not to be waited on.
  const ms = Math.min(520, 220 + far * 0.12);
  const started = performance.now();
  return new Promise((done) => {
    const step = (now) => {
      const t = Math.min(1, (now - started) / ms);
      // Ease in and out: it sets off and arrives gently, which is what makes
      // it read as the page moving rather than as the page being replaced.
      const e = t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
      wrap.scrollLeft = fromX + (toX - fromX) * e;
      wrap.scrollTop = fromY + (toY - fromY) * e;
      if (t < 1) {
        requestAnimationFrame(step);
        return;
      }
      done();
    };
    requestAnimationFrame(step);
  });
}

// flashMeasure lights the bar that was arrived at, and lets it fade.
//
// The selection marks it too, but a selection is a quiet thing that stays;
// this says "here" once, at the moment the movement stops, and takes itself
// away. The fading is the stylesheet's — it ends with the animation, so
// there is nothing to clean up if the view moves again first.
function flashMeasure(box) {
  const svg = currentSVG();
  if (!svg) return;
  svg.querySelectorAll("rect.goto-flash").forEach((r) => r.remove());

  const z = state.zoom;
  const r = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  r.setAttribute("class", "goto-flash");
  r.setAttribute("x", String((box.x0 - 0.5) * z));
  r.setAttribute("y", String((box.top - 1.5) * z));
  r.setAttribute("width", String((box.x1 - box.x0 + 1) * z));
  r.setAttribute("height", String((box.bottom - box.top + 3) * z));
  r.setAttribute("rx", String(z * 0.4));
  svg.appendChild(r);
  r.addEventListener("animationend", () => r.remove());
}

// cursorMargin is how near the edge of the window the caret may come before
// the page moves, in staff spaces. A note's width is about one and a half of
// them, so this is a bar or so of warning: writing against the edge of the
// glass, with the next note off it, is what this is here to prevent.
const cursorMargin = 6;

function scrollSelectionIntoView() {
  const svg = currentSVG();
  const wrap = $("#canvas-wrap");
  if (!svg || !wrap) return;
  // The caret first, and the notehead only if there is none.
  //
  // What was picked up and where the next note goes are two different
  // places, and typing moves the second one. Following the first meant the
  // page stood still while the caret walked off the right of it: the note
  // being written was the one note that could not be seen, which is exactly
  // backwards. With nothing picked up there was no scrolling at all, and
  // writing a bar of notes picks nothing up.
  const target = svg.querySelector(".write-block") ||
    (state.selection?.ref
      ? svg.querySelector(`[data-ref="${state.selection.ref}"]`)
      : null);
  if (!target || !target.getBoundingClientRect) return;

  const box = target.getBoundingClientRect();
  const view = wrap.getBoundingClientRect();
  const margin = cursorMargin * state.zoom;

  // Across: the page moves only when the caret comes near an edge, and then
  // it moves far enough to leave most of a screen of room in the direction
  // the writing is going. Moving by just enough to bring the caret back
  // inside would leave it against that edge, and then every note after it
  // would move the page again.
  const x = box.left - view.left + wrap.scrollLeft;
  const furthest = Math.max(0, wrap.scrollWidth - wrap.clientWidth);
  let left = wrap.scrollLeft;
  if (box.right > view.right - margin) {
    left = x - wrap.clientWidth * cursorLead;
  } else if (box.left < view.left + margin) {
    // Going back the other way, the room goes behind it instead.
    left = x - wrap.clientWidth * (1 - cursorLead);
  }
  left = Math.min(furthest, Math.max(0, left));

  // Down the page the music does not move continuously — it steps from one
  // system to the next — so this moves only when the caret has left the one
  // in view.
  const top = box.top - view.top + wrap.scrollTop;
  const bottom = box.bottom - view.top + wrap.scrollTop;
  let up = wrap.scrollTop;
  if (top < wrap.scrollTop || bottom > wrap.scrollTop + wrap.clientHeight) {
    up = Math.max(0, top - wrap.clientHeight * 0.3);
  }

  if (left === wrap.scrollLeft && up === wrap.scrollTop) return;
  if (wrap.scrollTo) wrap.scrollTo({ left, top: up, behavior: "smooth" });
}

async function copySelection() {
  const sel = selectionTargets() || measureTargets();
  if (!sel) {
    status("Select something to copy");
    return;
  }
  const res = await fetch("/api/copy?sel=" + encodeURIComponent(sel), { method: "POST" });
  if (!res.ok) {
    status("Could not copy: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  status(`Copied ${d.copied} ${d.copied === 1 ? "note" : "notes"}`);
}

// measureTargets is the bar the cursor stands in, for a copy that was asked
// for with nothing picked up.
//
// Being in a bar is being somewhere, and "copy" said about a place with no
// note picked up has one sensible meaning: this bar. It used to answer
// "Select something to copy" to somebody who could see perfectly well where
// they were — and the way to pick a whole bar up, the band over it, is not
// something anybody finds without being told.
//
// Only copy falls back to this. Cut does not: emptying a bar on a keystroke
// meant for one note is not a mistake anybody would forgive, and Backspace
// at a cursor already means the note before it, not the bar around it.
function measureTargets() {
  const sel = state.selection;
  if (!sel || state.multi.length) return null;
  if (sel.ref !== null && sel.ref !== undefined) return null;
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure);
  const els = hit?.elements || [];
  if (!els.length) return null;
  return els.map((el) => `${sel.part}:${sel.measure}:${el.index}`).join(",");
}

async function cutSelection() {
  const sel = selectionTargets();
  if (!sel) {
    status("Select something to cut");
    return;
  }
  const res = await fetch("/api/copy?sel=" + encodeURIComponent(sel), { method: "POST" });
  if (!res.ok) {
    status("Could not cut: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  await deleteSelected();
  status(`Cut ${d.copied} ${d.copied === 1 ? "note" : "notes"}`);
}

async function pasteClipboard() {
  const sel = state.selection;
  if (!sel) {
    status("Click where it should go");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at),
  });
  // Over a bar, the copy takes the bar's place rather than being pushed into
  // the front of it: pasting four bars of a part into another should give
  // four bars of it, not eight bars of everything shoved along.
  if (sel.bar) p.set("at", "0");
  const res = await fetch("/api/paste?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not paste: " + (await res.text()).trim());
    return;
  }
  const d = await res.json();
  if (d.measure !== undefined) {
    sel.measure = d.measure;
    sel.at = d.at;
  }
  sel.ref = null;
  state.multi = [];
  await refreshScore();
  markSelection();
  status(`Pasted ${d.pasted} ${d.pasted === 1 ? "note" : "notes"}`);
}

// selectionTargets spells what is selected the way the server wants it, or
// returns null when nothing is.
function selectionTargets() {
  if (state.multi.length) {
    return orderedSelection().map((m) => `${m.part}:${m.measure}:${m.index}`).join(",");
  }
  const sel = state.selection;
  if (!sel) return null;
  if (sel.bar) {
    // A whole bar: everything in it, in the order it is played.
    const hit = state.hits.find(
      (h) => h.part === sel.part && h.measure === sel.measure);
    const els = hit?.elements || [];
    if (!els.length) return null;
    return els.map((el) => `${sel.part}:${sel.measure}:${el.index}`).join(",");
  }
  if (sel.ref === null || sel.ref === undefined) return null;
  return `${sel.part}:${sel.measure}:${sel.at - 1}`;
}

/* ---------- player mode ---------- */

// Reading the settings in the page's address.
//
// gscore is an editor, but a page that is about a piece of music does not want
// an editor on it: it wants the music, and a way to hear it. Player mode is
// that — the same program with everything that edits taken away, which is
// better than a second, lesser program that only plays, because there is only
// one engraver and one synthesiser to get right.
//
//   ?player=1      no menus, no palette, no inspector: the score and a play button
//   ?player=bare   not even that: only the score, which plays when clicked
//   ?score=URL     the file to show, fetched and opened
//   ?autoplay=1    start playing as soon as it can
//   ?tune=N        which tune of a book to show
//   ?zoom=N        staff space in pixels
//   ?loop=1        start again at the end
//
// Autoplay is asked for, not taken: a browser will not make a sound until
// somebody has clicked on the page, so the first click plays it.
function readOptions() {
  const q = new URLSearchParams(location.search);
  const opt = {
    player: q.get("player"),
    score: q.get("score"),
    autoplay: q.get("autoplay") === "1",
    loop: q.get("loop") === "1",
    // "strip" draws the music as one unbroken line and "page" breaks it
    // into pages. Both have always been in the editor, under View; this
    // lets a page that embeds the editor say which it wants. A score shown
    // in a band across a web page wants the line, not a paginated score
    // squeezed into a frame.
    mode: q.get("mode"),
    tune: q.has("tune") ? Number(q.get("tune")) : null,
    zoom: q.has("zoom") ? Number(q.get("zoom")) : null,
  };
  opt.isPlayer = opt.player === "1" || opt.player === "bare" || opt.player === "true";
  opt.bare = opt.player === "bare";
  return opt;
}

// applyPlayerMode strips the editor down to a player.
async function applyPlayerMode(opt) {
  document.body.classList.add("player");
  if (opt.bare) document.body.classList.add("bare");
  state.player = true;

  // Wrapped to the frame unless the page asked for the strip. The strip
  // is the editor's view and the right one for a band across a page; for
  // a frame it shows the first two bars and a scrollbar.
  if (!opt.mode) state.mode = "page";

  // A frame can be resized — by the page, by the reader turning a phone
  // — and the music has to be re-wrapped when it is. Settled a moment
  // after the last change rather than on every pixel of a drag.
  let settling = null;
  window.addEventListener("resize", () => {
    if (settling) clearTimeout(settling);
    settling = setTimeout(() => {
      settling = null;
      void refreshScore();
    }, 150);
  });

  // Nothing writes any more, so a click on the staff is not an edit.
  state.readOnly = true;

  if (opt.bare) {
    // With no transport to press, the score itself is the button.
    $("#canvas-wrap").addEventListener("click", () => {
      void togglePlay();
    });
    $("#canvas-wrap").style.cursor = "pointer";
  }
  if (opt.loop) state.loop = true;
}

// loadFromURL opens a score named in the address, so a page can point at one.
async function loadFromURL(url) {
  status("Loading…");
  const res = await fetch(url);
  if (!res.ok) {
    status("Could not fetch " + url);
    return false;
  }
  const blob = await res.blob();
  const name = url.split("/").pop() || "score";
  const ok = await openFile(new File([blob], name));
  return ok !== false;
}

async function main() {
  // The palette and the menus are built first: they need nothing from the
  // core and showing them at once makes the window look alive.
  buildPalette(paletteAction);
  markTools();
  setupMenus();
  setupFileInput();
  setupKeys();
  setupZoomGestures();
  setupRangeClicks();
  setupKeyMapClicks();
  setupScoreMapClicks();
  setupVoiceLeadingClicks();
  setupSpacingClicks();
  setupRhythmClicks();
  setupMelodyClicks();
  setupTypeBar();
  setupConsole();
  setupScriptResult();
  setupPartDialog();
  setupScoreSetupDialog();
  setupPanning();
  setupMarquee();
  setupSymbolDragging();
  setupFollowRelease();
  setupFullscreen();
  setupCanvasClicks();
  setupStripHover();
  setupVirtualDraw();
  setupDrawing();
  setupPDFReview();
  markMode();

  $("#shortcuts-reset").addEventListener("click", async () => {
    await resetKeys();
    if (redrawShortcuts) redrawShortcuts();
  });
  $("#tune-select").addEventListener("change", (e) => selectTune(Number(e.target.value)));
  $("#show-all").addEventListener("change", async (e) => {
    state.showAll = e.target.checked;
    await drawScore();
  });
  $("#zoom").addEventListener("input", (e) => setZoom(Number(e.target.value)));
  $("#drumpad-kit").addEventListener("change", (e) => void changeDrumKit(e.target.value));
  $("#play-btn").addEventListener("click", () => void togglePlay());
  $("#stop-btn").addEventListener("click", () => stop());

  const opt = readOptions();
  if (opt.mode === "strip" || opt.mode === "page") {
    // Before anything is drawn, so the first drawing is the right one.
    state.mode = opt.mode;
  }
  if (opt.isPlayer) await applyPlayerMode(opt);
  if (opt.zoom) setZoom(opt.zoom);

  // As a page, the core is WebAssembly and is still starting.
  if (window.gscoreCoreReady) {
    status("Starting…");
    await window.gscoreCoreReady;
  }

  // What was remembered from last time.
  //
  // After the core is up, not before. Asked earlier, this request found no
  // core to answer it and went out to the network instead, where the host
  // that serves the page has no /api/settings and says 404 — so on the web
  // nothing was ever remembered between visits, and the only sign of it was
  // a line in the browser's console.
  let kept = null;
  try {
    kept = await (await fetch("/api/settings")).json();
    state.trackBarlines = kept.trackBarlines !== false;
    state.keys = kept.keys || {};
    // So the tick in the View menu tells the truth from the first look:
    // the colours are remembered for the person, so a score opened today
    // comes up coloured if that is how it was left.
    state.colourNotes = !!kept.colourNotes;
    // How far behind the sound is, where the browser will not say and
    // somebody has measured it. See outputLag.
    state.playLag = Number(kept.playLag) || 0;
    // Whether a note is sounded as it is written. On unless it has been
    // turned off: see soundAsWritten.
    state.entrySound = kept.entrySound !== false;
    // Before anything is drawn, so the interface does not come up at one
    // size and jump to another.
    applyUIScale(rememberedUIScale(kept));
    showZoom(startingZoom());
  } catch (_) {
    // Served without a core behind it. What the browser remembers still
    // holds: the sizes are the browser's to know.
    applyUIScale(rememberedUIScale(null));
    showZoom(startingZoom());
  }
  // Before the menus are filled in and the keys are written into them: a
  // menu translated after its keystrokes were put there would have to have
  // them put there twice.
  await startLanguage(kept);
  markBarlineTick();
  // After the language, since the sizes are named in it.
  fillUIScaleMenu();
  loadMouseWrites();
  loadMetronome();
  syncMenuKeys();
  syncMenuIcons();

  if (opt.score) {
    await loadFromURL(opt.score);
  } else {
    await reopen();
  }
  if (opt.tune !== null && !Number.isNaN(opt.tune)) {
    await selectTune(opt.tune);
  }

  const n = (state.info?.tunes || []).length;
  status(n > 1 ? `${n} tunes in this file` : "Ready");

  // Instruments kept from a previous visit are loaded now, so that the first
  // time play is pressed there is nothing to wait for. Not awaited: it takes
  // a second to parse thirty megabytes of samples and the score is already
  // on screen and editable.
  if (state.info?.inBrowser) {
    void loadStoredInstruments().then((ok) => {
      if (ok) status("Playing through " + pageFont.name);
    });
  }

  if (opt.autoplay) {
    // Browsers refuse to make a sound before the page has been clicked, so
    // this tries and, if it is refused, arms the next click to play.
    try {
      await play();
    } catch (_) {
      const once = () => {
        document.removeEventListener("click", once);
        void play();
      };
      document.addEventListener("click", once);
    }
  }

  // A page that embeds gscore can drive it, and wants to know when it may.
  window.gscore = {
    play, pause, stop, loadFromURL,
    selectTune,
    isPlaying: () => state.playing,
    setZoom,
  };

  // The macOS application drives the editor from its menu bar. Each item
  // carries the name of a command — the same name the toolbar puts in a
  // data-action attribute — and sends it here, so the menu bar and the
  // interface reach one dispatcher and cannot drift apart: there is one list
  // of things gscore can do and two ways of reaching it.
  // Inside the Mac application there is a menu bar at the top of the screen
  // with these same commands on it, so the page's own goes away: two menu
  // bars is the same list twice, and one of them in the place a Mac does not
  // put menus.
  if (window.gscoreNativeMenu) {
    document.documentElement.classList.add("native-menu");
  }

  // The menu bar a Mac puts at the top of the screen is built from these
  // same menus, rather than being a second list in another language that
  // drifts from this one. Everything here is what the page shows; the
  // application reads it once and makes its own bar out of it.
  window.gscoreMenuTree = () => {
    const read = (root) =>
      Array.from(root.children).flatMap((node) => {
        if (node.tagName === "HR") return [{ kind: "separator" }];
        if (node.classList.contains("submenu")) {
          const title = node.querySelector(".submenu-title span");
          const items = node.querySelector(".menu-items");
          return [{
            kind: "submenu",
            label: (title?.textContent || "").trim(),
            items: items ? read(items) : [],
          }];
        }
        // Hidden here, hidden in the native menu bar: one list of what can
        // be done, not two that disagree.
        if (node.hidden) return [];
        if (node.tagName !== "BUTTON" || !node.dataset.action) return [];
        return [{
          kind: "item",
          label: (node.querySelector("span")?.textContent || "").trim(),
          action: node.dataset.action,
          enabled: !node.disabled,
          // The picture beside the words, as the characters it is made of
          // rather than as a drawing of them. A Mac draws its own menus and
          // wants to draw these too — in the colours the system is using at
          // the moment, at the size its own text is at — so it is given the
          // music font's codepoints and left to it. One table of what each
          // command looks like, two places that draw from it.
          icon: MENU_ICONS[node.dataset.action] || "",
        }];
      });

    return Array.from(document.querySelectorAll("#menubar .menu")).map((m) => ({
      label: (m.querySelector(".menu-title")?.textContent || "").trim(),
      items: read(m.querySelector(".menu-items")),
    }));
  };

  window.gscoreCommand = (name) => {
    // The shell has opened a file into the server underneath us, so the page
    // goes and looks again. A reload would do it too and would lose the
    // zoom, the selection and where you were on the page.
    if (name === "reopen-file") {
      void reopen();
      return;
    }
    void action(name);
  };
  window.dispatchEvent(new CustomEvent("gscore:ready", { detail: window.gscore }));
  setupEmbedMessages();
  announceToParent();
}

// Being driven from the page that embeds it.
//
// An iframe is a different document on a different origin, so a page cannot
// simply call into it: the only way across is a message. Every message gscore
// answers carries a `gscore` key, so nothing else on the page can be mistaken
// for one, and anything it does not recognise is ignored rather than guessed
// at.
//
//   { gscore: "play" }                  start, from the beginning
//   { gscore: "stop" }
//   { gscore: "toggle" }                 play, or pause where it is
//   { gscore: "tune",  value: 3 }       show tune 3 of a book
//   { gscore: "load",  value: "x.abc" } open another score
//   { gscore: "open",  value: "X:1\n…", name: "tune.abc" }
//                                       one written into the page itself
//   { gscore: "zoom",  value: 10 }
//   { gscore: "mode",  value: "strip" } one unbroken line, or "page" to
//                                       wrap it to the frame again
//
// It sends two of its own back to whoever embedded it: "ready" once the score
// is up, and "ended" when the music stops.
function setupEmbedMessages() {
  window.addEventListener("message", async (e) => {
    const m = e.data;
    if (!m || typeof m !== "object" || typeof m.gscore !== "string") return;
    switch (m.gscore) {
      case "play": await playFromStart(); break;
      case "stop": stop(); break;
      case "toggle": await togglePlay(); break;
      case "tune": await selectTune(Number(m.value) || 0); break;
      case "load":
        await loadFromURL(String(m.value));
        tellParent("ready");
        break;
      // A score written into the page rather than kept in a file.
      //
      // Whatever arrives is handed to the reader the name belongs to,
      // which is how a file dropped on the window is read too — so every
      // format gscore knows works here, and one it learns later will too.
      // Text for ABC and MusicXML, bytes for the zipped ones: a
      // MuseScore file cannot be written into a page as itself, so a
      // page sends it decoded. Both are what File takes.
      case "open": {
        const what = typeof m.value === "string"
          ? String(m.value)
          : new Uint8Array(m.value);
        await openFile(new File([what], String(m.name || "tune.abc")));
        // What it turned out to be, for the page to write under it.
        tellParent("ready");
        break;
      }
      case "zoom": setZoom(Number(m.value) || state.zoom); break;
      // Wrapped to the frame, or one unbroken line.
      //
      // The same switch as the View menu's, reachable from the page, so
      // that a page offering its reader the choice can make it without
      // reloading the frame and losing where the music had got to.
      case "mode": {
        const want = String(m.value) === "strip" ? "strip" : "page";
        if (want === state.mode) break;
        state.mode = want;
        markMode();
        await drawScore();
        await loadHits();
        tellParent("ready"); // the music is a different shape now
        break;
      }
    }
  });
}

// Leaving a browser tab with work in hand.
//
// The browser writes the warning, not the page — every one of them refuses a
// custom message now — and it only shows one at all if the page objects
// during the event itself. So the answer has to be already in hand, which it
// is: the server sets it on every edit and clears it on every save, and the
// interface reloads what it knows after each one.
//
// This is the browser's half. The native window cannot be asked this way at
// all, because its close button belongs to the operating system; gscore asks
// there after the window has gone. See OfferToSaveOnExit.
window.addEventListener("beforeunload", (e) => {
  if (state.readOnly || !state.info?.unsaved) return;
  e.preventDefault();
  e.returnValue = ""; // what older browsers look at
});

// announceToParent tells the embedding page that the score is up and that it
// may start sending messages.
function announceToParent() {
  if (window.parent === window) return;
  tellParent("ready");
  window.addEventListener("gscore:ended", () => tellParent("ended"));
}

// oneSystem is how tall one system of this score is, in pixels: every
// instrument of it, once, with the room its marks take above and below.
//
// It is what an embedded score should come up at. The whole music is the
// wrong answer — a tune of forty systems would open a frame forty
// systems tall — and a fixed number is the wrong answer too, because a
// song is one staff and a symphony is twenty-nine. A system is the unit
// the music is actually read in: a frame that tall shows every
// instrument playing at once, and a reader who wants less says so and
// scrolls.
//
// Measured from where the staves were drawn rather than counted from the
// parts: what a system costs is its staves plus whatever the slurs, the
// dynamics and the bar numbers needed, and only the engraver knows that.
function oneSystem() {
  const hits = state.hits || [];
  if (!hits.length) return 0;
  // The first system is every staff drawn at the first bar of it.
  let first = Infinity;
  for (const h of hits) first = Math.min(first, h.measure);
  let top = Infinity, bottom = -Infinity;
  for (const h of hits) {
    if (h.measure !== first) continue;
    top = Math.min(top, h.staffTop);
    bottom = Math.max(bottom, h.staffTop + h.staffHeight);
  }
  if (!isFinite(top) || !isFinite(bottom)) return 0;

  // From the top of the drawing to the foot of the system: the room
  // above the first staff is where its marks went, so measuring from
  // zero counts them, and a couple of spaces under the last staff keeps
  // the system off the edge. Adding the room above a second time, as
  // this did at first, opened a frame half again as tall as it needed
  // for a score with anything written over it.
  const music = (bottom + 2) * state.zoom;

  // And the chrome around the music: the transport, if there is one.
  const wrap = $("#canvas-wrap");
  const chrome = wrap
    ? Math.max(0, document.documentElement.clientHeight - wrap.clientHeight)
    : 0;
  return Math.round(music + chrome + 8);
}

// tellParent sends one of gscore's own messages out to the page that
// embedded it.
//
// "ready" is sent again whenever a score is opened, not only at startup.
// A page that writes the music into itself sends it after the frame is
// up, so the first "ready" is the empty page saying it is listening and
// the second carries what the piece turned out to be — which is how the
// page gets the title to write under the music. Nothing is lost on a
// page that only reads the first: the fields say the same things.
function tellParent(what) {
  if (window.parent === window) return;
  try {
    window.parent.postMessage({
      gscore: what,
      title: state.info?.title || "",
      tunes: (state.info?.tunes || []).length,
      // How tall the music turned out to be, so that a page sizing a
      // frame for it need not guess. A frame guessed at is a frame that
      // cuts the second system in half.
      //
      // Two numbers, because they answer different questions: system is
      // one system of every instrument, which is what a frame should
      // open at, and height is all of the music, which is what a page
      // that wants no scrollbar at all would need.
      system: oneSystem(),
      height: Math.ceil(document.documentElement.scrollHeight),
    }, "*");
  } catch (_) {
    /* a parent on another origin that refuses messages is not an error */
  }
}

main();

/* ---------- harmonics ---------- */

// What is sounding at one moment, and whether its partials agree.
//
// Not a picture of the loudspeaker. A spectrum taken off the audio shows the
// soundfont — its samples, its filters, its reverb — and gives a different
// answer for the same chord played on another instrument. The question a
// composer is asking when they ask whether a chord works is about the notes:
// every one of them carries a series of partials, and whether the chord is
// smooth or rough depends on whether those partials land on each other,
// stand clear, or land close enough to beat. All of that is arithmetic on
// the written pitches, so it answers before you press play and answers the
// same way every time.

// harmColours are the colours the notes are told apart by, which also tint
// the partials belonging to each. Chosen to stay distinct for the most
// common kinds of colour blindness, since the whole panel is read by colour.
const harmColours = ["#2f6fb3", "#d1702f", "#3f8f5a", "#9a4fa3", "#b03a4a", "#7a6a3a"];

// state.harm is what the panel is showing and whether it is open.
state.harm = { open: false, at: 0, following: false };

// harmAsked is when the panel last asked, so that playing does not ask sixty
// times a second for an answer that changes when the notes do.
let harmAsked = 0;

async function openHarmonics() {
  const dlg = $("#harmonics");
  if (!dlg) return;
  state.harm.open = true;
  if (!dlg.open) dlg.showModal();
  dlg.addEventListener("close", () => { state.harm.open = false; }, { once: true });
  await refreshHarmonics();
}

// refreshHarmonics asks the core what is sounding and draws the answer.
//
// The moment is named by the note that is picked up rather than by a time:
// the editor knows which notehead that is and nothing about the clock, and
// the core has the timeline. While the music is playing the time wins,
// because then the moment is wherever the cursor has got to.
async function refreshHarmonics() {
  if (!state.harm.open) return;
  const p = new URLSearchParams();
  if (state.harm.following) {
    p.set("at", String(state.harm.at));
  } else if (state.selection?.ref) {
    p.set("ref", String(state.selection.ref));
  } else if (state.selection) {
    p.set("measure", String(state.selection.measure));
  } else {
    p.set("at", "0");
  }

  let d;
  try {
    const res = await fetch("/api/harmonics?" + p.toString());
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // the panel is a reader, not an editor: a failed read is nothing
  }
  drawHarmonics(d);
}

// staffName is what a staff is called, by the number the analysis gives it.
function staffName(staves, part) {
  return (staves || [])[part] || `Staff ${(part || 0) + 1}`;
}

// harmStaves is every staff the sounding notes are on, in the order they
// are listed and without repeats.
function harmStaves(d) {
  const staves = d.staves || [];
  const out = [];
  for (const n of d.notes || []) {
    const name = staffName(staves, n.part);
    if (!out.includes(name)) out.push(name);
  }
  return out;
}

function drawHarmonics(d) {
  const words = $("#harm-words");
  const number = $("#harm-number");
  const fill = $("#harm-fill");
  if (words) words.textContent = d.verdict || "";
  if (number) number.textContent = (d.roughness || 0).toFixed(2);
  if (fill) {
    // The bar runs to one semitone's worth of roughness, which is where the
    // words stop being about a chord and start being about two notes
    // arguing. Past that it is full.
    fill.style.width = Math.min(100, (d.roughness || 0) * 100) + "%";
  }

  // The head says what is being read and when, and stays one line so that
  // the window does not move while the selection does.
  const staff = $("#harm-staff");
  if (staff) {
    const line = harmStaves(d).join(" · ");
    staff.textContent = line
      ? `${line} — at ${(d.at || 0).toFixed(2)}s`
      : `Nothing sounding at ${(d.at || 0).toFixed(2)}s`;
    // The whole of it, for a head too narrow to show the whole of it.
    staff.title = staff.textContent;
  }

  const where = $("#harm-where");
  if (where) {
    // Which staff each note is on, where the chord is spread over more than
    // one of them. On a single staff the head has already said so, and
    // repeating it beside every note would be noise.
    const staves = d.staves || [];
    const spread = harmStaves(d).length > 1;
    const names = (d.notes || []).map((n, i) => {
      const colour = harmColours[i % harmColours.length];
      const on = spread
        ? ` <span class="harm-on">(${escapeHTML(staffName(staves, n.part))})</span>`
        : "";
      return `<em style="color:${colour}">${escapeHTML(n.name)}</em>${on}`;
    });
    where.innerHTML = names.length ? names.join(" ") : "Nothing is sounding there.";
  }

  drawChord(d);
  drawSpectrum(d);
  drawIntervals(d);
}

// drawChord says what the chord is called and what it is doing.
//
// Two different claims. The name is a fact about the notes: A, C♯ and E with
// E at the bottom is A major in second inversion whatever else is going on.
// The numeral is a fact about the notes and the key together — in E that is
// IV, and in A it is the tonic — and it is the one that says what the chord
// is *for*, which is the question somebody trying to understand a piece is
// actually asking.
//
// The key is read from the phrase around the chord rather than from the key
// signature, so it follows the music where it goes. How clear that reading
// is comes back with it, and a reading that is not clear says so: a bare
// fifth is in a dozen keys at once and claiming one of them would be a
// guess told in the voice of a fact.
function drawChord(d) {
  const box = $("#harm-chord");
  if (!box) return;
  const c = d.chord;
  if (!c || !c.named) {
    box.innerHTML = "";
    return;
  }
  const clear = (d.keyFit?.margin ?? 0) >= 0.15;
  const where = d.keyName
    ? `<span class="harm-in-key">in ${escapeHTML(d.keyName)}` +
      `${clear ? "" : ", though the key is not settled here"}</span>`
    : "";
  const numeral = d.numeral && clear
    ? `<span class="harm-numeral">${escapeHTML(d.numeral)}</span>` : "";
  const what = d.function && clear ? ` · ${escapeHTML(d.function)}` : "";
  box.innerHTML = numeral +
    `<span class="harm-chord-name">${escapeHTML(c.name)}${what}</span> ${where}`;

  // And whether it will speak down there. The roughness bar below says how
  // rough the chord is; this is the orchestrator's version of the same
  // thing about the bottom of it, where a chord turns to mud first.
  const thin = $("#harm-thick");
  if (thin) {
    thin.textContent = d.thick ? d.thick.text : "";
    thin.hidden = !d.thick;
  }
}

// drawSpectrum draws every partial as a line on a log frequency axis.
//
// Logarithmic because that is how pitch works: an octave is a doubling
// wherever it falls, so on a log axis every octave is the same width and the
// partials of a note are evenly spread rather than crowded at the bottom.
// The lines are drawn tallest first so a quiet partial is never hidden
// behind a loud one at the same place — which is exactly where the
// interesting ones are.
function drawSpectrum(d) {
  const box = $("#harm-spectrum");
  if (!box) return;
  const partials = d.partials || [];
  if (!partials.length) {
    box.innerHTML = `<p class="small">Nothing is sounding there.</p>`;
    return;
  }

  const W = 560, H = 150, pad = 22;
  const lo = Math.log2(Math.min(...partials.map((p) => p.freq)) * 0.9);
  const hi = Math.log2(Math.max(...partials.map((p) => p.freq)) * 1.1);
  const x = (f) => pad + (Math.log2(f) - lo) / (hi - lo) * (W - pad * 2);
  const y = (a) => H - pad - a * (H - pad * 2);

  const parts = [];
  // The octave gridlines, so the eye has something to measure against.
  for (let f = 27.5; f < 20000; f *= 2) {
    if (Math.log2(f) < lo || Math.log2(f) > hi) continue;
    parts.push(`<line class="harm-grid" x1="${x(f).toFixed(1)}" y1="${pad - 6}" ` +
      `x2="${x(f).toFixed(1)}" y2="${H - pad}"/>`);
    parts.push(`<text class="harm-tick" x="${x(f).toFixed(1)}" y="${H - pad + 11}">` +
      `${f < 1000 ? Math.round(f) : (f / 1000).toFixed(1) + "k"}</text>`);
  }

  for (const p of [...partials].sort((a, b) => b.amp - a.amp)) {
    const colour = harmColours[p.note % harmColours.length];
    parts.push(`<line class="harm-partial" stroke="${colour}" ` +
      `x1="${x(p.freq).toFixed(1)}" y1="${(H - pad).toFixed(1)}" ` +
      `x2="${x(p.freq).toFixed(1)}" y2="${y(p.amp).toFixed(1)}"/>`);
  }

  // Where two notes' partials meet, which is what consonance is made of.
  for (const c of d.coincident || []) {
    parts.push(`<circle class="harm-meet" cx="${x(c.freq).toFixed(1)}" ` +
      `cy="${(pad - 4).toFixed(1)}" r="3"/>`);
  }

  box.innerHTML = `<svg viewBox="0 0 ${W} ${H}" class="harm-svg" ` +
    `role="img" aria-label="The partials of the sounding notes">${parts.join("")}</svg>`;

  const legend = $("#harm-legend");
  if (legend) {
    const met = (d.coincident || []).length;
    legend.textContent = met === 0
      ? "No partials meet: nothing is reinforcing anything."
      : met === 1
        ? "One pair of partials meets (circled)."
        : `${met} pairs of partials meet (circled).`;
  }
}

// drawIntervals lists what is between each pair of notes, and how far equal
// temperament has moved it from the ratio it is named after.
function drawIntervals(d) {
  const table = $("#harm-intervals");
  if (!table) return;
  const rows = d.intervals || [];
  if (!rows.length) {
    table.innerHTML = "";
    return;
  }
  const name = (i) => {
    const n = (d.notes || [])[i];
    const colour = harmColours[i % harmColours.length];
    return `<span style="color:${colour}">${n ? n.name : "?"}</span>`;
  };
  table.innerHTML =
    `<tr><th>Notes</th><th>Interval</th><th>Just</th><th>Tempered by</th></tr>` +
    rows.map((iv) => {
      // Two cents is nothing; fifteen is a third on a piano, and audible as
      // a slow beat. Saying which is which is the point of the column.
      const off = Math.abs(iv.cents) < 5 ? "harm-near" : "harm-far";
      return `<tr><td>${name(iv.a)}–${name(iv.b)}</td><td>${iv.name}</td>` +
        `<td>${iv.ratio}</td>` +
        `<td class="${off}">${iv.cents >= 0 ? "+" : ""}${iv.cents.toFixed(1)}¢</td></tr>`;
    }).join("");
}

/* ---------- how far each staff reaches ---------- */

// The one check a composer cannot do for themselves.
//
// Everything else about a part can be heard by reading it: whether the tune
// is any good, whether the rhythm sits, whether the harmony moves. Whether a
// horn can play a written passage cannot — it depends on the horn and not on
// the music, and somebody who does not play one has no way of knowing and no
// way of finding out except by asking a player.
//
// Two different kinds of answer come back and the panel keeps them apart. A
// note outside the instrument altogether is a fact: it will not be played.
// A part sitting above the comfortable stretch is a judgement: the top of a
// trumpet part is where the piece arrives, and that is the point of it.
// Running the two together would make the facts arguable and the judgements
// binding, and both of those are worse than saying nothing.

// The lowest and highest notes a piano has, which every staff's strip is
// drawn against so that two staves can be compared by looking at them.
const rangeLowest = 21;
const rangeHighest = 108;

async function openRange() {
  const dlg = $("#range");
  if (!dlg) return;
  if (!dlg.open) dlg.showModal();
  await refreshRange();
}

async function refreshRange() {
  const box = $("#range-body");
  if (!box) return;
  let d;
  try {
    const res = await fetch("/api/reach");
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // a reader, not an editor: a failed read is nothing
  }
  drawRange(d);
}

function drawRange(d) {
  const box = $("#range-body");
  if (!box) return;
  const parts = d.parts || [];
  if (!parts.length) {
    box.innerHTML = `<p class="small">There is nothing written yet.</p>`;
    return;
  }
  box.innerHTML = parts.map(rangeStaff).join("");

  const where = $("#range-where");
  if (where) {
    const found = parts.reduce((n, p) => n + (p.findings || []).length, 0);
    where.textContent = found === 0
      ? `${parts.length} ${parts.length === 1 ? "staff" : "staves"}, nothing out of reach`
      : `${found} thing${found === 1 ? "" : "s"} to look at`;
  }
}

// rangeStaff is one staff's row: what it covers, drawn, and anything worth
// saying about it.
function rangeStaff(p) {
  const head =
    `<div class="range-name"><b>${escapeHTML(p.name || "Staff " + (p.part + 1))}</b>` +
    `<span class="range-inst">${escapeHTML(p.instrument || "")}</span></div>`;

  if (p.percussion) {
    return `<div class="range-staff">${head}` +
      `<div class="range-quiet">Percussion — its lines are places on a kit ` +
      `rather than pitches, so there is no range to ask about.</div></div>`;
  }
  if (!p.notes) {
    return `<div class="range-staff">${head}` +
      `<div class="range-quiet">Nothing written on it.</div></div>`;
  }

  const says =
    `${escapeHTML(p.lowest.name)} to ${escapeHTML(p.highest.name)}` +
    (p.tessitura ? `, sitting ${escapeHTML(midiName(p, p.tessitura.low))}` +
      `–${escapeHTML(midiName(p, p.tessitura.high))}` : "");

  return `<div class="range-staff">${head}${rangeStrip(p)}` +
    `<div class="range-says">${says}</div>${rangeFindings(p)}</div>`;
}

// midiName names a pitch the panel only has a number for. The two ends of
// the part arrive named; the tessitura is a number because it is worked out
// rather than read off a note.
function midiName(p, midi) {
  if (midi === p.lowest?.midi) return p.lowest.name;
  if (midi === p.highest?.midi) return p.highest.name;
  const names = ["C", "C♯", "D", "E♭", "E", "F", "F♯", "G", "A♭", "A", "B♭", "B"];
  return names[((midi % 12) + 12) % 12] + (Math.floor(midi / 12) - 1);
}

// rangeStrip draws the staff against its instrument: the whole keyboard as
// the ruler, the instrument's range as the track, the comfortable stretch
// lighter inside it, and what is written as a bar across it.
function rangeStrip(p) {
  const W = 300, H = 20;
  const x = (midi) => {
    const t = (midi - rangeLowest) / (rangeHighest - rangeLowest);
    return Math.max(0, Math.min(1, t)) * W;
  };
  const parts = [];
  // Every octave C, so the eye has something to measure against.
  for (let m = 24; m <= rangeHighest; m += 12) {
    parts.push(`<line class="range-grid" x1="${x(m).toFixed(1)}" y1="2" ` +
      `x2="${x(m).toFixed(1)}" y2="${H - 2}" stroke="currentColor" ` +
      `stroke-opacity=".12"/>`);
  }
  if (p.playable) {
    parts.push(`<rect x="${x(p.playable.low).toFixed(1)}" y="7" ` +
      `width="${(x(p.playable.high) - x(p.playable.low)).toFixed(1)}" height="6" ` +
      `rx="3" fill="currentColor" fill-opacity=".12"/>`);
  }
  if (p.easy) {
    parts.push(`<rect x="${x(p.easy.low).toFixed(1)}" y="7" ` +
      `width="${(x(p.easy.high) - x(p.easy.low)).toFixed(1)}" height="6" ` +
      `rx="3" fill="currentColor" fill-opacity=".16"/>`);
  }
  // What is written: the whole span as a line, the tessitura solid.
  parts.push(`<line x1="${x(p.lowest.midi).toFixed(1)}" y1="10" ` +
    `x2="${x(p.highest.midi).toFixed(1)}" y2="10" stroke="currentColor" ` +
    `stroke-width="1.5"/>`);
  if (p.tessitura) {
    parts.push(`<rect x="${x(p.tessitura.low).toFixed(1)}" y="6" ` +
      `width="${Math.max(2, x(p.tessitura.high) - x(p.tessitura.low)).toFixed(1)}" ` +
      `height="8" rx="2" class="range-tessitura"/>`);
  }
  // The two ends, so the extremes are not lost in the band.
  for (const end of [p.lowest, p.highest]) {
    parts.push(`<line x1="${x(end.midi).toFixed(1)}" y1="4" ` +
      `x2="${x(end.midi).toFixed(1)}" y2="16" stroke="currentColor" stroke-width="1.5"/>`);
  }
  return `<svg class="range-strip" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" ` +
    `role="img" aria-label="Where this staff sits in its instrument's range">` +
    parts.join("") + `</svg>`;
}

function rangeFindings(p) {
  const list = p.findings || [];
  if (!list.length) return "";
  return `<ul class="range-findings">` + list.map((f) => {
    const kind = f.severity === "impossible" ? "range-impossible" : "range-strain";
    const mark = f.severity === "impossible" ? "cannot be played" : "worth a look";
    const bars = f.fromBar === f.toBar
      ? `bar ${barNumberOf(f.fromBar)}`
      : `bars ${barNumberOf(f.fromBar)}–${barNumberOf(f.toBar)}`;
    return `<li class="${kind}"><span class="mark">${mark}</span> — ` +
      `<button type="button" class="range-go" data-part="${p.part}" ` +
      `data-bar="${f.worst?.bar ?? f.fromBar}">${escapeHTML(f.text)} ` +
      `<span class="where">(${bars})</span></button></li>`;
  }).join("") + `</ul>`;
}

// Clicking a finding goes to the bar it is about, on the staff it is about.
function setupRangeClicks() {
  const box = $("#range-body");
  if (!box) return;
  box.addEventListener("click", async (e) => {
    const go = e.target.closest ? e.target.closest(".range-go") : null;
    if (!go) return;
    const dlg = $("#range");
    if (dlg?.open) dlg.close();
    await goToMeasure(Number(go.dataset.bar), Number(go.dataset.part));
  });
}

/* ---------- where the music goes ---------- */

// The key the music is in, through the piece.
//
// Not what the key signature says. A signature is written once and stands
// until somebody writes another one; the music moves through keys inside it
// all the time, and where it moves is most of what there is to understand
// about a piece. The development of a sonata is the signature of the
// exposition and almost none of its keys.
//
// Read from how long each of the twelve notes sounds over the phrase around
// each bar, against profiles of how long each degree sounds in music known
// to be in a key. The method is Krumhansl and Schmuckler's and it is
// arithmetic on the written notes, so it answers before anything is played.

async function openKeyMap() {
  const dlg = $("#keymap");
  if (!dlg) return;
  if (!dlg.open) dlg.showModal();
  await refreshKeyMap();
}

async function refreshKeyMap() {
  let d;
  try {
    const res = await fetch("/api/keys");
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // a reader, not an editor
  }
  drawKeyMap(d);
}

// keyColour puts related keys next to each other in colour, by where the
// tonic falls on the circle of fifths rather than where it falls on the
// keyboard. A move to the dominant is then a small step of hue and a move to
// the far side of the circle is the other side of the wheel — which is what
// the circle is a picture of, and is the thing worth seeing at a glance.
function keyColour(key, muted) {
  const pc = (((key?.tonic ?? 0) % 12) + 12) % 12;
  const hue = ((pc * 7) % 12) * 30;
  if (key?.minor) return `hsl(${hue} 38% ${muted ? 62 : 46}%)`;
  return `hsl(${hue} 58% ${muted ? 72 : 58}%)`;
}

function drawKeyMap(d) {
  const strip = $("#keymap-strip");
  const body = $("#keymap-body");
  if (!strip || !body) return;
  const keys = d.keys || [];
  const bars = d.bars || 0;
  if (!keys.length || !bars) {
    strip.innerHTML = "";
    body.innerHTML = `<p class="small">There is not enough music to read a key from.</p>`;
    return;
  }

  // The whole piece as one bar, each stretch as wide as it is long.
  strip.innerHTML =
    `<div class="keymap-bar">` +
    keys.map((k) => {
      const width = ((k.toBar - k.fromBar + 1) / bars) * 100;
      const title = `${k.name}, ${barsLabel(k)}`;
      return `<span class="keymap-seg${k.clear ? "" : " unsettled"}" ` +
        `style="width:${width.toFixed(3)}%;background:${keyColour(k.key, !k.clear)}" ` +
        `title="${escapeHTML(title)}"></span>`;
    }).join("") +
    `</div><div class="keymap-scale"><span>${escapeHTML(barsAt(0))}</span>` +
    `<span>${escapeHTML(barsAt(bars - 1))}</span></div>`;

  body.innerHTML = keys.map((k) =>
    `<button type="button" class="keymap-row" data-bar="${k.fromBar}">` +
    `<span class="keymap-swatch" style="background:${keyColour(k.key, !k.clear)}"></span>` +
    `<span class="keymap-name">${escapeHTML(k.name)}</span>` +
    `<span class="keymap-bars">${escapeHTML(barsLabel(k))}</span>` +
    (k.clear ? "" : `<span class="keymap-doubt">unsettled</span>`) +
    `</button>`).join("");

  const where = $("#keymap-where");
  if (where) {
    const moves = keys.length - 1;
    where.textContent = moves <= 0
      ? `${bars} bars, and it never leaves ${keys[0].name}`
      : `${bars} bars, through ${keys.length} keys`;
  }
}

// barsLabel names a stretch the way the bars are numbered on the page,
// which in a piece with movements is from the start of each movement.
function barsLabel(k) {
  return k.fromBar === k.toBar
    ? `bar ${barNumberOf(k.fromBar)}`
    : `bars ${barNumberOf(k.fromBar)}–${barNumberOf(k.toBar)}`;
}

function barsAt(bar) {
  const m = movementAt(bar);
  const n = barNumberOf(bar);
  return m && m.title ? `${m.title}, bar ${n}` : `bar ${n}`;
}

// Clicking a stretch goes to where it begins.
function setupKeyMapClicks() {
  const body = $("#keymap-body");
  if (!body) return;
  body.addEventListener("click", async (e) => {
    const row = e.target.closest ? e.target.closest(".keymap-row") : null;
    if (!row) return;
    const dlg = $("#keymap");
    if (dlg?.open) dlg.close();
    await goToMeasure(Number(row.dataset.bar));
  });
}

/* ---------- the shape of a piece ---------- */

// Who is playing when, and what repeats what.
//
// Both are questions about the whole of a piece, and the editor could answer
// nothing of that kind. A symphony is fifty-three staves and two and a half
// thousand bars: there was no way to see which instruments carry a passage,
// where the material comes back, or where the piece is thick and where it
// thins out — all of which are read off a score in a glance by anybody
// holding one, and none of which a window showing twelve bars can show.
//
// What repeats what is the nearer thing to an explanation. Music is made of
// things coming back, and where they come back is the skeleton somebody
// listening is trying to hear.

// mapColumns is how many columns the piece is drawn in.
//
// A bar of the Ninth is a fifth of a pixel wide, so the bars are gathered
// into columns and each column shows the busiest bar in it. Busiest and not
// the average: an entry is the thing worth seeing, and averaging it with the
// rests around it is how an entry disappears.
const mapColumns = 360;

// mapLevels is how many shades the strip is drawn in. Five is enough to tell
// resting from playing from a tutti, and few enough that the rectangles join
// into runs instead of being one per bar.
const mapLevels = 5;

async function openScoreMap() {
  const dlg = $("#scoremap");
  if (!dlg) return;
  if (!dlg.open) dlg.showModal();
  await refreshScoreMap();
}

async function refreshScoreMap() {
  let d;
  try {
    const res = await fetch("/api/map");
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // a reader, not an editor
  }
  drawScoreMap(d);
}

function drawScoreMap(d) {
  const body = $("#map-body");
  if (!body) return;
  const bars = d.bars || 0;
  if (!bars) {
    body.innerHTML = `<p class="small">There is nothing written yet.</p>`;
    return;
  }
  const staves = d.staves || [];
  const echoes = d.echoes || [];

  body.innerHTML =
    `<h3>who is playing when</h3>` +
    staves.map((s) => mapRow(s, bars, d.busiest || 1)).join("") +
    `<h3>what repeats what</h3>` +
    (echoes.length
      ? echoes.map(mapEcho).join("")
      : `<p class="small">Nothing in it comes back — no stretch of two bars ` +
        `or more is heard twice.</p>`);

  const where = $("#map-where");
  if (where) {
    where.textContent =
      `${bars} bars · ${staves.length} ${staves.length === 1 ? "staff" : "staves"}` +
      (echoes.length ? ` · ${echoes.length} return${echoes.length === 1 ? "" : "s"}` : "");
  }
}

function mapRow(s, bars, busiest) {
  return `<div class="map-row"><span class="map-staff" title="${escapeHTML(s.name)}">` +
    `${escapeHTML(s.name)}</span>${mapStrip(s.bars || [], bars, busiest)}</div>`;
}

// mapStrip draws one staff's whole part as a strip.
//
// Adjacent columns of the same shade are drawn as one rectangle rather than
// one each: a staff that rests for eighty bars is one wide pale rectangle,
// and a symphony comes out in a few hundred shapes rather than twenty
// thousand.
function mapStrip(counts, bars, busiest) {
  const W = mapColumns, H = 9;
  const per = bars / W;
  const level = (col) => {
    let most = 0;
    const from = Math.floor(col * per);
    const to = Math.max(from + 1, Math.ceil((col + 1) * per));
    for (let i = from; i < to && i < counts.length; i++) {
      if (counts[i] > most) most = counts[i];
    }
    if (most === 0) return 0;
    const n = Math.ceil((most / Math.max(1, busiest)) * (mapLevels - 1));
    return Math.min(mapLevels - 1, Math.max(1, n));
  };

  const parts = [`<rect class="map-rest" x="0" y="0" width="${W}" height="${H}"/>`];
  let at = 0;
  while (at < W) {
    const here = level(at);
    let end = at + 1;
    while (end < W && level(end) === here) end++;
    if (here > 0) {
      const shade = 0.22 + (here / (mapLevels - 1)) * 0.78;
      parts.push(`<rect x="${at}" y="0" width="${end - at}" height="${H}" ` +
        `class="map-busy" fill-opacity="${shade.toFixed(2)}"/>`);
    }
    at = end;
  }
  return `<svg class="map-busy" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" ` +
    `role="img" aria-label="Where this staff plays">${parts.join("")}</svg>`;
}

function mapEcho(e) {
  const here = e.fromBar === e.toBar
    ? `bar ${barNumberOf(e.fromBar)}`
    : `bars ${barNumberOf(e.fromBar)}–${barNumberOf(e.toBar)}`;
  const there = e.bars === 1
    ? `bar ${barNumberOf(e.likeBar)}`
    : `bars ${barNumberOf(e.likeBar)}–${barNumberOf(e.likeBar + e.bars - 1)}`;
  return `<button type="button" class="map-echo" data-bar="${e.fromBar}">` +
    `<span><b>${escapeHTML(here)}</b> are ${escapeHTML(there)} again</span>` +
    `<span class="moved">${escapeHTML(movedBy(e.transposed))}</span></button>`;
}

// movedBy says how far a return has moved, in the words a musician uses: the
// recapitulation of a sonata brings its second subject back a fifth lower,
// and "−7 semitones" is the same fact in a language nobody reads music in.
function movedBy(semitones) {
  if (!semitones) return "the same notes";
  const names = ["", "a semitone", "a tone", "a minor third", "a major third",
                 "a fourth", "a tritone", "a fifth", "a minor sixth",
                 "a major sixth", "a minor seventh", "a major seventh"];
  const up = semitones > 0;
  const n = Math.abs(semitones);
  const octaves = Math.floor(n / 12);
  const rest = n % 12;
  let what = names[rest] || "";
  if (octaves && rest) what = `${what} and ${octaves === 1 ? "an octave" : octaves + " octaves"}`;
  else if (octaves) what = octaves === 1 ? "an octave" : `${octaves} octaves`;
  return `${what} ${up ? "higher" : "lower"}`;
}

function setupScoreMapClicks() {
  const body = $("#map-body");
  if (!body) return;
  body.addEventListener("click", async (e) => {
    const row = e.target.closest ? e.target.closest(".map-echo") : null;
    if (!row) return;
    const dlg = $("#scoremap");
    if (dlg?.open) dlg.close();
    await goToMeasure(Number(row.dataset.bar));
  });
}

/* ---------- how the lines move against each other ---------- */

// The one part of the analysis that judges rather than measures, and it has
// to be honest about that.
//
// Parallel fifths are not wrong. They are wrong *in a tradition* — roughly,
// European art music between Palestrina and Brahms — and they are the whole
// sound of organum, of Debussy, of most film music and of every power chord
// ever played. A panel that said "error" would be saying something false
// about every one of those.
//
// So nothing here is an error. Each finding says what was seen, where, and
// in which pair of lines, and the composer decides whether it is what they
// meant. Where it is, they ignore it and have lost a glance; where it is
// not, they have been handed the one thing that is genuinely hard to see in
// your own work, because you know what you meant by it.

async function openVoiceLeading() {
  const dlg = $("#voices");
  if (!dlg) return;
  if (!dlg.open) dlg.showModal();
  await refreshVoiceLeading();
}

async function refreshVoiceLeading() {
  let d;
  try {
    const res = await fetch("/api/voices");
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // a reader, not an editor
  }
  drawVoiceLeading(d);
}

// What it judges by, said once at the top rather than argued in every line.
const voicesTradition =
  "Judged by the part-writing of European art music between Palestrina and " +
  "Brahms, which is the tradition these rules come from and the only one " +
  "they are about. Nothing here is an error: parallel fifths are the sound " +
  "of organum, of Debussy and of every power chord ever played. What is " +
  "offered is what was seen, so that you can decide whether it is what you " +
  "meant.";

function drawVoiceLeading(d) {
  const body = $("#voices-body");
  const where = $("#voices-where");
  if (!body) return;

  if (d.tooMany) {
    body.innerHTML = `<p class="voices-tradition">${escapeHTML(voicesTradition)}</p>` +
      `<p class="small">This score has ${d.lines} lines of music, and the ` +
      `rules are about music of ${d.tooMany} or fewer — a chorale, a quartet, ` +
      `a keyboard piece in parts. The fifths between a second clarinet and a ` +
      `cello eight bars apart in register are not a fact anybody can use, and ` +
      `there would be thousands of them.</p>`;
    if (where) where.textContent = `${d.lines} lines — too many to be worth asking`;
    return;
  }

  const motions = d.motions || [];
  body.innerHTML = `<p class="voices-tradition">${escapeHTML(voicesTradition)}</p>` +
    (motions.length
      ? motions.map(voiceLeadingRow).join("")
      : `<p class="small">Nothing to report: no parallel fifths or octaves, ` +
        `no crossing, no overlap.</p>`);

  if (where) {
    const lines = `${d.lines} line${d.lines === 1 ? "" : "s"}`;
    where.textContent = motions.length
      ? `${lines} · ${motions.length} thing${motions.length === 1 ? "" : "s"} to look at`
      : `${lines} · nothing to look at`;
  }
}

function voiceLeadingRow(m) {
  const bars = m.fromBar === m.toBar
    ? `bar ${barNumberOf(m.fromBar)}`
    : `bars ${barNumberOf(m.fromBar)}–${barNumberOf(m.toBar)}`;
  // How often, where it is more than once. A left hand in octaves does the
  // same thing in every bar for pages, and the count is what says so.
  const often = m.times > 1 ? `, ${m.times} times` : "";
  return `<button type="button" class="voices-row" data-bar="${m.fromBar}">` +
    `<span><span class="voices-kind">${escapeHTML(m.kind)}</span>` +
    `${escapeHTML(m.text)}</span>` +
    `<span class="where">${escapeHTML(bars + often)}</span></button>`;
}

function setupVoiceLeadingClicks() {
  const body = $("#voices-body");
  if (!body) return;
  body.addEventListener("click", async (e) => {
    const row = e.target.closest ? e.target.closest(".voices-row") : null;
    if (!row) return;
    const dlg = $("#voices");
    if (dlg?.open) dlg.close();
    await goToMeasure(Number(row.dataset.bar));
  });
}

/* ---------- how low the bottom of the harmony is spaced ---------- */

// The roughness of the harmonics panel, pointed outward.
//
// That panel answers "is this chord rough" with a number. An orchestrator is
// asking the same question the other way round — "will this spacing speak
// down there" — and wants a warning on a bar rather than a number in a
// dialog. It is the same physics: low notes have their partials packed close
// together in frequency, so an interval that is clear in the middle of the
// piano beats against itself an octave and a half down. A major third on
// middle C is a chord; the same third two octaves lower is a rumble with no
// third in it.
//
// Advice and never a fault. The limits are conventional and the books do not
// agree to the semitone, a piano's left hand does this all day and is fine,
// and a composer writing a low third in the bassoons may want exactly that
// rumble. Plenty of music is made of it.

const spacingNote =
  "An orchestrator's guideline, about chords held low rather than notes " +
  "going past: the lower two notes of a sonority have their partials packed " +
  "close enough together to beat, and the narrower the interval the higher " +
  "it has to sit to stay clear. The numbers are conventional and the books " +
  "differ by a semitone or so. A piano's left hand does this all day and is " +
  "fine; a low third in the bassoons may be exactly the rumble you want.";

// spacingDeepest is how far below a limit the bar is drawn full. Past an
// octave under there is nothing left to say but "very".
const spacingDeepest = 12;

async function openSpacing() {
  const dlg = $("#spacing");
  if (!dlg) return;
  if (!dlg.open) dlg.showModal();
  await refreshSpacing();
}

async function refreshSpacing() {
  let d;
  try {
    const res = await fetch("/api/spacing");
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // a reader, not an editor
  }
  drawSpacing(d);
}

function drawSpacing(d) {
  const body = $("#spacing-body");
  const where = $("#spacing-where");
  if (!body) return;
  const thick = d.thick || [];

  body.innerHTML = `<p class="voices-tradition">${escapeHTML(spacingNote)}</p>` +
    (thick.length
      ? thick.map(spacingRow).join("")
      : `<p class="small">Nothing held low enough to turn thick.</p>`);

  if (where) {
    where.textContent = thick.length
      ? `${thick.length} place${thick.length === 1 ? "" : "s"} to listen to`
      : "Nothing to listen to";
  }
}

function spacingRow(t) {
  const bars = t.fromBar === t.toBar
    ? `bar ${barNumberOf(t.fromBar)}`
    : `bars ${barNumberOf(t.fromBar)}–${barNumberOf(t.toBar)}`;
  const often = t.times > 1 ? `, ${t.times} times` : "";
  const deep = Math.min(100, (t.below / spacingDeepest) * 100);
  return `<button type="button" class="spacing-row" data-bar="${t.fromBar}" ` +
    `title="${escapeHTML(t.below + " semitones below the usual limit")}">` +
    `<span class="spacing-deep"><i style="width:${deep.toFixed(0)}%"></i></span>` +
    `<span>${escapeHTML(t.text)}</span>` +
    `<span class="where">${escapeHTML(bars + often)}</span></button>`;
}

function setupSpacingClicks() {
  const body = $("#spacing-body");
  if (!body) return;
  body.addEventListener("click", async (e) => {
    const row = e.target.closest ? e.target.closest(".spacing-row") : null;
    if (!row) return;
    const dlg = $("#spacing");
    if (dlg?.open) dlg.close();
    await goToMeasure(Number(row.dataset.bar));
  });
}

/* ---------- where the notes fall against the beat ---------- */

// A metre is a pattern of strong and weak, and almost everything a rhythm
// does it does by agreeing with that pattern or pulling against it. A bar of
// four with the notes on one and three is the metre spoken plainly; the same
// notes a quaver late are the same notes and a different piece.
//
// What comes back is a description and not a verdict: syncopation is not a
// fault, it is most of what makes music move. Two things in the panel are
// checks rather than descriptions and are kept apart from it — a bar holding
// more or less than its metre says, and a passage written faster than it can
// be articulated.

async function openRhythm() {
  const dlg = $("#rhythm");
  if (!dlg) return;
  if (!dlg.open) dlg.showModal();
  await refreshRhythm();
}

async function refreshRhythm() {
  let d;
  try {
    const res = await fetch("/api/rhythm");
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // a reader, not an editor
  }
  drawRhythm(d);
}

function drawRhythm(d) {
  const body = $("#rhythm-body");
  if (!body) return;
  const pulses = (d.pulses || []).filter((p) => p.notes > 0);
  if (!pulses.length) {
    body.innerHTML = `<p class="small">There is nothing written yet.</p>`;
    return;
  }
  const odd = d.odd || [];
  const rushed = d.rushed || [];

  body.innerHTML =
    `<h3>where the notes fall</h3>` +
    pulses.map((p) => beatRow(p, d.bars || 0)).join("") +
    (odd.length
      ? `<h3>bars that do not add up</h3>` + odd.map(oddRow).join("")
      : "") +
    (rushed.length
      ? `<h3>faster than it can be played</h3>` + rushed.map(rushedRow).join("")
      : "");

  const where = $("#rhythm-where");
  if (where) {
    const sync = pulses.reduce((n, p) => n + p.syncopated, 0);
    where.textContent = `${d.bars} bars · ` +
      (sync ? `${sync} notes pull against the metre` : "nothing pulls against the metre") +
      (odd.length ? ` · ${odd.length} bar${odd.length === 1 ? "" : "s"} do not add up` : "");
  }
}

function beatRow(p, bars) {
  const total = Math.max(1, p.notes);
  const part = (n) => (n / total) * 100;
  const pct = (n) => Math.round(part(n));
  return `<div class="beat-row">` +
    `<div class="beat-name"><b>${escapeHTML(p.name)}</b>` +
    `<span class="beat-of">${p.notes} notes · ${pct(p.downbeats)}% on the ` +
    `downbeat, ${pct(p.onBeat)}% on other beats, ${pct(p.offBeat)}% between them` +
    (p.syncopated ? ` · ${p.syncopated} syncopated` : "") +
    `</span></div>` +
    `<div class="beat-bar">` +
    `<span class="beat-down" style="width:${part(p.downbeats).toFixed(2)}%"></span>` +
    `<span class="beat-on" style="width:${part(p.onBeat).toFixed(2)}%"></span>` +
    `<span class="beat-off" style="width:${part(p.offBeat).toFixed(2)}%"></span>` +
    `</div>` +
    (p.syncopated ? mapStrip(p.bars || [], bars, maxOf(p.bars)) : "") +
    `</div>`;
}

function maxOf(ns) {
  let most = 1;
  for (const n of ns || []) if (n > most) most = n;
  return most;
}

function oddRow(o) {
  return `<button type="button" class="voices-row" data-bar="${o.bar}">` +
    `<span>${escapeHTML(o.text)}</span>` +
    `<span class="where">bar ${barNumberOf(o.bar)}</span></button>`;
}

function rushedRow(r) {
  const bars = r.fromBar === r.toBar
    ? `bar ${barNumberOf(r.fromBar)}`
    : `bars ${barNumberOf(r.fromBar)}–${barNumberOf(r.toBar)}`;
  const ms = Math.round(r.each * 1000);
  return `<button type="button" class="voices-row" data-bar="${r.fromBar}">` +
    `<span><span class="voices-kind">${escapeHTML(r.name)}</span>` +
    `${r.notes} notes of about ${ms} milliseconds each, where this instrument ` +
    `needs ${Math.round(r.limit * 1000)} to speak</span>` +
    `<span class="where">${escapeHTML(bars)}</span></button>`;
}

function setupRhythmClicks() {
  const body = $("#rhythm-body");
  if (!body) return;
  body.addEventListener("click", async (e) => {
    const row = e.target.closest ? e.target.closest(".voices-row") : null;
    if (!row) return;
    const dlg = $("#rhythm");
    if (dlg?.open) dlg.close();
    await goToMeasure(Number(row.dataset.bar));
  });
}

/* ---------- what each line is made of ---------- */

// A melody is a shape, and the shape is made of the intervals between one
// note and the next. A line that is nearly all steps is a tune somebody can
// hum after one hearing; a line of leaps is an accompaniment figure, a
// fanfare or an arpeggio. Neither is better. What is worth being told is
// which one you have written, because a composer writing a vocal line and
// getting the second is usually not doing it on purpose.
//
// A description and not a mark. Stravinsky's lines leap and are not
// improvable.

async function openMelody() {
  const dlg = $("#melody");
  if (!dlg) return;
  if (!dlg.open) dlg.showModal();
  await refreshMelody();
}

async function refreshMelody() {
  let d;
  try {
    const res = await fetch("/api/melody");
    if (!res.ok) return;
    d = await res.json();
  } catch (_) {
    return; // a reader, not an editor
  }
  drawMelody(d);
}

function drawMelody(d) {
  const body = $("#melody-body");
  if (!body) return;
  const shapes = (d.shapes || []).filter((s) => s.moves > 0);
  if (!shapes.length) {
    body.innerHTML = `<p class="small">There is nothing written yet.</p>`;
    return;
  }
  const awkward = d.awkward || [];
  body.innerHTML =
    `<h3>what each line is made of</h3>` +
    shapes.map(shapeRow).join("") +
    (awkward.length
      ? `<h3>moves worth a second look</h3>` + awkward.map(awkwardRow).join("")
      : "");

  const where = $("#melody-where");
  if (where) {
    const hum = shapes.filter((s) => (s.same + s.steps) / s.moves >= 0.66).length;
    where.textContent = `${shapes.length} line${shapes.length === 1 ? "" : "s"} · ` +
      `${hum} mostly stepwise`;
  }
}

// intervalWord names a width the way a musician says it, for the one
// interval a line uses more than any other.
function intervalWord(semitones) {
  const names = ["the same note", "a semitone", "a tone", "a minor third",
                 "a major third", "a fourth", "a tritone", "a fifth",
                 "a minor sixth", "a major sixth", "a minor seventh",
                 "a major seventh", "an octave"];
  return names[semitones] || "more than an octave";
}

function shapeRow(s) {
  const part = (n) => (n / Math.max(1, s.moves)) * 100;
  const stepwise = (s.same + s.steps) / Math.max(1, s.moves) >= 0.66;
  return `<div class="shape-row">` +
    `<div class="beat-name"><b>${escapeHTML(s.name)}</b>` +
    `<span class="shape-hum">${stepwise ? "mostly steps — singable" : "mostly leaps"}` +
    `</span></div>` +
    `<div class="shape-bar">` +
    `<span class="shape-same" style="width:${part(s.same).toFixed(2)}%"></span>` +
    `<span class="shape-step" style="width:${part(s.steps).toFixed(2)}%"></span>` +
    `<span class="shape-leap" style="width:${part(s.leaps).toFixed(2)}%"></span>` +
    `</div>` +
    `<div class="shape-says">${s.moves} moves · ${Math.round(part(s.steps))}% steps, ` +
    `${Math.round(part(s.same))}% repeated, ${Math.round(part(s.leaps))}% leaps · ` +
    `mostly ${escapeHTML(intervalWord(s.commonest))} · widest ` +
    `${escapeHTML(intervalWord(Math.min(13, s.widest)))} in bar ` +
    `${barNumberOf(s.widestBar)}</div></div>`;
}

function awkwardRow(a) {
  const bars = a.bar === a.toBar
    ? `bar ${barNumberOf(a.bar)}`
    : `bars ${barNumberOf(a.bar)}–${barNumberOf(a.toBar)}`;
  const often = a.times > 1 ? `, ${a.times} times` : "";
  return `<button type="button" class="voices-row" data-bar="${a.bar}">` +
    `<span><span class="voices-kind">${escapeHTML(a.name)}</span>` +
    `${escapeHTML(a.text)}</span>` +
    `<span class="where">${escapeHTML(bars + often)}</span></button>`;
}

function setupMelodyClicks() {
  const body = $("#melody-body");
  if (!body) return;
  body.addEventListener("click", async (e) => {
    const row = e.target.closest ? e.target.closest(".voices-row") : null;
    if (!row) return;
    const dlg = $("#melody");
    if (dlg?.open) dlg.close();
    await goToMeasure(Number(row.dataset.bar));
  });
}

/* ---------- typing music in ---------- */

// A line you type the music into, standing where the cursor is.
//
// NoteWriter II had one of these in 1988 and called it the command window:
// a line where you typed a short code for any symbol and it appeared. What
// made it fast was that your hands never left the keys — the review of it
// says the program is "easy to use and fast" for anybody who already knows
// how to write music, and that is the whole idea. Pointing at a staff is
// slow because a note is two facts, which pitch and how long, and a mouse
// can only say one of them at a time.
//
// What is typed here is the same two facts in the order a musician says
// them: how long, then which notes. "4 c d e f" is four quarter notes
// climbing. The length is sticky, the way a tool is, so a tune in quavers is
// "8" once and then letters.
//
// A number on its own is a length and a number inside a note is its octave:
// "4 c4" is a quarter note on middle C. That is the one thing to learn, and
// it is the reason lengths are numbers at all — the letters are needed for
// the notes, and e is the note E long before it is an eighth.
//
// Notes sounded together are a chord, and go in brackets: "2 [ceg]" is a
// half-note triad. ABC notation has written them that way since the 1990s,
// as has everyone who copied it — the same reason the barlines and the
// flats are spelled the way they are here — so for anybody who types music
// there is nothing in it to learn.

// typeHint is what the bar says it will take, shown under it.
const typeHint =
  "8abcdef — lengths 1 2 4 8 16 32, dotted 4. · letters a–g, with # for a " +
  "sharp, _ for a flat and n for a natural, and an octave: f#3 · " +
  "[ceg] the notes of a chord, from the bottom up · " +
  "+3 −2 for an interval from the note before · r rest · | next bar, || |: :| |] the barline it ends with · " +
  "tie, slur, stacc, ten, accent on everything after them, and off again · " +
  "abc … writes the rest of the line as ABC notation · " +
  "or the name of any command";

// typeBudget is how many things one line may write.
//
// A line is a phrase somebody typed, not a program: that is what the console
// is for. A thousand would be a mistake, and a mistake that takes a thousand
// notes back one at a time is a mistake nobody forgives.
const typeBudget = 256;

// MENU_DRAWINGS are the pictures for the entries that are not music.
//
// Cut, copy, paste, undo and the rest are not notation: a music font has
// no glyph for them, because scissors are not a thing anybody writes on a
// staff. They are drawn here instead, into the same 24-point square the
// music glyphs are fitted into, so the column of pictures stays a column.
//
// In the colour of the words rather than the red kept for marks. That red
// means "this is what the entry writes on the music"; these write nothing
// on the music, they move it about.
//
// Line drawings, one weight, no fill: at this size a solid shape is a
// blob, and the music glyphs beside them are strokes too.
// REMOVE_MARK is the picture every "take it away" entry carries.
//
// One picture for all of them rather than twelve. The entries differ in
// which mark they remove, and the words beside them say which; what they
// have in common is the only thing a drawing can show. A struck-through
// staccato and a struck-through tenuto are, at this size, two identical
// dots behind a stroke.
const REMOVE_MARK = '<circle cx="12" cy="12" r="8.5"/><path d="M6 18 18 6"/>';

const MENU_DRAWINGS = {
  // A fingering chart: the row of holes along an instrument, some
  // stopped and some open, which is what every chart in every method
  // book looks like and what the window draws.
  "flute-chart": '<line x1="2.5" y1="12" x2="21.5" y2="12"/>' +
    '<circle cx="6" cy="12" r="2.6" fill="currentColor"/>' +
    '<circle cx="12" cy="12" r="2.6"/>' +
    '<circle cx="18" cy="12" r="2.6" fill="currentColor"/>',
  undo: '<path d="M4.5 11h9.5a5 5 0 0 1 0 10H9"/><path d="M8.5 6.5 4 11l4.5 4.5"/>',
  redo: '<path d="M19.5 11H10a5 5 0 0 0 0 10h4.5"/><path d="M15.5 6.5 20 11l-4.5 4.5"/>',
  // Blades and handles, which is the one picture everybody reads as cut.
  cut: '<circle cx="6.5" cy="18" r="2.7"/><circle cx="17.5" cy="18" r="2.7"/>' +
    '<path d="M8.4 16 18 4"/><path d="M15.6 16 6 4"/>',
  copy: '<rect x="9" y="9" width="11" height="12" rx="2"/>' +
    '<path d="M15 6V5a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h1"/>',
  paste: '<path d="M9 4H7a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2h-2"/>' +
    '<rect x="9" y="2.5" width="6" height="4" rx="1"/>',
  // The key it is on, with what it does to what is under it.
  delete: '<path d="M21 5H9l-6 7 6 7h12a1 1 0 0 0 1-1V6a1 1 0 0 0-1-1z"/>' +
    '<path d="M12.5 9.5 18 15M18 9.5l-5.5 5.5"/>',
  // What is being picked up, with the dashes that mean picked up: a whole
  // staff, and a stretch of one.
  "select-all": '<path d="M4 9h16M4 13h16" opacity=".55"/>' +
    '<rect x="2" y="5" width="20" height="14" rx="2" stroke-dasharray="3 2.5"/>',
  "select-region": '<path d="M2 12h20" opacity=".55"/>' +
    '<rect x="7" y="5" width="10" height="14" rx="2" stroke-dasharray="3 2.5"/>',

  /* Files. A page, a folder and a floppy disc: nobody has put a disc in a
   * machine for twenty years and everybody still reads it as save, which
   * is the only thing an icon has to do. */
  "new": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8.5z"/>' +
    '<path d="M13.5 3v5.5H19"/>',
  "open": '<path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4L11 8.5h8.5A1.5 1.5 0 0 1 21 10v8a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18z"/>',
  "save": '<path d="M5 4h11l3.5 3.5V19a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/>' +
    '<path d="M8 4v5h7V4"/><path d="M7.5 20v-6h9v6"/>',
  // The same disc with a nib on it: saved, but under a name you choose.
  "save-as": '<path d="M5 4h9l3 3v4"/><path d="M4 5v14a1 1 0 0 0 1 1h5"/>' +
    '<path d="M8 4v4h5V4"/><path d="M19.5 12.5 13 19l-3 1 1-3 6.5-6.5a1.8 1.8 0 0 1 2.5 2.5z"/>',

  /* Writing the music out as something else. Four pictures and not nine:
   * the kinds of thing differ, the formats within a kind do not, and nine
   * drawings that each claimed to mean a particular file extension would
   * be nine guesses for a reader to make where the words beside them say
   * it outright. A page you look at, a page of markup, a keyboard and a
   * loudspeaker is the whole of the difference that can be drawn. */
  "save-pdf": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h4"/>' +
    '<path d="M13.5 3v5.5H19V12"/><path d="M14 18h7"/><path d="M18 15l3 3-3 3"/>',
  "save-svg": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8.5z"/>' +
    '<path d="M13.5 3v5.5H19"/><path d="M8 17.5c1.6-5 6.4-5 8 0"/>' +
    '<circle cx="8" cy="17.5" r="1.3"/><circle cx="16" cy="17.5" r="1.3"/>',
  "save-page": '<rect x="3" y="5" width="18" height="14" rx="2"/>' +
    '<path d="M3 9h18"/><circle cx="6.3" cy="7" r=".9"/><circle cx="9.1" cy="7" r=".9"/>',
  "save-musicxml": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8.5z"/>' +
    '<path d="M13.5 3v5.5H19"/><path d="M10.5 12.5 8 15l2.5 2.5"/><path d="M14 12.5 16.5 15 14 17.5"/>',
  "save-mxl": '<rect x="3" y="7" width="18" height="13" rx="2"/><path d="M3 11h18"/>' +
    '<path d="M10 15 8 17l2 2"/><path d="M14 15l2 2-2 2"/>',
  "save-abc": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8.5z"/>' +
    '<path d="M13.5 3v5.5H19"/><path d="M8.5 13h7M8.5 16h7M8.5 19h4"/>',
  "save-lilypond": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8.5z"/>' +
    '<path d="M13.5 3v5.5H19"/><circle cx="10" cy="17.5" r="2.1"/><path d="M12.1 17.5v-5.5"/>',
  "save-guido": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8.5z"/>' +
    '<path d="M13.5 3v5.5H19"/><path d="M11 12.5H9.4v6H11"/><path d="M14 12.5h1.6v6H14"/>',
  "save-midi": '<rect x="2.5" y="7" width="19" height="10" rx="1.5"/>' +
    '<path d="M8.2 7v6M12 7v6M15.8 7v6"/>',
  "save-audio": '<path d="M4 9.5h3.5L12 5.5v13L7.5 14.5H4z"/>' +
    '<path d="M16 9.5a4 4 0 0 1 0 5"/><path d="M18.6 7a7.5 7.5 0 0 1 0 10"/>',

  /* Printing, and the page it goes on. */
  "page-setup": '<rect x="4.5" y="3" width="15" height="18" rx="1.5"/>' +
    '<rect x="8" y="7" width="8" height="10" stroke-dasharray="2.5 2"/>',
  "print": '<path d="M7 9V3.5h10V9"/>' +
    '<path d="M7 17H5a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-2"/>' +
    '<rect x="7" y="14" width="10" height="7"/>',
  "print-preview": '<path d="M6.5 10V3.5h11V10"/>' +
    '<path d="M6.5 17H5a2 2 0 0 1-2-2v-3.5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2V15a2 2 0 0 1-2 2h-1.5"/>' +
    '<rect x="6.5" y="14" width="11" height="7"/><circle cx="12" cy="17.5" r="2"/>',
  "settings": '<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/>' +
    '<circle cx="16" cy="7" r="2.3"/><circle cx="8" cy="17" r="2.3"/>',
  // A door with the way out marked, which is what closing a window is.
  "close": '<path d="M14 4H6a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h8"/>' +
    '<path d="M11 12h10"/><path d="M18 9l3 3-3 3"/>',

  /* Taking a mark off again. One picture for all of them, because they are
   * all the same act: there is no drawing of "no staccato" that is not a
   * drawing of a staccato with something done to it, and twelve of those
   * side by side would be twelve near-identical dots. The words say which
   * mark; this says the entry takes one away. */
  // One long line with a notehead on it: the staff this entry makes.
  "staff-lines": '<line x1="2" y1="12" x2="22" y2="12"/>' +
    '<circle cx="9" cy="12" r="2.6"/>' +
    '<line x1="11.5" y1="12" x2="11.5" y2="5"/>',
  "artic-none": REMOVE_MARK,
  "breath-none": REMOVE_MARK,
  "slur-none": REMOVE_MARK,
  "tie-none": REMOVE_MARK,
  "dyn-none": REMOVE_MARK,
  "hairpin-none": REMOVE_MARK,
  "octave-none": REMOVE_MARK,
  "tuplet-none": REMOVE_MARK,
  "tremolo-none": REMOVE_MARK,
  "repeat-none": REMOVE_MARK,
  "ending-none": REMOVE_MARK,
  "group-none": REMOVE_MARK,
  "fingering-off": REMOVE_MARK,
  "tab-none": REMOVE_MARK,

  /* Cross-staff writing. Two staves and a note that has gone from one to
   * the other, which is the whole of what the entry does; which way the
   * note went is which staff it is on. */
  "cross-below": '<path d="M3 5.5H21"/><path d="M3 8.5H21"/>' +
    '<path d="M3 18.5H21"/><path d="M3 21.5H21"/>' +
    '<circle cx="9" cy="15" r="2.6"/><path d="M11.6 15V7"/>',
  "cross-above": '<path d="M3 2.5H21"/><path d="M3 5.5H21"/>' +
    '<path d="M3 15.5H21"/><path d="M3 18.5H21"/>' +
    '<circle cx="9" cy="9" r="2.6"/><path d="M11.6 9V17"/>',
  "cross-own": REMOVE_MARK,

  /* Playing the music in: the keyboard it is played on.
   *
   * Drawn rather than set from the font, because the font has no piano
   * keyboard in it — SMuFL's "keyboard" glyphs are the pedalling and
   * fingering marks written over one, not the thing itself. */
  "midi-input": '<rect x="2.5" y="7" width="19" height="10" rx="1.5"/>' +
    '<path d="M8.8 7V13"/><path d="M15.2 7V13"/>' +
    '<rect x="6.6" y="7" width="2.2" height="6"/>' +
    '<rect x="11" y="7" width="2.2" height="6"/>' +
    '<rect x="15.2" y="7" width="2.2" height="6"/>',

  /* The tab marks no font has a sign for.
   *
   * Each is the two notes it is about and the mark between them, because
   * that is what the mark means: a hammer-on is not something done to a
   * note, it is how you get from one note to the next. The pair of
   * hammer-on and pull-off, and of the two slides, are mirror images of
   * each other, which is the difference a guitarist reads them by. */
  "tab-hammer-on": '<circle cx="5.5" cy="18" r="2.2"/>' +
    '<circle cx="18.5" cy="12" r="2.2"/><path d="M5.5 14.8Q12 4.5 18.5 8.8"/>',
  "tab-pull-off": '<circle cx="5.5" cy="12" r="2.2"/>' +
    '<circle cx="18.5" cy="18" r="2.2"/><path d="M5.5 8.8Q12 4.5 18.5 14.8"/>',
  "tab-slide-up": '<circle cx="5" cy="18" r="2.2"/>' +
    '<circle cx="19" cy="9" r="2.2"/><path d="M8 16.1 16 10.9"/>',
  "tab-slide-down": '<circle cx="5" cy="9" r="2.2"/>' +
    '<circle cx="19" cy="18" r="2.2"/><path d="M8 10.9 16 16.1"/>',

  /* The two harmonics the font has no circle for. An artificial harmonic
   * is the diamond it is written with; a pinched one is the same note
   * with the pick biting into it, drawn as the rays coming off it. */
  "tab-artificial-harmonic": '<path d="M12 5 18 12 12 19 6 12Z"/>',
  "tab-pinch-harmonic": '<circle cx="12" cy="12" r="3.2"/>' +
    '<path d="M12 3.5V7"/><path d="M12 17V20.5"/>' +
    '<path d="M3.5 12H7"/><path d="M17 12H20.5"/>',

  /* Stems and beams, which are about how notes are joined rather than
   * about a mark written on one, so there is no glyph for them: a beam is
   * drawn between two stems and the font has stems and beams only as part
   * of whole notes. Two stems and what happens between them. */
  "stem-auto": '<circle cx="6" cy="16.5" r="2.6"/><path d="M8.6 16.5V5"/>' +
    '<circle cx="18" cy="7.5" r="2.6"/><path d="M15.4 7.5V19"/>',
  "beam-join": '<path d="M6 17V6.5M18 17V8.5"/><path d="M5 7.5l14-2v3l-14 2z"/>',
  "beam-break": '<path d="M6 17V6.5M18 17V8.5"/>' +
    '<path d="M5 7.5l4-.6v3l-4 .6z"/><path d="M15 6.1l4-.6v3l-4 .6z"/>',
  "beam-auto": '<path d="M6 17V6.5M18 17V8.5"/>' +
    '<path d="M5.5 7l13-1.9" stroke-dasharray="3 2.2"/>',

  /* Typing music instead of clicking it, and the line you type into. */
  "type-music": '<rect x="2.5" y="12.5" width="19" height="8" rx="1.8"/>' +
    '<path d="M6 15.5h.01M9.5 15.5h.01M13 15.5h.01M16.5 15.5h.01M8 18.5h8"/>' +
    '<circle cx="8.5" cy="7.5" r="2.2"/><path d="M10.7 7.5V2.5"/>',
  "console": '<rect x="2.5" y="4.5" width="19" height="15" rx="2"/>' +
    '<path d="M6.5 9.5 9.5 12l-3 2.5"/><path d="M12 15h5.5"/>',
  /* The score and the parts it is printed as. The score is a stack of
   * staves braced together; a part is one page with a staff on it; making
   * them is the score with the pages coming off it. */
  "show-score": '<path d="M4.5 4.5h15v15h-15z"/>' +
    '<path d="M7.5 8.5h8M7.5 12h8M7.5 15.5h8"/>',
  "show-part": '<path d="M6.5 3.5h11v17h-11z"/>' +
    '<path d="M9.5 9.5h5M9.5 13h5"/>',
  "part-setup": '<path d="M6.5 3.5h11v17h-11z"/>' +
    '<circle cx="12" cy="11.5" r="2.6"/>' +
    '<path d="M12 6.5v1.6M12 14.9v1.6M8.8 11.5h-1.1M16.3 11.5h-1.1"/>',
  "make-parts": '<path d="M3.5 4.5h11v11h-11z"/>' +
    '<path d="M6.5 8h5M6.5 11.5h5"/>' +
    '<path d="M10.5 18.5h10v-10"/><path d="M17.5 5.5 20.5 8.5 17.5 11.5"/>',
  /* One of the scripts in the plugins folder: a page with a few lines on it
   * and the arrow that runs it. Shared by every entry on the Scripts menu,
   * which is why it is keyed by the word in front of the colon — the
   * entries themselves are named after files nobody can draw in advance. */
  "plugin": '<path d="M5.5 3.5h8.5l4.5 4.5v12.5h-13z"/>' +
    '<path d="M14 3.5v5h4.5"/><path d="M8 12.5h5M8 15.5h3"/>' +
    '<polygon points="14,14.5 18,16.75 14,19"/>',
  "insert-text": '<path d="M5.5 7V5h13v2"/><path d="M12 5v14"/><path d="M9 19h6"/>',

  /* The bracket over the bars of a repeat, with the time through it covers.
   * The numbers are drawn rather than set, because at this size a digit
   * from a font is three pixels of stem and nothing else. */
  "ending-1": '<path d="M4 11V5.5h16"/><path d="M9.6 14.2 12 12.5V20"/>',
  "ending-2": '<path d="M4 11V5.5h16"/>' +
    '<path d="M9.3 14.4a2.5 2.5 0 1 1 4.5 1.7L9.3 20h5"/>',
  "ending-12": '<path d="M4 11V5.5h16"/><path d="M5 14.2 7 12.5V20"/>' +
    '<path d="M11.8 14.4a2.5 2.5 0 1 1 4.5 1.7L11.8 20h5"/>',
  // A tuplet whose number you choose: the bracket, and the number left open.
  "tuplet-other": '<path d="M4 8.5V13M20 8.5V13"/><path d="M4 8.5h5.5M14.5 8.5H20"/>' +
    '<circle cx="9" cy="17.5" r="1"/><circle cx="12" cy="17.5" r="1"/>' +
    '<circle cx="15" cy="17.5" r="1"/>',

  /* Symbols drawn by hand, and the drawer they are kept in. */
  "symbol-palette": '<rect x="3" y="3.5" width="7.5" height="7.5" rx="1.5"/>' +
    '<rect x="13.5" y="3.5" width="7.5" height="7.5" rx="1.5"/>' +
    '<rect x="3" y="13" width="7.5" height="7.5" rx="1.5"/>' +
    '<rect x="13.5" y="13" width="7.5" height="7.5" rx="1.5"/>',
  "draw-symbol": '<path d="M19.5 4.5a2.1 2.1 0 0 1 0 3L9 18l-4 1 1-4L16.5 4.5a2.1 2.1 0 0 1 3 0z"/>' +
    '<path d="M14.5 6.5 17.5 9.5"/>',
  "import-symbols": '<path d="M4 15v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3"/>' +
    '<path d="M12 3.5v11"/><path d="M8 10.5l4 4 4-4"/>',
  "export-symbols": '<path d="M4 15v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3"/>' +
    '<path d="M12 14.5v-11"/><path d="M8 7.5l4-4 4 4"/>',

  /* What belongs to the whole score. */
  "set-header": '<path d="M4.5 5.5h15M7 9.5h10"/>' +
    '<path d="M4.5 14.5h15M4.5 18.5h11"/>',
  // A stack of staves with a bracket down the left of it, which is what
  // this entry makes and what nothing else in the menu does.
  "score-setup": '<path d="M7 5.5h14M7 9.5h14M7 14.5h14M7 18.5h14"/>' +
    '<path d="M4 4.5v15"/><path d="M4 4.5c0-1 .8-1.6 1.8-1.6"/>' +
    '<path d="M4 19.5c0 1 .8 1.6 1.8 1.6"/>',
  // The short bar a piece can begin on, and the first full bar after it.
  "set-pickup": '<path d="M2.5 5.5v13M9.5 5.5v13M21.5 5.5v13"/>' +
    '<circle cx="5.3" cy="14.5" r="2"/><path d="M7.3 14.5V8"/>',
  "score-options": '<path d="M13.5 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8.5z"/>' +
    '<path d="M13.5 3v5.5H19"/><path d="M7.5 13.5h9M7.5 17.5h9"/>' +
    '<circle cx="10.5" cy="13.5" r="1.5"/><circle cx="13.5" cy="17.5" r="1.5"/>',
  "parameters": '<circle cx="12" cy="12" r="3.2"/>' +
    '<path d="M12 3v2.5M12 18.5V21M3 12h2.5M18.5 12H21M5.6 5.6l1.8 1.8M16.6 16.6l1.8 1.8M18.4 5.6l-1.8 1.8M7.4 16.6l-1.8 1.8"/>',

  /* Staves: the five lines, and what the entry does to them. */
  "add-staff": '<path d="M3 7h12M3 10h12M3 13h12M3 16h12M3 19h12" opacity=".7"/>' +
    '<path d="M19 4v7M15.5 7.5h7"/>',
  "remove-staff": '<path d="M3 7h12M3 10h12M3 13h12M3 16h12M3 19h12" opacity=".7"/>' +
    '<path d="M15.5 7.5h7"/>',
  "hide-staff": '<path d="M3 7h12M3 10h12M3 13h12M3 16h12M3 19h12" opacity=".7"/>' +
    '<path d="M15 4.5 22 20"/>',
  "show-all-staves": '<path d="M2.5 6h11M2.5 9h11M2.5 12h11M2.5 15h11M2.5 18h11" opacity=".7"/>' +
    '<path d="M13.5 17c2.2-3.4 6.3-3.4 8.5 0-2.2 3.4-6.3 3.4-8.5 0z"/>' +
    '<circle cx="17.7" cy="17" r="1.1"/>',
  "staff-up": '<path d="M3 10h12M3 13h12M3 16h12M3 19h12" opacity=".7"/>' +
    '<path d="M19 20V5"/><path d="M15.5 8.5 19 5l3.5 3.5"/>',
  "staff-down": '<path d="M3 5h12M3 8h12M3 11h12M3 14h12" opacity=".7"/>' +
    '<path d="M19 4v15"/><path d="M15.5 15.5 19 19l3.5-3.5"/>',
  /* A rehearsal mark and a chord symbol: the two things written over a
   * staff that are letters rather than signs.
   *
   * Neither is in the music font, which carries the coda, the segno and
   * the chord qualities but not the letters they are written beside — so
   * the font table had a coda standing for the rehearsal mark and a bare
   * major-seventh triangle standing for a chord symbol, and neither says
   * what the entry does. Drawn, they can be what they are. */
  "insert-marker": '<rect x="3.5" y="4.5" width="17" height="15" rx="1.5"/>' +
    '<path d="M8 16.3 12 7.7l4 8.6"/><path d="M9.4 13.4h5.2"/>',
  // The same boxed letter, with the next one of the sequence behind it:
  // this entry is about what they are all called, not about one of them.
  "marker-style": '<rect x="2.5" y="7.5" width="12" height="11" rx="1.5"/>' +
    '<path d="M5.6 15.8 8.5 9.5l2.9 6.3"/><path d="M6.6 13.7h3.8"/>' +
    '<rect x="15.5" y="5.5" width="6" height="6" rx="1" opacity=".5"/>',
  // A letter and the figure beside it, which is the shape of every chord
  // symbol ever written and belongs to nothing else.
  "insert-chord": '<path d="M13.8 8.6a6 6 0 1 0 0 7.4"/>' +
    '<path d="M16.4 5.2h5l-3.2 6.4"/>',

  // The instrument itself: a tube, its keys and the bell on the end.
  //
  // Not a clef, which is what this was. A clef is a thing written on a
  // staff and there are entries about writing one, so a treble clef here
  // read as "set the clef" or, next to the key signature entry, as the
  // key — which is the one place beside a key signature a sharp sign
  // must not be mistaken for.
  "set-instrument": '<path d="M3.2 10.3h11.3v3.4H3.2a1.7 1.7 0 0 1 0-3.4z"/>' +
    '<path d="M14.5 8 21.5 4.5v15L14.5 16z"/>' +
    '<path d="M6.6 10.3V7M9.8 10.3V7M13 10.3V7"/>',

  // Words under the staff, which is where lyrics go.
  "set-lyrics": '<path d="M3 4.5h18M3 7.5h18M3 10.5h18" opacity=".7"/>' +
    '<path d="M3.5 16h5M10 16h1.5M13 16h7.5"/><path d="M3.5 20h9"/>',

  /* Looking at the page rather than changing it. */
  "mode-strip": '<rect x="2" y="8" width="20" height="8" rx="1.5"/>' +
    '<path d="M7 8v8M12 8v8M17 8v8" opacity=".6"/>',
  "mode-page": '<rect x="3" y="3.5" width="8" height="17" rx="1.5"/>' +
    '<rect x="13" y="3.5" width="8" height="17" rx="1.5"/>',
  "show-all-tunes": '<path d="M4 6.5h16M4 12h16M4 17.5h16"/>' +
    '<circle cx="2" cy="6.5" r=".9"/><circle cx="2" cy="12" r=".9"/>' +
    '<circle cx="2" cy="17.5" r=".9"/>',
  "fullscreen": '<path d="M3.5 9V4.5a1 1 0 0 1 1-1H9"/><path d="M15 3.5h4.5a1 1 0 0 1 1 1V9"/>' +
    '<path d="M20.5 15v4.5a1 1 0 0 1-1 1H15"/><path d="M9 20.5H4.5a1 1 0 0 1-1-1V15"/>',
  "zoom-in": '<circle cx="10.5" cy="10.5" r="6.5"/><path d="M15.3 15.3 21 21"/>' +
    '<path d="M10.5 7.5v6M7.5 10.5h6"/>',
  "zoom-out": '<circle cx="10.5" cy="10.5" r="6.5"/><path d="M15.3 15.3 21 21"/>' +
    '<path d="M7.5 10.5h6"/>',
  "zoom-reset": '<circle cx="10.5" cy="10.5" r="6.5"/><path d="M15.3 15.3 21 21"/>' +
    '<path d="M8.6 9.3 10.5 8v5.5"/>',
  // A number written over a bar, which is what the entry decides the
  // printing of.
  "set-numbering": '<path d="M4 13h16M4 17h16M4 21h16" opacity=".6"/>' +
    '<path d="M4 13V21M12 13v8M20 13v8" opacity=".6"/>' +
    '<path d="M5.4 4.4 7 3.2V10"/>' +
    '<path d="M12.6 4.6a2.3 2.3 0 1 1 4.1 1.6L12.6 10h4.6"/>',

  // Lines of music, and where one is cut off from the next: two staves
  // with the break between them marked.
  "set-systems": '<path d="M3 4.5h18M3 8h18" opacity=".6"/>' +
    '<path d="M3 16h18M3 19.5h18" opacity=".6"/>' +
    '<path d="M3 12h5M16 12h5"/><path d="M11.5 9.5 13 12l-1.5 2.5"/>',

  // A magnifier over words, which is what is being searched: a score's
  // writing and not its notes.
  "find-text": '<path d="M3 5.5h9M3 9.5h7"/>' +
    '<circle cx="14" cy="14" r="5"/><path d="M17.8 17.8 21.5 21.5"/>',

  // A label, which is what a staff name is.
  "view-names": '<path d="M3 8.5a2 2 0 0 1 2-2h8l6 5.5-6 5.5H5a2 2 0 0 1-2-2z"/>' +
    '<circle cx="7" cy="12" r="1.1"/>',
  "view-numbers": '<rect x="3.5" y="6" width="17" height="12" rx="2"/>' +
    '<path d="M7.6 10.2 9.2 9v6"/><path d="M12.6 10.6a2 2 0 1 1 3.6 1.4L12.6 15h4"/>',
  // A tuning fork: concert pitch is the pitch a tuner sounds, not the one
  // the player reads.
  "view-concert": '<path d="M8 3.5v7a4 4 0 0 0 8 0v-7"/><path d="M12 14.5V21"/>' +
    '<path d="M9.5 21h5"/>',

  /* Playing. */
  "play": '<path d="M7 4.5 19.5 12 7 19.5z"/>',
  "play-from-start": '<path d="M9.5 4.5 21 12 9.5 19.5z"/><path d="M4.5 4.5v15"/>',
  "stop": '<rect x="5.5" y="5.5" width="13" height="13" rx="1.5"/>',
  "playback-sound": '<path d="M3.5 9.5H7L11.5 5.5v13L7 14.5H3.5z"/>' +
    '<path d="M15.5 9.5a4 4 0 0 1 0 5"/><path d="M18.2 7a7.5 7.5 0 0 1 0 10"/>',
  // A clock, because what is being set is a length of time, and a speaker
  // here would be the entry above it with a decoration.
  "sound-delay": '<circle cx="12" cy="12" r="8"/>' +
    '<path d="M12 7.5V12l3 2"/>',

  /* Looking at what the music is made of. */
  "range": '<path d="M4 5.5h16M4 18.5h16" opacity=".7"/><path d="M12 7.5v9"/>' +
    '<path d="M9 10.5 12 7.5l3 3"/><path d="M9 13.5 12 16.5l3-3"/>',
  "keymap": '<rect x="3" y="4.5" width="18" height="15" rx="2"/>' +
    '<path d="M9 4.5v15M15 4.5v15M3 9.5h18M3 14.5h18" opacity=".7"/>',
  "scoremap": '<rect x="2.5" y="6" width="19" height="12" rx="1.5"/>' +
    '<path d="M5 9.5h14M5 12h9M5 14.5h11" opacity=".7"/>' +
    '<rect x="3.8" y="8.2" width="7" height="7.5" rx="1"/>',
  // Two lines of music on one staff, which is what a voice is: the stems
  // tell them apart, so the stems are the picture.
  "voices": '<circle cx="6.5" cy="8" r="2.4"/><path d="M8.9 8V3"/>' +
    '<circle cx="16" cy="16" r="2.4"/><path d="M13.6 16v5"/>',
  "spacing": '<path d="M5 4.5v15M19 4.5v15"/><path d="M8 12h8"/>' +
    '<path d="M11 9 8 12l3 3"/><path d="M13 9l3 3-3 3"/>',
  "melody": '<path d="M3.5 17 8 12.5l4 2.5 8.5-8"/>' +
    '<circle cx="3.5" cy="17" r="1.3"/><circle cx="8" cy="12.5" r="1.3"/>' +
    '<circle cx="12" cy="15" r="1.3"/><circle cx="20.5" cy="7" r="1.3"/>',

  /* Help. */
  "shortcuts": '<rect x="2" y="6" width="20" height="12" rx="2"/>' +
    '<path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7.5 14h9"/>',
  "typing-help": '<circle cx="12" cy="12" r="9"/>' +
    '<path d="M9.4 9.6a2.7 2.7 0 1 1 3.4 3.2V15"/><path d="M12.8 18h.01"/>',
  "report-bug": '<rect x="8" y="7.5" width="8" height="11" rx="4"/>' +
    '<path d="M9.5 6 8 4M14.5 6 16 4"/>' +
    '<path d="M8 10.5H4.5M8 15H4.5M16 10.5h3.5M16 15h3.5"/>',
  "import-log": '<path d="M4 5.5h10M4 9.5h10M4 13.5h6"/>' +
    '<path d="M16.3 11.2 21.4 19.6a1 1 0 0 1-.9 1.4h-10a1 1 0 0 1-.9-1.4l5.1-8.4a1 1 0 0 1 1.6 0z"/>' +
    '<path d="M15.5 15v2.2M15.5 19h.01"/>',
  "about": '<circle cx="12" cy="12" r="9"/><path d="M12 11v5.5"/><path d="M12 7.8h.01"/>',
};

// menuDrawing wraps one of those in the square it is drawn in.
function menuDrawing(body) {
  return '<svg viewBox="0 0 24 24" width="24" height="23" fill="none" ' +
    'stroke="currentColor" stroke-width="2" stroke-linecap="round" ' +
    'stroke-linejoin="round">' + body + "</svg>";
}

// typeNote reads one token as a note, and says nothing for anything else.
//
// A letter a–g, any number of accidentals, and an octave. The accidentals
// are # or s for sharp, b for flat, x for a double sharp and n for a
// natural; a natural is worth having because in D major a bare f is the
// key's F sharp and "fn" is the F the writer meant.
function typeNote(token) {
  const m = /^([a-gA-G])([#sbxnSBXN]*)(-?\d+)?$/.exec(token);
  if (!m) return null;
  // An octave is one digit, from the sub-contra octave to the top of the
  // piano. Two of them is a length that was meant to be a new length and
  // was read as an octave instead, because inside a run the digits after a
  // note are its octave — "8a4b16cd" is A4 and then a B in octave sixteen.
  // Saying so is better than handing the server a note nobody can play.
  if (m[3] !== undefined) {
    const oct = Number(m[3]);
    if (!Number.isInteger(oct) || oct < -1 || oct > 9) return null;
  }
  // "b" on its own is the note B; "bb" is B flat. The letter is the first
  // character and everything after it is an accidental, which is what makes
  // that work without a special case.
  return { pitch: m[1] + (m[2] || "") + (m[3] || "") };
}

// typeBars are the barlines a bar can be ended with, written the way they
// are written everywhere music is typed: a plain | goes on to the next bar
// and leaves its barline alone, and these say what that barline is.
//
// ABC has spelled them this way since the 1990s and so does everyone who
// copied it, so there is nothing here to learn.
const typeBars = {
  "|": "",
  "||": "double",
  "|:": "repeat-start",
  ":|": "repeat-end",
  "|]": "final",
};

// typeLengths maps the numbers everybody already writes to the names the
// editor uses.
const typeLengths = {
  1: "whole", 2: "half", 4: "quarter", 8: "eighth",
  16: "sixteenth", 32: "32nd", 64: "64th",
};

// typeMarks are the words that turn a mark on for everything typed after
// them, rather than doing something once.
//
// "tie 8 a b d e f" ties the whole run: a tie is a thing a phrase has, not a
// thing you do to one note, and a typed phrase is where you say so. They
// work the way a note length works — on until you say otherwise — and the
// same word again turns the mark off, which is how NoteWriter's command line
// behaved and the only rule simple enough not to need remembering.
const typeMarks = {
  tie: { kind: "tie" },
  slur: { kind: "slur" },
  staccato: { kind: "articulation", name: "staccato" },
  stacc: { kind: "articulation", name: "staccato" },
  tenuto: { kind: "articulation", name: "tenuto" },
  ten: { kind: "articulation", name: "tenuto" },
  accent: { kind: "articulation", name: "accent" },
  marcato: { kind: "articulation", name: "marcato" },
  fermata: { kind: "articulation", name: "fermata" },
};

// splitRun breaks a run of notes written without spaces into its pieces.
//
// "8abcdef" is six notes, and typing six spaces to say so is six keystrokes
// nobody should have to make: a scale is one gesture in the hand and ought
// to be one in the typing. Spaces are then only needed where a word begins.
//
// The one thing a run cannot have is b for a flat. In "8abcdef" the b is the
// note B, and it has to be, or the commonest thing anybody types comes out
// as A flat followed by five notes. So inside a run a flat is written _ —
// "a_" is A flat, the way ABC notation has spelled it since the 1990s. A
// note standing on its own as a whole token still takes b, because "bb4" is
// how everybody writes B flat and a token with one note in it cannot be
// misread.
function splitRun(token) {
  const out = [];
  let i = 0;
  while (i < token.length) {
    const c = token[i];
    if (c >= "0" && c <= "9") {
      let n = "";
      while (i < token.length && token[i] >= "0" && token[i] <= "9") n += token[i++];
      while (i < token.length && token[i] === ".") n += token[i++];
      out.push(n);
      continue;
    }
    const letter = c.toLowerCase();
    if (letter >= "a" && letter <= "g") {
      let note = c;
      i++;
      // Accidentals, and no b among them: in a run the b is the note.
      while (i < token.length && "#sSxXnN_".includes(token[i])) {
        note += token[i] === "_" ? "b" : token[i];
        i++;
      }
      // Then an octave, where one is given.
      while (i < token.length && token[i] >= "0" && token[i] <= "9") note += token[i++];
      out.push(note);
      continue;
    }
    // A barline, which may be one character or two: "|:" and ":|" and "||"
    // are each one thing, and reading them a character at a time would turn
    // a close repeat into a bar of nothing followed by a new bar.
    if (c === "|" || c === ":") {
      const two = token.slice(i, i + 2);
      if (typeBars[two] !== undefined) {
        out.push(two);
        i += 2;
        continue;
      }
      if (typeBars[c] === undefined) return null; // a lone colon is not music
      out.push(c);
      i++;
      continue;
    }
    if (c === "r" || c === "R") {
      out.push(c);
      i++;
      continue;
    }
    // A chord, taken whole: what is between the brackets is one sound and
    // is read as a chord, not as the run of separate notes its letters
    // would otherwise be. An unclosed bracket is handed on as it stands,
    // so that the reading of it can say what is missing.
    if (c === "[") {
      const close = token.indexOf("]", i);
      out.push(close < 0 ? token.slice(i) : token.slice(i, close + 1));
      i = close < 0 ? token.length : close + 1;
      continue;
    }
    return null; // not a run of music, so it is something else entirely
  }
  return out.length ? out : null;
}

// typeChord reads one token as a chord, and says what is wrong with it when
// it is not one.
//
// The noteheads go in brackets with nothing between them — "[ceg]" — which
// is how ABC notation has written a chord since the 1990s and so how
// everybody who types music already writes one. Inside the brackets the
// pieces are read by the same reader a run of notes uses, so b is the note
// B and _ is the flat there exactly as it is outside them.
//
// The length is not in the brackets. It goes in front, where a note's
// length goes and where it is already sticky — "4 [ceg]" — because a chord
// is one sound and a sound has one length.
function typeChord(token) {
  if (!token.startsWith("[")) {
    return { error: `${token} is not a chord — a chord is written [ceg]` };
  }
  // The brackets have to close, and nothing may come between them but the
  // notes: a line is split at its spaces before any of it is read, so
  // "[c e g]" arrives here as three pieces and the first of them is this.
  if (!token.endsWith("]")) {
    return {
      error: `${token} is not a chord — a chord is written [ceg], ` +
        `with no spaces inside the brackets`,
    };
  }
  const inner = token.slice(1, -1);
  if (!inner) return { error: "a chord needs some notes in it: [ceg]" };
  const pieces = splitRun(inner);
  if (!pieces) {
    return { error: `${token} is not a chord — a chord is written [ceg]` };
  }
  const pitches = [];
  for (const piece of pieces) {
    const note = typeNote(piece);
    if (!note) {
      if (/^\d/.test(piece)) {
        return {
          error: `a chord is one length, and the length goes in front of ` +
            `it: 4 ${token}`,
        };
      }
      return { error: `${piece} is not a note, so it cannot be in a chord` };
    }
    pitches.push(note.pitch);
  }
  return { pitches };
}

// parseTyping turns a typed line into the things to do, in order.
//
// It reads the whole line before anything is written, so a line with a
// mistake in it writes nothing: half a phrase, with the editor stopped
// somewhere in the middle of it, is worse than being told what the mistake
// was.
function parseTyping(line) {
  const out = [];
  let dur = null, dots = 0;

  // Each word, and then each word broken into its pieces where it is a run
  // of notes written without spaces. A word that names a mark or a command
  // is left whole: "tie" is a word and not T, I and the note E.
  //
  // Which word each piece came out of is kept with it. The spacing of a
  // typed line is the only phrasing it can carry, so it is read as one:
  // notes written together are beamed together and a space breaks the
  // beam. See applyTypedBeams.
  const words = [];
  let wordNo = 0;
  for (const raw of String(line).trim().split(/\s+/)) {
    if (!raw) continue;
    wordNo++;
    const push = (t) => words.push({ t, g: wordNo });
    const word = raw.toLowerCase();
    if (typeMarks[word] || typeNote(raw) || /^[+\u2212-]/.test(raw) ||
        /^\d+\.*$/.test(raw) || typeBars[raw] !== undefined) {
      push(raw);
      continue;
    }
    const run = splitRun(raw);
    if (run) {
      run.forEach(push);
      continue;
    }
    push(raw);
  }

  for (const { t: token, g: group } of words) {

    if (typeBars[token] !== undefined) {
      out.push({ kind: "bar", barline: typeBars[token] });
      continue;
    }
    // A length, with any dots on it. Sticky: it stands until another is
    // given, the way holding a note value does.
    let m = /^(\d+)(\.*)$/.exec(token);
    if (m) {
      const name = typeLengths[Number(m[1])];
      if (!name) {
        return { error: `${m[1]} is not a note length — try 1, 2, 4, 8, 16 or 32` };
      }
      dur = name;
      dots = m[2].length;
      continue;
    }
    // An interval from the note before. Signed, always: a bare number is a
    // length, and a melody said out loud is "up a third" rather than "three".
    m = /^([+−-])(\d+)$/.exec(token);
    if (m) {
      const n = Number(m[2]);
      if (n < 1 || n > 24) {
        return { error: `${token} is further than an interval goes` };
      }
      out.push({ kind: "note", interval: (m[1] === "+" ? n : -n), dur, dots, group });
      continue;
    }
    if (/^[rR]$/.test(token)) {
      out.push({ kind: "rest", dur, dots });
      continue;
    }
    // A chord. Anything with a bracket in it is meant as one, so a bracket
    // out of place is answered as the chord it was trying to be rather than
    // falling through to the end of this and being read as a command.
    if (token.includes("[") || token.includes("]")) {
      const chord = typeChord(token);
      if (chord.error) return { error: chord.error };
      out.push({ kind: "chord", pitches: chord.pitches, dur, dots, group });
      continue;
    }
    const note = typeNote(token);
    if (note) {
      out.push({ kind: "note", pitch: note.pitch, dur, dots, group });
      continue;
    }
    // A note with an impossible octave is a length somebody meant to
    // change in the middle of a run, where there is no room to say it.
    if (/^[a-gA-G][#sbxnSBXN]*\d\d/.test(token)) {
      return {
        error: `${token} is not a note — a length inside a run of notes ` +
          `needs a space in front of it`,
      };
    }
    const word = token.toLowerCase();
    if (typeMarks[word]) {
      out.push({ kind: "mark", mark: word });
      continue;
    }
    // Not music and not a mark, so it is a command — which is the other
    // half of what NoteWriter's command line was for, and how anything
    // without a key of its own is reached.
    out.push({ kind: "command", name: word });
  }
  if (out.length > typeBudget) {
    return { error: `that is ${out.length} things at once; ${typeBudget} is the most` };
  }
  return { steps: out };
}

// runTyping writes what a line says, as one thing to undo.
//
// One thing, because a phrase somebody typed is one thought: taking back
// "4 c d e f" should take back the phrase and not the f. The editor already
// has the bracket for that, and the console will use the same one.
// abcPrefix is the word that hands the rest of the line to ABC.
//
// A word rather than a punctuation mark, because the line already uses most
// of the punctuation for music and "abc" is what the notation is called. It
// cannot be mistaken for the music that follows it either: the editor's own
// shorthand has no bare word "abc" in it — a run of those three notes is
// written with a length in front, "8abc", and that is not this.
const abcPrefix = /^abc\s+/i;

// runABC writes a line of ABC notation in at the cursor.
//
// The reading is done by the same parser that opens an .abc file, so a
// phrase types the way a file reads — chords, ties, slurs, broken rhythms
// and all — and there is no second, poorer ABC in the editor to keep in
// step with the first.
async function runABC(text) {
  const sel = state.selection;
  if (!sel) {
    status("Put the cursor in a bar first");
    return false;
  }
  const p = new URLSearchParams({
    part: String(sel.part),
    measure: String(sel.measure),
    at: String(sel.at),
  });
  // One thing to undo, like any other typed line: a phrase somebody typed
  // is one thought.
  await fetch("/api/script/begin", { method: "POST" });
  let res = null;
  try {
    res = await fetch("/api/abc?" + p.toString(), {
      method: "POST",
      headers: { "Content-Type": "text/plain" },
      body: text,
    });
  } finally {
    await fetch("/api/script/end?changed=" + (res && res.ok ? "1" : "0"),
                { method: "POST" });
  }
  if (!res || !res.ok) {
    status(res ? (await res.text()).trim() : "could not write that");
    return false;
  }
  const d = await res.json().catch(() => null);
  if (d) {
    // The cursor walks on to the end of what was written, so a second line
    // carries on from the first.
    state.selection = {
      ...sel, measure: d.measure, at: d.at, ref: null, replace: false,
    };
  }
  await refreshScore();
  markSelection();
  const n = d ? d.wrote : 0;
  const trouble = d && d.trouble ? ` (${d.trouble})` : "";
  status(`Wrote ${n} thing${n === 1 ? "" : "s"} from ABC${trouble}`);
  return true;
}

// fromScript is true when the line came from the console rather than from
// somebody typing it, which changes one thing: a bare word in a typed line
// is a command, and a script may not run every command a person may.
async function runTyping(line, fromScript) {
  // "abc" and a space hands the rest of the line to ABC notation. The
  // editor's own shorthand stays what a bare line is read as, because it is
  // the shorter of the two and the one the hint under the box describes.
  const asABC = abcPrefix.exec(String(line));
  if (asABC) {
    return runABC(String(line).slice(asABC[0].length));
  }
  const read = parseTyping(line);
  if (read.error) {
    status(read.error);
    return false;
  }
  if (!read.steps.length) return false;
  const sel = state.selection;
  if (!sel) {
    status("Put the cursor in a bar first");
    return false;
  }

  await fetch("/api/script/begin", { method: "POST" });
  let wrote = 0, failed = "";
  // Which marks are on, and where every note written under them landed.
  // The marks are put on afterwards rather than as each note is written: a
  // tie joins a note to the one after it, and the one after it does not
  // exist yet.
  const on = {};
  const written = [];
  try {
    for (const step of read.steps) {
      if (step.kind === "mark") {
        on[step.mark] = !on[step.mark];
        continue;
      }
      if (step.kind === "command") {
        const cmd = commandNamed(step.name);
        if (!cmd) {
          failed = `there is no ${step.name}`;
          break;
        }
        if (fromScript) {
          const why = scriptRefusal(cmd);
          if (why) {
            failed = why;
            break;
          }
        }
        await cmd.run();
        wrote++;
        continue;
      }
      if (step.kind === "bar") {
        // The barline belongs to the bar being finished, so it is written
        // before the cursor moves on.
        if (step.barline) {
          const why = await typeBarline(step.barline);
          if (why) {
            failed = why;
            break;
          }
          wrote++;
        }
        await nextBarForTyping();
        continue;
      }
      const where = await typeOne(step);
      if (where.error) {
        failed = where.error;
        break;
      }
      if (step.kind !== "rest") {
        written.push({
          at: where.at,
          marks: Object.keys(on).filter((k) => on[k]),
          // Which word it was typed in, and whether it is a length that
          // has a flag to join to the next one.
          group: step.group,
          beamed: beamedLengths.has(step.dur || state.tool.duration),
        });
      }
      wrote++;
    }
    if (!failed) {
      const why = await applyTypedMarks(written);
      if (why) failed = why;
    }
    if (!failed) {
      const why = await applyTypedBeams(written);
      if (why) failed = why;
    }
  } finally {
    // Always closed, however it went. A batch left open would swallow
    // every edit after it into one undo that nobody asked for.
    await fetch("/api/script/end?changed=" + (wrote ? "1" : "0"),
                { method: "POST" });
  }

  await refreshScore();
  markSelection();
  if (failed) {
    status(wrote ? `Wrote ${wrote}, then stopped: ${failed}` : failed);
    return false;
  }
  status(`Wrote ${wrote} thing${wrote === 1 ? "" : "s"}`);
  return true;
}

// typeOne writes a single note, rest or chord where the cursor is, and says
// what went wrong if it could not.
async function typeOne(step) {
  const sel = state.selection;
  if (!sel) return { error: "the cursor is nowhere" };
  const p = new URLSearchParams({
    part: String(sel.part),
    measure: String(sel.measure),
    at: String(sel.at),
    kind: step.kind === "rest" ? "rest" : "note",
  });
  if (step.dur) p.set("dur", step.dur);
  else p.set("dur", state.tool.duration);
  if (step.dots) p.set("dots", String(step.dots));
  // A chord goes in as its bottom notehead, and the rest of them are added
  // to it: that is how the editor builds a chord, and the mouse does the
  // same thing by clicking in the column a second time.
  const pitch = step.kind === "chord" ? step.pitches[0] : step.pitch;
  if (pitch) p.set("pitch", pitch);
  if (step.drum) p.set("drum", String(step.drum));
  if (step.interval !== undefined) p.set("interval", String(step.interval));
  // A note with no pitch and no interval is one typed as a bare length,
  // which means "where the cursor is pointing" — the same as pressing the
  // key for it.
  if (!pitch && step.interval === undefined && step.kind !== "rest") {
    p.set("step", String(sel.step ?? 0));
  }

  const res = await fetch("/api/insert?" + p.toString(), { method: "POST" });
  if (!res.ok) return { error: (await res.text()).trim() };
  const d = await res.json().catch(() => null);
  // The cursor walks on, so the next token lands after this one — which is
  // what makes a line of them a phrase rather than a pile.
  //
  // Where it actually landed, not where it was aimed: a bar that is full
  // sends what is written next into the bar after it, and a line typed into
  // a nearly full bar has to carry on in the next one rather than keep
  // trying to write into this one. The answer already gives the place after
  // the element, so it is taken as it is.
  if (d && d.measure !== undefined) {
    state.selection = {
      ...sel, measure: d.measure, at: d.at, ref: null, replace: false,
    };
    // Where the thing itself is, which is one before where the cursor now
    // stands: the answer gives the place after it.
    const at = `${sel.part}:${d.measure}:${d.at - 1}`;
    if (step.kind === "chord") {
      const why = await typeNoteheads(sel.part, d.measure, d.at - 1,
                                      step.pitches.slice(1));
      if (why) return { error: why };
    }
    return { at };
  }
  state.selection = { ...sel, at: sel.at + 1, ref: null, replace: false };
  return { at: `${sel.part}:${sel.measure}:${sel.at - 1}` };
}

// typeNoteheads adds the rest of a chord's noteheads to the note just
// written, and says what went wrong if it could not.
//
// Each one is stacked just above the one before it, because a chord is read
// and written from the bottom up: "[ca]" is a sixth, and the nearest A to a
// C is the one below it. An octave written out is taken as it is written,
// so a chord wider than an octave is "[c4g4e5]".
async function typeNoteheads(part, measure, index, pitches) {
  for (const pitch of pitches) {
    const p = new URLSearchParams({
      part: String(part),
      measure: String(measure),
      index: String(index),
      pitch,
      above: "1",
    });
    const res = await fetch("/api/addpitch?" + p.toString(), { method: "POST" });
    if (!res.ok) return (await res.text()).trim();
  }
  return "";
}

// applyTypedMarks puts the sticky marks on the notes that were written
// under them.
//
// Afterwards, because a tie joins a note to the one after it and a slur
// goes from one note to another: neither can be said until both ends are
// on the page. A run of notes under one mark is one phrase, so a tie
// stops at the last of them — a tie with nothing after it is half a mark
// — and a slur arcs from the first to the last rather than note by note.
async function applyTypedMarks(written) {
  for (const mark of Object.keys(typeMarks)) {
    const what = typeMarks[mark];
    // Every unbroken run of notes carrying this mark.
    let i = 0;
    while (i < written.length) {
      if (!written[i].marks.includes(mark)) {
        i++;
        continue;
      }
      let j = i;
      while (j + 1 < written.length && written[j + 1].marks.includes(mark)) j++;
      const run = written.slice(i, j + 1).map((n) => n.at);
      i = j + 1;

      if (what.kind === "articulation") {
        const p = new URLSearchParams({
          articulation: what.name, sel: run.join(","),
        });
        const res = await fetch("/api/articulation?" + p, { method: "POST" });
        if (!res.ok) return (await res.text()).trim();
        continue;
      }
      if (run.length < 2) continue; // nothing to join to
      if (what.kind === "slur") {
        const res = await fetch(
          "/api/slur?slur=yes&sel=" + encodeURIComponent(run.join(",")),
          { method: "POST" });
        if (!res.ok) return (await res.text()).trim();
        continue;
      }
      // A tie is between neighbours, and the endpoint ties each note of a
      // selection to the one after it — so the last is left out, because
      // what it would tie to is outside the phrase.
      const res = await fetch(
        "/api/tie?tie=yes&sel=" + encodeURIComponent(run.join(",")),
        { method: "POST" });
      if (!res.ok) return (await res.text()).trim();
    }
  }
  return "";
}

// beamedLengths are the lengths that have a flag to join to the next note.
// A quarter note has none, so nothing a typed line says about it is a
// statement about beams.
const beamedLengths = new Set(["eighth", "sixteenth", "32nd", "64th"]);

// applyTypedBeams beams each run of notes written without spaces between
// them, and breaks the beam wherever there was a space.
//
// "32 abcd bcde" is two groups of four. The engraver cannot know that:
// left to itself it groups by beat, and eight thirty-seconds are one beat,
// so it writes them as one beam of eight. The spacing of a typed line is
// the only phrasing mark it has, and this is what reads it.
//
// A note standing alone between spaces is taken off the beam rather than
// left to the engraver, because the space either side of it is where the
// beam was asked to break. That is the same statement said twice and it
// has to be, or "8 a b c d" would come out as the two beamed pairs the
// beat says, which is the opposite of what was typed.
async function applyTypedBeams(written) {
  let i = 0;
  while (i < written.length) {
    let j = i;
    while (j + 1 < written.length && written[j + 1].group === written[i].group) j++;
    const run = written.slice(i, j + 1).filter((n) => n.beamed);
    i = j + 1;
    if (!run.length) continue;
    const p = new URLSearchParams({
      beam: run.length > 1 ? "join" : "none",
      sel: run.map((n) => n.at).join(","),
    });
    const res = await fetch("/api/beam?" + p.toString(), { method: "POST" });
    if (!res.ok) return (await res.text()).trim();
  }
  return "";
}

// typeBarline ends the bar the cursor is in with the barline named, and says
// what went wrong if it could not.
//
// Its own request rather than setBarline's, because that one redraws and
// talks to the status line: a typed line is one thought, drawn once when
// the whole of it has been written.
async function typeBarline(style) {
  const sel = state.selection;
  if (!sel) return "the cursor is nowhere";
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), style,
  });
  const res = await fetch("/api/barline?" + p.toString(), { method: "POST" });
  if (!res.ok) return (await res.text()).trim();
  return "";
}

// nextBarForTyping moves the cursor to the start of the next bar, making
// one where the music has run out.
async function nextBarForTyping() {
  const sel = state.selection;
  if (!sel) return;
  const last = measureCount() - 1;
  if (sel.measure >= last) {
    await fetch("/api/addmeasure", { method: "POST" });
    await loadHits();
  }
  state.selection = { ...sel, measure: sel.measure + 1, at: 0, ref: null, replace: false };
}

// commandNamed finds a command by what somebody typed: its id, or the start
// of what it is called. "tie" finds the tie; "cresc" finds the crescendo.
//
// One match or none. Two commands starting with the same letters is an
// ambiguous thing to have typed, and guessing which was meant is how a line
// of music quietly turns into a key change.
// WORD_COMMANDS are commands with no key: things you run by typing their
// name, in the typing bar or in the console.
//
// They are not in KEY_BINDINGS because everything there answers to a
// keystroke — the table is checked for it, no two of them may share a key,
// and a menu entry for one has to show the key beside it. Keys are scarce
// and these three do not need one; being nameable is the whole of what
// they need.
const WORD_COMMANDS = [
  { id: "transpose", label: "Transpose", run: () => action("transpose") },
  { id: "transpose-up", label: "Transpose up a semitone",
    run: () => void transposeBy(1, 1) },
  { id: "transpose-down", label: "Transpose down a semitone",
    run: () => void transposeBy(-1, -1) },
  // The typing help offers "tempo" as its example of naming a command by
  // the start of it, and until this was here that example named nothing:
  // the speed is set from a dialog and the dialog had no command.
  { id: "set-tempo", label: "Set tempo", run: () => action("set-tempo") },
  { id: "select-region", label: "Pick up a stretch of bars",
    run: () => action("select-region") },
];

/*
 * Which of them a script may run.
 *
 * run("tie") is most of what a plug-in in any other program does: the
 * editing is already written and named, and a script that could not reach
 * it would have to ask the server for everything again in its own words.
 * So the commands are open to a script — but not all of them, and the
 * line is not where it first looks.
 *
 * What is kept back is not what is dangerous to the music; it is what is
 * not the script's to decide. The file commands are the ones a script has
 * no business having: a script that can save is a script that can write
 * over your work from a score somebody sent you, and that is exactly the
 * door this console does not have. Undo and redo are the person's hands
 * on the last thing that happened, and a script runs as one undo —
 * undoing from inside it undoes the bar before and then goes on writing
 * on top of it. Playing, zooming and the typing bar are the editor rather
 * than the score; a script that left the window somewhere else has done
 * something nobody asked it for.
 *
 * Named by the group they are listed under rather than one by one, so
 * that a command added to the file menu tomorrow is refused by having
 * been added there, and not by somebody remembering to come back here.
 */
const SCRIPT_GROUPS = new Set([
  "Writing", "Note lengths", "Accidentals", "Marks", "Tuplets",
  "Selecting and moving",
]);

// The few inside those groups that are still not a script's: two modes,
// which a script would leave switched on behind it, and the go-to-bar
// window, which is at(bar) said with a dialog in the way.
const SCRIPT_NOT = new Set(["mouse-writes", "type-music", "jump", "jump-mod"]);

// And the ones with no group, which are the commands that answer only to
// their name. Listed rather than allowed wholesale: the others open a
// window and wait, which is not something to do in the middle of a queue.
const SCRIPT_ALSO = new Set(["transpose-up", "transpose-down"]);

function scriptMayRun(cmd) {
  if (!cmd || SCRIPT_NOT.has(cmd.id)) return false;
  return SCRIPT_GROUPS.has(cmd.group || "") || SCRIPT_ALSO.has(cmd.id);
}

// scriptRefusal is what the console says about a command it will not run.
function scriptRefusal(cmd) {
  if (scriptMayRun(cmd)) return "";
  return `${cmd.id} is not a script's to run — a script writes music; ` +
    "the file, undo, playing and the window stay yours";
}

function commandNamed(name) {
  const all = KEY_BINDINGS.concat(WORD_COMMANDS);
  const exact = all.find((c) => c.id === name);
  if (exact) return exact;
  const starts = all.filter((c) =>
    c.id.startsWith(name) || (c.label || "").toLowerCase().startsWith(name));
  if (starts.length === 1) return starts[0];
  const holds = all.filter((c) => (c.label || "").toLowerCase().includes(name));
  return holds.length === 1 ? holds[0] : null;
}

// state.typing is whether the bar is up. It is a mode, like writing with
// the mouse: off until somebody asks for it, and remembered while the score
// is open rather than between sessions, because it is about what you are
// doing this minute.
state.typing = false;

function toggleTypeBar() {
  setTypeBar(!state.typing);
}

function setTypeBar(on) {
  state.typing = !!on && !state.readOnly;
  const tick = $("#type-tick");
  if (tick) tick.textContent = state.typing ? "✓" : "";
  const line = $("#type-line");
  if (line && !state.typing) line.value = "";
  placeTypeBar();
  if (state.typing && line) line.focus();
  status(state.typing
    ? "Type the music: " + typeHint
    : "Typing off");
}

// placeTypeBar shows or hides the line, and says where it is writing.
//
// It is docked under the music rather than standing at the cursor. Standing
// at the cursor put it squarely over the bar being written, which is the one
// thing you have to see while writing it — and it only appeared at all when
// the cursor's bar happened to be among the regions drawn, so closing it and
// opening it again in a bar past the end of the music showed nothing.
//
// What is left of following the cursor is the part that was worth having:
// it says which staff and which bar the next thing typed will go into, so
// the connection is in words instead of in pixels.
function placeTypeBar() {
  const bar = $("#typebar");
  if (!bar) return;
  bar.hidden = !state.typing;
  if (!state.typing) return;

  const hint = $("#type-hint");
  const line = $("#type-line");
  if (hint) {
    hint.textContent = typeHint;
    // Only while there is nothing typed: it is there to be read once, and
    // after that it is in the way of the thing being written.
    hint.hidden = !!(line && line.value);
  }

  const where = $("#type-where");
  if (!where) return;
  const sel = state.selection;
  if (!sel) {
    where.textContent = "Click a bar first";
    return;
  }
  const name = (state.info?.parts || [])[sel.part]?.name || `Staff ${sel.part + 1}`;
  where.textContent = `${name} · bar ${barNumberOf(sel.measure)}`;
}

// setupTypeBar wires the line: Enter writes it, Escape puts it away, and
// everything else is left to the box so that typing a note does not also
// press the key that note is bound to.
function setupTypeBar() {
  const line = $("#type-line");
  if (!line) return;
  line.addEventListener("keydown", async (e) => {
    e.stopPropagation();
    if (e.key === "Escape") {
      e.preventDefault();
      setTypeBar(false);
      $("#canvas-wrap")?.focus?.();
      return;
    }
    if (e.key !== "Enter") return;
    e.preventDefault();
    const what = line.value;
    if (!what.trim()) {
      setTypeBar(false);
      return;
    }
    if (await runTyping(what)) {
      // Cleared on success and left alone on failure, so a line with a
      // mistake in it can be corrected rather than typed again.
      line.value = "";
    }
    placeTypeBar();
    line.focus();
  });
  // The hint is there to be read once; once there is something in the box
  // it is in the way of the thing being written.
  line.addEventListener("input", () => {
    const hint = $("#type-hint");
    if (hint) hint.hidden = !!line.value;
  });
  // Typing in the box is not typing at the score.
  line.addEventListener("keypress", (e) => e.stopPropagation());
  line.addEventListener("keyup", (e) => e.stopPropagation());
}

/* ---------- what the typing line takes ---------- */

// The reference for it, built from the same tables the parser reads.
//
// Built rather than written out, because a reference kept by hand is wrong
// by the second change — and a wrong reference is worse than none, since it
// is believed. The lengths and the marks here are the very objects the
// parser looks things up in, so a length added to the program appears here
// without anybody remembering to add it.

function openTypingHelp() {
  const dlg = $("#typing-help");
  if (!dlg) return;
  drawTypingHelp();
  if (!dlg.open) dlg.showModal();
}

// syntaxRows renders one group of examples.
function syntaxRows(rows) {
  return `<dl class="syntax">` + rows.map(([what, means]) =>
    `<dt>${escapeHTML(what)}</dt><dd>${means}</dd>`).join("") + `</dl>`;
}

function drawTypingHelp() {
  const body = $("#typing-help-body");
  if (!body) return;

  // The lengths, in the order they run, from the table the parser uses.
  const lengths = Object.keys(typeLengths)
    .map(Number).sort((a, b) => a - b)
    .map((n) => [String(n), typeLengths[n]]);

  // The marks, each listed once however many words reach it.
  const seen = new Map();
  for (const word of Object.keys(typeMarks)) {
    const m = typeMarks[word];
    const key = m.kind + (m.name || "");
    if (!seen.has(key)) seen.set(key, { words: [], what: m });
    seen.get(key).words.push(word);
  }
  const marks = [...seen.values()].map(({ words, what }) => [
    words.join(" or "),
    what.kind === "tie" ? "ties each note to the next one"
      : what.kind === "slur" ? "one slur from the first note to the last"
      : escapeHTML(what.name) + " on every note",
  ]);

  body.innerHTML =
    `<h3>a line</h3>` +
    syntaxRows([
      ["8abcdef", "six eighth notes, climbing — this is the whole of it"],
      ["4cde 8fg", "quarters <em>c d e</em>, then eighths <em>f g</em>"],
      ["tie 8abcdef", "the same six, tied together"],
    ]) +
    `<h3>how long</h3>` +
    syntaxRows(lengths.map(([n, name]) => [n, `a ${escapeHTML(name)} note`])
      .concat([["4.", "dotted — as many dots as you write"]])) +
    `<p class="syntax-rule">A length stays until you give another, the way
       holding a note value does. Inside a run of notes it needs a space in
       front of it: the digits straight after a note are its octave.</p>` +
    `<h3>which note</h3>` +
    syntaxRows([
      ["c", "the C nearest the note before it"],
      ["c4", "middle C — the octave is scientific"],
      ["f#3", "F sharp, and <em>s</em> works for the sharp as well"],
      ["a_", "A flat, inside a run"],
      ["bb4", "B flat, where the note stands on its own"],
      ["fn", "F natural, whatever the key signature says"],
      ["[ceg]", "a chord — its notes from the bottom up"],
      ["[c4g4e5]", "a chord wider than an octave, written out"],
      ["+3", "a third above the note before"],
      ["−2", "a step below it"],
      ["r", "a rest, as long as the length in force"],
      ["|", "the next bar, making one where the music has run out"],
    ]) +
    `<p class="syntax-rule">In a run <em>b</em> is the note B — it has to be,
       or <em>8abcdef</em> would start on an A flat. So a flat inside a run
       is written <em>_</em>. A note standing on its own as a whole word
       still takes <em>b</em>, because <em>bb4</em> is how everybody writes
       B flat.</p>` +
    `<p class="syntax-rule">A chord is read from the bottom up, each notehead
       the nearest one above the last: <em>[ca]</em> is a sixth, not an A
       under the C. Its length goes in front of the brackets — a chord is one
       sound and a sound has one length — and inside them a flat is
       <em>_</em>, as it is in a run.</p>` +
    `<h3>marks</h3>` +
    syntaxRows(marks) +
    `<p class="syntax-rule">A mark goes on everything typed after it, and the
       same word again turns it off. They are put on once the line is
       written, because a tie joins a note to the one after it and that note
       does not exist yet.</p>` +
    `<h3>anything else</h3>` +
    syntaxRows([
      ["transpose-up", "a word that is not music runs the command of that name"],
      ["tempo", "the start of a name is enough, where only one command starts that way"],
    ]) +
    `<p class="syntax-rule">A line is read whole before anything is written,
       so a line with a mistake in it writes nothing and says what was wrong
       — with your line still in the box to correct. Everything one line
       writes is taken back by one press of undo.</p>`;
}

/* ---------- the script console ---------- */

// A place to write a program that writes music.
//
// A piece built from a process — a row, an isorhythm, a phase, a canon at
// the fifth — is a piece nobody wants to type out twice. The process is the
// thought; the notes are its output. So this is a console, and what it
// gives you is a dozen words and JavaScript's own loops and conditions: the
// power is in the language, which everybody already has, and not in a music
// language nobody has ever written in.
//
// Four things make it a tool rather than a liability, and each of them is
// here on purpose.
//
//   - The verbs are few. Eight of them and one that takes a line of the
//     typing language, which is how anything not worth a verb is reached.
//   - Everything one script writes is one undo. A script of two hundred
//     notes that came back two hundred presses at a time is a script
//     nobody would dare run.
//   - There is a budget. A loop that writes for ever stops, and says so.
//   - Nothing in a file ever runs by itself. A score carries its scripts
//     and opening it shows them, unrun. A document that executes code on
//     being opened is a document that can attack whoever opens it, which
//     is the mistake Word made and spent a decade undoing.

// consoleBudget is how much one script may write, and how long it may take
// about it.
//
// The count catches a loop that writes for ever; the clock catches one that
// writes slowly for ever. Neither catches a loop that calls nothing at all
// — "while (true) {}" will hang the page the same way it would in any
// browser console — and the help says so rather than pretending otherwise.
const consoleBudget = { steps: 4000, ms: 3000 };

// consoleVerbs builds the words a script is given.
//
// They are synchronous and they queue: calling note() writes nothing, it
// remembers that a note was asked for, and the whole queue is sent in order
// once the script has finished. That is what lets a script read as music
// rather than as plumbing —
//
//     for (const p of row) note(p, 8);
//
// — with no await anywhere to forget. The cost is that what a script reads
// is the score as it stood when the script began, which is said plainly in
// the help.
// given is everything else that was read before the script ran: what the
// person had picked up, and what the server says a chord is made of. Both
// are read once for the same reason the music is — the verbs are
// synchronous, and a synchronous verb cannot go and ask.
// drawings is where draw() puts its pictures, beside said: both are what the
// script has to show when it has finished, and they are shown together.
function consoleVerbs(out, said, read, question, given, drawings = []) {
  const picked = given && given.picked ? given.picked : null;
  const chords = given && given.chords ? given.chords : null;
  const started = Date.now();
  // How many staves the script has asked for so far, so that staff() can
  // say which number the next one will be without waiting to find out.
  let made = 0;
  // How long a question has been on the screen.
  //
  // The clock is on what the script does, not on how long somebody takes
  // to answer it. Without this, a script that asks two questions is stopped
  // for running too long while the person it asked is still reading the
  // first one — which is the one script in the editor that cannot possibly
  // be in a loop, because it is waiting for them.
  let waited = 0;
  // When the progress bar was last drawn, so that a script calling for it
  // every bar of a long score hands back a frame a few times a second
  // rather than a few thousand times.
  let drawn = 0;
  // A score that has just been made is one staff with nothing on it. A
  // script writing for four instruments should finish with four staves and
  // not five, so the first one it asks for moves into the staff that is
  // already there rather than standing beside it. Only a brand new score:
  // an empty staff somebody put in the middle of their own music is a staff
  // they meant to have.
  const spare = (read.staves || []).length === 1 &&
    ((read.staves[0] || {}).bars || []).every((b) => b.every((n) => n.rest));
  const spend = () => {
    if (out.length >= consoleBudget.steps) {
      throw new Error(`a script may write ${consoleBudget.steps} things; ` +
        `this one is still going`);
    }
    if (Date.now() - started - waited > consoleBudget.ms) {
      throw new Error("this script has been running too long and was stopped");
    }
  };
  const lengthOf = (len) => {
    if (len === undefined || len === null) return undefined;
    const name = typeLengths[Number(len)];
    if (!name) throw new Error(`${len} is not a note length`);
    return name;
  };

  return {
    // write takes a line of the typing language, which is how everything
    // the verbs do not cover is reached: marks, barlines, commands.
    write(line) {
      spend();
      out.push({ do: "line", line: String(line) });
    },
    // run does one of the editor's own commands, by the name the
    // shortcuts window lists it under: run("tie"), run("beam"),
    // run("artic-staccato"). The start of what it is called will do, the
    // same way the typing bar takes it.
    //
    // This is most of what a plug-in elsewhere is made of. Everything the
    // editor can do to the music is already written and named, and a
    // script that could not reach it would be reduced to asking the
    // server for each of them again in its own words.
    //
    // Which commands is decided where the commands are, not here: the
    // file, undo, playing and the window are not a script's to run, and
    // the reason is written beside the rule.
    run(name) {
      spend();
      const want = String(name).trim().toLowerCase();
      if (!want) throw new Error("run needs the name of a command");
      // Looked up now rather than when the queue is written, so that a
      // misspelled command is an error where it was written and nothing
      // has been done to the score yet.
      const cmd = commandNamed(want);
      if (!cmd) throw new Error(`there is no command called ${want}`);
      const why = scriptRefusal(cmd);
      if (why) throw new Error(why);
      out.push({ do: "run", name: cmd.id });
    },
    note(pitch, len) {
      spend();
      out.push({ do: "note", pitch: String(pitch), dur: lengthOf(len) });
    },
    rest(len) {
      spend();
      out.push({ do: "rest", dur: lengthOf(len) });
    },
    chord(pitches, len) {
      spend();
      const all = [].concat(pitches).map(String);
      if (!all.length) throw new Error("a chord needs notes in it");
      out.push({ do: "chord", pitches: all, dur: lengthOf(len) });
    },
    // measure reads what is written on a bar rather than in it: the clef,
    // the key, the meter, the rehearsal mark, the tempo, the barline
    // that closes it.
    //
    // Separate from notes() because they are separate questions. "Which
    // bars have no dynamic" walks the notes; "which bars are marked a
    // tempo" does not look at a note at all.
    // Called measure and not bar because bar() already writes one. The
    // two words mean the same thing and the score model uses this one.
    measure(n, staff) {
      const staves = read.staves || [];
      const si = staff === undefined
        ? (state.selection?.part ?? 0)
        : Math.max(0, Math.trunc(Number(staff)) - 1);
      const bi = Math.max(0, Math.trunc(Number(n)) - 1);
      const got = (staves[si]?.measures || [])[bi];
      if (!got) return null;
      // A copy, so a script may do as it likes with what it is given.
      return { ...got, notes: got.notes.map((x) => ({ ...x })) };
    },
    // draw puts a picture in what the script has to show, under what it
    // has said. A drawing and a caption: draw(svg, "Bar 12").
    //
    // Because a great deal of what a script finds out is not a sentence. A
    // flute fingering is a row of circles, some of them filled in; a chart
    // of where the parallel fifths are is a row of bars; a key map is a
    // colour. Said in words — "Th 1 2 3 | 1 2 3 D#" — it is something to
    // decode rather than something to read, and a player reading it at the
    // stand has no time for that.
    //
    // SVG, because that is what the rest of the editor draws in and what
    // the window can show at any size. The markup goes in as the script
    // wrote it: a script is already running in this page and could reach
    // the window anyway, so what is kept here is a vocabulary, not a cage
    // — the promise gscore makes is that nothing runs until you have read
    // it, and that is a promise about running, not about drawing.
    draw(picture, caption) {
      spend();
      const svg = String(picture);
      if (!svg.trim()) throw new Error("draw needs something to draw");
      if (drawings.length >= 200) {
        throw new Error("a script may draw 200 pictures; this one is still going");
      }
      drawings.push({
        svg,
        caption: caption === undefined ? "" : String(caption),
      });
    },
    // progress says how far the script has got, as a bar in the console's
    // own window.
    //
    // await progress(i, n) — because of what the await is for. A script
    // runs on the same thread that draws the window, so a loop over two
    // and a half thousand bars holds the screen exactly as it was until it
    // finishes: a progress bar written without yielding is a progress bar
    // nobody ever sees move. The await hands the frame back, the bar is
    // drawn, and the script carries on.
    //
    // Called without the await it still works and still shows nothing
    // until the script next waits for something, which is honest enough:
    // it is the yielding that makes it visible, not the telling.
    //
    // The reading time is not the script's, the same way a question's is
    // not: a script stopped for having spent its three seconds drawing the
    // bar it was asked to draw would be a trap rather than a budget. What
    // is not given away is the counting — a frame is handed back at most
    // every fiftieth of a second, so a script calling this in a tight loop
    // is still a script on the clock.
    progress(done, total) {
      // Synchronously, before any promise: a script that calls this
      // without awaiting it should still be stopped by the budget, and a
      // throw inside a promise nobody waits for stops nothing.
      spend();
      const n = Math.max(0, Math.trunc(Number(done)) || 0);
      const all = total === undefined || total === null
        ? 0 : Math.max(0, Math.trunc(Number(total)) || 0);
      const now = Date.now();
      if (now - drawn < 50) return Promise.resolve();
      drawn = now;
      if (typeof showConsoleProgress === "function") {
        showConsoleProgress(n, all);
      }
      return new Promise((go) => {
        const back = () => {
          waited += Date.now() - now;
          go();
        };
        if (typeof requestAnimationFrame === "function") {
          requestAnimationFrame(back);
        } else {
          setTimeout(back, 0);
        }
      });
    },
    // selection is what the person picked up before pressing Run.
    //
    // The commonest shape a script has anywhere is "do this to what I have
    // selected", and without this a script can only be told about the
    // whole piece. It is what makes run() worth having too: a command acts
    // on the selection, so a script that walks a passage has to know where
    // the passage is.
    //
    // Nothing picked up gives back nothing, which is a script's cue to say
    // so rather than quietly going over the whole score.
    //
    // As it was when the script began, for the same reason the music is:
    // the verbs queue, and the cursor moves as the queue is written. A
    // script that wants to put it back where it found it says
    // at(was.bar, was.staff, was.at).
    selection() {
      if (!picked) return null;
      return {
        ...picked,
        from: picked.from ? { ...picked.from } : null,
        to: picked.to ? { ...picked.to } : null,
      };
    },
    // chordNotes says what a chord symbol is made of: chordNotes("Cmaj7")
    // is ["cn4", "en4", "gn4", "bn4"], ready to hand to chord().
    //
    // This is what makes a lead sheet playable from a script. Every verb
    // here could already write a chord; none of them could say which notes
    // "Bbm7b5" is, and working that out in the script means a table of
    // chord spellings in every script anybody writes.
    //
    // The second number is the octave the root goes in, four by default —
    // middle C's. Notes above the root climb out of it as they must, so
    // the ninth of a chord is in the octave above whatever its root is in.
    //
    // Spelled out, accidental or natural on every note, because the staff
    // it is going on may be in a key the script was not thinking in.
    // Nothing is guessed: a quality gscore does not know gives back
    // nothing rather than the triad somebody probably meant.
    chordNotes(symbol, octave) {
      if (!chords) {
        throw new Error("the chords could not be read from the score");
      }
      return chordNotesOf(symbol, octave, chords);
    },
    // latinSyllables divides a Latin word the way it is sung:
    // latinSyllables("alleluia") is ["al", "le", "lu", "ia"].
    //
    // Setting a text under music means knowing where the words break, and
    // a Mass is four hundred words to hyphenate before a note of it can be
    // typed. Latin divides by rule and English does not — heav-en and
    // bea-ver are the same shape — so this is Latin, and the name says so
    // rather than a comment somewhere saying so.
    //
    // Nothing is guessed. A word that cannot be Latin at all, and anything
    // that is not a word, comes back as nothing.
    latinSyllables(word) {
      return latinSyllables(word);
    },
    // staves lists what each staff is: its name, its instrument, the clef
    // and key and meter it starts in, and how far its written pitches sit
    // from what is heard. A script checking whether the clarinet goes
    // over its range has to know the staff is a clarinet.
    staves() {
      return (read.staves || []).map((st, i) => ({
        number: i + 1,
        name: st.name,
        instrument: st.instrument || "",
        clef: st.clef || "",
        key: st.key || "",
        time: st.time || "",
        transpose: st.transpose || 0,
        bars: (st.measures || []).length,
      }));
    },
    // step moves from the note before by an interval, which is how a
    // melody is thought about: up a third, down a step.
    step(n, len) {
      spend();
      out.push({ do: "step", interval: Math.trunc(Number(n)), dur: lengthOf(len) });
    },
    bar() {
      spend();
      out.push({ do: "bar" });
    },
    // staff adds a staff and gives back its number, so that at(bar, staff)
    // has something to be pointed at.
    //
    // The number is known before the staff exists, because staves are added
    // at the end in the order they are asked for. That is what lets the verb
    // queue with the rest of them instead of being the second thing in the
    // language a script has to await.
    staff(instrument, opts) {
      spend();
      const o = opts || {};
      made++;
      const take = spare && made === 1;
      out.push({
        do: "staff", take,
        instrument: String(instrument),
        name: o.name === undefined ? "" : String(o.name),
        clef: o.clef === undefined ? "" : String(o.clef),
      });
      return take ? 1 : (read.staves || []).length + made - (spare ? 1 : 0);
    },
    // drum names what to hit on a drum staff, where a line of the staff is
    // an instrument and not a pitch. By name — "snare drum", "hi-hat
    // closed" — or by its MIDI key, for whoever already thinks in those.
    drum(which, len) {
      spend();
      out.push({ do: "drum", which: String(which), dur: lengthOf(len) });
    },
    // key is the signature of the piece, counted in sharps or flats:
    // key(4) is four sharps, key(-3) is three flats.
    //
    // Set once for the piece and not per staff, because a signature is a
    // fact about the music — a part for an instrument in B-flat gets the
    // signature its own player reads for that key, which is a different
    // number, and the score works that out rather than the script.
    key(fifths) {
      spend();
      const n = Math.trunc(Number(fifths));
      if (!(n >= -7 && n <= 7)) {
        throw new Error(`${fifths} is not a key signature — \u22127 to 7`);
      }
      out.push({ do: "key", fifths: n });
    },
    // dynamic marks the note just written. Written after the note rather
    // than given to it, because that is where it goes in the music.
    dynamic(name) {
      spend();
      out.push({ do: "dynamic", name: String(name) });
    },
    // at moves the cursor to a bar, and with a third number to a place
    // inside it — how many things are before the cursor there, counted the
    // way selection() reports it. Without one it goes to the start of the
    // bar, which is where a script that is about to write wants it; with
    // one, at(was.bar, was.staff, was.at) is how a script that moved the
    // cursor puts it back where it found it.
    at(bar, staff, place) {
      spend();
      out.push({
        do: "at",
        bar: Math.max(0, Math.trunc(Number(bar)) - 1),
        staff: staff === undefined ? undefined : Math.max(0, Math.trunc(Number(staff)) - 1),
        place: place === undefined ? undefined : Math.max(0, Math.trunc(Number(place))),
      });
    },
    // notes reads a bar: what is written there, as it stood when the
    // script began.
    //
    // As it stood, because the verbs queue and nothing has been written
    // yet. A script reads the music it was given and writes a new one from
    // it — copy this bar up a fifth, answer it in the bass, invert it —
    // which is what reading is for; a script that wants to read what it
    // has just written is asking the queue a question, and the queue is
    // not a score.
    notes(bar, staff) {
      const staves = read.staves || [];
      const si = staff === undefined
        ? ((state.selection?.part ?? 0))
        : Math.max(0, Math.trunc(Number(staff)) - 1);
      const bi = Math.max(0, Math.trunc(Number(bar)) - 1);
      const got = (staves[si]?.bars || [])[bi] || [];
      // A copy, so that a script may do as it likes with what it is given.
      return got.map((n) => ({ ...n, chord: n.chord ? [...n.chord] : undefined }));
    },
    // transpose moves a note by diatonic steps, which is how a passage is
    // moved: up a fifth is four steps, down an octave is seven.
    //
    // Steps and not semitones, because the key signature spells what comes
    // out — the same way a note written by hand is spelled. A script that
    // wants an exact semitone writes the accidental itself.
    transpose(pitch, steps) {
      return transposeName(String(pitch), Math.trunc(Number(steps)));
    },
    // ask puts a question to whoever is running the script.
    //
    // The one word here that has to be waited for, and the only place a
    // script says await. The writing verbs queue and so stay synchronous,
    // which is what keeps a loop of them readable; a question cannot
    // queue, because the answer is what the rest of the script is about.
    //
    // Answering nothing — closing the question — comes back as null, and a
    // script that carries on regardless is a script that writes something
    // nobody asked for. The examples check it.
    async ask(q, choices, opts) {
      const began = Date.now();
      try {
        const many = !!(opts && opts.many);
        if (Array.isArray(choices) && choices.length) {
          return await question({
            title: String(q), label: (opts && opts.label) || "Choose",
            options: choices, many,
            note: (opts && opts.note) || "",
          });
        }
        return await question({
          title: String(q), label: (opts && opts.label) || "",
          value: choices === undefined || choices === null ? "" : String(choices),
          note: (opts && opts.note) || "",
        });
      } finally {
        waited += Date.now() - began;
      }
    },
    // say prints, which is how a script explains itself to whoever runs it.
    say(...what) {
      said.push(what.map((w) =>
        typeof w === "object" ? JSON.stringify(w) : String(w)).join(" "));
    },
  };
}

// runConsole runs a script and then does what it asked for.
//
// Two halves, and the split is the whole design. The script runs first and
// writes nothing: it fills a queue. Then the queue is sent, in order, inside
// one undo. A script that throws halfway has asked for nothing, so nothing
// has happened, and the error is the only thing that reaches the score.
// showConsoleProgress says how far a script has got, or takes the line away
// when called with nothing.
//
// In the console's own window rather than over the whole editor with
// Gilbert: the window is open and being watched, and a mascot covering the
// script somebody is halfway through reading is the wrong place for the
// news. The bar is the browser's own, so it needs no drawing and reads
// right in both themes.
function showConsoleProgress(done, total) {
  const box = $("#console-progress");
  if (!box) return;
  if (done === undefined || done === null) {
    box.hidden = true;
    return;
  }
  box.hidden = false;
  const bar = $("#console-progress-bar");
  const text = $("#console-progress-text");
  if (total > 0) {
    if (bar) {
      bar.max = total;
      bar.value = Math.min(done, total);
      bar.removeAttribute("data-waiting");
    }
    if (text) text.textContent = `${done} of ${total}`;
    return;
  }
  // No total: a script that knows how far it has got and not how far there
  // is to go. The bar has nothing to show, so it shows nothing.
  if (bar) {
    bar.max = 1;
    bar.value = 0;
    bar.setAttribute("data-waiting", "1");
  }
  if (text) text.textContent = String(done);
}

// pickedSelection says what is picked up, in the numbers a script counts in.
//
// Staves and bars from one, the way the page is numbered and the way every
// other verb takes them. The editor counts from nought inside itself, and a
// script that had to know which of the two it was being handed would get it
// wrong once and never find out: an off-by-one in a bar number is a report
// about the wrong bar, not an error.
//
// from and to are the ends of a stretch, in reading order. A bare cursor is
// not a stretch — there is a place but nothing picked up — so it has none,
// and count says which of the two it is.
function pickedSelection() {
  const sel = state.selection;
  const multi = state.multi || [];
  if (!sel && !multi.length) return null;

  const where = (s) => ({
    staff: s.part + 1, bar: s.measure + 1, index: s.index,
  });
  let from = null, to = null;
  if (multi.length) {
    // In reading order: the list is kept in it, but a selection grown by
    // dragging backwards is put back in order rather than trusted.
    const sorted = multi.slice().sort((a, b) =>
      a.part - b.part || a.measure - b.measure || a.index - b.index);
    from = where(sorted[0]);
    to = where(sorted[sorted.length - 1]);
  }
  return {
    staff: sel ? sel.part + 1 : from.staff,
    bar: sel ? sel.measure + 1 : from.bar,
    // How many things are before the cursor in its bar, which is what at()
    // takes back.
    at: sel ? (sel.at || 0) : 0,
    count: multi.length,
    from, to,
  };
}

async function runConsole(source) {
  const queue = [], said = [], drawings = [];
  // The score as it stands, read once before the script runs. The verbs
  // are synchronous so that a script reads as music rather than as
  // plumbing, and a synchronous verb cannot go and ask.
  let read = { staves: [] };
  try {
    const res = await fetch("/api/contents");
    if (res.ok) read = await res.json();
  } catch (_) {
    // A script that writes without reading still works.
  }
  // What a chord is made of, which the server owns: the one place in the
  // program that says what maj7 means.
  let chords = null;
  try {
    const res = await fetch("/api/chordqualities");
    if (res.ok) chords = (await res.json()).qualities || null;
  } catch (_) {
    // A script that never asks about a chord does not need it, and one
    // that does is told it could not be read rather than given a guess.
  }
  const verbs = consoleVerbs(queue, said, read, ask,
                             { picked: pickedSelection(), chords }, drawings);

  // What the score looked like when the script began, for a script that
  // wants to know. It does not change as the queue is written, which the
  // help says in as many words.
  const look = {
    bars: measureCount(),
    staves: (state.info?.parts || []).length,
    staffNames: (state.info?.parts || []).map((p) => p.name || ""),
  };

  try {
    // Run as an async function, so that a script may await a question or
    // the drawing of its progress. The writing verbs stay synchronous and
    // queue, so there is no await to forget in a loop that writes.
    const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
    const run = new AsyncFunction(...Object.keys(verbs), "score", "Math", "JSON",
      `"use strict";\n${source}\n`);
    await run(...Object.values(verbs), look, Math, JSON);
  } catch (e) {
    return { said, drawings, error: String(e && e.message ? e.message : e) };
  } finally {
    // Whatever the script showed about how far it had got, it has got
    // there. A bar left at four fifths after the script has stopped is a
    // script that looks as though it is still going.
    showConsoleProgress(null);
  }
  if (!queue.length) {
    return { said, drawings, wrote: 0 };
  }

  await fetch("/api/script/begin", { method: "POST" });
  let wrote = 0, error = "";
  try {
    for (const step of queue) {
      const why = await doConsoleStep(step);
      if (why) {
        error = why;
        break;
      }
      wrote++;
    }
  } finally {
    await fetch("/api/script/end?changed=" + (wrote ? "1" : "0"),
                { method: "POST" });
  }
  await refreshScore();
  markSelection();
  return { said, drawings, wrote, error };
}

// What was run last, so that the result panel can run it again without
// anybody having to find it on the menu a second time.
let lastScript = { source: "", name: "", about: "" };

// runConsoleAndShow runs a script and shows what it had to show.
//
// One place, because there are three ways in — the Run button, a plugin
// picked off the Scripts menu, and Run it again — and a script run one way
// that said nothing where the same script run another way said five things
// would be a script that looked broken.
//
// The console window closes first. A script is about the music, and the
// window it was started from sits squarely over the music: the thing a
// script wrote cannot be seen until the window is out of the way, and the
// thing it found out is about bars you want to be looking at. So what it
// said and drew goes to a panel in the corner instead, which leaves the
// score usable underneath it.
async function runConsoleAndShow(source, called, about) {
  lastScript = { source, name: called || "Script", about: about || "" };
  const dlg = $("#console");
  if (dlg && dlg.open) dlg.close();

  const got = await runConsole(source);
  const lines = [...got.said];
  if (got.error) lines.push(got.error);
  else if (got.wrote) {
    lines.push(`Wrote ${got.wrote} thing${got.wrote === 1 ? "" : "s"}.`);
  }
  // The console keeps its own copy, for whoever reopens it to carry on
  // writing: the panel is the result, the box is the workshop.
  consoleSay(lines.join("\n") || "Nothing to do.", !!got.error);
  showScriptResult(lastScript, lines, got.drawings || [], !!got.error);
  return got;
}

// showScriptResult puts what a script said and drew in the corner panel.
//
// Always, even when there is nothing to say. "I ran it and nothing
// happened" is the complaint a script that silently did nothing earns, and
// a panel that says in as many words that the script looked and found
// nothing is the answer to it.
function showScriptResult(what, lines, drawings, bad) {
  const dlg = $("#script-result");
  if (!dlg) return;
  $("#script-result-name").textContent = what.name || "Script";
  const about = $("#script-result-about");
  about.textContent = what.about || "";
  about.hidden = !what.about;

  const said = $("#script-result-said");
  said.textContent = lines.join("\n");
  said.classList.toggle("console-bad", !!bad);

  const drawn = $("#script-result-drawn");
  drawn.innerHTML = "";
  for (const picture of drawings) {
    const fig = document.createElement("figure");
    // The drawing as the script wrote it. A script already runs in this
    // page and could reach the window without asking, so nothing is gained
    // by picking its markup apart here — what gscore promises is that
    // nothing runs until you have read it.
    fig.innerHTML = picture.svg;
    if (picture.caption) {
      const cap = document.createElement("figcaption");
      cap.textContent = picture.caption;
      fig.append(cap);
    }
    drawn.append(fig);
  }

  // Shown, not shown modally: the score behind it stays scrollable,
  // clickable and playable while the report is up, which is the whole
  // reason the report is not in the window it was started from.
  if (!dlg.open) dlg.show();
}

// doConsoleStep carries out one thing the script asked for.
async function doConsoleStep(step) {
  switch (step.do) {
    case "line":
      // The typing language, so everything it can say the console can say
      // — under the same rule about which commands a script may run, since
      // a bare word in a typed line is a command like any other.
      return (await runTyping(step.line, true))
        ? "" : "could not write " + step.line;
    case "run": {
      const cmd = commandNamed(step.name);
      if (!cmd) return "there is no command called " + step.name;
      const why = scriptRefusal(cmd);
      if (why) return why;
      await cmd.run();
      return "";
    }
    case "at": {
      const sel = state.selection || { part: 0, measure: 0, step: 4 };
      const part = step.staff === undefined ? sel.part : step.staff;
      while (measureCount() <= step.bar) {
        await fetch("/api/addmeasure", { method: "POST" });
        await loadHits();
      }
      state.selection = {
        ...sel, part, measure: step.bar,
        at: step.place === undefined ? 0 : step.place,
        ref: null, replace: false,
      };
      return "";
    }
    case "bar":
      await nextBarForTyping();
      return "";
    case "staff": {
      const q = new URLSearchParams({ instrument: step.instrument });
      if (step.clef) q.set("clef", step.clef);
      if (step.take) {
        // Moving into the empty staff a new score comes with, rather than
        // leaving it above the music as a staff nobody asked for. It has to
        // be given a name here: the staff already has one, and the one it
        // has is "Staff".
        q.set("part", "0");
        q.set("name", step.name || step.instrument);
        const res = await fetch("/api/staff?" + q, { method: "POST" });
        if (!res.ok) return (await res.text()).trim();
        await loadHits();
        return "";
      }
      if (step.name) q.set("name", step.name);
      const res = await fetch("/api/addstaff?" + q, { method: "POST" });
      if (!res.ok) return (await res.text()).trim();
      // The new staff has the bars the others have, and at() counts them.
      await loadHits();
      return "";
    }
    case "drum": {
      const key = await drumKeyFor(step.which);
      if (!key) return "there is no drum called " + step.which;
      const got = await typeOne({ kind: "note", drum: key, dur: step.dur });
      return got.error || "";
    }
    case "key": {
      const res = await fetch("/api/setkey?fifths=" + step.fifths,
                              { method: "POST" });
      return res.ok ? "" : (await res.text()).trim();
    }
    case "dynamic": {
      const sel = state.selection;
      if (!sel || !sel.at) return "there is no note for " + step.name;
      const q = new URLSearchParams({
        part: String(sel.part), measure: String(sel.measure),
        at: String(sel.at), dynamic: step.name,
      });
      const res = await fetch("/api/dynamic?" + q, { method: "POST" });
      return res.ok ? "" : (await res.text()).trim();
    }
    case "chord": {
      // The first note is written and the rest are stacked onto it, which
      // is how a chord is built everywhere else in the editor.
      const first = await typeOne({ kind: "note", pitch: step.pitches[0], dur: step.dur });
      if (first.error) return first.error;
      const sel = state.selection;
      for (const p of step.pitches.slice(1)) {
        const q = new URLSearchParams({
          part: String(sel.part), measure: String(sel.measure),
          index: String(sel.at - 1), pitch: p,
        });
        const res = await fetch("/api/addpitch?" + q, { method: "POST" });
        if (!res.ok) return (await res.text()).trim();
      }
      return "";
    }
    default: {
      const what = {
        kind: step.do === "rest" ? "rest" : "note",
        pitch: step.pitch, dur: step.dur,
      };
      if (step.do === "step") what.interval = step.interval;
      const got = await typeOne(what);
      return got.error || "";
    }
  }
}

// drumKeyFor turns what a script called a drum into the key that sounds it.
//
// The largest kit is asked for, not the one the drum pad happens to be set
// to: a script naming the ride bell should not be told there is no such
// drum because somebody left the pad on the six-pad kit.
let drumNames = null;
async function drumKeyFor(which) {
  const n = Number(which);
  if (Number.isInteger(n) && n > 0) return n;
  if (!drumNames) {
    try {
      const res = await fetch("/api/percussion?kit=large");
      drumNames = res.ok ? (await res.json()).pads || [] : [];
    } catch (_) {
      drumNames = [];
    }
  }
  const want = String(which).trim().toLowerCase();
  const pad = drumNames.find((d) => d.name.toLowerCase() === want) ||
    drumNames.find((d) => d.name.toLowerCase().startsWith(want));
  return pad ? pad.key : 0;
}

// consoleHelp is the vocabulary, beside the buttons.
//
// Two lines, where it was one and was getting longer every time a word was
// added. One line was the right shape while every word wrote music: a dozen
// verbs that do one kind of thing read as a sentence. Then the console
// learned to read the score and to run the editor's own commands, and the
// line was carrying two unrelated languages end to end — a script writer
// looking for "how do I ask what is in this bar" had to read past eleven
// ways of writing a note to find out.
//
// So the split is the one the language actually has, and it is a better
// answer than a longer line: what writes music, what asks about it, what
// shows you the answer. Three short labelled lines read faster than one
// long one, which is the test — not the count. A line long enough to need
// reading twice is the thing to avoid, and the answer to that would be a
// help that is not the bar at the bottom of the window.
const consoleHelp = [
  "Writing: note(\"c4\", 8) · rest(4) · chord(…, 2) · step(+3) · bar() · " +
  "at(bar) · staff(name) · key(4) · dynamic(name) · drum(name) · " +
  "write(\"8abc\") · run(\"tie\")",
  "Asking: notes(bar) · measure(bar) · staves() · selection() · " +
  "score.bars · transpose(\"c4\", 4) · chordNotes(\"Cmaj7\") · " +
  "latinSyllables(\"alleluia\") · ask(q)",
  "Showing: say(…) · draw(svg, caption) · await progress(i, n)",
];

// consoleExamples are whole scripts to start from, built into the editor.
//
// They open in the box and do not run, which is the same rule a script a
// file carries lives under: nothing in gscore runs code you have not read.
// One example that really writes a piece says more about what the console
// is for than a page of documentation about what each verb does.
const consoleExamples = [
  {
    name: "After the Moonlight",
    about: "asks which instruments, then writes a Presto agitato of its own",
    body: `// After the Moonlight — a Presto agitato out of rules and dice.
//
// Not a copy of Beethoven's finale. What is taken from it is the argument:
// C sharp minor at speed, a texture that never stops moving, sforzandi
// that arrive without being led up to, a dry bass underneath, and a
// harmony that refuses to settle. What is left to the dice is everything
// else — which bar does what, how long a thought lasts, where it sits,
// how loud, and what the kit is doing about it.
//
// So it writes a different piece every time, and none of them is his. Run
// it twice and compare the two; that difference is the part that is yours
// to steer, and every knob for it is a table at the top of this script.

// What each player is given, and what it can reach.
//
// base is the octave a figure would like to start from, span how many notes
// of the chord it climbs through, low and high the notes the instrument
// actually has — in concert pitch, what you hear. The range is not
// decoration: a figure that will not fit inside it is moved by octaves
// until it does, and narrowed if that is not enough, so an oboe gets the
// shape a piano gets without being asked for notes it does not own.
//
// up is how far the part is written above what it sounds, in letters and
// semitones, so a part for an instrument in E flat comes out in the key its
// player reads. poly says the staff can hold a chord; one flute cannot.
const PLAYERS = {
  piano:         { role: "both",  base: 3, span: 9, low: "a0",  high: "c8", poly: true },
  harpsichord:   { role: "both",  base: 3, span: 8, low: "f1",  high: "f6", poly: true },
  harp:          { role: "both",  base: 3, span: 9, low: "c1",  high: "g7", poly: true },
  violin:        { role: "line",  base: 4, span: 8, low: "g3",  high: "a7" },
  viola:         { role: "line",  base: 3, span: 8, low: "c3",  high: "e6", clef: "alto" },
  flute:         { role: "line",  base: 4, span: 8, low: "c4",  high: "d7" },
  oboe:          { role: "line",  base: 4, span: 7, low: "bb3", high: "a6" },
  clarinet:      { role: "line",  base: 4, span: 8, low: "d3",  high: "bb6", up: [1, 2] },
  trumpet:       { role: "line",  base: 4, span: 7, low: "e3",  high: "c6",  up: [1, 2] },
  "french horn": { role: "line",  base: 3, span: 7, low: "b1",  high: "f5",  up: [4, 7] },
  "alto sax":    { role: "line",  base: 4, span: 7, low: "db3", high: "ab5", up: [5, 9] },
  "tenor sax":   { role: "line",  base: 3, span: 7, low: "ab2", high: "e5",  up: [8, 14] },
  cello:         { role: "bass",  base: 2, span: 6, low: "c2",  high: "a5" },
  bassoon:       { role: "bass",  base: 2, span: 6, low: "bb1", high: "e5" },
  contrabass:    { role: "bass",  base: 2, span: 5, low: "e1",  high: "g3", up: [7, 12] },
  "drum kit":    { role: "drums" },
};

const CHOICES = Object.keys(PLAYERS);

// The chords, spelled out. A diminished fifth is not an augmented fourth
// and a B sharp is not a C, however the keyboard feels about it: the
// spelling says which chord this is, and it is the spelling that has to
// survive being written out for a part in another key.
const CHORDS = {
  "i":     ["c#", "e",  "g#"],
  "iv":    ["f#", "a",  "c#"],
  "V7":    ["g#", "b#", "d#", "f#"],
  "VI":    ["a",  "c#", "e"],
  "ii0":   ["d#", "f#", "a"],
  "N":     ["d",  "f#", "a"],
  "vii07": ["b#", "d#", "f#", "a"],
  "III":   ["e",  "g#", "b"],
};

// Where each chord is willing to go next. This is the whole of the harmony:
// no rule about what a chord is, only about what follows what, which is the
// part a listener actually hears.
const NEXT = {
  "i":     ["iv", "V7", "VI", "N", "vii07", "III"],
  "iv":    ["V7", "vii07", "i", "N"],
  "V7":    ["i", "VI"],
  "VI":    ["iv", "ii0", "N", "V7"],
  "ii0":   ["V7", "vii07"],
  "N":     ["V7", "vii07"],
  "vii07": ["i", "V7"],
  "III":   ["VI", "iv"],
};

// The scale the runs walk. The seventh is raised where the harmony leans on
// it, which is the one thing the minor scale cannot make its mind up about.
const SCALE = ["c#", "d#", "e", "f#", "g#", "a", "b"];
const LEADING = ["V7", "vii07"];

const LETTERS = "cdefgab";
const STEPS = [0, 2, 4, 5, 7, 9, 11];
const SIGNS = ["bb", "b", "n", "#", "x"];

function pick(list) {
  return list[Math.floor(Math.random() * list.length)];
}

function chance(p) {
  return Math.random() < p;
}

// midi is what a written note sounds as, so two of them can be compared.
function midi(note) {
  const letter = LETTERS.indexOf(note[0]);
  const octave = Number(note[note.length - 1]);
  let alter = 0;
  for (const c of note.slice(1, -1)) {
    if (c === "#") alter += 1;
    else if (c === "x") alter += 2;
    else if (c === "b") alter -= 1;
  }
  return 12 * (octave + 1) + STEPS[letter] + alter;
}

function moveOctave(note, by) {
  return note.slice(0, -1) + (Number(note[note.length - 1]) + by);
}

// written turns a note as it sounds into the note its player reads, spelled
// out — accidental and all, including the naturals.
//
// Spelled out because the key signature is not to be trusted to say it. A D
// in C sharp minor is a D sharp unless somebody writes otherwise, and the D
// of the Neapolitan is the one note in the chord that has to.
//
// up is how far above the sounding note the written one goes, as a number
// of letters and a number of semitones — a major sixth for an alto
// saxophone is five letters and nine semitones. Both are needed and
// neither on its own will do: the letters decide what it is called and the
// semitones decide what it sounds like, and the accidental is whatever
// makes the two agree. That is how the leading note of C sharp minor,
// written for alto saxophone, comes out a G double sharp.
function written(note, up) {
  up = up || [0, 0];
  const letter = LETTERS.indexOf(note[0]);
  const octave = Number(note[note.length - 1]);
  let alter = 0;
  for (const c of note.slice(1, -1)) {
    if (c === "#") alter += 1;
    else if (c === "x") alter += 2;
    else if (c === "b") alter -= 1;
  }
  const reach = letter + up[0];
  const over = Math.floor(reach / 7);
  const to = reach - over * 7;
  const plain = STEPS[to] + 12 * over - STEPS[letter];
  const sign = alter + up[1] - plain;
  if (sign < -2 || sign > 2) {
    throw new Error(note + " cannot be written for that instrument");
  }
  return LETTERS[to] + SIGNS[sign + 2] + (octave + over);
}

// climb is a set of notes laid out upward from an octave, as many as asked.
//
// The octave goes up whenever the letter does not: F sharp, A, C sharp is a
// chord of F sharp minor, and its C sharp is the one above the A and not
// the one three notes below it.
function climb(notes, from, count) {
  const out = [];
  let octave = from, last = -1;
  for (let i = 0; i < count; i++) {
    const note = notes[i % notes.length];
    const letter = LETTERS.indexOf(note[0]);
    if (letter <= last) octave++;
    last = letter;
    out.push(note + octave);
  }
  return out;
}

// inside moves a run of notes by whole octaves until all of it is within
// reach, and says so by answering with nothing if it cannot be.
function inside(run, p) {
  const low = midi(p.low), high = midi(p.high);
  for (const by of [0, -1, 1, -2, 2, -3, 3]) {
    const moved = run.map((n) => moveOctave(n, by));
    const got = moved.map(midi);
    if (Math.min(...got) >= low && Math.max(...got) <= high) return moved;
  }
  return null;
}

// reach is the notes this player can have of this chord, as high and as
// wide as it can take them.
//
// The home octave first, then the octave either side of it, and only then
// a narrower figure: a sweep in the wrong octave is further from what was
// meant than a shorter one in the right place.
function reach(notes, p, lift) {
  // Never fewer than four notes to work with: the shapes below count on
  // having somewhere to fall to, and a chord narrowed to three of them is
  // a shape with a hole where its fourth note should be.
  for (let span = p.span; span >= 4; span--) {
    const got = inside(climb(notes, p.base + (lift || 0), span), p);
    if (got) return got;
  }
  throw new Error("nothing of that chord fits this instrument");
}

// zigzag walks up the notes it is given and turns round at the top.
function zigzag(tones, count, from) {
  const out = [];
  let i = from || 0, step = 1;
  for (let k = 0; k < count; k++) {
    out.push(tones[i]);
    i += step;
    if (i >= tones.length) { i = tones.length - 2; step = -1; }
    if (i < 0) { i = 1; step = 1; }
  }
  return out;
}

// ------------------------------------------------------------ the shapes
//
// A bar is a list of moments: what sounds, for how long, and whether
// anything is marked on it. Building it as a list rather than writing it
// out as we go puts the arithmetic in one place — a shape that does not
// add up to a bar is caught here, by the bar itself, and not later by
// somebody counting beats on the page.

function bar16(events, what) {
  let held = 0;
  for (const e of events) held += 1 / e.len;
  if (Math.abs(held - 1) > 1e-9) {
    throw new Error((what || "a shape") + " came out " + held + " of a bar");
  }
  return events;
}

const one = (t, len, mark) => ({ tones: [t], len, mark });
const many = (ts, len, mark) => ({ tones: ts, len, mark });
const gap = (len) => ({ rest: true, len });

// Every shape is handed the same four things: the chord tones it may use,
// the scale behind them, who is playing, and the bar's dice.
//
// The dice are thrown once for the bar and then given to everybody, rather
// than each player throwing their own. That is what lets two instruments
// double a line: they are reading the same throw, so they turn at the same
// note. Each throwing separately gave four players four different versions
// of one idea, played at once, which is not an orchestration — it is a
// smear.

// The sweep: eight sixteenths climbing, two eighths where it breaks, four
// falling away. The one shape that is his outright.
function sweep(tones, scale, p, r) {
  const run = zigzag(tones, 14, r.from);
  const top = tones[tones.length - 1];
  return bar16([
    ...run.slice(0, 8).map((t) => one(t, 16)),
    one(top, 8, "sfz"), one(top, 8, "sfz"),
    ...[2, 3, 4, 5].map((k) => one(tones[Math.max(0, tones.length - k)], 16)),
  ], "the sweep");
}

// A wave with no break in it: up and straight back down.
function wave(tones, scale, p, r) {
  return bar16(zigzag(tones, 16, r.from).map((t) => one(t, 16)), "the wave");
}

// A run up the scale, which is where the piece goes when the arpeggios have
// been going long enough.
function scaleRun(tones, scale, p, r) {
  const from = midi(tones[0]);
  const line = climb(scale, Number(tones[0][tones[0].length - 1]), 24)
    .filter((n) => midi(n) >= from);
  if (line.length < 8) throw new Error("the scale ran out under the run");
  const walk = r.upward ? line.slice(0, 8) : line.slice(0, 8).reverse();
  return bar16([
    ...walk.map((t) => one(t, 16)),
    ...walk.slice().reverse().map((t) => one(t, 16)),
  ], "the run");
}

// Broken octaves, or whatever interval the chord puts under the hand.
function broken(tones, scale, p, r) {
  const lo = tones[r.from], hi = tones[Math.min(r.from + 2, tones.length - 1)];
  const out = [];
  for (let i = 0; i < 16; i++) out.push(one(i % 2 ? hi : lo, 16));
  return bar16(out, "the broken octaves");
}

// A turn round each chord tone: the note, the one above, the note, the one
// below. Four beats of fidgeting, which is what agitato means.
function turn(tones, scale, p, r) {
  const out = [];
  for (let beat = 0; beat < 4; beat++) {
    const t = tones[(beat + r.from) % tones.length];
    const i = scale.indexOf(t.slice(0, -1));
    const oct = Number(t[t.length - 1]);
    const ladder = climb(scale, oct, 9);
    const over = i < 0 ? t : ladder[i + 1];
    const under = i <= 0 ? t : ladder[i - 1];
    out.push(one(t, 16), one(over, 16), one(t, 16), one(under, 16));
  }
  return bar16(out, "the turn");
}

// The chord itself, hammered.
//
// A piano can hammer a chord; one flute cannot, and eight repetitions of a
// single note is not a texture, it is a stuck key. So a part that can hold
// only one note at a time rocks between two of the chord instead, which is
// the same energy and is a line.
function hammer(tones, scale, p, r) {
  const top = tones.slice(-3);
  const two = [tones[tones.length - 1], tones[Math.max(0, tones.length - 3)]];
  const out = [];
  for (let i = 0; i < 8; i++) {
    const mark = i === 0 ? "sfz" : undefined;
    out.push(p.poly ? many(top, 8, mark) : one(two[i % 2], 8, mark));
  }
  return bar16(out, "the hammered chord");
}

// Two blows and two silences. Nothing in the movement is louder than the
// beat he leaves empty after one.
function stab(tones, scale, p, r) {
  const top = p.poly ? tones.slice(-3) : [tones[tones.length - 1]];
  return bar16(
    [many(top, 4, "sfz"), gap(4), many(top, 4, "sfz"), gap(4)], "the stab");
}

// The end: one chord, held, and done.
function close(tones, scale, p, r) {
  return bar16([many(p.poly ? tones.slice(-3) : [tones[0]], 1)], "the last bar");
}

const FIGURES = {
  sweep, wave, run: scaleRun, broken, turn, hammer, stab,
};
// What the figure does over a phrase, and how often. The sweep is the
// piece's own voice and the rest are what it says in between, so it comes
// up more often than any one of them.
const FIGURE_WEIGHTS = [
  "sweep", "sweep", "sweep", "wave", "wave",
  "run", "run", "broken", "turn", "hammer", "stab",
];

// ------------------------------------------------------------- the bass

function bassLine(chord, p, kind) {
  const tones = reach(chord, p, 0);
  const root = tones[0];
  const fifth = tones[Math.min(2, tones.length - 1)];
  const over = inside([moveOctave(root, 1)], p);
  const high = over ? over[0] : fifth;

  switch (kind) {
    case "held":
      return bar16([one(root, 1)]);
    case "halves":
      return bar16([one(root, 2), one(fifth, 2)]);
    case "walk":
      // Four quarters, however many notes the chord gave us to walk on.
      return bar16([0, 1, 2, 3].map((i) => one(tones[i % tones.length], 4)));
    case "offbeat": {
      const out = [];
      for (let i = 0; i < 4; i++) out.push(gap(8), one(i % 2 ? fifth : root, 8));
      return bar16(out);
    }
    case "murky": {
      // The eighteenth century's own engine: root and fifth alternating in
      // sixteenths under everything, and nobody has bettered it.
      const out = [];
      for (let i = 0; i < 16; i++) out.push(one(i % 2 ? fifth : root, 16));
      return bar16(out);
    }
    case "pedal": {
      const out = [];
      for (let i = 0; i < 8; i++) out.push(one(root, 8));
      return bar16(out);
    }
    default: {
      const out = [];
      for (let i = 0; i < 8; i++) out.push(one(i % 2 ? high : root, 8));
      return bar16(out);
    }
  }
}

const BASSES = [
  "octaves", "octaves", "pedal", "murky", "held", "halves", "walk", "offbeat",
];
// Which bass goes under which figure. A held chord under a hammered one is
// two things standing still; the engine shapes want something moving.
const BASS_FOR = {
  hammer: ["held", "halves", "offbeat"],
  stab: ["held", "halves"],
  run: ["octaves", "pedal", "held"],
};

// ------------------------------------------------------------- the drums

const KICK = "bass drum", HAT = "hi-hat closed", OPEN = "hi-hat open";
const SNARE = "snare drum", CRASH = "crash cymbal", RIDE = "ride cymbal";
const TOMS = ["high tom", "low-mid tom", "low tom", "low floor tom"];

function drumLine(kind, breaks, ending) {
  const hit = (name, len) => ({ drum: name, len });
  if (ending) {
    // A fill, because a phrase that simply stops has not ended.
    return bar16([
      hit(KICK, 4), hit(SNARE, 4),
      ...TOMS.map((t) => hit(t, 16)),
      hit(SNARE, 16), hit(SNARE, 16), hit(SNARE, 16), hit(CRASH, 16),
    ], "the fill");
  }
  switch (kind) {
    case "backbeat":
      return bar16([
        hit(KICK, 8), hit(HAT, 8), hit(SNARE, 8), hit(HAT, 8),
        hit(KICK, 8), hit(KICK, 8), hit(SNARE, 8), hit(OPEN, 8),
      ]);
    case "roll": {
      const out = [];
      for (let i = 0; i < 16; i++) out.push(hit(i % 4 ? SNARE : KICK, 16));
      return bar16(out);
    }
    case "sparse":
      return bar16([
        hit(breaks ? CRASH : KICK, 4), { rest: true, len: 4 },
        hit(RIDE, 4), { rest: true, len: 4 },
      ]);
    default: {
      const out = [hit(KICK, 16)];
      for (let i = 1; i < 8; i++) out.push(hit(HAT, 16));
      out.push(hit(breaks ? CRASH : HAT, 8), hit(breaks ? CRASH : HAT, 8));
      for (const t of TOMS) out.push(hit(t, 16));
      return bar16(out);
    }
  }
}

const KITS = ["drive", "drive", "backbeat", "roll", "sparse"];

// -------------------------------------------------------------- the page

// play writes one bar out, in whatever key its player reads.
function play(events, p) {
  for (const e of events) {
    if (e.rest) { rest(e.len); continue; }
    if (e.drum) { drum(e.drum, e.len); continue; }
    const notes = e.tones.map((t) => written(t, p.up));
    if (notes.length > 1 && p.poly) chord(notes, e.len);
    else note(notes[notes.length - 1], e.len);
    if (e.mark) dynamic(e.mark);
  }
}

// ---------------------------------------------------------------- the run

const want = await ask("Which instruments?", CHOICES, {
  many: true, label: "Instruments",
  note: "Hold shift or command to take more than one.",
});
if (!want || !want.length) {
  say("Nothing chosen, so nothing written.");
  return;
}

const asked = await ask("How many measures?", "16", {
  note: "Four to twenty-four. It comes out in four-bar thoughts.",
});
if (asked === null) {
  say("Nothing written.");
  return;
}
let bars = Math.trunc(Number(asked));
if (!(bars >= 4)) bars = 16;
if (bars > 24) bars = 24;
bars -= bars % 4;

// The piece, phrase by phrase, before a note of it is written.
//
// Deciding it all first is what lets a phrase be a thought rather than four
// unrelated bars: the bass knows what the figure is doing, the kit knows
// where the phrase ends, and the harmony can hold a chord across two bars
// because somebody is looking at the whole four.
// Who the figure belongs to, phrase by phrase.
//
// Four instruments playing the same line is one instrument with a thicker
// sound, and that is what four bars of it sound like. So they take turns:
// one carries the figure, one holds a note under it, one plays the bass,
// one sits the phrase out and comes back. Rotating the jobs is what turns
// a texture into an orchestration, and it costs four lines.
const JOBS = ["under", "hold", "hold", "figure", "tacet"];

const melodic = want.filter((n) => PLAYERS[n] && PLAYERS[n].role === "line");

const phrases = [];
for (let at = 0; at < bars; at += 4) {
  const last = at + 4 >= bars;
  const shape = last ? "sweep" : pick(FIGURE_WEIGHTS);
  const n = phrases.length;
  // A different one leads each phrase, and the rest fall in behind it in
  // the order they were chosen.
  const jobs = {};
  melodic.forEach((name, i) => {
    const away = (i - n % melodic.length + melodic.length) % melodic.length;
    jobs[name] = away === 0 ? "figure" : pick(JOBS);
  });
  phrases.push({
    at,
    shape,
    jobs,
    bass: pick(BASS_FOR[shape] || BASSES),
    kit: pick(KITS),
    // Where it sits. A phrase an octave up is the same thought said
    // higher, and it is most of what stops sixteen bars sounding like four
    // bars written out four times.
    lift: pick([0, 0, 0, 1, -1]),
    loud: last ? "ff" : pick(["p", "p", "mp", "mf", "f", "f"]),
    last,
  });
}

// The dice for each bar, thrown once and read by everybody. Two players
// doubling a line have to turn at the same note, and they cannot if each
// of them is throwing separately.
const rolls = [];
for (let i = 0; i < bars; i++) {
  rolls.push({ from: chance(0.3) ? 1 : 0, upward: chance(0.5) });
}

// The harmony. One chord to the bar, except where a phrase wants to lean on
// one for two bars, which is how a piece breathes.
const plan = [];
let now = "i";
for (let i = 0; i < bars; i++) {
  plan.push(now);
  if (!chance(0.25)) now = pick(NEXT[now]);
}
// Home at the end whatever the dice said. A piece that stops wherever the
// random walk had got to does not sound finished, it sounds dropped.
plan[bars - 3] = "V7";
plan[bars - 2] = "V7";
plan[bars - 1] = "i";

say("C sharp minor, " + bars + " bars, " + phrases.length + " phrases");
say(plan.join(" "));
say(phrases.map((f) => f.shape + "/" + f.bass + " " + f.loud).join(", "));
for (const name of melodic) {
  say("  " + name + ": " + phrases.map((f) => f.jobs[name]).join(" "));
}

// Four sharps, once, for the whole score — before any staff is made, so
// that a drum staff, which is in no key at all, is never given one. Every
// other staff gets the signature its own player reads for that key.
key(4);

function scaleFor(name) {
  return LEADING.includes(name)
    ? SCALE.slice(0, 6).concat("b#")
    : SCALE;
}

// oneBar writes whatever this player is doing in this bar.
function oneBar(p, f, i, voice) {
  const name = plan[i];
  const notes = CHORDS[name];
  const ending = f.last && i === bars - 1;
  const job = ending ? "figure" : (f.jobs ? f.jobs[p.who] : "figure");

  if (job === "tacet") {
    rest(1);
    return;
  }
  if (job === "under") {
    play(bassLine(notes, p, f.bass), p);
    return;
  }
  if (job === "hold") {
    // One note of the chord, held — a different one for each player, so
    // that what they are holding between them is the chord.
    const tones = reach(notes, p, f.lift);
    const held = tones[voice % tones.length];
    play(bar16([one(held, 1, i === f.at ? "p" : undefined)], "a held note"), p);
    return;
  }
  const tones = reach(notes, p, f.lift);
  const shape = ending ? close : FIGURES[f.shape];
  const events = shape(tones, scaleFor(name), p, rolls[i]);
  if (i === f.at) events[0] = { ...events[0], mark: f.loud };
  play(events, p);
}

for (const name of want) {
  const p = PLAYERS[name];
  if (!p) continue;

  if (p.role === "drums") {
    const where = staff(name);
    at(1, where);
    for (let i = 0; i < bars; i++) {
      const f = phrases[Math.floor(i / 4)];
      play(drumLine(f.kit, i % 2 === 1, i === f.at + 3 && !f.last), p);
      if (i < bars - 1) bar();
    }
    continue;
  }

  const two = p.role === "both";
  if (two || p.role === "line") {
    // who is how the phrase finds out what this player was given to do, and
    // voice is which note of a held chord is theirs.
    const me = { ...p, who: name };
    const voice = Math.max(0, melodic.indexOf(name));
    const where = staff(name, {
      clef: p.clef || "treble",
      name: two ? name + " right" : "",
    });
    at(1, where);
    for (let i = 0; i < bars; i++) {
      oneBar(me, phrases[Math.floor(i / 4)], i, voice);
      if (i < bars - 1) bar();
    }
  }
  if (two || p.role === "bass") {
    const low = two ? { ...p, base: p.base - 1, span: 6 } : p;
    const where = staff(name, {
      clef: "bass",
      name: two ? name + " left" : "",
    });
    at(1, where);
    for (let i = 0; i < bars; i++) {
      const f = phrases[Math.floor(i / 4)];
      const kind = f.last && i === bars - 1 ? "held" : f.bass;
      play(bassLine(CHORDS[plan[i]], low, kind), low);
      if (i < bars - 1) bar();
    }
  }
}

say("Written. Run it again for a different piece.");
`,
  },
];

function drawExamples() {
  const box = $("#console-examples");
  if (!box) return;
  box.innerHTML =
    '<span class="console-saved-note">Examples:</span>' +
    consoleExamples.map((e, i) =>
      `<button type="button" class="console-script" data-example="${i}" ` +
      `title="${escapeHTML(e.about)}">${escapeHTML(e.name)}</button>`).join("");
}

// state.scripts is what the score carries. Shown, never run.
state.scripts = [];

async function openConsole() {
  const dlg = $("#console");
  if (!dlg) return;
  await loadScripts();
  if (!dlg.open) dlg.showModal();
  $("#console-source")?.focus?.();
}

// loadScripts reads what the file carries and puts it on the screen.
//
// On the screen and nowhere else. This is the whole of the promise: a score
// may arrive from anybody, and opening one must never be a way for them to
// run something on your machine. So the scripts are shown, and a person
// reads them and presses Run.
async function loadScripts() {
  try {
    const res = await fetch("/api/scripts");
    if (res.ok) state.scripts = (await res.json()).scripts || [];
  } catch (_) {
    state.scripts = [];
  }
  drawSavedScripts();
  drawExamples();
  const help = $("#console-help");
  if (help) {
    help.textContent = "";
    for (const line of consoleHelp) {
      const row = document.createElement("div");
      row.textContent = line;
      help.append(row);
    }
  }
  return state.scripts;
}

function drawSavedScripts() {
  const box = $("#console-saved");
  if (!box) return;
  if (!state.scripts.length) {
    box.innerHTML = "";
    return;
  }
  box.innerHTML =
    state.scripts.map((s, i) =>
      `<button type="button" class="console-script" data-script="${i}">` +
      `${escapeHTML(s.Name || s.name || `script ${i + 1}`)}</button>`).join("") +
    `<span class="console-saved-note">kept with the score — opened, not run</span>`;
}

// saveScript keeps what is in the box with the score, under the name given.
async function saveScript() {
  const source = $("#console-source")?.value ?? "";
  if (!source.trim()) {
    consoleSay("There is nothing to keep", true);
    return;
  }
  // A name, always. A score may carry several scripts and the name is the
  // only thing that tells them apart: an unnamed one is one nobody can ask
  // for again. Keeping a script under a name that is already there
  // replaces it, which is what saving the thing you just opened means.
  const name = ($("#console-name")?.value || "").trim();
  if (!name) {
    consoleSay("Give it a name first — that is how you find it again", true);
    $("#console-name")?.focus?.();
    return;
  }
  const next = state.scripts.filter((s) => (s.Name || s.name) !== name);
  next.push({ Name: name, Body: source });

  const res = await fetch("/api/scripts", {
    method: "POST",
    body: JSON.stringify({ scripts: next }),
  });
  if (!res.ok) {
    consoleSay((await res.text()).trim(), true);
    return;
  }
  state.scripts = next;
  drawSavedScripts();
  consoleSay(name ? `Kept as "${name}"` : "Kept with the score");
}

function consoleSay(text, bad) {
  const out = $("#console-out");
  if (!out) return;
  out.textContent = text;
  out.className = bad ? "console-bad" : "";
}

function setupConsole() {
  const dlg = $("#console");
  if (!dlg) return;
  const source = $("#console-source");

  $("#console-run")?.addEventListener("click", () =>
    void runConsoleAndShow(source?.value ?? ""));

  $("#console-save")?.addEventListener("click", () => void saveScript());

  // An example opens in the box, under its own name, so that keeping it
  // after changing it keeps it as something of yours.
  $("#console-examples")?.addEventListener("click", (e) => {
    const b = e.target.closest ? e.target.closest(".console-script") : null;
    if (!b || !source) return;
    const ex = consoleExamples[Number(b.dataset.example)];
    if (!ex) return;
    source.value = ex.body;
    const name = $("#console-name");
    if (name) name.value = ex.name;
    consoleSay("Opened. Read it, then press Run.");
  });

  // Clicking a saved script opens it in the box. Opens: the one thing it
  // does not do is run it.
  $("#console-saved")?.addEventListener("click", (e) => {
    const b = e.target.closest ? e.target.closest(".console-script") : null;
    if (!b || !source) return;
    const s = state.scripts[Number(b.dataset.script)];
    if (!s) return;
    source.value = s.Body || s.body || "";
    const name = $("#console-name");
    if (name) name.value = s.Name || s.name || "";
    consoleSay("Opened. Read it, then press Run.");
  });

  // What is typed in the box is not typed at the score.
  for (const kind of ["keydown", "keypress", "keyup"]) {
    source?.addEventListener(kind, (e) => e.stopPropagation());
    $("#console-name")?.addEventListener(kind, (e) => e.stopPropagation());
  }
}

// transposeName moves a written note by diatonic steps.
//
// Diatonic, so that "up a fifth" is four steps and comes out spelled the
// way the key spells it rather than as whatever semitone arithmetic
// produces: a fifth above F is C and not B sharp, and no amount of counting
// semitones will tell you which of those was meant. Any accidental written
// on the note is dropped for the same reason — the key signature spells the
// answer, exactly as it does for a note written by hand.
/*
 * Dividing a Latin word into the syllables it is sung on.
 *
 * Setting a text under music means knowing where the words break, and
 * doing it by hand is the slowest part of entering a chorus: a Mass is
 * four hundred words and every one of them has to be hyphenated before a
 * note of it can be typed.
 *
 * Latin only, and it says so in its name. Latin can be divided by rule —
 * the rules below are the ones printed at the front of the Liber Usualis
 * and are what choral editions follow — and English cannot: "heaven" is
 * heav-en and "beaver" is bea-ver, and nothing in the letters says which.
 * English needs a hyphenation dictionary, which is a different piece of
 * work; a guesser that was right four words in five would leave every
 * word to be checked by eye, which is the work this is meant to save.
 *
 * It cannot tell Latin from English by looking — "mea" and "idea" are the
 * same shape — so the name is the contract. What it does refuse is a word
 * that cannot be Latin at all, and anything that is not a word.
 *
 * The rules, in the order they are applied:
 *
 *	ch ph th rh gn	one consonant, never divided: a-gnus, ma-chi-na
 *	qu, and gu after n	the u belongs to the consonant: a-qua, lin-gua
 *	i before a vowel	a consonant where it begins a word or stands
 *			between two vowels: Je-sus, al-le-lu-ia
 *	ae oe au	sung on one note, so one syllable: cae-lum, lau-da
 *	x		closes the syllable before it: dix-it, ex-cel-sis
 *	one consonant	begins the next syllable: Do-mi-nus
 *	two consonants	divided, the first closing: om-nis, ec-ce
 *	 but s+consonant,
 *	 mute+l or r	not divided: ho-stis, pa-tris
 *	three or more	the first closes, the rest begin: san-ctus, mon-stro
 *
 * Two places where editions disagree with each other rather than with
 * this: "eu" is a diphthong in words taken from Greek and two syllables in
 * Latin's own — De-us, me-us, but Eu-cha-ri-sti-a — and nothing in the
 * letters says which, so it is read as two; and "ct" is divided here by
 * the two-consonant rule, where editions set in Italy often keep it
 * together as di-ctus.
 */

// The letters Latin is written in, with the accents chant books mark the
// stress with, each against the plain letter it is.
const latinPlain = {
  à: "a", á: "a", â: "a", ä: "a", ā: "a",
  è: "e", é: "e", ê: "e", ë: "e", ē: "e",
  ì: "i", í: "i", î: "i", ï: "i", ī: "i",
  ò: "o", ó: "o", ô: "o", ö: "o", ō: "o",
  ù: "u", ú: "u", û: "u", ü: "u", ū: "u",
  ý: "y", ÿ: "y",
};
const latinVowels = "aeiouyæœ";
// A mute followed by a liquid is not divided: pa-tris, du-plex, a-gri.
const latinMutes = "bcdfgpt";
// And s before one of these is not divided: ho-stis, Chri-stus, pa-schae.
// These and not every consonant, because Latin begins words with st, sp and
// sc and never with sr or sl — the broader rule divided Is-ra-el as I-sra-el.
const latinAfterS = "cptq";

// latinSyllables divides a Latin word, giving back its syllables in order,
// or nothing where the word cannot be Latin.
//
// The syllables come back spelled as they were given, accents and capitals
// and all, so that joining them with hyphens gives the word back.
function latinSyllables(word) {
  const raw = String(word).trim();
  if (!raw) return null;
  const letters = [...raw];
  const plain = letters.map((ch) => {
    const low = ch.toLowerCase();
    return latinPlain[low] || low;
  });
  // A word, and a word that could be Latin. W is the one letter Latin has
  // no use for at all; j and k and y it has, in the words it took from
  // Greek and in the spellings modern books print.
  for (const ch of plain) {
    if (ch < "a" || ch > "z") {
      if (ch !== "æ" && ch !== "œ") return null;
    }
    if (ch === "w") return null;
  }

  const isVowel = (i) => i >= 0 && i < plain.length &&
    latinVowels.includes(plain[i]);

  // Which letters are vowels, once the ones that are standing in for
  // consonants have been taken out of the count.
  const vowel = plain.map((ch, i) => latinVowels.includes(ch));
  for (let i = 0; i < plain.length; i++) {
    if (plain[i] === "i" && isVowel(i + 1)) {
      // The consonant the English alphabet writes J. It begins a word, or
      // stands between two vowels; an i after a consonant is a vowel, so
      // the i of "Gloria" is not this.
      if (i === 0 || isVowel(i - 1)) vowel[i] = false;
    }
    if (plain[i] === "u" && i > 0) {
      // The u of qu, and of the ngu in lingua, belongs to the consonant
      // before it rather than being a syllable of its own.
      if (plain[i - 1] === "q") vowel[i] = false;
      if (plain[i - 1] === "g" && plain[i - 2] === "n" && isVowel(i + 1)) {
        vowel[i] = false;
      }
    }
  }

  // The nuclei: one per syllable, each the vowel or the pair of vowels it
  // is sung on. Everything between them is the consonants to be divided.
  const pairs = ["ae", "oe", "au"];
  const spelled = plain.join("");
  const nuclei = [];
  for (let i = 0; i < plain.length; i++) {
    if (!vowel[i]) continue;
    let end = i;
    if (vowel[i + 1] && pairs.includes(plain[i] + plain[i + 1])) {
      end = i + 1;
      // Except in the Hebrew names, where the two vowels are sung
      // separately: Is-ra-el, Mi-cha-el, Ra-pha-el, Na-tha-na-el. They all
      // end in -ael, which is as near a rule as there is — Latin's own
      // words have no such ending, and nothing else in the letters
      // distinguishes the two. (The word for air is the one that gets
      // away: a-er, read here as one syllable.)
      if (plain[i] === "a" && plain[i + 1] === "e" && spelled.endsWith("ael") &&
          i + 2 === plain.length - 1) {
        end = i;
      }
    }
    nuclei.push({ from: i, to: end });
    i = end;
  }
  if (!nuclei.length) return null; // no vowel in it, so not a word

  // Where each syllable begins. The first one begins at the beginning,
  // whatever consonants are in front of its vowel.
  const cuts = [0];
  for (let n = 1; n < nuclei.length; n++) {
    const after = nuclei[n - 1].to + 1;
    const before = nuclei[n].from;
    cuts.push(after + latinCarry(plain, after, before));
  }

  const out = [];
  for (let i = 0; i < cuts.length; i++) {
    const end = i + 1 < cuts.length ? cuts[i + 1] : letters.length;
    out.push(letters.slice(cuts[i], end).join(""));
  }
  return out;
}

// latinCarry says how many of the consonants between two vowels stay with
// the first of them.
function latinCarry(plain, from, to) {
  const run = plain.slice(from, to);
  if (!run.length) return 0;
  // x is two sounds in one letter and closes the syllable it follows:
  // dix-it, ex-cel-sis.
  if (run[0] === "x") return 1;

  // The pairs that are one consonant. Counted as one so that the rule
  // about two consonants is not applied to the two halves of a single
  // sound: a-gnus and ma-chi-na, not ag-nus and mach-i-na.
  const digraphs = ["ch", "ph", "th", "rh", "gn", "qu"];
  const units = [];
  for (let i = 0; i < run.length; i++) {
    if (i + 1 < run.length && digraphs.includes(run[i] + run[i + 1])) {
      units.push(run[i] + run[i + 1]);
      i++;
      continue;
    }
    units.push(run[i]);
  }

  if (units.length === 1) return 0; // one consonant begins the next syllable
  if (units.length === 2) {
    const [a, b] = units;
    // s before a mute, and a mute before l or r, are never divided:
    // ho-stis, Chri-stus, pa-schae, pa-tris.
    //
    // Before a mute, not before any consonant: Latin begins words with st,
    // sp and sc and never with sr or sl, and the broader rule divided
    // Is-ra-el as I-sra-el.
    if (a === "s" && latinAfterS.includes(b[0])) return 0;
    if (a.length === 1 && latinMutes.includes(a) && (b === "l" || b === "r")) {
      return 0;
    }
    return a.length; // divided, the first closing the syllable
  }
  // Three or more: the first closes the syllable and the rest begin the
  // next one — san-ctus, mon-stro, ex-em-plum.
  return units[0].length;
}

// How many semitones above C each letter sits, naturally.
const letterSemis = { c: 0, d: 2, e: 4, f: 5, g: 7, a: 9, b: 11 };

// chordNotesOf spells a chord symbol out as the notes it is made of.
//
// qualities is the table from the server, which owns what a chord is; this
// owns the letters. Nothing here knows that maj7 has an eleventh in it, and
// nothing there knows that the third of E flat is spelled G.
//
// The spelling is the point. Four semitones above C is E or F flat, and the
// difference between the two is the difference between a chord and a puzzle:
// what makes it E is being the third, so each tone is spelled on the letter
// its degree lands on and given whatever accidental makes it sound right.
// Doing it the other way about — picking the nearest black note and calling
// it a sharp — gives Cmaj7 a B and E flat major a D sharp, and a page of
// those is a page nobody can read.
//
// A quality the table does not know gives back nothing, because a chord
// filled in with the triad somebody probably meant is a chord that will be
// read, believed and played.
function chordNotesOf(symbol, octave, qualities) {
  if (!qualities) return null;
  const text = String(symbol).trim();
  // The root, and everything after it. A flat or a sharp belongs to the
  // root; a lower-case "b" straight after a letter is the flat and not the
  // start of the quality, which is why the accidentals are spelled out here
  // rather than left to a looser pattern.
  const m = /^([A-Ga-g])(##|bb|#|b|x|♯|♭|𝄪|𝄫)?(.*)$/.exec(text);
  if (!m) return null;
  // A slash bass is where the chord sits, not what it is made of. The
  // notes are the same either way, so the bass is dropped rather than
  // guessed at.
  const after = m[3].split("/")[0];
  const tones = qualities[after] ?? qualities[after.toLowerCase()];
  if (!tones) return null;

  const letters = "cdefgab";
  const letter = m[1].toLowerCase();
  const alter = { "#": 1, "♯": 1, "##": 2, x: 2, "𝄪": 2,
                  b: -1, "♭": -1, bb: -2, "𝄫": -2 }[m[2]] || 0;
  const from = letters.indexOf(letter);
  const oct = octave === undefined || octave === null ? 4 : Math.trunc(Number(octave));
  const rootSemi = letterSemis[letter] + alter;

  const marks = { 2: "x", 1: "#", 0: "n", "-1": "b", "-2": "bb" };
  const out = [];
  for (const t of tones) {
    const deg = from + t.degree;
    const up = letters[deg % 7];
    // Where the letter has crossed a C, and so an octave, on the way.
    const crossed = Math.floor(deg / 7);
    // What that letter sounds at, counted from the root's own C, and what
    // it has to be sharpened or flattened by to land where the chord says.
    const plain = letterSemis[up] + 12 * crossed;
    const want = rootSemi + t.semitones;
    const need = want - plain;
    if (!(need in marks)) return null; // nothing anybody would write
    // Every note spelled out, accidental or natural, because a script may
    // be writing onto a staff whose key signature is not the one it was
    // thinking in — a plain letter there is a different note.
    out.push(up + marks[need] + String(oct + crossed));
  }
  return out;
}

function transposeName(pitch, steps) {
  const m = /^([a-gA-G])([#sbxnSBXN]*)(-?\d+)?$/.exec(String(pitch).trim());
  if (!m) return pitch;
  const letters = "cdefgab";
  const from = letters.indexOf(m[1].toLowerCase());
  if (from < 0) return pitch;
  const octave = m[3] === undefined ? 4 : Number(m[3]);
  // Counted from C of octave zero, so that moving across a C is moving
  // into the next octave without a special case for it.
  const index = octave * 7 + from + steps;
  let letter = index % 7;
  let oct = Math.floor(index / 7);
  if (letter < 0) {
    letter += 7;
    oct -= 1;
  }
  return letters[letter] + (m[3] === undefined ? "" : String(oct));
}

/* ---------- listening ---------- */

// Recording an instrument, one note at a time.
//
// The microphone is the page's because a browser hands a page the sound and
// hands a program nothing. What the page does with it is as little as
// possible: take the samples, send them to the core, and show what came
// back. Which note it was, where the sound starts and stops, and where a
// loop can be hidden in it are all worked out in Go — see package
// recording — so that a note recorded in the browser and the same note
// recorded in the application cannot disagree about what it is.
let mic = null;

// askForMicrophone opens the microphone, once, and keeps it.
//
// Kept because opening one is slow and asks the person a question the first
// time: a dialog that records twelve notes should ask once and then be
// ready. It is let go when the dialog closes — a program holding a
// microphone it is not using is a program with a light on for no reason.
async function askForMicrophone() {
  if (mic && mic.stream && mic.stream.active) return mic;
  if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
    throw new Error("this browser will not let a page listen");
  }
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: {
      // None of the three. They are made for speech on a telephone and
      // every one of them is wrong here: the gate cuts the end of a note,
      // the echo canceller fights the room, and the automatic gain turns
      // a diminuendo into a crescendo.
      echoCancellation: false,
      noiseSuppression: false,
      autoGainControl: false,
      channelCount: 1,
    },
  });
  wakeAudio();
  await audio.resume().catch(() => {});
  mic = { stream, source: audio.createMediaStreamSource(stream) };
  return mic;
}

// closeMicrophone lets it go.
function closeMicrophone() {
  if (!mic) return;
  try {
    for (const t of mic.stream.getTracks()) t.stop();
    mic.source.disconnect();
  } catch (_) {
    /* already gone */
  }
  mic = null;
}

// recordTake listens until stop() is called, and hands back the samples.
//
// Through a ScriptProcessor rather than a MediaRecorder: what is wanted is
// the samples themselves, and a MediaRecorder gives back an encoded file —
// Opus, usually, which is lossy and would put the compression's own noise
// into every sample of the soundfont.
//
// It is the deprecated way of getting at audio in a page. The replacement,
// an AudioWorklet, has to be a separate file fetched from a URL, which the
// browser build cannot do: gscore in the browser is one page with no server
// behind it. A worklet is worth having when there is somewhere to put it.
function recordTake(onLevel) {
  if (!mic) throw new Error("the microphone is not open");
  const size = 4096;
  const node = audio.createScriptProcessor(size, 1, 1);
  const chunks = [];
  let frames = 0;
  let peak = 0;

  node.onaudioprocess = (ev) => {
    const got = ev.inputBuffer.getChannelData(0);
    const copy = new Float32Array(got.length);
    copy.set(got);
    chunks.push(copy);
    frames += copy.length;
    // The loudest thing so far, for the level meter — and for the warning
    // afterwards, since a take that touched the top was already clipped
    // by the time it arrived.
    for (let i = 0; i < copy.length; i++) {
      const a = Math.abs(copy[i]);
      if (a > peak) peak = a;
    }
    if (onLevel) onLevel(peak, frames / audio.sampleRate);
  };
  mic.source.connect(node);
  // Into a gain of nothing: a ScriptProcessor does not run unless it is
  // connected to something, and what it must not do is put the microphone
  // through the speakers, which is a howl.
  const sink = audio.createGain();
  sink.gain.value = 0;
  node.connect(sink);
  sink.connect(audio.destination);

  return {
    rate: audio.sampleRate,
    stop() {
      node.onaudioprocess = null;
      try {
        mic.source.disconnect(node);
        node.disconnect();
        sink.disconnect();
      } catch (_) {
        /* already gone */
      }
      // One run of samples, as 16-bit, which is what a soundfont holds and
      // what the core is expecting.
      const out = new Int16Array(frames);
      let at = 0;
      for (const c of chunks) {
        for (let i = 0; i < c.length; i++) {
          const v = Math.max(-1, Math.min(1, c[i]));
          out[at++] = v < 0 ? v * 32768 : v * 32767;
        }
      }
      return out;
    },
  };
}

// readTake sends a recording to the core and says what it turned out to be.
async function readTake(samples, rate) {
  const res = await fetch("/api/take?rate=" + encodeURIComponent(rate), {
    method: "POST",
    headers: { "Content-Type": "application/octet-stream" },
    body: samples.buffer.slice(samples.byteOffset,
      samples.byteOffset + samples.byteLength),
  });
  if (!res.ok) {
    throw new Error((await res.text()).trim());
  }
  return res.json();
}
