/*
 * symbolpicture.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/symbol"
)

/*
 * A picture used as a symbol, as it is.
 *
 * The drawing window is for drawing: a grid of ink and nothing, eight
 * pixels to the staff space, which is what the 287 pixmaps of 0.0.9 are and
 * what somebody sketching a sign wants. Putting a photograph or a
 * publisher's device through it reduces the thing to one bit and then asks
 * the person to repair it square by square, which is not editing, it is
 * penance.
 *
 * This is the other door. The picture becomes the symbol: a drawing keeps
 * its outlines and prints at the printer's own resolution, and a photograph
 * keeps its colours. Nothing is traced and nothing is thresholded. How
 * large it is drawn is then a question for the score — see
 * SymbolMark.Scale, which is per placement — rather than a property of the
 * picture, so the same device can be small over a note and large at the
 * head of a movement.
 */

const (
	// symbolPictureHeight is how tall an imported picture is, in staff
	// spaces, before anybody resizes it.
	//
	// Two spaces: half the height of a staff, which is large enough to be
	// recognised and small enough not to be in the way of the music while
	// it is being put where it goes. It is a starting size and not a
	// decision — the corner handles are the decision.
	symbolPictureHeight = 2.0

	// symbolPictureGrid is the longest side a picture is kept at, in
	// pixels.
	//
	// A symbol is saved into the score as XPM, which is a palette format:
	// a photograph kept at its own size would write a palette of tens of
	// thousands of colours into the file and make the score slow to open
	// for a mark a centimetre tall. At this size a picture is a few
	// thousand pixels, which is more than any printer asks of something
	// two staff spaces high.
	symbolPictureGrid = 192
)

// handleSymbolPicture makes a symbol out of a picture, as it is.
//
//	POST /api/symbolpicture?name=device   with the file as the body
func (a *App) handleSymbolPicture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, artworkLimit))
	if err != nil {
		http.Error(w, "the picture could not be read", http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	height, _ := strconv.ParseFloat(q.Get("height"), 64)
	a.symbolFromPicture(w, q.Get("name"), data, height)
}

// symbolFromPicture is the work, shared by the two ways a picture arrives:
// handed up from the page as a body, and read off the disk after a native
// dialog. The reading is the same either way.
func (a *App) symbolFromPicture(w http.ResponseWriter, name string, data []byte,
	height float64) {

	name = strings.TrimSpace(name)
	if name == "" {
		http.Error(w, "the symbol needs a name to be inserted by",
			http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "no picture was sent", http.StatusBadRequest)
		return
	}
	if height <= 0 {
		height = symbolPictureHeight
	}

	sym, err := symbolOf(name, data, height)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.lock()
	defer a.mu.Unlock()
	if err := a.symbols.Add(sym); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"status": "ok", "name": sym.Name,
		"kind": kindName(sym), "height": height,
	})
}

func kindName(s *symbol.Symbol) string {
	if s.Kind == symbol.FromVector {
		return "outline"
	}
	return "picture"
}

// symbolOf turns a file into a symbol, whichever kind of file it is.
//
// A drawing stays a drawing. An SVG becomes outlines, which print at the
// printer's resolution rather than at whatever grid they were sampled on —
// for a device of fine rules and small lettering that is the difference
// between a mark and a smudge. Everything else is a grid of colours, kept
// as it is rather than reduced to ink and nothing.
func symbolOf(name string, data []byte, height float64) (*symbol.Symbol, error) {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	if bytes.Contains(head, []byte("<svg")) {
		sym, err := symbol.ReadSVG(bytes.NewReader(data), name, height)
		if err != nil {
			return nil, fmt.Errorf("that drawing could not be read: %w", err)
		}
		return sym, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("that is not a PNG, a JPEG or an SVG")
	}
	img = scaleToFit(img, symbolPictureGrid)
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, fmt.Errorf("that picture has no size")
	}
	bm := symbol.NewBitmap(b.Dx(), b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, al := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			bm.Set(x, y, color.NRGBA{
				R: uint8(r >> 8), G: uint8(g >> 8),
				B: uint8(bl >> 8), A: uint8(al >> 8),
			})
		}
	}
	// How much of a staff space one pixel covers, so that the picture comes
	// out the height asked for whatever its own size is. The grid's own
	// eight-to-the-space is for drawings made in the drawing window.
	bm.PixelSize = height / float64(b.Dy())
	// Pinned by its middle. A sign put beside a note is put by eye and then
	// nudged, and a picture that grows from its top left corner runs away
	// from the place it was put as soon as it is made larger.
	bm.OriginX, bm.OriginY = float64(b.Dx())/2, float64(b.Dy())/2
	return &symbol.Symbol{Name: name, Kind: symbol.FromPixels, Pixels: bm}, nil
}
