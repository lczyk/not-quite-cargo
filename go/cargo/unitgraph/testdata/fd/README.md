# fd unit-graph + build-plan golden fixture

provenance for the captured fixtures in this directory.

| field        | value                                                |
| ------------ | ---------------------------------------------------- |
| crate        | [`fd`](https://github.com/sharkdp/fd)                |
| pinned ref   | `v10.2.0`                                            |
| image        | `rust:1.84` (cargo 1.84.x; in the overlap window)    |
| capture cmd  | `./capture.sh`                                       |

cargo `1.44.0 -- 1.92.x` is the overlap window where both
`-Z unstable-options --unit-graph` and `-Z unstable-options --build-plan`
exist. `rust:1.84` sits in the middle, so a single container can emit
both fixtures plus `rustc --print cfg` in one pass.

## files

- `ug.json` -- cargo's `--unit-graph` output. lower's input.
- `build-plan.json` -- cargo's `--build-plan` output. the ground-truth
  invocation list we compare lower's output against.
- `host-cfg.txt` -- `rustc --print cfg` for the container's host (linux,
  arch matching the capture machine). feeds lower's `--cfg` flag.

paths in the JSON files are anonymised: container-side `/tmp/fd` and
`/cargo-home` become `{{PROJECT_ROOT}}` and `{{CARGO_HOME}}` so the
fixtures stay portable.

## refresh

```
cd go/cargo/unitgraph/testdata/fd
./capture.sh
```

requires docker (or set `DOCKER=podman`). pulls `rust:1.84`, clones fd
at the pinned ref, runs both `-Z` flags + `rustc --print cfg`, sed-
anonymises the path-bearing JSON files. about 3 minutes on a warm
image. nqc itself is **not** invoked inside the container -- the
fixture pipeline is cargo + rustc + sed only.

if `fd` changes its dep graph in a way that breaks the golden, bump
the pinned ref above + re-run the capture.

## variables

- `FD_REF` -- override the pinned fd revision (default: `v10.2.0`).
- `RUST_IMAGE` -- override the image (default: `rust:1.84`). must be
  in the 1.44 -- 1.92 cargo window for both `-Z` flags to coexist.
- `DOCKER` -- set to `podman` to use podman instead.
