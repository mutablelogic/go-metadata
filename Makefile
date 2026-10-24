# Paths to programs
GO ?= $(shell which go)
PKG_CONFIG ?= $(shell which pkg-config)

# Locations
BUILD_DIR ?= build
CMD_DIR := $(wildcard cmd/*)
COVERAGE ?= $(BUILD_DIR)/coverage

# Set OS. Windows sets OS=Windows_NT in the environment, which ?= would keep,
# so replace it
ifeq ($(OS),Windows_NT)
OS := $(shell uname | tr A-Z a-z | sed -E 's/^(msys|mingw|cygwin).*/windows/')
else
OS ?= $(shell uname | tr A-Z a-z | sed -E 's/^(msys|mingw|cygwin).*/windows/')
endif
EXE := $(if $(filter windows,$(OS)),.exe)

# Build with this module's go.mod, rather than a go.work in a folder above
# it, unless GOWORK is set, such as GOWORK=../go.work
GOWORK ?= off
export GOWORK

# Arguments to go test, such as ARGS=-v or ARGS="-run Test_pdf"
ARGS ?=

# A comma, which can't be written in a function's arguments
comma := ,

###############################################################################
# CGO

# The C libraries of the handlers which use cgo, which are found with
# pkg-config: FFmpeg, for audio and video, libexif, for the EXIF metadata of
# photos, libheif, for HEIF and AVIF images, and LibRaw, for camera RAW images
FFMPEG_MODULES := libavcodec libavformat libavutil libswscale libswresample libavdevice libavfilter
CGO_MODULES := $(FFMPEG_MODULES) libexif libheif libraw

# The version of FFmpeg's libavformat which is FFmpeg 8, which go-media's
# bindings are for. FFmpeg 9 removed parts of its API which they use.
FFMPEG_VERSION := 62

# GO_MEDIA_PREFIX is where go-media built its static libraries, with make in
# a clone of it, such as ../go-media/build/install, which are used rather
# than the libraries which are installed, such as when they're too old
GO_MEDIA_PREFIX ?=
ifneq ($(GO_MEDIA_PREFIX),)
export PKG_CONFIG_PATH := $(abspath $(GO_MEDIA_PREFIX))/lib/pkgconfig$(if $(PKG_CONFIG_PATH),:$(PKG_CONFIG_PATH))
CGO_LDFLAGS += -lstdc++
endif

# The flags of the C libraries which cgo doesn't allow by default: the -W and
# -D flags of go-media's static libraries, and the -Xpreprocessor flag of
# LibRaw built with OpenMP, as Homebrew's is, which is before -fopenmp
CGO_CFLAGS_ALLOW ?= -Xpreprocessor
CGO_LDFLAGS_ALLOW ?= -Xpreprocessor|-(W|D).*
export CGO_CFLAGS_ALLOW CGO_LDFLAGS_ALLOW

# On macOS, Homebrew's ffmpeg@8, which is keg-only, is used rather than its
# ffmpeg, which is a later version, when it's installed. The linker warns
# about the libraries which more than one of the C libraries link, which
# isn't a problem.
ifeq ($(OS),darwin)
BREW ?= $(shell command -v brew 2>/dev/null)
BREW_FFMPEG := $(if $(BREW),$(wildcard $(shell $(BREW) --prefix ffmpeg@8 2>/dev/null)/lib/pkgconfig))
ifneq ($(BREW_FFMPEG),)
ifeq ($(GO_MEDIA_PREFIX),)
export PKG_CONFIG_PATH := $(BREW_FFMPEG)$(if $(PKG_CONFIG_PATH),:$(PKG_CONFIG_PATH))
endif
endif
CGO_LDFLAGS += -Wl,-no_warn_duplicate_libraries
endif
ifneq ($(strip $(CGO_LDFLAGS)),)
export CGO_LDFLAGS
endif

# pkg-config, with the folders of the C libraries above, as $(shell) doesn't
# have the variables which are exported in older versions of make, such as
# macOS's
PKGCONF := PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" $(PKG_CONFIG)

# The oldest versions of the C libraries which go-media's bindings compile
# against, as module:version, such as LibRaw 0.22, which added the JPEG XL
# and H.265 thumbnails which they use
CGO_MIN_VERSIONS := libraw:0.22

# The C libraries which aren't found, which are older than they need to be,
# and whether FFmpeg isn't FFmpeg 8
FFMPEG_NEXT := $(shell echo $$(($(FFMPEG_VERSION)+1)))
FFMPEG_INSTALLED := $(if $(PKG_CONFIG),$(shell $(PKGCONF) --modversion libavformat 2>/dev/null | sed 's/^/(libavformat /;s/$$/)/'))
CGO_MISSING := $(if $(PKG_CONFIG),$(shell for m in $(CGO_MODULES); do $(PKGCONF) --exists $$m || echo $$m; done),$(CGO_MODULES))
CGO_OLD := $(if $(PKG_CONFIG),$(shell for mv in $(CGO_MIN_VERSIONS); do m=$$(echo $$mv | cut -d: -f1); v=$$(echo $$mv | cut -d: -f2); $(PKGCONF) --exists $$m && ! $(PKGCONF) --atleast-version=$$v $$m && echo "$$m<$$v"; done))
FFMPEG_WRONG := $(if $(PKG_CONFIG),$(shell $(PKGCONF) --exists libavformat && ! ($(PKGCONF) --atleast-version=$(FFMPEG_VERSION) libavformat && ! $(PKGCONF) --atleast-version=$(FFMPEG_NEXT) libavformat) && echo yes))

# Build with cgo when every C library is found, and isn't too old, and FFmpeg
# is FFmpeg 8, or else without it, unless CGO_ENABLED is set
CGO_ENABLED ?= $(if $(CGO_MISSING)$(CGO_OLD)$(FFMPEG_WRONG),0,1)
export CGO_ENABLED

# Chromaprint, for the fingerprints of audio, which are looked up on AcoustID,
# is optional: with cgo, and when it's found, it's built with the chromaprint
# tag, or else audio isn't looked up on AcoustID
CHROMAPRINT := $(if $(filter 1,$(CGO_ENABLED)),$(if $(PKG_CONFIG),$(shell $(PKGCONF) --exists libchromaprint && echo yes)))
GO_TAGS ?= $(if $(CHROMAPRINT),chromaprint)
GO_TAGS_FLAG := $(if $(GO_TAGS),-tags "$(GO_TAGS)")

# What isn't read without cgo
NOCGO_NOTE := without cgo, audio and video, the EXIF metadata of photos, and HEIF and RAW images aren't read

###############################################################################
# TARGETS

.PHONY: all
all: cmd

# Build the commands, with cgo, when the C libraries are found
.PHONY: cmd
cmd: $(CMD_DIR)

$(CMD_DIR): go-dep cgo-dep mkdir
	@echo Build $(notdir $@) "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" build $(GO_TAGS_FLAG) -ldflags "-s -w" -o $(BUILD_DIR)/$(notdir $@)$(EXE) ./$@

# Build the commands without cgo, which don't read audio and video, the EXIF
# metadata of photos, or HEIF and RAW images
.PHONY: cmd-nocgo
cmd-nocgo:
	@$(MAKE) --no-print-directory cmd CGO_ENABLED=0

# Install the commands in GOBIN, or else GOPATH/bin
.PHONY: install
install: go-dep cgo-dep
	@echo Install $(notdir $(CMD_DIR)) "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" install $(GO_TAGS_FLAG) -ldflags "-s -w" $(addprefix ./,$(CMD_DIR))

###############################################################################
# TESTS

# The packages, which can each be tested, such as make test-image
TEST_PACKAGES := application audio image message text video pkg internal dublincore cmd

# Test everything
.PHONY: test
test: go-dep cgo-dep
	@echo Test "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" test $(GO_TAGS_FLAG) $(ARGS) ./...

# Test everything with the race detector, which needs cgo
.PHONY: test-race
test-race: go-dep cgo-dep
	@echo Test with the race detector "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" test $(GO_TAGS_FLAG) -race $(ARGS) ./...

# Test everything, and report the coverage of each function, and as an HTML
# page, $(COVERAGE).html
.PHONY: test-coverage
test-coverage: go-dep cgo-dep mkdir
	@echo Test with coverage "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" test $(GO_TAGS_FLAG) -covermode=atomic -coverpkg=./... -coverprofile=$(COVERAGE).out $(ARGS) ./...
	@"$(GO)" tool cover -func=$(COVERAGE).out | tail -1
	@"$(GO)" tool cover -html=$(COVERAGE).out -o $(COVERAGE).html
	@echo Coverage report $(COVERAGE).html

# Test everything without cgo, as it's built when the C libraries aren't
# found
.PHONY: test-nocgo
test-nocgo:
	@$(MAKE) --no-print-directory test CGO_ENABLED=0

# Test the core package, which detects content types and runs the handlers
.PHONY: test-core
test-core: go-dep cgo-dep
	@echo Test core "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" test $(GO_TAGS_FLAG) $(ARGS) .

# Test a package and the packages in it, such as make test-image
.PHONY: $(addprefix test-,$(TEST_PACKAGES))
$(addprefix test-,$(TEST_PACKAGES)): test-%: go-dep cgo-dep
	@echo Test $* "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" test $(GO_TAGS_FLAG) $(ARGS) ./$*/...

###############################################################################
# CHECKS

# Report the C libraries, and whether the build uses cgo
.PHONY: deps
deps: go-dep
	@echo "Go:         $(shell "$(GO)" env GOVERSION) $(shell "$(GO)" env GOOS)/$(shell "$(GO)" env GOARCH)"
	@echo "pkg-config: $(if $(PKG_CONFIG),$(PKG_CONFIG),not found)"
	@for m in $(CGO_MODULES); do \
		if [ -n "$(PKG_CONFIG)" ] && $(PKGCONF) --exists $$m; then \
			printf "  %-14s %s\n" $$m "$$($(PKGCONF) --modversion $$m)"; \
		else \
			printf "  %-14s %s\n" $$m "not found"; \
		fi; \
	done
	@$(if $(FFMPEG_WRONG),echo "FFmpeg isn't FFmpeg 8 (libavformat $(FFMPEG_VERSION)): on macOS brew install ffmpeg@8$(comma) or set GO_MEDIA_PREFIX")
	@$(if $(CGO_MISSING),echo "Not found: $(CGO_MISSING)")
	@$(if $(CGO_OLD),echo "Too old: $(CGO_OLD)")
	@echo "CGO_ENABLED=$(CGO_ENABLED)"
	@echo "Chromaprint: $(if $(CHROMAPRINT),$(shell $(PKGCONF) --modversion libchromaprint) (-tags chromaprint),not found$(comma) so audio isn't looked up on AcoustID)"
	@$(if $(filter 0,$(CGO_ENABLED)),echo "$(NOCGO_NOTE)")

# Check the code with go vet
.PHONY: vet
vet: go-dep cgo-dep
	@echo Vet "(CGO_ENABLED=$(CGO_ENABLED))"
	@"$(GO)" vet $(GO_TAGS_FLAG) ./...

# Check the code is formatted, and list the files which aren't
.PHONY: fmt
fmt: go-dep
	@echo Check formatting
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))" || (gofmt -l $$(git ls-files '*.go') && exit 1)

