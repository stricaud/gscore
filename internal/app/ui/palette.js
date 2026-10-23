/*
 * gscore - a musical score editor
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * The palette.
 *
 * gscore's own toolbar held forty-nine items: every note value and rest, the
 * accidentals, the articulations, nine dynamics, the barlines, beam, tie, stem
 * up and down, the augmentation dot and the cursor. That list is the spine of
 * what is here, because it is what the program was designed around.
 *
 * It is arranged the way MuseScore arranges a palette — named sections that
 * open and close — rather than as one long strip of buttons, because fifty
 * items in a column is a list, not a tool. Sibelius groups by what a thing
 * does to a note; Finale groups by tool. Grouping by what you are trying to
 * write is the one that survives having a hundred items in it.
 *
 * Every entry is a SMuFL glyph, so the palette is drawn in the same font as
 * the score and shows exactly the mark that will appear on the page.
 */

// Each item: glyph, label, and what clicking it does.
//
//   tool      — picks what a click on the staff writes
//   apply     — sends a command for the current selection
//   measure   — changes something about the measure
//
// The glyphs are SMuFL codepoints; the names beside them are the canonical
// SMuFL names, so they can be checked against the font.
const PALETTE = [
  {
    id: "tools",
    name: "Tools",
    open: true,
    items: [
      { icon: "tool-select.png", label: "Select", tool: "select" },
      { g: "", label: "Whole", tool: "whole" },
      { g: "", label: "Half", tool: "half" },
      { g: "", label: "Quarter", tool: "quarter" },
      { g: "", label: "Eighth", tool: "eighth" },
      { g: "", label: "16th", tool: "sixteenth" },
      { g: "", label: "32nd", tool: "32nd" },
      { g: "", label: "64th", tool: "64th" },
      { g: "", label: "Augmentation dot", apply: "dot" },
    ],
  },
  {
    id: "rests",
    size: 17,
    name: "Rests",
    items: [
      { g: "", label: "Whole rest", tool: "rest", dur: "whole" },
      { g: "", label: "Half rest", tool: "rest", dur: "half" },
      { g: "", label: "Quarter rest", tool: "rest", dur: "quarter" },
      { g: "", label: "Eighth rest", tool: "rest", dur: "eighth" },
      { g: "", label: "16th rest", tool: "rest", dur: "sixteenth" },
      { g: "", label: "32nd rest", tool: "rest", dur: "32nd" },
    ],
  },
  {
    id: "accidentals",
    name: "Accidentals",
    items: [
      { g: "", label: "Natural", tool: "acc", acc: "natural" },
      { g: "", label: "Sharp", tool: "acc", acc: "sharp" },
      { g: "", label: "Flat", tool: "acc", acc: "flat" },
      { g: "", label: "Double sharp", tool: "acc", acc: "double sharp" },
      { g: "", label: "Double flat", tool: "acc", acc: "double flat" },
      { text: "—", label: "No accidental", tool: "acc", acc: "none" },
    ],
  },
  {
    id: "articulations",
    name: "Articulations",
    items: [
      { g: "", label: "Staccato", apply: "articulation", value: "staccato" },
      { g: "", label: "Staccatissimo", apply: "articulation", value: "staccatissimo" },
      { g: "", label: "Accent", apply: "articulation", value: "accent" },
      { g: "", label: "Tenuto", apply: "articulation", value: "tenuto" },
      { g: "", label: "Marcato", apply: "articulation", value: "marcato" },
      { g: "", label: "Marcato down", apply: "articulation", value: "marcato down" },
      { g: "", label: "Fermata", apply: "articulation", value: "fermata" },
      { g: "", label: "Up bow", apply: "articulation", value: "up-bow" },
      { g: "", label: "Down bow", apply: "articulation", value: "down-bow" },
      { text: "—", label: "Clear", apply: "articulation", value: "" },
    ],
  },
  {
    id: "dynamics",
    // Dynamics are wide rather than tall; three to a row at a smaller size.
    size: 16,
    columns: 3,
    name: "Dynamics",
    items: [
      { g: "", label: "pppp", apply: "dynamic", value: "pppp" },
      { g: "", label: "ppp", apply: "dynamic", value: "ppp" },
      { g: "", label: "pp", apply: "dynamic", value: "pp" },
      { g: "", label: "p", apply: "dynamic", value: "p" },
      { g: "", label: "mp", apply: "dynamic", value: "mp" },
      { g: "", label: "mf", apply: "dynamic", value: "mf" },
      { g: "", label: "f", apply: "dynamic", value: "f" },
      { g: "", label: "ff", apply: "dynamic", value: "ff" },
      { g: "", label: "fff", apply: "dynamic", value: "fff" },
      { g: "", label: "ffff", apply: "dynamic", value: "ffff" },
      { g: "", label: "sf", apply: "dynamic", value: "sf" },
      { text: "—", label: "No dynamic", apply: "dynamic", value: "none" },
    ],
  },
  {
    id: "stems",
    name: "Stems",
    size: 18,
    items: [
      // A note drawn the way its stem will point says what the button does;
      // a bare stem glyph says nothing at all.
      { g: "", label: "Stem up", apply: "stem", value: "up" },
      { g: "", label: "Stem down", apply: "stem", value: "down" },
      { text: "auto", label: "Automatic stem", apply: "stem", value: "auto" },
    ],
  },
  {
    id: "barlines",
    // Barlines are thin and tall; small enough and they all look alike.
    size: 22,
    name: "Barlines",
    items: [
      { g: "", label: "Single", measure: "barline", value: "single" },
      { g: "", label: "Double", measure: "barline", value: "double" },
      { g: "", label: "Final", measure: "barline", value: "final" },
      { g: "", label: "Open repeat", measure: "barline", value: "repeat-start" },
      { g: "", label: "Close repeat", measure: "barline", value: "repeat-end" },
    ],
  },
  {
    id: "clefs",
    // A clef is drawn around the whole staff and is several times the height
    // of a notehead; set at the same size it simply does not fit the square.
    size: 13,
    name: "Clefs",
    items: [
      { g: "", label: "Treble", staff: "clef", value: "treble" },
      { g: "", label: "Bass", staff: "clef", value: "bass" },
      { g: "", label: "Alto", staff: "clef", value: "alto" },
      { g: "", label: "Tenor", staff: "clef", value: "tenor" },
      { g: "", label: "Percussion", staff: "clef", value: "percussion" },
    ],
  },
  {
    id: "time",
    // A time signature is two digits stacked, so it is tall as well.
    size: 16,
    columns: 3,
    name: "Time signatures",
    items: [
      { g: "", label: "Common", staff: "time", value: "4/4c" },
      { g: "", label: "Cut", staff: "time", value: "2/2c" },
      { text: "2/4", label: "2/4", staff: "time", value: "2/4" },
      { text: "3/4", label: "3/4", staff: "time", value: "3/4" },
      { text: "4/4", label: "4/4", staff: "time", value: "4/4" },
      { text: "6/8", label: "6/8", staff: "time", value: "6/8" },
      { text: "9/8", label: "9/8", staff: "time", value: "9/8" },
      { text: "12/8", label: "12/8", staff: "time", value: "12/8" },
    ],
  },
  {
    id: "symbols",
    name: "Your symbols",
    items: [], // filled from the symbol registry
    custom: true,
  },
];

