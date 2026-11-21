//go:build !(darwin && cgo)

/*
 * savepanel_other.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package desktop

// SavePanelLocks says the save panel can ask about the password itself.
// Here it cannot: the dialogs on Linux and Windows are other programs'
// and take nothing added to them, so the editor asks before the panel.
const SavePanelLocks = false

// SavePanel is not available here; see SavePanelLocks.
func SavePanel(dir, name string, lockable, locked bool) (string, Lock, string) {
	return "", LockOpen, ""
}
