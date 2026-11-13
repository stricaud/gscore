/*
 * artwork.go
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
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The cover and the stamp: two pictures printed around the music.
 *
 * Neither is music, and neither is shown while the music is being written —
 * a cover standing between you and bar one is in the way. They appear when
 * the score is printed: the cover as the first page of the PDF, the stamp
 * along the foot of its last page, where a printer has always put a block
 * of their own.
 *
 * The file travels in the score rather than a path to it. A score that is
 * copied, renamed or emailed takes its pictures with it; a path is a
 * picture that is there on the machine it was made on and missing
 * everywhere else.
 */

// artworkLimit is the largest picture that will be taken in.
//
// Eight megabytes. A cover scan at a printable resolution is a few hundred
// kilobytes and a drawing is a few, so this is far above anything anybody
// means to use — it is here so that a mistaken file does not put a
// hundred-megabyte photograph inside a score and make it unopenable.
const artworkLimit = 8 << 20

// handleCover and handleStamp take a picture in, or take one away.
//
//	POST /api/cover          with the file as the body
//	POST /api/cover?clear=1  to take it away
//	POST /api/stamp?align=right
func (a *App) handleCover(w http.ResponseWriter, r *http.Request) {
	a.artworkEndpoint(w, r, "cover", func(s *model.Score) **model.Artwork {
		return &s.Cover
	})
}

func (a *App) handleStamp(w http.ResponseWriter, r *http.Request) {
	a.artworkEndpoint(w, r, "stamp", func(s *model.Score) **model.Artwork {
		return &s.Stamp
	})
}

func (a *App) artworkEndpoint(w http.ResponseWriter, r *http.Request,
	what string, field func(*model.Score) **model.Artwork) {

	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	if q.Get("clear") == "1" {
		a.lock()
		defer a.mu.Unlock()
		*field(a.score) = nil
		writeJSON(w, map[string]any{"status": "ok", what: nil})
		return
	}

	// The alignment on its own, for a picture already there: moving a
	// stamp from the middle to the right should not mean uploading it
	// again.
	align := strings.ToLower(strings.TrimSpace(q.Get("align")))
	if r.ContentLength == 0 && align != "" {
		a.lock()
		defer a.mu.Unlock()
		art := *field(a.score)
		if art == nil {
			http.Error(w, "there is no "+what+" to move", http.StatusBadRequest)
			return
		}
		if !okAlign(align) {
			http.Error(w, "align is left, center or right", http.StatusBadRequest)
			return
		}
		art.Align = align
		writeJSON(w, map[string]any{"status": "ok", "align": align})
		return
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, artworkLimit+1))
	if err != nil {
		http.Error(w, "the picture could not be read", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "no picture was sent", http.StatusBadRequest)
		return
	}
	if align != "" && !okAlign(align) {
		http.Error(w, "align is left, center or right", http.StatusBadRequest)
		return
	}
	a.setArtworkFrom(w, what, data, align)
}

// setArtworkFrom puts bytes already in hand into the score, and answers the
// way the endpoint does.
//
// Shared by the two ways a picture arrives: handed up from the page as a
// body, and read off the disk after a native dialog. The checking is the
// same either way and belongs in one place.
func (a *App) setArtworkFrom(w http.ResponseWriter, which string, data []byte, align string) {
	if len(data) == 0 {
		http.Error(w, "no picture was sent", http.StatusBadRequest)
		return
	}
	if len(data) > artworkLimit {
		http.Error(w, fmt.Sprintf(
			"that picture is larger than %d MB, which is more than a page "+
				"of print can use", artworkLimit>>20), http.StatusBadRequest)
		return
	}
	kind, ok := artworkKind(data)
	if !ok {
		http.Error(w, "that is not a PNG, a JPEG or an SVG",
			http.StatusBadRequest)
		return
	}
	align = strings.ToLower(strings.TrimSpace(align))
	if align != "" && !okAlign(align) {
		align = ""
	}
	a.lock()
	defer a.mu.Unlock()
	art := &model.Artwork{Data: data, Format: kind, Align: align}
	if which == "stamp" {
		a.score.Stamp = art
	} else {
		a.score.Cover = art
	}
	writeJSON(w, map[string]any{
		"status": "ok", "format": kind, "bytes": len(data), "align": align,
	})
}

func okAlign(s string) bool {
	return s == "left" || s == "center" || s == "right"
}

// artworkKind says what a file is, by looking at it rather than at its name.
//
// A browser sends whatever the file was called, and a drawing saved as
// "cover.png" is still a drawing. Reading the first bytes is the only
// answer that cannot be wrong — and refusing what is not a picture here
// means the PDF writer never has to wonder: by the time it sees one, it is
// one.
func artworkKind(data []byte) (string, bool) {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	if bytes.Contains(head, []byte("<svg")) {
		return "svg", true
	}
	if _, kind, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		switch kind {
		case "png", "jpeg":
			return kind, true
		}
	}
	return "", false
}

// artworkInfo is what the dialog is told about a picture: enough to say
// there is one and what it is, and not the picture itself.
type artworkInfo struct {
	Format string `json:"format"`
	Bytes  int    `json:"bytes"`
	Align  string `json:"align,omitempty"`
}

func artworkInfoOf(a *model.Artwork) *artworkInfo {
	if a == nil || len(a.Data) == 0 {
		return nil
	}
	return &artworkInfo{Format: a.Format, Bytes: len(a.Data), Align: a.Align}
}
