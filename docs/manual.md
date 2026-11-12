# The user manual

Everything Gscore does, in order, for when you want the detail rather than the tour.

If you have not written anything yet, start with the
[quick start](quickstart.md) — it is twenty minutes and it covers the eighty
per cent. This chapter is the other twenty, and it is meant to be dipped into
rather than read straight through. Musical words are explained in the
[index of musical terms](glossary.md).

## The window

<ul class="plain">
<li><b>Left — the toolbox.</b> What you are about to write, and a zoom slider.</li>
<li><b>Middle — the music.</b> With a strip of buttons down its left edge, one
per staff, that jump to the end of that staff. (The score itself opens at the
beginning, the way a document does; the buttons are for somebody who opened it
to <i>add</i> to it.)</li>
<li><b>Right — the inspector.</b> What this score is, which tune of a book you
are looking at, the staves, and the fingering for the note you are on.</li>
<li><b>Top — the menus</b>, a play button and a clock.</li>
</ul>

Drum staves get a set of input pads, because a drum staff's lines name
instruments rather than pitches and there is nothing on the page to tell you
that the top space is a hi-hat.

## The ideas it is built on

**A score has parts; a part has measures; a measure has voices; a voice has
elements.** An element is a note, a chord or a rest. Two independent lines
written on one staff are two *voices*, and they are written across each other
rather than after each other.

**A pitch is a step, an octave and an alteration** — not a position on a
staff. That is why transposing keeps your spelling, and why what *sounds* can
differ from what is *printed*.

**The engraving holds no geometry.** Laying out a page is a pure function from
music to shapes, measured in staff spaces rather than pixels. That is what
lets the same engine serve the screen, the printer and the PDF without any of
them disagreeing.

**Strip or pages.** Gscore opens as a continuous strip — one unbroken line of
music — because that is the shape writing wants.
<span class="menu-path">View → Paginated</span> shows pages. Printing always
paginates whatever the screen is doing.

**Concert pitch is a view, not a change.** A clarinet in B flat reads a C and
a B flat comes out. <span class="menu-path">View → Concert pitch</span> shows
every part as it *sounds*; what is stored is untouched, and writing on a
transposing staff still goes in as what its player reads.

## Writing music

There are four ways in. They write the same music and you will use all four.

### With the mouse

<kbd>i</kbd> turns the pointer into a pen; <kbd>i</kbd> again turns it back.
<kbd>Shift</kbd> flips whichever mode you are in for as long as you hold it,
so picking something up while writing — or writing one note while editing —
does not cost a mode change.

### With the keyboard

Choose a length, choose a pitch, press <kbd>Space</kbd>.

| Key | Length |
|---|---|
| <kbd>1</kbd> | whole |
| <kbd>2</kbd> | half |
| <kbd>3</kbd> | dotted half |
| <kbd>4</kbd> | quarter |
| <kbd>5</kbd> | dotted quarter |
| <kbd>8</kbd> | eighth |
| <kbd>9</kbd> | sixteenth |
| <kbd>0</kbd> | thirty-second |
| <kbd>.</kbd> | dot what is selected — again for two dots, three, then none |

<kbd>↑</kbd> and <kbd>↓</kbd> move the pitch a step; with <kbd>Shift</kbd>, an
octave. <kbd>Enter</kbd> writes a rest of the current length.
<kbd>Ctrl</kbd>/<kbd>⌘</kbd>+<kbd>Space</kbd> adds another notehead to the note
just written, which is how chords are built.

Accidentals: <kbd>+</kbd> sharpens (again for a double sharp, again for none),
<kbd>-</kbd> flattens the same way. <kbd>#</kbd>, <kbd>_</kbd> and <kbd>=</kbd>
set sharp, flat and natural outright — and <kbd>=</kbd> with nothing selected
ties instead, because that is what the key is free for.

A sign is printed where it tells the reader something they do not already
know, and nowhere else. An accidental holds to the end of its bar, so a note
written on a line something has already altered takes that alteration: write
a B flat and the next B in the bar is a B flat too, with nothing in front of
it. Ask for a natural and you get a natural — once, on the note that needs
it, not again on the two after it. A reminder you want anyway is what the
brackets are for.

