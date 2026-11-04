/*
 * gscore - a musical score editor
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Fingering charts.
 *
 * A student reading a new piece does not have trouble with the notes; they
 * have trouble with the fingers. So when this is switched on, the note that is
 * sounding is also shown as a picture of the instrument's keys, and following
 * the music means watching your own hands move.
 *
 * The charts are drawn rather than drawn from: a flute is six open holes and a
 * few levers, a saxophone is a column of pearls and some side keys, and both
 * are more legible as a diagram than as a photograph. Each fingering is a list
 * of which keys are pressed, which is small enough to read and to correct.
 *
 * It is written as a plugin — a thing that watches playback and the selection
 * and draws beside them — so that adding a clarinet or a recorder later means
 * adding a chart, not touching the editor.
 */

/* ---------- the charts ---------- */

// A fingering is named by sounding pitch, as a note name with an octave:
// "C4" is middle C. Enharmonics are listed separately where the fingering
// differs and shared where it does not, because on a flute F sharp and G flat
// are the same fingers and on paper they are not the same note.

// Flute, the Boehm system.
//
// The keys, left hand first: the thumb (B or B-flat), then the three fingers
// of the left hand, then the little finger's G-sharp lever, then the three
// fingers of the right hand, and the right little finger's E-flat key, which
// is held down for almost everything — which is why it is listed and why
// beginners' low notes go wrong when they let it go.
const FLUTE_KEYS = ["Th", "T", "1", "2", "3", "G#", "4", "5", "6", "Eb", "C#", "C", "B"];

// Each entry lists what is pressed. An empty list is every key open.
const FLUTE = {
  "C4":  ["Th", "T", "1", "2", "3", "4", "5", "6", "Eb", "C#", "C"],
  "C#4": ["Th", "T", "1", "2", "3", "4", "5", "6", "Eb", "C#"],
  "D4":  ["Th", "T", "1", "2", "3", "4", "5", "6", "Eb"],
  "D#4": ["Th", "T", "1", "2", "3", "4", "5", "6", "Eb", "D#"],
  "E4":  ["Th", "T", "1", "2", "3", "4", "5", "Eb"],
  "F4":  ["Th", "T", "1", "2", "3", "4", "Eb"],
  "F#4": ["Th", "T", "1", "2", "3", "6", "Eb"],
  "G4":  ["Th", "T", "1", "2", "3", "Eb"],
  "G#4": ["Th", "T", "1", "2", "3", "G#", "Eb"],
  "A4":  ["Th", "T", "1", "2", "Eb"],
  "A#4": ["Th", "T", "1", "4", "Eb"],
  "B4":  ["Th", "T", "1", "Eb"],
  "C5":  ["Th", "T", "Eb"],
  "C#5": ["Eb"],
  "D5":  ["T", "1", "2", "3", "4", "5", "6", "Eb"],
  "D#5": ["T", "1", "2", "3", "4", "5", "6", "Eb", "D#"],
  "E5":  ["Th", "T", "1", "2", "3", "4", "5", "Eb"],
  "F5":  ["Th", "T", "1", "2", "3", "4", "Eb"],
  "F#5": ["Th", "T", "1", "2", "3", "6", "Eb"],
  "G5":  ["Th", "T", "1", "2", "3", "Eb"],
  "G#5": ["Th", "T", "1", "2", "3", "G#", "Eb"],
  "A5":  ["Th", "T", "1", "2", "Eb"],
  "A#5": ["Th", "T", "1", "4", "Eb"],
  "B5":  ["Th", "T", "1", "Eb"],
  "C6":  ["Th", "T", "Eb"],
  "C#6": ["Eb"],
  // The third octave is where the fingerings stop following a pattern; these
  // are the standard ones.
  "D6":  ["T", "2", "3", "4", "5", "Eb"],
  "D#6": ["T", "2", "3", "4", "Eb", "D#"],
  "E6":  ["Th", "1", "2", "4", "5", "Eb"],
  "F6":  ["Th", "1", "2", "4", "Eb"],
  "F#6": ["Th", "1", "2", "6", "Eb"],
  "G6":  ["Th", "1", "2", "Eb"],
  "G#6": ["Th", "1", "G#", "Eb"],
  "A6":  ["Th", "2", "G#", "Eb"],
  "A#6": ["Th", "2", "3", "G#", "Eb"],
  "B6":  ["T", "1", "2", "G#", "4", "Eb"],
  "C7":  ["T", "2", "G#", "4", "Eb"],
};

// Saxophone, which is the same for all of them: the alto, the tenor and the
// soprano are fingered alike and differ only in what comes out. The names are
// written pitches, so what is shown is what the player reads.
const SAX_KEYS = ["Oct", "1", "2", "3", "Bis", "4", "5", "6", "Eb", "C", "G#", "Low Bb", "Low B", "Side C"];

