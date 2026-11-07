/*
 * Printing.swift
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

import AppKit
import PDFKit

/// Printing, which a Mac application does itself.
///
/// The editor is a web page, and a web view has no print machinery behind it:
/// window.print() does nothing and window.open answers with nothing, so the
/// page cannot put a print panel on the screen from in here. What it could do
/// was ask the system to open the same pages in a real browser and let that
/// print them — which is not printing from this application, and left the
/// person looking at an empty preview in a window they did not ask for.
///
/// So the page says what it wants printed and this asks the core for it as a
/// PDF: the same file the Export PDF entry writes, with the pages already cut,
/// the margins already placed and the type size already chosen. The panel
/// shows the sheet that will come out of the printer, and printing and
/// exporting are the same document by construction rather than by two pieces
/// of code agreeing.
enum Printing {
    /// Shows the print panel for a document the editor serves at `path`.
    ///
    /// The work is a fetch, so it is done off the main thread and the panel is
    /// put up when the pages are in hand. A score of any size takes long
    /// enough to engrave that doing it in front of the run loop would freeze
    /// the window for as long as it took.
    static func show(path: String, base: URL, over window: NSWindow?) {
        guard let url = URL(string: path, relativeTo: base) else {
            complain("Gscore could not work out what to print.")
            return
        }
        URLSession.shared.dataTask(with: url) { data, response, error in
            let code = (response as? HTTPURLResponse)?.statusCode ?? 0
            guard error == nil, code < 400, let data,
                  let doc = PDFDocument(data: data), doc.pageCount > 0 else {
                DispatchQueue.main.async {
                    complain("Gscore could not make the pages to print.")
                }
                return
            }
            DispatchQueue.main.async { panel(for: doc, over: window) }
        }.resume()
    }

    /// Puts the pages in front of the person with the system's own panel,
    /// where the printer, the copies, the range and "Save as PDF" live.
    private static func panel(for doc: PDFDocument, over window: NSWindow?) {
        let info = (NSPrintInfo.shared.copy() as? NSPrintInfo) ?? NSPrintInfo.shared
        // The paper the score was engraved for, rather than whatever was
        // printed last: the pages were cut to a sheet of one size, and a
        // panel that starts on another either scales them or loses the foot
        // of every page.
        //
        // The way round comes first, because setting it is what swaps the
        // paper's sides; set afterwards it would undo the size just given.
        if let first = doc.page(at: 0) {
            let box = first.bounds(for: .mediaBox)
            info.orientation = box.width > box.height ? .landscape : .portrait
            info.paperSize = box.size
        }
        // And no margins here, because the margins are in the pages. The
        // engraver placed them; taking the printer's as well would inset the
        // music twice and shrink it to fit what was left.
        info.topMargin = 0
        info.bottomMargin = 0
        info.leftMargin = 0
        info.rightMargin = 0
        info.isHorizontallyCentered = true
        info.isVerticallyCentered = true

        guard let op = doc.printOperation(for: info, scalingMode: .pageScaleNone,
                                          autoRotate: false) else {
            complain("Gscore could not open the print panel.")
            return
        }
        op.showsPrintPanel = true
        op.showsProgressPanel = true
        if let window {
            op.runModal(for: window, delegate: nil, didRun: nil, contextInfo: nil)
        } else {
            op.run()
        }
    }

    /// Says what went wrong, in the one place that can: a web page told to
    /// print and never heard from again has nothing to show for it.
    private static func complain(_ message: String) {
        let alert = NSAlert()
        alert.messageText = message
        alert.informativeText = "Exporting a PDF from the File menu writes the "
            + "same pages, and will say more about what failed."
        alert.alertStyle = .warning
        alert.runModal()
    }
}
