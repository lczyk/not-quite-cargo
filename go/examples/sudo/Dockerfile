# Demo base image: ubuntu/rust:1.75-24.04_stable + the C libs sudo-rs
# needs at link time.
#
# libpam0g-dev provides libpam.so and the PAM headers. pkg-config is used
# by several rust crates' build.rs scripts to locate native libs.
FROM ubuntu/rust:1.75-24.04_stable

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libpam0g-dev \
        pkg-config \
    && rm -rf /var/lib/apt/lists/*
