BINARY := hangar
MODULE := github.com/v4run/hangar
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install test lint clean run

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/hangar/

install:
	go install $(LDFLAGS) ./cmd/hangar/

test:
	go test ./... -v

test-race:
	go test ./... -race -v

lint:
	go vet ./...

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY)
