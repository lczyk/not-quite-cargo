# fd unit-graph golden fixture

provenance for the captured fixtures in this directory.

| field        | value                                                |
| ------------ | ---------------------------------------------------- |
| crate        | [`fd`](https://github.com/sharkdp/fd)                |
| pinned ref   | `v10.2.0`                                            |
| cargo        | `1.94.0` (host) for `ug.json`                        |
| target       | `aarch64-apple-darwin` (irrelevant after anonymise)  |
| capture      | locally on the host, see below                       |

`ug.json` was captured by:

```
git clone --depth 1 --branch v10.2.0 https://github.com/sharkdp/fd.git
cd fd
cargo fetch
RUSTC_BOOTSTRAP=1 cargo build -Z unstable-options --unit-graph > ug.raw.json
```

then anonymised (planner-side paths `/Users/.../`, `/tmp/...`, and the
absolute `rustc` path replaced with `{{CARGO_HOME}}`, `{{PROJECT_ROOT}}`,
`{{RUSTC}}` placeholders). this makes the fixture portable across hosts.

## ground-truth build-plan

a corresponding `build-plan.json` (cargo's own `--build-plan` output) is
not committed yet. `--build-plan` was removed in cargo 1.93.0, so it
must be captured inside `rust:1.84` via `./capture.sh` -- which requires
docker and a few minutes. when present, a future stricter test can diff
the lower output against it semantically.

## refresh

```
cd go/cargo/unitgraph/testdata/fd
./capture.sh
```

requires docker (or set `DOCKER=podman`). pulls `rust:1.84`, clones fd
at the pinned ref, runs both `-Z` flags, runs nqc patch to anonymise.
about 3 minutes on a warm image.

if `fd` changes its dep graph in a way that breaks the golden, bump the
pinned ref above + re-run the capture.
