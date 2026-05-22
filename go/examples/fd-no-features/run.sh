#!/bin/sh
# Run stage scriptlet. Runs inside upstream ubuntu/rust:1.85-24.04_edge
# (no derived demo image -- this variant doesn't need any extra
# utilities). Deletes the cargo binary so the rest of the build has to
# go through `not-quite-cargo run`, then runs it.
set -e

rm -f /usr/local/cargo/bin/cargo /usr/lib/rust-1.85/bin/cargo
command -v cargo >/dev/null 2>&1 && {
    echo "cargo still on PATH; demo invalid" >&2
    exit 1
}

cd /work
not-quite-cargo run build-plan.json
