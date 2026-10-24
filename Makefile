# gscore — building and packaging.
#
#   make              build and package for this machine
#   make dmg          a macOS disk image
#   make app          a macOS application bundle, unpackaged
#   make windows      a Windows .exe, cross-compiled from here
#   make debian       a .deb, cross-compiled from here
#   make soundfonts   fetch the instruments the packages ship with
#
# The macOS and Windows packages carry a General MIDI soundfont — thirty
# megabytes of recorded instruments — so that the application can play
# something the first time it is opened. It is fetched once into build/ and
# kept there. See BUNDLE_FONTS below for which, and why the ZanderJaz
# collection is not among them.
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

# What "make package" comes to on this machine, for the help to say so
# rather than leaving somebody to read the conditional below.
ifeq ($(HOST_OS),darwin)
PACKAGE_FOR := a macOS disk image
else ifeq ($(HOST_OS),linux)
PACKAGE_FOR := a Debian package
else
PACKAGE_FOR := a bare binary
endif

# Typing make on its own prints the list. Building is a choice between a
# disk image, an installer, a web page and a bare binary, and guessing which
# one somebody meant from the machine they are sitting at is how you end up
# waiting on a two-architecture Swift build you did not ask for.
.DEFAULT_GOAL := help
.PHONY: default package all build run test vet clean web app dmg windows \
	debian help soundfonts

## package: build the package this machine is for
package default:
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
	@echo "built $(BUILD)/$(BINARY) ($$(du -h $(BUILD)/$(BINARY) | cut -f1))"

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

# The instruments the application is shipped with.
#
# gscore plays through a soundfont — somebody's recordings of real
# instruments — and carries none in the binary, because thirty megabytes of
# samples has no business inside an editor that most people will use to write
# a tune and play it themselves. The application bundle is the exception: it
# is a download already, and an application that cannot make a sound until
# you have found a soundfont is an application that cannot make a sound.
#
# Only what may be redistributed. GeneralUser GS says so in as many words.
# FluidR3 GM is MIT and may be added with BUNDLE_FONTS="generaluser fluidr3",
# at the cost of 148MB in the disk image. The ZanderJaz collection is not
# here and will not be: it is five hundred files of mixed provenance, a good
# many of them lifted from Roland, E-Mu and Korg ROMs, and shipping those in
# a signed application is not the same act as a user fetching one for
# themselves. They stay a download, which is what the dialog offers.
BUNDLE_FONTS ?= generaluser
FONT_DIR     := $(BUILD)/soundfonts

generaluser_URL := https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/main/GeneralUser-GS.sf2
generaluser_SF2 := $(FONT_DIR)/GeneralUser GS.sf2
fluidr3_URL     := https://www.zanderjaz.com/soundfonts/packs/FluidR3%20GM.SF2
fluidr3_SF2     := $(FONT_DIR)/FluidR3 GM.sf2

## soundfonts: fetch the instruments the packages are shipped with
soundfonts:
	@mkdir -p $(FONT_DIR)
	@for name in $(BUNDLE_FONTS); do \
		url=$$(make --no-print-directory print-$${name}_URL); \
		out=$$(make --no-print-directory print-$${name}_SF2); \
		if [ -s "$$out" ]; then \
			echo "  $$name: already here"; \
		else \
			echo "  $$name: downloading"; \
			curl -fsSL "$$url" -o "$$out.part" && mv "$$out.part" "$$out" || { \
				rm -f "$$out.part"; \
				echo "could not download $$name from $$url" >&2; exit 1; }; \
		fi; \
	done

# Reading a variable from a recipe, which is the only way to index one by a
# name that is only known while the loop is running.
print-%:
	@echo '$($*)'

## app: a macOS application bundle
app: $(BUILD)/gscore.icns soundfonts
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
	@# Beside the executable, where the program looks for them: a bundle that
	@# is moved or copied takes its instruments with it.
	@mkdir -p "$(APP)/Contents/Resources/soundfonts"
	@for name in $(BUNDLE_FONTS); do \
		cp "$$(make --no-print-directory print-$${name}_SF2)" \
			"$(APP)/Contents/Resources/soundfonts/"; \
	done
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
	@echo "built $(APP) ($$(du -sh $(APP) | cut -f1)), carrying:"
	@for name in $(BUNDLE_FONTS); do \
		sf=$$(make --no-print-directory print-$${name}_SF2); \
		echo "  $$(basename "$$sf") ($$(du -h "$$sf" | cut -f1)) — the instruments it plays through"; \
	done

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
	@echo "built $(DMG) ($$(du -h $(DMG) | cut -f1), from $$(du -sh $(APP) | cut -f1) of application)"

