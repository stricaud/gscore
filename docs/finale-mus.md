# Reading Finale's older files

Finale stopped being sold in 2024. The files did not go anywhere: people have
decades of them, and for most of that time Finale wrote `.mus`, not the `.musx`
it moved to in 2014. So the advice gscore used to give somebody holding a
`.mus` — open it in Finale and save it as something else — stopped being advice
anybody could take. This is why gscore reads it instead.

It reads both of them, because `.mus` is two formats. The extension never
changed; what Finale put inside it did.

| | files from | the music is behind | records are |
|---|---|---|---|
| the older | 2001, 2004, 2005 | four **PKWARE DCL** streams, each labelled | fixed 16-byte rows with two-character tags |
| the newer | 2011, 2012 | a chain of **zlib** streams, unlabelled | variable-length, each carrying its own tag and key |

What they share is the header, the offset the payload starts at, the way the
notes themselves are stored — and the document model, which is the same one
the `.musx` carries as XML. That last point is the whole reason this is a few
hundred lines rather than a second program: `build` in
[`internal/format/enigma/enigma.go`](https://github.com/stricaud/gscore/tree/main/internal/format/enigma/enigma.go)
turns one Enigma document into a score, and it neither knows nor cares which
of the three containers that document arrived in.

Everything else is kept apart. There are two importers, two error types and
two reports, because a message about one format must never be a message about
the other.

## What is read, and what is not

Read: the staves in score order, their names, their transpositions, the
brackets and braces that join them, the bars with their meters, keys, clefs,
barlines, repeat signs and rehearsal marks, and in each bar the notes, chords
and rests with their pitches, lengths and ties.

Not read: most of what Finale writes around the music — articulations, slurs,
hairpins, dynamics, tempo words, lyrics, tuplets, numbered endings, chord
symbols. Each one is counted and named in the report the editor shows when the
file opens, so a score that arrives without its slurs says so rather than
looking finished.

This is a larger loss than a `.musx` takes, where most of that *is* read. The
difference is not the container, it is that nobody has worked out what the
individual records of the older pools mean. The walk reaches every record and
hands over its tag, its key and its bytes; what those bytes say is per-record
work that has not been done.

## Eight things worth knowing

**A `.mus` has no categories, so nothing in it says what kind of mark a
marking is.** Finale did not group its expressions until long after these
files were written: before that an expression was a line in one flat list,
and all a definition carries is what the mark does when the score is played.
That identifies the dynamics and the tempo marks, which set the loudness and
the speed, and nothing else.

What is left has to be told apart by how it is drawn, and one kind can be:
a marking written against a bar and drawn inside a box is a rehearsal mark.
That is what the box is for — it is the one marking on a page that has to be
found from across a hall. A boxed direction would be read as one too, which
is the cost of having no categories and is the right way round: the mark
lands on the bar it was written on either way.

A mark Finale numbers for itself carries no words, only the library's
description of it — "Boxed Rehearsal Number". What it prints is the number of
the bar it stands on, which is a thing a reader can work out. A mark Finale
*letters* for itself is refused, because a letter depends on how many marks
came before it, and a rehearsal mark guessed wrong is worse than one missing:
it is an address.

**Most `.mus` files hold no music, and that is not a failure.** A template is
staves and bars and nothing else, and so is the file an exercise starts from.
Of the 102 in the Berklee Press collection, 71 are blank — among them a
36-stave orchestral score with every instrument named and 55 bars ruled,
waiting to be filled in. gscore refused all 71 until it was first run against
real files, on the reasoning that a document placing no music on any staff
could not be a score. It is a score; it is an empty one, which is the whole of
what a template is for. What tells a blank file from an unreadable one is the
file itself: a frame hold is Finale's own statement of which music sits on a
staff in a bar, and a hold naming no frame is the file saying that bar is
empty.

**A key signature is two fields in one word.** The low byte counts the
accidentals, sharps up and flats down as a signed byte; the high byte is the
mode. Read whole, the word is right for sharps by accident and wrong for every
flat — two flats is `0xFE`, and 254 is not a signature anybody has. A Bach
fugue in G minor engraved with no flats at all is how that was found. The mode
matters because the same two flats are G minor or B flat major depending on
it: the staff looks identical, and the difference shows up in how a chord over
that bar is named.

**The byte order is read off the file.** A Mac wrote its numbers big end first
and a PC little, and nothing in the file says which. What says it is the first
pool record's kind, which is always the same number — and that number is only
itself one way round. So the test that finds the byte order is the same test
that there is a container there at all, which is why gscore gives up rather
than picking one.

**The entry pool is a live database, not a list of the music.** A `.mus` is
written in place, so an entry deleted in Finale keeps its 38-byte slot until
the file is next compacted. The pool therefore holds both the music as it
stands and whatever earlier music has not been written over yet, and the frames
are the only thing that says which is which. Read as a list, a score comes back
with bars of deleted music in it.

**A staff holds the number of a record that names its words, not the number
of the words.** The two are different — in a saxophone quartet the staves ask
for 50 to 57 and the words are in blocks 18 to 25 — and reading the first as
the second looks for words that are not there. That is a nameless staff,
which is a staff played as a piano, and nothing is reported, because a staff
with no name is not an error. Four saxophones came through as four pianos
that way, and the only sign of it was on the page.

**A transposition is stored once and has to be said twice.** What the file
holds is how many sharps the staff's key gains; what a written note needs is
also how many steps up the staff it sits. The second is not missing, it is
the same answer in other words — three sharps is three fifths up, which is a
sixth, which is five steps and nine semitones — so the steps are worked out
from the sharps and the two halves agree by construction.

Leaving the steps at nothing while reading the semitones is the failure worth
naming, because of how it looks: every note written where it sounds and then
given nine semitones of accidental to make up the difference. The sound is
right and the page is a wall of double sharps, two octaves from where a
saxophone plays. A reader that gets half of a transposition is worse than one
that gets none.

**What the file really does not hold is the octave.** Finale folds the
interval into a residue within an octave and keeps the octaves in its
instrument definition, which is not in the file, so a tenor saxophone and a
soprano store identical bytes. That much is absent rather than hidden — but
the staff's name is not, and a tenor saxophone is a tenor saxophone wherever
it is written down. So the name supplies the octave where gscore knows the
instrument and the interval within the octave agrees; where it does not, the
staff stays right to within an octave and no better. The name is trusted to
settle what the file leaves open and never to overrule what it answers.

## Where this knowledge comes from

Finale's publisher released one preliminary document, in February 1996, on one
pool of one version — the entry pool of Finale 3.5 — and nothing else, ever. It
is a good document and it settles the part of the format that matters most: the
note records, the flag bits, and that a pitch is counted in scale steps from
the tonic of the key rather than from middle C. gscore's `.musx` reader had
that last point wrong until a piece in G major went through it.

It is also wrong about one thing, which real files settle: it describes a
note's alteration as a signed quantity, meaning two's complement, and it is
stored as a sign and a size. The two readings agree up to a sharp and part
company at a flat.

Everything else gscore knows about the format was worked out from files by
[finale-file-parser](https://github.com/jsawruk/finale-file-parser), whose
notes record both what was found and what was refuted along the way — which is
the more useful half. No code was taken from it; what was taken is facts about
a file format, and those are nobody's property. The compression is a port of
Mark Adler's `blast.c`, under the zlib licence. Both are credited in
[`LICENSE`](https://github.com/stricaud/gscore/tree/main/LICENSE).

## What this cannot promise

**There is no `.mus` in this repository to test against, because a `.mus` is
somebody's music.** The tests write their own files — both containers, both
byte orders — and read them back, which checks the framing, the slot walk and
the dead-slot pruning. What they cannot check is a field this reader puts at
the wrong offset, because the test puts it at the same wrong offset. The
compression is the one part pinned against something outside gscore: the test
vector `blast.c` documents.

That limit was not theoretical. This reader was written without a single real
file, and the first 102 it met — the companion files to a Berklee Press book
on Finale, all written in 2004 and 2005 on a Mac, and freely downloadable —
found two faults of exactly the predicted kind. 71 of the 102 were refused for
being blank. Of the 31 that opened, one engraved a Bach fugue in G minor with
no key signature, because the key word was read whole instead of as its two
halves. Everything else held: the framing, the byte order, the entry walk, the
pitches, the ties, the beams, the clefs, the bar counts and the titles were
right the first time. All 102 now open.

So where a field's position is taken on trust, the code says so where the
offset is written down. A real file that disagrees has somewhere obvious to
point, and if you have one that opens wrongly, that is still the bug report
worth sending.

**A repeat is a flag on a measure, and it is read from both sides.** The bar
that closes a repeated section carries the backward sign; the bar that opens
the next one carries the forward sign, against its own left-hand edge. gscore
stores one style per bar, on the bar the sign stands after, so a forward sign
is read off the bar after the one it is stored on — left where it was, a
repeat opening bar five would be drawn closing it. A section that ends where
another begins comes out as the one mark it is printed as.

The numbered endings beside them are not read. The bit says one begins here
and not which times through it is played nor where it stops, and a first-time
bar played every time is a piece of the wrong length.

**One offset has already disagreed and is not used.** A group record is meant
to carry, at +21, a flag saying whether the barlines run down through the gaps
between its staves. On the saxophone quartet this reader was fixed against,
that byte is zero and the printed page runs them through plainly. So the
offset is not the flag, or not only the flag, and gscore reads the shape
instead: a bracket or a brace is what joins players into one thing and its
barlines run through, a plain line stops short of saying so. That is Finale's
default for a new group, and the case it does not cover is a group whose
barlines have been turned off by hand.

**What is still wrong in those files.** A staff's own default clef is not
read, so a bar with no frame hold falls back to treble — which is invisible on
a full score and shows up on a blank template, where a 36-stave orchestral
skeleton comes out with its trombones and timpani in treble. Layers past the
first are counted and not read. And a piano arrives as two staves rather than
one braced instrument.
