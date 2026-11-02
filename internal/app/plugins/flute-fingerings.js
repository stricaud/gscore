// @name  Flute fingerings
// @about draws the fingering for every note of a flute passage
// @menu  Scripts

// Flute fingerings.
//
// What a teacher does over a hard passage: writes the fingering above each
// note. Pick up the bars that are giving trouble and this draws them —
// every note in order, with the keys to hold down for it, and how far out
// of tune that fingering sits on the instrument. Pick up nothing and it
// draws the chart of every note the part uses, which is the page to put on
// the stand while learning the piece.
//
// The intonation is the part worth staring at. A flute is not an even
// instrument: low C is twenty cents flat, top C is thirty cents sharp, and
// the two B flats either side of the break are fifteen. Knowing which notes
// in this passage are the ones you will have to lip is half of playing it
// in tune, and it is not something the notes on the page can tell you.
//
// The fingerings are the standard ones — the chart everybody learns. The
// source this came from carries some twenty-eight thousand alternatives as
// well, and none of them is offered here: most are harmonics and trick
// fingerings, nothing in the data says which are the ones a player would
// actually reach for, and a fingering suggested wrongly is worse than one
// not suggested at all. Somebody who knows the alternates can add them to
// the table below.

// The notes, each with the keys held down and how many cents sharp or flat
// that comes out on a well-made instrument.
//
// Left hand before the bar, right hand after it, which is how every flute
// chart is written. Th is the thumb, BbTh the B flat thumb; 1, 2 and 3 are
// the first, second and third fingers of each hand; the named ones are the
// levers — G sharp under the left little finger, D sharp, C sharp, C and B
// under the right, the B flat lever beside the right first finger, the two
// trill keys, and the gizmo on the foot.
const CHART = {
  "C4":  ["Th 1 2 3 | 1 2 3 C", -20],
  "C#4": ["Th 1 2 3 | 1 2 3 C#", -30],
  "D4":  ["Th 1 2 3 | 1 2 3", -25],
  "D#4": ["Th 1 2 3 | 1 2 3 D#", -20],
  "E4":  ["Th 1 2 3 | 1 2 D#", -10],
  "F4":  ["Th 1 2 3 | 1 D#", -5],
  "F#4": ["Th 1 2 3 | 3 D#", -5],
  "G4":  ["Th 1 2 3 | D#", 0],
  "G#4": ["Th 1 2 3 G# | D#", -5],
  "A4":  ["Th 1 2 | D#", -5],
  "A#4": ["BbTh 1 | D#", 0],
  "B4":  ["Th 1 | D#", 10],
  "C5":  ["1 | D#", 10],
  "C#5": ["| D#", 25],
  "D5":  ["Th 2 3 | 1 2 3", 5],
  "D#5": ["Th 2 3 | 1 2 3 D#", 0],
  "E5":  ["Th 1 2 3 | 1 2 D#", -5],
  "F5":  ["Th 1 2 3 | 1 D#", 0],
  "F#5": ["Th 1 2 3 | 3 D#", 0],
  "G5":  ["Th 1 2 3 | D#", 5],
  "G#5": ["Th 1 2 3 G# | D#", 5],
  "A5":  ["Th 1 2 | D#", 5],
  "A#5": ["BbTh 1 | D#", 15],
  "B5":  ["Th 1 | D#", 15],
  "C6":  ["1 | D#", 10],
  "C#6": ["| D#", 15],
  "D6":  ["Th 2 3 | D#", 5],
  "D#6": ["Th 1 2 3 G# | 1 2 3 D#", 10],
  "E6":  ["Th 1 2 | 1 2 D#", 5],
  "F6":  ["Th 1 3 | 1 D#", 10],
  "F#6": ["Th 1 3 | 3 D#", 10],
  "G6":  ["1 2 3 | D#", 15],
  "G#6": ["2 3 G# | D#", 25],
  "A6":  ["Th 2 | 1 D#", 0],
  "A#6": ["Th 1 | 1 tr1 D#", -10],
  "B6":  ["Th 1 3 | tr2 D#", 10],
  "C7":  ["1 2 3 G# | 1", 30],
};

