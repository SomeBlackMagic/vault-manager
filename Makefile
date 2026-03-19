DESTDIR      ?= /usr/local
RELEASE_ROOT ?= release
TARGETS      ?= linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

GIT_REVISION := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
GO_LDFLAGS := -ldflags="-X main.Version=$(VERSION) -X main.Revision=$(GIT_REVISION)"

build:
	go build $(GO_LDFLAGS) .
	./vault-manager -v

unit-test:
	go test ./...

test: build unit-test
	./tests

release: build
	mkdir -p $(RELEASE_ROOT)
	@go install github.com/mitchellh/gox@latest
	gox -osarch="$(TARGETS)" --output="$(RELEASE_ROOT)/artifacts/vault-manager-{{.OS}}-{{.Arch}}" $(GO_LDFLAGS)

install: build
	mkdir -p $(DESTDIR)/bin
	cp vault-manager $(DESTDIR)/bin
