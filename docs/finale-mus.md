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
[`internal/format/musx/enigma.go`](https://github.com/stricaud/gscore/tree/main/internal/format/musx/enigma.go)
turns one Enigma document into a score, and it neither knows nor cares which
of the three containers that document arrived in.

Everything else is kept apart. There are two importers, two error types and
two reports, because a message about one format must never be a message about
the other.

## What is read, and what is not

Read: the staves in score order, their names, their transpositions, the bars
with their meters, keys, clefs and barlines, and in each bar the notes, chords
and rests with their pitches, lengths and ties.

Not read: everything Finale writes around the music — articulations, slurs,
hairpins, dynamics, tempo words, lyrics, tuplets, repeats, chord symbols. Each
one is counted and named in the report the editor shows when the file opens, so
a score that arrives without its slurs says so rather than looking finished.

This is a larger loss than a `.musx` takes, where most of that *is* read. The
difference is not the container, it is that nobody has worked out what the
individual records of the older pools mean. The walk reaches every record and
hands over its tag, its key and its bytes; what those bytes say is per-record
work that has not been done.

## Three things worth knowing

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

**A transposing staff's octave is not in the file.** Finale folds a
transposition into a residue within an octave and keeps the octaves in its
instrument definition, so two staves a unison and an octave apart store
identical bytes. The octave is not hidden, it is absent, and no reader can
recover it; Finale pinned it down when it wrote a `.musx`. A transposing staff
from a `.mus` is therefore right to within an octave and no better.

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
byte orders — and read them back, which checks the framing, the slot walk, the
dead-slot pruning and every refusal. What it cannot check is a field this
reader puts at the wrong offset, because the test puts it at the same wrong
offset. The compression is the one part pinned against something outside
gscore: the test vector `blast.c` documents.

So where a field's position is taken on trust, the code says so where the
offset is written down. A real file that disagrees has somewhere obvious to
point, and if you have one that opens wrongly, that is the bug report worth
sending.
