# Makefile for AWS IAM Role Explorer TUI

.PHONY: build run clean test deps help

APP_NAME = atui
GO_PKG_NAME = 'atui'
PROJECT_DIR = $(shell readlink -e .) 

## Development Commands

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

deps: ## Install dependencies
	@echo "Installing dependencies..."
	@go get github.com/charmbracelet/bubbletea
	@go get github.com/charmbracelet/bubbles
	@go get github.com/charmbracelet/lipgloss
	@go get github.com/aws/aws-sdk-go-v2
	@go get github.com/aws/aws-sdk-go-v2/config
	@go get github.com/aws/aws-sdk-go-v2/service/iam
	@go get github.com/aws/aws-sdk-go-v2/service/sts
	@go mod tidy

init: ## Initialize the project
	@go mod init $(GO_PKG_NAME)
	@go mod tidy
	@go mod vendor
	@go mod download
	@go mod verify

build: ## Build the application
	@echo "Building $(APP_NAME)..."
	@go build -o $(APP_NAME) main.go

run: ## Run the application
	@go run main.go

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(APP_NAME)
	@go clean

test: ## Run tests
	@go test -v ./...

fmt: ## Format code
	@go fmt ./...

vet: ## Run go vet
	@go vet ./...

lint: fmt vet ## Lint and format code
	@echo "Linting completed."
