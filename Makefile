# gscore — building and packaging.
#
#   make              build and package for this machine
#   make dmg          a macOS disk image
#   make app          a macOS application bundle, unpackaged
#   make windows      a Windows .exe, cross-compiled from here
#   make debian       a .deb, cross-compiled from here
#   make web          the static page, for GitHub Pages
#   make all          every package above
#   make test         vet and the test suite
#   make run          build and run
#   make clean
#
# Everything the program needs is inside the binary: the interface, the music
# font and the SMuFL glyph names are all embedded, so a build is one file with
# nothing to install beside it.
#
# One thing differs between platforms and it is worth knowing before you ship.
# The native window wraps the system's own web view through cgo, which means
# the platform's headers have to be present at build time:
#
#   macOS     WebKit, part of the system          → native window
#   Windows   WebView2, via mingw-w64             → native window
#   Linux     WebKitGTK, not available on a Mac   → opens a browser instead
#
# So a Linux build made here is complete and self-contained, but shows the
# interface in the default browser rather than a window of its own. Built on
# a Linux machine with libwebkit2gtk-4.0-dev present it gets the window too,
# and `make debian` says as much while it runs.
#
# (C) Copyright 2026 Sebastien Tricaud. GPL v2 or later.

BINARY  := gscore
VERSION := $(shell sed -n 's/.*Version = "\(.*\)".*/\1/p' internal/version/version.go)
BUILD   := build
LDFLAGS := -s -w

HOST_OS := $(shell go env GOOS)

.DEFAULT_GOAL := default
.PHONY: default all build run test vet clean web app dmg windows debian help

## default: package for this machine
default:
ifeq ($(HOST_OS),darwin)
	@$(MAKE) --no-print-directory dmg
else ifeq ($(HOST_OS),linux)
	@$(MAKE) --no-print-directory debian
else
	@$(MAKE) --no-print-directory build
endif

## all: every package
all: dmg windows debian web

## build: a binary for this machine
build:
	@mkdir -p $(BUILD)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY) .
	@echo "built $(BUILD)/$(BINARY) ($(shell du -h $(BUILD)/$(BINARY) 2>/dev/null | cut -f1))"

## run: build and run
run: build
	./$(BUILD)/$(BINARY)

## test: what the CI runs
test: vet
	go test ./...

vet:
	CGO_ENABLED=0 go vet ./...

# ── macOS ───────────────────────────────────────────────────────────────────

# The Mac application is not the Go binary in a folder. It is an AppKit
# application — a menu bar, panels, documents, a window of its own — written
# in Swift under ui/macosx, with the Go core linked into it as a library.
# Everything about music stays in Go and is tested headlessly; everything
# about being a Mac application stays in Swift, where AppKit already knows
# the answers.
#
# So a bundle is built in three moves: the core as a C archive, the
# application against it, and the two architectures joined with lipo.

