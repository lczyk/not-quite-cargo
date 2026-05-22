.SUFFIXES:

# sudo-rs compilation demo via nqc's --unit-graph experimental path.
# Six stages (image -> binary -> clone -> plan -> run -> prove).
#
# Deliberately *does not* use cargo --build-plan, even though the
# pinned rust:1.84 image still has it. Planner runs cargo --unit-graph
# then nqc build, mirroring what cargo >= 1.93 deployments must do
# (--build-plan was removed in 1.93.0).
#
# Overridable knobs:
#   SUDO_RS_REF=<tag-or-sha>   pinned upstream revision (default v0.2.3)
#   DOCKER=<docker|podman>     container runtime (default docker)

SUDO_RS_REF        ?= v0.2.3
DOCKER             ?= docker
DEMO_IMAGE         ?= nqc-sudo-ug-demo:1.93-26.04
PLAN_IMAGE         ?= ubuntu/rust:1.93-26.04_edge
SUDO_RS_REPO       ?= https://github.com/trifectatechfoundation/sudo-rs.git
PROVE_IMAGE        ?= ubuntu:26.04

SCRIPT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
GO_ROOT    := $(SCRIPT_DIR)/../..
WORKDIR    := $(SCRIPT_DIR)/work
BINARY     := $(WORKDIR)/not-quite-cargo
SUDO_RS_DIR := $(WORKDIR)/sudo-rs

# Map host -m output to the rust/docker arch token (arm64 / amd64).
HOST_ARCH := $(shell uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')
# nqc build wants the rust arch name (aarch64 / x86_64), not arm64 / amd64.
NQC_ARCH := $(shell uname -m | sed -e 's/arm64/aarch64/' -e 's/amd64/x86_64/')

help:  ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: image binary clone plan run prove  ## Run the full demo end to end

.PHONY: image
image:  ## Build the rust+libpam demo image (no cargo)
	@if ! $(DOCKER) image inspect $(DEMO_IMAGE) >/dev/null 2>&1; then \
		printf '%s building demo image %s\n' "[demo]" "$(DEMO_IMAGE)"; \
		$(DOCKER) build --platform=linux/$(HOST_ARCH) -t $(DEMO_IMAGE) $(SCRIPT_DIR); \
	else \
		printf '%s demo image %s already built (run make clean-image to rebuild)\n' "[demo]" "$(DEMO_IMAGE)"; \
	fi

.PHONY: binary
binary: $(BINARY)  ## build nqc for linux/<arch>

$(BINARY): $(shell find $(GO_ROOT)/cargo $(GO_ROOT)/cmd -name '*.go' ! -name '*_test.go') $(GO_ROOT)/go.mod $(GO_ROOT)/go.sum makefile
	@printf '%s cross-building not-quite-cargo for linux/%s\n' "[demo]" "$(HOST_ARCH)"
	mkdir -p $(WORKDIR)
	cd $(GO_ROOT) && GOOS=linux GOARCH=$(HOST_ARCH) CGO_ENABLED=0 \
		go build -ldflags="-s -w" -o $(BINARY) ./cmd/not-quite-cargo

.PHONY: clone
clone: $(SUDO_RS_DIR)  ## clone sudo-rs at the pinned ref

$(SUDO_RS_DIR):
	@printf '%s cloning sudo-rs @ %s\n' "[demo]" "$(SUDO_RS_REF)"
	git -c advice.detachedHead=false clone --depth 1 --branch $(SUDO_RS_REF) $(SUDO_RS_REPO) $(SUDO_RS_DIR)

.PHONY: plan
plan: binary clone  ## cargo --unit-graph + nqc build + nqc patch
	@printf '%s planner: cargo --unit-graph + nqc build + nqc patch\n' "[demo]"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--volume $(SUDO_RS_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home \
		--workdir /work \
		-e CARGO_HOME=/cargo-home -e RUSTC_BOOTSTRAP=1 \
		$(PLAN_IMAGE) exec sh -c 'set -e; cd /work; mkdir -p /cargo-home; cargo build -j1 -Z unstable-options --unit-graph > unit-graph.json; not-quite-cargo build --os linux --arch $(NQC_ARCH) --libc gnu unit-graph.json > build-plan.json; not-quite-cargo patch build-plan.json'

.PHONY: run
run: image binary plan  ## execute the patched plan, no cargo, no network
	@printf '%s runner: executing patched plan (network=none, cargo removed)\n' "[demo]"
	$(DOCKER) run --rm \
		--platform=linux/$(HOST_ARCH) \
		--network=none \
		--volume $(SUDO_RS_DIR):/work \
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--volume $(WORKDIR)/cargo-home:/cargo-home:ro \
		--workdir /work \
		-e CARGO_HOME=/cargo-home \
		$(DEMO_IMAGE) exec sh -c 'set -e; cd /work; not-quite-cargo run build-plan.json'
	@printf '[demo] built artefacts:\n'
	@ls -1 $(SUDO_RS_DIR)/target/debug/{sudo,su,visudo} 2>/dev/null || true

.PHONY: prove
prove: run  ## drop built sudo into ubuntu, run sudo whoami
	@printf '%s prove: drop built sudo into %s, run sudo whoami\n' "[demo]" "$(PROVE_IMAGE)"
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
		--volume $(BINARY):/usr/local/bin/not-quite-cargo:ro \
		--workdir /work \
		$(DEMO_IMAGE) exec sh

.PHONY: clean
clean:  ## Remove work/ (binaries, clone, cargo cache, target)
	rm -rf $(WORKDIR)

.PHONY: clean-image
clean-image:  ## delete the cached demo image so it gets rebuilt
	$(DOCKER) image rm $(DEMO_IMAGE) || true
