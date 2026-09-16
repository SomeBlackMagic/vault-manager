DESTDIR      ?= /usr/local
RELEASE_ROOT ?= release
TARGETS      ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64

GIT_REVISION := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
GO_LDFLAGS   := -s -w -buildid= -extldflags '-static' \
                -X 'main.Version=$(VERSION)' -X 'main.Revision=$(GIT_REVISION)'
GO_BUILD     := SOURCE_DATE_EPOCH=0 CGO_ENABLED=0 go build \
                -v -trimpath -mod=readonly -buildvcs=false \
                -tags netgo,osusergo,timetzdata -pgo=auto \
                -ldflags="$(GO_LDFLAGS)"

build:
	$(GO_BUILD) -o vault-manager .
	./vault-manager -v

unit-test:
	go test ./...

test: build unit-test
	./tests

release:
	mkdir -p $(RELEASE_ROOT)/artifacts
	@for target in $(TARGETS); do \
		os=$${target%/*}; arch=$${target#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch $(GO_BUILD) \
			-o $(RELEASE_ROOT)/artifacts/vault-manager-$$os-$$arch$$ext . || exit 1; \
	done

install: build
	mkdir -p $(DESTDIR)/bin
	cp vault-manager $(DESTDIR)/bin