#!/usr/bin/env bash
# Prove stage of the demo. Runs inside ubuntu:24.04 (no rust toolchain,
# network off). Drops the built eza binary into a stock distro env and
# runs `eza --version` + a basic listing. Confirms the binary executes
# and behaves like a real eza.
set -ex

BIN=$(find /eza-target -name "eza-*" -type f -executable ! -name "*.d" | head -n1)
[ -n "$BIN" ] || { echo "no eza binary under /eza-target" >&2; exit 1; }

install -m 755 "$BIN" /usr/local/bin/eza

ldd /usr/local/bin/eza
eza --version
eza /etc | head -n 5
