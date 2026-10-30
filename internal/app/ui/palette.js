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
      // Brackets round whatever sign is already there. A bracketed
      // accidental is a reminder rather than an instruction — the note
      // sounds the same either way — so this says nothing about which
      // sign it is, and a note without one has nothing to bracket.
      { g: "\uE26A\uE26B", label: "In brackets", action: "acc-parens" },
      // No "none" button. Clicking the sharp on a note that is already
      // sharp takes the sharp off, which is the same thing said where
      // anybody would look for it.
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
    id: "lines",
    name: "Lines and endings",
    size: 18,
    columns: 3,
    items: [
      // A hairpin and an octave line cover a stretch of music, so they need
      // a stretch selected: click a note, then shift-click a later one.
      { g: "\uE53E", label: "Crescendo", apply: "hairpin", value: "crescendo",
        needsRange: true },
      { g: "\uE53F", label: "Diminuendo", apply: "hairpin", value: "diminuendo",
        needsRange: true },
      { text: "—", label: "No hairpin", apply: "hairpin", value: "none" },

      { g: "\uE1FD", label: "Slur", apply: "slur", value: "on", needsRange: true },
      { g: "\uE1FD", label: "Tie", apply: "tie", value: "on", needsRange: true },
      { text: "—", label: "No slur", apply: "slur", value: "none" },

      { g: "\uE511", label: "8va — an octave up", apply: "octave", value: "8va",
        needsRange: true },
      { g: "\uE51C", label: "8vb — an octave down", apply: "octave", value: "8vb",
        needsRange: true },
      { text: "—", label: "No octave line", apply: "octave", value: "none" },

      // An ending is a bracket over whole bars, so it acts on the measure
      // the cursor is in rather than on a note. Select a range of bars to
      // put one bracket over all of them.
      { text: "1.", label: "First time", ending: "1" },
      { text: "2.", label: "Second time", ending: "2" },
      { text: "3.", label: "Third time", ending: "3" },
      { text: "1.2.", label: "First and second time", ending: "1,2" },
      { text: "—", label: "No ending", ending: "" },
    ],
  },
  {
    id: "jumps",
    name: "Repeats and jumps",
    size: 20,
    columns: 3,
    items: [
      // The places a direction refers to. They are written at a bar and mean
      // nothing on their own: it is the direction further on that gives the
      // segno and the Fine their work to do.
      { g: "\uE047", label: "Segno — the sign a D.S. goes back to", sign: "segno" },
      { g: "\uE048", label: "Coda — where the music goes on after To Coda", sign: "coda" },
      { text: "Fine", label: "Fine — where the piece ends on the way back", sign: "fine" },
      { text: "To\u00A0Coda", label: "To Coda — leave here on the way back", sign: "tocoda" },
      { text: "—", label: "No sign here", sign: "none" },

      // The directions themselves, written at the end of the bar the cursor
      // is in. Each is taken once: a D.S. taken every time round is a chart
      // that never ends.
      { text: "D.C.", label: "Da capo — back to the beginning", jump: "dc" },
      { text: "D.C.F", label: "D.C. al Fine — back to the beginning, stop at Fine", jump: "dc-fine" },
      { text: "D.C.C", label: "D.C. al Coda — back to the beginning, then the coda", jump: "dc-coda" },
      { text: "D.S.", label: "Dal segno — back to the sign", jump: "ds" },
      { text: "D.S.F", label: "D.S. al Fine — back to the sign, stop at Fine", jump: "ds-fine" },
      { text: "D.S.C", label: "D.S. al Coda — back to the sign, then the coda", jump: "ds-coda" },
      { text: "—", label: "No direction here", jump: "none" },
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
    id: "drums",
    name: "Drum kit",
    size: 17,
    columns: 2,
    // Filled from the kit when the score has a drum staff. A drum staff's
    // lines name instruments rather than pitches, so the only way to know
    // that the top space is a hi-hat is to be shown — which is what a
    // printed drum key does for a player.
    items: [],
    custom: true,
  },
  {
    id: "symbols",
    name: "Your symbols",
    items: [], // filled from the symbol registry
    custom: true,
  },
];

// A notehead shape as the font draws it, for showing a drum in the palette.
const HEAD_GLYPH = {
  normal: "\uE0A4",
  x: "\uE0A9",
  "circle-x": "\uE0B3",
  triangle: "\uE0BE",
  diamond: "\uE0DB",
  slash: "\uE100",
};

// fillPaletteSection puts items into a section that is built at run time
// rather than written out above — the drum kit, and the symbols somebody has
// drawn. The section hides itself when there is nothing in it.
function fillPaletteSection(id, items, onAction, size) {
  const grid = document.getElementById("pal-" + id);
  if (!grid) return;
  const wrap = grid.closest(".pal");
  if (wrap) wrap.hidden = items.length === 0;
  grid.innerHTML = "";
  for (const item of items) {
    grid.appendChild(paletteButton(item, onAction, size));
  }
  fitGlyphs(grid);
}

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
    head.innerHTML = `<span class="pal-arrow">▾</span>`;
    head.append(t(section.name));
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
  fitGlyphs(box);
}

