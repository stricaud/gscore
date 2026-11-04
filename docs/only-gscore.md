# Things only Gscore does

Nine things this program does that you will not find together anywhere else — and in a couple of cases, anywhere else at all.

Every notation program writes notes on staves. That part is solved. What
follows is the list of things Gscore does that made it worth writing, roughly
in order of how surprised people are when they see them.

## 1. The file is also a PDF

Save a score as `.gscore`. Now rename it `.pdf` and double-click it.

It opens. In Preview, in Acrobat, in your browser, in whatever your phone
uses — a proper, readable, printable PDF of the music.

Rename the same file `.zip` and open *that*, and you get plain readable text:
the music as JSON, the hand-drawn symbols as images, a little manifest saying
what version wrote it.

One file. Three honest answers to "what is this?", depending on what you need
it to be. Send it to a player who has nothing installed and they can print it.
Send it to somebody with Gscore and they can edit it. Open it in ten years
with neither and you can still read every byte of it.

<p class="note">If you would rather not pay for the PDF on every save — it is
a full rendering of the score each time — <span class="menu-path">File →
Settings…</span> turns it off. It is on by default because being readable by
everybody is most of what the format is for. The innards are documented in
<a href="gscore-format.html">the <code>.gscore</code> format</a>.</p>

## 2. The whole program runs in a web page

Not a viewer. Not a cut-down demo. The entire editor, compiled to WebAssembly,
running at [the editor](../app/) with a file host behind it and nothing else —
no server, no database, no account.

Which means you can also put a piece of music in *your* page:

```html
<iframe src="https://stricaud.github.io/gscore/?player=1&score=tune.abc"
        width="100%" height="220" style="border:0"></iframe>
```

Player mode takes away the menus, the palette and the inspector and leaves the
score and a play button. A blog post about a tune can show the tune, and play
it, and the reader needs nothing. See [putting Gscore in a page](embedding.md).

And <span class="menu-path">File → Export → Web page…</span> hands you a
single `.html` file with the music, the music font and the player *inside* it.
Drop it on any host, or email it. It works where it lands.

## 3. It reads what the other programs wrote

| From | Extension |
|---|---|
| ABC | `.abc` |
| Finale | `.musx` |
| GUIDO | `.gmn` |
| MuseScore | `.mscz`, `.mscx` |
| MusicXML | `.musicxml`, `.xml`, `.mxl` |
| NoteWorthy Composer | `.nwctxt`, `.nwc` |
| Gscore | `.gscore` |

Finale's own format. MuseScore's own format. NoteWorthy's own format. Not
"export MusicXML from the other program first" — the actual files, directly.
For anybody with a folder of work in a program they have stopped paying for,
that is the whole ballgame.

And it writes back out as PDF, SVG, web page, MIDI, MusicXML, ABC, LilyPond,
GUIDO and FLAC audio.

<p class="tip"><b>It never refuses a file.</b> Hit a mistyped character, a
wrapped line, a construction it does not know? It reads everything around the
problem and lists what it skipped under <span class="menu-path">Help → Lines
that could not be read</span>. A thirty-page score with one bad byte opens as
a thirty-page score. Programs that throw the whole file away over one
character are the reason people keep paper.</p>

## 4. You can write a program that writes the music

<span class="menu-path">Insert → Script console…</span>:

```js
// A canon at the fifth: the tune, then the tune again a fifth up.
const tune = ["c4","d4","e4","g4","f4","e4","d4","c4"];
for (const p of tune) note(p, 4);
bar();
for (const p of tune) note(p, 4);
run("transpose-up");   // the editor's own command, by name
```

A handful of verbs — `note`, `rest`, `chord`, `bar`, `staff`, `drum`, and a
`write()` that takes a line of the typing language — plus `run()`, which
reaches **any command the editor has**, by the name the shortcuts window lists
it under. Everything the program can do to music is already written and named;
a scripting system that could not reach it would be reinventing the program in
a worse language.

Four things make this a tool rather than a liability:

- **The verbs are few.** The power is in JavaScript, which you already know or
  can look up, not in a music language nobody has ever written in.
- **One script is one undo.** Two hundred notes come back in one press. A
  script you cannot undo is a script nobody dares run.
