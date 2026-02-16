BINARY  := claudeRemote
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOARCH  := $(shell go env GOARCH)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install clean deb checksums

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/claude-remote

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/claude-remote

deb: build
	VERSION=$(VERSION) ARCH=$(GOARCH) nfpm package --packager deb --target $(BINARY)_$(VERSION)_$(GOARCH).deb

checksums:
	sha256sum $(BINARY)*.deb > checksums.txt

clean:
	rm -f $(BINARY) *.deb checksums.txt
