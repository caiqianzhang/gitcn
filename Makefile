VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -X github.com/caiqianzhang/gitcn/internal/cli.Version=$(VERSION)

.PHONY: build install release test vet fmt

build:
	go build -ldflags "$(LDFLAGS)" -o bin/gitcn ./cmd/gitcn

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/gitcn

release:
	VERSION=$(VERSION) ./scripts/release.sh

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l .
