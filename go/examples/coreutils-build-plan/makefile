.SUFFIXES:

# uutils coreutils compilation demo. Five stages
# (image -> binary -> clone -> plan -> run -> prove). Each stage is
# its own target so callers can re-run any single piece in isolation.
#
# Custom Dockerfile (vs the upstream-as-is fd-no-features / eza-build-plan
# pattern) -- needed because uu_stdbuf's libstdbuf build script invokes
# c++ via the cpp_build crate, so both plan + run images need g++.
#
# Overridable knobs:
#   COREUTILS_REF=<tag-or-sha>  pinned upstream revision (default 0.0.30)
#   DOCKER=<docker|podman>      container runtime (default docker)

COREUTILS_REF      ?= 0.0.30
DOCKER             ?= docker
DEMO_IMAGE         ?= nqc-coreutils-demo:1.85-24.04
PLAN_IMAGE         ?= nqc-coreutils-plan:1.85-24.04
COREUTILS_REPO     ?= https://github.com/uutils/coreutils.git
PROVE_IMAGE        ?= docker.io/ubuntu:24.04

SCRIPT_DIR     := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
GO_ROOT        := $(SCRIPT_DIR)/../..
WORKDIR        := $(SCRIPT_DIR)/work
BINARY         := $(WORKDIR)/not-quite-cargo
COREUTILS_DIR  := $(WORKDIR)/coreutils

# Map host -m output to the rust/docker arch token (arm64 / amd64).
HOST_ARCH := $(shell uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: image binary clone plan run prove  ## Run the full demo end to end

.PHONY: image
image:  ## Build the plan + demo images (cargo kept / removed)
	@if ! $(DOCKER) image inspect $(PLAN_IMAGE) >/dev/null 2>&1; then \
		echo "[demo] building plan image $(PLAN_IMAGE)"; \
		$(DOCKER) build --platform=linux/$(HOST_ARCH) --build-arg REMOVE_CARGO=0 -t $(PLAN_IMAGE) $(SCRIPT_DIR); \
	else \
		echo "[demo] plan image $(PLAN_IMAGE) already built (run make clean-image to rebuild)"; \
	fi
	@if ! $(DOCKER) image inspect $(DEMO_IMAGE) >/dev/null 2>&1; then \
		echo "[demo] building demo image $(DEMO_IMAGE)"; \
		$(DOCKER) build --platform=linux/$(HOST_ARCH) --build-arg REMOVE_CARGO=1 -t $(DEMO_IMAGE) $(SCRIPT_DIR); \
	else \
		echo "[demo] demo image $(DEMO_IMAGE) already built (run make clean-image to rebuild)"; \
	fi

.PHONY: binary
binary: $(BINARY)  ## build nqc for linux/<arch>

$(BINARY): $(shell find $(GO_ROOT)/cargo $(GO_ROOT)/cmd -name '*.go' ! -name '*_test.go') $(GO_ROOT)/go.mod $(GO_ROOT)/go.sum makefile
	@echo "[demo] cross-building not-quite-cargo for linux/$(HOST_ARCH)"
	mkdir -p $(WORKDIR)
	cd $(GO_ROOT) && GOOS=linux GOARCH=$(HOST_ARCH) CGO_ENABLED=0 \
		go build -ldflags="-s -w" -o $(BINARY) ./cmd/not-quite-cargo

.PHONY: clone
clone: $(COREUTILS_DIR)  ## clone coreutils at the pinned ref

$(COREUTILS_DIR):
	@echo "[demo] cloning coreutils @ $(COREUTILS_REF)"
	git -c advice.detachedHead=false clone --depth 1 --branch $(COREUTILS_REF) $(COREUTILS_REPO) $(COREUTILS_DIR)

.PHONY: plan
plan: image binary clone  ## generate & patch build-plan.json
	@echo "[demo] planner: generating build-plan.json in container"
	mkdir -p $(WORKDIR)/cargo-home
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(COREUTILS_DIR):/work \
		--volume $(WORKDIR)/cargo-home:/cargo-home \
		--workdir /work \
		-e CARGO_HOME=/cargo-home -e RUSTC_BOOTSTRAP=1 \
		$(PLAN_IMAGE) exec sh -c 'set -e; cd /work; cargo build -j1 --no-default-features --features unix -Z unstable-options --build-plan > build-plan.json'
	@echo "[demo] planner: patching build-plan.json on host"
	cd $(GO_ROOT) && go run ./cmd/not-quite-cargo patch --project-root=/work --cargo-home=/cargo-home --inplace $(COREUTILS_DIR)/build-plan.json

.PHONY: run
run: image binary plan  ## execute the patched plan, no cargo, no network
	@echo "[demo] runner: executing patched plan (network=none, cargo removed)"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(COREUTILS_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home:ro \
		--workdir /work \
		-e CARGO_HOME=/cargo-home \
		$(DEMO_IMAGE) exec sh -c 'set -e; cd /work; not-quite-cargo run build-plan.json'
	@echo "[demo] built artefacts:"
	@ls -1 $(COREUTILS_DIR)/target/debug/coreutils 2>/dev/null || true

.PHONY: prove
prove: run  ## drop built coreutils into ubuntu:24.04, run coreutils --version
	@echo "[demo] prove: drop built coreutils into $(PROVE_IMAGE), run coreutils --version"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(COREUTILS_DIR)/target:/coreutils-target:ro \
		--volume $(SCRIPT_DIR)/prove.sh:/prove.sh:ro \
		$(PROVE_IMAGE) \
		bash /prove.sh

.PHONY: shell
shell: image  ## drop into a shell in the demo image
	$(DOCKER) run --rm -it \
		--platform=linux/$(HOST_ARCH) \
		--volume $(COREUTILS_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--workdir /work \
		$(DEMO_IMAGE) exec sh

.PHONY: clean
clean:  ## Remove work/ (binaries, clone, cargo cache, target)
	rm -rf $(WORKDIR)

.PHONY: clean-image
clean-image:  ## delete the cached plan + demo images so they get rebuilt
	$(DOCKER) image rm $(DEMO_IMAGE) || true
	$(DOCKER) image rm $(PLAN_IMAGE) || true

# Internal: run `all` with all output redirected to work/quiet.log.
# Exits with `all`'s exit code, no printing. Used by the top-level
# `make test-examples` runner, which handles PASS/FAIL reporting.
.PHONY: _quiet
_quiet:
	@mkdir -p $(WORKDIR); $(MAKE) --no-print-directory all >$(WORKDIR)/quiet.log 2>&1
