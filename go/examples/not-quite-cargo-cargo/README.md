# rust-lang/cargo compilation demo (cargo builds cargo)

end-to-end build of [rust-lang/cargo](https://github.com/rust-lang/cargo)
itself, via `--build-plan`. the joke writes itself: not-quite-cargo
builds quite-cargo. mirrors [`../coreutils-build-plan`](../coreutils-build-plan)
shape (custom Dockerfile, zig wrapped as c / c++) but cranks the
native-dep dial much higher.

cargo pulls a stack of `*-sys` crates -- `openssl-sys`, `libgit2-sys`,
`libssh2-sys`, `curl-sys`, `libsqlite3-sys`, `libnghttp2-sys`,
`libz-sys`, `libsrc/openssl-src`. by default these expect system
`libssl-dev` / `libgit2-dev` / etc on the runner. since the run stage
is `network=none` and we don't want to apt-extract the dev headers,
we build with `--no-default-features --features all-static`:

- `vendored-openssl` -> pulls `openssl-src` (ships the OpenSSL tarball
  inside the crate registry download, perl `./Configure` + make
  inside the build script)
- `vendored-libgit2` -> libgit2 c sources bundled in libgit2-sys,
  cc-rs compiles them
- `curl/static-curl` -> curl c sources bundled in curl-sys

so everything c-side arrives via the cargo registry tarball at plan
time and lands in `/cargo-home`. run stage just c-compiles, no
network needed.

the demo image therefore needs:
- a c / c++ compiler (zig 0.13 wrapped at `/usr/bin/{c++,cc,g++,gcc}`)
- perl (openssl-src's Configure is perl)
- make (openssl-src + others)
- git (cargo's own root build.rs runs `git rev-parse` for the version
  banner; falls back gracefully but cleaner with git present)

all side-installed via `apt-get install --download-only` + `dpkg-deb -x`
(stripped base image has no `_apt` user, so normal apt postinst
fails). see `cc-setup.sh`.

planning uses
`cargo build --no-default-features --features all-static -Z unstable-options --build-plan > build-plan.json`
in the plan image. then `not-quite-cargo patch build-plan.json` on
the host strips build-specific paths, and the patched plan runs in
the demo image -- same base, but cargo deleted and the network bridge
off. and... it builds(!)... (hopefully)

prove stage closes the loop quine-style (cf. nqc-2's
`rust/examples/quine/`, but only one round here -- cargo-builds-cargo
is slow enough already). drops the built cargo at
`/usr/local/bin/cargo` in the demo image (rust + zig + perl + make,
cargo deleted), mounts the cargo source + `cargo-home` registry cache,
wipes `target/`, and runs:

```
cargo build --offline -j1 --no-default-features --features all-static
```

`network=none` + `--offline` keeps it honest: the round-2 build
reaches only the mounted registry cache. binary lands at
`/work/target/debug/cargo`; we then run that one's `--version` to
confirm. the loop closes -- not-quite-cargo built cargo built cargo.

run with `make all`. expect a long run -- ~400 rust crates +
heavy c compile chains (openssl, libgit2, curl). easily the slowest
demo in the suite.
