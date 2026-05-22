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
# Paths inside the JSON files are container-internal (/tmp/fd,
# /cargo-home, the image's rustc path); they're stable across captures
# and the test wires up matching placeholders, so no host-side
# anonymisation step is needed. The nqc binary itself is not used inside
# the container -- only cargo + rustc.
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
        -e CARGO_HOME=/tmp/cargo-home \
        -e RUSTC_BOOTSTRAP=1 \
        --user "$(id -u):$(id -g)" \
        --workdir /tmp \
        "${RUST_IMAGE}" \
        bash -c '
            set -e
            git clone --depth 1 --branch "${FD_REF}" \
                https://github.com/sharkdp/fd.git /tmp/fd
            mkdir -p /tmp/cargo-home
            cd /tmp/fd
            cargo fetch -q
            cargo build -j1 -Z unstable-options --unit-graph \
                > /out/ug.json
            cargo build -j1 -Z unstable-options --build-plan \
                > /out/build-plan.json
            rustc --print cfg > /out/host-cfg.txt
        '

    _info "captured into ${SCRIPT_DIR}:"
    ls -lh "${SCRIPT_DIR}/ug.json" "${SCRIPT_DIR}/build-plan.json" "${SCRIPT_DIR}/host-cfg.txt"
}

main "$@"
