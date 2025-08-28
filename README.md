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

## Succesfully compiled

- [sudo-rs](https://github.com/trifectatechfoundation/sudo-rs)
- [fd](https://github.com/sharkdp/fd)
- [eza](https://github.com/eza-community/eza)

## TODOs

- [ ] Instructions on statically compiling python 3.9
- [ ] Instructions on how to complile specific examples above