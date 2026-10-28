# Makefile for my-capture.
#
# Run "make" or "make help" to list the targets and the variables that
# change how they behave.
#
# The program uses cgo (Fyne and miniaudio), so it cannot be cross-compiled
# with the Go toolchain alone: the macOS artefact must be built on a Mac, and
# the Windows artefact needs a Windows C compiler (MinGW-w64), either
# cross-compiling from macOS or Linux or running natively on Windows.
#
# Written for GNU make 3.81, the version that ships with macOS.

# -----------------------------------------------------------------------
# Settings
# -----------------------------------------------------------------------

# APP is the name of the executable and the stem of the artefact file names.
APP := my-capture

# APP_BUNDLE is the name of the macOS application bundle. It contains a
# space, so every use below is quoted.
APP_BUNDLE := My Capture.app

# BIN_DIR holds the development build; DIST_DIR holds the release artefacts.
BIN_DIR  := bin
DIST_DIR := dist

# GO is the Go command, overridable for a specific toolchain.
GO ?= go

# VERSION names the artefacts and is shown as the macOS bundle version. It
# defaults to the nearest git tag (or the commit), and to "dev" outside git.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# BUNDLE_VERSION is VERSION without a leading "v", as macOS expects.
BUNDLE_VERSION := $(patsubst v%,%,$(VERSION))

# BUNDLE_FFMPEG: with 1 (the default) the release artefacts contain a small
# LGPL-licensed ffmpeg, built from source by packaging/ffmpeg/build.sh the
# first time and kept in FFMPEG_DIR, so users do not have to install ffmpeg.
# The program looks for ffmpeg next to itself first. With any other value the
# artefacts contain no ffmpeg (a quick local build).
BUNDLE_FFMPEG ?= 1
FFMPEG_DIR    := build/ffmpeg

# FFMPEG_BIN and FFMPEG_EXE are optional paths of an ffmpeg executable to
# copy into the macOS bundle and the Windows archive instead of the one built
# above. Their licence notices are then the responsibility of whoever sets them.
FFMPEG_BIN ?=
FFMPEG_EXE ?=

# MACOS_FFMPEG_BUILT and WINDOWS_FFMPEG_BUILT are the paths of the ffmpeg built
# from source, set only when it is to be built and bundled.
ifeq ($(BUNDLE_FFMPEG),1)
ifeq ($(FFMPEG_BIN),)
MACOS_FFMPEG_BUILT := $(FFMPEG_DIR)/macos/ffmpeg
FFMPEG_BIN         := $(MACOS_FFMPEG_BUILT)
endif
ifeq ($(FFMPEG_EXE),)
WINDOWS_FFMPEG_BUILT := $(FFMPEG_DIR)/windows/ffmpeg.exe
FFMPEG_EXE           := $(WINDOWS_FFMPEG_BUILT)
endif
endif

# MACOS_ARCHS lists the architectures in the macOS build. Two entries make a
# universal binary that runs natively on Apple silicon and Intel Macs.
MACOS_ARCHS ?= arm64 amd64

# MACOS_MIN is the oldest macOS version the build supports. It must match
# LSMinimumSystemVersion in assets/macos/Info.plist.
MACOS_MIN ?= 12.0

# SIGN_IDENTITY is the code signing identity for the macOS bundle. "-" means
# an ad-hoc signature, which is enough to run on the Mac that built it and to
# let macOS ask for camera and microphone access.
SIGN_IDENTITY ?= -

# MINGW_CC is the Windows C compiler used for dist-windows when the build
# runs on macOS or Linux. On Windows itself plain gcc is used.
MINGW_CC ?= x86_64-w64-mingw32-gcc

# Every build records VERSION in the program (shown by "my-capture -version")
# so a bug report can say exactly which release it came from. Release builds
# also drop the symbol table and debug information to save space.
VERSION_FLAG := -X main.version=$(VERSION)
LDFLAGS      := -s -w $(VERSION_FLAG)

# On Windows (MSYS2, Git Bash) the C compiler is the native gcc.
HOST_OS := $(shell uname -s)
ifneq (,$(findstring MINGW,$(HOST_OS))$(findstring MSYS,$(HOST_OS)))
WINDOWS_CC := gcc
else
WINDOWS_CC := $(MINGW_CC)
endif

