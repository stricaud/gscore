/*
 * gscore - a musical score editor
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * The editor front end: menus, toolbox, the engraved score, and playback.
 * The score is drawn by the Go engraver and arrives as SVG, so what runs here
 * only has to present it, scroll it and make a noise.
 */

const $ = (sel) => document.querySelector(sel);
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
  tool: { duration: "quarter", accidental: "none", rest: false, select: false },
  // A file may hold many scores; showAll puts the whole book on screen at
  // once rather than one tune at a time. It is decided when a file is opened:
  // opening a book should show the book.
  showAll: false,
  showAllDecided: false,
  // Everything a dragged selection picked up, as staff:measure:index.
  multi: [],
};

/* ---------- toolbox ---------- */

// The palette lives in palette.js, which holds what is in it. This is what
// happens when one is clicked.
async function paletteAction(item) {
  if (item.tool) {
    switch (item.tool) {
      case "select":
        state.tool.select = true;
        state.tool.rest = false;
        break;
      case "rest":
        state.tool.select = false;
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
        state.tool.select = false;
        state.tool.rest = false;
        state.tool.duration = item.tool;
    }
    document.body.classList.toggle("tool-select", state.tool.select);
    markPaletteTool((label) => label === currentToolLabel());
    status(item.label);
    return;
  }

  if (item.apply === "dot") {
    status("Dotted notes are written by choosing the duration; the dot tool is not wired yet");
    return;
  }
  if (item.apply) {
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
  if (state.tool.select) return "Select";
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
    menu.addEventListener("mouseenter", () => {
      // Once one menu is open, sliding along the bar opens the others, the
      // way a menu bar is expected to behave.
      if ($(".menu.open")) {
        closeMenus();
        menu.classList.add("open");
      }
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
}

async function action(name) {
  switch (name) {
    case "new":
      await fetch("/api/new", { method: "POST" });
      await reload();
      status("New score");
      break;
    case "open":
      await openScore();
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
      selectTool(!state.tool.select);
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
    case "about":
      $("#about").showModal();
      break;
  }
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
      (p, idx) => `<div class="part-card">
        <div class="name">${escapeHTML(p.name)}</div>
        <div><span>Clef</span><b>${p.clef}</b></div>
        <div><span>Key</span><button class="linklike" data-key-part="${idx}">${p.key}</button></div>
        <div><span>Time</span><button class="linklike" data-time-part="${idx}">${p.time}</button></div>
        <div><span>Sounds as</span><button class="linklike" data-inst-part="${idx}">${escapeHTML(p.instrument || "piano")}</button></div>
        <div><span>Measures</span><b>${p.measures}</b></div>
      </div>`
    )
    .join("");
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

  // An importer that only understood part of a file says so here, rather than
  // leaving half a score on screen with no explanation.
  if (i.warning) status("Opened with warnings — " + i.warning);
}

async function reload() {
  stop();
  state.showAllDecided = false;
  state.selection = null;
  await loadInfo();
  await drawScore();
  await loadHits();
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

    const hit = hitAt(x, y);
    if (!hit) {
      clearSelection();
      status("Ready");
      return;
    }

    const step = stepAt(hit, y);
    const picking = state.tool.select || e.shiftKey;
    const el = picking ? elementAt(hit, x, y) : columnAt(hit, x);

    // A click landing on something already there picks it up — writing a note
    // exactly on top of one is never what was meant. But with a note tool
    // held, a click in the same column at a height nothing occupies stacks a
    // second pitch on what is there: that is how a chord is written. gscore
    // asked for Ctrl to be held for this; the column does the asking now.
    if (el) {
      const stackable =
        !picking && !state.tool.rest &&
        (el.kind === "note" || el.kind === "chord") &&
        !(el.steps || []).includes(step);
      if (stackable) {
        await addPitch(hit, el, step);
        return;
      }
      select(hit, el);
      return;
    }

    state.multi = [];
    state.selection = {
      part: hit.part, measure: hit.measure,
      at: insertIndex(hit, x), step: step, ref: null,
    };

    // The cursor tool selects and never writes — that is the whole point of
    // it, and of gscore testing Selection.object_type == CURSOR before
    // deciding what the mouse means. Shift asks for the same thing without
    // putting the tool down.
    if (picking) {
      markSelection();
      status(`Measure ${hit.measure + 1}`);
      return;
    }
    await insertAtSelection(state.tool.rest, false);
  });
}

