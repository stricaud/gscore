/*
 * Glyphs.swift
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

import AppKit
import CoreText

/// say puts a line on the error stream.
///
/// Not NSLog: that goes to the system's log, which is one more place to go
/// and look, and from a terminal it may show nothing at all. Anybody
/// running the application to find out why it is not drawing something
/// wants the answer in front of them.
func say(_ message: String) {
    FileHandle.standardError.write(Data(("gscore: " + message + "\n").utf8))
}

/// The little pictures beside the menu entries.
///
/// gscore 0.0.9 put one next to every entry that writes something, and the
/// reason they work is the colour: the black is the note the entry is about
/// and the red is what the entry does to it, so the eye finds the entry it
/// wants without reading a word. The web interface draws them from the music
/// font; this draws the same ones, from the same font and the same table.
///
/// The table is not copied here. Each menu entry arrives from the page
/// carrying the codepoints its picture is made of — one character for a mark
/// drawn in the colour of what it writes, two for a note with a mark on it —
/// so there is one list of what each command looks like and two places that
/// draw from it. Copying it would be a list that is right on the day it is
/// written and wrong a week later.
///
/// Nothing of this reaches the screen on macOS 27, and the fault is not
/// here. Measured on 2026-11-01: the font arrives, every codepoint in the
/// table resolves to a glyph with ink, an image is made for each of the
/// sixty-four entries that asks for one, and all sixty-four are on the
/// menu bar when it is read back — and none of them is drawn. A stock
/// system symbol put on the same entry is not drawn either, while the
/// three entries macOS inserts into the Edit menu itself keep their
/// icons. So this system does not draw an application's own menu-item
/// pictures, and no change here will make it.
///
/// Left in place rather than deleted: it is correct, it costs one fetch
/// at launch, and an older or newer system may well draw them. Anybody
/// finding the menus bare should read this paragraph before reading the
/// code.
///
/// Drawn rather than received as a picture, because a menu is the system's
/// and not ours: the words beside these are set in whatever the system is
/// using, in whatever colour it has decided on, and change when somebody
/// turns the lights off. An image sent over from the page would be the
/// colours the page had when it was asked.
enum Glyphs {
    /// The music font, once it has been fetched and registered.
    private static var font: CGFont?

    /// What a glyph is drawn at, in points, against menu text of about 13.
    ///
    /// Larger than the words, because a notehead at the size of the letters
    /// beside it is a dot. The box below is what keeps it from crowding
    /// them.
    private static let size: CGFloat = 21

    /// The box a picture is fitted into, in points.
    ///
    /// Wider than it is tall, because a barline is a tall thin thing and a
    /// note with a mark under it is taller still, and both have to come
    /// down to the height of a row. The same proportions the page uses, so
    /// that the two menus look like the same program.
    private static let box = NSSize(width: 24, height: 21)

    /// ready says whether there is a font to draw pictures with.
    static var ready: Bool { font != nil }

    /// blank is the room a picture takes, for an entry that has none.
    ///
    /// Every entry gets one or the other, so that the words line up: a menu
    /// where half the labels are indented past a picture and half start at
    /// the edge is harder to read than one with no pictures at all.
    static let blank: NSImage = {
        let image = NSImage(size: box, flipped: false) { _ in true }
        image.isTemplate = false
        return image
    }()

    /// load fetches the music font from the editor.
    ///
    /// From the editor rather than from the bundle: it is the same file the
    /// page draws with, served by the core that is already running, so the
    /// menus and the music can never be set in two different versions of
    /// Bravura. `then` is called on the main queue once it is there, and not
    /// at all if it is not — a menu bar with no pictures in it is a menu bar.
    ///
    /// Not installed into the system's list of fonts, only held. Nothing
    /// here ever asks for it by name: the glyphs are drawn through the face
    /// itself, and registering a font to draw with it would be putting
    /// Bravura in every other application's font menu for as long as gscore
    /// is open.
    static func load(from base: URL, then: @escaping () -> Void) {
        guard let url = URL(string: "font.otf", relativeTo: base) else {
            say("no address for the music font under \(base.absoluteString)")
            return
        }
        URLSession.shared.dataTask(with: url) { data, response, error in
            // Said out loud, because every way this can fail ends in a menu
            // bar with no pictures in it and nothing anywhere saying why.
            // Three silent guards is an afternoon spent reading the drawing
            // code, which was never the part that was wrong.
            if let error {
                say("the music font did not arrive from \(url.absoluteString): "
                     + error.localizedDescription)
                return
            }
            if let http = response as? HTTPURLResponse, http.statusCode != 200 {
                say("\(url.absoluteString) answered \(http.statusCode) for the music font")
                return
            }
            guard let data, !data.isEmpty else {
                say("the music font came back empty from \(url.absoluteString)")
                return
            }
            guard let provider = CGDataProvider(data: data as CFData),
                  let face = CGFont(provider) else {
                say("the \(data.count) bytes at \(url.absoluteString) are not a font this can read")
                return
            }
            DispatchQueue.main.async {
                font = face
                then()
            }
        }.resume()
    }

    /// image draws one menu picture, or nothing when there is nothing to
    /// draw it with.
    ///
    /// The drawing is done when the image is drawn rather than now, so that
    /// the colours are the ones in force at that moment: `labelColor` is
    /// black on a light menu and white on a dark one, and a picture baked
    /// here would be whichever it was at launch.
    static func image(_ chars: String) -> NSImage? {
        guard let face = font, !chars.isEmpty else { return nil }
        let text = CTFontCreateWithGraphicsFont(face, size, nil, nil)

        // What is being drawn: the parts, in order, and where each one's
        // ink lies. A single character is the mark itself and is drawn in
        // the colour kept for what an entry writes; two are a note with a
        // mark on it, and then the note is the colour of the words beside
        // it and the mark is not.
        let parts = Array(chars)
        var inks: [CGRect] = []
        var ids: [CGGlyph] = []
        for ch in parts {
            var units = Array(String(ch).utf16)
            var glyph = CGGlyph(0)
            guard CTFontGetGlyphsForCharacters(text, &units, &glyph, units.count),
                  glyph != 0 else { return nil }
            var one = glyph
            var ink = CGRect.zero
            CTFontGetBoundingRectsForGlyphs(text, .horizontal, &one, &ink, 1)
            guard ink.width > 0, ink.height > 0 else { return nil }
            ids.append(glyph)
            inks.append(ink)
        }

        // Where the mark goes under the note it belongs to: hung a little
        // below the bottom of the note and centred on it, by ink rather
        // than by a number per mark, so a staccato dot and a marcato wedge
        // both land where they should. Below, because the note is drawn
        // with its stem up and an articulation goes on the side away from
        // the stem — which is what a printed score does, and what keeps the
        // mark out of the stem's way.
        //
        // The same rule as the page's, and the same quarter of a staff
        // space between them.
        var at: [CGPoint] = [.zero]
        if parts.count > 1 {
            let gap = size * 0.0625
            let note = inks[0], mark = inks[1]
            at.append(CGPoint(
                x: note.midX - mark.midX,
                y: note.minY - gap - mark.maxY))
        }

        // The box the whole thing occupies, which is what gets scaled down
        // to the size of a row.
        var bounds = inks[0]
        for i in 1..<inks.count {
            bounds = bounds.union(inks[i].offsetBy(dx: at[i].x, dy: at[i].y))
        }
        let scale = min(box.width / bounds.width, box.height / bounds.height, 1)
        let drawn = NSSize(width: bounds.width * scale, height: bounds.height * scale)

        let image = NSImage(size: drawn, flipped: false) { _ in
            guard let ctx = NSGraphicsContext.current?.cgContext else { return false }
            ctx.scaleBy(x: scale, y: scale)
            ctx.translateBy(x: -bounds.minX, y: -bounds.minY)
            for i in ids.indices {
                // One character is the mark; of two, the second is.
                let isMark = parts.count == 1 || i > 0
                // setFill rather than a CGColor worked out here: these two
                // are catalogue colours, black on a light menu and white on
                // a dark one, and asking one for its components resolves it
                // against whatever appearance happens to be current rather
                // than against the menu being drawn. Asked this way it is
                // the menu's.
                (isMark ? NSColor.systemRed : NSColor.labelColor).setFill()
                var glyph = ids[i]
                var where_ = at[i]
                CTFontDrawGlyphs(text, &glyph, &where_, 1, ctx)
            }
            return true
        }
        // Not a template: a template image is tinted to one colour by the
        // system, and the whole point of these is that they are two.
        image.isTemplate = false
        return image
    }
}
