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
  rect.gscore-cursor { fill: #2d7ff9; opacity: .22; pointer-events: none; }
  .gscore-score .sounding { fill: #2d7ff9; }
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

  var audio = null, master = null, voices = [];
  var playing = false, startedAt = 0, from = 0, raf = 0, lit = null;

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
    voices.push(osc);
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

  var cursor = null;
  function showCursor(at) {
    if (!at) return;
    if (!cursor || !cursor.isConnected) {
      cursor = document.createElementNS("http://www.w3.org/2000/svg", "rect");
      cursor.setAttribute("class", "gscore-cursor");
      svg.appendChild(cursor);
    }
    cursor.setAttribute("x", at.x - 1.5);
    cursor.setAttribute("y", at.top - 6);
    cursor.setAttribute("width", 3);
    cursor.setAttribute("height", at.bottom - at.top + 12);
    // Keep it on screen, without fighting a scroll of your own: only when
    // it has gone out of sight.
    var r = cursor.getBoundingClientRect();
    if (r.top < 0 || r.bottom > window.innerHeight) {
      cursor.scrollIntoView({ block: "center", behavior: "smooth" });
    }
  }

  function light(now) {
    var on = null;
    var notes = sound.notes || [];
    for (var i = 0; i < notes.length; i++) {
      var n = notes[i];
      if (n.r && now >= n.t && now < n.t + n.d) { on = n.r; break; }
    }
    if (on === lit) return;
    if (lit !== null) {
      svg.querySelectorAll('[data-ref="' + lit + '"]').forEach(function (e) {
        e.classList.remove("sounding");
      });
    }
    if (on !== null) {
      svg.querySelectorAll('[data-ref="' + on + '"]').forEach(function (e) {
        e.classList.add("sounding");
      });
    }
    lit = on;
  }

  function say(t) {
    var m = Math.floor(t / 60), s = Math.floor(t % 60);
    clock.textContent = m + ":" + (s < 10 ? "0" : "") + s;
  }

  function follow() {
    var now = from + (audio.currentTime - startedAt);
    if (now >= sound.duration + 0.3) { stop(); return; }
    say(Math.max(0, now));
    showCursor(placeAt(now));
    light(now);
    raf = requestAnimationFrame(follow);
  }

  function play() {
    wake();
    master = audio.createGain();
    master.gain.value = 1;
    master.connect(audio.destination);
    voices = [];
    var lead = 0.08;
    var t0 = audio.currentTime + lead;
    startedAt = t0;
    (sound.notes || []).forEach(function (n) {
      var end = n.t + n.d;
      if (end <= from) return;
      if (n.t >= from) { sound1(n, t0 + n.t - from); return; }
      // Already ringing when we come in: take it up for what is left.
      sound1({ m: n.m, d: end - from, v: n.v, f: n.f }, t0);
    });
    playing = true;
    button.innerHTML = "&#10073;&#10073; Pause";
    raf = requestAnimationFrame(follow);
  }

  function silence() {
    if (raf) cancelAnimationFrame(raf);
    raf = 0;
    voices.forEach(function (v) { try { v.stop(); } catch (e) {} });
    voices = [];
    playing = false;
    button.innerHTML = "&#9654;&#65038; Play";
  }

  // Pausing keeps its place; stopping at the end goes back to the top.
  function pause() {
    from = Math.max(0, from + (audio.currentTime - startedAt));
    silence();
  }
  function stop() {
    silence();
    from = 0;
    say(0);
    light(-1);
    if (cursor) { cursor.remove(); cursor = null; }
  }

  button.addEventListener("click", function () {
    if (playing) { pause(); return; }
    play();
  });
  document.addEventListener("keydown", function (e) {
    if (e.code !== "Space" || e.target.matches("input, textarea")) return;
    e.preventDefault();
    if (playing) { pause(); } else { play(); }
  });
})();
`
