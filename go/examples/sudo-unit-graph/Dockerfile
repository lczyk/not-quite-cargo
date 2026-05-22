# Demo base image: rust:1.84 + the C libs sudo-rs needs at link time.
#
# Same image shape as ../sudo/Dockerfile. Shipped separately so this
# example stays self-contained -- the unit-graph flow doesn't depend on
# the --build-plan demo's image being built.
FROM rust:1.84

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libpam0g-dev \
    && rm -rf /var/lib/apt/lists/*
