//go:build js

/*
 * video_nodisk.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/stricaud/gscore/internal/model"
)

// recordVideo needs two programs on the machine, and a page has none.
func recordVideo(context.Context, io.Writer, *model.Score, recording) error {
	return errors.New("a video is made by gscore on your own machine, not in the page")
}

func videoTools() (string, string, []string) { return "", "", []string{"ffmpeg", "rsvg-convert"} }

func videoInstall() string { return "" }

// handleVideoTools says that nothing here can make a video.
func (a *App) handleVideoTools(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"missing": []string{"ffmpeg", "rsvg-convert"}, "install": ""})
}

// handleVideoProgress has nothing to report.
func (a *App) handleVideoProgress(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"phase": "", "percent": 0})
}
