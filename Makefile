SHELL := /bin/bash

GO       ?= go
GOFLAGS  ?=
TIMEOUT  ?= 60s

.DEFAULT_GOAL := help

##@ General

help: ## Show this help
	@awk 'BEGIN{FS=":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0,5)}' $(MAKEFILE_LIST)

version: ## Print SDK version
	@cat VERSION

##@ Build / dev

tidy: ## go mod tidy
	$(GO) mod tidy

build: ## Build all packages (no binary)
	$(GO) build ./...

vet: ## go vet
	$(GO) vet ./...

##@ Test

test: ## Run unit tests with race detector
	$(GO) test -race -timeout $(TIMEOUT) ./...

test-cover: ## Tests + HTML coverage report
	$(GO) test -race -coverprofile=coverage.txt -covermode=atomic ./...
	$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "open coverage.html"

##@ Lint

lint: ## golangci-lint run (requires golangci-lint installed)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

##@ Examples

run-minimal: ## Run examples/minimal
	cd examples/minimal && $(GO) run .

run-http: ## Run examples/http-server (listens on :8080)
	cd examples/http-server && $(GO) run .

##@ Validate

ci: tidy vet test ## Full CI flow

.PHONY: help version tidy build vet test test-cover lint run-minimal run-http ci
