/*
 * player.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package html

// The player that travels with a shared page.
//
// It is not the editor's. The editor's player knows about selections, tunes,
// books, drum kits and a score that can change under it; a page is one
// score, drawn once, and the whole of what it has to do is make the sounds
// at the right times and show where they are. Writing it small is what keeps
// a shared file a file rather than a program.
//
// Everything it needs was worked out before the page was written: when each
// note sounds, how long the bars are, and where each bar was drawn. It does
// no engraving and no arithmetic about music.

const playerCSS = `
  #transport {
    position: sticky; top: 0; z-index: 2;
    max-width: 1100px; margin: 0 auto 12px;
    display: flex; align-items: center; gap: 12px;
    padding: 8px 12px; border-radius: 4px;
    background: #fff; box-shadow: 0 1px 4px rgba(0,0,0,.18);
    font: 13px/1 system-ui, -apple-system, "Segoe UI", sans-serif;
  }
  #transport button {
    font: inherit; font-size: 15px; line-height: 1;
    padding: 7px 13px; border: 1px solid #ccc; border-radius: 4px;
    background: #fafafa; color: #111; cursor: pointer;
  }
  #transport button:hover { background: #f0f0f0; }
  #transport .clock { font-variant-numeric: tabular-nums; color: #444; }
  #transport .fill { flex: 1; }
  #transport .note { color: #888; }
  /*
   * Showing what is sounding.
   *
   * A hairline through the bar says where the music has got to and nothing
   * about what is being played, which on a page with fifty-four staves on it
   * is the more interesting half. So there are three things now: a wash
   * trailing behind the playhead, so the eye is led rather than pointed; the
   * heads that are sounding, coloured; and a ring leaving each one as it
   * speaks, which is what makes a chord across the staves read as a chord
   * rather than as a column of coloured dots.
   *
   * The ring is drawn and thrown away. It is a moment, not a state.
   */
  rect.gscore-wash { fill: url(#gscore-wash); pointer-events: none; }
  rect.gscore-cursor { fill: #e8513a; opacity: .5; pointer-events: none; }
  .gscore-score .sounding {
    fill: #e8513a;
    transition: fill 60ms linear;
  }
  circle.gscore-ping {
    fill: none; stroke: #e8513a; stroke-width: 1.1; pointer-events: none;
    /* Scaled rather than grown by its radius: a geometry property in a
       stylesheet is newer than some of the browsers this page will be
       opened in, and a transform is not. */
    transform-box: fill-box; transform-origin: 50% 50%;
    animation: gscore-ping 620ms cubic-bezier(.22,.7,.3,1) forwards;
  }
  @keyframes gscore-ping {
    from { transform: scale(.3);  opacity: .75; }
    to   { transform: scale(2.6); opacity: 0; }
  }
  /* Somebody who would rather the page held still gets the colour and the
     wash, and no ring. */
  @media (prefers-reduced-motion: reduce) {
    circle.gscore-ping { display: none; }
  }
  @media print { #transport { display: none; } }
`

const playerHTML = `<div id="transport">
  <button id="play" aria-label="Play">&#9654;&#65038; Play</button>
  <span class="clock" id="clock">0:00</span>
  <span class="fill"></span>
  <span class="note">Engraved with Gscore</span>
</div>
`

const playerJS = `
(function () {
  "use strict";
  // The drawing first, because on a long score it is not in the page: it is
  // fetched in pieces, and the sooner the first one is asked for the sooner
  // there is music to look at. See the band machinery below.
  var svg = document.querySelector("svg.gscore-score");
  if (!svg) return;
  startBands();

  var el = document.getElementById("gscore-sound");
  if (!el) return;
  var sound = JSON.parse(el.textContent);
  var button = document.getElementById("play");
  var clock = document.getElementById("clock");
  if (!sound || !button) return;

  // A voice for each family the score names. Not the editor's — this is
  // what fits in a shared file: one oscillator, one filter, one envelope.
  // Something held and something struck are the two things that have to
  // sound different, and the families divide along that line.
  var VOICES = {
    flute:     { wave: "sine",     cut: 2400, a: 0.045, d: 0.10, s: 0.85, r: 0.12 },
    reed:      { wave: "sawtooth", cut: 2200, a: 0.030, d: 0.12, s: 0.80, r: 0.12 },
    brass:     { wave: "sawtooth", cut: 2800, a: 0.035, d: 0.14, s: 0.78, r: 0.12 },
    bowed:     { wave: "sawtooth", cut: 2000, a: 0.060, d: 0.14, s: 0.80, r: 0.18 },
    sustained: { wave: "sawtooth", cut: 2600, a: 0.040, d: 0.12, s: 0.75, r: 0.14 },
    plucked:   { wave: "triangle", cut: 3200, a: 0.004, d: 0.22, s: 0.0,  r: 0.10 },
    tuned:     { wave: "triangle", cut: 3600, a: 0.002, d: 0.30, s: 0.0,  r: 0.08 },
    // Struck and left to ring, which is not the same as plucked.
    //
    // The model keeps these two apart and says why: a plectrum is bright at
    // the moment it picks and gone very soon after, where a hammer is softer
    // at the front and the note rings on for as long as it is written. There
    // was no piano here, so every piano in every shared page fell through to
    // the pluck below and came out buzzing and then cut off halfway through
    // the note — on a song for voice and piano, which is most of what a
    // shelf holds, that is nearly all of what you hear.
    //
    // The editor worked this out already; these are its numbers. A little
    // sustain, so the note lasts its written length instead of ringing down
    // to nothing in the middle of it.
    piano:     { wave: "triangle", cut: 2400, a: 0.004, d: 0.90, s: 0.16, r: 0.18 }
  };
  // Anything the score does not name is played as a piano, which is what a
  // keyboard with nothing else said about it should sound like. Sustained —
  // an organ — was the old answer, and a score that said nothing about its
  // instrument came out droning. Then it was the pluck, which buzzed.
  function voiceFor(f) { return VOICES[f] || VOICES.piano; }

  // A recording of the score, served beside the page, if there is one.
  //
  // With it the page plays the instruments instead of imitating them, and
  // makes no sound of its own at all: the oscillators below are never
  // reached, and this element is both the transport and the clock. Without
  // it everything carries on as before — a page sent to somebody has nowhere
  // to fetch a recording from, so it has to be able to do without.
  var media = document.getElementById("gscore-audio");

  var audio = null, master = null, voices = [];
  var playing = false, startedAt = 0, from = 0, raf = 0, lit = [];
  // next is how far the scheduler has got through the notes, lightAt how
  // far the highlight has; pumping is the timer that tops the first up.
  var next = 0, lightAt = 0, pumping = 0, lightNow = -1, shownBar = -1;

  // The notes in the order they sound.
  //
  // The page lists them staff by staff, because that is the order they were
  // resolved in: the whole of the piccolo's part, then the whole of the
  // first flute's, and so on. A player that schedules as it goes needs them
  // in time order instead, so they are put in it once, here — rather than
  // asking the page for them that way, because the pages already published
  // do not.
  var notes = (sound.notes || []).slice().sort(function (a, b) {
    return a.t - b.t;
  });

  // How far ahead of the sound the notes are handed to the audio clock, and
  // how often that is topped up.
  //
  // All of them at once is what this did, and it is not a thing that can be
  // done: the ninth symphony is a hundred and thirty-one thousand notes and
  // three audio nodes each, and a browser asked for four hundred thousand
  // oscillators in one turn stops making them partway through. What came out
  // was the handful of staves whose notes were asked for first — the piccolo
  // and the flutes — and an orchestra of silent ones behind them. It sounded
  // like a page too big to play, and it was really a queue too long to make.
  //
  // The window is wider than the one second a background tab throttles a
  // timer to, so a page left in another tab keeps playing.
  var HORIZON = 2.5, PUMP = 200;

  // Score time to audio time. The two are a fixed distance apart for as long
  // as a playing lasts: startedAt is when score time "from" was heard.
  function at(t) { return startedAt + (t - from); }

  // How far behind the audio clock the sound you can hear is.
  //
  // audio.currentTime is where the browser is writing sound, not where the
  // speaker is playing it. Everything already handed over is still on its way
  // out through the device's buffers, and over Bluetooth that journey is a
  // fifth of a second — at this music's tempo, half a note. A cursor driven
  // by the writing clock is that far ahead of the sound: you watch a notehead
  // light up and then hear it, and at the end of the piece the highlight
  // finishes while the last chord is still on its way to the speaker, which
  // is exactly what it sounds like — notes after the last one.
  //
  // outputLatency is the whole journey and the right number. Safari publishes
  // only baseLatency, which is the first leg of it — better than nothing.
  function lag() {
    if (!audio) return 0;
    var l = audio.outputLatency;
    if (!(l > 0)) l = audio.baseLatency;
    if (!(l > 0)) return 0;
    // A device claiming more than this is either wrong or unlistenable, and
    // believing it would put the cursor behind the music instead of ahead.
    return Math.min(l, 0.5);
  }

  // Where the music has got to, in the score's own seconds.
  //
  // The recording's own clock when there is one, and otherwise how far the
  // audio clock has moved since we started, less what is still in the pipe.
  // Either way it is the clock of the sound that is reaching the listener and
  // not the wall's, nor the scheduler's: a cursor driven by a clock that
  // drifts from what you can hear ends up pointing at the wrong bar, which is
  // the one thing it must never do.
  function clockNow() {
    if (media) return media.currentTime;
    return from + (audio.currentTime - lag() - startedAt);
  }

  function wake() {
    if (!audio) audio = new (window.AudioContext || window.webkitAudioContext)();
    if (audio.state !== "running") audio.resume();
    return audio;
  }

  function sound1(n, when) {
    var v = voiceFor(n.f);
    var hz = 440 * Math.pow(2, (n.m - 69) / 12);
    // n.g is how much of the written length sounds: the staccato dots and
    // slurs, worked out when the page was made.
    var gap = n.d * (1 - (n.g > 0 ? n.g : 0.88));
    if (gap > 0) gap = Math.max(0.02, gap);
    var dur = Math.max(0.05, n.d - gap);
    var osc = audio.createOscillator();
    var filter = audio.createBiquadFilter();
    var gain = audio.createGain();
    osc.type = v.wave;
    osc.frequency.setValueAtTime(hz, when);
    filter.type = "lowpass";
    filter.frequency.setValueAtTime(Math.min(v.cut, hz * 6 + 400), when);

    var peak = 0.22 * (0.35 + 0.65 * ((n.v || 80) / 127));
    var g = gain.gain;
    g.setValueAtTime(0, when);
    g.linearRampToValueAtTime(peak, when + v.a);
    // The decay cannot outlast the note it is decaying through.
    //
    // A piano falls away over nine tenths of a second, which is longer than
    // half the notes in a song: left uncapped, the fall was aimed at a moment
    // past the end of the note and the fade-out before it, so the events
    // arrived out of order and the note did not fade at all. Capped at half
    // the note, a short one decays quickly and a long one rings, which is
    // what a piano does.
    var decay = Math.min(v.d, dur * 0.5);
    if (v.s === 0) {
      g.exponentialRampToValueAtTime(0.0005, when + Math.min(dur, decay) + 0.01);
    } else {
      g.linearRampToValueAtTime(peak * v.s, when + v.a + decay);
      g.setValueAtTime(peak * v.s, Math.max(when + v.a + decay, when + dur - v.r));
      g.linearRampToValueAtTime(0, when + dur);
    }
    osc.connect(filter); filter.connect(gain); gain.connect(master);
    osc.start(when);
    osc.stop(when + dur + 0.06);
    // Held with the time it falls silent, so the ones that are over can be
    // let go of without asking each of them.
    voices.push({ o: osc, end: when + dur + 0.06 });
  }

  // Hand the audio clock everything that sounds between now and the edge of
  // the window, and let go of what has finished.
  function pump() {
    if (!playing || !audio) return;
    var until = audio.currentTime + HORIZON;
    while (next < notes.length && at(notes[next].t) <= until) {
      sound1(notes[next], at(notes[next].t));
      next++;
    }
    if (voices.length < 512) return;
    var now = audio.currentTime, keep = [];
    for (var i = 0; i < voices.length; i++) {
      if (voices[i].end > now) keep.push(voices[i]);
    }
    voices = keep;
  }

  // Where the music is, as a place on the page: the bar it is in, and how
  // far across that bar. Time is the thing every staff agrees on.
  var boxOf = {};
  (sound.boxes || []).forEach(function (b) { boxOf[b.i] = b; });

  // Carried on from the bar the last frame was in, not begun at the first.
  //
  // The bars are in playing order, so their starts only increase; a moment
  // earlier than the last — a seek, or somebody moving the recording — starts
  // the walk again. Reading all of them every frame costs what the piece has
  // already played, and the ninth symphony has three thousand two hundred.
  var barFrom = 0;
  function placeAt(now) {
    var bars = sound.bars || [];
    if (!bars.length) return null;
    if (barFrom >= bars.length || bars[barFrom].t > now) barFrom = 0;
    while (barFrom + 1 < bars.length && bars[barFrom + 1].t <= now) barFrom++;
    var bar = bars[barFrom];
    if (now < bar.t || now >= bar.t + bar.d) return null;
    var box = boxOf[bar.i];
    if (!box) return null;
    var part = bar.d > 0 ? (now - bar.t) / bar.d : 0;
    return { x: box.a + (box.b - box.a) * part, top: box.c, bottom: box.d,
             bar: bar.i };
  }

  function svgEl(name) {
    return document.createElementNS("http://www.w3.org/2000/svg", name);
  }

  // The gradient the wash is painted with, made once.
  var defined = false;
  function define() {
    if (defined) return;
    defined = true;
    var d = svgEl("defs");
    var g = svgEl("linearGradient");
    g.setAttribute("id", "gscore-wash");
    g.setAttribute("x1", "0"); g.setAttribute("y1", "0");
    g.setAttribute("x2", "1"); g.setAttribute("y2", "0");
    var ends = [["0", "0"], ["1", ".20"]];
    for (var i = 0; i < ends.length; i++) {
      var s = svgEl("stop");
      s.setAttribute("offset", ends[i][0]);
      s.setAttribute("stop-color", "#e8513a");
      s.setAttribute("stop-opacity", ends[i][1]);
      g.appendChild(s);
    }
    d.appendChild(g);
    svg.insertBefore(d, svg.firstChild);
  }

  // How far the wash reaches back behind the playhead.
  var TRAIL = 46;

  var cursor = null, wash = null;
  function showCursor(at) {
    if (!at) return;
    if (!cursor || !cursor.isConnected) {
      define();
      // Behind the music, so the noteheads stay black and legible; the
      // playhead itself goes on top, where it is the one thing in front.
      wash = svgEl("rect");
      wash.setAttribute("class", "gscore-wash");
      svg.insertBefore(wash, svg.firstChild);
      cursor = svgEl("rect");
      cursor.setAttribute("class", "gscore-cursor");
      svg.appendChild(cursor);
    }
    var top = at.top - 6, height = at.bottom - at.top + 12;
    var back = Math.min(TRAIL, at.x);
    wash.setAttribute("x", at.x - back);
    wash.setAttribute("y", top);
    wash.setAttribute("width", back);
    wash.setAttribute("height", height);
    cursor.setAttribute("x", at.x - 1);
    cursor.setAttribute("y", top);
    cursor.setAttribute("width", 2);
    cursor.setAttribute("height", height);
  }

  // Keep the music in view.
  //
  // Asked once a bar, and never from inside a frame. What this replaces
  // measured the cursor every frame and scrolled it into view whenever it
  // was not wholly on screen — and the cursor is as tall as the system it
  // is in, so on the ninth symphony it was four thousand nine hundred
  // pixels tall and could not be wholly on screen in any window ever made.
  // The test was therefore always true: a smooth scroll was asked for sixty
  // times a second, each one restarting the one before, each one measuring
  // a document of one and a third million shapes. The page stopped
  // answering and the music never started. Pressing play did nothing at
  // all, which is exactly what it looked like.
  //
  // A system taller than the window cannot be shown whole, so this follows
  // its top rather than trying to centre it, and leaves it alone while any
  // of it is in view.
  var sentTo = null;
  function keepInView() {
    if (!cursor) return;
    var r = cursor.getBoundingClientRect();
    var h = window.innerHeight || 800;
    // Some of the system is on screen: whatever you are looking at, the
    // music is in front of you.
    if (r.top < h * 0.75 && r.bottom > h * 0.1) { sentTo = null; return; }
    var target = Math.max(0, (window.scrollY || 0) + r.top - h * 0.2);
    // Once per move, not once per frame.
    if (sentTo !== null && Math.abs(sentTo - target) < 8) return;
    sentTo = target;
    window.scrollTo({ top: target, behavior: "smooth" });
  }

  function light(now) {
    // Going back means starting the walk again: a recording can be moved to
    // anywhere in itself, and lightAt only ever moves forward.
    if (now < lightNow) lightAt = 0;
    lightNow = now;
    // In time order, so the search does not start at the beginning every
    // time: everything before lightAt has stopped sounding. Reading the
    // whole list on every frame is what made a long score crawl — a hundred
    // and thirty-one thousand notes, sixty times a second — and the work
    // here is now the handful of notes that overlap this instant.
    while (lightAt < notes.length &&
           notes[lightAt].t + notes[lightAt].d <= now) {
      lightAt++;
    }
    // Every note sounding, and not the first one found.
    //
    // A score has as many lines as it has players, and they sound together:
    // that is what a score is. Taking the first and stopping showed the
    // ninth symphony as one blue notehead with fifty-three silent staves
    // under it — the music was all playing, and the page said a soloist was.
    //
    // Everything before lightAt has finished, and the notes are in time
    // order, so this reaches as far as the first note that has not started.
    // What it looks at is the notes overlapping this instant, which is the
    // number of players and not the length of the piece.
    var on = [];
    for (var i = lightAt; i < notes.length && notes[i].t <= now; i++) {
      var n = notes[i];
      if (n.r && now < n.t + n.d) on.push(n.r);
    }

    if (on.length === lit.length) {
      var same = true;
      for (var k = 0; k < on.length; k++) {
        if (on[k] !== lit[k]) { same = false; break; }
      }
      if (same) return;
    }
    // Only what has just come in gets a ring. A note held across three
    // frames was struck once, and should say so once.
    var fresh = [];
    for (var a = 0; a < on.length; a++) {
      if (lit.indexOf(on[a]) < 0) fresh.push(on[a]);
    }
    paint(lit, false);
    paint(on, true);
    // A tutti chord is fifty-four rings at one instant, which is a lot of
    // drawing for something nobody can pick out anyway. Past a certain
    // number the colour says it on its own.
    if (fresh.length <= 24) {
      for (var b = 0; b < fresh.length; b++) ring(fresh[b]);
    }
    lit = on;
  }

  // A ring leaving a notehead, thrown away when it has finished.
  function ring(ref) {
    var found = headsFor(ref);
    for (var i = 0; i < found.length; i++) {
      var box;
      // getBBox throws on anything not being rendered, and a page can be
      // playing with the drawing scrolled out of sight.
      try { box = found[i].getBBox(); } catch (e) { continue; }
      if (!box || !box.height) continue;
      var c = svgEl("circle");
      c.setAttribute("class", "gscore-ping");
      c.setAttribute("cx", box.x + box.width / 2);
      c.setAttribute("cy", box.y + box.height / 2);
      c.setAttribute("r", Math.max(2.2, box.height * 0.45));
      svg.appendChild(c);
      drop(c);
    }
  }

  /*
   * The drawing, when it comes in pieces.
   *
   * A score small enough to carry is written into the page and none of this
   * runs. A long one is not: the ninth symphony inline is seventy-four
   * megabytes and nine hundred thousand shapes, which a browser spends
   * seconds laying out and then has no budget left to animate — the music
   * played with the page frozen behind it, no cursor and no lit heads.
   *
   * So the page carries the drawing's shell and the pieces are fetched as
   * they are wanted: the stretch on screen, and a screenful either way so
   * that scrolling and playing both find the music already there. It is what
   * the editor does against its own server, except that here the answers were
   * written out beforehand and are sitting on the file host as files.
   */
  // Set by startBands, which is called before these lines are reached: the
  // first piece of a long score should be asked for before anything else
  // here happens, and a declaration with a value on it would not have run
  // yet. So they are declared bare and filled in there.
  //
  // bandList is the pieces; onPage maps an index to the <g> it was put in,
  // or to "loading" while it is on its way.
  var bandList, onPage, drawnWide;

  // The stretch of the drawing the window is looking at, in the drawing's own
  // units — which are the units the bands and the bar boxes are in, so
  // nothing has to be converted twice.
  function onScreen() {
    var r = svg.getBoundingClientRect();
    var scale = r.width && drawnWide ? r.width / drawnWide : 1;
    var top = r.top + (window.scrollY || 0);
    var y = window.scrollY || 0, h = window.innerHeight || 800;
    return { y0: (y - top) / scale, y1: (y + h - top) / scale };
  }

  function haveBands(range) {
    if (!bandList || !bandList.length) return;
    // A screenful either way. Less and the music arrives after you have got
    // to where it should be.
    var pad = Math.max(range.y1 - range.y0, 1);
    var keep = {};
    for (var i = 0; i < bandList.length; i++) {
      var b = bandList[i];
      if (b.y1 >= range.y0 - pad && b.y0 <= range.y1 + pad) {
        keep[i] = true;
        bandIn(i);
      }
    }
    // Let go of the rest, or a score played through ends with all of itself
    // on the page and we are back where we started.
    var going = [];
    for (var k in onPage) { if (!keep[k]) going.push(k); }
    for (var g = 0; g < going.length; g++) bandOut(going[g]);
  }

  function bandIn(i) {
    if (onPage[i]) return;
    onPage[i] = "loading";
    fetch(bandList[i].u).then(function (r) {
      if (!r.ok) throw new Error("" + r.status);
      return r.text();
    }).then(function (text) {
      // It may have been let go of while it was in the air.
      if (onPage[i] !== "loading") return;
      // Parsed as a document rather than assigned as innerHTML: an SVG
      // fragment put in by innerHTML is parsed as HTML, where <text> and
      // <line> mean other things or nothing.
      var doc = new DOMParser().parseFromString(
        '<svg xmlns="http://www.w3.org/2000/svg">' + text + '</svg>',
        "image/svg+xml");
      if (doc.getElementsByTagName("parsererror").length) {
        delete onPage[i];
        return;
      }
      var g = svgEl("g");
      g.setAttribute("data-band", String(i));
      // Counted, not drained. importNode copies and leaves the original
      // where it was, so draining the list it is read from never empties it:
      // the page hangs on the first piece it fetches, with the tab pinned at
      // a hundred per cent and nothing drawn.
      var kids = doc.documentElement.childNodes;
      for (var k = 0; k < kids.length; k++) {
        g.appendChild(document.importNode(kids[k], true));
      }
      // Under the playhead and the rings, which are added after it.
      svg.insertBefore(g, cursor || null);
      onPage[i] = g;
      drawingChanged();
    }).catch(function () { delete onPage[i]; });
  }

  function bandOut(i) {
    var g = onPage[i];
    if (g && g !== "loading" && g.parentNode) g.parentNode.removeChild(g);
    delete onPage[i];
    drawingChanged();
  }

  // The heads were filed from the drawing as it stood, and a piece arriving
  // or leaving makes that list wrong — it would point at shapes that are no
  // longer on the page, and miss the ones that now are.
  function drawingChanged() {
    heads = null;
    lit = [];
  }

  function startBands() {
    onPage = {};
    bandList = [];
    var e = document.getElementById("gscore-bands");
    if (e) {
      try { bandList = JSON.parse(e.textContent) || []; } catch (err) { }
    }
    var boxAttr = (svg.getAttribute("viewBox") || "").split(/[\s,]+/);
    drawnWide = parseFloat(boxAttr[2]) || 0;

    if (!bandList.length) return;
    haveBands(onScreen());
    var soon = 0;
    window.addEventListener("scroll", function () {
      if (soon) return;
      soon = setTimeout(function () {
        soon = 0;
        haveBands(onScreen());
      }, 120);
    }, { passive: true });
    window.addEventListener("resize", function () {
      haveBands(onScreen());
    }, { passive: true });
  }

  // Take it off the page when the animation is over — and, for a browser
  // that never says so because it never ran it, shortly afterwards anyway.
  function drop(c) {
    var gone = function () {
      if (c.parentNode) c.parentNode.removeChild(c);
    };
    c.addEventListener("animationend", gone);
    setTimeout(gone, 1200);
  }

  // The heads, filed by the number the engraver gave them.
  //
  // Found in one pass and kept, because an attribute selector is a walk down
  // the whole drawing, and a symphony's drawing is one and a third million
  // shapes. Fifty-four of those walks a frame is not a thing that can be
  // done; one walk, the first time anybody presses play, is. The drawing in
  // a shared page never changes under us, so once is enough.
  var heads = null;
  function headsFor(ref) {
    if (!heads) {
      heads = {};
      var all = svg.querySelectorAll("[data-ref]");
      for (var i = 0; i < all.length; i++) {
        var r = all[i].getAttribute("data-ref");
        if (!heads[r]) heads[r] = [];
        heads[r].push(all[i]);
      }
    }
    return heads[ref] || [];
  }

  // Turn the heads for these notes on or off.
  function paint(refs, want) {
    for (var i = 0; i < refs.length; i++) {
      var found = headsFor(refs[i]);
      for (var k = 0; k < found.length; k++) {
        if (want) found[k].classList.add("sounding");
        else found[k].classList.remove("sounding");
      }
    }
  }

  function say(t) {
    var m = Math.floor(t / 60), s = Math.floor(t % 60);
    clock.textContent = m + ":" + (s < 10 ? "0" : "") + s;
  }

  function follow() {
    var now = clockNow();
    // A recording says when it is over itself. Its length and the score's
    // are worked out by different means and come out a little apart, so
    // trusting the score's would cut the last chord off or leave the cursor
    // sitting past the end waiting for a moment that never arrives.
    if (media ? media.ended : now >= sound.duration + 0.3) { stop(); return; }
    say(Math.max(0, now));
    var pt = placeAt(now);
    showCursor(pt);
    // The view is looked at when the bar changes and not otherwise: reading
    // where things are on the page forces the browser to work out the
    // layout of the whole drawing, which is not a thing to ask of it sixty
    // times a second when the answer can only change once a bar.
    if (pt && pt.bar !== shownBar) {
      shownBar = pt.bar;
      // Have the music that is about to be played, and the system after it.
      // Following the playhead rather than the window because the two part
      // company the moment anybody scrolls away, and the player's business is
      // the music it is playing.
      haveBands({ y0: pt.top, y1: pt.bottom });
      keepInView();
    }
    light(now);
    raf = requestAnimationFrame(follow);
  }

  function play() {
    // Where the walk down the bars and the view are starting from.
    barFrom = 0;
    shownBar = -1;
    sentTo = null;
    if (media) {
      // Nothing to schedule: the recording is the sound and the clock both.
      if (Math.abs(media.currentTime - from) > 0.05) {
        try { media.currentTime = from; } catch (e) {}
      }
      lightAt = 0;
      // Browsers return a promise here and reject it if they would rather
      // not start. Nothing to be done about that, but an unhandled
      // rejection in the console is worth not printing.
      var asked = media.play();
      if (asked && asked.catch) asked.catch(function () {});
      playing = true;
      button.innerHTML = "&#10073;&#10073; Pause";
      raf = requestAnimationFrame(follow);
      return;
    }

    wake();
    master = audio.createGain();
    master.gain.value = 1;
    master.connect(audio.destination);
    voices = [];
    // A moment before the first note, to get it made and scheduled.
    startedAt = audio.currentTime + 0.08;

    // Where the scheduler picks the music up. The notes before this one all
    // began before we came in.
    next = 0;
    while (next < notes.length && notes[next].t < from) next++;
    lightAt = 0;

    // The ones still ringing at that point, taken up for what is left of
    // them. Sorted by when they start, a note that began earlier can still
    // be sounding, so this looks back over all of them rather than stopping
    // at the first that has finished.
    for (var i = 0; i < next; i++) {
      var n = notes[i];
      if (n.t + n.d <= from) continue;
      sound1({ m: n.m, d: n.t + n.d - from, v: n.v, f: n.f }, startedAt);
    }

    playing = true;
    button.innerHTML = "&#10073;&#10073; Pause";
    // The first windowful now, the rest on the timer: waiting 200ms for it
    // would be 200ms of nothing at the top of every piece.
    pump();
    pumping = setInterval(pump, PUMP);
    raf = requestAnimationFrame(follow);
  }

  function silence() {
    if (raf) cancelAnimationFrame(raf);
    raf = 0;
    if (pumping) clearInterval(pumping);
    pumping = 0;
    if (media) { try { media.pause(); } catch (e) {} }
    // Stopping a voice that is scheduled but has not started yet is what
    // keeps a pause silent: the window means there is always some of the
    // music already handed over.
    voices.forEach(function (v) { try { v.o.stop(); } catch (e) {} });
    voices = [];
    playing = false;
    button.innerHTML = "&#9654;&#65038; Play";
  }

  // Pausing keeps its place; stopping at the end goes back to the top.
  function pause() {
    from = Math.max(0, clockNow());
    silence();
  }
  function stop() {
    silence();
    from = 0;
    if (media) { try { media.currentTime = 0; } catch (e) {} }
    say(0);
    light(-1);
    if (cursor) { cursor.remove(); cursor = null; }
    if (wash) { wash.remove(); wash = null; }
  }

  button.addEventListener("click", function () {
    if (playing) { pause(); return; }
    play();
  });
  if (media) {
    media.addEventListener("ended", stop);
    // A recording that cannot be fetched or decoded would otherwise leave
    // the button saying Pause over silence. Say so instead.
    media.addEventListener("error", function () {
      silence();
      button.disabled = true;
      button.innerHTML = "&#9654;&#65038; Play";
      if (clock) clock.textContent = "no recording";
    });
  }
  document.addEventListener("keydown", function (e) {
    if (e.code !== "Space" || e.target.matches("input, textarea")) return;
    e.preventDefault();
    if (playing) { pause(); } else { play(); }
  });
})();
`
