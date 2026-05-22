.SUFFIXES:

# fd compilation demo (no default features). Four stages
# (binary -> clone -> plan -> run -> prove). Each stage is its own
# target so callers can re-run any single piece in isolation.
#
# No custom Dockerfile -- this variant runs everything against the
# upstream ubuntu/rust:1.85-24.04_edge image directly. The `run` stage
# deletes cargo inside the container at the start, before invoking
# not-quite-cargo (see run.sh).
#
# Overridable knobs:
#   FD_REF=<tag-or-sha>    pinned upstream revision (default v10.2.0)
#   DOCKER=<docker|podman> container runtime (default docker)

FD_REF             ?= v10.2.0
DOCKER             ?= docker
IMAGE              ?= docker.io/ubuntu/rust:1.85-24.04_edge
FD_REPO            ?= https://github.com/sharkdp/fd.git
PROVE_IMAGE        ?= docker.io/ubuntu:24.04

SCRIPT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
GO_ROOT    := $(SCRIPT_DIR)/../..
WORKDIR    := $(SCRIPT_DIR)/work
BINARY     := $(WORKDIR)/not-quite-cargo
FD_DIR     := $(WORKDIR)/fd

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
clone: $(FD_DIR)  ## clone fd at the pinned ref

$(FD_DIR):
	@echo "[demo] cloning fd @ $(FD_REF)"
	git -c advice.detachedHead=false clone --depth 1 --branch $(FD_REF) $(FD_REPO) $(FD_DIR)

.PHONY: plan
plan: binary clone  ## generate & patch build-plan.json
	@echo "[demo] planner: generating build-plan.json in container"
	mkdir -p $(WORKDIR)/cargo-home
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(FD_DIR):/work \
		--volume $(WORKDIR)/cargo-home:/cargo-home \
		--workdir /work \
		-e CARGO_HOME=/cargo-home -e RUSTC_BOOTSTRAP=1 \
		$(IMAGE) exec sh -c 'set -e; cd /work; cargo build -j1 --no-default-features -Z unstable-options --build-plan > build-plan.json'
	@echo "[demo] planner: patching build-plan.json on host"
	cd $(GO_ROOT) && go run ./cmd/not-quite-cargo patch --project-root=/work --cargo-home=/cargo-home --inplace $(FD_DIR)/build-plan.json

.PHONY: run
run: binary plan  ## execute the patched plan, no cargo (deleted at run start), no network
	@echo "[demo] runner: deleting cargo + executing patched plan (network=none)"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(FD_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home:ro \
		--volume $(SCRIPT_DIR)/run.sh:/run.sh:ro \
		--workdir /work \
		-e CARGO_HOME=/cargo-home \
		$(IMAGE) exec sh /run.sh
	@echo "[demo] built artefacts:"
	@ls -1 $(FD_DIR)/target/debug/fd 2>/dev/null || true

.PHONY: prove
prove: run  ## drop built fd into ubuntu:24.04, run fd --version
	@echo "[demo] prove: drop built fd into $(PROVE_IMAGE), run fd --version"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(FD_DIR)/target:/fd-target:ro \
		--volume $(SCRIPT_DIR)/prove.sh:/prove.sh:ro \
		$(PROVE_IMAGE) \
		bash /prove.sh

.PHONY: shell
shell:  ## drop into a shell in the upstream image
	$(DOCKER) run --rm -it \
		--platform=linux/$(HOST_ARCH) \
		--volume $(FD_DIR):/work \
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
