#!/usr/bin/env bash
# Capture cargo's --unit-graph + --build-plan + rustc --print cfg from
# inside the rust:1.84 image, where both unstable flags coexist (cargo
# 1.44 -- 1.92 is the overlap window; rust:1.84 sits in the middle).
#
# Drops three files into this directory:
#   ug.json          -- cargo build -Z unstable-options --unit-graph
#   build-plan.json  -- cargo build -Z unstable-options --build-plan
#   host-cfg.txt     -- rustc --print cfg (host = container's linux/<arch>)
#
# Container-side absolute paths (`/fd`, `/cargo-home`) are substituted on
# the host for the standard `{{PROJECT_ROOT}}` / `{{CARGO_HOME}}`
# placeholders so the fixtures are portable across hosts. The nqc binary
# itself is *not* used inside the container -- only cargo + rustc + sed.
set -e

FD_REF="${FD_REF:-v10.2.0}"
RUST_IMAGE="${RUST_IMAGE:-rust:1.84}"
DOCKER="${DOCKER:-docker}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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
    _info "host arch: ${arch}"
    _info "fd ref:    ${FD_REF}"
    _info "image:     ${RUST_IMAGE}"

    "${DOCKER}" run --rm \
        --platform=linux/"${arch}" \
        --volume "${SCRIPT_DIR}":/out \
        -e FD_REF="${FD_REF}" \
        -e CARGO_HOME=/cargo-home \
        -e RUSTC_BOOTSTRAP=1 \
        --user "$(id -u):$(id -g)" \
        --workdir / \
        "${RUST_IMAGE}" \
        bash -c '
            set -e
            cd /tmp
            git clone --depth 1 --branch "${FD_REF}" \
                https://github.com/sharkdp/fd.git /tmp/fd
            mkdir -p /cargo-home
            cd /tmp/fd
            cargo fetch -q
            cargo build -j1 -Z unstable-options --unit-graph \
                > /out/ug.raw.json
            cargo build -j1 -Z unstable-options --build-plan \
                > /out/build-plan.raw.json
            rustc --print cfg > /out/host-cfg.txt
        '

    _info "anonymising planner paths"
    # /fd and /cargo-home are fixed container-side absolute paths;
    # rewrite them to the standard nqc placeholders so the fixtures
    # work regardless of where the capture ran.
    for raw in "${SCRIPT_DIR}/ug.raw.json" "${SCRIPT_DIR}/build-plan.raw.json"; do
        sed -e 's|/tmp/fd|{{PROJECT_ROOT}}|g' \
            -e 's|/cargo-home|{{CARGO_HOME}}|g' \
            "${raw}" > "${raw%.raw.json}.json"
        rm "${raw}"
    done

    _info "captured into ${SCRIPT_DIR}:"
    ls -lh "${SCRIPT_DIR}/ug.json" "${SCRIPT_DIR}/build-plan.json" "${SCRIPT_DIR}/host-cfg.txt"
}

main "$@"
