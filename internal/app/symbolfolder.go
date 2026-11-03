/*
 * symbolfolder.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stricaud/gscore/internal/symbol"
)

/*
 * A folder of symbols, beside the folder of plugins.
 *
 * A symbol drawn in one score was only ever in that score. It travelled
 * with the file, which is right — a score has to carry its own marks or it
 * cannot be opened anywhere else — and it meant that somebody who drew a
 * set of marks for their own music had to import them into every new piece
 * from an old one.
 *
 * So: Gscore/symbols, read when the editor starts. What is in it belongs to
 * the person rather than to a score, is there in every score they open, and
 * is still written into each file that uses it. The folder is a second
 * source and not a replacement: a .gscore stays self-contained, which is
 * the thing that must not change.
 *
 * Where the two disagree, the score wins. Opening somebody else's piece
 * must not quietly redraw their marks with yours — their file says what
 * their music looks like, and a mark that changed meaning depending on who
 * opened it would be worse than no folder at all.
 */

// SymbolsFolder is the folder symbols live in, inside the Gscore folder.
const SymbolsFolder = "symbols"

// symbolFileMax is as much of a file as is read. A symbol is a small
// drawing; anything past this is not one, and reading it to find out would
// be reading whatever happened to be dropped in the folder.
const symbolFileMax = 2 << 20

// SymbolsDir is the folder symbols are kept in, made the first time it is
// asked for.
//
// It falls back rather than failing, the way the plugins folder does: a
// home that cannot be written to is no reason to refuse to run the editor.
// There are then no folder symbols, which is what an empty folder means
// anyway.
func SymbolsDir() string { return symbolsDirIn(ScoresDir()) }

// symbolsDirIn is the same, under a folder it is given rather than under
// the one the program found. Separate for the reason pluginsDirIn is: a
// test has to be able to say where the home is.
func symbolsDirIn(scores string) string {
	if scores == "" {
		return ""
	}
	dir := filepath.Join(scores, SymbolsFolder)
	if st, err := os.Stat(dir); err == nil {
		if !st.IsDir() {
			// Something that is not a folder is already called that. It
			// is not gscore's to move.
			return ""
		}
		return dir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	return dir
}

// folderSymbol is one file read out of the folder, and what went wrong
// where something did.
type folderSymbol struct {
	Sym  *symbol.Symbol
	File string
	Err  string
}

// readSymbolFolder reads every drawing in a folder.
//
// A file it cannot read is recorded and the rest are read, which is the
// rule every reader here follows: one bad file in a folder of forty is not
// a reason to have none of them.
//
// Named after the file, without its extension, because that is the name
// the person gave it and the one they will look for.
func readSymbolFolder(dir string) []folderSymbol {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []folderSymbol
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".svg", ".xpm", ".png", ".gif", ".jpg", ".jpeg":
		default:
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		sym, err := readSymbolFile(filepath.Join(dir, e.Name()), name, ext)
		if err != nil {
			out = append(out, folderSymbol{File: e.Name(), Err: err.Error()})
			continue
		}
		out = append(out, folderSymbol{Sym: sym, File: e.Name()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// readSymbolFile reads one drawing.
func readSymbolFile(path, name, ext string) (*symbol.Symbol, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err == nil && st.Size() > symbolFileMax {
		return nil, errTooBig
	}

	if ext == ".svg" {
		return symbol.ReadSVG(f, name, 0)
	}
	if ext == ".xpm" {
		return symbol.ReadXPMNamed(f, name)
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return symbolFromImage(img, name), nil
}

type tooBig struct{}

func (tooBig) Error() string {
	return "the file is larger than a symbol could be"
}

var errTooBig = tooBig{}

// symbolFromImage turns a picture into a grid, taking anything that is not
// transparent and not nearly white as ink.
//
// The same reading the drawing window uses for an imported picture, so a
// file dropped in the folder and the same file imported by hand come out
// alike.
func symbolFromImage(img image.Image, name string) *symbol.Symbol {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > 256 {
		w = 256
	}
	if h > 256 {
		h = 256
	}
	bm := symbol.NewBitmap(w, h)
	bm.OriginY = float64(h) / 2
	ink := color.NRGBA{A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a < 0x8000 {
				continue
			}
			if r > 0xc000 && g > 0xc000 && bl > 0xc000 {
				continue // near enough to white to be the paper
			}
			bm.Set(x, y, ink)
		}
	}
	return &symbol.Symbol{Name: name, Kind: symbol.FromPixels, Pixels: bm}
}

// loadFolderSymbols puts the folder's drawings into the registry and says
// what it found.
//
// Called before a score's own symbols are read, so that a score that
// carries a symbol of the same name writes over the folder's copy rather
// than the other way about. See the note at the top of this file.
func (a *App) loadFolderSymbols() []folderSymbol {
	found := readSymbolFolder(SymbolsDir())
	for _, f := range found {
		if f.Sym == nil {
			continue
		}
		f.Sym.Source = symbol.FromFolder
		_ = a.symbols.Add(f.Sym)
	}
	return found
}
