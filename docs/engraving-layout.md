# How gscore lays a page out

Read this before changing anything in `internal/engrave`.

`docs/engraving.md` is the other half of the story: what the 2001 C engine
decided and why, kept or discarded on purpose. This file is the rules the Go
engraver follows *now* — the ones that were arrived at by getting them wrong
first, which is why they are written down rather than left to be rediscovered.

Everything here is in **staff spaces**, never pixels, and **y grows
downwards**: above the staff is a smaller number. The layout is a pure
function of the model, which is what makes all of it testable without drawing
anything.

---

## 1. Barlines come from the column, not from the music

A measure's closing barline is placed from the column the next bar starts in,
less the air that belongs after the line — never from the point that staff's
own notes happened to reach.

Every staff of a system shares the columns but fills them at its own rate: a
bar of four eighths and a bar of one half note are stretched differently to
end in the same place. Each staff drawing its own line where its notes ran out
left a system with a ragged edge instead of a column, by over a staff space in
a band score and worst at the end of a piece.

It is measured with the **system's** stretch (`e.stretch`), not the staff's
own. Scaling the gap after the barline by the staff's rate is the same mistake
one step further on — it was my first attempt and it made the spread worse.

## 2. Repeats and the final barline belong to the piece

Written on any staff at a bar, they are given to every staff at that bar, in
`Score.Normalise`. Everybody goes back at the same bar and everybody stops at
the same one; a sign down one staff of a system and not the rest is not a
repeat, it is a mistake, and a player reading any other part never sees it.

Files do not say this — MuseScore writes the repeat once, on the first staff —
so it is a rule about music and lives in the model, not in each reader.

**A double barline is not included.** A section can end in the flutes while
the horns are still in one. This is not a rule that every barline down a
column has to match.

## 3. Everything outside the staff is placed against a skyline

See `internal/engrave/skyline.go`.

A skyline is the outline of what is already drawn on a staff: for every point
along it, how far the music reaches above and how far below. A mark is placed
against that outline and then **becomes part of it**, so the next one clears
it too.

Marks used to be placed at a fixed distance from the staff. That is right
until two of them want the same place, and then they are drawn through each
other — a hairpin through a slur and through the dynamic it ran between, a
boxed rehearsal letter around a note on ledger lines, an 8va sign on top of a
segno.

**The outline is read off the layout**, from a snapshot taken before the
staff's music (`e.marked()`, `e.outline(...)`). Stems, beams, ledger lines and
slurs are lines and curves whose extents the engraver already computed, so
those go in exactly. Only glyph heights are estimated, in `glyphReach` — the
font carries advance widths but not bounding boxes, and reading those out of
CFF outlines is a lot of machinery for a number that only has to be about
right. Generous is the safe direction.

**Marks that cannot be placed where they are met wait.** A hairpin is not even
drawn until the bar it closes in, which can be four bars later. They are
queued with `e.wait(waitingMark{...})` as the staff is drawn and placed by
`e.placeWaiting` once it is finished.

### The order, outwards from the staff

These are LilyPond's `outside-staff-priority` numbers, from
`scm/define-grobs.scm`. They are kept as LilyPond's values rather than
renumbered 1, 2, 3, so the order can be checked against the program that
settled it and there is room to put something between two of them.

| rank | what | constant |
|---|---|---|
| 50 | trill and its wavy line | `rankTrill` |
| 100 | bar number | `rankBarNumber` |
| 250 | dynamics **and** hairpins | `rankDynamic`, `rankHairpin` |
| 400 | octave lines (8va) | `rankOctave` |
| 450 | words on a note ("rit.") | `rankText` |
| 600 | volta brackets | `rankVolta` |
| 1500 | rehearsal letters, signs, tempo, jumps | `rankMarks` |

This is real engraving knowledge, not an arbitrary choice. The bar number sits
closest of anything written above; the rehearsal letter is outermost because
it is what somebody calls across a room. My own invented order had the bar
number outside the volta brackets, which left it floating at the top of the
gap.

Dynamics and hairpins **share** a rank because in a printed score they sit on
one line — `f ——— ff` lines up. gscore gives them the same rank so they do not
stack; it does not yet align them onto a common line.

### Everything in a row sits on the row's near edge

`skyline.place` returns the edge nearest the staff. Everything drawn in that
band is measured from it, and nothing may hang past it.

The marks row is as tall as its **tallest** member, which is the segno at
`signSize`, not the boxed letter. Sizing it to the letter left the segno
hanging out of the bottom of its band and back through the 8va — after the row
itself had been placed correctly.

## 4. The room above a system is reserved by stacking, not by maxing

`systemMarkRoom` adds the rows up. It used to take the largest of what each
needed, which is one allowance shared among four things; once the skyline
started stacking them properly they stacked straight into the gap above, which
is where the system before it is.

