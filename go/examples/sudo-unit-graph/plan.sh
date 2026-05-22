#!/bin/sh
# Plan stage of the demo. Runs inside the PLAN_IMAGE (has cargo + rustc).
# Reads NQC_ARCH from env (rust arch name -- aarch64 / x86_64).
# Produces unit-graph.json via cargo, then derives build-plan.json via
# `nqc build` and patches it -- deliberately *not* using cargo's
# removed-in-1.93 --build-plan flag.
set -ex

: "${NQC_ARCH:?NQC_ARCH must be set (aarch64|x86_64)}"

mkdir -p /cargo-home
cargo build -j1 -Z unstable-options --unit-graph > unit-graph.json
not-quite-cargo build \
    --os linux --arch "$NQC_ARCH" --libc gnu \
    unit-graph.json > build-plan.json
not-quite-cargo patch build-plan.json
