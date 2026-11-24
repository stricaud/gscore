//go:build js

/*
 * assistantlog_nodisk.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

// logAssistant has nowhere to write in a build with no disk.
func logAssistant(assistantEntry) {}
