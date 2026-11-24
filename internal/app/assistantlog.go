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
	When time.Time
	Took time.Duration
	// Excerpted says the whole score was too long and the bars around the
	// cursor were sent instead.
	Excerpted bool
	// Streamed says the thinking and the reply went into the log as they
	// arrived, and are not to be written again at the end.
	Streamed bool
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

// Asked is the start of an entry, written as the request goes: when, to
// whom, and what was asked.
func (e assistantEntry) Asked() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== %s · %s %s ===\n", e.When.Format("2006-01-02 15:04:05"),
		e.Provider, e.Model)
	fmt.Fprintf(&b, "--- asked, at bar %d (score of %d characters sent with it) ---\n",
		e.Bar, e.Score)
	b.WriteString(strings.TrimRight(e.Prompt, "\n") + "\n")
	return b.String()
}

// Answered is the rest of it, written when the request is over: what the
// model thought and said, how long it took, and what came of it. Each part
// under a line of its own, so the file reads from top to bottom like the
// conversation.
func (e assistantEntry) Answered() string {
	var b strings.Builder
	if e.Excerpted && !e.Streamed {
		b.WriteString("--- the whole score was too long; asked again with the bars around the cursor ---\n")
	}
	if t := strings.TrimSpace(e.Thinking); t != "" && !e.Streamed {
		b.WriteString("--- thinking ---\n" + t + "\n")
	}
	switch r := strings.TrimSpace(e.Reply); {
	case r != "" && !e.Streamed:
		b.WriteString("--- reply ---\n" + r + "\n")
	case r == "":
		b.WriteString("--- reply ---\n(nothing arrived from the model)\n")
	}
	took := e.Took.Round(time.Second)
	switch {
	case e.Err != "":
		fmt.Fprintf(&b, "--- error, after %s ---\n%s\n", took, e.Err)
	case e.Whole:
		fmt.Fprintf(&b, "--- result, after %s ---\nreplaced the score: %d bars\n", took, e.Placed)
	default:
		fmt.Fprintf(&b, "--- result, after %s ---\n%d bars placed from bar %d\n", took, e.Placed, e.At)
	}
	b.WriteString("\n")
	return b.String()
}

// logStream writes what a model sends into the log as it arrives: its
// thinking, then its reply, each under its own heading, a few times a second.
//
// Written at the end only, the log said nothing while the model worked —
// the question, and then minutes of nothing, with the thinking on the screen
// and not in the file — and a request that died partway took everything it
// had thought with it.
type logStream struct {
	section string
	buf     strings.Builder
	last    time.Time
	wrote   bool
	// open says what was last written out did not end its line.
	open bool
}

// logEvery is how often what has arrived is written out.
const logEvery = 500 * time.Millisecond

// add puts text under a heading, starting the heading if it is a new one.
func (l *logStream) add(section, text string) {
	if text == "" {
		return
	}
	if section != l.section {
		if l.section != "" && !strings.HasSuffix(l.buf.String(), "\n") && l.wroteMidLine() {
			l.buf.WriteString("\n")
		}
		l.buf.WriteString("--- " + section + " ---\n")
		l.section = section
	}
	l.buf.WriteString(text)
	l.wrote = true
	if time.Since(l.last) >= logEvery {
		l.flush()
	}
}

// note puts a line of its own in the log: something gscore did meanwhile.
func (l *logStream) note(text string) {
	l.endLine()
	l.buf.WriteString("--- " + text + " ---\n")
	l.section = ""
	l.flush()
}

// end finishes the last line, so the outcome starts on one of its own.
func (l *logStream) end() {
	l.endLine()
	l.flush()
}

func (l *logStream) endLine() {
	if l.wrote && l.wroteMidLine() {
		l.buf.WriteString("\n")
	}
}

// midLine says the last thing written did not end its line.
func (l *logStream) wroteMidLine() bool {
	if l.buf.Len() > 0 {
		return !strings.HasSuffix(l.buf.String(), "\n")
	}
	return l.open
}

func (l *logStream) flush() {
	l.last = time.Now()
	if l.buf.Len() == 0 {
		return
	}
	text := l.buf.String()
	l.open = !strings.HasSuffix(text, "\n")
	writeAssistantLog(text)
	l.buf.Reset()
}
