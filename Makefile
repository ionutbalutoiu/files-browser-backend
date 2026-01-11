.PHONY: build test clean run docker

# Binary name
BINARY=upload-server

# Build flags
LDFLAGS=-ldflags="-s -w"

# Default target
all: build

# Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY) .

# Run tests
test:
	go test -v ./...

# Run tests with coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f $(BINARY) coverage.out coverage.html

# Run locally (requires /tmp/files to exist)
run: build
	mkdir -p /tmp/files
	./$(BINARY) -base-dir /tmp/files -listen :8080

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
	sudo cp $(BINARY) /usr/local/bin/
	sudo chmod 755 /usr/local/bin/$(BINARY)
