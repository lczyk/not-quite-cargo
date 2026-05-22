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
./bin/not-quite-cargo lower --target x86_64-unknown-linux-gnu ug.json > build_plan.json
./bin/not-quite-cargo patch build_plan.json
./bin/not-quite-cargo run build_plan.json
```

flags on `lower`:

- `--target <triple>` -- target triple the plan will run on; drives both
  the host-info side and `CARGO_CFG_*` env synthesis. defaults to
  runtime detection.
- `--project-root <path>` -- defaults to cwd
- `--cargo-home <path>` -- defaults to `$HOME/.cargo`
- `--rustc <name>` -- program string in the emitted plan; defaults to `rustc`
- `--skip-manifest-errors` -- fall back to pkg_id-only metadata when a
  Cargo.toml can't be loaded (git sources, missing registry caches)

the lowered plan is written to stdout; pipe or redirect.

[bp-removal]: https://github.com/rust-lang/cargo/pull/16212
[plan]: ../unit-graph-plan.md