### By typing

<kbd>Ctrl</kbd>/<kbd>⌘</kbd>+<kbd>K</kbd> opens the typing bar. A note is two
facts, how long and which pitch, in the order a musician says them.

| Written | Means |
|---|---|
| `8abcdef` | six eighth notes, climbing |
| `4cde 8fg` | quarters c d e, then eighths f g |
| `4.c` | dotted — as many dots as you write |
| `c4` | middle C; the octave is scientific |
| `f#3` | F sharp (`s` works for the sharp too) |
| `a_` | A flat, *inside a run* |
| `bb4` | B flat, where the note stands on its own |
| `fn` | F natural, whatever the signature says |
| `[ceg]` | a chord, read from the bottom up |
| `+3` / `−2` | a third above / a step below the note before |
| `r` | a rest |
| `\|` | the next bar, making one if the music has run out |
| `tie`, `slur`, … | a mark on everything after it; the same word again turns it off |
| `transpose-up` | a word that is not music runs the command of that name |

A length stays in force until you give another. Inside a run it needs a space
in front of it, because digits straight after a note are its octave. In a run
`b` is the note B — it has to be, or `8abcdef` would start on an A flat — so a
flat inside a run is `_`; a note standing alone still takes `bb4`, because
that is how everybody writes B flat.

A line is read whole before anything is written, so a line with a mistake in
it writes nothing, says what was wrong, and leaves your line in the box to
fix. Everything one line writes is one undo.

<span class="menu-path">Help → How to type music</span> is the full reference,
generated from the parser, so it cannot go stale.

### By script