# Tidy go.mod and go.sum
.PHONY: tidy
tidy: go-dep
	@echo Tidy
	@"$(GO)" mod tidy

###############################################################################
# DEPENDENCIES, ETC

.PHONY: go-dep
go-dep:
	@test -n "$(GO)" && test -x "$(GO)" || (echo "Missing go binary" && exit 1)

# Check the C libraries are found, and FFmpeg is FFmpeg 8, when building with
# cgo, as the Go compiler's errors when they aren't don't say why
.PHONY: cgo-dep
cgo-dep:
ifeq ($(CGO_ENABLED),1)
ifneq ($(CGO_MISSING),)
	@echo "Can't build with cgo, as these C libraries aren't found: $(CGO_MISSING)"
	@echo "Install them (see README.md), or build without cgo with CGO_ENABLED=0"
	@exit 1
endif
ifneq ($(CGO_OLD),)
	@echo "Can't build with cgo, as these C libraries are older than go-media's bindings need: $(CGO_OLD)"
	@echo "Install newer ones, or build go-media's libraries with make in a clone of it, and set GO_MEDIA_PREFIX to its build/install folder"
	@echo "Or build without cgo with CGO_ENABLED=0"
	@exit 1
endif
ifneq ($(FFMPEG_WRONG),)
	@echo "Can't build with cgo, as FFmpeg $(FFMPEG_INSTALLED) is installed, and go-media's bindings need FFmpeg 8 (libavformat $(FFMPEG_VERSION))"
	@echo "On macOS, brew install ffmpeg@8, which is used rather than ffmpeg"
	@echo "Or build go-media's libraries with make in a clone of it, and set GO_MEDIA_PREFIX to its build/install folder"
	@echo "Or build without cgo with CGO_ENABLED=0"
	@exit 1
