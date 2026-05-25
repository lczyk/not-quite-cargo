#!/bin/sh
# Prove stage. Closes the loop: the cargo we just built (with nqc, no
# cargo, no network) is dropped at /usr/local/bin/cargo and used to
# rebuild cargo from the same source tree. Runs in the demo image
# (rust + zig + perl + make, cargo deleted) with network=none.
set -e

cd /work

echo "[prove] cargo --version:"
cargo --version
echo "[prove] cargo --list (first 10):"
cargo --list | head -n 10

echo "[prove] rebuild cargo with cargo (--offline, --features all-static):"
# /cargo-home is mounted ro from plan stage so every dep is already in
# the registry cache. --offline forces cargo to never reach for the
# network (sanity check; network=none kills it anyway).
cargo build --offline -j1 --no-default-features --features all-static

[ -x /work/target/debug/cargo ] || {
    echo "[prove] no /work/target/debug/cargo after round-2 build" >&2
    exit 1
}

echo "[prove] round-2 cargo --version:"
/work/target/debug/cargo --version

echo "[prove] loop closed -- cargo built cargo."
