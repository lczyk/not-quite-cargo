#!/usr/bin/env bash
# Prove stage of the demo. Runs inside ubuntu:24.04 (no rust toolchain,
# network off). Drops the built fd binary into a stock distro env and
# runs `fd --version` + a basic search. Confirms the binary executes
# and behaves like a real fd.
set -ex

BIN=$(find /fd-target -name "fd-*" -type f -executable ! -name "*.d" | head -n1)
[ -n "$BIN" ] || { echo "no fd binary under /fd-target" >&2; exit 1; }

install -m 755 "$BIN" /usr/local/bin/fd

ldd /usr/local/bin/fd
fd --version
fd '\.conf$' /etc | head -n 5
