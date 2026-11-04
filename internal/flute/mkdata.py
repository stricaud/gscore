#!/usr/bin/env python3
"""Turn a scrape of flute fingerings into the table gscore carries.

    python3 internal/flute/mkdata.py ~/git/hub/methode-pour-flute/fingerings/fingerings

The scrape is one file per note, each with the standard fingering and
every alternative the source lists — some twenty-nine thousand of them,
written as "Th 1 2 3 | 1 2 3 C" and a sentence about the intonation. That
is three and a half megabytes of text for what is really a handful of bits
per fingering: which of twenty-one keys are down, and how far out of tune
it comes out.

So each fingering becomes one integer and one number of cents, and the
whole table is small enough to carry in the program. The key order is the
one below and is part of the format: changing it changes every number.
"""
import json, os, re, sys

# The keys, left hand then right, in the order a chart is written. The
# order is the format: see KEYS in fingerings.go, which must match.
KEYS = [
    "Th", "BbTh", "1", "2", "3", "G#", "A#",          # left
    "1", "2", "3", "Bblever", "F#", "G", "C", "C#",   # right
    "B", "D#", "gizmo", "tr1", "tr2", "split E",      # right, continued
]
LEFT = 7


def bits(line):
    """The keys of one fingering as a number, with the bar telling hands."""
    line = line.replace(" ", " ")
    left, right = line.split("|", 1)
    # "(split E)" is two words for one key.
    right = right.replace("(split E)", "split\u0000E")
    out = 0
    for side, part, lo, hi in (("L", left, 0, LEFT), ("R", right, LEFT, len(KEYS))):
        for tok in part.split():
            tok = tok.replace("\u0000", " ")
            for i in range(lo, hi):
                if KEYS[i] == tok:
                    out |= 1 << i
                    break
            else:
                raise SystemExit("unknown %s key %r in %r" % (side, tok, line))
    return out


def cents(text):
    m = re.search(r"(plus|minus) (\d+) cents", text)
    if not m:
        raise SystemExit("no intonation in %r" % text)
    return int(m.group(2)) * (1 if m.group(1) == "plus" else -1)


def main(src):
    notes = {}
    for name in sorted(os.listdir(src)):
        if not name.endswith(".json"):
            continue
        with open(os.path.join(src, name)) as f:
            d = json.load(f)
        note = name[:-5]
        std = d["standard"]
        seen = {}
        rows = []
        for e in [std] + d.get("alternatives", []):
            b = bits(e["keys"])
            if b in seen:
                continue          # the scrape lists a few twice
            seen[b] = True
            rows.append([b, cents(e["intonation_text"])])
        notes[note] = {"std": rows[0], "alt": rows[1:]}
    out = {"keys": KEYS, "left": LEFT, "notes": notes}
    here = os.path.dirname(os.path.abspath(__file__))
    with open(os.path.join(here, "fingerings.json"), "w") as f:
        json.dump(out, f, separators=(",", ":"))
        f.write("\n")
    print("%d notes, %d fingerings" %
          (len(notes), sum(1 + len(v["alt"]) for v in notes.values())))


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else
         os.path.expanduser("~/git/hub/methode-pour-flute/fingerings/fingerings"))
