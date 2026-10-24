/*
 * AppDelegate.swift
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

import AppKit
import WebKit
import UniformTypeIdentifiers

/// The application.
///
/// It owns the window, the menu bar and the panels; the editor inside it is
/// the same one the web build shows, served from the Go core linked into this
/// binary. The split is deliberate: everything about music stays in Go and is
/// tested headlessly, and everything about being a Mac application stays
/// here, where AppKit already knows the answers.
final class AppDelegate: NSObject, NSApplicationDelegate, WKNavigationDelegate {
    private var window: NSWindow!
    private var web: WKWebView!
    private var ready = false
    /// A file the Finder handed us before the editor had started. Opening it
    /// has to wait, and dropping it would mean double-clicking a score
    /// sometimes opened an empty one.
    private var pending: String?

    func applicationDidFinishLaunching(_ note: Notification) {
        guard let url = Engine.start() else {
            fail("Gscore could not start its editor.",
                 "It could not listen on a port of its own, which usually "
                 + "means something is badly wrong with the network stack.")
            return
        }

        let config = WKWebViewConfiguration()
        // The editor saves through its own panels and keeps nothing in the
        // page, so a fresh store every launch is one less thing to go stale.
        config.websiteDataStore = .nonPersistent()

        // Tell the page it is inside this application, before it draws
        // anything. It carries a menu bar of its own for the browser and the
        // plain window, and in here that is the second menu bar on the
        // screen — the same commands twice, one of them in the wrong place
        // for a Mac. It hides its own and uses ours.
        config.userContentController.addUserScript(WKUserScript(
            source: "window.gscoreNativeMenu = true;",
            injectionTime: .atDocumentStart,
            forMainFrameOnly: true))
        web = WKWebView(frame: .zero, configuration: config)
        web.navigationDelegate = self
        web.allowsBackForwardNavigationGestures = false
        web.setValue(false, forKey: "drawsBackground")

        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1280, height: 820),
            styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
            backing: .buffered, defer: false)
        window.title = "Gscore"
        window.minSize = NSSize(width: 720, height: 480)
        window.contentView = web
        window.setFrameAutosaveName("gscore.main")
        window.center()
        window.makeKeyAndOrderFront(nil)

        Menus.install(target: self)
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)

        web.load(URLRequest(url: url))
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ app: NSApplication) -> Bool {
        true
    }

    // MARK: - Documents

    /// The Finder opening a score on us: a double-click, a drop on the icon,
    /// or the file given on the command line.
    func application(_ app: NSApplication, open urls: [URL]) {
        guard let url = urls.first else { return }
        open(path: url.path)
    }

    func application(_ sender: NSApplication, openFile filename: String) -> Bool {
        open(path: filename)
        return true
    }

    @objc func openDocument(_ sender: Any?) {
        let panel = NSOpenPanel()
        panel.title = "Open score"
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.allowedContentTypes = Self.readableTypes
        panel.beginSheetModal(for: window) { answer in
            guard answer == .OK, let url = panel.url else { return }
            self.open(path: url.path)
        }
    }

    /// Everything the importers read. The list is here rather than guessed at
    /// from the plist, so the panel offers exactly what will open.
    private static var readableTypes: [UTType] {
        ["gscore", "abc", "mscz", "mscx", "musicxml", "mxl", "xml",
         "nwc", "nwctxt"].compactMap { UTType(filenameExtension: $0) }
    }

    private func open(path: String) {
        guard ready else {
            pending = path
            return
        }
        if let problem = Engine.open(path: path) {
            fail("Gscore could not open that file.", problem)
            return
        }
        window.title = (path as NSString).lastPathComponent + " — Gscore"
        window.representedFilename = path
        NSDocumentController.shared.noteNewRecentDocumentURL(URL(fileURLWithPath: path))
        // The score changed underneath the page, so the page is told to go
        // and look again rather than being reloaded: a reload would lose the
        // zoom, the selection and where you were on the page.
        run("reopen-file")
    }

    // MARK: - Commands

    /// Every menu item that is not a system one comes through here and is
    /// sent to the editor by name.
    @objc func runCommand(_ sender: NSMenuItem) {
        guard let name = sender.representedObject as? String else { return }
        run(name)
    }

    private func run(_ command: String) {
        guard ready else { return }
        let quoted = command.replacingOccurrences(of: "\\", with: "\\\\")
                            .replacingOccurrences(of: "'", with: "\\'")
        web.evaluateJavaScript("window.gscoreCommand && window.gscoreCommand('\(quoted)')")
    }

    func webView(_ view: WKWebView, didFinish navigation: WKNavigation!) {
        ready = true
        rebuildMenus()
        if let path = pending {
            pending = nil
            open(path: path)
        }
    }

    /// Ask the editor what it can do, and make the menu bar out of the
    /// answer.
    ///
    /// The bar installed at launch is a skeleton — enough to exist before
    /// the page has loaded. The real list of commands lives in the page's
    /// own menus and grows with every feature; read once, here, it cannot
    /// fall behind the way a second list written by hand does.
    private func rebuildMenus(tries: Int = 25) {
        // The page finishing loading is not the editor being ready: it sets
        // itself up asynchronously — fetching the score, the layout and the
        // symbols — and names its commands at the end of that. So this asks
        // again until there is an answer rather than once into the gap.
        let js = "window.gscoreMenuTree ? JSON.stringify(window.gscoreMenuTree()) : ''"
        web.evaluateJavaScript(js) { [weak self] result, _ in
            guard let self else { return }
            if let text = result as? String, !text.isEmpty,
               let data = text.data(using: .utf8),
               let tree = try? JSONSerialization.jsonObject(with: data)
                   as? [[String: Any]], !tree.isEmpty {
                Menus.rebuild(from: tree, target: self)
                return
            }
            guard tries > 1 else { return } // leave the bar it was given
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) {
                self.rebuildMenus(tries: tries - 1)
            }
        }
    }

    private func fail(_ message: String, _ detail: String) {
        let alert = NSAlert()
        alert.alertStyle = .critical
        alert.messageText = message
        alert.informativeText = detail
        alert.runModal()
    }
}