const SAX = {
  "Bb3": ["1", "2", "3", "4", "5", "6", "Low Bb"],
  "B3":  ["1", "2", "3", "4", "5", "6", "Low B"],
  "C4":  ["1", "2", "3", "4", "5", "6", "C"],
  "C#4": ["1", "2", "3", "4", "5", "6", "Low B", "C"],
  "D4":  ["1", "2", "3", "4", "5", "6"],
  "D#4": ["1", "2", "3", "4", "5", "6", "Eb"],
  "E4":  ["1", "2", "3", "4", "5"],
  "F4":  ["1", "2", "3", "4"],
  "F#4": ["1", "2", "3", "5"],
  "G4":  ["1", "2", "3"],
  "G#4": ["1", "2", "3", "G#"],
  "A4":  ["1", "2"],
  "A#4": ["1", "Bis"],
  "B4":  ["1"],
  "C5":  ["2"],
  "C#5": [],
  "D5":  ["Oct", "1", "2", "3", "4", "5", "6"],
  "D#5": ["Oct", "1", "2", "3", "4", "5", "6", "Eb"],
  "E5":  ["Oct", "1", "2", "3", "4", "5"],
  "F5":  ["Oct", "1", "2", "3", "4"],
  "F#5": ["Oct", "1", "2", "3", "5"],
  "G5":  ["Oct", "1", "2", "3"],
  "G#5": ["Oct", "1", "2", "3", "G#"],
  "A5":  ["Oct", "1", "2"],
  "A#5": ["Oct", "1", "Bis"],
  "B5":  ["Oct", "1"],
  "C6":  ["Oct", "2"],
  "C#6": ["Oct"],
  "D6":  ["Oct", "1", "2", "3", "4", "5", "6"],
  "D#6": ["Oct", "1", "2", "3", "4", "5", "6", "Eb"],
  "E6":  ["Oct", "1", "2", "3", "4", "5"],
  "F6":  ["Oct", "1", "2", "3", "4"],
};

// Which chart suits which instrument. Anything not here has no chart and the
// plugin simply says so rather than showing the wrong fingers.
const CHARTS = {
  flute:      { name: "Flute", keys: FLUTE_KEYS, map: FLUTE, draw: drawFlute },
  piccolo:    { name: "Piccolo", keys: FLUTE_KEYS, map: FLUTE, draw: drawFlute },
  "alto sax": { name: "Alto saxophone", keys: SAX_KEYS, map: SAX, draw: drawSax },
  "tenor sax": { name: "Tenor saxophone", keys: SAX_KEYS, map: SAX, draw: drawSax },
  "soprano sax": { name: "Soprano saxophone", keys: SAX_KEYS, map: SAX, draw: drawSax },
};

/* ---------- the plugin ---------- */

const fingering = {
  on: false,
  chart: null,
  last: null,
  // Which staff to follow. A score has many, and the student is learning one
  // instrument: showing the trumpet's fingers while the flute part plays
  // would be worse than showing none.
  staff: 0,
};

// fingeringChartFor picks the chart for a staff's instrument.
function fingeringChartFor(instrument) {
  return CHARTS[(instrument || "").toLowerCase()] || null;
}

// fingeringFor looks a note up, trying the enharmonic spelling when the one
// written is not in the chart: a chart that lists F sharp and is handed a
// G flat should still answer.
function fingeringFor(chart, midi) {
  const sharps = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
  const name = sharps[((midi % 12) + 12) % 12] + (Math.floor(midi / 12) - 1);
  return { name, keys: chart.map[name] || null };
}

// showFingering draws the fingering for one sounding note.
//
// The pitch is the written one, because what a player puts their fingers on is
// what is in front of them: an alto saxophone reading a written A fingers an A
// whatever comes out of the bell.
function showFingering(writtenMidi, instrument) {
  const box = document.getElementById("fingering");
  if (!box || !fingering.on) return;

  const chart = fingeringChartFor(instrument);
  if (!chart) {
    box.innerHTML =
      `<div class="fing-none">No fingering chart for ${escapeHTML(instrument || "this staff")}.<br>` +
      `<span class="small">Flute, piccolo and the saxophones have one.</span></div>`;
    return;
  }
  if (writtenMidi === null) {
    box.innerHTML = `<div class="fing-none">${escapeHTML(chart.name)}<br>` +
      `<span class="small">Play, or click a note.</span></div>`;
    return;
  }

  const f = fingeringFor(chart, writtenMidi);
  if (fingering.last === f.name) return;
  fingering.last = f.name;

  const pressed = new Set(f.keys || []);
  box.innerHTML =
    `<div class="fing-note">${escapeHTML(f.name.replace("#", "♯"))}</div>` +
    (f.keys
      ? chart.draw(pressed)
      : `<div class="fing-none small">Out of the range of this chart.</div>`);
}

