# gscore

A musical score editor, by Sébastien Tricaud.

gscore was written in C and GTK+ between 2001 and 2005, and released up to
0.0.9 "harder, better, stronger, faster" in April 2005. It was a
*non-page-oriented* notation program: no page breaks, no justification, one
unbroken strip of music designed for editing rather than printing. Mandrake and
Mandriva packaged it from December 2004, and OpenMandriva still ships it today.

This tree is the 2026 rewrite. The original is preserved intact in
[`legacy/`](legacy/) and remains the specification.

Version 0.1.0. The series restarts because nothing but the music is carried
over.

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
go run ./cmd/gscore -v legacy/samples/abc/boite-musique.abc
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
| [`docs/engraving.md`](docs/engraving.md) | how the layout is decided |
| [`docs/symbols.md`](docs/symbols.md) | drawing marks no font carries |
| [`docs/gscore-format.md`](docs/gscore-format.md) | what is inside a `.gscore` file |
| [`docs/embedding.md`](docs/embedding.md) | putting gscore in a page |

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
legacy/              gscore 0.0.9, unmodified
testdata/reference/  output the C engine produced in 2004
```

## Licence

GPL v2 or later, as the original. See [LICENSE](LICENSE).

The sample scores and all 287 symbols in `legacy/` are part of the original
release. Gilbert, the mascot in `legacy/pixmaps/gilbert.png` and the icon in
`assets/icon/`, predates all of this.
