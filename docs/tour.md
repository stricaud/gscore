# A tour of the editor

Every window Gscore can open, what it is actually for, and which three in each menu are worth your time.

Gscore has forty-odd dialogs. That sounds like a lot until you notice that
most of them you will open once, set, and never think about again — and about
six of them you will live in. This chapter walks the menus in order and tells
you which is which.

<p class="note"><b>A shortcut through the whole thing:</b> almost everything
below can be reached by clicking the thing itself. Click a tempo marking to
change the tempo. Click a key signature to change the key. Click a slur to
select it, then <kbd>Backspace</kbd> to remove it. The menus are the map; the
score is the territory, and the territory is clickable.</p>

## File — getting music in and out

The two entries that matter are the ones at the ends.

**Open** reads rather more than you would expect. Gscore imports **ABC**,
**MusicXML** (plain and compressed), **MuseScore** (`.mscz` and `.mscx`),
**Finale** (`.musx`), **NoteWorthy Composer** (`.nwctxt`, `.nwc`) and
**GUIDO** — no exporter from the other program required, no round trip
through anything.

When an import hits something it cannot read, it does not refuse the file. It
reads everything around the problem and lists what it skipped in
<span class="menu-path">Help → Lines that could not be read</span>. A
thirty-page score with one mistyped character opens as a thirty-page score and
a note about the character, which is the only sane behaviour and is
surprisingly rare.

**Export** writes PDF, SVG, a self-contained web page, MIDI, MusicXML
(compressed or not), ABC, LilyPond, GUIDO, and audio as FLAC.

Also here:

- **Page setup…** — the paper this score is printed on. It is kept *with the
  score*, because paper size decides how wide a system may be, which decides
  how the music is laid out. It is not a print-time afterthought.
- **Print Preview…** and **Print…** — the print dialog asks what to print
  (this piece, the whole book, or the score and every part); paper and margins
  come from your system's own print dialog, which opens next.
- **Settings…** — the language the interface is written in, and whether a
  saved `.gscore` carries a PDF of itself. (See
  [things only Gscore does](only-gscore.md) for why that second one is
  interesting.)

## Edit — changing what is already there

Mostly the usual, with the usual keys. Three worth knowing:

- **Go to measure…** (<kbd>j</kbd>) — in a long score this is how you move.
- **Find and replace…** — for the *words* in a score, and there are more of
  them than you think: the title block, staff names, every syllable of lyrics,
  chord symbols, rehearsal marks, movement titles, and every "rit." and
  "solo" somebody has written. Renaming an instrument or fixing a spelling
  used to mean hunting each one by eye.
- **Pick up bars…** — select a stretch by number rather than by dragging,
  one staff at a time. (A slur over three staves at once is not a thing that
  can be drawn, so the dialog does not pretend otherwise.)

The submenus — **Stem**, **Articulation**, **Accidental**, **Beam**, **Cross
staff**, **Slur and tie** — are all reachable from single keys once you know
them: <kbd>u</kbd> and <kbd>d</kbd> for stems, <kbd>,</kbd> for staccato,
<kbd>!</kbd> for an accent, <kbd>b</kbd> to beam, <kbd>S</kbd> to slur,
<kbd>T</kbd> to tie. The menu is there for the day you forget.

**Cross staff** is the piano one: *Write on the staff below* moves a notehead
down to the other hand while it keeps belonging to the line it is part of.

## Insert — putting things on the page

This is the biggest menu in the program, and it is mostly submenus you dip
into: barlines, dynamics from `pppp` to `fff`, hairpins, octave lines
(8va, 8vb, 15ma, 15mb), tuplets from duplet to nonuplet, tremolos including a
buzz roll, and first/second-time repeat endings.

The ones that open a real window:

- **Chord symbol…** (<kbd>c</kbd>) — type `C7#9` and it shows you what it
  made of that before you commit. You can have the name, a fretboard diagram,
  or both.
- **Text…** (<kbd>t</kbd>) — words beside a note. Crucially, you say *what
  kind* of writing it is, and the kind decides how it is set and where it
  goes. The result is that every "rit." in the score comes out identical
  without you formatting anything.
- **Rehearsal mark…** — letters or numbers, boxed, and they renumber
  themselves when you insert one in the middle.
- **Script console…** — the JavaScript console from the
  [quick start](quickstart.md).
- **Tab** — a whole submenu of guitar notation: hammer-ons, pull-offs,
  slides, bends, pre-bends, vibrato, palm mute, let ring, three kinds of
  harmonic, tap, dead note.
- **Symbol → Draw a new one…** — a little pixel editor. Draw a mark no font
  has, and it becomes a symbol you can place on a note like any other. Mark
  it, share it as a `.gscore`, import somebody else's.

## Score — facts about the whole piece

Open these once per piece, early.

- **Score setup…** — who is playing. Pick the instruments and Gscore lays the
  staves out in score order, brackets each family, and gives the piano its two
  staves. This is the fastest way to start anything bigger than a tune.
- **Set tempo…**, **Key signature…**, **Time signature from here…** — the
  obvious three, and the last one means *from here*, not *everywhere*.
- **Pickup measure…** — a piece that starts before the first beat. The bar is
  left short, bar one becomes the first *full* bar, and the numbering follows
  — which is what every published edition does and what a surprising number
  of programs get wrong.
