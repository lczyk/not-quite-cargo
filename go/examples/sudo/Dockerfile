# Runner image: ubuntu/rust:1.75-24.04_stable + libpam, with cargo
# deleted -- the runner side of the demo proves the build can finish
# without it. Plan stage uses the base image directly (which still has
# cargo).
#
# libpam0g-dev provides libpam.so + the PAM headers sudo-rs links
# against. dpkg-dev restores the helper scripts the stripped base image
# is missing.
FROM ubuntu/rust:1.75-24.04_stable

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        dpkg-dev \
        libpam0g-dev \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /usr/local/cargo/bin/cargo
