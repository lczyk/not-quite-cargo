# ubuntu/rust:1.75-24.04_stable + libpam, with cargo deleted

FROM docker.io/ubuntu/rust:1.75-24.04_stable

# download .debs for libpam + transitive runtime deps, then cherry-pick
# only the .so

# sudo-rs uses raw FFI bindings so we can skip headers
RUN set -eux; \
    triplet="$(uname -m)-linux-gnu"; \
    apt-get update; \
    mkdir -p /tmp/debs; cd /tmp/debs; \
    apt-get download libpam0g libpam0g-dev libaudit1 libcap-ng0; \
    for deb in libpam0g_*.deb; do \
        dpkg-deb --fsys-tarfile "$deb" | tar -xf - -C / \
            "./usr/lib/${triplet}/libpam.so.0" \
            "./usr/lib/${triplet}/libpam.so.0.85.1"; \
    done; \
    for deb in libpam0g-dev_*.deb; do \
        dpkg-deb --fsys-tarfile "$deb" | tar -xf - -C / \
            "./usr/lib/${triplet}/libpam.so"; \
    done; \
    for deb in libaudit1_*.deb; do \
        dpkg-deb --fsys-tarfile "$deb" | tar -xf - -C / \
            "./usr/lib/${triplet}/libaudit.so.1" \
            "./usr/lib/${triplet}/libaudit.so.1.0.0"; \
    done; \
    for deb in libcap-ng0_*.deb; do \
        dpkg-deb --fsys-tarfile "$deb" | tar -xf - -C / \
            "./usr/lib/${triplet}/libcap-ng.so.0" \
            "./usr/lib/${triplet}/libcap-ng.so.0.0.0"; \
    done; \
    cd /; rm -rf /tmp/debs /var/lib/apt/lists/*

# remove cargo
RUN rm -f /usr/local/cargo/bin/cargo
