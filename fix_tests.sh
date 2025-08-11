#!/bin/bash

# Fix all NewServer calls in test files to include a logger
echo "Fixing NewServer calls in test files..."

# Replace NewServer(manager) with NewServer(manager, slog.Default())
find cmd/mcp-server -name "*.go" -type f -exec sed -i 's/NewServer(manager)/NewServer(manager, slog.Default())/g' {} \;

echo "Fixed all NewServer calls"

# Run tests to verify
echo "Running tests to verify fixes..."
go test ./cmd/mcp-server -v