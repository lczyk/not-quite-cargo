# not-quite-cargo

Have you ever needed to compile a big rust project without cargo? No?
How about trying to compile a project with cargo on one machine, rustc
it is trying to use on another one, talking over ssh (with bash shims
in between)? Still no? Ah, what a happy life you lead...

This project allows for first running cargo as a build plan geenrator (using)
the [highly contentious](https://github.com/rust-lang/cargo/issues/7614)
`--build-plan` option to generate the list of steps to execute, and then
executing them separately, possibly with a different rustc / on a different
machine and in different environment. The world is, really, your oyster
(if your idea of oysters is ephemeral container images which you want to
use for compiling your rust projects that is).

This is not necessarilly a new idea, there have been
[other](https://github.com/rust-lang/cargo/issues/5579#issuecomment-438426743)
attampts at doing that, but this particular one is mine.

Tested with cargo `v1.84.1` but ought to work in any version with `--build-plan`. Note, however, that build plan was removed in `1.93.0` in [this](https://github.com/rust-lang/cargo/pull/16212) fateful commit. if you want to go spelunking, the earliest cargo version with `--build-plan` looks like `1.28.0` all the way from the distant land of 2018. i absolutelly *did not* test with that old of a cargo version and i'm sure the plan format changed a bunch there.

For cargo `>= 1.93.0` the go side carries an **experimental** `lower` subcommand that derives a build plan from cargo's `--unit-graph` output (the closest surviving replacement, still nightly-gated). see [`unit-graph-plan.md`](unit-graph-plan.md) for the design and [`go/readme.md`](go/readme.md) for the usage. correctness is best-effort while we figure out which corners of cargo's command-building actually matter for real crates.

## Succesfully compiled

- [sudo-rs](https://github.com/trifectatechfoundation/sudo-rs)
- [fd](https://github.com/sharkdp/fd)
- [eza](https://github.com/eza-community/eza)

## TODOs

- [ ] Instructions on statically compiling python 3.9
- [ ] Instructions on how to complile specific examples above