/* ---------- the drawings ---------- */

// A hole is drawn filled when it is covered and open when it is not, which is
// how every fingering chart ever printed shows it, so nobody has to be told.
function hole(on, label, cls) {
  return `<span class="fing-hole${on ? " on" : ""}${cls ? " " + cls : ""}"` +
         ` title="${escapeHTML(label)}"></span>`;
}

function drawFlute(p) {
  return `<div class="fing-chart flute">
    <div class="fing-row">
      <span class="fing-label">L thumb</span>
      ${hole(p.has("Th"), "Thumb B")}
    </div>
    <div class="fing-row">
      <span class="fing-label">Left</span>
      ${hole(p.has("T"), "1st finger")}${hole(p.has("1"), "2nd finger")}${hole(p.has("2"), "3rd finger")}${hole(p.has("3"), "4th finger")}
      ${hole(p.has("G#"), "G sharp lever", "lever")}
    </div>
    <div class="fing-row">
      <span class="fing-label">Right</span>
      ${hole(p.has("4"), "1st finger")}${hole(p.has("5"), "2nd finger")}${hole(p.has("6"), "3rd finger")}
      ${hole(p.has("Eb"), "E flat key", "lever")}
    </div>
    <div class="fing-row extra">
      <span class="fing-label">Foot</span>
      ${hole(p.has("C#"), "C sharp roller", "lever")}${hole(p.has("C"), "C roller", "lever")}${hole(p.has("B"), "B roller", "lever")}
    </div>
  </div>`;
}

function drawSax(p) {
  return `<div class="fing-chart sax">
    <div class="fing-row">
      <span class="fing-label">Octave</span>
      ${hole(p.has("Oct"), "Octave key", "lever")}
    </div>
    <div class="fing-row">
      <span class="fing-label">Left</span>
      ${hole(p.has("1"), "1st finger")}${hole(p.has("Bis"), "Bis key", "small")}${hole(p.has("2"), "2nd finger")}${hole(p.has("3"), "3rd finger")}
      ${hole(p.has("G#"), "G sharp", "lever")}
    </div>
    <div class="fing-row">
      <span class="fing-label">Right</span>
      ${hole(p.has("4"), "1st finger")}${hole(p.has("5"), "2nd finger")}${hole(p.has("6"), "3rd finger")}
      ${hole(p.has("Eb"), "E flat", "lever")}${hole(p.has("C"), "Low C", "lever")}
    </div>
    <div class="fing-row extra">
      <span class="fing-label">Bell</span>
      ${hole(p.has("Low B"), "Low B", "lever")}${hole(p.has("Low Bb"), "Low B flat", "lever")}
    </div>
  </div>`;
}

/* ---------- the whole table, to be looked through ---------- */

/*
 * The charts above are one fingering a note: what a player holds, drawn
 * beside the music as it goes past. This is the other thing a flute
 * player wants — every fingering the instrument has, to look through.
 *
 * The keys are laid out as they sit on the instrument rather than listed,
 * because the question being asked of a fingering is "what do my hands
 * do", and that is a shape and not a sentence. The same drawing serves a
 * fingering on its own, a fingering with the move out of it marked, and a
 * trill, which is a fingering with one key shaking.
 *
 * The names are the table's own, which come from the endpoint: Th and the
 * B flat thumb, the three fingers of each hand, the levers, the two trill
 * keys and the gizmo. Nothing here writes them down a second time.
 */

// Where each key sits, in the picture's own coordinates. side tells the
// two hands' first, second and third fingers apart, which share names.
const FLUTE_PLAN = [
  { id: "Th", side: "L", x: 16, y: 46, r: 9, label: "Th", at: "over" },
  { id: "BbTh", side: "L", x: 16, y: 74, r: 7, label: "B♭ th", at: "under" },
  { id: "1", side: "L", x: 44, y: 46, r: 9, label: "1", at: "under" },
  { id: "A#", side: "L", x: 58, y: 22, r: 6, label: "A♯", at: "over" },
  { id: "2", side: "L", x: 70, y: 46, r: 9, label: "2", at: "under" },
  { id: "3", side: "L", x: 94, y: 46, r: 9, label: "3", at: "under" },
  { id: "G#", side: "L", x: 112, y: 66, r: 6, label: "G♯", at: "under" },

  { id: "G", side: "R", x: 132, y: 22, r: 6, label: "G", at: "over" },
  { id: "F#", side: "R", x: 148, y: 22, r: 6, label: "F♯", at: "over" },
  { id: "1", side: "R", x: 172, y: 46, r: 9, label: "1", at: "under" },
  { id: "2", side: "R", x: 196, y: 46, r: 9, label: "2", at: "under" },
  { id: "3", side: "R", x: 220, y: 46, r: 9, label: "3", at: "under" },
  { id: "Bblever", side: "R", x: 150, y: 72, r: 6, label: "B♭ lev", at: "under" },
  { id: "tr1", side: "R", x: 184, y: 72, r: 6, label: "tr1", at: "under" },
  { id: "tr2", side: "R", x: 208, y: 72, r: 6, label: "tr2", at: "under" },
  { id: "B", side: "R", x: 244, y: 26, r: 6, label: "B", at: "over" },
  { id: "gizmo", side: "R", x: 266, y: 26, r: 5, label: "gizmo", at: "right" },
  { id: "C", side: "R", x: 262, y: 46, r: 6, label: "C", at: "right" },
  { id: "C#", side: "R", x: 256, y: 66, r: 6, label: "C♯", at: "under" },
  { id: "D#", side: "R", x: 232, y: 66, r: 6, label: "D♯", at: "under" },
];

