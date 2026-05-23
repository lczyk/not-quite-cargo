#!/bin/sh
# Prove stage of the demo. Runs inside alpine:3.21 (no rust toolchain,
# network off, no extra apk packages). The fd binary is built with
# -Z build-std + panic_immediate_abort so libstd has no unwinder -- no
# libgcc_s dependency, fully self-contained against alpine's stock
# musl loader.
set -ex

BIN=$(find /fd-target/release/deps -name "fd-*" -type f -executable ! -name "*.d" | head -n1)
[ -n "$BIN" ] || { echo "no fd binary under /fd-target" >&2; exit 1; }

install -m 755 "$BIN" /usr/local/bin/fd

ldd /usr/local/bin/fd || true
fd --version
fd '\.conf$' /etc | head -n 5