// Where every key sits in the drawing, laid out as the instrument is: the
// left hand along the left, the right hand along the right, with the levers
// above and below the fingers that work them. Big circles are the holes a
// finger covers; small ones are levers.
//
// side says which hand the chart writes it under, because 1, 2 and 3 are
// the names of three fingers on each hand and a bare "2" means nothing
// until you know which side of the bar it was written on.
// at says where the key's name goes: under it, over it, or to the right.
// Written out per key rather than worked out, because the keys of a flute
// are not on a grid — the thumb has another key beneath it and the right
// little finger has four in a cluster, and a name placed by rule lands on
// top of the circle next door.
const KEYS = [
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

  // The right little finger's cluster, at the end of the instrument.
  { id: "B", side: "R", x: 244, y: 26, r: 6, label: "B", at: "over" },
  { id: "gizmo", side: "R", x: 266, y: 26, r: 5, label: "gizmo", at: "right" },
  { id: "C", side: "R", x: 262, y: 46, r: 6, label: "C", at: "right" },
  { id: "C#", side: "R", x: 256, y: 66, r: 6, label: "C♯", at: "under" },
  { id: "D#", side: "R", x: 232, y: 66, r: 6, label: "D♯", at: "under" },
];

// held reads a line of the chart into the set of keys it means, by side.
function held(line) {
  const [left, right] = String(line).split("|");
  const set = {};
  for (const [side, part] of [["L", left], ["R", right]]) {
    for (const token of String(part || "").trim().split(/\s+/)) {
      if (token) set[side + ":" + token] = true;
    }
  }
  return set;
}

// diagram draws one fingering: the keys of the instrument, with the ones
// held down filled in.
//
// currentColor throughout, so it reads in the light theme and the dark one
// alike — the panel it is drawn into sets the ink and the circles take it.
// next, when it is given, is the fingering of the note that follows: a
// triangle appears under every key that has to go down for it and over
// every key that has to come up. That is the move to practise, and it is a
// picture rather than a sentence because what a finger has to do is not a
// sentence.
//
// Triangles and not colour. A second colour would have to be a colour that
// reads on both the light ground and the dark one, and the shape says it
// without one: pointing down is press, pointing up is lift.
function diagram(line, title, next) {
  const down = held(line);
  const then = next ? held(next) : null;
  let out = '<svg viewBox="0 0 306 100" role="img" aria-label="' +
    esc(title + ": " + line) + '">';
  // The two hands, said once, so the three fingers on each side are not
  // three unlabelled circles in a row of six.
  out += '<text x="8" y="11" font-size="8" fill="currentColor"' +
    ' opacity="0.65">left hand</text>';
  out += '<text x="298" y="11" font-size="8" fill="currentColor"' +
    ' opacity="0.65" text-anchor="end">right hand</text>';
  // The gap between the hands, which is the bar in the written chart.
  out += '<line x1="122" y1="30" x2="122" y2="62" stroke="currentColor"' +
    ' stroke-width="0.7" opacity="0.3"/>';

  for (const k of KEYS) {
    const id = k.side + ":" + k.id;
    const on = !!down[id];
    out += '<circle cx="' + k.x + '" cy="' + k.y + '" r="' + k.r +
      '" stroke="currentColor" stroke-width="1.2" fill="' +
      (on ? "currentColor" : "none") + '"/>';

    // The name beside each one, so the picture explains itself to somebody
    // who has not learnt the chart yet.
    //
    // The room for a triangle is left whether or not there is one to draw,
    // so that the same key sits in the same place in every diagram: a
    // chart whose circles shuffle between one note and the next is a chart
    // you have to re-read instead of compare.
    let tx = k.x, ty = k.y + k.r + 14, anchor = "middle";
    if (k.at === "over") ty = k.y - k.r - 11;
    if (k.at === "right") {
      tx = k.x + k.r + 3;
      ty = k.y + 2;
      anchor = "start";
    }
    out += '<text x="' + tx + '" y="' + ty + '" font-size="6" text-anchor="' +
      anchor + '" fill="currentColor" opacity="' + (on ? "0.9" : "0.5") +
      '">' + esc(k.label) + '</text>';

    if (!then) continue;
    const soon = !!then[id];
    if (soon === on) continue;
    // A finger about to go down, or about to come up. Beside the circle on
    // the side the name is not, so the two never sit on each other.
    const left = k.x - 4, right = k.x + 4;
    if (soon) {
      const top = k.y + k.r + 1;
      out += '<polygon points="' + left + ',' + top + ' ' + right + ',' + top +
        ' ' + k.x + ',' + (top + 6) + '" fill="currentColor"/>';
    } else {
      const base = k.y - k.r - 1;
      out += '<polygon points="' + left + ',' + base + ' ' + right + ',' + base +
        ' ' + k.x + ',' + (base - 6) + '" fill="currentColor" opacity="0.55"/>';
    }
  }
  out += "</svg>";
  return out;
}

