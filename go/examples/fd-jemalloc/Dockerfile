# ubuntu/rust:1.85-24.04_edge + shell utilities needed by
# jemalloc-sys' configure script (grep, sed, m4, gawk, coreutils -- the
# stripped base image is missing them).
#
# Reused for both the plan stage (cargo kept) and the run stage (cargo
# removed) via REMOVE_CARGO. fd v10.2.0 needs rustc >= 1.77.2.
FROM docker.io/ubuntu/rust:1.85-24.04_edge

RUN set -eux; \
    apt-get update; \
    mkdir -p /tmp/debs; cd /tmp/debs; \
    apt-get download grep coreutils sed m4 mawk libsigsegv2 make; \
    for f in *.deb; do dpkg-deb -x "$f" /; done; \
    ln -sf /usr/bin/mawk /usr/bin/awk; \
    cd /; rm -rf /tmp/debs /var/lib/apt/lists/*

ARG REMOVE_CARGO=1
RUN if [ "$REMOVE_CARGO" = "1" ]; then \
        rm -f /usr/local/cargo/bin/cargo /usr/lib/rust-1.85/bin/cargo; \
    fi
