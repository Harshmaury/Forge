# Makefile — Forge
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.forgeVersion=$(VERSION)
BINDIR  := $(HOME)/bin

.PHONY: all build clean

all: build

build:
	@echo "  → forge $(VERSION)"
	@CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/forge ./cmd/forge/

clean:
	@rm -f $(BINDIR)/forge
