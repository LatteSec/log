default: all

## default: Runs build and test
.PHONY: default
all: help

# =================================== HELPERS =================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'CTFx Logger - A CTF hosting platform'
	@echo ''
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Commands:'
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' |  sed -e 's/^/ /'




## install: Install dependencies
.PHONY: install
install:
	go get ./...

## test: Runs tests
.PHONY: test
test:
	go mod tidy
	go mod verify
	go vet ./...
	go test -race ./...

## bench: Run benchmarks
bench:
	go test -v -bench=. -benchmem ./...

## lint: Lint code
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run

## security: Run security checks
.PHONY: security
security:
	go run github.com/securego/gosec/v2/cmd/gosec@latest -quiet ./...
	go run github.com/go-critic/go-critic/cmd/gocritic@latest check -enableAll ./...
	go run github.com/google/osv-scanner/cmd/osv-scanner@latest -r .

## fmt: Format code
.PHONY: fmt
fmt:
	go fmt ./...
	go mod tidy -v
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fix

## tidy: Clean up code artifacts
.PHONY: tidy
tidy:
	go clean ./...
