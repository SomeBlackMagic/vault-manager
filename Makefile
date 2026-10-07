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

# Runtime logging for the sync-* targets (see README "Logging").
# `go build -v` above is compiler verbosity and is unrelated to these.
LOG_LEVEL  ?= info
LOG_FORMAT ?= text
SYNC_ENV   := VAULT_MANAGER_LOG_LEVEL=$(LOG_LEVEL) VAULT_MANAGER_LOG_FORMAT=$(LOG_FORMAT)

check-sync-args:
	@test -n "$(VAULT_PATH)" || { echo "VAULT_PATH is required, e.g. make sync-plan VAULT_PATH=secret/myapp LOCAL_DIR=./secrets" >&2; exit 1; }
	@test -n "$(LOCAL_DIR)" || { echo "LOCAL_DIR is required, e.g. make sync-plan VAULT_PATH=secret/myapp LOCAL_DIR=./secrets" >&2; exit 1; }
	@test -x ./vault-manager || { echo "./vault-manager not found; run 'make build' first" >&2; exit 1; }

sync-pull: check-sync-args
	$(SYNC_ENV) ./vault-manager sync pull "$(VAULT_PATH)" "$(LOCAL_DIR)"

sync-plan: check-sync-args
	$(SYNC_ENV) ./vault-manager sync plan "$(VAULT_PATH)" "$(LOCAL_DIR)"

sync-apply: check-sync-args
	$(SYNC_ENV) ./vault-manager sync apply "$(VAULT_PATH)" "$(LOCAL_DIR)"

.PHONY: build unit-test test release install check-sync-args sync-pull sync-plan sync-apply

install: build
	mkdir -p $(DESTDIR)/bin
	cp vault-manager $(DESTDIR)/bin