# sudo-rs compilation demo (rust port)

mirrors [the go example](../../../go/examples/sudo-build-plan/). six stages:
image -> binary -> cross -> clone -> plan -> run -> prove.

## diffs from the go version

- **extra `cross` stage** -- go cross-compiles to linux from any host with
  `GOOS=linux GOARCH=<arch>`. rust needs a linux toolchain, so we spin up
  `ubuntu/rust:1.85-24.04_edge`, install git via apt, clone `lczyk/version` by
  hand (container libgit2 is broken on this setup), and redirect cargo's git
  dep with `--config patch."https://...".version.path="/tmp/version/rust"`.
  the resulting `work/not-quite-cargo-cross` is a proper Linux ELF.

- **two binaries** -- `work/not-quite-cargo` is the host binary (Mach-O on
  macOS), used for the `patch` step which runs on the host. the `run` and
  `shell` steps use `work/not-quite-cargo-cross` (Linux ELF).

- **`CROSS_IMAGE` knob** -- overridable image for the cross-compile step.
  defaults to `docker.io/ubuntu/rust:1.85-24.04_edge` (must support edition
  2024). the `PLAN_IMAGE` stays at 1.75 for sudo-rs compatibility.

everything else -- Dockerfile, prove.sh, the plan/run/prove flow -- is
identical to the go example.
