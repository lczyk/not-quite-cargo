.SUFFIXES:

# sudo-rs compilation demo. six stages
# (image -> binary -> cross -> clone -> plan -> run -> prove). Each
# stage is its own target so callers can re-run any single piece in
# isolation.
#
# Overridable knobs:
#   SUDO_RS_REF=<tag-or-sha>   pinned upstream revision (default v0.2.3)
#   DOCKER=<docker|podman>     container runtime (default docker)

SUDO_RS_REF        ?= v0.2.3
DOCKER             ?= docker
DEMO_IMAGE         ?= nqc-sudo-demo:1.75-24.04
PLAN_IMAGE         ?= docker.io/ubuntu/rust:1.75-24.04_stable
CROSS_IMAGE        ?= docker.io/ubuntu/rust:1.85-24.04_edge
SUDO_RS_REPO       ?= https://github.com/trifectatechfoundation/sudo-rs.git
PROVE_IMAGE        ?= docker.io/ubuntu:24.04

SCRIPT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
RUST_ROOT   := $(shell realpath $(SCRIPT_DIR)/../..)
WORKDIR    := $(SCRIPT_DIR)/work
BINARY     := $(WORKDIR)/not-quite-cargo
CROSS := $(WORKDIR)/not-quite-cargo-cross
SUDO_RS_DIR := $(WORKDIR)/sudo-rs

# Map host -m output to the rust/docker arch token (arm64 / amd64).
HOST_ARCH := $(shell uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: image binary cross clone plan run prove  ## Run the full demo end to end

.PHONY: image
image:  ## Build the rust+libpam demo image
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
cross: $(CROSS)  ## compile nqc for linux inside the plan image

$(CROSS): $(shell find $(RUST_ROOT)/src -name '*.rs') $(RUST_ROOT)/Cargo.toml $(RUST_ROOT)/Cargo.lock $(RUST_ROOT)/build.rs makefile
	@echo "[demo] cross-compiling not-quite-cargo for linux inside $(CROSS_IMAGE)"
	mkdir -p $(WORKDIR)
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(RUST_ROOT):/rust \
		--volume $(WORKDIR):/out \
		$(CROSS_IMAGE) exec sh -c 'set -e; apt-get -qq update; apt-get -qq install -y --no-install-recommends git; apt-get -qq clean; git -c advice.detachedHead=false clone --branch rust/v0.5.0 https://github.com/lczyk/version /tmp/version; cd /rust; CARGO_TARGET_DIR=/tmp/target cargo build --release --config "patch.\"https://github.com/lczyk/version\".version.path=\"/tmp/version/rust\""; cp /tmp/target/release/not-quite-cargo /out/not-quite-cargo-cross'

.PHONY: clone
clone: $(SUDO_RS_DIR)  ## clone sudo-rs at the pinned ref

$(SUDO_RS_DIR):
	@echo "[demo] cloning sudo-rs @ $(SUDO_RS_REF)"
	git -c advice.detachedHead=false clone --depth 1 --branch $(SUDO_RS_REF) $(SUDO_RS_REPO) $(SUDO_RS_DIR)

.PHONY: plan
plan: binary clone  ## generate & patch build-plan.json
	@echo "[demo] planner: generating build-plan.json in container"
	mkdir -p $(WORKDIR)/cargo-home
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(SUDO_RS_DIR):/work \
		--volume $(WORKDIR)/cargo-home:/cargo-home \
		--workdir /work \
		-e CARGO_HOME=/cargo-home -e RUSTC_BOOTSTRAP=1 \
		$(PLAN_IMAGE) exec sh -c 'set -e; cd /work; cargo build -j1 -Z unstable-options --build-plan > build-plan.json'
	@echo "[demo] planner: patching build-plan.json on host"
	$(BINARY) patch --project-root=/work --cargo-home=/cargo-home --inplace $(SUDO_RS_DIR)/build-plan.json

.PHONY: run
run: image cross plan  ## execute the patched plan, no cargo, no network
	@echo "[demo] runner: executing patched plan (network=none, cargo removed)"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(SUDO_RS_DIR):/work \
		--volume $(CROSS):/usr/local/bin/not-quite-cargo:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home:ro \
		--workdir /work \
		-e CARGO_HOME=/cargo-home \
		$(DEMO_IMAGE) exec sh -c 'set -e; cd /work; not-quite-cargo run build-plan.json'
	@echo "[demo] built artefacts:"
	@ls -1 $(SUDO_RS_DIR)/target/debug/{sudo,su,visudo} 2>/dev/null || true

.PHONY: prove
prove: run  ## drop built sudo into ubuntu:24.04, run sudo whoami
	@echo "[demo] prove: drop built sudo into $(PROVE_IMAGE), run sudo whoami"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(SUDO_RS_DIR)/target:/sudo-target:ro \
		--volume $(SCRIPT_DIR)/prove.sh:/prove.sh:ro \
		$(PROVE_IMAGE) \
		bash /prove.sh

.PHONY: shell
shell: image  ## drop into a shell in the demo image
	$(DOCKER) run --rm -it \
		--platform=linux/$(HOST_ARCH) \
		--volume $(SUDO_RS_DIR):/work \
		--volume $(CROSS):/usr/local/bin/not-quite-cargo:ro \
		--workdir /work \
		$(DEMO_IMAGE) exec sh

.PHONY: clean
clean:  ## Remove work/ (binaries, clone, cargo cache, target)
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
