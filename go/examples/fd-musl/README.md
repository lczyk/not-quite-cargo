# fd compilation demo (musl, via --unit-graph)

this example mirrors [`../fd-no-features`](../fd-no-features) but
targets musl libc instead of glibc, via the experimental `nqc build
--libc=musl` path (and therefore the `--unit-graph` flow, not
`--build-plan`).

it's the most involved demo here, because "musl, fully static, runs
anywhere" turned out to need a pile of pieces clicking into place.
each piece below took a swing or two to land -- the README is partly
a writeup of what worked and what didn't.

## the alpine triple

alpine's rust toolchain installs rust-std under
`/usr/lib/rustlib/aarch64-alpine-linux-musl/` -- note the `alpine`
vendor token, not the `unknown` that rust-lang's official builds use.
rustc looks for the corresponding sysroot dir and doesn't find it.
so when we tell `cargo build --target=aarch64-unknown-linux-musl`.
because of this nqc gained a `--vendor=alpine` flag.

## the static-link saga

first attempt: `cargo build --release --no-default-features`. binary
links fine, but `ldd` shows `libgcc_s.so.1` dynamic dep, which alpine
doesn't ship in base (you have to `apk add libgcc`). what? libgcc? but
what about musl!? well, turns out alpine's `libstd-rust` is built with
`panic=unwind` and dynamic-links the unwinder from libgcc_s, so even
our `panic=abort` user code pulls libgcc_s through libstd...

second attempt: rebuild libstd via `-Z build-std=std,panic_abort -Z
build-std-features=panic_immediate_abort`. this drops the unwinder
from libstd entirely. works but `nqc` needed some more patches:
unit-graph for build-std runs includes std/core/alloc as units with
their own pkg_ids under `path+file:///usr/lib/rustlib/...`, and `nqc`s
`deriveProjectRoot` was including those paths in its longest-common-
prefix computation, collapsing project root to `/`. on top of that,
alpine's rustc 1.83 can't actually rebuild its own bundled core
source out of the box **or**, quite possibly, i'm just bad at doing it.
either way, the frustration is begining to mount at this point

third attempt: force `-C target-feature=+crt-static` on every target unit.
apline has libgcc.a so we can include all the `_Unwind_` targets from alpine's
unfortunate `rust-std`. we are, therefore statically linking with musl *and*
just a smideon of libgcc -- for the extra flavour notes. this one
worked and i've decided not to ask more questions and just be happy.

run, somehow, with `make all`
