#!/usr/bin/env bash
# Captures unit-graph + build-plan for fd at a pinned ref inside the
# rust:1.84 docker image (where --build-plan still works), then runs
# nqc patch to anonymise the planner-side paths. Output lands in this
# directory.
set -e

FD_REF="${FD_REF:-v10.2.0}"
RUST_IMAGE="${RUST_IMAGE:-rust:1.84}"
DOCKER="${DOCKER:-docker}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_ROOT="${SCRIPT_DIR}/../../../.."
WORK="${SCRIPT_DIR}/.capture-work"

function _info() {
    printf '[capture] %s\n' "$1"
}

function _host_arch() {
    local arch
    arch="$(uname -m)"
    case "${arch}" in
        (aarch64|arm64) printf 'arm64\n' ;;
        (x86_64|amd64)  printf 'amd64\n' ;;
        (*) printf '[capture] unsupported arch: %s\n' "${arch}" >&2; exit 1 ;;
    esac
}

function main() {
    local arch
    arch="$(_host_arch)"
    rm -rf "${WORK}"
    mkdir -p "${WORK}"

    _info "cross-building not-quite-cargo for linux/${arch}"
    ( cd "${GO_ROOT}/go" && \
        GOOS=linux GOARCH="${arch}" CGO_ENABLED=0 \
        go build -ldflags="-s -w" -o "${WORK}/not-quite-cargo" ./cmd/not-quite-cargo )

    _info "cloning fd @ ${FD_REF}"
    git clone --depth 1 --branch "${FD_REF}" https://github.com/sharkdp/fd.git "${WORK}/fd"

    _info "running cargo -Z unstable-options --unit-graph / --build-plan inside ${RUST_IMAGE}"
    "${DOCKER}" run --rm \
        --platform=linux/"${arch}" \
        --volume "${WORK}/fd":/work \
        --volume "${WORK}":/out \
        --workdir /work \
        -e CARGO_HOME=/cargo-home \
        -e RUSTC_BOOTSTRAP=1 \
        --user "$(id -u):$(id -g)" \
        --volume "${WORK}/cargo-home":/cargo-home \
        "${RUST_IMAGE}" \
        bash -c '
            set -e
            mkdir -p /cargo-home
            cargo build -j1 -Z unstable-options --unit-graph > /out/ug.raw.json
            cargo clean -q
            cargo build -j1 -Z unstable-options --build-plan > /out/build-plan.raw.json
        '

    _info "anonymising via nqc patch"
    # The nqc patch step operates on a build-plan-shape file; we run it
    # against build-plan.raw.json first to discover the planner paths,
    # then mirror the same string substitutions onto the unit-graph by
    # hand (deepReplace is in nqc proper but not surfaced via the cli).
    cp "${WORK}/build-plan.raw.json" "${WORK}/build-plan.json"
    PROJECT_ROOT="${WORK}/fd" CARGO_HOME="${WORK}/cargo-home" \
        "${WORK}/not-quite-cargo" patch "${WORK}/build-plan.json"

    # Manual substitution for the unit-graph (string-only, no schema
    # interpretation needed).
    python3 - <<PY
import json, sys
proj = "${WORK}/fd"
cargo = "${WORK}/cargo-home"
with open("${WORK}/ug.raw.json") as f:
    data = json.load(f)
def walk(x):
    if isinstance(x, dict):
        return {walk(k): walk(v) for k, v in x.items()}
    if isinstance(x, list):
        return [walk(i) for i in x]
    if isinstance(x, str):
        return x.replace(cargo, "{{CARGO_HOME}}").replace(proj, "{{PROJECT_ROOT}}")
    return x
with open("${SCRIPT_DIR}/ug.json", "w") as f:
    json.dump(walk(data), f, indent=2)
PY

    cp "${WORK}/build-plan.json" "${SCRIPT_DIR}/build-plan.json"

    _info "captured into ${SCRIPT_DIR}/{ug,build-plan}.json"
    _info "ug.json: $(du -h "${SCRIPT_DIR}/ug.json" | cut -f1)"
    _info "build-plan.json: $(du -h "${SCRIPT_DIR}/build-plan.json" | cut -f1)"
}

main "$@"
