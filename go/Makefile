.SUFFIXES:

SRCS := $(shell find ./cmd ./cargo -name '*.go' ! -name '*_test.go')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ./bin/not-quite-cargo  ## Build the binary (upx-compressed if available)

./bin/not-quite-cargo: $(SRCS) Makefile go.mod go.sum
	mkdir -p ./bin
	go build -ldflags="-s -w" -o $@ ./cmd/not-quite-cargo
	@if command -v upx >/dev/null 2>&1; then \
		upx $@ || echo "upx failed, skipping compression"; \
	fi
	@echo "Built $@ ($$(du -h $@ | cut -f1))"

.PHONY: test
test:  ## Run the test suite
	go test -race ./...

.PHONY: lint
lint:  ## go vet + gofmt check (no writes)
	go vet ./...
	@out=$$(gofmt -s -l ./cmd ./cargo); \
	if [ -n "$$out" ]; then echo "Unformatted files:"; echo "$$out"; exit 1; fi

.PHONY: fmt
fmt:  ## gofmt the tree in place
	gofmt -s -w ./cmd ./cargo

.PHONY: cover
cover:  ## Coverage profile + HTML report (cover.out, cover.html)
	go test -coverpkg=./... -coverprofile=cover.out -race ./...
	go tool cover -func=cover.out
	go tool cover -html=cover.out -o cover.html

.PHONY: cover-open
cover-open: cover  ## Run coverage and open the HTML report in a browser
	go tool cover -html=cover.out

.PHONY: clean
clean:  ## Remove build artifacts
	rm -rf ./bin
	rm -f cover.out cover.html
