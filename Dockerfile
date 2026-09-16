# syntax=docker/dockerfile:1.8
# check=error=true

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

ENV CGO_ENABLED=0 \
    GOMODCACHE=/go/pkg/mod \
    GOCACHE=/root/.cache/go-build \
    GOTOOLCHAIN=local \
    TZ=UTC \
    SOURCE_DATE_EPOCH=0

WORKDIR /workspace

# warm up module cache
COPY go.mod go.sum ./
RUN \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# copy sources
COPY . .


# target parameters for cross-compilation
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG REVISION

# build the binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS:-$(go env GOOS)} \
    GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build \
      -v \
      -o /workspace/vault-manager \
      -trimpath \
      -mod=readonly \
      -buildvcs=false \
      -tags netgo,osusergo,timetzdata \
      -pgo=auto \
      -ldflags "-s -w -buildid= \
                -extldflags '-static' \
                -X 'main.Version=${VERSION}' \
                -X 'main.Revision=${REVISION}'" \
      .

FROM alpine:edge

RUN apk --no-cache add ca-certificates

# copy the binary (read/execute permissions are enough)
COPY --from=build --chmod=0555 /workspace/vault-manager /usr/local/bin/vault-manager

ENTRYPOINT ["/usr/local/bin/vault-manager"]

USER nobody