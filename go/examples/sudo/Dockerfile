# Demo base image: rust:1.84 + the C libs sudo-rs needs at link time.
#
# libpam0g-dev provides libpam.so and the PAM headers. pkg-config is used by
# several rust crates' build.rs scripts to locate native libs.
FROM rust:1.84

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libpam0g-dev \
        pkg-config \
    && rm -rf /var/lib/apt/lists/*
