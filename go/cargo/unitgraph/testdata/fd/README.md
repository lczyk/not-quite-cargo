# fd unit-graph + build-plan golden fixture

provenance for the captured fixtures in this directory.

| field        | value                                                |
| ------------ | ---------------------------------------------------- |
| crate        | [`fd`](https://github.com/sharkdp/fd)                |
| pinned ref   | `v10.2.0`                                            |
| cargo        | `1.84.1` (inside `rust:1.84` docker image)           |
| capture cmd  | `./capture.sh`                                       |

both files are committed in their anonymised form (paths templated to
`{{PROJECT_ROOT}}` / `{{CARGO_HOME}}` placeholders so the fixtures are
portable). re-anonymisation happens via the existing nqc patch step.

## files

- `ug.json` -- cargo's `-Z unstable-options --unit-graph` output
- `build-plan.json` -- cargo's `-Z unstable-options --build-plan` output;
  this is the ground truth our lower output is compared against
  (semantic match, hash-dependent paths normalised)

## refresh

```
cd go/cargo/unitgraph/testdata/fd
./capture.sh
```

requires docker (or set `DOCKER=podman`). pulls `rust:1.84`, clones fd
at `v10.2.0`, runs both `-Z` flags, runs nqc patch to anonymise. about
3 minutes on a warm image.

if `fd` changes its dep graph in a way that breaks the golden, bump the
pinned ref above + re-run the capture.
