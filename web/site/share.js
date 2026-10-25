/*
 * share.js - putting a score inside a link.
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

/*
 * A tune is small. The eight bars of Ode to Joy are 152 bytes of ABC, and
 * 152 bytes fit in a URL with room to spare — so the link does not point at
 * the music, the link *is* the music.
 *
 * A whole piano rag is not small. The Entertainer is 8 KB of compressed
 * MusicXML, which is 10,500 characters of link: a browser will carry it
 * quite happily, since a fragment never goes anywhere near a server, but
 * nobody wants to paste it into a message. So both are offered and the page
 * says which is which — a short link where the music fits in one, and the
 * built page otherwise, which is a few tens of kilobytes and still needs
 * nothing at the other end.
 *
 * Nothing is uploaded, because there is nowhere to upload it to. The part of
 * a URL after the # is never sent to the server: it stays in the browser.
 * So a shared score has no account behind it, no row in anybody's database,
 * nothing to expire and nothing to take down. Paste the link into a message
 * and the person who opens it has the piece.
 *
 * This file is loaded by the landing page, which writes the links, and by
 * the editor's boot script, which reads them.
 */

(function (root) {
  "use strict";

  // Base64 as a URL may carry it: + and / mean something in a URL, and the =
  // padding is noise once the length is known.
  function toBase64url(bytes) {
    let s = "";
    for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
    return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  }

  function fromBase64url(s) {
    const b = atob(s.replace(/-/g, "+").replace(/_/g, "/"));
    const out = new Uint8Array(b.length);
    for (let i = 0; i < b.length; i++) out[i] = b.charCodeAt(i);
    return out;
  }

  async function squeeze(bytes, how) {
    const stream = new Blob([bytes]).stream().pipeThrough(new how("deflate-raw"));
    return new Uint8Array(await new Response(stream).arrayBuffer());
  }

  // The formats a link may carry, by the one letter that names it in the
  // fragment and the extension gscore opens it as.
  const KINDS = { a: "abc", m: "mxl", x: "musicxml", g: "gscore", s: "mscz" };
  const LETTER = { abc: "a", mxl: "m", musicxml: "x", gscore: "g", mscz: "s" };

  /*
   * encode packs a score into the text that goes after the #.
   *
   * Two letters in front say how it was packed and what it is, so that a
   * link made today is still readable by a browser that cannot deflate and
   * by whatever this grows into later:
   *
   *   ua<base64url>   the bytes as they are, ABC
   *   za<base64url>   deflated first, ABC
   *   zm<base64url>   deflated first, compressed MusicXML
   *
   * A single letter and no kind is ABC, which is what the first links said.
   *
   * Compression is skipped when it does not pay. A few bars of ABC are
   * shorter left alone than deflated and re-encoded, and a .mxl is a zip
   * already — deflating one again buys about four per cent.
   */
  async function encode(data, kind) {
    const bytes =
      typeof data === "string" ? new TextEncoder().encode(data) : data;
    const letter = LETTER[kind || "abc"] || "a";
    if (typeof CompressionStream === "function") {
      try {
        const packed = await squeeze(bytes, CompressionStream);
        if (packed.length < bytes.length) return "z" + letter + toBase64url(packed);
      } catch (_) {
        // An older browser, or a stream it will not do. Send it plain.
      }
    }
    return "u" + letter + toBase64url(bytes);
  }

  // decode turns that text back into a score, or returns null if it is not
  // one. A fragment can be anything — someone's bookmark, a heading anchor —
  // so this never throws.
  async function decode(s) {
    if (!s || s.length < 2) return null;
    const how = s[0];
    // The second letter names the format, when there is one. The first
    // links had none and were always ABC.
    let kind = "abc";
    let rest = s.slice(1);
    if (KINDS[s[1]]) {
      kind = KINDS[s[1]];
      rest = s.slice(2);
    }
    let bytes;
    try {
      bytes = fromBase64url(rest);
    } catch (_) {
      return null;
    }
    if (how === "z") {
      if (typeof DecompressionStream !== "function") return null;
      try {
        const stream = new Blob([bytes]).stream()
          .pipeThrough(new DecompressionStream("deflate-raw"));
        bytes = new Uint8Array(await new Response(stream).arrayBuffer());
      } catch (_) {
        return null;
      }
    } else if (how !== "u") {
      return null;
    }
    return { kind, bytes };
  }

  // fromLocation is the score in the address bar, if there is one.
  async function fromLocation() {
    const m = /[#&]s=([A-Za-z0-9\-_]+)/.exec(root.location.hash || "");
    return m ? decode(m[1]) : null;
  }

  root.gscoreShare = { encode, decode, fromLocation };
})(window);