// select picks up an existing element, so the next command acts on it.
function select(hit, el) {
  state.selection = {
    part: hit.part,
    measure: hit.measure,
    // The marking commands act on the element before the cursor, so the
    // cursor sits just after what is selected.
    at: el.index + 1,
    step: 0,
    ref: el.ref,
  };
  markSelection();
  status(`Selected ${el.kind} ${el.index + 1} of measure ${hit.measure + 1}`);
}

function clearSelection() {
  state.selection = null;
  state.multi = [];
  markSelection();
}

// selectTool switches the palette to the cursor, or back to writing.
function selectTool(wantSelect) {
  state.tool.select = wantSelect;
  document.body.classList.toggle("tool-select", wantSelect);
  markPaletteTool((label) => label === currentToolLabel());
  status(wantSelect ? "Select — click a note, or drag to select a range"
                    : "Writing — click the staff to add a note");
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
  const what = rest ? "Rest" : state.tool.duration + " note";
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
    p.set("sel", state.multi.map((m) => `${m.part}:${m.measure}:${m.index}`).join(","));
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

async function setTimeDialog() {
  const part = state.selection?.part ?? 0;
  const now = state.info?.parts?.[part]?.time || "4/4";
  const v = await ask({
    title: "Time signature", label: "Meter", value: now,
    note: "Written as 4/4 or 6/8. Add a c for the common and cut symbols.",
  });
  if (v === null || v === "") return;
  await setStaffProperty("time", v);
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

/* ---------- key signature picker ---------- */

// The picker shows each signature engraved on a staff rather than named, which
// is how gscore 0.0.9 did it with its treble_*.xpm pictures, and how Sibelius
// does it. You recognise two sharps faster than you decode the words "D major".
let keyOptions = null;

async function openKeyPicker(partIndex) {
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

  $$("#keysig-grid .keysig-option").forEach((b) =>
    b.addEventListener("click", async () => {
      const fifths = b.dataset.fifths;
      await fetch(`/api/setkey?part=${partIndex}&fifths=${fifths}`, { method: "POST" });
      $("#keysig").close();
      await reload();
      status("Key signature changed");
    })
  );

  $("#keysig").showModal();
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
  const what = await askPrint(many, parts);
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
function askPrint(many, parts) {
  const dlg = $("#print");
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
    return;
  }
  await reload();
  status("Opened " + file.name);
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
        await reload();
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
let current = null;
let cursorEl = null;

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

// sound plays one event. Everything it makes stops by itself and is collected
// in `voices` so that pressing stop can cut it short.
function sound(ev, start, master) {
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
  await play();
}

function scrollToTop() {
  const wrap = $("#canvas-wrap") || $("#canvas")?.parentElement;
  if (wrap && wrap.scrollTo) wrap.scrollTo({ top: 0, left: 0, behavior: "smooth" });
}

async function play() {
  if (state.playing) stop();

  const pb = await (await fetch("/api/playback")).json();
  if (!pb.events || pb.events.length === 0) {
    status("Nothing to play");
    return;
  }
  // The cursor is placed from the engraved positions, so they have to be in
  // hand before it can be shown.
  await loadHits();

  audio = audio || new (window.AudioContext || window.webkitAudioContext)();
  await audio.resume();

  const master = audio.createGain();
  master.gain.value = 0.22; // keep simultaneous notes from clipping
  master.connect(audio.destination);

  const lead = 0.1;
  const t0 = audio.currentTime + lead;
  playStart = performance.now() + lead * 1000;
  voices = [];

  for (const ev of pb.events) {
    sound(ev, t0 + ev.start, master);
  }

  state.playing = true;
  $("#play-btn").classList.add("active");
  status(`Playing — ${pb.events.length} notes at ${pb.tempo} bpm`);

  // The cursor goes up before a note sounds, at the start of the music, so
  // pressing play is visibly answered even when the piece opens with rests —
  // Mozart's Introitus rests for nine bars before anyone sings.
  makeCursor();
  const at0 = cursorAt(pb.measures, 0);
  if (at0) {
    moveCursor(at0);
    scrollToPoint(at0);
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
    x: box.x0 + (box.x1 - box.x0) * f,
    top: box.top,
    bottom: box.bottom,
    measure: m.index,
  };
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
      if (pt.measure !== lastMeasure) {
        scrollToPoint(pt);
        lastMeasure = pt.measure;
      }
    }

    // And which note is sounding, which is not the same question: a note
    // keeps ringing after the cursor has moved past its head.
    let active = null;
    for (const ev of pb.events) {
      if (shown >= ev.start && shown < ev.start + ev.dur) {
        active = ev;
        break;
      }
    }
    const ref = active ? active.ref : null;
    if (ref !== current) {
      if (current !== null) {
        svg?.querySelector(`[data-ref="${current}"]`)?.classList.remove("playing");
      }
      if (ref !== null) {
        svg?.querySelector(`[data-ref="${ref}"]`)?.classList.add("playing");
      }
      current = ref;
    }

    if (now >= total + 0.3) {
      stop();
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

function scrollToPoint(pt) {
  if (!pt) return;
  const wrap = $("#canvas-wrap");
  const z = state.zoom;
  const x = pt.x * z;
  const y = pt.top * z;
  // In the strip the music runs off to the right, so the view chases the
  // cursor rather than the other way round.
  if (x < wrap.scrollLeft + 40 || x > wrap.scrollLeft + wrap.clientWidth - 80) {
    wrap.scrollTo({ left: Math.max(0, x - wrap.clientWidth * 0.35), behavior: "smooth" });
  }
  if (y < wrap.scrollTop || y > wrap.scrollTop + wrap.clientHeight - 80) {
    wrap.scrollTo({ top: Math.max(0, y - wrap.clientHeight * 0.4), behavior: "smooth" });
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
  if (current !== null) {
    svg?.querySelector(`[data-ref="${current}"]`)?.classList.remove("playing");
    current = null;
  }
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
    // The cursor tool selects, and so does Shift with any tool. Ctrl and
    // Command are already panning and zooming, so Shift is the one left —
    // and holding it to select is what every other editor does anyway.
    if (!(state.tool.select || e.shiftKey)) return;
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
    state.selection = state.multi.length
      ? { part: state.multi[0].part, measure: state.multi[0].measure,
          at: state.multi[0].index + 1, step: 0, ref: state.multi[0].ref }
      : state.selection;
    markSelection();
    status(state.multi.length
      ? `${state.multi.length} selected`
      : "Nothing in that range");
  });
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

    // Ctrl/Cmd with plus, minus or zero zooms.
    if (e.ctrlKey || e.metaKey) {
      switch (e.key) {
        case "+": case "=":
          e.preventDefault(); setZoom(state.zoom + 1); return;
        case "-": case "_":
          e.preventDefault(); setZoom(state.zoom - 1); return;
        case "0":
          e.preventDefault(); setZoom(8); return;
      }
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
      case "N": case "i":
        selectTool(!state.tool.select);
        break;
      case "u": action("stem-up"); break;
      case "d": action("stem-down"); break;
      case "Backspace": e.preventDefault(); action("delete"); break;
      case "n": action("new"); break;
      case "o": action("open"); break;
      case "e": action("save-svg"); break;
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
  setupShiftHint();
  setupFullscreen();
  setupCanvasClicks();
  setupDrawing();
  markMode();

  $("#tune-select").addEventListener("change", (e) => selectTune(Number(e.target.value)));
  $("#show-all").addEventListener("change", async (e) => {
    state.showAll = e.target.checked;
    await drawScore();
  });
  $("#zoom").addEventListener("input", (e) => setZoom(Number(e.target.value)));
  $("#play-btn").addEventListener("click", () => (state.playing ? stop() : play()));
  $("#stop-btn").addEventListener("click", stop);
  $("#input-btn").addEventListener("click", () => selectTool(!state.tool.select));

  await reload();
  const n = (state.info?.tunes || []).length;
  status(n > 1 ? `${n} tunes in this file` : "Ready");
}

main();
