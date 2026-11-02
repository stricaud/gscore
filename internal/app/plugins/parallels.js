// @name  Parallel fifths and octaves
// @about reads the outer parts and says where they move in parallel
// @menu  Scripts
//
// Parallel fifths and octaves.
//
// The check a theory teacher makes by hand down the side of a page. Two
// parts a fifth apart that move the same way to another fifth are
// parallel fifths; the same at the octave. It is the first thing anybody
// is taught not to write and the easiest to write without noticing,
// because each part on its own is perfectly good.
//
// Nothing is changed. It reads and reports, which is what a check is
// for: a piece may want the parallels it has — organum is nothing else,
// and so is most of what a horn section plays — and that decision is
// not a script's to make.
//
// It is also the shortest answer to what reading a score is good for.
// Everything here comes from three verbs: staves() for who is playing,
// notes(bar, staff) for what they play, measure(bar, staff) for what the
// bar is called.

const pick = staves();
if (pick.length < 2) {
  say("This needs two staves or more to compare.");
} else {
  // The outer parts, which is where parallels are heard and so where
  // they are looked for: an inner part doubling at the fifth for a bar
  // is a colour, the outer two doing it is the mistake.
  const top = 1, bottom = pick.length;

  // What was picked up, if anything was. "Do this to what I have
  // selected" is what a check is usually wanted for — somebody has just
  // written four bars and wants those four looked at, not the ninety they
  // wrote last week.
  const held = selection();
  const first = held && held.count ? held.from.bar : 1;
  const last = held && held.count ? held.to.bar : score.bars;

  say("Looking at " + pick[0].name + " against " + pick[bottom - 1].name +
      (held && held.count ? ", bars " + first + " to " + last : "") + ".");

  // The top line of the upper staff and the bottom line of the lower
  // one, as semitones, with where in the bar each note falls. A chord
  // counts as its outermost note, because that is the one in the line.
  const edge = (bar, staff, lowest) => {
    const out = [];
    for (const n of notes(bar, staff) || []) {
      if (n.rest || !n.midi) continue;
      const all = [n.midi].concat(n.chordMidi || []);
      out.push({ at: n.at, midi: lowest ? Math.min(...all) : Math.max(...all) });
    }
    return out;
  };

  let found = 0;
  for (let b = first; b <= last && found < 40; b++) {
    // Two and a half thousand bars of a symphony is a moment's work, and
    // a window that says nothing for that moment looks stuck.
    await progress(b - first + 1, last - first + 1);
    const hi = edge(b, top, false);
    const lo = edge(b, bottom, true);
    // Only the places both staves strike at once. Two parts whose notes
    // fall between each other are not moving together, whatever the
    // intervals look like on the page.
    const both = [];
    for (const h of hi) {
      const l = lo.find((x) => x.at === h.at);
      if (l) both.push({ at: h.at, hi: h.midi, lo: l.midi });
    }

    for (let i = 0; i + 1 < both.length; i++) {
      const a = both[i], c = both[i + 1];
      const was = a.hi - a.lo, now = c.hi - c.lo;
      if (was !== now) continue;
      // A fifth or an octave, counting compound ones: a twelfth is a
      // fifth as far as the ear and the rule are concerned.
      const over = was % 12;
      if (over !== 0 && over !== 7) continue;
      if (was === 0) continue; // a unison is a different argument
      // Both parts have to have moved, and the same way. One holding
      // while the other moves is nobody's mistake.
      if (a.hi === c.hi || a.lo === c.lo) continue;
      if ((c.hi - a.hi) * (c.lo - a.lo) <= 0) continue;

      found++;
      const m = measure(b, top);
      say("Bar " + (m ? m.number : b) + ": parallel " +
          (over === 0 ? "octaves" : "fifths") +
          ", beat " + a.at + " to " + c.at + ".");
      if (found >= 40) {
        say("… stopping at forty; there may be more.");
        break;
      }
    }
  }
  say(found === 0
    ? "No parallel fifths or octaves between the outer parts."
    : found + " to look at.");
}
