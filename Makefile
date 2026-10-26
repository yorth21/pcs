.PHONY: build test test-race vet clean install

BINARY := bin/pcs

build:
	go build -o $(BINARY) ./cmd/pcs

test:
	go test ./...

test-race:
	go test -v -race ./...

vet:
	go vet ./...

clean:
	rm -rf bin/ dist/ coverage.out

install:
	go install ./cmd/pcs
