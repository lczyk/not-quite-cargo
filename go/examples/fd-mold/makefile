.SUFFIXES:

# fd compilation demo using mold as the linker via the nqc cc-driver
# (see ../../driver). Same shape as ../fd-wild but swaps wild out for
# rui314/mold to exercise the driver against a second, independent
# ld-style linker.
#
# Overridable knobs:
#   FD_REF=<tag-or-sha>      pinned upstream fd revision (default v10.2.0)
#   MOLD_VERSION=<x.y.z>     prebuilt mold release to grab (default 2.41.0)
#   DOCKER=<docker|podman>   container runtime (default docker)

FD_REF             ?= v10.2.0
MOLD_VERSION       ?= 2.41.0
DOCKER             ?= docker
IMAGE              ?= docker.io/ubuntu/rust:1.85-24.04_edge
FD_REPO            ?= https://github.com/sharkdp/fd.git
PROVE_IMAGE        ?= docker.io/ubuntu:24.04

SCRIPT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
GO_ROOT    := $(SCRIPT_DIR)/../..
WORKDIR    := $(SCRIPT_DIR)/work
BINARY     := $(WORKDIR)/not-quite-cargo
FD_DIR     := $(WORKDIR)/fd
MOLD_BIN   := $(WORKDIR)/mold

# Map host -m output to the rust/docker arch token (arm64 / amd64).
HOST_ARCH := $(shell uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')

# Map host arch to the mold release tarball arch token.
ifeq ($(HOST_ARCH),amd64)
  MOLD_ARCH := x86_64
else ifeq ($(HOST_ARCH),arm64)
  MOLD_ARCH := aarch64
else
  $(error unsupported HOST_ARCH: $(HOST_ARCH))
endif

MOLD_TARBALL := mold-$(MOLD_VERSION)-$(MOLD_ARCH)-linux.tar.gz
MOLD_URL     := https://github.com/rui314/mold/releases/download/v$(MOLD_VERSION)/$(MOLD_TARBALL)

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: binary mold clone plan run prove  ## Run the full demo end to end

.PHONY: binary
binary: $(BINARY)  ## build nqc for linux/<arch>

$(BINARY): $(shell find $(GO_ROOT)/cargo $(GO_ROOT)/cmd $(GO_ROOT)/driver -name '*.go' ! -name '*_test.go') $(GO_ROOT)/go.mod $(GO_ROOT)/go.sum makefile
	@echo "[demo] cross-building not-quite-cargo for linux/$(HOST_ARCH)"
	mkdir -p $(WORKDIR)
	cd $(GO_ROOT) && GOOS=linux GOARCH=$(HOST_ARCH) CGO_ENABLED=0 \
		go build -ldflags="-s -w" -o $(BINARY) ./cmd/not-quite-cargo

.PHONY: mold
mold: $(MOLD_BIN)  ## download the mold linker prebuilt binary

$(MOLD_BIN):
	@echo "[demo] downloading mold $(MOLD_VERSION) ($(MOLD_ARCH)-linux)"
	mkdir -p $(WORKDIR)
	curl -fsSL --retry 5 --retry-delay 5 -o $(WORKDIR)/$(MOLD_TARBALL) $(MOLD_URL)
	tar -xzf $(WORKDIR)/$(MOLD_TARBALL) -C $(WORKDIR)
	mv $(WORKDIR)/mold-$(MOLD_VERSION)-$(MOLD_ARCH)-linux/bin/mold $(MOLD_BIN)
	chmod +x $(MOLD_BIN)

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
	@echo "[demo] planner: patching build-plan.json on host (--linker=/usr/bin/cc -> nqc driver)"
	cd $(GO_ROOT) && go run ./cmd/not-quite-cargo patch \
		--project-root=/work --cargo-home=/cargo-home \
		--linker=/usr/bin/cc --inplace $(FD_DIR)/build-plan.json

.PHONY: run
run: binary mold plan  ## execute the patched plan with mold as the linker
	@echo "[demo] runner: replacing /usr/bin/{cc,gcc,ld,cargo}, installing mold + nqc, then running plan"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(FD_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--volume $(MOLD_BIN):/usr/local/bin/mold:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home:ro \
		--volume $(SCRIPT_DIR)/run.sh:/run.sh:ro \
		--workdir /work \
		-e CARGO_HOME=/cargo-home \
		$(IMAGE) exec sh /run.sh
	@echo "[demo] built artefacts:"
	@ls -1 $(FD_DIR)/target/debug/fd 2>/dev/null || true

.PHONY: prove
prove: run  ## drop built fd into ubuntu:24.04, run fd --version + a search
	@echo "[demo] prove: drop built fd into $(PROVE_IMAGE), run fd --version"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(FD_DIR)/target:/fd-target:ro \
		--volume $(SCRIPT_DIR)/prove.sh:/prove.sh:ro \
		$(PROVE_IMAGE) \
		bash /prove.sh

.PHONY: clean
clean:  ## Remove work/ (binaries, clone, cargo cache, target)
	rm -rf $(WORKDIR)

# Internal: run `all` with all output redirected to work/quiet.log.
# Used by the top-level `make test-examples` runner.
.PHONY: _quiet
_quiet:
	@mkdir -p $(WORKDIR); $(MAKE) --no-print-directory all >$(WORKDIR)/quiet.log 2>&1
