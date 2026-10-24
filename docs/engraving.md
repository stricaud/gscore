# Engraving decisions carried over from gscore 0.0.9

> Changing the engraver? Read [`engraving-layout.md`](engraving-layout.md)
> first: it is the rules the Go engraver follows now — how a page is laid
> out, what goes where above and below a staff, and in what order. This file
> is the history behind some of them.

This records the engraving judgment embedded in the C engine, so that the Go
rewrite preserves it deliberately rather than by accident, and discards the
rest on purpose. Line references are into `legacy/`.

The engine was `src/realize_area.c` (1,452 lines), with spacing in
`src/spacings.c`, chords in `src/chords.c`, beams in `src/beam.c` and barlines
in `src/draw_barline.c`.

---

## Keep: spacing is a tunable table, not constants

`src/include/spacings.h` defines `Spacings_t`, a tree of named spacing values
grouped by what they space — accidentals, augmentation dots, barlines, beams,
clefs, key signatures, measures, notes and rests, tempo, time signatures —
reachable from a settings dialog (`pixmaps/spcg_*.xpm` are its icons) and read
through `object_get_spacing()`.

**This was the right call and SMuFL later formalised the same idea.** The
mapping is close to one-to-one:

| `Spacings_t` field | meaning | SMuFL `engravingDefaults` |
|---|---|---|
| `Barlines.tlt` | thin line thickness | `thinBarlineThickness` |
| `Barlines.hlt` | heavy line thickness | `thickBarlineThickness` |
| `Barlines.sbhatl` | space, heavy to thin | `barlineSeparation` |
| `Barlines.swdb` | space within double barline | `doubleBarlineSeparation` |
| `Beams.bt` | beam thickness | `beamThickness` |
| `Accidentals.sbaan` | accidental to notehead | (no default; keep ours) |
| `Accidentals.sbaon` | between stacked accidentals | (no default; keep ours) |
| `AugmentationDots.sbdan` | dot to notehead | (no default; keep ours) |
| `AugmentationDots.sbd` | between dots | (no default; keep ours) |
| `KeySignatures.sbksa` | between signature accidentals | (no default; keep ours) |
| `Clefs.sb` / `Clefs.sa` | space before / after clef | (no default; keep ours) |

So the Go `engrave` package keeps this structure, loads SMuFL's
`engravingDefaults` as the defaults, and leaves every field user-overridable.
The fields with no SMuFL equivalent stay as gscore's own, which is the whole
reason to keep the struct rather than lean on the font alone.

One change: the values were `gint` **pixels**. They become `float64` **staff
spaces**, which is SMuFL's unit and the only way the same table can serve
screen, print and PDF.

## Keep: stem direction from the staff midpoint, with manual override

`realize_area.c:605`:

```c
if ( (y < staff->extremity_begin_y + average) || object->nature & O_STEMDOWN )
        stemup = FALSE;
else
        stemup = TRUE;
if ( object->nature & O_STEMUP )
        stemup = TRUE;
```

`average` is half the staff height, so the middle line is the pivot: notes
above it get stems down, notes on or below get stems up, and `O_STEMUP` /
`O_STEMDOWN` on the object override the computation. That is standard practice
and the override is essential for voiced music. **Carry it over unchanged**,
with the midpoint expressed as a staff position rather than a y pixel.

## Keep the intent, fix the logic: noteheads a second apart are displaced

`chords.c:93` tries to detect an adjacent pitch in the same chord and push its
notehead across the stem:

```c
if ( ( ++pitch == object->pitch ) || ( --pitch == object->pitch ) )
        *notehead_left = FALSE;
```

Knowing that seconds must be displaced is real engraving knowledge and worth
keeping. **The implementation is broken, though, in two ways:**

1. `++pitch` then `--pitch` leaves the second comparison testing the
   *original* pitch, not `pitch - 1`. The "second below" case is never
   detected; only the second above is.
2. `||` short-circuits, so when the first test matches, `pitch` stays
   incremented for every later iteration of the loop and the comparisons drift
   against a corrupted value.

The Go version computes the displaced set properly: sort the chord's pitches,
walk adjacent pairs, and displace on the side away from the stem.

## Reconsider: chord stem direction should come from the outer notes

`make_chord()` takes the stem direction from whichever note happens to carry
`id == group_id` — the chord's first-entered note. Standard practice is to
decide from the note furthest from the middle line, so that the stem serves the
whole chord. There is also a `FIXME` admitting the group breaks when that root
note is deleted. The Go model removes the cause by holding `Chord.Notes`
together instead of linking noteheads by `group_id`.

## Reconsider: the single-pass fixed-width spacing

`object_get_spacing()` returns a fixed "space after" per note value, and x
accumulates left to right (`xpos += object_get_spacing(object->type)` in
`chords.c:100`, `Staff_t.current_x`). Consequences:

- **No justification.** Nothing can stretch or compress a run of music to meet
  a margin, because each duration's width is a constant.
- **Quadratic.** `make_chord()` re-walks the whole object list, summing
  spacings from the start of the staff, once per chord note.
- **Durations that aren't in the table have no width** — the table is keyed by
  note value, so a tuplet member has nowhere to come from.

Replace with proportional spacing: an ideal width per element derived from its
duration, a minimum width from the `Spacings` table so collisions stay
impossible, and a second pass that distributes slack. The `Spacings` values
become the *floor*, which is what they are actually good at, rather than the
answer.

## Reconsider: "non-page-oriented" has to become a view mode

There is no line breaking, system breaking or justification anywhere in the
engine — `grep` for it finds nothing, and `draw.c:318` simply grows the canvas:

```c
score->width = x2 + 300;
```

The score is one infinite horizontal strip. `draw_page_limit()` draws a marker
where a page edge would fall without ever breaking there.

This was the program's stated identity, and it is genuinely good for editing —
it is why the fixed spacing above was tenable at all. But the 0.0.9 `TODO`
lists printing support as a 1.0 goal, and printing needs breaking and
justification. So: **keep the infinite strip as the editing view**, and add a
paginated view that shares the same spacing engine. Both are consumers of
`model.Score`; neither is the model's business.

---

## What this means for the Go `engrave` package

```
engrave.Spacings        the Spacings_t tree, in staff spaces, SMuFL-seeded
engrave.Engrave(score, spacings, mode) -> Layout
  mode = Strip          the 0.0.9 behaviour, one unbroken system
  mode = Paginated      breaks and justifies to a page
Layout                  glyphs, lines and beams with staff-space coordinates
```

`Engrave` is a pure function, so the whole of the above is testable against
`legacy/samples/abc/boite-musique.png` — the output the C engine actually
produced in 2004, kept in `testdata/reference/`.