<span class="menu-path">Insert → Script console…</span>. See
[Scripting](#scripting) below.

### Bars

Fill a bar and Gscore starts the next one. <kbd>Tab</kbd> ends one early.
<kbd>m</kbd> adds a bar at the end;
<span class="menu-path">Insert → Measures after this one…</span> adds several.

<span class="menu-path">Score → Keep bars to the meter</span> decides what
happens when a note will not fit. On — the default — the overflow moves into
the next bar and the arithmetic stays right, which is what makes typing music
in feel like typing. Off, bars may be any length, which plenty of old music
barred by phrase needs and no amount of correction will fix.

A bar you are part-way through keeps the room the rest of it will take, so
there is somewhere to put the cursor. There is always one empty bar drawn past
the end of the music, so there is somewhere to click to carry on; it becomes
real when you write in it, and a score does not pick up a stray empty bar just
for having been looked at. Neither happens on a printed page, where a short
bar is short on purpose.

<span class="menu-path">Score → Pickup measure…</span> sets an anacrusis: the
bar is left short and bar *one* becomes the first full bar, so the numbering
matches a published edition.

### Tuplets

<kbd>Ctrl</kbd>/<kbd>⌘</kbd> plus <kbd>2</kbd>–<kbd>9</kbd> makes a duplet
through a nonuplet; on a selection, those notes become the group.
<kbd>Ctrl</kbd>+<kbd>1</kbd> takes the group back to one note.
<span class="menu-path">Insert → Tuplet → Other number…</span> for anything
stranger. A group that is beamed together gets no bracket, which is the rule
in Gould and in every published score.

A run that ran over the barline while you were writing it comes home when
you make it a group. Three eighths take a beat and a half and a triplet of
them takes one, so the third note — which had nowhere to go but the next bar
— fits in the bar you wrote it in as soon as you say what it is. Select all
three across the barline and ask for the triplet. A group that still would
not fit is refused rather than shuffled about.

### Words on a note

<span class="menu-path">Insert → Text…</span> writes words beside the note
that is selected. The *kind* — expression, technique, tempo, direction,
plain — decides how they are set and where they go, so every "rit." in the
score comes out alike and they change together. **Where** overrules that for
one mark where the music wants it: over the staff or under, whatever its
kind usually does.

Click the words to pick them up: Enter opens them for changing, with what
they say already in the box, and Backspace takes them off. A note that
carries two keeps the other one.

### Printing

<span class="menu-path">File → Print</span> lays the score out on the paper
<span class="menu-path">File → Page setup</span> says it is printed on,
chooses the type size the way the PDF does, and gives each page of the
engraving a sheet of its own — so a staff is never cut in half by the edge
of the paper, and what comes out of the printer is what comes out of
<span class="menu-path">File → Export → PDF</span>. Gilbert goes in the
corner of the first sheet, as he does in the PDF, and on no other.

## Marks

Most have a single key. All are in <span class="menu-path">Edit</span> or
<span class="menu-path">Insert</span>.

| Mark | Key |
|---|---|
| Tie | <kbd>T</kbd> or <kbd>/</kbd> |
| Slur | <kbd>S</kbd> or <kbd>;</kbd> |
| Beam together, or break the beam | <kbd>b</kbd> |
| Stem up / down | <kbd>u</kbd> / <kbd>d</kbd> |
| Accent | <kbd>!</kbd> |
| Marcato | <kbd>^</kbd> |
| Staccato | <kbd>,</kbd> |
| Staccatissimo | <kbd>'</kbd> |
| Tenuto | <kbd>:</kbd> |
| Crescendo / diminuendo over the selection | <kbd>&lt;</kbd> / <kbd>&gt;</kbd> |
| Repeat the bar before | <kbd>r</kbd> |
| A word over the music | <kbd>t</kbd> |
| Chord symbol | <kbd>c</kbd> |
| Lyrics | <kbd>l</kbd> |

Also on the menus: dynamics from `pppp` to `fff`; octave lines (8va, 8vb,
15ma, 15mb); tremolos of one to four slashes, two-note tremolos, and an
unmeasured buzz roll; first- and second-time repeat endings; barlines single,
double, final and repeat.

Everything written outside the staff is placed against an outline of what is
already there, working outwards in the order engraving puts them in — trills
and ornaments nearest, then bar numbers, then dynamics and hairpins on one
line, then octave lines, directions, volta brackets, and furthest out the
rehearsal letters and the tempo. That is why marks do not collide, and why a
row of dynamics stays on one line instead of stepping down the page.

### Words, chord symbols, rehearsal marks

**Text** (<kbd>t</kbd>) asks what *kind* of writing it is, and the kind
decides how it is set and where it goes — so every "rit." in a score comes out
identical without you formatting anything.

**Chord symbols** (<kbd>c</kbd>) show you what Gscore made of what you typed
before you commit, which is the only way to find out what it thinks `C7#9` is
without writing it in first. You can have the name, a fretboard diagram, or
both.

**Rehearsal marks** are stored as a place in a sequence rather than as a
letter, so inserting one in the middle renumbers the rest. On a large score
the mark is repeated over the top staff of each bracketed family, because a
cellist at the foot of a twenty-stave page cannot read a letter eighteen
staves up.

**Lyrics** (<kbd>l</kbd>): type the verse as you would write it. Syllables go
to the notes in order, one each; hyphens break a word across notes.

### Symbols nobody's font has

<span class="menu-path">Insert → Symbol → Draw a new one…</span> is a pixel
editor with the staff lines showing, so you can see where your mark will sit.
Place the result on a note like any other articulation. It travels inside a
`.gscore` file attached to its note, so it is still a musical mark when it
arrives somewhere else rather than a floating picture.

Symbols can be imported from another `.gscore` and shared as one. Whatever is
in your symbols folder is available to every score.

## Staves and parts

<span class="menu-path">Score → Score setup…</span> is the fast start: pick
the instruments and Gscore lays the staves out in score order, brackets each
family and gives the piano its two staves.

After that, <span class="menu-path">Staff</span> adds, removes, hides,
reorders, and joins staves with a brace (one instrument) or a bracket (a
family). **Set instrument…** decides what a staff sounds like and what counts
as out of its range.

Hiding a staff is how you get the rest of the orchestra out of the way. A
hidden staff is not drawn and not played — hearing what you have just put away
is no help — but the bars keep their length, so nothing shifts under you when
it goes or comes back.

**Cross-staff writing**: <span class="menu-path">Edit → Cross staff</span>
moves a notehead to the staff above or below while it keeps belonging to the
line it is part of. The piano writing that needs it is the reason it exists.

**Parts** are the separate documents the score is printed as, one per stand.
The menu builds itself from the score; a score with none offers to make them,
and <span class="menu-path">File → Page setup…</span> has a per-part section
for what each part wants of its own page.

## Guitar, tab and drums

<span class="menu-path">Staff → Write as tablature…</span> turns a staff into
tab with a tuning. **Chord builder…** gives you a fretboard to click
fingerings onto; **Tab transposition…** handles an instrument that sounds
somewhere other than where it reads. Notes move between strings with
<span class="menu-path">Staff → Move note to the string above / below</span>.

<span class="menu-path">Insert → Tab</span> carries the marks: hammer-on,
pull-off, slide up and down, bends of a semitone or a whole tone, bend and
release, pre-bend, vibrato and wide vibrato, palm mute, let ring, natural,
artificial and pinch harmonics, tap, and the dead note.

Drum staves get input pads, because the lines name instruments rather than
pitches.

## Playing

| | |
|---|---|
| Play, pause, carry on | <kbd>Shift</kbd>+<kbd>Space</kbd> |
| Play from the beginning | <kbd>Ctrl</kbd>/<kbd>⌘</kbd>+<kbd>Shift</kbd>+<kbd>Space</kbd> |
| Stop | <kbd>Esc</kbd> |

Playback follows repeats and first- and second-time endings; a bar with a
repeat sign on it sounds as the bar it stands for, because the sign is an
instruction to the player and so it has to be one to the playback. Dynamics,
hairpins, accents, staccato, tenuto, slurs, trills and octave lines are all
heard, not just drawn. A cursor follows the music down the page and the
sounding notehead lights up.

The desktop builds carry a General MIDI soundfont, so instruments work the
first time with nothing to fetch; **Playback sound…** chooses another and
remembers it. Without a soundfont — in a browser, say — Gscore builds the
sound out of oscillators instead.

**Metronome click** puts a click on every beat at the score's own speed. It
counts the beat the way a musician does: 6/8 clicks twice in a bar, not six
times, and a one-beat pickup gets one click rather than four. It is off until
you ask for it and then remembered between sessions.

**MIDI keyboard…** plays music *in*: a key pressed writes a note at the
cursor.

## How the page is laid out

<span class="menu-path">File → Page setup…</span> sets the paper, and it is
kept with the score, because the paper decides how wide a system may be and so
how the music is laid out — not only how it is printed.

<span class="menu-path">Score → Engraving…</span> is every spacing the
engraver uses, in staff spaces, each with a picture showing in red the
distance it sets. Every change redraws the page at once. If you have opinions
about engraving, this is where they go; if you do not, the defaults are
Bravura's own and you can leave it alone for ever.

<span class="menu-path">Score → Lines of music…</span> says where a line of
music ends, overruling the engraver's fitting — and pins those bars so they
stop moving about.

<span class="menu-path">Score → Bar numbers…</span> sets how often numbers are
printed and whether they are boxed. The count restarts at each movement
already; the dialog is for the rest — a section a publisher numbered
separately, or a study that begins again at one.

<span class="menu-path">View → Multi-measure rests</span> counts a run of
silent bars as one bar with a number over it. It is off while writing, because
a bar that is not drawn is a bar you cannot click in, and on for the printed
part, because a player counting fourteen empty bars is a player who loses
count. The print and export dialogs ask for it separately for that reason.

## What the score remembers about how you were reading it

A `.gscore` file carries the drawing options it was saved with: pages or
strip, staff names, bar numbers, multi-measure rests, the tempo marking, the
beat marks, concert pitch, and the note colours. Open the file again — on
another machine, by somebody else — and it comes back looking as it was left.

It is kept for the same reason the cursor position is: it is work you did on
*this* score. Turning the beat marks on to pick apart a syncopated entry, or
colouring a part for a pupil, is a decision about this piece, and having to
make it again every time the file is opened is wearing.

Two rules follow from that, and they are worth knowing:

- **A file that says nothing is left alone.** Every score written before this
  existed, and every score imported from any other format, carries no view at
  all — and is drawn however your own settings and the defaults say. "Nothing
  recorded" and "everything turned off" are different answers.
- **Where a file names colours, those are the ones drawn.** Your own palette
  is the default for scores that have not been given one, not an override of
  those that have. Change the colours while such a score is open and your
  choice lands on it, because choosing a colour is an instruction about what
  is in front of you.

What is deliberately *not* kept with the score is the zoom. How large the
music is drawn is a fact about the screen in front of you rather than about
the piece — the same score wants one size on a laptop and another on a
monitor — so it stays with you, as the interface scale does.

## Reading aids

These change what *you* see. None of them changes the music.

- **Note colours…** — a colour per note name. C sharp takes C's colour; every
  octave matches; it is kept both as your own default and in the score you
  save, so every score you open is coloured that way and a part you send
  arrives as you prepared it; and it prints. See
  [things only Gscore does](only-gscore.md#6-colours-that-teach-reading-and-print).
- **Metronome marking** — an arrow over every beat, where the beat falls in
  that bar. The same rule the metronome click counts by, so what you see and
  what you hear cannot disagree.
- **Tempo marking** — the "Andante ♩ = 58" over the first bar. Taking it off
  does not change the speed.
- **Concert pitch** — the score as it sounds.
- **Staff names**, **Measure numbers** — ticks.
- **Flute fingerings…** and <kbd>F2</kbd> — every fingering the instrument
  has, with how far out of tune each comes and how many keys it is from the
  standard, trills included.
- **Interface size** and the zoom slider — the first is how big the menus and
  panels are drawn, which is a fact about your screen; the second is how big
  the music is.

## Import and export

Reads: ABC, Finale `.musx` and the older `.mus`, GUIDO, MuseScore
`.mscz`/`.mscx`, MusicXML (`.musicxml`, `.xml`, `.mxl`), NoteWorthy
`.nwctxt`/`.nwc`, ASCII tablature (`.tab`, `.btab`, `.txt`), and `.gscore`.

Writes: PDF, SVG, web page, MIDI, MusicXML (compressed or not), ABC,
LilyPond, GUIDO, and audio as FLAC.

An import never refuses a file. What it could not read is listed under
<span class="menu-path">Help → Lines that could not be read</span>, and
everything around it is imported.

A `.gscore` file is a ZIP of readable text that is *also* a valid PDF — rename
it `.pdf` and it opens in anything. [The format](gscore-format.md) has the
details; <span class="menu-path">File → Settings…</span> turns the PDF off if
you would rather not pay for it on every save.

For putting music in a web page, see [embedding](embedding.md).

## Analysis

Eight windows that read the music back to you: **Harmonics**, **Range**,
**Keys and modulations**, **Shape of the piece**, **Voice leading**,
**Spacing**, **Rhythm and metre**, **Melodic shape**. The
[tour](tour.md#analysis--the-menu-no-other-notation-editor-has) describes what
each is for.

## Scripting

<span class="menu-path">Insert → Script console…</span> — a handful of verbs
and the whole of JavaScript.

| Verb | Does |
|---|---|
| `note(pitch, len)` | writes a note — `note("c#4", 8)` |
| `rest(len)` | a rest |
| `chord(pitches, len)` | `chord(["c4","e4","g4"], 4)` |
| `bar()` | starts a new bar |
| `staff(instrument, opts)` | adds a staff |
| `drum(which, len)` | a drum, by name |
| `step(n, len)` | an interval from the note before |
| `write(line)` | a line of the typing language — anything the verbs miss |
| `run(name)` | any command the editor has, by its name in the shortcuts window |
| `measure(n, staff)` | reads what is written *on* a bar: clef, key, meter, mark, tempo, barline |
| `staves()`, `selection()` | reads the score and what is picked up |
| `chordNotes(symbol, octave)` | what a chord symbol is made of |
| `draw(picture, caption)`, `progress(done, total)` | show something while it runs |

The verbs are synchronous and they queue: `note()` remembers that a note was
asked for, and the whole queue is sent in order when the script finishes. That
is what lets a script read as music rather than as plumbing — no `await`
anywhere to forget. The cost is that what a script *reads* is the score as it
stood when the script began, which is worth knowing before you write a loop
that depends on it.

Limits, all deliberate: one script is one undo; there is a budget, so a loop
that writes for ever stops and says so; and the file, undo, playback and
window commands are not a script's to run.

<p class="warn"><b>A score may carry scripts, and opening it never runs
them.</b> They are shown, unrun, for you to read and run yourself.</p>

Whatever is in your `Gscore/plugins` folder appears in the
<span class="menu-path">Scripts</span> menu. Putting a file there is
installing it.

## Settings and the keyboard

<span class="menu-path">File → Settings…</span> holds the few things kept
between sessions: the interface language, and whether a saved `.gscore`
carries a PDF of itself. Your soundfont choice, note colours, interface size,
metronome and keyboard changes are remembered too.

<span class="menu-path">Help → Keyboard shortcuts</span> lists every binding
and lets you change any of them: click Change and press the keys you would
rather use. Only your changes are stored, so the defaults can improve under
you rather than being frozen to whatever Gscore did the day you first ran it.

### The whole keyboard

<kbd>Mod</kbd> is <kbd>⌘</kbd> on a Mac and <kbd>Ctrl</kbd> everywhere else.

**Writing**

| | |
|---|---|
| <kbd>Space</kbd> | write the note the palette is showing |
| <kbd>Enter</kbd> | a rest of the same length |
| <kbd>Mod</kbd>+<kbd>Space</kbd> | another notehead on the note just written |
| <kbd>Tab</kbd> | end the bar here |
| <kbd>m</kbd> | add a bar at the end |
| <kbd>Backspace</kbd> | delete what is picked up |
| <kbd>i</kbd> | write with the mouse, rather than point with it |
| <kbd>Mod</kbd>+<kbd>k</kbd> | type the music in, at the cursor |

**Selecting and moving**

| | |
|---|---|
| <kbd>←</kbd> <kbd>→</kbd> | back / on one note, or move the selection |
| <kbd>Shift</kbd>+<kbd>←</kbd> <kbd>→</kbd> | grow the selection |
| <kbd>↑</kbd> <kbd>↓</kbd> | up / down a step |
| <kbd>Shift</kbd>+<kbd>↑</kbd> <kbd>↓</kbd> | up / down an octave |
| <kbd>Alt</kbd>+<kbd>↑</kbd> <kbd>↓</kbd> | move the note itself a step |
| <kbd>Mod</kbd>+<kbd>←</kbd> <kbd>→</kbd> | move the notes earlier / later |
| <kbd>Mod</kbd>+<kbd>a</kbd> | pick up the whole staff |
| <kbd>Esc</kbd> | stop playing, or let go of the selection |
| <kbd>j</kbd> or <kbd>Mod</kbd>+<kbd>g</kbd> | go to a bar |

**The file and the window**

| | |
|---|---|
| <kbd>n</kbd> <kbd>o</kbd> <kbd>s</kbd> <kbd>p</kbd> | new, open, save, print |
| <kbd>Shift</kbd>+<kbd>P</kbd> | export as PDF |
| <kbd>e</kbd> | export as SVG |
| <kbd>Shift</kbd>+<kbd>H</kbd> | export as a web page |
| <kbd>f</kbd> or <kbd>F11</kbd> | fill the screen |
| <kbd>Mod</kbd>+<kbd>+</kbd> / <kbd>-</kbd> / <kbd>0</kbd> | bigger, smaller, usual size |
| <kbd>Mod</kbd>+<kbd>z</kbd> / <kbd>Mod</kbd>+<kbd>Shift</kbd>+<kbd>z</kbd> | undo, redo |

## Where next

<div class="next-chapter">
<img src="../icon.png" alt="">
<p><span class="where">Next chapter</span>
<a href="glossary.html"><b>Index of musical terms</b></a> — every musical word
used in this documentation, in alphabetical order, explained in a
sentence.</p>
</div>
