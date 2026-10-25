#!/bin/sh
# Build gscore as a static web page.
#
# The output is a directory that any plain file host can serve: the interface's
# own files, the Go core as WebAssembly, and the loader that joins them.
#
#     sh web/build.sh [outdir]      # default: web/dist
#
# (C) Copyright 2026 Sebastien Tricaud. MIT licence; see LICENSE.

set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-"$root/web/dist"}

rm -rf "$out"
mkdir -p "$out"

echo "building the core for WebAssembly…"
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" \
    -o "$out/gscore.wasm" "$root/web"

# wasm_exec.js is the Go runtime's own loader and must match the toolchain
# that built the module, so it is copied from this Go installation rather
# than kept in the repository.
goroot=$(go env GOROOT)
for candidate in "$goroot/lib/wasm/wasm_exec.js" "$goroot/misc/wasm/wasm_exec.js"; do
    if [ -f "$candidate" ]; then
        cp "$candidate" "$out/wasm_exec.js"
        break
    fi
done
if [ ! -f "$out/wasm_exec.js" ]; then
    echo "could not find wasm_exec.js in $goroot" >&2
    exit 1
fi

echo "copying the interface…"
# Everything the interface is made of, rather than a list of names. A list
# goes stale the moment a file is added, and the page then loads an index
# that refers to something which is not there — which nothing would notice
# until the site was open in front of someone.
for f in "$root"/internal/app/ui/*; do
    name=$(basename "$f")
    [ "$name" = "index.html" ] && continue   # patched below, not copied
    cp "$f" "$out/"
done
cp "$root/web/boot.js" "$out/"

# The mascot is kept with the name rather than with the interface, because
# the PDF writer stamps the same drawing on every page it prints. As a page
# he is a plain file, so he is copied beside the rest.
cp "$root/internal/brand/gilbert.png" "$out/"
cp "$root/internal/brand/gilbert.svg" "$out/"

# The font is served by the Go handler in the application; as a page it is a
# plain file, so it is copied beside the rest.
cp "$root/internal/smufl/data/Bravura.otf" "$out/font.otf"

# The page differs from the application's only in loading the runtime and the
# core before the interface.
sed -e 's|<script src="app.js"></script>|<script src="wasm_exec.js"></script>\n<script src="boot.js"></script>\n<script src="app.js"></script>|' \
    "$root/internal/app/ui/index.html" > "$out/index.html"

# GitHub Pages serves through Jekyll by default, which drops files it does not
# recognise; this turns that off.
: > "$out/.nojekyll"

size=$(ls -lh "$out/gscore.wasm" | awk '{print $5}')
echo "built $out (gscore.wasm $size)"
