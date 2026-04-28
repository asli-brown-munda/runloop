.PHONY: build test fmt fmt-check lint lint-install run-daemon run-cli clean

GOLANGCI_LINT_VERSION ?= v2.8.0
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || printf '%s/bin/golangci-lint' "$$(go env GOPATH)")
GOIMPORTS ?= $(shell command -v goimports 2>/dev/null || printf '%s/bin/goimports' "$$(go env GOPATH)")

build:
	mkdir -p bin
	go build -o bin/runloopd ./cmd/runloopd
	go build -o bin/runloop ./cmd/runloop

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal
	@test -x "$(GOIMPORTS)" || { echo "goimports is not installed. Run 'go install golang.org/x/tools/cmd/goimports@latest'."; exit 1; }
	$(GOIMPORTS) -w ./cmd ./internal

fmt-check:
	@unformatted="$$(gofmt -l ./cmd ./internal)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt is required on:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@test -x "$(GOIMPORTS)" || { echo "goimports is not installed. Run 'go install golang.org/x/tools/cmd/goimports@latest'."; exit 1; }
	@unimported="$$( $(GOIMPORTS) -l ./cmd ./internal )"; \
	if [ -n "$$unimported" ]; then \
		echo "goimports is required on:"; \
		echo "$$unimported"; \
		exit 1; \
	fi

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
