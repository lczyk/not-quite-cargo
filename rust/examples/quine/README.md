# quine

nqc compiles nqc. six stages:
image -> binary -> cross -> clone -> plan -> prove.

same shape as [sudo-build-plan](../sudo-build-plan/), but the project
being built is this very repo. live `rust/` source is bind-mounted
into the plan / prove containers (read-only) with a separate target
dir overlayed at `/work/target` so the host's `rust/target` stays
untouched. the `version` git dep is cloned into `work/version-src/`
once and redirected via `--config patch` in the cross + plan steps.

`prove` closes the loop in a fresh demo-image container (no cargo,
no network): the cross nqc builds nqc (round 1), the result is dropped
at `/usr/local/bin/not-quite-cargo-02` and used to rebuild (round 2),
and so on for five rounds. each round wipes `/work/target` first so
the build actually reruns.
