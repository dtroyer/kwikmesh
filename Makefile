# kwikmesh Makefile

PKG_NAME   := kwikmesh
PKG_PREFIX ?= ./build

CMD_NAME  := km
CMD_PKG   := ./cmd/$(CMD_NAME)

GO        ?= go
GOFLAGS   ?=
LINT_ARGS :=

all: lint test build

.PHONY: build
build: export CGO_ENABLED=0
build:
	$(GO) build $(GOFLAGS) -o ./bin/$(CMD_NAME) $(CMD_PKG)

.PHONY: clean
clean:
	rm -f ./bin/$(CMD_NAME)
	go clean -testcache

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: lint
lint:
	golangci-lint run ./... $(LINT_ARGS)

.PHONY: test
test:
	$(GO) test $(GOFLAGS) ./...

.PHONY: tidy
tidy:
	go mod tidy
