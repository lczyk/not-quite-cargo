#!/usr/bin/env bash
# Prove stage of the demo. Runs inside ubuntu:24.04 (no rust toolchain,
# network off). Drops the built coreutils multicall binary into a stock
# distro env and runs `coreutils --version` + a basic listing.
# Confirms the binary executes and behaves like real uutils coreutils.
set -ex

BIN=$(find /coreutils-target -name "coreutils-*" -type f -executable ! -name "*.d" | head -n1)
[ -n "$BIN" ] || { echo "no coreutils binary under /coreutils-target" >&2; exit 1; }

install -m 755 "$BIN" /usr/local/bin/coreutils

ldd /usr/local/bin/coreutils
# coreutils is a multicall binary: bare `coreutils --version` treats
# --version as the util name. Invoke via a subutil instead.
coreutils ls --version
coreutils ls /etc | head -n 5
