# rust version of not-quite-cargo

[![lint_and_test_rust](https://github.com/lczyk/not-quite-cargo/actions/workflows/lint_and_test_rust.yml/badge.svg)](https://github.com/lczyk/not-quite-cargo/actions/workflows/lint_and_test_rust.yml)

native `not-quite-cargo` (`nqc`) for rust users who'd rather not keep a go
toolchain around just to drive the plan executor.

it takes a cargo `--build-plan`, `patch`es the embedded host paths into
`{{PROJECT_ROOT}}` / `{{CARGO_HOME}}` placeholders so the plan can be shipped
elsewhere, and then `run`s the compile / build-script / link steps wherever you
drop it -- no cargo on the runner.

this port covers the stable `--build-plan` path only (`patch` + `run`). the
experimental `--unit-graph` `build` subcommand (for cargo 1.93+) lives on the
[go side](../go/readme.md) for now.

## layout

- `src/main.rs` -- cli entrypoint, uses [lexopt](https://github.com/blyxxyz/lexopt)
- `src/lib.rs` -- library crate (config, plan, patch, run, topo, deepreplace, directives, profile)
- `src/testdata/` -- fixtures for the patch golden test
- `build.rs` -- emits generated version info via the [`version`](https://github.com/lczyk/version) crate
- `examples/sudo-build-plan/` -- end-to-end demo: compile sudo-rs without cargo on the runner
- `examples/quine/` -- nqc compiles nqc, five rounds over (cargo-free self-build)

## build

```
make build         # ./bin/not-quite-cargo
make test-unit     # cargo test
make test-examples # runs each examples/*/makefile end-to-end (needs docker or podman)
make test          # test-unit + test-examples
make lint          # cargo clippy -D warnings + cargo fmt --check
make fmt           # cargo fmt
make cover         # llvm-cov, if installed
make clean
```

`make help` lists the rest.

## usage

```
cargo build -j1 -Z unstable-options --build-plan > build-plan.json
./bin/not-quite-cargo patch \
    --project-root="$PWD" --cargo-home="$CARGO_HOME" --inplace \
    build-plan.json
# ship to runner
./bin/not-quite-cargo run build-plan.json
```

`patch` rewrites the build/cargo-home paths into `{{PROJECT_ROOT}}` /
`{{CARGO_HOME}}` placeholders, so the runner can mount different concrete
paths. `--inplace` writes the patched plan back over the input; drop it to get
the patched plan on stdout instead.

requires `-Z unstable-options` on nightly cargo, or `RUSTC_BOOTSTRAP=1` on
stable.

extra flags on `patch`:

- `--profile <name>` -- rewrite the plan for a target profile: `release` or
  `debug` (flips the output dir + opt-level / debuginfo flags).
- `--debuginfo <level>` -- override `-C debuginfo=N` on every rustc invocation
  (e.g. `0` to strip debug info).

flags on `run`:

- `-j, --jobs <N>` -- worker count. default `1` (serial). `0` = max available
  cores. positive `N` = up to `N` (capped at max). negative `N` = max + N
  (floored at 1, so `-j=-1` = max-1).
