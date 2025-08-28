# not-quite-cargo

Have you ever needed to compile a big rust project without cargo? No? How about trying to compile a project with cargo
on one machine, rustc it's trying to use on another, talking over ssh (with bash shims in between)? Ah, what a happy
life you lead...

This project allows for running cargo as a build plan geenrator (using)

This is not necessarilly a new idea, there have been [other](https://github.com/rust-lang/cargo/issues/5579#issuecomment-438426743)
attampts at doing that, but this particular one is mine.


## Succesfully compiled

- [sudo-rs](https://github.com/trifectatechfoundation/sudo-rs)
- [fd](https://github.com/sharkdp/fd)
- [eza](https://github.com/eza-community/eza)