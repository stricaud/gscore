/*
 * Engine.swift
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

import Foundation

/// Engine is the Go core, linked into this application rather than run beside
/// it.
///
/// Everything gscore knows about music is in there: the model, the engraver,
/// the importers, the renderers. This application supplies the things AppKit
/// is for — a menu bar, panels, documents, a window — and shows the editor's
/// own pages inside it.
enum Engine {
    /// Starts the editor and returns where its pages are being served.
    static func start() -> URL? {
        guard let c = GscoreStart() else { return nil }
        defer { free(c) }
        let s = String(cString: c)
        return s.isEmpty ? nil : URL(string: s)
    }

    /// Reads a file into the running editor, answering with what went wrong.
    @discardableResult
    static func open(path: String) -> String? {
        let problem = path.withCString { GscoreOpen(UnsafeMutablePointer(mutating: $0)) }
        guard let problem else { return "the editor is not running" }
        defer { free(problem) }
        let s = String(cString: problem)
        return s.isEmpty ? nil : s
    }
}
