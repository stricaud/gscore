/*
 * hero.js - the score on the front page.
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

/*
 * What a visitor sees first, and what happens when they press play.
 *
 * The music on this page was engraved by gscore at the time the site was
 * built, and is sitting here as SVG. That is what paints: a picture of the
 * piece, immediately, with nothing to download.
 *
 * Pressing play hands over to the real thing. The editor is loaded into a
 * frame in its player mode — the same program as the desktop build, as
 * WebAssembly — and from then on the page is running gscore rather than an
 * imitation of it: the same engraving, the same cursor travelling the music,
 * the same playback, with real instruments if the visitor has fetched a
 * soundfont and oscillators if not.
 *
 * It is seventeen megabytes, which is why it waits to be asked. This page
 * used to answer with a small player of its own; it started faster and
 * behaved differently from the program it was advertising, which is the one
 * thing a demonstration must not do.
 */

(function () {
  "use strict";

  const $ = (sel, within) => (within || document).querySelector(sel);
  const $$ = (sel, within) => Array.from((within || document).querySelectorAll(sel));

  const pieces = $$(".piece").map((el) => ({
    id: el.dataset.id,
    title: el.dataset.title,
    composer: el.dataset.composer || "",
    note: el.dataset.note || "",
    file: el.dataset.file,
    src: el.dataset.src,
    kind: el.dataset.kind,
    abc: $(".abc", el) ? $(".abc", el).textContent : "",
    el,
  }));
  if (!pieces.length) return;

  const stage = $("#stage");
  let current = pieces[0];
  let frame = null; // the editor, once it has been asked for

  /* ---- handing over to the editor ---------------------------------- */

  function editorURL(piece, autoplay) {
    // player=1 takes away the menus and the palette and leaves the score
    // and a transport. score= is resolved against the editor's own page.
    const q = new URLSearchParams({
      player: "1",
      score: "../scores/" + piece.src,
      zoom: "9",
    });
    if (autoplay) q.set("autoplay", "1");
    return "app/?" + q.toString();
  }

  function start(piece, autoplay) {
    if (!frame) {
      frame = document.createElement("iframe");
      frame.className = "editor";
      frame.title = "gscore";
      frame.setAttribute("allow", "autoplay");
      stage.appendChild(frame);
      document.body.classList.add("live");
      say("Starting gscore — the whole program, in this page.");
    }
    frame.src = editorURL(piece, autoplay);
  }

  function say(words) {
    const el = $("#said");
    el.textContent = words;
    el.classList.add("said");
    setTimeout(() => el.classList.remove("said"), 4000);
  }

  /* ---- choosing a piece -------------------------------------------- */

  function show(piece) {
    current = piece;
    pieces.forEach((p) => (p.el.hidden = p !== piece));
    $$("#pieces button").forEach((b) =>
      b.setAttribute("aria-pressed", String(b.dataset.id === piece.id)),
    );
    $("#by").textContent = piece.composer
      ? piece.title + " — " + piece.composer
      : piece.title;
    $("#note").textContent = piece.note;
    // Only reload the editor if it is already here. Switching pieces before
    // anybody has pressed play should not fetch seventeen megabytes.
    if (frame) start(piece, false);
    refreshLinks();
  }

  /* ---- sharing ------------------------------------------------------ */

  let linkFor = "";

  // Above this a link stops being something anybody pastes into a message.
  // It still works — a fragment never goes near a server — but the page
  // should not pretend otherwise.
  const COMFORTABLE = 2000;

  async function refreshLinks() {
    const base = location.href.replace(/[?#].*$/, "").replace(/index\.html$/, "");
    const size = $("#size");
    const copy = $("#copy");

    let packed;
    if (current.abc) {
      packed = await window.gscoreShare.encode(current.abc, "abc");
    } else {
      // Written in something other than ABC, so the file itself goes in. A
      // .mxl is already a zip; it comes out about eight kilobytes, which is
      // ten thousand characters of link.
      const res = await fetch("scores/" + current.src);
      packed = await window.gscoreShare.encode(
        new Uint8Array(await res.arrayBuffer()), current.kind);
    }

    linkFor = base + "app/#s=" + packed;
    $("#open").href = linkFor;
    $("#file").href = "scores/" + current.file;

    if (linkFor.length <= COMFORTABLE) {
      size.textContent = linkFor.length + " characters";
      copy.textContent = "Copy the link";
      copy.dataset.page = "";
    } else {
      size.textContent =
        "this piece needs " + linkFor.length.toLocaleString() +
        " characters, so the button copies a link to the page instead";
      copy.textContent = "Copy a link to it";
      copy.dataset.page = base + "scores/" + current.file;
    }
  }

  async function copyLink() {
    const page = $("#copy").dataset.page;
    const what = page || linkFor;
    try {
      await navigator.clipboard.writeText(what);
      say(page
        ? "Copied. That page carries the music with it — no account, nothing to expire."
        : "Copied. That link is the music — paste it anywhere.");
    } catch (_) {
      // Clipboard refused, which some browsers do unless the page is
      // focused. Show it instead, so there is still something to copy.
      say(what);
    }
  }

  /* ---- wiring -------------------------------------------------------- */

  $("#play").addEventListener("click", () => start(current, true));
  $("#copy").addEventListener("click", copyLink);
  $$("#pieces button").forEach((b) =>
    b.addEventListener("click", () => show(pieces.find((p) => p.id === b.dataset.id))),
  );

  // The still picture is a button too: clicking the music is what somebody
  // does when they want to hear it.
  stage.addEventListener("click", (e) => {
    if (frame || e.target.closest("button, a")) return;
    start(current, true);
  });

  show(pieces[0]);
})();
