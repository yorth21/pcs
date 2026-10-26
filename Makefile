.PHONY: build test test-race vet clean install

BINARY := bin/pcs

# The race detector needs cgo. Enable it only when the Go toolchain reports
# CGO_ENABLED=1 (Go disables cgo by default when no C compiler is installed,
# which is the usual case on Windows). Override with `make test RACE=` or
# `make test RACE=-race`; `make test-race` always requests the race detector.
RACE ?= $(if $(filter 1,$(shell go env CGO_ENABLED)),-race,)

CLEAN_PATHS := bin dist coverage.out

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

build:
	go build -o $(BINARY) ./cmd/pcs

test:
	go test $(RACE) ./...

test-race:
	go test -v -race ./...

vet:
	go vet ./...

clean:
	$(CLEAN_CMD)

install:
	go install ./cmd/pcs
