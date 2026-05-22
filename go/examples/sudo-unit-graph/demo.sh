#!/usr/bin/env bash
# spellchecker: words rustc rustup
#
# sudo-rs compilation demo via nqc's --unit-graph experimental path.
#
# This example deliberately *does not* use cargo --build-plan, even
# though the pinned rust:1.84 image still has it. The planner stage
# runs cargo --unit-graph then nqc build to derive a plan from scratch
# -- mirroring what a cargo >= 1.93 deployment would have to do (since
# --build-plan was removed in 1.93.0).
#
# Three docker steps, all against the same image:
#   1. PLANNER  -- has cargo + rustc. clones sudo-rs, runs --unit-graph,
#                  then nqc build to produce build_plan.json.
#   2. PATCHER  -- nqc patch templates planner-side paths to placeholders.
#                  same container, separate step for clarity.
#   3. RUNNER   -- cargo stripped from PATH, network OFF. consumes the
#                  patched plan via nqc run.
#
# Run from the repo root or from go/examples/sudo-unit-graph. Set
# DEMO_SHELL=1 to drop into a shell in the runner. Set SUDO_RS_REF to
# pin a different sudo-rs tag/sha (default: v0.2.3).
set -e

# top-level configuration
DEMO_IMAGE="nqc-sudo-ug-demo:1.84"
SUDO_RS_REPO="https://github.com/trifectatechfoundation/sudo-rs.git"
SUDO_RS_REF="${SUDO_RS_REF:-v0.2.3}"
DOCKER="${DOCKER:-docker}"
SHELL_MODE="${DEMO_SHELL:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_ROOT="${SCRIPT_DIR}/../.."
WORKDIR="${SCRIPT_DIR}/work"
BINARY="${WORKDIR}/not-quite-cargo"
SUDO_RS_DIR="${WORKDIR}/sudo-rs"

# colour is on iff the host stream is a tty and NO_COLOR is unset.
_USE_COLOR=1
if [ -n "${NO_COLOR:-}" ]; then
    _USE_COLOR=0
fi
if [ ! -t 1 ] || [ ! -t 2 ]; then
    _USE_COLOR=0
fi

if [ "${_USE_COLOR}" = "1" ]; then
    _PREFIX_STDOUT=$'\033[32m[demo]\033[0m'
    _PREFIX_STDERR=$'\033[32m[demo]\033[0m'
    _TTY_FLAG="-t"
    _NO_COLOR_FLAG=""
else
    _PREFIX_STDOUT='[demo]'
    _PREFIX_STDERR='[demo]'
    _TTY_FLAG=""
    _NO_COLOR_FLAG="-e NO_COLOR=1"
fi

function _info() {
    printf '%s %s\n' "${_PREFIX_STDOUT}" "$1"
}

function _fail() {
    printf '%s error: %s\n' "${_PREFIX_STDERR}" "$1" >&2
    exit 1
}

function _host_arch() {
    local arch
    arch="$(uname -m)"
    case "${arch}" in
        (aarch64|arm64) printf 'arm64\n' ;;
        (x86_64|amd64)  printf 'amd64\n' ;;
        (*) _fail "unsupported host arch: ${arch}" ;;
    esac
}

function _build_image() {
    local arch="$1"
    if "${DOCKER}" image inspect "${DEMO_IMAGE}" >/dev/null 2>&1; then
        _info "demo image ${DEMO_IMAGE} already built (delete to rebuild)"
        return
    fi
    _info "building demo image ${DEMO_IMAGE} (rust:1.84 + libpam0g-dev)"
    "${DOCKER}" build \
        --platform=linux/"${arch}" \
        -t "${DEMO_IMAGE}" \
        "${SCRIPT_DIR}"
}

function _build_binary() {
    local arch="$1"
    _info "cross-building not-quite-cargo for linux/${arch}"
    mkdir -p "${WORKDIR}"
    ( cd "${GO_ROOT}" && \
        GOOS=linux GOARCH="${arch}" CGO_ENABLED=0 \
        go build -ldflags="-s -w" -o "${BINARY}" ./cmd/not-quite-cargo )
}

function _clone_sudo_rs() {
    if [ -d "${SUDO_RS_DIR}" ]; then
        _info "sudo-rs already cloned at ${SUDO_RS_DIR} (delete to refetch)"
        return
    fi
    _info "cloning sudo-rs @ ${SUDO_RS_REF}"
    git -c advice.detachedHead=false clone --depth 1 \
        --branch "${SUDO_RS_REF}" \
        "${SUDO_RS_REPO}" "${SUDO_RS_DIR}"
}

