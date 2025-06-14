 # > make help
#
# The following commands can be used.
#
# init:  sets up environment and installs requirements
# install:  Installs development requirments
# format:  Formats the code with autopep8
# lint:  Runs flake8 on src, exit if critical rules are broken
# clean:  Remove build and cache files
# env:  Source venv and environment files for testing
# leave:  Cleanup and deactivate venv
# test:  Run pytest
# run:  Executes the logic

GO_PKG_NAME = 'go-sandbox'
ENVIRONMENT_VARIABLE_FILE='.env'
# DOCKER_NAME='metric_spoon'
# DOCKER_TAG=$DOCKER_TAG
#GOPATH = '.'
PROJECT_DIR = $(shell readlink -e .)

define find.functions
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
endef

init: ## Initialize the project
	@go mod init $(GO_PKG_NAME)
	@go mod tidy
	@go mod vendor
	@go mod download
	@go mod verify

install: ## Tidy the project
	@go mod tidy

lint:
lint: ## Run linter
	golangci-lint run

fmt:
fmt: ## Run gofmt
	gofmt -s -w .

build:
build: ## Build the project
	go build -o $(GO_PKG_NAME)

build-linux:
build-linux: ## Build the project for linux
	GOOS=linux GOARCH=amd64 go build -o $(GO_PKG_NAME)

build-mac:
build-mac: ## Build the project for mac
	GOOS=darwin GOARCH=amd64 go build -o $(GO_PKG_NAME).dpkg

build-windows:
build-windows: ## Build the project for windows
	GOOS=windows GOARCH=amd64 go build -o $(GO_PKG_NAME).exe

run:
run: ## Run the project
	go run main.go

test:
test: ## Run tests
	go test

show-env:
show-env: ## Show environment variables
	go env GOARCH GOOS GOPATH GOROOT
