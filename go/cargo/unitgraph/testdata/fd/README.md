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

- `unit-graph.json` -- cargo's `--unit-graph` output. lower's input.
- `build-plan.json` -- cargo's `--build-plan` output. the ground-truth
  invocation list we compare lower's output against.
- `cfg.txt` -- `rustc --print cfg` for the container's host (linux,
  arch matching the capture machine). feeds lower's `--cfg` flag.

both JSON files are passed through `jq .` for pretty-printing if `jq`
is on the host's PATH; if not the capture leaves them as cargo emitted
them (single-line JSON) -- still valid input for the test, just less
readable when diffing.

paths in the JSON files are container-internal (`/tmp/fd`,
`/tmp/cargo-home`, the image's rustc path) -- stable across captures by
construction, no host-side anonymisation needed. the lowering test
configures matching `ProjectRoot` / `CargoHome` values so the paths
line up.

## refresh

```
cd go/cargo/unitgraph/testdata/fd
./capture.sh
```

requires docker (or set `DOCKER=podman`). pulls `rust:1.84`, clones fd
at the pinned ref, runs both `-Z` flags + `rustc --print cfg`, drops
all three files via a single mount. about 3 minutes on a warm image.
nqc itself is **not** invoked inside the container -- the fixture
pipeline is cargo + rustc only.

if `fd` changes its dep graph in a way that breaks the golden, bump
the pinned ref above + re-run the capture.

## variables

- `FD_REF` -- override the pinned fd revision (default: `v10.2.0`).
- `RUST_IMAGE` -- override the image (default: `rust:1.84`). must be
  in the 1.44 -- 1.92 cargo window for both `-Z` flags to coexist.
- `DOCKER` -- set to `podman` to use podman instead.
