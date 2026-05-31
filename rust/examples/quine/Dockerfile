# ubuntu/rust:1.85-24.04_edge with libc-bin (for ldd), mold (linker),
# and cargo deleted.

FROM docker.io/ubuntu/rust:1.85-24.04_edge

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends libc-bin mold; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*; \
    update-alternatives --install /usr/bin/ld ld /usr/bin/mold 100; \
    update-alternatives --set ld /usr/bin/mold

RUN rm -f /usr/local/cargo/bin/cargo
