/*
 * hero.js - the score on the front page: playing it, changing it, sharing it.
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

/*
 * The music on this page was engraved by gscore itself, at the time the site
 * was built, and is sitting in the page as SVG with the performance beside it
 * as JSON. Nothing here draws a note or works out a rhythm: that was all
 * decided by the same Go code the desktop program runs, which is the point
 * being made — what you are looking at is the program's own output, not a
 * picture of it.
 *
 * What is left for this file is the part a visitor touches. The voices below
 * mirror the ones in internal/render/html/player.go, which is where the
 * shipped single-file player gets them; that player has no knobs on it, and
 * this page wants knobs.
 */

(function () {
  "use strict";

  // How each family of instrument is sounded. Oscillators rather than
  // recordings: thirty megabytes of samples to hear eight bars is not a
  // landing page. The editor plays through real soundfonts.
  const VOICES = {
    flute: { wave: "sine", cut: 2400, a: 0.045, d: 0.1, s: 0.85, r: 0.12 },
    reed: { wave: "sawtooth", cut: 2200, a: 0.03, d: 0.12, s: 0.8, r: 0.12 },
    brass: { wave: "sawtooth", cut: 2800, a: 0.035, d: 0.14, s: 0.78, r: 0.12 },
    bowed: { wave: "sawtooth", cut: 2000, a: 0.06, d: 0.14, s: 0.8, r: 0.18 },
    sustained: { wave: "sawtooth", cut: 2600, a: 0.04, d: 0.12, s: 0.75, r: 0.14 },
    plucked: { wave: "triangle", cut: 3200, a: 0.004, d: 0.22, s: 0.0, r: 0.1 },
    tuned: { wave: "triangle", cut: 3600, a: 0.002, d: 0.3, s: 0.0, r: 0.08 },
  };

  const $ = (sel, within) => (within || document).querySelector(sel);
  const $$ = (sel, within) => Array.from((within || document).querySelectorAll(sel));

  const pieces = $$(".piece").map((el) => ({
    id: el.dataset.id,
    title: el.dataset.title,
    composer: el.dataset.composer || "",
    note: el.dataset.note || "",
    file: el.dataset.file,
    abc: $(".abc", el).textContent,
    sound: JSON.parse($(".sound", el).textContent),
    paper: $(".paper", el),
    el,
  }));
  if (!pieces.length) return;

  let current = pieces[0];
  let rate = 1;
  let voice = "";          // "" means each part keeps its own sound
  let audio = null;
  let playing = false;
  let startedAt = 0;
  let timer = null;

  /* ---- playing ---------------------------------------------------------- */

  function context() {
    if (!audio) audio = new (window.AudioContext || window.webkitAudioContext)();
    return audio;
  }

  function sound(ctx, n, at) {
    const v = VOICES[voice || n.f] || VOICES.sustained;
    const hz = 440 * Math.pow(2, (n.m - 69) / 12);
    // A note is held for as long as it is written, less whatever the
    // articulation takes off it: that is what g carries, and it is why a
    // staccato sounds short here and not only on paper.
    const held = (n.d * (n.g || 1)) / rate;

    const osc = ctx.createOscillator();
    osc.type = v.wave;
    osc.frequency.value = hz;

    const tone = ctx.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = v.cut;

    const gain = ctx.createGain();
    const peak = ((n.v || 80) / 127) * 0.22;
    const g = gain.gain;
    g.setValueAtTime(0.0001, at);
    g.exponentialRampToValueAtTime(peak, at + v.a);
    g.exponentialRampToValueAtTime(Math.max(peak * v.s, 0.0001), at + v.a + v.d);
    g.setValueAtTime(Math.max(peak * v.s, 0.0001), at + Math.max(held, v.a + v.d));
    g.exponentialRampToValueAtTime(0.0001, at + Math.max(held, v.a + v.d) + v.r);

    osc.connect(tone).connect(gain).connect(ctx.destination);
    osc.start(at);
    osc.stop(at + Math.max(held, v.a + v.d) + v.r + 0.05);
  }

  function play() {
    stop();
    const ctx = context();
    if (ctx.state === "suspended") ctx.resume();

    const lead = 0.08; // a moment to schedule in, so the first note is not clipped
    startedAt = ctx.currentTime + lead;
    for (const n of current.sound.notes) sound(ctx, n, startedAt + n.t / rate);

    playing = true;
    document.body.classList.add("playing");
    $("#play").textContent = "Stop";
    $("#play").setAttribute("aria-label", "Stop");
    follow();
  }

  function stop() {
    if (audio) {
      // Closing and reopening is the blunt way to silence everything already
      // scheduled, and the only one that does not need a handle on each note.
      audio.close();
      audio = null;
    }
    playing = false;
    document.body.classList.remove("playing");
    const b = $("#play");
    if (b) {
      b.textContent = "Play";
      b.setAttribute("aria-label", "Play");
    }
    if (timer) cancelAnimationFrame(timer);
    timer = null;
    clearHighlight();
  }

  /* ---- following the music across the page ------------------------------ */

  function clearHighlight() {
    $$(".lit").forEach((e) => e.classList.remove("lit"));
    const box = $("#barbox");
    if (box) box.style.opacity = "0";
  }

  function barBox() {
    let box = $("#barbox", current.paper);
    if (!box) {
      const svg = $("svg", current.paper);
      if (!svg) return null;
      box = document.createElementNS("http://www.w3.org/2000/svg", "rect");
      box.setAttribute("id", "barbox");
      box.setAttribute("rx", "1");
      svg.insertBefore(box, svg.firstChild);
    }
    return box;
  }

  function follow() {
    if (!playing || !audio) return;
    const now = (audio.currentTime - startedAt) * rate;
    const end = current.sound.duration || 0;

    if (now > end + 0.4) {
      stop();
      return;
    }

    // The bar being played, lit behind the notes.
    const bar = current.sound.bars.find((b) => now >= b.t && now < b.t + b.d);
    const box = barBox();
    if (box && bar) {
      const g = (current.sound.boxes || []).find((x) => x.i === bar.i);
      if (g) {
        box.setAttribute("x", g.a);
        box.setAttribute("y", g.c);
        box.setAttribute("width", Math.max(g.b - g.a, 0));
        box.setAttribute("height", Math.max(g.e - g.c, 0));
        box.style.opacity = "1";
      }
    }

    // And the notes themselves, which the engraver tagged with the same
    // number it gave the performance.
    const want = new Set();
    for (const n of current.sound.notes) {
      if (n.r && now >= n.t && now < n.t + n.d) want.add(String(n.r));
    }
    $$("[data-ref]", current.paper).forEach((e) => {
      const on = want.has(e.getAttribute("data-ref"));
      if (on !== e.classList.contains("lit")) e.classList.toggle("lit", on);
    });

    timer = requestAnimationFrame(follow);
  }

  /* ---- choosing a piece ------------------------------------------------- */

  function show(piece) {
    stop();
    current = piece;
    pieces.forEach((p) => (p.el.hidden = p !== piece));
    $$("#pieces button").forEach((b) =>
      b.setAttribute("aria-pressed", String(b.dataset.id === piece.id)),
    );
    const by = $("#by");
    by.textContent = piece.composer ? piece.title + " — " + piece.composer : piece.title;
    $("#note").textContent = piece.note;
    refreshLinks();
  }

  /* ---- sharing ---------------------------------------------------------- */

  let linkFor = "";

  async function refreshLinks() {
    const packed = await window.gscoreShare.encode(current.abc);
    const base = location.href.replace(/[?#].*$/, "").replace(/index\.html$/, "");
    linkFor = base + "app/#s=" + packed;
    $("#open").href = linkFor;
    $("#file").href = "scores/" + current.file;
    $("#size").textContent = linkFor.length + " characters";
  }

  async function copyLink() {
    const said = $("#said");
    try {
      await navigator.clipboard.writeText(linkFor);
      said.textContent = "Copied. That link is the music — paste it anywhere.";
    } catch (_) {
      // Clipboard refused, which some browsers do unless the page is focused.
      // Show it instead, so there is still something to copy.
      said.textContent = linkFor;
    }
    said.classList.add("said");
    setTimeout(() => said.classList.remove("said"), 4000);
  }

  /* ---- wiring ----------------------------------------------------------- */

  $("#play").addEventListener("click", () => (playing ? stop() : play()));
  $("#copy").addEventListener("click", copyLink);

  $("#speed").addEventListener("input", (e) => {
    rate = Number(e.target.value);
    $("#speedsaid").textContent = rate === 1 ? "normal" : rate.toFixed(2) + "×";
    if (playing) play();
  });

  $("#voice").addEventListener("change", (e) => {
    voice = e.target.value;
    if (playing) play();
  });

  $$("#pieces button").forEach((b) =>
    b.addEventListener("click", () => show(pieces.find((p) => p.id === b.dataset.id))),
  );

  // Space plays, the way it does in the editor — but not while someone is
  // using the keyboard to reach a button.
  document.addEventListener("keydown", (e) => {
    if (e.code !== "Space" || e.target.closest("button, select, input, a")) return;
    e.preventDefault();
    playing ? stop() : play();
  });

  show(pieces[0]);
})();
