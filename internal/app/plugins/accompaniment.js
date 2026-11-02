// @name  Accompaniment from the chord symbols
// @about writes a left hand under a lead sheet, from the chords written over it
// @menu  Scripts

// Accompaniment from the chord symbols.
//
// The plug-in everybody writes first. A lead sheet is a tune with chord
// symbols over it, and what is wanted under it is something to play: the
// root, then the chord, bar after bar. Doing it by hand is typing the same
// four notes two hundred times with the spelling changed each time.
//
// What the chords are is not this script's to know. chordNotes("Bbm7b5")
// comes back spelled as it should be written — B flat, D flat, F flat, A
// flat, and not a handful of sharps that sound the same — because the
// editor asks the part of gscore that knows what a chord is. A table of
// chord spellings inside this file would be a fourth copy of something the
// program already says once.
//
// It writes onto a staff of its own, below the tune, and never over the
// tune: a script that wrote into the staff you were looking at would be a
// script you ran once.

const pick = staves();
const chordsOn = Number(await ask(
  "Which staff has the chord symbols over it?", "1"));
if (!(chordsOn >= 1 && chordsOn <= pick.length)) {
  say("There is no staff " + chordsOn + ".");
} else {
  const pattern = await ask(
    "Root then chord, chord on every beat, or the root alone?",
    "root then chord");
  if (!pattern) {
    say("Nothing written.");
  } else {
    // Read the whole piece before writing a note of it, because reading is
    // of the score as it stands and the first thing written changes that.
    const found = [];
    for (let b = 1; b <= score.bars; b++) {
      await progress(b, score.bars);
      for (const n of notes(b, chordsOn)) {
        if (n.chordSymbol) found.push({ bar: b, at: n.at, symbol: n.chordSymbol });
      }
    }
    if (!found.length) {
      say("No chord symbols on " + pick[chordsOn - 1].name + ".");
    } else {
      say(found.length + " chord" + (found.length === 1 ? "" : "s") + " to set.");

      // Everything that cannot be spelled is said once, at the end, rather
      // than stopping the run: a lead sheet with one chord nobody can read
      // in it is still a lead sheet worth accompanying.
      const unknown = [];
      const low = staff("piano", { name: "Accompaniment", clef: "bass" });

      let wrote = 0;
      for (let b = 1; b <= score.bars; b++) {
        at(b, low);
        const here = found.filter((f) => f.bar === b);
        if (!here.length) {
          rest(1);
          continue;
        }
        // One chord to the bar is the common case and the one worth
        // playing well; two or more and the bar is simply divided
        // between them.
        const each = here.length === 1 ? 2 : 4;
        for (const f of here) {
          const voiced = chordNotes(f.symbol, 3);
          if (!voiced) {
            unknown.push(f.symbol);
            rest(here.length === 1 ? 1 : 4);
            continue;
          }
          if (pattern.indexOf("root") === 0 && pattern.indexOf("alone") < 0) {
            // The root, then the rest of the chord above it: the oldest
            // left hand there is.
            note(voiced[0], each);
            chord(voiced.slice(1), each);
          } else if (pattern.indexOf("alone") >= 0) {
            note(voiced[0], here.length === 1 ? 1 : 4);
          } else {
            chord(voiced, here.length === 1 ? 1 : 4);
          }
          wrote++;
        }
      }
      say("Wrote " + wrote + " of them under the tune.");
      if (unknown.length) {
        say("Left as rests, not being chords gscore can read: " +
            unknown.filter((s, i) => unknown.indexOf(s) === i).join(", "));
      }
    }
  }
}