function paletteButton(item, onAction, size) {
  const b = document.createElement("button");
  b.className = "pal-item";
  // The tooltip is read, so it is translated. dataset.label is not: it is
  // how markPaletteTool knows which button is which, and a name that
  // changes with the language is a name that cannot be matched against.
  b.title = t(item.label);
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
    g.dataset.size = String(item.size || size || palGlyphSize);
    g.style.fontSize = g.dataset.size + "px";
    if (item.flip) g.classList.add("flip");
    b.appendChild(g);
  }
  b.addEventListener("click", () => onAction(item, b));
  return b;
}

// palGlyphSize is what a palette glyph is drawn at when its section does not
// say otherwise, in pixels of staff height.
const palGlyphSize = 19;

/*
 * Drawing a glyph so that all of it is there.
 *
 * A SMuFL glyph is drawn against a staff that is not there. Its ink reaches
 * far above and below the baseline — a treble clef spans seven spaces, a
 * stem three and a half — while the line box the browser gives it is one em
 * tall and centred on that baseline. Centre the line box in a square button
 * and most of the ink is outside it; the button clips, and what is left is a
 * note with its stem cut off, a barline with no ends and a treble clef with
 * no curl. Reported as "notes are being eaten by the top, no margin, same
 * with barlines and clefs".
 *
 * So each glyph is measured and then drawn in an SVG whose view is exactly
 * its own ink. The browser centres that, and nothing is left outside. The
 * size it is drawn at is still the section's, because a dot and a clef must
 * not come out the same size — fitting every glyph to the square would make
 * an augmentation dot as big as a treble clef. Fitting only caps it: the
 * glyph is its own size unless that would not fit, and then it is as big as
 * fits.
 */

// glyphInk is how far a glyph's ink reaches from the point it is drawn at,
// in ems. Null where it cannot be measured, which is the signal to leave the
// character as plain text.
const glyphInkCache = new Map();
function glyphInk(ch) {
  if (glyphInkCache.has(ch)) return glyphInkCache.get(ch);
  let ink = null;
  try {
    glyphInk.canvas = glyphInk.canvas || document.createElement("canvas");
    const ctx = glyphInk.canvas.getContext("2d");
    // Measured large and divided down, because the numbers come back in
    // whole pixels on some browsers and a glyph measured at 1px is a box
    // of zeroes.
    const at = 200;
    ctx.font = at + "px " + glyphFont;
    const m = ctx.measureText(ch);
    const left = m.actualBoundingBoxLeft, right = m.actualBoundingBoxRight;
    const up = m.actualBoundingBoxAscent, down = m.actualBoundingBoxDescent;
    const w = left + right, h = up + down;
    if ([left, right, up, down].every((v) => typeof v === "number" && isFinite(v)) &&
        w > 0 && h > 0) {
      ink = { x: -left / at, y: -up / at, w: w / at, h: h / at };
    }
  } catch (_) {
    // No canvas, or a browser that does not measure ink. The character
    // stays as it was, which is what happened before any of this.
  }
  // Cached only when the measurement can be trusted. The music font is
  // declared in the stylesheet and loaded when something needs it, so a
  // measurement taken before it arrives is a measurement of whatever the
  // browser fell back to — and these codepoints are in no other font, so it
  // comes back as nothing or as a tofu box. Caching that would make the
  // first bad answer permanent.
  if (glyphFontReady()) glyphInkCache.set(ch, ink);
  return ink;
}

// glyphFontReady says whether the music font is loaded and can be measured.
function glyphFontReady() {
  try {
    return !document.fonts || !document.fonts.check ||
      document.fonts.check('16px "' + glyphFont + '"');
  } catch (_) {
    return true; // no way to ask, so take the measurement as it comes
  }
}

const glyphFont = "Bravura";
const svgNS = "http://www.w3.org/2000/svg";

// markGap is the air between a notehead and the mark written under it, in
// ems. A quarter of a staff space, which is what an engraver leaves.
const markGap = 0.0625;

// markOn works out where a mark goes under the note it belongs to.
//
// By ink, not by a number per mark. Each glyph is measured, and the mark is
// hung a little below the bottom of the note and centred on it — so a
// staccato dot and a marcato wedge both land where they should without
// anyone writing down how tall either of them is.
//
// Below, because the note is drawn with its stem up and an articulation
// goes on the side away from the stem. That is what a printed score does
// and it is also what keeps the mark out of the stem's way.
function markOn(note, mark) {
  const dy = note.y + note.h + markGap - mark.y;
  const dx = (note.x + note.w / 2) - (mark.x + mark.w / 2);
  return { dx, dy };
}

