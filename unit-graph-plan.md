# unit-graph experimental support -- design notes

worked out via `/grill-me`. captures every decision so we can implement
without re-litigating each branch.

## status

shipped on the `unit-graph` branch. all seven planned commits + nine
follow-up validation-driven fixes have landed.

end-to-end validated on real crates (all locally, build + run, binary
runs and behaves):

- hello world (1 unit, no deps)
- once_cell user (2 units, default features)
- bs-test (3 units, build.rs emitting rustc-cfg directive)
- pm-test (16 units, full serde derive chain incl. proc-macro2 +
  quote + syn + serde_derive + serde + build scripts)
- **fd v10.2.0 (77 units, full real-world crate with clap proc
  macros, multiple build scripts, large transitive DAG) -- the
  flagship target. `fd --version` reports `fd 10.2.0`.**

real fd unit-graph fixture committed at
`go/cargo/unitgraph/testdata/fd/ug.json` (path-anonymised, ~96K). the
ground-truth `build-plan.json` from cargo is not committed yet b/c
cargo 1.93+ removed `--build-plan`; refresh requires docker + the
existing `capture.sh` against `rust:1.84`.

sudo-rs (the plan's stated MVP target) compiles on linux only; on macOS
both cargo + our build path fail identically on missing
linux-specific libc symbols. demo's docker flow remains the route
there.

bugs found + fixed during validation (each its own commit):

1. `-C key=value` flags joined into single argv element -- rustc
   rejected the embedded space. now `(-C, key=value)` pair.
2. `-C incremental` emitted bare -- rustc requires a path. dropped
   then later (this iteration) re-added with a proper per-unit
   `target/<profile>/incremental/<crate>-<hash>` directory.
3. enabled features not forwarded as `--cfg feature="<name>"`.
4. build-script binary path off because target name has hyphens but
   rustc canonicalises to underscores.
5. depIsCompileBuildScript heuristic broke after #4 -- replaced
   path-based detection with an explicit unit-graph-derived flag.
6. `--cap-lints warn` missing for non-primary packages -- their
   `#![deny(...)]` settings poisoned the local build.
7. `OUT_DIR` not propagated from build-script-run to package compile
   -- crates including `include!(env!("OUT_DIR"))` blew up.
8. proc-macro dylib extension wrong on darwin -- Platform field was
   driving both directory layout and file extension; split into
   `Platform` and `ExtPlatform`.
9. workspace inheritance (`field.workspace = true`) -- manifest
   loader now walks up for the workspace root and resolves
   inheritable fields.
10. build-script `CARGO_FEATURE_*` env taken from the package's
    compile unit features, not the (empty) run-custom-build unit.
11. `--extern proc_macro` missing for proc-macro crates -- rustc's
    `--crate-type proc-macro` doesn't auto-inject the extern.

if any of these surface again, the commit message of the matching
fix is the place to start.

## why

cargo 1.93.0 removed `--build-plan` ([rust-lang/cargo#16212][pr]). the only
surviving plan-export on cargo is `-Z unstable-options --unit-graph`, which
gives the unit DAG + per-unit metadata but **not** rustc args, env vars or
hashed output paths. to keep not-quite-cargo working on current cargo we
need to derive build-plan-shape invocations from unit-graph ourselves.

target -- and ceiling for every decision below -- is compiling sudo-rs end
to end. proc macros, build scripts, native libs (libpam), full profile +
edition coverage.

[pr]: https://github.com/rust-lang/cargo/pull/16212

## planner contract

exactly two subprocess kinds allowed on the planner:

1. `cargo build -Z unstable-options --unit-graph > ug.json` (once per
   workspace).
2. `rustc --print cfg > host-cfg.txt` (once per distinct platform in the
   unit-graph). run manually by the user; build does not shell out.

no compilation, no other cargo invocations.

## flow

```
# planner
cargo build -Z unstable-options --unit-graph > ug.json
rustc --print cfg > host-cfg.txt
nqc build --cfg host-cfg.txt ug.json build-plan.json
nqc patch build-plan.json

# ship build-plan.json + sources + cargo-home to runner

# runner
nqc run build-plan.json
```

five planner-side steps, one runner step. patch + run are unchanged from
today's --build-plan flow.

## design decisions

### generation strategy: reimplement cargo's command-building

read profile / target / features / edition / deps from unit-graph and
build the rustc command from scratch. no scraping `cargo build -vv` or
shelling out to cargo for command-extraction -- planner contract forbids
it.

### plan format: build at plan time

planner emits build-plan-shape JSON (same schema today's runner already
walks). means:

- runner code unchanged
- patch step unchanged
- all new logic lives behind a single boundary: `Build(ug, opts) -> cargo.BuildOutput`
- intermediate `build-plan.json` is human-inspectable, diffable against
  a cargo-generated `--build-plan` when debugging

### hashing: self-consistent, not cargo-matching

compute a stable 16-hex digest per unit from
`(pkg_id, target_name, mode, profile_name, features sorted, platform, host)`.
feed it into `-C metadata=`, `-C extra-filename=` and the output file
name; dependents' `--extern foo=path/to/foo-<our-hash>.rlib` use the
same hash. internally consistent within our generated plan; never seen
by cargo on the runner b/c cargo isn't there.

doesn't try to match cargo's internal `fingerprint::hash_u64` -- chasing
that is a maintenance treadmill for zero functional benefit.

### env-var policy: full coverage

every cargo-set env var rustc + build scripts read:

- from Cargo.toml extract: `CARGO_PKG_NAME`, `CARGO_PKG_VERSION` (+
  `_MAJOR` / `_MINOR` / `_PATCH` / `_PRE`), `CARGO_PKG_AUTHORS`,
  `CARGO_PKG_DESCRIPTION`, `CARGO_PKG_HOMEPAGE`, `CARGO_PKG_LICENSE`,
  `CARGO_PKG_LICENSE_FILE`, `CARGO_PKG_REPOSITORY`,
  `CARGO_PKG_RUST_VERSION`, `CARGO_PKG_README`
- per-unit: `CARGO_CRATE_NAME`, `CARGO_MANIFEST_DIR`,
  `CARGO_MANIFEST_PATH`, `CARGO_BIN_NAME` (for bins)
- per-feature: `CARGO_FEATURE_<NAME>=1` for each enabled feature
- build script extras: `OUT_DIR`, `TARGET`, `HOST`, `OPT_LEVEL`,
  `PROFILE`, `DEBUG`, `NUM_JOBS=1`, `RUSTC`, `RUSTC_LINKER`
- from `--cfg`: `CARGO_CFG_TARGET_OS`, `_ARCH`, `_FAMILY`, `_ENV`,
  `_POINTER_WIDTH`, `_ENDIAN`, `_FEATURE` (comma-joined),
  `_HAS_ATOMIC` (etc), `CARGO_CFG_UNIX`/`_WINDOWS`

cfg parser converts `rustc --print cfg` text into the `CARGO_CFG_*` map.

### cli shape: explicit `nqc build` subcommand

new top-level verb. flags:

- `--cfg <path>` -- required. raw `rustc --print cfg` output for host.
- positional `<unit-graph.json>` and `<out-build-plan.json>`.

no gating flag; the help text + README mark it as experimental. failures
are expected during early iteration; reusable users can pin nqc version.

### code structure: `cargo/unitgraph/` package, file-per-concern

```
go/cargo/unitgraph/
  build.go      -- orchestrator: builds a cargo.BuildPlan from a UnitGraph
  args.go       -- rustc args from a single unit
  env.go        -- env-var synthesis (incl. CARGO_CFG_*)
  hash.go       -- self-consistent metadata hash
  paths.go      -- output path computation
  manifest.go   -- Cargo.toml loader (per-pkg metadata extract)
  *_test.go     -- table-driven per file
  testdata/
    fd/
      ug.json        -- captured + path-anonymised
      build-plan.json -- captured + path-anonymised, ground truth
      README.md      -- provenance: sha, rust version, capture cmd
```

### tests

two layers:

- **unit tests** per file: table-driven against hand-built unit-graph
  fragments. hash determinism, args derivation per profile + edition,
  env synthesis, cfg parsing, path computation.
- **golden integration**: capture full unit-graph + `--build-plan` for
  [fd][fd] (pinned by sha) inside `rust:1.84` (where `--build-plan`
  still works). anonymise both via the existing patch step so the
  fixtures contain `{{PROJECT_ROOT}}` / `{{CARGO_HOME}}` placeholders
  instead of machine-specific paths. test loads the unit-graph, builds,
  compares semantically against the captured build-plan:
  - exact match: `program`, `compile_mode`, `target_kind`,
    `package_name`, env keys, env values for cargo-derived vars
  - normalised match: filenames -- hash component replaced with a
    placeholder before compare (b/c our hash differs from cargo's by
    design)
  - tolerated diffs: arg order within `-L`/`--extern` groups (cargo
    may reorder); we normalise both sides

[fd]: https://github.com/sharkdp/fd

e2e validation happens via the existing `go/examples/sudo-build-plan` demo --
no new CI needed for the experimental feature. once unitgraph is
stable enough, we add an `examples/fd-*` analogue.

## ship plan

seven small commits:

1. `feat: cargo/unitgraph skeleton -- hash + paths`
   smallest landable slice. just `hash.go` + `paths.go` + their tests.
   no orchestrator yet, no integration with anything; demonstrates the
   hash + path scheme is internally consistent.

2. `feat: cargo/unitgraph env-var synthesis`
   `env.go` + cfg parser + tests. still no orchestrator.

3. `feat: cargo/unitgraph args + manifest loader`
   `args.go` + `manifest.go` + tests. now we have all the per-unit
   derivers.

4. `feat: cargo/unitgraph Build orchestrator`
   `build.go` (`Build()` function) wires the above into one entry point. unit tests using
   hand-built small unit-graphs.

5. `feat: nqc build subcommand`
   cli wiring. main.go gains a third subcommand using go-flags. README
   updated to describe the experimental flow.

6. `test: fd unit-graph + build-plan golden fixture`
   capture from rust:1.84, anonymise, commit. golden integration test.

7. `docs: README + sudo-demo notes on the unit-graph experimental flow`

est ~1500 LoC total across src + tests + fixtures. ~2-3 weeks of
part-time work. ship 1+2 first as the smallest combined slice.

## open items / non-goals

- cross-compilation: deferred. mvp assumes host == target. when
  cross-compile lands, `--cfg-host` + `--cfg-target` flags.
- workspace builds with multiple roots: mvp covers single-root only.
  larger workspaces should work but aren't validated.
- target features beyond what cfg surfaces: deferred.
- the `cargo plumbing` project ([crate-ci/cargo-plumbing][plumbing])
  is the longer-term replacement upstream is building. once it
  stabilises, unitgraph likely becomes obsolete. that's fine --
  experimental scope is "buy us time until plumbing is real".

[plumbing]: https://github.com/crate-ci/cargo-plumbing
