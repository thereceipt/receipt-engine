.PHONY: build clean run test cross-compile

# Build for current platform
build:
	go build -o receipt-engine ./cmd/server

# Run the server
run:
	go run ./cmd/server

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f receipt-engine
	rm -rf dist/

# Cross-compile for all platforms
cross-compile:
	mkdir -p dist
	# macOS (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build -o dist/receipt-engine-darwin-arm64 ./cmd/server
	# macOS (Intel)
	GOOS=darwin GOARCH=amd64 go build -o dist/receipt-engine-darwin-amd64 ./cmd/server
	# Windows (64-bit)
	GOOS=windows GOARCH=amd64 go build -o dist/receipt-engine-windows-amd64.exe ./cmd/server
	# Linux (64-bit)
	GOOS=linux GOARCH=amd64 go build -o dist/receipt-engine-linux-amd64 ./cmd/server
	# Linux (ARM64)
	GOOS=linux GOARCH=arm64 go build -o dist/receipt-engine-linux-arm64 ./cmd/server

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Development mode with auto-reload
dev:
	air
