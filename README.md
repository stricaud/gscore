# gscore

A musical score editor, by Sébastien Tricaud.

gscore was written in C and GTK+ between 2001 and 2005, and released up to
0.0.9 "harder, better, stronger, faster" in April 2005. It was a
*non-page-oriented* notation program: no page breaks, no justification, one
unbroken strip of music designed for editing rather than printing. Mandrake and
Mandriva packaged it from December 2004, and OpenMandriva still ships it today.

This tree is the 2026 rewrite. The 2001–2005 C source is not in it: it was
somebody else's work in part — two other people contributed to it, and one
header came from the `abc2midi` package under the GPL — so it could not come
along to an MIT-licensed tree. It remains in this repository's history, and
the decisions it made are written down in [`docs/engraving.md`](docs/engraving.md).

Version 0.1.1, from the [`VERSION`](VERSION) file at the root: the packages,
the macOS `Info.plist` and the program's own `--version` all read it, so they
cannot disagree. Tagging a commit builds the installers and publishes them —
see [`.github/workflows/release.yml`](.github/workflows/release.yml).

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

Headless so far — there is no editor yet, by design. The model, the importers
and the engraver are being built as a library with tests before any window
exists, so that both halves stay testable.

| | |
|---|---|
| Model | done — pitches, durations, chords, voices, measures, parts |
| ABC import | done — no external dependency |
| Symbols | done — SMuFL-ready, with XPM and hand-drawn grids |
| Engraving | next, specified in [`docs/engraving.md`](docs/engraving.md) |
| SVG rendering | after that |
| Editor | Wails, once the above draws correctly |

```
go build ./...
go test ./...
go run . samples/exercise.abc
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

| | |
|---|---|
| [`docs/engraving-layout.md`](docs/engraving-layout.md) | how a page is laid out: the rules to read before changing the engraver |
| [`docs/engraving.md`](docs/engraving.md) | what the 2001 engine decided, kept or discarded on purpose |
| [`docs/symbols.md`](docs/symbols.md) | drawing marks no font carries |
| [`docs/gscore-format.md`](docs/gscore-format.md) | what is inside a `.gscore` file |
| [`docs/embedding.md`](docs/embedding.md) | putting gscore in a page |
| [`docs/comparison.md`](docs/comparison.md) | gscore beside MuseScore, NoteWorthy and Frescobaldi |
| [`docs/presentation.tex`](docs/presentation.tex) | the talk — `make slides` builds it |

## Layout

```
cmd/gscore/          the command-line entry point
internal/model/      music, with no geometry and no I/O
internal/format/     the import and export registry
internal/format/abc/ the ABC reader
internal/symbol/     font glyphs, outlines and hand-drawn grids
internal/version/
assets/icon/         Gilbert, the application icon
docs/                design decisions
samples/             an embeddable page, and a score to put in it
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
