# Symbols: drawing anything, including what no font has

Notation has a long tail. A composer invents an articulation; an editor needs a
neume, a percussion pictogram, a microtonal accidental, a fingering glyph for
an instrument nobody standardised. A program that can only draw what its music
font knows cannot set those pieces at all.

gscore could always draw them, in its own way. The C program of 2001–2005 had
no music font at all: every one of its 287 symbols was an XPM file — a pixel
grid written as C source and editable by hand in any text editor — drawn one
at a time. That capability is kept, and generalised.

## Three sources, one abstraction

`internal/symbol` resolves every drawable mark through one type, whatever it is
made of:

| Kind | Artwork | Scales | Use for |
|---|---|---|---|
| `FromFont` | a SMuFL codepoint | yes | everything standard |
| `FromVector` | an outline, SVG path syntax | yes | a custom symbol that must print sharply |
| `FromPixels` | a hand-drawn grid | no | a symbol that exists nowhere else |

The engraver asks the registry for a name and gets back something drawable. It
never learns which kind it got, so a symbol can start as a sketched grid and
later become an outline without the score changing.

## Everything is measured in staff spaces

A grid of pixels cannot be placed on a staff on its own: it has no musical size
and no idea which of its pixels should land on the note's position. XPM records
neither, which is exactly why the C engine had to hardcode both — every pixmap
was drawn at an offset worked out by hand, in the drawing code. Both are
explicit here:

- **`PixelSize`** — how much of a staff space one pixel covers. The default,
  `1/8`, is not arbitrary: 0.0.9 ruled every staff with eight pixels between
  the lines, so all 287 of its pixmaps were drawn to that scale. It shows in
  the files themselves — `noteheads-2.xpm` is 9×8 pixels, which at this scale
  is exactly one staff space tall, as a notehead must be, and `clefs-G.xpm` is
  18×53, which comes out at 6.6 staff spaces, right for a treble clef.
- **`OriginX`, `OriginY`** — the registration point inside the grid, the pixel
  placed at the symbol's staff position. The default is the left edge,
  vertically centred, which is what a notehead or any staff-positioned symbol
  wants.

Because of this, a hand-drawn symbol zooms and transposes with everything else.
It does not gain resolution, but it is never the wrong size.

## Reading XPM

`symbol.DecodeXPM` reads the format directly, and reads all of it: one and two
characters per pixel, up to 4,501 colours, `None` transparency,
`#RGB`/`#RRGGBB`/`#RRRRGGGGBBBB` hex, named colours, and X11's `gray0`–`gray100`
percentage greys.

That last form is in there because the decoder was once run over the whole
0.0.9 symbol set and seventeen toolbar pixmaps turned out to use it — the kind
of thing no specification tells you and only real files do. A handful of those
files are kept as fixtures in `internal/symbol/testdata`, chosen for being the
awkward ones, and the tests decode them on every run.

So any symbol drawn for the old program is still usable, and XPM stays a
supported authoring format: a symbol can be written by typing.

## Attaching a symbol to music

The model refers to symbols by name only and stores nothing about how they
look:

```go
// On a note or chord, alongside the built-in articulations:
note.Symbols = append(note.Symbols, model.SymbolMark{
        Name:      "myOwnMark",
        Placement: model.PlacementAbove,
})

// Or standing alone in a voice, taking room but no time:
voice.Elements = append(voice.Elements, &model.SymbolElement{
        Symbol: model.SymbolMark{Name: "segno"},
})
```

`Placement` is `Above`, `Below` or `Auto` — a musical decision, not a
coordinate. The engraver turns it into a position, which keeps the rule that
the model holds no geometry.

A user symbol may take a name the font already defines; the registry lets the
later definition win, which is how someone replaces a standard notehead with
their own drawing.

## What a bitmap symbol costs

A grid has a fixed resolution. On screen it is indistinguishable from a glyph;
in print at 1200 dpi, a symbol drawn at eight pixels per staff space will look
soft beside font glyphs around it. That is a real limitation and not worth
hiding.

The intended path is that drawing by hand is how a symbol *starts*. The pixel
editor is the fastest way to get a mark onto the page; tracing a finished grid
to an outline, or building a handful of finished symbols into a font in SMuFL's
private range at U+F400–U+F8FF, is how it becomes permanent. Nothing in the
score has to change when it does, because the score only ever knew the name.
