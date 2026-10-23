/*
 * gscore - a musical score editor
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Starting gscore in a browser with no server behind it.
 *
 * The interface talks to gscore over HTTP, and that does not change here: the
 * requests are answered by the same Go handlers, compiled to WebAssembly and
 * reached through a replacement for fetch. app.js is untouched and does not
 * know which it is talking to.
 */

(function () {
  // Running as a page, the core is a WebAssembly module that takes a moment
  // to start. app.js runs as soon as it is parsed and immediately asks the
  // core for the score — so without something to wait on, that request goes
  // out to the network, the host answers 404, and the interface comes up
  // empty with no sign of why. This promise is what app.js waits on; the
  // desktop build has no boot.js, finds no promise, and carries straight on.
  let coreReady;
  window.gscoreCoreReady = new Promise((resolve) => {
    coreReady = resolve;
  });

  // Every path the interface asks the core for. Anything missing here goes
  // to the network instead, where there is nothing to answer it.
  const HANDLED = [
    /^\/api\//,
    /^\/score\.svg/,
    /^\/keysig\.svg/,
    /^\/timesig\.svg/,
    /^\/symbol\.png/,
    /^\/font\.otf/,
    /^\/print/,
  ];

  function isOurs(url) {
    return HANDLED.some((re) => re.test(url));
  }

  const realFetch = window.fetch.bind(window);

  window.fetch = async function (input, init) {
    const url = typeof input === "string" ? input : input.url;
    const path = url.startsWith("http") ? new URL(url).pathname + new URL(url).search : url;

    if (!isOurs(path) || typeof window.gscoreFetch !== "function") {
      return realFetch(input, init);
    }

    const method = (init && init.method) || "GET";
    let body = null;
    if (init && init.body) {
      body =
        init.body instanceof ArrayBuffer
          ? new Uint8Array(init.body)
          : new Uint8Array(await new Response(init.body).arrayBuffer());
    }

    const res = window.gscoreFetch(method, path, body);
    return new Response(res.body, {
      status: res.status,
      headers: { "Content-Type": res.contentType || "application/octet-stream" },
    });
  };

  // The desktop build opens files through the system dialog; a page cannot, so
  // the hidden file input does the work and app.js already falls back to it.
  window.gscoreReady = function () {
    document.body.classList.add("wasm");
    const el = document.getElementById("status");
    if (el) el.textContent = "Ready";
    coreReady();
  };

  async function start() {
    const el = document.getElementById("status");
    if (el) el.textContent = "Loading gscore…";

    const go = new Go();
    const resp = await realFetch("gscore.wasm");
    // Streaming compilation avoids holding the whole module twice, but not
    // every host serves wasm with the right type, so fall back.
    let result;
    try {
      result = await WebAssembly.instantiateStreaming(resp.clone(), go.importObject);
    } catch (_) {
      const bytes = await resp.arrayBuffer();
      result = await WebAssembly.instantiate(bytes, go.importObject);
    }
    go.run(result.instance);
  }

  window.addEventListener("DOMContentLoaded", () => {
    start().catch((err) => {
      const el = document.getElementById("status");
      if (el) el.textContent = "Could not start gscore: " + err;
      console.error(err);
      // Let the interface come up anyway and say so, rather than hang.
      coreReady();
    });
  });
})();