# Artefact paths.
MACOS_DIR   := $(DIST_DIR)/macos
MACOS_ZIP   := $(DIST_DIR)/$(APP)-$(VERSION)-macos-universal.zip
WINDOWS_DIR := $(DIST_DIR)/windows/$(APP)
WINDOWS_ZIP := $(DIST_DIR)/$(APP)-$(VERSION)-windows-amd64.zip

.DEFAULT_GOAL := help

.PHONY: help build run fmt fmt-check vet tidy icons test test-race test-e2e check \
        dist dist-macos dist-windows clean clean-ffmpeg ffmpeg-macos ffmpeg-windows \
        require-macos require-mingw

# -----------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------

define HELP_TEXT
my-capture build targets

Usage: make [target] [VARIABLE=value ...]

Development:
  build         Build bin/my-capture for this computer.
  run           Build, then start the program.
  fmt           Format all Go code with gofmt.
  fmt-check     Fail if any Go file is not formatted (changes nothing).
  vet           Run go vet.
  tidy          Run go mod tidy.
  icons         Redraw the application icon (tools/genicon) and rebuild
                assets/icon/icon.icns and rsrc_windows_amd64.syso from it.
                Only needed after changing the icon design; the results are
                kept in the repository. Must run on a Mac (uses iconutil).
  clean         Delete bin/ and dist/.
  clean-ffmpeg  Delete the ffmpeg built for the release artefacts (build/), so
                it is built again next time. Needed after editing
                packaging/ffmpeg/build.sh.

Tests:
  test          Run the unit tests (go test ./...).
  test-race     Run the unit tests with the race detector.
  test-e2e      Run the end-to-end tests in tests/e2e, which need a real
                capture device and are left out of "make test". Does
                nothing if that directory does not exist.
  check         fmt-check, vet and test together. Run this before a release.

Release artefacts (written to dist/):
  dist-macos    A universal "My Capture.app" in a zip, with ffmpeg inside.
                Must run on a Mac.
  dist-windows  A zip with my-capture.exe (64-bit Windows) and ffmpeg.exe.
                Needs MinGW-w64.
  dist          Both of the above. Needs a Mac with MinGW-w64 installed.
  ffmpeg-macos  Build only the universal macOS ffmpeg that goes in the bundle.
  ffmpeg-windows  Build only the Windows ffmpeg.exe that goes in the zip.
                Both are built from the FFmpeg source release named in
                packaging/ffmpeg/build.sh and take a few minutes the first
                time. They need nasm (macOS: brew install nasm), and for the
                Windows one a MinGW-w64 compiler.

Variables (set on the command line, or as environment variables):
  VERSION       Names the artefacts, is the version inside the macOS bundle,
                and is what "my-capture -version" prints. Default: the
                nearest git tag or commit, else "dev". Current: $(VERSION)
  BUNDLE_FFMPEG 1 puts a small LGPL ffmpeg, built from source, in the macOS
                bundle and the Windows zip, so users do not have to install
                ffmpeg. Any other value leaves it out, for a faster local
                build. Default: 1.
  FFMPEG_BIN    Path of an ffmpeg executable to copy into the macOS bundle
                instead of the one built here. Default: empty (use the built
                one). Check that ffmpeg's licence allows you to redistribute it.
  FFMPEG_EXE    Path of ffmpeg.exe to copy into the Windows zip instead of the
                built one. Default: empty. The same licence advice applies.
  MACOS_ARCHS   Architectures in the macOS binary. Default: "arm64 amd64"
                (universal). Use MACOS_ARCHS=arm64 for a faster build that
                only runs on Apple silicon.
  MACOS_MIN     Oldest macOS version supported. Default: 12.0.
  SIGN_IDENTITY Code signing identity for the macOS bundle. Default: "-"
                (ad-hoc signature). Set it to a Developer ID such as
                "Developer ID Application: Name (TEAMID)" for a bundle that
                other Macs accept; it is then signed with the hardened
                runtime and the entitlements in packaging/macos. Notarization
                is not done by this Makefile.
  MINGW_CC      Windows C compiler used when building on macOS or Linux.
                Default: x86_64-w64-mingw32-gcc. On macOS install it with
                "brew install mingw-w64".
  GO            The Go command. Default: go.