// buildPalette renders the sections into the sidebar.
function buildPalette(onAction) {
  const box = document.getElementById("palette-panel");
  box.innerHTML = "";

  for (const section of PALETTE) {
    const wrap = document.createElement("section");
    wrap.className = "pal";
    wrap.dataset.section = section.id;

    const head = document.createElement("button");
    head.className = "pal-head";
    head.innerHTML = `<span class="pal-arrow">▾</span>${section.name}`;
    head.addEventListener("click", () => {
      wrap.classList.toggle("closed");
      try {
        localStorage.setItem("pal:" + section.id, wrap.classList.contains("closed") ? "0" : "1");
      } catch (_) {
        /* a private window may refuse; the palette still works */
      }
    });
    wrap.appendChild(head);

    const grid = document.createElement("div");
    grid.className = "pal-grid";
    grid.id = "pal-" + section.id;
    if (section.columns) {
      grid.style.gridTemplateColumns = `repeat(${section.columns}, 1fr)`;
    }
    for (const item of section.items) {
      grid.appendChild(paletteButton(item, onAction, section.size));
    }
    wrap.appendChild(grid);

    let open = section.open !== false;
    try {
      const saved = localStorage.getItem("pal:" + section.id);
      if (saved !== null) open = saved === "1";
    } catch (_) {
      /* no stored preference */
    }
    if (!open) wrap.classList.add("closed");

    box.appendChild(wrap);
  }
}

function paletteButton(item, onAction, size) {
  const b = document.createElement("button");
  b.className = "pal-item";
  b.title = item.label;
  b.dataset.label = item.label;

  if (item.icon) {
    const img = document.createElement("img");
    img.src = item.icon;
    img.alt = item.label;
    b.appendChild(img);
  } else if (item.text) {
    b.textContent = item.text;
    b.classList.add("pal-text");
    // "12/8" does not fit a square sized for "f". Shrink by length rather
    // than clipping, which is what was happening.
    const n = item.text.length;
    b.style.fontSize = (n <= 1 ? 15 : n <= 2 ? 13 : n <= 3 ? 11 : 9.5) + "px";
  } else {
    // The glyph sits in a span so it can be centred on its own ink rather
    // than on a baseline that assumes a staff underneath it.
    const g = document.createElement("span");
    g.className = "pal-glyph";
    g.textContent = item.g;
    if (item.size || size) g.style.fontSize = (item.size || size) + "px";
    if (item.flip) g.classList.add("flip");
    b.appendChild(g);
  }
  b.addEventListener("click", () => onAction(item, b));
  return b;
}

// markPaletteTool shows which tool is held, since a tool is a mode and a mode
// has to be visible.
function markPaletteTool(predicate) {
  document.querySelectorAll(".pal-item").forEach((b) => {
    b.classList.toggle("selected", predicate(b.dataset.label));
  });
}
