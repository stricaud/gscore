/*
 * symbols.go
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
	"encoding/json"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/symbol"
)

// Drawing a symbol by hand.
//
// internal/symbol can hold, decode and encode a hand-drawn glyph, and the
// model can refer to one by name. These are the doors into that: a grid comes
// in from the editor, is registered, and can then be attached to a note or
// dropped into a measure like any other mark.
//
// This is the one thing a music font cannot give you — a symbol nobody has
// standardised — and it is how gscore drew everything in 2005, as 287 XPM
// files anyone could edit in a text editor.

type symbolDoc struct {
	Name string `json:"name"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	// Pixels is row-major, one entry per pixel: 0 is transparent and 1 is ink.
	// A grid is a drawing, not a photograph, so one bit is enough.
	Pixels []int `json:"pixels"`
	// PixelSize is how much of a staff space one pixel covers. The default
	// matches the 287 original pixmaps, which were drawn eight pixels to the
	// staff space.
	PixelSize float64 `json:"pixelSize"`
	OriginX   float64 `json:"originX"`
	OriginY   float64 `json:"originY"`
}

type symbolInfo struct {
	Name      string  `json:"name"`
	W         int     `json:"w"`
	H         int     `json:"h"`
	PixelSize float64 `json:"pixelSize"`
	Kind      string  `json:"kind"`
	// Source is "score" for a symbol that travels in the file and
	// "folder" for one out of the symbols folder, so the palette can say
	// which is which.
	Source string `json:"source"`
}

// handleSymbols lists what has been drawn, or accepts a new drawing.
func (a *App) handleSymbols(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.createSymbol(w, r)
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	out := []symbolInfo{}
	for _, name := range a.symbols.Names() {
		s, ok := a.symbols.Lookup(name)
		if !ok {
			continue
		}
		info := symbolInfo{Name: name, Kind: s.Kind.String(),
			Source: s.Source.String()}
		if s.Pixels != nil {
			info.W, info.H = s.Pixels.W, s.Pixels.H
			info.PixelSize = s.Pixels.PixelSize
		}
		out = append(out, info)
	}
	writeJSON(w, out)
}

func (a *App) createSymbol(w http.ResponseWriter, r *http.Request) {
	var doc symbolDoc
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&doc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if doc.Name == "" {
		http.Error(w, "the symbol needs a name", http.StatusBadRequest)
		return
	}
	if doc.W <= 0 || doc.H <= 0 || doc.W > 256 || doc.H > 256 {
		http.Error(w, "the grid must be between 1 and 256 pixels each way",
			http.StatusBadRequest)
		return
	}
	if len(doc.Pixels) != doc.W*doc.H {
		http.Error(w, "the grid does not match its own size", http.StatusBadRequest)
		return
	}

	bm := symbol.NewBitmap(doc.W, doc.H)
	if doc.PixelSize > 0 {
		bm.PixelSize = doc.PixelSize
	}
	bm.OriginX, bm.OriginY = doc.OriginX, doc.OriginY
	if doc.OriginX == 0 && doc.OriginY == 0 {
		// Centre it vertically by default, which is what a staff-positioned
		// symbol wants.
		bm.OriginY = float64(doc.H) / 2
	}

	ink := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	for i, v := range doc.Pixels {
		if v != 0 {
			bm.Set(i%doc.W, i/doc.W, ink)
		}
	}

	// A name already taken is refused rather than written over.
	//
	// Add replaces what is there, which is right for reading a file and
	// wrong for somebody drawing: the symbol that goes is one they drew
	// earlier, it is already in their scores, and nothing said it had
	// gone. Saying so and letting them decide costs one more click and
	// cannot lose anything.
	a.mu.RLock()
	_, taken := a.symbols.Lookup(doc.Name)
	a.mu.RUnlock()
	if taken && r.URL.Query().Get("replace") != "1" {
		http.Error(w, "there is already a symbol called "+doc.Name,
			http.StatusConflict)
		return
	}

	a.lock()
	err := a.symbols.Add(&symbol.Symbol{
		Name: doc.Name, Kind: symbol.FromPixels, Pixels: bm,
	})
	a.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "name": doc.Name})
}

// handleSymbolPNG shows one symbol on its own, for the palette.
func (a *App) handleSymbolPNG(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	a.mu.RLock()
	s, ok := a.symbols.Lookup(name)
	a.mu.RUnlock()
	if !ok || s.Pixels == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	if err := pngEncode(w, s.Pixels.Image()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleInsertSymbol attaches a symbol to a measure, either on its own or on
// the element the cursor sits after.
func (a *App) handleInsertSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("name")
	if name == "" {
		http.Error(w, "which symbol?", http.StatusBadRequest)
		return
	}
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		at = -1
	}

	placement := model.PlacementAuto
	switch q.Get("placement") {
	case "above":
		placement = model.PlacementAbove
	case "below":
		placement = model.PlacementBelow
	}

	a.lock()
	defer a.mu.Unlock()

	_, m, err := a.locate(partIdx, measIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v := m.Voices[0]
	mark := model.SymbolMark{Name: name, Placement: placement}

	// Attached to the note just placed when there is one, so that a symbol
	// travels with its note; otherwise it stands on its own in the bar.
	if q.Get("attach") == "1" && at > 0 && at <= len(v.Elements) {
		switch el := v.Elements[at-1].(type) {
		case *model.Note:
			el.Symbols = append(el.Symbols, mark)
			writeJSON(w, map[string]string{"status": "ok", "attached": "note"})
			return
		case *model.Chord:
			el.Symbols = append(el.Symbols, mark)
			writeJSON(w, map[string]string{"status": "ok", "attached": "chord"})
			return
		}
	}

	el := &model.SymbolElement{Symbol: mark}
	if at < 0 || at > len(v.Elements) {
		at = len(v.Elements)
	}
	v.Elements = append(v.Elements, nil)
	copy(v.Elements[at+1:], v.Elements[at:])
	v.Elements[at] = el
	writeJSON(w, map[string]string{"status": "ok", "attached": "measure"})
}

// handleSymbolImport reads an existing picture into the editor's grid.
//
// Starting from something is far easier than starting from nothing, and there
// are 287 symbols in 0.0.9's pixmaps to start from. XPM is read directly, since
// that is what they are; PNG is accepted too, for anything drawn elsewhere.
func (a *App) handleSymbolImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writePictureGrid(w, data, "")
}

// writePictureGrid turns a picture into the grid the drawing window works
// in and answers with it.
//
// Shared by the two ways a picture arrives: handed up from the page as a
// body, and read off the disk after a native dialog. The reading is the
// same either way and belongs in one place — the window that receives it
// cannot tell, and should not have to.
func writePictureGrid(w http.ResponseWriter, data []byte, name string) {
	var bm *symbol.Bitmap
	var err error
	if bytes.Contains(data[:min(len(data), 256)], []byte("XPM")) {
		bm, err = symbol.DecodeXPM(bytes.NewReader(data))
	} else {
		var img image.Image
		img, _, err = image.Decode(bytes.NewReader(data))
		if err == nil {
			bm, err = fromImage(img)
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if bm.W > 256 || bm.H > 256 {
		http.Error(w, "that picture is larger than the editor's 256 pixel grid",
			http.StatusBadRequest)
		return
	}

	// The editor works in ink and nothing, so anything mostly opaque becomes
	// ink. A drawing is a shape, not a photograph.
	pix := make([]int, bm.W*bm.H)
	for y := 0; y < bm.H; y++ {
		for x := 0; x < bm.W; x++ {
			c := bm.At(x, y)
			lum := (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
			if c.A > 128 && lum < 160 {
				pix[y*bm.W+x] = 1
			}
		}
	}
	writeJSON(w, symbolDoc{
		Name: name, W: bm.W, H: bm.H, Pixels: pix,
		PixelSize: bm.PixelSize, OriginX: bm.OriginX, OriginY: bm.OriginY,
	})
}

func fromImage(img image.Image) (*symbol.Bitmap, error) {
	b := img.Bounds()
	bm := symbol.NewBitmap(b.Dx(), b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, al := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			bm.Set(x, y, color.NRGBA{
				R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: uint8(al >> 8),
			})
		}
	}
	return bm, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleMoveSymbol nudges a symbol from where the engraver put it.
func (a *App) handleMoveSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	dx, _ := strconv.ParseFloat(q.Get("dx"), 64)
	dy, _ := strconv.ParseFloat(q.Get("dy"), 64)
	name := q.Get("name")

	// A symbol standing in the bar on its own is moved first, since it
	// carries no Marks for markAt to find: it is the symbol, rather than
	// something a note is wearing.
	if lone, ok := a.loneSymbolAt(q); ok {
		a.lock()
		defer a.mu.Unlock()
		if name != "" && lone.Symbol.Name != name {
			http.Error(w, "no such symbol there", http.StatusBadRequest)
			return
		}
		if q.Get("absolute") == "1" {
			lone.Symbol.OffsetX, lone.Symbol.OffsetY = dx, dy
		} else {
			lone.Symbol.OffsetX += dx
			lone.Symbol.OffsetY += dy
		}
		writeJSON(w, map[string]int{"moved": 1})
		return
	}

	m, err := a.markAt(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.lock()
	defer a.mu.Unlock()

	moved := 0
	for i := range m.Symbols {
		if name != "" && m.Symbols[i].Name != name {
			continue
		}
		if q.Get("absolute") == "1" {
			m.Symbols[i].OffsetX, m.Symbols[i].OffsetY = dx, dy
		} else {
			m.Symbols[i].OffsetX += dx
			m.Symbols[i].OffsetY += dy
		}
		moved++
	}
	if moved == 0 {
		http.Error(w, "no such symbol on that element", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"moved": moved})
}

// handleRemoveSymbol takes a symbol off the page.
//
// Both kinds: one standing in a bar on its own, and one a note is
// wearing. Addressed the way moving one is, so that whatever the editor
// has picked up can be deleted without it having to know which kind it
// turned out to be.
func (a *App) handleRemoveSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("name")

	// A symbol of its own is an element of the bar, so taking it away is
	// taking an element out rather than clearing a field.
	if lone, ok := a.loneSymbolAt(q); ok && (name == "" || lone.Symbol.Name == name) {
		partIdx, _ := strconv.Atoi(q.Get("part"))
		measIdx, _ := strconv.Atoi(q.Get("measure"))
		at, _ := strconv.Atoi(q.Get("at"))

		a.lock()
		defer a.mu.Unlock()
		v := a.score.Parts[partIdx].Measures[measIdx].Voices[0]
		v.Elements = append(v.Elements[:at-1], v.Elements[at:]...)
		writeJSON(w, map[string]any{"status": "ok", "removed": 1})
		return
	}

	m, err := a.markAt(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.lock()
	defer a.mu.Unlock()

	kept := m.Symbols[:0]
	removed := 0
	for _, sm := range m.Symbols {
		if name == "" || sm.Name == name {
			removed++
			continue
		}
		kept = append(kept, sm)
	}
	if removed == 0 {
		http.Error(w, "no such symbol there", http.StatusBadRequest)
		return
	}
	m.Symbols = kept
	writeJSON(w, map[string]any{"status": "ok", "removed": removed})
}

// loneSymbolAt is the symbol standing on its own that a request names, if
// that is what it names.
//
// Looked at before the marks, because the two are addressed the same way —
// a part, a bar and a place in it — and only one of them can be right.
// Addressed the way markAt addresses a mark — at is one past the element,
// which is the convention the whole editor sends — so that one request can
// be tried as either without the caller having to know which it is.
func (a *App) loneSymbolAt(q url.Values) (*model.SymbolElement, bool) {
	partIdx, _ := strconv.Atoi(q.Get("part"))
	measIdx, _ := strconv.Atoi(q.Get("measure"))
	at, err := strconv.Atoi(q.Get("at"))
	if err != nil {
		return nil, false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if partIdx < 0 || partIdx >= len(a.score.Parts) {
		return nil, false
	}
	p := a.score.Parts[partIdx]
	if measIdx < 0 || measIdx >= len(p.Measures) {
		return nil, false
	}
	m := p.Measures[measIdx]
	if len(m.Voices) == 0 {
		return nil, false
	}
	v := m.Voices[0]
	if at <= 0 || at > len(v.Elements) {
		return nil, false
	}
	el, ok := v.Elements[at-1].(*model.SymbolElement)
	return el, ok
}

// handleImportSymbols takes the symbols out of a .gscore and adds them here.
//
// Any .gscore will do: one saved as a pack, or somebody's piece that happens
// to use marks you want. The music in it is ignored.
func (a *App) handleImportSymbols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var src io.Reader
	if path := r.URL.Query().Get("path"); path != "" {
		if !a.local {
			http.Error(w, "reading a path is only allowed on this machine",
				http.StatusForbidden)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()
		src = f
	} else {
		src = http.MaxBytesReader(w, r.Body, 32<<20)
	}

	incoming, err := gscorefmt.ReadSymbols(src)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.lock()
	added, replaced := 0, 0
	for _, name := range incoming.Names() {
		s, ok := incoming.Lookup(name)
		if !ok {
			continue
		}
		if _, exists := a.symbols.Lookup(name); exists {
			replaced++
		} else {
			added++
		}
		_ = a.symbols.Add(s)
	}
	a.mu.Unlock()

	writeJSON(w, map[string]any{
		"status": "ok", "added": added, "replaced": replaced,
	})
}

// importSymbolsFrom reads a pack chosen in the file dialog.
func (a *App) importSymbolsFrom(w http.ResponseWriter, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()

	incoming, err := gscorefmt.ReadSymbols(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.lock()
	added, replaced := 0, 0
	for _, name := range incoming.Names() {
		s, ok := incoming.Lookup(name)
		if !ok {
			continue
		}
		if _, exists := a.symbols.Lookup(name); exists {
			replaced++
		} else {
			added++
		}
		_ = a.symbols.Add(s)
	}
	a.mu.Unlock()
	writeJSON(w, map[string]any{"status": "ok", "added": added, "replaced": replaced})
}

// handleExportSymbols saves every drawing as a .gscore pack, for sharing.
func (a *App) handleExportSymbols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	a.mu.RLock()
	symbols := a.symbols
	n := symbols.Len()
	a.mu.RUnlock()
	if n == 0 {
		http.Error(w, "there are no symbols to share yet", http.StatusBadRequest)
		return
	}

	// Written to the response when no path is given, so a browser can
	// download it; written to disk when one is.
	path := r.URL.Query().Get("path")
	if path == "" && r.URL.Query().Get("dialog") == "1" && a.local {
		var err error
		var shown bool
		path, shown, err = saveDialog("symbols.gscore")
		if err != nil || (!shown && path == "") {
			writeJSON(w, map[string]any{
				"needPath":  true,
				"suggested": filepath.Join(a.saveDirectory(), "symbols.gscore"),
			})
			return
		}
		if path == "" {
			writeJSON(w, map[string]any{"cancelled": true})
			return
		}
	}
	if path != "" {
		if !a.local {
			http.Error(w, "saving to a path is only allowed on this machine",
				http.StatusForbidden)
			return
		}
		f, err := os.Create(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()
		if err := gscorefmt.WritePack(f, symbols); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"status": "ok", "path": path, "symbols": n})
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="symbols.gscore"`)
	if err := gscorefmt.WritePack(w, symbols); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
