/*
 * zanderjaz.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package app

// The ZanderJaz collection.
//
// Five hundred-odd soundfonts gathered at zanderjaz.com, sorted into
// categories: a violin, a Hammond, a Rhodes, a trombone — one instrument per
// file, recorded by whoever recorded it. That is a different thing from a
// General MIDI bank and is better at what it is for. A bank gives you all 128
// programs at a uniform level of "fine"; this gives you one instrument that
// somebody cared about, which is what you want for the staff that carries the
// tune.
//
// Only the index travels with gscore — which files exist and what they are
// called, a few kilobytes. The soundfonts themselves are fetched when one is
// chosen, because the whole collection is gigabytes and nobody wants all of
// it.
//
// The list is theirs and parts of it have rotted: a file that has gone is a
// failed download with a message, which is what it should look like, and not
// something to prune the list over.

import (
	_ "embed"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
)

// zanderjazBase is where the files are.
const zanderjazBase = "https://www.zanderjaz.com/soundfonts/"

// zanderjazIndex is the collection's contents, one "category/file" per line,
// taken from the list the Debian packaging of it downloads.
//
//go:embed data/zanderjaz.txt
var zanderjazIndex string

// zanderjazFont is one soundfont in the collection.
type zanderjazFont struct {
	// Name is what to show: the file's name, readable.
	Name string `json:"name"`
	// URL is where it is.
	URL string `json:"url"`
}

type zanderjazCategory struct {
	Name  string          `json:"name"`
	Fonts []zanderjazFont `json:"fonts"`
}

var zanderjaz struct {
	once       sync.Once
	categories []zanderjazCategory
	count      int
}

// zanderjazCategories reads the index, once.
func zanderjazCategories() ([]zanderjazCategory, int) {
	zanderjaz.once.Do(func() {
		byName := map[string]*zanderjazCategory{}
		var order []string
		for _, line := range strings.Split(zanderjazIndex, "\n") {
			line = strings.TrimSpace(line)
			category, file, ok := strings.Cut(line, "/")
			if !ok || file == "" {
				continue
			}
			c := byName[category]
			if c == nil {
				c = &zanderjazCategory{Name: category}
				byName[category] = c
				order = append(order, category)
			}
			c.Fonts = append(c.Fonts, zanderjazFont{
				Name: readableFontName(file),
				URL:  zanderjazBase + category + "/" + file,
			})
			zanderjaz.count++
		}
		sort.Strings(order)
		for _, name := range order {
			c := byName[name]
			sort.Slice(c.Fonts, func(i, j int) bool {
				return strings.ToLower(c.Fonts[i].Name) < strings.ToLower(c.Fonts[j].Name)
			})
			zanderjaz.categories = append(zanderjaz.categories, *c)
		}
	})
	return zanderjaz.categories, zanderjaz.count
}

// readableFontName turns a file name into something to put in a list: the
// escaping undone and the extension off, and nothing else. The names are
// whoever made them's, and tidying them further would only make two different
// soundfonts look like the same one.
func readableFontName(file string) string {
	if unescaped, err := url.PathUnescape(file); err == nil {
		file = unescaped
	}
	return strings.TrimSuffix(file, path.Ext(file))
}

// handleZanderjaz lists the collection for the dialog to browse.
func (a *App) handleZanderjaz(w http.ResponseWriter, r *http.Request) {
	categories, count := zanderjazCategories()
	writeJSON(w, map[string]any{
		"categories": categories,
		"count":      count,
		"about": "Soundfonts collected at zanderjaz.com, mostly one instrument " +
			"each. They are fetched one at a time; some of the links have gone " +
			"stale over the years and will say so.",
	})
}
