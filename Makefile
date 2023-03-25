PKG := "github.com/labstack/echo-contrib"
PKG_LIST := $(shell go list ${PKG}/...)

.DEFAULT_GOAL := check
check: lint vet race ## Check project

init:
	@go install honnef.co/go/tools/cmd/staticcheck@latest

format: ## Format the source code
	@find ./ -type f -name "*.go" -exec gofmt -w {} \;

lint: ## Lint the files
	@staticcheck -tests=false ${PKG_LIST}

vet: ## Vet the files
	@go vet ${PKG_LIST}

test: ## Run tests
	@go test -short ${PKG_LIST}

race: ## Run tests with data race detector
	@go test -race ${PKG_LIST}

benchmark: ## Run benchmarks
	@go test -run="-" -bench=".*" ${PKG_LIST}

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

goversion ?= "1.18"
test_version: ## Run tests inside Docker with given version (defaults to 1.18 oldest supported). Example: make test_version goversion=1.18
	@docker run --rm -it -v $(shell pwd):/project golang:$(goversion) /bin/sh -c "cd /project && make race"
