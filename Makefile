# Makefile for the vd CLI.
# Usage: make build | make test | make vet | make lint | make run

# Use mise-managed Go if available; callers can still override: GO=/path/to/go make build
GO      ?= $(shell mise which go 2>/dev/null || command -v go)
VERSION ?= dev
BINARY  := vd
LDFLAGS := -ldflags "-X github.com/vanducng/vd-cli/v2/internal/version.Version=$(VERSION)"

.PHONY: build test vet lint run clean web web-dev desktop desktop-dev

build:
	$(GO) build $(LDFLAGS) -o $(BINARY) ./cmd/vd

# Rebuild the embedded `vd web` SPA into internal/ui/web/static (committed output).
# Requires Node; the Go build itself never needs Node.
web:
	cd web && npm ci && npm run build

# Live-reload web dev server (proxies /api to a running `vd web`).
web-dev:
	cd web && npm install && npm run dev

# Build the Wails desktop app (separate module; needs the wails CLI + a C toolchain).
# Kept out of the main module so the pure-Go `vd` CLI stays CGO-free.
desktop:
	cd desktop && wails build -s

desktop-dev:
	cd desktop && wails dev

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found — skipping (install from https://golangci-lint.run/usage/install/)"; \
	fi

run:
	$(GO) run $(LDFLAGS) ./cmd/vd $(ARGS)

clean:
	rm -f $(BINARY)
