#!/bin/sh

if [ -z "$TARGET" ]; then
    echo "TARGET env var must be set (e.g. aarch64-alpine-linux-musl)" >&2
    exit 1
fi

set -e

cd /work

# NOTE: the lto override is needed because fd's release profile turns lto and breaks things
cargo \
    --config profile.release.lto=false build \
    --release -j1 \
    --no-default-features \
    --target="$TARGET" \
    -Z unstable-options --unit-graph > unit-graph.json
