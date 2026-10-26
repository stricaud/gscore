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
#
# Every script and stylesheet then carries the version it was built with.
#
# Without it a browser that has been here before serves itself yesterday's
# app.js against today's core, and the two are one program split in half:
# the editor asks the core for something it did not used to do, or does not
# ask for what it now can. That is not a failure anybody can read — the page
# comes up, the menus work, and one thing in it quietly does not — and the
# only way out is knowing to empty the cache, which is not something to ask
# of someone opening a web page. Reported as "it works, but I have to kill
# the cache".
#
# A query string is enough: the file on disk is unchanged and any host
# serves it, while the address is new for every release, so the browser
# fetches the whole set together or keeps the whole set together.
sed -e 's|<script src="app.js"></script>|<script src="wasm_exec.js"></script>\
<script src="share.js"></script>\
<script src="boot.js"></script>\
<script src="app.js"></script>|' \
    "$root/internal/app/ui/index.html" \
  | sed -E -e "s|(<script src=\"[A-Za-z0-9_.-]+\.js)\"|\1?v=$version\"|g" \
           -e "s|(<link rel=\"stylesheet\" href=\"[A-Za-z0-9_.-]+\.css)\"|\1?v=$version\"|g" \
  > "$out/app/index.html"

# ── the scores, set by gscore itself ────────────────────────────────────────

echo "engraving the sample scores…"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# Without cgo. This copy of gscore only has to read scores and write them
# out again — it never opens a window — and the native window wraps
# WebKitGTK on Linux, whose headers a build machine has no reason to carry.
# With cgo on, the site could only be built on a desktop.
CGO_ENABLED=0 go build -trimpath \
    -ldflags="-X github.com/stricaud/gscore/internal/version.Version=$version" \
    -o "$tmp/gscore" "$root"

for src in "$root"/samples/*; do
    case "$src" in
    *.abc|*.mxl|*.musicxml|*.xml|*.mscz|*.mscx|*.nwctxt|*.gscore) ;;
    *) continue ;;   # the embedding example and anything else
    esac
    id=$(basename "${src%.*}")
    ext="${src##*.}"
    printf '  %s' "$id"
    # The page a visitor opens, and the one the front page takes its drawing
    # from. It carries the music font inside it, because this is the file the
    # "Download the page" button hands over and the whole promise of that
    # button is a page that works wherever it ends up. Linked to the copy
    # served beside it here, it worked perfectly on the site and showed a
    # grid of empty boxes the moment anybody saved it — every notehead a
    # tofu square with E0A4 printed in it.
    #
    # The front page does not pay for this. It shows the drawing alone, as
    # <id>.inline.svg, which names the face that site.css declares once.
    "$tmp/gscore" -o "$out/scores/$id.html" "$src" >/dev/null
    # What the program says the piece is, for the pages that list it. Asked
    # of gscore rather than parsed out of the file here, because gscore is
    # the thing that knows how to read every format it reads.
    "$tmp/gscore" -describe "$src" 2>/dev/null > "$out/scores/$id.txt"
    # And the same music every other way out of the program, because that is
    # the claim the site makes about it.
    # The still picture the front page shows before anybody presses play.
    # Standing alone too, for the same reason: it is offered as a file.
    "$tmp/gscore" -o "$out/scores/$id.svg" "$src" >/dev/null 2>&1 || \
        printf ' (no svg)'
    for want in pdf mid; do
        "$tmp/gscore" -o "$out/scores/$id.$want" "$src" >/dev/null 2>&1 || \
            printf ' (no %s)' "$want"
    done
    for want in musicxml ly; do
        "$tmp/gscore" -convert "$out/scores/$id.$want" "$src" >/dev/null 2>&1 || \
            printf ' (no %s)' "$want"
    done
    # And the file it was written in, whatever that was.
    cp "$src" "$out/scores/$id.$ext"
    printf '\n'
done

# ── everything the site itself is made of ───────────────────────────────────

cp "$site/site.css" "$site/hero.js" "$site/share.js" "$out/"
cp "$root/internal/brand/gilbert.png" "$root/internal/brand/gilbert.svg" "$out/"
# The application icon, which is what the page wears as its own mark.
# Gilbert on his own is the mascot, and the loading animation uses him.
cp "$root/assets/gscore.svg" "$out/gscore.svg"
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
scores=$(ls "$out"/scores/*.html | wc -l | tr -d ' ')
scores=$((scores - 1))   # the gallery's own index
docs=$(ls "$root"/docs/*.md | wc -l | tr -d ' ')
echo "built $out (editor $wasm, $scores scores, $docs pages of documentation)"
