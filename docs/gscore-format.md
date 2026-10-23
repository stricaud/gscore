# The `.gscore` file format

A `.gscore` file is a ZIP container. Rename one to `.zip` and open it and
everything in it is readable text, which is the point: a file somebody cannot
open in ten years is a file that has lost their work.

```
META-INF/format.json     what this is and which version
score.json               the music
symbols/index.json       the hand-drawn symbols' metrics
symbols/*.xpm            the symbols themselves
```

## Why there is a format at all

MusicXML is the right way to exchange music with other programs, and gscore
reads and writes it. The container exists for the last two lines.

MusicXML cannot carry a symbol somebody drew. MuseScore can embed a picture,
but only as decoration floating on the page with no musical meaning, so a score
using an invented articulation arrives somewhere else as a smudge. Here the
artwork travels with the music and stays attached to the note it belongs to,
which means a score is still a score when it comes back — and means one
person's set of marks can be handed to another as a file.

## `META-INF/format.json`

```json
{
  "format": "gscore",
  "version": 1,
  "generator": "gscore 0.1.0"
}
```

`version` is the container's, and is raised only when an older reader could no
longer make sense of a newer file. A reader that finds a higher number than it
knows says so and stops rather than guessing. Fields a reader does not
recognise are ignored, so a file written by a later gscore still opens.

## `score.json`

The music. Everything is spelled out in words rather than numbers, so the file
reads sensibly and does not depend on the value of a Go constant:

```json
{
  "title": "Exercise 01 - C major",
  "composer": "",
  "tempo": { "bpm": 96, "unit": "1/4" },
  "parts": [
    {
      "name": "Flute",
      "instrument": "flute",
      "program": 73,
      "clef": "treble",
      "fifths": 0,
      "mode": "major",
      "beats": 4,
      "beatValue": 4,
      "measures": [ … ]
    }
  ]
}
```

### Parts

| Field | Meaning |
|---|---|
| `id`, `name` | What the staff is called |
| `instrument`, `program` | The name and the General MIDI number; the number wins |
| `clef` | `treble`, `bass`, `alto`, `tenor`, `percussion` |
| `fifths`, `mode` | The key signature: −7 to 7, and `major`, `minor`, `dorian` … |
| `beats`, `beatValue`, `timeSymbol` | The meter; `timeSymbol` is `common`, `cut` or absent |
| `transpose` | `{diatonic, chromatic, octaveChange}` — see below |
| `measures` | The bars |

### Transposing instruments

```json
"transpose": { "diatonic": -5, "chromatic": -9 }
```

The interval between what the player reads and what is heard, written
written-to-sounding exactly as MusicXML's `<transpose>` is, so a file that
passes through another program keeps its meaning. An alto saxophone reading a
written C sounds the E flat a major sixth below it, which is `-5` and `-9`.

Both numbers are needed and for the same reason an interval needs both: the
semitones say what is heard, the steps say how it is spelled.

**Everything in `measures` is written pitch** — what is actually printed on
that staff. That is what makes clicking a staff put a note where it was
clicked, whatever instrument the staff is for. Playback is where the
transposition is applied.

### Measures

```json
{
  "barline": "repeat-end",
  "fifths": -3, "mode": "minor",
  "beats": 6, "beatValue": 8,
  "ending": { "numbers": [1], "start": true, "stop": true },
  "voices": [ [ … ], [ … ] ]
}
```

`barline` is what *closes* the measure: a measure's opening barline is the
previous measure's closing one, so only one is stored.

`fifths`/`mode` and `beats`/`beatValue` here are a **change** taking effect at
this bar, which is how a piece modulates or changes meter in the middle. They
are absent when nothing changes — absent is not the same as a change to C
major, which is why the reader distinguishes them.

`ending` is a volta bracket: `numbers` are the times through that this bracket
is played, `start` and `stop` say where it opens and closes, and `open` leaves
off the closing hook, which is how a last ending is written.

