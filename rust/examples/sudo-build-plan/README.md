# sudo-rs compilation demo (rust port)

mirrors [the go example](../../../go/examples/sudo-build-plan/). seven stages:
image -> binary -> cross -> clone -> plan -> run -> prove.

note the extra `cross` stage. go cross-compiles to linux from any host with
`GOOS=linux GOARCH=<arch>` but rust is a bit harder. since we don't wanna
mess with the host's `rustup` etc, we spin up the `ubuntu/rust:1.85-24.04_edge`
rock and compile nqc there to get a proper linux ELF.

everything else is pretty much the same.
