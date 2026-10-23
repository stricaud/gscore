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

APP      := $(BUILD)/$(BINARY).app
DMG      := $(BUILD)/$(BINARY)-$(VERSION).dmg
MAC_ARCH := arm64 amd64

## app: a macOS application bundle
app:
	@test "$(HOST_OS)" = darwin || { echo "a .app can only be built on macOS" >&2; exit 1; }
	@rm -rf $(APP)
	@mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources
	@# One binary for both kinds of Mac, so the bundle runs anywhere.
	@for arch in $(MAC_ARCH); do \
		echo "  compiling for $$arch"; \
		CGO_ENABLED=1 GOOS=darwin GOARCH=$$arch \
			go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-$$arch . ; \
	done
	@lipo -create -output $(APP)/Contents/MacOS/$(BINARY) \
		$(foreach a,$(MAC_ARCH),$(BUILD)/$(BINARY)-$(a))
	@rm -f $(foreach a,$(MAC_ARCH),$(BUILD)/$(BINARY)-$(a))
	@cp assets/icon/gscore.icns $(APP)/Contents/Resources/
	@printf '%s' '$(INFO_PLIST)' > $(APP)/Contents/Info.plist
	@echo "built $(APP) ($(shell du -sh $(APP) 2>/dev/null | cut -f1))"

## dmg: a macOS disk image
dmg: app
	@rm -f $(DMG)
	@rm -rf $(BUILD)/dmg && mkdir -p $(BUILD)/dmg
	@cp -R $(APP) $(BUILD)/dmg/
	@# A link to Applications, so the image is a drag-and-drop installer.
	@ln -s /Applications $(BUILD)/dmg/Applications
	@hdiutil create -quiet -volname "$(BINARY) $(VERSION)" \
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

define INFO_PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>gscore</string>
  <key>CFBundleDisplayName</key><string>gscore</string>
  <key>CFBundleIdentifier</key><string>org.gscore.gscore</string>
  <key>CFBundleVersion</key><string>$(VERSION)</string>
  <key>CFBundleShortVersionString</key><string>$(VERSION)</string>
  <key>CFBundleExecutable</key><string>gscore</string>
  <key>CFBundleIconFile</key><string>gscore.icns</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>LSMinimumSystemVersion</key><string>10.15</string>
  <key>NSHumanReadableCopyright</key><string>GPL v2 or later</string>
  <key>CFBundleDocumentTypes</key>
  <array>
    <dict>
      <key>CFBundleTypeName</key><string>gscore score</string>
      <key>CFBundleTypeRole</key><string>Editor</string>
      <key>CFBundleTypeIconFile</key><string>gscore.icns</string>
      <key>CFBundleTypeExtensions</key>
      <array><string>gscore</string></array>
    </dict>
    <dict>
      <key>CFBundleTypeName</key><string>Music score</string>
      <key>CFBundleTypeRole</key><string>Editor</string>
      <key>CFBundleTypeIconFile</key><string>gscore.icns</string>
      <key>CFBundleTypeExtensions</key>
      <array>
        <string>abc</string><string>musicxml</string><string>mxl</string>
        <string>nwc</string><string>nwctxt</string>
      </array>
    </dict>
  </array>
</dict>
</plist>
endef

define DESKTOP_FILE
[Desktop Entry]
Type=Application
Name=gscore
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
