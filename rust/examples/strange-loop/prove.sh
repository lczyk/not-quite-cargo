#!/usr/bin/env bash
# Prove stage. Fresh demo-image container (network off, no cargo).
# Closes the strange loop: build nqc with the cross-compiled nqc,
# then replace the cross nqc with the freshly built one and rebuild
# nqc again with itself.
set -ex

cd /work
BIN=/work/target/release/not-quite-cargo

echo "[prove] round 1: cross nqc builds nqc from source"
not-quite-cargo run /build-plan.json > /dev/null 2>&1
[ -x "$BIN" ] || { echo "round 1: no nqc binary at $BIN" >&2; exit 1; }

# stash the freshly built nqc alongside the cross one
# (can't replace /usr/local/bin/not-quite-cargo -- it's bind-mounted)
mv "$BIN" /usr/local/bin/not-quite-cargo-02
rm -rf /work/target/*

echo "[prove] round 2: nqc built by nqc, builds nqc"
echo "PATH=$PATH"
ls -l /usr/local/bin/not-quite-cargo /usr/local/bin/not-quite-cargo-02
file /usr/local/bin/not-quite-cargo-02 || true
ldd /usr/local/bin/not-quite-cargo-02 || true
/usr/local/bin/not-quite-cargo-02 run /build-plan.json > /dev/null 2>&1
[ -x "$BIN" ] || { echo "round 2: no nqc binary at $BIN" >&2; exit 1; }

mv "$BIN" /usr/local/bin/not-quite-cargo-03
rm -rf /work/target/*

echo "[prove] round 3: turtles all the way down"
not-quite-cargo-03 run /build-plan.json > /dev/null 2>&1
[ -x "$BIN" ] || { echo "round 3: no nqc binary at $BIN" >&2; exit 1; }

mv "$BIN" /usr/local/bin/not-quite-cargo-04
rm -rf /work/target/*

echo "[prove] round 4: still going"
not-quite-cargo-04 run /build-plan.json > /dev/null 2>&1
[ -x "$BIN" ] || { echo "round 4: no nqc binary at $BIN" >&2; exit 1; }

mv "$BIN" /usr/local/bin/not-quite-cargo-05
rm -rf /work/target/*

echo "[prove] round 5: loop closed"
not-quite-cargo-05 run /build-plan.json > /dev/null 2>&1
[ -x "$BIN" ] || { echo "round 5: no nqc binary at $BIN" >&2; exit 1; }

echo "[prove] final --help:"
"$BIN" --help
