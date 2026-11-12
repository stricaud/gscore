# gscore

A musical score editor built for writing music fast, by Sébastien Tricaud.

One key per note value and one per mark, a typing bar that writes a bar in a
second — `8abcdef` is six eighth notes climbing — and a continuous strip of
music rather than a page to fight while you work. Then it plays, prints, and
goes out as a link that holds the notes themselves.

gscore was written in C and GTK+ between 2001 and 2005, and released up to
0.0.9 "harder, better, stronger, faster" in April 2005. It was a
*non-page-oriented* notation program: no page breaks, no justification, one
unbroken strip of music designed for editing rather than printing. Mandrake and
Mandriva packaged it from December 2004, and OpenMandriva still ships it today.

This tree is the 2026 rewrite. The C version of Gscore still remains in this 
repository's history, and the decisions it made are written down in [`docs/engraving.md`](docs/engraving.md).

The version lives in the [`VERSION`](VERSION) file at the root, and nowhere
else: the packages, the macOS `Info.plist` and the program's own `--version`
all read it, so they cannot disagree — and neither can this paragraph, which
is why it no longer names a number. Tagging a commit builds the installers and
publishes them — see
[`.github/workflows/release.yml`](.github/workflows/release.yml).

## Install

From [the latest release](https://github.com/stricaud/gscore/releases/latest):

| | Download | Then |
|---|---|---|
| macOS | `Gscore-<version>.dmg` | Open it, drag Gscore to Applications |
| Windows | `gscore-<version>-windows-setup.exe` | Run it |
| Linux | `gscore-<version>-linux-amd64.tar.gz` | Unpack, run `./gscore`, or `sudo ./install.sh` |
| Debian, Ubuntu | `gscore_<version>_amd64.deb` | `sudo apt install ./gscore_<version>_amd64.deb` |

Each one carries a General MIDI soundfont, so it plays real instruments the
first time it is opened with nothing to fetch. None of them is signed with a
paid certificate, so macOS and Windows will both want telling once that you
meant to open it; the release notes say how.

Or build it: `make` lists the targets.

## Status

There is an editor. It writes, plays, analyses, imports eight formats and
exports nine, and it runs on a desktop and in a browser from the same source.

The order it was built in was deliberate and is worth knowing, because it is
why the tests are where they are: the model, the importers and the engraver
were finished as a headless library first, and the window came last. Every
musical decision is therefore testable without a screen, and the interface is
a view of a thing that already worked.

| | |
|---|---|
| Model | pitches, durations, chords, voices, measures, parts, no geometry |
| Import | ABC, MusicXML, MuseScore, Finale `.musx` and `.mus`, NoteWorthy, Guitar Pro `.gp3`/`.gp4`/`.gp5`, ASCII tab, GUIDO, `.gscore`, engraved PDF |
| Export | PDF, SVG, web page, MIDI, MusicXML, ABC, LilyPond, GUIDO, FLAC |
| Engraving | the rules are in [`docs/engraving-layout.md`](docs/engraving-layout.md) |
| Rendering | SVG and PDF from one layout, measured in staff spaces |
| Editor | a native window around a web interface, and the same interface as a page |
| Playback | a General MIDI soundfont, or oscillators where there is no disk |
| Analysis | harmonics, range, keys, shape, voice leading, spacing, rhythm, melody |


### Reading an engraved PDF

A PDF made by a notation program still contains the music, glyph by glyph: a
notehead is a named glyph at a position good to six decimal places, so its
pitch is how far it sits above the bottom staff line. That is subtraction,
not recognition — on a file from LilyPond or MuseScore the notes that come
out are the notes that went in. A PDF that was *scanned* is a photograph and
cannot be read this way; it says so rather than guessing.

What is read: notes with their lengths, accidentals and dots, clefs, key and
time signatures, rests, barlines. What is not, and has to be put back by
hand: slurs and ties, which are curves carrying no pitch, and the dynamics,
articulations, lyrics, tuplets and directions.

One thing a PDF never says is **how many staves make a system** — which is to
say how many players are reading at once. It has to be inferred from a line
crossing the gap between two staves, and where an engraver draws each staff's
barlines separately there is nothing to find: every staff becomes a system of
its own, and the whole file arrives as one part thousands of bars long. That
is not a subtle failure but it is an invisible one, because the score that
comes out is perfectly well formed.

So opening a PDF shows the reading rather than asserting it. The page is
drawn back as the reader perceived it — the staves it found, coloured by the
part each went into, every notehead ringed with the pitch it was given, over
the strokes the page is actually made of — with the number of instrument
lines beside it to overrule the guess and read again. Where that drawing and
the real page disagree, the disagreement is the mistake, and it is visible at
a glance.

A PDF also never says **what each staff is**, so the staves arrive nameless
and are played as pianos — and **Score setup** opens on them as soon as the
reading is settled, listing them one by one in the order they are on the
page. Over a score that already has music, nothing in that window lays the
staves out again: adding, removing, moving and bracketing go straight to the
score as they are done, each undoable on its own, and setting an instrument
changes that staff and nothing else. A staff takes the name of what it has
been told it plays. Two staves given a piano, a harp or another instrument
written on two become a single braced grand staff — whereas two staves given
a flute stay Flute 1 and Flute 2, because two flutes are two players however
close together they are printed.

The window is a [`webview`](https://github.com/webview/webview_go) around a
local HTTP server — WebKit on macOS, WebKitGTK on Linux, WebView2 on Windows —
and the *same* interface compiles to WebAssembly and runs with no server at
all. One interface, two ways of hosting it, rather than two interfaces to keep
in step.

```
go build ./...
go test ./...
go run . samples/altes-10e-solo.abc          # opens the editor
go run . -o out.pdf samples/the-entertainer.mxl
sh web/build.sh                   # the site, including these documents
```

## What changed, and why

**Go, not C.** 15,000 lines of C with manual memory management and GLib linked
lists throughout. Of that, roughly 5,500 lines carried the music and 6,900 were
GTK2 dialog plumbing.

**No GTK.** Not a preference — a necessity. The UI was built on libglade, which
has been obsolete since 2008 and is unavailable on macOS, and drew through the
`gdk_draw_*` family, which GTK3 removed outright. Reviving the old interface
would have cost more than replacing it.

**No CMake, no autotools, no SCons.** The tree has carried all three over its
life. `go build` now.

**The model holds no geometry.** The 0.0.9 `Object_t` stored six screen
coordinates on every musical object, and `Score_t` held widths, heights and a
window title. Engraving is now a pure function from music to geometry, measured
in staff spaces rather than pixels, which is what lets the same engine serve
screen, print and PDF.

**Real pitches.** `pitch` used to be a staff position. It is now a step, an
octave and an alteration, so that transposition keeps its spelling and what
*sounds* stays separate from what is *printed*.

**Voices.** The old model had none, which is why its own sample file — a
three-voice piece, `V:0` through `V:2` — could not round-trip.

**Compiled-in formats.** The dlopen plugin loader is a registry. Go's `plugin`
package does not work on Windows and demands an identical toolchain where it
does. ABC no longer needs the external libabcp, whose build problems the 0.0.9
release notes had to apologise for.

**A music font.** 287 XPM bitmaps become SMuFL glyphs, with Bravura as the
default. Hand-drawn symbols remain fully supported for everything no font
carries — see [`docs/symbols.md`](docs/symbols.md).

## In a web page

The whole program compiles to WebAssembly, so a page about a piece of music can
show it and play it with nothing behind it but a file host:

```html
<iframe src="https://stricaud.github.io/gscore/?player=1&score=tune.abc"
        width="100%" height="220" style="border:0"></iframe>
```

Player mode takes away the menus, the palette and the inspector and leaves the
score and a play button. There is a working page in
[`samples/embed.html`](samples/embed.html) and the settings are in
[`docs/embedding.md`](docs/embedding.md).

## Documents

The whole of this is published at
[stricaud.github.io/gscore/docs/](https://stricaud.github.io/gscore/docs/),
built from these files by `web/build.sh`. `docs/ORDER` decides what the index
lists first.

For anybody using gscore:

| | |
|---|---|
| [`docs/quickstart.md`](docs/quickstart.md) | twenty minutes from opening it to a printed part |
| [`docs/tour.md`](docs/tour.md) | every window gscore can open, and what it is for |
| [`docs/only-gscore.md`](docs/only-gscore.md) | the nine things no other notation editor does |
| [`docs/manual.md`](docs/manual.md) | the user manual, in full |
| [`docs/curl.md`](docs/curl.md) | writing, engraving and recording a score with `curl` alone |
| [`docs/glossary.md`](docs/glossary.md) | an index of the musical terms used |

For anybody changing it:

| | |
|---|---|
| [`docs/engraving-layout.md`](docs/engraving-layout.md) | how a page is laid out: the rules to read before changing the engraver |
| [`docs/engraving.md`](docs/engraving.md) | what the 2001 engine decided, kept or discarded on purpose |
| [`docs/symbols.md`](docs/symbols.md) | drawing marks no font carries |
| [`docs/gscore-format.md`](docs/gscore-format.md) | what is inside a `.gscore` file |
| [`docs/finale-mus.md`](docs/finale-mus.md) | reading Finale's older `.mus`, which is really two formats |
| [`docs/embedding.md`](docs/embedding.md) | putting gscore in a page |
| [`docs/presentation.tex`](docs/presentation.tex) | the talk — `make slides` builds it |

## Layout

The one rule: music goes in, geometry comes out, and the two never meet in
the same package. `model` knows no pixels; `engrave` knows no files; `render`
knows no music.

```
main.go              the entry point: a window, a server, or one file in and out

internal/model/      music, with no geometry and no I/O
internal/format/     the import and export registry, one package per format
internal/engrave/    music in, shapes out — the layout, in staff spaces
internal/render/     those shapes as SVG, PDF or a standalone page
internal/app/        the editor: HTTP endpoints, and ui/ is the interface
internal/desktop/    the native window around it
internal/analysis/   reading the music back: keys, range, voice leading
internal/smufl/      the music font and the glyph names
internal/symbol/     hand-drawn marks, for what no font carries
internal/synth/      playback, and internal/midi, internal/flac beside it
internal/flute/      every fingering the instrument has
internal/brand/      Gilbert
internal/pdf/        a PDF writer, and internal/sfnt reads the font for it

web/                 the WebAssembly build and the site generator
docs/                the documentation — user chapters and design decisions
samples/             scores, and an embeddable page to put one in
packaging/           what the installers are built from
assets/history/      a screenshot of gscore 0.0.9, from 2005
testdata/reference/  output the C engine produced in 2004
```

## Licence

MIT. See [LICENSE](LICENSE).

The music font is Bravura, under the SIL Open Font License, and keeps that
licence wherever it travels; the LICENSE file says so and names everything
else in here that is somebody else's work.

Gilbert — the mascot in `internal/brand/` and the icon in `assets/icon/` —
was the splash screen in 2005 and predates all of this.
