.SUFFIXES:

# eza compilation demo. Four stages
# (binary -> clone -> plan -> run -> prove). Each stage is its own
# target so callers can re-run any single piece in isolation.
#
# No custom Dockerfile -- this variant runs everything against the
# upstream ubuntu/rust:1.85-24.04_edge image directly. The `run` stage
# deletes cargo inside the container at the start, before invoking
# not-quite-cargo (see run.sh).
#
# Overridable knobs:
#   EZA_REF=<tag-or-sha>    pinned upstream revision (default v0.20.24)
#   DOCKER=<docker|podman>  container runtime (default docker)

EZA_REF            ?= v0.20.24
DOCKER             ?= docker
IMAGE              ?= docker.io/ubuntu/rust:1.85-24.04_edge
EZA_REPO           ?= https://github.com/eza-community/eza.git
PROVE_IMAGE        ?= docker.io/ubuntu:24.04

SCRIPT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
GO_ROOT    := $(SCRIPT_DIR)/../..
WORKDIR    := $(SCRIPT_DIR)/work
BINARY     := $(WORKDIR)/not-quite-cargo
EZA_DIR    := $(WORKDIR)/eza

# Map host -m output to the rust/docker arch token (arm64 / amd64).
HOST_ARCH := $(shell uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: binary clone plan run prove  ## Run the full demo end to end

.PHONY: binary
binary: $(BINARY)  ## build nqc for linux/<arch>

$(BINARY): $(shell find $(GO_ROOT)/cargo $(GO_ROOT)/cmd -name '*.go' ! -name '*_test.go') $(GO_ROOT)/go.mod $(GO_ROOT)/go.sum makefile
	@echo "[demo] cross-building not-quite-cargo for linux/$(HOST_ARCH)"
	mkdir -p $(WORKDIR)
	cd $(GO_ROOT) && GOOS=linux GOARCH=$(HOST_ARCH) CGO_ENABLED=0 \
		go build -ldflags="-s -w" -o $(BINARY) ./cmd/not-quite-cargo

.PHONY: clone
clone: $(EZA_DIR)  ## clone eza at the pinned ref

$(EZA_DIR):
	@echo "[demo] cloning eza @ $(EZA_REF)"
	git -c advice.detachedHead=false clone --depth 1 --branch $(EZA_REF) $(EZA_REPO) $(EZA_DIR)

.PHONY: plan
plan: binary clone  ## generate & patch build-plan.json
	@echo "[demo] planner: generating build-plan.json in container"
	mkdir -p $(WORKDIR)/cargo-home
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(EZA_DIR):/work \
		--volume $(WORKDIR)/cargo-home:/cargo-home \
		--workdir /work \
		-e CARGO_HOME=/cargo-home -e RUSTC_BOOTSTRAP=1 \
		$(IMAGE) exec sh -c 'set -e; cd /work; cargo build -j1 --no-default-features -Z unstable-options --build-plan > build-plan.json'
	@echo "[demo] planner: patching build-plan.json on host"
	cd $(GO_ROOT) && go run ./cmd/not-quite-cargo patch --project-root=/work --cargo-home=/cargo-home --inplace $(EZA_DIR)/build-plan.json

.PHONY: run
run: binary plan  ## execute the patched plan, no cargo (deleted at run start), no network
	@echo "[demo] runner: deleting cargo + executing patched plan (network=none)"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(EZA_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home:ro \
		--volume $(SCRIPT_DIR)/run.sh:/run.sh:ro \
		--workdir /work \
		-e CARGO_HOME=/cargo-home \
		$(IMAGE) exec sh /run.sh
	@echo "[demo] built artefacts:"
	@ls -1 $(EZA_DIR)/target/debug/eza 2>/dev/null || true

.PHONY: prove
prove: run  ## drop built eza into ubuntu:24.04, run eza --version
	@echo "[demo] prove: drop built eza into $(PROVE_IMAGE), run eza --version"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(EZA_DIR)/target:/eza-target:ro \
		--volume $(SCRIPT_DIR)/prove.sh:/prove.sh:ro \
		$(PROVE_IMAGE) \
		bash /prove.sh

.PHONY: shell
shell:  ## drop into a shell in the upstream image
	$(DOCKER) run --rm -it \
		--platform=linux/$(HOST_ARCH) \
		--volume $(EZA_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--workdir /work \
		$(IMAGE) exec sh

.PHONY: clean
clean:  ## Remove work/ (binaries, clone, cargo cache, target)
	rm -rf $(WORKDIR)

# Internal: run `all` with all output redirected to work/quiet.log.
# Exits with `all`'s exit code, no printing. Used by the top-level
# `make test-examples` runner, which handles PASS/FAIL reporting.
.PHONY: _quiet
_quiet:
	@mkdir -p $(WORKDIR); $(MAKE) --no-print-directory all >$(WORKDIR)/quiet.log 2>&1
