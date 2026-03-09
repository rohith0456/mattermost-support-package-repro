#!/usr/bin/env bash
# bootstrap.sh — Initialize the mm-repro development environment.
# Run this once after cloning the repository.
set -euo pipefail

echo "mm-repro bootstrap"
echo "=================="
echo ""

# Check Go
if ! command -v go &>/dev/null; then
    echo "ERROR: Go is not installed. Install Go 1.22+ from https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | grep -oP '\d+\.\d+' | head -1)
echo "Go version: $(go version)"

# Download dependencies
echo ""
echo "Downloading Go dependencies..."
go mod tidy
go mod download

# Build
echo ""
echo "Building mm-repro..."
go build -o bin/mm-repro ./cmd/mm-repro/

echo ""
echo "Build successful: ./bin/mm-repro"
echo ""
echo "Run 'make doctor' to check prerequisites."
echo "Run 'make test' to run tests."
echo ""
echo "Bootstrap complete!"
