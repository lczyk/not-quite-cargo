# sudo-rs compilation demo via --unit-graph

mirrors [`../sudo`](../sudo) but uses cargo's `--unit-graph` instead of
`--build-plan`. proves the experimental `nqc build` path can derive a
working build plan for a real native-lib-using crate.

cargo 1.93 removed `--build-plan`. `--unit-graph` is the closest
surviving export. this demo pins `rust:1.84` (where both still exist)
but pretends `--build-plan` is gone so the result is what cargo 1.93+
users would have to do.

## what it does

three steps against `nqc-sudo-ug-demo:1.84` (= `rust:1.84` +
`libpam0g-dev` + `pkg-config` -- needed b/c sudo-rs links against
`libpam`):

1. **planner** -- clones sudo-rs at the pinned tag, runs
   `cargo build -Z unstable-options --unit-graph`, then
   `nqc build --os linux --arch <arch> --libc gnu unit-graph.json > build_plan.json`,
   then `nqc patch build_plan.json`.
2. **runner** -- cargo deleted from `PATH`, `--network=none`. consumes
   the patched plan via `nqc run`.

if cargo's absence in the runner stage caused the build to fail, the
demo would exit non-zero. instead the runner produces the sudo-rs
binaries -- entirely from rustc + the derived plan, no cargo.

## prerequisites

- docker (or set `DOCKER=podman`)
- go (the nqc binary is cross-built for `linux/$arch` from go/)
- git

## run

```
./demo.sh
```

artefacts land under `work/`:

- `work/not-quite-cargo` -- cross-built nqc, mounted into both stages
- `work/sudo-rs/` -- shallow clone of sudo-rs
- `work/sudo-rs/unit-graph.json` -- cargo's --unit-graph dump
- `work/sudo-rs/build_plan.json` -- nqc's derived + patched plan
- `work/sudo-rs/target/.../sudo` (etc.) -- the rust artefacts
- `work/cargo-home/` -- registry / git caches; mounted read-only into
  the runner

## flags

- `SUDO_RS_REF=<tag-or-sha>` -- override the pinned sudo-rs revision
  (default: `v0.2.3`).
- `DOCKER=podman` -- use podman instead.
- `DEMO_SHELL=1` -- drop into a shell inside the runner container
  instead of running the build.

## comparison with `../sudo`

both demos:

- pin `rust:1.84` so cargo's removed unstable flags still work
- derive a plan on a "planner" container, then replay on a "runner"
  container without cargo
- end in the same binary output

key difference: this one's planner step uses `--unit-graph` + `nqc
build`, the other's uses `--build-plan` directly. when this demo
breaks but `../sudo` still works, the regression is in `nqc build`,
not in the runner / patcher.
