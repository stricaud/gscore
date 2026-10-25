# Engraving decisions carried over from gscore 0.0.9

> Changing the engraver? Read [`engraving-layout.md`](engraving-layout.md)
> first: it is the rules the Go engraver follows now — how a page is laid
> out, what goes where above and below a staff, and in what order. This file
> is the history behind some of them.

This records the engraving judgment that was built into gscore 0.0.9, the C
and GTK+ program written between 2001 and 2005, so that the Go rewrite keeps
it deliberately rather than by accident and discards the rest on purpose.

That C source is not in this repository. It was not all one person's to
relicense when gscore moved to MIT, so it was taken out; what it *decided*
is written down here, which is the part worth carrying. The engine was
around fifteen hundred lines of drawing code, with spacing, chords, beams
and barlines each in a file of their own.

---

## Keep: spacing is a tunable table, not constants

0.0.9 kept every spacing measurement in one structure: named values grouped
by what they space — accidentals, augmentation dots, barlines, beams, clefs,
key signatures, measures, notes and rests, tempo, time signatures. It had
icons and a settings dialog, so the values belonged to the user rather than
being constants buried in the drawing code.

**This was the right call and SMuFL later formalised the same idea.** The
mapping is close to one-to-one:

| what 0.0.9 measured | meaning | SMuFL `engravingDefaults` |
|---|---|---|
| thin barline | thin line thickness | `thinBarlineThickness` |
| heavy barline | heavy line thickness | `thickBarlineThickness` |
| heavy to thin | space between them | `barlineSeparation` |
| within a double barline | space between the lines | `doubleBarlineSeparation` |
| beam | beam thickness | `beamThickness` |
| accidental to notehead | the air after an accidental | (no default; keep ours) |
| between accidentals | stacked in a chord | (no default; keep ours) |
| dot to notehead | the air before a dot | (no default; keep ours) |
| between dots | a double dot | (no default; keep ours) |
| between signature accidentals | inside a key signature | (no default; keep ours) |
| before / after a clef | the air either side | (no default; keep ours) |

So the Go `engrave` package keeps this structure, loads SMuFL's
`engravingDefaults` as the defaults, and leaves every field user-overridable.
The measurements with no SMuFL equivalent stay as gscore's own, which is the
whole reason to keep a table rather than lean on the font alone.

One change: the values were integer **pixels**. They are now floating-point
**staff spaces**, which is SMuFL's unit and the only way one table can serve
the screen, the printer and a PDF at once.

## Keep: stem direction from the staff midpoint, with manual override

The pivot was half the staff height — the middle line. A note above it got a
stem down, a note on or below it got a stem up, and a flag on the note could
override the computation either way.

That is standard practice, and the override is what makes two voices on one
staff writable at all. **Carried over unchanged**, with the midpoint
expressed as a position on the staff rather than as a pixel.

## Keep the intent, fix the logic: noteheads a second apart are displaced

The C engine looked for a pitch a second away inside the same chord and
pushed that notehead across to the other side of the stem. Knowing that
seconds must be displaced is real engraving knowledge and worth keeping.

**The implementation was wrong in two ways**, which is why the rewrite had to
re-derive the rule rather than translate the code:

1. The test for the note *below* compared against a value that had already
   been changed back, so only the second **above** was ever detected.
2. The two tests were joined by a short-circuiting `or`. Once the first
   matched, the value it had modified stayed modified for the rest of the
   loop, and every later comparison drifted against it.

The Go version computes the displaced set properly: sort the chord's pitches,
walk adjacent pairs, and displace on the side away from the stem. A cluster
of three alternates, so the third note comes back to the first column —
which is the part the old rule could not express at all.

## Reconsider: chord stem direction should come from the outer notes

0.0.9 took a chord's stem direction from whichever note had been entered
first, because that was the note the rest were linked to. Standard practice
is to decide from the note furthest from the middle line, so that one stem
serves the whole chord.

The old code carried a `FIXME` admitting that a chord fell apart when that
first note was deleted — the hazard of linking noteheads to a root instead of
holding them together. The Go model removes the cause: a chord owns its
notes, and none of them is special.

## Reconsider: the single-pass fixed-width spacing

Each note value had a fixed "space after" and x accumulated left to right.
Consequences:

- **No justification.** Nothing could stretch or compress a run of music to
  meet a margin, because each duration's width was a constant.
- **Quadratic.** Placing one chord re-walked the whole bar, summing widths
  from the start of the staff, once per notehead.
- **Durations not in the table had no width at all.** The table was keyed by
  note value, so a tuplet member had nowhere to come from.

Replaced with proportional spacing: an ideal width per element derived from
its duration, a minimum width from the spacing table so collisions stay
impossible, and a second pass that distributes the slack. The old values
become the *floor*, which is what they were always good at, rather than the
answer.

## Reconsider: "non-page-oriented" has to become a view mode

There was no line breaking, system breaking or justification anywhere in the
engine. The canvas simply grew to the right as music was added, and a score
was one unbroken horizontal strip. A marker was drawn where a page edge would
fall, without ever breaking there.

This was the program's stated identity, and it is genuinely good for editing
— it is why the fixed spacing above was tenable at all. But 0.0.9's own
`TODO` listed printing as a 1.0 goal, and printing needs breaking and
justification. So: **keep the unbroken strip as the editing view**, and add a
paginated view that shares the same spacing engine. Both are consumers of
`model.Score`; neither is the model's business.

---

## What this means for the Go `engrave` package

```
engrave.Spacings        the spacing table, in staff spaces, SMuFL-seeded
engrave.Engrave(score, spacings, mode) -> Layout
  mode = Strip          the 0.0.9 behaviour, one unbroken system
  mode = Paginated      breaks and justifies to a page
Layout                  glyphs, lines and beams with staff-space coordinates
```

`Engrave` is a pure function, so the whole of the above is testable against
`testdata/reference/boite-musique.png` — the output the C engine actually
produced in 2004. The picture was kept precisely because it outlives the code
that made it: it is still the record of what the old behaviour looked like.
