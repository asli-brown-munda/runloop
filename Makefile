.PHONY: build test fmt lint lint-install run-daemon run-cli clean

GOLANGCI_LINT_VERSION ?= v2.8.0
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || printf '%s/bin/golangci-lint' "$$(go env GOPATH)")

build:
	mkdir -p bin
	go build -o bin/runloopd ./cmd/runloopd
	go build -o bin/runloop ./cmd/runloop

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

lint:
	go vet ./...
	@test -x "$(GOLANGCI_LINT)" || { echo "golangci-lint is not installed. Run 'make lint-install'."; exit 1; }
	$(GOLANGCI_LINT) run ./...

lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

run-daemon:
	go run ./cmd/runloopd

run-cli:
	go run ./cmd/runloop

clean:
	rm -rf bin coverage.out
