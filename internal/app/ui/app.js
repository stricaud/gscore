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
  // once rather than one tune at a time.
  showAll: false,
  // Everything a dragged selection picked up, as staff:measure:index.
  multi: [],
};

/* ---------- toolbox ---------- */

const DURATIONS = [
  ["", "whole"], ["", "half"], ["", "quarter"],
  ["", "eighth"], ["", "sixteenth"], ["", "rest"],
];
const ACCIDENTALS = [
  ["", "natural"], ["", "sharp"], ["", "flat"],
  ["", "double sharp"], ["", "double flat"], ["", "none"],
];
const ARTICULATIONS = [
  ["", "staccato"], ["", "accent"], ["", "tenuto"],
  ["", "marcato"], ["", "fermata"], ["", "up bow"],
];

function buildTools(containerId, items, defaultIndex) {
  const box = $("#" + containerId);
  items.forEach(([glyph, name, icon], i) => {
    const b = document.createElement("button");
    b.className = "tool" + (glyph === "" && !icon ? " small" : "");
    if (icon) {
      const img = document.createElement("img");
      img.src = icon;
      img.alt = name;
      b.appendChild(img);
    } else {
      b.textContent = glyph || "—";
    }
    b.title = name;
    b.dataset.tool = name;
    if (i === defaultIndex) b.classList.add("selected");
    b.addEventListener("click", () => {
      box.querySelectorAll(".tool").forEach((t) => t.classList.remove("selected"));
      b.classList.add("selected");
      if (containerId === "durations") {
        state.tool.select = name === "select";
        state.tool.rest = name === "rest";
        if (!state.tool.select && !state.tool.rest) state.tool.duration = name;
        document.body.classList.toggle("tool-select", state.tool.select);
      } else if (containerId === "accidentals") {
        state.tool.accidental = name;
      }
      status(name);
    });
    box.appendChild(b);
  });
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

  // Clicking a tune makes it the one being worked on.
  $$("#canvas .tune").forEach((d) =>
    d.addEventListener("click", () => {
      const i = Number(d.dataset.tune);
      if (i !== state.info.current) selectTune(i);
    })
  );
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
  status(state.info.tunes?.[i]?.title || "Tune " + (i + 1));
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
  rows.push(["Tempo", i.tempo + " bpm"]);
  $("#score-info").innerHTML = rows
    .map(([k, v]) => `<div><span>${k}</span><b>${escapeHTML(v)}</b></div>`)
    .join("");

  $("#part-info").innerHTML = i.parts
    .map(
      (p, idx) => `<div class="part-card">
        <div class="name">${escapeHTML(p.name)}</div>
        <div><span>Clef</span><b>${p.clef}</b></div>
        <div><span>Key</span><button class="linklike" data-key-part="${idx}">${p.key}</button></div>
        <div><span>Time</span><b>${p.time}</b></div>
        <div><span>Measures</span><b>${p.measures}</b></div>
      </div>`
    )
    .join("");
  $$("[data-key-part]").forEach((b) =>
    b.addEventListener("click", () => openKeyPicker(Number(b.dataset.keyPart)))
  );

  const tunes = i.tunes || [];
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

async function exportSVG() {
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
    // A modified click is a pan gesture, handled elsewhere.
    if (e.ctrlKey || e.metaKey || panned) return;
    const svg = $("#canvas svg");
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

    const el = elementAt(hit, x, y);

    // A click landing on something already there picks it up, whichever tool
    // is held. Writing a note exactly on top of one is never what was meant.
    if (el) {
      select(hit, el);
      return;
    }

    state.multi = [];
    state.selection = {
      part: hit.part, measure: hit.measure,
      at: insertIndex(hit, x), step: stepAt(hit, y), ref: null,
    };

    if (state.tool.select) {
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
  const box = $("#durations");
  const want = wantSelect ? "select" : state.tool.duration;
  const b = box.querySelector(`[data-tool="${want}"]`);
  if (b) b.click();
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
  sel.at += 1;
  sel.ref = null;
  await refreshScore();
  // Select what was just written, so a dynamic or a stem change lands on it.
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  const el = hit && (hit.elements || [])[sel.at - 1];
  if (el) sel.ref = el.ref;
  markSelection();
  status(rest ? "Rest inserted" : state.tool.duration + " note inserted");
  void keepSelection;
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

function markSelection() {
  const svg = $("#canvas svg");
  if (!svg) return;
  svg.querySelectorAll("rect.selection").forEach((r) => r.remove());
  svg.querySelectorAll(".selected").forEach((n) => n.classList.remove("selected"));

  const sel = state.selection;
  if (!sel) return;

  if (state.multi.length) {
    for (const m of state.multi) {
      const el = svg.querySelector(`[data-ref="${m.ref}"]`);
      if (el) el.classList.add("selected");
    }
  } else if (sel.ref) {
    const el = svg.querySelector(`[data-ref="${sel.ref}"]`);
    if (el) el.classList.add("selected");
  }
  const hit = state.hits.find(
    (h) => h.part === sel.part && h.measure === sel.measure
  );
  if (!hit) return;
  const r = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  r.setAttribute("class", "selection");
  r.setAttribute("x", String(hit.x0 * state.zoom));
  r.setAttribute("y", String((hit.staffTop - 1) * state.zoom));
  r.setAttribute("width", String((hit.x1 - hit.x0) * state.zoom));
  r.setAttribute("height", String(6 * state.zoom));
  svg.insertBefore(r, svg.firstChild);
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
  $("#palette-grid").innerHTML = list
    .map(
      (s) => `<button class="palette-item" data-symbol="${escapeHTML(s.name)}">
        <img src="/symbol.png?name=${encodeURIComponent(s.name)}&_=${Date.now()}" alt="">
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

/* ---------- key signature picker ---------- */

// The picker shows each signature engraved on a staff rather than named, which
// is how gscore 0.0.9 did it with its treble_*.xpm pictures, and how Sibelius
// does it. You recognise two sharps faster than you decode the words "D major".
let keyOptions = null;

async function openKeyPicker(partIndex) {
  keyOptions = keyOptions || (await (await fetch("/api/keysignatures")).json());
  const clef = state.info?.parts?.[partIndex]?.clef || "treble";

  $("#keysig-grid").innerHTML = keyOptions
    .map(
      (k) => `<button class="keysig-option" data-fifths="${k.fifths}">
        <img src="/keysig.svg?fifths=${k.fifths}&clef=${clef}&space=7" alt="">
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
async function printScore() {
  status("Preparing to print…");
  const res = await fetch(scoreURL({ mode: "page", space: 6, width: 150 }));
  const svgText = await res.text();

  const w = window.open("", "_blank");
  if (!w) {
    status("The browser blocked the print window");
    return;
  }
  w.document.open();
  w.document.write(`<!doctype html>
<html><head><meta charset="utf-8">
<title>${escapeHTML(state.info?.title || "Score")}</title>
<style>
  @page { size: auto; margin: 12mm; }
  html, body { margin: 0; padding: 0; background: #fff; }
  /* The SVG is one tall drawing; letting it set its own height and flow from
     the top is what keeps it from being parked in the middle of page one. */
  svg { display: block; width: 100%; height: auto; }
</style></head>
<body>${svgText}</body></html>`);
  w.document.close();

  // Give the embedded font a moment to load, or the first page prints blank.
  const go = () => {
    w.focus();
    w.print();
  };
  if (w.document.fonts && w.document.fonts.ready) {
    w.document.fonts.ready.then(() => setTimeout(go, 150));
  } else {
    setTimeout(go, 600);
  }
  status("Ready");
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
    const osc = audio.createOscillator();
    const gain = audio.createGain();
    osc.type = "triangle";
    osc.frequency.value = midiToHz(ev.midi);

    const start = t0 + ev.start;

    // Two notes of the same pitch in a row have to be heard as two. What
    // separates them is not the silence between — a hair of it is enough —
    // but the attack at the front of each: a soft ramp makes the second note
    // sound like the first one still ringing. So every note gets a short
    // gap and a sharp transient, then eases back to a sustain.
    const gap = Math.min(0.09, Math.max(0.035, ev.dur * 0.12));
    const dur = Math.max(0.06, ev.dur - gap);
    const attack = 0.004;
    const decay = Math.min(0.07, dur * 0.3);
    const release = Math.min(0.05, dur * 0.35);
    const peak = 0.95;
    const sustain = 0.62;

    const g = gain.gain;
    g.setValueAtTime(0, start);
    g.linearRampToValueAtTime(peak, start + attack);
    g.linearRampToValueAtTime(sustain, start + attack + decay);
    g.setValueAtTime(sustain, start + dur - release);
    g.linearRampToValueAtTime(0, start + dur);

    osc.connect(gain);
    gain.connect(master);
    osc.start(start);
    osc.stop(start + dur + 0.02);
    voices.push(osc);
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
  const svg = $("#canvas svg");
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
  const svg = $("#canvas svg");
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
  const svg = $("#canvas svg");
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
    if (!state.tool.select || e.button !== 0 || e.ctrlKey || e.metaKey) return;
    const svg = $("#canvas svg");
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
    band.setAttribute("y", String((hit.staffTop - 1.5) * state.zoom));
    band.setAttribute("height", String(7 * state.zoom));
    svg.appendChild(band);
    e.preventDefault();
  });

  window.addEventListener("mousemove", (e) => {
    if (!band) return;
    const svg = $("#canvas svg");
    const r = svg.getBoundingClientRect();
    const x = (e.clientX - r.left) / state.zoom;
    const lo = Math.min(startX, x), hi = Math.max(startX, x);
    band.setAttribute("x", String(lo * state.zoom));
    band.setAttribute("width", String((hi - lo) * state.zoom));
  });

  window.addEventListener("mouseup", (e) => {
    if (!band) return;
    const svg = $("#canvas svg");
    const r = svg.getBoundingClientRect();
    const x = (e.clientX - r.left) / state.zoom;
    band.remove();
    band = null;

    const lo = Math.min(startX, x), hi = Math.max(startX, x);
    if (hi - lo < 0.5) return; // a click, not a drag

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
        state.playing ? stop() : play();
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
      case "+": case "=": setZoom(state.zoom + 1); break;
      case "-": setZoom(state.zoom - 1); break;
      case "0": setZoom(8); break;
    }
  });
}

async function main() {
  buildTools("durations", DURATIONS, 2);
  buildTools("accidentals", ACCIDENTALS, 5);
  buildTools("articulations", ARTICULATIONS, 0);
  setupMenus();
  setupFileInput();
  setupKeys();
  setupZoomGestures();
  setupPanning();
  setupMarquee();
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
  status("Ready");
}

main();
