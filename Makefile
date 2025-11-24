.PHONY: build dev test clean docker

# Variables
BINARY_NAME=cilium-microservices
VERSION=1.0.0
BUILD_TIME=$(shell date +%Y-%m-%d_%H:%M:%S)
BUILD_USER=EmAchieng

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build all services
build:
	@echo "Building all microservices..."
	cd api-gateway && $(GOBUILD) -o ../bin/api-gateway -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
	cd user-service && $(GOBUILD) -o ../bin/user-service -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
	cd product-service && $(GOBUILD) -o ../bin/product-service -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
	@echo "Build complete!"

# Run in development mode
dev:
	@echo "Starting development environment..."
	@echo "Starting API Gateway on port 8000..."
	cd api-gateway && $(GOCMD) run main.go &
	@echo "Starting User Service on port 8001..."
	cd user-service && $(GOCMD) run service.go &
	@echo "Starting Product Service on port 8002..."
	cd product-service && $(GOCMD) run main.go &
	@echo "All services started!"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Clean build files
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Docker build
docker:
	@echo "Building Docker images..."
	docker build -t cilium-microservices/api-gateway:$(VERSION) -f api-gateway/Dockerfile .
	docker build -t cilium-microservices/user-service:$(VERSION) -f user-service/Dockerfile .
	docker build -t cilium-microservices/product-service:$(VERSION) -f product-service/Dockerfile .

# Help
help:
	@echo "Available targets:"
	@echo "  build   - Build all microservices"
	@echo "  dev     - Run in development mode"
	@echo "  test    - Run tests"
	@echo "  clean   - Clean build files"
	@echo "  deps    - Download dependencies"
	@echo "  docker  - Build Docker images"
	@echo "  help    - Show this help"
