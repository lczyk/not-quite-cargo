# Demo base image: ubuntu/rust:1.75-24.04_stable + libpam dev headers.
#
# libpam0g-dev provides libpam.so + the PAM headers sudo-rs links against.
FROM ubuntu/rust:1.75-24.04_stable

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libpam0g-dev \
    && rm -rf /var/lib/apt/lists/*
