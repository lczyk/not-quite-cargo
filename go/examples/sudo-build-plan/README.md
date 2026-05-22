# sudo-rs compilation demo

end-to-end demo of building [sudo-rs](https://github.com/trifectatechfoundation/sudo-rs)
without cargo in the build host, only on the machine on which we plan the build

rust depends on `libpam0g-dev` which we need to supply somehow. we could just built it outside od the containter but, for convenience, here we pre-built the build image, drop `libpam0g-dev` libs (and all transitive deps) into the right place on the filesystem and, jsut to not have any aces in one's sleeve, remove `cargo` binary.

planing then happens in upstream `ubuntu/rust:1.75-24.04` (rock). it uses that image's `cargo build -Z unstable-options --build-plan > build-plan.json`. then we run `not-quite-cargo patch build-plan.json` on it (on the host) to patch out build-specific paths / variables and drop it into the pre-built build image -- without cargo but with `nqc` -- which we also deprived of a network bridge just to make sure nothing touches network and... it builds(!)...

ok, just because it does not fail does not mean it works. let's then proove it works by dropping the compiled binary into *yet another* image ( this time `ubuntu:24.04` ) set up all the "correct" sudo config like `root ALL=(ALL:ALL) NOPASSWD: ALL` and we get `sudo whoami | grep -F root` passing.

run with `make install`