# ── Windows ─────────────────────────────────────────────────────────────────

WIN_DIR := $(BUILD)/windows
WIN_CC  := x86_64-w64-mingw32-gcc

## windows: a Windows .exe, cross-compiled
windows: soundfonts
	@command -v $(WIN_CC) >/dev/null || { \
		echo "need mingw-w64 for the native window: brew install mingw-w64" >&2; exit 1; }
	@mkdir -p $(WIN_DIR)
	@# -H windowsgui keeps a console from opening behind the window.
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		CC=$(WIN_CC) CXX=x86_64-w64-mingw32-g++ \
		go build -trimpath -ldflags "$(LDFLAGS) -H windowsgui" \
		-o $(WIN_DIR)/$(BINARY).exe . 2>&1 | grep -v '^In file\|warning:\|^ *[0-9]* |\|^ *|' || true
	@test -f $(WIN_DIR)/$(BINARY).exe
	@# Beside the executable, which is where the program looks: a folder
	@# somebody unzipped and runs from is as much of an installation as
	@# Windows gets here, and the instruments have to be in it.
	@mkdir -p $(WIN_DIR)/soundfonts
	@for name in $(BUNDLE_FONTS); do \
		cp "$$(make --no-print-directory print-$${name}_SF2)" $(WIN_DIR)/soundfonts/; \
	done
	@cp COPYING $(WIN_DIR)/LICENSE.txt
	@cd $(BUILD) && zip -qr $(BINARY)-$(VERSION)-windows.zip windows
	@echo "built $(BUILD)/$(BINARY)-$(VERSION)-windows.zip ($$(du -h $(BUILD)/$(BINARY)-$(VERSION)-windows.zip | cut -f1))"

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
	@# From real files, for the same reason the Info.plist is: GNU make
	@# 3.81 runs each recipe line in its own shell, and a variable holding
	@# a multi-line document is split across them — the first line ends
	@# inside an unclosed quote and the build stops with a syntax error
	@# nobody can trace back to a desktop entry.
	@cp packaging/debian/gscore.desktop $(DEB_ROOT)/usr/share/applications/$(BINARY).desktop
	@sed -e 's/@VERSION@/$(VERSION)/g' packaging/debian/control.in > $(DEB_ROOT)/DEBIAN/control
	@chmod 755 $(DEB_ROOT)/usr/bin/$(BINARY)
	@dpkg-deb --root-owner-group --build $(DEB_ROOT) $(DEB) >/dev/null
	@rm -rf $(DEB_ROOT)
	@echo "built $(DEB) ($$(du -h $(DEB) | cut -f1))"

# ── the web page ────────────────────────────────────────────────────────────

## web: the static page
web:
	@sh web/build.sh $(BUILD)/web

## clean: remove everything built
clean:
	rm -rf $(BUILD) web/dist

## help: this list
help:
	@echo "gscore $(VERSION) — make <target>"
	@echo
	@sed -n 's/^## /  /p' $(MAKEFILE_LIST)
	@echo
	@echo "  this machine is $(HOST_OS), so \"make package\" builds $(PACKAGE_FOR)"
	@echo "  the macOS and Windows packages carry a soundfont, so the"
	@echo "  application plays real instruments with nothing to fetch:"
	@echo "  $(BUNDLE_FONTS) (BUNDLE_FONTS=\"generaluser fluidr3\" adds"
	@echo "  FluidR3 GM, +148MB). The .deb does not: a Debian machine gets"
	@echo "  one from Debian, and fluid-soundfont-gm is already packaged."

# ── the files that go inside the packages ───────────────────────────────────


# The .desktop entry and the Debian control file live in packaging/debian,
# as files rather than as variables here. See the recipe above for why.
