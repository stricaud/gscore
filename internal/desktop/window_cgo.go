//go:build cgo && !nowebview

/*
 * window_cgo.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package desktop

import (
	webview "github.com/webview/webview_go"
)

// Available reports that a native window can be opened.
const Available = true

// Open shows gscore in a native window.
//
// The window is the platform's own web view — WKWebView on macOS, WebView2 on
// Windows, WebKitGTK on Linux — so this is a real application window with no
// browser around it, while the interface inside stays one codebase. The same
// pages are still served over HTTP, which is what keeps scores embeddable in
// a web page.
//
// It must run on the main goroutine: the macOS and GTK event loops both insist
// on it, so main locks the OS thread before calling this.
func Open(url, title string, width, height int) error {
	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle(title)
	w.SetSize(width, height, webview.HintNone)
	w.Navigate(url)
	w.Run()
	return nil
}
