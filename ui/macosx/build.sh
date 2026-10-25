#!/bin/sh
#
# build.sh
# gscore - a musical score editor
#
# (C) Copyright 2026 Sebastien Tricaud
#
# Permission is hereby granted, free of charge, under the terms of the MIT
# licence; see the LICENSE file at the root of this repository.
#
# Builds the macOS application for one architecture.
#
#   build.sh <arm64|amd64> <deployment-target> <output>
#
# Two moves. The Go core becomes a C archive — the model, the engraver, the
# importers and the server, with no window of its own — and the Swift
# application is compiled and linked against it. The Makefile calls this once
# per architecture and joins the results with lipo.
#
# It lives here rather than in the Makefile because a shell loop with a case
# statement, line continuations and nested quotes inside a make recipe is a
# thing nobody can read or fix.

set -eu

arch=${1:?which architecture}
min=${2:?deployment target}
out=${3:?output path}

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

case "$arch" in
arm64) triple="arm64-apple-macos$min" ;;
amd64) triple="x86_64-apple-macos$min" ;;
*) echo "build.sh: unknown architecture $arch" >&2; exit 2 ;;
esac

lib="build/cgo/$arch"
mkdir -p "$lib"

CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" MACOSX_DEPLOYMENT_TARGET="$min" \
	go build -tags nowebview -trimpath -buildmode=c-archive \
		-o "$lib/libgscore.a" ./ui/macosx/bridge

# The linker warns that Go's objects were built for a newer macOS than the
# application asks for. They were, and it does not matter: the deployment
# target is the oldest system the application runs on, and Go's runtime does
# not use anything newer. Everything else is worth seeing.
MACOSX_DEPLOYMENT_TARGET="$min" swiftc \
	-target "$triple" -O \
	-import-objc-header ui/macosx/Sources/Bridging.h \
	-I "$lib" \
	ui/macosx/Sources/*.swift \
	"$lib/libgscore.a" \
	-framework Cocoa -framework WebKit -framework UniformTypeIdentifiers \
	-lresolv \
	-o "$out" 2>&1 | grep -v 'was built for newer' || true

test -x "$out" || { echo "build.sh: $out was not produced" >&2; exit 1; }