Notes:
  The cgo requirement means each artefact has to be built on a matching
  system or with a matching C compiler; see the top of this Makefile.
endef
export HELP_TEXT

help:
	@printf '%s\n' "$$HELP_TEXT"

# -----------------------------------------------------------------------
# Development
# -----------------------------------------------------------------------

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(VERSION_FLAG)" -o $(BIN_DIR)/$(APP) .

run: build
	./$(BIN_DIR)/$(APP)

fmt:
	gofmt -w .

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt (run: make fmt):"; echo "$$unformatted"; exit 1; \
	fi

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

# icons draws the icon with tools/genicon, turns the macOS image set into an
# icns file with iconutil, and turns the Windows .ico into a resource object
# (.syso) that Go links into the Windows executable automatically, because
# its name ends in _windows_amd64.
icons:
	$(GO) run ./tools/genicon
	iconutil -c icns assets/icon/icon.iconset -o assets/icon/icon.icns
	rm -rf assets/icon/icon.iconset
	$(GO) run github.com/akavel/rsrc@v0.10.2 -arch amd64 -ico assets/icon/icon.ico -o rsrc_windows_amd64.syso

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

clean-ffmpeg:
	rm -rf build

# The ffmpeg binaries are built by packaging/ffmpeg/build.sh. They have no
# prerequisites on purpose: once built they are reused, which matters because
# building takes minutes and the CI cache restores files with old timestamps.
# After editing the script, run "make clean-ffmpeg".
$(FFMPEG_DIR)/macos/ffmpeg:
	MACOS_MIN=$(MACOS_MIN) packaging/ffmpeg/build.sh macos $(FFMPEG_DIR)/macos

$(FFMPEG_DIR)/windows/ffmpeg.exe:
	packaging/ffmpeg/build.sh windows $(FFMPEG_DIR)/windows

ffmpeg-macos: $(FFMPEG_DIR)/macos/ffmpeg

ffmpeg-windows: $(FFMPEG_DIR)/windows/ffmpeg.exe

# -----------------------------------------------------------------------
# Tests
# -----------------------------------------------------------------------

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

# End-to-end tests carry the "e2e" build tag so that "go test ./..." skips
# them. -count=1 stops Go from replaying a cached result for a real device.
test-e2e:
	@if [ -d tests/e2e ]; then \
		$(GO) test -tags e2e -count=1 ./tests/e2e/...; \
	else \
		echo "No end-to-end tests: tests/e2e does not exist."; \
	fi

check: fmt-check vet test

# -----------------------------------------------------------------------
# Release artefacts
# -----------------------------------------------------------------------

dist: dist-macos dist-windows

require-macos:
	@if [ "$(HOST_OS)" != "Darwin" ]; then \
		echo "dist-macos must run on a Mac: the program uses cgo and the macOS SDK."; exit 1; \
	fi

require-mingw:
	@if ! command -v $(WINDOWS_CC) >/dev/null 2>&1; then \
		echo "$(WINDOWS_CC) was not found. dist-windows needs a Windows C compiler."; \
		echo "  macOS:   brew install mingw-w64"; \
		echo "  Windows: install MSYS2 and its mingw-w64 gcc, then run make there."; \
		exit 1; \
	fi

