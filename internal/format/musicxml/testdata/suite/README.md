# The MusicXML test suite

182 files, each one written to exercise a single feature of MusicXML and
nothing else: one file for microtones, one for composite time signatures, one
for grace notes at the end of a bar. They are named for what they test, and
the numbers group them — 01–03 pitches, rests and rhythm; 11–14 staff
attributes; 21–24 notes, chords, tuplets and grace notes; 31–34 notations;
41–46 parts, repeats and barlines; 51–59 layout; 61 lyrics; 71–75
instruments; 81–89 sound; 90 compressed files; 95–99 the awkward ones.

## Where it came from

Written by Reinhold Kainhofer for LilyPond's `musicxml2ly`, forked with his
blessing by Michael Scott Asato Cuthbert, and donated in 2026 to the W3C Music
Notation Community Group, which now keeps it. The MusicXML repository uses it
to validate its own schemas.

- <https://github.com/w3c-cg/musicxml>
- MIT licensed; the licence is in this directory beside the files, and the
  copyright is Michael Scott Asato Cuthbert's.

The files are kept here rather than fetched because a test that needs the
network is a test that fails for reasons that have nothing to do with the
code, and because a suite that moves underneath you cannot catch a
regression.

## What gscore does with them

`suite_test.go` reads and engraves every one of them on each run. It asks
three things:

1. **Nothing is refused.** A file gscore does not entirely understand still
   has to come back with the music that was in it, and a note of what did
   not. Anything else loses somebody's work.
2. **Everything engraves.** Every file that comes in has to come out as a
   drawing, with no panic and no empty page.
3. **Every notehead arrives.** The file is counted and the engraving is
   counted, and they have to agree. This is the one that finds real bugs:
   it is how the grace notes turned out to be going missing, and the drums,
   and every note of a file whose time signature read "3+2".

Where the two counts do not agree yet, the file is listed in the test with
the number it currently draws. That list is the honest state of the importer,
it only ever shrinks, and the test fails if a file drops below it.
