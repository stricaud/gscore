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
  view: { names: false, numbers: true },
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

// The palette lives in palette.js, which holds what is in it. This is what
// happens when one is clicked.
async function paletteAction(item) {
  if (item.tool) {
    switch (item.tool) {
      case "rest":
        state.tool.rest = true;
        if (item.dur) state.tool.duration = item.dur;
        if (item.dur && hasSelection()) {
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
        if (hasSelection()) {
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
  status(`${what}: ${value}`);
}

/* ---------- menus ---------- */

function setupMenus() {
  $$(".menu").forEach((menu) => {
    menu.querySelector(".menu-title").addEventListener("click", (e) => {
      e.stopPropagation();
      const wasOpen = menu.classList.contains("open");
      closeMenus();
      if (!wasOpen) menu.classList.add("open");
    });
    // Moving back up to the plain items closes the submenu, so the panel
    // does not sit over the item being reached for.
    // The menu's own items, not the ones inside its submenus: those are
    // matched by the same selector, so hovering "Crescendo" would have
    // closed the submenu it is in.
    menu.querySelectorAll(":scope > .menu-items > button").forEach((b) => {
      b.addEventListener("mouseenter", () => {
        $$(".submenu").forEach((s) => s.classList.remove("open", "flip"));
      });
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
    sub.querySelector(".submenu-title").addEventListener("click", (e) => {
      e.stopPropagation();
      if (sub.classList.contains("open")) {
        sub.classList.remove("open", "flip");
        return;
      }
      openSubmenu(sub);
    });
    // Hovering the title opens it and closes every other, which is how a
    // menu bar behaves. Hovering the open panel itself must not close it, so
    // the listener is on the title and not on the submenu as a whole.
    sub.querySelector(".submenu-title").addEventListener("mouseenter", () => {
      openSubmenu(sub);
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
  $$(".menu").forEach((m) => m.classList.remove("open"));
  $$(".submenu").forEach((m) => m.classList.remove("open", "flip"));
}

// openSubmenu shows one submenu and closes the rest.
//
// Exactly one is open at a time, and this is the only thing that decides it.
function openSubmenu(sub) {
  $$(".submenu").forEach((s) => {
    if (s !== sub) s.classList.remove("open", "flip");
  });
  sub.classList.add("open");
  placeSubmenu(sub);
}

// placeSubmenu opens a submenu to the left instead of the right when there is
// no room on the right, which there is not for a menu near the window edge.
function placeSubmenu(sub) {
  sub.classList.remove("flip");
  const items = sub.querySelector(".menu-items");
  if (!items) return;
  const r = items.getBoundingClientRect();
  if (r.right > window.innerWidth - 8) sub.classList.add("flip");
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
    case "type-music": toggleTypeBar(); break;
    case "typing-help": openTypingHelp(); break;
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

    case "hairpin-cresc": await setHairpin("crescendo", "Crescendo"); break;
    case "hairpin-dim": await setHairpin("diminuendo", "Diminuendo"); break;
    case "hairpin-none": await setHairpin("none", "Hairpin removed"); break;

    case "octave-8va": await setOctave("8va", "8va"); break;
    case "octave-8vb": await setOctave("8vb", "8vb"); break;
    case "octave-15ma": await setOctave("15ma", "15ma"); break;
    case "octave-15mb": await setOctave("15mb", "15mb"); break;
    case "octave-none": await setOctave("none", "Octave line removed"); break;

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

    case "insert-chord": await chordSymbolDialog(); break;
    case "insert-text": await textDialog(); break;
    case "insert-marker": await markerDialog(); break;

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
  return p;
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
let tileHome = null;

// forgetTiles throws the drawing away, which is what a new score, a new
// zoom or a new view needs.
function forgetTiles() {
  tileEls = new Map();
  tileText = new Map();
  tileHome = null;
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

// showTile puts one piece on the page.
async function showTile(plan, size, i) {
  if (tileEls.has(i)) return;
  // Claimed before it is awaited, so two passes over the same piece do not
  // both draw it.
  tileEls.set(i, null);
  const text = await fetchTile(plan, size, i);
  if (!tileHome || !tileHome.isConnected) return; // the drawing has gone
  if (tileEls.get(i) !== null) return;            // forgotten while we waited
  const g = document.createElementNS("http://www.w3.org/2000/svg", "g");
  g.setAttribute("data-tile", String(i));
  g.innerHTML = text;
  tileHome.appendChild(g);
  tileEls.set(i, g);
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
  }

  // And the next one along each way, fetched but not drawn, so that
  // reaching it is only the drawing of it.
  for (const i of [want.lo - 1, want.hi + 1]) {
    if (i < 0 || tileText.has(i)) continue;
    const end = plan.downTheLength ? size.height : size.width;
    if (i * plan.span >= end) continue;
    void fetchTile(plan, size, i);
  }
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
    box.hidden = true;
    box.textContent = "";
    sprintKey = "";
    return;
  }
  box.hidden = false;

  const rows = sprintRows();
  const key = rows.map((r) => r.part + "@" + r.top.toFixed(2)).join(",");
  if (key !== sprintKey) {
    sprintKey = key;
    box.textContent = "";
    for (const r of rows) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "staff-sprint";
      b.dataset.name = r.name;
      const what = "Continue editing " + r.name;
      b.title = what;
      b.setAttribute("aria-label", what);
      // An arrow with a wall at the end of it: go as far as this goes.
      b.textContent = "⇥";
      b.addEventListener("click", () => void sprintToStaffEnd(r.part, r.top));
      box.appendChild(b);
    }
  }
  placeSprints(rows, box, svg, wrap);
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
  const check = () => {
    frame = 0;
    // The buttons are pinned to the panel and their staves are not, so they
    // move whenever the view does.
    drawSprints();
    if (!state.tiles) return; // the whole score is in the page
    void ensureTiles();
  };
  const ask = () => {
    if (!frame) frame = requestAnimationFrame(check);
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

function setZoom(z) {
  state.zoom = Math.min(26, Math.max(5, Math.round(z)));
  $("#zoom").value = String(state.zoom);
  $("#zoom-readout").textContent = state.zoom + " px/space";
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

  if (state.info?.local) {
    let res = await fetch("/api/save?" + q.toString(), { method: "POST" });
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
      res = await fetch("/api/save?" + q.toString(), { method: "POST" });
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
    const res = await fetch("/api/save?" + p.toString(), { method: "POST" });
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
    const res = await fetch("/api/save?" + p.toString(), { method: "POST" });
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
    const res = await fetch("/api/save?" + p.toString(), { method: "POST" });
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
    const res = await fetch("/api/save?" + p.toString(), { method: "POST" });
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
async function startLive(from, master, t0) {
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

  live = { id: d.id, rate: d.rate, at: t0, ended: false, timer: null, master, font: d.font };
  pumpLive();
  return true;
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
    let res;
    try {
      res = await fetch("/api/play/chunk?id=" + encodeURIComponent(mine.id));
    } catch (_) {
      return; // stopped, or the server went away
    }
    if (mine !== live) return; // something else started while we waited
    if (!res.ok) {
      mine.ended = true;
      return;
    }
    const pcm = new Int16Array(await res.arrayBuffer());
    if (res.headers.get("X-Gscore-End")) mine.ended = true;
    if (!pcm.length) break;

    const frames = pcm.length / 2;
    const buf = audio.createBuffer(2, frames, mine.rate);
    const L = buf.getChannelData(0);
    const R = buf.getChannelData(1);
    for (let i = 0; i < frames; i++) {
      L[i] = pcm[2 * i] / 32768;
      R[i] = pcm[2 * i + 1] / 32768;
    }
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
  options.push({ value: "get", label: "Download another…" });
  if (state.info?.inBrowser) {
    options.push({ value: "file", label: "Open a .sf2 from this computer…" });
    if (pageFont.url) {
      options.push({ value: "forget", label: "Forget the kept instruments" });
    }
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
    const res = await fetch("/api/save?" + p.toString(), { method: "POST" });
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
  $("#record-font").onchange = async (e) => {
    record.font = e.target.value;
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
  const p = new URLSearchParams({ font: record.font });
  const res = await fetch("/api/soundfonts?" + p.toString());
  if (!res.ok) return;
  const d = await res.json();
  record.fonts = d.installed || [];
  record.catalogue = d.catalogue || [];
  record.collection = d.collection || null;
  record.voices = d.voices || [];
  record.dir = d.dir;
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
  $("#record-voices-note").textContent = missing
    ? "This soundfont has nothing for " + missing
      + (missing === 1 ? " instrument" : " instruments")
      + " below. They will be silent — choose another soundfont for them."
    : "What each staff is scored for, and what the soundfont will play it with.";
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
  options.push({ value: "url", label: "From a link to a .sf2 file…" });
  if (page) {
    options.push({ value: "file", label: "Open a .sf2 from this computer…" });
  }

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

  const query = new URLSearchParams(name ? { url, name } : { url });

  status("Downloading the soundfont…");
  const res = await fetch("/api/soundfont/fetch?" + query.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not download: " + (await res.text()).trim());
    return;
  }
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
  const res = await fetch("/api/hits?mode=" + state.mode);
  const d = await res.json();
  state.hits = d.hits || [];
  state.dynamics = d.dynamics || [];
  state.chordMarks = d.chords || [];
  state.tempos = d.tempos || [];
  state.hairpins = d.hairpins || [];
  state.arcs = d.arcs || [];
  state.tremolos = d.tremolos || [];
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
  await refreshScore();
  // Select what was just written, so a dynamic or a stem change lands on it.
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  const el = hit && (hit.elements || [])[sel.at - 1];
  if (el) sel.ref = el.ref;
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
  });
  const res = await fetch("/api/delete?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not delete: " + (await res.text()).trim());
    return;
  }
  sel.at -= 1;
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
  return !(sel.arc || sel.hairpin || sel.tremolo || sel.dynamic ||
    sel.chordSymbol || (sel.tempo !== undefined && sel.tempo !== null));
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

// cycleAccidental moves what is picked up one step sharper or flatter.
const cycleAccidental = (dir) =>
  mark("accidental", { cycle: dir > 0 ? "up" : "down", ...pickedHead() },
       (dir > 0 ? "Sharpened" : "Flattened") + onThatNotehead());

const setAccidental = (acc, said) =>
  mark("accidental", { accidental: acc, ...pickedHead() },
       (acc === "none" ? "Accidental removed" : "Written as " + said.toLowerCase()) +
       onThatNotehead());

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
  $("#sym-import").addEventListener("click", () => $("#sym-file").click());

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
    const d = await res.json();
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
    if (!$("#sym-name").value) {
      $("#sym-name").value = file.name.replace(/\.[^.]+$/, "").replace(/[^\w-]+/g, "");
    }
    drawRender();
    status("Imported " + file.name + " — edit and save");
    e.target.value = "";
  });

  $("#sym-save").addEventListener("click", async () => {
    const name = $("#sym-name").value.trim();
    if (!name) {
      status("The symbol needs a name");
      return;
    }
    const res = await fetch("/api/symbols", {
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
    if (!res.ok) {
      status("Could not save: " + (await res.text()).trim());
      return;
    }
    status("Saved the symbol " + name);
    $("#draw").close();
    await openPalette();
  });
}

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

  $("#palette-grid").innerHTML = list
    .map(
      (s, i) => `<button class="palette-item" data-symbol="${escapeHTML(s.name)}">
        <img src="${pics[i]}" alt="">
        <span>${escapeHTML(s.name)}</span>
      </button>`
    )
    .join("");
  $$("#palette-grid .palette-item").forEach((b) =>
    b.addEventListener("click", () => insertSymbol(b.dataset.symbol))
  );
  $("#palette").showModal();
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
async function saveScore(ask) {
  const here = state.info?.filename || "";

  // Nowhere to write to: the file comes back and the browser asks where it
  // goes, so saving in a page is always a save-as. The bytes are the ones
  // the desktop would have written — the drawings and the PDF of the page
  // are in there, which the plain exporter does not carry.
  if (!state.info?.local && !state.info?.native) {
    const res = await fetch("/api/save", { method: "POST" });
    if (!res.ok) {
      status("Could not save: " + (await res.text()).trim());
      return;
    }
    const name = await keepFile(res, "score.gscore");
    await loadInfo();
    status("Saved " + name);
    return;
  }

  if (!ask && here) {
    const back = await fetch("/api/save?path=" + encodeURIComponent(here),
                             { method: "POST" });
    if (back.ok) {
      const d = await back.json();
      await loadInfo();
      status("Saved " + d.path);
      return;
    }
    // gscore cannot write that format back. Ask where it should go instead.
  }
  let res = await fetch("/api/save", { method: "POST" });
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
    res = await fetch("/api/save?path=" + encodeURIComponent(where.trim()),
                      { method: "POST" });
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
async function settingsDialog() {
  const dlg = $("#settings");
  const pdf = $("#setting-score-pdf");
  const now = await (await fetch("/api/settings")).json().catch(() => ({}));
  pdf.checked = now.scorePDF !== false;

  await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve();
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });

  await fetch("/api/settings?scorePDF=" + (pdf.checked ? "1" : "0"),
              { method: "POST" });
  status(pdf.checked
    ? "Saved scores will carry a PDF — rename one .pdf to read it anywhere"
    : "Saved scores will hold the music only");
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
  });
  const all = useList ? options.map(asOption) : [];
  let onFilter = null;

  if (useList) {
    const render = (list, keep) => {
      select.innerHTML = list
        .map((o) => `<option value="${escapeHTML(o.value)}">${escapeHTML(o.label)}</option>`)
        .join("");
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
        render(q ? all.filter((o) => o.label.toLowerCase().includes(q)) : all, keep);
      };
      filter.addEventListener("input", onFilter);
    }
    $("#ask-label").setAttribute("for", useFilter ? "ask-filter" : "ask-select");
  } else {
    input.value = value;
    $("#ask-label").setAttribute("for", "ask-input");
  }

  return new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
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

async function setHeaderDialog() {
  const i = state.info || {};
  const title = await ask({ title: "Title", label: "Title", value: i.title || "" });
  if (title === null) return;
  const composer = await ask({
    title: "Composer", label: "Composer", value: i.composer || "",
  });
  if (composer === null) return;
  const p = new URLSearchParams({ title, composer });
  const res = await fetch("/api/header?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status("Title and composer set");
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

async function setInstrumentDialog() {
  const list = await (await fetch("/api/instruments")).json();
  const part = state.selection?.part ?? 0;
  const staff = state.info?.parts?.[part];
  const v = await ask({
    title: "Instrument",
    label: staff ? `What ${staff.name} sounds as` : "Instrument",
    value: staff?.instrument || "piano",
    note: "Used for playback. Type to search.",
    search: true,
    options: list.map((x) => ({ value: x.name, label: `${x.name} — ${x.family}` })),
  });
  if (v === null) return;
  await setStaffProperty("instrument", v);
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
  // The staff step is what is on the page; the clef turns it into a pitch.
  showFingering(midiForStep(part?.clef || "treble", el.steps[el.steps.length - 1]),
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
  // Diatonic index 0 is C0; seven steps to the octave.
  const octave = Math.floor(index / 7);
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
  markViewTicks();
  await refreshScore();
  status(what === "names"
    ? (on ? "Staff names shown" : "Staff names hidden")
    : (on ? "Measure numbers shown" : "Measure numbers hidden"));
}

function markViewTicks() {
  const n = $("#names-tick"), m = $("#numbers-tick");
  if (n) n.textContent = state.view.names ? "✓" : "";
  if (m) m.textContent = state.view.numbers ? "✓" : "";
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
  const v = await ask({
    title: "Text", label: "Words",
    note: "Written above the staff beside the note that is selected. " +
          "Drag it afterwards to move it.",
  });
  if (v === null) return;
  const sel = state.selection;
  const p = new URLSearchParams({
    part: String(sel.part), measure: String(sel.measure), at: String(sel.at),
    text: v,
  });
  const res = await fetch("/api/text?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not write it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(v === "" ? "Text removed" : "Text written");
}

// markerDialog puts a rehearsal mark on the current bar, across every staff.
async function markerDialog() {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  // The next letter, so marks run A, B, C without being typed each time.
  const v = await ask({
    title: `Rehearsal mark at measure ${barNumberOf(sel.measure)}`,
    label: "Mark",
    value: nextMarkerLetter(),
    note: "Written over every staff, because “from B” has to mean the same " +
          "bar to everyone. Leave it empty to take it away.",
  });
  if (v === null) return;
  const p = new URLSearchParams({ measure: String(sel.measure), text: v });
  const res = await fetch("/api/marker?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set it: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(v === "" ? "Mark removed" : "Mark " + v);
}

// nextMarkerLetter is what comes after the marks already written.
//
// Counted in the score rather than on the page. It used to count the single
// capital letters in the drawing, which worked while the whole score was
// drawn and stopped working when a long one began to be drawn a screenful
// at a time: the marks already written are mostly not on the page, so it
// offered A again and again.
function nextMarkerLetter() {
  const n = state.info?.markers ?? 0;
  return String.fromCharCode(65 + Math.min(25, n));
}

// joinStaves draws a brace or a bracket down the staves that are selected.
async function joinStaves(symbol) {
  const parts = state.info?.parts || [];
  if (parts.length < 2) {
    status("There is only one staff to join");
    return;
  }
  // Which staves: the ones touched by the selection, or all of them.
  let first = 0, last = parts.length - 1;
  if (state.multi.length) {
    first = Math.min(...state.multi.map((m) => m.part));
    last = Math.max(...state.multi.map((m) => m.part));
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
  // edit was made.
  if (d.tune !== undefined && d.tune !== state.info?.current) {
    await reload();
  } else {
    await reload();
  }
  markHistory(d.canUndo, d.canRedo);
  status(which === "undo" ? "Undone" : "Redone");
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
  const list = await (await fetch("/api/instruments")).json();
  const inst = await ask({
    title: "Add a staff",
    label: "What plays it",
    value: "flute",
    note: "The clef and the transposition follow from the instrument. " +
          "Choose the drum kit for a staff whose lines name drums. Type to search.",
    search: true,
    options: list.map((x) => ({ value: x.name, label: `${x.name} — ${x.family}` })),
  });
  if (inst === null) return;

  const p = new URLSearchParams({ instrument: inst, clef: clefFor(inst) });
  // Below the staff the cursor is on, which is where somebody adding a
  // staff while looking at one almost always means.
  if (state.selection) p.set("at", String((state.selection.part ?? 0) + 1));

  const res = await fetch("/api/addstaff?" + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not add it: " + (await res.text()).trim());
    return;
  }
  await reload();
  status("Added a staff for " + inst);
}

// clefFor is the clef an instrument is written in, so nobody has to tell a
// program that a double bass reads bass clef.
function clefFor(name) {
  const n = (name || "").toLowerCase();
  if (/drum|percussion/.test(n)) return "percussion";
  if (/bass|cello|bassoon|trombone|tuba|contrabass|baritone/.test(n)) return "bass";
  if (/viola/.test(n)) return "alto";
  return "treble";
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

  const path = "/print?what=" + encodeURIComponent(choice.what);

  // A native window has no print machinery and window.open returns nothing
  // without a UI delegate, so printing is handed to the real browser, which
  // has both. The page it opens prints itself.
  if (state.info?.local && state.info?.native) {
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
      });
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
}

/* ---------- opening a file ---------- */

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
let playStart = 0;

// The performance being rendered on the other side, when the score is being
// played through a soundfont rather than through the oscillators.
//
// null whenever it is not: everything else about playing — the clock, the
// cursor, the notes lighting up — is driven by playStart and does not know
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

  const lead = 0.1;
  const t0 = audio.currentTime + lead;
  // The clock is kept in the score's own time, not in time since the button
  // was pressed, so everything downstream — the cursor, the highlight, the
  // end of the piece — needs no telling that it started in the middle.
  playStart = performance.now() + lead * 1000 - from * 1000;
  voices = [];

  let sounded = 0;
  for (const ev of pb.events) {
    if (ev.start + ev.dur > from) sounded++;
  }

  // Real instruments where there are any, and the oscillators where there
  // are not. Which one it is changes nothing else about playing: both end up
  // as sound scheduled against the same clock.
  master.gain.value = 1;
  if (!(await startLive(from, master, t0))) {
    master.gain.value = 0.22; // keep simultaneous notes from clipping
    scheduleOscillators(pb.events, from, t0, master);
  }

  state.playing = true;
  // Starting is asking to be shown the music, so the page follows again
  // however it was left.
  userScrolled = false;
  $("#play-btn").classList.add("active");
  const where = from > 0
    ? ` from measure ${(cursorAt(pb.measures, from)?.measure ?? 0) + 1}`
    : "";
  status(`Playing${where} — ${sounded} notes at ${pb.tempo} bpm`);

  // The cursor goes up before a note sounds, at the start of the music, so
  // pressing play is visibly answered even when the piece opens with rests —
  // Mozart's Introitus rests for nine bars before anyone sings.
  makeCursor();
  const at0 = cursorAt(pb.measures, from);
  if (at0) {
    moveCursor(at0);
    followCursor(at0);
  }
  follow(pb);
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

// cursorAt converts a moment into a place on the page.
//
// The cursor follows the timeline, not the notes. A score has one clock and
// many staves, each with its notes in different places, so a cursor built from
// note positions lurches backwards and forwards between parts. Walking the
// measures and sliding across each one keeps it moving the way a conductor's
// eye does.
function cursorAt(measures, now) {
  if (!measures || !measures.length) return null;

  let m = measures[0];
  for (const cand of measures) {
    if (now >= cand.start) m = cand;
    else break;
  }
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
  let map = null;
  let made = null;
  const none = [];
  return (ref) => {
    const svg = currentSVG();
    if (!svg) return none;
    // Made again when the drawing is: a long score is drawn a screenful at
    // a time and the whole of it is replaced as the music moves along.
    if (!map || made !== svg) {
      map = new Map();
      made = svg;
      for (const n of svg.querySelectorAll("[data-ref]")) {
        const k = n.getAttribute("data-ref");
        const got = map.get(k);
        if (got) got.push(n);
        else map.set(k, [n]);
      }
    }
    return map.get(String(ref)) || none;
  };
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
    const now = (performance.now() - playStart) / 1000;
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
      moveCursor(pt);
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
    if (!sameSet(sounding, current)) {
      for (const ref of current) {
        if (sounding.has(ref)) continue;
        for (const n of byRef(ref)) n.classList.remove("playing");
      }
      for (const ref of sounding) {
        if (current.has(ref)) continue;
        for (const n of byRef(ref)) n.classList.add("playing");
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

  const max = Math.max(0, wrap.scrollWidth - wrap.clientWidth);
  const want = Math.min(max, Math.max(0, pt.x * z - wrap.clientWidth * cursorLead));
  // Set rather than animated: it is already a frame at a time, and asking
  // for a smooth scroll sixty times a second makes each one fight the last.
  wrap.scrollLeft = want;

  // Down the page the music does not move continuously — it steps from one
  // system to the next — so this moves only when the cursor has left the
  // one in view.
  const top = pt.top * z + off;
  const bottom = pt.bottom * z + off;
  if (top >= wrap.scrollTop && bottom <= wrap.scrollTop + wrap.clientHeight) {
    // The music is on screen. Whatever you were looking at, you are looking
    // at it again, so following takes over from here.
    userScrolled = false;
    return;
  }
  // Out of view, which is either the music having moved on or you having
  // gone to look at something else. Only the first is ours to answer.
  if (userScrolled) return;
  wrap.scrollTo({
    top: Math.max(0, top - wrap.clientHeight * 0.3),
    behavior: "smooth",
  });
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

// setupFollowRelease watches for the scrolling being somebody's own.
//
// The gestures, not the scroll event: following sets scrollLeft on every
// frame, so by the time it is a scroll event there is no telling whose it
// was.
function setupFollowRelease() {
  const wrap = $("#canvas-wrap");
  if (!wrap) return;
  const mine = (e) => {
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
  for (const v of voices) {
    try {
      v.stop();
    } catch (_) {
      /* already finished */
    }
  }
  voices = [];
  const svg = currentSVG();
  // Clear every notehead that was lit, on every staff.
  svg?.querySelectorAll(".playing").forEach((n) => n.classList.remove("playing"));
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
  const at = (performance.now() - playStart) / 1000;
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
    const img = e.target.closest ? e.target.closest("image[data-symbol]") : null;
    if (!img || e.button !== 0) return;
    const ref = Number(img.dataset.ref || 0);
    const where = elementByRef(ref);
    if (!where) return;

    e.preventDefault();
    e.stopPropagation();
    dragging = {
      img, where, name: img.dataset.symbol,
      startX: e.clientX, startY: e.clientY,
      x: parseFloat(img.getAttribute("x")), y: parseFloat(img.getAttribute("y")),
    };
    img.setPointerCapture(e.pointerId);
    img.classList.add("dragging");
    status("Moving " + dragging.name);
  });

  wrap.addEventListener("pointermove", (e) => {
    if (!dragging) return;
    // Moved live, so it is clear what is happening; the score is redrawn
    // from the model once the mark is let go.
    dragging.img.setAttribute("x", String(dragging.x + (e.clientX - dragging.startX)));
    dragging.img.setAttribute("y", String(dragging.y + (e.clientY - dragging.startY)));
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
    run: () => { if (pickedNotes()) void nudge(1); else moveCursorPitch(1); } },
  { id: "cursor-down", menu: "pitch-down", keys: "ArrowDown", group: "Selecting and moving",
    label: "Down a step — or move the selected notes down",
    run: () => { if (pickedNotes()) void nudge(-1); else moveCursorPitch(-1); } },
  { id: "cursor-up-octave", menu: "octave-up", keys: "Shift+ArrowUp", group: "Selecting and moving",
    label: "Up an octave",
    run: () => { if (pickedNotes()) void nudge(7); else moveCursorPitch(7); } },
  { id: "cursor-down-octave", menu: "octave-down", keys: "Shift+ArrowDown", group: "Selecting and moving",
    label: "Down an octave",
    run: () => { if (pickedNotes()) void nudge(-7); else moveCursorPitch(-7); } },
  { id: "nudge-up", keys: "Alt+ArrowUp", group: "Selecting and moving",
    label: "Move the note up a step", run: () => void nudge(1) },
  { id: "nudge-down", keys: "Alt+ArrowDown", group: "Selecting and moving",
    label: "Move the note down a step", run: () => void nudge(-1) },
  { id: "shift-earlier", keys: "Mod+ArrowLeft", group: "Selecting and moving",
    label: "Move the notes themselves earlier", run: () => void shiftPassage(-1) },
  { id: "shift-later", keys: "Mod+ArrowRight", group: "Selecting and moving",
    label: "Move the notes themselves later", run: () => void shiftPassage(1) },
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
  { id: "save", menu: "save", keys: "s", group: "The file",
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
function syncMenuKeys() {
  for (const b of KEY_BINDINGS) {
    if (!b.menu) continue;
    const el = document.querySelector(`[data-action="${b.menu}"] kbd`);
    if (el) el.textContent = shortcutText(keyFor(b), true);
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
      h.textContent = g.name;
      body.appendChild(h);
      for (const b of g.rows) {
        const chord = keyFor(b);
        const row = document.createElement("div");
        row.className = "shortcut";

        const key = document.createElement("kbd");
        key.textContent = shortcutText(chord);
        // A binding another one has taken over is still listed, because it
        // is still in the program; saying so is better than showing a key
        // that does nothing.
        if (bound.get(chord) !== b) {
          row.classList.add("shadowed");
          key.title = "Taken by " + bound.get(chord).label;
        }
        row.appendChild(key);

        const what = document.createElement("span");
        what.textContent = b.label;
        row.appendChild(what);

        const change = document.createElement("button");
        change.type = "button";
        change.className = "linklike";
        change.textContent = (state.keys || {})[b.id] ? "Change \u00b7 reset" : "Change";
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
  await mark("nudge", { steps: String(steps) },
             steps > 0 ? (steps === 7 ? "Up an octave" : "Up")
                       : (steps === -7 ? "Down an octave" : "Down"));
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
  const sel = selectionTargets();
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
  setupPanning();
  setupMarquee();
  setupSymbolDragging();
  setupFollowRelease();
  setupFullscreen();
  setupCanvasClicks();
  setupStripHover();
  setupVirtualDraw();
  setupDrawing();
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
  try {
    const kept = await (await fetch("/api/settings")).json();
    state.trackBarlines = kept.trackBarlines !== false;
    state.keys = kept.keys || {};
  } catch (_) {
    // Served without a disk behind it. The default stands.
  }
  markBarlineTick();
  loadMouseWrites();
  syncMenuKeys();

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
//   { gscore: "zoom",  value: 10 }
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
      case "load": await loadFromURL(String(m.value)); break;
      case "zoom": setZoom(Number(m.value) || state.zoom); break;
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
  const send = (what, extra) => {
    try {
      window.parent.postMessage(Object.assign({ gscore: what }, extra), "*");
    } catch (_) {
      /* a parent on another origin that refuses messages is not an error */
    }
  };
  send("ready", {
    title: state.info?.title || "",
    tunes: (state.info?.tunes || []).length,
  });
  window.addEventListener("gscore:ended", () => send("ended"));
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
      `height="8" rx="2" fill="var(--accent)"/>`);
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
        `fill="var(--accent)" fill-opacity="${shade.toFixed(2)}"/>`);
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

// typeHint is what the bar says it will take, shown under it.
const typeHint =
  "8abcdef — lengths 1 2 4 8 16 32, dotted 4. · letters a–g, with # for a " +
  "sharp, _ for a flat and n for a natural, and an octave: f#3 · " +
  "+3 −2 for an interval from the note before · r rest · | next bar · " +
  "tie, slur, stacc, ten, accent on everything after them, and off again · " +
  "or the name of any command";

// typeBudget is how many things one line may write.
//
// A line is a phrase somebody typed, not a program: that is what the console
// is for. A thousand would be a mistake, and a mistake that takes a thousand
// notes back one at a time is a mistake nobody forgives.
const typeBudget = 256;

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
    if (c === "|" || c === "r" || c === "R") {
      out.push(c);
      i++;
      continue;
    }
    return null; // not a run of music, so it is something else entirely
  }
  return out.length ? out : null;
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
  const words = [];
  for (const raw of String(line).trim().split(/\s+/)) {
    if (!raw) continue;
    const word = raw.toLowerCase();
    if (typeMarks[word] || typeNote(raw) || /^[+\u2212-]/.test(raw) ||
        /^\d+\.*$/.test(raw) || raw === "|") {
      words.push(raw);
      continue;
    }
    const run = splitRun(raw);
    if (run) {
      words.push(...run);
      continue;
    }
    words.push(raw);
  }

  for (const token of words) {

    if (token === "|") {
      out.push({ kind: "bar" });
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
      out.push({ kind: "note", interval: (m[1] === "+" ? n : -n), dur, dots });
      continue;
    }
    if (/^[rR]$/.test(token)) {
      out.push({ kind: "rest", dur, dots });
      continue;
    }
    const note = typeNote(token);
    if (note) {
      out.push({ kind: "note", pitch: note.pitch, dur, dots });
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
async function runTyping(line) {
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
        await cmd.run();
        wrote++;
        continue;
      }
      if (step.kind === "bar") {
        await nextBarForTyping();
        continue;
      }
      const where = await typeOne(step);
      if (where.error) {
        failed = where.error;
        break;
      }
      if (step.kind !== "rest") {
        written.push({ at: where.at, marks: Object.keys(on).filter((k) => on[k]) });
      }
      wrote++;
    }
    if (!failed) {
      const why = await applyTypedMarks(written);
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

// typeOne writes a single note or rest where the cursor is, and says what
// went wrong if it could not.
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
  if (step.pitch) p.set("pitch", step.pitch);
  if (step.drum) p.set("drum", String(step.drum));
  if (step.interval !== undefined) p.set("interval", String(step.interval));
  // A note with no pitch and no interval is one typed as a bare length,
  // which means "where the cursor is pointing" — the same as pressing the
  // key for it.
  if (!step.pitch && step.interval === undefined && step.kind !== "rest") {
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
    return { at: `${sel.part}:${d.measure}:${d.at - 1}` };
  }
  state.selection = { ...sel, at: sel.at + 1, ref: null, replace: false };
  return { at: `${sel.part}:${sel.measure}:${sel.at - 1}` };
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
function commandNamed(name) {
  const all = KEY_BINDINGS;
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
function consoleVerbs(out, said, read, question) {
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
    at(bar, staff) {
      spend();
      out.push({
        do: "at",
        bar: Math.max(0, Math.trunc(Number(bar)) - 1),
        staff: staff === undefined ? undefined : Math.max(0, Math.trunc(Number(staff)) - 1),
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
async function runConsole(source) {
  const queue = [], said = [];
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
  const verbs = consoleVerbs(queue, said, read, ask);

  // What the score looked like when the script began, for a script that
  // wants to know. It does not change as the queue is written, which the
  // help says in as many words.
  const look = {
    bars: measureCount(),
    staves: (state.info?.parts || []).length,
    staffNames: (state.info?.parts || []).map((p) => p.name || ""),
  };

  try {
    // Run as an async function, so that a script may await a question. The
    // writing verbs stay synchronous and queue; await is for ask and for
    // nothing else, which is why there is no await to forget in a loop.
    const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
    const run = new AsyncFunction(...Object.keys(verbs), "score", "Math", "JSON",
      `"use strict";\n${source}\n`);
    await run(...Object.values(verbs), look, Math, JSON);
  } catch (e) {
    return { said, error: String(e && e.message ? e.message : e) };
  }
  if (!queue.length) {
    return { said, wrote: 0 };
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
  return { said, wrote, error };
}

// doConsoleStep carries out one thing the script asked for.
async function doConsoleStep(step) {
  switch (step.do) {
    case "line":
      // The typing language, so everything it can say the console can say.
      return (await runTyping(step.line)) ? "" : "could not write " + step.line;
    case "at": {
      const sel = state.selection || { part: 0, measure: 0, step: 4 };
      const part = step.staff === undefined ? sel.part : step.staff;
      while (measureCount() <= step.bar) {
        await fetch("/api/addmeasure", { method: "POST" });
        await loadHits();
      }
      state.selection = {
        ...sel, part, measure: step.bar, at: 0, ref: null, replace: false,
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

// consoleHelp is the vocabulary, said in one line beside the buttons. The
// whole of it fits there, which is the point of there being so few words.
const consoleHelp =
  "note(\"c4\", 8) · rest(4) · chord([…], 2) · step(+3) · bar() · " +
  "at(bar, staff) · staff(\"flute\") · key(4) · dynamic(\"sfz\") · " +
  "drum(\"snare drum\") · write(\"8abc\") · notes(bar, staff) · " +
  "transpose(\"c4\", 4) · ask(q, […]) · say(…) · score.bars";

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
  if (help) help.textContent = consoleHelp;
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

  $("#console-run")?.addEventListener("click", async () => {
    const got = await runConsole(source?.value ?? "");
    const lines = [...got.said];
    if (got.error) lines.push(got.error);
    else lines.push(`Wrote ${got.wrote} thing${got.wrote === 1 ? "" : "s"}.`);
    consoleSay(lines.join("\n") || "Nothing to do.", !!got.error);
  });

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