# dist-macos builds one binary per architecture, joins them into a universal
# binary with lipo, wraps it in an application bundle, signs the bundle and
# zips it with ditto (zip would lose the symbolic links and permissions that
# signed bundles rely on). --norsrc leaves out the "._" shadow files for
# extended attributes, which only confuse people unzipping on Windows.
dist-macos: require-macos $(MACOS_FFMPEG_BUILT)
	rm -rf "$(MACOS_DIR)"
	mkdir -p "$(MACOS_DIR)/$(APP_BUNDLE)/Contents/MacOS" "$(MACOS_DIR)/$(APP_BUNDLE)/Contents/Resources"
	cp assets/icon/icon.icns "$(MACOS_DIR)/$(APP_BUNDLE)/Contents/Resources/icon.icns"
	for arch in $(MACOS_ARCHS); do \
		echo "Building macOS $$arch"; \
		MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN) CGO_ENABLED=1 GOOS=darwin GOARCH=$$arch \
			$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$(MACOS_DIR)/$(APP)-$$arch" . || exit 1; \
	done
	lipo -create -output "$(MACOS_DIR)/$(APP_BUNDLE)/Contents/MacOS/$(APP)" \
		$(foreach arch,$(MACOS_ARCHS),"$(MACOS_DIR)/$(APP)-$(arch)")
	rm -f $(foreach arch,$(MACOS_ARCHS),"$(MACOS_DIR)/$(APP)-$(arch)")
	sed 's/@VERSION@/$(BUNDLE_VERSION)/g' assets/macos/Info.plist \
		> "$(MACOS_DIR)/$(APP_BUNDLE)/Contents/Info.plist"
	@if [ -n "$(FFMPEG_BIN)" ]; then \
		echo "Bundling $(FFMPEG_BIN)"; \
		cp "$(FFMPEG_BIN)" "$(MACOS_DIR)/$(APP_BUNDLE)/Contents/MacOS/ffmpeg"; \
	fi
	@if [ -n "$(MACOS_FFMPEG_BUILT)" ]; then \
		mkdir -p "$(MACOS_DIR)/$(APP_BUNDLE)/Contents/Resources/ffmpeg-licenses"; \
		cp $(FFMPEG_DIR)/macos/COPYING.LGPLv2.1 $(FFMPEG_DIR)/macos/FFMPEG-NOTICE.txt \
			"$(MACOS_DIR)/$(APP_BUNDLE)/Contents/Resources/ffmpeg-licenses/"; \
	fi
	@if [ "$(SIGN_IDENTITY)" = "-" ]; then \
		codesign --force --deep --sign - "$(MACOS_DIR)/$(APP_BUNDLE)"; \
	else \
		codesign --force --deep --timestamp --options runtime \
			--entitlements packaging/macos/entitlements.plist \
			--sign "$(SIGN_IDENTITY)" "$(MACOS_DIR)/$(APP_BUNDLE)"; \
	fi
	rm -f "$(MACOS_ZIP)"
	ditto -c -k --norsrc --keepParent "$(MACOS_DIR)/$(APP_BUNDLE)" "$(MACOS_ZIP)"
	@echo "Built $(MACOS_ZIP)"

# dist-windows cross-compiles with MinGW-w64. -H=windowsgui marks the program
# as a GUI program so Windows does not open a console window behind it.
dist-windows: require-mingw $(WINDOWS_FFMPEG_BUILT)
	rm -rf "$(WINDOWS_DIR)"
	mkdir -p "$(WINDOWS_DIR)"
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(WINDOWS_CC) \
		$(GO) build -trimpath -ldflags "$(LDFLAGS) -H=windowsgui" \
		-o "$(WINDOWS_DIR)/$(APP).exe" .
	cp README.md "$(WINDOWS_DIR)/README.md"
	@if [ -n "$(FFMPEG_EXE)" ]; then \
		echo "Bundling $(FFMPEG_EXE)"; \
		cp "$(FFMPEG_EXE)" "$(WINDOWS_DIR)/ffmpeg.exe"; \
	fi
	@if [ -n "$(WINDOWS_FFMPEG_BUILT)" ]; then \
		mkdir -p "$(WINDOWS_DIR)/ffmpeg-licenses"; \
		cp $(FFMPEG_DIR)/windows/COPYING.LGPLv2.1 $(FFMPEG_DIR)/windows/FFMPEG-NOTICE.txt \
			"$(WINDOWS_DIR)/ffmpeg-licenses/"; \
	fi
	rm -f "$(WINDOWS_ZIP)"
	cd "$(DIST_DIR)/windows" && zip -qr "$(CURDIR)/$(WINDOWS_ZIP)" "$(APP)"
	@echo "Built $(WINDOWS_ZIP)"
