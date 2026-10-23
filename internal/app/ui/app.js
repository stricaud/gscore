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
  //
  // Writing is the default, so a new score can be typed into straight away
  // with no mode to enter first. What makes that bearable is that a click
  // landing on a note already there selects it instead of stacking another on
  // top — which is what anyone means by clicking a note they can see.
  // Alt-click always selects, and N turns writing off entirely for anyone who
  // prefers a strict mode.
  selection: null,

  hits: [],
  tool: { duration: "quarter", accidental: "none", rest: false, drum: null },
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
  // Drawing options that belong to the view rather than to the file.
  view: { names: false, numbers: false },
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
        break;
      case "acc":
        state.tool.accidental = item.acc;
        // An accidental means two things depending on what is in front of
        // you. With a note selected it belongs to that note and is written on
        // it now; with nothing selected it is a decision about the next note,
        // the way the palette's other tools are.
        if (hasSelection()) {
          await setAccidental(item.acc, item.label);
          markPaletteTool((label) => label === currentToolLabel());
          return;
        }
        break;
      default:
        state.tool.rest = false;
        state.tool.duration = item.tool;
    }
    markPaletteTool((label) => label === currentToolLabel());
    status(item.label);
    return;
  }

  if (item.apply === "dot") {
    status("Dotted notes are written by choosing the duration; the dot tool is not wired yet");
    return;
  }
  // An ending is a bracket over whole bars, so it acts on the measure the
  // cursor is in rather than on the note.
  // A drum is a tool, like a note value: it says what the next click writes.
  if (item.drum !== undefined) {
    state.tool.drum = item.drum;
    state.tool.rest = false;
    markPaletteTool((label) => label === item.label);
    status(item.label + " — click the drum staff to write it");
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

// currentToolLabel names the tool held, so the palette can show it.
function currentToolLabel() {
  if (state.tool.rest) {
    return { whole: "Whole rest", half: "Half rest", quarter: "Quarter rest",
             eighth: "Eighth rest", sixteenth: "16th rest", "32nd": "32nd rest" }[state.tool.duration];
  }
  return { whole: "Whole", half: "Half", quarter: "Quarter", eighth: "Eighth",
           sixteenth: "16th", "32nd": "32nd", "64th": "64th" }[state.tool.duration];
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
      play();
      break;
    case "play-from-start":
      await playFromStart();
      break;
    case "stop":
      stop();
      break;
    case "note-input":
      status("Click the staff to write. Hold shift to select instead.");
      break;
    case "insert-note":
      await insertAtSelection(false);
      break;
    case "insert-rest":
      await insertAtSelection(true);
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

    case "beam-join": await mark("beam", { beam: "join" }, "Beamed together"); break;
    case "beam-break": await mark("beam", { beam: "break" }, "Beam broken"); break;
    case "beam-auto": await mark("beam", { beam: "auto" }, "Beaming left to the engraver"); break;

    case "pitch-up": await nudge(1); break;
    case "pitch-down": await nudge(-1); break;
    case "octave-up": await nudge(7); break;
    case "octave-down": await nudge(-7); break;

    case "acc-sharp": await setAccidental("sharp", "Sharp"); break;
    case "acc-flat": await setAccidental("flat", "Flat"); break;
    case "acc-natural": await setAccidental("natural", "Natural"); break;
    case "acc-dsharp": await setAccidental("double sharp", "Double sharp"); break;
    case "acc-dflat": await setAccidental("double flat", "Double flat"); break;
    case "acc-none": await setAccidental("none", "None"); break;

    case "hairpin-cresc": await setHairpin("crescendo", "Crescendo"); break;
    case "hairpin-dim": await setHairpin("diminuendo", "Diminuendo"); break;
    case "hairpin-none": await setHairpin("none", "Hairpin removed"); break;

    case "octave-8va": await setOctave("8va", "8va"); break;
    case "octave-8vb": await setOctave("8vb", "8vb"); break;
    case "octave-15ma": await setOctave("15ma", "15ma"); break;
    case "octave-15mb": await setOctave("15mb", "15mb"); break;
    case "octave-none": await setOctave("none", "Octave line removed"); break;

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

    case "insert-text": await textDialog(); break;
    case "insert-marker": await markerDialog(); break;

    case "group-brace": await joinStaves("brace"); break;
    case "group-bracket": await joinStaves("bracket"); break;
    case "group-none": await joinStaves("none"); break;
    case "save":
      await saveScore();
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

function scoreURL(opts = {}) {
  const p = new URLSearchParams({ space: String(opts.space ?? state.zoom) });
  if (opts.tune !== undefined) p.set("tune", String(opts.tune));
  const mode = opts.mode ?? state.mode;
  if (mode === "strip") p.set("mode", "strip");
  if (opts.width) p.set("width", String(opts.width));
  p.set("_", String(Date.now()));
  return "/score.svg?" + p.toString();
}

async function drawScore() {
  if (state.showAll && (state.info?.tunes || []).length > 1) {
    await drawBook();
    return;
  }
  const res = await fetch(scoreURL());
  if (!res.ok) {
    status("Could not draw the score");
    return;
  }
  $("#canvas").innerHTML = await res.text();
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

  // In a native window the title bar already says gscore; repeating it
  // inside the window is what makes a web view look like a web page.
  document.body.classList.toggle("native", !!i.native);
  $("#version").textContent = "gscore " + i.version;
  $("#about-version").textContent = "Version " + i.version;
  $("#filename").textContent = i.filename || "untitled";

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
  stop();
  state.selection = null;
  state.multi = [];
  await loadInfo();
  await drawScore();
  await loadHits();
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
  const what = await askPrint(many, parts, "Export PDF", "Export");
  if (!what) return;

  const q = new URLSearchParams({ as: "pdf", what });

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

/* ---------- editing ---------- */

// Every edit changes the model on the server and the score is drawn again.
// Nothing is patched in place, because the drawing holds no music and the
// music holds no geometry; that separation is what makes this simple.

async function loadHits() {
  const res = await fetch("/api/hits?mode=" + state.mode);
  const d = await res.json();
  state.hits = d.hits || [];
  state.barlines = d.barlines || [];
  state.headers = d.headers || [];
  state.strips = d.strips || [];
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
function stripAt(x, y) {
  for (const st of state.strips) {
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
  let best = null;
  for (const h of state.hits) {
    if (x < h.x0 || x > h.x1) continue;
    // Allow a few spaces above and below for ledger lines.
    if (y < h.staffTop - 5 || y > h.staffTop + 9) continue;
    best = h;
    break;
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

  const clear = () => {
    currentSVG()?.querySelectorAll("rect.strip").forEach((r) => r.remove());
    document.body.classList.remove("over-strip");
    shown = null;
  };

  wrap.addEventListener("mousemove", (e) => {
    if (state.readOnly || state.playing) {
      if (shown !== null) clear();
      return;
    }
    const svg = currentSVG();
    if (!svg) return;
    const r = svg.getBoundingClientRect();
    const x = (e.clientX - r.left) / state.zoom;
    const y = (e.clientY - r.top) / state.zoom;

    const strip = stripAt(x, y);
    if (!strip) {
      if (shown !== null) clear();
      return;
    }
    if (shown === strip.measure) return;
    clear();
    shown = strip.measure;

    const z = state.zoom;
    const band = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    band.setAttribute("class", "strip");
    band.setAttribute("x", String(strip.x0 * z));
    band.setAttribute("y", String(strip.top * z));
    band.setAttribute("width", String((strip.x1 - strip.x0) * z));
    band.setAttribute("height", String((strip.bottom - strip.top) * z));
    band.setAttribute("rx", String(0.3 * z));
    svg.insertBefore(band, svg.firstChild);
    document.body.classList.add("over-strip");
  });

  wrap.addEventListener("mouseleave", () => {
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

    const hit = hitAt(x, y);
    if (!hit) {
      clearSelection();
      status("Ready");
      return;
    }

    const step = stepAt(hit, y);
    // Writing is what a click does. Holding shift is what selects — which
    // is a thing you do while you hold it, not a mode you are left in.
    const picking = e.shiftKey;
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
      const stackable =
        !picking && !state.tool.rest &&
        (el.kind === "note" || el.kind === "chord") &&
        !(el.steps || []).includes(step);
      if (stackable) {
        await addPitch(hit, el, step);
        return;
      }
      select(hit, el, step);
      return;
    }

    state.multi = [];
    state.selection = {
      part: hit.part, measure: hit.measure,
      at: insertIndex(hit, x), step: step, ref: null,
    };

    // Shift selects and never writes. gscore 0.0.9 made this a mode, with a
    // cursor tool to put down and pick up, and it was confusing for the
    // reason modes usually are: what a click does depended on something you
    // had to remember rather than on something you were doing.
    if (picking || state.readOnly) {
      markSelection();
      status(`Measure ${hit.measure + 1}`);
      return;
    }
    await insertAtSelection(state.tool.rest, false);
  });
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
  markSelection();
  status(state.multi.length === 1
    ? "1 selected"
    : `${state.multi.length} selected \u2014 measures ${a.measure + 1}\u2013${b.measure + 1}`);
}

// selectMeasure picks up a whole bar rather than a note in it.
//
// This is what a key or a meter needs: "from this bar on" is a thing you say
// about a bar, and there was no way to say it except by finding a note in the
// bar and selecting that instead.
function selectMeasure(part, measure) {
  state.multi = [];
  state.anchor = null;
  state.selection = { part, measure, at: 0, step: 0, ref: null };
  markSelection();
  const hit = state.hits.find((h) => h.part === part && h.measure === measure);
  const has = [];
  if (hit?.keyChange) has.push("a key change");
  if (hit?.timeChange) has.push("a meter change");
  status(`Measure ${measure + 1}` +
    (has.length ? ` — ${has.join(" and ")} here` : "") +
    " · Score ▸ Key or Time applies from here");
}

// select picks up an existing element, so the next command acts on it.
function select(hit, el, step) {
  state.anchor = { part: hit.part, measure: hit.measure, index: el.index };
  state.multi = [];
  // Which notehead of a chord was clicked, so a tie can be put on that one
  // rather than on all of them. Null means the element as a whole.
  let head = null;
  if (step !== undefined && (el.steps || []).includes(step)) {
    head = step;
  }
  state.selection = {
    part: hit.part,
    measure: hit.measure,
    // The marking commands act on the element before the cursor, so the
    // cursor sits just after what is selected.
    at: el.index + 1,
    step: 0,
    head,
    ref: el.ref,
  };
  markSelection();
  updateFingering();
  status(`Selected ${el.kind} ${el.index + 1} of measure ${hit.measure + 1}`);
}

function clearSelection() {
  state.selection = null;
  state.multi = [];
  state.anchor = null;
  markSelection();
}


async function insertAtSelection(rest, keepSelection) {
  const sel = state.selection;
  if (!sel) {
    status("Click a measure first");
    return;
  }
  const p = new URLSearchParams({
    part: String(sel.part),
    measure: String(sel.measure),
    at: String(sel.at),
    step: String(sel.step),
    dur: state.tool.duration,
    kind: rest ? "rest" : "note",
  });
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
  // A bar that is already full sends what is written next into the bar after
  // it, making one if there is none, so the answer says where the note ended
  // up rather than where it was aimed. The cursor follows it there, which is
  // what makes writing bar after bar a matter of carrying on clicking.
  const where = await res.json().catch(() => null);
  const moved = where && where.measure !== undefined && where.measure !== sel.measure;
  if (where && where.measure !== undefined) {
    sel.measure = where.measure;
    sel.at = where.at;
  } else {
    sel.at += 1;
  }
  sel.ref = null;
  await refreshScore();
  // Select what was just written, so a dynamic or a stem change lands on it.
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  const el = hit && (hit.elements || [])[sel.at - 1];
  if (el) sel.ref = el.ref;
  markSelection();
  let what = rest ? "Rest" : state.tool.duration + " note";
  // On a drum staff, say which drum. "quarter note" tells a drummer nothing
  // about what they just wrote.
  if (!rest && isDrumStaff(sel.part)) {
    const name = drumName(state.tool.drum, sel.step);
    if (name) what = name + ", " + state.tool.duration;
  }
  status(moved ? `${what} — measure ${sel.measure + 1}` : what + " inserted");
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
  const svg = currentSVG();
  if (!svg) return;
  svg.querySelectorAll("rect.selection").forEach((r) => r.remove());
  svg.querySelectorAll(".selected").forEach((n) => n.classList.remove("selected"));

  const sel = state.selection;
  if (!sel) return;
  const z = state.zoom;

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
        top: hit.staffTop - 1.6, bottom: hit.staffTop + STAFF_HEIGHT + 1.6,
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
    return;
  }

  // Nothing picked, but a measure is current: outline the measure instead.
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  if (!hit) return;
  svg.insertBefore(
    selectionRect(hit.x0, hit.staffTop - 1.6, hit.x1 - hit.x0, 7.2, z),
    svg.firstChild
  );
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
  }
  const res = await fetch(`/api/${endpoint}?` + p.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not apply: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(said);
}

// hasSelection says whether a note is picked up, rather than merely a place
// in a measure being remembered.
function hasSelection() {
  if (state.multi.length) return true;
  const sel = state.selection;
  return !!(sel && sel.ref !== null && sel.ref !== undefined);
}

const setAccidental = (acc, said) =>
  mark("accidental", { accidental: acc },
       acc === "none" ? "Accidental removed" : "Written as " + said.toLowerCase());

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
  status(none ? `Cleared at measure ${sel.measure + 1}`
              : `${label.split(" \u2014 ")[0]} at measure ${sel.measure + 1}`);
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
async function setTie(what, head) {
  if (what !== "none" && state.multi.length < 2) {
    status("A tie joins two notes — click one, then shift-click the next");
    return;
  }
  const params = { tie: what };
  if (head && state.selection?.head !== null && state.selection?.head !== undefined) {
    params.step = String(state.selection.head);
  }
  await mark("tie", params,
    what === "none" ? "Tie removed"
      : params.step !== undefined ? "Tied that notehead" : "Tied");
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

async function saveScore() {
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

// ask puts one question and returns the answer, or null if it was dismissed.
//
// It replaces prompt(), which does not exist in a native window: the web
// views used by the desktop builds implement no JavaScript dialog delegate,
// so prompt() quietly returns null and whatever asked for the value does
// nothing at all.
function ask({ title, label, value = "", note = "", options = null, search = false }) {
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
    select.size = useFilter ? Math.min(10, all.length) : 0;
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
      resolve((useList ? select.value : input.value).trim());
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
    // Ready to type, which is the point of a one-question dialog.
    setTimeout(() => (useFilter ? filter : useList ? select : input).focus(), 0);
  });
}

// The things that belong to the score or the staff rather than to a note.
// Each asks for one value and sends it; nothing here is worth a panel.

async function setTempoDialog() {
  const now = state.info?.tempo || 120;
  const v = await ask({
    title: "Tempo", label: "Beats per minute", value: String(now),
    note: "Counted in quarter notes.",
  });
  if (v === null) return;
  const bpm = parseInt(v, 10);
  if (!Number.isFinite(bpm)) {
    status("That is not a tempo");
    return;
  }
  const res = await fetch(`/api/tempo?bpm=${bpm}&unit=quarter`, { method: "POST" });
  if (!res.ok) {
    status("Could not set the tempo: " + (await res.text()).trim());
    return;
  }
  await refreshScore();
  status(bpm + " bpm");
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
      ? `From measure ${measure + 1} to the end, on every staff.`
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
    status(`The meter change at measure ${measure + 1} is gone`);
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
    ? `${meter} from measure ${measure + 1}, on every staff`
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
    title: `Barline after measure ${bar.measure + 1}`,
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
    ],
  });
  if (v === null) return;

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
    title: `Rehearsal mark at measure ${sel.measure + 1}`,
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

// nextMarkerLetter guesses what comes next by counting what is drawn.
function nextMarkerLetter() {
  const svg = currentSVG();
  let n = 0;
  svg?.querySelectorAll("text.t").forEach((t) => {
    if (/^[A-Z]$/.test(t.textContent.trim())) n++;
  });
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
    pad.addEventListener("click", () => pickDrum(d.key, true));
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
  $("#drumpad").hidden = false;
}

// pickDrum chooses what the next note written on a drum staff will be.
function pickDrum(key, say) {
  state.tool.drum = key;
  state.tool.rest = false;
  for (const pad of $$("#drumpad .pad")) {
    pad.classList.toggle("on", Number(pad.dataset.drum) === key);
  }
  if (say) {
    const d = (state.kit?.pads || []).find((p) => p.key === key);
    status((d ? d.name : "Drum") + " — click the drum staff to write it");
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
  pickDrum(pad.key, false);
  await insertAtSelection(false);
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
  const part = state.selection?.part ?? 0;
  const name = state.info?.parts?.[part]?.name || "that staff";
  const sure = await ask({
    title: "Remove a staff", label: "Type the name to remove it",
    note: `This takes away ${name} and the music on it.`,
  });
  if (sure === null) return;
  if (sure.trim().toLowerCase() !== name.toLowerCase()) {
    status("Left it alone — the name did not match");
    return;
  }
  const res = await fetch("/api/removestaff?part=" + part, { method: "POST" });
  if (!res.ok) {
    status("Could not remove it: " + (await res.text()).trim());
    return;
  }
  await reload();
  status("Removed " + name);
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
  // Which one the staff is in now, so the picker opens showing the answer.
  const current = keyOptions.find((k) => staff && staff.key.startsWith(
    k.fifths === 0 ? "\u266e" : Math.abs(k.fifths) + (k.fifths > 0 ? "\u266f" : "\u266d")
  ))?.fifths;

  // The pictures are fetched and put straight into the page rather than
  // loaded as images. An SVG in an <img> is an isolated document: it cannot
  // reach /font.otf, so every glyph in it comes out as a missing-character
  // box. Inline, it uses the page's own font.
  const drawings = await Promise.all(
    keyOptions.map((k) =>
      fetch(`/keysig.svg?fifths=${k.fifths}&clef=${clef}&space=9`).then((r) => r.text())
    )
  );

  $("#keysig-grid").innerHTML = keyOptions
    .map(
      (k, i) => `<button class="keysig-option" data-fifths="${k.fifths}"
                    ${k.fifths === current ? 'aria-current="true"' : ""}>
        <span class="keysig-art">${drawings[i]}</span>
        <span>${k.major} / ${k.minor}</span>
      </button>`
    )
    .join("");

  // Where it takes effect. A key set from bar nine looks exactly like a key
  // that did nothing, unless something says which bar it was.
  const measure = at !== undefined ? at : (state.selection?.measure ?? 0);
  const where = $("#keysig-where");
  if (where) {
    where.textContent = measure > 0
      ? `From measure ${measure + 1} to the end, on every staff.`
      : "For the whole piece, on every staff.";
  }

  // Offer to take a change away only where there is one.
  const hit = state.hits.find(
    (h) => h.part === partIndex && h.measure === measure);
  const remove = $("#keysig-remove");
  if (remove) remove.hidden = !(measure > 0 && hit?.keyChange);

  const dlg = $("#keysig");
  $$("#keysig-grid .keysig-option").forEach((b) =>
    b.addEventListener("click", () => {
      dlg.returnValue = "picked:" + b.dataset.fifths;
      dlg.close();
    })
  );

  const result = await new Promise((resolve) => {
    const done = () => {
      dlg.removeEventListener("close", done);
      resolve(dlg.returnValue);
    };
    dlg.addEventListener("close", done);
    dlg.returnValue = "";
    dlg.showModal();
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
    status(`The key change at measure ${measure + 1} is gone`);
    return;
  }
  if (!result.startsWith("picked:")) return;

  const q = new URLSearchParams({
    part: String(partIndex), fifths: result.slice(7),
  });
  if (measure > 0) q.set("at", String(measure));
  const res = await fetch("/api/setkey?" + q.toString(), { method: "POST" });
  if (!res.ok) {
    status("Could not set the key: " + (await res.text()).trim());
    return;
  }
  await reload();
  status(measure > 0
    ? `Key signature from measure ${measure + 1}, on every staff`
    : "Key signature changed, on every staff");
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
  const what = await askPrint(many, parts, "Print", "Print");
  if (!what) return;

  const path = "/print?what=" + encodeURIComponent(what);

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
function askPrint(many, parts, title, verb) {
  const dlg = $("#print");
  dlg.querySelector("h2").textContent = title || "Print";
  dlg.querySelector(".primary").textContent = verb || "Print";
  $("#print-book-row").hidden = !many;
  $("#print-parts-row").hidden = !parts;
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
      resolve(picked ? picked.value : "tune");
    };
    dlg.addEventListener("close", done);
    dlg.showModal();
  });
}

/* ---------- opening a file ---------- */

async function openFile(file) {
  if (!file) return;
  status("Opening " + file.name + "…");
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
        await reopen();
        status("Opened " + d.filename);
        return;
      }
    } catch (_) {
      /* fall through to the browser picker */
    }
  }
  $("#file-input").click();
}

