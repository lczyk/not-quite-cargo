# ubuntu/rust:1.85-24.04_edge with libc-bin (for ldd) and cargo deleted.

FROM docker.io/ubuntu/rust:1.85-24.04_edge

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends libc-bin; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*

RUN rm -f /usr/local/cargo/bin/cargo
