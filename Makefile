.PHONY: build test test-race vet clean install

BINARY := bin/pcs

# The race detector needs cgo. Enable it only when the Go toolchain reports
# CGO_ENABLED=1 (Go disables cgo by default when no C compiler is installed,
# which is the usual case on Windows). Override with `make test RACE=` or
# `make test RACE=-race`; `make test-race` always requests the race detector.
RACE ?= $(if $(filter 1,$(shell go env CGO_ENABLED)),-race,)

CLEAN_PATHS := bin dist coverage.out

# Build metadata injected into `pcs version` (the same -X flags GoReleaser
# uses). Each value falls back when git (or a tag) is unavailable; override with
# e.g. `make build VERSION=1.2.3`. The build date is UTC RFC 3339, produced by
# PowerShell on Windows and POSIX date elsewhere.
VERSION_PKG := github.com/yorth21/pcs/internal/installer

# Portable recursive delete: PowerShell on Windows (where rm is unavailable in
# cmd.exe), POSIX rm elsewhere. Missing paths are not an error on either side.
empty :=
space := $(empty) $(empty)
comma := ,
ifeq ($(OS),Windows_NT)
CLEAN_CMD = powershell -NoProfile -NonInteractive -Command "Remove-Item -Recurse -Force -ErrorAction SilentlyContinue -Path $(subst $(space),$(comma),$(strip $(CLEAN_PATHS)))"
else
CLEAN_CMD = rm -rf $(CLEAN_PATHS)
endif

ifeq ($(OS),Windows_NT)
NULL_DEVICE := NUL
BUILD_DATE_CMD := powershell -NoProfile -NonInteractive -Command "[DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')"
else
NULL_DEVICE := /dev/null
BUILD_DATE_CMD := date -u +%Y-%m-%dT%H:%M:%SZ
endif

VERSION ?= $(patsubst v%,%,$(or $(shell git describe --tags --always --dirty 2>$(NULL_DEVICE)),dev))
COMMIT ?= $(or $(shell git rev-parse HEAD 2>$(NULL_DEVICE)),unknown)
BUILD_DATE ?= $(or $(shell $(BUILD_DATE_CMD)),unknown)
LDFLAGS = -X $(VERSION_PKG).version=$(VERSION) -X $(VERSION_PKG).commit=$(COMMIT) -X $(VERSION_PKG).date=$(BUILD_DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/pcs

test:
	go test $(RACE) ./...

test-race:
	go test -v -race ./...

vet:
	go vet ./...

clean:
	$(CLEAN_CMD)

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/pcs
