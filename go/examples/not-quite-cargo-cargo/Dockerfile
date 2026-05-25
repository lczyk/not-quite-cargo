# ubuntu/rust:1.85-24.04_edge + zig (used as the c / c++ compiler via
# wrapper shims) + perl + make + git, for building rust-lang/cargo with
# `--features all-static`.
#
# all-static pulls openssl-src (vendors OpenSSL, perl Configure +
# make), libgit2-sys (vendors libgit2 c sources, cc-rs), curl-sys
# static-curl (vendors curl c sources, cc-rs). Everything lands in the
# cargo registry tarball at plan time, so the run stage builds with
# network=none.
#
# Toolchain install lives in cc-setup.sh.
#
# Reused for both the plan stage (cargo kept) and the run stage (cargo
# removed) via REMOVE_CARGO.
FROM docker.io/ubuntu/rust:1.85-24.04_edge

ARG ZIG_VERSION=0.13.0
ENV ZIG_VERSION=${ZIG_VERSION}

COPY cc-setup.sh /tmp/cc-setup.sh
RUN sh -ex /tmp/cc-setup.sh && rm /tmp/cc-setup.sh

ARG REMOVE_CARGO=1
RUN if [ "$REMOVE_CARGO" = "1" ]; then \
        rm -f /usr/local/cargo/bin/cargo /usr/lib/rust-1.85/bin/cargo; \
    fi
