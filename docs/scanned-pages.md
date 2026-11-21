# Reading scanned and photographed pages

A picture of a page — a scan, a phone photograph, a JPEG on a website — holds ink and no notes. Gscore opens it anyway, by running homr, and this is how to set that up, use it from the window, the command line and your own code, and ship it.

## What happens

Turning ink into notes is optical music recognition, which is a trained
model, a Python runtime and a minute of a processor's time per page. None of
that is inside Gscore. Gscore runs [homr](https://github.com/liebharc/homr),
which does it well, as a separate program: it hands over the pictures, homr
writes MusicXML beside them, and Gscore opens that with the MusicXML
importer it already has. Several pages go to homr in one call and come back
as one score, in order, with the meter carried from each page to the next.

What you get is a skeleton to correct rather than a finished engraving. homr
reads notes, rests, voices, grand staves, keys and meters well; it leaves
beams to Gscore, which draws them itself; and it does not promise lyrics,
chord symbols, octave lines or glissandi. A photographed page opens with the
right bars in the right places and some wrong notes, which is a better place
to start from than an empty score.

## Opening a picture

<span class="menu-path">File → Import</span> gathers everything that is not
a score file yet, and all of it goes through the page reader:

- <span class="menu-path">Import → PDF…</span> takes one PDF, scanned or
  engraved. A scan goes to the page reader. A PDF drawn by a notation
  program, with glyphs to read, raises a question when there is a page
  reader, because the two readings want opposite answers: *from its
  glyphs* keeps every staff and part with the notes exact and reads no
  slurs, dynamics or words; *as pictures* reads those but may fold the
  staves of several instruments into a piano part and misread a note. The
  window says how many staves each system has and suggests the glyphs for
  more than two — an eighteen-stave *Non, je ne regrette rien* came back as
  a piano part the other way. Without a page reader the glyphs are read as
  they always were, and a scan is refused with a word on what to install.
- <span class="menu-path">Import → Pictures of pages…</span> takes `.png`,
  `.jpg` and `.jpeg`, several at once. With more than one, a window lists
  them in the order they will be read — by the numbers in their names to
  start with, so `page2` comes before `page10` — with arrows to move any
  of them.
- <span class="menu-path">Import → From the web…</span> takes their
  addresses, one to a line, pictures or a PDF. The usual keys paste into
  the box:

  ```
  https://www.ninasimone.com/wp-content/uploads/2021/04/iputaspellonyousheetmusic1.jpg
  https://www.ninasimone.com/wp-content/uploads/2021/04/iputaspellonyousheetmusic2.jpg
  https://www.ninasimone.com/wp-content/uploads/2021/04/iputaspellonyousheetmusic3.jpg
  https://www.ninasimone.com/wp-content/uploads/2021/04/iputaspellonyousheetmusic4.jpg
  ```

  The machine Gscore runs on fetches them, which is why the entry is only
  there when the window is on that machine — a server reached from
  elsewhere does not go and get things for whoever asks, any more than it
  opens paths.

<span class="menu-path">File → Open score…</span> still takes a picture or
a PDF like any other file, and several pictures dropped on the window are
put in order the same way.

While homr works the mascot is up with *Importing…* and a percentage, and
the status line says which page of how many and which staff of how many
it is on, read from what homr itself reports. A page takes a few seconds on
an Apple Silicon Mac, where homr uses CoreML without being told, and about
a minute on a plain processor.

## Where homr comes from

Gscore looks for the program in this order and uses the first it finds.
<span class="menu-path">File → Settings…</span>, under *Reading scanned
pages*, shows which one that was.

1. **`GSCORE_HOMR`**, an environment variable holding a whole command line.
   For developers and for anybody with a homr of their own:

   ```sh
   GSCORE_HOMR="uvx --from 'homr[cuda]' homr" gscore
   GSCORE_HOMR="poetry run homr" gscore
   GSCORE_HOMR=/opt/homr/bin/homr gscore
   ```

   Quotes group words as a shell's do, so `'homr[cuda]'` is one word.
2. **The command in Settings**, when *A command of my own* is chosen there.
   The same thing without the variable.
3. **A copy beside the binary**, in a folder named `homr`, or `homr-arm64`
   and the like for one architecture of a universal binary — next to the
   executable, or in a macOS bundle's `Resources`. The `-omr` downloads
   carry one; see below.
4. **uv**, where it is installed: Gscore runs
   `uvx --from 'homr[cpu]' homr`, and uv fetches homr the first time and
   keeps it. Settings chooses between *The processor* and *An NVIDIA
   graphics card*, which only changes `cpu` to `cuda` here — which build
   of the ONNX runtime uv fetches. homr picks the device itself once it
   runs. An AMD card goes through a command of your own, with the line
   homr's README gives for ROCm.
5. **`homr` on the PATH.**

With none of these, opening a picture says so and names the first two
things to try.

### As a developer

With [uv](https://docs.astral.sh/uv/) installed there is nothing to set up:
`go run .` finds `uvx`. The first page waits for uv to build the environment
and for homr to fetch its models, about 200 MB, so do that once beforehand
and watch it happen:

```sh
uvx --from 'homr[cpu]' homr --init --no-title
```

`--init` fetches what is missing and stops; `--no-title` skips a second
model that reads the title off the page, which Gscore never asks for.

The Go tests never run the real homr. `internal/format/scanned` and
`internal/app/scanengine_test.go` use a shell script that stands in for it,
writing a known page of MusicXML beside the first picture it is given, so
the suite needs nothing installed and runs in milliseconds. To try the real
thing, open a picture.

## From the command line

With Gscore serving — `gscore -headless`, or `gscore -serve 127.0.0.1:8099`
as the examples assume — pictures go to `/api/openscan`, as the parts of a
multipart request named `page`, as `url=` addresses, or as `path=` files on
the machine the server runs on:

```sh
G=http://127.0.0.1:8099
curl -X POST "$G/api/openscan" -F page=@page1.jpg -F page=@page2.jpg
curl -X POST "$G/api/openscan?url=https://…/sheet1.jpg&url=https://…/sheet2.jpg"
curl -X POST "$G/api/openscan?path=$PWD/page1.jpg&path=$PWD/page2.jpg"
curl "$G/api/pdf" -o score.pdf
```

The reply is opened at once and kept alive with spaces while homr works —
the native window gives up on a reply that has gone quiet for a minute —
and is then the answer: `{"status":"ok","pages":2,"filename":"page1.gscore",…}`,
or `{"error":"…"}` when the pages could not be read. A request with no
reader to run, or with files that are not pictures, is refused outright
with a 400. Fetching by address and reading by path are refused with a 403
unless the server is bound to this machine. `GET /api/settings` reports
`scanEngine`, the command Gscore would run and where it found it, or its
`error`.

A single picture or a PDF also opens through the ordinary `/api/open` and
`/api/openpath`, kept alive the same way. For a PDF, `reader=glyphs` or
`reader=pages` says which way, and `ask=1` has a PDF that could go either
way answered with `status: choose` and the `staves` to a system instead of
being opened; the answer's `engraved` says whether a PDF was read glyph by
glyph, with a reading to review, or as pictures. `GET /api/scanprogress`
says how far the reader has got while it works. `/api/pickfile`, the system's open dialog, hands several pictures
back as `pages` to be put in order rather than opening them, and does so
for any number with `list=1`.

## Shipping it: the `-omr` packages

The ordinary downloads leave homr out. Beside them, each release carries a
second set, named `-omr`, with homr inside so that there is nothing else to
install: a private Python, homr and its ONNX runtime, the models already
fetched, and a launcher where Gscore looks. They are about half a gigabyte
larger. The macOS reader is built for Apple Silicon; the binary is still
universal, and an Intel Mac falls back to uv or `GSCORE_HOMR`.

`packaging/homr/build.sh` builds that folder, on the platform it is for,
needing only uv:

```sh
packaging/homr/build.sh build/homr-sidecar        # cpu
packaging/homr/build.sh build/homr-sidecar cuda   # an NVIDIA card
```

It installs a relocatable Python 3.12 through uv, installs `homr[cpu]` into
it at the version pinned in the script, runs `--init` twice — once for the
processor and once for whatever this kind of machine would pick on its own,
so that nothing is fetched later on a machine without network — writes
`bin/homr` and `homr.cmd`, and puts homr's licence and source archive in
the folder. The result can be moved anywhere. `bin/homr --init --no-title`
printing `Init finished` is the check that it runs from where it is.

The Makefile's `OMR_SIDECAR` puts the folder in a package and names it
`-omr`:

```sh
make dmg OMR_SIDECAR=build/homr-sidecar      # Resources/homr-arm64 in the bundle
make linux OMR_SIDECAR=build/homr-sidecar    # homr/ beside the binary
make windows OMR_SIDECAR=build/homr-sidecar  # homr\ in the zip, a section in the installer
```

The *OMR packages* workflow, `.github/workflows/omr.yml`, does all three on
every tag and on request, the Windows reader on a Windows runner since its
packages are compiled, and adds the packages, their checksums and a note to
the release the *Release* workflow publishes. The *Release* workflow itself
is unchanged: the packages without homr are built as they always were.

## The licence

homr is free software under the AGPL-3.0, by Christian Liebhardt and its
contributors. Gscore runs it unmodified as a separate process, which its
authors' licence FAQ names as a clear separation, and Gscore stays MIT. Each
`-omr` package carries homr's licence as `LICENSE-homr.txt` and its source
as `homr-<version>.tar.gz` in the reader's folder, with a `README.txt`
saying where both came from. The models are under the same licence; the
data they were trained on is listed in homr's `LICENSE_FAQ.md`.