// moving is what changes between two fingerings: the keys that go down and
// the keys that come up, by the names a player calls them.
function moving(from, to) {
  const a = held(from), b = held(to);
  const down = [], up = [];
  for (const k of KEYS) {
    const id = k.side + ":" + k.id;
    const was = !!a[id], now = !!b[id];
    if (now && !was) down.push(k.side + " " + k.label);
    if (was && !now) up.push(k.side + " " + k.label);
  }
  return { down, up };
}

function esc(s) {
  return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;")
    .replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

// The name a fingering chart calls a sounding pitch. Sharps, because that
// is how the chart is written; the score may spell the same note as a flat
// and it is the same three fingers either way.
const SHARPS = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
function chartName(midi) {
  return SHARPS[((midi % 12) + 12) % 12] + (Math.floor(midi / 12) - 1);
}

// The three B flats.
//
// The one real fingering decision on a flute, and the one every teacher
// spends a lesson on: B flat has three fingerings and which to use depends
// on what is around it, not on the note itself. They are all in tune and
// they are all correct; choosing wrongly costs a scramble in the middle of
// a run.
//
//	thumb B flat	the thumb takes the lower of its two keys. The
//			default in flat keys, and impossible to use next
//			to a B natural: the thumb would have to move.
//	one and one	the right first finger. The one to use when B flat
//			and B natural are both about, since the thumb then
//			stays where it is.
//	the lever	the side lever under the right first finger. For
//			B flat to and from C, and for chromatic runs, where
//			neither of the others leaves the fingers still.
const BFLATS = {
  "thumb B♭": "BbTh 1 | D#",
  "one and one": "Th 1 | 1 D#",
  "the B♭ lever": "Th 1 | Bblever D#",
};

// adviseBflat says which of the three to use here, and why.
//
// From what is actually around the note rather than from the key
// signature: a piece in F with a B natural in this bar is a passage for one
// and one whatever the signature says.
function adviseBflat(before, after) {
  const near = [before, after].filter(Boolean);
  const plain = (n) => n && n.replace(/-?\d+$/, "");
  if (near.some((n) => plain(n) === "B")) {
    return ["one and one",
      "a B natural is next to it, and the thumb cannot be in two places"];
  }
  if (near.some((n) => plain(n) === "C")) {
    return ["the B♭ lever",
      "it goes to or from C, which is what the lever is for"];
  }
  return ["thumb B♭", "nothing round it needs the thumb anywhere else"];
}

// How far out of tune, said the way a player would say it.
function tuning(cents) {
  if (!cents) return "in tune";
  return Math.abs(cents) + " cents " + (cents > 0 ? "sharp" : "flat");
}

/* ---------------------------------------------------------------- */

const players = staves();
// The flute, if one of the staves says it is one. A score with a flute part
// and a piano under it should not have to be told which is which.
let which = players.findIndex((s) =>
  /flute|piccolo/i.test(s.instrument + " " + s.name)) + 1;
if (!which) which = 1;

const held_ = selection();
const picked = held_ && held_.count ? held_ : null;
if (picked) which = picked.staff;

const first = picked ? picked.from.bar : 1;
const last = picked ? picked.to.bar : score.bars;

say(players[which - 1]
  ? "Reading " + players[which - 1].name +
    (picked ? ", bars " + first + " to " + last : ", the whole part") + "."
  : "There is no staff " + which + ".");

if (players[which - 1]) {
  // Every note of the passage, in order, and how often each pitch comes up.
  const run = [];
  const times = {};
  const pitchOf = {};
  const missing = {};
  for (let b = first; b <= last; b++) {
    await progress(b - first + 1, last - first + 1);
    for (const n of notes(b, which)) {
      if (n.rest || !n.midi) continue;
      const name = chartName(n.midi);
      if (!CHART[name]) {
        missing[name] = (missing[name] || 0) + 1;
        continue;
      }
      run.push({ bar: b, name });
      times[name] = (times[name] || 0) + 1;
      pitchOf[name] = n.midi;
    }
  }

  if (!run.length) {
    say("No notes on that staff" +
        (Object.keys(missing).length ? " that a flute can play." : "."));
  } else {
    // In pitch order, which is the order a chart is read in.
    const inOrder = Object.keys(times).sort((a, b) => pitchOf[a] - pitchOf[b]);

    say(run.length + " notes, " + inOrder.length + " of them different, " +
        "from " + inOrder[0] + " to " + inOrder[inOrder.length - 1] + ".");

    // The notes that will fight you. Ten cents is about where a listener
    // starts to hear it, so that is where the list begins.
    const awkward = inOrder.filter((n) => Math.abs(CHART[n][1]) >= 10);
    if (awkward.length) {
      say("Watch the tuning on: " +
          awkward.map((n) => n + " (" + tuning(CHART[n][1]) + ")").join(", "));
    }

    /*
     * What the fingers have to do, which is the other half of reading a
     * passage. The notes say what to sound; the chart says what to hold;
     * only the difference between one fingering and the next says what to
     * move, and that is the thing being practised.
     */
    const steps = [];
    for (let i = 0; i + 1 < run.length; i++) {
      const from = run[i], to = run[i + 1];
      if (from.name === to.name) continue; // a repeated note moves nothing
      const m = moving(CHART[from.name][0], CHART[to.name][0]);
      steps.push({
        bar: to.bar, from: from.name, to: to.name,
        down: m.down, up: m.up,
        // Fingers going both ways at once is the hard kind of move: one
        // hand pressing while the other lifts is where a run trips, and
        // it is why the break between C sharp and D is the hardest join
        // on the instrument.
        both: m.down.length > 0 && m.up.length > 0,
        count: m.down.length + m.up.length,
      });
    }

    // The B flats, each with the fingering to use for it and the reason.
    const flats = [];
    for (let i = 0; i < run.length; i++) {
      if (!/^A#/.test(run[i].name)) continue;
      const [pick, why] = adviseBflat(
        run[i - 1] && run[i - 1].name, run[i + 1] && run[i + 1].name);
      flats.push({ bar: run[i].bar, note: run[i].name, pick, why });
    }
    if (flats.length) {
      say("");
      say(flats.length + " B flat" + (flats.length === 1 ? "" : "s") + ":");
      // Said once per fingering rather than once per note: "use the lever
      // in bars 3, 7 and 11" is something to remember, and eleven lines
      // each naming one bar is not.
      for (const pick of Object.keys(BFLATS)) {
        const mine = flats.filter((f) => f.pick === pick);
        if (!mine.length) continue;
        const bars = mine.map((f) => f.bar)
          .filter((b, i, all) => all.indexOf(b) === i);
        say("  " + pick + " in bar" + (bars.length === 1 ? " " : "s ") +
            bars.join(", ") + " — " + mine[0].why);
        draw(diagram(BFLATS[pick], "B flat, " + pick),
             "B flat — " + pick);
      }
    }

    // The moves worth practising, hardest first: most fingers moving, and
    // the ones where they move in both directions at once.
    const hard = steps.slice()
      .sort((a, b) => (b.both - a.both) || (b.count - a.count) || (a.bar - b.bar))
      .filter((s) => s.both || s.count >= 3)
      .slice(0, 8);
    if (hard.length) {
      say("");
      say("The joins to practise:");
      for (const s of hard) {
        say("  bar " + s.bar + ", " + s.from + " to " + s.to + " — " +
            (s.down.length ? s.down.length + " down" : "") +
            (s.down.length && s.up.length ? ", " : "") +
            (s.up.length ? s.up.length + " up" : "") +
            (s.both ? ", both at once" : ""));
      }
    }

    // The passage itself, note by note, where a passage was picked up.
    // Only then: a whole part is two hundred diagrams nobody reads, and the
    // chart of what it uses is the better page for that.
    if (picked) {
      let bar = 0;
      const show = run.slice(0, 120);
      for (let i = 0; i < show.length; i++) {
        const note = show[i], next = show[i + 1];
        // Each note with the move out of it marked on it: a triangle under
        // every key that goes down next and over every key that comes up.
        // One picture for the note and the step away from it, because that
        // is how a passage is practised — not note by note but join by
        // join.
        const move = next && next.name !== note.name
          ? " → " + next.name
          : "";
        const caption = (note.bar === bar ? "" : "Bar " + note.bar + " — ") +
          note.name + move + ", " + tuning(CHART[note.name][1]);
        bar = note.bar;
        draw(diagram(CHART[note.name][0], note.name,
                     next && CHART[next.name] ? CHART[next.name][0] : null),
             caption);
      }
      if (run.length > 120) {
        say("Drawn the first 120; pick up fewer bars for the rest.");
      }
    } else {
      for (const name of inOrder) {
        draw(diagram(CHART[name][0], name),
             name + " — " + tuning(CHART[name][1]) + ", " +
             times[name] + (times[name] === 1 ? " time" : " times"));
      }
    }

    const outside = Object.keys(missing);
    if (outside.length) {
      say("Outside the flute's range, so not drawn: " + outside.join(", "));
    }
  }
}
