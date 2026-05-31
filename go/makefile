.SUFFIXES:

# `./internal` is gitignored (holds the generated version.go); only
# include it in the find if it exists, otherwise fresh clones error.
SRCS := $(shell find ./cmd ./cargo $$([ -d ./internal ] && echo ./internal) -name '*.go' ! -name '*_test.go' ! -name 'version.go')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: generate-version
generate-version:  ## Generate internal/version/version.go from VERSION + git state
	mkdir -p ./internal/version
	go run github.com/lczyk/version/go/cmd/generate-version -out ./internal/version/version.go -pkg version

.PHONY: build
build: ./bin/not-quite-cargo  ## Build the binary (upx-compressed if available)

./bin/not-quite-cargo: $(SRCS) makefile go.mod go.sum generate-version
	mkdir -p ./bin
	go build -ldflags="-s -w" -o $@ ./cmd/not-quite-cargo
	@if command -v upx >/dev/null 2>&1; then \
		upx $@ || echo "upx failed, skipping compression"; \
	fi
	@echo "Built $@ ($$(du -h $@ | cut -f1))"

.PHONY: test-unit
test-unit: generate-version  ## Run the Go unit tests
	go test -race ./...

.PHONY: test-examples
test-examples:  ## Run each examples/*/makefile end-to-end (PASS/FAIL only)
	@for d in examples/*/; do \
		if [ -f "$$d/makefile" ] || [ -f "$$d/Makefile" ]; then \
			name=$$(basename "$$d"); \
			echo "[test-examples] running $$name ..."; \
			if $(MAKE) --no-print-directory -C "$$d" _quiet; then \
				echo "[test-examples] PASS $$name"; \
			else \
				rc=$$?; \
				echo "[test-examples] FAIL $$name -- log: $$d/work/quiet.log"; \
				cat "$$d/work/quiet.log" 2>/dev/null || true; \
				exit $$rc; \
			fi; \
		fi; \
	done

.PHONY: test
test: test-unit test-examples  ## Run unit tests + example demos

.PHONY: lint
lint: generate-version  ## go vet + gofmt check (no writes)
	go vet ./...
	@out=$$(gofmt -s -l ./cmd ./cargo ./internal); \
	if [ -n "$$out" ]; then echo "Unformatted files:"; echo "$$out"; exit 1; fi

.PHONY: fmt
fmt:  ## gofmt the tree in place
	gofmt -s -w ./cmd ./cargo ./internal

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
	rm -f ./internal/version/version.go