`voices` is a list of lists: two independent lines sharing one staff are two
voices. Most music has one.

### Elements

Each element says what kind it is:

```json
{ "kind": "note", "dur": "1/8", "pitch": { "step": "F", "octave": 5, "alter": 1 },
  "accidental": "sharp", "marks": { … } }

{ "kind": "rest", "dur": "1/4" }

{ "kind": "chord", "dur": "1/2",
  "notes": [ { "step": "C", "octave": 4 }, { "step": "E", "octave": 4 } ] }

{ "kind": "symbol", "symbol": { "name": "my-mark", "offsetX": 0.5, "offsetY": -1 } }
```

`dur` is a fraction of a whole note, so an eighth note is `"1/8"` and a dotted
quarter is `"3/8"`. A dot is not stored: a duration already says how long the
note is, and the engraver works out that three eighths are written as a dotted
quarter.

`alter` is how the pitch *sounds*, in semitones. `accidental` is what is
*printed*. They are not the same question: in D major an F sharp has
`"alter": 1` and no accidental at all, because the key signature has already
said so.

### Marks

Everything written on or around a note:

```json
"marks": {
  "articulations": ["staccato", "accent"],
  "tieStart": true, "slurStop": true,
  "tuplet": { "actual": 3, "normal": 2, "start": true },
  "stem": "up",
  "dynamic": "mf",
  "hairpin": { "kind": "crescendo", "start": true },
  "octave":  { "amount": 1, "start": true },
  "lyrics": [ { "text": "Glo", "verse": 1, "syllabic": "begin" } ],
  "symbols": [ { "name": "my-mark", "offsetX": 0.4, "offsetY": -0.8 } ]
}
```

A hairpin and an octave line each reach from one note to another, so they are
written on both: `start` on the first, `stop` on the last. `amount` is in
octaves and signed — `1` is 8va, `-1` is 8vb, `2` is 15ma.

`syllabic` is `begin`, `middle`, `end`, or absent for a whole word, which is
what decides whether a hyphen is drawn after it. `extend` draws the line that
carries a held syllable across the notes it is sung over.

`offsetX` and `offsetY` are in **staff spaces**, and are the one place where
the model holds anything about position. That is deliberate: where a symbol
goes is a musical decision the person who drew it made, not something the
engraver can work out. Everything else about the layout is computed, which is
why the same file engraves properly at any size and on any paper.

## The symbols

`symbols/index.json` is a list:

```json
[
  { "name": "my-mark", "file": "symbols/my-mark.xpm",
    "pixelSize": 0.125, "originX": 8, "originY": 8, "advance": 1.2 }
]
```

`pixelSize` is how big one pixel is in staff spaces, so a drawing keeps its
proportions at any zoom and on any paper. `originX` and `originY` are the point
in the grid that lands where the symbol is placed — usually its middle.

The drawings themselves are XPM: a pixel grid written as text, one character
per pixel with a small colour table at the top. It is the format the original
gscore drew all 287 of its symbols in, it is readable in any text editor, and
it diffs — which matters for a file people are meant to send each other and
keep in version control.

Only the symbols a score actually uses are written, so a file does not carry a
whole library around for the sake of one mark.

## Symbol packs

A `.gscore` holding symbols and no music is a symbol library. Somebody draws
the marks their music needs and nobody else's font has, saves them, and anyone
can load them into their own scores.

The container is the same one scores use, so a pack opens as an empty score and
a score opens as a pack: nothing new to learn, and `Insert ▸ Symbol ▸ Import
from a .gscore` will take the marks out of *any* `.gscore`, whether it was
saved as a pack or is somebody's piece that happens to use them.

## Reading one without gscore

```python
import json, zipfile

with zipfile.ZipFile("piece.gscore") as z:
    score = json.loads(z.read("score.json"))
    for part in score["parts"]:
        print(part["name"], len(part["measures"]), "bars")
```

That is the whole of it. No schema to fetch, no namespaces, no binary.
