#!/bin/sh
#
# build.sh — a homr to ship beside gscore
# gscore - a musical score editor
#
# (C) Copyright 2026 Sebastien Tricaud
#
# Permission is hereby granted, free of charge, under the terms of the MIT
# licence; see the LICENSE file at the root of this repository.
#
# homr reads photographed and scanned pages of music; gscore runs it as a
# program of its own (internal/format/scanned, internal/app/scanengine.go).
# This builds a copy that needs nothing installed: a private Python, homr
# and the ONNX runtime it runs on, the models it would otherwise fetch on
# first use, and the launcher gscore looks for. The folder is relocatable
# and is dropped beside the gscore binary by the Makefile when OMR_SIDECAR
# names it — see the omr workflow.
#
#   packaging/homr/build.sh DEST [cpu|cuda]
#
# Built on the platform it is for: the packages are compiled, and so is the
# interpreter. uv does the fetching: https://docs.astral.sh/uv/
#
# homr is AGPL-3.0, unmodified, run as a separate process. Its licence and
# its source archive go in the folder with it.

set -eu

dest=${1:?usage: build.sh DEST [cpu|cuda]}
extra=${2:-cpu}
version=${HOMR_VERSION:-0.7.1}
python=${HOMR_PYTHON:-3.12}

command -v uv >/dev/null 2>&1 || {
	echo "need uv: https://docs.astral.sh/uv/getting-started/installation/" >&2
	exit 1
}

rm -rf "$dest"
mkdir -p "$dest"
# The folder has to be named by an absolute path from here on: the launcher
# and uv both resolve against it.
dest=$(cd "$dest" && pwd)

echo "== a Python $python of its own"
# A relocatable interpreter, which is what uv installs; it lands in a folder
# named by its full version, renamed to something the launcher can find.
UV_PYTHON_INSTALL_DIR="$dest/pythons" uv python install "$python" --no-bin >/dev/null 2>&1 \
	|| UV_PYTHON_INSTALL_DIR="$dest/pythons" uv python install "$python"
# uv puts a link named by the short version beside the folder named by the
# full one; the folder is what is wanted.
for d in "$dest"/pythons/cpython-*; do
	if [ -d "$d" ] && [ ! -L "$d" ]; then
		mv "$d" "$dest/python"
		break
	fi
done
rm -rf "$dest/pythons"
test -d "$dest/python" || { echo "uv installed no Python in $dest/pythons" >&2; exit 1; }
if [ -x "$dest/python/bin/python3" ]; then
	py="$dest/python/bin/python3"
else
	py="$dest/python/python.exe"
fi
# uv marks the interpreter as its own and not to be installed into. This
# copy is nobody's but this folder's, so the mark comes off.
rm -f "$dest"/python/lib/python*/EXTERNALLY-MANAGED "$dest"/python/Lib/EXTERNALLY-MANAGED
"$py" -I -c 'import sys; print("  ", sys.version.split()[0], sys.platform)'

echo "== homr $version [$extra]"
uv pip install --python "$py" --no-progress "homr[$extra]==$version"

echo "== the models"
run() { "$py" -I -c 'from homr.main import main; main()' "$@"; }
# What the processor runs, and what this kind of machine would pick on its
# own — the same files but for a graphics card or CoreML, where there is
# one. Both, so that nothing is fetched on a machine that has no network.
run --init --no-title --gpu no
run --init --no-title

echo "== the launchers"
mkdir -p "$dest/bin"
cat > "$dest/bin/homr" <<'SH'
#!/bin/sh
# homr, shipped beside gscore: run by the Python next to this script.
here=$(cd "$(dirname "$0")/.." && pwd)
exec "$here/python/bin/python3" -I -c 'from homr.main import main; main()' "$@"
SH
chmod 755 "$dest/bin/homr"
printf '@echo off\r\n"%%~dp0python\\python.exe" -I -c "from homr.main import main; main()" %%*\r\n' \
	> "$dest/homr.cmd"

echo "== the licence and the source"
site=$("$py" -I -c 'import sysconfig; print(sysconfig.get_paths()["purelib"])')
for f in "$site"/homr-*.dist-info/licenses/LICENSE "$site"/homr-*.dist-info/LICENSE; do
	if [ -f "$f" ]; then cp "$f" "$dest/LICENSE-homr.txt"; break; fi
done
test -f "$dest/LICENSE-homr.txt" || echo "  (no licence file in the package; see the README)"
# The AGPL asks that whoever gets the program can get its source. The
# archive it was built from, from PyPI, goes in beside it.
"$py" -I - "$version" "$dest" <<'PY'
import json, sys, urllib.request
version, dest = sys.argv[1], sys.argv[2]
with urllib.request.urlopen("https://pypi.org/pypi/homr/%s/json" % version) as r:
    info = json.load(r)
for f in info["urls"]:
    if f["packagetype"] == "sdist":
        urllib.request.urlretrieve(f["url"], dest + "/" + f["filename"])
        print("  ", f["filename"])
        break
else:
    print("   (PyPI has no source archive for this version)")
PY
cat > "$dest/README.txt" <<TXT
homr $version, shipped beside gscore.

homr reads photographed and scanned pages of music and writes MusicXML,
which gscore then opens. gscore runs it as a separate program: bin/homr
(homr.cmd on Windows) starts it with the Python in python/.

homr is free software under the GNU Affero General Public License,
version 3 — see LICENSE-homr.txt — by Christian Liebhardt and the homr
contributors, https://github.com/liebharc/homr. It is unmodified; its
source is beside this file as homr-$version.tar.gz and at
https://pypi.org/project/homr/$version/. The models it uses are under
the same licence; the data they were trained on is listed in the
project's LICENSE_FAQ.md.

Built with onnxruntime for: $extra
TXT

echo "== trimming"
# The interpreter's own test suite and the things a desktop Python ships
# that nothing here uses.
for d in test idlelib tkinter turtledemo ensurepip; do
	rm -rf "$dest"/python/lib/python*/"$d" "$dest/python/Lib/$d"
done
find "$dest/python" -name '__pycache__' -type d -prune -exec rm -rf {} + 2>/dev/null || true

echo "== checking"
if [ -x "$dest/bin/homr" ] && [ -x "$dest/python/bin/python3" ]; then
	"$dest/bin/homr" --init --no-title 2>&1 | tail -1
fi
echo "built $dest ($(du -sh "$dest" | cut -f1))"
