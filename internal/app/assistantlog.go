/*
 * assistantlog.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A record of what was asked of the assistant and what it answered.
 *
 * A model writes music nobody can see it reason about, and what it was told
 * and what it said are gone the moment the window moves on. Kept, they are
 * how a good answer is asked for again, how a bad one is understood, and
 * how anybody finds out what the program sent where.
 */

package app

import (
	"fmt"
	"strings"
	"time"
)

// assistantEntry is one request and what came of it.
type assistantEntry struct {
	When     time.Time
	Provider string
	Model    string
	Prompt   string
	Bar      int // where the cursor was, counted from one
	Score    int // how long the score sent with it was, in characters
	Reply    string
	Thinking string
	// Placed is how many bars went onto the page and At the first of them;
	// Whole says the reply replaced the score rather than adding to it.
	Placed, At int
	Whole      bool
	Err        string
}

// String is the entry as it is written to the log: a heading with when and
// to whom, then the prompt, the reply and the outcome, each under a line of
// its own so that the file reads from top to bottom like the conversation.
func (e assistantEntry) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== %s · %s %s ===\n", e.When.Format("2006-01-02 15:04:05"),
		e.Provider, e.Model)
	fmt.Fprintf(&b, "--- asked, at bar %d (score of %d characters sent with it) ---\n",
		e.Bar, e.Score)
	b.WriteString(strings.TrimRight(e.Prompt, "\n") + "\n")
	if t := strings.TrimSpace(e.Thinking); t != "" {
		b.WriteString("--- thinking ---\n" + t + "\n")
	}
	if r := strings.TrimSpace(e.Reply); r != "" {
		b.WriteString("--- reply ---\n" + r + "\n")
	}
	switch {
	case e.Err != "":
		b.WriteString("--- error ---\n" + e.Err + "\n")
	case e.Whole:
		fmt.Fprintf(&b, "--- result ---\nreplaced the score: %d bars\n", e.Placed)
	default:
		fmt.Fprintf(&b, "--- result ---\n%d bars placed from bar %d\n", e.Placed, e.At)
	}
	b.WriteString("\n")
	return b.String()
}
