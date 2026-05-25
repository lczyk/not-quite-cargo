# ubuntu/rust:1.85-24.04_edge + zig (used as the c++ compiler via a
# wrapper shim) for uu_stdbuf's libstdbuf build script. The build
# script goes through the `cpp` / `cpp_build` crate, which shells out
# to `c++` at build-script time.
#
# Toolchain install lives in cc-setup.sh -- side-installs curl, pulls
# the pinned zig tarball, drops /usr/local/bin/{c++,cc} shims that
# exec `zig c++` / `zig cc`.
#
# Reused for both the plan stage (cargo kept) and the run stage (cargo
# removed) via REMOVE_CARGO. uutils/coreutils 0.0.30 needs a recent
# rustc; 1.85 is fine.
FROM docker.io/ubuntu/rust:1.85-24.04_edge

ARG ZIG_VERSION=0.13.0
ENV ZIG_VERSION=${ZIG_VERSION}

COPY cc-setup.sh /tmp/cc-setup.sh
RUN sh -ex /tmp/cc-setup.sh && rm /tmp/cc-setup.sh

ARG REMOVE_CARGO=1
RUN if [ "$REMOVE_CARGO" = "1" ]; then \
        rm -f /usr/local/cargo/bin/cargo /usr/lib/rust-1.85/bin/cargo; \
    fi
