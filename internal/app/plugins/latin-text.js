// @name  Divide a Latin text into syllables
// @about hyphenates a Latin text for setting under the notes
// @menu  Scripts

// Divide a Latin text into syllables.
//
// Setting a text under music means knowing where the words break, and a
// Mass is four hundred words to hyphenate before a note of it can be typed.
// Latin divides by rule, so this does it: paste the text in and it comes
// back hyphenated, ready to be typed into the lyric line, with the count of
// syllables beside each line so it can be checked against the notes.
//
// It changes nothing in the score. What it is for is the moment before the
// typing, and a script that put the words in itself would have to guess
// which note each one goes on — which is the one part of the job that is
// musical rather than mechanical.
//
// Latin, and it says so. English cannot be divided by rule: "heaven" is
// heav-en and "beaver" is bea-ver, and nothing in the letters says which.
// A word this cannot read is left alone and marked, rather than divided
// wrongly and left looking right.

const text = await ask(
  "The Latin text, as many lines as you like:",
  "Kyrie eleison. Christe eleison.");
if (!text) {
  say("Nothing to divide.");
} else {
  let words = 0, syllables = 0;
  const trouble = [];

  for (const line of String(text).split(/\r?\n/)) {
    if (!line.trim()) {
      say("");
      continue;
    }
    const out = [];
    let count = 0;
    // Split on the spaces and keep what is not a letter where it was: the
    // punctuation of a liturgical text is how a singer finds the phrase,
    // and a text that came back without its commas would have to be read
    // against the original to be used.
    for (const token of line.split(/(\s+)/)) {
      if (!token.trim()) {
        out.push(token);
        continue;
      }
      // The word, and whatever is clinging to either end of it.
      const m = /^([^\p{L}]*)(.*?)([^\p{L}]*)$/u.exec(token);
      const word = m[2];
      const got = word ? latinSyllables(word) : null;
      if (!got) {
        if (word) trouble.push(word);
        out.push(token);
        continue;
      }
      words++;
      count += got.length;
      syllables += got.length;
      out.push(m[1] + got.join("-") + m[3]);
    }
    say(out.join("") + "   (" + count + ")");
  }

  say("");
  say(words + " word" + (words === 1 ? "" : "s") + ", " +
      syllables + " syllable" + (syllables === 1 ? "" : "s") + ".");
  if (trouble.length) {
    const once = trouble.filter((w, i) => trouble.indexOf(w) === i);
    say("Left whole, not being Latin this can read: " + once.join(", "));
  }
}
