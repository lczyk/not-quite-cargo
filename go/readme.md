# go version of not-quite-cargo

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/lczyk/not-quite-cargo?filename=go%2Fgo.mod)
![GitHub Tag](https://img.shields.io/github/v/tag/lczyk/not-quite-cargo?label=release)
[![lint_and_test](https://github.com/lczyk/not-quite-cargo/actions/workflows/lint_and_test.yml/badge.svg)](https://github.com/lczyk/not-quite-cargo/actions/workflows/lint_and_test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/lczyk/not-quite-cargo/go.svg)](https://pkg.go.dev/github.com/lczyk/not-quite-cargo/go)
[![Go Report Card](https://goreportcard.com/badge/github.com/lczyk/not-quite-cargo/go)](https://goreportcard.com/report/github.com/lczyk/not-quite-cargo/go)

Not-quite-car**go**...

~~go port of the python tool~~

default implementation of `not-quite-cargo` (`nqc`)

it takes a cargo `--build-plan` (or, for cargo 1.93+, a `--unit-graph`), `patch`es the embedded host paths into `{{PROJECT_ROOT}}` / `{{CARGO_HOME}}` placeholders so the plan can be shipped elsewhere, and then `run`s the compile / build-script / link steps wherever you drop it -- no cargo on the runner. for cargo 1.93+ the experimental `build` subcommand derives a build-plan-shape file from `--unit-graph` (since `--build-plan` was removed).

## layout

- `cmd/not-quite-cargo/` -- cli entrypoint, uses [go-flags](https://github.com/jessevdk/go-flags)
- `cargo/` -- library package (config, plan, patch, run, topo, deepreplace, directives)
- `cargo/unitgraph/` -- **experimental**: derive a build plan from a cargo `--unit-graph`
- `cargo/testdata/` -- fixtures for the patch golden test
- `internal/version/` -- generated version info (gitignored, regenerated via `make build`)
- `examples/sudo-build-plan/` -- end-to-end demo: compile sudo-rs without cargo on the runner (via cargo `--build-plan`)
- `examples/sudo-unit-graph/` -- same end-to-end demo, but via cargo `--unit-graph` + experimental `nqc build`
- `examples/fd-jemalloc/` -- compile fd with default features (pulls jemalloc-sys + autoconf tooling)
- `examples/fd-no-features/` -- compile fd with `--no-default-features` (minimal, no native build deps)
- `examples/fd-musl/` -- compile fd statically against musl (alpine target, no glibc loader on the runner)
- `examples/eza-build-plan/` -- compile eza via `--build-plan` (default features)

## build

```
make build         # ./bin/not-quite-cargo
make test-unit     # go test -race ./...
make test-examples # runs each examples/*/makefile end-to-end (needs docker or podman)
make test          # test-unit + test-examples
make lint          # go vet + gofmt -l check
make fmt           # gofmt -s -w
make clean
```

`make help` lists the rest.

## usage

### stable: cargo `--build-plan` (cargo 1.28 -- 1.92)

```
cargo build -j1 -Z unstable-options --build-plan > build-plan.json
./bin/not-quite-cargo patch \
    --project-root="$PWD" --cargo-home="$CARGO_HOME" --inplace \
    build-plan.json
# ship to runner
./bin/not-quite-cargo run build-plan.json
```

`patch` rewrites the build/cargo-home paths into `{{PROJECT_ROOT}}` /
`{{CARGO_HOME}}` placeholders, so the runner can mount different
concrete paths. `--inplace` writes the patched plan back over the input;
drop it to get the patched plan on stdout instead.

requires `-Z unstable-options` on nightly cargo, or `RUSTC_BOOTSTRAP=1` on
stable.

### experimental: `--unit-graph` (cargo 1.93+)

cargo 1.93.0 removed `--build-plan` ([rust-lang/cargo#16212][bp-removal]).
the closest surviving replacement is `--unit-graph`, but it only carries
the unit DAG + metadata; nqc derives a build-plan-shape file from it
via the `build` subcommand. correctness is best-effort.

```
cargo build -Z unstable-options --unit-graph > unit-graph.json
./bin/not-quite-cargo build --os linux --arch aarch64 unit-graph.json > build-plan.json
./bin/not-quite-cargo patch \
    --project-root="$PWD" --cargo-home="$CARGO_HOME" --inplace \
    build-plan.json
./bin/not-quite-cargo run build-plan.json
```

flags on `build`:

- `--os <name>` -- target OS: `linux` or `macos`. (v0 doesn't support
  anything else.) required.
- `--arch <name>` -- target arch: `aarch64` (verified path -- fd fixture
  runs on it) or `x86_64`. required. accepts `arm64` / `amd64` aliases.
- `--libc <name>` -- `gnu` (default) / `musl` on linux, or `none` on
  macos.
- `--vendor <name>` -- override the vendor token in the target triple
  (e.g. `alpine` for `aarch64-alpine-linux-musl`). default: `unknown`
  (linux) / `apple` (macos).
- `--rustc <name>` -- program string in the emitted plan; defaults to
  `rustc`.

manifest loads are best-effort: when a Cargo.toml can't be found
(captured plans often reference machines that don't have every dep's
source on disk), the build step warns + falls back to pkg_id-only
metadata for that unit.

the derived plan is written to stdout; pipe or redirect.

[bp-removal]: https://github.com/rust-lang/cargo/pull/16212