# uutils coreutils compilation demo

end-to-end build of [uutils/coreutils](https://github.com/uutils/coreutils) --
the rust reimplementation of gnu coreutils -- via `--build-plan`. mirrors
[`../fd-jemalloc`](../fd-jemalloc) (custom Dockerfile, plan + demo image
variants) rather than the leaner [`../fd-no-features`](../fd-no-features)
shape, because `uu_stdbuf` ships a `libstdbuf` build script that goes
through the `cpp_build` crate, which shells out to `c++` at build-script
time. so plan + run images both need a working c++ toolchain.

instead of apt-installing g++ (which the stripped base image doesn't
support cleanly -- no `_apt` user, dpkg postinst fails) we drop in
[zig](https://ziglang.org) and wrap `zig c++` / `zig cc` as
`/usr/local/bin/c++` and `/usr/local/bin/cc`. zig ships as a single
static tarball with no system deps, so the install is just untar +
two shim scripts. `cc_build` / `cpp_build` pick the wrappers up via
the default tool-detection path.

coreutils is a single crate with one multicall binary (`coreutils`) that
dispatches to each util (`ls`, `cat`, `cp`, ...). we build with
`--no-default-features --features unix` -- the upstream-recommended
linux feature set.

planning uses
`cargo build --no-default-features --features unix -Z unstable-options --build-plan > build-plan.json`
in the plan image. then `not-quite-cargo patch build-plan.json` on the
host strips build-specific paths, and the patched plan runs in the demo
image -- same base, but with cargo deleted and the network bridge off,
so nothing can sneak out to crates.io. and... it builds(!)...

ok, just because it does not fail does not mean it works. let's then
prove it works by dropping the compiled `coreutils` multicall binary
into *yet another* image (this time `ubuntu:24.04`) and running
`coreutils --version` + a basic `coreutils ls /etc` listing. version
banner + a few entries come back -- the binary really is coreutils.

run with `make all`
