# go version of not-quite-cargo

Not-quite-car**go**...

This is an AI rewrite of the python version which was then audited for bits
quiteliterally lost in translation. I would not trust this version as much as
the other one. Over time though, as i vet it more thougoughly, this will become
the default verion due to the ease cross compilation + binary size + it being 
only one binary.

## layout

- `cmd/not-quite-cargo/` -- cli entrypoint, uses [go-flags](https://github.com/jessevdk/go-flags)
- `cargo/` -- library package (config, plan, patch, run, topo, deepreplace, directives)
- `cargo/unitgraph/` -- **experimental**: derive a build plan from a cargo `--unit-graph`
- `cargo/testdata/` -- fixtures for the patch golden test
- `internal/version/` -- generated version info (gitignored, regenerated via `make build`)
- `examples/sudo-build-plan/` -- end-to-end demo: compile sudo-rs without cargo on the runner (via cargo `--build-plan`)
- `examples/sudo-unit-graph/` -- same end-to-end demo, but via cargo `--unit-graph` + experimental `nqc build`

## build

```
make build      # ./bin/not-quite-cargo
make test       # go test -race ./...
make lint       # go vet + gofmt -l check
make fmt        # gofmt -s -w
make clean
```

`make help` lists the rest.

## usage

### stable: cargo `--build-plan` (cargo 1.28 -- 1.92)

```
cargo build -j1 -Z unstable-options --build-plan > build-plan.json
./bin/not-quite-cargo patch build-plan.json
# ship to runner
./bin/not-quite-cargo run build-plan.json
```

requires `-Z unstable-options` on nightly cargo, or `RUSTC_BOOTSTRAP=1` on
stable.

### experimental: `--unit-graph` (cargo 1.93+)

cargo 1.93.0 removed `--build-plan` ([rust-lang/cargo#16212][bp-removal]).
the closest surviving replacement is `--unit-graph`, but it only carries
the unit DAG + metadata; nqc derives a build-plan-shape file from it
via the `build` subcommand. correctness is best-effort; see
[`unit-graph-plan.md`][plan] at the repo root for the design notes and
known limitations.

```
cargo build -Z unstable-options --unit-graph > unit-graph.json
./bin/not-quite-cargo build --os linux --arch aarch64 unit-graph.json > build-plan.json
./bin/not-quite-cargo patch build-plan.json
./bin/not-quite-cargo run build-plan.json
```

flags on `build`:

- `--os <name>` -- target OS: `linux` or `macos`. (v0 doesn't support
  anything else.)
- `--arch <name>` -- target arch: `aarch64` (verified path -- fd fixture
  runs on it) or `x86_64` (allowed optimistically, not yet validated
  against a fixture).
- `--libc <name>` -- `gnu` (default) / `musl` on linux, or `none` on
  macos.
- `--rustc <name>` -- program string in the emitted plan; defaults to
  `rustc`.

`--project-root` and `--cargo-home` are auto-derived from the input
unit-graph (path+ pkg_id for the workspace, registry+ source paths
for the cargo home). v0 scope -- linux + macos on aarch64 only. defaults to
  `rustc` (every other flag is required).

manifest loads are best-effort: when a Cargo.toml can't be found
(captured plans often reference machines that don't have every dep's
source on disk), the build step warns + falls back to pkg_id-only
metadata for that unit.

the derived plan is written to stdout; pipe or redirect.

[bp-removal]: https://github.com/rust-lang/cargo/pull/16212
[plan]: ../unit-graph-plan.md