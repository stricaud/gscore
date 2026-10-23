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
`https://stricaud.github.io/gscore/` needs no build at all.

One thing to watch: `score=` is resolved against the **gscore page**, not
against your page. The sample reaches back out with `../../samples/…` for
exactly that reason; an absolute URL avoids the question.

## The short version

```html
<iframe src="https://stricaud.github.io/gscore/?player=1&score=tune.abc"
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
frame.contentWindow.postMessage({ gscore: "zoom", value: 11 }, "*");
```

It sends two of its own back:

```js
window.addEventListener("message", (e) => {
  if (e.data?.gscore === "ready") {
    // The score is up. e.data.title and e.data.tunes say what it is.
  }
  if (e.data?.gscore === "ended") {
    // The music has finished.
  }
});
```

Wait for `ready` before sending anything: the core is a WebAssembly module and
takes a moment to start.

## On the same page rather than in a frame

If you are serving gscore's files yourself and want it in the page rather than
in a frame, load `index.html`'s scripts directly. `window.gscore` appears once
the score is up and carries `play`, `stop`, `loadFromURL`, `selectTune`,
`setZoom` and `isPlaying`, and a `gscore:ready` event fires at the same moment.

The frame is the better choice unless you have a reason: gscore brings its own
stylesheet and its own key bindings, and a frame keeps both to itself.

## What it reads

Everything gscore reads: ABC, MusicXML (`.musicxml`, `.mxl`), NoteWorthy
Composer (`.nwc`, `.nwctxt`) and gscore's own `.gscore`. A tune book — an ABC
file with forty exercises in it — is one file and one frame, with `tune=N`
choosing which to show.

## Hosting it yourself

```sh
make web            # builds into build/web
```

That directory is the whole thing: an `index.html`, a stylesheet, the
WebAssembly core and the music font. Copy it anywhere that serves files. It
needs no server-side anything, which is also why GitHub Pages can host it.

One thing to know: the page must be served over HTTP, not opened as a
`file://` URL. WebAssembly will not start from a file URL. `python3 -m
http.server` in that directory is enough for a look.