- **There is a budget.** A loop that writes for ever stops and says so.
- **Nothing in a file ever runs by itself.** A score carries its scripts and
  opening it *shows* them, unrun. Word spent a decade undoing that mistake;
  Gscore declines to repeat it.

## 5. It analyses the music, not just the notation

Eight windows under <span class="menu-path">Analysis</span> that read the
score back to you: harmonics, range, keys and modulations, the shape of the
piece, voice leading, spacing, rhythm and metre, melodic shape.

The two that earn their keep fastest:

- **Range** tells you whether the instrument you wrote for can actually play
  what you wrote. Before the rehearsal, not during it.
- **Keys and modulations** reads the key from *the notes*, not from the key
  signature — so it will tell you the passage has gone to the relative minor
  whether or not anybody wrote the accidentals as a signature.

This is the kind of thing a teacher says while looking over your shoulder. It
is not in notation programs because notation programs think their job ends at
the page.

## 6. Colours that teach reading, and print

<span class="menu-path">View → Note colours…</span> gives every note name its
own colour — C red, D orange, up the spectrum to a violet B, which is the
order coloured bells and keyboard stickers already use.

What makes it work rather than merely exist:

- **C sharp is a C**, so it takes C's colour. The colour teaches the *name*.
- **Every octave the same**, so a C is a C all the way up.
- **It is yours, not the score's.** Set it once and every score you open
  afterwards is coloured that way. A beginner should not have to configure a
  program before they can practise.
- **It prints.** PDF and paper, not just screen. The student takes the part
  home and the colours come with it.

Alongside it, <span class="menu-path">View → Metronome marking</span> draws an
arrow over every beat — not "♩ = 120" at the top of the page, but a mark at
each beat *where it actually falls in that bar*, which is what somebody
counting a tied entry or a bar of 7/8 needs to see. Turn on
<span class="menu-path">Play → Metronome click</span> and the click lands
exactly where the arrows are, because both are counted from the same rule.

## 7. Every fingering the flute has

Not a chart. The full table — twenty-nine thousand fingerings — with how far
out of tune each comes out and how many keys it is from the standard.

Which means the alternate you want is findable: one key from the standard, and
in tune. It covers trills, which are a pair of notes rather than a note.
<span class="menu-path">View → Flute fingerings…</span>, or <kbd>F2</kbd> to
show the fingering for whatever note you are on as you write.

## 8. Draw the mark that does not exist

<span class="menu-path">Insert → Symbol → Draw a new one…</span> opens a small
pixel editor with the staff lines showing, so you can see where your mark will
sit. Draw it. Now place it on a note like any other articulation.

Then — and this is the part that matters — **it travels with the file**.
MusicXML cannot carry a symbol somebody invented. MuseScore can embed a
picture, but only as decoration floating on the page with no musical meaning,
so your invented articulation arrives at the other end as a smudge. In a
`.gscore` the artwork is attached to the note it belongs to, so the score is
still a score when it comes back, and one person's set of marks can be handed
to another as a file.

## 9. It was built for writing, not for printing

Gscore opens as a **continuous strip**: one unbroken line of music, no page
breaks, no justification. That was the original program's defining decision in
2001 and it survived the rewrite on purpose, because when you are *writing*,
pagination is noise. <span class="menu-path">View → Paginated</span> when you
want to see what the printer will see.

The same instinct runs through the small things:

- A note too long for the bar pushes the overflow into the next bar and keeps
  the arithmetic right — unless you turn that off, because music barred by
  phrase needs bars of any length.
- A bar you are part-way through keeps the room the rest of it will take, so
  there is somewhere to put the cursor. Printed, a short bar is short on
  purpose.
- There is always one empty bar drawn past the end of the music, so there is
  somewhere to click to carry on. It becomes real when you write in it.
- A pickup measure leaves the bar short and makes bar one the first *full*
  bar, so your numbering matches the published edition.

None of these is a feature anybody puts on a box. Together they are the
difference between a program you write music in and a program you fight.

## Where next

<div class="next-chapter">
<img src="../icon.png" alt="">
<p><span class="where">Next chapter</span>
<a href="manual.html"><b>The user manual</b></a> — all of the above and
everything else, in order, for when you need the detail rather than the
pitch.</p>
</div>

- [Quick start](quickstart.md) — if you have not written your first eight bars yet.
- [Index of musical terms](glossary.md) — every musical word used here.