# The bundle carries the name as it is written for somebody to read, while
# the executable inside it keeps the command's own name: in the Finder and
# the menu bar this is Gscore, and in a terminal it is still gscore.
APP_NAME := Gscore
APP      := $(BUILD)/$(APP_NAME).app
DMG      := $(BUILD)/$(APP_NAME)-$(VERSION).dmg
MAC_ARCH := arm64 amd64
MAC_MIN  := 13.0
MAC_SRC  := $(wildcard ui/macosx/Sources/*.swift)
ICONSET  := $(BUILD)/gscore.iconset
# The icon is made from the PNG rather than kept as a second copy in .icns:
# one drawing, and every size generated from it.
ICON_PNG := assets/icon/gscore.png

## app: a macOS application bundle
app: $(BUILD)/gscore.icns
	@test "$(HOST_OS)" = darwin || { echo "a .app can only be built on macOS" >&2; exit 1; }
	@command -v swiftc >/dev/null || { \
		echo "need the Swift compiler: xcode-select --install" >&2; exit 1; }
	@rm -rf $(APP)
	@mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources
	@for arch in $(MAC_ARCH); do \
		echo "  building for $$arch"; \
		ui/macosx/build.sh $$arch $(MAC_MIN) $(BUILD)/$(BINARY)-app-$$arch; \
	done
	@lipo -create -output $(APP)/Contents/MacOS/$(BINARY) \
		$(foreach a,$(MAC_ARCH),$(BUILD)/$(BINARY)-app-$(a))
	@rm -f $(foreach a,$(MAC_ARCH),$(BUILD)/$(BINARY)-app-$(a))
	@cp $(BUILD)/gscore.icns $(APP)/Contents/Resources/
	@# From a real file, not from a make variable. GNU make 3.81 — which is
	@# the make macOS ships — runs each line of a recipe in its own shell,
	@# and a variable holding a multi-line document is split across them:
	@# the first line ends inside an unclosed quote and the build stops with
	@# a syntax error nobody can trace back to a plist.
	@sed -e 's/@VERSION@/$(VERSION)/g' -e 's/@MIN@/$(MAC_MIN)/g' \
		ui/macosx/Info.plist.in > $(APP)/Contents/Info.plist
	@# Unsigned, the Finder will not open a downloaded bundle. An ad-hoc
	@# signature is enough for a build you run yourself, and is what makes
	@# the file associations register at all.
	@codesign --force --deep --sign - $(APP) 2>/dev/null || \
		echo "  (could not sign; the bundle still runs from here)"
	@echo "built $(APP) ($(shell du -sh $(APP) 2>/dev/null | cut -f1))"

# The .icns the bundle carries, generated from the one PNG.
$(BUILD)/gscore.icns: $(ICON_PNG)
	@mkdir -p $(ICONSET)
	@for s in 16 32 128 256 512; do \
		sips -z $$s $$s $(ICON_PNG) --out $(ICONSET)/icon_$${s}x$${s}.png >/dev/null; \
		d=$$((s * 2)); \
		sips -z $$d $$d $(ICON_PNG) --out $(ICONSET)/icon_$${s}x$${s}@2x.png >/dev/null; \
	done
	@iconutil -c icns $(ICONSET) -o $@
	@rm -rf $(ICONSET)
	@echo "built $@ from $(ICON_PNG)"

## dmg: a macOS disk image
dmg: app
	@rm -f $(DMG)
	@rm -rf $(BUILD)/dmg && mkdir -p $(BUILD)/dmg
	@cp -R $(APP) $(BUILD)/dmg/
	@# A link to Applications, so the image is a drag-and-drop installer.
	@ln -s /Applications $(BUILD)/dmg/Applications
	@hdiutil create -quiet -volname "$(APP_NAME) $(VERSION)" \
		-srcfolder $(BUILD)/dmg -ov -format UDZO $(DMG)
	@rm -rf $(BUILD)/dmg
	@echo "built $(DMG) ($(shell du -h $(DMG) 2>/dev/null | cut -f1))"

# ── Windows ─────────────────────────────────────────────────────────────────

WIN_DIR := $(BUILD)/windows
WIN_CC  := x86_64-w64-mingw32-gcc

## windows: a Windows .exe, cross-compiled
windows:
	@command -v $(WIN_CC) >/dev/null || { \
		echo "need mingw-w64 for the native window: brew install mingw-w64" >&2; exit 1; }
	@mkdir -p $(WIN_DIR)
	@# -H windowsgui keeps a console from opening behind the window.
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		CC=$(WIN_CC) CXX=x86_64-w64-mingw32-g++ \
		go build -trimpath -ldflags "$(LDFLAGS) -H windowsgui" \
		-o $(WIN_DIR)/$(BINARY).exe . 2>&1 | grep -v '^In file\|warning:\|^ *[0-9]* |\|^ *|' || true
	@test -f $(WIN_DIR)/$(BINARY).exe
	@cp COPYING $(WIN_DIR)/LICENSE.txt
	@cd $(BUILD) && zip -qr $(BINARY)-$(VERSION)-windows.zip windows
	@echo "built $(BUILD)/$(BINARY)-$(VERSION)-windows.zip ($(shell du -h $(BUILD)/$(BINARY)-$(VERSION)-windows.zip 2>/dev/null | cut -f1))"

# ── Debian ──────────────────────────────────────────────────────────────────

DEB_ROOT := $(BUILD)/deb
DEB      := $(BUILD)/$(BINARY)_$(VERSION)_amd64.deb

## debian: a .deb, cross-compiled
debian:
	@command -v dpkg-deb >/dev/null || { \
		echo "need dpkg: brew install dpkg" >&2; exit 1; }
	@rm -rf $(DEB_ROOT)
	@mkdir -p $(DEB_ROOT)/DEBIAN \
		$(DEB_ROOT)/usr/bin \
		$(DEB_ROOT)/usr/share/applications \
		$(DEB_ROOT)/usr/share/icons/hicolor/192x192/apps \
		$(DEB_ROOT)/usr/share/doc/$(BINARY)
	@# Without cgo, since WebKitGTK's headers are not on this machine. The
	@# result is static and depends on nothing; it opens the interface in the
	@# default browser rather than in a window of its own.
	@echo "  cross-compiling without cgo: this build opens a browser, not a window"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags "$(LDFLAGS)" -o $(DEB_ROOT)/usr/bin/$(BINARY) .
	@cp assets/icon/gscore.png $(DEB_ROOT)/usr/share/icons/hicolor/192x192/apps/$(BINARY).png
	@cp COPYING $(DEB_ROOT)/usr/share/doc/$(BINARY)/copyright
	@printf '%s' '$(DESKTOP_FILE)' > $(DEB_ROOT)/usr/share/applications/$(BINARY).desktop
	@printf '%s' '$(DEB_CONTROL)' > $(DEB_ROOT)/DEBIAN/control
	@chmod 755 $(DEB_ROOT)/usr/bin/$(BINARY)
	@dpkg-deb --root-owner-group --build $(DEB_ROOT) $(DEB) >/dev/null
	@rm -rf $(DEB_ROOT)
	@echo "built $(DEB) ($(shell du -h $(DEB) 2>/dev/null | cut -f1))"

# ── the web page ────────────────────────────────────────────────────────────

## web: the static page
web:
	@sh web/build.sh $(BUILD)/web

## clean: remove everything built
clean:
	rm -rf $(BUILD) web/dist

## help: this list
help:
	@sed -n 's/^## //p' $(MAKEFILE_LIST)

# ── the files that go inside the packages ───────────────────────────────────


define DESKTOP_FILE
[Desktop Entry]
Type=Application
Name=Gscore
GenericName=Music score editor
Comment=A musical score editor
Exec=gscore %f
Icon=gscore
Terminal=false
Categories=AudioVideo;Audio;Music;
MimeType=application/x-gscore;text/vnd.abc;application/vnd.recordare.musicxml+xml;
endef

define DEB_CONTROL
Package: gscore
Version: $(VERSION)
Section: sound
Priority: optional
Architecture: amd64
Maintainer: Sebastien Tricaud <toady@gscore.org>
Homepage: https://github.com/stricaud/gscore
Description: A musical score editor
 gscore writes, reads and plays musical scores. It reads ABC, MusicXML and
 NoteWorthy Composer files and has a format of its own that carries
 hand-drawn symbols along with the music.
 .
 The interface is shown in the system's web browser. For a window of its
 own, build on a machine with libwebkit2gtk-4.0-dev installed.
endef
