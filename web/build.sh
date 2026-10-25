#!/bin/sh
# Build the gscore web site.
#
# The output is a directory any plain file host can serve:
#
#     /          the front page, with music in it you can play and share
#     /app/      the editor itself — the whole program, as WebAssembly
#     /scores/   every sample, engraved, in every format gscore writes
#     /docs/     the markdown in docs/, as pages
#
#     sh web/build.sh [outdir]      # default: web/dist
#
# Nothing here engraves anything. The scores are set by running the real
# gscore binary over samples/, which is the same thing a user does with
# "Export as a web page", so what the site shows is the program's own output
# and not a second rendering of it that nobody tests.
#
# (C) Copyright 2026 Sebastien Tricaud. MIT licence; see LICENSE.

set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-"$root/web/dist"}
site="$root/web/site"
version=$(tr -d ' \t\n\r' < "$root/VERSION")

rm -rf "$out"
mkdir -p "$out/app" "$out/scores" "$out/docs"

# ── the editor, as WebAssembly ──────────────────────────────────────────────

echo "building the core for WebAssembly…"
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w \
    -X github.com/stricaud/gscore/internal/version.Version=$version" \
    -o "$out/app/gscore.wasm" "$root/web"

# wasm_exec.js is the Go runtime's own loader and must match the toolchain
# that built the module, so it is copied from this Go installation rather
# than kept in the repository.
goroot=$(go env GOROOT)
for candidate in "$goroot/lib/wasm/wasm_exec.js" "$goroot/misc/wasm/wasm_exec.js"; do
    if [ -f "$candidate" ]; then
        cp "$candidate" "$out/app/wasm_exec.js"
        break
    fi
done
if [ ! -f "$out/app/wasm_exec.js" ]; then
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
    cp "$f" "$out/app/"
done
cp "$root/web/boot.js" "$site/share.js" "$out/app/"
cp "$root/internal/brand/gilbert.png" "$root/internal/brand/gilbert.svg" "$out/app/"
cp "$root/internal/smufl/data/Bravura.otf" "$out/app/font.otf"

# The page differs from the application's only in loading the runtime and the
# core before the interface. share.js comes first because boot.js reads a
# score out of the address bar with it.
sed -e 's|<script src="app.js"></script>|<script src="wasm_exec.js"></script>\
<script src="share.js"></script>\
<script src="boot.js"></script>\
<script src="app.js"></script>|' \
    "$root/internal/app/ui/index.html" > "$out/app/index.html"

# ── the scores, set by gscore itself ────────────────────────────────────────

echo "engraving the sample scores…"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# Without cgo. This copy of gscore only has to read scores and write them
# out again — it never opens a window — and the native window wraps
# WebKitGTK on Linux, whose headers a build machine has no reason to carry.
# With cgo on, the site could only be built on a desktop.
CGO_ENABLED=0 go build -trimpath -o "$tmp/gscore" "$root"

for abc in "$root"/samples/*.abc; do
    id=$(basename "$abc" .abc)
    printf '  %s' "$id"
    # The page a visitor opens, and the one the front page takes its drawing
    # from. The font is linked rather than carried: it is served beside the
    # page here, and one copy of it does for the whole site.
    "$tmp/gscore" -o "$out/scores/$id.html" -font ../font.otf "$abc" >/dev/null
    # And the same music every other way out of the program, because that is
    # the claim the site makes about it.
    for ext in pdf mid; do
        "$tmp/gscore" -o "$out/scores/$id.$ext" "$abc" >/dev/null 2>&1 || \
            printf ' (no %s)' "$ext"
    done
    for ext in musicxml ly; do
        "$tmp/gscore" -convert "$out/scores/$id.$ext" "$abc" >/dev/null 2>&1 || \
            printf ' (no %s)' "$ext"
    done
    cp "$abc" "$out/scores/$id.abc"
    printf '\n'
done

# ── everything the site itself is made of ───────────────────────────────────

cp "$site/site.css" "$site/hero.js" "$site/share.js" "$out/"
cp "$root/internal/brand/gilbert.png" "$root/internal/brand/gilbert.svg" "$out/"
cp "$root/internal/app/ui/icon.png" "$out/icon.png"

# The embedding example, with its iframes pointed at where the editor now
# lives. docs/embedding.md links to it, and a documented example that 404s
# is worse than none.
mkdir -p "$out/samples"
for f in "$root"/samples/*.html; do
    sed -e 's|\.\./build/web/app/|../app/|g' \
        -e 's|\.\./\.\./samples/|../scores/|g' "$f" > "$out/samples/$(basename "$f")"
done
cp "$root/internal/smufl/data/Bravura.otf" "$out/font.otf"

echo "writing the pages…"
go run "$root/web/mksite" \
    -out "$out" -site "$site" -docs "$root/docs" \
    -samples "$root/samples" -version "$version"

# GitHub Pages serves through Jekyll by default, which drops files it does not
# recognise; this turns that off.
: > "$out/.nojekyll"

wasm=$(ls -lh "$out/app/gscore.wasm" | awk '{print $5}')
scores=$(ls "$root"/samples/*.abc | wc -l | tr -d ' ')
docs=$(ls "$root"/docs/*.md | wc -l | tr -d ' ')
echo "built $out (editor $wasm, $scores scores, $docs pages of documentation)"
