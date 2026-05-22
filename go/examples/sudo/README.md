# sudo-rs compilation demo

end-to-end demo of building [sudo-rs](https://github.com/trifectatechfoundation/sudo-rs)
without cargo on the build machine -- only on the planner machine. uses the
go version of not-quite-cargo.

## what it does

four steps: a one-time base-image build, two docker stages against
that image (`nqc-sudo-demo:1.75-24.04` =
`ubuntu/rust:1.75-24.04_stable` + `libpam0g-dev` + `pkg-config`, needed
b/c sudo-rs links against `libpam`), then a final prove step in a
fresh `ubuntu:24.04`:

1. **planner** -- has cargo + rustc. clones sudo-rs at a pinned tag, runs
   `cargo build -Z unstable-options --build-plan > build_plan.json` (with
   `RUSTC_BOOTSTRAP=1` to unlock `-Z` on stable cargo), then
   `not-quite-cargo patch build_plan.json`.
2. **runner** -- same image but cargo is deleted from `PATH` and the network
   is off (`--network=none`). consumes the patched plan via
   `not-quite-cargo run`.

if cargo's absence in stage 2 caused the build to fail, the demo would
exit with a non-zero status. instead the runner produces the sudo-rs
binaries from rustc alone.

3. **prove** -- spin up `ubuntu:24.04` (no rust toolchain at all,
   network off). copy the built `sudo` binary in, set the setuid bit,
   drop a minimal `/etc/sudoers` (root NOPASSWD) and a permissive
   `/etc/pam.d/sudo`, then run `sudo whoami`. it should print
   `root` -- proving the binary actually executes + elevates in a
   stock distro env.

## prerequisites

- docker (or set `DOCKER=podman`)
- go (the binary is cross-built for `linux/$arch` from the go/ source tree)
- git

## run

```
./demo.sh
```

artefacts land in `work/`:

- `work/not-quite-cargo` -- the cross-built binary that's mounted into the
  containers
- `work/sudo-rs/` -- the shallow clone
- `work/sudo-rs/build_plan.json` -- patched plan
- `work/sudo-rs/target/.../sudo` (etc.) -- the rust artefacts built without cargo
- `work/cargo-home/` -- registry / git caches populated by the planner stage,
  mounted read-only into the runner

## flags

- `SUDO_RS_REF=<tag-or-sha>` -- override the pinned sudo-rs revision
  (default: `v0.2.3`).
- `DOCKER=podman` -- use podman instead.
- `DEMO_SHELL=1` -- drop into a shell inside the runner container instead
  of running the build. useful for poking around.

## notes

- the work/ tree is gitignored. delete it to restart from scratch.
- the `target/` dir inside sudo-rs is created by `cargo build --build-plan`
  even though no compilation has happened yet. the runner stage fills it.
- network is disabled in the runner stage to demonstrate that the patched
  plan is genuinely self-contained -- no crates.io fetches happen.
- the demo image is built once and cached locally; delete with
  `docker image rm nqc-sudo-demo:1.75-24.04` to force a rebuild (e.g. after
  editing the Dockerfile).