- **Bar numbers…** — how often, and whether boxed.
- **Lines of music…** — say where a line of music ends. The engraver fits as
  many bars to a line as will go; this is how you overrule it, and it also
  pins those bars so they stop moving about.
- **Engraving…** — the one to look at if you care about how the page looks.
  Every spacing the engraver uses, in staff spaces, each with a little
  picture showing the distance it sets in red, and every change redraws the
  page at once.
- **Keep bars to the meter** — a tick. On, a note too long for the bar pushes
  the overflow into the next one and the arithmetic stays right. Off, bars may
  be any length you like, which is what old music barred by phrase needs.

## Staff — the players

Add a staff, add a grand staff, hide one, reorder them, bracket or brace them.

Hiding is how you get the rest of the orchestra out of the way while you work
on the strings. A hidden staff is not drawn and not *played* either — hearing
the parts you have just put away is no help at all — but the bars keep their
length, so nothing shifts under you when a staff goes out of sight or comes
back.

- **Set instrument…** — what this staff *is*, which decides what it sounds
  like and what counts as out of range.
- **Lyrics…** (<kbd>l</kbd>) — type the verse the way you would write it.
  Syllables are handed to the notes in order; break a word across notes with
  hyphens.
- **Write as tablature…** — turn a staff into tab. **Chord builder…** gives
  you a fretboard to click fingerings onto.
- **Show fingering** (<kbd>F2</kbd>) — for flute, see below.

## View — how it looks to you, not to the printer

- **Continuous strip** / **Paginated** — write in the strip, check in pages.
- **Staff names**, **Measure numbers**, **Multi-measure rests** — ticks. Turn
  multi-measure rests on to see what a player will actually get; leave it off
  while writing, because a bar that is not drawn is a bar you cannot click in.
- **Tempo marking** — the "Andante ♩ = 58" over the first bar.
- **Metronome marking** — an arrow over every beat, where the beat actually
  falls. For counting a syncopated entry or an odd meter.
- **Note colours…** — a colour per note name, for readers who cannot yet name
  a line by looking at it. Kept for you rather than for the score, and it
  prints.
- **Concert pitch** — the score as it *sounds* rather than as each player
  reads it. Three staves in three keys playing the same line look like three
  different lines until this is on.
- **Flute fingerings…** — this one deserves its own paragraph.

<p class="tip"><b>The flute chart.</b> Not "a fingering" — <i>every</i>
fingering the instrument has, with how far out of tune each one comes out and
how many keys it is from the standard. That means the alternate you actually
want is one key away and in tune, and you can see that before you commit to a
passage. It covers trills too, which are a pair of notes rather than a
note.</p>

## Play

<kbd>Shift</kbd>+<kbd>Space</kbd> plays and pauses. Beyond that:

- **MIDI keyboard…** — play the music *in*. A key pressed writes a note at
  the cursor, which is how most people get music into a notation program.
- **Metronome click** — a click on every beat, at the score's own speed,
  counted the way a musician counts it. Independent of the arrows on the page,
  because wanting to hear the beat and wanting to see it are two different
  wants.
- **Playback sound…** — which soundfont to play through. Set once; remembered,
  because you went and found that file and should not be asked twice.

## Analysis — the menu no other notation editor has

This is the part of Gscore that surprises people. Eight windows that read the
music back to you:

- **Harmonics…** — pick a note, or press play and it follows along.
- **Range…** — how far each staff reaches, and whether that instrument can
  actually go there. Writes you out of a lot of trouble before a rehearsal.
- **Keys and modulations…** — read from *the notes*, not from the key
  signature. It will tell you the piece is in D minor even if nobody wrote
  the signature.
- **Shape of the piece…** — who is playing when, and what repeats what, as a
  picture of the whole score.
- **Voice leading…** — how the lines move against each other.
- **Spacing…** — how low the bottom of the harmony is written. Mud, found
  before it is heard.
- **Rhythm and metre…** — where the notes fall against the beat.
- **Melodic shape…** — what each line is actually made of.

None of this is required to write music. All of it is the kind of thing a
teacher says out loud while looking over your shoulder, and having it in the
program is the difference between *noticing* and *being told*.

## Parts, Scripts, Help

**Parts** builds itself from the score: which parts exist is a fact about the
music and not about a menu. A score with none offers to make them.

**Scripts** lists whatever is in your `Gscore/plugins` folder. Putting a file
there *is* installing it. (The menu disappears entirely when Gscore is served
to a browser somewhere else, because then the folder would be the server's and
not yours.)

**Help** has the two references worth bookmarking — **Keyboard shortcuts**
(every binding, and you can change any of them) and **How to type music**
(the typing syntax, generated from the parser itself) — plus **Lines that
could not be read**, the import log mentioned above.

## Where next

<div class="next-chapter">
<img src="../icon.png" alt="">
<p><span class="where">Next chapter</span>
<a href="only-gscore.html"><b>Things only Gscore does</b></a> — the short
version of why this program is worth having alongside whatever else you
use.</p>
</div>

- [The user manual](manual.md) — the same ground, in depth, for reference.
- [Index of musical terms](glossary.md) — every musical word used here.
