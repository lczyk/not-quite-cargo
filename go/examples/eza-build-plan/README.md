# eza compilation demo

this example mirrors [`../fd-no-features`](../fd-no-features) but for
[eza](https://github.com/eza-community/eza) -- the modern ls
replacement. same shape: pure rust, `--no-default-features` to keep
the dep graph tight, no custom Dockerfile, runs against upstream
`ubuntu/rust:1.85-24.04_edge` directly, cargo deleted inside the run
container via `run.sh`.

planning uses
`cargo build --no-default-features -Z unstable-options --build-plan > build-plan.json`
in the upstream image. then we run `not-quite-cargo patch
build-plan.json` on it (on the host) to patch out build-specific
paths / variables and drop it into the run container -- without cargo
and with no network -- and... it builds(!)...

ok, just because it does not fail does not mean it works. let's then
prove it works by dropping the compiled binary into *yet another*
image (this time `ubuntu:24.04`) and running `eza --version` + a
basic `eza /etc` listing. version banner + a few entries come back --
the binary really is eza.

run with `make all`
