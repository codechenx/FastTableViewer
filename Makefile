.PHONY: all build clean test lint install uninstall release snapshot help

BINARY_NAME=ftv
INSTALL_PATH=/usr/local/bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags="-s -w -X main.version=$(VERSION)"

all: build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME)

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf dist/

## test: Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

## lint: Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install: https://golangci-lint.run/usage/install/" && exit 1)
	@golangci-lint run

## install: Install the binary to system
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
	@echo "✓ Installed successfully"

## uninstall: Remove the binary from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ Uninstalled successfully"

## release: Create a new release (requires goreleaser)
release:
	@which goreleaser > /dev/null || (echo "goreleaser not found. Install: https://goreleaser.com/install/" && exit 1)
	@echo "Creating release..."
	@goreleaser release --clean

## snapshot: Create a snapshot release (local testing)
snapshot:
	@which goreleaser > /dev/null || (echo "goreleaser not found. Install: https://goreleaser.com/install/" && exit 1)
	@echo "Creating snapshot..."
	@goreleaser release --snapshot --clean

## run: Build and run with sample data
run: build
	@./$(BINARY_NAME) --help

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

## check: Run tests and linters
check: test lint

## version: Update version in all files
version:
	@echo "Updating version to $(VERSION)..."
	@sed -i "" "s/version: '.*'/version: '$(VERSION)'/g" snap/snapcraft.yaml
	@sed -i "" -E "s#/releases/download/v[0-9][^/]*/FastTableViewer_[0-9][^_]*_#/releases/download/v$(VERSION)/FastTableViewer_$(VERSION)_#g" README.md
	@sed -i "" "s/Version: \".*\"/Version: \"$(VERSION)\"/g" ftv.go
	@sed -i "" "s/pkgver=.*/pkgver=$(VERSION)/g" PKGBUILD

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
