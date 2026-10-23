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
