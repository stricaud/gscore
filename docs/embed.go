/*
 * embed.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package docs carries the API description into the binary.
//
// It is a Go file in the documentation folder, which looks odd until you ask
// where else it could be: go:embed cannot reach upwards out of a package, so
// a server that serves this file either lives beside it or holds a second
// copy of it. A second copy is the thing worth avoiding — the one in docs/
// is what a reader opens, the one embedded is what the running program
// answers with, and two files that are meant to be identical eventually are
// not. This way there is one file, read by whoever asks and by whatever
// serves it.
package docs

import _ "embed"

// OpenAPI is the description of everything the server does, served at
// /openapi.yaml and checked against the route table by
// internal/app/api_test.go.
//
//go:embed openapi.yaml
var OpenAPI []byte