// glyphSVG draws a glyph, or a note with a mark on it, in a box that is
// exactly their ink.
//
// size is what it would be drawn at as text, and cap how much of its button
// it may fill. Both are needed: the first keeps a dot a dot, and the second
// keeps a clef inside its square.
//
// The mark is drawn in the colour reserved for it, and the note it sits on
// in the colour of the text around it. gscore 0.0.9 drew its menu pictures
// that way and it is the whole reason they work: the black is the note the
// entry is about and the red is what the entry does to it, so the eye finds
// the entry it wants without reading a word.
function glyphSVG(ch, ink, size, cap, mark, markInk) {
  const parts = [{ ch, x: 0, y: 0, fill: "currentColor" }];
  let box = { x: ink.x, y: ink.y, w: ink.w, h: ink.h };
  if (mark && markInk) {
    const at = markOn(ink, markInk);
    parts.push({ ch: mark, x: at.dx, y: at.dy, fill: "var(--mark)" });
    const x0 = Math.min(box.x, markInk.x + at.dx);
    const y0 = Math.min(box.y, markInk.y + at.dy);
    const x1 = Math.max(box.x + box.w, markInk.x + at.dx + markInk.w);
    const y1 = Math.max(box.y + box.h, markInk.y + at.dy + markInk.h);
    box = { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
  }

  const svg = document.createElementNS(svgNS, "svg");
  svg.setAttribute("viewBox",
    `${box.x.toFixed(4)} ${box.y.toFixed(4)} ${box.w.toFixed(4)} ${box.h.toFixed(4)}`);
  // The drawing keeps its shape inside whatever box the cap leaves it.
  svg.setAttribute("preserveAspectRatio", "xMidYMid meet");
  svg.setAttribute("focusable", "false");
  svg.setAttribute("aria-hidden", "true");
  svg.style.width = (box.w * size).toFixed(2) + "px";
  svg.style.height = (box.h * size).toFixed(2) + "px";
  if (cap) {
    svg.style.maxWidth = cap;
    svg.style.maxHeight = cap;
  }
  for (const part of parts) {
    const t = document.createElementNS(svgNS, "text");
    t.setAttribute("x", part.x.toFixed(4));
    t.setAttribute("y", part.y.toFixed(4));
    t.setAttribute("font-size", "1");
    t.setAttribute("font-family", glyphFont);
    t.setAttribute("fill", part.fill);
    t.textContent = part.ch;
    svg.appendChild(t);
  }
  return svg;
}

// fitGlyph swaps a character for a drawing of itself that fits.
//
// It leaves the character alone where the ink cannot be measured, so a
// browser that does not answer the question still shows the palette it
// always showed.
function fitGlyph(el, cap) {
  const ch = el.dataset.glyph || el.textContent;
  if (!ch || el.firstElementChild) return;
  const mark = el.dataset.mark || "";
  const ink = glyphInk(ch);
  const markInk = mark ? glyphInk(mark) : null;
  if (!ink || (mark && !markInk)) {
    // Remembered, so that a later pass knows there is still something here
    // to draw even though the box is empty now.
    el.dataset.glyph = ch;
    return;
  }
  el.dataset.glyph = ch;
  el.textContent = "";
  el.appendChild(glyphSVG(ch, ink, Number(el.dataset.size) || palGlyphSize,
                          cap, mark, markInk));
}

// fitGlyphs waits for the music font and then fits every glyph on the page.
//
// After the font, because a glyph measured before it arrives is measured in
// whatever the browser fell back to, which for these codepoints is a box or
// nothing at all.
function fitGlyphs(root) {
  const run = () => {
    for (const el of (root || document).querySelectorAll(".pal-glyph")) {
      fitGlyph(el, "84%");
    }
    for (const el of (root || document).querySelectorAll(".menu-icon")) {
      fitGlyph(el, "100%");
    }
  };
  // Asked for by name, not waited for in general. document.fonts.ready
  // resolves when nothing is *pending*, and a font the page has declared
  // but not yet drawn anything in is not pending — so it resolved at once,
  // every glyph was measured in the fallback font, and the menus came up
  // with no pictures in them. Which menus depended on whether the palette
  // had happened to pull the font in first, which is why it was sometimes.
  try {
    if (document.fonts && document.fonts.load) {
      document.fonts.load('16px "' + glyphFont + '"').then(run, run);
      return;
    }
  } catch (_) {
    // An older browser with no font loading API. Measure and hope.
  }
  run();
}

// markPaletteTool shows which tool is held, since a tool is a mode and a mode
// has to be visible.
function markPaletteTool(predicate) {
  document.querySelectorAll(".pal-item").forEach((b) => {
    b.classList.toggle("selected", predicate(b.dataset.label));
  });
}
