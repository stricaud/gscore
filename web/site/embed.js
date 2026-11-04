/*
 * embed.js - music written into a page, engraved where it stands.
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

/*
 * Putting a tune in a page without putting it in a file.
 *
 * gscore embeds as a frame with a score= address, which means the music
 * lives in a file beside the page. For eight bars in the middle of a
 * paragraph that is a file, a path and a deploy step for four lines of
 * ABC, and the music is no longer where the writing is.
 *
 * So: write the tune in the page and load this. Every element it finds is
 * engraved in place, and the text stays in the page where it was typed.
 *
 *     <script src="https://stricaud.github.io/gscore/app/embed.js"></script>
 *
 *     <div class="gscore">
 *     X:1
 *     T:N&deg; 1
 *     K:C
 *     c2 d2 e2 f2|
 *     </div>
 *
 * The text is read with textContent, so HTML entities are already letters
 * by the time this sees them — &deg; is ° and &amp; is &, which is what
 * anybody writing a title in a web page expects.
 *
 * Nothing is uploaded. The frame is gscore, which reads the ABC in the
 * reader's own browser, as it does everywhere else.
 */

(function () {
  "use strict";

  // Where gscore is, taken from this script's own address: a page that
  // loaded embed.js from a copy of the site should get the frames from
  // that copy and not from somewhere else.
  const self = document.currentScript;
  const app = new URL(self ? self.src : "./embed.js", location.href);
  // data-app points the frames somewhere else, for a page that keeps its
  // own copy of this script but uses a gscore hosted elsewhere — or the
  // other way round.
  //
  //     <script src="embed.js" data-app="https://stricaud.github.io/gscore/app/">
  const APP = self && self.dataset.app
    ? new URL(self.dataset.app, location.href).href.replace(/\/?$/, "/") +
      "index.html"
    : app.href.replace(/[^/]*$/, "index.html");

  // What to look for.
  //
  // One class, and it says the program rather than the format: a block
  // may hold MusicXML or a MuseScore file as readily as ABC, and a page
  // marking those up as a source of ABC would be saying something
  // untrue. A
  // page with a name of its own says so on the tag:
  //
  //     <script src="…/embed.js" data-selector=".tune"></script>
  const SELECTOR = (self && self.dataset.selector) ||
    ".gscore";

  // What the music is written in.
  //
  // gscore reads several things and the text alone does not always say
  // which: ABC and a MusicXML document are both text, and a MuseScore
  // file is a zip. So the element says, in whichever way the page
  // already says such things:
  //
  //     <div class="gscore" data-format="musicxml">
  //     <pre><code class="language-abc">        (what markdown emits)
  //     <div class="gscore">                (ABC, the common case)
  //
  // The name is what the reader is chosen by, the same as a file being
  // opened — so a format gscore learns later works here without this
  // script being touched.
  function formatOf(el) {
    if (el.dataset.format) return el.dataset.format.toLowerCase();
    for (const c of el.classList) {
      const m = /^(?:language|lang)-([a-z0-9]+)$/i.exec(c);
      if (m) return m[1].toLowerCase();
    }
    return "abc";
  }

  // The bytes to hand over.
  //
  // Text as it stands, and base64 decoded first: a MuseScore file or a
  // compressed MusicXML is a zip, and a zip cannot be written into a web
  // page as itself. Decoded here rather than in gscore because this is
  // where the page said what it had put there.
  function contentOf(el, text) {
    if ((el.dataset.encoding || "").toLowerCase() !== "base64") return text;
    const raw = atob(text.replace(/\s+/g, ""));
    const bytes = new Uint8Array(raw.length);
    for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
    return bytes;
  }

  // dedent takes off the indentation the page is written with.
  //
  // ABC cares about the left margin: a line that begins with spaces is
  // not a header and a tune indented to match the HTML around it does not
  // read. Nobody should have to break the shape of their page to write
  // music in it, so the common indentation of the block comes off — the
  // same rule every language with a block string uses.
  function dedent(text) {
    const lines = text.replace(/\r\n?/g, "\n").split("\n");
    while (lines.length && !lines[0].trim()) lines.shift();
    while (lines.length && !lines[lines.length - 1].trim()) lines.pop();
    let common = null;
    for (const line of lines) {
      if (!line.trim()) continue;
      const indent = line.match(/^[ \t]*/)[0].length;
      common = common === null ? indent : Math.min(common, indent);
    }
    if (!common) return lines.join("\n");
    return lines.map((l) => l.slice(common)).join("\n");
  }

  // The address of the frame for one score. Everything the embedding
  // documentation lists can be set on the element: data-player,
  // data-zoom, data-autoplay, data-loop, data-tune.
  function address(el) {
    const q = new URLSearchParams();
    q.set("player", el.dataset.player || "1");
    for (const name of ["zoom", "autoplay", "loop", "tune"]) {
      if (el.dataset[name]) q.set(name, el.dataset[name]);
    }
    return APP + "?" + q.toString();
  }

  // engrave puts a frame where the text is and sends it the tune.
  //
  // The text is left in the page rather than thrown away: it is what the
  // author wrote, it is what a reader copying the tune wants, and a page
  // whose source disappears into a frame cannot be searched for the music
  // in it. It is hidden, and the frame goes beside it.
  function engrave(el) {
    if (el.dataset.gscoreDone) return;
    el.dataset.gscoreDone = "1";

    const written = dedent(el.textContent || "");
    if (!written.trim()) return;
    const format = formatOf(el);
    const content = contentOf(el, written);
    const name = el.dataset.name || "tune." + format;

    const frame = document.createElement("iframe");
    frame.src = address(el);
    frame.className = "gscore-frame";
    frame.setAttribute("title", "Music, engraved by gscore");
    frame.style.width = "100%";
    frame.style.border = "0";
    frame.style.height = (el.dataset.height || "240") + "px";
    frame.loading = "lazy";

    // Waiting for the frame to say it is up. The core is WebAssembly and
    // takes a moment; sending before then sends into a page that has no
    // listener yet and the tune is silently never read.
    const sent = (e) => {
      if (e.source !== frame.contentWindow) return;
      const m = e.data;
      if (!m || typeof m !== "object" || m.gscore !== "ready") return;
      frame.contentWindow.postMessage(
        { gscore: "open", value: content, name }, "*");
      window.removeEventListener("message", sent);
    };
    window.addEventListener("message", sent);

    el.hidden = true;
    el.style.display = "none";   // in case the page styles it visible
    el.parentNode.insertBefore(frame, el.nextSibling);
  }

  function engraveAll(root) {
    const where = root || document;
    where.querySelectorAll(SELECTOR).forEach(engrave);
  }

  // Whenever the page is ready, and once more for anything added later —
  // a tune written into a page by a script of the page's own is still a
  // tune. Asked for by name so a page can do it itself.
  window.gscoreEmbed = engraveAll;
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => engraveAll());
  } else {
    engraveAll();
  }
})();
