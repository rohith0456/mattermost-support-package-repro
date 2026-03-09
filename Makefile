# mm-repro Makefile
# ==================

BINARY_NAME    := mm-repro
MODULE         := github.com/mattermost/mattermost-support-package-repro
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
COMMIT         ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE     ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS        := -ldflags "-X $(MODULE)/pkg/version.Version=$(VERSION) \
                             -X $(MODULE)/pkg/version.Commit=$(COMMIT) \
                             -X $(MODULE)/pkg/version.BuildDate=$(BUILD_DATE)"

GOOS           ?= $(shell go env GOOS)
GOARCH         ?= $(shell go env GOARCH)
GO             := go
GOFLAGS        ?=

OUTPUT_DIR     := bin
DIST_DIR       := dist

.PHONY: all build clean test lint fmt vet tidy help \
        build-linux build-darwin build-windows build-all \
        install uninstall doctor run stop reset

## help: Show this help message
help:
	@echo "mm-repro build targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## all: Build and test
all: tidy fmt vet test build

## build: Build the binary for the current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) ./cmd/mm-repro/

## build-linux: Build for Linux amd64
build-linux:
	GOOS=linux GOARCH=amd64 $(MAKE) build
	mv $(OUTPUT_DIR)/$(BINARY_NAME) $(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64

## build-darwin: Build for macOS (Intel and Apple Silicon)
build-darwin:
	GOOS=darwin GOARCH=amd64 $(MAKE) build
	mv $(OUTPUT_DIR)/$(BINARY_NAME) $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 $(MAKE) build
	mv $(OUTPUT_DIR)/$(BINARY_NAME) $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64

## build-windows: Build for Windows amd64
build-windows:
	GOOS=windows GOARCH=amd64 $(MAKE) build
	mv $(OUTPUT_DIR)/$(BINARY_NAME) $(OUTPUT_DIR)/$(BINARY_NAME)-windows-amd64.exe

## build-all: Build for all supported platforms
build-all: build-linux build-darwin build-windows

## install: Install mm-repro to $GOPATH/bin
install:
	$(GO) install $(GOFLAGS) $(LDFLAGS) ./cmd/mm-repro/
	@echo "Installed mm-repro to $$(go env GOPATH)/bin/"

## uninstall: Remove mm-repro from $GOPATH/bin
uninstall:
	rm -f "$$(go env GOPATH)/bin/$(BINARY_NAME)"

## test: Run all unit tests
test:
	@echo "Running tests..."
	$(GO) test ./... -v -count=1

## test-short: Run tests excluding integration tests
test-short:
	$(GO) test ./... -short -count=1

## test-coverage: Run tests with coverage report
test-coverage:
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run golangci-lint (requires golangci-lint installed)
lint:
	golangci-lint run ./...

## fmt: Format Go source files
fmt:
	$(GO) fmt ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## tidy: Tidy go modules
tidy:
	$(GO) mod tidy

## clean: Remove build artifacts and generated repro projects
clean:
	rm -rf $(OUTPUT_DIR) $(DIST_DIR) coverage.out coverage.html
	@echo "Cleaned build artifacts"

## doctor: Run the mm-repro doctor check
doctor: build
	./$(OUTPUT_DIR)/$(BINARY_NAME) doctor

## run: Start a generated repro project (PROJECT env var required)
run:
ifndef PROJECT
	$(error PROJECT is not set. Usage: make run PROJECT=./generated-repro/my-repro)
endif
	docker compose -f $(PROJECT)/docker-compose.yml --env-file $(PROJECT)/.env up -d
	@echo "Repro environment started. Check $(PROJECT)/REPRO_SUMMARY.md for details."

## stop: Stop a generated repro project
stop:
ifndef PROJECT
	$(error PROJECT is not set. Usage: make stop PROJECT=./generated-repro/my-repro)
endif
	docker compose -f $(PROJECT)/docker-compose.yml --env-file $(PROJECT)/.env down

## reset: Reset a generated repro project (removes volumes)
reset:
ifndef PROJECT
	$(error PROJECT is not set. Usage: make reset PROJECT=./generated-repro/my-repro)
endif
	docker compose -f $(PROJECT)/docker-compose.yml --env-file $(PROJECT)/.env down -v
	@echo "Repro environment reset. All volumes removed."

## example: Run the basic example repro
example: build
	./$(OUTPUT_DIR)/$(BINARY_NAME) init \
		--support-package ./examples/basic/sample-support-package.zip \
		--output ./generated-repro/example

## dist: Create distribution archives
dist: build-all
	@mkdir -p $(DIST_DIR)
	cd $(OUTPUT_DIR) && for f in $(BINARY_NAME)-*; do \
		tar -czf ../$(DIST_DIR)/$$f.tar.gz $$f 2>/dev/null || \
		zip ../$(DIST_DIR)/$$f.zip $$f 2>/dev/null || true; \
	done
	@echo "Distribution archives in $(DIST_DIR)/"
