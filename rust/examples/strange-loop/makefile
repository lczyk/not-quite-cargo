.SUFFIXES:

# Strange-loop demo: nqc compiles nqc. Six stages
# (image -> binary -> cross -> clone -> plan -> prove). Same shape as
# sudo-build-plan, but the project being built is nqc itself. `prove`
# does the loop: cross nqc builds nqc, then the freshly built nqc
# builds nqc again from scratch in the same container.
#
# Overridable knobs:
#   VERSION_REF=<tag-or-sha>   pinned version-crate revision
#   DOCKER=<docker|podman>     container runtime (default docker)

VERSION_REF        ?= rust/v0.5.0
VERSION_REPO       ?= https://github.com/lczyk/version
DOCKER             ?= docker
DEMO_IMAGE         ?= nqc-strange-loop:1.85-24.04
PLAN_IMAGE         ?= docker.io/ubuntu/rust:1.85-24.04_edge
CROSS_IMAGE        ?= docker.io/ubuntu/rust:1.85-24.04_edge

SCRIPT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
RUST_ROOT   := $(shell realpath $(SCRIPT_DIR)/../..)
WORKDIR    := $(SCRIPT_DIR)/work
BINARY     := $(WORKDIR)/not-quite-cargo
CROSS      := $(WORKDIR)/not-quite-cargo-cross
VERSION_SRC := $(WORKDIR)/version-src
PLAN_FILE  := $(WORKDIR)/build-plan.json
TARGET_PROVE := $(WORKDIR)/target-prove

# Map host -m output to the rust/docker arch token (arm64 / amd64).
HOST_ARCH := $(shell uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: clean image binary cross clone plan prove  ## Run the full demo end to end (wipes work/ first)

.PHONY: image
image:  ## Build the strange-loop demo image (no cargo)
	@if ! $(DOCKER) image inspect $(DEMO_IMAGE) >/dev/null 2>&1; then \
		echo "[demo] building demo image $(DEMO_IMAGE)"; \
		$(DOCKER) build --platform=linux/$(HOST_ARCH) -t $(DEMO_IMAGE) $(SCRIPT_DIR); \
	else \
		echo "[demo] demo image $(DEMO_IMAGE) already built (run make clean-image to rebuild)"; \
	fi

.PHONY: binary
binary: $(BINARY)  ## build nqc for host (used by the patch step)

$(BINARY): $(shell find $(RUST_ROOT)/src -name '*.rs') $(RUST_ROOT)/Cargo.toml $(RUST_ROOT)/Cargo.lock $(RUST_ROOT)/build.rs makefile
	@echo "[demo] building not-quite-cargo for host"
	mkdir -p $(WORKDIR)
	cd $(RUST_ROOT) && cargo build --release
	cp $(RUST_ROOT)/target/release/not-quite-cargo $(BINARY)

.PHONY: cross
cross: $(CROSS)  ## compile nqc for linux inside the cross image

$(CROSS): $(shell find $(RUST_ROOT)/src -name '*.rs') $(RUST_ROOT)/Cargo.toml $(RUST_ROOT)/Cargo.lock $(RUST_ROOT)/build.rs makefile
	@echo "[demo] cross-compiling not-quite-cargo for linux inside $(CROSS_IMAGE)"
	mkdir -p $(WORKDIR)
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(RUST_ROOT):/rust \
		--volume $(WORKDIR):/out \
		$(CROSS_IMAGE) exec sh -c 'set -e; apt-get -qq update; apt-get -qq install -y --no-install-recommends git; apt-get -qq clean; git -c advice.detachedHead=false clone --branch $(VERSION_REF) $(VERSION_REPO) /tmp/version; cd /rust; CARGO_TARGET_DIR=/tmp/target cargo build --release --config "patch.\"$(VERSION_REPO)\".version.path=\"/tmp/version/rust\""; cp /tmp/target/release/not-quite-cargo /out/not-quite-cargo-cross'

.PHONY: clone
clone: $(VERSION_SRC)/.git  ## clone the version crate at the pinned ref

$(VERSION_SRC)/.git:
	@echo "[demo] cloning version crate @ $(VERSION_REF)"
	rm -rf $(VERSION_SRC)
	git -c advice.detachedHead=false clone --depth 1 --branch $(VERSION_REF) $(VERSION_REPO) $(VERSION_SRC)

.PHONY: plan
plan: binary clone  ## generate & patch build-plan.json
	@echo "[demo] planner: generating build-plan.json in container"
	mkdir -p $(WORKDIR)/cargo-home $(WORKDIR)/target-plan
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(RUST_ROOT):/work:ro \
		--volume $(WORKDIR)/target-plan:/work/target \
		--volume $(VERSION_SRC):/version-src \
		--volume $(WORKDIR):/out \
		--volume $(WORKDIR)/cargo-home:/cargo-home \
		--workdir /work \
		-e CARGO_HOME=/cargo-home -e RUSTC_BOOTSTRAP=1 \
		$(PLAN_IMAGE) exec sh -c 'set -e; cd /work; cargo build --release -j1 -Z unstable-options --build-plan --config "patch.\"$(VERSION_REPO)\".version.path=\"/version-src/rust\"" > /out/build-plan.json'
	@echo "[demo] planner: patching build-plan.json on host"
	$(BINARY) patch --project-root=/work --cargo-home=/cargo-home --inplace $(PLAN_FILE)

.PHONY: prove
prove: image cross plan  ## close the strange loop: cross nqc builds nqc, then built nqc rebuilds nqc
	@echo "[demo] prove: fresh rebuild from cross + source inside $(DEMO_IMAGE)"
	rm -rf $(TARGET_PROVE)
	mkdir -p $(TARGET_PROVE)
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(RUST_ROOT):/work:ro \
		--volume $(TARGET_PROVE):/work/target \
		--volume $(VERSION_SRC):/version-src:ro \
		--volume $(PLAN_FILE):/build-plan.json:ro \
		--volume $(CROSS):/usr/local/bin/not-quite-cargo:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home:ro \
		--volume $(SCRIPT_DIR)/prove.sh:/prove.sh:ro \
		--workdir /work \
		-e CARGO_HOME=/cargo-home \
		$(DEMO_IMAGE) exec sh /prove.sh

.PHONY: shell
shell: image  ## drop into a shell in the demo image
	$(DOCKER) run --rm -it \
		--platform=linux/$(HOST_ARCH) \
		--volume $(RUST_ROOT):/work:ro \
		--volume $(CROSS):/usr/local/bin/not-quite-cargo:ro \
		--workdir /work \
		$(DEMO_IMAGE) exec sh

.PHONY: clean
clean:  ## Remove work/ (binaries, plan, cargo cache, targets)
	rm -rf $(WORKDIR)

.PHONY: clean-image
clean-image:  ## delete the cached demo image so it gets rebuilt
	$(DOCKER) image rm $(DEMO_IMAGE) || true

# Internal: run `all` with all output redirected to work/quiet.log.
# Exits with `all`'s exit code, no printing. Used by the top-level
# `make test-examples` runner, which handles PASS/FAIL reporting.
.PHONY: _quiet
_quiet:
	@mkdir -p $(WORKDIR); $(MAKE) --no-print-directory all >$(WORKDIR)/quiet.log 2>&1
