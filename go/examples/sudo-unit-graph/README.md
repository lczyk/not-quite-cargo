# sudo-rs compilation demo via --unit-graph

this example mirrors [`../sudo-build-plan`](../sudo-build-plan) but here we we use a newer cargo version (`1.93`) and therefore don't have `--build-plan` (sad) so we have to remake it ourselves from `--unit-graph`.

the only real difference is that the ouptu of the plan container is `unit-graph.json` which we then buid and patch on host. from then all proceeds in the same way as in the `build-plan` example.

in this example, to match the `ubuntu/rust:1.93-26.04_edge` image used for the build base, we also use `ubuntu:26.04` (rock) as the prove image.
