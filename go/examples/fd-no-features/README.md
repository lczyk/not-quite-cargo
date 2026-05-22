# fd compilation demo (no default features)

this example mirrors [`../fd-jemalloc`](../fd-jemalloc) but here we
build fd with `cargo build --no-default-features`, dropping the
`use-jemalloc` default. that means no `jemalloc-sys`, no `./configure`
+ `make` step pulling in `grep` / `coreutils` / `sed` / `m4` / `mawk` --
and therefore the stripped `ubuntu/rust:1.85-24.04_edge` image is
enough as-is, no extra utilities to apt-download.

we don't even build a derived image. plan + run both run against
upstream `ubuntu/rust:1.85-24.04_edge` directly. the run stage just
deletes `cargo` inside the container before invoking `nqc` (see
`run.sh`).

planning uses
`cargo build --no-default-features -Z unstable-options --build-plan > build-plan.json`
inside the upstream image. then we run `not-quite-cargo patch
build-plan.json` on it (on the host) to patch out build-specific paths
/ variables and drop it into the run container -- same image as plan
but with cargo deleted on entry and the network bridge off -- so
nothing can sneak out to crates.io. and... it builds(!)...

ok, just because it does not fail does not mean it works. let's then
prove it works by dropping the compiled binary into *yet another* image
(this time `ubuntu:24.04`) and running `fd --version` + a basic
`fd <regex> /etc` search. version banner + a few `.conf` paths come
back -- the binary really is fd.

run with `make all`
