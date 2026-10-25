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
      // One unbroken line, the way the still picture above it is drawn and
      // the way a score reads in a band across a page. The editor can do
      // either; left alone in a frame this narrow, pages look cramped.
      mode: "strip",
      score: "../scores/" + piece.src,
      zoom: "9",
    });
    if (autoplay) q.set("autoplay", "1");
    return "app/?" + q.toString();
  }

  function start(piece, autoplay) {
    if (!frame) {
      showLoader();
      frame = document.createElement("iframe");
      frame.className = "editor";
      frame.title = "gscore";
      frame.setAttribute("allow", "autoplay");
      stage.appendChild(frame);
      document.body.classList.add("live");
    }
    frame.src = editorURL(piece, autoplay);
  }

  /* ---- waiting for it ------------------------------------------------ */

  /*
   * Seventeen megabytes is a few seconds on a good line and rather longer on
   * a bad one, and a page that sits there doing nothing during them looks
   * broken. So Gilbert comes out and waits with you.
   *
   * The bar is honest about being a guess: nothing here can see how much of
   * the editor has arrived, because it is fetched by the page inside the
   * frame. It eases towards nearly-full and stops there, and the editor
   * itself finishes it — it posts "ready" to this page once the score is
   * drawn, which is the moment that actually matters.
   */
  const WAITING = [
    "Unpacking the engraver…",
    "Counting the staff lines…",
    "Sharpening the quills…",
    "Teaching Gilbert the tune…",
    "Almost there…",
  ];

  let loader = null;
  let loaderTimers = [];

  function showLoader() {
    if (loader) return;
    loader = document.createElement("div");
    loader.className = "loading";
    loader.innerHTML =
      '<div class="gilbert"><img src="gilbert.svg" alt="" width="72" height="72"></div>' +
      '<p class="loading-says">' + WAITING[0] + "</p>" +
      '<div class="loading-bar"><i></i></div>' +
      '<p class="loading-fine">gscore itself, not a player pretending to be it.</p>';
    stage.appendChild(loader);

    const fill = loader.querySelector(".loading-bar i");
    const says = loader.querySelector(".loading-says");

    // Towards 92% and no further, in steps that get smaller: quick at first,
    // so it is plain that something is happening, then patient.
    let at = 0;
    let step = 0;
    loaderTimers.push(setInterval(() => {
      at += (92 - at) * 0.12;
      fill.style.width = at.toFixed(1) + "%";
    }, 220));
    loaderTimers.push(setInterval(() => {
      step = Math.min(step + 1, WAITING.length - 1);
      says.textContent = WAITING[step];
    }, 2600));
  }

  function hideLoader() {
    if (!loader) return;
    loaderTimers.forEach(clearInterval);
    loaderTimers = [];
    const fill = loader.querySelector(".loading-bar i");
    if (fill) fill.style.width = "100%";
    loader.classList.add("done");
    const going = loader;
    loader = null;
    // Long enough to see the bar reach the end, which is the point of a bar.
    setTimeout(() => going.remove(), 420);
  }

  // The editor says when it is up. Nothing else can: the frame's load event
  // fires when its HTML arrives, which is long before the program has.
  window.addEventListener("message", (e) => {
    if (e.data && e.data.gscore === "ready") {
      hideLoader();
      say("gscore is running in this page — the whole program.");
    }
  });

  function say(words) {
    const el = $("#said");
    el.textContent = words;
    el.classList.add("said");
    setTimeout(() => el.classList.remove("said"), 4000);
  }

  /* ---- choosing a piece -------------------------------------------- */

  // The engraving, fetched the first time a piece is looked at. It is the
  // same drawing the editor will make, so the page does not change its
  // appearance when it hands over — it simply starts moving.
  async function picture(piece) {
    const paper = $(".paper", piece.el);
    if (!paper || paper.dataset.loaded) return;
    paper.dataset.loaded = "1";
    try {
      const res = await fetch("scores/" + piece.id + ".inline.svg");
      if (res.ok) paper.innerHTML = await res.text();
    } catch (_) {
      // No picture, but play still works: it is the editor that plays.
    }
  }

  function show(piece) {
    current = piece;
    void picture(piece);
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

    $("#file").href = "scores/" + current.file;

    let packed = "";
    if (current.abc) {
      packed = await window.gscoreShare.encode(current.abc, "abc");
    } else {
      const res = await fetch("scores/" + current.src);
      const bytes = new Uint8Array(await res.arrayBuffer());
      // A whole sonata does not go in a link, and there is no sense
      // building a quarter of a million characters to find that out. A
      // tune of a few bars is a few hundred; past this it is a file, and
      // the editor is pointed at it instead.
      if (bytes.length <= 24 * 1024) {
        packed = await window.gscoreShare.encode(bytes, current.kind);
      }
    }

    if (packed) {
      linkFor = base + "app/#s=" + packed;
    } else {
      // Not carried in the link, but still one address that opens the
      // piece in the editor — it fetches the file beside it.
      linkFor = base + "app/?score=../scores/" + current.src;
    }
    $("#open").href = linkFor;

    if (packed && linkFor.length <= COMFORTABLE) {
      size.textContent = linkFor.length + " characters";
      copy.textContent = "Copy the link";
      copy.dataset.page = "";
    } else if (packed) {
      size.textContent =
        "this piece needs " + linkFor.length.toLocaleString() +
        " characters, so the button copies a link to the page instead";
      copy.textContent = "Copy a link to it";
      copy.dataset.page = base + "scores/" + current.file;
    } else {
      size.textContent =
        "too long a piece to carry in a link, so the button copies one to " +
        "the page instead — which still needs no account and nothing installed";
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