function setupFileInput() {
  $("#file-input").addEventListener("change", async (e) => {
    await openFile(e.target.files[0]);
    e.target.value = "";
  });
  const wrap = $("#canvas-wrap");
  wrap.addEventListener("dragover", (e) => e.preventDefault());
  wrap.addEventListener("drop", async (e) => {
    e.preventDefault();
    await openFile(e.dataTransfer.files[0]);
  });
}

/* ---------- playback ---------- */

let audio = null;
let voices = [];
let rafID = null;
let playStart = 0;
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
function sound(ev, start, master) {
  if (ev.drum) {
    hit(ev, start, master);
    return;
  }
  const v = VOICES[ev.family] || VOICES.sustained;
  const hz = midiToHz(ev.midi);

  // Two notes of the same pitch in a row have to be heard as two. What
  // separates them is not the silence between — a hair of it is enough — but
  // the attack at the front of each: a soft ramp makes the second note sound
  // like the first one still ringing. So every note gets a short gap and its
  // family's own transient.
  const gap = Math.min(0.09, Math.max(0.035, ev.dur * 0.12));
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
async function play(opts) {
  if (state.playing) stop();

  // In a book, play the tune being looked at. Scrolling down to a tune and
  // pressing space is how somebody asks to hear it; without this the first
  // tune played instead, and since the cursor went with it the view jumped
  // back to the top of the book.
  //
  // A selection says which tune more precisely than the scroll position does,
  // so it wins — clicking a note in one tune and scrolling to another should
  // still play the note that was clicked.
  if (!(opts && opts.fromStart) && !hasSelection()) {
    const seen = visibleTune();
    if (seen !== null && seen !== state.info?.current) {
      await selectTune(seen);
    }
  }

  const pb = await (await fetch("/api/playback")).json();
  if (!pb.events || pb.events.length === 0) {
    status("Nothing to play");
    return;
  }

  // Where to begin. A note played more than once — anything inside a repeat
  // — is started at the first time through, which is the one being looked at.
  let from = 0;
  if (!(opts && opts.fromStart) && hasSelection()) {
    const ref = state.multi.length ? orderedSelection()[0].ref : state.selection.ref;
    const ev = pb.events.find((x) => x.ref === ref);
    if (ev) from = ev.start;
  }
  // The cursor is placed from the engraved positions, so they have to be in
  // hand before it can be shown.
  await loadHits();

  if (!audio) {
    audio = new (window.AudioContext || window.webkitAudioContext)();
    noise = null;
  }
  await audio.resume();

  const master = audio.createGain();
  master.gain.value = 0.22; // keep simultaneous notes from clipping
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
    const end = ev.start + ev.dur;
    if (end <= from) continue; // already over by the time we start
    sounded++;
    if (ev.start >= from) {
      sound(ev, t0 + ev.start - from, master);
      continue;
    }
    // A note already ringing when we come in: take it up part way, for what
    // is left of it, rather than striking it again.
    sound({ ...ev, dur: end - from }, t0, master);
  }

  state.playing = true;
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
function measureBox(index) {
  let x0 = Infinity, x1 = -Infinity;
  let top = Infinity, bottom = -Infinity;
  for (const h of state.hits) {
    if (h.measure !== index) continue;
    x0 = Math.min(x0, h.x0);
    x1 = Math.max(x1, h.x1);
    top = Math.min(top, h.staffTop);
    bottom = Math.max(bottom, h.staffTop + STAFF_HEIGHT);
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

function systemSpan(measure) {
  let top = Infinity;
  let bottom = -Infinity;
  for (const h of state.hits) {
    if (h.measure !== measure) continue;
    top = Math.min(top, h.staffTop);
    bottom = Math.max(bottom, h.staffTop + STAFF_HEIGHT);
  }
  if (top === Infinity) {
    // The measure is not on this page; fall back to the first system.
    const h = state.hits[0];
    if (!h) return null;
    return { top: h.staffTop, bottom: h.staffTop + STAFF_HEIGHT };
  }
  return { top, bottom };
}

// follow moves the cursor and lights up whichever notehead is sounding. The
// engraver and the playback builder number elements identically, so an
// event's ref finds its glyph directly.
function follow(pb) {
  const svg = currentSVG();
  const total = pb.duration;

  const tick = () => {
    const now = (performance.now() - playStart) / 1000;
    const shown = Math.max(0, Math.min(now, total));
    $("#clock").textContent = clock(shown);

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
    for (const ev of pb.events) {
      if (shown >= ev.start && shown < ev.start + ev.dur) {
        sounding.add(ev.ref);
        if (lead === null || ev.part === fingering.staff) lead = ev;
      }
    }
    if (!sameSet(sounding, current)) {
      for (const ref of current) {
        if (sounding.has(ref)) continue;
        svg?.querySelectorAll(`[data-ref="${ref}"]`)
           .forEach((n) => n.classList.remove("playing"));
      }
      for (const ref of sounding) {
        if (current.has(ref)) continue;
        svg?.querySelectorAll(`[data-ref="${ref}"]`)
           .forEach((n) => n.classList.add("playing"));
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

function makeCursor() {
  const svg = currentSVG();
  if (!svg) return;
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
function moveCursor(pt) {
  if (!cursorEl || !pt) return;
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
  if (top < wrap.scrollTop || bottom > wrap.scrollTop + wrap.clientHeight) {
    wrap.scrollTo({
      top: Math.max(0, top - wrap.clientHeight * 0.3),
      behavior: "smooth",
    });
  }
}


function stop() {
  if (rafID) cancelAnimationFrame(rafID);
  rafID = null;
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
  cursorEl?.remove();
  cursorEl = null;
  lastMeasure = null;
  state.playing = false;
  $("#play-btn").classList.remove("active");
  $("#clock").textContent = "0:00";
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

    band = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    band.setAttribute("class", "marquee");
    band.setAttribute("y", String((hit.staffTop - 1.6) * state.zoom));
    band.setAttribute("height", String(7.2 * state.zoom));
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
      state.anchor = { part: f.part, measure: f.measure, index: f.index };
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

// Holding Shift turns the pointer into the selecting one, so the gesture is
// visible before it is used.
function setupShiftHint() {
  const apply = (on) => document.body.classList.toggle("shift-select", on);
  window.addEventListener("keydown", (e) => {
    if (e.key === "Shift") apply(true);
  });
  window.addEventListener("keyup", (e) => {
    if (e.key === "Shift") apply(false);
  });
  window.addEventListener("blur", () => apply(false));
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

function setupKeys() {
  document.addEventListener("keydown", (e) => {
    if (e.target.matches("input, textarea")) return;

    // Ctrl/Cmd with plus, minus or zero zooms; with C and V it is the
    // clipboard, where every other program puts it.
    if (e.ctrlKey || e.metaKey) {
      switch (e.key) {
        case "+": case "=":
          e.preventDefault(); setZoom(state.zoom + 1); return;
        case "-": case "_":
          e.preventDefault(); setZoom(state.zoom - 1); return;
        case "0":
          e.preventDefault(); setZoom(8); return;
        case "c": case "C":
          e.preventDefault(); void copySelection(); return;
        case "x": case "X":
          e.preventDefault(); void cutSelection(); return;
        case "v": case "V":
          e.preventDefault(); void pasteClipboard(); return;
        case "z":
          e.preventDefault(); void stepHistory("undo"); return;
        case "Z":
          // Shift with Z: the usual redo everywhere but Windows, which also
          // has Ctrl+Y, handled below.
          e.preventDefault(); void stepHistory("redo"); return;
        case "y": case "Y":
          e.preventDefault(); void stepHistory("redo"); return;
      }
      return;
    }

    // The arrow keys move the selected notes and the cursor.
    switch (e.key) {
      case "ArrowUp":
        e.preventDefault();
        void nudge(e.shiftKey ? 7 : 1);
        return;
      case "ArrowDown":
        e.preventDefault();
        void nudge(e.shiftKey ? -7 : -1);
        return;
      case "ArrowLeft":
        e.preventDefault();
        moveCursorBy(-1);
        return;
      case "ArrowRight":
        e.preventDefault();
        moveCursorBy(1);
        return;
    }

    // On a drum staff the letters type the kit. A groove is typed, not
    // clicked thirty-two times, and each pad shows the letter that writes it.
    // The rule is the cursor: while it is on a drum staff those letters mean
    // drums, and everywhere else they mean what they mean everywhere else.
    // Scoping it to the staff is what lets both sets of letters be the easy
    // ones to reach.
    if (!e.shiftKey && !e.altKey && state.selection &&
        isDrumStaff(state.selection.part) && drumPadForKey(e.key)) {
      e.preventDefault();
      void typeDrum(e.key);
      return;
    }

    switch (e.key) {
      case " ":
        e.preventDefault();
        if (e.shiftKey) {
          void playFromStart();
        } else {
          state.playing ? stop() : play();
        }
        break;
      case "Escape":
        if (state.playing) stop();
        else clearSelection();
        break;
      case "m": action("add-measure"); break;
      case "l": action("set-lyrics"); break;
      case "F2": e.preventDefault(); toggleFingering(); break;
      case "t": action("insert-text"); break;
      case "b": action("beam-join"); break;
      case "S": action("slur"); break;
      case "=": action("tie"); break;
      case "r": action("repeat-bar"); break;
      case "N": case "i":
        status("Click the staff to write. Hold shift to select instead.");
        break;
      case "u": action("stem-up"); break;
      case "d": action("stem-down"); break;
      case "Backspace": e.preventDefault(); action("delete"); break;
      case "n": action("new"); break;
      case "o": action("open"); break;
      case "e": action("save-svg"); break;
      case "P": action("save-pdf"); break;
      case "p": action("print"); break;
      case "s": action("save"); break;
      case "f": action("fullscreen"); break;
      case "F11":
        e.preventDefault();
        action("fullscreen");
        break;
      case "+": case "=": setZoom(state.zoom + 1); break;
      case "-": setZoom(state.zoom - 1); break;
      case "0": setZoom(8); break;
    }
  });
}

/* ---------- moving, copying and pasting ---------- */

// nudge moves what is selected up or down the staff.
//
// Steps, not semitones: a step up from F in G major is G, and from F sharp it
// is still G. The arrow keys are for moving a note to the right place on the
// staff, which is what a player reads.
async function nudge(steps) {
  if (!hasSelection()) {
    status("Click a note first");
    return;
  }
  await mark("nudge", { steps: String(steps) },
             steps > 0 ? (steps === 7 ? "Up an octave" : "Up")
                       : (steps === -7 ? "Down an octave" : "Down"));
}

// moveCursorBy steps the selection along the music, one element at a time,
// over barlines and on to the next measure.
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
    for (const el of h.elements || []) {
      line.push({ measure: h.measure, index: el.index, ref: el.ref });
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
  state.anchor = { part: sel.part, measure: p.measure, index: p.index };
  state.selection = {
    part: sel.part, measure: p.measure, at: p.index + 1, step: 0, ref: p.ref,
  };
  markSelection();
  scrollSelectionIntoView();
  status(`Measure ${p.measure + 1}, note ${p.index + 1}`);
}

// scrollSelectionIntoView keeps the cursor on screen as it walks the music.
function scrollSelectionIntoView() {
  const svg = currentSVG();
  if (!svg || !state.selection?.ref) return;
  const el = svg.querySelector(`[data-ref="${state.selection.ref}"]`);
  if (el && el.scrollIntoView) {
    el.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "smooth" });
  }
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
  if (!sel || sel.ref === null || sel.ref === undefined) return null;
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
      state.playing ? stop() : play();
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
  markPaletteTool((label) => label === currentToolLabel());
  setupMenus();
  setupFileInput();
  setupKeys();
  setupZoomGestures();
  setupPanning();
  setupMarquee();
  setupSymbolDragging();
  setupShiftHint();
  setupFullscreen();
  setupCanvasClicks();
  setupStripHover();
  setupDrawing();
  markMode();

  $("#tune-select").addEventListener("change", (e) => selectTune(Number(e.target.value)));
  $("#show-all").addEventListener("change", async (e) => {
    state.showAll = e.target.checked;
    await drawScore();
  });
  $("#zoom").addEventListener("input", (e) => setZoom(Number(e.target.value)));
  $("#drumpad-kit").addEventListener("change", (e) => void changeDrumKit(e.target.value));
  $("#play-btn").addEventListener("click", () => (state.playing ? stop() : play()));
  $("#stop-btn").addEventListener("click", stop);

  const opt = readOptions();
  if (opt.isPlayer) await applyPlayerMode(opt);
  if (opt.zoom) setZoom(opt.zoom);

  // As a page, the core is WebAssembly and is still starting.
  if (window.gscoreCoreReady) {
    status("Starting…");
    await window.gscoreCoreReady;
  }

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
    play, stop, loadFromURL,
    selectTune,
    isPlaying: () => state.playing,
    setZoom,
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
//   { gscore: "toggle" }
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
      case "toggle": state.playing ? stop() : await play(); break;
      case "tune": await selectTune(Number(m.value) || 0); break;
      case "load": await loadFromURL(String(m.value)); break;
      case "zoom": setZoom(Number(m.value) || state.zoom); break;
    }
  });
}

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