function _planner_run() {
    local arch="$1"
    _info "planner: cargo --unit-graph + nqc build + nqc patch"
    "${DOCKER}" run --rm ${_TTY_FLAG} \
        --platform=linux/"${arch}" \
        --volume "${SUDO_RS_DIR}":/work \
        --volume "${BINARY}":/usr/local/bin/not-quite-cargo:ro \
        --workdir /work \
        -e CARGO_HOME=/cargo-home \
        -e RUSTC_BOOTSTRAP=1 \
        -e DEMO_OS=linux \
        -e "DEMO_ARCH=${arch}" \
        ${_NO_COLOR_FLAG} \
        --user "$(id -u):$(id -g)" \
        --volume "${WORKDIR}/cargo-home":/cargo-home \
        "${DEMO_IMAGE}" \
        bash -c '
            set -e
            mkdir -p /cargo-home
            # --unit-graph is unstable; RUSTC_BOOTSTRAP=1 unlocks -Z on stable.
            cargo build -j1 -Z unstable-options --unit-graph > unit-graph.json
            # NOTE: pretending --build-plan does not exist. nqc build derives
            # an equivalent plan from --unit-graph alone.
            not-quite-cargo build \
                --os "${DEMO_OS}" --arch "${DEMO_ARCH}" --libc gnu \
                unit-graph.json > build_plan.json
            not-quite-cargo patch build_plan.json
        '
}

function _runner_run() {
    local arch="$1"
    local entrypoint=(bash -c '
        set -e
        rm -f /usr/local/cargo/bin/cargo
        command -v cargo >/dev/null 2>&1 && {
            printf "cargo still on PATH; demo invalid\n" >&2
            exit 1
        }
        not-quite-cargo run build_plan.json
        printf "built artefacts:\n"
        find target -maxdepth 3 -type f -name sudo -o -name su -o -name visudo
    ')
    local it="${_TTY_FLAG}"
    if [ "${SHELL_MODE}" = "1" ]; then
        it="-it"
        entrypoint=(bash)
    fi
    _info "runner: executing patched plan (network=none, cargo removed)"
    "${DOCKER}" run --rm ${it} \
        --platform=linux/"${arch}" \
        --network=none \
        --volume "${SUDO_RS_DIR}":/work \
        --volume "${BINARY}":/usr/local/bin/not-quite-cargo:ro \
        --volume "${WORKDIR}/cargo-home":/cargo-home:ro \
        --workdir /work \
        -e CARGO_HOME=/cargo-home \
        ${_NO_COLOR_FLAG} \
        --user "$(id -u):$(id -g)" \
        "${DEMO_IMAGE}" \
        "${entrypoint[@]}"
}

function _prove_run() {
    local arch="$1"
    _info "prove: drop built sudo into ubuntu:26.04, run 'sudo whoami'"
    # Fresh image (no rust toolchain). Network off -- proves the binary
    # carries its own runtime needs. install sets the setuid bit; minimal
    # /etc/sudoers + /etc/pam.d/sudo let the binary actually elevate.
    "${DOCKER}" run --rm ${_TTY_FLAG} \
        --platform=linux/"${arch}" \
        --network=none \
        --volume "${SUDO_RS_DIR}/target":/sudo-target:ro \
        ${_NO_COLOR_FLAG} \
        ubuntu:26.04 \
        bash -c '
            set -e
            BIN=$(find /sudo-target -name "sudo-*" -type f -executable | head -n1)
            [ -n "$BIN" ] || { echo "no sudo binary under /sudo-target" >&2; exit 1; }
            install -m 4755 -o root -g root "$BIN" /usr/local/bin/sudo
            echo "root ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers
            chmod 440 /etc/sudoers
            {
                echo "auth sufficient pam_permit.so"
                echo "account sufficient pam_permit.so"
                echo "session sufficient pam_permit.so"
            } > /etc/pam.d/sudo
            echo "linkage:"
            ldd /usr/local/bin/sudo
            echo
            echo "running: sudo whoami"
            sudo whoami
        '
}

function main() {
    command -v "${DOCKER}" >/dev/null 2>&1 || _fail "${DOCKER} not on PATH"
    command -v go >/dev/null 2>&1 || _fail "go not on PATH (need to cross-build the binary)"
    command -v git >/dev/null 2>&1 || _fail "git not on PATH"

    local arch
    arch="$(_host_arch)"
    _info "host arch: ${arch}"

    mkdir -p "${WORKDIR}"
    _build_image "${arch}"
    _build_binary "${arch}"
    _clone_sudo_rs
    _planner_run "${arch}"
    _runner_run "${arch}"
    _prove_run "${arch}"

    _info "done. output binary should be under ${SUDO_RS_DIR}/target/"
}

main "$@"
