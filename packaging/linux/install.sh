#!/bin/sh
# install.sh — put gscore on this machine.
#
# gscore is one static binary and a soundfont; there is nothing to compile
# and nothing to fetch. This script only copies them somewhere on your PATH
# and registers the desktop entry so the program appears in the menus.
#
# Run it as root to install for everybody, or as yourself to install under
# ~/.local. Either way it prints what it did and where, so uninstalling is
# deleting those files.
#
# (C) Copyright 2026 Sebastien Tricaud. MIT licence; see LICENSE.

set -e

here=$(cd "$(dirname "$0")" && pwd)

if [ -n "$PREFIX" ]; then
	prefix=$PREFIX
elif [ "$(id -u)" = 0 ]; then
	prefix=/usr/local
else
	prefix=$HOME/.local
	echo "not root, so installing for you alone under $prefix"
	echo "(run as root, or set PREFIX, to install for everybody)"
fi

bin=$prefix/bin
# Where gscore looks for instruments by itself, so that the soundfont is
# found with nothing to configure after this.
fonts=$prefix/share/soundfonts
apps=$prefix/share/applications
icons=$prefix/share/icons/hicolor/192x192/apps

mkdir -p "$bin" "$fonts" "$apps" "$icons"

install -m 755 "$here/gscore" "$bin/gscore"
echo "installed $bin/gscore"

for sf2 in "$here"/soundfonts/*.sf2; do
	[ -e "$sf2" ] || continue
	install -m 644 "$sf2" "$fonts/"
	echo "installed $fonts/$(basename "$sf2")"
done

install -m 644 "$here/gscore.desktop" "$apps/gscore.desktop"
install -m 644 "$here/gscore.png" "$icons/gscore.png"
echo "installed $apps/gscore.desktop"

# Harmless if either is absent: the entry is still on the disk and the menu
# picks it up at the next login.
command -v update-desktop-database >/dev/null 2>&1 && \
	update-desktop-database "$apps" 2>/dev/null || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && \
	gtk-update-icon-cache -q -t "$prefix/share/icons/hicolor" 2>/dev/null || true

echo
case ":$PATH:" in
*":$bin:"*) echo "done — run: gscore" ;;
*)          echo "done — run: $bin/gscore"
            echo "($bin is not on your PATH; add it to run gscore by name)" ;;
esac