endif
endif

.PHONY: mkdir
mkdir:
	@install -d $(BUILD_DIR)

.PHONY: clean
clean:
	@echo Clean
	@rm -fr $(BUILD_DIR)
	@"$(GO)" clean -testcache

# List the targets
.PHONY: help
help:
	@echo "make                 Build the commands in $(BUILD_DIR), with cgo when the C libraries are found"
	@echo "make cmd-nocgo       Build the commands without cgo"
	@echo "make install         Install the commands"
	@echo "make deps            Report the C libraries, and whether the build uses cgo"
	@echo "make test            Test everything"
	@echo "make test-race       Test everything with the race detector"
	@echo "make test-coverage   Test everything, and report the coverage"
	@echo "make test-nocgo      Test everything without cgo"
	@echo "make test-core       Test the core package"
	@echo "make test-<package>  Test a package: $(TEST_PACKAGES)"
	@echo "make vet fmt tidy    Check and tidy the code"
	@echo "make clean           Remove $(BUILD_DIR), and the test cache"
	@echo ""
	@echo "ARGS=-v                       Arguments to go test"
	@echo "CGO_ENABLED=0|1               Build with or without cgo, rather than by the C libraries found"
	@echo "GO_MEDIA_PREFIX=<dir>         Use go-media's static libraries, such as ../go-media/build/install"
	@echo "GOWORK=<file>                 Use a Go workspace, rather than this module's go.mod"
