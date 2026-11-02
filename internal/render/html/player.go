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
  var el = document.getElementById("gscore-sound");
  if (!el) return;
  var sound = JSON.parse(el.textContent);
  var svg = document.querySelector("svg.gscore-score");
  var button = document.getElementById("play");
  var clock = document.getElementById("clock");
  if (!sound || !svg || !button) return;

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
    tuned:     { wave: "triangle", cut: 3600, a: 0.002, d: 0.30, s: 0.0,  r: 0.08 }
  };
  // Anything the score does not name is played as a piano, which is what a keyboard with nothing else said about it
  // should sound like. Sustained — an organ — was the old answer, and a
  // score that said nothing about its instrument came out droning.
  function voiceFor(f) { return VOICES[f] || VOICES.plucked; }

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
  var next = 0, lightAt = 0, pumping = 0, lightNow = -1;

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

  // Where the music has got to, in the score's own seconds.
  //
  // The recording's own clock when there is one, and otherwise how far the
  // audio clock has moved since we started. Either way it is the clock of the
  // thing making the sound and not the wall's: a cursor driven by a clock
  // that drifts from what you can hear ends up pointing at the wrong bar,
  // which is the one thing it must never do.
  function clockNow() {
    if (media) return media.currentTime;
    return from + (audio.currentTime - startedAt);
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
    if (v.s === 0) {
      g.exponentialRampToValueAtTime(0.0005, when + Math.min(dur, v.d) + 0.01);
    } else {
      g.linearRampToValueAtTime(peak * v.s, when + v.a + v.d);
      g.setValueAtTime(peak * v.s, Math.max(when + v.a + v.d, when + dur - v.r));
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

  function placeAt(now) {
    var bars = sound.bars || [];
    for (var i = 0; i < bars.length; i++) {
      var bar = bars[i];
      if (now < bar.t || now >= bar.t + bar.d) continue;
      var box = boxOf[bar.i];
      if (!box) return null;
      var part = bar.d > 0 ? (now - bar.t) / bar.d : 0;
      return { x: box.a + (box.b - box.a) * part, top: box.c, bottom: box.d };
    }
    return null;
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
    // Keep it on screen, without fighting a scroll of your own: only when
    // it has gone out of sight.
    var r = cursor.getBoundingClientRect();
    if (r.top < 0 || r.bottom > window.innerHeight) {
      cursor.scrollIntoView({ block: "center", behavior: "smooth" });
    }
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
    showCursor(placeAt(now));
    light(now);
    raf = requestAnimationFrame(follow);
  }

  function play() {
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
