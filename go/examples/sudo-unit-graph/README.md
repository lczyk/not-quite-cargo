# sudo-rs compilation demo via --unit-graph

mirrors [`../sudo`](../sudo) but uses cargo's `--unit-graph` instead of
`--build-plan`. proves the experimental `nqc build` path can derive a
working build plan for a real native-lib-using crate.

cargo 1.93 removed `--build-plan`. `--unit-graph` is the closest
surviving export. this demo pins `ubuntu/rust:1.93-26.04_edge` -- a
cargo where `--build-plan` is genuinely gone -- so the planner has
no choice but to use `--unit-graph`.

## what it does

three container stages, two images:

1. **planner** (`ubuntu/rust:1.93-26.04_edge` -- has cargo + rustc) --
   clones sudo-rs at the pinned tag, runs
   `cargo build -Z unstable-options --unit-graph`, then
   `nqc build --os linux --arch <arch> --libc gnu unit-graph.json > build-plan.json`,
   then `nqc patch build-plan.json`.
2. **runner** (`nqc-sudo-ug-demo:1.93-26.04` = base + `libpam0g-dev`,
   cargo deleted at image-build time, `--network=none`) -- consumes the
   patched plan via `nqc run`.
3. **prove** (`ubuntu:26.04`, no rust toolchain, network off) -- copy
   the built `sudo` binary in, set the setuid bit, drop a minimal
   `/etc/sudoers` (root NOPASSWD) and a permissive `/etc/pam.d/sudo`,
   run `sudo whoami`. should print `root` -- proves the binary actually
   executes + elevates in a stock distro.

if cargo's absence in the runner stage caused the build to fail, the
demo would exit non-zero. instead the runner produces the sudo-rs
binaries -- entirely from rustc + the derived plan, no cargo.

## prerequisites

- docker (or set `DOCKER=podman`)
- go (the nqc binary is cross-built for `linux/$arch` from go/)
- git

## run

```
make all        # full pipeline (image -> binary -> clone -> plan -> run -> prove)
make help       # list every target
```

individual stages: `make image`, `make binary`, `make clone`, `make plan`,
`make run`, `make prove`. each depends on its predecessors, so
`make prove` from a clean tree runs the lot.

artefacts land under `work/`:

- `work/not-quite-cargo` -- cross-built nqc, mounted into both stages
- `work/sudo-rs/` -- shallow clone of sudo-rs
- `work/sudo-rs/unit-graph.json` -- cargo's --unit-graph dump
- `work/sudo-rs/build-plan.json` -- nqc's derived + patched plan
- `work/sudo-rs/target/.../sudo` (etc.) -- the rust artefacts
- `work/cargo-home/` -- registry / git caches; mounted read-only into
  the runner

## flags

- `SUDO_RS_REF=<tag-or-sha> make all` -- override the pinned sudo-rs
  revision (default: `v0.2.3`).
- `DOCKER=podman make all` -- use podman instead.
- `make shell` -- drop into bash in the runner image for poking around.

## comparison with `../sudo`

both demos:

- derive a plan on a "planner" container, then replay on a "runner"
  container without cargo
- end in the same binary output

key differences:

- `../sudo` pins `ubuntu/rust:1.75-24.04_stable` and uses
  `cargo --build-plan` directly.
- this one pins `ubuntu/rust:1.93-26.04_edge` (where `--build-plan` is
  gone) and uses `cargo --unit-graph` + `nqc build` instead.

when this demo breaks but `../sudo` still works, the regression is in
`nqc build`, not in the runner / patcher.
