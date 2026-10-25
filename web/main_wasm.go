//go:build js && wasm

/*
 * main_wasm.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Command gscore-wasm is gscore running in a browser, with no server at all.
//
// The whole program — model, importers, engraver, renderer — compiles to
// WebAssembly, and the same http.Handler that serves the desktop application
// answers requests here through an in-memory recorder. Nothing is duplicated
// and nothing is reimplemented in JavaScript: a score opened on a web page
// goes through exactly the code that opens it on the desktop.
//
// This is what makes gscore publishable as a static site, and it is the same
// capability that lets a score be embedded in someone else's page.
package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"syscall/js"

	"github.com/stricaud/gscore/internal/app"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/version"

	// The importers register themselves, exactly as in the desktop build.
	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
	_ "github.com/stricaud/gscore/internal/format/lilypond"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
	_ "github.com/stricaud/gscore/internal/format/nwc"
)

func main() {
	a := app.New(model.NewScore(), "")
	// The core is the page here, which is what lets it play through a
	// soundfont: there is no server in between and no stranger to render
	// for.
	a.InBrowser()
	mux := http.NewServeMux()
	a.APIRoutes(mux)

	js.Global().Set("gscoreFetch", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return serve(mux, args)
	}))
	js.Global().Set("gscoreVersion", js.ValueOf(version.Version))

	// Tell the page the core is up, then stay alive: a Go program that
	// returns from main takes its exported functions with it.
	if ready := js.Global().Get("gscoreReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

// serve answers one request. It takes a method, a URL and an optional body,
// and returns the status, content type and bytes.
func serve(mux *http.ServeMux, args []js.Value) any {
	method := args[0].String()
	url := args[1].String()

	var body io.Reader
	if len(args) > 2 && args[2].Truthy() {
		n := args[2].Get("length").Int()
		buf := make([]byte, n)
		js.CopyBytesToGo(buf, args[2])
		body = bytes.NewReader(buf)
	}

	req := httptest.NewRequest(method, url, body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()

	data, _ := io.ReadAll(res.Body)
	arr := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(arr, data)

	// Every header, not only the type. One of them is how the page is told
	// that the music it is being handed a piece at a time has run out, and
	// an answer that drops it is an answer the page cannot act on.
	headers := js.Global().Get("Object").New()
	for k := range res.Header {
		headers.Set(k, res.Header.Get(k))
	}

	out := js.Global().Get("Object").New()
	out.Set("status", res.StatusCode)
	out.Set("contentType", res.Header.Get("Content-Type"))
	out.Set("headers", headers)
	out.Set("body", arr)
	return out
}
