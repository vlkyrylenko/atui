# Makefile for AWS IAM Role Explorer TUI

.PHONY: build run clean test deps help build-all build-linux build-darwin build-windows

APP_NAME = atui
GO_PKG_NAME = 'atui'
PROJECT_DIR = $(shell readlink -e .) 
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

## Development Commands

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Install dependencies
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

build: ## Build the application for current platform
	@echo "Building $(APP_NAME)..."
	@go build $(LDFLAGS) -o $(APP_NAME) main.go

build-all: build-linux build-darwin build-windows ## Build for all platforms

build-linux: ## Build for Linux (amd64)
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-linux-amd64 main.go

build-darwin: ## Build for macOS (amd64 and arm64)
	@echo "Building for macOS..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-darwin-amd64 main.go
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(APP_NAME)-darwin-arm64 main.go

build-windows: ## Build for Windows (amd64)
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-windows-amd64.exe main.go

install: build ## Install the binary to $GOPATH/bin
	@echo "Installing $(APP_NAME) to $(GOPATH)/bin..."
	@cp $(APP_NAME) $(GOPATH)/bin/

dist: clean build-all ## Create distribution packages
	@echo "Creating distribution packages..."
	@mkdir -p dist
	@cd dist && \
	tar -czf $(APP_NAME)-linux-amd64.tar.gz $(APP_NAME)-linux-amd64 && \
	tar -czf $(APP_NAME)-darwin-amd64.tar.gz $(APP_NAME)-darwin-amd64 && \
	tar -czf $(APP_NAME)-darwin-arm64.tar.gz $(APP_NAME)-darwin-arm64 && \
	zip $(APP_NAME)-windows-amd64.zip $(APP_NAME)-windows-amd64.exe

run: ## Run the application
	@go run main.go

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(APP_NAME)
	@rm -rf dist/
	@go clean

test: ## Run tests
	@go test -v ./...

fmt: ## Format code
	@go fmt ./...

lint: ## Run linter
	@golangci-lint run

dev-setup: deps ## Setup development environment
	@echo "Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
