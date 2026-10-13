# kwikmesh Makefile

PKG_NAME   := kwikmesh
PKG_PREFIX ?= ./build

CMD_NAME  := km
CMD_PKG   := ./cmd/$(CMD_NAME)

GO        ?= go
GOFLAGS   ?= -trimpath
LINT_ARGS :=

# Handle release versioning
# The semver base is taken from the most recent tagged commit. If the current
# commit is not tagged, a dev version is created using the most recent tag
# and the short commit hash.
VERSION_VAR = kwikmesh/commands.Version
VERSION := $(shell git describe --exact-match --tags HEAD 2>/dev/null)
ifeq ($(VERSION),)
        # Create a dev version: vX.Y.Z-dev
        VERSION := $(addsuffix -$(shell git rev-parse --short HEAD),$(shell git describe --abbrev=0 --tags HEAD))
endif
VERSION_LD := -X $(VERSION_VAR)=$(VERSION)

# Strip debug and DWARF info for release builds.
RELEASE_LD := -s -w

all: lint test build

.PHONY: build
build: export CGO_ENABLED=0
build: LDFLAGS ?= -ldflags "$(VERSION_LD)"
build:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o ./bin/$(CMD_NAME) $(CMD_PKG)

.PHONY: clean
clean:
	rm -f ./bin/$(CMD_NAME)
	go clean -testcache

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: lint
lint:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	$(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint run ./... $(LINT_ARGS)"; \
		golangci-lint run ./... $(LINT_ARGS); \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

.PHONY: release
release: export CGO_ENABLED=0
release: LDFLAGS ?= -ldflags "$(VERSION_LD) $(RELEASE_LD)"
release: lint test
	@if git describe --exact-match --tags HEAD 2>/dev/null; then \
		echo "Building release for $(VERSION)"; \
		$(MAKE) build LDFLAGS='$(LDFLAGS)'; \
	else \
		echo "Not on a tagged commit, skipping release build"; \
		exit 1; \
	fi

.PHONY: test
test:
	$(GO) test $(GOFLAGS) ./...

.PHONY: tidy
tidy:
	go mod tidy
