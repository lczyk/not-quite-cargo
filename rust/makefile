.SUFFIXES:

SRCS := $(shell find ./src -name '*.rs')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ./bin/not-quite-cargo  ## Build the binary (upx-compressed if available)

./bin/not-quite-cargo: $(SRCS) Cargo.toml build.rs
	mkdir -p ./bin
	cargo build --release
	cp ./target/release/not-quite-cargo $@
	@if command -v upx >/dev/null 2>&1; then \
		upx $@ || echo "upx failed, skipping compression"; \
	fi
	@echo "Built $@ ($$(du -h $@ | cut -f1))"

.PHONY: test-unit
test-unit:  ## Run Rust unit tests
	cargo test

.PHONY: test-examples
test-examples:  ## Run each examples/*/makefile end-to-end
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
lint:  ## clippy + fmt check (no writes)
	cargo clippy -- -D warnings
	cargo fmt --check

.PHONY: fmt
fmt:  ## cargo fmt the tree in place
	cargo fmt

.PHONY: cover
cover:  ## Coverage profile (llvm-cov, if installed)
	@if command -v cargo-llvm-cov >/dev/null 2>&1; then \
		cargo llvm-cov; \
	else \
		echo "cargo-llvm-cov not installed; skipping coverage"; \
	fi

.PHONY: clean
clean:  ## Remove build artifacts
	cargo clean
	rm -rf ./bin