It is **counted from the model, not measured**, because a system's height is
decided before anything in it is drawn. `musicAbove` gives how far the top
staff's notes reach above it; the rows are added on top. Over-reserving a
little is safe, under-reserving puts marks in the system above.

## 5. Spanners

Things that reach from one place to another — slurs, ties, hairpins, octave
lines, trills, voltas.

- **Both ends carry the mark.** MuseScore writes `<next>` at the opening and
  `<prev>` at the close, which saves the reader working out where the other
  end landed.
- **Close before opening.** A note in the middle of a run carries both: it
  ends the one before it and begins the one after. Opening first finds a
  spanner already open, does nothing, and then the close takes that one away —
  four whole notes joined by three slurs came out with one.
- **Cut at the margin and picked up again** across a system break
  (`breakSpans` / `resumeSpans`). The stub runs to the **end of the staff**,
  which is `right`; `staffLines` next to it takes a *width*, and passing this
  the width stopped every hairpin, octave line and tie four spaces short.
- **A resumed curve comes back at the height it left, on the new staff.**
  `openSpan.rel` holds the height relative to its staff top. Keeping the
  absolute y drew the continuation from the old staff to the new one: one
  near-vertical stroke the height of the page.
- **`resumeRoom`** is held back after the meter at the head of a system that
  carries a curve in, or the continuation has nowhere to start but the column
  the note is already in.
- **A curve is only an arc while it is wider than it is tall** —
  `archPerWidth`. A stub a notehead wide drawn at full arch is a caret over
  the note, which on a score is an accent and means something else.
- **A slur clears the music it arcs over** (`clearArch`). Its ends belong to
  its notes and stay there; its middle is lifted until the curve is clear.
  An arch of fixed height is only clear of a phrase that happens to be flat.

## 6. A rest carries marks like anything else

`placed.marks()` answers for notes, chords **and rests**. Saying a rest had
none lost every mark on one: a crescendo closing on a rest never closed and
was redrawn on every system to the end of the piece, and a fermata over a bar
of silence simply vanished. A rest is positioned at the middle line
(`middleStep`) so that what is placed against it lands sensibly.

## 7. Noteheads a second apart move across the stem, alternately

See `displacedHeads` in `internal/engrave/draw.go`.

Two heads a second apart cannot share a column: drawn there they are one
through the other. One of each pair goes to the other side of the stem, and
which one is not a free choice — the chord is read **from the stem outwards**,
the note nearest it stays in the column the stem is drawn against, and from
there they alternate.

Alternate is the part that is easy to miss. The rule used to be "move me if
anything is a second from me", which is right for a pair and wrong for
anything longer: in a cluster of three it moved the middle note and the top
one both, and left those two in the same column a second apart — the one
thing the rule exists to prevent. The third note of a cluster has to come
back to the first column.

A chord with no seconds in it is one column. Nothing moves that does not have
to, or a plain triad is printed looking like a cluster.

---

## What is still wrong

- **Cross-staff collisions.** Each staff's outline covers only its own pass,
  and staves are drawn top to bottom, so a hairpin hanging below one staff
  cannot see a slur arcing above the staff underneath it. They meet in the
  gap. Fixing it means deferring every staff's outside marks to the end of
  the system and building each staff's outline from the whole system's music
  clipped to a band around that staff.
- **Dynamics and hairpins are not aligned** onto a common line (see above).
- **A displaced head always goes to the right.** With the stem up that is
  the convention; with the stem down the chord should extend to the *left* of
  the stem instead, and gscore puts it on the same side either way. Doing it
  properly also means the column's width has to know about it — the spacing
  does not currently reserve the extra notehead, in either direction.
- **Slurs get one shot.** LilyPond enumerates candidate curves and scores them
  on `encompass`, `extra_encompass`, `edges`, `slopes` and variance, then
  takes the cheapest — `lily/slur-scoring.cc` and `lily/slur-configuration.cc`.
  `clearArch` approximates the `encompass` term alone.

## How this is tested

As properties over the layout, not as pictures:

- `barcolumn_test.go` — every bar's barlines share one x, exactly.
- `skyline_test.go` — the stacking arithmetic directly; nothing above or below
  the staff drawn through anything else, on a bar crowded with a letter, a
  segno, an 8va, two dynamics, a hairpin, a slur and notes on ledger lines; a
  slur clears a phrase that peaks between its ends.
- `restmarks_test.go` — a hairpin closes on a rest and stays on its system.
- `spanbreak_test.go` — a curve broken over a system stays on its own staff,
  is an arc and not a spike, and runs to the end of the line.
- `chordheads_test.go` — no two heads of a chord a second apart share a
  column, from a pair up to a cluster of four; a chord with no seconds in it
  is left in one column; the note nearest the stem is the one that stays.

**Write the failing case first and check it fails.** Twice here a test passed
against the broken code because the fixture did not actually exercise the bug:
a slur over a rising run already clears the notes between its ends, and so
does one under a phrase that peaks. The case that bites is a peak with the
slur *above* it.
