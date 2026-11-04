# Putting gscore in a page

gscore runs in a browser with nothing behind it: the engraver, the readers for
every format it knows and the synthesiser are all compiled to WebAssembly and
served as static files. That means a page about a piece of music can show the
music and play it, and the only thing it needs is somewhere to put an
`<iframe>`.

There is a working page in [`samples/embed.html`](../samples/embed.html):

```sh
make web
python3 -m http.server --directory . 8000
open http://localhost:8000/samples/embed.html
```

It has to be served rather than opened as a file, because WebAssembly will not
start from a `file://` URL. Pointing the frames at a published copy such as
`https://stricaud.github.io/gscore/app/` needs no build at all.

One thing to watch: `score=` is resolved against the **gscore page**, not
against your page. The sample reaches back out with `../../samples/…` for
exactly that reason; an absolute URL avoids the question.

## The short version

```html
<iframe src="https://stricaud.github.io/gscore/app/?player=1&score=tune.abc"
        width="100%" height="220" style="border:0"></iframe>
```

`tune.abc` is fetched by the page and read in the browser. Nothing is uploaded
anywhere, and the score never leaves the reader's machine.

## Player mode

gscore is an editor, and a page about music does not want an editor on it.
`player=1` takes away the menu bar, the palette, the inspector and the status
bar, and leaves the score and a play button. `player=bare` takes away the play
button too: the score itself is then the button, and clicking it starts and
stops the music.

The difference matters more than it sounds. A disabled menu bar is still a
menu bar; somebody who came to listen reads it as "this is a program" and
starts looking for what they are supposed to do. Player mode is a page with
music on it.

## The settings

They go in the address, after the `?`:

| Setting | What it does |
|---|---|
| `player=1` | No menus, no palette, no inspector: the score and a play button |
| `player=bare` | Not even that — the score, which plays when it is clicked |
| `score=URL` | The file to open, relative to the gscore page or absolute |
| `autoplay=1` | Start playing as soon as the browser allows it |
| `loop=1` | Start again at the end |
| `tune=N` | Which tune of a book to show, counted from 0 |
| `zoom=N` | Staff space in pixels; 8 is the default, 12 is large |
| `mode=page` | The music in lines, wrapped to the width of the frame. The default |
| `mode=strip` | The music as one unbroken line, scrolling sideways |

## Lines or one line

An embedded score is wrapped to the frame by default: the music is broken
into systems the width of the frame and the frame scrolls down, which is
how a page of music is read.

`mode=strip` draws it as one unbroken line instead, scrolling sideways. It
is the right answer more often than it sounds:

- **A band across a page.** A tune in a strip two staves tall sits in the
  flow of the writing. The same tune wrapped needs a tall frame or it
  shows two systems and the top of a third.
- **One bar quoted in a sentence.** Nothing to wrap, and a wrapped frame
  leaves white space under it where the next system would have gone.
- **Something to play along with.** The music runs past as it is played,
  the way a scrolling score does, instead of jumping from line to line.

Wrapping is better for anything somebody is going to read as music —
an exercise, a study, a piece. A reader following a line of notes does not
want to be scrolling sideways to do it.

Either way the frame opens as tall as the music needs, so a strip comes up
exactly as tall as its one line. `data-height` or a height on the
`<iframe>` overrides that and the frame scrolls.

A browser will not make a sound until somebody has clicked on the page, so
`autoplay=1` is a request rather than an instruction: if it is refused, the
first click anywhere on the page plays the music instead.

## Driving it from your own page

An `<iframe>` is a separate document, so a page cannot simply call into it. It
sends a message instead. Every message gscore answers carries a `gscore` key,
so nothing else on the page can be mistaken for one:

```js
const frame = document.getElementById("score");

frame.contentWindow.postMessage({ gscore: "play" }, "*");
frame.contentWindow.postMessage({ gscore: "stop" }, "*");
frame.contentWindow.postMessage({ gscore: "toggle" }, "*");
frame.contentWindow.postMessage({ gscore: "tune", value: 3 }, "*");
frame.contentWindow.postMessage({ gscore: "load", value: "other.abc" }, "*");
// A score written into your page rather than kept in a file. The value is
// the text, or the bytes for a format that is not text; the name picks the
// reader, as a filename does everywhere else in gscore.
frame.contentWindow.postMessage(
  { gscore: "open", value: "X:1\nK:C\nc d e f|", name: "tune.abc" }, "*");
frame.contentWindow.postMessage({ gscore: "zoom", value: 11 }, "*");
```

It sends two of its own back:

```js
window.addEventListener("message", (e) => {
  if (e.data?.gscore === "ready") {
    // The score is up. e.data.title and e.data.tunes say what it is, and
    // e.data.system how tall one system of it is — every instrument
    // once, which is what a frame should open at — and e.data.height how
    // tall all of the music is, for a page that wants no scrolling at
    // all. It is sent again whenever a score is opened, so a page that
    // sends one of its own hears what it turned out to be.
  }
  if (e.data?.gscore === "ended") {
    // The music has finished.
  }
});
```

Wait for `ready` before sending anything: the core is a WebAssembly module and
takes a moment to start.

## Music written into the page

A file beside the page is the wrong shape for eight bars in the middle of a
paragraph: a file, a path and a deploy step for four lines of ABC, and the
music ends up somewhere other than the writing. So write it where it belongs
and load one script:

```html
<script src="https://stricaud.github.io/gscore/app/embed.js"></script>

<div class="gscore">
X:1
T:N&deg; 1
M:C
L:1/4
K:C
c2 c'2 | b2 a2 | g2 f2 | e2 d2 :|
</div>
```

Everything the script finds is engraved where it stands. The text is read with
`textContent`, so HTML entities are letters by the time gscore sees them —
`&deg;` is ° — and the block's own indentation comes off, so the music can be
indented to match the page around it.

The source stays in the page, hidden, with the frame beside it. A reader who
wants the notes can still find them, and so can a search engine.

### Saying what it is

gscore reads several formats and the text does not always say which. The
element does, in whichever way the page already says such things:

```html
<div class="gscore" data-format="musicxml">…</div>
<pre><code class="language-abc">…</code></pre>
```

The second is what a fenced code block in markdown already produces, so the
tunes in a page of documentation engrave themselves:

```html
<script src="…/embed.js" data-selector="code[class^=language-abc]"></script>
```

### Something that is not text

A MuseScore file or a compressed MusicXML is a zip, and a zip cannot be
written into a page as itself. Say how it is written:

```html
<div class="gscore" data-format="mxl" data-encoding="base64">
UEsDBBQAAAgAAAAAIQAuQhWCIgAAACIAAAAIAAAAbWltZXR5cGU…
</div>
```

which is what `base64 -w0 tune.mxl` gives you.

### The settings

| On the element | What it does |
|---|---|
| `data-format=abc` | What the music is written in; the default is ABC |
| `data-encoding=base64` | The text is base64; decode it first |
| `data-height=240` | How tall the frame is, in pixels. Left out, it comes up as tall as one system — every instrument of the score, once — and a frame made shorter than that simply scrolls |
| `data-player=1` | As `player=` above: `1` or `bare` |
| `data-mode=strip` | One unbroken line, scrolling sideways, instead of wrapped to the frame. See [Lines or one line](#lines-or-one-line) |
| `data-zoom=N` | Staff space in pixels |
| `data-autoplay=1`, `data-loop=1`, `data-tune=N` | As in the address |
| `data-name=x.abc` | The filename, where the format alone is not enough |

| On the script tag | What it does |
|---|---|
| `data-selector=…` | What to look for; the default is `.gscore` |
| `data-app=URL` | Where gscore is, for a page hosting its own copy of this script |

The piece's name is written under the music as the page's own text rather
than engraved into the frame — a title block inside the frame takes the top
of it, and what a reader sees first is a name they have just read. Say
`data-title="off"` where the page has named the piece itself.

A page that writes music into itself later can engrave it with
`window.gscoreEmbed(element)`.

There is a page about all of this, with the snippet written for you as you
type, at [stricaud.github.io/gscore/embed.html](https://stricaud.github.io/gscore/embed.html).

## On the same page rather than in a frame

If you are serving gscore's files yourself and want it in the page rather than
in a frame, load `index.html`'s scripts directly. `window.gscore` appears once
the score is up and carries `play`, `stop`, `loadFromURL`, `selectTune`,
`setZoom` and `isPlaying`, and a `gscore:ready` event fires at the same moment.

The frame is the better choice unless you have a reason: gscore brings its own
stylesheet and its own key bindings, and a frame keeps both to itself.

## What it reads

Everything gscore reads: ABC, MuseScore (`.mscz`, `.mscx`), MusicXML
(`.musicxml`, `.mxl`), NoteWorthy Composer (`.nwc`, `.nwctxt`) and gscore's own
`.gscore`. A tune book — an ABC file with forty exercises in it — is one file
and one frame, with `tune=N` choosing which to show.

## Hosting it yourself

```sh
make web            # builds the site into build/web; the editor is in app/
```

That directory is the whole thing: an `index.html`, a stylesheet, the
WebAssembly core and the music font. Copy it anywhere that serves files. It
needs no server-side anything, which is also why GitHub Pages can host it.

One thing to know: the page must be served over HTTP, not opened as a
`file://` URL. WebAssembly will not start from a file URL. `python3 -m
http.server` in that directory is enough for a look.
