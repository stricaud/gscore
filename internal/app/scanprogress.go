/*
 * scanprogress.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How far the page reader has got, for the page to show while it waits.
 *
 * A read takes a minute a page, and the reply that is waiting for it says
 * nothing until it is done. The reader says where it is as it works, the
 * scanned package follows it, and this keeps the latest word for the page
 * to ask for — which it does every second or so while an import runs, and
 * puts beside the mascot as a percentage.
 */

package app

import (
	"net/http"
	"sync"

	"github.com/stricaud/gscore/internal/format/scanned"
)

// scanState is the page reader's progress, under a lock of its own: it is
// written from the goroutine reading the reader's output while the
// request that started it may hold the document's lock.
type scanState struct {
	mu     sync.Mutex
	active bool
	now    scanned.Progress
}

// begin says a read has started; end that it is over.
func (s *scanState) begin() {
	s.mu.Lock()
	s.active, s.now = true, scanned.Progress{}
	s.mu.Unlock()
}

func (s *scanState) end() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
}

// report is what the reader is told to call as it goes.
func (s *scanState) report(p scanned.Progress) {
	s.mu.Lock()
	s.now = p
	s.mu.Unlock()
}

// handleScanProgress answers where the page reader is: active, and the
// page and staff it is on with the percentage they come to, or inactive.
func (a *App) handleScanProgress(w http.ResponseWriter, r *http.Request) {
	a.scan.mu.Lock()
	active, now := a.scan.active, a.scan.now
	a.scan.mu.Unlock()
	writeJSON(w, map[string]any{
		"active": active, "percent": now.Percent,
		"page": now.Page, "pages": now.Pages,
		"staff": now.Staff, "staves": now.Staves,
		"what": now.String(),
	})
}
