GO ?= $(shell command -v go 2>/dev/null)
ifeq ($(GO),)
$(error go not found in PATH)
endif

APP_NAME ?= bedrock

# detected Go environment
GO_VERSION := $(shell $(GO) version | awk '{print $$3}')
GOOS := $(shell $(GO) env GOOS)
GOARCH := $(shell $(GO) env GOARCH)
GOHOSTOS := $(shell $(GO) env GOHOSTOS)
GOHOSTARCH := $(shell $(GO) env GOHOSTARCH)
CGO_ENABLED ?= $(shell $(GO) env CGO_ENABLED)

# build/compile flags (override from CLI as needed)
GOFLAGS ?= -trimpath
GCFLAGS ?= all=-trimpath=$(CURDIR)
ASMFLAGS ?= all=-trimpath=$(CURDIR)
LDFLAGS ?= -s -w
BUILD_FLAGS ?=
RUN_FLAGS ?=

.PHONY: help env compile build run test clean lint fmt vet e2e

help:
	@echo "Available targets:"
	@echo "  env      - Print detected Go environment"
	@echo "  compile  - Compile the Go application"
	@echo "  build    - Build the Go application"
	@echo "  run      - Run the Go application"
	@echo "  test     - Run tests"
	@echo "  clean    - Remove build artifacts"
	@echo "  lint     - Run linter"
	@echo "  fmt      - Format code"
	@echo "  vet      - Run go vet"
	@echo "  e2e      - Run end-to-end tests (requires 'e2e' build tag)"

env:
	@echo "Go binary: $(GO)"
	@echo "Go version: $(GO_VERSION)"
	@echo "Target: $(GOOS)/$(GOARCH)"
	@echo "Host: $(GOHOSTOS)/$(GOHOSTARCH)"
	@echo "CGO_ENABLED: $(CGO_ENABLED)"

compile:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(GOFLAGS) -gcflags "$(GCFLAGS)" -asmflags "$(ASMFLAGS)" -ldflags "$(LDFLAGS)" $(BUILD_FLAGS) -o $(APP_NAME) .

build:
	$(MAKE) compile

run:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) run $(GOFLAGS) $(RUN_FLAGS) .

test:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) test -v ./...

e2e:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) test -tags=e2e ./tests/e2e/

clean:
	rm -f $(APP_NAME)

lint:
	golangci-lint run ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...
