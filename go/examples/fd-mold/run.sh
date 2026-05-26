#!/bin/sh
# Run stage scriptlet for the fd-mold demo. Runs inside upstream
# ubuntu/rust:1.85-24.04_edge.
#
# Same shape as ../fd-wild/run.sh: strip the toolchain bindings,
# install mold at /usr/bin/mold, point /usr/bin/cc at the
# not-quite-cargo binary so rustc's cc-driver invocation lands in
# driver.Drive, which translates and forwards to mold.
set -e

# Remove the existing toolchain bindings so we can prove the build
# is using *only* nqc + mold for the link step.
rm -f /usr/bin/cc /usr/bin/gcc /usr/bin/ld /usr/bin/cargo \
      /usr/local/cargo/bin/cargo /usr/lib/rust-1.85/bin/cargo \
      /usr/lib/gcc-*/bin/* /usr/lib/llvm-*/bin/lld

# mold lives at /usr/local/bin/mold via the makefile bind-mount;
# expose it as /usr/bin/mold for the driver env var below.
ln -sf /usr/local/bin/mold /usr/bin/mold

# Symlink /usr/bin/cc to not-quite-cargo so rustc -C linker=/usr/bin/cc
# routes into nqc's driver via the argv[0]==cc shortcut.
ln -sf /usr/local/bin/not-quite-cargo /usr/bin/cc

# Sanity check: cc must not be the real gcc anymore.
command -v cargo >/dev/null 2>&1 && {
    echo "cargo still on PATH; demo invalid" >&2
    exit 1
}
test "$(readlink -f /usr/bin/cc)" = "/usr/local/bin/not-quite-cargo" || {
    echo "/usr/bin/cc not pointing at nqc; demo invalid" >&2
    exit 1
}

# Ubuntu 24.04 ships gcc-13, so point the driver at that lib dir.
# Triple is autodetected from runtime.GOARCH inside the driver.
arch=$(uname -m)
case "$arch" in
    aarch64) triple=aarch64-linux-gnu ;;
    x86_64)  triple=x86_64-linux-gnu ;;
    *)       echo "unsupported arch $arch" >&2; exit 1 ;;
esac
export NQC_DRIVER_GCC_LIB_DIR=/usr/lib/gcc/${triple}/13
# Tell the driver to forward to mold explicitly (the shim mode has no
# CLI of its own; env vars are the only knob).
export NQC_DRIVER_LINKER=/usr/bin/mold

cd /work
not-quite-cargo run build-plan.json