// fluteKeysDown reads a fingering's bits into the set of keys it holds.
//
// The bit order is the endpoint's key list, and the two hands are told
// apart by where the list divides: everything before `left` is the left
// hand's. A name appears on both sides — each hand has a 1, a 2 and a 3 —
// so the side is part of what a key is.
function fluteKeysDown(bits, names, left) {
  const down = {};
  for (let i = 0; i < names.length; i++) {
    if (!(bits & (1 << i))) continue;
    down[(i < left ? "L:" : "R:") + names[i]] = true;
  }
  return down;
}

// fluteChart draws one fingering.
//
// shake, where it is given, is the key of a trill: it is ringed rather
// than filled, because it is neither down nor up — it is the one that
// moves. Everything is currentColor, so the picture reads in the light
// theme and the dark one alike.
function fluteChart(bits, names, left, opts) {
  const o = opts || {};
  const down = fluteKeysDown(bits, names, left);
  const shake = o.shake ? o.shake : null;
  let out = '<svg viewBox="0 0 306 92" class="fl-chart" role="img" aria-label="' +
    fluteSpell(bits, names, left) + '">';
  out += '<line x1="122" y1="30" x2="122" y2="62" stroke="currentColor"' +
    ' stroke-width="0.7" opacity="0.3"/>';
  for (const k of FLUTE_PLAN) {
    const id = k.side + ":" + k.id;
    const on = !!down[id];
    const moves = shake && shake.side === k.side && shake.id === k.id;
    out += '<circle cx="' + k.x + '" cy="' + k.y + '" r="' + k.r +
      '" stroke="currentColor" stroke-width="' + (moves ? "2" : "1.2") +
      '" fill="' + (on ? "currentColor" : "none") + '"' +
      (moves ? ' stroke-dasharray="3 2"' : "") + "/>";
    let tx = k.x, ty = k.y + k.r + 11, anchor = "middle";
    if (k.at === "over") ty = k.y - k.r - 4;
    if (k.at === "right") { tx = k.x + k.r + 3; ty = k.y + 2; anchor = "start"; }
    out += '<text x="' + tx + '" y="' + ty + '" font-size="6" text-anchor="' +
      anchor + '" fill="currentColor" opacity="' +
      (moves ? "1" : on ? "0.9" : "0.45") + '">' + fluteEsc(k.label) + "</text>";
  }
  out += "</svg>";
  return out;
}

// fluteSpell writes a fingering the way a chart does, for the picture's
// own description: somebody reading the page with a screen reader gets
// the chart in words.
function fluteSpell(bits, names, left) {
  const l = [], r = [];
  for (let i = 0; i < names.length; i++) {
    if (!(bits & (1 << i))) continue;
    (i < left ? l : r).push(names[i]);
  }
  return (l.join(" ") + " | " + r.join(" ")).trim();
}

function fluteEsc(s) {
  return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;")
    .replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

// fluteNeeds is what a fingering asks of the instrument beyond the keys
// in the picture.
//
// "Split E" is the one: it is a mechanism a flute either has or has not,
// not something a finger does, so there is nothing to draw for it and
// leaving it out silently would make two different fingerings look
// identical. It is said in words beside the picture instead.
function fluteNeeds(bits, names) {
  const out = [];
  for (let i = 0; i < names.length; i++) {
    if ((bits & (1 << i)) && names[i] === "split E") out.push("split E");
  }
  return out;
}

// fluteTuning says what a number of cents means, in words a player uses.
function fluteTuning(cents) {
  if (!cents) return "in tune";
  const how = Math.abs(cents) >= 25 ? "far " : Math.abs(cents) <= 5 ? "a shade " : "";
  return how + (cents > 0 ? "sharp" : "flat") + " (" +
    (cents > 0 ? "+" : "") + cents + ")";
}
