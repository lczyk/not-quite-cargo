# fd compilation demo (mold linker, no cc/gcc/ld at link time)

second cross-check of the [`../../driver`](../../driver) cc-driver
emulator against a totally independent ld-style linker.
[`../fd-wild`](../fd-wild) builds fd through nqc + wild; this
example does the exact same thing but with
[`mold`](https://github.com/rui314/mold) in wild's place to prove
the driver is linker-agnostic.

mechanics (mirror `fd-wild`):

* planning happens in the upstream `ubuntu/rust:1.85-24.04_edge`
  image w/ cargo + cc intact (`cargo build --no-default-features -Z
  unstable-options --build-plan`)
* the plan is patched on the host with `--linker=/usr/bin/cc` so
  every rustc invocation in the plan carries `-C linker=/usr/bin/cc`
* the run stage spins up the same upstream image but `run.sh` first
  deletes the existing `cc` / `gcc` / `ld` / `cargo` bindings,
  installs `mold` at `/usr/bin/mold`, and symlinks `/usr/bin/cc` ->
  `/usr/local/bin/not-quite-cargo`
* rustc then invokes `/usr/bin/cc <link-args>`; argv[0] = `cc` makes
  nqc shortcut into its `driver.Drive` mode, translate the gcc-style
  args (`-Wl,foo,bar`, `-nodefaultlibs`, etc) into raw ld-style, and
  exec `mold` with the result
* `prove.sh` drops the linked binary into a stock `ubuntu:24.04`
  image (no rust, no mold, network off) and runs `fd --version` +
  a search to confirm the mold-linked binary is functional

the run container is configured by env (no flags in argv[0]==cc mode):

* `NQC_DRIVER_LINKER=/usr/bin/mold` -- forward translated args here
* `NQC_DRIVER_GCC_LIB_DIR=/usr/lib/gcc/<triple>/13` -- the upstream
  image is ubuntu 24.04 with gcc-13, not the default gcc-14

run with `make all`.
