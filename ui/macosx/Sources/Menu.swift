/*
 * Menu.swift
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

import AppKit

/// The menu bar.
///
/// Every item but the system ones carries the name of a command the editor
/// already has — "save-pdf", "set-key", "beam-join" — and does nothing but
/// send it. The web interface names its commands the same way, in
/// `data-action` attributes, and one dispatcher in app.js answers both. So
/// the menu bar and the toolbar cannot drift apart: there is one list of
/// things gscore can do, and two ways of reaching it.
///
/// The system items — Hide, Quit, Minimise, Full Screen — are left to AppKit,
/// which already knows what they mean and what they are called in every
/// language the Mac speaks.
enum Menus {
    static func install(target: AnyObject) {
        let bar = NSMenu()
        bar.addItem(appMenu())
        bar.addItem(fileMenu(target))
        bar.addItem(editMenu(target))
        bar.addItem(viewMenu(target))
        bar.addItem(scoreMenu(target))
        bar.addItem(playMenu(target))
        bar.addItem(windowMenu())
        bar.addItem(helpMenu(target))
        NSApp.mainMenu = bar
    }

    /// item makes one entry that sends a command to the editor.
    private static func item(_ title: String, _ action: String,
                             _ key: String = "", _ mods: NSEvent.ModifierFlags = .command,
                             icon: String = "", target: AnyObject) -> NSMenuItem {
        let it = NSMenuItem(title: title,
                            action: #selector(AppDelegate.runCommand(_:)), keyEquivalent: key)
        it.keyEquivalentModifierMask = mods
        it.representedObject = action
        it.target = target
        // The picture beside the words. It is the page's own — the same
        // table, the same music font — drawn here so that it is in the
        // colours the system is using rather than the ones the page had.
        //
        // An entry with no picture of its own gets the room one would take,
        // so that the words line up down the menu. Before the font has
        // arrived nothing gets either, which is a menu bar without pictures
        // rather than a menu bar indented for pictures that never come.
        if Glyphs.ready {
            it.image = Glyphs.image(icon) ?? Glyphs.blank
        }
        return it
    }

    private static func submenu(_ title: String, _ items: [NSMenuItem]) -> NSMenuItem {
        let holder = NSMenuItem(title: title, action: nil, keyEquivalent: "")
        let menu = NSMenu(title: title)
        items.forEach { menu.addItem($0) }
        holder.submenu = menu
        return holder
    }

    private static func appMenu() -> NSMenuItem {
        let name = ProcessInfo.processInfo.processName
        return submenu(name, [
            NSMenuItem(title: "About \(name)",
                       action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)),
                       keyEquivalent: ""),
            .separator(),
            NSMenuItem(title: "Hide \(name)",
                       action: #selector(NSApplication.hide(_:)), keyEquivalent: "h"),
            {
                let it = NSMenuItem(title: "Hide Others",
                                    action: #selector(NSApplication.hideOtherApplications(_:)),
                                    keyEquivalent: "h")
                it.keyEquivalentModifierMask = [.command, .option]
                return it
            }(),
            NSMenuItem(title: "Show All",
                       action: #selector(NSApplication.unhideAllApplications(_:)),
                       keyEquivalent: ""),
            .separator(),
            NSMenuItem(title: "Quit \(name)",
                       action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"),
        ])
    }

    private static func fileMenu(_ t: AnyObject) -> NSMenuItem {
        submenu("File", [
            item("New", "new", "n", target: t),
            {
                let it = NSMenuItem(title: "Open…",
                                    action: #selector(AppDelegate.openDocument(_:)),
                                    keyEquivalent: "o")
                it.target = t
                return it
            }(),
            .separator(),
            // Saving always asks where, so there is no second entry that
            // would do the same thing under a different name. Saving back
            // over the file that was opened, without asking, is worth having
            // and is not here yet.
            item("Save…", "save", "s", target: t),
            // A saved score carries a PDF of itself unless this is turned
            // off: renamed .pdf, the same file opens in anything that reads
            // one.
            item("…with a PDF Inside It", "save-with-pdf", "", [], target: t),
            .separator(),
            item("Export PDF…", "save-pdf", "p", [.command, .shift], target: t),
            item("Export SVG…", "save-svg", "e", [.command, .shift], target: t),
            // One file anybody can open: the music, the font and the player
            // all written into it, so it needs nothing else to work.
            item("Export Web Page…", "save-page", "h", [.command, .shift], target: t),
            .separator(),
            item("Print…", "print", "p", target: t),
            .separator(),
            {
                let it = NSMenuItem(title: "Close Window",
                                    action: #selector(NSWindow.performClose(_:)),
                                    keyEquivalent: "w")
                return it
            }(),
        ])
    }

    private static func editMenu(_ t: AnyObject) -> NSMenuItem {
        submenu("Edit", [
            item("Undo", "undo", "z", target: t),
            item("Redo", "redo", "z", [.command, .shift], target: t),
            .separator(),
            item("Cut", "cut", "x", target: t),
            item("Copy", "copy", "c", target: t),
            item("Paste", "paste", "v", target: t),
            item("Delete", "delete", "\u{8}", [], target: t),
            .separator(),
            item("Go to Measure…", "jump", "g", target: t),
            item("Add Measure", "add-measure", "m", target: t),
        ])
    }

    private static func viewMenu(_ t: AnyObject) -> NSMenuItem {
        submenu("View", [
            item("Zoom In", "zoom-in", "+", target: t),
            item("Zoom Out", "zoom-out", "-", target: t),
            item("Actual Size", "zoom-reset", "0", target: t),
            .separator(),
            item("Continuous Strip", "mode-strip", "", [], target: t),
            item("Pages", "mode-page", "", [], target: t),
        ])
    }

    private static func scoreMenu(_ t: AnyObject) -> NSMenuItem {
        submenu("Score", [
            item("Title and Composer…", "set-header", "", [], target: t),
            item("Tempo…", "set-tempo", "", [], target: t),
            item("Key Signature…", "set-key", "", [], target: t),
            item("Time Signature…", "set-time", "", [], target: t),
            .separator(),
            item("Instrument…", "set-instrument", "", [], target: t),
            item("Add Staff", "add-staff", "", [], target: t),
            item("Add Grand Staff", "add-grand-staff", "", [], target: t),
            item("Remove Staff", "remove-staff", "", [], target: t),
            .separator(),
            item("Lyrics…", "set-lyrics", "l", target: t),
            item("Slur", "slur", "", [], target: t),
            item("Tie", "tie", "", [], target: t),
            item("Join Beam", "beam-join", "b", target: t),
        ])
    }

    private static func playMenu(_ t: AnyObject) -> NSMenuItem {
        submenu("Play", [
            item("Play / Pause", "play", " ", [.command], target: t),
            item("Play from the Beginning", "play-from-start", " ",
                 [.command, .shift], target: t),
            // Pausing keeps its place in the music, so there has to be a way
            // of saying "and go back to the top" that is not playing from the
            // top. Escape does it in the page; a menu bar needs an item.
            item("Stop", "stop", ".", [.command], target: t),
        ])
    }

    private static func windowMenu() -> NSMenuItem {
        let holder = NSMenuItem(title: "Window", action: nil, keyEquivalent: "")
        let menu = NSMenu(title: "Window")
        menu.addItem(NSMenuItem(title: "Minimise",
                                action: #selector(NSWindow.performMiniaturize(_:)),
                                keyEquivalent: "m"))
        menu.addItem(NSMenuItem(title: "Zoom",
                                action: #selector(NSWindow.performZoom(_:)),
                                keyEquivalent: ""))
        holder.submenu = menu
        // AppKit fills this one in with the open windows by itself.
        NSApp.windowsMenu = menu
        return holder
    }

    private static func helpMenu(_ t: AnyObject) -> NSMenuItem {
        let holder = submenu("Help", [item("Gscore Help", "about", "?", target: t)])
        NSApp.helpMenu = holder.submenu
        return holder
    }
}

/// Building the bar from the page's own menus.
///
/// The hand-written menus above are a skeleton: enough to have a bar before
/// the editor has loaded, and enough for the system items. The real list of
/// what gscore can do lives in one place — the page's menus — and it grows
/// every time a feature does. Mirroring it by hand means a bar that is right
/// on the day it is written and wrong a week later, which is exactly how a
/// hundred commands came to be missing from it.
///
/// So once the page is up, the bar is rebuilt from what the page says it
/// has. The shortcuts stay here, because what a key means on a Mac is a Mac
/// question: the page's own are bare letters, which a menu bar must not
/// claim.
extension Menus {
    /// The shortcuts the Mac bar advertises, by command name.
    static let keys: [String: (String, NSEvent.ModifierFlags)] = [
        "new": ("n", .command),
        "save": ("s", .command),
        "save-pdf": ("p", [.command, .shift]),
        "save-svg": ("e", [.command, .shift]),
        "save-page": ("h", [.command, .shift]),
        "print": ("p", .command),
        "undo": ("z", .command),
        "redo": ("z", [.command, .shift]),
        "jump": ("g", .command),
        "add-measure": ("m", .command),
        "play": (" ", .command),
        "play-from-start": (" ", [.command, .shift]),
        "stop": (".", .command),
        "zoom-in": ("+", .command),
        "zoom-out": ("-", .command),
        "zoom-reset": ("0", .command),
    ]

    /// rebuild replaces the bar with one made from the page's menus.
    ///
    /// The application, Window and Help menus are the system's and are kept
    /// as they are; everything between them comes from the page.
    static func rebuild(from tree: [[String: Any]], target: AnyObject) {
        guard !tree.isEmpty else { return }
        let bar = NSMenu()
        bar.addItem(appMenu())
        var placedWindow = false
        for menu in tree {
            guard let title = menu["label"] as? String, !title.isEmpty,
                  let items = menu["items"] as? [[String: Any]] else { continue }
            // Window goes where a Mac puts it: before Help and after
            // everything else.
            if title.caseInsensitiveCompare("Help") == .orderedSame,
               !placedWindow {
                bar.addItem(windowMenu())
                placedWindow = true
            }
            let holder = NSMenuItem(title: title, action: nil, keyEquivalent: "")
            let sub = NSMenu(title: title)
            fill(sub, items, target: target)
            holder.submenu = sub
            bar.addItem(holder)
        }
        if !placedWindow {
            bar.addItem(windowMenu())
        }
        NSApp.mainMenu = bar
    }

    private static func fill(_ menu: NSMenu, _ items: [[String: Any]],
                             target: AnyObject) {
        // Not "item": that is the maker below, and a loop variable of the
        // same name hides it.
        for entry in items {
            switch entry["kind"] as? String {
            case "separator":
                menu.addItem(.separator())
            case "submenu":
                guard let label = entry["label"] as? String,
                      let kids = entry["items"] as? [[String: Any]] else { continue }
                let holder = NSMenuItem(title: label, action: nil, keyEquivalent: "")
                let sub = NSMenu(title: label)
                fill(sub, kids, target: target)
                holder.submenu = sub
                menu.addItem(holder)
            default:
                guard let label = entry["label"] as? String,
                      let action = entry["action"] as? String,
                      !label.isEmpty else { continue }
                let key = keys[action]
                let made = item(label, action, key?.0 ?? "", key?.1 ?? [],
                                icon: entry["icon"] as? String ?? "",
                                target: target)
                if let enabled = entry["enabled"] as? Bool, !enabled {
                    made.isEnabled = false
                }
                menu.addItem(made)
            }
        }
    }
}
