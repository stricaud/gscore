/*
 * savepanel.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package desktop

// Lock is what the save panel says to do about a .gscore's password.
type Lock int

const (
	LockOpen Lock = iota // save it open
	LockNew              // lock it under the password typed
	LockKeep             // keep the password it was opened with
)
