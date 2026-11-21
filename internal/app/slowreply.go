/*
 * slowreply.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Answering a request whose work may take minutes.
 *
 * Reading a photographed page runs a recogniser for a minute a page, and
 * the native window gives up on a reply that has gone quiet for one. So a
 * reply that is not ready soon is opened anyway and kept alive with
 * spaces, which JSON allows in front of a value, until the answer can be
 * written behind them. The price is that the status line has been sent by
 * then: an error after that point travels in the body, as {"error":"…"},
 * and the page reads it from there.
 */

package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// slowAfter is how long a request is given to answer the ordinary way
// before the reply is opened and kept alive. Long enough for anything but
// the page reader. A variable for the tests, which cannot wait that long.
var slowAfter = 15 * time.Second

// slowly answers with what work returns: as a status and a body when it is
// quick, through fail where it failed; as a kept-alive 200 carrying the
// body or {"error"} when it is not. work runs on its own goroutine and must
// not touch w.
func slowly(w http.ResponseWriter, work func() (any, error), fail func(http.ResponseWriter, error)) {
	type result struct {
		body any
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		body, err := work()
		ch <- result{body, err}
	}()

	soon := time.NewTimer(slowAfter)
	defer soon.Stop()
	select {
	case res := <-ch:
		if res.err != nil {
			if body, ok := answerOf(res.err); ok {
				writeJSON(w, body)
				return
			}
			fail(w, res.err)
			return
		}
		writeJSON(w, res.body)
		return
	case <-soon.C:
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}
	tick := time.NewTicker(scanKeepalive)
	defer tick.Stop()
	for {
		select {
		case res := <-ch:
			if res.err != nil {
				if body, ok := answerOf(res.err); ok {
					json.NewEncoder(w).Encode(body)
					return
				}
				json.NewEncoder(w).Encode(map[string]string{"error": res.err.Error()})
				return
			}
			json.NewEncoder(w).Encode(res.body)
			return
		case <-tick.C:
			io.WriteString(w, " ")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

// badRequest is the fail for a request whose errors are all its own fault.
func badRequest(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}

// answerer is an error that is really an answer: the work stopped short of
// opening anything, to ask something first, and what it asks goes back as
// the body with a 200 whichever way the reply is being given.
type answerer interface {
	error
	Answer() any
}

func answerOf(err error) (any, bool) {
	var a answerer
	if errors.As(err, &a) {
		return a.Answer(), true
	}
	return nil, false
}
