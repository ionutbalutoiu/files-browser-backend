.PHONY: build test clean run docker

# Binary name
BINARY=files-svc

# Build directory
BUILD_DIR=build

# Build flags
LDFLAGS=-ldflags="-s -w"

# Default target
all: build

# Build the binary
build:
	mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/files-svc

# Run tests
test:
	go test -v ./...

# Run tests with coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

# Run locally (requires /tmp/files to exist)
run: build
	mkdir -p /tmp/files
	./$(BUILD_DIR)/$(BINARY) -base-dir /tmp/files -listen :8080

# Build Docker image
docker:
	docker build -t $(BINARY) .

# Run in Docker
docker-run: docker
	docker run -d --name $(BINARY) -p 8080:8080 -v /tmp/files:/data $(BINARY)

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Install to system (requires sudo)
install: build
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/
	sudo chmod 755 /usr/local/bin/$(BINARY)
