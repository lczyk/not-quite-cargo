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
- `cargo/unitgraph/` -- **experimental**: lower a cargo `--unit-graph` into a build plan
- `cargo/testdata/` -- fixtures for the patch golden test
- `internal/version/` -- generated version info (gitignored, regenerated via `make build`)
- `examples/sudo/` -- end-to-end demo: compile sudo-rs without cargo on the runner

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
cargo build -j1 -Z unstable-options --build-plan > build_plan.json
./bin/not-quite-cargo patch build_plan.json
# ship to runner
./bin/not-quite-cargo run build_plan.json
```

requires `-Z unstable-options` on nightly cargo, or `RUSTC_BOOTSTRAP=1` on
stable.

### experimental: `--unit-graph` (cargo 1.93+)

cargo 1.93.0 removed `--build-plan` ([rust-lang/cargo#16212][bp-removal]).
the closest surviving replacement is `--unit-graph`, but it only carries
the unit DAG + metadata; nqc lowers it back into a build-plan-shape file
via the `lower` subcommand. correctness is best-effort; see
[`unit-graph-plan.md`][plan] at the repo root for the design notes and
known limitations.

```
cargo build -Z unstable-options --unit-graph > ug.json
./bin/not-quite-cargo lower \
    --os linux --arch x86_64 --env gnu \
    --project-root /work --cargo-home /work/.cargo --rustc rustc \
    ug.json > build_plan.json
./bin/not-quite-cargo patch build_plan.json
./bin/not-quite-cargo run build_plan.json
```

every `lower` flag is required -- no defaults, no host-detection. the
tool is a pure transform: same inputs give same output regardless of
the machine it runs on.

- `--os <name>` -- target OS (linux, macos, windows, freebsd, ...).
- `--arch <name>` -- target arch (x86_64, aarch64, ...).
- `--env <name>` -- target libc env (gnu, musl, msvc). use the empty
  string for OSes that don't have a libc env distinction (e.g. macos).
- `--project-root <path>` -- spliced into output paths.
- `--cargo-home <path>` -- spliced into manifest dir paths; no file
  lookups happen against it.
- `--rustc <name>` -- program string in the emitted plan. defaults to
  `rustc` (every other flag is required).

manifest loads are best-effort: when a Cargo.toml can't be found
(captured plans often reference machines that don't have every dep's
source on disk), the lowerer warns + falls back to pkg_id-only
metadata for that unit.

the lowered plan is written to stdout; pipe or redirect.

[bp-removal]: https://github.com/rust-lang/cargo/pull/16212
[plan]: ../unit-graph-plan.md