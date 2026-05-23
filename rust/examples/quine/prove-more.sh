#!/usr/bin/env bash
# Prove-more stage. Same loop as prove.sh but runs N rounds
# (default 1000). Override with N=<count> make prove-more.
set -e

N=${N:-1000}
JOBS=${JOBS:-0}   # 0 = max available
cd /work
BIN=/work/target/debug/not-quite-cargo
COMPILER=not-quite-cargo

echo "[prove-more] running $N rounds"
W=${#N}

total_start=$(date +%s%3N)
for i in $(seq 1 "$N"); do
    round_start=$(date +%s%3N)
    "$COMPILER" run -j "$JOBS" /build-plan.json > /dev/null 2>&1
    elapsed=$(($(date +%s%3N) - round_start))
    [ -x "$BIN" ] || { echo "round $i: no nqc binary at $BIN" >&2; exit 1; }
    cp -L "$BIN" /usr/local/bin/nqc-current
    rm -rf /work/target/*
    COMPILER=/usr/local/bin/nqc-current
    printf '[prove-more] %0*d / %0*d (%dms)\n' "$W" "$i" "$W" "$N" "$elapsed"
done
total=$(($(date +%s%3N) - total_start))
printf '[prove-more] %d rounds total: %dms (avg %dms/round)\n' "$N" "$total" "$((total / N))"

echo "[prove-more] $N rounds done. final --help:"
"$COMPILER" --help
