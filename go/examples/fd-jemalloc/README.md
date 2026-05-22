# fd compilation demo

end-to-end demo of building [fd](https://github.com/sharkdp/fd) without
cargo in the build host, only on the machine on which we plan the build

fd is pure rust but its default features pull `jemalloc-sys`, whose
`build.rs` shells out to `./configure` + `make` and needs `grep`,
`coreutils`, `sed`, `m4`, `mawk`, `make` -- none of which ship in the
stripped `ubuntu/rust:1.85-24.04_edge` image. we could supply those
outside the container but, for convenience, here we pre-build the
build image, drop the tools (and `libsigsegv2` for mawk) into the right
place on the filesystem and, just to not have any aces in one's sleeve,
remove the `cargo` binary. fd needs rustc >= 1.77.2 so we go up to the
1.85 image rather than the 1.75 one [`../sudo-build-plan`](../sudo-build-plan)
uses.

planning then happens in the same image with cargo kept (a sibling
`nqc-fd-jemalloc-plan:1.85-24.04` tag, built from the same Dockerfile via
`--build-arg REMOVE_CARGO=0`). it uses that image's
`cargo build -Z unstable-options --build-plan > build-plan.json`.
then we run `not-quite-cargo patch build-plan.json` on it (on the host)
to patch out build-specific paths / variables and drop it into the
cargo-less build image -- without cargo but with `nqc` -- which we also
deprived of a network bridge just to make sure nothing touches network
and... it builds(!)...

ok, just because it does not fail does not mean it works. let's then
prove it works by dropping the compiled binary into *yet another* image
(this time `ubuntu:24.04`) and running `fd --version` + a basic
`fd <regex> /etc` search. version banner + a few `.conf` paths come
back -- the binary really is fd.

run with `make all`
